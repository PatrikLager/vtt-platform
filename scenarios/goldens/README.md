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

Six scenarios, covering **all thirteen** command types.

`adventure-night` and `toy-brawl` roll dice, and are here because their event
streams are shape-STABLE: the same events in the same order every run, with
only the roll values differing (measured across repeated captures at 208 = 208
and 178 = 178 lines). The drift gate masks dice-decided fields — `results`,
`total`, `outcomeSummary`, `delta`, `newValue`, `outcome` — on BOTH sides of
its comparison, so everything else is still checked: event order, sequences,
which events are emitted, and every non-dice field. The committed streams keep
their REAL dice, because the fold gate needs them to reproduce the
hand-derived state.

That masking was verified not to have neutered the gate: changing a non-dice
field fails two scenarios, and suppressing `conditionApplied` fails the one
that emits it.

### goblin-fight is deliberately absent

Not for want of trying. Its stream differs in SHAPE between runs — **519 vs
507 lines**, measured — because a miss emits fewer events than a hit, and no
masking of values can make two different event sequences comparable. Including
it would mean either a permanently-red drift gate or an exemption that hides
real drift.

It costs nothing in coverage: every command type it uses is already covered by
`toy-brawl`. Adding it needs a seedable roller at the gateway, which
contradicts `WithRuleset`'s documented "never separately configurable at this
layer" and is therefore its own decision, not a test convenience.

## Deriving a dice scenario's golden

The roll values are taken as TESTIMONY — they come from the recorded stream,
exactly as server-assigned event ids and sequences already do. Everything else
is derived independently, from the scenario definition and (for
`adventure-night`) the adventure's own source files. The derivation answers
"given these rolls, what state must result", which is a human act the machine
does not do for you.

