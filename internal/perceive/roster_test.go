package perceive_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt pins the shape of
// an introduction. Keep all three claims: no controller on the ActorAdded,
// one grant per controller in the actor's own order, and the batch folds.
// VTT-209
func TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt(t *testing.T) {
	st := twoRooms()
	// Keep the scout shared: one controller cannot show a grant per controller,
	// nor their order.
	mustApply(st, 8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	mustApply(st, 10, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-3",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	if ids := st.Actors["scout"].GetControllerIds(); len(ids) != 2 {
		t.Fatalf("fixture check: the scout must be shared, got %v", ids)
	}

	pr := newViewer(t, player())
	place := &vttv1.TokenPlaced{TokenId: "t-scout", SceneId: "s", ActorId: "scout",
		Position: &vttv1.GridPosition{X: 4, Y: 1}} // behind the shut door: unseen
	mustApply(st, 11, place)
	out := pr.event(envelope(11, place), st)

	var introIdx = -1
	var grants []*vttv1.ActorControlGranted
	for i, e := range out {
		if a := e.GetActorAdded(); a != nil && a.GetActor().GetActorId() == "scout" {
			introIdx = i
			if got := a.GetActor().GetControllerId(); got != "" {
				t.Errorf("the introduction carries controller_id %q — no ActorAdded may name a "+
					"controller, and both folds refuse one", got)
			}
			if got := a.GetActor().GetControllerIds(); len(got) != 0 {
				t.Errorf("the introduction carries controller_ids %v — no ActorAdded may name a "+
					"controller, and both folds refuse one", got)
			}
			if got := a.GetActor().GetKind(); got != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
				t.Errorf("the introduction lost the actor's kind: got %v", got)
			}
		}
		if g := e.GetActorControlGranted(); g != nil && g.GetActorId() == "scout" {
			if introIdx < 0 {
				t.Error("a grant reached this viewer BEFORE the actor it names: both folds " +
					"reject a grant over an unknown actor, so the order is not cosmetic")
			}
			grants = append(grants, g)
		}
	}
	if introIdx < 0 {
		t.Fatal("the scout is a party member and must be introduced at all")
	}
	if len(grants) != 2 {
		t.Fatalf("want one grant per controller (2), got %d", len(grants))
	}
	for i, want := range []string{"p-2", "p-3"} {
		if got := grants[i].GetParticipantId(); got != want {
			t.Errorf("grant %d is for %q, want %q — the set's order decides the mirror, so "+
				"reordering here hands controller_id to the wrong participant", i, got, want)
		}
		if got := grants[i].GetKind(); got != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
			t.Errorf("grant %d states kind %v, want the actor's own", i, got)
		}
	}

	viewer := engine.NewState()
	for i, e := range out {
		if err := engine.Apply(viewer, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	if ids := viewer.Actors["scout"].GetControllerIds(); len(ids) != 2 ||
		ids[0] != "p-2" || ids[1] != "p-3" {
		t.Errorf("the viewer's fold ends with controller_ids %v, want [p-2 p-3] — the "+
			"introduction plus its grants must reconstruct what the server holds", ids)
	}
}

// Keep the rogue another player's: this holds the party half of the roster
// rule, not the own-character half.
// VTT-208
func TestAPartyMemberStaysKnownEvenWhenOutOfSight(t *testing.T) {
	rogue := &vttv1.Actor{ActorId: "rogue", Name: "Rogue",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: rogue})
	mustApply(st, 9, &vttv1.TokenPlaced{TokenId: "t-rogue", SceneId: "s",
		ActorId: "rogue", Position: &vttv1.GridPosition{X: 5, Y: 1}}) // behind the closed door
	mustApply(st, 10, &vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})

	pr := newViewer(t, player())
	out := pr.event(envelope(8, &vttv1.ActorAdded{Actor: rogue}), st)

	var knowsRogue bool
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil && a.GetActor().GetActorId() == "rogue" {
			knowsRogue = true
		}
	}
	if !knowsRogue {
		t.Error("a player-controlled actor must reach every player's roster even out of sight")
	}

	out = pr.event(envelope(9, &vttv1.TokenPlaced{TokenId: "t-rogue",
		SceneId: "s", ActorId: "rogue",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)
	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-rogue" {
			t.Error("the rogue is behind a closed door — knowing they exist is not seeing them")
		}
	}
}

