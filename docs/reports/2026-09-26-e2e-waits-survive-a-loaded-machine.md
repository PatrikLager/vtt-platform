# An e2e wait for a subprocess survives a loaded machine: the change

**Ticket:** `docs/superpowers/specs/2026-09-26-e2e-waits-survive-a-loaded-machine-design.md`,
amended at sign-off on the points its verification found (the count over the
ten files, `startMCPSession`, two more failed tests, two runs and one push,
"a stall" for "load", item 3 narrowed to subprocess waits, one rule).
**Plan:** `docs/superpowers/plans/2026-09-26-e2e-waits-survive-a-loaded-machine.md`,
verified by `verify-ticket` (Passes with gaps); its seven sign-off questions
were answered on 2026-09-26: 1, 3, 5 and 6 as the plan proposed; 2 with the
in-process waits left alone; 4 as the plan proposed, then not applied (below);
7 with one row.
**Last commit that changes code:** `3aed0ce`, on `4d7de04`, `main` at the time.
Every code reference below is to that tree.

## The period, in commits

    git log --oneline 4d7de04..3aed0ce

    3aed0ce An e2e wait for a subprocess ends within a bound it takes from one place: VTT-079

`git diff --stat 4d7de04..3aed0ce`: 11 files changed, 1204 insertions(+), 40 deletions(-).

The gate: `task check`, whole, over the tree of `3aed0ce` before it was
committed: exit 0 on its second run, every step printing its own verdict,
`check:race` green with `cmd/vtt` at 143 seconds, `check:comments` ending
`clean`, `check:requirements-chain` reading 79 rows, `check:mutation`
reporting fourteen packages with zero unadjudicated survivors; the first run
stopped at `lint` (below, Deviations). Before it: `gofmt`, `go vet`, the package plain and
under load, `check:comments` after `--write-ledger`, `check:requirements-chain`,
`check:new-prose`; then the pre-commit hook's nine checks.

## Done looks like, answered

1. `[~]` Under generator A (one busy loop per core, `uptime` 12 to 18, `ok`
   in 30 seconds) and
   generator B (two per core plus `go build -a ./...` looped three times,
   `uptime` 28 to 50, `ok` in 37 seconds),
   `go test -race -count=3 -p 1 -run '<the three
   tests>' ./cmd/vtt/` was green on the changed tree, and `task
   test:external` whole was green under B, the generators killed and `pgrep
   yes` empty afterwards. The item's "today the same run is red" did not
   hold: the verification ran the same generators against `4d7de04` and
   reddened nothing, because the runs were lost to a paging stall (ten
   gigabytes into an 11.3 GB swap) that CPU load does not reproduce. What
   holds the new shape against a stall is item 2's bound and the figures'
   arithmetic; the load runs show the bounds cost nothing on a green run.
2. `[x]` `TestWaitForHealthzGivesUpOnAListenerThatNeverAnswers`,
   `TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites` and
   `TestWaitWithTimeoutKillsASubprocessThatNeverExits` in
   `cmd/vtt/e2e_wait_test.go`, each on a fixture that never answers with a
   bound of 500 ms and a 40-second watch; the first was red on `4d7de04`'s
   `waitForHealthz` (`waitForHealthz(500ms) did not return within 40s against
   a listener that never answers`); the other two hold logic `4d7de04` already
   had (the connect's `context.WithTimeout`, `waitWithTimeout`'s body,
   untouched here) behind a helper new in this commit, and go red under
   breaks B2 and B3.
3. `[x]` The eleven subprocess-wait sites take `subprocessAnswers` or
   `subprocessExits` from `cmd/vtt/e2e_wait_test.go`; `grep -nE '[0-9]+ ?\*
   ?time\.Second' cmd/vtt/*_test.go` prints 51 lines, none at a
   subprocess wait, listed under "The sites" below; at `4d7de04` it printed
   60, eleven at those sites.
