package campaign_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

const (
	defaultPropertySeed = 1
	propertyEventCount  = 400
	propertyCheckEvery  = 50
)

// propertySeeds is the walk (or walks) this run performs.
//
// THE SEED WAS A CONST UNTIL 2026-08-27, and that made this a fixed scenario
// wearing a property test's clothes: every run, in every CI job, walked the
// identical 400 actions. Running it a thousand times explored one path a
// thousand times, so it could never find tomorrow what it did not find today.
// The model, the generators and the oracle were all already here; only the
// search was missing.
//
// DEFAULT IS UNCHANGED AND DELIBERATELY SO. With neither variable set this
// returns exactly seed 1, so tier 1, the commit hook and CI keep running the
// same walk they always did, at the same cost, with the same determinism. A
// sweep is something you ASK for.
//
//	VTT_PROPERTY_SEEDS=500  walks seeds 1..500 as subtests (~0.13s each)
//	VTT_PROPERTY_SEED=37    walks only seed 37
//
// The single-seed form is not a convenience: once seeds vary, a failure report
// naming seed 37 is only reproducible if seed 37 can be re-run on its own, and
// propMust's doc records that reproducing from the output alone is a spec
// requirement. A sweep-only knob would have quietly broken it. If BOTH are set,
// VTT_PROPERTY_SEED wins — it is checked first, and naming one seed is the more
// specific request.
//
// THE TWO BOOLS SAY WHAT WAS ASKED FOR, NOT HOW MANY SEEDS CAME BACK, and that
// distinction is load-bearing: keying the ensemble check on len(seeds) > 1 left
// VTT_PROPERTY_SEEDS=1 judged by neither guard, which is the first thing anyone
// types to check the harness works.
//
// A named seed skips the PER-KIND coverage check. Somebody reproducing a walk
// the sweep pointed at does not need to be told it is narrow — 15% of walks
// legitimately never move a token — and the check can only ever speak on a
// reproduce that PASSED, since a real failure aborts before it.
//
// IT USED TO SKIP ONE OF TWO. A second guard, assertUndoExercised, ran in
// every mode and reded a walk that never retracted, because retraction was
// what a rebuild-equals-live property was really testing. Retraction left the
// platform on 2026-08-31 (spec 2026-08-30-retraction-leaves) and the guard
// left with the action; what this test now exercises is a log that only grows,
// which is the only kind there is.
func propertySeeds(t *testing.T) (seeds []int64, guardEachWalk, guardEnsemble bool) {
	t.Helper()
	if v := os.Getenv("VTT_PROPERTY_SEED"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			t.Fatalf("VTT_PROPERTY_SEED=%q is not an integer: %v", v, err)
		}
		return []int64{n}, false, false
	}
	if v := os.Getenv("VTT_PROPERTY_SEEDS"); v != "" {
		// The cap is not fussiness: make([]int64, n) for a typo'd
		// 9999999999 asks for 80 GB and the binary dies to the OOM killer
		// with no diagnostic at all, 17s in.
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100000 {
			t.Fatalf("VTT_PROPERTY_SEEDS=%q must be a positive integer count of at most 100000", v)
		}
		out := make([]int64, n)
		for i := range out {
			out[i] = int64(i + 1)
		}
		return out, false, true
	}
	return []int64{defaultPropertySeed}, true, false
}

