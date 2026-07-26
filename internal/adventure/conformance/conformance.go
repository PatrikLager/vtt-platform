// Package conformance is the adventure-format analog of
// internal/rules/conformance (adventure-format spec §6, task-12-3-brief.md):
// Run validates that an adventure directory conforms to the ruleset it
// itself declares — resolves and loads that DECLARED ruleset from a
// rulesets root, loads and fully validates the adventure against it
// (internal/adventure.Load), compiles it against a brand-new, EMPTY
// campaign state (internal/adventure.Compile against engine.NewState()),
// and compares the exact resulting envelope batch against a hand-derived
// golden fixture (<adventureDir>/goldens/compiled-batch.json). Run also
// requires a non-empty guide.md (spec §6's own binding, distinct from
// rules.Load's guide.md-must-exist-but-may-be-empty convention). Run knows
// nothing about any specific adventure's content: the SAME
// Run(adventureDir, rulesetsRoot) call gates cellar-rats, goblin-ambush, or
// any future adventure committed under adventures/ — this genericity, not
// any one adventure's content, is the P4 proof, exactly as
// internal/rules/conformance.Run is for rulesets.
//
// # Compiled-batch golden format (owned by this package, not part of the
// adventure format itself — see internal/adventure/format.go/load.go for
// that)
//
// <adventureDir>/goldens/compiled-batch.json is a top-level JSON ARRAY, one
// entry per envelope Compile emits, in Compile's own binding order
// (AdventureLoaded first; scenes; actors; placements; notes; opening
// narration last — see internal/adventure/compile.go's doc comment). Each
// entry is a "type"-tagged union with exactly one of the following six
// sibling objects populated, matching that "type" value:
//
//	{"type": "adventure_loaded", "adventure_loaded": {"adventure_id": "...", "name": "..."}}
//	{"type": "scene_created", "scene_created": {"scene_id": "...", "name": "...", "grid_width": 0, "grid_height": 0}}
//	{"type": "actor_added", "actor_added": {"actor_id": "...", "name": "...",
//	  "attributes": {"...": 0}, "resources": {"...": {"current": 0, "max": 0}}}}
//	{"type": "token_placed", "token_placed": {"token_id": "...", "scene_id": "...", "actor_id": "...", "x": 0, "y": 0}}
//	{"type": "note_upserted", "note_upserted": {"key": "...", "title": "...", "text": "..."}}
//	{"type": "narration_added", "narration_added": {"text": "..."}}
//
// Envelope metadata (event_id/sequence/occurred_at/session_id/actor_role/
// participant_id) is deliberately absent — Compile itself never sets those
// fields (they are stamped later by whatever calls campaign.AppendBatch),
// so a golden pinning them would pin nothing meaningful.
package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// Run validates the adventure directory at adventureDir against the
// ruleset it declares, resolved as filepath.Join(rulesetsRoot,
// <declared-ruleset-id>). A nil return means adventureDir is a fully
// conformant adventure: its declared ruleset loads cleanly, the adventure
// itself loads and validates against that ruleset (internal/adventure.Load),
// its guide.md is non-empty, and compiling it against a brand-new
// engine.NewState() reproduces EXACTLY the batch recorded at
// goldens/compiled-batch.json.
func Run(adventureDir, rulesetsRoot string) error {
	rulesetID, err := peekRulesetID(adventureDir)
	if err != nil {
		return fmt.Errorf("conformance: %s: %w", adventureDir, err)
	}

	rs, err := rules.Load(filepath.Join(rulesetsRoot, rulesetID))
	if err != nil {
		return fmt.Errorf("conformance: %s: load declared ruleset %q: %w", adventureDir, rulesetID, err)
	}

	adv, err := adventure.Load(adventureDir, rs)
	if err != nil {
		return fmt.Errorf("conformance: %s: %w", adventureDir, err)
	}

	if err := checkGuide(adv.GuidePath); err != nil {
		return fmt.Errorf("conformance: %s: %w", adventureDir, err)
	}

	envs, err := adventure.Compile(adv, engine.NewState())
	if err != nil {
		return fmt.Errorf("conformance: %s: compile: %w", adventureDir, err)
	}

	if err := checkCompiledBatchGolden(adventureDir, envs); err != nil {
		return fmt.Errorf("conformance: %s: %w", adventureDir, err)
	}
	return nil
}