// actorIDsIn is the roster as it reaches the wire: every actor id out
// introduces. Read the frames, never the projection's internals.
func actorIDsIn(out []*vttv1.Envelope) map[string]bool {
	ids := map[string]bool{}
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil {
			ids[a.GetActor().GetActorId()] = true
		}
	}
	return ids
}

// Keep the goblin behind the shut door: then only the roster rule could
// introduce it, and a grant to the DM must not (SPEC-016).
// VTT-194 VTT-242
func TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster(t *testing.T) {
	st := twoRooms()
	grant := &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "dm-1"}
	mustApply(st, 8, grant)

	pr := newViewer(t, player())
	roster := actorIDsIn(pr.event(envelope(8, grant), st))

	if !roster["hero"] {
		t.Fatal("fixture check: the player's own character must be on this roster, " +
			"or the absence asserted below proves nothing")
	}
	if roster["goblin"] {
		t.Error("a monster the DM happens to hold is still a monster (spec §5.1): " +
			"it must not reach a player's roster")
	}
}

// TestAKindlessGrantConfersControlAndNothingElse builds by hand an event the
// command boundary refuses (validateGrantActorControl): a grant is the one
// event that confers control, so control alone must confer nothing here.
// VTT-242
func TestAKindlessGrantConfersControlAndNothingElse(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "cleric", Name: "Cleric"}})
	mustApply(st, 9, &vttv1.TokenPlaced{TokenId: "t-cleric", SceneId: "s",
		ActorId: "cleric", Position: &vttv1.GridPosition{X: 4, Y: 1}}) // behind the shut door
	silent := &vttv1.ActorControlGranted{ActorId: "cleric", ParticipantId: "p-2"}
	if k := silent.GetKind(); k != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatalf("fixture check: this grant must state NO kind, got %v", k)
	}
	mustApply(st, 10, silent)

	if ids := st.Actors["cleric"].GetControllerIds(); len(ids) != 1 || ids[0] != "p-2" {
		t.Fatalf("fixture check: the grant must actually have conferred control, got %v", ids)
	}
	pr := newViewer(t, player())
	roster := actorIDsIn(pr.event(envelope(10, silent), st))
	if !roster["hero"] {
		t.Fatal("fixture check: the player's own character must be on this roster, " +
			"or the absence asserted below proves nothing")
	}
	if roster["cleric"] {
		t.Error("a grant that said nothing about what it was granting made a party member: " +
			"control confers control, and nothing else")
	}
}

// Keep the kind on the actor, not on its holder's role: a party member the
// DM runs is still a party member.
// VTT-208
func TestAPartyMemberIsKnownEvenWhenHeldByTheDM(t *testing.T) {
	st := twoRooms()
	cleric := &vttv1.Actor{ActorId: "cleric", Name: "Cleric",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}
	mustApply(st, 8, &vttv1.ActorAdded{Actor: cleric})
	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "cleric", ParticipantId: "dm-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	place := &vttv1.TokenPlaced{TokenId: "t-cleric", SceneId: "s",
		ActorId: "cleric", Position: &vttv1.GridPosition{X: 5, Y: 1}} // behind the shut door
	mustApply(st, 10, place)

	pr := newViewer(t, player())
	roster := actorIDsIn(pr.event(envelope(10, place), st))

	if !roster["cleric"] {
		t.Error("a party member whose player is away is still a party member: " +
			"the DM holding their character must not erase them from the roster")
	}
}

