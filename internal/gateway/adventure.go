package gateway

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// errNoAdventuresAvailable is handleLoadAdventure's clean, ok=false error
// when s has no adventures configured (spec §7 binding: serving without
// --adventures-dir keeps load_adventure rejected with a clean "no
// adventures available", the exact sibling of errNoRulesetLoaded above for
// use_ability). A plain string, not a sentinel error var, for the same
// reason errNoRulesetLoaded is one: nothing else in this package needs to
// errors.Is/As against it.
const errNoAdventuresAvailable = "gateway: no adventures available"

// handleLoadAdventure runs the authorized-load_adventure pipeline (spec §5,
// §7, adventure-format Task 4): lookup by id among the BOOT-TIME preloaded
// adventures (s.adventures, set via WithAdventures — never loaded per
// request, see that method's doc comment), adventure.Compile against a
// read-only state snapshot, then one campaign.AppendBatch for the whole
// ordered event batch Compile returns. This mirrors handleUseAbility
// (ruleset.go) exactly: every failure — no adventures configured, an
// unknown id, a Compile collision, an AppendBatch rejection — is a clean
// ok=false CommandResult, never a connection drop. The caller
// (handleCommand) already ran Authorize before reaching here, so everything
// from this point on is either a config gap or a content-collision
// rejection, not an authorization concern.
//
// TOCTOU race posture (investigated, documented here per task-12-4-brief.md):
// st is a snapshot read via campaign.State() BEFORE this call (handleCommand,
// server.go), so adventure.Compile's own checkCollisions runs against a
// snapshot that could in principle be stale by the time AppendBatch actually
// persists. campaign.AppendBatch closes that window for scene/actor/token
// ids: it re-validates the WHOLE batch by folding CLONES against a FRESH
// snapshot taken under the SAME mutex acquisition that persists (campaign.go's
// own doc comment) — and engine.Apply's SceneCreated/ActorAdded/TokenPlaced
// arms (apply.go) all reject a duplicate id with an error, so any collision
// that arose from a race between this handler's State() call and its own
// AppendBatch call is still caught, atomically, by AppendBatch's internal
// re-fold — the whole batch is rejected, nothing persists. The double-load
// scenario this task's own harness step requires (scenarios/adventure-night.
// json) is safe under this: a second load_adventure of the SAME adventure
// re-emits the SAME scene/actor/token ids, which collide and abort the batch
// fold before any later envelope (including NoteUpserted) is even reached —
// AppendBatch rejects any envelope's fold failure by returning immediately,
// so envelopes after the failing one are never folded at all.
//
// One residual, narrower gap, NOT exercised by the double-load scenario and
// out of this task's named check (ActorAdded/SceneCreated/TokenPlaced only):
// engine.Apply's NoteUpserted arm (apply.go) is a deliberate UPSERT — it has
// no duplicate-key rejection of its own (world-layer note semantics: DM
// upsert_note is meant to overwrite). Compile's own checkCollisions is
// therefore the ONLY layer that treats a note-key collision as a rejection
// (spec §5: "checked against the live snapshot before the batch — rejection,
// not overwrite"), and that check runs against the pre-call snapshot, not a
// fresh one under the AppendBatch lock. A contrived race — a concurrent
// upsert_note (or a second, DIFFERENT adventure) landing on the exact same
// note key between this handler's State() call and its AppendBatch call —
// could in principle have its note silently overwritten rather than
// rejected. This is real but does not affect the double-load-of-the-SAME-
// adventure case (scene/actor/token collide first, well before the batch
// reaches its NoteUpserted envelope, aborting the whole batch) — the two
// committed adventures' note keys never collide with each other either.
// Flagged for controller visibility, not a blocker for this task.
func (s *Server) handleLoadAdventure(requestID string, cmd *vttv1.LoadAdventure, st *engine.State, p *identity.Participant) *vttv1.CommandResult {
	if len(s.adventures) == 0 {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: errNoAdventuresAvailable}
	}
	adv, ok := s.adventures[cmd.GetAdventureId()]
	if !ok {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: fmt.Sprintf("gateway: unknown adventure %q", cmd.GetAdventureId())}
	}

	envs, err := adventure.Compile(adv, st)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// adventure.Compile leaves EventId/ParticipantId/ActorRole/OccurredAt
	// zero on every envelope it returns (same convention rules.Resolve
	// follows for handleUseAbility, and ToEvent for every single-event
	// command) — stamp them here before handing the batch to
	// campaign.AppendBatch.
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
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: firstSeq}
}
