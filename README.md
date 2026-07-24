# vtt-platform

An LLM-native VTT (virtual tabletop) platform: an event-sourced, API-first
Go core that an LLM agent (and/or human clients) drive over a single
WebSocket/HTTP gateway, rather than a traditional GUI-first VTT with an API
bolted on. See the design specs in [`docs/superpowers/specs/`](docs/superpowers/specs/)
and the architecture decisions in [`docs/adr/`](docs/adr/) for the full
rationale.

## Running

The `vtt` CLI (`cmd/vtt`) opens one campaign SQLite file per invocation:

```sh
# Mint an invite token for a new participant (DM-side, CLI-only).
vtt invite --campaign campaign.db --name "Alice" --role player

# Serve that campaign over the WebSocket/HTTP gateway.
vtt serve --campaign campaign.db --addr :8080

# Revoke a participant's token if it leaks or is no longer needed.
vtt revoke --campaign campaign.db --id <participant-id>
```

Clients connect to `ws://<addr>/ws?token=<token>&after=<sequence>`.

## Simulation harness: scenarios and soak

`vtt client run <scenario.json>` drives a declarative scenario (see
`internal/harness/scenario.go` for the format) self-contained by default
(boots its own throwaway server) or against a live `vtt serve` with
`--server <ws-url> --tokens <tokens.json>`. `vtt client soak --seed <n>
--events <n>` runs the wire-level soak generator the same two ways. Every
scenario assumes a FRESH campaign (absolute sequence numbers from zero) —
running one against a campaign with prior history fails loudly rather than
producing confusing assertion mismatches.

`tokens.json` (`{"participants": {"<name>": "<token>"}}`) holds plaintext
bearer credentials for live-mode runs — `chmod 600` it and never commit it
(see `.gitignore`).

## Claude Code: seating an LLM as the agent participant

`vtt mcp --server <ws-url> [--token <token>]` serves an MCP server over
stdio: nine tools (seven generic command tools plus `get_state`/
`get_events_since`) that let an MCP host play at the table as the agent
participant, judged by the exact same authz table any other client is (see
`docs/superpowers/specs/2026-07-24-mcp-gateway-design.md`). `--token` and
the `VTT_TOKEN` environment variable are both honored; `--token` wins if
both are given.

Add this to Claude Code's `.mcp.json` to seat it at a running table:

```json
{
  "mcpServers": {
    "vtt": {
      "command": "vtt",
      "args": ["mcp", "--server", "ws://localhost:8443/ws"],
      "env": { "VTT_TOKEN": "<agent-invite-token>" }
    }
  }
}
```

The token grants everything the `agent` role is authorized for at this
table — treat it as a credential, never commit it or paste it into a
tracked file. The env form above (rather than a literal `--token` in
`args`) keeps it out of `.mcp.json` and shell history alike.

Demo runbook:

1. `vtt serve --campaign campaign.db --addr :8443`
2. `vtt invite --campaign campaign.db --name "Claude" --role agent` — copy the printed token.
3. `export VTT_TOKEN=<the printed token>` (or set it in `.mcp.json`'s `env`, as above).
4. Open Claude Code with this repo's `.mcp.json` in scope.
5. Suggested opening prompt: "Check get_state, then start a session, create a scene, and place a token on it."

## Security note: invite tokens and the connection URL

Invite tokens travel in the WebSocket URL's `token` query parameter. The
server itself never logs this URL. However, any fronting reverse proxy or
HTTP access log sitting in front of the server *will* capture the full URL,
tokens included — do not front the server with URL-logging infrastructure
until TLS and/or header-based auth lands. In the meantime, tokens are
revocable at any time via `vtt revoke`.
