# Event Core — Design Spec (sub-project 2)

**Date:** 2026-07-23
**Status:** Approved design (brainstorming output)
**Parent:** Platform spec §4.2 (`2026-07-23-llm-native-vtt-platform-design.md`); pillars P1–P4; ADR-003 (event-sourced state), ADR-007 (protobuf contract)
**Consumes:** `vttv1.Envelope` from the completed contract pipeline (sub-project 1)

> **AMENDED 2026-08-30 — UNDO WAS BUILT AS SPECIFIED AND HAS SINCE BEEN REMOVED.**
> Patrik's ruling of 2026-08-30: a retraction exists to make something not have
> happened, and it cannot — the person already read the log. Sub-project 13
> (`2026-08-30-retraction-leaves-design.md`) took it out: `EventsRetracted` and
> `RetractEvents` left the contract in `59542e1`, `campaign.Undo` and
> `retractedSet` in `133e896`, and `campaign.foldEvents`/`FoldPrefix` collapsed to
> a single pass because the pass that collected retracted ranges was the only
> reason a second one existed. Nothing replaced it: the platform's answer to a
> mistake is a further event that removes the thing going forward (`remove_token`,
> `remove_actor`), which the log records as having happened.
>
> Everything else in this spec is live and unchanged — the store, the sequence
> authority, the single fold, subscriptions, the poison contract. The passages
> that are now history rather than design are §1's "truthful undo", §2's decisions
> 1 and 2 and the "undo rebuild" clause of decision 4, §3's `Undo` line, §5's
> `EventsRetracted` table row, **§6 in its entirety**, §7's `Undo` mention, §8's
> `EventsRetracted` message and §9's undo/retraction tests. Each is marked where it
> stands; none is deleted, because the decision that was made and the decision that
> replaced it are both worth being able to read.

## 1. Purpose

The event core is the platform's memory: an append-only SQLite log of
`vtt.v1.Envelope` events per campaign, a derived in-memory game state, live
subscriptions, and truthful undo. Everything later (API gateway, rules
interpreter, LLM context feed) reads and writes THROUGH this layer; only this
layer touches the log.

*AMENDED 2026-08-30: "and truthful undo" is no longer part of what this layer
is.* `campaign.Undo` was deleted in `133e896`; see the banner above. The rest of
the sentence is exactly what `internal/store`, `internal/engine` and
`internal/campaign` do today, and the last clause — only this layer touches the
log — is stronger than ever, since there is now one way in and none out.

## 2. Decisions (locked in brainstorming)

1. **Undo = compensating marker events.** `EventsRetracted{from_sequence,
   to_sequence, reason}` is an ordinary appended event; the log is never
   truncated or mutated. Derivation skips retracted ranges. Retraction events
   themselves cannot be retracted (no nesting — documented and tested).

   *SUPERSEDED 2026-08-30.* This decision was implemented exactly as written and
   then reversed. "Derivation skips retracted ranges" is the sentence to watch:
   it is what made every fold in the platform two-pass, in Go and in TypeScript
   alike, and removing it is what collapsed all four of them to one pass
   (`92f1284`, `d3e2f28`, `133e896`). The half of the decision that survives is
   the half about the log: it is still never truncated and never mutated. What
   was wrong was believing a marker could make a reader forget.
2. **Contract grows a minimal lifecycle set** (additive, through the armed
   gates): `SceneCreated`, `ActorAdded`, `TokenPlaced`, `SessionStarted`,
   `SessionEnded`, `EventsRetracted` as new envelope oneof variants (tags
   12–17). No HP/condition/initiative events — those belong to the rules-module
   work that gives them meaning (sub-project 5).

   *AMENDED 2026-08-30:* `EventsRetracted` and its oneof arm were deleted in
   `59542e1`, with no `reserved` left behind — the platform is pre-release, so
   ADR-007's additive-only rule does not yet bind; see its 2026-08-30 amendment
   and the `contract/RELEASED` marker that turns it on. The other five variants
   are unchanged.
