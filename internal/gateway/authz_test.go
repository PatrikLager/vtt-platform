package gateway_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// moveTokenCmd builds a MoveToken ClientCommand targeting tokenID.
func moveTokenCmd(tokenID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: tokenID, To: &vttv1.GridPosition{X: 1, Y: 1}},
	}}
}

// commandFor builds a minimal, valid ClientCommand for the given oneof field
// name. move_token targets "t1" so it composes with the ownership fixture
// shared by the table-driven test below.
func commandFor(t *testing.T, name string) *vttv1.ClientCommand {
	t.Helper()
	switch name {
	case "move_token":
		return moveTokenCmd("t1")
	case "grant_actor_control":
		return grantActorControlCmd("p-2")
	case "promote_participant":
		return promoteCmd("p-2", "player")
	case "set_join_door":
		return &vttv1.ClientCommand{
			RequestId: "r-door",
			Command: &vttv1.ClientCommand_SetJoinDoor{
				SetJoinDoor: &vttv1.SetJoinDoor{Door: vttv1.JoinDoor_JOIN_DOOR_OPEN},
			},
		}
	case "rotate_join_link":
		return &vttv1.ClientCommand{
			RequestId: "r-rotate",
			Command: &vttv1.ClientCommand_RotateJoinLink{
				RotateJoinLink: &vttv1.RotateJoinLink{},
			},
		}
	case "revoke_actor_control":
		// Names "p-1", which is who the table test runs as — so the player
		// row exercises the SELF case, the only one a player may issue.
		return revokeActorControlCmd("p-1")
	case "create_scene":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_CreateScene{
			CreateScene: &vttv1.CreateScene{SceneId: "s1", Name: "Cave"},
		}}
	case "add_actor":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{
			AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: "a2", Name: "Goblin",
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		}}
	case "place_token":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{
			PlaceToken: &vttv1.PlaceToken{TokenId: "t2", SceneId: "s1", ActorId: "a1"},
		}}
	case "start_session":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{
			StartSession: &vttv1.StartSession{Name: "session"},
		}}
	case "end_session":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{
			EndSession: &vttv1.EndSession{},
		}}
	case "retract_events":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RetractEvents{
			RetractEvents: &vttv1.RetractEvents{FromSequence: 1, ToSequence: 1, Reason: "r"},
		}}
	case "use_ability":
		return useAbilityCmd("a1")
	case "remove_condition":
		return removeConditionCmd("a1")
	case "add_narration":
		return addNarrationCmd()
	case "upsert_note":
		return upsertNoteCmd()
	case "delete_note":
		return deleteNoteCmd()
	case "load_adventure":
		return loadAdventureCmd()
	case "load_map":
		return loadMapCmd()
	case "open_door":
		// Scene "scn", not "s1": Task 6's adjacency check (mayWorkDoor) now
		// consults the state, and "scn" is where ownershipFixture's t1 sits —
		// at the zero position, adjacent to (0,1). See ownershipFixture's own
		// comment for why that placement is deliberate, not incidental.
		return openDoorCmd("scn", 0, 1)
	case "close_door":
		return closeDoorCmd("scn", 0, 1)
	case "set_viewpoint":
		// "a1", not a name of its own: MayPerch (Task 6) now runs inside
		// Authorize and only a PARTY MEMBER is a shoulder (spec §5.1), so the
		// spectator cell in the matrix needs an actor that qualifies —
		// ownershipFixture's "a1" declares no kind and has a controller, which
		// is §5.1's migration shape and reads as a party member. Same dependency the open_door/close_door cases
		// above have on that fixture's token position.
		return setViewpointCmd("a1")
	default:
		t.Fatalf("commandFor: unknown command name %q", name)
		return nil
	}
}

// setViewpointCmd builds a minimal, valid SetViewpoint ClientCommand naming
// actorID as the shoulder to perch on (visibility spec §3.1.1). Authorize
// checks role AND the actor: MayPerch runs in the every-role section, above
// the player-only switch that carries mayWorkDoor, because a perch is a
// spectator's command and that switch never fires for one.
func setViewpointCmd(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID},
	}}
}

// loadAdventureCmd builds a minimal, valid LoadAdventure ClientCommand
// (adventure-format Task 4). Authorize never checks adventure existence —
// only role — so a bare id is a trivially valid command for authz purposes;
// the gateway handler (adventure.go) owns the "unknown adventure" lookup.
func loadAdventureCmd() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_LoadAdventure{
		LoadAdventure: &vttv1.LoadAdventure{AdventureId: "goblin-ambush"},
	}}
}

// loadMapCmd builds a minimal, valid LoadMap ClientCommand (maps-as-geometry
// C1 remediation) — loadAdventureCmd's sibling. Authorize never checks map
// existence, only role, so a bare id is trivially valid for authz purposes;
// the gateway handler (map.go) owns the "unknown map" lookup.
func loadMapCmd() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_LoadMap{
		LoadMap: &vttv1.LoadMap{MapId: "cellar"},
	}}
}

// addNarrationCmd builds a plain, unanchored AddNarration ClientCommand — a
// trivially valid command for authz purposes (Authorize never checks
// narration content/anchor sanity, only role; that validation lives in the
// fold, internal/engine/apply.go).
func addNarrationCmd() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: "the door creaks open"},
	}}
}

