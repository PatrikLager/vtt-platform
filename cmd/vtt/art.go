package main

// art.go is `vtt art install` (2026-09-02-art-is-a-flat-library design spec §5).
//
// THE COMMAND HAS NO AUTHORITY, and that is the design rather than a limit on
// it. Copying files into art/ is the primitive and it is complete: §3.3 —
// "Two art pieces cannot share a name because two files cannot share a name.
// There is no duplicate check to write, no collision prompt to design, and no
// registry to keep consistent." A DM who prefers `cp` is not doing anything
// wrong and loses only the early validation. THE LOADER is the backstop that
// cannot be bypassed (artlib.Validate at boot, artlib.Lookup at load_map).
//
// SO EVERY REFUSAL HERE IS A CONVENIENCE, and each one is chosen because the
// operator is HOLDING THE FILE. The same conditions at the table are warnings
// (spec §4: a campaign that will not load is worse than one that loads slightly
// plain), and the same conditions at boot are a report that starts the server
// anyway (Patrik's severity ruling, 2026-09-03). This is the one place the
// person who can fix the file is standing right there, so §5 says it "refuses
// outright".
//
// ALL OR NOTHING. A batch either installs completely and resolves, or art/ is
// exactly as it was — new files removed, overwritten files put back. A half-
// installed batch is the failure mode this command exists to prevent, arrived
// at from the inside: the DM would be left with some of their art in place,
// some not, and a message about only one file.

import (
	"fmt"
	"image"
	_ "image/png" // registers the PNG decoder image.DecodeConfig reads with
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/artlib"
	"github.com/PatrikLager/vtt-platform/internal/campaigncfg"
)

// installed records one file this run put into art/, and what was there
// before, so a refusal after the copy can put the directory back exactly as it
// found it. backup is empty when nothing was overwritten, which is every case
// but --force.
//
// wrote SAYS WHETHER dest IS THIS RUN'S DOING, and it is not the same question
// as "is dest set". installOne returns its record on failure too, so the undo
// can restore a backup it had already moved aside — which means a record can
// name a destination this run never wrote: the set-aside rename fails, the
// original art is still sitting there untouched, and an undo keyed on dest
// alone DELETES IT (review finding F4, 2026-09-05,
// TestAnInstallThatCannotSetTheOldArtAsideLeavesItInPlace). A refusal must
// never cost a DM art that this command did not put there.
type installed struct {
	dest   string
	backup string
	wrote  bool
}

// newArtCmd assembles `vtt art`, whose only subcommand is `install`. A parent
// with one child rather than a bare `vtt art-install`, because the noun is what
// a DM is thinking about and the next verb this grows (a listing, say) belongs
// under the same one.
func newArtCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "art",
		Short: "Manage a campaign's flat art/ directory",
	}
	cmd.AddCommand(newArtInstallCmd())
	return cmd
}

func newArtInstallCmd() *cobra.Command {
	var campaignPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "install <path>...",
		Short: "Copy art files into the campaign's art/ directory",
		Long: "Copy pictures and sidecars into <campaign>/art/, keeping their own filenames — " +
			"the filename is the art id a map names. Refuses a directory, refuses a name already " +
			"installed unless --force, and validates every sidecar before it lands.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runArtInstall(cmd, campaignPath, force, args)
		},
	}
	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign directory (required)")
	// --force is the ANSWER TO A QUESTION A CLI CANNOT ASK. Patrik's rule,
	// 2026-09-02: "it will ask if you want to overwrite, otherwise it will ask
	// you to change name/id". A non-interactive command cannot ask, so the
	// refusal is the question and this flag is the answer — and §3.3 already
	// says being asked first is what `cp -i` is for.
	cmd.Flags().BoolVar(&force, "force", false,
		"replace art already installed under the same filename")
	_ = cmd.MarkFlagRequired("campaign")
	return cmd
}

