package main

// mcp_ruleset_e2e_test.go covers the MCP-layer piece of ruleset-interpreter
// Task 6's exit criteria (task brief, verbatim): "use_ability through MCP
// against a toy-ruleset server (assert isError=false, result carries first
// seq, follow-up get_events_since shows the batch); no-ruleset server +
// use_ability -> isError=true with 'no ruleset loaded'; get_ruleset_guide
// with and without --ruleset." The tool-count-is-12 assertion itself lives
// in mcp_e2e_test.go's TestMCPCommandServesRealStdioTransport (unchanged
// file, just its literal bumped 11->12).

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// startMCPFixtureWithRuleset is startMCPFixture's (mcp_e2e_test.go) sibling,
// composing the server WITH rulesetDir loaded (empty rulesetDir behaves
// identically to startMCPFixture itself).
func startMCPFixtureWithRuleset(t *testing.T, rulesetDir string) mcpFixture {
	t.Helper()
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", rulesetDir, "")
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

// mustCallTool calls name/args on cs and fails the test on a PROTOCOL error
// (never on a tool-level isError, which callers assert on explicitly).
func mustCallTool(t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

func toolResultText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("want text content, got %T", res.Content[0])
	}
	return text.Text
}

// toolCommandResult decodes a command tool's result content (protojson
// CommandResult) into a generic map — good enough to read "ok"/"sequence"
// without importing protojson's typed unmarshal here (this file already
// has enough imports; a generic map keeps the assertions simple).
func toolCommandResult(t *testing.T, res *mcpsdk.CallToolResult) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(toolResultText(t, res)), &m); err != nil {
		t.Fatalf("decode CommandResult JSON: %v (body: %s)", err, toolResultText(t, res))
	}
	return m
}

// setUpTavernBrawlActorsViaMCP drives the same setup a toy-brawl scenario
// would (session/scene/two actors/two adjacent tokens) through MCP command
// tools — every call MUST succeed (isError=false, ok=true), asserted
// inline so a setup failure names exactly which step broke.
func setUpTavernBrawlActorsViaMCP(t *testing.T, cs *mcpsdk.ClientSession) {
	t.Helper()
	steps := []struct {
		tool string
		args map[string]any
	}{
		{"start_session", map[string]any{"name": "mcp ruleset e2e"}},
		{"create_scene", map[string]any{"sceneId": "tavern", "name": "Tavern", "gridWidth": 5, "gridHeight": 5}},
		{"add_actor", map[string]any{"actor": map[string]any{
			"actorId": "brawler", "name": "Brawler",
			"attributes": map[string]any{"brawn": 3, "grit": 1},
		}}},
		{"add_actor", map[string]any{"actor": map[string]any{
			"actorId": "patron", "name": "Patron",
			"attributes": map[string]any{"footing": 0},
			"resources":  map[string]any{"drink": map[string]any{"current": 0, "max": 5}},
		}}},
		{"place_token", map[string]any{"tokenId": "tok-brawler", "sceneId": "tavern", "actorId": "brawler", "position": map[string]any{"x": 0, "y": 0}}},
		{"place_token", map[string]any{"tokenId": "tok-patron", "sceneId": "tavern", "actorId": "patron", "position": map[string]any{"x": 1, "y": 0}}},
	}
	for _, st := range steps {
		res := mustCallTool(t, cs, st.tool, st.args)
		result := toolCommandResult(t, res)
		if res.IsError || result["ok"] != true {
			t.Fatalf("setup step %s: want ok, got isError=%v result=%v", st.tool, res.IsError, result)
		}
	}
}

