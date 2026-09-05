package main

// art_test.go covers `vtt art install` (art-is-a-flat-library design spec §5).
//
// THE POINT OF THE COMMAND IS THAT IT HAS NO AUTHORITY. Copying files into
// art/ is the primitive and it is complete — §3.3: "Two art pieces cannot share
// a name because two files cannot share a name." A DM who prefers `cp` is not
// doing anything wrong and loses only the early validation, and THE LOADER is
// the backstop that cannot be bypassed. So every refusal below is a
// convenience, and each test says which one and why a hand copy skips it.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaigncfg"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// srcFile writes one file into a source directory the DM is "holding", and
// returns its path. Nothing here writes into the campaign — that is the
// command's job, and a test that pre-installed the file would be asserting its
// own setup.
func srcFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// srcPNG writes a REAL PNG of the given pixel size. Real bytes rather than a
// stand-in, because cell_px is checked by decoding the picture's header — the
// one thing in this tree that reads a picture's content at all.
func srcPNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{R: 1, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func artPath(campaignPath, name string) string {
	return filepath.Join(campaignPath, "art", name)
}

func mustNotExist(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists: %s", path, why)
	}
}

// TestArtInstallCopiesFilesIntoAFlatArtDirectory is the happy path, and the
// only test here that proves the command does anything at all: the files land
// in <campaign>/art/ under their own names, because THE FILENAME IS THE
// IDENTITY (design spec §3.2) and nothing renames anything.
//
// It creates art/ on the way, so installing art can be the FIRST thing done to
// a campaign — the improvisation case §3.4 celebrates, and the same
// first-command ordering `vtt invite` already supports.
func TestArtInstallCopiesFilesIntoAFlatArtDirectory(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	pic := srcPNG(t, src, "masonry-1.png", 64, 64)
	side := srcFile(t, src, "masonry-1.json", `{"format_version":1,"kind":"wall","material":"stone"}`)

	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic, side)
	if err != nil {
		t.Fatalf("art install: %v (output %s)", err, out)
	}
	for _, name := range []string{"masonry-1.png", "masonry-1.json"} {
		got, err := os.ReadFile(artPath(campaignPath, name))
		if err != nil {
			t.Fatalf("%s was not installed: %v", name, err)
		}
		want, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: installed bytes differ from the source", name)
		}
		if !strings.Contains(out, name) {
			t.Errorf("output does not name %s: %q", name, out)
		}
	}
}

