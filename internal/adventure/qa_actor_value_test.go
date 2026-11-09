package adventure_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep a declared key on each side, in byte order, of the key each
// single-fault test faults, the case-only tests aside.
func qaAVARuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	manifest := `{"id":"qa-avrs","name":"QA Values","format_version":"3","properties":[
	{"key":"qa_alpha","type":"number","name":"Alpha","description":"A number."},
	{"key":"qa_num","type":"number","name":"Num","description":"A number."},
	{"key":"qa_zeta","type":"number","name":"Zeta","description":"A number."},
	{"key":"qa_pool","type":"resource","name":"Pool","description":"A resource."},
	{"key":"qa_spare","type":"resource","name":"Spare","description":"A resource."},
	{"key":"qa_well","type":"resource","name":"Well","description":"A resource."}]}`
	qaAVAWrite(t, filepath.Join(dir, "ruleset.json"), manifest)
	qaAVAWrite(t, filepath.Join(dir, "guide.md"), "# QA Values\n")
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaAVAWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	qaAVAGoodAttrs = `{"qa_alpha": 3, "qa_num": 4, "qa_zeta": 5}`
	qaAVAGoodRes   = `{"qa_pool": {"current": 1, "max": 2}, "qa_spare": {"current": 3, "max": 4}, "qa_well": {"current": 5, "max": 6}}`
)

func qaAVAActor(id, attrs, res string) string {
	return fmt.Sprintf(`{"actor_id":%q,"name":%q,"kind":"non_party","attributes":%s,"resources":%s}`,
		id, "Actor "+id, attrs, res)
}

// Keep the sheet under test in qa-m, the middle file, never the first or last.
func qaAVAAdventure(t *testing.T, attrs, res string) (dir, faultPath string) {
	t.Helper()
	dir = t.TempDir()
	qaAVAWrite(t, filepath.Join(dir, "adventure.json"),
		`{"id":"qa-adv","name":"QA Adventure","format_version":"1","ruleset":"qa-avrs","opening_narration":"A quiet room."}`)
	qaAVAWrite(t, filepath.Join(dir, "actors", "qa-a.json"), qaAVAActor("qa-a", qaAVAGoodAttrs, qaAVAGoodRes))
	faultPath = filepath.Join(dir, "actors", "qa-m.json")
	qaAVAWrite(t, faultPath, qaAVAActor("qa-m", attrs, res))
	qaAVAWrite(t, filepath.Join(dir, "actors", "qa-z.json"), qaAVAActor("qa-z", qaAVAGoodAttrs, qaAVAGoodRes))
	return dir, faultPath
}

func qaAVALoad(t *testing.T, attrs, res string) (*adventure.Adventure, string, error) {
	t.Helper()
	rs := qaAVARuleset(t)
	dir, faultPath := qaAVAAdventure(t, attrs, res)
	a, err := adventure.Load(dir, rs)
	return a, faultPath, err
}

func qaAVAMiddleActor(t *testing.T, a *adventure.Adventure) adventure.AdventureActor {
	t.Helper()
	for _, act := range a.Actors {
		if act.ID == "qa-m" {
			return act
		}
	}
	t.Fatalf("actor qa-m not loaded; actors: %+v", a.Actors)
	return adventure.AdventureActor{}
}

func qaAVARefused(t *testing.T, err error, faultPath, field, key string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load accepted the sheet; want a refusal naming %s, %q and %q", faultPath, field, key)
	}
	msg := err.Error()
	if !strings.Contains(msg, faultPath) {
		t.Errorf("error %q does not name the file %q", msg, faultPath)
	}
	rest := qaAVARest(err, faultPath)
	for _, want := range []string{field, key} {
		if !strings.Contains(rest, want) {
			t.Errorf("error %q does not name %q outside the file's path", msg, want)
		}
	}
	for _, other := range []string{"qa-a.json", "qa-z.json"} {
		if strings.Contains(msg, other) {
			t.Errorf("error %q names %s, which holds no fault", msg, other)
		}
	}
}

// Match words only outside the path: a temporary path carries the test's name.
func qaAVARest(err error, faultPath string) string {
	if err == nil {
		return ""
	}
	return strings.Replace(err.Error(), faultPath, "", 1)
}

