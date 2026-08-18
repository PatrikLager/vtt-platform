package main

// mcp_adventure_e2e_test.go covers adventure-format Task 4's MCP exit
// criterion (task-12-4-brief.md, verbatim): "load_adventure round-trip
// against a live server (batch observed via get_events_since, note in
// get_state), guide served, no-dir clean error." Built the same way
// mcp_ruleset_e2e_test.go covers ruleset-interpreter Task 6's own MCP exit
// criterion — real composeServer gateway + the REAL `vtt mcp` subprocess
// binary (dialMCPSubprocess, mcp_ruleset_e2e_test.go's own established
// pattern) for the flag-wiring proofs, since --adventures-dir is a cmd/vtt
// flag, not something internal/mcp's own test suite can exercise on its
// own (that package's adventure_guide_tool_test.go already covers the tool
// handler in isolation against a fake wire).

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// adventuresFixtureDir resolves the repo's real adventures/ library. It is
// deliberately MIXED — cellar-rats declares tavern-brawl, goblin-ambush
// declares dnd45e-minimal — so pointing the MCP fixture at it also exercises
// loadAdventuresDir's ruleset selection on the way in. It used to resolve a
// symlinked single-ruleset fixture, which existed only because the loader
// refused any mismatch (see adventures.go's loadAdventuresDir doc comment).
func adventuresFixtureDir(t *testing.T) string {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}
	return filepath.Join(root, "adventures")
}

// startMCPFixtureWithRulesetAndAdventures is startMCPFixtureWithRuleset's
// (mcp_ruleset_e2e_test.go) sibling: composes the GATEWAY with BOTH a
// ruleset and adventures loaded, so load_adventure actually has something
// to compile against.
func startMCPFixtureWithRulesetAndAdventures(t *testing.T, rulesetDir, adventuresDir string) mcpFixture {
	t.Helper()
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", rulesetDir, adventuresDir, "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		srv.Close()
		if err := closeFn(); err != nil {
			t.Errorf("closeFn: %v", err)
		}
	})
	go func() { _ = srv.Serve(ln) }()

	if err := waitForHealthz("http://"+ln.Addr().String(), 3*time.Second); err != nil {
		t.Fatalf("healthz never became ready: %v", err)
	}

	agentToken := mintInviteToken(t, campaignPath, identity.RoleAgent, "agent")
	return mcpFixture{
		wsURL:        "ws://" + ln.Addr().String() + "/ws",
		agentToken:   agentToken,
		campaignPath: campaignPath,
	}
}

// TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved is the exit
// criterion's payoff case: `vtt mcp --ruleset --adventures-dir` against a
// gateway ALSO configured with both — load_adventure through the tool
// round-trips to ok=true with a non-zero first sequence, the whole compiled
// batch (adventureLoaded + sceneCreated + actorAdded x3 + tokenPlaced x3 +
// noteUpserted + narrationAdded, internal/adventure/conformance's own
// hand-derived goblin-ambush golden) shows up in get_events_since, the
// loaded note shows up in get_state's Notes map, and get_adventure_guide
// returns the committed guide.md verbatim.
func TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved(t *testing.T) {
	rulesetDir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir(dnd45e-minimal): %v", err)
	}
	adventuresDir := adventuresFixtureDir(t)
	guideBytes, err := os.ReadFile(filepath.Join(adventuresDir, "goblin-ambush", "guide.md"))
	if err != nil {
		t.Fatalf("read committed guide.md: %v", err)
	}

	fx := startMCPFixtureWithRulesetAndAdventures(t, rulesetDir, adventuresDir)

	binPath := buildVTTBinary(t)
	cs, cleanup := dialMCPSubprocess(t, binPath, fx.wsURL, fx.agentToken, "--ruleset", rulesetDir, "--adventures-dir", adventuresDir)
	defer cleanup()

	res := mustCallTool(t, cs, "load_adventure", map[string]any{"adventureId": "goblin-ambush"})
	if res.IsError {
		t.Fatalf("load_adventure: want isError=false, got %+v", res)
	}
	result := toolCommandResult(t, res)
	if result["ok"] != true {
		t.Fatalf("load_adventure CommandResult: want ok=true, got %v", result)
	}
	firstSeqStr, _ := result["sequence"].(string) // protojson int64 -> JSON string
	if firstSeqStr == "" || firstSeqStr == "0" {
		t.Fatalf("load_adventure CommandResult: want a non-zero first sequence, got %v", result["sequence"])
	}

	// --- get_adventure_guide serves the committed guide.md verbatim ---
	guideRes := mustCallTool(t, cs, "get_adventure_guide", map[string]any{"adventureId": "goblin-ambush"})
	if guideRes.IsError {
		t.Fatalf("get_adventure_guide: want isError=false, got %+v", guideRes)
	}
	if got := toolResultText(t, guideRes); got != string(guideBytes) {
		t.Fatalf("get_adventure_guide text != committed guide.md verbatim (got %d bytes, want %d)", len(got), len(guideBytes))
	}

	// --- the whole compiled batch shows up in get_events_since ---
	deadline := time.Now().Add(5 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		sinceRes := mustCallTool(t, cs, "get_events_since", map[string]any{"afterSequence": 0, "limit": 200})
		if sinceRes.IsError {
			t.Fatalf("get_events_since: want isError=false, got %+v", sinceRes)
		}
		body = toolResultText(t, sinceRes)
		if strings.Contains(body, "adventureLoaded") && strings.Contains(body, "narrationAdded") &&
			strings.Contains(body, "tok-fighter") && strings.Contains(body, "ravine-trail-warning") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(body, "adventureLoaded") {
		t.Fatalf("get_events_since never showed the full load_adventure batch within 5s: %s", body)
	}

	// --- the loaded note shows up in get_state's Notes map ---
	stateDeadline := time.Now().Add(5 * time.Second)
	var stateBody string
	for time.Now().Before(stateDeadline) {
		stateRes := mustCallTool(t, cs, "get_state", map[string]any{})
		if stateRes.IsError {
			t.Fatalf("get_state: want isError=false, got %+v", stateRes)
		}
		stateBody = toolResultText(t, stateRes)
		if strings.Contains(stateBody, "ravine-trail-warning") {
			return // proof complete.
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("get_state never showed the loaded note within 5s: %s", stateBody)
}

// TestMCPAdventuresDirWithoutRulesetFlagIsAStartupError covers the MCP flag
// precedent (spec §7, task-12-4-brief.md binding): `vtt mcp --adventures-dir`
// WITHOUT --ruleset fails immediately as a startup flag error, before ever
// dialing the gateway — get_adventure_guide needs no ruleset itself, but
// LOADING the adventures directory does (every adventure declares the
// ruleset id it was written for), so the pairing is required.
func TestMCPAdventuresDirWithoutRulesetFlagIsAStartupError(t *testing.T) {
	adventuresDir := adventuresFixtureDir(t)
	_, err := runCLI(t, "mcp", "--server", "ws://127.0.0.1:1/ws", "--token", "irrelevant", "--adventures-dir", adventuresDir)
	if err == nil {
		t.Fatal("want an error starting `vtt mcp --adventures-dir` without --ruleset")
	}
	if !strings.Contains(err.Error(), "--adventures-dir") || !strings.Contains(err.Error(), "--ruleset") {
		t.Fatalf("error = %q, want it to name both --adventures-dir and --ruleset", err.Error())
	}
}

// TestMCPEmptyAdventuresDirIsAStartupError covers fix-wave F4's `vtt mcp`
// half: an EXISTING --adventures-dir with zero subdirectories fails
// immediately as a startup error, before ever dialing the gateway — the
// same fail-loud-at-boot posture TestMCPAdventuresDirWithoutRulesetFlag
// IsAStartupError already covers for the flag-pairing case, now for the
// zero-loaded case (loadAdventuresDir, adventures.go).
func TestMCPEmptyAdventuresDirIsAStartupError(t *testing.T) {
	rulesetDir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir(dnd45e-minimal): %v", err)
	}
	emptyAdventuresDir := t.TempDir() // exists, zero subdirectories

	_, err = runCLI(t, "mcp", "--server", "ws://127.0.0.1:1/ws", "--token", "irrelevant",
		"--ruleset", rulesetDir, "--adventures-dir", emptyAdventuresDir)
	if err == nil {
		t.Fatal("want an error starting `vtt mcp --adventures-dir` pointed at an existing-but-empty directory")
	}
	if !strings.Contains(err.Error(), "no adventures") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "no adventures")
	}
}

// TestMCPGetAdventureGuideNoDirCleanError covers the "no dir → clean error"
// half of the exit criterion: `vtt mcp` started with NEITHER
// --adventures-dir NOR --ruleset (startMCPFixture, unchanged from before
// this task) rejects get_adventure_guide with a clean tool-level isError
// naming "no adventures available" — never a protocol error, crash, or
// hang, and the connection stays usable for a follow-up call.
func TestMCPGetAdventureGuideNoDirCleanError(t *testing.T) {
	fx := startMCPFixture(t) // no --ruleset, no --adventures-dir
	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	defer cleanup()

	res := mustCallTool(t, cs, "get_adventure_guide", map[string]any{"adventureId": "goblin-ambush"})
	if !res.IsError {
		t.Fatalf("get_adventure_guide: want isError=true with no adventures configured, got %+v", res)
	}
	body := toolResultText(t, res)
	if !strings.Contains(body, "no adventures available") {
		t.Fatalf("get_adventure_guide error body = %q, want it to contain %q", body, "no adventures available")
	}

	// Connection intact: a follow-up ordinary read tool call still succeeds.
	_ = callGetStateGeneric(t, cs)
}
