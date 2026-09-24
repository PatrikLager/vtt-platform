// Package identity manages participants, invite tokens, revocation and the
// campaign's join door, in SQLite tables beside the event log (SPEC-009).
package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// Do not add a migration to drop the `controls` column an old campaign file
// carries: every such open would then write, which read-only media refuses
// (TestAnAlreadyMigratedReadOnlyCampaignStillOpens). No statement names it.
const schema = `
CREATE TABLE IF NOT EXISTS participants (
  id           TEXT PRIMARY KEY,
  display_name TEXT,
  role         TEXT,
  token_hash   BLOB UNIQUE,
  revoked      INTEGER DEFAULT 0
);

-- The shared join door (SPEC-009). A SEPARATE TABLE rather than columns on
-- participants: Open applies this schema with CREATE TABLE IF NOT EXISTS,
-- which is a NO-OP on a table that already exists, so a new COLUMN never
-- reaches an existing campaign and a new TABLE does.
--
-- id = 1 and the CHECK make it a single row by construction.
--
-- Closed-by-default is carried by the INSERTs in ensureJoinRow and
-- RotateJoinSecret, which write open=0 EXPLICITLY, and by SetJoinOpen's, which
-- writes the state asked for. This DEFAULT 0 is not the guard: it covers a row
-- inserted by some other path.
CREATE TABLE IF NOT EXISTS join_access (
  id          INTEGER PRIMARY KEY CHECK (id = 1),
  secret      TEXT NOT NULL,
  open        INTEGER NOT NULL DEFAULT 0,
  -- The admission budget: admitted counts what THIS OPENING has let through,
  -- admit_limit is what the DM allowed, and both reset when the door opens.
  -- These are COLUMNS on an existing table, which this schema cannot deliver
  -- on its own; migrate adds them.
  admitted    INTEGER NOT NULL DEFAULT 0,
  admit_limit INTEGER NOT NULL DEFAULT 0
);`

// Keep migrate idempotent: ALTER TABLE ADD COLUMN errors on a column already
// there, and this runs on every Open. Keep the budget on join_access's one row,
// so that spending an admission stays ONE conditional UPDATE.
func migrate(db *sql.DB) error {
	// Read first and take no write lock when nothing is pending: this runs on every
	// Open, the file is shared with internal/store's append transaction, and a
	// current campaign on read-only media must still open.
	ctx := context.Background()
	pending, err := migrationPending(ctx, db)
	if err != nil {
		return err
	}
	if !pending {
		return nil
	}

	// Keep BEGIN IMMEDIATE on one pinned connection, never db.Begin(): DEFERRED
	// takes a read lock it must upgrade, busy_timeout does not retry an upgrade,
	// and two openers racing must re-read the shape under the lock (migrateLocked).
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("identity: migrate: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("identity: migrate: begin: %w", err)
	}
	if err := migrateLocked(ctx, conn); err != nil {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		return err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("identity: migrate: commit: %w", err)
	}
	return nil
}

// Keep migrationPending read-only: it decides whether the write lock is taken.
func migrationPending(ctx context.Context, db *sql.DB) (bool, error) {
	have, err := columnNames(ctx, db, joinAccessShape)
	if err != nil {
		return false, err
	}
	if !have["admitted"] || !have["admit_limit"] {
		return true, nil
	}

	var stranded int
	if err := db.QueryRow(
		`SELECT count(*) FROM join_access WHERE open = 1 AND admit_limit = 0`).Scan(&stranded); err != nil {
		return false, fmt.Errorf("identity: read join budget state: %w", err)
	}
	return stranded > 0, nil
}

// Keep the pragma a literal: SQLite will not bind a parameter in a PRAGMA.
// Both callers must name the table identically, so the label travels with it.
type tableShape struct {
	pragma string
	name   string
}

var joinAccessShape = tableShape{`PRAGMA table_info(join_access)`, "join_access"}

// Do not narrow shapeReader: r is *sql.DB before the lock and *sql.Conn under it.
func columnNames(ctx context.Context, r shapeReader, t tableShape) (map[string]bool, error) {
	rows, err := r.QueryContext(ctx, t.pragma)
	if err != nil {
		return nil, fmt.Errorf("identity: read %s shape: %w", t.name, err)
	}
	defer rows.Close()

	have := map[string]bool{}
	for rows.Next() {
		// Scan as `any`: PRAGMA table_info yields cid, name, type, notnull,
		// dflt_value, pk, and dflt_value is NULL for a column with no default.
		var cid, name, typ, notnull, dfltValue, pk any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("identity: read %s shape: %w", t.name, err)
		}
		if s, ok := name.(string); ok {
			have[s] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: read %s shape: %w", t.name, err)
	}
	return have, nil
}

