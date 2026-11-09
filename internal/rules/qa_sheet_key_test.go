package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Keep a declared key on each side, in byte order, of the key each
// single-fault test faults, the case-only tests aside.
func qaSKRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	manifest := `{"id":"qa-skrs","name":"QA Sheet Keys","format_version":"3","properties":[
	{"key":"qa_alpha","type":"number","name":"Alpha","description":"A number."},
	{"key":"qa_num","type":"number","name":"Num","description":"A number."},
	{"key":"qa_zeta","type":"number","name":"Zeta","description":"A number."},
	{"key":"qa_pool","type":"resource","name":"Pool","description":"A resource."},
	{"key":"qa_spare","type":"resource","name":"Spare","description":"A resource."},
	{"key":"qa_well","type":"resource","name":"Well","description":"A resource."}]}`
	for name, body := range map[string]string{"ruleset.json": manifest, "guide.md": "# QA Sheet Keys\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaSKWantFault(t *testing.T, got *rules.SheetFault, field, key string) {
	t.Helper()
	if got == nil {
		t.Fatalf("SheetKeyFault = nil; want the fault %s %q", field, key)
	}
	if got.Field != field || got.Key != key {
		t.Errorf("SheetKeyFault = {Field:%q Key:%q Reason:%q}; want Field %q, Key %q",
			got.Field, got.Key, got.Reason, field, key)
	}
}

// VTT-617, VTT-618
func TestQASKSheetOfDeclaredKeysHasNoFault(t *testing.T) {
	rs := qaSKRuleset(t)
	if got := rs.SheetKeyFault(
		[]string{"qa_zeta", "qa_alpha", "qa_num"},
		[]string{"qa_well", "qa_pool", "qa_spare"},
	); got != nil {
		t.Errorf("SheetKeyFault = %+v; want nil, every key fits", got)
	}
}

// VTT-617, VTT-618
func TestQASKSheetHoldingNoValueForADeclaredPropertyHasNoFault(t *testing.T) {
	rs := qaSKRuleset(t)
	cases := []struct {
		name                  string
		attributes, resources []string
	}{
		{"nil maps", nil, nil},
		{"empty maps", []string{}, []string{}},
		{"some numbers only", []string{"qa_num"}, nil},
		{"some resources only", nil, []string{"qa_spare"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rs.SheetKeyFault(c.attributes, c.resources); got != nil {
				t.Errorf("SheetKeyFault = %+v; want nil", got)
			}
		})
	}
}

// VTT-617
func TestQASKUndeclaredAttributeIsTheFault(t *testing.T) {
	rs := qaSKRuleset(t)
	qaSKWantFault(t, rs.SheetKeyFault([]string{"qa_alpha", "qa_nope", "qa_zeta"}, []string{"qa_pool"}),
		"attributes", "qa_nope")
}

// VTT-617
func TestQASKUndeclaredResourceIsTheFault(t *testing.T) {
	rs := qaSKRuleset(t)
	qaSKWantFault(t, rs.SheetKeyFault([]string{"qa_num"}, []string{"qa_pool", "qa_nope", "qa_well"}),
		"resources", "qa_nope")
}

// VTT-617
func TestQASKKeyDifferingOnlyInCaseIsUndeclared(t *testing.T) {
	rs := qaSKRuleset(t)
	qaSKWantFault(t, rs.SheetKeyFault([]string{"QA_NUM"}, nil), "attributes", "QA_NUM")
	qaSKWantFault(t, rs.SheetKeyFault(nil, []string{"Qa_pool"}), "resources", "Qa_pool")
}

// VTT-618
func TestQASKAttributeDeclaredAsResourceIsTheFault(t *testing.T) {
	rs := qaSKRuleset(t)
	qaSKWantFault(t, rs.SheetKeyFault([]string{"qa_alpha", "qa_spare", "qa_zeta"}, nil),
		"attributes", "qa_spare")
}

// VTT-618
func TestQASKResourceDeclaredAsNumberIsTheFault(t *testing.T) {
	rs := qaSKRuleset(t)
	qaSKWantFault(t, rs.SheetKeyFault(nil, []string{"qa_pool", "qa_num", "qa_well"}),
		"resources", "qa_num")
}

// VTT-617, VTT-618
func TestQASKAttributesFaultComesBeforeAResourcesFault(t *testing.T) {
	rs := qaSKRuleset(t)
	qaSKWantFault(t, rs.SheetKeyFault([]string{"zz_attr_nope"}, []string{"aa_res_nope"}),
		"attributes", "zz_attr_nope")
	qaSKWantFault(t, rs.SheetKeyFault([]string{"qa_well"}, []string{"qa_alpha"}),
		"attributes", "qa_well")
}

// VTT-617, VTT-618
func TestQASKFirstFaultIsInByteOrderNotInputOrder(t *testing.T) {
	rs := qaSKRuleset(t)
	cases := []struct {
		name                  string
		attributes, resources []string
		field, key            string
	}{
		{"uppercase sorts before lowercase",
			[]string{"qa_b_nope", "qa_alpha", "Qa_c_nope"}, nil, "attributes", "Qa_c_nope"},
		{"underscore sorts before a lowercase letter",
			[]string{"qaa_nope", "qa__nope"}, nil, "attributes", "qa__nope"},
		{"a digit sorts before a letter",
			[]string{"qa_x_nope", "qa_9_nope"}, nil, "attributes", "qa_9_nope"},
		{"a multi-byte rune sorts after ASCII",
			[]string{"é_nope", "z_nope"}, nil, "attributes", "z_nope"},
		{"a declared key sorting first is passed over",
			[]string{"qa_alpha", "qb_nope", "qa_zeta"}, nil, "attributes", "qb_nope"},
		{"attributes: other-type key sorting before an undeclared one",
			[]string{"qb_nope", "qa_pool"}, nil, "attributes", "qa_pool"},
		{"attributes: undeclared key sorting before an other-type one",
			[]string{"qa_pool", "qa_aa_nope"}, nil, "attributes", "qa_aa_nope"},
		{"resources: other-type key sorting before an undeclared one",
			[]string{"qa_num"}, []string{"qb_nope", "qa_pool", "qa_num"}, "resources", "qa_num"},
		{"resources: undeclared key sorting before an other-type one",
			[]string{"qa_num"}, []string{"qa_zeta", "qa_pool", "qa_aa_nope"}, "resources", "qa_aa_nope"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			qaSKWantFault(t, rs.SheetKeyFault(c.attributes, c.resources), c.field, c.key)
		})
	}
}