// propModel tracks just enough campaign shape to generate only valid
// forward actions: which scenes/actors/tokens exist (so place/move never
// reference something that isn't there), which sequences have been appended
// (so a narration can anchor to a real one), and whether a session is
// currently open.
//
// EVERY ACTION IT GENERATES IS FORWARD, and since 2026-08-31 that is not a
// property of the model but of the platform: it drew an undo until then, and
// tracked a retracted set to keep from offering the same sequence twice. The
// every-50-events close/reopen checkpoint in TestRebuildEqualsLiveProperty
// still doubles as a corruption detector — a file that will not reopen fails
// there.
type propModel struct {
	scenes []string
	actors []string

	tokenIDs   []string
	tokenScene map[string]string
	tokenPos   map[string][2]int32

	allSeqs []int64

	sessionOpen bool

	sceneN, actorN, tokenN, noteN int

	// noteKeys tracks keys the model believes are CURRENTLY present (world-
	// layer Task 3): doUpsertNote appends a fresh key or re-upserts an
	// existing one (last-write-wins exercised); doDeleteNote removes a
	// tracked key on success. Deliberately NOT unioned into allSeqs — see
	// doAddNarration/doUpsertNote/doDeleteNote's doc comments: narration/
	// note events are exercised as their own action kind, not folded into
	// the pool a narration anchors into, keeping this task's addition minimal
	// and independently verifiable against the pre-existing action mix.
	noteKeys []string

	// THE FOUR ADD/REMOVE PAIRS, added 2026-09-12, and what they buy is
	// measured rather than argued. Five store round-trip faults are green at
	// the merge base and red here: DoorOpened.At dropped on read, DoorClosed
	// skipped, TokenRemoved skipped, ConditionApplied.Source dropped,
	// ActorControlGranted.ParticipantId corrupted. None of those twelve event
	// types reached a generated walk before, so nothing here would have
	// noticed.
	//
	// Removal is the half worth having: a live incremental fold and a
	// from-scratch replay agree cheaply while nothing is ever unset, and until
	// now the only action that removed anything was doDeleteNote.
	//
	// Each map is what the MODEL believes, so an action can choose a legal
	// target and assert the engine accepts it — and choose an illegal one and
	// assert the engine refuses. engine.Apply's refusals are the contract
	// being modelled here: an unknown scene for a door, an unknown or
	// already-applied condition, a token that is not on the board, and an
	// actor that still has one.
	tokenActor map[string]string // token -> the actor it stands for
	// A SET, because engine.Apply's OpenDoors is one. A slice let the same
	// (x,y) be recorded twice when a walk opened a door it had already opened,
	// and one close then left the model believing a door was open that the
	// engine had deleted. No assertion could fire on that — closing an
	// unopened door is legal — so it showed up only as doCloseDoor hitting a
	// genuinely-open door less often than the counts suggested.
	openDoors  map[string]map[[2]int32]bool // scene -> positions currently open
	grants     map[string][]string          // actor -> participants controlling it
	conditions map[string][]string          // actor -> condition ids applied
}

func newPropModel() *propModel {
	return &propModel{
		tokenScene: map[string]string{},
		tokenPos:   map[string][2]int32{},
		tokenActor: map[string]string{},
		openDoors:  map[string]map[[2]int32]bool{},
		grants:     map[string][]string{},
		conditions: map[string][]string{},
	}
}

func (m *propModel) canPlaceToken() bool { return len(m.scenes) > 0 && len(m.actors) > 0 }
func (m *propModel) canMoveToken() bool  { return len(m.tokenIDs) > 0 }

// propMust appends env and fails the test with the failing action index and
// seed on error (spec requirement: failures must be reproducible from the
// output alone). The seed reaches the message through the subtest's NAME,
// which TestRebuildEqualsLiveProperty formats as "seed=N".
func propMust(t *testing.T, c *campaign.Campaign, env *vttv1.Envelope, idx int, kind string) int64 {
	t.Helper()
	seq, err := c.Append(env)
	if err != nil {
		t.Fatalf("property test (%s): action #%d (%s) failed: %v", t.Name(), idx, kind, err)
	}
	return seq
}

// doSceneCreated appends one SceneCreated straight to the log.
//
// NAMED FOR THE EVENT, unlike doAddActor/doPlaceToken beside it, and the
// inconsistency is deliberate: it was doCreateScene until 2026-09-02, when
// create_scene left the platform (Patrik's ruling, 2026-09-01) and there was
// no longer a command of that name for the label to mean. This model appends
// EVENTS — it never issues a command at all, which is why the rename costs
// nothing here — and SceneCreated is what it appends. Its siblings keep their
// command-shaped names because those commands still exist.
func (m *propModel) doSceneCreated(t *testing.T, c *campaign.Campaign, idx int) {
	t.Helper()
	m.sceneN++
	id := fmt.Sprintf("prop-scn-%d", m.sceneN)
	seq := propMust(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 20, GridHeight: 20,
	}), idx, "sceneCreated")
	m.scenes = append(m.scenes, id)
	m.allSeqs = append(m.allSeqs, seq)
}

func (m *propModel) doAddActor(t *testing.T, c *campaign.Campaign, idx int) {
	t.Helper()
	m.actorN++
	id := fmt.Sprintf("prop-actor-%d", m.actorN)
	seq := propMust(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: id, ModuleId: "prop-module"},
	}), idx, "addActor")
	m.actors = append(m.actors, id)
	m.allSeqs = append(m.allSeqs, seq)
}

func (m *propModel) doPlaceToken(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int) {
	t.Helper()
	m.tokenN++
	id := fmt.Sprintf("prop-tok-%d", m.tokenN)
	scene := m.scenes[rng.Intn(len(m.scenes))]
	actor := m.actors[rng.Intn(len(m.actors))]
	x, y := int32(rng.Intn(50)), int32(rng.Intn(50))
	seq := propMust(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: id, SceneId: scene, ActorId: actor,
		Position: &vttv1.GridPosition{X: x, Y: y},
	}), idx, "placeToken")
	m.tokenIDs = append(m.tokenIDs, id)
	m.tokenScene[id] = scene
	m.tokenPos[id] = [2]int32{x, y}
	m.tokenActor[id] = actor
	m.allSeqs = append(m.allSeqs, seq)
}