3. **Enforcement lands with the code:** semgrep game-vocabulary ban and
   go-arch-lint layer map join `task check` in this sub-project, guarding the
   first engine packages from their first commit.
4. **Architecture = single-fold hybrid (Approach C):** one pure
   `apply(state, event)` function is the only code that changes state, used by
   both full replay (campaign open, undo rebuild) and live append. Divergence
   between "replayed" and "live" state is structurally impossible; a property
   test enforces it besides.

   *AMENDED 2026-08-30:* "undo rebuild" is gone as a caller. The decision itself
   is untouched, is stronger for the removal — the fold no longer walks the log
   twice — and is now CLAUDE.md rule 4.

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

*AMENDED 2026-08-30: the `Undo` line is gone (`133e896`).* What `campaign`
composes today is `Open`, `Append`, `AppendBatch` — the atomic multi-event path
`remove_actor` and `load_map` use — `State`, `Close`, and two subscribe entry
points: `Subscribe`, and `SubscribeWithNoProgressTimeout`, which is the one the
gateway actually calls. Plus the poison contract of §7. CLAUDE.md's Layout section carries the current one-line
description.

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
| EventsRetracted | not applied in-line; triggers projection rebuild with retracted ranges filtered — **REMOVED 2026-08-30** (`59542e1`); no arm of `engine.Apply` filters by sequence any more |

Readers receive deep copies or immutable views — never aliases into the live
projection (single-writer rule holds at the type level).

## 6. Undo

> **OBSOLETE IN FULL, 2026-08-30, and kept rather than deleted.** Everything in
> this section was built and shipped, and all of it was removed by `133e896`:
> `campaign.Undo`, the dry-run, the no-nesting and double-retraction rules, the
> rebuild. It is kept because it is the most complete written record of what the
> platform did for a year, and because the shape of the mistake is instructive —
> read the dry-run paragraph in particular. It dry-ran THE LOG, which is exactly
> the assumption that broke once a seat could receive a projection of the log
> rather than the log (`2026-08-30-dm-hands-and-retraction-design.md` §5.1), and
> that defect is what led to the ruling that removed the operation altogether.
> Nothing here describes current behaviour. What replaced it is
> `2026-08-30-retraction-leaves-design.md` §5: `remove_token` and `remove_actor`,
> which take a thing out of the world going forward and never claim it was not
> there.

`Undo(from, to, reason, eventID, actorRole, participantID) (markerSeq, err)`
validates the range (exists; contains no EventsRetracted event — the
no-nesting rule; not already retracted), then (attribution params are
caller-supplied — campaign has no identity concept; the gateway is the
attribution authority per gateway-spec §4. The sessionID param was removed
when campaign became the session-stamping authority; the marker-sequence
return landed as a P6 pre-step so CommandResult can carry it)
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

  *AMENDED 2026-08-30: the poison contract is live and unchanged; two of its
  named triggers moved.* `Undo` is no longer a poisoned entry point and
  "post-marker rebuild error" is no longer a failure mode, because neither
  exists; `AppendBatch`, added with the ruleset interpreter, is one.
  `internal/campaign`'s own poison tests are the authority on the current list.
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
wire convention 1 applies). *(2026-08-30: `EventsRetracted` was added as
specified and deleted in `59542e1`. The other five messages are on the wire
today.)* Carry-forward honored: the contract-evolving task
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

*AMENDED 2026-08-30 — the undo tests are gone; the properties they served are
not.* The undo unit tests, and the retraction interleaving in the keystone
property, left with `campaign.Undo` (`133e896`). `rebuild(log) == live
projection` is still the keystone and is still property-tested
(`TestRebuildEqualsLiveProperty`); it is simply generated from appends alone,
since there is no operation left that makes a rebuild differ from a replay. The exit scenario is `scenarios/three-role-exit.json`
in the committed library, with the retraction leg removed and every other leg —
denials, no-broadcast, reconnect equality — intact.

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