// VTT-241
func TestAPlayerIsIntroducedToATokenlessActorItIsGranted(t *testing.T) {
	familiar := &vttv1.Actor{ActorId: "familiar", Name: "Familiar",
		Kind:      vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		Resources: map[string]*vttv1.Resource{"pool": {Current: 2, Max: 2}}}
	out := runLog(t, player(), []step{
		{1, &vttv1.SessionStarted{Name: "n"}},
		{2, &vttv1.ActorAdded{Actor: familiar}},
		{3, &vttv1.ActorControlGranted{ActorId: "familiar", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
	})
	viewer := engine.NewState()
	for i, e := range out {
		if err := engine.Apply(viewer, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	a, ok := viewer.Actors["familiar"]
	if !ok {
		t.Fatal("the player was not introduced to the token-less actor it was granted")
	}
	if ids := a.GetControllerIds(); len(ids) != 1 || ids[0] != "p-1" {
		t.Errorf("the player's fold holds the familiar with controllers %v, want [p-1]", ids)
	}
	if got := a.GetResources()["pool"].GetCurrent(); got != 2 {
		t.Errorf("the familiar arrived with pool %d, want 2", got)
	}
}

// TestAnActorWithNoDeclaredKindIsNotAPartyMemberWhoeverHoldsIt: an absent kind
// is no party member, always, and nothing reads controller_ids to decide.
// VTT-242
func TestAnActorWithNoDeclaredKindIsNotAPartyMemberWhoeverHoldsIt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     string
		holder string
	}{
		{"held by another player", "ghost", "p-2"},
		{"held by the DM", "wisp", "dm-1"},
		{"held by nobody", "rat", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := twoRooms()
			a := &vttv1.Actor{ActorId: tc.id, Name: tc.id}
			if k := a.GetKind(); k != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
				t.Fatalf("fixture check: this actor must declare NO kind, got %v", k)
			}
			mustApply(st, 8, &vttv1.ActorAdded{Actor: a})
			if tc.holder != "" {
				// A kindless grant: the command boundary refuses it, the fold
				// accepts it.
				mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: tc.id, ParticipantId: tc.holder})
			}
			place := &vttv1.TokenPlaced{TokenId: "t-" + tc.id, SceneId: "s",
				ActorId: tc.id, Position: &vttv1.GridPosition{X: 5, Y: 1}} // behind the shut door
			mustApply(st, 10, place)

			pr := newViewer(t, player())
			roster := actorIDsIn(pr.event(envelope(10, place), st))
			if !roster["hero"] {
				t.Fatal("fixture check: the player's own character must be on this roster, " +
					"or the absence asserted below proves nothing")
			}
			if roster[tc.id] {
				t.Errorf("%q declares no kind, so it is not a party member and must not reach "+
					"a player's roster — control confers nothing", tc.id)
			}
		})
	}
}

// Keep the hero check: it is what stops the loop passing on an empty batch.
// VTT-208
func TestASpectatorGetsNoSightFromAnNPCTheDMControls(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "dm-1"})

	pr := newViewer(t, perceive.Eyes{Shoulder: "goblin"})

	out := firstPlace(pr, st)
	if !actorIDsIn(out)["hero"] {
		t.Fatal("fixture check: a spectator must still be told of the party, " +
			"or the absence asserted below proves nothing")
	}
	for _, e := range out {
		switch {
		case e.GetSceneCreated() != nil, e.GetSceneSeen() != nil, e.GetTokenPlaced() != nil:
			t.Fatalf("a perch on a monster the DM holds must yield no sight at all, got %v", e)
		}
	}
}

// VTT-220
func TestAnUnrecognisedPayloadIsWithheldFromAPlayer(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	out := pr.event(&vttv1.Envelope{Sequence: 99}, st) // no payload at all
	if len(out) != 0 {
		t.Fatalf("an unknown payload must be withheld from a player, got %d", len(out))
	}
}

// TestAnUnrecognisedPayloadIsWithheldFromASpectator is the spectator's half of
// TestAnUnrecognisedPayloadIsWithheldFromAPlayer. Keep the control: a fixture
// with nothing to send passes the first check whatever the projection does.
// VTT-227
func TestAnUnrecognisedPayloadIsWithheldFromASpectator(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, perceive.Eyes{Shoulder: "hero"})
	if out := pr.event(&vttv1.Envelope{Sequence: 99}, st); len(out) != 0 {
		t.Fatalf("an unknown payload must be withheld from a spectator, got %d frames", len(out))
	}
	if sceneSeenIn(firstPlace(pr, st), "s") == nil {
		t.Fatal("the control fails: the next recognised event must bring the shoulder's board")
	}
}

