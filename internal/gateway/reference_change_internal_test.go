package gateway

import (
	"maps"
	"reflect"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func sheetAddReference(actorID, id string, r *vttv1.Reference) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "add-reference", Command: &vttv1.ClientCommand_AddReference{
		AddReference: &vttv1.AddReference{ActorId: actorID, ReferenceId: id, Reference: r}}}
}

func sheetChangeReference(actorID, id, place, state string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "change-reference", Command: &vttv1.ClientCommand_ChangeReference{
		ChangeReference: &vttv1.ChangeReference{ActorId: actorID, ReferenceId: id, Place: place, State: state}}}
}

func sheetRemoveReference(actorID, id string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "remove-reference", Command: &vttv1.ClientCommand_RemoveReference{
		RemoveReference: &vttv1.RemoveReference{ActorId: actorID, ReferenceId: id}}}
}

// referenceSheet serves rs, or none when nil, over a campaign holding fit,
// whose sheet holds refs, and other, whose sheet holds k as delta in kilo.
// Neither sheet is read against rs on its way in.
func referenceSheet(t *testing.T, rs *rules.Ruleset, refs map[string]*vttv1.Reference) (*Server, *campaign.Campaign, *identity.Participant) {
	t.Helper()
	s, camp, dm := fitServer(t, rs)
	seatAppend(t, camp, "fit-added", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "fit", Name: "Fit",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, References: refs}})
	seatAppend(t, camp, "other-added", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "other", Name: "Other",
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		References: map[string]*vttv1.Reference{"k": reference("delta", "kilo", "")}}})
	return s, camp, dm
}

// fitSheet is fit's sheet: a, echo in mike, and k, delta in kilo.
func fitSheet(more map[string]*vttv1.Reference) map[string]*vttv1.Reference {
	refs := map[string]*vttv1.Reference{"a": reference("echo", "mike", "lima"), "k": reference("delta", "kilo", "")}
	maps.Copy(refs, more)
	return refs
}

func referenceRulesets(t *testing.T) map[string]*rules.Ruleset {
	t.Helper()
	return map[string]*rules.Ruleset{"no ruleset": nil, "a ruleset": referenceRuleset(t)}
}

func issuerFor(who string, dm *identity.Participant) *identity.Participant {
	if who == "agent" {
		return &identity.Participant{ID: "agent-1", Role: identity.RoleAgent}
	}
	return dm
}

func logHead(t *testing.T, camp *campaign.Campaign) int64 {
	t.Helper()
	_, unsubscribe, head, err := camp.Subscribe(0, 1<<12)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	unsubscribe()
	return head
}

// issueOne sends cmd as p, wants it accepted as the one envelope the log gains
// and the rest of the state unchanged but fit's references, and returns it.
func issueOne(t *testing.T, s *Server, camp *campaign.Campaign, p *identity.Participant, cmd *vttv1.ClientCommand,
	want map[string]*vttv1.Reference) *vttv1.Envelope {
	t.Helper()
	before, head := camp.State().Snapshot(), logHead(t, camp)
	r := s.handleCommand(p, cmd, nil)
	if !r.GetOk() || r.GetSequence() != head+1 {
		t.Fatalf("%v answered %+v, want ok at sequence %d", cmd.GetCommand(), r, head+1)
	}
	env := appendedAt(t, camp, head+1)
	sameStateButReferences(t, before, camp.State(), want)
	return env
}

func issueRefused(t *testing.T, s *Server, camp *campaign.Campaign, p *identity.Participant, cmd *vttv1.ClientCommand, want string) {
	t.Helper()
	head := logHead(t, camp)
	if r := s.handleCommand(p, cmd, nil); r.GetOk() || r.GetError() != want {
		t.Errorf("%v answered %+v, want %q", cmd.GetCommand(), r, want)
	}
	if got := logHead(t, camp); got != head {
		t.Errorf("%v moved the log's head from %d to %d", cmd.GetCommand(), head, got)
	}
}

// sameStateButReferences wants after to be before with fit's references want.
func sameStateButReferences(t *testing.T, before, after *engine.State, want map[string]*vttv1.Reference) {
	t.Helper()
	for name, pair := range map[string][2]any{
		"scenes": {before.Scenes, after.Scenes}, "tokens": {before.Tokens, after.Tokens},
		"sessions": {before.Sessions, after.Sessions}, "conditions": {before.Conditions, after.Conditions},
		"notes": {before.Notes, after.Notes},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("the state's %s changed: %v, then %v", name, pair[0], pair[1])
		}
	}
	if ids, now := slices.Sorted(maps.Keys(before.Actors)), slices.Sorted(maps.Keys(after.Actors)); !slices.Equal(ids, now) {
		t.Errorf("the state's actors changed: %v, then %v", ids, now)
	}
	for aid, a := range before.Actors {
		wantActor := proto.Clone(a).(*vttv1.Actor)
		if aid == "fit" {
			wantActor.References = want
		}
		if !proto.Equal(after.Actors[aid], wantActor) {
			t.Errorf("actor %q is %v, want %v", aid, after.Actors[aid], wantActor)
		}
	}
}