// doMoveToken uses the model's last known position as From. Nothing can make
// that tracked position stale now that every action is forward — it could
// until 2026-08-31, when an undo could retract an earlier move the model had
// already recorded — and it would not matter if something did: engine.Apply
// never validates From against current position, only that the token exists
// and To is set. A stale From cannot turn this into an invalid action; it
// only means From/To are not always contiguous.
func (m *propModel) doMoveToken(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int) {
	t.Helper()
	id := m.tokenIDs[rng.Intn(len(m.tokenIDs))]
	from := m.tokenPos[id]
	to := [2]int32{int32(rng.Intn(50)), int32(rng.Intn(50))}
	seq := propMust(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: id, SceneId: m.tokenScene[id],
		From: &vttv1.GridPosition{X: from[0], Y: from[1]},
		To:   &vttv1.GridPosition{X: to[0], Y: to[1]},
	}), idx, "moveToken")
	m.tokenPos[id] = to
	m.allSeqs = append(m.allSeqs, seq)
}

func (m *propModel) doStartSession(t *testing.T, c *campaign.Campaign, idx int) {
	t.Helper()
	seq := propMust(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "prop-session"}), idx, "startSession")
	m.sessionOpen = true
	m.allSeqs = append(m.allSeqs, seq)
}

func (m *propModel) doEndSession(t *testing.T, c *campaign.Campaign, idx int) {
	t.Helper()
	seq := propMust(t, c, cenv(nextID(), &vttv1.SessionEnded{}), idx, "endSession")
	m.sessionOpen = false
	m.allSeqs = append(m.allSeqs, seq)
}

// doAddNarration appends a NarrationAdded event, mixing anchored and
// unanchored draws (world-layer Task 3, spec §4): roughly half of every
// draw with at least two prior sequences on record attempts an anchor
// pointing at two ALREADY-RECORDED sequences (never a future one —
// respecting the spec's backward-only anchor rule) drawn from allSeqs, which
// is every sequence this walk has appended and is now the only thing that pool
// is for — doUndo drew its retraction targets from it until retraction left on
// 2026-08-31. Both anchored and unanchored draws are expected to succeed
// unconditionally — this exercises both code paths.
//
// FORMERLY a known bug here (P11 Task 3's original report): campaign.Append
// used to validate the caller's envelope directly while its Sequence was
// still 0 (the store assigns the real value strictly AFTER the validating
// Apply call), so engine.Apply's anchor check `AnchorToSeq >= env.Sequence`
// always compared against 0 — every anchored narration was rejected
// regardless of validity. FIXED by the controller-authorized follow-up in
// this same task (internal/campaign/campaign.go's Append now validates a
// proto.Clone stamped with the provisional sequence c.head+1 — the same
// fix AppendBatch already applied for its own sequence-dependent folds;
// see Append's doc comment and append_sequence_validation_test.go for the
// full proof). Anchored draws here are no longer expected to fail.
func (m *propModel) doAddNarration(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	na := &vttv1.NarrationAdded{Text: fmt.Sprintf("narration entry #%d", idx)}
	if len(m.allSeqs) >= 2 && rng.Float64() < 0.5 {
		from := m.allSeqs[rng.Intn(len(m.allSeqs))]
		to := m.allSeqs[rng.Intn(len(m.allSeqs))]
		if from > to {
			from, to = to, from
		}
		na.AnchorFromSeq = from
		na.AnchorToSeq = to
	}
	env := &vttv1.Envelope{EventId: nextID(), Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: na}}
	propMust(t, c, env, idx, "addNarration")
	counts["addNarration"]++
}

// doUpsertNote appends a NoteUpserted event (world-layer Task 3): about
// 30% of draws with an existing tracked key re-upsert it (last-write-wins
// exercised — the SAME key, a new title/text, no rejection expected),
// the rest mint a fresh key.
func (m *propModel) doUpsertNote(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	var key string
	if len(m.noteKeys) > 0 && rng.Float64() < 0.30 {
		key = m.noteKeys[rng.Intn(len(m.noteKeys))]
	} else {
		m.noteN++
		key = fmt.Sprintf("prop-note-%d", m.noteN)
		m.noteKeys = append(m.noteKeys, key)
	}
	env := &vttv1.Envelope{EventId: nextID(), Payload: &vttv1.Envelope_NoteUpserted{
		NoteUpserted: &vttv1.NoteUpserted{
			Key: key, Title: fmt.Sprintf("Note %s", key), Text: fmt.Sprintf("text for %s at action #%d", key, idx),
		},
	}}
	propMust(t, c, env, idx, "upsertNote")
	counts["upsertNote"]++
}

