# An e2e wait for a subprocess survives a loaded machine — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-26-e2e-waits-survive-a-loaded-machine-design.md`
**Verified:** 2026-09-26, by `verify-ticket`, an agent that did not write the
ticket, against `4d7de04` on `main` (branch
`chore/e2e-deadlines-survive-a-loaded-machine`, tree clean apart from the
ticket). Verdict: **Passes with gaps.** Every path and symbol resolves. Item 2's
first case is red today for a reason the ticket does not name: `waitForHealthz`
cannot honour its deadline against a listener that accepts and never answers.
Item 1's red could not be provoked by CPU load, the ticket's own generator or a
stronger one, and the evidence in the lost runs points at a different mechanism.
Four figures in the problem statement are off. All of it is under "What the
verification found" and travels into this plan, which does not edit the ticket.

**Goal, in the ticket's words:** every wait for a subprocess in `cmd/vtt`'s
tests takes its bound from one place, that bound survives a loaded machine, and
a subprocess that never answers still fails its test within a bound and says so.

**MapTool (CLAUDE.md rule 9), answered in one line.** The ticket says no
tabletop function is designed and MapTool's JUnit suite starts no subprocess;
checked, agreed, nothing to borrow.

## What the verification found

All at `4d7de04`. Which checks were commands and which readings is said per
item. The load runs were made on the unchanged tree.

- **Every path and symbol resolves** (command). The ten files exist.
  `buildVTTBinary`, `waitWithTimeout` and `syncBuffer.waitForLines` are in
  `client_e2e_test.go`; `waitForHealthz` and `readCommandResult` in
  `serve_e2e_test.go`; `dialMCPSubprocess` in `mcp_ruleset_e2e_test.go`;
  `startMCPSession` in `mcp_e2e_test.go`. The four tests the ticket names exist:
  `TestThreeRoleExitScenarioOverLiveServeSubprocess` (`library_test.go`),
  `TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved`
  (`mcp_adventure_e2e_test.go`), `TestMCPRulesetGuideWithAndWithoutRulesetFlag`
  (`mcp_ruleset_e2e_test.go`), `TestEventsTailBinaryExitsCleanlyOnSIGINT`
  (`client_e2e_test.go`). No caller of any of these helpers exists outside
  `cmd/vtt`, and no other test package in the tree runs `exec.Command`.
- **The figures** (command). `grep -cE '[0-9]+ ?\* ?time\.Second'
  cmd/vtt/*_test.go` sums to 60 over the 18 test files in `cmd/vtt`, and to
  **54 over the ten the ticket names**. The other six are in
  `state_dump_head_test.go`, arguments to `drainToHead` in its unit tests: not
  waits, not for a subprocess, not in the ticket. The 10 ms poll is
  `waitForHealthz`'s and `waitForLines`'s. The 3-second healthz sites are nine,
  all on an in-process `composeServer` behind a `net.Listen` the test holds; the
  5-second ones are three, two on a `vtt serve` subprocess
  (`TestThreeRoleExitScenarioOverLiveServeSubprocess`,
  `TestServeSubprocessExitsCleanlyOnSIGTERM`) and one in-process
  (`startArtCampaign`). The 10-second connect is at four sites: two inline in
  `mcp_e2e_test.go` (`TestMCPCommandServesRealStdioTransport`,
  `TestMCPSubprocessExitsCleanlyOnSIGTERM`), one in `dialMCPSubprocess`, and one
  in `startMCPSession` — **which starts no subprocess**: it connects over
  `mcpsdk.NewInMemoryTransports` to a `Server.Run` goroutine, so the ticket's
  sentence "`dialMCPSubprocess` and `startMCPSession` give the MCP client
  `10*time.Second` to connect over stdio" is half wrong. `waitWithTimeout` is
  called with 5 s at four sites and 7 s at two.
