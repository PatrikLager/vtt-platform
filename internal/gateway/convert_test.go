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

// TestToEventCreateSceneCarriesTilesAndObjects pins that a CreateScene
// declaring terrain does not get it silently discarded on the way to
// SceneCreated — maps-as-geometry Task 1 added Tiles/Objects to CreateScene
// and tools/toolgen advertises both to MCP as part of create_scene's
// contract, so an agent-issued command with terrain must produce an event
// carrying that SAME terrain, not a scene with none. This is the same class
// of defect Task 1 already fixed once for OpenDoor/CloseDoor, but silent
// rather than an error: without this test, ToEvent's CreateScene arm could
// drop Tiles/Objects and every other test here (which never sets them)
// would still pass.
func TestToEventCreateSceneCarriesTilesAndObjects(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	tiles := map[string]*vttv1.TileRef{
		"0,0": {Kind: "wall", Material: "stone"},
		"1,0": {Kind: "floor", Material: "wood", Art: "planks-3"},
	}
	objects := []*vttv1.SceneObject{{
		ObjectId: "o1", Kind: "boulder",
		At: &vttv1.GridPosition{X: 1, Y: 0}, Width: 1, Height: 1,
		BlocksSight: true, BlocksMove: true,
	}}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_CreateScene{
		CreateScene: &vttv1.CreateScene{
			SceneId: "scn", Name: "Cave", GridWidth: 2, GridHeight: 1,
			Tiles: tiles, Objects: objects,
		},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := env.Payload.(*vttv1.Envelope_SceneCreated)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_SceneCreated", env.Payload)
	}
	if len(sc.SceneCreated.GetTiles()) != 2 {
		t.Fatalf("Tiles = %v, want 2 entries (Tiles was dropped)", sc.SceneCreated.GetTiles())
	}
	if got := sc.SceneCreated.GetTiles()["1,0"]; got.GetArt() != "planks-3" || got.GetMaterial() != "wood" {
		t.Fatalf("Tiles[1,0] = %v, want art planks-3, material wood", got)
	}
	if len(sc.SceneCreated.GetObjects()) != 1 || sc.SceneCreated.GetObjects()[0].GetObjectId() != "o1" {
		t.Fatalf("Objects = %v, want one entry with object_id o1 (Objects was dropped)", sc.SceneCreated.GetObjects())
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

// TestToEventOpenDoorProducesDoorOpened and its CloseDoor counterpart below
// are maps-as-geometry Task 1's fix: OpenDoor/CloseDoor carry scene_id and
// at straight through to DoorOpened/DoorClosed, the same plain
// single-Envelope conversion as remove_condition/grant_actor_control above
// — no movement/adjacency check here, that is Task 6's job at the call
// site (engine.State.Blocked doesn't exist until Task 5).
func TestToEventOpenDoorProducesDoorOpened(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_OpenDoor{
		OpenDoor: &vttv1.OpenDoor{SceneId: "cellar", At: &vttv1.GridPosition{X: 0, Y: 1}},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	do, ok := env.Payload.(*vttv1.Envelope_DoorOpened)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_DoorOpened", env.Payload)
	}
	if do.DoorOpened.GetSceneId() != "cellar" || do.DoorOpened.GetAt().GetX() != 0 || do.DoorOpened.GetAt().GetY() != 1 {
		t.Fatalf("DoorOpened = %+v, want cellar/(0,1)", do.DoorOpened)
	}
}

func TestToEventCloseDoorProducesDoorClosed(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: "cellar", At: &vttv1.GridPosition{X: 0, Y: 1}},
	}}

	env, err := gateway.ToEvent(cmd, p)
	if err != nil {
		t.Fatal(err)
	}
	dc, ok := env.Payload.(*vttv1.Envelope_DoorClosed)
	if !ok {
		t.Fatalf("payload = %T, want *Envelope_DoorClosed", env.Payload)
	}
	if dc.DoorClosed.GetSceneId() != "cellar" || dc.DoorClosed.GetAt().GetX() != 0 || dc.DoorClosed.GetAt().GetY() != 1 {
		t.Fatalf("DoorClosed = %+v, want cellar/(0,1)", dc.DoorClosed)
	}
}

