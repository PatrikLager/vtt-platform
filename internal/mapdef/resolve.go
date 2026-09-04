package mapdef

import (
	"errors"
	"fmt"

	"github.com/PatrikLager/vtt-platform/internal/artlib"
)

// Resolved is what one square becomes on the wire (spec §5): the facts the
// engine enforces (Kind, Material) plus the picture name the renderer draws
// (Art). Kind and Material NEVER come from art — see Resolve's doc comment —
// so a Resolved value is safe to hand downstream precisely because its facts
// already passed through StandardTile before any art was ever looked up.
type Resolved struct{ Kind, Material, Art string }

// Resolve turns one square into the facts the engine needs plus the art name
// the renderer needs, reading art out of artDir (internal/artlib — one flat
// directory where the filename is the identity, 2026-09-02-art-is-a-flat-library
// design spec §3).
//
// NATURE ALWAYS COMES FROM m.Tiles. The override supplies art and nothing
// else (maps-as-geometry spec §3.2, Patrik: "the ART will never decide the
// 'nature' of the square/item"). A kind mismatch WARNS rather than refusing,
// because a wall that looks like a passage is an illusory wall — legitimate
// dungeon craft, and refusing it would forbid a feature one arc away.
//
// ABSENT ART DEGRADES; A SIDECAR THAT CANNOT BE PARSED REFUSES
// (art-is-a-flat-library spec §4). Those are two different facts about the
// world, and the second is what makes the first safe to ship: a reference with
// no file behind it is a typo or a piece nobody installed yet, and the square
// is fully described without it, so it draws plain and the DM is told which
// reference dropped. A sidecar that EXISTS and does not parse is a defect in
// installed content, and drawing that square plain would ship the broken file
// silently. artlib draws the line with sentinels — artlib.ErrNotFound and
// artlib.ErrArtDirUnreadable — so this decision is never made by reading error
// text.
//
// TWO CASES THAT LOOK LIKE "BROKEN" DEGRADE ANYWAY, both by Patrik's ruling of
// 2026-09-03, and both because the alternative takes a table down over a file
// an operator can fix in a second:
//
//   - A PICTURE WITH NO SIDECAR named as tile art. Refusing it stopped the
//     whole server booting, since composeServer turns any loadMapsDir error
//     into a refusal to start — the campaign down for everyone over one
//     missing JSON file, and over exactly the move spec §3.4 teaches (drop a
//     PNG in and use it). It warns with its own sentence naming the file to
//     write, because "not installed" would be false of a file that is sitting
//     right there. Spec §3.4 and exit criterion 6 were amended the same day and
//     now say this outright; criterion 6 read "tile art without one is refused"
//     until then, so a reader holding an older copy will find them disagreeing.
//     Note the asymmetry it removes: the mirror case, a sidecar whose picture
//     is absent, already degraded.
//   - AN ART ROOT THAT CANNOT BE OPENED, here at request time. The boot keeps
//     refusing it — cmd/vtt's artRootIsOpenable, called from composeServer
//     rather than from the maps walk, so that a campaign with art and no map
//     yet is checked too — where an operator is at a terminal and can act. A DM
//     cannot chmod a directory from a browser, and a campaign that worked five
//     minutes ago should not stop working.
//
// The two refusals this replaced were `p == nil` ("needs a pack to resolve")
// and `m.Pack != p.ID` ("that is not this map's pack"), each with a long
// comment arguing for its exact wording. Both arguments were about WHICH
// container a name belonged to, and there are no containers now: any map may
// name any installed art (spec §3.3, exit criterion 2). Their reasoning is not
// carried forward because it has no subject left, and the one durable piece of
// it — that an error must be true of the world rather than of the call — is
// honoured by the degrade warning below, which says the art is not installed
// rather than that no art directory was handed in.
func Resolve(m *Map, artDir, square string) (Resolved, []string, error) {
	base, ok := m.Tiles[square]
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s has no tile", square)
	}
	kind, material, ok := StandardTile(base)
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s names unknown tile %q", square, base)
	}
	art, hasArt := m.Overrides[square]
	if !hasArt {
		return Resolved{Kind: kind, Material: material}, nil, nil
	}
	piece, err := artlib.Lookup(artDir, art)
	switch {
	case errors.Is(err, artlib.ErrNotFound):
		// DEGRADE, not refuse (spec §4). The nature is already in hand from
		// m.Tiles, which is the whole reason this is safe: the square keeps
		// being a wall, it just stops being a PARTICULAR wall.
		return Resolved{Kind: kind, Material: material}, []string{artNotInstalled(art)}, nil
	case errors.Is(err, artlib.ErrArtDirUnreadable):
		return Resolved{Kind: kind, Material: material}, []string{artDirUnreadableWarning}, nil
	case err != nil:
		// A sidecar that EXISTS and cannot be read is a defect to fix, not a
		// square to draw plain. Degrading here would ship a broken sidecar
		// silently.
		return Resolved{}, nil, err
	}
	// A SIDECAR IS EXPECTED FOR TILE ART and never for object art (spec §3.4),
	// and this is the only place in the tree that can tell the two apart:
	// artlib resolves a bare picture happily either way and reports what it
	// found (Piece.HasSidecar) precisely because it does not know whether the
	// caller is drawing a square or a piece of furniture. A square's kind is a
	// fact the engine acts on, so a picture that does not declare one is
	// incomplete AS TILE ART, while the same file is complete object art.
	//
	// DEGRADED, NOT REFUSED — see this function's own doc comment for the
	// ruling. The warning does not say "not installed", which would be false
	// of a file the DM can see in the directory; it names the JSON file to
	// write, which is the whole remedy.
	if !piece.HasSidecar {
		return Resolved{Kind: kind, Material: material}, []string{fmt.Sprintf(
			"art %q has a picture but no sidecar; drawing it plain — write art/%s.json "+
				"to say what kind of square it is (design spec §3.4)", art, art)}, nil
	}
	var warnings []string
	// piece.Kind is advisory and OPTIONAL (artlib's sidecar decoder requires
	// only format_version), so an empty Kind means "not declared" rather than
	// "declared as nothing" — treating it as a mismatch would warn on every
	// override drawn from a piece whose author never filled in the metadata,
	// which is noise the one warning channel must stay free of.
	if piece.Kind != "" && piece.Kind != kind {
		warnings = append(warnings, fmt.Sprintf(
			"art %q is drawn for %s but the square under it is a %s", art, piece.Kind, kind))
	}
	return Resolved{Kind: kind, Material: material, Art: art}, warnings, nil
}

