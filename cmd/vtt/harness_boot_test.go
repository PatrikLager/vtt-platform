package main

// harness_boot_test.go covers resolveRulesetDir/findRepoRoot (harness_boot.go,
// ruleset-interpreter Task 6): the "relative to repo root" resolution rule
// bootSelfContained uses to turn a scenario's bare Ruleset id into a
// directory rules.Load can open.

import (
	"strings"
	"testing"
)

// TestResolveRulesetDirFindsCommittedTavernBrawl proves the resolution rule
// against the REAL committed rulesets/tavern-brawl directory: since `go
// test` sets this package's cwd to cmd/vtt itself (the Go toolchain's own
// convention — see resolveRulesetDir's doc comment), this test running at
// all IS the proof the walk-up-to-go.mod logic correctly reaches the repo
// root from a subdirectory two levels below it.
func TestResolveRulesetDirFindsCommittedTavernBrawl(t *testing.T) {
	dir, err := resolveRulesetDir("tavern-brawl")
	if err != nil {
		t.Fatalf("resolveRulesetDir(tavern-brawl): %v", err)
	}
	if !strings.HasSuffix(dir, "rulesets/tavern-brawl") && !strings.HasSuffix(dir, `rulesets\tavern-brawl`) {
		t.Fatalf("resolveRulesetDir(tavern-brawl) = %q, want it to end in rulesets/tavern-brawl", dir)
	}
}

// TestResolveRulesetDirUnknownIDErrorsCleanly proves an id with no matching
// rulesets/<id> directory is a named, clean error — not a panic or a
// silent empty string a later rules.Load call would fail on with a less
// specific message.
func TestResolveRulesetDirUnknownIDErrorsCleanly(t *testing.T) {
	_, err := resolveRulesetDir("no-such-ruleset-id")
	if err == nil {
		t.Fatal("want error for an unknown ruleset id")
	}
	if !strings.Contains(err.Error(), "no-such-ruleset-id") {
		t.Fatalf("error = %q, want it to name the unresolved id", err.Error())
	}
}
