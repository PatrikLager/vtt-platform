package perceive_test

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// qaRFViewer is one character log: its eyes, its projection and the fold of
// every frame it was sent.
type qaRFViewer struct {
	name      string
	eyes      perceive.Eyes
	pr        *perceive.Projection
	log       *engine.State
	frames    map[string][]*vttv1.Envelope
	foldFails []string
}

type qaRFTable struct {
	t       *testing.T
	seq     int64
	world   *engine.State
	viewers []*qaRFViewer
}

func qaRFEnvelope(seq int64, p any) *vttv1.Envelope {
	e := &vttv1.Envelope{
		EventId:       fmt.Sprintf("evt-%d", seq),
		Sequence:      seq,
		SessionId:     "sess-qa",
		ActorRole:     "dm",
		ParticipantId: "p-dm",
	}
	switch v := p.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: v}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: v}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: v}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: v}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: v}
	case *vttv1.TokenRemoved:
		e.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: v}
	case *vttv1.ResourceAdded:
		e.Payload = &vttv1.Envelope_ResourceAdded{ResourceAdded: v}
	case *vttv1.ResourceRemoved:
		e.Payload = &vttv1.Envelope_ResourceRemoved{ResourceRemoved: v}
	case *vttv1.ResourceChanged:
		e.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: v}
	default:
		panic(fmt.Sprintf("qa: no envelope arm for %T", p))
	}
	return e
}

func qaRFFloor(id string) *vttv1.SceneCreated {
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 5; x++ {
		for y := 0; y < 5; y++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	return &vttv1.SceneCreated{SceneId: id, Name: id, GridWidth: 5, GridHeight: 5, Tiles: tiles}
}

// step appends p to the world and hands the event to every viewer's
// projection, folding each frame into that viewer's log as a seat would.
func (tb *qaRFTable) step(p any) *vttv1.Envelope {
	tb.t.Helper()
	tb.seq++
	e := qaRFEnvelope(tb.seq, p)
	if err := engine.Apply(tb.world, e); err != nil {
		tb.t.Fatalf("world refused %v: %v", e.GetPayload(), err)
	}
	for _, v := range tb.viewers {
		frames := v.pr.Event(v.eyes, e, v.log, tb.world)
		v.frames[e.GetEventId()] = frames
		for _, f := range frames {
			if err := engine.Apply(v.log, f); err != nil {
				v.foldFails = append(v.foldFails, fmt.Sprintf("%s at %s: frame %v: %v", v.name, e.GetEventId(), f, err))
				// VTT-224: every frame a log is sent folds onto it.
				tb.t.Errorf("a frame did not fold: %s", v.foldFails[len(v.foldFails)-1])
			}
		}
	}
	return e
}

func qaRFNewTable(t *testing.T) *qaRFTable {
	t.Helper()
	tb := &qaRFTable{t: t, world: engine.NewState()}
	for _, v := range []struct {
		name string
		eyes perceive.Eyes
	}{
		{"hero", perceive.Eyes{Shoulder: "hero"}},
		{"mate", perceive.Eyes{Controller: "p-2"}},
		{"far", perceive.Eyes{Shoulder: "far"}},
	} {
		tb.viewers = append(tb.viewers, &qaRFViewer{
			name:   v.name,
			eyes:   v.eyes,
			pr:     perceive.New(v.eyes, nil),
			log:    engine.NewState(),
			frames: map[string][]*vttv1.Envelope{},
		})
	}
	return tb
}

func (tb *qaRFTable) viewer(name string) *qaRFViewer {
	for _, v := range tb.viewers {
		if v.name == name {
			return v
		}
	}
	tb.t.Fatalf("no viewer %q", name)
	return nil
}

// qaRFSetup builds two scenes: hero, mate and the goblin stand in sc-a, where
// each sees the others; far stands alone in sc-b.
func qaRFSetup(tb *qaRFTable) {
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	tb.step(&vttv1.SessionStarted{Name: "Evening"})
	tb.step(qaRFFloor("sc-a"))
	tb.step(qaRFFloor("sc-b"))
	tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "Hero", Kind: party,
		Attributes: map[string]int32{"nerve": 1},
		Resources:  map[string]*vttv1.Resource{"pool": {Current: 3, Max: 5}}}})
	tb.step(&vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1", Kind: party})
	tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "mate", Name: "Mate", Kind: party,
		Resources: map[string]*vttv1.Resource{"pool": {Current: 2, Max: 4}, "grit": {Current: 1, Max: 2}}}})
	tb.step(&vttv1.ActorControlGranted{ActorId: "mate", ParticipantId: "p-2", Kind: party})
	tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "far", Name: "Far", Kind: party,
		Resources: map[string]*vttv1.Resource{"pool": {Current: 6, Max: 7}}}})
	tb.step(&vttv1.ActorControlGranted{ActorId: "far", ParticipantId: "p-3", Kind: party})
	tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: "Goblin", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		Resources: map[string]*vttv1.Resource{"pool": {Current: 2, Max: 3}}}})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "sc-a", ActorId: "hero", Position: &vttv1.GridPosition{X: 0, Y: 0}})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-mate", SceneId: "sc-a", ActorId: "mate", Position: &vttv1.GridPosition{X: 4, Y: 4}})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-far", SceneId: "sc-b", ActorId: "far", Position: &vttv1.GridPosition{X: 0, Y: 0}})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "sc-a", ActorId: "gob", Position: &vttv1.GridPosition{X: 2, Y: 0}})
}

