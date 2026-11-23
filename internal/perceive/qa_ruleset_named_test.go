package perceive_test

import (
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// Keep one party member in sight of a token and one with none (VTT-980).
func qarnWorld() []*vttv1.Envelope {
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	envs := []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "Evening"}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "sc-1", Name: "Hall", GridWidth: 6, GridHeight: 5}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a-1", Name: "Ada", Kind: party, Attributes: map[string]int32{"k1": 3}}}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "c-1", Name: "Cleo", Kind: party}}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "b-1", Name: "Bram", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}}},
		{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: "a-1", ParticipantId: "p-1", Kind: party}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-1", SceneId: "sc-1", ActorId: "a-1", Position: &vttv1.GridPosition{X: 1, Y: 2}}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-2", SceneId: "sc-1", ActorId: "b-1", Position: &vttv1.GridPosition{X: 4, Y: 3}}}},
		{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{ActorId: "a-1", ConditionId: "cond-1", Source: "manual"}}},
	}
	for i, e := range envs {
		qarnStamp(e, int64(i+1))
	}
	return envs
}

func qarnStamp(e *vttv1.Envelope, seq int64) {
	e.Sequence = seq
	e.EventId = fmt.Sprintf("ev-%d", seq)
	e.SessionId = "s-1"
	e.OccurredAt = timestamppb.New(time.Date(2026, 11, 23, 12, 0, int(seq), 0, time.UTC))
	e.ParticipantId = "p-dm"
	e.ActorRole = "dm"
}

func qarnRulesetNamed(seq int64, id, version, name string) *vttv1.Envelope {
	e := &vttv1.Envelope{Payload: &vttv1.Envelope_RulesetNamed{RulesetNamed: &vttv1.RulesetNamed{RulesetId: id, Version: version, Name: name}}}
	qarnStamp(e, seq)
	return e
}

type qarnViewer struct {
	label string
	eyes  perceive.Eyes
	pr    *perceive.Projection
	shown *engine.State
}

func qarnViewers() []*qarnViewer {
	vs := []*qarnViewer{
		{label: "eyes that see a lit scene", eyes: perceive.Eyes{Shoulder: "a-1"}},
		{label: "eyes with no token anywhere", eyes: perceive.Eyes{Shoulder: "c-1"}},
		{label: "no eyes at all", eyes: perceive.Eyes{}},
		{label: "eyes through a controller", eyes: perceive.Eyes{Controller: "p-1"}},
	}
	for _, v := range vs {
		v.pr = perceive.New(v.eyes, nil)
		v.shown = engine.NewState()
	}
	return vs
}

func qarnSend(t *testing.T, world *engine.State, vs []*qarnViewer, env *vttv1.Envelope) map[string][]*vttv1.Envelope {
	t.Helper()
	if err := engine.Apply(world, env); err != nil {
		t.Fatalf("world refuses event %d: %v", env.Sequence, err)
	}
	sent := map[string][]*vttv1.Envelope{}
	for _, v := range vs {
		frames := v.pr.Event(v.eyes, env, v.shown, world)
		for _, f := range frames {
			// VTT-272
			if f.ParticipantId != "" || f.ActorRole != "" {
				t.Errorf("%s: event %d: a frame carries participant %q, role %q", v.label, env.Sequence, f.ParticipantId, f.ActorRole)
			}
			if err := engine.Apply(v.shown, f); err != nil {
				t.Fatalf("%s: event %d: a frame does not fold: %v", v.label, env.Sequence, err)
			}
		}
		sent[v.label] = frames
	}
	return sent
}

func qarnRequireSentIt(t *testing.T, label string, frames []*vttv1.Envelope, env *vttv1.Envelope) {
	t.Helper()
	var got []*vttv1.Envelope
	for _, f := range frames {
		if f.GetRulesetNamed() != nil {
			got = append(got, f)
		}
	}
	// VTT-980
	if len(got) != 1 {
		t.Fatalf("%s: sent %d RulesetNamed frames, want 1 (frames %v)", label, len(got), frames)
	}
	f := got[0]
	// VTT-980
	if !proto.Equal(f.GetRulesetNamed(), env.GetRulesetNamed()) {
		t.Errorf("%s: sent %v, want %v", label, f.GetRulesetNamed(), env.GetRulesetNamed())
	}
	// VTT-980: the forwarded event keeps its id (SPEC-016, SPEC-021).
	if f.EventId != env.EventId {
		t.Errorf("%s: sent event id %q, want %q", label, f.EventId, env.EventId)
	}
	// VTT-272
	if f.ParticipantId != "" || f.ActorRole != "" {
		t.Errorf("%s: sent participant %q, role %q", label, f.ParticipantId, f.ActorRole)
	}
}

func TestQARNEveryLogIsSentARulesetNamedWhateverItsEyesSee(t *testing.T) {
	world := engine.NewState()
	vs := qarnViewers()
	envs := qarnWorld()
	for _, e := range envs {
		qarnSend(t, world, vs, e)
	}
	// VTT-980: the fixture's eyes differ, one seeing tokens and one none.
	if len(vs[0].shown.Tokens) < 2 || len(vs[1].shown.Tokens) != 0 {
		t.Fatalf("degenerate fixture: lit eyes hold %d tokens, tokenless eyes %d", len(vs[0].shown.Tokens), len(vs[1].shown.Tokens))
	}
	seq := int64(len(envs))
	for i, rn := range []*vttv1.Envelope{
		qarnRulesetNamed(seq+1, "qa-rules", "2.5.0", "Plain Rules"),
		qarnRulesetNamed(seq+2, "qa-rules", "2.6.0", "Plain Rules Revised"),
	} {
		sent := qarnSend(t, world, vs, rn)
		for _, v := range vs {
			qarnRequireSentIt(t, v.label, sent[v.label], rn)
			p := rn.GetRulesetNamed()
			// VTT-979: the log's fold names what it was sent.
			if v.shown.RulesetID != p.RulesetId || v.shown.RulesetVersion != p.Version || v.shown.RulesetName != p.Name {
				t.Errorf("%s: after ruleset %d the log's fold names (%q, %q, %q)", v.label, i, v.shown.RulesetID, v.shown.RulesetVersion, v.shown.RulesetName)
			}
		}
	}
}

func TestQARNAFreshProjectionIsSentARulesetNamedWhateverItsEyesSee(t *testing.T) {
	envs := qarnWorld()
	world := engine.NewState()
	for _, e := range envs {
		if err := engine.Apply(world, e); err != nil {
			t.Fatalf("world refuses event %d: %v", e.Sequence, err)
		}
	}
	vs := qarnViewers()
	rn := qarnRulesetNamed(int64(len(envs)+1), "qa-rules", "2.5.0", "Plain Rules")
	sent := qarnSend(t, world, vs, rn)
	for _, v := range vs {
		qarnRequireSentIt(t, v.label, sent[v.label], rn)
	}
}
