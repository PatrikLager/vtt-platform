package conformance_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qascShipped struct {
	dir string
	rs  *rules.Ruleset
}

type qascEntryFile struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Form        string         `json:"form"`
	Extends     string         `json:"extends"`
	Values      map[string]int `json:"values"`
}

func qascRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "rulesets")); err != nil {
		t.Fatalf("no rulesets directory under %s: %v", root, err)
	}
	return root
}

func qascLoadRoot(root string) ([]qascShipped, error) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []qascShipped
	for _, de := range ents {
		dir := filepath.Join(root, de.Name())
		if _, err := os.Stat(filepath.Join(dir, "ruleset.json")); !de.IsDir() || err != nil {
			continue
		}
		rs, err := rules.Load(dir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		out = append(out, qascShipped{dir: dir, rs: rs})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no ruleset under %s", root)
	}
	return out, nil
}

func qascMustLoadRoot(t *testing.T, root string) []qascShipped {
	t.Helper()
	shipped, err := qascLoadRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	return shipped
}

func qascShippedRulesets(t *testing.T) []qascShipped {
	t.Helper()
	return qascMustLoadRoot(t, filepath.Join(qascRepoRoot(t), "rulesets"))
}

func qascCatalogueRuleset(t *testing.T, shipped []qascShipped) qascShipped {
	t.Helper()
	for _, s := range shipped {
		if len(s.rs.Catalogue) > 0 {
			return s
		}
	}
	t.Fatal("no shipped ruleset holds a catalogue entry")
	return qascShipped{}
}

func qascHasDigit(s string) bool {
	return strings.IndexFunc(s, unicode.IsDigit) >= 0
}

func qascEntryDescriptionFaults(shipped []qascShipped) []string {
	var faults []string
	total := 0
	for _, s := range shipped {
		for id, e := range s.rs.Catalogue {
			total++
			// VTT-1007
			if e.Description == "" {
				faults = append(faults, fmt.Sprintf(
					"%s: entry %q has no description (VTT-1007)", s.rs.ID, id))
			}
			if qascHasDigit(e.Description) {
				faults = append(faults, fmt.Sprintf(
					"%s: entry %q description %q holds a digit (VTT-1007)",
					s.rs.ID, id, e.Description))
			}
		}
	}
	// VTT-1007
	if total == 0 {
		faults = append(faults, "no catalogue entry in any shipped ruleset (VTT-1007)")
	}
	return faults
}

func qascAbilityDescriptionFaults(shipped []qascShipped) []string {
	var faults []string
	total := 0
	for _, s := range shipped {
		files, err := filepath.Glob(filepath.Join(s.dir, "abilities", "*.json"))
		if err != nil {
			faults = append(faults, err.Error())
		}
		// VTT-850
		if len(files) != len(s.rs.Compiled) {
			faults = append(faults, fmt.Sprintf(
				"%s: %d ability files but %d loaded abilities (VTT-850)",
				s.rs.ID, len(files), len(s.rs.Compiled)))
		}
		for id, cp := range s.rs.Compiled {
			total++
			// VTT-850
			if cp.Description == "" || qascHasDigit(cp.Description) {
				faults = append(faults, fmt.Sprintf(
					"%s: ability %q description %q is empty or holds a digit (VTT-850)",
					s.rs.ID, id, cp.Description))
			}
		}
	}
	if total == 0 {
		faults = append(faults, "no ability in any shipped ruleset (VTT-850)")
	}
	return faults
}

