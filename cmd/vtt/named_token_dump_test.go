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

// TestTheDumpOfANamedTokenCarriesItsTexts folds the log client/test's
// fold-unit dumps too, and holds writeDump to the same committed bytes.
func TestTheDumpOfANamedTokenCarriesItsTexts(t *testing.T) {
	raw, err := os.ReadFile("../../contract/testdata/named_token_log.json")
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
	st, err := harness.Fold(envs)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := writeDump(&got, st, headSequence(envs), nil); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../contract/testdata/named_token_dump.json")
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != string(want) {
		t.Fatalf("dump:\n%s\nwant:\n%s", got.String(), want)
	}
}
