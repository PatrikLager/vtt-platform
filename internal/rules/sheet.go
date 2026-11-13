package rules

import (
	"fmt"
	"slices"
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
// hold as an ability, or nil (VTT-685, VTT-687, VTT-688).
func (rs *Ruleset) SheetAbilityFault(ids []string) *SheetFault {
	for _, id := range slices.Sorted(slices.Values(ids)) {
		if _, held := rs.Compiled[id]; !held {
			return &SheetFault{Field: "ability_ids", Key: id, Reason: fmt.Sprintf("names ability %q, which the ruleset does not hold", id)}
		}
	}
	return nil
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
