package gateway_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
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
	case *vttv1.EventsRetracted:
		env.Payload = &vttv1.Envelope_EventsRetracted{EventsRetracted: p}
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
	// The hero DECLARES NO KIND on purpose, so the whole suite below keeps
	// running the migration rule spec §5.1 wrote for logs recorded before the
	// field existed (absent + a controller means party member). The goblin
	// declares one, because a monster's kind is the fact this fixture needs to
	// survive somebody handing it to a participant.
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "goblin", Name: "Goblin",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}) // no controller: an NPC
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	return st
}

func player() gateway.Viewer {
	return gateway.Viewer{ParticipantID: "p-1", Role: identity.RolePlayer}
}

func TestAPlayerNeverReceivesATokenBehindAClosedDoor(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())

	// The goblin's own placement, replayed to this player.
	out := pr.Project(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob",
		SceneId: "s", ActorId: "goblin",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)

	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			t.Fatal("the goblin is behind a closed door and must not be on this player's wire")
		}
	}
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

func TestOpeningTheDoorIntroducesTheGoblinToThePlayer(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())
	pr.Project(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)

	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.Project(envelope(7, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	var sawActor, sawToken bool
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil && a.GetActor().GetActorId() == "goblin" {
			sawActor = true
		}
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			sawToken = true
			if e.GetSequence() != 7 {
				t.Errorf("a synthesized introduction carries the CAUSING sequence, got %d",
					e.GetSequence())
			}
		}
	}
	if !sawActor || !sawToken {
		t.Fatalf("opening the door must introduce the goblin (actor=%v token=%v)",
			sawActor, sawToken)
	}
}

func TestClosingTheDoorHidesTheGoblinAgain(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())
	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	pr.Project(envelope(7, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	mustApply(st, 8, &vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.Project(envelope(8, &vttv1.DoorClosed{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	var hidden bool
	for _, e := range out {
		if h := e.GetTokenHidden(); h != nil && h.GetTokenId() == "t-gob" {
			hidden = true
		}
	}
	if !hidden {
		t.Fatal("closing the door must hide the goblin from this player")
	}
}

func TestAPartyMemberStaysKnownEvenWhenOutOfSight(t *testing.T) {
	// Spec §5: party members are ALWAYS known — and §5.1 decides what counts
	// as one. "Rogue" here declares no kind and has a controller, so it is a
	// party member by the migration rule. You know your party
	// exists when the rogue is two rooms away; you merely cannot see their
	// token. Dropping them from your own roster because they turned a corner
	// reads as a bug, not as fog.
	st := twoRooms()
	mustApply(st, 7, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "rogue", Name: "Rogue", ControllerIds: []string{"p-2"}}})
	mustApply(st, 8, &vttv1.TokenPlaced{TokenId: "t-rogue", SceneId: "s",
		ActorId: "rogue", Position: &vttv1.GridPosition{X: 5, Y: 1}}) // behind the closed door

	pr := gateway.NewProjector(player())
	out := pr.Project(envelope(7, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "rogue", Name: "Rogue", ControllerIds: []string{"p-2"}}}), st)

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
	out = pr.Project(envelope(8, &vttv1.TokenPlaced{TokenId: "t-rogue",
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
	mustApply(st, 7, grant)

	pr := gateway.NewProjector(player())
	roster := actorIDsIn(pr.Project(envelope(7, grant), st))

	if !roster["hero"] {
		t.Fatal("fixture check: the player's own character must be on this roster, " +
			"or the absence asserted below proves nothing")
	}
	if roster["goblin"] {
		t.Error("a monster the DM happens to hold is still a monster (spec §5.1): " +
			"it must not reach a player's roster")
	}
}

func TestAPartyMemberIsKnownEvenWhenHeldByTheDM(t *testing.T) {
	// THE OTHER DIRECTION, and the reason kind belongs to the ACTOR rather
	// than to whoever holds it. A player's character run by the DM while its
	// player is offline is STILL a party member (spec §5.1), and the party must
	// still know they exist. A rule keyed on the CONTROLLER'S ROLE — the
	// obvious repair, and the wrong one — drops them from every roster.
	st := twoRooms()
	cleric := &vttv1.Actor{ActorId: "cleric", Name: "Cleric",
		Kind:          vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		ControllerIds: []string{"dm-1"}}
	mustApply(st, 7, &vttv1.ActorAdded{Actor: cleric})
	place := &vttv1.TokenPlaced{TokenId: "t-cleric", SceneId: "s",
		ActorId: "cleric", Position: &vttv1.GridPosition{X: 5, Y: 1}} // behind the shut door
	mustApply(st, 8, place)

	pr := gateway.NewProjector(player())
	roster := actorIDsIn(pr.Project(envelope(8, place), st))

	if !roster["cleric"] {
		t.Error("a party member whose player is away is still a party member: " +
			"the DM holding their character must not erase them from the roster")
	}
}

func TestAnActorFromBeforeTheKindFieldIsAPartyMemberWhenSomeoneControlsIt(t *testing.T) {
	// MIGRATION, first half (spec §5.1): absent + has a controller means party
	// member. Every ActorAdded already written lacks the field, so a plain
	// fail-closed default would retroactively drop existing party members from
	// every roster the moment they turned a corner — breaking §5 for every
	// campaign already recorded, which is precisely what a "safe" default
	// looks like from the inside.
	st := twoRooms()
	ghost := &vttv1.Actor{ActorId: "ghost", Name: "Ghost", ControllerIds: []string{"p-2"}}
	if ghost.GetKind() != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatalf("fixture check: this actor must declare NO kind, got %v", ghost.GetKind())
	}
	mustApply(st, 7, &vttv1.ActorAdded{Actor: ghost})
	place := &vttv1.TokenPlaced{TokenId: "t-ghost", SceneId: "s",
		ActorId: "ghost", Position: &vttv1.GridPosition{X: 5, Y: 1}} // behind the shut door
	mustApply(st, 8, place)

	pr := gateway.NewProjector(player())
	if roster := actorIDsIn(pr.Project(envelope(8, place), st)); !roster["ghost"] {
		t.Error("a log written before the kind field said party membership with a " +
			"controller, and that is what it still says (spec §5.1)")
	}
}

func TestAnActorFromBeforeTheKindFieldIsNotAPartyMemberWithNoController(t *testing.T) {
	// MIGRATION, second half. Absent + nobody controls it means NOT a party
	// member — the same reading "empty controller_ids" already had, kept
	// rather than reinterpreted. Without this half the migration rule would
	// read "absent means party member", and every monster in every recorded
	// campaign would be on every roster.
	st := twoRooms()
	rat := &vttv1.Actor{ActorId: "rat", Name: "Rat"}
	if ratKind := rat.GetKind(); ratKind != vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
		t.Fatalf("fixture check: this actor must declare NO kind, got %v", ratKind)
	}
	mustApply(st, 7, &vttv1.ActorAdded{Actor: rat})
	place := &vttv1.TokenPlaced{TokenId: "t-rat", SceneId: "s",
		ActorId: "rat", Position: &vttv1.GridPosition{X: 5, Y: 1}} // behind the shut door
	mustApply(st, 8, place)

	pr := gateway.NewProjector(player())
	roster := actorIDsIn(pr.Project(envelope(8, place), st))
	if !roster["hero"] {
		t.Fatal("fixture check: the player's own character must be on this roster, " +
			"or the absence asserted below proves nothing")
	}
	if roster["rat"] {
		t.Error("an uncontrolled actor with no declared kind is not a party member")
	}
}

func TestASpectatorGetsNoSightFromAnNPCTheDMControls(t *testing.T) {
	// The SAME defect at the perch, which is why one rule has to govern both
	// (spec §5.1, "one rule, both call sites"). §3.1.1 calls this the
	// constraint the whole idea rests on: a watcher perched on the Goblin
	// Archer sees the ambush from INSIDE it. Fixing only the roster would
	// leave that open to any DM who takes control of a monster.
	st := twoRooms()
	mustApply(st, 7, &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "dm-1"})

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

// --- everything below is this task's own, beyond the brief -------------------

// firstPlace runs the projector over the goblin's placement, which is the
// cheapest way to bring a fresh projector up to "this player has been shown
// their scene, their party and what they can see". Returns the envelopes so a
// caller can assert on the introduction batch itself.
func firstPlace(pr *gateway.Projector, st *engine.State) []*vttv1.Envelope {
	return pr.Project(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob",
		SceneId: "s", ActorId: "goblin",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)
}

func TestASeatWithNoActorIsToldOfNoSceneAtAll(t *testing.T) {
	// Spec §3.1 / exit criterion 4: a seat with no actor is not in a scene,
	// so it has no board — not its name, not its size. Onboarding starts here:
	// you log in with no character and the DM assigns one afterwards.
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{ParticipantID: "p-nobody", Role: identity.RolePlayer})

	for _, e := range firstPlace(pr, st) {
		if e.GetSceneCreated() != nil {
			t.Fatalf("a seat with no actor must learn nothing of any scene, got %v", e)
		}
	}
}

func TestAPlayerLearnsOnlyTheSceneTheirActorStandsIn(t *testing.T) {
	// Spec §4.2 / exit criterion 6: a scene a player has never entered is
	// absent from their stream ENTIRELY. Six loaded scenes must not hand a
	// player a table of contents for an adventure they have not played.
	st := twoRooms()
	mustApply(st, 7, &vttv1.SceneCreated{
		SceneId: "lair", Name: "The Dragon's Lair", GridWidth: 40, GridHeight: 40})

	pr := gateway.NewProjector(player())
	var scenes []string
	for _, e := range firstPlace(pr, st) {
		if sc := e.GetSceneCreated(); sc != nil {
			scenes = append(scenes, sc.GetSceneId())
		}
	}
	if len(scenes) != 1 || scenes[0] != "s" {
		t.Fatalf("a player must be able to enumerate exactly the scene they stand in, got %v", scenes)
	}
}

func TestAnIntroducedSceneCarriesTheOutlineButNoTerrain(t *testing.T) {
	// Spec §4.2: "of course there is a board, but you do not know what is in
	// the black area before you enter the black area." Grid dimensions are the
	// shape of the paper; tiles and objects are what you have to walk to.
	st := twoRooms()
	pr := gateway.NewProjector(player())

	var found *vttv1.SceneCreated
	for _, e := range firstPlace(pr, st) {
		if sc := e.GetSceneCreated(); sc != nil {
			found = sc
		}
	}
	if found == nil {
		t.Fatal("a player standing in a scene must be given its board")
	}
	if found.GetGridWidth() != 7 || found.GetGridHeight() != 3 {
		t.Errorf("the outline must be the real one, got %dx%d",
			found.GetGridWidth(), found.GetGridHeight())
	}
	if n := len(found.GetTiles()); n != 0 {
		t.Errorf("an introduced scene must carry NO tiles, got %d", n)
	}
	if n := len(found.GetObjects()); n != 0 {
		t.Errorf("an introduced scene must carry NO objects, got %d", n)
	}
}

func TestSceneSeenCarriesOnlyTheSquaresInSight(t *testing.T) {
	// Spec §5: SceneSeen carries the viewer's WHOLE CURRENT visible set, never
	// a delta — and never a square they cannot see. The far room's floor at
	// 5,1 is the assertion that matters; 1,1 is the control that proves the
	// message is populated at all.
	st := twoRooms()
	pr := gateway.NewProjector(player())

	var seen *vttv1.SceneSeen
	for _, e := range firstPlace(pr, st) {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
	}
	if seen == nil {
		t.Fatal("a player in a scene must be told what they can see of it")
	}
	if _, ok := seen.GetTiles()[key(1, 1)]; !ok {
		t.Errorf("the square the hero stands on must be in their visible set: %v", seen.GetTiles())
	}
	if _, ok := seen.GetTiles()[key(5, 1)]; ok {
		t.Error("the far room is behind a closed door and must not be in the visible set")
	}
}

// sceneSeenFor picks the SceneSeen for one scene out of a batch, or nil.
func sceneSeenIn(out []*vttv1.Envelope, sceneID string) *vttv1.SceneSeen {
	var found *vttv1.SceneSeen
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil && ss.GetSceneId() == sceneID {
			found = ss
		}
	}
	return found
}

