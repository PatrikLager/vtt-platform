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
// EVERY ART PROBLEM DEGRADES BUT ONE (art-is-a-flat-library spec §4). A square
// is fully described without art — the nature comes from m.Tiles — so an art
// reference that does not resolve costs the picture and nothing else: the
// square draws plain and the DM is told which reference dropped and why.
//
// THE EXCEPTION IS A SIDECAR DECLARING A format_version THIS SERVER DOES NOT
// UNDERSTAND, and it refuses (Patrik, 2026-09-04). It is not "this file is
// broken" but "this content is newer than this server", and the two want
// different answers: degrading it would turn a v2 art set into hundreds of
// plain squares and a wall of warnings reading "my art is broken", when the
// remedy is a newer server. One refusal naming both versions says that; ninety
// warnings do not.
//
// THE DECISION IS MADE ON SENTINELS, NEVER ON MESSAGE TEXT —
// artlib.ErrNotFound, artlib.ErrArtDirUnreadable, artlib.ErrFormatVersion —
// which is the discipline artlib has kept since Task 1 built the first of
// them. A strings.Contains here would silently stop refusing the day somebody
// rewords an error.
//
// FOUR CASES THAT LOOK LIKE "BROKEN" DEGRADE ANYWAY, and every one of them
// because the alternative takes a table down over a file an operator can fix
// in a second:
//
//   - A PICTURE WITH NO SIDECAR named as tile art (Patrik, 2026-09-03).
//     Refusing it stopped the whole server booting, since composeServer turns
//     any loadMapsDir error into a refusal to start — the campaign down for
//     everyone over one missing JSON file, and over exactly the move spec §3.4
//     teaches (drop a PNG in and use it). It warns with its own sentence naming
//     the file to write, because "not installed" would be false of a file that
//     is sitting right there. Spec §3.4 and exit criterion 6 were amended the
//     same day and now say this outright; criterion 6 read "tile art without one
//     is refused" until then, so a reader holding an older copy will find them
//     disagreeing. Note the asymmetry it removes: the mirror case, a sidecar
//     whose picture is absent, already degraded.
//   - AN ART ROOT THAT CANNOT BE OPENED, here at request time (Patrik,
//     2026-09-03). The boot keeps refusing it — cmd/vtt's artRootIsOpenable,
//     called from composeServer rather than from the maps walk, so that a
//     campaign with art and no map yet is checked too — where an operator is at
//     a terminal and can act. A DM cannot chmod a directory from a browser, and
//     a campaign that worked five minutes ago should not stop working.
//   - A SIDECAR THAT CANNOT BE READ: a missing brace, a truncated copy, a
//     wrongly typed field, a format_version holding something that is not a
//     version number, a picture that turns out to be a directory (Patrik,
//     2026-09-04). This one REFUSED until that ruling, and the refusal was
//     measured: one corrupt sidecar named by one committed map stopped the
//     server booting, exit status 1, every other map fine, with the boot log
//     printing "the server is starting anyway" one line earlier. It is the same
//     shape ruled against three times already, and it survived only because
//     nobody re-asked after the warning channel existed to carry what the
//     refusal used to carry.
//   - A DOOR SIDECAR DECLARING ONLY kind AND material, which the ruling does
//     not name and which is decided here as part of the case above. The line
//     the ruling draws is "is this file broken, or is this server too old", and
//     a door with one picture is a broken file: this server reads it perfectly
//     and finds it incomplete, and the remedy is to edit that one file, exactly
//     as for a missing brace. campaigns/example ships a door, so under a
//     refusal an operator who drops the "open" line out of art/cellar-door.json
//     cannot start the server at all. No code decides it — the door error is
//     simply not one of the three sentinels — and that is the evidence the line
//     is in the right place.
//
// The two refusals this replaced were `p == nil` ("needs a pack to resolve")
// and a comparison of the map's own declared pack against the pack it was
// handed ("that is not this map's pack"), each with a long comment arguing for
// its exact wording. Both arguments were about WHICH container a name belonged
// to, and there are no containers now: any map may name any installed art
// (spec §3.3, exit criterion 2). Their reasoning is not carried forward because
// it has no subject left — Map.Pack, the field the second one read, was itself
// deleted at Task 5 of the same plan, which made a map file that declares one a
// refusal at Load. The one durable piece of that reasoning — that an error must
// be true of the world rather than of the call — is honoured by the degrade
// warning below, which says the art is not installed rather than that no art
// directory was handed in.
// Resolve is the single-square form. A load resolving MANY squares must not
// call it: artlib.Open reads the art directory, so one call per square is one
// directory scan per square. BuildSceneCreated opens the library once and uses
// resolveWith below; this wrapper exists for callers with one square to answer.
func Resolve(m *Map, artDir, square string) (Resolved, []string, error) {
	return resolveWith(m, artlib.Open(artDir), square)
}

