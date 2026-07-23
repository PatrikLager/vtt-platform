# ADR-003: Event-sourced state

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** Foundry-style mutable state with no history makes "why is my HP 12?"
unanswerable and undo impossible; the RPTool old system's ChangeLog instinct
pointed the right direction but was never generalized. An LLM DM additionally
needs a reliable feed of what happened at the table, not just a snapshot of where
things stand now.
**Decision:** The append-only game log is the source of truth; state is derived,
never mutated in place. Only the event-application package writes state. Yields
undo, replay, audit, and the LLM context feed.
**Consequences:** The event store is SQLite-backed and append-only, with
subscriptions fanning events out to human and LLM clients alike; a semgrep guard
forbids direct state writes outside the event-application package. Session replay,
spectator catch-up, and undo fall out of the log for free rather than needing
bespoke implementations.