func TestASceneThatLeavesSightEntirelyIsReportedDark(t *testing.T) {
	// THE HOLE TASK 7 CLOSES. transitions emits SceneSeen by walking the scenes
	// currently IN sight, so a scene that drops out of sight entirely has no
	// entry to walk and no envelope is emitted at all. The viewer's last
	// SceneSeen for it therefore stands forever — and the client reads the
	// newest SceneSeen as its CURRENT visible set (client/src/fold.ts's
	// sceneSeen arm), so that room stays lit on their board with nobody in it.
	//
	// The DM reassigning a character is how a seat loses every eye it had:
	// eyes() derives a player's actors from ControllerIds live off state, so a
	// revoke leaves them with none.
	st := twoRooms()
	pr := gateway.NewProjector(player())

	if lit := sceneSeenIn(firstPlace(pr, st), "s"); lit == nil || len(lit.GetTiles()) == 0 {
		t.Fatalf("the control fails: the hero's room must be reported LIT first, got %v", lit)
	}

	mustApply(st, 7, &vttv1.ActorControlRevoked{ActorId: "hero", ParticipantId: "p-1"})
	out := pr.Project(envelope(7, &vttv1.ActorControlRevoked{
		ActorId: "hero", ParticipantId: "p-1"}), st)

	dark := sceneSeenIn(out, "s")
	if dark == nil {
		t.Fatal("a scene this seat can no longer see anything of must be reported dark, " +
			"not left at whatever it last saw")
	}
	if n := len(dark.GetTiles()); n != 0 {
		t.Errorf("dark means the whole current visible set is EMPTY, got %d tiles", n)
	}
}

func TestASceneAlreadyReportedDarkIsNotReportedDarkAgain(t *testing.T) {
	// The other half of the same behaviour: emitting the empty set must not
	// become a per-event heartbeat for every scene the seat has ever seen. It
	// cannot, because the scene is dropped from the projector's record in the
	// same step — but "cannot" is the kind of claim this arc has been wrong
	// about in prose, so it is asserted instead.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	mustApply(st, 7, &vttv1.ActorControlRevoked{ActorId: "hero", ParticipantId: "p-1"})
	pr.Project(envelope(7, &vttv1.ActorControlRevoked{
		ActorId: "hero", ParticipantId: "p-1"}), st)

	again := pr.Project(envelope(8, &vttv1.NarrationAdded{Text: "the room falls quiet"}), st)
	if ss := sceneSeenIn(again, "s"); ss != nil {
		t.Errorf("a scene already reported dark must stay silent, got %v", ss)
	}
}

func TestASceneRetractedOutOfTheWorldIsForgottenSILENTLY(t *testing.T) {
	// The union walk above reaches a scene the projector REMEMBERS, and memory
	// can outlive the world. engine.State.Scenes is a VALUE map, so a missing
	// key reads as the zero Scene with an empty ID — and a SceneSeen naming ""
	// is the worst envelope this file can send. Both folds reject it ("scene
	// seen for unknown scene"), and client/src/session.ts re-folds its entire
	// accumulated log on every event, so the throw would recur forever and
	// freeze that viewer for the rest of the session.
	//
	// It is REACHABLE, not hypothetical. seat.receive rebuilds the world with
	// campaign.FoldPrefix, which skips retracted ranges, so an undo covering a
	// SceneCreated (together with what depends on it) removes that scene from
	// the state while pr.seen still holds its id. Modelled here by projecting
	// against a world that no longer has the scene, which is exactly what that
	// re-fold hands this function.
	//
	// SILENTLY, and that is the second half. Naming the scene correctly would
	// not help: the retraction reached this viewer too, so their own fold has
	// no such scene either and a well-formed dark SceneSeen would throw just
	// the same. There is nothing to report about a scene that no longer exists.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	if lit := sceneSeenIn(firstPlace(pr, st), "s"); lit == nil {
		t.Fatal("the control fails: the projector must remember this room first")
	}

	gone := engine.NewState()
	mustApply(gone, 1, &vttv1.SessionStarted{Name: "n"})
	out := pr.Project(envelope(8, &vttv1.NarrationAdded{Text: "the scene is undone"}), gone)

	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil {
			t.Errorf("a scene the world no longer has must produce no SceneSeen at all, got %v", ss)
		}
	}

	// And it is forgotten, so nothing re-fires it once the world moves on.
	again := pr.Project(envelope(9, &vttv1.NarrationAdded{Text: "quiet"}), gone)
	for _, e := range again {
		if ss := e.GetSceneSeen(); ss != nil {
			t.Errorf("the scene was already forgotten and must stay silent, got %v", ss)
		}
	}
}

// twoRoomsMissing is twoRooms with ONE square's terrain removed — a legal
// scene, since tiles are optional per square (Patrik's ruling 2026-08-13) and
// nothing in the fold requires a grid to be fully declared.
func twoRoomsMissing(sq string) *engine.State {
	tiles := twoRoomsTiles()
	delete(tiles, sq)
	st := engine.NewState()
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: tiles})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	return st
}

