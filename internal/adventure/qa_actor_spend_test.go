package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaSFTavernDir = "../../rulesets/tavern-brawl"

func qaSFCopyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o600)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

func qaSFLoadRuleset(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load ruleset %s: %v", dir, err)
	}
	return rs
}

// qaSFVariant is tavern-brawl (same id) with a resource "anger" declared
// after stamina, "rage-punch" spending anger and "bottle-smash" spending
// stamina beside chair-swing. drink is still spent by no ability.
func qaSFVariant(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	qaSFCopyDir(t, qaSFTavernDir, dir)
	manifest := filepath.Join(dir, "ruleset.json")
	var doc map[string]any
	qaSFReadJSON(t, manifest, &doc)
	props, ok := doc["properties"].([]any)
	if !ok {
		t.Fatalf("ruleset.json properties: %T", doc["properties"])
	}
	doc["properties"] = append(props, map[string]any{
		"key": "anger", "type": "resource", "name": "Anger",
		"description": "QA fixture resource spent by rage-punch.", "thresholds": []any{},
	})
	qaSFWriteJSON(t, manifest, doc)
	for _, a := range []struct{ id, resource string }{{"rage-punch", "anger"}, {"bottle-smash", "stamina"}} {
		var ab map[string]any
		qaSFReadJSON(t, filepath.Join(dir, "abilities", "chair-swing.json"), &ab)
		ab["id"] = a.id
		ab["name"] = a.id
		ab["usage"] = map[string]any{"limited": map[string]any{"resource": a.resource, "cost": 1}}
		qaSFWriteJSON(t, filepath.Join(dir, "abilities", a.id+".json"), ab)
	}
	return qaSFLoadRuleset(t, dir)
}

func qaSFReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func qaSFWriteJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// qaSFCellar copies adventures/cellar-rats (tavern-brawl; hollis and mara
// each name chair-swing and hold drink and stamina) into a temp dir and
// returns the dir.
func qaSFCellar(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	qaSFCopyDir(t, "../../adventures/cellar-rats", dir)
	return dir
}

// qaSFEditHollis rewrites hollis.json's ability_ids and resources; a nil
// resources map removes the field. It returns the file's path.
func qaSFEditHollis(t *testing.T, dir string, abilities []string, resources map[string]any) string {
	t.Helper()
	path := filepath.Join(dir, "actors", "hollis.json")
	var actor map[string]any
	qaSFReadJSON(t, path, &actor)
	actor["ability_ids"] = abilities
	if resources == nil {
		delete(actor, "resources")
	} else {
		actor["resources"] = resources
	}
	qaSFWriteJSON(t, path, actor)
	return path
}

func qaSFRes(current, maximum int) map[string]any {
	return map[string]any{"current": current, "max": maximum}
}

// qaSFWantSpendRefusal asserts err is `adventure: <path>: field "<field>":
// <reason>`, carrying the fault Ruleset.SheetSpendFault answers for the same
// sheet, and that the fault is the one expected (VTT-776).
func qaSFWantSpendRefusal(t *testing.T, err error, rs *rules.Ruleset, path string, abilities, resources []string, field, key string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load accepted the adventure; want field %q key %q refused", field, key)
	}
	f := rs.SheetSpendFault(abilities, resources)
	if f == nil {
		t.Fatalf("SheetSpendFault(%q, %q) = nil; the fixture does not fault", abilities, resources)
	}
	if f.Field != field || f.Key != key {
		t.Fatalf("fixture fault is field %q key %q, want %q %q", f.Field, f.Key, field, key)
	}
	want := "adventure: " + path + `: field "` + field + `": ` + f.Reason
	if err.Error() != want {
		t.Fatalf("Load error\n got: %s\nwant: %s", err.Error(), want)
	}
	if !strings.Contains(err.Error(), key) {
		t.Fatalf("error %q does not name the key %q", err, key)
	}
}

