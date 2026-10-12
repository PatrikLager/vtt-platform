# A gateway behaviour a recorded recipe breaks goes red in a test that names it: the change

**Ticket:** `docs/superpowers/specs/2026-10-11-a-gateway-recipe-reds-a-test-that-names-it-design.md`,
extended by its writer after verification, on the owner's ruling (Done item
12).
**Plan:** `docs/superpowers/plans/2026-10-11-a-gateway-recipe-reds-a-test-that-names-it.md`,
written by `verify-ticket` (passes with gaps) and extended by the ticket's
writer with the owner's answers; the plan names which parts are whose.
**The owner's rulings:** before the ticket was written, a direct test for
each of the three recipes that were already red, and one ticket although it
names more than ten files; at sign-off, Phase 4a skipped (Q1), the move-reason
fixture mended in this ticket (Q2), and the box-level test closing the `wake`
entry (Q4), with the hop back citing VTT-195 as proposed (Q3); after the
reading review, every finding fixed but VTT-256's wording, which stands;
after this report's review, every finding fixed, one sentence of
`docs/verification-debt.md` among them, in this report's commit.
**Last code commit:** `b0e4172`, on `d64cc9c`, `main` at the time. Every code
reference below is to `b0e4172`'s tree.

## The period, in commits

    git log --oneline d64cc9c..b0e4172

    b0e4172 Seven gateway recipes each red a test that names what they break

    git diff --stat d64cc9c..b0e4172

     docs/requirements.md                               |  15 +-
     ...1-a-gateway-recipe-reds-a-test-that-names-it.md | 891 +++++++++++++++++++++
     ...eway-recipe-reds-a-test-that-names-it-design.md | 160 ++++
     docs/verification-debt.md                          |  50 ++
     internal/gateway/authz_test.go                     |  28 +
     internal/gateway/map_test.go                       |  39 +
     internal/gateway/project_test.go                   |  46 ++
     internal/gateway/qa_move_reason_test.go            |  57 +-
     internal/gateway/remove_actor_test.go              |  35 +
     internal/gateway/server_test.go                    |  21 +
     internal/gateway/viewpoint_internal_test.go        |  52 ++
     tools/comment-ceilings.txt                         |  12 +-
     12 files changed, 1373 insertions(+), 33 deletions(-)

This report, and the commit that adds three ids to three specifications'
`Requirements` lists, follow.

## What the recipes did on `d64cc9c`, before any test was written

The ticket's subject is seven entries under `Open debt` in
`docs/verification-debt.md`, each saying an edit to `internal/gateway` leaves
the package green. Each edit was applied to a clone of `d64cc9c` and `go test
-count=1 ./internal/gateway/...` run under it, once (Appendix A, `recipes.py`
and its log).

| Entry | Under its recipe on `d64cc9c` |
|---|---|
| `mayWorkDoor`'s y bound | green; the x bound reds `TestAPlayerMayOnlyWorkADoorTheyAreNextTo` alone |
| `handleLoadMap`'s `Compile` refusal arm | green, and `go test ./cmd/vtt/ -run 'Map\|Art\|Load'` green |
| `perchBox.wake`'s capacity | red: `TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause`, `TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause` |
| a shoulder sat on and left | red: `TestQANoteProjectionAPerchChangesNothingAboutNotes`, after 30 seconds |
| `sameSet`'s membership test | red: three `TestQATestimonySight...` tests; `go test ./cmd/vtt/...` green |
| an unknown payload and a spectator | green |
| a removal's stamping; a refused door | green, both |

So four entries held as written and three did not: tests that arrived after
those three were recorded already reddened under their recipes, and none of
them states the behaviour the recipe breaks. Repeated eight times each
(`repeat.py`), the `wake` recipe reddened its two tests in six to eight runs of
eight and the `sameSet` recipe its three in eight of eight. Its output was
not kept in a file; the first reviewer's repeat on `d64cc9c` printed seven and
eight of eight for the `wake` recipe's two tests and eight of eight for each of
the `sameSet` recipe's three. The verifier's own
package run under the `wake` recipe left the second of those two tests green
and reddened a move-reason test instead. The owner ruled that all three still
get a test of their own.

The two edits of the entry on the busy skip's wiring in `serve`, which is not
this ticket's, were run in the same pass and both left the package green; that
entry stands in `Open debt` unchanged.

## Done looks like, answered

The observations for items 1 to 9, and for item 12 where it names
`b0e4172`'s fixture, were made in a clone holding `b0e4172`'s seven test files
byte for byte (`cmp` against the commit), with one non-test file edited at a
time. Items 10 and 11 are read off the repository and the gate's log. The
edits are named as the plan names them and listed under "The breaks".

1. `[x]` **A test in `authz_test.go` reds under each door bound made `<= 2`.**
   `TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis`. Under R1y it
   fails at its row `dx 1, dy 2` with `opening the door at (3,3) from (4,5) was
   allowed`; under R1x at `dx 2, dy 1` with `opening the door at (3,3) from
   (5,4) was allowed`. Before it, R1y reddened nothing and R1x no test in that
   file (the table above).
2. `[x]` **A test in `map_test.go` reds when the `Compile` refusal arm answers
   ok.** `TestAMapInTheSetWhoseArtStoppedCompilingIsRefused`. It boots with the
   cellar map in the set, seeds the actor the map places, overwrites
   `masonry-1`'s sidecar with `format_version` 99, loads the map, and asserts
   ok=false, `masonry-1` named, neither the art directory nor `os.TempDir()` in
   the error, and the same catch-up head before and after. Under R2 it fails
   with `a map in the set loaded although its art declares a later format`.
   Under X1 it fails at the path assertion, and under X6 at the first. The
   reading review changed this test (Deviations).
3. `[x]` **A test in `viewpoint_internal_test.go` reds on every run with `wake`
   unbuffered.** `TestAPerchSetWhileNothingWaitsStillWakesThePump`. Under R3
   it fails with `a perch set while nothing waited on wake left no wake-up for
   the pump`, thirty runs of thirty (`go test -count=30`). It observes the
   box, not a connection (What could not be established).