// Keep mike's additions naming one entry twice: a place that holds copies
// holds any number of references.
// VTT-1046
func TestAnAddReferenceAppendsOneReferenceAddedAndChangesOnlyTheSheet(t *testing.T) {
	adds := []struct {
		id string
		r  *vttv1.Reference
	}{
		{"b", reference("echo", "mike", "oscar")},
		{"c", reference("delta", "mike", "lima")},
		{"d", reference("echo", "mike", "lima")},
	}
	for name, rs := range referenceRulesets(t) {
		for _, who := range []string{"dm", "agent"} {
			t.Run(name+"/"+who, func(t *testing.T) {
				s, camp, dm := referenceSheet(t, rs, fitSheet(nil))
				want := fitSheet(nil)
				for _, add := range adds {
					want[add.id] = add.r
					env := issueOne(t, s, camp, issuerFor(who, dm), sheetAddReference("fit", add.id, add.r), maps.Clone(want))
					added := env.GetReferenceAdded()
					if added.GetActorId() != "fit" || added.GetReferenceId() != add.id || !proto.Equal(added.GetReference(), add.r) {
						t.Fatalf("the addition appended %v, want a ReferenceAdded of %q as %v to fit", env, add.id, add.r)
					}
				}
			})
		}
	}
}

// Keep b and z either side of k: the refusal names the reference added and the
// one it meets, whichever sorts first.
// VTT-1046
func TestWithARulesetAnAddReferenceThatDoesNotFitIsRefusedAndAppendsNothing(t *testing.T) {
	cases := []struct {
		name, actor, id string
		r               *vttv1.Reference
		want            string
	}{
		{"an entry the catalogue does not hold", "fit", "b", reference("victor", "mike", "lima"),
			`reference "b" names entry "victor", which the catalogue does not hold`},
		{"a place the ruleset does not declare", "fit", "b", reference("echo", "whiskey", ""),
			`reference "b" is in place "whiskey", which the ruleset does not declare`},
		{"an entry of a form its place does not hold", "fit", "b", reference("echo", "kilo", ""),
			`reference "b" names entry "echo" of form "zulu", which place "kilo" does not hold`},
		{"a state its place does not name", "fit", "b", reference("echo", "mike", "papa"),
			`reference "b" is in state "papa", which place "mike" does not name`},
		{"no state where its place names some", "fit", "b", reference("echo", "mike", ""),
			`reference "b" names no state, and place "mike" names its states`},
		{"a state where its place names none", "fit", "b", reference("delta", "kilo", "lima"),
			`reference "b" is in state "lima", and place "kilo" names none`},
		{"an entry its place holds once and holds already, the new id sorting first", "fit", "b", reference("delta", "kilo", ""),
			`reference "b" names entry "delta", which reference "k" holds in place "kilo" already, and that place holds an entry once`},
		{"an entry its place holds once and holds already, the new id sorting last", "fit", "z", reference("delta", "kilo", ""),
			`reference "z" names entry "delta", which reference "k" holds in place "kilo" already, and that place holds an entry once`},
		{"an actor the campaign does not hold, with a reference that does not fit", "ghost", "b", reference("victor", "mike", "lima"),
			`reference "b" names entry "victor", which the catalogue does not hold`},
	}
	rs := referenceRuleset(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, camp, dm := referenceSheet(t, rs, fitSheet(nil))
			issueRefused(t, s, camp, dm, sheetAddReference(c.actor, c.id, c.r),
				`gateway: add_reference: actor "`+c.actor+`": references: `+c.want)
		})
	}
}

// Keep fit's sheet holding a reference that does not fit and two copies in a
// place that holds an entry once: an addition is read alone against the others.
// VTT-1046
func TestAnAddReferenceIsReadAgainstTheSheetsOthersAndNotForTheirFaults(t *testing.T) {
	sheet := map[string]*vttv1.Reference{"k": reference("delta", "kilo", ""), "k2": reference("delta", "kilo", ""),
		"x": reference("victor", "whiskey", "papa")}
	s, camp, dm := referenceSheet(t, referenceRuleset(t), sheet)
	want := maps.Clone(sheet)
	want["b"] = reference("echo", "mike", "lima")
	issueOne(t, s, camp, dm, sheetAddReference("fit", "b", reference("echo", "mike", "lima")), want)
}

