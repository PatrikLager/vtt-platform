# Every rule an identity test holds has a row: the change

**Ticket:** `docs/superpowers/specs/2026-09-25-identity-rules-have-rows-design.md`,
amended before work started on the one figure its verification found (29
cited tests, not 26).
**Plan:** `docs/superpowers/plans/2026-09-25-identity-rules-have-rows.md`,
verified by `verify-ticket` (Passes with gaps); its eight sign-off questions
were answered with the recommendations on 2026-09-25, row H split in two.
**Last commit that changes code:** `4886f9d`, on `0dd0e65`, `main` at the time.
Every code reference below is to that tree.

## The period, in commits

    git log --oneline 0dd0e65..4886f9d

    4886f9d Every rule an identity test holds has a row: VTT-060 to VTT-077

`git diff --stat 0dd0e65..4886f9d`: 7 files changed, 739 insertions(+), 10 deletions(-).

The gate: `task check`, whole, over the tree of `4886f9d` before it was
committed: exit 0, every step printing its own verdict, `check:comments`
ending `clean`, `check:requirements-chain` reading 77 rows, `check:new-prose`
clean over the 50 added lines, `check:mutation` reporting fourteen packages
with zero unadjudicated survivors. Before it, D11's steps by hand: `gofmt`,
`go vet`, the package's tests, the plan's awk, the token comparison,
`check:requirements-chain`, `check:comments` with the ledger diff; then the
pre-commit hook's nine checks.

## Done looks like, answered

1. `[x]` The plan's awk over `internal/identity/*_test.go` prints exactly two
   names, `TestVerifyUsesConstantTimeCompare` and
   `TestMigratingTwiceIsNotAnError`, the tests the sort and the review found
   hold no rule of their own; at `0dd0e65` it printed 48 names.
2. `[x]` `task check:requirements-chain` prints `77 rows, 191 test files, 4
   specifications; every citation resolves and every row's evidence holds`;
   seventeen new rows name tests under `internal/identity/`, and one, VTT-077,
   reads `**OPEN — no test yet**` and is named under "The rows" below.
3. `[x]` The Requirements line of `docs/specifications/009-identity-and-joining.md`
   names 62 ids (44 old, 18 new: `sed -n '/^## Requirements/,$p' | grep -o
   'VTT-[0-9]*' | sort -u | wc -l`); its "How it works" is unchanged, and its
   Status paragraph gained `identity_failure_test.go`, which four rows name
   for five of its tests (plan D7, sign-off question 5).
4. `[x]` `task check:comments` ends `237 files, 0 added comment lines, 237
   ledger rows; clean` and `git diff --stat tools/comment-ceilings.txt` prints
   nothing: the 46 added lines and the 4 extended ones are citation lines
   under VTT-059. Break B5 in `4886f9d`'s message is the observation that a
   worded line still counts.
5. `[x]` The go/scanner token streams of `identity_test.go` (8974),
   `fault_internal_test.go` (1883) and `identity_failure_test.go` (692) are
   identical to `0dd0e65`'s; break B6 shows one changed operator as `differ`.
6. `[x]` `task check` whole was green over `4886f9d`'s tree (above).

## What the rules became

| Candidate, as the ticket words it | Became |
|---|---|
| Opening a campaign migrates it once: a second open, or a concurrent one, changes nothing and reports no error | VTT-061 (migrated once under racing opens) and VTT-062 (a current campaign opens without writing); "changes nothing" refused, no test reads the schema after a second open |
| Opening a current campaign takes no write lock, so a current campaign on read-only media opens | VTT-062 |
| A migration that cannot complete refuses the campaign rather than leaving it half-applied | VTT-060, "whose migration state cannot be read, or whose migration cannot complete" (one of the seven opens a current file whose budget state cannot be read); "half-applied" refused as the how, the observation is that `Open` fails |
| A door left open before the budget existed still admits after the migration | VTT-063, with the default budget, which `TestPreBudgetFixtureReallyDropsTheColumns` observes |
| The door's state survives closing and reopening the file | VTT-064 |
| Only the four roles are accepted, by `ParseRole`, `CreateInvite` and `SetRole` | VTT-066 |
| The door opens whether or not the link was read first, and opening it mints a secret | VTT-065, reworded at review to the one rule both tests observe: the door and the link come in either order |
| An operation naming an unknown participant reports it rather than succeeding quietly; promoting to the same role is not an error | VTT-067 and VTT-068, split at sign-off (question 7) |
| A stored role that does not parse, or is empty, is refused by `Verify`, `Lookup` and `List` | VTT-069; "or is empty" refused, `ParseRole("")` errors so it is the same rule |
| Every operation on a closed or unusable store reports an error and returns no result | VTT-070, narrowed at review: `JoinOpen` reads as shut (VTT-048), which `TestTheDoorRefusesWhenTheDatabaseIsUnusable` asserts; `Close` on a closed store returns nil, which is `database/sql`'s and no test observes |
| Opening a file that is not a database, or with a driver that cannot be opened, fails | VTT-071 |
| Listing orders by display name then id, a total order | VTT-072 |
| Reading the budget of a campaign whose door was never touched answers zeros with no error | VTT-073, the fuller sentence (question 8): the test holds four states |
| A credential that was never issued, or was revoked, does not verify | VTT-074, with `Lookup`'s unknown and revoked ids |
| Lookup answers the participant as they are now: a promotion is visible at once, and the whole participant comes back | VTT-075 for the whole participant; "visible at once" refused, the identity half is VTT-029 and "at once" is VTT-031's at the gateway |
| Two racing first reads of the link leave one secret | VTT-077, OPEN |
| — | VTT-076, identity and the log share the file, from `TestCoexistsWithStoreOnSameFile`, which the ticket left undecided |

