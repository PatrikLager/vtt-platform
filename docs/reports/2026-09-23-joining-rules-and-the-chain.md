# Joining rules and the chain: the arc's rules in the register, held by a gate

**Ticket:** `docs/superpowers/specs/2026-09-23-joining-rules-and-the-chain-design.md`.
**Plan:** `docs/superpowers/plans/2026-09-23-joining-rules-and-the-chain.md`, verified
by `verify-ticket` (Passes with gaps); its decisions were signed off with their
defaults on 2026-09-23.
**Last commit that changes the tree:** `fcf4ea3`. The base is `ad68765`, the
branch's tip before this ticket.

## The period, in commits

    git log --oneline ad68765..fcf4ea3

    fcf4ea3 The joining-a-table arc's rules are in the register, cited by their tests
    a8ea40b The chain gate: a cited id resolves to a row, and a row's evidence resolves to a check

`git diff --stat ad68765..fcf4ea3`: 15 files changed, 2032 insertions(+), 26 deletions(-).

The gate: `task check`, whole, run over the tree of `fcf4ea3` before it was
committed: green in every step. `check:coverage`: 20 packages at or above their floors. `check:requirements-chain`: 37 rows, 185 test files, 2 specifications. `check:new-prose`: 199 added lines across 13 files, all clean. `check:drift`, `check:breaking` (reporting, `contract/RELEASED` absent), `check:invariants`, `check:doc-owner`, `check:no-retraction`, `check:no-create-scene`, `check:no-pack`, `check:arch` and `check:race` clean. `check:mutation`: 14 packages, zero unadjudicated survivors, six mutants not evaluated and counted as killed. `check:ts-mutation`: 2882 mutants, 2783 killed, 29 timed out, 70 survivors all adjudicated equivalent, zero unadjudicated. The steps the first commit could reach were run before
it: `check:fast`, `check:new-prose`, `check:doc-owner` and
`check:requirements-chain`, all green.

## Done looks like, answered

1. `[x]` `grep -cE '^\| VTT-[0-9]{3} \|' docs/requirements.md` prints 37, one
   row per rule the sort accepted plus the one the reading review split out;
   every id was allocated by `requirement-id`; no evidence cell is blank or a
   sentence, which `check:requirements-chain` refuses.
2. `[x]` `grep -rlE '\bVTT-[0-9]{3}\b' internal tools` lists exactly the six
   files the rows name: `internal/identity/identity_test.go`,
   `internal/identity/fault_internal_test.go`, `internal/gateway/join_test.go`,
   `internal/gateway/authz_test.go`, `internal/gateway/server_test.go` and
   `tools/check_requirements_chain_test.py`; in each, the id sits on the last
   comment line directly above the check, and the three checks holding two rows
   carry both ids on that line. The reading review checked placement
   mechanically in both directions.
3. `[x]` `task check:requirements-chain` is a step of `task check`, between
   `check:doc-owner` and `check:new-prose`. On the real tree, each refusal was
   observed by injection and revert, the gate naming the offender each time: a
   test citing VTT-999 (`internal/identity/identity_test.go cites VTT-999 and no
   row ... defines it`); a specification naming it; a row whose evidence names a
   missing file, a file without the id, a check the file lacks; a blank cell; an
   id of another shape; two rows with one id; a READING naming no review. Then
   the other direction: a correct chain left the gate silent. With the rows in
   place, two more: a citation one above the highest row, refused naming the
   file; a cited check renamed in its entry, refused naming the row. Exit codes:
   0 clean, 1 findings, 2 nothing scanned.
4. `[x]` `python3 tools/check_requirements_chain_test.py -q` passes: 28 cases,
   one red case per refusal and per pass, fixtures tagged `TT`; it was red before
   the checker existed.
5. `[x]` SPEC-008's Status reads `Accepted. Implemented by ...`, naming the
   register, the dispenser, the checker, the Taskfile step and both test files;
   `grep -cE 'VTT-[0-9]{3}'` on it printed 0 until its Requirements section named
   VTT-001 to VTT-004, which are rows; `grep -c 'Nothing checks the chain here
   today' CLAUDE.md` prints 0.
