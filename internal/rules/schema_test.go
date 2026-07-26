package rules_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// readSchema loads and generically decodes one embedded schema document.
func readSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := fs.ReadFile(rules.Schemas, "schema/"+name)
	if err != nil {
		t.Fatalf("reading embedded schema %q: %v", name, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("schema %q is not valid JSON: %v", name, err)
	}
	return doc
}

// propertyKeys returns the top-level "properties" object's keys.
func propertyKeys(t *testing.T, schema map[string]any) map[string]bool {
	t.Helper()
	raw, ok := schema["properties"]
	if !ok {
		t.Fatal(`schema has no "properties" object`)
	}
	props, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf(`schema "properties" is not an object: %#v`, raw)
	}
	out := make(map[string]bool, len(props))
	for k := range props {
		out[k] = true
	}
	return out
}

// copyDir recursively copies src into dst (both must exist/be creatable) —
// used to build a scratch copy of testdata/valid that a subtest can mutate
// without disturbing the fixture other tests rely on.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("copyDir: read %s: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("copyDir: mkdir %s: %v", dst, err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyDir(t, s, d)
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			t.Fatalf("copyDir: read %s: %v", s, err)
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			t.Fatalf("copyDir: write %s: %v", d, err)
		}
	}
}

// jsonKeys returns the top-level keys of the JSON object at path.
func jsonKeys(t *testing.T, path string) map[string]bool {
	t.Helper()
	obj := readJSONFile(t, path)
	out := make(map[string]bool, len(obj))
	for k := range obj {
		out[k] = true
	}
	return out
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readJSONFile: read %s: %v", path, err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("readJSONFile: unmarshal %s: %v", path, err)
	}
	return obj
}

func writeJSONFile(t *testing.T, path string, obj map[string]any) {
	t.Helper()
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("writeJSONFile: marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("writeJSONFile: write %s: %v", path, err)
	}
}

func setJSONField(t *testing.T, path, key string, value any) {
	t.Helper()
	obj := readJSONFile(t, path)
	obj[key] = value
	writeJSONFile(t, path, obj)
}

// TestSchemaPropertiesCoverFixtureFields cross-checks each schema
// document's declared "properties" against the actual keys the valid
// fixture uses at that level (ruleset.json top level, one ability file top
// level plus its nested targeting/attack objects, one condition file) — a
// renamed or newly-added field on one side without the other would fail
// here, closing the "hand validation and schema documents can't drift
// apart" requirement from the field-name direction.
func TestSchemaPropertiesCoverFixtureFields(t *testing.T) {
	cases := []struct {
		schemaFile  string
		fixtureFile string
	}{
		{"ruleset.schema.json", "testdata/valid/ruleset.json"},
		{"ability.schema.json", "testdata/valid/abilities/strike.json"},
		{"condition.schema.json", "testdata/valid/conditions/guarded.json"},
	}
	for _, tc := range cases {
		t.Run(tc.schemaFile, func(t *testing.T) {
			schema := readSchema(t, tc.schemaFile)
			props := propertyKeys(t, schema)
			for key := range jsonKeys(t, tc.fixtureFile) {
				if !props[key] {
					t.Errorf("fixture %s uses field %q, but %s's properties do not declare it", tc.fixtureFile, key, tc.schemaFile)
				}
			}
		})
	}
}