// TestArtInstallWarnsBeforeOverwritingAnExistingStem is Patrik's rule,
// 2026-09-02: "it will ask if you want to overwrite, otherwise it will ask you
// to change name/id". A CLI cannot ask, so the refusal IS the question and
// --force is the answer.
//
// Copying by hand stays legal and skips this — the command is a convenience,
// and §3.3 says replacing art is a file write: "drop a better masonry-1.png
// over the old one and every map using it is restyled. Being asked first is
// `cp -i`."
//
// THE EXISTING FILE MUST BE NAMED, and must still be there afterwards. A
// refusal that quietly half-installed the batch would be worse than no refusal.
func TestArtInstallWarnsBeforeOverwritingAnExistingStem(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	pic := srcPNG(t, src, "masonry-1.png", 64, 64)
	srcFile(t, src, "masonry-1.json", `{"format_version":1,"kind":"wall"}`)

	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath,
		pic, filepath.Join(src, "masonry-1.json")); err != nil {
		t.Fatalf("first install: %v (%s)", err, out)
	}

	// A DIFFERENT picture under the same stem — so "the old one is still
	// there" is an assertion about bytes rather than about a file existing.
	second := t.TempDir()
	newPic := srcPNG(t, second, "masonry-1.png", 64, 64)
	if err := os.WriteFile(newPic, []byte("a completely different picture"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Something else installs cleanly in the same command, so the refusal is
	// shown to be about the batch rather than about the one file.
	other := srcPNG(t, second, "earth-1.png", 64, 64)

	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, newPic, other)
	if err == nil {
		t.Fatalf("second install succeeded without --force; it must ask first (output %s)", out)
	}
	if !strings.Contains(err.Error(), "masonry-1.png") {
		t.Errorf("refusal does not name the existing file: %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("refusal does not say how to proceed: %v", err)
	}
	installed, readErr := os.ReadFile(artPath(campaignPath, "masonry-1.png"))
	if readErr != nil {
		t.Fatalf("the existing art was removed by a refused install: %v", readErr)
	}
	if string(installed) == "a completely different picture" {
		t.Fatal("the existing art was overwritten by an install that refused")
	}
	mustNotExist(t, artPath(campaignPath, "earth-1.png"),
		"a refused install must land nothing, or half a batch is installed and nothing says which half")

	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, "--force", newPic, other); err != nil {
		t.Fatalf("--force install: %v (%s)", err, out)
	}
	replaced, err := os.ReadFile(artPath(campaignPath, "masonry-1.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(replaced) != "a completely different picture" {
		t.Error("--force did not replace the installed art")
	}
}

// TestArtInstallRefusesADirectory: art/ is flat (design spec §3.1, §3.3).
// Accepting a directory here would install a pack — and a subdirectory is a
// namespace, which is what the whole sub-project exists to delete.
//
// The loader would catch it too (artlib.Validate names a subdirectory), but
// only at the next boot and only as a warning that the tree is inert. Here the
// operator is holding the file, so it is a refusal.
func TestArtInstallRefusesADirectory(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	packish := filepath.Join(src, "pack-ish")
	srcPNG(t, packish, "masonry-1.png", 64, 64)

	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, packish)
	if err == nil {
		t.Fatalf("installing a directory succeeded; art/ is flat (output %s)", out)
	}
	if !strings.Contains(err.Error(), "pack-ish") {
		t.Errorf("refusal does not name the directory: %v", err)
	}
	mustNotExist(t, filepath.Join(campaignPath, "art", "pack-ish"),
		"a directory must not be created inside art/ by a refused install")
}

// TestArtInstallRefusesANameNoMapCouldEverSpell closes the half a directory
// check does not: a file whose NAME cannot be an art id. The stem is the id
// (§3.2), so a name a map cannot spell installs art nothing can reference —
// inert, and silently so.
//
// The uppercase row is the one that matters and it is not cosmetic: artlib's
// own package doc records that Masonry-1.png resolves for "masonry-1" on macOS
// and does not exist at all on a case-sensitive Linux server, and that
// `vtt art install` is what is supposed to catch it.
func TestArtInstallRefusesANameNoMapCouldEverSpell(t *testing.T) {
	for _, tc := range []struct{ name, why string }{
		{"Masonry-1.png", "resolves on macOS and vanishes on Linux — artlib's own package doc"},
		{"masonry_1.png", "underscores are not kebab-case"},
		{"masonry-1.svg", "an SVG can embed <script>; it is not art"},
		{"notes.txt", "not art at all"},
		{"masonry--1.png", "a doubled hyphen is not a kebab-case id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaignPath := t.TempDir()
			src := t.TempDir()
			p := srcFile(t, src, tc.name, "bytes")
			out, err := runCLI(t, "art", "install", "--campaign", campaignPath, p)
			if err == nil {
				t.Fatalf("installed %s; %s (output %s)", tc.name, tc.why, out)
			}
			if !strings.Contains(err.Error(), tc.name) {
				t.Errorf("refusal does not name the file: %v", err)
			}
			mustNotExist(t, artPath(campaignPath, tc.name), "a refused install must land nothing")
		})
	}
}

