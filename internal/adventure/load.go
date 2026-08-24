package adventure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Size/anchor limits mirroring the world layer's caps (internal/engine/
// apply.go's unexported maxNoteKeyBytes/maxNoteTitleBytes/maxTextBytes:
// 128/256/8192). engine does not export those constants, so this package
// cannot import and compare against them directly — they are MIRRORED here
// as local literals instead, with format_test.go's TestSizeCapsMirrorEngine
// pinning the exact literal values (128/256/8192) so a change to either
// side is at least visible as a failing assertion naming both constants,
// even though no compiler-enforced link between the two packages exists.
// If engine's caps ever change, a human must re-sync both sides by hand.
const (
	maxNoteKeyBytes   = 128
	maxNoteTitleBytes = 256
	maxTextBytes      = 8192 // shared by note text AND opening narration, exactly as engine's maxTextBytes is shared by note text and NarrationAdded.text
)

// supportedFormatVersions is the set of format_version values Load accepts
// for an adventure manifest (spec §4: "format_version \"1\""). Mirrors
// internal/rules/load.go's own supportedFormatVersions — the ruleset
// format's load-bearing version switch — so this sibling loader rejects an
// absent, empty, or arbitrary value (fix-wave F1) instead of silently
// loading under whatever semantics happen to be current, the same
// discipline that let the ruleset format retire v1 cleanly when v2 arrived
// (docs/superpowers/plans/2026-07-25-format-v2-composition.md). Only "1"
// exists today; a future adventure format v2 adds its value here.
var supportedFormatVersions = map[string]bool{"1": true}

// Load reads and fully validates the adventure directory at dir against rs
// (the already-loaded ruleset the adventure declares itself written for):
// strict JSON decoding of adventure.json/scenes/*.json/actors/*.json/
// notes/*.json (no unknown fields tolerated), the ruleset-id match, every
// statblock's attribute/resource vocabulary against rs, every world-layer
// byte cap, every id/token uniqueness rule WITHIN the adventure, and every
// placement's actor reference and grid-bounds sanity. Every error names the
// offending file and field; nothing is left half-loaded on any failure —
// Load returns (nil, err) as soon as the first violation is found.
func Load(dir string, rs *rules.Ruleset) (*Adventure, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("adventure: load %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("adventure: load %s: not a directory", dir)
	}

	manifest, err := loadManifest(filepath.Join(dir, "adventure.json"))
	if err != nil {
		return nil, err
	}
	if manifest.Ruleset != rs.ID {
		// The existing wording is preserved verbatim — it is what a boot
		// failure prints — and ErrRulesetMismatch is appended so a caller
		// reading a MULTI-TABLE directory can skip this adventure without
		// matching on the sentence. Every other error from Load means the
		// adventure is malformed and must still fail loud.
		return nil, fmt.Errorf("adventure: %s: field %q: declares ruleset %q, but the served ruleset is %q (%w)",
			filepath.Join(dir, "adventure.json"), "ruleset", manifest.Ruleset, rs.ID, ErrRulesetMismatch)
	}
	if len(manifest.OpeningNarration) == 0 {
		return nil, fieldErr(filepath.Join(dir, "adventure.json"), "opening_narration", "must not be empty")
	}
	if len(manifest.OpeningNarration) > maxTextBytes {
		return nil, fieldErr(filepath.Join(dir, "adventure.json"), "opening_narration",
			fmt.Sprintf("must be at most %d bytes, got %d", maxTextBytes, len(manifest.OpeningNarration)))
	}

	// attrOrDefSet: a defense's value is exposed through the same
	// attribute-shaped vocabulary a statblock declares (v2 convention,
	// mirroring internal/rules/compile.go's attrOrDefSet) — a statblock's
	// "attributes" map may therefore legally name either an attribute or a
	// defense.
	attrOrDefSet := make(map[string]bool, len(rs.Attributes)+len(rs.Defenses))
	for _, a := range rs.Attributes {
		attrOrDefSet[a] = true
	}
	for _, d := range rs.Defenses {
		attrOrDefSet[d] = true
	}
	resSet := make(map[string]bool, len(rs.Resources))
	for _, r := range rs.Resources {
		resSet[r.Name] = true
	}

	actors, actorIDs, err := loadActors(filepath.Join(dir, "actors"), attrOrDefSet, resSet)
	if err != nil {
		return nil, err
	}

	// Loaded BEFORE scenes: loadScenes needs pack in hand to prove each
	// scene's terrain actually resolves (fail loud at boot — see
	// loadScenes' own doc comment for why this cannot wait for Compile).
	pack, err := loadEmbeddedPack(dir)
	if err != nil {
		return nil, err
	}

	scenes, err := loadScenes(filepath.Join(dir, "scenes"), actorIDs, pack)
	if err != nil {
		return nil, err
	}

	notes, err := loadNotes(filepath.Join(dir, "notes"))
	if err != nil {
		return nil, err
	}

	if len(scenes) == 0 && len(actors) == 0 && len(notes) == 0 {
		return nil, fmt.Errorf("adventure: %s: empty adventure — no scenes, actors, or notes (a manifest alone is a mistake)", dir)
	}

	return &Adventure{
		ID:               manifest.ID,
		Name:             manifest.Name,
		RulesetID:        manifest.Ruleset,
		OpeningNarration: manifest.OpeningNarration,
		Scenes:           scenes,
		Actors:           actors,
		Notes:            notes,
		Pack:             pack,
		GuidePath:        filepath.Join(dir, "guide.md"),
	}, nil
}

