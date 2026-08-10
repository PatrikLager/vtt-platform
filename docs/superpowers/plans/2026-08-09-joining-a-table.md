# Joining a table — plan

**Spec:** `docs/superpowers/specs/2026-08-08-joining-a-table-design.md`
— **ACCEPTED, Patrik 2026-08-09.** **Status:** AGREED, Patrik 2026-08-09.

**Goal.** The DM shares ONE link. Anyone opening it while the door is open joins
as a spectator with their own credential, and the DM promotes them. Everything
the per-person invite model guarantees survives unchanged.

Branch: `feat/joining-a-table`. One TDD cycle per task, reviewed before it
lands, `task check` green pre-commit.

## The lesson this plan is built around

The presence plan assigned every LAYER and left the SEAMS between them unowned.
`grant_actor_control` reached the contract, both folds, authz and the MCP tool
list — and shipped DEAD, because no task owned `gateway/convert.go` and no task
owned the client. Two separate reviews found those halves, eight commits apart,
with a green gate throughout. Then the same shape twice more: `app.ts`'s
participants wiring, and the spectator header one line below it.

So every task below names its seam explicitly, and **T6 is a seam task with no
feature in it at all**. The rule: a task is not done when its layer works; it is
done when something that already existed can REACH it.

## Task order, and why

Identity first, because everything consumes it. The door before the endpoint,
so the endpoint is never briefly open-by-default. Promotion before the DM
console, so the console has something real to call. Client last, because it
consumes the wire — and the seam task after that, because the client existing
is not the same as it being reachable.

---

### T1 — the door and the link (identity)

**Files:** `internal/identity/identity.go`, its schema and tests.

Campaign-scoped operational state: a join secret and an open/closed flag.
Neither is an event (spec §4) — replaying a campaign must not reopen a door.

- `JoinSecret() (string, error)`, `RotateJoinSecret() (string, error)`
- `SetJoinOpen(bool)`, `JoinOpen() bool`
- schema migration, additive; the table already exists and is created with
  `CREATE TABLE IF NOT EXISTS`, so check how an EXISTING db picks up a new
  column before assuming it does

**Watch:** closed by default is the security property (spec §2). A migration
that defaults to open on an existing campaign is the whole feature backwards.
Test it against a db created BEFORE the migration, not only a fresh one.

---

### T2 — the join endpoint (gateway)

**Files:** `internal/gateway/` — an HTTP handler, not a WS command.

`POST /join` with the secret and a display name → mint via `CreateInvite` with
role `spectator` → return the token.

- refused when the door is CLOSED, and refused the same way for a wrong secret:
  a distinguishable error tells a prober which half they got right, and
  `Verify` already sets this precedent deliberately
- reuses `CreateInvite` unchanged — that is what keeps §3.1a true, so a joiner
  who reconnects gets the same participant id and their characters back
- empty/whitespace display name refused; it is what the table will see

**Watch:** this endpoint is UNAUTHENTICATED by construction (spec §5). It mints
a spectator and returns a token and it does NOTHING else — no campaign state, no
event, no role choice from the caller.

---

### T3 — promotion (identity + gateway)

**Files:** `internal/identity`, `internal/gateway/authz.go` + tests.

`SetRole(id, role)` updating `participants.role` — one source of truth, per
spec §3.1. Plus a `promote_participant` command, dm/agent only.

`commandRoles` grows 60 → 64 literal cells. Both directions per new cell.

**Watch, and this is the sharp one:** a spectator must not promote itself
(spec §5). That is not the same test as "spectator cannot promote" — check the
self case explicitly, because the participant id in the command and the id on
the connection are different fields and confusing them is exactly how the
`revoke_actor_control` self-check nearly shipped unpinned.

---

### T4 — a promotion reaches the live connection

**Files:** `internal/gateway/server.go`, presence registry.

**AMENDED 2026-08-09, Patrik's call, and the amendment is the interesting
part.** This task originally read: "`Verify` runs once at connect and the
`Participant` is held for the connection's life, so a role change is invisible
until reconnect. Close the promoted participant's connections." Patrik rejected
it — *"What role a participant has and what connection it has should be fully
separated"* — and he was right. Caching the answer to "what may this person do"
for the life of a socket is the DEFECT; a reconnect is not a fix for it, it is
a way of paying for it. Recorded rather than quietly rewritten, because the
wrong version is the more natural one to reach for and will be proposed again.

**So: the participant is RE-RESOLVED, per command and per delivered event.** A
promotion and a revocation both bite on the very next thing that person does or
is shown. No reconnect, no dropped socket, no waiting. Spec §3.2.

**Watch — three of them, all found the hard way:**

- A SPECTATOR ISSUES NO COMMANDS, so the command path alone reaches nobody who
  joined through the link. Delivery is where a watcher meets the server.
- PRESENCE FRAMES DO NOT TRAVEL THE PUMP. They are written straight into each
  connection's channel, so the delivery gate misses them too and a revoked
  watcher goes on seeing who arrives. That needs its own denial, resolved
  OUTSIDE the registry lock — a lookup inside the broadcast loop puts one
  SQLite read per connection under the global mutex.
- ONLY `ErrInvalidToken` MAY END A CONNECTION. A busy database is not a fact
  about anybody's credential, and closing on it tells a player in good standing
  that theirs is no longer valid.

**Measured before adopting** (spec §3.2 asked for it): 15.5µs against a
40-participant table, on a path that already folds state, appends to SQLite and
writes a socket frame.

---

### T5 — the client join view

**Files:** `client/src/` — a join view, `app.ts` wiring.

No stored token and a join secret in the URL → ask for a display name → POST →
store the token exactly as the invite flow does (localStorage, stripped from
the URL, per `app.ts`'s existing handling).

**Watch:** `task check:ts-mutation:docker` BEFORE pushing. The gate covers every
file under `client/src` and cannot run on macOS.

---

### T6 — the seams, and nothing else

**No new feature. This task exists because the presence branch proved that a
feature complete in every layer can still be unreachable.**

- **DM console**: open/close the door, show and rotate the link, promote a
  spectator. Beside T7's "Who controls what" panel — promote and assign a
  character in one place.
- **`vtt` CLI**: whatever the DM needs before a browser exists.
- **MCP**: `promote_participant` auto-appears as a tool; the count moves and the
  stale-comment ripple gets grepped for the WORD, not the number.
- **An end-to-end test that a REAL PERSON can do it**: open the link, land as a
  spectator, be promoted, act. Through the browser, against the shipped binary,
  the way `handover.spec.ts` does it.

**The completeness gate added in `convert.go` will catch a missing conversion
arm. Nothing catches a missing UI.** That is what this task is for.

---

## Final review and demo gate

Whole-branch review, then a remote-friendly demo: the DM opens the door, shares
one link, a second browser joins by name, lands as a spectator and can only
watch, the DM promotes them and assigns a character, they play. Then the DM
rotates the link and a third browser is refused. Screenshots at each beat.

Patrik's merge call closes it.

## Carry-forwards, not to fix here

- The audit trail for role changes (spec §3.1) — narration event or audit
  table. Decide when there is a reason, not on the day promotion ships.
- A QR code (spec §6.2), purely additive once one link exists.
- `vtt invite` stays. Minting a DM or an agent is not something a shared
  player link should ever do.
