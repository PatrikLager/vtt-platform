# LLM-Native Virtual Tabletop Platform — Design Spec

**Date:** 2026-07-23
**Status:** Approved design (brainstorming output)
**Working name:** `vtt-platform` (placeholder — rename freely)
**Predecessor:** RPTool (MapTool + MTScript + rule-framework), kept untouched as reference material

---

## 1. Vision

A new virtual tabletop platform, built from scratch. RPTool/MapTool is inspiration, not a
porting target. The defining feature: **an LLM is a first-class participant at the table** —
up to and including acting as the Dungeon Master — interacting through the same APIs as every
human client.

Game systems are pluggable **rule modules**: D&D 4.5e is the first, but the engine is built
so a RuneScape-flavored or Middle-earth module can be added without engine changes.

Must-have modern features beyond the LLM DM:

- **Voice at the table** — the AI DM speaks (TTS) and listens (STT) during sessions.
- **AI content generation** — maps, NPC portraits, tokens, handouts generated mid-session.
- **Campaign world layer** — journals, NPC/location wikis, plot tracking as structured data
  the LLM can read and write; the world, not just combat.
- **Tablet/phone play** — players join from touch devices with a responsive client.

**Audience (assumption):** self-hosted for the owner's own table first. A future public
release is not planned for, but nothing in the design forecloses it (no hardcoded campaign
data, clean module boundary).

## 2. Non-Goals

- **No feature parity race with Foundry VTT.** Its canvas polish, dynamic lighting engine,
  and content ecosystem are a decade of work; we compete on architecture, not breadth.
- **No embedded macro language.** MTScript-class limitations (nesting caps, CSV state,
  untestability) must not be recreated. Extensibility comes from rule-module data and the
  public API, never from an in-app scripting layer.
- **No porting of rule-framework code.** The TypeScript rule-framework validated ideas
  (declarative templates, conditions, testing discipline) under MTScript's constraints; the
  new platform reuses the *lessons*, not the code.

## 3. Design Pillars (locked)

These four decisions are foundation-level. Foundry VTT cannot retrofit them; they are the
reason this platform exists.

- **P1 — API-first, headless core.** Every game action (move token, roll attack, apply
  condition, reveal fog, narrate) is a structured, permission-checked API operation with a
  subscribeable event stream. The human UI is just another API client. The LLM gateway falls
  out of the API; it is never a side channel.
- **P2 — Rules as declarative data.** Game logic lives in rule-module data interpreted by
  one engine. No game-system concept appears in engine code. Rules are testable, diffable,
  hot-reloadable, and LLM-legible in both directions (the AI DM can read a power to
  understand it and write a new one as data mid-session).
- **P3 — Event-sourced state.** The append-only game log is the source of truth; current
  state is derived. This yields undo, session replay, spectator catch-up, auditability
  ("why is my HP 12?"), and a perfect LLM context feed.
- **P4 — Engine ↔ rule-module boundary.** The engine knows actors, scenes, turns, effects,
  dice — never healing surges. The boundary is proven by a deliberately tiny second "toy"
  module that must pass the same conformance suite as 4.5e without any engine change.

**Meta-pillar (from ckeletin-go): enforcement by automation.** Every pillar has a
machine-checkable guard (Section 8). Unenforced rules erode.

## 4. Architecture (Section A)

Two-layer architecture: **Go server + thin TypeScript browser client**, bridged by a
schema-generated contract.

```
┌────────────────────────── Go Server (auditable core) ──────────────────────────┐
│  Event Store (SQLite)   append-only event log; state derived, never mutated    │
│  Game Engine            state derivation, scene/turn lifecycle                 │
│  Rules Interpreter      executes rule-module DATA (no game logic in Go)        │
│  Module Loader          loads rule modules: schemas + content + LLM context    │
│  World Store            journals, NPCs, locations, plots — structured,         │
│                         versioned, LLM-read/writable                           │
│  Asset Service          maps/portraits/audio, incl. AI-generated media         │
│  Permissions            roles: DM, player, spectator, agent (scoped LLM)       │
│  API Gateway            WebSocket + HTTP — the ONLY way in, for every client   │
└─────────────┬─────────────────────┬─────────────────────┬──────────────────────┘
              │                     │                     │
       TS Browser Client     MCP Adapter (LLM)     Simulation Harness
       PixiJS map, schema-   tools generated        scripted headless players
       driven sheets, chat,  from schemas; event    driving the real API
       dice, touch-first UI  stream as context      before any UI exists
```

