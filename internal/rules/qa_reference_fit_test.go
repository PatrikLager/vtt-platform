package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaRF2PlacesOne = `[
 {"key": "mike", "name": "Mike", "description": "The QA mike.",
  "forms": ["zulu"], "states": ["lima", "oscar"]},
 {"key": "kilo", "name": "Kilo", "description": "The QA kilo.",
  "forms": ["zulu", "yankee"], "copies": true}
]`

const qaRF2PlacesTwo = `[
 {"key": "mike", "name": "Mike", "description": "The QA mike.",
  "forms": ["zulu", "yankee"]},
 {"key": "kilo", "name": "Kilo", "description": "The QA kilo.",
  "forms": ["zulu"], "copies": true, "states": ["lima", "oscar"]}
]`

const qaRF2PlacesThree = `[
 {"key": "mike", "name": "Mike", "description": "The QA mike.",
  "forms": ["zulu"]},
 {"key": "kilo", "name": "Kilo", "description": "The QA kilo.",
  "forms": ["yankee"], "copies": true, "states": ["lima", "oscar"]}
]`

const qaRF2Manifest = `{
 "id": "qa-rf2", "name": "QA RF2", "format_version": "3", "version": "1",
 "properties": [
  {"key": "pool_a", "type": "resource", "name": "Pool a",
   "description": "The QA pool."},
  {"key": "brawn", "type": "number", "name": "Brawn",
   "description": "The QA brawn."},
  {"key": "guard", "type": "number", "name": "Guard",
   "description": "The QA guard."}
 ],
 "forms": [
  {"key": "zulu", "name": "Zulu", "description": "The QA zulu."},
  {"key": "yankee", "name": "Yankee", "description": "The QA yankee."}
 ],
 "places": PLACES
}`

