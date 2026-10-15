# The perception decision leaves the gateway: implementation plan

- **Ticket:**
  `docs/superpowers/specs/2026-10-15-the-perception-decision-leaves-the-gateway-design.md`
  (the revision verified 2026-10-15).
- **Verdict:** PASSES WITH GAPS, gaps G1 and G2, both planned around below.
  The first verification returned the ticket; the owner ruled option (ii) and
  the revision passed. The probes behind the verdicts ran in a scratch copy and
  are not committed.
- **Signed off** by the owner on 2026-10-15, G1 accepted.
- **Base:** main `93e98ca`.
- **Branch:** `refactor/the-perception-decision-leaves-the-gateway`.

Written by the agent that verified the ticket, after reading the code each task
touches: `project.go`, `seat.go`, `viewpoint.go`, the `server.go` move gate,
`campaign.FoldPrefix`, `engine.Apply`'s actor, condition and control arms, the
`.go-arch-lint.yml` entries, the twelve projection test files, the three
`project.go` adjudications, `check-mutation.py`'s runner, the coverage and
comment gates, and SPEC-008, SPEC-010, SPEC-015, SPEC-016, SPEC-018 and
SPEC-020.

## Constraints that bind every task

These are named here, not restated.

- **CLAUDE.md rules:**
  - 2: `task check` is never weakened, and a gate change is its own reviewed
    decision.
  - 3: additive contract. This plan changes no `.proto`.
  - 4: one fold.
  - 5: no game-system vocabulary.
  - 8: citations name durable things.
  - 9: RPTool first (answered below).
  - 10: a comment is a warning or a pointer.
- **The `dev-cycle` package's phases.** Tests first, a deliberate break for
  every new check, Phase 4a QA, Phase 4b review before every commit, the
  report, then the specifications. They live in the package and are not
  repeated here.
- **The owner's rulings of 2026-10-15:**
  - The arc is four tickets, in order.
  - No dual path, no flag, no migration. No log is ever regenerated.
  - Every viewer's stream is unchanged outside the three histories.
  - Option (ii): the viewer's fold is the memory at every event.
  - The client and contract comment pointers wait for ticket 4.
  - VTT-211 is withdrawn, succeeded by rule 3.
- **SPEC-008:** ids come from the dispenser, only after sign-off.
- **SPEC-010:**
  - A file with no ledger row may carry at most a 25% comment share.
  - Ceilings only go down (`check-comments.py --write-ledger`).
  - History goes to the report, never beside the code.
- **SPEC-020:** an adjudication names the code it excuses.
- **The golden discipline** (`cmd/vtt/scenario_goldens_test.go`, ADR-009): a
  golden is derived by hand first, and there is no `-update` flag.
- **Fixed by this ticket's item 1:** `internal/perceive`, its test files
  included, may import nothing in the module but `contract`, `engine`, `sight`
  and itself.
- **The perception code must not write to the states it is handed.** The live
  event and the folded states are shared. Semgrep's `no-direct-state-writes`
  covers `internal/perceive` automatically.
- **Operational lessons in the project memory, binding here:**
  - Review packages are built with `git diff HEAD`.
  - Never stash while a reviewer reads.
  - Settle the review before starting the gate, and re-point the mutation keys
    last.
  - Read the machine's load with `os.getloadavg()` before a gate run, launch
    `task check` with `start_new_session=True`, and never edit the tree while
    the mutation gate runs.
  - Verify what landed after every commit.

## Rule 9: how MapTool solves this

Confirmed against `~/dev/RPTool/maptool` at `f4b7fef6c`. The section
"Rule 9: how MapTool solves this" in
`docs/superpowers/plans/2026-09-14-per-character-logs.md` holds, and each
claim below is now checked by symbol.

**Borrowed:**

- `Zone.exposedAreaMeta` is a `Map<GUID, ExposedAreaMetaData>`, and
  `ExposedAreaMetaData.exposedAreaHistory` is an accumulated `Area`. That is
  memory per pair of eyes, written when the seeing happened.
