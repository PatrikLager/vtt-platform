# A deadline test bounds the answer, not the start-up — implementation plan

**Ticket:** `docs/superpowers/specs/2026-10-01-a-deadline-test-bounds-the-answer-not-the-start-up-design.md`
**Verified:** 2026-10-01, by `verify-ticket`, an agent that did not write the
ticket, against `759bced` on `fix/mcp-deadline-fixture` (`main` at the time;
the ticket untracked). Verdict: **Passes with gaps.** The gaps are listed at
the end and travel with this plan. This plan does not edit the ticket.

**Goal.** The four `TestQAMCPConnect*` deadline tests hand
`connectMCPSubprocess` a 500 ms bound that today also covers the fixture
shell's start-up. When start-up outlasts the bound, the shell is killed before
it writes its pid and the test fails on its precondition. After this ticket
the bound starts once the fixture has started. No bound is raised and no
assertion is skipped.

**The owner's ruling, settled and not re-opened.** Option 1: the bound covers
the MCP handshake, not the shell's start-up. The fixture waits for its pid
file under a generous start-up bound before the 500 ms bound begins.

**MapTool (`CLAUDE.md` rule 9).** Rule 9 covers areas a virtual tabletop
already has to solve. A test fixture's start-up under a handshake bound is
not one of them, so this plan did not look at `~/dev/RPTool`, and it records
that it did not.

## Verification, check by check

1. **Every path resolves — by command.** All seven files the ticket names
   exist. Each symbol it names is in the file it names, once:
   `connectMCPSubprocess`, `qaConnect`, `qaBounded`, `qaScript`, `qaReadPID`,
   `qaPromptly`, `neverAnswersBound`, `subprocessAnswers`, the four tests,
   `TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites`, the message
   "the fixture never got to run", VTT-079's row and the debt entry.
   `waitForHealthz` is in `serve_e2e_test.go` and `waitWithTimeout` is in
   `client_e2e_test.go`. Gap 11 and gap 9 of the two cited plans say what the
   ticket says they say.
2. **"Done" is an observation — by command.** Item 1 fails today. A fixture
   that runs `sleep` for twice `neverAnswersBound` before writing its pid,
   connected through today's `qaConnect`, failed 5 of 5 runs with "the fixture
   never got to run: open …/pid: no such file or directory". It passes on the
   verifier's sketch. Items 2, 3 and 5 are observations, shown below by
   breaks, a diff and the gate. Item 4 is an observation, but "while the
   machine is loaded" does not reproduce the failure as worded: CPU load left
   today's tree green (gap 1). The run in D10 does reproduce it.
3. **Each rule is breakable — a reading.** The one rule, "a deadline test's
   bound covers the subprocess's answer, not its start-up", has an observation
   that would fail. A fixture whose start-up outlasts the bound fails on its
   precondition. That observation is the item 1 test (row A).
4. **The scope matches the claim — by command, then a reading.**
   `connectMCPSubprocess` has four call sites: `qaConnect`,
   `TestQAMCPConnectSucceedsAgainstASubprocessThatAnswers`,
   `TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites` and
   `dialMCPSubprocess`. `dialMCPSubprocess` is called twice in
   `mcp_ruleset_e2e_test.go` and once in `mcp_adventure_e2e_test.go`.
   `qaScript` has five callers, all in `qa_e2e_wait_test.go`. Under D3,
   `connectMCPSubprocess` keeps its signature, so the only edits are the two
   files and the three records the ticket names. The ticket names
   `e2e_wait_test.go` and the plan leaves it unedited. The ticket does not
   name `mcp_adventure_e2e_test.go`, an indirect caller, and the plan leaves
   that unedited too. The scope holds.
5. **No recorded decision is contradicted silently — a reading.** No
   specification or ADR covers e2e waits:
   `grep -il 'subprocess\|neverAnswersBound\|e2e wait\|deadline test\|VTT-079\|qaConnect\|connectMCPSubprocess'`
   over `docs/specifications/*.md` and `docs/adr/*.md` prints nothing.
   VTT-079 stays true as worded. The 2026-09-26 report refused "an e2e test
   waits long enough that load alone does not fail it", and this ticket does
   not re-open that refusal: it raises no bound, and its rule is narrower.
6. **The records the work moves are named, and reachable — by command, then a
   reading.** The ticket says `None.`, and that is correct: the search in
   check 5 finds no specification describing these helpers.