// TestArtInstallValidatesTheSidecarAtInstallRatherThanAtTheTable is design
// spec §5's whole reason for the command to exist: "validates each sidecar so a
// malformed one is caught at install rather than at the table."
//
// format_version 99 is the one art failure that still REFUSES a map load
// (Patrik, 2026-09-04 — artlib.ErrFormatVersion), so shipping it would take a
// map down for everyone. The other rows degrade at the table to one plain
// square and a warning, which is exactly why catching them HERE is worth
// anything: the operator is holding the file, and the alternative is a DM
// finding out mid-session.
//
// AND THE DIRECTORY IS LEFT AS IT WAS. A validation that refuses after leaving
// the broken file installed would have moved the problem to the table rather
// than caught it.
func TestArtInstallValidatesTheSidecarAtInstallRatherThanAtTheTable(t *testing.T) {
	for _, tc := range []struct{ name, sidecar, want string }{
		{"a format this server does not understand", `{"format_version":99,"kind":"wall"}`, "99"},
		{"a missing brace", `{"format_version":1,"kind":"wall"`, "masonry-1"},
		{"a field this server does not know", `{"format_version":1,"knid":"wall"}`, "masonry-1"},
		{"a door naming only one of its two pictures",
			`{"format_version":1,"kind":"door","closed":"masonry-1-closed.png"}`, "masonry-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaignPath := t.TempDir()
			src := t.TempDir()
			pic := srcPNG(t, src, "masonry-1.png", 64, 64)
			side := srcFile(t, src, "masonry-1.json", tc.sidecar)

			out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic, side)
			if err == nil {
				t.Fatalf("installed a sidecar that is %s (output %s)", tc.name, out)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refusal does not name %q: %v", tc.want, err)
			}
			mustNotExist(t, artPath(campaignPath, "masonry-1.json"),
				"a refused install left the broken sidecar in art/, which is the table finding out instead of us")
			mustNotExist(t, artPath(campaignPath, "masonry-1.png"),
				"a refused install left half the batch behind")
		})
	}
}

// TestARefusedForcedInstallPutsTheOldArtBack is the arm --force opens and
// nothing else can reach: the destination already holds good art, the incoming
// sidecar is broken, and the command has already been told it may overwrite.
//
// Either the whole batch installs and resolves, or art/ is exactly as it was.
// Without the restore, `--force` on a bad file would DESTROY working art and
// then refuse — the worst of both answers, and one a DM would discover at the
// table with the file they were replacing already gone.
func TestARefusedForcedInstallPutsTheOldArtBack(t *testing.T) {
	campaignPath := t.TempDir()
	first := t.TempDir()
	pic := srcPNG(t, first, "masonry-1.png", 64, 64)
	good := srcFile(t, first, "masonry-1.json", `{"format_version":1,"kind":"wall","material":"stone"}`)
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic, good); err != nil {
		t.Fatalf("first install: %v (%s)", err, out)
	}
	before, err := os.ReadFile(artPath(campaignPath, "masonry-1.json"))
	if err != nil {
		t.Fatal(err)
	}

	second := t.TempDir()
	bad := srcFile(t, second, "masonry-1.json", `{"format_version":99}`)
	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, "--force", bad)
	if err == nil {
		t.Fatalf("a forced install of a broken sidecar succeeded (output %s)", out)
	}
	after, readErr := os.ReadFile(artPath(campaignPath, "masonry-1.json"))
	if readErr != nil {
		t.Fatalf("--force destroyed the installed art and then refused: %v", readErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("art/masonry-1.json = %q after a refused forced install, want the original %q",
			after, before)
	}
}

