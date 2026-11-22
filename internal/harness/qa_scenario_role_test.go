package harness_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

type qaRoleParticipant struct{ name, role string }

func qaWriteRoleScenario(t *testing.T, ps []qaRoleParticipant) string {
	t.Helper()
	var parts []map[string]string
	for _, p := range ps {
		parts = append(parts, map[string]string{"name": p.name, "role": p.role})
	}
	doc := map[string]any{
		"name":         "qa",
		"participants": parts,
		"steps":        []any{},
		"probes":       []any{},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "s.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// VTT-941
func TestQAScenarioRolePlayerRefusedAnywhere(t *testing.T) {
	valid := []qaRoleParticipant{{"boss", "dm"}, {"bot", "agent"}, {"eye", "spectator"}}
	cases := map[string][]qaRoleParticipant{
		"alone":  {{"zed", "player"}},
		"first":  append([]qaRoleParticipant{{"zed", "player"}}, valid...),
		"last":   append(append([]qaRoleParticipant{}, valid...), qaRoleParticipant{"zed", "player"}),
		"middle": {valid[0], {"zed", "player"}, valid[2]},
	}
	for label, ps := range cases {
		t.Run(label, func(t *testing.T) {
			sc, err := harness.LoadScenario(qaWriteRoleScenario(t, ps))
			if err == nil {
				t.Fatalf("loaded a player scenario: %+v", sc)
			}
			msg := err.Error()
			if !strings.Contains(msg, "zed") {
				t.Errorf("error does not name participant: %q", msg)
			}
			if !strings.Contains(msg, "spectator") {
				t.Errorf("error does not say spectator: %q", msg)
			}
		})
	}
}

// VTT-941
func TestQAScenarioRoleTwoPlayersRefused(t *testing.T) {
	ps := []qaRoleParticipant{{"boss", "dm"}, {"zed", "player"}, {"yan", "player"}}
	_, err := harness.LoadScenario(qaWriteRoleScenario(t, ps))
	if err == nil {
		t.Fatal("loaded a scenario with two players")
	}
	msg := err.Error()
	if !strings.Contains(msg, "zed") && !strings.Contains(msg, "yan") {
		t.Errorf("error names neither player: %q", msg)
	}
	if !strings.Contains(msg, "spectator") {
		t.Errorf("error does not say spectator: %q", msg)
	}
}

// VTT-941
func TestQAScenarioRoleDMAgentSpectatorLoadAsWritten(t *testing.T) {
	ps := []qaRoleParticipant{{"boss", "dm"}, {"bot", "agent"}, {"eye", "spectator"}}
	sc, err := harness.LoadScenario(qaWriteRoleScenario(t, ps))
	if err != nil {
		t.Fatalf("refused a valid scenario: %v", err)
	}
	if len(sc.Participants) != len(ps) {
		t.Fatalf("got %d participants, want %d", len(sc.Participants), len(ps))
	}
	for i, p := range ps {
		got := sc.Participants[i]
		if got.Name != p.name || got.Role != p.role {
			t.Errorf("participant %d = %+v, want %+v", i, got, p)
		}
	}
}

// VTT-941
func TestQAScenarioRolePlayerNamedOthersAreNotRefused(t *testing.T) {
	ps := []qaRoleParticipant{{"player", "spectator"}, {"dm", "dm"}}
	if _, err := harness.LoadScenario(qaWriteRoleScenario(t, ps)); err != nil {
		t.Fatalf("refused a spectator merely named player: %v", err)
	}
}
