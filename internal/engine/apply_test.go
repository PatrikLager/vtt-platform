package engine_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func env(seq int64, payload any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1"}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SessionEnded:
		e.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: p}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.AttackRolled:
		e.Payload = &vttv1.Envelope_AttackRolled{AttackRolled: p}
	case *vttv1.EventsRetracted:
		e.Payload = &vttv1.Envelope_EventsRetracted{EventsRetracted: p}
	case *vttv1.AbilityUsed:
		e.Payload = &vttv1.Envelope_AbilityUsed{AbilityUsed: p}
	case *vttv1.ResourceChanged:
		e.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: p}
	case *vttv1.ConditionApplied:
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: p}
	case *vttv1.ConditionRemoved:
		e.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: p}
	case *vttv1.NarrationAdded:
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: p}
	case *vttv1.NoteUpserted:
		e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: p}
	case *vttv1.NoteDeleted:
		e.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: p}
	case *vttv1.AdventureLoaded:
		e.Payload = &vttv1.Envelope_AdventureLoaded{AdventureLoaded: p}
	case *vttv1.TokenHidden:
		e.Payload = &vttv1.Envelope_TokenHidden{TokenHidden: p}
	case *vttv1.SceneSeen:
		e.Payload = &vttv1.Envelope_SceneSeen{SceneSeen: p}
	}
	return e
}

// actorAddedEnv builds an ActorAdded envelope with optional resources, for
// ResourceChanged test setup.
func actorAddedEnv(seq int64, actorID string, resources map[string]*vttv1.Resource) *vttv1.Envelope {
	return env(seq, &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: actorID, Name: "Hero", ModuleId: "m", Resources: resources},
	})
}

// seedScene applies SessionStarted + SceneCreated and returns the state.
func seedScene(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	must(t, engine.Apply(st, env(1, &vttv1.SessionStarted{Name: "n"})))
	must(t, engine.Apply(st, env(2, &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10})))
	return st
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSceneActorTokenLifecycle(t *testing.T) {
	st := seedScene(t)

	must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
	})))

	must(t, engine.Apply(st, env(4, &vttv1.TokenPlaced{
		TokenId:  "t1",
		SceneId:  "scn",
		ActorId:  "a1",
		Position: &vttv1.GridPosition{X: 3, Y: 7},
	})))

	must(t, engine.Apply(st, env(5, &vttv1.TokenMoved{
		TokenId: "t1",
		SceneId: "scn",
		From:    &vttv1.GridPosition{X: 3, Y: 7},
		To:      &vttv1.GridPosition{X: 5, Y: 8},
	})))

	tok, ok := st.Tokens["t1"]
	if !ok {
		t.Fatal("want token t1 present")
	}
	if tok.X != 5 || tok.Y != 8 {
		t.Fatalf("token position: got (%d,%d), want (5,8)", tok.X, tok.Y)
	}

	if _, ok := st.Scenes["scn"]; !ok {
		t.Fatal("want scene scn present")
	}
	if _, ok := st.Actors["a1"]; !ok {
		t.Fatal("want actor a1 present")
	}

	if len(st.Sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(st.Sessions))
	}
	if st.Sessions[0].EndSeq != 0 {
		t.Fatalf("want open session (EndSeq 0), got EndSeq %d", st.Sessions[0].EndSeq)
	}
}

// TestSessionEndedSuccess pins the success path of SessionEnded: the open
// session (index 0, the common single-session case) gets its EndSeq stamped
// and Apply returns nil. Regression target: engine: openSession() >= 0 is
// used to find the open session by index; an off-by-one on that index
// comparison (i <= 0 instead of i < 0) would reject ending the very first
// session even though it is open.
func TestSessionEndedSuccess(t *testing.T) {
	st := seedScene(t)

	must(t, engine.Apply(st, env(3, &vttv1.SessionEnded{})))

	if len(st.Sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(st.Sessions))
	}
	if st.Sessions[0].EndSeq != 3 {
		t.Fatalf("want session EndSeq 3, got %d", st.Sessions[0].EndSeq)
	}
}

