# Adventure Format — Design Spec (sub-project 9)

**Date:** 2026-07-26
**Status:** Approved design (brainstorming output, Patrik remote)
**Parent:** the three-layer terminology binding from 5a (RULESET ≠
ADVENTURE ≠ campaign log) — this is the middle layer, the
Temple-of-Elemental-Evil vision. Consumes the world layer's primitives
(notes, narration) and the batch primitive. Pillars P1–P4 apply.

## 1. Purpose

Prepared content becomes loadable data. An ADVENTURE is a directory
(scenes, statblock instances, revealed-world notes, an opening, and a
secrets-bearing DM guide) written FOR a ruleset. Loading one compiles to
ORDINARY setup events in one atomic batch — the platform gains no new
runtime concept; an adventure simply *becomes* log history. After 9, an
LLM DM runs a prepared campaign: fetch the guide, load the module, play.

## 2. Decisions (locked in brainstorming, 2026-07-26)

1. **Load surface = DM tool call.** `vtt serve --adventures-dir <dir>`
   makes adventures available; the DM calls `load_adventure` when the
   table is ready. The compile lands as ONE `AppendBatch` —
   the batch primitive's second production caller.
2. **Statblocks are self-contained.** The adventure carries complete
   statblocks validated against its declared ruleset at load. No
   bestiary format; rulesets stay pure mechanics (bestiary = residue).
3. **Secrets live in the guide only.** The adventure guide is fetched by
   the DM via MCP (the `get_ruleset_guide` precedent) and NEVER enters
   the event log. World notes hold only REVEALED facts; the DM upserts
   new notes as the party discovers things. No read-visibility mechanism
   exists or is needed — players see only what the table knows.

## 3. Contract additions (minimal, additive)

- Command `LoadAdventure{adventure_id}` (+ ClientCommand oneof tag; MCP
  tool auto-appears). dm/agent only.
- Event `AdventureLoaded{adventure_id, name}` (+ Envelope oneof tag) —
  pure testimony (engine no-op, AbilityUsed's pattern), the batch's
  FIRST event, making the log self-describing about what was loaded.
  Every other event in the batch is EXISTING vocabulary: SceneCreated,
  ActorAdded, TokenPlaced, NoteUpserted, NarrationAdded (the opening).

## 4. Adventure format v1

```
adventures/<id>/
  adventure.json   manifest: id, name, format_version "1",
                   ruleset (required ruleset id), opening_narration
                   (text, ≤ 8 KiB — becomes a NarrationAdded)
  scenes/*.json    scene id, name, grid, and token placements
                   [{token_id, actor_id, x, y}]
  actors/*.json    complete statblock instances (actor_id, name,
                   attributes, resources; controller unset — the DM
                   drives; a player can be given control later via
                   the existing actor-control mechanism)
  notes/*.json     the initially-REVEALED world notes [{key, title, text}]
  guide.md         DM affordances + ALL secrets: beats, hidden rooms,
                   villain identity, when to reveal which note. Served
                   via get_adventure_guide; never in the log.
```

Load-time validation (every error names file+field; nothing persists on
any failure): manifest ruleset MUST equal the served ruleset's id
(mismatch = clean error); `format_version` must be present and equal
"1" — any other or missing value is rejected at load (amended
2026-07-26, merge gate: the rules-loader precedent applies, keeping the
versioning escape hatch load-bearing); statblock attribute/resource
names must be declared by the ruleset (defenses valued in attributes
per the v2 convention); resource current ≤ max when max > 0; note
keys/titles/texts and narration within the world layer's byte caps; all
placement actor_ids resolve within the adventure; scene/token/actor/
note ids must NOT collide with existing campaign state at load time
(checked against the live snapshot before the batch — rejection, not
overwrite).

## 5. Loader & compile (`internal/adventure`)

`Load(dir) (*Adventure, error)` — strict decoding + validation vs a
`*rules.Ruleset`. `Compile(adv, st *engine.State) ([]*Envelope, error)`
— deterministic (file-order by name; placements in declared order):
AdventureLoaded, then scenes, actors, placements, notes, opening
narration. Gateway handler: authz → Load'd-at-boot adventure lookup by
id (unknown id = clean error; no adventures dir = "no adventures
available") → Compile against the live snapshot → campaign.AppendBatch;
result carries first sequence. arch-lint: `adventure → {engine,
contract, rules, adventure}`; gateway and cmd gain adventure (amended
2026-07-26, merge gate: cmd's edge is forced by §7's boot-time
validation of `serve`/`mcp --adventures-dir`); nothing else may import
it.

## 6. The proof — conformance analog + two adventures

`internal/adventure/conformance`: runs any adventure dir against its
ruleset — load, validate, compile, and an EXACT compiled-batch golden
(fully deterministic — no dice in setup). Wired into the test suite for
`adventures/*` forever. Two committed adventures prove format-vs-content
independence (the P4 pattern): `adventures/cellar-rats/` for
tavern-brawl (toy — one scene, two brawlers, one note, a two-line
guide) and `adventures/goblin-ambush/` for dnd45e-minimal — the demo's
ravine ambush as REAL prepared content (fighter + both goblins placed,
ambush note, opening narration, a guide with the secret the archer
flees at half hp).

## 7. Wiring

- serve: `--adventures-dir` (optional; without it, load_adventure →
  clean "no adventures available"). All available adventures load+
  validate at BOOT (fail loud at startup, not at the table).
- Gateway authz: load_adventure — dm/agent only (13 commands × 4 roles).
- MCP: load_adventure auto-appears; `get_adventure_guide{adventure_id}`
  via `vtt mcp --adventures-dir` (startup snapshot, the ruleset-guide
  precedent; server-authoritative delivery stays residue). Tool count
  15 → 17.
- Harness: existing generic command step carries load_adventure; new
  library scenario `scenarios/adventure-night.json`: load goblin-ambush,
  probe the placed tokens/notes, play one deterministic beat, denial
  rows (player/spectator load_adventure; unknown id; double-load
  collision rejection).
- Undo: the adventure batch retracts as a range (existing machinery) —
  tested.

## 8. Testing (ADR-009 binding)

Stub-first behavioral RED; loader validation catalogue with one focused
invalid fixture per rule; compiled-batch goldens hand-derived; the
collision rule tested against live state (load twice = second rejected);
boot-time validation failure = serve exits nonzero with the file+field
error; property/scenario pins byte-identical except the new scenario's
own pins; MCP e2e: 17 tools, load_adventure round-trip, guide served,
no-dir clean error. Remote demo before the merge gate (the 5b pattern):
a fresh subagent DM with guide-only knowledge loads goblin-ambush and
plays the opening; event log independently verified.

## 9. Non-goals (YAGNI)

Staged/chaptered reveals (the guide tells the DM when — the DM upserts);
bestiary references; dm-only note visibility; assets/maps/images
(sub-project 9-assets, later); adventure hot-reload or unload
(retraction covers mistakes); multi-adventure simultaneous load
(sequential loads are legal if ids don't collide — that IS the
multi-module story); authoring tools; adventure-declared ability grants
(ability lists stay narrative discipline per the ruleset guide).

## 10. Open questions (deferred, with owners)

- Bestiary sharing across adventures — with the first author who has two
  adventures duplicating statblocks.
- Asset/map layer — its own sub-project (build-order item 9-assets).
- Server-authoritative guide delivery (one wire round-trip for both
  guide kinds) — with the client, which will want it anyway.
