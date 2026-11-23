package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

func foldedDump(t *testing.T, envs []*vttv1.Envelope) string {
	t.Helper()
	st, err := harness.Fold(envs)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := writeDump(&got, st, headSequence(envs), nil); err != nil {
		t.Fatal(err)
	}
	return got.String()
}

// VTT-979 VTT-983
func TestTheDumpAfterRulesetsNamedIsTheCommittedOne(t *testing.T) {
	got := foldedDump(t, readEnvelopes(t, "../../contract/testdata/ruleset_named_log.json"))
	want, err := os.ReadFile("../../contract/testdata/ruleset_named_dump.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("dump:\n%s\nwant:\n%s", got, want)
	}
}

// VTT-987
func TestTheDumpOfALogThatNamesNoRulesetWritesNothingOfOne(t *testing.T) {
	envs := readEnvelopes(t, "../../contract/testdata/ruleset_named_log.json")
	if envs[2].GetRulesetNamed() == nil {
		t.Fatal("the fixture's third entry is no RulesetNamed, so its first two prove nothing")
	}
	if got := foldedDump(t, envs[:2]); strings.Contains(got, "Ruleset") {
		t.Errorf("the dump of a log naming no ruleset writes one:\n%s", got)
	}
}

// VTT-974 VTT-983
func TestEachRulesetNamedRefusalInTheCommittedFixtureTheServerRefuses(t *testing.T) {
	log := readEnvelopes(t, "../../contract/testdata/ruleset_named_log.json")
	refused := readEnvelopes(t, "../../contract/testdata/ruleset_named_refused.json")
	if len(refused) == 0 {
		t.Fatal("the refused fixture holds no envelope, so this test would prove nothing")
	}
	for i, env := range refused {
		if _, err := harness.Fold(append(append([]*vttv1.Envelope{}, log...), env)); err == nil {
			t.Errorf("refused envelope %d (%v) folded onto the log", i, env.GetPayload())
		}
	}
}
