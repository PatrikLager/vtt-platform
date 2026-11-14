package adventure_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaeaAdventure = "../../adventures/goblin-ambush"
	qaeaRuleset   = "../../rulesets/dnd45e-minimal"
	qaeaActorFile = "act-archer.json"
)

func qaeaRules(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(qaeaRuleset)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

// qaeaCopy copies the committed adventure into a temp dir, lets edit change
// the archer's actor file as a JSON object, and returns the dir.
func qaeaCopy(t *testing.T, edit func(actor map[string]any)) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(qaeaAdventure, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(qaeaAdventure, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatalf("copy adventure: %v", err)
	}
	if edit != nil {
		p := filepath.Join(dst, "actors", qaeaActorFile)
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read actor: %v", err)
		}
		var actor map[string]any
		if err := json.Unmarshal(raw, &actor); err != nil {
			t.Fatalf("decode actor: %v", err)
		}
		edit(actor)
		out, err := json.MarshalIndent(actor, "", "  ")
		if err != nil {
			t.Fatalf("encode actor: %v", err)
		}
		if err := os.WriteFile(p, out, 0o644); err != nil {
			t.Fatalf("write actor: %v", err)
		}
	}
	return dst
}

func qaeaLoadErr(t *testing.T, dir string, rs *rules.Ruleset) error {
	t.Helper()
	adv, err := adventure.Load(dir, rs)
	if err == nil && adv == nil {
		t.Fatalf("Load returned neither an adventure nor an error")
	}
	if err != nil && adv != nil {
		t.Fatalf("Load returned an adventure beside its error %v", err)
	}
	return err
}

func qaeaAbilities(ids ...string) func(map[string]any) {
	return func(a map[string]any) {
		list := make([]any, len(ids))
		for i, id := range ids {
			list[i] = id
		}
		a["ability_ids"] = list
	}
}

// VTT-748: the fixture loads as committed and with a list holding no empty id.
func TestQAEAFixtureLoadsWithNoEmptyId(t *testing.T) {
	rs := qaeaRules(t)
	for name, edit := range map[string]func(map[string]any){
		"as committed": nil,
		"held ids":     qaeaAbilities("goblin-scimitar", "rally", "goblin-scimitar"),
		"empty list":   qaeaAbilities(),
		"null list":    func(a map[string]any) { a["ability_ids"] = nil },
	} {
		if err := qaeaLoadErr(t, qaeaCopy(t, edit), rs); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// VTT-748
func TestQAEAAnActorFileWhoseAbilityIdsHoldAnEmptyIdIsRefused(t *testing.T) {
	rs := qaeaRules(t)
	for name, ids := range map[string][]string{
		"only empty":  {""},
		"empty last":  {"goblin-scimitar", ""},
		"empty first": {"", "goblin-shortbow"},
		"empty twice": {"", "rally", ""},
		"empty amid":  {"goblin-scimitar", "", "goblin-shortbow"},
	} {
		err := qaeaLoadErr(t, qaeaCopy(t, qaeaAbilities(ids...)), rs)
		if err == nil {
			t.Fatalf("%s: %q loaded", name, ids)
		}
		msg := err.Error()
		t.Logf("%s: %s", name, msg)
		for _, want := range []string{qaeaActorFile, "ability_ids", "empty"} {
			if !strings.Contains(msg, want) {
				t.Fatalf("%s: refusal %q does not contain %q", name, msg, want)
			}
		}
		if strings.Contains(msg, "does not hold") {
			t.Fatalf("%s: refusal %q reads the empty id against the ruleset", name, msg)
		}
	}
}

// VTT-748: before its ability ids are read against the ruleset.
func TestQAEAAnEmptyIdIsRefusedBeforeTheIdsAreReadAgainstTheRuleset(t *testing.T) {
	rs := qaeaRules(t)
	err := qaeaLoadErr(t, qaeaCopy(t, qaeaAbilities("zzz-not-held", "")), rs)
	if err == nil {
		t.Fatalf("loaded")
	}
	msg := err.Error()
	if !strings.Contains(msg, "empty") || strings.Contains(msg, "zzz-not-held") || strings.Contains(msg, "does not hold") {
		t.Fatalf("refusal %q is not the empty id's", msg)
	}
	ctl := qaeaLoadErr(t, qaeaCopy(t, qaeaAbilities("zzz-not-held")), rs)
	if ctl == nil || !strings.Contains(ctl.Error(), "zzz-not-held") {
		t.Fatalf("control: an id the ruleset does not hold gave %v", ctl)
	}
}

// VTT-748: after its refusals of an actor's id, name, kind, keys and values.
func TestQAEAAnEmptyIdIsRefusedAfterTheActorsEarlierRefusals(t *testing.T) {
	rs := qaeaRules(t)
	earlier := map[string]func(map[string]any){
		"id missing":       func(a map[string]any) { delete(a, "actor_id") },
		"id empty":         func(a map[string]any) { a["actor_id"] = "" },
		"name too long":    func(a map[string]any) { a["name"] = strings.Repeat("n", 300) },
		"kind missing":     func(a map[string]any) { delete(a, "kind") },
		"kind unknown":     func(a map[string]any) { a["kind"] = "qa-no-such-kind" },
		"undeclared key":   func(a map[string]any) { a["attributes"].(map[string]any)["qa-undeclared-key"] = 1 },
		"key in other map": func(a map[string]any) { a["resources"].(map[string]any)["dex"] = map[string]any{"current": 1} },
		"null value":       func(a map[string]any) { a["attributes"].(map[string]any)["dex"] = nil },
	}
	for name, edit := range earlier {
		aloneDir := qaeaCopy(t, edit)
		alone := qaeaLoadErr(t, aloneDir, rs)
		if alone == nil {
			t.Fatalf("%s: control loaded, the fault is not a refusal", name)
		}
		bothDir := qaeaCopy(t, func(a map[string]any) {
			edit(a)
			qaeaAbilities("goblin-scimitar", "")(a)
		})
		both := qaeaLoadErr(t, bothDir, rs)
		if both == nil {
			t.Fatalf("%s with an empty id: loaded", name)
		}
		a := strings.ReplaceAll(alone.Error(), aloneDir, "<dir>")
		b := strings.ReplaceAll(both.Error(), bothDir, "<dir>")
		t.Logf("%s: %s", name, b)
		if a != b {
			t.Fatalf("%s: with an empty id refused as %q, without as %q", name, b, a)
		}
	}
}
