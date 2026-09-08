package campaigncfg_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaigncfg"
)

// write drops raw at <dir>/campaign.json. Raw rather than a struct, because
// every interesting case here is a file a HUMAN wrote by hand — a missing
// brace, a forgotten format_version, a number where a number does not belong —
// and none of those can be expressed by marshalling a valid Go value.
func write(t *testing.T, dir, raw string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "campaign.json"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAnAbsentCampaignJSONGivesTheDefaultCellPx is the plan's own RED test
// (docs/superpowers/plans/2026-09-02-art-is-a-flat-library.md, Task 6 Step 1)
// and the criterion design spec §6 states in one line: "The file is optional
// and cell_px defaults to 64 when it is absent, so an existing campaign keeps
// working untouched and a new one need not create it."
//
// Every campaign in this repo is in exactly this state, so this is not an edge
// case — it is the ONLY state any campaign is in until somebody writes the
// file, and a Load that erred here would stop every one of them booting.
func TestAnAbsentCampaignJSONGivesTheDefaultCellPx(t *testing.T) {
	cfg, err := campaigncfg.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v — campaign.json is optional", err)
	}
	if cfg.CellPx != 64 {
		t.Fatalf("CellPx = %d, want the documented default 64", cfg.CellPx)
	}
}

// TestAnAbsentCampaignDirectoryIsTheSameAsAnAbsentFile pins the arm that is
// one os.ReadFile error apart from the one above and answers differently if
// the check is written as os.Stat on the FILE rather than on the error: a
// campaign path that does not exist at all. `vtt art install --campaign` may
// be the first command ever run against a path, exactly as `vtt invite` is
// (cmd/vtt/invite.go's own doc comment), so this is a real ordering rather
// than a defensive branch.
func TestAnAbsentCampaignDirectoryIsTheSameAsAnAbsentFile(t *testing.T) {
	cfg, err := campaigncfg.Load(filepath.Join(t.TempDir(), "not-created-yet"))
	if err != nil {
		t.Fatalf("Load: %v — a campaign that does not exist yet has no settings, which is not a failure", err)
	}
	if cfg.CellPx != campaigncfg.DefaultCellPx {
		t.Fatalf("CellPx = %d, want the documented default %d", cfg.CellPx, campaigncfg.DefaultCellPx)
	}
}

// TestADeclaredCellPxIsRead is the whole point of the file existing: a
// campaign that authors its art at a different resolution says so once, in one
// number (design spec §6 — "a grid is uniform... One number per campaign says
// that plainly").
func TestADeclaredCellPxIsRead(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"format_version":1,"cell_px":32}`)
	cfg, err := campaigncfg.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CellPx != 32 {
		t.Fatalf("CellPx = %d, want 32 — the declared number, not the default", cfg.CellPx)
	}
}

// TestAFileDeclaringNoCellPxStillGetsTheDefault pins the arm a plain int32
// field gets wrong for the reason artlib.declaredFormat records: a Go zero
// value cannot carry PRESENCE, so {"format_version":1} with no cell_px would
// decode to 0 and a campaign that wrote the file to say nothing about cell
// size would be told its grid is zero pixels wide.
func TestAFileDeclaringNoCellPxStillGetsTheDefault(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"format_version":1}`)
	cfg, err := campaigncfg.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CellPx != campaigncfg.DefaultCellPx {
		t.Fatalf("CellPx = %d, want the default %d — an omitted field is not a declaration of zero",
			cfg.CellPx, campaigncfg.DefaultCellPx)
	}
}

