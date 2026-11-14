package engine_test

import (
	"reflect"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func abilityGiven(seq int64, actorID, abilityID string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{
			ActorId: actorID, AbilityId: abilityID}}}
}

func abilityTaken(seq int64, actorID, abilityID string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{
			ActorId: actorID, AbilityId: abilityID}}}
}

// abilityTable holds a1 with a value in every field an event can set, its
// list out of byte order and naming rally twice, beside a2, a token, a
// condition and a note.
func abilityTable(t *testing.T) *engine.State {
	t.Helper()
	st := seedScene(t)
	lore, err := structpb.NewStruct(map[string]any{"lore": "an old oath"})
	must(t, err)
	must(t, engine.Apply(st, env(3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "a1", Name: "Ada", ModuleId: "m", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		Attributes: map[string]int32{"brawn": 2},
		Resources:  map[string]*vttv1.Resource{"stamina": {Current: 3, Max: 5}},
		ModuleData: lore,
		AbilityIds: []string{"rally", "fists", "rally", "lunge"}}})))
	must(t, engine.Apply(st, env(4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "a2", Name: "Bo", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		AbilityIds: []string{"rally"}}})))
	must(t, engine.Apply(st, env(5, &vttv1.TokenPlaced{TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 1, Y: 1}, Name: "a cloaked shape"})))
	must(t, engine.Apply(st, env(6, &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: "p1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})))
	must(t, engine.Apply(st, env(7, &vttv1.ConditionApplied{ActorId: "a1", ConditionId: "marked", Source: "trap"})))
	must(t, engine.Apply(st, env(8, &vttv1.NoteUpserted{Key: "k", Title: "T", Text: "x",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC})))
	return st
}

// sameExceptAbilities fails unless the snapshot after is the snapshot before
// with a1's ability_ids replaced by want, walking every field of every actor
// and of the state. Compare snapshots alone: Snapshot makes a nil map empty.
func sameExceptAbilities(t *testing.T, before, after *engine.State, want []string) {
	t.Helper()
	if got := after.Actors["a1"].GetAbilityIds(); !slices.Equal(got, want) {
		t.Errorf("a1 names %q, want %q", got, want)
	}
	if len(after.Actors) != len(before.Actors) {
		t.Fatalf("the state holds %d actors, want the %d it held", len(after.Actors), len(before.Actors))
	}
	for id, was := range before.Actors {
		now, ok := after.Actors[id]
		if !ok {
			t.Fatalf("actor %q left the state", id)
		}
		fields := was.ProtoReflect().Descriptor().Fields()
		for i := range fields.Len() {
			fd := fields.Get(i)
			if id == "a1" && fd.Name() == "ability_ids" {
				continue
			}
			if !proto.Equal(only(was, fd), only(now, fd)) {
				t.Errorf("actor %q's %s changed: was %v, now %v", id, fd.Name(),
					was.ProtoReflect().Get(fd), now.ProtoReflect().Get(fd))
			}
		}
	}
	for name, pair := range map[string][2]any{
		"Scenes":     {before.Scenes, after.Scenes},
		"Tokens":     {before.Tokens, after.Tokens},
		"Sessions":   {before.Sessions, after.Sessions},
		"Conditions": {before.Conditions, after.Conditions},
		"Notes":      {before.Notes, after.Notes},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("the state's %s changed: was %v, now %v", name, pair[0], pair[1])
		}
	}
}

// only is a copy of a holding fd alone.
func only(a *vttv1.Actor, fd protoreflect.FieldDescriptor) proto.Message {
	c := proto.Clone(a)
	m := c.ProtoReflect()
	m.Range(func(f protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		if f != fd {
			m.Clear(f)
		}
		return true
	})
	return c
}

// VTT-715
func TestAnAbilityGivenEndsTheListAndChangesNothingElse(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	must(t, engine.Apply(st, abilityGiven(9, "a1", "dodge")))
	sameExceptAbilities(t, before, st.Snapshot(), []string{"rally", "fists", "rally", "lunge", "dodge"})
}

// VTT-715
func TestAnAbilityTakenLeavesNoOccurrenceAndChangesNothingElse(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	held := st.Actors["a1"].GetAbilityIds()
	must(t, engine.Apply(st, abilityTaken(9, "a1", "rally")))
	sameExceptAbilities(t, before, st.Snapshot(), []string{"fists", "lunge"})
	if !slices.Equal(held, []string{"rally", "fists", "rally", "lunge"}) {
		t.Errorf("a list read before the take now reads %q: the take wrote into it", held)
	}
}

// VTT-715
func TestAGiveAfterATakeEndsTheListWithTheId(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	must(t, engine.Apply(st, abilityTaken(9, "a1", "fists")))
	must(t, engine.Apply(st, abilityGiven(10, "a1", "fists")))
	if got := st.Actors["a1"].GetAbilityIds(); !slices.Equal(got, []string{"rally", "rally", "lunge", "fists"}) {
		t.Errorf("a1 names %q, want [rally rally lunge fists]", got)
	}
	must(t, engine.Apply(st, abilityTaken(11, "a1", "lunge")))
	must(t, engine.Apply(st, abilityTaken(12, "a1", "rally")))
	must(t, engine.Apply(st, abilityTaken(13, "a1", "fists")))
	if got := st.Actors["a1"].GetAbilityIds(); len(got) != 0 {
		t.Errorf("a1 names %q after every id was taken, want none", got)
	}
	sameExceptAbilities(t, before, st.Snapshot(), nil)
}

// refusedUnchanged applies env, wants exactly msg, and wants st as it was.
func refusedUnchanged(t *testing.T, st *engine.State, env *vttv1.Envelope, msg string) {
	t.Helper()
	before := st.Snapshot()
	err := engine.Apply(st, env)
	if err == nil || err.Error() != msg {
		t.Fatalf("Apply = %v, want %q", err, msg)
	}
	sameExceptAbilities(t, before, st.Snapshot(), before.Actors["a1"].GetAbilityIds())
}

// VTT-722
func TestAnAbilityGivenWithAnEmptyIdIsRefused(t *testing.T) {
	st := abilityTable(t)
	refusedUnchanged(t, st, abilityGiven(9, "a1", ""), "engine: ability_given requires an ability id")
	refusedUnchanged(t, st, abilityGiven(9, "ghost", ""), "engine: ability_given requires an ability id")
}

// VTT-717
func TestAnAbilityGivenOrTakenNamingNoActorIsRefused(t *testing.T) {
	st := abilityTable(t)
	refusedUnchanged(t, st, abilityGiven(9, "ghost", "rally"), `engine: ability_given names unknown actor "ghost"`)
	refusedUnchanged(t, st, abilityTaken(9, "ghost", "rally"), `engine: ability_taken names unknown actor "ghost"`)
	refusedUnchanged(t, st, abilityTaken(9, "", "rally"), `engine: ability_taken names unknown actor ""`)
}

// VTT-720
func TestAnAbilityGivenTheSheetAlreadyNamesIsRefused(t *testing.T) {
	st := abilityTable(t)
	refusedUnchanged(t, st, abilityGiven(9, "a1", "lunge"), `engine: actor "a1" already names ability "lunge"`)
	refusedUnchanged(t, st, abilityGiven(9, "a2", "rally"), `engine: actor "a2" already names ability "rally"`)
}

// VTT-721
func TestAnAbilityTakenTheSheetDoesNotNameIsRefused(t *testing.T) {
	st := abilityTable(t)
	refusedUnchanged(t, st, abilityTaken(9, "a2", "fists"), `engine: actor "a2" does not name ability "fists"`)
	must(t, engine.Apply(st, abilityTaken(9, "a1", "fists")))
	refusedUnchanged(t, st, abilityTaken(10, "a1", "fists"), `engine: actor "a1" does not name ability "fists"`)
}
