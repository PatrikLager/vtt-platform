package main

// mcp_e2e_test.go covers plan Task 3 in full (task-3-brief.md):
//
//   - resolveMCPToken's flag/env precedence and the missing-both error —
//     genuinely new cmd-level logic, given true stub-first behavioral RED
//     (ADR-009 rule 1/2): the report's RED transcript captures these against
//     a temporarily-stubbed resolveMCPToken that always errors.
//   - TestMCPCommandServesRealStdioTransport — proves `vtt mcp`'s actual
//     RunE (StdioTransport, not an in-memory substitute) really works,
//     built as a real subprocess talking newline-delimited JSON over OS
//     pipes. Also genuine stub-first RED: against the stubbed
//     resolveMCPToken, RunE returns immediately and the subprocess never
//     completes an MCP initialize handshake.
//   - TestMCPSpecSevenExitCriteria — the spec §7 exit test: a REAL
//     composeServer gateway + a real agent invite, `vtt mcp`'s own
//     internal/mcp.Server served over the SDK's in-memory transport (the
//     brief's explicit "(or in-process transport)" allowance — deterministic
//     and fast, the same choice internal/mcp's own test suite makes) to an
//     SDK test client. This is a KEYSTONE/characterization test over already
//     -correct code (internal/mcp shipped fully tested in P7 Tasks 1-2;
//     composeServer/identity likewise) — per ADR-009 rule 3 it has no
//     natural red phase of its own; its fault-injection proof is this task's
//     Step 3 (see the report).
//
// Every test here lives in package main (cmd/vtt), same as
// client_e2e_test.go/serve_e2e_test.go, because it needs composeServer
// (unexported, cmd-only) — "cmd may compose" — and toolsJSON (the embedded
// var declared in mcp.go, same package).

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	mcppkg "github.com/PatrikLager/vtt-platform/internal/mcp"
)

// --- resolveMCPToken: flag/env precedence (stub-first behavioral RED) -----

func TestResolveMCPTokenFlagWinsOverEnv(t *testing.T) {
	t.Setenv("VTT_TOKEN", "env-token")
	got, err := resolveMCPToken("flag-token")
	if err != nil {
		t.Fatalf("resolveMCPToken: unexpected error: %v", err)
	}
	if got != "flag-token" {
		t.Fatalf("resolveMCPToken = %q, want %q (flag must win over env)", got, "flag-token")
	}
}

func TestResolveMCPTokenFallsBackToEnvWhenFlagEmpty(t *testing.T) {
	t.Setenv("VTT_TOKEN", "env-token")
	got, err := resolveMCPToken("")
	if err != nil {
		t.Fatalf("resolveMCPToken: unexpected error: %v", err)
	}
	if got != "env-token" {
		t.Fatalf("resolveMCPToken = %q, want %q (VTT_TOKEN env fallback)", got, "env-token")
	}
}

func TestResolveMCPTokenMissingBothReturnsClearError(t *testing.T) {
	t.Setenv("VTT_TOKEN", "")
	_, err := resolveMCPToken("")
	if err == nil {
		t.Fatal("resolveMCPToken: want an error when neither --token nor VTT_TOKEN is set")
	}
	if !strings.Contains(err.Error(), "--token") || !strings.Contains(err.Error(), "VTT_TOKEN") {
		t.Fatalf("resolveMCPToken error = %q, want it to name both --token and VTT_TOKEN", err.Error())
	}
}

// TestMCPCommandMissingServerFlagErrors covers --server's required-flag
// validation, the same structural shape cli_test.go's
// TestServeMissingCampaignErrors covers for serve — cobra's own machinery,
// not new logic, so this is a structural check (immediately satisfiable),
// not a RED-phase assertion.
func TestMCPCommandMissingServerFlagErrors(t *testing.T) {
	if _, err := runCLI(t, "mcp"); err == nil {
		t.Fatal("mcp: want error for missing --server flag")
	}
}

// TestMCPCommandMissingTokenErrorsViaRunCLI drives the real cobra command
// (runCLI, cli_test.go's established pattern) with --server set but no
// --token flag and VTT_TOKEN unset: RunE must fail with resolveMCPToken's
// own clear error BEFORE ever attempting to dial (deterministic, no network
// dependency — --server points at a reserved, never-listening port so a
// regression that skipped straight to dialing would hang or produce a
// completely different error, not this test's specific one).
func TestMCPCommandMissingTokenErrorsViaRunCLI(t *testing.T) {
	t.Setenv("VTT_TOKEN", "")
	_, err := runCLI(t, "mcp", "--server", "ws://127.0.0.1:1/ws")
	if err == nil {
		t.Fatal("mcp: want error when neither --token nor VTT_TOKEN is set")
	}
	if !strings.Contains(err.Error(), "--token") || !strings.Contains(err.Error(), "VTT_TOKEN") {
		t.Fatalf("mcp: error = %q, want it to name both --token and VTT_TOKEN", err.Error())
	}
}

