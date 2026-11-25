package engine_test

import (
	"encoding/json"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func qarfRef(entry, place, state string) *vttv1.Reference {
	return &vttv1.Reference{EntryId: entry, Place: place, State: state}
}

func qarfApplyOK(t *testing.T, st *engine.State, env *vttv1.Envelope) {
	t.Helper()
	if err := engine.Apply(st, env); err != nil {
		t.Fatalf("fixture event refused: %v", err)
	}
}

func qarfAdded(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: a},
	}}
}

func qarfRefAdded(actor, id string, ref *vttv1.Reference) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceAdded{
		ReferenceAdded: &vttv1.ReferenceAdded{
			ActorId: actor, ReferenceId: id, Reference: ref,
		},
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

// Keep two of these on one entry, and every reference id apart from its entry.
func qarfSheetA(t *testing.T) *vttv1.Actor {
	t.Helper()
	md, err := structpb.NewStruct(map[string]any{"kilo": 3.0, "mike": "oscar"})
	if err != nil {
		t.Fatal(err)
	}
	return &vttv1.Actor{
		ActorId:    "a-1",
		Name:       "First",
		ModuleId:   "mod-q",
		Attributes: map[string]int32{"kilo": 4, "mike": -2},
		Resources: map[string]*vttv1.Resource{
			"lima": {Current: 2, Max: 5},
		},
		ModuleData: md,
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		AbilityIds: []string{"echo-ab", "delta-ab"},
		References: map[string]*vttv1.Reference{
			"r-1": qarfRef("echo", "mike", "lima"),
			"r-2": qarfRef("echo", "kilo", "oscar"),
			"r-3": qarfRef("delta", "", ""),
		},
	}
}

func qarfSheetB() *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    "a-2",
		Name:       "Second",
		ModuleId:   "mod-q",
		Attributes: map[string]int32{"kilo": 1},
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		References: map[string]*vttv1.Reference{
			"r-2": qarfRef("delta", "mike", "oscar"),
			"r-9": qarfRef("echo", "kilo", "lima"),
		},
	}
}

// Keep every kind of state in this world: qarfOthersUnchanged reads them all.
func qarfWorld(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	events := []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_RulesetNamed{RulesetNamed: &vttv1.RulesetNamed{
			RulesetId: "oscar-set", Version: "1.0.0", Name: "Oscar",
		}}},
		{SessionId: "s-1", Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "first"},
		}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "kilo", Name: "Kilo", GridWidth: 4, GridHeight: 4,
			Tiles: map[string]*vttv1.TileRef{
				"1,1": {Kind: "floor"}, "2,1": {Kind: "wall"},
			},
		}}},
		qarfAdded(qarfSheetA(t)),
		qarfAdded(qarfSheetB()),
		qarfAdded(&vttv1.Actor{
			ActorId: "a-4", ModuleId: "mod-q",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}),
		{Payload: &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{
				ActorId: "a-1", ParticipantId: "p-1",
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			},
		}},
		{Payload: &vttv1.Envelope_ConditionApplied{
			ConditionApplied: &vttv1.ConditionApplied{
				ActorId: "a-1", ConditionId: "oscar-c", Source: "kilo",
			},
		}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "t-1", SceneId: "kilo", ActorId: "a-1",
			Position: &vttv1.GridPosition{X: 1, Y: 1}, Name: "tok",
		}}},
		{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key: "n-1", Title: "T", Text: "X",
			Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		}}},
	}
	for i, env := range events {
		env.Sequence = int64(i + 1)
		qarfApplyOK(t, st, env)
	}
	return st
}

type qarfCopy struct {
	actors map[string]*vttv1.Actor
	rest   string
}

// Copy without engine.Snapshot: an aliasing Snapshot would hide a write.
func qarfTake(t *testing.T, st *engine.State) qarfCopy {
	t.Helper()
	actors := make(map[string]*vttv1.Actor, len(st.Actors))
	for id, a := range st.Actors {
		actors[id] = proto.Clone(a).(*vttv1.Actor)
	}
	rest, err := json.Marshal(struct {
		Scenes, Tokens, Sessions, Conditions, Notes any
		RID, RVer, RName                            string
	}{
		st.Scenes, st.Tokens, st.Sessions, st.Conditions, st.Notes,
		st.RulesetID, st.RulesetVersion, st.RulesetName,
	})
	if err != nil {
		t.Fatal(err)
	}
	return qarfCopy{actors: actors, rest: string(rest)}
}