// TestSchemaPropertiesCoverV2FixtureFields is
// TestSchemaPropertiesCoverFixtureFields's format-v2 counterpart: atom.
// schema.json (new this task) and ability.schema.json's v2 composition
// shape, cross-checked against testdata/valid-v2 fixtures. Kept as its
// own test rather than added to TestSchemaPropertiesCoverFixtureFields's
// `cases` table because that table already has a "ability.schema.json"
// entry (the v1 fixture) — a second entry with the same schemaFile would
// still run correctly (Go disambiguates same-named subtests), but a
// distinct function name is clearer about which fixture generation is
// under test.
func TestSchemaPropertiesCoverV2FixtureFields(t *testing.T) {
	cases := []struct {
		schemaFile  string
		fixtureFile string
	}{
		{"atom.schema.json", "testdata/valid-v2/atoms/clash-roll.json"},
		{"ability.schema.json", "testdata/valid-v2/abilities/quick-jab.json"},
	}
	for _, tc := range cases {
		t.Run(tc.schemaFile+"/"+filepath.Base(tc.fixtureFile), func(t *testing.T) {
			schema := readSchema(t, tc.schemaFile)
			props := propertyKeys(t, schema)
			for key := range jsonKeys(t, tc.fixtureFile) {
				if !props[key] {
					t.Errorf("fixture %s uses field %q, but %s's properties do not declare it", tc.fixtureFile, key, tc.schemaFile)
				}
			}
		})
	}
}

// --- generalized nested-"required" walker ---
//
// The point of this machinery (review fix wave, task-4-brief follow-up):
// a schema document's "required" array can appear at ANY nesting depth —
// not just the top level — reached through plain "properties" nesting,
// array "items", and "$ref"-to-"$defs" indirection (oneOf branches
// included). TestSchemaRequiredFieldsMatchLoaderEnforcement below walks
// every one of those and, for each nested required field a schema
// declares, deletes it from wherever the valid fixture set demonstrates
// that exact structure and asserts Load rejects it. This is the
// structural fix for the class of bug where a schema document is correct
// but the hand-written loader quietly doesn't enforce what it claims
// (e.g. resources[].thresholds[].remove_when_false, usage.limited.cost,
// targeting.range) — those three are no longer special-cased tests, they
// are just three of the paths this walker discovers on its own.

// pathStep is one navigation step from a JSON document's root: either
// "descend into object key K" or "take array element [0]" (arrays in this
// walker are always tested via their first element — every fixture array
// this schema format uses has at least one populated entry to probe).
type pathStep struct {
	key    string
	index0 bool
}

func pathString(path []pathStep) string {
	var b strings.Builder
	for _, s := range path {
		if s.index0 {
			b.WriteString("[0]")
			continue
		}
		if b.Len() > 0 {
			b.WriteString(".")
		}
		b.WriteString(s.key)
	}
	return b.String()
}

func extendPath(path []pathStep, step pathStep) []pathStep {
	out := make([]pathStep, len(path), len(path)+1)
	copy(out, path)
	return append(out, step)
}

// navigatePath walks path from root (as decoded by encoding/json into
// map[string]any / []any) and returns the map found there. Used both to
// locate a schema node's corresponding fixture location, and (by
// deleteNestedJSONKey) to find the exact map to delete a key from.
func navigatePath(root any, path []pathStep) (map[string]any, bool) {
	cur := root
	for _, step := range path {
		if step.index0 {
			arr, ok := cur.([]any)
			if !ok || len(arr) == 0 {
				return nil, false
			}
			cur = arr[0]
			continue
		}
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := obj[step.key]
		if !ok {
			return nil, false
		}
		cur = next
	}
	obj, ok := cur.(map[string]any)
	return obj, ok
}

// resolveRef resolves a "$ref": "#/$defs/NAME" pointer against root — the
// only $ref shape this repo's schemas use.
func resolveRef(t *testing.T, root map[string]any, ref string) map[string]any {
	t.Helper()
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		t.Fatalf("unsupported $ref %q (only #/$defs/NAME is handled by this test)", ref)
	}
	name := strings.TrimPrefix(ref, prefix)
	defs, ok := root["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("schema has $ref %q but no $defs object", ref)
	}
	def, ok := defs[name].(map[string]any)
	if !ok {
		t.Fatalf("schema $defs has no entry %q for $ref %q", name, ref)
	}
	return def
}

type reqCase struct {
	path  []pathStep
	field string
}

