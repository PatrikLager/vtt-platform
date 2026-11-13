package conformance_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaDataField struct {
	key string
	raw json.RawMessage
}

func qaDataFields(raw []byte) ([]qaDataField, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("not an object: %v", tok)
	}
	var out []qaDataField
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k, ok := kt.(string)
		if !ok {
			return nil, fmt.Errorf("key %v is not a string", kt)
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, qaDataField{k, v})
	}
	return out, nil
}

func qaDataJoin(fields []qaDataField) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(f.key)
		b.Write(k)
		b.WriteByte(':')
		b.Write(f.raw)
	}
	b.WriteByte('}')
	return b.Bytes()
}

// qaDataEdit rewrites the value at path, keeping every key's order; fn
// returning false deletes the key.
func qaDataEdit(raw []byte, path []string, fn func(json.RawMessage) (json.RawMessage, bool)) ([]byte, error) {
	fields, err := qaDataFields(raw)
	if err != nil {
		return nil, err
	}
	var out []qaDataField
	found := false
	for _, f := range fields {
		if f.key != path[0] {
			out = append(out, f)
			continue
		}
		found = true
		if len(path) == 1 {
			if v, keep := fn(f.raw); keep {
				out = append(out, qaDataField{f.key, v})
			}
			continue
		}
		v, err := qaDataEdit(f.raw, path[1:], fn)
		if err != nil {
			return nil, err
		}
		out = append(out, qaDataField{f.key, v})
	}
	if !found {
		return nil, fmt.Errorf("no key %q", path[0])
	}
	return qaDataJoin(out), nil
}

func qaDataIDs(ids ...string) json.RawMessage {
	b, _ := json.Marshal(ids)
	return b
}

func qaDataRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repository root not found at %s: %v", root, err)
	}
	return root
}

func qaDataCopyTree(t *testing.T, src, dst string) {
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
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// qaDataTempRoot is a throwaway copy of the data directories, for injecting faults.
func qaDataTempRoot(t *testing.T) string {
	t.Helper()
	root := qaDataRoot(t)
	tmp := t.TempDir()
	for _, d := range []string{"adventures", "rulesets", "scenarios"} {
		qaDataCopyTree(t, filepath.Join(root, d), filepath.Join(tmp, d))
	}
	return tmp
}

func qaDataRewrite(t *testing.T, path string, fn func([]byte) ([]byte, error)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out, err := fn(b)
	if err != nil {
		t.Fatalf("rewrite %s: %v", path, err)
	}
	if bytes.Equal(out, b) {
		t.Fatalf("rewrite %s changed nothing", path)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

type qaDataActorFile struct {
	ActorID    string   `json:"actor_id"`
	AbilityIDs []string `json:"ability_ids"`
}

func qaDataReadActors(dir string) (map[string][]string, []string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "actors", "*.json"))
	if err != nil {
		return nil, nil, err
	}
	out := map[string][]string{}
	var order []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		var a qaDataActorFile
		if err := json.Unmarshal(b, &a); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", f, err)
		}
		out[a.ActorID] = a.AbilityIDs
		order = append(order, a.ActorID)
	}
	return out, order, nil
}

