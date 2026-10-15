package gateway_test

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// key must format a square as the wire does, column then row, as engine's
// gridKey and sight do.
func key(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

// envelope wraps payload at seq. Keep the panic on an unknown payload: an empty
// Envelope would pass the projection's unrecognised arm for the wrong reason.
func envelope(seq int64, payload proto.Message) *vttv1.Envelope {
	env := &vttv1.Envelope{Sequence: seq}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		env.SessionId = "sess-1"
		env.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SessionEnded:
		env.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: p}
	case *vttv1.SceneCreated:
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.TokenPlaced:
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		env.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.TokenRemoved:
		env.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: p}
	case *vttv1.ActorRemoved:
		env.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: p}
	case *vttv1.DoorOpened:
		env.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: p}
	case *vttv1.DoorClosed:
		env.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: p}
	case *vttv1.NarrationAdded:
		env.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: p}
	case *vttv1.NoteUpserted:
		env.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: p}
	case *vttv1.NoteDeleted:
		env.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: p}
	case *vttv1.ActorControlGranted:
		env.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: p}
	case *vttv1.ActorControlRevoked:
		env.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: p}
	case *vttv1.SceneSeen:
		env.Payload = &vttv1.Envelope_SceneSeen{SceneSeen: p}
	case *vttv1.ConditionApplied:
		env.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: p}
	case *vttv1.ConditionRemoved:
		env.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: p}
	case *vttv1.ResourceChanged:
		env.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: p}
	case *vttv1.AttackRolled:
		env.Payload = &vttv1.Envelope_AttackRolled{AttackRolled: p}
	case *vttv1.AbilityUsed:
		env.Payload = &vttv1.Envelope_AbilityUsed{AbilityUsed: p}
	default:
		panic(fmt.Sprintf("envelope: no oneof arm wired for %T", payload))
	}
	return env
}

// mustApply folds one event into st, panicking on rejection. Panicking rather
// than taking a *testing.T is forced by the fixtures below, which build a
// state outside any test's scope — and it is the right failure anyway: a
// fixture the fold refuses is a broken fixture, not a failing assertion.
func mustApply(st *engine.State, seq int64, payload proto.Message) {
	if err := engine.Apply(st, envelope(seq, payload)); err != nil {
		panic(fmt.Sprintf("mustApply seq %d: %v", seq, err))
	}
}

// twoRooms: 7x3, a dividing wall at x=3 with a CLOSED door at 3,1.
//
//	 x: 0    1    2    3     4    5    6
//	y=0 wall wall wall wall  wall wall wall
//	y=1 wall flr  flr  door  flr  flr  wall
//	y=2 wall wall wall wall  wall wall wall
func twoRoomsTiles() map[string]*vttv1.TileRef {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		tiles[key(x, 0)] = &vttv1.TileRef{Kind: "wall"}
		tiles[key(x, 2)] = &vttv1.TileRef{Kind: "wall"}
	}
	tiles[key(0, 1)] = &vttv1.TileRef{Kind: "wall"}
	tiles[key(6, 1)] = &vttv1.TileRef{Kind: "wall"}
	for _, x := range []int32{1, 2, 4, 5} {
		tiles[key(x, 1)] = &vttv1.TileRef{Kind: "floor"}
	}
	tiles[key(3, 1)] = &vttv1.TileRef{Kind: "door"}
	return tiles
}

func twoRooms() *engine.State {
	st := engine.NewState()
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()})
	// Keep every actor's kind declared and its control a grant at the end: an
	// absent kind is no party member, and only a grant confers control (SPEC-013).
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "goblin", Name: "Goblin",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}) // never granted: an NPC
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	mustApply(st, 7, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

func player() gateway.Viewer {
	return gateway.Viewer{ParticipantID: "p-1", Role: identity.RolePlayer}
}

func TestTheDMReceivesEverythingUnchanged(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{ParticipantID: "dm", Role: identity.RoleDM})

	in := envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	out := pr.Project(in, st)

	if len(out) != 1 {
		t.Fatalf("the DM's projection must be the identity function: one envelope, got %d", len(out))
	}
	// BY POINTER, and the two failures are split so the message names which
	// one happened: a clone would satisfy proto.Equal while still failing exit
	// criterion 8's "byte-for-byte unchanged", and it would put a per-seat copy
	// on the path that today does no work at all.
	if out[0] != in {
		t.Fatal("the DM must receive the SAME envelope, not an equal copy of it")
	}
}

