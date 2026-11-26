package gateway

import (
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Read an addition against rs alone; the sheet is read by handleReferenceChange
// and the rest by the fold (SPEC-013, VTT-1046, VTT-1061).
func validateAddReference(cmd *vttv1.AddReference, rs *rules.Ruleset) error {
	if cmd.GetReferenceId() == "" {
		return referenceChangeFault("add_reference", cmd.GetActorId(), "reference_id", "must not be empty")
	}
	if rs == nil {
		return nil
	}
	if f := rs.ReferenceFault(cmd.GetReferenceId(), cmd.GetReference()); f != nil {
		return referenceChangeFault("add_reference", cmd.GetActorId(), f.Field, f.Reason)
	}
	return nil
}

// handleReferenceChange appends the one ReferenceAdded or ReferenceChanged
// ToEvent makes of cmd, and never a RulesetNamed, or nothing (SPEC-028,
// VTT-1046, VTT-1047, VTT-1050).
func (s *Server) handleReferenceChange(requestID string, cmd *vttv1.ClientCommand, st *engine.State,
	p *identity.Participant) *vttv1.CommandResult {
	if err := referenceSheetFault(cmd, st, s.ruleset); err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	env, err := ToEvent(cmd, p)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	seq, err := s.campaign.Append(env)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: seq}
}

// Leave an actor the snapshot does not hold, an id an addition's sheet holds
// already and an id a change's sheet does not hold to the fold (VTT-1046,
// VTT-1047).
func referenceSheetFault(cmd *vttv1.ClientCommand, st *engine.State, rs *rules.Ruleset) error {
	if rs == nil {
		return nil
	}
	if ar := cmd.GetAddReference(); ar != nil {
		refs := st.Actors[ar.GetActorId()].GetReferences()
		if _, holds := refs[ar.GetReferenceId()]; holds {
			return nil
		}
		if f := rs.CopyFault(refs, ar.GetReferenceId(), ar.GetReference()); f != nil {
			return referenceChangeFault("add_reference", ar.GetActorId(), f.Field, f.Reason)
		}
		return nil
	}
	rc := cmd.GetChangeReference()
	refs := st.Actors[rc.GetActorId()].GetReferences()
	was, holds := refs[rc.GetReferenceId()]
	if !holds {
		return nil
	}
	r := &vttv1.Reference{EntryId: was.GetEntryId(), Place: rc.GetPlace(), State: rc.GetState()}
	if f := rs.ReferenceFault(rc.GetReferenceId(), r); f != nil {
		return referenceChangeFault("change_reference", rc.GetActorId(), f.Field, f.Reason)
	}
	if f := rs.CopyFault(refs, rc.GetReferenceId(), r); f != nil {
		return referenceChangeFault("change_reference", rc.GetActorId(), f.Field, f.Reason)
	}
	return nil
}

func referenceChangeFault(command, actorID, field, reason string) error {
	return fmt.Errorf("gateway: %s: actor %q: %s: %s", command, actorID, field, reason)
}