// --- declared-ruleset resolution ---

// manifestPeek decodes only the one field Run needs before a *rules.Ruleset
// even exists to pass to adventure.Load: which ruleset directory to resolve
// under rulesetsRoot. This is deliberately NOT strict decoding (unlike
// adventure.Load's own manifest handling) — it exists solely to answer "which
// ruleset", not to validate the manifest; adventure.Load performs the real,
// strict, unknown-fields-rejecting decode of the exact same file once rs is
// available.
type manifestPeek struct {
	Ruleset string `json:"ruleset"`
}

func peekRulesetID(adventureDir string) (string, error) {
	path := filepath.Join(adventureDir, "adventure.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	var m manifestPeek
	if err := json.Unmarshal(b, &m); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if m.Ruleset == "" {
		return "", fmt.Errorf("%s: field %q: must not be empty", path, "ruleset")
	}
	return m.Ruleset, nil
}

// --- guide.md ---

// checkGuide enforces spec §6's own binding: an adventure's guide.md must be
// non-empty. This is stricter than rules.Load's own guide.md handling (which
// only requires the file to exist and be readable, not to carry any
// content) — an adventure's guide is where the beats/secrets live (spec
// §4), so an empty one is never a legitimate adventure, only a forgotten
// file.
func checkGuide(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("guide.md: %w", err)
	}
	if len(b) == 0 {
		return fmt.Errorf("guide.md: must not be empty")
	}
	return nil
}

// --- compiled-batch golden ---

func checkCompiledBatchGolden(adventureDir string, envs []*vttv1.Envelope) error {
	path := filepath.Join(adventureDir, "goldens", "compiled-batch.json")
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("missing compiled-batch golden %s: %w", path, err)
	}

	dec := json.NewDecoder(bytes.NewReader(wantBytes))
	dec.DisallowUnknownFields()
	var want []envelopeDump
	if err := dec.Decode(&want); err != nil {
		return fmt.Errorf("compiled-batch golden %s: decode: %w", path, err)
	}

	got, err := toBatchDump(envs)
	if err != nil {
		return fmt.Errorf("compiled-batch golden %s: %w", path, err)
	}

	if len(got) != len(want) {
		gotBytes, _ := json.MarshalIndent(got, "", "  ")
		return fmt.Errorf("compiled-batch golden %s: got %d envelopes, want %d\ngot:\n%s\nwant:\n%s",
			path, len(got), len(want), gotBytes, wantBytes)
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			gotOne, _ := json.MarshalIndent(got[i], "", "  ")
			wantOne, _ := json.MarshalIndent(want[i], "", "  ")
			return fmt.Errorf("compiled-batch golden %s: envelope[%d] does not match (first differing index)\ngot:\n%s\nwant:\n%s",
				path, i, gotOne, wantOne)
		}
	}
	return nil
}

// DumpCompiledBatch renders envs as the canonical JSON serialization
// compiled-batch goldens pin (this package doc's own union format) — the
// dump helper for authoring: derive a golden by hand FIRST (ADR-009), then
// load the real adventure, Compile it, run this over the result, and use it
// only to VERIFY the hand-derivation, never to generate a golden no human
// derived first.
func DumpCompiledBatch(envs []*vttv1.Envelope) ([]byte, error) {
	dump, err := toBatchDump(envs)
	if err != nil {
		return nil, fmt.Errorf("conformance: dump compiled batch: %w", err)
	}
	b, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("conformance: dump compiled batch: %w", err)
	}
	return append(b, '\n'), nil
}

func toBatchDump(envs []*vttv1.Envelope) ([]envelopeDump, error) {
	out := make([]envelopeDump, len(envs))
	for i, e := range envs {
		d, err := toEnvelopeDump(e)
		if err != nil {
			return nil, fmt.Errorf("envelope[%d]: %w", i, err)
		}
		out[i] = d
	}
	return out, nil
}

// envelopeDump is one goldens/compiled-batch.json array entry: a
// "type"-tagged union over Compile's six possible payload kinds (spec §5),
// exactly one of the pointer fields populated per entry, matching Type.
type envelopeDump struct {
	Type string `json:"type"`

	AdventureLoaded *adventureLoadedDump `json:"adventure_loaded,omitempty"`
	SceneCreated    *sceneCreatedDump    `json:"scene_created,omitempty"`
	ActorAdded      *actorAddedDump      `json:"actor_added,omitempty"`
	TokenPlaced     *tokenPlacedDump     `json:"token_placed,omitempty"`
	NoteUpserted    *noteUpsertedDump    `json:"note_upserted,omitempty"`
	NarrationAdded  *narrationAddedDump  `json:"narration_added,omitempty"`
}