// collectRequired recursively walks a schema node, collecting every
// "required" array found anywhere under it (via "properties", "items",
// "oneOf", and "$ref"/"$defs" resolution), each paired with the path
// (relative to root) at which that requirement applies. depth guards
// against a self-referential schema causing infinite recursion (none of
// this repo's schemas are self-referential, but the guard costs nothing).
func collectRequired(t *testing.T, root, node map[string]any, path []pathStep, depth int) []reqCase {
	t.Helper()
	if depth > 20 {
		t.Fatalf("collectRequired: exceeded max depth at path %q — possible $ref cycle", pathString(path))
	}

	if ref, ok := node["$ref"].(string); ok {
		return collectRequired(t, root, resolveRef(t, root, ref), path, depth+1)
	}

	var out []reqCase
	if reqRaw, ok := node["required"].([]any); ok {
		for _, f := range reqRaw {
			field, ok := f.(string)
			if !ok {
				t.Fatalf("schema %q required entry is not a string: %#v", pathString(path), f)
			}
			out = append(out, reqCase{path: path, field: field})
		}
	}
	if props, ok := node["properties"].(map[string]any); ok {
		for name, subRaw := range props {
			sub, ok := subRaw.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, collectRequired(t, root, sub, extendPath(path, pathStep{key: name}), depth+1)...)
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		out = append(out, collectRequired(t, root, items, extendPath(path, pathStep{index0: true}), depth+1)...)
	}
	if oneOf, ok := node["oneOf"].([]any); ok {
		for _, branchRaw := range oneOf {
			branch, ok := branchRaw.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, collectRequired(t, root, branch, path, depth+1)...)
		}
	}
	return out
}

