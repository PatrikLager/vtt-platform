package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func TestToolsMatchGolden(t *testing.T) {
	raw, err := os.ReadFile("../../contract/testdata/expected_tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var want []map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(buildTools())
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(gotJSON, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("tools mismatch:\nwant %v\ngot  %v", want, got)
	}
}

// Every command message (name ending in "Request") must have a manifest entry —
// forgetting one means the LLM silently loses a capability.
func TestManifestCoversAllCommandMessages(t *testing.T) {
	msgs := vttv1.File_vtt_v1_commands_proto.Messages()
	for i := 0; i < msgs.Len(); i++ {
		name := string(msgs.Get(i).FullName())
		if !strings.HasSuffix(name, "Request") {
			continue
		}
		found := false
		for _, spec := range manifest {
			if spec.message == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("command message %s has no toolgen manifest entry", name)
		}
	}
}