- This arc keeps memory per viewer, and from ticket 2 per character, as the
  fold of what that viewer was sent.

**Refused, as before:**

- `Zone.toDto` writes every token's exposure (`dto.putExposedAreaMeta`).
- `MapToolServer.addRemoteConnection` sends `SetCampaignMsg` with
  `campaign.toDto()` to every connecting client. Visibility is therefore a
  client-side filter, which is the premise this platform rejects.

**Where MapTool does not help:**

- It keeps no memory of what a client was sent, and never resumes a
  per-client decision; it resends the whole campaign. Item 4 has no precedent
  there.
- It never meets the reused-id question. Its identities are fresh
  (`Token.exposedAreaGUID = new GUID()`, built from a `UUID`), while this
  platform's `add_actor` takes the actor id from the client.
- Refusing reuse the MapTool way would change the fold and the commands, and
  is outside this ticket. D3 is the in-scope answer.

## Decisions this plan makes

Each decision names what forced it.

**D1. The memory is the viewer's fold, rebuilt at every event.**

- The gateway's `Projector` keeps the frames it has produced.
- On every `Project` and `reperch` it folds those frames with
  `campaign.FoldPrefix`.
- The decision rebuilds its working maps (`scenes`, `actors`, `tokens`,
  `notes`, `doors`, `seen`, `belief`, `gone`) from that fold and the world.
- If the viewer's fold fails, the event is withheld (fail closed, as VTT-191
  does for the world's fold).

*Forced by:*

- The owner's ruling (ii).
- Rule 4: `FoldPrefix` exists so the gateway keeps no loop of its own.
- `perceive` may not import `campaign` (item 1), so it is handed the folded
  state rather than folding.
- The cost is O(n²) per seat, which is the shape the world fold already has.
  Ticket 3 deletes it.

**D2. `sighted` is the one value carried from event to event.**

- A decision started over sets it to the look at the world it starts from.
- A decision started from the first event is the same constructor, given the
  empty world.

*Forced by:*

- `sighted` is in no fold (verdict, first round).
- Recomputing it per event from the previous world doubles the sight work,
  which `FoldPrefix`'s doc comment names as the dominant cost (15-176 ms per
  eye).
- With one constructor, rule 2 rests on this single value, and item 4 tests
  it.

**D3. An actor the viewer holds is `gone` in two cases.**

- The world no longer has it.
- Or the world's actor differs in any field that only `ActorAdded` writes:
  name, module id, attributes, module data, or the resource names and maxima.

The status fields (resources' current values, conditions, controllers and
kind) are corrected rather than compared here.

*Forced by:*

- The owner's ruling: "an identical one is corrected".
- Gap G2: comparing only name, attributes and maxima left a viewer holding the
  wrong `module_id` (probe), which makes rule 3's first clause false.

The register row's text is written at the sort with this list (Before Task 1).

**D4. A correction follows the order the viewer's fold holds.**

- It removes conditions and revokes controllers in that order.
- When the kind changes and no grant carries it, it restates the first
  controller the fold holds.

*Forced by:*

- Every stream outside the three histories stays as it is.
- Probe: conditions B and A removed while unseen. Main and the fold's order
  both send B, A; sorting sends A, B. So sorting changes a history outside
  the three, and the fold's order changes frames only where main's
  world-ordered `belief` disagreed with the fold.
- Neither the corpus nor 12 `eventgen` seeds reaches such a correction, so
  Task 4 pins it with a test of its own.

**D5. The behaviour changes first, in `internal/gateway/project.go`; the code
moves after.**

*Forced by:* this keeps the move behaviour-neutral, so it is checked by an
unchanged gateway suite and goldens. The behaviour change is then reviewed as
a small diff against the whole existing suite. In the probe that suite passes
under (ii) unchanged.

**D6. Eyes cross the boundary without a role.**

- `perceive` takes either "the actors whose controllers include participant
  P" or "this one shoulder, when it is a party member".
- The gateway's `Projector` keeps the role switch:
  - DM and agent receive the event itself;
  - an unknown role is sent nothing;
  - a player is mapped to its controlled actors and its viewpoint is ignored;
  - a spectator is mapped to its shoulder.

