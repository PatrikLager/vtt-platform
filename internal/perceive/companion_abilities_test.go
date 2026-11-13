package perceive_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// abilityAlly is a party member whose sheet names abilities and holds no
// value, so its list alone tells its unseen form from its whole one.
func abilityAlly() *vttv1.Actor {
	return &vttv1.Actor{ActorId: "ally", Name: "Ally", ModuleId: "mod-ally", Kind: sheetParty,
		AbilityIds: []string{"rally", "fists"}}
}

// VTT-698
func TestACreatureIsIntroducedWithoutItsAbilities(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, false))
	tb.step(&vttv1.NarrationAdded{Text: "look"})
	var out []*vttv1.Envelope
	out = append(out, tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "beast", Name: "Beast",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, AbilityIds: []string{"fists"}}})...)
	out = append(out, tb.step(&vttv1.TokenPlaced{TokenId: "t-beast", SceneId: "s", ActorId: "beast",
		Position: &vttv1.GridPosition{X: 2, Y: 1}})...)
	if len(qacsFind(out, qacsAddedOf("beast"))) == 0 {
		t.Fatalf("the creature in sight was never introduced: %v", out)
	}
	for _, i := range qacsFind(out, qacsAddedOf("beast")) {
		if ids := out[i].GetActorAdded().GetActor().GetAbilityIds(); len(ids) != 0 {
			t.Errorf("the creature was introduced naming %q", ids)
		}
	}
	if ids := tb.v.shown.Actors["beast"].GetAbilityIds(); len(ids) != 0 {
		t.Errorf("the player holds the creature naming %q", ids)
	}
}

// VTT-698
func TestAnUnseenCompanionIsIntroducedWithoutItsAbilities(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	about := sheetAbout(tb.step(&vttv1.NarrationAdded{Text: "look"}))
	if len(about) == 0 || about[0].GetActorAdded() == nil {
		t.Fatalf("the unseen ally was not introduced first: %v", about)
	}
	if ids := about[0].GetActorAdded().GetActor().GetAbilityIds(); len(ids) != 0 {
		t.Errorf("the unseen ally was introduced naming %q", ids)
	}
	if ids := tb.v.shown.Actors["ally"].GetAbilityIds(); len(ids) != 0 {
		t.Errorf("the player holds the unseen ally naming %q", ids)
	}
}

// VTT-699
func TestACompanionNamingOnlyAbilitiesIsBroughtWholeAtItsFirstSight(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	tb.step(&vttv1.NarrationAdded{Text: "look"})
	about := sheetAbout(tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor}))
	added := qacsFind(about, qacsAddedOf("ally"))
	if len(added) != 1 || about[0].GetActorRemoved() == nil {
		t.Fatalf("at its first sight the ally was sent %v, want it withdrawn and introduced again", shapes(about))
	}
	if a := about[added[0]].GetActorAdded().GetActor(); !proto.Equal(a, qacsWhole(tb.st.Actors["ally"])) {
		t.Errorf("at its first sight the ally was introduced as %v, want it whole", a)
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-699
func TestACompanionInSightIsIntroducedWithItsAbilities(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	sheetHeldAsTheWorldHoldsIt(t, tb)
	if ids := tb.v.shown.Actors["ally"].GetAbilityIds(); !sameStrings(ids, []string{"rally", "fists"}) {
		t.Errorf("the player holds the ally in sight naming %q, want [rally fists]", ids)
	}
}

// VTT-699
func TestAResumedLogHoldingACompanionWithoutItsAbilitiesIsBroughtToThem(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	tb.step(&vttv1.NarrationAdded{Text: "look"})
	tb.step(&vttv1.ConditionRemoved{ActorId: "ally", ConditionId: "marked"})
	shown := tb.v.shown.Snapshot()
	tb.seq++
	mustApply(tb.st, tb.seq, &vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	resumed := perceive.Resume(player(), shown, tb.st)

	tb.seq++
	change := envelope(tb.seq, &vttv1.ConditionApplied{ActorId: "ally", ConditionId: "dazed"})
	change.EventId = "evt-change"
	if err := engine.Apply(tb.st, change); err != nil {
		t.Fatalf("fixture: the world refused the change: %v", err)
	}
	out := resumed.Event(player(), change, shown, tb.st)
	for i, e := range out {
		if err := engine.Apply(shown, e); err != nil {
			t.Fatalf("frame %d (%T) does not fold onto the resumed log: %v\nframes: %v", i, e.GetPayload(), err, out)
		}
	}
	if ids := shown.Actors["ally"].GetAbilityIds(); !sameStrings(ids, []string{"rally", "fists"}) {
		t.Errorf("the resumed log holds the ally naming %q, want [rally fists]: %v", ids, out)
	}
}

// VTT-699 VTT-707
func TestACompanionAddedAgainWithAnotherListOfItsLengthIsSeenWithIt(t *testing.T) {
	for _, list := range [][]string{{"shove", "dodge"}, {"fists", "rally"}} {
		t.Run(list[0], func(t *testing.T) {
			tb := newSheetTable(t, abilityAlly())
			tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			if ids := tb.v.shown.Actors["ally"].GetAbilityIds(); !sameStrings(ids, []string{"rally", "fists"}) {
				t.Fatalf("fixture: the player holds the ally in sight naming %q, want [rally fists]", ids)
			}
			tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
			again := abilityAlly()
			again.AbilityIds = list
			for _, p := range []proto.Message{
				&vttv1.TokenRemoved{TokenId: "t-ally"},
				&vttv1.ActorRemoved{ActorId: "ally"},
				&vttv1.ActorAdded{Actor: again},
				&vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally",
					Position: &vttv1.GridPosition{X: 5, Y: 1}},
				&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2", Kind: sheetParty},
				&vttv1.DoorOpened{SceneId: "s", At: sheetDoor},
			} {
				tb.step(p)
			}
			if ids := tb.v.shown.Actors["ally"].GetAbilityIds(); !sameStrings(ids, list) {
				t.Errorf("the player holds the ally added again naming %q, want %q", ids, list)
			}
			sheetHeldAsTheWorldHoldsIt(t, tb)
		})
	}
}