// qaRFForwardOf is what a log is sent when the event is forwarded: the event
// under its own id, its issuer cleared (VTT-272).
func qaRFForwardOf(e *vttv1.Envelope) *vttv1.Envelope {
	want := proto.Clone(e).(*vttv1.Envelope)
	want.ParticipantId = ""
	want.ActorRole = ""
	return want
}

func qaRFHasForward(frames []*vttv1.Envelope, e *vttv1.Envelope) bool {
	want := qaRFForwardOf(e)
	for _, f := range frames {
		if proto.Equal(f, want) {
			return true
		}
	}
	return false
}

func qaRFCarriesID(frames []*vttv1.Envelope, id string) bool {
	for _, f := range frames {
		if f.GetEventId() == id {
			return true
		}
	}
	return false
}

func qaRFResourceEventOf(f *vttv1.Envelope) (actor string, ok bool) {
	if a := f.GetResourceAdded(); a != nil {
		return a.GetActorId(), true
	}
	if r := f.GetResourceRemoved(); r != nil {
		return r.GetActorId(), true
	}
	return "", false
}

// VTT-779: a character log is forwarded a ResourceAdded or a ResourceRemoved
// of a party member, as the event under its own id, exactly when its eyes saw
// that party member before the event: hero's log (which sees mate) and mate's
// own log are forwarded mate's; far's log (which does not see mate) is not, and
// far's own log is forwarded far's, which hero's log is not.
// VTT-272: the forwarded copy carries no participant id or actor role.
func TestQAResourceEventsOfAPartyMemberAreForwardedToALogThatSawIt(t *testing.T) {
	tb := qaRFNewTable(t)
	qaRFSetup(tb)
	addMate := tb.step(&vttv1.ResourceAdded{ActorId: "mate", Resource: "focus", Value: &vttv1.Resource{Current: 3, Max: 5}})
	removeMate := tb.step(&vttv1.ResourceRemoved{ActorId: "mate", Resource: "grit"})
	addFar := tb.step(&vttv1.ResourceAdded{ActorId: "far", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 4}})
	removeFar := tb.step(&vttv1.ResourceRemoved{ActorId: "far", Resource: "pool"})

	for _, c := range []struct {
		viewer  string
		ev      *vttv1.Envelope
		forward bool
	}{
		{"hero", addMate, true},
		{"mate", addMate, true},
		{"far", addMate, false},
		{"hero", removeMate, true},
		{"mate", removeMate, true},
		{"far", removeMate, false},
		{"far", addFar, true},
		{"hero", addFar, false},
		{"mate", addFar, false},
		{"far", removeFar, true},
		{"hero", removeFar, false},
		{"mate", removeFar, false},
	} {
		frames := tb.viewer(c.viewer).frames[c.ev.GetEventId()]
		if c.forward && !qaRFHasForward(frames, c.ev) {
			t.Errorf("%s's log was not forwarded %s (%v) as the event less its issuer; frames: %v", c.viewer, c.ev.GetEventId(), c.ev.GetPayload(), frames)
		}
		if !c.forward && qaRFCarriesID(frames, c.ev.GetEventId()) {
			t.Errorf("%s's log, whose eyes did not see the party member, was sent %s under its id; frames: %v", c.viewer, c.ev.GetEventId(), frames)
		}
		for _, f := range frames {
			if f.GetParticipantId() != "" || f.GetActorRole() != "" {
				t.Errorf("%s's log was sent a frame at %s carrying participant %q role %q", c.viewer, c.ev.GetEventId(), f.GetParticipantId(), f.GetActorRole())
			}
		}
	}
}

