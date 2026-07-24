# MCP Gateway Implementation Plan (sub-project 6)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver `vtt mcp` per the approved spec (`docs/superpowers/specs/2026-07-24-mcp-gateway-design.md`): nine tools (seven generic from tools.json + two read tools), thin instructions, agent-token security, in-process MCP test suite, ending at the live-demo gate.

**Architecture:** `internal/mcp` (MCP server on the official Go SDK; wire client via internal/harness; generic command dispatch from the committed tools.json) + `cmd/vtt` mcp subcommand. P1 rule extended: mcp may import harness/engine/contract/SDK only.

**Tech Stack:** existing + `github.com/modelcontextprotocol/go-sdk` (pin exact latest stable at implementation; RECORD the version; if the official SDK proves unusable at its current maturity, STOP and report BLOCKED — do not substitute an unofficial SDK without controller sign-off).

## Global Constraints

- Branch `feat/mcp-gateway` from `main`. Adapted review-before-commit flow; reviewers READ-ONLY (restated everywhere); injections in throwaway rsync copies.
- ADR-009 binding: stub-first behavioral RED; injection proofs for after-the-fact tests; boundary behavior only.
- arch-lint gains `mcp: { in: internal/mcp, mayDependOn: [harness, engine, contract, mcp] }`; `cmd` gains mcp. Vendor (SDK) free via depOnAnyVendor. Bite-proof required (mcp importing gateway → build fails).
- tools.json is consumed AS COMMITTED (no regeneration in this branch; contract untouched — drift/breaking trivially green).
- No secrets in logs or committed files; VTT_TOKEN env honored, flag wins; README config snippet uses env form.
- The heavy-suite serialization in task check: if internal/mcp tests run a composeServer they may be socket-heavy — add `./internal/mcp/...` to the SERIALIZED second go-test command in Taskfile check, not the parallel first (same contention rationale).

---

### Task 1: internal/mcp — server core + generic command tools

**Files:**
- Create: `internal/mcp/server.go` (SDK server, session lifecycle, instructions string), `internal/mcp/tools.go` (tools.json loading + generic dispatch), `internal/mcp/server_test.go`, `internal/mcp/tools_test.go`
- Modify: `go.mod`/`go.sum` (SDK pinned exact), `.go-arch-lint.yml`

**Interfaces (Tasks 2–3 depend on):**
```go
func New(cfg Config) (*Server, error)   // Config{WSURL, Token string; ToolsJSON []byte}
func (s *Server) Run(ctx context.Context, transport T) error  // T = the SDK's transport interface type — an explicit at-implementation resolution (the SDK's API is probed in Step 1); record the resolved signature in the report
// internal: dial via harness.Dial on Run; one connection per Run lifetime;
// reconnect on loss: redial with after=<last seen sequence>, dedupe by sequence (harness client semantics), tool errors while disconnected are clean MCP errors not crashes.
```
- Generic dispatch (the payoff): parse tools.json; for each tool, register an MCP tool whose handler wraps the raw JSON arguments as the protojson body of the matching ClientCommand oneof field (match via the toolgen manifest's message name ↔ oneof field descriptor — walk `vttv1.ClientCommand`'s descriptor at startup, build name→field map; NO per-command switch). Handler: build ClientCommand (fresh request_id via harness), SendCommand, return CommandResult protojson as the tool result (ok=false results are tool-level errors with the error string, isError=true per MCP semantics — decide and document which; binding: ok=false → MCP tool result with isError=true and the CommandResult JSON as content, so the LLM sees structured failure).
- Instructions string: the spec §5 content, one const, wire conventions included verbatim from contract/README.md's four points (summarized ≤20 lines).

- [ ] **Step 1:** `go get github.com/modelcontextprotocol/go-sdk@latest` → record version → pin exact. Probe the SDK's server+transport API surface (a 20-line spike in the test file is fine); if fundamentally broken → BLOCKED report.
- [ ] **Step 2: Stub-first behavioral RED** — in-process tests: SDK client over the SDK's in-memory/pipe transport ↔ our Server ↔ a FAKE wire (the harness client-test fake-server pattern — httptest+coder/websocket+contract only, reuse the shape from internal/harness/client_test.go): list_tools returns 9 (7 commands present by name from tools.json + 2 read tools registered in Task 2 — for THIS task assert the 7 + that dispatch works); call move_token with valid args → fake wire receives a well-formed ClientCommand with those protojson args and fresh request_id → canned ok result → tool result carries sequence; call with ok=false canned result → isError=true + error string surfaced; call while disconnected → clean MCP error; unknown tool name → SDK-level error. RED against stubs (methods errUnimplemented), capture, implement, GREEN.
- [ ] **Step 3:** arch bite-proof (throwaway: mcp file importing internal/gateway → go-arch-lint fails naming it). `-race`; `task check` green (with mcp added to the serialized test group if socket-heavy — do it now either way for determinism).
- [ ] **Step 4: Commit point** — `feat: mcp server core — nine-seat generic tool dispatch from tools.json`