// ResolveObjectArt resolves one object's art against artDir, returning the art
// name to ship and any warning it produced. It closes maps-as-geometry spec
// §4.4's "every `art` name resolves" for objects — the half
// CheckObjectArtDeclared (load.go) cannot perform, since Load reads no art
// (whole-branch-review finding I1: before this function existed, an object's
// art was checked against nothing at all, and a typo produced an invisible
// barrier — a blocked square nothing ever explained).
//
// This is deliberately its OWN function rather than folded into Resolve
// above, and not a call TO Resolve either: Resolve is keyed by a square
// ("x,y", the one unit maps-as-geometry §4.1's two layers are BOTH keyed by),
// and an object has no square key of its own — its X/Y is an anchor, not an
// identity — so forcing it through Resolve's square-shaped signature would
// misrepresent what an object is. idx identifies the object by its position
// in Map.Objects (the same "objects[N]" addressing load.go's other Check*
// functions already use for this exact reason), since an object's own ID is
// author-supplied and neither required nor guaranteed unique.
//
// THE RETURNED ART IS EMPTY WHEN THE PIECE IS NOT INSTALLED, and the object
// still ships. art-is-a-flat-library spec §4: "Object art that does not
// resolve leaves the object in place, with its blocking behaviour intact,
// drawn from its kind. An object is a thing in the world before it is a
// picture, and dropping it because its picture is missing would change what
// the room is." So this makes the same absent/unreadable split Resolve does,
// for the same reasons, with one asymmetry: a bare picture is COMPLETE object
// art (spec §3.4), so nothing here warns about a missing sidecar the way
// Resolve does. That is the whole of the difference between the two now —
// since 2026-09-03 the tile side warns rather than refusing there, so the
// asymmetry costs a sentence to a DM instead of a map to a table.
//
// An empty o.Art is refused, redundantly with CheckObjectArtDeclared:
// BuildSceneCreated (compile.go, which calls this per object) is the ONE
// shared construction site both the standalone-map and adventure-embedded
// load paths call directly, and a caller that builds a *Map by hand — as
// internal/adventure/load.go's own dry run, and several tests in this package,
// both do — can reach here without ever having run Load's checks at all. It
// stays a refusal while an unresolvable name became a warning because the two
// are different facts: an object that names no art at all is a map file with a
// hole in it, not a picture that is missing.
func ResolveObjectArt(idx int, o Object, artDir string) (string, []string, error) {
	if o.Art == "" {
		return "", nil, fmt.Errorf(
			"mapdef: objects[%d] has no art (objects have no standard art fallback — only tiles do)", idx)
	}
	_, err := artlib.Lookup(artDir, o.Art)
	switch {
	case errors.Is(err, artlib.ErrNotFound):
		return "", []string{fmt.Sprintf(
			"art %q is not installed; the object stays, drawn from its kind", o.Art)}, nil
	case errors.Is(err, artlib.ErrArtDirUnreadable):
		return "", []string{artDirUnreadableWarning +
			"; the object stays, drawn from its kind"}, nil
	case err != nil:
		return "", nil, err
	}
	return o.Art, nil, nil
}

// WARNINGS NAME THE ART, NEVER THE SQUARE, and that is what makes
// BuildSceneCreated able to tell a DM about a missing piece ONCE (spec §4)
// instead of once per square. Measured on the shipped campaigns/example
// cellar.json before this changed: four missing art names produced 96
// warnings and 6840 bytes, which the client joins into one untruncated toast;
// at MaxWireTiles the same shape is ~270 KB in a single CommandResult, over
// the 200 KiB read limit Go clients set. Deduplication needs identical
// strings, so the square key cannot be in them — compile.go appends the count
// instead, which is the number a DM can act on anyway.
//
// NOR DOES ANY OF THEM NAME A PATH. A warning rides back on an ok=true
// CommandResult to whoever issued the command, exactly as an error does, so
// the no-server-layout rule LoadInstalled states applies to warnings word for
// word. artDirUnreadableWarning is deliberately a constant with nothing
// interpolated into it.
// NOT "this campaign's" — an adventure resolves against its OWN bundle
// (Adventure.ArtDir = <adventure>/art), so the possessive sent a bundle
// author to look in the wrong directory. The caller qualifies it where it
// can: adventure.Compile prefixes the scene, exactly as it does for every
// other warning it forwards.
const artDirUnreadableWarning = "the art directory cannot be read; drawing it plain"

// artNotInstalled is the ordinary degrade sentence, in one place because
// Resolve is not the only thing that has to produce it byte-identically:
// compile.go's aggregation groups by exact string.
func artNotInstalled(art string) string {
	return fmt.Sprintf("art %q is not installed; drawing it plain", art)
}
