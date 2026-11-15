package engine_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// qaRCLog numbers the envelopes the QA world is built from.
type qaRCLog struct{ seq int64 }

func (l *qaRCLog) env(p any) *vttv1.Envelope {
	l.seq++
	e := &vttv1.Envelope{
		EventId:       fmt.Sprintf("qa-rc-%d", l.seq),
		Sequence:      l.seq,
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
	case *vttv1.ConditionApplied:
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: v}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: v}
	case *vttv1.NoteUpserted:
		e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: v}
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

func qaRCMustApply(t *testing.T, st *engine.State, e *vttv1.Envelope) {
	t.Helper()
	if err := engine.Apply(st, e); err != nil {
		t.Fatalf("Apply(%v) refused: %v", e.GetPayload(), err)
	}
}

func qaRCJSON(t *testing.T, st *engine.State) string {
	t.Helper()
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	return string(b)
}

// qaRCWorld is a state with two actors whose sheets overlap on purpose: lera
// holds an ATTRIBUTE named focus and the goblin a RESOURCE named focus, so a
// fold that writes the wrong map or the wrong actor shows.
func qaRCWorld(t *testing.T) (*engine.State, *qaRCLog) {
	t.Helper()
	st := engine.NewState()
	l := &qaRCLog{}
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	md, err := structpb.NewStruct(map[string]any{"mood": "wary"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []*vttv1.Envelope{
		l.env(&vttv1.SessionStarted{Name: "Evening"}),
		l.env(&vttv1.SceneCreated{SceneId: "sc-1", Name: "Hall", GridWidth: 3, GridHeight: 3, Tiles: tiles}),
		l.env(&vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId:    "act-lera",
			Name:       "Lera",
			ModuleId:   "mod-q",
			Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Attributes: map[string]int32{"nerve": 2, "focus": 7},
			Resources: map[string]*vttv1.Resource{
				"pool": {Current: 4, Max: 6},
				"grit": {Current: 1, Max: 3},
			},
			ModuleData: md,
			AbilityIds: []string{"rally", "lunge"},
		}}),
		l.env(&vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId:    "act-gob",
			Name:       "Goblin",
			Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			Attributes: map[string]int32{"nerve": 1},
			Resources: map[string]*vttv1.Resource{
				"pool":  {Current: 2, Max: 2},
				"focus": {Current: 5, Max: 8},
				"spark": {},
			},
		}}),
		l.env(&vttv1.ActorControlGranted{ActorId: "act-lera", ParticipantId: "p-1", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}),
		l.env(&vttv1.ConditionApplied{ActorId: "act-lera", ConditionId: "dazed", Source: "trap"}),
		l.env(&vttv1.TokenPlaced{TokenId: "tok-lera", SceneId: "sc-1", ActorId: "act-lera", Position: &vttv1.GridPosition{X: 1, Y: 1}, Name: "Lera"}),
		l.env(&vttv1.TokenPlaced{TokenId: "tok-gob", SceneId: "sc-1", ActorId: "act-gob", Position: &vttv1.GridPosition{X: 2, Y: 2}}),
		l.env(&vttv1.NoteUpserted{Key: "lore", Title: "Lore", Text: "old", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}),
	} {
		qaRCMustApply(t, st, e)
	}
	return st, l
}

// VTT-769: engine.Apply, at a ResourceAdded, gives the named actor the named
// resource at the event's current and max.
func TestQAResourceAddedGivesTheResourceAtTheEventsCurrentAndMax(t *testing.T) {
	st, l := qaRCWorld(t)
	qaRCMustApply(t, st, l.env(&vttv1.ResourceAdded{ActorId: "act-lera", Resource: "focus", Value: &vttv1.Resource{Current: 3, Max: 5}}))
	got := st.Actors["act-lera"].GetResources()["focus"]
	if got == nil {
		t.Fatalf("act-lera holds no resource focus after its ResourceAdded; resources: %v", st.Actors["act-lera"].GetResources())
	}
	if got.GetCurrent() != 3 || got.GetMax() != 5 {
		t.Fatalf("act-lera's focus = %d/%d, want 3/5", got.GetCurrent(), got.GetMax())
	}
}

