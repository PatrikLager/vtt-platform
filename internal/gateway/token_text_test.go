package gateway_test

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// VTT-465 VTT-462
func TestAPlayerIsSentNoFrameNamingACreatureItSees(t *testing.T) {
	f := newGWFixture(t)
	mustAppend(t, f.campaign, "gob-actor", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a-gob", Name: "Goblin Cutter",
			Kind:      vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			Resources: map[string]*vttv1.Resource{"vigor": {Current: 5, Max: 5}}},
	}})
	mustAppend(t, f.campaign, "gob-token", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "t-gob", SceneId: "scn1", ActorId: "a-gob", Position: &vttv1.GridPosition{X: 4, Y: 7},
		Name: "a hunched shape", GmName: "the chief's son",
	}})

	player := drainEvents(t, f.dial(f.playerToken, 0), 500*time.Millisecond)
	shown := false
	for _, e := range player {
		if e.GetTokenPlaced().GetTokenId() == "t-gob" && e.GetTokenPlaced().GetName() == "a hunched shape" {
			shown = true
		}
	}
	if !shown {
		t.Fatal("the player was not sent the creature's token by its name, so nothing here was tested")
	}
	dm := drainEvents(t, f.dial(f.dmToken, 0), 500*time.Millisecond)
	for _, needle := range []string{"Goblin Cutter", "vigor", "the chief's son"} {
		if mentions(t, player, needle) {
			t.Errorf("the player was sent a frame holding %q", needle)
		}
		if !mentions(t, dm, needle) {
			t.Errorf("the DM was not sent %q", needle)
		}
	}
}
