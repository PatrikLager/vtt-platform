package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// TestScenarioGoldenStreamsHaveNotDrifted re-runs each corpus scenario
// against a real self-contained server and asserts the normalized event
// stream still equals the committed scenarios/goldens/<name>/stream.json.
//
// THERE IS DELIBERATELY NO -update FLAG. The original plan for this task
// generated the corpus behind one, and that was rejected: the rule shipped
// in internal/adventure/conformance says to "derive a golden by hand FIRST
// (ADR-009) ... never to generate a golden no human derived first". A
// regenerate-on-demand switch is exactly how a golden stops being a claim
// anyone checked. When this test fails, read the diff and decide whether the
// SERVER changed (fix it) or the corpus is legitimately stale (re-derive the
// state by hand, then re-record the stream).
//
// Division of labour with internal/harness's TestFoldGoldenCorpus:
//   - stream.json is a recorded observation of the server; THIS test pins it.
//   - state.json is hand-derived from the scenario definition; the fold gate
//     checks it. Neither file was produced from the other, which is what
//     makes their agreement evidence rather than a tautology.
func TestScenarioGoldenStreamsHaveNotDrifted(t *testing.T) {
	dirs := goldenDirs(t)

	for _, dir := range dirs {
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(dir, "stream.json"))
			if err != nil {
				t.Fatal(err)
			}
			got := captureNormalizedStream(t, name)
			if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
				t.Errorf("recorded stream differs from the committed golden.\n"+
					"Either the server's emitted events changed (a real regression, or a "+
					"deliberate change needing the hand-derived state.json re-derived too), "+
					"or the normalization drifted.\n--- recorded ---\n%s\n--- committed ---\n%s",
					got, want)
			}
		})
	}
}

// captureNormalizedStream boots a throwaway server, runs the scenario, and
// returns its event stream with every per-run value normalized.
//
// Normalization (client plan's contract, plus one field the plan missed):
//
//	eventId       -> "evt-<sequence>"
//	occurredAt    -> omitted
//	sessionId     -> "sess-N", N in order of first appearance
//	participant   -> "p-<name>" — EVERYWHERE the id appears, not only in the
//	                 envelope's participantId field. Actor.controller_id
//	                 carries one inside the payload, which the plan's
//	                 four-field table did not cover; three-role-exit and
//	                 story-table both set it, and without this the stream
//	                 differed on every capture and this drift check could
//	                 never have gone green.
func captureNormalizedStream(t *testing.T, scenario string) []byte {
	t.Helper()

	sc, err := harness.LoadScenario(filepath.Join("../../scenarios", scenario+".json"))
	if err != nil {
		t.Fatal(err)
	}
	boot, err := bootSelfContained(sc)
	if err != nil {
		t.Fatal(err)
	}
	defer boot.close()

	rep, err := harness.RunScenario(context.Background(), sc, dialerFor(boot.WSURL, boot.Tokens), boot.IDs, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Pass {
		t.Fatalf("scenario %q did not pass, so its stream is not a valid golden: %+v", scenario, rep.Steps)
	}

	observer := sc.Participants[0].Name
	c, err := harness.Dial(context.Background(), boot.WSURL, boot.Tokens[observer], 0)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	events := drainQuiescent(c.Events(), dumpQuietWindow)

	idToName := make(map[string]string, len(boot.IDs))
	for n, id := range boot.IDs {
		idToName[id] = n
	}
	sessionIDs := map[string]string{}

	out := make([]json.RawMessage, 0, len(events))
	for _, env := range events {
		e := &vttv1.Envelope{}
		raw, err := protojson.Marshal(env)
		if err != nil {
			t.Fatal(err)
		}
		if err := protojson.Unmarshal(raw, e); err != nil {
			t.Fatal(err)
		}
		e.EventId = fmt.Sprintf("evt-%d", e.Sequence)
		e.OccurredAt = nil
		if e.SessionId != "" {
			if _, seen := sessionIDs[e.SessionId]; !seen {
				sessionIDs[e.SessionId] = fmt.Sprintf("sess-%d", len(sessionIDs)+1)
			}
			e.SessionId = sessionIDs[e.SessionId]
		}
		norm, err := protojson.MarshalOptions{Multiline: false}.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		for id, n := range idToName {
			norm = bytes.ReplaceAll(norm, []byte(id), []byte("p-"+n))
		}
		out = append(out, norm)
	}

	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(buf, '\n')
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
