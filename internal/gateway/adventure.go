package gateway

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const errNoAdventuresAvailable = "gateway: no adventures available"

// Answer every failure as an ok=false result, never a close: authorization
// already ran in handleCommand (SPEC-012).
func (s *Server) handleLoadAdventure(requestID string, cmd *vttv1.LoadAdventure, st *engine.State, p *identity.Participant) *vttv1.CommandResult {
	if len(s.adventures) == 0 {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: errNoAdventuresAvailable}
	}
	adv, ok := s.adventures[cmd.GetAdventureId()]
	if !ok {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: fmt.Sprintf("gateway: unknown adventure %q", cmd.GetAdventureId())}
	}

	// Do not rely on AppendBatch to refuse a note-key collision: its re-fold
	// upserts (docs/verification-debt.md).
	envs, warnings, err := adventure.Compile(adv, st)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// Stamp the four fields Compile leaves zero: store.AppendBatch requires an
	// EventId (SPEC-012).
	now := timestamppb.Now()
	for _, env := range envs {
		id, err := newEventID()
		if err != nil {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
		}
		env.EventId = id
		env.ParticipantId = p.ID
		env.ActorRole = string(p.Role)
		env.OccurredAt = now
	}

	firstSeq, err := s.campaign.AppendBatch(envs)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	// Return the warnings to the issuer only; nothing broadcasts them (SPEC-012).
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: firstSeq, Warnings: warnings}
}