4. `[x]` `task check` whole green (above), `check:race` included; `task
   test:external` green under generator B (133 seconds, `load-B-external.log`
   in the session's scratch directory) and plain in the session's terminal,
   not logged.

## What the rules became

| Rule, as the ticket words it after sign-off | Became |
|---|---|
| An e2e wait for a subprocess that never answers ends within a bound and reports it | VTT-079, the three negative tests as evidence |

Refused: the plan's rule A, that an e2e test waits long enough that machine
load alone does not fail it, the ticket's reason: no check the gate
can run holds it, and neither the verification nor Task 6 could make the old
tree red under CPU load (see item 1).

## The sites

| Constant | Sites |
|---|---|
| `subprocessAnswers` (40 s) | `waitForHealthz` on the `vtt serve` subprocess in `TestThreeRoleExitScenarioOverLiveServeSubprocess` and `TestServeSubprocessExitsCleanlyOnSIGTERM`; the first-line wait in `TestEventsTailBinaryExitsCleanlyOnSIGINT`; the connect in `TestMCPCommandServesRealStdioTransport`, `TestMCPSubprocessExitsCleanlyOnSIGTERM` and `dialMCPSubprocess` through `connectMCPSubprocess` |
| `subprocessExits` (30 s) | `waitWithTimeout` in the SIGINT test, `TestServeSubprocessExitsCleanlyOnSIGTERM`, `TestMCPCommandServesRealStdioTransport`, `TestMCPSubprocessExitsCleanlyOnSIGTERM` and the one cleanup closure `connectMCPSubprocess` builds |

The in-process waits (fixture `waitForHealthz` calls, WebSocket dial, write
and read contexts, tool calls over an open session, poll loops) keep their
literals: sign-off question 2, since the failures were all at a subprocess
start or exit. Six of them, in four tests, wait for the subprocess to answer
a tool call (`ListTools` in the two stdio tests, `mustCallTool`'s context,
the two batch polls in the adventure test and the one in the ruleset test)
with 5 seconds; they are read as in-process under question 2, and a stall
that expires one prints `ListTools over real stdio: context deadline
exceeded (stderr: )`; the next lost run there is that reading's cost, not a
surprise. The plan's twelve was eleven: its verification counted
`waitWithTimeout` "at four sites and 7 s at two" where there are five
callers, one cleanup closure counted twice.

## The breaks

| Break, one edit in a scratch clone | Red |
|---|---|
| B1 a bare `http.Get` back in `waitForHealthz` | the healthz test: `did not return within 40s against a listener that never answers` |
| B2 `context.WithCancel` in place of the bound on the MCP connect | the connect test: `did not return within 40s against a subprocess that never writes` |
| B3 `waitWithTimeout`'s `time.After` arm made one that never fires | the exit test: `did not return within 40s for a subprocess that never exits` |

B3 was first tried as deleting the arm, which leaves an unused import and a
build failure, and a build failure reads as no red; the session's shell
loop that ran the breaks was made to print the exit code and look for
`build failed` before B3 was re-run.

## QA adjudications

QA derived fourteen cases from the ticket and a signature list of the three
helpers and the constants, without the diff or the test files; all fourteen
passed on first run, seven were shown red against copies of the helpers
with their bounds removed and the rest under mutated fixtures.

- The ticket named `dialMCPSubprocess` where the observable helper is
  `connectMCPSubprocess`: the ticket now names both.
- "Reports it" says nothing of what the report carries: left so; QA asserts
  the status for a 503 and the deadline sentinel for a stall, and only logs
  the URL and "did not exit within timeout".
- That a failed connect leaves no subprocess behind, killed and reaped
  before the error returns, is held by four of QA's cases and stated by no
  row: a candidate for a later sort.
- `waitWithTimeout` passes a non-zero exit through as an error: observed,
  unstated, left so.
- QA's cases cite VTT-079 and now stand in its evidence cell beside the
  three negative tests, since each observes the rule.
- QA's comment on `kill(pid, 0)` had the hazard backwards (a zombie reads
  as present, not as gone): reworded at review; its cleanup kill is now
  guarded so a pid proved reaped is never signalled.
- QA's positive MCP case answers the SDK's `initialize` with a canned
  protocol version; its comment warns the next SDK bump to follow it.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Sign-off question 4: the elapsed assertion in `testStateDumpForAProjectedSeat` compares with `dumpCatchUpTimeout`. | It keeps its 10-second literal. | `dumpCatchUpTimeout` is 30 seconds and is the command's own deadline: `drainToHead` returns `errCatchUpDeadline` when it fires with the head unreached, which fails the first assertion; a comparison with 30 seconds could fire only on a success slower than that, the deadline arm with the head reached, by milliseconds of dial overhead, a different defect the message does not name. The literal is the "promptly" the test's own comment asks for, an in-process figure question 2 leaves alone; the reading review confirmed the reasoning. |
| Plan D7: 54 sites re-pointed. | Eleven. | Sign-off question 2; the plan's twelve counted `connectMCPSubprocess`'s cleanup closure twice. |
| Plan D5: no elapsed assertion tighter than the watch. | QA's file carries fourteen `took` bounds; at review the nine on failure paths (ten times the 500 ms bound, proving the helper honours what it was handed) stay, and the five on success paths were raised to QA's 20-second ceiling, the shape that lost the runs. | Found by the reading review. |
| Plan D2: "seven subprocess starts". | Eight: the ruleset guide test runs two subtests. 8 × 40 + 177 = 497 seconds, inside the ten-minute default. | The reading review counted `dialMCPSubprocess` callers, not `buildVTTBinary` callers. |
| Plan Task 8 (`task check` exits 0) and the review's gate lines. | The first whole `task check` run stopped at `lint`: staticcheck ST1008 on QA's `qaConnect`, an error returned before the closer. | Neither the implementer's local steps nor the review ran `task lint`; only `go vet`. Fixed by reordering the return; the gate run again. |

## What could not be established

- Whether the bounds survive the stall that lost the runs: it was a paging
  stall under memory pressure (ten gigabytes into an 11.3 GB swap), and no
  generator that reproduces it was run on this machine, which had 1.3 GB of
  swap left at the verification's run (1.1 GB at its end); CPU load to
  `uptime` 43 on the old tree and 50 on the new reddened neither.
- `waitForHealthz` held a bare `http.Get` with no timeout from the day it was
  written, so a listener that accepted and never answered hung the test
  forever; no gate covered it, so it is a finding here and not a recipe in
  `docs/verification-debt.md`.
- The forty-two in-process waits keep 2- to 10-second literals; a stall
  long enough expires them too (six of them, in four tests, wait for the
  subprocess to answer a tool call). Sign-off question 2's reading; a later
  ticket if it costs a run.

## What was deliberately left out, and where it went

- `buildVTTBinary` per test: kept (plan D8); a `TestMain` build is a ticket of
  its own if wanted.
- The two inline connects in `mcp_e2e_test.go` keep their shape and take the
  constant; routing them through `connectMCPSubprocess` is a refactor for a
  later ticket.
- A `-timeout` for `cmd/vtt` in the Taskfile: not this ticket's; the
  constants' arithmetic against the default ten minutes is plan D2's.
- No production line under `cmd/vtt/`, `internal/` or `client/src` changed.

## The sort

One candidate accepted as VTT-079; one refused, above.