- **The failures, verbatim from the logs** (command). `task-check-shown.log`,
  2026-09-26 07:37, `check:race`:
  `--- FAIL: TestThreeRoleExitScenarioOverLiveServeSubprocess (14.78s)` /
  `vtt serve subprocess healthz never became ready: Get
  "http://127.0.0.1:52479/healthz": dial tcp 127.0.0.1:52479: connect:
  connection refused` and
  `--- FAIL: TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved (14.20s)`
  / `stdio client Connect: context deadline
  exceeded (stderr: )`. `task-check-shown2.log`, 07:52, `check:race`:
  `--- FAIL: TestEventsTailBinaryExitsCleanlyOnSIGINT (7.94s)` /
  `subprocess produced no output within 5s` (each message quoted from the `go test` line after its `file:line:` prefix).
  `push-main.log`, 2026-09-25 13:21, `test:external` (no `-race`): five tests
  — `TestThreeRoleExitScenarioOverLiveServeSubprocess (33.77s)` with the same
  healthz text, and `TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved
  (46.92s)`, `TestMCPCommandServesRealStdioTransport (12.74s)`,
  `TestMCPSubprocessExitsCleanlyOnSIGTERM (12.45s)`,
  `TestMCPRulesetGuideWithAndWithoutRulesetFlag/with_--ruleset (10.04s)`, each
  `stdio client Connect: context deadline exceeded (stderr: )`. The ticket
  names four of these seven tests; the two inline connects in
  `mcp_e2e_test.go` are missing from it. `push-main2.log` (13:26) is green
  (`ok cmd/vtt 145.040s`), as are `task-check-shown3.log`, `shown4.log`,
  `race-rerun.log`, `push-main3.log` and `push-branch.log`; so the logs I was
  pointed at show **two** lost `task check` runs and **one** lost push, against
  the ticket's three and two. The 09-25 report of the comment-gate change
  narrates the same two runs.
- **What the durations say** (reading of the logs). Every failed wait expired
  whole. The subtraction of the bound from the test's time gives what came
  before it: 3 s for the SIGINT test, ~10 s for the three-role test under
  `-race`, and in `push-main.log` ~29 s and ~37 s for a `go build` plus fixture
  that costs 2.38 s cold and 0.90 s warm here. So during the lost runs the
  machine was ten to fifteen times slower on a build and still could not
  start a listener in 5 s or answer an MCP initialize in 10 s — a stall, not a
  slowdown proportional to load.
- **Item 1, tried, green both times** (command; the tree unchanged; the
  generator killed afterwards and `pgrep yes` printing nothing). The machine:
  8 cores, 8.6 GB, Go 1.26.4, and already 10.0 GB into an 11.3 GB swap before I
  started, with another session's compile running (`uptime` 7–10).
  Run 1, eight busy loops (`for i in $(seq 8); do yes > /dev/null & done`),
  `go test -race -count=3 -p 1 -run
  'TestThreeRoleExitScenarioOverLiveServeSubprocess|TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved|TestEventsTailBinaryExitsCleanlyOnSIGINT'
  ./cmd/vtt/`: nine passes, 3.08–7.57 s each, `ok cmd/vtt 43.719s`; `uptime`
  3.67 at start (the loops had just launched), 9.71 at the end, 9.90 during the
  probe that ran beside it. Run 2, sixteen busy loops plus `go build -a ./...`
  three times over (34–42 s each): nine passes, 4.41–11.11 s each, `ok cmd/vtt
  65.941s`, exit 0; `uptime` 15.73 at start, 31.66 at the end, 38.39 and 43.53
  in the two minutes after. Hand-timed beside run 2, from a fresh binary path
  each time as `buildVTTBinary` makes one, at `uptime` 16–17: `vtt serve` to
  healthz 200 in 0.46–0.71 s (five tries), `vtt mcp` to its initialize reply
  in 0.44–1.53 s (five tries); at `uptime` 9.7–9.9 the same were 0.35–0.60 s
  and 0.35–0.36 s. Swap in use rose from 9,978 MB to 10,198 MB across run 2.
  So CPU load to four times the ticket's range leaves every wait an order of
  magnitude inside its bound, and the ticket's "machine load alone" is not
  what stalled the lost runs: the machine that lost them was paging. A
  memory-pressure generator was not run — this machine has 1.1 GB of swap
  left and other sessions on it — and is proposed under G1 instead.