// Keep the intro batch in the fold below: a later batch folded alone fails on
// the test's account, not the projection's.
// VTT-210 VTT-249
func TestAConditionAppliedOutOfSightArrivesWithTheActor(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.ConditionApplied{
		ActorId: "goblin", ConditionId: "marked", Source: "the DM"})

	pr := newViewer(t, player())

	intro := pr.event(envelope(8, &vttv1.ConditionApplied{
		ActorId: "goblin", ConditionId: "marked", Source: "the DM"}), st)
	for _, e := range intro {
		if e.GetConditionApplied() != nil {
			t.Fatal("a condition on an unseen actor names it, and must be withheld")
		}
	}

	mustApply(st, 9, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.event(envelope(9, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	var got *vttv1.ConditionApplied
	for _, e := range out {
		if ca := e.GetConditionApplied(); ca != nil && ca.GetActorId() == "goblin" {
			got = ca
		}
	}
	if got == nil {
		t.Fatal("an introduced actor must arrive with the conditions already on it")
	}
	if got.GetConditionId() != "marked" || got.GetSource() != "" {
		t.Errorf("the condition must arrive by its id and without what applied it, got id %q source %q",
			got.GetConditionId(), got.GetSource())
	}

	viewerState := engine.NewState()
	stream := append(append([]*vttv1.Envelope{}, intro...), out...)
	for i, e := range stream {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	if n := len(viewerState.Conditions["goblin"]); n != 1 {
		t.Fatalf("the introduction must leave the condition ON the folded actor, got %d", n)
	}
	mustApply(st, 10, &vttv1.ConditionRemoved{ActorId: "goblin", ConditionId: "marked"})
	for _, e := range pr.event(envelope(10, &vttv1.ConditionRemoved{
		ActorId: "goblin", ConditionId: "marked"}), st) {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("the removal does not fold, which is a permanent client freeze: %v", err)
		}
	}
	if n := len(viewerState.Conditions["goblin"]); n != 0 {
		t.Errorf("the condition was removed and must be gone, got %d", n)
	}
}

// removeActorBatch projects the batch remove_actor emits, in its handler's
// order: one TokenRemoved per token in token-id order, then the ActorRemoved.
func removeActorBatch(pr *testViewer, st *engine.State, seq int64,
	actorID string, tokenIDs []string) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, id := range tokenIDs {
		mustApply(st, seq, &vttv1.TokenRemoved{TokenId: id})
		out = append(out, pr.event(envelope(seq, &vttv1.TokenRemoved{TokenId: id}), st)...)
		seq++
	}
	mustApply(st, seq, &vttv1.ActorRemoved{ActorId: actorID})
	return append(out, pr.event(envelope(seq, &vttv1.ActorRemoved{ActorId: actorID}), st)...)
}

// TestARemovedActorReachesOnlyTheSeatThatHeldIt holds both directions of the
// removal ruling: an actor the seat never held is not announced, since its
// fold would refuse the removal, and the seat's own character, which it sees,
// is (SPEC-016).
// VTT-215
func TestARemovedActorReachesOnlyTheSeatThatHeldIt(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	intro := firstPlace(pr, st)

	unseen := removeActorBatch(pr, st, 8, "goblin", []string{"t-gob"})
	for _, e := range unseen {
		if e.GetActorRemoved() != nil {
			t.Fatal("an actor this seat never held must not be announced as removed — it would name a creature they never saw")
		}
	}

	held := removeActorBatch(pr, st, 10, "hero", []string{"t-hero"})
	var told bool
	for _, e := range held {
		if ar := e.GetActorRemoved(); ar != nil && ar.GetActorId() == "hero" {
			told = true
		}
	}
	if !told {
		t.Fatal("want the raw ActorRemoved forwarded to a seat that sees the actor")
	}

	viewerState := engine.NewState()
	var stream []*vttv1.Envelope
	stream = append(stream, intro...)
	stream = append(stream, unseen...)
	stream = append(stream, held...)
	for i, e := range stream {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
}

// TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh holds the case where
// the seat saw the removal: the projector forgets the actor with its fold, so
// the id used again is introduced in full (SPEC-016).
// VTT-313
func TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	stream := firstPlace(pr, st)

	stream = append(stream, removeActorBatch(pr, st, 8, "hero", []string{"t-hero"})...)

	steps := []struct {
		seq     int64
		payload proto.Message
	}{
		{10, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "Hero II",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}},
		{11, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		{12, &vttv1.TokenPlaced{TokenId: "t-hero-2", SceneId: "s", ActorId: "hero",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
	}
	mark := len(stream)
	for _, step := range steps {
		mustApply(st, step.seq, step.payload)
		stream = append(stream, pr.event(envelope(step.seq, step.payload), st)...)
	}

	var reintroduced bool
	for _, e := range stream[mark:] {
		if aa := e.GetActorAdded(); aa != nil && aa.GetActor().GetActorId() == "hero" {
			reintroduced = true
		}
	}
	if !reintroduced {
		t.Fatal("want the reused actor id introduced again — the seat's own fold no longer has it")
	}

	viewerState := engine.NewState()
	for i, e := range stream {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
}

// TestAnEventNamingAnUnknownActorIsWithheld: what leaks here is existence.
// Keep the control: withholding everything satisfies the first loop.
func TestAnEventNamingAnUnknownActorIsWithheld(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st) // the goblin is behind a closed door and unknown

	hidden := []struct {
		name    string
		payload proto.Message
	}{
		{"attack", &vttv1.AttackRolled{AttackerId: "goblin", TargetId: "hero", Total: 17}},
		{"condition", &vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "hidden"}},
		{"resource", &vttv1.ResourceChanged{ActorId: "goblin", Resource: "pool-a", Delta: -3}},
		{"ability", &vttv1.AbilityUsed{ActorId: "goblin", AbilityId: "ambush"}},
	}
	for _, h := range hidden {
		in := &vttv1.Envelope{Sequence: 7}
		switch p := h.payload.(type) {
		case *vttv1.AttackRolled:
			in.Payload = &vttv1.Envelope_AttackRolled{AttackRolled: p}
		case *vttv1.ConditionApplied:
			in.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: p}
		case *vttv1.ResourceChanged:
			in.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: p}
		case *vttv1.AbilityUsed:
			in.Payload = &vttv1.Envelope_AbilityUsed{AbilityUsed: p}
		}
		for _, e := range pr.event(in, st) {
			if e == in {
				t.Errorf("%s: an event naming an actor this player has never seen must be withheld", h.name)
			}
		}
	}

	known := &vttv1.Envelope{Sequence: 8, EventId: "evt-8", Payload: &vttv1.Envelope_ConditionApplied{
		ConditionApplied: &vttv1.ConditionApplied{ActorId: "hero", ConditionId: "hidden"}}}
	var forwarded bool
	for _, e := range pr.event(known, st) {
		if e.GetEventId() != "" && proto.Equal(e, forwardedOf(known)) {
			forwarded = true
		}
	}
	if !forwarded {
		t.Error("a condition on the player's own character must reach them")
	}
}

