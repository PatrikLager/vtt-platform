// Package identity manages participants, invite tokens, and revocation for
// a campaign. It opens its OWN SQLite handle on the same campaign file the
// store uses (spec §5) and is deliberately NOT event-sourced: identity is
// infrastructure, not game history — revocation must not be undoable via
// game-log mechanics such as retraction.
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS participants (
  id           TEXT PRIMARY KEY,
  display_name TEXT,
  role         TEXT,
  controls     TEXT, -- JSON array
  token_hash   BLOB UNIQUE,
  revoked      INTEGER DEFAULT 0
);

-- The shared join door (joining-a-table spec §2, §4). A SEPARATE TABLE rather
-- than columns on participants, and that is the whole migration story: Open()
-- applies this schema with CREATE TABLE IF NOT EXISTS, so on a campaign that
-- already has a participants table that statement is a NO-OP and a new COLUMN
-- there would never appear. A new TABLE does not exist yet on any database, so
-- IF NOT EXISTS creates it — correctly, on fresh and existing campaigns alike.
--
-- id = 1 and the CHECK make it a single row by construction, so there is no
-- "which row is the real one" question to get wrong later.
--
-- The security property — a campaign comes up CLOSED, including one that
-- predates this feature — is carried by the INSERT in ensureJoinRow, which
-- writes open=0 EXPLICITLY. This DEFAULT 0 is belt-and-braces for a row
-- inserted by some other path, and injection proves the difference: flipping
-- the DEFAULT fails nothing, flipping the INSERT fails two tests. Worth
-- stating, because the obvious reading is that the DEFAULT is the guard:
-- flipping DEFAULT 0 to 1 fails NO test, while flipping ensureJoinRow's
-- inserted literal fails TestReadingTheLinkDoesNotOpenTheDoor. Naming the test
-- makes the claim checkable rather than a number to be trusted.
CREATE TABLE IF NOT EXISTS join_access (
  id     INTEGER PRIMARY KEY CHECK (id = 1),
  secret TEXT NOT NULL,
  open   INTEGER NOT NULL DEFAULT 0
);`

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

// Participant is a resolved identity: who this token belongs to, at what
// role, and which actors (if any) they control.
type Participant struct {
	ID       string
	Name     string
	Role     Role
	Controls []string
}

// ErrInvalidToken is returned by Verify for a token that is unknown,
// malformed, or revoked. It deliberately does not distinguish between the
// three so callers cannot use error content to probe token validity.
var ErrInvalidToken = errors.New("identity: invalid or revoked token")

// DB is a handle on a campaign's participants table.
type DB struct {
	db *sql.DB
}

// Open opens (creating if necessary) the participants table on the SQLite
// file at path. This is an independent handle from store.Open — both may
// be opened on the same campaign file concurrently.
func Open(path string) (*DB, error) {
	// busy_timeout(5000): see internal/store/store.go's Open — same
	// intra-/cross-process SQLITE_BUSY hardening, same ledgered
	// carry-forward (P6 Task 4 review).
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("identity: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close() // closing a handle whose schema init just failed
		return nil, fmt.Errorf("identity: init schema: %w", err)
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

// SetJoinOpen opens or closes the door.
func (d *DB) SetJoinOpen(open bool) error {
	v := 0
	if open {
		v = 1
	}
	// ONE upsert rather than ensure-then-update. Atomic, one round trip, and
	// it cannot leave the row half-made if the second statement fails — a
	// shape that also had an error branch no test could reach, because it
	// needed the database to work for the first call and fail for the second.
	secret := newSecret() // used only if the row does not exist yet
	if _, err := d.db.Exec(
		`INSERT INTO join_access (id, secret, open) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET open = excluded.open`, secret, v,
	); err != nil {
		return fmt.Errorf("identity: set join open: %w", err)
	}
	return nil
}

// JoinSecret returns the current join secret, minting one on first use.
//
// STABLE until rotated: the DM shares it, so a value that changed per call
// would invalidate the link the moment anyone looked at it.
func (d *DB) JoinSecret() (string, error) {
	return d.ensureJoinRow()
}

// JoinAllows reports whether candidate opens the door: it must be OPEN and the
// secret must match. IT NEVER WRITES, and that is a security property rather
// than a tidiness one.
//
// This is the only call in this package an UNAUTHENTICATED stranger can drive
// (the join endpoint, spec §5), and spec §2 rests its whole case against rate
// limiting on a closed door leaving nothing to hammer. Answering through
// JoinSecret does not leave it inert: that mints the row on a campaign which
// has never had one, so a REFUSED anonymous request took SQLite's write lock
// on the file internal/store writes to inside a transaction on every event
// append — the exact hazard ensureJoinRow's read-first path was restructured
// to avoid for the DM console, reintroduced on the one path a stranger drives.
//
// Both halves come from ONE query and the compare runs unconditionally, so a
// closed door and a wrong secret cost the same work as well as returning the
// same answer. The && below short-circuits on an already-computed bool, so it
// cannot reintroduce a timing difference between the two.
//
// The comparison lives here, not in the gateway, so the secret never leaves
// this package to be checked. An empty stored secret admits NOBODY:
// ConstantTimeCompare("", "") returns 1 and a request body omitting the field
// decodes to "", so the degenerate row would otherwise admit the world.
func (d *DB) JoinAllows(candidate string) (bool, error) {
	var (
		secret string
		open   int
	)
	err := d.db.QueryRow(`SELECT secret, open FROM join_access WHERE id = 1`).Scan(&secret, &open)
	if errors.Is(err, sql.ErrNoRows) {
		// Never touched, so closed — and answered without creating anything.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("identity: read join access: %w", err)
	}
	match := subtle.ConstantTimeCompare([]byte(secret), []byte(candidate)) == 1
	return open == 1 && secret != "" && match, nil
}

// RotateJoinSecret replaces the secret and returns the new one.
//
// This closes a leaked link to NEWCOMERS and touches nobody already through
// it — participants keep their own tokens, which is the property that makes a
// leak survivable without re-inviting the table (spec §2).
func (d *DB) RotateJoinSecret() (string, error) {
	secret := newSecret()
	// Upsert, same reasoning as SetJoinOpen. The DO UPDATE branch does NOT
	// touch `open` — rotating closes the link to newcomers holding the OLD
	// secret and says nothing about whether the door is open. The INSERT
	// branch writes 0, which is not an exception to that: reaching it means no
	// row existed, and no row already means closed.
	if _, err := d.db.Exec(
		`INSERT INTO join_access (id, secret, open) VALUES (1, ?, 0)
		 ON CONFLICT(id) DO UPDATE SET secret = excluded.secret`, secret,
	); err != nil {
		return "", fmt.Errorf("identity: rotate join secret: %w", err)
	}
	return secret, nil
}

// ensureJoinRow returns the secret, minting one if this campaign has never had
// it. Reads first (see below) and falls through to an atomic upsert, so two
// callers racing cannot leave two secrets live.
func (d *DB) ensureJoinRow() (string, error) {
	secret := newSecret()

	// READ FIRST. The upsert below is a genuine write even on the conflict
	// path, so using it unconditionally made every "show me the link" take
	// SQLite's write lock — and identity deliberately shares its file with
	// internal/store, which writes inside a transaction on every event append.
	// Measured by review: with another handle holding a write txn, this
	// blocked for the full busy_timeout(5000) and then failed SQLITE_BUSY,
	// while a plain SELECT answered immediately. The DM console polls this.
	var stored string
	err := d.db.QueryRow(`SELECT secret FROM join_access WHERE id = 1`).Scan(&stored)
	if err == nil {
		return stored, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("identity: read join secret: %w", err)
	}

	// No row yet: mint one. The upsert is atomic, so two callers racing here
	// cannot leave two secrets live — the loser's RETURNING gives the winner's
	// value. `SET secret = secret` is a no-op update whose only job is to make
	// RETURNING fire on the conflict path; INSERT OR IGNORE would return NO
	// ROW there, which is the trap this avoids.
	if err := d.db.QueryRow(
		`INSERT INTO join_access (id, secret, open) VALUES (1, ?, 0)
		 ON CONFLICT(id) DO UPDATE SET secret = secret
		 RETURNING secret`, secret,
	).Scan(&stored); err != nil {
		return "", fmt.Errorf("identity: mint join secret: %w", err)
	}
	return stored, nil
}

// newSecret mints 32 crypto/rand bytes, base64url — the same shape and
// strength as an invite token, because it guards the same kind of door.
//
// NO error return, deliberately. crypto/rand.Read never returns one; it fills
// the buffer entirely or crashes the program. Carrying an error here meant
// three propagation branches no test could ever reach, which the coverage
// ratchet correctly refused to accept as tested. (CreateInvite above still
// carries the older shape; changing it is not this task's business.)
func newSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// SetRole changes a participant's authorization level.
//
// ONE SOURCE OF TRUTH, deliberately (joining-a-table spec §3.1). Role stays in
// participants.role beside the token rather than becoming an event: the fold
// contains no reference to Role at all, so putting it in the log would drag an
// identity concern into engine.State AND create a second place authorization
// lives. What a second source of truth costs is on the record — controller_id
// mirroring controller_ids needed an invariant, fault-injection proof on both
// folds and a golden scenario before it could be trusted.
//
// It changes ONLY the role. The token, the id, the display name and the
// controls belong to the person and survive: a promotion that rewrote the
// credential would log them out, and one that cleared controls would strip the
// characters they hold.
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
// the caller exactly ONCE — only its SHA-256 hash is persisted, so it can
// never be recovered from the database again.
func (d *DB) CreateInvite(name string, role Role, controls []string) (token string, id string, err error) {
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

	if controls == nil {
		controls = []string{}
	}
	controlsJSON, err := json.Marshal(controls)
	if err != nil {
		return "", "", fmt.Errorf("identity: marshal controls: %w", err)
	}

	if _, err := d.db.Exec(
		`INSERT INTO participants (id, display_name, role, controls, token_hash, revoked) VALUES (?, ?, ?, ?, ?, 0)`,
		id, name, string(role), string(controlsJSON), hash[:],
	); err != nil {
		return "", "", fmt.Errorf("identity: insert participant: %w", err)
	}
	return token, id, nil
}

// Verify resolves token to its Participant. DESIGN NOTE (task-3-brief.md
// Step 2, binding): the SQL lookup is `WHERE token_hash = ?` on the SHA-256
// hash — a plain indexed equality comparison. This is safe even though
// SQLite's comparison is not constant-time, because the hash itself is not
// secret in a timing-sensitive sense (an attacker who can compute a
// matching hash has already broken the token; hash-equality timing leaks
// nothing about the token bytes beyond hash equality, which is the query's
// own point). The confirmation step below still wraps the comparison in
// subtle.ConstantTimeCompare for defense in depth and to make the contract
// explicit, rather than relying solely on the database's semantics.
func (d *DB) Verify(token string) (*Participant, error) {
	sum := sha256.Sum256([]byte(token))

	row := d.db.QueryRow(
		`SELECT id, display_name, role, controls, token_hash, revoked FROM participants WHERE token_hash = ?`,
		sum[:],
	)
	var (
		id, name, roleStr, controlsJSON string
		storedHash                      []byte
		revoked                         int
	)
	if err := row.Scan(&id, &name, &roleStr, &controlsJSON, &storedHash, &revoked); err != nil {
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

	var controls []string
	if err := json.Unmarshal([]byte(controlsJSON), &controls); err != nil {
		return nil, fmt.Errorf("identity: unmarshal controls: %w", err)
	}
	role, err := ParseRole(roleStr)
	if err != nil {
		return nil, fmt.Errorf("identity: stored role invalid: %w", err)
	}

	return &Participant{ID: id, Name: name, Role: role, Controls: controls}, nil
}

// Lookup resolves a participant by id, as they are NOW.
//
// This is the LIVE half of identity; Verify is the connection-time half.
// Authentication happens once — you present a token and we learn who you are.
// Authorization is a live fact: what you may do can change while you are
// connected, so the gateway re-resolves through here on every command instead
// of trusting the answer it got at connect (joining-a-table spec §3.2).
//
// Two things that used to require a reconnect now take effect on the very next
// action: a promotion, and a REVOCATION. The second was a real hole — the only
// Verify in the WS path ran at connect, so a revoked participant kept playing
// until they chose to disconnect, and throwing someone out of a table did
// nothing without their cooperation.
//
// A revoked participant does not resolve, and revoked and unknown share one
// error — the same posture Verify already takes, for the same reason.
//
// MEASURED before adopting, because the objection to doing this per command
// was cost: 15.5µs against a table of 40 participants, on a path that already
// folds state, appends to SQLite and writes a socket frame. Microseconds
// against milliseconds.
func (d *DB) Lookup(id string) (*Participant, error) {
	var (
		name, roleStr, controlsJSON string
		revoked                     int
	)
	err := d.db.QueryRow(
		`SELECT display_name, role, controls, revoked FROM participants WHERE id = ?`, id,
	).Scan(&name, &roleStr, &controlsJSON, &revoked)
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
	var controls []string
	if err := json.Unmarshal([]byte(controlsJSON), &controls); err != nil {
		return nil, fmt.Errorf("identity: lookup %s: controls: %w", id, err)
	}
	return &Participant{ID: id, Name: name, Role: role, Controls: controls}, nil
}

// List returns everyone who can still act at this table, ordered by display
// name.
//
// It exists because the DM console has to answer "who is a spectator, and can
// I promote them?" and presence cannot answer it. Presence frames carry a
// display name and a connection state, deliberately: presence is
// CONNECTION-scoped, a role is campaign-scoped, and a role folded into a
// presence frame would go stale the moment somebody was promoted without
// reconnecting — exactly what live re-resolution made possible (spec §3.2).
// So this reads the one source of truth for roles (spec §3.1) instead.
//
// REVOKED PARTICIPANTS ARE OMITTED. They cannot connect and cannot act, so
// listing them would offer a DM promote controls for people who are gone, and
// would make a revoked name look like somebody still at the table.
//
// Ordered by name in SQL rather than by the caller, so two consumers cannot
// disagree about it and a console does not reshuffle under the DM's cursor
// between renders. Ties break on id, which is unique, so the order is total.
func (d *DB) List() ([]*Participant, error) {
	rows, err := d.db.Query(
		`SELECT id, display_name, role, controls FROM participants
		 WHERE revoked = 0 ORDER BY display_name, id`)
	if err != nil {
		return nil, fmt.Errorf("identity: list participants: %w", err)
	}
	defer rows.Close()

	var out []*Participant
	for rows.Next() {
		var id, name, roleStr, controlsJSON string
		if err := rows.Scan(&id, &name, &roleStr, &controlsJSON); err != nil {
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
		var controls []string
		if err := json.Unmarshal([]byte(controlsJSON), &controls); err != nil {
			return nil, fmt.Errorf("identity: list %s: controls: %w", id, err)
		}
		out = append(out, &Participant{ID: id, Name: name, Role: role, Controls: controls})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("identity: list participants: %w", err)
	}
	return out, nil
}

// Revoke permanently flips the revoked flag for participant id. This is a
// direct table mutation, not a logged event — it cannot be undone by any
// game-log/retraction mechanism (spec §5).
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