*Forced by:*

- Item 1: `perceive` may not import `identity`.
- Ruling 2: the player's union of controlled actors stays until ticket 3.

**D7. The gateway keeps its names and signatures.**

These stay: `Viewer`, `NewProjector`, `Project`, `reperch`, `perchSequence`,
`viewerFor`, `canSee`. `perceive` takes the perch's sequence as an argument.

*Forced by:*

- SPEC-013 and SPEC-011 name `canSee`, `viewerFor` and `newSeat`.
- The ticket's "What it touches" item 4 promises that the other gateway test
  files change only where these do.
- Ticket 3 deletes them.

**D8. Tests move; they are not copied.**

- A test moves into `internal/perceive` when it can be written against
  `perceive` alone. That means no `identity`, `campaign`, `adventure`,
  `eventgen`, `rules` or server.
- Every test needing those stays in `internal/gateway`, and so does every
  golden, keystone, property or `FoldPrefix` test.
- A requirement row whose test moves is re-pointed in the same task.

*Forced by:*

- Item 6: gremlins runs a package's own tests, and coverage is measured per
  test binary.
- Item 1: test files are checked by arch-lint.
- A rule kept in two places drifts.

**D9. `perceive`'s tests fold frames with a local loop over `engine.Apply`.**

*Forced by:*

- They cannot import `campaign` (item 1).
- Precedent: `project_test.go` and `project_property_test.go` fold a viewer's
  frames this way today.

Rule 4 binds production code, so this is named as a decision for the review
to confirm or overrule.

**D10. Main's fold for item 5 is derived by hand, written down, and checked
against today's projector before anything changes (Task 3).**

- Task 3's commit, in which the test passes on main's projector, is the
  evidence that main leaves that fold. The report names it by hash.
- Later tasks may not edit those expected folds. The review checks for that
  with `git diff --stat` on `testdata/`.

*Forced by:*

- The golden discipline: no `-update` flag, derived by hand first.
- Main's projector is gone from the tree after Task 5.

**D11. The arch change is Task 1, landed with a package file that holds only
the package doc.**

- `check:coverage` and `check:comments` cannot be green for
  `internal/perceive` until it has measured tests and code (Task 10).
- Both run only under `task check`, never in the commit hook, so the window is
  inside the branch and never on main.

*Forced by:*

- The owner and coordinator: the arch change goes first.
- Probes:
  - `go-arch-lint` refuses a component with no directory.
  - It refuses an unattached package.
  - A doc-only file has about a 50% comment share.
  - Every package with Go files owes a coverage measurement
    (`list-measured-packages.sh`).

**D12. Rule 1's register row is held by a tier-1 test in `internal/perceive`.**

- The test parses the package's own files, test files included, with
  `go/parser`. No subprocess, so it stays tier 1.
- It refuses any import inside the module other than `contract`, `engine`,
  `sight` and `perceive`.
- `go-arch-lint` stays the enforcement.

*A preference, named as one.* It lets the row cite a check rather than a
reading. The cost is that the allowed list exists twice. A `**READING**`
evidence cell is the alternative if the review prefers it.

**D13. Item 4's test lives in `internal/gateway`.**

- The continuing side is `gateway.NewProjector`, the production path, fed from
  the first event.
- The resumed side is `perceive`'s constructor given the world at the prefix,
  then fed the `FoldPrefix` of the frames sent so far, plus its own frames,
  at each later event.
- The `eventgen` part is sampled: seeds 1-6, every 10th prefix for fixed seats,
  every 5th for the percher. Runtime is measured and reported, aiming at 30 s
  or less. `internal/gateway` is tier 2 on pre-push, and the unsampled probe
  took about 214 s.

*Forced by:*

- The test needs `FoldPrefix`, the goldens and `eventgen`.
- Putting it in `perceive` would need an `excludeFiles` entry, which is a
  widening (rule 2).

**D14. The mutation keys are re-pointed in Task 10, after the port's reviews
have settled.**

