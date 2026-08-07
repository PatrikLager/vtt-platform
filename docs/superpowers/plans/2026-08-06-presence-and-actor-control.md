# Presence and actor control — plan

**Spec:** `docs/superpowers/specs/2026-08-06-presence-and-actor-control-design.md`
— **ACCEPTED, Patrik 2026-08-07.**

This header previously claimed the spec was "Accepted, Patrik 2026-08-06". It
was not: the file did not exist and had never been committed, so for Tasks 1 and
2 the citation pointed at nothing and "specs are truth" had nothing to check
against. Found by review 2026-08-07, written from the decisions actually taken,
and accepted the same day.

**Goal.** Peers learn when someone joins or leaves; actor control can be
granted, released and held by more than one participant. Nothing about the
invite credential changes, and nothing about DM authority changes — §3.2 and
§5.4 of the spec record both as already correct.

Branch: `feat/presence-and-actor-control`. One TDD cycle per task, reviewed
before it lands (CLAUDE.md rule 6), `task check` green pre-commit.

## Task order, and why

Contract first because every other task consumes it; engine before gateway
because authz reads folded state; client last because it consumes the wire.
Presence (Tasks 4–5) is independent of control (Tasks 1–3) and could be
parallelised, but the contract task touches one file for both, so serialising
avoids a merge conflict for no real gain.

---

### Task 1 — contract (additive only)

**Files:** `contract/vtt/v1/{commands,events}.proto`, regenerate via
`task generate:contract` (generated code is committed).

```proto
// events.proto — Actor gains the set; controller_id is UNTOUCHED.
message Actor {
  string controller_id = 7;            // still populated, still meaningful
  repeated string controller_ids = N;  // authoritative
}

message ActorControlGranted { string actor_id = 1; string participant_id = 2; }
message ActorControlRevoked { string actor_id = 1; string participant_id = 2; }

// A wire frame, NOT an event — never appended to the log.
message PresenceChanged {
  string participant_id = 1;
  string display_name   = 2;
  PresenceState state   = 3;  // AMENDED: was `bool connected`
}
message PresenceSnapshot { repeated PresenceChanged present = 1; }

// ServerFrame gains two oneof arms alongside result/event/catch_up_head.
```

AMENDED 2026-08-07: the sketch above said `bool connected`. An enum shipped
instead, and the reason is not cosmetic — protojson omits zero values, so a
bool would carry DISCONNECT as an ABSENT field, making the single most
important transition a silence. UNSPECIFIED also catches a sender that forgot
to set it. See spec §4.

Commands imperative, events past-tense (rule 3). `check:breaking` must stay
green — every change here is a new field or message, nothing renumbered.

**Gate:** `task check:breaking` plus the contract round-trip tests.

---

### Task 2 — engine fold maintains the set

**Files:** `internal/engine/{apply,state}.go` and their tests.

`ActorControlGranted` adds to `controller_ids`; `Revoked` removes. Both are
idempotent: granting twice is not an error and does not duplicate, revoking a
participant who does not hold control is a no-op. `AddActor`'s existing
`controller_id` seeds the set with one element.

`controller_id` mirrors `controller_ids[0]`, and is empty ONLY when the set is
empty. **The two must never disagree**, and that is the load-bearing assertion —
it needs a fault-injection proof, because a reader that trusts the mirror while
the set says otherwise is precisely how a player would silently gain or lose a
character.

AMENDED 2026-08-07, Patrik's call, at the merge gate per CLAUDE.md rule 7. This
paragraph previously read "the single element when there is exactly one, empty
otherwise". That is the alternative the design REJECTED, and shipping it would
have been a real defect: protojson omits empty strings, so blanking the scalar
for a SHARED actor makes it byte-identical on the wire to an UNOWNED one — and
empty already means DM/agent-only at `internal/gateway/authz.go:90`. Every
reader predating the set would silently reclassify a shared character as
unowned. With `controller_ids[0]` such a reader is incomplete but never wrong.
See spec §5.1.

Only `engine.Apply` writes state (rule 4). No second fold.

