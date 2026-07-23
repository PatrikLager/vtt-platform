# ADR-002: Rules as declarative data

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** The RPTool/MapTool predecessor hardcoded logic per power as imperative
MTScript macros — untestable, uncopyable across powers, and opaque to anything but
a human macro author. The platform must support pluggable game systems (D&D 4.5e
first, a RuneScape-flavored or Middle-earth module later) and an LLM DM that can
both read and author rules content mid-session.
**Decision:** Game logic lives in rule-module data interpreted by one engine. No
game-system concept appears in engine code (machine-enforced by a semgrep
vocabulary ban). Rules are testable, diffable, and LLM-legible/authorable as data.
**Consequences:** The engine needs a rules interpreter and module loader rather
than per-system code paths; a banned-word list (`healingSurge`, `dailyPower`,
`fortitude`, …) must be maintained alongside the 4.5e module and enforced in CI.
New game systems become installable content, not engine forks.
