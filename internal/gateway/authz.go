// Package gateway is the platform's authorization/conversion/codec core
// over vtt.v1 commands and events (spec §4). It imports engine.State only
// to answer the player-ownership question and never mutates it.
//
// This doc used to end "it does no I/O — Task 5 wires this core to a real
// WebSocket server", from when the package was a pure core with no server
// around it. Both halves have since stopped being true, and the second one
// first: the WebSocket server and the static bundle both live here now (so did
// raw pack-file serving, until 2026-09-02-art-is-a-flat-library Task 7 deleted
// it with the pack). As of 2026-09-01-create-scene-leaves Task 6 the package
// also READS one file — map.go's mapByID probes the campaign's maps/ when
// load_map names a map the set does not hold, because that plan's design
// spec §5 assigns the probe to the server on purpose. Authorization itself
// (this file) still does no I/O, which is the property that was worth
// stating and the one worth keeping.
package gateway

import (
	"errors"
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// commandRoles is THE authorization policy (spec §4): it says which roles may
// issue a command, and everything not listed here is denied. Six of the seven
// commands a player may issue carry an additional ownership check on top, in
// playerRules below.
var commandRoles = map[string]map[identity.Role]bool{
	"move_token":  {identity.RoleDM: true, identity.RoleAgent: true, identity.RolePlayer: true},
	"add_actor":   {identity.RoleDM: true, identity.RoleAgent: true},
	"place_token": {identity.RoleDM: true, identity.RoleAgent: true},
	// remove_token (retraction-leaves Task 8, spec §5.1: "takes a piece off
	// the board"). SAME ROLE SET AS place_token, deliberately — see
	// authz_test.go's authzCases comment on this row for the reasoning:
	// removal is place_token's inverse (authoring the board) and NOT
	// move_token's neighbour (using a piece already on it), so a player who
	// may move a token they control does not thereby get to remove it.
	"remove_token": {identity.RoleDM: true, identity.RoleAgent: true},
	// remove_actor (retraction-leaves Task 9, spec §5.2). DM/agent only, and
	// this row is add_actor's read backwards: removing an actor authors WHO IS
	// IN THE WORLD, so it inherits add_actor's role set rather than
	// move_token's. No player row even for an actor the player CONTROLS —
	// control is not ownership of existence, and this command emits
	// remove_token's event for every token the actor has, so a player row
	// would be a route around remove_token's own. See authz_test.go's
	// authzCases comment on this row for the argument in full.
	"remove_actor":  {identity.RoleDM: true, identity.RoleAgent: true},
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
	// as add_actor/place_token, no additional ownership check
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
	// it" — is mayWorkDoor, below, wired in through playerRules (Task 6).
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
	// spectators, and playerRules below is unreachable for them.
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

// authorizePlayer is the player half, split out so a test can reach the
// missing-rule arm through the REAL code path. A hook that re-decided it
// alongside would prove only that the hook works: measured 2026-09-12, a first
// version of AuthorizeUndecidedForTest called errUndecided itself, and reverting
// this arm to the old `return nil` left its test green.
// CALLERS MUST PASS commandName(cmd). Inside the old inline switch that was
// structural — name was derived two lines above and nothing else could reach it
// — and as a package-level function it is a convention instead. It is what
// keeps each rule's cmd.GetX() matched to the key it is filed under; pass a
// mismatched pair and a command would be judged by another's rule. Authorize is
// the only production caller and derives both together.
func authorizePlayer(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State, name string) error {
	rule, decided := playerRules[name]
	if !decided {
		// FAIL CLOSED, the way commandRoles above already does. This switch was
		// a switch with no default until 2026-09-12, falling through to a bare
		// `return nil`: a command that granted a player cell and had no
		// ownership arm was allowed unconditionally, and nothing anywhere
		// noticed. add_narration relied on that fall-through deliberately —
		// every future one would have inherited it by accident.
		return errUndecided(p, name)
	}
	return rule(p, cmd, st)
}

// playerRule decides whether THIS player may issue this command, after
// commandRoles has already decided their role may issue it at all.
type playerRule func(p *identity.Participant, cmd *vttv1.ClientCommand, st *engine.State) error

// playerRules is the second half of the authorization policy, and it is a table
// rather than a switch so that it can be COMPARED against the first half.
//
// commandRoles says which roles may issue a command; this says what a player
// additionally has to own, control or stand next to.
//
// THE SECURITY FIX IS THE MISSING-RULE ARM, NOT THE TABLE, and an earlier draft
// of this comment blurred the two. A `default:` on the switch this replaced
// would close the same leak: an unruled player command is refused either way.
// The table is not what makes a forgotten rule visible either — measured, the
// 88-cell matrix (TestAuthorizeTableAllCommandsAllRoles) catches a deleted
// entry on its own, by flipping that cell to denied.
//
// WHAT BEING DATA ACTUALLY BUYS is two checks a switch cannot support, both in
// TestEveryPlayerCommandHasARule, and both about a command going quietly
// UNUSABLE or a rule going stale rather than about the leak direction:
//
//   - a command given a player cell but no matrix row. The reflection gate
//     demands a ROLE row, not a matrix row, so that command would be refused by
//     the arm above with nothing red to say so.
//   - a rule left behind after a player cell is removed. A switch cannot be
//     enumerated, so no test over one can ask this at all.
//
// AN ENTRY IS A DECISION, INCLUDING unrestricted. There is no nil value and no
// exemption list beside the table: a command that genuinely needs no ownership
// rule says so in the same place as one that does, because an absence cannot be
// told from an oversight.
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
	// NARRATION IS NOT SCOPED TO AN ACTOR, so there is nothing for a player to
	// own here — everyone at the table narrates (spec §5, and commandRoles'
	// own note on this row). It was the only command relying on the old
	// fall-through, and writing it out is the point: the decision is now in the
	// table beside the six that do restrict, rather than being the shape of a
	// missing case.
	"add_narration": unrestricted,
}

// errUndecided is the refusal the missing-rule arm produces. It is a function so
// that a test can reach the arm at all: with every player cell ruled on, the
// path is unreachable through Authorize, and an unreachable guard is one nobody
// has read.
func errUndecided(p *identity.Participant, name string) error {
	return fmt.Errorf("%w: role %q may issue %q and no player rule decides it",
		ErrUnauthorized, p.Role, name)
}

// unrestricted is the rule for a player command that has no ownership
// condition. It exists so that "no rule applies" is something the table SAYS
// rather than something a reader infers from a name not being there.
func unrestricted(*identity.Participant, *vttv1.ClientCommand, *engine.State) error { return nil }

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
