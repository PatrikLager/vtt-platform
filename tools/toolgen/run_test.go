package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run is the generator's whole job: render buildTools() as indented JSON and
// put it either on a writer or on disk. Its output IS cmd/vtt/tools.json,
// which the drift gate pins and which defines all 13 MCP command tools — a
// silent failure here ships wrong tool definitions to the seated agent.
//
// Note the deleted sibling: valueSchema (a one-line wrapper over
// valueSchemaWithOverrides) had zero call sites anywhere in the module. It
// was removed rather than tested — a test for unreachable code raises the
// coverage number without pinning any behavior.

func TestRunWritesToWriterWhenNoPath(t *testing.T) {
	var buf bytes.Buffer
	if err := run("", &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("want output on the writer")
	}
	var tools []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &tools); err != nil {
		t.Fatalf("output must be valid JSON: %v", err)
	}
	if len(tools) == 0 {
		t.Fatal("want at least one tool definition")
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		t.Error("output must end with a newline (the committed file does)")
	}
}

func TestRunWritesToPathAndMatchesWriterOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tools.json")
	if err := run(path, nil); err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := run("", &buf); err != nil {
		t.Fatal(err)
	}

	// Both paths must render byte-identically: the drift gate compares the
	// on-disk copy, so a divergence between them would make the gate
	// inspect something other than what the generator prints.
	if !bytes.Equal(onDisk, buf.Bytes()) {
		t.Error("file output and writer output must be byte-identical")
	}
}

func TestRunReturnsErrorForUnwritablePath(t *testing.T) {
	// A path whose parent does not exist — the generator must report it
	// rather than panic, which is what it did before run was split out.
	path := filepath.Join(t.TempDir(), "no-such-dir", "tools.json")
	err := run(path, nil)
	if err == nil {
		t.Fatal("want error writing to an unwritable path")
	}
	// Naming the path is the assertion that bites: `if got := err.Error();
	// got == ""` would be unfalsifiable for a fmt.Errorf-produced error, so
	// it could not catch someone replacing the wrap with a bare `return err`.
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error must name the path it failed to write, got: %v", err)
	}
}

// TestRunOutputMatchesCommittedToolsJSON is a second, independent line of
// defense beside task check:drift: it proves the generator in THIS tree
// reproduces the committed artifact, without needing buf or the full
// generate:contract pipeline to run first.
func TestRunOutputMatchesCommittedToolsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := run("", &buf); err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(filepath.Join("..", "..", "cmd", "vtt", "tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), committed) {
		t.Error("generator output does not match committed cmd/vtt/tools.json")
	}
}