// TestAnUnsupportedFormatVersionIsRefusedAndNamesBothVersions mirrors
// artlib.ErrFormatVersion's ruling one directory up (Patrik, 2026-09-04): a
// declared version ABOVE this server's means the content is newer than the
// server, and the remedy is a newer server. The message has to carry both
// numbers or it cannot say which half is behind.
//
// 99 IS THE ONLY DIRECTION THIS TEST COVERS, which is why it is not the whole
// story: TestAVersionBelowThisServersIsNotCalledNewerThanTheServer holds the
// other one, and the single arm that used to serve both told an operator with a
// typo to go and get a newer server.
func TestAnUnsupportedFormatVersionIsRefusedAndNamesBothVersions(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"format_version":99,"cell_px":64}`)
	_, err := campaigncfg.Load(dir)
	if err == nil {
		t.Fatal("Load succeeded on format_version 99; a campaign written for a later server must say so")
	}
	if !errors.Is(err, campaigncfg.ErrFormatVersion) {
		t.Errorf("err = %v, want it to answer errors.Is(ErrFormatVersion) — the caller branches on the "+
			"sentinel, never on message text", err)
	}
	for _, want := range []string{"99", "1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to name version %s", err, want)
		}
	}
}

// TestABrokenCampaignJSONIsRefusedAtBootAndNamesTheFile is the one place this
// package deliberately does NOT follow art's degrade ruling, and the reason is
// the reader rather than the file. Every art degrade exists because a DM at a
// browser cannot act on a filesystem path mid-session (design spec §4). This
// file has exactly one reader — composeServer, at boot, with an operator at a
// terminal — and the same posture a broken MAP already gets there. Silently
// falling back to 64 would leave a campaign that DECLARED 32 drawing at 64
// with nothing said.
func TestABrokenCampaignJSONIsRefusedAtBootAndNamesTheFile(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"a missing brace", `{"format_version":1,"cell_px":64`},
		{"a version that is not a number", `{"format_version":"1","cell_px":64}`},
		{"no format_version at all", `{"cell_px":64}`},
		{"an unknown field", `{"format_version":1,"cell_px":64,"cell_size":32}`},
		{"a cell size that is not a number", `{"format_version":1,"cell_px":"64"}`},
		{"a fractional cell size", `{"format_version":1,"cell_px":63.5}`},
		{"a JSON array", `[]`},
		{"an empty file", ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, tc.raw)
			if _, err := campaigncfg.Load(dir); err == nil {
				t.Fatalf("Load succeeded on %s; a campaign.json that is present and unreadable "+
					"must be named where the operator can fix it", tc.name)
			} else if !strings.Contains(err.Error(), "campaign.json") {
				t.Errorf("err = %q, want it to name campaign.json — the operator is holding the file", err)
			}
		})
	}
}

// TestACellPxOutsideTheBoundsIsRefused pins the value check, widened on
// 2026-09-05 from "must be positive" to MapTool's bounded shape: 0 and 100000
// are not smaller and larger squares, they are a file that cannot mean what it
// says. Left unchecked, {"cell_px":0} would be indistinguishable on the wire
// from a server that never read the file.
//
// REFUSED, NEVER CLAMPED INTO RANGE — see mapdef.MinCellPx's own doc comment for
// the argument, which is the same one on both sides of the pair.
func TestACellPxOutsideTheBoundsIsRefused(t *testing.T) {
	for _, raw := range []string{
		`{"format_version":1,"cell_px":0}`,
		`{"format_version":1,"cell_px":-64}`,
		`{"format_version":1,"cell_px":1}`,
		`{"format_version":1,"cell_px":100000}`,
	} {
		dir := t.TempDir()
		write(t, dir, raw)
		_, err := campaigncfg.Load(dir)
		if err == nil {
			t.Errorf("Load succeeded on %s; a grid square is between %d and %d pixels",
				raw, campaigncfg.MinCellPx, campaigncfg.MaxCellPx)
			continue
		}
		for _, want := range []string{fmt.Sprint(campaigncfg.MinCellPx), fmt.Sprint(campaigncfg.MaxCellPx)} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("on %s: err = %q, want it to name the bound %q — an author who got one "+
					"wrong cannot tell from a message that names only the one they crossed",
					raw, err, want)
			}
		}
	}
}

// TestTheBoundsThemselvesAreAccepted pins the edges as INCLUSIVE, which a
// `<`/`<=` slip changes without any other test noticing, and pins that the
// documented default sits inside them — otherwise every campaign that declares
// nothing holds a number its own campaign.json could not declare.
func TestTheBoundsThemselvesAreAccepted(t *testing.T) {
	for _, v := range []int32{campaigncfg.MinCellPx, campaigncfg.MaxCellPx, campaigncfg.DefaultCellPx} {
		dir := t.TempDir()
		write(t, dir, fmt.Sprintf(`{"format_version":1,"cell_px":%d}`, v))
		cfg, err := campaigncfg.Load(dir)
		if err != nil {
			t.Fatalf("cell_px %d was refused: %v", v, err)
		}
		if cfg.CellPx != v {
			t.Errorf("CellPx = %d, want %d", cfg.CellPx, v)
		}
	}
}

// TestAnUnreadableCampaignJSONIsRefused covers the read failure that is not a
// parse failure — a directory sitting where the file belongs, which is what a
// mistaken `mkdir campaign.json` leaves behind. Without this arm an
// os.ReadFile error other than fs.ErrNotExist could be folded into "absent"
// and the campaign would boot on defaults it never declared.
func TestAnUnreadableCampaignJSONIsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "campaign.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := campaigncfg.Load(dir); err == nil {
		t.Fatal("Load succeeded with a DIRECTORY named campaign.json; " +
			"unreadable is not the same fact as absent")
	}
}

// TestTheDefaultIsTheOneWrittenInTheSpec is a guard against the two halves
// drifting: the constant is what an absent file resolves to, and 64 is the
// number design spec §6 writes down. A test asserting Load's answer against
// the constant alone would pass with both changed together.
func TestTheDefaultIsTheOneWrittenInTheSpec(t *testing.T) {
	if campaigncfg.DefaultCellPx != 64 {
		t.Fatalf("DefaultCellPx = %d, want 64 (design spec §6)", campaigncfg.DefaultCellPx)
	}
	if campaigncfg.FormatVersion != 1 {
		t.Fatalf("FormatVersion = %d, want 1 (design spec §6's own example file)", campaigncfg.FormatVersion)
	}
}

// TestAVersionNOBODYWROTEIsNeverReportedAsOneIsArtlib's lesson arriving one
// directory over, and the hole was the exact one artlib.pieceFromSidecar's own
// comment describes: "JSON null unmarshals into any pointer as nil without
// erroring, so a plain int32 here would read {"format_version": null} as 0 and
// refuse it as 'declares 0' — a version the author never wrote."
//
// Load decoded doc.FormatVersion into an int32 until 2026-09-05 (review finding
// F2), so an explicit null left the zero value in place and the operator was
// told their file declared format_version 0 and that it was "newer than the
// server". Two false sentences about a file that says nothing of the kind.
//
// NO SENTINEL, and that is artlib's ruling too: a value that is not a version
// AT ALL is a broken file, and a broken file does not assert that its content
// is newer than this server. Only a DECLARED number this server does not
// understand does that.
func TestAVersionNobodyWroteIsNeverReportedAsOne(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"format_version":null,"cell_px":64}`)
	_, err := campaigncfg.Load(dir)
	if err == nil {
		t.Fatal("Load succeeded on an explicit null format_version; the file declares no format")
	}
	if strings.Contains(err.Error(), "declares") {
		t.Errorf("err = %q — it reports a version the author never wrote. null is the ABSENCE of a "+
			"number, and the message must not put one in the operator's mouth", err)
	}
	if errors.Is(err, campaigncfg.ErrFormatVersion) {
		t.Errorf("err = %v answers errors.Is(ErrFormatVersion), which asserts the file was written "+
			"for a later server. A null declares nothing, so it asserts no such thing", err)
	}
	if !strings.Contains(err.Error(), "campaign.json") {
		t.Errorf("err = %q, want it to name campaign.json — the operator is holding the file", err)
	}
}

