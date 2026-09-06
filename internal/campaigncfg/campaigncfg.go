// Package campaigncfg reads campaign.json — the one settings file a campaign
// directory may hold, and today the one number in it.
//
// WHY THE FILE EXISTS AT ALL. cell_px was a PACK field: how many pixels one
// grid square of art occupies. 2026-09-02-art-is-a-flat-library deletes the
// pack, and design spec §6 rehomes the number to the campaign rather than to
// each art piece, on the reasoning that "a grid is uniform, and art pieces at
// differing native resolutions on the same board is a rendering problem, not a
// capability. One number per campaign says that plainly."
//
// THE FILE IS OPTIONAL AND THAT IS LOAD-BEARING, not a convenience. Every
// campaign that exists today has no campaign.json, so a Load that treated
// absence as a failure would stop all of them booting; §6 states the default in
// the same sentence it states the file. Absence is the ordinary case, and it is
// the case with no I/O in it.
//
// THIS PACKAGE IS STRICT WHERE artlib IS LENIENT, and the difference is the
// READER rather than the file. Every art degrade in this sub-project exists
// because a DM sitting at a browser cannot act on a filesystem path mid-session
// (design spec §4), so a broken sidecar costs one square instead of the server.
// campaign.json has exactly one reader — cmd/vtt's composeServer, at boot, with
// an operator at a terminal — and no request-time reader at all, so there is
// nobody it could degrade FOR. The posture is a broken map's: refuse, and name
// the file. Falling back to the default would leave a campaign that DECLARED 32
// being served 64 with nothing said, which is the silence spec §4's warning
// channel exists to prevent, arrived at from the other direction.
package campaigncfg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FileName is campaign.json, named once so the loader and every error that
// mentions it cannot disagree about the spelling.
const FileName = "campaign.json"

// FormatVersion is the campaign.json format this server understands, on its own
// clock like every other format_version in this tree (design spec §3.4's
// reasoning for art applies here unchanged: the file is authored outside the
// platform and a file written against a later format must say so itself).
const FormatVersion int32 = 1

// DefaultCellPx is what cell_px means when campaign.json is absent, or is
// present and declares no cell_px: 64, the number design spec §6 writes into
// its own example file.
//
// THE GATEWAY CARRIES ITS OWN COPY of this number (gateway.DefaultCellPx), and
// deliberately: internal/gateway reads no files and must not gain a dependency
// on a package that does. cmd/vtt imports both and asserts they agree
// (TestTheServerDefaultAndTheCampaignDefaultAreTheSameNumber), which is the one
// place that can see both.
const DefaultCellPx int32 = 64

// MinCellPx and MaxCellPx bound cell_px, here and in a map file alike.
//
// DUPLICATED FROM mapdef.MinCellPx/MaxCellPx, WHICH CARRY THE REASONING —
// including why these numbers are 8..1024 where MapTool's per-Zone equivalent is
// 9..350, which is a deliberate divergence rather than a typo. Read that comment
// before changing either number here.
//
// The duplication is not laziness: this package is self-only by design (a
// settings reader has no business knowing the map format), so cmd/vtt imports
// both and asserts they agree
// (TestTheCellPxConstantsAgreeAcrossThePackagesThatCarryThem). A value outside
// them is REFUSED rather than clamped into range.
const (
	MinCellPx int32 = 8
	MaxCellPx int32 = 1024
)

// ErrFormatVersion marks the campaign.json failure that is a fact about the
// FORMAT rather than about the syntax: the file declares a format_version this
// server does not understand. Exactly what artlib.ErrFormatVersion means one
// directory up, and its MESSAGE is neutral about which side is behind for the
// same reason ours is.
//
// THE TWO SENTINELS NOW COVER DIFFERENT SETS, and a caller must not read one
// through the other. artlib's fires only for a version LATER than that server
// understands: art-is-a-flat-library Task 8 moved a typo'd 0 out of it, because
// there the sentinel decides refuse-versus-degrade and a refusal costs the map
// and the boot. Ours still fires in both directions, because campaign.json has
// no request-time reader to degrade for — every arm below refuses, and the
// direction changes only the sentence. This paragraph said artlib's SENTINEL
// DOC claimed the upgrade-your-server remedy for a typo'd 0; that was true when
// it was written on 2026-09-05 and Task 8 fixed the sentinel rather than the
// doc.
//
// IT SAID "so the content is NEWER THAN THIS SERVER and the remedy is a newer
// server" until 2026-09-05, and that was true of the only case anyone had
// tested (99) and false of the one an operator actually types: a typo'd
// `"format_version": 0` is not a file from the future, and no server has ever
// written one (review finding F2). The DIRECTION now lives in Load's two arms,
// where it can be said only when it is true; the sentinel stays one thing, so a
// caller cannot end up branching on half a question.
//
// A caller that wants to tell "your file is broken" from "this format is not
// one I know" must branch on identity, never on message text. Nothing branches
// on it today — composeServer refuses on either — so it is here because the
// message a DM or an operator reads is the only difference, and that difference
// has to survive a rewording.
var ErrFormatVersion = errors.New("campaigncfg: unsupported campaign.json format")

