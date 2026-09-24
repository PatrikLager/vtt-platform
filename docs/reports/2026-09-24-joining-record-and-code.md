# Joining record and code: the arc's decisions are one record, and its code carries only warnings

**Ticket:** `docs/superpowers/specs/2026-09-24-joining-record-and-code-design.md`.
**Plan:** `docs/superpowers/plans/2026-09-24-joining-record-and-code.md`, verified
by `verify-ticket` (Passes with gaps); its seven sign-off questions were answered
with the recommendations on 2026-09-24.
**Last commit that changes code:** `37a714c`. The base is `8ebb2b1`, the branch's
tip before this ticket.

## The period, in commits

    git log --oneline 8ebb2b1..37a714c

    37a714c The arc's remaining rules are in the register, cited by their tests
    9920a67 identity.go says what its code cannot, and JoinAllows is gone
    9c31031 The three OPEN rows have their tests, and the fold's role-blindness has its own
    7537b7c The identity specification, and a defect writing it surfaced

`git diff --stat 8ebb2b1..37a714c`: `19 files changed, 2034 insertions(+), 448
deletions(-)`.

The gate: `task check`, whole, over the tree of `9920a67` before it was
committed: green, exit 0, every step; the Go mutation gate reported 14 packages
with zero unadjudicated survivors (seven mutants timed out and are counted as
killed without being measured; the run before this ticket had six), the TS
mutation gate 2882 mutants, 2783 killed, 29 timed out, 70 survivors all
adjudicated equivalent. `37a714c` changes only comment lines in code files, the
register and SPEC-009's Requirements list, so `9920a67` is the last tree that
changes a non-test `.go` file; it ran the steps that reach its files: `go test`
on identity, gateway and engine, `check:requirements-chain`, `check:new-prose`,
`check:doc-owner` and both mutation self-tests. The first two commits ran the
same reachable steps; the commit gate's nine pre-commit checks ran on all four.

## Done looks like, answered

1. `[~]` `docs/specifications/009-identity-and-joining.md` exists with the five
   headings; its Status names the five implementing files and the seven test
   files the rows cite; its Requirements names VTT-005 to VTT-034, VTT-036,
   VTT-037 and the twelve rows this ticket adds, VTT-038 to VTT-049. It does
   not name VTT-035, which the ticket's "VTT-005 to VTT-037" included and which
   is withdrawn (item 6); the chain gate holds the list as it is. `task
   check:requirements-chain` reports three specifications and every citation
   resolving. Its title reads "Identity lives beside the log, not in it": the
   reading review applied the specification skill's catch 14 to the draft's
   "Identity and joining".
2. `[x]` `grep -rn 'JoinAllows' internal/` prints nothing, from `9920a67`. The
   function, its doc and two twin tests are gone;
   `TestRotatingBeforeAnythingElseLeavesTheDoorSHUT` and
   `TestTheDoorNeedsBOTHTheFlagAndTheSecret` call `JoinAdmits`; the assertion
   in `TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting` is gone; the
   four gateway comments are reworded.
3. `[x]` `grep -rnE '15\.5|task-3-brief' --include='*.go' internal/` prints
   nothing, from `9920a67`.
4. `[x]` The item's grep on `internal/identity/identity.go` prints 0, from
   `9920a67`. Every remaining block was read against the rule by the Task 3
   reading review, block by block against the plan's table, which found no
   narrative block and two false sentences, both corrected before the commit.
   Comment lines before, at `8ebb2b1`: 400 `//` and 26 `--` of 898. After, at
   `9920a67`: 259 and 15 of 730. That commit's own message says 14 and 729, a
   count taken before the reading review's corrections added a line; the file
   is the measure.
5. `[x]` `TestAShutDoorRefusesWithoutTouchingTheDatabase` is in
   `internal/identity/fault_internal_test.go`, cites VTT-008, passes, and reds
   when `open != 1 ||` is removed from `JoinAdmits`'s guard: observed by the
   implementer, by the independent QA's own shut-door test under the same edit,
   and by the reading reviewer, who reproduced it. VTT-008's row names it;
   `docs/verification-debt.md`'s entry dated 2026-09-23 carries a `Closed by`
   line. Under that same edit `TestAClosedDoorSpendsNothing` stays green, as
   the entry said.