// TestAVersionBelowThisServersIsNotCalledNewerThanTheServer is the second half
// of the same finding, and the one an operator meets by typing rather than by
// hand-writing null: `{"format_version":0}` is a plausible typo, and the single
// `declared != FormatVersion` arm that shipped here told them "the file is newer
// than the server, not broken" — advice to go and fetch a newer server for a
// file no server has ever written.
//
// STILL ErrFormatVersion, because the sentinel means what artlib's does: a
// declared format this server does not understand. What changes with the
// direction is the MESSAGE, because only one direction has "get a newer server"
// as its remedy.
func TestAVersionBelowThisServersIsNotCalledNewerThanTheServer(t *testing.T) {
	for _, raw := range []string{`{"format_version":0}`, `{"format_version":-1,"cell_px":64}`} {
		dir := t.TempDir()
		write(t, dir, raw)
		_, err := campaigncfg.Load(dir)
		if err == nil {
			t.Fatalf("Load succeeded on %s; %d is the only format this server understands",
				raw, campaigncfg.FormatVersion)
		}
		if strings.Contains(err.Error(), "newer") {
			t.Errorf("on %s: err = %q — a version BELOW this server's is not a file from the "+
				"future, and telling the operator to upgrade sends them away from the typo they "+
				"are holding", raw, err)
		}
		if !errors.Is(err, campaigncfg.ErrFormatVersion) {
			t.Errorf("on %s: err = %v, want the sentinel — this is a declared format this server "+
				"does not understand, which is what the sentinel means", raw, err)
		}
		if !strings.Contains(err.Error(), fmt.Sprint(campaigncfg.FormatVersion)) {
			t.Errorf("on %s: err = %q, want it to name the version this server does understand",
				raw, err)
		}
	}
}

// TestACellPxOfNullIsNotReportedAsZero closes the same hole one field over,
// found while fixing the one above. {"cell_px":null} left the zero value in
// place and was refused as "declares cell_px 0; one grid square is between 8
// and 1024" — a number the author never wrote, and a bound they never crossed.
//
// REFUSED RATHER THAN DEFAULTED, like every other thing this file can get
// wrong: an absent field takes the default (TestAFileDeclaringNoCellPxStillGets
// TheDefault), and a field that is PRESENT and unreadable is a file the
// operator should look at.
func TestACellPxOfNullIsNotReportedAsZero(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, `{"format_version":1,"cell_px":null}`)
	_, err := campaigncfg.Load(dir)
	if err == nil {
		t.Fatal("Load succeeded on an explicit null cell_px; the field is present and says nothing")
	}
	if strings.Contains(err.Error(), "declares cell_px") {
		t.Errorf("err = %q — null is not a declaration of zero, and a bound the author never "+
			"crossed is the wrong thing to send them to look at", err)
	}
	if !strings.Contains(err.Error(), "cell_px") {
		t.Errorf("err = %q, want it to name the field", err)
	}
}