// upsertNoteCmd builds a minimal, valid UpsertNote ClientCommand.
func upsertNoteCmd() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UpsertNote{
		UpsertNote: &vttv1.UpsertNote{Key: "n1", Title: "A Note", Text: "note text"},
	}}
}

// deleteNoteCmd builds a minimal, valid DeleteNote ClientCommand.
func deleteNoteCmd() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_DeleteNote{
		DeleteNote: &vttv1.DeleteNote{Key: "n1"},
	}}
}

// useAbilityCmd builds a UseAbility ClientCommand acting AS actorID,
// self-targeting (a trivially valid target list for authz purposes —
// Authorize never checks ability/target validity, only role and actor
// ownership; that deeper validation is rules.Resolve's job, ruleset-
// interpreter Task 6).
func useAbilityCmd(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{
		UseAbility: &vttv1.UseAbility{ActorId: actorID, AbilityId: "jab", TargetIds: []string{actorID}},
	}}
}

// removeConditionCmd builds a RemoveCondition ClientCommand acting AS
// actorID.
func removeConditionCmd(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveCondition{
		RemoveCondition: &vttv1.RemoveCondition{ActorId: actorID, ConditionId: "dazed"},
	}}
}

// grantActorControlCmd builds a GrantActorControl naming participantID as the
// participant GAINING control of actor "a1".
func grantActorControlCmd(participantID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{
		GrantActorControl: &vttv1.GrantActorControl{ActorId: "a1", ParticipantId: participantID},
	}}
}

// revokeActorControlCmd builds a RevokeActorControl naming participantID as the
// participant LOSING control of actor "a1".
func revokeActorControlCmd(participantID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{ActorId: "a1", ParticipantId: participantID},
	}}
}

// promoteCmd builds a PromoteParticipant naming who is being promoted, and to
// what.
func promoteCmd(participantID, role string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PromoteParticipant{
		PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: participantID, Role: role},
	}}
}

// openDoorCmd/closeDoorCmd build door commands with a caller-chosen scene and
// position, parameterized (unlike commandFor's fixed builders) so the
// adjacency tests below can place the door near or far from a token without
// a family of near-duplicate literals.
func openDoorCmd(sceneID string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_OpenDoor{
		OpenDoor: &vttv1.OpenDoor{SceneId: sceneID, At: &vttv1.GridPosition{X: x, Y: y}},
	}}
}

func closeDoorCmd(sceneID string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: sceneID, At: &vttv1.GridPosition{X: x, Y: y}},
	}}
}

// authzCase is one cell of the 21 commands x 4 roles authorization matrix.
// want is written out LITERALLY per task-4-brief.md Step 1 — it must never
// be derived from commandRoles (the map under test) or this test proves
// nothing about the table's actual content.
type authzCase struct {
	command string
	role    identity.Role
	want    bool
}