// runArtInstall is the whole command: check everything that can be checked
// before writing, copy, then resolve each installed stem through the SAME
// artlib.Lookup a map load runs — never a second validator that could disagree
// with it, which is the divergence hazard mapdef.LoadInstalled's own doc comment
// is written about.
//
// THE ORDER IS LOAD-BEARING. Every refusal that can be made from the source
// files alone is made BEFORE anything is copied, so the ordinary mistakes
// (a directory, a name no map could spell, a stem already installed) never touch
// the campaign at all. Only the checks that depend on art/ AS IT WILL BE — a
// sidecar whose picture is already installed, or is arriving in the same batch —
// have to run after, and those are the ones the undo below exists for.
func runArtInstall(cmd *cobra.Command, campaignPath string, force bool, sources []string) error {
	// Read before writing: a campaign.json that cannot be read is the
	// operator's to fix, and hearing it now beats hearing it when `vtt serve`
	// refuses to boot.
	cfg, err := campaigncfg.Load(campaignPath)
	if err != nil {
		return fmt.Errorf("vtt art install: %w", err)
	}

	// 0o750 / 0o600 below, matching cmd/vtt's own convention (client_soak.go,
	// harness_boot.go) and gosec's G301/G302: a campaign's art is read by the
	// server process running as the same user, and nothing else needs it.
	artDir := filepath.Join(campaignPath, "art")
	if err := os.MkdirAll(artDir, 0o750); err != nil {
		return fmt.Errorf("vtt art install: create %s: %w", artDir, err)
	}

	// --- everything that can refuse before a byte is written -----------------
	stems := map[string]bool{}
	// THE FILESYSTEM IS THE UNIQUENESS RULE (design spec §3.3) and it holds
	// inside art/ — but it cannot see two SOURCES sharing one basename, because
	// at pre-flight neither is installed yet and the overwrite check below asks
	// only about the destination. Left alone, the copy loop would write one over
	// the other and the last one would silently win. --force does not answer it
	// either: that flag means "replace what is installed", which says nothing
	// about which of two incoming files you meant.
	incoming := map[string]bool{}
	for _, src := range sources {
		name := filepath.Base(src)
		if incoming[name] {
			return fmt.Errorf(
				"vtt art install: two of the files named on this command line are called %q, and "+
					"art/ can hold one of them. Rename one — two pieces cannot share a name "+
					"(design spec §3.3)", name)
		}
		incoming[name] = true
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("vtt art install: %w", err)
		}
		if info.IsDir() {
			return fmt.Errorf(
				"vtt art install: %s is a directory; art/ is flat — a subfolder is a namespace, "+
					"and a namespace is a pack (design spec §3.1). Install the files inside it "+
					"individually, renaming any that would collide", src)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf(
				"vtt art install: %s is not a regular file; art/ holds files, and a link is a "+
					"second name for one piece (design spec §3.3)", src)
		}
		if !artlib.IsArtFileName(name) {
			return fmt.Errorf(
				"vtt art install: %q is not an art filename: a kebab-case stem — lowercase letters "+
					"and digits joined by single hyphens — and a lowercase .png or .json, so the stem "+
					"IS the id a map names (design spec §3.2)", name)
		}
		dest := filepath.Join(artDir, name)
		if _, err := os.Lstat(dest); err == nil && !force {
			return fmt.Errorf(
				"vtt art install: %s is already installed; art/%s would be replaced. Pass --force to "+
					"replace it, or rename the incoming piece — two pieces cannot share a name "+
					"(design spec §3.3)", name, name)
		}
		stems[strings.TrimSuffix(strings.TrimSuffix(name, ".png"), ".json")] = true
	}

	// --- copy, remembering enough to undo ------------------------------------
	var done []installed
	undo := func() {
		// Reverse order, so a destination that was both backed up and written
		// is restored after its replacement is removed.
		//
		// REMOVE ONLY WHAT THIS RUN WROTE. The two fields answer two questions
		// and both are asked: `wrote` says the file at dest is ours to delete,
		// `backup` says something of the DM's was moved aside and has to come
		// back. A record can carry neither — see installed.wrote.
		for i := len(done) - 1; i >= 0; i-- {
			if done[i].wrote {
				_ = os.Remove(done[i].dest)
			}
			if done[i].backup != "" {
				_ = os.Rename(done[i].backup, done[i].dest)
			}
		}
	}
	for _, src := range sources {
		rec, err := installOne(src, artDir)
		if err != nil {
			done = append(done, rec) // whatever it managed, so undo sees it
			undo()
			return fmt.Errorf("vtt art install: %w", err)
		}
		done = append(done, rec)
	}

	// --- the same resolution a map load performs ------------------------------
	//
	// Run against art/ AS IT NOW IS, which is the only honest place to run it:
	// a sidecar may name pictures already installed, or arriving beside it in
	// this same batch, and a check over the source files alone would refuse
	// both.
	for stem := range stems {
		if _, err := artlib.Lookup(artDir, stem); err != nil {
			undo()
			return fmt.Errorf(
				"vtt art install: %w — refused here rather than at the table, and art/ is unchanged", err)
		}
	}

	// NOTHING REFUSED, so the set-aside originals are litter and go. artlib
	// ignores them — a ".vtt-replaced" suffix is neither a picture nor a sidecar,
	// so Validate passes over it exactly as it passes over a .DS_Store — which is
	// what would make one accumulate per --force, unremarked, in a directory
	// whose whole design is that a human can read it (§9).
	for _, rec := range done {
		if rec.backup != "" {
			_ = os.Remove(rec.backup)
		}
	}

	out := cmd.OutOrStdout()
	for _, rec := range done {
		fmt.Fprintf(out, "installed art/%s\n", filepath.Base(rec.dest))
		warnOffGrid(out, rec.dest, cfg.CellPx)
	}
	return nil
}