func qarfUnchanged(t *testing.T, before qarfCopy, st *engine.State) {
	t.Helper()
	qarfOthersUnchanged(t, before, st, "")
}

func qarfOthersUnchanged(t *testing.T, before qarfCopy, st *engine.State, skip string) {
	t.Helper()
	after := qarfTake(t, st)
	if after.rest != before.rest {
		t.Errorf("non-actor state changed:\nbefore %s\nafter  %s", before.rest, after.rest)
	}
	wantN := len(before.actors)
	if _, was := before.actors[skip]; !was && skip != "" {
		wantN++
	}
	if len(after.actors) != wantN {
		t.Errorf("actor count %d, want %d", len(after.actors), wantN)
	}
	for id, b := range before.actors {
		if id == skip {
			continue
		}
		if !proto.Equal(b, after.actors[id]) {
			t.Errorf("actor %q changed:\nbefore %v\nafter  %v", id, b, after.actors[id])
		}
	}
}

func qarfOnlyRefs(t *testing.T, before qarfCopy, st *engine.State, id string, want map[string]*vttv1.Reference) {
	t.Helper()
	qarfOthersUnchanged(t, before, st, id)
	got, ok := st.Actors[id]
	if !ok {
		t.Fatalf("actor %q gone", id)
	}
	g := proto.Clone(got).(*vttv1.Actor)
	b := proto.Clone(before.actors[id]).(*vttv1.Actor)
	g.References, b.References = nil, nil
	if !proto.Equal(g, b) {
		t.Errorf("actor %q changed beyond its references:\nbefore %v\nafter  %v", id, b, g)
	}
	qarfRefsEqual(t, got.GetReferences(), want)
}

func qarfRefsEqual(t *testing.T, got, want map[string]*vttv1.Reference) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("references %v, want %v", got, want)
		return
	}
	for id, w := range want {
		g, ok := got[id]
		if !ok || !proto.Equal(g, w) {
			t.Errorf("reference %q = %v, want %v", id, g, w)
		}
	}
}

func qarfCloneRefs(m map[string]*vttv1.Reference) map[string]*vttv1.Reference {
	out := make(map[string]*vttv1.Reference, len(m))
	for k, v := range m {
		out[k] = proto.Clone(v).(*vttv1.Reference)
	}
	return out
}

func qarfRefused(t *testing.T, build func(*testing.T) *engine.State, env *vttv1.Envelope) string {
	t.Helper()
	st := build(t)
	before := qarfTake(t, st)
	err := engine.Apply(st, proto.Clone(env).(*vttv1.Envelope))
	if err == nil {
		t.Fatalf("Apply accepted %v", env)
	}
	// docs/blueprint.md principle 9.
	qarfUnchanged(t, before, st)
	return err.Error()
}

func qarfWorldWith(extra ...*vttv1.Envelope) func(*testing.T) *engine.State {
	return func(t *testing.T) *engine.State {
		t.Helper()
		st := qarfWorld(t)
		for _, env := range extra {
			qarfApplyOK(t, st, env)
		}
		return st
	}
}

var qarfLong = strings.Repeat("x", 300)

// VTT-1023.
func TestQARFActorAddedKeepsItsReferencesWhole(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	refs := map[string]*vttv1.Reference{
		"r-2":    qarfRef("echo", "mike", "lima"),
		"r-9":    qarfRef("delta", "kilo", "oscar"),
		"r-5":    qarfRef("echo", "", "oscar"),
		qarfLong: qarfRef(qarfLong, qarfLong, qarfLong),
	}
	actor := &vttv1.Actor{
		ActorId: "a-3", Name: "Third", ModuleId: "mod-q",
		Attributes: map[string]int32{"kilo": 7},
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		References: qarfCloneRefs(refs),
	}
	qarfApplyOK(t, st, qarfAdded(actor))
	got, ok := st.Actors["a-3"]
	if !ok {
		t.Fatal("actor a-3 not added")
	}
	qarfRefsEqual(t, got.GetReferences(), refs)
	if !proto.Equal(got, actor) {
		t.Errorf("actor stored %v, want %v", got, actor)
	}
	qarfOthersUnchanged(t, before, st, "a-3")
}

