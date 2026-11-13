package rules_test

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func TestSheetKeyFaultNamesTheFirstKeyThatDoesNotFit(t *testing.T) {
	rs := &rules.Ruleset{Properties: []rules.Property{
		{Key: "brace", Type: rules.PropertyNumber},
		{Key: "focus", Type: rules.PropertyResource},
	}}
	cases := []struct {
		name               string
		attributes, pools  []string
		field, key, reason string
	}{
		{name: "declared keys, each in its own type's map, fit",
			attributes: []string{"brace"}, pools: []string{"focus"}},
		{name: "a sheet holding nothing fits"},
		{name: "an undeclared attribute",
			attributes: []string{"brace", "nope"},
			field:      "attributes", key: "nope", reason: `references undeclared property "nope"`},
		{name: "a resource among the attributes",
			attributes: []string{"focus"},
			field:      "attributes", key: "focus", reason: `gives resource property "focus" a number's value`},
		{name: "an undeclared resource",
			pools: []string{"focus", "nope"},
			field: "resources", key: "nope", reason: `references undeclared property "nope"`},
		{name: "a number among the resources",
			pools: []string{"brace"},
			field: "resources", key: "brace", reason: `gives number property "brace" a resource's value`},
		{name: "attributes are read before resources",
			attributes: []string{"zz"}, pools: []string{"aa"},
			field: "attributes", key: "zz", reason: `references undeclared property "zz"`},
		{name: "keys are read in byte order, upper case before lower",
			attributes: []string{"b", "a", "B"},
			field:      "attributes", key: "B", reason: `references undeclared property "B"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := rs.SheetKeyFault(c.attributes, c.pools)
			if c.field == "" {
				if f != nil {
					t.Fatalf("SheetKeyFault = %+v, want nil", *f)
				}
				return
			}
			if f == nil {
				t.Fatalf("SheetKeyFault = nil, want a fault on %s %q", c.field, c.key)
			}
			if f.Field != c.field || f.Key != c.key || f.Reason != c.reason {
				t.Errorf("SheetKeyFault = %+v, want {Field:%s Key:%s Reason:%s}", *f, c.field, c.key, c.reason)
			}
		})
	}
}

// Keep this test: SheetKeyFault sorts its callers' keys, which they collect
// from Go maps in an order that changes from run to run.
func TestSheetKeyFaultIsTheSameWhateverOrderItsKeysArrive(t *testing.T) {
	rs := &rules.Ruleset{Properties: []rules.Property{{Key: "brace", Type: rules.PropertyNumber}}}
	orders := [][]string{{"c", "a", "b"}, {"b", "c", "a"}, {"a", "b", "c"}}
	for _, keys := range orders {
		f := rs.SheetKeyFault(keys, nil)
		if f == nil || f.Key != "a" {
			t.Fatalf("SheetKeyFault(%v) = %+v, want the fault on \"a\"", keys, f)
		}
	}
}

func TestSheetKeyFaultLeavesItsCallersKeysInTheirOrder(t *testing.T) {
	rs := &rules.Ruleset{}
	keys := []string{"c", "a", "b"}
	rs.SheetKeyFault(keys, keys)
	if keys[0] != "c" || keys[1] != "a" || keys[2] != "b" {
		t.Fatalf("keys = %v after SheetKeyFault, want them as passed", keys)
	}
}

// VTT-685 VTT-687 VTT-688
func TestSheetAbilityFaultNamesTheFirstIdTheRulesetDoesNotHold(t *testing.T) {
	rs := &rules.Ruleset{Compiled: map[string]*rules.CompiledPower{"fists": {}, "rally": {}}}
	cases := []struct {
		name string
		ids  []string
		key  string
	}{
		{name: "held ids fit", ids: []string{"rally", "fists"}},
		{name: "a sheet naming no ability fits"},
		{name: "a repeated held id fits", ids: []string{"fists", "fists"}},
		{name: "an id the ruleset does not hold", ids: []string{"fists", "nope"}, key: "nope"},
		{name: "ids are read in byte order, upper case before lower",
			ids: []string{"zz", "b", "B", "fists"}, key: "B"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := rs.SheetAbilityFault(c.ids)
			if c.key == "" {
				if f != nil {
					t.Fatalf("SheetAbilityFault = %+v, want nil", *f)
				}
				return
			}
			want := rules.SheetFault{Field: "ability_ids", Key: c.key,
				Reason: `names ability "` + c.key + `", which the ruleset does not hold`}
			if f == nil || *f != want {
				t.Fatalf("SheetAbilityFault = %+v, want %+v", f, want)
			}
		})
	}
}

// VTT-683
func TestSheetAbilityFaultLeavesItsCallersIdsInTheirOrder(t *testing.T) {
	rs := &rules.Ruleset{}
	ids := []string{"c", "a", "b"}
	rs.SheetAbilityFault(ids)
	if ids[0] != "c" || ids[1] != "a" || ids[2] != "b" {
		t.Fatalf("ids = %v after SheetAbilityFault, want them as passed", ids)
	}
}
