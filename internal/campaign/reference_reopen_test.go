package campaign_test

import (
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-1036
func TestACampaignReopenedAfterReferencesAddedChangedAndRemovedResumesEveryLog(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c := openAt(t, dir)
	ally := listedMember("ally")
	ally.Actor.References = map[string]*vttv1.Reference{"r1": {EntryId: "echo", Place: "mike", State: "lima"}}
	must(t, c, cenv(nextID(), twoRooms()))
	must(t, c, cenv(nextID(), listedMember("hero")))
	must(t, c, cenv(nextID(), ally))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero", Position: at(1, 1)}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally", Position: at(2, 1)}))
	added := must(t, c, cenv(nextID(), &vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "r2",
		Reference: &vttv1.Reference{EntryId: "delta", Place: "kilo"}}))
	for _, p := range []any{
		&vttv1.TokenMoved{TokenId: "t-ally", SceneId: "s", From: at(2, 1), To: at(5, 1)},
		&vttv1.ReferenceChanged{ActorId: "ally", ReferenceId: "r1", Place: "mike", State: "oscar"},
		&vttv1.ReferenceRemoved{ActorId: "ally", ReferenceId: "r2"},
		&vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "r2", Reference: &vttv1.Reference{EntryId: "echo", Place: "mike", State: "lima"}},
		&vttv1.ReferenceAdded{ActorId: "ally", ReferenceId: "r3", Reference: &vttv1.Reference{EntryId: "delta", Place: "kilo"}},
		&vttv1.TokenMoved{TokenId: "t-ally", SceneId: "s", From: at(5, 1), To: at(2, 1)},
	} {
		must(t, c, cenv(nextID(), p))
	}
	for _, e := range logOf(t, c, "hero") {
		if e.WorldSeq < added {
			continue
		}
		if a := e.Env.GetActorAdded().GetActor().GetActorId(); a == "ally" || e.Env.GetActorRemoved().GetActorId() == "ally" {
			t.Errorf("at or after the reference added the hero's log was written %s", describe(e.Env))
		}
	}
	held, world := c.CharacterFoldForTest("hero").Actors["ally"].GetReferences(), c.State().Actors["ally"].GetReferences()
	if !proto.Equal(&vttv1.Actor{References: held}, &vttv1.Actor{References: world}) || len(world) != 3 {
		t.Fatalf("fixture: the hero's log holds the ally's references %v, the world %v; want both at r1, r2 and r3", held, world)
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
				*vttv1.Envelope_ReferenceAdded, *vttv1.Envelope_ReferenceRemoved,
				*vttv1.Envelope_ReferenceChanged:
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
