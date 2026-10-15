package gateway

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// VTT-314
func TestAProjectorWhoseViewerFoldFailsSendsNothing(t *testing.T) {
	log := []*vttv1.Envelope{
		{Sequence: 1, SessionId: "s", Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "n"}}},
		{Sequence: 2, Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "s1", Name: "s1", GridWidth: 5, GridHeight: 5}}},
		{Sequence: 3, Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "hero", Name: "hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}},
		{Sequence: 4, Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "t-hero", SceneId: "s1", ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}}}},
	}
	feed := func(poison bool) []*vttv1.Envelope {
		pr := NewProjector(Viewer{ParticipantID: "w", Role: identity.RoleSpectator, Viewpoint: "hero"})
		var out []*vttv1.Envelope
		for i, env := range log {
			world, err := campaign.FoldPrefix(log[:i+1])
			if err != nil {
				t.Fatalf("the world does not fold at %d: %v", i+1, err)
			}
			if poison && i == len(log)-1 {
				// A token on a scene the viewer was never shown: its fold refuses it.
				pr.sent = append(pr.sent, &vttv1.Envelope{Sequence: 3, Payload: &vttv1.Envelope_TokenPlaced{
					TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-x", SceneId: "nowhere", ActorId: "hero"}}})
			}
			out = pr.Project(env, world)
		}
		return out
	}
	if len(feed(false)) == 0 {
		t.Fatal("the control sends nothing for the token that brings hero's board, so the check below proves nothing")
	}
	if got := feed(true); len(got) != 0 {
		t.Errorf("a projector whose viewer's fold fails sent %d frames, want none", len(got))
	}
}
