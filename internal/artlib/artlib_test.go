package artlib_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			"sidecar is refused) needs this bit, and Kind == \"\" cannot carry it: a "+
			"sidecar declaring no kind produces the same empty string", p)
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
			t.Fatalf("%s: got %v, want a refusal that is NOT ErrNotFound: a symlink out of "+
				"art/ is art that exists and must not be read, which is a defect to fix",
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
				t.Fatalf("got %v, want a refusal that is NOT ErrNotFound: a sidecar naming "+
					"something that is not a picture in art/ is malformed art", err)
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

func TestAnUnsupportedFormatVersionIsRefusedNotDegraded(t *testing.T) {
	for _, tc := range []struct{ name, json, want string }{
		{"a later format", `{"format_version":2,"kind":"wall"}`, "declares 2"},
		{"a nonsense format", `{"format_version":-1,"kind":"wall"}`, "declares -1"},
		{"no format at all", `{"kind":"wall"}`, "required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTileArt(t, dir, "future", tc.json)
			_, err := artlib.Lookup(dir, "future")
			if err == nil || errors.Is(err, artlib.ErrNotFound) {
				t.Fatalf("got %v, want a refusal that is NOT ErrNotFound: art that exists and "+
					"cannot be read is a defect to fix, not a square to draw plain", err)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "1") {
				t.Fatalf("error %q must say %q and name the version this server understands",
					err, tc.want)
			}
		})
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
		t.Fatalf("got %v, want a refusal that is NOT ErrNotFound: a sidecar that will not "+
			"even parse is a defect to fix, not a square to draw plain", err)
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
			t.Fatalf("%s: got %v, want a refusal: a door with only one picture is a defect "+
				"to fix, not a square to draw plain", sidecar, err)
		}
		if !strings.Contains(err.Error(), "a door declares both") {
			t.Errorf("%s: error = %q, want it to name what a door must declare", sidecar, err)
		}
	}
}

func TestAnUnreadableSidecarIsRefusedNotDegraded(t *testing.T) {
	dir := t.TempDir()
	// A directory where the sidecar filename is expected makes the read fail
	// with something other than fs.ErrNotExist — the case Lookup's default
	// branch exists for.
	if err := os.Mkdir(filepath.Join(dir, "weird.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := artlib.Lookup(dir, "weird")
	if err == nil || errors.Is(err, artlib.ErrNotFound) {
		t.Fatalf("got %v, want a refusal that is NOT ErrNotFound: an unreadable sidecar "+
			"is a defect to fix, not absent art", err)
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
