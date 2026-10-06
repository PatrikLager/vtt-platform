package rules_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaRuleBound = 128

const (
	qaRuleAbilityFile      = "qa-ability.json"
	qaRuleExtraAbilityFile = "qa-extra-ability.json"
	qaRuleConditionFile    = "qa-condition.json"
	qaRuleExtraCondFile    = "qa-extra-condition.json"
	qaRuleRollFile         = "qa-roll.json"
	qaRuleManifestFile     = "ruleset.json"
)

const (
	qaRuleFieldAbility   = "ability id"
	qaRuleFieldCondition = "condition id"
	qaRuleFieldAttribute = "attribute name"
	qaRuleFieldDefense   = "defense name"
	qaRuleFieldResource  = "resource name"
	qaRuleFieldBranch    = "branch label"
)

var qaRuleFields = []string{
	qaRuleFieldAbility, qaRuleFieldCondition, qaRuleFieldAttribute,
	qaRuleFieldDefense, qaRuleFieldResource, qaRuleFieldBranch,
}

type qaRuleNames struct {
	ability, cond, attr, def, res, ge, lt string
}

func qaRuleShort() qaRuleNames {
	return qaRuleNames{
		ability: "qa-ability", cond: "qa-cond", attr: "qa_attr", def: "qa_def", res: "qa_res",
		ge: "qa-ge", lt: "qa-lt",
	}
}

func qaRulePad(prefix, fill string, n int) string {
	s := prefix
	for len(s)+len(fill) <= n {
		s += fill
	}
	return s + strings.Repeat("x", n-len(s))
}

func qaRuleAtBound() qaRuleNames {
	return qaRuleNames{
		ability: strings.Repeat("é", 64),
		cond:    strings.Repeat("€", 42) + "ab",
		attr:    qaRulePad("attr_", "a", qaRuleBound),
		def:     qaRulePad("def_", "d", qaRuleBound),
		res:     qaRulePad("res_", "r", qaRuleBound),
		ge:      strings.Repeat("😀", 32),
		lt:      "lt" + strings.Repeat("é", 63),
	}
}

type qaRuleSet struct {
	n                             qaRuleNames
	extraAbility, extraCond       string
	extraAttr, extraDef, extraRes string
	thresholds                    []map[string]any
}

func qaRuleWrite(t *testing.T, p string, doc any) {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal %s: %v", p, err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func qaRuleComposition(id, usageRes string) map[string]any {
	var usage any = "at_will"
	if usageRes != "" {
		usage = map[string]any{"limited": map[string]any{"resource": usageRes, "cost": 1}}
	}
	return map[string]any{
		"id": id, "name": "QA Ability", "usage": usage,
		"compose": []map[string]any{
			{"atom": "qa-reach", "bind": map[string]any{}},
			{"atom": "qa-roll", "bind": map[string]any{}},
			{"atom": "qa-on-hit", "bind": map[string]any{}},
		},
	}
}

