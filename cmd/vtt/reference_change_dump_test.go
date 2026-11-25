package main

import (
	"os"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// VTT-1028
func TestTheDumpAfterReferenceChangesIsTheCommittedOne(t *testing.T) {
	got := foldedDump(t, readEnvelopes(t, "../../contract/testdata/reference_change_log.json"))
	want, err := os.ReadFile("../../contract/testdata/reference_change_dump.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("dump:\n%s\nwant:\n%s", got, want)
	}
}

// VTT-1028
func TestEachReferenceRefusalInTheCommittedFixtureTheServerRefuses(t *testing.T) {
	log := readEnvelopes(t, "../../contract/testdata/reference_change_log.json")
	refused := readEnvelopes(t, "../../contract/testdata/reference_change_refused.json")
	if len(refused) == 0 {
		t.Fatal("the refused fixture holds no envelope, so this test would prove nothing")
	}
	for i, env := range refused {
		if _, err := harness.Fold(append(append([]*vttv1.Envelope{}, log...), env)); err == nil {
			t.Errorf("refused envelope %d (%v) folded onto the log", i, env.GetPayload())
		}
	}
}