// Keep reference a's addition as delta in kilo, which k holds: an id the sheet
// holds is the fold's to refuse, before the copies rule.
// VTT-1046
func TestAnAddReferenceTheFoldRefusesReachesTheIssuerAndAppendsNothing(t *testing.T) {
	for name, rs := range referenceRulesets(t) {
		t.Run(name, func(t *testing.T) {
			cases := []struct {
				cmd  *vttv1.ClientCommand
				want string
			}{
				{sheetAddReference("ghost", "b", reference("echo", "mike", "lima")), `engine: reference_added names unknown actor "ghost"`},
				{sheetAddReference("fit", "a", reference("echo", "mike", "lima")), `engine: actor "fit" already holds reference "a"`},
				{sheetAddReference("fit", "a", reference("delta", "kilo", "")), `engine: actor "fit" already holds reference "a"`},
			}
			if rs == nil {
				cases = append(cases, struct {
					cmd  *vttv1.ClientCommand
					want string
				}{sheetAddReference("fit", "b", nil), "engine: reference_added requires a reference"})
			}
			s, camp, dm := referenceSheet(t, rs, fitSheet(nil))
			for _, c := range cases {
				issueRefused(t, s, camp, dm, c.cmd, c.want)
			}
		})
	}
}

// VTT-1046
func TestAnAddReferenceWithAnEmptyIdIsRefusedWithOrWithoutARuleset(t *testing.T) {
	want := `gateway: add_reference: actor "fit": reference_id: must not be empty`
	for name, rs := range referenceRulesets(t) {
		t.Run(name, func(t *testing.T) {
			s, camp, dm := referenceSheet(t, rs, fitSheet(nil))
			issueRefused(t, s, camp, dm, sheetAddReference("fit", "", reference("echo", "mike", "lima")), want)
			issueRefused(t, s, camp, dm, sheetAddReference("fit", "", reference("victor", "whiskey", "")), want)
		})
	}
}

// VTT-1061
func TestWithNoRulesetAnAddOrAChangeIsRefusedForNoEntryPlaceStateOrCopy(t *testing.T) {
	s, camp, dm := referenceSheet(t, nil, fitSheet(nil))
	want := fitSheet(nil)
	for _, add := range []struct {
		id string
		r  *vttv1.Reference
	}{
		{"b", reference("victor", "whiskey", "papa")},
		{"c", reference("delta", "kilo", "")},
		{"d", reference("echo", "kilo", "lima")},
	} {
		want[add.id] = add.r
		if issueOne(t, s, camp, dm, sheetAddReference("fit", add.id, add.r), maps.Clone(want)).GetReferenceAdded() == nil {
			t.Fatalf("the addition of %q appended no ReferenceAdded", add.id)
		}
	}
	for _, change := range []struct{ id, place, state, entry string }{
		{"a", "whiskey", "", "echo"},
		{"k", "kilo", "lima", "delta"},
		{"a", "kilo", "", "echo"},
	} {
		want[change.id] = reference(change.entry, change.place, change.state)
		if issueOne(t, s, camp, dm, sheetChangeReference("fit", change.id, change.place, change.state),
			maps.Clone(want)).GetReferenceChanged() == nil {
			t.Fatalf("the change of %q appended no ReferenceChanged", change.id)
		}
	}
}

// VTT-1047
func TestAChangeReferenceAppendsOneReferenceChangedKeepingItsEntry(t *testing.T) {
	for name, rs := range referenceRulesets(t) {
		for _, who := range []string{"dm", "agent"} {
			t.Run(name+"/"+who, func(t *testing.T) {
				s, camp, dm := referenceSheet(t, rs, fitSheet(nil))
				env := issueOne(t, s, camp, issuerFor(who, dm), sheetChangeReference("fit", "a", "mike", "oscar"),
					fitSheet(map[string]*vttv1.Reference{"a": reference("echo", "mike", "oscar")}))
				changed := env.GetReferenceChanged()
				if changed.GetActorId() != "fit" || changed.GetReferenceId() != "a" || changed.GetPlace() != "mike" ||
					changed.GetState() != "oscar" {
					t.Fatalf("the change appended %v, want a ReferenceChanged of a to mike, oscar on fit", env)
				}
			})
		}
	}
}

