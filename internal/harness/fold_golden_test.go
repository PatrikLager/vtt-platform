package harness_test

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

// TestFoldGoldenCorpus is the Go half of the cross-language fold-parity
// keystone (client spec §6); the TS half will assert the SAME corpus.
//
// The two files in each golden directory are derived INDEPENDENTLY, and that
// independence is the whole point:
//
//   - state.json is HAND-DERIVED from the scenario definition, by reasoning
//     about what each step does to the fold. ADR-009 and
//     internal/adventure/conformance's rule both forbid a golden the machine
//     produced and no human derived — "never to generate a golden no human
//     derived first".
//   - stream.json is a RECORDED observation of the real server, normalized
//     (see cmd/vtt/scenario_goldens_test.go for the normalization and the
//     drift check that keeps it honest). A recording is a fixture, not an
//     assertion.
//
// So this test is not a tautology: it checks a machine-captured input
// against a human-derived expectation. If the recording were wrong, folding
// it would not reproduce a state derived without looking at it.
func TestFoldGoldenCorpus(t *testing.T) {
	dirs := goldenDirs(t)

	for _, dir := range dirs {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			rawStream, err := os.ReadFile(filepath.Join(dir, "stream.json"))
			if err != nil {
				t.Fatal(err)
			}
			var msgs []json.RawMessage
			if err := json.Unmarshal(rawStream, &msgs); err != nil {
				t.Fatalf("stream.json: %v", err)
			}
			events := make([]*vttv1.Envelope, 0, len(msgs))
			for i, m := range msgs {
				env := &vttv1.Envelope{}
				if err := protojson.Unmarshal(m, env); err != nil {
					t.Fatalf("stream.json[%d]: %v", i, err)
				}
				events = append(events, env)
			}

			st, err := harness.Fold(events)
			if err != nil {
				t.Fatalf("Fold: %v", err)
			}

			var head int64
			for _, e := range events {
				if e.Sequence > head {
					head = e.Sequence
				}
			}

			got, err := dumpJSON(st, head)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join(dir, "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
				t.Errorf("folded state != hand-derived state.json\n--- got ---\n%s\n--- want ---\n%s",
					got, want)
			}
		})
	}
}

// dumpJSON mirrors cmd/vtt's writeDump byte-for-byte: the state's own fields
// plus headSequence, two-space indented. Duplicated rather than exported
// because cmd/vtt is package main and this gate must not reach into it.
func dumpJSON(st any, head int64) ([]byte, error) {
	raw, err := json.Marshal(st)
	if err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	headRaw, err := json.Marshal(head)
	if err != nil {
		return nil, err
	}
	fields["headSequence"] = headRaw
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fields); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// goldenDirs returns each corpus scenario directory. It filters to
// DIRECTORIES deliberately: a plain `filepath.Glob("*")` also matched the
// corpus README the moment one was added, and the gate failed trying to read
// README.md/stream.json.
func goldenDirs(t *testing.T) []string {
	t.Helper()
	entries, err := filepath.Glob("../../scenarios/goldens/*")
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		info, err := os.Stat(e)
		if err != nil {
			t.Fatal(err)
		}
		if info.IsDir() {
			dirs = append(dirs, e)
		}
	}
	if len(dirs) == 0 {
		t.Fatal("no golden scenario directories found — an empty corpus must fail rather " +
			"than vacuously pass")
	}
	return dirs
}
