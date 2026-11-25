package eventgen

import (
	"math/rand"
	"slices"
	"testing"
)

// Keep open doors that share a column: only they reach the sort's row tie-break.
func doorModel() *Model {
	m := New()
	m.scenes = []string{"scn"}
	m.openDoors["scn"] = map[[2]int32]bool{{3, 1}: true, {3, 4}: true, {3, 2}: true, {1, 9}: true}
	return m
}

func TestACloseDrawsTheSameOpenDoorForTheSameSeed(t *testing.T) {
	closed := func(seed int64) (*Model, [2]int32) {
		m := doorModel()
		at := m.closeDoor(rand.New(rand.NewSource(seed)), 0).Env.GetDoorClosed().GetAt()
		return m, [2]int32{at.GetX(), at.GetY()}
	}
	fromSet := 0
	for seed := int64(1); seed <= 40; seed++ {
		m, first := closed(seed)
		for range 5 {
			if _, again := closed(seed); again != first {
				t.Fatalf("seed %d closed %v, then %v", seed, first, again)
			}
		}
		if len(m.openDoors["scn"]) == 3 {
			fromSet++
			if !doorModel().openDoors["scn"][first] || m.openDoors["scn"][first] {
				t.Fatalf("seed %d closed %v, which the open set did not hold or still holds", seed, first)
			}
		}
	}
	if fromSet == 0 {
		t.Fatal("no seed closed a door the model held open")
	}
}

func TestARemovalWithEveryActorOnTheBoardDrawsNothingOrARefusal(t *testing.T) {
	none := 0
	for seed := int64(1); seed <= 40; seed++ {
		m := New()
		m.actors = []string{"a1", "a2"}
		m.tokenActor = map[string]string{"t1": "a1", "t2": "a2"}
		a := m.removeActor(rand.New(rand.NewSource(seed)))
		switch {
		case a.Env == nil:
			none++
			if a.MustFail || a.Anchors {
				t.Fatalf("seed %d: no event, yet MustFail %t Anchors %t", seed, a.MustFail, a.Anchors)
			}
		case !a.MustFail:
			t.Fatalf("seed %d: removed %s while it holds a token", seed, a.Env.GetActorRemoved().GetActorId())
		}
		if !slices.Equal(m.actors, []string{"a1", "a2"}) {
			t.Fatalf("seed %d: the model now holds %v", seed, m.actors)
		}
	}
	if none == 0 {
		t.Fatal("no seed drew the step with nothing removable")
	}
}

func TestATakeWhenNoActorNamesAnAbilityDrawsARefusedTakeFromAKnownActor(t *testing.T) {
	m := New()
	m.actors = []string{"a1", "a2"}
	a := m.takeAbility(rand.New(rand.NewSource(1)), 7)
	tk := a.Env.GetAbilityTaken()
	if !a.MustFail || a.Anchors || !slices.Contains(m.actors, tk.GetActorId()) || tk.GetAbilityId() != "prop-ability-absent-7" {
		t.Fatalf("drew %v MustFail %t Anchors %t, want a refused take of an absent id from a1 or a2", tk, a.MustFail, a.Anchors)
	}
}

func TestAReferenceDrawAfterARemovalNamesNoRemovedActor(t *testing.T) {
	m := New()
	m.actors = []string{"a1"}
	m.references = map[string][]string{"a1": {"prop-ref-0"}}
	if a := m.removeActor(rand.New(rand.NewSource(1))); a.MustFail || a.Env.GetActorRemoved().GetActorId() != "a1" {
		t.Fatalf("fixture: drew %v MustFail %t, want a1 removed", a.Env, a.MustFail)
	}
	m.actors = append(m.actors, "a2")
	for seed := int64(1); seed <= 20; seed++ {
		rr := m.removeReference(rand.New(rand.NewSource(seed)), 0).Env.GetReferenceRemoved()
		rc := m.changeReference(rand.New(rand.NewSource(seed)), 0).Env.GetReferenceChanged()
		if rr.GetActorId() != "a2" || rc.GetActorId() != "a2" {
			t.Fatalf("seed %d drew a removal from %q and a change of %q, want a2 for both", seed, rr.GetActorId(), rc.GetActorId())
		}
	}
}
