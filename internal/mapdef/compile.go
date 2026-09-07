package mapdef

import (
	"fmt"
	"strings"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// MaxWireTiles is the largest number of tiles one SceneCreated may carry, and
// it is a TRANSPORT limit rather than a rule about maps (spec §7).
//
// SceneCreated ships one TileRef per declared tile as protojson, and every
// byte figure below has TWO honest values because protojson randomises its own
// output: it appends a space after every comma in roughly half of all builds,
// seeded by internal/detrand from a hash of the binary, deliberately, so that
// "the output is unstable across different builds". Task 4's long-standing
// "45.5 KiB, about 45.5 bytes a tile" for a 32x32 is the SPACED regime,
// confirmed by re-running BuildSceneCreated on the real ravine; the same scene
// is 43.5 KiB / 43.5 bytes a tile in a compact build. 3600 tiles is the 60x60
// the spec calls "the honest limit today": 153.6 KiB compact, 160.6 KiB
// spaced, against the 200 KiB read limit Go clients set
// (internal/harness/client.go's readLimit), leaving room for the objects,
// placements and names that ride in the same frame.
//
// THE HEADROOM ABOVE ASSUMES NO ART OVERRIDES, which is where this constant
// and the read limit stop agreeing. Overriding a square costs exactly
// len(name) + 9 bytes compact and +10 spaced — the `,"art":""` scaffolding
// plus the name. Shipped TILE names run 7 to 11 characters: the only shipped
// map that overrides anything is campaigns/example/maps/cellar.json, whose
// overrides name earth-1, masonry-1, flagstone-1 and cellar-door.
// (client/public/std-pack is
// a client-side rendering manifest the server never reads, and its object
// names ride in SceneObject.art, not TileRef.art — counting either widens
// the range spuriously.) So overriding all 3600 tiles lands
// between 209.8 and 223.9 KiB compact, 220.4 and 234.4 spaced: over the limit
// while still inside this cap, because this counts TILES and the limit counts
// BYTES. Recorded here rather than repaired, since changing the cap is a
// decision about the wire format rather than a correction to a number.
//
// Past the cap the frame simply does not arrive, and the way that presents
// is a connection torn down mid-session — which is exactly how loading
// goblin-ambush killed every connection before that read limit was raised.
// Refusing at compile turns a mystery at the table into a message at load.
//
// COUNTED IN TILES, NOT GRID SQUARES, and the distinction is load-bearing:
// tiles are optional (Patrik's ruling 2026-08-13), so a large grid that
// declares no terrain costs nothing on the wire and must still load —
// internal/rules/conformance relies on that with a tile-less 100x100 scene.
// Sizing this on GridW*GridH would refuse maps that are free to send.
//
// NOT A PERMANENT CEILING. Spec §7 files the remedy: a compact wire encoding
// (a palette plus index rows) would put 200x200 near 40 KB. When that lands,
// this constant moves or goes — the authoring format was never the problem,
// and §7 is explicit that "authoring and transport need not be the same shape".
const MaxWireTiles = 3600

// Compile turns m, resolved against the flat art directory artDir, into the
// ordered wire events one atomic AppendBatch applies:
// exactly one SceneCreated carrying the resolved terrain of every square the
// map DECLARES — none, for a scene that declares no tiles, which is legal
// (see Map.Tiles) — plus its objects, followed by one TokenPlaced per
// placement in declaration order
// (spec §4.3: "both paths compile through one code path to the same
// events"). Every warning Resolve and ResolveObjectArt produce along the way
// — an override's kind not matching its base tile (maps-as-geometry spec
// §3.2), and art that is not installed, has no sidecar, cannot be read, or
// sits in an art directory this process cannot open (art-is-a-flat-library
// spec §4) — is collected, DEDUPLICATED (see warningTally below: once per
// distinct message, with the number of squares or objects it happened to) and
// returned rather than dropped; Compile itself never refuses on one. A sidecar
// declaring a format_version LATER than this server understands is the one case
// that does refuse, and it comes back as an error rather than a warning. (It
// read "a format_version this server does not understand" until
// art-is-a-flat-library Task 8, when a typo'd 0 stopped refusing: nothing below
// this server's own version is content it is too old for, so those degrade with
// the broken files.)
func Compile(m *Map, artDir string) ([]*vttv1.Envelope, []string, error) {
	sc, warnings, err := BuildSceneCreated(m, artDir)
	if err != nil {
		return nil, warnings, err
	}

	envs := make([]*vttv1.Envelope, 0, 1+len(m.Placements))
	envs = append(envs, &vttv1.Envelope{
		Payload: &vttv1.Envelope_SceneCreated{SceneCreated: sc},
	})
	for _, pl := range m.Placements {
		envs = append(envs, &vttv1.Envelope{
			Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
				TokenId: pl.TokenID, SceneId: m.ID, ActorId: pl.ActorID,
				Position: &vttv1.GridPosition{X: pl.X, Y: pl.Y},
			}},
		})
	}
	return envs, warnings, nil
}

