package gateway

import (
	"fmt"
	"maps"
	"slices"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// handleGiveAbility appends the AbilityGiven and a ResourceAdded per resource
// the give brings, as one batch, or nothing (SPEC-025, VTT-760, VTT-761, VTT-762).
func (s *Server) handleGiveAbility(requestID string, cmd *vttv1.ClientCommand, g *vttv1.GiveAbility,
	st *engine.State, p *identity.Participant) *vttv1.CommandResult {
	if err := giveSheetFault(g, st, s.ruleset); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	env, err := ToEvent(cmd, p)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	envs := []*vttv1.Envelope{env}
	for _, name := range slices.Sorted(maps.Keys(g.GetResources())) {
		// Copy the carried value whole: the gateway reads no current (SPEC-025).
		value, _ := proto.Clone(g.GetResources()[name]).(*vttv1.Resource)
		envs = append(envs, &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceAdded{ResourceAdded: &vttv1.ResourceAdded{
			ActorId: g.GetActorId(), Resource: name, Value: value}}})
	}
	return s.appendChange(requestID, envs, env, p)
}

// giveSheetFault reads a give against the actor's sheet; an actor the state
// does not hold is the fold's to refuse (VTT-717).
func giveSheetFault(g *vttv1.GiveAbility, st *engine.State, rs *rules.Ruleset) error {
	a, held := st.Actors[g.GetActorId()]
	if rs == nil || !held {
		return nil
	}
	for _, key := range slices.Sorted(maps.Keys(g.GetResources())) {
		if _, holds := a.GetResources()[key]; holds {
			return giveFault(g, "resources", fmt.Sprintf("the sheet holds resource %q already", key))
		}
	}
	spent, limited := rs.Spends(g.GetAbilityId())
	if !limited {
		return nil
	}
	_, holds := a.GetResources()[spent]
	if _, carried := g.GetResources()[spent]; !holds && !carried {
		return giveFault(g, "resources", fmt.Sprintf(
			"ability %q spends resource %q, which the sheet does not hold: give it with its value", g.GetAbilityId(), spent))
	}
	return nil
}

// handleTakeAbility appends the AbilityTaken and, with a ruleset served, the
// ResourceRemoved of the resource the taken ability spends when no ability left
// on the sheet spends it, as one batch, or nothing (SPEC-025, VTT-767, VTT-768).
func (s *Server) handleTakeAbility(requestID string, cmd *vttv1.ClientCommand, tk *vttv1.TakeAbility,
	st *engine.State, p *identity.Participant) *vttv1.CommandResult {
	env, err := ToEvent(cmd, p)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	envs := []*vttv1.Envelope{env}
	if name, ok := orphanedBy(tk, st, s.ruleset); ok {
		envs = append(envs, &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceRemoved{ResourceRemoved: &vttv1.ResourceRemoved{
			ActorId: tk.GetActorId(), Resource: name}}})
	}
	return s.appendChange(requestID, envs, env, p)
}

// orphanedBy answers the resource a take leaves no ability on the sheet to spend.
func orphanedBy(tk *vttv1.TakeAbility, st *engine.State, rs *rules.Ruleset) (string, bool) {
	a, held := st.Actors[tk.GetActorId()]
	if rs == nil || !held || !rules.Offers(a, tk.GetAbilityId()) {
		return "", false
	}
	spent, limited := rs.Spends(tk.GetAbilityId())
	if _, holds := a.GetResources()[spent]; !limited || !holds {
		return "", false
	}
	for _, id := range a.GetAbilityIds() {
		if other, ok := rs.Spends(id); id != tk.GetAbilityId() && ok && other == spent {
			return "", false
		}
	}
	return spent, true
}

// appendChange stamps envs as first is stamped and appends them as one batch.
func (s *Server) appendChange(requestID string, envs []*vttv1.Envelope, first *vttv1.Envelope,
	p *identity.Participant) *vttv1.CommandResult {
	for _, env := range envs {
		id, err := newEventID()
		if err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
		env.EventId, env.ParticipantId, env.ActorRole, env.OccurredAt = id, p.ID, string(p.Role), first.GetOccurredAt()
	}
	seq, err := s.campaign.AppendBatch(envs)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: seq}
}
