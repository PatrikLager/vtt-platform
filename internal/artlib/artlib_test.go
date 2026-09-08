package artlib_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/PatrikLager/vtt-platform/internal/artlib"
)

// The fixtures below name every file LITERALLY. An earlier helper took an id
// and always wrote "<id>.png" beside the sidecar, which is why round 1 shipped
// a door with a third picture no design ever asked for: every door fixture had
// a cellar-door.png sitting next to cellar-door-open.png, so the assertion
// could not tell the two apart. Spec §3.1's own listing has no such file.

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// writePicture drops image-shaped bytes at exactly name. Nothing in artlib
// reads them; what matters is that the entry exists.
func writePicture(t *testing.T, dir, name string) string {
	t.Helper()
	return writeFile(t, dir, name, "fake-png")
}

// writeTileArt is the ordinary tile case: a sidecar and the one picture its
// stem names. Doors are built by hand, file by file, because a door's pictures
// are NOT its stem.
func writeTileArt(t *testing.T, dir, id, sidecarJSON string) {
	t.Helper()
	writeFile(t, dir, id+".json", sidecarJSON)
	writePicture(t, dir, id+".png")
}

// specArtDir builds design spec §3.1's canonical listing, byte for byte:
// masonry-1, earth-1 and pillar-stone, plus a cellar-door whose two pictures
// are named by its sidecar and which has NO cellar-door.png.
func specArtDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTileArt(t, dir, "masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`)
	writeTileArt(t, dir, "earth-1", `{"format_version":1,"kind":"floor","material":"earth"}`)
	writeFile(t, dir, "cellar-door.json", `{"format_version":1,"kind":"door","material":"wood",
		"closed":"cellar-door-closed.png","open":"cellar-door-open.png"}`)
	writePicture(t, dir, "cellar-door-open.png")
	writePicture(t, dir, "cellar-door-closed.png")
	writePicture(t, dir, "pillar-stone.png")
	return dir
}

func TestObjectArtNeedsNoSidecar(t *testing.T) {
	dir := t.TempDir()
	writePicture(t, dir, "pillar-stone.png")
	p, err := artlib.Lookup(dir, "pillar-stone")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.ID != "pillar-stone" || p.File != "pillar-stone.png" || p.Kind != "" {
		t.Fatalf("got %+v, want object art whose file is its own name", p)
	}
	if p.HasSidecar {
		t.Fatalf("got %+v, want HasSidecar false — exit criterion 6 (tile art without a "+
			"sidecar degrades with its own warning) needs this bit, and Kind == \"\" "+
			"cannot carry it: a sidecar declaring no kind produces the same empty "+
			"string", p)
	}
}

func TestTileArtDeclaresItsNature(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`)
	p, err := artlib.Lookup(dir, "masonry-1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Kind != "wall" || p.Material != "stone" {
		t.Fatalf("got %+v, want kind=wall material=stone", p)
	}
	if !p.HasSidecar {
		t.Fatalf("got %+v, want HasSidecar true", p)
	}
	// The id and the file are the same string on this path too, and both are
	// load-bearing downstream: the id is what a map wrote, the file is what
	// GET /api/art/{file} will be asked for (spec §6).
	if p.ID != "masonry-1" || p.File != "masonry-1.png" {
		t.Fatalf("got %+v, want ID and File derived from the filename", p)
	}
}

// TestADoorHasTwoPicturesAndNoThird pins spec §3.1's own listing, which has
// cellar-door.json, cellar-door-open.png and cellar-door-closed.png and NO
// cellar-door.png. A door's pictures are the two its sidecar names; a third
// file named after the stem is not part of the design, so Lookup must not
// invent one and Validate must not demand one.
func TestADoorHasTwoPicturesAndNoThird(t *testing.T) {
	dir := specArtDir(t)
	p, err := artlib.Lookup(dir, "cellar-door")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if p.Open != "cellar-door-open.png" || p.Closed != "cellar-door-closed.png" {
		t.Fatalf("got %+v, want both door pictures", p)
	}
	if p.File != "" {
		t.Fatalf("got File=%q, want empty: a door has two pictures and no third, and "+
			"%q does not exist in spec §3.1's own art directory", p.File, p.File)
	}
}

// TestValidateAcceptsTheDirectoryTheSpecDraws is the other half: the exact
// listing spec §3.1 prints must pass validation, or Task 8's migration
// produces a campaign that cannot boot and a `vtt art install` that refuses
// its own output.
func TestValidateAcceptsTheDirectoryTheSpecDraws(t *testing.T) {
	dir := specArtDir(t)
	if err := artlib.Validate(dir); err != nil {
		t.Fatalf("Validate on spec §3.1's own listing: %v", err)
	}
	for _, id := range []string{"masonry-1", "earth-1", "cellar-door", "pillar-stone"} {
		if _, err := artlib.Lookup(dir, id); err != nil {
			t.Errorf("Lookup(%q): %v", id, err)
		}
	}
}

func TestMissingArtIsErrNotFoundSoCallersCanDegrade(t *testing.T) {
	dir := t.TempDir()
	_, err := artlib.Lookup(dir, "nothing-here")
	if !errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound — Resolve distinguishes absent art (degrade) "+
			"from malformed art (refuse) by this sentinel", err)
	}
}

// TestACampaignWithNoArtDirectoryDegrades is the same claim one level up. A
// campaign that has installed no art at all is ordinary, and its maps must
// still load and draw from the built-in vocabulary (spec §4).
func TestACampaignWithNoArtDirectoryDegrades(t *testing.T) {
	_, err := artlib.Lookup(filepath.Join(t.TempDir(), "art"), "masonry-1")
	if !errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound: no art/ at all is absent art, not a broken "+
			"campaign — refusing here would reproduce sub-project 15's boot-order defect", err)
	}
}

// TestAnArtPathThatIsNotADirectoryIsRefused: a path that EXISTS and is not a
// directory is a different fact from "no art/ yet", and neither entry point
// may wave it through.
func TestAnArtPathThatIsNotADirectoryIsRefused(t *testing.T) {
	notADir := writeFile(t, t.TempDir(), "art", "nope")
	if err := artlib.Validate(notADir); err == nil {
		t.Errorf("Validate(%q): want an error, got nil", notADir)
	}
	_, err := artlib.Lookup(notADir, "masonry-1")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Errorf("Lookup: got %v, want a refusal that is NOT ErrNotFound", err)
	}
}

// TestASidecarWhosePictureIsNotInstalledDoesNotResolve is spec §5's backstop
// at the entry point that cannot be bypassed. A sidecar copied in by hand with
// no picture beside it used to resolve cleanly and hand the renderer a
// filename that 404s; the square must degrade instead, and be named in the
// load warning §4 designs.
func TestASidecarWhosePictureIsNotInstalledDoesNotResolve(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "masonry-1.json", `{"format_version":1,"kind":"wall","material":"stone"}`)
	_, err := artlib.Lookup(dir, "masonry-1")
	if !errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound: half-installed art is art that is not there", err)
	}
}

func TestADoorWhosePicturesAreNotInstalledDoesNotResolve(t *testing.T) {
	for _, tc := range []struct{ name, present string }{
		{"neither picture", ""},
		{"only the open one", "cellar-door-open.png"},
		{"only the closed one", "cellar-door-closed.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, "cellar-door.json", `{"format_version":1,"kind":"door",
				"material":"wood","open":"cellar-door-open.png","closed":"cellar-door-closed.png"}`)
			if tc.present != "" {
				writePicture(t, dir, tc.present)
			}
			_, err := artlib.Lookup(dir, "cellar-door")
			if !errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("got %v, want ErrNotFound: a door whose picture is not installed "+
					"is art that is not there", err)
			}
		})
	}
}

// TestValidateRefusesASidecarWithNoPicture keeps spec §3.4's "a sidecar with
// no matching picture is an error" where it can name the file: at install and
// at boot. The two good pieces sort BEFORE the orphan, so a Validate that
// looked only at the first entry would pass.
func TestValidateRefusesASidecarWithNoPicture(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "aaa-first", `{"format_version":1,"kind":"wall","material":"stone"}`)
	writePicture(t, dir, "bbb-second.png")
	writeFile(t, dir, "orphan.json", `{"format_version":1,"kind":"wall","material":"stone"}`)
	err := artlib.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("got %v, want a refusal naming orphan", err)
	}
}

// TestValidateRefusesADoorWhosePicturesAreMissing is the inversion of the
// round-1 defect: the door names two pictures that are not there, and a
// cellar-door.png that no design mentions IS there. Checking the stem rather
// than the names both accepted this and refused spec §3.1's own directory.
func TestValidateRefusesADoorWhosePicturesAreMissing(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "cellar-door", `{"format_version":1,"kind":"door","material":"wood",
		"open":"cellar-door-open.png","closed":"cellar-door-closed.png"}`)
	err := artlib.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "cellar-door-open.png") {
		t.Fatalf("got %v, want a refusal naming the picture the sidecar declares", err)
	}
}

// TestASubdirectoryIsRefusedByName: the subdirectory sorts LAST here, after
// two well-formed pieces, so the refusal proves Validate walks the whole
// directory rather than inspecting whatever ReadDir returned first.
func TestASubdirectoryIsRefusedByName(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "aaa-first", `{"format_version":1,"kind":"wall","material":"stone"}`)
	writePicture(t, dir, "bbb-second.png")
	if err := os.Mkdir(filepath.Join(dir, "cellar-basics"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := artlib.Validate(dir)
	if err == nil || !strings.Contains(err.Error(), "cellar-basics") {
		t.Fatalf("got %v, want a refusal naming cellar-basics — a subfolder is a "+
			"namespace and a namespace is a pack (spec §3.3)", err)
	}
}

// TestASymlinkInTheArtDirectoryIsRefusedByName closes the flatness rule's
// back door. fs.DirEntry.IsDir() reports the type from the directory entry,
// which for a symlink is ModeSymlink — so a symlink TO a directory answers
// IsDir() == false and walks straight past a subdirectory check. `ln -s`
// re-creates the pack the design deletes, and a symlink to a file re-creates
// the second name for one piece that §3.3 says cannot exist.
func TestASymlinkInTheArtDirectoryIsRefusedByName(t *testing.T) {
	for _, tc := range []struct{ name, target string }{
		{"a directory", "pack-dir"},
		{"a file", "masonry-1.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "art")
			writeTileArt(t, dir, "masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`)
			if err := os.Mkdir(filepath.Join(root, "pack-dir"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, tc.target), filepath.Join(dir, "sneaky")); err != nil {
				t.Fatal(err)
			}
			err := artlib.Validate(dir)
			if err == nil || !strings.Contains(err.Error(), "sneaky") {
				t.Fatalf("got %v, want a refusal naming sneaky", err)
			}
		})
	}
}

