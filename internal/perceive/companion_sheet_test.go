package perceive_test

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const sheetParty = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER

var sheetDoor = &vttv1.GridPosition{X: 3, Y: 1}

type sheetTable struct {
	t   *testing.T
	st  *engine.State
	v   *testViewer
	seq int64
}

// Keep the ally in the world before the player's first event: that is how a
// log's opening finds a party member (VTT-663).
func newSheetTable(t *testing.T, ally *vttv1.Actor) *sheetTable {
	t.Helper()
	tb := &sheetTable{t: t, st: engine.NewState(), v: newViewer(t, player())}
	for _, p := range []proto.Message{
		&vttv1.SessionStarted{Name: "n"},
		&vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3,
			Tiles: twoRoomsTiles()},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "Hero", Kind: sheetParty}},
		&vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero",
			Position: &vttv1.GridPosition{X: 1, Y: 1}},
		&vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1", Kind: sheetParty},
		&vttv1.ActorAdded{Actor: ally},
		&vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally",
			Position: &vttv1.GridPosition{X: 5, Y: 1}},
		&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2", Kind: sheetParty},
		&vttv1.ConditionApplied{ActorId: "ally", ConditionId: "marked"},
	} {
		tb.seq++
		mustApply(tb.st, tb.seq, p)
	}
	return tb
}

// Keep the event id: it tells a forwarded frame from a built one.
func (tb *sheetTable) step(payload proto.Message) []*vttv1.Envelope {
	tb.t.Helper()
	tb.seq++
	env := envelope(tb.seq, payload)
	env.EventId = fmt.Sprintf("evt-%d", tb.seq)
	if err := engine.Apply(tb.st, env); err != nil {
		tb.t.Fatalf("fixture: the world refused event %d: %v", tb.seq, err)
	}
	return tb.v.event(env, tb.st)
}

func sheetAlly(t *testing.T, attributes, resources bool) *vttv1.Actor {
	t.Helper()
	a := &vttv1.Actor{ActorId: "ally", Name: "Ally", ModuleId: "mod-ally", Kind: sheetParty}
	if attributes {
		motto, err := structpb.NewStruct(map[string]any{"motto": "steady"})
		if err != nil {
			t.Fatal(err)
		}
		a.Attributes, a.ModuleData = map[string]int32{"heft": 4}, motto
	}
	if resources {
		a.Resources = map[string]*vttv1.Resource{"pool": {Current: 5, Max: 9}}
	}
	return a
}

func sheetAbout(out []*vttv1.Envelope) []*vttv1.Envelope {
	var got []*vttv1.Envelope
	for _, e := range out {
		if qacsActorOf(e) == "ally" {
			got = append(got, e)
		}
	}
	return got
}

func sheetHeldAsTheWorldHoldsIt(t *testing.T, tb *sheetTable) {
	t.Helper()
	if held, w := tb.v.shown.Actors["ally"], tb.st.Actors["ally"]; !proto.Equal(held, w) {
		t.Errorf("the player holds the ally as\n  %v\nthe world as\n  %v", held, w)
	}
	if held, w := qacsConds(tb.v.shown, "ally"), qacsConds(tb.st, "ally"); !qacsSame(held, w) {
		t.Errorf("the player holds the ally with %v, the world with %v", held, w)
	}
}

