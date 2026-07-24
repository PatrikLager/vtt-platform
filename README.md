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

## Security note: invite tokens and the connection URL

Invite tokens travel in the WebSocket URL's `token` query parameter. The
server itself never logs this URL. However, any fronting reverse proxy or
HTTP access log sitting in front of the server *will* capture the full URL,
tokens included — do not front the server with URL-logging infrastructure
until TLS and/or header-based auth lands. In the meantime, tokens are
revocable at any time via `vtt revoke`.
