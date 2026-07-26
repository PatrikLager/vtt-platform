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
	case "create_scene":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_CreateScene{
			CreateScene: &vttv1.CreateScene{SceneId: "s1", Name: "Cave"},
		}}
	case "add_actor":
		return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{
			AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: "a2", Name: "Goblin"}},
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
	default:
		t.Fatalf("commandFor: unknown command name %q", name)
		return nil
	}
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

// authzCase is one cell of the 13 commands x 4 roles authorization matrix.
// want is written out LITERALLY per task-4-brief.md Step 1 — it must never
// be derived from commandRoles (the map under test) or this test proves
// nothing about the table's actual content.
type authzCase struct {
	command string
	role    identity.Role
	want    bool
}

// authzCases is the full 52-cell matrix (spec §4/§7, grown from 48 by
// adventure-format Task 4's load_adventure row, which itself grew from 36 by
// world-layer Task 3's add_narration/upsert_note/delete_note rows, and from
// 28 by ruleset-interpreter Task 6's use_ability/remove_condition rows):
// every command against every one of the four roles. move_token/player,
// use_ability/player, and remove_condition/player are all TRUE here because
// the shared fixture in TestAuthorizeTableAllCommandsAllRoles gives
// participant "p-1" ownership of actor "a1" (and its token "t1") — the
// table alone allows it, and the dedicated ownership tests below
// independently prove each additional check. add_narration/upsert_note/
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
}

// ownershipFixture returns a State where actor "a1" is controlled by
// participant "p-1" and owns token "t1" — the shape the table-driven test's
// move_token/use_ability/remove_condition player cells need to legitimately
// come out true.
func ownershipFixture() *engine.State {
	st := engine.NewState()
	st.Actors["a1"] = &vttv1.Actor{ActorId: "a1", Name: "Hero", ControllerId: "p-1"}
	st.Tokens["t1"] = engine.Token{ID: "t1", SceneID: "scn", ActorID: "a1"}
	return st
}

func TestAuthorizeTableAllCommandsAllRoles(t *testing.T) {
	if len(authzCases) != 52 {
		t.Fatalf("authzCases has %d entries, want 52 (13 commands x 4 roles)", len(authzCases))
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
	st.Actors["a1"] = &vttv1.Actor{ActorId: "a1", Name: "Hero", ControllerId: "someone-else"}
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
	st.Actors["a1"] = &vttv1.Actor{ActorId: "a1", Name: "Hero", ControllerId: "someone-else"}
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
	st.Actors["a1"] = &vttv1.Actor{ActorId: "a1", Name: "Hero", ControllerId: "someone-else"}
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
