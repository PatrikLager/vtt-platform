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
	if name == "move_token" && p.Role == identity.RolePlayer {
		return authorizeTokenOwnership(p, cmd.GetMoveToken(), st)
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
	default:
		return ""
	}
}
