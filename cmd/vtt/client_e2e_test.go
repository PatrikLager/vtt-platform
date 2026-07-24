package main

// client_e2e_test.go drives the cobra root in-process (runCLI, cli_test.go's
// established pattern) against `vtt client run`, `vtt events tail`, and
// `vtt state dump` (task-3-brief.md Step 1): a minimal inline scenario run
// self-contained (pass + exit 0), a failing scenario (exit 1 + --json report
// naming the failing step), a live-mode run against a composeServer instance
// this test starts itself with a hand-written tokens.json, and tail/dump run
// against that SAME live instance afterward — tail bounded to N lines then
// stopped via context cancellation, dump's printed state JSON checked for an
// expected token position.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// --- vtt client run: self-contained ---------------------------------------

// TestClientRunSelfContainedMinimalScenarioPasses runs a one-step scenario
// with no --server/--tokens: client_run.go's default boots a throwaway
// server via harness_boot.go, mints the dm's invite, runs, and tears down —
// exit 0, human step log mentions the passing step.
func TestClientRunSelfContainedMinimalScenarioPasses(t *testing.T) {
	path := writeScenarioFile(t, `{
		"name": "self-contained-smoke",
		"participants": [{"name": "dm", "role": "dm"}],
		"steps": [
			{"by": "dm", "command": {"startSession": {"name": "s1"}}, "expect": {"ok": true}}
		],
		"probes": []
	}`)

	out, err := runCLI(t, "client", "run", path)
	if err != nil {
		t.Fatalf("client run: unexpected error: %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "pass=true") {
		t.Fatalf("client run output missing a passing step: %q", out)
	}
}

// TestClientRunSelfContainedFailingScenarioReportsFailingStepAndExitsNonZero
// runs a scenario whose one step expects a denial that never happens (a
// dm's StartSession always succeeds) — the run must fail: exit 1, and the
// --json report must name the failing step with Pass=false and a Detail
// explaining why, proving the "report says which step" and "--json report
// shape" requirements together.
func TestClientRunSelfContainedFailingScenarioReportsFailingStepAndExitsNonZero(t *testing.T) {
	path := writeScenarioFile(t, `{
		"name": "self-contained-failing",
		"participants": [{"name": "dm", "role": "dm"}],
		"steps": [
			{"by": "dm", "command": {"startSession": {"name": "s1"}}, "expect": {"deniedContaining": "nonsense"}}
		],
		"probes": []
	}`)

	out, err := runCLI(t, "client", "run", path, "--json")
	if err == nil {
		t.Fatal("client run: want a non-nil error (exit 1) for a failing scenario, got nil")
	}

	var rep harness.Report
	if jsonErr := json.Unmarshal([]byte(out), &rep); jsonErr != nil {
		t.Fatalf("client run --json: output did not decode as harness.Report: %v (output: %s)", jsonErr, out)
	}
	if rep.Pass {
		t.Fatalf("report.Pass = true, want false: %+v", rep)
	}
	if len(rep.Steps) != 1 {
		t.Fatalf("len(report.Steps) = %d, want 1: %+v", len(rep.Steps), rep)
	}
	step := rep.Steps[0]
	if step.Pass {
		t.Fatalf("report.Steps[0].Pass = true, want false: %+v", step)
	}
	if step.Index != 0 {
		t.Fatalf("report.Steps[0].Index = %d, want 0 (names the failing step)", step.Index)
	}
	if !strings.Contains(step.Detail, "ok=true") {
		t.Fatalf("report.Steps[0].Detail = %q, want it to explain the mismatch", step.Detail)
	}
	// The prior "len(rep.Probes) != 0" check here was vacuous: this
	// scenario declares zero probes, so rep.Probes decodes to length 0 for
	// EVERY implementation, correct or broken alike — no production
	// behavior could ever fail it. What's actually decidable is whether the
	// "Probes" key is present in the RAW JSON output at all (Report has no
	// `omitempty` tag today, so it always is) — checked against out before
	// json.Unmarshal has already thrown that information away, so this
	// would catch a future omitempty regression the length check never
	// could.
	if !strings.Contains(out, `"Probes"`) {
		t.Fatalf("client run --json output missing the \"Probes\" key (shape check: the field must always be present, never omitted): %s", out)
	}
}

// TestClientRunMissingScenarioArgErrors covers the required positional arg
// (cobra.ExactArgs(1)), the same "flag validation before RunE" shape
// cli_test.go's TestServeMissingCampaignErrors covers for serve.
func TestClientRunMissingScenarioArgErrors(t *testing.T) {
	if _, err := runCLI(t, "client", "run"); err == nil {
		t.Fatal("client run: want error for missing scenario path argument")
	}
}

// TestClientRunSelfContainedRunsCommittedThreeRoleExitScenario is the P6
// Task 4 fix-round proof: a completely BARE `vtt client run
// scenarios/three-role-exit.json`, self-contained (no --server/--tokens, no
// test-side substitution of any kind — this is exactly what a human running
// the committed file straight from a shell would type) must pass on its
// own. scenarios/three-role-exit.json's "act-lera" AddActor step embeds
// {{id:player}} in its controllerId; before this fix round nothing supplied
// an ids map to harness.RunScenario at all (the parameter didn't exist),
// and mintInvites (harness_boot.go) discarded the minted participant id via
// `token, _, err` — so this scenario's "player moves own token" step always
// came back denied (ControllerId stayed the literal, never-matching string
// "{{id:player}}"). This is genuine behavioral RED against the pre-fix
// code, captured in the P6 Task 4 report's fix-round section (the assertion
// below is exactly what failed, at the exact step the report names), not a
// hypothetical: `vtt client run`'s self-contained boot glue now threads
// bootSelfContained's IDs field through to RunScenario's ids parameter, so
// this resolves automatically.
func TestClientRunSelfContainedRunsCommittedThreeRoleExitScenario(t *testing.T) {
	out, err := runCLI(t, "client", "run", filepath.Join("..", "..", "scenarios", "three-role-exit.json"))
	if err != nil {
		t.Fatalf("client run (bare, self-contained, committed scenario): unexpected error: %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "pass=true") {
		t.Fatalf("client run output missing passing steps: %q", out)
	}
	if strings.Contains(out, "pass=false") {
		t.Fatalf("client run output contains a failing step: %q", out)
	}
}

// --- vtt client soak: the real, self-contained soak e2e (plan Task 5) ------

// TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts is
// task-5-brief.md's Step 2: a completely bare, self-contained `vtt client
// soak --seed 1 --events 500` (no --server/--tokens — boots its own
// throwaway server via harness_boot.go's bootSelfContained, exactly like a
// human running this straight from a shell would) must Pass, exercising the
// checkpoint fold-equality machinery at least once (Checkpoints > 0). The
// seed-1 accepted/denied/checkpoint counts are DETERMINISTIC (the
// generator-determinism proof in internal/harness/soak_test.go covers WHY)
// and are pinned here verbatim, recorded by running the exact same command
// against the real self-contained boot path
// (.superpowers/sdd/p6-task-5-report.md's transcript): Events=500,
// Accepted=480, Denied=20, Checkpoints=5.
func TestClientSoakSelfContainedSeed1Events500PassesWithPinnedCounts(t *testing.T) {
	out, err := runCLI(t, "client", "soak", "--seed", "1", "--events", "500", "--json")
	if err != nil {
		t.Fatalf("client soak (bare, self-contained, seed=1, events=500): unexpected error: %v (output: %s)", err, out)
	}

	var rep harness.SoakReport
	if jsonErr := json.Unmarshal([]byte(out), &rep); jsonErr != nil {
		t.Fatalf("client soak --json: output did not decode as harness.SoakReport: %v (output: %s)", jsonErr, out)
	}

	if !rep.Pass {
		t.Fatalf("Report.Pass = false, want true: %+v", rep)
	}
	if rep.Checkpoints <= 0 {
		t.Fatalf("Report.Checkpoints = %d, want > 0 (the checkpoint fold-equality machinery must actually run)", rep.Checkpoints)
	}

	const (
		wantEvents      = 500
		wantAccepted    = 480
		wantDenied      = 20
		wantCheckpoints = 5
	)
	if rep.Events != wantEvents {
		t.Errorf("Report.Events = %d, want %d (pinned, seed=1)", rep.Events, wantEvents)
	}
	if rep.Accepted != wantAccepted {
		t.Errorf("Report.Accepted = %d, want %d (pinned, seed=1)", rep.Accepted, wantAccepted)
	}
	if rep.Denied != wantDenied {
		t.Errorf("Report.Denied = %d, want %d (pinned, seed=1)", rep.Denied, wantDenied)
	}
	if rep.Checkpoints != wantCheckpoints {
		t.Errorf("Report.Checkpoints = %d, want %d (pinned, seed=1)", rep.Checkpoints, wantCheckpoints)
	}
	if rep.Denied != rep.Counts["deniedAttempt"] {
		t.Errorf("Report.Denied = %d, Counts[deniedAttempt] = %d, want equal (players-only-move-own invariant: nothing outside the deliberate bucket should ever be denied)",
			rep.Denied, rep.Counts["deniedAttempt"])
	}
}

// TestClientSoakMissingRequiredFlagsErrors covers --seed/--events' required-
// flag validation, the same shape cli_test.go's TestServeMissingCampaignErrors
// covers for serve and client_e2e_test.go's TestEventsTailMissingFlagsErrors
// covers for events tail.
func TestClientSoakMissingRequiredFlagsErrors(t *testing.T) {
	_, err := runCLI(t, "client", "soak")
	if err == nil {
		t.Fatal("client soak: want error for missing --seed/--events flags")
	}
	if !strings.Contains(err.Error(), "required flag") {
		t.Fatalf("client soak: error = %q, want it to name the missing required flag(s)", err.Error())
	}
}

// --- vtt client run --server/--tokens (live mode), events tail, state dump ---

// liveFixture is one composeServer instance this test starts and tears
// down itself (the test is in package main / cmd, which is allowed to
// compose — task-3-brief.md Step 1), with one dm participant's invite
// minted directly via identity, mirroring serve_e2e_test.go's pattern.
type liveFixture struct {
	wsURL   string
	dmToken string
}

// startLiveFixture boots a real gateway on an OS-assigned loopback port,
// waits for it to be ready, and mints a dm invite. t.Cleanup tears the
// server down.
func startLiveFixture(t *testing.T) liveFixture {
	t.Helper()
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0")
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

	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	defer ids.Close()
	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	return liveFixture{wsURL: "ws://" + ln.Addr().String() + "/ws", dmToken: dmToken}
}

// liveModeScenario is a four-command dm scenario (start session, create a
// scene, add an actor, place that actor's token at (3, 4)) — deliberately
// the same shape events tail and state dump assert against below, so all
// three commands in this task run against ONE shared live instance and ONE
// shared bit of history (plan Task 3's binding: "tail/dump: against the
// same live instance").
const liveModeScenario = `{
	"name": "live-mode",
	"participants": [{"name": "dm", "role": "dm"}],
	"steps": [
		{"by": "dm", "command": {"startSession": {"name": "s1"}}, "expect": {"ok": true}},
		{"by": "dm", "command": {"createScene": {"sceneId": "scene-1", "name": "Scene One", "gridWidth": 10, "gridHeight": 10}}, "expect": {"ok": true}},
		{"by": "dm", "command": {"addActor": {"actor": {"actorId": "act-1", "name": "Actor One"}}}, "expect": {"ok": true}},
		{"by": "dm", "command": {"placeToken": {"tokenId": "tok-1", "sceneId": "scene-1", "actorId": "act-1", "position": {"x": 3, "y": 4}}}, "expect": {"ok": true}}
	],
	"probes": [
		{"tokenAt": {"tokenId": "tok-1", "x": 3, "y": 4}}
	]
}`

// TestClientRunLiveModeAgainstComposeServer runs liveModeScenario in live
// mode (--server/--tokens) against a composeServer instance the TEST
// started, with a hand-written tokens.json in the {"participants": {name:
// token}} shape (task-3-brief.md's binding format) — exit 0.
func TestClientRunLiveModeAgainstComposeServer(t *testing.T) {
	fx := startLiveFixture(t)
	scenarioPath := writeScenarioFile(t, liveModeScenario)
	tokensPath := writeTokensFile(t, map[string]string{"dm": fx.dmToken}, nil)

	out, err := runCLI(t, "client", "run", scenarioPath, "--server", fx.wsURL, "--tokens", tokensPath)
	if err != nil {
		t.Fatalf("client run (live mode): unexpected error: %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "pass=true") {
		t.Fatalf("client run (live mode) output missing passing steps: %q", out)
	}

	t.Run("events tail streams N lines then stops on context cancel", func(t *testing.T) {
		testEventsTailAgainstFixture(t, fx)
	})
	t.Run("state dump prints the expected token position", func(t *testing.T) {
		testStateDumpAgainstFixture(t, fx)
	})
}

// testEventsTailAgainstFixture dials `vtt events tail` against fx's live
// server (already carrying the four liveModeScenario events from the
// parent test), reads exactly 4 protojson lines, cancels the command's
// context, and asserts the command returns with no error (a canceled tail
// is a clean stop, not a failure) and every line decodes as a
// *vttv1.Envelope.
func testEventsTailAgainstFixture(t *testing.T, fx liveFixture) {
	t.Helper()
	const wantLines = 4

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := newSyncBuffer()
	done := runCLIStreaming(ctx, out, "events", "tail", "--server", fx.wsURL, "--token", fx.dmToken)

	lines := out.waitForLines(t, wantLines, 5*time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("events tail: want nil error after context cancel, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("events tail: did not return within 5s of context cancel")
	}

	if len(lines) != wantLines {
		t.Fatalf("events tail: got %d lines by cancel time, want %d: %q", len(lines), wantLines, lines)
	}
	for i, line := range lines {
		var env vttv1.Envelope
		if err := protojson.Unmarshal([]byte(line), &env); err != nil {
			t.Fatalf("events tail: line %d does not decode as protojson Envelope: %v (line: %q)", i, err, line)
		}
		if env.Sequence != int64(i+1) {
			t.Fatalf("events tail: line %d sequence = %d, want %d", i, env.Sequence, i+1)
		}
	}
}

// dumpStateShape mirrors internal/engine.State's exported field names for
// the one sub-shape this test checks (Tokens), without cmd/vtt importing
// internal/engine directly — cmd may depend on harness (which returns
// *engine.State from Fold) but NOT on engine itself
// (.go-arch-lint.yml's cmd.mayDependOn list); state_dump.go itself never
// names the engine package either, letting Go infer harness.Fold's return
// type instead (see its own comment) — this struct is the JSON-shape
// equivalent for a test-side assertion, not a live import of the type.
type dumpStateShape struct {
	Tokens map[string]struct {
		ID, SceneID, ActorID string
		X, Y                 int32
	}
	// HeadSequence is the highest envelope sequence this dump actually
	// received before folding — the caller's staleness check, since a dump
	// is a point-in-time snapshot, not a live guarantee (see
	// state_dump.go's doc comment).
	HeadSequence int64 `json:"headSequence"`
}

// testStateDumpAgainstFixture runs `vtt state dump` against fx's live
// server and asserts the printed JSON places tok-1 at (3, 4) — the
// position liveModeScenario's PlaceToken step set — and that headSequence
// equals 4, the known last sequence of the parent test's four-command
// history (startSession, createScene, addActor, placeToken).
func testStateDumpAgainstFixture(t *testing.T, fx liveFixture) {
	t.Helper()
	out, err := runCLI(t, "state", "dump", "--server", fx.wsURL, "--token", fx.dmToken)
	if err != nil {
		t.Fatalf("state dump: unexpected error: %v (output: %s)", err, out)
	}

	var st dumpStateShape
	if err := json.Unmarshal([]byte(out), &st); err != nil {
		t.Fatalf("state dump: output did not decode as expected state JSON: %v (output: %s)", err, out)
	}
	tok, ok := st.Tokens["tok-1"]
	if !ok {
		t.Fatalf("state dump: Tokens[\"tok-1\"] missing from output: %s", out)
	}
	if tok.X != 3 || tok.Y != 4 {
		t.Fatalf("state dump: tok-1 position = (%d, %d), want (3, 4)", tok.X, tok.Y)
	}
	if tok.SceneID != "scene-1" || tok.ActorID != "act-1" {
		t.Fatalf("state dump: tok-1 = %+v, want SceneID=scene-1 ActorID=act-1", tok)
	}
	if st.HeadSequence != 4 {
		t.Fatalf("state dump: headSequence = %d, want 4 (the last of the four seeded commands)", st.HeadSequence)
	}
}

// TestEventsTailMissingFlagsErrors and TestStateDumpMissingFlagsErrors cover
// --server/--token's required-flag validation, the same shape
// cli_test.go's TestServeMissingCampaignErrors covers for serve. Both
// assert on cobra's specific "required flag(s) ... not set" error text
// (not just "an error occurred") deliberately: RunE's own downstream
// harness.Dial("", "", ...) also errors on empty inputs, so a weaker
// err!=nil assertion would still pass even with MarkFlagRequired removed —
// see the ADR-009 fault-injection proof transcripts in
// .superpowers/sdd/p6-task-3-report.md's Fix 3(b) section, which is
// exactly what caught that and is why these check the specific text.
func TestEventsTailMissingFlagsErrors(t *testing.T) {
	_, err := runCLI(t, "events", "tail")
	if err == nil {
		t.Fatal("events tail: want error for missing --server/--token flags")
	}
	if !strings.Contains(err.Error(), "required flag") {
		t.Fatalf("events tail: error = %q, want it to name the missing required flag(s)", err.Error())
	}
}

func TestStateDumpMissingFlagsErrors(t *testing.T) {
	_, err := runCLI(t, "state", "dump")
	if err == nil {
		t.Fatal("state dump: want error for missing --server/--token flags")
	}
	if !strings.Contains(err.Error(), "required flag") {
		t.Fatalf("state dump: error = %q, want it to name the missing required flag(s)", err.Error())
	}
}

// --- vtt events tail: real OS signal delivery (binary-level) ---------------

// TestEventsTailBinaryExitsCleanlyOnSIGINT is the one case in-process cobra
// driving (runCLI/runCLIStreaming) cannot cover: real OS signal delivery
// only reaches a real process, not a goroutine inside the test binary. It
// builds the actual vtt binary, runs `events tail` as a SUBPROCESS against
// a live composeServer instance, waits for it to have caught up (one
// seeded event observed on its stdout) and be blocked tailing, sends it
// SIGINT, and asserts it exits promptly with code 0 — a clean, cancel-
// driven stop — rather than being torn down by the signal's OS-default
// action (main.go must translate the signal into a canceled context for
// this to hold; see main.go's own comment).
func TestEventsTailBinaryExitsCleanlyOnSIGINT(t *testing.T) {
	binPath := buildVTTBinary(t)
	fx := startLiveFixture(t)
	seedOneEvent(t, fx)

	cmd := exec.Command(binPath, "events", "tail", "--server", fx.wsURL, "--token", fx.dmToken)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start subprocess: %v", err)
	}

	// Wait for the seeded event's catch-up line: proof the subprocess has
	// dialed, replayed history, and is now blocked in its select loop
	// (rather than still mid-startup) — the meaningful moment to signal it.
	gotLine := make(chan struct{}, 1)
	go func() {
		if bufio.NewScanner(stdout).Scan() {
			gotLine <- struct{}{}
		}
	}()
	select {
	case <-gotLine:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("subprocess produced no output within 5s")
	}

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send SIGINT: %v", err)
	}

	if err := waitWithTimeout(cmd, 5*time.Second); err != nil {
		t.Fatalf("subprocess did not exit cleanly after SIGINT: %v", err)
	}
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("subprocess exit code after SIGINT = %d, want 0", code)
	}
}

// buildVTTBinary builds the real `vtt` binary from this package (`.` — the
// test process's working directory is cmd/vtt, Go testing's own
// convention) into t.TempDir() and returns its path.
func buildVTTBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "vtt")
	build := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/vtt: %v\n%s", err, out)
	}
	return binPath
}

