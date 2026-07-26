package adventure

import "testing"

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
