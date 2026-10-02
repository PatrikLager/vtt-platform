# A deadline test bounds the answer, not the start-up: the change

**Ticket:** `docs/superpowers/specs/2026-10-01-a-deadline-test-bounds-the-answer-not-the-start-up-design.md`,
its first paragraph corrected before C1 (Deviations).
**Plan:** `docs/superpowers/plans/2026-10-01-a-deadline-test-bounds-the-answer-not-the-start-up.md`,
verified by `verify-ticket` (Passes with gaps), corrected in the same place.
Its seven sign-off questions were answered on 2026-10-01, all seven as the
plan proposed: the wait lives in `qaConnect` and `connectMCPSubprocess` splits
into start and connect; the rule is a new row cited only by the new test; the
start-up wait is bounded by `subprocessAnswers`; the debt entry moves under a
dated heading with a Closed-by line; the pid line's order is recorded as open
debt; `go clean -cache` before the whole gate; one code commit and a report.
**The owner's ruling** behind the ticket, of 2026-10-01: the bound covers the
MCP handshake, not the shell's start-up.
**Last code commit:** `1724a69`, on `759bced`, `main` at the time. Every code
reference below is to that tree.

## The period, in commits

    git log --oneline 759bced..1724a69

    1724a69 A deadline test bounds the subprocess's answer, not its shell's start-up

`git diff --stat 759bced..1724a69`: 8 files changed, 861 insertions(+), 52
deletions(-).

The gate, `task check` whole, after `1724a69`: it ran twice. The first run
passed every step up to `check:mutation`, whose own self-test then refused on
disk space, 15.6 GiB free against its 16 GiB floor; nothing in the tree was
wrong. This session's old scratch clones and test binaries were removed and
the build cache cleaned, and the second run exited 0 with no step failed,
`check:mutation` printing 16.3 GiB free when it began. Its check steps' own
verdict lines read `check:comments` clean over 251 files,
`check:requirements-chain` 253 rows and 10 specifications, `check:doc-owner`
80 files, `check:new-prose` 282 added lines clean, `check:coverage` 20
packages at or above their floors, `check:no-pack`, `check:no-retraction` and
`check:no-create-scene` clean, `task lint` 0 issues, `check:breaking`
reporting pre-release, `check:mutation` 14 packages with zero unadjudicated
survivors, and `check:ts-mutation` 2909 mutants, 2808 killed, 70 survivors all
adjudicated equivalent and zero unadjudicated, 31 timed out and counted as
killed. Neither mutation gate re-mutated: no package `check:mutation` gates
changed, so it reused every package's verdict, and `check:ts-mutation`
verified its stored report since none of its inputs changed. `cmd/vtt` took
154.9 s in `check:race`.

C1's first commit attempt was refused by the pre-commit hook's `lint`, which
ran 305 s against the 5-minute timeout in `.golangci.yml`, on a build cache
just cleaned; `uptime` read a one-minute load of 98.8 moments later. Both
figures were read in the terminal; the hook log kept holds only the retry,
which passed with `lint` in 4.73 s.

## Done looks like, answered

1. `[x]` A deadline test whose fixture shell starts later than
   `neverAnswersBound` reaches every one of its assertions:
   `cmd/vtt/qa_e2e_wait_test.go#TestQAMCPConnectReachesItsAssertionsWhenTheFixtureStartsLate`,
   whose fixture sleeps twice the bound before writing its pid, is red with
   "the fixture never got to run" when `qaConnect` connects without waiting
   for the pid line, as `759bced`'s did (the first break in C1's message), and
   green on `1724a69`. QA's
   `cmd/vtt/qa_fixture_startup_test.go#TestQAFixtureStartupDeadlineTestsReachEveryAssertion`
   runs four fixture bodies under start-ups of zero to three times the bound,
   every row reaching every assertion.
2. `[x]` Each of the four deadline tests still asserts, on every run, the
   deadline (or prompt) error, that the bounded handshake returns within
   `qaPromptly`, and that the subprocess is gone: their assertions are
   unchanged. A failed connect returning a nil error reds all four on the
   error, and a kill without its Wait reds all four on the subprocess being
   gone; a connect context of twenty times the bound reds the three deadline
   tests on `qaPromptly`. The fourth,
   `TestQAMCPConnectFailsPromptlyWhenTheSubprocessExitsWithoutAnswering`,
   stays green under that break, since EOF ends its connect before any
   deadline, as the plan's D9 measured; no break reaches its `qaPromptly`
   assertion. The SIGTERM test's `trap '' TERM` now runs before the pid line:
   `cmd/vtt/qa_fixture_startup_test.go#TestQAFixtureStartupTheTrapIsInstalledBeforeThePidLine`
   reds when `qaScript` writes the pid line before the setup.
