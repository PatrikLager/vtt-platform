package perceive_test

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// resourceLines are the frames about actorID that add, remove or change a
// resource or a condition, each as "<kind> <name>" with its value, in order.
func resourceLines(out []*vttv1.Envelope, actorID string) []string {
	var got []string
	for _, e := range out {
		switch p := e.GetPayload().(type) {
		case *vttv1.Envelope_ResourceAdded:
			if p.ResourceAdded.GetActorId() == actorID {
				v := p.ResourceAdded.GetValue()
				got = append(got, fmt.Sprintf("added %s %d/%d", p.ResourceAdded.GetResource(), v.GetCurrent(), v.GetMax()))
			}
		case *vttv1.Envelope_ResourceRemoved:
			if p.ResourceRemoved.GetActorId() == actorID {
				got = append(got, "removed "+p.ResourceRemoved.GetResource())
			}
		case *vttv1.Envelope_ResourceChanged:
			if p.ResourceChanged.GetActorId() == actorID {
				got = append(got, fmt.Sprintf("changed %s %d", p.ResourceChanged.GetResource(), p.ResourceChanged.GetNewValue()))
			}
		case *vttv1.Envelope_ConditionApplied:
			if p.ConditionApplied.GetActorId() == actorID {
				got = append(got, "condition "+p.ConditionApplied.GetConditionId())
			}
		}
	}
	return got
}

func sameLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// VTT-779 VTT-781
func TestAResourceAddedOrRemovedInSightIsForwardedAloneAndHeld(t *testing.T) {
	for name, change := range map[string]proto.Message{
		"an addition": &vttv1.ResourceAdded{ActorId: "ally", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 2}},
		"a removal":   &vttv1.ResourceRemoved{ActorId: "ally", Resource: "pool"},
	} {
		t.Run(name, func(t *testing.T) {
			tb := newSheetTable(t, sheetAlly(t, false, true))
			tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			out := sheetAbout(tb.step(change))
			if len(out) != 1 || out[0].GetEventId() == "" {
				t.Fatalf("%s in sight sent %v, want the event alone", name, shapes(out))
			}
			if n := arrivals(out, "ally"); n != 0 {
				t.Errorf("%s in sight sent %d ActorRemoved or ActorAdded of the ally", name, n)
			}
			for _, p := range []proto.Message{&vttv1.NarrationAdded{Text: "later"}, &vttv1.DoorClosed{SceneId: "s", At: sheetDoor}} {
				if about := sheetAbout(tb.step(p)); len(about) != 0 {
					t.Errorf("%T after %s sent %v about the ally, want nothing", p, name, shapes(about))
				}
			}
			sheetHeldAsTheWorldHoldsIt(t, tb)
		})
	}
}

// VTT-780 VTT-781 VTT-796
func TestResourcesChangedOutOfSightAreCorrectedAtSightInByteOrderBeforeConditions(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, true))
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	for _, p := range []proto.Message{
		&vttv1.ConditionApplied{ActorId: "ally", ConditionId: "dazed"},
		&vttv1.ResourceAdded{ActorId: "ally", Resource: "zeal", Value: &vttv1.Resource{Current: 3}},
		&vttv1.ResourceRemoved{ActorId: "ally", Resource: "pool"},
		&vttv1.ResourceAdded{ActorId: "ally", Resource: "pool", Value: &vttv1.Resource{Current: 2, Max: 4}},
		&vttv1.ResourceAdded{ActorId: "ally", Resource: "focus", Value: &vttv1.Resource{Current: 0, Max: 1}},
	} {
		if about := sheetAbout(tb.step(p)); len(about) != 0 {
			t.Fatalf("%T out of sight sent %v about the ally", p, shapes(about))
		}
	}
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	if n := arrivals(out, "ally"); n != 0 {
		t.Errorf("the ally's return sent %d ActorRemoved or ActorAdded of it: %v", n, shapes(out))
	}
	want := []string{"added focus 0/1", "removed pool", "added pool 2/4", "added zeal 3/0", "condition dazed"}
	if got := resourceLines(out, "ally"); !sameLines(got, want) {
		t.Errorf("the ally's return sent\n%v\nwant\n%v", got, want)
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-780
func TestAResourceRemovedOutOfSightIsRemovedAtSightAndOneWhoseCurrentMovedIsChanged(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, true))
	tb.step(&vttv1.ResourceAdded{ActorId: "ally", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 3}})
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.ResourceRemoved{ActorId: "ally", Resource: "focus"})
	tb.step(&vttv1.ResourceChanged{ActorId: "ally", Resource: "pool", Delta: -2, NewValue: 3})
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	if got, want := resourceLines(out, "ally"), []string{"removed focus", "changed pool 3"}; !sameLines(got, want) {
		t.Errorf("the ally's return sent %v, want %v", got, want)
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-796 VTT-224 VTT-782
func TestACurrentNoResourceChangedCanLandOnIsCorrectedByARemovalAndAnAddition(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, true))
	tb.step(&vttv1.ResourceAdded{ActorId: "ally", Resource: "nerve", Value: &vttv1.Resource{Current: 2}})
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	for _, p := range []proto.Message{
		&vttv1.ResourceRemoved{ActorId: "ally", Resource: "nerve"},
		&vttv1.ResourceAdded{ActorId: "ally", Resource: "nerve", Value: &vttv1.Resource{Current: -1}},
		&vttv1.ResourceRemoved{ActorId: "ally", Resource: "pool"},
		&vttv1.ResourceAdded{ActorId: "ally", Resource: "pool", Value: &vttv1.Resource{Current: 12, Max: 9}},
	} {
		tb.step(p)
	}
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	want := []string{"removed nerve", "added nerve -1/0", "removed pool", "added pool 12/9"}
	if got := resourceLines(out, "ally"); !sameLines(got, want) {
		t.Errorf("the ally's return sent %v, want %v", got, want)
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)

	// VTT-782: held at the world's values, it is sent nothing again and resumes.
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	if got := resourceLines(tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor}), "ally"); len(got) != 0 {
		t.Errorf("the ally's second return, unchanged, sent %v", got)
	}
	eyes := player()
	attack := envelope(tb.seq+1, &vttv1.AttackRolled{AttackerId: "hero", TargetId: "ally", Total: 7})
	world := tb.st.Snapshot()
	mustFold(t, world, attack)
	shown := tb.v.shown.Snapshot()
	if out := perceive.Resume(eyes, shown, world).Event(eyes, attack, shown, world); !forwardsAttack(out) {
		t.Errorf("a fold holding currents outside the range at the world's values does not resume: %v", shapes(out))
	}
}

