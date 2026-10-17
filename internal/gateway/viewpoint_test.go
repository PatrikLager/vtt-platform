package gateway_test

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// key must format a square as the wire does, column then row, as engine's
// gridKey and sight do.
func key(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

// envelope wraps payload at seq. Keep the panic on an unknown payload: an empty
// Envelope fails later and blames the fold.
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

// Keep every test's world twoRooms: hero is a party member p-1 controls, goblin
// an NPC nobody holds.
func spectator() *identity.Participant {
	return &identity.Participant{ID: "s-1", Role: identity.RoleSpectator}
}

func owner() *identity.Participant {
	return &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
}

// VTT-181
func TestASpectatorMayPerchOnAPartyMemberButNotOnAnNPC(t *testing.T) {
	st := twoRooms()
	if err := gateway.MayWatch(spectator(), "hero", st.Actors); err != nil {
		t.Errorf("a spectator may watch a party member: %v", err)
	}
	if err := gateway.MayWatch(spectator(), "goblin", st.Actors); err == nil {
		t.Fatal("watching an NPC must be refused by the server")
	}
}

// VTT-181
func TestASpectatorMayNotPerchOnAnNPCTheDMControls(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "dm-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	if err := gateway.MayWatch(spectator(), "goblin", st.Actors); err == nil {
		t.Fatal("a monster the DM holds is still a monster: watching it must be refused")
	}
}

// VTT-181
func TestAnActorWithNoDeclaredKindIsNoShoulderHoweverManyHoldIt(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wisp", Name: "Wisp"}})
	// A kindless grant: the command boundary refuses it and the fold accepts it.
	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "wisp", ParticipantId: "s-1"})
	mustApply(st, 10, &vttv1.ActorControlGranted{ActorId: "wisp", ParticipantId: "p-1"})
	for _, p := range []*identity.Participant{spectator(), owner()} {
		if err := gateway.MayWatch(p, "wisp", st.Actors); err == nil {
			t.Errorf("%s may watch an actor that never said what it is", p.Role)
		}
		// Keep the control: a MayWatch refusing everything passes the line above.
		if err := gateway.MayWatch(p, "hero", st.Actors); err != nil {
			t.Errorf("%s may not watch a declared party member it may watch: %v", p.Role, err)
		}
	}
}

// VTT-343
func TestAPlayerMayWatchOnlyThePartyMembersTheyControl(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	if err := gateway.MayWatch(owner(), "hero", st.Actors); err != nil {
		t.Errorf("a player may watch the party member they control: %v", err)
	}
	if err := gateway.MayWatch(owner(), "scout", st.Actors); err == nil {
		t.Error("a player may not watch another player's party member")
	}
}

// Compare bytes: a message that merely avoids naming the reason is still an
// oracle (TestAPlayerCannotProbeTheDarkWithMoveCommands).
// VTT-182 VTT-347
func TestAPerchRefusalDoesNotSayWhetherTheActorExists(t *testing.T) {
	present := twoRooms()
	mustApply(present, 8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(present, 9, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	absent := twoRooms()
	delete(absent.Actors, "goblin")
	delete(absent.Tokens, "t-gob")

	for _, p := range []*identity.Participant{spectator(), owner()} {
		npc := gateway.MayWatch(p, "goblin", present.Actors)
		unknown := gateway.MayWatch(p, "goblin", absent.Actors)
		if npc == nil || unknown == nil {
			t.Fatalf("%s: both must be refused (npc=%v unknown=%v)", p.Role, npc, unknown)
		}
		if npc.Error() != unknown.Error() {
			t.Fatalf("%s: a refusal says whether the actor exists:\n npc: %s\n absent: %s", p.Role, npc, unknown)
		}
	}
	others := gateway.MayWatch(owner(), "scout", present.Actors)
	npc := gateway.MayWatch(owner(), "goblin", present.Actors)
	if others == nil || others.Error() != gateway.MayWatch(owner(), "scout", absent.Actors).Error() ||
		strings.ReplaceAll(others.Error(), "scout", "goblin") != npc.Error() {
		t.Fatalf("a player's refusal says whose party member it is: %v / %v", others, npc)
	}
}

// VTT-140
func TestTheDMAndTheAgentWatchNoCharacterLog(t *testing.T) {
	st := twoRooms()
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := &identity.Participant{ID: "p-1", Role: role}
		if err := gateway.MayWatch(p, "hero", st.Actors); err == nil {
			t.Errorf("role %q may watch a character log: it is sent the world", role)
		}
	}
}

// VTT-345
func TestUnperchingNamesNoActorAndIsAllowed(t *testing.T) {
	if err := gateway.MayWatch(spectator(), "", twoRooms().Actors); err != nil {
		t.Fatalf("naming no actor is how a spectator leaves a shoulder: %v", err)
	}
}

// VTT-346
func TestAPlayerMayNotWatchNobody(t *testing.T) {
	if err := gateway.MayWatch(owner(), "", twoRooms().Actors); err == nil {
		t.Fatal("a player's set_viewpoint naming no actor was accepted")
	}
}