func TestApplyRejections(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T) *engine.State
		envFunc func(st *engine.State) *vttv1.Envelope
	}{
		{
			name:  "duplicate scene id",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.SceneCreated{SceneId: "scn", Name: "Dup", GridWidth: 5, GridHeight: 5})
			},
		},
		{
			name:  "TokenPlaced into unknown scene",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.TokenPlaced{
					TokenId: "t1", SceneId: "missing-scene", ActorId: "a1",
					Position: &vttv1.GridPosition{X: 1, Y: 1},
				})
			},
		},
		{
			name: "TokenPlaced with unknown actor",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				st := seedScene(t)
				return st
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.TokenPlaced{
					TokenId: "t1", SceneId: "scn", ActorId: "missing-actor",
					Position: &vttv1.GridPosition{X: 1, Y: 1},
				})
			},
		},
		{
			name:  "TokenMoved for unknown token",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.TokenMoved{
					TokenId: "missing-token", SceneId: "scn",
					From: &vttv1.GridPosition{X: 0, Y: 0},
					To:   &vttv1.GridPosition{X: 1, Y: 1},
				})
			},
		},
		{
			name: "duplicate actor id",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				st := seedScene(t)
				must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{
					Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
				})))
				return st
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(4, &vttv1.ActorAdded{
					Actor: &vttv1.Actor{ActorId: "a1", Name: "Dup", ModuleId: "m"},
				})
			},
		},
		{
			name: "duplicate token id",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				st := seedScene(t)
				must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{
					Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
				})))
				must(t, engine.Apply(st, env(4, &vttv1.TokenPlaced{
					TokenId: "t1", SceneId: "scn", ActorId: "a1",
					Position: &vttv1.GridPosition{X: 1, Y: 1},
				})))
				return st
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(5, &vttv1.TokenPlaced{
					TokenId: "t1", SceneId: "scn", ActorId: "a1",
					Position: &vttv1.GridPosition{X: 2, Y: 2},
				})
			},
		},
		{
			name: "SessionEnded with no open session",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				return engine.NewState()
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(1, &vttv1.SessionEnded{})
			},
		},
		{
			name:  "second SessionStarted while one is open",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.SessionStarted{Name: "n2"})
			},
		},
		{
			name:  "ResourceChanged for unknown actor",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.ResourceChanged{
					ActorId: "missing-actor", Resource: "pool-a", Delta: 1, NewValue: 1,
				})
			},
		},
		{
			name: "ResourceChanged for unknown resource",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				st := seedScene(t)
				must(t, engine.Apply(st, actorAddedEnv(3, "a1", nil)))
				return st
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(4, &vttv1.ResourceChanged{
					ActorId: "a1", Resource: "pool-a", Delta: 1, NewValue: 1,
				})
			},
		},
		{
			name: "ResourceChanged new_value mismatch",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				st := seedScene(t)
				must(t, engine.Apply(st, actorAddedEnv(3, "a1", map[string]*vttv1.Resource{
					"pool-a": {Current: 5, Max: 10},
				})))
				return st
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				// Correct post-clamp value is 3 (5-2); assert a wrong one.
				return env(4, &vttv1.ResourceChanged{
					ActorId: "a1", Resource: "pool-a", Delta: -2, NewValue: 999,
				})
			},
		},
		{
			name: "duplicate condition (actor,id)",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				st := seedScene(t)
				must(t, engine.Apply(st, actorAddedEnv(3, "a1", nil)))
				st.Conditions["a1"] = []engine.ActorCondition{
					{ID: "cond1", Source: "spell", AppliedSeq: 3},
				}
				return st
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(4, &vttv1.ConditionApplied{
					ActorId: "a1", ConditionId: "cond1", Source: "other",
				})
			},
		},
		{
			name: "ConditionRemoved for absent condition",
			setup: func(t *testing.T) *engine.State {
				t.Helper()
				st := seedScene(t)
				must(t, engine.Apply(st, actorAddedEnv(3, "a1", nil)))
				return st
			},
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(4, &vttv1.ConditionRemoved{
					ActorId: "a1", ConditionId: "cond1", Reason: "cured",
				})
			},
		},
		{
			name:  "ConditionApplied for unknown actor",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.ConditionApplied{
					ActorId: "missing-actor", ConditionId: "cond1", Source: "spell",
				})
			},
		},
		{
			name:  "ConditionRemoved for unknown actor",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.ConditionRemoved{
					ActorId: "missing-actor", ConditionId: "cond1", Reason: "cured",
				})
			},
		},
		{
			name:  "NoteUpserted with empty key",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NoteUpserted{Key: "", Title: "T", Text: "hello"})
			},
		},
		{
			name:  "NoteUpserted with key exceeding 128 bytes",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NoteUpserted{Key: strings.Repeat("k", 129), Title: "T", Text: "hello"})
			},
		},
		{
			name:  "NoteUpserted with title exceeding 256 bytes",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NoteUpserted{Key: "k1", Title: strings.Repeat("t", 257), Text: "hello"})
			},
		},
		{
			name:  "NoteUpserted with empty text",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NoteUpserted{Key: "k1", Title: "T", Text: ""})
			},
		},
		{
			name:  "NoteUpserted with text exceeding 8192 bytes",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NoteUpserted{Key: "k1", Title: "T", Text: strings.Repeat("x", 8193)})
			},
		},
		{
			name:  "NoteDeleted for absent key",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NoteDeleted{Key: "missing-note"})
			},
		},
		{
			name:  "NarrationAdded with empty text",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NarrationAdded{Text: ""})
			},
		},
		{
			name:  "NarrationAdded with text exceeding 8192 bytes",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NarrationAdded{Text: strings.Repeat("x", 8193)})
			},
		},
		{
			name:  "NarrationAdded with negative anchor from",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(20, &vttv1.NarrationAdded{Text: "hello", AnchorFromSeq: -1, AnchorToSeq: 5})
			},
		},
		{
			name:  "NarrationAdded with negative anchor to",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(20, &vttv1.NarrationAdded{Text: "hello", AnchorFromSeq: 1, AnchorToSeq: -5})
			},
		},
		{
			name:  "NarrationAdded with anchor from greater than to",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(100, &vttv1.NarrationAdded{Text: "hello", AnchorFromSeq: 6, AnchorToSeq: 3})
			},
		},
		{
			// Boundary: to == this event's own sequence is still rejected —
			// anchors point strictly backward at recorded history, never at
			// or beyond the narrating event itself.
			name:  "NarrationAdded with anchor to at or after this event's own sequence",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(10, &vttv1.NarrationAdded{Text: "hello", AnchorFromSeq: 3, AnchorToSeq: 10})
			},
		},
		{
			name:  "NarrationAdded with anchor from set and to zero (half-set)",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(20, &vttv1.NarrationAdded{Text: "hello", AnchorFromSeq: 5, AnchorToSeq: 0})
			},
		},
		{
			name:  "NarrationAdded with anchor to set and from zero (half-set)",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(20, &vttv1.NarrationAdded{Text: "hello", AnchorFromSeq: 0, AnchorToSeq: 5})
			},
		},
		{
			// Merge-gate MUST-FIX (overturning the task-level "deliberate"
			// ruling): `as` was the only uncapped participant-writable
			// world-layer field, its effective bound resting silently on
			// coder/websocket's unpinned ~32 KiB default read limit rather
			// than a posture the fold itself owns. One over the new
			// maxNarrationAsBytes cap (256, matching the analogous
			// NoteUpserted.Title cap) must reject.
			name:  "NarrationAdded with as exceeding 256 bytes",
			setup: seedScene,
			envFunc: func(st *engine.State) *vttv1.Envelope {
				return env(3, &vttv1.NarrationAdded{Text: "hello", As: strings.Repeat("a", 257)})
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := c.setup(t)
			before := st.Snapshot()

			err := engine.Apply(st, c.envFunc(st))
			if err == nil {
				t.Fatalf("%s: want non-nil error", c.name)
			}

			after := st.Snapshot()
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("%s: state changed after rejected Apply\nbefore: %+v\nafter:  %+v", c.name, before, after)
			}
			if len(before.Tokens) != len(after.Tokens) {
				t.Fatalf("%s: Tokens count changed after rejected Apply: before %d, after %d", c.name, len(before.Tokens), len(after.Tokens))
			}
			if len(before.Actors) != len(after.Actors) {
				t.Fatalf("%s: Actors count changed after rejected Apply: before %d, after %d", c.name, len(before.Actors), len(after.Actors))
			}
			if len(before.Scenes) != len(after.Scenes) {
				t.Fatalf("%s: Scenes count changed after rejected Apply: before %d, after %d", c.name, len(before.Scenes), len(after.Scenes))
			}
		})
	}
}

