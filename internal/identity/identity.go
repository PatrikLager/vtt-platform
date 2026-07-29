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