// Config is campaign.json, decoded and defaulted.
//
// CellPx is always positive on any Config a nil error accompanies: absent file,
// absent field and declared value all resolve here, and a declared zero or
// negative is refused rather than passed on. A caller therefore never has to
// ask whether the number it holds was defaulted, which is the whole reason the
// defaulting happens in one place.
type Config struct {
	CellPx int32
}

// configJSON is campaign.json's on-disk shape.
//
// json.RawMessage for both fields, for the reason artlib.declaredFormat
// records: a Go zero value cannot carry PRESENCE. A plain int32 cannot tell an
// absent cell_px from an explicit {"cell_px": 0}, so "an omitted field takes the
// default" and "zero pixels is refused" would be one branch pretending to be
// two — and a *int32 HERE would move the hole rather than close it, since JSON
// null unmarshals into a pointer as nil and would then be indistinguishable
// from an absent field. Raw bytes decide nothing until Load decides.
//
// LOAD ITSELF DOES USE A POINTER, one level down, and the two are not
// alternatives: these bytes answer "was the field written at all", and
// unmarshalling them into a *int32 answers "was a number written in it". This
// comment read as an argument against the pointer outright until 2026-09-05,
// when Load was decoding both fields into plain int32s and reporting `null` as
// a declared zero — the very defect the first half of this paragraph is about,
// through the door the second half left open.
type configJSON struct {
	FormatVersion json.RawMessage `json:"format_version"`
	CellPx        json.RawMessage `json:"cell_px"`
}

