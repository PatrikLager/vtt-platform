# Every rule an identity test holds has a row — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-25-identity-rules-have-rows-design.md`
**Verified:** 2026-09-25, by `verify-ticket`, an agent that did not write the
ticket, against `0dd0e65` on `chore/identity-rules-have-rows` (`main` at the
same commit). Verdict: **Passes with gaps.** Every path resolves and every
figure but one reproduced by command; the one that did not (a "26" for the
package's cited tests) was a count over three files reported as a count over
four, and the ticket was corrected to 29 after this verification, at
sign-off. The gaps are listed at the end and travel with this plan. This
plan does not edit the ticket.

**Goal, in the ticket's words:** every rule an identity test holds has a row,
and the test cites it.

**MapTool (CLAUDE.md rule 9), answered in one line.** No tabletop function is
designed; the ticket says so. Nothing to borrow.

## What the verification found

By command, at `0dd0e65`, unless marked as a reading.

- **The four test files hold 77 tests**: `identity_test.go` 56,
  `fault_internal_test.go` 13, `identity_failure_test.go` 5,
  `qa_joining_internal_test.go` 3, by `grep -c '^func Test'`. Reading the
  comment block directly above each `func Test` for a `VTT-NNN`: **48 cite
  none** (35, 8, 5, 0) and **29 cite one** (21, 5, 0, 3). The ticket's "48"
  reproduces. The ticket's "the package's other 26 tests" is the count over
  the three files the work touches (21 + 5); over the package's four it is 29.
  A figure, not a defect in the work; D13 records it.
- **`docs/requirements.md` holds 59 rows** (`grep -c '^| VTT-'`), and
  SPEC-009's Requirements line names **44** ids: VTT-005 to VTT-049 less
  VTT-035, the withdrawn one. Both figures reproduce.
- **None of the 48 names appears in any evidence cell** (`grep "#<name>"` over
  the register, each of the 48: nothing), and none appears anywhere under
  `docs/specifications/`.
- **SPEC-009 states three rules with no id**, at the sentences the ticket
  names: `ordered by display name and then id`; `gives a door that stands
  open with no budget the default`; `A stored role that does not parse is
  refused by Verify, Lookup and List`. Each holds against `identity.go`:
  `List`'s `ORDER BY display_name, id`; `migrateLocked`'s `UPDATE ... WHERE
  open = 1 AND admit_limit = 0`; the `ParseRole` error arm in each of
  `Verify`, `Lookup` and `List`.
- **Today's gates.** `task check:requirements-chain` prints `59 rows, 191 test
  files, 4 specifications; every citation resolves and every row's evidence
  holds`. `task check:comments` prints `237 files, 0 added comment lines, 237
  ledger rows; clean`. `git diff --stat tools/comment-ceilings.txt` prints
  nothing. `go test -count=1 ./internal/identity/` is green. The identity rows
  of the ledger: `fault_internal_test.go` 7.4 (measured 7.3), `identity.go`
  19.9, `identity_failure_test.go` 6.9 (measured 6.9), `identity_test.go` 2.5
  (measured 2.4), `qa_joining_internal_test.go` 12.2.
- **The one-line awk prints 48 names today** (it is the command in Task 2).
- **The token-stream instrument is blind to citation lines.** The sweep plan's
  Task 0 program (`go/scanner`, mode 0), rebuilt in the scratchpad: the four
  files scan to 8,974, 1,883, 692 and 1,200 tokens. A copy of each touched file
  with `// VTT-042` inserted directly above every uncited test (+35, +8, +5
  lines) scans to an identical stream, `cmp` silent. The control, `granted !=
  1` changed to `!= 2` in a copy, prints one differing line. So item 5's
  observation is live and item 5's instrument cannot be fooled by the work.
