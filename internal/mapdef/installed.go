package mapdef

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPackNotLoaded is gone, and its absence is the point. It marked the one
// failure whose remedy depended on WHO was asking — a map naming a pack the
// caller had not handed over, which at boot meant "not installed" and at
// request time could also mean "installed since we started, and packs are
// read once". Nothing can produce that failure any more: art is read from a
// directory at load time, per art-is-a-flat-library design spec §3.6, so
// there is no boot-time art load for a piece to arrive after. Its two
// callers' handling went with it (internal/gateway's mapByID added the
// restart sentence; this function named the pack).

// LoadInstalled loads and fully validates the ONE map installed as
// <mapsDir>/<id>.json: the file is read, every check Load makes runs, the
// filename and the map's own declared id must agree, and the map is dry-run
// compiled against artDir, the campaign's flat art/ directory. The returned
// *Map has passed everything a map must pass before it can be put in front
// of a table.
//
// A map whose art is not installed PASSES (art-is-a-flat-library design spec
// §4): the dry run degrades those squares and warns, and this function
// discards the warnings because it answers with a *Map rather than with a
// load result — the live Compile call the caller makes next returns the same
// warnings, computed against the same directory, and that is the one that
// reaches whoever asked. Art that exists and CANNOT BE READ degrades the same
// way as of Patrik's ruling of 2026-09-04; the one art failure still fatal
// here is a sidecar declaring a format_version LATER than this server
// understands (see Resolve's own doc comment for why those two go opposite
// ways; a version BELOW it is a typo and degrades). An empty
// artDir is legal and means no art resolves.
//
// This function exists because there are two ways into a campaign's maps
// and they must not disagree. cmd/vtt's loadMapsDir calls it once per file
// at boot; internal/gateway's mapByID calls it on a lookup miss, while the
// server is running (2026-09-01-create-scene-leaves design spec §5,
// "Lookup on demand"). The design spec names the alternative as a hazard in
// its own right (§12): "Boot validation and on-demand validation can
// diverge. Two code paths reach mapdef and they must agree, or a map that
// boots cleanly could be refused on reload. They should share one function
// rather than two similar ones." Every rule about what makes an installed
// map loadable therefore lives HERE, not in either caller.
//
// EVERY ERROR NAMES THE FILE AS "maps/<id>.json", never as the path this
// process actually opened. mapsDir is an absolute path on the server, and
// these errors reach a client verbatim: mapByID forwards all of them but
// the not-installed one, so any absolute path in any of them is disclosed
// to every seat that can issue load_map. That was a real leak, found in
// review of 2026-09-01-create-scene-leaves Task 6, round 1 — an id of 260
// characters returns
// ENAMETOOLONG rather than ENOENT, and an ordinary typo in a tile name
// returns a field error, and both used to carry the server's layout. The
// relative form is also the more useful one at boot, where the surrounding
// error already names the maps directory.
//
// A map that is not installed comes back as an error wrapping
// fs.ErrNotExist (through loadAs' own decodeStrict), so a caller can tell
// "nothing is installed under that name" from "something is installed and
// it is broken" without matching on message text.
func LoadInstalled(mapsDir, id, artDir string) (*Map, error) {
	file := id + ".json"
	if !idIsAFilename(id) {
		return nil, fmt.Errorf(
			"mapdef: %q is not a map id: a map's filename IS its id, and %q is not a "+
				"name a map file can have — an id names no directory and holds no "+
				"path separator", id, file)
	}
	// The name every error below carries. NOT the path opened: see this
	// function's own doc comment.
	// THE FILENAME THE DIRECTORY ACTUALLY HOLDS, not the filesystem's idea of a
	// match. The path here is built FROM the id, and a case-insensitive volume
	// (APFS by default) resolves Cellar.json for "cellar" — the declared-id
	// check below then passes too, because that file declares "cellar". The map
	// loads on the machine it was authored on and 404s on a Linux server.
	//
	// loadMapsDir's own doc rests on this being exact: "a map's declared id is
	// always exactly its filename minus .json, and a filesystem cannot hold two
	// entries of the same name, so the duplicate-id collision cannot arise here
	// by construction". That holds only under an exact comparison. Boot was
	// already safe — it derives the id from the REAL entry name — so what this
	// closes is a map installed AFTER boot, the case install-then-load exists
	// for. Same hole artlib.Library closed for art the same day.
	if err := refuseCaseOnlyMatch(mapsDir, file); err != nil {
		return nil, err
	}

	display := "maps/" + file

	m, err := loadAs(filepath.Join(mapsDir, file), display)
	if err != nil {
		return nil, err
	}

	// THE FILENAME IS THE ID (design spec §6): Compile takes a scene's id
	// from m.ID, never from the file it was loaded from, so a filename that
	// disagrees with the map's own declared id would let a rename silently
	// mint a second scene for the same place while the original stayed in
	// the world. Refused loudly, naming BOTH strings.
	if m.ID != id {
		return nil, fmt.Errorf(
			"%s declares id %q: a map's filename is its id, and a "+
				"disagreement would put a second scene in the world for the same "+
				"place — mapdef.Compile takes the scene id from the map's ID",
			display, m.ID)
	}

	// Dry run: proves every square's nature resolves against the standard
	// vocabulary, and that no art this map names was written for a format LATER
	// than this server understands, before the map is considered loadable at
	// all. Compile itself, not a bespoke second check — the same "one
	// construction site" discipline internal/adventure/load.go's loadScenes
	// follows, and the reason boot and on-demand cannot drift apart on what a
	// loadable map is.
	if _, _, err := Compile(m, artDir); err != nil {
		return nil, fmt.Errorf("map %q (%s): %w", m.ID, display, err)
	}
	return m, nil
}