// VTT-1023.
func TestQARFActorAddedHoldsNoReferenceForAnActorWithNone(t *testing.T) {
	st := qarfWorld(t)
	if n := len(st.Actors["a-4"].GetReferences()); n != 0 {
		t.Errorf("a-4 holds %d references, want 0", n)
	}
}

// VTT-1023.
func TestQARFActorAddedRefusesAnEmptyReferenceId(t *testing.T) {
	cases := map[string]map[string]*vttv1.Reference{
		"beside others": {
			"r-1": qarfRef("echo", "mike", "lima"),
			"":    qarfRef("delta", "kilo", "oscar"),
			"r-7": qarfRef("delta", "mike", "oscar"),
		},
		"alone": {"": qarfRef("echo", "kilo", "lima")},
	}
	for name, refs := range cases {
		t.Run(name, func(t *testing.T) {
			env := qarfAdded(&vttv1.Actor{
				ActorId: "a-3", ModuleId: "mod-q",
				Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
				References: refs,
			})
			qarfRefused(t, qarfWorld, env)
		})
	}
}

// VTT-290, VTT-297, VTT-1023, SPEC-018.
func TestQARFActorAddedRefusesItsAttributeNamesBeforeAnEmptyReferenceId(t *testing.T) {
	bad := map[string]*vttv1.Reference{"": qarfRef("echo", "mike", "lima")}
	good := map[string]*vttv1.Reference{"r-1": qarfRef("echo", "mike", "lima")}
	long := strings.Repeat("k", 129)
	faults := map[string]func(*vttv1.Actor){
		"long attribute name": func(a *vttv1.Actor) {
			a.Attributes = map[string]int32{long: 1, "kilo": 2}
		},
		"empty attribute name": func(a *vttv1.Actor) {
			a.Attributes = map[string]int32{"": 1, "kilo": 2}
		},
		"long resource name": func(a *vttv1.Actor) {
			a.Resources = map[string]*vttv1.Resource{long: {Current: 1}}
		},
		"duplicate actor": func(a *vttv1.Actor) { a.ActorId = "a-2" },
	}
	for name, fault := range faults {
		t.Run(name, func(t *testing.T) {
			mk := func(refs map[string]*vttv1.Reference) *vttv1.Envelope {
				a := &vttv1.Actor{
					ActorId: "a-3", ModuleId: "mod-q",
					Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
					References: qarfCloneRefs(refs),
				}
				fault(a)
				return qarfAdded(a)
			}
			both := qarfRefused(t, qarfWorld, mk(bad))
			first := qarfRefused(t, qarfWorld, mk(good))
			refOnly := qarfRefused(t, qarfWorld, qarfAdded(&vttv1.Actor{
				ActorId: "a-3", ModuleId: "mod-q",
				Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
				References: qarfCloneRefs(bad),
			}))
			if both != first {
				t.Errorf("two faults refused as %q, want the earlier fault's %q", both, first)
			}
			if first == refOnly {
				t.Fatalf("degenerate: both refusals read %q", first)
			}
		})
	}
}

