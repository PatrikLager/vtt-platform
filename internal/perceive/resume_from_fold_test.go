package perceive_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

func forwardsAttack(out []*vttv1.Envelope) bool {
	for _, e := range out {
		if e.GetAttackRolled() != nil {
			return true
		}
	}
	return false
}

// VTT-325
func TestAResumedProjectionSeesOnlyWhatItsViewerHoldsAsTheWorldDoes(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", To: &vttv1.GridPosition{X: 2, Y: 1}})
	hero := perceive.Eyes{Shoulder: "hero"}
	v := newViewer(t, hero)
	v.event(envelope(8, &vttv1.NarrationAdded{Text: "the goblin steps in"}), st)
	attack := envelope(9, &vttv1.AttackRolled{AttackerId: "hero", TargetId: "goblin", Total: 7})

	if out := perceive.Resume(hero, v.shown, st).Event(hero, attack, v.shown, st); !forwardsAttack(out) {
		t.Fatalf("a fold holding the goblin as the world does: the attack is withheld: %v", out)
	}

	lacking := v.shown.Snapshot()
	mustFold(t, lacking, &vttv1.Envelope{Sequence: 9, Payload: &vttv1.Envelope_TokenHidden{TokenHidden: &vttv1.TokenHidden{TokenId: "t-gob"}}})
	mustApply(lacking, 10, &vttv1.ActorRemoved{ActorId: "goblin"})
	stale := v.shown.Snapshot()
	mustApply(stale, 9, &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "dazed"})
	another := v.shown.Snapshot()
	mustFold(t, another, &vttv1.Envelope{Sequence: 9, Payload: &vttv1.Envelope_TokenHidden{TokenHidden: &vttv1.TokenHidden{TokenId: "t-gob"}}})
	another.Actors["goblin"].Name = "Imposter"
	for name, shown := range map[string]*engine.State{
		"a fold without the goblin":               lacking,
		"a fold with a condition the world lacks": stale,
		"a fold holding another goblin":           another,
	} {
		if !forwardsAttack(perceive.New(hero, st).Event(hero, attack, shown, st)) {
			t.Fatalf("%s: New withholds the attack too, so this case proves nothing", name)
		}
		out := perceive.Resume(hero, shown, st).Event(hero, attack, shown, st)
		if forwardsAttack(out) {
			t.Errorf("%s: the attack is forwarded before the goblin is corrected: %v", name, out)
		}
		next := shown.Snapshot()
		for i, e := range out {
			if err := engine.Apply(next, e); err != nil {
				t.Fatalf("%s: frame %d (%T) does not fold: %v", name, i, e.GetPayload(), err)
			}
		}
		if got := len(next.Conditions["goblin"]); got != 0 || next.Actors["goblin"] == nil {
			t.Errorf("%s: after the frames the viewer holds the goblin %v with %d condition(s)", name, next.Actors["goblin"] != nil, got)
		}
	}
}

func mustFold(t *testing.T, st *engine.State, env *vttv1.Envelope) {
	t.Helper()
	if err := engine.Apply(st, env); err != nil {
		t.Fatal(err)
	}
}
