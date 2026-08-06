# Adventure Format Implementation Plan (sub-project 9)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver sub-project 9 per the approved spec (`docs/superpowers/specs/2026-07-26-adventure-format-design.md`): the adventure format, loader/compiler, two committed adventures with a conformance analog, and full wiring — loading prepared content as one atomic batch of existing events.

**Architecture:** new package `internal/adventure` (Load + Compile; arch-lint `adventure → {engine, contract, rules, adventure}`); one new command + one testimony event; gateway handler = lookup → Compile vs live snapshot → AppendBatch; everything else is existing vocabulary.

**Tech Stack:** existing; stdlib-only.

## Global Constraints

- Branch `feat/adventure-format` from main. Adapted review-before-commit flow; reviewers READ-ONLY; injections/mutations ONLY in throwaway rsync copies (absolute; ledgered incidents).
- Contract additive only; Task 1 drift-caveat protocol. PRE-AUTHORIZED (learned twice): the tool-count pin tests update in Task 1 itself (15 → 16 with load_adventure; Task 4's get_adventure_guide makes 17 and updates them again) — keep the branch green at every commit.
- ADR-009 binding; vocabulary ban (adventure/scene/note are platform-generic; adventure CONTENT under adventures/ is content, unscanned — but internal/adventure code and fixtures stay generic, bare `hp` banned).
- toolgen requiredOverride discipline (the fabrication-trap lesson): LoadAdventure's single field is genuinely required — no override needed, but SAY SO in the Task 1 report (checked, not forgotten).
- Pins: property/soak/scenario byte-identical; adventure-night.json adds its own pins.
- No engine/store/campaign changes EXCEPT the AdventureLoaded no-op fold arm (Task 2, tested).

## Shared interfaces (binding across tasks)

```go
// internal/adventure (Task 2)
type Adventure struct {
    ID, Name, RulesetID string
    OpeningNarration    string
    Scenes  []AdventureScene   // {ID, Name string; GridW, GridH int32; Placements []Placement{TokenID, ActorID string; X, Y int32}}
    Actors  []AdventureActor   // {ID, Name string; Attributes map[string]int32; Resources map[string]ResourceVal{Current, Max int32}}
    Notes   []AdventureNote    // {Key, Title, Text string}
    GuidePath string           // dir/guide.md — read by MCP, never compiled
}
func Load(dir string, rs *rules.Ruleset) (*Adventure, error)
func Compile(adv *Adventure, st *engine.State) ([]*vttv1.Envelope, error)
// Compile order (deterministic, binding): AdventureLoaded, scenes (file-name order),
// actors (file-name order), placements (scene order, declared order), notes
// (declared order), opening NarrationAdded. Collision checks vs st BEFORE any output.
```

---

### Task 1: Contract — LoadAdventure + AdventureLoaded

**Files:** contract protos (+`LoadAdventure{adventure_id}` command, next free ClientCommand tag; +`AdventureLoaded{adventure_id, name}` event, next free Envelope tag — VERIFY tags fresh), toolgen manifest +1 (load_adventure: "Load a prepared adventure into the campaign — compiles its scenes, statblocks, notes, and opening narration into one atomic batch of setup events. DM/agent only."), regenerate gen/ + cmd/vtt/tools.json + expected golden, fixtures (`adventure_loaded_envelope.json`, `load_adventure_command.json`), both round-trip suites, PRE-AUTHORIZED tool-count pin updates 15→16. Evidence protocol P3-Task-1 (drift RED pre-commit captured). requiredOverride check documented (adventure_id genuinely required — no override).

### Task 2: internal/adventure — Load, Compile + the engine no-op

**Files:** internal/adventure/{format.go,load.go,compile.go} + tests + testdata fixtures; internal/engine/{apply.go,apply_test.go} (AdventureLoaded no-op arm, tested with full-state-unchanged assert); .go-arch-lint.yml (+adventure component; gateway gains adventure in Task 4 — add both edges now, documented).
Load: strict decode; validation catalogue (one focused invalid fixture + file+field error + test EACH): unknown/mismatched ruleset id handled at the CALLER (Load takes the ruleset — the id equality check lives in Load: manifest.ruleset != rs.ID → error); undeclared attribute/resource name in a statblock (defenses valued in attributes per v2 convention — validate against the union the ruleset declares); resource current > max (when max > 0); note key/title/text over the world-layer byte caps (SAME constants — import or mirror? BINDING: mirror as local consts with a test pinning equality to engine's values); opening narration empty or > 8 KiB; duplicate scene/actor/token/note ids WITHIN the adventure; placement referencing unknown actor or scene; empty adventure (no scenes AND no actors AND no notes = error — a manifest alone is a mistake); grid bounds sane (w/h ≥ 1); placement coordinates within the scene grid.
Compile: deterministic order per the shared-interfaces block; collision checks vs live State (existing scene/actor/token id, existing note key → error naming the collision, NOTHING emitted); output envelopes carry payloads only (campaign stamps).
- [ ] RED-first throughout; valid-fixture compile test asserts the EXACT hand-derived envelope list.

### Task 3: Two adventures + conformance analog

**Files:** internal/adventure/conformance/{conformance.go,conformance_test.go} (+testdata invalid fixtures); adventures/cellar-rats/** (tavern-brawl toy: one scene, two brawlers, one note, two-line guide); adventures/goblin-ambush/** (dnd45e-minimal: ravine scene, fighter + cutter + archer statblocks from the ruleset guide, ambush note, opening narration, guide.md with beats + the archer-flees-at-bloodied secret).
Conformance Run(dir): resolve+load the DECLARED ruleset from rulesets/ (path convention: ../../rulesets/<id> from repo root — take a rulesets root parameter), validate, compile against an EMPTY campaign state, compare EXACT compiled-batch golden (`goldens/compiled-batch.json` per adventure — canonical serialization, deterministic). conformance_test globs adventures/* — both committed adventures gated forever. Guide files: non-empty check; cellar-rats exercises minimal shape, goblin-ambush the full shape.
- [ ] Goldens hand-derived (setup is dice-free — derivation is mechanical but SHOW it in the report); RED = conformance demands goldens before they exist.

### Task 4: Wiring — serve/gateway/harness/MCP

**Files:** cmd/vtt/serve.go (+--adventures-dir; boot: Load+validate EVERY adventure in the dir against the served ruleset, exit nonzero with file+field error on any failure; adventures with a DIFFERENT ruleset id than served: SKIPPED, not a boot error — **AMENDED 2026-08-06 by Patrik**; the original binding read "boot error too — the dir is for THIS table", which made the repo's own ./adventures unbootable (cellar-rats declares tavern-brawl, goblin-ambush declares dnd45e-minimal) and forced a symlinked single-ruleset fixture that gremlins' workdir copy silently drops, leaving cmd/vtt unmeasurable. An adventures dir is a LIBRARY: serve what is written for this table, skip the rest, and still fail loud when NOTHING matches — naming the served ruleset, so "no adventures" is never printed about a directory the operator can see is full. A malformed adventure is still a boot error; only adventure.ErrRulesetMismatch is skippable), internal/gateway/{authz.go,authz_test.go,server.go or ruleset.go-style handler file,convert.go} (load_adventure dm/agent only — 13 commands × 4 roles = 52 literal cells; handler: authz → lookup by id (unknown → clean error; none configured → "no adventures available") → Compile vs live snapshot → AppendBatch → first seq; collision/validation errors → ok=false clean), internal/harness (scenario library `scenarios/adventure-night.json`: serve-with-adventures boot config (scenario gains optional "adventuresDir"? CHECK how ruleset field works in self-contained boot and mirror: BINDING — scenario gains optional top-level `"adventures": "<dir>"`), load goblin-ambush, probe placed tokens (tokenAt) + notes (noteAt) + narration observed, one deterministic beat (rally-at-full-hp clamp trick if needed), denials: player + spectator load_adventure, unknown id, DOUBLE-LOAD collision rejection), undo-of-adventure-batch test (retract the range, state returns to pre-load — campaign-level test), MCP: get_adventure_guide{adventure_id} tool via `vtt mcp --adventures-dir` (ruleset-guide precedent; no dir → clean error; unknown id → clean error), tool count pins 16→17, e2e: load_adventure round-trip + guide served + no-dir error.

### Task 5: Workflow-level final review + fix wave → remote demo → merge gate

Lenses: batch/data-integrity (the second AppendBatch caller — collision TOCTOU between Compile's snapshot and AppendBatch's validation, double-load races), wire/authz, spec-vs-impl sweep, test-integrity (fresh mutations). Dedup → 3-refuter panels → triage → fix wave → verify. Then the REMOTE DEMO (5b pattern): live serve --adventures-dir, fresh subagent DM with guide-only knowledge loads goblin-ambush and plays the opening beats; event log independently verified. Phone-sized report → merge gate.
