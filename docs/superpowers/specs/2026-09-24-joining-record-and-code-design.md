# The joining-a-table arc's decisions are one record, and its code carries only warnings

## The problem

The joining-a-table arc's decisions have two present-tense homes and a third
that narrates them. `docs/adr/011-identity-and-authorization.md` states them as
decisions; `internal/identity/identity.go` restates them in 400 comment lines of
898, and 26 more inside the schema's SQL literal, with the road to each: what
`participants.controls` used to be and why nothing migrates it away, what a
deferred transaction cost in trials, what the mutation gate found on which day.
`docs/reports/2026-08-09-joining-a-table.md` already carries that road, and its
section 6 says for each of sixteen blocks what must stay at the line. No
specification covers identity or joining; `docs/specifications/` holds 007 and
008.

`JoinAllows` in `internal/identity/identity.go` has no production caller:
`handleJoin` in `internal/gateway/join.go` calls `JoinAdmits`. It keeps a
22-line comment arguing a live security property, and five tests in
`internal/identity/identity_test.go` call it: two are twins of tests on the live
path (`TestAnEmptyStoredSecretAdmitsNobody` beside
`TestAnEmptyStoredSecretAdmitsNobodyThroughTheLivePath`,
`TestCheckingTheDoorMintsNothing` beside
`TestJoinAdmitsOnACampaignWithNoDoorRowRefusesWithoutCreatingOne`); two hold
rules of their own through the dead function
(`TestRotatingBeforeAnythingElseLeavesTheDoorSHUT`,
`TestTheDoorNeedsBOTHTheFlagAndTheSecret`); one asserts on it in passing
(`TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting`). Four comments in
`internal/gateway/join.go` and `join_test.go` name it.

Two pointers, in four places, resolve to nothing a reader can follow: `Verify`'s
doc comment and `TestVerifyUsesConstantTimeCompare`'s cite `task-3-brief.md`,
a file that exists only under gitignored `.superpowers/`; `Lookup`'s doc
comment and a comment above `s.ids.Lookup(p.ID)` in `internal/gateway/server.go`
quote 15.5µs, a measurement no benchmark in the tree produces.

Three of the register's thirty-seven rows are `OPEN`: a join refused at a shut
door writes nothing (VTT-008; the two sibling refusals have fault-armed tests,
this one does not), no event carries a role (VTT-035), a promotion appends no
event (VTT-036). Nine rules the arc's tests hold have no row, named under
`Rules this puts on the system`.

## Done looks like

1. `docs/specifications/009-identity-and-joining.md` exists with the five
   headings, a Status that names the implementing files and the pinning tests,
   and a Requirements section naming every row of the arc, VTT-005 to VTT-037
   and the rows this ticket adds; `task check:requirements-chain` is green with
   it. Today: no such file.
2. `grep -rn 'JoinAllows' internal/` prints nothing. Today: fourteen lines in
   four files.
3. `grep -rnE '15\.5|task-3-brief' --include='*.go' internal/` prints nothing.
   Today: four lines in three files.
4. `grep -cE '20[0-9]{2}-[0-9]{2}-[0-9]{2}|used to |an earlier version|previously|once shipped|turned out|[Mm]easured' internal/identity/identity.go`
   prints 0, and every comment block that remains says what the code cannot
   say about itself: a constraint, a reason a line is unusual, a warning to
   whoever edits next. The comment-line count of the file, before and after, is
   in the report. Today: the grep prints 17.
5. `TestAShutDoorRefusesWithoutTouchingTheDatabase` exists in
   `internal/identity/fault_internal_test.go`, cites VTT-008, passes, and fails
   when the door term is removed from `JoinAdmits`'s guard; VTT-008's row names
   it; `docs/verification-debt.md`'s entry dated 2026-09-23 says what closed it.
   Today: the row is `OPEN` and the entry is open.
6. A test holds "no event carries a role" and cites VTT-035, and a test holds
   "a promotion appends no event" and cites VTT-036; both rows name them. Today:
   both rows are `OPEN`.
