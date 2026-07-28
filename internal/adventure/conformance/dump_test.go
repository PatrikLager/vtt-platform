package conformance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// DumpCompiledBatch was at 0% coverage: the helper that VERIFIES every
// hand-derived golden was itself unverified. Its own doc comment states the
// discipline it exists to serve — "derive a golden by hand FIRST (ADR-009)
// ... use this only to VERIFY the hand-derivation, never to generate a
// golden no human derived first." A silently-wrong dump would make every
// such verification vacuous: a hand-derivation would be checked against a
// misrendering, and a genuine mismatch could read as agreement.
//
// The load-bearing property is therefore that DumpCompiledBatch renders
// EXACTLY the serialization checkCompiledBatchGolden compares against. That
// is asserted below against a real committed golden, not a synthetic one.

// LOAD-BEARING FIXTURE — do not delete testdata/adventures/ok/actors/wraith.json
// without re-running the mutation audit.
//
// It is the only actor in any fixture with NEITHER attributes nor resources,
// and that is its entire purpose: `len(a.Attributes) > 0` / `len(a.Resources)
// > 0` (conformance.go:319, :325) both survived mutation to `>=` until it
// existed, because every other actor gave both maps. Removing it drops this
// package from 100% to 91.30% efficacy with both mutants LIVED again.
//
// It reads like an oversight — an actor with no stats — which is exactly why
// this note exists. Its golden entry is derivable, not observed: actors load
// in FILE-NAME order (loadActors documents it at load.go:165-169; jsonFilesIn
// does the os.ReadDir + sort.Strings at load.go:398-414),
// so hero.json precedes wraith.json; toActorAddedDump leaves both maps nil and
// both carry omitempty, so {"actor_id":"wraith","name":"Wraith"} is the only
// possible entry.

func compileFixture(t *testing.T, name string) []*vttv1.Envelope {
	t.Helper()
	dir := fixture(name)
	rs, err := rules.Load(filepath.Join(fixtureRulesetsRoot(), "fixture-ruleset"))
	if err != nil {
		t.Fatal(err)
	}
	adv, err := adventure.Load(dir, rs)
	if err != nil {
		t.Fatal(err)
	}
	envs, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		t.Fatal(err)
	}
	return envs
}

// TestDumpIsSemanticallyEqualToTheGoldenItVerifies pins the property the
// helper actually guarantees: the dump carries the same CONTENT as the
// golden checkCompiledBatchGolden accepts.
//
// It is deliberately NOT a byte comparison, and that is a finding rather
// than a convenience. checkCompiledBatchGolden decodes the golden into
// []envelopeDump and compares with reflect.DeepEqual — order-insensitive
// for maps. DumpCompiledBatch renders text via json.MarshalIndent, which
// sorts map keys alphabetically. The committed fixture golden stores
// attributes in declaration order (vim, vigor, brace), so dump and golden
// disagree TEXTUALLY while agreeing semantically, on any adventure with two
// or more attributes or resources.
//
// That matters because of what the helper is for: its doc comment says to
// hand-derive a golden first and use the dump "only to VERIFY the
// hand-derivation". An author who diffs the two sees spurious ordering
// noise; an author who pastes the dump in as the golden gets a file that
// still passes the check but has silently reordered. Whether to canonicalize
// (sort the golden, or emit declaration order) is a design decision for the
// owner, not something to change while raising coverage — so this test pins
// today's real contract and names the divergence.
func TestDumpIsSemanticallyEqualToTheGoldenItVerifies(t *testing.T) {
	envs := compileFixture(t, "ok")

	dumped, err := conformance.DumpCompiledBatch(envs)
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile(filepath.Join(fixture("ok"), "goldens", "compiled-batch.json"))
	if err != nil {
		t.Fatal(err)
	}

	var gotAny, wantAny []map[string]any
	if err := json.Unmarshal(dumped, &gotAny); err != nil {
		t.Fatalf("dump must be valid JSON: %v", err)
	}
	if err := json.Unmarshal(golden, &wantAny); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotAny, wantAny) {
		t.Errorf("dump is not semantically equal to the golden it verifies\n--- got ---\n%s\n--- want ---\n%s", dumped, golden)
	}
}

func TestDumpEndsWithNewline(t *testing.T) {
	envs := compileFixture(t, "ok")
	got, err := conformance.DumpCompiledBatch(envs)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Error("dump must end with a newline — committed goldens do, and a " +
			"missing one would make every comparison fail on the last byte")
	}
}

func TestDumpEmptyBatchIsAnEmptyArray(t *testing.T) {
	got, err := conformance.DumpCompiledBatch(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "[]\n" {
		t.Errorf("dump of an empty batch: got %q, want %q", got, "[]\n")
	}
}

// TestDumpRejectsUnknownPayload pins the failure direction: a payload kind
// outside Compile's closed six-variant union must be a named error, never a
// silently-skipped or zero-valued entry. A dropped envelope here would make
// a golden LOOK correct while describing fewer events than were compiled.
func TestDumpRejectsUnknownPayload(t *testing.T) {
	_, err := conformance.DumpCompiledBatch([]*vttv1.Envelope{{
		EventId:  "evt-1",
		Sequence: 1,
		Payload:  &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: "tok-1"}},
	}})
	if err == nil {
		t.Fatal("want error for a payload kind outside the compiled-batch union")
	}
}