// BuildSceneCreated is the one place a map FILE's tile names are resolved into
// TileRefs (maps-as-geometry Task 4's point): both Compile above (the
// standalone map path) and internal/adventure/compile.go (the
// adventure-embedded path) call this exact function, so the two load paths
// cannot drift out of shape with each other —
// TestBothLoadPathsEmitIdenticalSceneEvents (internal/adventure/compile_test.go)
// pins it.
//
// IT IS NOT THE ONLY PLACE A SceneCreated IS BUILT, and this comment has now
// claimed otherwise twice. Grep and see for yourself — the whole tree, not the
// file you are editing:
//
//	grep -rn 'vttv1\.SceneCreated{' --include='*.go' . | grep -v _test.go | grep -v /gen/
//
// Two live sites answer, and they have different inputs, which is why neither
// can be folded into the other:
//
//   - HERE. A map FILE's tile names, resolved against the standard vocabulary
//     into TileRefs, with art names resolved against artDir. Plus its objects.
//     The full room.
//   - internal/gateway/project.go's per-viewer scene introduction. A REDACTED
//     outline — id, name, grid width and height, and deliberately NO tiles and
//     NO objects at all (visibility spec §4.2: "of course there is a board, but
//     you do not know what is in the black area before you enter the black
//     area"). Its squares arrive later, one at a time, through SceneSeen. It
//     landed 2026-08-20 (83c9c8f) and has been a second construction site ever
//     since.
//
// The count was two on 2026-08-18 as well, when a "there is literally one
// construction site" claim was CORRECTED here — but the site named then was
// internal/gateway/convert.go, building one from a create_scene COMMAND whose
// Tiles arrived already resolved. That command left the platform on 2026-09-02
// (Patrik's ruling, 2026-09-01), its conversion arm went with it, and the
// correction's own list went stale rather than its conclusion: the tally never
// dropped to one, because project.go had joined it in between.
//
// ADDING A FIELD TO SceneCreated MEANS TOUCHING BOTH, or deciding in writing
// that the projected seat is not supposed to have it. Update only this one and
// every DM sees the field while every projected seat gets the zero value, and
// nothing in this package would say so: TestBothLoadPathsEmitIdenticalSceneEvents
// holds the map path against the adventure path, and BOTH of those come through
// this function, so it cannot see the redacted builder at all. The corpus's
// reach is narrow too — scenarios/goldens/session-zero is the only scenario
// carrying projections/*/state.json, so its player and spectator are the only
// projected seats anything folds.
//
// artDir may be empty, name a directory that does not exist, or name one this
// process cannot open: a campaign that has installed no art is ordinary, and
// its maps still load and draw plain. Every override then degrades to its base
// tile (art-is-a-flat-library spec §4), and the DM gets ONE warning per
// distinct reason rather than one per square — see Resolve's own doc comment
// for the line between that and the one sidecar failure that still refuses,
// and warningTally below for why the count matters.
//
// Squares are resolved in ROW-MAJOR order (y outer, x inner, both from 0),
// walking the grid rather than ranging m.Tiles: Go map iteration order is
// randomized per run, and while the returned Tiles value is itself a map
// (so its own iteration order is not observable on the wire), the ORDER
// warnings accumulate in — and, for a broken map, WHICH square's error is
// reported first — would flake between runs without a fixed traversal.
// Golden-file stability depends on this being deterministic.
//
// m.Tiles empty means m declared no terrain at all (Patrik's ruling,
// 2026-08-13: tiles is optional, and a map with none has no terrain,
// exactly as every map did before this format existed). The square loop is
// skipped entirely in that case — calling Resolve per square would
// otherwise fail "square has no tile" for every one of them, since Resolve
// has no notion of "this map opted out of terrain" — and Tiles ships empty
// on the wire.
func BuildSceneCreated(m *Map, artDir string) (*vttv1.SceneCreated, []string, error) {
	// BEFORE the square loop, so an oversized map costs one comparison rather
	// than resolving thousands of tiles it can never deliver.
	if len(m.Tiles) > MaxWireTiles {
		return nil, nil, fmt.Errorf(
			"mapdef: scene %q declares %d tiles, over the %d this wire format can deliver "+
				"(one TileRef per tile at ~43.5-45.5 bytes, the spread being protojson's "+
				"own build-to-build variation, against a 200 KiB client read limit; "+
				"%d tiles is roughly 60x60). A larger scene compiles but its SceneCreated "+
				"never arrives, tearing down the connection instead. Split the map, or drop "+
				"the tiles it does not need — terrain is optional and a tile-less scene of "+
				"any size is free to send",
			m.ID, len(m.Tiles), MaxWireTiles, MaxWireTiles)
	}

	tiles := make(map[string]*vttv1.TileRef, len(m.Tiles))
	var squareWarnings, objectWarnings warningTally
	if len(m.Tiles) > 0 {
		for y := int32(0); y < m.GridH; y++ {
			for x := int32(0); x < m.GridW; x++ {
				key := squareKey(x, y)
				res, w, err := Resolve(m, artDir, key)
				if err != nil {
					return nil, squareWarnings.render("square"), err
				}
				tiles[key] = &vttv1.TileRef{Kind: res.Kind, Material: res.Material, Art: res.Art}
				// A KIND MISMATCH IS THE ONE WARNING WHOSE REMEDY IS THE
				// SQUARE, so it is the one that carries square keys — see
				// warningTally.addAt. It is told apart from the three degrade
				// warnings structurally rather than by matching text: a degrade
				// returns no Art (there is no picture to draw), a mismatch
				// returns the art it warned about. Nothing else distinguishes
				// them, and nothing else needs to.
				if res.Art != "" {
					squareWarnings.addAt(key, w...)
				} else {
					squareWarnings.add(w...)
				}
			}
		}
	}

	objects := make([]*vttv1.SceneObject, 0, len(m.Objects))
	for i, o := range m.Objects {
		// Whole-branch-review finding I1: an object's art was never
		// resolved against anything — ResolveObjectArt (resolve.go) is the
		// object-shaped sibling of the Resolve call the square loop above
		// already makes, run here so a bad object art name is REPORTED by
		// this exact dry run (maps.go's boot-time mapdef.Compile call) the
		// same way an unresolvable tile override is, rather than silently
		// riding through to a SceneObject nothing can ever draw.
		//
		// It reports rather than refuses now: art returns empty and w carries
		// the warning when the piece is not installed, because the object
		// stays in the world either way (art-is-a-flat-library spec §4).
		art, w, err := ResolveObjectArt(i, o, artDir)
		if err != nil {
			return nil, allWarnings(squareWarnings, objectWarnings), err
		}
		objectWarnings.add(w...)
		objects = append(objects, &vttv1.SceneObject{
			ObjectId: o.ID, Kind: o.Kind,
			At:    &vttv1.GridPosition{X: o.X, Y: o.Y},
			Width: o.W, Height: o.H,
			RotationDegrees: o.Rotation,
			BlocksSight:     o.BlocksSight, BlocksMove: o.BlocksMove,
			Art: art,
		})
	}

	return &vttv1.SceneCreated{
		SceneId: m.ID, Name: m.Name, GridWidth: m.GridW, GridHeight: m.GridH,
		Tiles: tiles, Objects: objects,
	}, allWarnings(squareWarnings, objectWarnings), nil
}