## Measurements this plan stands on

All taken at `759bced`, by command, in a scratch clone and never in the
working tree. The sketch is the verifier's (D3 to D6), not the plan's code. It
showed the design runs and was then discarded. Load figures are `uptime`.

- **Start-up is the first exec of a newly written file.** Fifteen samples
  each, from `cmd.Start` to a complete pid line:
  - a freshly written script: median 233 ms, max 247 ms;
  - the same script run again: median 6 ms;
  - a fresh script run as `/bin/sh <script>`: median 4 ms;
  - a fresh script under 16 `yes` loops: median 225 ms, max 1.24 s;
  - a fresh script under 8 loops that each write and run a new script
    (generator C, D10): min 1.01 s, median 1.06 s, max 1.48 s.

  So even on an idle machine about 46% of the 500 ms bound goes to start-up.
  Today the three deadline tests take 0.51 s each. Blaming macOS's check on
  the first exec of a new file is an inference: what was measured is that a
  new file costs this and a re-run does not.
- **CPU load does not reproduce the failure; fresh-exec load does.** Today's
  four tests, `-test.count=10`:
  - 16 `yes` loops (load to 16): 40 of 40 green;
  - 32 `yes` loops (load to 35): 40 of 40 green;
  - generator C: 0 of 40 green;
  - generator C at `-test.count=20`: 0 of 80 green, every failure "the
    fixture never got to run".

  The sketch under generator C:
  - 40 of 40 green, then 80 of 80, with `took` at most 528.0 ms;
  - a `-race` build, the late-start test included, `-test.count=10`: 50 of
    50 green, `took` at most 509.4 ms.
- **The SIGTERM probes**, under generator C, `-test.count=3`, with the kill
  replaced by `Signal(SIGTERM)`:
  - today's tree: red 3 of 3, on the precondition;
  - today's tree plus a fallback that reads a missing pid file as a gone
    process: **green 3 of 3**, a false green;
  - the sketch: red 3 of 3 with "connectMCPSubprocess did not return within
    20s".

  The broken runs leaked one `sleep 1000` each, which ignores SIGTERM. All
  were killed with SIGKILL.
- **The breaks in D9**, on the sketch with the machine idle, gave the results
  listed there.
- **Run time.** Whole `go test -count=1 ./cmd/vtt/`, alternating, idle: today
  131.8 s and 132.8 s, the sketch 134.8 s and 136.1 s. Mean of three idle runs
  of the `TestQAMCPConnect*` set:
  - today, five tests: 2.15 s in total;
  - the sketch, six tests: 4.69 s in total, the three deadline tests going
    from 0.51 to 0.78 s each and the new test taking 1.77 s.

  `cmd/vtt` runs three times per `task check` (`check:coverage` twice,
  `check:race` once) and once at pre-push (`test:external`), so the change
  costs about 10 s per gate.
- **The mutation gate is untouched.** `cmd/vtt` is not among the 14
  `PACKAGES` in `tools/check-mutation.py`, and no gated package's
  `go list -deps -test` reaches it. So no key moves, the skip cache does not
  change and the run time does not change.
- **Gates on the sketch:**
  - `golangci-lint run ./cmd/vtt/...`: 0 issues.
  - `check-new-prose.py main`: clean.
  - `check-requirements-chain.py .`: 252 rows, clean.
  - `check-doc-owner.py .`: clean.
  - `check-comments.py main` and the comment ceilings: see D8.
- **Disk.** 14 GiB free on the volume that holds `TMPDIR`. GOCACHE is 4.1 GB.
  `MIN_FREE_BYTES` in `tools/check-mutation.py` is 16 GiB.

## Constraints that bind every task

- `CLAUDE.md` rule 2: no bound changes value. That covers `neverAnswersBound`,
  `subprocessAnswers`, `subprocessExits`, `qaPromptly` and `qaCeiling`. No
  assertion is removed, weakened or reworded. The comment ledger only goes
  down, and only through `--write-ledger`.
- Rule 8: this plan, the report and the debt entry cite names, never a line.
- Rule 10 and SPEC-010: no comment line is added to `qa_e2e_wait_test.go`
  (D8). Any comment added elsewhere is a warning or a pointer, inside
  `check:new-prose`'s 55 to 85 band, in a block of at most 6 lines.
- SPEC-008: ids come only from `requirement-id`, after sign-off, at the start
  of Task 1.