7. The rules the sort accepts from the candidates below have rows, each citing
   the test named. Today: none has a row.
8. Still true afterwards: `go test -count=1 -p 1 ./internal/identity/...
   ./internal/gateway/...` is green; `TestRotatingBeforeAnythingElseLeavesTheDoorSHUT`,
   `TestTheDoorNeedsBOTHTheFlagAndTheSecret` and
   `TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting` pass, on
   `JoinAdmits`; `task check` whole is green.
9. Still true afterwards: `git diff --stat ad68765 -- docs/reports/2026-08-09-joining-a-table.md`
   prints nothing; a report is not revised for later work.

## Rules this puts on the system

Candidates for the sort, each with the check that holds it today.

- Rotating the link resets the admission budget.
  (`TestRotatingAfterASpentBudgetGivesAWorkingLink`)
- Rotating the link leaves the door as it was.
  (`TestRotatingTheLinkLocksOutTheOldOneAndNobodyElse`)
- Rotating the link on a campaign nobody has opened leaves the door shut.
  (`TestRotatingBeforeAnythingElseLeavesTheDoorSHUT`, on `JoinAdmits` after
  this ticket)
- The door admits only when it is open and the secret matches, and refuses the
  other three cases each for its own reason. (`TestTheDoorNeedsBOTHTheFlagAndTheSecret`,
  on `JoinAdmits` after this ticket)
- A database that cannot answer keeps the door shut.
  (`TestTheDoorRefusesWhenTheDatabaseIsUnusable`,
  `TestTheDoorStateReadFailingIsNotAnAdmission`,
  `TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting`)
- An admission that cannot be spent is not granted.
  (`TestAnAdmissionThatCannotBeSpentIsNotGranted`)
- A promotion is announced to the promoted participant.
  (`TestAPromotionIsAnnouncedToThePromotedPersonThemselves`)
- The join secret is stable until rotated. (`TestTheJoinSecretIsStableUntilRotated`)
- A token is never stored; only its hash is. (`TestTokenNotRecoverableFromDB`)

The three `OPEN` rows gain no rule; they gain a test each.

## What it touches

In the order they must change:

1. `internal/identity/fault_internal_test.go` (the shut-door test),
   `internal/gateway/server_test.go` (a promotion appends no event),
   `internal/gateway/authz_test.go` or a new test file beside it (no event
   carries a role): the tests first, red on the break, before anything else
   moves.
2. `internal/identity/identity.go`: `JoinAllows` and its comment gone; the
   sixteen blocks and the schema's SQL comment reduced to what stays; `Verify`'s
   and `Lookup`'s comments without the two pointers.
   `internal/identity/identity_test.go`: two tests gone, two re-pointed, one
   assertion removed, one comment shortened.
   `internal/gateway/join.go`, `internal/gateway/join_test.go`,
   `internal/gateway/server.go`: the comments naming `JoinAllows` and the
   measurement.
3. `docs/requirements.md`: the new rows; VTT-008, VTT-035 and VTT-036 with
   their tests. `docs/verification-debt.md`: the shut-door entry closed.
4. `docs/specifications/009-identity-and-joining.md`, last, after the code it
   describes is final.

Ten existing files and one new one, across identity, gateway, the register and
the records. Whether to split is asked at verification.

## Specifications this moves

New: identity and joining, the present tense of ADR-011 and of the code

## What could not be established

- Whether `docs/adr/011-identity-and-authorization.md` gets a one-line pointer
  to SPEC-009. `CLAUDE.md` says an ADR is never rewritten and that the present
  tense moves out to a specification; Patrik's single-source-of-truth ruling
  says one home. The first ADR read this way sets the form; the choice is
  Patrik's, asked at sign-off.
- How to hold "no event carries a role": a reflection walk over the
  `Envelope` payload's messages for a field named `role`, or a search of
  `internal/engine`; the plan decides, and the test's name is not known until
  it is written.
- Whether every one of the sixteen blocks' "stays" lists in the report of
  2026-08-09 is still what the code needs, one block at a time; that is the
  reading the sort of the comments consists of.
