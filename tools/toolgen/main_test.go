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

// Every command message must have a manifest entry — forgetting one means the
// LLM silently loses a capability. Two registries are checked: legacy
// "Request"-suffixed messages (pre-ClientCommand convention), and every
// message that appears as a ClientCommand oneof variant — the latter IS the
// command registry now that commands are imperative-named (CreateScene, not
// CreateSceneRequest) and dispatched through ClientCommand's oneof.
func TestManifestCoversAllCommandMessages(t *testing.T) {
	msgs := vttv1.File_vtt_v1_commands_proto.Messages()
	for i := 0; i < msgs.Len(); i++ {
		name := string(msgs.Get(i).FullName())
		if !strings.HasSuffix(name, "Request") {
			continue
		}
		requireManifestEntry(t, name)
	}

	cc := (&vttv1.ClientCommand{}).ProtoReflect().Descriptor()
	fields := cc.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		oo := f.ContainingOneof()
		if oo == nil || oo.IsSynthetic() {
			continue // not a command oneof variant (e.g. request_id)
		}
		requireManifestEntry(t, string(f.Message().FullName()))
	}
}

func requireManifestEntry(t *testing.T, message string) {
	t.Helper()
	for _, spec := range manifest {
		if spec.message == message {
			return
		}
	}
	t.Fatalf("command message %s has no toolgen manifest entry", message)
}
