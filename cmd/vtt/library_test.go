package main

// library_test.go (task-4-brief.md Step 2/2b) is the scenario library
// runner: it globs scenarios/*.json (the committed library, one directory
// up from cmd/vtt at the repo root) and runs each one self-contained,
// asserting Report.Pass with per-step diagnostics on failure — so the
// library executes inside `task check` on every commit, forever (plan
// Task 4's binding). A second test runs three-role-exit.json specifically
// against a REAL `vtt serve` subprocess (spec §8's literal exit criterion:
// "green via self-contained run AND against a live vtt serve process").
//
// This file lives in cmd/vtt, not internal/harness, deliberately (brief's
// own file-list note): the library runner needs the self-contained boot
// glue (composeServer, invite minting) that only cmd/vtt is allowed to
// import (internal/harness's P1 arch rule forbids it — see client.go's
// package comment) — "cmd may compose".
//
// Participant-id placeholder resolution (P6 Task 4 fix round): this file no
// longer does any of its own — internal/harness/engine.go's RunScenario now
// takes an ids parameter and resolves scenarios/three-role-exit.json's
// {{id:player}} placeholder itself, once, before dispatch. Both tests below
// get their ids map "for free": the self-contained path from
// bootSelfContained's IDs field (harness_boot.go), the live-subprocess path
// from mintInvites (also harness_boot.go, reused directly rather than
// duplicated) threaded into tokens.json's additive "ids" field.

import (
	"context"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// scenarioLibraryGlob finds the committed scenario library relative to this
// package's directory (cmd/vtt) — the repo root's scenarios/ dir.
const scenarioLibraryGlob = "../../scenarios/*.json"

// TestScenarioLibraryRunsSelfContained globs scenarios/*.json and runs each
// one self-contained (its own fresh temp campaign + server instance, via
// bootSelfContained — the exact same boot glue `vtt client run` itself uses
// with no --server/--tokens), asserting Report.Pass. A failing scenario's
// t.Fatalf includes RunScenario's own per-step/per-probe human log (every
// step always runs — see internal/harness/engine.go's RunScenario doc
// comment — so a failure names exactly which step or probe broke, not just
// "something failed").
func TestScenarioLibraryRunsSelfContained(t *testing.T) {
	paths, err := filepath.Glob(scenarioLibraryGlob)
	if err != nil {
		t.Fatalf("glob %s: %v", scenarioLibraryGlob, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no scenarios found matching %s — committed library is empty or path is wrong", scenarioLibraryGlob)
	}

	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			runLibraryScenarioSelfContained(t, path)
		})
	}
}

// runLibraryScenarioSelfContained loads one scenario file, boots a fresh
// self-contained server for it alone (bootSelfContained: isolation between
// scenarios in the same glob run, same as a bare `vtt client run
// <this-file>` invocation would get), and runs it — failing the test with
// the full step/probe diagnostic log on Report.Pass=false.
func runLibraryScenarioSelfContained(t *testing.T, path string) {
	t.Helper()

	sc, err := harness.LoadScenario(path)
	if err != nil {
		t.Fatalf("LoadScenario(%s): %v", path, err)
	}

	boot, err := bootSelfContained(sc)
	if err != nil {
		t.Fatalf("bootSelfContained(%s): %v", path, err)
	}
	t.Cleanup(func() {
		if err := boot.close(); err != nil {
			t.Errorf("boot.close() for %s: %v", path, err)
		}
	})

	dial := dialerFor(boot.WSURL, boot.Tokens)

	var log strings.Builder
	rep, err := harness.RunScenario(context.Background(), sc, dial, boot.IDs, &log)
	if err != nil {
		t.Fatalf("RunScenario(%s): %v", path, err)
	}
	if !rep.Pass {
		t.Fatalf("scenario %s did not pass:\n%s", path, log.String())
	}
}

// --- vtt serve as a real subprocess (spec §8 literal) -----------------

// TestThreeRoleExitScenarioOverLiveServeSubprocess is spec §8's second exit
// criterion: three-role-exit.json green not just self-contained (covered by
// TestScenarioLibraryRunsSelfContained's glob) but against a REAL `vtt
// serve` process — the actual built binary, run as an OS subprocess on a
// temp campaign and a freshly-picked free port, with invites minted
// test-side (mintInvites, harness_boot.go — reused directly, not
// duplicated) and the ORIGINAL, unmodified committed scenario file driven
// via `vtt client run --server --tokens` (in-process, through runCLI —
// cli_test.go's established pattern; only the SERVER side needs to be a
// genuine separate process for this to prove anything the in-process
// composeServer tests don't already cover). The minted participant ids flow
// through tokens.json's additive "ids" field — RunScenario resolves
// three-role-exit.json's {{id:player}} placeholder from that, the same
// mechanism a real operator would use (tokens.json's ids come from `vtt
// invite`'s own printed "participant id: ..." line).
// Process.Kill is the documented teardown (brief, verbatim): `vtt serve`
// has no graceful-shutdown RunE path yet (see serve.go/serve_compose.go's
// own doc comments) — the connection-drain carry-forward
// (docs/superpowers/sdd/progress.md) owns closing that gap, not this test.
func TestThreeRoleExitScenarioOverLiveServeSubprocess(t *testing.T) {
	binPath := buildVTTBinary(t)

	dir := t.TempDir()
	campaignPath := filepath.Join(dir, "campaign.db")
	addr := mustFreeAddr(t)

	cmd := exec.Command(binPath, "serve", "--campaign", campaignPath, "--addr", addr)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start vtt serve subprocess: %v", err)
	}
	// Safety-net teardown in case the test fails/fatals before the explicit
	// Kill below ever runs (t.Fatalf unwinds via runtime.Goexit, skipping
	// the rest of the function body but still running registered Cleanups).
	// A second Kill on an already-dead process is a harmless no-op error.
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	base := "http://" + addr
	if err := waitForHealthz(base, 5*time.Second); err != nil {
		t.Fatalf("vtt serve subprocess healthz never became ready: %v", err)
	}

	scenarioPath := filepath.Join("..", "..", "scenarios", "three-role-exit.json")
	sc, err := harness.LoadScenario(scenarioPath)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	tokens, ids, err := mintInvites(campaignPath, sc)
	if err != nil {
		t.Fatalf("mint invites: %v", err)
	}
	tokensPath := writeTokensFile(t, tokens, ids)

	wsURL := "ws://" + addr + "/ws"
	out, err := runCLI(t, "client", "run", scenarioPath, "--server", wsURL, "--tokens", tokensPath)
	if err != nil {
		t.Fatalf("vtt client run (live subprocess mode): %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "pass=true") {
		t.Fatalf("vtt client run (live subprocess mode) output missing passing steps: %q", out)
	}
	if strings.Contains(out, "pass=false") {
		t.Fatalf("vtt client run (live subprocess mode) output contains a failing step: %q", out)
	}

	// The documented teardown (brief, verbatim: "Process.Kill documented as
	// the teardown; no graceful path exists").
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill vtt serve subprocess: %v", err)
	}
	_ = cmd.Wait()
}

// mustFreeAddr picks a free loopback address by listening on port 0 and
// immediately closing the listener, handing the now-free address to a
// caller who will bind a DIFFERENT process to it (brief, verbatim: "pick a
// free port by listening+closing"). This has an inherent (accepted, per the
// brief) TOCTOU window: something else could grab the port between Close
// and the subprocess's own bind — the same tradeoff any "find a free port"
// helper without OS-level reservation has.
func mustFreeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick free port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close free-port listener: %v", err)
	}
	return addr
}