func qaRF2CopyDir(t *testing.T, from, to string) {
	t.Helper()
	files, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(from, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, f.Name()), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func qaRF2Write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func qaRF2Load(t *testing.T, places string) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("testdata", "valid")
	for _, sub := range []string{"abilities", "atoms", "conditions"} {
		qaRF2CopyDir(t, filepath.Join(src, sub), filepath.Join(dir, sub))
	}
	guide, err := os.ReadFile(filepath.Join(src, "guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	qaRF2Write(t, filepath.Join(dir, "guide.md"), string(guide))
	qaRF2Write(t, filepath.Join(dir, "ruleset.json"),
		strings.Replace(qaRF2Manifest, "PLACES", places, 1))
	cat := filepath.Join(dir, "catalogue")
	if err := os.MkdirAll(cat, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, form := range map[string]string{"echo": "zulu", "delta": "yankee"} {
		qaRF2Write(t, filepath.Join(cat, id+".json"), `{"id": "`+id+
			`", "name": "N", "description": "The QA entry.", "form": "`+form+`"}`)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return rs
}

func qaRF2Ref(entry, place, state string) *vttv1.Reference {
	return &vttv1.Reference{EntryId: entry, Place: place, State: state}
}

func qaRF2Want(t *testing.T, f *rules.SheetFault, id string, want, not []string) {
	t.Helper()
	if f == nil {
		t.Fatalf("fault = nil, want one on reference %q", id)
	}
	if f.Field != "references" {
		t.Errorf("Field = %q, want references", f.Field)
	}
	if !strings.Contains(f.Reason, `"`+id+`"`) {
		t.Errorf("Reason %q does not name reference %q", f.Reason, id)
	}
	for _, w := range want {
		if !strings.Contains(f.Reason, w) {
			t.Errorf("Reason %q does not name %q", f.Reason, w)
		}
	}
	for _, n := range not {
		if strings.Contains(f.Reason, n) {
			t.Errorf("Reason %q names %q, a later fault", f.Reason, n)
		}
	}
}

// VTT-1037.
func TestQARF2EachFaultAlone(t *testing.T) {
	rs := qaRF2Load(t, qaRF2PlacesOne)
	cases := []struct {
		name string
		ref  *vttv1.Reference
		want []string
	}{
		{"entry not held", qaRF2Ref("november", "mike", "lima"), []string{"november"}},
		{"entry in another case", qaRF2Ref("Echo", "mike", "lima"), []string{"Echo"}},
		{"place not declared", qaRF2Ref("echo", "papa", "lima"), []string{"papa"}},
		{"place in another case", qaRF2Ref("echo", "Mike", "lima"), []string{"Mike"}},
		{"form the place does not hold", qaRF2Ref("delta", "mike", "lima"), nil},
		{"state not named", qaRF2Ref("echo", "mike", "quebec"), []string{"quebec"}},
		{"state in another case", qaRF2Ref("echo", "mike", "Lima"), []string{"Lima"}},
		{"no state where some", qaRF2Ref("echo", "mike", ""), nil},
		{"state where none", qaRF2Ref("echo", "kilo", "lima"), []string{"lima"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			qaRF2Want(t, rs.ReferenceFault("romeo", c.ref), "romeo", c.want, nil)
			refs := map[string]*vttv1.Reference{"romeo": c.ref}
			qaRF2Want(t, rs.SheetReferenceFault(refs), "romeo", c.want, nil)
		})
	}
}

// VTT-1037, VTT-1038.
func TestQARF2FittingReferencesAnswerNil(t *testing.T) {
	rs := qaRF2Load(t, qaRF2PlacesOne)
	fits := map[string]*vttv1.Reference{
		"a": qaRF2Ref("echo", "mike", "lima"),
		"b": qaRF2Ref("delta", "kilo", ""),
		"c": qaRF2Ref("echo", "kilo", ""),
	}
	for id, r := range fits {
		if f := rs.ReferenceFault(id, r); f != nil {
			t.Errorf("ReferenceFault(%q) = %+v, want nil", id, *f)
		}
	}
	if f := rs.SheetReferenceFault(fits); f != nil {
		t.Errorf("SheetReferenceFault = %+v, want nil", *f)
	}
	if f := rs.SheetReferenceFault(nil); f != nil {
		t.Errorf("SheetReferenceFault(nil) = %+v, want nil", *f)
	}
	if f := rs.SheetReferenceFault(map[string]*vttv1.Reference{}); f != nil {
		t.Errorf("SheetReferenceFault(empty) = %+v, want nil", *f)
	}
}

// VTT-1037.
func TestQARF2FaultPairsInOneReference(t *testing.T) {
	one := qaRF2Load(t, qaRF2PlacesOne)
	three := qaRF2Load(t, qaRF2PlacesThree)
	cases := []struct {
		name      string
		rs        *rules.Ruleset
		ref       *vttv1.Reference
		want, not []string
	}{
		{"entry before place", one, qaRF2Ref("november", "papa", "quebec"),
			[]string{"november"}, []string{"papa", "quebec"}},
		{"entry before state not named", one, qaRF2Ref("november", "mike", "quebec"),
			[]string{"november"}, []string{"quebec"}},
		{"entry before no state", one, qaRF2Ref("november", "mike", ""),
			[]string{"november"}, []string{"state"}},
		{"entry before state where none", one, qaRF2Ref("november", "kilo", "quebec"),
			[]string{"november"}, []string{"quebec"}},
		{"place before state", one, qaRF2Ref("echo", "papa", "quebec"),
			[]string{"papa"}, []string{"quebec"}},
		{"form before state not named", one, qaRF2Ref("delta", "mike", "quebec"),
			[]string{"delta"}, []string{"quebec"}},
		{"form before no state", one, qaRF2Ref("delta", "mike", ""),
			[]string{"delta"}, []string{"state"}},
		{"form before state where none", three, qaRF2Ref("delta", "mike", "quebec"),
			[]string{"delta"}, []string{"quebec"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := c.rs.ReferenceFault("romeo", c.ref)
			qaRF2Want(t, f, "romeo", c.want, c.not)
			refs := map[string]*vttv1.Reference{"romeo": c.ref}
			g := c.rs.SheetReferenceFault(refs)
			if g == nil || f == nil || g.Reason != f.Reason {
				t.Errorf("SheetReferenceFault = %v, ReferenceFault = %v", g, f)
			}
		})
	}
}

// VTT-1037.
func TestQARF2FaultsAcrossReferencesInByteOrder(t *testing.T) {
	rs := qaRF2Load(t, qaRF2PlacesOne)
	cases := []struct {
		name string
		refs map[string]*vttv1.Reference
		want string
	}{
		{"upper case before lower", map[string]*vttv1.Reference{
			"alpha": qaRF2Ref("november", "mike", "lima"),
			"Zed":   qaRF2Ref("echo", "mike", "quebec"),
		}, "Zed"},
		{"a prefix before its extension", map[string]*vttv1.Reference{
			"ab": qaRF2Ref("november", "mike", "lima"),
			"a":  qaRF2Ref("echo", "kilo", "lima"),
		}, "a"},
		{"not by entry id", map[string]*vttv1.Reference{
			"b": qaRF2Ref("echo", "papa", ""),
			"c": qaRF2Ref("delta", "papa", ""),
			"d": qaRF2Ref("echo", "mike", "lima"),
		}, "b"},
		{"a fitting lower id is passed over", map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "mike", "lima"),
			"b": qaRF2Ref("echo", "mike", ""),
			"c": qaRF2Ref("november", "mike", "lima"),
		}, "b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for range 20 {
				f := rs.SheetReferenceFault(c.refs)
				if f == nil {
					t.Fatal("fault = nil")
				}
				if !strings.Contains(f.Reason, `"`+c.want+`"`) {
					t.Fatalf("Reason %q, want reference %q", f.Reason, c.want)
				}
			}
		})
	}
}