func TestTheAgentSeatReceivesEverythingUnchangedToo(t *testing.T) {
	// Exit criterion 8 names the agent seat explicitly, alongside the DM. It
	// shares a case clause with RoleDM today, so this is cheap — but "shares a
	// clause today" is precisely the kind of fact a later edit changes without
	// noticing, and the agent is the seat an LLM sits in: a projected agent
	// would be an LLM quietly reasoning about a board it has been given a
	// redacted view of, which is worse than a wrong pixel.
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{ParticipantID: "agent-1", Role: identity.RoleAgent})

	in := envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	out := pr.Project(in, st)

	if len(out) != 1 {
		t.Fatalf("the agent's projection must be the identity function: one envelope, got %d", len(out))
	}
	if out[0] != in {
		t.Fatal("the agent must receive the SAME envelope, not an equal copy of it")
	}
}

// actorIDsIn is the roster a batch introduces. Read the envelopes, never the
// projection's internals: the rule is about what a player receives.
func actorIDsIn(out []*vttv1.Envelope) map[string]bool {
	ids := map[string]bool{}
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil {
			ids[a.GetActor().GetActorId()] = true
		}
	}
	return ids
}

// shippedActor returns one actor of the committed goblin-ambush adventure, as
// load_adventure logs it. Keep it read from disk: only shipped content shows the
// kind its author wrote.
func shippedActor(t *testing.T, actorID string) *vttv1.Actor {
	t.Helper()
	envs, _, err := adventure.Compile(loadGoblinAmbush(t, loadDnd45eMinimal(t)), engine.NewState())
	if err != nil {
		t.Fatalf("adventure.Compile(goblin-ambush): %v", err)
	}
	for _, env := range envs {
		if a := env.GetActorAdded().GetActor(); a.GetActorId() == actorID {
			return a
		}
	}
	t.Fatalf("goblin-ambush ships no actor %q", actorID)
	return nil
}

// VTT-242
func TestGrantingAnAgentTheShippedGoblinArcherDoesNotPublishItToThePlayers(t *testing.T) {
	// Keep this on shipped content: it walks the whole path the leak took, from a
	// file on disk to a player's roster. The archer's own non_party kind holds this
	// too, so this test does not guard the grant's kind alone.
	archer := shippedActor(t, "act-archer")
	if k := archer.GetKind(); k != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Fatalf("fixture check: the SHIPPED archer must declare itself non-party — it is the "+
			"creature this whole arc is named after — and it declares %v", k)
	}

	// twoRooms' geometry, because sight is what makes the roster exception
	// observable: the archer stands behind a SHUT door, so nothing this
	// player can see introduces it.
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: archer})
	mustApply(st, 9, &vttv1.TokenPlaced{TokenId: "t-archer", SceneId: "s",
		ActorId: "act-archer", Position: &vttv1.GridPosition{X: 4, Y: 1}})
	take := &vttv1.ActorControlGranted{ActorId: "act-archer", ParticipantId: "agent-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}
	mustApply(st, 10, take)

	pr := gateway.NewProjector(player())
	roster := actorIDsIn(pr.Project(envelope(10, take), st))

	if !roster["hero"] {
		t.Fatal("fixture check: the player's own character must be on this roster, " +
			"or the absence asserted below proves nothing")
	}
	if roster["act-archer"] {
		t.Error("an agent taking the Goblin Archer to run it said what it was taking " +
			"(spec §5.1): the archer is still a monster and must not reach a player's roster")
	}
}

// VTT-208
func TestTheSameShippedArcherAssignedToAPlayerIsAPartyMember(t *testing.T) {
	// Keep the archer, not the fighter: the archer is born non-party, so only this
	// grant's kind can put it on the roster.
	archer := shippedActor(t, "act-archer")
	if k := archer.GetKind(); k != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Fatalf("fixture check: the shipped archer must be born non-party, or the grant "+
			"below is not what puts it on the roster; it declares %v", k)
	}

	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: archer})
	mustApply(st, 9, &vttv1.TokenPlaced{TokenId: "t-archer", SceneId: "s",
		ActorId: "act-archer", Position: &vttv1.GridPosition{X: 4, Y: 1}}) // behind the shut door
	assign := &vttv1.ActorControlGranted{ActorId: "act-archer", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}
	mustApply(st, 10, assign)

	pr := gateway.NewProjector(player())
	if roster := actorIDsIn(pr.Project(envelope(10, assign), st)); !roster["act-archer"] {
		t.Error("a creature the DM handed to a player AS THEIRS is a party member from that " +
			"moment: you know your party exists even when they are two rooms away (spec §5), " +
			"and the grant is what said so")
	}
}

