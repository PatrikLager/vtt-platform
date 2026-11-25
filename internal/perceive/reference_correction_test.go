package perceive_test

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// Give referencedAlly no value: only its references may tell its unseen form
// from its whole one.
func referencedAlly() *vttv1.Actor {
	return &vttv1.Actor{ActorId: "ally", Name: "Ally", ModuleId: "mod-ally", Kind: sheetParty,
		References: map[string]*vttv1.Reference{
			"r1": {EntryId: "echo", Place: "mike", State: "lima"},
			"r2": {EntryId: "delta", Place: "kilo"},
		}}
}

func referenceLines(out []*vttv1.Envelope, actorID string) []string {
	var got []string
	for _, e := range out {
		if ra := e.GetReferenceAdded(); ra != nil && ra.GetActorId() == actorID {
			r := ra.GetReference()
			got = append(got, fmt.Sprintf("added %s %s %s/%s", ra.GetReferenceId(), r.GetEntryId(), r.GetPlace(), r.GetState()))
		}
		if rr := e.GetReferenceRemoved(); rr != nil && rr.GetActorId() == actorID {
			got = append(got, "removed "+rr.GetReferenceId())
		}
		if rc := e.GetReferenceChanged(); rc != nil && rc.GetActorId() == actorID {
			got = append(got, fmt.Sprintf("changed %s %s/%s", rc.GetReferenceId(), rc.GetPlace(), rc.GetState()))
		}
	}
	return got
}

func sameReferences(a, b map[string]*vttv1.Reference) bool {
	return proto.Equal(&vttv1.Actor{References: a}, &vttv1.Actor{References: b})
}

func refsOf(st *engine.State, id string) map[string]*vttv1.Reference {
	return st.Actors[id].GetReferences()
}

// VTT-1033
func TestACreatureIsIntroducedWithoutItsReferencesAndIsNeverBroughtThem(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, false))
	tb.step(&vttv1.NarrationAdded{Text: "look"})
	var out []*vttv1.Envelope
	out = append(out, tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "beast", Name: "Beast",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, References: referencedAlly().GetReferences()}})...)
	out = append(out, tb.step(&vttv1.TokenPlaced{TokenId: "t-beast", SceneId: "s", ActorId: "beast",
		Position: &vttv1.GridPosition{X: 2, Y: 1}})...)
	if len(qacsFind(out, qacsAddedOf("beast"))) == 0 {
		t.Fatalf("the creature in sight was never introduced: %v", shapes(out))
	}
	for _, i := range qacsFind(out, qacsAddedOf("beast")) {
		if refs := out[i].GetActorAdded().GetActor().GetReferences(); len(refs) != 0 {
			t.Errorf("the creature was introduced holding %v", refs)
		}
	}
	away := &vttv1.TokenMoved{TokenId: "t-beast", SceneId: "s", From: &vttv1.GridPosition{X: 2, Y: 1}, To: &vttv1.GridPosition{X: 5, Y: 1}}
	if got := referenceLines(tb.step(away), "beast"); len(got) != 0 {
		t.Errorf("%T sent the creature's %v", away, got)
	}
	if perceive.SeesActor(player(), tb.st, "beast") {
		t.Fatalf("fixture: the player still sees the creature at 5,1")
	}
	for _, p := range []proto.Message{
		&vttv1.ReferenceAdded{ActorId: "beast", ReferenceId: "r3", Reference: &vttv1.Reference{EntryId: "echo", Place: "mike", State: "oscar"}},
		&vttv1.ReferenceChanged{ActorId: "beast", ReferenceId: "r1", Place: "mike", State: "oscar"},
		&vttv1.TokenMoved{TokenId: "t-beast", SceneId: "s", From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}},
	} {
		if got := referenceLines(tb.step(p), "beast"); len(got) != 0 {
			t.Errorf("%T sent the creature's %v", p, got)
		}
	}
	if refs := refsOf(tb.v.shown, "beast"); len(refs) != 0 {
		t.Errorf("the player holds the creature holding %v", refs)
	}
}

