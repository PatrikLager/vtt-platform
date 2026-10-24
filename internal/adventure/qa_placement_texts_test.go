package adventure_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

var (
	qaptAdvDir      = filepath.Join("testdata", "qa-pt", "texts")
	qaptRulesetDir  = filepath.Join("testdata", "ruleset")
	qaptGoblinDir   = filepath.Join("..", "..", "adventures", "goblin-ambush")
	qaptCellarDir   = filepath.Join("..", "..", "adventures", "cellar-rats")
	qaptRulesetsDir = filepath.Join("..", "..", "rulesets")
)

type qaptTexts struct{ Name, Notes, GMName, GMNotes string }

func qaptPlacementTexts(p adventure.Placement) qaptTexts {
	return qaptTexts{p.Name, p.Notes, p.GMName, p.GMNotes}
}

func qaptTokenTexts(tp *vttv1.TokenPlaced) qaptTexts {
	return qaptTexts{
		tp.GetName(), tp.GetNotes(), tp.GetGmName(), tp.GetGmNotes(),
	}
}

func qaptRuleset(t *testing.T, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load(%s): %v", dir, err)
	}
	return rs
}

func qaptPlacementsByID(
	t *testing.T, adv *adventure.Adventure,
) map[string]adventure.Placement {
	t.Helper()
	out := map[string]adventure.Placement{}
	for _, sc := range adv.Scenes {
		for _, p := range sc.Placements {
			if _, dup := out[p.TokenID]; dup {
				t.Fatalf("two placements for %q", p.TokenID)
			}
			out[p.TokenID] = p
		}
	}
	return out
}

func qaptTokensByID(
	t *testing.T, envs []*vttv1.Envelope,
) map[string]*vttv1.TokenPlaced {
	t.Helper()
	out := map[string]*vttv1.TokenPlaced{}
	for _, e := range envs {
		tp := e.GetTokenPlaced()
		if tp == nil {
			continue
		}
		if _, dup := out[tp.GetTokenId()]; dup {
			t.Fatalf("two TokenPlaced for %q", tp.GetTokenId())
		}
		out[tp.GetTokenId()] = tp
	}
	return out
}

// qaptLoadCompile loads dir against rs and compiles it into an empty state.
func qaptLoadCompile(
	t *testing.T, dir string, rs *rules.Ruleset,
) (*adventure.Adventure, []*vttv1.Envelope) {
	t.Helper()
	adv, err := adventure.Load(dir, rs)
	if err != nil {
		t.Fatalf("adventure.Load(%s): %v", dir, err)
	}
	envs, warns, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		t.Fatalf("adventure.Compile(%s): %v", dir, err)
	}
	if len(warns) != 0 {
		t.Logf("Compile warnings: %v", warns)
	}
	return adv, envs
}

func qaptCopyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// qaptYardVariant copies the QA adventure and edits the placement tok-brace
// in its scenes/yard.json with edit, returning the copy's directory.
func qaptYardVariant(t *testing.T, edit func(p map[string]any)) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "texts")
	qaptCopyTree(t, qaptAdvDir, dir)
	yard := filepath.Join(dir, "scenes", "yard.json")
	raw, err := os.ReadFile(yard)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	edited := false
	for _, p := range doc["placements"].([]any) {
		pm := p.(map[string]any)
		if pm["token_id"] == "tok-brace" {
			edit(pm)
			edited = true
		}
	}
	if !edited {
		t.Fatal("no tok-brace placement in the yard fixture")
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yard, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// map-format §6: "The same four keys work in an adventure's `scenes/*.json`."
func TestQAPTAdventurePlacementTextsReachTokenPlaced(t *testing.T) {
	adv, envs := qaptLoadCompile(t, qaptAdvDir, qaptRuleset(t, qaptRulesetDir))
	want := map[string]qaptTexts{
		"tok-vim": {},
		"tok-brace": {
			"a guard in grey — gråklädd, 灰色の衛兵",
			"He watches the well.\nHe says \"halt\" twice.",
			"Brace the Paid",
			"Paid by the smugglers; flees at dusk.",
		},
		"tok-brace-2": {
			Name:  "a figure by the hearth",
			Notes: "It warms its hands.",
		},
		"tok-vim-2": {
			GMName:  "Vim at the gate",
			GMNotes: "Carries the key.",
		},
		"tok-scout-2": {},
		"tok-scout":   {},
	}
	placed := qaptPlacementsByID(t, adv)
	tokens := qaptTokensByID(t, envs)
	if len(tokens) != len(want) || len(placed) != len(want) {
		t.Fatalf("%d placements, %d TokenPlaced, want %d of each",
			len(placed), len(tokens), len(want))
	}
	for id, w := range want {
		if got := qaptPlacementTexts(placed[id]); got != w {
			t.Errorf("Placement %q texts = %+v, want %+v", id, got, w)
		}
		tp, ok := tokens[id]
		if !ok {
			t.Fatalf("Compile emitted no TokenPlaced %q", id)
		}
		if got := qaptTokenTexts(tp); got != w {
			t.Errorf("TokenPlaced %q texts = %+v, want %+v", id, got, w)
		}
	}
}

func qaptBig(seed string, n int) string {
	return strings.Repeat(seed, n/len(seed)+1)[:n]
}

// map-format §6: "four optional texts, each a string and none bounded in
// length"; "The same four keys work in an adventure's `scenes/*.json`."
func TestQAPTAdventureTextsHaveNoByteBound(t *testing.T) {
	const n = 100 << 10
	w := qaptTexts{
		qaptBig("name-", n), qaptBig("notes-", n),
		qaptBig("gm-name-", n), qaptBig("gm-notes-", n),
	}
	dir := qaptYardVariant(t, func(p map[string]any) {
		p["name"], p["notes"] = w.Name, w.Notes
		p["gm_name"], p["gm_notes"] = w.GMName, w.GMNotes
	})
	adv, envs := qaptLoadCompile(t, dir, qaptRuleset(t, qaptRulesetDir))
	if got := qaptPlacementTexts(qaptPlacementsByID(t, adv)["tok-brace"]); got != w {
		t.Errorf("Placement texts differ from the 100 KiB texts written")
	}
	tp := qaptTokensByID(t, envs)["tok-brace"]
	if got := qaptTokenTexts(tp); got != w {
		t.Errorf("TokenPlaced texts differ from the 100 KiB texts written")
	}
}

// map-format §0: "Unknown fields are refused."
func TestQAPTAdventureUnknownKeyBesideTextsRefused(t *testing.T) {
	rs := qaptRuleset(t, qaptRulesetDir)
	for _, key := range []string{"gm_note", "gmname", "description"} {
		t.Run(key, func(t *testing.T) {
			dir := qaptYardVariant(t, func(p map[string]any) { p[key] = "x" })
			_, err := adventure.Load(dir, rs)
			if err == nil {
				t.Fatalf("Load accepted unknown placement key %q", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("refusal does not name %q: %v", key, err)
			}
			if !strings.Contains(err.Error(), "yard.json") {
				t.Errorf("refusal does not name yard.json: %v", err)
			}
		})
	}
}

// map-format §6: "four optional texts, each a string"; in a scene too.
func TestQAPTAdventureNonStringTextRefused(t *testing.T) {
	rs := qaptRuleset(t, qaptRulesetDir)
	cases := map[string]any{
		"name":     5,
		"notes":    true,
		"gm_name":  []any{"Brace"},
		"gm_notes": map[string]any{"text": "x"},
	}
	for key, val := range cases {
		t.Run(key, func(t *testing.T) {
			dir := qaptYardVariant(t, func(p map[string]any) { p[key] = val })
			if _, err := adventure.Load(dir, rs); err == nil {
				t.Fatalf("Load accepted %s = %v", key, val)
			}
		})
	}
}

func qaptGoblin(t *testing.T) (*adventure.Adventure, []*vttv1.Envelope) {
	t.Helper()
	rs := qaptRuleset(t, filepath.Join(qaptRulesetsDir, "dnd45e-minimal"))
	return qaptLoadCompile(t, qaptGoblinDir, rs)
}

// goblin-ambush's two creatures carry their texts, and tok-fighter none.
func TestQAPTGoblinAmbushNamesItsTwoCreatures(t *testing.T) {
	adv, envs := qaptGoblin(t)
	want := map[string]qaptTexts{
		"tok-cutter": {Name: "a hunched goblin", GMName: "Goblin Cutter"},
		"tok-archer": {
			Name:   "a goblin with a bow",
			Notes:  "It keeps to the rocks above the trail.",
			GMName: "Goblin Archer",
		},
		"tok-fighter": {},
	}
	placed := qaptPlacementsByID(t, adv)
	tokens := qaptTokensByID(t, envs)
	for id, w := range want {
		p, ok := placed[id]
		if !ok {
			t.Fatalf("goblin-ambush has no placement %q", id)
		}
		if got := qaptPlacementTexts(p); got != w {
			t.Errorf("Placement %q texts = %+v, want %+v", id, got, w)
		}
		tp, ok := tokens[id]
		if !ok {
			t.Fatalf("goblin-ambush compiles no TokenPlaced %q", id)
		}
		if got := qaptTokenTexts(tp); got != w {
			t.Errorf("TokenPlaced %q texts = %+v, want %+v", id, got, w)
		}
	}
}

func qaptActorsByID(envs []*vttv1.Envelope) map[string]*vttv1.Actor {
	out := map[string]*vttv1.Actor{}
	for _, e := range envs {
		if a := e.GetActorAdded().GetActor(); a != nil {
			out[a.GetActorId()] = a
		}
	}
	return out
}

// Each of goblin-ambush's non-party actors is placed with a name a player may
// see and a GM name.
func TestQAPTGoblinAmbushNamesEveryNonPartyPlacement(t *testing.T) {
	_, envs := qaptGoblin(t)
	actors := qaptActorsByID(envs)
	nonParty := 0
	for id, tp := range qaptTokensByID(t, envs) {
		a, ok := actors[tp.GetActorId()]
		if !ok {
			t.Fatalf("TokenPlaced %q names no ActorAdded actor", id)
		}
		if a.GetKind() != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
			continue
		}
		nonParty++
		if tp.GetName() == "" || tp.GetGmName() == "" {
			t.Errorf("non-party %q: name %q, gm_name %q, want both set",
				id, tp.GetName(), tp.GetGmName())
		}
	}
	if nonParty == 0 {
		t.Error("goblin-ambush places no non-party actor, so nothing was checked")
	}
}

// map-format §6: write "a hunched goblin" rather than the creature's own name.
func TestQAPTGoblinAmbushPlayerTextsNameNoActor(t *testing.T) {
	_, envs := qaptGoblin(t)
	names := []string{"Goblin Cutter", "Goblin Archer", "Human Fighter"}
	for _, a := range qaptActorsByID(envs) {
		names = append(names, a.GetName())
	}
	for id, tp := range qaptTokensByID(t, envs) {
		for field, text := range map[string]string{
			"name": tp.GetName(), "notes": tp.GetNotes(),
		} {
			for _, n := range names {
				if strings.Contains(strings.ToLower(text),
					strings.ToLower(n)) {
					t.Errorf("TokenPlaced %q %s %q contains actor name %q",
						id, field, text, n)
				}
			}
		}
	}
}

// cellar-rats places only party members, and its placements carry no text.
func TestQAPTCellarRatsCarriesNoTexts(t *testing.T) {
	rs := qaptRuleset(t, filepath.Join(qaptRulesetsDir, "tavern-brawl"))
	_, envs := qaptLoadCompile(t, qaptCellarDir, rs)
	for id, tp := range qaptTokensByID(t, envs) {
		if got := qaptTokenTexts(tp); got != (qaptTexts{}) {
			t.Errorf("cellar-rats TokenPlaced %q texts = %+v", id, got)
		}
	}
}