// TestToEventGrantActorControlCarriesTheKind guards the SILENT half of
// visibility spec §5.1's third rule. The refusal for a grant that states no
// kind lives at the command boundary (validateGrantActorControl, called from
// handleCommand), but nothing there helps if the conversion then drops the
// field a caller did supply: the grant would be accepted, written kindless,
// and read back as something nobody declared — the original leak's shape,
// reached through the very check meant to close it, with ok=true.
//
// This is the same failure mode the CreateScene arm's own comment records
// (Tiles/Objects dropped in conversion, silent, ok=true), which is why it is
// asserted rather than assumed.
func TestToEventGrantActorControlCarriesTheKind(t *testing.T) {
	p := &identity.Participant{ID: "p-dm", Role: identity.RoleDM}
	for _, kind := range []vttv1.ActorKind{
		vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	} {
		t.Run(kind.String(), func(t *testing.T) {
			cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{
				GrantActorControl: &vttv1.GrantActorControl{
					ActorId: "act-archer", ParticipantId: "p-2", Kind: kind},
			}}

			env, err := gateway.ToEvent(cmd, p)
			if err != nil {
				t.Fatal(err)
			}
			g, ok := env.Payload.(*vttv1.Envelope_ActorControlGranted)
			if !ok {
				t.Fatalf("payload = %T, want *Envelope_ActorControlGranted", env.Payload)
			}
			if got := g.ActorControlGranted.GetKind(); got != kind {
				t.Fatalf("ActorControlGranted.Kind = %v, want %v — a kind dropped in "+
					"conversion is an accepted grant that reads as a party member", got, kind)
			}
			if g.ActorControlGranted.GetActorId() != "act-archer" ||
				g.ActorControlGranted.GetParticipantId() != "p-2" {
				t.Fatalf("ActorControlGranted = %+v, want act-archer/p-2", g.ActorControlGranted)
			}
		})
	}
}

func TestToEventUnknownCommandErrors(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RoleDM}
	if env, err := gateway.ToEvent(&vttv1.ClientCommand{}, p); err == nil {
		t.Fatalf("want error for an empty/unset command, got envelope %v", env)
	}
}

// TestEveryClientCommandConverts is the gate that would have caught the defect
// this test was written for, and it is the reason it exists rather than just
// two more cases below.
//
// grant_actor_control and revoke_actor_control shipped ADVERTISED and DEAD:
// the contract carried them, both folds applied their events, the 60-cell
// authz matrix permitted them, and cmd/vtt/tools.json offered them to agents
// as MCP tools — but ToEvent had no arm, so every one returned
// ErrUnknownCommand. Eight commits and a green `task check` later, the
// feature's primary flow (spec §3.1, "actors are assigned afterwards") could
// not happen at all.
//
// Nothing noticed because the authz tests stop at Authorize and never cross
// into conversion, and the MCP tests dispatch against a fake server that
// always replies ok. A per-command test would have missed the NEXT command the
// same way; this iterates the oneof, so a variant joins the gate by existing.
// internal/mcp/tools.go:56 and tools/toolgen already do exactly this — the
// gateway was the layer without it.
func TestEveryClientCommandConverts(t *testing.T) {
	// Commands that deliberately do NOT convert here, each with its reason.
	// Adding a name to this list is a decision; forgetting one is a failure.
	notConverted := map[string]string{
		"use_ability":    "resolved through the ruleset, which emits its own events (server.go)",
		"load_adventure": "expands to a batch of events, handled before ToEvent (adventure.go)",
		"load_map": "expands to a batch of events, handled before ToEvent (map.go) — the " +
			"same shape as load_adventure directly above",
		"retract_events": "a retraction range, not a single event (handleRetraction)",
		"promote_participant": "changes IDENTITY, not campaign state, so it produces no " +
			"event at all — a role lives in participants.role beside the token, one " +
			"source of truth, never in the log (joining-a-table spec §3.1, §3.1a)",
		"set_join_door": "operational state, like presence — a replay must not reopen a " +
			"door somebody closed, so the door lives in identity and not in the log " +
			"(joining-a-table spec §2, §4)",
		"rotate_join_link": "mints a new shared secret in identity; the same reasoning as " +
			"set_join_door, and putting a secret in the permanent log would be worse " +
			"than useless (joining-a-table spec §2, §4)",
		"set_viewpoint": "appends nothing, the same shape as set_join_door above: where a " +
			"spectator points their camera is a view preference, not campaign history, so " +
			"logging it would replay forever and make it retractable (visibility spec " +
			"§3.1.1). serve answers it directly (handleSetViewpoint) and it never reaches " +
			"ToEvent at all.",
	}

	oneof := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor().Oneofs().ByName("command")
	if oneof == nil {
		t.Fatal("vttv1.ClientCommand has no \"command\" oneof")
	}
	p := &identity.Participant{ID: "p-dm", Name: "DM", Role: identity.RoleDM}

	for i := range oneof.Fields().Len() {
		fd := oneof.Fields().Get(i)
		name := string(fd.Name())
		t.Run(name, func(t *testing.T) {
			if why, skip := notConverted[name]; skip {
				t.Logf("not converted here: %s", why)
				return
			}
			// Build the command with only the oneof arm set. ToEvent must not
			// need a populated payload to know which event it becomes.
			cmd := &vttv1.ClientCommand{RequestId: "r-" + name}
			cmd.ProtoReflect().Set(fd, cmd.ProtoReflect().NewField(fd))

			env, err := gateway.ToEvent(cmd, p)
			if err != nil {
				t.Fatalf("%s does not convert: %v — it is reachable from the wire and from "+
					"MCP, so this command is advertised and dead", name, err)
			}
			if env.GetPayload() == nil {
				t.Fatalf("%s converted to an envelope with no payload", name)
			}
		})
	}
}
