package adventure_test

import (
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// TestCompileValidFixtureExactEnvelopeList is the hand-derived golden the
// brief requires: every field of every envelope Compile produces for
// testdata/valid, in the EXACT binding order (AdventureLoaded; scenes,
// file-name order; actors, file-name order; placements, scene order then
// declared order; notes, declared order; opening narration last).
// Envelopes carry payloads only — asserted separately below.
//
// "brace-yard" has FIVE scenes (cellar/gate/hall/loft/yard) and THREE
// actors (review-fix, p12-task-2 fix wave item 1): a one-scene fixture
// cannot distinguish a correct file-name-order slice walk from a broken
// map-iteration one — with 5 scenes (5! = 120 orderings) a map-iteration
// mutation of Compile is overwhelmingly unlikely to coincidentally match
// this golden by chance, so this test actually pins the ordering claim
// instead of trivially passing regardless of it.
func TestCompileValidFixtureExactEnvelopeList(t *testing.T) {
	rs := loadFixtureRuleset(t)
	adv, err := adventure.Load("testdata/valid", rs)
	if err != nil {
		t.Fatal(err)
	}
	st := engine.NewState()

	got, err := adventure.Compile(adv, st)
	if err != nil {
		t.Fatal(err)
	}

	want := []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_AdventureLoaded{AdventureLoaded: &vttv1.AdventureLoaded{
			AdventureId: "brace-yard", Name: "Brace Yard",
		}}},
		// scenes/*.json file-name order: cellar, gate, hall, loft, yard.
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "cellar", Name: "The Cellar", GridWidth: 6, GridHeight: 6,
		}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "gate", Name: "The Gate", GridWidth: 8, GridHeight: 8,
		}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "hall", Name: "The Hall", GridWidth: 7, GridHeight: 7,
		}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "loft", Name: "The Loft", GridWidth: 5, GridHeight: 5,
		}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "yard", Name: "The Yard", GridWidth: 10, GridHeight: 10,
		}}},
		// actors/*.json file-name order: brace-guard, grit-scout, vim-fighter.
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{
				ActorId:    "brace-guard",
				Name:       "Brace Guard",
				Attributes: map[string]int32{"vim": 10, "vigor": 10, "brace": 14},
				Resources:  map[string]*vttv1.Resource{"focus": {Current: 10, Max: 10}},
			},
		}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{
				ActorId:    "grit-scout",
				Name:       "Grit Scout",
				Attributes: map[string]int32{"vim": 8, "vigor": 16, "brace": 8},
				Resources:  map[string]*vttv1.Resource{"focus": {Current: 6, Max: 8}},
			},
		}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: &vttv1.Actor{
				ActorId:    "vim-fighter",
				Name:       "Vim Fighter",
				Attributes: map[string]int32{"vim": 14, "vigor": 12, "brace": 10},
				Resources:  map[string]*vttv1.Resource{"focus": {Current: 8, Max: 10}},
			},
		}}},
		// placements: scene order (cellar, gate, hall, loft, yard), then
		// declared (array) order within each scene.
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-scout", SceneId: "cellar", ActorId: "grit-scout",
			Position: &vttv1.GridPosition{X: 1, Y: 1},
		}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-vim-2", SceneId: "gate", ActorId: "vim-fighter",
			Position: &vttv1.GridPosition{X: 0, Y: 0},
		}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-brace-2", SceneId: "hall", ActorId: "brace-guard",
			Position: &vttv1.GridPosition{X: 2, Y: 2},
		}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-scout-2", SceneId: "loft", ActorId: "grit-scout",
			Position: &vttv1.GridPosition{X: 3, Y: 3},
		}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-vim", SceneId: "yard", ActorId: "vim-fighter",
			Position: &vttv1.GridPosition{X: 2, Y: 3},
		}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-brace", SceneId: "yard", ActorId: "brace-guard",
			Position: &vttv1.GridPosition{X: 5, Y: 5},
		}}},
		{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key: "yard-rumor", Title: "Yard Rumor", Text: "Something stirs behind the well.",
		}}},
		{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
			Text: "The yard is quiet before the bell rings.",
		}}},
	}

	assertEnvelopes(t, got, want)

	for i, e := range got {
		if e.EventId != "" || e.Sequence != 0 || e.SessionId != "" ||
			e.ActorRole != "" || e.ParticipantId != "" || e.OccurredAt != nil {
			t.Errorf("envelope[%d] carries more than a payload (sequence/session/participant/event_id/actor_role/occurred_at must all be zero — the caller of AppendBatch stamps them): %v", i, e)
		}
	}
}