func qascAsWrittenFaults(s qascShipped) []string {
	var faults []string
	catDir := filepath.Join(s.dir, "catalogue")
	if _, err := os.Stat(catDir); err != nil {
		// VTT-997
		if len(s.rs.Catalogue) != 0 {
			faults = append(faults, fmt.Sprintf(
				"%s: no catalogue/ but %d entries loaded (VTT-997)",
				s.rs.ID, len(s.rs.Catalogue)))
		}
		return faults
	}
	files, err := filepath.Glob(filepath.Join(catDir, "*.json"))
	if err != nil {
		return append(faults, err.Error())
	}
	// VTT-997
	if len(files) != len(s.rs.Catalogue) {
		faults = append(faults, fmt.Sprintf(
			"%s: %d catalogue files but %d entries loaded (VTT-997)",
			s.rs.ID, len(files), len(s.rs.Catalogue)))
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			faults = append(faults, err.Error())
			continue
		}
		var want qascEntryFile
		if err := json.Unmarshal(raw, &want); err != nil {
			faults = append(faults, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		got := s.rs.Catalogue[want.ID]
		// VTT-997
		if got == nil {
			faults = append(faults, fmt.Sprintf(
				"%s: entry %q not held by its id (VTT-997)", path, want.ID))
			continue
		}
		if got.ID != want.ID || got.Name != want.Name ||
			got.Description != want.Description || got.Form != want.Form {
			faults = append(faults, fmt.Sprintf(
				"%s: loaded %+v, file says id %q name %q description %q form %q (VTT-997)",
				path, *got, want.ID, want.Name, want.Description, want.Form))
		}
		faults = append(faults, qascValueFaults(path, want, got)...)
	}
	return faults
}

func qascValueFaults(path string, want qascEntryFile, got *rules.Entry) []string {
	var faults []string
	// VTT-997
	for k, v := range want.Values {
		if gv, ok := got.Values[k]; !ok || gv != v {
			faults = append(faults, fmt.Sprintf(
				"%s: value %q loaded as %d (present %t), written %d (VTT-997)",
				path, k, gv, ok, v))
		}
	}
	if want.Extends == "" && len(got.Values) != len(want.Values) {
		faults = append(faults, fmt.Sprintf(
			"%s: no extends, yet %d values loaded and %d written (VTT-997)",
			path, len(got.Values), len(want.Values)))
	}
	return faults
}

func qascCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		r, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, r), 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, r), b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func qascWantNone(t *testing.T, faults []string) {
	t.Helper()
	for _, f := range faults {
		t.Error(f)
	}
}

func qascWantSome(t *testing.T, faults []string) {
	t.Helper()
	if len(faults) == 0 {
		t.Error("the break went unnoticed")
	}
	t.Logf("caught: %s", strings.Join(faults, " | "))
}

