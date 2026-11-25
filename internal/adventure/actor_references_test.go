package adventure_test

import (
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// referenceRuleset is the fixture ruleset with forms zulu and yankee, places
// mike and kilo, and entries echo of zulu and delta of yankee.
func referenceRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := copyFixtureDirExcluding(t, "testdata/ruleset")
	editJSONFile(t, filepath.Join(dir, "ruleset.json"), func(m map[string]any) {
		m["forms"] = []any{
			map[string]any{"key": "zulu", "name": "Zulu", "description": "An invented form."},
			map[string]any{"key": "yankee", "name": "Yankee", "description": "Another invented form."},
		}
		m["places"] = []any{
			map[string]any{"key": "mike", "name": "Mike", "description": "Holds either form, more than once.",
				"forms": []any{"zulu", "yankee"}, "copies": true, "states": []any{"lima", "oscar"}},
			map[string]any{"key": "kilo", "name": "Kilo", "description": "Holds the second form once.",
				"forms": []any{"yankee"}},
		}
	})
	catalogue := filepath.Join(dir, "catalogue")
	if err := os.MkdirAll(catalogue, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct{ id, form string }{{"echo", "zulu"}, {"delta", "yankee"}} {
		body := `{"id": "` + e.id + `", "name": "` + e.id + `", "description": "An invented entry.", "form": "` + e.form + `"}`
		if err := os.WriteFile(filepath.Join(catalogue, e.id+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

// braceGuardReferences is testdata/valid with its brace guard holding refs as
// written, and the brace guard's file.
func braceGuardReferences(t *testing.T, refs string) (dir, path string) {
	t.Helper()
	dir = writeBraceGuardSheet(t, braceGuardFits, `, "references": `+refs)
	return dir, filepath.Join(dir, "actors", "brace-guard.json")
}

// compiledReferences answers the references of each actor Compile adds.
func compiledReferences(t *testing.T, adv *adventure.Adventure) map[string]map[string]*vttv1.Reference {
	t.Helper()
	envs, _, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]*vttv1.Reference{}
	for _, e := range envs {
		if a := e.GetActorAdded().GetActor(); a != nil {
			out[a.GetActorId()] = a.GetReferences()
		}
	}
	return out
}

func sameReferences(got, want map[string]*vttv1.Reference) bool {
	return proto.Equal(&vttv1.Actor{References: got}, &vttv1.Actor{References: want})
}

const fitsInMike = `{"entry_id": "echo", "place": "mike", "state": "lima"}`

// VTT-1042, VTT-1072
func TestAnActorFilesReferencesReachItsActorAddedAsWritten(t *testing.T) {
	dir, _ := braceGuardReferences(t, `{
		"r-two": {"entry_id": "delta", "place": "kilo"},
		"r-one": {"entry_id": "echo", "place": "mike", "state": "lima"},
		"r-three": {"entry_id": "echo", "place": "mike", "state": "oscar"},
		"r-four": {"entry_id": "delta", "place": "mike", "state": "oscar"}}`)
	adv, err := adventure.Load(dir, referenceRuleset(t))
	if err != nil {
		t.Fatalf("an actor holding references that fit was refused: %v", err)
	}
	got := compiledReferences(t, adv)
	want := map[string]*vttv1.Reference{
		"r-one":   {EntryId: "echo", Place: "mike", State: "lima"},
		"r-two":   {EntryId: "delta", Place: "kilo"},
		"r-three": {EntryId: "echo", Place: "mike", State: "oscar"},
		"r-four":  {EntryId: "delta", Place: "mike", State: "oscar"},
	}
	if !sameReferences(got["brace-guard"], want) {
		t.Errorf("brace-guard's ActorAdded holds %v, want %v", got["brace-guard"], want)
	}
	if len(got["grit-scout"]) != 0 {
		t.Errorf("grit-scout's file holds no reference, and its ActorAdded holds %v", got["grit-scout"])
	}
}

// VTT-1042
func TestAnActorFileWhoseReferencesAreNullOrEmptyHoldsNone(t *testing.T) {
	for _, refs := range []string{`null`, `{}`} {
		t.Run(refs, func(t *testing.T) {
			dir, _ := braceGuardReferences(t, refs)
			adv, err := adventure.Load(dir, referenceRuleset(t))
			if err != nil {
				t.Fatalf("an actor whose references are written %s was refused: %v", refs, err)
			}
			if got := compiledReferences(t, adv)["brace-guard"]; len(got) != 0 {
				t.Errorf("brace-guard's ActorAdded holds %v, want none", got)
			}
		})
	}
}

// VTT-1042, VTT-1043
func TestAReferenceFieldWrittenNullReadsAsOneLeftOut(t *testing.T) {
	dir, _ := braceGuardReferences(t, `{"r": {"entry_id": "delta", "place": "kilo", "state": null}}`)
	adv, err := adventure.Load(dir, referenceRuleset(t))
	if err != nil {
		t.Fatalf("a reference whose state is written null, in a place naming none, was refused: %v", err)
	}
	want := map[string]*vttv1.Reference{"r": {EntryId: "delta", Place: "kilo"}}
	if got := compiledReferences(t, adv)["brace-guard"]; !sameReferences(got, want) {
		t.Errorf("brace-guard's ActorAdded holds %v, want %v", got, want)
	}
	for _, c := range []struct{ ref, reason string }{
		{`{"entry_id": null, "place": "kilo"}`, `names entry "", which the catalogue does not hold`},
		{`{"entry_id": "delta", "place": null}`, `is in place "", which the ruleset does not declare`},
	} {
		t.Run(c.ref, func(t *testing.T) {
			dir, path := braceGuardReferences(t, `{"r": `+c.ref+`}`)
			_, err := adventure.Load(dir, referenceRuleset(t))
			want := `adventure: ` + path + `: field "references": reference "r" ` + c.reason
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-1042, VTT-1073
func TestAnActorFileHoldingAReferenceWrittenNullIsRefused(t *testing.T) {
	for _, c := range []struct{ name, refs, id string }{
		{"one written null", `{"r": null}`, "r"},
		{"the first in byte order of ids", `{"r-b": null, "r-a": null, "R-c": null}`, "R-c"},
		{"before the ruleset reads them", `{"r-a": {"entry_id": "victor", "place": "mike", "state": "lima"}, "r-b": null}`, "r-b"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, path := braceGuardReferences(t, c.refs)
			_, err := adventure.Load(dir, referenceRuleset(t))
			want := `adventure: ` + path + `: field "references": reference "` + c.id +
				`" must be an object giving its entry_id and place, not null`
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-1043, VTT-1073
func TestAnActorFileHoldingAnEmptyReferenceIdIsRefused(t *testing.T) {
	for _, c := range []struct{ name, refs string }{
		{"an empty id alone", `{"": ` + fitsInMike + `}`},
		{"an empty id beside one that fits", `{"r": ` + fitsInMike + `, "": ` + fitsInMike + `}`},
		{"before the ruleset reads them", `{"": ` + fitsInMike + `, "r": {"entry_id": "victor", "place": "mike", "state": "lima"}}`},
		{"an empty id written null", `{"": null}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, path := braceGuardReferences(t, c.refs)
			_, err := adventure.Load(dir, referenceRuleset(t))
			want := `adventure: ` + path + `: field "references": must not hold an empty id`
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-1043
func TestAnActorFileHoldingAReferenceThatDoesNotFitIsRefused(t *testing.T) {
	for _, c := range []struct{ name, ref, reason string }{
		{"an entry the catalogue does not hold", `{"entry_id": "victor", "place": "mike", "state": "lima"}`,
			`names entry "victor", which the catalogue does not hold`},
		{"a place the ruleset does not declare", `{"entry_id": "echo", "place": "whiskey", "state": "lima"}`,
			`is in place "whiskey", which the ruleset does not declare`},
		{"an entry of a form its place does not hold", `{"entry_id": "echo", "place": "kilo"}`,
			`names entry "echo" of form "zulu", which place "kilo" does not hold`},
		{"no state, in a place naming its states", `{"entry_id": "echo", "place": "mike"}`,
			`names no state, and place "mike" names its states`},
		{"a state, in a place naming none", `{"entry_id": "delta", "place": "kilo", "state": "lima"}`,
			`is in state "lima", and place "kilo" names none`},
		{"a state its place does not name", `{"entry_id": "echo", "place": "mike", "state": "papa"}`,
			`is in state "papa", which place "mike" does not name`},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, path := braceGuardReferences(t, `{"r-fits": `+fitsInMike+`, "r-misfit": `+c.ref+`}`)
			_, err := adventure.Load(dir, referenceRuleset(t))
			want := `adventure: ` + path + `: field "references": reference "r-misfit" ` + c.reason
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-1043
func TestAnActorFilesFirstMisfitInByteOrderOfIdsIsNamed(t *testing.T) {
	dir, path := braceGuardReferences(t, `{
		"b": {"entry_id": "victor", "place": "mike", "state": "lima"},
		"B": {"entry_id": "echo", "place": "whiskey", "state": "lima"}}`)
	_, err := adventure.Load(dir, referenceRuleset(t))
	want := `adventure: ` + path + `: field "references": reference "B" is in place "whiskey", which the ruleset does not declare`
	if err == nil || err.Error() != want {
		t.Fatalf("Load = %v, want %q", err, want)
	}
}

// VTT-1072
func TestAnActorFileHoldingASecondReferenceToAnEntryItsPlaceHoldsOnceIsRefused(t *testing.T) {
	dir, path := braceGuardReferences(t, `{
		"r-b": {"entry_id": "delta", "place": "kilo"},
		"r-a": {"entry_id": "delta", "place": "kilo"}}`)
	_, err := adventure.Load(dir, referenceRuleset(t))
	want := `adventure: ` + path + `: field "references": reference "r-b" names entry "delta", ` +
		`which reference "r-a" holds in place "kilo" already, and that place holds an entry once`
	if err == nil || err.Error() != want {
		t.Fatalf("Load = %v, want %q", err, want)
	}
}

// VTT-1072
func TestAnActorFilesReferencesAreReadForFitBeforeTheCopiesRule(t *testing.T) {
	dir, path := braceGuardReferences(t, `{
		"r-a": {"entry_id": "delta", "place": "kilo"},
		"r-b": {"entry_id": "delta", "place": "kilo"},
		"r-c": {"entry_id": "victor", "place": "mike", "state": "lima"}}`)
	_, err := adventure.Load(dir, referenceRuleset(t))
	want := `adventure: ` + path + `: field "references": reference "r-c" names entry "victor", which the catalogue does not hold`
	if err == nil || err.Error() != want {
		t.Fatalf("Load = %v, want %q", err, want)
	}
}

// VTT-1043, VTT-1072, VTT-1073
func TestAnActorFilesSpendRuleIsReadBeforeItsReferences(t *testing.T) {
	for _, c := range []struct{ name, refs string }{
		{"an empty id", `{"": ` + fitsInMike + `}`},
		{"one written null", `{"r": null}`},
		{"one that does not fit", `{"r": {"entry_id": "victor", "place": "mike", "state": "lima"}}`},
		{"a second in a place holding an entry once",
			`{"r-a": {"entry_id": "delta", "place": "kilo"}, "r-b": {"entry_id": "delta", "place": "kilo"}}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs := referenceRuleset(t)
			rs.Compiled["swing"] = &rules.CompiledPower{Usage: rules.Usage{Limited: &rules.LimitedUsage{Resource: "focus", Cost: 1}}}
			dir, path := braceGuardReferences(t, c.refs)
			_, err := adventure.Load(dir, rs)
			want := `adventure: ` + path + `: field "resources": holds resource "focus", which no ability the sheet names spends`
			if err == nil || err.Error() != want {
				t.Fatalf("Load = %v, want %q", err, want)
			}
		})
	}
}

// VTT-1043
func TestAnActorFileHoldingReferencesThatFitLoadsWithTheSpendRuleMet(t *testing.T) {
	rs := referenceRuleset(t)
	rs.Compiled["swing"] = &rules.CompiledPower{Usage: rules.Usage{Limited: &rules.LimitedUsage{Resource: "focus", Cost: 1}}}
	dir := writeBraceGuardSheet(t, braceGuardFits, `, "ability_ids": ["swing"], "references": {"r": `+fitsInMike+`}`)
	for _, name := range []string{"grit-scout.json", "vim-fighter.json"} {
		editJSONFile(t, filepath.Join(dir, "actors", name), func(m map[string]any) {
			m["ability_ids"] = []any{"swing"}
		})
	}
	if _, err := adventure.Load(dir, rs); err != nil {
		t.Fatalf("actors meeting the spend rule, one holding a reference that fits, were refused: %v", err)
	}
}
