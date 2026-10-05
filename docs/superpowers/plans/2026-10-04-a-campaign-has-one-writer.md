# A campaign has one writer — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-04-a-campaign-has-one-writer-design.md`
**Verified:** 2026-10-04 by `verify-ticket` (dev-cycle 0.4.0), run by an agent
that did not write the ticket, against `4966408` on `feat/campaign-writer-lock`
(equal to `main`, with the ticket untracked). Verdict: **Passes with gaps.**
The gaps are listed at the end and travel with this plan. This plan does not
edit the ticket.

**Goal, in the ticket's words:** while a `Campaign` is open on a directory, a
second `campaign.Open` of it is refused, in this process or another, with an
error naming the directory. Closing the first `Campaign`, or the death of its
process, lets the next `Open` succeed. `vtt invite`, `vtt revoke` and
`vtt join-link` keep working beside a running `vtt serve`, and a second
`vtt serve` exits with the refusal. The debt entry closes, and `task check`
whole is green.

**MapTool (CLAUDE.md rule 9): it does not solve this.** `git grep` over
`src/main/java` for `FileLock`, `FileChannel`, `lockfile` and `.lck` finds no
lock on a campaign file. Its guards are in-process only:
`AppState.backgroundTaskLock`, a `ReentrantLock` serialising load, save and
autosave in one JVM, and `MapTool.startServer`'s check of its own static
`server` field ("You are already running a server."). `PersistenceUtil.saveCampaign`
copies a temporary file over the `.cmpgn`, so with two instances the last save
wins whole. Nothing is corrupted, because a MapTool campaign is a snapshot,
not a log. **Borrowed:** nothing. **Refused:** an in-process guard alone,
because our second writer is usually another process; and last-writer-wins,
because a log interleaves rather than replaces. Read in `~/dev/RPTool/maptool`
at `f4b7fef6c`.

## Verification, check by check

1. **Every path resolves (by command).** `ls` finds every path the ticket
   names, and `grep` finds each named symbol where the ticket puts it. That
   covers `campaign.go`'s `Open`, `Close`, `Append`, `AppendBatch` and
   `LogPath`; `store.Open`'s `busy_timeout(5000)`; `composeServer`; the
   `campaign.Open` then `identity.Open(campaign.LogPath(...))` pair in
   `invite.go`, `revoke.go` and `joinlink.go`; the debt entry; and VTT-191,
   OPEN. The ticket's `grep -rn -i 'flock\|lockfile\|O_EXCL'` prints nothing,
   and its recipe reproduces at the base (P3).
2. **"Done" is an observation (by command).** The ticket names no test, so
   there is none to run. Sketches of each fail today where they should (P5).
   Items 1 and 4, and item 2's crash half, are red. Item 2's close half cannot
   fail today (Gap 3). Item 3 is green today by design, and red on a tree that
   has the hold but no CLI change (P6). Item 5 is the gate.
3. **Each rule is breakable (a reading, then probes).** Each of the four rules
   has a single edit that reds a named test (K1 to K15). Rules 1 and 2 become
   row A, rule 3 becomes rows B and C, and rule 4 becomes row D, narrowed
   (D12).
4. **The scope matches the claim (by command, then a reading).**
   `campaign.Open` has four production callers: `composeServer`, `invite`,
   `revoke` and `withIdentity`. Every test that opens a campaign directory, by
   `campaign.Open` (51 call sites in test files) or by `composeServer`, ran
   under the hold; one broke (P4), as item 3 anticipates. `serve_compose.go`
   needs no edit (P7). Files beyond the ticket's list, none widening the two
   components: `remove_actor_test.go`, `viewpoint_internal_test.go` (D10),
   SPEC-015, `README.md`, the ledger and two new test files.
5. **No recorded decision is contradicted silently (a reading).** SPEC-009's
   "`identity.Open` takes its own handle on that file, independent of
   `store.Open`" is kept, and rules out every SQLite-level lock (P1). SPEC-015
   states the lock's absence as present fact, made false unnamed (Gap 1).
   SPEC-011's `closeFn` order is unchanged; no ADR speaks to writers.
