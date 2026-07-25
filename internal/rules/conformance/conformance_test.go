package conformance_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules/conformance"
)

func fixture(elem ...string) string {
	return filepath.Join(append([]string{"testdata"}, elem...)...)
}

// TestRunValidRuleset is the full happy path: a ruleset that loads, whose
// one ability resolves against the generic fixture actor, and whose two
// golden fixtures (one hit, one out-of-range rejection) both reproduce
// exactly.
func TestRunValidRuleset(t *testing.T) {
	if err := conformance.Run(fixture("minimal")); err != nil {
		t.Fatalf("Run(minimal): unexpected error: %v", err)
	}
}

// TestRunLoadError proves Run propagates a Load failure rather than
// panicking or silently succeeding.
func TestRunLoadError(t *testing.T) {
	err := conformance.Run(fixture("minimal-load-error"))
	if err == nil {
		t.Fatal("Run(minimal-load-error): want error, got nil")
	}
	if !strings.Contains(err.Error(), "id") {
		t.Errorf("Run(minimal-load-error) error = %q, want it to mention the missing %q field", err.Error(), "id")
	}
}

// TestRunSmokeFailure proves the smoke pass — every declared ability must
// resolve against the manifest-generated fixture — actually runs and
// actually fails when an ability can never succeed against it (a
// limited-use cost that exceeds the resource's own default_max_expr).
func TestRunSmokeFailure(t *testing.T) {
	err := conformance.Run(fixture("minimal-smoke-fail"))
	if err == nil {
		t.Fatal("Run(minimal-smoke-fail): want error, got nil")
	}
	if !strings.Contains(err.Error(), "big-move") {
		t.Errorf("Run(minimal-smoke-fail) error = %q, want it to name the failing ability %q", err.Error(), "big-move")
	}
}

// TestRunGoldenMismatch proves golden comparison is exact, not vacuous: a
// golden fixture whose want_events deliberately disagrees with what
// Resolve actually produces must fail Run.
func TestRunGoldenMismatch(t *testing.T) {
	err := conformance.Run(fixture("minimal-golden-mismatch"))
	if err == nil {
		t.Fatal("Run(minimal-golden-mismatch): want error, got nil")
	}
	if !strings.Contains(err.Error(), "poke-hit-wrong") {
		t.Errorf("Run(minimal-golden-mismatch) error = %q, want it to name the failing golden %q", err.Error(), "poke-hit-wrong")
	}
}

func TestRunUnknownDirectory(t *testing.T) {
	if err := conformance.Run(fixture("does-not-exist")); err == nil {
		t.Fatal("Run(does-not-exist): want error, got nil")
	}
}

// TestRunMissingPerAbilityGolden pins F7: spec §8 mandates a golden scenario
// per ability, so a ruleset that loads and smoke-passes but ships no golden
// for one of its abilities must fail Run, naming the uncovered ability —
// otherwise the P4 forever-gate silently loses its pins to a rename, a typo,
// or a deletion, and 5b's dnd45e-minimal could satisfy "the SAME suite" with
// no golden coverage at all.
func TestRunMissingPerAbilityGolden(t *testing.T) {
	err := conformance.Run(fixture("minimal-missing-golden"))
	if err == nil {
		t.Fatal("Run(minimal-missing-golden): want error for the ability with no golden, got nil")
	}
	if !strings.Contains(err.Error(), "prod") {
		t.Errorf("Run(minimal-missing-golden) error = %q, want it to name the uncovered ability %q", err.Error(), "prod")
	}
}

// TestConformanceOverRulesetsGlob is the "forever" gate the task brief
// calls for: every ruleset committed under rulesets/ — today just
// tavern-brawl, tomorrow also 5b's dnd45e-minimal — must pass Run
// untouched. Wired into `go test ./...` (and so `task check`) by simply
// existing in this suite; no separate Taskfile target needed.
func TestConformanceOverRulesetsGlob(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "..", "rulesets", "*"))
	if err != nil {
		t.Fatalf("glob rulesets/*: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("glob rulesets/*: matched zero directories — expected at least tavern-brawl")
	}
	for _, dir := range dirs {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			if err := conformance.Run(dir); err != nil {
				t.Errorf("Run(%s): %v", dir, err)
			}
		})
	}
}
