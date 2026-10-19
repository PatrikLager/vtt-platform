# A creature a player runs is a party member: the change

**Ticket:** `docs/superpowers/specs/2026-10-19-a-creature-a-player-runs-is-a-party-member-design.md`,
verified by an agent that did not write it (passes with gaps G1-G7).
**Plan:** `docs/superpowers/plans/2026-10-19-a-creature-a-player-runs-is-a-party-member.md`,
written by the verifier and amended by the ticket's writer after the verdict
and after the reading review of `5b1a629`, the commit that carried both.
**The owner's rulings, 2026-10-19:**
- asked whether a spectator may perch on a summoned wolf, he ruled that a
  summoned creature is a party member until it vanishes, "with everything
  that implies";
- asked whether that was new functionality at all, he chose that such a
  creature is an ordinary party member, granted `ACTOR_KIND_PARTY_MEMBER`,
  with no kind of its own ("Vanlig partymedlem");
- revoking its last controller does not close its log ("Loggen
  fortsätter");
- the kind is the grant's to set and never follows control ("det ska
  fungera som idag");
- at sign-off: the plan as written ("Ja, kör"), the rule of earlier tickets
  for review findings ("Ja, samma regel"), and a party member removed out of
  a character's sight staying on that character's roster left as it is
  ("Låt det vara").

**Last code commit:** `fd2a9ee`, on `abf4eb5`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `5b1a629` | The ticket, the plan, VTT-389 to VTT-392 |
| `dfc425d` | Task 1: the agent's tool descriptions |
| `11071e9` | Task 2: `ActorKind`'s comment in the contract |
| `fd2a9ee` | Task 3: the walk and QA's tests |

## What the code did on `abf4eb5`

A creature granted `ACTOR_KIND_PARTY_MEMBER` already had everything a party
member has: a log of its own from the fan-out (SPEC-021), a place on every
log's roster and eyes of its own (SPEC-016), a player's `set_viewpoint` on it
and the move gate judged by its eyes while watched (SPEC-015), and a log that
paused at a non-party grant or a removal and resumed at the next party-member
grant (VTT-316, VTT-318). The agent was told the opposite:
`grant_actor_control`'s `kind` description offered `ACTOR_KIND_NON_PARTY` for
"a monster, NPC or creature for someone to operate", and `add_actor`'s said "a
charmed monster is still a monster". `ActorKind`'s comment in
`contract/vtt/v1/events.proto` gave the same example and named "a familiar, a
summon" as a foreseeable third kind.

An earlier ticket for a third, temporary kind with logs of their own was
written the same day and withdrawn before any commit, when the owner asked
whether this was new functionality at all.

## Done looks like, answered

1. [x] `TestGrantActorControlSaysACreatureAPlayerRunsIsAPartyMember` in
   `tools/toolgen/main_test.go` asserts the continuous phrase that ties a
   creature a player runs to `ACTOR_KIND_PARTY_MEMBER`, summoned, possessed
   and charmed, the run's end by a non-party grant and a revocation or
   `remove_actor`, that revoking alone leaves it a party member, and the
   absence of "for someone to operate". Every phrase was measured absent on
   the base, and the test red.
2. [x] `TestAddActorSaysAnIDIsNeverReused` asserts `add_actor`'s `kind` names
   no charm and says the grant decides, and its `actorId` says never reuse an
   id because a used id continues the earlier actor's log. Red on the base.
3. [x] `contract/gen/tools/tools.json`, `cmd/vtt/tools.json` and
   `contract/testdata/expected_tools.json` are byte-identical (`cmp`), each
   four lines from the base. `TestToolsMatchGolden` and
   `TestRunOutputMatchesCommittedToolsJSON` pass; `task check:drift` exited 0
   after `dfc425d`, `11071e9` and inside `task check`. The two false sentences
   in `TestAddActorFieldDocsNameOptionalFieldsAgainstFabrication` are
   rewritten, and a third clause the Task 0 review found false, that
   `actorId`'s description "only states the bound", is deleted.
4. [x] `grep -rn 'still a monster\|a familiar\|a summon'` over
   `events.proto`, `events.pb.go`, `events_pb.ts` and `internal/gateway` finds
   one line, `internal/gateway/viewpoint_test.go`'s failure message "a
   monster the DM holds is still a monster", which is true and stays. The
   proto diff has no line outside a comment; `task check:breaking` reports
   nothing; `task build:client` leaves `cmd/vtt/webdist` unchanged.
5. [x] `TestACreatureGrantedToAPlayerAsAPartyMemberIsRunAsOne` in
   `internal/gateway/creature_test.go` passes on the tree, as the ticket said
   it would, and nine production breaks red it at their own steps (below).
   The walk does not assert the item's "Watching another character changes
   no kind: only the grant does", which is held by a reading: outside tests,
   `engine.Apply`'s `ActorControlGranted` arm holds the only `.Kind =` write
   (`grep -rn -E '\.Kind = ' --include='*.go' internal cmd | grep -v _test`),
   and its `ActorAdded` arm stores the actor whole.
6. [ ] SPEC-021 records the ruling in the specification step that follows
   this report, in its own commit. The item stands.
7. [x] `task check`, whole, exited 0 on `fd2a9ee` in 4221 s, launched
   detached; the figures are below.

## What the rules became

- Rule 1 became **VTT-389**, held by T1.
- Rule 2 became **VTT-390**, held by T2.
- The Task 0 reading review found two behaviours of the fold that the first
  rule's run-end steps rely on and no row stated: **VTT-391** (a grant restates
  the kind even for a participant who already controls the actor) and
  **VTT-392** (a revocation leaves a party member a party member). Both are
  held by existing tests in `internal/engine/actor_control_test.go`, which now
  carry the ids; VTT-391's test gained the party-member-to-monster direction
  in Task 1, and the walk and two QA tests join its evidence, one QA test
  VTT-392's.