// VTT-783
func TestAPartyMemberAddedAgainWithOtherResourcesIsCorrectedNotIntroducedAfresh(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, true))
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.TokenRemoved{TokenId: "t-ally"})
	tb.step(&vttv1.ActorRemoved{ActorId: "ally"})
	again := sheetAlly(t, false, false)
	again.Resources = map[string]*vttv1.Resource{"focus": {Current: 1, Max: 1}}
	tb.step(&vttv1.ActorAdded{Actor: again})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	tb.step(&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2", Kind: sheetParty})
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	if n := arrivals(out, "ally"); n != 0 {
		t.Errorf("the ally added again with other resources sent %d ActorRemoved or ActorAdded: %v", n, shapes(out))
	}
	if got, want := resourceLines(out, "ally"), []string{"added focus 1/1", "removed pool"}; !sameLines(got, want) {
		t.Errorf("the ally added again was corrected by %v, want %v", got, want)
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-468: nameless, so its name does not already tell the two forms apart.
func TestANamelessPartyMemberHoldingOnlyResourcesThatBecomesACreatureInSightIsIntroducedAfresh(t *testing.T) {
	tb := newSheetTable(t, &vttv1.Actor{ActorId: "ally", ModuleId: "mod-ally", Kind: sheetParty,
		Resources: map[string]*vttv1.Resource{"pool": {Current: 5, Max: 9}}})
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	out := tb.step(&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	if n := arrivals(out, "ally"); n != 2 {
		t.Fatalf("the ally becoming a creature in sight sent %d ActorRemoved or ActorAdded, want both: %v", n, shapes(out))
	}
	if held := tb.v.shown.Actors["ally"]; len(held.GetResources()) != 0 {
		t.Errorf("the log holds the creature with resources %v, want none", held.GetResources())
	}
}

// VTT-779
func TestACreaturesResourcesChangedOutOfSightAreNotCorrectedAtSight(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, true))
	tb.step(&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.ResourceAdded{ActorId: "ally", Resource: "focus", Value: &vttv1.Resource{Current: 1}})
	tb.step(&vttv1.ResourceRemoved{ActorId: "ally", Resource: "pool"})
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	for _, line := range resourceLines(out, "ally") {
		if line != "condition marked" {
			t.Errorf("the creature's return sent %q", line)
		}
	}
	if held := tb.v.shown.Actors["ally"]; len(held.GetResources()) != 0 {
		t.Errorf("the log holds the creature with resources %v, want none", held.GetResources())
	}
}

// VTT-782
func TestAResumeHoldsTheWorldsResourcesAndStartsAfreshFromALaggingSet(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, true))
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.ResourceAdded{ActorId: "ally", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 2}})
	eyes := player()
	attack := envelope(tb.seq+1, &vttv1.AttackRolled{AttackerId: "hero", TargetId: "ally", Total: 7})
	world := tb.st.Snapshot()
	mustFold(t, world, attack)

	shown := tb.v.shown.Snapshot()
	if out := perceive.Resume(eyes, shown, world).Event(eyes, attack, shown, world); !forwardsAttack(out) {
		t.Errorf("a fold holding the world's resources does not resume: %v", shapes(out))
	}

	for name, lag := range map[string]func(a *vttv1.Actor){
		"lacking one":             func(a *vttv1.Actor) { delete(a.Resources, "focus") },
		"holding one too many":    func(a *vttv1.Actor) { a.Resources["zeal"] = &vttv1.Resource{Current: 1} },
		"holding another max":     func(a *vttv1.Actor) { a.Resources["focus"] = &vttv1.Resource{Current: 1, Max: 9} },
		"holding another current": func(a *vttv1.Actor) { a.Resources["focus"] = &vttv1.Resource{Current: 0, Max: 2} },
	} {
		shown := tb.v.shown.Snapshot()
		lag(shown.Actors["ally"])
		if !forwardsAttack(perceive.New(eyes, world).Event(eyes, attack, shown, world)) {
			t.Fatalf("%s: New withholds the attack too, so this case proves nothing", name)
		}
		out := perceive.Resume(eyes, shown, world).Event(eyes, attack, shown, world)
		if forwardsAttack(out) {
			t.Errorf("a fold %s resumes: %v", name, shapes(out))
		}
		if n := arrivals(out, "ally"); n != 0 {
			t.Errorf("a fold %s is sent %d ActorRemoved or ActorAdded of the ally: %v", name, n, shapes(out))
		}
		for i, e := range out {
			if err := engine.Apply(shown, e); err != nil {
				t.Fatalf("%s: frame %d (%T) does not fold: %v", name, i, e.GetPayload(), err)
			}
		}
		if got, w := shown.Actors["ally"].GetResources(), world.Actors["ally"].GetResources(); len(got) != len(w) {
			t.Errorf("a fold %s holds %v after the frames, the world %v", name, got, w)
		}
	}
}
