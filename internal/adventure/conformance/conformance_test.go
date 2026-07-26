package conformance_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
)

func fixtureRulesetsRoot() string {
	return filepath.Join("testdata", "rulesets")
}

func fixture(name string) string {
	return filepath.Join("testdata", "adventures", name)
}

// TestRunValidAdventure is the full happy path: an adventure whose declared
// ruleset ("fixture-ruleset") resolves under testdata/rulesets, loads and
// validates cleanly, compiles against a brand-new engine.NewState(), and
// reproduces its goldens/compiled-batch.json exactly.
func TestRunValidAdventure(t *testing.T) {
	if err := conformance.Run(fixture("ok"), fixtureRulesetsRoot()); err != nil {
		t.Fatalf("Run(ok): unexpected error: %v", err)
	}
}

// TestRunMissingGolden proves a missing goldens/compiled-batch.json is a
// named failure (not a silent skip, not a panic) — the adventure otherwise
// loads and compiles fine.
func TestRunMissingGolden(t *testing.T) {
	err := conformance.Run(fixture("missing-golden"), fixtureRulesetsRoot())
	if err == nil {
		t.Fatal("Run(missing-golden): want error, got nil")
	}
	if !strings.Contains(err.Error(), "compiled-batch.json") {
		t.Errorf("Run(missing-golden) error = %q, want it to name the missing golden file", err.Error())
	}
}

// TestRunGoldenMismatch proves the golden comparison is exact, not vacuous:
// a goldens/compiled-batch.json that deliberately disagrees with what
// Compile actually produces (actor_added's resources.focus.max: 99 instead
// of 20) must fail Run, naming the first differing envelope index (index 2,
// the ActorAdded envelope — AdventureLoaded and SceneCreated both match).
func TestRunGoldenMismatch(t *testing.T) {
	err := conformance.Run(fixture("golden-mismatch"), fixtureRulesetsRoot())
	if err == nil {
		t.Fatal("Run(golden-mismatch): want error, got nil")
	}
	if !strings.Contains(err.Error(), "envelope[2]") {
		t.Errorf("Run(golden-mismatch) error = %q, want it to name the first differing index (envelope[2])", err.Error())
	}
}

// TestRunLoadError proves Run propagates a Load failure rather than
// panicking or silently succeeding: the adventure declares a ruleset id
// ("does-not-exist-ruleset") that resolves to no directory at all under
// rulesetsRoot.
func TestRunLoadError(t *testing.T) {
	err := conformance.Run(fixture("load-error"), fixtureRulesetsRoot())
	if err == nil {
		t.Fatal("Run(load-error): want error, got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist-ruleset") {
		t.Errorf("Run(load-error) error = %q, want it to name the unresolvable ruleset id", err.Error())
	}
}

// TestRunEmptyGuideRejected pins the spec §6 "Non-empty guide.md required"
// rule directly: an adventure that otherwise loads, validates, and compiles
// perfectly still fails Run if guide.md is a zero-byte file.
func TestRunEmptyGuideRejected(t *testing.T) {
	err := conformance.Run(fixture("empty-guide"), fixtureRulesetsRoot())
	if err == nil {
		t.Fatal("Run(empty-guide): want error, got nil")
	}
	if !strings.Contains(err.Error(), "guide.md") {
		t.Errorf("Run(empty-guide) error = %q, want it to name guide.md", err.Error())
	}
}

func TestRunUnknownDirectory(t *testing.T) {
	if err := conformance.Run(fixture("does-not-exist"), fixtureRulesetsRoot()); err == nil {
		t.Fatal("Run(does-not-exist): want error, got nil")
	}
}

// TestConformanceOverAdventuresGlob is the "forever" gate the task brief
// calls for (mirrors internal/rules/conformance_test.go's own
// TestConformanceOverRulesetsGlob, same three-levels-up repo-root
// discovery: internal/adventure/conformance -> adventure -> internal ->
// root): every adventure committed under adventures/ — cellar-rats and
// goblin-ambush today, any future adventure tomorrow — must pass Run
// untouched, resolving each one's declared ruleset from the real,
// top-level rulesets/ directory. Wired into `go test ./...` (and so `task
// check`) by simply existing in this suite; no separate Taskfile target
// needed.
func TestConformanceOverAdventuresGlob(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "..", "adventures", "*"))
	if err != nil {
		t.Fatalf("glob adventures/*: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("glob adventures/*: matched zero directories — expected at least cellar-rats and goblin-ambush")
	}
	rulesetsRoot := filepath.Join("..", "..", "..", "rulesets")
	for _, dir := range dirs {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			if err := conformance.Run(dir, rulesetsRoot); err != nil {
				t.Errorf("Run(%s): %v", dir, err)
			}
		})
	}
}