// TestValidateReportsEveryProblemNotOnlyTheFirst is Patrik's boot-severity
// ruling of 2026-09-03 seen from inside this package. composeServer runs this
// walk at start, reports what it finds, and STARTS THE SERVER ANYWAY — so an
// early return costs a DM one boot per mistake, and a campaign with two
// hand-copied files fixes one, restarts, and learns about the other. Every
// problem in one pass, or the report is a guessing game.
//
// EVERY ARM OF THE WALK IS PLANTED, AND IN ReadDir ORDER, which is the whole
// shape of the test. os.ReadDir sorts by name, so the entries arrive aaa-good
// (fine), bbb-Bad.png, ccc-link.png, ddd-subdir, eee-orphan.json, fff-orphan.json —
// a filename no map could spell (spec §3.2), a symlink (§3.3), a subdirectory
// (§3.3) and two sidecars with no picture (§3.4, found through Lookup). Four
// different arms, because collecting is a property of the LOOP and a version
// that collected in one arm and returned from another would satisfy a
// single-arm test.
//
// THE PREFIXES ARE LOAD-BEARING, not decoration, and so is the SECOND orphan.
// Every arm is followed by at least one more problem, so a Validate that
// answered with its FIRST finding loses a name this test asks for — measured by
// injecting exactly that into the collecting loop, where it costs four of the
// five (Task 4 report). A fixture whose only bad filename sorted LAST would have
// left that arm invisible, which is the degenerate-fixture shape this repo has
// been bitten by before; a single orphan would have done the same to the Lookup
// arm.
func TestValidateReportsEveryProblemNotOnlyTheFirst(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "aaa-good", `{"format_version":1,"kind":"wall","material":"stone"}`)
	writePicture(t, dir, "bbb-Bad.png")
	// RELATIVE, AND NAMED LIKE A PICTURE — `ln -s aaa-good.png ccc-link.png`,
	// which is what a DM types, and the one symlink shape that RESOLVES at the
	// table: measured 2026-09-03, Lookup(dir, "ccc-link") returns a Piece and
	// the square renders. An ABSOLUTE target is refused by os.Root at load time
	// ("path escapes from parent"), so a fixture built that way leaves the shape
	// a person actually creates untested (review finding F2). Validate's arm
	// reads e.Type() and so refuses both identically — that is the point: this
	// report is the ONLY thing that will ever mention the relative one.
	if err := os.Symlink("aaa-good.png", filepath.Join(dir, "ccc-link.png")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "ddd-subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "eee-orphan.json", `{"format_version":1,"kind":"wall","material":"stone"}`)
	writeFile(t, dir, "fff-orphan.json", `{"format_version":1,"kind":"wall","material":"stone"}`)

	err := artlib.Validate(dir)
	if err == nil {
		t.Fatal("Validate: nil, want every one of the five problems planted here")
	}
	for _, want := range []string{"bbb-Bad.png", "ccc-link.png", "ddd-subdir", "eee-orphan", "fff-orphan"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate = %v\n  want it to also name %q: an operator who is told about "+
				"one problem per boot pays one boot per problem", err, want)
		}
	}
	// The good piece is not reported as a problem — without this, "report
	// everything" is satisfied by a walk that complains about every entry.
	if strings.Contains(err.Error(), "aaa-good") {
		t.Errorf("Validate = %v\n  aaa-good is a well-formed piece and must not be named", err)
	}
}

func TestValidateAcceptsAnAbsentArtDirectory(t *testing.T) {
	if err := artlib.Validate(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("Validate on an absent art/: %v — a campaign with no art yet is "+
			"ordinary, and refusing it would reproduce the boot-order defect this "+
			"design removes", err)
	}
}

// TestLookupRefusesAnIdThatIsNotAFilename plants a LOADABLE piece of art at
// every escape target and proves it resolves at its real path first, so the
// refusal below can only be the id check and not an unreadable file. Round 1
// asserted err != nil against an empty temp dir, where every escaped path was
// missing anyway: `idIsAFilename` could be replaced with `return true` and the
// whole suite stayed green.
func TestLookupRefusesAnIdThatIsNotAFilename(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "art")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The escape targets, each real art that Lookup accepts where it lives.
	writeTileArt(t, root, "secret", `{"format_version":1,"kind":"wall","material":"SECRET-OUTSIDE"}`)
	writeTileArt(t, filepath.Join(dir, "sub"), "nested", `{"format_version":1,"kind":"wall","material":"stone"}`)
	for _, at := range []struct{ dir, id string }{{root, "secret"}, {filepath.Join(dir, "sub"), "nested"}} {
		if _, err := artlib.Lookup(at.dir, at.id); err != nil {
			t.Fatalf("test setup bug: %q must be loadable where it lives, got %v", at.id, err)
		}
	}

	for _, id := range []string{
		"../secret", `..\secret`, "sub/nested", `sub\nested`, "", ".", "..", "x/../../secret",
	} {
		p, err := artlib.Lookup(dir, id)
		if err == nil {
			t.Fatalf("Lookup(%q) was accepted and returned %+v; an id becomes a path and "+
				"must be one plain filename in art/", id, p)
		}
		if strings.Contains(err.Error(), "SECRET-OUTSIDE") || p.Material == "SECRET-OUTSIDE" {
			t.Fatalf("Lookup(%q) reached outside art/: %+v %v", id, p, err)
		}
		if !strings.Contains(err.Error(), "kebab-case") {
			t.Errorf("Lookup(%q): error = %q, want the id guard's own message — an error "+
				"that merely says the file is missing cannot tell the guard from a typo", id, err)
		}
	}
}

// TestAnIdThatCannotNameAFileNamesNoArt: an id longer than a filename may be,
// or carrying a NUL, cannot possibly be installed. That is absence, and spec
// §4 wants absence to degrade — round 1 returned the raw ENAMETOOLONG/EINVAL,
// which is not ErrNotFound, so one over-long name in a map refused the whole
// map.
func TestAnIdThatCannotNameAFileNamesNoArt(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{strings.Repeat("a", 300), "mason\x00ry"} {
		if _, err := artlib.Lookup(dir, id); !errors.Is(err, artlib.ErrNotFound) {
			t.Errorf("Lookup(%q) = %v, want ErrNotFound: an id that cannot name a file in "+
				"art/ names no art, and a typo is a warning (spec §4), not a refused map", id, err)
		}
	}
}

