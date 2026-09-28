// Package gateway is the WebSocket and HTTP gateway over vtt.v1 commands and
// events: authorization, conversion, the connection, the read surface and the
// seat's projection.
package gateway

import (
	"errors"
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Add a command's roles here and nowhere else: a command with no row is refused
// for every role (SPEC-013, TestEveryClientCommandHasRoleCells).
var commandRoles = map[string]map[identity.Role]bool{
	"move_token":  {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"add_actor":   {identity.RoleDM: true, identity.RoleAgent: true},
	"place_token": {identity.RoleDM: true, identity.RoleAgent: true},
	// Keep the player off both removals, even of what they control (SPEC-013).
	"remove_token":         {identity.RoleDM: true, identity.RoleAgent: true},
	"remove_actor":         {identity.RoleDM: true, identity.RoleAgent: true},
	"start_session":        {identity.RoleDM: true, identity.RoleAgent: true},
	"end_session":          {identity.RoleDM: true, identity.RoleAgent: true},
	"use_ability":          {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"remove_condition":     {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"add_narration":        {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"upsert_note":          {identity.RoleDM: true, identity.RoleAgent: true},
	"delete_note":          {identity.RoleDM: true, identity.RoleAgent: true},
	"load_adventure":       {identity.RoleDM: true, identity.RoleAgent: true},
	"load_map":             {identity.RoleDM: true, identity.RoleAgent: true},
	"grant_actor_control":  {identity.RoleDM: true, identity.RoleAgent: true},
	"revoke_actor_control": {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"promote_participant":  {identity.RoleDM: true, identity.RoleAgent: true},
	"set_join_door":        {identity.RoleDM: true, identity.RoleAgent: true},
	"rotate_join_link":     {identity.RoleDM: true, identity.RoleAgent: true},
	"open_door":            {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"close_door":           {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"set_viewpoint":        {identity.RoleSpectator: true},
}

// ErrUnauthorized is wrapped by every denial Authorize returns.
var ErrUnauthorized = errors.New("gateway: not authorized")

// Authorize decides whether p may issue cmd against st: the role table, the two
// checks for every role, then the player rules (SPEC-013).
func Authorize(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
	name := commandName(cmd)
	roles, known := commandRoles[name]
	if !known || !roles[p.Role] {
		return fmt.Errorf("%w: role %q may not issue %q", ErrUnauthorized, p.Role, name)
	}
	// Check this for every role, the DM included: it bounds what a promotion may
	// make someone (SPEC-009).
	if name == "promote_participant" {
		if err := authorizePromotionTarget(cmd.GetPromoteParticipant()); err != nil {
			return err
		}
	}
	// Check this for every role: a spectator never reaches the player half below
	// (SPEC-013).
	if name == "set_viewpoint" {
		if err := MayPerch(p, cmd.GetSetViewpoint().GetActorId(), st); err != nil {
			return err
		}
	}

	if p.Role != identity.RolePlayer {
		return nil
	}
	return authorizePlayer(p, cmd, st, name)
}

// Pass the name Authorize derived with cmd: a mismatched pair judges a command
// by another's rule (SPEC-013).
func authorizePlayer(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State, name string) error {
	rule, decided := playerRules[name]
	if !decided {
		// Refuse a player cell no rule decides (TestAPlayerCommandNobodyRuledOnIsRefused).
		return errUndecided(p, name)
	}
	return rule(p, cmd, st)
}

type playerRule func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error

// Keep this a table keyed like commandRoles, and write unrestricted rather than
// leave a name out: TestEveryPlayerCommandHasARule diffs the two.
var playerRules = map[string]playerRule{
	"move_token": func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
		return authorizeTokenOwnership(p, cmd.GetMoveToken(), st)
	},
	"use_ability": func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
		return authorizeActorOwnership(p, cmd.GetUseAbility().GetActorId(), st)
	},
	"remove_condition": func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
		return authorizeActorOwnership(p, cmd.GetRemoveCondition().GetActorId(), st)
	},
	"revoke_actor_control": func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
		return authorizeSelfRevoke(p, cmd.GetRevokeActorControl(), st)
	},
	"open_door": func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
		od := cmd.GetOpenDoor()
		return mayWorkDoor(p, st, od.GetSceneId(), od.GetAt())
	},
	"close_door": func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
		cd := cmd.GetCloseDoor()
		return mayWorkDoor(p, st, cd.GetSceneId(), cd.GetAt())
	},
	"add_narration": unrestricted,
}

func errUndecided(p *identity.Participant, name string) error {
	return fmt.Errorf("%w: role %q may issue %q and no player rule decides it",
		ErrUnauthorized, p.Role, name)
}

func unrestricted(*identity.Participant, *vttv1.ClientCommand, *engine.State) error { return nil }

// Keep this spatial: it asks where a controlled token stands, never reach,
// movement cost or a ruleset (SPEC-013).
func mayWorkDoor(p *identity.Participant, st *engine.State, sceneID string, at *vttv1.GridPosition) error {
	if p.Role != identity.RolePlayer {
		return nil
	}
	for _, tok := range st.Tokens {
		if tok.SceneID != sceneID {
			continue
		}
		actor, ok := st.Actors[tok.ActorID]
		if !ok || !controls(actor, p.ID) {
			continue
		}
		if abs(tok.X-at.GetX()) <= 1 && abs(tok.Y-at.GetY()) <= 1 {
			return nil
		}
	}
	return fmt.Errorf("%w: participant %q has no token adjacent to that door", ErrUnauthorized, p.ID)
}

func abs(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

func authorizeTokenOwnership(p *identity.Participant, req *vttv1.MoveTokenRequest, st *engine.State) error {
	tok, ok := st.Tokens[req.GetTokenId()]
	if !ok {
		return fmt.Errorf("%w: unknown token %q", ErrUnauthorized, req.GetTokenId())
	}
	actor, ok := st.Actors[tok.ActorID]
	if !ok || !controls(actor, p.ID) {
		return fmt.Errorf("%w: token %q is not controlled by participant %q", ErrUnauthorized, req.GetTokenId(), p.ID)
	}
	return nil
}

func authorizeActorOwnership(p *identity.Participant, actorID string, st *engine.State) error {
	actor, ok := st.Actors[actorID]
	if !ok || !controls(actor, p.ID) {
		return fmt.Errorf("%w: actor %q is not controlled by participant %q", ErrUnauthorized, actorID, p.ID)
	}
	return nil
}

// Read controller_ids, never the controller_id mirror: it holds one controller
// of several (SPEC-013).
//
// Keep the empty-id guard: on a self-revoke by an empty id the comparison passes
// and this is the only refusal left (SPEC-013,
// TestAuthorizeEmptyParticipantMatchesNothing).
func controls(actor *vttv1.Actor, participantID string) bool {
	if participantID == "" {
		return false
	}
	for _, id := range actor.GetControllerIds() {
		if id == participantID {
			return true
		}
	}
	return false
}

// Refuse a self-revoke of control never held here: the fold would append it as
// a change to nothing (SPEC-013).
func authorizeSelfRevoke(p *identity.Participant, req *vttv1.RevokeActorControl, st *engine.State) error {
	if req.GetParticipantId() != p.ID {
		return fmt.Errorf("%w: player %q may only revoke their OWN control, not %q's",
			ErrUnauthorized, p.ID, req.GetParticipantId())
	}
	return authorizeActorOwnership(p, req.GetActorId(), st)
}

// Keep this narrower than identity.ParseRole: it must refuse "dm" and "agent"
// (SPEC-009).
func authorizePromotionTarget(req *vttv1.PromoteParticipant) error {
	switch req.GetRole() {
	case string(identity.RolePlayer), string(identity.RoleSpectator):
		return nil
	default:
		return fmt.Errorf("%w: a participant may be promoted only to player or spectator, not %q",
			ErrUnauthorized, req.GetRole())
	}
}

// Keep each name the proto field's: commandRoles and playerRules are keyed by
// it (TestEveryClientCommandHasRoleCells).
func commandName(cmd *vttv1.ClientCommand) string {
	switch cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_SetJoinDoor:
		return "set_join_door"
	case *vttv1.ClientCommand_RotateJoinLink:
		return "rotate_join_link"
	case *vttv1.ClientCommand_MoveToken:
		return "move_token"
	case *vttv1.ClientCommand_AddActor:
		return "add_actor"
	case *vttv1.ClientCommand_PlaceToken:
		return "place_token"
	case *vttv1.ClientCommand_RemoveToken:
		return "remove_token"
	case *vttv1.ClientCommand_RemoveActor:
		return "remove_actor"
	case *vttv1.ClientCommand_StartSession:
		return "start_session"
	case *vttv1.ClientCommand_EndSession:
		return "end_session"
	case *vttv1.ClientCommand_UseAbility:
		return "use_ability"
	case *vttv1.ClientCommand_RemoveCondition:
		return "remove_condition"
	case *vttv1.ClientCommand_AddNarration:
		return "add_narration"
	case *vttv1.ClientCommand_UpsertNote:
		return "upsert_note"
	case *vttv1.ClientCommand_DeleteNote:
		return "delete_note"
	case *vttv1.ClientCommand_LoadAdventure:
		return "load_adventure"
	case *vttv1.ClientCommand_LoadMap:
		return "load_map"
	case *vttv1.ClientCommand_GrantActorControl:
		return "grant_actor_control"
	case *vttv1.ClientCommand_RevokeActorControl:
		return "revoke_actor_control"
	case *vttv1.ClientCommand_PromoteParticipant:
		return "promote_participant"
	case *vttv1.ClientCommand_OpenDoor:
		return "open_door"
	case *vttv1.ClientCommand_CloseDoor:
		return "close_door"
	case *vttv1.ClientCommand_SetViewpoint:
		return "set_viewpoint"
	default:
		return ""
	}
}
