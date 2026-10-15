# The perception decision leaves the gateway: the change

**Ticket:** `docs/superpowers/specs/2026-10-15-the-perception-decision-leaves-the-gateway-design.md`,
returned by its first verification and revised on the owner's ruling, then
amended by its writer after the second (rule 3's wording, item 5's field list)
and after the first commit's review (rule 1, rule 3, G1 in The problem).
**Plan:** `docs/superpowers/plans/2026-10-15-the-perception-decision-leaves-the-gateway.md`,
written by the agent that verified the ticket; its header, G1, Task 3's
histories, Task 4's register line and the section "Before Task 1" were
edited by the ticket's writer before `c80e502` first committed it, and the
plan does not mark which.
**The owner's rulings (2026-10-15):** the per-character-logs arc is four
tickets in order and this is the first; one character at a time, no viewpoint
no events, and a late log opens with the present (all for later tickets);
option (ii), the viewer's fold is the decision's memory at every event; the
client and contract comment pointers wait for ticket 4; at sign-off the plan
approved and G1 accepted; and from Task 1 on, every review finding fixed and
each task committed without asking unless a finding is a design question.
**Last code commit:** `6d560e5`, on `93e98ca`, `main` at the time. It
changes only comments in Go files; the last commit that changes what the
code does is `1a5f8a7`. Every code reference below is to `6d560e5`'s tree.

## The period, in commits

    git log --oneline 93e98ca..6d560e5

    6d560e5 Comment lines this branch added fit the prose gate's width
    1a5f8a7 internal/perceive is held by the mutation gate and a coverage floor
    0d3af5b The testimony eyes tests move, and three branches get perceive's own tests
    cfa1da2 The roster and status tests move into internal/perceive
    1bb9d11 The board tests move into internal/perceive
    112e47a A projection started over from its viewer's fold sends what one fed from the start sends
    125298c The perception decision lives in internal/perceive
    63bdb3a A projection's memory is the fold of what its viewer was sent
    16f515d What a viewer holds after five histories is pinned before the memory changes
    733cee1 VTT-311 is held by a test that reads the perception decision's imports
    c80e502 The perception decision gets a component of its own

    git diff --stat 93e98ca..6d560e5

      .go-arch-lint.yml                                  |   14 +-
      docs/requirements.md                               |  102 +-
      ...5-the-perception-decision-leaves-the-gateway.md |  695 +++++
      ...erception-decision-leaves-the-gateway-design.md |  152 +
      docs/verification-debt.md                          |    6 +-
      internal/engine/visibility_fold_test.go            |    2 +-
      internal/gateway/export_test.go                    |    6 +
      internal/gateway/history_fold_test.go              |  158 +
      internal/gateway/keystone_test.go                  |   21 -
      internal/gateway/project.go                        |  849 +----
      internal/gateway/project_internal_test.go          |  111 +-
      internal/gateway/project_test.go                   | 3278 +-------------------
      internal/gateway/qa_fold_memory_test.go            |  803 +++++
      internal/gateway/qa_testimony_eyes_test.go         |  282 --
      internal/gateway/qa_testimony_removal_test.go      |   38 +-
      internal/gateway/resume_test.go                    |  301 ++
      internal/gateway/seat.go                           |    6 +-
      .../history-fold/condition-order/state.json        |   93 +
      .../history-fold/controller-restatement/state.json |   90 +
      .../history-fold/reuse-identical/state.json        |   85 +
      .../history-fold/reuse-module-id/state.json        |   86 +
      .../testdata/history-fold/reuse-renamed/state.json |   85 +
      internal/gateway/viewpoint_internal_test.go        |   24 +
      internal/perceive/board_test.go                    | 1104 +++++++
      internal/perceive/deps_test.go                     |  115 +
      internal/perceive/history_test.go                  |  230 ++
      internal/perceive/projection.go                    |  884 ++++++
      internal/perceive/projection_internal_test.go      |   85 +
      internal/perceive/qa_testimony_eyes_test.go        |  447 +++
      internal/perceive/roster_test.go                   |  860 +++++
      internal/perceive/status_test.go                   |  498 +++
      internal/perceive/unseen_test.go                   |  160 +
      internal/perceive/viewer_test.go                   |  238 ++
      tools/check-mutation.py                            |    8 +-
      tools/comment-ceilings.txt                         |   25 +-
      tools/coverage-thresholds.txt                      |    3 +-
      tools/mutation-equivalents.txt                     |   12 +-
      tools/mutation-scope.md                            |    8 +-
      38 files changed, 7501 insertions(+), 4463 deletions(-)