6. **The records the work moves are named (by command, then a reading).** The
   section is present and says `New:`. The reading finds that SPEC-015's
   "Nothing locks a campaign directory to one writer ..." becomes false, and
   the section does not name it (Gap 1, D11). SPEC-009 and SPEC-011 stay true.
   019 is free on `main`, `feat/per-character-logs` and `origin/main`, and
   nothing cites `SPEC-019`.

## Measurements this plan stands on

Every measurement was taken by command, by the verifier, in a scratch clone of
`4966408` or a `git archive` of it, never in the working tree. The sketches
are discarded, and the clone is deleted. Linux runs used Docker Desktop's
`linuxkit 6.12.65` (aarch64), on overlay and tmpfs directories, with binaries
cross-compiled with `CGO_ENABLED=0`.

**P1. The mechanism.** The probe imports the real `store` and `identity`. A
holder child prints `HELD` and sleeps, and the parent SIGKILLs it.

| Candidate | 2nd open, same process | 2nd open, other process | after holder SIGKILL | `identity.Open` + `CreateInvite` while held, both processes | leaves |
|---|---|---|---|---|---|
| `flock` on the directory's descriptor | refused at once | refused | acquired | ok, 1 ms | nothing |
| `flock` on a `writer.lock` file | refused at once | refused | acquired | ok | `writer.lock` |
| `flock` on `log.db` itself | refused | refused | acquired | **darwin `SQLITE_BUSY` after 5.1 s; Linux ok** | — |
| `fcntl` `F_SETLK` on a lock file | **acquired** | refused | acquired | ok | `writer.lock` |
| `O_EXCL` lock file | refused | refused | **refused: the file outlives its process** | ok | `writer.excl` |
| SQLite `locking_mode=EXCLUSIVE`, or a held `BEGIN IMMEDIATE` | refused after 5.1 s | refused after 5.1 s | acquired | **`SQLITE_BUSY` after 5.1 s** (`identity.Open`; `CreateInvite`) | — |

Linux matches darwin on the five rows it ran. The SQLite rows were not run
there, because they already break SPEC-009. The refusal is `EWOULDBLOCK`
("resource temporarily unavailable") on both.