// seedOneEvent dials fx directly (in-process, via internal/harness — not
// the subprocess under test) and sends one StartSession, so the SIGINT
// test has a deterministic single catch-up line to wait for.
func seedOneEvent(t *testing.T, fx liveFixture) {
	t.Helper()
	c, err := harness.Dial(context.Background(), fx.wsURL, fx.dmToken, 0)
	if err != nil {
		t.Fatalf("seedOneEvent: dial: %v", err)
	}
	defer c.Close()
	res, err := c.SendCommand(context.Background(), &vttv1.ClientCommand{
		Command: &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "seed"}},
	})
	if err != nil {
		t.Fatalf("seedOneEvent: SendCommand: %v", err)
	}
	if !res.Ok {
		t.Fatalf("seedOneEvent: StartSession denied: %s", res.Error)
	}
}

// waitWithTimeout calls cmd.Wait(), bounded by timeout — killing the
// process and returning a timeout error rather than hanging forever if it
// never exits (e.g. a regression where the signal is swallowed and the
// process just keeps tailing).
func waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
		return errors.New("subprocess did not exit within timeout")
	}
}

// --- test helpers ----------------------------------------------------------

func writeScenarioFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writeTokensFile writes tokens.json in the binding shape
// ({"participants": {name: token}, "ids": {name: id}}) — ids is ADDITIVE
// (P6 Task 4 fix round: internal/harness's RunScenario now resolves a
// scenario's {{id:<name>}} placeholders from it) and may be nil, which
// `omitempty` drops from the written JSON entirely, keeping every
// pre-existing caller's tokens.json byte-for-byte identical to before this
// field existed.
func writeTokensFile(t *testing.T, participants, ids map[string]string) string {
	t.Helper()
	body, err := json.Marshal(tokensFile{Participants: participants, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runCLIStreaming builds a fresh root, wires out as both its stdout and
// stderr, and runs root.ExecuteContext(ctx) on a goroutine — unlike
// cli_test.go's runCLI (which blocks until Execute returns), this lets the
// caller observe partial output (via out's waitForLines) WHILE the command
// is still running, then cancel ctx to stop it. The returned channel
// receives Execute's error exactly once, when it returns.
func runCLIStreaming(ctx context.Context, out *syncBuffer, args ...string) <-chan error {
	root := newRootCmd()
	root.SetOut(out)
	root.SetErr(out)
	root.SetArgs(args)

	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(ctx) }()
	return done
}

// syncBuffer is a mutex-guarded bytes.Buffer safe for one goroutine to
// write to (the command under test, via cobra's OutOrStdout) while another
// polls it (the test, via waitForLines) concurrently.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newSyncBuffer() *syncBuffer { return &syncBuffer{} }

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// waitForLines polls s until it holds at least n complete (newline-
// terminated) lines or timeout elapses, then returns exactly those first n
// lines — failing the test on timeout.
func (s *syncBuffer) waitForLines(t *testing.T, n int, timeout time.Duration) []string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lines []string
	for time.Now().Before(deadline) {
		scanner := bufio.NewScanner(strings.NewReader(s.String()))
		lines = nil
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
		if len(lines) >= n {
			return lines[:n]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitForLines: only %d/%d lines within %s (buffer: %q)", len(lines), n, timeout, s.String())
	return nil
}

// waitForHealthz is defined in serve_e2e_test.go (same package) and reused
// here as-is.
