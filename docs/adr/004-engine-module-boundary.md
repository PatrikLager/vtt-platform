# ADR-004: Engine ↔ rule-module boundary

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** Foundry's system-agnostic core with installable systems is worth
adopting, but without a hard, enforced boundary the temptation to leak
game-specific concepts into the engine — as RPTool's overloaded properties and
per-power macros did — reappears over time. The platform must support systems
beyond D&D 4.5e without engine changes.
**Decision:** The engine knows actors, scenes, turns, effects, dice — never
healing surges. Proven permanently by a toy second module passing the same
conformance suite as D&D 4.5e with zero engine changes.
**Consequences:** A deliberately tiny toy skirmish module (a weekend's work) must
exist alongside the 4.5e module. The module conformance gate runs both modules
through the same suite in CI, forever, so any future coupling between the engine
and 4.5e-specific concepts is caught immediately rather than discovered when a
second system is finally attempted.