// TestAnInstallThatCannotSetTheOldArtAsideLeavesItInPlace is the arm the undo
// path got backwards (review finding F4, 2026-09-05): when the set-aside rename
// fails, installOne returned a record naming a destination it had NOT written,
// the caller appended it to the done list, and undo() removed it — deleting
// working art that this run never touched, against this file's own header
// promise that a refused batch leaves "art/ exactly as it was".
//
// A STALE .vtt-replaced DIRECTORY IS WHAT REACHES IT, and it is the reason this
// is a test rather than an argument: rename(2) will not replace a directory
// with a file, so the set-aside fails while os.Remove on the original would
// have succeeded — the one combination that turns the undo into a deletion. A
// read-only art/ fails BOTH operations, so nothing is lost there and the bug is
// invisible. The directory is a plausible leftover, too: `.vtt-replaced` is what
// this command names its own backups, and artlib ignores the suffix entirely.
//
// THE ASSERTION IS THE FILE'S BYTES, not the error. A refusal here is correct
// and expected; what must never happen is the refusal costing the DM the art
// they already had.
func TestAnInstallThatCannotSetTheOldArtAsideLeavesItInPlace(t *testing.T) {
	campaignPath := t.TempDir()
	first := t.TempDir()
	pic := srcPNG(t, first, "masonry-1.png", 64, 64)
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic); err != nil {
		t.Fatalf("first install: %v (%s)", err, out)
	}
	before, err := os.ReadFile(artPath(campaignPath, "masonry-1.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(artPath(campaignPath, "masonry-1.png.vtt-replaced"), 0o755); err != nil {
		t.Fatal(err)
	}

	second := t.TempDir()
	replacement := srcPNG(t, second, "masonry-1.png", 128, 128)
	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, "--force", replacement)
	if err == nil {
		t.Fatalf("the install reported success although it could not set the old art aside (%s)", out)
	}
	after, readErr := os.ReadFile(artPath(campaignPath, "masonry-1.png"))
	if readErr != nil {
		t.Fatalf("art/masonry-1.png is gone after a failed install: %v — the undo removed a "+
			"destination this run never wrote, which is the DM's working art deleted by a "+
			"command that refused", readErr)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("art/masonry-1.png changed although the install refused")
	}
}

// TestArtInstallAcceptsAPictureWithNoSidecar is design spec §3.4's
// improvisation case, and it is the reason this command must not require a
// sidecar: "dropping a PNG into art/ and referencing it as furniture must be a
// one-step act. Requiring a sidecar for it would put a JSON file between the DM
// and a piece of scenery for no gain."
func TestArtInstallAcceptsAPictureWithNoSidecar(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	pic := srcPNG(t, src, "pillar-stone.png", 64, 64)
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic); err != nil {
		t.Fatalf("art install: %v (%s) — object art needs no sidecar", err, out)
	}
	if _, err := os.Stat(artPath(campaignPath, "pillar-stone.png")); err != nil {
		t.Fatalf("pillar-stone.png was not installed: %v", err)
	}
}

// TestArtInstallRefusesASidecarWhoseArtIsNotInstalled pins the other direction
// of the same asymmetry: a sidecar with no picture is an error (§3.4), and this
// is the one Lookup check that depends on the DESTINATION rather than on the
// file being copied — so it can only be made after the copy, against the
// directory as it will actually be.
func TestArtInstallRefusesASidecarWhoseArtIsNotInstalled(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	side := srcFile(t, src, "masonry-1.json", `{"format_version":1,"kind":"wall"}`)

	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, side)
	if err == nil {
		t.Fatalf("installed a sidecar with no picture anywhere (output %s)", out)
	}
	mustNotExist(t, artPath(campaignPath, "masonry-1.json"), "a refused install must land nothing")
}

// TestASidecarInstallsAgainstAPictureAlreadyInArt is the case the test above
// would hide if the check ran on the SOURCE files rather than on art/ as it
// will be: installing only the sidecar, when the picture is already installed,
// is an ordinary act and must work.
func TestASidecarInstallsAgainstAPictureAlreadyInArt(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	pic := srcPNG(t, src, "masonry-1.png", 64, 64)
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic); err != nil {
		t.Fatalf("installing the picture: %v (%s)", err, out)
	}
	side := srcFile(t, src, "masonry-1.json", `{"format_version":1,"kind":"wall","material":"stone"}`)
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, side); err != nil {
		t.Fatalf("installing the sidecar beside an already-installed picture: %v (%s)", err, out)
	}
}

