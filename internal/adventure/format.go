// Package adventure is the loader/compiler for the adventure format v1
// (adventure-format spec, sub-project 9 §4/§5): a directory of prepared
// content — scenes, statblock instances, revealed-world notes, an opening,
// and a secrets-bearing DM guide — written FOR a specific ruleset. Load
// reads and fully validates one such directory against a *rules.Ruleset;
// Compile turns a loaded Adventure into the ordinary setup events (spec
// §3) one atomic AppendBatch will apply — the platform gains no new
// runtime concept, an adventure simply becomes log history.
package adventure

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

	GuidePath string
}

// AdventureScene is one scenes/*.json file: a scene plus its token
// placements, in declared (array) order.
type AdventureScene struct {
	ID, Name     string
	GridW, GridH int32
	Placements   []Placement
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
type AdventureActor struct {
	ID, Name   string
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
