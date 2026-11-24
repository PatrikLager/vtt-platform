package rules_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qarvBaseRuleset = "../../rulesets/tavern-brawl"

type qarvManifest map[string]json.RawMessage

func qarvJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatalf("encode %v: %v", v, err)
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n"))
}

// qarvLoad copies the base ruleset, lets edit change its manifest, writes it
// back with extra raw `"key": value` members appended, and loads it.
func qarvLoad(t *testing.T, edit func(m qarvManifest), extra ...string) (string, *rules.Ruleset, error) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rs")
	if err := os.CopyFS(dir, os.DirFS(qarvBaseRuleset)); err != nil {
		t.Fatalf("copy base ruleset: %v", err)
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	m := qarvManifest{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if edit != nil {
		edit(m)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	members := make([]string, 0, len(keys)+len(extra))
	for _, k := range keys {
		members = append(members, string(qarvJSON(t, k))+": "+string(m[k]))
	}
	members = append(members, extra...)
	out := "{\n" + strings.Join(members, ",\n") + "\n}\n"
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	rs, err := rules.Load(dir)
	return dir, rs, err
}

var qarvWord = regexp.MustCompile(`[A-Za-z0-9_]`)

// qarvNamesField: the field as a whole word, so "format_version" does not
// count as naming "version".
func qarvNamesField(msg, field string) bool {
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(field) + `($|[^A-Za-z0-9_])`)
	if qarvWord.MatchString(field[len(field)-1:]) {
		return re.MatchString(msg)
	}
	return strings.Contains(msg, `"`+field+`"`)
}

func qarvRequireRefusal(t *testing.T, dir string, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("loaded; want a refusal naming ruleset.json and field %q", field)
	}
	msg := strings.ReplaceAll(err.Error(), dir, "<dir>")
	t.Logf("refusal: %s", msg)
	if !strings.Contains(msg, "ruleset.json") {
		t.Errorf("refusal does not name the file ruleset.json: %s", msg)
	}
	if !qarvNamesField(msg, field) {
		t.Errorf("refusal does not name the field %q: %s", field, msg)
	}
}

func qarvRequireLoads(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("refused; want it to load: %v", err)
	}
}

func qarvSet(t *testing.T, key string, v any) func(m qarvManifest) {
	t.Helper()
	raw := qarvJSON(t, v)
	return func(m qarvManifest) { m[key] = raw }
}

// VTT-970
func TestQARVTheDeclaredVersionIsKeptAsWritten(t *testing.T) {
	for _, v := range []string{
		"2.0.0-rc.1+build.7",
		" 1.0 ",
		"  ",
		"0",
		"版本 ß — 2",
		"\t3\n",
	} {
		t.Run(v, func(t *testing.T) {
			_, rs, err := qarvLoad(t, qarvSet(t, "version", v))
			qarvRequireLoads(t, err)
			if rs.Version != v { // VTT-970
				t.Errorf("Version = %q, want %q as written", rs.Version, v)
			}
		})
	}
}

// VTT-971
func TestQARVAnAbsentVersionIsRefusedNamingFileAndField(t *testing.T) {
	dir, _, err := qarvLoad(t, func(m qarvManifest) { delete(m, "version") })
	qarvRequireRefusal(t, dir, err, "version")
}

// VTT-971
func TestQARVAnEmptyVersionIsRefusedNamingFileAndField(t *testing.T) {
	dir, _, err := qarvLoad(t, qarvSet(t, "version", ""))
	qarvRequireRefusal(t, dir, err, "version")
}

// VTT-971: a member present as null is read as absent (SPEC-024).
func TestQARVANullVersionIsRefusedNamingFileAndField(t *testing.T) {
	dir, _, err := qarvLoad(t, func(m qarvManifest) { m["version"] = json.RawMessage("null") })
	qarvRequireRefusal(t, dir, err, "version")
}

// qarvOfBytes: exactly n bytes; with multi, of multi-byte runes, so that the
// rune count stays far below n.
func qarvOfBytes(n int, multi bool) string {
	if !multi {
		return strings.Repeat("x", n)
	}
	s := strings.Repeat("€", n/3)
	return s + strings.Repeat("y", n-len(s))
}

func qarvRunes(s string) int { return len([]rune(s)) }