// TestAVisibleSquareWithNoTerrainIsAbsentFromTheVisibleSet PINS AN INFERENCE
// THE WIRE FORCES, so that changing it is a decision rather than an accident.
//
// SceneSeen has one place to put squares — `tiles` — so the only current-sight
// signal it can carry is "here is the terrain you can see". sceneSeenFor
// therefore skips a visible square that declares no Tile: there is nothing to
// send for it. Since Task 7 the client derives Scene.Visible from exactly those
// keys, so "visible" on the client means "visible AND declares terrain".
//
// FOR EXPLORED THAT INFERENCE WAS HARMLESS. Explored has carried it since Task
// 3, and a square with no Tile produces no DrawOp either (scene-plan.ts's
// planTiles skips it), so not remembering it changed nothing anyone could see.
// Task 7 is where it starts governing what is DRAWN, and the consequence is no
// longer "a square goes unshaded" but "a creature standing there is not drawn"
// — see the client half of this pin in client/test/visibility.test.ts.
//
// A WHOLLY tile-less scene fails LOUDLY (nothing renders, so nobody ships it).
// A single missing square fails SILENTLY, which is why the hole is the case
// worth pinning: MEASURED here, the complete room reports 8 squares and the
// holed one reports 7, with the hero's own square the one that vanishes.
//
// NOT AN ASSERTION THAT THIS IS RIGHT. It is what the wire does today, awaiting
// Patrik's ruling; whichever way that goes, this test is what makes the change
// deliberate. Neither shipped adventure can reach it — cellar-rats declares 36
// tiles for 6x6 and goblin-ambush 1024 for 32x32 — so nothing in the product
// depends on the answer yet.
func TestAVisibleSquareWithNoTerrainIsAbsentFromTheVisibleSet(t *testing.T) {
	full := sceneSeenIn(firstPlace(gateway.NewProjector(player()), twoRooms()), "s")
	if n := len(full.GetTiles()); n != 8 {
		t.Fatalf("the control fails: a complete room must report 8 visible squares, got %d", n)
	}
	if full.GetTiles()[key(1, 1)] == nil {
		t.Fatal("the control fails: the hero's own square must be in the complete room's set")
	}

	holed := sceneSeenIn(firstPlace(gateway.NewProjector(player()), twoRoomsMissing(key(1, 1))), "s")
	if n := len(holed.GetTiles()); n != 7 {
		t.Errorf("removing one square's terrain must remove exactly it, got %d squares", n)
	}
	if holed.GetTiles()[key(1, 1)] != nil {
		t.Error("a square with no Tile has nothing to send, so it cannot be in the set")
	}
	// The rest of the room is untouched, which is what makes this SILENT: the
	// board still renders, and only the one square is missing from it.
	if holed.GetTiles()[key(2, 1)] == nil {
		t.Error("every square that DOES declare terrain must still be reported")
	}
}

// TestASceneThatComesBackIsUsableAgain pins what forgetting a vanished scene
// actually has to mean, which is more than dropping its visible set.
//
// A scene id is CALLER-SUPPLIED (CreateScene.scene_id, passed straight through
// convert.go), so re-creating one under the same id after an undo removed it is
// ordinary, not exotic: the "scene %q already exists" check has nothing left to
// collide with. When that happens the viewer's own fold has dropped the scene
// too — EventsRetracted is forwarded to players — so everything the projection
// then says about it lands on a scene they do not have. Both folds answer that
// with a hard error, and client/src/session.ts re-folds its whole log on every
// event, so the throw recurs forever.
//
// THE ASSERTION IS THAT THE BATCH FOLDS, not that some particular envelope
// appears. That is the only thing the viewer cares about and the only thing
// that cannot be satisfied by a stub: measured before the fix, the returning
// scene produced a TokenPlaced and a SceneSeen and BOTH were unfoldable —
// "token placed in unknown scene" and "scene seen for unknown scene" — because
// pr.scenes still marked the scene introduced, so no introduction was re-sent.
func TestASceneThatComesBackIsUsableAgain(t *testing.T) {
	pr := gateway.NewProjector(player())
	if lit := sceneSeenIn(firstPlace(pr, twoRooms()), "s"); lit == nil {
		t.Fatal("the control fails: the projector must remember this room first")
	}

	// The world after an undo that took the scene and the tokens standing in it
	// but left the actors — the shape a retraction has when the actors were
	// created before the scene, which is the ordinary order. Isolating it that
	// way keeps this test about the SCENE: pr.actors deliberately never forgets
	// (see the Projector doc comment), so an undo reaching the actors as well is
	// a separate gap one layer up, and not what this pins.
	gone := engine.NewState()
	mustApply(gone, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(gone, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(gone, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	pr.Project(envelope(8, &vttv1.NarrationAdded{Text: "the scene is undone"}), gone)

	// The same room again, which is what a re-created scene folds to.
	out := pr.Project(envelope(9, &vttv1.NarrationAdded{Text: "and rebuilt"}), twoRooms())

	// A viewer who dropped the scene when they folded the retraction, and who
	// still holds the actors it did not reach.
	viewer := gone.Snapshot()
	for _, e := range out {
		if err := engine.Apply(viewer, e); err != nil {
			t.Fatalf("a returning scene must arrive foldable, got %v", err)
		}
	}
	if _, ok := viewer.Scenes["s"]; !ok {
		t.Fatal("the viewer must end up holding the scene they are standing in again")
	}
	if n := len(viewer.Scenes["s"].Visible); n != 8 {
		t.Errorf("and told the whole of what they can see of it, got %d squares", n)
	}
}

// TestASceneThatComesBackBringsItsOpenDoorsBack pins the `doors` half of the
// forgetting loop, which the foldability test above cannot: dropping only
// `delete(pr.doors, id)` leaves every other test in this package green.
//
// It is the same defect TestAnIntroducedSceneArrivesWithItsDoorsAlreadyOpen
// guards for a first introduction, reached the other way round. A door's open
// state travels in neither the redacted SceneCreated nor SceneSeen, so
// doorTransitions is the only thing that can correct it — and it only emits
// when the world disagrees with what the viewer is BELIEVED to think. Leave
// that belief behind after the scene vanishes and a returning scene whose door
// is still open matches it exactly, so nothing is emitted and the viewer is
// re-introduced to a room with a door their board draws shut. It never
// self-corrects, and the re-introduction is what guarantees that rather than
// merely failing to help: the redacted SceneCreated rebuilds OpenDoors EMPTY in
// both folds (apply.go's and fold.ts's SceneCreated arms), and after that only
// the two door arms ever write it — so a DoorOpened that is never sent is a
// door that stays shut for the rest of the session.
func TestASceneThatComesBackBringsItsOpenDoorsBack(t *testing.T) {
	open := func() *engine.State {
		st := twoRooms()
		mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
		return st
	}
	pr := gateway.NewProjector(player())
	var believed bool
	for _, e := range firstPlace(pr, open()) {
		if e.GetDoorOpened() != nil {
			believed = true
		}
	}
	if !believed {
		t.Fatal("the control fails: the viewer must be told the door is open the first time")
	}

	gone := engine.NewState()
	mustApply(gone, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(gone, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(gone, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	pr.Project(envelope(8, &vttv1.NarrationAdded{Text: "the scene is undone"}), gone)

	var again bool
	for _, e := range pr.Project(envelope(9, &vttv1.NarrationAdded{Text: "and rebuilt"}), open()) {
		if d := e.GetDoorOpened(); d != nil && d.GetSceneId() == "s" {
			again = true
		}
	}
	if !again {
		t.Error("a scene the viewer is being introduced to again must arrive with its " +
			"doors open, not left shut by a belief that outlived the scene")
	}
}

// bareCanvas is a scene that declares NO terrain: 3x3, two tokens, nothing
// underfoot. Legal, and not degenerate — mapdef.CheckEverySquarePresent is
// all-or-nothing (zero tiles passes; one tile means all must be present) and it
// guards both the map-file path and the CreateScene command path, so this is
// the shape a caller gets by simply not sending tiles. A token is a FREE OBJECT
// that needs no terrain to stand on (Patrik's ruling 2026-08-22).
func bareCanvas() *engine.State {
	st := engine.NewState()
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 2, Y: 2}})
	return st
}

// TestSceneSeenCarriesTheVisibleSquaresEvenWithNoTerrain is the defect this
// round exists to close, at the point where it starts.
//
// The server already decides correctly: sight.VisibleFrom walks the GRID, not
// the tile map, so a scene with no terrain has no blockers and every in-range
// square is visible; look() marks the tokens on those squares visible without
// terrain entering it at all; and a TokenPlaced duly goes out. Then sceneSeenFor
// projected the square set through Tiles and DROPPED every square with no tile,
// so the set was destroyed on the wire and the client re-derived a different,
// smaller one — overruling a decision the server had already made. Measured
// before the fix: both tokens sent, SceneSeen carrying 0 tiles and 0 objects.
//
// The visible set now travels as itself.
func TestSceneSeenCarriesTheVisibleSquaresEvenWithNoTerrain(t *testing.T) {
	st := bareCanvas()
	pr := gateway.NewProjector(player())
	out := pr.Project(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 2, Y: 2}}), st)

	var sent []string
	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil {
			sent = append(sent, tp.GetTokenId())
		}
	}
	sort.Strings(sent)
	if len(sent) != 2 {
		t.Fatalf("the control fails: the server must send both tokens, got %v", sent)
	}

	seen := sceneSeenIn(out, "s")
	if seen == nil {
		t.Fatal("a player standing in a scene must be told what they can see of it")
	}
	if n := len(seen.GetTiles()); n != 0 {
		t.Errorf("there is no terrain to describe, got %d tiles", n)
	}
	// All nine squares: no terrain means no blockers, and range is unlimited.
	if n := len(seen.GetVisible()); n != 9 {
		t.Fatalf("every square of a bare 3x3 is visible, got %d: %v", n, seen.GetVisible())
	}
	for _, sq := range []string{key(1, 1), key(2, 2)} {
		var found bool
		for _, v := range seen.GetVisible() {
			if v == sq {
				found = true
			}
		}
		if !found {
			t.Errorf("square %s carries a token the server just sent and must be visible", sq)
		}
	}
}