| # | Tree | Result |
|---|---|---|
| P2 | the sketch's `internal/campaign` test binary, on Linux | whole package `PASS`, the six hold tests included |
| P3 | the debt recipe at the base: two `Open`s, one `SceneCreated` for scene `s` each | both appends `err=<nil>`; the next `Open`: `campaign: corrupt log at seq 2: engine: scene "s" already exists` |
| P4 | the hold in `campaign.Open` alone, CLI untouched | `./internal/... ./contract/... ./tools/...`: one red, `TestARemovedActorLeavesALogThatStillFolds` ("reopen campaign (this is the re-fold): campaign: another writer holds ..."); `./cmd/...` green, so no test runs an identity command beside an open `Campaign` |
| P5 | D9's tests at the base, with a stub `ErrHeld` | red: the same-process, other-process and kill tests, and the serve e2e (the second serve printed `listening on`). Green: the close, failed-open (a) and child tests, and item 3's test |
| P6 | P4's tree with item 3's test | red: `vtt invite: open campaign: campaign: another writer holds ...`; reverting any one of the three commands alone reds it |
| P7 | the full sketch, D1 to D8 | green: `go vet`; `go test -count=1` over `./internal/... ./contract/... ./tools/...` and `-p 1 ./cmd/vtt/...` (138 s); `-race` on the new tests and the fixed one. Clean: `golangci-lint` `config verify` and `run ./...` (0 issues); `semgrep` (0); `go-arch-lint` (OK); `check-doc-owner.py`; `check-new-prose.py main` (373 lines, 8 files) |
| P8 | lint of three forms | `int(f.Fd())` into `syscall.Flock`: gosec **G115**; with a `fd > math.MaxInt` guard first: 0 issues; D2's form: 0 issues |
| P9 | gremlins on `internal/campaign`, with the gate's flags | base: 35 killed, 0 lived. `*os.File` with the guard: 41 killed, **2 lived** (the guard's CONDITIONALS_BOUNDARY; `Close`'s `err == nil` CONDITIONALS_NEGATION). D2's form: **41 killed, 0 lived** |
| P10 | coverage | `internal/campaign` 86.3 % at the base, 86.4 % on the sketch, against a threshold of 85.5. The subprocess pass over the serve e2e writes one counter file, and `go tool covdata` merges it |
| P11 | cross-builds | `GOOS=windows go build ./...` passes at the base and fails on the sketch with `undefined: syscall.Flock`. linux/amd64 and freebsd build |
| P12 | D10's VTT-191 test | green at the base; with `receive` forwarding an event whose fold failed, it is the only red in `internal/gateway` |
| P13 | P7 with ids simulated (rows A to D, VTT-191's evidence, a SPEC-019 stub) | `check-requirements-chain.py`: `270 rows, 213 test files, 13 specifications` |

**What breaks what.** Each break is one edit on the sketch, restored
afterwards. K13 to K15 are not yet measured; Task 9 measures them.

| # | Edit | Red |
|---|---|---|
| K1 | the `syscall.Flock` call removed | the same-process, other-process and kill tests; the serve e2e (P5) |
| K2 | `Close` does not call `release` | the close and child tests, and six existing reopen tests (`TestCloseReopenStateDeepEquals`, `TestRebuildEqualsLiveProperty` and `TestExitScenario` among them) |
| K3 | the rebuild-failure path keeps the hold | the failed-open test |
| K4, K5 | the refusal omits the directory, or does not wrap `ErrHeld` | the same-process and other-process tests, for each |
| K6 | `O_CLOEXEC` dropped | the child test **alone** |
| K7 | `LOCK_SH` instead of `LOCK_EX` | the same-process, other-process and kill tests |
| K8 | `LOCK_NB` dropped | the same-process test, by its 10 s bound |
| K9 to K11 | `invite`, `revoke` or `join-link` back on `campaign.Open` | item 3's test, for each alone |
| K12 | `receive` forwards an event whose fold failed | the VTT-191 test, **alone** in `internal/gateway` |
| K13 | `art install` opens a `Campaign` | item 3's test (expected) |
| K14 | the `store.Open`-failure path keeps the hold | the failed-open test, case (b) (expected) |
| K15 | `takeHold` uses an `O_EXCL` file that `release` removes | the kill test, and the e2e's third serve (expected, from P1) |

**Comment shares.** `check-comments.py main` on the sketch refused four files
that had fallen more than 1.0 under their rows. `--write-ledger` lowered six
rows: `campaign.go` from 53.5 to 49.4, `invite.go` from 35.0 to 23.6,
`revoke.go` from 16.3 to 5.6, `joinlink.go` from 29.0 to 27.4,
`viewpoint_internal_test.go` from 25.8 to 24.3, and `remove_actor_test.go`
from 37.0 to 36.4. It added the two new test files at 0.0, and the run then
read `256 files, 18 added comment lines, 256 ledger rows; clean`.

**Mutation keys: none moves.** `internal/campaign` is gated, but
`tools/mutation-equivalents.txt` holds no key for it. `cmd/vtt` is not in
`PACKAGES`, the `internal/gateway` edits are test files, and `client/` is
untouched. Both self-tests print `OK` on the sketch.

**At the base:** the chain reads `266 rows, 211 test files, 12
specifications`; `gofmt -l` prints `internal/gateway/scenario_test.go` (older).
Free space was 31,159,948 KiB, then 31,694,792 KiB (30.2 GiB) after the clone
was deleted, and the Go cache grew from 7.9 to 9.2 GiB. Load was 7.3 to 9.3 at
first, from another session, then 1.3 to 2.8.

## Constraints that bind every task

- **Rule 2:** no gate is weakened. The ledger goes down only through
  `--write-ledger`, and the fixture fix keeps every assertion (D8).
- **Rule 3:** no proto change.
- **Rule 4:** the hold adds no fold. The fixed test still re-folds through
  `campaign.Open`.
- **Rule 8:** cite names, never lines. **Rule 9:** answered above.
- **Rule 10 and SPEC-010:** a comment is a warning, a pointer or a one-line
  doc (D13). A block over 6 lines that gains a line is refused, so no block
  moves.
- **SPEC-008:** ids come from `requirement-id`, after sign-off, A to D in
  order. **SPEC-009:** nothing locks the SQLite file.
- **Order and hygiene.** `check:drift` passes only on a committed tree, so the
  order is review, commit, gate. The review package is `git diff HEAD`, and
  nothing is stashed or checked out while a reviewer reads. `git add` and `git
  commit` run in separate calls, with `git show --stat HEAD` after each. A new
  test goes after a closing brace.
- The ticket and every report under `docs/reports/` are not edited.

## Decisions this plan makes

**D1. The hold is `flock(LOCK_EX|LOCK_NB)` on a descriptor of the campaign
directory itself (Q1).** Forced by P1. It is the only candidate that conflicts
within a process and across processes, ends when its holder is SIGKILLed, and
leaves identity working. `fcntl` locks belong to the process, `O_EXCL` goes
stale on a crash, both SQLite forms stall identity for 5.1 s and then fail,
and `flock` on `log.db` blocks SQLite on darwin only. Choosing the directory
over a lock file is a preference with a reason: P1 measured the two equal,
but a lock file could never be deleted, since an unlink on close lets a third
opener lock a fresh inode while a second still holds the old one.

**D2. The form: `syscall.Open(dir, O_RDONLY|O_CLOEXEC, 0)` into an `int`, and
a `release func() error` field on `Campaign` that closes it.** Forced by P8 and
P9. `int(f.Fd())` trips G115, and this repo does not silence a linter
(`internal/eventgen`'s `Int31n` is the precedent). The `*os.File` form with a
range guard left two survivors, the guard's and `Close`'s `err == nil`. This
form leaves none, needs no `-1` sentinel, and has no finalizer to release
early. `syscall.Open` does not add `O_CLOEXEC`
the way `os.Open` does, and without it a child keeps the hold past `Close`
(K6).

**D3. `Open`'s order is `EnsureDir`, `takeHold`, `store.Open`,
`rebuildLocked`,** and every failure after the hold calls `release`. The hold
comes first, so a refused opener never opens SQLite (`store.Open` runs a
`CREATE TABLE`). A reading holds this order (refusal 8).

**D4. `Close` closes the log, then calls `release`,** so that no second writer
opens the log while this handle exists. It then sets `release` to nil and
returns `errors.Join(logErr, holdErr)`. A second `Close` returns nil, which
the close test asserts.

**D5. The refusal.** An exported `var ErrHeld = errors.New("campaign: another
writer holds the campaign directory")`. `takeHold` wraps it with the directory
as `"%w %s; a campaign has one writer at a time, so stop the vtt serve or
close the Campaign that has it open"`. Through `vtt serve` it reads `vtt serve:
open campaign: campaign: another writer holds the campaign directory <dir>;
...`. `LOCK_NB` makes the refusal immediate, where blocking would hang a
second `vtt serve` silently (K8). Any other `flock` error fails closed as
`campaign: take the writer hold on <dir>: <err>`, because a hold silently not
taken is the same defect again. `ErrHeld` is exported so that QA, which sees
only `go doc`, can assert the error's kind without matching prose.

**D6. The identity commands stop opening a `Campaign` (Q2 (i)).** A new
exported `campaign.EnsureDir(dir) error` is `Open`'s `os.Stat` and `MkdirAll`
half: the plain-file refusal, `0o750` and the "must exist and be writable"
wrap. `Open` calls it first. `invite`, `revoke` and `withIdentity` call it,
then `identity.Open(campaign.LogPath(...))`, keeping their `open campaign:`
prefix. This is forced by the ticket's fourth rule and by P6. Option (ii)
would be a second kind of `Campaign`, folding the whole log only to make a
directory exist. Option (iii), going over the wire, cannot mint a DM, which
SPEC-009 says is done out of band.

**Consequence:** these commands no longer read or fold the log. An
invite-first directory holds only identity's tables until `serve` adds
`events`; "invite then serve" in `TestCampaignDirectoryWorksInEitherCLIOrdering`
holds this (green in P7). An invite into a campaign whose log does not fold
now works. SPEC-019 says both. **Layout:** `EnsureDir` takes `Open`'s place in
the file, so its two inner comment blocks stay unchanged context. Moving them
would add lines to over-bound blocks carrying `spec §`, which SPEC-010 refuses.
P7 is clean in this layout.

**D7. Every other opener.** `vtt serve` takes the hold through
`composeServer`, unedited, and so do `vtt client run` and `soak` through
`bootSelfContained` on a fresh temporary directory. `mintInvites` (identity
only) and `vtt art install` (it writes `art/` with no `Campaign`) must not take
it, and do not. `state dump`, `events tail` and `mcp` go over the wire.
`internal/harness` and production `internal/gateway` never call
`campaign.Open`. `client/e2e/setup.ts` mints before `vtt serve` starts; that is
a reading, since `task e2e` is outside `check`.

**D8. The one test that breaks.** `TestARemovedActorLeavesALogThatStillFolds`
calls `dmConn.CloseNow()`, `f.srv.Close()` and `f.campaign.Close()` (with its
error checked) before its `campaign.Open(f.path)`. The re-fold and every
assertion are unchanged. The fixture's cleanups then close twice, which D4 and
`httptest.Server.Close` both allow.

**D9. The tests, in two new files.** `internal/campaign/writer_hold_test.go`
(`package campaign_test`) has a `TestMain` that, when
`VTT_CAMPAIGN_HOLDER_DIR` is set, opens that directory, prints `HELD` and
sleeps. `startHolder` runs `os.Args[0] -test.run=^$` with that variable set,
and reads the line. A `TestMain` is used rather than a skipping helper test,
so no permanent SKIP shows in `-v` output. A package binary takes one
`TestMain`, which D16 tells QA.

| Done | Test | Today |
|---|---|---|
| 1 | `TestASecondOpenInTheSameProcessIsRefused`: the second `Open` runs in a goroutine bounded at 10 s; `errors.Is(err, campaign.ErrHeld)`, and the error names the directory | red |
| 1 | `TestAnOpenWhileAnotherProcessHoldsIsRefused`: the same assertions | red |
| 2 | `TestClosingTheHolderLetsTheNextOpenSucceed`, including that a second `Close` returns nil | green (Gap 3) |
| 2 | `TestTheHoldEndsWhenItsProcessIsKilled`: refused while the holder lives; SIGKILL, `Wait`, then `Open` succeeds | red |
| 2 | `TestAnOpenThatFailsReleasesTheHold`, two cases: (a) a log with two `SceneCreated` for one scene, written through `store`; (b) a `log.db` that is a directory. Each is opened twice, and both times fails with its own error, never `ErrHeld` | (a) green; (b) unmeasured (K14) |
| 2 | `TestAChildProcessDoesNotInheritTheHold`: `Open`, start `sleep 30`, `Close`, then `Open` succeeds | green |
| 3 | `cmd/vtt/writer_hold_test.go`'s `TestCommandsThatAppendNothingWorkWhileTheCampaignIsServed`: while `composeServer` holds the directory, `runCLI` runs `invite`, `revoke` of that id, `join-link` `show`, `open`, `close` and `rotate`, and `art install` of an `srcPNG`; all succeed | green; red on P6's tree |
| 4 | `TestASecondServeOnAHeldDirectoryExitsWithTheRefusal`, using `buildVTTBinary`. A second `vtt serve` exits non-zero within `subprocessExits`, with the `ErrHeld` text and the directory and no `listening on`, and nothing answers `/healthz` at its address. After SIGKILL of the first, a third serves `/healthz` | red |

Item 3's other half, against a directory nothing holds, stays with
`TestCampaignDirectoryWorksInEitherCLIOrdering`, `TestInviteThenRevoke` and
`TestJoinLink*`, all unchanged and green (P7).

**D10. VTT-191 joins this ticket (Q4 (a)).**
`TestASeatIsSentNothingForAnEventWhoseFoldFails` goes after the last closing
brace of `internal/gateway/viewpoint_internal_test.go`, a file SPEC-015's
Status already names. A player seat that has received `SceneCreated` for `s`
must be sent nothing for a second one, and nothing for a following
`NarrationAdded`. A fresh seat whose prefix folds is sent the narration. The
test is green today, and it is K12's only red (P12). Under the hold, a log
that does not fold can only come from outside it, and this test is the one
thing holding that arm. It closes the debt entry whole.

**D11. The records.** **New:
`docs/specifications/019-a-campaign-directory-has-one-writer.md`**, written
with the `specification` skill against the code. It records:

- what holds a directory (D1 and D2, taken by `Open` after `EnsureDir` and
  before the log);
- what a second opener is told (D5);
- when the hold ends (`Close`, a failed `Open`, process death, never by
  inheritance);
- who takes it (`vtt serve` and the self-contained harness, through
  `composeServer`);
- who must not (`invite`, `revoke`, `join-link`, `art install`,
  `mintInvites`);
- that identity's handle is untouched (SPEC-009).

Consequences:

- the hold is unix only (Q3);
- a filesystem without `flock` refuses every `Open` (Gap 6);
- the identity commands neither read nor fold the log;
- a log written without the hold can still fail to fold, which is SPEC-015's
  arm.

Requirements: A to D.

**SPEC-015:** the sentence from "Nothing locks a campaign directory to one
writer" through "the next `Open` refuses the log
(`docs/verification-debt.md`)" becomes:

> "`campaign.Open` takes the directory's writer hold first (SPEC-019), so a
> second `Campaign` on one directory, in this process or another, is refused
> before it can append; a seat reaches this arm only on a log written without
> the hold, and the next `Open` refuses that log too."

SPEC-015's Status and Requirements are unchanged, since VTT-191 is already
listed. Nothing in SPEC-009 or SPEC-011 becomes false.

**`README.md`'s Running paragraph** gains: "`vtt serve` holds the directory
while it runs: a second `vtt serve` on it is refused, and `invite`, `revoke`,
`join-link` and `art install` work beside it."

**The debt entry** moves, unedited, to a dated section in date order, `##
2026-09-29 — two writers on one campaign directory could write a log that no
longer opened`. It gains a **Closed by** paragraph naming the two item-1 tests
and the break that reds them (K1), and, for VTT-191, D10's test and its break
(K12), observed in C1's breaks. **The register** gains rows A to D, and
VTT-191's evidence becomes D10's test.

**D12. The sort.** Forced by SPEC-008. The rows are lettered so nothing reads
as an id.

| # | Rule | Proposed | Red when | Tests | Record |
|---|---|---|---|---|---|
| A | `campaign.Open` refuses a directory another open `Campaign` holds, in the same process or another, without waiting, with an error that names the directory and wraps `campaign.ErrHeld`. | accept: rules 1 and 2 | K1, K4, K5, K7, K8 | the same-process and other-process tests, and the serve e2e | SPEC-019 |
| B | Closing a `Campaign`, or an `Open` that fails after taking the hold, ends the hold, so the next `Open` of that directory succeeds. | accept: rule 3, the holder's half | K2, K3, K14 | the close and failed-open tests | SPEC-019 |
| C | A holder's hold ends when its process dies, killed or not, and no child process it starts keeps it. | accept: rule 3, the process's half | K6, K15 | the kill and child tests, and the serve e2e | SPEC-019 |
| D | `vtt invite`, `vtt revoke`, `vtt join-link` and `vtt art install` work against a directory a `Campaign` holds, and take no hold. | accept: rule 4, narrowed | K9 to K11, K13 | item 3's test | SPEC-019 |

**Refused, one line each:**

1. "At most one `Campaign` is open", alone: it is seen only as A's refusal.
2. "A second open says why", alone: it is how A is observed.
3. Rule 4 as a universal: it cannot be observed for commands that do not
   exist.
4. Item 3's "against one nothing holds": the existing CLI tests hold it.
5. "A refused `vtt serve` serves nothing": it is A through the binary.
6. "A refusal does not block": it is A's "without waiting".
7. "Identity works while held": it is SPEC-009's independence.
8. "A refused opener never opens SQLite": it is D3's order, which nothing
   observes cheaply.

Four rows accepted, eight refused, and VTT-191 gains evidence without a new
row.

**D13. Comments and the ledger.** New: the one-line docs of `EnsureDir`,
`ErrHeld`, `Open` and `Close`; `// Keep O_CLOEXEC: a child process that
inherits the descriptor keeps the hold past Close.` above `takeHold`; and `//
Call release once: a second close of its descriptor can close an unrelated
file.` on `release`. Four blocks that would become false are cut to a warning
and a pointer: `invite.go`'s doc from 14 lines to 4 (keep it off
`campaign.Open`, SPEC-019, item 3's test), `revoke.go`'s from 7 to 2,
`withIdentity`'s from 5 to 1, and `Open`'s 12 to `EnsureDir`'s 2. Wrap under
85 columns: the sketch's first draft failed `check:new-prose` on five lines.
Tests carry their `// VTT-NNN` line only. `--write-ledger` runs once, before
C1.

**D14. Mutation keys: none to re-point.** After the review settles, run
`python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q` anyway; they take seconds. If a review
touches a non-test file in a gated package that has keys, re-measure. Keep
D2's form: the other lint-clean form leaves two survivors (P9).

**D15. One code commit and the report (Q5).** Forced by P6: the hold without
the CLI change breaks `vtt invite` beside `vtt serve`, so the CLI change
cannot come after the hold, whatever the ticket's "campaign package first"
says. The alternative is a split, with `EnsureDir`, the CLI change and item
3's test first. It is safe, but costs a second review record for about 60
lines. **C1** holds the code, the tests, the fixture fix, SPEC-019, SPEC-015,
the README, the rows, the debt move and the ledger. **C2** holds the report.

**D16. Phase 4a for C1.** One QA agent per `qa-prompt.md`, never given the
diff, the source, the existing tests or the implementer's report. It gets rows
A to D and VTT-191, SPEC-019 whole, SPEC-015's withholding paragraph, `go doc
-all ./internal/campaign`, `go doc ./internal/identity`, and the `--help` of
`vtt serve`, `invite`, `revoke`, `join-link` and `art install`. It also gets
one harness fact: the package's `TestMain` already holds
`$VTT_CAMPAIGN_HOLDER_DIR` and prints `HELD`, so it must not write a second.
It writes `internal/campaign/qa_writer_hold_test.go` and
`cmd/vtt/qa_writer_hold_test.go`. Adjudications go in the report, and an
escape goes to the debt file as a recipe.

**D17. Phase 4b for C1, after 4a.** One reviewer at high effort, briefed to
verify by command every sentence C1 adds: SPEC-019, SPEC-015's edit, the
README, the debt move, and D13's docs and warnings. It also confirms that the
fixture still re-folds through `Open`, and checks the break lines in the
draft message. If it dies on a model limit, say so and re-dispatch the same
brief on `fable`.

**D18. The breaks, one per check relied on.** K1 to K15, in a scratch clone of
C1's final tree. Each gate is clean first, each break is one edit, and each is
undone by hand with the inverse edit.

**D19. Disk and load.** Forced by `check:mutation`'s 16 GiB floor and by the
MCP e2e deadlines under load. Immediately before `task check`, run `go clean
-cache`, `df -k` and `uptime`, and wait out any load above 4. Launch it once,
after C1, in its own session (`start_new_session=True`). Below 16 GiB free,
stop.

## Tasks, in dependency order

**Task 0 — Baselines.** **Done when** these are recorded, and `requirement-id`
is on the path: `df -k`, `uptime`, the chain (`266 rows, 211 test files, 12
specifications`), `check-comments.py main`, `check-doc-owner.py .` and both
self-tests.

**Task 1 — Rows.** **Files:** `docs/requirements.md`. After sign-off, run
`requirement-id` once for each of A to D, in order. **Done when:** there are
four new OPEN rows.

**Task 2 — The hold.** **Files:** `internal/campaign/campaign.go`,
`writer_hold_test.go` (new) and `internal/gateway/remove_actor_test.go`. D9's
six tests come first, with P5's three red. Then come D1 to D6 in
`campaign.go`, then D8. **Done when:** `go test -count=1 ./internal/...` is
green, and `golangci-lint run ./internal/campaign/...` reports 0 issues.

**Task 3 — The CLI.** **Files:** `cmd/vtt/invite.go`, `revoke.go`,
`joinlink.go` and `writer_hold_test.go` (new). Write item 3's test first; it
is red on Task 2's tree (P6). Then make the `EnsureDir` switch, then write the
serve e2e. **Done when:** `go test -count=1 -p 1 ./cmd/vtt/...` is green.

**Task 4 — VTT-191, if Q4 is (a).** **Files:**
`internal/gateway/viewpoint_internal_test.go`. **Done when:** D10's test is
green, and red under K12.

**Task 5 — The records.** **Files:** SPEC-019 (new), SPEC-015, `README.md`,
`docs/verification-debt.md`, and `docs/requirements.md` (the evidence for A to
D and VTT-191). The content is D11. **Done when** the chain prints `270 rows,
213 test files, 13 specifications`, none of A to D or VTT-191 is OPEN, and
`grep -c 'Nothing locks a campaign directory'` over SPEC-015 prints 0.

**Task 6 — Local gates.** Run `gofmt`, `go vet`, `task lint`, and `go test
-count=1` over `./internal/... ./contract/... ./tools/...` and `-p 1
./cmd/vtt/...`. Run `-race` on the new tests, then `semgrep`, `go-arch-lint
check`, `check:comments`, `check:doc-owner` and `check:new-prose`. **Done
when:** each prints its own completion line.

**Task 7 — Phase 4a, then 4b.** D16 and D17. Findings are fixed, and the
affected task's "done" is re-run. The review settles before Task 8 begins.

**Task 8 — Ledger, self-tests, commit C1.** D13 and D14, then C1's message,
which lists the ids and D18's lines. **Done when:** both self-tests print
`OK`, `git show --stat HEAD` lists C1's files, and `task check:drift` is clean.

**Task 9 — Breaks and the whole gate.** D18, then D19, then `task check` once.
**Done when:** each break gives its red, and `task check` exits 0 with every
step's own verdict, `check:mutation` included.

**Task 10 — The report.** Write
`docs/reports/2026-10-04-a-campaign-has-one-writer.md` per the
`implementation-report` skill: each Done item with its observation, the rows
and refusals, the rulings, the rule-9 answer, P1, the breaks and the gaps.
**Done when:** C2 holds the report alone. Push after C2, and let the pre-push
hook finish (about three minutes).

## Gaps that travel with this plan

1. **"Specifications this moves" names only the new record.** SPEC-015 states
   that no lock exists, and that becomes false. D11 rewrites it; the writer
   may revise the section.
2. **The ticket does not answer rule 9.** This plan answers it.
3. **Neither Done item 2's close half nor item 3 can fail today,** since
   nothing holds a directory yet. K2 and K9 to K11 hold them, not a red on the
   base.
4. **Rule 4 is a universal.** Row D names the four commands that exist. A
   future command gets no row until it exists.
5. **Windows stops building (P11).** No record names Windows, CI is
   `ubuntu-latest`, and no gate cross-builds, so nothing would notice (Q3).
6. **NFS is unmeasured, and Linux was measured in a container, not on CI's
   runner.** On NFS, Linux emulates `flock` with `fcntl`, which would lose the
   in-process conflict or refuse an exclusive lock on a read-only descriptor;
   D5 fails closed either way.
7. **VTT-191 is outside the ticket's Done list.** D10 adds it, for Q4.
8. **The verifier did not run `task check` whole,** as instructed. P9 and P10
   ran mutation and coverage for `internal/campaign` only.

## Questions for sign-off

1. **The mechanism: `flock` on the directory's own descriptor, or on a
   `writer.lock` file inside it?** Recommend the directory (D1). The two
   measured identical on all four questions, and the directory leaves no file
   that can never be deleted. Every other candidate failed a question in P1.
2. **How do `invite`, `revoke` and `join-link` keep working beside `vtt
   serve`? (i) An exported `campaign.EnsureDir` and no `Campaign`; (ii) a
   `Campaign` opened without the hold; (iii) other.** Recommend (i) (D6). The
   commands append nothing and need only the directory, while (ii) folds the
   whole log for nothing and adds a second kind of `Campaign`.
3. **Windows: (a) accept that `GOOS=windows` no longer builds, and say so in
   SPEC-019; (b) add a `//go:build !unix` file whose `Open` refuses at run
   time; or (c) use `LockFileEx` through `golang.org/x/sys/windows`?**
   Recommend (a). Nobody ships Windows, (b) adds a file that only says no,
   and (c) cannot be measured on this machine.
4. **VTT-191 here? (a) Yes, D10's internal test, and the debt entry closes
   whole; (b) no, the entry splits and a VTT-191 paragraph stays open.**
   Recommend (a). It is one test, green today and the only thing holding the
   arm (P12), and the entry names both halves.
5. **One code commit and the report (D15)?** Recommend yes. The hold cannot
   land before the CLI change (P6), and a split buys a second review for
   about 60 lines.
6. **Rows A to D, with the eight refusals, and VTT-191 gaining evidence but no
   new row?** Recommend yes (D12). Rules 1 and 2 are one observation. Rule 3
   has two independent halves, the holder's and the process's. Rule 4 is
   observable only for the commands that exist.
