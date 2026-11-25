package adventure_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaarRulesetJSON = `{
  "id": "qa-ar-rules",
  "name": "QA AR Rules",
  "format_version": "3",
  "version": "1",
  "properties": [
    {"key": "tango", "type": "number", "name": "Tango", "description": "A number."},
    {"key": "sierra", "type": "resource", "name": "Sierra",
     "description": "A pool.", "default_max_expr": "5", "thresholds": []}
  ],
  "forms": [
    {"key": "zulu", "name": "Zulu", "description": "A form.", "properties": []},
    {"key": "yankee", "name": "Yankee", "description": "A form.", "properties": []}
  ],
  "places": [
    {"key": "mike", "name": "Mike", "description": "Holds copies.",
     "forms": ["zulu"], "copies": true, "states": ["lima", "oscar"]},
    {"key": "kilo", "name": "Kilo", "description": "Holds once.",
     "forms": ["zulu", "yankee"]}
  ]
}`

var qaarRulesetFiles = map[string]string{
	"ruleset.json": qaarRulesetJSON,
	"guide.md":     "# QA AR Rules\n",
	"catalogue/echo.json": `{"id": "echo", "name": "Echo",
  "description": "An entry.", "form": "zulu"}`,
	"catalogue/delta.json": `{"id": "delta", "name": "Delta",
  "description": "An entry.", "form": "yankee"}`,
	"atoms/whiskey.json": `{"id": "whiskey", "params": [],
  "provides": ["delivery"], "consumes": [],
  "contributes": [{"kind": "targeting", "range": 0, "max_targets": 1}]}`,
	"abilities/victor.json": `{"id": "victor", "name": "Victor",
  "usage": {"limited": {"resource": "sierra", "cost": 1}},
  "compose": [{"atom": "whiskey", "bind": {}}]}`,
}

func qaarWrite(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func qaarRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "qa-ar-rules")
	qaarWrite(t, dir, qaarRulesetFiles)
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("the QA ruleset does not load: %v", err)
	}
	return rs
}

func qaarActor(id, extra string) string {
	return `{"actor_id": "` + id + `", "name": "Actor ` + id +
		`", "kind": "non_party"` + extra + `}`
}

func qaarAdventureDir(t *testing.T, actors map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"adventure.json": `{"id": "qa-ar", "name": "QA AR",
  "format_version": "1", "ruleset": "qa-ar-rules",
  "opening_narration": "It begins."}`,
		"guide.md": "# QA AR\n",
	}
	for name, body := range actors {
		files[filepath.Join("actors", name)] = body
	}
	qaarWrite(t, dir, files)
	return dir
}

func qaarLoad(t *testing.T, actors map[string]string) (*adventure.Adventure, error) {
	t.Helper()
	return adventure.Load(qaarAdventureDir(t, actors), qaarRuleset(t))
}

func qaarActorAdded(t *testing.T, adv *adventure.Adventure, id string) *vttv1.Actor {
	t.Helper()
	envs, _, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	for _, env := range envs {
		if a := env.GetActorAdded().GetActor(); a != nil && a.GetActorId() == id {
			return a
		}
	}
	t.Fatalf("Compile gave no ActorAdded for %q", id)
	return nil
}

func qaarLoadedActor(t *testing.T, adv *adventure.Adventure, id string) adventure.AdventureActor {
	t.Helper()
	for _, a := range adv.Actors {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("Load holds no actor %q", id)
	return adventure.AdventureActor{}
}

func qaarMustRefuse(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load accepted; want a refusal holding %q", wants)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("refusal %q does not hold %q", err, w)
		}
	}
}

