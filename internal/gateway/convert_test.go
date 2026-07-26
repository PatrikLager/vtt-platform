package gateway_test

import (
	"errors"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func TestToEventStampsParticipantRoleAndID(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{
		EndSession: &vttv1.EndSession{},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	if env.ParticipantId != "p-1" {
		t.Fatalf("ParticipantId = %q, want %q", env.ParticipantId, "p-1")
	}
	if env.ActorRole != "dm" {
		t.Fatalf("ActorRole = %q, want %q", env.ActorRole, "dm")
	}
	if env.EventId == "" {
		t.Fatal("want non-empty EventId")
	}
	if env.OccurredAt == nil {
		t.Fatal("want non-nil OccurredAt")
	}
}

func TestToEventEventIDsAreUniquePerCall(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{
		EndSession: &vttv1.EndSession{},
	}}

	e1, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	if e1.EventId == e2.EventId {
		t.Fatalf("want distinct EventIds across calls, got %q twice", e1.EventId)
	}
}

func TestToEventMoveTokenProducesTokenMoved(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: "t1", To: &vttv1.GridPosition{X: 5, Y: 8}},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	tm, ok := env.Payload.(*vttv1.Envelope_TokenMoved)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_TokenMoved", env.Payload)
	}
	if tm.TokenMoved.TokenId != "t1" {
		t.Fatalf("TokenId = %q, want %q", tm.TokenMoved.TokenId, "t1")
	}
	if tm.TokenMoved.To.X != 5 || tm.TokenMoved.To.Y != 8 {
		t.Fatalf("To = %+v, want (5,8)", tm.TokenMoved.To)
	}
}

func TestToEventCreateSceneProducesSceneCreated(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_CreateScene{
		CreateScene: &vttv1.CreateScene{SceneId: "scn", Name: "Cave", GridWidth: 10, GridHeight: 8},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := env.Payload.(*vttv1.Envelope_SceneCreated)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_SceneCreated", env.Payload)
	}
	if sc.SceneCreated.SceneId != "scn" || sc.SceneCreated.Name != "Cave" ||
		sc.SceneCreated.GridWidth != 10 || sc.SceneCreated.GridHeight != 8 {
		t.Fatalf("SceneCreated = %+v, want scn/Cave/10/8", sc.SceneCreated)
	}
}

func TestToEventAddActorProducesActorAdded(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleAgent}
	actor := &vttv1.Actor{ActorId: "a1", Name: "Goblin"}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: actor},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	aa, ok := env.Payload.(*vttv1.Envelope_ActorAdded)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_ActorAdded", env.Payload)
	}
	if aa.ActorAdded.Actor.GetActorId() != "a1" {
		t.Fatalf("ActorId = %q, want %q", aa.ActorAdded.Actor.GetActorId(), "a1")
	}
}

func TestToEventPlaceTokenProducesTokenPlaced(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{
		PlaceToken: &vttv1.PlaceToken{
			TokenId: "t1", SceneId: "scn", ActorId: "a1",
			Position: &vttv1.GridPosition{X: 3, Y: 7},
		},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	tp, ok := env.Payload.(*vttv1.Envelope_TokenPlaced)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_TokenPlaced", env.Payload)
	}
	if tp.TokenPlaced.TokenId != "t1" || tp.TokenPlaced.SceneId != "scn" ||
		tp.TokenPlaced.ActorId != "a1" || tp.TokenPlaced.Position.X != 3 || tp.TokenPlaced.Position.Y != 7 {
		t.Fatalf("TokenPlaced = %+v, want t1/scn/a1/(3,7)", tp.TokenPlaced)
	}
}

func TestToEventStartSessionProducesSessionStarted(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: "session one"},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	ss, ok := env.Payload.(*vttv1.Envelope_SessionStarted)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_SessionStarted", env.Payload)
	}
	if ss.SessionStarted.Name != "session one" {
		t.Fatalf("Name = %q, want %q", ss.SessionStarted.Name, "session one")
	}
}

