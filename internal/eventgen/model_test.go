package eventgen_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/eventgen"
)

// walk drives a model the way a property test does: draw, then report the
// sequence back for the draws that are meant to succeed. It stands in for the
// caller's apply step, which this package deliberately does not perform.
func walk(t *testing.T, seed int64, n int) (kinds []string, envs []*vttv1.Envelope) {
	t.Helper()
	m := eventgen.New()
	rng := rand.New(rand.NewSource(seed))
	var seq int64
	for i := 0; i < n; i++ {
		a := m.Step(rng, i)
		kinds = append(kinds, fmt.Sprintf("%s/%t", a.Kind, a.MustFail))
		if a.Env == nil {
			continue
		}
		envs = append(envs, a.Env)
		if !a.MustFail {
			seq++
			m.Accepted(a, seq)
		}
	}
	return kinds, envs
}

// TestTheSameSeedDrawsTheSameWalk is the contract every caller rests on: a
// property test that reports "seed 37 failed" is only useful if seed 37 can be
// re-run on its own and do the same thing.
//
// IT IS NOT A RESTATEMENT OF "rand is deterministic". The model reads its own
// maps to choose targets, and ranging a Go map is deliberately unordered — this
// package sorts in three places for exactly that reason (closeDoor's open set,
// removeCondition's pool, and removeActor working from the actors slice rather
// than the tokenActor map). Delete any one sort and this test is what notices.
func TestTheSameSeedDrawsTheSameWalk(t *testing.T) {
	for _, seed := range []int64{1, 7, 99} {
		first, firstEnvs := walk(t, seed, 400)
		second, secondEnvs := walk(t, seed, 400)
		if strings.Join(first, ",") != strings.Join(second, ",") {
			t.Errorf("seed %d drew a different sequence of actions on its second run", seed)
		}
		if len(firstEnvs) != len(secondEnvs) {
			t.Errorf("seed %d emitted %d envelopes then %d", seed, len(firstEnvs), len(secondEnvs))
		}
		for i := range firstEnvs {
			if firstEnvs[i].String() != secondEnvs[i].String() {
				t.Errorf("seed %d, envelope %d differs between runs:\n  %s\n  %s",
					seed, i, firstEnvs[i], secondEnvs[i])
				break
			}
		}
	}
}

// TestAWalkDrawsEveryKind refuses a model that has gone vacuous. A band that
// can never be reached proves nothing about the events behind it, and the
// campaign property test's own coverage guard can only speak for the single
// default seed — this speaks for several.
func TestAWalkDrawsEveryKind(t *testing.T) {
	want := []string{
		"sceneCreated", "addActor", "placeToken", "moveToken", "startSession", "endSession",
		"addNarration", "upsertNote", "deleteNote",
		"openDoor", "closeDoor", "grantControl", "revokeControl",
		"applyCondition", "removeCondition", "removeToken", "removeActor",
	}
	seen := map[string]int{}
	for seed := int64(1); seed <= 5; seed++ {
		kinds, _ := walk(t, seed, 400)
		for _, k := range kinds {
			seen[strings.SplitN(k, "/", 2)[0]]++
		}
	}
	for _, k := range want {
		if seen[k] == 0 {
			t.Errorf("action %q was never drawn across five 400-action walks", k)
		}
	}
}

// TestBothSidesOfEveryRefusalAreDrawn is the half that makes MustFail worth
// having. An action that only ever aims at legal targets never exercises the
// engine guard behind it, and one that only ever aims at illegal ones never
// proves the legal case works.
func TestBothSidesOfEveryRefusalAreDrawn(t *testing.T) {
	// The actions that model a refusal engine.Apply really has. closeDoor,
	// grantControl and revokeControl are absent on purpose: closing an unopened
	// door and re-granting held control are documented no-ops, so drawing a
	// "must fail" there would assert a rule the engine does not have.
	want := []string{"deleteNote", "openDoor", "applyCondition", "removeCondition",
		"removeToken", "removeActor"}
	got := map[string]map[bool]int{}
	for seed := int64(1); seed <= 5; seed++ {
		kinds, _ := walk(t, seed, 400)
		for _, k := range kinds {
			parts := strings.SplitN(k, "/", 2)
			if got[parts[0]] == nil {
				got[parts[0]] = map[bool]int{}
			}
			got[parts[0]][parts[1] == "true"]++
		}
	}
	for _, k := range want {
		if got[k][true] == 0 {
			t.Errorf("action %q never drew the target engine.Apply must refuse", k)
		}
		if got[k][false] == 0 {
			t.Errorf("action %q never drew a legal target", k)
		}
	}
}