// loadEmbeddedPack loads dir/tiles/pack.json if present — mirrors a
// standalone map's own art-pack directory convention (maps-as-geometry
// implementation plan, Task 10: "maps/cellar/tiles/pack.json"), embedded
// rather than referenced by id because the adventure format is
// self-contained (adventure-format spec §2.2: "No bestiary format" — shared
// content libraries were rejected). Absence is legal and the common case:
// most adventures use only standard tiles and declare no Overrides at all,
// so nothing ever needs a pack to resolve against.
func loadEmbeddedPack(dir string) (*mapdef.Pack, error) {
	packDir := filepath.Join(dir, "tiles")
	if _, err := os.Stat(filepath.Join(packDir, "pack.json")); err != nil {
		if os.IsNotExist(err) {
			// (nil, nil) is the correct, intentional result here — not the
			// ambiguous API smell nilnil normally catches: this is an
			// unexported helper with exactly one caller (Load, immediately
			// above), which stores the result straight into Adventure.Pack
			// and documents nil as a legal, expected value. There is no
			// caller anywhere who could mistake "no error" for "safe to
			// dereference".
			//nolint:nilnil
			return nil, nil
		}
		return nil, fmt.Errorf("adventure: %s: %w", packDir, err)
	}
	return mapdef.LoadPack(packDir)
}

// --- adventure.json ---

type manifestJSON struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	FormatVersion    string `json:"format_version"`
	Ruleset          string `json:"ruleset"`
	OpeningNarration string `json:"opening_narration"`
}

func loadManifest(path string) (*manifestJSON, error) {
	var raw manifestJSON
	if err := decodeStrict(path, &raw); err != nil {
		return nil, err
	}
	if raw.ID == "" {
		return nil, fieldErr(path, "id", "must not be empty")
	}
	if raw.Name == "" {
		return nil, fieldErr(path, "name", "must not be empty")
	}
	if !supportedFormatVersions[raw.FormatVersion] {
		return nil, fieldErr(path, "format_version", fmt.Sprintf("unsupported value %q (want \"1\")", raw.FormatVersion))
	}
	if raw.Ruleset == "" {
		return nil, fieldErr(path, "ruleset", "must not be empty")
	}
	return &raw, nil
}

// --- actors/*.json (one statblock instance per file) ---

type actorJSON struct {
	ActorID string `json:"actor_id"`
	Name    string `json:"name"`
	// What this creature IS — see actorKinds below for the vocabulary and for
	// why an authored actor may not leave it out.
	Kind       string                     `json:"kind"`
	Attributes map[string]int32           `json:"attributes"`
	Resources  map[string]resourceValJSON `json:"resources"`
}