6. `[x]` `go test -count=1 -p 1 ./internal/identity/... ./internal/gateway/...`
   is green; every test a row names exists as `func <name>(` in the file named,
   which the gate checks on every run; `TestARefusedJoinWritesNothingAtAll` and
   `TestAClosedDoorAndAWrongSecretAreRefusedIDENTICALLY` pass, each with one
   comment line added above it and nothing else changed.

## What the rules became

The ticket's four chain rules and its arc rules went through the `requirements`
skill's sort after sign-off; the record of the sort, with what it refused and
why, is below under "The sort". Thirty-seven rows:

| Id | Rule, as the register states it | Holds |
|---|---|---|
| VTT-001 | An id cited by a test or named by a specification resolves to a row in the register. | self-test |
| VTT-002 | A row's evidence names only files that exist, carry the row's id, and hold the check named. | self-test |
| VTT-003 | A row's id is the project tag and a number. | self-test |
| VTT-004 | No two rows share an id. | self-test |
| VTT-005 | A wrong-secret join is refused and writes nothing. | fault-armed test |
| VTT-006 | A spent-budget join is refused and writes nothing. | fault-armed test |
| VTT-007 | A shut-door join is refused. | two tests |
| VTT-008 | A shut-door refusal writes nothing. | OPEN |
| VTT-009 | The three refusals share status and body. | two tests |
| VTT-010 | A joiner is a spectator. | one test |
| VTT-011 | A refused join creates no participant. | three tests |
| VTT-012 | A display name is bounded and printable, and an ordinary one admitted. | two tests |
| VTT-013 | A refused name is refused distinctly from the door. | one test |
| VTT-014 | An oversized body is refused as malformed. | one test |
| VTT-015 | The door is shut until opened, pre-door campaigns included. | two tests |
| VTT-016 | Reading the link does not open the door. | one test |
| VTT-017 | A join on an untouched campaign creates no door row. | one test |
| VTT-018 | An empty stored secret admits nobody. | one test |
| VTT-019 | Rotation refuses the old secret and admits the new. | one test |
| VTT-020 | Rotation touches no participant already through. | two tests |
| VTT-021 | The budget is per opening. | one test |
| VTT-022 | No stated or a non-positive budget admits the default. | one test, the negative case added here |
| VTT-023 | Racing for the last slot, exactly one is admitted. | one test |
| VTT-024 | Two joiners are two participants. | one test |
| VTT-025 | Promotion targets player or spectator only. | three tests |
| VTT-026 | Promotion cannot unmake a dm or an agent. | one test |
| VTT-027 | A spectator cannot promote itself. | one test |
| VTT-028 | Door, rotation and promotion are DM-and-agent only. | one test |
| VTT-029 | Promotion changes the role and nothing else. | one test |
| VTT-030 | A revoked participant stays revoked through a promotion. | one test |
| VTT-031 | Promotion bites on the next command, no reconnect. | one test |
| VTT-032 | Revocation bites on the next command, event and presence frame. | three tests |
| VTT-033 | An unanswering identity store refuses, keeps the connection, delivery continues. | one test |
| VTT-034 | Revoked participants are not listed. | one test |
| VTT-035 | No event carries a role. | OPEN |
| VTT-036 | A promotion appends no event. | OPEN |
| VTT-037 | Promoting one participant changes no other's role. | one test |

The register carries each rule's full sentence and its evidence entries; the
column above abbreviates. The rows the sort refused, one line each, are in
"The sort" below.

## QA adjudications

QA ran once, per `qa-prompt.md`, with the requirement, SPEC-008, the ticket and
the checker's docstring; it never saw the checker's source or its self-test. It
wrote 35 tests in `tools/check_requirements_chain_qa_test.py`, all passing, 33 of
them shown red by injection, and listed sixteen underdetermined points. Each is
ruled on here, one line per finding, in the shape `qa-prompt.md` gives. None
escaped a gate into history: every ruling landed before the first commit, so
`docs/verification-debt.md` carries no recipe for them, and the plan's step
that asks for one had nothing to write.

