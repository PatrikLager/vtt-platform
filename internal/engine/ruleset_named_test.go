package engine_test

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

func rulesetNamed(seq int64, id, version, name string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: "e", Sequence: seq, SessionId: "s1",
		Payload: &vttv1.Envelope_RulesetNamed{RulesetNamed: &vttv1.RulesetNamed{
			RulesetId: id, Version: version, Name: name}}}
}

// namesTheRuleset fails unless after is before with the ruleset it names
// replaced by id, version and name (sameExcept walks the rest).
func namesTheRuleset(t *testing.T, before, after *engine.State, id, version, name string) {
	t.Helper()
	got := [3]string{after.RulesetID, after.RulesetVersion, after.RulesetName}
	if want := [3]string{id, version, name}; got != want {
		t.Errorf("the state names the ruleset %q, want %q", got, want)
	}
	sameExcept(t, before, after, "")
}

// VTT-979
func TestARulesetNamedNamesItsRulesetAndChangesNothingElse(t *testing.T) {
	st := abilityTable(t)
	before := st.Snapshot()
	must(t, engine.Apply(st, rulesetNamed(9, "qa-rules", "1", "Plain Rules")))
	namesTheRuleset(t, before, st.Snapshot(), "qa-rules", "1", "Plain Rules")
}

// VTT-979
func TestALaterRulesetNamedReplacesTheRulesetTheStateNames(t *testing.T) {
	st := abilityTable(t)
	must(t, engine.Apply(st, rulesetNamed(9, "qa-rules", "1", "Plain Rules")))
	before := st.Snapshot()
	must(t, engine.Apply(st, rulesetNamed(10, "qa-rules", "2", "Plain Rules")))
	namesTheRuleset(t, before, st.Snapshot(), "qa-rules", "2", "Plain Rules")
	must(t, engine.Apply(st, rulesetNamed(11, "other-rules", "2", "Other Rules")))
	namesTheRuleset(t, before, st.Snapshot(), "other-rules", "2", "Other Rules")
}

// VTT-979
func TestASnapshotNamesTheRulesetItsStateNames(t *testing.T) {
	st := engine.NewState()
	must(t, engine.Apply(st, rulesetNamed(1, "qa-rules", "1", "Plain Rules")))
	snap := st.Snapshot()
	if got := [3]string{snap.RulesetID, snap.RulesetVersion, snap.RulesetName}; got != [3]string{"qa-rules", "1", "Plain Rules"} {
		t.Errorf("the snapshot names the ruleset %q, want the state's", got)
	}
}

// VTT-974
func TestARulesetNamedWhoseIDVersionOrNameIsEmptyOrPastItsBoundIsRefusedNamingTheField(t *testing.T) {
	id, version, name := strings.Repeat("é", 64), strings.Repeat("ü", 64), strings.Repeat("é", 128)
	const (
		badID      = "engine: ruleset id must be 1-128 bytes, got "
		badVersion = "engine: ruleset version must be 1-128 bytes, got "
		badName    = "engine: ruleset name must be 1-256 bytes, got "
	)
	for _, c := range []struct{ id, version, name, msg string }{
		{"", version, name, badID + "0"},
		{id + "a", version, name, badID + "129"},
		{id, "", name, badVersion + "0"},
		{id, version + "v", name, badVersion + "129"},
		{id, version, "", badName + "0"},
		{id, version, name + "a", badName + "257"},
		{"", "", "", badID + "0"},
		{id, version + "v", "", badVersion + "129"},
	} {
		st := abilityTable(t)
		must(t, engine.Apply(st, rulesetNamed(9, "qa-rules", "1", "Plain Rules")))
		before := st.Snapshot()
		err := engine.Apply(st, rulesetNamed(10, c.id, c.version, c.name))
		if err == nil || err.Error() != c.msg {
			t.Errorf("Apply(id %d, version %d, name %d bytes) = %v, want %q",
				len(c.id), len(c.version), len(c.name), err, c.msg)
		}
		namesTheRuleset(t, before, st.Snapshot(), "qa-rules", "1", "Plain Rules")
	}
}

// VTT-974
func TestARulesetNamedWhoseIDVersionAndNameAreAtTheirBoundsIsFolded(t *testing.T) {
	id, version, name := strings.Repeat("é", 64), strings.Repeat("ü", 64), strings.Repeat("é", 128)
	st := abilityTable(t)
	before := st.Snapshot()
	must(t, engine.Apply(st, rulesetNamed(9, id, version, name)))
	namesTheRuleset(t, before, st.Snapshot(), id, version, name)
	must(t, engine.Apply(st, rulesetNamed(10, "r", "1", "n")))
	namesTheRuleset(t, before, st.Snapshot(), "r", "1", "n")
}
