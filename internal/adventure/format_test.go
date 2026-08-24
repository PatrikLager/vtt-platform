package adventure

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestSizeCapsMirrorEngine pins this package's local size-cap constants
// (load.go) against the literal values internal/engine/apply.go's
// maxNoteKeyBytes/maxNoteTitleBytes/maxTextBytes currently declare. engine
// does not export those constants, so no compiler-enforced link between the
// two packages is possible — this test is an internal (package adventure,
// not adventure_test) test specifically so it can see these unexported
// consts, and it pins them against hand-verified literals with this
// comment naming the engine source of truth: if engine's caps ever change,
// this test will keep passing (it only checks self-consistency) and a
// human must re-sync both sides by hand — the same caveat the brief that
// commissioned this mirroring named explicitly.
func TestSizeCapsMirrorEngine(t *testing.T) {
	// internal/engine/apply.go:
	//   maxNoteKeyBytes   = 128
	//   maxNoteTitleBytes = 256
	//   maxTextBytes      = 8192 // shared by note text and NarrationAdded.text
	if maxNoteKeyBytes != 128 {
		t.Errorf("maxNoteKeyBytes = %d, want 128 (engine's maxNoteKeyBytes)", maxNoteKeyBytes)
	}
	if maxNoteTitleBytes != 256 {
		t.Errorf("maxNoteTitleBytes = %d, want 256 (engine's maxNoteTitleBytes)", maxNoteTitleBytes)
	}
	if maxTextBytes != 8192 {
		t.Errorf("maxTextBytes = %d, want 8192 (engine's maxTextBytes)", maxTextBytes)
	}
}

// TestActorKindNamesAreExactlyTheVocabulary closes the one hole this file's
// own subject warns about: actorKindNames (load.go) restates actorKinds' key
// set by hand, so it is a second place holding the same fact, and the whole
// point of this task is that a fact stated twice can disagree.
//
// It is not derived from the map because the ORDER is load-bearing — a map
// range would offer the two answers in a different order each run, which is a
// diff in every failure output. So the order is written down and this is what
// keeps the CONTENT honest: add a third kind to actorKinds and forget the
// list, and the refusal messages would go on offering two.
func TestActorKindNamesAreExactlyTheVocabulary(t *testing.T) {
	if len(actorKindNames) != len(actorKinds) {
		t.Fatalf("actorKindNames = %v, actorKinds has %d entries", actorKindNames, len(actorKinds))
	}
	seen := map[string]bool{}
	for _, n := range actorKindNames {
		if _, ok := actorKinds[n]; !ok {
			t.Errorf("actorKindNames offers %q, which the loader would then refuse", n)
		}
		if seen[n] {
			t.Errorf("actorKindNames lists %q twice", n)
		}
		seen[n] = true
	}
	// UNSPECIFIED must have no spelling: an author who could write the silence
	// down would be writing exactly what this field exists to abolish.
	for name, k := range actorKinds {
		if k == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
			t.Errorf("actorKinds maps %q to UNSPECIFIED, which gives absence a name", name)
		}
	}
}
