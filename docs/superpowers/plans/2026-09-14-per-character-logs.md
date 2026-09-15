# Per-Character Logs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every viewpoint-capable character its own authoritative log,
written at append time and never regenerated, so a player's connection carries
only what that character perceived — structural isolation in place of a
read-time filter.

**Architecture:** The decision about what a character perceives already exists,
in `internal/gateway/project.go`. This arc MOVES it from read time to write
time: `campaign.Append` fans one world event out to the world log and to every
perceiving character's log in a single transaction, and a seat then forwards its
character's log verbatim. `project.go`, the seat's projector, `pastResume` and
the two-sided keystone are deleted.

**Tech Stack:** Go 1.23 + SQLite (`internal/store`), protobuf `vtt.v1`,
TypeScript client (`client/src/fold.ts`).

**Spec:** `docs/superpowers/specs/2026-08-30-per-character-logs-design.md`

---

## How to read this plan, and why it is shaped this way

**THIS PLAN DOES NOT RESTATE THE PERCEPTION RULES. That is deliberate, and it is
the most important thing on this page.**

Two earlier drafts tried. The first derived the rules from
`contract/vtt/v1/events.proto` and produced a table that was wrong in almost
every row — including one that would have written every tile of the map into a
player's log, permanently, since §3.3 refuses regeneration. The second read
`project.go`'s arms and restated them, and was wrong again in twelve places:
six arms do not read world state at all but the projector's own memory; the
scene introduction is a REDACTED `SceneCreated` that the draft omitted entirely,
so every character log would have failed to fold at its first entry; the door
transition walk was dropped; controller grants and condition replay were
missing. Twenty-four defects across two attempts, every one of them
manufactured by the act of restating.

`project.go` is 1245 lines and nearly every rule in it carries a written reason
and a scar. Prose cannot restate it faithfully. So this plan does two things
instead:

1. **Task 1 captures what it does as DATA**, before anything is built and while
   the code still exists — a committed oracle.
2. **Every later task PORTS code and proves the port against that oracle.** The
   arms are read, not paraphrased. Where this plan names a rule at all, it names
   it to tell you where to look.

If you find yourself writing a rule into code from this document's prose rather
than from `project.go`, stop: this plan is wrong and the source is right.

---

## Global Constraints

- **Airtight TDD (ADR-009).** Tests first, RED before the solution exists;
  behavioural RED over compile-failure RED wherever a stub can compile. An
  after-the-fact test needs fault-injection proof per load-bearing assertion.
- **`task check` is the single quality gateway.** Never weaken a gate to pass it.
- **Contract evolution is additive only** (ADR-007). This arc needs NO new
  contract message: every entry a character log holds is an existing past-tense
  event.
- **One fold.** `engine.Apply` is the only code that changes game state. This arc
  runs that one fold over more than one log; it does not add a second.
  `campaign.FoldPrefix` is the helper (`engine.Fold` does not exist).
- **No game-system vocabulary in platform code** (semgrep enforces).
- **Review before commit**, per task, on that task's own diff.
- **Specs are truth.** Deviations are adjudicated and the spec amended at the
  merge gate — Task 16.
- **Citations name durable things** (rule 8): a function, test, constant, commit
  hash or dated decision. Never a bare `file.go:123`, never a `.superpowers/`
  path, never "this task".
- **Fail closed.** Where the answer is unclear, write nothing. A character who
  missed a sighting can be told by the DM; one who was shown a secret cannot be
  un-shown.
- **No character log is ever regenerated.** No rebuild path, not even for tests.
  Tests build history by appending, as a session does.
- **No dual-path period.** Patrik's ruling (2026-09-04, repeated 09-06): nobody
  uses the product, so nothing stands on the old path and nobody is arriving at
  the new one. No feature flag, no phased removal, no migration for existing
  campaign databases.

---

## Rule 9: how MapTool solves this

**MapTool has a real analogue.** `Zone.exposedAreaMeta` is a
`Map<GUID, ExposedAreaMetaData>`: per-token exposure keyed by the token's
`exposedAreaGUID`, where `ExposedAreaMetaData` holds one field,
`exposedAreaHistory`, an `Area` accumulated as that token moves. Memory per pair
of eyes, written when the seeing happened, never recomputed — structurally what
a character log is, with twenty years of production behind it.

**Borrow:** the keying per viewing entity rather than per player account, so one
player holding three characters has three memories; the accumulate-at-sighting
rule; and the memory as a stored first-class thing rather than a derived view.

**Reject, and this is the whole reason for the sub-project:** it lives on the
`Zone`, which every client receives in full — `Zone.toDto()` ships every token's
exposure history to every client, and the client renders the subset it owns. The
memory is per-token; the distribution is not filtered at all. That is the model
this platform's premise rejects, and it is what `internal/gateway/seat.go` means
by "IT IS NOT THE SECURITY BOUNDARY".

**Where MapTool does not help:** its exposure is accumulated geometry, answering
"what squares has this token seen". It has no per-player event history at all —
chat is broadcast with a whisper flag and dropped client-side. The character
*log* has no precedent there, so §3 is ours to get right unaided. Recorded so
the next reader does not re-ask.

---

## Decisions this plan makes

**D1. The oracle is a spectator perch, and needs no new production code.**
`Projector.eyes` has exactly two role arms. For `RoleSpectator` it returns
`[]string{pr.viewer.Viewpoint}` — ONE actor's eyes — and refuses a non-party
member. `Project` consults `viewer.Role` in only one other place, where
`RolePlayer` and `RoleSpectator` are the same arm. So a `Projector` built as

```go
gateway.NewProjector(gateway.Viewer{
	ParticipantID: "oracle",
	Role:          identity.RoleSpectator,
	Viewpoint:     actorID,
})
```

produces exactly the stream one character should receive. The oracle is today's
code, asked the question this arc is about.

**D2. `perceive` is its own component.** `.go-arch-lint.yml` allows
`campaign -> contract, store, engine, campaign, eventgen` and NOT `-> sight`, so
§4.2 as written fails `check:arch`. A new component `perceive`
(`-> contract, engine, sight, perceive`) gives the decision a boundary and a test
file; `campaign` gains one edge rather than a game-visibility concept. This is a
gate change and therefore its own reviewed decision — Task 3, alone.

**D3. The verdict stays THREE-VALUED.** `project.go` distinguishes
`unrecognised` (the zero value — a payload no arm names), `withheld` and
`forwarded`, and `Project` returns before synthesizing transitions when the
verdict is `unrecognised`: "The projection cannot tell what an unknown payload
did to the world, so it does not narrate the aftermath." A boolean cannot carry
that, and collapsing it would narrate the aftermath of an unknown event into
every character's log — permanently, under §3.3.

**D4. Perception is judged against the state AFTER the event.** `project.go`:
"st is the state AFTER env was folded". It is load-bearing: the destination
check, the placement of a token that just arrived, and the withdrawal of one
that just left all read post-event state. `campaign.Append` already folds into a
validation snapshot that is exactly this and currently discards it.

**D5. The memory is the fold of the character's own log, and nothing else.**
Spec §4.3, and Patrik's framing on 2026-09-15: one log per character, replay it,
the log is the record. No side table, no `known{}` struct, no second
interpretation free to drift. Note which field answers which question —
`Scene.Visible` is "REPLACED wholesale by each SceneSeen" and is what the
projector's `seen` map corresponds to; `Scene.Explored` "only ever grows" and is
NOT the same question. An earlier draft used `Explored` and would have emitted a
full `SceneSeen` per character per event.

**D6. `store.AppendFanout` takes a SLICE and returns the FIRST world sequence.**
A slice because `AppendBatch` must be atomic as a whole. First, not last, because
`store.AppendBatch` returns `first` today and four gateway call sites bind it as
`firstSeq`.

