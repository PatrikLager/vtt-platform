# Event Core — Design Spec (sub-project 2)

**Date:** 2026-07-23
**Status:** Approved design (brainstorming output)
**Parent:** Platform spec §4.2 (`2026-07-23-llm-native-vtt-platform-design.md`); pillars P1–P4; ADR-003 (event-sourced state), ADR-007 (protobuf contract)
**Consumes:** `vttv1.Envelope` from the completed contract pipeline (sub-project 1)

## 1. Purpose

The event core is the platform's memory: an append-only SQLite log of
`vtt.v1.Envelope` events per campaign, a derived in-memory game state, live
subscriptions, and truthful undo. Everything later (API gateway, rules
interpreter, LLM context feed) reads and writes THROUGH this layer; only this
layer touches the log.

## 2. Decisions (locked in brainstorming)

1. **Undo = compensating marker events.** `EventsRetracted{from_sequence,
   to_sequence, reason}` is an ordinary appended event; the log is never
   truncated or mutated. Derivation skips retracted ranges. Retraction events
   themselves cannot be retracted (no nesting — documented and tested).
2. **Contract grows a minimal lifecycle set** (additive, through the armed
   gates): `SceneCreated`, `ActorAdded`, `TokenPlaced`, `SessionStarted`,
   `SessionEnded`, `EventsRetracted` as new envelope oneof variants (tags
   12–17). No HP/condition/initiative events — those belong to the rules-module
   work that gives them meaning (sub-project 5).
3. **Enforcement lands with the code:** semgrep game-vocabulary ban and
   go-arch-lint layer map join `task check` in this sub-project, guarding the
   first engine packages from their first commit.
4. **Architecture = single-fold hybrid (Approach C):** one pure
   `apply(state, event)` function is the only code that changes state, used by
   both full replay (campaign open, undo rebuild) and live append. Divergence
   between "replayed" and "live" state is structurally impossible; a property
   test enforces it besides.

## 3. Package architecture

```
internal/store      Append-only SQLite log. Append (assigns authoritative
                    sequence, stamps it into the Envelope, persists protobuf
                    binary), ReadAll, Subscribe (Go channel fan-out),
                    Close. Knows Envelope framing; knows NO game state.
internal/engine     The projection. State types (scenes, actors, tokens,
                    sessions) + pure apply(). Imports vttv1 only —
                    no store, no SQLite, no I/O.
internal/campaign   Composition root. Open(path) → replay log through apply →
                    live projection. Append(event) → store persists (THE
                    commit point) → apply advances projection → subscribers
                    notified. Undo(fromSeq, toSeq, reason) → append
                    EventsRetracted → rebuild projection. Close().
```

**Machine-enforced rules (new in this sub-project):**
- go-arch-lint: `store` ⊄ `engine`, `engine` ⊄ `store`; only `campaign` may
  import both. (Import-level layering is what the gate enforces; mutation
  confinement to `engine.Apply` holds by construction and code review —
  `State`'s fields are exported.)
- semgrep: game-system vocabulary (`healingSurge`, `dailyPower`, `fortitude`,
  `encounterPower`, `bloodied`, …; list maintained in the semgrep config)
  forbidden across `internal/` — pillar P2/P4 made mechanical.
- Both wired into `task check`.

## 4. Store

- One campaign = one SQLite file. Driver: `modernc.org/sqlite` (pure Go —
  preserves single-binary distribution; version pinned in go.mod).
- Schema: `events(seq INTEGER PRIMARY KEY, event_id TEXT UNIQUE NOT NULL,
  session_id TEXT NOT NULL, occurred_at TEXT NOT NULL, payload BLOB NOT NULL)`.
  Sequence is assigned transactionally by the store (`MAX(seq)+1` under the
  write lock), not by SQLite AUTOINCREMENT. `payload` is the protobuf-binary
  Envelope (post-stamping, so stored bytes contain the true sequence).
- Sequence is store-assigned and authoritative: callers submit envelopes with
  `sequence=0`; Append stamps the assigned value and returns it. A non-zero
  incoming sequence is an error (protects against replayed/forged ordering).
- `event_id` is caller-supplied (ULID convention) and UNIQUE — natural
  idempotency guard against double-append.
- Subscribe returns a channel receiving every event from a given sequence
  onward (catch-up from log, then live). Slow consumers get a bounded buffer;
  overflow closes that subscriber's channel (channel close is the only signal —
  no error value) — the log is the recovery path, so no subscriber can block
  appends.

## 5. Engine (projection)