// TestARefusedDrawNeverNamesALiveTarget pins the structural separation the
// model relies on instead of bookkeeping: every deliberately-illegal id carries
// an "absent" infix, so a MustFail draw cannot collide with something the walk
// really created and turn a required refusal into a legitimate success.
func TestARefusedDrawNeverNamesALiveTarget(t *testing.T) {
	m := eventgen.New()
	rng := rand.New(rand.NewSource(3))
	var seq int64
	live := map[string]bool{}
	for i := 0; i < 400; i++ {
		a := m.Step(rng, i)
		if a.Env == nil {
			continue
		}
		id := targetID(a.Env)
		if a.MustFail {
			if live[id] {
				t.Errorf("action #%d (%s) aimed at %q, which the walk really created — "+
					"the refusal it asserts could be a legitimate success", i, a.Kind, id)
			}
			continue
		}
		seq++
		m.Accepted(a, seq)
		if id != "" {
			live[id] = true
		}
	}
}

// targetID is the id an action names, for the collision check above. It is
// deliberately partial: actions whose refusal is not about an id return "".
func targetID(e *vttv1.Envelope) string {
	switch p := e.GetPayload().(type) {
	case *vttv1.Envelope_NoteUpserted:
		return "note:" + p.NoteUpserted.GetKey()
	case *vttv1.Envelope_NoteDeleted:
		return "note:" + p.NoteDeleted.GetKey()
	case *vttv1.Envelope_SceneCreated:
		return "scene:" + p.SceneCreated.GetSceneId()
	case *vttv1.Envelope_DoorOpened:
		return "scene:" + p.DoorOpened.GetSceneId()
	case *vttv1.Envelope_TokenPlaced:
		return "token:" + p.TokenPlaced.GetTokenId()
	case *vttv1.Envelope_TokenRemoved:
		return "token:" + p.TokenRemoved.GetTokenId()
	case *vttv1.Envelope_ActorAdded:
		return "actor:" + p.ActorAdded.GetActor().GetActorId()
	}
	return ""
}

// TestOnlyAnchoringActionsFeedTheAnchorPool pins the one mechanism the split
// introduced. A sequence is assigned by whatever APPLIES an event, so the model
// cannot fill its own anchor pool — Accepted is how it comes back, and Anchors
// decides whether it joins. Notes and narration deliberately do not.
func TestOnlyAnchoringActionsFeedTheAnchorPool(t *testing.T) {
	m := eventgen.New()
	rng := rand.New(rand.NewSource(11))
	anchoring := map[string]bool{}
	for i := 0; i < 400; i++ {
		a := m.Step(rng, i)
		if a.Env == nil {
			continue
		}
		anchoring[a.Kind] = anchoring[a.Kind] || a.Anchors
		if !a.MustFail {
			m.Accepted(a, int64(i+1))
		}
	}
	for _, k := range []string{"addNarration", "upsertNote", "deleteNote"} {
		if anchoring[k] {
			t.Errorf("%q anchors, and narration/note events deliberately do not join the "+
				"pool narration anchors are drawn from", k)
		}
	}
	for _, k := range []string{"sceneCreated", "addActor", "placeToken", "moveToken"} {
		if !anchoring[k] {
			t.Errorf("%q never anchored, so narration can never anchor over it", k)
		}
	}
}

// TestAWalkWithNothingAcceptedStillTerminates covers the caller that reports no
// sequences back — a legal thing to do, and the shape a first-draft consumer
// takes before it wires Accepted up. The walk must still draw valid actions
// rather than panicking on an empty anchor pool.
func TestAWalkWithNothingAcceptedStillTerminates(t *testing.T) {
	m := eventgen.New()
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 400; i++ {
		if a := m.Step(rng, i); a.Env != nil && a.Kind == "" {
			t.Fatalf("action #%d emitted an envelope under no kind", i)
		}
	}
}

// TestModelViewsAreCopies stops a caller mutating the model's own slices
// through the accessors it uses to set a scene or a viewer up.
func TestModelViewsAreCopies(t *testing.T) {
	m := eventgen.New()
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 200; i++ {
		a := m.Step(rng, i)
		if a.Env != nil && !a.MustFail {
			m.Accepted(a, int64(i+1))
		}
	}
	actors := m.Actors()
	if len(actors) == 0 {
		t.Fatal("200 actions drew no actor at all")
	}
	actors[0] = "clobbered"
	if m.Actors()[0] == "clobbered" {
		t.Error("Actors() hands out the model's own slice — a caller can rewrite its state")
	}
	scenes := m.Scenes()
	if len(scenes) > 0 {
		scenes[0] = "clobbered"
		if m.Scenes()[0] == "clobbered" {
			t.Error("Scenes() hands out the model's own slice")
		}
	}
	tokens := m.Tokens()
	if len(tokens) > 0 {
		tokens[0] = "clobbered"
		if m.Tokens()[0] == "clobbered" {
			t.Error("Tokens() hands out the model's own slice")
		}
	}
}