func dedupeReqCases(cases []reqCase) []reqCase {
	seen := map[string]bool{}
	var out []reqCase
	for _, c := range cases {
		key := pathString(c.path) + "\x00" + c.field
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

func deleteNestedJSONKey(t *testing.T, path string, steps []pathStep, field string) {
	t.Helper()
	root := readJSONFile(t, path)
	container, ok := navigatePath(any(root), steps)
	if !ok {
		t.Fatalf("deleteNestedJSONKey: path %q not found in %s", pathString(steps), path)
	}
	if _, has := container[field]; !has {
		t.Fatalf("deleteNestedJSONKey: %s has no field %q at path %q", path, field, pathString(steps))
	}
	delete(container, field)
	writeJSONFile(t, path, root)
}

// TestSchemaRequiredFieldsMatchLoaderEnforcement is the core cross-check:
// for every "required" field a schema document declares — at ANY nesting
// depth, resolved through properties/items/oneOf/$ref — deleting that
// field from wherever a valid fixture demonstrates that exact structure
// must make Load fail. This is an injection-style test by construction
// (each subtest IS the fault injection); it is also the generalized
// mechanism that would have caught (and now does directly re-test)
// resources[].thresholds[].remove_when_false, usage.limited.cost, and
// targeting.range without those three needing to be named specially.
//
// If a schema-declared nested requirement has no demonstration anywhere
// in testdata/valid (e.g. this fixture set never puts an apply_condition
// outcome inside a "miss" list), the subtest is skipped rather than
// failed — the walker can only exercise what fixtures actually show it;
// growing the fixture set to demonstrate more combinations is a fixture
// task, not evidence of a loader bug.
func TestSchemaRequiredFieldsMatchLoaderEnforcement(t *testing.T) {
	cases := []struct {
		schemaFile string
		probeFiles []string // tried in order; first with the field present wins
	}{
		{"ruleset.schema.json", []string{"ruleset.json"}},
		{"ability.schema.json", []string{"abilities/strike.json", "abilities/guard-stance.json", "abilities/stand-down.json"}},
		{"condition.schema.json", []string{"conditions/guarded.json"}},
	}

	for _, tc := range cases {
		schema := readSchema(t, tc.schemaFile)
		reqCases := dedupeReqCases(collectRequired(t, schema, schema, nil, 0))
		if len(reqCases) == 0 {
			t.Fatalf("%s declares no required fields anywhere — nothing to cross-check (this itself would be a schema authoring bug)", tc.schemaFile)
		}

		for _, rc := range reqCases {
			name := tc.schemaFile + "/"
			if p := pathString(rc.path); p != "" {
				name += p + "."
			}
			name += rc.field

			t.Run(name, func(t *testing.T) {
				var targetFile string
				for _, pf := range tc.probeFiles {
					data := readJSONFile(t, filepath.Join("testdata/valid", pf))
					container, found := navigatePath(any(data), rc.path)
					if !found {
						continue
					}
					if _, hasField := container[rc.field]; hasField {
						targetFile = pf
						break
					}
				}
				if targetFile == "" {
					t.Skipf("no fixture under testdata/valid demonstrates %s required field %q at path %q — nothing to exercise", tc.schemaFile, rc.field, pathString(rc.path))
				}

				dir := t.TempDir()
				copyDir(t, "testdata/valid", dir)
				deleteNestedJSONKey(t, filepath.Join(dir, targetFile), rc.path, rc.field)

				_, err := rules.Load(dir)
				if err == nil {
					t.Fatalf("%s declares %q required at %q, but Load succeeded after deleting it from %s", tc.schemaFile, rc.field, pathString(rc.path), targetFile)
				}
			})
		}
	}
}

// TestSchemaRequiredFieldsMatchLoaderEnforcementV2 is
// TestSchemaRequiredFieldsMatchLoaderEnforcement's format-v2 counterpart
// (P10 task-2 brief: "the nested-required anti-drift walker must cover
// atom.schema.json"). The walker mechanism above does NOT auto-glob
// schema/*.json — its `cases` table is hand-maintained and hardcoded to
// testdata/valid, so atom.schema.json (new this task) and ability.
// schema.json's v2 "compose" branch need their OWN entry point: running
// them through the v1 function's `cases` table would just skip every v2
// requirement (as the v1 test's own skip messages already show for
// "compose" — testdata/valid, a v1 fixture set, never demonstrates it).
// Same collectRequired/navigatePath/deleteNestedJSONKey/dedupeReqCases
// machinery, pointed at testdata/valid-v2 instead. Kept as a separate
// function (duplicating the v1 loop's body rather than parameterizing it)
// so this task's addition carries zero risk of altering the v1 test's
// behavior — v1 loading, and everything that verifies it, stays untouched
// per this task's brief.
func TestSchemaRequiredFieldsMatchLoaderEnforcementV2(t *testing.T) {
	cases := []struct {
		schemaFile string
		probeFiles []string // tried in order; first with the field present wins
	}{
		{"atom.schema.json", []string{
			"atoms/reach-delivery.json", // targeting contribution
			"atoms/clash-roll.json",     // resolution contribution
			"atoms/clash-damage.json",   // outcome contribution + resource_change effect
			"atoms/rally-effect.json",   // outcome contribution, branch "always" / null key
			"atoms/ward-mark.json",      // outcome effects[0]: apply_condition (review fix wave, item 4 — closes the prior apply_condition SKIP)
			"atoms/ward-clear.json",     // outcome effects[0]: remove_condition (walker only probes array index 0, so this needed its own file — closes the prior remove_condition SKIP)
			"atoms/wide-delivery.json",  // targeting max_targets as a "{param}" placeholder
			"atoms/guard-check.json",    // defense-kind param used in "vs"
		}},
		{"ability.schema.json", []string{
			"abilities/quick-jab.json",
			"abilities/rally.json",
			"abilities/tag-team.json",
			"abilities/ward-shift.json",
		}},
	}

	for _, tc := range cases {
		schema := readSchema(t, tc.schemaFile)
		reqCases := dedupeReqCases(collectRequired(t, schema, schema, nil, 0))
		if len(reqCases) == 0 {
			t.Fatalf("%s declares no required fields anywhere — nothing to cross-check (this itself would be a schema authoring bug)", tc.schemaFile)
		}

		for _, rc := range reqCases {
			name := tc.schemaFile + "/"
			if p := pathString(rc.path); p != "" {
				name += p + "."
			}
			name += rc.field

			t.Run(name, func(t *testing.T) {
				var targetFile string
				for _, pf := range tc.probeFiles {
					data := readJSONFile(t, filepath.Join("testdata/valid-v2", pf))
					container, found := navigatePath(any(data), rc.path)
					if !found {
						continue
					}
					if _, hasField := container[rc.field]; hasField {
						targetFile = pf
						break
					}
				}
				if targetFile == "" {
					t.Skipf("no fixture under testdata/valid-v2 demonstrates %s required field %q at path %q — nothing to exercise", tc.schemaFile, rc.field, pathString(rc.path))
				}

				dir := t.TempDir()
				copyDir(t, "testdata/valid-v2", dir)
				deleteNestedJSONKey(t, filepath.Join(dir, targetFile), rc.path, rc.field)

				_, err := rules.Load(dir)
				if err == nil {
					t.Fatalf("%s declares %q required at %q, but Load succeeded after deleting it from %s", tc.schemaFile, rc.field, pathString(rc.path), targetFile)
				}
			})
		}
	}
}

// TestSchemaFormatVersionEnumMatchesLoader cross-checks the declared enum
// on ruleset.schema.json's format_version against what Load actually
// accepts: every enum member must load cleanly (format-version-wise), and
// a representative non-member must be rejected naming format_version.
func TestSchemaFormatVersionEnumMatchesLoader(t *testing.T) {
	schema := readSchema(t, "ruleset.schema.json")
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal(`ruleset.schema.json "properties" is not an object`)
	}
	fv, ok := props["format_version"].(map[string]any)
	if !ok {
		t.Fatal(`ruleset.schema.json properties.format_version is missing or not an object`)
	}
	enumRaw, ok := fv["enum"].([]any)
	if !ok || len(enumRaw) == 0 {
		t.Fatal(`ruleset.schema.json properties.format_version has no non-empty "enum"`)
	}

	for _, v := range enumRaw {
		version, ok := v.(string)
		if !ok {
			t.Fatalf("format_version enum entry is not a string: %#v", v)
		}
		t.Run("accepts_"+version, func(t *testing.T) {
			dir := t.TempDir()
			copyDir(t, "testdata/valid", dir)
			setJSONField(t, filepath.Join(dir, "ruleset.json"), "format_version", version)
			if _, err := rules.Load(dir); err != nil {
				t.Errorf("Load with format_version %q (declared in the schema enum): unexpected error: %v", version, err)
			}
		})
	}

	// A value absent from the declared enum must be rejected, and the
	// rejection must name format_version — otherwise the schema's enum
	// claim and the loader's actual accept-set have drifted apart.
	t.Run("rejects_non_member", func(t *testing.T) {
		for _, v := range enumRaw {
			if v == "not-a-real-version" {
				t.Fatal("test fixture collision: pick a different non-member probe value")
			}
		}
		dir := t.TempDir()
		copyDir(t, "testdata/valid", dir)
		setJSONField(t, filepath.Join(dir, "ruleset.json"), "format_version", "not-a-real-version")
		_, err := rules.Load(dir)
		if err == nil {
			t.Fatal(`Load with format_version "not-a-real-version" (not in the schema enum): want error, got nil`)
		}
		if !strings.Contains(err.Error(), "format_version") {
			t.Errorf("error = %q, want it to mention format_version", err)
		}
	})
}

// TestSchemaOutcomeOneOfMatchesLoader cross-checks the ability/atom
// schemas' outcome-effect oneOf (exactly one of resource_change/
// apply_condition/remove_condition) against the loader: an effect object
// setting ZERO of the three, and one setting TWO, must both be rejected —
// AND named as this specific validation rule (not just "some error"),
// since a v2 outcome-effect list lives inside an atom's outcome
// contribution and several OTHER load-time checks could otherwise mask a
// gap here with an unrelated rejection.
//
// Format v2 (Task 4): v1's abilityJSON.Hit/Miss/Effect (a bare outcome
// list on the ability itself) is gone — the shape this test exercises now
// lives exclusively inside an atom's outcome contribution's "effects"
// array (decodeOutcomeContribution, load.go), so this test targets
// testdata/valid/atoms/apply-guarded.json instead of an ability file.
// Before this rewrite, this test targeted abilities/guard-stance.json's
// (removed) top-level "effect" field — once that field stopped existing
// in valid v2 ability JSON, writing to it just tripped
// decodeStrict's DisallowUnknownFields ("unknown field \"effect\"") and
// the test kept PASSING for an entirely different, unintended reason
// (any error satisfies "err == nil" failing) — a latent coverage gap this
// rewrite closes, not just a mechanical port.
func TestSchemaOutcomeOneOfMatchesLoader(t *testing.T) {
	t.Run("zero_of_three", func(t *testing.T) {
		dir := t.TempDir()
		copyDir(t, "testdata/valid", dir)
		writeOutcomeAtomEffects(t, dir, `[ {} ]`)
		_, err := rules.Load(dir)
		if err == nil {
			t.Fatal("Load with an effect setting none of resource_change/apply_condition/remove_condition: want error, got nil")
		}
		if !strings.Contains(err.Error(), "must set exactly one of") {
			t.Errorf("error = %q, want it to name the exactly-one-of-three rule (not some other rejection)", err)
		}
	})
	t.Run("two_of_three", func(t *testing.T) {
		dir := t.TempDir()
		copyDir(t, "testdata/valid", dir)
		writeOutcomeAtomEffects(t, dir, `[ {"apply_condition": {"id": "guarded"}, "remove_condition": {"id": "guarded"}} ]`)
		_, err := rules.Load(dir)
		if err == nil {
			t.Fatal("Load with an effect setting two of resource_change/apply_condition/remove_condition: want error, got nil")
		}
		if !strings.Contains(err.Error(), "must set exactly one of") {
			t.Errorf("error = %q, want it to name the exactly-one-of-three rule (not some other rejection)", err)
		}
	})
}

// writeOutcomeAtomEffects overwrites atoms/apply-guarded.json's
// contributes[0].effects list with rawEffects (a JSON array literal) for
// TestSchemaOutcomeOneOfMatchesLoader — the v2 home of the outcome-effect
// oneOf shape (decodeOutcomeContribution, load.go).
func writeOutcomeAtomEffects(t *testing.T, dir, rawEffects string) {
	t.Helper()
	path := filepath.Join(dir, "atoms", "apply-guarded.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("writeOutcomeAtomEffects: read %s: %v", path, err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("writeOutcomeAtomEffects: unmarshal %s: %v", path, err)
	}
	var contributes []map[string]json.RawMessage
	if err := json.Unmarshal(obj["contributes"], &contributes); err != nil {
		t.Fatalf("writeOutcomeAtomEffects: unmarshal %s contributes: %v", path, err)
	}
	if len(contributes) != 1 {
		t.Fatalf("writeOutcomeAtomEffects: %s: want exactly 1 contribution, got %d", path, len(contributes))
	}
	contributes[0]["effects"] = json.RawMessage(rawEffects)
	contributesOut, err := json.Marshal(contributes)
	if err != nil {
		t.Fatalf("writeOutcomeAtomEffects: marshal contributes: %v", err)
	}
	obj["contributes"] = contributesOut
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("writeOutcomeAtomEffects: marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("writeOutcomeAtomEffects: write %s: %v", path, err)
	}
}
