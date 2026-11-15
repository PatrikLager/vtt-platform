package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

func qaRCDReadLog(t *testing.T, name string) []*vttv1.Envelope {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	out := make([]*vttv1.Envelope, 0, len(items))
	for i, it := range items {
		e := &vttv1.Envelope{}
		if err := protojson.Unmarshal(it, e); err != nil {
			t.Fatalf("%s[%d]: %v", name, i, err)
		}
		out = append(out, e)
	}
	return out
}

func qaRCDFold(t *testing.T, log []*vttv1.Envelope) (any, int64) {
	t.Helper()
	st, err := harness.Fold(log)
	if err != nil {
		t.Fatalf("the log is refused: %v", err)
	}
	var head int64
	for _, e := range log {
		head = max(head, e.GetSequence())
	}
	return st, head
}

// VTT-774: `vtt state dump`'s printer, handed the fold of a log of
// ResourceAdded and ResourceRemoved, prints the hand-derived dump the client's
// fold is held to in client/test/qa-resource-change.test.ts, byte for byte.
func TestQAResourceChangeLogDumpsAsTheSharedGolden(t *testing.T) {
	st, head := qaRCDFold(t, qaRCDReadLog(t, "qa_resource_change_log.json"))
	var buf bytes.Buffer
	if err := writeDump(&buf, st, head, nil); err != nil {
		t.Fatalf("writeDump: %v", err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "contract", "testdata", "qa_resource_change_dump.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if got != string(want) {
		t.Fatalf("vtt state dump of the QA log differs from the golden\n--- got\n%s\n--- want\n%s", got, want)
	}
}

// VTT-774 (refusals included): the server's fold refuses each envelope of
// qa_resource_change_refused.json after the log; the client test holds its
// fold to the same refusals.
func TestQAResourceChangeRefusalsAreRefusedAfterTheLog(t *testing.T) {
	log := qaRCDReadLog(t, "qa_resource_change_log.json")
	refused := qaRCDReadLog(t, "qa_resource_change_refused.json")
	if len(refused) == 0 {
		t.Fatal("the refused fixture holds no envelope, so this test would prove nothing")
	}
	for i, e := range refused {
		_, err := harness.Fold(append(append([]*vttv1.Envelope{}, log...), e))
		if err == nil {
			t.Errorf("refused[%d] %v accepted", i, e.GetPayload())
			continue
		}
		t.Logf("refused[%d]: %v", i, err)
	}
}