// VTT-658 VTT-660 VTT-661
func TestACompanionItsCharacterHasNotSeenArrivesWithoutItsSheet(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, true, true))
	about := sheetAbout(tb.step(&vttv1.NarrationAdded{Text: "look"}))
	if len(about) == 0 || about[0].GetActorAdded() == nil {
		t.Fatalf("the unseen ally was not introduced first: %v", about)
	}
	if a := about[0].GetActorAdded().GetActor(); !proto.Equal(a, qacsUnseen(tb.st.Actors["ally"])) {
		t.Errorf("the unseen ally was introduced as %v, want its id, name, module and kind alone", a)
	}
	want := []string{"*vttv1.Envelope_ActorAdded ally", "*vttv1.Envelope_ActorControlGranted ally"}
	if got := shapes(about); !sameStrings(got, want) {
		t.Fatalf("the unseen ally arrived as\n%v\nwant\n%v", got, want)
	}
	if g := about[1].GetActorControlGranted(); g.GetParticipantId() != "p-2" || g.GetKind() != sheetParty {
		t.Errorf("the unseen ally's introduction is followed by %v, want its grant to p-2", g)
	}
	for _, p := range []proto.Message{
		&vttv1.ResourceChanged{ActorId: "ally", Resource: "pool", Delta: -2, NewValue: 3},
		&vttv1.ConditionApplied{ActorId: "ally", ConditionId: "dazed"},
		&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-3", Kind: sheetParty},
		&vttv1.ActorControlRevoked{ActorId: "ally", ParticipantId: "p-2"},
		&vttv1.NarrationAdded{Text: "later"},
	} {
		out := tb.step(p)
		removed := false
		for _, e := range out {
			removed = removed || e.GetActorRemoved() != nil
		}
		if len(sheetAbout(out)) != 0 || removed {
			t.Errorf("%T while the ally is unseen sent %v", p, out)
		}
	}
	held := tb.v.shown.Actors["ally"]
	if len(held.GetAttributes()) != 0 || len(held.GetResources()) != 0 || held.GetModuleData() != nil ||
		len(tb.v.shown.Conditions["ally"]) != 0 {
		t.Errorf("the player holds the unseen ally as %v with %v, want no sheet and no condition",
			held, qacsConds(tb.v.shown, "ally"))
	}
	if held.GetName() != "Ally" || held.GetKind() != sheetParty || !sameStrings(held.GetControllerIds(), []string{"p-2"}) {
		t.Errorf("the player holds the unseen ally as %v, want it as it was introduced", held)
	}
}

// VTT-659 VTT-657
func TestACompanionsSheetArrivesWhenItsCharacterFirstSeesIt(t *testing.T) {
	stripped := []string{
		"*vttv1.Envelope_ActorRemoved ally",
		"*vttv1.Envelope_ActorAdded ally",
		"*vttv1.Envelope_ActorControlGranted ally",
		"*vttv1.Envelope_ActorControlGranted ally",
		"*vttv1.Envelope_ConditionApplied ally",
		"*vttv1.Envelope_ConditionApplied ally",
		"*vttv1.Envelope_TokenPlaced t-ally",
	}
	corrected := []string{
		"*vttv1.Envelope_ConditionApplied ally",
		"*vttv1.Envelope_ConditionApplied ally",
		"*vttv1.Envelope_ActorControlGranted ally",
		"*vttv1.Envelope_TokenPlaced t-ally",
	}
	// Keep the case with no resource: a resource set the fold lacks makes
	// correct re-introduce the ally, which hides a first sight left unmarked.
	for _, c := range []struct {
		name                  string
		attributes, resources bool
		want                  []string
	}{
		{"attributes and module data, no resource", true, false, stripped},
		{"resources", false, true, stripped},
		{"neither, so nothing to strip", false, false, corrected},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := newSheetTable(t, sheetAlly(t, c.attributes, c.resources))
			tb.step(&vttv1.NarrationAdded{Text: "look"})
			tb.step(&vttv1.ConditionApplied{ActorId: "ally", ConditionId: "dazed"})
			tb.step(&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-3", Kind: sheetParty})
			out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			about := sheetAbout(out)
			if got := shapes(about); !sameStrings(got, c.want) {
				t.Fatalf("at its first sight the ally was sent\n%v\nwant\n%v", got, c.want)
			}
			for _, e := range about {
				if e.GetSequence() != tb.seq || e.GetEventId() != "" {
					t.Errorf("a frame about the ally carries sequence %d and event id %q, want the door's %d and none: %v",
						e.GetSequence(), e.GetEventId(), tb.seq, e)
				}
			}
			if last := out[len(out)-1]; last.GetDoorOpened() == nil || last.GetEventId() == "" {
				t.Errorf("the frames do not end with the door's own event: %v", last)
			}
			if c.attributes || c.resources {
				bare := &vttv1.Envelope{Sequence: tb.seq,
					Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: "ally"}}}
				if !proto.Equal(about[0], bare) {
					t.Errorf("the withdrawal is not bare: %v", about[0])
				}
				if a := about[1].GetActorAdded().GetActor(); !proto.Equal(a, qacsWhole(tb.st.Actors["ally"])) {
					t.Errorf("at its first sight the ally was introduced as %v, want it whole", a)
				}
			}
			sheetHeldAsTheWorldHoldsIt(t, tb)

			tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
			later := []proto.Message{&vttv1.ConditionRemoved{ActorId: "ally", ConditionId: "marked"}}
			if c.resources {
				later = append(later, &vttv1.ResourceChanged{ActorId: "ally", Resource: "pool", Delta: -2, NewValue: 3})
			}
			for _, p := range later {
				tb.step(p)
			}
			out = tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			for _, e := range out {
				if e.GetActorRemoved() != nil || e.GetActorAdded() != nil {
					t.Errorf("the ally seen before was sent %v on its return, want a correction", e)
				}
			}
			if fix := correctionFrames(t, out, "ally"); len(fix) != len(later) {
				t.Errorf("the ally's return sent %v, want one correction per change", fix)
			}
			sheetHeldAsTheWorldHoldsIt(t, tb)
		})
	}
}