- The sort refused six candidates, listed in the plan's Requirements.
- The walk adds no row and joins the evidence of VTT-316, 317, 318, 321, 326,
  340, 342, 343, 350 and 391.

## Phase 4a

Tasks 1 and 2 had none: prose for the agent, a contract comment and
generated files. Task 3 had one QA agent, given the ruling, eleven rows,
SPEC-007, SPEC-015 and SPEC-021 whole, and `go doc` of gateway, campaign and
identity, and withheld the walk, the plan, the verdict and the source. It
wrote eleven tests in `internal/gateway/qa_creature_test.go`, one of them
with three subtests, all passing, and showed each able to go red: its
injection logs hold 34 distinct injections into its own inputs and
expectations, each red, at least one per test. The Task 3 commit message
says 37, which no log holds.
`TestQACreatureRemoveActorEndsTheRun`'s one injection was aimed at a check
the Task 3 review dropped; its remaining VTT-316 check reds when its
condition is made always true, by an overlay the report's reviewer ran and
the writer ran again.

### QA adjudications

No QA test failed, so there was no behaviour defect or spec ambiguity to
rule. QA's seven spec gaps and its list of sentences needing an id:

- QA finding 1: spec gap — the opening's party, which SPEC-016 defines and QA
  was not given — no change.
- QA finding 2: spec gap — a re-added actor, which VTT-313 states and QA was
  not given — no change.
- QA finding 3: spec gap — what a log "holds", computed by QA from the log's
  own entries — no change.
- QA finding 4: spec gap — what a player sees between the non-party grant and
  the revocation — no change; VTT-389's description names both calls and no
  rule binds the window.
- QA finding 5: spec gap — a removal batch's earlier entries, which SPEC-015
  says are dropped with the subscription and VTT-348 does not promise — no
  change.
- QA finding 6: spec gap — absence checks sampled by a sentinel on another
  connection — no change.
- QA finding 7: spec gap — a player revoking themselves, untested — no change;
  VTT-392 holds every revocation.
- Sentences needing an id: each was mapped to an existing row and QA's
  citations re-pointed (a kindless grant to VTT-151, a log still written to
  VTT-319, the resume's present square to VTT-318 and VTT-319, a cursor no
  longer watchable to VTT-349, the default to VTT-340); "watching another
  character changes no kind" was refused by the sort and its comment dropped.
  No id was allocated.

QA's comments were reduced to citation lines, as in the earlier QA files
(rule 10), and the Task 3 review then re-pointed two of them and dropped two.

## Phase 4b

- Task 0 (`5b1a629`): one reviewer. Must: the contradicting sentences'
  origin. Shoulds: charm ruled on 2026-08-23; MapTool's default views; two
  description sentences; D10's reason; a third false test sentence and two
  gateway test comments; VTT-391 and VTT-392; a hook claim. Nits applied.