## What the code did on `93e98ca`

The `Projector` in `internal/gateway/project.go` decided what a player or a
spectator is sent of each event, one per projected connection, fed the log
from its first event by `seat.receive`. It remembered what it had sent in
nine maps (`scenes`, `actors`, `tokens`, `seen`, `doors`, `notes`, `sighted`,
`belief`, `gone`) that only `Project` and `reperch` changed, so nothing could
build one from what a viewer held. `campaign` could not call it: the code
lived in `gateway` and called `sight`, and `.go-arch-lint.yml` allowed
`campaign` neither. In three histories the maps said more than any fold of
what was sent: an actor added again, unchanged, under an id the viewer still
held (`gone`); a correction removing several conditions, whose order came
from the world's (`belief`); and a kind restatement, which named the world's
first controller.

## Done looks like, answered

1. **[x]** `internal/perceive` exists and `.go-arch-lint.yml` names a
   `perceive` component allowed `contract`, `engine`, `sight` and itself.
   `go-arch-lint check` is green on `6d560e5`, and was seen red, naming the
   file, on a production import of `internal/store`, `internal/identity` and
   `internal/gateway`, and on a test file importing `internal/campaign`
   (`c80e502`'s breaks). `TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight`
   holds the same rule as a test (VTT-311).
2. **[x]** `grep -n 'func (pr \*Projector) \(classify\|look\|transitions\)'
   internal/gateway/*.go` prints nothing on `6d560e5`; on `93e98ca` it
   printed three lines. The decision is `perceive.Projection` (`New`,
   `Event`, `Perch`, `CanSee`). In the move itself, `125298c` against
   `63bdb3a`, 28 of the moved declarations are verbatim apart from the
   receiver and comments, `eyes` and `recall` being the two that differ
   (checked by script and by the reviewer's AST comparison).
3. **[x]** `git diff --stat 93e98ca 6d560e5 -- scenarios/goldens` prints
   nothing. `TestTheProjectedGoldensAreWhatTheProjectionActuallySends` and
   `TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees` pass, and
   every test citing a row of SPEC-015's or SPEC-016's Requirements passes
   where it now lives; `task check:requirements-chain` is green (314 rows).
4. **[~]** `TestADecisionStartedOverFromWhatItsViewerHoldsSendsWhatOneFedFromTheStartSends`
   in `internal/gateway/resume_test.go` (VTT-312) decides each history twice
   and requires `proto.Equal` frames: the 11 oracle seats and the five
   hand-written histories at every prefix, and eventgen seeds 1-6 (two
   players, a fixed spectator and a spectator that perches) at every 10th
   prefix and at and after each perch. Changed from the item's wording: the
   eventgen part is sampled, per the plan's D13 (the percher otherwise than
   D13 says, under Deviations), and the histories are five where the item
   names the ticket's three (two more pin VTT-313's afresh clause). It cannot
   pass on `93e98ca`, where nothing builds a `Projector` from a fold; it reds
   on a resume that starts with nothing sighted and on `gone` kept across
   events (`112e47a`'s breaks). About 28 s when committed; the report's
   reviewer measured 23 s on `6d560e5`.
   `TestAProjectionStartedOverFromItsViewersFoldSendsWhatTheContinuingOneSends`
   holds the same rule in perceive's own suite.
5. **[~]** `TestEachHistoryFoldAViewerIsSentIsTheOneDerivedByHand`
   (`internal/gateway/history_fold_test.go`) compares each history's viewer
   fold with a `state.json` derived by hand and committed in `16f515d`, where
   the test passed on `93e98ca`'s projector; no later commit changed those
   files. The frames the decision now sends are pinned by
   `TestAnActorAddedAgainIsIntroducedAfreshOnlyWhenItIsAnother` and
   `TestACorrectionFollowsTheOrderTheViewerHolds`, both red on `93e98ca`'s
   projector exactly for the identical re-add and the two reordered
   corrections. VTT-211 is withdrawn, succeeded by VTT-313. Changed in when
   it lands: the item's last clause, that SPEC-016 says in what order a
   correction sends removals and revocations and which controller it
   restates, is not met on `6d560e5`, where `git diff --stat 93e98ca 6d560e5
   -- docs/specifications` prints nothing and SPEC-016 still says a belief is
   written from `st`, false since `63bdb3a`. The process writes a
   specification after its report, so the specification step, in the commit
   after this report's own, amends SPEC-016 to say it.
6. **[x]** `./internal/perceive/` is in `check-mutation.py`'s PACKAGES, and
   item 7's run measured it: "4 survivor(s), 4 adjudicated, 1 TIMED OUT",
   the timed-out mutant being `INCREMENT_DECREMENT` on `y++` in
   `objectInSight`'s footprint walk (below, under what could not be
   established); `internal/gateway` "1 survivor(s), 1 adjudicated".
   `tools/coverage-thresholds.txt` holds `internal/perceive 94.5`, and
   `check:coverage` measured 95.3% from perceive's own tests. The three
   adjudications that named `internal/gateway project.go` name
   `internal/perceive projection.go`.