func qascReload(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func qascSortedIDs(rs *rules.Ruleset) []string {
	ids := make([]string, 0, len(rs.Catalogue))
	for id := range rs.Catalogue {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// VTT-1007
func TestQASCShippedEntryDescriptionsHoldNoDigit(t *testing.T) {
	qascWantNone(t, qascEntryDescriptionFaults(qascShippedRulesets(t)))
}

// VTT-850
func TestQASCShippedAbilityDescriptionsHoldNoDigit(t *testing.T) {
	qascWantNone(t, qascAbilityDescriptionFaults(qascShippedRulesets(t)))
}

// VTT-997
func TestQASCShippedEntriesLoadAsWritten(t *testing.T) {
	for _, s := range qascShippedRulesets(t) {
		qascWantNone(t, qascAsWrittenFaults(s))
	}
}

func TestQASCWitnessEntryDescriptions(t *testing.T) {
	src := filepath.Join(qascRepoRoot(t), "rulesets")
	copyRoot := func(t *testing.T) string {
		t.Helper()
		dst := t.TempDir()
		qascCopyDir(t, src, dst)
		return dst
	}
	t.Run("an unbroken copy passes", func(t *testing.T) {
		qascWantNone(t, qascEntryDescriptionFaults(qascMustLoadRoot(t, copyRoot(t))))
	})
	t.Run("a digit in one written description", func(t *testing.T) {
		root := copyRoot(t)
		files, err := filepath.Glob(filepath.Join(root, "*", "catalogue", "*.json"))
		if err != nil || len(files) == 0 {
			t.Fatalf("no catalogue file to break: %v", err)
		}
		var m map[string]any
		raw, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		m["description"] = fmt.Sprint(m["description"], " Weighs 3.")
		out, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(files[0], out, 0o600); err != nil {
			t.Fatal(err)
		}
		qascWantSome(t, qascEntryDescriptionFaults(qascMustLoadRoot(t, root)))
	})
	t.Run("every catalogue removed", func(t *testing.T) {
		root := copyRoot(t)
		dirs, err := filepath.Glob(filepath.Join(root, "*", "catalogue"))
		if err != nil || len(dirs) == 0 {
			t.Fatalf("no catalogue to remove: %v", err)
		}
		for _, d := range dirs {
			if err := os.RemoveAll(d); err != nil {
				t.Fatal(err)
			}
		}
		qascWantSome(t, qascEntryDescriptionFaults(qascMustLoadRoot(t, root)))
	})
	t.Run("an empty or non-ASCII digit description in memory", func(t *testing.T) {
		for _, d := range []string{"", "Weighs ٣."} {
			shipped := qascMustLoadRoot(t, src)
			s := qascCatalogueRuleset(t, shipped)
			s.rs.Catalogue[qascSortedIDs(s.rs)[0]].Description = d
			qascWantSome(t, qascEntryDescriptionFaults(shipped))
		}
	})
}

func TestQASCWitnessAbilityDescriptions(t *testing.T) {
	src := filepath.Join(qascRepoRoot(t), "rulesets")
	breaks := map[string]func(rs *rules.Ruleset, id string){
		"a digit":      func(rs *rules.Ruleset, id string) { rs.Compiled[id].Description += " 2" },
		"empty":        func(rs *rules.Ruleset, id string) { rs.Compiled[id].Description = "" },
		"one unloaded": func(rs *rules.Ruleset, id string) { delete(rs.Compiled, id) },
	}
	for name, brk := range breaks {
		t.Run(name, func(t *testing.T) {
			shipped := qascMustLoadRoot(t, src)
			for id := range shipped[0].rs.Compiled {
				brk(shipped[0].rs, id)
				break
			}
			qascWantSome(t, qascAbilityDescriptionFaults(shipped))
		})
	}
}

func TestQASCWitnessAsWritten(t *testing.T) {
	shipped := qascShippedRulesets(t)
	s := qascCatalogueRuleset(t, shipped)
	breaks := map[string]func(e *rules.Entry){
		"name":        func(e *rules.Entry) { e.Name += "x" },
		"description": func(e *rules.Entry) { e.Description += "x" },
		"form":        func(e *rules.Entry) { e.Form += "x" },
		"a value":     func(e *rules.Entry) { e.Values["qasc_extra"] = 1 },
	}
	for name, brk := range breaks {
		t.Run(name, func(t *testing.T) {
			rs := qascReload(t, s.dir)
			brk(rs.Catalogue[qascSortedIDs(rs)[0]])
			qascWantSome(t, qascAsWrittenFaults(qascShipped{dir: s.dir, rs: rs}))
		})
	}
	t.Run("an entry dropped", func(t *testing.T) {
		rs := qascReload(t, s.dir)
		delete(rs.Catalogue, qascSortedIDs(rs)[0])
		qascWantSome(t, qascAsWrittenFaults(qascShipped{dir: s.dir, rs: rs}))
	})
	t.Run("an entry where no catalogue is", func(t *testing.T) {
		dir := t.TempDir()
		qascCopyDir(t, s.dir, dir)
		if err := os.RemoveAll(filepath.Join(dir, "catalogue")); err != nil {
			t.Fatal(err)
		}
		rs := qascReload(t, dir)
		rs.Catalogue = map[string]*rules.Entry{"qasc": {ID: "qasc"}}
		qascWantSome(t, qascAsWrittenFaults(qascShipped{dir: dir, rs: rs}))
	})
}
