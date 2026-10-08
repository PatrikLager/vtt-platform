package campaign_test

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func qaCmdLog(t *testing.T, envs ...*vttv1.Envelope) string {
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
		env.EventId = fmt.Sprintf("qa-cmd-log-%d", i)
		if _, err := s.Append(env); err != nil {
			t.Fatalf("store.Append %d: %v", i, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("store.Close: %v", err)
	}
	return dir
}

func qaCmdScene(id string, objectIDs ...string) *vttv1.Envelope {
	objs := make([]*vttv1.SceneObject, 0, len(objectIDs))
	var col int32
	for _, oid := range objectIDs {
		objs = append(objs, &vttv1.SceneObject{
			ObjectId: oid, Kind: "crate", At: &vttv1.GridPosition{X: col, Y: 0}, Width: 1, Height: 1, Art: "qa-crate",
		})
		col++
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: id, Name: "qa", GridWidth: 4, GridHeight: 4, Objects: objs,
	}}}
}

func qaCmdActor(id string, edit func(*vttv1.Actor)) *vttv1.Envelope {
	a := &vttv1.Actor{ActorId: id, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}
	if edit != nil {
		edit(a)
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qaCmdGrant(participantID string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "qa-a", ParticipantId: participantID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		},
	}}
}

func qaCmdRevoke(participantID string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{
		ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: "qa-a", ParticipantId: participantID},
	}}
}

func qaCmdSession(sessionID string) *vttv1.Envelope {
	return &vttv1.Envelope{SessionId: sessionID, Payload: &vttv1.Envelope_SessionStarted{
		SessionStarted: &vttv1.SessionStarted{Name: "qa"},
	}}
}

func qaCmdBoard(sceneID string) []*vttv1.Envelope {
	return []*vttv1.Envelope{
		qaCmdScene(sceneID), qaCmdActor("qa-a", nil),
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "qa-t", SceneId: sceneID, ActorId: "qa-a", Position: &vttv1.GridPosition{X: 1, Y: 1},
		}}},
	}
}

func qaCmdMove(sceneID string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: "qa-t", SceneId: sceneID, From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1},
	}}}
}

type qaCmdLogCase struct {
	label string
	log   func(v string) []*vttv1.Envelope
	holds func(st *engine.State, v string) bool
}

func qaCmdLogCases() []qaCmdLogCase {
	actorWith := func(edit func(*vttv1.Actor, string)) func(string) []*vttv1.Envelope {
		return func(v string) []*vttv1.Envelope {
			return []*vttv1.Envelope{qaCmdActor("qa-a", func(a *vttv1.Actor) { edit(a, v) })}
		}
	}
	return []qaCmdLogCase{
		{
			"grant participant id",
			func(v string) []*vttv1.Envelope { return []*vttv1.Envelope{qaCmdActor("qa-a", nil), qaCmdGrant(v)} },
			func(st *engine.State, v string) bool {
				return slices.Contains(st.Actors["qa-a"].GetControllerIds(), v)
			},
		},
		{
			"revoke participant id",
			func(v string) []*vttv1.Envelope {
				return []*vttv1.Envelope{qaCmdActor("qa-a", nil), qaCmdGrant("qa-p"), qaCmdRevoke(v)}
			},
			func(st *engine.State, _ string) bool { return st.Actors["qa-a"] != nil },
		},
		{
			"module id",
			actorWith(func(a *vttv1.Actor, v string) { a.ModuleId = v }),
			func(st *engine.State, v string) bool { return st.Actors["qa-a"].GetModuleId() == v },
		},
		{
			"resource name",
			actorWith(func(a *vttv1.Actor, v string) {
				a.Resources = map[string]*vttv1.Resource{v: {Current: 1, Max: 2}}
			}),
			func(st *engine.State, v string) bool { _, ok := st.Actors["qa-a"].GetResources()[v]; return ok },
		},
		{
			"attribute name",
			actorWith(func(a *vttv1.Actor, v string) { a.Attributes = map[string]int32{v: 3} }),
			func(st *engine.State, v string) bool { _, ok := st.Actors["qa-a"].GetAttributes()[v]; return ok },
		},
		{
			"object id",
			func(v string) []*vttv1.Envelope { return []*vttv1.Envelope{qaCmdScene("qa-s", "qa-1", v)} },
			func(st *engine.State, v string) bool {
				objs := st.Scenes["qa-s"].Objects
				return len(objs) == 2 && objs[1].ObjectID == v
			},
		},
		{
			"session id",
			func(v string) []*vttv1.Envelope { return []*vttv1.Envelope{qaCmdSession(v)} },
			func(st *engine.State, v string) bool { return len(st.Sessions) == 1 && st.Sessions[0].ID == v },
		},
		{
			"move scene id",
			func(v string) []*vttv1.Envelope { return append(qaCmdBoard(v), qaCmdMove(v)) },
			func(st *engine.State, _ string) bool { return st.Tokens["qa-t"].X == 2 },
		},
	}
}

