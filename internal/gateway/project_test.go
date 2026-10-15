package gateway_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// key formats a square the way the wire does: column then row, comma
// separated (maps-as-geometry spec §4.1). Same format as engine's gridKey and
// sight's squareKey.
func key(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

// envelope wraps a payload in an Envelope at the given sequence.
//
// One switch, and it PANICS on a payload it does not know rather than
// returning a payload-less Envelope: a silently empty Envelope would sail
// through the projection's unrecognised arm and make whatever test used it
// pass for the wrong reason. These tests exist to catch exactly that class of
// mistake in the code under test, so the helper must not commit it.
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
	// EVERY ACTOR DECLARES ITS KIND, and both of them have to: an absent kind
	// is NOT a party member, always, with no second branch that reads control
	// instead (spec §5.1, Patrik's ruling 2026-08-24). A hero left silent here
	// would drop out of its own player's roster and off every shoulder, and
	// the suite below would be asserting the wrong world.
	//
	// AND CONTROL IS A SEPARATE EVENT, at the end rather than at creation,
	// because that is the only way an actor can hold one now — and because it
	// is the order a table actually plays: you log in with no character and
	// the DM assigns one afterwards.
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

// TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt pins the SHAPE of
// an introduction, which until this test was pinned only by files a regression
// can regenerate.
//
// MEASURED, in review: deleting the grant loop in transitions fails only
// TestTheProjectedGoldensAreWhatTheProjectionActuallySends, and emitting the
// grants BEFORE the ActorAdded fails only that plus the fold-parity gate.
// Both are committed-bytes comparisons over one scenario — re-record them and
// the rule is gone with nothing to say so. This asserts the rule itself.
//
// Three claims, and each is a separate way to get it wrong:
//
//   - the ActorAdded carries NO controller, in either spelling. It cannot: both
//     folds refuse one ("creating an actor does not hand it to anyone"), and a
//     clone of live state carries whatever grants have accumulated.
//   - exactly one grant per controller, IN THE SET'S OWN ORDER, so the mirror
//     (controller_ids[0]) lands on the same participant the server has.
//   - the batch FOLDS, which is the property the other two exist to protect.

// VTT-209
func TestAnIntroductionCarriesNoControllerAndTheGrantsBehindIt(t *testing.T) {
	st := twoRooms()
	// A SHARED party member, because one controller cannot tell "a grant per
	// controller" from "a grant" and cannot show order at all.
	mustApply(st, 8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	mustApply(st, 10, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-3",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	if ids := st.Actors["scout"].GetControllerIds(); len(ids) != 2 {
		t.Fatalf("fixture check: the scout must be shared, got %v", ids)
	}

	// Projected to a player who is NOT one of its controllers, so the scout
	// reaches them by the roster exception rather than by being their own.
	pr := gateway.NewProjector(player())
	place := &vttv1.TokenPlaced{TokenId: "t-scout", SceneId: "s", ActorId: "scout",
		Position: &vttv1.GridPosition{X: 4, Y: 1}} // behind the shut door: unseen
	mustApply(st, 11, place)
	out := pr.Project(envelope(11, place), st)

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

	// And the whole batch folds, which is the property the assertions above
	// exist to protect rather than a restatement of them.
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

// VTT-208
func TestAPartyMemberStaysKnownEvenWhenOutOfSight(t *testing.T) {
	// Spec §5: party members are ALWAYS known — and §5.1 decides what counts
	// as one. "Rogue" here SAYS it is a party member, which is now the only
	// way to be one: an absent kind is not a party member, whoever holds the
	// actor. You know your party exists when the rogue is two rooms away; you
	// merely cannot see their token. Dropping them from your own roster
	// because they turned a corner reads as a bug, not as fog.
	//
	// It belongs to ANOTHER player (p-2, granted below), so this is the party
	// half of the exception rather than the "your own character" half.
	rogue := &vttv1.Actor{ActorId: "rogue", Name: "Rogue",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: rogue})
	mustApply(st, 9, &vttv1.TokenPlaced{TokenId: "t-rogue", SceneId: "s",
		ActorId: "rogue", Position: &vttv1.GridPosition{X: 5, Y: 1}}) // behind the closed door
	mustApply(st, 10, &vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})

	pr := gateway.NewProjector(player())
	out := pr.Project(envelope(8, &vttv1.ActorAdded{Actor: rogue}), st)

	var knowsRogue bool
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil && a.GetActor().GetActorId() == "rogue" {
			knowsRogue = true
		}
	}
	if !knowsRogue {
		t.Error("a player-controlled actor must reach every player's roster even out of sight")
	}

	// ...but their TOKEN must not, because creatures are pure line of sight.
	out = pr.Project(envelope(9, &vttv1.TokenPlaced{TokenId: "t-rogue",
		SceneId: "s", ActorId: "rogue",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)
	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-rogue" {
			t.Error("the rogue is behind a closed door — knowing they exist is not seeing them")
		}
	}
}

// actorIDsIn is the viewer's ROSTER as it reaches their wire: every actor id a
// batch of projected envelopes introduces. Reading the envelopes rather than
// the Projector's internals is the point — spec §5's exception is about what a
// player receives, and a test that asked pr.actors would pin an internal.
func actorIDsIn(out []*vttv1.Envelope) map[string]bool {
	ids := map[string]bool{}
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil {
			ids[a.GetActor().GetActorId()] = true
		}
	}
	return ids
}

// VTT-194 VTT-242
func TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster(t *testing.T) {
	// THE LEAK, stated as a test — the whole-branch review's finding I1, in a
	// shape the keystone structurally cannot provide: §4.3's oracle
	// TRANSCRIBES the same predicate, so both sides of that equation agree
	// while both are wrong.
	//
	// The goblin archer stands at (5,1) behind a SHUT door, so nothing this
	// player can see introduces it. Then the DM takes control of it, which is
	// an ordinary act: grant_actor_control constrains the ISSUER (DM/agent
	// only, authz.go) and says nothing about the TARGET. Under a "has any
	// controller" roster that one grant publishes the whole cloned Actor —
	// name, attributes, resources, module_data — to every player at the table.
	st := twoRooms()
	grant := &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "dm-1"}
	mustApply(st, 8, grant)

	pr := gateway.NewProjector(player())
	roster := actorIDsIn(pr.Project(envelope(8, grant), st))

	if !roster["hero"] {
		t.Fatal("fixture check: the player's own character must be on this roster, " +
			"or the absence asserted below proves nothing")
	}
	if roster["goblin"] {
		t.Error("a monster the DM happens to hold is still a monster (spec §5.1): " +
			"it must not reach a player's roster")
	}
}