func qaAVAControlLoads(t *testing.T, attrs, res string) {
	t.Helper()
	if _, _, err := qaAVALoad(t, attrs, res); err != nil {
		t.Fatalf("control adventure (fault repaired) refused: %v", err)
	}
}

// VTT-670
func TestQAAVANumberWrittenNullIsRefused(t *testing.T) {
	qaAVAControlLoads(t, `{"qa_zeta": 5, "qa_num": 4, "qa_alpha": 3}`, qaAVAGoodRes)
	_, path, err := qaAVALoad(t, `{"qa_zeta": 5, "qa_num": null, "qa_alpha": 3}`, qaAVAGoodRes)
	qaAVARefused(t, err, path, "attributes", "qa_num")
}

// VTT-670
func TestQAAVAResourceWrittenNullIsRefused(t *testing.T) {
	res := `{"qa_well": {"current": 5, "max": 6}, "qa_spare": %s, "qa_pool": {"current": 1, "max": 2}}`
	qaAVAControlLoads(t, qaAVAGoodAttrs, fmt.Sprintf(res, `{"current": 3, "max": 4}`))
	_, path, err := qaAVALoad(t, qaAVAGoodAttrs, fmt.Sprintf(res, `null`))
	qaAVARefused(t, err, path, "resources", "qa_spare")
}

// VTT-670
func TestQAAVAResourceCurrentWrittenNullIsRefused(t *testing.T) {
	res := `{"qa_well": {"current": 5, "max": 6}, "qa_spare": %s, "qa_pool": {"current": 1, "max": 2}}`
	qaAVAControlLoads(t, qaAVAGoodAttrs, fmt.Sprintf(res, `{"current": 3, "max": 4}`))
	_, path, err := qaAVALoad(t, qaAVAGoodAttrs, fmt.Sprintf(res, `{"current": null, "max": 4}`))
	qaAVARefused(t, err, path, "resources", "qa_spare")
}

// VTT-670
func TestQAAVAResourceCurrentLeftOutIsRefused(t *testing.T) {
	res := `{"qa_well": {"current": 5, "max": 6}, "qa_spare": %s, "qa_pool": {"current": 1, "max": 2}}`
	qaAVAControlLoads(t, qaAVAGoodAttrs, fmt.Sprintf(res, `{"current": 3, "max": 4}`))
	_, path, err := qaAVALoad(t, qaAVAGoodAttrs, fmt.Sprintf(res, `{"max": 4}`))
	qaAVARefused(t, err, path, "resources", "qa_spare")
}

// VTT-670
func TestQAAVAEmptyResourceObjectIsRefused(t *testing.T) {
	res := `{"qa_well": {"current": 5, "max": 6}, "qa_spare": %s, "qa_pool": {"current": 1, "max": 2}}`
	qaAVAControlLoads(t, qaAVAGoodAttrs, fmt.Sprintf(res, `{"current": 3}`))
	_, path, err := qaAVALoad(t, qaAVAGoodAttrs, fmt.Sprintf(res, `{}`))
	qaAVARefused(t, err, path, "resources", "qa_spare")
}

// VTT-670
func TestQAAVANullRefusalHasTheAdventureFieldForm(t *testing.T) {
	cases := []struct {
		name, attrs, res string
	}{
		{"number null", `{"qa_num": null}`, `{}`},
		{"resource null", `{}`, `{"qa_spare": null}`},
		{"current null", `{}`, `{"qa_spare": {"current": null, "max": 4}}`},
		{"current left out", `{}`, `{"qa_spare": {"max": 4}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, path, err := qaAVALoad(t, c.attrs, c.res)
			if err == nil {
				t.Fatal("Load accepted the sheet")
			}
			if prefix := "adventure: " + path + `: field "`; !strings.HasPrefix(err.Error(), prefix) {
				t.Errorf("error %q does not begin %q", err, prefix)
			}
		})
	}
}

// VTT-670
func TestQAAVAZeroNumberLoadsAsAValue(t *testing.T) {
	a, _, err := qaAVALoad(t, `{"qa_alpha": 3, "qa_num": 0, "qa_zeta": 5}`, qaAVAGoodRes)
	if err != nil {
		t.Fatalf("a number of 0 was refused: %v", err)
	}
	got, ok := qaAVAMiddleActor(t, a).Attributes["qa_num"]
	if !ok || got != 0 {
		t.Errorf("Attributes[qa_num] = %d, present %v; want 0, present", got, ok)
	}
}