- QA finding 1: spec ambiguity — SPEC-008 said "every test file under client/" and "every file under docs/specifications/" while the checker skips node_modules and its kin and reads *.md only — SPEC-008 now leaves the lists to the checker's docstring and states the decisions only.
- QA finding 2: behaviour defect — a row numbered zero passed while SPEC-008 says ids are counted from one — the checker refuses a zero number; a self-test case holds it; the self-test did not hold it before, and the reason is that the case was not thought of, not that a check was skipped.
- QA finding 3: spec ambiguity — which register defects are "nothing scanned" — SPEC-008 names them: no register, no project line, no header, no test file.
- QA finding 4: spec ambiguity — an id inside a string literal reads as a citation though the spec said "in a comment" — SPEC-008 and the docstring both say an id with the tag anywhere in a scanned file is a citation.
- QA finding 5: spec ambiguity — the citation's placement above the check is not gated — SPEC-008 says placement is held by a reading; the gate reads that the file carries the id.
- QA finding 6: spec ambiguity — "a row that nothing holds" fails the gate, yet an OPEN row passes by the spec's own instruction — SPEC-008's Consequences says an evidence entry that resolves to nothing fails; a test citing an OPEN row passes, recorded as a known gap in `docs/verification-debt.md`.
- QA finding 7: behaviour defect in the wiring — QA's test that runs `task check:requirements-chain` recursed without end once the step ran QA's file; first guarded through the environment, then dropped on the reading review's finding that the dry-run test already pins the wiring and `task check` runs the step itself; found by running the step, not by QA, and before any commit.
- QA finding 8: consistent — a Go check declared with a receiver is accepted; the docstring says so.
- QA finding 9: consistent — an absent `docs/specifications/` scans as zero specifications and passes; nothing to name yet.
- QA finding 10: consistent — entries separated by a comma with no space are accepted.
- QA finding 11: consistent — a whitespace-only review is refused as a READING that names none.
- QA finding 12: consistent — a bare `**OPEN**` is refused; the dispenser's exact cell is the answer.
- QA finding 13: consistent — a four-digit id passes, a one-digit id is refused; the dispenser pads to three and counts past 999.
- QA finding 14: consistent — near-miss ids around a tag do not read as citations; `XQAX-999` and `QAX-99` are data.
- QA finding 15: consistent — a trailing comma is refused as an entry naming no check; conservative.
- QA finding 16: consistent — a second table after prose is not part of the rows; the table is the one under the header.
- Reading review finding, after QA: a comma inside a TypeScript test's name split the evidence entry (283 client test names carry one) — entries now split on a comma that begins the next `path#Check`, a self-test case holds it, and SPEC-008 says so.
- Reading review finding, after QA: QA's dispenser test hardcoded the plugin's cache path and skipped when it moved — it resolves `requirement-id` on the path first and keeps the cache path as the fallback.
- Reading review finding, after the rows landed: QA's real-tree fixture injected rows carrying `<tag>-001`, written when the register had no rows; with thirty-six rows that id is taken and the gate refused the injected row as a duplicate, so two of QA's tests asserted the wrong reason — the fixture now computes the first free id from the copied register. Found by running the step after the rows landed; the gate was right, the fixture assumed an empty register that no record promised.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Plan D8: QA's tests wired into the step. | Wired, and one of them dropped. | QA's test that ran `task check:requirements-chain` recursed without end once the step ran QA's file; first guarded through the environment, then dropped on the reading review's finding that the dry-run test already pins the wiring and `task check` runs the step. |
| SPEC-008 and the checker's docstring each enumerate what is scanned and refused (plan Task 1 step 5). | SPEC-008 states the decisions and points at the docstring for the lists. | The reading review found the two already differing; one holds the lists, per Patrik's single-source-of-truth ruling. The docstring changes with the code and is what QA reads. |
| Commit 2 changes comments and the register only. | `TestADoorOpenedWithNoStatedBudgetStillAdmits` gained a negative-budget case. | The reading review found VTT-022's "non-positive" held by nothing and the guard's `== 0` mutant surviving; Patrik chose the test case over narrowing the row. |
| Thirty-six rows, the sort's count. | Thirty-seven. | The review found VTT-029's second test held a rule its sentence did not state; Patrik chose to split it, VTT-037. |
| VTT-033 and VTT-035 as the ticket worded them. | Reworded before landing. | "Only an invalid credential ends a connection" is a universal no test holds; "lives beside the credential" is how. Nothing was renumbered: no row had been committed. |
| `task check` whole once, before commit 2 (plan D13). | Run, stopped at the mutation stage, run again. | The first run was over a tree the review's fixes then changed; a gate over a tree that moves proves nothing, so it was stopped and rerun over the final tree. |
| QA's real-tree fixture injecting `<tag>-001`. | It injects the first free id. | With rows in the register the id was taken and the gate refused the injected row as a duplicate, the wrong reason for two of QA's tests. |