// shippedActor returns one actor from the REAL committed goblin-ambush
// adventure, compiled exactly as load_adventure would put it in the log.
//
// It reads from disk on purpose, and what that buys is narrower than it looks
// — narrow enough to state exactly. The BEHAVIOURAL half of the test below
// would reproduce against `&vttv1.Actor{ActorId: "act-archer"}`: actorIDsIn
// reads ids off ActorAdded envelopes and nothing asserts the archer's shipped
// attributes or resources.
//
// What only shipped sourcing buys is the FIXTURE CHECK — that the compiled
// actor's kind is the one its author wrote down. Task 1 shipped a green suite
// while the exposure it was closing was still live end to end, because every
// fixture that EXPRESSED the leak declared ACTOR_KIND_NON_PARTY by hand
// (twoRooms' hero and the kindless-actor fixtures elsewhere in this file are
// deliberately kindless, so the claim is about the leak fixtures, not about
// all of them). A hand-built actor cannot notice what adventure content can
// and cannot say.
//
// The named example this paragraph used to give,
// TestAnActorFromBeforeTheKindFieldIsAPartyMemberWhenSomeoneControlsIt, was
// deleted with the migration rule on 2026-08-24 and the reference outlived it.
// Two more of the same dangling name survive at keystone_test.go:182 and
// viewpoint_test.go:48; they are not this task's to move, but they are the
// same rot and somebody should.
//
// WHAT IT COULD NOT SAY, UNTIL TASK 7, WAS ANYTHING. actorJSON accepted four
// keys — actor_id, name, attributes, resources — and decodeStrict calls
// DisallowUnknownFields, so a "kind" in an adventure file was a boot-time
// error rather than a value: every shipped actor compiled to UNSPECIFIED,
// including the Human Fighter standing beside the two goblins. These fixture
// checks used to assert exactly that, with a note saying that the day it
// changed, §5.1's first rule wanted rereading. It changed; the rule is being
// reread; and the checks now assert the author's own word instead, which is a
// stronger thing to pin than an absence.
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
	// THE ARCHER THIS ARC IS NAMED AFTER, against shipped content.
	//
	// Task 1 closed the mechanism and left the exposure open, and said so:
	// adventure content COULD NOT express a kind at all, so every actor in both
	// committed adventures was UNSPECIFIED with no controller. One
	// grant_actor_control on the Goblin Archer — an ordinary act, since a
	// grant constrains the ISSUER and says nothing about the TARGET — gave it
	// a controller, and UNSPECIFIED plus a controller was what §5.1's
	// migration rule then read as a party member. The whole cloned Actor went
	// to every player at the table, and MayPerch and eyes() opened on it.
	// (Both halves are gone as of 2026-08-24: the migration rule was deleted,
	// and Task 7 gave the adventure format a kind field the archer now uses.)
	//
	// What closes it is that the GRANT now declares the kind. The agent
	// taking the archer to run it says what it is taking, and the archer stays
	// a monster.
	//
	// AND SINCE TASK 7, THE ARCHER SAYS SO ITSELF: adventures/goblin-ambush/
	// actors/act-archer.json declares "non_party", so the leak is closed at
	// birth as well as at the grant. That makes this test belt-and-braces
	// rather than the only defence, and it is worth keeping in exactly that
	// role — it is the one test that walks the whole path the bug took, from a
	// file on disk to a player's roster.
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
	// THE OTHER HALF OF THE AMBIGUITY, and the reason no rule could have
	// resolved it before. This grant is byte-identical to the one above in
	// every field the contract used to carry — THE SAME ACTOR, the same shape,
	// a different participant — and it must come out the other way. An agent
	// taking a goblin to run it, and a player being handed that same goblin as
	// theirs (charmed, tamed, whatever the fiction is), are the same event
	// until somebody is asked.
	//
	// THE ARCHER, NOT THE FIGHTER, and that is the whole bite (review finding,
	// 2026-08-24). This test used to seed act-fighter, which worked only while
	// every shipped actor compiled to UNSPECIFIED: since Task 7 the Human
	// Fighter's own file says "party_member", so it would have reached the
	// roster with this grant's Kind DELETED and the assertion would have gone
	// on passing. The archer is born NON_PARTY, so the grant is the only thing
	// that can put it there — which is what this test claims to be about.
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

// TestTheShippedHumanFighterIsAPartyMemberBeforeAnybodyIsAssignedToIt is what
// Task 7 buys, and it is the case §5.1's first rule currently denies.
//
// goblin-ambush ships a Human Fighter beside two goblins, and cellar-rats
// ships Hollis Ketch and Mara Voss. All three are player characters, written
// down as such by the person who made them, and until this task the file had
// no way to say so — actorJSON took four keys and none of them was what the
// creature IS. So the AUTHORED path, the one written deliberately in advance
// by somebody who knew exactly what they were making, was the only path that
// could not speak, while the improvised one (add_actor) is compelled to. That
// was backwards.
//
// NOBODY IS GRANTED ANYTHING HERE, deliberately. A pregenerated character
// sitting in a campaign before assignment IS a party member — the party seeing
// the four available sheets is correct, not a leak — and §5.1's "an ungranted
// actor is NOT a party member" is the sentence that makes this test fail on
// paper while passing in fact. Patrik is amending it.
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

// TestTheShippedGoblinArcherCannotBeGivenAControllerAtCreation closes the last
// route by which the archer this arc is named after could become a party
// member without anybody saying so.
//
// Two commands could confer control. grant_actor_control was made to declare a
// kind (Task 2), and add_actor was not — so a caller who wanted the old
// behaviour still had it: create the archer with a controller_id, state no
// kind, and §5.1's migration rule as it then stood read that as a party member
// on the spot. Same leak, different door, and reachable from both seats that
// may issue the command.
//
// OVER THE WIRE, against SHIPPED content, because that is where the previous
// two attempts at this rule were green while the exposure was live: every
// fixture that expressed the leak declared its own kind by hand, and the
// shipped archer could not. The fold refuses the same shape (engine.Apply's
// ActorAdded arm), so this is not the only place the rule lives — it is the
// place a caller finds out, and the refusal it asserts names the command that
// does confer control.
//
// THE REFUSAL IT PINS IS THE CONTROLLER ONE, and the shipped archer now states
// a kind, which is what keeps that true: an actor carrying a controller AND no
// kind would be refused either way since Task 7, and this test would go on
// passing with the controller check deleted.
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