func TestAttackRolledIsDeliberateNoOp(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(3, &vttv1.AttackRolled{
		AttackerId: "a1", TargetId: "a2", Expression: "1d20+5", Total: 15,
	})))

	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("AttackRolled must not mutate state")
	}
}

// TestAbilityUsedIsDeliberateNoOp pins AbilityUsed as testimony-only (spec
// §3): rules meaning arrives via the ResourceChanged/ConditionApplied/
// ConditionRemoved events in the same AppendBatch (sub-project 5), not via
// this event's own fold.
func TestAbilityUsedIsDeliberateNoOp(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(3, &vttv1.AbilityUsed{
		ActorId: "a1", AbilityId: "fireball", TargetIds: []string{"a2"},
		OutcomeSummary: "hit",
	})))

	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("AbilityUsed must not mutate state")
	}
}

// TestAdventureLoadedIsDeliberateNoOp pins AdventureLoaded as testimony-only
// (adventure-format spec §3, mirroring AbilityUsed's pattern): it is the
// compile batch's FIRST event, making the log self-describing about what was
// loaded — the actual state changes arrive via the SceneCreated/ActorAdded/
// TokenPlaced/NoteUpserted/NarrationAdded events that follow it in the SAME
// batch (internal/adventure's Compile), never through this event's own fold.
func TestAdventureLoadedIsDeliberateNoOp(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(3, &vttv1.AdventureLoaded{
		AdventureId: "goblin-ambush", Name: "Goblin Ambush",
	})))

	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("AdventureLoaded must not mutate state")
	}
}

