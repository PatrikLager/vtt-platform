package gateway_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Keep these histories' events equal to internal/perceive's historyFolds.
type historyFold struct {
	name   string
	events []any
	why    string
}

func historyEnvelope(seq int64, p any) *vttv1.Envelope {
	e := &vttv1.Envelope{Sequence: seq, EventId: fmt.Sprintf("e%d", seq), SessionId: "s"}
	switch v := p.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: v}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: v}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: v}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: v}
	case *vttv1.TokenRemoved:
		e.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: v}
	case *vttv1.ActorRemoved:
		e.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: v}
	case *vttv1.ConditionApplied:
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: v}
	case *vttv1.ConditionRemoved:
		e.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: v}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: v}
	case *vttv1.ActorControlRevoked:
		e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: v}
	default:
		panic(fmt.Sprintf("historyEnvelope: no arm for %T", p))
	}
	return e
}

func historyGob(name, moduleID string) *vttv1.ActorAdded {
	return &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: name, ModuleId: moduleID,
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, Resources: map[string]*vttv1.Resource{"vigor": {Current: 5, Max: 5}}}}
}

func historyPlace(token string) *vttv1.TokenPlaced {
	return &vttv1.TokenPlaced{TokenId: token, SceneId: "s1", ActorId: "gob", Position: &vttv1.GridPosition{X: 3, Y: 1}}
}

// historyBase leaves hero's spectator seeing gob at (3,1) from (1,1), on a
// 5x5 scene with no wall.
func historyBase(more ...any) []any {
	return append([]any{
		&vttv1.SessionStarted{Name: "n"},
		&vttv1.SceneCreated{SceneId: "s1", Name: "s1", GridWidth: 5, GridHeight: 5},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		&vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s1", ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}},
		historyGob("Old Gob", ""),
		historyPlace("t-gob"),
	}, more...)
}

func historyFolds() []historyFold {
	reAdd := func(added *vttv1.ActorAdded) []any {
		return historyBase(&vttv1.TokenRemoved{TokenId: "t-gob"}, &vttv1.ActorRemoved{ActorId: "gob"},
			added, historyPlace("t-gob2"))
	}
	grant := func(p string, k vttv1.ActorKind) *vttv1.ActorControlGranted {
		return &vttv1.ActorControlGranted{ActorId: "gob", ParticipantId: p, Kind: k}
	}
	condition := func(id string) *vttv1.ConditionApplied {
		return &vttv1.ConditionApplied{ActorId: "gob", ConditionId: id}
	}
	uncondition := func(id string) *vttv1.ConditionRemoved {
		return &vttv1.ConditionRemoved{ActorId: "gob", ConditionId: id}
	}
	nonParty := vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	return []historyFold{
		{"reuse-identical", reAdd(historyGob("Old Gob", "")),
			"gob leaves sight with its token, is removed unseen and added again unchanged; its new token at (3,1) " +
				"brings it back. The viewer ends holding one gob, Old Gob at 5 of 5 vigor, and hero's token and gob's new " +
				"one on the board."},
		{"reuse-renamed", reAdd(historyGob("New Gob", "")),
			"As reuse-identical, but the actor added again is New Gob: the viewer ends holding the new name."},
		{"reuse-module-id", reAdd(historyGob("Old Gob", "other")),
			"As reuse-identical, but the actor added again carries module id other: the viewer ends holding it."},
		{"condition-order", historyBase(
			condition("A"), condition("B"), &vttv1.TokenRemoved{TokenId: "t-gob"},
			uncondition("A"), condition("C"), condition("A"), historyPlace("t-gob2"),
			&vttv1.TokenRemoved{TokenId: "t-gob2"}, uncondition("A"), uncondition("B"), historyPlace("t-gob3")),
			"The viewer sees A and B applied at 7 and 8. Unseen, the world removes A, applies C and A; on sight at " +
				"13 the viewer is told of C, so it holds A, B, C. Unseen again, the world removes A and B; on sight at " +
				"17 both are removed from the viewer. It ends holding C alone, applied at 13, when it was told."},
		{"controller-restatement", historyBase(
			grant("p1", nonParty), grant("p2", nonParty), &vttv1.TokenRemoved{TokenId: "t-gob"},
			&vttv1.ActorControlRevoked{ActorId: "gob", ParticipantId: "p1"}, grant("p1", nonParty), historyPlace("t-gob2"),
			&vttv1.TokenRemoved{TokenId: "t-gob2"}, grant("p2", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER), historyPlace("t-gob3")),
			"The viewer sees p1 then p2 granted, gob not a party member. Unseen, p1 is revoked and granted again: the " +
				"same set, so no correction on sight at 12. Unseen, a grant to p2 makes gob a party member; on sight at " +
				"15 one grant restates a controller with that kind. The viewer ends with p1 and p2, in the order it was " +
				"told, and gob a party member."},
	}
}

// canonicalJSON sorts object keys and keeps array order, the only order that matters.
func canonicalJSON(t *testing.T, b []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEachHistoryFoldAViewerIsSentIsTheOneDerivedByHand(t *testing.T) {
	for _, h := range historyFolds() {
		t.Run(h.name, func(t *testing.T) {
			pr := gateway.NewProjector(gateway.Viewer{ParticipantID: "w", Role: identity.RoleSpectator, Viewpoint: "hero"})
			var log, sent []*vttv1.Envelope
			for i, p := range h.events {
				log = append(log, historyEnvelope(int64(i+1), p))
				world, err := campaign.FoldPrefix(log)
				if err != nil {
					t.Fatalf("the world does not fold at %d: %v", i+1, err)
				}
				sent = append(sent, pr.Project(log[i], world)...)
			}
			shown, err := campaign.FoldPrefix(sent)
			if err != nil {
				t.Fatalf("the viewer's frames do not fold: %v", err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", "history-fold", h.name, "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			got := canonicalJSON(t, keystoneDump(t, shown, int64(len(log))))
			if !bytes.Equal(got, canonicalJSON(t, want)) {
				t.Errorf("%s\n\nthe viewer's fold is\n%s\nwant\n%s", h.why, got, canonicalJSON(t, want))
			}
		})
	}
}
