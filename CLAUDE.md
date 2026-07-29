# vtt-platform — Agent Way of Working

LLM-native VTT platform. Event-sourced Go core, thin TS client (future),
protobuf contract. Start here: `docs/superpowers/specs/` (design specs),
`docs/adr/` (all decisions), `README.md`.

## Non-negotiable rules

1. **Airtight TDD (ADR-009).** Tests first, run RED before the solution
   exists; behavioral RED over compile-failure RED wherever a stub can
   compile; after-the-fact tests (keystone/scenario) need fault-injection
   proof per load-bearing assertion; tests pin boundary behavior, never
   internals. No impl-then-test, even with fully-specified interfaces.
2. **`task check` is the single quality gateway.** All gates green before
   any work is called done. Never weaken a gate to pass it; a gate change is
   its own reviewed decision.
   `task check:fast` (vet + lint + tier-1 tests) is an inner-loop
   convenience ONLY -- it is not this gate and never satisfies it. Tests are
   tiered by what they ARE, not by runtime: tier 1 unit/area, 2 cross-layer,
   3 external, 4 whole product (`task check`). A slow unit test stays in
   tier 1. See Taskfile.yml's tier comment.
3. **Contract evolution is additive only** (ADR-007; `check:breaking`
   enforces). Generated code is committed; regenerate via
   `task generate:contract`. Commands are imperative (`CreateScene`), events
   past-tense (`SceneCreated`).
4. **One fold.** `engine.Apply` (via `campaign.foldEvents`) is the only
   code that changes game state. Never add a second event-application loop.
5. **No game-system vocabulary in platform code** (pillar P2/P4; semgrep
   enforces). Rules concepts live in rule-module data (sub-project 5+).
6. **Review before commit.** Nothing lands unreviewed; the dev-cycle hook
   enforces this. Spec/plan prose changes need Patrik's explicit approval.
7. **Specs are truth.** If delivered behavior must deviate, the deviation is
   adjudicated, documented where readers look, and the spec amended at the
   merge gate — never silently.

## Layout

`contract/` wire constitution (protobuf, vtt.v1) · `internal/store` append-only
log · `internal/engine` state + fold · `internal/campaign` composition, undo,
poison contract · `internal/identity` invites/roles · `internal/gateway` WS
API + authz · `cmd/vtt` CLI · `tools/toolgen` MCP tool definitions ·
`contract-spike/` frozen ADR-007 evidence (read-only).
