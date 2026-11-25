package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// referenceRuleset loads internal/rules/testdata/valid, whose guard-stance
// spends pool_a, with forms zulu and yankee, places mike and kilo in place of
// its own, entries echo of zulu and delta of yankee, and no sheet.
func referenceRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "rules", "testdata", "valid"))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	m["forms"] = append(m["forms"].([]any),
		map[string]any{"key": "zulu", "name": "Zulu", "description": "An invented form."},
		map[string]any{"key": "yankee", "name": "Yankee", "description": "Another invented form."})
	m["places"] = []any{
		map[string]any{"key": "mike", "name": "Mike", "description": "Holds either form, more than once.",
			"forms": []any{"zulu", "yankee"}, "copies": true, "states": []any{"lima", "oscar"}},
		map[string]any{"key": "kilo", "name": "Kilo", "description": "Holds the second form once.",
			"forms": []any{"yankee"}},
	}
	delete(m, "sheet")
	writeReferenceJSON(t, path, m)
	writeReferenceJSON(t, filepath.Join(dir, "catalogue", "echo.json"),
		map[string]any{"id": "echo", "name": "Echo", "description": "An entry of zulu.", "form": "zulu"})
	writeReferenceJSON(t, filepath.Join(dir, "catalogue", "delta.json"),
		map[string]any{"id": "delta", "name": "Delta", "description": "An entry of yankee.", "form": "yankee"})
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func writeReferenceJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func referenceAddActor(refs map[string]*vttv1.Reference, abilityIDs ...string) *vttv1.ClientCommand {
	cmd := fitAddActor("fit", nil, nil)
	cmd.GetAddActor().GetActor().References = refs
	cmd.GetAddActor().GetActor().AbilityIds = abilityIDs
	return cmd
}

func reference(entry, place, state string) *vttv1.Reference {
	return &vttv1.Reference{EntryId: entry, Place: place, State: state}
}

func refusedWith(t *testing.T, rs *rules.Ruleset, cmd *vttv1.ClientCommand, want string) {
	t.Helper()
	s, camp, dm := fitServer(t, rs)
	r := s.handleCommand(dm, cmd, nil)
	if r.GetOk() || r.GetError() != want {
		t.Errorf("add_actor answered %+v, want %q", r, want)
	}
	if _, added := camp.State().Actors["fit"]; added {
		t.Error("the refused actor was appended")
	}
}

func appendedHolding(t *testing.T, rs *rules.Ruleset, refs map[string]*vttv1.Reference) {
	t.Helper()
	s, camp, dm := fitServer(t, rs)
	r := s.handleCommand(dm, referenceAddActor(refs), nil)
	if !r.GetOk() {
		t.Fatalf("add_actor refused: %s", r.GetError())
	}
	got := camp.State().Actors["fit"].GetReferences()
	if len(got) != len(refs) {
		t.Fatalf("the appended actor holds %v, want %v", got, refs)
	}
	for id, want := range refs {
		if !proto.Equal(got[id], want) {
			t.Errorf("the appended actor holds %q as %v, want %v", id, got[id], want)
		}
	}
}