// TestTheVisibleSetIsSentInAStableOrder guards the one field sceneSeenFor builds
// FROM A MAP into an ordered wire field, which is what makes it the only one Go's
// randomised iteration can reach. Not "every emitting loop is sorted" — that was
// this test's opening sentence until 2026-08-22 and it is false: sceneSeenFor
// also emits `repeated SceneObject objects = 3`, ordered on the wire, by walking
// the sc.Objects SLICE, which already has an order and needs no sorter.
// TestTheSameLogProjectsTheSameStreamEveryTime would catch an unsorted walk
// eventually and by coin flip; this catches it directly.
func TestTheVisibleSetIsSentInAStableOrder(t *testing.T) {
	for i := 0; i < 20; i++ {
		seen := sceneSeenIn(pr20(), "s")
		got := seen.GetVisible()
		// THE CONTROL, and it is not decoration: sort.StringsAreSorted(nil) is
		// TRUE, and sceneSeenIn returns nil for a batch with no SceneSeen at
		// all, so without this the test PASSES against a sceneSeenFor that
		// emits no visible set whatsoever — measured, it reported PASS with the
		// field deleted. An ordering test that cannot tell "sorted" from
		// "absent" is checking nothing.
		if len(got) != 9 {
			t.Fatalf("the control fails: a bare 3x3 has nine visible squares, got %v", got)
		}
		if !sort.StringsAreSorted(got) {
			t.Fatalf("the visible set must be emitted sorted, got %v", got)
		}
	}
}

// pr20 is one fresh projection of bareCanvas, the shape the ordering test
// repeats. Split out so the loop above reads as twenty independent draws
// against Go's randomised map iteration rather than twenty uses of one fixture.
func pr20() []*vttv1.Envelope {
	st := bareCanvas()
	return gateway.NewProjector(player()).Project(
		envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
			ActorId: "goblin", Position: &vttv1.GridPosition{X: 2, Y: 2}}), st)
}

// TestVisibleAndExploredComeFromDifferentSourcesAndMayDiffer pins the pair a
// future reader will assume matches. They no longer share a source: Explored is
// TERRAIN remembered (unioned from tiles, and a square with no terrain has
// nothing to remember, so there is nothing to fog there), while Visible is the
// server's own sight answer and owes nothing to terrain. On a bare canvas they
// are maximally apart — everything visible, nothing explored.
func TestVisibleAndExploredComeFromDifferentSourcesAndMayDiffer(t *testing.T) {
	seen := sceneSeenIn(pr20(), "s")

	viewer := engine.NewState()
	mustApply(viewer, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(viewer, 2, &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})
	must3(t, engine.Apply(viewer, envelope(3, seen)))

	sc := viewer.Scenes["s"]
	if len(sc.Visible) != 9 {
		t.Errorf("every square is visible on a bare canvas, got %d", len(sc.Visible))
	}
	if len(sc.Explored) != 0 {
		t.Errorf("there is no terrain to remember, so nothing is explored, got %v", sc.Explored)
	}
}

func must3(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("fold: %v", err)
	}
}

func TestASpectatorRidesTheShoulderTheyPerchOn(t *testing.T) {
	// Spec §3.1.1: "like a bird hopping from one shoulder to another". A
	// spectator perched on the hero sees the hero's room and, once the door
	// opens, the goblin.
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{
		ParticipantID: "sp-1", Role: identity.RoleSpectator, Viewpoint: "hero"})
	firstPlace(pr, st)

	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	var sawGoblin bool
	for _, e := range pr.Project(envelope(7, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st) {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			sawGoblin = true
		}
	}
	if !sawGoblin {
		t.Fatal("a spectator must see what the shoulder they ride sees")
	}
}

func TestAPerchOnAnNpcYieldsNoSightAtAll(t *testing.T) {
	// THE CONSTRAINT THE WHOLE IDEA RESTS ON (spec §3.1.1): a spectator
	// perched on the Goblin Archer would watch the ambush from inside it.
	// Task 6 refuses the perch at the command; this is the projection
	// refusing to honour one that got past it anyway — defence in depth for
	// security-critical code (spec §8).
	//
	// "No sight" is scene, terrain and tokens — NOT the party roster, which a
	// spectator needs in order to have a shoulder to choose (spec §5: party
	// members are always known, whoever holds them — §5.1).
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{
		ParticipantID: "sp-1", Role: identity.RoleSpectator, Viewpoint: "goblin"})

	for _, e := range firstPlace(pr, st) {
		switch {
		case e.GetSceneCreated() != nil, e.GetSceneSeen() != nil, e.GetTokenPlaced() != nil:
			t.Fatalf("a perch on an NPC must yield no sight at all, got %v", e)
		}
	}
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

func TestProjectingChangesNeitherTheEventNorTheState(t *testing.T) {
	// CLAUDE.md rule 4: engine.Apply is the only writer, so the projection
	// READS state. And the envelope it is handed is SHARED — the pump hands
	// the same pointer to every connection — so redacting in place would edit
	// the DM's copy of history from inside a player's projection.
	// SceneCreated leads the table deliberately: it is the payload the
	// projection REDACTS, so it is the one an implementation is most tempted
	// to edit where it stands instead of building a fresh one.
	ins := []*vttv1.Envelope{
		envelope(2, &vttv1.SceneCreated{SceneId: "s", Name: "S",
			GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()}),
		envelope(4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "goblin", Name: "Goblin"}}),
		envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
			ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}}),
	}
	for _, in := range ins {
		st := twoRooms()
		pr := gateway.NewProjector(player())
		inBefore := proto.Clone(in)
		stBefore, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}

		pr.Project(in, st)

		if !proto.Equal(in, inBefore) {
			t.Errorf("the projection must not touch the envelope it was handed:\n got %v\nwant %v",
				in, inBefore)
		}
		stAfter, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		if string(stAfter) != string(stBefore) {
			t.Errorf("the projection must not touch state:\n got %s\nwant %s", stAfter, stBefore)
		}
	}
}

// step is one event of a hand-built log: the sequence it takes and the
// payload it carries.
type step struct {
	seq     int64
	payload proto.Message
}