- The `requirements` skill: one thing, breakable, named by a check; each
  refusal gets one line.
- Tests come before code. One deliberate break per check, both directions,
  each recorded as a line in the commit (D9).
- A reading review's package is `git diff HEAD`. Run `git add` and
  `git commit` as separate calls, then `git show --stat HEAD`.
- Run `uptime` before every timed or stress run and before the gate. Launch
  `task check` detached with `start_new_session=True`. Give `git push`
  600 000 ms and never kill it under five minutes.
- These are not edited: the ticket, `docs/adr/`,
  `docs/reports/2026-09-26-e2e-waits-survive-a-loaded-machine.md`, and the
  other callers of `connectMCPSubprocess`.

## Decisions this plan makes

**D1. The bound starts after start-up.** The fixture's pid line marks the end
of start-up. `qaConnect` waits for it, then hands the started subprocess to
the bounded handshake. *Forced by:* the ruling, and the start-up measurement:
about 230 ms idle and up to 1.48 s under fresh-exec load, against a 500 ms
bound.

**D2. Three options are refused.**
- **Fall back to the process table when the pid file is missing.** Measured
  false green: with a SIGTERM-only kill, the SIGTERM test passed 3 of 3 under
  load, because the shell died before installing its trap.
- **Raise the fixture's bound.** It moves the threshold. Start-up grows with
  contention (1.48 s at 8 loops), and the ruling refuses it.
- **Run the fixture through `/bin/sh`** so that no new file is executed. It
  shrinks start-up from 233 to 4 ms, but the bound still covers start-up, so
  the rule is still broken. It also cannot be done without changing how
  `connectMCPSubprocess` builds its command line. It too moves the threshold.

**D3. The wait lives in `qaConnect`, and the helper splits in two.**
- `startMCPSubprocess(binPath, wsURL, token string, extraArgs ...string)
  (*mcpSubprocess, error)` builds the command, the pipes and the stderr
  buffer, and starts the process.
- `(*mcpSubprocess).connect(t, bound)` is today's body from the transport
  onward.
- `(*mcpSubprocess).kill()` is today's `Kill` then `Wait`.
- `connectMCPSubprocess` becomes `startMCPSubprocess` followed by `connect`.
  Its signature, its doc comment and its order of operations stay as they
  are.

*Forced by:*
- `connectMCPSubprocess` creates the bounded context after `cmd.Start` with
  nothing between that waits, so nothing outside it can wait unless the
  helper changes.
- `qaBounded` starts both its clock and its `qaCeiling` (20 s) watch before
  whatever it wraps. If the wait went inside `connectMCPSubprocess`, as an
  option only the fixture passes, it would run inside `qaBounded`. Then
  `took`, asserted under `qaPromptly`, would still include start-up. Done
  item 2 says "the bounded handshake", so that would keep half the rule. The
  start-up wait would also have to stay under 20 s, which couples two bounds
  or raises one.
- Under the split, `qaBounded` wraps exactly `connect`: the handshake, the
  kill and the reap, which is what VTT-079 bounds.

*Cost:* `qaConnect` must kill and reap the process on its own failure path.

**D4. The start-up wait is bounded by `subprocessAnswers` (40 s).** It polls
every 10 ms, like `qaAwaitEnded`, until the pid file holds a complete line,
meaning it ends in a newline. If the wait runs out, `qaConnect` kills and
reaps the subprocess, then fails with "the fixture did not write its pid
within 40s". *Forced by:* the ticket names `subprocessAnswers`, the pid line
is the fixture's first output, and no new constant is needed. The wait runs
outside `qaBounded`, so `qaCeiling` does not limit it. On a green run it costs
only the real start-up.

**D5. The pid line comes after a setup the test supplies.**
`qaScript(t, setup, body)` writes `#!/bin/sh`, then `setup`, then the pid
line, then `body`. The SIGTERM test moves `trap '' TERM` into `setup`. The
other four callers pass `""`. *Forced by:* Done item 2 requires the trap to be
installed before the kill. Today the pid line comes first, so a pid file shows
only that the shell ran its first command. Measured: the sketch reds the
SIGTERM-only break on the ceiling, which means the trap was in place.

**D6. The new test is `TestQAMCPConnectReachesItsAssertionsWhenTheFixtureStartsLate`.**
- Its `setup` is `sleep` for `2 * neverAnswersBound`, written as a multiple
  of the bound as the file's own warning about literals requires.