func resolveWith(m *Map, lib *artlib.Library, square string) (Resolved, []string, error) {
	base, ok := m.Tiles[square]
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s has no tile", square)
	}
	kind, material, ok := StandardTile(base)
	if !ok {
		return Resolved{}, nil, fmt.Errorf("mapdef: square %s names unknown tile %q",
			square, base)
	}
	art, hasArt := m.Overrides[square]
	if !hasArt {
		return Resolved{Kind: kind, Material: material}, nil, nil
	}
	piece, err := lib.Lookup(art)
	var mismatch *artlib.CaseMismatch
	switch {
	// BEFORE the plain ErrNotFound arm, which this error also satisfies. A
	// case-only mismatch degrades identically; the whole difference is in what
	// the DM is told, and they are told it while looking straight at the file.
	case errors.As(err, &mismatch):
		return Resolved{Kind: kind, Material: material},
			[]string{artCaseMismatch(art, mismatch.Real)}, nil
	case errors.Is(err, artlib.ErrNotFound):
		// DEGRADE, not refuse (spec §4). The nature is already in hand from
		// m.Tiles, which is the whole reason this is safe: the square keeps
		// being a wall, it just stops being a PARTICULAR wall.
		return Resolved{Kind: kind, Material: material}, []string{artNotInstalled(art)}, nil
	case errors.Is(err, artlib.ErrArtDirUnreadable):
		return Resolved{Kind: kind, Material: material}, []string{artDirUnreadableWarning}, nil
	case errors.Is(err, artlib.ErrFormatVersion):
		// THE ONE ART FAILURE THAT STILL REFUSES, and it is a fact about the
		// SERVER rather than about the file — see this function's own doc
		// comment for the ruling. The error already names both versions.
		return Resolved{}, nil, err
	case err != nil:
		// EVERYTHING ELSE A SIDECAR CAN GET WRONG DEGRADES (Patrik,
		// 2026-09-04). The nature is already in hand from m.Tiles, so the
		// square keeps being what the map says it is and loses only its
		// picture — and the campaign keeps booting, which is what the refusal
		// this replaced actually cost.
		return Resolved{Kind: kind, Material: material},
			[]string{artCannotBeUsed(art, err, "drawing it plain")}, nil
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
				"to say what kind of square it is (design spec §3.4)", name(art), name(art))}, nil
	}
	var warnings []string
	// piece.Kind is advisory and OPTIONAL (artlib's sidecar decoder requires
	// only format_version), so an empty Kind means "not declared" rather than
	// "declared as nothing" — treating it as a mismatch would warn on every
	// override drawn from a piece whose author never filled in the metadata,
	// which is noise the one warning channel must stay free of.
	if piece.Kind != "" && piece.Kind != kind {
		warnings = append(warnings, fmt.Sprintf(
			"art %q is drawn for %s but the square under it is a %s",
			name(art), artlib.Clip(piece.Kind, artlib.MaxFragment), kind))
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
// THE RETURNED ART IS EMPTY WHENEVER THE PIECE DOES NOT RESOLVE, and the
// object still ships. art-is-a-flat-library spec §4: "Object art that does not
// resolve leaves the object in place, with its blocking behaviour intact,
// drawn from its kind. An object is a thing in the world before it is a
// picture, and dropping it because its picture is missing would change what
// the room is." So this makes the same four-way split Resolve does, for the
// same reasons, with one asymmetry: a bare picture is COMPLETE object art
// (spec §3.4), so nothing here warns about a missing sidecar the way Resolve
// does. That is the whole of the difference between the two now.
//
// THE SPLIT IS WRITTEN OUT TWICE, HERE AND IN Resolve, and nothing forces the
// two switches to agree — which is why internal/mapdef's object-side tests are
// not redundant with the tile-side ones. A ruling applied to one arm and not
// the other compiles and passes every test about the other.
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
// ResolveObjectArt is the single-object form; see Resolve on why a load uses
// the library-taking variant instead.
func ResolveObjectArt(idx int, o Object, artDir string) (string, []string, error) {
	return resolveObjectArtWith(idx, o, artlib.Open(artDir))
}

func resolveObjectArtWith(idx int, o Object, lib *artlib.Library) (string, []string, error) {
	if o.Art == "" {
		return "", nil, fmt.Errorf(
			"mapdef: objects[%d] has no art (objects have no standard art fallback — only tiles do)", idx)
	}
	_, err := lib.Lookup(o.Art)
	var mismatch *artlib.CaseMismatch
	switch {
	// THE SAME ARM Resolve has, and it has to be written twice because this
	// switch is written twice — see this function's own doc on why nothing
	// forces the two to agree. Objects are where a DM most often drops a
	// hand-copied file, so this is the side that needs the filename most.
	case errors.As(err, &mismatch):
		return "", []string{fmt.Sprintf(
			"art %q is not installed, but the art directory holds %q, which differs only "+
				"in case; rename it — the object stays, drawn from its kind",
			name(o.Art), mismatch.Real)}, nil
	case errors.Is(err, artlib.ErrNotFound):
		return "", []string{fmt.Sprintf(
			"art %q is not installed; the object stays, drawn from its kind", name(o.Art))}, nil
	case errors.Is(err, artlib.ErrArtDirUnreadable):
		return "", []string{artDirUnreadableWarning +
			"; the object stays, drawn from its kind"}, nil
	case errors.Is(err, artlib.ErrFormatVersion):
		return "", nil, err
	case err != nil:
		return "", []string{artCannotBeUsed(o.Art, err,
			"the object stays, drawn from its kind")}, nil
	}
	return o.Art, nil, nil
}

// WARNINGS NAME THE ART, NEVER THE SQUARE, and that is what makes
// BuildSceneCreated able to tell a DM about a missing piece ONCE (spec §4)
// instead of once per square. Deduplication needs identical strings, so the
// square key cannot be in them — compile.go appends the count instead, which
// is the number a DM can act on anyway. What makes the collapse load-bearing
// is a bound per ADVENTURE rather than per map: adventure.Compile puts every
// scene's warnings on one CommandResult and nothing caps the scene count.
// CORRECTED 2026-09-07 — the "6840 bytes, ~270 KB at MaxWireTiles, over the
// 200 KiB read limit" this used to cite does not reproduce, and at the real
// rate one map stays under that limit. See warningTally in compile.go.
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
	return fmt.Sprintf("art %q is not installed; drawing it plain", name(art))
}