// VTT-779 (an actor that is not a party member): no character log is sent a
// ResourceAdded or a ResourceRemoved of a creature, forwarded or as a
// correction: not while a log's eyes see the goblin, not while they do not,
// and not when it comes back into sight after both changed out of sight.
func TestQAResourceEventsOfACreatureReachNoLog(t *testing.T) {
	tb := qaRFNewTable(t)
	qaRFSetup(tb)
	tb.step(&vttv1.ResourceAdded{ActorId: "gob", Resource: "focus", Value: &vttv1.Resource{Current: 2, Max: 3}})
	tb.step(&vttv1.ResourceRemoved{ActorId: "gob", Resource: "pool"})
	tb.step(&vttv1.TokenRemoved{TokenId: "t-gob"})
	tb.step(&vttv1.ResourceAdded{ActorId: "gob", Resource: "spark", Value: &vttv1.Resource{Current: 1, Max: 6}})
	tb.step(&vttv1.ResourceRemoved{ActorId: "gob", Resource: "focus"})
	back := tb.step(&vttv1.TokenPlaced{TokenId: "t-gob2", SceneId: "sc-a", ActorId: "gob", Position: &vttv1.GridPosition{X: 2, Y: 0}})

	hero := tb.viewer("hero")
	if _, held := hero.log.Actors["gob"]; !held {
		t.Fatalf("hero's log does not hold the goblin after it came back into sight; the creature half proves nothing")
	}
	if !qaRFHasTokenPlaced(hero.frames[back.GetEventId()], "t-gob2") {
		t.Fatalf("hero's log was not sent the goblin's token at %s; the goblin never came back into sight", back.GetEventId())
	}
	for _, v := range tb.viewers {
		for id, frames := range v.frames {
			for _, f := range frames {
				if actor, ok := qaRFResourceEventOf(f); ok && actor == "gob" {
					t.Errorf("%s's log was sent a resource event of the goblin at %s: %v", v.name, id, f)
				}
			}
		}
	}
}

func qaRFHasTokenPlaced(frames []*vttv1.Envelope, token string) bool {
	for _, f := range frames {
		if f.GetTokenPlaced().GetTokenId() == token {
			return true
		}
	}
	return false
}

// VTT-779 VTT-781 VTT-224: what hero's log is forwarded of mate's resource events
// folds, with no withdrawal or introduction of mate, and the log then holds
// mate's resources as the world does.
// SPEC-016 "A character log folds in the order it is written".
func TestQAForwardedResourceEventsFoldIntoTheLog(t *testing.T) {
	tb := qaRFNewTable(t)
	qaRFSetup(tb)
	add := tb.step(&vttv1.ResourceAdded{ActorId: "mate", Resource: "focus", Value: &vttv1.Resource{Current: 3, Max: 5}})
	remove := tb.step(&vttv1.ResourceRemoved{ActorId: "mate", Resource: "grit"})
	tb.step(&vttv1.ResourceAdded{ActorId: "far", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 4}})
	if t.Failed() {
		for _, e := range []*vttv1.Envelope{add, remove} {
			for i, f := range tb.viewer("hero").frames[e.GetEventId()] {
				t.Logf("hero's frame %d at %s: %v", i, e.GetEventId(), f)
			}
		}
	}
	for _, name := range []string{"hero", "mate"} {
		got := tb.viewer(name).log.Actors["mate"].GetResources()
		want := tb.world.Actors["mate"].GetResources()
		if len(got) != len(want) {
			t.Errorf("%s's log holds mate's resources %v, the world %v", name, got, want)
			continue
		}
		for k, w := range want {
			if !proto.Equal(got[k], w) {
				t.Errorf("%s's log holds mate's %s as %v, the world as %v", name, k, got[k], w)
			}
		}
	}
}