// warningTally collects Resolve's warnings and tells the DM about each one
// ONCE, with the number of things it happened to — spec §4's "The DM is told
// which references did not resolve, once, as a warning on the load".
//
// THE PROBLEM IT SOLVES IS A REAL BOUND, not a tidiness preference — and the
// bound is per ADVENTURE, not per map.
//
// CORRECTED 2026-09-07, and the correction reverses the argument rather than
// tidying it. This paragraph used to say the un-deduplicated form put 6840
// bytes on the shipped cellar.json and "roughly 270 KB" at MaxWireTiles, over
// the 200 KiB read limit Go clients set (internal/harness/client.go's
// readLimit). The 6840 does not reproduce — those 96 warnings measure 4917 —
// and at the honest rate a map at MaxWireTiles lands UNDER that limit. So the
// threshold this type was said to exist for is one a single map never crosses,
// and a reader re-deriving the design from that sentence would conclude the
// collapse is unnecessary.
//
// THE BOUND THAT HOLDS: adventure.Compile concatenates EVERY scene's warnings
// into ONE CommandResult (scene-qualified in its scene loop, handed to a single
// frame by internal/gateway's handleLoadAdventure). MaxWireTiles caps a SCENE
// and nothing caps the scene count, so this collapse is the only thing standing
// between a bundle whose art/ did not travel and a result too large to arrive —
// which would deliver no warning at the moment every one of them was true.
// Do not restore a byte figure here. State the invariant.
//
// GROUPING IS BY THE EXACT STRING, which is why resolve.go's warnings name the
// art and never the square: two squares missing masonry-1 must produce the
// same sentence or nothing collapses. Insertion order is preserved, so the
// row-major grid walk above still decides the order warnings appear in, and
// TestWarningsSurfaceInRowMajorOrder still has something deterministic to pin.
//
// ONE WARNING KEEPS ITS SQUARES, through addAt: a kind mismatch. The others are
// actionable on the art name alone — install the file, write the sidecar, fix
// the broken one, fix the directory — but a mismatch's remedy is one of two
// opposite things, "this is a deliberate illusory wall" or "I put the wrong art
// here", and only the square tells them apart. A DM with one deliberate
// illusion and one typo sharing an art name gets one line either way; with the
// squares listed they can act on it.
type warningTally struct {
	order []string
	count map[string]int
	// at holds the places that produced each message, in first-encounter
	// order, for the messages that were recorded WITH a place. A message
	// recorded through add has no entry here and renders as a count alone.
	at map[string][]string
}

