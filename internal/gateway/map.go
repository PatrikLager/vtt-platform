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
// against the campaign's flat art directory (s.artDir, set by WithArtDir —
// art is read at load time now, not once at boot:
// 2026-09-02-art-is-a-flat-library design spec §3.6, and an unresolved
// reference degrades one square rather than refusing the map, §4), then one
// campaign.AppendBatch for the whole ordered
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
// mapdef.Compile's second return value is a []string of warnings. There are
// four producers, and only the first predates 2026-09-02-art-is-a-flat-library
// Task 3: an override's kind disagreeing with its base tile's
// (2026-08-12-maps-as-geometry design spec §3.2 — warns, never refuses), art
// that is not installed, art that has a picture and no sidecar, and an art
// directory this process cannot open (all three that sub-project's design spec
// §4). Each is reported ONCE per distinct message with the number of squares
// or objects it affected, because the un-deduplicated version put 96 warnings
// and 6840 bytes on one result for the shipped cellar map — see
// mapdef.BuildSceneCreated's warningTally. This handler carries them onto the
// ok=true CommandResult it
// returns (CommandResult.warnings, field 5, added by
// 2026-09-02-art-is-a-flat-library's Task 2) — the channel this doc comment
// used to explain the absence of. They go on THIS result and nowhere else:
// warnings are for whoever issued load_map, not for the table, so nothing
// here broadcasts them — serve's own read loop (server.go) is what makes
// that automatic, writing the CommandResult answerCommand returns to only
// THIS connection's own outCh.
//
// This used to say "the other two production call sites" discard the
// identical warning. That miscounted them and misplaced one.
// cmd/vtt/maps.go never calls Compile or BuildSceneCreated itself — its
// boot-time walk reaches the discard INDIRECTLY, inside
// mapdef.LoadInstalled's own dry-run Compile call (installed.go). mapByID
// (below) calls that exact same LoadInstalled on a cache miss, which makes
// it not an "other" site at all from here: a map loaded for the first time
// through THIS handler has its warnings computed twice in one request —
// once inside LoadInstalled's dry run, discarded, and once by the Compile
// call below, kept.
//
// THE ADVENTURE SIDE NO LONGER DISCARDS ANYTHING ON ITS LIVE PATH, and this
// paragraph said the opposite until 2026-09-03. It read: "adventure.Compile's
// signature is ([]*vttv1.Envelope, error) and swallows the warnings internally
// -- so widening any of them means changing a signature that drops the warning
// before a CommandResult is ever in scope, which is a decision this task did
// not scope." That was a correct description of a gap and an incorrect
// description of its cost. Once art-is-a-flat-library Task 3 made unresolvable
// art WARN instead of refusing, the swallowed warning became the only thing
// that would have been said at all, so the signature was widened:
// adventure.Compile returns ([]*vttv1.Envelope, []string, error) and
// handleLoadAdventure (adventure.go) puts them on its own CommandResult, the
// same way this handler does.
//
// internal/adventure/load.go's loadScenes still discards them in ITS dry run,
// at adventure-LOAD time, and that one is genuinely fine: it runs at boot with
// no CommandResult anywhere, and every warning it drops is recomputed on the
// live path a moment later.
func (s *Server) handleLoadMap(requestID string, cmd *vttv1.LoadMap, p *identity.Participant) *vttv1.CommandResult {
	m, lookupErr := s.mapByID(cmd.GetMapId())
	if lookupErr != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: lookupErr.Error()}
	}

	envs, warnings, err := mapdef.Compile(m, s.artDir)
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
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: firstSeq, Warnings: warnings}
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
// THE PACK-NOT-LOADED ANSWER IS GONE from here, with the mechanism that
// produced it (2026-09-02-art-is-a-flat-library Task 3). It existed because
// packs were read once at boot, so a map installed together with a new pack
// was refused until a restart and this handler was the only layer that knew
// so. Art is now read from a directory when the map is loaded (design spec
// §3.6), so nothing can be "installed but not loaded" and no answer here has
// a restart to suggest.
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

	installed, err := mapdef.LoadInstalled(s.mapsDir, id, s.artDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf(
				"gateway: unknown map %q: nothing installed at maps/%s.json in this campaign", id, id)
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