- **Item 2's first case is red today** (command). A program in the scratchpad
  holding a verbatim copy of `waitForHealthz`, pointed at a `net.Listen` whose
  accepted connections are held and never written to, with a 2 s bound:
  `waitForHealthz(2s) HAS NOT RETURNED after 15s`. The cause is in the helper:
  `http.Get` uses `http.DefaultClient`, whose `Timeout` is zero, so the loop's
  `deadline` is checked only between requests and the first request never
  returns. The ticket's second rule is broken on today's tree at this site.
  The second case (`dialMCPSubprocess`) is not red today: the SDK honours the
  connect context, which is what every `context deadline exceeded` above
  proves; its test's red will be the deliberate break (D6).
- **Item 3's command cannot print 0 as written** (reading). `grep -c` over
  `cmd/vtt/*_test.go` prints one count per file; `state_dump_head_test.go`
  will still print 6, and whichever file holds the named constants prints its
  own literals. The plan reads item 3 as "0 for each of the ten named files"
  and the report prints the whole output. The ticket's "today it prints 60" is
  54 for the ten.
- **Each rule is breakable, and they are two** (reading). Rule A, "waits long
  enough that machine load alone does not fail it", fails in a world where the
  documented generator reds item 1's run: a lower bound on the wait. Rule B,
  "a wait for a subprocess that never answers ends within a bound and reports
  it", fails in a world where a never-answering fixture hangs the test or
  fails it with no error naming the wait: an upper bound. Different worlds
  break them, so two. Rule B's "and": one observation (the error the wait
  returns) covers ending and reporting, so it stays one rule; the sort at
  Phase 2 may split it if the message's wording matters. Rule A has a
  problem the sort must face: no check in the gate can hold it, because the
  gate cannot load the machine it runs on, and this verification could not
  make its observation fail; its evidence is a reading of the report's
  measurement (G6).
- **Scope** (command, then reading). Subprocess waits live in six of the ten
  files: `library_test.go`, `serve_e2e_test.go`, `client_e2e_test.go`,
  `mcp_e2e_test.go`, `mcp_ruleset_e2e_test.go`, `mcp_adventure_e2e_test.go`.
  The other four hold only in-process waits: `maps_e2e_test.go` (nine, all on
  `composeServer` fixtures and their WebSocket connections),
  `state_dump_cause_test.go` (one, a write deadline inside `truncatingGateway`'s
  handler, a fake server the test runs), `scenario_goldens_test.go` (one,
  `drainToCatchUpHead`'s `CatchUpHead` over `harness.Client` to an in-process
  gateway), `events_tail_truncation_test.go` (one, a goroutine returning after
  cancel). They are in scope only because item 3's grep names them. The ticket
  is right about the set of files and under-names the subprocess sites by two.
- **No recorded decision is overturned** (command over `docs/specifications/`,
  then reading). SPEC-007 to SPEC-010 say nothing about test waits;
  "Specifications this moves: None." is present and right; SPEC-010 governs
  the comments this work deletes and adds but is not changed by them. Two
  recorded arguments in `Taskfile.yml` are near this work and both stand: the
  `check` task's correction that a repeated failure once blamed on load was a
  frame inversion and cost an hour — which is why this plan says "a stall" and
  not "load" (the failure shapes here are a refused connection for five
  seconds straight and no first byte for ten, on a tree with no Go line
  changed, not a desynchronised read); and `check:race`'s "-p 1 is kept for
  genuine resource reasons, NOT as a flake mitigation", which this plan does
  not touch.
