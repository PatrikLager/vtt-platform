# A viewer is told no issuer and no unseen cause: the change

**Ticket:** `docs/superpowers/specs/2026-10-05-a-viewer-is-told-no-issuer-and-no-unseen-cause-design.md`,
revised by its writer after verification (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-05-a-viewer-is-told-no-issuer-and-no-unseen-cause.md`,
verified by `verify-ticket` (passes with gaps).
**The owner's rulings:** at sign-off on 2026-10-05, Q1 (a), that no status
change's cause reaches a player or a spectator whether or not they saw it,
and Q2 to Q8 as the plan proposed: clone every forwarded frame; warn in
`events.proto`; re-point the recognisers through one `forwardedOf` and rename
the two QA tests whose names stated the overturned sentence; the property
clause, its stamped issuer and a floor; the envelope-field guard; one code
commit and the report; SPEC-007, SPEC-009, SPEC-012, SPEC-013 and SPEC-015
unedited. On the review, the same day: every finding fixed, and QA's two
out-of-scope observations made open debt; on verification the second of
them, a clamped change's delta, proved to be the design
`docs/reports/2026-07-25-ruleset-interpreter.md` records, and the owner
withdrew its entry.
**Last code commit:** `a0c48c9`, on `e628e18`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline e628e18..a0c48c9

    a0c48c9 A viewer is told no issuer and no cause

`git diff --stat e628e18..a0c48c9`: 31 files changed, 2146 insertions(+), 208
deletions(-).

The gate, `task check` whole, after `a0c48c9`, once: it exited 0 with no
step failed. Its check steps' own verdict lines read `check:comments` clean
over 259 files, `check:requirements-chain` 273 rows, 216 test files and 13
specifications, `check:doc-owner` 80 files, `check:new-prose` 1170 added
lines clean, `check:coverage` 20 packages at or above their floors
(`internal/gateway` at 94.6 % against 88.5), `check:no-pack`,
`check:no-retraction` and `check:no-create-scene` clean, `task lint` 0
issues, `check:breaking` reporting pre-release with no objection,
`check:mutation` 14 packages with zero unadjudicated survivors (twelve
mutated afresh, since `events.pb.go` is in nearly every closure, and
`internal/artlib` and `internal/campaigncfg` reusing their verdicts; six
mutants timed out in `internal/sight`, `internal/rules` and `internal/mcp`
and counted as killed), and `check:ts-mutation`, verifying its stored
report since none of its hashed inputs (client sources, tests, config,
lockfile) changed, 2917 mutants, 2818 killed, 70
survivors all adjudicated equivalent, zero unadjudicated, 29 timed out and
counted as killed. `check:mutation` began with 32.0 GiB free; `task
check:drift` exited 0 after the commit.

## Done looks like, answered

1. `[x]` A player whose own character an ability changes, used by an actor
   the player does not see, is sent the change with nothing naming the
   ability or the issuer:
   `internal/gateway/project_test.go#TestAChangeByAnUnseenUserNamesNoAbilityAndNoIssuer`,
   for a `ResourceChanged` and a `ConditionApplied`, red on `e628e18` with
   `ability:claw:hit` and the issuer on both frames, for the rogue's player
   and a spectator perched on it. QA's
   `internal/gateway/qa_issuer_test.go#TestQAIssAStatusChangeArrivesWithoutCauseWhetherOrNotItsUserIsSeen`
   and `#TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause`
   hold the same for a change to `patron`, an actor the player sees but does
   not run, the second over a real server where a user in an unseen cellar
   acts.
2. `[x]` A player or a spectator is sent no frame whose envelope carries a
   `participant_id` or an `actor_role`:
   `internal/gateway/server_visibility_test.go#TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause`,
   red on `e628e18` ("patron: seq 10 names its issuer"), with the issuer's
   own seat among the three it reads;
   `internal/gateway/project_internal_test.go#TestEveryEnvelopeFieldIsKeptOrClearedByForwardable`,
   red on `e628e18` (`forwardable keeps "actor_role"`); and the property
   walk's clause in
   `internal/gateway/project_property_test.go#TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer`,
   red at seed 1, action 1. A control event's own `participant_id`, its
   subject, still reaches a viewer that sees the actor:
   `qa_issuer_test.go#TestQAIssAForwardedFrameKeepsEverythingButIssuerAndCause`.
3. `[x]` A cause the viewer did see follows the owner's ruling, Q1 (a): it is
   not sent.
   `server_visibility_test.go#TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause`'s
   `fists`, whose user the patron sees, and its `"manual"` removal arrive
   without cause, and QA's
   `#TestQAIssAStatusChangeArrivesWithoutCauseWhetherOrNotItsUserIsSeen`
   holds a seen and an unseen user alike.
4. `[x]` The DM and the agent are still sent every event as the log holds
   it: `internal/gateway/server_visibility_test.go#TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection`
   passes on both trees; the wire test's DM half reads an issuer on every
   event after the seed and all three causes; QA's
   `#TestQAIssTheDMAndTheAgentAreSentEachEventAsLogged`. No append path
   changed, and `TestToEventStampsParticipantRoleAndID` still holds the
   log's issuer for `ToEvent`.
5. `[x]` The twelve projected golden streams change only by the removed
   fields, 133 lines out and 4 in, byte-equal to a line deletion of those
   fields from the committed streams; no `state.json`, `viewer.json` or
   log-level `stream.json` moves; `internal/gateway/keystone_test.go#TestTheProjectedGoldensAreWhatTheProjectionActuallySends`
   and `client/test/projection-parity.test.ts` pass. `task check` whole is
   green, as the gate paragraph above records.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A player or a spectator is never sent an envelope's `participant_id` or `actor_role`, the issuer of its event. | VTT-272 |
| A player or a spectator is never sent the cause of a status change whose cause it did not see. | VTT-273, which under Q1 (a) refuses every cause, seen or not |
| Projecting an event writes nothing to the event. | VTT-222, cited by the unit test, not restated |

## The sort

Two rows accepted as D8 proposed (A and B, now VTT-272 and VTT-273), and ten
candidates refused, as D8 lists them; none was reversed.

## Phase 4a: QA adjudications

**`a0c48c9` (VTT-272, VTT-273, with VTT-176, VTT-222, VTT-243, VTT-244,
VTT-249 and VTT-262), QA on opus, given those rows, SPEC-016 whole, the text
of `Envelope`, `ResourceChanged`, `ConditionApplied`, `ConditionRemoved`,
`TokenMoved` and `AbilityUsed`, `go doc -all` of `internal/gateway`,
`internal/engine`, `internal/campaign`, `internal/identity` and
`internal/rules`, and `rulesets/tavern-brawl` as data; it wrote
`internal/gateway/qa_issuer_test.go`.** Eleven tests, eight driving the
projector and three a real server, none failing, under `-count=5` and
`-race`; nineteen injections into its own file, all red, the file restored
by its hash.

- VTT-249 names no time where SPEC-016's correction paragraph does: the row
  is narrower than the record; QA tested to the record and it passes. No
  change; the row is not this ticket's.
- The keep half of `forwardable` has no row: it is the sort's third refusal,
  held by the unit test's equality and the envelope guard, and QA's keep
  test pins it as well. No change.
- `AttackRolled.Modifier.source` is kept and could name a party outside the
  sight test: the plan's D3 and Gap 7, since nothing in production writes
  it. Made open debt by the owner's ruling.
- A path other than the WebSocket that sends a player envelopes: none; the
  gateway's routes in `server.go` carry an envelope on `/ws` alone. No
  change.
- Presence frames carry a `participant_id` by design (SPEC-011), outside
  VTT-272's "envelope". No change.
- QA saw a `chair-swing` on a full drink logged `delta: 5, new_value: 5` and
  read the delta as not matching the change. Not a defect: `delta` is the
  requested change and `new_value` the value after the clamp, and
  `engine.Apply`'s `ResourceChanged` arm recomputes the clamp and refuses an
  event whose `new_value` differs, the design
  `docs/reports/2026-07-25-ruleset-interpreter.md` records. The owner
  withdrew its debt entry.
- Requirements QA asked an id for: five SPEC-016 sentences, the keep half
  and "a copy" (the sort's third refusal), the introduction's absent source
  and the DM's same pointer (VTT-249's and VTT-176's neighbours in the
  record), and the correction's time (VTT-249's gap). None dispensed.
