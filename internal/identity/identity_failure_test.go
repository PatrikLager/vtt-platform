package identity_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// This file pins identity's FAILURE behavior: what happens when the
// participants table has been tampered with, when the file is not a usable
// database, and when the handle is closed underneath a caller. These are the
// paths a coverage gate would otherwise leave dark on the one package every
// authorization decision ultimately rests on (gateway.Authorize resolves a
// role only via DB.Verify).
//
// The governing requirement is FAIL CLOSED: a corrupt or unusable store must
// surface an error, never a usable *Participant and never a panic. A silent
// success here would hand out a role.
//
// Deliberately NOT tested, because they are unreachable rather than untested
// (writing tests for them would raise the coverage number without pinning any
// behavior — see the enforcement plan's note on coverage theater):
//   - crypto/rand.Read failure in CreateInvite: crypto/rand does not fail on
//     any supported platform; Go 1.24 made it panic rather than return.
//   - the subtle.ConstantTimeCompare mismatch branch in Verify: the row was
//     selected BY that same hash (WHERE token_hash = ?), so a mismatch is
//     unreachable by construction. It is documented defense-in-depth
//     (identity.go:134-143) and TestVerifyUsesConstantTimeCompare already
//     pins that the comparison is performed.

// tamperRow opens an independent handle on the same file and overwrites one
// column of the single participant row, simulating a tampered or corrupted
// database. It mirrors TestTokenNotRecoverableFromDB's second-handle pattern.
func tamperRow(t *testing.T, path, column, value string) {
	t.Helper()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	// #nosec G202 -- column is a test-supplied literal, never external input.
	if _, err := raw.Exec(`UPDATE participants SET `+column+` = ?`, value); err != nil {
		t.Fatal(err)
	}
}

// A TestVerifyFailsClosedOnCorruptControls used to sit here, tampering with
// participants.controls and requiring Verify to refuse the row. The column was
// deleted on 2026-08-24 — it recorded control a second time and granted
// nothing — and with it went the only field Verify decoded rather than read.
// Its sibling below, on the stored role, carries the fail-closed property that
// remains reachable.

func TestVerifyFailsClosedOnInvalidStoredRole(t *testing.T) {
	d, path := openTemp(t)
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	// An unrecognized role must never resolve. If this ever returned a
	// participant, gateway.Authorize would look the role up in commandRoles,
	// miss, and deny — but the participant would still be authenticated.
	tamperRow(t, path, "role", "superuser")

	p, err := d.Verify(token)
	if err == nil {
		t.Fatalf("want error for invalid stored role, got participant %+v", p)
	}
	if p != nil {
		t.Errorf("want nil participant on error, got %+v", p)
	}
}

func TestVerifyRejectsEmptyStoredRole(t *testing.T) {
	d, path := openTemp(t)
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	tamperRow(t, path, "role", "")

	if _, err := d.Verify(token); err == nil {
		t.Fatal("want error for empty stored role")
	}
}

func TestOpenRejectsFileThatIsNotADatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-db")
	if err := os.WriteFile(path, []byte("this is not a SQLite file"), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := identity.Open(path)
	if err == nil {
		d.Close()
		t.Fatal("want error opening a non-database file")
	}
	if d != nil {
		t.Errorf("want nil DB on error, got %+v", d)
	}
}

func TestRevokeUnknownParticipantErrors(t *testing.T) {
	d, _ := openTemp(t)
	if err := d.Revoke("no-such-participant"); err == nil {
		t.Fatal("want error revoking an unknown participant")
	}
}

// TestOperationsFailAfterClose pins that every exported operation surfaces an
// error rather than panicking once the handle is closed — the shape a caller
// hits if identity is closed underneath a live connection, which is exactly
// the ledgered shutdown race composeServer's doc comment describes
// (cmd/vtt/serve_compose.go:31-54).
func TestOperationsFailAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("CreateInvite", func(t *testing.T) {
		if _, _, err := d.CreateInvite("Arel", identity.RoleDM); err == nil {
			t.Error("want error after Close")
		}
	})
	t.Run("Verify", func(t *testing.T) {
		if _, err := d.Verify(token); err == nil {
			t.Error("want error after Close")
		}
	})
	t.Run("Revoke", func(t *testing.T) {
		if err := d.Revoke(id); err == nil {
			t.Error("want error after Close")
		}
	})
}