### 4.1 Contract layer

Schemas are the single source of truth, defined once and code-generated into three
consumers: Go types, TypeScript types, and LLM tool definitions. The concrete schema
format (protobuf vs JSON Schema vs OpenAPI) is deliberately deferred to sub-project 1.
CI regenerates from schemas and fails on drift.

### 4.2 Server components

- **Event store:** append-only, SQLite-backed. Only the event-application package writes
  state. Subscriptions fan out events to connected clients (human and LLM alike).
- **Game engine:** derives current state from the log; owns generic concepts only —
  actors, tokens, scenes, grids, turn framework, effects framework, dice, chat.
- **Rules interpreter:** the single execution point for rule-module data. Turn *structure*
  is module-defined (a module may schedule real-time ticks instead of rounds; the engine
  just processes events).
- **Permissions:** four roles — DM, player, spectator, **agent**. The agent role makes
  "LLM as full DM" a permissions question, not an integration hack: an agent can hold DM
  authority, and humans can always take over or scope it down.
- **API gateway:** WebSocket for live sessions + HTTP for CRUD/assets. There is no other
  path into game state.

### 4.3 Client

Deliberately thin: renders state, submits intents. No rules execution client-side; the
code the owner cannot audit is also the code that cannot corrupt a campaign. PixiJS for
map/token rendering; character sheets generated from module schemas; responsive,
touch-first layout (tablet/phone play is a launch requirement, not a retrofit).
Optimistic-UI is out of scope initially — round-trips to a LAN/VPS server are fine for
turn-based play.

### 4.4 LLM integration

MCP adapter over the public API (official Go MCP SDK). Tools are generated from the same
schemas as everything else. The event stream is the LLM's context feed. Voice needs no
special foundations: STT produces ordinary chat/intent events; the AI DM's narration
events feed TTS in the client — "events carry narration" is the only requirement.

### 4.5 Server binary and CLI

The server ships as a single binary that is itself a CLI: `vtt serve` plus
subcommands (`vtt version`, `vtt module validate`, `vtt campaign export`, …).
The ckeletin-go CLI scaffold (ultra-thin commands, Viper config precedence) is
a candidate for this command shell — decided in sub-project 3. **Guardrail:**
every subcommand that touches game state is an API *client*; only pre-runtime
operations (`serve`, `migrate`) may bypass the API. The CLI must never become
a side door past P1.

### 4.6 Rule modules

A module is a data package containing:

- **Schemas** — character sheet shape, power/item/spell definitions
- **Rules logic as data** — how attacks resolve, action economy, defenses
- **Content** — conditions, powers, monsters, equipment
- **Sheet UI descriptors** — how humans view/edit system data
- **LLM affordances** — tool descriptions and rules context that teach the AI DM to run
  this system (install the 4.5e module and the LLM knows "1d20 + STR vs. AC" and what
  Dazed does)

First real module: minimal D&D 4.5e. Boundary proof: a toy skirmish module (a weekend's
work) passing the same conformance suite with zero engine changes.

## 5. Build Order (Section B) — Foundations First

The platform is built horizontally before any session is played. Risk mitigation for the
"no play validation" problem: the **simulation harness is built alongside the foundations,
not after** — scripted campaigns exercise every API headlessly, and double as the LLM's
dress rehearsal later.

Each sub-project gets its own design → plan → implement → review cycle:

1. **Contract & codegen pipeline** — schema format decision, generators (Go/TS/LLM tools),
   versioning discipline. Everything depends on this.
2. **Event core** — append-only store, subscriptions, state derivation, replay/undo,
   campaign persistence.
3. **API gateway & permissions** — WebSocket/HTTP surface, session auth, four roles
   (agent role designed on day one).
4. **Simulation harness** — built alongside 2–3: scripted headless players driving the
   real API, realized as a CLI API client (`vtt client --script`, `vtt events tail`,
   `vtt state dump`) that doubles as debug tooling and the first working proof of P1.
5. **Module loader & rules interpreter** — module package format, minimal 4.5e module,
   toy module + conformance suite.
6. **MCP/LLM gateway** — tools from schemas, event-stream context feed, agent-role
   enforcement.