// VTT-1023.
func TestQARFActorAddedWritesReferencesTheEventDoesNotShare(t *testing.T) {
	t.Run("event mutated after", func(t *testing.T) {
		st := qarfWorld(t)
		env := qarfAdded(qarfSheetA(t))
		env.GetActorAdded().Actor.ActorId = "a-3"
		want := qarfCloneRefs(env.GetActorAdded().GetActor().GetReferences())
		qarfApplyOK(t, st, env)
		refs := env.GetActorAdded().GetActor().GetReferences()
		refs["r-1"].Place = "kilo"
		delete(refs, "r-2")
		refs["r-8"] = qarfRef("delta", "", "")
		qarfRefsEqual(t, st.Actors["a-3"].GetReferences(), want)
	})
	t.Run("state mutated after", func(t *testing.T) {
		st := qarfWorld(t)
		env := qarfAdded(qarfSheetA(t))
		env.GetActorAdded().Actor.ActorId = "a-3"
		want := proto.Clone(env).(*vttv1.Envelope)
		qarfApplyOK(t, st, env)
		st.Actors["a-3"].References["r-1"].State = "oscar"
		if !proto.Equal(env, want) {
			t.Errorf("event changed by a write to state: %v", env)
		}
	})
	t.Run("later reference events", func(t *testing.T) {
		st := qarfWorld(t)
		env := qarfAdded(qarfSheetA(t))
		env.GetActorAdded().Actor.ActorId = "a-3"
		want := proto.Clone(env).(*vttv1.Envelope)
		qarfApplyOK(t, st, env)
		qarfApplyOK(t, st, qarfRefAdded("a-3", "r-8", qarfRef("delta", "kilo", "lima")))
		qarfApplyOK(t, st, qarfRefChanged("a-3", "r-1", "kilo", "oscar"))
		qarfApplyOK(t, st, qarfRefRemoved("a-3", "r-2"))
		if !proto.Equal(env, want) {
			t.Errorf("ActorAdded changed by later events:\nnow  %v\nwant %v", env, want)
		}
	})
}

// VTT-1024.
func TestQARFReferenceAddedAddsTheReferenceAndChangesNothingElse(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	// Keep r-9 on a-2 as well: two sheets may use one id.
	qarfApplyOK(t, st, qarfRefAdded("a-1", "r-9", qarfRef("echo", "kilo", "oscar")))
	want := qarfCloneRefs(before.actors["a-1"].GetReferences())
	want["r-9"] = qarfRef("echo", "kilo", "oscar")
	qarfOnlyRefs(t, before, st, "a-1", want)
}

// VTT-1024.
func TestQARFReferenceAddedToASheetHoldingNone(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	qarfApplyOK(t, st, qarfRefAdded("a-4", "r-1", qarfRef("delta", "mike", "lima")))
	qarfOnlyRefs(t, before, st, "a-4", map[string]*vttv1.Reference{
		"r-1": qarfRef("delta", "mike", "lima"),
	})
}

// VTT-1024.
func TestQARFReferenceAddedAddsAnEmptyButPresentReference(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	qarfApplyOK(t, st, qarfRefAdded("a-1", "r-6", &vttv1.Reference{}))
	want := qarfCloneRefs(before.actors["a-1"].GetReferences())
	want["r-6"] = &vttv1.Reference{}
	qarfOnlyRefs(t, before, st, "a-1", want)
	if st.Actors["a-1"].GetReferences()["r-6"] == nil {
		t.Error("r-6 stored as nil")
	}
}

// VTT-1024.
func TestQARFReferenceAddedBoundsNoLength(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	ref := qarfRef(qarfLong, qarfLong+"m", qarfLong+"l")
	qarfApplyOK(t, st, qarfRefAdded("a-1", qarfLong, ref))
	want := qarfCloneRefs(before.actors["a-1"].GetReferences())
	want[qarfLong] = ref
	qarfOnlyRefs(t, before, st, "a-1", want)
}

// VTT-1024.
func TestQARFReferenceAddedRefusesEachFault(t *testing.T) {
	cases := map[string]*vttv1.Envelope{
		"empty reference id":  qarfRefAdded("a-1", "", qarfRef("echo", "kilo", "lima")),
		"no reference":        qarfRefAdded("a-1", "r-6", nil),
		"unknown actor":       qarfRefAdded("a-8", "r-6", qarfRef("echo", "kilo", "lima")),
		"empty actor":         qarfRefAdded("", "r-6", qarfRef("echo", "kilo", "lima")),
		"id held already":     qarfRefAdded("a-1", "r-2", qarfRef("delta", "mike", "lima")),
		"held id, same value": qarfRefAdded("a-1", "r-2", qarfRef("echo", "kilo", "oscar")),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) { qarfRefused(t, qarfWorld, env) })
	}
}