// TestCreatingAnActorAndThenGrantingItIsTheTwoStepThatReplacesTheOne is the
// control, and it is where this task's cost is written down rather than
// implied: an agent creating an NPC it will run now sends TWO commands.
//
// The trade is two round trips against two rules that must stay in sync
// forever. RPTool shipped the second and carries the resulting invariant bug
// in its tree today (clearAllOwners leaves ownerType == OWNER_TYPE_ALL, worked
// around at EditTokenDialog.java:885).
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
	// The half the one-step could never state: a player holding a monster is
	// still holding a monster (§5.1, "control describes who is driving").
	if got := actor.GetKind(); got != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Errorf("kind = %v, want ACTOR_KIND_NON_PARTY — the grant declares it and nothing else does", got)
	}
}

// TestAKindlessGrantConfersControlAndNothingElse is the INVERSION of
// TestAnOldLogsKindlessGrantStillMakesAPartyMember, which stood here until
// 2026-08-24 and asserted the opposite.
//
// That test pinned the migration rule at the grant: a grant with no kind, over
// an actor with no kind, made a party member because a controller was now
// present. It was right for as long as the rule existed to protect logs
// written before the field did. There are none, the rule is deleted, and the
// grant path is where its absence is most worth stating: a grant is the ONE
// event that confers control, so "control alone confers nothing" has to be
// true HERE or it is not true anywhere.
//
// The command boundary refuses a kindless grant (validateGrantActorControl),
// which is why this has to build the event by hand. Belt and braces, in that
// order: the refusal stops it being written, and this says what it would mean
// if it were.
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
	pr := gateway.NewProjector(player())
	roster := actorIDsIn(pr.Project(envelope(10, silent), st))
	if !roster["hero"] {
		t.Fatal("fixture check: the player's own character must be on this roster, " +
			"or the absence asserted below proves nothing")
	}
	if roster["cleric"] {
		t.Error("a grant that said nothing about what it was granting made a party member: " +
			"control confers control, and nothing else")
	}
}

// VTT-208
func TestAPartyMemberIsKnownEvenWhenHeldByTheDM(t *testing.T) {
	// THE OTHER DIRECTION, and the reason kind belongs to the ACTOR rather
	// than to whoever holds it. A player's character run by the DM while its
	// player is offline is STILL a party member (spec §5.1), and the party must
	// still know they exist. A rule keyed on the CONTROLLER'S ROLE — the
	// obvious repair, and the wrong one — drops them from every roster.
	st := twoRooms()
	cleric := &vttv1.Actor{ActorId: "cleric", Name: "Cleric",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}
	mustApply(st, 8, &vttv1.ActorAdded{Actor: cleric})
	// Handed to the DM by a grant — the only way anyone holds anything now —
	// and the grant AGREES with the actor's own declaration. A party member
	// whose player is away is still a party member, so the DM taking it over
	// says so again rather than restating it as something else.
	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "cleric", ParticipantId: "dm-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	place := &vttv1.TokenPlaced{TokenId: "t-cleric", SceneId: "s",
		ActorId: "cleric", Position: &vttv1.GridPosition{X: 5, Y: 1}} // behind the shut door
	mustApply(st, 10, place)

	pr := gateway.NewProjector(player())
	roster := actorIDsIn(pr.Project(envelope(10, place), st))

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
	out := runLog(player(), []step{
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

// TestAnActorWithNoDeclaredKindIsNotAPartyMemberWhoeverHoldsIt is the rule
// that REPLACED the migration rule, and it is the inversion of a test that
// stood here until 2026-08-24.
//
// The old pair pinned two branches: absent kind + a controller meant party
// member; absent kind + nobody meant not. The first branch existed only to
// keep logs written before the kind field behaving as they had, and Patrik
// ruled it out on the ground that there are no such logs — no campaign exists
// outside this repo's own fixtures.
//
// DELETING IT IS THE POINT, not a tidy-up. That branch is precisely what could
// not tell "a log written before kind existed" from "a grant issued today that
// forgot", and every leak this arc chased lived in that gap. One rule now: an
// absent kind is NOT a party member, always, and nothing reads controller_ids
// to decide. The ghost below is the exact fixture the deleted test asserted the
// OPPOSITE about — kept, rather than removed, so the change is visible as a
// change.
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
				// A KINDLESS grant, which the command boundary refuses and the
				// fold still accepts — the only remaining way an actor can hold
				// a controller while saying nothing about what it is.
				mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: tc.id, ParticipantId: tc.holder})
			}
			place := &vttv1.TokenPlaced{TokenId: "t-" + tc.id, SceneId: "s",
				ActorId: tc.id, Position: &vttv1.GridPosition{X: 5, Y: 1}} // behind the shut door
			mustApply(st, 10, place)

			pr := gateway.NewProjector(player())
			roster := actorIDsIn(pr.Project(envelope(10, place), st))
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

// VTT-208
func TestASpectatorGetsNoSightFromAnNPCTheDMControls(t *testing.T) {
	// The SAME defect at the perch, which is why one rule has to govern both
	// (spec §5.1, "one rule, both call sites"). §3.1.1 calls this the
	// constraint the whole idea rests on: a watcher perched on the Goblin
	// Archer sees the ambush from INSIDE it. Fixing only the roster would
	// leave that open to any DM who takes control of a monster.
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "dm-1"})

	pr := gateway.NewProjector(gateway.Viewer{
		ParticipantID: "sp-1", Role: identity.RoleSpectator, Viewpoint: "goblin"})

	out := firstPlace(pr, st)
	// The roster still reaches a watcher — you cannot choose a shoulder you
	// have never been told about (spec §5). It is also what stops the loop
	// below passing vacuously on an empty slice, which is the shape this
	// file's own TestAnNPCHeldByTheDMIsNotPublishedToThePartysRoster guards.
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
	// FAIL CLOSED (spec §4.4). A payload the projection does not understand
	// must NOT be forwarded — that default is how this ships broken.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	out := pr.Project(&vttv1.Envelope{Sequence: 99}, st) // no payload at all
	if len(out) != 0 {
		t.Fatalf("an unknown payload must be withheld from a player, got %d", len(out))
	}
}