// VTT-779 ("its projection does not start afresh at that event"): the same
// ResourceAdded is forwarded to hero's log when its decision is resumed from a
// fold that is up to date, or started by New from the world before the event
// (VTT-312), and not when it starts afresh at the event: New with nothing sent,
// or Resume from a fold that lags the world for mate.
func TestQAResourceEventIsNotForwardedWhereTheProjectionStartsAfresh(t *testing.T) {
	eyes := perceive.Eyes{Shoulder: "hero"}
	ev := &vttv1.ResourceAdded{ActorId: "mate", Resource: "focus", Value: &vttv1.Resource{Current: 3, Max: 5}}

	// prefix builds the world through setup and hero's fold of it, then hands
	// back copies of both and the next sequence.
	prefix := func(t *testing.T, lag bool) (*engine.State, *engine.State, int64) {
		t.Helper()
		tb := qaRFNewTable(t)
		qaRFSetup(tb)
		if lag {
			// The world changes mate while hero's log is sent nothing of it.
			tb.seq++
			lagging := qaRFEnvelope(tb.seq, &vttv1.ResourceChanged{ActorId: "mate", Resource: "pool", Delta: 1, NewValue: 3})
			if err := engine.Apply(tb.world, lagging); err != nil {
				t.Fatalf("world refused the lagging change: %v", err)
			}
		}
		return tb.world.Snapshot(), tb.viewer("hero").log.Snapshot(), tb.seq + 1
	}

	run := func(t *testing.T, world, fold *engine.State, pr *perceive.Projection, seq int64) ([]*vttv1.Envelope, *vttv1.Envelope) {
		t.Helper()
		e := qaRFEnvelope(seq, ev)
		if err := engine.Apply(world, e); err != nil {
			t.Fatalf("world refused the ResourceAdded: %v", err)
		}
		return pr.Event(eyes, e, fold, world), e
	}

	t.Run("New from the world before the event forwards", func(t *testing.T) {
		world, fold, seq := prefix(t, false)
		frames, e := run(t, world, fold, perceive.New(eyes, world.Snapshot()), seq)
		if !qaRFHasForward(frames, e) {
			t.Fatalf("not forwarded; frames: %v", frames)
		}
	})
	t.Run("Resume from an up-to-date fold forwards", func(t *testing.T) {
		world, fold, seq := prefix(t, false)
		frames, e := run(t, world, fold, perceive.Resume(eyes, fold, world.Snapshot()), seq)
		if !qaRFHasForward(frames, e) {
			t.Fatalf("not forwarded; frames: %v", frames)
		}
	})
	t.Run("New with nothing sent does not forward", func(t *testing.T) {
		world, _, seq := prefix(t, false)
		frames, e := run(t, world, engine.NewState(), perceive.New(eyes, nil), seq)
		if qaRFCarriesID(frames, e.GetEventId()) {
			t.Fatalf("a projection starting afresh at the event was sent it under its id; frames: %v", frames)
		}
	})
	t.Run("Resume from a lagging fold does not forward", func(t *testing.T) {
		world, fold, seq := prefix(t, true)
		frames, e := run(t, world, fold, perceive.Resume(eyes, fold, world.Snapshot()), seq)
		if qaRFCarriesID(frames, e.GetEventId()) {
			t.Fatalf("a resume that starts empty was sent the event under its id; frames: %v", frames)
		}
	})
}
