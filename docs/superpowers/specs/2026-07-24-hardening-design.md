# Hardening Pass — Design Spec (sub-project 3.5)

**Date:** 2026-07-24
**Status:** Approved design (conversation, Patrik 2026-07-24)
**Parent:** ADR-009 (airtight TDD) applied retroactively; motivated by the
regression-net rationale: post-prototype changes must be caught by a net
whose every strand is PROVEN able to fail.

## 1. Purpose & scope

Three workstreams, one branch (`feat/hardening`):

1. **session_id stamping** (Patrik-decided at the P4 merge gate): campaign
   stamps `Envelope.session_id` under its lock. SessionStarted events get a
   fresh campaign-generated session id; all other events (and Undo markers)
   get the currently-open session's id; no open session → empty (legitimate:
   out-of-session table setup). `campaign.Undo` drops its now-redundant
   `sessionID` parameter (Go signature churn is fine; additive-only binds the
   CONTRACT, not internal signatures).
2. **Teeth audit (retroactive ADR-009 rule 3):** machine mutation testing via
   gremlins (validated working: 11 runnable mutants in engine, 100% mutator
   coverage on dry-run) over the fast-suite packages `internal/{store,engine,
   campaign,identity}`; every surviving mutant is triaged: new/tightened test,
   or documented equivalent-mutant acceptance. `internal/gateway`'s heavy
   suite (multi-second socket tests) gets HAND fault-injection instead, over
   an enumerated top-10 load-bearing assertion list (authz cells, ownership
   denial, verify-before-upgrade, catch-up equality, overflow close, backfill,
   attribution, marker broadcast, malformed-frame isolation, result/sequence
   match).
3. **Known-gap closure** (from the ledger): partial-overlap retraction range
   test; non-empty `CommandResult.error` round-trip fixture (Go+TS);
   toolgen `IsList` path exercised via `google.protobuf.ListValue`'s repeated
   field (no contract change needed); `store.Notify` seq-0 defensive guard +
   test; `vtt serve` composition path made testable (extract an internal
   compose helper returning a shutdownable server; RunE stays ≤30 lines) with
   an end-to-end boot→healthz→ws-roundtrip→shutdown test.

Explicit non-goals: rebuilding suites that pass audit; re-staging historical
red phases (ceremony, not evidence); mutation-testing generated code,
`contract-spike/`, or `cmd/` glue beyond the compose test; the
revoked-vs-unknown timing distinction (previously adjudicated out of scope).

## 2. Exit criteria

- Mutation runs on the four fast packages report ZERO surviving mutants, or
  each survivor is documented in the final report as an accepted equivalent
  mutant with reasoning, or as a proven-infinite-loop timeout (mutant makes a
  loop non-terminating; timeout = legitimate kill, verified by bounded-timeout
  hand-injection). (Category added at merge: the audit legitimately used it.)
- The gateway hand-injection list: all ten proven (injection transcript each).
- All ledgered gaps closed; session_id stamped per decision with behavioral-
  RED TDD (ADR-009 now binding).
- All gates green; property counts re-pinned if the session_id change
  legitimately shifts them (session ids becoming non-empty may alter state
  equality inputs — expected, re-pin deliberately, document).

## 3. Testing philosophy note

Mutation testing proves LOCAL fault-detection; it does not prove foresight of
cross-system interactions — that remains the property test's and scenarios'
job. Both kinds of net are required; this pass strengthens the first and
leaves the second untouched.
