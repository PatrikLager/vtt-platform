package engine_test

import (
	"reflect"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// VTT-039
func TestTheFoldIgnoresTheEnvelopesRole(t *testing.T) {
	// The same script folded under two roles must give one state. A fold
	// that consulted actor_role would be a second place authorization
	// lives: a role is in the participants table, and the fold reads none.
	script := func() []*vttv1.Envelope {
		return []*vttv1.Envelope{
			env(1, &vttv1.SessionStarted{Name: "n"}),
			env(2, &vttv1.SceneCreated{SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10}),
			actorAddedEnv(3, "a1", nil),
			env(4, &vttv1.TokenPlaced{
				TokenId: "t1", SceneId: "scn", ActorId: "a1",
				Position: &vttv1.GridPosition{X: 3, Y: 7},
			}),
			env(5, &vttv1.TokenMoved{
				TokenId: "t1", SceneId: "scn",
				From: &vttv1.GridPosition{X: 3, Y: 7}, To: &vttv1.GridPosition{X: 5, Y: 8},
			}),
			env(6, &vttv1.NarrationAdded{Text: "x"}),
		}
	}
	var states []*engine.State
	for _, role := range []string{"dm", "spectator"} {
		st := engine.NewState()
		for _, e := range script() {
			e.ActorRole = role
			must(t, engine.Apply(st, e))
		}
		states = append(states, st)
	}
	if !reflect.DeepEqual(states[0], states[1]) {
		t.Fatal("the fold's result depends on the envelope's actor_role — a role lives beside " +
			"the credential, and the fold holds none")
	}
}