func qaDataAdventureRuleset(dir string) (string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "adventure.json"))
	if err != nil {
		return "", err
	}
	var head struct {
		Ruleset string `json:"ruleset"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return "", err
	}
	return head.Ruleset, nil
}

func qaDataAdventureProblems(root string) []string {
	heads, _ := filepath.Glob(filepath.Join(root, "adventures", "*", "adventure.json"))
	if len(heads) == 0 {
		return []string{"no adventure under " + root}
	}
	var probs []string
	for _, h := range heads {
		dir := filepath.Dir(h)
		place := "adventures/" + filepath.Base(dir)
		rsID, err := qaDataAdventureRuleset(dir)
		if err != nil {
			probs = append(probs, place+": "+err.Error())
			continue
		}
		rs, err := rules.Load(filepath.Join(root, "rulesets", rsID))
		if err != nil {
			probs = append(probs, place+": ruleset: "+err.Error())
			continue
		}
		got, order, err := qaDataReadActors(dir)
		if err != nil {
			probs = append(probs, place+": "+err.Error())
			continue
		}
		for _, id := range order {
			for _, ab := range got[id] {
				if rs.Compiled[ab] == nil {
					probs = append(probs, fmt.Sprintf("%s: actor %s names %q, which ruleset %s does not hold", place, id, ab, rsID))
				}
			}
		}
		adv, err := adventure.Load(dir, rs)
		if err != nil {
			probs = append(probs, place+": adventure.Load: "+err.Error())
			continue
		}
		for _, a := range adv.Actors {
			if !slices.Equal(a.AbilityIDs, got[a.ID]) {
				probs = append(probs, fmt.Sprintf("%s: loaded actor %s holds %q, its file names %q", place, a.ID, a.AbilityIDs, got[a.ID]))
			}
		}
	}
	return probs
}

type qaDataStep struct {
	By      string                     `json:"by"`
	Command map[string]json.RawMessage `json:"command"`
	Expect  *struct {
		OK               bool   `json:"ok"`
		DeniedContaining string `json:"deniedContaining"`
	} `json:"expect"`
}

type qaDataUse struct {
	ActorID   string `json:"actorId"`
	AbilityID string `json:"abilityId"`
}

func qaDataCountKey(v any, key string) int {
	n := 0
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			if k == key {
				n++
			}
			n += qaDataCountKey(c, key)
		}
	case []any:
		for _, c := range x {
			n += qaDataCountKey(c, key)
		}
	}
	return n
}

// qaDataSheetRefusal says whether an expected refusal is the caster's sheet
// not naming the ability, the one refusal the use check exempts.
func qaDataSheetRefusal(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "sheet") || strings.Contains(l, "does not name") || strings.Contains(l, "names no")
}

// qaDataScenarioProblems returns the scenario and use checks' problems, and one line
// per useAbility saying how it was classified.
func qaDataScenarioProblems(root string) (probs, uses []string) {
	files, _ := filepath.Glob(filepath.Join(root, "scenarios", "*.json"))
	if len(files) == 0 {
		return []string{"no scenario under " + root}, nil
	}
	for _, f := range files {
		place := "scenarios/" + filepath.Base(f)
		b, err := os.ReadFile(f)
		if err != nil {
			probs = append(probs, place+": "+err.Error())
			continue
		}
		var generic any
		var scn struct {
			Adventures string       `json:"adventures"`
			Steps      []qaDataStep `json:"steps"`
		}
		if err := json.Unmarshal(b, &generic); err != nil {
			probs = append(probs, place+": "+err.Error())
			continue
		}
		if err := json.Unmarshal(b, &scn); err != nil {
			probs = append(probs, place+": "+err.Error())
			continue
		}
		sheets := map[string][]string{}
		seenUses := 0
		for i, s := range scn.Steps {
			ok := s.Expect != nil && s.Expect.OK
			if raw, has := s.Command["addActor"]; has {
				var add struct {
					Actor struct {
						ActorID    string   `json:"actorId"`
						AbilityIDs []string `json:"abilityIds"`
					} `json:"actor"`
				}
				if err := json.Unmarshal(raw, &add); err != nil {
					probs = append(probs, fmt.Sprintf("%s step %d: %v", place, i, err))
					continue
				}
				if !ok {
					continue
				}
				sheets[add.Actor.ActorID] = add.Actor.AbilityIDs
			}
			if raw, has := s.Command["loadAdventure"]; has && ok {
				var load struct {
					AdventureID string `json:"adventureId"`
				}
				if err := json.Unmarshal(raw, &load); err != nil {
					probs = append(probs, fmt.Sprintf("%s step %d: %v", place, i, err))
					continue
				}
				if scn.Adventures == "" {
					probs = append(probs, fmt.Sprintf("%s step %d: loads an adventure with no adventures dir", place, i))
					continue
				}
				got, _, err := qaDataReadActors(filepath.Join(root, scn.Adventures, load.AdventureID))
				if err != nil {
					probs = append(probs, fmt.Sprintf("%s step %d: %v", place, i, err))
					continue
				}
				for id, ids := range got {
					sheets[id] = ids
				}
			}
			raw, has := s.Command["useAbility"]
			if !has {
				continue
			}
			seenUses++
			var use qaDataUse
			if err := json.Unmarshal(raw, &use); err != nil {
				probs = append(probs, fmt.Sprintf("%s step %d: %v", place, i, err))
				continue
			}
			class := "expected to succeed"
			mustName := true
			switch {
			case ok:
			case s.Expect != nil && s.Expect.DeniedContaining != "" && qaDataSheetRefusal(s.Expect.DeniedContaining):
				class = fmt.Sprintf("refused for the sheet (%q), exempt", s.Expect.DeniedContaining)
				mustName = false
			case s.Expect != nil && s.Expect.DeniedContaining != "":
				class = fmt.Sprintf("refused for another reason (%q)", s.Expect.DeniedContaining)
			default:
				class = "no expectation, held as expected to succeed"
			}
			sheet, known := sheets[use.ActorID]
			names := known && slices.Contains(sheet, use.AbilityID)
			uses = append(uses, fmt.Sprintf("%s step %d: %s sends %s using %s; %s; sheet %q names it: %v",
				place, i, s.By, use.ActorID, use.AbilityID, class, sheet, names))
			if mustName && !names {
				probs = append(probs, fmt.Sprintf("%s step %d: %s uses %s but its sheet %q does not name it (%s)",
					place, i, use.ActorID, use.AbilityID, sheet, class))
			}
		}
		if total := qaDataCountKey(generic, "useAbility"); total != seenUses {
			probs = append(probs, fmt.Sprintf("%s: %d useAbility keys in the file, %d in step commands", place, total, seenUses))
		}
	}
	return probs, uses
}

var (
	qaDataFence    = regexp.MustCompile("(?s)```json\n(.*?)```")
	qaDataAnyFence = regexp.MustCompile("(?s)```.*?```")
	qaDataSpace    = regexp.MustCompile(`\s+`)
	qaDataNoField  = regexp.MustCompile("(?i)not (a )?part of|no (such )?field|no `?abilit\\w*`? field|" +
		"has no `?abilit|carries no `?abilit|nowhere to name|not (carried )?in (the|its) payload|" +
		"outside (the|its) payload|there is no `?abilit|no `?abilit\\w*`? (key|argument|parameter)")
)