*Forced by:* SPEC-020, and the memory lessons on settling the review before
the gate and on keys moving.

## Gaps carried from the verdict

- **G1.** An identical re-add whose conditions or controllers cross the re-add
  leaves the fold equal to main's only as sets, at VTT-248's grain. Order and
  `AppliedSeq` differ.
  - Task 4 pins it at that grain.
  - The owner accepted it at sign-off on 2026-10-15: what can show is the
    order of a token's condition dots, and nothing outside tests reads
    `AppliedSeq`. The report records it.
- **G2.** Rule 3's field list. D3 decides it.

## Tasks

The tasks are in dependency order. Each one is one review.

### Before Task 1: allocate the requirement ids

This is the `requirements` skill, run after this plan was signed off. DONE
2026-10-15, as follows.

- **Four ids, not three.** VTT-311 is rule 1, VTT-312 rule 2 and VTT-313 rule
  3, with D3's field list. The sort also accepted VTT-314, "A projected seat is
  sent nothing for an event when the fold of what it was already sent fails",
  which D1 puts on the system and Task 4's internal test holds.
- **VTT-313 covers a removal the viewer saw as well,** "introduced afresh when
  the viewer no longer holds the id", so VTT-211's two halves both have a
  successor. `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh` and
  `TestAnIdReusedAfterAnUnseenRemovalFoldsAgain` will move from VTT-211 to
  VTT-313 in Task 4.
- **VTT-211 is withdrawn in Task 4, not here.** It is true of the code until
  Task 4 changes it, and VTT-313 is false of it until then. So Task 4 marks
  VTT-211 WITHDRAWN in the register's form (date, why, "Succeeded by
  VTT-313"), re-points the two tests, and gives VTT-313 and VTT-314 their
  evidence, in the commit that makes them true.
- The four new rows read `**OPEN — no test yet**` until their tests land.

**True when done:** `task check:requirements-chain` is green (it is: 314 rows).
**Gate:** pre-commit hook.

### Task 1: the `perceive` component (a gate change)

**Files:**

- `.go-arch-lint.yml`:
  - `perceive: { in: internal/perceive }`;
  - `perceive: { mayDependOn: [contract, engine, sight, perceive] }`;
  - `perceive` added to `gateway`'s list;
  - a comment giving the reason: the code leaves a component allowed eleven
    dependencies for one allowed four, and the gateway's added edge reaches
    only what it may reach already.
- `internal/perceive/perceive.go`: the package clause and a one-line doc
  pointing to SPEC-016.
- Not `campaign`: that edge is ticket 2's.

**True when done:** `go-arch-lint check` is green.

**Deliberate breaks.** Each is made in the working tree, seen red, and undone:

1. The package file without the yml edit: "not attached to any component".
2. A production import of `internal/store`, then of `internal/identity`: red.
3. A `_test.go` importing `internal/campaign`: red, because test files are
   checked.
4. An import of `internal/gateway`: red, because it is not in the list.

**Known red until Task 10:** `check:coverage` and `check:comments` for
`internal/perceive` (D11).

**Gate:** pre-commit hook (arch included).

### Task 2: rule 1 is held by a test

**Files:** `internal/perceive/deps_test.go` (D12), citing rule 1's id.
`docs/requirements.md`: rule 1's evidence.

**True when done:** the test passes and lists the files it read. A vacuity
guard fails if it read none.

**Deliberate break:** add `import _ ".../internal/store"` to a `perceive` file.
This test reds, and so does `go-arch-lint`.

**Gate:** pre-commit hook; `task check:requirements-chain`.

### Task 3: main's viewer fold for the three histories, pinned before anything changes

**Files:**

- `internal/gateway/history_fold_test.go` (new).
- `internal/gateway/testdata/history-fold/<history>/state.json`, each derived
  by hand, with a `why` written by a person in the test or beside the file.

**The histories.** Each is a log driven through `gateway.NewProjector` with a
spectator perched on `hero`:

1. An identical re-add while unseen.
2. A re-add with another name.
3. A re-add with another module id.
4. The condition-order history.
5. The restated-controller history.

