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

A scenario may also carry a `projections/` subdirectory — one seat per
directory inside it. See **Projected seats** below.

## What no fixture here may contain

`internal/harness.TestTheCorpusNeverConfersControlAtCreationNorGrantsInSilence`
walks every `.json` under `scenarios/` — definitions, streams, projections, all
of it — and holds each actor creation and each grant to visibility spec §5.1:

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
adding it to a list. **There is no exemption list**, on the same reasoning as
the projected-fixture gate below: a list of fixtures the rule does not apply to
is the artifact that goes stale silently. The one apparent exception is derived
instead — `scenarios/denials.json` sends both forbidden shapes on purpose and
asserts the server refuses them, and a step that pins a REFUSAL is defending the
rule rather than breaking it, because a refused command never becomes history.

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

## Projected seats

`<scenario>/projections/<seat>/` is what ONE viewer receives of that scenario's
log — the right-hand side of the wire rather than the log itself. Three files:

| File | What it is | Who derives it | Gate |
|---|---|---|---|
| `viewer.json` | Which seat: participant, role, and a spectator's perch | Declared. **A perch never reaches the log** (it is connection state, `internal/gateway/seat.go`), so it cannot be derived from the stream beside it | — |
| `stream.json` | What `gateway.Projector` emits for that seat | **Derived**, and pinned by `internal/gateway.TestTheProjectedGoldensAreWhatTheProjectionActuallySends`, which recomputes it and compares bytes | that test |
| `state.json` | Folded state + `headSequence` | **A human, by hand**, same rule as every other `state.json` here | Go fold (same test) + `client/test/projection-parity.test.ts` |

**`stream.json` here is NOT an independent recording, and that difference
matters.** The log-level `stream.json` one directory up is testimony from a real
server run; this one is a derivation of it. It is committed for one reason:
**TypeScript has no projector and is never going to have one** — spec §6.2's
"server decides what you may see; the client draws it" — so the only way
`client/src/fold.ts` can be held to the same bytes the Go fold sees is for those
bytes to be on disk. The independence that makes agreement evidence lives in
`state.json`, which was hand-derived and which THREE things are held to: the Go
fold, the TypeScript fold, and (for everything but `Explored`) the independent
sight oracle in `internal/gateway/keystone_test.go`.

### Why the corpus needed them

Until `session-zero` landed, `grep -ric sceneseen scenarios/` returned zero.
Every scene in the corpus was untiled or entirely floor, with no wall, no closed
door and no `blocks_sight` object anywhere — so nothing was ever hidden from
anyone, and the keystone (visibility spec §4.3) could not tell a correct
projection from one that forwards the whole log to everybody.
`internal/gateway.TestTheKeystoneCorpusCanTellAProjectionFromAPassthrough` is
that guard, kept as a test so the corpus cannot quietly regress to it.

MEASURED, by deleting `session-zero` and re-running **six** fault injections
against the keystone: **three then pass unnoticed, and all three are OVER-SEND**
— a projection that shows every creature in any scene the viewer can see part of
(session zero itself), one that ignores sight blockers entirely, and one that
ships a scene's whole tile map. All three under-send and roster faults survived
the deletion. Every fault the old corpus missed was an over-send, and over-send
is the direction this arc exists to close.

### What `session-zero` deliberately does NOT do

**It never retracted an event that CAUSED an introduction**, and that was an
exclusion rather than an oversight — the two look identical in a diff a year
from now, so it was written down here rather than only in a task report. It is
kept as a CLOSED record: sub-project 13 removed retraction from the platform
outright (spec `2026-08-30-retraction-leaves`), so there is no longer a command,
a marker, or a scenario step that could reach the shape below.

**What the hazard was.** Synthesized envelopes carry the sequence of the event
that caused them (visibility spec §4.2), so retracting the event that first
revealed a scene deleted the viewer's own `SceneCreated` for it. `transitions`
then still emitted an empty `SceneSeen` for that scene at the retraction's
sequence — its union walk keeps a scene in play for one more step — and the
recipient's fold rejected it with `scene seen for unknown scene`. Both folds are
strict and `client/src/session.ts` re-folds its whole log on every event, so that
was a permanent freeze rather than one bad frame. Putting it into a scenario
would have left the gate red on a design decision that belonged to whoever made
it, not to a corpus entry.

The corpus's own three retraction steps went with the platform's: two in
`denials` that were authorization probes, refused before they reached a log, and
`three-role-exit`'s, which landed. That one retracted an ordinary MOVE and folded
cleanly, which is exactly what made the point above legible — retraction itself
was never the hazard here, only a retraction landing on an event that CAUSED an
introduction was.

**Two things this section used to cite are gone**, said plainly so that the next
reader does not go hunting for them. It quoted a prediction from `transitions`'
doc comment ("spec §4.3's keystone is where it is catchable"), which was
rewritten when retraction left the gateway on 2026-08-31; and it warned readers
off the forgetting loop further down that same function, which shared the `scene
seen for unknown scene` string while answering a different case. That loop was
deleted the same day, because nothing removes a scene from the world — spec
`2026-08-30-retraction-leaves` §5.3: there is no `delete_scene`.

### Deriving a projected golden

`Explored` is unioned from each `sceneSeen`'s **`tiles` keys**, never from its
`visible` list.