func qaDataSentences(text string) []string {
	flat := qaDataSpace.ReplaceAllString(qaDataAnyFence.ReplaceAllString(text, " "), " ")
	var out []string
	start := 0
	for i := 0; i+1 < len(flat); i++ {
		if (flat[i] == '.' || flat[i] == '!' || flat[i] == '?' || flat[i] == ':') && flat[i+1] == ' ' {
			out = append(out, strings.TrimSpace(flat[start:i+1]))
			start = i + 2
		}
	}
	return append(out, strings.TrimSpace(flat[start:]))
}

func qaDataGuideProblems(root string) (probs, notes []string) {
	b, err := os.ReadFile(filepath.Join(root, "rulesets", "dnd45e-minimal", "guide.md"))
	if err != nil {
		return []string{err.Error()}, nil
	}
	guide := string(b)
	actors := map[string][]string{}
	heads, _ := filepath.Glob(filepath.Join(root, "adventures", "*", "adventure.json"))
	for _, h := range heads {
		if id, err := qaDataAdventureRuleset(filepath.Dir(h)); err == nil && id == "dnd45e-minimal" {
			got, _, err := qaDataReadActors(filepath.Dir(h))
			if err != nil {
				probs = append(probs, err.Error())
			}
			for k, v := range got {
				actors[k] = v
			}
		}
	}
	start := strings.Index(guide, "\n## Reference statblocks")
	if start < 0 {
		return append(probs, "guide has no Reference statblocks section"), nil
	}
	section := guide[start+1:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	blocks := qaDataFence.FindAllStringSubmatch(section, -1)
	if len(blocks) == 0 {
		probs = append(probs, "Reference statblocks has no JSON block")
	}
	for _, m := range blocks {
		var block map[string]json.RawMessage
		if err := json.Unmarshal([]byte(m[1]), &block); err != nil {
			probs = append(probs, "statblock does not parse: "+err.Error())
			continue
		}
		var id string
		var ids []string
		_ = json.Unmarshal(block["actorId"], &id)
		raw, has := block["abilityIds"]
		if !has {
			probs = append(probs, fmt.Sprintf("statblock %s carries no abilityIds", id))
			continue
		}
		if err := json.Unmarshal(raw, &ids); err != nil {
			probs = append(probs, fmt.Sprintf("statblock %s abilityIds: %v", id, err))
			continue
		}
		want, found := actors[id]
		switch {
		case !found:
			probs = append(probs, fmt.Sprintf("statblock %s has no adventure actor under dnd45e-minimal", id))
		case !slices.Equal(ids, want):
			probs = append(probs, fmt.Sprintf("statblock %s names %q, its adventure actor %q", id, ids, want))
		default:
			notes = append(notes, fmt.Sprintf("statblock %s names %q, equal to its adventure actor", id, ids))
		}
	}
	schema := 0
	for _, s := range qaDataSentences(guide) {
		if strings.Contains(s, "wire schema") && strings.Contains(s, "add_actor") {
			schema++
			notes = append(notes, "wire-schema sentence: "+s)
			if !strings.Contains(s, "abilityIds") {
				probs = append(probs, "the wire-schema sentence names no abilityIds: "+s)
			}
		}
		if !strings.Contains(strings.ToLower(s), "abilit") {
			continue
		}
		if qaDataNoField.MatchString(s) {
			probs = append(probs, "a sentence says abilities have no place in add_actor: "+s)
		}
		l := strings.ToLower(s)
		if strings.Contains(l, "field") || strings.Contains(l, "payload") || strings.Contains(l, "wire") || strings.Contains(l, "bind") {
			notes = append(notes, "read by hand: "+s)
		}
	}
	if schema == 0 {
		probs = append(probs, "no sentence of the guide gives add_actor's wire schema")
	}
	return probs, notes
}

func qaDataGoldenProblems(root string) []string {
	var probs []string
	dirs, _ := filepath.Glob(filepath.Join(root, "scenarios", "goldens", "*", "state.json"))
	if len(dirs) == 0 {
		return []string{"no scenario golden under " + root}
	}
	for _, statePath := range dirs {
		dir := filepath.Dir(statePath)
		name := filepath.Base(dir)
		b, err := os.ReadFile(statePath)
		if err != nil {
			probs = append(probs, err.Error())
			continue
		}
		var top map[string]json.RawMessage
		if err := json.Unmarshal(b, &top); err != nil {
			probs = append(probs, name+" state: "+err.Error())
			continue
		}
		actors, err := qaDataFields(top["Actors"])
		if err != nil {
			probs = append(probs, name+" state Actors: "+err.Error())
			continue
		}
		held := map[string][]string{}
		for _, a := range actors {
			fields, err := qaDataFields(a.raw)
			if err != nil {
				probs = append(probs, name+" state "+a.key+": "+err.Error())
				continue
			}
			keys := make([]string, len(fields))
			for i, f := range fields {
				keys[i] = f.key
			}
			at := slices.Index(keys, "ability_ids")
			if at < 0 {
				held[a.key] = nil
				continue
			}
			if k := slices.Index(keys, "kind"); k < 0 || k+1 != at {
				probs = append(probs, fmt.Sprintf("%s state %s: ability_ids does not follow kind, keys %q", name, a.key, keys))
			}
			var ids []string
			_ = json.Unmarshal(fields[at].raw, &ids)
			held[a.key] = ids
		}
		sb, err := os.ReadFile(filepath.Join(dir, "stream.json"))
		if err != nil {
			probs = append(probs, err.Error())
			continue
		}
		var stream []struct {
			ActorAdded *struct {
				Actor struct {
					ActorID    string   `json:"actorId"`
					AbilityIDs []string `json:"abilityIds"`
				} `json:"actor"`
			} `json:"actorAdded"`
		}
		if err := json.Unmarshal(sb, &stream); err != nil {
			probs = append(probs, name+" stream: "+err.Error())
			continue
		}
		added := map[string][]string{}
		for _, e := range stream {
			if e.ActorAdded != nil {
				added[e.ActorAdded.Actor.ActorID] = e.ActorAdded.Actor.AbilityIDs
			}
		}
		for _, a := range actors {
			if got, want := added[a.key], held[a.key]; !slices.Equal(got, want) {
				probs = append(probs, fmt.Sprintf("%s stream actorAdded %s carries %q, its state holds %q", name, a.key, got, want))
			}
		}
	}
	return probs
}

func qaDataReport(t *testing.T, probs []string) {
	t.Helper()
	for _, p := range probs {
		t.Error(p)
	}
}

// Keep every adventure actor's abilities ones its ruleset holds, loaded as its
// file names them.
func TestQADataAdventureActorsNameAbilitiesTheirRulesetHolds(t *testing.T) {
	qaDataReport(t, qaDataAdventureProblems(qaDataRoot(t)))
}

// Keep every scenario use not refused for the sheet cast by an actor whose sheet
// names the ability.
func TestQADataScenarioSheetsNameTheirUses(t *testing.T) {
	probs, uses := qaDataScenarioProblems(qaDataRoot(t))
	for _, u := range uses {
		t.Log(u)
	}
	if len(uses) == 0 {
		t.Error("no useAbility found in any scenario")
	}
	qaDataReport(t, probs)
}

type qaDataMutation struct {
	name   string
	golden bool
	fn     func(json.RawMessage) (json.RawMessage, bool)
}

func qaDataConformanceCopy(t *testing.T, name string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), name)
	qaDataCopyTree(t, filepath.Join(qaDataRoot(t), "adventures", name), dst)
	return dst
}