// doDeleteNote appends a NoteDeleted event (world-layer Task 3): about 30%
// of draws (or any draw with no tracked key at all) target an absent key
// deliberately — deleteNote's own rejection posture (matches condition
// removal, spec §3) — counted as deleteNoteRejected, not a test failure. The
// rest delete a real tracked key and untrack it.
func (m *propModel) doDeleteNote(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	absent := len(m.noteKeys) == 0 || rng.Float64() < 0.30
	var key string
	if absent {
		key = fmt.Sprintf("prop-note-absent-%d", idx)
	} else {
		i := rng.Intn(len(m.noteKeys))
		key = m.noteKeys[i]
		m.noteKeys = append(m.noteKeys[:i], m.noteKeys[i+1:]...)
	}
	env := &vttv1.Envelope{EventId: nextID(), Payload: &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{Key: key}}}
	_, err := c.Append(env)
	if err != nil {
		if !absent {
			t.Fatalf("property test (%s): action #%d (deleteNote) failed unexpectedly for a tracked key %q: %v", t.Name(), idx, key, err)
		}
		counts["deleteNoteRejected"]++
		return
	}
	if absent {
		t.Fatalf("property test (%s): action #%d (deleteNote) unexpectedly succeeded for an absent key %q", t.Name(), idx, key)
	}
	counts["deleteNote"]++
}

// doOpenDoor and doCloseDoor exercise the door pair, putting OpenDoors through
// the store round-trip and the rebuild that follows it.
//
// WHAT THIS ORACLE CAN SEE, precisely, because a first draft of this comment
// claimed more: both sides of rebuild-equals-live run the same engine.Apply, so
// a fold that applied these out of order or skipped one is wrong IDENTICALLY on
// both sides and the comparison cancels it out. Review deleted the
// delete(sc.OpenDoors, ...) from the DoorClosed arm and this stayed green.
// Semantics are internal/engine's own tests to hold.
//
// What it does catch is a round-trip that loses them: dropping DoorOpened.At on
// read reds this at the first reopen. Five such faults were measured, all green
// at the merge base and all red here — DoorOpened.At dropped, DoorClosed
// skipped on read, TokenRemoved skipped on read, ConditionApplied.Source
// dropped, ActorControlGranted.ParticipantId corrupted. That is the whole of
// what these four pairs buy, and it is worth having: none of those twelve event
// types reached a generated walk before.
//
// The unknown-scene refusal is drawn deliberately, the way doDeleteNote draws
// an absent key: a guard nobody trips is a guard nobody has tested.
func (m *propModel) doOpenDoor(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	unknown := rng.Float64() < 0.15
	scene := fmt.Sprintf("prop-scn-absent-%d", idx)
	if !unknown {
		scene = m.scenes[rng.Intn(len(m.scenes))]
	}
	x, y := int32(rng.Intn(20)), int32(rng.Intn(20))
	env := cenv(nextID(), &vttv1.DoorOpened{SceneId: scene, At: &vttv1.GridPosition{X: x, Y: y}})
	seq, err := c.Append(env)
	if err != nil {
		if !unknown {
			t.Fatalf("property test (%s): action #%d (openDoor) failed for a known scene %q: %v",
				t.Name(), idx, scene, err)
		}
		counts["openDoorRejected"]++
		return
	}
	if unknown {
		t.Fatalf("property test (%s): action #%d (openDoor) succeeded for an absent scene %q",
			t.Name(), idx, scene)
	}
	if m.openDoors[scene] == nil {
		m.openDoors[scene] = map[[2]int32]bool{}
	}
	m.openDoors[scene][[2]int32{x, y}] = true
	m.allSeqs = append(m.allSeqs, seq)
	counts["openDoor"]++
}

func (m *propModel) doCloseDoor(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	// Closing a door that was never opened is LEGAL — the arm deletes from a
	// map, which is a no-op on a missing key — so unlike doOpenDoor there is
	// no refusal to draw here, and inventing one would assert a rule the
	// engine does not have.
	scene := m.scenes[rng.Intn(len(m.scenes))]
	var at [2]int32
	if open := m.openDoors[scene]; len(open) > 0 && rng.Float64() < 0.8 {
		// SORTED, for the reason doRemoveCondition's pool is: ranging a map is
		// unordered and this choice feeds the walk, so an unsorted pick would
		// stop the same seed reproducing.
		keys := make([][2]int32, 0, len(open))
		for k := range open {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b [2]int32) int {
			if a[0] != b[0] {
				return int(a[0] - b[0])
			}
			return int(a[1] - b[1])
		})
		at = keys[rng.Intn(len(keys))]
		delete(open, at)
	} else {
		at = [2]int32{int32(rng.Intn(20)), int32(rng.Intn(20))}
	}
	seq := propMust(t, c, cenv(nextID(), &vttv1.DoorClosed{
		SceneId: scene, At: &vttv1.GridPosition{X: at[0], Y: at[1]},
	}), idx, "closeDoor")
	m.allSeqs = append(m.allSeqs, seq)
	counts["closeDoor"]++
}