// idIsAFilename reports whether id is the name of one entry in a maps
// directory — which is the only thing a map id ever is (design spec §6:
// "A directory cannot hold two cellar.json, so the guarantee MapTool buys
// with a UUID we get for free").
//
// The id an on-demand lookup passes is whatever a client put in
// vttv1.LoadMap.MapId, and two different things are refused there. An id
// like "../elsewhere" would read a file the campaign's maps/ does not
// contain, and — since the map's own ID field would then agree with the id
// that was asked for — install it as a legitimate map. An id like
// "sub/level-2" escapes nothing, but loadMapsDir's flat walk can never
// produce that id at boot, so it would load once and vanish on the next
// restart: the divergence §12 warns about, arriving from the other side.
//
// BOOT REACHES THIS TOO, which round 1 of 2026-09-01-create-scene-leaves
// Task 6 wrongly denied (found in review of that round). loadMapsDir derives an id by trimming ".json" off
// a directory entry's name, and three real filenames trim to something
// that is not a filename: ".json" leaves "", "..json" leaves ".", and
// "...json" leaves "..". A map file may not be called any of those, and now
// the refusal says so by naming the FILE as well as the id, so an operator
// is not told about an id nobody typed.
//
// filepath.Base does the separator work, on whatever separator this
// platform actually uses; "." and ".." need naming separately because Base
// returns them unchanged. An empty id needs no case of its own — Base("")
// is ".", which is not "".
func idIsAFilename(id string) bool {
	return id != "." && id != ".." && filepath.Base(id) == id
}

// refuseCaseOnlyMatch reports a directory entry differing from file only in
// case, and says nothing otherwise.
//
// A directory that cannot be read, and a file genuinely absent, both return nil
// deliberately: loadAs reports those in the messages every caller already
// expects, and repeating them here would be a second thing to keep in
// agreement. This adds exactly one refusal, for the one state the platform
// could not otherwise see.
//
// ONE ReadDir PER CALL, and that is quadratic at boot on purpose: loadMapsDir
// calls LoadInstalled once per entry, so a campaign of n maps costs n scans of
// n names. Measured 2026-09-08 — 10 maps 1ms, 100 maps 12ms, 400 maps 88ms —
// against a boot that happens once, so the snapshot-threading artlib needed is
// not earned here. artlib's Validate was the opposite call: 773ms at 800
// pieces, on a path `vtt art install` runs repeatedly. Measure before copying
// either answer.
//
// ToLower rather than strings.EqualFold, matching artlib for the same reason:
// EqualFold applies Unicode simple folding, so it calls "maſonry-1" a case-only
// match for "masonry-1" and would name a file whose rename fixes nothing.
func refuseCaseOnlyMatch(mapsDir, file string) error {
	// The error is DISCARDED, not tested-and-ignored, and the difference is
	// what golangci-lint's nilerr objects to in the tested form. A directory
	// that cannot be read has nothing to say about case: loadAs reports it in
	// the message every caller already expects. os.ReadDir returns what it
	// managed to read alongside any error, so a partial listing still answers
	// the only question asked here, and an empty one falls through to nil.
	entries, _ := os.ReadDir(mapsDir)
	for _, e := range entries {
		if e.Name() == file {
			return nil
		}
	}
	want := strings.ToLower(file)
	for _, e := range entries {
		if strings.ToLower(e.Name()) == want {
			return fmt.Errorf(
				"mapdef: maps/%s: the directory holds %q, which differs only in case — "+
					"a map's filename IS its id, and matching it loosely would load this "+
					"map here and on no case-sensitive filesystem; rename it to %q",
				file, e.Name(), file)
		}
	}
	return nil
}