// A pregenerated character is a party member before anybody is assigned it: grant
// nothing here, since the party seeing the available sheets is not a leak.
// VTT-208
func TestTheShippedHumanFighterIsAPartyMemberBeforeAnybodyIsAssignedToIt(t *testing.T) {
	fighter := shippedActor(t, "act-fighter")
	if k := fighter.GetKind(); k != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Fatalf("fixture check: goblin-ambush's Human Fighter is a player character and its "+
			"file must say so; it declares %v", k)
	}
	if len(fighter.GetControllerIds()) != 0 || fighter.GetControllerId() != "" {
		t.Fatal("fixture check: an adventure actor is compiled with NO controller " +
			"(adventure/compile.go's buildActor), or this pins the granted case instead")
	}

	// Behind the shut door, so nothing this player can SEE introduces it: the
	// roster exception is the only thing that can put it on the list.
	st := twoRooms()
	added := &vttv1.ActorAdded{Actor: fighter}
	mustApply(st, 8, added)
	mustApply(st, 9, &vttv1.TokenPlaced{TokenId: "t-fighter", SceneId: "s",
		ActorId: "act-fighter", Position: &vttv1.GridPosition{X: 4, Y: 1}})

	pr := gateway.NewProjector(player())
	if roster := actorIDsIn(pr.Project(envelope(8, added), st)); !roster["act-fighter"] {
		t.Error("a pregenerated character the adventure declares a party member must reach " +
			"the party's roster with no grant at all: nobody inferred it, the author said it")
	}
}

// Keep this over the wire and on shipped content: add_actor must refuse a
// controller, and the shipped archer's stated kind is what keeps the refusal
// asserted the controller one (engine.Apply's ActorAdded arm refuses it too).
func TestTheShippedGoblinArcherCannotBeGivenAControllerAtCreation(t *testing.T) {
	archer := shippedActor(t, "act-archer")
	if k := archer.GetKind(); k != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Fatalf("fixture check: the SHIPPED archer must declare itself non-party, or the "+
			"refusal below could be about its kind rather than its controller; it declares %v", k)
	}

	f := newGWFixture(t)
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)

	seeded := proto.Clone(archer).(*vttv1.Actor)
	seeded.ControllerId = f.playerID
	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "seed-archer",
		Command:   &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: seeded}},
	})
	res := readResult(t, dm)
	if res.GetOk() {
		t.Fatal("add_actor accepted a controller: the archer was created already holding " +
			"one, with no kind stated — the shape that was a party member under the " +
			"migration rule, and that no log may contain at all now")
	}
	if !strings.Contains(res.GetError(), "grant_actor_control") {
		t.Errorf("refusal %q never names the command that DOES confer control — the "+
			"caller who tried the one step has no way to find the two", res.GetError())
	}
	if _, exists := f.campaign.State().Actors["act-archer"]; exists {
		t.Error("the refusal must happen BEFORE anything is written: a refused command " +
			"that still created the actor is worse than one that created it with a controller")
	}
}

// The control for the test above: the two steps that replace the refused one.
func TestCreatingAnActorAndThenGrantingItIsTheTwoStepThatReplacesTheOne(t *testing.T) {
	f := newGWFixture(t)
	dm := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, dm)
	expectPresenceSnapshot(t, dm)

	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "make-bandit",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-bandit", Name: "Bandit",
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}},
	})
	if res := readResult(t, dm); !res.GetOk() {
		t.Fatalf("add_actor STAYS — it is the runtime path for a creature nobody wrote in "+
			"advance — and it was refused: %q", res.GetError())
	}

	sendCommand(t, dm, &vttv1.ClientCommand{
		RequestId: "hand-bandit-over",
		Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: "act-bandit", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
	})
	if res := readResult(t, dm); !res.GetOk() {
		t.Fatalf("the second step of the two-step was refused: %q", res.GetError())
	}

	actor, ok := f.campaign.State().Actors["act-bandit"]
	if !ok {
		t.Fatal("act-bandit is not in state")
	}
	if got := actor.GetControllerIds(); len(got) != 1 || got[0] != f.playerID {
		t.Errorf("controller_ids = %v, want exactly [%q] — the grant is what confers control now",
			got, f.playerID)
	}
	// A player holding a monster still holds a monster: control describes who is
	// driving (SPEC-013).
	if got := actor.GetKind(); got != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Errorf("kind = %v, want ACTOR_KIND_NON_PARTY — the grant declares it and nothing else does", got)
	}
}