func (s qaRuleSet) write(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	attrs := []string{s.n.attr}
	if s.extraAttr != "" {
		attrs = append(attrs, s.extraAttr)
	}
	defs := []string{s.n.def}
	if s.extraDef != "" {
		defs = append(defs, s.extraDef)
	}
	thresholds := s.thresholds
	if thresholds == nil {
		thresholds = []map[string]any{{"when": "#" + s.n.res, "apply_condition": s.n.cond, "remove_when_false": true}}
	}
	resources := []map[string]any{{"name": s.n.res, "thresholds": thresholds}}
	if s.extraRes != "" {
		resources = append(resources, map[string]any{"name": s.extraRes})
	}
	qaRuleWrite(t, filepath.Join(dir, qaRuleManifestFile), map[string]any{
		"id": "qa-rules", "name": "QA Rules", "format_version": "2",
		"attributes": attrs, "defenses": defs, "resources": resources,
	})
	qaRuleWrite(t, filepath.Join(dir, "conditions", qaRuleConditionFile), map[string]any{
		"id": s.n.cond, "name": "QA Condition",
	})
	if s.extraCond != "" {
		qaRuleWrite(t, filepath.Join(dir, "conditions", qaRuleExtraCondFile), map[string]any{
			"id": s.extraCond, "name": "QA Extra Condition",
		})
	}
	qaRuleWrite(t, filepath.Join(dir, "atoms", "qa-reach.json"), map[string]any{
		"id": "qa-reach", "params": []any{}, "provides": []string{"delivery"}, "consumes": []string{},
		"contributes": []map[string]any{{"kind": "targeting", "range": 1, "max_targets": 1}},
	})
	qaRuleWrite(t, filepath.Join(dir, "atoms", qaRuleRollFile), map[string]any{
		"id": "qa-roll", "params": []any{}, "provides": []string{"qa-key"}, "consumes": []string{"delivery"},
		"contributes": []map[string]any{{
			"kind": "resolution", "key": "qa-key",
			"roll": "1d20 + @caster." + s.n.attr, "vs": "@target." + s.n.def,
			"branches": []string{s.n.ge, s.n.lt},
		}},
	})
	qaRuleWrite(t, filepath.Join(dir, "atoms", "qa-on-hit.json"), map[string]any{
		"id": "qa-on-hit", "params": []any{}, "provides": []string{}, "consumes": []string{"qa-key"},
		"contributes": []map[string]any{{
			"kind": "outcome", "key": "qa-key", "branch": s.n.ge,
			"effects": []map[string]any{
				{"resource_change": map[string]any{"resource": s.n.res, "delta_expr": "0 - 1"}},
				{"apply_condition": map[string]any{"id": s.n.cond}},
			},
		}},
	})
	qaRuleWrite(t, filepath.Join(dir, "abilities", qaRuleAbilityFile), qaRuleComposition(s.n.ability, s.n.res))
	if s.extraAbility != "" {
		qaRuleWrite(t, filepath.Join(dir, "abilities", qaRuleExtraAbilityFile), qaRuleComposition(s.extraAbility, ""))
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# QA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

type qaRuleOver struct {
	label string
	value string
}

func qaRuleOverAny() []qaRuleOver {
	return []qaRuleOver{
		{"129 ascii", strings.Repeat("a", 129)},
		{"70000 ascii", strings.Repeat("a", 70000)},
		{"65 two-byte runes", strings.Repeat("é", 65)},
		{"one ascii then 64 two-byte runes", "a" + strings.Repeat("é", 64)},
		{"43 three-byte runes", strings.Repeat("€", 43)},
		{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
	}
}

func qaRuleOverIdent() []qaRuleOver {
	return []qaRuleOver{
		{"129 ascii", "q" + strings.Repeat("a", 128)},
		{"70000 ascii", "q" + strings.Repeat("a", 69999)},
	}
}

func qaRuleOverFor(field string) []qaRuleOver {
	switch field {
	case qaRuleFieldAttribute, qaRuleFieldDefense, qaRuleFieldResource:
		return qaRuleOverIdent()
	}
	return qaRuleOverAny()
}

var qaRuleFieldName = map[string][]*regexp.Regexp{
	qaRuleFieldAbility:   {regexp.MustCompile(`(?i)\bid\b`)},
	qaRuleFieldCondition: {regexp.MustCompile(`(?i)\bid\b`)},
	qaRuleFieldAttribute: {regexp.MustCompile(`(?i)attributes?\b`)},
	qaRuleFieldDefense:   {regexp.MustCompile(`(?i)defenses?\b`)},
	qaRuleFieldResource:  {regexp.MustCompile(`(?i)resources?\b`), regexp.MustCompile(`(?i)\bname\b`)},
	qaRuleFieldBranch:    {regexp.MustCompile(`(?i)branch`)},
}

func qaRuleUsed(field, v string) (qaRuleSet, string) {
	s := qaRuleSet{n: qaRuleShort()}
	file := qaRuleManifestFile
	switch field {
	case qaRuleFieldAbility:
		s.n.ability, file = v, qaRuleAbilityFile
	case qaRuleFieldCondition:
		s.n.cond, file = v, qaRuleConditionFile
	case qaRuleFieldAttribute:
		s.n.attr = v
	case qaRuleFieldDefense:
		s.n.def = v
	case qaRuleFieldResource:
		s.n.res = v
	case qaRuleFieldBranch:
		s.n.ge, file = v, qaRuleRollFile
	}
	return s, file
}

func qaRuleAlone(field, v string) (qaRuleSet, string) {
	s := qaRuleSet{n: qaRuleShort()}
	file := qaRuleManifestFile
	switch field {
	case qaRuleFieldAbility:
		s.extraAbility, file = v, qaRuleExtraAbilityFile
	case qaRuleFieldCondition:
		s.extraCond, file = v, qaRuleExtraCondFile
	case qaRuleFieldAttribute:
		s.extraAttr = v
	case qaRuleFieldDefense:
		s.extraDef = v
	case qaRuleFieldResource:
		s.extraRes = v
	case qaRuleFieldBranch:
		s.n.lt, file = v, qaRuleRollFile
	}
	return s, file
}

func qaRuleLoadRefusal(t *testing.T, dir, value string) string {
	t.Helper()
	rs, err := rules.Load(dir)
	if err == nil {
		t.Fatalf("rules.Load accepted a ruleset holding a %d-byte string (abilities %d, conditions %d)",
			len(value), len(rs.Compiled), len(rs.Conditions))
	}
	msg := strings.ReplaceAll(err.Error(), filepath.Dir(dir), "<root>")
	return strings.ReplaceAll(msg, value, "<s>")
}

// VTT-283 VTT-284
func TestQARuleLoadRefusesANameOverTheBoundNamingTheFileAndTheField(t *testing.T) {
	builds := map[string]func(string, string) (qaRuleSet, string){"used": qaRuleUsed, "alone": qaRuleAlone}
	for _, field := range qaRuleFields {
		for how, build := range builds {
			for _, c := range qaRuleOverFor(field) {
				t.Run(field+"/"+how+"/"+c.label, func(t *testing.T) {
					if len(c.value) <= qaRuleBound {
						t.Fatalf("fixture %q is %d bytes, within the bound", c.label, len(c.value))
					}
					s, file := build(field, c.value)
					msg := qaRuleLoadRefusal(t, s.write(t), c.value)
					t.Logf("refusal: %s", msg)
					if !strings.Contains(msg, file) {
						t.Errorf("refusal %q does not name the file %s", msg, file)
					}
					for _, re := range qaRuleFieldName[field] {
						if !re.MatchString(msg) {
							t.Errorf("refusal %q does not name the %s field (%s)", msg, field, re)
						}
					}
				})
			}
		}
	}
}

func qaRuleContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func qaRuleLoaded(t *testing.T, n qaRuleNames, dir string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load refused a ruleset whose names are at most %d bytes: %v", qaRuleBound, err)
	}
	p := rs.Compiled[n.ability]
	if p == nil || p.ID != n.ability {
		t.Fatalf("the loaded ruleset does not hold the %d-byte ability id whole", len(n.ability))
	}
	if c := rs.Conditions[n.cond]; c == nil || c.ID != n.cond {
		t.Fatalf("the loaded ruleset does not hold the %d-byte condition id whole", len(n.cond))
	}
	if !qaRuleContains(rs.Attributes, n.attr) || !qaRuleContains(rs.Defenses, n.def) {
		t.Fatalf("the loaded ruleset does not hold the attribute and defense names whole")
	}
	if len(rs.Resources) != 1 || rs.Resources[0].Name != n.res {
		t.Fatalf("the loaded ruleset does not hold the %d-byte resource name whole", len(n.res))
	}
	if p.Resolution == nil || p.Resolution.Branches != [2]string{n.ge, n.lt} {
		t.Fatalf("the loaded ability does not carry its branch labels whole")
	}
	return rs
}

// VTT-283 VTT-284 VTT-287
func TestQARuleARulesetWhoseNamesAreAllAtTheBoundLoads(t *testing.T) {
	n := qaRuleAtBound()
	for _, s := range []string{n.ability, n.cond, n.attr, n.def, n.res, n.ge, n.lt} {
		if len(s) != qaRuleBound {
			t.Fatalf("fixture %q is %d bytes, want %d", s, len(s), qaRuleBound)
		}
	}
	qaRuleLoaded(t, n, qaRuleSet{n: n}.write(t))
}

type qaRuleMaxRoller struct{}

func (qaRuleMaxRoller) Roll(n, sides int) ([]int, int) {
	out := make([]int, n)
	for i := range out {
		out[i] = sides
	}
	return out, n * sides
}

func qaRuleEnv(seq int64, env *vttv1.Envelope) *vttv1.Envelope {
	env.Sequence = seq
	env.EventId = "qa-rule-" + strconv.FormatInt(seq, 10)
	return env
}

func qaRuleTable(t *testing.T, actors ...*vttv1.Actor) *engine.State {
	t.Helper()
	st := engine.NewState()
	envs := []*vttv1.Envelope{{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "qa-scene", Name: "qa", GridWidth: 4, GridHeight: 4,
	}}}}
	for i, a := range actors {
		envs = append(envs,
			&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}},
			&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
				TokenId: "qa-token-" + a.GetActorId(), SceneId: "qa-scene", ActorId: a.GetActorId(),
				Position: &vttv1.GridPosition{X: int32(1 + i), Y: 1},
			}}})
	}
	for i, env := range envs {
		if err := engine.Apply(st, qaRuleEnv(int64(i+1), env)); err != nil {
			t.Fatalf("setup event %d refused: %v", i, err)
		}
	}
	return st
}

