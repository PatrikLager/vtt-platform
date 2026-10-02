# A deadline test bounds the subprocess's answer, not its shell's start-up

## The problem

`connectMCPSubprocess` (`cmd/vtt/mcp_ruleset_e2e_test.go`) starts the
subprocess and, with nothing between that waits, creates its connect context
with the bound it was handed. The four deadline tests in
`cmd/vtt/qa_e2e_wait_test.go`
(`TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessNeverWrites`,
`TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessIgnoresStdio`,
`TestQAMCPConnectEndsASubprocessThatIgnoresSIGTERM`,
`TestQAMCPConnectFailsPromptlyWhenTheSubprocessExitsWithoutAnswering`) hand it
`neverAnswersBound`, 500 ms, through `qaConnect`, and a shell script from
`qaScript` whose first command writes the shell's pid to a file; after the
connect returns, `qaReadPID` reads that file to find the process and check
that it is gone. On a loaded machine the bound passes, and
`connectMCPSubprocess` kills the subprocess, before the shell has run its
first command, so `qaReadPID` fails with "the fixture never got to run": a
failed precondition, not the behaviour VTT-079 states. `qaBounded` measures
the whole call, start-up included, against `qaPromptly`. It stopped a whole
`task check` run on 2026-09-30 (`check:coverage`) and on 2026-10-01
(`check:race`), and two earlier plans record the same family failing under
load (gap 11 of `docs/superpowers/plans/2026-09-29-the-seat-and-the-perch-have-a-record.md`,
gap 9 of `docs/superpowers/plans/2026-09-29-the-projection-has-a-record.md`).
`docs/verification-debt.md` carries it as open debt.

Not in this ticket: `connectMCPSubprocess`'s behaviour against the real
`vtt mcp` binary, `neverAnswersBound`'s value, `waitForHealthz` and
`waitWithTimeout`, and the other e2e tests.

## Done looks like

1. A deadline test whose fixture shell starts later than `neverAnswersBound`
   still reaches every one of its assertions: a named test with a fixture that
   waits before writing its pid fails on today's tree with "the fixture never
   got to run" and passes after.
2. Each of the four deadline tests still asserts, on every run, the deadline
   (or prompt) error, that the bounded handshake returns within `qaPromptly`,
   and that the subprocess is gone afterwards; the SIGTERM test still runs
   against a shell that has installed its trap.
3. `neverAnswersBound`, `subprocessAnswers` and `qaPromptly` keep their
   values, and `TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites`
   and the other callers of `connectMCPSubprocess` behave as before.
4. The four tests pass under load where they failed: a stated stress run (the
   four tests repeated while the machine is loaded) shows it.
5. `docs/verification-debt.md`'s entry is closed by the test that holds it, and
   `task check` whole is green.

## Rules this puts on the system

- A deadline test's bound covers the subprocess's answer, not its start-up.

## What it touches

1. `cmd/vtt/mcp_ruleset_e2e_test.go` (`connectMCPSubprocess`) and its callers
   in `cmd/vtt/e2e_wait_test.go`, `cmd/vtt/qa_e2e_wait_test.go` and
   `cmd/vtt/mcp_ruleset_e2e_test.go`.
2. `cmd/vtt/qa_e2e_wait_test.go` (`qaConnect`, `qaBounded`, `qaScript`,
   `qaReadPID`).
3. `docs/verification-debt.md`; `docs/requirements.md`.

## Specifications this moves

None.

## What could not be established

- Whether a start-up wait belongs inside `connectMCPSubprocess`, as an option
  only the fixture uses, or in `qaConnect` around it; the plan decides.
- How long a shell may take to start before the test should fail rather than
  wait; `subprocessAnswers` is the existing bound for a subprocess's first
  answer.
- Whether the rule earns a row of its own or is VTT-079's, which states the
  bound and not what it covers.
