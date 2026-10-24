package mapdef_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

type qaptTexts struct{ Name, Notes, GMName, GMNotes string }

func qaptPlacementTexts(p mapdef.Placement) qaptTexts {
	return qaptTexts{p.Name, p.Notes, p.GMName, p.GMNotes}
}

func qaptTokenTexts(tp *vttv1.TokenPlaced) qaptTexts {
	return qaptTexts{
		tp.GetName(), tp.GetNotes(), tp.GetGmName(), tp.GetGmNotes(),
	}
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

func qaptPlacementsByID(
	t *testing.T, ps []mapdef.Placement,
) map[string]mapdef.Placement {
	t.Helper()
	out := map[string]mapdef.Placement{}
	for _, p := range ps {
		out[p.TokenID] = p
	}
	return out
}

// qaptLoadCompile loads path and compiles it, returning both halves.
func qaptLoadCompile(
	t *testing.T, path string,
) (*mapdef.Map, map[string]*vttv1.TokenPlaced) {
	t.Helper()
	m, err := mapdef.Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", path, err)
	}
	envs, warns, err := mapdef.Compile(m, t.TempDir())
	if err != nil {
		t.Fatalf("Compile(%s): %v", path, err)
	}
	if len(warns) != 0 {
		t.Logf("Compile warnings: %v", warns)
	}
	tokens := qaptTokensByID(t, envs)
	if len(tokens) != len(m.Placements) {
		t.Fatalf("%d TokenPlaced for %d placements",
			len(tokens), len(m.Placements))
	}
	return m, tokens
}