// actorKinds is the authored vocabulary for an actor's kind, and the whole of
// it: two names, mapping one-for-one onto the two ActorKind values that mean
// something. ACTOR_KIND_UNSPECIFIED has no name here ON PURPOSE — it is the
// absence this field exists to abolish, and giving it a spelling would hand an
// author a way to write the silence down and call it an answer.
//
// SHORT NAMES rather than the wire's ACTOR_KIND_ constants, following the
// terrain vocabulary this format already borrows (mapdef.StandardTile: a scene
// says "wall", not TILE_KIND_WALL). They are the wire names' own suffixes
// lowercased, so the mapping is legible in both directions rather than a
// second vocabulary for the same idea.
var actorKinds = map[string]vttv1.ActorKind{
	"party_member": vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	"non_party":    vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
}

// actorKindNames is actorKinds' key set in a stable order, for error messages
// — a map range would offer the two answers in a different order each run,
// which is a diff in every failure output and a coin toss in a golden.
// format_test.go's TestActorKindNamesAreExactlyTheVocabulary pins the two
// against each other, since this is the same fact written down twice.
var actorKindNames = []string{"party_member", "non_party"}

type resourceValJSON struct {
	Current int32 `json:"current"`
	Max     int32 `json:"max"`
}

// loadActors decodes every actors/*.json file (file-name order — spec §5's
// Compile order binding), validating each actor's declared KIND against the
// two-name vocabulary above, each statblock's attribute/resource vocabulary
// against the ruleset's declared names, and each resource's current<=max
// (when max>0). Returns the loaded actors in file-name order plus the set of
// actor ids declared, so loadScenes can validate placement actor references
// without a second directory read.
//
// "kind" is REQUIRED and has no default — the one field in this format whose
// absence is refused rather than interpreted. The reasoning is at the check
// itself, where the next person to wonder about it will be standing.
func loadActors(dir string, attrOrDefSet, resSet map[string]bool) ([]AdventureActor, map[string]bool, error) {
	paths, err := jsonFilesIn(dir)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	actorIDs := map[string]bool{}
	out := make([]AdventureActor, 0, len(paths))
	for _, path := range paths {
		var raw actorJSON
		if err := decodeStrict(path, &raw); err != nil {
			return nil, nil, err
		}
		if raw.ActorID == "" {
			return nil, nil, fieldErr(path, "actor_id", "must not be empty")
		}
		if raw.Name == "" {
			return nil, nil, fieldErr(path, "name", "must not be empty")
		}
		if seen[raw.ActorID] {
			return nil, nil, fieldErr(path, "actor_id", fmt.Sprintf("duplicate actor id %q", raw.ActorID))
		}
		seen[raw.ActorID] = true

		// AN AUTHORED ACTOR MUST SAY WHAT IT IS, and this is the one place in
		// the format where absence is refused rather than given a meaning
		// (actor-kind plan Task 7; visibility spec §5.1).
		//
		// It is the opposite ruling to "tiles is optional" (loadScenes' doc
		// comment) and the difference is what makes both right. An absent
		// tiles map MEANS something coherent — this scene has no terrain — and
		// a file written before terrain existed keeps loading. An absent kind
		// means "not a party member", which is not a coherent default but a
		// WRONG ANSWER for three of the five actors this repo ships: the Human
		// Fighter, Hollis Ketch and Mara Voss would each drop off their own
		// party's roster the moment they turned a corner, which is the exact
		// regression the visibility spec's §5 exception exists to prevent.
		//
		// And the author is the one person who KNOWS. Adventure content is
		// written deliberately, in advance, by someone who decided what each
		// creature is for; add_actor — the improvised path, an LLM DM
		// inventing a bandit mid-session — is already refused when it stays
		// silent (gateway's validateAddActor). The prepared path being the
		// only one that could not speak was backwards.
		//
		// Refused at LOAD, so it fails at boot rather than at the table
		// (adventure-format spec §7's posture, which every rule in this file
		// follows).
		kind, ok := actorKinds[raw.Kind]
		if !ok {
			if raw.Kind == "" {
				return nil, nil, fieldErr(path, "kind", fmt.Sprintf(
					"must say what this creature IS — %q for a character the party knows about, "+
						"%q for one they must discover by seeing it; there is no default, because "+
						"an unstated kind cannot be told from a deliberate one",
					actorKindNames[0], actorKindNames[1]))
			}
			return nil, nil, fieldErr(path, "kind", fmt.Sprintf(
				"unknown kind %q (want %q or %q)", raw.Kind, actorKindNames[0], actorKindNames[1]))
		}

		for name := range raw.Attributes {
			if !attrOrDefSet[name] {
				return nil, nil, fieldErr(path, "attributes", fmt.Sprintf("references undeclared attribute %q (not declared as an attribute or defense by the ruleset)", name))
			}
		}
		resources := make(map[string]ResourceVal, len(raw.Resources))
		for name, rv := range raw.Resources {
			if !resSet[name] {
				return nil, nil, fieldErr(path, "resources", fmt.Sprintf("references undeclared resource %q", name))
			}
			if rv.Max > 0 && rv.Current > rv.Max {
				return nil, nil, fieldErr(path, fmt.Sprintf("resources.%s.current", name), fmt.Sprintf("must not exceed max (%d > %d)", rv.Current, rv.Max))
			}
			resources[name] = ResourceVal(rv)
		}

		actorIDs[raw.ActorID] = true
		out = append(out, AdventureActor{
			ID: raw.ActorID, Name: raw.Name, Kind: kind,
			Attributes: raw.Attributes,
			Resources:  resources,
		})
	}
	return out, actorIDs, nil
}

