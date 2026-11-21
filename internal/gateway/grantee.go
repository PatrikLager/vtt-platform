package gateway

import (
	"errors"
	"strings"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// granteeRefusal answers a grant naming nobody the table holds, or leaving a
// player holding an actor that is not a party member; nil otherwise (VTT-894,
// VTT-913, VTT-926). Keep it a handler step: it reads identity (SPEC-013).
func (s *Server) granteeRefusal(requestID string, cmd *vttv1.ClientCommand, st *engine.State) *vttv1.CommandResult {
	g := cmd.GetGrantActorControl()
	if g == nil {
		return nil
	}
	refused := func(msg string) *vttv1.CommandResult {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: msg}
	}
	grantee, err := s.ids.Lookup(g.GetParticipantId())
	if errors.Is(err, identity.ErrInvalidToken) {
		return refused("There is no such participant.")
	}
	if err != nil {
		return refused(err.Error())
	}
	if g.GetKind() == vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		return nil
	}
	if !runsTheTable(grantee.Role) {
		return refused(grantee.Name + " can only be given a party member.")
	}
	actor := st.Actors[g.GetActorId()]
	var players []string
	for _, id := range actor.GetControllerIds() {
		holder, err := s.ids.Lookup(id)
		if errors.Is(err, identity.ErrInvalidToken) {
			continue
		}
		if err != nil {
			return refused(err.Error())
		}
		if !runsTheTable(holder.Role) {
			players = append(players, holder.Name)
		}
	}
	if len(players) == 0 {
		return nil
	}
	verb := " controls "
	if len(players) > 1 {
		verb = " control "
	}
	name := actor.GetName()
	if name == "" {
		name = actor.GetActorId()
	}
	return refused(strings.Join(players, " and ") + verb + name +
		": revoke it from them before you make it a non-party actor.")
}

// runsTheTable is whether r is the DM's or the agent's: the two roles that may
// hold an actor that is not a party member (VTT-913).
func runsTheTable(r identity.Role) bool {
	return r == identity.RoleDM || r == identity.RoleAgent
}
