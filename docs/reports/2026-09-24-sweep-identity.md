# internal/identity carries only warnings and pointers: the sweep

**Ticket:** `docs/superpowers/specs/2026-09-24-sweep-identity-design.md`.
**Plan:** `docs/superpowers/plans/2026-09-24-sweep-identity.md`, verified by
`verify-ticket`; its eight sign-off questions were answered with the
recommendations on 2026-09-24.
**Last commit that changes code:** `8ac4913`, on `e872467`, `main` at the time.
Every code reference below is to that tree.

## The period, in commits

    git log --oneline e872467..8ac4913

    8ac4913 Sweep internal/identity: warnings, pointers and doc sentences only

`git diff --stat e872467..8ac4913`: 8 files changed, 939 insertions(+), 764 deletions(-).

The gate: `task check`, whole, over the tree of `8ac4913` before it was
committed: exit 0, every step printing its own verdict, `check:comments`
ending `clean` and `check:mutation` reporting fourteen packages with zero
unadjudicated survivors. Before it, by hand and in the plan's order (D14):
`gofmt`, `go vet`, the package's tests, `check:comments`, `check:doc-owner`,
`check:requirements-chain`, `check:new-prose`, both mutation self-tests and
`task lint`; then the pre-commit hook's nine checks.

## Done looks like, answered

1. `[x]` `python3 tools/check-comments.py --report | grep internal/identity/`
   prints `banned 0` and `blocks>6 0` on all five lines. At `e872467` it
   printed 29 banned lines and 41 blocks over the bound; the per-file table
   below has the split.