4. `[x]` **A test reds at once when `reperch` answers nothing for a shoulder
   served before.** `TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt` in
   `viewpoint_internal_test.go`. Under R4 it fails with `hopping back must
   light the first shoulder's room again, got 0 of 9 tiles`. The test waits
   on nothing, so it fails as soon as the third perch returns.
5. `[x]` **A test in `project_test.go` carrying VTT-201 reds with `sameSet`'s
   membership test off, and VTT-201's row names it.**
   `TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain`. Under R5 it
   fails with `the squares the hero sees changed to as many others and no
   SceneSeen was sent`. `grep '^| VTT-201 ' docs/requirements.md` prints the
   row with `internal/gateway/project_test.go#` and that name as its evidence.
   The chain gate does not hold this half: it passes a test that cites a row
   still marked `OPEN`, which `docs/verification-debt.md` already records.
6. `[x]` **A test in `project_test.go` carrying VTT-227 reds with the
   `unrecognised` arm narrowed to players, and VTT-227's row names it.**
   `TestAnUnrecognisedPayloadIsWithheldFromASpectator`. Under R6 it fails with
   `an unknown payload must be withheld from a spectator, got 5 frames`; under
   X3 at its control. `grep '^| VTT-227 ' docs/requirements.md` prints the row
   with that name as its evidence.
7. `[x]` **A test in `remove_actor_test.go` reds with the two assignments out
   of the stamping loop.** `TestARemovalBatchCarriesItsIssuersParticipantAndRole`.
   Under R7 it fails with `batch envelope 0 carries participant "" and role
   ""`; under X2 in its agent run alone, and under X4 in both runs.
8. `[x]` **A test in `server_test.go` reds when the refusal arm shuts the
   door, from an open door.** `TestAnUnspecifiedDoorLeavesAnOpenDoorOpen`.
   Under R8 it fails with `a refused set_join_door shut a door that stood
   open`.
9. `[x]` **Every new test passes unbroken, and no non-test file under
   `internal/gateway` changed.** `go test -count=1 -v -run` over the eight
   names prints eight top-level `--- PASS` lines (`grep -c '^--- PASS'`; four
   more are subtests), before the first break and after the last. `git diff
   --stat d64cc9c b0e4172 -- internal/gateway` names seven files, each ending
   `_test.go`.
10. `[x]` **Each of the seven entries carries a `**Closed by**` paragraph.**
    `grep -c '^\*\*Closed by\*\*' docs/verification-debt.md` prints 15 on
    `b0e4172` and 8 on `d64cc9c`, and `git diff d64cc9c b0e4172 --
    docs/verification-debt.md` holds no removed line. The three whose recipe
    already reddened name the tests and the commit that brought them
    (`904f79b` and `a0c48c9`, `ce31229`, `390239a`).
11. `[x]` **`task check` whole is green.** Run on `b0e4172` with a clean
    tree: the log ends `TASK_CHECK_EXIT=0` and holds no `task: Failed` line.
    `check:mutation` measured `internal/gateway` again, the one package whose
    test closure changed, and printed `ok  internal/gateway: 4 survivor(s), 4
    adjudicated`, as many as `tools/mutation-equivalents.txt` held for it on
    `d64cc9c`; thirteen packages reused their verdicts, and its last line is
    `check:mutation: 14 packages, zero unadjudicated survivors.`
    `check:ts-mutation` verified its stored report, the client being unchanged.