The three histories of the ticket are 1, 4 and 5. Histories 2 and 3 pin
rule 3's afresh clause. Each starts the same way: a session; scene `s1`, 5x5;
party member `hero` with a token at (1,1); the spectator perched on `hero`;
an NPC `gob` with resource `hp` 5/5 and a token at (3,1), in sight. Then:

1. `TokenRemoved` of gob's token, `ActorRemoved gob`, `ActorAdded gob` with
   the same fields, and a new token of gob placed at (3,1).
2. As 1, with another name in the `ActorAdded`.
3. As 1, with another module id in the `ActorAdded`.
4. gob gets conditions A and B; leaves sight; the world removes A, applies C
   and applies A again; gob comes back into sight; leaves it; the world
   removes A and B; gob comes back into sight.
5. gob is granted to p1 and p2; leaves sight; p1 is revoked and granted
   again; gob is seen and leaves sight again; a grant to p2 changes its kind;
   gob comes back into sight.

**The check.** `FoldPrefix` of the frames sent, rendered with
`keystoneDump`, must byte-equal the hand-derived `state.json`.

**True when done:** green on today's projector, which is D10's evidence. No
production file changes.

**Deliberate breaks:**

1. Change one value in one `state.json`: red.
2. In `project.go`, skip the `withdraw` arm for a gone id: histories 2 and 3
   red, because the fold keeps the old name or module id. Undo.

**Gate:** pre-commit hook; `go test ./internal/gateway/ -run HistoryFold`;
`check:comments` on the new file (25% or less).

### Task 4: the memory becomes the viewer's fold (D1-D4, still in the gateway)

**Files:**

- `internal/gateway/project.go`:
  - `Projector` keeps the frames it sent.
  - A rebuild of every map but `sighted` from `FoldPrefix` of those frames and
    the world, at each `Project` and `reperch`.
  - D3's same-incarnation test.
  - `belief` taken from the fold.
  - The now-dead belief writes in `introduce`, `correct` and
    `snapshotSighted` removed.
  - A failed viewer fold withholds the event.
  - `Projector`'s doc comment rewritten. Its "never a snapshot" sentence
    becomes false.
- `internal/gateway/history_fold_test.go`: frame assertions added. The fold
  assertions and `testdata/` stay untouched.
  - History 1: a correction only, with no `ActorRemoved` or `ActorAdded`.
  - Histories 2 and 3: a bare `ActorRemoved`, then `ActorAdded`.
  - History 4: removals in the fold's order.
  - History 5: restates the fold's first controller.
- New cases:
  - Conditions B then A, both removed while unseen, are sent B, A. This pins
    D4 against sorting.
  - An identical re-add carrying conditions: the fold equals the world at
    VTT-248's grain (G1).
- `internal/gateway/project_internal_test.go`: one internal test seeds the
  sent frames with one that does not fold, and asserts that `Project` sends
  nothing. It cites VTT-314.
- `docs/requirements.md`: VTT-211 marked WITHDRAWN, succeeded by VTT-313;
  `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh` and
  `TestAnIdReusedAfterAnUnseenRemovalFoldsAgain` re-pointed from VTT-211 to
  VTT-313; VTT-313's evidence (those two and the history tests) and
  VTT-314's.

**True when done:**

- The whole `internal/gateway` suite passes unchanged (it did in the probe).
- The goldens are byte-identical.
- Task 3's folds still hold.

The verifier's probe built this shape in a scratch copy, as a rebuild of the
maps from the fold, a rebuild from a given fold and world, and a
same-incarnation test; it is not committed, and its probe hooks are not a
model.

**Deliberate breaks:**

1. Write `belief` from the world again: history 4 reds.
2. Sort the removals: the B, A case reds.
3. Drop the module-id comparison: history 3 reds.
4. Drop the name comparison: history 2 reds.
5. Treat every held id as gone: history 1 reds.
6. Remove the fold-failure guard: the internal test reds.

**Gate:** pre-commit hook; `go test ./internal/gateway/`;
`check:requirements-chain`; `check:comments`.

### Task 5: the decision moves to `internal/perceive` (behaviour-neutral)

