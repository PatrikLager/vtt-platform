package harness_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// TestLoadScenarioValidMinimal loads a well-formed scenario (one command
// step, one reconnect step, one probe) and checks every top-level field
// decoded as written.
func TestLoadScenarioValidMinimal(t *testing.T) {
	sc, err := harness.LoadScenario("testdata/valid_minimal.json")
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if sc.Name != "valid-minimal" {
		t.Fatalf("Name = %q, want %q", sc.Name, "valid-minimal")
	}
	if len(sc.Participants) != 1 || sc.Participants[0].Name != "dm" || sc.Participants[0].Role != "dm" {
		t.Fatalf("Participants = %+v, want one {dm, dm}", sc.Participants)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(sc.Steps))
	}
	first := sc.Steps[0]
	if first.By != "dm" || len(first.Command) == 0 || first.Expect == nil || !first.Expect.OK {
		t.Fatalf("Steps[0] = %+v, want a command step by dm with expect.ok=true", first)
	}
	if first.Reconnect != nil {
		t.Fatalf("Steps[0].Reconnect = %+v, want nil", first.Reconnect)
	}
	second := sc.Steps[1]
	if second.Reconnect == nil || second.Reconnect.AfterSequence != 0 {
		t.Fatalf("Steps[1].Reconnect = %+v, want {AfterSequence: 0}", second.Reconnect)
	}
	if len(second.Command) != 0 {
		t.Fatalf("Steps[1].Command = %q, want empty", second.Command)
	}
	if len(sc.Probes) != 1 || sc.Probes[0].SessionCount == nil || sc.Probes[0].SessionCount.Open != 1 || sc.Probes[0].SessionCount.Total != 1 {
		t.Fatalf("Probes = %+v, want one sessionCount{open:1,total:1}", sc.Probes)
	}
}

// TestLoadScenarioRejectsUnknownField loads a scenario whose one step
// carries an extra "bogus" field: strict decoding must reject it, and the
// error must name the offending step's index (0).
func TestLoadScenarioRejectsUnknownField(t *testing.T) {
	_, err := harness.LoadScenario("testdata/unknown_field.json")
	if err == nil {
		t.Fatal("LoadScenario: want error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "step 0") {
		t.Fatalf("error = %q, want it to name step 0", err.Error())
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("error = %q, want it to name the unknown field \"bogus\"", err.Error())
	}
}

// TestLoadScenarioRequiresExpectWithCommand loads a scenario whose one
// command step omits "expect": load must fail, naming step 0.
func TestLoadScenarioRequiresExpectWithCommand(t *testing.T) {
	_, err := harness.LoadScenario("testdata/missing_expect.json")
	if err == nil {
		t.Fatal("LoadScenario: want error for missing expect, got nil")
	}
	if !strings.Contains(err.Error(), "step 0") {
		t.Fatalf("error = %q, want it to name step 0", err.Error())
	}
	if !strings.Contains(err.Error(), "expect") {
		t.Fatalf("error = %q, want it to mention \"expect\"", err.Error())
	}
}

// TestLoadScenarioRejectsBothCommandAndReconnect loads a scenario whose one
// step sets BOTH command and reconnect: load must fail, naming step 0.
func TestLoadScenarioRejectsBothCommandAndReconnect(t *testing.T) {
	_, err := harness.LoadScenario("testdata/both_command_and_reconnect.json")
	if err == nil {
		t.Fatal("LoadScenario: want error for both command and reconnect set, got nil")
	}
	if !strings.Contains(err.Error(), "step 0") {
		t.Fatalf("error = %q, want it to name step 0", err.Error())
	}
}

// TestLoadScenarioRejectsNeitherCommandNorReconnect is the fixtures'
// implicit fourth case (not a named testdata file — constructed inline):
// a step with NEITHER command nor reconnect set must also fail to load,
// naming step 0, proving the "exactly one" rule rejects zero as well as two.
func TestLoadScenarioRejectsNeitherCommandNorReconnect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "neither.json")
	body := `{
		"name": "neither",
		"participants": [{"name": "dm", "role": "dm"}],
		"steps": [{"by": "dm"}],
		"probes": []
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := harness.LoadScenario(path)
	if err == nil {
		t.Fatal("LoadScenario: want error for neither command nor reconnect set, got nil")
	}
	if !strings.Contains(err.Error(), "step 0") {
		t.Fatalf("error = %q, want it to name step 0", err.Error())
	}
}

// TestLoadScenarioRejectsAmbiguousExpect loads a scenario whose one command
// step sets Expect.OK=false with an empty DeniedContaining — a shape that
// (before this fix) silently ran as an ok-expectation, the opposite of what
// an author writing "ok": false almost certainly meant. Load must fail,
// naming step 0, and the error must point at the two valid shapes.
func TestLoadScenarioRejectsAmbiguousExpect(t *testing.T) {
	_, err := harness.LoadScenario("testdata/ambiguous_expect.json")
	if err == nil {
		t.Fatal("LoadScenario: want error for an ambiguous expect (ok=false, deniedContaining=\"\"), got nil")
	}
	if !strings.Contains(err.Error(), "step 0") {
		t.Fatalf("error = %q, want it to name step 0", err.Error())
	}
	if !strings.Contains(err.Error(), `"ok": true`) || !strings.Contains(err.Error(), "deniedContaining") {
		t.Fatalf("error = %q, want it to name both valid expect shapes", err.Error())
	}
}

// TestLoadScenarioRejectsAmbiguousProbe constructs a scenario whose one
// probe sets BOTH tokenAt and actorExists — the exactly-one-of-three-kinds
// rule Probe shares with Step's Command/Reconnect exclusivity.
func TestLoadScenarioRejectsAmbiguousProbe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ambiguous_probe.json")
	body := `{
		"name": "ambiguous-probe",
		"participants": [{"name": "dm", "role": "dm"}],
		"steps": [
			{"by": "dm", "command": {"startSession": {"name": "s1"}}, "expect": {"ok": true}}
		],
		"probes": [
			{"tokenAt": {"tokenId": "t", "x": 1, "y": 1}, "actorExists": {"actorId": "a"}}
		]
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := harness.LoadScenario(path)
	if err == nil {
		t.Fatal("LoadScenario: want error for a probe with two kinds set, got nil")
	}
	if !strings.Contains(err.Error(), "probe 0") {
		t.Fatalf("error = %q, want it to name probe 0", err.Error())
	}
}

// TestLoadScenarioMissingFile checks the plain I/O failure path.
func TestLoadScenarioMissingFile(t *testing.T) {
	_, err := harness.LoadScenario("testdata/does_not_exist.json")
	if err == nil {
		t.Fatal("LoadScenario: want error for a missing file, got nil")
	}
}