6. `[~]` The "no event carries a role" half changed shape. VTT-035 was false as
   worded: `Envelope.actor_role` is a role field on every envelope, stamped by
   the gateway and read by nothing that folds. Its row is withdrawn and keeps
   its id; two rows succeed it, VTT-038 "No event payload names a role"
   (`internal/gateway/authz_test.go#TestNoEventPayloadNamesARole`, a
   protoreflect walk over every message under the payload oneof, plus QA's
   `TestQANoEventPayloadNamesARole`, which also checks message and enum names)
   and VTT-039 "Folding an event yields the same state whatever role its
   envelope carries"
   (`internal/engine/role_test.go#TestTheFoldIgnoresTheEnvelopesRole`, plus
   QA's `TestQAFoldingIgnoresTheEnvelopesRole`, which reaches eleven of
   `Apply`'s arms to the implementer's six and every role; together they
   exercise twelve of twenty-three). The other half is as written:
   `TestAPromotionAppendsNoEvent` in `internal/gateway/server_test.go` cites
   VTT-036 and the row names it.
7. `[x]` The eight rules the sort accepted have rows, VTT-040 to VTT-047, each
   citing the tests the sort named, less the one VTT-048 took, and the reading
   review of those rows added two, VTT-048 and VTT-049; see "What the rules
   became". Five of the eight were reworded by that review before any was
   committed, and the register carries the final sentences.
8. `[x]` `go test -count=1 -p 1 ./internal/identity/... ./internal/gateway/...`
   is green; the three named tests pass on `JoinAdmits`; `task check` whole was
   green over `9920a67`'s tree, the last tree that changes a non-test `.go`
   file.
9. `[x]` `git diff --stat ad68765 --
   docs/reports/2026-08-09-joining-a-table.md` prints nothing.

## What the rules became

The nine candidates went through the `requirements` skill's sort after
sign-off; the record is under "The sort" below.

| Candidate | Became |
|---|---|
| Rotating the link resets the admission budget | VTT-044, reworded to the observation and, in review, bounded to an open door: at an open door, a link rotated after its budget is spent admits a holder of the new secret |
| Rotating the link leaves the door as it was | VTT-043, "open or shut as it was", holding three tests |
| Rotating on a campaign nobody has opened leaves the door shut | folded into VTT-043 as its no-row case |
| The door admits only when open and the secret matches, and refuses the other three cases each for its own reason | VTT-042, the admitting cell only; "each for its own reason" refused as unobservable |
| A database that cannot answer keeps the door shut | VTT-046 "admits nobody" and, split out in review, VTT-048 "reads as shut"; the two are held by different tests |
| An admission that cannot be spent is not granted | VTT-045 |
| A promotion is announced to the promoted participant | VTT-047 |
| The join secret is stable until rotated | VTT-040, narrowed in review to what the test observes: reading the join secret does not change it |
| A token is never stored; only its hash is | VTT-041, reworded twice: a participant's credential is stored as its hash |

The three OPEN rows gained tests, not rules; VTT-035 was withdrawn and
succeeded by VTT-038 and VTT-039, which the sort split out of the plan's one
successor sentence. VTT-049, "Rotating the link against a database that cannot
answer reports the failure", was no candidate: the reading review found two
tests holding it and no row stating it.

## QA adjudications

QA wrote ten tests in three files
(`internal/identity/qa_joining_internal_test.go`,
`internal/gateway/qa_joining_test.go`, `internal/engine/qa_role_test.go`), all
green, nine assertions redded by injection into the tests' own inputs. No
behaviour defect. Committed with the task.

- QA finding 1: spec ambiguity — "names a role" (VTT-038) did not say whether a
  field name, a message name, an enum, or a string value counts — SPEC-009 now
  says the rule is a field whose name contains `role` under the payload oneof,
  and that a string field carrying a role as a value is not decidable from the
  contract and not claimed.
- QA finding 2: spec ambiguity — "before any statement writes" did not say
  whether a lock or a BEGIN counts — SPEC-009 now says the refusing path runs
  the one SELECT and no other statement, so it takes no write lock and opens no
  write transaction (verified: `JoinAdmits` returns after its `QueryRow`).
- QA finding 3: out of scope — what `CommandResult.sequence` carries for a
  command that appends nothing is the wire contract's, SPEC-007; not amended
  here.
- QA finding 4: no change — "a campaign with no door row is refused without
  creating one" already has a row, VTT-017; QA's requirement file showed only
  the four rows under test.