// TestAnUnrecognisedPayloadIsWithheldFromASpectator is the spectator's half of
// TestAnUnrecognisedPayloadIsWithheldFromAPlayer. Keep the control: a fixture
// with nothing to send passes the first check whatever Project does.
// VTT-227
func TestAnUnrecognisedPayloadIsWithheldFromASpectator(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{
		ParticipantID: "sp-1", Role: identity.RoleSpectator, Viewpoint: "hero"})
	if out := pr.Project(&vttv1.Envelope{Sequence: 99}, st); len(out) != 0 {
		t.Fatalf("an unknown payload must be withheld from a spectator, got %d frames", len(out))
	}
	if sceneSeenIn(firstPlace(pr, st), "s") == nil {
		t.Fatal("the control fails: the next recognised event must bring the shoulder's board")
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

// sceneSeenIn picks the SceneSeen for one scene out of a batch, or nil.
func sceneSeenIn(out []*vttv1.Envelope, sceneID string) *vttv1.SceneSeen {
	var found *vttv1.SceneSeen
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil && ss.GetSceneId() == sceneID {
			found = ss
		}
	}
	return found
}

func TestAPlayerCannotBorrowAnNpcsEyesByPerching(t *testing.T) {
	// Perching is the SPECTATOR's affordance (spec §3.1.1: "an unassigned
	// PLAYER does not perch"). A player who sets a viewpoint must keep seeing
	// exactly what their own actors see — otherwise Viewpoint is a
	// client-settable field that reopens session zero.
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

// step is one event of a hand-built log: the sequence it takes and the
// payload it carries.
type step struct {
	seq     int64
	payload proto.Message
}

// VTT-210 VTT-249
func TestAConditionAppliedOutOfSightArrivesWithTheActor(t *testing.T) {
	// Both folds refuse to remove a condition they never received, so an
	// introduction carries the conditions applied while the actor was unseen.
	st := twoRooms()
	mustApply(st, 8, &vttv1.ConditionApplied{
		ActorId: "goblin", ConditionId: "marked", Source: "the DM"})

	pr := gateway.NewProjector(player())

	// While the goblin is behind the closed door the condition must not leak:
	// it names an actor this player has never seen.
	//
	// The batch is KEPT, not discarded, and that is not tidiness. A BATCH IS
	// NOT A FOLD UNIT — the stream is. This is the first event a fresh
	// projector ever saw, so this batch is where the player is given their own
	// scene, their own actor and their own token; the batches after it carry
	// only the diff. Folding a later batch into an empty state therefore fails
	// on the projection's behalf ("token placed in unknown scene"), which says
	// nothing about the projection and everything about where the test cut the
	// stream.
	intro := pr.Project(envelope(8, &vttv1.ConditionApplied{
		ActorId: "goblin", ConditionId: "marked", Source: "the DM"}), st)
	for _, e := range intro {
		if e.GetConditionApplied() != nil {
			t.Fatal("a condition on an unseen actor names it, and must be withheld")
		}
	}

	// The door opens, the goblin is introduced — and its condition with it.
	mustApply(st, 9, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.Project(envelope(9, &vttv1.DoorOpened{
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

	// And the WHOLE PROJECTED STREAM folds, in order, with the removal on the
	// end — the assertion that would have caught this as a client freeze. From
	// the beginning, because that is the only thing a client ever folds: it
	// receives every batch this projector emits, in order, starting from the
	// first.
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
	for _, e := range pr.Project(envelope(10, &vttv1.ConditionRemoved{
		ActorId: "goblin", ConditionId: "marked"}), st) {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("the removal does not fold, which is a permanent client freeze: %v", err)
		}
	}
	if n := len(viewerState.Conditions["goblin"]); n != 0 {
		t.Errorf("the condition was removed and must be gone, got %d", n)
	}
}

// runLogSteps folds each step and projects it, exactly as the pump will, and
// returns one batch per step.
func runLogSteps(v gateway.Viewer, steps []step) [][]*vttv1.Envelope {
	st := engine.NewState()
	pr := gateway.NewProjector(v)
	out := make([][]*vttv1.Envelope, len(steps))
	for i, s := range steps {
		env := envelope(s.seq, s.payload)
		if err := engine.Apply(st, env); err != nil {
			panic(fmt.Sprintf("runLog seq %d: %v", s.seq, err))
		}
		out[i] = pr.Project(env, st)
	}
	return out
}

func runLog(v gateway.Viewer, steps []step) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, batch := range runLogSteps(v, steps) {
		out = append(out, batch...)
	}
	return out
}

// removeActorBatch projects the batch remove_actor emits — one TokenRemoved
// per token of the actor, in token-id order, then the ActorRemoved — through
// pr, folding each event into st first, exactly as the pump does. It returns
// everything the viewer was sent, in order.
//
// The order is the handler's (gateway handleRemoveActor); reproducing it here
// rather than asserting against a hand-picked single event is what makes these
// tests about the REAL sequence a seat receives.
func removeActorBatch(pr *gateway.Projector, st *engine.State, seq int64,
	actorID string, tokenIDs []string) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, id := range tokenIDs {
		mustApply(st, seq, &vttv1.TokenRemoved{TokenId: id})
		out = append(out, pr.Project(envelope(seq, &vttv1.TokenRemoved{TokenId: id}), st)...)
		seq++
	}
	mustApply(st, seq, &vttv1.ActorRemoved{ActorId: actorID})
	return append(out, pr.Project(envelope(seq, &vttv1.ActorRemoved{ActorId: actorID}), st)...)
}

// TestARemovedActorReachesOnlyTheSeatThatHeldIt holds both directions of the
// removal ruling: an actor the seat never held is not announced, since its
// fold would refuse the removal, and the seat's own character, which it sees,
// is (SPEC-016).
// VTT-215
func TestARemovedActorReachesOnlyTheSeatThatHeldIt(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())
	intro := firstPlace(pr, st)

	// The GOBLIN first: this player has never seen it (it stands behind a
	// closed door) and its removal must reach them as nothing at all.
	unseen := removeActorBatch(pr, st, 8, "goblin", []string{"t-gob"})
	for _, e := range unseen {
		if e.GetActorRemoved() != nil {
			t.Fatal("an actor this seat never held must not be announced as removed — it would name a creature they never saw")
		}
	}

	// Then their OWN character, which they see as an eye: its removal reaches them.
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

	// And everything this seat was sent, from the introductions onward, folds
	// through the SAME Apply the client mirrors — in the order it arrived.
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
	pr := gateway.NewProjector(player())
	stream := firstPlace(pr, st)

	stream = append(stream, removeActorBatch(pr, st, 8, "hero", []string{"t-hero"})...)

	// The same id, added again — a fresh party member, granted to the same
	// player, with a token of its own.
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
		stream = append(stream, pr.Project(envelope(step.seq, step.payload), st)...)
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

func TestAnEventNamingAnUnknownActorIsWithheld(t *testing.T) {
	// Spec §4.4 names three of these: "AttackRolled names a target,
	// ConditionApplied names an actor". None of them carries a position, so
	// what leaks is EXISTENCE — an attack roll against a creature the player
	// has never seen tells them there is one, and a condition on it tells
	// them what it is doing. Both are session zero's finding at the roster
	// layer rather than the board layer.
	st := twoRooms()
	pr := gateway.NewProjector(player())
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
		for _, e := range pr.Project(in, st) {
			if e == in {
				t.Errorf("%s: an event naming an actor this player has never seen must be withheld", h.name)
			}
		}
	}

	// The control: the SAME payloads about an actor they do know go through,
	// or "withhold everything" would satisfy the loop above.
	known := &vttv1.Envelope{Sequence: 8, EventId: "evt-8", Payload: &vttv1.Envelope_ConditionApplied{
		ConditionApplied: &vttv1.ConditionApplied{ActorId: "hero", ConditionId: "hidden"}}}
	var forwarded bool
	for _, e := range pr.Project(known, st) {
		if e.GetEventId() != "" && proto.Equal(e, forwardedOf(known)) {
			forwarded = true
		}
	}
	if !forwarded {
		t.Error("a condition on the player's own character must reach them")
	}
}

func TestTheProjectionFailsClosedWhenItHasNothingToGoOn(t *testing.T) {
	// Spec §4.4's direction, at the three doors into this function. None of
	// these is reachable from today's pump, which is the point: the projection
	// is security-critical code (spec §8) and must not depend on its caller
	// being careful.
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

// VTT-216
func TestNarrationReachesAPlayer(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	narration := envelope(8, &vttv1.NarrationAdded{Text: "The door creaks.", As: "DM"})
	var toldStory bool
	for _, e := range pr.Project(narration, st) {
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
	pr     *gateway.Projector
	folded *engine.State
}

func noteSeats() []*noteSeat {
	return []*noteSeat{
		{name: "player", pr: gateway.NewProjector(player()), folded: engine.NewState()},
		{name: "spectator", pr: gateway.NewProjector(gateway.Viewer{
			ParticipantID: "p-watch", Role: identity.RoleSpectator}), folded: engine.NewState()},
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
		out := s.pr.Project(env, st)
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
	st, seats := engine.NewState(), noteSeats()
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
	seats := append(noteSeats(),
		&noteSeat{name: "player with no actor", pr: gateway.NewProjector(
			gateway.Viewer{ParticipantID: "p-9", Role: identity.RolePlayer}), folded: engine.NewState()},
		&noteSeat{name: "spectator on the hero", pr: gateway.NewProjector(gateway.Viewer{
			ParticipantID: "p-bird", Role: identity.RoleSpectator, Viewpoint: "hero"}), folded: engine.NewState()},
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
	st, seats := engine.NewState(), noteSeats()
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
			st, seats := engine.NewState(), noteSeats()
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
	st, seats := engine.NewState(), noteSeats()
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
	st, seats := engine.NewState(), noteSeats()
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

// --- the removal batch, as bytes both folds have to accept -------------------

// removalBatchFixture is the seat's whole stream for the mixed-visibility
// removal batch, shared with the TypeScript side.
//
// contract/testdata is the ONE directory both languages already read
// (contract/roundtrip_test.go and contract/events.test.ts split every fixture
// in it), which is why this lives there rather than beside either test.
const removalBatchFixture = "../../contract/testdata/removal_batch_projected_stream.json"

// removalBatchWorld is twoRooms with the goblin holding TWO tokens on opposite
// sides of the closed door: t-gob at 2,1 in the hero's own room, t-gob-far at
// 5,1 behind it. The seat is the hero's player, so it is SHOWN the goblin (and
// therefore "holds" it, in classify's sense) while never learning that
// t-gob-far exists at all.
//
// It is not twoRooms() itself because twoRooms puts the goblin's only token
// behind the door: every seat there sees all of the goblin or none of it, and
// the mixed case is the one nothing in this suite reaches.
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

// projectRemovalBatchSeat replays this world to the hero's player and returns
// everything that seat receives: the introductions carried by the first event
// it is projected, the three-envelope batch remove_actor emits, and the id
// used again by a second goblin placed in sight.
func projectRemovalBatchSeat(st *engine.State) []*vttv1.Envelope {
	pr := gateway.NewProjector(player())
	stream := pr.Project(envelope(7, &vttv1.TokenPlaced{TokenId: "t-gob-far",
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
		stream = append(stream, pr.Project(envelope(s.seq, s.payload), st)...)
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
	stream := projectRemovalBatchSeat(removalBatchWorld())

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

	// COMPARED WITH proto.Equal RATHER THAN BYTES, deliberately: protojson's
	// output spacing is randomised per binary (two regimes, chosen by a hash
	// of the build), so a byte comparison of re-marshalled output would fail
	// on some builds and pass on others. The fixture is still the authority on
	// content — it is just read as messages.
	if len(stream) != len(want) {
		t.Fatalf("the seat received %d envelopes, the fixture holds %d", len(stream), len(want))
	}
	for i := range want {
		if !proto.Equal(stream[i], want[i]) {
			t.Errorf("envelope %d differs\n got: %v\nwant: %v", i, stream[i], want[i])
		}
	}

	// THE MIXED HALF, asserted by name so a fixture regenerated from a world
	// where BOTH tokens were visible could not quietly pass the comparison
	// above. t-gob-far must appear nowhere in this seat's stream: not placed,
	// not hidden, not removed.
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

// testimonyLog is twoRooms as a log, with a party member of another player,
// the rogue, granted in the hero's room, and extra appended from sequence 11.
func testimonyLog(extra ...proto.Message) []step {
	steps := []step{
		{1, &vttv1.SessionStarted{Name: "n"}},
		{2, &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3,
			Tiles: twoRoomsTiles()}},
		{3, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "Hero",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}},
		{4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}},
		{5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin",
			Position: &vttv1.GridPosition{X: 5, Y: 1}}},
		{7, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		{8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "rogue", Name: "Rogue",
			Kind:      vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Resources: map[string]*vttv1.Resource{"pool": {Current: 5, Max: 5}}}}},
		{9, &vttv1.TokenPlaced{TokenId: "t-rogue", SceneId: "s", ActorId: "rogue",
			Position: &vttv1.GridPosition{X: 2, Y: 1}}},
		{10, &vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-2",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
	}
	for i, p := range extra {
		steps = append(steps, step{int64(11 + i), p})
	}
	return steps
}

var testimonyDoor = &vttv1.GridPosition{X: 3, Y: 1}

// rogueLeaves takes the rogue behind the shut door.
var rogueLeaves = &vttv1.TokenMoved{TokenId: "t-rogue", SceneId: "s", To: &vttv1.GridPosition{X: 4, Y: 1}}

// runTestimony projects steps for v as a seat does, each event stamped with
// the metadata a log carries, so a forwarded frame can be told from a built one.
func runTestimony(v gateway.Viewer, steps []step) [][]*vttv1.Envelope {
	st := engine.NewState()
	pr := gateway.NewProjector(v)
	out := make([][]*vttv1.Envelope, len(steps))
	for i, s := range steps {
		env := envelope(s.seq, s.payload)
		env.EventId = fmt.Sprintf("evt-%d", s.seq)
		env.ActorRole = "dm"
		env.ParticipantId = "p-dm"
		env.SessionId = "sess-1"
		if err := engine.Apply(st, env); err != nil {
			panic(fmt.Sprintf("runTestimony seq %d: %v", s.seq, err))
		}
		out[i] = pr.Project(env, st)
	}
	return out
}

// foldSteps folds every frame a seat was sent, failing on the first refused.
func foldSteps(t *testing.T, frames [][]*vttv1.Envelope) *engine.State {
	t.Helper()
	viewer := engine.NewState()
	for i, batch := range frames {
		for _, e := range batch {
			if err := engine.Apply(viewer, e); err != nil {
				t.Fatalf("step %d: projected %T does not fold: %v", i, e.GetPayload(), err)
			}
		}
	}
	return viewer
}

// VTT-243
func TestAConditionOnACreatureBehindAShutDoorReachesNoPlayer(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
		&vttv1.DoorClosed{SceneId: "s", At: testimonyDoor},
		&vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "marked"},
	))
	viewer := foldSteps(t, frames)
	if _, ok := viewer.Actors["goblin"]; !ok {
		t.Fatal("fixture check: the player must have seen the goblin while the door stood open")
	}
	if out := frames[12]; len(out) != 0 {
		t.Errorf("a condition on a goblin behind a shut door reached the player: %v", out)
	}
}

// VTT-243
func TestAPartyMemberInAnotherSceneIsNotHeardOf(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		&vttv1.SceneCreated{SceneId: "far", Name: "Far", GridWidth: 3, GridHeight: 3},
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.TokenPlaced{TokenId: "t-rogue-far", SceneId: "far", ActorId: "rogue",
			Position: &vttv1.GridPosition{X: 1, Y: 1}},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "marked"},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3},
		&vttv1.ConditionRemoved{ActorId: "rogue", ConditionId: "marked"},
	))
	viewer := foldSteps(t, frames)
	if _, ok := viewer.Actors["rogue"]; !ok {
		t.Fatal("fixture check: a party member stays on every roster (VTT-208)")
	}
	for i := 13; i <= 15; i++ {
		if len(frames[i]) != 0 {
			t.Errorf("step %d about a party member in another scene reached the player: %v", i, frames[i])
		}
	}
	if got := viewer.Actors["rogue"].GetResources()["pool"].GetCurrent(); got != 5 {
		t.Errorf("the player's rogue holds pool %d, want the 5 it was introduced with", got)
	}
}

