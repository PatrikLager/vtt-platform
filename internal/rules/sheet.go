package rules

import (
	"fmt"
	"slices"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// SheetFault names a sheet's first entry that does not fit its ruleset, and why
// (SPEC-025).
type SheetFault struct {
	// Field holds Key: "attributes", "resources" or "ability_ids".
	Field  string
	Key    string
	Reason string
}

// SheetKeyFault answers the first key of attributes, then of resources, each
// in byte order, that rs does not declare as a number or a resource
// respectively, or nil (VTT-617, VTT-618, VTT-677, VTT-678).
func (rs *Ruleset) SheetKeyFault(attributes, resources []string) *SheetFault {
	if f := rs.keyFault("attributes", attributes, PropertyNumber); f != nil {
		return f
	}
	return rs.keyFault("resources", resources, PropertyResource)
}

// SheetAbilityFault answers the first of ids, in byte order, that rs does not
// hold as an ability, or nil (VTT-688, VTT-750, VTT-751).
func (rs *Ruleset) SheetAbilityFault(ids []string) *SheetFault {
	for _, id := range slices.Sorted(slices.Values(ids)) {
		if _, held := rs.Compiled[id]; !held {
			return &SheetFault{Field: "ability_ids", Key: id, Reason: fmt.Sprintf("names ability %q, which the ruleset does not hold", id)}
		}
	}
	return nil
}

// SheetSpendFault answers the first of abilityIDs, in byte order, naming a
// limited ability whose resource is not among resources, then the first of
// resources some ability of rs spends and none of abilityIDs does, or nil
// (VTT-775, VTT-776, VTT-777).
func (rs *Ruleset) SheetSpendFault(abilityIDs, resources []string) *SheetFault {
	spent := map[string]bool{}
	for _, id := range slices.Sorted(slices.Values(abilityIDs)) {
		r, limited := rs.Spends(id)
		if !limited {
			continue
		}
		spent[r] = true
		if !slices.Contains(resources, r) {
			return &SheetFault{Field: "ability_ids", Key: id,
				Reason: fmt.Sprintf("names ability %q, which spends resource %q the sheet does not hold", id, r)}
		}
	}
	for _, r := range slices.Sorted(slices.Values(resources)) {
		if !spent[r] && rs.anySpends(r) {
			return &SheetFault{Field: "resources", Key: r,
				Reason: fmt.Sprintf("holds resource %q, which no ability the sheet names spends", r)}
		}
	}
	return nil
}

func (rs *Ruleset) anySpends(resource string) bool {
	for id := range rs.Compiled {
		if r, ok := rs.Spends(id); ok && r == resource {
			return true
		}
	}
	return false
}

// Spends answers the resource rs's limited ability abilityID spends (VTT-767).
func (rs *Ruleset) Spends(abilityID string) (string, bool) {
	c, held := rs.Compiled[abilityID]
	if !held || c.Usage.Limited == nil {
		return "", false
	}
	return c.Usage.Limited.Resource, true
}

// Offers reports whether a's sheet names abilityID (VTT-690).
func Offers(a *vttv1.Actor, abilityID string) bool {
	return slices.Contains(a.GetAbilityIds(), abilityID)
}

func (rs *Ruleset) keyFault(field string, keys []string, want string) *SheetFault {
	for _, key := range slices.Sorted(slices.Values(keys)) {
		p := rs.Property(key)
		if p == nil {
			return &SheetFault{Field: field, Key: key, Reason: fmt.Sprintf("references undeclared property %q", key)}
		}
		if p.Type != want {
			return &SheetFault{Field: field, Key: key, Reason: fmt.Sprintf("gives %s property %q a %s's value", p.Type, key, want)}
		}
	}
	return nil
}