- QA finding 5: tool doc defect, next ticket — `testdb.Arm` stacks by
  overwriting the one global arm, so a second `Arm` makes the first report
  tripped without having fired (`armed != f` in its closure). No test holds two
  arms live at once; the one test that calls `testdb.Arm` twice,
  `TestTrippedReportsWhetherTheFaultWasReached` in `internal/testdb`, reads the
  first before arming the second. The doc should say one arm is live at a time;
  `internal/testdb` is outside this ticket.
- QA finding 6: no change — the go doc QA received still shows `JoinAllows`,
  `15.5µs` and `task-3-brief.md`; those leave in Task 3, after this commit, as
  the ticket orders.
- QA finding 7: prose defect, fixed here — the gateway fixture's doc comment
  said "Sequences 1-4" while the seeded log's head is 5; corrected in
  `server_test.go` (a deviation: one comment line outside the ticket's named
  edits).

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket orders SPEC-009 last, after the code is final. | Written first (plan D1), commit `7537b7c`. | The sorted comments name it and the independent QA reads it; both need it to exist. Signed off. |
| Plan D14: the first commit carries SPEC-009 alone. | It carries the ticket, the plan and a debt entry too. | The ticket and plan are the signed-off contract; the debt entry records a defect the reading review of SPEC-009 surfaced, `announcePromotion` sending to every connection without consulting `revoked`. Patrik chose to fix it in the next ticket. |
| Plan D2: one successor row for VTT-035. | Two, VTT-038 and VTT-039. | The plan's premise that `engine.Apply`'s only input is the payload is false: it takes the whole envelope. The sort split the two obligations. |
| Plan D4: the fold's role-blindness stays a reading. | A test, `TestTheFoldIgnoresTheEnvelopesRole`. | A check can hold it and one written does not go red on correct work; the reviewer measured that it catches a consultation before the switch or in an exercised arm, and QA's sibling reaches more arms. |
| Plan D3: QA from three rows. | Four rows, the successor split in two; committed with three gate-driven edits. | The pre-commit gates refused them: staticcheck merged a declaration, the vocabulary gate refused "hp" in engine code (now "vigor"), and the arch gate refused identity's QA file importing `internal/testdb`; that file is excluded in `.go-arch-lint.yml` beside `fault_internal_test.go`, the same exception in the same shape. |
| The plan's table keeps "admitLimit is ignored when closing" at `SetJoinOpen`. | The doc says what the code does: the limit is written on every call and reported by `JoinBudget` until the next opening. | The upsert writes `admit_limit` unconditionally; the reviewer measured `SetJoinOpen(false, 5)` then `JoinBudget()` giving `0 5`. SPEC-007 said "ignored when closing" too and its one sentence was corrected in `9920a67`, a specification the ticket did not name. |
| The plan's table lists `RotateJoinSecret`'s "hands the DM a door that reads OPEN and admits nobody" under GOES. | Kept. | It is the warning to whoever removes the reset, not the history around it. |
| The ticket names only the `JoinAllows` lines in `join_test.go`. | One more sentence changed: "the exact shape a review caught in this session's previous change". | The paragraph was re-emitted by the rewording and the sentence names a workflow run that ended, rule 8's "this task" shape. |
| The gateway fixture's doc comment, outside the ticket's named edits. | "Sequences 1-4" corrected to five, naming the `ActorControlGranted` it omitted. | QA found the head at 5 before any command. |
| Plan Task 2's "`grep -rn 'VTT-008\|VTT-036' internal/` lists exactly the two new tests". | It lists QA's citations too. | QA's tests cite the rows they pin, as the QA prompt requires; the gate allows several files to cite one id. |
| Plan Task 4: eight rows, the sort's sentences. | Ten rows; five sentences changed before landing. | The reading review read each row against its test's assertions: VTT-044 was false without "at an open door", VTT-046 joined two obligations held by different tests, VTT-040 and VTT-041 claimed more than their tests observe, VTT-043 could be read as covering the spent count; and two tests held a rule no row stated. |
| Plan D12: `TestRotatingBeforeAnythingElseLeavesTheDoorSHUT` "refuses at the door term" on `JoinAdmits`. | It refuses on two terms at once on an untouched campaign, door shut and budget `0 >= 0`; the `JoinOpen` assertion above it is what catches a flipped `open` literal. | Measured by the reviewer; nothing is lost. |
| The stray comment block above that test, describing `TestCheckingTheDoorMintsNothing`. | Deleted with the test it described. | It named a function that no longer exists. |

## What could not be established

- Whether an `UPDATE` matching no rows takes SQLite's write lock, the premise
  of the three "writes nothing" rules and of the shut-door test's instrument.
  Carried from ADR-011 and the report of 2026-08-09; not measured here.