- Its body never writes.
- It makes the three assertions `TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessNeverWrites`
  makes, in the same words.
- It goes after the closing brace of
  `TestQAMCPConnectFailsPromptlyWhenTheSubprocessExitsWithoutAnswering` and
  before the next `// VTT-079` line, so it takes no other test's doc block.
  `check:doc-owner` does not read test files, per the open debt entry.

*Forced by:* Done item 1, and the measured red on today's tree, 5 of 5.

**D7. Every failure message stays as it is** (a preference, named as one).
That includes "connectMCPSubprocess took %s against a %s bound" and
`qaBounded`'s label inside `qaConnect`. The call being measured is
`connectMCPSubprocess`'s handshake half. The messages belong to VTT-079's
tests, and rewording them is outside this ticket.

**D8. No comment line is added to `qa_e2e_wait_test.go`, and the ledger goes
down by two rows.** Measured against its 1.6 ceiling:
- a two-line comment brings the share to 1.85 and is refused;
- one line brings it to 1.62 and is refused;
- none gives 1.4.

The new helpers in `mcp_ruleset_e2e_test.go` are unexported and get no doc
comment (lint reports 0 issues). That file then falls to 23.01, more than 1.0
below its 24.4 ceiling, which `check:comments` refuses until the ledger is
lowered. `--write-ledger` lowers `cmd/vtt/mcp_ruleset_e2e_test.go` from 24.4
to 23.1 and `cmd/vtt/qa_e2e_wait_test.go` from 1.6 to 1.4, and moves no other
row. The gate is clean afterwards. *Forced by:* SPEC-010's ceiling and band.
The cost is gap 2.

**D9. The deliberate breaks.** Each is an act in a scratch state, repaired
before the commit, and recorded as one line of the commit message.

| Check | The break, and what must go red | The other direction |
|---|---|---|
| row A, the new test | `qaConnect` connects without waiting for the pid line. The new test reds with "the fixture never got to run"; the four stay green on an idle machine (measured). | The repaired code is green on an idle machine, and under generator C for the four (80 of 80, measured). |
| VTT-079, the error | A failed `connect` returns a nil error. All five red: "want a deadline error, got <nil>", and "returned nil" for the one that exits (measured). | Repaired: green. |
| VTT-079, `qaPromptly` | The connect context gets `20*bound`. Three deadline tests and the new test red with "took 10.0…s against a 500ms bound". The one that exits stays green, as it does today, because EOF ends it (measured). | Repaired: green. |
| VTT-079, gone | `kill` sends `Kill` without `Wait`. All five red with "alive", or "zombie" for the one that exits (measured). | Repaired: green. |
| the trap (D5) | `kill` sends `Signal(SIGTERM)`, under generator C. The SIGTERM test reds with "did not return within 20s" in every run (3 of 3, measured), not on the precondition. Afterwards, `kill -9` the leaked `sleep 1000` processes and confirm `pgrep -f '^sleep 1000$'` prints nothing. | Repaired, under generator C: green. |

**D10. The stress run for Done item 4.**
- **Generator C** is 8 background loops, each running
  `d=$(mktemp -d); i=0; while :; do i=$((i+1)); f="$d/s$i.sh"; printf '#!/bin/sh\nexit 0\n' > "$f"; chmod +x "$f"; "$f"; rm -f "$f"; done`.
  Wait 3 s after starting them.
- **The run.** Build with `go test -c -o <bin> ./cmd/vtt/`. Then, from
  `cmd/vtt`, run
  `<bin> -test.count=20 -test.v -test.run '^TestQAMCPConnect(FailsWithADeadlineWhenTheSubprocessNeverWrites|FailsWithADeadlineWhenTheSubprocessIgnoresStdio|EndsASubprocessThatIgnoresSIGTERM|FailsPromptlyWhenTheSubprocessExitsWithoutAnswering)$'`.
  Do it on a build of the base and on a build of the change, plus one `-race`
  build of the change at count 10 with the new test included.
- **Afterwards**, kill the loops and confirm `pgrep -f` prints nothing.
- **Expected**, as measured: the base 0 of 80 green, every failure on the
  precondition; the change 80 of 80 green.
- **Not the stress:** CPU burners. They left the base green.

*Forced by:* the measurements. A stress that does not redden the base shows
nothing.