---

### Task 1: The oracle — one projection seat per party member

**Capture what the projection does, as committed data, while the code that does
it still exists.** Everything after this task is a port, and this is what makes
"faithful" a test rather than an opinion.

**This extends a corpus that already exists.**
`scenarios/goldens/session-zero/projections/` holds `player/` and `spectator/`,
each with `viewer.json`, `stream.json` and `state.json`. The spectator's
`viewer.json` already reads `"viewpoint": "act-healer"` — a seat perched on ONE
party member, which is exactly a character log. D1 says why that shape is the
right oracle: for `RoleSpectator`, `Projector.eyes` returns
`[]string{pr.viewer.Viewpoint}` and nothing else.

**Files:**
- Create: `scenarios/goldens/<scenario>/projections/<actor-id>/viewer.json`,
  `stream.json`, `state.json` — one directory per party member per scenario
- Modify: nothing. `keystone_test.go` needs no edit — see Step 5

**Interfaces:** none. This task produces DATA.

**THERE IS NO `-update` FLAG, and there must not be one.**
`cmd/vtt/scenario_goldens_test.go` records the rule and the reason: the original
plan for that task generated its corpus behind a switch and it was rejected —
"never to generate a golden no human derived first… A regenerate-on-demand
switch is exactly how a golden stops being a claim anyone checked."

That rule binds here in a specific way. A recorded oracle freezes current
behaviour INCLUDING any bug in it, so capturing without reading is how a defect
becomes the specification. Every `viewer.json` therefore carries a `why` written
by a person, as `session-zero/projections/spectator/viewer.json` already does
("this seat sees the camp and NOTHING of ambush — not its name, not its size").

**WHERE A CHARACTER'S LOG STARTS. Settled here, after getting it wrong once.**

**The oracle is captured from world event 1, for every seat.** That is what
today's projector does, and it is what makes D1 true: the oracle is today's code
asked this arc's question, with no new production code between.

A draft of this plan truncated each character's oracle at the world event where
that actor first qualified as a party member, reading spec §2's "their log starts
at join" literally. **That produces a log that cannot fold**, and it would have
broken the arc's own keystone invariant for two thirds of the corpus.
`classify` forwards `SessionStarted` and `SessionEnded` unconditionally — one
arm, no predicate, "Everyone at the table is at the same table… the client's
session panel is built from them" — and every party member qualifies AFTER
`SessionStarted`. A log starting at seq 6 therefore misses the start and receives
the end, which is `engine: no open session to end` in Go and a `FoldError` in
`client/src/fold.ts`. Six of the nine seats are in that shape.

**Nothing SIGHT-GATED reaches a pre-existence log.** Before the actor is in
`st.Actors` the spectator arm of `eyes` returns nil, so `look` yields empty
`squares` and `tokens`, and every loop in `transitions` and every sight arm in
`classify` iterates one of those empty sets. No scene, no token, no move, no
door.

**But ROSTER-GATED facts about OTHER party members do reach it, and that is a
real exposure this plan will not pretend away.** `classify`'s `knows()` is
`pr.actors[id] || now.actors[id]`, and `look` fills `now.actors` with every party
member from the first event. So `ActorControlGranted`, `AttackRolled`,
`AbilityUsed`, `ResourceChanged`, `ConditionApplied`/`Removed` and `ActorRemoved`
about an already-seated character forward into the log of a character who has not
joined yet. It is in the committed corpus already:
`session-zero/projections/spectator/stream.json` is perched on `act-healer`, who
enters at world seq 6, and carries `evt-5` — the raw `ActorControlGranted` for
`act-fighter` — at sequence 5. The tell is the `eventId`: synthesized entries
have none, forwarded ones do.

**Event 1 is still the right start, and the reason is that this is what the
product already does.** `seat.subscribeFrom` returns 0 for any projected seat,
and its comment says so: "A projected seat always replays the whole log." A
player connecting today receives exactly this stream. Starting a character log at
event 1 is not new exposure; it is the current behaviour, moved.

