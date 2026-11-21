package identity_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const qaMintPlayerRefusal = "identity: a participant becomes a player " +
	"when given a character; invite them as a spectator"

func qaMintOpen(t *testing.T) (*identity.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "log.db")
	db, err := identity.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, path
}

func qaMintNames(t *testing.T, db *identity.DB) []string {
	t.Helper()
	ps, err := db.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	names := make([]string, 0, len(ps))
	for _, p := range ps {
		names = append(names, p.Name+"/"+string(p.Role)+"/"+p.ID)
	}
	return names
}

func qaMintSame(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// VTT-911
func TestQAMintMintsDMAgentAndSpectator(t *testing.T) {
	db, _ := qaMintOpen(t)
	cases := []struct {
		name string
		role identity.Role
	}{
		{"Ada", identity.RoleDM},
		{"Bix", identity.RoleAgent},
		{"Cato", identity.RoleSpectator},
	}
	for _, c := range cases {
		token, id, err := db.CreateInvite(c.name, c.role)
		if err != nil {
			t.Fatalf("CreateInvite(%q, %q): %v", c.name, c.role, err)
		}
		if token == "" || id == "" {
			t.Fatalf("CreateInvite(%q, %q) = token %q id %q",
				c.name, c.role, token, id)
		}
		p, err := db.Verify(token)
		if err != nil {
			t.Fatalf("Verify after %q: %v", c.role, err)
		}
		if p.ID != id || p.Name != c.name || p.Role != c.role {
			t.Fatalf("Verify = %+v, want id %q name %q role %q",
				p, id, c.name, c.role)
		}
		l, err := db.Lookup(id)
		if err != nil {
			t.Fatalf("Lookup after %q: %v", c.role, err)
		}
		if l.ID != id || l.Name != c.name || l.Role != c.role {
			t.Fatalf("Lookup = %+v, want id %q name %q role %q",
				l, id, c.name, c.role)
		}
	}
	got := qaMintNames(t, db)
	if len(got) != len(cases) {
		t.Fatalf("List after three mints = %v, want three rows", got)
	}
}

// VTT-911
func TestQAMintRefusesPlayerInItsOwnWords(t *testing.T) {
	db, _ := qaMintOpen(t)
	if _, _, err := db.CreateInvite("Ada", identity.RoleDM); err != nil {
		t.Fatalf("seed dm: %v", err)
	}
	before := qaMintNames(t, db)
	token, id, err := db.CreateInvite("Pell", identity.RolePlayer)
	if err == nil {
		t.Fatalf("CreateInvite(player) minted token %q id %q", token, id)
	}
	if err.Error() != qaMintPlayerRefusal {
		t.Fatalf("player refusal = %q, want %q", err.Error(),
			qaMintPlayerRefusal)
	}
	if token != "" || id != "" {
		t.Fatalf("refused player returned token %q id %q", token, id)
	}
	after := qaMintNames(t, db)
	if !qaMintSame(before, after) {
		t.Fatalf("List changed across a refusal: before %v after %v",
			before, after)
	}
	_, sid, err := db.CreateInvite("Pell", identity.RoleSpectator)
	if err != nil {
		t.Fatalf("the refused name is not free afterwards: %v", err)
	}
	p, err := db.Lookup(sid)
	if err != nil || p.Role != identity.RoleSpectator {
		t.Fatalf("Lookup of the spectator = %+v, %v", p, err)
	}
}

// VTT-911
func TestQAMintRefusesEveryOtherRole(t *testing.T) {
	db, _ := qaMintOpen(t)
	if _, _, err := db.CreateInvite("Ada", identity.RoleDM); err != nil {
		t.Fatalf("seed dm: %v", err)
	}
	others := []identity.Role{
		"", "Player", "PLAYER", "gm", "owner", "DM", "Agent",
		"Spectator", " player", "player ", "spectator ", "dm\n",
		"players",
	}
	for _, r := range others {
		before := qaMintNames(t, db)
		token, id, err := db.CreateInvite("Rho", r)
		if err == nil {
			t.Fatalf("CreateInvite(role %q) minted token %q id %q",
				r, token, id)
		}
		if token != "" || id != "" {
			t.Fatalf("refused role %q returned token %q id %q",
				r, token, id)
		}
		after := qaMintNames(t, db)
		if !qaMintSame(before, after) {
			t.Fatalf("List changed across refusal of %q: %v -> %v",
				r, before, after)
		}
	}
}

// VTT-912
func TestQAMintParseRoleAcceptsExactlyFour(t *testing.T) {
	want := map[string]identity.Role{
		"dm":        identity.RoleDM,
		"agent":     identity.RoleAgent,
		"player":    identity.RolePlayer,
		"spectator": identity.RoleSpectator,
	}
	for s, r := range want {
		got, err := identity.ParseRole(s)
		if err != nil {
			t.Fatalf("ParseRole(%q): %v", s, err)
		}
		if got != r {
			t.Fatalf("ParseRole(%q) = %q, want %q", s, got, r)
		}
	}
	refused := []string{
		"", "DM", "Dm", "Agent", "Player", "PLAYER", "Spectator",
		"gm", "owner", " dm", "dm ", "player\n", "players",
		"spectators", "agent\x00", "d m",
	}
	for _, s := range refused {
		if got, err := identity.ParseRole(s); err == nil {
			t.Fatalf("ParseRole(%q) = %q, want an error", s, got)
		}
	}
}

// VTT-912
func TestQAMintOldStoredPlayerReadsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.db")
	db, err := identity.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	token, id, err := db.CreateInvite("Olm", identity.RoleSpectator)
	if err != nil {
		t.Fatalf("CreateInvite(spectator): %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	res, err := raw.Exec(
		"UPDATE participants SET role = 'player' WHERE id = ?", id)
	if err != nil {
		t.Fatalf("UPDATE: %v", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("UPDATE touched %d rows, want 1", n)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw Close: %v", err)
	}

	db, err = identity.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	l, err := db.Lookup(id)
	if err != nil {
		t.Fatalf("Lookup of a stored player: %v", err)
	}
	if l.Role != identity.RolePlayer || l.Name != "Olm" || l.ID != id {
		t.Fatalf("Lookup = %+v, want a player named Olm", l)
	}
	v, err := db.Verify(token)
	if err != nil {
		t.Fatalf("Verify of a stored player: %v", err)
	}
	if v.Role != identity.RolePlayer || v.ID != id {
		t.Fatalf("Verify = %+v, want player %q", v, id)
	}
	ps, err := db.List()
	if err != nil {
		t.Fatalf("List with a stored player: %v", err)
	}
	if len(ps) != 1 || ps[0].Role != identity.RolePlayer {
		t.Fatalf("List = %+v, want the one stored player", ps)
	}
}
