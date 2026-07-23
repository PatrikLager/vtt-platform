# ADR-001: API-first, headless core

**Status:** Accepted (founding decision, 2026-07-23)
**Context:** Foundry VTT's automation lives inside its browser client; there is no
sanctioned remote API, which makes an LLM participant a hack. Our defining feature
is an LLM as a first-class participant up to full DM.
**Decision:** Every game action is a structured, permission-checked API operation
with a subscribeable event stream. The human UI is just another API client. The
LLM gateway is derived from the API, never a side channel.
**Consequences:** The API surface must be complete before any UI exists; the
simulation harness (ADR-006) is the first consumer.