// Keep each move's state one only its new place accepts: a change read against
// the place it leaves is refused.
// VTT-1047
func TestAChangeIsReadAgainstThePlaceItMovesTo(t *testing.T) {
	s, camp, dm := referenceSheet(t, referenceRuleset(t), fitSheet(nil))
	issueOne(t, s, camp, dm, sheetChangeReference("fit", "k", "mike", "oscar"),
		fitSheet(map[string]*vttv1.Reference{"k": reference("delta", "mike", "oscar")}))
	issueOne(t, s, camp, dm, sheetChangeReference("fit", "k", "kilo", ""), fitSheet(nil))
}

// Keep b and z either side of k: the refusal names the reference changed and
// the one it meets, whichever sorts first.
// VTT-1047
func TestWithARulesetAChangeThatDoesNotFitIsRefusedAndAppendsNothing(t *testing.T) {
	cases := []struct {
		name, id, place, state, want string
	}{
		{"a place the ruleset does not declare", "a", "whiskey", "",
			`reference "a" is in place "whiskey", which the ruleset does not declare`},
		{"a place that does not hold its entry's form", "a", "kilo", "",
			`reference "a" names entry "echo" of form "zulu", which place "kilo" does not hold`},
		{"a state its place does not name", "a", "mike", "papa",
			`reference "a" is in state "papa", which place "mike" does not name`},
		{"no state where its place names some", "a", "mike", "",
			`reference "a" names no state, and place "mike" names its states`},
		{"a state where its place names none", "k", "kilo", "lima",
			`reference "k" is in state "lima", and place "kilo" names none`},
		{"a place holding its entry once that holds it already, the changed id sorting first", "b", "kilo", "",
			`reference "b" names entry "delta", which reference "k" holds in place "kilo" already, and that place holds an entry once`},
		{"a place holding its entry once that holds it already, the changed id sorting last", "z", "kilo", "",
			`reference "z" names entry "delta", which reference "k" holds in place "kilo" already, and that place holds an entry once`},
	}
	rs := referenceRuleset(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, camp, dm := referenceSheet(t, rs, fitSheet(map[string]*vttv1.Reference{
				"b": reference("delta", "mike", "lima"), "z": reference("delta", "mike", "oscar")}))
			issueRefused(t, s, camp, dm, sheetChangeReference("fit", c.id, c.place, c.state),
				`gateway: change_reference: actor "fit": references: `+c.want)
		})
	}
}

// VTT-1047
func TestAChangeKeptInAPlaceHoldingAnEntryOnceIsNotReadAgainstItself(t *testing.T) {
	s, camp, dm := referenceSheet(t, referenceRuleset(t), fitSheet(nil))
	if issueOne(t, s, camp, dm, sheetChangeReference("fit", "k", "kilo", ""), fitSheet(nil)).GetReferenceChanged() == nil {
		t.Fatal("the change appended no ReferenceChanged")
	}
}

// Keep fit's sheet holding a reference that does not fit and two copies in a
// place that holds an entry once: a change is read alone against the others.
// VTT-1047
func TestAChangeReferenceIsReadAgainstTheSheetsOthersAndNotForTheirFaults(t *testing.T) {
	more := map[string]*vttv1.Reference{"k2": reference("delta", "kilo", ""), "x": reference("victor", "whiskey", "papa")}
	s, camp, dm := referenceSheet(t, referenceRuleset(t), fitSheet(more))
	want := fitSheet(more)
	want["a"] = reference("echo", "mike", "oscar")
	issueOne(t, s, camp, dm, sheetChangeReference("fit", "a", "mike", "oscar"), want)
}

// Keep nope's change to whiskey one that fits no place: an id the sheet does
// not hold is the fold's to refuse, whatever the change names.
// VTT-1047
func TestAChangeReferenceTheFoldRefusesReachesTheIssuerAndAppendsNothing(t *testing.T) {
	for name, rs := range referenceRulesets(t) {
		t.Run(name, func(t *testing.T) {
			s, camp, dm := referenceSheet(t, rs, fitSheet(nil))
			for _, c := range []struct {
				cmd  *vttv1.ClientCommand
				want string
			}{
				{sheetChangeReference("ghost", "a", "mike", "oscar"), `engine: reference_changed names unknown actor "ghost"`},
				{sheetChangeReference("fit", "nope", "mike", "oscar"), `engine: actor "fit" does not hold reference "nope"`},
				{sheetChangeReference("fit", "nope", "whiskey", ""), `engine: actor "fit" does not hold reference "nope"`},
			} {
				issueRefused(t, s, camp, dm, c.cmd, c.want)
			}
		})
	}
}

