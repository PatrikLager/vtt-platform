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
	switch p.Role {
	case identity.RoleSpectator:
		if actorID == "" {
			// Accept the empty id: it is how a spectator leaves a shoulder (VTT-345).
			return nil
		}
	case identity.RolePlayer:
		if actorID == "" {
			return fmt.Errorf("%w: a player watches one of their own party members", ErrUnauthorized)
		}
	default:
		return fmt.Errorf("%w: role %q is sent the world and watches no character log",
			ErrUnauthorized, p.Role)
	}
	a, ok := actors[actorID]
	if !ok || !engine.IsPartyMember(a) || (p.Role == identity.RolePlayer && !controls(a, p.ID)) {
		// Answer every refusal of a named actor with this one string: a difference
		// enumerates the DM's cast or another player's party (VTT-347).
		return fmt.Errorf("%w: %q is not a party member you may watch", ErrUnauthorized, actorID)
	}
	return nil
}

// defaultViewpoint is p's first party member by id, or "" for a player who
// controls none and for every other role (VTT-340).
func defaultViewpoint(p *identity.Participant, actors map[string]*vttv1.Actor) string {
	if p.Role != identity.RolePlayer {
		return ""
	}
	var ids []string
	for id, a := range actors {
		if engine.IsPartyMember(a) && controls(a, p.ID) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return ""
	}
	sort.Strings(ids)
	return ids[0]
}

// watchable is every party member MayWatch accepts for p, by id (VTT-399).
func watchable(p *identity.Participant, actors map[string]*vttv1.Actor) *vttv1.WatchableChanged {
	set := &vttv1.WatchableChanged{}
	for id, a := range actors {
		if MayWatch(p, id, actors) == nil {
			set.Actors = append(set.Actors, &vttv1.WatchableActor{ActorId: id, Name: a.GetName()})
		}
	}
	sort.Slice(set.Actors, func(i, j int) bool { return set.Actors[i].GetActorId() < set.Actors[j].GetActorId() })
	return set
}