// VTT-244
func TestAnUnseenPartyMembersAttackReachesNoPlayer(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		rogueLeaves,
		&vttv1.AttackRolled{AttackerId: "rogue", TargetId: "hero", Total: 12, Outcome: "hit"},
		&vttv1.AbilityUsed{ActorId: "rogue", AbilityId: "taunt", TargetIds: []string{"hero"}},
		&vttv1.AttackRolled{AttackerId: "hero", TargetId: "goblin", Total: 9, Outcome: "miss"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
		&vttv1.AttackRolled{AttackerId: "hero", TargetId: "goblin", Total: 15, Outcome: "hit"},
	))
	foldSteps(t, frames)
	for i, what := range map[int]string{11: "the rogue's attack on the hero", 12: "the rogue's ability on the hero",
		13: "the hero's attack on a goblin it does not see"} {
		if len(frames[i]) != 0 {
			t.Errorf("%s reached the player: %v", what, frames[i])
		}
	}
	if out := frames[15]; len(out) != 1 || out[0].GetAttackRolled() == nil {
		t.Errorf("the hero's attack on a goblin it sees did not reach the player, got %v", out)
	}
}

// VTT-245
func TestAControlChangeOnAnUnseenPartyMemberReachesNoPlayer(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		rogueLeaves,
		&vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-3",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		&vttv1.ActorControlRevoked{ActorId: "rogue", ParticipantId: "p-2"},
	))
	viewer := foldSteps(t, frames)
	for i := 11; i <= 12; i++ {
		if len(frames[i]) != 0 {
			t.Errorf("a control change on the unseen rogue reached the player: %v", frames[i])
		}
	}
	if ids := viewer.Actors["rogue"].GetControllerIds(); len(ids) != 1 || ids[0] != "p-2" {
		t.Errorf("the player's rogue is held by %v, want the [p-2] it was introduced with", ids)
	}
}