// TestQASFCellarRatsAndGoblinAmbushStillLoad: the repository's adventures
// still load (VTT-802). It is also the control for every refusal below:
// the unedited copy loads.
func TestQASFCellarRatsAndGoblinAmbushStillLoad(t *testing.T) {
	if _, err := adventure.Load(qaSFCellar(t), qaSFLoadRuleset(t, qaSFTavernDir)); err != nil {
		t.Fatalf("cellar-rats: %v", err)
	}
	if _, err := adventure.Load("../../adventures/goblin-ambush", qaSFLoadRuleset(t, "../../rulesets/dnd45e-minimal")); err != nil {
		t.Fatalf("goblin-ambush: %v", err)
	}
}

// TestQASFLoadRefusesALimitedAbilityWithoutItsResource pins VTT-776's first
// half: an actor file naming a limited ability whose resource it does not
// hold, by an error naming the file, the field and the key.
func TestQASFLoadRefusesALimitedAbilityWithoutItsResource(t *testing.T) {
	rs := qaSFLoadRuleset(t, qaSFTavernDir)
	dir := qaSFCellar(t)
	abilities := []string{"chair-swing", "fists", "sober-up", "splash-of-water"}
	path := qaSFEditHollis(t, dir, abilities, map[string]any{"drink": qaSFRes(0, 5)})
	_, err := adventure.Load(dir, rs)
	qaSFWantSpendRefusal(t, err, rs, path, abilities, []string{"drink"}, "ability_ids", "chair-swing")
	if !strings.Contains(err.Error(), "stamina") {
		t.Errorf("error %q does not name the resource the ability spends", err)
	}
}

// TestQASFLoadRefusesASpentResourceNoNamedAbilitySpends pins VTT-776's second
// half: an actor file holding a resource some ability of the ruleset spends
// while no ability it names spends it.
func TestQASFLoadRefusesASpentResourceNoNamedAbilitySpends(t *testing.T) {
	rs := qaSFLoadRuleset(t, qaSFTavernDir)
	t.Run("at-will abilities named", func(t *testing.T) {
		dir := qaSFCellar(t)
		abilities := []string{"fists", "sober-up", "splash-of-water"}
		path := qaSFEditHollis(t, dir, abilities, map[string]any{"drink": qaSFRes(0, 5), "stamina": qaSFRes(2, 2)})
		_, err := adventure.Load(dir, rs)
		qaSFWantSpendRefusal(t, err, rs, path, abilities, []string{"drink", "stamina"}, "resources", "stamina")
	})
	t.Run("no ability named", func(t *testing.T) {
		dir := qaSFCellar(t)
		path := qaSFEditHollis(t, dir, nil, map[string]any{"stamina": qaSFRes(2, 2)})
		_, err := adventure.Load(dir, rs)
		qaSFWantSpendRefusal(t, err, rs, path, nil, []string{"stamina"}, "resources", "stamina")
	})
}

// TestQASFLoadAcceptsAResourceNoAbilitySpends pins VTT-776's reach: the
// rule reaches only a resource some ability of the ruleset spends; drink,
// which none spends, loads with or without abilities (VTT-776).
func TestQASFLoadAcceptsAResourceNoAbilitySpends(t *testing.T) {
	rs := qaSFLoadRuleset(t, qaSFTavernDir)
	for _, abilities := range [][]string{nil, {"fists", "sober-up", "splash-of-water"}} {
		dir := qaSFCellar(t)
		qaSFEditHollis(t, dir, abilities, map[string]any{"drink": qaSFRes(0, 5)})
		if _, err := adventure.Load(dir, rs); err != nil {
			t.Errorf("abilities %q with drink only: %v", abilities, err)
		}
	}
	dir := qaSFCellar(t)
	qaSFEditHollis(t, dir, []string{"fists"}, nil)
	if _, err := adventure.Load(dir, rs); err != nil {
		t.Errorf("an at-will ability and no resources: %v", err)
	}
}