So §2's "because they were not there before" is true of the WORLD and NOT of the
roster. Task 16 amends it to say that, and puts the roster-gated backlog to
Patrik alongside the other two knowingly-moved behaviours — it is a product
decision (what does a late joiner learn about the party's past?), not a porting
one.

**THE CORPUS IS EIGHT GOLDENS, NOT NINE SCENARIOS.** `scenarios/` holds nine
files; `scenarios/goldens/` holds eight. `goblin-fight` is deliberately excluded
because a miss emits fewer events than a hit, so no masking makes its stream
comparable. An oracle can only be captured where a committed world `stream.json`
exists.

- [ ] **Step 1: Enumerate the seats, and measure the corpus you actually have**

For each of the eight goldens, list every actor `isPartyMember` admits, and note
the world sequence at which each first qualifies. Measured at HEAD this yields
nine seats across six goldens — `denials` and `smoke` have no party member at
all, so they contribute none. Record the real shape in the task report; Task 3's
vacuity guard is written per SEAT for this reason, and a per-scenario guard would
be unsatisfiable for those two.

- [ ] **Step 2: Write each `viewer.json` BY HAND, with its `why`**

```json
{
  "seat": "act-healer",
  "participantId": "oracle",
  "role": "spectator",
  "viewpoint": "act-healer",
  "why": "<what this character can and cannot see here, and why it starts at 6>"
}
```

The `why` is the claim a human is making. Write it from the scenario definition
BEFORE looking at what the projector emits — that ordering is what makes Step 4 a
check rather than a transcription. The existing session-zero spectator `why` is
the model for tone.

Record in it what the character sees of the world BEFORE they exist, which is
nothing, and what still reaches them, which is the sitting, narration and the
party roster. A reader who does not know that will read the first entries as a
leak.

- [ ] **Step 3: Derive each `state.json` BY HAND**

This is the expensive half of the task and it is not optional. Two committed
gates read it, both directory-driven and both picking up a new seat
automatically: `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`
requires a `state.json` byte-equal to `keystoneDump(FoldPrefix(projected), ...)`,
and `client/test/projection-parity.test.ts` folds the stream through
`client/src/fold.ts` and compares to the same file.

`keystone_test.go`'s header states the convention: this is "a file no machine
produced and neither look() nor sight had any hand in — a human wrote 36 squares
down from the scene's geometry". Derive the visible and explored square sets per
scene from the map, by hand, as that sentence describes.

- [ ] **Step 4: Record the streams, then reconcile against the two hand-derived files**

Write each `stream.json` in `marshalStream`'s shape — it round-trips through
`any` and therefore SORTS KEYS, unlike `cmd/vtt`'s recorder; the two conventions
coexist in the corpus deliberately.

Then read each stream against its `why` and its `state.json`, and reconcile any
disagreement: either the prose was wrong (fix it) or the projection is (stop, and
raise it — a defect found here is found before it is frozen). A recorded oracle
freezes current behaviour including its bugs, and this step is the only thing
standing between that and a defect becoming the specification.

Record in the report which seats were reconciled and how.

- [ ] **Step 5: Run**

Run: `go test ./internal/gateway/ ./cmd/vtt/ -count=1 && bun test client/test/projection-parity.test.ts`

**The gate that picks these directories up is
`TestTheProjectedGoldensAreWhatTheProjectionActuallySends`**, whose glob Step 3
already names, plus `client/test/projection-parity.test.ts`. NOT `walkKeystone`,
which never reads `projections/` at all: `keystoneSeats` derives its seats from
CONTROLLERS, while these directories are per KIND, so the two sets differ — a
party member the DM holds gets an oracle directory but lands in
`spectator-on-npc/`. No edit to `keystone_test.go` is needed either way.

Expected: PASS, with every new seat folding at every prefix. That is the first
evidence a per-character stream CAN fold standalone, which Task 9 asserts
generally.

- [ ] **Step 6: Commit**

Stage the new golden directories, with the message
`What each character can see, recorded before the code that decides it goes`.

---

### Task 2: `isPartyMember` moves to `engine`

Everything downstream needs it, and `campaign` may not import `gateway`.

**Files:**
- Modify: `internal/engine/` (new `IsPartyMember`), `internal/gateway/viewpoint.go`

**Interfaces:**
- Produces: `func engine.IsPartyMember(a *vttv1.Actor) bool`.

**Move it, do not copy it.** That function's own comment is the argument: "IT
LIVES IN ONE FUNCTION BECAUSE IT USED NOT TO. The same predicate was transcribed
FIVE times: three production call sites in two files, plus TWICE into the
keystone's oracle." A sixth copy in `campaign` reopens exactly that. `gateway`
already depends on `engine`, so `MayPerch` and `eyes` call it from its new home
and the rule still has one place that decides it.

- [ ] **Step 1: Move the function and its comment**

Take the comment with it. It records which five copies existed and what went
wrong, and that history is the reason the function is shaped this way.

- [ ] **Step 2: Run**

Run: `go test ./internal/engine/ ./internal/gateway/ -count=1 && go-arch-lint check`

Expected: PASS with no new arch edge — `gateway -> engine` already exists.

- [ ] **Step 3: Fault-inject**

Invert the predicate. `MayPerch`'s refusal tests and the spectator `eyes` arm
must both red. If only one does, the second call site was not actually rewired.

- [ ] **Step 4: Commit**

Stage both packages, with the message
`The party-member question keeps one answer, in a package both callers reach`.

---

### Task 3: The `perceive` component, and `classify` ported

**Files:**
- Create: `internal/perceive/perceive.go`, `internal/perceive/classify_test.go`
- Modify: `.go-arch-lint.yml`

**Interfaces:**
- Produces:

```go
// Verdict is project.go's, unchanged, including the ordering: Unrecognised is
// the ZERO VALUE, so a payload no arm names reaches nobody.
type Verdict int

const (
	Unrecognised Verdict = iota
	Withheld
	Forwarded
)

// View is one character at one moment. It holds no state between calls: it is
// built per event, from the character's own folded log and the world after the
// event, so the same inputs always give the same answer.
type View struct {
	Shown *engine.State // the fold of THIS character's log — D5
	World *engine.State // the world AFTER this event — D4
	Lit   Lit           // what this character can see now
}

func (v View) Classify(env *vttv1.Envelope) Verdict
```

  Task 4 adds `Transitions` to the same type; Task 7 builds the `View`.

**`Lit` is `project.go`'s `sightView` under a name that says what it is** —
visible squares per scene, and the tokens and actors standing in them.

**PORT `classify` VERBATIM. Do not re-derive it, and do not summarise it here.**
Its arms are in `internal/gateway/project.go`, one `case` each, and nearly every
one carries a written reason. Six substitutions, and nothing else:

| In `project.go` | In `View` | Why they correspond |
|---|---|---|
| `pr.actors[id]` | `v.Shown.Actors[id]` | actors this character has been introduced to |
| `pr.tokens[id]` | `v.Shown.Tokens[id]` | tokens shown and not since withdrawn |
| `pr.scenes[id]` | `_, ok := v.Shown.Scenes[id]` | scenes introduced. `Scenes` is a map of VALUES, so this is a comma-ok, not a bool read |
| `pr.seen[id]` | `v.Shown.Scenes[id].Visible` | the squares last reported visible — **`Visible`, not `Explored`** (D5) |
| `pr.doors[id][sq]` | `v.Shown.Scenes[id].OpenDoors[sq]` | the door states this character believes |
| `now.*` | `v.Lit.*` | what is visible at this moment |

**THE PROJECTOR HAS FIVE MAPS AND ALL FIVE HAVE HOMES.** `pr.doors` is the one a
draft of this plan missed; `doorTransitions` reads `believed := pr.doors[id]` and
writes back to it. `v.Shown.Scenes[id].OpenDoors` is equal to it BY
CONSTRUCTION: `pr.doors[id][sq]` is written exactly when a `DoorOpened` or
`DoorClosed` lands in that character's log, and `apply.go`'s door arms write the
same `gridKey`. If a sixth read appears during the port that has no home here,
STOP and raise it — inventing a second memory is what D5 forbids.

**`Lit` is produced here, and it takes a SET of actor ids, not one.**

```go
func LookAll(world *engine.State, actorIDs []string) Lit
func Look(world *engine.State, actorID string) Lit // = LookAll(world, []string{actorID})
```

`Look` is what the fan-out uses: one character, one log. **`LookAll` exists
because `canSee` is a different question and must not change.**
`gateway.canSee` builds its projector from `viewerFor(p)` —
`Viewer{ParticipantID: p.ID, Role: p.Role}`, with NO viewpoint — so it goes
through `eyes`'s `RolePlayer` arm: the union of every actor that participant
controls, whatever their kind. Narrowing it to one party member would silently
change `move_token` authorisation, refusing every square to a player handed a
monster. None of the three tests pinning that check would red, because all three
use single-character seats. `canSee` therefore calls `LookAll` with the
participant's controlled actors and behaves exactly as today.

`campaign` cannot build a `Lit` itself: it may not import `sight` (D2), which is
the whole reason this component exists. `sightView`'s fields are unexported and
need exporting, or `Lit` needs a constructor.

**Six arms read MEMORY, not world state, and that is why `View` has `Shown`.**
An earlier draft of this plan proposed `Perceived(st, actorID, env) bool` and
could not express them: `knows()` is `pr.actors[id] || now.actors[id]`; three
arms are `pr.actors[id]` alone; `TokenMoved` is
`passIf(pr.tokens[id] && now.tokens[id])`; and `ActorRemoved` reads
`pr.actors[id]` for an actor that is ALREADY GONE from world state, so no
function of the world alone can ever answer it. Read the arms.

- [ ] **Step 1: Write the failing test — against the ORACLE, not against prose**

```go
// TestClassifyAgreesWithTheProjectionItReplaces runs both over the golden
// corpus. The oracle is Task 1's committed per-character streams; this asserts
// the ported classify reaches the same verdict for every (character, event) in
// them. It is deliberately not a table of expected verdicts written by hand —
// two drafts of this plan wrote such a table and both were wrong.
func TestClassifyAgreesWithTheProjectionItReplaces(t *testing.T) {
	for _, g := range characterOracles(t) { // scenario x party member
		t.Run(g.Scenario+"/"+g.ActorID, func(t *testing.T) {
			var forwarded int
			for _, step := range g.Steps { // each world event, in order
				got := perceive.View{
					Shown: step.Shown, World: step.World, Lit: step.Lit,
				}.Classify(step.Env)
				if got == perceive.Forwarded {
					forwarded++
				}
				// OracleVerdict is derived from the committed stream, not written
				// by hand: a SYNTHESIZED entry carries no EventId, a FORWARDED
				// one carries the world event's. So "this event's EventId appears
				// in the oracle stream" is Forwarded, and absent is Withheld.
				// Unrecognised cannot be read off a two-valued oracle and is
				// covered by the ported arm-coverage gate instead (Step 4).
				if want := step.OracleVerdict; got != want {
					t.Fatalf("event %d (%T): got %v, oracle says %v",
						step.Index, step.Env.Payload, got, want)
				}
			}
			// VACUITY GUARD, PER SEAT. A character that forwards nothing agrees
			// with the oracle trivially. Per seat and not per scenario: denials
			// and smoke have no party member at all, so a per-scenario guard
			// would be unsatisfiable for them (Task 1 Step 1).
			if forwarded == 0 {
				t.Fatalf("%s forwarded nothing; this seat proves nothing", g.ActorID)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify behavioural RED**

Ship `Classify` returning `Unrecognised` always, so the failure is behavioural.
Expected: FAIL on the first forwarded event in the first scenario.

- [ ] **Step 3: Port the arms**

Copy the `switch` from `project.go`, applying the substitutions above. Keep
every comment. If an arm reads something `View` does not have, STOP — the table
is incomplete and guessing is what produced twenty-four defects already.

**THE MEMORY WRITES ARE DROPPED, and the argument for that is load-bearing.**
Every ported function also WRITES its memory — `pr.scenes[id] = true`, the
`delete(pr.actors, id)` forgetting loop, `pr.tokens[id] = true` and its delete,
`pr.seen[id] = lit`, `believed[sq] = open`. Under the substitution those become
writes into `v.Shown`, an `*engine.State`, and two things follow. First,
`.semgrep/event-sourcing.yml`'s `no-direct-state-writes` matches six patterns —
`Scenes`, `Actors`, `Tokens`, `Conditions` and `Notes` by key, plus a `Sessions`
append — and excludes only `internal/engine/`, `*_test.go` and
`internal/rules/conformance/`. It has no `include:`, so `internal/perceive` is in
scope automatically and they fire there. Second, and worse, `delete(...)` matches NO pattern
in that rule, so the deletes would pass silently while corrupting Task 7's cached
per-character state — the second source of truth D5 exists to prevent.

Dropping them all is correct, for two reasons that must be written into the code
rather than assumed: within ONE event no later read depends on an earlier write
(the departure and arrival loops operate on disjoint id sets by construction),
and BETWEEN events the next `Shown` is the fold of the log, which reproduces
every one of them. That includes the forgetting loop, because the only thing that
removes an id from `st.Actors` is `ActorRemoved`, and `classify` forwards
`ActorRemoved` exactly when the seat held the actor.

- [ ] **Step 4: Port the arm-coverage gate**

`internal/gateway/project_internal_test.go` holds
`TestEveryEnvelopePayloadArmHasAnExplicitRuling`. Task 12 deletes that file, and
it is the ONLY thing that reds when a new contract arm lands without a ruling —
the gate that makes "Unrecognised is the zero value" a safety property rather
than a hope. Port it into `internal/perceive` in this task, not later.

- [ ] **Step 5: Add the component to the arch rules**

```yaml
  perceive: { in: internal/perceive }
```
```yaml
  perceive: { mayDependOn: [contract, engine, sight, perceive] }
```

Inline flow style, matching every existing entry in `.go-arch-lint.yml`.
plus `perceive` on `campaign`'s list AND on `gateway`'s. **`gateway` needs the
edge too**, and not for projection: `internal/gateway/seat.go`'s `canSee` calls
`pr.look` and `pr.canSeeSquare`, and its production caller is `server.go`'s
`move_token` destination check — pinned by
`TestAPlayerCannotProbeTheDarkWithMoveCommands`,
`TestAPlayerCannotStepOntoTerrainItRemembersButCannotSee` and
`TestAPlayerCannotStepWhereItCannotSeeButTheDMCan`. That question is asked
against LIVE world state at command time and survives this arc entirely; it is
authorisation, not projection. `perceive.Look` is its new implementation.

**`perceive` lists ITSELF**: the test file
is `package perceive_test`, go-arch-lint counts that as a dependency, and
`eventgen`'s entry records the exact error text for forgetting it.

- [ ] **Step 6: Settle how the oracle tests fold, because the rules forbid the obvious way**

`characterOracles(t)` must build `step.World` and `step.Shown`, both folds of an
envelope slice. `campaign.FoldPrefix` is the only exported one; `engine` exports
`Apply` and `NewState` and no slice fold. So `package perceive_test` cannot
import `campaign` without a new arch edge, and a local loop over `engine.Apply`
is the second event-application loop CLAUDE.md rule 4 forbids and that
`FoldPrefix`'s own doc comment exists to prevent.

**Take the narrow option: an `excludeFiles` entry for the oracle test file**,
the precedent being `internal/identity/fault_internal_test.go`, which is exempted
for exactly this shape of test-only need. Widening `perceive`'s `mayDependOn` to
include `campaign` would put a production edge in the graph for a test.

This is a gate change, so per D2 it is its own reviewed decision — raise it with
the task's diff rather than folding it in silently.

- [ ] **Step 7: Run**

Run: `go test ./internal/perceive/ -count=1 && go-arch-lint check`

- [ ] **Step 8: Commit**

Stage `internal/perceive` and `.go-arch-lint.yml`, with the message
`The verdict moves to write time, and the oracle says it did not change`.

---

### Task 4: The transition walk, ported

`transitions` is what synthesizes a character's entries. It is the other half of
the move and the half both earlier drafts got wrong.

**Files:**
- Modify: `internal/perceive/perceive.go` (add `Transitions`)
- Create: `internal/perceive/transitions_test.go`

**Interfaces:**
- Produces: `func (v View) Transitions(cause *vttv1.Envelope) []*vttv1.Envelope`.

**PORT THESE FUNCTIONS, ALL OF THEM:** `transitions`, `sceneSeenFor`,
`objectInSight`, `doorTransitions`, `canSeeSquare` (both door arms of `classify`
need it and it dies with `project.go`) and the helpers those call
(`doorSubject`, `squareAt`, `sortedSet`). Dropping one is not a simplification — `doorTransitions`
records what its absence costs: "every door worked before a seat had eyes… stays
shut on that player's board for the rest of the session. It never self-corrects…
and it never throws." Found at a table, not by CI.

**Memory substitution, per D5:** `pr.scenes` → `v.Shown.Scenes`;
`pr.actors` → `v.Shown.Actors`; `pr.tokens` → `v.Shown.Tokens`;
`pr.seen[id]` → `v.Shown.Scenes[id].Visible`. **`Visible`, not `Explored`** —
`Visible` is "REPLACED wholesale by each SceneSeen", which is what `seen`
compares; `Explored` "only ever grows" and is a different question. An earlier
draft used `Explored` and would have emitted a full `SceneSeen` for every
character on every event, and never matched at all on a bare canvas.

**What it emits is not this plan's to list.** Read `transitions`. Two things are
worth naming only because a draft omitted them and the omission was silent:

- **The scene introduction is a REDACTED `SceneCreated`** — id, name, grid width
  and height, no tiles and no objects — emitted BEFORE the `SceneSeen` that
  carries the squares. Its comment: "7x3 is the shape of the paper, not a leak;
  its tiles and objects arrive square by square through SceneSeen as the viewer
  looks around." Without it, `engine.Apply` rejects the `SceneSeen` with "scene
  seen for unknown scene", and every character log fails to fold at its FIRST
  entry.
- **A synthesized `ActorAdded` is cloned with its controllers cleared**, with
  `ActorControlGranted` emitted behind it, and any conditions on that actor
  replayed. `engine.Apply` refuses an `ActorAdded` naming a controller
  ("creating an actor does not hand it to anyone"), and a later forwarded
  `ConditionRemoved` is a hard fold error if the apply was never replayed.

- [ ] **Step 1: Write the failing test — the oracle again, now on payloads**

```go
// TestTransitionsReproduceTheOracleStream is the fidelity proof for the whole
// port. For each character oracle, replaying the world log through Classify and
// Transitions must produce the SAME payload sequence the projection produced.
//
// PAYLOADS ONLY, sequences ignored: the one intended difference in this arc is
// that an entry carries its own log's sequence instead of borrowing the world's
// (spec §1). Everything else must be identical.
func TestTransitionsReproduceTheOracleStream(t *testing.T) {
	for _, g := range characterOracles(t) {
		t.Run(g.Scenario+"/"+g.ActorID, func(t *testing.T) {
			got := replayCharacter(t, g)          // Classify + Transitions
			want := g.OracleStream                // Task 1's committed stream
			if diff := payloadDiff(want, got); diff != "" {
				t.Fatalf("the port is not faithful:\n%s", diff)
			}
			if len(got) == 0 {
				t.Fatal("this seat produced no entries at all; it proves nothing")
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify behavioural RED**

Ship `Transitions` returning nil. Expected: FAIL on the first scenario with a
diff naming the first missing entry — which, if Task 1 did its job, is the
redacted `SceneCreated`.

- [ ] **Step 3: Port**

Move the functions. Keep the comments. Change only the memory reads.

- [ ] **Step 4: Fault-inject, one per thing a draft got wrong**

Each must red `TestTransitionsReproduceTheOracleStream`, and the report names
which entry the diff pointed at:

1. Emit a full `SceneCreated` instead of the redacted one.
2. Drop the redacted `SceneCreated`, keeping the `SceneSeen`.
3. Drop `doorTransitions`.
4. Use `Explored` instead of `Visible` for `seen`.
5. Skip the controller-clearing on a synthesized `ActorAdded`.
6. Skip the condition replay.

An injection that does NOT red means the oracle does not cover that behaviour,
and the corpus needs a scenario that exercises it before this task is done.

- [ ] **Step 5: Commit**

Stage `internal/perceive`, with the message
`The walk moves whole, and the corpus says it still says the same thing`.

---

### Task 5: `character_events`, and the fan-out append

**Files:** Modify `internal/store/store.go`, `internal/store/store_test.go`

**Interfaces:**
```go
// Fanout is one world event and the entries it causes in each character's log.
type Fanout struct {
	World   *vttv1.Envelope
	PerChar map[string][]*vttv1.Envelope
}

// AppendFanout writes every world event and every character entry in ONE
// transaction and returns the FIRST world sequence.
func (s *Store) AppendFanout(fs []Fanout) (firstSeq int64, err error)
```

A SLICE because `AppendBatch` must be atomic as a whole. **FIRST, not last**
(D6): `store.AppendBatch` returns `first` today and four gateway call sites bind
it as `firstSeq` — `server.go`, `adventure.go`, `map.go`, `ruleset.go`.

Schema, appended to the existing `schema` constant (spec §5):

```sql
CREATE TABLE IF NOT EXISTS character_events (
  actor_id    TEXT    NOT NULL,
  seq         INTEGER NOT NULL,
  world_seq   INTEGER NOT NULL,
  event_id    TEXT    NOT NULL,
  occurred_at TEXT    NOT NULL,
  payload     BLOB    NOT NULL,
  PRIMARY KEY (actor_id, seq)
);
```

**No unique constraint on `event_id`, deliberately.** One world event reaching
four characters is the SAME event remembered in four logs. `world_seq` is
provenance only — never folded, never on the wire — and is safe to carry
precisely because retraction is gone (`59542e1`): a world sequence beside a
derived entry was the arrangement that let an undo delete what it did not own.

- [ ] **Step 1: Write the failing tests**

Happy path: per-character sequences start at 1 PER CHARACTER, and the returned
world sequence is the first.

Atomicity (spec §4.1) — and the forcing mechanism matters:

```go
func TestAFailedCharacterWriteLeavesNoWorldEvent(t *testing.T) {
	// internal/testdb is a SQLite driver that can be told to fail ONE
	// statement. That is the point: a closed handle fails the FIRST statement,
	// so it cannot reach the arm this test is about — the character insert that
	// fails AFTER the world row is already written.
	//
	// A nil envelope does NOT work here and an earlier draft of this plan
	// claimed it did: proto.Marshal of a typed-nil message returns empty bytes
	// and no error, and stamping Sequence on it panics before that.
	// A SECOND RAW HANDLE, which is the idiom store_failure_test.go already
	// uses for the corrupt-payload arm. internal/store hardcodes
	// sql.Open("sqlite", ...) and has no driver seam; reaching testdb from here
	// would need a production var, an internal test file AND a new
	// excludeFiles entry in .go-arch-lint.yml — a gate change, which D2 says is
	// its own reviewed decision. Dropping the table on a second handle fails
	// the character insert AFTER the world row is written, which is the arm
	// this test is about, at no such cost.
	s, path := openTempWithPath(t)
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.Exec(`DROP TABLE character_events`); err != nil {
		t.Fatal(err)
	}
	_, err = s.AppendFanout([]store.Fanout{{
		World:   newEnv("w1"),
		PerChar: map[string][]*vttv1.Envelope{"a-asme": {newEnv("c1")}},
	}})
	if err == nil {
		t.Fatal("a failed character write must fail the whole append")
	}
	if !strings.Contains(err.Error(), "character_events") {
		t.Fatalf("failed for the wrong reason (%v); this test proved nothing", err)
	}
	got, err := s.ReadAfter(0)
	if err != nil {
		t.Fatalf("ReadAfter: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("world log holds %d event(s); the transaction did not roll back", len(got))
	}
}
```

- [ ] **Step 2: Behavioural RED** — stub `AppendFanout` to
`return 0, errors.New("not implemented")` first.

- [ ] **Step 3: Implement**

Follow `Append`'s existing shape — `s.mu.Lock()`, `s.db.Begin()`,
`defer tx.Rollback()`. Assign world sequences from
`SELECT COALESCE(MAX(seq),0)+1 FROM events`, character sequences from
`... FROM character_events WHERE actor_id = ?` — a DIFFERENT table, not that
query scoped — and marshal each envelope after its sequence
is stamped. Reject a non-zero input `Sequence`, as `Append` already does, and **unstamp BOTH
halves on every failure path** — the equivalent of `resetBatchSequences`, which
exists so "a caller checking `env.Sequence != 0` can never mistake a partially-
stamped batch for a persisted one". The fan-out has two sets of envelopes to
unstamp, not one.
**Iterate actor ids in sorted order** — map order would make statement order
differ between runs and a test comparing database contents would flake.

- [ ] **Step 4: Run** — `go test ./internal/store/ -count=1 -race`

- [ ] **Step 5: Commit** — stage `internal/store`, message
`One event, one transaction, every log that has to remember it`.

---

### Task 6: Reading and subscribing to one character's log

**Files:** Modify `internal/store/store.go`, `subscribe.go`, `store_test.go`

**Interfaces:**
- `func (s *Store) ReadCharacterAfter(actorID string, afterSeq int64) ([]*vttv1.Envelope, error)`
- `func (s *Store) SubscribeCharacter(actorID string, afterSeq int64, buffer int, noProgress time.Duration) (<-chan *vttv1.Envelope, func(), int64, error)`
- `func (s *Store) NotifyCharacter(actorID string, env *vttv1.Envelope)`

**A SEPARATE per-actor registry, not the existing `s.subs` slice.**
`store.notifyLocked` fans to one flat slice and `subscriber.enqueue` dedupes on
`env.Sequence <= sub.lastSeq`. Mixing world sequences and character sequences in
one registry would silently DROP entries, because the two number spaces overlap.

**Register and catch up under ONE lock**, as `store.Subscribe` already does.
Reading the log and then subscribing leaves a window where an event is missed or
delivered twice; that shape exists to close it.

- [ ] **Step 1: Write the failing tests** — scoped to one character; survives
reopen; an unknown character reads empty rather than erroring (a character who
has perceived nothing is ordinary — §2, no backfilling); and a subscriber
registered at the same moment as an append sees every entry exactly once.

- [ ] **Step 2: Behavioural RED** with stubs returning `not implemented`.

- [ ] **Step 3: Implement**, mirroring `readAfterLocked` and `Subscribe`. Fail
loudly on a corrupt payload, as `TestReadAfterFailsLoudlyOnCorruptPayload`
already requires of the world log.

- [ ] **Step 4: Run** — `go test ./internal/store/ -count=1 -race`

- [ ] **Step 5: Commit** — message
`A character's log reads as its own stream, from its own cursor`.

---

### Task 7: Fan-out inside `Append` and `AppendBatch`

**Files:** Modify `internal/campaign/campaign.go`; create
`internal/campaign/fanout.go`, `internal/campaign/fanout_test.go`

**Interfaces:**
- `func (c *Campaign) CharacterLog(actorID string) []*vttv1.Envelope`
- `func (c *Campaign) SubscribeCharacter(actorID string, afterSeq int64, buffer int, noProgress time.Duration) (events <-chan *vttv1.Envelope, unsubscribe func(), catchUpHead int64, err error)`

  **The four-return shape is not negotiable:** `internal/gateway/server.go`
  consumes exactly `(events, unsubscribe, catchUpHead, err)` and hands
  `catchUpHead` to `sub.catchUp`.

**Who gets a log:** every actor `engine.IsPartyMember` admits (Task 2). A monster
accumulates nothing because no viewpoint will read it. An actor promoted to
party member starts a log at that moment and is NOT backfilled (§2).

**THE STATE EACH EVENT IS JUDGED AGAINST IS THE STATE AFTER IT** (D4).
`campaign.Append` already folds the event into a validation snapshot; that
snapshot IS the post-event state and is currently discarded. Use it.

**Every envelope landing in a log is that log's own copy.** Synthesized entries
get a fresh `EventId` and `Sequence` 0. The forwarded event gets `proto.Clone`
PER CHARACTER, because `store.Append` stamps `Sequence` IN PLACE: one event
reaching four characters plus the world log is five rows with five sequences, so
sharing a pointer means the last stamp wins.

**`NotifyCharacter` is called HERE.** `Append`/`AppendBatch` call
`c.log.Notify(env)` after the live apply; the character equivalent has no other
home, and Task 10's live delivery depends on it. Task 6 produces it; this task
wires it, once the store call has returned nil.

**The memory is `FoldPrefix(c.CharacterLog(actorID))`** — D5, and Patrik's
framing: one log per character, and the log is the record. Held in memory per
character the way `campaign` already holds world state, rebuilt by folding on
open (`rebuildLocked` gains that loop). It is a cache OF the log, never a second
source of truth.

- [ ] **Step 1: Write the failing tests, AND ONE AGAINST THE ORACLE**

The oracle test is the important one, and this task is the only place the
end-to-end claim can be made: replay a golden scenario through `Append` and
assert each character's resulting log matches that character's committed
oracle stream, payloads only.

**It is what catches the start-point disagreement.** Tasks 3 and 4 replay from
the character's first appearance because Task 1 captured it that way; this task
fans out at append time, and the two must agree. Without this test the
divergence surfaces at Task 15, ten tasks later.

Then the hand-written ones: an event reaches only the logs that perceived it; a
monster accumulates nothing; a batch (a scene load) folds. **The batch test must
assert the log is NON-EMPTY before asserting it folds** — an empty log folds, and
an earlier draft of this plan shipped that exact vacuity.

- [ ] **Step 2: Behavioural RED** — `CharacterLog` returns empty for everyone.

- [ ] **Step 3: Implement.** In `Append`, build a `perceive.View` per party
member against the post-event snapshot, call `Classify` and `Transitions`, and
pass one `store.Fanout` to `AppendFanout`. In `AppendBatch`, the same per
envelope in batch order, accumulating `[]store.Fanout` for a single store call —
**the snapshot advances within the batch**, because a scene load introduces a
scene and then places tokens in it.

**AND SO MUST EACH CHARACTER'S `Shown`, which is the harder half.** The entries
from envelope 1 are still accumulating in `[]store.Fanout` and are NOT in the
store, so re-reading `FoldPrefix(c.CharacterLog(actorID))` for envelope 2 returns
a stale log and re-introduces what envelope 1 already introduced. A duplicate
`ActorAdded` is a fold error — "on the client a permanent state freeze", in
`project.go`'s words — and §3.3 makes it unrepairable.

**ADVANCE A CLONE, AND COMMIT IT ONLY AFTER THE STORE RETURNS NIL.** Folding
entries straight into the live per-character cache leaves it one batch AHEAD of
the persisted log when `AppendFanout` fails: `AppendBatch` returns a store error
without poisoning, so the campaign stays live, and the next append skips an
introduction the log never received — a `TokenPlaced` for an un-introduced actor,
or a `ConditionRemoved` with no `ConditionApplied`. That is §6's invariant broken
by a FAILED write, which is the worst shape available: nothing is wrong at the
time and the log is unfoldable forever.

This is the ordering `campaign.AppendBatch` already uses for world state —
validate against `snap := c.state.Snapshot()`, persist, and only then advance
`c.state`. Mirror it per character.

- [ ] **Step 4: Fault-inject** — forward everything to everyone; make `Classify`
always `Forwarded`; judge against the PRE-event state; reverse batch order; and
**fail the store mid-batch, then append again and assert the next entries still
introduce**. The fifth is the one that catches a `Shown` cache advanced before
the write succeeded, and none of the other four reach it.

- [ ] **Step 5: Commit** — message
`The world event and every memory of it land together or not at all`.

---

### Task 8: A per-append visibility memo

`sight.VisibleFrom` is ~15ms on a sparse 60x60 and ~176ms on a dense one, PER
EYE PER EVENT, recomputed with no memo — `project.go` measured it. Today that is
paid per connection after the append commits; after Task 7 it is paid per party
member INSIDE the transaction, before anyone is acknowledged. A party of four on
a dense map is ~700ms on every append, holding the store mutex.

**A cheaper question is NOT available, and this is recorded so it is not
re-proposed.** A single-square `sight.Clear` predicate would answer "can this eye
see that square" without computing 3600 — but `SceneSeen` carries `Visible`,
"the whole current set, never a delta", and filters its `Tiles` and `Objects` by
it. The whole set is required. `VisibleFrom` is the right call.

**The move is what makes the memo safe.** `project.go` names the sound key —
`(scene id, eye position, the set of open doors)`, terrain being immutable after
`SceneCreated` and open doors the only other input — and says why it was not
built there: at read time it needs a cross-event invalidation argument, and
"deciding which events cannot change visibility is exactly where a leak would
hide". A memo scoped to ONE append against ONE snapshot needs no such argument.

**Files:** Modify `internal/perceive/perceive.go`, `internal/campaign/fanout.go`;
create `internal/perceive/memo_test.go`

- [ ] **Step 1: Write the failing test** — the memo answers IDENTICALLY to the
unmemoised path across generated worlds, and a door opening changes the answer.
**`Scene.OpenDoors` is keyed by SQUARE KEY** (`gridKey(x, y)`, e.g. `"5,0"`), set
that way in `apply.go`'s `DoorOpened` arm — an earlier draft keyed the fixture by
door id, which matches no square, so the answer never changed and the test would
have red against a correct memo.

- [ ] **Step 2: RED** with a memo keyed on scene id alone; the door test must fail.

- [ ] **Step 3: Implement.** `campaign` creates one memo per `Append`, one per
`AppendBatch`, and drops it.

- [ ] **Step 4: Measure** — benchmark a party of four on a dense 60x60 with the
memo on and off, and against today's read-time path at the branch point. Put the
figures and a ceiling into spec §9 (Task 16). "A party is small" is about rows
and says nothing about a 176ms computation.

- [ ] **Step 5: Commit** — message
`One snapshot, one memo, and no invalidation question to get wrong`.

---

### Task 9: The invariant that replaces the keystone

Visibility spec §4.3 asserted
`fold(project(log, viewer)) == visibleState(fold(log), viewer)`. It goes: under
§3.3 the two sides are not meant to agree, because a rebuild would be a different
history rather than a check on this one. What replaces it:

> **Every character log folds cleanly, standalone, at every prefix.**

**Files:** Create `internal/campaign/characterlog_invariant_test.go`

**`package campaign_test`**, not `package campaign`: `propertySeeds` is declared
in `internal/campaign/property_test.go`, which is an external test package, and
this test needs only the exported `FoldPrefix` and `CharacterLog`.

- [ ] **Step 1: Write the test** — over `internal/eventgen` seeds, for every
character: `FoldPrefix(log[:i])` must succeed for every `i`. **At every PREFIX,
not on the whole log**: an entry referencing something introduced LATER folds at
the end and not in the middle, and that is the shape that froze a client in
sub-project 12. Carry a vacuity guard — at least two logs non-empty, since empty
logs fold trivially — and use `propertySeeds(t)` with its real signature,
`(seeds []int64, guardEachWalk, guardEnsemble bool)`.

**The guard is a property of the seed draw, which has to be fixed first.**
`eventgen.addActor` sets no `Kind`; the only `ACTOR_KIND_PARTY_MEMBER` in
`internal/eventgen/model.go` is on `grantControl`'s `ActorControlGranted`. So a
generated actor becomes a party member only if `grantControl` happens to fire for
it, and "two non-empty logs" would depend on the draw rather than on the code.
Either name seeds measured to produce them, or make the model emit a party member
deterministically — the second is better, and it is a change to `eventgen`, so
say so in the task report.

- [ ] **Step 2: RED on a real defect** — drop the redacted `SceneCreated` from
Task 4's port. The invariant must fail naming the prefix. Restore; expect PASS.

- [ ] **Step 3: Check the guard is not behind the thing it guards** — make
`Transitions` return nil and confirm the VACUITY guard reds, not the fold.

- [ ] **Step 4: Commit** — message
`Every prefix of every memory folds, or the client freezes four hours in`.

---

### Task 10: A seat forwards its character's log

**Files:** Modify `internal/gateway/seat.go`, `server.go`; create
`internal/gateway/seat_viewpoint_test.go`

A seat with a character viewpoint subscribes to that character's log and forwards
each entry unchanged; a DM or agent seat subscribes to the world log as today.
`projected(role)` keeps its meaning — false for `RoleDM` and `RoleAgent` — but
now selects a LOG rather than a filter. `pastResume` and the subscribe-from-zero
rule go in THIS task, not Task 12: leaving them is a dual path, which the Global
Constraints forbid.

**Name the perch machinery's fate here too, because the compiler will not.**
`perchBox`, `seat.perch` and the pump's wake loop live in `seat.go` and compile
standalone, so Task 12's "let the compiler enumerate the damage" will not find
them — they would survive as dead code behind a `set_viewpoint` that now means
"empty, replay that log" (§3.4). `reperch` and `perchSequence` DO die with
`project.go`, and `seat.perch` returns `s.pr.reperch(...)`, so that one breaks
loudly. Decide each in this task and record it.

- [ ] **Step 1: Write the failing tests**

A player seat receives its character's entries and nothing else. **Its sequences
are a contiguous run from 1** — not "exactly one frame per world event", which an
earlier draft asserted and which is wrong: one perceived event produces several
entries (scene introduction, actor introduction and grants, placement,
`SceneSeen`, then the forward). A DM seat still receives the world log (a control
that passes before the change).

- [ ] **Step 2: Behavioural RED** — the first two fail; the DM control passes.

- [ ] **Step 3: Implement.** Catch-up on connect reads `CharacterLog` from the
client's cursor, through the one-lock subscribe of Task 6.

- [ ] **Step 4: Run** `task check:fast` and the gateway suite with `-race`. Many
existing gateway tests WILL fail here — that is expected and is Task 12's
subject. Record which; do NOT delete a test in this task.

- [ ] **Step 5: Commit** — message
`A seat forwards a log, it does not filter one`.

---

### Task 11: `set_viewpoint` generalises beyond spectators

**Files:** Modify `internal/gateway/viewpoint.go`, `authz.go` and their tests

Today `set_viewpoint` is spectator-only — `commandRoles` maps it to
`{identity.RoleSpectator: true}` (note: `commandRoles`, not `playerRules`; a
draft of this plan named the wrong map). After this task any seat selects a
viewpoint it is entitled to: a spectator borrows a shoulder, a player picks among
their own characters. The authorisation table decides who may select what; the
mechanism is one.

- [ ] **Step 1: Write the failing tests** — a player may select a character they
control; may NOT select one they do not; may NOT select a monster; and a
spectator still may only perch on a party member (the pre-existing rule,
restated so generalising cannot quietly widen it).

- [ ] **Step 2: Behavioural RED.** Note the asymmetry in the report: only the
first test reds. The other three pass beforehand because a player is refused
outright, so they are controls, not evidence.

- [ ] **Step 3: Implement.** Keep `engine.IsPartyMember` as the single place that
decides what a viewpoint may be; do not duplicate it.

- [ ] **Step 4: Fault-inject** — make the control check always true; the
"character they do not control" test must red.

- [ ] **Step 5: Commit** — message
`Any seat selects a viewpoint it is entitled to, by one mechanism`.

---

### Task 12: Delete the projection

**Files:** Delete `internal/gateway/project.go`, `project_test.go`,
`project_internal_test.go`, `project_property_test.go`, `keystone_test.go`.
Modify `docs/superpowers/specs/2026-08-18-visibility-design.md`.

**The oracle survives this task, which is the whole reason it is DATA.**
Task 1's per-character streams are committed files, so Tasks 3 and 4 keep
checking the port against them after the code that produced them is gone.

- [ ] **Step 1: Delete, and let the compiler enumerate the damage.**
`go build ./... 2>&1 | tee /tmp/fallout.txt`. Work the list; comment nothing out.

- [ ] **Step 2: Re-home the tests that pinned BEHAVIOUR rather than projection.**
`server_visibility_test.go` asserts what a player SEES, which this arc still
owes; those move to Task 10's seat tests, rewritten against a character log.

**Every deleted test is accounted for in the report, one line each**: moved to X,
or deleted because the behaviour no longer exists. Two in particular:
`TestTheProjectedGoldensAreWhatTheProjectionActuallySends` and
`TestTheKeystoneCorpusCanTellAProjectionFromAPassthrough` are the Go-side gates
on the oracle corpus, including the rule that a golden hiding a creature must
carry a `projections/` directory. After this task only
`client/test/projection-parity.test.ts` still reads those files. That is
defensible — the corpus has done its job by then — but it is a deliberate
reduction in what guards it, not an accident. Removing a function makes the
compiler shout; removing a test makes nothing shout, and the coverage floor's
slack hides it.

- [ ] **Step 3: Check the arm-coverage gate survived.** Confirm Task 3 really did
port `TestEveryEnvelopePayloadArmHasAnExplicitRuling`. If it did not, a new
contract arm will reach nobody silently, and nothing will say so.

- [ ] **Step 4: Amend the visibility spec** (rule 7). Keep §4.3's statement and
record, in the repo's change-record voice, that sub-project 14 superseded it —
the reason (§3.3 makes the two sides deliberately disagree) and what replaced it.

- [ ] **Step 5: Run** `go test ./... -count=1`.

- [ ] **Step 6: Commit** — message
`The filter is gone, so there is nothing left to be the boundary`.

---

### Task 13: Re-derive the floors, re-point the adjudications

**Files:** `tools/coverage-thresholds.txt`, `tools/mutation-equivalents.txt`,
`tools/ts-mutation-equivalents.txt`

- [ ] **Step 1: Measure.** `task check:coverage`. `internal/perceive` needs its
own floor entry (Task 3 set a 95% target; this is where it is recorded). If
`internal/gateway` DROPPED, name which deleted test covered what — a floor that
falls because coverage was deleted with its subject is correct; one that falls
because a behaviour lost its only test is a defect, and the number alone cannot
tell them apart.

- [ ] **Step 2: Set floors from the measurement**, each with a dated one-line
reason. Do not ratchet unrelated packages here.

- [ ] **Step 3: Re-point mutation adjudications LAST**, once the diff has stopped
moving — a `file:line:col` key moves when anything above it moves, and this arc
moves a great deal. Run both self-tests before committing. An adjudication whose
SUBJECT was deleted is removed with a line saying the code is gone; one that
vanishes silently is indistinguishable from one dropped to pass a gate.

- [ ] **Step 4: Run the gate whole.** `go clean -cache` first, then `task check`.

- [ ] **Step 5: Commit** — message `Pay for what the deletion moved`.

---

### Task 14: The client folds a character log, and can switch character

**Files:** Modify `client/src/session.ts`, `client/src/view/player.ts`,
`client/test/app.test.ts`; create `client/test/character-switch.test.ts`

**`client/src/fold.ts` is NOT modified** (spec §7): it folds a character log
exactly as it folds today's projected stream, because a character log is a
sequence of ordinary past-tense events. If this task edits `fold.ts`, something
upstream is emitting an entry the fold does not know — a Task 4 defect, fixed
there.

Switching is the same operation as perching: **empty, replay that log** — which
is the whole read model (§3.4, §7).

- [ ] **Step 1: Write the failing tests** — switching replays the other log and
shows only what it contains; a switch EMPTIES the board first, because carrying
state across would show one character what another saw.

- [ ] **Step 2: Behavioural RED** — there is no switch affordance yet.

- [ ] **Step 3: Implement.** Clicking an owned token sends `set_viewpoint`; on
acknowledgement the client clears state and replays from sequence 0 of the new
log. The resume cursor is the character's own sequence.

- [ ] **Step 4: Run and REBUILD THE BUNDLE** —
`bun test client/ && task client:typecheck && task build:client`. `cmd/vtt/webdist`
is committed and `check:drift` compares the working tree to HEAD; the commit hook
will not catch a stale bundle. Run both test and typecheck: `bun test` accepts
`NodeList` where `client:typecheck` does not.

- [ ] **Step 5: Commit** — stage sources, tests and `cmd/vtt/webdist`, message
`One viewpoint per connection, and switching is just replaying another log`.

---

### Task 15: The invariant in TypeScript, and a scenario that leaves

**Files:** Create `client/test/character-log-invariant.test.ts`,
`scenarios/character-joins-late.json`; modify `internal/harness/engine.go`

**This closes the blind spot §1 of the spec names.** Every whole-session fold
`internal/harness` performs runs on participant 0 — the scenario runner folds
`history[sc.Participants[0].Name]`, soak folds `soakObserverName` (`soakDM`), and
`cmd/vtt`'s golden capture takes `sc.Participants[0].Name`. Participant 0 is `dm`
in all nine scenarios. After this task the harness folds each character's log as
well, and the DM fold stays — the DM log IS the world.

- [ ] **Step 1: The TS invariant** — every character log folds at every prefix,
with a vacuity guard.

- [ ] **Step 2: The scenario** — a DM, two characters present throughout, and a
third who joins partway and leaves before the end. Their log starts at join (§2,
no backfilling) and simply stops.

- [ ] **Step 3: Fold every character's log in the harness**, alongside the
participant-0 fold.

**The harness reads logs over the WIRE, not out of a table.**
`harness: { mayDependOn: [contract, engine, harness] }` — it has no access to
`campaign` or `store`. Scenarios declare PARTICIPANTS, not characters, and
`soakObserverName == soakDM`. So folding a character's log means a connection per
character issuing `set_viewpoint` and draining what comes back. Say that in the
task, and expect it to be the bulk of the work.

- [ ] **Step 4: Prove the new corpus catches something** — drop the redacted
`SceneCreated` from Task 4 and run `task test:cross`; the new scenario must red.

- [ ] **Step 5: Re-bless goldens deliberately.** Goldens will move. Read each
diff before accepting it: a golden re-blessed unread has stopped asserting.

- [ ] **Step 6: Commit** — message
`Nine scenarios folded the DM's log; now they fold everyone's`.

---

### Task 16: Walk the exit criteria, amend the specs

**Files:** the two specs, and `README.md` if its seat description no longer holds

- [ ] **Step 1: Walk §10's eight criteria against the running product**, recording
for each how it was demonstrated, naming the test or observation. Record findings
during the walk; develop only once the walk is done.

**Criterion 8 is a cold read and needs a separate agent**: give one agent ONLY
the spec, forbid everything else, and see what it builds. A reviewer who has seen
the code cannot perform it.

- [ ] **Step 2: Amend the spec for every deviation** (rule 7). D1 through D6 are
deviations from the spec's text — §4.2 says `campaign` asks `internal/sight`
directly — and each needs its amendment in the repo's dated change-record voice.
So does Task 8's measured latency ceiling, into §9.

- [ ] **Step 3: Run the gate whole** — `go clean -cache`, then `task check`.

- [ ] **Step 4: Commit** — message
`The specs say what is now true, and what they used to say`.

---

## Self-review

**Spec coverage.** §1 → Tasks 1, 3, 4 (the isolation is delivered by 10 and 12).
§2 non-goals → nothing rebuilds a log, merges streams, gives a monster a log
(Task 7) or backfills (Task 15); `internal/sight` is untouched. §3.1 → Task 10.
§3.2 → Task 4. §3.3 → the no-rebuild constraint, `Unrecognised` as the zero
value, Task 3's coverage floor. §3.4 → Tasks 11 and 14. §4.1 → Tasks 5 and 7.
§4.2 → Tasks 3 and 4. §4.3 → D5, Task 7's cache. §5 → Task 5. §6 → Tasks 9 and
15. §7 → Tasks 10, 12, 14. §8 → Tasks 1, 3, 4, 9, 15. §9 → Task 8. §10 → Task 16.

**What this plan deliberately does NOT contain.** A table of perception rules.
Two earlier drafts had one; both were wrong, in twelve places each. The rules are
in `project.go`, the oracle in `scenarios/goldens/*/projections/`, and the port is
proven by Tasks 3 and 4 rather than described here.

**Interface consistency.** `View{Shown, World, Lit}` with `Classify` and
`Transitions` — Tasks 3, 4, 7. `store.Fanout` and
`AppendFanout([]Fanout) (firstSeq, error)` — Tasks 5 and 7.
`CharacterLog(actorID)` and the four-return `SubscribeCharacter` — Tasks 6, 7, 9,
10. `engine.IsPartyMember` — Tasks 2, 7, 11. `FoldPrefix` is the fold
(`engine.Fold` does not exist).

**Test helpers live where their package can reach them.** Task 3 and 4's are
`package perceive_test`; Task 7 and 9's are `package campaign`, because they call
unexported fan-out helpers. They are not shared across that boundary.

**Two knowingly-moved behaviours, recorded rather than left to be found.**

Today `eyes` for `RolePlayer` is the union of the actors a participant CONTROLS,
whatever their kind — so a player handed a monster sees through it. After this
arc only party members have logs, so that stops. §3.2 rules it and Task 11 tests
it, but it is a visible change at the table and Task 16 puts it to Patrik.

And `objectInSight` reveals an object's WHOLE footprint when any one square of it
is visible. Recomputed per read that is harmless; written into a log under §3.3 it
becomes permanent. It moves as-is — changing it is a visibility decision, not a
porting one — and Task 16 records it as a deviation for Patrik to rule on.