// VTT-289 VTT-290 VTT-291 VTT-293
func TestQACmdACampaignWhoseLogHoldsAnIDOverTheBoundDoesNotOpen(t *testing.T) {
	for _, c := range qaCmdLogCases() {
		for _, v := range []string{strings.Repeat("a", 129), strings.Repeat("é", 65)} {
			t.Run(fmt.Sprintf("%s/%d bytes", c.label, len(v)), func(t *testing.T) {
				envs := c.log(v)
				if c.label == "move scene id" {
					envs = append(qaCmdBoard("qa-s"), qaCmdMove(v))
				}
				c2, err := campaign.Open(qaCmdLog(t, envs...))
				if err == nil {
					_ = c2.Close()
					t.Fatalf("campaign.Open accepted a log holding a %d-byte %s", len(v), c.label)
				}
				msg := strings.ReplaceAll(err.Error(), v, "<v>")
				t.Logf("refusal: %s", msg)
				if !strings.Contains(msg, "128") || !strings.Contains(msg, strconv.Itoa(len(v))) {
					t.Fatalf("refusal %q does not name the bound and the %d-byte length", msg, len(v))
				}
			})
		}
	}
}

// VTT-289 VTT-290 VTT-291 VTT-293
func TestQACmdACampaignWhoseLogHoldsIDsAtTheBoundOpensWhole(t *testing.T) {
	for _, c := range qaCmdLogCases() {
		for _, v := range []string{strings.Repeat("😀", 32), "S" + strings.Repeat("a", 126) + "E"} {
			t.Run(fmt.Sprintf("%s/%d bytes", c.label, len(v)), func(t *testing.T) {
				c2, err := campaign.Open(qaCmdLog(t, c.log(v)...))
				if err != nil {
					t.Fatalf("campaign.Open refused a log holding a %d-byte %s: %v", len(v), c.label, err)
				}
				defer func() { _ = c2.Close() }()
				if !c.holds(c2.State(), v) {
					t.Fatalf("the reopened campaign does not hold the %d-byte %s", len(v), c.label)
				}
			})
		}
	}
}

// VTT-292 VTT-297
func TestQACmdACampaignWhoseLogHoldsAnEmptyOrRepeatedIDDoesNotOpen(t *testing.T) {
	empty := regexp.MustCompile(`128.*\b0\b`)
	cases := []struct {
		label string
		envs  []*vttv1.Envelope
		want  *regexp.Regexp
	}{
		{"empty object id", []*vttv1.Envelope{qaCmdScene("qa-s", "qa-1", "")}, empty},
		{"repeated object id", []*vttv1.Envelope{qaCmdScene("qa-s", "qa-1", "qa-2", "qa-1")},
			regexp.MustCompile(`engine: object "qa-1" appears twice in scene "qa-s"$`)},
		{"empty session id", []*vttv1.Envelope{qaCmdSession("")}, empty},
		{"empty move scene id", append(qaCmdBoard("qa-s"), qaCmdMove("")), empty},
		{"empty resource name", []*vttv1.Envelope{qaCmdActor("qa-a", func(a *vttv1.Actor) {
			a.Resources = map[string]*vttv1.Resource{"": {Current: 1, Max: 2}}
		})}, empty},
		{"empty attribute name", []*vttv1.Envelope{qaCmdActor("qa-a", func(a *vttv1.Actor) {
			a.Attributes = map[string]int32{"": 3}
		})}, empty},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			c2, err := campaign.Open(qaCmdLog(t, c.envs...))
			if err == nil {
				_ = c2.Close()
				t.Fatalf("campaign.Open accepted a log holding an %s", c.label)
			}
			t.Logf("refusal: %s", err)
			if !c.want.MatchString(err.Error()) {
				t.Fatalf("refusal %q does not match %s", err, c.want)
			}
		})
	}
}