// TestQASFLoadOrderAndSharedResource pins Ruleset.SheetSpendFault's order
// at Load: ability ids before resources, each in byte order; and a shared
// resource fitting while any spender is named (VTT-776).
func TestQASFLoadOrderAndSharedResource(t *testing.T) {
	rs := qaSFVariant(t)
	t.Run("ability ids in byte order", func(t *testing.T) {
		dir := qaSFCellar(t)
		abilities := []string{"rage-punch", "fists", "chair-swing"}
		path := qaSFEditHollis(t, dir, abilities, map[string]any{"drink": qaSFRes(0, 5)})
		_, err := adventure.Load(dir, rs)
		qaSFWantSpendRefusal(t, err, rs, path, abilities, []string{"drink"}, "ability_ids", "chair-swing")
	})
	t.Run("resources in byte order", func(t *testing.T) {
		dir := qaSFCellar(t)
		abilities := []string{"fists"}
		path := qaSFEditHollis(t, dir, abilities, map[string]any{"stamina": qaSFRes(2, 2), "anger": qaSFRes(1, 1)})
		_, err := adventure.Load(dir, rs)
		qaSFWantSpendRefusal(t, err, rs, path, abilities, []string{"anger", "stamina"}, "resources", "anger")
	})
	t.Run("abilities before resources", func(t *testing.T) {
		dir := qaSFCellar(t)
		abilities := []string{"chair-swing"}
		path := qaSFEditHollis(t, dir, abilities, map[string]any{"anger": qaSFRes(1, 1)})
		_, err := adventure.Load(dir, rs)
		qaSFWantSpendRefusal(t, err, rs, path, abilities, []string{"anger"}, "ability_ids", "chair-swing")
	})
	t.Run("shared resource with the other spender named loads", func(t *testing.T) {
		dir := qaSFCellar(t)
		qaSFEditHollis(t, dir, []string{"bottle-smash", "fists"}, map[string]any{"drink": qaSFRes(0, 5), "stamina": qaSFRes(2, 2)})
		if _, err := adventure.Load(dir, rs); err != nil {
			t.Fatalf("bottle-smash with stamina: %v", err)
		}
	})
	t.Run("shared resource with no spender named", func(t *testing.T) {
		dir := qaSFCellar(t)
		abilities := []string{"fists"}
		path := qaSFEditHollis(t, dir, abilities, map[string]any{"stamina": qaSFRes(2, 2)})
		_, err := adventure.Load(dir, rs)
		qaSFWantSpendRefusal(t, err, rs, path, abilities, []string{"stamina"}, "resources", "stamina")
	})
}

// TestQASFLoadEarlierRefusalsComeFirst pins VTT-776's "after its ability-id
// refusals" and VTT-751's "after its refusals of an actor's id, name, kind,
// keys and values": a file that also breaks this rule is refused for the
// earlier fault.
func TestQASFLoadEarlierRefusalsComeFirst(t *testing.T) {
	rs := qaSFLoadRuleset(t, qaSFTavernDir)
	cases := []struct {
		name      string
		abilities []string
		resources map[string]any
		want      []string
	}{
		{
			name:      "an id the ruleset does not hold (VTT-751)",
			abilities: []string{"chair-swing", "no-such-ability"},
			resources: map[string]any{"drink": qaSFRes(0, 5)},
			want:      []string{`field "ability_ids": `, `names ability "no-such-ability", which the ruleset does not hold`},
		},
		{
			name:      "an empty id (VTT-748)",
			abilities: []string{"", "chair-swing"},
			resources: map[string]any{"drink": qaSFRes(0, 5)},
			want:      []string{`field "ability_ids": must not hold an empty id`},
		},
		{
			name:      "an undeclared resource key",
			abilities: []string{"chair-swing"},
			resources: map[string]any{"zest": qaSFRes(1, 1)},
			want:      []string{`field "resources": `, `references undeclared property "zest"`},
		},
		{
			name:      "a resource with no current",
			abilities: []string{"chair-swing"},
			resources: map[string]any{"drink": map[string]any{"max": 5}},
			want:      []string{`resources.drink.current`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := qaSFCellar(t)
			path := qaSFEditHollis(t, dir, c.abilities, c.resources)
			_, err := adventure.Load(dir, rs)
			if err == nil {
				t.Fatal("Load accepted the adventure")
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error %q does not name the file", err)
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not contain %q", err, w)
				}
			}
			keys := make([]string, 0, len(c.resources))
			for k := range c.resources {
				keys = append(keys, k)
			}
			if f := rs.SheetSpendFault(c.abilities, keys); f == nil {
				t.Fatalf("fixture %q %q does not also break the spend rule", c.abilities, keys)
			} else if strings.Contains(err.Error(), f.Reason) {
				t.Errorf("error %q is the spend refusal, not the earlier one", err)
			}
		})
	}
}
