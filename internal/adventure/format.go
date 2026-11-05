// Package adventure is the loader/compiler for the adventure format v1
// (adventure-format spec, sub-project 9 §4/§5): a directory of prepared
// content — scenes, statblock instances, revealed-world notes, an opening,
// and a secrets-bearing DM guide — written FOR a specific ruleset. Load
// reads and fully validates one such directory against a *rules.Ruleset;
// Compile turns a loaded Adventure into the ordinary setup events (spec
// §3) one atomic AppendBatch will apply — the platform gains no new
// runtime concept, an adventure simply becomes log history.
//
// A scene is a map (maps-as-geometry spec §4.3: "an adventure still carries
// its own maps"): AdventureScene's Tiles/Overrides/Objects mirror
// mapdef.Map's own two-layer shape field-for-field, and Compile
// (compile.go) builds each scene's SceneCreated through the exact same
// mapdef.BuildSceneCreated function the standalone maps/ load path calls —
// see compile.go's doc comment for why that single construction site is the
// point of the whole task.
package adventure

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// Adventure is one fully-loaded, fully-validated adventure directory.
// GuidePath names dir/guide.md — served verbatim to the DM via MCP
// (get_adventure_guide, a future task) and NEVER read by this package: the
// guide's secrets must never enter the compiled event log (spec §2.3).
type Adventure struct {
	ID, Name, RulesetID string
	OpeningNarration    string

	// Scenes/Actors/Notes are stored in LOAD order (spec §5's deterministic
	// compile order reads them positionally): Scenes and Actors in
	// file-name order (one scene/actor per file, scenes/*.json and
	// actors/*.json respectively); Notes in file-name order across
	// notes/*.json files, then in declared (array) order within each file
	// (each notes/*.json file's top-level JSON value is an ARRAY of note
	// objects — see load.go's loadNotes doc comment for why this shape was
	// chosen over one-note-per-file).
	Scenes []AdventureScene
	Actors []AdventureActor
	Notes  []AdventureNote

	// ArtDir is the adventure's own flat art directory (dir/art), the root
	// every scene's Overrides and object art resolve against through
	// internal/artlib. It is the successor to the embedded tiles/pack.json
	// Task 7 of that plan deleted, and the reason an adventure stays
	// self-contained: art travels inside the bundle rather
	// than being installed into the campaign's art/, where two adventures
	// shipping the same filename would fight (controller's ruling,
	// 2026-09-03; art-is-a-flat-library plan, pre-flight finding).
	//
	// THIS SAID "Load refuses a bundle that still ships one, by name" until
	// 2026-09-07. It did, until 66ef637 deleted that refusal along with the
	// rest of the migration route; TestABundlesTilesDirectoryIsNotReadAtAll
	// now pins the opposite — a bundle still shipping tiles/pack.json loads,
	// and the directory is simply not read.
	//
	// WHAT IS BUILT HERE IS THE RESOLVING HALF ONLY, and a reader should not
	// take this field for a finished feature. ArtDir's single consumer is the
	// mapdef.BuildSceneCreated call in Compile, and the wire carries a bare art
	// id: TileRef.art and SceneObject.art name no root. The only route serving
	// art bytes is GET /api/art/{file}, whose handleArtFile opens the CAMPAIGN's
	// art directory, so a piece resolved from here is fetchable by no client —
	// it resolves with no warning and every browser draws the square plain,
	// which §4 of the design spec calls worse than the refusal it replaced. No
	// shipped adventure reaches this (neither declares an override or an object
	// and neither ships art/), and docs/map-format.md §9 points authors at the
	// campaign's art/ instead. Serving it needs a decision — an adventure-scoped
	// route beside GET /api/adventures/{id}/guide, or a documented install step
	// — not a patch. The
	// directory need not exist — an adventure with no art is legal and its
	// scenes draw from the built-in vocabulary, warning once per reference.
	ArtDir string

	GuidePath string
}

// AdventureScene is one scenes/*.json file: a scene plus its terrain and
// token placements, in declared (array) order. Tiles/Overrides/Objects
// mirror mapdef.Map's own fields exactly (see this file's package doc) —
// Objects reuses mapdef.Object directly rather than a local type, since the
// two formats' object shape is not merely similar but IDENTICAL by design.
type AdventureScene struct {
	ID, Name     string
	GridW, GridH int32
	Tiles        map[string]string
	Overrides    map[string]string
	Objects      []mapdef.Object
	Placements   []Placement
}

// asMap builds the *mapdef.Map compile.go hands to mapdef.BuildSceneCreated
// — the ONE construction site a SceneCreated event comes from, shared with
// the standalone maps/ load path (internal/mapdef/compile.go's own Compile).
// Placements is deliberately left unset: BuildSceneCreated never reads it
// (only the scene half of a compile — Compile's TokenPlaced loop is
// adventure's own, unchanged, since placements carry an ActorID a map
// alone knows nothing about).
func (sc AdventureScene) asMap() *mapdef.Map {
	return &mapdef.Map{
		ID: sc.ID, Name: sc.Name, GridW: sc.GridW, GridH: sc.GridH,
		Tiles: sc.Tiles, Overrides: sc.Overrides, Objects: sc.Objects,
	}
}

// Placement is one token placement, declared inline in its owning scene's
// placements array — it carries no scene reference of its own, because the
// containing AdventureScene IS that reference (spec §4's nested shape).
type Placement struct {
	TokenID, ActorID             string
	X, Y                         int32
	Name, Notes, GMName, GMNotes string
}

// AdventureActor is one actors/*.json file, its statblock checked at Load
// against the ruleset's properties by type (VTT-617, VTT-618). It has no
// controller: who drives a character is the table's to decide, later. Its
// Kind is the author's, required at load (actorKinds in load.go), and is
// carried as the wire enum so nothing downstream re-parses the name.
type AdventureActor struct {
	ID, Name   string
	Kind       vttv1.ActorKind
	Attributes map[string]int32
	Resources  map[string]ResourceVal
}

// ResourceVal is one named resource pool's starting current/max, before
// Compile turns it into a *vttv1.Resource on the compiled Actor.
type ResourceVal struct {
	Current, Max int32
}

// AdventureNote is one initially-REVEALED world note (spec §2.3 — the
// adventure carries only what the party is meant to already know at the
// table; the DM upserts new notes as things are discovered).
type AdventureNote struct {
	Key, Title, Text string
}