// TestAnArtIdIsKebabCaseOnEveryFilesystem is the cross-platform rule. macOS is
// case-insensitive, so Lookup(dir, "MASONRY-1") used to open masonry-1.json
// and return a Piece whose ID and File echoed the REQUEST rather than the
// filename on disk — "nothing can disagree with the filename" (spec §3.2) was
// false here, and the campaign broke on a case-sensitive Linux server. Spec
// §3.2's "stems are kebab-case" is enforced rather than assumed, so the id
// space has no case to disagree about on any filesystem.
func TestAnArtIdIsKebabCaseOnEveryFilesystem(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`)
	for _, id := range []string{
		"MASONRY-1", "Masonry-1", "masonry_1", "masonry.1", "masonry 1",
		"-masonry", "masonry-", "masonry--1",
	} {
		p, err := artlib.Lookup(dir, id)
		if !errors.Is(err, artlib.ErrNotFound) {
			t.Errorf("Lookup(%q) = (%+v, %v), want ErrNotFound: an art id is kebab-case, "+
				"so an id that differs from the filename resolves nowhere — on a "+
				"case-insensitive filesystem too", id, p, err)
			continue
		}
		// The same discriminator TestLookupRefusesAnIdThatIsNotAFilename
		// carries, and for the same reason: ErrNotFound alone cannot tell the
		// GUARD from a file that merely is not there, so without this every id
		// above would pass against a guard that had stopped checking. The
		// trailing-hyphen clause survived deletion for exactly that gap.
		if !strings.Contains(err.Error(), "kebab-case") {
			t.Errorf("Lookup(%q): error = %q, want the id guard's own message", id, err)
		}
	}
}

// TestTheLongestArtIdSitsExactlyOnTheFilenameLimit plants art whose stem
// is the longest a filename can carry — 250 bytes, because ".json" spends five
// of the 255 a directory entry has — and proves it resolves. A fixture one
// byte under the limit would resolve either way and pin nothing: the bound
// would be free to be off by one, and an id at exactly the limit would then be
// refused as absent for no reason a DM could see.
func TestTheLongestArtIdSitsExactlyOnTheFilenameLimit(t *testing.T) {
	const longest = 255 - len(".json")
	dir := t.TempDir()
	id := strings.Repeat("a", longest)
	writeTileArt(t, dir, id, `{"format_version":1,"kind":"wall","material":"stone"}`)
	p, err := artlib.Lookup(dir, id)
	if err != nil {
		t.Fatalf("Lookup on a %d-byte id: %v — this is a filename the filesystem "+
			"accepts, so artlib must too", len(id), err)
	}
	if p.ID != id {
		t.Fatalf("got ID %q, want the id asked for", p.ID)
	}
	if _, err := artlib.Lookup(dir, id+"a"); !errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("Lookup on a %d-byte id = %v, want ErrNotFound: one byte past the "+
			"longest filename is a name no art can have", len(id)+1, err)
	}
	if err := artlib.Validate(dir); err != nil {
		t.Fatalf("Validate: %v, want nil", err)
	}
}

// TestAnArtIdMaySpanTheWholeKebabAlphabet sits on the other four boundaries of
// the same guard at once: 'a' and 'z' at the ends of the letter range, '0' and
// '9' at the ends of the digit range, and hyphens that are neither leading,
// trailing nor doubled. Tighten any one comparison by a character and this id
// stops resolving.
func TestAnArtIdMaySpanTheWholeKebabAlphabet(t *testing.T) {
	dir := t.TempDir()
	const id = "a-z0-9"
	writeTileArt(t, dir, id, `{"format_version":1,"kind":"wall","material":"stone"}`)
	if _, err := artlib.Lookup(dir, id); err != nil {
		t.Fatalf("Lookup(%q): %v — every character here is legal kebab-case", id, err)
	}
	if err := artlib.Validate(dir); err != nil {
		t.Fatalf("Validate: %v, want nil", err)
	}
}

// TestValidateRefusesAFilenameThatIsNotAnArtName is the same rule's other
// half, on disk. A stem no map can name is art nobody can reference, and an
// uppercase .JSON is invisible to a suffix check while a case-insensitive
// filesystem hands it to Lookup anyway.
func TestValidateRefusesAFilenameThatIsNotAnArtName(t *testing.T) {
	for _, name := range []string{
		"Masonry-1.png", "masonry_1.png", "masonry-1.PNG", "y.JSON", "-x.png", "x-.png",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, name, "fake")
			err := artlib.Validate(dir)
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("got %v, want a refusal naming %s", err, name)
			}
		})
	}
}

// TestValidateIgnoresFilesThatAreNotArt draws the boundary the rule above
// needs: art is a .png and an optional .json, and everything else in the
// directory belongs to whoever put it there. A macOS .DS_Store must not stop a
// campaign booting.
func TestValidateIgnoresFilesThatAreNotArt(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`)
	writeFile(t, dir, ".DS_Store", "\x00\x00")
	writeFile(t, dir, "README.md", "where this art came from")
	if err := artlib.Validate(dir); err != nil {
		t.Fatalf("Validate: %v, want nil", err)
	}
}

// TestLookupWillNotFollowASymlinkOutOfTheArtDirectory is the threat
// cmd/vtt/maps.go already records for packs, arriving in a new place: art is
// read from a path a stranger controls once an adventure carries its own
// <adventure>/art/. A plain filepath.Join follows a symlink out of the tree,
// and fs.ValidPath never engages because there is no ".." in the name.
func TestLookupWillNotFollowASymlinkOutOfTheArtDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "art")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTileArt(t, root, "outside", `{"format_version":1,"kind":"wall","material":"SECRET-OUTSIDE"}`)
	if _, err := artlib.Lookup(root, "outside"); err != nil {
		t.Fatalf("test setup bug: the escape target must be loadable where it lives, got %v", err)
	}
	// Both spellings, because they are refused by different rules and only one
	// of them can travel inside a bundle: os.Root rejects an absolute link
	// outright, while a relative "../" link is the shape a tarball can carry.
	for i, target := range []string{"../outside", filepath.Join(root, "outside")} {
		id := fmt.Sprintf("evil-%d", i)
		if err := os.Symlink(target+".json", filepath.Join(dir, id+".json")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target+".png", filepath.Join(dir, id+".png")); err != nil {
			t.Fatal(err)
		}

		p, err := artlib.Lookup(dir, id)
		if err == nil {
			t.Fatalf("%s: Lookup followed a symlink out of art/ and returned %+v", target, p)
		}
		if p.Material == "SECRET-OUTSIDE" || strings.Contains(err.Error(), "SECRET-OUTSIDE") {
			t.Fatalf("%s: Lookup read the file outside art/: %+v %v", target, p, err)
		}
		if errors.Is(err, artlib.ErrNotFound) {
			t.Fatalf("%s: got %v, want an error that is NOT ErrNotFound: a symlink out of "+
				"art/ is art that exists and must not be read, and reporting it as absent "+
				"would tell a DM to go and install what is already there",
				target, err)
		}
	}
}

// TestADoorsPictureNamesMustBeFilenamesInTheArtDirectory: "open" and "closed"
// are strings from a sidecar that spec §6 turns into GET /api/art/{file}, so
// they are exactly as much a path as the id is. The escape targets are real,
// readable files, so the refusal can only be the guard.
func TestADoorsPictureNamesMustBeFilenamesInTheArtDirectory(t *testing.T) {
	for _, tc := range []struct{ field, open, closed string }{
		{"open", "../outside.png", "cellar-door-closed.png"},
		{"closed", "cellar-door-open.png", "../outside.png"},
		{"open", "/etc/hosts", "cellar-door-closed.png"},
		{"open", "sub/nested.png", "cellar-door-closed.png"},
		{"open", "cellar-door-open.jpg", "cellar-door-closed.png"},
		{"open", "Cellar-Door-Open.png", "cellar-door-closed.png"},
	} {
		t.Run(tc.field+" "+tc.open+" "+tc.closed, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "art")
			writePicture(t, root, "outside.png")
			writePicture(t, dir, "cellar-door-open.png")
			writePicture(t, dir, "cellar-door-closed.png")
			writePicture(t, filepath.Join(dir, "sub"), "nested.png")
			writeFile(t, dir, "cellar-door.json", `{"format_version":1,"kind":"door","material":"wood",
				"open":"`+tc.open+`","closed":"`+tc.closed+`"}`)

			_, err := artlib.Lookup(dir, "cellar-door")
			if err == nil || errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("got %v, want an error that is NOT ErrNotFound: a sidecar naming "+
					"something that is not a picture in art/ is installed and broken, not "+
					"absent", err)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Errorf("error %q must name the offending field", err)
			}
		})
	}
}

// TestANonDoorMayNotDeclareDoorPictures keeps every open/closed string that
// reaches a Piece behind the guard above: only a door has two pictures, so
// only a door may name them.
func TestANonDoorMayNotDeclareDoorPictures(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "masonry-1",
		`{"format_version":1,"kind":"wall","material":"stone","open":"../outside.png"}`)
	_, err := artlib.Lookup(dir, "masonry-1")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want a refusal that is NOT ErrNotFound", err)
	}
}

// TestAnUnknownFieldInASidecarIsRefused: plain json.Unmarshal accepts anything
// it does not recognise, so "knd" decoded to a Piece with an empty Kind and
// resolved as OBJECT art — the square would draw plain and nobody would learn
// why, which is the exact collapse this package exists to prevent. "pack" is
// the same mechanism carrying the word spec §7 refuses in a map file.
func TestAnUnknownFieldInASidecarIsRefused(t *testing.T) {
	for _, field := range []string{"knd", "pack", "kind_"} {
		t.Run(field, func(t *testing.T) {
			dir := t.TempDir()
			writeTileArt(t, dir, "masonry-1",
				`{"format_version":1,"`+field+`":"wall","material":"stone"}`)
			_, err := artlib.Lookup(dir, "masonry-1")
			if err == nil || errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("got %v, want a refusal that is NOT ErrNotFound", err)
			}
			if !strings.Contains(err.Error(), field) {
				t.Errorf("error %q must name the field that is not understood", err)
			}
		})
	}
}