// TestNarrationAddedIsDeliberateNoOp pins NarrationAdded as testimony-only
// (world-layer spec §4, mirroring AbilityUsed's pattern): the feed IS the
// log, read back via the existing event streams — the fold validates the
// envelope (a well-formed, validly-anchored narration here) but never
// touches State.
func TestNarrationAddedIsDeliberateNoOp(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(10, &vttv1.NarrationAdded{
		Text: "The goblin cutter snarls.", As: "Goblin Cutter",
		AnchorFromSeq: 3, AnchorToSeq: 5,
	})))

	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("NarrationAdded must not mutate state")
	}
}

// TestNarrationAddedAsAtCapIsAccepted is the boundary-accept counterpart to
// TestApplyRejections' "NarrationAdded with as exceeding 256 bytes" case
// (merge-gate MUST-FIX): exactly maxNarrationAsBytes (256) must be
// accepted, pinning the cap at `>`, not `>=`.
//
// CORRECTION (2026-07-27): this comment used to claim the note key/title/
// text caps "already follow implicitly via their own accept tests". That
// was FALSE — a mutation audit found `>` -> `>=` surviving at apply.go:202,
// :205 and :208, because nothing exercised those caps at their exact value.
// The claim went unchecked for two sub-projects. Their real at-cap tests now
// live in apply_boundary_test.go.
func TestNarrationAddedAsAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(10, &vttv1.NarrationAdded{
		Text: "hello", As: strings.Repeat("a", 256),
	})))

	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("NarrationAdded must not mutate state")
	}
}