// --- TestMCPCommandServesRealStdioTransport: the real RunE, real OS pipes -

// TestMCPCommandServesRealStdioTransport builds the actual vtt binary,
// starts `vtt mcp --server <fx> --token <agent>` as a real OS subprocess,
// and drives it with an SDK client wired to the subprocess's own
// Stdin/Stdout pipes (mcpsdk.IOTransport) — proving cmd/vtt/mcp.go's real
// RunE(ctx, &mcpsdk.StdioTransport{}) path actually works end to end, not
// just the in-memory-transport substitute TestMCPSpecSevenExitCriteria
// uses below. list_tools must report all 17 tools (adventure-format P12
// Task 1 contract addition — load_adventure — bumped this from 15 to 16;
// P12 Task 4 adds get_adventure_guide itself, bumping 16->17; both
// fix-forwards pre-authorized by their own task briefs); closing the client
// session then closes the subprocess's stdin, which is enough to let the
// process exit on its own (asserted via a bounded Wait, Kill as the
// fallback safety net matching this package's established subprocess-test
// shape — see client_e2e_test.go's TestEventsTailBinaryExitsCleanlyOnSIGINT).
func TestMCPCommandServesRealStdioTransport(t *testing.T) {
	binPath := buildVTTBinary(t)
	fx := startMCPFixture(t)

	cmd := exec.Command(binPath, "mcp", "--server", fx.wsURL, "--token", fx.agentToken)
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
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	clientTransport := &mcpsdk.IOTransport{Reader: stdout, Writer: stdin}
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connectCancel()
	cl := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mcp-stdio-smoke-client", Version: "0.0.1"}, nil)
	cs, err := cl.Connect(connectCtx, clientTransport, nil)
	if err != nil {
		t.Fatalf("stdio client Connect: %v (stderr: %s)", err, stderr.String())
	}

	listCtx, listCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer listCancel()
	res, err := cs.ListTools(listCtx, nil)
	if err != nil {
		t.Fatalf("ListTools over real stdio: %v (stderr: %s)", err, stderr.String())
	}
	if len(res.Tools) != 17 {
		t.Fatalf("ListTools over real stdio: got %d tools, want 17: %v", len(res.Tools), res.Tools)
	}

	cs.Close() // closes stdin -> subprocess sees EOF -> Run should return.
	if err := waitWithTimeout(cmd, 5*time.Second); err != nil {
		t.Fatalf("subprocess did not exit cleanly after stdin EOF: %v (stderr: %s)", err, stderr.String())
	}
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("subprocess exit code after stdin EOF = %d, want 0 (stderr: %s)", code, stderr.String())
	}
}

// --- TestMCPSubprocessExitsCleanlyOnSIGTERM: SIGTERM exit-code parity ------

