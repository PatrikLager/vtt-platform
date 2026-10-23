package gateway

import (
	"fmt"
	"sort"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// MayWatch reports whether p may watch actorID's log among actors (SPEC-015).
func MayWatch(p *identity.Participant, actorID string, actors map[string]*vttv1.Actor) error {
	ownOnly := false
	switch p.Role {
	case identity.RoleSpectator:
	case identity.RolePlayer:
		ownOnly = controlsAPartyMember(p.ID, actors)
	default:
		return fmt.Errorf("%w: role %q is sent the world and watches no character log",
			ErrUnauthorized, p.Role)
	}
	if actorID == "" {
		if ownOnly {
			return fmt.Errorf("%w: a player watches one of their own party members", ErrUnauthorized)
		}
		// Accept the empty id for a perch: it leaves a shoulder (VTT-345, VTT-452).
		return nil
	}
	a, ok := actors[actorID]
	if !ok || !engine.IsPartyMember(a) || (ownOnly && !controls(a, p.ID)) {
		// Answer every refusal of a named actor with this one string: a difference
		// enumerates the DM's cast or another player's party (VTT-347).
		return fmt.Errorf("%w: %q is not a party member you may watch", ErrUnauthorized, actorID)
	}
	return nil
}

func ownPartyMember(a *vttv1.Actor, participantID string) bool {
	return engine.IsPartyMember(a) && controls(a, participantID)
}

func controlsAPartyMember(participantID string, actors map[string]*vttv1.Actor) bool {
	for _, a := range actors {
		if ownPartyMember(a, participantID) {
			return true
		}
	}
	return false
}

// defaultViewpoint is p's first party member by id, or "" for a player who
// controls none and for every other role (VTT-340).
func defaultViewpoint(p *identity.Participant, actors map[string]*vttv1.Actor) string {
	if p.Role != identity.RolePlayer {
		return ""
	}
	var ids []string
	for id, a := range actors {
		if ownPartyMember(a, p.ID) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return ids[0]
}

// watchable is every party member MayWatch accepts for p, by id (VTT-399, VTT-454).
func watchable(p *identity.Participant, actors map[string]*vttv1.Actor) *vttv1.WatchableChanged {
	set := &vttv1.WatchableChanged{ControlsNoPartyMember: !controlsAPartyMember(p.ID, actors)}
	for id, a := range actors {
		if MayWatch(p, id, actors) == nil {
			set.Actors = append(set.Actors, &vttv1.WatchableActor{ActorId: id, Name: a.GetName()})
		}
	}
	sort.Slice(set.Actors, func(i, j int) bool { return set.Actors[i].GetActorId() < set.Actors[j].GetActorId() })
	return set
}
