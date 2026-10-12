# Plan: a gateway behaviour a recorded recipe breaks goes red in a test that names it

Ticket: `docs/superpowers/specs/2026-10-11-a-gateway-recipe-reds-a-test-that-names-it-design.md`.
Verified 2026-10-11 by an agent that did not write the ticket, on the tree at
`d64cc9c`. Verdict: **Passes with gaps**. The verifier wrote this file and
changed nothing else in the tree. The gaps are listed at the end, and the
questions for Patrik come before the tasks because their answers change what
the tasks carry.

Extended by the ticket's writer on 2026-10-11, after the owner answered the
four questions: the answers, D14, R9, X5, Task 2b and the lines that follow
from them are the writer's. The ticket gained Done item 12 by the owner's
ruling on Q2; its checks were run by the writer, which is a weaker check than
the verifier's.

## How the ticket was verified

| Check | How settled | Result |
|---|---|---|
| 1. Every path resolves | command | Every file the ticket names exists, each symbol was found by `grep` in the file the ticket gives for it, each of the six ids has a row, and `d64cc9c` is a commit. |
| 2. Done is an observation | command | All nine edits were applied in the verifier's own clone and the gateway package run under each (table below). Under no edit does a test in the file its Done item names go red today. The eight tests were then prototyped: each passes on the unbroken tree and each edit reds exactly its own. |
| 3. Each rule is breakable | reading, then a command per rule | Each of the nine rules has an observation that fails when it is broken: the test below that names it. Two halves have no break of their own (G2, G4). |
| 4. Scope matches the claim | command, then reading | Every function a recipe edits has one production caller, inside `internal/gateway` (`mayWorkDoor` has two, both entries of `playerRules`). No production file changes. The six test files, the register, the debt file, the ledger and three specifications are the whole of it, unless Q3 adds a fourth specification. (Task 2b later added `qa_move_reason_test.go`, on the owner's ruling.) |
| 5. No decision contradicted silently | reading | SPEC-008, -010, -014, -015, -016 and -017 were read whole; SPEC-009 and SPEC-013 at their door paragraphs, Status and Requirements, and by search. No new test pins anything a record states differently. Nothing is overturned. |
| 6. Records moved are named and reachable | command, then reading | The section is present and its three paths resolve. Evidence added to VTT-149, -168, -195, -201, -227 and -256 changes no specification's text: a `Requirements` list holds ids, and no specification's prose names one of the seven entries. The list is one short in a single case (G3, Q3). |

## What was run, and what was taken from the writer's log

Each edit was applied to a clone of `d64cc9c` made by the verifier, and
`go test -count=1 ./internal/gateway/...` run under it, once unless noted. The
writer's `cmd/vtt` runs were repeated too.

| Edit | The verifier's run | The writer's log |
|---|---|---|
| none | green | green |
| `mayWorkDoor`, y bound `<= 2` | green | green |
| `mayWorkDoor`, x bound `<= 2` | red: `TestAPlayerMayOnlyWorkADoorTheyAreNextTo` (`server_test.go`) | the same |
| `handleLoadMap`, `Ok: true` | first run red by one test that has nothing to do with the edit (F1); second run green; `go test ./cmd/vtt/ -run 'Map\|Art\|Load'` green both times | green |
| `wake` unbuffered | red: `TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause` (`server_visibility_test.go`) and `TestQAMoveReasonOverTheWireAPlayerIsNotSentBackItsOwnReason` (`qa_move_reason_test.go`) | red: the first, and `TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause` |
| `reperch`, a served shoulder answers nothing | red: `TestQANoteProjectionAPerchChangesNothingAboutNotes` (`qa_note_projection_test.go`), by its 30-second deadline | the same |
| `sameSet`, membership test off | red: the three `TestQATestimonySight...` tests (`qa_testimony_sight_test.go`); `go test ./cmd/vtt/...` green | the same |
| `Project`, `unrecognised` arm for players only | green | green |
| `handleRemoveActor`, the two assignments out | green | green |
| `handleJoinDoor`, refusal arm shuts the door | green | green |

Taken from the writer's log and not re-run: the eight-fold repeats of the
`wake` and `sameSet` edits ("six to eight runs of eight", "eight runs of
eight").

## What the tree showed that the ticket does not say

**F1. Seven tests in `qa_move_reason_test.go` read a connection in an order the
server does not promise, and one of them failed on a tree it has nothing to do
with.** `qaMRConn.command` reads frames until its result arrives and keeps what
it read. `qaMRStartLive` (after the perch) and `qaMRLive.barrier` (after the
narration) then wait on the same connection for a frame that command caused.
SPEC-007's ordering contract says a result is not ordered against the events
its command produced. When the frame comes first, `command` has already read
it, and the wait ends at the fixture's 30-second deadline.

- Observed: under the `handleLoadMap` edit, which no move-reason test reaches,
  `TestQAMoveReasonOverTheWireAHiddenMoveReachesNoPlayerOrSpectator` failed
  with `dm: timed out waiting for barrier 1`. The same edit was green on its
  second run.
- The mechanism, isolated: with `time.Sleep(50 * time.Millisecond)` added
  directly after `result := s.answerCommand(now, err, cmd, perches)` in
  `serve`'s read loop and nothing else changed,
  `go test -count=1 ./internal/gateway/` fails exactly the seven
  `TestQAMoveReasonOverTheWire...` tests, each with `spectator: timed out
  waiting for the near token placed after the perch`, and no other test. That
  is the fixture's first wait. The barrier's wait comes later in each test and
  has the same shape; it is the one the observed failure names, and it was
  read, not isolated.
- So a "stays green" observed once is weaker than it reads, and a gate run on
  this ticket can go red on one of those seven for a reason outside it (D11,
  Q2).

**F2. Which tests the `wake` edit reds varies by run.** The verifier's one
package run reddened a test the ticket does not list and left one it does list
green. The unlisted one failed with the message F1 also produces; run alone six
times under the edit it passed six times, so it is not attributed to the edit.
The closing paragraph for that entry names what was seen and does not call the
list complete (D9).

**F3. The ticket's second open point is settled by reading.**
`TestQANoteProjectionAPerchChangesNothingAboutNotes` perches on
`qanp-hero-a`, then `qanp-hero-b`, then nobody, then `qanp-hero-a` again, and
waits each time for a `SceneSeen` of the room concerned. Under the `reperch` edit the fourth
perch names a shoulder already served and is sent nothing, so the wait ends at
the deadline on every run. Two runs showed it, the writer's and the verifier's.

**F4. The ticket's first open point is settled by prototype.** None of the
eight tests is red on the unbroken tree: one run and then five more, and three
under the race detector. Every behaviour the ticket assumes is there.

**F5. `check:requirements-chain` passes a test that cites a row still marked
`OPEN`.** The prototype cited VTT-201 and VTT-227 with the register untouched
and the chain printed its clean line. `docs/verification-debt.md` already
records this. Done items 5 and 6 ("VTT-201's row names it") are therefore held
by Task 3's `grep`, not by the gate.

## Rule 9: how MapTool does this

No tabletop function is designed here. Each of the eight behaviours exists and
has its record; the ticket adds the tests that hold them and decides nothing
about geometry, coordinates or vocabulary. `~/dev/RPTool` was not searched.

## The constraints

These bind every task. They are named here, not restated.

- **CLAUDE.md rule 2.** `task check` is run whole on the final tree and is not
  weakened. A red from F1 is run again and recorded; no timeout is raised and
  no test is skipped.
- **CLAUDE.md rule 8.** The register, the debt file, the report and this plan
  cite tests, symbols and commits by name, never a line.
- **CLAUDE.md rule 10 and SPEC-010.** A new test's comment is a warning or a
  pointer. `check:comments` refuses a banned term on an added line, a block of
  more than six lines that the change added a line to, and a file fallen more
  than the band under its ceiling until `python3 tools/check-comments.py
  --write-ledger` lowers the row. The ledger only goes down.
- **`check:new-prose`.** Added comment lines in `.go` files are held to the
  wrap band and their citations must resolve.
- **SPEC-008.** An id comes from `requirement-id`, after sign-off, tried on a
  copy of the register first. A row's evidence is `path#Test`, the test file
  carries the row's id, and the citation is a `// VTT-NNN` line directly above
  `func Test...`. This plan holds no id that is not in the register today.
- **`check:doc-owner` does not read test files.** A new test goes after a
  closing brace, never between another test's doc block and its `func`, and
  never directly above another test's citation line.
- **`.go-arch-lint.yml`.** A test in `internal/gateway` imports only what the
  `gateway` component may: not `internal/store`, `internal/testdb` or
  `internal/harness`.
- **SPEC-020 and the mutation gate.** `internal/gateway`'s verdict is measured
  again because its test files are in its fingerprint. The four adjudications
  for the package sit on `abs`, `objectInSight`'s two loop bounds and
  `sortedSceneIDsUnion`'s capacity hint; no new test is aimed at any of them.
  An entry the run reports stale is judged again against its reason, never
  deleted on one run's word.
- **The ticket's own bound.** No non-test file under `internal/gateway`
  changes, and no other `Open debt` entry is touched.
- **Operational.**
  - Every break is made in a scratch clone, never in the working tree, and
    undone there with `git show HEAD:<file> > <file>`.
  - Nothing is stashed or checked out while a reviewer reads; the review
    package is `git diff HEAD`.
  - `git add` and `git commit` run in separate calls, with `git show --stat
    HEAD` after each commit.
  - The tree is not edited while a gate runs; `task check` is launched in its
    own process session, after reading the load with `os.getloadavg()`.
  - The ticket and every report under `docs/reports/` already committed are
    not edited.

## Decisions

**D1. Eight tests hold nine edits.** The two door bounds share one table test
with a row each. *Forced by:* Done item 1 names one test for both edits, and
each row was measured red under its own edit and green under the other.

**D2. The `wake` and hop-back tests stand at the box and at the seat, with no
connection.** *Forced by:* Done item 3 says "on every run" and Done item 4 says
"at once, not by a timeout". A connection test of either depends on where the
pump is when the perch arrives (F2) or ends at a deadline (F3).

**D3. The map joins the set at boot, through `WithMaps`.** `newMapFixture(t,
true)` hands the server the committed cellar map before it serves, so no first
`load_map` is needed and `mapByID` answers from the set. The art is then
overwritten in the fixture's own temp art directory with `installArt`. *Forced
by:* `mapByID` is reached only through `handleLoadMap`, but it is not the only
way into the set; SPEC-014 names `WithMaps` as the other.

**D4. "Nothing appended" is read as a catch-up head of 0** on a fresh DM
connection after the refusal, with `expectCatchUpHead`. *Forced by:*
`mapFixture` does not expose its campaign, the fixture seeds no event, and a
head is a positive read where "no event arrived within a wait" is not.
Changed by the reading review of C1: the test now seeds an actor, so it reads
the head before the load and requires the same head after it (Test 2).

**D5. The same-size change is `twoRooms` with the hero moved from (1,1) to
(4,1).** *Forced by a measurement:* from (1,1) and from (2,1) the hero sees the
same eight squares; from (4,1) and from (5,1) it sees eight others; the two
sets share the door square alone. A step inside a room changes nothing, and the
fold accepts a move through the shut door, as a DM's move is.

**D6. The same-size test checks its own fixture.** It fails with a fixture
message when the two sets differ in size or are equal, and it compares the
`SceneSeen` it was sent with the one a fresh projector is sent over the moved
state. *Forced by:* a set that grows or shrinks is caught by `sameSet`'s length
comparison, and the test would then pass under the edit.

**D7. The spectator's unknown-payload test carries a control.** After the
unknown payload it projects a recognised event and requires the shoulder's
`SceneSeen`. *Forced by a measurement:* an edit that runs `transitions` for a
spectator and discards its frames leaves the first assertion green and reds
only the control. The control is what holds "derives nothing".

**D8. The removal test runs once per issuer, DM and agent, each read by the
other.** *Forced by a measurement:* with the role stamped as a constant `dm`,
the DM's run is green and only the agent's is red.

**D9. A closing paragraph says what its test holds and what was seen.** For the
three entries whose edit already reds something, it names the tests seen red,
the commit that brought them (`a0c48c9` for the two issuer tests, `ce31229` for
the note-projection test, `390239a` for the three sight tests), and for `wake`
that the reds vary by run. *Forced by:* Done item 10 and F2.

**D10. Where each rule's citation goes.** Six tests cite ids the register
holds: VTT-149, VTT-168, VTT-195, VTT-201, VTT-227, VTT-256. Three rules have
no row and are named here by letter until Task 1 allocates:

- **Rule A:** a `load_map` whose map no longer compiles when it is loaded is
  refused and appends nothing.
- **Rule B:** a perch set while its connection's pump is not waiting for one is
  applied without another perch.
- **Rule C:** every event a `remove_actor` appends carries the participant id
  and role of whoever issued it.

The sort is the `requirements` skill's, after sign-off. The verifier's reading,
for it to use or overrule: A, B and C each get a row, under SPEC-014, SPEC-015
and SPEC-017; the hop back is VTT-195's (Q3).

**D11. F1 is mended here**, by the owner's ruling on Q2 (D14, Task 2b). Until
Task 2b is in the tree, a run that goes red on one of the seven tests with
`timed out waiting for` is run again, and the report records each occurrence
with its message. *Forced by:* the ruling, and rule 2.

**D12. One branch, three commits, one whole gate run.** *Forced by:* the last
arc's order (`53bccc6`, then `d64cc9c`), and `check:comments` and
`check:new-prose`, which read what a change adds against `main`: run after
commits made on `main` itself, they read nothing.

**D13. The breaks are made again on the committed tests.** Task 6 repeats all
nine in a scratch clone of the branch. *Forced by:* the measurements in this
plan are of the prototype text in Appendix A, and a test changed from it has
not been measured.

**D14. A wait starts at the first frame no wait has looked at, not at the
channel.** `qaMRConn` gains a cursor into `seen`. `until` looks from the cursor
through what `seen` already holds before it reads the channel, and leaves the
cursor after the frame it matched; `command` reads the channel for its result
and moves no cursor, so the frames it read past are still there for the next
wait. *Forced by:* SPEC-007's ordering contract, which lets a command's event
reach its issuer before the result; and by the fixture's own shape, whose
assertions read `seen` (`eventsAt`, `events`, `rawContains`), which this
leaves as it was, so what each of the seven asserts does not change.

## Questions for Patrik

**Answered 2026-10-11.** Q1: Phase 4a is skipped, on the owner's word; Task 7
does not run. Q2: the fixture is mended in this ticket, not recorded as debt;
the ticket gained Done item 12, and Appendix B's entry is not written. Q3: the
hop back cites VTT-195. Q4: the box-level test closes the `wake` entry, and
its paragraph says no connection test holds a busy pump.

- **Q1. Does Phase 4a run for this ticket?** The plan does not decide it.
  - *For:* each new test is shown to go red under one edit that was chosen
    before the test was written, which shows its power against that edit and
    nothing about its neighbours. An agent deriving tests from the six records
    alone, without the recipes, is the one reader who could find a ninth
    behaviour nothing holds; this verification found F1, which the move-reason
    arcs' reports do not mention.
  - *Against:* the ticket changes no behaviour, and QA derives tests to find
    where code departs from its record. QA already ran over these sections in
    the arcs that wrote them, and its tests are what three of the nine edits
    already redden. Phase 4b still reads every commit and can re-run any break.
- **Q2. Is F1 recorded now?** *Recommendation: yes, as one new `Open debt`
  entry in C1, with no fix.* The debt file is already in the commit and the
  recipe is in hand. The draft is in Appendix B. The alternative is a ticket of
  its own, or nothing.
- **Q3. Is the hop back to a shoulder VTT-195's, or a rule of its own?**
  *Recommendation: VTT-195's.* Under the edit a spectator perched on a shoulder
  does not see through it, which is that row's sentence. A row of its own would
  belong to SPEC-016, by SPEC-015's own words ("that a shoulder named again is
  served in full ... are SPEC-016's"), and SPEC-016 is not in the ticket's list
  of specifications moved; the writer would add it before C3.
- **Q4. Does a box-level test close the `wake` entry?** The entry's own closing
  sentence asks for a connection whose pump is busy delivering. Done item 3
  asks for the hand-off at the box, deterministic. *Recommendation: close it,
  and let the paragraph say that the test holds the box and no connection test
  holds the pump* (G2).

## The breaks

Each is an edit to one non-test file, made in a scratch clone.

| Name | The edit |
|---|---|
| R1y | In `mayWorkDoor`, `abs(tok.Y-at.GetY()) <= 1` becomes `<= 2`. |
| R1x | In `mayWorkDoor`, `abs(tok.X-at.GetX()) <= 1` becomes `<= 2`. |
| R2 | In `handleLoadMap`, the return under `mapdef.Compile`'s error answers `Ok: true`. |
| R3 | In `newPerchBox`, `make(chan struct{}, 1)` becomes `make(chan struct{})`. |
| R4 | `Projector` gains `served map[string]bool`; in `reperch`, after `pr.viewer.Viewpoint = actorID`, the map is made if nil, a shoulder already in it returns nil, and the shoulder is added. |
| R5 | In `sameSet`, `if !b[k] {` becomes `if !b[k] && false {`. |
| R6 | In `Project`, `if v == unrecognised {` becomes `if v == unrecognised && pr.viewer.Role == identity.RolePlayer {`. |
| R7 | In `handleRemoveActor`'s stamping loop, the `ParticipantId` and `ActorRole` assignments are removed (`_ = p` keeps it compiling). |
| R8 | In `handleJoinDoor`'s `default` arm, `_ = s.ids.SetJoinOpen(false, 0)` is added before the return. |

Two more for Done item 12, the writer's:

| Name | The edit | What it shows |
|---|---|---|
| R9 | In `serve`'s read loop, `time.Sleep(50 * time.Millisecond)` is added directly after `result := s.answerCommand(now, err, cmd, perches)`. | Before Task 2b the seven `TestQAMoveReasonOverTheWire...` tests fail under it; after, the package is green under it. |
| X5 | In `forwardable`, the `p.TokenMoved.Reason = ""` line is removed. | After Task 2b, tests among the seven still go red: the mended fixture tells a leaked reason from none. |

Four further edits were measured, one per assertion that the nine above do not
reach:

| Name | The edit | What goes red |
|---|---|---|
| X1 | R2's return keeps `Ok: false` and appends `" in " + s.artDir` to the error. | Test 2, at its path assertion. |
| X2 | The stamping loop writes `string(identity.RoleDM)` as the role. | Test 7's agent run alone. |
| X3 | In `Project`'s `unrecognised` arm, a spectator's `transitions` is run and its frames dropped. | Test 6, at its control. |
| X4 | The stamping loop keeps the role and drops the participant id. | Both runs of test 7. |

## The tests

All eight were prototyped in a clone and the text measured is Appendix A. On
the unbroken tree each passed in six runs. Under each of R1y to R8 exactly the
test named went red, in about two seconds, and the seven others stayed green.
With the prototypes in place, `go vet`, `golangci-lint run
./internal/gateway/...`, `go-arch-lint check`, `check-doc-owner.py`,
`check-requirements-chain.py` and `check-new-prose.py main` were clean, and
`check-comments.py main` was clean once the ledger was lowered (Task 5).

### Test 1: `TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis`

- **File and place:** `authz_test.go`, after the closing brace of
  `TestAuthorizePlayerMayNotWorkDiagonallyDistantDoor`.
- **Stands on:** `doorFixture`, `openDoorCmd`, `closeDoorCmd`,
  `gateway.Authorize`.
- **Asserts:** with the door at (3,3), a token at (4,4) may open it, which is
  the fixture's control; a token at (4,5) and a token at (5,4) may neither open
  nor close it.
- **Must go red under:** R1y, at the (4,5) row; R1x, at the (5,4) row.
- **Cites:** VTT-149. **Done item:** 1.

### Test 2: `TestAMapInTheSetWhoseArtStoppedCompilingIsRefused`

- **File and place:** `map_test.go`, after the closing brace of
  `TestAMapWithArtFromALaterFormatIsRefusedOnDemandExactlyAsAtBoot`.
- **Stands on:** `newMapFixture(t, true)`, `installArt`, `loadMapCmdFor`,
  `sendCommand`, `readResult`, `mapFixture.dial`, and `expectCatchUpHead` from
  `server_test.go`.
- **Asserts:** after `masonry-1`'s sidecar is overwritten with
  `format_version` 99, `load_map` of `cellar` answers ok=false; the error names
  `masonry-1`; it holds neither the art directory nor `os.TempDir()`; and a
  fresh DM connection's catch-up head is 0.
- **Must go red under:** R2, at the first assertion. X1 reds the path
  assertion.
- **Cites:** VTT-168 and rule A's id. **Done item:** 2.
- **Changed by the reading review of C1:** the test seeds `act-fighter`, which
  the cellar map's placement names, and compares the catch-up head before and
  after the refusal, where the prototype required 0; and it stands after the
  closing brace of `TestArtInstalledAfterBootDrawsWithoutARestart`, whose doc
  points at the test above it. Without the seed the fold refuses the load
  whatever the art. Appendix A holds the prototype as first measured.

### Test 3: `TestAPerchSetWhileNothingWaitsStillWakesThePump`

- **File and place:** `viewpoint_internal_test.go`, after the closing brace of
  `TestARapidHopIsCoalescedToTheShoulderItEndedOn`.
- **Stands on:** `newPerchBox`, `perchBox.set`, `perchBox.take`, and the `wake`
  channel read as the pump reads it.
- **Asserts:** after one `set` with nothing receiving, a receive from `wake`
  that does not block succeeds, and `take` then answers that shoulder.
- **Must go red under:** R3. It did in thirty runs of thirty.
- **Cites:** rule B's id. **Done item:** 3.

### Test 4: `TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt`

- **File and place:** `viewpoint_internal_test.go`, after the closing brace of
  `TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt`.
- **Stands on:** `newSeat`, `threeRoomLog`, `seat.receive`, `seat.perch`.
- **Asserts:** a spectator's seat perches on `a-a`, then on `a-b`, each sending
  something; the third perch, on `a-a` again, sends a `SceneSeen` of `r-a` with
  nine tiles and a `TokenPlaced` of `t-a-a`.
- **Must go red under:** R4, at once: the third perch returns nothing.
- **Cites:** VTT-195, by Q3. **Done item:** 4.

### Test 5: `TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain`

- **File and place:** `project_test.go`, after the closing brace of
  `TestASceneAlreadyReportedDarkIsNotReportedDarkAgain`.
- **Stands on:** `twoRooms`, `player`, `firstPlace`, `sceneSeenIn`,
  `mustApply`, `envelope`, `sameStrings`.
- **Asserts:** the hero's room is reported lit; after a `TokenMoved` of
  `t-hero` to (4,1), the projection of that event holds a `SceneSeen` of `s`;
  the two visible sets are one size and not equal (D6); and the `SceneSeen`
  equals the one a fresh projector is sent over the moved state.
- **Must go red under:** R5: no `SceneSeen` is sent.
- **Cites:** VTT-201. **Done item:** 5.

### Test 6: `TestAnUnrecognisedPayloadIsWithheldFromASpectator`

- **File and place:** `project_test.go`, after the closing brace of
  `TestAnUnrecognisedPayloadIsWithheldFromAPlayer`.
- **Stands on:** `twoRooms`, `firstPlace`, `sceneSeenIn`, and a spectator
  `gateway.Viewer` whose viewpoint is `hero`.
- **Asserts:** an envelope with no payload projects to no frame; the recognised
  event after it brings a `SceneSeen` of `s` (D7).
- **Must go red under:** R6: five frames are sent. X3 reds the control.
- **Cites:** VTT-227. **Done item:** 6.

### Test 7: `TestARemovalBatchCarriesItsIssuersParticipantAndRole`

- **File and place:** `remove_actor_test.go`, after the closing brace of
  `TestRemovingAnActorWithNoTokensYieldsABatchOfOne`. The file gains the
  import of `internal/identity`.
- **Stands on:** `newGWFixture`, `gwFixture.dial`, `gwFixture.head`,
  `gwFixture.playerIDFor`, `removeActorCmdFor`, `removalPayloadKind`,
  `sendCommand`, `readResult`, `readEvent`.
- **Asserts:** for the DM and then the agent as issuer, each on a fresh
  fixture, with the other of the two dialled at the head as reader: the removal
  of `a1` is accepted, and the reader's next two events are a `TokenRemoved`
  and an `ActorRemoved`, each carrying the issuer's participant id and role.
- **Must go red under:** R7. X2 and X4 red it too (D8).
- **Cites:** rule C's id. **Done item:** 7.

### Test 8: `TestAnUnspecifiedDoorLeavesAnOpenDoorOpen`

- **File and place:** `server_test.go`, after the closing brace of
  `TestAnUnspecifiedDoorIsRefusedRatherThanGuessedAt`.
- **Stands on:** `newGWFixture`, `gwFixture.dial`, `setDoor`,
  `identity.DB.JoinOpen` through `f.ids`.
- **Asserts:** the DM opens the door and `JoinOpen` answers true; an
  unspecified door is refused; `JoinOpen` still answers true.
- **Must go red under:** R8.
- **Cites:** VTT-256. **Done item:** 8.

## Tasks, in dependency order

### Task 0: branch and baselines

One branch for the ticket, cut from `main` at `d64cc9c`; the untracked ticket
and this plan travel with it. Read the load and `df -k`. Run `go test -count=1
./internal/gateway/...`, `python3 tools/check-requirements-chain.py .`,
`python3 tools/check-comments.py main`, `python3 tools/check-doc-owner.py .`
and both mutation self-tests.

*True when:* each prints its completion line and the gateway package is green.
A red that is one of F1's seven tests with `timed out waiting for` is run again
and noted for the report.

### Task 1: the sort, after sign-off

`dev-cycle:requirements`, over rules A, B and C and, if Q3 is answered "its
own", the hop back. The dispenser is tried on a copy of the register first.

*True when:* each rule the sort keeps has a row whose evidence is `OPEN`, the
ids are written down for Task 2, and the chain prints its clean line.

### Task 2: the eight tests

**Files:** `authz_test.go`, `map_test.go`, `viewpoint_internal_test.go`,
`project_test.go`, `remove_actor_test.go`, `server_test.go`, all under
`internal/gateway`. Each test as "The tests" states, at the place named, with
its citation line directly above `func`.

*True when:*
- `go test -count=1 -run` over the eight names passes;
- `git diff --stat d64cc9c -- internal/gateway` names those six files and,
  with Task 2b, `qa_move_reason_test.go`, and no other;
- `gofmt -l` prints none of the six, and `go vet ./internal/gateway/` is
  clean;
- each of the six existing ids, and each id Task 1 allocated, is on a citation
  line above the test "The tests" gives it.

### Task 2b: the move-reason fixture

**File:** `internal/gateway/qa_move_reason_test.go`: `qaMRConn`, `until` and
`command`, per D14, and a `read` the two now share. No test function in the
file changes. The change adds no comment: the file's ceiling in
`tools/comment-ceilings.txt` is 0.6 and `check:comments` refuses one added
comment line above it, so the cursor is named `lookedAt` and D14 holds the
reason.

*True when:*
- under R9, in a scratch clone, `go test -count=1 ./internal/gateway/` fails
  the seven tests before the change and is green after it;
- under X5 alone, tests among the seven go red after the change;
- on the unbroken tree `go test -count=3 -run TestQAMoveReason
  ./internal/gateway/` is green;
- `git diff d64cc9c -- internal/gateway/qa_move_reason_test.go` touches the
  struct, the two methods and `read` only.

### Task 3: the rows

**File:** `docs/requirements.md`. Each test joins the evidence of the row it
cites; VTT-201's and VTT-227's `OPEN` cells are replaced; each new row's `OPEN`
cell is replaced.

*True when:* for each of the eight names, `grep` finds
`internal/gateway/<file>#<name>` in the row of every id it cites; the rows of
VTT-201 and VTT-227 no longer hold `OPEN`; the chain prints its clean line.

### Task 4: the closings

**File:** `docs/verification-debt.md`. One `**Closed by**` paragraph directly
after each of the seven entries, per D9. The entry on the removal batch and the
join door names tests 7 and 8. No entry is added (Q2).

*True when:* `grep -c '^\*\*Closed by\*\*'` has risen by seven; each paragraph
names its test and file; no other entry's text has changed (`git diff` shows
additions only).

### Task 5: the ledger and the local gates

Run `python3 tools/check-comments.py main`; it refuses the files that fell more
than the band under their ceilings. Run `python3 tools/check-comments.py
--write-ledger`, then the gate again. Then `python3 tools/check-new-prose.py
main`, `python3 tools/check-doc-owner.py .`, the chain, `task lint`,
`go-arch-lint check`, `task check:invariants`, `go test -count=1
./internal/gateway/...` and both mutation self-tests.

*True when:* each prints its completion line; `git diff tools/comment-ceilings.txt`
shows rows of the six test files only, each lower. Two things printed on the
base are not this change's: `check:comments`' notice on
`cmd/vtt/library_test.go`, and `gofmt -l internal/gateway/` naming
`scenario_test.go`.

### Task 6: the breaks

In a scratch clone of the branch, R1y to R8 one at a time, each undone before
the next. For each, `go test -count=1 -v -run` over the eight names. Under R3,
test 3 once more with `-count=30`.

*True when:* under each edit the test "The tests" names is red and the seven
others green; test 3 is red thirty times of thirty; the clone is deleted and
the working tree's `git status` is what it was.

### Task 7: Phase 4a, if Q1 is answered yes

Not run: Q1 was answered no. What follows is what it would have been.

As the dev-cycle skill's Phase 4a states: one QA agent, given the six records
and `go doc ./internal/gateway`, not the source, the recipes or this plan's
test list. Its brief names
`.go-arch-lint.yml`. Its tests go in a `qa_` file of their own, which C1 then
carries with a ledger row; its adjudications go in the report, one line per
finding. Findings are fixed and Tasks 5 and 6 run again for what changed.

### Task 8: Phase 4b, then C1

The reviewer is given `git diff HEAD`, the ticket and this plan, and re-runs
the breaks it chooses to. Findings are settled, the review is recorded, and C1
is committed.

*True when:* `git show --stat HEAD` lists the ticket, this plan, the six test
files, `internal/gateway/qa_move_reason_test.go`, `docs/requirements.md`,
`docs/verification-debt.md` and `tools/comment-ceilings.txt`, and no non-test
file under `internal/gateway`.

### Task 9: the report, then C2

`docs/reports/<date>-a-gateway-recipe-reds-a-test-that-names-it.md`, per the
`implementation-report` skill: each Done item with its observation, the sort,
the breaks with their outputs, F1's occurrences, the answers to Q1 to Q4, Phase
4a's adjudications if it ran, and the gaps below. Its own reading review, then
C2.

*True when:* C2 holds the report alone.

### Task 10: the specifications, then C3

SPEC-014, SPEC-015 and SPEC-017 each gain, at the end of their `Requirements`
list, the id Task 1 allocated under them; SPEC-016 too if Q3 is answered "its
own" and the ticket names it. Its own reading review, then C3.

*True when:* C3's diff of `docs/specifications`, read with `-U0`, shows changes
inside `Requirements` lists only, and the chain prints its clean line.

### Task 11: the gate, the push, the merge

`task check` whole, on C3's tree. Then the push, whose hook runs for about
three minutes. Merging is Patrik's call; the branch is deleted locally and on
the remote with the merge.

*True when:* `task check` exits 0 with every step's own completion line,
`check:mutation` included (Done item 11), and Done item 9's `git diff --stat
d64cc9c` names no non-test file under `internal/gateway`.

## Commits

| Commit | Carries | Held by |
|---|---|---|
| C1 | the ticket, this plan, the six test files, `internal/gateway/qa_move_reason_test.go`, `docs/requirements.md`, `docs/verification-debt.md`, `tools/comment-ceilings.txt` | Tasks 5 and 6, Phase 4b, the pre-commit hook |
| C2 | the report | its reading review, the hook |
| C3 | the `Requirements` lists | its reading review, the hook |

`task check` runs once, whole, after C3. The pre-commit hook does not run the
gateway package (`test:unit` leaves it out); the pre-push hook does.

## Gaps that travel with this plan

- **G1. Seven existing tests can fail on a tree that is right** (F1). Task 2b
  mends the fixture (D14); R9 shows the one order that was isolated, and the
  barrier's wait has the same shape and is covered by the same change. Whether
  another fixture in the package has the fault under a longer delay than R9's
  was not measured.
- **G2. Rule B speaks of a connection's pump, and test 3 observes the box.**
  That the pump takes a waiting wake-up and applies it is held by the
  connection tests that perch at an idle table. No test holds a pump that is
  busy when the perch is set, which is the case the entry describes (Q4). The
  sort words the row with that in view.
- **G3. The ticket's list of specifications moved is right unless the hop back
  gets a row of its own.** Then SPEC-016 gains the id and the ticket does not
  name it (Q3).
- **G4. "Appends nothing" in rule A has no break of its own.**
  `mapdef.Compile` returns no envelope with its error, so no one-line edit of
  `handleLoadMap` appends on that path. Test 2 reads the head because the
  rule's sentence says it.
- **G5. The `wake` edit's reds are a sample.** Two tests were seen red by the
  writer and one of them by the verifier; a third seen red once is not
  attributed (F2). The closing paragraph says so.
- **G6. Two of the six rows are held by a `grep`.** The chain does not notice
  an `OPEN` row that a test cites (F5, Task 3).
- **G7. What the mutation gate will say of `internal/gateway` is not known
  before it runs.** No adjudication is expected to go stale; none was measured.
- **G8. The measurements are of Appendix A's text on `d64cc9c`.** Task 6
  measures what is committed.
- **G9. Which of rules A, B and C the sort keeps is open until Task 1.** A rule
  it folds into an existing row moves no specification, and Task 10 shrinks
  with it.

## Appendix A: the eight tests as prototyped

The measurements in this plan are of this text. Where a line reads `// (rule
A's id)`, Task 2 writes the id Task 1 allocated as a citation line in its
place; where an id stands beside it, both go on one line.

`authz_test.go`:

```go
// TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis holds each bound
// from above on its own. Keep the other delta at 1 in every row: at 2 on both
// axes either bound alone refuses.
// VTT-149
func TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis(t *testing.T) {
	p := &identity.Participant{ID: "p-1", Role: identity.RolePlayer}
	if err := gateway.Authorize(p, openDoorCmd("scn", 3, 3), doorFixture(4, 4)); err != nil {
		t.Fatalf("fixture: a token one square off on each axis must be allowed: %v", err)
	}
	for _, c := range []struct {
		name       string
		tokX, tokY int32
	}{
		{"dx 1, dy 2", 4, 5},
		{"dx 2, dy 1", 5, 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := doorFixture(c.tokX, c.tokY)
			if err := gateway.Authorize(p, openDoorCmd("scn", 3, 3), st); err == nil {
				t.Fatalf("opening the door at (3,3) from (%d,%d) was allowed", c.tokX, c.tokY)
			}
			if err := gateway.Authorize(p, closeDoorCmd("scn", 3, 3), st); err == nil {
				t.Fatalf("closing the door at (3,3) from (%d,%d) was allowed", c.tokX, c.tokY)
			}
		})
	}
}
```

`map_test.go`:

```go
// TestAMapInTheSetWhoseArtStoppedCompilingIsRefused drives handleLoadMap's own
// compile, past a lookup the set answers (SPEC-014). Break the art after the
// map joined the set: broken before, LoadInstalled refuses the map first.
// VTT-168 (rule A's id)
func TestAMapInTheSetWhoseArtStoppedCompilingIsRefused(t *testing.T) {
	f := newMapFixture(t, true)
	conn := f.dial(f.dmToken, 0)

	installArt(t, f.artDir, "masonry-1", `{"format_version":99,"kind":"wall"}`)

	sendCommand(t, conn, loadMapCmdFor("cellar"))
	res := readResult(t, conn)
	if res.GetOk() {
		t.Fatal("a map in the set loaded although its art declares a later format")
	}
	if !strings.Contains(res.GetError(), "masonry-1") {
		t.Errorf("error = %q, want it to name the art it refused", res.GetError())
	}
	if strings.Contains(res.GetError(), f.artDir) || strings.Contains(res.GetError(), os.TempDir()) {
		t.Errorf("a client was told where the campaign lives:\n  %s", res.GetError())
	}
	if head := expectCatchUpHead(t, f.dial(f.dmToken, 0)); head != 0 {
		t.Fatalf("log head = %d after a refused load, want 0: a refusal appends nothing", head)
	}
}
```

`viewpoint_internal_test.go`:

```go
// TestAPerchSetWhileNothingWaitsStillWakesThePump holds wake's capacity
// (SPEC-015). Receive from wake without blocking: a blocking receive hangs
// where this must fail.
// (rule B's id)
func TestAPerchSetWhileNothingWaitsStillWakesThePump(t *testing.T) {
	b := newPerchBox()
	b.set("hero")

	select {
	case <-b.wake:
	default:
		t.Fatal("a perch set while nothing waited on wake left no wake-up for the pump")
	}
	if got, ok := b.take(); !ok || got != "hero" {
		t.Fatalf("the wake-up must hand the pump the shoulder set, got %q (ok=%v)", got, ok)
	}
}
```

```go
// TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt holds the hop back to a
// shoulder the projector's memory did hold (SPEC-016). Keep the hop away
// between the two perches: without it the second has nothing to send.
// VTT-195
func TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt(t *testing.T) {
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	for _, env := range threeRoomLog() {
		s.receive(env)
	}
	if len(s.perch("a-a")) == 0 {
		t.Fatal("the watcher must be on the first shoulder before they can leave it")
	}
	if len(s.perch("a-b")) == 0 {
		t.Fatal("the watcher must have hopped away before they can hop back")
	}

	var seen *vttv1.SceneSeen
	var placed bool
	for _, e := range s.perch("a-a") {
		if ss := e.GetSceneSeen(); ss != nil && ss.GetSceneId() == "r-a" {
			seen = ss
		}
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-a-a" {
			placed = true
		}
	}
	if got := len(seen.GetTiles()); got != 9 {
		t.Fatalf("hopping back must light the first shoulder's room again, got %d of 9 tiles", got)
	}
	if !placed {
		t.Fatal("hopping back must place the first shoulder's token again")
	}
}
```

`project_test.go`:

```go
// TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain holds sameSet's
// membership test. Keep the two sets the same size, and keep the check that
// they are: a set that grows or shrinks is caught by the lengths alone.
// VTT-201
func TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(player())
	before := sceneSeenIn(firstPlace(pr, st), "s")
	if before == nil {
		t.Fatal("the control fails: the hero's room must be reported lit first")
	}

	move := &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 4, Y: 1}}
	mustApply(st, 8, move)
	after := sceneSeenIn(pr.Project(envelope(8, move), st), "s")
	if after == nil {
		t.Fatal("the squares the hero sees changed to as many others and no SceneSeen was sent")
	}
	if len(after.GetVisible()) != len(before.GetVisible()) || sameStrings(after.GetVisible(), before.GetVisible()) {
		t.Fatalf("fixture: want two different sets of one size, got %v then %v",
			before.GetVisible(), after.GetVisible())
	}
	want := sceneSeenIn(firstPlace(gateway.NewProjector(player()), st), "s")
	if !proto.Equal(after, want) {
		t.Errorf("the SceneSeen carries %v, want the whole set a fresh look gives, %v",
			after.GetVisible(), want.GetVisible())
	}
}
```

```go
// TestAnUnrecognisedPayloadIsWithheldFromASpectator is the spectator's half of
// the test above. Keep the control: a fixture with nothing to send passes the
// first check whatever Project does.
// VTT-227
func TestAnUnrecognisedPayloadIsWithheldFromASpectator(t *testing.T) {
	st := twoRooms()
	pr := gateway.NewProjector(gateway.Viewer{
		ParticipantID: "sp-1", Role: identity.RoleSpectator, Viewpoint: "hero"})
	if out := pr.Project(&vttv1.Envelope{Sequence: 99}, st); len(out) != 0 {
		t.Fatalf("an unknown payload must be withheld from a spectator, got %d frames", len(out))
	}
	if sceneSeenIn(firstPlace(pr, st), "s") == nil {
		t.Fatal("the control fails: the next recognised event must bring the shoulder's board")
	}
}
```

`remove_actor_test.go`:

```go
// TestARemovalBatchCarriesItsIssuersParticipantAndRole reads the batch off an
// unprojected seat, which is sent the log as it is. Keep both issuers: with
// one, a batch stamped with a fixed role passes.
// (rule C's id)
func TestARemovalBatchCarriesItsIssuersParticipantAndRole(t *testing.T) {
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		t.Run(string(role), func(t *testing.T) {
			f := newGWFixture(t)
			issuerToken, readerToken := f.dmToken, f.agentToken
			if role == identity.RoleAgent {
				issuerToken, readerToken = f.agentToken, f.dmToken
			}
			reader := f.dial(readerToken, f.head(t))
			issuer := f.dial(issuerToken, f.head(t))

			sendCommand(t, issuer, removeActorCmdFor("r-remove", "a1"))
			if res := readResult(t, issuer); !res.Ok {
				t.Fatalf("want ok=true removing actor a1, got %+v", res)
			}
			wantID := f.playerIDFor(t, issuerToken)
			for i, kind := range []string{"tokenRemoved", "actorRemoved"} {
				env := readEvent(t, reader)
				if got := removalPayloadKind(env); got != kind {
					t.Fatalf("batch envelope %d kind = %q, want %q", i, got, kind)
				}
				if env.GetParticipantId() != wantID || env.GetActorRole() != string(role) {
					t.Fatalf("batch envelope %d carries participant %q and role %q, want %q and %q",
						i, env.GetParticipantId(), env.GetActorRole(), wantID, role)
				}
			}
		})
	}
}
```

`server_test.go`:

```go
// TestAnUnspecifiedDoorLeavesAnOpenDoorOpen starts from an open door. Keep it
// so: from a shut door, a refusal that shuts the door changes nothing this can
// read.
// VTT-256
func TestAnUnspecifiedDoorLeavesAnOpenDoorOpen(t *testing.T) {
	f := newGWFixture(t)
	dm := f.dial(f.dmToken, 0)
	if res := setDoor(t, dm, vttv1.JoinDoor_JOIN_DOOR_OPEN); !res.GetOk() {
		t.Fatalf("setup: the DM could not open the door: %s", res.GetError())
	}
	if !f.ids.JoinOpen() {
		t.Fatal("setup: the door must stand open before the refusal")
	}
	if res := setDoor(t, dm, vttv1.JoinDoor_JOIN_DOOR_UNSPECIFIED); res.GetOk() {
		t.Fatal("an unspecified door must be refused, not resolved to a default")
	}
	if !f.ids.JoinOpen() {
		t.Fatal("a refused set_join_door shut a door that stood open")
	}
}
```

## Appendix B: the entry Q2 asked about, as drafted

Not written into `docs/verification-debt.md`: the owner ruled the fixture
mended in this ticket. Kept as the verifier's account of the fault.

```markdown
**The move-reason QA fixture waits for a frame it may already have read.**
`qaMRConn.command` in `internal/gateway/qa_move_reason_test.go` reads frames
until its result arrives and keeps them; `qaMRStartLive`, after its perch, and
`qaMRLive.barrier`, after its narration, then wait on the same connection for a
frame that command caused. SPEC-007's ordering contract leaves a result
unordered against its command's events, so when the frame arrives first the
wait ends at the fixture's 30-second deadline. Recipe: in `serve`'s read loop
in `internal/gateway/server.go`, add `time.Sleep(50 * time.Millisecond)`
directly after `result := s.answerCommand(now, err, cmd, perches)`;
`go test -count=1 ./internal/gateway/` then fails the seven
`TestQAMoveReasonOverTheWire...` tests and no other (observed 2026-10-11 on
`d64cc9c`). On the unbroken tree
`TestQAMoveReasonOverTheWireAHiddenMoveReachesNoPlayerOrSpectator` failed once
with `dm: timed out waiting for barrier 1`, in a package run on that date. No
gate runs a test against a reordered connection. Labels: `outside the tool`.
Closing it needs the fixture to search what it has read before it waits, or to
sort results from events as `frameQueue` in `server_test.go` does. Recorded
2026-10-11 by the verification of
`docs/superpowers/specs/2026-10-11-a-gateway-recipe-reds-a-test-that-names-it-design.md`.
```
