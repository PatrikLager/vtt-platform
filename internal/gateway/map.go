package gateway

import (
	"errors"
	"fmt"
	"io/fs"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// errNoMapsAvailable is handleLoadMap's clean, ok=false error when s has no
// maps configured — errNoAdventuresAvailable's exact sibling (adventure.go),
// for the same reason: a campaign whose maps/ is absent, or which has no
// maps installed yet (2026-09-01-create-scene-leaves Task 5), keeps
// load_map rejected with a clean "no maps available" rather than a
// connection drop or crash.
const errNoMapsAvailable = "gateway: no maps available"

// handleLoadMap runs the authorized-load_map pipeline (whole-branch-review
// C1 remediation, maps-as-geometry design spec §4.3): lookup by id via
// mapByID (below — the map set, and on a miss the campaign's own maps/
// directory, since 2026-09-01-create-scene-leaves Task 6), mapdef.Compile
// against the map's own pack (s.packs, keyed by the map's declared Pack id —
// may legally be nil/absent for a map with no overrides, see mapdef.Compile's
// own doc comment), then one campaign.AppendBatch for the whole ordered
// event batch Compile returns. This mirrors handleLoadAdventure
// (adventure.go) almost exactly: every failure — no maps configured, an
// unknown id, a map installed but broken, a Compile error, an AppendBatch
// rejection — is a clean ok=false CommandResult, never a connection drop.
// The caller (handleCommand) already ran Authorize before reaching here.
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
// an ok=true result, and adding one is a contract decision the
// maps-as-geometry task did not scope; (3) this package has no logging
// channel to write one to instead — which used to be stated here as
// "package gateway's core does no I/O of its own", and that is no longer
// true of this very file (mapByID reads a map off disk since
// 2026-09-01-create-scene-leaves Task 6). The absence of a logging channel
// is what the reasoning actually rested on, and it still holds. A live, per-load warning surface for the DM is a reasonable
// follow-up if wanted, but is a new decision, not a silent drop repeated
// for no reason.
func (s *Server) handleLoadMap(requestID string, cmd *vttv1.LoadMap, p *identity.Participant) *vttv1.CommandResult {
	m, lookupErr := s.mapByID(cmd.GetMapId())
	if lookupErr != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: lookupErr.Error()}
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

// mapByID answers with the map called id, loading it from the campaign's
// own maps/ directory if the set does not already hold it — the on-demand
// lookup this sub-project exists for (2026-09-01-create-scene-leaves design
// spec §5). Install and load are two separate acts (§4): a map file appears
// in maps/ by whatever means, and load_map brings it into play, with no
// restart and no reconnect in between. Boot-time preloading is unchanged —
// it is step 1 having happened early — so an operator still learns about a
// broken map before anyone connects.
//
// A server with no maps directory configured (s.mapsDir == "") keeps
// exactly the behaviour this handler had before: its map set is whatever
// WithMaps was given, an unknown id is unknown, and an empty set answers
// errNoMapsAvailable. That is not a legacy branch — it is what any caller
// that wires WithMaps without WithMapsDir gets, and it is the state
// map_test.go's own newMapFixture still pins.
//
// THE SHAPE OF THE LOAD is deliberate, and the three steps are not
// interchangeable:
//
//  1. Read under the read lock, and RELEASE it before doing anything else.
//  2. Load from disk holding NO lock. Reading and compiling a map is I/O
//     plus the whole of mapdef's validation; holding the write lock across
//     it would stall every concurrent reader of the map set — including
//     GET /api/maps — for as long as the disk takes.
//  3. Take the write lock and CHECK AGAIN before inserting. Another
//     goroutine may have finished the same load while this one was reading
//     the file (design spec §5: "Two load_maps racing on the same new id
//     must not both compile and cache it"). Whoever gets there first owns
//     the entry, and everyone else adopts it, so every caller holds the
//     same *mapdef.Map and the set gains exactly one — proven in
//     map_internal_test.go, which is the only place that property is
//     visible.
//
// s.packs is read without any lock because packs are still boot-time only
// (Server.packs' own doc comment): WithMaps sets them before the server
// serves anything, and nothing writes them afterwards. That is also the
// limit of what this lookup covers, deliberately — design spec §5 asks for
// maps on demand and says nothing about packs. A map installed together
// with a NEW pack is therefore refused until the server restarts, and THIS
// handler is the only layer that knows that is the remedy, so it is the one
// that says so: mapdef reports the pack is not among those loaded
// (mapdef.ErrPackNotLoaded) because at boot that means it is not installed,
// and only here does it also mean "installed since we started reading".
// Round 1 of 2026-09-01-create-scene-leaves Task 6 let mapdef's raw
// "no pack was given to resolve it"
// reach the DM, which is true of the function call and false about the
// world — a DM reading it goes and checks the pack field on a map that is
// correct.
//
// WHAT REACHES A CLIENT, and it is a narrower question than it looks:
// mapdef.LoadInstalled names every file it complains about as
// "maps/<id>.json" rather than the path it opened, so no error from this
// probe carries the server's absolute layout. That is a property of THAT
// function, deliberately, and not of a translation here — round 1 of that
// task translated only fs.ErrNotExist and forwarded the rest, and the rest
// includes an *fs.PathError for ENAMETOOLONG (an id of 260 characters, from
// any seat that can issue load_map) and every field error from an ordinary
// map with a typo in it. Both leaked the path. On top of that guarantee,
// two errors are given more here: a map that is not installed becomes an
// ordinary unknown-map answer, and a pack that is not loaded gains the
// restart. Everything else is forwarded verbatim, because a broken map is a
// thing the DM has to act on and mapdef already says it best.
func (s *Server) mapByID(id string) (*mapdef.Map, error) {
	s.mapsMu.RLock()
	m, ok := s.maps[id]
	loaded := len(s.maps)
	s.mapsMu.RUnlock()
	if ok {
		return m, nil
	}

	if s.mapsDir == "" {
		if loaded == 0 {
			return nil, errors.New(errNoMapsAvailable)
		}
		return nil, fmt.Errorf("gateway: unknown map %q", id)
	}

	installed, err := mapdef.LoadInstalled(s.mapsDir, id, s.packs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf(
				"gateway: unknown map %q: nothing installed at maps/%s.json in this campaign", id, id)
		}
		if errors.Is(err, mapdef.ErrPackNotLoaded) {
			return nil, fmt.Errorf("%w. Packs are read once, at startup, so a pack "+
				"installed since then is not available until this server restarts", err)
		}
		return nil, err
	}

	s.mapsMu.Lock()
	defer s.mapsMu.Unlock()
	if won, ok := s.maps[id]; ok {
		return won, nil
	}
	if s.maps == nil {
		s.maps = make(map[string]*mapdef.Map, 1)
	}
	s.maps[id] = installed
	return installed, nil
}
