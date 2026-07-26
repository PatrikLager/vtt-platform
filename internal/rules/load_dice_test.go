package rules_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// TestLoadRejectsDiceInThresholdWhen pins F8: a threshold's "when" expression
// is evaluated with the live Roller (via plain Eval, not evalRecording), so
// any dice it rolls are consumed but never recorded on AbilityUsed.rolls —
// a silent violation of spec §2 decision 3's "rolled once, recorded forever"
// testimony contract. v1 restriction: reject dice in "when" at LOAD time,
// naming file+field.
func TestLoadRejectsDiceInThresholdWhen(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "2",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    {
      "name": "pool_a",
      "default_max_expr": "10 + @brawn",
      "thresholds": [
        { "when": "#pool_a + 1d6", "apply_condition": "guarded", "remove_when_false": true }
      ]
    }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with dice in a threshold 'when': want error, got nil")
	}
	if !strings.Contains(err.Error(), "dice") || !strings.Contains(err.Error(), "when") {
		t.Errorf("error = %q, want it to name the 'when' field and mention dice", err.Error())
	}
}

// TestLoadRejectsDiceInDefaultMaxExpr pins F8 for the other dice-bearing
// expression position: default_max_expr must also reject dice at load time
// (same testimony/authoring rationale as the threshold "when").
func TestLoadRejectsDiceInDefaultMaxExpr(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "2",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    {
      "name": "pool_a",
      "default_max_expr": "10 + 1d6",
      "thresholds": [
        { "when": "#pool_a", "apply_condition": "guarded", "remove_when_false": true }
      ]
    }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with dice in default_max_expr: want error, got nil")
	}
	if !strings.Contains(err.Error(), "dice") || !strings.Contains(err.Error(), "default_max_expr") {
		t.Errorf("error = %q, want it to name the 'default_max_expr' field and mention dice", err.Error())
	}
}

// TestLoadResolutionContributionDeterministicMissingField pins finding R2:
// when a resolution contribution is missing more than one of key/roll/vs,
// WHICH field the load error names must be deterministic across process
// runs. The presence check formerly ranged a Go map literal, so Go's
// randomized map iteration surfaced "roll" on some runs and "vs" on
// others for the same atom — a CI/log-diff flake this package's own
// determinism discipline forbids. Loading the same missing-roll-and-vs atom
// many times in one process must report the SAME field every time (the
// first in fixed key/roll/vs order that is absent — here "roll"), the same
// way TestCompileDeterministic pins compile output.
func TestLoadResolutionContributionDeterministicMissingField(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	// A resolution contribution with key + branches but NEITHER roll NOR vs:
	// two required fields missing, so the reported one depends on iteration
	// order unless it is fixed.
	writeFile(t, filepath.Join(dir, "atoms", "twomiss.json"), `{
  "id": "twomiss",
  "params": [],
  "provides": ["x"],
  "consumes": [],
  "contributes": [
    { "kind": "resolution", "key": "x", "branches": ["hit", "miss"] }
  ]
}`)

	var firstField string
	for i := 0; i < 50; i++ {
		_, err := rules.Load(dir)
		if err == nil {
			t.Fatalf("run %d: Load with a resolution missing roll and vs: want error, got nil", i)
		}
		msg := err.Error()
		var field string
		switch {
		case strings.Contains(msg, "roll"):
			field = "roll"
		case strings.Contains(msg, "vs"):
			field = "vs"
		default:
			t.Fatalf("run %d: error %q names neither roll nor vs", i, msg)
		}
		if i == 0 {
			firstField = field
			continue
		}
		if field != firstField {
			t.Fatalf("run %d: error named %q, but run 0 named %q — nondeterministic field choice on a multi-missing resolution contribution", i, field, firstField)
		}
	}
	if firstField != "roll" {
		t.Errorf("reported field = %q, want %q (the first absent field in fixed key/roll/vs order)", firstField, "roll")
	}
}

// TestLoadRejectsExpressionSizedDiceInThresholdWhen is R4's expression-sized
// counterpart to TestLoadRejectsDiceInThresholdWhen: format v2 adds
// expression-sized dice ("1d(@brawn)"), which must be rejected in a
// threshold "when" exactly like literal dice — the rolled-once-recorded-
// forever contract does not care whether the die's sides are a literal or
// an expression. Pins that the load-side ban goes through Expr.HasDice's
// expression-sized path, not just its literal-dice path.
func TestLoadRejectsExpressionSizedDiceInThresholdWhen(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "2",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    {
      "name": "pool_a",
      "default_max_expr": "10 + @brawn",
      "thresholds": [
        { "when": "#pool_a + 1d(@brawn)", "apply_condition": "guarded", "remove_when_false": true }
      ]
    }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with expression-sized dice in a threshold 'when': want error, got nil")
	}
	if !strings.Contains(err.Error(), "dice") || !strings.Contains(err.Error(), "when") {
		t.Errorf("error = %q, want it to name the 'when' field and mention dice", err.Error())
	}
}

// TestLoadRejectsExpressionSizedDiceInDefaultMaxExpr is R4's expression-
// sized counterpart to TestLoadRejectsDiceInDefaultMaxExpr.
func TestLoadRejectsExpressionSizedDiceInDefaultMaxExpr(t *testing.T) {
	dir := t.TempDir()
	copyDir(t, fixture(t, "valid"), dir)
	writeFile(t, filepath.Join(dir, "ruleset.json"), `{
  "id": "test-ruleset",
  "name": "Test Ruleset",
  "format_version": "2",
  "attributes": ["brawn", "grit"],
  "defenses": ["guard"],
  "resources": [
    {
      "name": "pool_a",
      "default_max_expr": "10 + 1d(@brawn)",
      "thresholds": [
        { "when": "#pool_a", "apply_condition": "guarded", "remove_when_false": true }
      ]
    }
  ]
}`)
	_, err := rules.Load(dir)
	if err == nil {
		t.Fatal("Load with expression-sized dice in default_max_expr: want error, got nil")
	}
	if !strings.Contains(err.Error(), "dice") || !strings.Contains(err.Error(), "default_max_expr") {
		t.Errorf("error = %q, want it to name the 'default_max_expr' field and mention dice", err.Error())
	}
}
