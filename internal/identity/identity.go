// Package identity manages participants, invite tokens, and revocation for
// a campaign. It opens its OWN SQLite handle on the same campaign file the
// store uses (spec §5) and is deliberately NOT event-sourced: identity is
// infrastructure, not game history — revocation must not be undoable via
// game-log mechanics such as retraction.
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

// THERE IS NO `controls` COLUMN, and its absence is a decision (2026-08-24).
// It used to sit between role and token_hash, holding a JSON array of actor
// ids, and it GRANTED NOTHING: no updater ever existed, no code path turned it
// into an ActorControlGranted, and its only consumer echoed it back at
// /api/me. Control is a fact about the log — Actor.controller_ids, which
// gateway's authz.go controls() and eyes()'s player arm read to decide what a
// participant may do and see — so a second record of it here could only ever
// agree by luck, and did not: a DM who invited somebody "controlling Hollis"
// was told by /api/me that they did, while every rule that decides anything
// said they did not.
//
// Identity is deliberately not event-sourced (see the package comment), which
// is exactly why this belongs in the log rather than here: what a token makes
// you is infrastructure, what you hold at the table is history.
//
// NOTHING MIGRATES IT AWAY, and that is the second decision (2026-08-24). A
// migration that dropped the column from campaigns still carrying one shipped
// on this branch and was removed the same day: no campaign is in use by anyone,
// so it protected no data, and it charged every campaign still carrying the
// column one WRITABLE open — its shape check made migrationPending answer yes
// for every one of them, which takes migrate to BEGIN IMMEDIATE, which
// read-only media cannot give. A campaign that still carries the column opens
// (TestJoinIsClosedOnAnExistingCampaign's fixture carries it) and the column is
// inert, because no statement in this package names it.
const schema = `
CREATE TABLE IF NOT EXISTS participants (
  id           TEXT PRIMARY KEY,
  display_name TEXT,
  role         TEXT,
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
  id          INTEGER PRIMARY KEY CHECK (id = 1),
  secret      TEXT NOT NULL,
  open        INTEGER NOT NULL DEFAULT 0,
  -- The admission budget (spec §2, amended 2026-08-11). admitted counts what
  -- this OPENING has let through; admit_limit is what the DM allowed. Both
  -- reset when the door opens, so a budget is per-opening rather than
  -- per-campaign — a DM who opens the door twice means twice.
  --
  -- These are COLUMNS on an existing table, which is exactly what the comment
  -- above says this schema cannot deliver on its own. See migrate().
  admitted    INTEGER NOT NULL DEFAULT 0,
  admit_limit INTEGER NOT NULL DEFAULT 0
);`