// installOne copies src into artDir under its own name, via a temporary file
// and a rename so a partial copy is never visible under the real name — the
// same reason a map load may happen at any moment: art resolves on demand
// (design spec §3.6), so there is no quiet window in which a half-written file
// is not being read.
//
// It returns what it did even on failure, so the caller's undo can clean up a
// backup this call had already moved aside — and the record says what it DID,
// never merely what it was aiming at: a set-aside that fails leaves the
// destination untouched and the record records no write, because the undo would
// otherwise delete art this command never replaced.
func installOne(src, artDir string) (installed, error) {
	name := filepath.Base(src)
	dest := filepath.Join(artDir, name)
	rec := installed{dest: dest}

	if _, err := os.Lstat(dest); err == nil {
		// Only reachable with --force (the pre-flight refused it otherwise).
		// Moved rather than deleted: a batch that fails validation must be able
		// to put the working art back, and a --force that destroyed the file it
		// was replacing and THEN refused would be the worst of both answers.
		//
		// backup IS RECORDED ONLY ONCE THE RENAME HAS HAPPENED. Set before it,
		// a failed set-aside would hand the undo a path to restore FROM that
		// nothing was ever moved to.
		backup := dest + ".vtt-replaced"
		if err := os.Rename(dest, backup); err != nil {
			// The DM's art is still at dest, exactly as it was: this record
			// says so by carrying neither a backup nor a write.
			return rec, fmt.Errorf("set aside %s: %w", name, err)
		}
		rec.backup = backup
	}

	tmp := dest + ".vtt-installing"
	if err := copyFile(src, tmp); err != nil {
		_ = os.Remove(tmp)
		return rec, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return rec, fmt.Errorf("install %s: %w", name, err)
	}
	// THE ONE PLACE wrote BECOMES TRUE: dest now holds bytes this run put there,
	// and only now may an undo remove it.
	rec.wrote = true
	return rec, nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// warnOffGrid is cell_px's FIRST AND ONLY READER, and the answer to what the
// number does now that art can be any size.
//
// NOTHING IN THE RENDERER READS IT AND NOTHING SHOULD. client/src/view/
// spectator.ts's CELL = 44 is a SCREEN size — how large a square is drawn —
// while cell_px is a SOURCE size, how large the picture is; drawImage scales one
// to the other whatever they are, so a picture at any resolution draws correctly
// today. A comment once claimed the renderer read cell_px "to draw at the right
// scale" and it was false (corrected in review, 2026-09-04): wiring it into a
// draw call would change pixels for no requirement.
//
// WHAT THE NUMBER ACTUALLY ASSERTS is design spec §6's own sentence: "art pieces
// at differing native resolutions on the same board is a rendering problem, not
// a capability." A campaign whose grid is 64 and which then installs a 50x50 wall
// has that problem, and this is where somebody can still act on it.
//
// IT IS CHECKED AGAINST THE CAMPAIGN'S DEFAULT, and it cannot be anything else.
// Since 2026-09-05 a MAP may declare its own cell_px and a campaign may hold
// several drawn at different resolutions (mapdef.Map.CellPx, Patrik's ruling
// from MapTool's per-Zone grid size) — and this command is handed FILES, not a
// map, so it has no way to know which map a piece is destined for. The default
// is still the right thing to measure against: it is what a map draws at unless
// it says otherwise, which is every map that exists. The warning says which
// number it used, so a DM installing a 128 set for a map that declares 128 can
// read the note and ignore it.
//
// A WARNING, NEVER A REFUSAL. A lower-resolution picture draws perfectly well
// and the DM may mean it, so this is the same posture §4 takes about everything
// else art can get wrong — and it keeps the number advisory rather than turning
// campaign.json into a gate on files it was never meant to police.
//
// A WHOLE MULTIPLE RATHER THAN EQUALITY, because an object may be more than one
// square (mapdef's Object.size): a 2x1 brazier at 128x64 is exactly right, and
// demanding 64x64 of it would warn on correct art.
//
// A PICTURE THAT WILL NOT DECODE IS NOT AN ERROR HERE. Nothing else in this tree
// reads a picture's content at all — artlib.statPicture only stats — so
// refusing would invent a content rule design spec §3 does not have. The
// operator is told the grid could not be checked, and the file installs.
func warnOffGrid(out io.Writer, dest string, cellPx int32) {
	if filepath.Ext(dest) != ".png" {
		return
	}
	name := filepath.Base(dest)
	f, err := os.Open(dest)
	if err != nil {
		return
	}
	defer f.Close()
	cfgImg, _, err := image.DecodeConfig(f)
	if err != nil {
		fmt.Fprintf(out, "  note: %s could not be read as a PNG, so it was not checked against "+
			"this campaign's cell_px; it is installed either way\n", name)
		return
	}
	px := int(cellPx)
	if cfgImg.Width%px == 0 && cfgImg.Height%px == 0 {
		return
	}
	fmt.Fprintf(out, "  note: %s is %dx%d, which is not a whole number of %d-pixel squares "+
		"(this campaign's default cell_px). It will still draw — the renderer scales it — but a "+
		"board mixing resolutions is what cell_px exists to make visible. A map drawn at another "+
		"resolution declares its own cell_px, and this note does not know which map you meant\n",
		name, cfgImg.Width, cfgImg.Height, px)
}