// artCaseMismatch names the file the directory holds, because "not installed"
// is a sentence a DM reads while looking straight at it. The remedy is a rename
// and nothing else says so — art ids are lowercase by rule (artlib's isArtID),
// so the fix is always "make the filename match the id the map names".
func artCaseMismatch(art, onDisk string) string {
	return fmt.Sprintf("art %q is not installed, but the art directory holds %q, which "+
		"differs only in case; rename it — a filename IS the id a map names, and matching "+
		"it loosely would draw here and on no case-sensitive filesystem; drawing it plain",
		name(art), onDisk)
}

// artCannotBeUsed is the sentence for art that IS installed and does not
// resolve — a truncated sidecar, a wrongly typed field, a door that names one
// picture (Patrik's ruling, 2026-09-04, which turned all of these from a
// refused map into a plain square).
//
// IT IS NOT artNotInstalled WITH DIFFERENT WORDS, and that is the point §3.4
// makes about the sidecar-less picture for the same reason: the file is
// sitting in art/ where the DM can see it, so "not installed" would send them
// hunting for something that is already there. This names the piece, carries
// the cause verbatim, and leaves them looking at the file they have to fix.
//
// "CANNOT BE USED" RATHER THAN "CANNOT BE READ", because a door declaring only
// kind and material was read perfectly well and is merely incomplete — one
// sentence covers both, and neither half of it is false of either.
//
// tail is what happened instead: a square draws plain, an object stays. It is
// a parameter rather than two functions because the two callers must not drift
// apart on the first half, which is the half a DM reads.
//
// err IS SAFE TO INTERPOLATE, and this helper is the only thing in this file
// that interpolates one. THAT IS TWO OF THE EIGHT SENTENCES THE ART PATH CAN
// PRODUCE, not one: Resolve and ResolveObjectArt each call this, from switches
// that nothing forces to agree. The other six are the two not-installed
// sentences, the no-sidecar sentence, the kind mismatch, and the
// unreadable-root pair — of which the first four carry an art name and the
// last two are a constant carrying nothing at all.
//
// Every artlib error that can arrive here either never named a path or had it
// stripped by that package's bareCause, which matters because these warnings
// ride back to whoever issued load_map — an agent seat included — exactly as
// an error does. internal/mapdef's TestNoArtFailureNamesTheDirectoryItRead and
// internal/gateway's TestNoWarningTellsAClientWhereTheCampaignLives are the
// guards, at the unit and the wire, and the wire one needs BOTH arms driven:
// review measured (2026-09-05) that appending artDir to only the object tail
// left every gateway path test green while the unit test caught it.
//
// IT STILL DEDUPLICATES, which is not obvious once an error is inside the
// string: compile.go's warningTally groups by the exact sentence, so anything
// varying per SQUARE would put the 96-warning measurement spec §4 records
// straight back. artlib's errors are a function of the file, not of the
// lookup, so ninety squares naming one broken piece produce ninety identical
// sentences and one line with a count.
func artCannotBeUsed(art string, err error, tail string) string {
	return fmt.Sprintf("art %q is installed but cannot be used (%v); %s",
		art, err, tail)
}

// name bounds an art name on its way into one of this file's warnings.
//
// EVERY WARNING BELOW COMPOSES ITS OWN SENTENCE from the art name rather than
// rendering artlib's error, so artlib bounding the id inside ITS message does
// nothing for these — they are different paths, not two bounds on one path.
// That distinction was got wrong once: both clips looked redundant because no
// test drove this side, and deleting mapdef's left an override value of any
// length reaching a CommandResult. Measured then: 20,041 bytes, and the socket
// closed with "message too big".
//
// An override value is whatever the map file said. CheckOverridesInsideGrid
// validates the KEY and says outright that the value is not inspected, so
// nothing upstream has bounded it by the time it arrives here.
func name(art string) string { return artlib.Clip(art, artlib.MaxFragment) }