// doGrantControl and doRevokeControl exercise the control pair. Both are
// tolerant about the control SET — a re-grant returns nil rather than erroring,
// and a revoke of a participant who holds nothing filters an empty set — so
// this walk draws no refusal from either.
//
// THEY ARE NOT REFUSAL-FREE, and saying so would contradict doOpenDoor's own
// argument two functions up. Both enter controlTarget, which refuses an empty
// participant id and an unknown actor; this walk simply never supplies either.
// Those two guards are pinned by name in internal/engine's actor_control_test.
//
// Order is NOT what this oracle checks, though the order matters elsewhere
// (the set decides which participant a mirrored controller_id belongs to, which
// internal/gateway's projection tests hold). Review made the grant prepend
// instead of append and this stayed green, for the reason doOpenDoor records:
// both sides run the same fold.
func (m *propModel) doGrantControl(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	actor := m.actors[rng.Intn(len(m.actors))]
	pid := fmt.Sprintf("prop-p-%d", rng.Intn(4))
	seq := propMust(t, c, cenv(nextID(), &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: pid,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}), idx, "grantControl")
	held := m.grants[actor]
	if !slices.Contains(held, pid) {
		m.grants[actor] = append(held, pid)
	}
	m.allSeqs = append(m.allSeqs, seq)
	counts["grantControl"]++
}

func (m *propModel) doRevokeControl(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	actor := m.actors[rng.Intn(len(m.actors))]
	pid := fmt.Sprintf("prop-p-%d", rng.Intn(4))
	if held := m.grants[actor]; len(held) > 0 && rng.Float64() < 0.8 {
		pid = held[rng.Intn(len(held))]
	}
	seq := propMust(t, c, cenv(nextID(), &vttv1.ActorControlRevoked{
		ActorId: actor, ParticipantId: pid,
	}), idx, "revokeControl")
	if held := m.grants[actor]; len(held) > 0 {
		m.grants[actor] = slices.DeleteFunc(slices.Clone(held), func(s string) bool { return s == pid })
	}
	m.allSeqs = append(m.allSeqs, seq)
	counts["revokeControl"]++
}

// doApplyCondition and doRemoveCondition exercise the condition pair, which is
// the strictest of the four: the engine refuses a second application of a
// condition an actor already carries, and refuses removing one they do not.
// Both refusals are drawn.
func (m *propModel) doApplyCondition(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	actor := m.actors[rng.Intn(len(m.actors))]
	held := m.conditions[actor]
	duplicate := len(held) > 0 && rng.Float64() < 0.2
	cid := fmt.Sprintf("prop-cond-%d", rng.Intn(5))
	if duplicate {
		cid = held[rng.Intn(len(held))]
	} else if slices.Contains(held, cid) {
		duplicate = true // the random pick collided with one already applied
	}
	env := cenv(nextID(), &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cid, Source: "prop",
	})
	seq, err := c.Append(env)
	if err != nil {
		if !duplicate {
			t.Fatalf("property test (%s): action #%d (applyCondition) failed for a fresh "+
				"condition %q on actor %q: %v", t.Name(), idx, cid, actor, err)
		}
		counts["applyConditionRejected"]++
		return
	}
	if duplicate {
		t.Fatalf("property test (%s): action #%d (applyCondition) succeeded for condition %q "+
			"already on actor %q", t.Name(), idx, cid, actor)
	}
	m.conditions[actor] = append(held, cid)
	m.allSeqs = append(m.allSeqs, seq)
	counts["applyCondition"]++
}

