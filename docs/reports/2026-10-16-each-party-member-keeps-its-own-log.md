# Each party member keeps its own log: the change

**Ticket:** `docs/superpowers/specs/2026-10-16-each-party-member-keeps-its-own-log-design.md`,
returned by its first verification (four failures) and revised on the owner's
answers, amended after the first commit's review, and amended again in
`136e6c8` for the owner's ruling on an anchor that spans a resume.
**Plan:** `docs/superpowers/plans/2026-10-16-each-party-member-keeps-its-own-log.md`,
written by the agent that verified the ticket; its header, G1's answer, the
sort's table under "Before Task 1" and a dozen corrections from the first
review were edited by the ticket's writer before `dad9d85` first committed it.
**The owner's rulings:**
- 2026-10-15, for the arc: four tickets in order, and this is the second; a
  late log opens with the present; no dual path, no migration, no log ever
  regenerated.
- 2026-10-16, after the first verification:
  - an anchored narration is re-anchored onto the log's own entries;
  - a log pauses while its actor is not a party member, and resumes without
    a second opening;
  - frames that would not fold onto a log are not written to it;
  - the cost of deciding at append time is accepted for now.
- 2026-10-16, at sign-off: G1 (a), a resume hides every token the paused
  board holds; the plan signed off; every review finding fixed and each task
  committed without asking unless a finding is a design question.
- 2026-10-16, at Task 6's start: an anchor whose range spans a resume names
  only what came after the resume ("bara efter återupptagningen"). VTT-328
  reads it as what events at or after the latest resume in the range caused,
  the resuming event's own frames included: QA's reading A, which the code
  follows.

**Last code commit:** `77d75e0`, on `4fe0472`, `main` at the time. Every code
reference below is to its tree.

## The period, in commits

    git log --oneline 4fe0472..77d75e0

    77d75e0 The mutation gate and the coverage floors hold the character logs
    b333a16 A character log holds what a spectator perched on its character is sent
    00b0cb7 Every character log folds standalone at every prefix
    136e6c8 A character log pauses while its actor is not a party member, and resumes
    90943d9 An anchored narration names the entries of its own log
    d680433 Every party member's log is written as the event is appended
    47ebac8 Append and AppendBatch share one locked path, unchanged in behaviour
    0b81498 The store writes a world event and its character entries in one transaction
    dad9d85 campaign may depend on perceive: the ticket, the plan and VTT-315..327

    git diff --stat 4fe0472..77d75e0

    38 files changed, 6544 insertions(+), 527 deletions(-)

Most of the insertions are tests: the two QA files alone are about 2,900
lines.

## What the code did on `4fe0472`

The campaign's log was one table, `events`. `campaign.Append` and
`campaign.AppendBatch` wrote each event there alone. What a player or a
spectator was sent was decided after the append, per connection, by the
gateway's `Projector` calling `perceive.Projection.Event` with the viewer's
fold. Nothing recorded what a character perceived.

## Done looks like, answered

1. **[x]** `internal/store` holds `character_events` beside `events`.
   `TestACharacterLogNumbersItsEntriesFromOne` (VTT-321) appends to two
   actors' logs, interleaved with causes out of order, across a failed batch
   and a reopen, and reads each log back numbered from 1 with no gap. It does
   not compile on `4fe0472`.
2. **[x]** `TestAFailedCharacterWriteLeavesNoWorldEvent` (VTT-315) fails the
   second entry of a batch with a SQLite trigger, after both world rows and
   one entry are written. It shows the world log and every character log as
   they were and every sequence unstamped, and the same batch landing once
   the trigger is gone.
3. **[x]** `TestAFailedFanOutDoesNotAdvanceACharactersMemory` (VTT-325)
   fails the store write for a move that would bring the goblin into the
   hero's sight. A condition on the goblin appended next gives the hero's log
   nothing, the same move appended again introduces the goblin with its
   condition, and the log folds. `TestAFailedAppendAfterADropDoesNotRepeatIt`
   holds the same rule after a dropped frame set.
