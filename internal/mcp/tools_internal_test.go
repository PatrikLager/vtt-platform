package mcp

// tools_internal_test.go covers the startup-time agreement check between the
// committed tools.json this package was configured with and the
// vttv1.ClientCommand oneof it is running against.
//
// buildDispatch is the only place a tool name resolves to a command shape.
// If it accepted a disagreement, a stale embedded tools.json would surface
// tools that dispatch nowhere, or silently drop commands the contract has
// gained — and the failure would appear at tool-call time, to the seated
// agent, rather than at boot.
//
// The expected tool names below are written out LITERALLY, never derived
// from the oneof descriptor, following the binding note in
// internal/gateway/authz_test.go:123-125: a table derived from the thing
// under test proves nothing about its contents.

import (
	"strings"
	"testing"
)

// theThirteenCommandTools is the tools.json ↔ oneof agreement, spelled out.
// Adding a ClientCommand variant means adding it here too — that is the
// point.
var theThirteenCommandTools = []string{
	"move_token",
	"create_scene",
	"add_actor",
	"place_token",
	"start_session",
	"end_session",
	"retract_events",
	"use_ability",
	"remove_condition",
	"add_narration",
	"upsert_note",
	"delete_note",
	"load_adventure",
}

func TestParseToolsJSONRejectsMalformedInput(t *testing.T) {
	if _, err := parseToolsJSON([]byte("{not json")); err == nil {
		t.Fatal("want error for malformed tools.json")
	}
}

func TestParseToolsJSONRejectsEmptyManifest(t *testing.T) {
	_, err := parseToolsJSON([]byte("[]"))
	if err == nil {
		t.Fatal("want error for a tools.json with no entries")
	}
	if !strings.Contains(err.Error(), "no tool entries") {
		t.Errorf("error should say the manifest is empty, got: %v", err)
	}
}

func TestParseToolsJSONAcceptsAWellFormedEntry(t *testing.T) {
	entries, err := parseToolsJSON([]byte(`[{"name":"move_token","description":"d","inputSchema":{"type":"object"}}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Name != "move_token" {
		t.Errorf("name: got %q, want %q", entries[0].Name, "move_token")
	}
	if entries[0].InputSchema == nil {
		t.Error("inputSchema must survive decoding — it is handed straight to the SDK")
	}
}

func TestBuildDispatchAcceptsExactAgreement(t *testing.T) {
	dispatch, err := buildDispatch(theThirteenCommandTools)
	if err != nil {
		t.Fatalf("the committed tool names must agree with the oneof: %v", err)
	}
	if len(dispatch) != len(theThirteenCommandTools) {
		t.Fatalf("dispatch has %d entries, want %d", len(dispatch), len(theThirteenCommandTools))
	}
	for _, name := range theThirteenCommandTools {
		if _, ok := dispatch[name]; !ok {
			t.Errorf("dispatch is missing %q", name)
		}
	}
}

// TestBuildDispatchRejectsToolNameWithNoOneofField is the stale-tools.json
// direction: the embedded copy names a command this vttv1 build does not
// have.
func TestBuildDispatchRejectsToolNameWithNoOneofField(t *testing.T) {
	names := append(append([]string{}, theThirteenCommandTools...), "summon_kraken")
	_, err := buildDispatch(names)
	if err == nil {
		t.Fatal("want error when tools.json names a command the oneof lacks")
	}
	if !strings.Contains(err.Error(), "summon_kraken") {
		t.Errorf("error must name the offending tool, got: %v", err)
	}
}

// TestBuildDispatchRejectsOneofFieldWithNoToolEntry is the other direction:
// the contract gained a command and the embedded tools.json never learned
// about it. Without this check the command would be silently unreachable.
func TestBuildDispatchRejectsOneofFieldWithNoToolEntry(t *testing.T) {
	names := theThirteenCommandTools[:len(theThirteenCommandTools)-1] // drop load_adventure
	_, err := buildDispatch(names)
	if err == nil {
		t.Fatal("want error when the oneof has a field tools.json never names")
	}
	if !strings.Contains(err.Error(), "load_adventure") {
		t.Errorf("error must name the unreachable command, got: %v", err)
	}
}

// TestBuildDispatchReportsBothDirectionsAtOnce pins that a manifest wrong in
// both directions surfaces both lists, rather than stopping at the first.
func TestBuildDispatchReportsBothDirectionsAtOnce(t *testing.T) {
	names := append(append([]string{}, theThirteenCommandTools[:len(theThirteenCommandTools)-1]...), "summon_kraken")
	_, err := buildDispatch(names)
	if err == nil {
		t.Fatal("want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "summon_kraken") || !strings.Contains(msg, "load_adventure") {
		t.Errorf("error must report both directions, got: %v", msg)
	}
}
