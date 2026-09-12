package campaign_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/eventgen"
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
// applyDrawn's doc records that reproducing from the output alone is a spec
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

// TestRebuildEqualsLiveProperty is the keystone property (spec §9): for a
// long, varied, valid event history the state rebuilt from a full log replay
// always equals the live, incrementally-folded projection. It's checked by
// closing and reopening the campaign every 50 events (single-writer: the
// SQLite file must be closed before it can be reopened) and comparing State()
// before and after.
// WHAT THIS ORACLE CAN SEE, AND WHAT IT CANNOT, measured rather than argued —
// the paragraph moved here on 2026-09-12 when eventgen took the generator,
// because it describes the ASSERTION and not the draws.
//
// Both sides run the same engine.Apply, so a wrong fold is wrong IDENTICALLY on
// both and the comparison cancels it out. Three semantic faults injected into
// engine.Apply left this GREEN: making ActorRemoved a no-op, deleting
// delete(sc.OpenDoors, ...) from the DoorClosed arm, and making a control grant
// prepend instead of append. Semantics are internal/engine's own tests to hold,
// and reading a green run here as "the fold is correct" is the mistake this
// paragraph exists to prevent.
//
// What it catches is REPLAY FIDELITY, and there it bites. Five store round-trip
// faults were measured green before the four add/remove pairs were generated
// and red after: DoorOpened.At dropped on read, DoorClosed skipped on read,
// TokenRemoved skipped on read, ConditionApplied.Source dropped, and
// ActorControlGranted.ParticipantId corrupted. That is the whole of what those
// twelve event types buy by being drawn at all, and it is worth having — none
// of them reached a generated walk before.
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
// applyDrawn appends one drawn action and judges the response, which is the
// half eventgen deliberately does not do: the model says what it drew and
// whether the engine MUST refuse it, and this decides whether what came back
// was the right answer.
//
// REPRODUCING FROM THE OUTPUT ALONE IS A SPEC REQUIREMENT, and this function is
// where that is now recorded — propMust carried it until eventgen took the
// generator on 2026-09-12. Every failure here names the action index and kind,
// and the seed reaches the message through the subtest's own name, so a report
// can be re-run with VTT_PROPERTY_SEED and walk the identical history.
//
// A REFUSAL IS COUNTED, NOT TOLERATED. An action the model aimed at an absent
// key, an unknown scene or an actor still holding a token is required to fail —
// succeeding is a test failure, not a shrug — and the reverse is the assertion
// that matters more: a draw the model believes is legal must be accepted, so a
// guard that starts refusing too much reds here rather than passing quietly.
func applyDrawn(t *testing.T, c *campaign.Campaign, m *eventgen.Model, a eventgen.Action, idx int, counts map[string]int) {
	t.Helper()
	if a.Env == nil {
		return // a draw with nothing legal to aim at; see eventgen's removeActor
	}
	seq, err := c.Append(a.Env)
	if err != nil {
		if !a.MustFail {
			t.Fatalf("property test (%s): action #%d (%s) failed, and the model believes it "+
				"is legal: %v", t.Name(), idx, a.Kind, err)
		}
		counts[a.Kind+"Rejected"]++
		return
	}
	if a.MustFail {
		t.Fatalf("property test (%s): action #%d (%s) was accepted, and the model aimed it at "+
			"something engine.Apply is required to refuse", t.Name(), idx, a.Kind)
	}
	m.Accepted(a, seq)
	counts[a.Kind]++
}

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
	// eventgen's deleteNote, which aims at an absent key about 30% of the time
	// — but a zero count there is not itself a failure (a different seed or mix
	// could legitimately avoid drawing it); the actual counts are logged either
	// way.
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

	m := eventgen.New()
	counts := map[string]int{}

	for i := 0; i < propertyEventCount; i++ {
		applyDrawn(t, c, m, m.Step(rng, i), i, counts)

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
