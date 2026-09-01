# Web Client — Design Spec (sub-project 7)

**Date:** 2026-07-26
**Status:** Approved design (brainstorming output, Patrik remote)
**Parent:** platform spec build-order item 7 ("thin TS client"); ADR-007
(generated TS contract types are the client's vocabulary); pillars P1-P3.
The assets layer (maps/token art) is NOT this sub-project — the visual
design leaves slots for it.

## 1. Purpose

The table becomes something humans see and drive. One embedded web
client serves every role from the same URL: spectators watch, players
act, a human DM runs the whole table — while the LLM DM remains an
equal wire client through MCP. The client is pillar P3 made visible:
it folds the event stream into its own view state; the wire is the only
state channel.

## 2. Decisions (locked in brainstorming, 2026-07-26)

1. **v1 scope = full console:** watch + play + DM management UI.
2. **Delivery = embedded:** the built client ships inside the `vtt`
   binary (go:embed) and serves from the same address as the wire.
   Browser → paste invite token → play. Single-binary posture holds.
3. **Look = clean geometric:** dark table aesthetic; tokens as labeled
   discs with hp rings and condition badges; readable narration feed.
   Asset-ready underneath (art slots in later without rework).

## 3. Architecture

- **Client-side fold (P3):** the client connects to the existing WS
  wire (protojson ClientCommand/ServerFrame, gen/ts types), replays the
  event stream from sequence 0, and folds its own state — a TS mirror
  of the Go fold's OBSERVABLE semantics, pinned by cross-language
  parity goldens (§6). Live events keep folding; reconnect replays.
  No REST state endpoint exists or is added; the log is the truth.
- **Metadata over HTTP (new, wire-contract untouched):** things a UI
  needs that are content rather than campaign state — the served
  ruleset's ability list (id, name, targeting) and guide, the available
  adventures (id, name) and their guides — are served by `vtt serve` as
  read-only HTTP GET endpoints next to the static files (same
  authorization: invite-token header; DM/agent-only for adventure
  guides since they carry secrets — players get the ruleset guide
  only). This parallels the MCP flags (`--ruleset`/`--adventures-dir`)
  rather than adding wire commands; the event stream remains the only
  STATE channel. Endpoint shapes are internal to serve (not part of the
  frozen proto contract) and are documented in the client README.
- **Auth:** token paste on first load → verify-before-upgrade WS
  connect (existing mechanism) → role + participant shown; token kept
  in localStorage (documented: browser-local, same trust posture as the
  MCP env var).
- **Stack:** TypeScript + Vite, minimal dependencies, generated
  contract types from gen/ts; component approach is an implementation
  decision recorded in the plan (no heavy framework). Repo layout:
  `client/` at the root; build output committed to a deterministic
  `cmd/vtt/webdist/` (no content-hash filenames) embedded via go:embed
  and drift-gated like tools.json.

## 4. Features by role (v1)

**Everyone (spectator floor):** scene grid with tokens — discs showing
the actor's initial, ALL resources as small current/max chips, and ALL
conditions as dots with hover names (nothing genre-named; the platform
stays generic — a "primary resource ring" needs ruleset client-hints,
deferred to §9); the story feed (narration interleaved with mechanical
events, chronological, anchored narration grouped with its events);
notes panel; session status; live updates; event ticker with sequence
numbers.

**Player (+):** select an actor you control; move your token
(click-to-move on the grid); use an ability (ability picker from the
HTTP metadata, click-to-target on the grid, result toast with rolls);
speak (narration box, optional in-character `as`).

**DM (+):** act as ANY actor; session start/end; create scene; add
actor (form: id/name/attributes/resources, plus raw-JSON paste mode);
place token; load adventure (picker from HTTP metadata, guide viewer);
notes editor (upsert/delete); remove condition (from a token's badge);
undo (retract last event or a chosen recent range from the ticker, with
confirmation). Invite management stays CLI (non-goal).

*AMENDED 2026-08-30 — the undo controls were built and are gone.* Patrik's
ruling of 2026-08-30 removed retraction from the platform; the DM console's Undo
buttons, `client/src/undo.ts` and the feed's retraction rendering left in
`d3e2f28` (sub-project 13, `2026-08-30-retraction-leaves-design.md`). The DM
surface gained rather than shrank overall: door controls and a map picker
(sub-project 12), and the two removal commands that replace undo —
`remove_token` and `remove_actor`. `client/test/command-surface.test.ts` is the
live answer to "which surface issues each command", and it fails when a
`ClientCommand` arm has nowhere a human can reach it.

## 5. Serving

`vtt serve` gains the static handler (embedded dist at `/`, wire at
`/ws` as today) and the metadata endpoints. No new flags: the client is
always served; metadata endpoints reflect whatever `--ruleset`/
`--adventures-dir` provided (absent → clean empty responses the UI
renders honestly).

## 6. Testing (ADR-009, adapted for a UI)

- **Fold parity goldens (the keystone):** the TS fold consumes recorded
  event streams and must produce the same observable state as the Go
  fold. Fixtures: the committed scenario library's event streams
  (generated deterministically by a Go helper that runs each scenario
  self-contained and dumps the log + final state as JSON). bun test
  asserts TS-fold(stream) == dumped-state for EVERY committed scenario
  — cross-language drift fails CI forever. RED-first: fixtures land
  before the fold implementation.
- **Component/unit tests (bun):** story-feed grouping (anchors), grid
  geometry (click→coordinate), command construction (protojson shapes).
- **E2E (Playwright, headless):** against a real `vtt serve` with
  ruleset+adventures: token login per role; spectator sees the table;
  player moves + uses an ability end-to-end; DM loads an adventure and
  the table populates; role gating (player sees no DM console; DM
  guide endpoints deny players). Screenshot artifacts at each key
  screen — these are the remote demo medium.
- **Gates:** `task check` gains client unit tests + build + embed-drift
  check. Playwright e2e runs as its own task (browser dependency),
  mandatory in reviews and the demo, not in the default check loop.
- Go-side: serve's static/metadata handlers get standard Go tests incl.
  authz on the guide endpoints (players denied adventure guides).

## 7. The demo gate (remote-friendly)

Headless run against goblin-ambush: screenshots of (1) token login, (2)
the loaded table mid-fight with story feed, (3) the DM console with the
adventure guide open, (4) a player's ability result toast — sent to
Patrik's phone. His accept merges; a live look when he's home is the
victory lap, not the gate.

## 8. Non-goals (YAGNI)

Assets/maps/token art (next sub-project — the grid renders plain
squares); invite management UI; mobile layout polish (desktop-first,
degrade gracefully); animations beyond basic transitions; presence/
typing indicators; multi-campaign switching (one server = one campaign,
as today); offline/PWA; i18n; accessibility beyond semantic basics
(a11y pass is named residue, not skipped silently — headings, contrast,
keyboard nav for core actions).

## 9. Open questions (deferred, with owners)

- Filtered story read for very long campaigns (client currently replays
  all; fine for demo-scale) — revisit with real campaign sizes; the
  world-layer residue item lands here.
- Client hints in ruleset format (e.g. which resource is "primary" for
  ring display) — a v2 ruleset-format question, with the assets layer.
- Human-DM narration anchoring UX (auto-anchor to selected events?) —
  refine after first real table use.