## What could not be established

- Whether an `UPDATE` matching no rows takes SQLite's write lock, the premise
  behind the three "writes nothing" rules. Asserted by ADR-011 and the debt
  entry; not instrumented.
- Whether the three OPEN rows can be closed by a test: the shut-door write
  needs the armed database fault with a shut door; "no event carries a role"
  a reflection over the Envelope's payload; "a promotion appends no event" a
  read of the log head across a promotion. Each is the second ticket's or
  later; none was tried here.
- `gofmt -l internal/` names `internal/gateway/keepalive.go` and
  `internal/gateway/scenario_test.go`, neither touched by this ticket; the lint
  gate does not run gofmt. Left as found.

## What was deliberately left out, and where it went

- The identity and joining specification, the sort of `identity.go`'s comments,
  the removal of `JoinAllows` and the shut-door test: the arc's second ticket.
- Six rules tests hold that the ticket did not carry, named in "The sort": the
  second ticket.
- A gate refusal for a test citing a row marked OPEN: `docs/verification-debt.md`,
  entry under Open debt, dated 2026-09-23.
- A pre-commit hook seat for the chain step: declined at sign-off; `task check`
  only.

## The sort

Per the `requirements` skill, over the ticket's candidates. Existing behaviour:
each accepted rule was checked against the test it names by the verification's
reading and by this sort's re-read of the assertions. Evidence entries are the
shape D3 fixes, `path#Check`; the ids are allocated in Task 3, in this order.

### Accepted, the chain (held by the gate's self-test once it exists)

- C1. An id cited by a test or named by a specification resolves to a row in the register.
- C2. A row's evidence names only files that exist, carry the row's id, and hold the check named.
- C3. A row's id is the project tag and a number.
- C4. No two rows share an id.

### Accepted, the arc's rules

- A1. A join request carrying a secret that does not match the campaign's is refused, and the refusal writes nothing.
  `internal/identity/fault_internal_test.go#TestAWrongSecretRefusesWithoutTouchingTheDatabase`
- A2. A join request arriving when the opening's admission budget is spent is refused, and the refusal writes nothing.
  `internal/identity/fault_internal_test.go#TestASpentBudgetRefusesWithoutTouchingTheDatabase`
- A3. A join request arriving when the door is shut is refused.
  `internal/gateway/join_test.go#TestAClosedDoorMintsNobody`, `internal/identity/identity_test.go#TestAClosedDoorSpendsNothing`
- A4. A join request refused at a shut door writes nothing.
  OPEN — a gap worth recording; `docs/verification-debt.md` entry dated 2026-09-23 names the check that would close it.
- A5. A shut door, a wrong secret and a spent budget are refused with the same status and the same body.
  `internal/gateway/join_test.go#TestAClosedDoorAndAWrongSecretAreRefusedIDENTICALLY`, `internal/gateway/join_test.go#TestTheDoorStopsAdmittingWhenItsBudgetIsSpent`
- A6. A participant minted through the join link is a spectator.
  `internal/gateway/join_test.go#TestJoiningThroughAnOpenDoorMintsASpectator`
