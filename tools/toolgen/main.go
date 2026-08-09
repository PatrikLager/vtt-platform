// Command toolgen derives MCP tool definitions from the contract's protobuf
// descriptors. proto3 `optional` fields (synthetic oneofs) are omitted from
// each tool's `required` list — this is the contract's optionality annotation
// (ADR-007). Output is committed at contract/gen/tools/tools.json and covered
// by the drift gate.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/reflect/protoreflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// fieldOverride is manifest-level authoring guidance for one proto message
// type's generated JSON Schema, applied wherever that message type is
// reached during schemaFor's recursion (keyed by message full name, not by
// nesting path — see toolSpec.overrides and schemaForWithOverrides).
// requiredOverride, when non-nil, REPLACES the derived `required` list
// (proto3's own optionality annotation — ADR-007's synthetic-oneof rule —
// otherwise decides it) entirely; fieldDocs adds a per-property JSON
// Schema "description" for fields named in it. Both exist for exactly the
// case proto3 itself cannot express: a field that is technically
// non-optional on the wire but almost always meant to be omitted by an LLM
// caller, who will otherwise fabricate a value rather than send nothing —
// the add_actor fix (final review Fix 2) is the first user.
type fieldOverride struct {
	requiredOverride []string
	fieldDocs        map[string]string
}

type toolSpec struct {
	message     string
	name        string
	description string
	descriptor  protoreflect.MessageDescriptor
	// overrides maps a proto message full name (e.g. "vtt.v1.Actor") to
	// authoring guidance for that message's generated schema — nil for
	// every tool that needs none. See fieldOverride's doc comment.
	overrides map[protoreflect.FullName]fieldOverride
}