// VTT-216
func TestNarrationReachesAPlayer(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st)

	narration := envelope(8, &vttv1.NarrationAdded{Text: "The door creaks.", As: "DM"})
	var toldStory bool
	for _, e := range pr.event(narration, st) {
		if e.GetNarrationAdded() != nil {
			toldStory = true
		}
	}
	if !toldStory {
		t.Error("withholding narration from players silences the table's story channel")
	}
}

// noteSeat is one projected seat whose client folds every frame it is sent.
type noteSeat struct {
	name   string
	pr     *testViewer
	folded *engine.State
}

func noteSeats(tb testing.TB) []*noteSeat {
	tb.Helper()
	return []*noteSeat{
		{name: "player", pr: newViewer(tb, player()), folded: engine.NewState()},
		{name: "spectator", pr: newViewer(tb, perceive.Eyes{}), folded: engine.NewState()},
	}
}

// feedNote applies one event to the world and projects it to every seat,
// failing the test if a seat's fold refuses a frame it was sent.
func feedNote(t *testing.T, st *engine.State, seats []*noteSeat, seq int64, payload proto.Message) map[string][]*vttv1.Envelope {
	t.Helper()
	env := envelope(seq, payload)
	if err := engine.Apply(st, env); err != nil {
		t.Fatalf("the world refused seq %d: %v", seq, err)
	}
	sent := map[string][]*vttv1.Envelope{}
	for _, s := range seats {
		out := s.pr.event(env, st)
		for _, e := range out {
			if err := engine.Apply(s.folded, e); err != nil {
				t.Fatalf("%s's fold refused a frame of seq %d: %v", s.name, seq, err)
			}
		}
		sent[s.name] = out
	}
	return sent
}