// TestResourceChangedAccept covers the three accept sub-variants from the
// task-2 brief: clamp at 0, clamp at max, and an in-range change where
// new_value is asserted exactly (no clamping engaged).
func TestResourceChangedAccept(t *testing.T) {
	cases := []struct {
		name                string
		current, max, delta int32
		wantNewValue        int32
	}{
		{"clamp at 0", 2, 10, -10, 0},
		{"clamp at max", 8, 10, 10, 10},
		{"exact new_value, no clamp engaged", 5, 10, -2, 3},
		{"max == 0 means unlimited: large positive delta does not clamp", 5, 0, 500, 505},
		{"delta == 0 is accepted: new_value unchanged", 7, 10, 0, 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := seedScene(t)
			must(t, engine.Apply(st, actorAddedEnv(3, "a1", map[string]*vttv1.Resource{
				"pool-a": {Current: c.current, Max: c.max},
			})))

			must(t, engine.Apply(st, env(4, &vttv1.ResourceChanged{
				ActorId: "a1", Resource: "pool-a", Delta: c.delta, NewValue: c.wantNewValue,
			})))

			got := st.Actors["a1"].Resources["pool-a"]
			if got.Current != c.wantNewValue {
				t.Fatalf("want current %d, got %d", c.wantNewValue, got.Current)
			}
			if got.Max != c.max {
				t.Fatalf("want max unchanged %d, got %d", c.max, got.Max)
			}
		})
	}
}