7. **[x]** `task check`, whole, on `6d560e5`: started at 21:42 on
   2026-10-15 by a load guard, it printed `TASK_CHECK_EXIT=0` at 22:47 with
   no `task: Failed`. `check:mutation` measured perceive and gateway afresh
   and ended "15 packages, zero unadjudicated survivors";
   `check:requirements-chain` read 314 rows. The run before it, on
   `1a5f8a7`, failed at `check:new-prose` (under Deviations).

## What the rules became

- **VTT-311**, rule 1: held by `internal/perceive/deps_test.go#TestNoFileOfThePerceptionDecisionImportsBeyondContractEngineAndSight`.
- **VTT-312**, rule 2: held by the two resume tests named under item 4.
- **VTT-313**, rule 3, VTT-211's successor: held by
  `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh`,
  `TestAnIdReusedAfterAnUnseenRemovalFoldsAgain`,
  `TestAnActorAddedAgainIsIntroducedAfreshOnlyWhenItIsAnother`,
  `TestAnActorAddedAgainWithAnotherAttributeIsIntroducedAfresh` and two of
  QA's tests.
- **VTT-314**, from the plan's D1 (a seat is sent nothing when the fold of
  what it was sent fails): held by `TestAProjectorWhoseViewerFoldFailsSendsNothing`
  and QA's `TestQAFoldMemoryAViewerWhoseFoldFailsIsSentNothing`.
- **VTT-211** is withdrawn in `63bdb3a`, where it became false.

## The sort

Accepted: the ticket's three rules and D1's fail-closed rule (VTT-311..314).
Refused, one line each:
- D4, a correction follows the viewer's fold order and restates its first
  controller: no client relies on frame order within a correction; it is
  SPEC-016's description, pinned by tests that hold no row.
- "The decision does not write the states it is handed": already SPEC-016's,
  with its own row.
- A player's eyes and a spectator's: already SPEC-016's rows.
- `canSee` keeps its name and signature: how, not a rule.
- Tests move rather than copy; main's folds derived by hand: process.
- Item 4's runtime under 30 s: a measurement.
- perceive's floor and mutation gate: gate configuration.
- `New` given the empty world is the fresh constructor: how.

## Phase 4a

Run once, for `63bdb3a` (the one behaviour change). An agent given the
requirement, SPEC-016 whole and the exported surface, and none of the code,
diff or tests, wrote `internal/gateway/qa_fold_memory_test.go`: 12 tests
holding 38 subtests, all passing, 13 of its injections red. `63bdb3a`'s
message says 45 subtests, which matches neither count (`go test -v -run
TestQAFoldMemory ./internal/gateway/`).

### QA adjudications

QA finding 1: spec gap — a perch is unreachable from the exported surface (`reperch` is unexported) — no change: perch corrections are held by gateway internal tests (`TestAPerchOntoAStaleShoulderCorrectsIt`) and by item 4's perching histories.
QA finding 2: spec ambiguity — a party member added again as another while unseen is introduced afresh at the re-add, before sight, while VTT-313 speaks of a viewer that sees it — a party member is on every roster, seen or not (SPEC-016), so its introduction does not wait for sight — SPEC-016 says so at the specification step.
QA finding 3: spec ambiguity — the accepted G1 also leaves controller order and its `controller_id` mirror in the viewer's order after an unchanged re-add with controllers crossing it — within the owner's ruling: no player or spectator view reads either (`client/src`: `player.ts` tests membership, `dm.ts` is the DM's) — recorded here and in SPEC-016.
QA finding 4: spec gap — no order among several grants in one correction — the code grants in the world's controller order — SPEC-016 says so.
QA finding 5: spec gap — the order of a correction's categories was read from a semicolon list — SPEC-016 says "then".
QA finding 6: spec gap — VTT-314 does not say whether the breaking frames are sent or whether a seat recovers — the breaking frames are the projection's own and are sent; every later fold includes them, so the seat is sent nothing more, as VTT-191 says of the world's fold — SPEC-016 says so.
QA finding 7: requirements needing an id — the correction's order rules — not allocated: the sort refused them (see The sort).

## The verification and Phase 4b

The ticket's first verification returned it: rule 2 was false (an id reused
while unseen, and two order differences, are not functions of the fold and
the world), item 4's corpus reached none of those histories, SPEC-018 was not
named, and the file list missed the test port and the register. The owner
ruled option (ii); the revised ticket passed with gaps G1 (an unchanged
re-add carrying conditions or controllers ends equal only as sets) and G2
(rule 3's field list, which the plan's D3 widened to every field only
`ActorAdded` writes). The owner accepted G1 at sign-off.

