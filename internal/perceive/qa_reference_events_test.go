package perceive_test

import (
	"fmt"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type qarfLog struct {
	name  string
	eyes  perceive.Eyes
	pr    *perceive.Projection
	shown *engine.State
}

type qarfTable struct {
	world *engine.State
	logs  []*qarfLog
	seq   int64
}

func qarfNewTable(eyes map[string]perceive.Eyes) *qarfTable {
	tb := &qarfTable{world: engine.NewState()}
	for _, name := range []string{"kilo", "mike", "echo"} {
		e, ok := eyes[name]
		if !ok {
			continue
		}
		tb.logs = append(tb.logs, &qarfLog{
			name: name, eyes: e, pr: perceive.New(e, nil), shown: engine.NewState(),
		})
	}
	return tb
}

func (tb *qarfTable) step(t *testing.T, env *vttv1.Envelope) map[string][]*vttv1.Envelope {
	t.Helper()
	tb.seq++
	env.Sequence = tb.seq
	env.EventId = fmt.Sprintf("ev-%d", tb.seq)
	env.ParticipantId = "p-dm"
	env.ActorRole = "dm"
	env.SessionId = "s-1"
	env.OccurredAt = &timestamppb.Timestamp{Seconds: 1795000000 + tb.seq}
	if err := engine.Apply(tb.world, env); err != nil {
		t.Fatalf("world refused %v: %v", env, err)
	}
	sent := proto.Clone(env)
	out := map[string][]*vttv1.Envelope{}
	for _, l := range tb.logs {
		frames := l.pr.Event(l.eyes, env, l.shown, tb.world)
		for _, f := range frames {
			if err := engine.Apply(l.shown, f); err != nil {
				t.Fatalf("log %s refused its frame %v: %v", l.name, f, err)
			}
		}
		out[l.name] = frames
	}
	// SPEC-016.
	if !proto.Equal(env, sent) {
		t.Errorf("the event changed under Event: %v", env)
	}
	return out
}

func qarfRef(entry, place, state string) *vttv1.Reference {
	return &vttv1.Reference{EntryId: entry, Place: place, State: state}
}

func qarfToken(id, actor string, x int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: id, SceneId: "kilo", ActorId: actor,
		Position: &vttv1.GridPosition{X: x, Y: 1},
	}}}
}

func qarfActor(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: a},
	}}
}

// Keep every token in one lit row: VTT-1032 binds a log that saw the actor.
func qarfSetUp(t *testing.T, tb *qarfTable) {
	t.Helper()
	tiles := map[string]*vttv1.TileRef{}
	for x := range 6 {
		for y := range 3 {
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	other := vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	for _, env := range []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "kilo", Name: "Kilo", GridWidth: 6, GridHeight: 3, Tiles: tiles,
		}}},
		qarfActor(&vttv1.Actor{
			ActorId: "pc-kilo", Name: "Kilo", ModuleId: "mod-q",
			Attributes: map[string]int32{"kilo": 1}, Kind: party,
		}),
		qarfActor(&vttv1.Actor{
			ActorId: "pc-mike", Name: "Mike", ModuleId: "mod-q",
			Attributes: map[string]int32{"mike": 2}, Kind: party,
		}),
		qarfActor(&vttv1.Actor{
			ActorId: "cr-oscar", Name: "Oscar", ModuleId: "mod-q",
			Attributes: map[string]int32{"kilo": 5}, Kind: other,
			References: map[string]*vttv1.Reference{
				"r-1": qarfRef("echo", "kilo", "oscar"),
				"r-4": qarfRef("delta", "mike", "lima"),
			},
		}),
		qarfActor(&vttv1.Actor{
			ActorId: "cr-echo", Name: "Echo", ModuleId: "mod-q",
			Attributes: map[string]int32{"mike": 3}, Kind: other,
		}),
		{Payload: &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{
				ActorId: "cr-echo", ParticipantId: "p-9", Kind: other,
			},
		}},
		qarfToken("t-k", "pc-kilo", 1),
		qarfToken("t-m", "pc-mike", 2),
		qarfToken("t-o", "cr-oscar", 3),
		qarfToken("t-e", "cr-echo", 4),
	} {
		tb.step(t, env)
	}
}

func qarfRefFrames(frames []*vttv1.Envelope) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range frames {
		switch f.GetPayload().(type) {
		case *vttv1.Envelope_ReferenceAdded, *vttv1.Envelope_ReferenceRemoved,
			*vttv1.Envelope_ReferenceChanged:
			out = append(out, f)
		}
	}
	return out
}

func qarfRefAdded(actor, id string, ref *vttv1.Reference) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceAdded{
		ReferenceAdded: &vttv1.ReferenceAdded{ActorId: actor, ReferenceId: id, Reference: ref},
	}}
}

func qarfRefRemoved(actor, id string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceRemoved{
		ReferenceRemoved: &vttv1.ReferenceRemoved{ActorId: actor, ReferenceId: id},
	}}
}

func qarfRefChanged(actor, id, place, state string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceChanged{
		ReferenceChanged: &vttv1.ReferenceChanged{
			ActorId: actor, ReferenceId: id, Place: place, State: state,
		},
	}}
}

func qarfAllEyes() map[string]perceive.Eyes {
	return map[string]perceive.Eyes{
		"kilo": {Shoulder: "pc-kilo"},
		"mike": {Shoulder: "pc-mike"},
		"echo": {Controller: "p-9"},
	}
}

