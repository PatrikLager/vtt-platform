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
	case *vttv1.ClientCommand_RemoveCondition:
		// use_ability does NOT flow through ToEvent (server.go's
		// handleUseAbility routes it to rules.Resolve + campaign.AppendBatch
		// instead, ruleset-interpreter Task 6) — it produces a whole ordered
		// batch of events, not the single Envelope this function returns.
		// remove_condition has no such batch: it is a single, direct
		// ConditionRemoved, going through the SAME Authorize -> ToEvent ->
		// campaign.Append path every other one-event command uses.
		// engine.Apply's ConditionRemoved case (internal/engine/apply.go)
		// already rejects an absent condition, and campaign.Append validates
		// against a snapshot BEFORE persisting (internal/campaign/
		// campaign.go) — so an absent condition surfaces as an ordinary
		// ok=false CommandResult here, never a poisoned Campaign.
		env.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
			ActorId:     c.RemoveCondition.GetActorId(),
			ConditionId: c.RemoveCondition.GetConditionId(),
			Reason:      "manual",
		}}
	case *vttv1.ClientCommand_AddNarration:
		// world-layer Task 3: same plain single-Envelope conversion as
		// remove_condition above — size-cap/anchor-sanity validation lives in
		// the fold (internal/engine/apply.go), not here.
		env.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
			Text:          c.AddNarration.GetText(),
			As:            c.AddNarration.GetAs(),
			AnchorFromSeq: c.AddNarration.GetAnchorFromSeq(),
			AnchorToSeq:   c.AddNarration.GetAnchorToSeq(),
		}}
	case *vttv1.ClientCommand_UpsertNote:
		env.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key:   c.UpsertNote.GetKey(),
			Title: c.UpsertNote.GetTitle(),
			Text:  c.UpsertNote.GetText(),
		}}
	case *vttv1.ClientCommand_DeleteNote:
		env.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{
			Key: c.DeleteNote.GetKey(),
		}}
	case *vttv1.ClientCommand_GrantActorControl:
		// presence-and-actor-control Task 3. Plain single-Envelope conversion,
		// the same shape as remove_condition: the fold owns every rule about
		// what a control set may contain (unknown actor, empty participant,
		// idempotent re-grant), and authz has already decided this participant
		// may issue it. Nothing left to validate here.
		env.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId:       c.GrantActorControl.GetActorId(),
			ParticipantId: c.GrantActorControl.GetParticipantId(),
		}}
	case *vttv1.ClientCommand_RevokeActorControl:
		env.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
			ActorId:       c.RevokeActorControl.GetActorId(),
			ParticipantId: c.RevokeActorControl.GetParticipantId(),
		}}
	case *vttv1.ClientCommand_OpenDoor:
		// maps-as-geometry Task 1 fix. Plain single-Envelope conversion, same
		// shape as grant/revoke_actor_control above: no movement/adjacency
		// check here, since engine.State.Blocked and its gateway call site
		// don't exist yet (Tasks 5-6) — Authorize has already decided this
		// participant may issue the command, and there is nothing else to
		// validate at this layer.
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
