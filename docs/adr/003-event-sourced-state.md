# ADR-003: Event-sourced state

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** Foundry-style mutable state with no history makes "why is my HP 12?"
unanswerable and undo impossible; the RPTool old system's ChangeLog instinct
pointed the right direction but was never generalized. An LLM DM additionally
needs a reliable feed of what happened at the table, not just a snapshot of where
things stand now.
**Decision:** The append-only game log is the source of truth; state is derived,
never mutated in place. Only the event-application package writes state. Yields
replay, audit, and the LLM context feed.
**Consequences:** The event store is SQLite-backed and append-only, with
subscriptions fanning events out to human and LLM clients alike; a semgrep guard
forbids direct state writes outside the event-application package.

**Enforcement (added 2026-07-30).** That semgrep guard did not exist until
today — this ADR named its own enforcement for a week and did not have it,
which is precisely the gap ADR-010 was written about. It is now
`.semgrep/event-sourcing.yml`, run by `task check:invariants` in the gate and
in the pre-commit hook, and verified to fire by injecting a violation in a
throwaway copy rather than trusted because it exited 0. One exclusion is
recorded with it: `internal/rules/conformance` builds a synthetic fixture
world to evaluate abilities against, which is never persisted and never
derived from a log, so the concern here — state no event explains — does not
apply to it. Test files are excluded too, and that exclusion is a known blind
spot, noted in the rule. Session replay and
spectator catch-up fall out of the log for free rather than needing bespoke
implementations.

**Amended 2026-09-01 — undo is not one of the things this yields.** Two
sentences above, and the Context line's "undo impossible", named undo as a
consequence of the append-only log. `campaign.Undo` was deleted by `133e896`
(sub-project 13, spec `2026-08-30-retraction-leaves`) on Patrik's 2026-08-30
ruling: a retraction exists to make something not have happened, and it cannot
— the player already read the log. Nothing replaced it, and nothing may: the
platform's answer to a mistake is a further event that removes the thing going
forward (`remove_token`, `remove_actor`), which the log records as having
happened. This is a factual correction to what the decision delivers, not a
change to the decision: an append-only log is still the source of truth, still
derived-never-mutated, and still the reason replay, audit and the LLM feed are
free. The Context line's complaint about Foundry-style mutable state stands on
its first half — "why is my HP 12?" is unanswerable there — and the "undo
impossible" half is left as written because it records what motivated the
decision in 2026-07-23, not what the platform offers today.