// authzCases is the full 88-cell matrix (spec §4/§7, grown from 84 by
// visibility Task 6's set_viewpoint row, from 80 by the
// whole-branch-review C1 remediation's load_map row, from 72 by
// maps-as-geometry Task 1's open_door/close_door rows, from 52 by
// presence-and-actor-control Task 3's grant/revoke_actor_control rows, from 48 by
// adventure-format Task 4's load_adventure row, which itself grew from 36 by
// world-layer Task 3's add_narration/upsert_note/delete_note rows, and from
// 28 by ruleset-interpreter Task 6's use_ability/remove_condition rows):
// every command against every one of the four roles. move_token/player,
// use_ability/player, remove_condition/player, open_door/player and
// close_door/player are all TRUE here because the shared fixture in
// TestAuthorizeTableAllCommandsAllRoles gives participant "p-1" ownership of
// actor "a1" (and its token "t1", standing adjacent to the door commandFor
// builds) — the table alone allows it, and the dedicated ownership/adjacency
// tests below independently prove each additional check. add_narration/upsert_note/
// delete_note have NO ownership check at all (spec §5: "world facts are the
// DM's" is a role-only gate, unlike move_token/use_ability/remove_condition's
// per-actor ownership) — a plain role lookup is the entire story for these
// three rows. load_adventure is dm/agent only (spec §7: "the DM calls
// load_adventure when the table is ready") — same shape as create_scene/
// add_actor/place_token, no ownership check, no player row.
var authzCases = []authzCase{
	{"move_token", identity.RoleDM, true},
	{"move_token", identity.RoleAgent, true},
	{"move_token", identity.RolePlayer, true},
	{"move_token", identity.RoleSpectator, false},

	{"create_scene", identity.RoleDM, true},
	{"create_scene", identity.RoleAgent, true},
	{"create_scene", identity.RolePlayer, false},
	{"create_scene", identity.RoleSpectator, false},

	{"add_actor", identity.RoleDM, true},
	{"add_actor", identity.RoleAgent, true},
	{"add_actor", identity.RolePlayer, false},
	{"add_actor", identity.RoleSpectator, false},

	{"place_token", identity.RoleDM, true},
	{"place_token", identity.RoleAgent, true},
	{"place_token", identity.RolePlayer, false},
	{"place_token", identity.RoleSpectator, false},

	{"start_session", identity.RoleDM, true},
	{"start_session", identity.RoleAgent, true},
	{"start_session", identity.RolePlayer, false},
	{"start_session", identity.RoleSpectator, false},

	{"end_session", identity.RoleDM, true},
	{"end_session", identity.RoleAgent, true},
	{"end_session", identity.RolePlayer, false},
	{"end_session", identity.RoleSpectator, false},

	{"retract_events", identity.RoleDM, true},
	{"retract_events", identity.RoleAgent, true},
	{"retract_events", identity.RolePlayer, false},
	{"retract_events", identity.RoleSpectator, false},

	{"use_ability", identity.RoleDM, true},
	{"use_ability", identity.RoleAgent, true},
	{"use_ability", identity.RolePlayer, true},
	{"use_ability", identity.RoleSpectator, false},

	{"remove_condition", identity.RoleDM, true},
	{"remove_condition", identity.RoleAgent, true},
	{"remove_condition", identity.RolePlayer, true},
	{"remove_condition", identity.RoleSpectator, false},

	{"add_narration", identity.RoleDM, true},
	{"add_narration", identity.RoleAgent, true},
	{"add_narration", identity.RolePlayer, true},
	{"add_narration", identity.RoleSpectator, false},

	{"upsert_note", identity.RoleDM, true},
	{"upsert_note", identity.RoleAgent, true},
	{"upsert_note", identity.RolePlayer, false},
	{"upsert_note", identity.RoleSpectator, false},

	{"delete_note", identity.RoleDM, true},
	{"delete_note", identity.RoleAgent, true},
	{"delete_note", identity.RolePlayer, false},
	{"delete_note", identity.RoleSpectator, false},

	{"load_adventure", identity.RoleDM, true},
	{"load_adventure", identity.RoleAgent, true},
	{"load_adventure", identity.RolePlayer, false},
	{"load_adventure", identity.RoleSpectator, false},

	// load_map (whole-branch-review C1 remediation): dm/agent only, same
	// shape and same reasoning as load_adventure directly above — loading a
	// map rewrites the table's world, exactly like loading an adventure.
	{"load_map", identity.RoleDM, true},
	{"load_map", identity.RoleAgent, true},
	{"load_map", identity.RolePlayer, false},
	{"load_map", identity.RoleSpectator, false},

	// grant/revoke_actor_control (presence-and-actor-control Task 3, spec
	// §5.3). Handing a character to someone is the DM's, so grant has NO
	// player row. Revoke DOES: a player may put a character DOWN, naming
	// only THEMSELVES — you cannot take one from someone else. That is an
	// additional self-check, the same shape as the ownership helpers, and
	// the player cell below is true only because commandFor names "p-1",
	// which is the participant the table test runs as.
	// TestAuthorizePlayerRevokeOtherParticipantDenied proves the other half
	// independently — a row that only ever says yes is not a guard.
	{"grant_actor_control", identity.RoleDM, true},
	{"grant_actor_control", identity.RoleAgent, true},
	{"grant_actor_control", identity.RolePlayer, false},
	{"grant_actor_control", identity.RoleSpectator, false},

	{"revoke_actor_control", identity.RoleDM, true},
	{"revoke_actor_control", identity.RoleAgent, true},
	{"revoke_actor_control", identity.RolePlayer, true},
	{"revoke_actor_control", identity.RoleSpectator, false},

	// promote_participant (joining-a-table J3, spec §3.1a). DM and agent only,
	// with NO player row at all: a shared join link mints spectators, and a
	// player able to promote would make that link a path to authority in two
	// steps. TestASpectatorCannotPromoteItself covers the case the default
	// exists to prevent.
	// set_join_door / rotate_join_link (joining-a-table J6, spec §5). DM and
	// agent only, by the same argument that gates grant_actor_control: an OPEN
	// DOOR MINTS PARTICIPANTS. A spectator who could open one — and every
	// joiner through the shared link arrives as a spectator — could staff the
	// table with strangers, which is the exact thing admitting-as-spectator
	// exists to prevent.
	//
	// Rotation is gated for the mirror-image reason: it is the only way to
	// close a leaked link, so anyone who can rotate can also lock the DM's own
	// link out from under them mid-session.
	{"set_join_door", identity.RoleDM, true},
	{"set_join_door", identity.RoleAgent, true},
	{"set_join_door", identity.RolePlayer, false},
	{"set_join_door", identity.RoleSpectator, false},

	{"rotate_join_link", identity.RoleDM, true},
	{"rotate_join_link", identity.RoleAgent, true},
	{"rotate_join_link", identity.RolePlayer, false},
	{"rotate_join_link", identity.RoleSpectator, false},

	{"promote_participant", identity.RoleDM, true},
	{"promote_participant", identity.RoleAgent, true},
	{"promote_participant", identity.RolePlayer, false},
	{"promote_participant", identity.RoleSpectator, false},

	// open_door/close_door (maps-as-geometry Task 6, spec §6: "hard for
	// players, free for DM"). dm/agent/player all TRUE — a player may work a
	// door same as they may move a token — spectator FALSE, same shape as
	// every other command. The player cells are TRUE here only because
	// commandFor's open_door/close_door target a door adjacent to
	// ownershipFixture's t1 (mayWorkDoor, wired below in Authorize's switch);
	// TestAuthorizePlayerMayNotWorkDistantDoor proves the OTHER direction —
	// a row that only ever says yes is not a guard, the same argument the
	// revoke_actor_control comment above already makes for its own player
	// cell. dm/agent are TRUE unconditionally: mayWorkDoor returns nil for
	// any non-player role before it ever looks at token position, which is
	// the mechanical form of "free for DM" — TestAuthorizeDMMayWorkDoorRegardlessOfTokenPosition
	// pins it with a token nowhere near the door.
	{"open_door", identity.RoleDM, true},
	{"open_door", identity.RoleAgent, true},
	{"open_door", identity.RolePlayer, true},
	{"open_door", identity.RoleSpectator, false},

	{"close_door", identity.RoleDM, true},
	{"close_door", identity.RoleAgent, true},
	{"close_door", identity.RolePlayer, true},
	{"close_door", identity.RoleSpectator, false},

	// set_viewpoint (visibility Task 6, spec §3.1.1). THE ONLY ROW IN THIS
	// TABLE WHOSE ONLY TRUE CELL IS THE SPECTATOR'S, and it is the inverse of
	// every row above for a reason: a perch is how a watcher with no character
	// of their own borrows someone else's eyes, and "an unassigned PLAYER does
	// not perch" — their answer to an empty board is to be GIVEN a character,
	// which is the onboarding flow working as intended. The DM and the agent
	// see everything already, so there is no shoulder for them to gain.
	//
	// The spectator cell is TRUE here only because commandFor names "a1", which
	// ownershipFixture gives a controller — MayPerch, wired into Authorize's
	// every-role section, refuses any other kind of actor.
	// TestAuthorizeSpectatorMayNotPerchOnAnNpc proves that other direction: a
	// row that only ever says yes is not a guard, the same argument the
	// revoke_actor_control and open_door comments above make for their own
	// player cells.
	{"set_viewpoint", identity.RoleDM, false},
	{"set_viewpoint", identity.RoleAgent, false},
	{"set_viewpoint", identity.RolePlayer, false},
	{"set_viewpoint", identity.RoleSpectator, true},
}