// VTT-1039, VTT-1070
func TestAddActorWithARulesetRefusesAReferenceThatDoesNotFit(t *testing.T) {
	cases := []struct {
		name string
		refs map[string]*vttv1.Reference
		want string
	}{
		{"an entry the catalogue does not hold", map[string]*vttv1.Reference{"r": reference("victor", "mike", "lima")},
			`reference "r" names entry "victor", which the catalogue does not hold`},
		{"a place the ruleset does not declare", map[string]*vttv1.Reference{"r": reference("echo", "whiskey", "")},
			`reference "r" is in place "whiskey", which the ruleset does not declare`},
		{"an entry of a form its place does not hold", map[string]*vttv1.Reference{"r": reference("echo", "kilo", "")},
			`reference "r" names entry "echo" of form "zulu", which place "kilo" does not hold`},
		{"a state its place does not name", map[string]*vttv1.Reference{"r": reference("echo", "mike", "papa")},
			`reference "r" is in state "papa", which place "mike" does not name`},
		{"no state where its place names some", map[string]*vttv1.Reference{"r": reference("echo", "mike", "")},
			`reference "r" names no state, and place "mike" names its states`},
		{"a state where its place names none", map[string]*vttv1.Reference{"r": reference("delta", "kilo", "lima")},
			`reference "r" is in state "lima", and place "kilo" names none`},
		{"a second reference to one entry in a place holding it once", map[string]*vttv1.Reference{
			"r": reference("delta", "kilo", ""), "q": reference("delta", "kilo", "")},
			`reference "r" names entry "delta", which reference "q" holds in place "kilo" already, and that place holds an entry once`},
	}
	rs := referenceRuleset(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			refusedWith(t, rs, referenceAddActor(c.refs), `gateway: add_actor: actor "fit": references: `+c.want)
		})
	}
}

// VTT-1039
func TestAddActorWithARulesetAppendsASheetWhoseReferencesFit(t *testing.T) {
	appendedHolding(t, referenceRuleset(t), map[string]*vttv1.Reference{
		"a": reference("echo", "mike", "lima"), "b": reference("echo", "mike", "lima"),
		"c": reference("delta", "mike", "oscar"), "d": reference("delta", "kilo", "")})
}

// VTT-1039 VTT-1040 VTT-775
func TestAddActorReadsAnActorsReferencesAfterTheSpendRule(t *testing.T) {
	spend := `gateway: add_actor: actor "fit": ability_ids: names ability "guard-stance", which spends resource "pool_a" the sheet does not hold`
	rs := referenceRuleset(t)
	t.Run("a reference that does not fit", func(t *testing.T) {
		refusedWith(t, rs, referenceAddActor(map[string]*vttv1.Reference{"r": reference("victor", "mike", "lima")}, "guard-stance"), spend)
	})
	t.Run("an empty reference id", func(t *testing.T) {
		refusedWith(t, rs, referenceAddActor(map[string]*vttv1.Reference{"": reference("echo", "mike", "lima")}, "guard-stance"), spend)
	})
}

// VTT-1040
func TestAddActorRefusesAnEmptyReferenceIdWithOrWithoutARuleset(t *testing.T) {
	want := `gateway: add_actor: actor "fit": references: must not hold an empty id`
	for name, rs := range map[string]*rules.Ruleset{"no ruleset": nil, "a ruleset": referenceRuleset(t)} {
		t.Run(name, func(t *testing.T) {
			refusedWith(t, rs, referenceAddActor(map[string]*vttv1.Reference{
				"": reference("echo", "mike", "lima"), "r": reference("echo", "mike", "lima")}), want)
		})
	}
}

// VTT-1040
func TestAddActorRefusesAnEmptyReferenceIdBeforeReadingAReferenceAgainstTheRuleset(t *testing.T) {
	refusedWith(t, referenceRuleset(t), referenceAddActor(map[string]*vttv1.Reference{
		"": reference("echo", "mike", "lima"), "a": reference("victor", "kilo", "")}),
		`gateway: add_actor: actor "fit": references: must not hold an empty id`)
}

// VTT-1040 VTT-747
func TestAddActorWithNoRulesetRefusesAnEmptyAbilityIdBeforeAnEmptyReferenceId(t *testing.T) {
	refusedWith(t, nil, referenceAddActor(map[string]*vttv1.Reference{"": reference("echo", "mike", "")}, ""),
		`gateway: add_actor: actor "fit": ability_ids: must not hold an empty id`)
}

// VTT-1041
func TestAddActorWithNoRulesetReadsNoReferenceAgainstOne(t *testing.T) {
	appendedHolding(t, nil, map[string]*vttv1.Reference{
		"a": reference("victor", "whiskey", "papa"), "b": reference("echo", "kilo", ""),
		"c": reference("delta", "kilo", "lima"), "d": reference("delta", "kilo", ""), "e": reference("echo", "mike", "")})
}