func namesNote(out []*vttv1.Envelope, key string) bool {
	for _, e := range out {
		if e.GetNoteUpserted().GetKey() == key || e.GetNoteDeleted().GetKey() == key {
			return true
		}
	}
	return false
}

// VTT-233
func TestANoteRecordedWithoutAVisibilityReachesNoPlayer(t *testing.T) {
	st, seats := engine.NewState(), noteSeats(t)
	sent := feedNote(t, st, seats, 1, &vttv1.NoteUpserted{
		Key: "ambush", Title: "Ambush", Text: "Archer waits at 19,8"})
	for _, s := range seats {
		if namesNote(sent[s.name], "ambush") {
			t.Errorf("a note recorded without a visibility reached the %s", s.name)
		}
		if len(s.folded.Notes) != 0 {
			t.Errorf("the %s's fold holds %v", s.name, s.folded.Notes)
		}
	}
}

// VTT-234
func TestAPublicNoteReachesEveryPlayerAndSpectator(t *testing.T) {
	st := twoRooms()
	seats := append(noteSeats(t),
		&noteSeat{name: "player with no actor", pr: newViewer(t,
			perceive.Eyes{Controller: "p-9"}), folded: engine.NewState()},
		&noteSeat{name: "spectator on the hero", pr: newViewer(t,
			perceive.Eyes{Shoulder: "hero"}), folded: engine.NewState()},
	)
	sent := feedNote(t, st, seats, 8, &vttv1.NoteUpserted{Key: "tavern", Title: "Tavern",
		Text: "The Rusty Flagon.", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC})
	for _, s := range seats {
		if !namesNote(sent[s.name], "tavern") {
			t.Errorf("a public note did not reach the %s", s.name)
		}
		if got := s.folded.Notes["tavern"]; got.Text != "The Rusty Flagon." {
			t.Errorf("the %s's fold holds %+v for the public note", s.name, got)
		}
	}
}

// VTT-235
func TestANoteNeverPublicIsNeverNamedToAPlayer(t *testing.T) {
	st, seats := engine.NewState(), noteSeats(t)
	secret := vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET
	for seq, payload := range []proto.Message{
		&vttv1.NoteUpserted{Key: "ambush", Text: "Archer waits at 19,8", Visibility: secret},
		&vttv1.NoteUpserted{Key: "ambush", Text: "Two archers now", Visibility: secret},
		&vttv1.NoteDeleted{Key: "ambush"},
	} {
		sent := feedNote(t, st, seats, int64(seq+1), payload)
		for _, s := range seats {
			if namesNote(sent[s.name], "ambush") {
				t.Errorf("seq %d named a never-public note to the %s", seq+1, s.name)
			}
		}
	}
}