// Keep x a reference that fits no ruleset: a removal is read against none.
// VTT-1048
func TestARemoveReferenceAppendsOneReferenceRemovedAndChangesOnlyTheSheet(t *testing.T) {
	for name, rs := range referenceRulesets(t) {
		for _, who := range []string{"dm", "agent"} {
			t.Run(name+"/"+who, func(t *testing.T) {
				s, camp, dm := referenceSheet(t, rs, fitSheet(map[string]*vttv1.Reference{"x": reference("victor", "whiskey", "papa")}))
				env := issueOne(t, s, camp, issuerFor(who, dm), sheetRemoveReference("fit", "x"), fitSheet(nil))
				if removed := env.GetReferenceRemoved(); removed.GetActorId() != "fit" || removed.GetReferenceId() != "x" {
					t.Fatalf("the removal appended %v, want a ReferenceRemoved of x from fit", env)
				}
				issueOne(t, s, camp, issuerFor(who, dm), sheetRemoveReference("fit", "k"),
					map[string]*vttv1.Reference{"a": reference("echo", "mike", "lima")})
			})
		}
	}
}

// VTT-1048
func TestARemoveReferenceTheFoldRefusesReachesTheIssuerAndAppendsNothing(t *testing.T) {
	for name, rs := range referenceRulesets(t) {
		t.Run(name, func(t *testing.T) {
			s, camp, dm := referenceSheet(t, rs, fitSheet(nil))
			for _, c := range []struct {
				cmd  *vttv1.ClientCommand
				want string
			}{
				{sheetRemoveReference("ghost", "a"), `engine: reference_removed names unknown actor "ghost"`},
				{sheetRemoveReference("fit", "nope"), `engine: actor "fit" does not hold reference "nope"`},
				{sheetRemoveReference("fit", ""), `engine: actor "fit" does not hold reference ""`},
			} {
				issueRefused(t, s, camp, dm, c.cmd, c.want)
			}
		})
	}
}

// VTT-1050
func TestNoReferenceCommandAppendsARulesetNamed(t *testing.T) {
	s, camp, dm := referenceSheet(t, referenceRuleset(t), fitSheet(nil))
	if s.rulesetNamed(camp.State()) == nil {
		t.Fatal("the log names the served ruleset already, so this shows nothing")
	}
	b := reference("echo", "mike", "oscar")
	for _, c := range []struct {
		cmd  *vttv1.ClientCommand
		want map[string]*vttv1.Reference
		is   func(*vttv1.Envelope) bool
	}{
		{sheetAddReference("fit", "b", b), fitSheet(map[string]*vttv1.Reference{"b": b}),
			func(e *vttv1.Envelope) bool { return e.GetReferenceAdded() != nil }},
		{sheetChangeReference("fit", "b", "mike", "lima"), fitSheet(map[string]*vttv1.Reference{"b": reference("echo", "mike", "lima")}),
			func(e *vttv1.Envelope) bool { return e.GetReferenceChanged() != nil }},
		{sheetRemoveReference("fit", "b"), fitSheet(nil),
			func(e *vttv1.Envelope) bool { return e.GetReferenceRemoved() != nil }},
	} {
		if env := issueOne(t, s, camp, dm, c.cmd, c.want); !c.is(env) {
			t.Errorf("%v appended %v, want its own event alone", c.cmd.GetCommand(), env)
		}
	}
}

// Keep fit granted to p-1: the role table must refuse a player the very actor
// they control.
// VTT-1049
func TestAPlayerOrASpectatorIsRefusedEveryReferenceCommandAndAppendsNothing(t *testing.T) {
	s, camp, _ := referenceSheet(t, referenceRuleset(t), fitSheet(nil))
	seatAppend(t, camp, "fit-granted", &vttv1.ActorControlGranted{ActorId: "fit", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	for _, p := range []*identity.Participant{{ID: "p-1", Role: identity.RolePlayer}, {ID: "s-1", Role: identity.RoleSpectator}} {
		for name, cmd := range map[string]*vttv1.ClientCommand{
			"add_reference":    sheetAddReference("fit", "b", reference("echo", "mike", "oscar")),
			"change_reference": sheetChangeReference("fit", "a", "mike", "oscar"),
			"remove_reference": sheetRemoveReference("fit", "a"),
		} {
			issueRefused(t, s, camp, p, cmd, `gateway: not authorized: role "`+string(p.Role)+`" may not issue "`+name+`"`)
		}
	}
}
