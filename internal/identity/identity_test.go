package identity_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func openTemp(t *testing.T) (*identity.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")
	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d, path
}

func TestCreateInviteVerifyRoundTrip(t *testing.T) {
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer, []string{"act-lera"})
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("want non-empty token")
	}
	if id == "" {
		t.Fatal("want non-empty id")
	}

	p, err := d.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != id {
		t.Errorf("ID: got %q, want %q", p.ID, id)
	}
	if p.Name != "Lera" {
		t.Errorf("Name: got %q, want %q", p.Name, "Lera")
	}
	if p.Role != identity.RolePlayer {
		t.Errorf("Role: got %q, want %q", p.Role, identity.RolePlayer)
	}
	if len(p.Controls) != 1 || p.Controls[0] != "act-lera" {
		t.Errorf("Controls: got %v, want [act-lera]", p.Controls)
	}
}

// TestTokenNotRecoverableFromDB proves the raw token is not stored anywhere
// retrievable: it opens a second, independent SQLite handle on the same
// file and reads the persisted token_hash directly, asserting it neither
// equals the raw token bytes nor anything other than sha256(token).
func TestTokenNotRecoverableFromDB(t *testing.T) {
	d, path := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()

	var stored []byte
	if err := raw.QueryRow(`SELECT token_hash FROM participants WHERE id = ?`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(stored, []byte(token)) {
		t.Fatal("stored token_hash equals the raw token bytes — token is recoverable")
	}
	want := sha256.Sum256([]byte(token))
	if !bytes.Equal(stored, want[:]) {
		t.Fatalf("stored token_hash != sha256(token): got %x want %x", stored, want)
	}
}

func TestVerifyRejectsWrongToken(t *testing.T) {
	d, _ := openTemp(t)
	if _, _, err := d.CreateInvite("Lera", identity.RolePlayer, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify("this-is-not-a-real-token"); err == nil {
		t.Fatal("want error for a token that was never issued")
	}
}

func TestRevokedTokenRejectedAfterRevoke(t *testing.T) {
	d, _ := openTemp(t)
	token, id, err := d.CreateInvite("Lera", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err != nil {
		t.Fatalf("Verify before revoke: %v", err)
	}
	if err := d.Revoke(id); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err == nil {
		t.Fatal("want error for a revoked token")
	}
}

func TestParseRoleAcceptsExactlyTheFourRoles(t *testing.T) {
	cases := []struct {
		in   string
		want identity.Role
	}{
		{"dm", identity.RoleDM},
		{"agent", identity.RoleAgent},
		{"player", identity.RolePlayer},
		{"spectator", identity.RoleSpectator},
	}
	for _, c := range cases {
		got, err := identity.ParseRole(c.in)
		if err != nil {
			t.Errorf("ParseRole(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseRole(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	for _, bad := range []string{"", "DM", "Player", "admin", "gm", "npc"} {
		if _, err := identity.ParseRole(bad); err == nil {
			t.Errorf("ParseRole(%q): want error, got nil", bad)
		}
	}
}

func TestTwoInvitesProduceDistinctTokensAndIDs(t *testing.T) {
	d, _ := openTemp(t)
	token1, id1, err := d.CreateInvite("Lera", identity.RolePlayer, nil)
	if err != nil {
		t.Fatal(err)
	}
	token2, id2, err := d.CreateInvite("Ursus", identity.RoleAgent, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token1 == token2 {
		t.Fatal("want distinct tokens for two invites")
	}
	if id1 == id2 {
		t.Fatal("want distinct ids for two invites")
	}
}

// TestVerifyUsesConstantTimeCompare is the white-box half of the Step 1
// "constant-time-shaped" case (task-3-brief.md, binding DESIGN NOTE): the
// hash lookup itself (SELECT ... WHERE token_hash = ?) is fine as a plain
// indexed comparison because the hash is not secret-timing-sensitive, but
// the final confirmation of a match must be constant-time so no timing
// side channel on token bytes exists. That correctness is exercised
// behaviorally above (round-trip, wrong-token, revoked-token); this test
// enforces the documented implementation contract itself.
func TestVerifyUsesConstantTimeCompare(t *testing.T) {
	src, err := os.ReadFile("identity.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "subtle.ConstantTimeCompare") {
		t.Fatal("Verify must confirm the token hash match with subtle.ConstantTimeCompare")
	}
}

// TestCoexistsWithStoreOnSameFile proves identity opens its own SQLite
// handle independent of store.Store and that both handles operate
// correctly against the same campaign file (spec: identity is deliberately
// NOT event-sourced and lives beside, not inside, the event log).
func TestCoexistsWithStoreOnSameFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")

	d, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Exercise the identity handle.
	token, _, err := d.CreateInvite("Lera", identity.RolePlayer, []string{"act-lera"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Verify(token); err != nil {
		t.Fatal(err)
	}

	// Exercise the store handle on the SAME file.
	env := &vttv1.Envelope{
		EventId:   "e1",
		SessionId: "sess-1",
		ActorRole: "dm",
		Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "test"},
		},
	}
	if _, err := s.Append(env); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ReadAfter: got %d events, want 1", len(got))
	}

	// And once more, prove identity is still independently usable.
	if p, err := d.Verify(token); err != nil || p.Name != "Lera" {
		t.Fatalf("Verify after store use: p=%v err=%v", p, err)
	}
}
