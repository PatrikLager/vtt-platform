package campaign_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

const (
	qaIDScene     = "SceneCreated"
	qaIDActor     = "ActorAdded"
	qaIDToken     = "TokenPlaced"
	qaIDAdventure = "AdventureLoaded"
)

var qaIDKinds = []string{qaIDScene, qaIDActor, qaIDToken, qaIDAdventure}

type qaIDCase struct {
	label string
	id    string
}

func qaIDASCII(n int) string { return strings.Repeat("a", n) }

func qaIDEvent(kind, id string) *vttv1.Envelope {
	env := &vttv1.Envelope{EventId: "qa-id-event", Sequence: 1}
	switch kind {
	case qaIDScene:
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: "qa", GridWidth: 3, GridHeight: 3,
		}}
	case qaIDActor:
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: id, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}}}
	case qaIDToken:
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: id, SceneId: "qa-scene", ActorId: "qa-actor", Position: &vttv1.GridPosition{X: 1, Y: 1},
		}}
	case qaIDAdventure:
		env.Payload = &vttv1.Envelope_AdventureLoaded{AdventureLoaded: &vttv1.AdventureLoaded{
			AdventureId: id, Name: "qa",
		}}
	}
	return env
}

func qaIDHolds(st *engine.State, kind, id string) bool {
	switch kind {
	case qaIDScene:
		_, ok := st.Scenes[id]
		return ok
	case qaIDActor:
		_, ok := st.Actors[id]
		return ok
	case qaIDToken:
		_, ok := st.Tokens[id]
		return ok
	}
	return true
}

func qaIDLog(t *testing.T, envs ...*vttv1.Envelope) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	for i, env := range envs {
		env.Sequence = 0
		env.EventId = fmt.Sprintf("qa-log-%d", i)
		if _, err := s.Append(env); err != nil {
			t.Fatalf("store.Append %d: %v", i, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}
	return dir
}

func qaIDElide(msg, id string) string {
	if id == "" {
		return msg
	}
	return strings.ReplaceAll(msg, id, "<id>")
}

func qaIDLogFor(t *testing.T, kind, id string) string {
	t.Helper()
	var setup []*vttv1.Envelope
	if kind == qaIDToken {
		setup = []*vttv1.Envelope{qaIDEvent(qaIDScene, "qa-scene"), qaIDEvent(qaIDActor, "qa-actor")}
	}
	return qaIDLog(t, append(setup, qaIDEvent(kind, id))...)
}

// VTT-277 VTT-278
func TestQAIDACampaignWhoseLogHoldsAnIDOutsideTheBoundDoesNotOpen(t *testing.T) {
	for _, kind := range qaIDKinds {
		for _, c := range []qaIDCase{{"129 ascii", qaIDASCII(129)}, {"65 two-byte runes", strings.Repeat("é", 65)}, {"empty", ""}} {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				c2, err := campaign.Open(qaIDLogFor(t, kind, c.id))
				if err == nil {
					_ = c2.Close()
					t.Fatalf("campaign.Open accepted a log holding a %s id of %d bytes", kind, len(c.id))
				}
				t.Logf("refusal: %s", qaIDElide(err.Error(), c.id))
			})
		}
		t.Run(kind+"/64 two-byte runes", func(t *testing.T) {
			id := strings.Repeat("é", 64)
			c2, err := campaign.Open(qaIDLogFor(t, kind, id))
			if err != nil {
				t.Fatalf("campaign.Open refused a log holding a %s id of %d bytes: %v", kind, len(id), err)
			}
			defer func() { _ = c2.Close() }()
			if !qaIDHolds(c2.State(), kind, id) {
				t.Fatalf("the reopened campaign does not hold the %s id", kind)
			}
		})
	}
}