// VTT-1042.
func TestQAARReferencesReachTheActorAddedAsWritten(t *testing.T) {
	want := map[string]adventure.Reference{
		"zz-first-in-file": {EntryID: "echo", Place: "mike", State: "oscar"},
		"Mixed Case Id":    {EntryID: "echo", Place: "mike", State: "lima"},
		"aa-copy":          {EntryID: "echo", Place: "mike", State: "lima"},
		"echo-in-kilo":     {EntryID: "echo", Place: "kilo"},
		"delta-in-kilo":    {EntryID: "delta", Place: "kilo"},
	}
	refs := `, "references": {
  "zz-first-in-file": {"entry_id": "echo", "place": "mike", "state": "oscar"},
  "Mixed Case Id": {"entry_id": "echo", "place": "mike", "state": "lima"},
  "aa-copy": {"entry_id": "echo", "place": "mike", "state": "lima"},
  "echo-in-kilo": {"entry_id": "echo", "place": "kilo"},
  "delta-in-kilo": {"entry_id": "delta", "place": "kilo", "state": ""}}`
	adv, err := qaarLoad(t, map[string]string{
		"a-holder.json": qaarActor("holder", refs),
		"b-bare.json":   qaarActor("bare", ""),
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := qaarLoadedActor(t, adv, "holder").References
	if len(got) != len(want) {
		t.Fatalf("Load holds %d references, want %d: %v", len(got), len(want), got)
	}
	for id, w := range want {
		if g, ok := got[id]; !ok || g != w {
			t.Errorf("Load reference %q = %+v (held %v), want %+v", id, g, ok, w)
		}
	}
	compiled := qaarActorAdded(t, adv, "holder").GetReferences()
	if len(compiled) != len(want) {
		t.Fatalf("ActorAdded holds %d references, want %d", len(compiled), len(want))
	}
	for id, w := range want {
		wp := &vttv1.Reference{EntryId: w.EntryID, Place: w.Place, State: w.State}
		if g, ok := compiled[id]; !ok || !proto.Equal(g, wp) {
			t.Errorf("ActorAdded reference %q = %v (held %v), want %v", id, g, ok, wp)
		}
	}
	if n := len(qaarActorAdded(t, adv, "bare").GetReferences()); n != 0 {
		t.Errorf("an actor writing no references compiled %d", n)
	}
}

// VTT-1042.
func TestQAARReferencesAbsentNullOrEmptyHoldNone(t *testing.T) {
	for name, extra := range map[string]string{
		"absent": "",
		"null":   `, "references": null`,
		"empty":  `, "references": {}`,
	} {
		t.Run(name, func(t *testing.T) {
			adv, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x", extra)})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := qaarLoadedActor(t, adv, "x").References
			if len(got) != 0 {
				t.Errorf("Load holds %v, want none", got)
			}
			t.Logf("Load's References is nil: %v", got == nil)
			if n := len(qaarActorAdded(t, adv, "x").GetReferences()); n != 0 {
				t.Errorf("ActorAdded holds %d references, want none", n)
			}
		})
	}
}

// VTT-1042.
func TestQAARAReferenceWrittenNullIsObserved(t *testing.T) {
	_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
		`, "references": {"null-ref": null}`)})
	t.Logf("a reference written null: err = %v", err)
	if err != nil {
		qaarMustRefuse(t, err, filepath.Join("actors", "x.json"), "references")
	}
}

// VTT-1043.
func TestQAARAReferenceWrittenNullIsReadAfterTheSpendRule(t *testing.T) {
	_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
		`, "ability_ids": ["victor"], "references": {"n": null}`)})
	t.Logf("null reference beside a spend fault: %v", err)
	qaarMustRefuse(t, err, `"ability_ids"`)
	for name, refs := range map[string]string{
		"misfit before it in byte order": `{"a": {"entry_id": "foxtrot", "place": "kilo"}, "b": null}`,
		"misfit after it in byte order":  `{"a": null, "b": {"entry_id": "foxtrot", "place": "kilo"}}`,
		"beside an empty id":             `{"": {"entry_id": "echo", "place": "kilo"}, "b": null}`,
	} {
		_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
			`, "references": `+refs)})
		t.Logf("null reference, %s: %v", name, err)
		qaarMustRefuse(t, err, `field "references"`)
	}
}

// VTT-1042, VTT-1043.
func TestQAARAFieldWrittenNullIsObserved(t *testing.T) {
	cases := map[string]string{
		"state":    `{"entry_id": "echo", "place": "kilo", "state": null}`,
		"entry_id": `{"entry_id": null, "place": "kilo"}`,
		"place":    `{"entry_id": "echo", "place": null}`,
	}
	for field, ref := range cases {
		t.Run(field, func(t *testing.T) {
			adv, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
				`, "references": {"nulled": `+ref+`}`)})
			t.Logf("%s written null: err = %v", field, err)
			if err != nil {
				qaarMustRefuse(t, err, filepath.Join("actors", "x.json"),
					"references", `"nulled"`)
				return
			}
			got := qaarLoadedActor(t, adv, "x").References["nulled"]
			t.Logf("%s written null loads as %+v", field, got)
			if field != "state" {
				t.Errorf("%s written null loaded although no reference "+
					"with that field empty fits", field)
			}
		})
	}
}