func qaDataEditGoldenActor(t *testing.T, dir, actorID string, fn func(json.RawMessage) (json.RawMessage, bool)) {
	t.Helper()
	qaDataRewrite(t, filepath.Join(dir, "goldens", "compiled-batch.json"), func(b []byte) ([]byte, error) {
		var batch []json.RawMessage
		if err := json.Unmarshal(b, &batch); err != nil {
			return nil, err
		}
		hit := false
		for i, e := range batch {
			var head struct {
				Type  string `json:"type"`
				Added struct {
					ActorID string `json:"actor_id"`
				} `json:"actor_added"`
			}
			if err := json.Unmarshal(e, &head); err != nil {
				return nil, err
			}
			if head.Type != "actor_added" || head.Added.ActorID != actorID {
				continue
			}
			out, err := qaDataEdit(e, []string{"actor_added", "ability_ids"}, fn)
			if err != nil {
				return nil, err
			}
			batch[i], hit = out, true
		}
		if !hit {
			return nil, errors.New("no actor_added for " + actorID)
		}
		flat, err := json.Marshal(batch)
		if err != nil {
			return nil, err
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, flat, "", "  "); err != nil {
			return nil, err
		}
		pretty.WriteByte('\n')
		return pretty.Bytes(), nil
	})
}

