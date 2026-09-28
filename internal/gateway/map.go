package gateway

import (
	"errors"
	"fmt"
	"io/fs"

	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

const errNoMapsAvailable = "gateway: no maps available"

// Answer every failure as an ok=false result, never a close: authorization
// already ran in handleCommand (SPEC-014).
func (s *Server) handleLoadMap(requestID string, cmd *vttv1.LoadMap, p *identity.Participant) *vttv1.CommandResult {
	m, lookupErr := s.mapByID(cmd.GetMapId())
	if lookupErr != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: lookupErr.Error()}
	}

	envs, warnings, err := mapdef.Compile(m, s.artDir)
	if err != nil {
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}

	// Stamp the four fields Compile leaves zero: store.AppendBatch requires an
	// EventId (SPEC-014).
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
		// Translate this sentinel and no other: a DM sent to copy a map over an
		// unrelated fault cannot act on it (TestANonCollisionFailureKeepsItsOwnMessage).
		//
		// Say the scene id is in play, never that the map is loaded: a loaded
		// adventure can hold the id too
		// (TestASecondLoadTellsTheDMTheMapIsLoadedAndHowToReload).
		if errors.Is(err, engine.ErrSceneExists) {
			return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: fmt.Sprintf(
				"gateway: load_map: scene id %q is already in play — a map's scene id is its "+
					"own id, and a loaded adventure can claim one too; to load this map as "+
					"well, install a copy under a new id and load that",
				cmd.GetMapId())}
		}
		return &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}
	}
	// Return the warnings to the issuer only; nothing broadcasts them (SPEC-014).
	return &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: firstSeq, Warnings: warnings}
}

// mapByID is the lookup SPEC-014 states: the set, then the campaign's maps
// directory on a miss.
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

	// Hold no lock across this: it reads the disk, and GET /api/maps reads the set.
	installed, err := mapdef.LoadInstalled(s.mapsDir, id, s.artDir)
	if err != nil {
		// Translate not-installed alone and forward the rest: LoadInstalled names
		// maps/<id>.json and never the path it opened (SPEC-014).
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf(
				"gateway: unknown map %q: nothing installed at maps/%s.json in this campaign", id, id)
		}
		return nil, err
	}

	s.mapsMu.Lock()
	defer s.mapsMu.Unlock()
	// Look again under the write lock: racing lookups must end on one map
	// (TestConcurrentLookupsOfANewlyInstalledMapCompileItOnce).
	if won, ok := s.maps[id]; ok {
		return won, nil
	}
	if s.maps == nil {
		s.maps = make(map[string]*mapdef.Map, 1)
	}
	s.maps[id] = installed
	return installed, nil
}
