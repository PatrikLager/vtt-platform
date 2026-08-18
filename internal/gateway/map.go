package gateway

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// errNoMapsAvailable is handleLoadMap's clean, ok=false error when s has no
// maps configured — errNoAdventuresAvailable's exact sibling (adventure.go),
// for the same reason: serving without --maps-dir keeps load_map rejected
// with a clean "no maps available" rather than a connection drop or crash.
const errNoMapsAvailable = "gateway: no maps available"

// handleLoadMap runs the authorized-load_map pipeline (whole-branch-review
// C1 remediation, maps-as-geometry design spec §4.3): lookup by id among the
// BOOT-TIME preloaded maps (s.maps, set via WithMaps — never loaded per
// request, mirroring WithAdventures' own doc comment), mapdef.Compile
// against the map's own pack (s.packs, keyed by the map's declared Pack id —
// may legally be nil/absent for a map with no overrides, see mapdef.Compile's
// own doc comment), then one campaign.AppendBatch for the whole ordered
// event batch Compile returns. This mirrors handleLoadAdventure
// (adventure.go) almost exactly: every failure — no maps configured, an
// unknown id, a Compile error, an AppendBatch rejection — is a clean
// ok=false CommandResult, never a connection drop. The caller
// (handleCommand) already ran Authorize before reaching here.
//
// One deliberate difference from handleLoadAdventure: a map never creates
// actors of its own (spec §3.4 — maps are terrain and placements, never a
// second source of actors), so a placement naming an actor that does not
// yet exist in campaign state is not caught here at all — it surfaces as an
// ordinary AppendBatch rejection ("token placed for unknown actor"), the
// same clean-error path a hand-issued place_token would take. The DM is
// expected to add_actor first; this handler does not special-case that
// order.
//
// mapdef.Compile's second return value is a []string of kind-mismatch
// warnings (spec §3.2: an override's kind disagreeing with its base tile's
// warns, never refuses). This handler discards them, deliberately — see the
// task report for the full reasoning; in short: (1) it matches the two
// existing production call sites, cmd/vtt/maps.go's boot-time dry run and
// internal/adventure/compile.go's asMap path, both of which already discard
// the identical warning at the point closest to Compile; (2) CommandResult
// (contract/vtt/v1/commands.proto) has no field to carry a warning list on
// an ok=true result, and adding one is a contract decision this task did not
// scope; (3) package gateway's core (this file's siblings) does no I/O of
// its own — there is no established logging channel here to write one to
// instead. A live, per-load warning surface for the DM is a reasonable
// follow-up if wanted, but is a new decision, not a silent drop repeated
// for no reason.
func (s *Server) handleLoadMap(requestID string, cmd *vttv1.LoadMap, p *identity.Participant) *vttv1.CommandResult {
	if len(s.maps) == 0 {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: errNoMapsAvailable}
	}
	m, ok := s.maps[cmd.GetMapId()]
	if !ok {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: fmt.Sprintf("gateway: unknown map %q", cmd.GetMapId())}
	}

	// s.packs[""] is a legal, deliberate no-op lookup (Go's zero-value map
	// read) for a map that declares no Pack — mapdef.Compile accepts a nil
	// *Pack precisely for that case (see its own doc comment).
	pack := s.packs[m.Pack]

	envs, _, err := mapdef.Compile(m, pack)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// mapdef.Compile leaves EventId/ParticipantId/ActorRole/OccurredAt zero
	// on every envelope it returns (the same convention adventure.Compile
	// follows for handleLoadAdventure) — stamp them here before handing the
	// batch to campaign.AppendBatch.
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