- Task 1 (`dfc425d`): one reviewer. Must: T1 never tied the creature clause
  to `ACTOR_KIND_PARTY_MEMBER`, so moving it under `ACTOR_KIND_NON_PARTY`
  passed. Shoulds: `revoke_actor_control`'s description said revocation is
  for reassigning only; `actorId`'s condition was narrower than the risk.
- Task 2 (`11071e9`): read by the writer by command; its comment text had
  been read and amended by Task 0's reviewer.
- Task 3 (`fd2a9ee`): one reviewer with overlay probes. Must: the walk was
  VTT-343's evidence and sent no refused `set_viewpoint`. Shoulds: four QA
  citations named the wrong row or a check that could not fail alone.

## The verifier's gaps, G1-G7

All seven were fixed in the ticket before `5b1a629`: G1 the third-kind
sentence, G2 the 2026-08-23 history, G3 the third tools.json copy and the
tests the change moves, G4 two questions answered, G5 VTT-318 for a reused
id, G6 MapTool's `Token.Type`, G7 the grant deciding whatever the actor was
added as.

## The deliberate breaks

Task 1, each in `tools/toolgen/main.go` unless named, red at its own test
and restored byte for byte:

| Break | Red |
|---|---|
| a creature "for someone to operate" under `ACTOR_KIND_NON_PARTY` again | T1 |
| the run-end sentence cut to "it may change." | T1 |
| summoned, possessed and charmed dropped | T1 |
| the revocation step dropped | T1 |
| the revoking-alone sentence dropped | T1 |
| the creature clause moved under `ACTOR_KIND_NON_PARTY` | T1 |
| "and a charmed monster is still a monster" back | T2 |
| "unique in the campaign" back in `actorId` | T2 |
| `revoke_actor_control`'s old description | `TestRevokeActorControlSaysItEndsAPlayersRunOfACreature` |
| `engine.Apply` writing a grant's kind only when it promotes | `TestARegrantToAnExistingControllerStillRestatesTheKind` |

Task 3, each in production code, red at the walk's step with the earlier
steps green:

| Break | Step |
|---|---|
| `engine.Apply`'s grant never writing a kind | A1, the `set_viewpoint` refused |
| `pumpState.onPerch` subscribing from cursor 1 | A1, the log never arrives whole |
| `seat.judgedBy` always answering the default | A2, the accepted move |
| `handleCommand`'s sight refusal disabled | A2, the refused move |
| `seat.resolve` keeping any shown actor | A3, no `ViewpointChanged` |
| `Authorize` accepting any `set_viewpoint` | A3, the refused perch |
| the grant's kind written after its idempotent return | A3, no `ViewpointChanged` |
| `fanOutLocked` deleting a log at a pause | A4, two `SessionStarted` |
| the resume written with no lead | A4, the resume wrote nothing |

Three of them, `onPerch`'s cursor, `resolve` and the kind written after the
idempotent return, red through the shared helper's "no frame within 3s"
inside the step's block rather than through the step's own message.

## Rule 9: how MapTool does this

`Token.Type` has two values, `PC` and `NPC`, which the GM sets on any token;
no third for a summon. Under MapTool's defaults
(`StartServerDialogPreferences.getUseIndividualViews` off), an unselected
owned token gives a view only when it is a `PC`, so a summoned creature is
looked through as a character once the GM makes it one. That is the outcome
here through a grant of `ACTOR_KIND_PARTY_MEMBER`. The union of views, sight
by ownership and the filter in the client were not borrowed.

## Figures

Measured on `fd2a9ee` by `task check`, 2026-10-19; the run's log is not
committed.

- Coverage: `internal/gateway` 94.0% against its 93.5 floor,
  `internal/engine` 99.4% against 97.5, `tools/toolgen` 86.8% against 85.0;
  21 packages at or above their floors, 11 of them clearing it by more than
  1.0 point.
- Mutation: 15 packages, zero unadjudicated survivors. `internal/artlib` and
  `internal/campaigncfg` reused their last verdicts as unchanged; the other 13
  printed no such line and were mutated in the run. `internal/gateway` had
  one survivor, adjudicated. Seven mutants timed out and were counted as
  killed: four in `internal/sight`, one each in `internal/perceive`,
  `internal/rules` and `internal/mcp`. The client's figures are its stored
  report, checked rather than re-mutated: 2944 mutants, 2868 killed, 9 timed
  out, 67 survivors adjudicated.