var manifest = []toolSpec{
	{
		message:     "vtt.v1.MoveTokenRequest",
		name:        "move_token",
		description: "Move a token to a new grid position on its scene.",
		descriptor:  (&vttv1.MoveTokenRequest{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.CreateScene",
		name:        "create_scene",
		description: "Create a new scene with a grid.",
		descriptor:  (&vttv1.CreateScene{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.AddActor",
		name:        "add_actor",
		description: "Add an actor to the campaign.",
		descriptor:  (&vttv1.AddActor{}).ProtoReflect().Descriptor(),
		// The fabrication trap (final review Fix 2): none of Actor's
		// fields are proto3 `optional`, so the derived required list
		// forces every one of them — an LLM caller then has no way to
		// tell "must supply" from "the field just happens to be
		// non-optional on the wire", and fabricates a plausible-looking
		// moduleId/controllerId/etc. rather than sending nothing. Only
		// actorId is genuinely required to add an actor at all.
		overrides: map[protoreflect.FullName]fieldOverride{
			"vtt.v1.Actor": {
				requiredOverride: []string{"actorId"},
				fieldDocs: map[string]string{
					"name":          "Optional display label for the actor.",
					"controllerId":  "Omit or empty = DM/agent-controlled; set a participant id to hand control to a player. Use EITHER this or controllerIds, never both — if they disagree, controllerIds wins and this is overwritten with its first entry.",
					"controllerIds": "Optional; omit. Authoritative if set — controllerId is derived from it, so do not set both. The full set of participants who may act as this actor — control is normally granted AFTER creation with grant_actor_control, not seeded here. Setting it seeds the set; leaving it empty means DM/agent only, exactly as an empty controllerId does.",
					"moduleId":      "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
					"attributes":    "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
					"resources":     "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
					"moduleData":    "Optional; omit unless a rule module instructs otherwise — moduleData is opaque.",
				},
			},
		},
	},
	{
		message:     "vtt.v1.PlaceToken",
		name:        "place_token",
		description: "Place an actor's token on a scene's grid.",
		descriptor:  (&vttv1.PlaceToken{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.StartSession",
		name:        "start_session",
		description: "Start a new play session.",
		descriptor:  (&vttv1.StartSession{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.EndSession",
		name:        "end_session",
		description: "End the current play session.",
		descriptor:  (&vttv1.EndSession{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.RetractEvents",
		name:        "retract_events",
		description: "Retract a range of events from the record with a stated reason.",
		descriptor:  (&vttv1.RetractEvents{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.UseAbility",
		name:        "use_ability",
		description: "Use one of the loaded ruleset's abilities as an actor against explicit targets.",
		descriptor:  (&vttv1.UseAbility{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.RemoveCondition",
		name:        "remove_condition",
		description: "Remove a named condition from an actor (DM-ended durations).",
		descriptor:  (&vttv1.RemoveCondition{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.AddNarration",
		name:        "add_narration",
		description: "Add a story entry to the table's shared narrative — narration, in-character speech (set `as`), or table talk; optionally anchored to the event sequences it narrates.",
		descriptor:  (&vttv1.AddNarration{}).ProtoReflect().Descriptor(),
		// The fabrication trap (same shape as add_actor's fix above): none
		// of AddNarration's fields are proto3 `optional`, so the derived
		// required list would force `as` and both anchor fields even
		// though spec §3 documents all three as optional — an LLM caller
		// then fabricates a speaker label or an anchor pair rather than
		// sending nothing. A fabricated backward anchor pair passes ALL
		// engine validation (internal/engine/apply.go) and persists
		// forever in the append-only log. Only text is genuinely required
		// to narrate at all. Unlike add_actor's override (keyed on the
		// nested "vtt.v1.Actor" message reached via AddActor.actor),
		// AddNarration's fields are direct fields on AddNarration itself,
		// so this override is keyed on "vtt.v1.AddNarration".
		overrides: map[protoreflect.FullName]fieldOverride{
			"vtt.v1.AddNarration": {
				requiredOverride: []string{"text"},
				fieldDocs: map[string]string{
					"as":            "Optional speaker label — set to voice an NPC or PC in character; omit to speak as yourself (table talk).",
					"anchorFromSeq": "Optional; omit (0) if this narration is unanchored. Set together with anchorToSeq as a backward-pointing range (both > 0, anchorFromSeq <= anchorToSeq, anchorToSeq before this narration's own sequence) — never set one without the other.",
					"anchorToSeq":   "Optional; omit (0) if this narration is unanchored. Set together with anchorFromSeq as a backward-pointing range (both > 0, anchorFromSeq <= anchorToSeq, anchorToSeq before this narration's own sequence) — never set one without the other.",
				},
			},
		},
	},
	{
		message:     "vtt.v1.UpsertNote",
		name:        "upsert_note",
		description: "Create or replace a keyed world note (locations, NPCs, quest state) — the campaign's durable memory.",
		descriptor:  (&vttv1.UpsertNote{}).ProtoReflect().Descriptor(),
		// Same shape, lower stakes: title is not proto3 `optional` either,
		// so the derived required list would force it even though an
		// empty title is adjudicated-legal (the engine only enforces a max
		// — internal/engine/apply.go's NoteUpserted case). Keyed on
		// "vtt.v1.UpsertNote" itself for the same direct-field reason as
		// AddNarration above.
		overrides: map[protoreflect.FullName]fieldOverride{
			"vtt.v1.UpsertNote": {
				requiredOverride: []string{"key", "text"},
				fieldDocs: map[string]string{
					"title": "Optional; may be empty.",
				},
			},
		},
	},
	{
		message:     "vtt.v1.DeleteNote",
		name:        "delete_note",
		description: "Delete a world note by key.",
		descriptor:  (&vttv1.DeleteNote{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.LoadAdventure",
		name:        "load_adventure",
		description: "Load a prepared adventure into the campaign — compiles its scenes, statblocks, notes, and opening narration into one atomic batch of setup events. DM/agent only.",
		descriptor:  (&vttv1.LoadAdventure{}).ProtoReflect().Descriptor(),
		// requiredOverride check (the fabrication-trap lesson — see
		// add_actor/add_narration above): LoadAdventure has exactly one
		// field, adventure_id, and it is genuinely required — there is no
		// way to load "an" adventure without naming which one, unlike
		// add_actor's controllerId or add_narration's anchors, which are
		// legitimately omittable. The derived required list (["adventureId"],
		// since the field is not proto3 `optional`) is already semantically
		// honest, so no override is needed here.
	},
	{
		message:     "vtt.v1.GrantActorControl",
		name:        "grant_actor_control",
		description: "Give a participant control of an actor, so they may move its token and use its abilities. Control is a SET — granting does not take control away from anyone who already has it, and an actor may be controlled by several participants at once. DM/agent only. Note the DM never needs this to act: DM authority is independent of control, so a DM already moves and uses any actor while a player controls it.",
		descriptor:  (&vttv1.GrantActorControl{}).ProtoReflect().Descriptor(),
		// Both fields are genuinely required — there is no meaning to
		// granting control of nothing, or to nobody — so the derived
		// required list is honest and needs no override (same check as
		// load_adventure above).
	},
	{
		message:     "vtt.v1.RevokeActorControl",
		name:        "revoke_actor_control",
		description: "Take a participant out of an actor's control set. Used to REASSIGN a character — when a player leaves the table for good, or a character changes hands — never to let the DM act, which needs no revocation. DM/agent may revoke anyone; a player may revoke only themselves.",
		descriptor:  (&vttv1.RevokeActorControl{}).ProtoReflect().Descriptor(),
	},
	{
		message:     "vtt.v1.PromoteParticipant",
		name:        "promote_participant",
		description: "Change what a participant is ALLOWED to do. Someone who joined through the shared link arrives as a spectator and can only watch; promote them to \"player\" so they can be given a character and act. role accepts ONLY \"player\" or \"spectator\" — promotion can never reach dm or agent, because the join link would otherwise be a route to full authority in two steps. Minting a DM is a deliberate out-of-band act (`vtt invite`). This changes IDENTITY, not the campaign, so it writes no event.",
		descriptor:  (&vttv1.PromoteParticipant{}).ProtoReflect().Descriptor(),
	},
}

func isOptional(f protoreflect.FieldDescriptor) bool {
	oo := f.ContainingOneof()
	return oo != nil && oo.IsSynthetic()
}

// schemaForWithOverrides is schemaFor plus overrides: a proto message full
// name -> fieldOverride map (see fieldOverride's doc comment), threaded
// through every recursive call so a message type reached at ANY nesting
// depth (not just top-level) picks up its own entry, if any, by full name.
func schemaForWithOverrides(md protoreflect.MessageDescriptor, overrides map[protoreflect.FullName]fieldOverride) map[string]any {
	props := map[string]any{}
	required := []any{}
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		name := f.JSONName()
		if f.IsMap() {
			props[name] = map[string]any{
				"type":                 "object",
				"additionalProperties": valueSchemaWithOverrides(f.MapValue(), overrides),
			}
			if !isOptional(f) {
				required = append(required, name)
			}
			continue
		}
		if f.IsList() {
			props[name] = map[string]any{"type": "array", "items": valueSchemaWithOverrides(f, overrides)}
			if !isOptional(f) {
				required = append(required, name)
			}
			continue
		}
		props[name] = valueSchemaWithOverrides(f, overrides)
		if !isOptional(f) {
			required = append(required, name)
		}
	}

	if ov, ok := overrides[md.FullName()]; ok {
		if ov.requiredOverride != nil {
			required = make([]any, len(ov.requiredOverride))
			for i, name := range ov.requiredOverride {
				required[i] = name
			}
		}
		for fieldName, doc := range ov.fieldDocs {
			propSchema, ok := props[fieldName].(map[string]any)
			if !ok {
				panic(fmt.Sprintf("toolgen: fieldDocs names %q, not a property of %s", fieldName, md.FullName()))
			}
			propSchema["description"] = doc
		}
	}

	return map[string]any{"type": "object", "properties": props, "required": required}
}

// schemaFor derives md's JSON Schema with no manifest overrides applied — the
// plain recursive builder, used by main_test.go to check this package's
// structural handling (arrays, Struct) in isolation from any tool's authoring
// guidance. Production paths always go through schemaForWithOverrides.
func schemaFor(md protoreflect.MessageDescriptor) map[string]any {
	return schemaForWithOverrides(md, nil)
}

// valueSchemaWithOverrides derives the JSON Schema for a single
// scalar/message value, threading overrides into any
// recursive schemaForWithOverrides call it makes — shared by plain fields,
// map values, and list items. google.protobuf.Struct is emitted as a bare
// open object since the contract never inspects module-owned data (see
// README: Struct rules).
func valueSchemaWithOverrides(f protoreflect.FieldDescriptor, overrides map[protoreflect.FullName]fieldOverride) map[string]any {
	switch f.Kind() {
	case protoreflect.StringKind:
		return map[string]any{"type": "string"}
	case protoreflect.BoolKind:
		return map[string]any{"type": "boolean"}
	case protoreflect.Int32Kind, protoreflect.Int64Kind:
		return map[string]any{"type": "integer"}
	case protoreflect.MessageKind:
		if f.Message().FullName() == "google.protobuf.Struct" {
			return map[string]any{"type": "object"}
		}
		return schemaForWithOverrides(f.Message(), overrides)
	default:
		panic(fmt.Sprintf("toolgen: unhandled kind %v on %s", f.Kind(), f.FullName()))
	}
}

func buildTools() []map[string]any {
	var tools []map[string]any
	for _, spec := range manifest {
		tools = append(tools, map[string]any{
			"name":        spec.name,
			"description": spec.description,
			"inputSchema": schemaForWithOverrides(spec.descriptor, spec.overrides),
		})
	}
	return tools
}

func main() {
	out := flag.String("o", "", "output path (default stdout)")
	flag.Parse()
	if err := run(*out, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run renders the tool definitions to outPath, or to w when outPath is empty.
// Split out of main so the rendering and write paths are reachable from a
// test: main itself is now a flag-parsing shim thin enough that nothing
// untested lives in it. It also replaces main's two panics with a returned
// error — a generator that fails should print a message and exit non-zero,
// not dump a stack trace into the build output.
func run(outPath string, w io.Writer) error {
	data, err := json.MarshalIndent(buildTools(), "", "  ")
	if err != nil {
		return fmt.Errorf("toolgen: marshal: %w", err)
	}
	data = append(data, '\n')
	if outPath == "" {
		_, err := w.Write(data)
		return err
	}
	// #nosec G306 -- tools.json is a COMMITTED, non-secret generated
	// artifact that the drift gate diffs and every developer reads; 0600
	// would break that for no benefit.
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("toolgen: write %s: %w", outPath, err)
	}
	return nil
}
