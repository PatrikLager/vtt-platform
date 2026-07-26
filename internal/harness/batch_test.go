package harness_test

// batch_test.go covers observeBatchOnAll (engine.go), the batch-aware
// ok-step matcher (ruleset-interpreter Task 6, binding; extended to
// load_adventure by adventure-format Task 4 — see the
// TestRunScenarioLoadAdventureBatch* tests below): a use_ability OR
// load_adventure CommandResult carries only the FIRST sequence of its
// campaign.AppendBatch batch, so the ok-step assertion cannot wait for a
// known event count the way every other command's single-event
// observeOnAll does. These tests use the SAME scripted fakeConn
// infrastructure as engine_test.go (world map, okSend/broadcast) — a batch
// is simulated by scripting a SendCommand that broadcasts MULTIPLE
// envelopes (with contiguous sequences) to every participant in one call,
// exactly what a real AppendBatch broadcast looks like on the wire.

import (
	"context"
	"io"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// abilityUsedEnv is this file's own minimal batch-event builder. Every
// synthetic batch below is built ONLY from AbilityUsed-kind envelopes
// (distinguished by EventId, never by payload kind): observeBatchOnAll
// itself is payload-kind-agnostic (it only ever inspects Sequence and
// EventId — engine.go), and AbilityUsed folds as a deliberate no-op
// regardless of actor validity (apply.go: "testimony, not state"), so a
// run of them exercises the matcher's sequence/contiguity/cross-
// participant logic without needing a fully-populated fold-valid actor
// setup (ResourceChanged/ConditionApplied would each need a real preceding
// ActorAdded to fold cleanly through RunScenario's own end-of-run Fold —
// orthogonal to what this file tests; internal/gateway/ruleset_test.go
// already covers a REAL, fold-valid tavern-brawl batch end to end).
func abilityUsedEnv(id string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: id, Payload: &vttv1.Envelope_AbilityUsed{AbilityUsed: &vttv1.AbilityUsed{
		ActorId: "brawler", AbilityId: "fists", TargetIds: []string{"patron"},
	}}}
}

// useAbilityCmdJSON is a fixed use_ability oneof body for this file's tests
// — its exact field values are irrelevant to the matcher under test (the
// scripted fakeConn ignores the arguments entirely and just returns
// whatever the test wired up).
const useAbilityCmdJSON = `{"useAbility":{"actorId":"brawler","abilityId":"fists","targetIds":["patron"]}}`

// batchSend scripts a SendCommand that accepts the command (ok=true,
// firstSeq) and broadcasts envs — stamped with contiguous sequences
// starting at firstSeq — to precisely the participants named in to, one
// call.
func batchSend(world map[string]*fakeConn, firstSeq int64, envs []*vttv1.Envelope, to ...string) func(*vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	return func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		for i, env := range envs {
			env.Sequence = firstSeq + int64(i)
			broadcast(world, env, to...)
		}
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: firstSeq}, nil
	}
}

func runOneUseAbilityStep(t *testing.T, world map[string]*fakeConn, participants []harness.Participant) *harness.Report {
	t.Helper()
	sc := &harness.Scenario{
		Participants: participants,
		Steps: []harness.Step{
			{By: "dm", Command: rawCmd(t, useAbilityCmdJSON), Expect: &harness.Expect{OK: true}},
		},
	}
	rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	return rep
}

// TestRunScenarioUseAbilityBatchAllEventsObservedPasses is the payoff case:
// a three-event batch broadcast to both participants passes the ok-step,
// and every event lands in both participants' history in order (a REAL,
// fold-valid tavern-brawl batch — AbilityUsed, ResourceChanged,
// ConditionApplied — is covered end to end by
// internal/gateway/ruleset_test.go's TestUseAbilityHitProducesBatchFirst
// Sequence; this test isolates the harness matcher itself).
func TestRunScenarioUseAbilityBatchAllEventsObservedPasses(t *testing.T) {
	dm := newFakeConn("dm")
	watcher := newFakeConn("watcher")
	world := map[string]*fakeConn{"dm": dm, "watcher": watcher}
	envs := []*vttv1.Envelope{abilityUsedEnv("e1"), abilityUsedEnv("e2"), abilityUsedEnv("e3")}
	dm.send = batchSend(world, 1, envs, "dm", "watcher")

	rep := runOneUseAbilityStep(t, world, []harness.Participant{{Name: "dm"}, {Name: "watcher"}})
	if !rep.Pass || len(rep.Steps) != 1 || !rep.Steps[0].Pass {
		t.Fatalf("Report.Pass = %v, want true (steps = %+v)", rep.Pass, rep.Steps)
	}
}

