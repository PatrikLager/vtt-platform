// Package gateway is the platform's pure authorization/conversion/codec
// core over vtt.v1 commands and events (spec §4). It imports engine.State
// only to answer the player-ownership question and never mutates it; it
// does no I/O — Task 5 wires this core to a real WebSocket server.
package gateway

import (
	"errors"
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// commandRoles is THE authorization policy (spec §4). Player MoveToken has an
// additional ownership check in Authorize; everything not listed is denied.
var commandRoles = map[string]map[identity.Role]bool{
	"move_token":     {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"create_scene":   {identity.RoleDM: true, identity.RoleAgent: true},
	"add_actor":      {identity.RoleDM: true, identity.RoleAgent: true},
	"place_token":    {identity.RoleDM: true, identity.RoleAgent: true},
	"start_session":  {identity.RoleDM: true, identity.RoleAgent: true},
	"end_session":    {identity.RoleDM: true, identity.RoleAgent: true},
	"retract_events": {identity.RoleDM: true, identity.RoleAgent: true},
	// use_ability/remove_condition (ruleset-interpreter Task 6): dm/agent
	// may target any actor; a player may only act as an actor THEY
	// control — the additional ownership check below, on the command's own
	// actor_id field, mirrors move_token's token-ownership check exactly
	// (same shape, different field: the ACTOR being acted as, not a token
	// being moved).
	"use_ability":      {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"remove_condition": {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	// add_narration/upsert_note/delete_note (world-layer Task 3, spec §5):
	// everyone at the table narrates or speaks (dm/agent/player), spectators
	// stay read-only — the SAME role set as move_token/use_ability, but with
	// NO additional ownership check (narration/notes are not scoped to an
	// actor a participant controls). upsert_note/delete_note are dm/agent
	// only: world facts are the DM's (spec §5 — "revisit if players ever
	// co-author").
	"add_narration": {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"upsert_note":   {identity.RoleDM: true, identity.RoleAgent: true},
	"delete_note":   {identity.RoleDM: true, identity.RoleAgent: true},
	// load_adventure (adventure-format Task 4, spec §7): "the DM calls
	// load_adventure when the table is ready" — dm/agent only, same shape
	// as create_scene/add_actor/place_token, no additional ownership check
	// (an adventure load is not scoped to any actor a participant
	// controls).
	"load_adventure": {identity.RoleDM: true, identity.RoleAgent: true},
}

// ErrUnauthorized is wrapped by every denial Authorize returns.
var ErrUnauthorized = errors.New("gateway: not authorized")

// Authorize is the ONE authorization function (spec §4): a table lookup
// keyed by the ClientCommand oneof field name and the participant's role,
// plus one additional check for players moving tokens — the token's actor
// must be controlled by this participant. Anything not present in
// commandRoles (including an unset/unknown oneof) is denied by default.
func Authorize(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error {
	name := commandName(cmd)
	roles, known := commandRoles[name]
	if !known || !roles[p.Role] {
		return fmt.Errorf("%w: role %q may not issue %q", ErrUnauthorized, p.Role, name)
	}
	if p.Role != identity.RolePlayer {
		return nil
	}
	switch name {
	case "move_token":
		return authorizeTokenOwnership(p, cmd.GetMoveToken(), st)
	case "use_ability":
		return authorizeActorOwnership(p, cmd.GetUseAbility().GetActorId(), st)
	case "remove_condition":
		return authorizeActorOwnership(p, cmd.GetRemoveCondition().GetActorId(), st)
	}
	return nil
}

// authorizeTokenOwnership enforces the player-only ownership rule: the
// token must exist, its actor must exist, and that actor's ControllerId
// must equal p.ID. A controllerless actor (empty ControllerId) is DM/agent
// only (spec: Actor.controller_id doc comment) and so is denied to players.
func authorizeTokenOwnership(p *identity.Participant, req *vttv1.MoveTokenRequest, st *engine.State) error {
	tok, ok := st.Tokens[req.GetTokenId()]
	if !ok {
		return fmt.Errorf("%w: unknown token %q", ErrUnauthorized, req.GetTokenId())
	}
	actor, ok := st.Actors[tok.ActorID]
	if !ok || actor.GetControllerId() == "" || actor.GetControllerId() != p.ID {
		return fmt.Errorf("%w: token %q is not controlled by participant %q", ErrUnauthorized, req.GetTokenId(), p.ID)
	}
	return nil
}

// authorizeActorOwnership enforces the player-only ownership rule for
// use_ability and remove_condition: the actor named by the command's own
// actor_id must exist and be controlled by this participant. This is the
// SAME shape as authorizeTokenOwnership above, just checked directly
// against Actor.controller_id instead of resolving through a token first —
// use_ability/remove_condition name their acting actor directly, with no
// token indirection (a controllerless actor, empty ControllerId, is
// DM/agent only, exactly as move_token's ownership check treats it).
func authorizeActorOwnership(p *identity.Participant, actorID string, st *engine.State) error {
	actor, ok := st.Actors[actorID]
	if !ok || actor.GetControllerId() == "" || actor.GetControllerId() != p.ID {
		return fmt.Errorf("%w: actor %q is not controlled by participant %q", ErrUnauthorized, actorID, p.ID)
	}
	return nil
}

// commandName returns the oneof field name for cmd's set command, matching
// the proto field names used as commandRoles keys ("" for unset/unknown).
func commandName(cmd *vttv1.ClientCommand) string {
	switch cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_MoveToken:
		return "move_token"
	case *vttv1.ClientCommand_CreateScene:
		return "create_scene"
	case *vttv1.ClientCommand_AddActor:
		return "add_actor"
	case *vttv1.ClientCommand_PlaceToken:
		return "place_token"
	case *vttv1.ClientCommand_StartSession:
		return "start_session"
	case *vttv1.ClientCommand_EndSession:
		return "end_session"
	case *vttv1.ClientCommand_RetractEvents:
		return "retract_events"
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
	default:
		return ""
	}
}