// VTT-246
func TestAPlayersOwnTokenlessCharacterIsHeardOf(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "familiar", Name: "Familiar",
			Kind:      vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			Resources: map[string]*vttv1.Resource{"pool": {Current: 2, Max: 2}}}},
		&vttv1.ActorControlGranted{ActorId: "familiar", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		&vttv1.ResourceChanged{ActorId: "familiar", Resource: "pool", Delta: -1, NewValue: 1},
		&vttv1.ConditionApplied{ActorId: "familiar", ConditionId: "tired"},
	))
	viewer := foldSteps(t, frames)
	for i := 12; i <= 13; i++ {
		if len(frames[i]) != 1 || frames[i][0].GetEventId() == "" {
			t.Errorf("step %d about the player's own token-less familiar was not forwarded: %v", i, frames[i])
		}
	}
	if got := viewer.Actors["familiar"].GetResources()["pool"].GetCurrent(); got != 1 {
		t.Errorf("the player's familiar holds pool %d, want 1", got)
	}
}

// correctionFrames returns the status frames of out about actorID, failing on
// any that carries something a correction must not.
func correctionFrames(t *testing.T, out []*vttv1.Envelope, actorID string) []*vttv1.Envelope {
	t.Helper()
	var fix []*vttv1.Envelope
	for _, e := range out {
		var about, extra string
		switch p := e.GetPayload().(type) {
		case *vttv1.Envelope_ResourceChanged:
			about, extra = p.ResourceChanged.GetActorId(), p.ResourceChanged.GetReason()
		case *vttv1.Envelope_ConditionApplied:
			about, extra = p.ConditionApplied.GetActorId(), p.ConditionApplied.GetSource()
		case *vttv1.Envelope_ConditionRemoved:
			about, extra = p.ConditionRemoved.GetActorId(), p.ConditionRemoved.GetReason()
		case *vttv1.Envelope_ActorControlGranted:
			about = p.ActorControlGranted.GetActorId()
		case *vttv1.Envelope_ActorControlRevoked:
			about = p.ActorControlRevoked.GetActorId()
		default:
			continue
		}
		if about != actorID {
			continue
		}
		if e.GetEventId() != "" || e.GetActorRole() != "" || e.GetParticipantId() != "" ||
			e.GetSessionId() != "" || e.GetOccurredAt() != nil || extra != "" {
			t.Errorf("a frame bringing %s up to date carries more than its status: %v", actorID, e)
		}
		fix = append(fix, e)
	}
	return fix
}

