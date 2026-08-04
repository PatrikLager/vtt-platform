package conformance_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/rules"
	"github.com/PatrikLager/vtt-platform/internal/rules/conformance"
)

func fixture(elem ...string) string {
	return filepath.Join(append([]string{"testdata"}, elem...)...)
}

// TestRunValidRuleset is the full happy path: a ruleset that loads, whose
// one ability resolves against the generic fixture actor, and whose two
// golden fixtures (one hit, one out-of-range rejection) both reproduce
// exactly.
func TestRunValidRuleset(t *testing.T) {
	if err := conformance.Run(fixture("minimal")); err != nil {
		t.Fatalf("Run(minimal): unexpected error: %v", err)
	}
}

// TestRunLoadError proves Run propagates a Load failure rather than
// panicking or silently succeeding.
func TestRunLoadError(t *testing.T) {
	err := conformance.Run(fixture("minimal-load-error"))
	if err == nil {
		t.Fatal("Run(minimal-load-error): want error, got nil")
	}
	// NOT a bare `Contains(err, "id")`: "invalid" contains "id", so essentially
	// any schema complaint satisfied that, including ones that have nothing to
	// do with the missing field this fixture exists to produce.
	if !strings.Contains(err.Error(), `"id"`) {
		t.Errorf("Run(minimal-load-error) error = %q, want it to name the missing %q FIELD — a bare "+
			"substring check here passes on the word %q", err.Error(), "id", "invalid")
	}
}

// TestRunSmokeFailure proves the smoke pass — every declared ability must
// resolve against the manifest-generated fixture — actually runs and
// actually fails when an ability can never succeed against it (a
// limited-use cost that exceeds the resource's own default_max_expr).
func TestRunSmokeFailure(t *testing.T) {
	err := conformance.Run(fixture("minimal-smoke-fail"))
	if err == nil {
		t.Fatal("Run(minimal-smoke-fail): want error, got nil")
	}
	if !strings.Contains(err.Error(), "big-move") {
		t.Errorf("Run(minimal-smoke-fail) error = %q, want it to name the failing ability %q", err.Error(), "big-move")
	}
	// Naming the ability is NOT enough, and asserting only that was a hollow
	// test: this fixture produces an error mentioning "big-move" by at least
	// two different routes. If the fixture actor's resource max stops coming
	// from default_max_expr and falls back to fixtureResourceFallbackMax
	// (1000), big-move's cost of 5 becomes payable, the smoke pass SUCCEEDS,
	// and Run then fails further on with `has no golden scenario` — still
	// naming big-move, still "passing" this test, with the thing it exists to
	// prove no longer happening at all. Four surviving mutants in
	// buildResources lived behind exactly that gap.
	//
	// So pin the REASON. "have 1" is the whole point: 1 is focus's
	// default_max_expr, and no fallback or clamp can produce it.
	// "have 1" is the load-bearing half: 1 is focus's default_max_expr, and no
	// fallback (1000) or clamp (MaxInt32) can produce it. The cost is NOT
	// asserted — it comes straight off the ability manifest, no mutation here
	// can move it, and pinning it would couple this test to resolve.go's exact
	// "(have %d, need %d)" phrasing for nothing.
	for _, want := range []string{
		"failed to resolve against the generated fixture actor",
		"insufficient",
		"have 1",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Run(minimal-smoke-fail) error = %q, want it to contain %q — the smoke pass must fail "+
				"because the fixture actor's max came from default_max_expr, not for some later reason",
				err.Error(), want)
		}
	}
}