**D11. The register gets one new row (A), and VTT-079 is unchanged.** The new
test cites A and only A. The four tests keep `// VTT-079`. *Forced by:* when
the new test goes red, VTT-079 has not been broken: the connect still ended
within its bound and reported. A citation should name the rule whose breach
reddens the test. (Q2)

**D12. The debt entry is closed in C1, by the test that closes it.**
- The entry moves out of "Open debt" into a dated section
  `## 2026-09-30 — the deadline fixture failed when its shell started late`,
  placed before `## Open debt`, with its text unchanged.
- One line is appended:

  > **Closed by** `TestQAMCPConnectReachesItsAssertionsWhenTheFixtureStartsLate`
  > in `cmd/vtt/qa_e2e_wait_test.go`, which reds when `qaConnect` connects
  > without waiting for the pid line (observed <date>).

- Gap 2 becomes a new open entry.

*Forced by:* an entry left under "Open debt" after it is closed would say the
opposite of the truth. The precedent is the 2026-09-23 entry's
**Closed by**. No open-debt paragraph has ever been closed: the file's
history removes no entry. (Q4)

## The sort

Rows are lettered, and nothing becomes an id before sign-off.

| # | Rule (what) | Proposed | The observation that goes red | Test | Commit |
|---|---|---|---|---|---|
| A | A deadline test's bound on a subprocess's answer excludes the subprocess's start-up. | accept, the ticket's rule, as a new row (Q2) | a fixture whose start-up outlasts `neverAnswersBound` fails on "the fixture never got to run" | `qa_e2e_wait_test.go#TestQAMCPConnectReachesItsAssertionsWhenTheFixtureStartsLate` (new; red today 5 of 5, measured) | C1 |

**Refused, one line each:**

1. "Each deadline test reaches every assertion on every run" is how A is
   achieved. The breaks in D9 hold it.
2. "The SIGTERM test runs against a shell that has installed its trap" is a
   fixture's order, which is how VTT-079's SIGTERM case keeps its power. No
   check holds it after the commit (gap 2).
3. "The three constants keep their values" is a constraint (rule 2), with
   nothing in the system to observe.
4. "The other callers behave as before" is a constraint. Their own tests hold
   it.
5. "The four tests pass under load" is a measurement, refused for the same
   reason the 2026-09-26 report gave for its rule A: no check the gate runs
   holds it.
6. "The start-up wait is bounded by `subprocessAnswers`" describes how, and
   putting a number in a rule is out.
7. "The debt entry is closed" and "`task check` is green" are records and
   gates, not rules.

**Existing rows:** VTT-079's text and evidence are unchanged, and it stays
true as worded.

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository.
- `uptime` and `df -h $TMPDIR`.
- `which requirement-id`.
- `python3 tools/check-comments.py --report`, filtered to the three
  `cmd/vtt` files.
- `python3 tools/check-requirements-chain.py .`.
- `git status --short`, which should show the ticket and this plan untracked.

**Done when:** these match Measurements: 252 rows; shares of 24.4 and 1.5
against ceilings of 24.4 and 1.6.

### Task 1 — The bound starts after start-up (C1)