// openFloor is a w x h room of plain floor with no walls at all.
func openFloor(w, h int32) map[string]*vttv1.TileRef {
	tiles := map[string]*vttv1.TileRef{}
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			tiles[key(x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	return tiles
}

// aWholeFight is a log with every transition this projection has to get right:
// a room, a party member, TWO NPCs behind a closed door, the door opening, one
// of them moving in view, the door closing again, a line of narration — and
// then a second character handed to the player who already has tokens standing
// in two further scenes.
//
// TWO of everything, deliberately. Each of the ordered loops in transitions is
// only observably ordered when it emits more than one envelope for a single
// event, so a fixture that reveals one goblin at a time would let the sorting
// be deleted with nothing failing. Opening the door introduces two actors and
// two tokens at once, closing it hides two at once, and the grant at the end
// puts the player into two scenes at once.
func aWholeFight() []step {
	return []step{
		{1, &vttv1.SessionStarted{Name: "n"}},
		{2, &vttv1.SceneCreated{SceneId: "s", Name: "S",
			GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()}},
		{3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}}},
		{4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}}},
		{5, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "ogre", Name: "Ogre"}}},
		{6, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{7, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin",
			Position: &vttv1.GridPosition{X: 5, Y: 1}}},
		{8, &vttv1.TokenPlaced{TokenId: "t-ogre", SceneId: "s", ActorId: "ogre",
			Position: &vttv1.GridPosition{X: 4, Y: 1}}},
		{9, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}},
		{10, &vttv1.TokenMoved{TokenId: "t-ogre", SceneId: "s",
			From: &vttv1.GridPosition{X: 4, Y: 1}, To: &vttv1.GridPosition{X: 5, Y: 1}}},
		{11, &vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}},
		{12, &vttv1.NarrationAdded{Text: "The door slams.", As: "DM"}},

		// A scout with a foot in two rooms. Odd at a table and legal on the
		// wire — nothing binds an actor to one scene — and it is the only
		// shape that puts a viewer into two scenes on ONE event, which is
		// what makes the scene loop's ordering observable.
		{13, &vttv1.SceneCreated{SceneId: "vault", Name: "Vault",
			GridWidth: 3, GridHeight: 3, Tiles: openFloor(3, 3)}},
		{14, &vttv1.SceneCreated{SceneId: "crypt", Name: "Crypt",
			GridWidth: 3, GridHeight: 3, Tiles: openFloor(3, 3)}},
		{15, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout"}}},
		{16, &vttv1.TokenPlaced{TokenId: "t-sv", SceneId: "vault", ActorId: "scout",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{17, &vttv1.TokenPlaced{TokenId: "t-sc", SceneId: "crypt", ActorId: "scout",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{18, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-1"}},

		// A SECOND controller for an actor already introduced, and the log's
		// only event that mutates an Actor the projection has already put on
		// the wire: engine.Apply appends to ControllerIds IN PLACE, on the
		// pointer st.Actors holds. It is here so that
		// TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed can see whether
		// the ActorAdded emitted at step 18 was cloned or merely aliased —
		// without it, project.go's claim that it must be cloned is prose
		// nothing runs.
		{19, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-2"}},

		// A condition applied to an actor this player cannot see, and removed
		// once they can. Withheld, carried by the introduction, then removed —
		// and if the middle step is missing the LAST one is a hard fold error
		// rather than a wrong pixel. See
		// TestAConditionAppliedOutOfSightArrivesWithTheActor.
		{20, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "ghoul", Name: "Ghoul"}}},
		{21, &vttv1.ConditionApplied{ActorId: "ghoul", ConditionId: "marked", Source: "the DM"}},
		{22, &vttv1.ActorControlGranted{ActorId: "ghoul", ParticipantId: "p-1"}},
		{23, &vttv1.ConditionRemoved{ActorId: "ghoul", ConditionId: "marked"}},
	}
}

func TestAConditionAppliedOutOfSightArrivesWithTheActor(t *testing.T) {
	// THE FAILURE SPEC §8 CALLS THE WORST AVAILABLE, one layer over from the
	// example it uses. Both folds reject removing a condition that is not
	// there — engine.Apply: "condition %q not present on actor %q";
	// client/src/fold.ts: the same sentence — and Task 3 measured what a throw
	// costs a client: Session re-folds its whole log on every event, so one
	// throw freezes that viewer's state permanently.
	//
	// The projection can walk straight into it. A condition applied to an
	// actor the viewer cannot see is withheld; the actor then becomes known;
	// and the REMOVAL is forwarded, because knowing an actor is all that arm
	// asks. The viewer never received the application. So an introduction has
	// to carry the actor's conditions — unlike its resources, which are fields
	// of the Actor and ride along in the clone for free.
	st := twoRooms()
	mustApply(st, 7, &vttv1.ConditionApplied{
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
	intro := pr.Project(envelope(7, &vttv1.ConditionApplied{
		ActorId: "goblin", ConditionId: "marked", Source: "the DM"}), st)
	for _, e := range intro {
		if e.GetConditionApplied() != nil {
			t.Fatal("a condition on an unseen actor names it, and must be withheld")
		}
	}

	// The door opens, the goblin is introduced — and its condition with it.
	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.Project(envelope(8, &vttv1.DoorOpened{
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
	if got.GetConditionId() != "marked" || got.GetSource() != "the DM" {
		t.Errorf("the condition must arrive whole, got id %q source %q",
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
	mustApply(st, 9, &vttv1.ConditionRemoved{ActorId: "goblin", ConditionId: "marked"})
	for _, e := range pr.Project(envelope(9, &vttv1.ConditionRemoved{
		ActorId: "goblin", ConditionId: "marked"}), st) {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("the removal does not fold, which is a permanent client freeze: %v", err)
		}
	}
	if n := len(viewerState.Conditions["goblin"]); n != 0 {
		t.Errorf("the condition was removed and must be gone, got %d", n)
	}
}

// runLogSteps folds each step and projects it, exactly as the pump will: one
// envelope, folded and then handed to the projection. It returns ONE BATCH PER
// STEP rather than a flat stream, because the catch-up test below has to cut
// the stream at a step boundary and a flat slice has forgotten where those are.
func runLogSteps(v gateway.Viewer, steps []step, isolate bool) [][]*vttv1.Envelope {
	st := engine.NewState()
	pr := gateway.NewProjector(v)
	out := make([][]*vttv1.Envelope, len(steps))
	for i, s := range steps {
		env := envelope(s.seq, s.payload)
		if err := engine.Apply(st, env); err != nil {
			panic(fmt.Sprintf("runLog seq %d: %v", s.seq, err))
		}
		// isolate hands the projection a state nothing will ever mutate
		// again, which is what makes an ALIAS into live state observable.
		// The pump does not do this and must not have to.
		view := st
		if isolate {
			view = st.Snapshot()
		}
		out[i] = pr.Project(env, view)
	}
	return out
}

func runLog(v gateway.Viewer, steps []step) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, batch := range runLogSteps(v, steps, false) {
		out = append(out, batch...)
	}
	return out
}

func TestTheSameLogProjectsTheSameStreamEveryTime(t *testing.T) {
	// Spec §4.1 — THE property everything rests on: the projection is a pure
	// function of (log-so-far, viewer). Live streaming and reconnect catch-up
	// only agree because of it, so a player who drops mid-fight cannot be
	// shown what the live stream had already hidden.
	//
	// The bug this catches is not hypothetical. The projection diffs and
	// enumerates SETS — visible tokens, known actors, seen squares — and Go
	// randomises map iteration, so an unordered emission would make two runs
	// of the same log disagree about ORDER while agreeing about content, and
	// the byte-for-byte parity Task 8 rests on would be a coin flip.
	//
	// RUN MANY TIMES, because two is not enough and that is arithmetic rather
	// than caution: randomised iteration over a two-element set puts two runs
	// in the same order half the time. Measured, by deleting the sorting from
	// each of the projection's ordered loops in turn and running this test 50
	// times: the two-run version of it passed 22% of the time for one loop and
	// 56% for the other. Under the mutation gate, which runs the suite once per
	// mutant, that is a survivor. Twenty runs puts the false-pass probability
	// of a single two-element loop under 2e-6 and costs milliseconds; both
	// deletions now fail 50 times out of 50.
	const runs = 20
	first := runLog(player(), aWholeFight())
	for r := 1; r < runs; r++ {
		again := runLog(player(), aWholeFight())
		if len(first) != len(again) {
			t.Fatalf("the same log must project the same stream: %d envelopes then %d",
				len(first), len(again))
		}
		for i := range first {
			if !proto.Equal(first[i], again[i]) {
				t.Fatalf("run %d, envelope %d differs from the first run of the same log:\n %v\n %v",
					r, i, first[i], again[i])
			}
		}
	}

	// Non-vacuity: two empty streams would satisfy every assertion above. The
	// fight has to have actually been projected, hides included.
	var placed, hidden int
	for _, e := range first {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			placed++
		}
		if h := e.GetTokenHidden(); h != nil && h.GetTokenId() == "t-gob" {
			hidden++
		}
	}
	if placed != 1 || hidden != 1 {
		t.Fatalf("the goblin must arrive once and leave once, got %d arrivals and %d departures",
			placed, hidden)
	}
}

func TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed(t *testing.T) {
	// Spec §4.1's property stated as the thing it protects, and the half
	// TestTheSameLogProjectsTheSameStreamEveryTime does NOT establish.
	//
	// What live streaming and reconnect catch-up need is that the stream
	// SPLITS: a seat that folded events 1..cut live, dropped, and was then
	// handed a from-scratch projection of cut..end must end up in exactly the
	// state it would have reached had it never dropped. The first loop below
	// cuts at EVERY step of the fight and folds both halves through the very
	// fold the client mirrors.
	//
	// HOW MUCH THAT FIRST LOOP ACTUALLY PROVES, said plainly because an earlier
	// version of this comment overclaimed it. Its catch-up half is built the way
	// the contract REQUIRES — a fresh projector fed the log from the beginning,
	// with everything at or below the resume point discarded — and
	// TestTheSameLogProjectsTheSameStreamEveryTime already pins that this is a
	// deterministic function of (viewer, log). The catch-up stream is therefore
	// EQUAL to the live one, and the splice lands by construction. The loop
	// documents the contract and re-checks that the spliced stream folds; it
	// cannot fail on its own. Measured rather than argued: replacing the
	// catch-up projector with the live stream itself leaves the test passing.
	//
	// THE SECOND LOOP IS THE ONE WITH TEETH, and it is what makes Task 5's
	// seeding contract executable rather than a sentence in a report. It builds
	// the catch-up half the way an implementation is actually tempted to —
	// NewProjector against a subscription that starts at the resume point — and
	// requires that this BREAKS the seat, which is the claim Projector's doc
	// comment makes about why a projector may not be seeded that way.
	//
	// AND the catch-up run is handed an ISOLATED state per event, which pins a
	// second claim project.go makes in prose and nothing else runs: that an
	// introduction must CLONE the actor rather than hand out the pointer
	// st.Actors holds. Both streams are compared after both runs have
	// finished, so an aliased envelope has had time to be mutated behind the
	// wire's back — step 19's second grant is in the fixture for exactly that.
	steps := aWholeFight()
	live := runLogSteps(player(), steps, false)
	isolated := runLogSteps(player(), steps, true)

	for i := range steps {
		if len(live[i]) != len(isolated[i]) {
			t.Fatalf("step %d: the same log projected %d envelopes against live state and %d against an isolated copy",
				i, len(live[i]), len(isolated[i]))
		}
		for j := range live[i] {
			if !proto.Equal(live[i][j], isolated[i][j]) {
				t.Fatalf("step %d, envelope %d: the projection retained a reference into the state it was handed —\n live     %v\n isolated %v",
					i, j, live[i][j], isolated[i][j])
			}
		}
	}

	foldAll := func(t *testing.T, st *engine.State, batches [][]*vttv1.Envelope, from, to int, what string) {
		t.Helper()
		for i := from; i < to; i++ {
			for _, e := range batches[i] {
				if err := engine.Apply(st, e); err != nil {
					t.Fatalf("%s: step %d (%T) does not fold: %v", what, i, e.GetPayload(), err)
				}
			}
		}
	}

	// The seat that never dropped.
	want := engine.NewState()
	foldAll(t, want, live, 0, len(steps), "the live stream")
	if _, ok := want.Tokens["t-hero"]; !ok {
		t.Fatal("the live stream never delivered the player their own character, so nothing below is proved")
	}

	for cut := 1; cut < len(steps); cut++ {
		// The reconnecting seat: a NEW projector, fed the log from the
		// beginning, with everything at or below its resume point discarded.
		resumed := runLogSteps(player(), steps, false)

		got := engine.NewState()
		foldAll(t, got, live, 0, cut, fmt.Sprintf("cut %d: what the seat folded before it dropped", cut))
		foldAll(t, got, resumed, cut, len(steps), fmt.Sprintf("cut %d: the catch-up", cut))

		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		if string(gotJSON) != string(wantJSON) {
			t.Fatalf("dropping after step %d and catching up must land the seat where it would have been:\n got %s\nwant %s",
				cut, gotJSON, wantJSON)
		}
	}

	// THE CONSTRUCTION THAT MUST NOT BE USED. A projector created AT the resume
	// point has empty maps, so it re-introduces the scene, the actors and the
	// tokens the seat already holds — and a duplicate introduction is a hard
	// fold error, which on the client is the permanent state freeze Task 3
	// measured. Required to break, at least somewhere, because if it did not
	// then feeding the log from the beginning would be an optimisation to argue
	// about rather than the contract it is.
	foldErr := func(st *engine.State, batches [][]*vttv1.Envelope, from, to int) error {
		for i := from; i < to; i++ {
			for _, e := range batches[i] {
				if err := engine.Apply(st, e); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var broke int
	for cut := 1; cut < len(steps); cut++ {
		naive := runLogStepsResumingAt(player(), steps, cut)

		got := engine.NewState()
		err := foldErr(got, live, 0, cut)
		if err == nil {
			err = foldErr(got, naive, cut, len(steps))
		}
		if err != nil {
			broke++
			continue
		}
		gotJSON, mErr := json.Marshal(got)
		if mErr != nil {
			t.Fatalf("marshal state: %v", mErr)
		}
		wantJSON, mErr := json.Marshal(want)
		if mErr != nil {
			t.Fatalf("marshal state: %v", mErr)
		}
		if string(gotJSON) != string(wantJSON) {
			broke++
		}
	}
	if broke == 0 {
		t.Fatal("a projector seeded at the resume point must corrupt the seat it catches up — " +
			"if it does not, Projector's doc comment is wrong about why the log must be replayed from the beginning")
	}
}

// runLogStepsResumingAt is the construction Projector's doc comment forbids:
// the STATE is folded from the beginning, as any server's would be, but the
// PROJECTOR is created at the resume point, exactly as it would be if it were
// built against a subscription starting at after=N. Batches before from are
// nil, since that seat receives nothing for them.
func runLogStepsResumingAt(v gateway.Viewer, steps []step, from int) [][]*vttv1.Envelope {
	st := engine.NewState()
	out := make([][]*vttv1.Envelope, len(steps))
	var pr *gateway.Projector
	for i, s := range steps {
		env := envelope(s.seq, s.payload)
		if err := engine.Apply(st, env); err != nil {
			panic(fmt.Sprintf("runLogStepsResumingAt seq %d: %v", s.seq, err))
		}
		if i == from {
			pr = gateway.NewProjector(v)
		}
		if pr != nil {
			out[i] = pr.Project(env, st)
		}
	}
	return out
}

func TestAProjectedStreamFoldsCleanly(t *testing.T) {
	// The failure spec §8 calls the worst available: "a TokenHidden arrives
	// for a token the client never had, and the strict fold throws, taking the
	// whole client down". Task 3 measured the real mechanism — Session re-folds
	// its whole log on every event, so one throw FREEZES that viewer's state
	// permanently — and both folds are mirrors, so what engine.Apply refuses
	// here is what the client refuses there.
	//
	// So this folds the projected stream with the very fold the client mirrors:
	// a duplicate introduction, a token placed in a scene the viewer was never
	// given, or a SceneSeen for an unknown scene all become a hard error rather
	// than a silent client freeze found at a demo gate.
	out := runLog(player(), aWholeFight())
	if len(out) == 0 {
		t.Fatal("nothing was projected, so nothing was proved")
	}
	viewerState := engine.NewState()
	for i, e := range out {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	if _, ok := viewerState.Tokens["t-gob"]; ok {
		t.Error("the goblin is behind a closed door again and must be gone from the folded view")
	}
	if _, ok := viewerState.Tokens["t-hero"]; !ok {
		t.Error("a player must still be able to see their own character")
	}
}

func TestARetractionReachesThePlayerWithoutItsReason(t *testing.T) {
	// TWO rulings in one payload, pulling opposite ways.
	//
	// The RANGE must reach every seat: a retraction erases history, and a
	// player who never receives it keeps folding an event the table has agreed
	// did not happen. Skipping a sequence they never received is a no-op, so
	// forwarding the numbers is free.
	//
	// The REASON must not. `EventsRetracted.reason` is free text supplied by
	// whoever called Undo — "mis-keyed, the archer is at 19,8" is an ordinary
	// thing to type — which is the note ruling's argument under a different
	// message name (spec §4.4: a note can say anything).
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	in := envelope(7, &vttv1.EventsRetracted{FromSequence: 6, ToSequence: 6,
		Reason: "mis-keyed: the archer is at 19,8"})
	var got *vttv1.EventsRetracted
	for _, e := range pr.Project(in, st) {
		if r := e.GetEventsRetracted(); r != nil {
			got = r
		}
	}
	if got == nil {
		t.Fatal("a retraction must reach every seat: withholding it leaves erased history standing")
	}
	if got.GetFromSequence() != 6 || got.GetToSequence() != 6 {
		t.Errorf("the range is what makes a retraction work and must survive intact, got [%d,%d]",
			got.GetFromSequence(), got.GetToSequence())
	}
	if got.GetReason() != "" {
		t.Errorf("a retraction's free-text reason must not reach a player, got %q", got.GetReason())
	}
	// And the redaction is a COPY: editing the shared envelope in place would
	// erase the reason from the DM's copy of history too.
	if in.GetEventsRetracted().GetReason() == "" {
		t.Error("the projection redacted the envelope it was handed instead of a copy of it")
	}

	// The DM's own stream keeps the reason, and by pointer: the identity
	// projection is byte-for-byte what it is today (spec §3.1).
	dm := gateway.NewProjector(gateway.Viewer{ParticipantID: "dm", Role: identity.RoleDM})
	out := dm.Project(in, st)
	if len(out) != 1 {
		t.Fatalf("the DM's retraction must be the identity projection: one envelope, got %d", len(out))
	}
	if out[0] != in {
		t.Fatal("the DM keeps the reason, in the very envelope they were sent, not a copy")
	}
}

func TestAMoveIsWithheldWhileTheMoverIsOutOfSight(t *testing.T) {
	// TokenMoved names BOTH ends of the walk. Forwarding one whose `from` the
	// player never saw hands them the hidden square this arc exists to
	// protect — session zero's (19,8) by another route.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	mustApply(st, 7, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 4, Y: 1}})
	in := envelope(7, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 4, Y: 1}})

	for _, e := range pr.Project(in, st) {
		if e.GetTokenMoved() != nil {
			t.Fatalf("a move by a token behind a closed door must not reach this player: %v", e)
		}
	}
}

func TestSteppingIntoViewArrivesRatherThanMoves(t *testing.T) {
	// The other half of the TokenMoved rule, and the one that leaks if it is
	// dropped: a token that was HIDDEN and is now visible must not be
	// announced with the move that brought it, because the move names where
	// it came from. It arrives instead, at where it now stands, and the
	// player learns nothing about the room it walked out of.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	mustApply(st, 7, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})
	in := envelope(7, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})

	var arrived bool
	for _, e := range pr.Project(in, st) {
		if e.GetTokenMoved() != nil {
			t.Error("a move OUT of the dark names the dark square it started in")
		}
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			arrived = true
			if tp.GetPosition().GetX() != 2 || tp.GetPosition().GetY() != 1 {
				t.Errorf("the goblin arrives where it now stands, got %v", tp.GetPosition())
			}
		}
	}
	if !arrived {
		t.Error("the goblin walked into the room and must appear")
	}
}

func TestADoorInARoomYouAreNotInStaysSilent(t *testing.T) {
	// Doors are forwarded only when the viewer can see the square (spec §4.2:
	// events about what their actors can see). A door swinging in a scene
	// they have never entered would announce that the scene exists.
	st := twoRooms()
	mustApply(st, 7, &vttv1.SceneCreated{
		SceneId: "lair", Name: "The Dragon's Lair", GridWidth: 5, GridHeight: 5,
		Tiles: map[string]*vttv1.TileRef{key(2, 2): {Kind: "door"}}})
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "lair", At: &vttv1.GridPosition{X: 2, Y: 2}})
	in := envelope(8, &vttv1.DoorOpened{SceneId: "lair", At: &vttv1.GridPosition{X: 2, Y: 2}})
	for _, e := range pr.Project(in, st) {
		if e.GetDoorOpened() != nil {
			t.Error("a door in a scene this player has never entered must not reach them")
		}
	}
}

func TestADoorYouCanSeeDoesReachThePlayer(t *testing.T) {
	// THE CONTROL FOR THE TEST ABOVE, and it was missing until a
	// mutation run said so. Withholding EVERY door satisfies
	// TestADoorInARoomYouAreNotInStaysSilent completely while
	// making every door in the game silent, so that test alone pins
	// only half the ruling ("forward iff the square is visible").
	//
	// Found rather than guessed: CONDITIONALS_NEGATION on canSeeSquare's nil
	// guard — `if at == nil` -> `if at != nil`, which returns false for every
	// real position and so withholds every door — SURVIVED the suite. The
	// neighbouring TokenMoved pair had both halves; doors had only the
	// negative one.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	in := envelope(7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})

	// EXACTLY ONE, because two code paths now know about doors: classify
	// forwards this envelope, and doorTransitions corrects the viewer's belief
	// about every door square they can see — and this square is one. The
	// forwarded envelope is the one that should survive, since it carries the
	// provenance a synthesized one cannot. A duplicate folds harmlessly (both
	// folds are idempotent on doors) which is exactly why nothing else would
	// report it.
	var opened int
	for _, e := range pr.Project(in, st) {
		if e.GetDoorOpened() != nil {
			opened++
		}
	}
	if opened != 1 {
		t.Fatalf("a player must watch the door in their own room swing open, exactly once, got %d", opened)
	}
}

func TestADoorYouCanSeeSwingShutReachesThePlayerOnce(t *testing.T) {
	// The CLOSING direction, which the review named as the case that must not
	// break: a closed door's square is visible from the adjacent room, so
	// shutting a door in your own room does reach you.
	//
	// It is a separate test rather than a table row because the two
	// directions run through separate arms of doorSubject, and the
	// mutation gate proved that matters: with only the opening test
	// written, CONDITIONALS_NEGATION on the DoorClosed arm's `at != nil`
	// SURVIVED while the identical mutant on the DoorOpened arm was
	// killed. Negating it makes doorSubject disown the event, so the diff
	// stops recognising the causing square and emits a second DoorClosed
	// alongside the forwarded one.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	pr.Project(envelope(7, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	mustApply(st, 8, &vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	var closed int
	for _, e := range pr.Project(envelope(8, &vttv1.DoorClosed{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st) {
		if e.GetDoorClosed() != nil {
			closed++
		}
	}
	if closed != 1 {
		t.Fatalf("a player must watch the door in their own room swing shut, exactly once, got %d", closed)
	}
}

func TestAVisibleTokensMoveDoesReachThePlayer(t *testing.T) {
	// The control for TestAMoveIsWithheldWhileTheMoverIsOutOfSight: withholding
	// EVERY move would satisfy it while making the board static.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	mustApply(st, 7, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})
	in := envelope(7, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})

	var moved bool
	for _, e := range pr.Project(in, st) {
		if e.GetTokenMoved() != nil {
			moved = true
		}
	}
	if !moved {
		t.Fatal("a player must watch their own character walk")
	}
}

// oneRoomWithAPillar is twoRooms' corridor with the dividing wall removed and
// a sight-blocking OBJECT in its place: 7x3, walled edges, floor from 1,1 to
// 5,1, a pillar at 3,1 and a crate at 4,1. Nothing here is terrain — the only
// thing between the hero and the goblin is scenery.
//
//	 x: 0    1     2    3       4      5    6
//	y=1 wall hero  flr  pillar  crate  gob  wall
func oneRoomWithAPillar() *engine.State {
	st := engine.NewState()
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		tiles[key(x, 0)] = &vttv1.TileRef{Kind: "wall"}
		tiles[key(x, 2)] = &vttv1.TileRef{Kind: "wall"}
		tiles[key(x, 1)] = &vttv1.TileRef{Kind: "floor"}
	}
	tiles[key(0, 1)] = &vttv1.TileRef{Kind: "wall"}
	tiles[key(6, 1)] = &vttv1.TileRef{Kind: "wall"}

	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: tiles,
		Objects: []*vttv1.SceneObject{
			{ObjectId: "pillar", Kind: "pillar", At: &vttv1.GridPosition{X: 3, Y: 1},
				Width: 1, Height: 1, BlocksSight: true, BlocksMove: true},
			{ObjectId: "crate", Kind: "crate", At: &vttv1.GridPosition{X: 4, Y: 1},
				Width: 1, Height: 1},
			// Covers no square at all, so sight.Blockers casts no shadow for
			// it (movement's covers() admits nothing once the extent is
			// below 1) and nothing can ever reveal it. Sitting right next to
			// the hero is what makes that a real assertion rather than a
			// coincidence of distance.
			{ObjectId: "ghost", Kind: "smudge", At: &vttv1.GridPosition{X: 2, Y: 1},
				Width: 0, Height: 1},
		}})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	return st
}

func TestSightBlockingSceneryHidesWhatStandsBehindIt(t *testing.T) {
	// Spec §3.3/§3.5: "trees are pillars — you cannot see THROUGH a tree, only
	// BETWEEN trees." An object carrying blocks_sight is as opaque as a wall,
	// and the objects the viewer can see travel with SceneSeen — so a pillar
	// in the room is drawn and the crate in its shadow is not.
	st := oneRoomWithAPillar()
	pr := gateway.NewProjector(player())
	out := firstPlace(pr, st)

	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			t.Error("the goblin is behind a pillar — scenery blocks sight exactly as a wall does")
		}
	}

	var seen *vttv1.SceneSeen
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
	}
	if seen == nil {
		t.Fatal("a player in a scene must be told what they can see of it")
	}
	var ids []string
	for _, o := range seen.GetObjects() {
		ids = append(ids, o.GetObjectId())
	}
	if len(ids) != 1 || ids[0] != "pillar" {
		t.Errorf("the pillar is in plain sight and the crate is in its shadow, got %v", ids)
	}
}