// TestADeclaredFormatVersionThisServerDoesNotUnderstandCarriesItsOwnSentinel
// is the half of Patrik's ruling of 2026-09-04 that keeps its refusal. Every
// other unreadable sidecar now degrades one square (mapdef.Resolve), and this
// one may not join them: a declared version is not "this file is broken", it is
// "this content is newer than this server", and degrading a whole v2 art set
// into hundreds of plain squares reads as the first when the remedy is the
// second. One refusal naming both versions says that; ninety warnings do not.
//
// THE SENTINEL IS THE WHOLE POINT. mapdef.Resolve branches on errors.Is and
// must never branch on message text (Task 1's sentinel discipline), so a
// version refusal that came back as a plain error would silently degrade.
//
// THE THIRD ROW IS THE ONE THAT NEEDED CODE. A real v2 sidecar carries v2
// fields, and DisallowUnknownFields fires on those before any version check
// reads the file — so the sidecar this ruling exists for would have degraded
// as an unknown-field parse error while a bare {"format_version":2} refused.
// pieceFromSidecar reads the declared version first, on its own, for exactly
// this row; deleting that first pass leaves the other two green.
//
// EVERY ROW HERE IS LATER THAN THIS SERVER, and that is the whole membership
// rule. A `{"format_version":-1}` row sat here until art-is-a-flat-library
// Task 8 and was the defect in miniature: -1 is not a format anybody will ever
// ship, so the remedy the sentinel promises — go and get a newer server — is
// not available for it. It moved to
// TestAVersionBelowTheOneThisServerUnderstandsIsATypoAndDegrades with the
// typo'd 0 it always belonged beside, and 99 took its place so the table still
// spans more than one later number.
func TestADeclaredFormatVersionThisServerDoesNotUnderstandCarriesItsOwnSentinel(t *testing.T) {
	for _, tc := range []struct{ name, json, want string }{
		{"the next format", `{"format_version":2,"kind":"wall"}`, "declares 2"},
		{"a format far ahead of this one", `{"format_version":99,"kind":"wall"}`, "declares 99"},
		{"a later format carrying fields this one has never heard of",
			`{"format_version":2,"kind":"wall","variants":["mossy"]}`, "declares 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTileArt(t, dir, "future", tc.json)
			_, err := artlib.Lookup(dir, "future")
			if err == nil || errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("got %v, want a refusal that is NOT ErrNotFound: a sidecar written "+
					"for a later format is content this server is too old to read", err)
			}
			if !errors.Is(err, artlib.ErrFormatVersion) {
				t.Fatalf("error %v does not wrap ErrFormatVersion — without the sentinel "+
					"mapdef.Resolve degrades this the way it degrades a missing brace, and "+
					"a v2 art set becomes a wall of warnings", err)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "1") {
				t.Fatalf("error %q must say %q and name the version this server understands",
					err, tc.want)
			}
		})
	}
}

// TestAVersionBelowTheOneThisServerUnderstandsIsATypoAndDegrades is the
// direction half of the ruling, and the case an operator actually types.
//
// Review finding F4 of 2026-09-05 closed the PRESENCE half: {"format_version":
// 0} is declared, not absent, and must not be told "an undeclared format is not
// assumed to be any of them". It then made 0 refuse, which is the other error:
// ErrFormatVersion means the content is NEWER THAN THIS SERVER and the remedy
// is a newer server (spec §4, and the sentinel's own doc comment says so at
// length). No server has ever written a 0, so a 0 is a typo — and refusing it
// costs the whole map, and the whole BOOT when a committed map names it,
// because composeServer turns a map-load error into a refusal to start. That is
// the shape spec §4 exists to remove, arrived at through the version field.
//
// campaigncfg.Load already split this into two arms on 2026-09-05 for the same
// reason one directory over; there the split changes only the sentence, because
// campaign.json has no request-time reader to degrade for. Here it changes the
// VERDICT, which is why it is worth a test of its own.
//
// -1 IS THE SAME FACT and moved here from the sentinel test above with 0: it is
// not a later format either, and "declares -1" was refusing a map for a file
// nobody could have generated.
func TestAVersionBelowTheOneThisServerUnderstandsIsATypoAndDegrades(t *testing.T) {
	for _, tc := range []struct{ name, json, want string }{
		{"a typo'd zero", `{"format_version":0,"kind":"wall"}`, "declares 0"},
		{"a negative version", `{"format_version":-1,"kind":"wall"}`, "declares -1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTileArt(t, dir, "typo", tc.json)
			_, err := artlib.Lookup(dir, "typo")
			if err == nil || errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("got %v, want an error that is NOT ErrNotFound: the file is "+
					"installed and its version field cannot be honoured", err)
			}
			if errors.Is(err, artlib.ErrFormatVersion) {
				t.Fatalf("error %v wraps ErrFormatVersion, so mapdef.Resolve refuses the map "+
					"and composeServer refuses the boot — for a version no server has ever "+
					"written. The refusal is reserved for content NEWER than this server", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q must name the version the file declares", err)
			}
			if strings.Contains(err.Error(), "undeclared") {
				t.Fatalf("error %q calls a declared version undeclared", err)
			}
		})
	}
}

