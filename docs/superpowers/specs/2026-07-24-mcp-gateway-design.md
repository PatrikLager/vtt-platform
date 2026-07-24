# MCP Gateway — Design Spec (sub-project 6)

**Date:** 2026-07-24
**Status:** Approved design (brainstorming output)
**Parent:** Platform spec §4.4; pillars P1 (the MCP server is a wire client);
ADR-007 (tools.json is generated from the contract); sub-project 4's client
core, fold, and headSequence contract.

## 1. Purpose

`vtt mcp`: an MCP stdio server that seats an LLM at the table as the agent
participant. It is a WIRE CLIENT — the harness core's second consumer — and
adds zero privilege: every tool call becomes a `ClientCommand` judged by the
gateway's authz table server-side, stamped with the agent's participant_id.

## 2. Decisions (locked in brainstorming)

1. **Read surface = two poll tools** (`get_state`, `get_events_since`) —
   lowest-common-denominator across MCP hosts, deterministic, testable.
   Push notifications are v2.
2. **Acceptance = the live demo:** merge waits until Patrik has watched
   Claude DM a short session via Claude Code against a running `vtt serve`.

## 3. Architecture

- `internal/mcp`: MCP stdio server built on the official Go MCP SDK (pinned
  exact at implementation, recorded). Consumes `internal/harness` (Client,
  Fold) and contract types. arch-lint: `mcp → {harness, engine, contract,
  mcp(self)}` + the SDK as vendor — server internals FORBIDDEN (same P1
  rule, tests included).
- `cmd/vtt`: `vtt mcp --server ws://… [--token …]` (env `VTT_TOKEN` honored;
  flag wins). Ultra-thin per ADR-008. One WebSocket connection held for the
  process lifetime; on connection loss the MCP server reports tool errors
  and attempts reconnect with catch-up (harness reconnect semantics).

## 4. Tools (nine)

- **Seven command tools, loaded from the committed `contract/gen/tools/
  tools.json`** — generic dispatch: tool name → ClientCommand oneof field
  by matching the manifest's message; arguments are the protojson command
  body verbatim. Zero per-command code; the manifest-completeness test
  already guarantees every future command appears. Tool result: the
  `CommandResult` protojson (ok / error / sequence — read-your-writes).
- **`get_state`** — the dump contract verbatim: folded state JSON + top-level
  `headSequence`.
- **`get_events_since {afterSequence, limit}`** — protojson envelopes after
  the given sequence (bounded; default limit 50, max 200) + current
  `headSequence`. The game's who-did-what memory.

## 5. Instructions (server-level, deliberately thin)

Role (you are the agent participant; humans can see everything you do —
every event carries your participant id), the poll pattern (act → check
result → get_events_since for others' actions), and the wire conventions
(int64-as-string; oneof envelope keys; moduleData is opaque). NO game rules
— rule-module LLM affordances are sub-project 5's layer.

## 6. Security posture

Agent-role invite token only (mint: `vtt invite --role agent`); the token
grants exactly what the authz table grants agents (everything in-game,
nothing about invites). tokens are never logged; VTT_TOKEN env avoids
tokens in shell history / MCP config files where the host supports env.
README gains the Claude Code `.mcp.json` snippet + token-handling note.

## 7. Testing & exit criteria (ADR-009 binding)

- In-process: MCP SDK client ↔ stdio (or in-memory) transport ↔ `internal/
  mcp` ↔ real composeServer-backed gateway. Stub-first behavioral RED.
- Exit test: the MCP client plays `smoke.json`'s command list THROUGH THE
  TOOLS; `get_state` equals the harness Fold expectation incl. headSequence;
  `get_events_since` pagination walks the full log; a spectator-token run
  surfaces the authz denial as a clean tool error (connection intact).
- Injection proofs for after-the-fact tests; workflow-level final review;
  THEN the live demo (§2.2) gates the merge.

## 8. Non-goals (YAGNI)

No push notifications/sampling/resources. No rules knowledge. No narration
persistence (world layer). No practice-loop automation. No multi-agent
seats (one agent connection per `vtt mcp` process).

## 9. Open questions (deferred, with owners)

- Reconnect-with-catch-up depth (resume from last seen sequence vs after=0
  + fresh fold): decided at plan time against harness reconnect semantics.
- Whether get_events_since should filter event kinds: v2, driven by real
  LLM usage.
