package engine_test

import (
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// At-cap accept tests: every size cap in Apply is written `> max`, so a value
// of EXACTLY max must be accepted. TestApplyRejections covers the exceeding
// side of each cap; this file covers the accepting side, pinning each
// comparison at `>` rather than `>=`.
//
// PROVENANCE (do not delete these without re-running the mutation audit):
// gremlins found five surviving CONDITIONALS_BOUNDARY mutants in apply.go
// (:174:40, :189:24, :202:38, :205:20, :208:40) — each flipping a cap from
// `>` to `>=` with no test noticing. The gap was ledgered as a carry-forward
// at the world-layer merge gate ("multibyte/at-cap boundary tests") and went
// unpaid; separately, TestNarrationAddedAsAtCapIsAccepted's doc comment
// asserted the note key/title/text caps "already follow implicitly via their
// own accept tests" — the surviving mutants show that claim was untrue.
//
// The two remaining survivors in ResourceChanged (:141:15 `computed < 0` and
// :144:30 `computed > int64(res.Max)`) are EQUIVALENT, not gaps: flipping
// either to its inclusive form assigns the value it already holds. They are
// unkillable by construction and must not be "fixed" with a test.

func TestNarrationTextAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(10, &vttv1.NarrationAdded{
		Text: strings.Repeat("a", 8192),
	})))

	if !reflect.DeepEqual(before, st.Snapshot()) {
		t.Fatal("NarrationAdded must not mutate state")
	}
}

// TestNarrationSingleEventAnchorIsAccepted pins from == to as valid. This is
// not an exotic edge: it is the most ordinary anchoring case there is —
// narration describing exactly one preceding event. The `>` in
// `AnchorFromSeq > AnchorToSeq` is what permits it.
func TestNarrationSingleEventAnchorIsAccepted(t *testing.T) {
	st := seedScene(t)
	before := st.Snapshot()

	must(t, engine.Apply(st, env(10, &vttv1.NarrationAdded{
		Text: "The cutter's blade turns on the shield boss.",
		AnchorFromSeq: 5, AnchorToSeq: 5,
	})))

	if !reflect.DeepEqual(before, st.Snapshot()) {
		t.Fatal("NarrationAdded must not mutate state")
	}
}

func TestNoteKeyAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	key := strings.Repeat("k", 128)

	must(t, engine.Apply(st, env(10, &vttv1.NoteUpserted{
		Key: key, Title: "at cap", Text: "body",
	})))

	if _, ok := st.Notes[key]; !ok {
		t.Fatalf("note with a 128-byte key must be stored")
	}
}

func TestNoteTitleAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	title := strings.Repeat("t", 256)

	must(t, engine.Apply(st, env(10, &vttv1.NoteUpserted{
		Key: "k", Title: title, Text: "body",
	})))

	if got := st.Notes["k"].Title; got != title {
		t.Fatalf("title: got %d bytes, want %d", len(got), len(title))
	}
}

func TestNoteTextAtCapIsAccepted(t *testing.T) {
	st := seedScene(t)
	text := strings.Repeat("x", 8192)

	must(t, engine.Apply(st, env(10, &vttv1.NoteUpserted{
		Key: "k", Title: "at cap", Text: text,
	})))

	if got := st.Notes["k"].Text; got != text {
		t.Fatalf("text: got %d bytes, want %d", len(got), len(text))
	}
}