// TestMCPSubprocessExitsCleanlyOnSIGTERM proves `vtt mcp` filters
// context.Canceled from its RunE into a clean nil return, exiting 0 on
// SIGTERM exactly like `vtt serve` does (serve_e2e_test.go's
// TestServeSubprocessExitsCleanlyOnSIGTERM is this test's direct template
// — the ledgered Minor from P7 Task 3's report) and `vtt events tail`
// (client_e2e_test.go's TestEventsTailBinaryExitsCleanlyOnSIGINT), rather
// than propagating the SDK's own ctx.Err() (context.Canceled) straight out
// to main.go's `os.Exit(1)` path. Stdin is a live pipe kept open for the
// whole test (never closed) so the ONLY way this subprocess can exit is
// the signal — proving SIGTERM handling specifically, independent of
// TestMCPCommandServesRealStdioTransport's stdin-EOF exit path. Readiness
// (past initial dial, blocked serving) is proven the same way that test
// proves it: a real MCP handshake + ListTools over the subprocess's own
// stdio pipes, not a fixed sleep.
func TestMCPSubprocessExitsCleanlyOnSIGTERM(t *testing.T) {
	binPath := buildVTTBinary(t)
	fx := startMCPFixture(t)

	cmd := exec.Command(binPath, "mcp", "--server", fx.wsURL, "--token", fx.agentToken)
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
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	clientTransport := &mcpsdk.IOTransport{Reader: stdout, Writer: stdin}
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	cl := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mcp-sigterm-client", Version: "0.0.1"}, nil)
	cs, err := cl.Connect(connectCtx, clientTransport, nil)
	connectCancel()
	if err != nil {
		t.Fatalf("stdio client Connect: %v (stderr: %s)", err, stderr.String())
	}
	listCtx, listCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if _, err := cs.ListTools(listCtx, nil); err != nil {
		listCancel()
		t.Fatalf("ListTools over real stdio: %v (stderr: %s)", err, stderr.String())
	}
	listCancel()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	// Bounded well past anything this subprocess needs to unwind (no live
	// Shutdown-style drain here, just ctx cancellation propagating through
	// internal/mcp.Server.Run) — a correct implementation exits close to
	// immediately, so this margin is purely to distinguish "slow but
	// working" from "swallowed entirely" without flaking on the former
	// (same reasoning as TestServeSubprocessExitsCleanlyOnSIGTERM's own
	// margin).
	if err := waitWithTimeout(cmd, 7*time.Second); err != nil {
		t.Fatalf("subprocess did not exit cleanly after SIGTERM: %v (stderr: %s)", err, stderr.String())
	}
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("subprocess exit code after SIGTERM = %d, want 0 (stderr: %s)", code, stderr.String())
	}
}

// --- TestMCPSpecSevenExitCriteria: the spec §7 exit test -------------------

// mcpFixture is a live composeServer instance this test starts and tears
// down itself (client_e2e_test.go's liveFixture, but minting an AGENT
// invite — spec §6's "agent-invite-only" security posture — rather than a
// dm one, and keeping campaignPath around so this file can mint additional
// roles, e.g. the spectator token TestMCPSpecSevenExitCriteria needs).
type mcpFixture struct {
	wsURL        string
	agentToken   string
	campaignPath string
}

func startMCPFixture(t *testing.T) mcpFixture {
	t.Helper()
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
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

// mintInviteToken opens its own short-lived identity.DB handle on
// campaignPath (the same pattern harness_boot.go's mintInvites and
// serve_e2e_test.go's inline CreateInvite call both use against a server
// they didn't mint through) and mints one invite, closing the handle before
// returning.
func mintInviteToken(t *testing.T, campaignPath string, role identity.Role, name string) string {
	t.Helper()
	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	defer ids.Close()
	token, _, err := ids.CreateInvite(name, role, nil)
	if err != nil {
		t.Fatalf("CreateInvite(%s): %v", role, err)
	}
	return token
}

// startMCPSession builds a real internal/mcp.Server against wsURL/token
// (the SAME toolsJSON mcp.go itself go:embeds — this exercises the
// committed production artifact, not a hand-loaded fixture) and connects an
// SDK client to it over an in-memory transport pair — the brief's explicit
// "(or in-process transport)" allowance, and the same shape internal/mcp's
// own server_test.go uses (there, against a fake wire; here, against a
// REAL composeServer gateway).
func startMCPSession(t *testing.T, wsURL, token string) (*mcpsdk.ClientSession, func()) {
	t.Helper()
	srv, err := mcppkg.New(mcppkg.Config{WSURL: wsURL, Token: token, ToolsJSON: toolsJSON})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx, serverTransport) }()

	connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
	defer connectCancel()
	cl := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mcp-e2e-client", Version: "0.0.1"}, nil)
	cs, err := cl.Connect(connectCtx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatalf("client Connect: %v", err)
	}

	cleanup := func() {
		cs.Close()
		cancel()
		select {
		case <-runErr:
		case <-time.After(5 * time.Second):
			t.Error("startMCPSession: Server.Run did not return after ctx cancel")
		}
	}
	return cs, cleanup
}

