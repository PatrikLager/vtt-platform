package identity_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Do not add tests for CreateInvite's rand.Read failure (it never returns one)
// or Verify's hash mismatch branch (the row is selected BY that hash): neither
// is reachable. Everything here must refuse: a silent success hands out a role.

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

// VTT-069
func TestVerifyFailsClosedOnInvalidStoredRole(t *testing.T) {
	d, path := openTemp(t)
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	// Refuse, do not merely deny: a returned participant would be authenticated
	// even though gateway.Authorize misses its role.
	tamperRow(t, path, "role", "superuser")

	p, err := d.Verify(token)
	if err == nil {
		t.Fatalf("want error for invalid stored role, got participant %+v", p)
	}
	if p != nil {
		t.Errorf("want nil participant on error, got %+v", p)
	}
}

// VTT-069
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

// VTT-071
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

// VTT-067
func TestRevokeUnknownParticipantErrors(t *testing.T) {
	d, _ := openTemp(t)
	if err := d.Revoke("no-such-participant"); err == nil {
		t.Fatal("want error revoking an unknown participant")
	}
}

// Closes the handle first: the shape composeServer's shutdown race leaves.
// VTT-070
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