- What the two fold tests cannot catch: a role consultation inside an `Apply`
  arm neither exercises (eleven of twenty-three, measured by the reviewer), or
  in `campaign.foldEvents`. SPEC-009's `grep -rn ActorRole` sentence holds
  those by a reading.
- `testdb.Arm` stacks by overwriting the one global arm, so a second `Arm`
  makes the first report tripped without having fired. Found by QA; no test in
  the tree holds two arms live at once; the one that calls it twice,
  `TestTrippedReportsWhetherTheFaultWasReached`, reads the first before arming
  the second. The doc is `internal/testdb`'s and is the next ticket's.
- `cmd/vtt/joinlink.go`'s comment above its admissions line reasons that a shut
  door would show "0 of 0 left", so the line is printed only when the door is
  open. The premise is false: a shut door's `JoinBudget` reports nothing spent
  and the last written limit, measured by the reviewer as `0 5` after a close
  with 5 and `0 8` after a close with 0. The line's guard is right and its
  comment is not. Outside this ticket.
- `gofmt -l internal/` names `internal/gateway/keepalive.go` and
  `internal/gateway/scenario_test.go`, untouched by this ticket. Left as found.
- That the tree `task check` ran over is byte-identical to `9920a67`. The run
  started after the Task 3 reading review's corrections and ended before the
  commit, with nothing edited between by the implementer's own account; no
  fingerprint ties the log to the tree.

## What was deliberately left out, and where it went

- The `announcePromotion` defect against VTT-032: `docs/verification-debt.md`,
  entry dated 2026-09-24; the next ticket, by Patrik's decision.
- The form of a withdrawn row: a process ticket,
  `docs/tickets/a-withdrawn-row-has-a-form.md` in `~/dev/development_setup`,
  written 2026-09-24 and not yet committed there; committing it is that
  repository's own process. VTT-035's cell is the stopgap until it lands.
- `testdb.Arm`'s one-live-arm contract and the false premise in `joinlink.go`'s
  comment: named above; the next ticket.
- A one-line pointer from ADR-011 to SPEC-009: declined at sign-off; the ADR is
  frozen evidence and this report is the reverse link.
- `migrateLocked`'s unreachable re-read arm: `docs/verification-debt.md`, under
  Open debt, moved out of the comment at the arm.

## The sort

Source: the ticket's nine candidates under `Rules this puts on the system`,
plus the successor of VTT-035 (form A, signed off). Each test named was read
against the code before the rule was accepted. Rows are dispensed at the start
of the task whose commit carries them (plan D16): the successors at Task 2, the
rest at Task 4.

### Accepted, with the observation that goes red

1. VTT-040. The join secret is stable until rotated. Observation: reading the
   link twice gives the same secret.
   identity_test.go#TestTheJoinSecretIsStableUntilRotated

2. VTT-041. The store holds a credential's hash and not the credential.
   Observation: the persisted token_hash is sha256(token) and not the token.
   identity_test.go#TestTokenNotRecoverableFromDB ("never stored" reworded: the
   test observes the row, not every writer.)

3. VTT-042. A join carrying the current secret at an open door with budget
   remaining is admitted. Observation: the one admitting cell of the four.
   identity_test.go#TestTheDoorNeedsBOTHTheFlagAndTheSecret (on JoinAdmits
   after Task 3). The three refusing cells are VTT-005 and VTT-007's; "each for
   its own reason" REFUSED — every refusal is (false, nil) and no observation
   tells the reasons apart.

4. VTT-043. Rotating the link leaves the door as it was. Observation: the door
   reads the same before and after a rotation, open, shut, and on a campaign
   with no door row. identity_test.go#TestRotatingTheSecretLeavesTheDoorAlone
   (both states),
   identity_test.go#TestRotatingBeforeAnythingElseLeavesTheDoorSHUT (no row; on
   JoinAdmits after Task 3),
   server_test.go#TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse (open, over
   the wire; the test already carries VTT-020 and gains this id). The ticket's
   separate candidate "on a campaign nobody has opened leaves the door shut" is
   FOLDED IN: same rule, the no-row case; one row, three tests.

5. VTT-044. A link rotated after its budget is spent admits a holder of the new
   secret. Observation: open with budget 2, admit 2, rotate, the fresh secret
   admits. identity_test.go#TestRotatingAfterASpentBudgetGivesAWorkingLink
   ("resets the admission budget" is how; the admission is what.)