// VTT-670
func TestQAAVAResourceSpentToNothingLoads(t *testing.T) {
	a, _, err := qaAVALoad(t, qaAVAGoodAttrs,
		`{"qa_pool": {"current": 1, "max": 2}, "qa_spare": {"current": 0, "max": 4}, "qa_well": {"current": 5, "max": 6}}`)
	if err != nil {
		t.Fatalf("a resource spent to 0 was refused: %v", err)
	}
	got, ok := qaAVAMiddleActor(t, a).Resources["qa_spare"]
	if !ok || got != (adventure.ResourceVal{Current: 0, Max: 4}) {
		t.Errorf("Resources[qa_spare] = %+v, present %v; want {Current:0 Max:4}, present", got, ok)
	}
}

// VTT-670
func TestQAAVAResourceWithoutMaximumLoads(t *testing.T) {
	cases := []struct {
		name, spare string
		current     int32
	}{
		{"max left out", `{"current": 900}`, 900},
		{"max written null", `{"current": 900, "max": null}`, 900},
		{"spent to nothing, max left out", `{"current": 0}`, 0},
		{"spent to nothing, max written null", `{"max": null, "current": 0}`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, _, err := qaAVALoad(t, qaAVAGoodAttrs,
				`{"qa_pool": {"current": 1, "max": 2}, "qa_spare": `+c.spare+`, "qa_well": {"current": 5, "max": 6}}`)
			if err != nil {
				t.Fatalf("a resource with no maximum was refused: %v", err)
			}
			got, ok := qaAVAMiddleActor(t, a).Resources["qa_spare"]
			if !ok || got.Current != c.current {
				t.Errorf("Resources[qa_spare] = %+v, present %v; want Current %d, present", got, ok, c.current)
			}
			// See TestLoadAcceptsValuesExactlyOnEveryLimit: max 0 is none.
			if got.Max != 0 {
				t.Errorf("Resources[qa_spare].Max = %d; want 0, the no-maximum value", got.Max)
			}
		})
	}
}

// VTT-617
func TestQAAVAUndeclaredAttributeKeyIsRefused(t *testing.T) {
	_, path, err := qaAVALoad(t, `{"qa_alpha": 3, "qa_nope": 4, "qa_zeta": 5}`, qaAVAGoodRes)
	qaAVARefused(t, err, path, "attributes", "qa_nope")
}

// VTT-617
func TestQAAVAUndeclaredResourceKeyIsRefused(t *testing.T) {
	_, path, err := qaAVALoad(t, qaAVAGoodAttrs,
		`{"qa_pool": {"current": 1, "max": 2}, "qa_nope": {"current": 3, "max": 4}, "qa_well": {"current": 5, "max": 6}}`)
	qaAVARefused(t, err, path, "resources", "qa_nope")
}

// VTT-617
func TestQAAVAKeyDifferingOnlyInCaseIsUndeclared(t *testing.T) {
	_, path, err := qaAVALoad(t, `{"qa_alpha": 3, "QA_NUM": 4, "qa_zeta": 5}`, qaAVAGoodRes)
	qaAVARefused(t, err, path, "attributes", "QA_NUM")
}

// VTT-618
func TestQAAVAAttributeKeyDeclaredAsResourceIsRefused(t *testing.T) {
	_, path, err := qaAVALoad(t, `{"qa_alpha": 3, "qa_spare": 4, "qa_zeta": 5}`, qaAVAGoodRes)
	qaAVARefused(t, err, path, "attributes", "qa_spare")
	if err != nil && !strings.Contains(qaAVARest(err, path), rules.PropertyResource) {
		t.Errorf("error %q does not name the key's type %q", err, rules.PropertyResource)
	}
}

// VTT-618
func TestQAAVAResourceKeyDeclaredAsNumberIsRefused(t *testing.T) {
	_, path, err := qaAVALoad(t, qaAVAGoodAttrs,
		`{"qa_pool": {"current": 1, "max": 2}, "qa_num": {"current": 3, "max": 4}, "qa_well": {"current": 5, "max": 6}}`)
	qaAVARefused(t, err, path, "resources", "qa_num")
	if err != nil && !strings.Contains(qaAVARest(err, path), rules.PropertyNumber) {
		t.Errorf("error %q does not name the key's type %q", err, rules.PropertyNumber)
	}
}

