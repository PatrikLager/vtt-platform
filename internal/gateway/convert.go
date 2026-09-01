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
// it becomes, stamping EventId (fresh per call), ParticipantId, ActorRole,
// and OccurredAt. Not every command converts here; the ones that deliberately
// do not are listed with their reasons in TestEveryClientCommandConverts,
// which is the gate that keeps that list honest.
func ToEvent(cmd *vttv1.ClientCommand, p *identity.Participant) (*vttv1.Envelope, error) {
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
		// Tiles/Objects carried through (maps-as-geometry Task 1 added both
		// fields to CreateScene; tools/toolgen advertises them to MCP as
		// part of create_scene's contract). Dropping them here would be the
		// same class of defect Task 1 already fixed once for OpenDoor/
		// CloseDoor — worse, in fact: this failure mode is SILENT, not an
		// error, so an agent calling create_scene with terrain would see
		// ok=true and only discover the loss later, reading back a scene
		// with no tiles at all.
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId:    c.CreateScene.GetSceneId(),
			Name:       c.CreateScene.GetName(),
			GridWidth:  c.CreateScene.GetGridWidth(),
			GridHeight: c.CreateScene.GetGridHeight(),
			Tiles:      c.CreateScene.GetTiles(),
			Objects:    c.CreateScene.GetObjects(),
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
		// retraction-leaves Task 8. Plain single-Envelope conversion, the
		// same shape OpenDoor/CloseDoor use below: Authorize has already
		// decided this participant may issue it (a DM/agent-only row), and
		// engine.Apply owns the "unknown token" rejection — there is
		// nothing left to validate at this layer.
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
		// may issue it.
		//
		// Kind is carried THROUGH, and dropping it would be silent in the
		// worst way — the same failure mode CreateScene's arm above records
		// for Tiles/Objects, but with a security consequence rather than a
		// cosmetic one: an accepted grant, written kindless, DEMOTES the
		// character it was meant to hand over. Since §5.1's migration rule was
		// deleted (2026-08-24) an absent kind is not a party member, so the
		// dropped field fails closed rather than open — a character silently
		// off its own party's roster instead of a monster silently on it. Both
		// are the command answering ok=true and doing something else. The
		// completeness of this copy is pinned by
		// TestToEventGrantActorControlCarriesTheKind.
		//
		// Whether the caller stated a kind AT ALL is not decided here.
		// handleCommand refuses that before conversion is reached, beside
		// create_scene's terrain check — see validateGrantActorControl for why
		// that seam and not this one.
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
		// maps-as-geometry Task 1 fix. Plain single-Envelope conversion, same
		// shape as grant/revoke_actor_control above: no adjacency check HERE
		// — that lives in Authorize (authz.go's mayWorkDoor, Task 6), which
		// runs before ToEvent ever sees the command. By the time control
		// reaches this switch, Authorize has already decided this
		// participant may issue it, and there is nothing else to validate at
		// this layer.
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