6. VTT-045. An admission whose spend cannot be recorded is not granted.
   Observation: the UPDATE fails, JoinAdmits answers (false, err).
   fault_internal_test.go#TestAnAdmissionThatCannotBeSpentIsNotGranted

7. VTT-046. A door whose database cannot answer admits nobody. Observation:
   JoinAdmits answers false on a closed handle and on an armed SELECT fault.
   (Reading review, Task 4: the first wording joined "reads as shut", held by a
   different test; split into VTT-048.)
   fault_internal_test.go#TestTheDoorStateReadFailingIsNotAnAdmission,
   identity_test.go#TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting
   (on JoinAdmits alone after Task 3). Distinct from VTT-033, which is the
   gateway's side of the same failure. The same tests also hold that the
   failure is REPORTED (err != nil) rather than answered as a plain refusal;
   that is how the gateway tells, and it is not stated as a rule of its own.

8. VTT-047. A promotion is announced on the promoted participant's own
   connection. Observation: a presence frame naming the promoted participant,
   by id and display name, arrives on their socket.
   server_test.go#TestAPromotionIsAnnouncedToThePromotedPersonThemselves

9. VTT-048. A door whose database cannot answer reads as shut. Observation:
   JoinOpen answers false on a closed handle.
   identity_test.go#TestTheDoorRefusesWhenTheDatabaseIsUnusable. Split out of
   VTT-046 by the Task 4 reading review.

10. VTT-049. Rotating the link against a database that cannot answer reports
    the failure. Observation: RotateJoinSecret on a closed handle returns an
    error; the test's own words: or a DM believes a leaked link was closed when
    it was not. identity_test.go#TestTheDoorRefusesWhenTheDatabaseIsUnusable,
    identity_test.go#TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting.
    Added by the Task 4 reading review: held by two tests, stated by no row.

Rewordings by the Task 4 reading review, before any row was committed: VTT-040
says what the test observes (two reads, one value); VTT-041 names the
participant's credential and its hash rather than "the store"; VTT-043 says
"open or shut" so the reset spent count is not read as the door; VTT-044 says
"at an open door", without which a shut-then-rotated door falsifies it. VTT-044
stays apart from VTT-019 because VTT-019's test cannot see a spent budget,
where VTT-043 folded the no-row case in because its tests do see it. VTT-046
lost `TestTheDoorRefusesWhenTheDatabaseIsUnusable` to VTT-048 in the split,
since that test never calls `JoinAdmits`.

### The successor of VTT-035

The signed-off sentence "No event payload names a role, and the fold's state
holds none" is TWO obligations, and the sort splits it. The plan's D4 argued
the second follows from the first because "engine.Apply's only input is the
payload"; measured 2026-09-24, `func Apply(st *State, env *vttv1.Envelope)`
takes the whole envelope, which carries `actor_role`, so the second does not
follow and is a rule of its own.

- VTT-038. No event payload names a role. Observation: a protoreflect walk over
  every message reachable from the Envelope's payload oneof finds no field
  whose name contains "role". authz_test.go#TestNoEventPayloadNamesARole (Task
  2).

- VTT-039. Folding an event yields the same state whatever role its envelope
  carries. Observation: the same payload applied under two actor roles gives
  equal states.
  `internal/engine/role_test.go#TestTheFoldIgnoresTheEnvelopesRole` (Task 2) —
  a test, not the reading D4 named, because a check can hold it and one written
  would not go red on correct work. Deviation from D4, recorded for the report.

VTT-035 is withdrawn: its row keeps its id, its requirement cell opens
`WITHDRAWN 2026-09-24` and names VTT-038 and VTT-039 as successors, and its
evidence cell reads `**READING — verify-ticket check 5 of 2026-09-24;
withdrawn, held by nothing**`, the plan's form A, which the gate accepts as a
READING; the process ticket named under "What was deliberately left out" asks
for a proper form.

### Refused

- "The door ... refuses the other three cases each for its own reason": not
  observable; every refusal is (false, nil). The refusing cells are VTT-005 and
  VTT-007.
- "Rotating the link on a campaign nobody has opened leaves the door shut" as
  its own row: the no-row case of VTT-043, folded in.
- "A token is never stored" as worded: "never" claims every writer; the test
  observes one row. Reworded as VTT-041.
- "Rotating the link resets the admission budget" as worded: how, not what.
  Reworded as VTT-044.
- "The fold's state holds none" as a clause of VTT-038: two obligations; split
  into VTT-039.