type shapeReader interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func migrateLocked(ctx context.Context, conn *sql.Conn) error {
	// Re-read under the lock: a concurrent opener may have finished the migration
	// while this one waited. Wrap "migrate:" here, or an operator cannot tell this
	// read from migrationPending's. Unreachable through testdb:
	// docs/verification-debt.md.
	have, err := columnNames(ctx, conn, joinAccessShape)
	if err != nil {
		return fmt.Errorf("identity: migrate: %w", err)
	}

	if !have["admitted"] {
		if _, err := conn.ExecContext(ctx,
			`ALTER TABLE join_access ADD COLUMN admitted INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("identity: add join_access.admitted: %w", err)
		}
	}
	if !have["admit_limit"] {
		if _, err := conn.ExecContext(ctx,
			`ALTER TABLE join_access ADD COLUMN admit_limit INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("identity: add join_access.admit_limit: %w", err)
		}
	}

	// Key the repair on the state, never on which ALTER ran: a door left open
	// before the budget existed arrives with admit_limit 0, which admits nobody;
	// SetJoinOpen coerces every budget to at least 1, so no live row matches.
	if _, err := conn.ExecContext(ctx,
		`UPDATE join_access SET admit_limit = ?, admitted = 0
		 WHERE open = 1 AND admit_limit = 0`, DefaultAdmitLimit); err != nil {
		return fmt.Errorf("identity: budget an already-open door: %w", err)
	}
	return nil
}

// Role is a participant's authorization level, one of the four ParseRole accepts.
type Role string

const (
	RoleDM        Role = "dm"
	RoleAgent     Role = "agent"
	RolePlayer    Role = "player"
	RoleSpectator Role = "spectator"
)

// ParseRole parses s as a Role, accepting exactly the four defined roles.
func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleDM, RoleAgent, RolePlayer, RoleSpectator:
		return Role(s), nil
	default:
		return "", fmt.Errorf("identity: unknown role %q", s)
	}
}

// Participant is a resolved identity: who a token belongs to, and at what role.
// Do not add what they control: that is Actor.controller_ids in the log (SPEC-009).
type Participant struct {
	ID   string
	Name string
	Role Role
}

// ErrInvalidToken is Verify's answer for an unknown, malformed or revoked token,
// and Lookup's for an unknown or revoked id. Keep it one error: a distinct
// answer is a token-probing oracle.
var ErrInvalidToken = errors.New("identity: invalid or revoked token")

// DB is a handle on a campaign's identity tables.
type DB struct {
	db *sql.DB
}

// Set driverName only from this package's internal tests, and restore it: it
// exists so internal/testdb can fail a statement mid-operation, which closing
// a handle cannot (that fails the first statement and reaches no later arm).
var driverName = "sqlite"