Refused besides: the 32-byte base64url secret (a how and a measurement), two
campaigns not sharing a secret (one sample of randomness), "`task check` whole
is green" (a status), "the report names each OPEN row" (a sentence's
presence). Five tests cite rows that existed: VTT-017, VTT-019, VTT-024 and
VTT-029 twice; VTT-019 and VTT-024 are held at identity level by the
mechanism (the secret changes; two invites are two credentials), the
gateway's tests hold the refusal and the joiners.

## The rows

| Id | Rule | Held by |
|---|---|---|
| VTT-060 | A campaign whose migration state cannot be read, or whose migration cannot complete, does not open. | `TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying`, `TestAMigrationThatCannotStartRefusesTheCampaign`, `TestAMigrationThatCannotCommitRefusesTheCampaign`, `TestAMigrationThatCannotAddAColumnRefusesTheCampaign`, `TestAMigrationThatCannotReadTheShapeRefusesTheCampaign`, `TestAMigrationThatCannotReadTheBudgetStateRefusesTheCampaign`, `TestAMigrationThatCannotBudgetAnOpenDoorRefusesTheCampaign` |
| VTT-061 | A campaign that needs migrating is migrated once, however many opens race for it. | `TestMigrationSurvivesConcurrentFirstOpens` |
| VTT-062 | Opening a current campaign writes nothing to the file. | `TestOpeningACurrentCampaignTakesNoWriteLock`, `TestAnAlreadyMigratedReadOnlyCampaignStillOpens` |
| VTT-063 | A door left open before the budget existed is given the default budget by the migration. | `TestACampaignPredatingTheAdmissionBudgetStillWorks`, `TestPreBudgetFixtureReallyDropsTheColumns` |
| VTT-064 | The door reads as it was last set, across a close and reopen of the file. | `TestTheDoorOpensAndClosesAgain`, `TestTheDoorSurvivesAReopen` |
| VTT-065 | The door and the link come in either order: read first, the door still opens; opened first, the link still has a secret. | `TestTheDoorOpensOnACampaignThatAlreadyHasALink`, `TestOpeningTheDoorFirstStillMintsARealSecret` |
| VTT-066 | Only dm, agent, player and spectator are roles; ParseRole, CreateInvite and SetRole refuse any other. | `TestParseRoleAcceptsExactlyTheFourRoles`, `TestCreateInviteRefusesARoleThatIsNotOne`, `TestSetRoleRejectsARoleThatIsNotOneOfTheFour` |
| VTT-067 | SetRole and Revoke report a participant that does not exist. | `TestSetRoleOnSomeoneWhoDoesNotExistIsAnError`, `TestRevokeUnknownParticipantErrors` |
| VTT-068 | SetRole to the role a participant already holds is not an error. | `TestSetRoleToTheSameRoleIsFine` |
| VTT-069 | A stored role ParseRole refuses is refused by Verify, Lookup and List, never skipped or defaulted. | `TestLookupRefusesACorruptRow`, `TestListingRefusesACorruptRowRatherThanInventingARole`, `TestVerifyFailsClosedOnInvalidStoredRole`, `TestVerifyRejectsEmptyStoredRole` |
| VTT-070 | Every operation on a closed or unreadable store reports the failure and returns no result, except Close, and JoinOpen, which reads as shut (VTT-048). | `TestTheIdentityStoreReportsFailuresRatherThanPretending`, `TestListingRefusesWhenTheTableCannotBeRead`, `TestTheDoorRefusesWhenTheDatabaseIsUnusable`, `TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting`, `TestOperationsFailAfterClose` |
| VTT-071 | Opening a file that is not a database, or through a driver that cannot be opened, returns an error and no handle. | `TestOpeningAnUnreadableCampaignFailsLoudly`, `TestOpeningWithAnUnusableDriverIsReported`, `TestOpenRejectsFileThatIsNotADatabase` |
| VTT-072 | Listing orders by display name then id, a total order. | `TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder`, `TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo` |
| VTT-073 | Reading the budget reports what this opening has spent and allowed, and zeros with no error when the door was never touched. | `TestJoinBudgetReportsWhatHasBeenSpent` |
| VTT-074 | Verify and Lookup answer nobody for a credential never issued or a participant revoked or unknown. | `TestVerifyRejectsWrongToken`, `TestRevokedTokenRejectedAfterRevoke`, `TestLookupRefusesARevokedParticipant`, `TestLookupRefusesAnUnknownParticipant` |
| VTT-075 | Verify and Lookup answer the participant as stored: id, name and role. | `TestCreateInviteVerifyRoundTrip`, `TestLookupCarriesTheWholeParticipant` |
| VTT-076 | Identity and the event log open the same campaign file at once, and each works while the other is open. | `TestCoexistsWithStoreOnSameFile` |
| VTT-077 | Two racing first reads of the link leave one secret. | `**OPEN — no test yet**` |