const (
	typeAdventureLoaded = "adventure_loaded"
	typeSceneCreated    = "scene_created"
	typeActorAdded      = "actor_added"
	typeTokenPlaced     = "token_placed"
	typeNoteUpserted    = "note_upserted"
	typeNarrationAdded  = "narration_added"
)

type adventureLoadedDump struct {
	AdventureID string `json:"adventure_id"`
	Name        string `json:"name"`
}

type sceneCreatedDump struct {
	SceneID    string `json:"scene_id"`
	Name       string `json:"name"`
	GridWidth  int32  `json:"grid_width"`
	GridHeight int32  `json:"grid_height"`
}

type actorAddedDump struct {
	ActorID    string                  `json:"actor_id"`
	Name       string                  `json:"name"`
	Attributes map[string]int32        `json:"attributes,omitempty"`
	Resources  map[string]resourceDump `json:"resources,omitempty"`
}

type resourceDump struct {
	Current int32 `json:"current"`
	Max     int32 `json:"max"`
}

type tokenPlacedDump struct {
	TokenID string `json:"token_id"`
	SceneID string `json:"scene_id"`
	ActorID string `json:"actor_id"`
	X       int32  `json:"x"`
	Y       int32  `json:"y"`
}

type noteUpsertedDump struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type narrationAddedDump struct {
	Text string `json:"text"`
}

func toEnvelopeDump(env *vttv1.Envelope) (envelopeDump, error) {
	switch p := env.Payload.(type) {
	case *vttv1.Envelope_AdventureLoaded:
		al := p.AdventureLoaded
		return envelopeDump{Type: typeAdventureLoaded, AdventureLoaded: &adventureLoadedDump{
			AdventureID: al.AdventureId, Name: al.Name,
		}}, nil

	case *vttv1.Envelope_SceneCreated:
		sc := p.SceneCreated
		return envelopeDump{Type: typeSceneCreated, SceneCreated: &sceneCreatedDump{
			SceneID: sc.SceneId, Name: sc.Name, GridWidth: sc.GridWidth, GridHeight: sc.GridHeight,
		}}, nil

	case *vttv1.Envelope_ActorAdded:
		return envelopeDump{Type: typeActorAdded, ActorAdded: toActorAddedDump(p.ActorAdded.Actor)}, nil

	case *vttv1.Envelope_TokenPlaced:
		tp := p.TokenPlaced
		d := tokenPlacedDump{TokenID: tp.TokenId, SceneID: tp.SceneId, ActorID: tp.ActorId}
		if tp.Position != nil {
			d.X, d.Y = tp.Position.X, tp.Position.Y
		}
		return envelopeDump{Type: typeTokenPlaced, TokenPlaced: &d}, nil

	case *vttv1.Envelope_NoteUpserted:
		nu := p.NoteUpserted
		return envelopeDump{Type: typeNoteUpserted, NoteUpserted: &noteUpsertedDump{
			Key: nu.Key, Title: nu.Title, Text: nu.Text,
		}}, nil

	case *vttv1.Envelope_NarrationAdded:
		return envelopeDump{Type: typeNarrationAdded, NarrationAdded: &narrationAddedDump{
			Text: p.NarrationAdded.Text,
		}}, nil

	default:
		return envelopeDump{}, fmt.Errorf("unsupported envelope payload type %T (compiled-batch goldens cover only what Compile emits)", env.Payload)
	}
}

func toActorAddedDump(a *vttv1.Actor) *actorAddedDump {
	d := &actorAddedDump{ActorID: a.ActorId, Name: a.Name}
	if len(a.Attributes) > 0 {
		d.Attributes = make(map[string]int32, len(a.Attributes))
		for k, v := range a.Attributes {
			d.Attributes[k] = v
		}
	}
	if len(a.Resources) > 0 {
		d.Resources = make(map[string]resourceDump, len(a.Resources))
		for k, v := range a.Resources {
			d.Resources[k] = resourceDump{Current: v.Current, Max: v.Max}
		}
	}
	return d
}