// camelToSnake converts a protojson-oneof-style camelCase key ("moveToken")
// to the tools.json/oneof-field snake_case tool name ("move_token") —
// exactly the naming correspondence tools/toolgen's manifest establishes
// (see internal/mcp/tools.go's buildDispatch doc comment).
func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stepToolCall decodes one harness.Step's Command (protojson-shaped
// {"<oneofKey>": {...}}) into the MCP tool name/arguments pair that same
// command maps to — arguments are the oneof value verbatim, exactly the
// shape each tool's inputSchema in tools.json documents.
func stepToolCall(t *testing.T, step harness.Step) (name string, args map[string]any) {
	t.Helper()
	var oneof map[string]json.RawMessage
	if err := json.Unmarshal(step.Command, &oneof); err != nil {
		t.Fatalf("decode step command: %v", err)
	}
	if len(oneof) != 1 {
		t.Fatalf("step command has %d keys, want exactly 1: %s", len(oneof), step.Command)
	}
	for key, raw := range oneof {
		name = camelToSnake(key)
		if err := json.Unmarshal(raw, &args); err != nil {
			t.Fatalf("decode step %q args: %v", key, err)
		}
	}
	return name, args
}

// playScenarioThroughTools is the brief's "translate its steps to tool
// calls in the test — the scenario file is the source of the sequence":
// every step in sc.Steps is played as one MCP tool call against cs, and its
// result checked against step.Expect (the same two shapes
// harness.Scenario's own Expect supports). Returns the last accepted
// command's Sequence — smoke.json's final step (endSession) is the
// campaign's head at that point.
func playScenarioThroughTools(t *testing.T, cs *mcpsdk.ClientSession, sc *harness.Scenario) int64 {
	t.Helper()
	var lastSeq int64
	for i, step := range sc.Steps {
		if step.Reconnect != nil {
			t.Fatalf("step %d: reconnect steps are not supported by this tool-based player", i)
		}
		name, args := stepToolCall(t, step)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		cancel()
		if err != nil {
			t.Fatalf("step %d (%s): CallTool: %v", i, name, err)
		}
		text, ok := res.Content[0].(*mcpsdk.TextContent)
		if !ok {
			t.Fatalf("step %d (%s): want text content, got %T", i, name, res.Content[0])
		}
		var result vttv1.CommandResult
		if err := protojson.Unmarshal([]byte(text.Text), &result); err != nil {
			t.Fatalf("step %d (%s): result not valid CommandResult protojson: %v", i, name, err)
		}

		switch {
		case step.Expect != nil && step.Expect.OK:
			if res.IsError || !result.GetOk() {
				t.Fatalf("step %d (%s): want ok, got IsError=%v result=%+v", i, name, res.IsError, &result)
			}
			lastSeq = result.GetSequence()
		case step.Expect != nil && step.Expect.DeniedContaining != "":
			if !res.IsError || !strings.Contains(result.GetError(), step.Expect.DeniedContaining) {
				t.Fatalf("step %d (%s): want denial containing %q, got IsError=%v error=%q",
					i, name, step.Expect.DeniedContaining, res.IsError, result.GetError())
			}
		default:
			t.Fatalf("step %d (%s): step.Expect is neither ok nor a denial: %+v", i, name, step.Expect)
		}
	}
	return lastSeq
}

// callGetStateGeneric calls get_state and decodes its response into a
// generic map — deliberately NOT a named struct, so the equality assertion
// in TestMCPSpecSevenExitCriteria compares the WHOLE folded state (every
// field the dump contract emits), not just a hand-picked sub-shape.
func callGetStateGeneric(t *testing.T, cs *mcpsdk.ClientSession) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_state", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("get_state: CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_state: want IsError=false, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_state: want text content, got %T", res.Content[0])
	}
	return decodeGenericJSON(t, text.Text)
}

// waitForMCPHeadSequence polls get_state until headSequence reaches
// atLeast — the tool-call commands above are read-your-writes on the
// CommandResult itself, but the SAME event reaching Server.history (what
// get_state actually folds) is a separate, asynchronous drain of the
// wire's broadcast stream (internal/mcp/server.go's pump) that can lag
// briefly behind the command's own ack.
func waitForMCPHeadSequence(t *testing.T, cs *mcpsdk.ClientSession, atLeast int64) map[string]any {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		last = callGetStateGeneric(t, cs)
		if hs, ok := last["headSequence"].(float64); ok && int64(hs) >= atLeast {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("get_state: headSequence never reached %d (last=%v)", atLeast, last)
	return last
}

func decodeGenericJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("decode JSON: %v (body: %s)", err, raw)
	}
	return m
}