3. `[x]` The three bounds keep their values, and the other callers of
   `connectMCPSubprocess` call it as before: `git diff 759bced 1724a69`
   changes no line assigning `neverAnswersBound`, `subprocessAnswers`,
   `qaPromptly`, `qaCeiling` or `subprocessExits`.
   `git diff --stat 759bced 1724a69 -- cmd/vtt/e2e_wait_test.go cmd/vtt/mcp_adventure_e2e_test.go`
   prints nothing; `dialMCPSubprocess` in `cmd/vtt/mcp_ruleset_e2e_test.go`
   has no hunk; `TestQAMCPConnectSucceedsAgainstASubprocessThatAnswers`
   changed only by passing `qaScript` the new empty setup. All of them are
   green in the gate. `connectMCPSubprocess` keeps its signature, its doc
   comment and its order of operations;
   `cmd/vtt/e2e_wait_test.go#TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites`
   and the new
   `cmd/vtt/qa_e2e_wait_test.go#TestQAConnectMCPSubprocessFailsWithinItsOwnBound`
   hold its bound, the second to `qaPromptly`.
4. `[x]` The four tests pass under load where they failed, shown by the
   plan's D10 run: eight background loops each writing and running a new
   script, then the four tests twenty times each from a test binary.
   `759bced`'s build was green 0 of 80, every failure on the precondition.
   The change's build was green 80 of 80, the slowest handshake 512 ms, and
   its race build with the new test 50 of 50; both were built on 2026-10-01
   from code identical to `1724a69`'s, less the test the reading review added
   (Deviations). The load is process creation, not CPU: the plan measured 32
   `yes` loops at load 35 leaving the base green, 40 of 40, so a CPU-loaded
   run would not have shown the change.
5. `[x]` `docs/verification-debt.md`'s entry is closed: it moved under
   `## 2026-09-30 — the deadline fixture failed when its shell started late`
   with a **Closed by** line naming
   `TestQAMCPConnectReachesItsAssertionsWhenTheFixtureStartsLate`. `task check`
   whole is green, as the gate paragraph above records.

## What the rules became

| Ticket's rule | Became |
|---|---|
| A deadline test's bound covers the subprocess's answer, not its start-up. | VTT-253, "A deadline test whose assertions need its fixture to have run starts its bound only after the fixture reports that it has.", narrowed before C1 (Deviations) |

## The sort

One row accepted, A, and seven candidates refused, as the plan's "The sort"
lists them. Refusal 2, the SIGTERM test's trap order, said no check holds it;
QA's `TestQAFixtureStartupTheTrapIsInstalledBeforeThePidLine` and the setup
marker of `TestQAFixtureStartupDeadlineTestsReachEveryAssertion` now do, both
red when `qaScript` writes the pid line before the setup (observed on
2026-10-02 in a scratch clone), and VTT-253 cites both. It gained no row of
its own: the pid line marking the end of the setup is how VTT-253's bound
learns where start-up ends.

## Phase 4a: QA adjudications

**C1, `1724a69` (VTT-253, VTT-079), QA on opus. It was given the whole
ticket; VTT-079, and VTT-253 in its first wording; and, without their bodies,
the signatures of `connectMCPSubprocess`, `startMCPSubprocess`, the
`mcpSubprocess` type with its `connect` and `kill`, `qaBounded`, `qaScript`,
`qaReadPID`, `qaProcessState`, `qaAwaitEnded` and `qaConnect`, with the five
bound constants. It wrote `cmd/vtt/qa_fixture_startup_test.go`.** No failure.
Thirteen injections into its own file: eleven red, two of them in the tests
dropped below; two green, a trap moved to just after the pid line (a window of
microseconds) and a trap dropped from the matrix's SIGTERM rows (the kill is
SIGKILL, which no trap stops).

- `qaProcessState` reads a live pid that is not the test's child as gone,
  since `wait4` answers ECHILD: a helper limitation, not a defect of this
  change. Every caller passes a pid of the test's own child, the shell whose
  `$$` the pid line carries; QA's tests ask signal 0 as well. The helper is
  unchanged.
- The bounded part includes the kill and the reap, so a fixture child holding
  stderr lengthens it: by design, since VTT-079 bounds the failed connect's
  return, kill and reap included. Two fixtures leave such a child, the
  late-answer test's `sleep` and the matrix's SIGTERM body's `sleep 0.1`, the
  first stretching its bounded part to 1.51 s against a 500 ms bound (C1's
  reading review, on an instrumented copy); both are inside `qaPromptly`.
- A fixture that never writes its pid fails only after `subprocessAnswers`,
  and a child holding stderr stretches the reap past it (QA measured 60 s
  with `sleep 60` in the setup): a failure path only, red with "the fixture
  did not write its pid within 40s" either way. Unchanged; whether it should
  fail sooner is open (What could not be established).
- "The bounded handshake" includes the kill and the reap: answered by the
  second line.
- Done item 1's "fails on today's tree" could not be run by QA, which had no
  old tree; it emulated it (a `qaReadPID` before connect, and the slow start
  sent through `connectMCPSubprocess`).
- Requirements QA asked an id for: Done item 2's assertions are VTT-079's;
  its trap clause is VTT-253's, through the trap test; Done item 3 is a
  constraint on this change, not a rule.