// qaptVariant writes testdata/qa-pt/texts.json with its first placement
// edited by edit, and returns the new file's path.
func qaptVariant(
	t *testing.T, edit func(p map[string]any),
) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "qa-pt", "texts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	ps := doc["placements"].([]any)
	edit(ps[0].(map[string]any))
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "texts.json")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// map-format §6: "A placement may also carry four optional texts, each a
// string and none bounded in length".
func TestQAPTMapPlacementTextsReachTokenPlaced(t *testing.T) {
	path := filepath.Join("testdata", "qa-pt", "texts.json")
	m, tokens := qaptLoadCompile(t, path)
	want := map[string]qaptTexts{
		"tok-all": {
			"a hunched shape — gråsten, 小鬼",
			"It sleeps against the rock.\nIt mutters \"mine\".",
			"Goblin Cutter",
			"Wakes at any noise.",
		},
		"tok-none": {},
		"tok-player": {
			Name:  "a goblin with a bow",
			Notes: "It keeps to the rocks above the trail.",
		},
		"tok-gm": {
			GMName:  "Chief Rakk",
			GMNotes: "Will parley if outnumbered.",
		},
	}
	placed := qaptPlacementsByID(t, m.Placements)
	for id, w := range want {
		p, ok := placed[id]
		if !ok {
			t.Fatalf("Load kept no placement %q", id)
		}
		if got := qaptPlacementTexts(p); got != w {
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

// map-format §6: "A placement may also carry four optional texts, each a
// string".
func TestQAPTMapExplicitEmptyTextsLoad(t *testing.T) {
	path := filepath.Join("testdata", "qa-pt", "empty-texts.json")
	m, tokens := qaptLoadCompile(t, path)
	if got := qaptPlacementTexts(m.Placements[0]); got != (qaptTexts{}) {
		t.Errorf("Placement texts = %+v, want all empty", got)
	}
	if got := qaptTokenTexts(tokens["tok-empty"]); got != (qaptTexts{}) {
		t.Errorf("TokenPlaced texts = %+v, want all empty", got)
	}
}

func qaptBig(seed string, n int) string {
	return strings.Repeat(seed, n/len(seed)+1)[:n]
}

// map-format §6: "four optional texts, each a string and none bounded in
// length".
func TestQAPTMapTextsHaveNoByteBound(t *testing.T) {
	const n = 100 << 10
	w := qaptTexts{
		qaptBig("name-", n), qaptBig("notes-", n),
		qaptBig("gm-name-", n), qaptBig("gm-notes-", n),
	}
	path := qaptVariant(t, func(p map[string]any) {
		p["name"], p["notes"] = w.Name, w.Notes
		p["gm_name"], p["gm_notes"] = w.GMName, w.GMNotes
	})
	m, tokens := qaptLoadCompile(t, path)
	if got := qaptPlacementTexts(m.Placements[0]); got != w {
		t.Errorf("Placement texts differ from the 100 KiB texts written")
	}
	if got := qaptTokenTexts(tokens["tok-all"]); got != w {
		t.Errorf("TokenPlaced texts differ from the 100 KiB texts written")
	}
}

// SPEC-014: LoadInstalled refuses "a placement whose token id is empty or
// longer than 128 bytes"; map-format §6: texts are "none bounded in length".
func TestQAPTInstalledMapTextsHaveNoByteBound(t *testing.T) {
	big := qaptBig("gm-notes-", 100<<10)
	src := qaptVariant(t, func(p map[string]any) { p["gm_notes"] = big })
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	mapsDir := t.TempDir()
	dst := filepath.Join(mapsDir, "texts.json")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := mapdef.LoadInstalled(mapsDir, "texts", t.TempDir())
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	p := qaptPlacementsByID(t, m.Placements)["tok-all"]
	if p.GMNotes != big {
		t.Errorf("LoadInstalled kept gm_notes of %d bytes, want %d",
			len(p.GMNotes), len(big))
	}
	if p.Name != "a hunched shape — gråsten, 小鬼" {
		t.Errorf("LoadInstalled name = %q", p.Name)
	}
}

// map-format §0: "Unknown fields are refused. The loader rejects any field
// it does not recognise rather than ignoring it".
func TestQAPTMapUnknownKeyBesideTextsRefused(t *testing.T) {
	for _, key := range []string{"gm_note", "gmname", "description"} {
		t.Run(key, func(t *testing.T) {
			path := qaptVariant(t, func(p map[string]any) {
				p[key] = "x"
			})
			_, err := mapdef.Load(path)
			if err == nil {
				t.Fatalf("Load accepted unknown placement key %q", key)
			}
			if !strings.Contains(err.Error(), key) {
				t.Errorf("refusal does not name %q: %v", key, err)
			}
		})
	}
}

// SPEC-014: "No refusal a lookup or a compile produces, and no warning, names
// a path on the server."
func TestQAPTInstalledMapUnknownKeyBesideTextsRefused(t *testing.T) {
	src := qaptVariant(t, func(p map[string]any) { p["gm_note"] = "x" })
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	mapsDir := t.TempDir()
	dst := filepath.Join(mapsDir, "texts.json")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = mapdef.LoadInstalled(mapsDir, "texts", t.TempDir())
	if err == nil {
		t.Fatal("LoadInstalled accepted unknown placement key gm_note")
	}
	msg := err.Error()
	if !strings.Contains(msg, "gm_note") {
		t.Errorf("refusal does not name the key: %v", err)
	}
	if !strings.Contains(msg, "maps/texts.json") {
		t.Errorf("refusal does not name maps/texts.json: %v", err)
	}
	if strings.Contains(msg, mapsDir) {
		t.Errorf("refusal names the server path %s: %v", mapsDir, err)
	}
}

// map-format §6: "four optional texts, each a string".
func TestQAPTMapNonStringTextRefused(t *testing.T) {
	cases := map[string]any{
		"name":     5,
		"notes":    true,
		"gm_name":  []any{"Goblin Cutter"},
		"gm_notes": map[string]any{"text": "x"},
	}
	for key, val := range cases {
		t.Run(key, func(t *testing.T) {
			path := qaptVariant(t, func(p map[string]any) { p[key] = val })
			if _, err := mapdef.Load(path); err == nil {
				t.Fatalf("Load accepted %s = %v", key, val)
			}
		})
	}
}