func qaRuleActor(id string, attrs map[string]int32, res string, current int32) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: id, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, Attributes: attrs,
		Resources: map[string]*vttv1.Resource{res: {Current: current, Max: current}},
	}
}

// VTT-283 VTT-285
func TestQARuleWhatTheLoaderAcceptsAtTheBoundTheFoldAccepts(t *testing.T) {
	n := qaRuleAtBound()
	rs := qaRuleLoaded(t, n, qaRuleSet{n: n}.write(t))
	st := qaRuleTable(t,
		qaRuleActor("qa-caster", map[string]int32{n.attr: 5}, n.res, 9),
		qaRuleActor("qa-target", map[string]int32{n.def: 1}, n.res, 9),
	)
	cmd := &vttv1.UseAbility{ActorId: "qa-caster", AbilityId: n.ability, TargetIds: []string{"qa-target"}}
	envs, err := rules.Resolve(rs, st, cmd, qaRuleMaxRoller{})
	if err != nil {
		t.Fatalf("Resolve of the at-bound ability refused: %v", err)
	}
	var used, applied bool
	seq := int64(100)
	for i, env := range envs {
		seq++
		if err := engine.Apply(st, qaRuleEnv(seq, env)); err != nil {
			t.Fatalf("the fold refused event %d of the at-bound batch: %v", i, err)
		}
		if env.GetAbilityUsed().GetAbilityId() == n.ability {
			used = true
		}
		if env.GetConditionApplied().GetConditionId() == n.cond {
			applied = true
		}
	}
	if !used || !applied {
		t.Fatalf("the batch carried the %d-byte ability id: %v, the %d-byte condition id: %v",
			len(n.ability), used, len(n.cond), applied)
	}
	held := false
	for _, c := range st.Conditions["qa-target"] {
		held = held || c.ID == n.cond
	}
	if !held {
		t.Fatalf("the folded state does not hold the %d-byte condition on the target", len(n.cond))
	}
}