// VTT-1024. Vary the world, not the event, for a fault of the world's: a
// refusal's words may name the event's fields.
func TestQARFReferenceAddedRefusesInItsOrder(t *testing.T) {
	ref := qarfRef("echo", "kilo", "lima")
	heldA8 := qarfWorldWith(qarfAdded(&vttv1.Actor{
		ActorId: "a-8", ModuleId: "mod-q",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		References: map[string]*vttv1.Reference{
			"r-7": qarfRef("delta", "mike", "oscar"),
		},
	}))
	cases := []struct {
		name               string
		both, first, later *vttv1.Envelope
		bothW, firstW      func(*testing.T) *engine.State
		laterW             func(*testing.T) *engine.State
	}{{
		name:  "empty id before no reference",
		both:  qarfRefAdded("a-1", "", nil),
		first: qarfRefAdded("a-1", "", ref), later: qarfRefAdded("a-1", "r-6", nil),
		bothW: qarfWorld, firstW: qarfWorld, laterW: qarfWorld,
	}, {
		name:  "empty id before unknown actor",
		both:  qarfRefAdded("a-8", "", ref),
		first: qarfRefAdded("a-8", "", ref), later: qarfRefAdded("a-8", "r-6", ref),
		bothW: qarfWorld, firstW: heldA8, laterW: qarfWorld,
	}, {
		name:  "no reference before unknown actor",
		both:  qarfRefAdded("a-8", "r-6", nil),
		first: qarfRefAdded("a-8", "r-6", nil), later: qarfRefAdded("a-8", "r-6", ref),
		bothW: qarfWorld, firstW: heldA8, laterW: qarfWorld,
	}, {
		name:  "no reference before id held already",
		both:  qarfRefAdded("a-1", "r-2", nil),
		first: qarfRefAdded("a-1", "r-2", nil), later: qarfRefAdded("a-1", "r-2", ref),
		bothW: qarfWorld, firstW: qarfWorldWith(qarfRefRemoved("a-1", "r-2")),
		laterW: qarfWorld,
	}, {
		name:  "empty id, no reference, unknown actor",
		both:  qarfRefAdded("a-8", "", nil),
		first: qarfRefAdded("a-8", "", ref), later: qarfRefAdded("a-8", "r-6", nil),
		bothW: qarfWorld, firstW: heldA8, laterW: qarfWorld,
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			both := qarfRefused(t, c.bothW, c.both)
			first := qarfRefused(t, c.firstW, c.first)
			later := qarfRefused(t, c.laterW, c.later)
			if both != first {
				t.Errorf("refused as %q, want the earlier fault's %q", both, first)
			}
			if first == later {
				t.Fatalf("degenerate: both faults read %q", first)
			}
		})
	}
}

// VTT-1024.
func TestQARFReferenceAddedWritesAFreshMap(t *testing.T) {
	t.Run("old map", func(t *testing.T) {
		st := qarfWorld(t)
		old := st.Actors["a-1"].References
		want := qarfCloneRefs(old)
		qarfApplyOK(t, st, qarfRefAdded("a-1", "r-6", qarfRef("delta", "kilo", "lima")))
		qarfRefsEqual(t, old, want)
		if _, ok := st.Actors["a-1"].GetReferences()["r-6"]; !ok {
			t.Error("r-6 not added")
		}
	})
	t.Run("event mutated after", func(t *testing.T) {
		st := qarfWorld(t)
		env := qarfRefAdded("a-1", "r-6", qarfRef("delta", "kilo", "lima"))
		qarfApplyOK(t, st, env)
		env.GetReferenceAdded().Reference.Place = "mike"
		env.GetReferenceAdded().Reference.EntryId = "echo"
		got := st.Actors["a-1"].GetReferences()["r-6"]
		if !proto.Equal(got, qarfRef("delta", "kilo", "lima")) {
			t.Errorf("state followed the event: %v", got)
		}
	})
	t.Run("state mutated after", func(t *testing.T) {
		st := qarfWorld(t)
		env := qarfRefAdded("a-1", "r-6", qarfRef("delta", "kilo", "lima"))
		want := proto.Clone(env).(*vttv1.Envelope)
		qarfApplyOK(t, st, env)
		st.Actors["a-1"].References["r-6"].State = "oscar"
		if !proto.Equal(env, want) {
			t.Errorf("event followed the state: %v", env)
		}
	})
	t.Run("a later change", func(t *testing.T) {
		st := qarfWorld(t)
		env := qarfRefAdded("a-1", "r-6", qarfRef("delta", "kilo", "lima"))
		want := proto.Clone(env).(*vttv1.Envelope)
		qarfApplyOK(t, st, env)
		qarfApplyOK(t, st, qarfRefChanged("a-1", "r-6", "mike", "oscar"))
		if !proto.Equal(env, want) {
			t.Errorf("ReferenceAdded changed by a later change: %v", env)
		}
	})
}

