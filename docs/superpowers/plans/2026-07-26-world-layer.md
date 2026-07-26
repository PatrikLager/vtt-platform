# World Layer Implementation Plan (sub-project 8)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver sub-project 8 per the approved spec (`docs/superpowers/specs/2026-07-26-world-layer-design.md`): narration as events, world notes as folded state, wired through gateway/harness/MCP.

**Architecture:** three additive commands/events; NarrationAdded is a tested no-op fold (testimony); Notes fold into State; single Append per command (no batches). No new packages.

**Tech Stack:** existing.

## Global Constraints

- Branch `feat/world-layer` from main. Adapted review-before-commit flow; reviewers READ-ONLY; injections/mutations in throwaway rsync copies ONLY (never the real tree, even backup-and-restore — ledgered incidents make this absolute).
- Contract additive only; Task 1 drift-caveat protocol (base gates pre-commit, full check post-commit).
- ADR-009 binding: stub-first behavioral RED; injection proofs for after-the-fact tests; boundary behavior only.
- Vocabulary ban unchanged (semgrep + bare-`hp` rule; "narration"/"note" are platform-generic).
- THE 5c LESSON IS LAW: the task that adds State.Notes ALSO extends both statesEqual oracles (internal/campaign/scenario_test.go + internal/harness/soak.go) with discrimination tests — same task, same commit.
- Pins: property/soak pinned counts are DELIBERATELY re-baselined in Task 3 when the action mix grows (old + new counts documented in the report); scenario library gains story-table.json (its own new pins); everything else byte-identical.

---

### Task 1: Contract — narration & notes vocabulary

**Files:** `contract/vtt/v1/events.proto`, `contract/vtt/v1/commands.proto`, toolgen manifest (+3), golden update, fixtures (`narration_added_envelope.json` incl. anchors + `upsert_note_command.json`), both round-trip suites, gen regenerated, `cmd/vtt/tools.json` copy refreshed.

Messages verbatim (next free oneof tags — VERIFY against the current protos before assuming; events after AbilityUsed-family, commands after RemoveCondition):
```proto
message NarrationAdded {
  string text = 1;
  string as = 2;
  int64 anchor_from_seq = 3;
  int64 anchor_to_seq = 4;
}
message NoteUpserted { string key = 1; string title = 2; string text = 3; }
message NoteDeleted { string key = 1; }
message AddNarration {
  string text = 1;
  string as = 2;
  int64 anchor_from_seq = 3;
  int64 anchor_to_seq = 4;
}
message UpsertNote { string key = 1; string title = 2; string text = 3; }
message DeleteNote { string key = 1; }
```
Toolgen descriptions: add_narration "Add a story entry to the table's shared narrative — narration, in-character speech (set `as`), or table talk; optionally anchored to the event sequences it narrates."; upsert_note "Create or replace a keyed world note (locations, NPCs, quest state) — the campaign's durable memory."; delete_note "Delete a world note by key."
Evidence protocol: P3-Task-1 (drift red pre-commit expected; base gates green).

### Task 2: Engine — Notes state + folds

**Files:** `internal/engine/{state.go,apply.go,apply_test.go}`, `internal/campaign/scenario_test.go`, `internal/harness/soak.go` (+ its statesEqual discrimination test file).

State gains `Notes map[string]Note`; `Note{Title, Text string; UpdatedSeq int64}`; NewState initializes; Snapshot deep-copies. Folds: NoteUpserted → last-write-wins upsert (UpdatedSeq = env.Sequence); NoteDeleted → delete, ABSENT KEY = rejection (state unchanged, condition-removal posture); NarrationAdded → deliberate tested no-op. Fold-level validation (rejection, state unchanged, every rule its own test): note key empty or > 128 bytes; note/narration text empty or > 8192 bytes; narration anchors: from/to negative, from > to (when both set), to >= env.Sequence (anchors point backward only; 0/0 = unanchored is valid). BOTH statesEqual oracles gain Notes + a discrimination test each. Rejection-leaves-state-unchanged snapshot pattern throughout; deep-copy mutation-independence test extended to Notes.

### Task 3: Wiring — gateway/harness/MCP + scenario

**Files:** `internal/gateway/{authz.go,authz_test.go,server.go,convert.go,convert_test.go}`, `internal/harness/{scenario.go,engine.go}` (+ noteAt probe + tests both directions), `internal/campaign` property test action mix, `scenarios/story-table.json`, `internal/mcp` e2e extensions, `cmd/vtt` library/e2e tests as needed.

- Authz: add_narration — dm/agent/player allow, spectator deny; upsert_note/delete_note — dm/agent only. Table test grows to 12 commands × 4 roles, all literal cells, RED before rows exist.
- Handlers: existing single-Append path; command-level validation errors surface as ok=false (size caps and anchor sanity re-checked at the fold — the gateway just forwards; verify a rejection comes back clean, not poisoned).
- Harness: `noteAt{key, titleIs?, textContains?}` probe (pass AND fail directions in TestRunScenarioProbesPerKind — the P8 lesson); generic command steps already carry the new commands.
- Property test: action mix gains addNarration/upsertNote/deleteNote (deleteNote may target absent keys → rejection events count too); NEW pinned counts documented old→new in the report; rebuild==live now proves the Notes dimension.
- `scenarios/story-table.json`: session; anchored narration (narrate a moveToken by its sequence), free narration, `as`-voiced entry from the DM, player table-talk; note upsert → replace → noteAt probes → delete → deleted-absent rejection; denials: spectator add_narration, player upsert_note. All assertions deterministic.
- MCP e2e: tool list 15; add_narration + upsert_note through a live server; note in get_state; narration in get_events_since.

### Task 4: Workflow-level final review (standing preference) + fix wave → merge gate

Lenses scaled to the diff: data-integrity/engine, wire/authz, test-integrity (ADR-009 + the re-baselined pins audited as deliberate), spec-vs-implementation sweep. Dedup → 3-refuter panels → triage. Fix wave → verify → merge gate to Patrik (phone-sized report). Merge bundle: memory update; any spec amendments found.