// oneRoomWithAGapInTheWall is a 5x5 room cut in two by a wall along y=2 with a
// single floor GAP at 2,2. The hero stands at 2,4, so the northern half is dark
// except for the narrow cone the gap admits — which is what gives this fixture
// a dark square with LIT squares immediately below it and to its right.
//
//	 x: 0      1     2     3     4
//	y=0 flr    flr   flr   flr   flr
//	y=1 CACHE  flr   flr   flr   flr
//	y=2 wall   wall  gap   wall  wall
//	y=3 flr    flr   flr   flr   flr
//	y=4 flr    flr   hero  flr   flr
func oneRoomWithAGapInTheWall() *engine.State {
	st := engine.NewState()
	tiles := map[string]*vttv1.TileRef{}
	for y := int32(0); y < 5; y++ {
		for x := int32(0); x < 5; x++ {
			tiles[key(x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	for _, x := range []int32{0, 1, 3, 4} {
		tiles[key(x, 2)] = &vttv1.TileRef{Kind: "wall"}
	}
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 5, GridHeight: 5, Tiles: tiles,
		Objects: []*vttv1.SceneObject{
			// Blocks nothing: this test is about what REVEALS an object, so it
			// must not also be the thing casting the shadow it hides in.
			{ObjectId: "cache", Kind: "chest", At: &vttv1.GridPosition{X: 0, Y: 1},
				Width: 1, Height: 1},
		}})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}})
	mustApply(st, 4, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 2, Y: 4}})
	return st
}

func TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn(t *testing.T) {
	// An object is shown when its OWN footprint is visible — never because a
	// square just past its edge is. objectInSight walks the half-open extent
	// [o.Y, o.Y+o.Height) x [o.X, o.X+o.Width), and both bounds are one
	// mutation away from being inclusive.
	//
	// FOUND BY THE MUTATION GATE, not by reading: CONDITIONALS_BOUNDARY on
	// `int64(y) < int64(o.Y)+int64(o.Height)` (project.go:510:54) and on its
	// x twin (511:54) both SURVIVED the suite. Each makes the walk one square
	// too generous, so an object is revealed by the square below it or to its
	// right. That is a visibility LEAK of the arc's own kind — a chest the
	// player cannot see, drawn on their board because they can see the floor
	// next to it. TestSightBlockingSceneryHidesWhatStandsBehindIt did not
	// catch it: there the hidden crate has nothing lit adjacent.
	st := oneRoomWithAGapInTheWall()
	pr := gateway.NewProjector(player())
	out := pr.Project(envelope(4, &vttv1.TokenPlaced{TokenId: "t-hero",
		SceneId: "s", ActorId: "hero", Position: &vttv1.GridPosition{X: 2, Y: 4}}), st)

	var seen *vttv1.SceneSeen
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
	}
	if seen == nil {
		t.Fatal("a player in a scene must be told what they can see of it")
	}
	lit := seen.GetTiles() // every square carries terrain, so this IS the visible set

	// THE PRECONDITIONS ARE THE TEST. Without all three the assertion below
	// passes for the wrong reason: an unlit neighbour cannot reveal anything,
	// so a fixture that drifted would quietly stop pinning the boundary while
	// still going green.
	if _, ok := lit[key(0, 1)]; ok {
		t.Fatalf("fixture: the cache's own square must be DARK or nothing below is proved, lit %v", keysOf(lit))
	}
	if _, ok := lit[key(0, 2)]; !ok {
		t.Fatalf("fixture: the square just BELOW the cache must be LIT to catch the y bound, lit %v", keysOf(lit))
	}
	if _, ok := lit[key(1, 1)]; !ok {
		t.Fatalf("fixture: the square just RIGHT of the cache must be LIT to catch the x bound, lit %v", keysOf(lit))
	}

	for _, o := range seen.GetObjects() {
		if o.GetObjectId() == "cache" {
			t.Error("the cache stands on a square this player cannot see — " +
				"a lit square past its edge must not reveal it")
		}
	}
}