// VTT-1025.
func TestQARFReferenceRemovedTakesItOffAndChangesNothingElse(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	// Keep r-2 on a-2 as well: two sheets may use one id.
	qarfApplyOK(t, st, qarfRefRemoved("a-1", "r-2"))
	want := qarfCloneRefs(before.actors["a-1"].GetReferences())
	delete(want, "r-2")
	qarfOnlyRefs(t, before, st, "a-1", want)
}

// VTT-1025.
func TestQARFReferenceRemovedOfEveryReferenceLeavesNone(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	qarfApplyOK(t, st, qarfRefRemoved("a-2", "r-9"))
	qarfApplyOK(t, st, qarfRefRemoved("a-2", "r-2"))
	qarfOnlyRefs(t, before, st, "a-2", map[string]*vttv1.Reference{})
}

// VTT-1025.
func TestQARFReferenceRemovedRefusesEachFault(t *testing.T) {
	cases := map[string]*vttv1.Envelope{
		"unknown actor":            qarfRefRemoved("a-8", "r-1"),
		"empty actor":              qarfRefRemoved("", "r-1"),
		"id another sheet holds":   qarfRefRemoved("a-1", "r-9"),
		"an entry id, not a ref":   qarfRefRemoved("a-1", "echo"),
		"empty id":                 qarfRefRemoved("a-1", ""),
		"sheet holding none":       qarfRefRemoved("a-4", "r-1"),
		"removed already":          qarfRefRemoved("a-1", "r-6"),
		"case differs from r-1 id": qarfRefRemoved("a-1", "R-1"),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) { qarfRefused(t, qarfWorld, env) })
	}
}

// VTT-1025, VTT-1026.
func TestQARFAnUnknownActorIsRefusedBeforeAnIdNotHeld(t *testing.T) {
	heldA8 := qarfWorldWith(qarfAdded(&vttv1.Actor{
		ActorId: "a-8", ModuleId: "mod-q",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		References: map[string]*vttv1.Reference{
			"r-7": qarfRef("delta", "mike", "oscar"),
		},
	}))
	cases := map[string]*vttv1.Envelope{
		"removed": qarfRefRemoved("a-8", "r-1"),
		"changed": qarfRefChanged("a-8", "r-1", "kilo", "lima"),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			unknown := qarfRefused(t, qarfWorld, env)
			notHeld := qarfRefused(t, heldA8, env)
			if unknown == notHeld {
				t.Errorf("an unknown actor refused as an id not held: %q", unknown)
			}
		})
	}
}

// VTT-1025.
func TestQARFReferenceRemovedWritesAFreshMap(t *testing.T) {
	st := qarfWorld(t)
	old := st.Actors["a-1"].References
	want := qarfCloneRefs(old)
	qarfApplyOK(t, st, qarfRefRemoved("a-1", "r-2"))
	qarfRefsEqual(t, old, want)
	if _, ok := st.Actors["a-1"].GetReferences()["r-2"]; ok {
		t.Error("r-2 not removed")
	}
}

// VTT-1026.
func TestQARFReferenceChangedSetsPlaceAndStateKeepingItsEntry(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	// Keep r-2 on a-2 as well: two sheets may use one id.
	qarfApplyOK(t, st, qarfRefChanged("a-1", "r-2", "mike", "lima"))
	want := qarfCloneRefs(before.actors["a-1"].GetReferences())
	want["r-2"] = qarfRef("echo", "mike", "lima")
	qarfOnlyRefs(t, before, st, "a-1", want)
}