// TestRunGoldenMismatch proves golden comparison is exact, not vacuous: a
// golden fixture whose want_events deliberately disagrees with what
// Resolve actually produces must fail Run.
func TestRunGoldenMismatch(t *testing.T) {
	err := conformance.Run(fixture("minimal-golden-mismatch"))
	if err == nil {
		t.Fatal("Run(minimal-golden-mismatch): want error, got nil")
	}
	if !strings.Contains(err.Error(), "poke-hit-wrong") {
		t.Errorf("Run(minimal-golden-mismatch) error = %q, want it to name the failing golden %q", err.Error(), "poke-hit-wrong")
	}
	// The golden's NAME is in its path, and runGoldens wraps that path into
	// every failure for the file (`golden %s: %w`). So the assertion above
	// passes for a decode error too — one typo'd key, given
	// DisallowUnknownFields — while the exact want_events comparison this test
	// exists to prove never runs. Pin the comparison itself.
	if !strings.Contains(err.Error(), "event[1]") {
		t.Errorf("Run(minimal-golden-mismatch) error = %q, want it to identify the mismatching event "+
			"— naming the golden only proves the FILE was reached, not that its events were compared", err.Error())
	}
}

// TestRunOrphanCompiledGolden pins finding R1: a goldens/compiled/*.json
// file whose name matches no compiled ability (a stale pin left behind by a
// rename or deletion) must fail Run, naming the orphan — the reverse of the
// missing-golden check, closing the one load-bearing filename convention the
// suite previously validated in only one direction.
func TestRunOrphanCompiledGolden(t *testing.T) {
	err := conformance.Run(fixture("minimal-v2-orphan-compiled-golden"))
	if err == nil {
		t.Fatal("Run(minimal-v2-orphan-compiled-golden): want error for the orphan compiled golden, got nil")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("Run(minimal-v2-orphan-compiled-golden) error = %q, want it to name the orphan %q", err.Error(), "ghost")
	}
	if !strings.Contains(err.Error(), "orphan") {
		t.Errorf("Run(minimal-v2-orphan-compiled-golden) error = %q, want it to call out the orphan", err.Error())
	}
}

func TestRunUnknownDirectory(t *testing.T) {
	if err := conformance.Run(fixture("does-not-exist")); err == nil {
		t.Fatal("Run(does-not-exist): want error, got nil")
	}
}

// TestRunValidV2Ruleset is TestRunValidRuleset's format-v2 counterpart
// (Task 3): a single composed ability ("poke", minimal-v2/) whose batch
// golden (goldens/poke-hit.json) AND compiled-form golden
// (goldens/compiled/poke.json) both reproduce exactly — proving the smoke
// pass, the batch-golden pass, and the NEW compiled-form-golden pass
// (spec §8) all actually run for a v2 ruleset, not just a v1 one (a v2
// ruleset's Abilities map is deliberately empty — Task 2's design
// decision — so before Task 3 rewired smokeTest/runGoldens to iterate
// rs.Compiled instead, both passes silently covered ZERO abilities for
// any v2 ruleset; this test would have passed vacuously against that gap).
func TestRunValidV2Ruleset(t *testing.T) {
	if err := conformance.Run(fixture("minimal-v2")); err != nil {
		t.Fatalf("Run(minimal-v2): unexpected error: %v", err)
	}
}

// TestRunCompiledGoldenMissing proves the compiled-form golden pass is
// actually enforced for a v2 ruleset (spec §8, Task 3): a ruleset
// otherwise identical to minimal-v2 but shipping no goldens/compiled/
// directory at all must fail Run, naming the ability with no compiled
// golden.
func TestRunCompiledGoldenMissing(t *testing.T) {
	err := conformance.Run(fixture("minimal-v2-missing-compiled-golden"))
	if err == nil {
		t.Fatal("Run(minimal-v2-missing-compiled-golden): want error, got nil")
	}
	if !strings.Contains(err.Error(), "poke") {
		t.Errorf("Run(minimal-v2-missing-compiled-golden) error = %q, want it to name the ability %q", err.Error(), "poke")
	}
	if !strings.Contains(err.Error(), "missing compiled golden") {
		t.Errorf("Run(minimal-v2-missing-compiled-golden) error = %q, want it to say the golden is missing", err.Error())
	}
}