func qaRuleSchemaDoc(t *testing.T, base string) map[string]any {
	t.Helper()
	var found string
	err := fs.WalkDir(rules.Schemas, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && path.Base(p) == base {
			found = p
		}
		return err
	})
	if err != nil || found == "" {
		t.Fatalf("no %s in rules.Schemas (walk error %v)", base, err)
	}
	raw, err := fs.ReadFile(rules.Schemas, found)
	if err != nil {
		t.Fatalf("read %s: %v", found, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %s: %v", found, err)
	}
	return doc
}

func qaRuleSchemaDescription(t *testing.T, base string, keys ...string) string {
	t.Helper()
	node := qaRuleSchemaDoc(t, base)
	for _, k := range keys {
		next, ok := node[k].(map[string]any)
		if !ok {
			t.Fatalf("%s has no %s on the path %v", base, k, keys)
		}
		node = next
	}
	desc, _ := node["description"].(string)
	return desc
}

var qaRuleStatedBound = regexp.MustCompile(`At most (\d+) bytes of UTF-8`)

func qaRuleAtBytes(field string, n int) string {
	switch field {
	case qaRuleFieldAttribute, qaRuleFieldDefense, qaRuleFieldResource:
		return qaRulePad("q", "a", n)
	}
	return strings.Repeat("é", n/2) + strings.Repeat("a", n%2)
}