4. **[x]** `TestACharacterLogIsWhatASpectatorPerchedOnItIsSent`
   (`internal/gateway/character_log_test.go`, VTT-319) runs eventgen seeds
   1-6 × 200 one action at a time, and the nine corpus streams and a scripted
   pause-and-resume history in batches of one to four. After each event,
   every party member's new entries after its opening or resume lead equal,
   envelope for envelope apart from the sequence and a narration's anchors,
   the frames a gateway `Projector` for a spectator perched on it sends. The
   `Projector` is seeded with the log so far at the opening and at every
   resume (`NewProjectorWithSentForTest`). A log whose actor is not a party
   member takes nothing, and no lead may hold a scene, sighting, token
   placed or moved, door or narration frame. 2.8 s.
5. **[x]** Six campaign tests, all in `internal/campaign/character_log_test.go`:
   - `TestANonPartyActorHasNoLog` (VTT-316);
   - `TestALateLogOpensWithThePresent`, `TestALogOpensWhenAGrantMakesAPartyMember`
     and `TestALogOpensForAPartyMemberThatHadNone` (VTT-317), with
     `TestALogOpeningMidBatchIsTheWorldBeforeItsEvent` for an opening inside a
     batch;
   - `TestALogPausesWhileItsActorIsNotAPartyMember` (VTT-316, VTT-318), which
     includes the reused-id case G1 is about;
   - `TestANewActorUnderAnOldIdContinuesItsLog` (VTT-318).
   QA's `qa_character_log_test.go` and `qa_pause_resume_test.go` hold the
   same rules, written from the rows, the ticket, the arc design and SPEC-016
   without the code.
6. **[x]** `TestAnAnchoredNarrationNamesTheEntriesOfItsOwnLog` (VTT-322),
   six cases: a range wholly seen, several entries for one event, a range
   that starts before the log opened, a range in the same batch, a range
   across a reopen, and one none of whose events reached the log (unanchored).
   The resume clause added by the owner's ruling is VTT-328, held by
   `TestAnAnchorAcrossAPauseNamesOnlyWhatCameAfterTheResume` (live and after a
   reopen), `TestAnAnchorEndingAtAResumeThatCausedNothingIsUnanchored` and
   three of QA's tests.
7. **[~]** `TestEveryCharacterLogFoldsAtEveryPrefix`
   (`internal/campaign/character_log_property_test.go`, VTT-323) folds every
   prefix of every log over eventgen seeds 1-3 × 150 and the nine corpus
   streams appended again, and requires nothing dropped. Its guard is split
   by source and judged only on a whole, passing run, where the item names
   one guard over both: the seeds give 11 non-empty logs and 38 anchored
   narrations, the corpus 11 logs and 8 ending a session open at their
   opening. At the committed sample the seeds give one such log (seeds 1-6 ×
   300 give 14), so that clause is held by the corpus.
   QA's `TestQAPauseStatusChangedDuringThePauseStillFoldsAfterTheResume` folds
   every prefix of a resumed log, which no seed or corpus history produces.
8. **[x]** `TestFramesThatWouldNotFoldAreNotWritten` (VTT-320) crafts into
   the hero's log, through `store.AppendBatch` on a closed campaign, the end
   of a session and the goblin's departure while the world keeps both. The
   world's `SessionEnded` then gives the hero an introduction of the goblin
   and a `SessionEnded` its fold refuses. That set is dropped whole, the bard
   and the world get the event, and the next event about the goblin is
   written and folds. `TestADropThatLeftEveryActorHeldKeepsForwardingWhatTheCharacterSees`
   holds the restart after a drop that left nothing behind.
9. **[x]** `TestACharacterLogContinuesAfterReopen` (VTT-324, VTT-326) and
   `TestOpenRefusesACharacterLogThatDoesNotFold` (VTT-327).
   `TestWhatTheCampaignHoldsOfALogIsTheFoldOfThatLog` requires the memory the
   campaign holds to equal the fold of the stored log, live and after a
   reopen. The property walk requires each checkpoint's logs to stand
   unchanged as a prefix of the next checkpoint's.
10. **[x]** `.go-arch-lint.yml` lets `campaign` depend on `perceive`;
    `perceive`'s allowance is unchanged, and
    `TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight`
    and `check:arch` are green.