func TestToEventEndSessionProducesSessionEnded(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{
		EndSession: &vttv1.EndSession{},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := env.Payload.(*vttv1.Envelope_SessionEnded); !ok {
		t.Fatalf("payload = %T, want *Envelope_SessionEnded", env.Payload)
	}
}

// TestToEventRetractEventsReturnsSentinel covers the one command that does
// NOT become an Envelope here: campaign.Undo owns marker construction, so
// ToEvent returns nil plus an error wrapping ErrIsRetraction, carrying the
// parsed range via errors.As.
func TestToEventRetractEventsReturnsSentinel(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RetractEvents{
		RetractEvents: &vttv1.RetractEvents{FromSequence: 3, ToSequence: 5, Reason: "mistake"},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if env != nil {
		t.Fatalf("want nil envelope for a retraction command, got %v", env)
	}
	if !errors.Is(err, gateway.ErrIsRetraction) {
		t.Fatalf("want error wrapping ErrIsRetraction, got %v", err)
	}
	var rr *gateway.RetractionRange
	if !errors.As(err, &rr) {
		t.Fatalf("want error to unwrap to *RetractionRange, got %T", err)
	}
	if rr.FromSequence != 3 || rr.ToSequence != 5 || rr.Reason != "mistake" {
		t.Fatalf("RetractionRange = %+v, want {From:3 To:5 Reason:mistake}", rr)
	}
}

// TestToEventRemoveConditionProducesConditionRemoved covers the ONE Task 6
// command that DOES flow through ToEvent (use_ability does not — see
// server.go's handleCommand): a plain single-Envelope conversion, exactly
// like every pre-Task-6 command above.
func TestToEventRemoveConditionProducesConditionRemoved(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveCondition{
		RemoveCondition: &vttv1.RemoveCondition{ActorId: "a1", ConditionId: "dazed"},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	cr, ok := env.Payload.(*vttv1.Envelope_ConditionRemoved)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_ConditionRemoved", env.Payload)
	}
	if cr.ConditionRemoved.ActorId != "a1" || cr.ConditionRemoved.ConditionId != "dazed" {
		t.Fatalf("ConditionRemoved = %+v, want actor_id=a1 condition_id=dazed", cr.ConditionRemoved)
	}
	if cr.ConditionRemoved.Reason == "" {
		t.Fatal("want a non-empty Reason on a gateway-converted ConditionRemoved")
	}
}

// TestToEventAddNarrationProducesNarrationAdded covers the first of the
// three world-layer (Task 3) commands that flow through the SAME plain
// ToEvent -> campaign.Append path as every pre-Task-6 command — no batch,
// no special handling (unlike use_ability).
func TestToEventAddNarrationProducesNarrationAdded(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: "the door creaks open", As: "Goblin Cutter", AnchorFromSeq: 2, AnchorToSeq: 3},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	na, ok := env.Payload.(*vttv1.Envelope_NarrationAdded)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_NarrationAdded", env.Payload)
	}
	if na.NarrationAdded.Text != "the door creaks open" || na.NarrationAdded.As != "Goblin Cutter" ||
		na.NarrationAdded.AnchorFromSeq != 2 || na.NarrationAdded.AnchorToSeq != 3 {
		t.Fatalf("NarrationAdded = %+v, want text/as/anchors verbatim from the command", na.NarrationAdded)
	}
}

// TestToEventUpsertNoteProducesNoteUpserted covers the second world-layer
// command.
func TestToEventUpsertNoteProducesNoteUpserted(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UpsertNote{
		UpsertNote: &vttv1.UpsertNote{Key: "kobold-den", Title: "Kobold Den", Text: "Three kobolds guard the east tunnel."},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	nu, ok := env.Payload.(*vttv1.Envelope_NoteUpserted)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_NoteUpserted", env.Payload)
	}
	if nu.NoteUpserted.Key != "kobold-den" || nu.NoteUpserted.Title != "Kobold Den" || nu.NoteUpserted.Text != "Three kobolds guard the east tunnel." {
		t.Fatalf("NoteUpserted = %+v, want key/title/text verbatim from the command", nu.NoteUpserted)
	}
}

// TestToEventDeleteNoteProducesNoteDeleted covers the third world-layer
// command.
func TestToEventDeleteNoteProducesNoteDeleted(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_DeleteNote{
		DeleteNote: &vttv1.DeleteNote{Key: "kobold-den"},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	nd, ok := env.Payload.(*vttv1.Envelope_NoteDeleted)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_NoteDeleted", env.Payload)
	}
	if nd.NoteDeleted.Key != "kobold-den" {
		t.Fatalf("NoteDeleted.Key = %q, want %q", nd.NoteDeleted.Key, "kobold-den")
	}
}

func TestToEventUnknownCommandErrors(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	if env, err := gateway.ToEvent(&vttv1.ClientCommand{}, p); err == nil {
		t.Fatalf("want error for an empty/unset command, got envelope %v", env)
	}
}