// Load reads <campaignPath>/campaign.json and returns the campaign's settings.
//
// AN ABSENT FILE IS NOT AN ERROR and returns the documented defaults; so does
// an absent campaign DIRECTORY, because `vtt art install --campaign` may be the
// first command ever run against a path, exactly as `vtt invite` may be
// (cmd/vtt/invite.go). Both arrive as fs.ErrNotExist from one os.ReadFile, so
// there is one branch rather than a stat that could disagree with the read that
// follows it.
//
// EVERY OTHER FAILURE IS REFUSED AND NAMES THE FILE — see the package doc for
// why this is strict where artlib degrades. "Names the file" means the path, on
// purpose: the only caller is a boot check with an operator at a terminal, the
// same exemption artlib.Validate states for itself. If this ever answers a
// request, that line becomes a leak.
func Load(campaignPath string) (Config, error) {
	path := filepath.Join(campaignPath, FileName)
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{CellPx: DefaultCellPx}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("campaigncfg: read %s: %w", path, err)
	}

	// DisallowUnknownFields, so a typo is a refusal rather than a setting that
	// silently does nothing. That is only affordable because format_version is
	// required below: a campaign written for a later server, carrying a field
	// this one has never heard of, is caught by the version rather than by the
	// unknown field, and gets the message that names the right remedy.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var doc configJSON
	if err := dec.Decode(&doc); err != nil {
		return Config{}, fmt.Errorf("campaigncfg: %s: %w", path, err)
	}

	// THE VERSION IS READ FIRST AND ON ITS OWN, so "this server is too old" can
	// never be reported as "your file is broken". Everything after this line
	// assumes the file was written for a format this server knows.
	//
	// A POINTER, so an explicit null is told apart from a number — the reason
	// artlib.pieceFromSidecar's own comment states, which this package had not
	// followed: "JSON null unmarshals into any pointer as nil without erroring,
	// so a plain int32 here would read {"format_version": null} as 0 and refuse
	// it as 'declares 0' — a version the author never wrote." Load decoded
	// straight into an int32 until 2026-09-05 and did exactly that (review
	// finding F2).
	//
	// THE RawMessage FIELD IS STILL WHAT CARRIES PRESENCE and the pointer is not
	// a substitute for it: an ABSENT field is a nil RawMessage, which arrives
	// here as "unexpected end of JSON input" rather than as a nil pointer. Two
	// halves of one question, and each answers a case the other cannot.
	var version *int32
	if err := json.Unmarshal(doc.FormatVersion, &version); err != nil {
		// Covers an absent field (RawMessage nil, "unexpected end of JSON
		// input") and one holding something that is not a version number, which
		// are the same fact about the file: somebody hand-wrote it and left the
		// declaration out or got it wrong. Neither asserts that the content is
		// newer, so neither gets ErrFormatVersion.
		return Config{}, fmt.Errorf(
			"campaigncfg: %s: format_version must be a whole number, and %s must declare one "+
				"(design spec §6): %w", path, FileName, err)
	}
	if version == nil {
		// NULL IS THE ABSENCE OF A NUMBER, not the number zero, and no sentinel
		// for the same reason as above: a file that declares nothing does not
		// assert that it was written for a later server.
		return Config{}, fmt.Errorf(
			"campaigncfg: %s: format_version is null; %s must declare the format it was written "+
				"for as a whole number (design spec §6)", path, FileName)
	}
	declared := *version
	// TWO ARMS RATHER THAN ONE MESSAGE WITH A CONDITIONAL CLAUSE. A single
	// `declared != FormatVersion` arm shipped here and told an operator who had
	// typed `"format_version": 0` that "the file is newer than the server, not
	// broken" — go and fetch a newer server for a file no server has ever
	// written. Only one direction has that remedy, so only one direction says
	// it, and the split is also what makes both boundaries killable: at
	// declared == FormatVersion each arm's `>=`/`<=` mutant refuses a file this
	// server understands, and TestADeclaredCellPxIsRead says so.
	//
	// BOTH KEEP ErrFormatVersion, because the sentinel means what
	// artlib.unsupportedFormat's does — a declared format this server does not
	// understand — and artlib's message is neutral for the same reason. What
	// varies with the direction is the sentence an operator reads.
	if declared > FormatVersion {
		return Config{}, fmt.Errorf(
			"campaigncfg: %s declares format_version %d; this server understands %d — "+
				"the file is newer than the server, not broken: %w",
			path, declared, FormatVersion, ErrFormatVersion)
	}
	if declared < FormatVersion {
		return Config{}, fmt.Errorf(
			"campaigncfg: %s declares format_version %d; this server understands %d: %w",
			path, declared, FormatVersion, ErrFormatVersion)
	}

	cfg := Config{CellPx: DefaultCellPx}
	if doc.CellPx == nil {
		// A file that says nothing about cell size is not a file declaring
		// zero. Only presence changes the answer.
		return cfg, nil
	}
	// A POINTER HERE TOO, and for the same reason one line of JSON away:
	// {"cell_px": null} decoded into an int32 was refused as "declares cell_px
	// 0; one grid square is between 8 and 1024 pixels" — a number the author
	// never wrote and a bound they never crossed, which sends them to look at
	// the wrong thing (found while fixing the version above, 2026-09-05).
	var declaredPx *int32
	if err := json.Unmarshal(doc.CellPx, &declaredPx); err != nil {
		return Config{}, fmt.Errorf("campaigncfg: %s: cell_px must be a whole number: %w", path, err)
	}
	if declaredPx == nil {
		// PRESENT AND SAYING NOTHING IS NOT ABSENT. An omitted cell_px takes the
		// documented default (above); a field written as null is a file the
		// operator should look at, like everything else here that is present and
		// unreadable.
		return Config{}, fmt.Errorf(
			"campaigncfg: %s: cell_px is null; a whole number of pixels, or leave the field out "+
				"to use the documented default %d", path, DefaultCellPx)
	}
	cellPx := *declaredPx
	if cellPx < MinCellPx || cellPx > MaxCellPx {
		return Config{}, fmt.Errorf(
			"campaigncfg: %s declares cell_px %d; one grid square of art is between %d and %d pixels. "+
				"Leave the field out to use the documented default %d",
			path, cellPx, MinCellPx, MaxCellPx, DefaultCellPx)
	}
	cfg.CellPx = cellPx
	return cfg, nil
}