func (m *propModel) doRemoveCondition(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	// PREFER AN ACTOR THAT ACTUALLY CARRIES SOMETHING, the way doCloseDoor
	// prefers a door that is open: the pool is the actors carrying a condition
	// when there are any, and every actor otherwise. Drawing uniformly made
	// most draws hit an actor with nothing on them, which turned them into the
	// absent case.
	//
	// IT IS THE SMALLER OF THE TWO LEVERS and review measured that: the pool
	// alone moved the successful removal from one per walk to two, because it
	// can only redistribute the draws this action GETS. Widening the band from
	// 0.03 to 0.05 is what moved it to seven.
	//
	// ONE rng.Intn EITHER WAY, deliberately. An extra draw here would consume a
	// random number and reshuffle every action after it: the first version of
	// this gated the preference behind an rng.Float64() and moved the whole
	// walk, taking removeCondition from one to zero and reding
	// assertKindCoverage. Changing WHICH element is picked is free; changing
	// HOW MANY numbers are drawn is not.
	pool := make([]string, 0, len(m.conditions))
	for a, carried := range m.conditions {
		if len(carried) > 0 {
			pool = append(pool, a)
		}
	}
	// SORTED, because ranging a map is unordered and this choice feeds the
	// walk: an unsorted pick would make the same seed stop reproducing, which
	// propMust's doc records as a spec requirement.
	slices.Sort(pool)
	if len(pool) == 0 {
		pool = m.actors
	}
	actor := pool[rng.Intn(len(pool))]
	held := m.conditions[actor]
	absent := len(held) == 0 || rng.Float64() < 0.25
	cid := fmt.Sprintf("prop-cond-absent-%d", idx)
	var pick int
	if !absent {
		pick = rng.Intn(len(held))
		cid = held[pick]
	}
	env := cenv(nextID(), &vttv1.ConditionRemoved{
		ActorId: actor, ConditionId: cid, Reason: "prop",
	})
	seq, err := c.Append(env)
	if err != nil {
		if !absent {
			t.Fatalf("property test (%s): action #%d (removeCondition) failed for a condition "+
				"%q the model believes actor %q carries: %v", t.Name(), idx, cid, actor, err)
		}
		counts["removeConditionRejected"]++
		return
	}
	if absent {
		t.Fatalf("property test (%s): action #%d (removeCondition) succeeded for condition %q "+
			"absent from actor %q", t.Name(), idx, cid, actor)
	}
	m.conditions[actor] = append(held[:pick], held[pick+1:]...)
	m.allSeqs = append(m.allSeqs, seq)
	counts["removeCondition"]++
}

// doRemoveToken and doRemoveActor are the pair that takes things out of the
// world, and the one with a CROSS-ENTITY rule: engine.Apply refuses to remove
// an actor that still has a token on the board, because clearing the board is
// the command's job (handleRemoveActor emits one TokenRemoved per token ahead
// of the ActorRemoved, as one batch) and cascading in the fold would put the
// same rule in two places.
//
// That rule is why the model tracks tokenActor. Both directions are drawn: an
// actor with no tokens must be accepted, and an actor still holding one must
// be refused — the second is the guard, and a walk that only ever removed
// clean actors would never touch it.
func (m *propModel) doRemoveToken(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	absent := len(m.tokenIDs) == 0 || rng.Float64() < 0.2
	id := fmt.Sprintf("prop-tok-absent-%d", idx)
	var pick int
	if !absent {
		pick = rng.Intn(len(m.tokenIDs))
		id = m.tokenIDs[pick]
	}
	seq, err := c.Append(cenv(nextID(), &vttv1.TokenRemoved{TokenId: id}))
	if err != nil {
		if !absent {
			t.Fatalf("property test (%s): action #%d (removeToken) failed for a token %q the "+
				"model believes is on the board: %v", t.Name(), idx, id, err)
		}
		counts["removeTokenRejected"]++
		return
	}
	if absent {
		t.Fatalf("property test (%s): action #%d (removeToken) succeeded for an absent token %q",
			t.Name(), idx, id)
	}
	m.tokenIDs = append(m.tokenIDs[:pick], m.tokenIDs[pick+1:]...)
	delete(m.tokenScene, id)
	delete(m.tokenPos, id)
	delete(m.tokenActor, id)
	m.allSeqs = append(m.allSeqs, seq)
	counts["removeToken"]++
}

func (m *propModel) doRemoveActor(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	onBoard := map[string]bool{}
	for _, a := range m.tokenActor {
		onBoard[a] = true
	}
	var clean, held []string
	for _, a := range m.actors {
		if onBoard[a] {
			held = append(held, a)
		} else {
			clean = append(clean, a)
		}
	}
	// Draw the guard when there is one to draw: an actor that still holds a
	// token MUST be refused, and that arm is unreachable from a walk that only
	// ever picks clean actors.
	blocked := len(held) > 0 && rng.Float64() < 0.25
	var id string
	switch {
	case blocked:
		id = held[rng.Intn(len(held))]
	case len(clean) > 0:
		id = clean[rng.Intn(len(clean))]
	default:
		return // nothing removable this step; the mix will come back around
	}
	seq, err := c.Append(cenv(nextID(), &vttv1.ActorRemoved{ActorId: id}))
	if err != nil {
		if !blocked {
			t.Fatalf("property test (%s): action #%d (removeActor) failed for actor %q, which "+
				"the model believes holds no token: %v", t.Name(), idx, id, err)
		}
		counts["removeActorRejected"]++
		return
	}
	if blocked {
		t.Fatalf("property test (%s): action #%d (removeActor) succeeded for actor %q, which "+
			"still has a token on the board", t.Name(), idx, id)
	}
	m.actors = slices.DeleteFunc(slices.Clone(m.actors), func(a string) bool { return a == id })
	delete(m.grants, id)
	delete(m.conditions, id)
	m.allSeqs = append(m.allSeqs, seq)
	counts["removeActor"]++
}