Every task had its own Phase 4b reading, by `pr-review-toolkit:code-reviewer`
at high effort, each with probes in a copy of the tree; each finding was fixed
before its commit unless named below. What the readings caught that the gates
did not, by task:
- `c80e502`: VTT-313 and rule 3 claimed the viewer's fold equals the world's,
  which G1 denies; reworded before the row was first committed.
- `733cee1`: the test accepted subpackages the archfile refuses.
- `16f515d`: the test's name did not match the plan's `-run HistoryFold`, so
  that gate ran nothing; and the restatement history used an `ActorKind` the
  contract does not declare. (The commit hook's vocabulary gate, not the
  reading, then refused the resource name `hp`.)
- `63bdb3a`: the fold error was dropped from the log.
- `125298c`: the move changed behaviour: a perch refused before the seat had a
  world no longer moved the eyes; `Event` now takes the eyes on every call
  (`TestAPerchBeforeTheSeatHasAWorldStillMovesItsEyes`, red before the fix).
- `112e47a`: item 4 lists a fixed eventgen spectator, which the test lacked;
  the eventgen seats were thin (one percher whose seven perches all missed a
  party member, a player controlling nothing) and passed the vacuity guard;
  and perch boundaries were thinly sampled. The seats were rebuilt, every
  history must resume with eyes at least once, and the steps of and after
  each perch are sampled.
- `1bb9d11`, `cfa1da2`: moved tests checked declaration by declaration;
  comments the move made false, an orphaned history block, two archer
  comments that claimed more than their tests guard.
- `cfa1da2`: a comment ceiling raised (below).
- `0d3af5b`: three branches of `projection.go` reached only by gateway tests,
  which the mutation gate would have counted NOT COVERED rather than report.
- `1a5f8a7`: perceive's place in PACKAGES and its runtime, the timed-out
  mutant left unrecorded, VTT-312's row not naming the new test, and the
  gateway's coverage floor (Deviations). The reviewer also tried and failed to kill the
  new adjudication with a dedicated probe, and confirmed the removed note
  code dead.
- `6d560e5`: the first shortening of `forwardedOf`'s doc said "a viewer",
  which takes in the DM and the agent, who are sent the envelope unchanged;
  and the shortened `scenes` comment had lost its warning.

**A rule 2 slip, owned.** Twice a row of `tools/comment-ceilings.txt`
already committed on this branch was deleted and regenerated to fit a
comment, which raised it while `check-comments.py main` stayed green:
`internal/perceive/projection.go` 18.2 → 18.3 (landed in `1bb9d11`) and
`internal/gateway/history_fold_test.go` 3.7 → 4.0 (caught by `cfa1da2`'s
review). Both are back at 18.2 and 3.7 from `cfa1da2` on, every row was
checked to only fall across the branch, and every later commit ran
`check-comments.py` against both `main` and `HEAD`.

## Rule 9: how MapTool does this

