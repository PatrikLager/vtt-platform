package mapdef

import (
	"errors"
	"fmt"
	"path/filepath"
)

// ErrPackNotLoaded marks the one failure whose remedy depends on WHO is
// asking: a map that names a pack the caller did not hand over. At boot
// (cmd/vtt's loadMapsDir) that means the pack is not installed, and the
// operator installs it. At request time (internal/gateway's mapByID) it
// can also mean the pack IS installed, was put there after the server
// started, and is not loaded yet — packs are still boot-time only, because
// the 2026-09-01-create-scene-leaves design spec §5 asks for maps on
// demand and is silent about packs.
//
// Wrapped rather than described in prose so the on-demand caller can add
// the half only it knows to be true (restarting picks the pack up) without
// the boot caller saying the same thing, where it would be wrong.
var ErrPackNotLoaded = errors.New("that pack is not among the ones loaded")

// LoadInstalled loads and fully validates the ONE map installed as
// <mapsDir>/<id>.json: the file is read, every check Load makes runs, the
// filename and the map's own declared id must agree, and the map is
// dry-run compiled against the pack it names (packs[m.Pack] — a map that
// declares no pack looks up packs[""], a legal zero-value read, and Compile
// accepts the nil *Pack that comes back). The returned *Map has passed
// everything a map must pass before it can be put in front of a table.
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
func LoadInstalled(mapsDir, id string, packs map[string]*Pack) (*Map, error) {
	file := id + ".json"
	if !idIsAFilename(id) {
		return nil, fmt.Errorf(
			"mapdef: %q is not a map id: a map's filename IS its id, and %q is not a "+
				"name a map file can have — an id names no directory and holds no "+
				"path separator", id, file)
	}
	// The name every error below carries. NOT the path opened: see this
	// function's own doc comment.
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

	// Dry run: proves every override and every object's art actually
	// resolves (kind/material from the standard vocabulary, art from the
	// pack) before this map is considered loadable at all. Compile itself,
	// not a bespoke second check — the same "one construction site"
	// discipline internal/adventure/load.go's loadScenes follows.
	pack := packs[m.Pack]
	if _, _, err := Compile(m, pack); err != nil {
		// A map naming a pack that was not handed over fails Compile for
		// art it cannot resolve, and that error says only that the art
		// needs a pack. Which pack, and the fact that it is not loaded, is
		// knowledge only this function has; WHY it is not loaded is
		// knowledge only the caller has, so that half is left to the
		// caller through ErrPackNotLoaded (above).
		if pack == nil && m.Pack != "" {
			return nil, fmt.Errorf("map %q (%s) declares pack %q: %w — %w",
				m.ID, display, m.Pack, ErrPackNotLoaded, err)
		}
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
