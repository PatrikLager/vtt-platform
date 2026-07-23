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

// ErrIsRetraction signals that cmd was a RetractEvents command: ToEvent
// deliberately does not build an Envelope for it, because campaign.Undo owns
// constructing the EventsRetracted marker (spec §6 — the marker is derived
// from a dry-run replay, not a plain conversion). Every error ToEvent
// returns for a RetractEvents command is a *RetractionRange, which wraps
// this sentinel via Unwrap so callers can branch with errors.Is and recover
// the parsed range with errors.As.
var ErrIsRetraction = errors.New("gateway: command is a retraction, not a plain event")

// RetractionRange carries the parsed [FromSequence, ToSequence] range and
// reason from a RetractEvents command, so the caller (Task 5) can pass it
// straight to campaign.Undo.
type RetractionRange struct {
	FromSequence, ToSequence int64
	Reason                   string
}

func (r *RetractionRange) Error() string { return ErrIsRetraction.Error() }
func (r *RetractionRange) Unwrap() error { return ErrIsRetraction }

// ErrUnknownCommand is returned by ToEvent for an unset or unrecognized
// ClientCommand oneof.
var ErrUnknownCommand = errors.New("gateway: unknown or empty command")

// ToEvent converts an authorized ClientCommand into the past-tense Envelope
// it becomes, stamping EventId (fresh per call), ParticipantId, ActorRole,
// and OccurredAt. RetractEvents is the one exception: it returns a
// *RetractionRange error instead of an Envelope (see ErrIsRetraction).
func ToEvent(cmd *vttv1.ClientCommand, p *identity.Participant) (*vttv1.Envelope, error) {
	if r, ok := cmd.GetCommand().(*vttv1.ClientCommand_RetractEvents); ok {
		return nil, &RetractionRange{
			FromSequence: r.RetractEvents.GetFromSequence(),
			ToSequence:   r.RetractEvents.GetToSequence(),
			Reason:       r.RetractEvents.GetReason(),
		}
	}

	env := &vttv1.Envelope{
		ParticipantId: p.ID,
		ActorRole:     string(p.Role),
		OccurredAt:    timestamppb.Now(),
	}
	switch c := cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_MoveToken:
		env.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: c.MoveToken.GetTokenId(),
			To:      c.MoveToken.GetTo(),
		}}
	case *vttv1.ClientCommand_CreateScene:
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId:    c.CreateScene.GetSceneId(),
			Name:       c.CreateScene.GetName(),
			GridWidth:  c.CreateScene.GetGridWidth(),
			GridHeight: c.CreateScene.GetGridHeight(),
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
	case *vttv1.ClientCommand_StartSession:
		env.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{
			Name: c.StartSession.GetName(),
		}}
	case *vttv1.ClientCommand_EndSession:
		env.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}
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

// newEventID returns a fresh, random hex event id (16 bytes from
// crypto/rand — collision-negligible; the store enforces uniqueness as the
// hard guarantee, this just needs to not collide in practice).
func newEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("gateway: generate event id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
