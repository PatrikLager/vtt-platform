package rules_test

import (
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// spendRuleset holds swing and haymaker spending stamina, flurry spending
// focus, and jab, which spends nothing; pool is a resource no ability spends.
func spendRuleset() *rules.Ruleset {
	limited := func(resource string) *rules.CompiledPower {
		return &rules.CompiledPower{Usage: rules.Usage{Limited: &rules.LimitedUsage{Resource: resource, Cost: 1}}}
	}
	return &rules.Ruleset{Compiled: map[string]*rules.CompiledPower{
		"swing": limited("stamina"), "haymaker": limited("stamina"), "flurry": limited("focus"),
		"jab": {Usage: rules.Usage{AtWill: true}},
	}}
}

// VTT-775 VTT-776 VTT-777
func TestSheetSpendFaultNamesTheFirstAbilityWithoutItsResourceThenTheFirstOrphan(t *testing.T) {
	cases := []struct {
		name               string
		abilities, pools   []string
		field, key, reason string
	}{
		{name: "a limited ability beside its resource fits", abilities: []string{"swing"}, pools: []string{"stamina"}},
		{name: "two abilities sharing one resource fit", abilities: []string{"swing", "haymaker"}, pools: []string{"stamina"}},
		{name: "a resource no ability spends fits beside an at-will ability", abilities: []string{"jab"}, pools: []string{"pool"}},
		{name: "a resource no ability spends fits with no ability named", pools: []string{"pool"}},
		{name: "a sheet naming and holding nothing fits"},
		{name: "a limited ability without its resource",
			abilities: []string{"jab", "swing"}, pools: []string{"pool"},
			field: "ability_ids", key: "swing", reason: `names ability "swing", which spends resource "stamina" the sheet does not hold`},
		{name: "a resource some ability spends that no named ability spends",
			abilities: []string{"jab"}, pools: []string{"stamina", "pool"},
			field: "resources", key: "stamina", reason: `holds resource "stamina", which no ability the sheet names spends`},
		{name: "abilities are read before resources",
			abilities: []string{"swing"}, pools: []string{"focus"},
			field: "ability_ids", key: "swing", reason: `names ability "swing", which spends resource "stamina" the sheet does not hold`},
		{name: "abilities are read in byte order",
			abilities: []string{"swing", "flurry"},
			field:     "ability_ids", key: "flurry", reason: `names ability "flurry", which spends resource "focus" the sheet does not hold`},
		{name: "resources are read in byte order",
			pools: []string{"stamina", "focus"},
			field: "resources", key: "focus", reason: `holds resource "focus", which no ability the sheet names spends`},
		{name: "an id the ruleset does not hold is read by SheetAbilityFault, not here", abilities: []string{"nope"}},
	}
	rs := spendRuleset()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := rs.SheetSpendFault(c.abilities, c.pools)
			if c.field == "" {
				if f != nil {
					t.Fatalf("SheetSpendFault = %+v, want nil", f)
				}
				return
			}
			if f == nil || f.Field != c.field || f.Key != c.key || f.Reason != c.reason {
				t.Fatalf("SheetSpendFault = %+v, want {%s %s %s}", f, c.field, c.key, c.reason)
			}
		})
	}
}
