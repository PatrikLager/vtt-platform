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
                   attributes, resources, kind — "party_member" or
                   "non_party", REQUIRED; no controller, ever — control
                   is conferred only by a grant, which may also restate
                   kind)
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
versioning escape hatch load-bearing); **every actor must declare a
`kind` of `"party_member"` or `"non_party"` — absent or unrecognised is
rejected** (amended 2026-08-24, see below); statblock attribute/resource
names must be declared by the ruleset (defenses valued in attributes
per the v2 convention); resource current ≤ max when max > 0; note
keys/titles/texts and narration within the world layer's byte caps; all
placement actor_ids resolve within the adventure; scene/token/actor/
note ids must NOT collide with existing campaign state at load time
(checked against the live snapshot before the batch — rejection, not
overwrite).

**AMENDED 2026-08-24: actors declare a required `kind`.** Until now this
format could not say whether a creature was a player character or a monster,
and it ships both — `goblin-ambush` carries a Human Fighter beside two goblins,
`cellar-rats` carries Hollis Ketch and Mara Voss. That was the only creation
path in the system unable to state what it was making, while the runtime path
(`add_actor`) is now compelled to. Backwards, since authored content is written
deliberately, in advance, by someone who knows exactly what a creature is for.

**Required, not optional, and the difference matters.** An absent `kind` is not
a coherent default the way an absent `tiles` map is — it is a *wrong answer*
that would drop three of the five shipped actors off their own party's roster.
Silence is what made the visibility arc's leak reachable (visibility spec §5.1);
this format does not get to be silent.

**No controller, ever.** The old text said "controller unset — the DM drives; a
player can be given control later". Control is now conferred ONLY by a grant,
which may also restate kind — so there is nothing for this format to say about
control, and it is not merely unset but unsayable.

**`format_version` is NOT bumped, deliberately.** A required new key does
invalidate every v1 actor file, which normally forces a bump. It is skipped
because no v1 content exists outside this repository — Patrik, 2026-08-24: no
campaign or ruleset is in use by anyone — so the version would mark a boundary
nothing crossed, and the two shipped adventures are updated in the same commit.
**Recorded rather than assumed, so that a future format change starts from a
stated position rather than from this precedent read as a habit.** If content
ever ships outside this repo, the next such change bumps.

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
  clean "no adventures available"). All adventures FOR THE SERVED RULESET
  load+validate at BOOT (fail loud at startup, not at the table);
  adventures declaring another ruleset are skipped, and a directory that
  yields none for this table is still a boot error naming the ruleset
  (amended 2026-08-06 — the dir is a library that may hold books for
  several tables; per-adventure load still rejects a mismatch outright,
  §"Load-time validation" above, since asking for THAT adventure is a
  mistake).
- Gateway authz: load_adventure — dm/agent only (the table goes 12 → 13
  commands × 4 roles; corrected from a bare "13" 2026-08-25, see §8).
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
own pins; MCP e2e: the tool total, derived rather than written down;
load_adventure round-trip, guide served, no-dir clean error. Remote demo
before the merge gate (the 5b pattern): a fresh subagent DM with
guide-only knowledge loads goblin-ambush and plays the opening; event log
independently verified.

**AMENDED 2026-08-25: the e2e pins the tool total as a relationship, not a
count.** "17 tools" was a TARGET the day this was written, never a description
— §7 above records the surface as 15 on that same date — and it was stale by
the next contract addition. This number and the literal beside it in the test
were two copies of one fact, and neither could tell the other it had gone
wrong. `cmd/vtt`'s stdio e2e now computes the total: the command tools DERIVED
from the embedded `tools.json`, the hand-registered ones counted from the one
list `mcp.GoRegisteredToolNames` — declared rather than derived, deliberately,
since nothing generates them. A command added later is covered with nobody
editing a number. The e2e asserts the TOTAL; the names are pinned in
`internal/mcp`'s own ListTools test.

§7's "Tool count 15 → 17" stands: it records the delta THIS sub-project made,
which is history, not a claim about today. Its neighbour did not stand — "13
commands × 4 roles" was a bare snapshot of the authz table, stale by nine, and
is corrected to the delta it meant.

And what made any of this findable was NOT the copies disagreeing. They agreed
with each other perfectly — 13 command tools, 13+3 = 16, 13+4 = 17, a
consistent arithmetic that read like corroboration — and every one of them
disagreed with the code, which had 22. Agreeing copies are the harder case,
because each one confirms the others. The only thing that catches them is
re-running the sentence against the thing it describes.

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