func TestAResumedLogHoldingAnUnseenCompanionInSightIsBroughtToItsSheet(t *testing.T) {
	for _, tc := range []struct {
		name      string
		resources bool
		before    proto.Message
		change    proto.Message
	}{
		{"resources", true, nil,
			&vttv1.ResourceChanged{ActorId: "ally", Resource: "pool", Delta: -2, NewValue: 3}},
		// Clear the ally's condition: Resume refuses a fold whose conditions differ,
		// which would hide a Resume that accepts the unseen form.
		{"attributes and module data, no resource", false,
			&vttv1.ConditionRemoved{ActorId: "ally", ConditionId: "marked"},
			&vttv1.ConditionApplied{ActorId: "ally", ConditionId: "dazed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := newSheetTable(t, sheetAlly(t, true, tc.resources))
			tb.step(&vttv1.NarrationAdded{Text: "look"})
			if tc.before != nil {
				tb.step(tc.before)
			}
			// Fold the unseen introduction by hand, as a log a dropped frame set left
			// holds it: the case must not rest on what the projection sent first.
			shown := tb.v.shown.Snapshot()
			for _, p := range []proto.Message{
				&vttv1.ActorRemoved{ActorId: "ally"},
				&vttv1.ActorAdded{Actor: qacsUnseen(tb.st.Actors["ally"])},
				&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2", Kind: sheetParty},
			} {
				mustApply(shown, tb.seq, p)
			}
			tb.seq++
			mustApply(tb.st, tb.seq, &vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			resumed := perceive.Resume(player(), shown, tb.st)

			tb.seq++
			change := envelope(tb.seq, tc.change)
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
			added := qacsFind(out, qacsAddedOf("ally"))
			if len(added) != 1 || !proto.Equal(out[added[0]].GetActorAdded().GetActor(), qacsWhole(tb.st.Actors["ally"])) {
				t.Errorf("the resumed log was not brought to the ally's sheet as it is now: %v", out)
			}
			for _, e := range out {
				if e.GetEventId() != "" {
					t.Errorf("the change was forwarded to a log that held the ally without its sheet: %v", e)
				}
			}
			if !tc.resources {
				return
			}
			if got := shown.Actors["ally"].GetResources()["pool"].GetCurrent(); got != 3 {
				t.Errorf("the resumed log holds the ally's pool at %d, want 3", got)
			}
		})
	}
}
