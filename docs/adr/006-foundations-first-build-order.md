# ADR-006: Foundations-first build order

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** Building a full session's worth of features before any API is
exercised end-to-end risks discovering architectural problems only after a lot of
UI and content work is sunk into them — the classic "no play validation" problem.
The platform must also prove, early, that an LLM can act as an API client, not
just a human's browser.
**Decision:** Platform built horizontally before first play; the simulation
harness is built alongside the foundations (not after) so every API is exercised
by scripted play from the start. Exit criteria per spec §5.
**Consequences:** Each sub-project (contract/codegen, event core, API gateway,
simulation harness, module loader, MCP gateway, client, world layer, asset
service) runs its own investigate → plan → develop → review cycle. Foundations
are not done until a full simulated combat encounter runs headlessly through the
real API as both DM-agent and players, the toy module passes conformance, and
`task check` is green with all Section 8 gates active — only then does feature
buildout (full 4.5e module, voice pipeline, polish) begin.