**Files:**
- `cmd/vtt/mcp_ruleset_e2e_test.go` (D3);
- `cmd/vtt/qa_e2e_wait_test.go` (D4 to D6);
- `docs/requirements.md` (row A);
- `tools/comment-ceilings.txt` (D8);
- `docs/verification-debt.md` (D12, and gap 2's entry);
- the ticket and this plan, which are committed with C1 as house practice.

**Test first.**
1. Add `setup` to `qaScript`: four callers pass `""` and the SIGTERM test's
   trap moves into `setup`. The package stays green.
2. Write the new test (D6), calling today's `qaConnect`. It must go red with
   "the fixture never got to run" (measured 5 of 5).

**Do.**
1. Dispense row A and put its id on the new test.
2. Split the helper (D3).
3. Give `qaConnect` the `pidFile` parameter and the wait (D4). Its four
   callers pass `pidFile`.
4. Run the breaks in D9 and the stress run in D10. Record their lines for the
   commit message.
5. Run Phase 4a: one QA agent, given row A, VTT-079, the ticket's Done items
   and the signatures of `startMCPSubprocess`, `connect`, `qaScript` and
   `qaConnect`, without their bodies. Record its adjudications in the report.
6. Run Phase 4b.
7. Run `--write-ledger` last, then `check:comments` clean.

**Done when:**
- `go test -count=1 ./cmd/vtt/` is green;
- D10's numbers hold;
- `git diff HEAD -- cmd/vtt/e2e_wait_test.go cmd/vtt/mcp_adventure_e2e_test.go`
  prints nothing;
- `git diff HEAD` shows no change to any of the five bound names;
- the chain checker reports 253 rows;
- the commit is made.

### Task 2 — The whole gate

Run `uptime` and `df -h $TMPDIR`. At least 16 GiB must be free (Q6). Launch
`task check` detached, once, after C1's review has settled.

**Done when:** it exits 0 and every step prints its own completion line. A
failure is fixed in a new commit through the same cycle.

### Task 3 — The report (C2), then push

Write a new `docs/reports/2026-10-0N-a-deadline-test-bounds-the-answer-not-the-start-up.md`
per the `implementation-report` skill. It answers the ticket's five items,
names C1 as the last code commit, and covers:
- the start-up measurements and the mechanism, as an inference;
- the base and change stress numbers;
- the break lines;
- the QA adjudications under their own heading;
- the sign-off answers;
- the gaps.

Then: its own review and commit. Fetch, merge if behind, push.

## Commits

| Commit | Carries | Gate steps |
|---|---|---|
| C1 | the split, the wait, `setup`, the new test, row A, the two ledger rows, the debt closure and gap 2's entry, the ticket and this plan | hook; pre-push (`test:external` runs `cmd/vtt`); `task check` whole |
| C2 | the report | hook |

The pre-commit hook does not run these tests: `cmd/vtt` is tier 3. Pre-push
runs them in `test:external`, and `task check` runs them in `check:coverage`
and `check:race`.

## Gaps that travel with this plan

1. **Done item 4's "while the machine is loaded" does not reproduce the
   failure as worded.** CPU load left the base green, 80 of 80. D10 states
   the run that does reproduce it. This plan sharpens the item and does not
   change it.
2. **Nothing checks that the pid line comes after `setup`.** A later edit
   that moves it first keeps every test green, and the SIGTERM test would
   then lose its trap whenever load delays the shell. Only a reading holds
   the order, because the comment ceiling refuses a warning line (D8). It is
   recorded in C1 as open debt, labelled `test asserts nothing`.
3. **Other e2e tests also run a newly built file.** The `vtt` binary from
   `go build` pays the same first-exec cost. Their bounds are
   `subprocessAnswers` (40 s), so the 1.48 s measured here does not reach
   them, but nobody has measured that cost for a fresh Mach-O binary under
   contention.
4. **The mechanism is inferred.** Measured: a new file costs about 230 ms to
   start and a re-run does not. That the cost is macOS's check on a first
   exec is not established. CI, when someone dispatches it by hand, may not
   see the cost at all.
5. **`task check` cannot pass on this machine below 16 GiB free.** 14 GiB is
   free now (Q6).

## Questions for sign-off

1. **Does the wait live in `qaConnect`, with `connectMCPSubprocess` split into
   start and connect (D3)?** Recommend yes. It is the only placement in which
   `qaBounded` measures exactly what VTT-079 bounds without coupling the
   start-up wait to `qaCeiling`. Every other caller keeps its call
   unchanged.
2. **Is the rule a new row A, cited only by the new test (D11)?** Recommend
   yes. The new test reddens when VTT-079 still holds, so citing VTT-079
   would point the register at the wrong rule.
3. **Is the start-up bound `subprocessAnswers`, 40 s (D4)?** Recommend yes. It
   is the existing bound for a subprocess's first output, it costs nothing on
   a green run, and it is 27 times the worst start-up measured.
4. **Does the debt entry move under a dated heading with a Closed-by line
   (D12), rather than staying in "Open debt" with the line appended?**
   Recommend moving it. A closed entry under "Open debt" misstates the file.
5. **Is gap 2 recorded as open debt, rather than paying for a one-line
   warning comment with code?** Recommend open debt. Padding code to buy
   comment share works against the purpose of the ceiling.
6. **May the implementer run `go clean -cache` before the whole gate?**
   Recommend yes. It frees about 4 GiB, enough to clear the 16 GiB floor once
   the verifier's clone is gone. It is the gate's own remedy.
7. **One code commit and a report, with `task check` whole once after C1?**
   Recommend yes. The change is a single TDD cycle in two test files.
