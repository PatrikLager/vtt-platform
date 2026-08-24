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

	// Pack is the adventure's own embedded art pack (dir/tiles/pack.json),
	// loaded once at Load and shared by every scene's Overrides. It mirrors
	// a standalone map's Pack reference (maps-as-geometry spec §4.2's two
	// resolution levels) but is embedded rather than named by id, because
	// the adventure format is self-contained (adventure-format spec §2.2:
	// "No bestiary format" — shared content libraries were rejected). Nil
	// when the adventure directory has no tiles/pack.json; legal as long as
	// no scene declares an Overrides entry — Compile's delegated call into
	// mapdef.BuildSceneCreated fails loud, through Resolve, the moment one
	// does and Pack is nil.
	Pack *mapdef.Pack

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
	TokenID, ActorID string
	X, Y             int32
}

// AdventureActor is one actors/*.json file: a complete statblock instance,
// validated at Load against the ruleset's declared vocabulary (attribute
// names against the union of declared attributes+defenses — a defense's
// value is carried the same way an attribute's is, per the ruleset
// v2 convention; resource names against the ruleset's declared resources).
// Controller is deliberately absent from this shape (and left unset on the
// compiled Actor): the DM drives every adventure-placed actor at load time;
// a player can be given control later via the existing actor-control
// mechanism (spec §4).
//
// KIND IS NOT ABSENT, and the contrast with Controller is the point. Who
// drives a character is a fact about the table, and the table does not exist
// yet when the file is written. What a creature IS is a fact about the
// content, decided by the author at the moment they wrote it — so the file is
// exactly the right place to say it, and Load REFUSES an actor that does not
// (load.go's actorKinds, and loadActors' own comment for why absence is not
// given a default here the way an absent tiles map is).
//
// The JSON field is "kind" and its two values are "party_member" and
// "non_party" (load.go's actorKinds). This field carries the resolved wire
// enum rather than the authored string: the mapping is checked once, at load,
// so nothing downstream has to re-parse a name or invent a fallback for one
// it does not recognise.
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