// observeStateIndependently is the "separate wire observation" the brief
// requires get_state be checked against: a FRESH harness.Client dial
// (after=0, nothing shared with the mcp.Server under test), drained to
// read to its announced catch-up head (state_dump.go's drainToHead, reused
// as-is), folded (harness.Fold), and shaped exactly like `vtt state dump`
// (state_dump.go's writeDump, also reused as-is) — the identical algorithm
// internal/mcp/read_tools.go's marshalStateWithHead independently
// duplicates for get_state (see that file's doc comment on why it's a
// deliberate duplication, not shared code).
func observeStateIndependently(t *testing.T, wsURL, token string) map[string]any {
	t.Helper()
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dialCancel()
	c, err := harness.Dial(dialCtx, wsURL, token, 0)
	if err != nil {
		t.Fatalf("independent observation: dial: %v", err)
	}
	defer c.Close()

	events := drainToCatchUpHead(t, c)
	st, err := harness.Fold(events)
	if err != nil {
		t.Fatalf("independent observation: Fold: %v", err)
	}
	var buf bytes.Buffer
	if err := writeDump(&buf, st, headSequence(events)); err != nil {
		t.Fatalf("independent observation: writeDump: %v", err)
	}
	return decodeGenericJSON(t, buf.String())
}

// TestMCPSpecSevenExitCriteria is docs/superpowers/specs/2026-07-24-mcp-
// gateway-design.md §7's exit test, verbatim: play scenarios/smoke.json's
// command list through the tools against a real composeServer gateway with
// a real agent invite; get_state must equal harness.Fold of a separate wire
// observation, headSequence included; get_events_since pagination must walk
// the full log; a spectator-token connection's command tool call must come
// back isError=true with the authz message while the connection itself
// stays intact (a follow-up get_state still works).
func TestMCPSpecSevenExitCriteria(t *testing.T) {
	fx := startMCPFixture(t)
	sc, err := harness.LoadScenario(filepath.Join("..", "..", "scenarios", "smoke.json"))
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}

	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	defer cleanup()

	// --- play smoke.json through the tools ---

	lastSeq := playScenarioThroughTools(t, cs, sc)
	if lastSeq != 6 {
		t.Fatalf("playScenarioThroughTools: last accepted sequence = %d, want 6 (smoke.json's 6 steps, one event each)", lastSeq)
	}

	// --- get_state == harness.Fold of a separate wire observation ---

	gotState := waitForMCPHeadSequence(t, cs, lastSeq)

	// wireConnected (final review Fix 6a) is get_state's OWN addition on
	// top of the shared dump-contract shape — `vtt state dump`'s
	// writeDump (what observeStateIndependently below reuses) has no
	// such key, since a one-shot dump process has no persistent
	// connection to report on (see read_tools.go's marshalStateWithHead
	// doc comment). Assert it here on its own terms — true, since this
	// scenario's wire never drops — then exclude it before the DeepEqual
	// against that independent observation.
	wireConnected, ok := gotState["wireConnected"].(bool)
	if !ok || !wireConnected {
		t.Fatalf(`get_state: "wireConnected" = %#v, want true (the wire never drops in this scenario)`, gotState["wireConnected"])
	}
	delete(gotState, "wireConnected")

	wantState := observeStateIndependently(t, fx.wsURL, fx.agentToken)
	if !reflect.DeepEqual(gotState, wantState) {
		t.Fatalf("get_state != independent Fold observation:\n got:  %#v\n want: %#v", gotState, wantState)
	}
	if hs, ok := gotState["headSequence"].(float64); !ok || int64(hs) != lastSeq {
		t.Fatalf("get_state: headSequence = %v, want %d", gotState["headSequence"], lastSeq)
	}

	// --- get_events_since pagination walks the full log ---

	assertGetEventsSinceWalksFullLog(t, cs, lastSeq)

	// --- spectator-token variant: command denied, connection intact ---

	assertSpectatorCommandDeniedConnectionIntact(t, fx)
}