2. `[x]` Above every `func Test` in the four test files there is at most one
   line saying how the test observes what its name states, then its `VTT-NNN`
   line or lines: the plan's `test-doc-blocks.py` prints `adjacent>2: 0,
   gate>2: 0` over the four files, from `adjacent>2: 30` at `e872467`. Eight
   tests carry a how-line, named under "What survives" below. The counter is
   the plan's instrument, listed under its Task 0 and not a tracked file;
   `grep -B3 '^func Test' internal/identity/*_test.go`, looking for a third
   comment line, reproduces the reading by hand.
3. `[x]` Every block left in `identity.go` was classified by the Phase 4b
   reading (VTT-051) as a warning, a pointer or the doc sentence of an exported
   symbol: of its 44 blocks, the package doc and 20 exported symbols' doc
   sentences, most with a warning or a pointer under them, and 23 warnings
   inside bodies or on unexported symbols, the `busy_timeout` warning plan D8
   keeps among them. `--report` shows `identity.go` at 19.9 percent, from
   38.2, under the ticket's 20.0; the reading governed and no true warning
   was cut for the figure.
4. `[x]` `tools/comment-ceilings.txt` lost four rows' height by `--write-ledger`
   in `8ac4913` (old to new: `fault_internal_test.go` 24.5 to 8.8,
   `identity.go` 38.3 to 19.9, `identity_failure_test.go` 29.2 to
   6.9, `identity_test.go` 22.8 to 4.0;
   `qa_joining_internal_test.go` stays at 12.7, untouched). `task
   check:comments` ends `clean` on that tree. The break the item asks for is in
   `8ac4913`'s message: one `// SPEC-009` line inside a function body of
   `identity.go`, in a scratch clone carrying the tree as `main`, refused with
   `comment share 20.0 is above its ceiling 19.9 and this change added a
   comment line to it`, naming the file.
5. `[x]` `go test -count=1 ./internal/identity/...` is green; `task check`
   whole is green (above); SPEC-009 gained five facts under "How it works",
   each named under "Sentences moved into SPEC-009" below with the symbol it
   describes, and nothing else in it changed.
6. `[x]` `git diff --stat e872467 -- docs/reports/` lists only this report;
   the two reports that hold the package's history are not revised.

## What the rules became

The ticket adds no rule and says so. The sort had nothing to sort; the work
runs under VTT-050, VTT-052, VTT-053 and VTT-055, held by `check:comments`,
and under VTT-051, held by the Phase 4b reading of this change.

## What survives, per file

Measured at `e872467` and at `8ac4913` with `python3 tools/check-comments.py
--report` (shares, banned lines, blocks over the bound) and with `measure` in
`tools/check-comments.py` (comment lines and block counts). Comment lines are
lines the gate counts; share is comment lines over non-blank lines.

| File | Comment lines | Share | Blocks | Over the bound | Banned lines |
|---|---|---|---|---|---|
| `identity.go` | 259 to 104 | 38.2 to 19.9 | 46 to 44 | 14 to 0 | 0 to 0 |
| `identity_test.go` | 380 to 53 | 22.7 to 3.9 | 79 to 41 | 22 to 0 | 20 to 0 |
| `fault_internal_test.go` | 94 to 28 | 24.4 to 8.8 | 20 to 18 | 4 to 0 | 7 to 0 |
| `identity_failure_test.go` | 39 to 7 | 29.1 to 6.9 | 5 to 4 | 1 to 0 | 2 to 0 |
| `qa_joining_internal_test.go` | 23 to 23 | 12.6 to 12.6 | untouched | 0 | 0 |

Token streams (go/scanner, comments dropped), before and after: `identity.go`
2742, `identity_test.go` 8974, `fault_internal_test.go` 1883,
`identity_failure_test.go` 692; each pair identical, so no code line changed.
Each test file's multiset of `VTT-NNN` citations is unchanged; `identity.go`
gained twelve pointers it had none of (Deviations).

**`identity.go`.** Forty-four blocks: the package doc; twenty doc sentences
on exported symbols, each followed where the code needs it by a warning or a
pointer; twenty-three warnings, on unexported symbols or inside bodies, of one
to three lines with a pointer where a row or a record holds the rule. Three
blocks went whole: the doc of `shapeReader`, of `migrateLocked` and of
`ensureJoinRow` (unexported, D2; the warnings they held are at the lines they
guard). Both trailing comments went, on `db.Close()` in `Open`'s error arm
and on `newSecret()` in `SetJoinOpen`. The doc sentences of
the exported symbols are one line each, with a second line where a fact would
be lost (`JoinAdmits`, `RotateJoinSecret`, `CreateInvite`, `ErrInvalidToken`,
`Revoke`).

**`identity_test.go`.** Thirty-eight doc blocks above tests: twenty-one
reduced to their `VTT-NNN` lines, sixteen cut whole (their tests have no row),
one replaced by a how-line alone (`TestMigratingTwiceIsNotAnError`); six carry
a how-line (`TestTokenNotRecoverableFromDB`,
`TestJoinIsClosedOnAnExistingCampaign`,
`TestTheDoorNeedsBOTHTheFlagAndTheSecret`, `TestOnlyOneJoinerTakesTheLastSlot`,
`TestAClosedDoorSpendsNothing`, `TestMigratingTwiceIsNotAnError`); the three
section banners naming a plan's task numbers went, and so did the 27-line
block about two deleted tests above `TestMigratingTwiceIsNotAnError`; of
forty-one body blocks,
seventeen became warnings of one or two lines above the assertion they guard
and twenty-four went; two warnings were added, at the assertions that hold
`SetJoinOpen`'s CONFLICT branch and its INSERT branch's secret, SQL text the
mutation gate cannot mutate; two trailing comments went, both claiming a
migration on a fresh file.

**`fault_internal_test.go`.** The file-level block became a three-line warning
(no parallel runs, no closed handle in place of the fault driver); the
twenty-line block about `withControlColumnCampaign` and four deleted tests went
whole, and one how-line stands above
`TestAMigrationThatCannotBudgetAnOpenDoorRefusesTheCampaign`; the docs of
`withFaultDriver` went and of `preBudgetCampaign` became a warning; of eleven
body blocks, ten became warnings of one or two lines and one went.

**`identity_failure_test.go`.** The 24-line block holding the file header
and `tamperRow`'s doc became a three-line warning (the two unreachable
branches not to test for; everything here must refuse), detached from
`tamperRow`, whose own doc went; the block about a
deleted test went; one how-line stands above `TestOperationsFailAfterClose`;
the two bare line ranges (into `identity.go` and `cmd/vtt/serve_compose.go`)
went with the blocks that held them. `// #nosec G202 -- ...` is verbatim.

## Sentences moved into SPEC-009

Five facts, three as new sentences and two as clauses in sentences that
existed. Each was in a comment and not in SPEC-009 at `e872467` (the second is
also VTT-048's rule); each is checked against the named symbol and re-read by
Phase 4b.

1. `JoinSecret` reads before it writes and `JoinBudget` never writes, so once
   the secret exists the console's poll takes no write lock: `ensureJoinRow`'s
   SELECT before its upsert, and `JoinBudget`'s single SELECT.
2. `JoinOpen` answers false when the database cannot be read: its error arm
   returns `false, err`.
3. `SetJoinOpen` writes the limit on every call, a close included, and
   `JoinBudget` reports it until the next `SetJoinOpen`: the upsert's `SET` list.
4. `Verify` finds the row by equality on the SHA-256 hash and confirms the
   match with `subtle.ConstantTimeCompare`: its `WHERE token_hash = ?` and the
   compare after the scan.
5. A stored role that does not parse is refused by `Verify`, `Lookup` and
   `List`: each one's `ParseRole` error arm.

## Candidate rows (D6)

No row was dispensed. Each line names the test that would be its evidence and
the observation that fails if the rule is broken; a later ticket sorts them.

1. Two racing first mints leave one secret, the loser's `RETURNING` giving the
   winner's value: `ensureJoinRow`; no test isolates it, so the evidence would
   be a new test.
2. The join secret and the invite token are 32 bytes from `crypto/rand`,
   base64url: `newSecret`, `CreateInvite`; no test holds the length.
3. A door opened before the budget existed still admits after the migration:
   `TestACampaignPredatingTheAdmissionBudgetStillWorks`; SPEC-009 states the
   repair and its default, no row does, and no test holds the count.
4. The door's state survives a reopen: `TestTheDoorSurvivesAReopen`.
5. `SetJoinOpen` opens an existing row, and when it creates the row it writes a
   non-empty secret: `TestTheDoorOpensOnACampaignThatAlreadyHasALink`,
   `TestOpeningTheDoorFirstStillMintsARealSecret`.
6. Promoting to the same role is not an error: `TestSetRoleToTheSameRoleIsFine`.
7. Listing orders by display name then id, a total order:
   `TestListingBreaksTiesOnIdSoTwoKimsHaveAFixedOrder`; SPEC-009 states it, no
   row does. An unreadable table gives an error and no list:
   `TestListingRefusesWhenTheTableCannotBeRead`.
8. The migration's properties, one row each or one row for the family: it is
   idempotent (`TestMigratingTwiceIsNotAnError`), survives concurrent first
   opens (`TestMigrationSurvivesConcurrentFirstOpens`), takes no write lock on
   a current campaign (`TestOpeningACurrentCampaignTakesNoWriteLock`), opens a
   current campaign on read-only media
   (`TestAnAlreadyMigratedReadOnlyCampaignStillOpens`), and refuses rather
   than half-applies when it cannot write
   (`TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying` and the six
   `TestAMigrationThatCannot...RefusesTheCampaign` tests in
   `fault_internal_test.go`).
9. `CreateInvite` refuses a role that is not one of the four:
   `TestCreateInviteRefusesARoleThatIsNotOne`.
10. Every operation on a closed or unusable handle reports an error and never
    panics: `TestOperationsFailAfterClose`,
    `TestTheIdentityStoreReportsFailuresRatherThanPretending`,
    `TestOpenRejectsFileThatIsNotADatabase`,
    `TestOpeningAnUnreadableCampaignFailsLoudly`.
11. `JoinBudget` on a campaign whose door was never touched answers zeros with no
    error: `TestJoinBudgetReportsWhatHasBeenSpent`.
12. A stored role that does not parse is refused: `TestVerifyFailsClosedOnInvalidStoredRole`,
    `TestLookupRefusesACorruptRow`,
    `TestListingRefusesACorruptRowRatherThanInventingARole`; SPEC-009 states it
    from this ticket, no row does.

## QA adjudications

Phase 4a was skipped (plan D15): the change has no behaviour, and the token
comparison above is the observation that holds that.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Plan D5: above a test, one how-line at most. | So; eight tests carry one, the rest only their `VTT-NNN` lines. | A name that states the rule needs no second line; the eight name what the test does that its name hides (a second raw handle, a hand-built schema, a four-cell walk, contended goroutines, a raw read of `admitted`, three opens of one file, a fault armed under the lock, a closed handle). |
| Plan D1: every surviving line a warning, a pointer or a doc sentence, written once. | Phase 4b found five surviving sentences false against the code or a probe, and eleven that described rather than warned; all rewritten or cut before the ledger was written. | Two falsities were the old text's, carried over (`vtt state dump` never opens identity; `BEGIN IMMEDIATE` succeeds on a read-only file, probed). Three were the sweep's own compressions: a how-line that named a mechanism the test does not reach (`ALTER TABLE` on a fresh file), a warning whose stated cause a probe disproved (a `WHERE`-shaped armed text still reaches the `UPDATE`), and a consequence that did not reproduce (`ALTER TABLE` with an open cursor). The "directory too" warning was cut after the test passed with the directory writable. |
| Plan's candidate list: five sentences for SPEC-009. | Five, one narrowed by the review: `JoinSecret` writes on its first call on an untouched campaign, so a poll takes no write lock once the secret exists, not before. | `ensureJoinRow`'s `INSERT ... RETURNING` runs when no row exists. |
| Plan D2: unexported symbols keep no doc sentence. | So; `identity.go` gained twelve `VTT-NNN` pointers it had none of, on warnings that guard a rule with a row. | A warning that names its row is a pointer, which the rule allows; the chain gate does not read `identity.go`, and all twelve ids resolve to rows. |
| Plan D12: the ledger written once, last. | Written once, after the review's fixes; `identity.go` measured 19.9 and `identity_test.go` 3.9, rounded up by `--write-ledger` to 4.0. | A block moved into a body during review changed no share; the review's cuts moved `fault_internal_test.go` from 9.1 to 8.8. |

## What could not be established

- `TestVerifyUsesConstantTimeCompare` passes on text a comment can supply
  (plan gap 4). Not introduced here and not fixed here, since fixing it changes
  a code line; recorded in `docs/verification-debt.md` under "Open debt" in
  this report's commit.
- Whether SPEC-010 reaches the fifteen `--` lines inside the `schema` string
  literal (plan gap 5). Left as the ticket requires; a question for its own
  ticket.
- Item 2's shape (id line plus one how-line) is an exit state: nothing holds
  it afterwards, since the gate allows six lines above a test (plan gap 7).
- Item 3's bound was met at 19.9, one tenth under 20.0; the reading, not the
  figure, decided every line, and nothing was cut to reach it.
- The reviewer's probes ran through `go test -overlay` in the session's
  scratch directory and are not in the tree; the two falsities they
  established (a read-only file accepts `BEGIN IMMEDIATE`; an `ALTER TABLE`
  succeeds beside an open `PRAGMA` cursor in this driver) are recorded here
  and in no test.

## What was deliberately left out, and where it went

- `qa_joining_internal_test.go`: clean at `e872467`, untouched, its row unmoved.
- The SQL `--` comments inside `schema`, the `// #nosec G202` directive and the
  `busy_timeout` warning at `ensureJoinRow`, which `internal/gateway`'s
  `server_test.go` points at: kept, plan D8.
- The next packages: one ticket each, `internal/gateway` after its
  specifications exist.
- The two reports holding this package's history,
  `docs/reports/2026-08-09-joining-a-table.md` and
  `docs/reports/2026-09-24-joining-record-and-code.md`: not revised.

## The sort

The ticket puts no rule on the system; nothing was proposed and nothing
refused.