- The implementer changed QA's file: six citation lines moved to the end of
  their comment blocks and the quotations ended with a period, for
  `check:new-prose`; six comment blocks describing helpers deleted, per the
  reading review (SPEC-010); and a quotation re-pointed to SPEC-016's
  corrected sentence.

## The breaks

The commit's message carries them, one line each, with the checks that
spoke; each was run in a scratch clone of the commit's final tree and
restored from its saved text, checked by hash. Fourteen: `forwardable`
returning the event; the event cleared in place; the copy clearing the
event id; the copy keeping the role, then the participant; every status
cause kept; `ConditionRemoved`'s reason alone kept; a move's reason kept; the
copy clearing the session; the DM's arm forwarding a copy; `ResourceChanged`'s
reason alone kept; `ConditionApplied`'s source alone kept; every cause in the
walk emptied; the guard's cleared table without `actor_role`. Every one went
red.

## Rule 9: how MapTool does this

Answered in the plan, from `~/dev/RPTool/maptool` at `f4b7fef6c`: a status
change in MapTool carries no author and no cause. `PutTokenMsg` and
`UpdateTokenPropertyMsg` carry the token and what changed, and
`ServerMessageHandler.handleMessage` relays them to every other client; "who"
travels only in a move's player id and in chat, where `TextMessageDto.source`
names the sender and `MapTool.addServerMessage` filters GM lines on the
client. Borrowed: the update's shape, a change that says what changed and
nothing of who or why. Refused: chat's broadcast source with a client-side
filter, which is the distribution model rule 9 names; and a per-message
audience on the author, which no client asks for and the ruling does not
need.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the ticket's first rule and Done item 2: the participant or role that issued an event | bound to the envelope's own `participant_id` and `actor_role`, by its writer before sign-off; SPEC-016 likewise, after the reading review | a control event's `participant_id` is its subject, which both folds require (the verification's defect 1, the review's Must) |
| D4: a floor of five seeds of six for a forwarded status change whose event carried a cause | six of six, with the walk's injected resource change given the cause `prop` | the reading review: the walk never saw a `ResourceChanged`'s cause, so `ResourceChanged`'s arm alone had no walk to red it |
| D7: one warning on `actor_role` naming both fields | a warning on each, so the generated `ParticipantId` doc carries one too | a warning above `actor_role` documents only `ActorRole` in `events.pb.go`; `ParticipantId`'s doc carried none |
| D14: K2 to K7, K9 and K10, the walk's floor (eventgen's two causes emptied) and the guard | K1 to K14: K1 and K8 from the plan's K table, K11 and K12 new, the floor numbered K13 and also emptying the walk's injected resource change's cause, the guard numbered K14 | the review asked for each cause arm alone: K7 and K8 already were, K11 and K12 add `ResourceChanged`'s and `ConditionApplied`'s; once the floor counted the injected `prop` (D4's row above), K13 had to empty it too; K1, `forwardable` returning the event, is the K table's first and the plainest regression |
| D5: `TestAnEventNamingAnUnknownActorIsWithheld`'s recogniser `proto.Equal(e, forwardedOf(known))` | `known` gains an event id and the recogniser demands one | without it a frame the projection builds could equal the expected copy |
| "`keystone_test.go` needs no edit" | its golden test's citation gains VTT-272 and VTT-273 | row A's and B's evidence name the test, and the chain requires the file to carry the id |
| D9, read as covering QA's new file: "The QA files gain no comment line" | `qa_issuer_test.go` keeps quotations and citations only | the reading review's SPEC-010 finding: six blocks described helpers |
| D14: "restored by the inverse edit" | each break restored from its saved text and checked by hash | the breaks ran by script in a scratch clone, where the saved text is the inverse edit and the hash proves it |
| the owner's ruling on the review: two open-debt entries | one, for `Modifier.source` | the second rested on QA's reading of a clamped change as a mismatch, which the implementer carried into the review; verified as designed, the owner withdrew it |