// assertGetEventsSinceWalksFullLog pages through the full history two at a
// time (smoke.json's 6 events -> 3 pages), asserting the walk's sequences
// are contiguous 1..head and `more` is true for every page except the last.
func assertGetEventsSinceWalksFullLog(t *testing.T, cs *mcpsdk.ClientSession, head int64) {
	t.Helper()
	const pageSize = 2

	var got []string
	after := int64(0)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "get_events_since",
			Arguments: map[string]any{"afterSequence": after, "limit": pageSize},
		})
		cancel()
		if err != nil {
			t.Fatalf("get_events_since: CallTool: %v", err)
		}
		if res.IsError {
			t.Fatalf("get_events_since: want IsError=false, got %+v", res)
		}
		text, ok := res.Content[0].(*mcpsdk.TextContent)
		if !ok {
			t.Fatalf("get_events_since: want text content, got %T", res.Content[0])
		}
		var page struct {
			Events       []json.RawMessage `json:"events"`
			HeadSequence int64             `json:"headSequence"`
			More         bool              `json:"more"`
		}
		if err := json.Unmarshal([]byte(text.Text), &page); err != nil {
			t.Fatalf("get_events_since: response did not decode: %v (body: %s)", err, text.Text)
		}
		if page.HeadSequence != head {
			t.Fatalf("get_events_since: headSequence = %d, want %d", page.HeadSequence, head)
		}
		for _, raw := range page.Events {
			var env struct {
				Sequence string `json:"sequence"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				t.Fatalf("get_events_since: envelope sequence not a JSON string: %v (body: %s)", err, raw)
			}
			got = append(got, env.Sequence)
			after++
		}
		if !page.More {
			break
		}
		if len(page.Events) == 0 {
			t.Fatal("get_events_since: more=true but page carried zero events — would loop forever")
		}
	}

	want := make([]string, head)
	for i := range want {
		want[i] = strconv.FormatInt(int64(i+1), 10)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("get_events_since: walked sequences = %v, want %v (contiguous 1..%d)", got, want, head)
	}
}

// assertSpectatorCommandDeniedConnectionIntact mints a spectator invite
// against fx's SAME campaign, opens its own MCP session, calls move_token
// (spectator is absent from internal/gateway/authz.go's commandRoles for
// move_token) and asserts a tool-level isError=true carrying the gateway's
// authz message — then, on the SAME session, calls get_state and asserts
// it still succeeds: the wire connection survives an authz denial, exactly
// spec §1's "adds zero privilege... every tool call becomes a ClientCommand
// judged by the gateway's authz table" promise.
func assertSpectatorCommandDeniedConnectionIntact(t *testing.T, fx mcpFixture) {
	t.Helper()
	specToken := mintInviteToken(t, fx.campaignPath, identity.RoleSpectator, "spectator")
	cs, cleanup := startMCPSession(t, fx.wsURL, specToken)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "move_token",
		Arguments: map[string]any{
			"tokenId": "tok-smoke",
			"to":      map[string]any{"x": 0, "y": 0},
		},
	})
	if err != nil {
		t.Fatalf("spectator move_token: want a tool-level result (err=nil), got protocol error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("spectator move_token: want IsError=true, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("spectator move_token: want text content, got %T", res.Content[0])
	}
	var result vttv1.CommandResult
	if err := protojson.Unmarshal([]byte(text.Text), &result); err != nil {
		t.Fatalf("spectator move_token: result not valid CommandResult protojson: %v", err)
	}
	if result.GetOk() {
		t.Fatalf("spectator move_token: want ok=false, got %+v", &result)
	}
	if !strings.Contains(result.GetError(), "not authorized") || !strings.Contains(result.GetError(), "spectator") {
		t.Fatalf("spectator move_token: error = %q, want the gateway's authz message naming role %q", result.GetError(), "spectator")
	}

	// Connection intact: a follow-up get_state on the SAME session succeeds
	// (callGetStateGeneric itself fatals on IsError, which is exactly this
	// proof).
	_ = callGetStateGeneric(t, cs)
}

// --- world-layer (Task 3) round-trip: add_narration + upsert_note --------

// mustCallToolOK calls name/args against cs and asserts a plain (non-batch)
// ok=true CommandResult, returning its Sequence — the SAME decode shape
// playScenarioThroughTools uses per-step, pulled out here since this test
// calls tools directly rather than playing a scenario file.
func mustCallToolOK(t *testing.T, cs *mcpsdk.ClientSession, name string, args map[string]any) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: CallTool: %v", name, err)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("%s: want text content, got %T", name, res.Content[0])
	}
	var result vttv1.CommandResult
	if err := protojson.Unmarshal([]byte(text.Text), &result); err != nil {
		t.Fatalf("%s: result not valid CommandResult protojson: %v", name, err)
	}
	if res.IsError || !result.GetOk() {
		t.Fatalf("%s: want ok=true, got IsError=%v result=%+v", name, res.IsError, &result)
	}
	return result.GetSequence()
}

// TestMCPWorldLayerRoundTrip is world-layer (Task 3)'s own MCP exit
// criterion (spec §6): add_narration and upsert_note called against a REAL
// composeServer gateway (startMCPFixture, the same live-server fixture
// TestMCPSpecSevenExitCriteria uses) round-trip all the way to BOTH read
// paths — the upserted note surfaces in get_state's folded Notes map, and
// the narration surfaces as its own event in get_events_since — proving the
// wiring this task added is real, not just individually unit-tested at the
// gateway/harness layers.
func TestMCPWorldLayerRoundTrip(t *testing.T) {
	fx := startMCPFixture(t)
	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	defer cleanup()

	const narrationText = "The party enters the ruined keep."
	const noteKey = "ruined-keep"
	const noteTitle = "Ruined Keep"
	const noteText = "Collapsed east wall; goblins nest within."

	narrationSeq := mustCallToolOK(t, cs, "add_narration", map[string]any{"text": narrationText})
	noteSeq := mustCallToolOK(t, cs, "upsert_note", map[string]any{
		"key": noteKey, "title": noteTitle, "text": noteText,
	})
	if noteSeq <= narrationSeq {
		t.Fatalf("upsert_note sequence %d, want it to follow add_narration's sequence %d", noteSeq, narrationSeq)
	}

	// --- note visible in get_state ---
	//
	// get_state is NOT protojson (read_tools.go's own doc comment): top-
	// level keys are the exact Go struct field names off *engine.State
	// (e.g. "Notes"), and Note's own fields ("Title"/"Text"/"UpdatedSeq")
	// the same way — never protojson's camelCase.
	gotState := waitForMCPHeadSequence(t, cs, noteSeq)
	notes, ok := gotState["Notes"].(map[string]any)
	if !ok {
		t.Fatalf(`get_state: "Notes" = %#v, want a JSON object`, gotState["Notes"])
	}
	note, ok := notes[noteKey].(map[string]any)
	if !ok {
		t.Fatalf("get_state: Notes[%q] = %#v, want a JSON object (the upserted note)", noteKey, notes[noteKey])
	}
	if note["Title"] != noteTitle {
		t.Fatalf("get_state: Notes[%q].Title = %#v, want %q", noteKey, note["Title"], noteTitle)
	}
	if note["Text"] != noteText {
		t.Fatalf("get_state: Notes[%q].Text = %#v, want %q", noteKey, note["Text"], noteText)
	}
	if updatedSeq, ok := note["UpdatedSeq"].(float64); !ok || int64(updatedSeq) != noteSeq {
		t.Fatalf("get_state: Notes[%q].UpdatedSeq = %#v, want %d", noteKey, note["UpdatedSeq"], noteSeq)
	}

	// --- narration visible in get_events_since ---
	assertNarrationVisibleInEventsSince(t, cs, narrationSeq, narrationText)
}