// --- cell_px gets its first reader (design spec §6) -------------------------

// TestArtInstallWarnsWhenAPictureIsNotOnTheCampaignsGrid is what campaign.json
// is FOR, and it is the whole answer to "what does cell_px do now that art can
// be any size".
//
// NOTHING IN THE RENDERER READS IT AND NOTHING SHOULD. client/src/view/
// spectator.ts's CELL = 44 is a SCREEN size — how big a square is drawn — while
// cell_px is a SOURCE size, how big the picture is, and drawImage scales one to
// the other whatever they are. Wiring cell_px into a draw call would change
// pixels for no requirement. What the number actually asserts is design spec
// §6's own sentence: "a grid is uniform, and art pieces at differing native
// resolutions on the same board is a rendering problem, not a capability."
//
// So it binds where the operator is holding the file. A WARNING rather than a
// refusal, because a lower-resolution picture draws perfectly well and the DM
// may mean it — the same posture §4 takes about everything art can get wrong.
//
// A WHOLE MULTIPLE, not equality: an object may be more than one square wide
// (mapdef's Object.size), so a 2x1 brazier at 128x64 is exactly right.
func TestArtInstallWarnsWhenAPictureIsNotOnTheCampaignsGrid(t *testing.T) {
	for _, tc := range []struct {
		name       string
		w, h       int
		wantWarned bool
	}{
		{"one square at the declared size", 64, 64, false},
		{"a two-by-one object", 128, 64, false},
		{"half the declared size", 32, 32, true},
		{"square but off the grid", 50, 50, true},
		{"right in one axis and wrong in the other", 64, 50, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaignPath := t.TempDir()
			src := t.TempDir()
			pic := srcPNG(t, src, "masonry-1.png", tc.w, tc.h)
			out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic)
			if err != nil {
				t.Fatalf("art install: %v (%s) — a picture off the grid still installs", err, out)
			}
			warned := strings.Contains(out, "cell_px")
			if warned != tc.wantWarned {
				t.Fatalf("warned = %v, want %v (output %q)", warned, tc.wantWarned, out)
			}
			if tc.wantWarned {
				for _, want := range []string{"64", "masonry-1.png"} {
					if !strings.Contains(out, want) {
						t.Errorf("warning does not name %q: %q", want, out)
					}
				}
			}
		})
	}
}

// TestTheCampaignsOwnCellPxIsWhatThePictureIsCheckedAgainst is the half above
// cannot see: with campaign.json declaring 32, a 32x32 picture is on the grid
// and a 64x64 one is off it — the exact inverse of the default. Without this,
// a check hard-coded to 64 passes every row above.
func TestTheCampaignsOwnCellPxIsWhatThePictureIsCheckedAgainst(t *testing.T) {
	campaignPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(campaignPath, "campaign.json"),
		[]byte(`{"format_version":1,"cell_px":48}`), 0o644); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	onGrid := srcPNG(t, src, "masonry-1.png", 48, 48)
	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, onGrid)
	if err != nil {
		t.Fatalf("art install: %v (%s)", err, out)
	}
	if strings.Contains(out, "cell_px") {
		t.Errorf("48x48 warned against a campaign declaring cell_px 48: %q", out)
	}

	offGrid := srcPNG(t, src, "earth-1.png", 64, 64)
	out, err = runCLI(t, "art", "install", "--campaign", campaignPath, offGrid)
	if err != nil {
		t.Fatalf("art install: %v (%s)", err, out)
	}
	if !strings.Contains(out, "48") {
		t.Errorf("64x64 did not warn against a campaign declaring cell_px 48: %q", out)
	}
}