// TestRunScenarioUseAbilitySingleEventBatchPasses covers the minimum-length
// batch (e.g. a miss with no on-miss outcomes: AbilityUsed alone) — the
// matcher must not require MORE than one event.
func TestRunScenarioUseAbilitySingleEventBatchPasses(t *testing.T) {
	dm := newFakeConn("dm")
	watcher := newFakeConn("watcher")
	world := map[string]*fakeConn{"dm": dm, "watcher": watcher}
	dm.send = batchSend(world, 1, []*vttv1.Envelope{abilityUsedEnv("e1")}, "dm", "watcher")

	rep := runOneUseAbilityStep(t, world, []harness.Participant{{Name: "dm"}, {Name: "watcher"}})
	if !rep.Pass {
		t.Fatalf("Report.Pass = false, want true (single-event batch); steps = %+v", rep.Steps)
	}
}

// TestRunScenarioUseAbilityBatchMissingParticipantFails covers a
// participant that never receives ANY of the batch's events (a broadcast
// bug) — the step must fail, naming that participant.
func TestRunScenarioUseAbilityBatchMissingParticipantFails(t *testing.T) {
	dm := newFakeConn("dm")
	watcher := newFakeConn("watcher")
	world := map[string]*fakeConn{"dm": dm, "watcher": watcher}
	envs := []*vttv1.Envelope{abilityUsedEnv("e1"), abilityUsedEnv("e2")}
	// Broadcast to dm only — watcher never sees anything.
	dm.send = batchSend(world, 1, envs, "dm")

	rep := runOneUseAbilityStep(t, world, []harness.Participant{{Name: "dm"}, {Name: "watcher"}})
	if rep.Pass {
		t.Fatal("Report.Pass = true, want false (watcher never observed the batch)")
	}
	if !strings.Contains(rep.Steps[0].Detail, "watcher") {
		t.Fatalf("Detail = %q, want it to name the missing participant %q", rep.Steps[0].Detail, "watcher")
	}
}

// TestRunScenarioUseAbilityBatchPartialObservationFails covers a
// participant that observes a STRICT SUBSET of the batch (fewer events than
// everyone else, without a contiguity break — e.g. slow catch-up drops a
// tail event) — the length-mismatch cross-participant check must catch it
// even though this participant's own run is internally gap-free.
func TestRunScenarioUseAbilityBatchPartialObservationFails(t *testing.T) {
	dm := newFakeConn("dm")
	watcher := newFakeConn("watcher")
	world := map[string]*fakeConn{"dm": dm, "watcher": watcher}
	full := []*vttv1.Envelope{abilityUsedEnv("e1"), abilityUsedEnv("e2"), abilityUsedEnv("e3")}
	dm.send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		for i, env := range full {
			env.Sequence = 1 + int64(i)
			broadcast(world, env, "dm")
		}
		// watcher only ever sees the first two of the three.
		broadcast(world, full[0], "watcher")
		broadcast(world, full[1], "watcher")
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1}, nil
	}

	rep := runOneUseAbilityStep(t, world, []harness.Participant{{Name: "dm"}, {Name: "watcher"}})
	if rep.Pass {
		t.Fatal("Report.Pass = true, want false (watcher observed only 2 of 3 batch events)")
	}
	if !strings.Contains(rep.Steps[0].Detail, "watcher") {
		t.Fatalf("Detail = %q, want it to name the short-observing participant %q", rep.Steps[0].Detail, "watcher")
	}
}

// TestRunScenarioUseAbilityBatchNonContiguousEventFails covers a broken
// batch on the WIRE itself (a bug elsewhere, e.g. an interleaved foreign
// event) — a gap in the sequence run must fail the step rather than being
// silently accepted as "the batch ended early".
func TestRunScenarioUseAbilityBatchNonContiguousEventFails(t *testing.T) {
	dm := newFakeConn("dm")
	world := map[string]*fakeConn{"dm": dm}
	e1 := abilityUsedEnv("e1")
	e1.Sequence = 1
	e2 := abilityUsedEnv("e2")
	e2.Sequence = 3 // gap: skips sequence 2 entirely
	dm.send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		broadcast(world, e1, "dm")
		broadcast(world, e2, "dm")
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1}, nil
	}

	rep := runOneUseAbilityStep(t, world, []harness.Participant{{Name: "dm"}})
	if rep.Pass {
		t.Fatal("Report.Pass = true, want false (non-contiguous batch sequence)")
	}
	if !strings.Contains(rep.Steps[0].Detail, "non-contiguous") {
		t.Fatalf("Detail = %q, want it to name the contiguity break", rep.Steps[0].Detail)
	}
}

// TestRunScenarioUseAbilityBatchNoEventAtAllFails covers a use_ability
// command whose CommandResult claims ok=true but broadcasts NOTHING (the
// most basic wiring failure) — the same "no event observed" timeout path
// observeOnAll's own tests already cover for single-event commands.
func TestRunScenarioUseAbilityBatchNoEventAtAllFails(t *testing.T) {
	dm := newFakeConn("dm")
	world := map[string]*fakeConn{"dm": dm}
	dm.send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1}, nil
	}

	rep := runOneUseAbilityStep(t, world, []harness.Participant{{Name: "dm"}})
	if rep.Pass {
		t.Fatal("Report.Pass = true, want false (no batch event ever broadcast)")
	}
	if !strings.Contains(rep.Steps[0].Detail, "dm") {
		t.Fatalf("Detail = %q, want it to name dm", rep.Steps[0].Detail)
	}
}