// Keep the conformance golden pinning each compiled actor's ability ids: a batch
// naming other ids, or none, than the golden says is refused.
func TestQADataConformancePinsAbilityIDs(t *testing.T) {
	rulesetsRoot := filepath.Join(qaDataRoot(t), "rulesets")
	cases := map[string]struct {
		actor string
		extra string
	}{
		"goblin-ambush": {"act-fighter", "goblin-scimitar"},
		"cellar-rats":   {"hollis", ""},
	}
	for adv, c := range cases {
		t.Run(adv+"/unchanged copy passes", func(t *testing.T) {
			if err := conformance.Run(qaDataConformanceCopy(t, adv), rulesetsRoot); err != nil {
				t.Fatalf("unchanged copy refused: %v", err)
			}
		})
		t.Run(adv+"/golden re-encoded unchanged passes", func(t *testing.T) {
			dir := qaDataConformanceCopy(t, adv)
			before, _ := os.ReadFile(filepath.Join(dir, "goldens", "compiled-batch.json"))
			qaDataEditGoldenActorNoop(t, dir, before)
			if err := conformance.Run(dir, rulesetsRoot); err != nil {
				t.Fatalf("re-encoded golden refused: %v", err)
			}
		})
		muts := []qaDataMutation{
			{"one id removed", false, func(r json.RawMessage) (json.RawMessage, bool) {
				var ids []string
				_ = json.Unmarshal(r, &ids)
				return qaDataIDs(ids[:len(ids)-1]...), true
			}},
			{"reordered", false, func(r json.RawMessage) (json.RawMessage, bool) {
				var ids []string
				_ = json.Unmarshal(r, &ids)
				slices.Reverse(ids)
				return qaDataIDs(ids...), true
			}},
			{"list removed", false, func(json.RawMessage) (json.RawMessage, bool) { return nil, false }},
			{"list emptied", false, func(json.RawMessage) (json.RawMessage, bool) { return qaDataIDs(), true }},
			{"golden list removed", true, func(json.RawMessage) (json.RawMessage, bool) { return nil, false }},
			{"golden one id removed", true, func(r json.RawMessage) (json.RawMessage, bool) {
				var ids []string
				_ = json.Unmarshal(r, &ids)
				return qaDataIDs(ids[1:]...), true
			}},
		}
		if c.extra != "" {
			muts = append(muts, qaDataMutation{"another id added", false, func(r json.RawMessage) (json.RawMessage, bool) {
				var ids []string
				_ = json.Unmarshal(r, &ids)
				return qaDataIDs(append(ids, c.extra)...), true
			}})
		}
		for _, m := range muts {
			t.Run(adv+"/"+m.name+" is refused", func(t *testing.T) {
				dir := qaDataConformanceCopy(t, adv)
				if m.golden {
					qaDataEditGoldenActor(t, dir, c.actor, m.fn)
				} else {
					qaDataRewrite(t, filepath.Join(dir, "actors", c.actor+".json"), func(b []byte) ([]byte, error) {
						return qaDataEdit(b, []string{"ability_ids"}, m.fn)
					})
				}
				err := conformance.Run(dir, rulesetsRoot)
				if err == nil {
					t.Fatalf("%s %s: conformance.Run accepted it", adv, m.name)
				}
				t.Logf("refused: %v", err)
			})
		}
	}
}

