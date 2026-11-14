package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// readEnvelopes decodes a testdata file holding a JSON array of envelopes.
func readEnvelopes(t *testing.T, path string) []*vttv1.Envelope {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	envs := make([]*vttv1.Envelope, 0, len(items))
	for _, it := range items {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(it, env); err != nil {
			t.Fatal(err)
		}
		envs = append(envs, env)
	}
	return envs
}

// VTT-727
func TestTheDumpAfterGivesAndTakesIsTheCommittedOne(t *testing.T) {
	envs := readEnvelopes(t, "../../contract/testdata/ability_change_log.json")
	st, err := harness.Fold(envs)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := writeDump(&got, st, headSequence(envs), nil); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../contract/testdata/ability_change_dump.json")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("dump:\n%s\nwant:\n%s", got.String(), want)
	}
}

// VTT-727
func TestEachRefusalInTheCommittedFixtureTheServerRefuses(t *testing.T) {
	log := readEnvelopes(t, "../../contract/testdata/ability_change_log.json")
	refused := readEnvelopes(t, "../../contract/testdata/ability_change_refused.json")
	if len(refused) == 0 {
		t.Fatal("the refused fixture holds no envelope, so this test would prove nothing")
	}
	for i, env := range refused {
		if _, err := harness.Fold(append(append([]*vttv1.Envelope{}, log...), env)); err == nil {
			t.Errorf("refused envelope %d (%v) folded onto the log", i, env.GetPayload())
		}
	}
}