- `ambush` declares terrain on all 180 of its squares, so its `Explored` grows
  to exactly the 36 squares the player can see.
- `camp` declares all 9 of its own, so its `Explored` is all 9 — the whole room
  is in sight from where the healer stands.

**THESE TWO NO LONGER DISCRIMINATE THE MECHANISM THEY ARE CITED FOR.** In all
three of the corpus's `sceneSeen` the tile keys, the visible list and the
explored set are now the same set of squares, key for key — so a fold that built
`Explored` from `visible` instead of from `tiles` would reproduce every one of
these fixtures exactly. They still pin what the projection SENDS; they no longer
witness where `Explored` comes from. That property is pinned by constructed
tests instead, named at the end of this section.

**The bare-canvas half of that contrast is gone from this corpus, and it left
on purpose.** `camp` used to declare NO terrain, and it sat beside `ambush` so
that the "`Explored` comes from tiles, not from `visible`" rule was a fixture
rather than a sentence: its `Explored` stayed EMPTY however much of it was
visible. On 2026-09-01 `create_scene` began refusing a scene that leaves a
square undeclared (spec `2026-08-30-retraction-leaves` §6 — *a wall nobody
declared is an invisible barrier*), and every scene in this corpus that a
scenario CREATES is created by `create_scene`, so no fixture here can be a bare
canvas any more. (Corrected 2026-09-01: this said *every* scene.
`scenarios/adventure-night.json` issues only `loadAdventure`, and its scene
comes from a map file — the very exemption the next paragraph names. That
exemption is why the sentence needs the qualifier and not why it fails: an
adventure's map file is authored, not typed into a form, and nothing in this
corpus reaches the bare-canvas shape either way.)

The rule did not change and the shape is still reachable — a map FILE may still
omit tiles, which is the exemption that keeps files authored before
maps-as-geometry loading, and spec §6.2's "a token is a FREE OBJECT and needs no
terrain to exist" still holds. What changed is where the rule is pinned: it is
CONSTRUCTED rather than exhibited, in three tests, each of which builds a
`sceneSeen` that lists visible squares and carries no tiles and asserts
`Explored` stays empty:

- `internal/engine`'s `TestVisibleComesFromItsOwnFieldNotFromTheTiles`
- `client/test/visibility.test.ts`'s "a token on a bare canvas is drawn" —
  **pre-existing**, and it means the fixture was never the only cover
- `client/test/fold-unit.test.ts`'s "a sceneSeen with visible squares and no
  terrain remembers nothing" — added 2026-09-01 as the assertion-for-assertion
  mirror of the Go one, because the other TS case is a DRAWING test that
  happens to assert the property on its way elsewhere.

MEASURED, after an earlier draft of this paragraph claimed the wrong number:
injecting `Explored` follows `Visible` into `client/src/fold.ts`'s `sceneSeen`
arm reds **two** tests across `client/test`, the last two above.

`session-zero`'s sight is a rule rather than a table, which is what makes 36
squares hand-derivable: a solid wall fills column `x=3` for the full height of
the grid, so from anywhere west of it every square with `x <= 2` is reachable, a
square OF the wall is reachable (`sight.Clear` exempts a blocker containing the
target — "without this you cannot see the wall you are standing against"), and
every square with `x >= 4` is behind the full-height slab. The Goblin Archer
stands at **(19,8)** — session zero's own square — and at sequence 11, where
it is placed, neither seat's stream carries one byte about it. It breaks cover
at sequence 13 and returns at 14, which is what puts a `tokenHidden` and a
never-forgotten roster
entry in the fixture.

## Coverage

Eight scenarios, covering a subset of the contract's command types, **not
all of them.** "All fifteen" stood here until 2026-08-25: it was true when
fifteen WAS the whole contract, and stayed on the page as the contract grew
past it, turning a corpus statistic into a false completeness claim. Door,
map, viewpoint, join-link and participant-promotion commands are among those
with no golden today — examples, not the complete list. `retract_events` was
listed here beside them for the days between sub-project 13 deleting the
corpus's last retraction step and the same sub-project deleting the command
itself: it is not a gap now, because it is not a command. Derive the current
gap by diffing the scenarios' command keys against
`contract/gen/tools/tools.json` rather than trusting a list written here,
which is how this sentence went wrong in the first place.

`session-zero` was added 2026-08-22 with the visibility arc's keystone. It is
the only scenario in the corpus with a sight blocker, the only one with two
scenes, and the only one whose player controls two actors standing in different
ones — which is what makes its spectator seat (perched on the actor in `camp`)
see a strictly smaller world than its player seat rather than the same one
twice.

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

`adventure-night` and `toy-brawl` roll dice, and are here because their event
streams are shape-STABLE: **the same events in the same order every run, with
only the roll values differing.** That invariant is the claim; a line count is
not, and one used to stand here as if it were ("208 = 208 and 178 = 178",
measured across repeated captures before 2026-08-25). It was falsified by an
edit that had nothing to do with dice — `toy-brawl/stream.json` went 202 to 279
lines on 2026-09-01 when its scene declared its terrain. Compare a fresh capture
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
32x32 to **31x3** so it could declare every square, which `create_scene` now
requires, and its `sceneCreated` grew 93 tiles. Nothing about the shape
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