// assertNarrationVisibleInEventsSince pages get_events_since from the
// beginning and asserts exactly one envelope at wantSeq carries a
// NarrationAdded payload with the given text — each returned event is its
// own protojson encoding (get_events_since's own wire convention, unlike
// get_state), so it decodes straight into *vttv1.Envelope.
func assertNarrationVisibleInEventsSince(t *testing.T, cs *mcpsdk.ClientSession, wantSeq int64, wantText string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "get_events_since",
		Arguments: map[string]any{"afterSequence": int64(0), "limit": 200},
	})
	if err != nil {
		t.Fatalf("get_events_since: CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("get_events_since: want IsError=false, got %+v", res)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_events_since: want text content, got %T", res.Content[0])
	}
	var page struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal([]byte(text.Text), &page); err != nil {
		t.Fatalf("get_events_since: response did not decode: %v (body: %s)", err, text.Text)
	}

	var found *vttv1.NarrationAdded
	var foundSeq int64
	for _, raw := range page.Events {
		var env vttv1.Envelope
		if err := protojson.Unmarshal(raw, &env); err != nil {
			t.Fatalf("get_events_since: envelope did not decode as protojson: %v (body: %s)", err, raw)
		}
		if na := env.GetNarrationAdded(); na != nil {
			found = na
			foundSeq = env.GetSequence()
			break
		}
	}
	if found == nil {
		t.Fatalf("get_events_since: no NarrationAdded event found among %d events", len(page.Events))
	}
	if foundSeq != wantSeq {
		t.Fatalf("get_events_since: NarrationAdded sequence = %d, want %d", foundSeq, wantSeq)
	}
	if found.Text != wantText {
		t.Fatalf("get_events_since: NarrationAdded.Text = %q, want %q", found.Text, wantText)
	}
}

