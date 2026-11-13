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

const qaAbUnheldReason = `, which the ruleset does not hold`

func qaAbRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load("testdata/ruleset")
	if err != nil {
		t.Fatalf("rules.Load(testdata/ruleset): %v", err)
	}
	for _, id := range []string{"Alpha", "beta", "gamma"} {
		rs.Compiled[id] = &rules.CompiledPower{ID: id, Name: id}
	}
	return rs
}

func qaAbCopyValid(t *testing.T) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "valid")
	if err := os.CopyFS(dst, os.DirFS("testdata/valid")); err != nil {
		t.Fatalf("copy testdata/valid: %v", err)
	}
	return dst
}

func qaAbEditActor(t *testing.T, dir, file string, edit func(m map[string]any)) {
	t.Helper()
	p := filepath.Join(dir, "actors", file)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode %s: %v", p, err)
	}
	edit(m)
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("encode %s: %v", p, err)
	}
	if err := os.WriteFile(p, out, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func qaAbSetAbilities(ids any) func(m map[string]any) {
	return func(m map[string]any) { m["ability_ids"] = ids }
}

func qaAbActor(t *testing.T, adv *adventure.Adventure, id string) adventure.AdventureActor {
	t.Helper()
	for _, a := range adv.Actors {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("adventure holds no actor %q", id)
	return adventure.AdventureActor{}
}

func qaAbCompiledAbilities(t *testing.T, adv *adventure.Adventure, id string) []string {
	t.Helper()
	envs, _, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	var found [][]string
	for _, env := range envs {
		if added := env.GetActorAdded(); added != nil && added.GetActor().GetActorId() == id {
			found = append(found, added.GetActor().GetAbilityIds())
		}
	}
	if len(found) != 1 {
		t.Fatalf("Compile gave %d ActorAdded for %q, want 1", len(found), id)
	}
	return found[0]
}

func qaAbWantRefusal(t *testing.T, err error, file, id string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load = nil error, want a refusal naming %s and %q", file, id)
	}
	msg := err.Error()
	t.Logf("refusal: %s", msg)
	for _, want := range []string{
		filepath.Join("actors", file),
		`field "ability_ids"`,
		`names ability "` + id + `"` + qaAbUnheldReason,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("Load error %q does not contain %q", msg, want)
		}
	}
}

func qaAbWantOtherRefusal(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Load = nil error, want a refusal")
	}
	t.Logf("refusal: %s", err)
	if strings.Contains(err.Error(), "ability_ids") || strings.Contains(err.Error(), qaAbUnheldReason) {
		t.Fatalf("Load error %q names the ability list, want the earlier refusal", err)
	}
}

