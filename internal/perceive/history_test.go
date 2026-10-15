package perceive_test

import (
	"fmt"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// historyFold is one history a spectator perched on hero watches. Keep these
// events equal to internal/gateway's historyFolds: its testdata holds the
// folds they were derived by hand against.
type historyFold struct {
	name   string
	events []any
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
		{"reuse-identical", reAdd(historyGob("Old Gob", ""))},
		{"reuse-renamed", reAdd(historyGob("New Gob", ""))},
		{"reuse-module-id", reAdd(historyGob("Old Gob", "other"))},
		{"condition-order", historyBase(
			condition("A"), condition("B"), &vttv1.TokenRemoved{TokenId: "t-gob"},
			uncondition("A"), condition("C"), condition("A"), historyPlace("t-gob2"),
			&vttv1.TokenRemoved{TokenId: "t-gob2"}, uncondition("A"), uncondition("B"), historyPlace("t-gob3"))},
		{"controller-restatement", historyBase(
			grant("p1", nonParty), grant("p2", nonParty), &vttv1.TokenRemoved{TokenId: "t-gob"},
			&vttv1.ActorControlRevoked{ActorId: "gob", ParticipantId: "p1"}, grant("p1", nonParty), historyPlace("t-gob2"),
			&vttv1.TokenRemoved{TokenId: "t-gob2"}, grant("p2", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER), historyPlace("t-gob3"))},
	}
}

// historyFrames runs events through a projection for hero's spectator and
// returns the frames each event was sent and the world after the last.
func historyFrames(t *testing.T, events []any) ([][]*vttv1.Envelope, *engine.State) {
	t.Helper()
	pr := newViewer(t, perceive.Eyes{Shoulder: "hero"})
	world := engine.NewState()
	out := make([][]*vttv1.Envelope, len(events))
	for i, p := range events {
		env := historyEnvelope(int64(i+1), p)
		if err := engine.Apply(world, env); err != nil {
			t.Fatalf("the world does not fold at %d: %v", i+1, err)
		}
		out[i] = pr.event(env, world)
	}
	return out, world
}

// frameNames names each frame by its payload and the ids it carries, in order.
func frameNames(es []*vttv1.Envelope) []string {
	var out []string
	for _, e := range es {
		switch p := e.GetPayload().(type) {
		case *vttv1.Envelope_ActorRemoved:
			out = append(out, "ActorRemoved "+p.ActorRemoved.GetActorId())
		case *vttv1.Envelope_ActorAdded:
			out = append(out, "ActorAdded "+p.ActorAdded.GetActor().GetActorId()+" "+p.ActorAdded.GetActor().GetName())
		case *vttv1.Envelope_TokenPlaced:
			out = append(out, "TokenPlaced "+p.TokenPlaced.GetTokenId())
		case *vttv1.Envelope_ConditionApplied:
			out = append(out, "ConditionApplied "+p.ConditionApplied.GetConditionId())
		case *vttv1.Envelope_ConditionRemoved:
			out = append(out, "ConditionRemoved "+p.ConditionRemoved.GetConditionId())
		case *vttv1.Envelope_ActorControlGranted:
			g := p.ActorControlGranted
			out = append(out, "ActorControlGranted "+g.GetParticipantId()+" "+g.GetKind().String())
		default:
			out = append(out, fmt.Sprintf("%T", p))
		}
	}
	return out
}

func historyByName(t *testing.T, name string) historyFold {
	t.Helper()
	for _, h := range historyFolds() {
		if h.name == name {
			return h
		}
	}
	t.Fatalf("no history %q", name)
	return historyFold{}
}

// VTT-313
func TestAnActorAddedAgainIsIntroducedAfreshOnlyWhenItIsAnother(t *testing.T) {
	for name, want := range map[string][]string{
		"reuse-identical": {"TokenPlaced t-gob2"},
		"reuse-renamed":   {"ActorRemoved gob", "ActorAdded gob New Gob", "TokenPlaced t-gob2"},
		"reuse-module-id": {"ActorRemoved gob", "ActorAdded gob Old Gob", "TokenPlaced t-gob2"},
	} {
		frames, _ := historyFrames(t, historyByName(t, name).events)
		if got := frameNames(frames[len(frames)-1]); !slices.Equal(got, want) {
			t.Errorf("%s: the event that brings gob back is sent %q, want %q", name, got, want)
		}
	}
}

func TestACorrectionFollowsTheOrderTheViewerHolds(t *testing.T) {
	condition := func(id string) *vttv1.ConditionApplied {
		return &vttv1.ConditionApplied{ActorId: "gob", ConditionId: id}
	}
	uncondition := func(id string) *vttv1.ConditionRemoved {
		return &vttv1.ConditionRemoved{ActorId: "gob", ConditionId: id}
	}
	for _, c := range []struct {
		name   string
		events []any
		want   []string
	}{
		{"conditions the world reordered unseen", historyByName(t, "condition-order").events,
			[]string{"ConditionRemoved A", "ConditionRemoved B", "TokenPlaced t-gob3"}},
		{"controllers the world reordered unseen", historyByName(t, "controller-restatement").events,
			[]string{"ActorControlGranted p1 ACTOR_KIND_PARTY_MEMBER", "TokenPlaced t-gob3"}},
		{"conditions removed unseen, not sorted", historyBase(condition("B"), condition("A"),
			&vttv1.TokenRemoved{TokenId: "t-gob"}, uncondition("B"), uncondition("A"), historyPlace("t-gob2")),
			[]string{"ConditionRemoved B", "ConditionRemoved A", "TokenPlaced t-gob2"}},
	} {
		frames, _ := historyFrames(t, c.events)
		if got := frameNames(frames[len(frames)-1]); !slices.Equal(got, c.want) {
			t.Errorf("%s: gob coming back into sight is sent %q, want %q", c.name, got, c.want)
		}
	}
}

// VTT-248
func TestAnActorAddedAgainUnchangedWithItsConditionsReachesTheViewerAsTheWorldHoldsIt(t *testing.T) {
	condition := func(id string) *vttv1.ConditionApplied {
		return &vttv1.ConditionApplied{ActorId: "gob", ConditionId: id}
	}
	events := historyBase(condition("A"), condition("B"), &vttv1.TokenRemoved{TokenId: "t-gob"},
		&vttv1.ActorRemoved{ActorId: "gob"}, historyGob("Old Gob", ""), condition("B"), condition("A"),
		historyPlace("t-gob2"))
	frames, world := historyFrames(t, events)
	shown := engine.NewState()
	for _, f := range frames {
		for _, e := range f {
			if err := engine.Apply(shown, e); err != nil {
				t.Fatalf("the viewer's frames do not fold: %v", err)
			}
		}
	}
	ids := func(cs []engine.ActorCondition) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.ID)
		}
		slices.Sort(out)
		return out
	}
	if got, want := ids(shown.Conditions["gob"]), ids(world.Conditions["gob"]); !slices.Equal(got, want) {
		t.Errorf("the viewer holds gob's conditions %q, the world %q", got, want)
	}
	got, want := shown.Actors["gob"], world.Actors["gob"]
	if !proto.Equal(got, want) {
		t.Errorf("the viewer holds gob as %v, the world as %v", got, want)
	}
}
