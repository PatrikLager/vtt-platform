# Every rule an identity test holds has a row, and the test cites it

## The problem

`internal/identity`'s four test files hold 48 tests whose names state a rule
and which cite no requirement id (counted at `0dd0e65` by reading the
comment block directly above each `func Test`; the package's other 29 tests
cite one). `docs/requirements.md` holds 59 rows, SPEC-009 names 44 of them,
and none of the 48 tests appears in any row's evidence cell. So the rules
these tests hold are walkable in neither direction: nothing in the register
says a migration that cannot complete refuses the campaign, that a stored
role that does not parse is refused, that listing is a total order, or that
every operation on a closed store reports an error, and the tests that hold
those rules point at nothing. SPEC-009 states three of them in prose (the
listing order, the migration's door repair, the refused role) with no id to
name. Some of the 48 hold a rule the register already states from the
gateway's side, `TestRotatingTheSecretInvalidatesTheOldLink` for VTT-019,
`TestTwoInvitesProduceDistinctTokensAndIDs` for VTT-024,
`TestJoinAdmitsOnACampaignWithNoDoorRowRefusesWithoutCreatingOne` for
VTT-017, and cite nothing; a few hold no rule (`TestPreBudgetFixtureReallyDropsTheColumns`
is a fixture control, `TestVerifyUsesConstantTimeCompare` a check on text,
recorded as debt). The sweep of this package
(`docs/reports/2026-09-24-sweep-identity.md`, "Candidate rows") listed
twelve candidates and dispensed none; VTT-059 has since made a citation
line free of the comment share, which is what blocked the rows.

## Done looks like

1. Reading the comment block directly above every `func Test` in
   `internal/identity/*_test.go` finds a `VTT-NNN` on every test except the
   ones the implementation report names as holding no rule; the command is
   the one-line awk in the plan, and it prints 48 names today.
2. `task check:requirements-chain` prints more than 59 rows and `every
   citation resolves and every row's evidence holds`; every row the sort
   adds names at least one test under `internal/identity/` in its evidence
   cell, or reads `**OPEN — no test yet**` for a rule no test holds, and the
   report names each OPEN row.
3. The Requirements line of `docs/specifications/009-identity-and-joining.md`
   names every id the sort adds, and its "How it works" is unchanged unless
   the reading finds a rule it states falsely.
4. `task check:comments` ends `clean` and `git diff tools/comment-ceilings.txt`
   prints nothing: the added lines are citation lines and cost no share
   (VTT-059).
5. No code line changes: the go/scanner token stream of each of the four test
   files is identical to `0dd0e65`'s, the instrument the sweep's plan built.
6. `task check` whole is green.

## Rules this puts on the system

Candidates, one line each, for the sort after sign-off; a test whose rule the
register already states cites that row and earns none.

- Opening a campaign migrates it once: a second open, or a concurrent one,
  changes nothing and reports no error.
- Opening a current campaign takes no write lock, so a current campaign on
  read-only media opens.
- A migration that cannot complete refuses the campaign rather than leaving
  it half-applied.
- A door left open before the budget existed still admits after the
  migration.
- The door's state survives closing and reopening the file.
- The door opens whether or not the link was read first, and opening it
  mints a secret.
- Only the four roles are accepted, by `ParseRole`, `CreateInvite` and
  `SetRole`.
- An operation naming an unknown participant reports it rather than
  succeeding quietly; promoting to the same role is not an error.
- A stored role that does not parse, or is empty, is refused by `Verify`,
  `Lookup` and `List`.
- Every operation on a closed or unusable store reports an error and returns
  no result.
- Opening a file that is not a database, or with a driver that cannot be
  opened, fails.
- Listing orders by display name then id, a total order.
- Reading the budget of a campaign whose door was never touched answers
  zeros with no error.
- A credential that was never issued, or was revoked, does not verify.
- Lookup answers the participant as they are now: a promotion is visible at
  once, and the whole participant comes back.
- Two racing first reads of the link leave one secret.

## What it touches

1. `docs/requirements.md`, rows by the dispenser, evidence cells by hand
2. `internal/identity/identity_test.go`, citation lines only
3. `internal/identity/fault_internal_test.go`, citation lines only
4. `internal/identity/identity_failure_test.go`, citation lines only
5. `docs/specifications/009-identity-and-joining.md`, the Requirements line

One component; the register first, the citations and SPEC-009 with it, in
one commit. `qa_joining_internal_test.go` already cites its rows and is not
touched.

## Specifications this moves

docs/specifications/009-identity-and-joining.md

## What could not be established

- Which of the 48 tests cite an existing row and which earn a new one, and
  whether one row or several hold the six `TestAMigrationThatCannot...`
  tests and their two siblings: the sort's, after sign-off.
- Whether `TestCoexistsWithStoreOnSameFile`, `TestOpeningWithAnUnusableDriverIsReported`,
  `TestPreBudgetFixtureReallyDropsTheColumns` and
  `TestVerifyUsesConstantTimeCompare` state a rule at all; the sort says,
  and the report names the ones that do not.
- Whether the racing-first-reads rule gets an OPEN row or a test. A test is a
  code line change, which item 5 forbids; the plan decides between an OPEN
  row now and a ticket for the test.
- Whether a test may cite two rows on one line (`// VTT-048 VTT-049` is the
  shape VTT-059 accepts): the plan's.
- Rule 9 of CLAUDE.md does not apply: no tabletop function is designed.