// TestABrokenCampaignJSONStopsAnInstall keeps campaigncfg's refusal reachable
// from the one command an operator runs before the server ever starts. A
// campaign.json that cannot be read is the operator's to fix, and finding out
// now beats finding out when `vtt serve` refuses to boot.
func TestABrokenCampaignJSONStopsAnInstall(t *testing.T) {
	campaignPath := t.TempDir()
	if err := os.WriteFile(filepath.Join(campaignPath, "campaign.json"),
		[]byte(`{"format_version":1,"cell_px":`), 0o644); err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	pic := srcPNG(t, src, "masonry-1.png", 64, 64)
	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic)
	if err == nil {
		t.Fatalf("installed against an unreadable campaign.json (output %s)", out)
	}
	if !strings.Contains(err.Error(), "campaign.json") {
		t.Errorf("refusal does not name campaign.json: %v", err)
	}
}

// TestAPictureThatIsNotAPNGIsInstalledWithAWarning covers the arm the grid
// check opens and must not turn into a new rule: nothing else in this tree ever
// reads a picture's CONTENT (artlib.statPicture only stats), so a .png that is
// not a PNG is not something the platform has an opinion about. The operator is
// told the grid could not be checked, and the file installs — refusing here
// would invent a content rule design spec §3 does not have.
func TestAPictureThatIsNotAPNGIsInstalledWithAWarning(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	pic := srcFile(t, src, "masonry-1.png", "this is not a PNG at all")
	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic)
	if err != nil {
		t.Fatalf("art install: %v (%s)", err, out)
	}
	if _, statErr := os.Stat(artPath(campaignPath, "masonry-1.png")); statErr != nil {
		t.Fatalf("the file was not installed: %v", statErr)
	}
	if !strings.Contains(out, "masonry-1.png") || !strings.Contains(strings.ToLower(out), "png") {
		t.Errorf("nothing said the grid could not be checked: %q", out)
	}
}

// TestArtInstallRefusesASourceThatIsNotThere is the ordinary typo, and it is
// here so that "a refused install lands nothing" covers the case where the
// refusal happens before any file is even opened.
func TestArtInstallRefusesASourceThatIsNotThere(t *testing.T) {
	campaignPath := t.TempDir()
	missing := filepath.Join(t.TempDir(), "masonry-1.png")
	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, missing)
	if err == nil {
		t.Fatalf("installed a file that does not exist (output %s)", out)
	}
	mustNotExist(t, artPath(campaignPath, "masonry-1.png"), "nothing to install, nothing installed")
}

// TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber is the guard the
// two constants need and neither package can carry.
//
// internal/gateway reads no files and must not depend on internal/campaigncfg;
// internal/campaigncfg knows nothing about a server. cmd/vtt imports both — it
// is the composition root — so this is the one place that can see them at once,
// and without it a gateway defaulting to 64 and a loader defaulting to
// something else would disagree with nothing to say so.
func TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber(t *testing.T) {
	if gateway.DefaultCellPx != campaigncfg.DefaultCellPx {
		t.Fatalf("gateway.DefaultCellPx = %d, campaigncfg.DefaultCellPx = %d — a server told nothing "+
			"and a campaign that declared nothing must report the same grid",
			gateway.DefaultCellPx, campaigncfg.DefaultCellPx)
	}
}