- A7. A refused join creates no participant.
  `internal/gateway/join_test.go#TestAClosedDoorMintsNobody`, `internal/gateway/join_test.go#TestTheDoorStopsAdmittingWhenItsBudgetIsSpent`, `internal/gateway/join_test.go#TestAnOversizedBodyIsRefusedBeforeItIsRead`
- A8. A display name that is empty, longer than the bound, or carries a control, bidi or invisible-only content is refused, and an ordinary non-ASCII name is admitted.
  `internal/gateway/join_test.go#TestADisplayNameIsBoundedAndPrintable`, `internal/gateway/join_test.go#TestAnEmptyDisplayNameIsRefused` (the bound's figure lives in the test)
- A9. A refused display name is refused distinctly from the door's refusal.
  `internal/gateway/join_test.go#TestAnEmptyDisplayNameIsRefused`
- A10. A join request body larger than the cap is refused as malformed before it is read as a guess.
  `internal/gateway/join_test.go#TestAnOversizedBodyIsRefusedBeforeItIsRead`
- A11. A campaign's door is shut until the DM opens it, including a campaign that predates the door.
  `internal/identity/identity_test.go#TestJoinIsClosedOnAFreshCampaign`, `internal/identity/identity_test.go#TestJoinIsClosedOnAnExistingCampaign`
- A12. Reading the link does not open the door.
  `internal/identity/identity_test.go#TestReadingTheLinkDoesNotOpenTheDoor`
- A13. A join request on a campaign whose door has never been touched creates no door row.
  `internal/gateway/join_test.go#TestARefusedJoinWritesNothingAtAll`
- A14. An empty stored secret admits nobody.
  `internal/identity/identity_test.go#TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath`
- A15. Rotating the link refuses the old secret and admits the new one.
  `internal/gateway/join_test.go#TestRotatingTheLinkRefusesTheOldSecret`
- A16. Rotating the link touches no participant already through it.
  `internal/identity/identity_test.go#TestRotatingTheSecretLeavesParticipantsAlone`, `internal/gateway/server_test.go#TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse`
- A17. The admission budget is per opening: opening the door resets the count.
  `internal/identity/identity_test.go#TestABudgetIsPerOpeningNotPerCampaign`
- A18. A door opened with no stated budget, or a non-positive one, admits the default budget.
  `internal/identity/identity_test.go#TestADoorOpenedWithNoStatedBudgetStillAdmits` (the negative case was added in this ticket on the reading review's finding; before it the `<= 0` guard's `== 0` mutant survived)
- A19. Two joiners racing for the last admission: exactly one is admitted.
  `internal/identity/identity_test.go#TestOnlyOneJoinerTakesTheLastSlot`
- A20. Two joiners through the same link are two participants with distinct credentials.
  `internal/gateway/join_test.go#TestTwoJoinersGetDistinctIdentities`
- A21. A promotion may make a participant a player or a spectator and nothing else.
  `internal/gateway/authz_test.go#TestPromotionMayOnlyTargetPlayerOrSpectator`, `internal/gateway/server_test.go#TestPromotingToDMIsRefusedOverTheWire`, `internal/gateway/authz_test.go#TestAnAgentMayNotPromoteAnyoneToDMOrAgent`
- A22. A promotion cannot unmake a dm or an agent.
  `internal/gateway/server_test.go#TestPromotionCannotUNMAKEADMOrAgent`
- A23. A spectator cannot promote itself.
  `internal/gateway/authz_test.go#TestASpectatorCannotPromoteItself`
- A24. Opening or closing the door, rotating the link and promoting a participant are DM-and-agent only. (The ticket's candidate named the door and rotation; promotion is added because the same test holds it.)
  `internal/gateway/authz_test.go#TestAuthorizeTableAllCommandsAllRoles`
- A25. A promotion changes the role and nothing else about the participant.
  `internal/identity/identity_test.go#TestSetRoleDoesNotDisturbTheCredential`
- A25b. Promoting one participant changes no other participant's role.
  `internal/identity/identity_test.go#TestSetRoleLeavesEVERYONEElseAlone` (split out in the reading review: the second test held a rule the sentence did not state; allocated after the others as VTT-037)
- A26. A revoked participant stays revoked through a promotion.
  `internal/identity/identity_test.go#TestSetRoleOnARevokedParticipantStaysRevoked`
- A27. A promotion takes effect on the promoted participant's next command, without a reconnect.
  `internal/gateway/server_test.go#TestAPromotionBitesWithoutReconnecting`
- A28. A revoked participant is refused on their next command, their next delivered event and their next presence frame, without a reconnect.
  `internal/gateway/server_test.go#TestRevokingRemovesSomebodyWhoIsStillConnected`, `internal/gateway/server_test.go#TestARevokedSpectatorStopsSeeingTheTable`, `internal/gateway/server_test.go#TestARevokedWatcherIsNotEvenToldWhoElseArrives`
- A29. An identity store that cannot answer refuses the command, keeps the connection, and delivery continues.
  `internal/gateway/server_test.go#TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody`
  (reworded in the reading review: the ticket's "only an invalid credential ends a connection" is a universal no test holds, and the half that a revocation ends it is A28's)
- A30. Revoked participants are not listed.
  `internal/identity/identity_test.go#TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo`
- A31. No event carries a role.
  OPEN (reworded in the reading review from "lives beside the credential", which was how, to the observable) — a gap worth recording; held today by a search of `internal/engine` and by nothing under `task check`.
- A32. A promotion appends no event.
  OPEN — a gap worth recording; no test reads the log head across a promotion.

Thirty-seven rows, the last split out in the reading review. Each is breakable
and, but for three, names the test that goes red. The arc is the platform's one unauthenticated surface and its
authorization boundary, and the pre-ticket's rules were written one per
refusal; the count follows from that, not from a lenient sort.

### Refused, one line each

- "The joiner chooses no role": scenery of A6, the same rule seen from the caller's side.
- "The secret is compared in Go, in constant time": how; the what is A1.
- "Every refusal is decided from one read, before the UPDATE": how; the what is A1, A2 and A4.
- "There is no rate limiting": a decision about what is absent, not a rule the system obeys.
- "A count rather than a time window": how; the what is A17 and A18.
- "There is no upper bound on the budget": the absence of a rule.
- "Lookup costs 15.5 µs": a measurement, not a rule.
- "Promotion is a ClientCommand, not an endpoint": how; the what is A24.
- "A role change is an UPDATE to participants.role": how; the what is A31.
- "Delivery is where a watcher meets the server": a reason, not a rule.
- "Re-resolution happens outside the presence lock": how.
- "Presence frames carry no role": how, and not in the ticket's list.
- Seven candidates the ticket wrote as one were split, each half with its own observation: the shut-door rule (A3 and A4), the link-and-door rule (A12 and A13), the rotation rule (A15 and A16), the promotion-changes-only-the-role rule (A25 and A26, and A25b after review), the role-and-log rule (A31 and A32), the id rule (C3 and C4), and the display-name rule (A8 and A9). The revocation rule (A28) is kept as one property with three surfaces, as the pre-ticket states it.

Rules in the pre-ticket the ticket did not carry, left for the arc's second
ticket: the join secret is stable until rotated
(`TestTheJoinSecretIsStableUntilRotated`); tokens are never stored, only their
hashes (the invite arc's rule, `TestTokenNotRecoverableFromDB`).

Rules a test holds that the ticket did not carry, found by the reading review,
for the arc's second ticket: rotation resets the budget
(`TestRotatingAfterASpentBudgetGivesAWorkingLink`); rotation leaves the door open
(`TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse`); an unreadable database keeps
the door shut (`TestTheDoorRefusesWhenTheDatabaseIsUnusable`,
`TestTheDoorStateReadFailingIsNotAnAdmission`); an admission that cannot be spent is
not granted (`TestAnAdmissionThatCannotBeSpentIsNotGranted`); a promotion is announced
to the promoted person (`TestAPromotionIsAnnouncedToThePromotedPersonThemselves`).