// VTT-236 VTT-221
func TestANoteMadeSecretLeavesEveryFoldThatHeldIt(t *testing.T) {
	for _, flip := range []vttv1.NoteVisibility{
		vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
		vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED,
	} {
		t.Run(flip.String(), func(t *testing.T) {
			st, seats := engine.NewState(), noteSeats(t)
			feedNote(t, st, seats, 1, &vttv1.NoteUpserted{Key: "rumor", Text: "Gold under the cellar.",
				Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC})
			for _, s := range seats {
				if _, ok := s.folded.Notes["rumor"]; !ok {
					t.Fatalf("fixture check: the %s must hold the public note before it is made secret", s.name)
				}
			}
			sent := feedNote(t, st, seats, 2, &vttv1.NoteUpserted{Key: "rumor", Text: "Gold under the cellar.",
				Visibility: flip})
			for _, s := range seats {
				if _, ok := s.folded.Notes["rumor"]; ok {
					t.Errorf("the %s still holds a note that stopped being public", s.name)
				}
				for _, e := range sent[s.name] {
					if e.GetNoteDeleted() != nil && e.GetSequence() != 2 {
						t.Errorf("the %s's NoteDeleted carries seq %d, want the causing 2", s.name, e.GetSequence())
					}
					if e.GetNoteUpserted() != nil {
						t.Errorf("the %s was sent the secret upsert itself", s.name)
					}
				}
			}
		})
	}
}

// VTT-237
func TestADeletedPublicNoteLeavesEveryFoldThatHeldIt(t *testing.T) {
	st, seats := engine.NewState(), noteSeats(t)
	feedNote(t, st, seats, 1, &vttv1.NoteUpserted{Key: "tavern", Text: "The Rusty Flagon.",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC})
	for _, s := range seats {
		if _, ok := s.folded.Notes["tavern"]; !ok {
			t.Fatalf("fixture check: the %s must hold the public note before it is deleted", s.name)
		}
	}
	sent := feedNote(t, st, seats, 2, &vttv1.NoteDeleted{Key: "tavern"})
	for _, s := range seats {
		if _, ok := s.folded.Notes["tavern"]; ok {
			t.Errorf("the %s still holds a deleted note", s.name)
		}
		if n := len(sent[s.name]); n != 1 {
			t.Errorf("the %s was sent %d frames for one deletion, want 1", s.name, n)
		}
	}
}

// VTT-238
func TestADeletionAfterANoteWasMadeSecretReachesNoPlayer(t *testing.T) {
	st, seats := engine.NewState(), noteSeats(t)
	feedNote(t, st, seats, 1, &vttv1.NoteUpserted{Key: "rumor", Text: "Gold under the cellar.",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC})
	for _, s := range seats {
		if _, ok := s.folded.Notes["rumor"]; !ok {
			t.Fatalf("fixture check: the %s must hold the public note before it is made secret", s.name)
		}
	}
	feedNote(t, st, seats, 2, &vttv1.NoteUpserted{Key: "rumor", Text: "Gold under the cellar.",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET})
	for _, s := range seats {
		if _, ok := s.folded.Notes["rumor"]; ok {
			t.Fatalf("fixture check: the %s must have lost the note when it was made secret", s.name)
		}
	}
	sent := feedNote(t, st, seats, 3, &vttv1.NoteDeleted{Key: "rumor"})
	for _, s := range seats {
		if namesNote(sent[s.name], "rumor") {
			t.Errorf("the %s was sent the deletion of a note it no longer holds", s.name)
		}
	}
}

// removalBatchFixture is the seat's whole stream for the mixed-visibility
// removal batch. Keep it in contract/testdata: the TypeScript fold reads the
// same file (client/test/removal-batch-parity.test.ts).
const removalBatchFixture = "../../contract/testdata/removal_batch_projected_stream.json"