// VTT-1043.
func TestQAARAnEmptyReferenceIdIsRefused(t *testing.T) {
	for name, ref := range map[string]string{
		"fitting":    `{"entry_id": "echo", "place": "kilo"}`,
		"misfitting": `{"entry_id": "foxtrot", "place": "kilo"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := qaarLoad(t, map[string]string{
				"a-ok.json":    qaarActor("ok", ""),
				"b-empty.json": qaarActor("empty", `, "references": {"": `+ref+`}`),
			})
			t.Logf("refusal: %v", err)
			qaarMustRefuse(t, err, filepath.Join("actors", "b-empty.json"),
				"references", "empty id")
			if err != nil && strings.Contains(err.Error(), "a-ok.json") {
				t.Errorf("refusal names the other actor's file: %v", err)
			}
		})
	}
}

// VTT-1043.
func TestQAARAnEmptyIdIsRefusedBeforeAnotherReferenceMisfits(t *testing.T) {
	_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
		`, "references": {"!bad": {"entry_id": "foxtrot", "place": "kilo"},
  "": {"entry_id": "echo", "place": "kilo"}}`)})
	t.Logf("refusal: %v", err)
	qaarMustRefuse(t, err, "references", "empty id")
	if err != nil && strings.Contains(err.Error(), `"!bad"`) {
		t.Errorf("refusal names the misfit, not the empty id: %v", err)
	}
}

// VTT-1043, VTT-1037.
func TestQAAREachMisfitIsRefusedNamingFileFieldAndReference(t *testing.T) {
	cases := map[string]string{
		"entry the catalogue does not hold":   `{"entry_id": "foxtrot", "place": "kilo"}`,
		"place the ruleset does not declare":  `{"entry_id": "echo", "place": "november"}`,
		"form the place does not hold":        `{"entry_id": "delta", "place": "mike", "state": "lima"}`,
		"state the place does not name":       `{"entry_id": "echo", "place": "mike", "state": "papa"}`,
		"no state where the place names some": `{"entry_id": "echo", "place": "mike"}`,
		"a state where the place names none":  `{"entry_id": "echo", "place": "kilo", "state": "lima"}`,
	}
	for name, ref := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := qaarLoad(t, map[string]string{
				"a-ok.json": qaarActor("ok", `, "references": {
  "fine": {"entry_id": "echo", "place": "kilo"}}`),
				"b-bad.json": qaarActor("bad", `, "references": {
  "fine": {"entry_id": "delta", "place": "kilo"}, "qa-misfit": `+ref+`}`),
			})
			t.Logf("refusal: %v", err)
			qaarMustRefuse(t, err, filepath.Join("actors", "b-bad.json"),
				`"references"`, `"qa-misfit"`)
			if err != nil && strings.Contains(err.Error(), "a-ok.json") {
				t.Errorf("refusal names the fitting actor's file: %v", err)
			}
		})
	}
}

// VTT-1043, VTT-1037.
func TestQAARTheFirstMisfitInByteOrderOfIdsIsNamed(t *testing.T) {
	refs := map[string]string{
		"c-place": `{"entry_id": "echo", "place": "november"}`,
		"b-entry": `{"entry_id": "foxtrot", "place": "kilo"}`,
		"a-fits":  `{"entry_id": "echo", "place": "kilo"}`,
		"B-state": `{"entry_id": "echo", "place": "kilo", "state": "lima"}`,
	}
	order := []string{"c-place", "b-entry", "a-fits", "B-state"}
	for _, step := range []struct {
		drop []string
		want string
	}{
		{nil, "B-state"},
		{[]string{"B-state"}, "b-entry"},
		{[]string{"B-state", "b-entry"}, "c-place"},
	} {
		var parts []string
		for _, id := range order {
			if !slices.Contains(step.drop, id) {
				parts = append(parts, `"`+id+`": `+refs[id])
			}
		}
		_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
			`, "references": {`+strings.Join(parts, ", ")+`}`)})
		t.Logf("dropping %q: %v", step.drop, err)
		qaarMustRefuse(t, err, `"references"`, `"`+step.want+`"`)
		for _, other := range order {
			if other != step.want && err != nil &&
				strings.Contains(err.Error(), `"`+other+`"`) {
				t.Errorf("want %q named alone, refusal names %q: %v",
					step.want, other, err)
			}
		}
	}
}

// VTT-1072.
func TestQAARTheCopiesRuleNamesTheLaterReference(t *testing.T) {
	_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
		`, "references": {
  "k3": {"entry_id": "echo", "place": "kilo"},
  "k2": {"entry_id": "echo", "place": "kilo"},
  "k1": {"entry_id": "echo", "place": "kilo"}}`)})
	t.Logf("three to echo in kilo: %v", err)
	qaarMustRefuse(t, err, filepath.Join("actors", "x.json"), `"references"`, `"k2"`)
	if err != nil && strings.Contains(err.Error(), `"k3"`) {
		t.Errorf("refusal names k3, the last; want k2, the second: %v", err)
	}

	_, err = qaarLoad(t, map[string]string{"x.json": qaarActor("x",
		`, "references": {
  "k9": {"entry_id": "echo", "place": "kilo"},
  "k1": {"entry_id": "delta", "place": "kilo"},
  "k10": {"entry_id": "echo", "place": "kilo"}}`)})
	t.Logf("k10 and k9 to echo in kilo: %v", err)
	qaarMustRefuse(t, err, `"references"`, `"k9"`)

	adv, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
		`, "references": {
  "m1": {"entry_id": "echo", "place": "mike", "state": "lima"},
  "m2": {"entry_id": "echo", "place": "mike", "state": "lima"},
  "m3": {"entry_id": "echo", "place": "kilo"},
  "m4": {"entry_id": "delta", "place": "kilo"}}`)})
	if err != nil {
		t.Fatalf("copies in mike and echo in two places refused: %v", err)
	}
	if n := len(qaarActorAdded(t, adv, "x").GetReferences()); n != 4 {
		t.Errorf("ActorAdded holds %d references, want 4", n)
	}
}

// VTT-1072.
func TestQAARFitFaultsComeBeforeTheCopiesRule(t *testing.T) {
	_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x",
		`, "references": {
  "a": {"entry_id": "echo", "place": "kilo"},
  "b": {"entry_id": "echo", "place": "kilo"},
  "z": {"entry_id": "foxtrot", "place": "kilo"}}`)})
	t.Logf("copy at b, misfit at z: %v", err)
	qaarMustRefuse(t, err, `"references"`, `"z"`)
	if err != nil && strings.Contains(err.Error(), `"b"`) {
		t.Errorf("the copies rule was read before a fit fault: %v", err)
	}
}

// VTT-1043, VTT-1072, SPEC-025.
func TestQAAREarlierRefusalsComeBeforeReferences(t *testing.T) {
	refs := map[string]string{
		"empty id": `, "references": {"": {"entry_id": "echo", "place": "kilo"}}`,
		"misfit":   `, "references": {"r": {"entry_id": "foxtrot", "place": "kilo"}}`,
		"copy": `, "references": {"r": {"entry_id": "echo", "place": "kilo"},
  "s": {"entry_id": "echo", "place": "kilo"}}`,
	}
	type earlier struct {
		head, sheet, want string
	}
	full := `, "kind": "non_party"`
	ok := `{"actor_id": "x", "name": "N"` + full
	cases := map[string]earlier{
		"actor id": {`{"actor_id": "", "name": "N"` + full, "",
			`field "actor_id"`},
		"name": {`{"actor_id": "x", "name": ""` + full, "",
			`field "name"`},
		"kind": {`{"actor_id": "x", "name": "N"`, "",
			`field "kind"`},
		"key": {ok, `, "attributes": {"nope": 1}`,
			`field "attributes"`},
		"value": {ok, `, "attributes": {"tango": null}`,
			`field "attributes.tango"`},
		"empty ability id": {ok, `, "ability_ids": [""]`,
			`field "ability_ids": must not hold an empty id`},
		"unheld ability id": {ok, `, "ability_ids": ["nope"]`,
			`field "ability_ids": names ability "nope"`},
		"spend, ability": {ok, `, "ability_ids": ["victor"]`,
			`spends resource "sierra"`},
		"spend, resource": {ok, `, "resources": {"sierra": {"current": 1}}`,
			`field "resources"`},
	}
	for cname, c := range cases {
		for rname, r := range refs {
			t.Run(cname+" before "+rname, func(t *testing.T) {
				_, err := qaarLoad(t, map[string]string{"x.json": c.head + c.sheet + r + `}`})
				t.Logf("refusal: %v", err)
				qaarMustRefuse(t, err, filepath.Join("actors", "x.json"), c.want)
				if err != nil && strings.Contains(err.Error(), `field "references"`) {
					t.Errorf("references read before the %s refusal: %v", cname, err)
				}
			})
		}
	}
}

// The control for TestQAAREarlierRefusalsComeBeforeReferences.
func TestQAARTheOrderingFixturesMisfitOnceMended(t *testing.T) {
	for rname, r := range map[string]string{
		"empty id": `, "references": {"": {"entry_id": "echo", "place": "kilo"}}`,
		"misfit":   `, "references": {"r": {"entry_id": "foxtrot", "place": "kilo"}}`,
		"copy": `, "references": {"r": {"entry_id": "echo", "place": "kilo"},
  "s": {"entry_id": "echo", "place": "kilo"}}`,
	} {
		t.Run(rname, func(t *testing.T) {
			sheet := `, "attributes": {"tango": 1}, "ability_ids": ["victor"],
  "resources": {"sierra": {"current": 1, "max": 5}}`
			_, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x", sheet+r)})
			qaarMustRefuse(t, err, filepath.Join("actors", "x.json"), "references")
			if _, err := qaarLoad(t, map[string]string{"x.json": qaarActor("x", sheet)}); err != nil {
				t.Fatalf("the mended sheet without references is refused: %v", err)
			}
		})
	}
}
