package campaign_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
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
// A named seed skips the PER-KIND coverage check only. Somebody reproducing a
// walk the sweep pointed at does not need to be told it is narrow — 15% of
// walks legitimately never move a token — and the check can only ever speak on
// a reproduce that PASSED, since a real failure aborts before it. The undo
// check is NOT skipped: its measured miss rate over 500 walks is 0, so it
// cannot false-positive, and retraction is the property this test exists for.
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
// reference something that isn't there), which non-marker sequences remain
// eligible for undo, and whether a session is currently open.
//
// Undo targets ANY non-marker, un-retracted single sequence — not just
// TokenMoved. This exercises campaign.Undo's own dry-run viability check:
// Undo now folds the would-be-retracted log before persisting the marker,
// so a retraction that would corrupt replay (e.g. retracting a
// SessionEnded while a later SessionStarted exists, which would replay as
// "session already open") is rejected instead of bricking the file.
// doUndo therefore does not treat a rejection as a test failure: it counts
// it as undoRejected, leaves the model's retracted set untouched, and the
// run continues. The every-50-events close/reopen checkpoint in
// TestRebuildEqualsLiveProperty doubles as a corruption detector — a
// bricked file fails to reopen there.
type propModel struct {
	scenes []string
	actors []string

	tokenIDs   []string
	tokenScene map[string]string
	tokenPos   map[string][2]int32

	allSeqs   []int64
	retracted map[int64]bool

	sessionOpen bool

	sceneN, actorN, tokenN, noteN int

	// noteKeys tracks keys the model believes are CURRENTLY present (world-
	// layer Task 3): doUpsertNote appends a fresh key or re-upserts an
	// existing one (last-write-wins exercised); doDeleteNote removes a
	// tracked key on success. Deliberately NOT unioned into allSeqs — see
	// doAddNarration/doUpsertNote/doDeleteNote's doc comments: narration/
	// note events are exercised as their own action kind, not folded into
	// the undo-eligible pool, keeping this task's addition minimal and
	// independently verifiable against the pre-existing action mix.
	noteKeys []string
}

func newPropModel() *propModel {
	return &propModel{
		tokenScene: map[string]string{},
		tokenPos:   map[string][2]int32{},
		retracted:  map[int64]bool{},
	}
}

func (m *propModel) canPlaceToken() bool { return len(m.scenes) > 0 && len(m.actors) > 0 }
func (m *propModel) canMoveToken() bool  { return len(m.tokenIDs) > 0 }

func (m *propModel) eligibleUndoSeqs() []int64 {
	out := make([]int64, 0, len(m.allSeqs))
	for _, seq := range m.allSeqs {
		if !m.retracted[seq] {
			out = append(out, seq)
		}
	}
	return out
}

func (m *propModel) canUndo() bool { return len(m.eligibleUndoSeqs()) > 0 }

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

func (m *propModel) doCreateScene(t *testing.T, c *campaign.Campaign, idx int) {
	t.Helper()
	m.sceneN++
	id := fmt.Sprintf("prop-scn-%d", m.sceneN)
	seq := propMust(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 20, GridHeight: 20,
	}), idx, "createScene")
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
	m.allSeqs = append(m.allSeqs, seq)
}

// doMoveToken uses the model's last known position as From. That tracked
// position can go stale relative to live state after an undo retracts an
// earlier move on the same token (the model doesn't unwind on retraction),
// but engine.Apply never validates From against current position — only
// that the token exists and To is set — so a stale From cannot turn this
// into an invalid action; it only means From/To are not always contiguous,
// which doesn't matter for what this test verifies.
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

// doUndo picks a random eligible (non-marker, un-retracted) sequence and
// attempts to retract it alone. Unlike the other actions, a rejection here
// is not a test failure: campaign.Undo is now expected to reject ranges
// that would corrupt replay (e.g. a SessionEnded a later SessionStarted
// depends on), so doUndo counts the rejection as undoRejected, leaves the
// model's retracted set untouched, and lets the run continue.
//
// A successful retraction is counted as undo and marks the sequence
// retracted so it isn't offered again. Because eligibility is no longer
// restricted to TokenMoved, a successful retraction can remove a scene,
// actor, token, or session boundary from live state (campaign.Undo's own
// dry-run guarantees nothing still in the log depended on it, or the
// retraction would have been rejected). The model's bookkeeping for future
// action generation is resynced from live state afterward so it never
// offers a follow-up action — e.g. placing a token on a just-retracted
// actor — that only looks valid because the model forgot the retraction
// happened.
func (m *propModel) doUndo(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	eligible := m.eligibleUndoSeqs()
	seq := eligible[rng.Intn(len(eligible))]
	if _, err := c.Undo(seq, seq, "property-test undo", nextID(), "dm", "test-participant"); err != nil {
		counts["undoRejected"]++
		return
	}
	m.retracted[seq] = true
	counts["undo"]++
	m.resyncFromState(c.State())
}