// VTT-617, VTT-618
func TestQAAVAAttributesFaultIsNamedBeforeAResourcesFault(t *testing.T) {
	cases := []struct {
		name, attrs, res, key, notKey string
	}{
		{
			"undeclared attribute sorting after an undeclared resource",
			`{"zz_attr_nope": 1}`, `{"aa_res_nope": {"current": 1, "max": 2}}`,
			"zz_attr_nope", "aa_res_nope",
		},
		{
			"other-type attribute sorting after an other-type resource",
			`{"qa_well": 1}`, `{"qa_alpha": {"current": 1, "max": 2}}`,
			"qa_well", "qa_alpha",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, path, err := qaAVALoad(t, c.attrs, c.res)
			qaAVARefused(t, err, path, "attributes", c.key)
			if strings.Contains(qaAVARest(err, path), c.notKey) {
				t.Errorf("error %q names %q, a resources key; the attributes fault comes first", err, c.notKey)
			}
		})
	}
}

// VTT-617, VTT-618
func TestQAAVAFirstFaultInByteOrderOfKeysIsNamed(t *testing.T) {
	cases := []struct {
		name, attrs, res, field, key string
		notKeys                      []string
	}{
		{
			"attributes: uppercase sorts before lowercase, not in written order",
			`{"qa_b_nope": 1, "qa_alpha": 2, "Qa_c_nope": 1}`, `{}`,
			"attributes", "Qa_c_nope", []string{"qa_b_nope"},
		},
		{
			"attributes: other-type key sorting before an undeclared one",
			`{"qb_nope": 1, "qa_pool": 2}`, `{}`,
			"attributes", "qa_pool", []string{"qb_nope"},
		},
		{
			"attributes: undeclared key sorting before an other-type one",
			`{"qa_pool": 2, "qa_aa_nope": 1}`, `{}`,
			"attributes", "qa_aa_nope", []string{"qa_pool"},
		},
		{
			"resources: other-type key sorting before an undeclared one",
			qaAVAGoodAttrs, `{"qb_nope": {"current": 1, "max": 2}, "qa_pool": {"current": 1, "max": 2}, "qa_num": {"current": 1, "max": 2}}`,
			"resources", "qa_num", []string{"qb_nope"},
		},
		{
			"resources: undeclared key sorting before an other-type one",
			qaAVAGoodAttrs, `{"qa_zeta": {"current": 1, "max": 2}, "qa_pool": {"current": 1, "max": 2}, "qa_aa_nope": {"current": 1, "max": 2}}`,
			"resources", "qa_aa_nope", []string{"qa_zeta"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, path, err := qaAVALoad(t, c.attrs, c.res)
			qaAVARefused(t, err, path, c.field, c.key)
			for _, nk := range c.notKeys {
				if strings.Contains(qaAVARest(err, path), nk) {
					t.Errorf("error %q names %q; %q comes first in byte order", err, nk, c.key)
				}
			}
		})
	}
}

// VTT-617, VTT-618
func TestQAAVASheetHoldingNoValueForADeclaredPropertyLoads(t *testing.T) {
	cases := []struct {
		name, actor string
	}{
		{"both maps empty", qaAVAActor("qa-m", `{}`, `{}`)},
		{"some properties given", qaAVAActor("qa-m", `{"qa_num": 4}`, `{"qa_well": {"current": 5, "max": 6}}`)},
		{"both maps left out", `{"actor_id":"qa-m","name":"Actor qa-m","kind":"non_party"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs := qaAVARuleset(t)
			dir, faultPath := qaAVAAdventure(t, qaAVAGoodAttrs, qaAVAGoodRes)
			qaAVAWrite(t, faultPath, c.actor)
			a, err := adventure.Load(dir, rs)
			if err != nil {
				t.Fatalf("a sheet missing declared properties was refused: %v", err)
			}
			act := qaAVAMiddleActor(t, a)
			if _, ok := act.Attributes["qa_alpha"]; ok {
				t.Errorf("Attributes holds qa_alpha, which the sheet never gave: %+v", act.Attributes)
			}
			if _, ok := act.Resources["qa_pool"]; ok {
				t.Errorf("Resources holds qa_pool, which the sheet never gave: %+v", act.Resources)
			}
		})
	}
}