// TestAFormatVersionThatIsNotAVersionNumberIsCorruptRatherThanNewer is the
// THIRD answer this one field can give, and it exists because the other two
// are both wrong for it. A value that is not a version at all does not say the
// content is newer than this server (so it must not refuse), and it is not an
// absent field either (so "required" would be false of it).
//
// EVERY PLAUSIBLE SPELLING OF A LATER FORMAT IS A PLAIN JSON INTEGER, which is
// what makes degrading these safe: nothing this arm catches is the case the
// refusal was kept for. An integer past int32 is included deliberately —
// arguably "newer", certainly a typo, and the tie is broken by the same rule
// as the rest: this server cannot read it AS a version.
//
// null is here because it is the case a *int32 probe would still have got
// wrong: JSON null unmarshals into a pointer as nil without erroring, so it
// would have read as absent.
func TestAFormatVersionThatIsNotAVersionNumberIsCorruptRatherThanNewer(t *testing.T) {
	for _, tc := range []struct{ name, json string }{
		{"a string", `{"format_version":"2","kind":"wall"}`},
		{"a whole number written as a float", `{"format_version":1.0,"kind":"wall"}`},
		{"a fraction", `{"format_version":2.5,"kind":"wall"}`},
		{"exponent notation", `{"format_version":1e0,"kind":"wall"}`},
		{"past the end of an int32", `{"format_version":99999999999,"kind":"wall"}`},
		{"null", `{"format_version":null,"kind":"wall"}`},
		{"an object", `{"format_version":{"major":2},"kind":"wall"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTileArt(t, dir, "odd", tc.json)
			_, err := artlib.Lookup(dir, "odd")
			if err == nil || errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("got %v, want an error that is NOT ErrNotFound: the file is "+
					"installed and its version field is unusable", err)
			}
			if errors.Is(err, artlib.ErrFormatVersion) {
				t.Fatalf("error %v wraps ErrFormatVersion — this is not a declaration that "+
					"the content is newer than this server, it is a broken field, and "+
					"refusing it puts a typo back in the way of the whole boot", err)
			}
			if !strings.Contains(err.Error(), "is not a version number") {
				t.Fatalf("error %q must say what is wrong with the field", err)
			}
		})
	}
}

// TestAnUnreadableVersionFieldDoesNotShipItsWholeValueToTheClient bounds the
// one place this package interpolates author-controlled sidecar text into a
// message. Those messages ride to whoever issued load_map on a CommandResult,
// and spec §4 records a load whose warnings did not arrive because they went
// over the client read limit — so a sidecar carrying a very long number must
// not be able to put that number on the wire.
func TestAnUnreadableVersionFieldDoesNotShipItsWholeValueToTheClient(t *testing.T) {
	dir := t.TempDir()
	huge := strings.Repeat("9", 4000)
	writeTileArt(t, dir, "huge", `{"format_version":`+huge+`,"kind":"wall"}`)
	_, err := artlib.Lookup(dir, "huge")
	if err == nil {
		t.Fatal("want an error")
	}
	if len(err.Error()) > 500 {
		t.Fatalf("error is %d bytes; a 4000-digit number reached the wire whole",
			len(err.Error()))
	}
}

// TestAnUnreadableVersionFieldStaysValidUTF8OnTheWire guards the other half of
// quoting a campaign file's raw bytes back to a client, and it is the half a
// length check does not give.
//
// CommandResult.warnings is a proto3 repeated string, proto3 strings must be
// valid UTF-8, and this is the only place in the tree where raw bytes out of a
// campaign file reach one. Cutting at a fixed byte offset splits a multi-byte
// rune in half; a json.RawMessage also keeps whatever the file held, valid or
// not. Either one makes protojson refuse to marshal the frame — a campaign
// file deciding that a load_map answer never arrives at all, which is strictly
// worse than the warning it was carrying.
//
// The fixture is sized so the 40-byte bound lands INSIDE a two-byte rune: 21
// 'ü' is 42 bytes, so a naive cut at 40 ends on the lead byte of the 21st.
//
// FAULT-INJECTION PROOF (this assertion is after-the-fact, per CLAUDE.md rule
// 1). Removing clip's rune-boundary backup and cutting at the raw byte gives
// `…"üüüüüüüüüüüüüüüüüüü\xc3… is not a version number…` — a dangling lead byte,
// measured 2026-09-05. Nothing else in the suite notices: the verdict, the
// warning count and the square drawn plain are all unchanged, and the frame
// simply fails to marshal.
func TestAnUnreadableVersionFieldStaysValidUTF8OnTheWire(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "wide", `{"format_version":"`+strings.Repeat("ü", 21)+`","kind":"wall"}`)
	_, err := artlib.Lookup(dir, "wide")
	if err == nil {
		t.Fatal("want an error: a string is not a version number")
	}
	if !utf8.ValidString(err.Error()) {
		t.Fatalf("error is not valid UTF-8: %q — proto3 cannot carry it, so the frame "+
			"this rides on would not marshal at all", err.Error())
	}
	// THE EXACT FRAGMENT, not merely a valid one. Validity alone passes for a
	// clip that walks the WRONG WAY over the split rune — measured on the
	// first draft, whose cut++ mutant landed on the far boundary of the same
	// rune and produced a longer, still-valid string that nothing objected to.
	// 44 raw bytes, cut at 40, so the 20th u-umlaut is the one that splits.
	if want := `"` + strings.Repeat("ü", 19) + "…"; !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want it to carry %q", err.Error(), want)
	}
}

// TestAValueExactlyAtTheBoundIsNotClippedAtAll pins the bound itself. clip
// returns the whole fragment at exactly `limit` bytes and clips past it, and
// nothing else in this file distinguishes those: every other fixture is either
// far under or far over, so `len(s) <= limit` and `len(s) < limit` agree on all
// of them. The mutation gate found that gap rather than a reader.
//
// 38 x's inside two quotes is 40 raw bytes on the nose. Under the mutant the
// message gains an ellipsis while losing nothing, which is why the assertion is
// on the ellipsis and not on the length.
func TestAValueExactlyAtTheBoundIsNotClippedAtAll(t *testing.T) {
	dir := t.TempDir()
	value := strings.Repeat("x", 38)
	writeTileArt(t, dir, "onthenose", `{"format_version":"`+value+`","kind":"wall"}`)
	_, err := artlib.Lookup(dir, "onthenose")
	if err == nil {
		t.Fatal("want an error: a string is not a version number")
	}
	if !strings.Contains(err.Error(), `"`+value+`"`) {
		t.Fatalf("error = %q, want the whole 40-byte value", err.Error())
	}
	if strings.Contains(err.Error(), "…") {
		t.Fatalf("error = %q — 40 bytes is exactly the bound and is not clipped; an "+
			"ellipsis here means the comparison excludes its own boundary", err.Error())
	}
}

// TestTrailingBytesAfterASidecarAreIgnoredAsTheyAlwaysWere is review finding
// F2 of 2026-09-05, and it guards a NON-change rather than a change. The
// version pre-pass exists to reorder two reports; written with json.Unmarshal
// — the obvious spelling — it also silently narrowed what a sidecar may be,
// because Unmarshal refuses trailing bytes and Decode ignores them. Measured:
// both fixtures below resolved before the pre-pass existed and began degrading
// with "invalid character after top-level value" after it.
//
// That would also have made artlib stricter than mapdef.decodeStrict, which
// still ignores trailing data in a MAP file — one format tightened and its
// sibling not, by an edit whose stated purpose was neither.
//
// THIS TEST DOES NOT ARGUE THAT TRAILING BYTES SHOULD BE ACCEPTED. It pins
// that this task did not decide it. Refusing them is a real question for the
// sidecar and the map format together, and the day it is answered this test is
// the one to change, deliberately, rather than the assertion that quietly
// stopped being true.
func TestTrailingBytesAfterASidecarAreIgnoredAsTheyAlwaysWere(t *testing.T) {
	for _, tc := range []struct{ name, json string }{
		{"a word after the object", `{"format_version":1,"kind":"wall"} SURPRISE`},
		{"a second object", `{"format_version":1,"kind":"wall"}{"format_version":2}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTileArt(t, dir, "masonry-1", tc.json)
			p, err := artlib.Lookup(dir, "masonry-1")
			if err != nil {
				t.Fatalf("Lookup: %v — this resolved before the version pre-pass existed, "+
					"and the pre-pass was not a decision about trailing bytes", err)
			}
			if p.Kind != "wall" {
				t.Fatalf("got %+v, want the kind the first object declares", p)
			}
		})
	}
}

// TestASidecarThatDeclaresNoFormatVersionIsCorruptRatherThanNewer draws the
// line inside the format_version check itself. An undeclared version does not
// say the content is newer than this server; it says a field is missing, which
// is a hand-written file with a mistake in it — the same class as a missing
// brace, and it degrades with them (Patrik's ruling, 2026-09-04, read for what
// it says: the refusal is for a format "newer than this server").
//
// Keeping it a refusal would leave the measured defect half alive:
// {"kind":"wall","material":"stone"} is a plausible thing to hand-write, and
// under a refusal one such file in one campaign still stops the server booting
// over every other map.
func TestASidecarThatDeclaresNoFormatVersionIsCorruptRatherThanNewer(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "undeclared", `{"kind":"wall","material":"stone"}`)
	_, err := artlib.Lookup(dir, "undeclared")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want an error that is NOT ErrNotFound: the file is installed, "+
			"it is just incomplete", err)
	}
	if errors.Is(err, artlib.ErrFormatVersion) {
		t.Fatalf("error %v wraps ErrFormatVersion — an absent field is not a declaration "+
			"that the content is newer than this server, and refusing it puts one "+
			"hand-written sidecar back in the way of the whole boot", err)
	}
	if !strings.Contains(err.Error(), "required") {
		t.Fatalf("error %q must say the field is required", err)
	}
}

// TestASidecarThatIsNotValidJSONIsRefused's fixture is deliberately one
// wrong-TYPED field rather than plain garbage. encoding/json keeps decoding
// after a type error and returns it at the end, so the well-typed fields are
// populated either way — measured on this exact input:
//
//	{FormatVersion:1 Kind: Material:stone Open: Closed:}
//	err=json: cannot unmarshal number into Go struct field sidecar.kind of type string
//
// A mutant that dropped that error would therefore hand back
// Piece{ID:"broken", Kind:"", Material:"stone", File:"broken.png",
// HasSidecar:true} — a wall whose declared kind has silently become nothing,
// which is the collapse this package exists to prevent. A fixture like
// `{not json` would leave format_version at 0 and be caught by the NEXT check
// instead, proving nothing about this line.
func TestASidecarThatIsNotValidJSONIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeTileArt(t, dir, "broken", `{"format_version":1,"kind":123,"material":"stone"}`)
	_, err := artlib.Lookup(dir, "broken")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want an error that is NOT ErrNotFound: the caller degrades this "+
			"square either way, but "+
			"only a non-ErrNotFound answer gets the sentence that says there is a file "+
			"in art/ to go and fix", err)
	}
}

// TestADoorMissingOnePictureIsRefused asserts the MESSAGE, not merely that
// something went wrong. Deleting the completeness check leaves the empty
// string to fail the picture-name guard instead, which refuses for the right
// reason with the wrong explanation — "\"\" is not a picture in art/" tells a
// DM nothing about the field they left out.
func TestADoorMissingOnePictureIsRefused(t *testing.T) {
	for _, sidecar := range []string{
		`{"format_version":1,"kind":"door","material":"wood","open":"half-door-open.png"}`,
		`{"format_version":1,"kind":"door","material":"wood","closed":"half-door-closed.png"}`,
	} {
		dir := t.TempDir()
		writePicture(t, dir, "half-door-open.png")
		writePicture(t, dir, "half-door-closed.png")
		writeFile(t, dir, "half-door.json", sidecar)
		_, err := artlib.Lookup(dir, "half-door")
		if err == nil || errors.Is(err, artlib.ErrNotFound) {
			t.Fatalf("%s: got %v, want an error: a door with only one picture is an "+
				"incomplete file, and this package's job is to say so — what the caller "+
				"does with it is mapdef's (it degrades: internal/mapdef's "+
				"TestADoorMissingOnePictureDegradesRatherThanRefusingTheMap)", sidecar, err)
		}
		if !strings.Contains(err.Error(), "a door declares both") {
			t.Errorf("%s: error = %q, want it to name what a door must declare", sidecar, err)
		}
	}
}

// TestADoorMayNameTheSamePictureForBothStates pins a PERMISSION, and it is the
// sibling of the test above: a door missing one picture is refused, a door
// naming one picture twice is not. The reasoning is in pieceFromSidecar's door
// arm — a string comparison would refuse `"open"` and `"closed"` spelled alike
// while passing the likelier `cp closed.png open.png`, so it would read as a
// guarantee it cannot make.
//
// It exists so that whoever decides to refuse this later has to delete a test
// with the ruling attached, rather than adding a check over silence.
func TestADoorMayNameTheSamePictureForBothStates(t *testing.T) {
	dir := t.TempDir()
	writePicture(t, dir, "arch-both.png")
	writeFile(t, dir, "arch.json", `{"format_version":1,"kind":"door","material":"stone",
		"open":"arch-both.png","closed":"arch-both.png"}`)

	p, err := artlib.Lookup(dir, "arch")
	if err != nil {
		t.Fatalf("Lookup: %v — a door naming one picture twice is permitted on purpose; "+
			"see pieceFromSidecar's door arm", err)
	}
	if p.Open != "arch-both.png" || p.Closed != "arch-both.png" {
		t.Fatalf("got open %q closed %q, want both to be the one file the sidecar names",
			p.Open, p.Closed)
	}
	if p.File != "" {
		t.Errorf("got File=%q, want empty: it is still a door — two named states that "+
			"happen to share a picture, not a plain piece", p.File)
	}
	// Validate is the half `vtt art install` reads as its single refusal, so
	// installing such a door must not be refused either.
	if err := artlib.Validate(dir); err != nil {
		t.Errorf("Validate: %v — `vtt art install` refuses on any non-nil answer here", err)
	}
}

// TestAnUnreadableSidecarIsNotReportedAsAbsent was named
// TestAnUnreadableSidecarIsRefusedNotDegraded until 2026-09-04, and the second
// half of that stopped being true that day: the caller now degrades this
// square (mapdef.Resolve, Patrik's ruling). What this still pins is the half
// that matters here — the answer is NOT ErrNotFound, so the DM is told there
// is a file in art/ to go and fix rather than sent hunting for one that is not
// installed.
func TestAnUnreadableSidecarIsNotReportedAsAbsent(t *testing.T) {
	dir := t.TempDir()
	// A directory where the sidecar filename is expected makes the read fail
	// with something other than fs.ErrNotExist — the case Lookup's default
	// branch exists for.
	if err := os.Mkdir(filepath.Join(dir, "weird.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := artlib.Lookup(dir, "weird")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want an error that is NOT ErrNotFound: an unreadable sidecar "+
			"is a file sitting in art/, not absent art", err)
	}
}

// TestASidecarThatIsABrokenLinkIsRefusedNotTreatedAsAbsent: a dangling symlink
// reads as fs.ErrNotExist, which is the same answer as "there is no sidecar
// here" — and that collapse silently turned tile art into object art, drawing
// a wall as furniture. The entry exists; only its target does not.
func TestASidecarThatIsABrokenLinkIsRefusedNotTreatedAsAbsent(t *testing.T) {
	dir := t.TempDir()
	writePicture(t, dir, "masonry-1.png")
	// RELATIVE, and pointing inside art/: an absolute link is refused by
	// os.Root outright ("symbolic links must not be absolute") and would never
	// reach the ErrNotExist branch this test is about.
	if err := os.Symlink("gone.json", filepath.Join(dir, "masonry-1.json")); err != nil {
		t.Fatal(err)
	}
	p, err := artlib.Lookup(dir, "masonry-1")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got (%+v, %v), want a refusal that is NOT ErrNotFound: a sidecar that "+
			"exists and cannot be read is not the same fact as no sidecar", p, err)
	}
}

// TestAPictureThatCannotBeStattedIsRefusedNotTreatedAsAbsent: round 1
// discarded the stat error entirely, so EVERY stat failure read as "absent"
// and degraded. A symlink loop is not absence.
func TestAPictureThatCannotBeStattedIsRefusedNotTreatedAsAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("masonry-1.png", filepath.Join(dir, "masonry-1.png")); err != nil {
		t.Fatal(err)
	}
	_, err := artlib.Lookup(dir, "masonry-1")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want a refusal that is NOT ErrNotFound", err)
	}
}

func TestAPictureThatIsADirectoryIsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "masonry-1.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := artlib.Lookup(dir, "masonry-1")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want a refusal that is NOT ErrNotFound: a directory is not a "+
			"picture, and the renderer would be handed a name that cannot be served", err)
	}
}

func TestValidateAcceptsAWellFormedFlatDirectory(t *testing.T) {
	dir := t.TempDir()
	writePicture(t, dir, "pillar-stone.png")
	writeTileArt(t, dir, "masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`)
	if err := artlib.Validate(dir); err != nil {
		t.Fatalf("Validate(%q): %v, want nil — every sidecar has the pictures it names "+
			"beside it and there is no subdirectory", dir, err)
	}
}

// TestAnUnopenableArtDirIsASentinelAndNamesNoPath guards the two halves of a
// path disclosure that shipped in round 1 of art-is-a-flat-library Task 3 and
// was found in review.
//
// THE SENTINEL half: art/ present and unopenable is not one piece missing, it
// is every piece failing, and the two callers want opposite verdicts on it —
// a boot walk refuses (an operator can chmod a directory), a load at the table
// degrades (a DM in a browser cannot). Without ErrArtDirUnreadable there is no
// way to tell it from a parse failure, which must refuse in both.
//
// THE PATH half: this error reaches whoever issued load_map or load_adventure,
// verbatim, and load_adventure is granted to RoleAgent as well as RoleDM
// (internal/gateway/authz.go), so an agent seat would receive the operator's
// filesystem layout. os.OpenRoot's error is an *fs.PathError carrying the
// absolute path, so removing the path from the FORMAT STRING is not enough —
// round 1's %w kept it. The whole error has to be replaced by its inner
// syscall error, which is what the fs.ErrPermission assertion below pins is
// still reachable afterwards.
func TestAnUnopenableArtDirIsASentinelAndNamesNoPath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, root string) string
		is    error
	}{
		{"a plain file where art/ belongs", func(t *testing.T, root string) string {
			t.Helper()
			p := filepath.Join(root, "art")
			writeFile(t, root, "art", "not a dir")
			return p
		}, nil},
		{"a directory this process may not open", func(t *testing.T, root string) string {
			t.Helper()
			p := filepath.Join(root, "art")
			if err := os.Mkdir(p, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(p, 0o700) })
			return p
		}, fs.ErrPermission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := tc.build(t, root)

			_, err := artlib.Lookup(dir, "masonry-1")
			if err == nil {
				t.Fatal("an unopenable art directory resolved cleanly")
			}
			if !errors.Is(err, artlib.ErrArtDirUnreadable) {
				t.Fatalf("err = %v, want it to carry ErrArtDirUnreadable so a caller can "+
					"tell a broken art ROOT from a broken piece", err)
			}
			if errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("err = %v — art/ is THERE and unreadable; reporting it as absence "+
					"would make the boot walk degrade a broken installation", err)
			}
			if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), root) {
				t.Fatalf("err = %v — this text reaches an agent seat verbatim; it must not "+
					"carry the server's filesystem layout", err)
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("err = %v, want it to still answer errors.Is(%v) — dropping the path "+
					"must not drop the reason", err, tc.is)
			}
		})
	}
}

