package rules

import (
	"fmt"
	"maps"
	"slices"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// SheetFault names a sheet's first entry that does not fit its ruleset, and why
// (SPEC-025, VTT-1010).
type SheetFault struct {
	// Field holds Key: "attributes", "resources", "ability_ids" or "references".
	Field  string
	Key    string
	Reason string
	// Form is the form that lists Key, for a key a form lists (VTT-1010).
	Form string
}

// SheetKeyFault answers the first key of attributes, then of resources, each
// in byte order, that rs does not declare as a number or a resource
// respectively, then the first of attributes, in byte order, that a form
// lists, or nil (VTT-617, VTT-618, VTT-677, VTT-1010, VTT-1062 to VTT-1065).
func (rs *Ruleset) SheetKeyFault(attributes, resources []string) *SheetFault {
	if f := rs.keyFault("attributes", attributes, PropertyNumber); f != nil {
		return f
	}
	if f := rs.keyFault("resources", resources, PropertyResource); f != nil {
		return f
	}
	return rs.formKeyFault(attributes)
}

func (rs *Ruleset) formKeyFault(attributes []string) *SheetFault {
	for _, key := range slices.Sorted(slices.Values(attributes)) {
		for _, form := range rs.Forms {
			if slices.Contains(form.Properties, key) {
				return &SheetFault{Field: "attributes", Key: key, Form: form.Key,
					Reason: fmt.Sprintf("holds property %q, which form %q lists for its entries", key, form.Key)}
			}
		}
	}
	return nil
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

// ReferenceFault answers the first way r, the reference under id, does not fit
// rs: an entry its catalogue does not hold, a place it does not declare, an
// entry of a form the place does not hold, then no state where the place names
// some, a state where it names none, or a state it does not name; or nil
// (VTT-1037).
func (rs *Ruleset) ReferenceFault(id string, r *vttv1.Reference) *SheetFault {
	entry, held := rs.Catalogue[r.GetEntryId()]
	if !held {
		return referenceFault(id, fmt.Sprintf("names entry %q, which the catalogue does not hold", r.GetEntryId()))
	}
	p := rs.Place(r.GetPlace())
	if p == nil {
		return referenceFault(id, fmt.Sprintf("is in place %q, which the ruleset does not declare", r.GetPlace()))
	}
	if !slices.Contains(p.Forms, entry.Form) {
		return referenceFault(id, fmt.Sprintf("names entry %q of form %q, which place %q does not hold", entry.ID, entry.Form, p.Key))
	}
	state := r.GetState()
	if state == "" && len(p.States) > 0 {
		return referenceFault(id, fmt.Sprintf("names no state, and place %q names its states", p.Key))
	}
	if state != "" && len(p.States) == 0 {
		return referenceFault(id, fmt.Sprintf("is in state %q, and place %q names none", state, p.Key))
	}
	if state != "" && !slices.Contains(p.States, state) {
		return referenceFault(id, fmt.Sprintf("is in state %q, which place %q does not name", state, p.Key))
	}
	return nil
}

// SheetReferenceFault answers the first of refs, in byte order of their ids,
// that ReferenceFault refuses, then the first naming an entry an earlier one
// names in the same place, when that place holds an entry once, or nil
// (VTT-1037, VTT-1038).
func (rs *Ruleset) SheetReferenceFault(refs map[string]*vttv1.Reference) *SheetFault {
	ids := slices.Sorted(maps.Keys(refs))
	for _, id := range ids {
		if f := rs.ReferenceFault(id, refs[id]); f != nil {
			return f
		}
	}
	return rs.copiesFault(ids, refs)
}

// Pass only references that fit, ids in byte order: Place of an undeclared place
// is nil here, and the first id holding an entry is the one kept.
func (rs *Ruleset) copiesFault(ids []string, refs map[string]*vttv1.Reference) *SheetFault {
	type at struct{ place, entry string }
	// Keep the place in the key: one entry may be held once in each of two
	// places (VTT-1038).
	first := map[at]string{}
	for _, id := range ids {
		r := refs[id]
		k := at{r.GetPlace(), r.GetEntryId()}
		earlier, seen := first[k]
		if !seen {
			first[k] = id
			continue
		}
		if !rs.Place(k.place).Copies {
			return referenceFault(id, fmt.Sprintf("names entry %q, which reference %q holds in place %q already, "+
				"and that place holds an entry once", k.entry, earlier, k.place))
		}
	}
	return nil
}

func referenceFault(id, reason string) *SheetFault {
	return &SheetFault{Field: "references", Key: id, Reason: fmt.Sprintf("reference %q %s", id, reason)}
}

func (rs *Ruleset) anySpends(resource string) bool {
	for id := range rs.Compiled {
		if r, ok := rs.Spends(id); ok && r == resource {
			return true
		}
	}
	return false
}

// Spends answers the resource rs's limited ability abilityID spends (VTT-991).
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