---

### Task 2: Read tools — get_state and get_events_since

**Files:**
- Create: `internal/mcp/read_tools.go`, `internal/mcp/read_tools_test.go`

**Interfaces:**
- `get_state` {} → the dump contract verbatim: fold all events seen so far (server maintains the envelope history from its live connection — catch-up from after=0 at dial + live accumulation, mutex-guarded) → state JSON + top-level `headSequence`. IMPORTANT consistency rule: state is folded from the server's OWN accumulated history (single source), never a second connection.
- `get_events_since` {afterSequence (int64 as JSON number — MCP tools speak plain JSON; document the divergence from protojson's string convention IN the tool description so the LLM isn't confused), limit (default 50, max 200)} → {events: [protojson envelopes], headSequence, more: bool}.
- Both registered in the same tool table; list_tools now 9.

- [ ] **Step 1: Stub-first behavioral RED** — against the fake wire: seed canned envelopes (session/scene/place/move), get_state returns folded token position + headSequence == last seq; get_events_since pagination: limit walks with `more` flag correct at boundaries (exactly-limit, beyond-end, afterSequence==head → empty+more=false); events are protojson (sequence as STRING inside envelopes — assert, it pins the convention divergence the description documents); retraction in history → get_state reflects the fold-with-retraction (reuse a fold-parity style case). RED, implement, GREEN.
- [ ] **Step 2:** `-race`; `task check`. **Commit point** — `feat: mcp read tools — state and events-since with headSequence`

---

### Task 3: cmd wiring, e2e suite, README

**Files:**
- Create: `cmd/vtt/mcp.go`, `cmd/vtt/mcp_e2e_test.go`
- Modify: `README.md` (Claude Code `.mcp.json` snippet + token note), `Taskfile.yml` (only if Task 1 didn't already move mcp to the serialized group)

**Interfaces:** `vtt mcp --server ws://host/ws [--token T]` (VTT_TOKEN env honored, flag wins; missing both → clear error). RunE ≤30. **tools.json delivery (binding decision):** `internal/mcp` reads ToolsJSON from Config (no filesystem access at runtime); `cmd/vtt` supplies it via `go:embed` of `cmd/vtt/tools.json` — a COMMITTED COPY of `contract/gen/tools/tools.json`, refreshed by one `cp` line added to the Taskfile `generate:contract` target and therefore bound by `check:drift` (divergence fails the gate). File header comment: generated copy, do not edit. (Rationale: go:embed cannot cross package directories, and runtime file reads would break single-binary distribution.)
- [ ] **Step 1: Stub-first behavioral RED via the established runCLI pattern** — flag/env precedence cases; missing-token error. Then the e2e (spec §7 exit test): real composeServer gateway + agent invite; `vtt mcp` served over the SDK's stdio (or in-process transport) to an SDK test client; play `scenarios/smoke.json`'s command list THROUGH THE TOOLS (translate its steps to tool calls in the test — the scenario file is the source of the sequence); assert get_state == harness.Fold of a separate wire observation (equality incl. headSequence); get_events_since pagination walks the full log; spectator-token variant: move_token tool → isError with the authz message, connection intact (a follow-up get_state still works).
- [ ] **Step 2:** Taskfile `generate:contract` gains the tools.json copy line; run it; `check:drift` proves the copy is bound. README: `.mcp.json` snippet (command: vtt, args: [mcp, --server, ws://localhost:8443/ws], env: VTT_TOKEN) + two-line token-secrecy note + the demo runbook (five lines: serve, invite --role agent, paste token to env, open Claude Code, suggested opening prompt).
- [ ] **Step 3:** Injection proof for the e2e's state-equality assertion (after-the-fact over correct code): throwaway — corrupt read-tool fold path (skip one event) → e2e FAILS on state mismatch → restore. `-race`; full `task check` ×2. **Commit point** — `feat: vtt mcp — the agent's seat, end to end`

---

### Task 4: Workflow-level final review + fix wave (Patrik's standing preference) — then the LIVE DEMO gate, then merge.

## After this plan

The demo IS the milestone. Post-merge: sub-project 5 (module loader & rules) gives the seated agent a rulebook; the world layer gives it memory beyond the log.