// keysOf reports a tile map's squares in a stable order, so a fixture failure
// above names the visible set instead of a Go map's random spelling of it.
func keysOf(m map[string]*vttv1.TileRef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestAnIntroducedSceneArrivesWithItsDoorsAlreadyOpen(t *testing.T) {
	// THE ONBOARDING FLOW, which is how this defect actually reaches a table:
	// you log in with no character and the DM assigns one afterwards. Every
	// door worked before that assignment was withheld — correctly, the seat had
	// no eyes — so the introduction batch is the ONLY chance to tell the viewer
	// which doors stand open.
	//
	// engine.Scene.OpenDoors is NOT a field of anything the introduction already
	// carries. The outline is a redacted SceneCreated, and SceneSeen carries
	// tiles and objects; a door's OPEN state lives in neither. This is the
	// conditions problem at the scene layer: a fact that does not ride along
	// with the thing it belongs to.
	//
	// It never self-corrects and never throws — OpenDoors is written only by the
	// two door arms, and both folds treat a repeat as idempotent — so the seat
	// sees a lit room through a door its client draws SHUT
	// (client/src/view/scene-plan.ts:74,89), for the rest of the session.
	st := twoRooms()
	mustApply(st, 7, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})

	// A seat with no actor yet: the door event is rightly withheld. The batch
	// is KEPT — it already carries ActorAdded for the party (spec §5 knows a
	// controlled actor before it can see one), and a batch is not a fold unit.
	pr := gateway.NewProjector(gateway.Viewer{ParticipantID: "p-2", Role: identity.RolePlayer})
	before := pr.Project(envelope(7, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st)
	for _, e := range before {
		if e.GetDoorOpened() != nil {
			t.Fatal("a seat with no actor is in no scene and must not hear a door")
		}
	}

	// The DM assigns them a character. THIS is the introduction batch.
	mustApply(st, 8, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-2"})
	out := pr.Project(envelope(8, &vttv1.ActorControlGranted{
		ActorId: "hero", ParticipantId: "p-2"}), st)

	viewer := engine.NewState()
	for i, e := range append(append([]*vttv1.Envelope{}, before...), out...) {
		if err := engine.Apply(viewer, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	got := viewer.Scenes["s"].OpenDoors
	if !got[key(3, 1)] {
		t.Fatalf("the introduced scene must arrive with its open doors open, got %v", got)
	}
	if len(got) != len(st.Scenes["s"].OpenDoors) {
		t.Errorf("the viewer's doors must agree with the table's, got %v want %v",
			got, st.Scenes["s"].OpenDoors)
	}
}

func TestADoorOpenedOutOfSightArrivesWhenTheSquareComesIntoView(t *testing.T) {
	// The sibling of the test above, and the same defect without any
	// introduction involved. The viewer is already IN the scene; a door they
	// cannot see is opened, which classify rightly withholds; then they walk far
	// enough to see that square. SceneSeen brings them the door TILE, which says
	// a door is there and nothing about whether it stands open.
	//
	// So the viewer's belief about a door has to be refreshed whenever its
	// square comes into view, not only when the scene is first introduced.
	st := oneRoomWithAGapInTheWall()
	pr := gateway.NewProjector(player())
	pr.Project(envelope(4, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 2, Y: 4}}), st)

	// 0,1 is dark from 2,4 — TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn
	// pins that. Opening a door there must tell this viewer nothing yet.
	mustApply(st, 5, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 0, Y: 1}})
	for _, e := range pr.Project(envelope(5, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 0, Y: 1}}), st) {
		if e.GetDoorOpened() != nil {
			t.Fatal("a door on a square this viewer cannot see must stay silent")
		}
	}

	// Now they walk north through the gap, and 0,1 comes into view.
	mustApply(st, 6, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 2, Y: 4}, To: &vttv1.GridPosition{X: 1, Y: 1}})
	out := pr.Project(envelope(6, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 2, Y: 4}, To: &vttv1.GridPosition{X: 1, Y: 1}}), st)

	var seen *vttv1.SceneSeen
	var opened bool
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
		if do := e.GetDoorOpened(); do != nil && do.GetAt().GetX() == 0 && do.GetAt().GetY() == 1 {
			opened = true
		}
	}
	if seen == nil {
		t.Fatal("moving must re-report what the viewer can see")
	}
	if _, lit := seen.GetTiles()[key(0, 1)]; !lit {
		t.Fatalf("fixture: 0,1 must be VISIBLE from 1,1 or this proves nothing, lit %v", keysOf(seen.GetTiles()))
	}
	if !opened {
		t.Fatal("a door that comes into view must arrive in the state it is actually in, " +
			"or the client draws it shut for the rest of the session")
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
	known := &vttv1.Envelope{Sequence: 8, Payload: &vttv1.Envelope_ConditionApplied{
		ConditionApplied: &vttv1.ConditionApplied{ActorId: "hero", ConditionId: "hidden"}}}
	var forwarded bool
	for _, e := range pr.Project(known, st) {
		if e == known {
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

func TestNarrationReachesAPlayerAndANoteDoesNot(t *testing.T) {
	// The two payloads spec §4.4 names as reasons a forwarding default ships
	// broken, and they are ruled OPPOSITE ways on one distinction: narration
	// is ADDRESSED to the table by whoever writes it, and a note is a private
	// world record the DM keeps. See project.go for the full reasoning; this
	// test is what stops either ruling drifting silently.
	st := twoRooms()
	pr := gateway.NewProjector(player())
	firstPlace(pr, st)

	narration := envelope(7, &vttv1.NarrationAdded{Text: "The door creaks.", As: "DM"})
	var toldStory bool
	for _, e := range pr.Project(narration, st) {
		if e.GetNarrationAdded() != nil {
			toldStory = true
		}
	}
	if !toldStory {
		t.Error("withholding narration from players silences the table's story channel")
	}

	note := envelope(8, &vttv1.NoteUpserted{
		Key: "ambush", Title: "Ambush", Text: "Archer waits at 19,8"})
	for _, e := range pr.Project(note, st) {
		if e.GetNoteUpserted() != nil {
			t.Error("a note can say anything (spec §4.4) and must not reach a player")
		}
	}

	// BOTH note arms, because classify rules on them together and a test that
	// exercises one leaves the other free to drift to the opposite ruling. The
	// KEY alone is the leak here: "ambush" names the DM's plan whether or not
	// any text travels with it.
	deleted := envelope(9, &vttv1.NoteDeleted{Key: "ambush"})
	for _, e := range pr.Project(deleted, st) {
		if e.GetNoteDeleted() != nil {
			t.Error("deleting a note names the note, and must not reach a player either")
		}
	}
}
