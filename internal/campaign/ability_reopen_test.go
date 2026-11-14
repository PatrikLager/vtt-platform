package campaign_test

import (
	"path/filepath"
	"slices"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func listedMember(id string, abilities ...string) *vttv1.ActorAdded {
	return &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: id, Name: id,
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, AbilityIds: abilities}}
}

func sameSetOf(a, b []string) bool {
	for _, id := range a {
		if !slices.Contains(b, id) {
			return false
		}
	}
	for _, id := range b {
		if !slices.Contains(a, id) {
			return false
		}
	}
	return true
}

// VTT-742 VTT-744
func TestACampaignReopenedAfterAGiveResumesEveryLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	must(t, c, cenv(nextID(), twoRooms()))
	must(t, c, cenv(nextID(), listedMember("hero", "rally")))
	must(t, c, cenv(nextID(), listedMember("ally", "rally", "fists")))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally", Position: at(2, 1)}))
	given := must(t, c, cenv(nextID(), &vttv1.AbilityGiven{ActorId: "ally", AbilityId: "dodge"}))
	for _, p := range []any{
		&vttv1.TokenMoved{TokenId: "t-ally", SceneId: "s", From: at(2, 1), To: at(5, 1)},
		&vttv1.AbilityTaken{ActorId: "ally", AbilityId: "rally"},
		&vttv1.AbilityGiven{ActorId: "ally", AbilityId: "rally"},
		&vttv1.TokenMoved{TokenId: "t-ally", SceneId: "s", From: at(5, 1), To: at(2, 1)},
	} {
		must(t, c, cenv(nextID(), p))
	}
	for _, e := range logOf(t, c, "hero") {
		if e.WorldSeq < given {
			continue
		}
		if a := e.Env.GetActorAdded().GetActor().GetActorId(); a == "ally" || e.Env.GetActorRemoved().GetActorId() == "ally" {
			t.Errorf("at or after the give the hero's log was written %s", describe(e.Env))
		}
	}
	held, world := c.CharacterFoldForTest("hero").Actors["ally"].GetAbilityIds(), c.State().Actors["ally"].GetAbilityIds()
	if !sameSetOf(held, world) || slices.Equal(held, world) {
		t.Fatalf("fixture: the hero's log holds the ally naming %q, the world %q; want the same ids in another order", held, world)
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
				*vttv1.Envelope_AbilityGiven, *vttv1.Envelope_AbilityTaken:
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