// VTT-1033
func TestAnUnseenCompanionIsIntroducedWithoutItsReferences(t *testing.T) {
	tb := newSheetTable(t, referencedAlly())
	about := sheetAbout(tb.step(&vttv1.NarrationAdded{Text: "look"}))
	if len(about) == 0 || about[0].GetActorAdded() == nil {
		t.Fatalf("the unseen ally was not introduced first: %v", shapes(about))
	}
	if refs := about[0].GetActorAdded().GetActor().GetReferences(); len(refs) != 0 {
		t.Errorf("the unseen ally was introduced holding %v", refs)
	}
	if refs := refsOf(tb.v.shown, "ally"); len(refs) != 0 {
		t.Errorf("the player holds the unseen ally holding %v", refs)
	}
}

// VTT-1068 VTT-1069
func TestACompanionFirstSeenIsBroughtToItsReferencesAndThenFoldsTheirEvents(t *testing.T) {
	tb := newSheetTable(t, referencedAlly())
	tb.step(&vttv1.NarrationAdded{Text: "look"})
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	if n := arrivals(out, "ally"); n != 0 {
		t.Fatalf("at its first sight the ally was sent %v, want its references added, not it introduced again", shapes(out))
	}
	if got, want := referenceLines(out, "ally"), []string{"added r1 echo mike/lima", "added r2 delta kilo/"}; !sameStrings(got, want) {
		t.Errorf("at its first sight the ally was sent %q, want %q", got, want)
	}
	for _, e := range out {
		if e.GetReferenceAdded() != nil && (e.GetEventId() != "" || e.GetSequence() != tb.seq) {
			t.Errorf("a correction's addition carries event id %q at sequence %d, want none at %d", e.GetEventId(), e.GetSequence(), tb.seq)
		}
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
	if got := referenceLines(tb.step(&vttv1.ReferenceRemoved{ActorId: "ally", ReferenceId: "r1"}), "ally"); !sameStrings(got, []string{"removed r1"}) {
		t.Errorf("the removal in sight sent %q, want it forwarded", got)
	}
	tb.step(&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r2", Place: "mike", State: "oscar"})
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-1068 VTT-1069
func TestReferencesChangedOutOfSightAreCorrectedAtSightInByteOrderAfterTheAbilities(t *testing.T) {
	ally := referencedAlly()
	ally.AbilityIds = []string{"rally"}
	ally.References["Z"] = &vttv1.Reference{EntryId: "echo", Place: "mike", State: "oscar"}
	ally.References["a"] = &vttv1.Reference{EntryId: "delta", Place: "kilo"}
	tb := newSheetTable(t, ally)
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	for _, p := range []proto.Message{
		&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-3", Kind: sheetParty},
		&vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "b", Reference: &vttv1.Reference{EntryId: "echo", Place: "mike", State: "lima"}},
		&vttv1.ReferenceRemoved{ActorId: "ally", ReferenceId: "r2"},
		&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r1", Place: "mike", State: "oscar"},
		&vttv1.AbilityGiven{ActorId: "ally", AbilityId: "dodge"},
		&vttv1.ReferenceRemoved{ActorId: "ally", ReferenceId: "a"},
		&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "Z", Place: "mike", State: "lima"},
		&vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "A", Reference: &vttv1.Reference{EntryId: "delta", Place: "kilo"}},
	} {
		if got := referenceLines(tb.step(p), "ally"); len(got) != 0 {
			t.Fatalf("%T out of sight sent %v", p, got)
		}
	}
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	if n := arrivals(out, "ally"); n != 0 {
		t.Errorf("the ally's return sent %d ActorRemoved or ActorAdded of it: %v", n, shapes(out))
	}
	var got []string
	for _, e := range out {
		if g := e.GetAbilityGiven(); g != nil {
			got = append(got, "given "+g.GetAbilityId())
		}
		got = append(got, referenceLines([]*vttv1.Envelope{e}, "ally")...)
		if g := e.GetActorControlGranted(); g != nil {
			got = append(got, "granted "+g.GetParticipantId())
		}
	}
	want := []string{"given dodge", "added A delta kilo/", "changed Z mike/lima", "removed a",
		"added b echo mike/lima", "changed r1 mike/oscar", "removed r2", "granted p-3"}
	if !sameStrings(got, want) {
		t.Errorf("the ally's return sent\n%q\nwant\n%q", got, want)
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-1068 VTT-1069
func TestAReferenceRemovedAndAddedBackAsAnotherEntryOutOfSightIsRemovedThenAddedAtSight(t *testing.T) {
	tb := newSheetTable(t, referencedAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.ReferenceRemoved{ActorId: "ally", ReferenceId: "r1"})
	tb.step(&vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "r1", Reference: &vttv1.Reference{EntryId: "delta", Place: "mike", State: "lima"}})
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	if got, want := referenceLines(out, "ally"), []string{"removed r1", "added r1 delta mike/lima"}; !sameStrings(got, want) {
		t.Errorf("the ally's return sent %q, want %q", got, want)
	}
	if n := arrivals(out, "ally"); n != 0 {
		t.Errorf("the ally's return sent %d ActorRemoved or ActorAdded of it: %v", n, shapes(out))
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-1068
func TestAReferenceMovedOutOfSightIsChangedAtSightToItsPlaceAndState(t *testing.T) {
	for name, c := range map[string]struct {
		change *vttv1.ReferenceChanged
		want   string
	}{
		"its state alone": {&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r1", Place: "mike", State: "oscar"}, "changed r1 mike/oscar"},
		"its place alone": {&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r2", Place: "mike"}, "changed r2 mike/"},
		"both":            {&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r2", Place: "mike", State: "lima"}, "changed r2 mike/lima"},
	} {
		t.Run(name, func(t *testing.T) {
			tb := newSheetTable(t, referencedAlly())
			tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
			tb.step(c.change)
			out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			if got := referenceLines(out, "ally"); !sameStrings(got, []string{c.want}) {
				t.Errorf("a reference moved to %s out of sight sent %q at sight, want [%s]", name, got, c.want)
			}
			sheetHeldAsTheWorldHoldsIt(t, tb)
		})
	}
}

// VTT-1068 VTT-1069
func TestReferenceChangesUndoneOutOfSightSendNothingAtSight(t *testing.T) {
	tb := newSheetTable(t, referencedAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	for _, p := range []proto.Message{
		&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r1", Place: "kilo"},
		&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r1", Place: "mike", State: "lima"},
		&vttv1.ReferenceRemoved{ActorId: "ally", ReferenceId: "r2"},
		&vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "r2", Reference: &vttv1.Reference{EntryId: "delta", Place: "kilo"}},
	} {
		tb.step(p)
	}
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	if got := referenceLines(out, "ally"); len(got) != 0 {
		t.Errorf("changes undone out of sight sent %q at sight, want nothing", got)
	}
	if n := arrivals(out, "ally"); n != 0 {
		t.Errorf("changes undone out of sight sent %d ActorRemoved or ActorAdded at sight: %v", n, shapes(out))
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-1035
func TestAResumeHoldsTheWorldsReferencesAndStartsAfreshFromALaggingSet(t *testing.T) {
	tb := newSheetTable(t, referencedAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "r3", Reference: &vttv1.Reference{EntryId: "echo", Place: "mike", State: "oscar"}})
	eyes := player()
	attack := envelope(tb.seq+1, &vttv1.AttackRolled{AttackerId: "hero", TargetId: "ally", Total: 7})
	world := tb.st.Snapshot()
	mustFold(t, world, attack)

	shown := tb.v.shown.Snapshot()
	if out := perceive.Resume(eyes, shown, world).Event(eyes, attack, shown, world); !forwardsAttack(out) {
		t.Errorf("a fold holding the world's references does not resume: %v", shapes(out))
	}

	for name, lag := range map[string]func(a *vttv1.Actor){
		"lacking one":          func(a *vttv1.Actor) { delete(a.References, "r3") },
		"holding one too many": func(a *vttv1.Actor) { a.References["r9"] = &vttv1.Reference{EntryId: "delta", Place: "kilo"} },
		"holding another entry": func(a *vttv1.Actor) {
			a.References["r1"] = &vttv1.Reference{EntryId: "delta", Place: "mike", State: "lima"}
		},
		"holding another place": func(a *vttv1.Actor) { a.References["r2"] = &vttv1.Reference{EntryId: "delta", Place: "mike"} },
		"holding another state": func(a *vttv1.Actor) {
			a.References["r1"] = &vttv1.Reference{EntryId: "echo", Place: "mike", State: "oscar"}
		},
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
		if got, w := refsOf(shown, "ally"), refsOf(world, "ally"); !sameReferences(got, w) {
			t.Errorf("a fold %s holds %v after the frames, the world %v", name, got, w)
		}
	}
}

func referencedIn(t *testing.T, kind vttv1.ActorKind) *sheetTable {
	t.Helper()
	tb := newSheetTable(t, sheetAlly(t, false, false))
	tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "who", ModuleId: "mod-who", Kind: kind,
		References: map[string]*vttv1.Reference{"r1": {EntryId: "echo", Place: "mike", State: "lima"}}}})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-who", SceneId: "s", ActorId: "who", Position: &vttv1.GridPosition{X: 2, Y: 1}})
	if !perceive.SeesActor(player(), tb.st, "who") {
		t.Fatalf("fixture: the player does not see the nameless actor")
	}
	return tb
}

// VTT-468
func TestANamelessCreatureHoldingOnlyReferencesMadeAPartyMemberInSightIsIntroducedAfreshWithThem(t *testing.T) {
	tb := referencedIn(t, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
	if refs := refsOf(tb.v.shown, "who"); len(refs) != 0 {
		t.Fatalf("fixture: the log holds the nameless creature holding %v", refs)
	}
	out := tb.step(&vttv1.ActorControlGranted{ActorId: "who", ParticipantId: "p-3", Kind: sheetParty})
	if n := arrivals(out, "who"); n != 2 {
		t.Errorf("the creature made a party member in sight sent %d ActorRemoved or ActorAdded, want both: %v", n, shapes(out))
	}
	if got, w := refsOf(tb.v.shown, "who"), refsOf(tb.st, "who"); !sameReferences(got, w) {
		t.Fatalf("the log holds the party member holding %v, the world %v", got, w)
	}
	if got := referenceLines(tb.step(&vttv1.ReferenceRemoved{ActorId: "who", ReferenceId: "r1"}), "who"); !sameStrings(got, []string{"removed r1"}) {
		t.Errorf("the removal in sight sent %q, want it forwarded", got)
	}
}

// VTT-468
func TestANamelessPartyMemberHoldingOnlyReferencesMadeACreatureInSightIsIntroducedAfreshWithout(t *testing.T) {
	tb := referencedIn(t, sheetParty)
	if got, w := refsOf(tb.v.shown, "who"), refsOf(tb.st, "who"); !sameReferences(got, w) {
		t.Fatalf("fixture: the log holds the nameless party member holding %v, the world %v", got, w)
	}
	out := tb.step(&vttv1.ActorControlGranted{ActorId: "who", ParticipantId: "p-3", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	if n := arrivals(out, "who"); n != 2 {
		t.Errorf("the party member made a creature in sight sent %d ActorRemoved or ActorAdded, want both: %v", n, shapes(out))
	}
	if refs := refsOf(tb.v.shown, "who"); len(refs) != 0 {
		t.Errorf("the log holds the creature holding %v, want none", refs)
	}
}