// removalBatchWorld is twoRooms with the goblin holding two tokens either side
// of the shut door. Keep the far one unseen: the mixed case is the point.
func removalBatchWorld() *engine.State {
	st := engine.NewState()
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "goblin", Name: "Goblin", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 2, Y: 1}})
	mustApply(st, 7, &vttv1.TokenPlaced{TokenId: "t-gob-far", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	mustApply(st, 8, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

// projectRemovalBatchSeat is everything the hero's player receives of this
// world: its introductions, the removal batch and the id used again.
func projectRemovalBatchSeat(tb testing.TB, st *engine.State) []*vttv1.Envelope {
	tb.Helper()
	pr := newViewer(tb, player())
	stream := pr.event(envelope(7, &vttv1.TokenPlaced{TokenId: "t-gob-far",
		SceneId: "s", ActorId: "goblin",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)
	stream = append(stream, removeActorBatch(pr, st, 9, "goblin",
		[]string{"t-gob", "t-gob-far"})...)
	for _, s := range []step{
		{12, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Second Goblin",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}},
		{13, &vttv1.TokenPlaced{TokenId: "t-gob-2", SceneId: "s", ActorId: "goblin",
			Position: &vttv1.GridPosition{X: 2, Y: 1}}},
	} {
		mustApply(st, s.seq, s.payload)
		stream = append(stream, pr.event(envelope(s.seq, s.payload), st)...)
	}
	return stream
}

// TestARemovalBatchProjectsToTheBytesBothFoldsRead holds the stream both folds
// read (client/test/removal-batch-parity.test.ts): a token seen leaves as a
// TokenHidden, one never shown is named nowhere, the actor's removal, unseen
// once its token is gone, is not reported, and its id used again arrives behind
// a bare ActorRemoved (SPEC-016).
// VTT-200 VTT-221 VTT-251 VTT-252
func TestARemovalBatchProjectsToTheBytesBothFoldsRead(t *testing.T) {
	stream := projectRemovalBatchSeat(t, removalBatchWorld())

	raw, err := os.ReadFile(removalBatchFixture)
	if err != nil {
		t.Fatal(err)
	}
	var rawEnvelopes []json.RawMessage
	if err := json.Unmarshal(raw, &rawEnvelopes); err != nil {
		t.Fatal(err)
	}
	want := make([]*vttv1.Envelope, 0, len(rawEnvelopes))
	for i, one := range rawEnvelopes {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(one, env); err != nil {
			t.Fatalf("fixture envelope %d: %v", i, err)
		}
		want = append(want, env)
	}

	// Compare with proto.Equal, never bytes: protojson's spacing changes from
	// one build to the next.
	if len(stream) != len(want) {
		t.Fatalf("the seat received %d envelopes, the fixture holds %d", len(stream), len(want))
	}
	for i := range want {
		if !proto.Equal(stream[i], want[i]) {
			t.Errorf("envelope %d differs\n got: %v\nwant: %v", i, stream[i], want[i])
		}
	}

	// Keep this by name: a fixture regenerated with both tokens in sight would
	// pass the comparison above.
	for i, e := range stream {
		if b, err := protojson.Marshal(e); err == nil && strings.Contains(string(b), "t-gob-far") {
			t.Errorf("envelope %d names t-gob-far, a token this seat was never shown: %s", i, b)
		}
	}

	viewerState := engine.NewState()
	for i, e := range stream {
		if e.GetActorRemoved() != nil {
			if g := viewerState.Actors["goblin"]; g.GetName() != "Goblin" || len(viewerState.Tokens) != 1 {
				t.Errorf("before the reuse the seat must hold the first goblin and no token of it, got %v and %d tokens",
					g, len(viewerState.Tokens))
			}
		}
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	if g := viewerState.Actors["goblin"]; g.GetName() != "Second Goblin" {
		t.Errorf("after the reuse the seat must hold the second goblin, got %v", g)
	}
	if _, ok := viewerState.Actors["hero"]; !ok {
		t.Error("the seat's own character must survive another actor's removal")
	}
	for _, id := range []string{"t-gob", "t-gob-far"} {
		if _, ok := viewerState.Tokens[id]; ok {
			t.Errorf("%s must not be on the seat's board after the batch", id)
		}
	}
	for _, id := range []string{"t-hero", "t-gob-2"} {
		if _, ok := viewerState.Tokens[id]; !ok {
			t.Errorf("%s must be on the seat's board at the end", id)
		}
	}
}
