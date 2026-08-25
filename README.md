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

## Content directories: rulesets, adventures, maps

Three optional flags on `vtt serve` point at directories of content, each
loaded and validated fully at boot — never at the table:

```sh
vtt serve --campaign campaign.db --addr :8080 \
  --ruleset rulesets/dnd45e-minimal \
  --adventures-dir adventures \
  --maps-dir maps
```

`--maps-dir` serves every subdirectory of `maps/` as a standalone map (one
`map.json` plus an optional `tiles/pack.json` and its images, e.g.
`maps/cellar/`) over `GET /api/maps` and `GET /api/packs/{pack}/{file}` — see
[`docs/map-format.md`](docs/map-format.md) for the format itself, including a
complete worked example and every standard tile name. A map loads
independently of any adventure (design spec
`docs/superpowers/specs/2026-08-12-maps-as-geometry-design.md` §4.3): drop a
directory in, restart, and it is servable. `maps/cellar` is the platform's
own demo map — a small room with real cover (pillars, crates, an interior
wall and a door), generated art included (`tools/genmappack`, see that
package's own doc comment for how to re-run it).

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
stdio: every campaign command in the contract, plus read tools for state,
event history, the ruleset and adventure guides, and the join door. An MCP
host can play at the table as the agent participant, judged by the exact same
authz table any other client is (see
`docs/superpowers/specs/2026-07-24-mcp-gateway-design.md`). `--token` and
the `VTT_TOKEN` environment variable are both honored; `--token` wins if
both are given.

Add this to Claude Code's `.mcp.json` to seat it at a running table:

```json
{
  "mcpServers": {
    "vtt": {
      "command": "vtt",
      "args": ["mcp", "--server", "ws://localhost:8443/ws"]
    }
  }
}
```

Note what is deliberately absent: no `env` block, no token value anywhere
in this file. `vtt mcp` reads `VTT_TOKEN` from its own process environment
(`resolveMCPToken`'s env fallback) — and a subprocess Claude Code launches
inherits whatever environment the shell it was started from had. So
`export VTT_TOKEN=<token>` in the shell you launch `claude` from, *before*
opening it, and `vtt mcp` picks it up with nothing token-shaped ever
written to `.mcp.json` (a file that may well be tracked and shared). The
token grants everything the `agent` role is authorized for at this table —
treat it as a credential.

Demo runbook:

1. `vtt serve --campaign campaign.db --addr :8443`
2. `vtt invite --campaign campaign.db --name "Claude" --role agent` — it prints the token once.
3. In that same shell, capture it without it ever landing in shell history:
   `read -s VTT_TOKEN && export VTT_TOKEN` (prompts silently, nothing echoed,
   nothing to scroll back through — or set `HISTIGNORE='export VTT_TOKEN=*'`
   first if you'd rather type it directly).
4. Open Claude Code from that SAME shell (so the subprocess inherits
   `VTT_TOKEN`) with this repo's `.mcp.json` in scope.
5. Suggested opening prompt: "Check get_state, then start a session, create a scene, and place a token on it."

## Security note: invite tokens and the connection URL

Invite tokens travel in the WebSocket URL's `token` query parameter. The
server itself never logs this URL. However, any fronting reverse proxy or
HTTP access log sitting in front of the server *will* capture the full URL,
tokens included — do not front the server with URL-logging infrastructure
until TLS and/or header-based auth lands. In the meantime, tokens are
revocable at any time via `vtt revoke`.