- Dropped: `TestQAFixtureStartupTheBoundsKeepTheirValues` and
  `TestQAFixtureStartupConnectMCPSubprocessStillBoundsTheStartUp`. Each
  pinned Done item 3 for this change only and holds no rule, so a deliberate
  change to a bound or to `connectMCPSubprocess` would red it for nothing; the
  second also returned early, green, when its shell was killed before its
  first command. What the second held of the wrapper's bound is held by the
  test VTT-079 gained (Deviations).
- Retagged: QA's "ticket Done item 2" and "Done item 3" citations name no
  durable thing (`CLAUDE.md` rule 8) and became VTT-079 or VTT-253.

## The breaks

C1's message carries them, one line each, with the check that spoke; every
break was reverted and the repaired tree ran clean. Seven: `qaConnect` not
waiting for the pid line; a failed connect returning a nil error; the connect
context given twenty times the bound; `kill` without its Wait; `kill` sending
SIGTERM, under the D10 loops; the pid line written before the setup; and
`connectMCPSubprocess` handing its connect twenty times the bound.

## Rule 9: how MapTool does this

Answered in the plan: a test fixture's start-up under a handshake bound is not
an area a virtual tabletop has to solve, so `~/dev/RPTool` was not consulted,
and the plan records that it was not.

## What the start-up costs, and why

Measured at `759bced` by the plan's verifier, from `cmd.Start` to a complete
pid line: a freshly written script median 233 ms idle, up to 1.48 s under the
D10 loops; the same script run again, median 6 ms; a fresh script run as
`/bin/sh <script>`, 4 ms. C1's reading review, in its report, measured median
251 ms idle for a fresh script and 8 ms for a re-run; it did not re-measure
the loaded figure, so 1.48 s is the plan's measurement alone. That the cost is
macOS's check on the first exec of a new file is an inference: what was
measured is that a new file costs it and a re-run does not.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| the plan: the ticket is not edited | its first paragraph was corrected before C1: `connectMCPSubprocess` creates its connect context after `cmd.Start` with a statement between them, not "in the next statement" | it misdescribed `connectMCPSubprocess` (C1's reading review); the plan's D3 carried the same words and was corrected with it |
| D11, D12 and the plan's Q2 and Q5: row A cited only by the new test, and the pid line's order recorded as open debt in C1 | no debt entry was written; four of QA's tests cite VTT-253 too, and its evidence names the trap test and the matrix beside the new test | QA's tests hold the order (The sort) |
| D11: VTT-079 unchanged | VTT-079's evidence gained `cmd/vtt/qa_e2e_wait_test.go#TestQAConnectMCPSubprocessFailsWithinItsOwnBound`, which hands the wrapper `neverAnswersBound` and asks for the deadline error within `qaPromptly`; red when the wrapper hands its connect twenty times the bound | the four deadline tests stopped calling `connectMCPSubprocess`, so its bound was held only by `TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites`'s 40 s ceiling; C1's reading review handed the wrapper twenty times the bound and every test stayed green |
| row A as the sort worded it: "A deadline test's bound on a subprocess's answer excludes the subprocess's start-up." | VTT-253 narrowed to a deadline test whose assertions need its fixture to have run | `TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites` asserts a deadline whose bound covers start-up by construction, which the first wording made a violation (C1's reading review) |
| the process's Phase 4a: QA's passing tests committed with the task | two of QA's tests dropped and its citations retagged | the adjudications above |
| Task 2 and the plan's Q7: `task check` whole once | twice | the first run's `check:mutation` refused on its disk floor (the gate paragraph) |
| the plan's run-time cost, about 10 s per gate | two runs of the whole `go test -count=1 ./cmd/vtt/` on `1724a69`, in the terminal, took 141.4 s and 139.4 s with `uptime` reading a load of 4 to 5 before and after; C1's reading review measured 139.6 s; the plan's were 131.8 s and 132.8 s, idle, on `759bced` | QA's file adds its own runs to every `cmd/vtt` pass; the loads differ, so the cost is not settled closer than that |

## What could not be established

- **Why a new file costs about 230 ms to start.** Measured, not explained;
  the plan's gap 4. A CI runner, when somebody dispatches one, may not pay it.
- **Other e2e tests run a freshly built `vtt` binary.** Their bound is
  `subprocessAnswers`, far above a script's start-up measured here, but
  nobody has measured a fresh Mach-O binary under the D10 loops; the plan's
  gap 3.
- **A fixture that never writes its pid fails only after 40 s**, and longer if
  its setup left a child holding stderr. Nothing says whether an early exit
  should fail promptly, and the test is red either way.

## What was deliberately left out

The ticket's own exclusions: `connectMCPSubprocess` against the real `vtt mcp`
binary, `neverAnswersBound`'s value, `waitForHealthz`, `waitWithTimeout` and
the other e2e tests. `qaProcessState` stays as it is, per the first
adjudication.