// VTT-1038.
func TestQARF2CopiesInAPlaceThatHoldsOnce(t *testing.T) {
	rs := qaRF2Load(t, qaRF2PlacesOne)
	cases := []struct {
		name string
		refs map[string]*vttv1.Reference
		want string
	}{
		{"same state", map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "mike", "lima"),
			"b": qaRF2Ref("echo", "mike", "lima"),
		}, "b"},
		{"another state", map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "mike", "lima"),
			"b": qaRF2Ref("echo", "mike", "oscar"),
		}, "b"},
		{"byte order of ids", map[string]*vttv1.Reference{
			"alpha": qaRF2Ref("echo", "mike", "lima"),
			"Zed":   qaRF2Ref("echo", "mike", "oscar"),
		}, "alpha"},
		{"third of three", map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "mike", "lima"),
			"b": qaRF2Ref("delta", "kilo", ""),
			"c": qaRF2Ref("echo", "mike", "oscar"),
		}, "c"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, r := range c.refs {
				if f := rs.ReferenceFault("x", r); f != nil {
					t.Fatalf("fixture reference does not fit: %+v", *f)
				}
			}
			for range 20 {
				f := rs.SheetReferenceFault(c.refs)
				qaRF2Want(t, f, c.want, []string{"echo"}, nil)
				if t.Failed() {
					return
				}
			}
		})
	}
}

// VTT-1038.
func TestQARF2FirstCopyOfTwoEntries(t *testing.T) {
	rs := qaRF2Load(t, qaRF2PlacesTwo)
	refs := map[string]*vttv1.Reference{
		"a": qaRF2Ref("echo", "mike", ""),
		"b": qaRF2Ref("delta", "mike", ""),
		"c": qaRF2Ref("delta", "mike", ""),
		"d": qaRF2Ref("echo", "mike", ""),
	}
	for range 20 {
		qaRF2Want(t, rs.SheetReferenceFault(refs), "c", []string{"delta"}, nil)
		if t.Failed() {
			return
		}
	}
}

// VTT-1038.
func TestQARF2CopiesAccepted(t *testing.T) {
	one := qaRF2Load(t, qaRF2PlacesOne)
	two := qaRF2Load(t, qaRF2PlacesTwo)
	cases := []struct {
		name string
		rs   *rules.Ruleset
		refs map[string]*vttv1.Reference
	}{
		{"twice in a place that holds copies", one, map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "kilo", ""),
			"b": qaRF2Ref("echo", "kilo", ""),
			"c": qaRF2Ref("echo", "kilo", ""),
		}},
		{"copies with a state", two, map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "kilo", "lima"),
			"b": qaRF2Ref("echo", "kilo", "lima"),
		}},
		{"once in each of two places", one, map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "mike", "lima"),
			"b": qaRF2Ref("echo", "kilo", ""),
		}},
		{"two entries in a place that holds once", two, map[string]*vttv1.Reference{
			"a": qaRF2Ref("echo", "mike", ""),
			"b": qaRF2Ref("delta", "mike", ""),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if f := c.rs.SheetReferenceFault(c.refs); f != nil {
				t.Errorf("SheetReferenceFault = %+v, want nil", *f)
			}
		})
	}
}

// VTT-1038.
func TestQARF2FitFaultBeforeCopiesFault(t *testing.T) {
	rs := qaRF2Load(t, qaRF2PlacesOne)
	refs := map[string]*vttv1.Reference{
		"a": qaRF2Ref("echo", "mike", "lima"),
		"b": qaRF2Ref("echo", "mike", "oscar"),
		"c": qaRF2Ref("november", "kilo", ""),
	}
	for range 20 {
		qaRF2Want(t, rs.SheetReferenceFault(refs), "c", []string{"november"}, nil)
		if t.Failed() {
			return
		}
	}
}

// VTT-1037, VTT-1038.
func TestQARF2CommittedFixturePlaces(t *testing.T) {
	rs, err := rules.Load(filepath.Join("testdata", "valid"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[bool]bool{}
	for _, p := range rs.Places {
		if rs.Place(p.Key) == nil {
			t.Fatalf("Place(%q) = nil", p.Key)
		}
		entry := ""
		for id, e := range rs.Catalogue {
			if len(p.Forms) > 0 && e.Form == p.Forms[0] {
				entry = id
			}
		}
		if entry == "" {
			t.Fatalf("no entry of place %q's forms", p.Key)
		}
		state := ""
		if len(p.States) > 0 {
			state = p.States[0]
		}
		seen[p.Copies] = true
		twice := map[string]*vttv1.Reference{
			"a": qaRF2Ref(entry, p.Key, state),
			"b": qaRF2Ref(entry, p.Key, state),
		}
		f := rs.SheetReferenceFault(twice)
		if p.Copies && f != nil {
			t.Errorf("place %q holds copies, refused: %+v", p.Key, *f)
		}
		if !p.Copies {
			qaRF2Want(t, f, "b", []string{entry}, nil)
		}
		wrong := "november"
		if state != "" {
			wrong = ""
		}
		g := rs.ReferenceFault("a", qaRF2Ref(entry, p.Key, wrong))
		qaRF2Want(t, g, "a", []string{p.Key}, nil)
	}
	if !seen[true] || !seen[false] {
		t.Fatalf("fixture places copies %v, want one of each", seen)
	}
	if rs.Place("papa") != nil {
		t.Error(`Place("papa") is not nil`)
	}
}