// VTT-972
func TestQARVEachOfIDVersionAndNameAtTheBoundLoads(t *testing.T) {
	for _, tc := range []struct {
		field string
		bound int
	}{{"id", 128}, {"version", 128}, {"name", 256}} {
		for _, multi := range []bool{false, true} {
			v := qarvOfBytes(tc.bound, multi)
			if len(v) != tc.bound {
				t.Fatalf("fixture %q has %d bytes, want %d", tc.field, len(v), tc.bound)
			}
			t.Run(tc.field+map[bool]string{false: "_ascii", true: "_multibyte"}[multi], func(t *testing.T) {
				_, rs, err := qarvLoad(t, qarvSet(t, tc.field, v))
				qarvRequireLoads(t, err) // VTT-972
				if tc.field == "version" && rs.Version != v {
					t.Errorf("Version = %q, want %q", rs.Version, v) // VTT-970
				}
			})
		}
	}
}

// VTT-972
func TestQARVIDVersionAndNameAllAtTheBoundTogetherLoad(t *testing.T) {
	version := qarvOfBytes(128, true)
	_, rs, err := qarvLoad(t, func(m qarvManifest) {
		m["id"] = qarvJSON(t, qarvOfBytes(128, false))
		m["version"] = qarvJSON(t, version)
		m["name"] = qarvJSON(t, qarvOfBytes(256, true))
	})
	qarvRequireLoads(t, err)
	if rs.Version != version { // VTT-970
		t.Errorf("Version = %q, want %q", rs.Version, version)
	}
}

// VTT-972: the multi-byte cases are over the bound in bytes but not in runes.
func TestQARVEachOfIDVersionAndNameOverTheBoundIsRefused(t *testing.T) {
	for _, tc := range []struct {
		field string
		bound int
	}{{"id", 128}, {"version", 128}, {"name", 256}} {
		for _, multi := range []bool{false, true} {
			v := qarvOfBytes(tc.bound+1, multi)
			if len(v) != tc.bound+1 || (multi && qarvRunes(v) > tc.bound) {
				t.Fatalf("fixture %q: %d bytes, %d runes", tc.field, len(v), qarvRunes(v))
			}
			t.Run(tc.field+map[bool]string{false: "_ascii", true: "_multibyte"}[multi], func(t *testing.T) {
				dir, _, err := qarvLoad(t, qarvSet(t, tc.field, v))
				qarvRequireRefusal(t, dir, err, tc.field) // VTT-972
			})
		}
	}
}

// VTT-996
func TestQARVAnUndefinedTopLevelFieldIsRefusedNamingFileAndField(t *testing.T) {
	for _, field := range []string{"author", "versions", "vers", "ruleset_version", "version "} {
		t.Run(field, func(t *testing.T) {
			dir, _, err := qarvLoad(t, nil, string(qarvJSON(t, field))+`: "1"`)
			qarvRequireRefusal(t, dir, err, field)
		})
	}
}

// VTT-996: a case variant matches a defined field, so it is not undefined.
func TestQARVACaseVariantOfEachDefinedTopLevelFieldIsNotRefused(t *testing.T) {
	for _, tc := range []struct{ field, variant, value string }{
		{"id", "ID", ""},
		{"name", "Name", ""},
		{"format_version", "FORMAT_VERSION", ""},
		{"version", "VERSION", ""},
		{"properties", "Properties", ""},
		{"sheet", "Sheet", "null"},
		{"free_usage_name", "Free_Usage_Name", `"Free"`},
	} {
		t.Run(tc.variant, func(t *testing.T) {
			_, _, err := qarvLoad(t, func(m qarvManifest) {
				if tc.value != "" {
					m[tc.field] = json.RawMessage(tc.value)
				}
				m[tc.variant] = m[tc.field]
			})
			qarvRequireLoads(t, err) // VTT-996
		})
	}
}

// VTT-996 and VTT-970: "Version" is the field version, compared without
// regard to case, so its value is the ruleset's version.
func TestQARVACaseVariantVersionAloneIsTheVersion(t *testing.T) {
	_, rs, err := qarvLoad(t, func(m qarvManifest) {
		delete(m, "version")
		m["Version"] = qarvJSON(t, "4.5-case")
	})
	qarvRequireLoads(t, err)
	if rs.Version != "4.5-case" {
		t.Errorf("Version = %q, want %q", rs.Version, "4.5-case")
	}
}

type qarvSheetEdit struct {
	page, section, spends, sectionName  string
	extraPage, extraSection, extraUsage string
}