// step picks one random VALID action given the current model and applies it.
// The bands are the thresholds below, on one uniform draw, in order: create
// scene [0, 0.05), add actor [0.05, 0.15), place token [0.15, 0.28) when a
// scene and an actor exist, move token [0.28, 0.52) when a token is on the
// board, add narration [0.52, 0.60), upsert note [0.60, 0.64), delete note
// [0.64, 0.68), then the four add/remove pairs — open door [0.68, 0.72), close
// door [0.72, 0.76), grant control [0.76, 0.79), revoke control [0.79, 0.82),
// apply condition [0.82, 0.86), remove condition [0.86, 0.91), remove token
// [0.91, 0.94), remove actor [0.94, 0.96) — and the remainder [0.96, 1.0)
// starts or ends a session, or adds an actor when a session is already open.
//
// THIS TABLE IS TRANSCRIBED FROM THE SWITCH AND ROTS WHEN IT MOVES. It said
// move token [0.28, 0.68) and narration [0.68, 0.84) until 2026-09-12, five
// bands stale, because the four pairs were inserted without it — and it had
// come loose from this function entirely, sitting on doOpenDoor with no blank
// line between, so `go doc` gave step nothing and gave doOpenDoor this.
//
// A GUARDED CASE HANDS ITS DRAW DOWNWARD rather than redrawing: when
// canMoveToken is false the [0.28, 0.52) draw falls to narration, and when the
// roster is empty the condition and control bands fall to remove token. That
// is why band width alone does not predict a count, and why remove condition
// is wider than the neighbours it is drawn beside.
func (m *propModel) step(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	r := rng.Float64()
	switch {
	case r < 0.05:
		m.doSceneCreated(t, c, idx)
		counts["sceneCreated"]++
	case r < 0.15:
		m.doAddActor(t, c, idx)
		counts["addActor"]++
	case r < 0.28 && m.canPlaceToken():
		m.doPlaceToken(t, c, rng, idx)
		counts["placeToken"]++
	case r < 0.52 && m.canMoveToken():
		m.doMoveToken(t, c, rng, idx)
		counts["moveToken"]++
	case r < 0.60:
		m.doAddNarration(t, c, rng, idx, counts)
	case r < 0.64:
		m.doUpsertNote(t, c, rng, idx, counts)
	case r < 0.68:
		m.doDeleteNote(t, c, rng, idx, counts)
	// THE FOUR PAIRS, and the bands they occupy were paid for by more than one
	// action — a first version of this comment said moveToken funded them alone
	// and review measured that false. moveToken gave 0.16 of its 0.40, and
	// narration gave 0.08 of its 0.16, which is HALF of it; upsertNote gave
	// 0.02 and the session remainder 0.01. Measured over seeds 1..12 across the
	// merge base: addNarration 987 -> 496, moveToken 1758 -> 1064, upsertNote
	// 301 -> 171, endSession 40 -> 27.
	//
	// removeCondition's band is wider than its siblings on purpose. At 0.03 it
	// drew four times in 400 on the default seed where its identical-width
	// neighbour drew eleven: cases guarded by len(m.actors) > 0 hand their draw
	// to the next band down whenever the walk has just emptied the roster, and
	// this one sits directly above an unguarded case that absorbs them.
	case r < 0.72 && len(m.scenes) > 0:
		m.doOpenDoor(t, c, rng, idx, counts)
	case r < 0.76 && len(m.scenes) > 0:
		m.doCloseDoor(t, c, rng, idx, counts)
	case r < 0.79 && len(m.actors) > 0:
		m.doGrantControl(t, c, rng, idx, counts)
	case r < 0.82 && len(m.actors) > 0:
		m.doRevokeControl(t, c, rng, idx, counts)
	case r < 0.86 && len(m.actors) > 0:
		m.doApplyCondition(t, c, rng, idx, counts)
	case r < 0.91 && len(m.actors) > 0:
		m.doRemoveCondition(t, c, rng, idx, counts)
	case r < 0.94:
		m.doRemoveToken(t, c, rng, idx, counts)
	case r < 0.96 && len(m.actors) > 0:
		m.doRemoveActor(t, c, rng, idx, counts)
	default:
		switch {
		case !m.sessionOpen:
			m.doStartSession(t, c, idx)
			counts["startSession"]++
		case rng.Float64() < 0.15:
			m.doEndSession(t, c, idx)
			counts["endSession"]++
		default:
			m.doAddActor(t, c, idx)
			counts["addActor"]++
		}
	}
}