// ownershipFixture returns a State where actor "a1" is controlled by
// participant "p-1" and owns token "t1" — the shape the table-driven test's
// move_token/use_ability/remove_condition player cells need to legitimately
// come out true.
func ownershipFixture() *engine.State {
	st := engine.NewState()
	// Both fields, exactly as engine.Apply would leave them: the set is
	// authoritative and controller_id mirrors controller_ids[0]. Setting only
	// the scalar would build a state the fold cannot produce, and would make
	// this fixture disagree with production the moment authz reads the set.
	// KIND IS DECLARED, and it has to be: an absent kind is not a party member,
	// always (spec §5.1, 2026-08-24), and set_viewpoint's spectator cell asks
	// exactly that question. Leaving it silent would make this fixture disagree
	// with production in the one direction the matrix cannot see — a cell that
	// fails for a reason unrelated to authorization.
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		Kind:          vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		ControllerId:  "p-1",
		ControllerIds: []string{"p-1"},
	}
	// X/Y spelled out at (0,0) rather than left implicit: this position also
	// has to be adjacent to the door commandFor's open_door/close_door target
	// ((0,1) in this same scene, "scn") for the open_door/close_door player
	// cells in the matrix test to hold — see authzCases' own comment.
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1", X: 0, Y: 0}
	return st
}

func TestAuthorizeTableAllCommandsAllRoles(t *testing.T) {
	if len(authzCases) != 88 {
		t.Fatalf("authzCases has %d entries, want 88 (22 commands x 4 roles)", len(authzCases))
	}
	st := ownershipFixture()
	for _, tc := range authzCases {
		t.Run(tc.command+"/"+string(tc.role), func(t *testing.T) {
			p := &identity.Participant{ID: "p-1", Role: tc.role}
			cmd := commandFor(t, tc.command)
			err := gateway.Authorize(p, cmd, st)
			got := err == nil
			if got != tc.want {
				t.Fatalf("Authorize(role=%s, cmd=%s) allowed=%v, want %v (err=%v)",
					tc.role, tc.command, got, tc.want, err)
			}
		})
	}
}

func TestAuthorizePlayerOwnTokenOK(t *testing.T) {
	st := ownershipFixture()
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, moveTokenCmd("t1"), st); err != nil {
		t.Fatalf("want nil error moving own-controlled token, got %v", err)
	}
}

func TestAuthorizePlayerOtherTokenDenied(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		// The SET too, not the scalar alone. controls() reads only the set,
		// so a scalar-only fixture is denied for being UNOWNED — which makes
		// this a duplicate of the controllerless test and silently unpins
		// "another participant controls it".
		ControllerId:  "someone-else",
		ControllerIds: []string{"someone-else"},
	}
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1"}
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, moveTokenCmd("t1"), st); err == nil {
		t.Fatal("want error moving a token controlled by another participant")
	}
}

func TestAuthorizePlayerControllerlessActorTokenDenied(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{ActorId: "a1", Name: "Hero"} // ControllerId empty = DM/agent only
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1"}
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, moveTokenCmd("t1"), st); err == nil {
		t.Fatal("want error moving a controllerless actor's token as a player")
	}
}

func TestAuthorizePlayerUnknownTokenDenied(t *testing.T) {
	st := engine.NewState()
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, moveTokenCmd("no-such-token"), st); err == nil {
		t.Fatal("want error moving an unknown token")
	}
}

