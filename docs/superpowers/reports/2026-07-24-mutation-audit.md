# Mutation Audit — machine (gremlins, 4 packages) + gateway hand-injection

**Date:** 2026-07-24 · **Branch:** `feat/hardening` (sub-project 3.5,
hardening pass) · **Task:** Plan Task 3
(`docs/superpowers/plans/2026-07-24-hardening.md`) · **Full transcripts:**
`.superpowers/sdd/p5-task-3-report.md` (session-local, not committed)

This is the mutation-testing regression-net proof for ADR-009
(`docs/adr/009-airtight-tdd-protocol.md`): every existing test suite in
`internal/engine`, `internal/store`, `internal/campaign`, and
`internal/identity` was mutation-tested with
[gremlins](https://github.com/go-gremlins/gremlins), every surviving
mutant was triaged to a disposition, and ten hand-picked gateway
regressions (chosen because they cross package/transport boundaries in
ways gremlins' single-package token mutators cannot reach) were manually
injected and confirmed caught.

---

## 1. Machine mutation — per-package numbers

Run with `gremlins unleash <pkg> --workers 1 --timeout-coefficient 10` (see
§4 for why these flags, not gremlins defaults).

| Package | Mutants | Killed (before → after triage) | Survived (before → after) | Timed out | Not covered |
|---|---:|---:|---:|---:|---:|
| `internal/engine` | 11 | 10 → **11** | 1 → **0** | 0 | 0 |
| `internal/store` | 20 | 18 → **20** | 2 → **0** | 0 | 0 |
| `internal/identity` | 16 | 16 → **16** | 0 → **0** | 0 | 0 |
| `internal/campaign` | 42 | 34 → **36** | 6 → **4** (accepted equivalent) | 2 (accepted, own category) | 0 |

No package exceeded ~15 minutes (`campaign`, the property-test-bearing
package the brief flagged as the risk, completed in **38.5 seconds**), so
**no test exclusion was needed or applied** — the config-exclusion
contingency in the brief's Step 1 did not trigger.

Timed-out mutants are **not** folded into Killed. They are reported as
their own line per the task's explicit instruction; see §3 for why these
two specifically cannot be anything else (they are genuine infinite loops,
confirmed by hand-injection under a bounded `-timeout`, not slow tests or
worker contention).

---

## 2. Every survivor's disposition (no third category)

| Location | Mutator | Disposition | Evidence |
|---|---|---|---|
| `internal/engine/apply.go:30:8` | CONDITIONALS_BOUNDARY | **Killed by new test** — `TestSessionEndedSuccess` | RED under hand-injection, GREEN restored, gremlins re-confirms KILLED |
| `internal/store/store.go:75:30` | CONDITIONALS_NEGATION | **Killed by new test** — `TestAppendPersistsOccurredAt` | same pattern |
| `internal/store/subscribe.go:27:12` | CONDITIONALS_BOUNDARY | **Killed by new test** — `TestSubscribeAcceptsZeroBuffer` | same pattern |
| `internal/campaign/campaign.go:231:10` | CONDITIONALS_BOUNDARY | **Killed by new test** — `TestUndoAcceptsFromSequenceOne` | same pattern |
| `internal/campaign/campaign.go:239:25` | CONDITIONALS_BOUNDARY | **Killed by new test** — `TestUndoOnEmptyLogRejectsGracefully` | same pattern (mutant is a real `index out of range [-1]` panic) |
| `internal/campaign/campaign.go:264:55` | ARITHMETIC_BASE | **Accepted equivalent** | proof in §3 |
| `internal/campaign/campaign.go:264:62` | ARITHMETIC_BASE | **Accepted equivalent** | proof in §3 |
| `internal/campaign/campaign.go:264:62` | INVERT_NEGATIVES | **Accepted equivalent** | proof in §3 |
| `internal/campaign/campaign.go:264:68` | ARITHMETIC_BASE | **Accepted equivalent** | proof in §3 |
| `internal/campaign/campaign.go:109:87` | INCREMENT_DECREMENT | **Accepted timed-out** (genuine infinite loop) | proof in §3 |
| `internal/campaign/campaign.go:268:33` | INCREMENT_DECREMENT | **Accepted timed-out** (genuine infinite loop) | proof in §3 |

Every one of the four new tests (engine ×1, store ×2, campaign ×2 — 5
tests total) was proven by the full injection cycle ADR-009 rule 3
requires: hand-apply the exact mutation in a throwaway `rsync` copy →
confirm the new test fails specifically → restore → re-run gremlins on the
real package to confirm the mutant flips from LIVED to KILLED. All five
transcripts are in the full report.

### Why the two remaining LIVED groups get no test

**`campaign.go:264` capacity-hint mutants (4).** All four mutate tokens
strictly inside `make(map[int64]bool, len(already)+int(to-from)+1)`'s
THIRD argument — a map's preallocation size **hint**. Per the Go spec this
value is advisory only: it can never affect the resulting map's contents,
key set, or any externally observable behavior, by construction of the
built-in `make` primitive. Verified empirically (not just cited) with a
throwaway program that builds the same map contents under hints ranging
from `-1000` to `999999` — identical results, no panic, every time (full
transcript in `.superpowers/sdd/p5-task-3-report.md` §2). No test — present
or future — can distinguish these four mutants from the original code.

**`campaign.go:109`/`268` INCREMENT_DECREMENT mutants (2).** Both mutate a
loop's `seq++` to `seq--` inside `for seq := X; seq <= Y; seq++`, where the
loop's own entry condition guarantees `X <= Y`. Decrementing keeps the exit
condition permanently true — an unconditional infinite loop, not a slow
path. Confirmed directly: hand-injecting either mutation and running its
covering test under `go test -timeout 5s` produces a goroutine dump
pointing straight at the mutated line, followed by the Go test runner's own
timeout panic — the ONLY detection signal an infinite loop can ever
produce, since the mutated code never returns control for an assertion to
run against. gremlins categorizes this as `Timed out` rather than `Killed`
(a stricter label than the underlying reality — the test binary genuinely
crashes non-zero on this mutant), and this report keeps that distinction
rather than reclassifying it, per the task's explicit "don't hand-wave"
instruction.

---

## 3. Gateway hand-injection — 10/10 (summarized; full transcripts in the session report)

Gremlins mutates one token at a time within a single package; these ten
were chosen because each crosses a package/transport boundary (authz →
identity → WebSocket wire → shared campaign resource) that a single-token
mutator cannot reach. Procedure for every item: refresh a throwaway
`rsync` copy from the real tree → apply the named one-line break → run
**only** the named test → capture the failure → restore the line → confirm
(`diff -rq`, non-test files) the throwaway copy matches the real tree →
next item. The real working tree was never touched by any of these ten;
`docs/superpowers/reports` and `_test.go` files are the only diffs this
task leaves behind.

| # | Break (one line) | File | Test | Result |
|---|---|---|---|---|
| 1 | Authz table: player gains `create_scene` | `internal/gateway/authz.go` | `TestAuthorizeTableAllCommandsAllRoles` | `FAIL create_scene/player` |
| 2 | `authorizeTokenOwnership` always returns nil | `internal/gateway/authz.go` | `TestAuthorizePlayerOtherTokenDenied` | `FAIL`: "want error moving a token controlled by another participant" |
| 3 | `identity.Verify` always succeeds | `internal/identity/identity.go` | `TestConnectBadTokenRejectedBeforeUpgrade` + `TestConnectRevokedTokenRejectedBeforeUpgrade` | both `FAIL`: "want error connecting with a bad/revoked token" |
| 4 | `Subscribe`'s catch-up loop skips `history[0]` | `internal/store/subscribe.go` | `TestConnectAfterZeroReceivesFullHistoryThenLive` | `FAIL`: `history[0] = (seed-2, seq 2), want (seed-1, seq 1)` |
| 5 | Overflow force-close guarded out (`if false && ...`) | `internal/gateway/server.go` | `TestOverflowForcesSocketClosedAndOthersKeepServing` | `FAIL` on the tightened `Fatalf`: "read timed out instead of observing close" |
| 6 | TokenMoved `SceneId`/`From` backfill block dropped | `internal/gateway/server.go` | `TestMoveTokenBroadcastBackfillsSceneAndFrom` | `FAIL`: `SceneId = "", want scn1 (backfilled)` |
| 7 | `ActorRole` hardcoded to `"dm"` | `internal/gateway/convert.go` | `TestThreeRoleExitScenarioOverLiveWebSockets` | `FAIL`: `saw ActorRole="dm", want "player"` |
| 8 | Retraction marker's `Notify` call commented out | `internal/campaign/campaign.go` | `TestAgentRetractEventsBroadcastToAll` | `FAIL`: read times out — marker never broadcast |
| 9 | Malformed-frame path closes the SHARED campaign resource, not just the offending connection | `internal/gateway/server.go` | `TestMalformedFrameClosesOnlyThatConnection` | `FAIL`: the previously-untouched second connection dies too |
| 10 | `CommandResult.Sequence` zeroed | `internal/gateway/server.go` | `TestTwoClientsBothReceiveAcceptedCommandAsEvent` (its folded-in sequence-match assertion) | `FAIL`: `result.Sequence = 0, dmEvent.Sequence = 5` |

Item 9's break is the architecturally-faithful version of "closes ALL
conns": the gateway has no per-connection registry a single connection's
handler could misdirect a close onto, so the closest genuine one-line
regression is routing the close through the resource every connection's
subscription actually shares (`Campaign`/`Store`) — `store.Close()`'s
subscriber sweep then forces every socket closed via each connection's own
overflow force-close path (item 5's mechanism), which is exactly the
isolation violation `TestMalformedFrameClosesOnlyThatConnection` guards
against.

---

## 4. Methodology note: `--workers 1 --timeout-coefficient 10`

The first real `unleash` run (`internal/engine`, gremlins' default flags)
reported 10 of its 11 mutants as `TIMED OUT` in 7.4 seconds — implausible
for genuinely slow tests. gremlins computes its per-mutant timeout as a
small multiple of a single fast baseline measurement, then runs one worker
per CPU by default; with 8 workers compiling/running `go test` concurrently
on this machine, real per-mutant run time under contention regularly
exceeded the baseline-derived budget, producing false timeouts that had
nothing to do with the mutants themselves. Isolating the variable
(`--workers 1`, same package, same code) reproduced the dry-run's true
count exactly: 10 killed, 1 lived, 0 timed out, matching the 11-mutant
baseline the task's setup validated. `--timeout-coefficient 10` gives
headroom on top of that so genuine slowness (not contention) still gets a
fair budget. Every real run in this audit uses both flags; this is a
command-line methodology choice, not a committed gremlins config file (none
was added).

---

## 5. Cadence proposal (closing)

Mutation testing is valuable but expensive (minutes per package, not
seconds) and its findings are almost entirely about **test-suite gaps that
regular coverage can't see** (boundary values, empty-input edge cases,
write-only fields) rather than active bugs — of the 11 total survivors in
this audit, all 5 non-equivalent, non-timed-out ones were coverage gaps at
exact numeric/nil boundaries, none were live defects in the audited code
itself. That profile argues for running it periodically rather than on
every commit:

- **Not in `task check`.** Confirmed deliberately excluded — a
  multi-minute gate on every commit/PR is the wrong cost/benefit trade for
  a check that catches missing tests, not the regressions `check` itself
  already gates (vet, race tests, contract drift/breaking, vocabulary,
  architecture).
- **Post-merge, per sub-project.** Run the four state-owning packages'
  mutation suite once a sub-project's branch lands on `main` (not per
  task, not per PR) — frequent enough to catch newly-introduced coverage
  gaps before they compound across several sub-projects, infrequent enough
  to stay a deliberate, reviewed activity rather than routine overhead.
  `task audit:mutation` (added this task, NOT wired into `check`) is the
  one-command entry point for that cadence.
- **New packages join the rotation as they're added.** `internal/gateway`
  was deliberately left out of the machine run this task performed (its
  authz/identity/WebSocket-transport surface doesn't fit gremlins' single-
  package token-mutation model as cleanly as the other four — hence the
  hand-injection list instead); a future cadence pass should decide
  whether a package-scoped gremlins run adds anything gremlins' token
  mutators can reach there beyond what the hand-injection list already
  covers scenario-by-scenario.
- **Equivalent-mutant and infinite-loop dispositions carry forward.** The
  four `campaign.go:264` map-capacity-hint mutants and the two
  `seq++`/`seq--` timeout mutants are structural properties of the code
  shape, not this snapshot of it — they will keep reappearing on every
  future `campaign` run until that code is refactored, and should be
  recognized as "already adjudicated" rather than re-triaged from scratch
  each cadence pass.

Full command-by-command transcripts, the equivalent-mutant proof program,
and all fifteen fault-injection cycles (5 survivor-triage + 10 gateway) are
in the session-local report at `.superpowers/sdd/p5-task-3-report.md`.
