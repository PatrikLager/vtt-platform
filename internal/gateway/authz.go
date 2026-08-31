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
	"move_token":    {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"create_scene":  {identity.RoleDM: true, identity.RoleAgent: true},
	"add_actor":     {identity.RoleDM: true, identity.RoleAgent: true},
	"place_token":   {identity.RoleDM: true, identity.RoleAgent: true},
	"start_session": {identity.RoleDM: true, identity.RoleAgent: true},
	"end_session":   {identity.RoleDM: true, identity.RoleAgent: true},
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
	// load_map (whole-branch-review C1 remediation): dm/agent only, exactly
	// the same shape and reasoning as load_adventure directly above —
	// loading a standalone map rewrites the table's world just as loading
	// an adventure does, and no additional ownership check applies (a map
	// load is not scoped to any actor a participant controls).
	"load_map": {identity.RoleDM: true, identity.RoleAgent: true},
	// grant/revoke_actor_control (presence-and-actor-control Task 3, spec
	// §5.3). Handing a character to someone is the DM's call, so grant has
	// no player row at all. Revoke does: a player may put a character DOWN,
	// naming ONLY themselves — the additional self-check in Authorize, the
	// same shape as the ownership helpers. Neither is gated by ownership
	// for dm/agent, which is what keeps §3.2 true: the DM can take an actor
	// a player is holding without first revoking them.
	"grant_actor_control":  {identity.RoleDM: true, identity.RoleAgent: true},
	"revoke_actor_control": {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	// promote_participant (joining-a-table spec §3.1a). DM and agent only, and
	// NO player row: a shared join link mints spectators, so a player able to
	// promote would make that link a route to authority in two steps.
	"promote_participant": {identity.RoleDM: true, identity.RoleAgent: true},
	// The shared join door (joining-a-table spec §5). DM/agent only by the
	// same argument that gates grant_actor_control: an open door MINTS
	// PARTICIPANTS, and rotation is the only way to close a leaked link.
	"set_join_door":    {identity.RoleDM: true, identity.RoleAgent: true},
	"rotate_join_link": {identity.RoleDM: true, identity.RoleAgent: true},
	// open_door/close_door (maps-as-geometry Task 1 fix, spec §6: "hard for
	// players, free for DM"). Same role set as move_token: dm/agent/player
	// may all issue it, spectator may not. The additional adjacency check —
	// "a player may work a door only if a token they control is adjacent to
	// it" — is mayWorkDoor, below, wired into Authorize's switch (Task 6).
	"open_door":  {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"close_door": {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	// set_viewpoint (visibility spec §3.1.1). SPECTATOR ONLY, and it is the
	// only row in this table shaped that way: a perch is how a watcher with no
	// character of their own borrows someone else's eyes. "An unassigned
	// PLAYER does not perch" — their answer to an empty board is to be GIVEN a
	// character — and the DM and the agent see everything already, so there is
	// no shoulder for them to gain.
	//
	// The row alone is not the whole rule. A perch may only target a PARTY
	// MEMBER (visibility spec §5.1 — the actor's own kind, not whoever holds
	// it), which Authorize asks MayPerch below, in the section that runs for
	// every role; the handler is serve's handleSetViewpoint, and it appends
	// nothing.
	"set_viewpoint": {identity.RoleSpectator: true},
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
	// Checked for EVERY role, before the player-only ownership rules below:
	// this bounds what a promotion may DO, not who may issue it, so a DM is
	// subject to it too (spec §3.1a).
	if name == "promote_participant" {
		if err := authorizePromotionTarget(cmd.GetPromoteParticipant()); err != nil {
			return err
		}
	}
	// set_viewpoint's own additional check, the same shape as the promotion
	// target above and for the same reason: it bounds what the command may
	// NAME, not who may issue it. A perch may only target a PARTY MEMBER
	// (visibility spec §3.1.1, as §5.1 amended it) — see MayPerch, which is
	// where that rule and its argument live.
	//
	// Above the player-only section deliberately. Every other additional check
	// in this function is a rule about players; this one is a rule about
	// spectators, and the switch below is unreachable for them.
	if name == "set_viewpoint" {
		if err := MayPerch(p, cmd.GetSetViewpoint().GetActorId(), st); err != nil {
			return err
		}
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
	case "revoke_actor_control":
		return authorizeSelfRevoke(p, cmd.GetRevokeActorControl(), st)
	case "open_door":
		od := cmd.GetOpenDoor()
		return mayWorkDoor(p, st, od.GetSceneId(), od.GetAt())
	case "close_door":
		cd := cmd.GetCloseDoor()
		return mayWorkDoor(p, st, cd.GetSceneId(), cd.GetAt())
	}
	return nil
}

// mayWorkDoor enforces the player-only adjacency rule for open_door/
// close_door (maps-as-geometry Task 6, spec §6: "hard for players, free for
// DM"). A non-player role returns nil immediately — the DM and the agent
// author the world and are free of it, the same bypass move_token's
// ownership check never applies to them either.
//
// SPATIAL ONLY, deliberately (CLAUDE.md rule 5): this asks WHERE a
// participant's tokens are, never what edition or ruleset is in play — no
// reach, no movement cost, nothing a rule module would own. Adjacency uses
// Chebyshev distance (max of the two axis deltas <= 1), matching a
// standard 8-neighbour grid: the four orthogonal squares and the four
// diagonals all count as "next to".
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

// abs returns the absolute value of a grid-coordinate delta. Small enough
// that pulling in a generic math package for one int32 subtraction is not
// worth it; mayWorkDoor is its only caller.
func abs(n int32) int32 {
	if n < 0 {
		return -n
	}
	return n
}

// authorizeTokenOwnership enforces the player-only ownership rule: the token
// must exist, its actor must exist, and p.ID must be a MEMBER of that actor's
// ControllerIds. An actor with an EMPTY control set is DM/agent only (spec
// §5.3) and so is denied to players.
//
// Membership, not equality with the mirror: controller_id holds only
// controller_ids[0], so reading it here would deny every controller of a
// SHARED actor except the first.
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

// authorizeActorOwnership enforces the player-only ownership rule for
// use_ability, remove_condition and a player's own revoke_actor_control: the
// actor named by the command must exist and count this participant among its
// controllers. Same shape as authorizeTokenOwnership above, checked against
// the actor directly instead of resolving through a token first — these
// commands name their acting actor with no token indirection (an actor with
// an EMPTY control set is DM/agent only, exactly as move_token's ownership
// check treats it).
func authorizeActorOwnership(p *identity.Participant, actorID string, st *engine.State) error {
	actor, ok := st.Actors[actorID]
	if !ok || !controls(actor, p.ID) {
		return fmt.Errorf("%w: actor %q is not controlled by participant %q", ErrUnauthorized, actorID, p.ID)
	}
	return nil
}

// controls reports whether participantID is in actor's control set.
//
// controller_ids is the authority; controller_id is only its mirror, so
// reading the scalar here would see one of several controllers and deny the
// rest. An EMPTY set keeps the meaning the scalar's empty string had: nobody
// controls this actor, so it is DM/agent only.
//
// The empty participantID guard is not redundant with the fold's. This runs
// on a Participant from a verified invite, but an empty id must never match
// an empty entry should one ever reach state by a route the fold does not
// own — matching would hand a stranger every unowned actor at the table.
//
// It is also the LAST line of defence on the revoke path: authorizeSelfRevoke
// compares participant_id to p.ID, and for an empty participant that
// comparison is "" != "", which passes vacuously. This guard is then the only
// thing that denies.
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

// authorizeSelfRevoke enforces the player half of revoke_actor_control: a
// player may only give up control they THEMSELVES hold (spec §5.3).
//
// Two conditions, and both are load-bearing. Naming someone else is taking a
// character, not putting one down. Naming yourself on an actor you do not
// control is a no-op the fold would happily append, so it is refused here
// rather than written to a log that is append-only.
func authorizeSelfRevoke(p *identity.Participant, req *vttv1.RevokeActorControl, st *engine.State) error {
	if req.GetParticipantId() != p.ID {
		return fmt.Errorf("%w: player %q may only revoke their OWN control, not %q's",
			ErrUnauthorized, p.ID, req.GetParticipantId())
	}
	return authorizeActorOwnership(p, req.GetActorId(), st)
}

// authorizePromotionTarget bounds what a promotion may make someone.
//
// ONLY player or spectator (spec §3.1a). The shared join link mints
// spectators, so allowing dm or agent here would turn that link into a path to
// full authority in two steps — which is the thing admitting-as-spectator
// exists to prevent. Minting a DM stays with `vtt invite`, deliberately out of
// band.
//
// ParseRole would reject an unknown string anyway, but it would accept "dm";
// this is the narrower rule, and it is checked for every role including the
// DM's own.
func authorizePromotionTarget(req *vttv1.PromoteParticipant) error {
	switch req.GetRole() {
	case string(identity.RolePlayer), string(identity.RoleSpectator):
		return nil
	default:
		return fmt.Errorf("%w: a participant may be promoted only to player or spectator, not %q",
			ErrUnauthorized, req.GetRole())
	}
}

// commandName returns the oneof field name for cmd's set command, matching
// the proto field names used as commandRoles keys ("" for unset/unknown).
func commandName(cmd *vttv1.ClientCommand) string {
	switch cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_SetJoinDoor:
		return "set_join_door"
	case *vttv1.ClientCommand_RotateJoinLink:
		return "rotate_join_link"
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
