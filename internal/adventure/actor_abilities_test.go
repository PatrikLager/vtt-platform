package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// writeBraceGuardSheet rewrites testdata/valid's brace-guard.json in a copy of
// the adventure, its attributes and the rest of its sheet given as JSON.
func writeBraceGuardSheet(t *testing.T, attributes, rest string) string {
	t.Helper()
	dir := copyFixtureDirExcluding(t, "testdata/valid")
	path := filepath.Join(dir, "actors", "brace-guard.json")
	body := `{"actor_id": "brace-guard", "name": "Brace Guard", "kind": "non_party",
		"attributes": ` + attributes + `, "resources": {"focus": {"current": 10, "max": 10}}` + rest + `}`
	if !json.Valid([]byte(body)) {
		t.Fatalf("test input is not JSON: %s", body)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const braceGuardFits = `{"vim": 10, "vigor": 10, "brace": 14}`

// abilityRuleset is the fixture ruleset, holding the abilities named.
func abilityRuleset(t *testing.T, ids ...string) *rules.Ruleset {
	t.Helper()
	rs := loadFixtureRuleset(t)
	for _, id := range ids {
		rs.Compiled[id] = &rules.CompiledPower{}
	}
	return rs
}

// compiledAbilityIDs answers the ability ids of each actor Compile adds.
func compiledAbilityIDs(t *testing.T, adv *adventure.Adventure) map[string][]string {
	t.Helper()
	envs, _, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, e := range envs {
		if a := e.GetActorAdded().GetActor(); a != nil {
			out[a.GetActorId()] = a.GetAbilityIds()
		}
	}
	return out
}

// VTT-684
func TestAnActorFilesAbilityIdsReachItsActorAddedInTheirOrder(t *testing.T) {
	rs := abilityRuleset(t, "fists", "rally", "chair-swing")
	adv, err := adventure.Load(writeBraceGuardSheet(t, braceGuardFits,
		`, "ability_ids": ["rally", "fists", "chair-swing", "fists"]`), rs)
	if err != nil {
		t.Fatalf("an actor naming abilities the ruleset holds was refused: %v", err)
	}
	got := compiledAbilityIDs(t, adv)
	if want := []string{"rally", "fists", "chair-swing", "fists"}; !slices.Equal(got["brace-guard"], want) {
		t.Errorf("brace-guard's ActorAdded names %q, want %q", got["brace-guard"], want)
	}
	if len(got["grit-scout"]) != 0 {
		t.Errorf("grit-scout's file names no ability, and its ActorAdded names %q", got["grit-scout"])
	}
}

// VTT-684
func TestAnActorFileWhoseAbilityIdsAreNullNamesNone(t *testing.T) {
	adv, err := adventure.Load(writeBraceGuardSheet(t, braceGuardFits, `, "ability_ids": null`), abilityRuleset(t))
	if err != nil {
		t.Fatalf("an actor whose ability_ids are written null was refused: %v", err)
	}
	if got := compiledAbilityIDs(t, adv)["brace-guard"]; len(got) != 0 {
		t.Errorf("brace-guard's ActorAdded names %q, want none", got)
	}
}

// VTT-687
func TestAnActorFileNamingAnAbilityTheRulesetDoesNotHoldIsRefused(t *testing.T) {
	cases := []struct{ name, ids, key string }{
		{"an id the ruleset does not hold", `["fists", "nope"]`, "nope"},
		{"the first such id in byte order, upper case before lower", `["zz", "b", "B", "fists"]`, "B"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := adventure.Load(writeBraceGuardSheet(t, braceGuardFits, `, "ability_ids": `+c.ids),
				abilityRuleset(t, "fists"))
			if err == nil {
				t.Fatal("an actor naming an ability the ruleset does not hold was loaded")
			}
			for _, want := range []string{"brace-guard.json", `field "ability_ids"`,
				`names ability "` + c.key + `", which the ruleset does not hold`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), want)
				}
			}
		})
	}
}

// VTT-687
func TestAnActorFilesKeysAreReadBeforeItsAbilities(t *testing.T) {
	_, err := adventure.Load(writeBraceGuardSheet(t, `{"vim": 10, "vigor": 10, "nope": 1}`,
		`, "ability_ids": ["missing"]`), abilityRuleset(t))
	if err == nil {
		t.Fatal("an actor holding an undeclared key and naming an unheld ability was loaded")
	}
	if !strings.Contains(err.Error(), `field "attributes"`) || strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %q, want the key's refusal and not the ability's", err.Error())
	}
}