12. `[x]` **Under a 50 ms sleep in `serve` the package is green, and the seven
    move-reason tests still tell a leaked reason from none.** Under R9 with
    `d64cc9c`'s `qa_move_reason_test.go`, `go test -count=1 -v -run
    TestQAMoveReasonOverTheWire ./internal/gateway/` fails all seven; with
    `b0e4172`'s, `go test -count=1 -v ./internal/gateway/` passes 514 top-level
    tests and fails none.
    Under X5 alone, three of the seven fail
    (`...APlayerCatchingUpIsSentNoReason`,
    `...APlayerIsNotSentBackItsOwnReason`,
    `...PlayerAndSpectatorDoNotHearAnAgentsReason`) and four pass.

## What the rules became

The ticket names six existing rules it adds evidence to. Each row's evidence
now holds the test named:

| Id | Test added to its evidence |
|---|---|
| VTT-149 | `TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis` |
| VTT-168 | `TestAMapInTheSetWhoseArtStoppedCompilingIsRefused` |
| VTT-195 | `TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt` |
| VTT-201 | `TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain`, where the cell read `OPEN` |
| VTT-227 | `TestAnUnrecognisedPayloadIsWithheldFromASpectator`, where the cell read `OPEN` |
| VTT-256 | `TestAnUnspecifiedDoorLeavesAnOpenDoorOpen` |

The three rules no row stated each earned an id from the dispenser:

| The ticket's rule | Id | The row's sentence | Held by |
|---|---|---|---|
| A `load_map` whose map no longer compiles when it is loaded is refused and appends nothing. | VTT-305 | A `load_map` of a map the server already holds is refused when the map no longer compiles. | `TestAMapInTheSetWhoseArtStoppedCompilingIsRefused` |
| A perch set while its connection's pump is not waiting for one is applied without another perch. | VTT-306 | A perch set while the pump is not waiting for one is still handed to the pump. | `TestAPerchSetWhileNothingWaitsStillWakesThePump` |
| Every event a `remove_actor` appends carries the participant id and role of whoever issued it. | VTT-307 | Every event a `remove_actor` appends carries the participant id and the role of whoever issued it. | `TestARemovalBatchCarriesItsIssuersParticipantAndRole` |

VTT-306's sentence stops at the hand-off because that is what its test
observes; the ticket's wording reached the connection, which no test holds.

## The sort

Run after sign-off, over the ticket's three unstated rules and the candidates
the work raised. Accepted, each with the observation that goes red: the three
rows above. Refused:

- "…and appends nothing", the second half of the map rule: VTT-162 states it
  for every refused command.
- The hop back to a shoulder as a rule of its own: it is VTT-195. Under R4 a
  spectator perched on a shoulder does not see through it.
- "A refused `set_join_door` leaves the door as it stood": it is VTT-256, whose
  older test already reads the door after the refusal under that id.
- "`wake` has capacity one": how, not what. VTT-306 says what.
- "A fixture's wait starts at what was read": nothing about the system; no id.

## Phase 4a

Not run, on the owner's word at sign-off (Q1). The change adds tests and
changes no behaviour, each new test is shown red under an edit written before
the test existed, and the QA arcs that wrote these specification sections had
already run over them.

## The verification and Phase 4b

**`verify-ticket`**, by an agent that did not write the ticket: passes with
gaps. It re-ran all nine recipes, prototyped the eight tests, and found what
the ticket had not: seven tests in `qa_move_reason_test.go` whose fixture
waited for a frame it might already have read (the plan's F1). The 50 ms sleep
that became R9 isolates the fixture's first wait, the one after the perch. One
of the seven failed once in its runs, under the `handleLoadMap` edit, which no
move-reason test reaches. The owner ruled the fixture mended here.

**Phase 4b**, one reviewer over `git diff HEAD`, the ticket and the plan,
before `b0e4172`. It re-ran twelve of the breaks below and, by its message,
eight edits of its own, six of which are in its kept logs. Three Must, two
Should, three nits:

- **Must.** `TestAMapInTheSetWhoseArtStoppedCompilingIsRefused` never seeded
  `act-fighter`, which the cellar map's placement names, so the fold refused
  the load whatever the art, and its head assertion could not fail. Fixed: the
  seed, and the head read before and after.
- **Must.** That test's comment said `LoadInstalled` refuses first when the
  art is broken before the map joins the set; this fixture never reaches
  `LoadInstalled`. Fixed: the comment warns of what holds.
- **Must.** The `wake` entry's closing paragraph dated the first red to
  `a0c48c9`; the seven move-reason tests `904f79b` brought redden under the
  recipe too. Fixed.
- **Should.** The new map test stood between
  `TestAMapWithArtFromALaterFormatIsRefusedOnDemandExactlyAsAtBoot` and the
  comment of `TestArtInstalledAfterBootDrawsWithoutARestart`, which points at
  "the test above". Fixed: the new test stands after the second.
- **Should.** A new comment cited "the test above". Fixed: it names
  `TestAnUnrecognisedPayloadIsWithheldFromAPlayer`.
- **Nits.** Two lines of the plan counted six test files after the fixture
  made seven (fixed); VTT-256's row says "is refused" where the new test shows
  the door left as it stood (left, on the owner's ruling); and one wait in
  `qaMRStartLive` now rests on an unperched spectator being sent no
  `TokenPlaced`, which the reviewer probed and found to hold.

How often the seven fail was counted three ways. The first reviewer's message
gives one failure in 420 runs with `d64cc9c`'s fixture and none in 1,050 with
`b0e4172`'s, and 18 in 1,050 under the `wake` recipe across five of the seven;
no kept log holds those runs. Its kept log, on `e628e18` with that commit's
fixture, holds one failure in 700 runs with nothing broken and 20 in 700 under
the `wake` recipe. This report's reviewer ran the seven 150 times under R3
with `b0e4172`'s fixture and kept the output: 19 failures in 1,050, across all
seven, each `spectator: timed out waiting for the near token placed after the
perch`. Under the `wake` recipe none of the seven is immune.

**This report's review**, one reviewer over this file against `b0e4172`, the
scratch logs and the gate's log. It re-ran all fourteen edits of `breaks.py`,
R9 with each fixture and X5, and reproduced each. Two Must: this report said
the verifier's one fixture failure was on an unbroken tree, where it was under
an unrelated edit; and it counted eight `--- PASS` lines where the command
prints twelve, four of them subtests. Six Should, among them that the first
reviewer's run counts are in no kept log and that "five of the seven" was a
sample, that `repeat.py`'s output and the R9 runner were missing from Appendix
A, and that the barrier's wait was never isolated. Five nits. All fixed here.
The sample had reached the `wake` entry's closing paragraph in
`docs/verification-debt.md`, which this report's commit corrects to the kept
count.

## The breaks

Each is one edit to a non-test file in a clone, undone before the next. "Own
test" is the one of the eight that went red; the seven others stayed green
under each of the fourteen edits from R1y to X3 (Appendix A, `breaks.py` and
its log). R9 and X5 were run against the move-reason tests and the whole
package, not against the eight.

| Name | The edit | Own test, and what it printed |
|---|---|---|
| R1y | `mayWorkDoor`: `abs(tok.Y-at.GetY()) <= 1` made `<= 2` | door test: `opening the door at (3,3) from (4,5) was allowed` |
| R1x | `mayWorkDoor`: `abs(tok.X-at.GetX()) <= 1` made `<= 2` | door test: `opening the door at (3,3) from (5,4) was allowed` |
| R2 | `handleLoadMap`: the return under `Compile`'s error answers `Ok: true` | map test: `a map in the set loaded although its art declares a later format` |
| R3 | `newPerchBox`: `make(chan struct{}, 1)` made `make(chan struct{})` | wake test: `a perch set while nothing waited on wake left no wake-up for the pump`, 30 of 30 |
| R4 | `reperch` answers nil for a shoulder in a new per-projector set | hop-back test: `got 0 of 9 tiles` |
| R5 | `sameSet`: `if !b[k] {` made `if !b[k] && false {` | same-size test: `no SceneSeen was sent` |
| R6 | `Project`: the `unrecognised` arm holds for `RolePlayer` only | spectator test: `got 5 frames` |
| R7 | `handleRemoveActor`: the `ParticipantId` and `ActorRole` assignments removed | removal test: `participant "" and role ""` |
| R8 | `handleJoinDoor`: `SetJoinOpen(false, 0)` called in the refusal arm | door-refusal test: `shut a door that stood open` |
| X1 | R2's arm keeps `Ok: false` and appends the art directory to the error | map test: `a client was told where the campaign lives` |
| X6 | `handleLoadMap` compiles against no art directory | map test, at its first assertion |
| X2 | the stamping loop writes a constant `dm` role | removal test, agent run: `role "dm", want ... "agent"` |
| X4 | the stamping loop drops the participant id | removal test: `participant "" and role "dm"` |
| X3 | `Project` runs `transitions` for a spectator's unknown payload and drops the frames | spectator test: `the control fails` |
| R9 | `serve`: a 50 ms sleep after `answerCommand` | with the old fixture, the seven wire tests; with the mended one, nothing |
| X5 | `forwardable` keeps a `TokenMoved`'s reason | three of the seven wire tests |

X6 was added after the reading review: with the prototype's unseeded fixture it
would not have reddened the map test's first assertion, since the fold refused
the load either way.

## Rule 9: how MapTool does this

No tabletop function is designed here. Each behaviour exists and has its
record; the change adds the tests that hold them. `~/dev/RPTool` was not
searched.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Seven entries, each closed by the test its closing sentence asked for | Three were found already red under their recipes, and kept a direct test each | `ce31229`, `390239a`, `904f79b` and `a0c48c9` brought tests after the entries were written; none names the behaviour, the `wake` reds vary by run, and VTT-201's row needed a test that carries its id. The owner's ruling. |
| The ticket's eleven Done items | A twelfth, and `qa_move_reason_test.go` in the change | The verifier's first run under an unrelated edit went red in a move-reason test; the cause was the fixture's order of reads, and the owner ruled it mended here rather than recorded as debt. |
| The fixture's cursor carrying a warning comment, as the change was first written | No comment; the field is named `lookedAt` and the plan's D14 holds the reason | `check:comments` refused it: the file's ceiling in `tools/comment-ceilings.txt` is 0.6, and with the four comment lines it printed a share of 0.96. No log of that run was kept; the first reviewer measured 0.64 for one added line. |
| The `wake` closing naming `a0c48c9` as the commit since which its recipe reddens (the plan's D9) | It names `904f79b` first, and `a0c48c9` for the two tests that redden in most runs | The reading review found that the seven move-reason tests `904f79b` brought redden under the recipe too, rarely. |
| The map test as prototyped in the plan's Appendix A | It seeds `act-fighter`, compares the head before and after where the prototype required 0, and stands after `TestArtInstalledAfterBootDrawsWithoutARestart` | The reading review: without the seed the fold refuses the cellar map whatever the art, so the prototype's head assertion could not fail and an edit losing the compile refusal another way was caught only by the name check. |
| The `wake` entry closed by a connection test with a busy pump, as its own closing sentence asked | Closed by a test of the box | A connection test of it depends on where the pump is when the perch arrives, which is what makes the two tests the recipe already reddened fail in only six to eight runs of eight. The owner's ruling (Q4); the paragraph says what is not held. |
| The report's commit holding the report alone (the plan's Task 9) | It also changes one sentence of the `wake` entry's closing paragraph in `docs/verification-debt.md` | This report's review traced that sentence's count, 18 in 1,050 across five of the seven, to no kept log, and its own kept run gave 19 in 1,050 across all seven. The owner ruled it corrected here. |
| `task check` once, after the specification commit | Run on `b0e4172` before this report; the run on the tree that holds this report and the specification commit follows both | Done item 11 is answered against the last code commit, as the last arc's report did. |

## What could not be established

- **Whether another QA fixture in the package has the move-reason fixture's
  fault under a delay longer than 50 ms.** R9 failed the seven tests and no
  other; nothing longer was tried.
- **A connection whose pump is busy when a perch is set.** VTT-306 and its test
  hold the box. That the pump takes a waiting wake-up is held by the connection
  tests that perch at an idle table.
- **The map test's head assertion under an edit.** Its ok assertion is fatal,
  and R2 and X6 both stop there. The first reviewer's message says its variant
  of the test saw the head move from 1 to 3 under its two edits; no log of
  that was kept, and the committed test was not measured reaching that line.
- **How often the seven move-reason tests failed with nothing broken before
  the change.** The one kept count is one failure in 700 runs on `e628e18`.
  The verifier's one failure was under an unrelated edit, and the writer's
  runs saw none.
- **The fixture's barrier wait under an edit of its own.** R9 fails the old
  fixture at its first wait, after the perch. The barrier's wait has the same
  shape and is the one the verifier's failure named; no edit isolated it. The
  mended fixture is green under R9 through both waits.
- **The gate on the tree that holds this report and the specification
  commit.** It runs after both, so this report cannot cite it.

## What was deliberately left out, and where it went

- **Every other `Open debt` entry**, the gateway's among them: the busy skip's
  wiring in `serve`, the `load_adventure` note-key overwrite, a connection
  that outlives `Shutdown`, `Modifier.source`, a golden log with a removal.
  They stand in `docs/verification-debt.md`.
- **VTT-256's wording.** The row stands; the new test is in its evidence.
- **VTT-162 as a citation of the map test.** The head assertion is that rule's
  on this path, and no edit was measured reaching it, so the row was not given
  evidence that has not been seen to fail.
- **The specifications' `Requirements` lists.** SPEC-014, SPEC-015 and
  SPEC-017 gain VTT-305, VTT-306 and VTT-307 in the commit after this report.

## Appendix A: the scripts and what they printed

The scratch directory these ran in does not outlive its session, so they are
kept here whole. `<clone>` is a clone of the repository, bare `d64cc9c` for
`recipes.py` and `repeat.py` and holding the change set for the others;
`<worktree>` is the repository.

**`recipes.py`: each recipe against `d64cc9c`, before any test was written**

    """Run each verification-debt recipe against a clone and report whether the
    named packages stay green. Usage: python3 recipes.py <clone> [names...]"""
    import re
    import subprocess
    import sys
    import time

    clone = sys.argv[1]
    only = set(sys.argv[2:])


    def sub1(path, old, new, count=1):
        p = f"{clone}/{path}"
        s = open(p).read()
        assert s.count(old) == count, f"{path}: {old!r} stands {s.count(old)} times, want {count}"
        open(p, "w").write(s.replace(old, new))
        assert new in open(p).read()


    def door_y():
        sub1("internal/gateway/authz.go", "abs(tok.Y-at.GetY()) <= 1", "abs(tok.Y-at.GetY()) <= 2")


    def door_x():
        sub1("internal/gateway/authz.go", "abs(tok.X-at.GetX()) <= 1", "abs(tok.X-at.GetX()) <= 2")


    def compile_arm():
        sub1(
            "internal/gateway/map.go",
            "envs, warnings, err := mapdef.Compile(m, s.artDir)\n\tif err != nil {\n"
            "\t\treturn &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}",
            "envs, warnings, err := mapdef.Compile(m, s.artDir)\n\tif err != nil {\n"
            "\t\treturn &vttv1.CommandResult{RequestId: requestID, Ok: true, Error: err.Error()}",
        )


    def wake():
        sub1("internal/gateway/seat.go", "wake: make(chan struct{}, 1)", "wake: make(chan struct{})")


    def shoulder():
        sub1(
            "internal/gateway/project.go",
            "\tgone map[string]bool\n}",
            "\tgone map[string]bool\n\tserved map[string]bool\n}",
        )
        sub1(
            "internal/gateway/project.go",
            "\tpr.viewer.Viewpoint = actorID\n",
            "\tpr.viewer.Viewpoint = actorID\n\tif pr.served == nil {\n\t\tpr.served = map[string]bool{}\n\t}\n"
            "\tif pr.served[actorID] {\n\t\treturn nil\n\t}\n\tpr.served[actorID] = true\n",
        )


    def same_set():
        sub1("internal/gateway/project.go", "\t\tif !b[k] {\n", "\t\tif !b[k] && false {\n")


    def spectator_unknown():
        sub1(
            "internal/gateway/project.go",
            "\tif v == unrecognised {\n",
            "\tif v == unrecognised && pr.viewer.Role == identity.RolePlayer {\n",
        )


    REMOVE_LOOP = (
        "\t// Stamp the four fields the batch leaves zero: store.AppendBatch requires an\n"
        "\t// EventId (SPEC-017).\n"
        "\tnow := timestamppb.Now()\n"
        "\tfor _, env := range envs {\n"
        "\t\tid, err := newEventID()\n"
        "\t\tif err != nil {\n"
        "\t\t\treturn &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}\n"
        "\t\t}\n"
        "\t\tenv.EventId = id\n"
        "%s"
        "\t\tenv.OccurredAt = now\n"
    )


    def stamping():
        sub1(
            "internal/gateway/server.go",
            REMOVE_LOOP % "\t\tenv.ParticipantId = p.ID\n\t\tenv.ActorRole = string(p.Role)\n",
            REMOVE_LOOP % "\t\t_ = p\n",
        )


    def door_refusal():
        sub1(
            "internal/gateway/server.go",
            "\t\t// closed locks the table out (SPEC-009, JoinDoor).\n\t\treturn &vttv1.CommandResult{",
            "\t\t// closed locks the table out (SPEC-009, JoinDoor).\n"
            "\t\t_ = s.ids.SetJoinOpen(false, 0)\n\t\treturn &vttv1.CommandResult{",
        )


    def busy_bare():
        sub1("internal/gateway/server.go", "write := stampedWrite(&activity, func(b []byte) bool {",
             "write := func(_ *writeActivity, w func([]byte) bool) func([]byte) bool { return w }(&activity, func(b []byte) bool {")


    def busy_false():
        sub1("internal/gateway/server.go", "\t\tactivity.busy, conn.Ping,", "\t\tfunc() bool { return false }, conn.Ping,")


    GW = ["./internal/gateway/..."]
    RECIPES = [
        ("door_y", door_y, [GW]),
        ("door_x", door_x, [GW]),
        ("compile_arm", compile_arm, [GW, ["./cmd/vtt/", "-run", "Map|Art|Load"]]),
        ("wake", wake, [GW]),
        ("shoulder", shoulder, [GW]),
        ("same_set", same_set, [GW, ["./cmd/vtt/..."]]),
        ("spectator_unknown", spectator_unknown, [GW]),
        ("stamping", stamping, [GW]),
        ("door_refusal", door_refusal, [GW]),
        ("busy_bare", busy_bare, [GW]),
        ("busy_false", busy_false, [GW]),
    ]

    FILES = ["authz.go", "map.go", "seat.go", "project.go", "server.go"]


    def restore():
        for f in FILES:
            subprocess.run(["git", "-C", clone, "show", f"HEAD:internal/gateway/{f}"],
                           stdout=open(f"{clone}/internal/gateway/{f}", "wb"), check=True)
        out = subprocess.run(["git", "-C", clone, "status", "--short"], capture_output=True, text=True).stdout
        assert out == "", out


    def run(args):
        t = time.time()
        r = subprocess.run(["go", "test", "-count=1"] + args, cwd=clone, capture_output=True, text=True)
        text = r.stdout + r.stderr
        fails = sorted(set(re.findall(r"^--- FAIL: (\S+)", text, re.M)))
        build = "build failed" in text or "[build failed]" in text or "cannot use" in text
        return r.returncode, fails, build, time.time() - t, text


    restore()
    if "baseline" in only or not only:
        for args in ([GW[0]],):
            rc, fails, build, dt, _ = run(list(args))
            print(f"BASELINE {' '.join(args)}: exit={rc} fails={fails} build_failed={build} {dt:.0f}s", flush=True)

    for name, edit, runs in RECIPES:
        if only and name not in only:
            continue
        restore()
        edit()
        diff = subprocess.run(["git", "-C", clone, "diff", "--stat"], capture_output=True, text=True).stdout.strip().splitlines()
        assert diff, f"{name}: the edit changed nothing"
        for args in runs:
            rc, fails, build, dt, text = run(list(args))
            print(f"RECIPE {name} | go test {' '.join(args)}: exit={rc} fails={fails} build_failed={build} {dt:.0f}s", flush=True)
            if build:
                print(text[-1500:], flush=True)
        restore()
    print("RECIPES_DONE", flush=True)

**Its output**

    BASELINE ./internal/gateway/...: exit=0 fails=[] build_failed=False 46s
    RECIPE door_y | go test ./internal/gateway/...: exit=0 fails=[] build_failed=False 46s
    RECIPE door_x | go test ./internal/gateway/...: exit=1 fails=['TestAPlayerMayOnlyWorkADoorTheyAreNextTo'] build_failed=False 46s
    RECIPE compile_arm | go test ./internal/gateway/...: exit=0 fails=[] build_failed=False 45s
    RECIPE compile_arm | go test ./cmd/vtt/ -run Map|Art|Load: exit=0 fails=[] build_failed=False 6s
    RECIPE wake | go test ./internal/gateway/...: exit=1 fails=['TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause', 'TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause'] build_failed=False 45s
    RECIPE shoulder | go test ./internal/gateway/...: exit=1 fails=['TestQANoteProjectionAPerchChangesNothingAboutNotes'] build_failed=False 76s
    RECIPE same_set | go test ./internal/gateway/...: exit=1 fails=['TestQATestimonySightAChangeUndoneOutOfSightSendsNothingOnSight', 'TestQATestimonySightEveryPairOfStepsKeepsEachSeatToWhatItSees', 'TestQATestimonySightEveryRandomWalkKeepsEachSeatToWhatItSees'] build_failed=False 44s
    RECIPE same_set | go test ./cmd/vtt/...: exit=0 fails=[] build_failed=False 163s
    RECIPE spectator_unknown | go test ./internal/gateway/...: exit=0 fails=[] build_failed=False 45s
    RECIPE stamping | go test ./internal/gateway/...: exit=0 fails=[] build_failed=False 46s
    RECIPE door_refusal | go test ./internal/gateway/...: exit=0 fails=[] build_failed=False 46s
    RECIPE busy_bare | go test ./internal/gateway/...: exit=0 fails=[] build_failed=False 46s
    RECIPE busy_false | go test ./internal/gateway/...: exit=0 fails=[] build_failed=False 46s
    RECIPES_DONE
    exit=0

**`repeat.py`: two of the already-red recipes, eight runs each; its output was not kept**

    import re, subprocess, sys
    sys.argv = ["recipes.py", sys.argv[1], "none"]
    src = open("recipes.py").read().split("restore()\nif \"baseline\"")[0]
    exec(src)
    CASES = [
        ("wake", wake, "TestAPlayerAndASpectatorAreToldNoIssuerAndNoCause|TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause"),
        ("same_set", same_set, "TestQATestimonySightAChangeUndoneOutOfSightSendsNothingOnSight|TestQATestimonySightEveryPairOfStepsKeepsEachSeatToWhatItSees|TestQATestimonySightEveryRandomWalkKeepsEachSeatToWhatItSees"),
    ]
    for name, edit, pat in CASES:
        restore(); edit()
        r = subprocess.run(["go", "test", "-count=8", "-run", pat, "./internal/gateway/"], cwd=clone, capture_output=True, text=True)
        text = r.stdout + r.stderr
        fails = re.findall(r"^--- FAIL: (\S+)", text, re.M)
        passes = len(re.findall(r"^--- PASS", text, re.M))
        from collections import Counter
        print(name, "exit", r.returncode, dict(Counter(fails)), flush=True)
        r = subprocess.run(["go", "test", "-count=8", "-v", "-run", pat, "./internal/gateway/"], cwd=clone, capture_output=True, text=True)
        text = r.stdout + r.stderr
        print(name, "verbose: PASS", dict(Counter(re.findall(r"^--- PASS: (\S+)", text, re.M))), "FAIL", dict(Counter(re.findall(r"^--- FAIL: (\S+)", text, re.M))), flush=True)
    restore()
    print("REPEAT_DONE")

**`breaks.py`: every edit against the eight tests as `b0e4172` holds them**

    """Task 6: each break against the eight new tests, in a clone that holds the
    working tree's change set. Usage: python3 breaks.py <clone> <worktree>"""
    import re
    import subprocess
    import sys

    clone, work = sys.argv[1], sys.argv[2]
    G = "internal/gateway/"
    NEW = {
        1: "TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis",
        2: "TestAMapInTheSetWhoseArtStoppedCompilingIsRefused",
        3: "TestAPerchSetWhileNothingWaitsStillWakesThePump",
        4: "TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt",
        5: "TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain",
        6: "TestAnUnrecognisedPayloadIsWithheldFromASpectator",
        7: "TestARemovalBatchCarriesItsIssuersParticipantAndRole",
        8: "TestAnUnspecifiedDoorLeavesAnOpenDoorOpen",
    }
    PROD = ["authz.go", "map.go", "seat.go", "project.go", "server.go"]


    def sub1(path, old, new):
        p = f"{clone}/{path}"
        s = open(p).read()
        assert s.count(old) == 1, f"{path}: {old!r} stands {s.count(old)} times"
        open(p, "w").write(s.replace(old, new))
        assert new in open(p).read()


    def restore():
        for f in PROD:
            subprocess.run(["cp", f"{work}/{G}{f}", f"{clone}/{G}{f}"], check=True)


    STAMP = "\t\tenv.EventId = id\n%s\t\tenv.OccurredAt = now\n\t}\n\n\tfirstSeq, err := s.campaign.AppendBatch(envs)\n\tif err != nil {\n\t\treturn &vttv1.CommandResult{RequestId: requestID, Ok: false, Error: err.Error()}\n\t}\n\treturn &vttv1.CommandResult{RequestId: requestID, Ok: true, Sequence: firstSeq}\n}\n\n// handleJoinDoor"
    BOTH = "\t\tenv.ParticipantId = p.ID\n\t\tenv.ActorRole = string(p.Role)\n"
    COMPILE = "envs, warnings, err := mapdef.Compile(m, s.artDir)\n\tif err != nil {\n\t\treturn &vttv1.CommandResult{RequestId: requestID, Ok: %s}"

    EDITS = {
        "R1y": (1, lambda: sub1(G + "authz.go", "abs(tok.Y-at.GetY()) <= 1", "abs(tok.Y-at.GetY()) <= 2")),
        "R1x": (1, lambda: sub1(G + "authz.go", "abs(tok.X-at.GetX()) <= 1", "abs(tok.X-at.GetX()) <= 2")),
        "R2": (2, lambda: sub1(G + "map.go", COMPILE % "false, Error: err.Error()", COMPILE % "true, Error: err.Error()")),
        "R3": (3, lambda: sub1(G + "seat.go", "wake: make(chan struct{}, 1)", "wake: make(chan struct{})")),
        "R4": (4, lambda: (
            sub1(G + "project.go", "\tgone map[string]bool\n}", "\tgone map[string]bool\n\tserved map[string]bool\n}"),
            sub1(G + "project.go", "\tpr.viewer.Viewpoint = actorID\n",
                 "\tpr.viewer.Viewpoint = actorID\n\tif pr.served == nil {\n\t\tpr.served = map[string]bool{}\n\t}\n"
                 "\tif pr.served[actorID] {\n\t\treturn nil\n\t}\n\tpr.served[actorID] = true\n"))),
        "R5": (5, lambda: sub1(G + "project.go", "\t\tif !b[k] {\n", "\t\tif !b[k] && false {\n")),
        "R6": (6, lambda: sub1(G + "project.go", "\tif v == unrecognised {\n",
                               "\tif v == unrecognised && pr.viewer.Role == identity.RolePlayer {\n")),
        "R7": (7, lambda: sub1(G + "server.go", STAMP % BOTH, STAMP % "\t\t_ = p\n")),
        "R8": (8, lambda: sub1(G + "server.go",
                               "\t\t// closed locks the table out (SPEC-009, JoinDoor).\n\t\treturn &vttv1.CommandResult{",
                               "\t\t// closed locks the table out (SPEC-009, JoinDoor).\n"
                               "\t\t_ = s.ids.SetJoinOpen(false, 0)\n\t\treturn &vttv1.CommandResult{")),
        "X1": (2, lambda: sub1(G + "map.go", COMPILE % "false, Error: err.Error()", COMPILE % 'false, Error: err.Error() + " in " + s.artDir')),
        "X2": (7, lambda: sub1(G + "server.go", STAMP % BOTH,
                               STAMP % "\t\tenv.ParticipantId = p.ID\n\t\tenv.ActorRole = string(identity.RoleDM)\n")),
        "X3": (6, lambda: sub1(G + "project.go", "\tif v == unrecognised {\n",
                               "\tif v == unrecognised {\n\t\tif pr.viewer.Role == identity.RoleSpectator {\n"
                               "\t\t\t_ = pr.transitions(env, env.GetSequence(), now, st)\n\t\t}\n")),
        "X6": (2, lambda: sub1(G + "map.go", "envs, warnings, err := mapdef.Compile(m, s.artDir)", 'envs, warnings, err := mapdef.Compile(m, "")')),
        "X4": (7, lambda: sub1(G + "server.go", STAMP % BOTH, STAMP % "\t\tenv.ActorRole = string(p.Role)\n")),
    }


    def run(extra):
        pat = "^(" + "|".join(NEW.values()) + ")$"
        r = subprocess.run(["go", "test", "-count=1", "-v", "-run", pat] + extra + ["./internal/gateway/"],
                           cwd=clone, capture_output=True, text=True)
        text = r.stdout + r.stderr
        red = sorted(set(re.findall(r"^--- FAIL: (Test\w+)", text, re.M)))
        green = sorted(set(re.findall(r"^--- PASS: (Test\w+)", text, re.M)))
        return r.returncode, red, green, text


    restore()
    rc, red, green, text = run([])
    print(f"UNBROKEN exit={rc} red={red} green={len(green)}", flush=True)
    assert rc == 0 and len(green) == 8, text[-800:]

    bad = 0
    for name, (want, edit) in EDITS.items():
        restore()
        edit()
        rc, red, green, text = run([])
        ok = red == [NEW[want]] and len(green) == 7
        bad += not ok
        msg = re.findall(r"^\s+\w+_test\.go:\d+: (.*)$", text, re.M)
        print(f"{name} exit={rc} red={red} green={len(green)} {'AS PLANNED' if ok else 'NOT AS PLANNED'} | {msg[:1]}", flush=True)
        if "build failed" in text or (rc != 0 and not red):
            print(text[-1200:], flush=True)

    restore()
    EDITS["R3"][1]()
    r = subprocess.run(["go", "test", "-count=30", "-v", "-run", "^" + NEW[3] + "$", "./internal/gateway/"],
                       cwd=clone, capture_output=True, text=True)
    t = r.stdout + r.stderr
    print(f"R3x30 red={len(re.findall(r'^--- FAIL', t, re.M))} green={len(re.findall(r'^--- PASS', t, re.M))}", flush=True)
    restore()
    rc, red, green, text = run([])
    print(f"REPAIRED exit={rc} red={red} green={len(green)}", flush=True)
    print(f"BREAKS_DONE bad={bad}", flush=True)

**Its output**

    UNBROKEN exit=0 red=[] green=8
    R1y exit=1 red=['TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis'] green=7 AS PLANNED | ['opening the door at (3,3) from (4,5) was allowed']
    R1x exit=1 red=['TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis'] green=7 AS PLANNED | ['opening the door at (3,3) from (5,4) was allowed']
    R2 exit=1 red=['TestAMapInTheSetWhoseArtStoppedCompilingIsRefused'] green=7 AS PLANNED | ['a map in the set loaded although its art declares a later format']
    R3 exit=1 red=['TestAPerchSetWhileNothingWaitsStillWakesThePump'] green=7 AS PLANNED | ['a perch set while nothing waited on wake left no wake-up for the pump']
    R4 exit=1 red=['TestAShoulderSatOnAndLeftIsRestoredByHoppingBackToIt'] green=7 AS PLANNED | ["hopping back must light the first shoulder's room again, got 0 of 9 tiles"]
    R5 exit=1 red=['TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain'] green=7 AS PLANNED | ['the squares the hero sees changed to as many others and no SceneSeen was sent']
    R6 exit=1 red=['TestAnUnrecognisedPayloadIsWithheldFromASpectator'] green=7 AS PLANNED | ['an unknown payload must be withheld from a spectator, got 5 frames']
    R7 exit=1 red=['TestARemovalBatchCarriesItsIssuersParticipantAndRole'] green=7 AS PLANNED | ['batch envelope 0 carries participant "" and role "", want "485d82c64240f6a4cd5310fcf7410e53" and "dm"']
    R8 exit=1 red=['TestAnUnspecifiedDoorLeavesAnOpenDoorOpen'] green=7 AS PLANNED | ['a refused set_join_door shut a door that stood open']
    X1 exit=1 red=['TestAMapInTheSetWhoseArtStoppedCompilingIsRefused'] green=7 AS PLANNED | ['a client was told where the campaign lives:']
    X2 exit=1 red=['TestARemovalBatchCarriesItsIssuersParticipantAndRole'] green=7 AS PLANNED | ['batch envelope 0 carries participant "b9f809332f39b33cfb7ee4a716263b41" and role "dm", want "b9f809332f39b33cfb7ee4a716263b41" and "agent"']
    X3 exit=1 red=['TestAnUnrecognisedPayloadIsWithheldFromASpectator'] green=7 AS PLANNED | ["the control fails: the next recognised event must bring the shoulder's board"]
    X6 exit=1 red=['TestAMapInTheSetWhoseArtStoppedCompilingIsRefused'] green=7 AS PLANNED | ['a map in the set loaded although its art declares a later format']
    X4 exit=1 red=['TestARemovalBatchCarriesItsIssuersParticipantAndRole'] green=7 AS PLANNED | ['batch envelope 0 carries participant "" and role "dm", want "c41e5dd4e876c94fdf4bcdde4209775e" and "dm"']
    R3x30 red=30 green=0
    REPAIRED exit=0 red=[] green=8
    BREAKS_DONE bad=0

**`fixture-breaks.py`: the move-reason fixture under R9 and X5, as first run**

    """The move-reason fixture under R9 and X5. Usage: python3 fixture-breaks.py <clone> <worktree>"""
    import re, subprocess, sys
    clone, work = sys.argv[1], sys.argv[2]
    G="internal/gateway/"
    def sub1(path, old, new):
        p=f"{clone}/{path}"; s=open(p).read()
        assert s.count(old)==1, (path, old, s.count(old))
        open(p,"w").write(s.replace(old,new))
    def cp(f): subprocess.run(["cp", f"{work}/{G}{f}", f"{clone}/{G}{f}"], check=True)
    def old_fixture():
        out=subprocess.run(["git","-C",clone,"show",f"HEAD:{G}qa_move_reason_test.go"],capture_output=True,check=True).stdout
        open(f"{clone}/{G}qa_move_reason_test.go","wb").write(out)
    def r9(): sub1(G+"server.go","\t\tresult := s.answerCommand(now, err, cmd, perches)\n","\t\tresult := s.answerCommand(now, err, cmd, perches)\n\t\ttime.Sleep(50 * time.Millisecond)\n")
    def x5(): sub1(G+"project.go","\t\tp.TokenMoved.Reason = \"\"\n","")
    def run(args):
        r=subprocess.run(["go","test","-count=1","-v"]+args,cwd=clone,capture_output=True,text=True)
        t=r.stdout+r.stderr
        return r.returncode, sorted(set(re.findall(r"^--- FAIL: (Test\w+)",t,re.M))), len(set(re.findall(r"^--- PASS: (Test\w+)",t,re.M))), t
    for f in ["server.go","project.go"]: cp(f)
    old_fixture(); r9()
    rc,red,green,t=run(["-run","TestQAMoveReasonOverTheWire","./internal/gateway/"])
    print(f"R9 old fixture: exit={rc} red={len(red)} {red} green={green}", flush=True)
    cp("qa_move_reason_test.go")
    rc,red,green,t=run(["./internal/gateway/"])
    print(f"R9 mended fixture, whole package: exit={rc} red={red} green={green}", flush=True)
    if "build failed" in t: print(t[-800:])
    cp("server.go"); x5()
    rc,red,green,t=run(["-run","TestQAMoveReasonOverTheWire","./internal/gateway/"])
    print(f"X5 mended fixture: exit={rc} red={red} green={green}", flush=True)
    cp("project.go")
    rc,red,green,t=run(["-run","TestQAMoveReason","./internal/gateway/"])
    print(f"repaired, TestQAMoveReason: exit={rc} red={red} green={green}", flush=True)
    print("FIXTURE_DONE")

The second run, whose output comes first below, left out the old-fixture step
and ended with three unbroken runs of `TestQAMoveReason`.

**The fixture under R9 and X5, as `b0e4172` holds it**

    R9 mended fixture, whole package: exit=0 red=[] green=514
    X5 mended fixture: exit=1 red=['TestQAMoveReasonOverTheWireAPlayerCatchingUpIsSentNoReason', 'TestQAMoveReasonOverTheWireAPlayerIsNotSentBackItsOwnReason', 'TestQAMoveReasonOverTheWirePlayerAndSpectatorDoNotHearAnAgentsReason'] green=4
    repaired x3: ok  	github.com/PatrikLager/vtt-platform/internal/gateway	0.689s
    FIXTURE2_DONE

**The fixture under R9 before the change (the first line), from the first run**

    R9 old fixture: exit=1 red=7 ['TestQAMoveReasonOverTheWireADMCatchingUpIsSentEveryEventAfterItsCursorUnchanged', 'TestQAMoveReasonOverTheWireAHiddenMoveReachesNoPlayerOrSpectator', 'TestQAMoveReasonOverTheWireAMoveWithNoReasonAppendsNone', 'TestQAMoveReasonOverTheWireAPlayerCatchingUpIsSentNoReason', 'TestQAMoveReasonOverTheWireAPlayerIsNotSentBackItsOwnReason', 'TestQAMoveReasonOverTheWireDMAndAgentHearAnAgentsReason', 'TestQAMoveReasonOverTheWirePlayerAndSpectatorDoNotHearAnAgentsReason'] green=0
    R9 mended fixture, whole package: exit=0 red=[] green=514
    X5 mended fixture: exit=1 red=['TestQAMoveReasonOverTheWireAPlayerCatchingUpIsSentNoReason', 'TestQAMoveReasonOverTheWireAPlayerIsNotSentBackItsOwnReason', 'TestQAMoveReasonOverTheWirePlayerAndSpectatorDoNotHearAnAgentsReason'] green=4
    repaired, TestQAMoveReason: exit=0 red=[] green=32
    FIXTURE_DONE
