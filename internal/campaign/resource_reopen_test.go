package campaign_test

import (
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-781 VTT-782
func TestACampaignReopenedAfterResourcesAddedAndRemovedResumesEveryLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	ally := listedMember("ally")
	ally.Actor.Resources = map[string]*vttv1.Resource{"pool": {Current: 5, Max: 9}}
	must(t, c, cenv(nextID(), twoRooms()))
	must(t, c, cenv(nextID(), listedMember("hero")))
	must(t, c, cenv(nextID(), ally))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally", Position: at(2, 1)}))
	added := must(t, c, cenv(nextID(), &vttv1.ResourceAdded{ActorId: "ally", Resource: "focus",
		Value: &vttv1.Resource{Current: 1, Max: 2}}))
	for _, p := range []any{
		&vttv1.TokenMoved{TokenId: "t-ally", SceneId: "s", From: at(2, 1), To: at(5, 1)},
		&vttv1.ResourceRemoved{ActorId: "ally", Resource: "pool"},
		&vttv1.ResourceAdded{ActorId: "ally", Resource: "pool", Value: &vttv1.Resource{Current: 2, Max: 4}},
		&vttv1.TokenMoved{TokenId: "t-ally", SceneId: "s", From: at(5, 1), To: at(2, 1)},
	} {
		must(t, c, cenv(nextID(), p))
	}
	for _, e := range logOf(t, c, "hero") {
		if e.WorldSeq < added {
			continue
		}
		if a := e.Env.GetActorAdded().GetActor().GetActorId(); a == "ally" || e.Env.GetActorRemoved().GetActorId() == "ally" {
			t.Errorf("at or after the resource added the hero's log was written %s", describe(e.Env))
		}
	}
	held, world := c.CharacterFoldForTest("hero").Actors["ally"].GetResources(), c.State().Actors["ally"].GetResources()
	if len(held) != 2 || held["pool"].GetMax() != 4 || len(world) != 2 {
		t.Fatalf("fixture: the hero's log holds the ally's resources %v, the world %v; want both at pool 2/4 and focus", held, world)
	}
	kept := map[string]int{"hero": len(logOf(t, c, "hero")), "ally": len(logOf(t, c, "ally"))}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	c = openAt(t, dir)
	must(t, c, cenv(nextID(), &vttv1.AttackRolled{AttackerId: "hero", TargetId: "ally", Total: 7}))
	for id, n := range kept {
		got := logOf(t, c, id)
		after := got[n:]
		attacks := 0
		for _, e := range after {
			if e.Env.GetAttackRolled() != nil && e.Env.GetEventId() != "" {
				attacks++
			}
			switch e.Env.GetPayload().(type) {
			case *vttv1.Envelope_SceneCreated, *vttv1.Envelope_ActorAdded, *vttv1.Envelope_ActorRemoved,
				*vttv1.Envelope_TokenPlaced, *vttv1.Envelope_TokenHidden,
				*vttv1.Envelope_ResourceAdded, *vttv1.Envelope_ResourceRemoved:
				t.Errorf("after the reopen %s's log was written %s, as if it started afresh", id, describe(e.Env))
			}
		}
		if attacks != 1 {
			t.Errorf("after the reopen %s's log was written %v, want the attack forwarded once", id, kindsOf(after))
		}
		assertFoldsAtEveryPrefix(t, id+"'s log", got)
	}
	assertNoneDropped(t, c)
}