// TestTheCellPxConstantsAgreeAcrossThePackagesThatCarryThem is the same guard
// for the BOUNDS, and it exists because three packages hold a copy and none of
// them may import another.
//
// internal/campaigncfg is self-only (a settings reader has no business knowing
// the map format), internal/mapdef may not read campaign settings, and
// internal/gateway reads no files at all. cmd/vtt imports all three — it is the
// composition root — so this is the one place that can see them at once. Without
// it, a map file could declare a cell_px that campaign.json would refuse, or the
// reverse, with nothing anywhere to say so.
func TestTheCellPxConstantsAgreeAcrossThePackagesThatCarryThem(t *testing.T) {
	if mapdef.MinCellPx != campaigncfg.MinCellPx {
		t.Errorf("mapdef.MinCellPx = %d, campaigncfg.MinCellPx = %d — a map and its campaign must "+
			"agree about the smallest square either may declare",
			mapdef.MinCellPx, campaigncfg.MinCellPx)
	}
	if mapdef.MaxCellPx != campaigncfg.MaxCellPx {
		t.Errorf("mapdef.MaxCellPx = %d, campaigncfg.MaxCellPx = %d", mapdef.MaxCellPx, campaigncfg.MaxCellPx)
	}
	// AND THE DEFAULT SITS INSIDE THEM, or every campaign that declares nothing
	// holds a number no file in it could have written.
	if campaigncfg.DefaultCellPx < mapdef.MinCellPx || campaigncfg.DefaultCellPx > mapdef.MaxCellPx {
		t.Errorf("the default %d is outside the bounds %d..%d",
			campaigncfg.DefaultCellPx, mapdef.MinCellPx, mapdef.MaxCellPx)
	}
}

// TestASuccessfulForcedInstallLeavesNoLitter closes the other end of the
// backup TestARefusedForcedInstallPutsTheOldArtBack depends on: the replaced
// file is set aside so a refusal can put it back, and when nothing refuses it
// has to GO.
//
// artlib.Validate would not catch this — a ".vtt-replaced" suffix is not a
// sidecar or a picture extension, so entryProblem ignores it exactly as it
// ignores a .DS_Store — which is what makes it litter rather than an error: one
// more file per --force, forever, in a directory whose whole design is that a
// human can read it (design spec §9: "several hundred files in one folder is
// ordinary for a filesystem and awkward for a human browsing it").
func TestASuccessfulForcedInstallLeavesNoLitter(t *testing.T) {
	campaignPath := t.TempDir()
	src := t.TempDir()
	pic := srcPNG(t, src, "masonry-1.png", 64, 64)
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic); err != nil {
		t.Fatalf("first install: %v (%s)", err, out)
	}
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, "--force", pic); err != nil {
		t.Fatalf("forced install: %v (%s)", err, out)
	}
	entries, err := os.ReadDir(filepath.Join(campaignPath, "art"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 1 || names[0] != "masonry-1.png" {
		t.Fatalf("art/ holds %v, want exactly masonry-1.png — a successful install leaves nothing behind",
			names)
	}
}

// TestArtInstallRefusesTwoSourcesWithTheSameFilename is the collision that has
// no destination to check against, because neither file is installed yet.
//
// THE FILESYSTEM IS THE UNIQUENESS RULE (design spec §3.3) and it holds inside
// art/ — but a single command naming two sources with one basename resolves that
// by writing one over the other, and the last one silently wins. The pre-flight
// overwrite check cannot see it: it asks whether the DESTINATION exists, and at
// that moment neither does. That is the same "you have to rename one" answer
// §3.3 already gives, arrived at one step earlier.
func TestArtInstallRefusesTwoSourcesWithTheSameFilename(t *testing.T) {
	campaignPath := t.TempDir()
	a := t.TempDir()
	b := t.TempDir()
	first := srcPNG(t, a, "masonry-1.png", 64, 64)
	second := srcFile(t, b, "masonry-1.png", "a completely different picture")

	out, err := runCLI(t, "art", "install", "--campaign", campaignPath, first, second)
	if err == nil {
		t.Fatalf("installed two sources named masonry-1.png; one silently overwrote the other (output %s)", out)
	}
	if !strings.Contains(err.Error(), "masonry-1.png") {
		t.Errorf("refusal does not name the colliding file: %v", err)
	}
	mustNotExist(t, artPath(campaignPath, "masonry-1.png"), "a refused install must land nothing")
	// AND --force MUST NOT ANSWER IT. --force means "replace what is installed",
	// which says nothing about which of two incoming files you meant.
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, "--force", first, second); err == nil {
		t.Fatalf("--force installed two sources named masonry-1.png (output %s)", out)
	}
}
