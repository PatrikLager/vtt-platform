# An e2e wait for a subprocess survives a loaded machine

## The problem

`cmd/vtt`'s end-to-end tests start the real `vtt` binary as a subprocess
(`buildVTTBinary` builds it with `go build` into a temp dir, then
`exec.Command` runs `serve`, `mcp` or `events tail`) and wait for it with a
fixed literal: `waitForHealthz(base, 3*time.Second)` or `5*time.Second`
polls `/healthz` every 10 ms; `dialMCPSubprocess` and the two inline connects
in `TestMCPCommandServesRealStdioTransport` and
`TestMCPSubprocessExitsCleanlyOnSIGTERM` give the MCP client `10*time.Second`
to connect over stdio; the SIGINT test gives
the tail subprocess `5*time.Second` to print its first line; `waitWithTimeout`
kills after 5 or 7 seconds. There are 54 such second-literals across the ten
test files (60 across all eighteen), eleven of them at a wait for a
subprocess, and they are the same whether the test binary runs plain
(`task test:external`, `go test -p 1 ./cmd/vtt/...`) or under `-race`
(`check:race`, a step of `task check`), and whatever else the machine is
doing. On a machine another session was loading, ten gigabytes into an
11.3 GB swap with 8.6 GB of memory (`uptime` read 9 to 17), two `task
check` runs and one push were lost on 2026-09-25 and 2026-09-26 to exactly
these waits, each time a different set: `TestThreeRoleExitScenarioOverLiveServeSubprocess`
(`healthz never became ready`), `TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved`,
`TestMCPCommandServesRealStdioTransport`, `TestMCPSubprocessExitsCleanlyOnSIGTERM`
and `TestMCPRulesetGuideWithAndWithoutRulesetFlag` (`stdio client Connect:
context deadline exceeded`), `TestEventsTailBinaryExitsCleanlyOnSIGINT`
(`subprocess produced no output within 5s`); every one passed alone
afterwards, every failing run had changed no Go line, and in each lost run
a `go build` in the same pass took ten to fifteen times its usual time. The
mechanism is a stall of the whole machine under memory pressure, not CPU
load: the ticket's verification ran sixteen busy loops and a looped `go
build -a` (`uptime` to 43) against the unchanged tree and reddened nothing. The gate (`CLAUDE.md`
rule 2) is `task check` whole, so a run these waits fail is a run that proves
nothing, and the next attempt costs forty minutes.

## Done looks like

1. With a documented load generator running (the plan names it: one busy
   loop per core, or a second `go test ./...` in another checkout), `go test
   -race -count=3 -p 1 -run
   'TestThreeRoleExitScenarioOverLiveServeSubprocess|TestMCPLoadAdventureRoundTripGuideServedAndBatchObserved|TestEventsTailBinaryExitsCleanlyOnSIGINT'
   ./cmd/vtt/` is green; today the same run is red on at least one of them
   (the verification shows a red, or records that it could not provoke one
   and says what it tried).
2. Still true afterwards: a subprocess that never answers fails its test
   within a bound and says so. With `waitForHealthz` pointed at a listener
   that accepts and never answers (a `net.Listen` the test holds), the wait
   returns an error before the test's own timeout; with `vtt mcp` replaced by
   a binary that reads stdin and never writes, the connect under
   `dialMCPSubprocess` (`connectMCPSubprocess`, which returns the error the
   wrapper fatals on) fails with a deadline error. Each is a test case, red
   if the bound were removed.
3. Every wait for a subprocess in `cmd/vtt`'s tests, the eleven sites where
   a subprocess is waited for to answer or to exit, takes its bound from a
   named constant with a doc sentence saying what it bounds, in one file; the
   in-process waits between two ends already running keep their literals.
   `grep -nE '[0-9]+ ?\* ?time\.Second' cmd/vtt/*_test.go` prints no line at
   a subprocess wait, and the report lists the eleven sites; today the grep
   prints 60 lines, eleven of them at those sites.
4. `task check` whole is green, `check:race` included, and
   `task test:external` is green.

## Rules this puts on the system

An e2e wait for a subprocess that never answers ends within a bound and
reports it.

That the bound is long enough for a stalled machine not to expire it is the
ticket's reason and the plan's measurement, not a rule: no check the gate
can run holds it, and the verification could not make the old tree red
under any load it could generate.

## What it touches

1. `cmd/vtt/e2e_wait_test.go`, new: the constants and the negative tests
2. `cmd/vtt/serve_e2e_test.go` (`waitForHealthz`, the SIGTERM test)
3. `cmd/vtt/client_e2e_test.go` (`waitWithTimeout`, the SIGINT test)
4. `cmd/vtt/mcp_e2e_test.go`, `cmd/vtt/mcp_ruleset_e2e_test.go`
   (`dialMCPSubprocess`, split into a returning `connectMCPSubprocess` and
   the fatal wrapper), `cmd/vtt/library_test.go`
5. `docs/requirements.md`, one row after sign-off, by the dispenser
6. `cmd/vtt/qa_e2e_wait_test.go`, Phase 4a's file

The in-process waits in `cmd/vtt/maps_e2e_test.go`,
`mcp_adventure_e2e_test.go`, `events_tail_truncation_test.go`,
`state_dump_cause_test.go` and `scenario_goldens_test.go` are left alone
(sign-off question 2).

Test files only; no production line changes. One component, one commit.

## Specifications this moves

None.

## What could not be established

- The bound itself. The failures were at 5 and 10 seconds during a stall
  the verification could not reproduce with CPU load; the plan's figures
  rest on the arithmetic of the package's ten-minute timeout and on what a
  loaded start cost when measured, and whether the bound scales or is one
  figure is the plan's decision.
- Whether `buildVTTBinary`'s per-test `go build` belongs in `TestMain`
  instead. It runs before each wait starts, so it does not fail a wait, but
  it is the slowest thing these tests do under load; a speed change and not
  this ticket's.
- Whether the same waits fail under `check:coverage`'s subprocess pass
  (`VTT_SUBPROCESS_COVERDIR`, an instrumented binary): the same helpers
  serve it, so the same bound covers it; no run has been observed failing
  there.
- Rule 9 of CLAUDE.md does not apply: no tabletop function is designed;
  MapTool's test suite is not a subprocess harness.