- `check:race`: `internal/gateway` 82.2 s, `cmd/vtt` 177.5 s.
- `check:requirements-chain`: 392 rows, 266 test files, 15 specifications;
  `check:new-prose` and `check:comments` clean.
- Free disk at the end: 18.2 GiB.

Comment ceilings the branch lowered: `internal/engine/actor_control_test.go`
31.5 (from 33.0), `internal/gateway/add_actor_validate_test.go` 48.0 (from
48.8), `internal/gateway/grant_validate_test.go` 38.0 (from 40.7); the walk's
file and QA's file have new rows at 0.0.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Task 1 changes three descriptions | Four, `revoke_actor_control`'s too, with its own test | Its description said a revocation only reassigns, against the run-end step D1 tells the agent to take |
| Task 1 touches `tools/` and the JSON | Also `internal/engine/actor_control_test.go` | VTT-391's test held only the promoting direction, and the run's end uses the other |
| The walk joins nine rows | Ten, VTT-391 too, and QA tests join VTT-391 and VTT-392 | The Task 3 review showed the walk re-grants to the existing controller twice, and QA's tests hold both rows through the gateway |
| One ceiling row, for the walk | Two, the QA file's too, and three rows lowered | Every new file under `internal/` gets a row; shares fell where comments were cut |
| Two gateway test comments rewritten | Cut to four lines and a pointer | `check:comments`' bound on a touched block is six lines |
| D1's `kind` text ends "Revoking alone leaves it a party member, on every roster." | It ends "Revoking alone leaves it a party member." | The Task 1 review: a viewer that holds an actor keeps the kind it last saw (SPEC-016), so not every roster has it |
| D3's `actorId` says "a party member added under a used id continues the earlier actor's log, if it had one" | "an actor added under a used id continues the earlier actor's log, if it had one, once it is a party member" | The Task 1 review: an actor added as a monster and granted party member later continues the log too |
| Seven breaks for Task 1 and seven for Task 3 | Ten and nine | Each review added a check (T1's continuous phrase, `revoke_actor_control`'s test, VTT-391's second direction; A3's refused `set_viewpoint`, VTT-391 in the walk), and each check got a break of its own |
| QA gets nine rows, SPEC-015 and SPEC-021, and `go doc` of gateway and campaign | Eleven rows, VTT-391 and VTT-392 too, SPEC-007 too, and `go doc` of identity too | The plan's Task 3 brief was not amended when the Task 0 review added VTT-391 and VTT-392, and the run's end rests on both; SPEC-007 says what a grant command must carry; `gateway.New` takes an `*identity.DB` |
| One reviewer agent per task | Task 2 read by the writer alone, by command | The writer judged it covered: the proto comment's text was D6's, which Task 0's reviewer read, and the rest was generated. The two gateway test comments it cut were read by no second party until this report's reviewer, who found both true. The process gives no writer that judgement, and the owner was not asked |

## What could not be established

- Whether an agent follows the descriptions. No gate here runs an agent.
- Whether an agent ends a run with both calls. A creature left non-party in
  a player's control is moved under their own character's eyes, as before.

## What was deliberately left out, and where it went

- The client's grant control (`client/src/view/dm.ts`): its `KIND_CHOICES`
  offers both kinds already, so it needs no change and has no destination.
- A party member removed or demoted out of a character's sight staying on its
  roster: the owner's "Låt det vara", 2026-10-19.
- The withdrawn temporary-kind ticket: never committed. Its third kind, and
  a log that closes for good when the creature is removed or granted another
  kind, are superseded by "Vanlig partymedlem"; its rulings that such a
  creature is a party member and that revoking does not close its log stand
  above.
- `gofmt -l` lists `internal/gateway/scenario_test.go` on `abf4eb5` and on
  `fd2a9ee`, and no step of `task check` or of the hooks runs `gofmt`.
  Earlier reports noted the file and left it, from
  `docs/reports/2026-09-29-a-note-says-who-may-read-it.md` on. It goes to the
  owner with this ticket's hand-over: formatting the file is outside this
  ticket, and a gate that runs `gofmt` is a gate change (CLAUDE.md rule 2).
