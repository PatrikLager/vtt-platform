// Package identity manages participants, invite tokens, revocation and the
// campaign's join door. It opens its OWN SQLite handle on the same campaign
// file the store uses and is deliberately NOT event-sourced: revocation is not
// undone by replaying the log (SPEC-009).
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

// THERE IS NO `controls` COLUMN, and no statement in this package names one. A
// campaign file still carrying it opens, and the column is inert
// (TestJoinIsClosedOnAnExistingCampaign's fixture carries one). Do NOT add a
// migration to drop it: a migration that dropped it would have to make
// migrationPending answer yes for every campaign still carrying it, which takes
// migrate to BEGIN IMMEDIATE on open, which read-only media cannot give.
// Control is Actor.controller_ids in the log, read by gateway's authz.go.
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

// migrate adds the columns the schema above cannot deliver on its own, and
// budgets an already-open door the addition would otherwise strand.
//
// It runs on every Open and must stay idempotent: ALTER TABLE ADD COLUMN is an
// error, not a no-op, on a column already there. The budget lives on the same
// single row as `open` so that spending an admission is ONE conditional UPDATE
// against ONE row. It touches only join_access.
func migrate(db *sql.DB) error {
	// READ FIRST, and take no write lock at all when there is nothing to do.
	// This runs on EVERY Open, the file is shared with internal/store, which
	// writes inside a transaction on every event append, and an unconditional
	// BEGIN IMMEDIATE here takes the write lock on a read-only user's open
	// (`vtt state dump`, the DM console's polling) and makes a campaign on
	// read-only media impossible to open.
	ctx := context.Background()
	pending, err := migrationPending(ctx, db)
	if err != nil {
		return err
	}
	if !pending {
		return nil
	}

	// ONE PINNED CONNECTION, inside BEGIN IMMEDIATE. The scan and the ALTERs
	// are separate statements, so two processes opening the same campaign at
	// once both see the columns missing and both try to add them; the loser
	// dies on `duplicate column name` unless it waits for the write lock and
	// re-reads the shape under it, which migrateLocked does.
	//
	// IMMEDIATE and not db.Begin(), which is DEFERRED: a deferred transaction
	// takes a read lock first and must UPGRADE it to write, and busy_timeout
	// does not retry a lock upgrade, so the losers fail SQLITE_BUSY at once.
	// IMMEDIATE takes the write lock up front and the losers wait on it.
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

// migrationPending reports whether anything needs writing: a missing column, or
// an open door still carrying no budget. Reads only.
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

// tableShape is a table the migration reads the column set of: the PRAGMA that
// reads it, and the name that goes in the error when it cannot be read.
//
// ONE value carrying both, so a call site cannot pass a pragma and a label that
// disagree and mislabel the one message an operator gets. The pragma is a
// LITERAL rather than built from the name: SQLite will not accept a bind
// parameter in a PRAGMA, so building it would mean interpolating into SQL.
// Both callers must name the table identically.
type tableShape struct {
	pragma string
	name   string
}

var joinAccessShape = tableShape{`PRAGMA table_info(join_access)`, "join_access"}

// columnNames reads a table's column set: query it, drain it, close it.
//
// ONE implementation for TWO call sites, migrationPending before the lock and
// migrateLocked under it, so the two cannot name the table differently in the
// one message an operator gets.
//
// The rows handle is closed by DEFER in exactly one place, and migrateLocked
// must have it closed BEFORE its ALTER TABLE: SQLite will not alter a table
// with an open cursor on it.
//
// r is *sql.DB when nothing is locked and *sql.Conn when the migration holds
// the write lock; shapeReader is the narrowest interface serving both.
func columnNames(ctx context.Context, r shapeReader, t tableShape) (map[string]bool, error) {
	rows, err := r.QueryContext(ctx, t.pragma)
	if err != nil {
		return nil, fmt.Errorf("identity: read %s shape: %w", t.name, err)
	}
	defer rows.Close()

	have := map[string]bool{}
	for rows.Next() {
		// PRAGMA table_info yields cid, name, type, notnull, dflt_value, pk —
		// in that order. dflt_value is NULL for a column with no default, so
		// these are scanned as `any` rather than into typed variables.
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

// shapeReader is whatever can run the PRAGMA: the pool before the migration
// takes its lock, the pinned connection after.
type shapeReader interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// migrateLocked does the work, with the write lock already held.
func migrateLocked(ctx context.Context, conn *sql.Conn) error {
	// RE-READ inside the transaction: a concurrent opener may have completed
	// the whole migration while this one waited for the write lock, and ALTER
	// TABLE ADD COLUMN is an error on a column already there. WRAPPED
	// "migrate:", which the read in migrationPending is not, or an operator
	// cannot tell which of the two failed. This error arm is unreachable
	// through testdb; docs/verification-debt.md carries the gap.
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

	// A door this campaign's DM left OPEN keeps working, on a fresh budget: the
	// columns arrive defaulted to 0, and 0 admits nobody.
	//
	// KEYED ON THE STATE, never on which ALTER just ran: a database carrying
	// admit_limit but not admitted would otherwise open cleanly, keep a budget
	// of 0 and refuse every joiner at a door reading "open". The predicate
	// below is exactly "an open door with no budget" and cannot match a
	// legitimate row, because SetJoinOpen coerces every budget to at least 1.
	if _, err := conn.ExecContext(ctx,
		`UPDATE join_access SET admit_limit = ?, admitted = 0
		 WHERE open = 1 AND admit_limit = 0`, DefaultAdmitLimit); err != nil {
		return fmt.Errorf("identity: budget an already-open door: %w", err)
	}
	return nil
}

// Role is a participant's authorization level. The four roles are the
// complete set; ParseRole rejects everything else.
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

// Participant is a resolved identity: who this token belongs to, and at what
// role. NOT what they control — that is Actor.controller_ids in the log, and
// this type deliberately cannot answer it (see schema's note).
type Participant struct {
	ID   string
	Name string
	Role Role
}

// ErrInvalidToken is returned by Verify for a token that is unknown,
// malformed, or revoked. It deliberately does not distinguish between the
// three so callers cannot use error content to probe token validity.
var ErrInvalidToken = errors.New("identity: invalid or revoked token")

// DB is a handle on a campaign's participants table.
type DB struct {
	db *sql.DB
}

// driverName is "sqlite" in production. It is a variable only so that
// internal/testdb can substitute a fault-injecting wrapper: this package's
// error arms decide what happens when a database fails MID-OPERATION, and
// closing a handle is no substitute, because that fails the FIRST statement
// and leaves every later arm unreached. Set only by this package's internal
// tests, each of which restores it in t.Cleanup.
var driverName = "sqlite"

// Open opens (creating if necessary) the participants and join_access tables
// on the SQLite file at path. This is an independent handle from store.Open;
// both may be open on the same campaign file at once.
func Open(path string) (*DB, error) {
	// busy_timeout(5000): the same SQLITE_BUSY hardening as
	// internal/store/store.go's Open.
	db, err := sql.Open(driverName, path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("identity: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close() // closing a handle whose schema init just failed
		return nil, fmt.Errorf("identity: init schema: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{db: db}, nil
}

// JoinOpen reports whether the shared join link currently admits anybody.
//
// FALSE on any error, deliberately. This answer gates an unauthenticated,
// row-minting endpoint, so a database that cannot be read must refuse to let
// people in rather than fail open — the one direction where being wrong is
// expensive.
func (d *DB) JoinOpen() bool {
	var open int
	err := d.db.QueryRow(`SELECT open FROM join_access WHERE id = 1`).Scan(&open)
	if err != nil {
		return false
	}
	return open == 1
}

// SetJoinOpen opens or closes the door and sets the admission budget for THIS
// opening.
//
// admitted resets to 0 on EVERY call, a close included, so a shut door
// reports nothing spent. The limit is written on every call too, so a limit
// given when closing is stored and reported by JoinBudget until the next
// opening overwrites it.
//
// A non-positive admitLimit becomes DefaultAdmitLimit rather than "admit
// nobody": protojson omits zero values, so an absent field and a deliberate 0
// arrive as the same bytes, and a door that admits no one cannot be debugged
// from either end (SPEC-009).
func (d *DB) SetJoinOpen(open bool, admitLimit int) error {
	v := 0
	if open {
		v = 1
	}
	if admitLimit <= 0 {
		admitLimit = DefaultAdmitLimit
	}
	// ONE upsert rather than ensure-then-update: atomic, one round trip, and
	// it cannot leave the row half-made if a second statement fails.
	secret := newSecret() // used only if the row does not exist yet
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

// JoinBudget reports how many admissions this opening has spent and allowed.
//
// Read-only, and it MINTS NOTHING: the DM console polls it, and a poll must
// not take SQLite's write lock on the file internal/store appends events to.
// Both zero on a campaign whose door has never been touched. On a shut door it
// reports nothing spent and the limit the last SetJoinOpen wrote.
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
//
// STABLE until rotated: the DM shares it, so a value that changed per call
// would invalidate the link the moment anyone looked at it.
func (d *DB) JoinSecret() (string, error) {
	return d.ensureJoinRow()
}

// DefaultAdmitLimit is what a door opened without a stated budget allows.
//
// A number, not "unlimited": protojson omits zero values, so an absent field
// and "admit nobody" are the same wire bytes, and the default has to be chosen
// here rather than inferred from what arrived (SPEC-009).
const DefaultAdmitLimit = 8

// JoinAdmits reports whether this candidate may come through the door, AND
// spends one admission if so. A refusal the read can decide writes NOTHING.
//
// The work is split deliberately, because a cap that two concurrent joiners
// can both pass is not a cap:
//
//   - The SECRET is compared HERE, in Go, in constant time, and is never
//     handed out of this package to be checked. An EMPTY STORED SECRET admits
//     nobody, and the guard says so explicitly: ConstantTimeCompare("", "")
//     returns 1, and a request body omitting the field decodes to "".
//   - REFUSALS are decided here too, from the same read: wrong secret, shut
//     door, budget spent. An UPDATE matching zero rows still takes SQLite's
//     write lock on the file internal/store appends events to, on the one
//     path a stranger controls, so every refusal the read can decide returns
//     before the write (SPEC-009).
//   - The INCREMENT re-states the door and the budget in its WHERE, so the
//     read above is only a fast path for those. Two joiners racing for the
//     last slot both reach the UPDATE; SQLite serialises them, the second
//     matches no row, and RowsAffected says so.
//
// It does NOT re-check the secret, which stays in Go where the comparison is
// constant-time; a RotateJoinSecret landing between the SELECT and the UPDATE
// lets one in-flight holder of the old secret through. Accepted (SPEC-009).
//
// A CreateInvite failure after this returns true BURNS the slot. Do not add a
// compensating decrement: a second write that can itself fail leaves the
// budget wrong in the more dangerous direction.
func (d *DB) JoinAdmits(candidate string) (bool, error) {
	var (
		secret                 string
		open, admitted, budget int
	)
	err := d.db.QueryRow(
		`SELECT secret, open, admitted, admit_limit FROM join_access WHERE id = 1`,
	).Scan(&secret, &open, &admitted, &budget)
	if errors.Is(err, sql.ErrNoRows) {
		// Never touched, so closed — and answered without creating anything.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("identity: read join access: %w", err)
	}
	match := subtle.ConstantTimeCompare([]byte(secret), []byte(candidate)) == 1
	if open != 1 || secret == "" || !match || admitted >= budget {
		return false, nil
	}

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
	// Zero means a concurrent joiner took the last slot between the read and
	// the update. Refused, and nothing was spent.
	return n == 1, nil
}

// RotateJoinSecret replaces the secret and returns the new one.
//
// This closes a leaked link to NEWCOMERS and touches nobody already through
// it: participants keep their own tokens.
func (d *DB) RotateJoinSecret() (string, error) {
	secret := newSecret()
	// Upsert, same reasoning as SetJoinOpen. The DO UPDATE branch must NOT
	// touch `open`: rotating says nothing about whether the door is open. The
	// INSERT branch writes 0, which is not an exception: no row already means
	// closed.
	//
	// admitted RESETS: a new secret is a NEW OPENING, and nobody holding it
	// has spent anything. Without this, rotating a leaked link after its
	// budget ran out hands the DM a door that reads OPEN and admits nobody.
	//
	// admit_limit is NOT touched, or rotating becomes a second way to set a
	// budget.
	if _, err := d.db.Exec(
		`INSERT INTO join_access (id, secret, open, admitted, admit_limit) VALUES (1, ?, 0, 0, 0)
		 ON CONFLICT(id) DO UPDATE SET secret = excluded.secret, admitted = 0`, secret,
	); err != nil {
		return "", fmt.Errorf("identity: rotate join secret: %w", err)
	}
	return secret, nil
}

// ensureJoinRow returns the secret, minting one if this campaign has never had
// it. Reads first and falls through to an atomic upsert, so two callers racing
// cannot leave two secrets live.
func (d *DB) ensureJoinRow() (string, error) {
	secret := newSecret()

	// READ FIRST. The upsert below is a genuine write even on the conflict
	// path, this file is shared with internal/store, which writes inside a
	// transaction on every event append, and the DM console polls this: an
	// unconditional upsert blocks for the full busy_timeout behind another
	// handle's write transaction and then fails SQLITE_BUSY, where a SELECT
	// answers at once.
	var stored string
	err := d.db.QueryRow(`SELECT secret FROM join_access WHERE id = 1`).Scan(&stored)
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("identity: read join secret: %w", err)
	}

	// No row yet: mint one. The upsert is atomic, so two callers racing here
	// cannot leave two secrets live: the loser's RETURNING gives the winner's
	// value. `SET secret = secret` is a no-op update whose only job is to make
	// RETURNING fire on the conflict path; INSERT OR IGNORE returns NO ROW
	// there.
	if err := d.db.QueryRow(
		`INSERT INTO join_access (id, secret, open) VALUES (1, ?, 0)
		 ON CONFLICT(id) DO UPDATE SET secret = secret
		 RETURNING secret`, secret,
	).Scan(&stored); err != nil {
		return "", fmt.Errorf("identity: mint join secret: %w", err)
	}
	return stored, nil
}

// newSecret mints 32 crypto/rand bytes, base64url: the same shape and strength
// as an invite token, because it guards the same kind of door.
//
// NO error return: crypto/rand.Read never returns one; it fills the buffer
// entirely or crashes the program.
func newSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// SetRole changes a participant's authorization level.
//
// Role lives in participants.role beside the token and never in the log: the
// fold has no Role, and putting it there would create a second place
// authorization lives (SPEC-009).
//
// It changes ONLY the role. The token, the id and the display name belong to
// the person and survive: a promotion that rewrote the credential would log
// them out. The characters they hold live in the log, which SetRole cannot
// reach.
//
// A revoked participant stays revoked. Promotion is not a way back in.
func (d *DB) SetRole(id string, role Role) error {
	if _, err := ParseRole(string(role)); err != nil {
		return err
	}
	res, err := d.db.Exec(`UPDATE participants SET role = ? WHERE id = ?`, string(role), id)
	if err != nil {
		return fmt.Errorf("identity: set role: %w", err)
	}
	// Reporting "no such participant" rather than succeeding silently: a DM
	// console that says it promoted somebody who has already left, and a
	// caller with no way to tell, is worse than an error.
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

// CreateInvite mints a new participant and a one-time invite token: 32
// random bytes (crypto/rand), base64url-encoded. The token is returned to
// the caller exactly ONCE; only its SHA-256 hash is persisted, so it can
// never be recovered from the database again.
//
// It takes NO list of actors: an invite says who you are and at what role,
// and control of a character is an ActorControlGranted in the log.
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

// Verify resolves token to its Participant. The SQL lookup is
// `WHERE token_hash = ?` on the SHA-256 hash, a plain indexed equality, and
// that is safe although SQLite's comparison is not constant-time: the hash is
// not secret in a timing-sensitive sense, since an attacker who can compute a
// matching hash already holds the token. The confirmation below still uses
// subtle.ConstantTimeCompare to make the contract explicit
// (TestVerifyUsesConstantTimeCompare holds it).
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

// Lookup resolves a participant by id, as they are NOW.
//
// This is the LIVE half of identity; Verify is the connection-time half. The
// gateway re-resolves through here before every command, before the backlog
// and each live event, and before the connect and departure announcements, so
// a promotion and a revocation take effect on the very next action without a
// reconnect (SPEC-009).
//
// A revoked participant does not resolve, and revoked and unknown share one
// error: the posture Verify takes, for the same reason.
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
		// Named and wrapped like every other failure here. Verify says
		// "stored role invalid" for the same row; a bare ParseRole error
		// would be the one Lookup path that told an operator neither what
		// went wrong nor whose row it was.
		return nil, fmt.Errorf("identity: lookup %s: stored role invalid: %w", id, err)
	}
	return &Participant{ID: id, Name: name, Role: role}, nil
}

// List returns everyone who can still act at this table, ordered by display
// name.
//
// It reads the one source of truth for roles rather than presence: presence
// is CONNECTION-scoped and a role is campaign-scoped, so a role must NOT be
// folded into a presence frame (SPEC-009).
//
// REVOKED PARTICIPANTS ARE OMITTED. They cannot connect and cannot act, so
// listing them would offer a DM promote controls for people who are gone.
//
// Ordered in SQL by display_name, then id, a total order, so two consumers
// cannot disagree about it.
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
			// Refused, not skipped and not defaulted. A stored role that is
			// not a role means this table's authorization data is wrong, and
			// answering with a shorter list would hide that while a console
			// quietly showed the wrong people.
			return nil, fmt.Errorf("identity: list %s: stored role invalid: %w", id, err)
		}
		out = append(out, &Participant{ID: id, Name: name, Role: role})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list participants: %w", err)
	}
	return out, nil
}

// Revoke permanently flips the revoked flag for participant id. This is a
// direct table mutation, not a logged event, and replaying the log does not
// undo it (SPEC-009).
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