func qaDataEditGoldenActorNoop(t *testing.T, dir string, before []byte) {
	t.Helper()
	path := filepath.Join(dir, "goldens", "compiled-batch.json")
	var batch []json.RawMessage
	if err := json.Unmarshal(before, &batch); err != nil {
		t.Fatal(err)
	}
	flat, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, flat, "", "  "); err != nil {
		t.Fatal(err)
	}
	pretty.WriteByte('\n')
	if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Keep the ruleset guide's statblocks naming their adventure actors' abilities in
// the same order, its add_actor schema sentence naming abilityIds, and no
// sentence saying an actor's abilities have no field in add_actor.
func TestQADataGuideStatblocksNameAbilities(t *testing.T) {
	probs, notes := qaDataGuideProblems(qaDataRoot(t))
	for _, n := range notes {
		t.Log(n)
	}
	qaDataReport(t, probs)
}

// Keep each scenario golden's state.json holding an actor's ability_ids right
// after kind, and its stream.json's actorAdded carrying the list state holds.
func TestQADataScenarioGoldensCarryAbilityIDs(t *testing.T) {
	qaDataReport(t, qaDataGoldenProblems(qaDataRoot(t)))
}

func qaDataMustSee(t *testing.T, probs []string, want string) {
	t.Helper()
	for _, p := range probs {
		if strings.Contains(p, want) {
			t.Logf("reddened: %s", p)
			return
		}
	}
	t.Errorf("the check did not see the injected fault (wanted a problem containing %q); got %q", want, probs)
}