// migrate adds columns the schema above cannot deliver on its own, and repairs
// an already-open door left without a budget.
//
// CREATE TABLE IF NOT EXISTS is a NO-OP on a table that already exists, so a
// new COLUMN never appears on a campaign that has one — which the join_access
// comment records as the reason that table exists at all. The admission budget
// (spec §2, amended 2026-08-11) has no such escape: it belongs on the same
// single row as `open`, because spending an admission has to be ONE conditional
// UPDATE against ONE row to be race-proof, and splitting it across two tables
// would make that a transaction spanning both.
//
// It is deliberately the smallest thing that works: add what is missing, repair
// what the addition would otherwise strand, touch nothing else. It runs on every
// Open and must stay idempotent — ALTER TABLE ADD COLUMN is an error, not a
// no-op, if the column is already there.
//
// IT TOUCHES ONLY join_access, and that is the whole of it again: the branch
// that dropped participants.controls was removed on 2026-08-24. See the schema
// comment for why that column is not worth a migration, and what the migration
// was charging for the privilege.
func migrate(db *sql.DB) error {
	// READ FIRST, and take no write lock at all when there is nothing to do —
	// the same discipline ensureJoinRow documents, for the same measured
	// reason. This runs on EVERY Open, and identity deliberately shares its
	// file with internal/store, which writes inside a transaction on every
	// event append. An unconditional BEGIN IMMEDIATE here made opening a
	// campaign take the write lock even when the schema was already current,
	// which is a lock a read-only user (`vtt state dump`, the DM console's
	// polling) has no business taking — and made an already-migrated campaign
	// on read-only media impossible to open at all.
	ctx := context.Background()
	pending, err := migrationPending(ctx, db)
	if err != nil {
		return err
	}
	if !pending {
		return nil
	}

	// ONE PINNED CONNECTION, inside BEGIN IMMEDIATE. The scan and the ALTERs
	// are separate statements, and two processes opening the same campaign at
	// the same instant both see the columns missing and both try to add them:
	// measured 35 failures in 40 trials with four concurrent opens, dying on
	// `duplicate column name`. The server refuses to start or the DM gets a raw
	// SQL error, on the first run after an upgrade and no other — the one run
	// where nobody will connect it to the cause.
	//
	// IMMEDIATE and not db.Begin(), which is DEFERRED: a deferred transaction
	// takes a read lock first and must UPGRADE it to write, and busy_timeout
	// does not retry a lock upgrade. Measured, that turns 35 duplicate-column
	// failures into 36 immediate SQLITE_BUSY failures — a different error for
	// the same reason. IMMEDIATE takes the write lock up front and the losers
	// wait on it properly: 0 failures in 40 trials. Each then re-reads the
	// shape inside the transaction and finds nothing left to do.
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
// disagree and mislabel the one message an operator gets.
//
// The pragma is a LITERAL rather than built from the name: SQLite will not
// accept a bind parameter in a PRAGMA, so building it would mean interpolating
// into SQL — a shape a reader has to re-check for injection every time, even
// when the input is a literal three lines up.
//
// ONE SHAPE since 2026-08-24: participantsShape went with the controls
// migration. Kept as a value rather than folded back into its callers because
// both of them still read this table and must name it identically.
type tableShape struct {
	pragma string
	name   string
}

var joinAccessShape = tableShape{`PRAGMA table_info(join_access)`, "join_access"}

// columnNames reads a table's column set: query it, drain it, close it.
//
// ONE implementation for TWO call sites — migrationPending, deciding whether to
// take the write lock, and migrateLocked, re-deciding under it. The query and
// its wrapped error are shared as well as the draining, so the two cannot name
// the table differently in the one message an operator gets.
//
// The rows handle is closed by DEFER in exactly one place. That matters beyond
// tidiness: migrateLocked must close it BEFORE its ALTER TABLE, since SQLite
// will not alter a table with an open cursor on it, and inline rows.Close()
// calls left unhandled errors that gosec flags — which `_ = rows.Close()` would
// silence rather than answer.
//
// r is *sql.DB when nothing is locked and *sql.Conn when the migration holds
// the write lock; the interface is what lets one function serve both, and it is
// deliberately the narrowest thing that does.
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
	// RE-READ inside the transaction. A concurrent opener may have completed
	// the whole migration while this one waited for the write lock, and ALTER
	// TABLE ADD COLUMN is an error, not a no-op, on a column already there.
	// WRAPPED "migrate:", which the read in migrationPending is not. Both read
	// the SAME shape with the same helper and the same message, so without this
	// an operator cannot tell which of the two failed.
	//
	// NOTHING HAS EVER TESTED THIS ARM, and nothing can through testdb: Arm is
	// one-shot and matches by substring, and migrationPending runs the identical
	// PRAGMA first, so the single armed fault is always spent before this call.
	// Measured, not assumed — this arm read `1 0` in the coverage profile at
	// HEAD on 2026-08-24 and before it. The only under-lock shape read a fault
	// ever reached was the participants one, which went with the controls
	// migration. Said here so it reads as a known hole rather than an
	// unexplained gap in a report. It also matches how every other failure
	// inside this transaction already reads (migrate's begin and commit arms).
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

	// A door this campaign's DM left OPEN keeps working, on a fresh budget. The
	// columns arrive defaulted to 0, and 0 admits nobody — so without this,
	// upgrading would silently shut a door that was open, and the only symptom
	// would be strangers being turned away from a link the DM had shared.
	//
	// KEYED ON THE STATE, not on which ALTER just ran. Nesting it under the
	// admit_limit branch made it depend on this function's own statement order:
	// a database carrying admit_limit but not admitted would open cleanly, keep
	// a budget of 0, and refuse every joiner at a door reading "open". The
	// predicate below is exactly "an open door with no budget" and cannot match
	// a legitimate row, because SetJoinOpen coerces every budget to at least 1.
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

// Open opens (creating if necessary) the participants table on the SQLite
// file at path. This is an independent handle from store.Open — both may
// be opened on the same campaign file concurrently.
// driverName is "sqlite" in production, and the only reason it is a variable
// is that internal/testdb needs to substitute a fault-injecting wrapper.
//
// A test-only seam, in the same spirit as gateway's encodeFrame field and the
// presence registry's sendBudget: this package's error arms decide what happens
// when a database fails MID-OPERATION — whether a refused join admits anyway,
// whether a half-applied migration hands back a usable handle — and a real
// SQLite file will not fail on demand. Closing a handle is not a substitute: it
// fails the FIRST statement, so everything past it stays unreached.
//
// Set only by fault_internal_test.go, and restored by it.
var driverName = "sqlite"

func Open(path string) (*DB, error) {
	// busy_timeout(5000): see internal/store/store.go's Open — same
	// intra-/cross-process SQLITE_BUSY hardening, same ledgered
	// carry-forward (P6 Task 4 review).
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

// SetJoinOpen opens or closes the door, and when opening, sets the admission
// budget for THIS opening.
//
// admitLimit is ignored when closing. Opening RESETS the count: a DM who opens
// the door twice means twice, because the second opening is a decision about a
// fresh set of people rather than the remainder of an old one. Carrying the
// count over would let a campaign run out of admissions permanently, curable
// only by editing the database.
//
// A non-positive admitLimit becomes DefaultAdmitLimit rather than "admit
// nobody". The wire cannot tell them apart — protojson omits zero values, so an
// absent field and a deliberate 0 arrive identically — and of the two readings,
// silently opening a door that admits no one is the one nobody can debug from
// the outside.
func (d *DB) SetJoinOpen(open bool, admitLimit int) error {
	v := 0
	if open {
		v = 1
	}
	if admitLimit <= 0 {
		admitLimit = DefaultAdmitLimit
	}
	// ONE upsert rather than ensure-then-update. Atomic, one round trip, and
	// it cannot leave the row half-made if the second statement fails — a
	// shape that also had an error branch no test could reach, because it
	// needed the database to work for the first call and fail for the second.
	secret := newSecret() // used only if the row does not exist yet
	// admitted resets to 0 on EVERY call, including a close: leaving a spent
	// count behind would make the next opening's budget arrive already used.
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
// Read-only, and it MINTS NOTHING — the DM console polls it, and JoinSecret's
// own comment records what happens when a poll takes SQLite's write lock on the
// file internal/store appends events to. Both zero on a campaign whose door has
// never been touched, which is the same thing it reports for a shut one: the
// question "how many may still come in" has the same answer either way.
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

// DefaultAdmitLimit is what a door opened without a stated budget allows.
//
// A number, not "unlimited", because protojson omits zero values: an absent
// field and "admit nobody" are the same wire bytes, so the default has to be
// chosen here rather than inferred from what arrived. Sized for a table —
// four to six players and a DM, with room for a couple of onlookers — on the
// reasoning that a DM who wants more will say so, while a DM who wants fewer
// has lost nothing they can measure.
const DefaultAdmitLimit = 8

// JoinAdmits reports whether this candidate may come through the door, AND
// spends one admission if so. Never writes on a refusal.
//
// ATOMICITY IS THE POINT, because a cap that two concurrent joiners can both
// pass is not a cap. The work is split deliberately:
//
//   - The SECRET is compared HERE, in Go, in constant time. Comparing it in
//     SQL would be neither, and the secret would leave this package.
//   - REFUSALS are decided here too, from the same read — wrong secret, shut
//     door, budget spent. That is what keeps a refused anonymous request from
//     writing ANYTHING, which spec §2 rests on: an UPDATE matching zero rows
//     still takes SQLite's write lock on the file internal/store appends
//     events to, inside a transaction, on the one path a stranger controls.
//   - The INCREMENT re-states the door and the budget in its WHERE, so the
//     read above is only a fast path for those. Two joiners racing for the
//     last slot both reach the UPDATE; SQLite serialises them, the second
//     matches no row, and RowsAffected says so.
//
// It does NOT re-check the secret — that stays in Go, where the comparison is
// constant-time. So a RotateJoinSecret landing between the SELECT and the
// UPDATE lets one in-flight request through on the old secret. A microsecond
// window on a deliberate, rare, DM-authorized action, and the joiner it admits
// is one who already held the real secret a moment earlier; closing it would
// mean comparing the secret in SQL, which trades a timing side channel for a
// race nobody can reach on purpose.
//
// A CreateInvite failure after this returns true BURNS the slot. Deliberate:
// the alternative is a compensating decrement, which is a second write that
// can itself fail and leaves the budget wrong in the more dangerous direction.
// Losing one admission to a database error costs a DM one re-open; leaking one
// costs the cap its meaning.
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
// it — participants keep their own tokens, which is the property that makes a
// leak survivable without re-inviting the table (spec §2).
func (d *DB) RotateJoinSecret() (string, error) {
	secret := newSecret()
	// Upsert, same reasoning as SetJoinOpen. The DO UPDATE branch does NOT
	// touch `open` — rotating closes the link to newcomers holding the OLD
	// secret and says nothing about whether the door is open. The INSERT
	// branch writes 0, which is not an exception to that: reaching it means no
	// row existed, and no row already means closed.
	// admitted RESETS. A new secret is a NEW OPENING: nobody holding it has
	// spent anything, and the people who spent the old budget came in on a link
	// that no longer works. Without this, rotating a leaked link after its
	// budget ran out hands the DM a door that reads OPEN and admits nobody —
	// every legitimate player gets the byte-identical stranger's 403, and
	// nothing on either end says why. Rotating is what §2 tells a DM to do
	// about a leak, so the cure was worse than the leak.
	//
	// admit_limit is NOT touched, for the same reason `open` is not: rotating
	// answers "which secret", and saying anything about how many people may
	// come through would make it a second way to set a budget.
	if _, err := d.db.Exec(
		`INSERT INTO join_access (id, secret, open, admitted, admit_limit) VALUES (1, ?, 0, 0, 0)
		 ON CONFLICT(id) DO UPDATE SET secret = excluded.secret, admitted = 0`, secret,
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
// It changes ONLY the role. The token, the id and the display name belong to
// the person and survive: a promotion that rewrote the credential would log
// them out. The characters they hold survive too, and no longer because this
// statement is careful — they live in the log, which SetRole cannot reach.
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
//
// It takes NO list of actors (2026-08-24). An invite says who you are and at
// what role; it does not hand you a character, because the only thing that
// does is an ActorControlGranted in the log. The parameter it used to take was
// recorded and never read by anything that decides.
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