// VTT-769 (the event's current and max, each 0): 0 is a value, so a value whose
// numbers are both 0 is given as held at 0 and 0, not refused as no value.
func TestQAResourceAddedGivesAZeroValue(t *testing.T) {
	cases := []struct {
		name     string
		value    *vttv1.Resource
		cur, max int32
	}{
		{"both zero", &vttv1.Resource{}, 0, 0},
		{"current zero", &vttv1.Resource{Current: 0, Max: 4}, 0, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, l := qaRCWorld(t)
			if err := engine.Apply(st, l.env(&vttv1.ResourceAdded{ActorId: "act-lera", Resource: "spark", Value: c.value})); err != nil {
				t.Fatalf("ResourceAdded with value %v refused: %v", c.value, err)
			}
			got, ok := st.Actors["act-lera"].GetResources()["spark"]
			if !ok || got == nil {
				t.Fatalf("act-lera holds no spark after a ResourceAdded at %v", c.value)
			}
			if got.GetCurrent() != c.cur || got.GetMax() != c.max {
				t.Fatalf("act-lera's spark = %d/%d, want %d/%d", got.GetCurrent(), got.GetMax(), c.cur, c.max)
			}
		})
	}
}

// VTT-791: a current over its max, or below zero, is given as written.
func TestQAResourceAddedReadsNoNumbers(t *testing.T) {
	cases := []struct {
		name     string
		cur, max int32
	}{
		{"current over max", 9, 4},
		{"current below zero", -2, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, l := qaRCWorld(t)
			if err := engine.Apply(st, l.env(&vttv1.ResourceAdded{ActorId: "act-lera", Resource: "ember", Value: &vttv1.Resource{Current: c.cur, Max: c.max}})); err != nil {
				t.Fatalf("ResourceAdded at %d/%d refused: %v", c.cur, c.max, err)
			}
			got := st.Actors["act-lera"].GetResources()["ember"]
			if got.GetCurrent() != c.cur || got.GetMax() != c.max {
				t.Fatalf("act-lera's ember = %d/%d, want %d/%d", got.GetCurrent(), got.GetMax(), c.cur, c.max)
			}
		})
	}
}

// VTT-769 with VTT-770: "already holds" is the named actor's. The goblin holds
// a resource named focus and Lera an attribute named focus; neither is Lera's
// resource, so Lera's ResourceAdded of focus is given.
func TestQAResourceAddedHeldElsewhereIsGiven(t *testing.T) {
	st, l := qaRCWorld(t)
	if err := engine.Apply(st, l.env(&vttv1.ResourceAdded{ActorId: "act-lera", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 2}})); err != nil {
		t.Fatalf("ResourceAdded of a resource another actor holds, and an attribute key, refused: %v", err)
	}
	if got := st.Actors["act-lera"].GetResources()["focus"]; got.GetCurrent() != 1 || got.GetMax() != 2 {
		t.Fatalf("act-lera's focus = %v, want 1/2", got)
	}
}

