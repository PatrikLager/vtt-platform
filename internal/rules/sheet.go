package rules

import (
	"fmt"
	"slices"
)

// SheetFault is the first key of a sheet that its ruleset does not declare as
// a property of the type of the map that holds it (SPEC-024).
type SheetFault struct {
	// Field is the map that holds the key: "attributes" or "resources".
	Field  string
	Key    string
	Reason string
}

// SheetKeyFault answers the first key of attributes, then of resources, each
// in byte order, that rs does not declare as a number or a resource
// respectively, or nil when every key fits (VTT-617, VTT-618, VTT-677,
// VTT-678).
func (rs *Ruleset) SheetKeyFault(attributes, resources []string) *SheetFault {
	if f := rs.keyFault("attributes", attributes, PropertyNumber); f != nil {
		return f
	}
	return rs.keyFault("resources", resources, PropertyResource)
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
