package engine

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

var ErrUnknownVariant = errors.New("engine: unknown event variant")

// Size/anchor limits for the world layer (spec §4): a size posture, not a
// scripting surface — no game-system meaning, just wire-frame bounds.
const (
	maxNoteKeyBytes   = 128
	maxNoteTitleBytes = 256
	maxTextBytes      = 8192 // 8 KiB; applies to both narration and note text
	// maxNarrationAsBytes caps the narration speaker label the same way
	// NoteUpserted.Title is capped (256 B). Merge-gate MUST-FIX, overturning
	// the task-level "deliberate" ruling: `as` was otherwise the one
	// participant-writable world-layer field with no cap of its own,
	// leaving its effective bound resting silently on coder/websocket's
	// unpinned ~32 KiB default read limit — append-only permanence means
	// that posture has to be owned by the fold before any live log exists,
	// not inherited from a third-party default nobody pinned.
	maxNarrationAsBytes = 256
)

// Apply advances st by one event. It validates BEFORE mutating: any error
// return leaves st unchanged. AttackRolled, EventsRetracted, AbilityUsed,
// NarrationAdded, and AdventureLoaded are deliberate no-ops here (spec §5;
// world-layer spec §4 for NarrationAdded — the feed IS the log, read via
// the existing event streams; adventure-format spec §3 for AdventureLoaded
// — AbilityUsed's pattern, meaning arrives via the other events in the same
// Compile batch, internal/adventure).
// Complexity: one switch over every event type. ADR-003 mandates a SINGLE
// fold, so this is dispatch breadth, not tangled logic — splitting it into
// per-event helpers would scatter the very thing ADR-003 exists to keep in
// one place.
//
//nolint:gocyclo
func Apply(st *State, env *vttv1.Envelope) error {
	switch p := env.Payload.(type) {
	case *vttv1.Envelope_SessionStarted:
		if st.openSession() >= 0 {
			return fmt.Errorf("engine: session already open")
		}
		st.Sessions = append(st.Sessions, Session{
			ID: env.SessionId, Name: p.SessionStarted.Name, StartSeq: env.Sequence,
		})
		return nil

	case *vttv1.Envelope_SessionEnded:
		i := st.openSession()
		if i < 0 {
			return fmt.Errorf("engine: no open session to end")
		}
		st.Sessions[i].EndSeq = env.Sequence
		return nil

	case *vttv1.Envelope_SceneCreated:
		sc := p.SceneCreated
		if _, dup := st.Scenes[sc.SceneId]; dup {
			return fmt.Errorf("engine: scene %q already exists", sc.SceneId)
		}
		st.Scenes[sc.SceneId] = Scene{
			ID: sc.SceneId, Name: sc.Name,
			GridWidth: sc.GridWidth, GridHeight: sc.GridHeight,
		}
		return nil

	case *vttv1.Envelope_ActorAdded:
		a := p.ActorAdded.Actor
		if a == nil || a.ActorId == "" {
			return fmt.Errorf("engine: actor_added requires an actor with an id")
		}
		if _, dup := st.Actors[a.ActorId]; dup {
			return fmt.Errorf("engine: actor %q already exists", a.ActorId)
		}
		stored := proto.Clone(a).(*vttv1.Actor)
		// Seed the set from the declared controller, then mirror. Without the
		// seed, mirrorControl below sees an empty set and blanks ControllerId
		// AT CREATION — the declared controller is erased on the spot, not on
		// some later grant.
		if stored.GetControllerId() != "" && len(stored.GetControllerIds()) == 0 {
			stored.ControllerIds = []string{stored.GetControllerId()}
		}
		// Drop empty ids the payload may carry. The guard in controlTarget
		// only covers grant/revoke, so without this an ActorAdded carrying
		// controller_ids:[""] creates a NON-EMPTY set whose mirror is the
		// empty string — the "is this shared or unowned?" ambiguity the whole
		// rule exists to remove — and revoke then refuses to remove it,
		// permanently, in an append-only log. Reachable: gateway/convert.go
		// passes the client's Actor through verbatim.
		kept := make([]string, 0, len(stored.GetControllerIds()))
		for _, id := range stored.GetControllerIds() {
			if id != "" {
				kept = append(kept, id)
			}
		}
		stored.ControllerIds = kept
		mirrorControl(stored)
		st.Actors[a.ActorId] = stored
		return nil

	case *vttv1.Envelope_ActorControlGranted:
		g := p.ActorControlGranted
		actor, err := controlTarget(st, g.GetActorId(), g.GetParticipantId(), "actor_control_granted")
		if err != nil {
			return err
		}
		for _, id := range actor.GetControllerIds() {
			if id == g.GetParticipantId() {
				return nil // idempotent: already controls it
			}
		}
		actor.ControllerIds = append(actor.ControllerIds, g.GetParticipantId())
		mirrorControl(actor)
		return nil

	case *vttv1.Envelope_ActorControlRevoked:
		r := p.ActorControlRevoked
		actor, err := controlTarget(st, r.GetActorId(), r.GetParticipantId(), "actor_control_revoked")
		if err != nil {
			return err
		}
		// A fresh slice rather than the in-place [:0] trick: sets here are one
		// to three elements, so the allocation is free, and aliasing the
		// backing array would corrupt any caller holding the old slice across
		// this Apply. Nothing does today — every escape path proto.Clones —
		// but that is a property of today's callers, not of this code.
		kept := make([]string, 0, len(actor.GetControllerIds()))
		for _, id := range actor.GetControllerIds() {
			if id != r.GetParticipantId() {
				kept = append(kept, id)
			}
		}
		actor.ControllerIds = kept
		mirrorControl(actor)
		return nil

	case *vttv1.Envelope_TokenPlaced:
		tp := p.TokenPlaced
		if _, dup := st.Tokens[tp.TokenId]; dup {
			return fmt.Errorf("engine: token %q already exists", tp.TokenId)
		}
		if _, ok := st.Scenes[tp.SceneId]; !ok {
			return fmt.Errorf("engine: token placed in unknown scene %q", tp.SceneId)
		}
		if _, ok := st.Actors[tp.ActorId]; !ok {
			return fmt.Errorf("engine: token placed for unknown actor %q", tp.ActorId)
		}
		if tp.Position == nil {
			return fmt.Errorf("engine: token placed without position")
		}
		st.Tokens[tp.TokenId] = Token{
			ID: tp.TokenId, SceneID: tp.SceneId, ActorID: tp.ActorId,
			X: tp.Position.X, Y: tp.Position.Y,
		}
		return nil

	case *vttv1.Envelope_TokenMoved:
		tm := p.TokenMoved
		tok, ok := st.Tokens[tm.TokenId]
		if !ok {
			return fmt.Errorf("engine: moved unknown token %q", tm.TokenId)
		}
		if tm.To == nil {
			return fmt.Errorf("engine: token move without destination")
		}
		tok.X, tok.Y = tm.To.X, tm.To.Y
		st.Tokens[tm.TokenId] = tok
		return nil

	case *vttv1.Envelope_AttackRolled:
		return nil // testimony, not state — rules meaning arrives in sub-project 5

	case *vttv1.Envelope_EventsRetracted:
		return nil // handled by campaign rebuild, not in-line

	case *vttv1.Envelope_AbilityUsed:
		return nil // testimony, not state — meaning arrives via the ResourceChanged/ConditionApplied/ConditionRemoved events in the same batch (ruleset-interpreter spec §3)

	case *vttv1.Envelope_AdventureLoaded:
		return nil // testimony, not state — AbilityUsed's pattern: the compile batch's FIRST event, meaning arrives via the SceneCreated/ActorAdded/TokenPlaced/NoteUpserted/NarrationAdded events that follow it in the same batch (adventure-format spec §3, internal/adventure's Compile)

	case *vttv1.Envelope_ResourceChanged:
		rc := p.ResourceChanged
		actor, ok := st.Actors[rc.ActorId]
		if !ok {
			return fmt.Errorf("engine: resource changed for unknown actor %q", rc.ActorId)
		}
		res, ok := actor.Resources[rc.Resource]
		if !ok {
			return fmt.Errorf("engine: resource changed for unknown resource %q on actor %q", rc.Resource, rc.ActorId)
		}
		// Compute the clamp in int64 so int32 arithmetic can never wrap
		// before the floor/cap is applied: a hand-crafted event could
		// otherwise drive res.Current + rc.Delta past int32, wrap to a value
		// the floor pulls to 0 (or the cap accepts), and get accepted with a
		// dishonest new_value (spec §3; matches internal/rules' int32-bounded
		// emission so a legitimately-emitted event still verifies exactly).
		computed := int64(res.Current) + int64(rc.Delta)
		if computed < 0 {
			computed = 0
		}
		if res.Max > 0 && computed > int64(res.Max) {
			computed = int64(res.Max)
		}
		// The interpreter computes the post-clamp value; the engine
		// verifies it here rather than trusting it, keeping the log's
		// testimony honest (spec §3).
		if computed != int64(rc.NewValue) {
			return fmt.Errorf("engine: resource %q on actor %q: event new_value %d does not match computed %d", rc.Resource, rc.ActorId, rc.NewValue, computed)
		}
		// computed == rc.NewValue (an int32), so it fits int32 for storage.
		actor.Resources[rc.Resource] = &vttv1.Resource{Current: int32(computed), Max: res.Max}
		return nil

	case *vttv1.Envelope_ConditionApplied:
		ca := p.ConditionApplied
		if _, ok := st.Actors[ca.ActorId]; !ok {
			return fmt.Errorf("engine: condition applied for unknown actor %q", ca.ActorId)
		}
		for _, c := range st.Conditions[ca.ActorId] {
			if c.ID == ca.ConditionId {
				return fmt.Errorf("engine: condition %q already applied to actor %q", ca.ConditionId, ca.ActorId)
			}
		}
		st.Conditions[ca.ActorId] = append(st.Conditions[ca.ActorId], ActorCondition{
			ID: ca.ConditionId, Source: ca.Source, AppliedSeq: env.Sequence,
		})
		return nil

	case *vttv1.Envelope_NarrationAdded:
		na := p.NarrationAdded
		if len(na.Text) == 0 || len(na.Text) > maxTextBytes {
			return fmt.Errorf("engine: narration text must be 1-%d bytes, got %d", maxTextBytes, len(na.Text))
		}
		if len(na.As) > maxNarrationAsBytes {
			return fmt.Errorf("engine: narration as must be at most %d bytes, got %d", maxNarrationAsBytes, len(na.As))
		}
		if na.AnchorFromSeq < 0 || na.AnchorToSeq < 0 {
			return fmt.Errorf("engine: narration anchor sequence must not be negative (from %d, to %d)", na.AnchorFromSeq, na.AnchorToSeq)
		}
		// 0/0 is unanchored and valid; anchors are a range or absent, never
		// a half-range (spec §4).
		if na.AnchorFromSeq != 0 || na.AnchorToSeq != 0 {
			if na.AnchorFromSeq == 0 || na.AnchorToSeq == 0 {
				return fmt.Errorf("engine: narration anchor must set both anchor_from_seq and anchor_to_seq, or neither (got from %d, to %d)", na.AnchorFromSeq, na.AnchorToSeq)
			}
			if na.AnchorFromSeq > na.AnchorToSeq {
				return fmt.Errorf("engine: narration anchor_from_seq %d must not exceed anchor_to_seq %d", na.AnchorFromSeq, na.AnchorToSeq)
			}
			// Anchors point backward at recorded history, never forward at
			// or beyond the narrating event's own sequence.
			if na.AnchorToSeq >= env.Sequence {
				return fmt.Errorf("engine: narration anchor_to_seq %d must be before this event's own sequence %d", na.AnchorToSeq, env.Sequence)
			}
		}
		return nil // deliberate no-op — the feed IS the log (spec §4)

	case *vttv1.Envelope_NoteUpserted:
		nu := p.NoteUpserted
		if len(nu.Key) == 0 || len(nu.Key) > maxNoteKeyBytes {
			return fmt.Errorf("engine: note key must be 1-%d bytes, got %d", maxNoteKeyBytes, len(nu.Key))
		}
		if len(nu.Title) > maxNoteTitleBytes {
			return fmt.Errorf("engine: note title must be at most %d bytes, got %d", maxNoteTitleBytes, len(nu.Title))
		}
		if len(nu.Text) == 0 || len(nu.Text) > maxTextBytes {
			return fmt.Errorf("engine: note text must be 1-%d bytes, got %d", maxTextBytes, len(nu.Text))
		}
		st.Notes[nu.Key] = Note{Title: nu.Title, Text: nu.Text, UpdatedSeq: env.Sequence}
		return nil

	case *vttv1.Envelope_NoteDeleted:
		nd := p.NoteDeleted
		if _, ok := st.Notes[nd.Key]; !ok {
			return fmt.Errorf("engine: note %q not present", nd.Key)
		}
		delete(st.Notes, nd.Key)
		return nil

	case *vttv1.Envelope_ConditionRemoved:
		cr := p.ConditionRemoved
		if _, ok := st.Actors[cr.ActorId]; !ok {
			return fmt.Errorf("engine: condition removed for unknown actor %q", cr.ActorId)
		}
		list := st.Conditions[cr.ActorId]
		idx := -1
		for i, c := range list {
			if c.ID == cr.ConditionId {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("engine: condition %q not present on actor %q", cr.ConditionId, cr.ActorId)
		}
		st.Conditions[cr.ActorId] = append(list[:idx], list[idx+1:]...)
		return nil

	default:
		return fmt.Errorf("%w: %T", ErrUnknownVariant, env.Payload)
	}
}

// controlTarget resolves the actor a control event names, rejecting an unknown
// actor and an empty participant.
//
// Unknown actor is an error rather than a no-op for the same reason
// ConditionApplied/Removed reject one: an event that names something absent
// leaves the log meaning nothing, and a silent skip makes the divergence
// surface later, somewhere unrelated.
//
// Empty participant is rejected because "" in the set would make
// controller_ids non-empty while controller_id mirrors an empty string —
// reintroducing exactly the "is this shared or unowned?" ambiguity the mirror
// rule exists to prevent.
func controlTarget(st *State, actorID, participantID, event string) (*vttv1.Actor, error) {
	if participantID == "" {
		return nil, fmt.Errorf("engine: %s requires a participant id", event)
	}
	actor, ok := st.Actors[actorID]
	if !ok {
		return nil, fmt.Errorf("engine: %s names unknown actor %q", event, actorID)
	}
	return actor, nil
}

// mirrorControl restores the invariant every reader depends on:
// controller_id == controller_ids[0] when the set is non-empty, and empty
// only when the set is empty.
//
// The mirror exists because controller_id persists in ActorAdded events
// already written, and readers predating the set still consult it —
// internal/gateway/authz.go, client/src/player.ts's "your actors" filter,
// `vtt state dump`, MCP get_state. Letting the two disagree silently grants or
// removes a character for someone.
//
// controller_ids[0] rather than "empty when shared": protojson omits empty
// strings, so blanking it for a shared actor would be byte-identical on the
// wire to an UNOWNED one, and empty already means DM/agent-only. This way an
// old reader is incomplete but never wrong.
func mirrorControl(a *vttv1.Actor) {
	if len(a.GetControllerIds()) == 0 {
		a.ControllerId = ""
		return
	}
	a.ControllerId = a.GetControllerIds()[0]
}
