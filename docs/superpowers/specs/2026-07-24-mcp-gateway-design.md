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

## 4. Tools

> The count that stood in this heading is withdrawn, along with the "seven"
> that used to open the first bullet — see §10's 2026-08-25 amendment. Both
> were accurate when written and went stale by addition. What remains below
> describes the KINDS of tool, which is the part that has held.

- **Command tools, loaded from the committed `contract/gen/tools/
  tools.json`** — generic dispatch: tool name → ClientCommand oneof field
  by matching the manifest's message; arguments are the protojson command
  body verbatim. Zero per-command code; two tests plus a startup check
  guarantee every future command appears — see §10 for which link each holds.
  Tool result: the
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

## 10. Amendment (2026-07-25, sub-project 5a merge)

**AMENDED AGAIN 2026-08-25 (Patrik's ruling): the tool surface is stated as an
INVARIANT, not a count. §4's "nine" and this section's "TWELVE" are both
withdrawn — not because either was wrong, but because both were right and then
stopped being.**

The surface is:

- **Every campaign command in the contract is a tool.** The manifest is
  generated and `server.go` registers each entry through one shared handler, so
  a new command costs ZERO per-command MCP code — which is what §4 was really
  promising and what the count was standing in for.

  That invariant is TWO LINKS:

  1. contract command → manifest entry —
     `toolgen.TestManifestCoversAllCommandMessages`
  2. manifest entry → registered tool —
     `TestEveryManifestCommandIsRegisteredAsATool`, plus `buildDispatch`'s
     symmetric startup check, which makes any disagreement between tools.json
     and the ClientCommand oneof an error from `mcp.New` rather than a silent
     gap.

  Link 2 was not unguarded before 2026-08-25 — a skipped registration already
  reddened two tests. What it lacked was a SELF-MAINTAINING guard: the existing
  ones lean on `wantCommandToolNames`, a hand-written list, and that list has
  silently stopped growing before (`load_adventure`, recorded in tools_test.go).
  A command that reaches the manifest, never reaches the list, and is skipped by
  the loop was green. Test 2 reads the generated manifest instead, so it covers
  a command added tomorrow without anyone editing anything.
- **Plus a small hand-written set registered in Go**, each with its own
  behaviour and its own tests: `get_state`, `get_events_since`,
  `get_ruleset_guide`, `get_adventure_guide`, `get_join_link`,
  `get_participants`.

WHY NO NUMBER. A count is a fact about how many rows a generated file has. It
changes every time the contract grows, it says nothing about whether any
particular tool works, and most of what it counts is not even separate code —
the command tools are one dispatch path, not N implementations. Three counts in
three places had already gone stale by addition (§4's nine, this section's
twelve, README's nine) and every one of them was accurate on the day it was
written. What a reader needs is that the tool they are about to call exists and
does what it says; that is now asserted rather than asserted-about.

The changes this amendment originally recorded, which stand:

- `use_ability` + `remove_condition` auto-appeared from the toolgen
  manifest — zero per-command code, as §4 already guaranteed.
- `get_ruleset_guide` added: `vtt mcp --ruleset` flag takes a startup
  snapshot of `guide.md` and serves it verbatim. Server-authoritative guide
  delivery (the MCP server reading the guide live from a running `vtt
  serve` instead of its own snapshot) is a 5b/v2 contract-addition
  candidate, not built here.