- **What the tests observe, by reading their assertions** (check 3):
  - `TestACampaignPredatingTheAdmissionBudgetStillWorks` asserts **one
    admission** through the migrated door and nothing about the budget's size.
    The default figure is asserted by `TestPreBudgetFixtureReallyDropsTheColumns`,
    which opens a pre-budget campaign whose door is open and requires
    `JoinBudget` to answer `0` spent and `DefaultAdmitLimit` allowed. The
    ticket calls that test a fixture control holding no rule; its assertion
    is the only one in the package that observes the repair's figure. D14.
  - `TestLookupCarriesTheWholeParticipant` compares `ID`, `Name` and `Role`,
    and `Participant` has exactly those three fields. "The whole participant"
    is observed.
  - `TestRotatingTheSecretInvalidatesTheOldLink` never calls `JoinAdmits`: it
    observes that `RotateJoinSecret` returns a secret different from the one
    before and that `JoinSecret` then returns the new one. VTT-019's
    observation, the old secret refused at the door, is not in it. D1 cites
    VTT-019 for the mechanism and flags it for sign-off.
  - `TestTwoInvitesProduceDistinctTokensAndIDs` is two `CreateInvite` calls
    with different names and roles; VTT-024 says two joiners through one link.
    The join path mints through `CreateInvite`, so the test holds the
    mechanism the row rests on. Flagged with the one above.
  - `TestMigratingTwiceIsNotAnError` opens a **fresh** file three times; on a
    fresh file the schema already carries the budget columns and no migration
    runs (the sweep report says the same, under "Deviations"). It observes that
    a repeat open of a current campaign succeeds, not that a migration is
    idempotent.
  - `TestMigrationSurvivesConcurrentFirstOpens` observes "migrated once" through
    the error channel: a second `ALTER TABLE ADD COLUMN` on the same column
    fails, and `Open` would return it, so four concurrent opens all succeeding
    is evidence that `migrateLocked`'s re-read under the lock did its job.
  - `TestJoinBudgetReportsWhatHasBeenSpent` asserts four states: zeros with no
    error on an untouched door, `0/3` after opening with 3, `1/3` after one
    admission, `0` spent after a close. The ticket's candidate names only the
    first.
  - `TestOpeningTheDoorFirstStillMintsARealSecret` also asserts that two
    campaigns' secrets differ; that is scenery for the row (one sample of
    randomness), not a rule.
  - `TestOpenRejectsFileThatIsNotADatabase` and
    `TestOpeningAnUnreadableCampaignFailsLoudly` are one observation twice
    (a non-SQLite file, `Open` errors, no handle). Not this ticket's to
    remove; a gap for the report.
  - `TestVerifyUsesConstantTimeCompare` reads `identity.go` as text; the debt
    entry in `docs/verification-debt.md` under "Open debt" says so. No rule.
  - The racing-first-reads rule is in the code: `ensureJoinRow` reads, then
    `INSERT ... ON CONFLICT(id) DO UPDATE SET secret = secret RETURNING
    secret`, so the loser of two first reads receives the winner's secret. No
    test isolates it.
- **Scope, by command then reading** (check 4). The five files the ticket
  names are the ones the chain touches, plus two the process adds: the
  implementation report (Phase 5), and one paragraph of SPEC-009 the ticket's
  item 3 does not mention. SPEC-009's **Status** paragraph lists the test
  files the rows' checks live in, and it does not list
  `internal/identity/identity_failure_test.go`; after this work five rows name
  that file, so the paragraph is false unless it gains the name. D7. Neither
  hook runs `check:comments` or `check:requirements-chain` (`.lefthook.yml`'s
  pre-commit runs lint, vet, `test:unit`, arch, invariants, doc-owner, staged
  secrets, typecheck and the review gate), so both gates run by hand before
  the commit and again in `task check`, and nowhere else. D10 and D11.
- **No recorded decision is overturned** (check 5, a reading). SPEC-008: ids
  by the dispenser only, tried on a copy first, a ticket carries no id, a
  specification copies its ids from the register — the ticket obeys all four.
  SPEC-009: its Requirements line grows and its "How it works" is unchanged;
  no rule in it was found false. SPEC-010 and VTT-059: a `//` line carrying
  only ids counts nowhere, which is what makes item 4 possible.
- **The records the work moves** (check 6): `docs/specifications/009-identity-and-joining.md`
  resolves, and the reading finds no other specification the work moves.

## Constraints that bind every task

- **CLAUDE.md rule 2.** `task check` is never weakened; no ceiling, band or
  gate moves. The ledger is not rewritten: citation lines move no share
  (VTT-059), and `git diff tools/comment-ceilings.txt` prints nothing.
- **CLAUDE.md rule 8.** Names, never `file:line`, in this plan, the register's
  rows, SPEC-009, the commit message and the report.