// --- open_door / close_door adjacency (maps-as-geometry Task 6) -----------
//
// Same shape as the ownership tests above: the matrix cell shows only the
// permissive direction (a token happens to be adjacent), so these prove the
// other direction independently, plus the DM/agent bypass "hard for
// players, free for DM" is built on (Patrik, spec §6).

// doorFixture returns a State where actor "a1"/token "t1" belongs to
// participant "p-1", positioned at (x, y) in scene "scn" — parameterized so
// the adjacent and distant cases below share one shape while proving
// opposite outcomes.
func doorFixture(x, y int32) *engine.State {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		Kind:          vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		ControllerId:  "p-1",
		ControllerIds: []string{"p-1"},
	}
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1", X: x, Y: y}
	return st
}

func TestAuthorizePlayerMayWorkAdjacentDoor(t *testing.T) {
	st := doorFixture(0, 0) // door at (0,1): dx=0, dy=1 — adjacent
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, openDoorCmd("scn", 0, 1), st); err != nil {
		t.Fatalf("want nil error opening a door a controlled token stands next to: %v", err)
	}
	if err := gateway.Authorize(p, closeDoorCmd("scn", 0, 1), st); err != nil {
		t.Fatalf("want nil error closing a door a controlled token stands next to: %v", err)
	}
}

// TestAuthorizePlayerMayNotWorkDistantDoor is the guard direction: without
// it, mayWorkDoor could always return nil for a player and every open_door/
// close_door player cell in the matrix would pass for the wrong reason.
func TestAuthorizePlayerMayNotWorkDistantDoor(t *testing.T) {
	st := doorFixture(5, 5) // nowhere near the door at (0,1)
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, openDoorCmd("scn", 0, 1), st); err == nil {
		t.Fatal("want error opening a door with no controlled token nearby")
	}
	if err := gateway.Authorize(p, closeDoorCmd("scn", 0, 1), st); err == nil {
		t.Fatal("want error closing a door with no controlled token nearby")
	}
}

// TestAuthorizePlayerMayWorkAdjacentDoorAwayFromTheOrigin is the adjacent case
// with BOTH coordinates non-zero, and it exists because the fixture above
// cannot tell a subtraction from an addition: its token is at (0,0), so
// `tok.X - at.X` and `tok.X + at.X` are both 0 and both read as adjacent. Every
// adjacency assertion in this file sat on the one pair of coordinates where the
// operator does not matter.
//
// Kills ARITHMETIC_BASE and INVERT_NEGATIVES at authz.go:152:15. Here the token
// is at (3,3) and the door at (2,3): the true delta is 1 (adjacent), while the
// mutated sum is 5 — far enough to refuse a door the player is standing right
// beside.
func TestAuthorizePlayerMayWorkAdjacentDoorAwayFromTheOrigin(t *testing.T) {
	st := doorFixture(3, 3) // door at (2,3): dx=1, dy=0 — adjacent
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, openDoorCmd("scn", 2, 3), st); err != nil {
		t.Fatalf("want nil error opening a door one square west of a controlled token: %v", err)
	}
	if err := gateway.Authorize(p, closeDoorCmd("scn", 2, 3), st); err != nil {
		t.Fatalf("want nil error closing a door one square west of a controlled token: %v", err)
	}
}

// TestAuthorizePlayerMayNotWorkADoorItStandsWestOrNorthOf is the refusal
// direction from the OTHER SIDE. TestAuthorizePlayerMayNotWorkDistantDoor puts
// its token at (5,5) and the door at (0,1), so both deltas are positive and
// abs() never has to do anything — which is why the whole suite could not tell
// `return -n` from `return n`.
//
// Kills ARITHMETIC_BASE and INVERT_NEGATIVES at authz.go:164:10. With abs()
// returning its argument unchanged, a negative delta of any size satisfies
// `<= 1`, so a player could work a door from any distance as long as it lay to
// the west or north of their token. Both axes are checked because the two
// comparisons in mayWorkDoor are separate expressions.
func TestAuthorizePlayerMayNotWorkADoorItStandsWestOrNorthOf(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	for _, c := range []struct {
		name         string
		tokX, tokY   int32
		doorX, doorY int32
	}{
		{"door four squares east", 1, 1, 5, 1},  // dx = -4
		{"door four squares south", 1, 1, 1, 5}, // dy = -4
	} {
		t.Run(c.name, func(t *testing.T) {
			st := doorFixture(c.tokX, c.tokY)
			if err := gateway.Authorize(p, openDoorCmd("scn", c.doorX, c.doorY), st); err == nil {
				t.Fatalf("opening a door at (%d,%d) from (%d,%d) was allowed — a negative "+
					"delta must be measured as distance, not passed through",
					c.doorX, c.doorY, c.tokX, c.tokY)
			}
			if err := gateway.Authorize(p, closeDoorCmd("scn", c.doorX, c.doorY), st); err == nil {
				t.Fatalf("closing a door at (%d,%d) from (%d,%d) was allowed",
					c.doorX, c.doorY, c.tokX, c.tokY)
			}
		})
	}
}