// VTT-287
func TestQARuleTheSchemasStateTheBoundLoadEnforces(t *testing.T) {
	cases := []struct {
		field, schema string
		keys          []string
	}{
		{qaRuleFieldAbility, "ability.schema.json", []string{"properties", "id"}},
		{qaRuleFieldCondition, "condition.schema.json", []string{"properties", "id"}},
		{qaRuleFieldAttribute, "ruleset.schema.json", []string{"properties", "attributes"}},
		{qaRuleFieldDefense, "ruleset.schema.json", []string{"properties", "defenses"}},
		{qaRuleFieldResource, "ruleset.schema.json", []string{"properties", "resources", "items", "properties", "name"}},
		{qaRuleFieldBranch, "atom.schema.json", []string{"$defs", "resolutionContribution", "properties", "branches"}},
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			desc := qaRuleSchemaDescription(t, c.schema, c.keys...)
			m := qaRuleStatedBound.FindStringSubmatch(desc)
			if m == nil {
				t.Fatalf("%s states no byte bound for the %s: %q", c.schema, c.field, desc)
			}
			stated, _ := strconv.Atoi(m[1])
			if stated != qaRuleBound {
				t.Errorf("%s states %d bytes for the %s, want %d", c.schema, stated, c.field, qaRuleBound)
			}
			at := qaRuleAtBytes(c.field, stated)
			if len(at) != stated {
				t.Fatalf("fixture is %d bytes, want %d", len(at), stated)
			}
			s, _ := qaRuleAlone(c.field, at)
			if _, err := rules.Load(s.write(t)); err != nil {
				t.Errorf("rules.Load refused a %s of the stated %d bytes: %v", c.field, stated, err)
			}
			s, _ = qaRuleAlone(c.field, at+"a")
			if _, err := rules.Load(s.write(t)); err == nil {
				t.Errorf("rules.Load accepted a %s one byte over the stated %d", c.field, stated)
			}
		})
	}
}

const (
	qaRulePool   = "qa_pool"
	qaRuleMarker = "48611"
)

func qaRuleFailingThresholds() map[string]string {
	return map[string]string{
		"unknown attribute": "@qa_attr" + strings.Repeat(" + 0", 40) + " + " + qaRuleMarker,
		"division by zero":  qaRuleMarker + " / (#" + qaRulePool + " - #" + qaRulePool + ")",
	}
}