## What could not be established

- **VTT-176's wire test cannot see an event cleared in place or a DM arm
  that forwards a copy** (the plan's Gap 3); K2 is held by the unit test,
  `TestAForwardedMoveCarriesNoReasonAndTheEventKeepsIt` and the walk, and K10
  by `TestTheDMReceivesEverythingUnchanged`,
  `TestTheAgentSeatReceivesEverythingUnchangedToo` and
  `keystone_test.go#TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees`.
- **A batch shares one `occurred_at`, and withheld events leave gaps in a
  seat's sequences**, so a viewer can tell that a forwarded change came with
  an event it was not sent, though not which (the plan's Gap 6).
- **`NarrationAdded.as` can name an actor a viewer does not see**; narration
  is forwarded whole by the ruling of 2026-09-29 (Gap 9).
- **Presence names every participant**, and with control frames a viewer can
  work out who runs an actor it sees (Gap 10).
- **The log's issuer on the batch paths has no row.**
  `TestToEventStampsParticipantRoleAndID` holds it for `ToEvent`. The
  `use_ability` batch is observed by the DM half of
  `server_visibility_test.go#TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause`
  and by QA's `#TestQAIssTheDMAndTheAgentAreSentEachEventAsLogged`. Nothing
  observes `handleLoadAdventure`'s, `handleLoadMap`'s or `handleRemoveActor`'s
  batch (Gap 5; the open debt entry on a removal batch's participant and
  role).
- **No golden holds a cause the viewer did not see**;
  `TestAChangeByAnUnseenUserNamesNoAbilityAndNoIssuer`, QA's
  `TestQAIssAStatusChangeArrivesWithoutCauseWhetherOrNotItsUserIsSeen` and the
  walk do (Gap 8).

## What was deliberately left out, and where it went

- `AttackRolled.Modifier.source`: `docs/verification-debt.md`, Open debt, by
  the owner's ruling.
- A clamped change's delta: nowhere; it is designed, as
  `docs/reports/2026-07-25-ruleset-interpreter.md` records.
- SPEC-007, SPEC-009, SPEC-012, SPEC-013 and SPEC-015: unedited, by Q8.
