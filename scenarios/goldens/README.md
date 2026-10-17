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

## Where a scene comes from

Nowhere in this corpus does a scenario CREATE a scene. Since 2026-09-02 every
scene arrives from a file: almost every scenario gets theirs from a map
under `scenarios/maps/`, loaded by a `load_map` step, and `adventure-night`
gets its one from the adventure it loads. (Most of those have a golden
here; `goblin-fight` is the one that does not, for the reason at the bottom of
this file.) `create_scene` LEFT the platform on 2026-09-02
(`docs/superpowers/specs/2026-09-01-create-scene-leaves-design.md` — the kernel
serves maps, it does not make them), and this corpus was converted first so that
it could: the sentence above used to say "is leaving", and the conversion is
what made the removal possible rather than the other way round.

`scenarios/maps/<id>.json` is one standalone map in the `mapdef` format, and the
FILENAME IS THE ID — which is also the scene id every later step in the scenario
references, so the three cannot drift apart. The runner installs the directory a
scenario names (its `maps` key) into that scenario's throwaway campaign before
the server boots (`cmd/vtt`'s `installMaps`), because a map belongs to the
campaign that uses it and there is no server-wide maps flag to point at one.

**Two consequences worth knowing before reading a `sceneCreated` here.** A map
declares its terrain by standard tile NAME (`stone`, `wood`, `earth`,
`stone-wall`), and a name carries a `material` as well as a `kind` — so every
tile in these fixtures has one, where the `create_scene` command that used to
build them could only ever say `kind`. And every map in the corpus is
pack-free: `material` is `mapdef`'s, `art` is empty everywhere, and no scenario
needs custom art to prove anything.

`cmd/vtt.TestEveryMapTheCorpusNamesIsInstalledAndUsed` holds that directory to
the library in both directions — every id a `load_map` names has a file, and
every file is named. The first half is not decoration: `Authorize` runs before
any map lookup, so `denials.json`'s two REFUSED `load_map` steps pass with
"not authorized" whether or not the map they name exists. Measured on
2026-09-02: misspell that id and the scenario stays green; only that gate goes
red.

## What no fixture here may contain

`internal/harness.TestTheCorpusNeverConfersControlAtCreationNorGrantsInSilence`
walks every `.json` under `scenarios/` — definitions, streams, states and map
files, all of it — and holds each actor creation and each grant to visibility
spec §5.1:

- **an actor is never created holding a controller.** Creation makes a
  character; control is conferred once, by a grant.
- **every created actor states a kind**, because an unstated one cannot be told
  from a deliberate one.
- **every grant states a kind.** The COMMAND is refused on the wire, so a
  fixture must not encode what the wire rejects. A kindless `actorControlGranted`
  EVENT is a different argument — `engine.Apply` still accepts those on purpose
  — but nothing can produce one any more, so one in a committed stream is a
  hand-edit or a regression.

It matches by KEY wherever the key appears, in both wire spellings, so a new
file kind or a nesting level nobody has invented yet is covered without anyone
adding it to a list. **There is no exemption list**: a list of fixtures the
rule does not apply to is the artifact that goes stale silently. The one
apparent exception is derived instead — `scenarios/denials.json` sends both
forbidden shapes on purpose and asserts the server refuses them, and a step
that pins a REFUSAL is defending the rule rather than breaking it, because a
refused command never becomes history.

**A step must pin THAT shape's refusal, not merely some refusal.** `Authorize`
runs before the shape validators, so a player's `add_actor` is turned away as
unauthorized whatever else is wrong with it — and a gate that asked only "is
this step denied?" would let those steps carry a seeded controller into the
committed corpus, green, as the template the next author copies. The step's
`deniedContaining` therefore has to be part of the refusal that shape actually
earns — and has to name **exactly one** refusal, because every one of them
contains the word `actor` and so does the authz denial. A claim vague enough to
be any of them has identified none, and pins nothing. Nobody has to lie to slip
through a match-any rule; they only have to be lazy.

The corpus was the last place the old model could survive: no campaign is in
use by anyone, so these fixtures are the entire "existing history" this arc
deleted its migration rule for.

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

That last row is wider than the plan's contract, and it has to be: a
server-assigned participant id appears INSIDE payloads, not only on the
envelope. Every `ActorControlGranted` and `ActorControlRevoked` carries one,
and with only the envelope-level field normalized the stream differed on every
capture, so the drift gate could never have gone green.

(It used to say `Actor.controller_id`, naming `three-role-exit` and
`story-table` as the two scenarios that set it. That is no longer possible:
since 2026-08-24 an `ActorAdded` may not name a controller at all — creation
makes a character and a grant hands it over — so the id now travels on the
grant instead. The rule is unchanged and the reason for it is unchanged; only
the payload it lives in has moved.)

## Coverage

The goldens cover a subset of the contract's command types, **not all of
them.** "All fifteen" stood here until 2026-08-25: it was true when
fifteen WAS the whole contract, and stayed on the page as the contract grew
past it, turning a corpus statistic into a false completeness claim. Viewpoint,
join-link and participant-promotion commands are among those with no golden
today — examples, not the complete list. (`door` left that list on 2026-09-16
when `door-watch` arrived, whose map `door-hall` is the only one carrying door squares at all and
therefore its second sight blocker after `ambush`'s wall.) (`map` stood in that
list until 2026-09-02. `load_map` is in almost every golden, because
every scene a scenario puts in the world arrives that way; `adventure-night` is
the exception and gets its scene through `load_adventure` instead.) `retract_events` was
listed here beside them for the days between sub-project 13 deleting the
corpus's last retraction step and the same sub-project deleting the command
itself: it is not a gap now, because it is not a command. Derive the current
gap by diffing the scenarios' command keys against
`contract/gen/tools/tools.json` rather than trusting a list written here,
which is how this sentence went wrong in the first place.

`session-zero` is the only scenario in the corpus with a WALL, the only one
with two scenes, and the only one whose player controls two actors standing in
different ones.

`shared-control` was added 2026-08-07 with `grant_actor_control` and
`revoke_actor_control`, and it exists because the corpus was SILENT on both:
this gate is the branch's own stated defence against a Go/TS fold divergence,
and a divergence on the grant arm had already shipped and been caught elsewhere.
Injecting the mirror-drop into either fold's grant OR revoke arm produced ZERO
failures here before it, and 1 (TS parity) / 2 (Go corpus) after.

Getting it to bite took a second pass worth recording. A grant only moves
`controller_id` when the set was EMPTY — the mirror is `controller_ids[0]` and
a grant APPENDS, so on an already-owned actor the mirror does not move and
dropping it is invisible. The scenario therefore grants onto an unowned actor
(`act-herald`, and since 2026-08-24 `act-warden`'s and `act-scout`'s FIRST
grants too, because an actor is now born unowned without exception) as well as
onto a shared one, and revokes the set's HEAD so the mirror has to slide. Idempotent re-grant, revoking a non-controller, and
revoking the last controller back to unowned are all in the same stream.

`adventure-night`, `toy-brawl` and `door-watch` roll dice, and are here because their event
streams are shape-STABLE: **the same events in the same order every run, with
only the roll values differing.** That invariant is the claim; a line count is
not, and one used to stand here as if it were ("208 = 208 and 178 = 178",
measured across repeated captures before 2026-08-25). It was falsified by an
edit that had nothing to do with dice — `toy-brawl/stream.json` went 202 to 279
lines on 2026-09-01 when its scene declared its terrain, and 279 to 304 on
2026-09-02 when that terrain arrived from a map file and every tile gained the
`material` its tile name carries. Compare a fresh capture
against the committed file, which is what the drift gate does; do not compare
either against a number written here. The drift gate masks dice-decided fields — `results`,
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
507 lines**, measured on the 2026-08-25 corpus — because a miss emits fewer
events than a hit, and no masking of values can make two different event
sequences comparable. (Those two figures are a dated observation and no longer
the numbers a fresh run prints: on 2026-09-01 the scenario's grid went from
32x32 to **31x3** so it could declare every square, which `create_scene` then
required, and its `sceneCreated` grew 93 tiles. The grid is unchanged since; the
declaration moved. As of 2026-09-02 it lives in
`scenarios/maps/scn-goblin-fight.json` and the scenario loads it. Nothing about the shape
instability changed — re-measure rather than trusting the pair.) Including
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