**Files:**

- `internal/perceive/*.go` receives the decision:
  - `classify` and the verdict;
  - `look`, with D6's role-free eyes;
  - `transitions` and everything only it and `classify` call:
    `doorTransitions`, `doorSubject`, `squareAt`, `squareKey`, the sorted-set
    helpers, `sameSet`, `sceneSeenFor`, `objectInSight`, `noteTransitions`,
    `introduce`, `withdraw`, `correct`, `statusFrames`, `beliefOf`;
  - the Task 4 rebuild and D3's test;
  - `forwardable`;
  - a constructor taking the eyes and the world it starts from (D2);
  - an event entry taking the event, the viewer's fold and the world after;
  - a perch entry taking the eyes, the fold, the world and the sequence.
- `internal/gateway/project.go` keeps `Viewer`, `NewProjector` and `Project`
  (the role switch, `FoldPrefix` of the sent frames, the call into `perceive`),
  plus `reperch` and `perchSequence`.
- `internal/gateway/seat.go`: `canSee` asks `perceive`'s look. Its name and
  signature are kept, so SPEC-013 and SPEC-011 stay true.
- The two tests in `project_internal_test.go` that call moved code move to
  `internal/perceive`:
  - `TestEveryEnvelopePayloadArmHasAnExplicitRuling`;
  - `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable`. Re-point VTT-272.
- The fold-failure test stays in the gateway.
- `viewpoint_internal_test.go` uses `perceive`'s square key, or a local one.

**True when done:**

- Item 2's grep prints nothing.
- No other test file changes. Check with `git diff --stat HEAD --
  internal/gateway/*_test.go`.
- The whole gateway suite, the goldens, and Task 3 and Task 4 pass unchanged.
- The review compares the moved function bodies mechanically, for example by
  diffing each function's text across the two files.

**Deliberate breaks.** These show the moved checks still bite from their new
home:

1. A payload arm answering `unrecognised`: the arm test reds.
2. `forwardable` keeping `participant_id`: the field test reds.

**Gate:** pre-commit hook; `go test ./internal/...`;
`check:requirements-chain`.

### Task 6: rule 2, a decision started over from the fold (item 4)

**Files:** `internal/gateway/resume_test.go`, citing rule 2's id;
`docs/requirements.md` evidence.

**The histories:**

- every oracle seat, selected by `viewer.json` (role `spectator`, viewpoint
  non-empty), never by directory name;
- the `eventgen` seeds (D13);
- the three histories.

**What the test checks:**

- Both sides send equal frames (`proto.Equal`) for every later event.
- A per-seat vacuity guard: each seat must forward and resume at least once.

**True when done:** the test passes, and its runtime is recorded in the
report.

**Deliberate breaks:**

1. Start the resumed decision with an empty `sighted`: red.
2. Keep `gone` across events in `perceive`: the reused-id history reds.
3. Write `belief` from the world: the condition-order history reds.

**Gate:** pre-commit hook; `go test ./internal/gateway/`.

### Task 7: port the board tests

Scenes, `SceneSeen`, objects, doors, tokens and moves, from
`project_test.go`, into `internal/perceive`.

**Files:**

- `internal/perceive/board_test.go` (name free), written against `perceive`'s
  API and D9's fold.
- The moved tests are deleted from `project_test.go`.
- `docs/requirements.md`: every row citing a moved test, re-pointed. Find them
  by grepping the moved test names.
- Comment share stays at 25% or less. History goes to the report (rule 10).

**True when done:**

- `perceive`'s suite passes in tier 1.
- The gateway suite still passes.
- `check:requirements-chain` is green.

**Deliberate break:** one per area.

1. `objectInSight` admits a footprint one row too far: the object test reds.
2. `doorTransitions` skips its corrections: the door test reds.
3. `sceneSeenFor` drops `visible`: the `SceneSeen` test reds.

### Task 8: port the roster and status tests

Introductions, corrections, removals and `gone`, controllers and kinds, notes,
and the payload rulings, from `project_test.go`.

**What stays in `internal/gateway`:**