State derived from lifecycle + movement events only:

| Event | Effect on state |
|---|---|
| SessionStarted / SessionEnded | opens/closes session record; appends to a session index |
| SceneCreated | adds scene (id, name, grid dimensions) |
| ActorAdded | adds actor (the contract's open-map Actor, stored as-is) |
| TokenPlaced | adds token (id, sceneId, actorId, position) |
| TokenMoved | updates token position (unknown token = apply error, see §7) |
| AttackRolled | **no state change — deliberate.** It is testimony; rules-layer meaning arrives in sub-project 5 |
| EventsRetracted | not applied in-line; triggers projection rebuild with retracted ranges filtered |

Readers receive deep copies or immutable views — never aliases into the live
projection (single-writer rule holds at the type level).

## 6. Undo

`Undo(from, to, reason)` validates the range (exists; contains no
EventsRetracted event — the no-nesting rule; not already retracted), then
**dry-runs the fold with the would-be-retracted set before persisting: a
retraction that would leave the log unable to replay is rejected and persists
nothing** (without this, the rebuild promise below is unfulfillable for exactly
those ranges). Only on dry-run success does it append `EventsRetracted` like
any event and rebuild the projection by replaying with retracted ranges
skipped. Rebuild cost is acceptable at table
scale (thousands of events); incremental retraction is deliberately deferred.
Subscribers receive the EventsRetracted event itself — every observer's
history stays truthful, including the future LLM context feed.

## 7. Error handling

- **Append is atomic:** persist-then-apply; if apply rejects an event that was
  already persisted (e.g. TokenMoved for an unknown token), that is a bug in
  validation, so Append VALIDATES against the projection before persisting.
  Validation failures return errors to the caller and write nothing.
- Store-level append errors (I/O, duplicate event_id) persist nothing,
  propagate to the caller, and do NOT poison the campaign. A **post-persist**
  failure (live-apply divergence, post-marker rebuild error — both defensively
  unreachable by design) **poisons** the Campaign: every subsequent
  Append/Undo/Subscribe fails and State returns nil until Close + reopen
  (replay heals). Accepted risk: a commit-stage store error is treated as
  not-persisted; in the marginal case where the commit was nonetheless durable,
  divergence is healed on reopen.
- Malformed envelopes (no payload, unknown variant for this engine version)
  are rejected at Append. Unknown variants found in an EXISTING log during
  replay are skipped with a logged warning (forward compatibility: an older
  binary reading a newer campaign must not crash — additive evolution promise).

## 8. Contract additions (Task-gated by drift + breaking gates)

New messages in `events.proto` + envelope oneof variants (tags 12–17):
`SceneCreated{scene_id, name, grid_width, grid_height}`,
`ActorAdded{actor Actor}`, `TokenPlaced{token_id, scene_id, actor_id,
position GridPosition}`, `SessionStarted{name}`, `SessionEnded{}`,
`EventsRetracted{from_sequence, to_sequence, reason}` (int64 sequences —
wire convention 1 applies). Carry-forward honored: the contract-evolving task
runs `check:drift` post-commit (HEAD-relative semantic). No new toolgen
manifest entries — these are event records, not LLM commands (commands arrive
with the gateway, sub-project 3).

## 9. Testing & exit criteria

- Unit: per-event apply tests; store round-trip, sequence stamping, event_id
  idempotency; subscription catch-up + live + overflow; undo incl. no-nesting
  and double-retraction rejection.
- **Keystone property test:** after N random valid events with random valid
  retractions interleaved, `rebuild(log) == live projection` (deep equality).
- **Exit scenario** (seed of sub-project 4's harness): open campaign → start
  session → create scene → add actors → place tokens → move → retract a move →
  end session → close → reopen → state deep-equals pre-close state. Green in
  `go test`, all gates green in `task check`.

## 10. Non-goals (YAGNI, deliberate)

No snapshots (replay is fast at table scale; the store schema does not
preclude adding them). No cross-process subscribers (gateway's job). No log
compaction. No permissions (sub-project 3). No rules meaning for AttackRolled
(sub-project 5). No CLI surface yet (`vtt` binary shell is sub-project 3's
scaffold decision).

## 11. Open questions (deferred, with owners)

- Bounded-buffer size for subscribers: RESOLVED DIFFERENTLY — caller-supplied
  `buffer` parameter on Subscribe rather than a package constant; the gateway
  picks its value in sub-project 3.
- Whether `campaign` exposes a synchronous or async Append to the future
  gateway — decided in sub-project 3 against real WebSocket flow.