func qaRuleThresholdSet(t *testing.T, failing string, position int) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	thresholds := make([]map[string]any, 3)
	for i := range thresholds {
		thresholds[i] = map[string]any{
			"when": "#" + qaRulePool + " - #" + qaRulePool, "apply_condition": "qa-mark", "remove_when_false": false,
		}
	}
	thresholds[position]["when"] = failing
	qaRuleWrite(t, filepath.Join(dir, qaRuleManifestFile), map[string]any{
		"id": "qa-rules", "name": "QA Rules", "format_version": "2",
		"attributes": []string{"qa_attr"}, "defenses": []string{},
		"resources": []map[string]any{{"name": qaRulePool, "thresholds": thresholds}},
	})
	qaRuleWrite(t, filepath.Join(dir, "conditions", "qa-mark.json"), map[string]any{"id": "qa-mark", "name": "QA Mark"})
	qaRuleWrite(t, filepath.Join(dir, "atoms", "qa-self.json"), map[string]any{
		"id": "qa-self", "params": []any{}, "provides": []string{}, "consumes": []string{},
		"contributes": []map[string]any{{"kind": "targeting", "range": 0, "max_targets": 1}},
	})
	qaRuleWrite(t, filepath.Join(dir, "atoms", "qa-drain.json"), map[string]any{
		"id": "qa-drain", "params": []any{}, "provides": []string{}, "consumes": []string{},
		"contributes": []map[string]any{{
			"kind": "outcome", "key": nil, "branch": "always",
			"effects": []map[string]any{{"resource_change": map[string]any{"resource": qaRulePool, "delta_expr": "0 - 1"}}},
		}},
	})
	qaRuleWrite(t, filepath.Join(dir, "abilities", "qa-drain.json"), map[string]any{
		"id": "qa-drain", "name": "QA Drain", "usage": "at_will",
		"compose": []map[string]any{
			{"atom": "qa-self", "bind": map[string]any{}},
			{"atom": "qa-drain", "bind": map[string]any{}},
		},
	})
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("# QA\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

var qaRulePositionDigits = map[int]*regexp.Regexp{
	0: regexp.MustCompile(`\b[01]\b`),
	2: regexp.MustCompile(`\b[23]\b`),
}

// VTT-288
func TestQARuleAThresholdRefusalNamesTheResourceAndPositionNotTheExpression(t *testing.T) {
	for label, failing := range qaRuleFailingThresholds() {
		t.Run(label, func(t *testing.T) {
			msgs := map[int]string{}
			for _, position := range []int{0, 2} {
				rs, err := rules.Load(qaRuleThresholdSet(t, failing, position))
				if err != nil {
					t.Fatalf("rules.Load: %v", err)
				}
				st := qaRuleTable(t, qaRuleActor("qa-hero", nil, qaRulePool, 9))
				cmd := &vttv1.UseAbility{ActorId: "qa-hero", AbilityId: "qa-drain", TargetIds: []string{"qa-hero"}}
				envs, err := rules.Resolve(rs, st, cmd, qaRuleMaxRoller{})
				if err == nil {
					t.Fatalf("Resolve accepted a use whose threshold %d cannot be evaluated (%d events)", position, len(envs))
				}
				msg := err.Error()
				t.Logf("position %d refusal: %s", position, msg)
				if envs != nil {
					t.Errorf("Resolve returned %d events beside its refusal", len(envs))
				}
				if !strings.Contains(msg, qaRulePool) {
					t.Errorf("refusal %q does not name the resource %s", msg, qaRulePool)
				}
				if strings.Contains(msg, failing) || strings.Contains(msg, qaRuleMarker) {
					t.Errorf("refusal %q carries the threshold's expression", msg)
				}
				if !qaRulePositionDigits[position].MatchString(msg) {
					t.Errorf("refusal %q carries no position %d (0-based) or %d (1-based)", msg, position, position+1)
				}
				msgs[position] = msg
			}
			if msgs[0] == msgs[2] {
				t.Errorf("the refusal is the same whether the threshold is first or third: %q", msgs[0])
			}
		})
	}
}