// TestMCPUseAbilityAgainstToyRulesetServer is the exit criterion's payoff
// case: a real composeServer with tavern-brawl loaded, use_ability called
// through MCP (footing=0 on the target guarantees a hit regardless of the
// live crypto Roller's draw — same trick as internal/gateway/
// ruleset_test.go), and a follow-up get_events_since proving the WHOLE
// batch (not just the leading AbilityUsed) reached this MCP server's own
// accumulated history.
func TestMCPUseAbilityAgainstToyRulesetServer(t *testing.T) {
	rulesetDir, err := resolveRulesetDir("tavern-brawl")
	if err != nil {
		t.Fatalf("resolveRulesetDir(tavern-brawl): %v", err)
	}
	fx := startMCPFixtureWithRuleset(t, rulesetDir)
	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	defer cleanup()

	setUpTavernBrawlActorsViaMCP(t, cs)

	res := mustCallTool(t, cs, "use_ability", map[string]any{
		"actorId": "brawler", "abilityId": "fists", "targetIds": []any{"patron"},
	})
	if res.IsError {
		t.Fatalf("use_ability: want isError=false (footing=0 guarantees a hit), got %+v", res)
	}
	result := toolCommandResult(t, res)
	if result["ok"] != true {
		t.Fatalf("use_ability CommandResult: want ok=true, got %v", result)
	}
	firstSeqStr, _ := result["sequence"].(string) // protojson int64 -> JSON string
	if firstSeqStr == "" || firstSeqStr == "0" {
		t.Fatalf("use_ability CommandResult: want a non-zero first sequence, got %v", result["sequence"])
	}

	// Follow-up get_events_since must show the batch: at least the leading
	// AbilityUsed plus its ResourceChanged/ConditionApplied outcome/
	// threshold events (tavern-brawl's fists.json hit list + drink
	// threshold, read verbatim in rulesets/tavern-brawl/). Polled, not a
	// single call: the CommandResult's own ack (asserted above) is a
	// read-your-writes guarantee on the RESULT only — the SAME event
	// reaching this MCP server's own accumulated history (what
	// get_events_since reads) is a separate, asynchronous drain of the
	// wire's broadcast stream (server.go's pump) that can lag briefly
	// behind, exactly the race mcp_e2e_test.go's own
	// waitForMCPHeadSequence exists to absorb for get_state.
	deadline := time.Now().Add(5 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		sinceRes := mustCallTool(t, cs, "get_events_since", map[string]any{"afterSequence": 0, "limit": 200})
		if sinceRes.IsError {
			t.Fatalf("get_events_since: want isError=false, got %+v", sinceRes)
		}
		body = toolResultText(t, sinceRes)
		if strings.Contains(body, "abilityUsed") && strings.Contains(body, "resourceChanged") {
			return // batch fully observed — proof complete.
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("get_events_since never showed the full batch (abilityUsed + resourceChanged) within 5s: %s", body)
}

// TestMCPUseAbilityNoRulesetServerCleanError covers the OTHER exit-
// criterion half: a server composed WITHOUT --ruleset (startMCPFixture,
// unchanged from before this task) rejects use_ability with a clean
// tool-level isError naming "no ruleset loaded" — never a protocol error,
// crash, or hang.
func TestMCPUseAbilityNoRulesetServerCleanError(t *testing.T) {
	fx := startMCPFixture(t) // no ruleset
	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	defer cleanup()

	res := mustCallTool(t, cs, "use_ability", map[string]any{
		"actorId": "brawler", "abilityId": "fists", "targetIds": []any{"patron"},
	})
	if !res.IsError {
		t.Fatalf("use_ability: want isError=true with no ruleset loaded, got %+v", res)
	}
	body := toolResultText(t, res)
	if !strings.Contains(body, "no ruleset loaded") {
		t.Fatalf("use_ability error body = %q, want it to contain %q", body, "no ruleset loaded")
	}
}

// TestMCPRulesetGuideWithAndWithoutRulesetFlag covers `vtt mcp --ruleset`
// end to end as the REAL, separately-built subprocess binary (mirroring
// TestMCPCommandServesRealStdioTransport's own pattern, mcp_e2e_test.go) —
// deliberately NOT startMCPFixtureWithRuleset/startMCPSession: those
// configure the GATEWAY's ruleset (`vtt serve --ruleset`, used for
// use_ability resolution — TestMCPUseAbilityAgainstToyRulesetServer above),
// a completely independent flag from `vtt mcp`'s OWN --ruleset (used only
// for get_ruleset_guide's content, cmd/vtt/mcp.go). The gateway here is
// deliberately rulesetless: proves the two flags truly are orthogonal —
// use_ability still needs the SERVER's --ruleset regardless of what `vtt
// mcp --ruleset` is pointed at.
//
// WITH --ruleset, get_ruleset_guide returns the committed tavern-brawl
// guide.md's own content; WITHOUT it (the flag omitted entirely), the same
// tool call comes back isError=true naming "no ruleset loaded".
func TestMCPRulesetGuideWithAndWithoutRulesetFlag(t *testing.T) {
	rulesetDir, err := resolveRulesetDir("tavern-brawl")
	if err != nil {
		t.Fatalf("resolveRulesetDir(tavern-brawl): %v", err)
	}
	guideBytes, err := os.ReadFile(filepath.Join(rulesetDir, "guide.md"))
	if err != nil {
		t.Fatalf("read committed guide.md: %v", err)
	}

	binPath := buildVTTBinary(t)
	fx := startMCPFixture(t) // gateway has NO ruleset — see doc comment above.

	t.Run("with --ruleset", func(t *testing.T) {
		cs, cleanup := dialMCPSubprocess(t, binPath, fx.wsURL, fx.agentToken, "--ruleset", rulesetDir)
		defer cleanup()

		res := mustCallTool(t, cs, "get_ruleset_guide", map[string]any{})
		if res.IsError {
			t.Fatalf("get_ruleset_guide: want isError=false, got %+v", res)
		}
		if got := toolResultText(t, res); got != string(guideBytes) {
			t.Fatalf("get_ruleset_guide text != committed guide.md verbatim (got %d bytes, want %d)", len(got), len(guideBytes))
		}
	})

	t.Run("without --ruleset", func(t *testing.T) {
		cs, cleanup := dialMCPSubprocess(t, binPath, fx.wsURL, fx.agentToken)
		defer cleanup()

		res := mustCallTool(t, cs, "get_ruleset_guide", map[string]any{})
		if !res.IsError {
			t.Fatalf("get_ruleset_guide: want isError=true with no ruleset loaded, got %+v", res)
		}
		if body := toolResultText(t, res); !strings.Contains(body, "no ruleset loaded") {
			t.Fatalf("get_ruleset_guide error body = %q, want it to contain %q", body, "no ruleset loaded")
		}
	})
}

// dialMCPSubprocess starts `vtt mcp --server wsURL --token token <extraArgs...>`
// as a real OS subprocess and connects an SDK client over its stdio pipes
// (the exact pattern TestMCPCommandServesRealStdioTransport establishes,
// mcp_e2e_test.go) — the cleanup func closes the session (closing the
// subprocess's stdin, letting it exit on its own) and force-kills as a
// safety net.
func dialMCPSubprocess(t *testing.T, binPath, wsURL, token string, extraArgs ...string) (*mcpsdk.ClientSession, func()) {
	t.Helper()
	args := append([]string{"mcp", "--server", wsURL, "--token", token}, extraArgs...)
	cmd := exec.Command(binPath, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start vtt mcp subprocess: %v", err)
	}

	clientTransport := &mcpsdk.IOTransport{Reader: stdout, Writer: stdin}
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connectCancel()
	cl := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mcp-ruleset-e2e-client", Version: "0.0.1"}, nil)
	cs, err := cl.Connect(connectCtx, clientTransport, nil)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("stdio client Connect: %v (stderr: %s)", err, stderr.String())
	}

	cleanup := func() {
		cs.Close()
		if err := waitWithTimeout(cmd, 5*time.Second); err != nil {
			t.Errorf("subprocess did not exit cleanly after stdin EOF: %v (stderr: %s)", err, stderr.String())
		}
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return cs, cleanup
}