// TestAuthorizePlayerDoorAdjacencyIgnoresOtherScenes closes the gap between
// "position happens to be adjacent" and "adjacent in the right scene": a
// token that is numerically close but in a DIFFERENT scene must not count,
// or mayWorkDoor's scene filter is dead code no test would catch removing.
func TestAuthorizePlayerDoorAdjacencyIgnoresOtherScenes(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		ControllerId:  "p-1",
		ControllerIds: []string{"p-1"},
	}
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "a-different-scene", ActorID: "a1", X: 0, Y: 0}
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, openDoorCmd("scn", 0, 1), st); err == nil {
		t.Fatal("want error: the adjacent token is in a different scene than the door")
	}
}

// TestAuthorizeDMMayWorkDoorRegardlessOfTokenPosition is the mechanical form
// of "free for DM": mayWorkDoor returns nil for a non-player role before it
// ever inspects token position, so a token nowhere near the door still lets
// the DM and agent through.
func TestAuthorizeDMMayWorkDoorRegardlessOfTokenPosition(t *testing.T) {
	st := doorFixture(5, 5) // far from the door at (0,1) — irrelevant for dm/agent
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := &identity.Participant{ID: "someone-else", Role: role}
		if err := gateway.Authorize(p, openDoorCmd("scn", 0, 1), st); err != nil {
			t.Fatalf("%s must be free of the adjacency rule (Patrik: \"hard for players, "+
				"free for DM\"): %v", role, err)
		}
		if err := gateway.Authorize(p, closeDoorCmd("scn", 0, 1), st); err != nil {
			t.Fatalf("%s must be free of the adjacency rule: %v", role, err)
		}
	}
}

// --- set_viewpoint: which shoulders exist (visibility Task 6) -------------

// TestAuthorizeSpectatorMayNotPerchOnAnNpc is the direction the matrix cell
// cannot prove. Its spectator cell is true because commandFor names an actor
// that reads as a party member; this asks the same question about the Goblin
// Archer and requires Authorize itself — not a menu, not a client — to say no.
func TestAuthorizeSpectatorMayNotPerchOnAnNpc(t *testing.T) {
	st := ownershipFixture()
	// An actor with an EMPTY control set: DM/agent only, which is what "NPC"
	// means everywhere else in this file (see authorizeTokenOwnership).
	st.Actors["act-goblin-archer"] = &vttv1.Actor{ActorId: "act-goblin-archer", Name: "Goblin Archer"}

	p := &identity.Participant{ID: "s-1", Role: identity.RoleSpectator}
	if err := gateway.Authorize(p, setViewpointCmd("act-goblin-archer"), st); err == nil {
		t.Fatal("a spectator perched on the Goblin Archer would watch the ambush from " +
			"inside it: Authorize must refuse the perch")
	}
	// The control: the same spectator, the same command, an actor a player
	// controls. Without this the test above would pass against an Authorize
	// that refused every perch there is.
	if err := gateway.Authorize(p, setViewpointCmd("a1"), st); err != nil {
		t.Fatalf("a spectator may ride a party member's shoulder: %v", err)
	}
}

// --- use_ability / remove_condition actor ownership (Task 6) --------------
// Same shape as the move_token ownership tests above, checked against
// Actor.controller_id directly (no token indirection) — see
// authorizeActorOwnership's doc comment.

func TestAuthorizePlayerUseAbilityOwnActorOK(t *testing.T) {
	st := ownershipFixture()
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, useAbilityCmd("a1"), st); err != nil {
		t.Fatalf("want nil error using an ability as one's own controlled actor, got %v", err)
	}
}

func TestAuthorizePlayerUseAbilityOtherActorDenied(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		// The SET too, not the scalar alone. controls() reads only the set,
		// so a scalar-only fixture is denied for being UNOWNED — which makes
		// this a duplicate of the controllerless test and silently unpins
		// "another participant controls it".
		ControllerId:  "someone-else",
		ControllerIds: []string{"someone-else"},
	}
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, useAbilityCmd("a1"), st); err == nil {
		t.Fatal("want error using an ability as an actor controlled by another participant")
	}
}

func TestAuthorizePlayerUseAbilityControllerlessActorDenied(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{ActorId: "a1", Name: "Hero"} // ControllerId empty = DM/agent only
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, useAbilityCmd("a1"), st); err == nil {
		t.Fatal("want error using an ability as a controllerless actor")
	}
}

func TestAuthorizePlayerUseAbilityUnknownActorDenied(t *testing.T) {
	st := engine.NewState()
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, useAbilityCmd("no-such-actor"), st); err == nil {
		t.Fatal("want error using an ability as an unknown actor")
	}
}

func TestAuthorizePlayerRemoveConditionOwnActorOK(t *testing.T) {
	st := ownershipFixture()
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, removeConditionCmd("a1"), st); err != nil {
		t.Fatalf("want nil error removing a condition from one's own controlled actor, got %v", err)
	}
}

func TestAuthorizePlayerRemoveConditionOtherActorDenied(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		// The SET too, not the scalar alone. controls() reads only the set,
		// so a scalar-only fixture is denied for being UNOWNED — which makes
		// this a duplicate of the controllerless test and silently unpins
		// "another participant controls it".
		ControllerId:  "someone-else",
		ControllerIds: []string{"someone-else"},
	}
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, removeConditionCmd("a1"), st); err == nil {
		t.Fatal("want error removing a condition from an actor controlled by another participant")
	}
}