func (t *warningTally) add(warnings ...string) {
	t.record("", warnings)
}

// addAt is add for a warning whose remedy needs the place: where is a square
// key ("x,y"). Only the mismatch warning is recorded this way — see the type's
// own doc comment for why the other three are better off without.
func (t *warningTally) addAt(where string, warnings ...string) {
	t.record(where, warnings)
}

func (t *warningTally) record(where string, warnings []string) {
	for _, w := range warnings {
		if t.count == nil {
			t.count = make(map[string]int)
			t.at = make(map[string][]string)
		}
		if t.count[w] == 0 {
			t.order = append(t.order, w)
		}
		t.count[w]++
		if where != "" && len(t.at[w]) < maxListedSquares {
			t.at[w] = append(t.at[w], where)
		}
	}
}

// maxListedSquares bounds the places one warning names. Mismatches are rare by
// construction — an author has to deliberately put art of one kind on a square
// of another — so a handful covers the real cases, and the cap is what keeps
// this from re-creating the unbounded output deduplication exists to remove.
// Past it the count still tells a DM the scale.
const maxListedSquares = 4

// render returns one line per distinct warning. unit is "square" or "object":
// the same missing piece is a different fact about a floor than about a
// barrel, so the two are tallied separately rather than summed into a number a
// DM cannot map onto anything.
//
// A warning recorded with places names them; one recorded without gets a count
// when more than one thing produced it, and nothing at all when exactly one
// did — a bare "(1 squares)" is both ungrammatical and noise, and it is what a
// `n > 0` here would ship on every shipped campaign. TestASingleWarningCarries
// NoCountAtAll is the guard.
func (t *warningTally) render(unit string) []string {
	if len(t.order) == 0 {
		return nil
	}
	out := make([]string, 0, len(t.order))
	for _, w := range t.order {
		n := t.count[w]
		switch places := t.at[w]; {
		case len(places) > 0 && n > len(places):
			w = fmt.Sprintf("%s (%d %ss: %s; …)", w, n, unit, strings.Join(places, "; "))
		case len(places) > 1:
			w = fmt.Sprintf("%s (%d %ss: %s)", w, n, unit, strings.Join(places, "; "))
		case len(places) == 1:
			w = fmt.Sprintf("%s (%s %s)", w, unit, places[0])
		case n > 1:
			w = fmt.Sprintf("%s (%d %ss)", w, n, unit)
		}
		out = append(out, w)
	}
	return out
}

// allWarnings is the squares-then-objects order the scene itself is built in.
func allWarnings(squares, objects warningTally) []string {
	rendered := squares.render("square")
	if objs := objects.render("object"); objs != nil {
		rendered = append(rendered, objs...)
	}
	return rendered
}
