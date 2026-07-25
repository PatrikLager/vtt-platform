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