11. **[x]** `git diff --stat 4fe0472 77d75e0 -- scenarios/goldens` prints
    nothing, and `TestTheProjectedGoldensAreWhatTheProjectionActuallySends`
    passes.
12. **[x]** `task check`, whole, on `77d75e0`: started at 18:36 on
    2026-10-16 by a load guard, it printed `TASK_CHECK_EXIT=0` at 18:56 with
    no `task: Failed`. `check:mutation` reused, for all 15 packages, the
    verdicts of a run on the same source (17:31 to 18:03) that measured
    store, campaign, perceive, gateway and identity afresh: store 0 survivors, campaign
    0, gateway 1 and perceive 4, all adjudicated before this ticket, with
    perceive's `objectInSight` `y++` timed out as in ticket 1; "15 packages,
    zero unadjudicated survivors". `check:coverage`: "21 packages, all at or
    above their floors", store's floor now 94.0 and campaign's 92.5.
    `check:requirements-chain` read 328 rows.

## What the rules became

Every rule the sort accepted earned an id, allocated by the dispenser after
sign-off; the tests that hold each are in its row of `docs/requirements.md`.

- **VTT-315**, one transaction for the world event and its entries.
- **VTT-316**, written only while the actor is a party member after the event.
- **VTT-317**, the opening.
- **VTT-318**, the resume.
- **VTT-319**, the decision's frames and nothing else.
- **VTT-320**, frames that would not fold are not written.
- **VTT-321**, a log's own sequence from 1.
- **VTT-322**, an anchored narration names its own log's entries.
- **VTT-323**, every log folds at every prefix.
- **VTT-324**, a reopen introduces nothing again.
- **VTT-325**, a failed append changes no memory.
- **VTT-326**, an entry is never changed or removed.
- **VTT-327**, Open refuses a log that does not fold.
- **VTT-328**, an anchor spanning a resume names only what events at or
  after the latest resume in it caused: the owner's ruling at Task 6's
  start, read as QA's reading A, a new rule and a new id.

The thirteen rows VTT-315..327 were dispensed once, reworded after
`dad9d85`'s first review found three of them false or contradictory, and
dispensed again before that commit: removed as uncommitted rows and written
by the dispenser in the same order, so they kept their ids and no row was
edited by hand. VTT-328 was re-dispensed the same way within `136e6c8`, when
its review found its text saying "after" where the code and its tests count
the resuming event's own frames.

## The sort

Accepted: the ticket's thirteen rules, the plan's D11 (Open refuses a log
that does not fold) among them, as VTT-315..327; then VTT-328. Refused, one
line each:
- "An actor that is never a party member has no log": a consequence of
  VTT-316, held by the same tests.
- D10's "a dropped frame set is logged": no ticket item asks for it, and the
  count D10 adds is test apparatus.
- D3's entry identity (a forwarded entry keeps the world event's event_id,
  session_id and occurred_at; a synthesized one carries none): how an entry
  is stored. Its observable half is VTT-319's comparison, which reds when
  forwarded entries lose occurred_at and event_id.
- D1's and D4's shapes (one store write, one append path, `store.Append`
  deleted): how, not what.
- The first half of the ticket's reopen rule, "a character's memory is the
  fold of its log": the memory is internal state, seen only through a test
  hook; VTT-324 holds the observable half.
  `TestWhatTheCampaignHoldsOfALogIsTheFoldOfThatLog` holds the rest without
  a row.

## Phase 4a

Run twice, for the two tasks whose behaviour the plan names.

- **Task 4** (the fan-out, the opening, drops, failure and reopen): an agent
  given the rows, the ticket, the arc design and SPEC-016 whole, and the
  exported surface, and none of the code, diff or tests, wrote
  `internal/campaign/qa_character_log_test.go`. 15 tests (23 cases) pass and
  all 51 of its injections went red. It found no behaviour defect.
- **Task 6** (pause and resume): the same brief wrote
  `internal/campaign/qa_pause_resume_test.go`, 17 tests with 18 injections
  red. It found one behaviour defect, fixed before Task 6's commit (Task 6's
  QA finding 1 below).

Neither round was given a draft of the new specification, which the plan
names among QA's inputs: it was written after both (under Deviations).