// VTT-1026.
func TestQARFReferenceChangedSetsTheEventsValuesWhatever(t *testing.T) {
	cases := []struct {
		name, place, state string
		want               *vttv1.Reference
	}{
		{"both emptied", "", "", qarfRef("echo", "", "")},
		{"state alone", "kilo", "lima", qarfRef("echo", "kilo", "lima")},
		{"place alone", "mike", "oscar", qarfRef("echo", "mike", "oscar")},
		{"as it was", "kilo", "oscar", qarfRef("echo", "kilo", "oscar")},
		{"long", qarfLong, qarfLong, qarfRef("echo", qarfLong, qarfLong)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := qarfWorld(t)
			before := qarfTake(t, st)
			qarfApplyOK(t, st, qarfRefChanged("a-1", "r-2", c.place, c.state))
			want := qarfCloneRefs(before.actors["a-1"].GetReferences())
			want["r-2"] = c.want
			qarfOnlyRefs(t, before, st, "a-1", want)
		})
	}
	t.Run("an empty place filled", func(t *testing.T) {
		st := qarfWorld(t)
		before := qarfTake(t, st)
		qarfApplyOK(t, st, qarfRefChanged("a-1", "r-3", "kilo", "oscar"))
		want := qarfCloneRefs(before.actors["a-1"].GetReferences())
		want["r-3"] = qarfRef("delta", "kilo", "oscar")
		qarfOnlyRefs(t, before, st, "a-1", want)
	})
}

// VTT-1026.
func TestQARFReferenceChangedRefusesEachFault(t *testing.T) {
	cases := map[string]*vttv1.Envelope{
		"unknown actor":          qarfRefChanged("a-8", "r-1", "kilo", "lima"),
		"empty actor":            qarfRefChanged("", "r-1", "kilo", "lima"),
		"id another sheet holds": qarfRefChanged("a-1", "r-9", "kilo", "lima"),
		"an entry id, not a ref": qarfRefChanged("a-1", "echo", "kilo", "lima"),
		"empty id":               qarfRefChanged("a-1", "", "kilo", "lima"),
		"sheet holding none":     qarfRefChanged("a-4", "r-1", "kilo", "lima"),
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) { qarfRefused(t, qarfWorld, env) })
	}
}

// VTT-1026.
func TestQARFReferenceChangedWritesAFreshMap(t *testing.T) {
	t.Run("old map", func(t *testing.T) {
		st := qarfWorld(t)
		old := st.Actors["a-1"].References
		want := qarfCloneRefs(old)
		qarfApplyOK(t, st, qarfRefChanged("a-1", "r-2", "mike", "lima"))
		qarfRefsEqual(t, old, want)
		got := st.Actors["a-1"].GetReferences()["r-2"]
		if !proto.Equal(got, qarfRef("echo", "mike", "lima")) {
			t.Errorf("r-2 = %v", got)
		}
	})
	t.Run("ActorAdded's references", func(t *testing.T) {
		st := engine.NewState()
		env := qarfAdded(qarfSheetA(t))
		want := proto.Clone(env).(*vttv1.Envelope)
		qarfApplyOK(t, st, env)
		qarfApplyOK(t, st, qarfRefChanged("a-1", "r-2", "mike", "lima"))
		if !proto.Equal(env, want) {
			t.Errorf("ActorAdded changed by a change: %v", env)
		}
	})
}

// VTT-1024, VTT-1025, VTT-1026.
func TestQARFAReferenceRemovedCanBeAddedAgain(t *testing.T) {
	st := qarfWorld(t)
	before := qarfTake(t, st)
	qarfApplyOK(t, st, qarfRefRemoved("a-1", "r-1"))
	qarfApplyOK(t, st, qarfRefAdded("a-1", "r-1", qarfRef("delta", "kilo", "oscar")))
	qarfApplyOK(t, st, qarfRefChanged("a-1", "r-1", "mike", "lima"))
	want := qarfCloneRefs(before.actors["a-1"].GetReferences())
	want["r-1"] = qarfRef("delta", "mike", "lima")
	qarfOnlyRefs(t, before, st, "a-1", want)
}