// Open opens the campaign file's identity tables, creating them if necessary.
// Keep it independent of store.Open; both are open on one file at once.
func Open(path string) (*DB, error) {
	// Keep busy_timeout(5000), the SQLITE_BUSY hardening internal/store's Open has.
	db, err := sql.Open(driverName, path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("identity: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("identity: init schema: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

// JoinOpen reports whether the door is open.
// Answer false on any error: this gates an unauthenticated, row-minting
// endpoint (VTT-048).
func (d *DB) JoinOpen() bool {
	var open int
	err := d.db.QueryRow(`SELECT open FROM join_access WHERE id = 1`).Scan(&open)
	if err != nil {
		return false
	}
	return open == 1
}

// SetJoinOpen opens or closes the door and sets this opening's admission budget.
// Reset admitted on every call, a close included, so a shut door reports nothing
// spent; write the limit on every call too, JoinBudget reports it. Coerce a
// non-positive limit to DefaultAdmitLimit: protojson omits zero values (SPEC-009).
func (d *DB) SetJoinOpen(open bool, admitLimit int) error {
	v := 0
	if open {
		v = 1
	}
	if admitLimit <= 0 {
		admitLimit = DefaultAdmitLimit
	}
	// Keep it one upsert: atomic, and it cannot leave the row half-made.
	secret := newSecret()
	if _, err := d.db.Exec(
		`INSERT INTO join_access (id, secret, open, admitted, admit_limit)
		 VALUES (1, ?, ?, 0, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   open = excluded.open,
		   admitted = 0,
		   admit_limit = excluded.admit_limit`, secret, v, admitLimit,
	); err != nil {
		return fmt.Errorf("identity: set join open: %w", err)
	}
	return nil
}

// JoinBudget reports what this opening has spent and allowed.
// Keep it read-only: the DM console polls it, and a poll must not take the
// write lock on the file internal/store appends to.
func (d *DB) JoinBudget() (admitted, limit int, err error) {
	err = d.db.QueryRow(
		`SELECT admitted, admit_limit FROM join_access WHERE id = 1`).Scan(&admitted, &limit)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("identity: read join budget: %w", err)
	}
	return admitted, limit, nil
}

// JoinSecret returns the current join secret, minting one on first use.
// Keep it stable between rotations: the DM shares it (VTT-040).
func (d *DB) JoinSecret() (string, error) {
	return d.ensureJoinRow()
}

// DefaultAdmitLimit is what a door opened without a stated budget allows.
// Keep it a number, not "unlimited": protojson omits zero values, so an absent
// field and "admit nobody" are the same bytes (SPEC-009).
const DefaultAdmitLimit = 8

// JoinAdmits reports whether this candidate may come through the door and spends
// one admission if so; a refusal the read can decide writes nothing.
// Keep the compare here, in Go, constant-time: the secret is never handed out to
// be checked, and an empty stored secret must refuse ("" matches ""). Keep every
// refusal before the UPDATE: one matching no rows still takes the write lock.
func (d *DB) JoinAdmits(candidate string) (bool, error) {
	var (
		secret                 string
		open, admitted, budget int
	)
	err := d.db.QueryRow(
		`SELECT secret, open, admitted, admit_limit FROM join_access WHERE id = 1`,
	).Scan(&secret, &open, &admitted, &budget)
	if errors.Is(err, sql.ErrNoRows) {
		// No row means shut; answer without creating one (VTT-017).
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("identity: read join access: %w", err)
	}
	match := subtle.ConstantTimeCompare([]byte(secret), []byte(candidate)) == 1
	if open != 1 || secret == "" || !match || admitted >= budget {
		return false, nil
	}

	// Do not add a compensating decrement for a CreateInvite failure after this:
	// a second write that can itself fail can only err toward admitting more
	// (SPEC-009).
	res, err := d.db.Exec(
		`UPDATE join_access SET admitted = admitted + 1
		 WHERE id = 1 AND open = 1 AND admitted < admit_limit`)
	if err != nil {
		return false, fmt.Errorf("identity: spend join admission: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("identity: spend join admission: %w", err)
	}
	// Keep n == 0 a refusal: a concurrent joiner took the last slot between the
	// read and the update, and nothing is spent (VTT-023).
	return n == 1, nil
}

// RotateJoinSecret replaces the secret and returns the new one; participants
// already through keep their tokens (VTT-020).
func (d *DB) RotateJoinSecret() (string, error) {
	secret := newSecret()
	// Keep the DO UPDATE off `open` and `admit_limit`: rotating says nothing about
	// the door (VTT-043) or the budget. Reset admitted: a new secret is a new
	// opening, or a link rotated after a spent budget admits nobody (VTT-044).
	if _, err := d.db.Exec(
		`INSERT INTO join_access (id, secret, open, admitted, admit_limit) VALUES (1, ?, 0, 0, 0)
		 ON CONFLICT(id) DO UPDATE SET secret = excluded.secret, admitted = 0`, secret,
	); err != nil {
		return "", fmt.Errorf("identity: rotate join secret: %w", err)
	}
	return secret, nil
}

func (d *DB) ensureJoinRow() (string, error) {
	secret := newSecret()

	// Read first: the upsert below writes even on the conflict path, and this file
	// is shared with internal/store's append transaction. Unconditional, it blocks
	// the DM console's poll for the full busy_timeout(5000), then fails SQLITE_BUSY.
	var stored string
	err := d.db.QueryRow(`SELECT secret FROM join_access WHERE id = 1`).Scan(&stored)
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("identity: read join secret: %w", err)
	}

	// Keep `SET secret = secret`: a no-op whose only job is to make RETURNING fire
	// on the conflict path, which INSERT OR IGNORE would not.
	if err := d.db.QueryRow(
		`INSERT INTO join_access (id, secret, open) VALUES (1, ?, 0)
		 ON CONFLICT(id) DO UPDATE SET secret = secret
		 RETURNING secret`, secret,
	).Scan(&stored); err != nil {
		return "", fmt.Errorf("identity: mint join secret: %w", err)
	}
	return stored, nil
}

// Keep no error return: crypto/rand.Read never returns one; it fills the
// buffer or crashes the program.
func newSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// SetRole changes a participant's authorization level and nothing else.
// Keep the role in participants.role, never in the log: the fold has no Role
// (SPEC-009, VTT-039). Keep the token and id untouched, or a promotion logs
// them out (VTT-029); leave revoked alone (VTT-030).
func (d *DB) SetRole(id string, role Role) error {
	if _, err := ParseRole(string(role)); err != nil {
		return err
	}
	res, err := d.db.Exec(`UPDATE participants SET role = ? WHERE id = ?`, string(role), id)
	if err != nil {
		return fmt.Errorf("identity: set role: %w", err)
	}
	// Report n == 0: silently promoting somebody who has left is worse than an error.
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity: set role: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("identity: no participant %q", id)
	}
	return nil
}

// Close releases the underlying SQLite handle.
func (d *DB) Close() error {
	return d.db.Close()
}

// CreateInvite mints a participant and a one-time invite token, returned exactly
// once; only its SHA-256 hash is stored (VTT-041). Take no list of actors:
// control is an ActorControlGranted in the log (SPEC-009).
func (d *DB) CreateInvite(name string, role Role) (token string, id string, err error) {
	if _, err := ParseRole(string(role)); err != nil {
		return "", "", err
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", fmt.Errorf("identity: generate token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", fmt.Errorf("identity: generate id: %w", err)
	}
	id = hex.EncodeToString(idBytes)

	if _, err := d.db.Exec(
		`INSERT INTO participants (id, display_name, role, token_hash, revoked) VALUES (?, ?, ?, ?, 0)`,
		id, name, string(role), hash[:],
	); err != nil {
		return "", "", fmt.Errorf("identity: insert participant: %w", err)
	}
	return token, id, nil
}

// Verify resolves token to its Participant.
// Keep the lookup a plain equality on the SHA-256 hash (not secret to someone
// without the token) and the confirmation subtle.ConstantTimeCompare
// (TestVerifyUsesConstantTimeCompare).
func (d *DB) Verify(token string) (*Participant, error) {
	sum := sha256.Sum256([]byte(token))

	row := d.db.QueryRow(
		`SELECT id, display_name, role, token_hash, revoked FROM participants WHERE token_hash = ?`,
		sum[:],
	)
	var (
		id, name, roleStr string
		storedHash        []byte
		revoked           int
	)
	if err := row.Scan(&id, &name, &roleStr, &storedHash, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("identity: query participant: %w", err)
	}

	if subtle.ConstantTimeCompare(sum[:], storedHash) != 1 {
		return nil, ErrInvalidToken
	}
	if revoked != 0 {
		return nil, ErrInvalidToken
	}

	role, err := ParseRole(roleStr)
	if err != nil {
		return nil, fmt.Errorf("identity: stored role invalid: %w", err)
	}

	return &Participant{ID: id, Name: name, Role: role}, nil
}

// Lookup resolves a participant by id, as they are now.
// Keep revoked and unknown one error, as Verify does; the gateway re-resolves
// through here on every command and delivery (SPEC-009).
func (d *DB) Lookup(id string) (*Participant, error) {
	var (
		name, roleStr string
		revoked       int
	)
	err := d.db.QueryRow(
		`SELECT display_name, role, revoked FROM participants WHERE id = ?`, id,
	).Scan(&name, &roleStr, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, fmt.Errorf("identity: lookup %s: %w", id, err)
	}
	if revoked != 0 {
		return nil, ErrInvalidToken
	}
	role, err := ParseRole(roleStr)
	if err != nil {
		// Wrap it like every other failure here, naming whose row it was.
		return nil, fmt.Errorf("identity: lookup %s: stored role invalid: %w", id, err)
	}
	return &Participant{ID: id, Name: name, Role: role}, nil
}

// List returns everyone who can still act at this table, ordered by display name.
// Keep revoked participants out (VTT-034). Order in SQL by display_name then
// id, a total order, so two consumers agree.
func (d *DB) List() ([]*Participant, error) {
	rows, err := d.db.Query(
		`SELECT id, display_name, role FROM participants
		 WHERE revoked = 0 ORDER BY display_name, id`)
	if err != nil {
		return nil, fmt.Errorf("identity: list participants: %w", err)
	}
	defer rows.Close()

	var out []*Participant
	for rows.Next() {
		var id, name, roleStr string
		if err := rows.Scan(&id, &name, &roleStr); err != nil {
			return nil, fmt.Errorf("identity: list participants: %w", err)
		}
		role, err := ParseRole(roleStr)
		if err != nil {
			// Refuse, never skip or default: a stored role that is not a role means the
			// table's authorization data is wrong.
			return nil, fmt.Errorf("identity: list %s: stored role invalid: %w", id, err)
		}
		out = append(out, &Participant{ID: id, Name: name, Role: role})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list participants: %w", err)
	}
	return out, nil
}

// Revoke permanently flips the revoked flag for participant id; a table
// mutation the log cannot undo (SPEC-009).
func (d *DB) Revoke(id string) error {
	res, err := d.db.Exec(`UPDATE participants SET revoked = 1 WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("identity: revoke %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity: revoke %s: %w", id, err)
	}
	if n == 0 {
		return fmt.Errorf("identity: revoke: unknown participant %q", id)
	}
	return nil
}