- the DM and agent passthrough and the unknown-role test, which need
  `identity`;
- the shipped fighter and archer tests, which need `adventure`;
- every server and campaign test.

**Files:** as in Task 7: every row citing a moved test, re-pointed, found by
grepping the moved test names.

**Deliberate breaks:**

1. `statusFrames` ignores conditions: the correction test reds.
2. `noteTransitions` keeps a secret key: the note test reds.
3. `classify` forwards `AttackRolled` unseen: the testimony arm reds.

### Task 9: port the testimony QA tests

The sources are `qa_testimony_eyes_test.go`, `qa_testimony_sight_test.go` and
`qa_testimony_removal_test.go`. They import `engine`, `gateway`, `identity`
and `sight`, so they are portable once `Viewer` is replaced by D6's eyes.

**Files:** `internal/perceive/qa_testimony_*_test.go`, the originals deleted,
and every row citing a moved test re-pointed.

**Deliberate break:** one per file, chosen from what that file's header says
it holds.

### Task 10: `internal/perceive` under the mutation gate and a coverage floor

**Files:**

- `tools/check-mutation.py`: add `"./internal/perceive/"` to the package list.
- `tools/mutation-equivalents.txt`: the three `internal/gateway project.go`
  entries are re-keyed to `internal/perceive <file>`, per SPEC-020.
  - Recompute `offset`, `nth` and `copies` against the code where it now
    stands.
  - Rewrite the capacity-hint entry's reason: it names `pr.seen`, and a gateway
    test as the killer of its sibling slice mutant.
  - Run the gate's two self-tests first.
- Mutants that survive: kill each one with a `perceive` test. Adjudicate only
  with a one-sentence observable, per the file's header.
- `tools/coverage-thresholds.txt`: an `internal/perceive` floor a little under
  its measured coverage, and at least 85.
- `tools/comment-ceilings.txt`: run `python3 tools/check-comments.py
  --write-ledger`, which lowers rows (`project.go`, `seat.go`, the thinned
  tests) and drops the rows of deleted files. Never raise one by hand.

**True when done:**

- `check:mutation` is green for `internal/perceive` and still green for
  `internal/gateway`. The gateway loses tests in Tasks 7-9, and the memory
  lesson "deleting a test deletes coverage you did not name" applies.
- `check:coverage` is green for both. The gateway measured 94.3% without
  `project.go`, against a floor of 88.5.
- `check:comments` is green.

**Deliberate breaks:**

1. Delete one adjudication: the gate reds and names that survivor.
2. Delete one ported test file: `check:coverage` reds for `internal/perceive`.

### Task 11: `task check`, whole

Check the load with `os.getloadavg()` first. Launch with
`start_new_session=True`. Do not touch the tree during the run.

**True when done:** green, which is item 7. Items 1-6 have each been seen.

### Then the process's Phases 4a, 4b and 5

- **The report** (`docs/reports/`) holds:
  - Task 3's commit hash, as the evidence for main's folds;
  - the probe measurements;
  - G1 and the owner's acceptance at sign-off;
  - the runtime of item 4's test;
  - the QA adjudications, under their own heading.
- **The specifications, after the report:**
  - **SPEC-016:**
    - Status: the `perceive` files.
    - "What the projector remembers" becomes "the viewer's fold, rebuilt at
      every event", and the "never seeded from a state" sentence is
      withdrawn.
    - `gone` per D3.
    - The correction's order and the restated controller per D4.
    - The re-add consequence and VTT-211's successor in the Requirements
      list.
  - **SPEC-015:** `canSee` asks `perceive`'s look, and the `Projector` keeps
    what it sent.
  - **SPEC-018:** the `TokenHidden`/`SceneSeen` builder and its `git grep`
    now name the `perceive` file.

## What this plan does not do

- It does not add `campaign → perceive` (ticket 2).
- It does not change what a connection subscribes to, or `pastResume`
  (ticket 3).
- It does not touch the client or contract comment pointers (ticket 4, by
  ruling).
- It does not change `.proto`, `engine` or `sight`.