`perceive.Resume`, added in answer to Task 4's review, arrived after Task 4's
QA and was not given to a QA agent. A reviewer's control walk (24 eventgen
walks against a never-closed campaign) called it 10,515 times and found the
logs identical to the control's, but it fell back 0 times, so the walk shows
only that a consistent fold is left alone. The fallback is held by
`TestAResumedProjectionSeesOnlyWhatItsViewerHoldsAsTheWorldDoes` and by the
campaign's drop tests, `TestFramesThatWouldNotFoldAreNotWritten`,
`TestAFailedAppendAfterADropDoesNotRepeatIt` and
`TestAReopenAfterADropDoesNotRepeatIt`.

### QA adjudications

Task 4's round:

- QA finding 1: spec ambiguity — VTT-320 has no natural trigger — forced through `store.AppendBatch` on a closed campaign, the way the plan's D13 chose; no change.
- QA finding 2: spec ambiguity — "memory" (VTT-325, the reopen rule) is not observable as stated — QA measured it against a never-failed control campaign; the specification step after this report defines the memory.
- QA finding 3: spec ambiguity — an opening entry's world sequence is unstated — it is 0 (plan D2); the specification step states it.
- QA finding 4: spec ambiguity — which facts of each party member an opening carries — the party as the decision introduces it; the specification step states it.
- QA finding 5: spec ambiguity — the order inside an opening — session, party, public notes by key; the specification step states it.
- QA finding 6: not this ticket's — `campaign.Append` overwrites a caller's session_id (SPEC-018's table) — no change.
- QA finding 7: not this ticket's — a refused Open releases the writer hold (SPEC-019) — no change.
- QA finding 8: spec ambiguity — whether `CharacterLog` returns copies — it reads the store fresh at every call; the specification step states it.

Task 6's round:

- QA finding 1: behaviour defect — VTT-328 failed whenever a resume wrote no entries of its own (an empty board with the session unchanged, or a removal whose batch had already hidden every token), live and after a reopen, because the boundary was taken from the resume's own entries — fixed in `136e6c8`: the boundary is the world event that resumed the log, recorded live and found on Open by an observer passed to the one fold.
- QA finding 2: spec ambiguity — whether the resuming event's own frames are "after the resume" (reading A) or the range starts at the next event (B) — A; VTT-328's text says "at or after".
- QA finding 3: spec ambiguity read through the owner's ruling — a public note upserted during a pause never reaches the resumed log, since only an opening carries public notes and a resume is not one — a consequence for the specification step.
- QA finding 4: spec gap — VTT-318 does not say whether a resume leaves a trace in the log — it need not: the boundary is the resuming world event, not an entry (finding 1's fix).
- QA finding 5: spec ambiguity — VTT-318's row says "and hide", the ticket "then hide" — session entries come first, then the hides; the specification step states it.
- QA finding 6: spec gap — sessions that open and close during a pause are not in the log; the resume's SessionStarted carries the world's name — the specification step states both.

QA asked for ids for, among others: the entry-identity ruling (refused at the
sort, above); the withdraw-before-introduce of an old id (held under VTT-318
and, in perceive, VTT-313); public notes at a resume (Task 6's finding 3).
None was allocated.

## The verification and Phase 4b

- **The ticket** was returned once (R1: item 4 compared against a spectator
  fed from event 1, which remembers party members as they were before the
  log opened — 10 of 86 logs differed; R2-R4: the anchor guard, a rule that
  contradicted the immutability rule, and a party member with no log when
  its campaign opens) and passed against those four after the owner's
  answers. Checking the resume rule found G1, answered (a).
- **Task 1** (`dad9d85`): 3 Must, 7 Should, 8 Nit, all fixed; three rows
  rewritten through the dispenser (above).
- **Task 2** (`0b81498`): 0 Must, 6 Should (the read and write orders unheld
  by a degenerate fixture, the id and time columns unobserved, among them),
  4 Nit, all fixed except N4, which asked that the plan-against-tree
  differences be recorded; they are, under Deviations.
- **Task 3** (`47ebac8`): the reviewer ran `4fe0472`'s `Append` and
  `AppendBatch` verbatim beside the new path in a white-box differential:
  every observable matched. 1 Must (a comment pointing at deleted proof),
  2 Should, 3 Nit.
- **Task 4** (`d680433`): 0 Must, 5 Should, of which S1 was a real defect.
  After a dropped frame set, the plan's rebuild after a failed write (D8) and
  its load (D11), both `perceive.New(eyes, c.state)`, re-claimed sight of
  actors the log never got, so the next event about them was introduced and
  forwarded and its duplicate refused: a failed write or a reopen repeated
  the drop. Fixed by a new `perceive.Resume` (under Deviations). A second
  read of the fixes: 0 Must, 2 Should (`Resume`'s incarnation check unheld;
  `Resume` needed a home in SPEC-016), 5 Nit.
- **Task 5** (`90943d9`): 1 Must (the range's upper end unpinned: every case
  ended its range at the last event before the narration), 3 Should, one of
  them the design question that became VTT-328, 4 Nit.
- **Task 6** (`136e6c8`): 2 Must. The clip searched for the latest resume
  below the range's last caused entry instead of the range's end, so a range
  ending at a resume that caused nothing named pre-pause entries (32 of
  18,014 anchored narrations in the reviewer's walk); and VTT-328's wording.
  1 Should (two equivalent guards dropped), 4 Nit.
- **Task 7** (`00b0cb7`): 1 Must. The walk's read-back read the same rows
  twice with nothing written between, so it held nothing of VTT-326; it now
  compares checkpoint with checkpoint. 3 Should, 1 Nit.
- **Task 8** (`b333a16`): 0 Must, 3 Should (the oracle never ran a batch,
  while production opens logs in batches; frames mislabelled as lead at a
  resume were swallowed into the seed; duplicated helpers), 4 Nit.
- **Task 9** (`77d75e0`): self-reviewed (one line of `fanout.go` edited,
  dropping the guard's second clause, and two floors raised).

## Rule 9: how MapTool does this

Checked against `~/dev/RPTool/maptool` at `f4b7fef6c`. Borrowed: the pause.
`Zone.exposeArea(Area, Token)` adds to a token's exposure history on a
personal server, or under individual fog when a player owns the token
(`AppUtil.playerOwns`), and `Zone.exposeArea(Area, Set<GUID>)` adds for a
token with sight when the caller is the GM, token management is not strict,
or the caller owns it. Nothing clears that history when ownership ends; only
explicit actions do. So under individual fog a player's memory of a token
pauses while they do not own it and resumes when they do, which is the shape
of the owner's ruling. The plan's sentence on this misreads the condition:
it says history grows only when `playerOwns` holds, which is false on a
personal server and when the GM exposes. Where MapTool does not help: its
memory is accumulated terrain, every client receives the whole campaign, and
so token positions are never part of a per-viewer memory that a resume must
correct. G1 is that correction.

## Figures the plan asked for

All measured during this period, on this machine, and true at the commits
named; none is a bound anyone must keep.

- The verifier's probes, in a scratch copy of `4fe0472`: an oracle seeded
  with each log's opening matched 86 logs over 8,424 events with 0
  mismatches and 0 fold failures; eventgen seeds 1-12 remove a party member 9
  times and never resume a log, and the corpus never pauses one.
- The cost of the decision per append, at load 2.5: about 5-8 ms per party
  member on a sparse 60×60 map and 33-35 ms on a dense or walled one, so
  about 20 ms and 135-142 ms for a party of four; a world `Snapshot` of three
  tiled 60×60 scenes about 0.5 ms.
- Suite runtimes: campaign under `-race` 8.6 s after Task 4 (3.3-4.6 s
  before); the campaign suite 4.3 s after Task 7 (1.2 s at `4fe0472`); the
  gateway's oracle test 2.7-2.8 s of a 72 s package.
- `check:mutation`: 34 minutes for the run that re-measured every changed
  package and failed on two survivors, 32 for the one that passed. The log
  prints no runtime per package.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| D5: after a dropped frame set, the decision restarts from nothing seen | `perceive.Resume(eyes, fold, world)` | Starting from nothing seen withholds the next event between two actors the log already holds, an attack among them: `TestADropThatLeftEveryActorHeldKeepsForwardingWhatTheCharacterSees` reds on it |
| D8 and D11: after a failed write and on Open, the decision restarts from the world (`perceive.New(eyes, c.state)`) | `perceive.Resume(eyes, fold, c.state)`, which sees as the world does unless an actor in sight is missing from the log's fold, another incarnation, or one a correction would change, and otherwise sees nothing yet | After a drop the fold no longer holds what a continuing decision sent, which is VTT-312's premise, so starting from the world repeated the drop after a failed write or a reopen (Task 4's review, S1). The reviewer's two options, a `Projection.Clone` and an unsaved flag, both left the reopen half open; `Resume` decides from the log itself |
| D6: `perceive` unchanged; the ticket's "What it touches" item 4: perceive only if the opening needs it | `perceive.Resume` added, with `internal/perceive/resume_from_fold_test.go` | The same finding |
| D9: anchors rewritten onto the log's caused entries | As planned, plus the resume boundary VTT-328 needs: the world event that resumed the log, recorded live and found on Open by `resumeScan`, an observer handed to `foldEvents` | VTT-328 came with the owner's ruling at Task 6's start, and Task 6's QA finding 1 showed a boundary taken from a resume's own entries vanishes when a resume writes none |
| D10: one test-only count | Also `CharacterFoldForTest` in `internal/campaign/export_test.go` | Task 4's review (N3): nothing compared the memory the campaign holds with the fold of the stored log, and the head after an opening went unpinned |
| D13: item 8 forced by writing a `ConditionApplied(x, c)` into a log and appending the world's `ConditionApplied(x, c)` | `droppedHero` writes `TokenHidden`, `ActorRemoved` and `SessionEnded` into the hero's log and appends the world's `SessionEnded` | D13's fixture dropped only a duplicate condition, so "skip the restart after a drop" survived; and once `Resume` existed, a crafted lag of an actor is caught at load and never drops, so the fixture moved to a lag of the session, which `Resume` does not check |
| The plan's tests first, for Task 2 | The store's code was written before its tests | The breaks are the evidence that each test tells working code from broken: four by the author, more by the reviewer, every one red |
| D1 and Task 2: a test helper per package for the old `store.Append` call sites | Only the store's tests have one (`appendOne`); identity and the five campaign tests write the one-envelope batch inline | One or two call sites each |
| Task 2's break 4, entries stamped before the world rows are validated | Replaced by "no bounds check on an entry's cause" | Validation runs before any stamping, so the planned break had nothing to land on |
| Task 3: no test file changes | Three test comments the merge made false were corrected, no test code | Task 3's review |
| Task 4: nine tests | Fourteen: the nine, plus `TestALogOpeningMidBatchIsTheWorldBeforeItsEvent`, `TestWhatTheCampaignHoldsOfALogIsTheFoldOfThatLog`, `TestAFailedAppendAfterADropDoesNotRepeatIt`, `TestAReopenAfterADropDoesNotRepeatIt` and `TestADropThatLeftEveryActorHeldKeepsForwardingWhatTheCharacterSees` | Task 4's two reviews |
| QA given the new specification's draft | Neither round was | The draft was written after both rounds, with the report |
| Task 5: four anchor cases | Six: a range in the same batch and one across a reopen | Each holds a break the four do not |
| Task 5's break 2, "include world-0 entries" | Cannot red; the break run is "the opening recorded as caused by its event" | World sequence 0 never satisfies `from >= 1`, which the engine forces on an anchor |
| Task 6's break 3, a resume decision from the world before, predicted red | Survives, equivalent | A paused actor has no eyes in the world before its resuming event |
| Task 7's guard pooled over seeds and corpus | Split by source | The corpus alone met the pooled guard, so it protected no seed sample (Task 7's review) |
| Task 7's break 3, predicted to red the non-empty clause | Reds the anchored and ended-open clauses | Every log still opens, so none is empty |
| Task 7: the runtime in the commit message | Not in `00b0cb7`'s message | Recorded here instead, under the figures above |
| Task 8: an oracle over "Task 6's histories" | One scripted pause-and-resume history in the gateway test; it, and the corpus, run in batches of one to four, eventgen in single appends | The campaign tests' histories are not reachable from `package gateway_test`; production opens logs in batches; eventgen's model must see each action accepted before it draws the next |
| Task 9's break 2, delete the failed-fan-out test and see a survivor | Not run | It needs a third mutation run; the zero-survivor run stands |
| Task 9: adjudicate survivors | Two equivalent survivors on reanchor's guard for an unanchored narration: the guard's second clause was removed instead | A range (0, 0) names no caused entry, so the rewrite leaves it unanchored without the guard |
| One Phase 4b review per task | Task 9 self-reviewed | One line and two numbers, each shown by the gate |

## What could not be established

- **The cost after the third ticket.** The verification measured about 35 ms
  per party member per append on a dense 60×60 map; the owner accepted it
  for now. With the gateway's projection still in place, every append pays
  both. Not measured again here.
- **A VTT-320 gap** (Task 8's review, N4): after a drop, `Resume` handed the
  world before the event instead of after passes every test. It could
  forward a status event about an actor that has just left sight, but only
  after a drop at an event that changes the character's sight, which no
  forged drop reached.
- **The failed-resume branch is unreachable** (Task 6's review, N4): each
  resume entry folds by construction. Were it reached, its pause would exist
  only in memory, and a reopen would resume the log without its bridge.
- **QA's `TestQAPauseARefusedResumeChangesNoMemory` and
  `TestQAPauseARefusedPauseChangesNoMemory`**, under VTT-325, are refused at
  validation, before the fan-out runs; the store-refused case is
  `TestAFailedFanOutDoesNotAdvanceACharactersMemory`'s and QA's
  `TestQALogAFailedFanOutDoesNotAdvanceACharactersMemory`'s.
- The VTT-320 gap and the unreachable branch above are recorded in
  `docs/verification-debt.md` under Open debt.

## What was deliberately left out, and where it went

- No seat reads a character log, `set_viewpoint` is unchanged, and the
  gateway's projection stays: the third ticket.
- The client and the harness folding every character's log: the fourth.
- A sight memo: refused for now by the owner (Q2).
- What a pause and a drop cost the log, for the specification's
  consequences: a public note made public, or a session opened and closed,
  during a pause never reaches the resumed log; a drop loses for good the
  narration, the public notes and the session events in its set; and a
  session the log holds open after a drop, which the world has ended, makes
  the log miss the next `SessionStarted` (one more drop) and file that
  session's events under the old one.
- `perceive.Resume` and its fallback rule, its callers in `internal/campaign`
  and `internal/perceive/resume_from_fold_test.go` go into SPEC-016, and the
  character log into a new SPEC-021, at the specification step after this
  report. SPEC-015's sentence naming the store's append callers is reread
  there, since its grep now finds the one call in `appendLocked`.
- The ticket's ruling that a late log opens with the present means a log
  holds no narration from before it opened: by the owner's ruling, not a gap.

## History the code no longer carries

Removed from comments under rule 10 in this period, and recorded here:

- `campaign.Append`'s doc: why the `EventsRetracted` guard could leave (the
  message left the contract on 2026-08-31); and the 32-line comment in its
  body on the clone-and-stamp validation — the world layer's P11 Task 3
  finding that an unstamped clone refused every anchored narration, the
  single-append twin of the F1/5c double-`SessionEnded` bug, and why
  write-only folds were never broken by it.
- `campaign.AppendBatch`'s doc and the comment in its body: the argument that
  the provisional sequences `head+1..head+N` equal what the store assigns.
- `stampSessionID`'s doc (the method is gone): the merge-gate decision of
  sub-project 4 and the hardening spec of 2026-07-24.
- `append_sequence_validation_test.go`'s 22-line header, the same history.
- `head_after_batch_test.go`'s header: gremlins' three surviving mutants on
  the batch's head arithmetic, and the P11 anchor bug.
- `cenv`'s default arm: the four add/remove pairs and six cases measured on
  2026-09-12.
- `store_failure_test.go`'s PROVENANCE paragraph: gremlins found the
  `ts != nil` guard surviving in `AppendBatch` while `Append`'s twin was
  killed; `occurredAt` is one function now.
- `store.go`'s `Notify` doc and three store-test doc blocks, cut to their
  warnings.