func qaDataEditScenario(t *testing.T, path string, fn func(steps []any) error) {
	t.Helper()
	qaDataRewrite(t, path, func(b []byte) ([]byte, error) {
		var scn map[string]any
		if err := json.Unmarshal(b, &scn); err != nil {
			return nil, err
		}
		steps, _ := scn["steps"].([]any)
		if err := fn(steps); err != nil {
			return nil, err
		}
		return json.Marshal(scn)
	})
}

func qaDataStepCommand(step any, name string) map[string]any {
	s, _ := step.(map[string]any)
	c, _ := s["command"].(map[string]any)
	v, _ := c[name].(map[string]any)
	return v
}

// The vacuity half: each checker above, run on a copy holding one injected
// fault, must report that fault.
func TestQADataChecksSeeWrongData(t *testing.T) {
	t.Run("the adventure check sees an id the ruleset does not hold", func(t *testing.T) {
		root := qaDataTempRoot(t)
		qaDataRewrite(t, filepath.Join(root, "adventures", "cellar-rats", "actors", "hollis.json"), func(b []byte) ([]byte, error) {
			return qaDataEdit(b, []string{"ability_ids"}, func(r json.RawMessage) (json.RawMessage, bool) {
				var ids []string
				_ = json.Unmarshal(r, &ids)
				return qaDataIDs(append(ids, "no-such-ability")...), true
			})
		})
		qaDataMustSee(t, qaDataAdventureProblems(root), "does not hold")
	})
	t.Run("the use check sees an expected success the caster's sheet does not name", func(t *testing.T) {
		root := qaDataTempRoot(t)
		qaDataEditScenario(t, filepath.Join(root, "scenarios", "goblin-fight.json"), func(steps []any) error {
			for _, s := range steps {
				u := qaDataStepCommand(s, "useAbility")
				exp, _ := s.(map[string]any)["expect"].(map[string]any)
				if u != nil && u["abilityId"] == "goblin-shortbow" && exp["ok"] == true {
					u["actorId"] = "act-cutter"
					return nil
				}
			}
			return errors.New("no ok goblin-shortbow use")
		})
		probs, _ := qaDataScenarioProblems(root)
		qaDataMustSee(t, probs, "act-cutter uses goblin-shortbow")
	})
	t.Run("the use check reads a loaded adventure's actor file as the sheet", func(t *testing.T) {
		root := qaDataTempRoot(t)
		qaDataRewrite(t, filepath.Join(root, "adventures", "goblin-ambush", "actors", "act-fighter.json"), func(b []byte) ([]byte, error) {
			return qaDataEdit(b, []string{"ability_ids"}, func(json.RawMessage) (json.RawMessage, bool) {
				return qaDataIDs("crossbow-shot", "hunters-flurry", "longsword-strike", "staggering-blow"), true
			})
		})
		probs, _ := qaDataScenarioProblems(root)
		qaDataMustSee(t, probs, "scenarios/adventure-night.json step")
	})
	refusal := func(t *testing.T, denied string) []string {
		t.Helper()
		root := qaDataTempRoot(t)
		qaDataEditScenario(t, filepath.Join(root, "scenarios", "goblin-fight.json"), func(steps []any) error {
			for _, s := range steps {
				u := qaDataStepCommand(s, "useAbility")
				exp, _ := s.(map[string]any)["expect"].(map[string]any)
				if u != nil && u["actorId"] == "act-cutter" && exp["deniedContaining"] == "not authorized" {
					u["abilityId"] = "goblin-shortbow"
					exp["deniedContaining"] = denied
					return nil
				}
			}
			return errors.New("no refused act-cutter use")
		})
		probs, _ := qaDataScenarioProblems(root)
		return probs
	}
	t.Run("the use check holds a refusal for another reason to the sheet", func(t *testing.T) {
		qaDataMustSee(t, refusal(t, "not authorized"), "act-cutter uses goblin-shortbow")
	})
	t.Run("the use check exempts a refusal for the sheet", func(t *testing.T) {
		if probs := refusal(t, "does not name"); len(probs) != 0 {
			t.Errorf("a sheet refusal was held to the sheet: %q", probs)
		}
	})
	guide := func(t *testing.T, old, repl string) []string {
		t.Helper()
		root := qaDataTempRoot(t)
		qaDataRewrite(t, filepath.Join(root, "rulesets", "dnd45e-minimal", "guide.md"), func(b []byte) ([]byte, error) {
			if strings.Count(string(b), old) != 1 {
				return nil, fmt.Errorf("%q is not in the guide exactly once", old)
			}
			return []byte(strings.Replace(string(b), old, repl, 1)), nil
		})
		probs, _ := qaDataGuideProblems(root)
		return probs
	}
	t.Run("the guide check sees a statblock in another order", func(t *testing.T) {
		qaDataMustSee(t, guide(t, `"abilityIds": ["goblin-scimitar", "goblin-shortbow"]`,
			`"abilityIds": ["goblin-shortbow", "goblin-scimitar"]`), "statblock act-archer")
	})
	t.Run("the guide check sees a wire schema without abilityIds", func(t *testing.T) {
		qaDataMustSee(t, guide(t, "`resources`/`abilityIds` (plus", "`resources` (plus"), "names no abilityIds")
	})
	for _, claim := range []string{
		"An actor's abilities are not part of `add_actor`'s payload.",
		"There is no field for an actor's abilities.",
		"The actor has no `abilities` field.",
	} {
		t.Run("the guide check sees the claim "+claim, func(t *testing.T) {
			qaDataMustSee(t, guide(t, "## Reference statblocks\n\n", "## Reference statblocks\n\n"+claim+" "), "no place in add_actor")
		})
	}
	t.Run("the golden check sees ability_ids before kind", func(t *testing.T) {
		root := qaDataTempRoot(t)
		qaDataRewrite(t, filepath.Join(root, "scenarios", "goldens", "toy-brawl", "state.json"), func(b []byte) ([]byte, error) {
			return qaDataEdit(b, []string{"Actors", "act-brawler"}, func(r json.RawMessage) (json.RawMessage, bool) {
				fields, _ := qaDataFields(r)
				var front, rest []qaDataField
				for _, f := range fields {
					if f.key == "ability_ids" {
						front = append(front, f)
					} else {
						rest = append(rest, f)
					}
				}
				return qaDataJoin(append(front, rest...)), true
			})
		})
		qaDataMustSee(t, qaDataGoldenProblems(root), "toy-brawl state act-brawler: ability_ids does not follow kind")
	})
	t.Run("the golden check sees a state actor without its list", func(t *testing.T) {
		root := qaDataTempRoot(t)
		qaDataRewrite(t, filepath.Join(root, "scenarios", "goldens", "adventure-night", "state.json"), func(b []byte) ([]byte, error) {
			return qaDataEdit(b, []string{"Actors", "act-cutter", "ability_ids"}, func(json.RawMessage) (json.RawMessage, bool) {
				return nil, false
			})
		})
		qaDataMustSee(t, qaDataGoldenProblems(root), "adventure-night stream actorAdded act-cutter")
	})
	t.Run("the golden check sees a stream actorAdded list other than its state's", func(t *testing.T) {
		root := qaDataTempRoot(t)
		qaDataRewrite(t, filepath.Join(root, "scenarios", "goldens", "door-watch", "stream.json"), func(b []byte) ([]byte, error) {
			var stream []json.RawMessage
			if err := json.Unmarshal(b, &stream); err != nil {
				return nil, err
			}
			for i, e := range stream {
				if !bytes.Contains(e, []byte(`"actorAdded"`)) || !bytes.Contains(e, []byte(`"act-hidden"`)) {
					continue
				}
				out, err := qaDataEdit(e, []string{"actorAdded", "actor", "abilityIds"}, func(json.RawMessage) (json.RawMessage, bool) {
					return qaDataIDs("chair-swing", "sober-up", "splash-of-water"), true
				})
				if err != nil {
					return nil, err
				}
				stream[i] = out
				return json.Marshal(stream)
			}
			return nil, errors.New("no act-hidden actorAdded")
		})
		qaDataMustSee(t, qaDataGoldenProblems(root), "door-watch stream actorAdded act-hidden")
	})
}