## QA adjudications

Phase 4a was skipped (plan D8): citation lines and register rows are prose,
and the token comparison holds that no behaviour changed.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Ticket: `TestPreBudgetFixtureReallyDropsTheColumns` holds no rule; `TestCoexistsWithStoreOnSameFile` undecided. | Evidence for VTT-063 and VTT-076. | The first asserts `0`/`DefaultAdmitLimit` after migrating an open pre-budget door, the one observation of the repair's figure; the second observes the decision SPEC-009 opens with. Found by the verification's reading of the assertions. |
| Plan: sixteen new rows and one OPEN. | Seventeen and one. | Row H was two obligations joined by "and"; the sort split it (sign-off question 7). |
| Plan: `TestMigratingTwiceIsNotAnError` is VTT-062's third evidence, "the weakest". | It cites nothing and the report names it beside `TestVerifyUsesConstantTimeCompare`. | It opens a fresh file three times on writable media and asserts only that no error comes back; with `migrate`'s early return deleted it stays green, so it cannot go red on the row. Found by the reading review. |
| Plan: `TestRotatingTheSecretInvalidatesTheOldLink` cites VTT-019 as the identity half. | It does, and `TestRotatingAfterASpentBudgetGivesAWorkingLink` (VTT-044) carries VTT-019 as a second id. | The first observes only that the secret changed; the second asserts the old secret refused and the new admitted after a rotation, VTT-019's rule at identity level. Found by the reading review. |
| Plan D13: the ticket's "26" held in corrected form, not edited. | The ticket was corrected to 29 at sign-off, and the plan's three passages now say so. | The plan landed describing a ticket that had moved; the plan was unlanded, so its sentences were made true rather than recorded as false. |
| Plan D12: evidence entries in the ticket's file order. | So, after the review found four rows out of that order. | A hand-written cell; no gate reads the order. |

## What could not be established

- VTT-077 has no test: `ensureJoinRow`'s `INSERT ... ON CONFLICT(id) DO
  UPDATE SET secret = secret RETURNING secret` gives the loser of a race the
  winner's secret, and no test races two first reads (`grep -n 'go func'
  internal/identity/*_test.go` finds three sites, none calling
  `JoinSecret`). An OPEN row, no debt
  entry (question 3): the register is the gap's one record. The test is a
  code change and its own ticket.
- `TestVerifyUsesConstantTimeCompare` cites nothing: it reads the source as
  text and holds no rule; `docs/verification-debt.md` carries it.
- VTT-070 generalises VTT-049 (rotating against a dead database reports the
  failure): both of VTT-049's tests now carry VTT-070 too, and VTT-049 is one
  arm of it. Left as two rows; a later sort may fold them.
- VTT-071's "through a driver that cannot be opened" names a seam only
  `TestOpeningWithAnUnusableDriverIsReported` can reach (`driverName` is
  package-private); it is breakable in `Open`'s `sql.Open` arm, and reads as a
  product condition it is not.
- VTT-064 is held by its two tests together and by neither alone: one
  observes the state within a handle, the other across a reopen.

## What was deliberately left out, and where it went

- A test for VTT-077: a later ticket.
- The next packages' rules: their own sweeps and sorts, `internal/gateway`
  after its specifications exist.
- No code line changed; no mutation key moved.

## The sort

Eighteen rows accepted with the observations above; the refusals are listed
under "What the rules became".