// --- load_adventure is ALSO batch-aware (adventure-format Task 4) ---------
//
// load_adventure's CommandResult is the SAME shape as use_ability's: it
// carries only the batch's first sequence (campaign.AppendBatch's own
// contract — internal/gateway/adventure.go), never its length. The ok-step
// matcher must recognize BOTH oneof cases as batch-shaped, not just
// use_ability.

// loadAdventureCmdJSON is a fixed load_adventure oneof body for this file's
// tests — its exact adventure_id is irrelevant to the matcher under test
// (the scripted fakeConn ignores the arguments entirely).
const loadAdventureCmdJSON = `{"loadAdventure":{"adventureId":"goblin-ambush"}}`

func runOneLoadAdventureStep(t *testing.T, world map[string]*fakeConn, participants []harness.Participant) *harness.Report {
	t.Helper()
	sc := &harness.Scenario{
		Participants: participants,
		Steps: []harness.Step{
			{By: "dm", Command: rawCmd(t, loadAdventureCmdJSON), Expect: &harness.Expect{OK: true}},
		},
	}
	rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	return rep
}

// TestRunScenarioLoadAdventureBatchAllEventsObservedPasses mirrors
// TestRunScenarioUseAbilityBatchAllEventsObservedPasses exactly, but over a
// load_adventure command: a multi-event batch broadcast to both
// participants passes the ok-step, and every event lands in both
// participants' history in order.
func TestRunScenarioLoadAdventureBatchAllEventsObservedPasses(t *testing.T) {
	dm := newFakeConn("dm")
	watcher := newFakeConn("watcher")
	world := map[string]*fakeConn{"dm": dm, "watcher": watcher}
	envs := []*vttv1.Envelope{abilityUsedEnv("a1"), abilityUsedEnv("a2"), abilityUsedEnv("a3")}
	dm.send = batchSend(world, 1, envs, "dm", "watcher")

	rep := runOneLoadAdventureStep(t, world, []harness.Participant{{Name: "dm"}, {Name: "watcher"}})
	if !rep.Pass || len(rep.Steps) != 1 || !rep.Steps[0].Pass {
		t.Fatalf("Report.Pass = %v, want true (steps = %+v)", rep.Pass, rep.Steps)
	}
}

// TestRunScenarioLoadAdventureBatchLeavesNoLeftoverEventsForNextStep proves
// the matcher fully DRAINS a load_adventure batch before the step returns —
// not merely "sees one event with the right leading sequence and calls it
// done" (which observeOnAll, the single-event matcher, would do: it only
// ever reads ONE event per participant and checks its sequence, silently
// leaving the batch's remaining events queued on the connection). A
// three-event load_adventure batch is broadcast first; a second, ordinary
// single-event step follows. If load_adventure were (incorrectly) treated
// as a plain single-event command, this second step would observe one of
// the FIRST batch's own leftover events instead of its own — a sequence
// mismatch, failing the step and corrupting every later observation on this
// connection for the rest of the scenario.
func TestRunScenarioLoadAdventureBatchLeavesNoLeftoverEventsForNextStep(t *testing.T) {
	dm := newFakeConn("dm")
	world := map[string]*fakeConn{"dm": dm}

	loadEnvs := []*vttv1.Envelope{abilityUsedEnv("adv1"), abilityUsedEnv("adv2"), abilityUsedEnv("adv3")}
	nextEnv := abilityUsedEnv("next")
	calls := 0
	dm.send = func(cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
		calls++
		if calls == 1 {
			for i, env := range loadEnvs {
				env.Sequence = int64(1 + i)
				broadcast(world, env, "dm")
			}
			return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 1}, nil
		}
		nextEnv.Sequence = 4
		broadcast(world, nextEnv, "dm")
		return &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true, Sequence: 4}, nil
	}

	sc := &harness.Scenario{
		Participants: []harness.Participant{{Name: "dm"}},
		Steps: []harness.Step{
			{By: "dm", Command: rawCmd(t, loadAdventureCmdJSON), Expect: &harness.Expect{OK: true}},
			{By: "dm", Command: rawCmd(t, `{"endSession":{}}`), Expect: &harness.Expect{OK: true}},
		},
	}
	rep, err := harness.RunScenario(context.Background(), sc, fixedDialer(world), nil, io.Discard)
	if err != nil {
		t.Fatalf("RunScenario: %v", err)
	}
	if !rep.Pass {
		t.Fatalf("Report.Pass = false, want true (the load_adventure batch must be fully drained so the next step observes its OWN event, not a leftover): steps=%+v", rep.Steps)
	}
}