**Watch:** unknown actor id must be rejected symmetrically, the way
ConditionApplied/Removed already are (P8 Task 2 adjudication).

---

### Task 3 — authz reads the set

**Files:** `internal/gateway/authz.go`, `authz_test.go`.

Two call sites change and no others, because ownership gates players alone
(spec §3.2): `authorizeTokenOwnership:90` and `authorizeActorOwnership:106`
become "is `p.ID` in `controller_ids`" instead of "does it equal
`controller_id`". Empty set keeps its current meaning: DM/agent only.

`commandRoles` gains two entries:

| command | DM | agent | player | spectator |
|---|---|---|---|---|
| `grant_actor_control` | yes | yes | no | no |
| `revoke_actor_control` | yes | yes | **self only** | no |

"Self only" is a second check in the same shape as the ownership helpers: a
player may name themselves as the `participant_id` and no one else.

The matrix is literal, not sampled — 13 commands × 4 roles is currently 52
cells; this makes it 15 × 4 = **60**. Both directions per new cell: the allowed
one permits, the denied one denies. A guard that only ever says yes is not a
guard.

---

### Task 4 — gateway presence registry

**Files:** `internal/gateway/server.go`, `presence.go` (new), tests.

New server state, reference-counted **per participant, not per connection**
(spec §4): the same invite token may hold two connections, and closing one must
not tell the table someone left. `DISCONNECTED` is emitted only when the
last connection for that participant goes.

- On connect: register, emit the snapshot to the joining client immediately
  after `CatchUpHead`, then broadcast `PresenceChanged{CONNECTED}` to
  everyone else.
- On `serve` returning: deregister, and if the count reached zero broadcast
  `DISCONNECTED`.

**Both teardown paths must be covered**, and the second is the one that will be
forgotten: a clean quit, AND a wedged client force-closed by the write deadline
(PR #18). The latter is exactly the path whose absence went unnoticed before.

**Watch:** the presence broadcast must not block appends and must not
reintroduce a per-connection fan-out that can wedge — it goes through the same
`outCh` discipline as everything else.

---

### Task 5 — client: presence and manual reconnect

**Files:** `client/src/{wire,session}.ts`, a view, tests.

`wire.ts` already has `reconnect()` redialing at `after=<lastSeq>`; nothing
calls it. Surface the `"closed"` status and offer an explicit action that does.
Reconnect stays manual (spec §3.4) — no timers, no backoff, no guessing when a
network came back.

Presence renders as a participant list. `bun test` plus `client:typecheck`,
which has caught what `bun test` alone would not (a zero-arg fetch stub, and
`NodeList` needing `Array.from`).

---

### Task 6 — MCP tool count and guides

**Files:** `internal/mcp/*`, `cmd/vtt/tools.json`, tool-count pins.

New commands auto-appear as MCP tools (the `load_adventure` precedent), so
`TestListToolsReturnsSeventeenToolsIncludingReadAndGuideTools` and the
committed `tools.json` both move 17 → 19. Fix-forward on the count is
PRE-AUTHORIZED, as it was for P12, so a count bump does not need a separate
round trip.

**Watch:** the stale-comment ripple. "Seventeen" appears in test NAMES and
comments in at least three files; P12 hit this three times on the same rename.
Grep for the word, not just the number.

---

## Final review and demo gate

Workflow final review over the whole branch, then a remote-friendly demo
(Patrik often reviews from his phone): two browser clients, one DM one player;
the DM grants a second character to the player; the player moves both; the
player's tab closes and the table sees them leave; they reconnect and still
control both. Screenshots at each beat.

Patrik's merge call closes it.

## Carry-forwards to ledger, not to fix here

- `controller_id`'s eventual deprecation (spec §7.2) — decide once the set has
  lived a while, not on the day it is introduced.
- The `State.Actors map[string]*vttv1.Actor` coupling that makes a projection
  concern enter the wire contract at all. It surfaced during this design and is
  worth an ADR if it bites a second time — one sighting is not evidence.
