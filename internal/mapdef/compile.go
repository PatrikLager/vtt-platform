package mapdef

import (
	"fmt"

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
// and the read limit stop agreeing. Overriding every square with a name of
// ordinary length — 7 to 11 characters in every shipped pack — costs about
// 16 bytes a tile more, putting a 3600-tile scene at 209.8 KiB compact and
// 220.4 KiB spaced: over the limit while still inside this cap, because this
// counts TILES and the limit counts BYTES. Recorded here rather than repaired,
// since changing the cap is a decision about the wire format rather than a
// correction to a number. Past it the frame simply does not arrive, and the way that presents
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

// Compile turns m (+ its pack p, which may be nil when m carries no
// overrides) into the ordered wire events one atomic AppendBatch applies:
// exactly one SceneCreated carrying the resolved terrain of every square the
// map DECLARES — none, for a scene that declares no tiles, which is legal
// (see Map.Tiles) — plus its objects, followed by one TokenPlaced per
// placement in declaration order
// (spec §4.3: "both paths compile through one code path to the same
// events"). Any warning Resolve produces along the way (an override's kind
// not matching its base tile — spec §3.2) is collected and returned rather
// than dropped; Compile itself never refuses on one.
func Compile(m *Map, p *Pack) ([]*vttv1.Envelope, []string, error) {
	sc, warnings, err := BuildSceneCreated(m, p)
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
// CORRECTED 2026-08-18. This comment used to claim BuildSceneCreated was "the
// ONE construction site for a SceneCreated event", and the plan said the same
// ("there is literally one construction site"). Both were false:
// internal/gateway/convert.go also builds a SceneCreated, from a CreateScene
// COMMAND. That is not drift to be stamped out — the two have different
// inputs. convert.go receives Tiles as TileRefs the caller already resolved
// and carries them through; this function receives tile NAMES from a file and
// resolves them against a pack and the standard vocabulary. Only the
// resolution is shared, and only the file paths share it. Forcing the command
// path through here would mean inventing names for refs that arrive resolved.
//
// p may be nil when m carries no overrides (Resolve only needs one
// when an override is present); a nil Pack alongside a non-empty override
// fails loud through Resolve itself, exactly as it would for a standalone
// map — see Resolve's own doc comment.
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
func BuildSceneCreated(m *Map, p *Pack) (*vttv1.SceneCreated, []string, error) {
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
	var warnings []string
	if len(m.Tiles) > 0 {
		for y := int32(0); y < m.GridH; y++ {
			for x := int32(0); x < m.GridW; x++ {
				key := squareKey(x, y)
				res, w, err := Resolve(m, p, key)
				if err != nil {
					return nil, warnings, err
				}
				tiles[key] = &vttv1.TileRef{Kind: res.Kind, Material: res.Material, Art: res.Art}
				warnings = append(warnings, w...)
			}
		}
	}

	objects := make([]*vttv1.SceneObject, 0, len(m.Objects))
	for i, o := range m.Objects {
		// Whole-branch-review finding I1: an object's art was never
		// resolved against anything — ResolveObjectArt (resolve.go) is the
		// object-shaped sibling of the Resolve call the square loop above
		// already makes, run here so a bad object art name fails this exact
		// dry run (maps.go's boot-time mapdef.Compile call) the same way an
		// unresolvable tile override already does, rather than silently
		// riding through to a SceneObject nothing can ever draw.
		if err := ResolveObjectArt(i, o, p); err != nil {
			return nil, warnings, err
		}
		objects = append(objects, &vttv1.SceneObject{
			ObjectId: o.ID, Kind: o.Kind,
			At:    &vttv1.GridPosition{X: o.X, Y: o.Y},
			Width: o.W, Height: o.H,
			RotationDegrees: o.Rotation,
			BlocksSight:     o.BlocksSight, BlocksMove: o.BlocksMove,
			Art: o.Art,
		})
	}

	return &vttv1.SceneCreated{
		SceneId: m.ID, Name: m.Name, GridWidth: m.GridW, GridHeight: m.GridH,
		Tiles: tiles, Objects: objects,
	}, warnings, nil
}