// TestNoLookupErrorNamesTheDirectoryItRead is the CLASS guard, written after
// three narrow fixes each closed one phase and left another
// (art-is-a-flat-library Task 3, review rounds 1-3). It walks the syscall
// phases behind an os.Root rather than the failures somebody happened to think
// of, because that is the axis the disclosure varies along — see bareCause's
// own doc comment for the per-phase table.
//
// THE READ PHASE IS THE ONE THAT ESCAPED TWICE. Root.ReadFile opens and then
// reads, and Root.Open hands back an *os.File whose Name() is the joined
// ABSOLUTE path, so a failure after the descriptor exists carries the whole
// campaign path while openat and statat failures carry a relative one. A
// DIRECTORY named <id>.json is the cheap way to reach it — openat on a
// directory succeeds, read(2) returns EISDIR — and it is a shape spec §3.1
// expects to find in an art directory rather than an exotic one.
//
// Every case must still SAY something: an error that named nothing would pass
// a "does not contain the path" check while being useless to a DM.
func TestNoLookupErrorNamesTheDirectoryItRead(t *testing.T) {
	for _, tc := range []struct {
		name, phase string
		build       func(t *testing.T, dir string)
	}{
		{"open: the art root is a plain file", "openat", func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, filepath.Dir(dir), filepath.Base(dir), "not a dir")
		}},
		{"read: a directory wearing a sidecar's name", "read", func(t *testing.T, dir string) {
			t.Helper()
			if err := os.MkdirAll(filepath.Join(dir, "masonry-1.json"), 0o750); err != nil {
				t.Fatal(err)
			}
		}},
		{"stat: the picture is a directory", "statat", func(t *testing.T, dir string) {
			t.Helper()
			writeFile(t, dir, "masonry-1.json", `{"format_version":1,"kind":"wall"}`)
			if err := os.MkdirAll(filepath.Join(dir, "masonry-1.png"), 0o750); err != nil {
				t.Fatal(err)
			}
		}},
		{"parse: a sidecar this server does not understand", "none", func(t *testing.T, dir string) {
			t.Helper()
			writeTileArt(t, dir, "masonry-1", `{"format_version":99}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "art")
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatal(err)
			}
			if tc.phase == "openat" {
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
			}
			tc.build(t, dir)

			_, err := artlib.Lookup(dir, "masonry-1")
			if err == nil {
				t.Fatalf("%s: resolved cleanly; this fixture is broken and must fail", tc.phase)
			}
			if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), root) {
				t.Fatalf("%s phase leaked the campaign's layout to a client:\n  %v", tc.phase, err)
			}
			if !strings.Contains(err.Error(), "masonry-1") && !errors.Is(err, artlib.ErrArtDirUnreadable) {
				t.Errorf("%s: err = %v, want it to name the art or say the art dir is unreadable",
					tc.phase, err)
			}
		})
	}
}

// --- IsArtFileName (art-is-a-flat-library Task 6) ---------------------------

// TestIsArtFileNameAcceptsExactlyAPictureAndASidecar pins the rule
// GET /api/art/{file} serves by. It is the only guard between that route and a
// subdirectory: os.OpenRoot CONFINES WITHOUT FLATTENING, so "art/pack-ish/x.png"
// is legitimately inside the root and fs.ValidPath rejects only "..".
//
// The refusals are the assertions that matter, and each one is a distinct
// escape rather than a variation on one: a subdirectory (flatness, design spec
// §3.1/§3.3), a traversal, an absolute path, a dotfile, an SVG (a document that
// can embed <script>, read by a same-origin script that then holds this
// client's Bearer token), and a bare stem with no extension at all.
func TestIsArtFileNameAcceptsExactlyAPictureAndASidecar(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
		why  string
	}{
		{"masonry-1.png", true, "a picture is what the browser draws"},
		{"masonry-1.json", true, "a sidecar is what tells a client which of a door's two pictures to ask for"},
		{"cellar-door-open.png", true, "a door's own picture names are art ids too"},
		{"a.png", true, "a one-character stem is a kebab-case id"},

		{"pack-ish/x.png", false, "a subdirectory is a namespace, and a namespace is a pack"},
		{"../secret.png", false, "a traversal is not a filename"},
		{"/etc/passwd.png", false, "an absolute path is not a filename"},
		{".hidden.png", false, "a leading dot is not a kebab-case id"},
		{"icon.svg", false, "an SVG can embed <script>; it is not art and this route does not hand it out"},
		{"masonry-1", false, "a bare stem is an id, not a file"},
		{"masonry-1.PNG", false, "an uppercase extension resolves on one filesystem and not the next"},
		{"Masonry-1.png", false, "an uppercase stem is not an art id (see this package's own doc)"},
		{"", false, "the empty name is not a file"},
		{".png", false, "an extension with no stem names nothing"},
		{"masonry-1.png.json", false, "a stem may not carry another extension"},
	} {
		if got := artlib.IsArtFileName(tc.name); got != tc.want {
			t.Errorf("IsArtFileName(%q) = %v, want %v — %s", tc.name, got, tc.want, tc.why)
		}
	}
}

// --- the art this repository actually ships ---------------------------------

// shippedArtDir is campaigns/example/art — the demo campaign's own flat art
// directory, resolved the way internal/mapdef/apidoc_test.go resolves the map
// beside it.
const shippedArtDir = "../../campaigns/example/art"

// TestTheShippedArtResolvesThroughThisPackage is the first thing in this tree
// to run the real reader over the real files.
//
// EVERY OTHER TEST IN THIS FILE WRITES ITS OWN FIXTURE, which is right for
// pinning behaviour and proves nothing about what ships. campaigns/example had
// no art at all until 2026-09-02-art-is-a-flat-library Task 8: the demo's
// overrides named four tile pieces that did not exist, so every square drew from
// the built-in vocabulary and a green suite said so about none of it.
//
// STRUCTURAL, NOT A TABLE OF THE FILES' OWN CONTENTS. A hand-copied list of
// kinds and materials here would restate the sidecars and fail whenever somebody
// legitimately retunes one; what must hold whatever the campaign draws is that
// every stem resolves, that a sidecar means tile art and its absence means
// object art (spec §3.4's asymmetry, which mapdef.Resolve decides on
// Piece.HasSidecar alone), and that a door has two pictures and no third.
//
// tools/genmappack's own test pins these bytes as that generator's output, so
// "what the generator writes resolves" follows from this plus that, without
// this package's tests or that tool's reaching across the layering
// (.go-arch-lint.yml keeps artlib self-only and genmappack out of it).
func TestTheShippedArtResolvesThroughThisPackage(t *testing.T) {
	if err := artlib.Validate(shippedArtDir); err != nil {
		t.Fatalf("artlib.Validate(campaigns/example/art): %v — every boot of the demo "+
			"campaign prints this", err)
	}

	entries, err := os.ReadDir(shippedArtDir)
	if err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, e := range entries {
		present[e.Name()] = true
	}
	if len(present) == 0 {
		t.Fatal("campaigns/example/art is empty, so every assertion below is vacuous")
	}

	// One Lookup per PICTURE stem, which is every id a map could name: a door's
	// two pictures are legal ids of their own, and object art has no sidecar to
	// find it by.
	looked := 0
	for name := range present {
		if filepath.Ext(name) != ".png" {
			continue
		}
		id := strings.TrimSuffix(name, ".png")
		piece, err := artlib.Lookup(shippedArtDir, id)
		if err != nil {
			t.Errorf("Lookup(%q): %v — the file is in art/ and a map naming it draws plain",
				id, err)
			continue
		}
		looked++
		if piece.HasSidecar != present[id+".json"] {
			t.Errorf("Lookup(%q).HasSidecar = %v, and %s.json on disk is %v",
				id, piece.HasSidecar, id, present[id+".json"])
		}
		if piece.HasSidecar && piece.Kind == "" {
			t.Errorf("Lookup(%q) has a sidecar declaring no kind; mapdef.Resolve cannot "+
				"tell that from object art", id)
		}
		if !piece.HasSidecar && piece.File != name {
			t.Errorf("Lookup(%q).File = %q, want %s", id, piece.File, name)
		}
	}
	if looked == 0 {
		t.Fatal("no picture resolved, so the loop above asserted nothing")
	}

	// The door, named because it is the one shape whose pictures are not its own
	// name and the one this campaign is the first real user of.
	door, err := artlib.Lookup(shippedArtDir, "cellar-door")
	if err != nil {
		t.Fatalf("Lookup(cellar-door): %v", err)
	}
	if door.Kind != kindDoorLiteral || door.File != "" {
		t.Errorf("cellar-door = kind %q file %q, want a door with no third picture",
			door.Kind, door.File)
	}
	if !present[door.Open] || !present[door.Closed] || door.Open == door.Closed {
		t.Errorf("cellar-door names open %q and closed %q; both must be in art/ and they "+
			"must be different files, or an opened door looks shut", door.Open, door.Closed)
	}
	if present["cellar-door.png"] {
		t.Error("cellar-door.png exists; a door has two pictures and no third (spec §3.4), " +
			"and a stray one is what a renderer would ask for and never get")
	}
}

// kindDoorLiteral is "door" spelled once here rather than reached for out of the
// package under test: artlib's own kindDoor is unexported, and an external test
// asserting the on-disk word should not be able to move with the code it checks.
const kindDoorLiteral = "door"

// TestAnIdOnlyMatchesAFileNamedExactlyThat is the one property this whole
// sub-project rests on, tested on the filesystem that quietly breaks it.
//
// The design makes the FILESYSTEM the uniqueness rule: art/ is flat, the
// filename stem IS the id, and there is no registry to disagree with. That is
// only a rule if it means the same thing everywhere, and it does not. APFS is
// case-insensitive by default and so is every HFS-descended volume, so
// `Masonry-1.png` and `masonry-1.png` are ONE file on the machine a DM works
// on and TWO on the Linux box or CI runner their campaign eventually meets.
//
// MEASURED before this test existed: Lookup(dir, "masonry-1") with only
// Masonry-1.png and Masonry-1.json on disk RETURNED A PIECE, with File set to
// "masonry-1.png" — a name no directory entry has. That id then reaches the
// client, which fetches /api/art/masonry-1.png, which resolves the same
// forgiving way. The campaign draws correctly on the Mac it was authored on and
// loses every one of those squares to plain terrain on Linux, with no warning
// anywhere, because from the server's point of view the piece resolved.
//
// Validate does catch the filename — but it REPORTS and the server starts
// anyway (Patrik's ruling 2026-09-03), and it runs at BOOT, so art installed
// while the table is running is never seen by it at all.
//
// So the fix belongs at the lookup: an id resolves only to a file named exactly
// that. Then the Mac behaves like the Linux box — the square degrades, the DM
// gets the ordinary not-installed warning, and one campaign draws one way.
func TestAnIdOnlyMatchesAFileNamedExactlyThat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Masonry-1.png", "picture bytes")
	writeFile(t, dir, "Masonry-1.json", `{"format_version":1,"kind":"wall"}`)

	// Guard the fixture: on a case-INSENSITIVE volume this open succeeds, and
	// that is the whole reason the test exists. On a case-sensitive one it
	// fails and the assertion below passes trivially — so say which regime ran.
	if _, err := os.Stat(filepath.Join(dir, "masonry-1.png")); err == nil {
		t.Log("case-INSENSITIVE volume: the lookup below is a real test")
	} else {
		t.Log("case-sensitive volume: the lookup below passes by construction")
	}

	_, err := artlib.Lookup(dir, "masonry-1")
	if !errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("Lookup(masonry-1) = %v, want ErrNotFound: only Masonry-1.* is on "+
			"disk, and an id must match a filename exactly or the same campaign "+
			"draws differently on a case-insensitive volume than on a case-sensitive one", err)
	}
}

// TestEveryNameBelowTheIdGateIsExactToo is the half the first version of the
// exact-name rule missed, and it missed it in the shape this repository ships.
//
// Requiring the ID to match an entry exactly established only that SOMETHING
// exists under it. Every read after that — the sidecar, the picture the sidecar
// names, and a door's open and closed pictures — went straight back through the
// case-folding filesystem. So a correctly-named sidecar beside a miscased
// picture still resolved, and so did a correct cellar-door.json naming
// Cellar-Door-Open.png. The gate looked like the fix and covered one case of
// three; caught in review by measurement, not by reading.
func TestEveryNameBelowTheIdGateIsExactToo(t *testing.T) {
	onCaseInsensitive := func(t *testing.T, dir string) bool {
		t.Helper()
		writeFile(t, dir, "Probe.marker", "x")
		_, err := os.Stat(filepath.Join(dir, "probe.marker"))
		return err == nil
	}

	t.Run("a correct sidecar naming a miscased picture", func(t *testing.T) {
		dir := t.TempDir()
		if !onCaseInsensitive(t, dir) {
			t.Skip("case-sensitive volume: the filesystem already refuses this")
		}
		writeFile(t, dir, "masonry-1.json", `{"format_version":1,"kind":"wall"}`)
		writeFile(t, dir, "Masonry-1.png", "picture bytes")

		_, err := artlib.Lookup(dir, "masonry-1")
		if !errors.Is(err, artlib.ErrNotFound) {
			t.Fatalf("Lookup = %v, want ErrNotFound: the sidecar is exact but the "+
				"picture it resolves to is not, and Piece.File would name a file "+
				"no directory entry carries", err)
		}
	})

	t.Run("a miscased sidecar beside a correct picture is picture-only", func(t *testing.T) {
		dir := t.TempDir()
		if !onCaseInsensitive(t, dir) {
			t.Skip("case-sensitive volume: the filesystem already refuses this")
		}
		// The miscased sidecar declares a DOOR. Read loosely, the piece comes
		// back as a door where the map asked for a wall — a different KIND, not
		// merely a different picture.
		writeFile(t, dir, "Masonry-1.json", `{"format_version":1,"kind":"door","open":"a.png","closed":"b.png"}`)
		writeFile(t, dir, "masonry-1.png", "picture bytes")

		p, err := artlib.Lookup(dir, "masonry-1")
		if err != nil {
			t.Fatalf("Lookup = %v, want the ordinary picture-only piece", err)
		}
		if p.HasSidecar || p.Kind != "" {
			t.Errorf("piece = %+v, want no sidecar and no kind: the only sidecar on "+
				"disk is not named masonry-1.json, so this is exactly the "+
				"picture-only piece a case-sensitive filesystem returns", p)
		}
	})

	t.Run("a door whose named pictures are miscased", func(t *testing.T) {
		dir := t.TempDir()
		if !onCaseInsensitive(t, dir) {
			t.Skip("case-sensitive volume: the filesystem already refuses this")
		}
		// The shipped cellar-door shape: a correct sidecar naming two pictures.
		writeFile(t, dir, "cellar-door.json",
			`{"format_version":1,"kind":"door","open":"cellar-door-open.png","closed":"cellar-door-closed.png"}`)
		writeFile(t, dir, "cellar-door.png", "picture bytes")
		writeFile(t, dir, "Cellar-Door-Open.png", "picture bytes")
		writeFile(t, dir, "Cellar-Door-Closed.png", "picture bytes")

		_, err := artlib.Lookup(dir, "cellar-door")
		if !errors.Is(err, artlib.ErrNotFound) {
			t.Fatalf("Lookup = %v, want ErrNotFound: the door's own pictures are "+
				"miscased, so this door opens on a Mac and on nothing else", err)
		}
		// And the refusal names the file, since that is the whole remedy.
		var mismatch *artlib.CaseMismatch
		if !errors.As(err, &mismatch) || mismatch.Real != "Cellar-Door-Open.png" {
			t.Errorf("err = %v, want it to name Cellar-Door-Open.png so the DM "+
				"knows which of the four files to rename", err)
		}
	})
}

// TestASidecarOnlyPieceReportsItsCaseMismatchToo covers the branch that reports
// a miscased SIDECAR when no picture is involved at all.
//
// It was written with the picture branch and then never reached: deleting it
// left artlib, mapdef, gateway and cmd/vtt all green, because both other tests
// write a picture and the picture branch matches first. A live branch no test
// enters is a survivor the mutation gate will find, and worse, a message
// nobody has read.
func TestASidecarOnlyPieceReportsItsCaseMismatchToo(t *testing.T) {
	dir := t.TempDir()
	// A sidecar and NO picture anywhere, miscased.
	writeFile(t, dir, "Masonry-1.json", `{"format_version":1,"kind":"wall"}`)

	_, err := artlib.Lookup(dir, "masonry-1")
	var mismatch *artlib.CaseMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("Lookup = %v, want a CaseMismatch naming the sidecar", err)
	}
	if mismatch.Real != "Masonry-1.json" {
		t.Errorf("mismatch.Real = %q, want the sidecar filename: it is the only "+
			"file in the directory and the only thing to rename", mismatch.Real)
	}
	if !errors.Is(err, artlib.ErrNotFound) {
		t.Errorf("err = %v, want it to still satisfy ErrNotFound so mapdef degrades", err)
	}
}

// TestArtThatVanishesBetweenTheSnapshotAndTheRead pins what the snapshot does
// when the directory changes under it.
//
// The snapshot is what makes an id resolve only to an exactly-named file, and
// it also creates a window that did not exist before: between Open and the read
// somebody can edit art/. The library is deliberately not refreshed — a load is
// a picture of the directory as it was when the load began — so these paths are
// reachable ONLY this way, and before this test they were reachable by nothing,
// which is what dropped the package under its coverage floor.
//
// None of the three may panic, and none may invent an answer. Each degrades to
// something a DM can read, which is the same contract the rest of the package
// keeps.
func TestArtThatVanishesBetweenTheSnapshotAndTheRead(t *testing.T) {
	install := func(t *testing.T) (string, *artlib.Library) {
		t.Helper()
		dir := t.TempDir()
		writeFile(t, dir, "masonry-1.json", `{"format_version":1,"kind":"wall"}`)
		writeFile(t, dir, "masonry-1.png", "picture bytes")
		return dir, artlib.Open(dir)
	}

	t.Run("the sidecar goes: the piece becomes picture-only", func(t *testing.T) {
		dir, lib := install(t)
		if err := os.Remove(filepath.Join(dir, "masonry-1.json")); err != nil {
			t.Fatal(err)
		}
		p, err := lib.Lookup("masonry-1")
		if err != nil {
			t.Fatalf("Lookup = %v, want the picture-only piece: the picture is still there", err)
		}
		if p.HasSidecar {
			t.Errorf("piece = %+v, want HasSidecar false — the sidecar is gone", p)
		}
	})

	t.Run("the picture goes: not installed, naming the file", func(t *testing.T) {
		dir, lib := install(t)
		if err := os.Remove(filepath.Join(dir, "masonry-1.png")); err != nil {
			t.Fatal(err)
		}
		_, err := lib.Lookup("masonry-1")
		if !errors.Is(err, artlib.ErrNotFound) {
			t.Fatalf("Lookup = %v, want ErrNotFound", err)
		}
		if !strings.Contains(err.Error(), "masonry-1.png") {
			t.Errorf("err = %v, want it to name the picture that went missing", err)
		}
	})

	t.Run("the whole directory goes: not installed, not unreadable", func(t *testing.T) {
		dir, lib := install(t)
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		_, err := lib.Lookup("masonry-1")
		// ErrNotFound, NOT ErrArtDirUnreadable: a campaign with no art directory
		// is ordinary and degrades, where an unreadable one is an operator fault
		// mapdef reports differently. The distinction is the degrade-vs-refuse
		// split, so a race must not quietly cross it.
		if !errors.Is(err, artlib.ErrNotFound) {
			t.Fatalf("Lookup = %v, want ErrNotFound", err)
		}
		if errors.Is(err, artlib.ErrArtDirUnreadable) {
			t.Errorf("err = %v, want it NOT to read as an unreadable art dir — "+
				"a directory that is gone is not one that cannot be read", err)
		}
	})
}

// TestACaseMismatchSaysWhatToRenameAndWhy reads the message itself, which
// nothing did — the type carried the filename for mapdef and its own Error()
// went unexercised. A message no test reads is a message nobody has read.
func TestACaseMismatchSaysWhatToRenameAndWhy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Masonry-1.png", "picture bytes")

	_, err := artlib.Lookup(dir, "masonry-1")
	got := err.Error()
	for _, want := range []string{
		`"masonry-1"`,      // what the map asked for
		`"Masonry-1.png"`,  // what is on disk, which is the thing to rename
		"differs only in case",
		"case-sensitive",   // WHY it is refused rather than quietly accepted
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, missing %q", got, want)
		}
	}
}

// TestTheArtDirectoryTurningUnreadableMidLoad covers the other half of the
// snapshot window: the directory does not vanish, it stops being a directory.
//
// A campaign with no art/ is ORDINARY and degrades (ErrNotFound); an art/ that
// exists and cannot be opened is an operator fault mapdef reports differently
// (ErrArtDirUnreadable). The window between Open and the read must not blur the
// two, because that line is the degrade-versus-refuse split.
//
// A plain FILE where art/ belongs, rather than a permission bit: a chmod
// fixture passes trivially for a process running as root, and CI containers
// often are. Same shape internal/gateway's own fixture uses.
func TestTheArtDirectoryTurningUnreadableMidLoad(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "art")
	writeFile(t, dir, "masonry-1.png", "picture bytes")
	lib := artlib.Open(dir)

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("a plain file where art/ belongs"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := lib.Lookup("masonry-1")
	if !errors.Is(err, artlib.ErrArtDirUnreadable) {
		t.Fatalf("Lookup = %v, want ErrArtDirUnreadable: art/ is no longer a "+
			"directory, which is an operator fault and not a campaign that "+
			"simply has no art", err)
	}
}

// TestBothHalvesOfAPieceVanishingMidLoad reaches the picture check through the
// sidecar path: the snapshot still lists both names, the sidecar read fails
// because the file is gone rather than because it is a broken link, and the
// picture it would have fallen back to is gone too.
func TestBothHalvesOfAPieceVanishingMidLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "masonry-1.json", `{"format_version":1,"kind":"wall"}`)
	writeFile(t, dir, "masonry-1.png", "picture bytes")
	lib := artlib.Open(dir)

	for _, name := range []string{"masonry-1.json", "masonry-1.png"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}

	_, err := lib.Lookup("masonry-1")
	if !errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("Lookup = %v, want ErrNotFound: both files are gone", err)
	}
	// NOT the broken-symlink sentence, which is what the Lstat arm one line up
	// exists to tell apart — a file that was deleted is not a link that does
	// not resolve, and conflating them once turned tile art into furniture.
	if strings.Contains(err.Error(), "does not resolve to a file") {
		t.Errorf("err = %v, want the not-installed sentence rather than the "+
			"broken-link one", err)
	}
}
