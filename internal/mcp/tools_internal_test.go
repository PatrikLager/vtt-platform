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

// theCommandTools is the tools.json ↔ oneof agreement, spelled out.
//
// Deliberately NOT named for its length. It was theThirteenCommandTools until
// 2026-08-06, and a count in an identifier has to be renamed by every change
// that adds a command — P12 shipped the same stale number three separate
// times on one rename. The list is the assertion; its size is not.
// Adding a ClientCommand variant means adding it here too — that is the
// point.
var theCommandTools = []string{
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
	"grant_actor_control",
	"revoke_actor_control",
	"promote_participant",
	"set_join_door",
	"rotate_join_link",
	"open_door",
	"close_door",
	"load_map",
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
	dispatch, err := buildDispatch(theCommandTools)
	if err != nil {
		t.Fatalf("the committed tool names must agree with the oneof: %v", err)
	}
	if len(dispatch) != len(theCommandTools) {
		t.Fatalf("dispatch has %d entries, want %d", len(dispatch), len(theCommandTools))
	}
	for _, name := range theCommandTools {
		if _, ok := dispatch[name]; !ok {
			t.Errorf("dispatch is missing %q", name)
		}
	}
}

// TestBuildDispatchRejectsToolNameWithNoOneofField is the stale-tools.json
// direction: the embedded copy names a command this vttv1 build does not
// have.
func TestBuildDispatchRejectsToolNameWithNoOneofField(t *testing.T) {
	names := append(append([]string{}, theCommandTools...), "summon_kraken")
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
	// The dropped name is DERIVED, not spelled out. Naming it meant this test
	// had to be edited by every change that appends a command — the same
	// coupling that put a count in theCommandTools' old identifier.
	dropped := theCommandTools[len(theCommandTools)-1]
	names := theCommandTools[:len(theCommandTools)-1]
	_, err := buildDispatch(names)
	if err == nil {
		t.Fatal("want error when the oneof has a field tools.json never names")
	}
	if !strings.Contains(err.Error(), dropped) {
		t.Errorf("error must name the unreachable command %q, got: %v", dropped, err)
	}
}

// TestBuildDispatchReportsBothDirectionsAtOnce pins that a manifest wrong in
// both directions surfaces both lists, rather than stopping at the first.
func TestBuildDispatchReportsBothDirectionsAtOnce(t *testing.T) {
	dropped := theCommandTools[len(theCommandTools)-1]
	names := append(append([]string{}, theCommandTools[:len(theCommandTools)-1]...), "summon_kraken")
	_, err := buildDispatch(names)
	if err == nil {
		t.Fatal("want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "summon_kraken") || !strings.Contains(msg, dropped) {
		t.Errorf("error must report both directions (%q and summon_kraken), got: %v", dropped, msg)
	}
}
