# ADR-005: Two-layer stack

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** Foundry spreads imperative rules across client-side sheet classes
with no headless API, so the code the owner cannot fully audit is also the code
that can corrupt a campaign. The platform's owner (Patrik) needs to be able to
read and trust the auditable core, while the browser client only needs to render
state and submit intents.
**Decision:** Go server (auditable core, single-binary deploy) + thin TypeScript
browser client, bridged by types generated from a single schema source. Client
renders state and submits intents; no rules execution client-side.
**Consequences:** Schemas are the single source of truth, code-generated into Go
types, TypeScript types, and LLM tool definitions; CI regenerates from schemas and
fails on drift. Patrik owns and reviews the Go core, Claude owns the TS client
under functional review, and schemas are always decided together. The client
stays thin (PixiJS rendering, schema-driven sheets); optimistic UI is deferred
since round-trips to a LAN/VPS server are fine for turn-based play.
