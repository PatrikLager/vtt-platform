# The shared golden corpus

One directory per scenario, two files each, **derived independently of each
other**. That independence is the point — see below.

| File | What it is | Who derives it | Gate |
|---|---|---|---|
| `state.json` | Folded final state + `headSequence`, in `vtt state dump` shape | **A human, by hand**, from the scenario definition | `internal/harness.TestFoldGoldenCorpus` |
| `stream.json` | The server's normalized event stream | Recorded from a real run | `cmd/vtt.TestScenarioGoldenStreamsHaveNotDrifted` |

The fold gate asserts `Fold(stream.json) == state.json`. Because neither file
was produced from the other, agreement is evidence rather than a tautology: if
the recording were wrong, folding it would not reproduce a state derived
without looking at it.

## Why there is no `-update` flag

The original plan generated this corpus behind one. That was rejected against
the rule already shipped in `internal/adventure/conformance/conformance.go`:

> derive a golden by hand FIRST (ADR-009), then load the real adventure,
> Compile it, run this over the result, and use it only to VERIFY the
> hand-derivation, **never to generate a golden no human derived first**.

A regenerate-on-demand switch is how a golden quietly stops being a claim
anyone checked. When a gate here fails, read the diff and decide: did the
server change, or is the corpus legitimately stale? If the latter, re-derive
`state.json` by hand before re-recording `stream.json`.

## Normalization

Four things vary per run and are normalized before anything is committed:

| Field | Replacement |
|---|---|
| `eventId` | `evt-<sequence>` |
| `occurredAt` | omitted |
| `sessionId` | `sess-N`, N in order of first appearance |
| participant ids | `p-<name>` — **everywhere they appear**, not just `participantId` |

That last row is wider than the plan's contract, and it has to be:
`Actor.controller_id` carries a server-assigned participant id INSIDE the
payload. `three-role-exit` and `story-table` both set it, and with only the
envelope-level field normalized the stream differed on every capture, so the
drift gate could never have gone green.

## Coverage

Four scenarios, covering 10 of the 13 command types: sessions, scenes, actors,
tokens, narration, notes, and retraction.

`adventure-night`, `toy-brawl` and `goblin-fight` are **absent on purpose**.
They roll dice, and `WithRuleset` hardcodes an unseedable `CryptoRoller`
(`internal/rules/crypto_roller.go`) whose draws no human can derive in
advance. Adding them requires a roller seam — a production change that
contradicts `WithRuleset`'s "never separately configurable at this layer"
decision — and that is its own reviewed decision, not a test convenience to
be sneaked in here. Until then this corpus omits `useAbility`,
`loadAdventure` and `removeCondition`.

## What each scenario is for

- **smoke** — the minimum: one of each core event, one session opened and closed.
- **denials** — 18 steps, only 4 accepted. Proves rejected commands emit
  *nothing*: head stays 4 and the session stays open despite two `endSession`
  attempts.
- **three-role-exit** — carries the **retraction** case. Seq 9 retracts seq 8,
  so `tok-ursus` must stay at its placed `(0,0)`, not the `(5,5)` it was moved
  to. Also the only reconnect coverage.
- **story-table** — notes and narration. Four narrations leave *no* trace in
  state by design; `kobold-den` is upserted twice so last-write-wins is
  visible; `old-rumor` is created then deleted and must be absent rather than
  empty.