// VTT-770: engine.Apply refuses a ResourceAdded naming an actor it does not
// hold, a resource the actor already holds, no value, or a resource name that
// is empty or longer than 128 bytes; a refusal leaves the state unchanged, so
// the command that would append it appends nothing.
func TestQAResourceAddedRefusals(t *testing.T) {
	cases := []struct {
		name string
		ev   *vttv1.ResourceAdded
	}{
		{"unknown actor", &vttv1.ResourceAdded{ActorId: "act-ghost", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 1}}},
		{"empty actor", &vttv1.ResourceAdded{ActorId: "", Resource: "focus", Value: &vttv1.Resource{Current: 1, Max: 1}}},
		{"resource already held", &vttv1.ResourceAdded{ActorId: "act-lera", Resource: "pool", Value: &vttv1.Resource{Current: 1, Max: 1}}},
		{"resource already held at zero", &vttv1.ResourceAdded{ActorId: "act-gob", Resource: "spark", Value: &vttv1.Resource{Current: 1, Max: 1}}},
		{"no value", &vttv1.ResourceAdded{ActorId: "act-lera", Resource: "dust"}},
		{"empty resource name", &vttv1.ResourceAdded{ActorId: "act-lera", Resource: "", Value: &vttv1.Resource{Current: 1, Max: 1}}},
		{"129 one-byte characters", &vttv1.ResourceAdded{ActorId: "act-lera", Resource: strings.Repeat("a", 129), Value: &vttv1.Resource{Current: 1, Max: 1}}},
		{"65 two-byte characters (130 bytes)", &vttv1.ResourceAdded{ActorId: "act-lera", Resource: strings.Repeat("ö", 65), Value: &vttv1.Resource{Current: 1, Max: 1}}},
		{"33 four-byte characters (132 bytes)", &vttv1.ResourceAdded{ActorId: "act-lera", Resource: strings.Repeat("😀", 33), Value: &vttv1.Resource{Current: 1, Max: 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, l := qaRCWorld(t)
			before := qaRCJSON(t, st)
			err := engine.Apply(st, l.env(c.ev))
			if err == nil {
				t.Fatalf("ResourceAdded %v accepted; want a refusal", c.ev)
			}
			t.Logf("refusal: %v", err)
			if after := qaRCJSON(t, st); after != before {
				t.Fatalf("a refused ResourceAdded changed the state\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
}

// VTT-770: the name bound is 128 BYTES, so a name of exactly 128 bytes is
// given whatever its character count.
func TestQAResourceAddedNameOf128BytesIsGiven(t *testing.T) {
	for _, name := range []string{strings.Repeat("a", 128), strings.Repeat("ö", 64), strings.Repeat("😀", 32)} {
		t.Run(fmt.Sprintf("%d bytes, %d runes", len(name), len([]rune(name))), func(t *testing.T) {
			st, l := qaRCWorld(t)
			if err := engine.Apply(st, l.env(&vttv1.ResourceAdded{ActorId: "act-lera", Resource: name, Value: &vttv1.Resource{Current: 2, Max: 3}})); err != nil {
				t.Fatalf("ResourceAdded with a %d-byte name refused: %v", len(name), err)
			}
			if got := st.Actors["act-lera"].GetResources()[name]; got.GetCurrent() != 2 || got.GetMax() != 3 {
				t.Fatalf("resource %q = %v, want 2/3", name, got)
			}
		})
	}
}

// VTT-771: engine.Apply, at a ResourceRemoved, takes the named resource from
// the named actor; a resource so taken can be added again (VTT-769).
func TestQAResourceRemovedTakesTheResource(t *testing.T) {
	st, l := qaRCWorld(t)
	qaRCMustApply(t, st, l.env(&vttv1.ResourceRemoved{ActorId: "act-lera", Resource: "grit"}))
	if _, ok := st.Actors["act-lera"].GetResources()["grit"]; ok {
		t.Fatalf("act-lera still holds grit after its ResourceRemoved: %v", st.Actors["act-lera"].GetResources())
	}
	qaRCMustApply(t, st, l.env(&vttv1.ResourceAdded{ActorId: "act-lera", Resource: "grit", Value: &vttv1.Resource{Current: 9, Max: 9}}))
	if got := st.Actors["act-lera"].GetResources()["grit"]; got.GetCurrent() != 9 || got.GetMax() != 9 {
		t.Fatalf("grit added again = %v, want 9/9", got)
	}
}

// VTT-771: a resource held at 0 and 0 is a resource held, and is taken.
func TestQAResourceRemovedTakesAZeroValuedResource(t *testing.T) {
	st, l := qaRCWorld(t)
	if err := engine.Apply(st, l.env(&vttv1.ResourceRemoved{ActorId: "act-gob", Resource: "spark"})); err != nil {
		t.Fatalf("ResourceRemoved of a resource held at 0/0 refused: %v", err)
	}
	if _, ok := st.Actors["act-gob"].GetResources()["spark"]; ok {
		t.Fatal("act-gob still holds spark after its ResourceRemoved")
	}
}

// VTT-772: engine.Apply refuses a ResourceRemoved naming an actor it does not
// hold or a resource the actor does not hold, leaving the state unchanged, so
// the command that would append it appends nothing.
func TestQAResourceRemovedRefusals(t *testing.T) {
	cases := []struct {
		name string
		ev   *vttv1.ResourceRemoved
	}{
		{"unknown actor", &vttv1.ResourceRemoved{ActorId: "act-ghost", Resource: "pool"}},
		{"resource not held", &vttv1.ResourceRemoved{ActorId: "act-lera", Resource: "dust"}},
		{"resource another actor holds", &vttv1.ResourceRemoved{ActorId: "act-lera", Resource: "spark"}},
		{"an attribute's key", &vttv1.ResourceRemoved{ActorId: "act-lera", Resource: "nerve"}},
		{"empty resource name", &vttv1.ResourceRemoved{ActorId: "act-lera", Resource: ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, l := qaRCWorld(t)
			before := qaRCJSON(t, st)
			err := engine.Apply(st, l.env(c.ev))
			if err == nil {
				t.Fatalf("ResourceRemoved %v accepted; want a refusal", c.ev)
			}
			t.Logf("refusal: %v", err)
			if after := qaRCJSON(t, st); after != before {
				t.Fatalf("a refused ResourceRemoved changed the state\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
	t.Run("a resource already removed", func(t *testing.T) {
		st, l := qaRCWorld(t)
		qaRCMustApply(t, st, l.env(&vttv1.ResourceRemoved{ActorId: "act-gob", Resource: "pool"}))
		before := qaRCJSON(t, st)
		if err := engine.Apply(st, l.env(&vttv1.ResourceRemoved{ActorId: "act-gob", Resource: "pool"})); err == nil {
			t.Fatal("a second ResourceRemoved of the same resource accepted")
		}
		if after := qaRCJSON(t, st); after != before {
			t.Fatalf("a refused ResourceRemoved changed the state\nbefore: %s\nafter:  %s", before, after)
		}
	})
}

// VTT-773: engine.Apply changes nothing at a ResourceAdded or a
// ResourceRemoved but the named resource of the named actor: every scene,
// token, session, condition, note, every other actor (one holding a resource
// of the same name included) and every other field of the named actor (an
// attribute of the same name included) is as it was.
func TestQAResourceEventsChangeNothingElse(t *testing.T) {
	cases := []struct {
		name  string
		ev    any
		actor string
		res   string
	}{
		{"added", &vttv1.ResourceAdded{ActorId: "act-lera", Resource: "focus", Value: &vttv1.Resource{Current: 3, Max: 5}}, "act-lera", "focus"},
		{"removed", &vttv1.ResourceRemoved{ActorId: "act-gob", Resource: "focus"}, "act-gob", "focus"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, l := qaRCWorld(t)
			before := st.Snapshot()
			qaRCMustApply(t, st, l.env(c.ev))
			after := st.Snapshot()
			_, heldBefore := before.Actors[c.actor].GetResources()[c.res]
			_, heldAfter := after.Actors[c.actor].GetResources()[c.res]
			if heldBefore == heldAfter {
				t.Fatalf("%s.%s held before=%v after=%v; the event changed nothing", c.actor, c.res, heldBefore, heldAfter)
			}
			delete(before.Actors[c.actor].Resources, c.res)
			delete(after.Actors[c.actor].Resources, c.res)
			if b, a := qaRCJSON(t, before), qaRCJSON(t, after); b != a {
				t.Fatalf("the event changed more than %s.%s\nbefore: %s\nafter:  %s", c.actor, c.res, b, a)
			}
		})
	}
}

// VTT-792: a caller holding the actor's resource map from before the event
// sees it unchanged.
func TestQAResourceEventsLeaveTheOldMapAlone(t *testing.T) {
	t.Run("added", func(t *testing.T) {
		st, l := qaRCWorld(t)
		old := st.Actors["act-lera"].GetResources()
		qaRCMustApply(t, st, l.env(&vttv1.ResourceAdded{ActorId: "act-lera", Resource: "focus", Value: &vttv1.Resource{Current: 3, Max: 5}}))
		if _, ok := old["focus"]; ok {
			t.Fatal("the resource map held before the ResourceAdded gained focus")
		}
		if len(old) != 2 {
			t.Fatalf("the old map holds %d resources, want 2", len(old))
		}
	})
	t.Run("removed", func(t *testing.T) {
		st, l := qaRCWorld(t)
		old := st.Actors["act-lera"].GetResources()
		qaRCMustApply(t, st, l.env(&vttv1.ResourceRemoved{ActorId: "act-lera", Resource: "grit"}))
		if _, ok := old["grit"]; !ok {
			t.Fatal("the resource map held before the ResourceRemoved lost grit")
		}
	})
}