func assertEnvelopes(t *testing.T, got, want []*vttv1.Envelope) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d envelopes, want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("envelope[%d]:\n got  %v\n want %v", i, got[i], want[i])
		}
	}
}

// TestCompileIsDeterministic pins the binding "two Compiles → deep-equal"
// requirement: Compile performs no map iteration that reaches the output
// order (every top-level walk is over Adventure's own slices, in Load
// order), so calling it repeatedly against the same (adv, st) must always
// produce identical results. Runs 10 rounds (not just 2) against the
// 5-scene/3-actor fixture (review-fix, p12-task-2 fix wave item 1) — Go's
// map iteration order is re-randomized per range statement, so a
// map-based regression would very likely disagree with itself across
// several rounds even where two calls happened to agree once.
func TestCompileIsDeterministic(t *testing.T) {
	rs := loadFixtureRuleset(t)
	adv, err := adventure.Load("testdata/valid", rs)
	if err != nil {
		t.Fatal(err)
	}
	st := engine.NewState()

	first, err := adventure.Compile(adv, st)
	if err != nil {
		t.Fatal(err)
	}
	for round := 1; round < 10; round++ {
		got, err := adventure.Compile(adv, st)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(first) {
			t.Fatalf("round %d: got %d envelopes, want %d (same as round 0)", round, len(got), len(first))
		}
		for i := range first {
			if !proto.Equal(got[i], first[i]) {
				t.Errorf("round %d, envelope[%d]:\n got  %v\n want %v (round 0's result)", round, i, got[i], first[i])
			}
		}
	}
}

// TestCompileDoesNotMutateState mirrors internal/engine's own no-op-arm
// tests: Compile is a pure function over (adv, st) — it must not touch st.
func TestCompileDoesNotMutateState(t *testing.T) {
	rs := loadFixtureRuleset(t)
	adv, err := adventure.Load("testdata/valid", rs)
	if err != nil {
		t.Fatal(err)
	}
	st := engine.NewState()
	before := st.Snapshot()

	if _, err := adventure.Compile(adv, st); err != nil {
		t.Fatal(err)
	}
	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Compile must not mutate st")
	}
}

// TestCompileCollisions pins the binding "collision checks vs st BEFORE any
// output" requirement, one seeded collision at a time (scene, actor, token,
// note — every id/key kind Compile could emit): a colliding id/key rejects
// the whole call and emits NOTHING, not a partial batch.
func TestCompileCollisions(t *testing.T) {
	rs := loadFixtureRuleset(t)
	adv, err := adventure.Load("testdata/valid", rs)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		seed    func(*engine.State)
		wantErr string
	}{
		{
			name: "scene id collision",
			seed: func(st *engine.State) {
				st.Scenes["yard"] = engine.Scene{ID: "yard", Name: "Existing Yard", GridWidth: 4, GridHeight: 4}
			},
			wantErr: `scene id "yard"`,
		},
		{
			name: "actor id collision",
			seed: func(st *engine.State) {
				st.Actors["brace-guard"] = &vttv1.Actor{ActorId: "brace-guard", Name: "Existing"}
			},
			wantErr: `actor id "brace-guard"`,
		},
		{
			name: "token id collision",
			seed: func(st *engine.State) {
				st.Tokens["tok-vim"] = engine.Token{ID: "tok-vim", SceneID: "elsewhere", ActorID: "someone-else", X: 1, Y: 1}
			},
			wantErr: `token id "tok-vim"`,
		},
		{
			name: "note key collision",
			seed: func(st *engine.State) {
				st.Notes["yard-rumor"] = engine.Note{Title: "Existing", Text: "already here"}
			},
			wantErr: `note key "yard-rumor"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := engine.NewState()
			c.seed(st)

			envs, err := adventure.Compile(adv, st)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), c.wantErr)
			}
			if envs != nil {
				t.Errorf("want nil envelopes on collision (nothing emitted), got %d", len(envs))
			}
		})
	}
}