// TestRunCompiledGoldenMismatch proves compiled-golden comparison is exact,
// not vacuous: a goldens/compiled/poke.json that deliberately disagrees
// with what the ruleset actually compiles to (targeting.range: 99 instead
// of 1 — a drifted/hand-edited golden) must fail Run, naming the ability
// and the drift.
func TestRunCompiledGoldenMismatch(t *testing.T) {
	err := conformance.Run(fixture("minimal-v2-compiled-golden-mismatch"))
	if err == nil {
		t.Fatal("Run(minimal-v2-compiled-golden-mismatch): want error, got nil")
	}
	if !strings.Contains(err.Error(), "poke") {
		t.Errorf("Run(minimal-v2-compiled-golden-mismatch) error = %q, want it to name the ability %q", err.Error(), "poke")
	}
	if !strings.Contains(err.Error(), "drift") {
		t.Errorf("Run(minimal-v2-compiled-golden-mismatch) error = %q, want it to call out the drift", err.Error())
	}
	// Naming the ability and saying "drift" does not distinguish the drift
	// report from its FALLBACK. runCompiledGoldens re-dumps the compiled power
	// to show got-vs-want, and if that dump fails it returns a different error
	// that also names the ability and also says "drift" — so asserting only
	// those two passes while the diagnostic half is gone. The whole value of
	// this error to whoever hits it is the payload.
	for _, want := range []string{"got:", "want:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Run(minimal-v2-compiled-golden-mismatch) error = %q, want it to carry %q — a drift "+
				"report without the got/want dump tells the reader nothing they can act on", err.Error(), want)
		}
	}
}

// TestRunZeroDefaultMaxFallsBackRatherThanBuildingAnUnusableActor pins the
// `v > 0` boundary in buildResources. A default_max_expr that evaluates to 0 is
// not a usable maximum for a generic stand-in actor — every limited ability
// would be unaffordable and the smoke pass would report failures that say
// nothing about the ruleset. So 0 takes the fallback, exactly as a missing
// expression does.
//
// The fixture makes that observable: `pool` declares default_max_expr "0" and
// `poke` costs 1 of it. At `v >= 0` the actor is built with max 0, cannot pay,
// and Run fails naming poke.
func TestRunZeroDefaultMaxFallsBackRatherThanBuildingAnUnusableActor(t *testing.T) {
	if err := conformance.Run(fixture("minimal-v2-zero-default-max")); err != nil {
		t.Fatalf("Run(minimal-v2-zero-default-max): unexpected error: %v\n"+
			"a default_max_expr of 0 must fall back to fixtureResourceFallbackMax; building the "+
			"smoke actor with max 0 makes every limited ability unaffordable", err)
	}
}

// TestRunGoldenWithNoRecordedRollsDegradesRatherThanPanicking pins
// tableRoller.Roll's exhaustion guard at the exact-exhaustion boundary. A
// golden that records fewer steps than Resolve evaluates is a broken FIXTURE,
// and the contract is that it surfaces as an ordinary conformance failure
// naming the golden — never as a panic out of the roller, which would blame the
// platform for an authoring mistake and take the whole run down with it.
//
// The fixture records no rolls at all, so the first Roll call hits i == len(0).
// At `r.i > len(r.steps)` that case slips past the guard and indexes an empty
// slice. Run succeeding here is what separates >= from >.
func TestRunGoldenWithNoRecordedRollsDegradesRatherThanPanicking(t *testing.T) {
	// A panic in the roller would fail this test by crashing the process, which
	// is the point: the assertion is that we reach a verdict at all.
	if err := conformance.Run(fixture("minimal-v2-rolls-exhausted")); err != nil {
		t.Fatalf("Run(minimal-v2-rolls-exhausted): unexpected error: %v\n"+
			"an exhausted roll table must yield a zero roll, not an out-of-range index", err)
	}
}