As the plan's section, confirmed against `~/dev/RPTool/maptool` at
`f4b7fef6c`: per-token exposure (`Zone.exposedAreaMeta`) is memory per pair
of eyes, which this arc borrows as memory per viewer; it is sent whole to every
client (`Zone.toDto`, `SetCampaignMsg`), which this arc refuses; MapTool keeps
no memory of what a client was sent and never reuses an identity, so it has
no precedent for item 4 or for VTT-313's case.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| `check:comments` red for perceive until Task 10 (D11) | red only until `125298c` | the package doc moved into `projection.go` and the doc-only file was deleted |
| Behaviour-neutral move | the eyes are now passed to `Event` on every call | the move as first made changed what a refused perch does; see `125298c` |
| Task 9 ports all three QA testimony files | `qa_testimony_sight_test.go` and `qa_testimony_removal_test.go` stay | their worlds seat a DM and an agent and check VTT-176 at every event; D8 keeps tests needing identity in the gateway; three branches only they reached got perceive tests instead |
| Item 4's eventgen part at every prefix | sampled | D13: the unsampled run took about 214 s |
| D13: the percher resumed at every 5th prefix | at every 10th, as the fixed seats are, and at the step of each perch and the step after it | the perches are steps inserted after every 30th event, so their places in the history shift; a fixed stride reaches a perch only by chance, and `112e47a`'s review found the perch boundaries thinly sampled |
| `project_test.go` changes only by deletions | its remaining comments swept | the move left legacy history comments at 33% of the file, and git paired untouched lines as added, so `check:comments` refused them; rewritten as warnings and pointers rather than gaming the diff |
| Gateway floor unchanged | 88.5 → 93.5 | measured 94.5 by Task 10's run after the move (item 7's run measured 94.3); six points of slack would hide the next deleted test |
| Byte-equal comparison of the hand-derived folds | canonicalised JSON, array order kept | nested field order is Go's and carries no meaning |
| Task 11: one whole `task check` on the last code commit | two runs; the first, on `1a5f8a7`, failed at `check:new-prose`, and `6d560e5` was made for it | eleven comment lines the branch added were wider than 85 characters, six of mine and five section dividers in QA's `qa_fold_memory_test.go`; `check:new-prose` runs only in `task check`, never in the commit or push hooks, and no task before Task 11 ran it by hand, as each ran `check-comments.py` |

## What could not be established

- Whether the eventgen sample in item 4 reaches every case the unsampled run
  would. The verifier's unsampled probe (12 seeds) found no divergence; the
  committed test samples 6 seeds.
- The perceive mutant on objectInSight's `y++` is counted as killed by
  timeout, not measured; it is non-terminating and not equivalent
  (`tools/mutation-scope.md`).
- Two NOT COVERED mutants on `eyes()`'s switch case conditions are outside
  gremlins' reach; the reviewer injected both by hand and both are killed.

## What was deliberately left out, and where it went

- Comments outside the gateway naming `project.go`'s `classify`, `look`,
  `reperch` or `perchSequence` (`client/src/app.ts`, `fold.ts`,
  `view/spectator.ts`, `wire.ts`, `client/test/app.test.ts`, `session.test.ts`,
  `spectator-view.test.ts`, `contract/events.test.ts`): ticket 4, by the
  owner's ruling.
- Code comments in long blocks still placing moved code in the gateway:
  `internal/mapdef/compile.go`'s construction-site list,
  `internal/gateway/server_internal_test.go` ("project.go's order"),
  `internal/gateway/remove_actor_test.go`'s agent-seat comment: ticket 3,
  which deletes the gateway projection.
- `internal/engine/apply.go`'s claim that `project.go` quotes
  `ErrSceneExists`'s format was false before this ticket: noted for ticket 3.
- SPEC-015, SPEC-016 and SPEC-018: the specification step after this report.
- The two resume tests and two copies of the five histories' events
  (`internal/gateway/history_fold_test.go`, `internal/perceive/history_test.go`),
  each saying to keep them equal: the gateway's copy stays for the
  hand-derived fold test and the resume test, which cannot move.

## History the code no longer carries

**Three tests left with the forgetting loop on 2026-08-31.**
`TestASceneGoneFromTheWorldIsForgottenSILENTLY`,
`TestASceneThatComesBackIsUsableAgain` and
`TestASceneThatComesBackBringsItsOpenDoorsBack` built a world that had lost a
scene by hand, because only an undo could take one away. What they covered is
unreachable rather than untested: there is no `delete_scene`
(`docs/superpowers/specs/2026-08-30-retraction-leaves-design.md` §5.3), no
arm of `engine.Apply` deletes from `st.Scenes`, and a second `SceneCreated`
under a live id is refused (`internal/engine/apply_test.go`, "duplicate scene
id"). The day the world can lose a scene, `transitions` needs a forgetting
loop for `scenes`, and these three are the tests to restore with it; the
comment at `perceive.Projection`'s `scenes` points here. The block that said
so stood in `internal/gateway/project_test.go` until `1bb9d11`.

**Two keystone measurements** in `internal/gateway/keystone_test.go`, removed
in `1bb9d11` because the move made their counts false: a shared-fold injection
then redded seven top-level tests (two fixture-backed, five behavioural), and
making `sceneSeenFor` walk `sc.Tiles` redded seven, among them
`TestSceneSeenCarriesOnlyTheSquaresInSight`. Measured at their time.

**`internal/gateway/project_test.go`'s legacy comments**, swept in `cfa1da2`,
stand in git at `1bb9d11`.
