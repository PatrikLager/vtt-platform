# A campaign has one writer: the change

**Ticket:** `docs/superpowers/specs/2026-10-04-a-campaign-has-one-writer-design.md`,
revised by its writer after sign-off (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-04-a-campaign-has-one-writer.md`,
verified by `verify-ticket`.
**The owner's rulings:** at sign-off on 2026-10-05, every question as the plan
proposed: `flock` on the directory's own descriptor (Q1); `invite`, `revoke`
and `join-link` on a new `campaign.EnsureDir` and no `Campaign` (Q2 (i));
`GOOS=windows` no longer building, said in SPEC-019 (Q3 (a)); VTT-191's test
in this ticket (Q4 (a)); one code commit and the report (Q5); rows A to D
with eight refusals (Q6). On the review, on 2026-10-05: every finding fixed,
except that `Close`'s order and the admin test's dependence on the rollback
journal are recorded below as unobserved; QA's two tests that pinned an
absence dropped; QA's footprint test made to compare the directory while
held with the directory after `Close`; and, against the recommendation, a
row for the sort's eighth refusal (VTT-271). On this report's review, the
same day: the commit's message amended to correct a count, and the
cross-build gap recorded as open debt.
**Last code commit:** `855fafd`, on `4966408`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline 4966408..855fafd

    855fafd A campaign has one writer

`git diff --stat 4966408..855fafd`: 18 files changed, 2406 insertions(+), 94
deletions(-).

The gate, `task check` whole, once, on `855fafd`'s tree (`42c16cae`), which
was then the commit `991c40e`; the owner's ruling on this report amended its
message alone, from forty-two injections to forty-four, and the tree did not
change. It exited 0 with no step failed. Its check steps' own verdict lines
read `check:comments` clean over 258 files, `check:requirements-chain` 271
rows, 215 test files and 13 specifications, `check:doc-owner` 80 files,
`check:new-prose` 1662 added lines clean, `check:coverage` 20 packages at or
above their floors (`internal/campaign` at 89.3 % against 85.5),
`check:no-pack`, `check:no-retraction` and `check:no-create-scene` clean,
`task lint` 0 issues, `check:breaking` reporting pre-release with no
objection, `check:mutation` 14 packages with zero unadjudicated survivors
(`internal/campaign`, with none, and `internal/gateway`, with its four
adjudicated, mutated afresh; the other twelve reusing their verdicts; six
mutants timed out in `internal/sight`, `internal/rules` and `internal/mcp`
and counted as killed), and `check:ts-mutation`, verifying its stored report
since no client input changed, 2917 mutants, 2818 killed, 70 survivors all
adjudicated equivalent, zero unadjudicated, 29 timed out and counted as
killed. `check:mutation` began with 33.5 GiB free; `task check:drift` exited
0 after the commit.

## Done looks like, answered

1. `[x]` A second `campaign.Open` of a held directory is refused with an
   error that names the directory and says another writer holds it, in the
   same process or another:
   `internal/campaign/writer_hold_test.go#TestASecondOpenInTheSameProcessIsRefused`
   and `#TestAnOpenWhileAnotherProcessHoldsIsRefused`, both red on the tree
   without the hold (`err = <nil>, want campaign.ErrHeld`), and QA's
   `internal/campaign/qa_writer_hold_test.go#TestQAHoldOpenRefusesADirectoryHeldInThisProcessAtOnce`,
   `#TestQAHoldOpenRefusesADirectoryHeldByAnotherProcessAtOnce` and
   `#TestQAHoldOpenRefusalIsTheRecordedText`.
2. `[x]` Closing the first `Campaign` lets the next `Open` succeed, and so
   does the death of its process:
   `internal/campaign/writer_hold_test.go#TestClosingTheHolderLetsTheNextOpenSucceed`
   and `#TestTheHoldEndsWhenItsProcessIsKilled`, and QA's
   `internal/campaign/qa_writer_hold_test.go#TestQAHoldClosingACampaignEndsTheHold`
   and `#TestQAHoldEndsWhenItsHolderExitsWithoutClosing`. The kill test was
   red without the hold; the close half cannot be red where nothing holds,
   and K2 holds it.
3. `[x]` `vtt invite`, `revoke`, `join-link` and `art install` work against a
   directory a `Campaign` holds:
   `cmd/vtt/writer_hold_test.go#TestCommandsThatAppendNothingWorkWhileTheCampaignIsServed`,
   green at the base (the plan's P5, with a stub `ErrHeld`) and on
   `855fafd`, and red on the tree between them, which had the hold and not
   yet the `EnsureDir` switch (`vtt invite: open campaign: campaign: another
   writer holds ...`). Against a directory nothing holds,
   `TestCampaignDirectoryWorksInEitherCLIOrdering`, `TestInviteThenRevoke`,
   the `TestJoinLink` tests and
   `cmd/vtt/art_test.go#TestArtInstallCopiesFilesIntoAFlatArtDirectory` are
   unchanged and green. QA's
   `cmd/vtt/qa_writer_hold_test.go#TestQAHoldAdminCommandsHoldNothingWhileTheyRun`
   observes that `invite`, `revoke` and `join-link` take no hold while they
   run.
4. `[x]` `vtt serve` on a directory another `vtt serve` holds exits with the
   refusal and serves nothing:
   `cmd/vtt/writer_hold_test.go#TestASecondServeOnAHeldDirectoryExitsWithTheRefusal`,
   which also serves `/healthz` from a third `vtt serve` after the first is
   killed, and QA's
   `cmd/vtt/qa_writer_hold_test.go#TestQAHoldServeRefusalIsPrintedAfterItsPrefixExitsOneAndListensOnNothing`.
5. `[x]` `docs/verification-debt.md`'s entry is closed: it moved, unedited,
   to the dated section "2026-09-29 — two writers on one campaign directory
   could write a log that no longer opened", with a **Closed by** paragraph
   naming the two item-1 tests and the VTT-191 test and the breaks that red
   them (K1, K12). `task check` whole is green, as the gate paragraph above
   records.
6. `[x]` A seat sent an event whose fold fails withholds it and every event
   after it:
   `internal/gateway/viewpoint_internal_test.go#TestASeatIsSentNothingForAnEventWhoseFoldFails`,
   red under K12 and alone in `internal/gateway`. It was green on
   `4966408`, since the behaviour existed and only its test did not.

## What the rules became

| Ticket's rule | Became |
|---|---|
| At most one `Campaign` is open on a campaign directory at a time. | VTT-267, with rule 2; as a row of its own, refused: it is seen only as VTT-267's refusal |
| A second open of a held directory is refused, and says why. | VTT-267, with rule 1; as a row of its own, refused: it is how VTT-267 is observed |
| The hold ends when its holder closes or its process dies. | VTT-268 for the holder's half, VTT-269 for the process's |
| A command that appends no event does not need the hold. | VTT-270, narrowed to the four commands that take a campaign directory and append nothing |

VTT-271, that an `Open` refused for a held directory opens no log in it,
came from the owner's ruling on the review. VTT-191 gained its evidence and
no new row.

## The sort

Four rows accepted as D12 proposed (A to D, now VTT-267 to VTT-270), and
eight candidates refused. The eighth refusal, "a refused opener never opens
SQLite", gave as its reason that nothing observes it cheaply; QA observed it
with `TestQAHoldARefusedOpenerNeverOpensTheLog`, and the owner reversed the
refusal as VTT-271, held by that test and by K16. The other seven stand.

The mechanism stands on the plan's P1: of six candidates, only `flock`, on
the directory's descriptor or on a `writer.lock` file, refused a second
opener in the same process and in another, was released when its holder was
SIGKILLed, and left identity working; the directory was chosen because it
leaves no file (D1, Q1).

## Phase 4a: QA adjudications

**`855fafd` (VTT-191, VTT-267 to VTT-270), QA on opus, given those rows,
SPEC-019 and SPEC-015 whole, `go doc -all` of `internal/campaign` and
`internal/identity`, the `--help` of `vtt serve`, `invite`, `revoke`,
`join-link` and `art install`, and the fact that the package's `TestMain`
already holds `$VTT_CAMPAIGN_HOLDER_DIR`; it wrote
`internal/campaign/qa_writer_hold_test.go` and
`cmd/vtt/qa_writer_hold_test.go`.** Thirty-seven tests, none failing;
forty-four injections into its own files, all red (its hand-back counts
forty-two, but lists and ran forty-four), each file restored by its hash.
Thirty-five tests remain after the two dropped below.

- SPEC-015's last Consequences bullet still said nothing prevents two
  `Campaign`s on one directory: a spec defect the plan's D11 missed. It now
  binds appends to a `Campaign` and points at SPEC-019.
- VTT-191 could not be reached from the surface QA was given: expected, since
  the arm is unexported. It is held by the internal test D10 added. No change.
- An unknown payload variant is skipped with a warning and the `Open`
  succeeds, beside SPEC-015's "`campaign.Open` refuses a log that does not
  fold": not a contradiction, since a skipped variant folds. No change.
- The `--help` text QA was handed for `join-link`'s subcommands and for `art
  install` read `unknown command`: the implementer had passed each
  subcommand as one argument. QA built the binary and ran `--help` itself.
- Sentences QA could not observe: the plain-file refusal's text, whether
  `<dir>` is the path as given or cleaned, `Close`'s order, a lock failing
  with anything but `EWOULDBLOCK`, and the `client run`, `client soak` and
  `mintInvites` sentences. No change; "What could not be established" names
  the ones that matter.
- The fold case of a failed `Open` was not exercised by QA: it is
  `TestAnOpenThatFailsReleasesTheHold`'s "a log that does not fold" case,
  red under K3. No change.
- Whether `EnsureDir` creates missing parents is unobserved; it calls
  `MkdirAll`. No change.
- "Listens on nothing" is also held by
  `TestASecondServeOnAHeldDirectoryExitsWithTheRefusal`, which refuses a
  `listening on` line; a sub-second retry would pass QA's 1 s bound, and K8
  is what reds a wait. No change.
- The "no events" half of SPEC-019's Consequences sentence was ambiguous: it
  now says "no `events` table", and the table test asserts exactly that.
- QA's `GOOS=windows` build printed compiler diagnostics naming identifiers
  in `campaign.go`; it opened no source, and the test that ran it is
  dropped.
- Requirements QA asked an id for: fourteen SPEC-019 sentences. One became
  VTT-271 by the owner's ruling; the others are the sort's refusals, the
  rows' observations or no rule. None else dispensed.
- Dropped by the owner's ruling: `TestQAHoldCampaignDoesNotBuildForWindows`
  and `TestQAHoldAWriterWithoutACampaignIsNotStoppedAndTheNextOpenRefusesItsLog`,
  which pinned an absence no rule states and would red when a Windows hold
  or a guard against outside writers is added; the first also ran `go build`
  in every test run.
- The implementer also changed QA's files: fifteen citation lines rewrapped
  for `check:new-prose`; the footprint test rewritten, by the owner's ruling,
  to compare the directory while held with the directory after `Close` and
  to refuse any file but the log, so a later `maps/` from `Open` does not red
  it while a lock file still does (K15, K18); the table test made to assert
  no `events` table before `Open` and one after, rather than a count of two;
  the identity test moved to `cmd/vtt`, because `go-arch-lint` refuses
  `internal/campaign` importing `internal/identity`, even from a test; the
  build helper routed through `buildVTTBinary`, so `check:coverage` sees its
  subprocesses; and `TestQAHoldARefusedOpenerNeverOpensTheLog` cited as
  VTT-271.

## The breaks

The commit's message carries them, one line each, with the checks that
spoke; each was run in a scratch clone of the commit's final tree and
restored by its saved text, checked by hash, after each. Eighteen: the
`syscall.Flock` call removed; `Close` not calling `release`; the
fold-failure path keeping the hold; the refusal without the directory; the
refusal not wrapping `ErrHeld`; `O_CLOEXEC` dropped; `LOCK_SH` for
`LOCK_EX`; `LOCK_NB` dropped; `invite`, `revoke` or `join-link` back on
`campaign.Open`; `receive` forwarding an event whose fold failed; `art
install` opening a `Campaign`; the `store.Open`-failure path keeping the
hold; an `O_EXCL` lock file that `release` removes; `takeHold` after
`store.Open`; `Close` keeping `release`; and a `writer.lock` file flocked
and never removed. Every one went red, and none hung.

## Rule 9: how MapTool does this

Answered in the plan, from `~/dev/RPTool/maptool`: MapTool does not solve
this. No lock is taken on a campaign file; its guards are in-process only,
`AppState.backgroundTaskLock` and `MapTool.startServer`'s check of its own
`server` field, and `PersistenceUtil.saveCampaign` copies a temporary file
over the `.cmpgn`, so with two instances the last save wins whole, which a
snapshot survives. Borrowed: nothing. Refused: an in-process guard alone,
since our second writer is usually another process, and last-writer-wins,
since a log interleaves rather than replaces.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| D12: rows A to D, eight refusals | five rows, VTT-267 to VTT-271; seven refusals | QA observed the eighth refusal's rule, and the owner ruled it a row |
| D16: QA given SPEC-015's withholding paragraph | given SPEC-015 whole, which is how it found the stale Consequences bullet | the QA dispatch template requires the governing spec whole, never a section list |
| D16: QA given `go doc ./internal/identity` | given `go doc -all ./internal/identity` | the package index alone named no `DB` method, and QA's tests mint and list participants |
| D16: QA writes `internal/campaign/qa_writer_hold_test.go` and `cmd/vtt/qa_writer_hold_test.go` | QA's identity test now lives in the second | `go-arch-lint` refuses `internal/campaign` importing `internal/identity` |
| D10: the player seat "has received `SceneCreated` for `s`" first | it is sent a `NarrationAdded` first, and the fresh seat is sent the same narration at the sequence the broken seat withheld | a player seat is sent no `SceneCreated` before it sees anything, so the plan's precondition failed |
| D11: SPEC-015 changes one sentence; the README gains one | SPEC-015's last Consequences bullet rewritten too, and a second README sentence re-pointed from `campaign.Open`'s doc comment to `campaign.LogPath` and SPEC-019 | QA's finding (the bullet), and the implementer's while adjudicating (the README pointed at `Open`'s doc, which no longer says the directory is created); the reading review then reworded the bullet as a binding and cited `campaign.LogPath`, whose doc says what the directory holds |
| D9: `TestMain`'s holder prints `HELD` and sleeps | it polls its parent every 100 ms and exits when the parent is gone, where the first version slept an hour | the reading review: a holder outlived a test binary killed by a timeout panic |
| D18: K1 to K15, each undone by hand with the inverse edit | K1 to K18, each restored from its saved text and checked by hash; K8 reds every bounded refusal test, not the same-process test alone; K6 reds the child test and QA's child test | the review added K16 (D3's order) and K17 (`release = nil`); the implementer added K18 for the rewritten footprint test; the review found two unbounded `Open`s that made K8 hang, now bounded by `openWithin`; the breaks ran by script in a scratch clone |
| Task 5: the chain prints `270 rows, 213 test files, 13 specifications` | `271 rows, 215 test files, 13 specifications` | VTT-271 and QA's two files |
| D15: C1 holds the code and C2 the report alone | C1's message amended after the gate, from `991c40e` to `855fafd`, tree unchanged; C2 also carries an open-debt entry | the owner's rulings on this report: correct the injection count before push, and record the cross-build gap as debt |
| the plan's constraint that the ticket is not edited | its writer added `vtt art install` to item 3, item 6 for VTT-191, SPEC-015 to the specifications it moves, and SPEC-015, the README, the ledger and `viewpoint_internal_test.go` to what it touches | item 6 is the Q4 ruling; SPEC-015 is Gap 1; art install is the plan's D7 and row D; the other files are those the plan's scope check found beyond the ticket's list |

## What could not be established

- **`Close`'s order is observed by nothing.** SPEC-019 says `Close` closes
  the log and then the directory's descriptor; the reading review swapped the
  two and the whole package stayed green.
- **A `flock` that fails with anything but `EWOULDBLOCK`** refuses the
  `Open` as `campaign: take the writer hold on <dir>: <err>`; no test makes
  `flock` fail with another error, and none of the filesystems the plan
  measured did, so that arm is read, not run. NFS is unmeasured, and Linux
  was measured in a container rather than on CI's runner.
- **Nothing in the gate cross-builds,** so the next change that makes
  `GOOS=windows` build again, or breaks another platform, goes unnoticed.
- **`vtt client run` and `vtt client soak` taking the hold** is read from
  `bootSelfContained`, not observed by a test.
- **`TestQAHoldAdminCommandsHoldNothingWhileTheyRun` depends on SQLite's
  rollback journal:** under WAL a reader does not stall behind `BEGIN
  EXCLUSIVE`, and its `join-link show` iteration would red for a reason
  unrelated to the hold. Nothing sets WAL today.
- **The hold is advisory.** A process that writes `log.db` without a
  `Campaign` is not stopped, as SPEC-019 says; no test holds that sentence
  since QA's was dropped.

## What was deliberately left out, and where it went

- A Windows hold through `LockFileEx`: nowhere, by the owner's ruling on Q3;
  SPEC-019 says the hold is unix only.
- A guard against a writer outside a `Campaign`: nowhere; SPEC-019 records
  the hold as binding `campaign.Open` alone.
- A cross-build in the gate: `docs/verification-debt.md`, Open debt, by the
  owner's ruling on this report (the plan's Gap 5).