// TestDumpCompiledPowerSucceedsForARealCompiledPower pins the success path of
// the golden-authoring helper. `goldens/compiled/<id>.json` files are produced
// BY this function, so if it ever returns an error for an ordinary compiled
// power, the documented authoring workflow ("run this, write the result
// verbatim") stops working — and nothing else in the suite calls it outside
// the drift-diagnostic path, where its error return is swallowed into a
// message about diagnostics rather than surfaced.
func TestDumpCompiledPowerSucceedsForARealCompiledPower(t *testing.T) {
	rs, err := rules.Load(fixture("minimal-v2"))
	if err != nil {
		t.Fatalf("Load(minimal-v2): %v", err)
	}
	cp := rs.Compiled["poke"]
	if cp == nil {
		t.Fatal(`Load(minimal-v2): no compiled power for "poke"; the fixture changed shape`)
	}

	b, err := conformance.DumpCompiledPower(cp)
	if err != nil {
		t.Fatalf("DumpCompiledPower(poke): unexpected error: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("DumpCompiledPower(poke): empty output")
	}
	if b[len(b)-1] != '\n' {
		t.Error("DumpCompiledPower(poke): output must end in a newline — it is written verbatim to a file")
	}
	// Round-trips as JSON, which is what makes "write the result verbatim"
	// safe: runCompiledGoldens decodes these files back.
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("DumpCompiledPower(poke): output is not valid JSON: %v", err)
	}
	if back["id"] != "poke" {
		t.Errorf(`DumpCompiledPower(poke): dumped id = %v, want "poke"`, back["id"])
	}
}

// TestRunCompiledGoldenExemptForV1 (REMOVED, Task 4): proved the exemption
// half of spec §8's rule ("v1 rulesets exempt until none exist" — the
// task brief's own words) by pointing conformance.Run at a v1-shaped
// "minimal" fixture that shipped no goldens/compiled/ directory. Task 4's
// sunset retires format_version "1" entirely — Load rejects it before
// reading a single file, so no ruleset directory can be "v1-shaped and
// loadable" anymore, and this test's premise is unreproducible by
// construction, not merely stale. "minimal" itself migrated to v2 (Task
// 4 report) and now ships goldens/compiled/poke.json like every other v2
// fixture; runCompiledGoldens's former `if rs.FormatVersion != "2" { return
// nil }` exemption clause was consequently dead code (Load never returns a
// Ruleset with any other FormatVersion) and has been removed in the fix
// wave.

// TestRunMissingPerAbilityGolden pins F7: spec §8 mandates a golden scenario
// per ability, so a ruleset that loads and smoke-passes but ships no golden
// for one of its abilities must fail Run, naming the uncovered ability —
// otherwise the P4 forever-gate silently loses its pins to a rename, a typo,
// or a deletion, and 5b's dnd45e-minimal could satisfy "the SAME suite" with
// no golden coverage at all.
func TestRunMissingPerAbilityGolden(t *testing.T) {
	err := conformance.Run(fixture("minimal-missing-golden"))
	if err == nil {
		t.Fatal("Run(minimal-missing-golden): want error for the ability with no golden, got nil")
	}
	if !strings.Contains(err.Error(), "prod") {
		t.Errorf("Run(minimal-missing-golden) error = %q, want it to name the uncovered ability %q", err.Error(), "prod")
	}
	// Naming the ability is not enough: make prod's cost exceed its resource
	// and Run fails in the SMOKE pass instead, also naming prod, leaving F7's
	// per-ability-golden rule never reached and this test still green.
	if !strings.Contains(err.Error(), "has no golden scenario") {
		t.Errorf("Run(minimal-missing-golden) error = %q, want it to fail for the MISSING GOLDEN "+
			"specifically — any other failure mentioning %q would satisfy a bare name check", err.Error(), "prod")
	}
}

// TestConformanceOverRulesetsGlob is the "forever" gate the task brief
// calls for: every ruleset committed under rulesets/ — today just
// tavern-brawl, tomorrow also 5b's dnd45e-minimal — must pass Run
// untouched. Wired into `go test ./...` (and so `task check`) by simply
// existing in this suite; no separate Taskfile target needed.
func TestConformanceOverRulesetsGlob(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("..", "..", "..", "rulesets", "*"))
	if err != nil {
		t.Fatalf("glob rulesets/*: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("glob rulesets/*: matched zero directories — expected at least tavern-brawl")
	}
	for _, dir := range dirs {
		dir := dir
		t.Run(filepath.Base(dir), func(t *testing.T) {
			if err := conformance.Run(dir); err != nil {
				t.Errorf("Run(%s): %v", dir, err)
			}
		})
	}
}
