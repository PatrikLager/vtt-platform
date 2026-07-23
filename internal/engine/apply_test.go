package engine_test

import (
	"errors"
	"reflect"
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
	}
	return e
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

	snap := st.Snapshot()

	// Mutate the ORIGINAL after taking the snapshot.
	must(t, engine.Apply(st, env(5, &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 1, Y: 1},
		To:   &vttv1.GridPosition{X: 9, Y: 9},
	})))
	if st.Actors["a1"].Attributes == nil {
		st.Actors["a1"].Attributes = map[string]int32{}
	}
	st.Actors["a1"].Attributes["x"] = 1

	snapTok := snap.Tokens["t1"]
	if snapTok.X != 1 || snapTok.Y != 1 {
		t.Fatalf("snapshot token mutated: got (%d,%d), want (1,1)", snapTok.X, snapTok.Y)
	}

	if _, ok := snap.Actors["a1"].Attributes["x"]; ok {
		t.Fatal("snapshot actor attributes mutated: want \"x\" absent")
	}
}
