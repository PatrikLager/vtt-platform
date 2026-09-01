# World Layer — Narration & World Notes — Design Spec (sub-project 8)

**Date:** 2026-07-26
**Status:** Approved design (brainstorming output, Patrik remote)
**Parent:** platform spec (build-order item 8); pillars P1–P3 apply in
full. The adventure format (Temple-of-Evil vision) is NOT this
sub-project — it is the next one, and it consumes the primitives built
here.

## 1. Purpose

The table gains story-memory. Today the log remembers every die forever
while the DM's narration evaporates with the LLM's conversation. After 8:
narration is events (P3 — story and mechanics in ONE log, replayable
together), and world notes give the DM durable cross-session memory
("what is this town" survives the conversation that invented it).

## 2. Decisions (locked in brainstorming, 2026-07-26)

1. **v1 = narration feed + world notes.** Adventure format deferred (next
   sub-project, designed on these primitives).
2. **Everyone writes the feed, role-tagged:** dm/agent/player may narrate
   or speak; every entry carries its author via the envelope's existing
   participant attribution. Spectators stay read-only.
3. **Optional anchoring:** an entry MAY reference the mechanical event
   range it narrates, so replay can weave story and mechanics back
   together. Free text needs no anchor.

## 3. Contract additions (all additive)

Commands (+ ClientCommand oneof tags, auto-appearing MCP tools):
- `AddNarration{text, as, anchor_from_seq, anchor_to_seq}` — `as` is an
  optional speaker label (the DM voicing "Goblin Cutter", a player
  speaking in character; empty = the participant speaks as themselves).
  Anchors optional (0 = unanchored); when set, `from <= to` and both > 0.
- `UpsertNote{key, title, text}` — create-or-replace a world note.
- `DeleteNote{key}`.

Events (+ Envelope oneof tags):
- `NarrationAdded{text, as, anchor_from_seq, anchor_to_seq}` — pure
  testimony; the engine folds it as a deliberate no-op (the feed IS the
  log, read via the existing event streams).
- `NoteUpserted{key, title, text}` — folds into state.
- `NoteDeleted{key}` — folds; deleting an absent key is a rejection
  (matches the condition-removal posture).

No game-system vocabulary anywhere; the semgrep gate applies unchanged.

## 4. Engine & state

`State` gains `Notes map[string]Note` with
`Note{Title, Text string; UpdatedSeq int64}` — last write wins; history
lives in the log (prior versions retrievable by replay, free). Snapshot
deep-copies Notes; `NewState` initializes it. BOTH `statesEqual` oracles
(campaign scenario test + harness soak) gain Notes in the same task that
adds the field (the 5c lesson — the oracle lags the state at our peril).
`NarrationAdded` is a tested no-op (AbilityUsed's pattern). Validation in
the fold: note key non-empty and ≤ 128 bytes; narration/note text
non-empty and ≤ 8 KiB (a size posture, not a scripting surface); anchor
sanity `from <= to`, both ≥ 0, `to` ≤ the validating snapshot's head
(anchors point backward at recorded history, never forward).

*(Amended 2026-07-26, merge gate — the size posture in full, every cap
denominated in UTF-8 BYTES, not characters: note key ≤ 128 B non-empty;
note title ≤ 256 B may-be-empty; note text and narration text ≤ 8192 B
non-empty; narration speaker label `as` ≤ 256 B may-be-empty — the
final-review triage overturned an earlier leave-uncapped ruling because
append-only permanence means caps must precede any live log. The
gateway's 32 KiB per-frame websocket bound is an OWNED part of this
posture, pinned by an explicit SetReadLimit in internal/gateway rather
than inherited silently from a library default.)*

## 5. Wiring

- **Gateway/authz:** `add_narration` — dm/agent/player allow, spectator
  deny; `upsert_note`/`delete_note` — dm/agent only (world facts are the
  DM's; revisit if players ever co-author). Table grows to 12 commands ×
  4 roles, all cells literal. Single Append per command via the existing
  path (no batches needed).
- **Reading:** narration arrives through the existing event streams
  (`get_events_since` interleaves story with mechanics chronologically —
  that IS the story replay); notes arrive in `get_state`. No new read
  tools in v1; a filtered "narration only" read is named residue.
- **Harness:** the new commands work through the generic command step
  already; add a `noteAt{key, titleIs?, textContains?}` probe; one new
  library scenario `scenarios/story-table.json` exercising narration
  (anchored + free + `as`-voiced), note upsert/replace/delete, and the
  denial rows (spectator narrate; player upsert_note).
- **MCP:** three tools auto-appear from the toolgen manifest (count
  12 → 15). Instructions text: one line telling the agent narration is
  how the table remembers its story.

## 6. Testing (ADR-009 binding)

Stub-first behavioral RED throughout. Engine: fold/rejection/deep-copy
tests incl. rejection-leaves-state-unchanged; the no-op narration pin;
notes in both statesEqual oracles WITH a discrimination test each.
Gateway: authz table literal cells ×4 roles; size-cap and anchor
rejections surface as clean ok=false. Property test: the action mix
gains narration + note upserts/deletes (rebuild==live now covers the new
state dimension); soak/scenario pins re-baselined where the mix changes
(deliberate, documented — not byte-identical this time, the mix change
is the point). Library scenario runs in `task check` forever. MCP e2e:
tool list 15; add_narration + upsert_note round-trip through a live
server; note visible in get_state; narration visible in
get_events_since.

## 7. Non-goals (YAGNI)

Adventure format (next sub-project); narration editing/deletion (the log
is append-only — a correction is a new entry; Undo retracts like any
event — *2026-08-30: the first clause is now the whole rule. Undo left the
platform (`2026-08-30-retraction-leaves-design.md`), so a correction being a new
entry is not one option among two; it is the only one*); rich text/markup semantics (text is opaque UTF-8 to the
platform); filtered/paginated story read tools (residue); note
categories/hierarchies (keys are flat; a ruleset/adventure convention
can namespace them); summarization (the DM's job, not the platform's);
per-note permissions.

## 8. Open questions (deferred, with owners)

- Story read tool with filtering/pagination — with the client
  (sub-project 7), which will want it for the feed UI.
- Note namespacing conventions — with the adventure format.
- Player-editable notes (shared journals) — if a real table asks.