// --- scenes/*.json (one scene per file, placements declared inline) ---

// sceneJSON mirrors mapdef's own mapJSON shape for Tiles/Overrides/Objects
// (maps-as-geometry spec §4.1) — a scene IS a map (spec §4.3) — so the two
// formats decode identically field-for-field; only Pack is absent (an
// adventure's art pack is embedded once for the whole adventure, not named
// per scene — see load.go's loadEmbeddedPack doc comment).
type sceneJSON struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	GridWidth  int32               `json:"grid_width"`
	GridHeight int32               `json:"grid_height"`
	Tiles      map[string]string   `json:"tiles"`
	Overrides  map[string]string   `json:"overrides"`
	Objects    []mapdef.ObjectJSON `json:"objects"`
	Placements []placementJSON     `json:"placements"`
}

type placementJSON struct {
	TokenID string `json:"token_id"`
	ActorID string `json:"actor_id"`
	X       int32  `json:"x"`
	Y       int32  `json:"y"`
}

// loadScenes decodes every scenes/*.json file (file-name order), validating
// grid sanity (width/height >= 1), the tiles/overrides/objects completeness
// and bounds checks a standalone map file gets (spec §4.4) — reused directly
// from internal/mapdef rather than re-implemented, via the exported Check*
// functions load.go (mapdef) itself calls. TILES IS OPTIONAL (Patrik's
// ruling, 2026-08-13): a scene with no "tiles" key has no terrain, exactly
// as before this format existed — this format is written by third parties
// and by an LLM, and an existing scene file must keep loading. Once tiles
// carries ANY entry, completeness applies in full (CheckEverySquarePresent
// itself carries this rule now); an overrides entry with no tiles at all is
// its own refusal (CheckOverridesRequireTiles — an override names art for a
// square whose nature tiles declares, so there is nothing to attach it to
// with no tiles present). Also checked: that every square actually RESOLVES
// against pack (below — bounds alone is not enough: an override naming an
// art the pack doesn't define is a shape-valid, content-invalid scene),
// each placement's actor reference against actorIDs, each placement's
// coordinates against its own scene's grid, scene-id/token-id uniqueness
// WITHIN the adventure (token ids are unique across ALL scenes, not just
// within one — tracked via a running set as scenes are processed in
// file-name order), and finally that no placement starts inside a wall
// (also reused from mapdef — the one check the old scene-plus-four-numbers
// format could never even express, and a no-op when tiles is empty: no
// terrain declared means nothing to stand inside).
func loadScenes(dir string, actorIDs map[string]bool, pack *mapdef.Pack) ([]AdventureScene, error) {
	paths, err := jsonFilesIn(dir)
	if err != nil {
		return nil, err
	}
	seenScene := map[string]bool{}
	seenToken := map[string]bool{}
	out := make([]AdventureScene, 0, len(paths))
	for _, path := range paths {
		var raw sceneJSON
		if err := decodeStrict(path, &raw); err != nil {
			return nil, err
		}
		if raw.ID == "" {
			return nil, fieldErr(path, "id", "must not be empty")
		}
		if raw.Name == "" {
			return nil, fieldErr(path, "name", "must not be empty")
		}
		if seenScene[raw.ID] {
			return nil, fieldErr(path, "id", fmt.Sprintf("duplicate scene id %q", raw.ID))
		}
		seenScene[raw.ID] = true
		if raw.GridWidth < 1 {
			return nil, fieldErr(path, "grid_width", fmt.Sprintf("must be >= 1, got %d", raw.GridWidth))
		}
		if raw.GridHeight < 1 {
			return nil, fieldErr(path, "grid_height", fmt.Sprintf("must be >= 1, got %d", raw.GridHeight))
		}

		errf := func(field, msg string) error { return fieldErr(path, field, msg) }
		if err := mapdef.CheckEverySquarePresent(raw.Tiles, raw.GridWidth, raw.GridHeight, errf); err != nil {
			return nil, err
		}
		if err := mapdef.CheckTilesInsideGrid(raw.Tiles, raw.GridWidth, raw.GridHeight, errf); err != nil {
			return nil, err
		}
		if err := mapdef.CheckTileNamesKnown(raw.Tiles, errf); err != nil {
			return nil, err
		}
		if err := mapdef.CheckOverridesInsideGrid(raw.Overrides, raw.GridWidth, raw.GridHeight, errf); err != nil {
			return nil, err
		}
		if err := mapdef.CheckOverridesRequireTiles(raw.Tiles, raw.Overrides, errf); err != nil {
			return nil, err
		}
		objects := make([]mapdef.Object, 0, len(raw.Objects))
		for _, oj := range raw.Objects {
			objects = append(objects, oj.ToObject())
		}
		if err := mapdef.CheckObjectFootprints(objects, raw.GridWidth, raw.GridHeight, errf); err != nil {
			return nil, err
		}

		// Every square must actually RESOLVE, not just satisfy the shape and
		// bounds checks above — spec §4.4's fuller promise. An Overrides
		// entry naming an art the pack doesn't define (or one at all, with
		// no pack loaded) would otherwise pass every check here and only
		// surface later, at Compile — "at the table" rather than "at boot"
		// (adventure-format spec §7's explicit posture, which every other
		// rule in this function already follows: this is the one gap that
		// did not exist before this task, because a scene had nothing to
		// resolve). Reuses the exact call compile.go's own delegation makes
		// (mapdef.BuildSceneCreated), discarding the result: a genuine dry
		// run of the one construction site this task exists to create, not
		// a second check that could drift from what Compile actually does.
		if _, _, err := mapdef.BuildSceneCreated(&mapdef.Map{
			ID: raw.ID, Name: raw.Name, GridW: raw.GridWidth, GridH: raw.GridHeight,
			Tiles: raw.Tiles, Overrides: raw.Overrides, Objects: objects,
		}, pack); err != nil {
			return nil, fieldErr(path, "overrides", err.Error())
		}

		placements := make([]Placement, 0, len(raw.Placements))
		for i, p := range raw.Placements {
			field := fmt.Sprintf("placements[%d]", i)
			if p.TokenID == "" {
				return nil, fieldErr(path, field+".token_id", "must not be empty")
			}
			if p.ActorID == "" {
				return nil, fieldErr(path, field+".actor_id", "must not be empty")
			}
			if seenToken[p.TokenID] {
				return nil, fieldErr(path, field+".token_id", fmt.Sprintf("duplicate token id %q", p.TokenID))
			}
			seenToken[p.TokenID] = true
			if !actorIDs[p.ActorID] {
				return nil, fieldErr(path, field+".actor_id", fmt.Sprintf("references unknown actor %q (not declared by any actors/*.json file)", p.ActorID))
			}
			if p.X < 0 || p.X >= raw.GridWidth {
				return nil, fieldErr(path, field+".x", fmt.Sprintf("must be within the scene grid [0,%d), got %d", raw.GridWidth, p.X))
			}
			if p.Y < 0 || p.Y >= raw.GridHeight {
				return nil, fieldErr(path, field+".y", fmt.Sprintf("must be within the scene grid [0,%d), got %d", raw.GridHeight, p.Y))
			}
			placements = append(placements, Placement(p))
		}

		mp := make([]mapdef.Placement, len(placements))
		for i, p := range placements {
			mp[i] = mapdef.Placement(p)
		}
		if err := mapdef.CheckPlacementsNotInWalls(mp, raw.Tiles, raw.GridWidth, raw.GridHeight, errf); err != nil {
			return nil, err
		}

		out = append(out, AdventureScene{
			ID: raw.ID, Name: raw.Name,
			GridW: raw.GridWidth, GridH: raw.GridHeight,
			Tiles: raw.Tiles, Overrides: raw.Overrides, Objects: objects,
			Placements: placements,
		})
	}
	return out, nil
}

