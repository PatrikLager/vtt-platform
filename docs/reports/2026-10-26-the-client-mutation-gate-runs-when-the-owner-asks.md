# The client's full mutation run happens when the owner asks, and `task check` says when it last did: the change

**Ticket:** `docs/superpowers/specs/2026-10-26-the-client-mutation-gate-runs-when-the-owner-asks-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to
G6; G1, a false sentence of the problem, was corrected before Task 0's
commit, and the rule on the hash and item 5 were widened by the owner's
answer to Q3.
**Plan:** `docs/superpowers/plans/2026-10-26-the-client-mutation-gate-runs-when-the-owner-asks.md`,
written by that verifier and amended by the owner's answers and by Task
0's review.
**Where it came from:** the owner's question of 2026-10-26, "det känns som
du kör 'full check' med alla mutations för varje ticket, oberoende hur
stor den är", and his question whether Stryker could run per changed file.
**The owner's rulings, all 2026-10-26:**
- "ok, ny regel då, vi kör bara Full Stryker, när jag efterfrågar det. Du
  påminner mig och räknar upp alla ändringar (tickets) som gjorts sedan
  senaste Stryker körningen. Och jag avgör om jag vill att Stryker ska
  köras."
- "Går igenom med varning": with the stored report out of date, `task
  check` runs no Stryker, names the report's commit and the tickets since,
  and passes, the old report not checked.
- "vi kör inte CI på github längre".
- "ja, ta det som nästa ärende".
- On the plan's questions: Q1 "Ja", the report made on `db81c37` is stamped
  current; Q3 "Ja", the hash covers what the client's tests read by path;
  Q4 "Ny SPEC-023". Q2, the full run's name, was not put to him: the
  writer of the ticket took the recommendation, `check:ts-mutation:full`.
- The plan signed off: "Ja, kör".

**Last code commit:** `1268bee`, on `669c4d4`, `main` at the time. Every
code reference below is to its tree.

## The period, in commits

| Commit | What it holds |
|---|---|
| `ca41ac2` | Task 0: the ticket, the plan, VTT-507 to VTT-516 |
| `4e559a6` | Task 1: `tools/ts-mutation-report.py`, `check:ts-mutation` judging only, `check:ts-mutation:full`, `check:ts-mutation:docker` removed, the tests, QA's `tools/ts_mutation_report_qa_test.py` |
| `a6c2d3c` | Task 2: the input hash widened, the tests, QA's `tools/ts_mutation_inputs_qa_test.py` |
| `1268bee` | Task 4: `CLAUDE.md`, the comments of `.github/workflows/ci.yml`, a sentence of `check:ts-mutation:full`'s desc in `Taskfile.yml`, `docs/verification-debt.md`'s recipe, a comment in `contract/events.test.ts` |

Task 3 ran locally and committed nothing (below, under item 3).

## What the code did on `669c4d4`

- `check:` in `Taskfile.yml` ran `check:ts-mutation`, whose last command
  compared `tools/ts-mutation-inputs.py`'s hash with
  `reports/mutation/ts-inputs.sha256`. When they differed, or there was no
  `reports/mutation/mutation.json`, it started Stryker: on macOS 26 by `task
  check:ts-mutation:docker`, elsewhere by `npx stryker run`, and then wrote
  the hash to the stamp. On every path it judged the report with
  `tools/check-ts-mutation.py`.
- The stamp held the hash alone, and `mutation.json` names no commit, so
  which tree the report measured was recorded nowhere.
- The hash read `client/src` and `client/test` and five files. It did not
  read `contract/gen/ts`, which twelve files of `client/src` import, nor
  what the client's tests read outside `client/` by a path written in them.

## Done looks like, answered

1. [x] Run with the client changed since the stored report,
   `check:ts-mutation` starts neither Stryker nor its container, exits 0,
   and prints that the client's mutation was not measured, the commit the
   report measured and every ticket added since.
   - `tools/ts_mutation_report_test.py`'s
     "test_an_out_of_date_report_is_not_judged_and_names_its_commit_and_the_tickets_since"
     and "test_out_of_date_and_missing_reports_start_nothing", the second
     with stand-ins for `docker`, `npx`, `bunx`, `stryker` and `task` first
     on `PATH` and their log asserted absent.
   - "test_task_check_starts_no_stryker_run_and_no_container", over `task
     --dry check`, which runs none of its commands.
   - QA's "test_a_stale_report_passes_without_being_judged",
     "test_a_stale_report_lists_the_tickets_its_commits_tree_lacks" and
     "test_the_stale_path_starts_nothing_but_git".
2. [x] Run with no stored report, `check:ts-mutation` starts neither, exits
   0, and prints that there is no report: "test_no_stored_report_passes_and_says_there_is_none",
   the second test of item 1, and QA's
   "test_no_report_passes_and_says_there_is_none".
3. [x] Run with a current report, `check:ts-mutation` judges it against
   `tools/ts-mutation-equivalents.txt`, and a survivor no entry claims
   fails it: "test_a_current_report_is_judged_and_the_step_exits_with_the_verdict",
   and QA's "test_a_survivor_no_entry_claims_fails_the_step", which runs
   the real checker in a fixture. On the repository, after Task 3, `python3
   tools/ts-mutation-report.py check` prints "the stored report is current
   (its inputs are the tree's); it measured db81c37aa307" and the checker
   prints "3276 mutants, 3204 killed, 9 timed out, 63 survivors all
   adjudicated equivalent, zero unadjudicated survivors".
   - Task 3, by Q1's "Ja": the 24 `source`s of the stored report equal
     `git show db81c37:<path>`; the hasher of `db81c37`, run on a copy of
     `db81c37`, prints the old stamp, `7a6971ce…`; and `git diff --quiet
     db81c37 HEAD`, with `git status --ignored`, over every path the new
     hash reads, shows nothing, against a control that does
     (`e33376f..db81c37` over `client/src`).
   - The stamp was then written through the script's `write_stamp`,
     naming `db81c37aa307c3583f6eba7cf18764eaecdd57ca` with `clean` null and
     no `started`, and the old stamp, which read
     `7a6971ce34f4dbadda7d8e548e85cd2965f3825c3ef9332bc1dbb9aae18cd3f8`, was
     removed from `reports/mutation/`.
   - `tools/check_ts_mutation_qa_test.py`'s eight tests of the real report,
     which skipped while the report read out of date, run again.
4. [x] `check:ts-mutation:full`, not a step of `task check`, takes the
   snapshot, runs Stryker over the unchanged `mutate` list, stamps, and then
   judges; a `check:ts-mutation` right after it calls the report current
   and names the commit.
   - "test_the_full_run_stamps_after_stryker_and_before_it_judges", over
     `task --dry check:ts-mutation:full`.
   - "test_a_full_runs_report_is_current_and_judged_whatever_its_verdict",
     where a stand-in checker's 1 survives the stamp.
   - QA's FullRun class, which runs the block that dry run prints under
     `/bin/sh` in a fixture, with `docker`, `npx`, `uname` and `python3`
     replaced by stand-ins that write a report as Stryker would, after
     asserting each name resolves to its stand-in.
   - `git diff 669c4d4 HEAD -- stryker.conf.json` prints nothing.
   - A real full run is not observed: it needs the owner's word (G3).
5. [x] A change to `events_pb.ts`, to `join_url_format.json` and to a golden
   each moves the input hash: `tools/ts_mutation_inputs_test.py`'s
   "test_the_generated_contract_moves_it", "test_the_contract_testdata_moves_it"
   and "test_a_golden_moves_it", and QA's
   "test_item_5s_three_files_each_move_the_hash". On a copy, the three
   edits give four distinct hashes.
6. [x] `git grep -n -E "both mutation gates|IN .task check.|now the gate
   runs it" -- CLAUDE.md Taskfile.yml` prints nothing, and nothing outside
   `docs/reports`, `docs/superpowers` and `docs/adr` names
   `check:ts-mutation:docker` but the two tests that assert it is gone. Held
   by Task 4's review, which searched the rest of the tree (Phase 4b).
   - `docs/specifications/`: ``perl -0ne 'print "found\n" if
     /check:ts-mutation`?\s+steps?\s+run\s+inside\s+`task check`/'`` over each
     file prints `found` for SPEC-020 at `1268bee`, and nothing after the
     specification commit that follows this report. The plan's one-line
     `git grep` of that phrase prints nothing at `1268bee` too, since the
     phrase spans a line break, so it shows nothing.

## What the rules became

- "`task check` never starts Stryker": VTT-507, item 1's tests and
  QA's "test_task_check_names_no_stryker_and_no_container".
- "With the stored report out of date, `task check`'s client mutation step
  passes, and says the client's mutation was not measured, which commit the
  report measured, and which tickets were added since": two rows, VTT-508
  for the pass without judging and VTT-509 for what is said, since a break
  of each reds a different assertion.
- "With no stored report, the step passes and says there is none":
  VTT-510.
- "With the stored report current, the step judges it as it does today":
  VTT-511.
- "The full run records which commit's tree it measured": VTT-512. The
  sort gave three rows beside it, all of the full run's stamp: VTT-513,
  whether a file the hash reads differed from that commit; VTT-514, no
  stamp without a report newer than the run's start; and VTT-515, a red
  verdict staying red at the next `task check`.
- "A change to any tracked file a client source or test imports, or that a
  client test reads by a path written in it, moves the report's input
  hash": VTT-516, Task 2's tests.

The sort refused four, under the plan's "Requirements": the unchanged
`mutate` list, which `test_the_hashed_trees_match_what_stryker_actually_mutates`
already pins; item 6, a reading; an unhashable tree, a case of out of date;
and the stamp's keys, the order of the tickets and the wording of the
messages, which are how. QA listed four more it relied on (below); none
was allocated.

## Phase 4a

One QA agent per task that delivered behaviour, each blind to the
implementation, its tests and the diff, and each given the rows, the
ticket, the plan's decisions for its task verbatim (Task 1's D2 to D7 and
D11, Task 2's D8), and the command-line surface. Task 0's records and Task
4's prose had none to derive.

| Task | Tests, as committed | Failures |
|---|---|---|
| 1 | 60 in `tools/ts_mutation_report_qa_test.py` | 2 |
| 2 | 20 in `tools/ts_mutation_inputs_qa_test.py` | 0 |

Task 1's QA made 47 injections by varying fixtures, all red; Task 2's ran
its suite against nine wrong stand-in hashers, each red where it should
be, and searched the client itself for what it reaches outside
`client/`, finding nothing the hash misses.

### QA adjudications

- QA finding 1, Task 1: behaviour defect. An ignored file the hash reads
  left `clean` true, against VTT-513's "or was untracked". `git status`
  gained `--ignored`. No ignored file stood under a hashed tree.
- QA finding 2, Task 1: behaviour defect. A note began with the stamp's
  commit and capitalised its first letter, so an unknown SHA starting a–f
  printed upper-case. The notes now begin "The stamp's commit, …".
- QA finding 3, Task 1, a spec gap: `stamp` wrote `clean: "yes"`. It now
  refuses a snapshot whose fields are not of their kind.
- QA finding 4, Task 1, a spec gap ruled no code: a `thresholds.break` added
  to `stryker.conf.json` would make Stryker exit non-zero after writing its
  report, `set -e` would stop the full run before the stamp, and the next
  `task check` would call the report out of date. Nothing is hidden: the
  full run itself fails, and the step says the mutation was not measured.
- QA finding 5, Task 1, a spec gap: tickets in a subfolder of
  `docs/superpowers/specs/` are not listed. None stands there; SPEC-023
  says tickets are the folder's own files.
- QA finding 6, Task 2: a test pinned that the tree before the ticket
  hashes as the working tree, true of this ticket and false after the next
  client change. Removed before Task 2's commit.
- QA finding 7, Task 2, spec gaps, no code: an untracked file with a hashed
  suffix moves the hash, more than VTT-516 asks; a missing hashed file
  changes the hash rather than refusing, as `bun.lockb` always did.
- QA finding 8, Task 1: `stamp`'s refusal of a report whose mtime equals the
  snapshot's start said "older than". It says "not newer than".
- QA finding 9, Task 1, a spec gap: `_report_is_current` cannot be reached
  from QA's surface. The tests of the real report skipped until Task 3
  stamped it, and run since.
- QA finding 10, Task 1, noted: whether one of the step's own test files
  starts Stryker was not observed by QA; `task --dry check` shows they are
  Python suites. "Unchanged" `mutate` is a one-off observation, not a test.
- QA finding 11, Task 2, no code: an upper-case suffix is not hashed;
  `internal/artlib` refuses such a name.
- QA finding 12, Task 2: the `db81c37` stamp no longer matches under the
  wider hash. Task 3, by Q1.
- Relied on with no row, held by tests: a missing hashed tree refuses; a
  file without its tree's suffixes moves nothing; `check` writes nothing
  under `reports/`; the snapshot's one JSON line.

After Phase 4a the QA files changed by review findings:
- Task 1's: none.
- Task 2's: its copy of the inputs is taken from the working tree, not
  `git ls-files`, and the files it probes are derived rather than named,
  so an untracked test or a renamed golden leaves it green.

## Phase 4b

One reviewer per task, by command, with probes in scratch copies and
fixture repositories. Every finding was taken but one, a Nit accepted as
it stands: Task 2's QA suite costs 18 s inside `task check`.

| Task | Pass 1 | Pass 2 |
|---|---|---|
| 0 | 2 Must, 5 Should, 3 Nits | — |
| 1 | 3 Should, 4 Nits | 1 Should, 1 Nit |
| 2 | 1 Should, 5 Nits | 1 Nit |
| 4 | 2 Should, 1 Nit | 1 Should |

The findings that changed the work:
- Task 0's Musts: the plan said no gate moves, when `task check` stops
  producing the report, a gate change under rule 2 whose reason is the
  owner's rule; and `.github/workflows/ci.yml` named the task the plan
  removes.
- Task 0's Shoulds reworded VTT-513, whose flag would have read dirty over
  an unhashed file, and VTT-516, since `node_modules` is not tracked, both
  in place before the commit.
- Task 1: a worktree rename after `git add -N`, ` R new old`, read clean;
  a stamp whose fields were not of their kind crashed the step; and a
  400-digit `started` raised `OverflowError`.
- Task 2: QA's tests read the index, so an untracked client test turned
  the step red; and the wiring test missed a template literal, which is how
  two QA tests reach `contract/gen/tools/tools.json`.
- Task 4: `CLAUDE.md` said the judging runs at `task check` alone, and then
  read as if an out-of-date report were judged.

## The deliberate breaks

Each was made by hand, run, and undone from a saved copy checked with
`filecmp`; a stale `.pyc` was removed and Python run with `-B` each time.

| Break | What it did | Red |
|---|---|---|
| K1 | `check:ts-mutation:full` a step of `check:` | "test_task_check_starts_no_stryker_run_and_no_container" |
| K2 | `npx stryker run` a command of the step | the same |
| K3 | the stale path calls the checker | item 1's first test |
| K4 | the stale path runs `docker info` | the stand-ins' log |
| K5 | the stale path exits 1 | item 1's first test |
| K6 | no report exits 1 | item 2's test |
| K7 | the tickets from `git log`, missing an untracked one | item 1's first test |
| K8 | the commit left out of the stale lines | item 1's first test |
| K9 | the verdict dropped | item 3's test |
| K10 | the stamp reads `HEAD` again | "test_the_stamp_names_the_commit_checked_out_when_the_run_began" |
| K11 | `clean` always true | "test_the_stamp_says_whether_a_file_the_hash_reads_differed_from_the_commit" |
| K12 | no mtime rule | "test_no_report_newer_than_the_run_writes_no_stamp" |
| K13 | judged before stamped | "test_the_full_run_stamps_after_stryker_and_before_it_judges" |
| K14, K15, K16, K16b | `contract/gen/ts`, `contract/testdata`, the art, the goldens out of `TREES` | each one's own test, and "test_every_path_the_client_writes_outside_client_is_hashed" |
| K16c | `cellar.json` out of `FILES` | "test_the_tool_definitions_and_the_example_map_move_it", and the wiring test |
| K18 | `tools.json` out of `FILES` | the wiring test once it read template literals, and on a later run "test_the_tool_definitions_and_the_example_map_move_it" |
| F1 to F10 | each fix undone: `--ignored`; the SHA's case; the snapshot's kinds; the rename's column; the stamp read unchecked; NaN, an overflow, and a stamp with no `started`; the two `package.json` guards | QA's test that found it for F1 and F2; for the rest, the test each fix added |

## Rule 9: how MapTool does this

It does not. No Gradle file under `~/dev/RPTool/maptool` names a mutation
tool; the gate is this project's own tooling. Nothing to borrow and nothing
to reject.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Q2 put to the owner | Not put; `check:ts-mutation:full` | A name, with a recommendation and no consequence beyond it; the plan records that the writer took it |
| D2: `clean` from `git status` over the files the hasher reads | Ignored files included, and a rename read in either status column | QA finding 1 and Task 1's review |
| D2: the stamp holds `started` | A stamp may lack it on read | The stamp Task 3 writes for the `db81c37` report had no snapshot before it; `stamp` still requires it |
| `stamp` takes the snapshot `snapshot` printed | It refuses one whose fields are not of their kind | QA finding 3, and Task 1's review |
| D12: ci.yml's one sentence | Its header and its timeout comment as well | Both said CI holds the client's mutation testing, or quoted a desc sentence Task 1 dropped (Task 4's review) |
| Task 1 drops the step desc's "do not narrow `mutate` to make it fast" with the runtime trade | The sentence is back, in `check:ts-mutation:full`'s desc | ci.yml cites it, and it is the warning the full run needs |
| `contract/events.test.ts` untouched | Its comment names `stryker.conf.json`'s glob | It named the step's glob, which the step no longer uses |
| G5: `client/src/join.ts`'s false comment "a ticket of its own" | Raised to the owner as a candidate, not written | Which ticket is next is the owner's to choose, and fixing it reads the report out of date until a full run (D10) |

## What could not be established

- G3: a real full run has not been observed to stamp and then read
  current. Every test of it uses stand-ins; the first real observation is
  the owner's first requested run.
- G6: whether the tree was clean while the `db81c37` report was made. Its
  stamp says `clean` null, "not recorded".
- A path built from pieces in a client test is seen by neither wiring
  test. `contract/gen/tools/tools.json` sits in a template literal, which
  QA's wiring test read from the start and the implementer's only after
  Task 2's review; it is hashed by name either way.
- The method behind the ticket's 45 per cent, kept here as it asks: the
  relative imports of the 80 `.ts` files under `client/src` and
  `client/test` form a graph, with the three files outside `client/` they
  import; a source file reruns under the narrower rule when the changed file
  is it, or anything it imports or that imports it, transitively, and under
  the rule that keeps the guarantee when the changed
  file is, or lies in the forward closure of, a test file that reaches it.
  Weighted by `mutation.json`'s 3276 mutants that are not Ignored, a change
  to `client/src/view/player.ts` alone reruns 45 per cent and 100 per cent;
  the change sets of the two tickets before this one rerun 100 per cent
  under both.

## What was deliberately left out, and where it went

- A run per changed file: measured, and dropped (the ticket). No ticket.
- CI: by the owner's ruling it is not run. `.github/workflows/ci.yml`'s
  comments were corrected and its steps left as they are.
- `client/src/join.ts`'s doc comment above `joinSecretFrom` says "The
  container target is the only thing that runs it unasked" and that the
  gate skips itself on macOS, both false. An edit there moves the input
  hash and reads the report out of date until a full run, which is the
  owner's to ask for (D10). Raised to the owner with this report.
- The reminder the owner asked for, listing the tickets since the last full
  run before each push, is the session's, not the tree's.
- SPEC-023 and SPEC-020: the commit that follows this report.

## Figures

From the `task check` on `1268bee`, launched with this report, SPEC-023
and the change to SPEC-020 in the tree, uncommitted; it exited with code 0.
`go clean -cache` ran before it, on the owner's word of 2026-10-29: "ja,
rensa cachen och kör". No Go file, and nothing under `client/src`,
`internal`, `cmd` or `contract/gen`, differs from `669c4d4`.

- Go coverage: 21 packages at or above their floors; the gate notes 13 that
  clear theirs by more than 1.0pp. `internal/gateway` measured 94.1%
  against its 93.5% floor, and `internal/perceive` 95.7% against its 94.5%
  floor; on `db81c37`, whose Go files these are, they measured 94.2% and
  95.7%.
- Client coverage: 24 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors.
  - All 15 reused the verdict of their last passing run, nothing in their
    dependency closure differing.
  - The reused verdicts carry 7 timed-out mutants, counted as killed and
    not measured: 4 in `internal/sight`, 1 each in `internal/mcp`,
    `internal/perceive` and `internal/rules`.
  - `tools/mutation-equivalents.txt` is unchanged since `a8bb0ef`, on
    `main` before this ticket.
- Client mutation: not run, by the owner's rule of 2026-10-26. The log holds
  no `[check:ts-mutation:full]` line and no line naming Stryker.
  - `check:ts-mutation` printed "the stored report is current (its inputs
    are the tree's); it measured db81c37aa307, whether the tree was clean
    was not recorded. Judging it."
  - It judged that report: 3276 mutants, 3204 killed, 9 timed out and
    counted as killed, 63 survivors all adjudicated equivalent, zero
    unadjudicated. The 9 are 2 in `canvas.ts`, 6 in `scene-plan.ts` and 1
    in `wire.ts`.
  - `tools/ts-mutation-equivalents.txt` is unchanged since `65781c5`, on
    `main` before this ticket.
  - The step's six Python suites ran 70, 79, 20, 20, 20 and 60 tests, all
    passing.
- Race: both runs pass.
- Requirements chain: 516 rows, 319 test files, 17 specifications.
- Comments: 339 files under `internal`, `cmd` and `client/src`, clean, with
  no comment line added in them against `main`.
  Its two notices are the ones the gate gave on `db81c37`:
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6, and
  `internal/gateway/export_test.go` at 49.18 against 45.9.
- New prose, which reads Go and TypeScript only: 4 added lines in 1 file,
  `contract/events.test.ts`, clean.
- Disk: 34.6 GiB free just before the launch, after the cache was cleaned,
  and 33.2 GiB by the gate's own line when the Go mutation step began.
- Duration: about 1057 s, from the launch at 18:17:40 to the log's last
  write at 18:35:17 on 2026-10-29.

`task e2e`, once, against the same tree after the gate: 29 passed and 1
skipped, in 32.6 s. The skipped one is "a network drop while the table plays
on ends in a Reconnect", a `test.skip` in `client/e2e/handover.spec.ts`.
