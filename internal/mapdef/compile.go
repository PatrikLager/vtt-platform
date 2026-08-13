package mapdef

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Compile turns m (+ its pack p, which may be nil when m carries no
// overrides) into the ordered wire events one atomic AppendBatch applies:
// exactly one SceneCreated carrying every square's resolved terrain plus its
// objects, followed by one TokenPlaced per placement in declaration order
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

// BuildSceneCreated is the ONE construction site for a SceneCreated event
// (maps-as-geometry Task 4's whole point): both Compile above (the
// standalone map path) and internal/adventure/compile.go (the
// adventure-embedded path) call this exact function, so the two load paths
// cannot drift out of shape with each other —
// TestBothLoadPathsEmitIdenticalSceneEvents (internal/adventure/compile_test.go)
// pins it. p may be nil when m carries no overrides (Resolve only needs one
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
	for _, o := range m.Objects {
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