// firstPlace runs the projector over the goblin's placement, which is the
// cheapest way to bring a fresh projector up to "this player has been shown
// their scene, their party and what they can see". Returns the envelopes so a
// caller can assert on the introduction batch itself.
func firstPlace(pr *gateway.Projector, st *engine.State) []*vttv1.Envelope {
	return pr.Project(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob",
		SceneId: "s", ActorId: "goblin",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)
}

func TestAPlayerCannotBorrowAnNpcsEyesByPerching(t *testing.T) {
	// Keep a player seeing through its own actors whatever Viewpoint it sets: the
	// client sets that field, and honouring it reopens session zero (SPEC-015).
	st := twoRooms()
	perched := gateway.NewProjector(gateway.Viewer{
		ParticipantID: "p-1", Role: identity.RolePlayer, Viewpoint: "goblin"})
	for _, e := range firstPlace(perched, st) {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			t.Fatal("a player's viewpoint must not buy them an NPC's eyes")
		}
		if ss := e.GetSceneSeen(); ss != nil {
			if _, ok := ss.GetTiles()[key(5, 1)]; ok {
				t.Fatal("a player's viewpoint must not buy them an NPC's room")
			}
		}
	}
}

// VTT-262 VTT-222
func TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt(t *testing.T) {
	const why = "the floor gives way under the hero"
	move := func() *vttv1.Envelope {
		in := envelope(8, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
			From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1},
			Reason: why})
		in.EventId, in.ParticipantId, in.ActorRole, in.SessionId = "evt-8", "p-dm", "dm", "sess-1"
		return in
	}
	for _, v := range []gateway.Viewer{player(),
		{ParticipantID: "sp-1", Role: identity.RoleSpectator, Viewpoint: "hero"}} {
		st := twoRooms()
		pr := gateway.NewProjector(v)
		firstPlace(pr, st)
		mustApply(st, 8, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
			From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})
		in := move()
		before := proto.Clone(in)

		var sent *vttv1.Envelope
		for _, e := range pr.Project(in, st) {
			if e.GetTokenMoved() != nil {
				sent = e
			}
		}
		if sent == nil {
			t.Fatalf("%s: the move was not forwarded", v.Role)
		}
		if !proto.Equal(sent, forwardedOf(in)) {
			t.Errorf("%s: forwarded %v, want the event less its reason and issuer", v.Role, sent)
		}
		if !proto.Equal(in, before) {
			t.Errorf("%s: projecting the move changed the event", v.Role)
		}
	}

	dm := gateway.NewProjector(gateway.Viewer{ParticipantID: "dm", Role: identity.RoleDM})
	if out := dm.Project(move(), twoRooms()); len(out) != 1 || out[0].GetTokenMoved().GetReason() != why {
		t.Errorf("the DM must be sent the move with its reason, got %v", out)
	}
}

func TestTheProjectionFailsClosedWhenItHasNothingToGoOn(t *testing.T) {
	// Keep all three closed though no pump reaches them: the projection must not
	// depend on its caller being careful (SPEC-016).
	st := twoRooms()

	if out := gateway.NewProjector(player()).Project(nil, st); out != nil {
		t.Errorf("no event to project must yield nothing, got %v", out)
	}
	if out := gateway.NewProjector(player()).Project(envelope(1, &vttv1.SessionStarted{Name: "n"}), nil); out != nil {
		t.Errorf("no world to look at must yield nothing, got %v", out)
	}
	// identity.Role is a string, so an unknown one is a value the type
	// permits and this switch has never heard of.
	stranger := gateway.Viewer{ParticipantID: "p-1", Role: identity.Role("archivist")}
	if out := firstPlace(gateway.NewProjector(stranger), st); out != nil {
		t.Errorf("an unrecognised role must be shown nothing, got %v", out)
	}
}

// forwardedOf is env as a player or spectator is sent it (SPEC-016); keep it equal to perceive's.
func forwardedOf(env *vttv1.Envelope) *vttv1.Envelope {
	c := proto.Clone(env).(*vttv1.Envelope)
	c.ParticipantId, c.ActorRole = "", ""
	if m := c.GetTokenMoved(); m != nil {
		m.Reason = ""
	}
	if r := c.GetResourceChanged(); r != nil {
		r.Reason = ""
	}
	if a := c.GetConditionApplied(); a != nil {
		a.Source = ""
	}
	if r := c.GetConditionRemoved(); r != nil {
		r.Reason = ""
	}
	return c
}