func qarvSheet(e qarvSheetEdit) string {
	or := func(s, def string) string {
		if s == "" {
			return def
		}
		return s
	}
	return `[{"` + or(e.page, "name") + `": "Main"` + e.extraPage + `, "sections": [` +
		`{"` + or(e.sectionName, "name") + `": "Values", "` + or(e.section, "values") + `": ["brawn"]` + e.extraSection + `},` +
		`{"name": "State", "conditions": true},` +
		`{"name": "Moves", "abilities": ["at_will", {"` + or(e.spends, "spends") + `": "stamina"` + e.extraUsage + `}, {"spends": "drink"}]}` +
		`]}]`
}

// VTT-996: the control the sheet cases below vary.
func TestQARVTheBaseSheetLoads(t *testing.T) {
	_, _, err := qarvLoad(t, func(m qarvManifest) {
		m["sheet"] = json.RawMessage(qarvSheet(qarvSheetEdit{}))
	})
	qarvRequireLoads(t, err)
}

// VTT-996
func TestQARVAnUndefinedFieldWithinTheSheetIsRefusedNamingFileAndField(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		edit        qarvSheetEdit
	}{
		{"page", "title", qarvSheetEdit{extraPage: `, "title": "x"`}},
		{"section", "remarks", qarvSheetEdit{extraSection: `, "remarks": "x"`}},
		{"usage", "limit", qarvSheetEdit{extraUsage: `, "limit": 1`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, _, err := qarvLoad(t, func(m qarvManifest) {
				m["sheet"] = json.RawMessage(qarvSheet(tc.edit))
			})
			qarvRequireRefusal(t, dir, err, tc.field)
		})
	}
}

// VTT-996: within the sheet too, a field is compared without regard to case.
func TestQARVACaseVariantOfASheetFieldIsNotRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit qarvSheetEdit
	}{
		{"page_NAME", qarvSheetEdit{page: "NAME"}},
		{"section_Name", qarvSheetEdit{sectionName: "Name"}},
		{"section_VALUES", qarvSheetEdit{section: "VALUES"}},
		{"usage_SPENDS", qarvSheetEdit{spends: "SPENDS"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := qarvLoad(t, func(m qarvManifest) {
				m["sheet"] = json.RawMessage(qarvSheet(tc.edit))
			})
			qarvRequireLoads(t, err)
		})
	}
}

// VTT-970: every shipped ruleset declares a non-empty version and loads with
// it.
func TestQARVEveryShippedRulesetDeclaresItsVersion(t *testing.T) {
	entries, err := os.ReadDir("../../rulesets")
	if err != nil {
		t.Fatalf("read rulesets: %v", err)
	}
	seen := 0
	for _, e := range entries {
		dir := filepath.Join("../../rulesets", e.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "ruleset.json"))
		if !e.IsDir() || err != nil {
			continue
		}
		seen++
		t.Run(e.Name(), func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("parse: %v", err)
			}
			declared, ok := m["version"].(string)
			if !ok || declared == "" {
				t.Fatalf("ruleset.json declares no non-empty string version: %v", m["version"])
			}
			rs, err := rules.Load(dir)
			qarvRequireLoads(t, err)
			if rs.Version != declared {
				t.Errorf("Version = %q, want %q", rs.Version, declared)
			}
		})
	}
	if seen == 0 {
		t.Fatal("found no shipped ruleset")
	}
}

// VTT-986
func TestQARVTheSchemaDescribesVersionAsARequiredNonEmptyBoundedString(t *testing.T) {
	raw, err := os.ReadFile("schema/ruleset.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var s struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type        string `json:"type"`
			MinLength   *int   `json:"minLength"`
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	required := false
	for _, r := range s.Required {
		required = required || r == "version"
	}
	if !required {
		t.Errorf("required = %v, want it to hold \"version\"", s.Required)
	}
	v, ok := s.Properties["version"]
	if !ok {
		t.Fatalf("schema has no property \"version\"")
	}
	if v.Type != "string" {
		t.Errorf("version type = %q, want string", v.Type)
	}
	if v.MinLength == nil {
		t.Errorf("version has no minLength, want 1 (non-empty)")
	} else if *v.MinLength != 1 {
		t.Errorf("version minLength = %d, want 1 (non-empty)", *v.MinLength)
	}
	if !regexp.MustCompile(`(?i)at most 128 bytes`).MatchString(v.Description) {
		t.Errorf("version description does not state at most 128 bytes: %q", v.Description)
	}
}