- **CLAUDE.md rule 10 and SPEC-010.** The only lines added under `internal/`
  are citation lines: `//`, one or more ids of the register's tag, spaces
  between, nothing else (the shape in `tools/check-comments.py`'s docstring
  and in the citation plan's D2). A word beside an id makes it a comment line
  and the ceiling refuses it in two of the three files (D9's B5).
- **SPEC-008.** Every id comes from `requirement-id`, run from the repository
  root, tried first on a copy of the register; none is chosen by hand; a row's
  evidence entry is `<repo-relative path>#<check name>`, entries separated by
  `, `; a citation is the bare id on the line above the check it holds.
- **The ticket's item 5.** No code line changes. The token-stream comparison
  (D6) is the check, and it runs before the commit.
- **The `requirements` skill.** Existing behaviour: each row is verified
  against `identity.go` before it is written (the symbol that holds it is
  named beside each row below); a rule no check holds gets `OPEN`, `READING`
  or no row, said out loud; the sort reports what it refused.
- **Phase 4b reads everything this work writes.** The rows and the SPEC-009
  lines are prose describing a codebase; the citation lines are not read for
  content (they have none) but for placement (SPEC-008's "where a citation
  sits is held by a reading").
- **Commits need a review record** (`review-gate` in `.lefthook.yml`); `git
  add` and `git commit` in separate calls.
- **`qa_joining_internal_test.go` is not touched**, and `identity.go` is not
  touched: its pointers carry ids inside worded warnings, which are comment
  lines, and the ticket's scope is citation lines in test files.

## Decisions this plan makes

**D1 — The sort's starting point is the three lists below, confirmed by the
reading after sign-off, and no id exists until then.** Forced by: SPEC-008
("the ids are allocated after its plan is signed off") and the `requirements`
skill (the sort runs over what the work did). The lists are the verifier's
reading of every assertion, so the sort after sign-off starts from a reading
of the tree rather than from the ticket's candidate sentences; where the two
differ the difference is named. Rows are lettered here so that nothing in this
file can be mistaken for an id.

**D2 — One row for the migration family.** Forced by: the skill's "one thing"
and "what, not how". The seven tests (`TestAMigrationThatCannot{BudgetAnOpenDoor,
Start,Commit,AddAColumn,ReadTheShape,ReadTheBudgetState}RefusesTheCampaign` and
`TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying`) observe one
thing, `Open` returns an error and no handle, through seven fault points. A row
per fault point would name the statement that failed, which is how the rule is
held, not the rule; and seven rows for one rule is the register the skill warns
against. The ticket's "rather than leaving it half-applied" is not observed by
any of the seven (none reads the schema afterwards; the rollback is the how),
so the row's sentence stops at "does not open".

**D3 — The racing-first-reads rule gets an OPEN row now, and no test.** Forced
by: item 5 (a test is a code line) and the skill's first outcome for a rule no
check holds ("a gap worth recording ... the absence is recorded as an
absence"). The row's evidence reads `**OPEN — no test yet**`, the report names
it (item 2), and a ticket for the test is the report's to raise. No entry in
`docs/verification-debt.md`: the row is the claim on future work, and a second
copy of it drifts. CLAUDE.md says that file "also holds known coverage gaps",
so sign-off question 3 asks whether the register row alone is that record here.
The debt file's own open entry, "A test that cites a row marked `OPEN` passes
the chain gate", binds the other way: the OPEN row must have no citer, and
Task 3's grep confirms it.

**D4 — Two ids on one line are allowed, and three already-cited tests gain a
second id.** Forced by: the shape exists (`// VTT-048 VTT-049` above
`TestTheDoorRefusesWhenTheDatabaseIsUnusable`), VTT-059's evidence holds it
(`test_two_ids_on_one_line_are_one_citation_line` in
`tools/check_comments_test.py`), and the checker's docstring names it. The
three: `TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo` gains row L's
id, because it is the only test that observes the display-name half of the
order (Ari before Zoe; the ties test observes only the id tie-break among four
Kims); `TestTheDoorRefusesWhenTheDatabaseIsUnusable` and
`TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting` gain row J's id,
because each asserts operations on a dead handle that no other test covers
(`JoinSecret`, `SetJoinOpen`, `Lookup`, `SetRole`; `JoinBudget`). An existing
id is never dropped or moved (the sweep's D5). The sweep's multiset check is
not reused, since the multiset grows by design; the token stream is the
invariant.

**D5 — A citation line sits directly above `func Test`, below any how-line.**
Forced by: the sweep's D5 (the file's own convention in every existing
citation line) and SPEC-008 ("in a comment on the line above the check it
holds"). Three of the 48 carry a how-line, and the id goes between it and
`func`: `TestAMigrationThatCannotBudgetAnOpenDoorRefusesTheCampaign`,
`TestOperationsFailAfterClose`, `TestMigratingTwiceIsNotAnError`. The warnings
inside test bodies (`// Keep this test: ...`) are not touched.

**D6 — Item 5's instrument is the sweep plan's Task 0 program, rebuilt.**
Forced by: the sweep plan's D11 (tokens, not `gofmt` output, because a printer
re-derives blank lines) and the measurement above (the scanner in mode 0 drops
every comment, so an added citation line is invisible to it, and an operator
change is one differing line). The listing is in
`docs/superpowers/plans/2026-09-24-sweep-identity.md`, Task 0; it is built
once in the scratchpad (`$S/codetokens/codetokens`) and run from the repository
root:

    for f in identity_test.go fault_internal_test.go identity_failure_test.go qa_joining_internal_test.go; do
      p=internal/identity/$f
      git show 0dd0e65:$p > "$S/base.go"
      "$S/codetokens/codetokens" "$S/base.go" > "$S/base.tok"
      "$S/codetokens/codetokens" "$p" > "$S/work.tok"
      echo "$f $(wc -l < "$S/base.tok") $(wc -l < "$S/work.tok")"
      cmp "$S/base.tok" "$S/work.tok"
    done

Done reads: four lines, each with two equal non-zero counts (8974, 1883, 692,
1200 at `0dd0e65`), and `cmp` silent. A run proves it ran by the counts.

**D7 — SPEC-009 changes in two places and no third.** Forced by: item 3 and
the Status paragraph's own sentence. The Requirements line gains every
allocated id, copied from the register (the `specification` skill's step 6:
"this step copies"). The Status paragraph gains
`internal/identity/identity_failure_test.go` among the files the rows'
checks live in, because five rows will name it and the paragraph claims to
list them. "How it works" is unchanged: the three id-less sentences were read
against `identity.go` and hold. Item 3 speaks of the Requirements line only;
the Status amendment is beyond its letter and inside its "Specifications this
moves", and sign-off question 5 asks for it.

**D8 — Phase 4a is skipped, and the report says why.** Forced by: the
dev-cycle skill's Phase 4a rule ("Skip only for tasks with no observable
behaviour to derive: ... prose") and D6's measurement. The change adds lines
the Go scanner drops, rows to a register and ids to a Requirements line; no
sentence in SPEC-009's "How it works" changes, so there is no spec sentence
for QA to derive a case from. The report records the skip under its own
heading with this reason.

**D9 — The deliberate breaks, one per check this work relies on, in a scratch
clone.** Forced by: the dev-cycle's rule that a check is proven by a red, and
item 4's observation being green today as well as after (its liveness needs a
red). `git clone --no-hardlinks` of the repository into the scratchpad, the
working tree's `git diff HEAD` applied there with `git apply` and the untracked
ticket copied in, then each break is one edit, the finding recorded verbatim,
the inverse edit, and `git diff --stat` printing nothing before the next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | `// VTT-999` directly above one uncited test | `check:requirements-chain`: `cites VTT-999 and no row in docs/requirements.md defines it (test citation)` |
| B2 | one new row's evidence entry re-pointed at a test in a file that does not carry the id (`internal/identity/qa_joining_internal_test.go#TestQAAShutDoorRefusalReachesNoWriteStatement`) | `check:requirements-chain`: `does not carry the id, so the link walks one way only` |
| B3 | one new row's evidence entry with a letter dropped from the test's name | `check:requirements-chain`: `declares no check named ...` |
| B4 | SPEC-009's Requirements line gains `VTT-999` | `check:requirements-chain`: `cites VTT-999 ... (specification citation)` |
| B5 | `// VTT-0NN holds this` in place of one citation line in `identity_failure_test.go` | `check:comments`: `internal/identity/identity_failure_test.go: comment share 7.8 is above its ceiling 6.9 and this change added a comment line to it (SPEC-010)` (8 over 103 non-blank; in `identity_test.go` one worded line sits under the 2.5 row's headroom and would not red, which is why the break is placed here) |
| B6 | `granted != 1` changed to `granted != 2` in `identity_test.go` | D6's loop: `cmp` prints `differ` for that file |

**D10 — One commit for the change; the implementation report in its own
commit.** Forced by: the chain is one link — a row's evidence names tests that
must carry the id in the same tree, and SPEC-009 names ids that must be rows —
and neither hook runs `check:requirements-chain`, so a split (register first,
citations second) would leave a commit that `task check` reds and nothing at
commit time notices. The commit carries the ticket (untracked today), this
plan, the register, the three test files and SPEC-009. The report follows per
Phase 5, after its own reading review. The pattern is `8ac4913` then
`8181d47`, and `1d139e9` then `0dd0e65`.

**D11 — The gate steps before the commit, in order, on a tree the review has
settled** (a run the review still edits under is wasted; this repository has
lost two forty-minute runs that way):

1. `gofmt -l internal/identity/` prints nothing; `go vet ./internal/identity/`.
2. `go test -count=1 ./internal/identity/`: green.
3. Task 2's awk: prints exactly one name, `TestVerifyUsesConstantTimeCompare`.
4. D6's token comparison: four identical pairs, non-zero counts.
5. `task check:requirements-chain`: `<59 + N> rows, 191 test files, 4
   specifications; every citation resolves and every row's evidence holds`.
6. `task check:comments`: `237 files, 0 added comment lines, 237 ledger rows;
   clean`; `git diff --stat tools/comment-ceilings.txt` prints nothing.
7. `task check` whole, item 6, launched detached in its own session (a
   background run here dies at thirty-eight minutes otherwise), the tree
   untouched while it runs. `check:new-prose` reads Go comments and finds only
   ids; `check:drift` has nothing to compare; the mutation gates run over Go
   that did not change.
8. Phase 4b's reading review, the review record, `git add` in its own call and
   `git commit` in the next.
9. `git push`: the pre-push hook runs about three minutes and is not killed
   under five.

**D12 — Evidence cells are written by hand after the dispenser, and one stays
OPEN.** Forced by: SPEC-008 (the dispenser writes `**OPEN — no test yet**`;
what replaces it is the sort's) and D3. Each entry is
`internal/identity/<file>#<TestName>`; a row with several tests lists them
comma-separated in the order the files are listed in the ticket. Row Q keeps
`OPEN`.

**D13 — The ticket's figure for cited tests was corrected by its writer at
sign-off, from 26 to 29.** Forced by: the verify-ticket skill (the verifier
does not edit the ticket) and the count by command: 29 tests in the package
cite an id at `0dd0e65`, of which 26 are in the three files the work touches
and 3 in `qa_joining_internal_test.go`. The report says so once.

**D14 — Where the ticket's two undecided tests go.** Forced by: their
assertions. `TestPreBudgetFixtureReallyDropsTheColumns` is evidence for row D
(it is the one test that observes the repair's figure), not a no-rule test as
the ticket proposes; its name says "fixture control" and its assertion says
"the door was open, so the repair should have budgeted it". `TestCoexistsWithStoreOnSameFile`
earns row P: it observes the decision SPEC-009 opens with (`identity.Open`
takes its own handle on the file `store.Open` uses), and no row states it. The
ticket lists both as "could not be established"; sign-off question 1 settles
them.

## The sort's starting point

For the `requirements` skill after sign-off. Each row is one sentence; the
symbol in `identity.go` that holds it is named so the sort's verify-against-
the-code step has its target; the tests are the evidence cell. Forty-two of
the 48 fall under a new row, five cite a row that exists, one has no rule.

### Tests that cite a row that exists (five)

| Test | Cites | Why, from its assertions |
|---|---|---|
| `TestJoinAdmitsOnACampaignWithNoDoorRowRefusesWithoutCreatingOne` | VTT-017 | `(false, nil)` and `count(*) FROM join_access` is 0: the row's own observation, at identity level |
| `TestSetRolePromotesTheNamedParticipant` | VTT-029 | after `SetRole`, `Verify(token).Role` is the new role: the "changes the role" half of the row |
| `TestLookupReflectsAPromotionImmediately` | VTT-029 | the same through `Lookup(id)`; "immediately" is VTT-031's, at the gateway, and identity has no cache for it to be a rule about |
| `TestRotatingTheSecretInvalidatesTheOldLink` | VTT-019 | the secret after rotation differs and is the one read; the test never reaches `JoinAdmits`, so it holds the row's mechanism, not its refusal. **Flagged**: the alternative is a row of its own, "Rotating the link replaces the secret, and the replacement is the secret read afterwards" |
| `TestTwoInvitesProduceDistinctTokensAndIDs` | VTT-024 | two `CreateInvite` calls give distinct tokens and ids; the row says joiners, who are minted through `CreateInvite`. **Flagged**: the alternative is a row, "Two invites are two participants with distinct credentials" |

### Tests that earn a new row (forty-two, sixteen rows)

| Row | Sentence, one line | Held by | Evidence |
|---|---|---|---|
| A | A campaign whose migration cannot be read or completed does not open. | `migrate`, `migrationPending`, `migrateLocked`, `Open`'s error arm | the six `TestAMigrationThatCannot...RefusesTheCampaign`, `TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying` |
| B | A campaign that needs migrating is migrated once, however many opens race for it. | `migrateLocked`'s re-read under `BEGIN IMMEDIATE` | `TestMigrationSurvivesConcurrentFirstOpens` |
| C | Opening a current campaign writes nothing to the file. | `migrate` returning before `db.Conn` when `migrationPending` is false | `TestOpeningACurrentCampaignTakesNoWriteLock`, `TestAnAlreadyMigratedReadOnlyCampaignStillOpens`, `TestMigratingTwiceIsNotAnError` (the weakest: it observes only that a repeat open succeeds) |
| D | A door left open before the budget existed is given the default budget by the migration. | `migrateLocked`'s `UPDATE ... WHERE open = 1 AND admit_limit = 0` | `TestACampaignPredatingTheAdmissionBudgetStillWorks` (admits), `TestPreBudgetFixtureReallyDropsTheColumns` (`0`/`DefaultAdmitLimit`) |
| E | The door reads as it was last set, across a close and reopen of the file. | `SetJoinOpen`'s upsert, `JoinOpen` | `TestTheDoorOpensAndClosesAgain`, `TestTheDoorSurvivesAReopen` |
| F | Opening the door works whether or not the link was read first, and a door opened first has a secret. | `SetJoinOpen`'s INSERT and CONFLICT branches | `TestTheDoorOpensOnACampaignThatAlreadyHasALink`, `TestOpeningTheDoorFirstStillMintsARealSecret` |
| G | Only dm, agent, player and spectator are roles; ParseRole, CreateInvite and SetRole refuse any other. | `ParseRole`; the `ParseRole` call opening `CreateInvite` and `SetRole` | `TestParseRoleAcceptsExactlyTheFourRoles`, `TestCreateInviteRefusesARoleThatIsNotOne`, `TestSetRoleRejectsARoleThatIsNotOneOfTheFour` |
| H | SetRole and Revoke report a participant that does not exist, and SetRole to the role already held does not. | the `RowsAffected` arm of `SetRole` and of `Revoke` | `TestSetRoleOnSomeoneWhoDoesNotExistIsAnError`, `TestRevokeUnknownParticipantErrors`, `TestSetRoleToTheSameRoleIsFine` |
| I | A stored role ParseRole refuses is refused by Verify, Lookup and List, never skipped or defaulted. | the `ParseRole` error arm in each | `TestVerifyFailsClosedOnInvalidStoredRole`, `TestVerifyRejectsEmptyStoredRole`, `TestLookupRefusesACorruptRow`, `TestListingRefusesACorruptRowRatherThanInventingARole` |
| J | Every operation on a closed or unreadable store reports an error and returns no result. | every method's error arm | `TestOperationsFailAfterClose`, `TestTheIdentityStoreReportsFailuresRatherThanPretending`, `TestListingRefusesWhenTheTableCannotBeRead`; second id on `TestTheDoorRefusesWhenTheDatabaseIsUnusable`, `TestTheJoinPathReportsDatabaseFailuresRatherThanAdmitting` (D4) |
| K | Opening a file that is not a database, or through a driver that cannot be opened, returns an error and no handle. | `Open`'s `sql.Open` and `db.Exec(schema)` arms | `TestOpenRejectsFileThatIsNotADatabase`, `TestOpeningAnUnreadableCampaignFailsLoudly`, `TestOpeningWithAnUnusableDriverIsReported` |
| L | Listing orders by display name then id, a total order. | `List`'s `ORDER BY display_name, id` | `TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder`; second id on `TestListingParticipantsShowsWhoIsHereAndWhatTheyMayDo` (D4) |
| M | Reading the budget reports what this opening has spent and allowed, and zeros with no error when the door was never touched. | `JoinBudget`, its `ErrNoRows` arm | `TestJoinBudgetReportsWhatHasBeenSpent` |
| N | Verify and Lookup answer nobody for a credential never issued or a participant revoked or unknown. | the `ErrInvalidToken` arms of `Verify` and `Lookup` | `TestVerifyRejectsWrongToken`, `TestRevokedTokenRejectedAfterRevoke`, `TestLookupRefusesARevokedParticipant`, `TestLookupRefusesAnUnknownParticipant` |
| O | Verify and Lookup answer the participant as stored: id, name and role. | the `Participant` each returns | `TestCreateInviteVerifyRoundTrip`, `TestLookupCarriesTheWholeParticipant` |
| P | Identity and the event log open the same campaign file at once, and each works while the other is open. | `Open`, independent of `store.Open` | `TestCoexistsWithStoreOnSameFile` |

### A rule no test holds (one row, OPEN)

| Row | Sentence | Held by | Evidence |
|---|---|---|---|
| Q | Two racing first reads of the link leave one secret. | `ensureJoinRow`'s `INSERT ... ON CONFLICT(id) DO UPDATE SET secret = secret RETURNING secret` | `**OPEN — no test yet**` |

### No rule (one test)

`TestVerifyUsesConstantTimeCompare`: it reads `identity.go` as text and passes
while the string appears anywhere in the file; the debt entry under "Open debt"
in `docs/verification-debt.md` carries it. No id; the report names it as the
one test item 1 excepts.

### Refused, and why

- "The join secret and the invite token are 32 bytes from `crypto/rand`,
  base64url" (the sweep report's candidate 2): a how and a measurement; no
  test holds it and none should.
- "Two campaigns do not share a join secret" (the second assertion of
  `TestOpeningTheDoorFirstStillMintsARealSecret`): one sample of randomness;
  scenery under F.
- "A second open changes nothing" (the ticket's first candidate, as worded):
  no test reads the schema after a second open; what is observed is B and C.
- "rather than leaving it half-applied" (the ticket's third candidate): the
  rollback is how A is held; the observation is that `Open` fails.
- "or is empty" (the ticket's ninth candidate): `ParseRole("")` errors, so an
  empty role is a role that does not parse; I's sentence covers it.
- "A promotion is visible at once" (the ticket's fifteenth candidate): the
  identity half is VTT-029's "changes the role", cited; "at once" is VTT-031's
  at the gateway; the "whole participant" clause is O.
- "`task check` whole is green": a status.
- "The report names each OPEN row": a sentence's presence, held by Task 7's
  grep.

## Tasks, in dependency order

### Task 1 — Sign-off, then the ids

Files: `docs/requirements.md`, by the dispenser only. After sign-off settles
the lists (questions 1, 2, 7, 8), for each row A to Q in order: `cp
docs/requirements.md "$S/register.md" && requirement-id --register
"$S/register.md" "<sentence>"` once to see the id it would give, then
`requirement-id "<sentence>"` from the repository root. Record each id beside
its letter in the session for Tasks 2 to 4.
**Done when:** `task check:requirements-chain` is green and prints `<59 + N>
rows`; `git diff docs/requirements.md` shows only appended rows, each ending
`| **OPEN — no test yet** |`; `grep -c 'OPEN — no test yet'
docs/requirements.md` prints N.

### Task 2 — The citation lines

Files: `internal/identity/identity_test.go`,
`internal/identity/fault_internal_test.go`,
`internal/identity/identity_failure_test.go`. Per D4 and D5: one line per
uncited test except `TestVerifyUsesConstantTimeCompare`, directly above `func
Test`, below any how-line; a second id appended to the existing line of the
three tests D4 names. Nothing else changes.
**Done when:** the one-line awk

    awk '/^func Test/{n=$2;sub(/\(.*/,"",n);if(b!~/VTT-[0-9][0-9][0-9]/)print FILENAME": "n;b="";next}/^\/\//{b=b $0;next}{b=""}' internal/identity/*_test.go

prints exactly `internal/identity/identity_test.go: TestVerifyUsesConstantTimeCompare`
(48 lines at `0dd0e65`); D6's comparison prints four identical pairs with
non-zero counts; `go test -count=1 ./internal/identity/` is green; `gofmt -l
internal/identity/` prints nothing; `python3 tools/check-comments.py main`
ends `0 added comment lines, 237 ledger rows; clean` and `git diff --stat
tools/comment-ceilings.txt` prints nothing.

### Task 3 — The evidence cells

File: `docs/requirements.md`, evidence cells only, by hand, per D12: the new
rows' cells from `OPEN` to their tests, and the five existing rows the first
list names (VTT-017, VTT-019, VTT-024, VTT-029) gain the citing test as one
more entry, so the register carries the reverse link SPEC-008 asks for.
**Done when:** `task check:requirements-chain` prints `every citation resolves
and every row's evidence holds`; `grep -c 'OPEN — no test yet'
docs/requirements.md` prints 1 (row Q); `grep -rn '<Q's id>' internal/`
prints nothing (the debt file's open entry: a citer of an OPEN row passes the
gate, so the absence is checked here); for each of the 47 cited names,
`grep '#<name>' docs/requirements.md` prints the row its letter or cited id
names, and for the three D4 names it prints one row more than at `0dd0e65`.

### Task 4 — SPEC-009

File: `docs/specifications/009-identity-and-joining.md`, per D7: the
Requirements line and the Status paragraph, read against the `specification`
skill's catches list.
**Done when:** `sed -n '/^## Requirements/,$p' <file> | grep -o 'VTT-[0-9]*' |
sort -u | wc -l` prints `44 + N`; `grep -c 'identity_failure_test.go' <file>`
prints at least 1; `git diff <file>` shows those two hunks and no other;
`task check:requirements-chain` is green.

### Task 5 — The breaks

Per D9, in the scratch clone carrying the tree Tasks 1 to 4 produce.
**Done when:** D9's table has a recorded finding per break, verbatim, matching
the expected column, and the clone's `git diff --stat` prints nothing at the
end.

### Task 6 — The gate, the review, the commit, the push

Per D10 and D11. The commit message names the ticket, this plan, SPEC-009, the
ids allocated (with Q marked OPEN), the one no-rule test, the five citations
of existing rows, B1 to B6 with the check that spoke for each, and ends with
the attribution lines.
**Done when:** `task check` exited 0 with both gates' completion lines in its
output; the review record matches the committed tree; `git log -1 --stat`
lists the ticket, this plan, `docs/requirements.md`, the three test files and
SPEC-009, and no other file; `git diff 0dd0e65 --stat -- internal/` shows
only the three test files with insertions and zero deletions except the three
lines D4 lengthens.

### Task 7 — The implementation report

File: `docs/reports/2026-09-25-identity-rules-have-rows.md`, per the
`implementation-report` skill, in its own commit after its own reading review.
It answers the ticket's six items in the ticket's numbering; names row Q as
the OPEN row and `TestVerifyUsesConstantTimeCompare` as the no-rule test; holds
the sort's three lists as they were confirmed, with what was refused; records
D13's corrected count, the duplicate observation (`TestOpenRejectsFileThatIsNotADatabase`
and `TestOpeningAnUnreadableCampaignFailsLoudly`), the weak third evidence of
row C, and Phase 4a's skip with D8's reason; and raises the ticket for Q's
test as a sentence, not a file.
**Done when:** `grep -c 'OPEN' <report>` is at least 1 naming Q's id; the
report's commit lists only that file; `task check:requirements-chain` is
unchanged by it.

## Sign-off questions

1. **The starting point (D1, D14).** Sixteen rows, one OPEN row, five
   citations of existing rows, one no-rule test — as listed. Four of them
   differ from the ticket's proposals or fill its blanks:
   `TestPreBudgetFixtureReallyDropsTheColumns` is evidence for D rather than a
   no-rule test; `TestCoexistsWithStoreOnSameFile` earns P;
   `TestRotatingTheSecretInvalidatesTheOldLink` cites VTT-019 though it never
   reaches the door (or earns its own row); `TestTwoInvitesProduceDistinctTokensAndIDs`
   cites VTT-024 though it is about invites (or earns its own row). Confirm,
   or say which move.
2. **One row for the migration family (D2).** Seven tests under A, one
   sentence. Agreed?
3. **The OPEN row (D3).** Q is recorded in the register and the report, with
   no entry in `docs/verification-debt.md` and no test in this ticket. Agreed,
   or does the debt file's "known coverage gaps" role want a copy?
4. **Second ids on three cited tests (D4).** The three lines become two-id
   lines. Agreed, or do already-cited tests stay as they are and rows J and L
   go without them?
5. **SPEC-009's Status paragraph (D7).** It gains
   `internal/identity/identity_failure_test.go`, beyond item 3's letter.
   Agreed?
6. **Phase 4a skipped (D8).** Agreed?
7. **Row H's second clause.** "and SetRole to the role already held does not"
   is the boundary of the first clause, held as one row here; the sort may
   split it into two. Which?
8. **Row M's sentence.** Broader than the ticket's candidate (which names
   only the zeros), because the test asserts four states. Keep the fuller
   sentence, or the ticket's?

## Gaps carried from the verification

- **The ticket's original "26"** counted the three files the work touches;
  the package's four hold 29 cited tests. Corrected in the ticket at sign-off
  (D13).
- **Item 4 is green today as well as after**, so its liveness rests on B5, not
  on a before/after difference. `identity_test.go` has headroom for one worded
  line under its 2.5 row; the break is placed in `identity_failure_test.go`,
  where one worded line reds.
- **Two tests are one observation** (`TestOpenRejectsFileThatIsNotADatabase`,
  `TestOpeningAnUnreadableCampaignFailsLoudly`); both cite K. Removing one is
  a code line and a ticket of its own.
- **Row C's third test is weak**: `TestMigratingTwiceIsNotAnError` opens a
  fresh file, so it observes a repeat open succeeding and nothing about
  writes. The other two hold the row; the third is kept as a citer because
  item 1 wants every test to point somewhere, and the report says which of
  the three is the weak one.
- **Rows for VTT-019 and VTT-024 gain identity-level evidence that holds the
  mechanism, not the refusal or the join.** Flagged in question 1; if
  sign-off prefers rows, N becomes 18 or 19 and Tasks 1 to 4 count from
  there.
- **`identity.go`'s own pointers** (`migrateLocked`'s warning could point at
  D's id; `ensureJoinRow`'s at Q's) are comment lines, not citation lines, and
  the ticket's scope is test files. Left as they are; a later sweep may add
  them.
- **The awk reads only `//` lines directly above `func Test`.** A citation
  placed inside a body, or above a helper, would not count; SPEC-008 leaves
  placement to a reading, and Phase 4b reads it.