// VTT-248 VTT-249 VTT-247
func TestDamageTakenOutOfSightArrivesWhenTheEyeWalksIn(t *testing.T) {
	steps := testimonyLog(
		rogueLeaves,
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3, Reason: "ability:stab:hit"},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "bleeding", Source: "ability:stab:hit"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	)
	frames := runTestimony(player(), steps)
	viewer := foldSteps(t, frames)

	fix := correctionFrames(t, frames[13], "rogue")
	if len(fix) != 2 || fix[0].GetResourceChanged() == nil || fix[1].GetConditionApplied() == nil {
		t.Fatalf("walking in on the rogue sent %v, want its pool and its condition", fix)
	}
	if rc := fix[0].GetResourceChanged(); rc.GetDelta() != -2 || rc.GetNewValue() != 3 {
		t.Errorf("the rogue's pool arrived as delta %d new %d, want -2 and 3", rc.GetDelta(), rc.GetNewValue())
	}
	for _, e := range fix {
		if e.GetSequence() != 14 {
			t.Errorf("a correction carries sequence %d, want the door's 14", e.GetSequence())
		}
	}
	if got := viewer.Actors["rogue"].GetResources()["pool"].GetCurrent(); got != 3 {
		t.Errorf("the player's rogue holds pool %d after seeing it, want 3", got)
	}
	if c := viewer.Conditions["rogue"]; len(c) != 1 || c[0].ID != "bleeding" {
		t.Errorf("the player's rogue carries %v after seeing it, want [bleeding]", c)
	}
}

// VTT-248 VTT-247
func TestAKindChangedOutOfSightIsCorrectedOnSight(t *testing.T) {
	charm := &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "p-3",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}
	for _, tc := range []struct {
		name  string
		after []proto.Message
		want  []string
	}{
		{"with a controller", nil, []string{"p-3"}},
		{"with none", []proto.Message{&vttv1.ActorControlRevoked{ActorId: "goblin", ParticipantId: "p-3"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := []proto.Message{
				&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
				&vttv1.DoorClosed{SceneId: "s", At: testimonyDoor},
				charm,
			}
			log = append(log, tc.after...)
			log = append(log, &vttv1.DoorOpened{SceneId: "s", At: testimonyDoor})
			frames := runTestimony(player(), testimonyLog(log...))
			viewer := foldSteps(t, frames)
			for i := 12; i < len(frames)-1; i++ {
				if len(frames[i]) != 0 {
					t.Errorf("step %d, a control change on the unseen goblin, reached the player: %v", i, frames[i])
				}
			}
			g := viewer.Actors["goblin"]
			if g.GetKind() != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
				t.Errorf("the player's goblin is %v after seeing it, want a party member", g.GetKind())
			}
			if ids := g.GetControllerIds(); !sameStrings(ids, tc.want) {
				t.Errorf("the player's goblin is held by %v after seeing it, want %v", ids, tc.want)
			}
		})
	}
}

func sameStrings(a, b []string) bool {
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

// VTT-248
func TestAResourceStartedBelowZeroIsCorrectedOnSight(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wraith", Name: "Wraith",
			Kind:      vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Resources: map[string]*vttv1.Resource{"pool": {Current: math.MinInt32}}}},
		&vttv1.TokenPlaced{TokenId: "t-wraith", SceneId: "s", ActorId: "wraith",
			Position: &vttv1.GridPosition{X: 5, Y: 1}},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: 1, NewValue: 0},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: math.MaxInt32, NewValue: math.MaxInt32},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	viewer := foldSteps(t, frames)
	fix := correctionFrames(t, frames[14], "wraith")
	if len(fix) != 2 {
		t.Fatalf("seeing the wraith sent %v, want two frames through zero", fix)
	}
	first, second := fix[0].GetResourceChanged(), fix[1].GetResourceChanged()
	if first.GetDelta() != 0 || first.GetNewValue() != 0 ||
		second.GetDelta() != math.MaxInt32 || second.GetNewValue() != math.MaxInt32 {
		t.Errorf("the wraith's pool arrived as %v then %v, want 0/0 then %d/%d",
			first, second, math.MaxInt32, math.MaxInt32)
	}
	if got := viewer.Actors["wraith"].GetResources()["pool"].GetCurrent(); got != math.MaxInt32 {
		t.Errorf("the player's wraith holds pool %d, want %d", got, math.MaxInt32)
	}
}

// VTT-250
func TestAChangeUndoneOutOfSightSendsNothingOnSight(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		rogueLeaves,
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "marked"},
		&vttv1.ConditionRemoved{ActorId: "rogue", ConditionId: "marked"},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: 2, NewValue: 5},
		&vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-3",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		&vttv1.ActorControlRevoked{ActorId: "rogue", ParticipantId: "p-3"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	foldSteps(t, frames)
	if fix := correctionFrames(t, frames[17], "rogue"); len(fix) != 0 {
		t.Errorf("seeing the rogue, whose changes were all undone unseen, sent %v", fix)
	}
}