func qarfRefsEqual(t *testing.T, where string, got, want map[string]*vttv1.Reference) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: references %v, want %v", where, got, want)
		return
	}
	for id, w := range want {
		if g, ok := got[id]; !ok || !proto.Equal(g, w) {
			t.Errorf("%s: reference %q = %v, want %v", where, id, g, w)
		}
	}
}

// VTT-1032.
func TestQARFAPartyMembersReferenceEventsReachALogThatSawIt(t *testing.T) {
	tb := qarfNewTable(qarfAllEyes())
	qarfSetUp(t, tb)
	events := []*vttv1.Envelope{
		qarfRefAdded("pc-mike", "r-2", qarfRef("delta", "kilo", "oscar")),
		qarfRefAdded("pc-mike", "r-1", qarfRef("delta", "mike", "lima")),
		qarfRefChanged("pc-mike", "r-2", "mike", "lima"),
		qarfRefChanged("pc-mike", "r-1", "", ""),
		qarfRefRemoved("pc-mike", "r-2"),
	}
	for _, env := range events {
		sent := tb.step(t, env)
		want := proto.Clone(env).(*vttv1.Envelope)
		want.ParticipantId, want.ActorRole = "", ""
		for _, l := range tb.logs {
			got := qarfRefFrames(sent[l.name])
			if len(got) != 1 {
				t.Errorf("log %s sent %d reference frames for %v, want 1: %v",
					l.name, len(got), env.GetPayload(), sent[l.name])
				continue
			}
			if !proto.Equal(got[0], want) {
				t.Errorf("log %s sent %v, want %v", l.name, got[0], want)
			}
		}
	}
	for _, l := range tb.logs {
		qarfRefsEqual(t, "log "+l.name, l.shown.Actors["pc-mike"].GetReferences(),
			tb.world.Actors["pc-mike"].GetReferences())
	}
	qarfRefsEqual(t, "world", tb.world.Actors["pc-mike"].GetReferences(),
		map[string]*vttv1.Reference{"r-1": qarfRef("delta", "", "")})
}

// VTT-1032.
func TestQARFNoLogIsSentACreaturesReferenceEvents(t *testing.T) {
	tb := qarfNewTable(qarfAllEyes())
	qarfSetUp(t, tb)
	events := []*vttv1.Envelope{
		qarfRefAdded("cr-oscar", "r-2", qarfRef("delta", "kilo", "oscar")),
		qarfRefChanged("cr-oscar", "r-1", "mike", "lima"),
		qarfRefRemoved("cr-oscar", "r-4"),
		qarfRefAdded("cr-echo", "r-3", qarfRef("echo", "mike", "oscar")),
		qarfRefChanged("cr-echo", "r-3", "kilo", ""),
		qarfRefRemoved("cr-echo", "r-3"),
	}
	for _, env := range events {
		sent := tb.step(t, env)
		for _, l := range tb.logs {
			if got := qarfRefFrames(sent[l.name]); len(got) != 0 {
				t.Errorf("log %s sent %v for %v", l.name, got, env.GetPayload())
			}
		}
	}
	for _, l := range tb.logs {
		for _, id := range []string{"cr-oscar", "cr-echo"} {
			if a, ok := l.shown.Actors[id]; !ok {
				t.Errorf("log %s holds no %s: the fixture does not see it", l.name, id)
			} else if n := len(a.GetReferences()); n != 0 {
				t.Errorf("log %s holds %d references of %s", l.name, n, id)
			}
		}
	}
}

// VTT-1032.
func TestQARFInterleavedOnlyThePartyMembersReach(t *testing.T) {
	tb := qarfNewTable(qarfAllEyes())
	qarfSetUp(t, tb)
	steps := []struct {
		env   *vttv1.Envelope
		reach bool
	}{
		{qarfRefAdded("cr-oscar", "r-2", qarfRef("delta", "kilo", "oscar")), false},
		{qarfRefAdded("pc-mike", "r-2", qarfRef("delta", "kilo", "oscar")), true},
		{qarfRefChanged("cr-oscar", "r-2", "mike", "lima"), false},
		{qarfRefChanged("pc-mike", "r-2", "mike", "lima"), true},
		{qarfRefRemoved("cr-oscar", "r-2"), false},
		{qarfRefRemoved("pc-mike", "r-2"), true},
	}
	for i, s := range steps {
		sent := tb.step(t, s.env)
		for _, l := range tb.logs {
			n := len(qarfRefFrames(sent[l.name]))
			if s.reach && n != 1 || !s.reach && n != 0 {
				t.Errorf("step %d, log %s: %d reference frames, reach %v", i, l.name, n, s.reach)
			}
		}
	}
}

// SPEC-016.
func TestQARFACreatureIsIntroducedWithNoReferences(t *testing.T) {
	tb := qarfNewTable(qarfAllEyes())
	qarfSetUp(t, tb)
	if n := len(tb.world.Actors["cr-oscar"].GetReferences()); n != 2 {
		t.Fatalf("degenerate: the world's creature holds %d references", n)
	}
	for _, l := range tb.logs {
		a, ok := l.shown.Actors["cr-oscar"]
		if !ok {
			t.Fatalf("log %s holds no cr-oscar: the fixture does not see it", l.name)
		}
		if n := len(a.GetReferences()); n != 0 {
			t.Errorf("log %s introduced to cr-oscar with %d references", l.name, n)
		}
	}
}
