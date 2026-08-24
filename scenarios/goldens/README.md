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

**It never retracts an event that CAUSED an introduction**, and that is an
exclusion rather than an oversight — the two look identical in a diff a year
from now, so it is written down here rather than only in a task report.

Synthesized envelopes carry the sequence of the event that caused them
(visibility spec §4.2), so retracting the event that first revealed a scene
deletes the viewer's own `SceneCreated` for it. `transitions` then still emits an
empty `SceneSeen` for that scene at the retraction's sequence — its union walk
keeps a scene in play for one more step — and the recipient's fold rejects it
with `scene seen for unknown scene`. Both folds are strict and
`client/src/session.ts` re-folds its whole log on every event, so that is a
permanent freeze rather than one bad frame.

The defect is pre-existing, and `internal/gateway/project.go`'s `transitions` doc
comment is where it is recorded — as the DANGLING-REFERENCE form, a later
forwarded event about a retracted introduction failing with `moved unknown
token`. The prediction "spec §4.3's keystone is where it is catchable" sits
there, verbatim, and is correct.

**Do not read the forgetting-loop comment further down that function as the same
thing.** It contains the string `scene seen for unknown scene` too, which makes it
look like the nearer match, and it is not: its case is an undo covering the
`SceneCreated` ITSELF, its cause is `pr.scenes`/`pr.seen` outliving `st.Scenes`
rather than an introduction stamped at a retracted sequence, and it says
"Measured before this loop existed" — the loop directly beneath it **fixed** that
case. Shared error string, different defect, already closed.

Putting the shape above into this scenario would leave the gate red, and the fix
is a design decision (a per-viewer pre-flight in `campaign.Undo`, or a different
sequence for a synthesized introduction) that belongs to whoever makes it, not to
a corpus entry. `three-role-exit` DOES retract, and the keystone folds it cleanly,
because what it retracts is a MOVE.

### Deriving a projected golden

`Explored` is unioned from each `sceneSeen`'s **`tiles` keys**, never from its
`visible` list. `session-zero` carries one scene of each kind side by side so
that the difference is a fixture rather than a sentence:

- `ambush` declares terrain on all 180 of its squares, so its `Explored` grows
  to exactly the 36 squares the player can see.
- `camp` declares none. It is a bare canvas, which is a legal scene and not a
  degenerate one (spec §6.2: "a token is a FREE OBJECT and needs no terrain to
  exist"), and its `Explored` therefore stays EMPTY however much of it is
  visible — there is no terrain there to remember.

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

Eight scenarios, covering **all fifteen** command types.

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