// resyncFromState rebuilds the model's scene/actor/token/session bookkeeping
// from live campaign state. Called after a successful Undo (see doUndo)
// since a retraction can make the model's tracked entities stale.
//
// Go map iteration order is randomized per run, so each rebuilt slice is
// sorted before use: downstream generation (doPlaceToken, doMoveToken, …)
// indexes into these slices with rng.Intn, and an unsorted, iteration-order
// slice would let the same rng draw sequence pick a different entity from
// run to run — silently breaking the seed-1 run's reproducibility even
// though every individual action stays valid.
func (m *propModel) resyncFromState(st *engine.State) {
	m.scenes = m.scenes[:0]
	for id := range st.Scenes {
		m.scenes = append(m.scenes, id)
	}
	sort.Strings(m.scenes)

	m.actors = m.actors[:0]
	for id := range st.Actors {
		m.actors = append(m.actors, id)
	}
	sort.Strings(m.actors)

	m.tokenIDs = m.tokenIDs[:0]
	for id, tok := range st.Tokens {
		m.tokenIDs = append(m.tokenIDs, id)
		m.tokenScene[id] = tok.SceneID
		m.tokenPos[id] = [2]int32{tok.X, tok.Y}
	}
	sort.Strings(m.tokenIDs)
	for id := range m.tokenScene {
		if _, ok := st.Tokens[id]; !ok {
			delete(m.tokenScene, id)
			delete(m.tokenPos, id)
		}
	}

	m.sessionOpen = false
	for _, s := range st.Sessions {
		if s.EndSeq == 0 {
			m.sessionOpen = true
			break
		}
	}
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
// respecting the spec's backward-only anchor rule) drawn from allSeqs, the
// same pool doUndo already treats as "real, addressable history". Both
// anchored and unanchored draws are expected to succeed unconditionally —
// this exercises both code paths.
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
// removal, spec §3) — counted as deleteNoteRejected, not a test failure,
// exactly like doUndo counts undoRejected. The rest delete a real tracked
// key and untrack it.
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

// step picks one random VALID action given the current model and applies
// it, per the action mix (re-baselined this task, world-layer Task 3 —
// deliberately NOT the same mix as before; see the report for old->new
// pinned counts, both the wiring pass and the anchor-fix follow-up pass):
// create scene ~5%, add actor ~10%, place token ~13% (when scene+actor
// exist), move token ~40% (when tokens exist), undo ~8% (when an eligible
// non-marker, un-retracted sequence exists — see doUndo for why a rejected
// undo is not a test failure), add narration ~9% (mix of anchored/
// unanchored, both expected to succeed — see doAddNarration's doc comment
// for the anchor-validation fix that made anchored draws reliably
// succeed), upsert note ~8%, delete note ~5% (absent-key rejections
// counted, not failures — same posture as undo), start/end session for
// the remainder (start if none open; end if one's open, gated further to
// rand<0.15 so sessions stay open across most of the run). Any bucket
// whose precondition
// isn't met falls through to the next check using the same draw, and the
// session bucket's own "session open, but the 0.15 gate didn't fire" branch
// falls back to addActor — always valid — so every iteration guarantees
// forward progress toward the requested event count.
func (m *propModel) step(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	r := rng.Float64()
	switch {
	case r < 0.05:
		m.doCreateScene(t, c, idx)
		counts["createScene"]++
	case r < 0.15:
		m.doAddActor(t, c, idx)
		counts["addActor"]++
	case r < 0.28 && m.canPlaceToken():
		m.doPlaceToken(t, c, rng, idx)
		counts["placeToken"]++
	case r < 0.68 && m.canMoveToken():
		m.doMoveToken(t, c, rng, idx)
		counts["moveToken"]++
	case r < 0.76 && m.canUndo():
		m.doUndo(t, c, rng, idx, counts)
	case r < 0.84:
		m.doAddNarration(t, c, rng, idx, counts)
	case r < 0.90:
		m.doUpsertNote(t, c, rng, idx, counts)
	case r < 0.94:
		m.doDeleteNote(t, c, rng, idx, counts)
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
// long, varied, valid event history — including undo — the state rebuilt
// from a full log replay always equals the live, incrementally-folded
// projection. It's checked by closing and reopening the campaign every 50
// events (single-writer: the SQLite file must be closed before it can be
// reopened) and comparing State() before and after.
func TestRebuildEqualsLiveProperty(t *testing.T) {
	seeds, guardEachWalk, guardEnsemble := propertySeeds(t)

	total := map[string]int{}
	for _, seed := range seeds {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			counts := runPropertyWalk(t, seed)
			for k, v := range counts {
				total[k] += v
			}
			assertUndoExercised(t, "this walk", counts)
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

// assertActionCoverage refuses a VACUOUS run: a walk that never drew an action
// kind proves nothing about it, however green it looks.
//
// THE SCOPE IS THE WHOLE POINT, AND IT CHANGED 2026-08-27 when the seed stopped
// being a constant. Per WALK is right for the default single-seed run — one
// fixed walk that quietly stopped exercising undo would otherwise pass forever,
// which is what this guard was added to prevent.
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
		"createScene", "addActor", "placeToken", "moveToken", "startSession", "endSession",
		"addNarration", "upsertNote", "deleteNote",
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

// assertUndoExercised runs in EVERY mode, unlike the per-kind check.
//
// It is the one guard that cannot false-positive: over 500 seeds undo was drawn
// in every single walk, minimum 13 times, so a zero here means the generator
// stopped drawing it rather than that this walk was narrow. It is also the
// guard that matters most — retraction is what a rebuild-equals-live property
// is really testing, and a run that never retracted proves nothing about it
// however green it looks.
func assertUndoExercised(t *testing.T, scope string, counts map[string]int) {
	t.Helper()
	if counts["undo"] == 0 {
		t.Fatalf("property test (%s): undo was never exercised — %s proves nothing about retraction",
			t.Name(), scope)
	}
}

// runPropertyWalk is one seed's walk: propertyEventCount model-driven actions
// against a real campaign file, closing and reopening every propertyCheckEvery
// to check that the state rebuilt from the log equals the state held live.
//
// Returns its action counts so the caller can judge coverage at the right
// scope — see assertActionCoverage.
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