// TestRebuildEqualsLiveProperty is the keystone property (spec §9): for a
// long, varied, valid event history the state rebuilt from a full log replay
// always equals the live, incrementally-folded projection. It's checked by
// closing and reopening the campaign every 50 events (single-writer: the
// SQLite file must be closed before it can be reopened) and comparing State()
// before and after.
func TestRebuildEqualsLiveProperty(t *testing.T) {
	seeds, guardEachWalk, guardEnsemble := propertySeeds(t)

	total := map[string]int{}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			counts := runPropertyWalk(t, seed)
			for k, v := range counts {
				total[k] += v
			}
			if guardEachWalk {
				assertKindCoverage(t, "this walk", counts)
			}
		})
	}
	if guardEnsemble {
		t.Logf("sweep of %d seeds, aggregate action counts: %+v", len(seeds), total)
		// A seed that Fatalf'd never reached the accumulation above, so `total`
		// describes a sweep that did not happen. Judging it invents a finding
		// and lands it on top of the real one — the same noise-on-top-of-the-
		// answer this file argues against for the reproduce path.
		if !t.Failed() {
			assertKindCoverage(t, "the sweep", total)
		}
	}
}

// assertKindCoverage refuses a VACUOUS run: a walk that never drew an action
// kind proves nothing about it, however green it looks.
//
// THE SCOPE IS THE WHOLE POINT, AND IT CHANGED 2026-08-27 when the seed stopped
// being a constant. Per WALK is right for the default single-seed run — one
// fixed walk that quietly stopped exercising an action kind would otherwise
// pass forever, which is what this guard was added to prevent.
//
// Per walk is WRONG for a sweep, and not marginally. placeToken needs a scene
// AND an actor to exist first; endSession needs a session already open; so a
// legitimately narrow walk is not a defect. MEASURED over 500 seeds: 15% of
// walks never move a token, 13% never place one, 21% never end a session.
// Guarding each walk individually reds 132 of 500 runs — while the property
// itself, rebuild == live, held on every one of them. A sweep that is red by
// construction is a sweep nobody can automate or read.
//
// So the ensemble carries it instead: across the seeds actually run, every
// action kind must appear somewhere. That claim is WEAKER than requiring it of
// every walk — 499 silent walks and one that ends a session would still pass
// it. What it buys is sensitivity to the thing this guard exists for, a kind
// that stops being drawn AT ALL: the default walk draws endSession exactly
// once, where 500 walks draw it 910 times, so a regression that silences the
// draw reds the sweep as surely as it reds the default, without the 132 red
// walks that are merely narrow.
func assertKindCoverage(t *testing.T, scope string, counts map[string]int) {
	t.Helper()
	for _, kind := range []string{
		"sceneCreated", "addActor", "placeToken", "moveToken", "startSession", "endSession",
		"addNarration", "upsertNote", "deleteNote",
		// The four add/remove pairs. Listed here for the reason the originals
		// are: a walk that never drew one proves nothing about it, however
		// green it looks.
		"openDoor", "closeDoor", "grantControl", "revokeControl",
		"applyCondition", "removeCondition", "removeToken", "removeActor",
	} {
		if counts[kind] == 0 {
			t.Errorf("property test (%s): action type %q was never exercised in %s",
				t.Name(), kind, scope)
		}
	}
	// deleteNoteRejected (absent-key) is EXPECTED to be non-zero too — see
	// doDeleteNote's doc comment — but a zero count there is not itself a
	// failure (a different seed/mix could legitimately avoid drawing it);
	// the actual counts are logged either way.
}

// runPropertyWalk is one seed's walk: propertyEventCount model-driven actions
// against a real campaign file, closing and reopening every propertyCheckEvery
// to check that the state rebuilt from the log equals the state held live.
//
// Returns its action counts so the caller can judge coverage at the right
// scope — see assertKindCoverage.
func runPropertyWalk(t *testing.T, seed int64) map[string]int {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))

	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("property test (seed=%d): initial open: %v", seed, err)
	}
	t.Cleanup(func() {
		if c != nil {
			c.Close()
		}
	})

	m := newPropModel()
	counts := map[string]int{}

	for i := 0; i < propertyEventCount; i++ {
		m.step(t, c, rng, i, counts)

		if (i+1)%propertyCheckEvery == 0 {
			snapshot := c.State()

			if err := c.Close(); err != nil {
				t.Fatalf("property test (seed=%d): close after action #%d: %v", seed, i, err)
			}
			c, err = campaign.Open(path)
			if err != nil {
				t.Fatalf("property test (seed=%d): reopen after action #%d: %v", seed, i, err)
			}

			live := c.State()
			if !statesEqual(snapshot, live) {
				t.Fatalf("property test (seed=%d): rebuild != live after action #%d\nsnapshot: %+v\nlive:     %+v\n"+
					"reproduce: VTT_PROPERTY_SEED=%d go test -run TestRebuildEqualsLiveProperty ./internal/campaign/",
					seed, i, snapshot, live, seed)
			}
		}
	}

	t.Logf("property test (seed=%d): %d events, action counts: %+v", seed, propertyEventCount, counts)
	return counts
}
