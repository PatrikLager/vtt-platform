package main

import (
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func qaInviteCLI(t *testing.T, dir, name, role string) (string, error) {
	t.Helper()
	return runCLI(t, "invite", "--campaign", dir, "--name", name, "--role", role)
}

func qaNames(t *testing.T, dir string) []string {
	t.Helper()
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ps, err := db.List()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

// VTT-543
func TestQAInviteRefusesAHeldNameAndCreatesNobody(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	if _, err := qaInviteCLI(t, dir, "Kim", "dm"); err != nil {
		t.Fatalf("first invite: %v", err)
	}
	for _, n := range []string{"Kim", "kim", "KIM"} {
		if out, err := qaInviteCLI(t, dir, n, "player"); err == nil {
			t.Errorf("invite %q succeeded, out=%q", n, out)
		}
	}
	if got := qaNames(t, dir); len(got) != 1 {
		t.Errorf("participants = %v, want just Kim", got)
	}
	if _, err := qaInviteCLI(t, dir, "Lou", "player"); err != nil {
		t.Errorf("free name refused: %v", err)
	}
	if got := qaNames(t, dir); len(got) != 2 {
		t.Errorf("participants = %v, want Kim and Lou", got)
	}
}

// VTT-544
func TestQAInviteReusesARevokedName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	out, err := qaInviteCLI(t, dir, "Kim", "player")
	if err != nil {
		t.Fatal(err)
	}
	id := extractField(t, out, "participant id: ")
	if _, err := qaInviteCLI(t, dir, "KIM", "player"); err == nil {
		t.Fatal("held name admitted before revoke")
	}
	if _, err := runCLI(t, "revoke", "--campaign", dir, "--id", id); err != nil {
		t.Fatal(err)
	}
	if _, err := qaInviteCLI(t, dir, "KIM", "player"); err != nil {
		t.Errorf("invite after revoke: %v", err)
	}
}