func TestConditionAppliedAccept(t *testing.T) {
	st := seedScene(t)
	must(t, engine.Apply(st, actorAddedEnv(3, "a1", nil)))

	must(t, engine.Apply(st, env(4, &vttv1.ConditionApplied{
		ActorId: "a1", ConditionId: "cond1", Source: "spell",
	})))

	got := st.Conditions["a1"]
	want := []engine.ActorCondition{{ID: "cond1", Source: "spell", AppliedSeq: 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

func TestConditionRemovedAccept(t *testing.T) {
	st := seedScene(t)
	must(t, engine.Apply(st, actorAddedEnv(3, "a1", nil)))
	st.Conditions["a1"] = []engine.ActorCondition{
		{ID: "cond1", Source: "spell", AppliedSeq: 3},
		{ID: "cond2", Source: "trap", AppliedSeq: 4},
	}

	must(t, engine.Apply(st, env(5, &vttv1.ConditionRemoved{
		ActorId: "a1", ConditionId: "cond1", Reason: "cured",
	})))

	got := st.Conditions["a1"]
	want := []engine.ActorCondition{{ID: "cond2", Source: "trap", AppliedSeq: 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

// TestNoteUpsertedAccept covers create: a fresh key gets a Note recording
// UpdatedSeq from the envelope's own Sequence, not any caller-supplied
// value (there isn't one — Note has no wire-supplied sequence field).
func TestNoteUpsertedAccept(t *testing.T) {
	st := seedScene(t)

	must(t, engine.Apply(st, env(4, &vttv1.NoteUpserted{
		Key: "town-hollowreach", Title: "Hollowreach", Text: "A river town.",
	})))

	got, ok := st.Notes["town-hollowreach"]
	if !ok {
		t.Fatal("want note town-hollowreach present")
	}
	want := engine.Note{Title: "Hollowreach", Text: "A river town.", UpdatedSeq: 4}
	if got != want {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

// TestNoteUpsertedReplaceIsLastWriteWins covers the replace path: upserting
// an existing key overwrites Title/Text wholesale and re-stamps UpdatedSeq
// from the later event's Sequence — no merge, no history kept in State (the
// log is the history, per spec §4).
func TestNoteUpsertedReplaceIsLastWriteWins(t *testing.T) {
	st := seedScene(t)
	must(t, engine.Apply(st, env(4, &vttv1.NoteUpserted{
		Key: "town-hollowreach", Title: "Hollowreach", Text: "A river town.",
	})))

	must(t, engine.Apply(st, env(9, &vttv1.NoteUpserted{
		Key: "town-hollowreach", Title: "Hollowreach (burned)", Text: "Was a river town.",
	})))

	got := st.Notes["town-hollowreach"]
	want := engine.Note{Title: "Hollowreach (burned)", Text: "Was a river town.", UpdatedSeq: 9}
	if got != want {
		t.Fatalf("want %+v, got %+v", want, got)
	}
}

// TestNoteDeletedAccept covers delete of a present key.
func TestNoteDeletedAccept(t *testing.T) {
	st := seedScene(t)
	must(t, engine.Apply(st, env(4, &vttv1.NoteUpserted{
		Key: "town-hollowreach", Title: "Hollowreach", Text: "A river town.",
	})))

	must(t, engine.Apply(st, env(5, &vttv1.NoteDeleted{Key: "town-hollowreach"})))

	if _, ok := st.Notes["town-hollowreach"]; ok {
		t.Fatal("want note town-hollowreach absent after delete")
	}
}

func TestEventsRetractedIsNoOpInline(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	err := engine.Apply(st, env(3, &vttv1.EventsRetracted{
		FromSequence: 1, ToSequence: 2, Reason: "mistake",
	}))
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	after := st.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("EventsRetracted must not mutate state in-line")
	}
}

func TestUnknownVariantError(t *testing.T) {
	st := engine.NewState()
	err := engine.Apply(st, &vttv1.Envelope{EventId: "u", Sequence: 9})
	if err == nil {
		t.Fatal("want non-nil error for unknown/nil payload variant")
	}
	if !errors.Is(err, engine.ErrUnknownVariant) {
		t.Fatalf("want errors.Is(err, engine.ErrUnknownVariant), got %v", err)
	}
}

func TestSnapshotIsDeepCopy(t *testing.T) {
	st := seedScene(t)
	must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
	})))
	must(t, engine.Apply(st, env(4, &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 1, Y: 1},
	})))
	must(t, engine.Apply(st, env(5, &vttv1.ConditionApplied{
		ActorId: "a1", ConditionId: "cond1", Source: "spell",
	})))
	must(t, engine.Apply(st, env(6, &vttv1.NoteUpserted{
		Key: "town-hollowreach", Title: "Hollowreach", Text: "A river town.",
	})))

	snap := st.Snapshot()

	// Mutate the ORIGINAL after taking the snapshot.
	must(t, engine.Apply(st, env(7, &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 1, Y: 1},
		To:   &vttv1.GridPosition{X: 9, Y: 9},
	})))
	if st.Actors["a1"].Attributes == nil {
		st.Actors["a1"].Attributes = map[string]int32{}
	}
	st.Actors["a1"].Attributes["x"] = 1
	must(t, engine.Apply(st, env(8, &vttv1.ConditionApplied{
		ActorId: "a1", ConditionId: "cond2", Source: "trap",
	})))
	// Overwrite the SAME key post-snapshot: if Snapshot had aliased the map
	// instead of copying it, this would silently corrupt snap's entry too.
	must(t, engine.Apply(st, env(9, &vttv1.NoteUpserted{
		Key: "town-hollowreach", Title: "Hollowreach (burned)", Text: "Was a river town.",
	})))

	snapTok := snap.Tokens["t1"]
	if snapTok.X != 1 || snapTok.Y != 1 {
		t.Fatalf("snapshot token mutated: got (%d,%d), want (1,1)", snapTok.X, snapTok.Y)
	}

	if _, ok := snap.Actors["a1"].Attributes["x"]; ok {
		t.Fatal("snapshot actor attributes mutated: want \"x\" absent")
	}

	snapConds := snap.Conditions["a1"]
	wantConds := []engine.ActorCondition{{ID: "cond1", Source: "spell", AppliedSeq: 5}}
	if !reflect.DeepEqual(snapConds, wantConds) {
		t.Fatalf("snapshot conditions mutated: got %+v, want %+v", snapConds, wantConds)
	}

	snapNote := snap.Notes["town-hollowreach"]
	wantNote := engine.Note{Title: "Hollowreach", Text: "A river town.", UpdatedSeq: 6}
	if snapNote != wantNote {
		t.Fatalf("snapshot note mutated: got %+v, want %+v", snapNote, wantNote)
	}
}