// VTT-684
func TestQAAbAnActorFilesAbilityIdsLoadAndCompileInTheirOrder(t *testing.T) {
	rs := qaAbRuleset(t)
	dir := qaAbCopyValid(t)
	want := []string{"gamma", "Alpha", "beta"}
	qaAbEditActor(t, dir, "vim-fighter.json", qaAbSetAbilities(want))
	adv, err := adventure.Load(dir, rs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := qaAbActor(t, adv, "vim-fighter").AbilityIDs; !slices.Equal(got, want) {
		t.Fatalf("loaded AbilityIDs = %q, want %q", got, want)
	}
	if got := qaAbCompiledAbilities(t, adv, "vim-fighter"); !slices.Equal(got, want) {
		t.Fatalf("ActorAdded ability_ids = %q, want %q", got, want)
	}
	if got := qaAbCompiledAbilities(t, adv, "brace-guard"); len(got) != 0 {
		t.Fatalf("ActorAdded ability_ids for an actor naming none = %q, want none", got)
	}
}

// VTT-684
func TestQAAbARepeatedHeldIdLoadsAndCompilesAsWritten(t *testing.T) {
	rs := qaAbRuleset(t)
	dir := qaAbCopyValid(t)
	want := []string{"beta", "Alpha", "beta"}
	qaAbEditActor(t, dir, "grit-scout.json", qaAbSetAbilities(want))
	adv, err := adventure.Load(dir, rs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := qaAbActor(t, adv, "grit-scout").AbilityIDs; !slices.Equal(got, want) {
		t.Fatalf("loaded AbilityIDs = %q, want %q", got, want)
	}
	if got := qaAbCompiledAbilities(t, adv, "grit-scout"); !slices.Equal(got, want) {
		t.Fatalf("ActorAdded ability_ids = %q, want %q", got, want)
	}
}

// VTT-684
func TestQAAbAListWrittenNullLoadsAsNone(t *testing.T) {
	rs := qaAbRuleset(t)
	dir := qaAbCopyValid(t)
	qaAbEditActor(t, dir, "grit-scout.json", qaAbSetAbilities(nil))
	qaAbEditActor(t, dir, "vim-fighter.json", qaAbSetAbilities([]string{"beta"}))
	adv, err := adventure.Load(dir, rs)
	if err != nil {
		t.Fatalf("Load with ability_ids null: %v", err)
	}
	if got := qaAbActor(t, adv, "grit-scout").AbilityIDs; len(got) != 0 {
		t.Fatalf("loaded AbilityIDs for null = %q, want none", got)
	}
	if got := qaAbCompiledAbilities(t, adv, "grit-scout"); len(got) != 0 {
		t.Fatalf("ActorAdded ability_ids for null = %q, want none", got)
	}
	if got := qaAbCompiledAbilities(t, adv, "vim-fighter"); !slices.Equal(got, []string{"beta"}) {
		t.Fatalf("ActorAdded ability_ids beside a null list = %q, want [beta]", got)
	}
}

// VTT-687
func TestQAAbAnUnheldIdIsRefusedNamingTheFileFieldAndFirstIdInByteOrder(t *testing.T) {
	rs := qaAbRuleset(t)
	dir := qaAbCopyValid(t)
	qaAbEditActor(t, dir, "vim-fighter.json", qaAbSetAbilities([]string{"zz-unheld", "beta", "alpha-unheld", "Yak-unheld", "gamma"}))
	adv, err := adventure.Load(dir, rs)
	qaAbWantRefusal(t, err, "vim-fighter.json", "Yak-unheld")
	if adv != nil {
		t.Fatalf("Load returned an adventure beside its refusal")
	}
}

// VTT-687
func TestQAAbAHeldIdBesideAnUnheldOneIsNotNamed(t *testing.T) {
	rs := qaAbRuleset(t)
	dir := qaAbCopyValid(t)
	qaAbEditActor(t, dir, "grit-scout.json", qaAbSetAbilities([]string{"Alpha", "beta", "delta"}))
	_, err := adventure.Load(dir, rs)
	qaAbWantRefusal(t, err, "grit-scout.json", "delta")
}

// VTT-687
func TestQAAbAnIdDifferingFromAHeldOneOnlyInCaseIsRefused(t *testing.T) {
	rs := qaAbRuleset(t)
	dir := qaAbCopyValid(t)
	qaAbEditActor(t, dir, "brace-guard.json", qaAbSetAbilities([]string{"alpha"}))
	_, err := adventure.Load(dir, rs)
	qaAbWantRefusal(t, err, "brace-guard.json", "alpha")
}

// VTT-687
func TestQAAbTheEarlierRefusalsOfAnActorComeBeforeItsAbilities(t *testing.T) {
	longName := strings.Repeat("n", 300)
	cases := []struct {
		name string
		edit func(m map[string]any)
	}{
		{"no id", func(m map[string]any) { delete(m, "actor_id") }},
		{"a name over its bound", func(m map[string]any) { m["name"] = longName }},
		{"no kind", func(m map[string]any) { delete(m, "kind") }},
		{"an undeclared key", func(m map[string]any) {
			m["attributes"].(map[string]any)["qa-undeclared"] = 3
		}},
		{"a resource under the numbers", func(m map[string]any) {
			m["attributes"].(map[string]any)["focus"] = 3
		}},
		{"a number written null", func(m map[string]any) {
			m["attributes"].(map[string]any)["vim"] = nil
		}},
		{"a current left out", func(m map[string]any) {
			m["resources"] = map[string]any{"focus": map[string]any{"max": 10}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := qaAbRuleset(t)
			dir := qaAbCopyValid(t)
			qaAbEditActor(t, dir, "vim-fighter.json", func(m map[string]any) {
				tc.edit(m)
				m["ability_ids"] = []string{"beta"}
			})
			_, heldErr := adventure.Load(dir, rs)
			qaAbWantOtherRefusal(t, heldErr)
			qaAbEditActor(t, dir, "vim-fighter.json", qaAbSetAbilities([]string{"zz-unheld"}))
			_, err := adventure.Load(dir, rs)
			qaAbWantOtherRefusal(t, err)
			if err.Error() != heldErr.Error() {
				t.Fatalf("refusal with an unheld id = %q, want the refusal with a held one, %q", err, heldErr)
			}
		})
	}
}