// VTT-250
func TestConditionsReorderedOutOfSightSendNothingOnSight(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "a"},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "b"},
		rogueLeaves,
		&vttv1.ConditionRemoved{ActorId: "rogue", ConditionId: "a"},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "a"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	foldSteps(t, frames)
	if fix := correctionFrames(t, frames[15], "rogue"); len(fix) != 0 {
		t.Errorf("seeing the rogue, whose conditions were only reordered unseen, sent %v", fix)
	}
}

// VTT-248
func TestADifferenceOfExactlyMaxInt32IsOneCorrection(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wraith", Name: "Wraith",
			Kind:      vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Resources: map[string]*vttv1.Resource{"pool": {Current: -1}}}},
		&vttv1.TokenPlaced{TokenId: "t-wraith", SceneId: "s", ActorId: "wraith",
			Position: &vttv1.GridPosition{X: 5, Y: 1}},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: 1, NewValue: 0},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: math.MaxInt32 - 1, NewValue: math.MaxInt32 - 1},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	foldSteps(t, frames)
	fix := correctionFrames(t, frames[14], "wraith")
	if len(fix) != 1 || fix[0].GetResourceChanged().GetDelta() != math.MaxInt32 {
		t.Errorf("a difference of exactly MaxInt32 arrived as %v, want one frame carrying it", fix)
	}
}

// VTT-251
func TestARemovalOutOfSightIsNotReported(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		rogueLeaves,
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.ActorRemoved{ActorId: "rogue"},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "familiar", Name: "Familiar",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "familiar", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		&vttv1.ActorRemoved{ActorId: "familiar"},
	))
	viewer := foldSteps(t, frames)
	if out := frames[12]; len(out) != 0 {
		t.Errorf("the removal of the rogue, unseen behind the door, reached the player: %v", out)
	}
	if _, ok := viewer.Actors["rogue"]; !ok {
		t.Error("the player's fold dropped the rogue it never saw removed")
	}
	if out := frames[15]; len(out) != 1 || out[0].GetActorRemoved().GetActorId() != "familiar" {
		t.Errorf("the removal of the player's own familiar, which it sees, did not reach it: %v", out)
	}
}

// VTT-252 VTT-313
func TestAnIdReusedAfterAnUnseenRemovalFoldsAgain(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		rogueLeaves,
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.ActorRemoved{ActorId: "rogue"},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "rogue", Name: "Second Rogue",
			Kind:      vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			Resources: map[string]*vttv1.Resource{"pool": {Current: 9, Max: 9}}}},
		&vttv1.TokenPlaced{TokenId: "t-rogue-2", SceneId: "s", ActorId: "rogue",
			Position: &vttv1.GridPosition{X: 2, Y: 1}},
	))
	viewer := foldSteps(t, frames)
	out := frames[14]
	if len(out) < 2 || out[0].GetActorRemoved().GetActorId() != "rogue" || out[0].GetEventId() != "" ||
		out[1].GetActorAdded().GetActor().GetActorId() != "rogue" {
		t.Fatalf("the reused rogue arrived as %v, want a bare ActorRemoved and then its introduction", out)
	}
	if a := viewer.Actors["rogue"]; a.GetName() != "Second Rogue" || a.GetResources()["pool"].GetCurrent() != 9 {
		t.Errorf("the player holds rogue as %v, want the second incarnation", a)
	}
}

// VTT-252
func TestARemovedActorWhoseIdNeverReturnsStaysInTheFold(t *testing.T) {
	frames := runTestimony(player(), testimonyLog(
		rogueLeaves,
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.ActorRemoved{ActorId: "rogue"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
		&vttv1.ConditionApplied{ActorId: "hero", ConditionId: "relieved"},
	))
	viewer := foldSteps(t, frames)
	for i, batch := range frames {
		for _, e := range batch {
			if e.GetActorRemoved().GetActorId() == "rogue" {
				t.Errorf("step %d sent the player a removal of the rogue, whose id never returned: %v", i, e)
			}
		}
	}
	if _, ok := viewer.Actors["rogue"]; !ok {
		t.Error("the player's fold dropped the rogue, whose removal it never saw")
	}
}

// forwardedOf is env as SPEC-016 says a player or a spectator is sent it.
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

// VTT-272 VTT-273 VTT-222 VTT-243
func TestAChangeByAnUnseenUserNamesNoAbilityAndNoIssuer(t *testing.T) {
	const cause = "ability:claw:hit"
	steps := testimonyLog(
		&vttv1.AbilityUsed{ActorId: "goblin", AbilityId: "claw", TargetIds: []string{"rogue"}},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3, Reason: cause},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "bleeding", Source: cause},
	)
	for _, v := range []gateway.Viewer{
		{ParticipantID: "p-2", Role: identity.RolePlayer},
		{ParticipantID: "sp-1", Role: identity.RoleSpectator, Viewpoint: "rogue"},
	} {
		st := engine.NewState()
		pr := gateway.NewProjector(v)
		for _, s := range steps {
			in := envelope(s.seq, s.payload)
			in.EventId, in.ActorRole, in.ParticipantId, in.SessionId = fmt.Sprintf("evt-%d", s.seq), "player", "p-3", "sess-1"
			before := proto.Clone(in)
			if err := engine.Apply(st, in); err != nil {
				t.Fatalf("seq %d: %v", s.seq, err)
			}
			out := pr.Project(in, st)
			if !proto.Equal(in, before) {
				t.Errorf("%s: projecting seq %d changed the event", v.Role, s.seq)
			}
			if s.seq < 11 {
				continue
			}
			var sent *vttv1.Envelope
			for _, e := range out {
				if e.GetAbilityUsed() != nil {
					t.Errorf("%s: sent the ability of a goblin it does not see", v.Role)
				}
				if e.GetEventId() != "" {
					sent = e
				}
			}
			if s.seq == 11 {
				continue
			}
			if sent == nil {
				t.Fatalf("%s: the change at seq %d on the rogue was not forwarded", v.Role, s.seq)
			}
			if sent.GetParticipantId() != "" || sent.GetActorRole() != "" ||
				sent.GetResourceChanged().GetReason() != "" || sent.GetConditionApplied().GetSource() != "" {
				t.Errorf("%s: seq %d names its issuer or its cause: %v", v.Role, s.seq, sent)
			}
			if !proto.Equal(sent, forwardedOf(in)) {
				t.Errorf("%s: forwarded %v, want the event less its issuer and its cause", v.Role, sent)
			}
		}
	}
}
