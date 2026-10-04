package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// ErrUnknownCommand is returned by ToEvent for an unset or unrecognized
// ClientCommand oneof.
var ErrUnknownCommand = errors.New("gateway: unknown or empty command")

// ToEvent converts an authorized ClientCommand into the past-tense Envelope
// it becomes, stamping EventId, ParticipantId, ActorRole and OccurredAt
// (SPEC-013). TestEveryClientCommandConverts lists the commands that do not
// convert here.
func ToEvent(cmd *vttv1.ClientCommand, p *identity.Participant) (*vttv1.Envelope, error) {
	env := &vttv1.Envelope{
		ParticipantId: p.ID,
		ActorRole:     string(p.Role),
		OccurredAt:    timestamppb.Now(),
	}
	// Validate nothing in these arms: Authorize, the validators and the fold
	// have (SPEC-013).
	switch c := cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_MoveToken:
		env.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: c.MoveToken.GetTokenId(),
			To:      c.MoveToken.GetTo(),
			Reason:  c.MoveToken.GetReason(),
		}}
	case *vttv1.ClientCommand_AddActor:
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
			Actor: c.AddActor.GetActor(),
		}}
	case *vttv1.ClientCommand_PlaceToken:
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId:  c.PlaceToken.GetTokenId(),
			SceneId:  c.PlaceToken.GetSceneId(),
			ActorId:  c.PlaceToken.GetActorId(),
			Position: c.PlaceToken.GetPosition(),
		}}
	case *vttv1.ClientCommand_RemoveToken:
		env.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{
			TokenId: c.RemoveToken.GetTokenId(),
		}}
	case *vttv1.ClientCommand_StartSession:
		env.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{
			Name: c.StartSession.GetName(),
		}}
	case *vttv1.ClientCommand_EndSession:
		env.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}
	case *vttv1.ClientCommand_RemoveCondition:
		env.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
			ActorId:     c.RemoveCondition.GetActorId(),
			ConditionId: c.RemoveCondition.GetConditionId(),
			Reason:      "manual",
		}}
	case *vttv1.ClientCommand_AddNarration:
		env.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
			Text:          c.AddNarration.GetText(),
			As:            c.AddNarration.GetAs(),
			AnchorFromSeq: c.AddNarration.GetAnchorFromSeq(),
			AnchorToSeq:   c.AddNarration.GetAnchorToSeq(),
		}}
	case *vttv1.ClientCommand_UpsertNote:
		env.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key:        c.UpsertNote.GetKey(),
			Title:      c.UpsertNote.GetTitle(),
			Text:       c.UpsertNote.GetText(),
			Visibility: c.UpsertNote.GetVisibility(),
		}}
	case *vttv1.ClientCommand_DeleteNote:
		env.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{
			Key: c.DeleteNote.GetKey(),
		}}
	case *vttv1.ClientCommand_GrantActorControl:
		// Carry Kind through (TestToEventGrantActorControlCarriesTheKind): a
		// dropped kind answers ok=true and records a grant that changes no kind.
		env.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId:       c.GrantActorControl.GetActorId(),
			ParticipantId: c.GrantActorControl.GetParticipantId(),
			Kind:          c.GrantActorControl.GetKind(),
		}}
	case *vttv1.ClientCommand_RevokeActorControl:
		env.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
			ActorId:       c.RevokeActorControl.GetActorId(),
			ParticipantId: c.RevokeActorControl.GetParticipantId(),
		}}
	case *vttv1.ClientCommand_OpenDoor:
		env.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{
			SceneId: c.OpenDoor.GetSceneId(),
			At:      c.OpenDoor.GetAt(),
		}}
	case *vttv1.ClientCommand_CloseDoor:
		env.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: &vttv1.DoorClosed{
			SceneId: c.CloseDoor.GetSceneId(),
			At:      c.CloseDoor.GetAt(),
		}}
	default:
		return nil, ErrUnknownCommand
	}

	id, err := newEventID()
	if err != nil {
		return nil, err
	}
	env.EventId = id
	return env, nil
}

func newEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gateway: generate event id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
