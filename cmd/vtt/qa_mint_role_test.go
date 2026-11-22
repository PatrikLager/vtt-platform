package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/harness"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func qaMintDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "camp")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func qaMintScenario(ps ...harness.Participant) *harness.Scenario {
	return &harness.Scenario{Name: "qa", Participants: ps}
}

// VTT-942
func TestQAMintEachRoleAtExactlyThatRole(t *testing.T) {
	dir := qaMintDir(t)
	sc := qaMintScenario(
		harness.Participant{Name: "boss", Role: "dm"},
		harness.Participant{Name: "bot", Role: "agent"},
		harness.Participant{Name: "eye", Role: "spectator"},
	)
	tokens, ids, err := mintInvites(dir, sc)
	if err != nil {
		t.Fatalf("mintInvites: %v", err)
	}
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if len(tokens) != 3 || len(ids) != 3 {
		t.Fatalf("tokens=%d ids=%d, want 3 each", len(tokens), len(ids))
	}
	for _, p := range sc.Participants {
		tok, ok := tokens[p.Name]
		if !ok {
			t.Fatalf("no token for %s", p.Name)
		}
		got, err := db.Verify(tok)
		if err != nil {
			t.Fatalf("verify %s: %v", p.Name, err)
		}
		if string(got.Role) != p.Role {
			t.Errorf("%s minted as %q, declared %q", p.Name, got.Role, p.Role)
		}
		if got.ID != ids[p.Name] {
			t.Errorf("%s verify id %q != returned %q", p.Name, got.ID, ids[p.Name])
		}
	}
}

// VTT-942
func TestQAMintPlayerRefusedWithCreateInvitesText(t *testing.T) {
	dir := qaMintDir(t)
	refDir := qaMintDir(t)
	ref, err := identity.Open(campaign.LogPath(refDir))
	if err != nil {
		t.Fatal(err)
	}
	_, _, want := ref.CreateInvite("x", identity.Role("player"))
	ref.Close()
	if want == nil {
		t.Fatal("CreateInvite minted a player")
	}
	sc := qaMintScenario(
		harness.Participant{Name: "boss", Role: "dm"},
		harness.Participant{Name: "zed", Role: "player"},
	)
	_, _, err = mintInvites(dir, sc)
	if err == nil {
		t.Fatal("mintInvites minted a player")
	}
	if !strings.Contains(err.Error(), want.Error()) {
		t.Errorf("error %q lacks CreateInvite's refusal %q", err, want)
	}
}

// VTT-942
func TestQAMintPlayerFirstMintsNothingAsSpectator(t *testing.T) {
	dir := qaMintDir(t)
	sc := qaMintScenario(harness.Participant{Name: "zed", Role: "player"})
	tokens, _, err := mintInvites(dir, sc)
	if err == nil {
		t.Fatalf("minted: %v", tokens)
	}
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, _, err := db.CreateInvite("zed", identity.RoleSpectator); err != nil {
		t.Errorf("zed was minted despite refusal: %v", err)
	}
}

// VTT-942
func TestQAMintUnknownRoleIsNotRewritten(t *testing.T) {
	dir := qaMintDir(t)
	sc := qaMintScenario(harness.Participant{Name: "zed", Role: "wizard"})
	if _, _, err := mintInvites(dir, sc); err == nil {
		t.Fatal("minted a participant at an undeclared-for role")
	}
}
