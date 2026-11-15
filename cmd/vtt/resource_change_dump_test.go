package main

import (
	"bytes"
	"os"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// VTT-774
func TestTheDumpAfterResourcesAddedAndRemovedIsTheCommittedOne(t *testing.T) {
	envs := readEnvelopes(t, "../../contract/testdata/resource_change_log.json")
	st, err := harness.Fold(envs)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := writeDump(&got, st, headSequence(envs), nil); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../contract/testdata/resource_change_dump.json")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("dump:\n%s\nwant:\n%s", got.String(), want)
	}
}

// VTT-774
func TestEachResourceRefusalInTheCommittedFixtureTheServerRefuses(t *testing.T) {
	log := readEnvelopes(t, "../../contract/testdata/resource_change_log.json")
	refused := readEnvelopes(t, "../../contract/testdata/resource_change_refused.json")
	if len(refused) == 0 {
		t.Fatal("the refused fixture holds no envelope, so this test would prove nothing")
	}
	for i, env := range refused {
		if _, err := harness.Fold(append(append([]*vttv1.Envelope{}, log...), env)); err == nil {
			t.Errorf("refused envelope %d (%v) folded onto the log", i, env.GetPayload())
		}
	}
}