// TestAuthorizeUnknownCommandDeniedForEveryRole is the default-deny case:
// an empty/unset ClientCommand oneof is not in commandRoles at all, so it
// must be denied regardless of role.
func TestAuthorizeUnknownCommandDeniedForEveryRole(t *testing.T) {
	st := engine.NewState()
	roles := []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			p := &identity.Participant{ID: "p-1", Role: role}
			if err := gateway.Authorize(p, &vttv1.ClientCommand{}, st); err == nil {
				t.Fatalf("want error for empty command, role %s", role)
			}
		})
	}
}

// --- Task 3: control is a SET -------------------------------------------
//
// The tests below are the ones that would have passed for the wrong reason
// under the old scalar check. Each names a state only a SET can express.

// TestAuthorizeSecondControllerMayMoveTheToken is the point of the whole
// change. Under the scalar rule this participant was denied, because
// controller_id can only ever hold ONE of the two.
func TestAuthorizeSecondControllerMayMoveTheToken(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		ControllerId:  "p-1", // mirrors controller_ids[0]
		ControllerIds: []string{"p-1", "p-2"},
	}
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1"}

	for _, id := range []string{"p-1", "p-2"} {
		p := &identity.Participant{ID: id, Role: identity.RolePlayer}
		if err := gateway.Authorize(p, moveTokenCmd("t1"), st); err != nil {
			t.Fatalf("participant %q is in controller_ids and must be allowed: %v", id, err)
		}
	}
}

// TestAuthorizeNonControllerStillDeniedWhenActorIsShared guards the direction
// that matters more: widening to a set must not widen to EVERYONE.
func TestAuthorizeNonControllerStillDeniedWhenActorIsShared(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		ControllerId:  "p-1",
		ControllerIds: []string{"p-1", "p-2"},
	}
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1"}

	p := &identity.Participant{ID: "p-3", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, moveTokenCmd("t1"), st); err == nil {
		t.Fatal("a participant absent from controller_ids must be denied")
	}
}

// TestAuthorizeDMMayActOnAnActorAPlayerControls pins the property Patrik
// called out as non-negotiable: the DM can always grab a character token,
// even while a player controls it, WITHOUT revoking that player's control.
//
// It holds because ownership is consulted for RolePlayer only. That is easy
// to break by "tidying" the ownership check to run for every role, which
// reads like a tightening and would silently lock the DM out of every
// assigned character mid-session.
func TestAuthorizeDMMayActOnAnActorAPlayerControls(t *testing.T) {
	st := ownershipFixture() // a1 controlled by p-1
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := &identity.Participant{ID: "someone-else", Role: role}
		if err := gateway.Authorize(p, moveTokenCmd("t1"), st); err != nil {
			t.Fatalf("%s must be able to act on a player-controlled actor: %v", role, err)
		}
		if err := gateway.Authorize(p, useAbilityCmd("a1"), st); err != nil {
			t.Fatalf("%s must be able to act AS a player-controlled actor: %v", role, err)
		}
	}
	// And the player keeps control throughout — grabbing is not a transfer.
	if ids := st.Actors["a1"].GetControllerIds(); len(ids) != 1 || ids[0] != "p-1" {
		t.Fatalf("controller_ids = %v, want [p-1] untouched by the DM acting", ids)
	}
}

// TestAuthorizePlayerRevokeSelfOK: a player may put a character down.
func TestAuthorizePlayerRevokeSelfOK(t *testing.T) {
	st := ownershipFixture()
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, revokeActorControlCmd("p-1"), st); err != nil {
		t.Fatalf("a player naming THEMSELVES must be allowed to revoke: %v", err)
	}
}

// TestAuthorizePlayerRevokeOtherParticipantDenied is the other half, and the
// half that makes the player row a guard rather than an opening: a player may
// not take a character from someone else.
func TestAuthorizePlayerRevokeOtherParticipantDenied(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		ControllerId:  "p-1",
		ControllerIds: []string{"p-1", "p-2"},
	}
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, revokeActorControlCmd("p-2"), st); err == nil {
		t.Fatal("a player naming ANOTHER participant must be denied — that is taking, not putting down")
	}
}

// TestAuthorizePlayerRevokeSelfOnAnActorTheyDoNotControlDenied closes the gap
// between "names themselves" and "has anything to give up". Without it, the
// self-check alone would let any player issue revoke against any actor.
func TestAuthorizePlayerRevokeSelfOnAnActorTheyDoNotControlDenied(t *testing.T) {
	st := ownershipFixture() // a1 controlled by p-1, NOT p-9
	p := &identity.Participant{ID: "p-9", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, revokeActorControlCmd("p-9"), st); err == nil {
		t.Fatal("a player must not revoke control they never held")
	}
}

// TestAuthorizeEmptyParticipantMatchesNothing pins the guard in controls().
//
// Defence in depth, deliberately: T2's fold filters empty ids out of
// controller_ids, so this state should be unreachable through engine.Apply.
// It is built DIRECTLY here because that is the point — if an empty id ever
// reached state by a route the fold does not own, an empty participant id
// matching it would hand a stranger every such actor at the table. The two
// guards are independent, and this one is authz's own.
func TestAuthorizeEmptyParticipantMatchesNothing(t *testing.T) {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{
		ActorId: "a1", Name: "Hero",
		ControllerIds: []string{""}, // only the fold's failure could produce this
	}
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1"}

	p := &identity.Participant{ID: "", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, moveTokenCmd("t1"), st); err == nil {
		t.Fatal("an empty participant id must never match an empty entry in controller_ids")
	}
}

