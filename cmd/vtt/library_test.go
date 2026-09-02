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
	"fmt"
	"io/fs"
	"net"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
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

// TestEveryScenarioFileIsActuallyRun fails if a scenario exists that the glob
// above does not execute.
//
// THE GLOB IS ONE DIRECTORY DEEP AND NOTHING SAID SO. `scenarios/*.json` does
// not descend, while the corpus guard that validates this library
// (internal/harness/corpus_actor_kind_test.go) walks every .json under
// scenarios/ AT ANY DEPTH. So a scenario committed to scenarios/arcs/foo.json
// would be validated, would appear in the library to anyone reading the
// directory, and would run ZERO TIMES — the shape this repo keeps finding:
// something that looks covered and is not. Latent when written (all scenarios
// sit at the top level), exactly as a byte-vs-character column was latent while
// every key happened to be ASCII.
//
// The existing "no scenarios found" guard above does not cover this. It catches
// the glob matching NOTHING; this catches it matching only SOME.
//
// TWO subdirectories are excluded, and both hold .json that is not a
// scenario. Feeding either to LoadScenario is a category error.
//
// maps/ holds MAP FILES (2026-09-01-create-scene-leaves Task 7) — the
// mapdef format, one standalone map per file named by its own id, which the
// runner installs into a scenario's campaign so its load_map steps have
// something to name. They are INPUTS to the corpus the way rulesets/ and
// adventures/ are, and they live under scenarios/ rather than beside those
// because they exist only to serve this library. Nothing about them is a
// scenario: they declare no participants and no steps.
// TestEveryMapTheCorpusNamesIsInstalledAndUsed below is what holds them to
// the library instead, in both directions, so excluding them here does not
// leave them unchecked.
//
// goldens/ is excluded because its contents are not scenarios either. How
// each was produced differs, and the differences are load-bearing rather
// than trivia (scenarios/goldens/README.md):
//
//	state.json                    HAND-DERIVED from the scenario definition
//	<name>/stream.json            RECORDED from a real server run
//	<name>/projections/*/stream.json  DERIVED from that recording, and
//	                              byte-pinned by recomputing it
//	<name>/projections/*/viewer.json  DECLARED — which seat a projection is
//	                              for; a perch never reaches the log, so it
//	                              cannot be derived from the stream beside it
//
// The hand-derived state and the recorded stream are arrived at INDEPENDENTLY,
// and that independence is what makes their agreement evidence instead of a
// tautology — regenerating state.json from a run would quietly destroy the
// property the fold gate exists to test. Feeding any of these to LoadScenario
// would be a category error.
//
// The exclusion is deliberately narrow: two NAMED subdirectories, not "any
// subdirectory", so a new subdirectory holding a .json is a failure and
// somebody has to decide what it is. maps/ became the second one by exactly
// that route — it failed here first, and this comment is the decision.
func TestEveryScenarioFileIsActuallyRun(t *testing.T) {
	run := map[string]bool{}
	paths, err := filepath.Glob(scenarioLibraryGlob)
	if err != nil {
		t.Fatalf("glob %s: %v", scenarioLibraryGlob, err)
	}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			t.Fatalf("abs(%s): %v", p, err)
		}
		run[abs] = true
	}

	root := filepath.Dir(scenarioLibraryGlob)
	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// EqualFold on the EXTENSION, matching the corpus guard
		// (internal/harness/corpus_actor_kind_test.go) rather than the glob.
		// A case-sensitive suffix test here would reproduce this test's own
		// premise verbatim: `scenarios/Adventure.JSON` is not matched by
		// `*.json` (filepath.Match is case-sensitive on every platform) so it
		// never runs, IS walked by the corpus guard so it is validated and
		// looks like part of the library — and a case-sensitive check here
		// would skip it before the comparison and say nothing.
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".json") {
			return nil
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		seen[abs] = true
		if run[abs] {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if strings.HasPrefix(slash, "goldens/") || strings.HasPrefix(slash, "maps/") {
			return nil
		}
		t.Errorf("scenarios/%s exists but %s does not match it, so it never runs. "+
			"The glob wants a file at the TOP LEVEL with a lower-case .json extension — "+
			"a nested path and an upper-case .JSON both miss it. Move or rename it, or if it "+
			"is not a scenario at all, exclude it here and say what it is.",
			filepath.ToSlash(rel), scenarioLibraryGlob)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	// THE WALK MUST HAVE SEEN WHAT THE GLOB SEES, or this test compared two
	// sets and one of them was empty. Every sibling that enumerates this corpus
	// carries a vacuity guard and the repo treats it as a rule
	// (scenario_goldens_test.go: "an empty corpus must fail rather than
	// vacuously pass"; corpus_actor_kind_test.go has a dedicated test for it).
	//
	// "The walk saw every glob match" rather than "the walk saw something",
	// because it is a SUPERSET whenever the glob is non-empty: it catches the
	// walk seeing nothing AND the walk seeing only some. A bare emptiness check
	// would pass on a partial divergence.
	//
	// Not because the weaker form misses the symlink case — an earlier draft of
	// this comment claimed that and was refuted by its own example. Make
	// scenarios/ a symlink and WalkDir does not follow the root: it lstats it,
	// sees a non-directory, and fires one callback with no .json, so `seen`
	// ends up EMPTY and either form fires. Glob, meanwhile, resolves straight
	// through and matches every scenario.
	//
	// What this form does NOT cover: both sides empty. The loop then iterates
	// nothing and passes. That case belongs to the sibling above, whose
	// len(paths) == 0 guard fires on it.
	for abs := range run {
		if !seen[abs] {
			t.Errorf("%s matched %s but the walk of %s never visited it, so this test compared "+
				"nothing. A symlinked scenarios/ does exactly this: Glob resolves through it, "+
				"WalkDir does not.", abs, scenarioLibraryGlob, root)
		}
	}
}

// TestEveryMapTheCorpusNamesIsInstalledAndUsed holds scenarios/maps/ to the
// library in BOTH directions: every map id a scenario's load_map names has a
// file, and every file is named by some scenario.
//
// THE FIRST DIRECTION EXISTS BECAUSE A DENIAL CANNOT CHECK IT. Authorize runs
// before any map lookup (internal/gateway's handleCommand, then
// handleLoadMap), so a player's load_map naming a map that does not exist is
// still refused with "not authorized" — which means scenarios/denials.json's
// two refused load_map steps would go on passing, green and vacuous, if their
// map id were misspelled or the file were deleted. MEASURED 2026-09-02, not
// reasoned: misspell that id and TestScenarioLibraryRunsSelfContained/
// denials.json stays GREEN, and this gate is the only thing that reds. Nothing
// INSIDE a scenario can catch it, because every wrong id produces the identical
// refusal — which is why the check has to come from outside, by requiring the
// file.
//
// The second direction catches the leftover: rename a scene and the old map
// keeps loading and validating at every boot with nothing naming it, which is
// dead weight that reads as coverage.
//
// DERIVED, WITH NO EXEMPTION LIST, on the same reasoning
// internal/harness/corpus_actor_kind_test.go states for its own gate and
// internal/gateway's projected-fixture guard states for its: a list of files
// the rule does not apply to is the artifact that goes stale silently.
func TestEveryMapTheCorpusNamesIsInstalledAndUsed(t *testing.T) {
	paths, err := filepath.Glob(scenarioLibraryGlob)
	if err != nil {
		t.Fatalf("glob %s: %v", scenarioLibraryGlob, err)
	}
	named := map[string][]string{}
	// THE DIRECTORIES COME FROM THE SCENARIOS, not from a constant here. Every
	// scenario that loads a map declares which directory its campaign is to be
	// built from (Scenario.Maps), and today all eight name the same one — so a
	// hardcoded "../../scenarios/maps/*.json" would be exactly correct and
	// would go blind the moment a ninth scenario named a different directory:
	// its maps would be checked by neither direction below. Deriving the set
	// costs one map and removes that hole rather than documenting it, and it
	// reuses resolveMapsDir (harness_boot.go) so this gate resolves the path
	// the same way the runner that installs it does.
	declared := map[string]bool{}
	for _, p := range paths {
		sc, err := harness.LoadScenario(p)
		if err != nil {
			t.Fatalf("LoadScenario(%s): %v", p, err)
		}
		if sc.Maps != "" {
			declared[sc.Maps] = true
		}
		for i, st := range sc.Steps {
			if len(st.Command) == 0 {
				continue
			}
			var cmd vttv1.ClientCommand
			if err := protojson.Unmarshal(st.Command, &cmd); err != nil {
				t.Fatalf("%s step %d: %v", p, i, err)
			}
			if lm := cmd.GetLoadMap(); lm != nil {
				named[lm.GetMapId()] = append(named[lm.GetMapId()],
					fmt.Sprintf("%s step %d", filepath.Base(p), i))
			}
		}
	}
	// Vacuity guard, the rule this corpus's every other enumerating gate
	// carries: a library that names no map at all must fail here rather than
	// pass by having nothing to check.
	if len(named) == 0 {
		t.Fatal("no scenario issues load_map, so this gate checked nothing — the corpus's " +
			"maps are unreachable and scenarios/maps/ is unheld")
	}

	// A scenario issuing load_map while declaring no maps directory has no
	// campaign to load from, and the loop below would then compare the ids it
	// named against an EMPTY installed set — reporting every one of them as
	// missing, which is true but says the wrong thing. Say the actual thing.
	if len(declared) == 0 {
		t.Fatal("a scenario issues load_map but no scenario declares a \"maps\" directory, " +
			"so nothing is installed into any campaign and every load_map names a map that " +
			"cannot be there")
	}

	installed := map[string]string{} // map id -> the declared directory holding it
	for rel := range declared {
		dir, err := resolveMapsDir(rel)
		if err != nil {
			t.Fatalf("a scenario declares maps dir %q: %v", rel, err)
		}
		entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		for _, e := range entries {
			installed[strings.TrimSuffix(filepath.Base(e), ".json")] = rel
		}
	}

	for id, where := range named {
		if _, ok := installed[id]; !ok {
			t.Errorf("%s names map %q but no declared maps directory (%s) holds %s.json. "+
				"If that step expects ok, its scenario fails too and this gate only says so "+
				"sooner. If it expects a DENIAL, its scenario does NOT fail: Authorize runs "+
				"before any map lookup, so the step is refused with the same message whether "+
				"or not the map exists, and passes for the wrong reason. That second case is "+
				"why this gate exists.",
				strings.Join(where, ", "), id, strings.Join(sortedKeysOf(declared), ", "), id)
		}
	}
	for id, rel := range installed {
		if _, ok := named[id]; !ok {
			t.Errorf("%s/%s.json is installed into every campaign that declares that "+
				"directory, and validated at each of their boots, but no scenario names it. "+
				"Delete it, or have a scenario load it.", rel, id)
		}
	}
}

// sortedKeysOf names the declared maps directories in a stable order, so a
// failure message does not shuffle between runs.
func sortedKeysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
// Process.Kill is this test's own teardown (brief, verbatim, predates
// serve.go's SIGTERM shutdown path — see serve_e2e_test.go's
// TestServeSubprocessExitsCleanlyOnSIGTERM for that path's own coverage):
// Kill remains a valid, simpler teardown choice here since this test
// doesn't need to observe a graceful stop, just end the subprocess. The
// connection-drain carry-forward (docs/superpowers/sdd/progress.md) is
// still open regardless of which teardown a given test uses.
func TestThreeRoleExitScenarioOverLiveServeSubprocess(t *testing.T) {
	binPath := buildVTTBinary(t)

	dir := t.TempDir()
	campaignPath := filepath.Join(dir, "campaign.db")
	addr := mustFreeAddr(t)

	// This path does NOT go through bootSelfContained, so nothing has
	// installed the scenario's maps for it — and since Task 5 of the
	// 2026-09-01-create-scene-leaves plan there is no --maps-dir to point
	// `vtt serve` at one: a map belongs to the campaign that uses it. So the
	// install happens here, on the campaign directory, BEFORE the subprocess
	// starts, which is exactly the order an operator works in. The scenario
	// is read first only to learn which directory it asks for; resolveMapsDir
	// and installMaps are harness_boot.go's own, reused rather than
	// duplicated (the same reuse mintInvites gets below).
	liveSC, err := harness.LoadScenario(filepath.Join("..", "..", "scenarios", "three-role-exit.json"))
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	mapsDir, err := resolveMapsDir(liveSC.Maps)
	if err != nil {
		t.Fatalf("resolveMapsDir(%q): %v", liveSC.Maps, err)
	}
	if err := installMaps(mapsDir, campaignPath); err != nil {
		t.Fatalf("installMaps: %v", err)
	}

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
	tokens, ids, err := mintInvites(campaignPath, liveSC)
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