// --- notes/*.json (each file's top-level value is an ARRAY of notes) ---

type noteJSON struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// loadNotes decodes every notes/*.json file as a JSON ARRAY of note objects
// (not one-note-per-file, unlike actors/scenes): the adventure-format
// spec's Compile-order binding names "notes (declared order)" — the SAME
// phrase used for a scene's placements array — distinct from the plain
// "file-name order" given for scenes/actors (which have no further
// within-file ordering dimension to name). Flattened result order is
// file-name order across files, then declared (array) order within each
// file. Validates key/title/text against the world-layer byte caps and
// note-key uniqueness WITHIN the adventure (across all files).
func loadNotes(dir string) ([]AdventureNote, error) {
	paths, err := jsonFilesIn(dir)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []AdventureNote
	for _, path := range paths {
		var raw []noteJSON
		if err := decodeStrictSlice(path, &raw); err != nil {
			return nil, err
		}
		for i, n := range raw {
			field := fmt.Sprintf("[%d]", i)
			if n.Key == "" {
				return nil, fieldErr(path, field+".key", "must not be empty")
			}
			if len(n.Key) > maxNoteKeyBytes {
				return nil, fieldErr(path, field+".key", fmt.Sprintf("must be at most %d bytes, got %d", maxNoteKeyBytes, len(n.Key)))
			}
			if len(n.Title) > maxNoteTitleBytes {
				return nil, fieldErr(path, field+".title", fmt.Sprintf("must be at most %d bytes, got %d", maxNoteTitleBytes, len(n.Title)))
			}
			if n.Text == "" {
				return nil, fieldErr(path, field+".text", "must not be empty")
			}
			if len(n.Text) > maxTextBytes {
				return nil, fieldErr(path, field+".text", fmt.Sprintf("must be at most %d bytes, got %d", maxTextBytes, len(n.Text)))
			}
			if seen[n.Key] {
				return nil, fieldErr(path, field+".key", fmt.Sprintf("duplicate note key %q", n.Key))
			}
			seen[n.Key] = true
			out = append(out, AdventureNote(n))
		}
	}
	return out, nil
}

// --- shared decode/error helpers (mirrors internal/rules/load.go's style) ---

// decodeStrict decodes the JSON file at path into v with unknown fields
// disallowed.
func decodeStrict(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("adventure: %s: %w", path, err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("adventure: %s: %w", path, err)
	}
	return nil
}

// decodeStrictSlice is decodeStrict for a top-level JSON array (notes/*.json's
// shape) — encoding/json.Decoder.DisallowUnknownFields still applies to each
// element's object fields.
func decodeStrictSlice(path string, v any) error {
	return decodeStrict(path, v)
}

// ErrRulesetMismatch marks the one load failure a caller may act on rather
// than surface: the adventure is well-formed but written for another table.
var ErrRulesetMismatch = errors.New("not written for the served ruleset")

// fieldErr builds a load error naming both the offending file and field.
func fieldErr(path, field, msg string) error {
	return fmt.Errorf("adventure: %s: field %q: %s", path, field, msg)
}

// jsonFilesIn returns the sorted list of *.json paths directly inside dir
// (non-recursive; dir not existing is not an error — an adventure with no
// scenes, for instance, may still be valid as long as it has actors or
// notes — see Load's empty-adventure check).
func jsonFilesIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("adventure: %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}