7. **Client foundations** — thin shell: map rendering, tokens, chat, schema-driven
   sheets, tablet-first responsive layout.
8. **World layer** — campaign wiki as structured, LLM-read/writable data.
9. **Asset service & AI content generation** — media storage, generation hooks.

Then feature buildout, each its own cycle: full 4.5e module, voice pipeline, polish.

**Foundations exit criteria:** a full simulated combat encounter runs headlessly through
the real API (harness as both DM-agent and players); the toy module passes conformance;
`task check` is green with all Section 8 gates active.

## 6. Working Model (Section C)

- Every sub-project runs the dev-cycle protocol: investigate → plan (todo list, user
  sign-off) → develop → review → land. Nothing lands before review sign-off.
- **Ownership:** Patrik owns and reviews the Go core (his language; Claude's Go must
  survive his reading). Claude owns the TS client under functional review. **Schemas are
  always decided together** — they are the constitution of the system.
- **Gates:** `go test` + simulation harness + `task check` are the merge gates.
- New repo, fresh history. RPTool remains untouched as reference (4.5e PDFs, the old
  macro doc as feature checklist: auras, recharge, ammunition, multi-size flanking, undo).

## 7. Lessons Carried from RPTool and Foundry

- From **Foundry** (adopt): self-hosted + own-your-data deployment, browser clients,
  system-agnostic core with installable systems, document/effects data model.
- From **Foundry** (avoid): extension by monkey-patching an exposed client (libWrapper-style
  collision management, per-version ecosystem breakage); no headless API; imperative rules
  spread across sheet classes; mutable state with no history.
- From **RPTool old system** (adopt): the ChangeLog/undo instinct (generalized by P3);
  the feature checklist above.
- From **RPTool old system** (avoid): per-power copy-paste macros, positional CSV state,
  overloaded properties, honor-system conventions.
- From **rule-framework** (adopt): declarative templates/conditions, test-first discipline,
  schema validation of all content.

## 8. Repo Discipline & Enforcement (Section D)

Adopted from ckeletin-go's discipline layer (its CLI scaffold is not used):

- **Task** (taskfile.dev) as the single quality gateway: `task check` runs everything,
  identically in local dev, lefthook pre-commit hooks, and CI.
- **golangci-lint** + **go-arch-lint** with our layer map: gateway → engine → event store;
  no layer skipping; only the event-application package writes state.
- **semgrep pillar guards:**
  - P2/P4: game-system vocabulary (`healingSurge`, `dailyPower`, `fortitude`, …) is
    forbidden in engine packages — banned-word list maintained alongside the 4.5e module.
  - P3: direct state writes outside the event-application package are forbidden.
- **Contract drift gate:** CI regenerates all code from schemas and fails on diff.
- **Module conformance gate:** 4.5e and the toy module pass the same suite in CI, forever.
- **Coverage gate:** ≥85% on the Go core, race detector on.
- **ADR discipline:** decisions live in `docs/adr/`, machine-checkable where possible.
  Founding ADRs, recorded on repo creation:
  - ADR-001: API-first headless core (P1)
  - ADR-002: Rules as declarative data (P2)
  - ADR-003: Event-sourced state (P3)
  - ADR-004: Engine ↔ rule-module boundary (P4)
  - ADR-005: Two-layer stack — Go server, thin TS client, schema-generated contract
  - ADR-006: Foundations-first build order with simulation harness
- **AI configuration stack:** AGENTS.md + CLAUDE.md rules cascade + hooks, so the
  guardrails bind Claude-written code exactly as they bind human-written code.
- **goreleaser** for single-binary server distribution.

## 9. Open Questions (deferred, with owners)

- **Project name** — placeholder `vtt-platform`; Patrik picks when ready. (Rename is a
  directory + module-path change; do it before sub-project 1 lands to avoid churn.)
- **Contract format** (protobuf vs JSON Schema vs OpenAPI) — the *first decision inside*
  sub-project 1, not before: it needs a day of comparative prototyping against real
  event/tool-definition needs.
- **Auth mechanism** for table members (invite links? passwords?) — decided in
  sub-project 3.
- **LLM provider surface** — MCP-first assumed (Claude-native); whether to also expose a
  raw REST surface for other agents is decided in sub-project 6.
- **Hosting target** (home server vs VPS) — affects nothing architecturally (single
  binary + SQLite); decide at first deployment.