// TestMCPAddNarrationWithAnchorsRoundTrips closes the merge-gate MUST-FIX
// anchor-coverage gap: neither TestMCPWorldLayerRoundTrip nor its fake-wire
// counterpart (internal/mcp/world_layer_e2e_test.go) ever sent
// anchorFromSeq/anchorToSeq, so the generic dispatch's int64 anchor decode
// (internal/mcp/server.go's protojson.Unmarshal, fed args where
// add_narration's own inputSchema declares both fields "type": "integer" —
// the ordinary MCP-tool-argument convention, NOT protojson's quoted-string
// int64 convention) had zero end-to-end coverage in either direction. This
// test sends a valid BACKWARD anchor pair as plain JSON integers against a
// REAL composeServer gateway (the same live-server fixture
// TestMCPWorldLayerRoundTrip uses) and asserts both the call succeeds and
// the anchors the engine fold accepted are exactly what comes back out of
// get_events_since.
func TestMCPAddNarrationWithAnchorsRoundTrips(t *testing.T) {
	fx := startMCPFixture(t)
	cs, cleanup := startMCPSession(t, fx.wsURL, fx.agentToken)
	defer cleanup()

	firstSeq := mustCallToolOK(t, cs, "add_narration", map[string]any{
		"text": "The party arrives at the ruined gate.",
	})

	const anchoredText = "The rusted portcullis groans upward."
	anchoredSeq := mustCallToolOK(t, cs, "add_narration", map[string]any{
		"text":          anchoredText,
		"anchorFromSeq": firstSeq,
		"anchorToSeq":   firstSeq,
	})
	if anchoredSeq <= firstSeq {
		t.Fatalf("anchored add_narration sequence %d, want it to follow the first narration's sequence %d", anchoredSeq, firstSeq)
	}

	// POLLED, not read once.
	//
	// SendCommand returning does not mean that command's OWN broadcast has
	// reached this session's event history — client.go's SendCommand doc
	// comment says so, and get_events_since deliberately reports only what the
	// session has SEEN (which is what headSequence exists to signal). Reading
	// immediately races that propagation, and this test failed exactly that
	// way under load: "no NarrationAdded event found at sequence 2 among 1
	// events" — one event short, intermittently, on CI and on a loaded laptop.
	//
	// Waiting for the CONDITION is the fix, as it was for the soak checkpoint
	// and for session.test.ts. The deadline is a backstop; a healthy round
	// trip satisfies this on the first attempt.
	var found *vttv1.NarrationAdded
	lastCount := 0
	deadline := time.Now().Add(10 * time.Second)
	for attempt := 1; found == nil; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
			Name:      "get_events_since",
			Arguments: map[string]any{"afterSequence": int64(0), "limit": 200},
		})
		cancel()
		if err != nil {
			t.Fatalf("get_events_since: CallTool: %v", err)
		}
		if res.IsError {
			t.Fatalf("get_events_since: want IsError=false, got %+v", res)
		}
		text, ok := res.Content[0].(*mcpsdk.TextContent)
		if !ok {
			t.Fatalf("get_events_since: want text content, got %T", res.Content[0])
		}
		var page struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := json.Unmarshal([]byte(text.Text), &page); err != nil {
			t.Fatalf("get_events_since: response did not decode: %v (body: %s)", err, text.Text)
		}
		lastCount = len(page.Events)
		for _, raw := range page.Events {
			var env vttv1.Envelope
			if err := protojson.Unmarshal(raw, &env); err != nil {
				t.Fatalf("get_events_since: envelope did not decode as protojson: %v (body: %s)", err, raw)
			}
			if env.GetSequence() != anchoredSeq {
				continue
			}
			if na := env.GetNarrationAdded(); na != nil {
				found = na
				break
			}
		}
		if found == nil && time.Now().After(deadline) {
			t.Fatalf("get_events_since: no NarrationAdded at sequence %d after %d attempts (last page had %d events) — the session never observed its own narration", anchoredSeq, attempt, lastCount)
		}
		if found == nil {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if found.Text != anchoredText {
		t.Fatalf("get_events_since: NarrationAdded.Text = %q, want %q", found.Text, anchoredText)
	}
	if found.AnchorFromSeq != firstSeq || found.AnchorToSeq != firstSeq {
		t.Fatalf("get_events_since: NarrationAdded anchors = (%d, %d), want (%d, %d)",
			found.AnchorFromSeq, found.AnchorToSeq, firstSeq, firstSeq)
	}
}