// TestAuthorizeDMMayRevokeAnotherParticipantsControl is the half of §3.2 that
// the ownership tests cannot reach: grant/revoke route through
// authorizeSelfRevoke, not through the ownership helpers.
//
// The motivating case from spec §1 — a player leaves and the DM reclaims their
// character — needs the DM to name SOMEONE ELSE on an actor the DM does not
// control. Both those things are exactly what the player self-check forbids,
// so the DM's bypass of that check is load-bearing, and until this test existed
// it could be removed with the whole suite still green: every role in the
// matrix test runs as "p-1", so the DM and agent revoke cells passed the
// self-check VACUOUSLY rather than by bypassing it.
func TestAuthorizeDMMayRevokeAnotherParticipantsControl(t *testing.T) {
	st := ownershipFixture() // a1 controlled by p-1
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := &identity.Participant{ID: "someone-else", Role: role}
		if err := gateway.Authorize(p, revokeActorControlCmd("p-1"), st); err != nil {
			t.Fatalf("%s must be able to revoke ANOTHER participant's control: %v", role, err)
		}
		if err := gateway.Authorize(p, grantActorControlCmd("p-2"), st); err != nil {
			t.Fatalf("%s must be able to grant control of a player-held actor: %v", role, err)
		}
	}
}

// --- promotion may not reach dm or agent (spec §3.1a) -----------------------

func TestPromotionMayOnlyTargetPlayerOrSpectator(t *testing.T) {
	// The escalation path this guard exists to close: a shared join link mints
	// SPECTATORS, so if promotion could reach dm or agent, that link would be
	// a route to full authority in two steps — exactly what admitting people
	// as spectators is for. Minting a DM stays with `vtt invite`, out of band.
	st := ownershipFixture()
	dm := &identity.Participant{ID: "p-dm", Role: identity.RoleDM}

	for _, role := range []string{"player", "spectator"} {
		if err := gateway.Authorize(dm, promoteCmd("p-2", role), st); err != nil {
			t.Fatalf("a DM must be able to promote to %q: %v", role, err)
		}
	}
	for _, role := range []string{"dm", "agent"} {
		if err := gateway.Authorize(dm, promoteCmd("p-2", role), st); err == nil {
			t.Fatalf("promotion to %q must be refused even for a DM — the join link would "+
				"otherwise reach full authority in two steps", role)
		}
	}
	// And a role that is not a role at all.
	if err := gateway.Authorize(dm, promoteCmd("p-2", "superuser"), st); err == nil {
		t.Fatal("an unknown role must be refused")
	}
}

func TestASpectatorCannotPromoteItself(t *testing.T) {
	// NOT the same test as "a spectator cannot promote". The participant id in
	// the COMMAND and the id on the CONNECTION are different fields, and
	// confusing them is how revoke_actor_control's self-check nearly shipped
	// unpinned. Self-promotion is the case the spectator default exists to
	// prevent, so it gets its own assertion.
	st := ownershipFixture()
	me := &identity.Participant{ID: "p-self", Role: identity.RoleSpectator}
	if err := gateway.Authorize(me, promoteCmd("p-self", "player"), st); err == nil {
		t.Fatal("a spectator promoting ITSELF must be refused — anyone through the shared " +
			"link could otherwise make themselves a player")
	}
}

// TestEveryClientCommandHasRoleCells is the authz twin of
// TestEveryClientCommandConverts, and it exists because commandName and
// commandRoles are both HAND-WRITTEN LISTS over a oneof that grows.
//
// The failure mode is quieter than a missing conversion arm and so easier to
// ship: an unlisted command resolves to the empty name, misses commandRoles,
// and is denied for EVERY role. Fail-closed, which is the right direction —
// but the command is then advertised on the wire and through MCP while being
// impossible for anyone to issue, and the only symptom is a DM being told they
// "may not issue \"\"".
//
// The 72-cell table beside this cannot catch it: its count is a literal, so a
// command added without cells leaves the count untouched and every existing
// case still passes. Reflection over the oneof is what makes forgetting
// impossible rather than merely unlikely.
func TestEveryClientCommandHasRoleCells(t *testing.T) {
	oneof := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor().Oneofs().ByName("command")
	if oneof == nil {
		t.Fatal("vttv1.ClientCommand has no \"command\" oneof")
	}
	for i := range oneof.Fields().Len() {
		name := string(oneof.Fields().Get(i).Name())
		t.Run(name, func(t *testing.T) {
			// The name must round-trip through commandName, or authorization
			// is deciding about a command it cannot identify.
			if got := gateway.CommandNameForTest(commandFor(t, name)); got != name {
				t.Fatalf("commandName resolved %q to %q — an unlisted arm is denied for "+
					"every role, so this command is advertised and unusable", name, got)
			}
			if !gateway.HasRoleCellsForTest(name) {
				t.Fatalf("%q has no row in commandRoles, so no role may issue it", name)
			}
		})
	}
}