- **CLAUDE.md rule 2, argued both ways** (reading). *For "this weakens the
  gate":* a deadline is a threshold; raised, it accepts a `vtt serve` that
  takes twenty seconds to listen, which today's 5 s refuses. *Against:* what
  the gate holds at these sites is that a subprocess answers, exits on its
  signal and exits 0 — no requirement, specification or test sentence sets a
  startup latency, and the 5 s was written (per `waitWithTimeout`'s and
  `TestServeSubprocessExitsCleanlyOnSIGTERM`'s own comments) to distinguish
  "slow but working" from "swallowed entirely", i.e. to bound a hang. Every
  one of these waits returns on the event, so the happy path's runtime is
  unchanged (run 1 above: nine tests in 43.7 s); only a genuinely hung
  subprocess pays the larger figure. What is given up is the accidental
  ability of an idle machine to notice a five-to-ten-second startup
  regression, which a loaded one turned into a false failure in three runs
  out of the last seven. And rule 2's own condition for a gate change — "its
  own reviewed decision, with its own reason" — is what this ticket, plan,
  review and report are. Verdict: not a weakening, provided the bound stays
  finite and proven (D3–D5), and the report records the loaded measurements
  as the baseline a future latency rule would start from.

## Constraints that bind every task

- **CLAUDE.md rule 2.** `Taskfile.yml`, `.lefthook.yml`, `-p 1`, `-race`, the
  coverage floors and every other gate stay as they are; this work changes
  test files only.
- **CLAUDE.md rule 8.** Names, not `file:line`, in this plan, the tests, the
  doc sentences, the report and the commit message.
- **CLAUDE.md rule 10 and SPEC-010.** A doc sentence says what a constant is
  for; the measurements and the argument for the figures live here and in the
  report. `check:comments` refuses a comment line added to a file at its
  ceiling (every touched file is within 0.1 of its row), a new file above 25.0,
  and the banned terms — among them "measured", a date, "used to",
  "previously", "turned out". D9 keeps every file inside that.
- **The ticket's scope.** Nothing under `internal/`, `cmd/vtt/*.go` that is
  not a test, or `client/src` changes; `cmd/vtt` is outside `check:mutation`'s
  package list and `tools/mutation-equivalents.txt` holds no `cmd/vtt` key, so
  no adjudication key moves. `check:drift` has nothing to compare.
- **SPEC-008.** This plan allocates no id. Rows are dispensed after sign-off
  (question 7) and cited by the tests that hold them.
- **`go test`'s default package timeout is ten minutes** and the Taskfile
  passes no `-timeout`; `cmd/vtt` runs 122–177 s per pass today. Every figure
  in D2 is checked against that arithmetic.
- **Commits need a review record** (`review-gate` in `.lefthook.yml`); the
  review settles before the gate starts, and the gate runs detached.
- **The load generator is killed after every use** and `pgrep yes` printing
  nothing is recorded with the run.

## Decisions this plan makes

1. **D1 — The one place is a new file, `cmd/vtt/e2e_wait_test.go`, holding
   named constants.** Forced by item 3: a constant declared with a second
   literal inside any of the ten files makes that file's count 1, and writing
   `time.Second * 40` to slip past the regex would be gaming the observation.
   Constants, not a helper: every site already passes a `time.Duration` to
   `context.WithTimeout`, `time.After` or a helper, and `e2eWait(kind)` would
   add a call and no meaning a name does not carry. Each constant has one doc
   sentence saying what it bounds.
2. **D2 — Three kinds, fixed figures, no scaling rule, no environment
   variable.**
   - `subprocessAnswers = 40 * time.Second`: a subprocess these tests start
     gives its first answer — healthz 200, the first line of `events tail`,
     the MCP initialize reply.
   - `subprocessExits = 30 * time.Second`: a subprocess ends after SIGINT,
     SIGTERM or stdin EOF.
   - `stepCompletes = 30 * time.Second`: one step between two ends already
     running — an in-process listener answering, a WebSocket dial, write or
     read, a tool call over an open session, a goroutine returning after
     cancel, a poll loop's give-up.
   - `neverAnswersBound = 500 * time.Millisecond`: what the negative tests
     hand a helper, so that a helper proven to honour it is proven to honour
     the three above.
   What forced the figures: the lost runs expired 5 s and 10 s whole while a
   build in the same run ran ten to fifteen times slow, so the stall to cover
   is longer than 10 s and not measurable by this verification; under CPU load
   to 38 a first answer costs at most 1.5 s, so 40 s is over twenty-five times
   the loaded cost and four times the largest bound that expired; seven
   subprocess starts at 40 s plus 177 s is 457 s, inside the ten-minute
   package timeout even if every subprocess hangs; an exit after SIGTERM
   includes `serveShutdownTimeout` (5 s in `serve.go`), so 30 s is six times a
   correct worst case; the in-process steps take the same 30 s because the
   same stall halts the test process and the figure is paid only on failure.
   No scaling rule, because nothing readable at runtime predicts a paging
   stall. No `-race` multiplier: the flag costs the test binary about 1.3×
   (`check:race`'s own figure) and `push-main.log` lost five tests without it;
   if one is ever wanted, the runtime exposes nothing and the way is a pair of
   test files under `//go:build race` and `//go:build !race` declaring `const
   raceEnabled`. No environment variable: a knob is a threshold anyone can
   raise to get a green run, the one move rule 2 refuses; it is a second place
   the bound comes from, against item 3; and CI runs the identical `task
   check` by hand on `ubuntu-latest`, where the same bound must hold.
3. **D3 — `waitForHealthz` carries its deadline on every request.** One
   `context.WithTimeout` for the loop; each request is
   `http.NewRequestWithContext` on it, sent through `http.DefaultClient.Do`;
   the loop runs while `ctx.Err() == nil` and returns the last error. Forced by
   item 2's first case, red today because `http.DefaultClient` has no timeout.
   The 10 ms poll stays.
4. **D4 — `dialMCPSubprocess` splits into a core that returns and a wrapper
   that fatals.** `connectMCPSubprocess(t, binPath, wsURL, token, bound,
   extraArgs...) (*mcpsdk.ClientSession, func(), error)` starts the process,
   connects within `bound`, and on failure kills, waits and returns the error;
   `dialMCPSubprocess` keeps its signature, calls the core with
   `subprocessAnswers` and fatals as today. Forced by item 2's second case: a
   test cannot observe a helper that calls `t.Fatalf`. The two inline connects
   in `mcp_e2e_test.go` keep their shape — they need `cmd` for the signal and
   the exit-code assertion — and take `subprocessAnswers`; routing them through
   the core is a refactor the ticket does not ask for and the report may
   propose.
5. **D5 — Three negative tests, in `e2e_wait_test.go`, each on a fixture that
   never answers.** `TestWaitForHealthzGivesUpOnAListenerThatNeverAnswers`: a
   `net.Listen` on `127.0.0.1:0`, a goroutine that accepts and drains each
   connection without writing, `waitForHealthz(url, neverAnswersBound)`;
   `TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites`:
   `connectMCPSubprocess` on the script fixture with `neverAnswersBound`;
   `TestWaitWithTimeoutKillsASubprocessThatNeverExits`: `exec.Command` on the
   same script with its stdin pipe held open, `waitWithTimeout(cmd,
   neverAnswersBound)`, then `cmd.ProcessState` shows it ended. The script
   fixture, `neverAnswersScript(t) string`, writes `#!/bin/sh` and `while read
   line; do :; done` into `t.TempDir()` with mode `0o755`: it ignores its
   arguments, reads stdin, writes nothing, ends on EOF or SIGKILL; `/bin/sh`
   exists on this machine and on `ubuntu-latest`. Each test runs the wait in a
   goroutine and observes it through a `select` bounded by `subprocessAnswers`,
   asserting a non-nil error that satisfies `errors.Is(err,
   context.DeadlineExceeded)` where the helper returns a context error, or the
   helper's own message otherwise — and no tighter elapsed assertion, because
   under the stall this ticket exists for a tight one fails the other way. The
   third test goes beyond the ticket's two; the ticket's rule covers exit
   waits and the test costs half a second (question 3).
6. **D6 — Deliberate breaks, one per check.** B1: put `http.Get` back in
   `waitForHealthz` → the healthz test reports "did not return" after
   `subprocessAnswers`. B2: `context.Background()` for the connect in
   `connectMCPSubprocess` → the MCP test reports the same. B3: delete the
   `time.After` arm of `waitWithTimeout` → the exit test reports the same.
   Each is followed by the other direction on the repaired code: `go test
   -count=1 -p 1 ./cmd/vtt/` green. Items 1 and 3 are observations, not
   checks: nothing to break, and the report says so.
7. **D7 — The 54 sites, by the sort below.** Substitution only; the three
   failure strings that say `within 5s` print the constant through `%s`; the
   two comment blocks that argue the old margins are deleted, because they
   would be false and are argument (the block above `waitWithTimeout(cmd,
   7*time.Second)` in `TestServeSubprocessExitsCleanlyOnSIGTERM`, and the one
   above the same call in `TestMCPSubprocessExitsCleanlyOnSIGTERM`); the
   `elapsed > 10*time.Second` assertion in `testStateDumpForAProjectedSeat`
   compares against `dumpCatchUpTimeout`, the deadline its own message says
   it guards against (question 4).
8. **D8 — `buildVTTBinary` stays per test.** The ticket scopes it out; the
   build precedes every wait and cannot expire one; `TestMain` would report a
   build failure at package level with no test's name and would have to carry
   the `VTT_SUBPROCESS_COVERDIR` branch; and the compile cache already makes a
   repeat build a link (0.90 s warm against 2.38 s cold here). If wanted, a
   ticket of its own.
9. **D9 — `check:comments` stays clean by construction.** The new file holds
   four one-line doc sentences, the fixture helper and three tests, well under
   25.0; `serve_e2e_test.go` gains one warning line at `waitForHealthz` ("each
   request carries the deadline; a listener that accepts and never answers
   holds a bare Get past it") and loses five, `mcp_e2e_test.go` loses seven;
   no other touched file gains a comment line. Where the band rule then asks,
   `python3 tools/check-comments.py --write-ledger` lowers the rows, which
   only go down. No doc sentence uses a banned term; the figures' reasons are
   D2's and the report's.
10. **D10 — Phase 4a applies, and derives from names, not source.** The rules
    describe observable behaviour. QA receives: the ticket; the rows once
    dispensed; and, because `go doc` shows nothing of an unexported helper in
    a `_test.go` file, the signature list — `waitForHealthz(base string,
    timeout time.Duration) error`, `connectMCPSubprocess(t *testing.T,
    binPath, wsURL, token string, bound time.Duration, extraArgs ...string)
    (*mcpsdk.ClientSession, func(), error)`, `waitWithTimeout(cmd *exec.Cmd,
    timeout time.Duration) error`, and the four constants by name and
    purpose. It does not receive the diff, `e2e_wait_test.go` or the report.
    Its brief is item 2's two cases in its own fixtures; item 1 is an
    observation QA can only repeat, not derive.
11. **D11 — The load generator the ticket asks the plan to name**, and the
    proof order. Generator A: one busy loop per core (`for i in $(seq $(sysctl
    -n hw.ncpu)); do yes > /dev/null & done`), `uptime` about 10 on this
    machine. Generator B: two per core plus `go build -a ./...` in a loop of
    three, `uptime` 17–38. Each ends with `pkill yes` and a recorded empty
    `pgrep yes`. The proof is item 1's command under both, then `task
    test:external` under B, `uptime` recorded before and after each. Neither
    generator reddened the unchanged tree; the report says so, as the ticket
    permits, and the memory-pressure generator of G1 is the one to try when a
    machine can afford it.
12. **D12 — One code commit, one report commit.** The ticket says one
    component, one commit; the report is its own, after.

## The sort

Every site in the ten files, by what is waited for. Counts are today's, by
command, and the report re-counts.

| Constant | Sites |
|---|---|
| `subprocessAnswers` | `waitForHealthz` on a `vtt serve` subprocess in `TestThreeRoleExitScenarioOverLiveServeSubprocess` and `TestServeSubprocessExitsCleanlyOnSIGTERM`; the first-line `time.After` in `TestEventsTailBinaryExitsCleanlyOnSIGINT`; the connect contexts in `TestMCPCommandServesRealStdioTransport`, `TestMCPSubprocessExitsCleanlyOnSIGTERM` and `dialMCPSubprocess` (via the core). Six. |
| `subprocessExits` | every `waitWithTimeout` call: the SIGINT test, `TestServeSubprocessExitsCleanlyOnSIGTERM`, `TestMCPCommandServesRealStdioTransport`, `TestMCPSubprocessExitsCleanlyOnSIGTERM`, the one cleanup closure
`connectMCPSubprocess` builds and `dialMCPSubprocess` returns. Five. |
| `stepCompletes` | the rest: in-process `waitForHealthz` (ten: `TestServeComposeEndToEnd`, `startLiveFixture`, `startMCPFixture`, the ruleset and adventure fixtures, `startArtCampaign`, and four more in `maps_e2e_test.go`); WebSocket dial, write, `readCommandResult` and `conn.Read` contexts; `srv.Shutdown` and the Serve-goroutine `time.After` in `TestServeComposeEndToEnd`; `startMCPSession`'s connect and its `Server.Run` return; `ListTools`, `CallTool`, `mustCallTool`, `mustCallToolOK`, `callGetStateGeneric` contexts; the poll deadlines in `waitForMCPHeadSequence`, the narration poll, and the three batch-observed polls; `waitForLines` and the `done` `time.After` in `testEventsTailAgainstFixture`; `drainToCatchUpHead`'s context; `truncatingGateway`'s write context; the cancel wait in `TestEventsTailStopsCleanlyOnContextCancel`; `observeStateIndependently`'s dial. Forty-two. |
| `dumpCatchUpTimeout` (existing, `state_dump.go`) | the elapsed assertion in `testStateDumpForAProjectedSeat`. One. |

Six plus five plus forty-two plus one is fifty-four; sign-off question 2 left
the forty-two and the one alone. The 20 ms and 50 ms poll
sleeps and the 10 ms ones are not seconds and stay.

## Tasks, in dependency order

### Task 1 — The core under `dialMCPSubprocess`

File: `cmd/vtt/mcp_ruleset_e2e_test.go`. `connectMCPSubprocess` per D4, with
the `10*time.Second` it inherits left as the literal for now (Task 4 moves
it); `dialMCPSubprocess` becomes the wrapper. No caller changes.

**Done when:** `go test -count=1 -p 1 -run 'TestMCP' ./cmd/vtt/` is green and
`git diff` shows no line outside the two functions.

### Task 2 — The bounds file and the negative tests, one of them red

File: `cmd/vtt/e2e_wait_test.go`, new. The four constants (D2) with their doc
sentences, `neverAnswersScript`, and the three tests (D5).

**Done when:** `go test -count=1 -p 1 -run 'NeverAnswers|NeverWrites|NeverExits'
./cmd/vtt/` reports `TestWaitForHealthzGivesUpOnAListenerThatNeverAnswers`
failed with "did not return" after `subprocessAnswers` and the other two
passed; the failure text is recorded for the report.

### Task 3 — `waitForHealthz` honours its deadline

File: `cmd/vtt/serve_e2e_test.go`. D3, plus the one warning line of D9.

**Done when:** the three negative tests pass; break B1 applied, the healthz
test red with the recorded text, B1 reverted by the inverse edit and the test
green again; `go test -count=1 -p 1 ./cmd/vtt/` green.

### Task 4 — The fifty-four sites

Files: the ten the ticket names. D7 by the sort table; the two comment
deletions; the three strings; the `dumpCatchUpTimeout` comparand.

**Done when:** `grep -cE '[0-9]+ ?\* ?time\.Second' cmd/vtt/*_test.go` prints
0 for each of the ten files (and 4 for `e2e_wait_test.go`, 6 for
`state_dump_head_test.go` — the whole output goes in the report); `go test
-count=1 -p 1 ./cmd/vtt/` green; `python3 tools/check-comments.py main` ends
clean, after `--write-ledger` if the band rule asked.

### Task 5 — The other two breaks

Files: `cmd/vtt/mcp_ruleset_e2e_test.go`, `cmd/vtt/client_e2e_test.go`. B2 and
B3 of D6, each applied, observed red, reverted by its inverse edit and
verified reverted by `git diff`; then the other direction, the whole package
green.

**Done when:** two recorded red texts, an empty `git diff` against Task 4's
tree for those two files, and one green package run after the last revert.

### Task 6 — The proof under load

No file. D11: generator A then B on the changed tree, item 1's command under
each, `task test:external` under B, `uptime` before and after each, the
generator killed and `pgrep yes` recorded empty.

**Done when:** both runs print `ok` with every test's time, and the
recordings are in hand for the report.

### Task 7 — Phase 4a

D10. One QA agent with the inputs D10 lists; its tests, where they pass,
committed with the task; each failure adjudicated in the report under its
own heading, one line per finding.

**Done when:** QA's report is read, its tests run green in the tree or their
findings are adjudicated, and nothing of the diff was given to it.

### Task 8 — The gate, the review, the commit, the push

`check:comments` and its self-tests first (seconds); the review, settled;
then `task check` whole, detached; the review record; one commit (D12) whose
message names the three breaks and which check spoke; the push, whose
pre-push runs `test:external` again.

**Done when:** `task check` exits 0 with `check:race` green, the pre-push
hook passes, and `origin` holds the commit.

### Task 9 — The implementation report

File: `docs/reports/2026-09-26-e2e-waits-survive-a-loaded-machine.md`. Against
the ticket's four items with the observations: the two load runs of this
verification and Task 6's, with `uptime` and swap figures; the grep output
whole; the negative tests' red texts and the three breaks; QA's
adjudications; the `waitForHealthz` hole as a finding (not
`docs/verification-debt.md`: it never passed a gate that should have caught
it, because no gate covered it); the gaps below and how sign-off settled
them; the follow-ups (D4's inline connects, D8's `TestMain`, a `-timeout` for
`cmd/vtt` in the Taskfile). Its own commit.

**Done when:** the report cites `4d7de04` as the base and the code commit as
the period's last code commit, and every code reference in it is a name.

## Sign-off questions

1. **The figures (D2).** 40 s to answer, 30 s to exit, 30 s per step. Larger
   ones cost nothing on a green run and only delay a hung one's report;
   smaller ones re-run the risk with no evidence for where it ends. Accept?
2. **In-process waits (D1, D7).** Item 3's grep forces the forty-one
   in-process sites onto `stepCompletes`. Or read item 3 as subprocess-only,
   leave those literals, and accept that the grep does not print 0?
3. **The third negative test (D5).** Keep `waitWithTimeout`'s, or hold to the
   ticket's two?
4. **The elapsed assertion (D7).** `dumpCatchUpTimeout` as the comparand, or
   `stepCompletes`?
5. **The ticket's figures (G2–G5).** The writer amends the problem statement
   — 54 for the ten, `startMCPSession` in-process, the two unnamed tests, two
   runs and one push in the logs, "a stall" for "load" — or the report records
   the corrected forms. Which?
6. **`TestMain` (D8).** Out, with a follow-up ticket if wanted. Agreed?
7. **The rows.** Two — rule A with `**READING**` evidence naming the report's
   measurement, rule B held by the three negative tests — or one, rule B only,
   with rule A recorded as the ticket's reason and not a rule?

## Gaps carried from the verification

- **G1.** Item 1's red was not provoked: eight busy loops (`uptime` to 9.9),
  then sixteen plus a looped `go build -a ./...` (`uptime` to 43.5), nine
  passes each. The lost runs happened on this 8.6 GB machine while it was ten
  gigabytes into swap with a parallel `go test ./...`; the generator that
  matches that is memory pressure (a process that allocates and keeps
  touching a few gigabytes) beside a build, and it was not run here because
  the machine had 1.1 GB of swap left and other sessions on it. Task 6 runs
  A and B; the report records that neither reddened the old tree, as the
  ticket permits, and names the memory generator for a machine that can
  afford it.
- **G2.** "60 across the ten test files" is 54; the six others are
  `drainToHead` arguments in `state_dump_head_test.go`. Item 3's command, as
  written, prints per-file counts and will never print 0 for that file or
  for the constants' file; the plan reads it as 0 per named file. Question 5.
- **G3.** `startMCPSession` starts no subprocess; sorted as a step. Question 5.
- **G4.** `TestMCPCommandServesRealStdioTransport` and
  `TestMCPSubprocessExitsCleanlyOnSIGTERM` failed in `push-main.log` and the
  ticket does not name them or their inline connects; the sort covers them.
  Question 5.
- **G5.** The logs show two lost `task check` runs and one lost push, not
  three and two; a third run and a second push may be in another session's
  scratchpad. Not load-bearing. Question 5.
- **G6.** Rule A is held by no check the gate can run; the register's honest
  evidence for it is a reading of the report's measurement, or the rule is
  the ticket's reason rather than a row. Question 7.
