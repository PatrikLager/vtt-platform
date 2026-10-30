# The DM console's eight lock-key mutants are adjudicated equivalent: the change

**Ticket:** `docs/superpowers/specs/2026-10-30-the-console-lock-keys-are-equivalent-design.md`.
An agent that did not write it verified it and passed it with gaps G1 to
G7.
**Plan:** `docs/superpowers/plans/2026-10-30-the-console-lock-keys-are-equivalent.md`,
written by that verifier from probes in a copy of the tree outside the
repository, and amended by Task 0's review.
**Where it came from:** the owner's "Kör full mutation" of 2026-10-30, the
client's full mutation run, `task check:ts-mutation:full`, on `main` at
`526885b`. It ended red: eight survivors that no entry in
`tools/ts-mutation-equivalents.txt` claimed.
**The owner's rulings, both 2026-10-30:**
- Asked whether to adjudicate the eight as equivalent or to have a helper
  make a button's `data-action` and its lock key one string, so that a
  test could observe the key: "Bedöm som ekvivalenta". Add from JSON's
  button carries no `data-action`, so the block in the file words the
  alternative as refactoring the keys so that a test could observe them.
- The plan signed off: "Ja, kör".

**No code changed.** Besides its records, the ticket changed the
adjudication file alone, in `17febe7`, whose parent `2ac1a4b` holds the
ticket and plan on `526885b`. Every code reference below is to the tree of
`526885b`, which neither commit changes under `client/`.

## The period, in commits

| Commit | What it holds |
|---|---|
| `2ac1a4b` | Task 0: the ticket and the plan |
| `17febe7` | Task 1: the eight entries and the block above them in `tools/ts-mutation-equivalents.txt` |
| this report's | Tasks 2 to 4: this report, its Figures, and the result of Task 3 |

## The run that found them

The owner's run on `526885b` went through the Docker container that
`check:ts-mutation:full` uses on macOS 26. The task started at 21:09:19 on
2026-10-30 and ended at 22:24:00. Stryker found 24 files to mutate,
instrumented 3337 mutants, ran them on four test runners, and reported "Done
in 73 minutes and 8 seconds". The report it wrote,
`reports/mutation/mutation.json`, holds 3251 killed, 71 survived, 9 timed out
and 6 ignored. The six are what the three `// Stryker disable next-line`
comments in `client/src` exclude, in `view/dm.ts`, `view/player.ts` and
`view/spectator.ts`, two mutants each. The checker counts the other 3331.

The snapshot taken before Stryker started became the stamp,
`reports/mutation/ts-mutation-stamp.json`: commit `526885b`, a clean tree,
and the input hash `1de3fc01949080af…`. Then `tools/check-ts-mutation.py`
judged the report. 63 of the 71 survivors had entries. Eight had none, and
the task failed.

Both files are gitignored. They are on the machine that ran the gate, and a
fresh clone has neither.

## What the code did on `526885b`

`sending(d, key, press)` in `client/src/view/dm.ts` wraps a console
control's press. It returns at once when `unanswered` holds `key`.
Otherwise it calls `press`, adds `key` to `unanswered` when `press` gave a
command, and deletes it when the send settles, answered or rejected.
`unanswered` is a `Set` declared in the module and exported by nothing.
`sending` asks it `has`, `add` and `delete` and nothing else. It never
iterates it or reads its size.

Ten calls pass `sending` a key. Eight pass a string literal: the console's
Say (`add-narration`), Start session (`start-session`), Add actor
(`add-actor`), Remove actor (`remove-actor`), Add from JSON (`actor-json`),
Place token (`place-token`), Remove token (`remove-token`) and Save note
(`upsert-note`). The other two build theirs with a template literal,
`` `describe-token:${id}` `` and `` `grant-actor-control:${a.actorId}` ``.

Stryker's `StringLiteral` mutator replaces each of the eight literals with
`""`, one per mutant. `""` is a key no other call passes. The seven other
literals are non-empty and differ from each other, and both template keys
open with a non-empty prefix. So under each mutant the control still holds
back itself, as VTT-523 requires of the seven forms and VTT-535 of the
Say, and no other control, as VTT-541 requires. The tests that hold the
three rows pass. The two template keys' mutants, an empty template
literal each, were killed.

## Done looks like, answered

1. [x] `python3 tools/ts-mutation-report.py check`, on `17febe7` against
   the stored report that measured `526885b`, printed "the stored report is
   current (its inputs are the tree's); it measured 526885bc0c77, from a
   clean tree" and "3331 mutants, 3251 killed, 9 timed out, 71 survivors
   all adjudicated equivalent, zero unadjudicated survivors.", no
   `SURVIVED` line, and exited 0. On `526885b` it printed eight `SURVIVED
   StringLiteral` lines and exited 1.
2. [x] Each of the eight entries removed in turn from a copy of
   `17febe7`'s file: `python3 tools/check-ts-mutation.py <copy>` exited 1,
   printed one `SURVIVED`, whose `code:` line was that entry's, and no
   "no longer survives" line. The file itself exits 0. This was Task 1's
   deliberate break, eight of eight. It was first run on the file as Task 1
   wrote it, before Task 1's review rewrote the block and the reasons, and
   run again on `17febe7`'s file with the same result. The checker keys an
   entry on its header and `code:` line, which the review left as they
   were.
3. [x] In a copy of the tree outside the repository, with `node_modules`
   linked from it, each key replaced by `""` in turn, the edit asserted to
   have landed once:

   | Key | `env -u CLAUDECODE bun test client/test` |
   |---|---|
   | none, the original | 1773 pass, 0 fail |
   | `add-narration` | 1773 pass, 0 fail |
   | `start-session` | 1773 pass, 0 fail |
   | `add-actor` | 1773 pass, 0 fail |
   | `remove-actor` | 1773 pass, 0 fail |
   | `actor-json` | 1773 pass, 0 fail |
   | `place-token` | 1773 pass, 0 fail |
   | `remove-token` | 1773 pass, 0 fail |
   | `upsert-note` | 1773 pass, 0 fail |

   `dm.ts` was restored after each run and compared equal to the
   repository's. The verifier, Task 0's reviewer, Task 1's reviewer and
   this report's reviewer each ran the same nine in copies of their own,
   with the same result.
4. [x] `task check` on `17febe7` exited 0. Its `check:ts-mutation` step
   printed that the stored report is current and measured `526885bc0c77`
   from a clean tree, judged it, and ended "zero unadjudicated survivors".
   It did not print THE CLIENT'S MUTATION WAS NOT MEASURED. The figures are
   below.
5. [x] `git diff main --numstat -- tools/ts-mutation-equivalents.txt` on
   `17febe7`: 57 added, 0 deleted. Eight added lines begin
   `client/src/view/dm.ts  StringLiteral`.

## What the rules became

No rule changed and no row was allocated. An equivalence entry is an
adjudication, kept where `CLAUDE.md`'s record says adjudications go. The
rows the entries cite, VTT-523, VTT-535 and VTT-541, are cited as they
stand.

## The entries

The eight entries sit in the file's `client/src/view/dm.ts` section, in the
order of their code. Their headers and `code:` lines are the ones the
checker printed. Each reason names its control and says why `""` changes
nothing. The `add-narration` reason adds that Send and Enter in `say-text`
both call the one `say`, so they stay one control.

A comment block at the head of the eight holds what they share. Each
reason restates the premise too, so that a grep for `unanswered` lands on
all eight. The block holds:
- the mutation;
- the premise in `sending` and `unanswered`;
- the measurements and where the plan keeps them, its P6 and P7;
- the distinguishers;
- the owner's ruling;
- an instruction that whoever edits `sending` or `unanswered` re-judges all
  eight by hand.

The section's paragraph above it opens "166 survivors -> 0 here". The block
begins "The 0 above held until the full run of 2026-10-30 on 526885b found
the eight entries below", so that opening reads as history and no line was
deleted.

**The distinguishers.** None of the console's behaviour tells a mutant from
the original. The verifier traced 13 controls as they were pressed, held,
pressed again on the old console and a fresh one, refused, and accepted.
Each step recorded a fresh paint's markup, every box's value, notices,
repaints and the commands sent, their random `requestId` masked. The
345,611-byte trace was the same under all eight mutants and under the
original and two re-runs of it. Task 1's reviewer built a trace of its
own, 450,148 bytes, with the same result. Under a key changed to collide with another,
`start-session` made `add-actor`, the reviewer's trace differed and three
tests failed, "a different control sends while Start session is
unanswered" among them. So the tests and the trace see a collision; `""`
never makes one.

Reflection does tell them apart, in two ways:
- A spy on `Set.prototype`'s `add`, `has` or `delete` sees `""` where the
  literal was.
- The module's source text, read through `renderDMConsole.toString()` or
  from `dm.ts` itself, holds the literal once fewer.

Neither is an input to the console. No test is written for either. A spy
on a builtin is rejected for the reason the `client/src/view/scene-plan.ts`
entry on `scene.Objects ?? []` gives its `String.prototype` distinguisher.
A test that reads the source text would pin a private key's spelling rather
than any behaviour. The block records both.

## Phase 4a

Not run, by the plan's D7. The ticket puts no rule and moves no
specification, so no sentence describes behaviour it delivers. An agent
that may not read the source cannot test a claim about the source. The
cycle's skip list names no adjudication file, so this was a judgment, and
the plan says so. What executes against the claim is Done item 3. The
implementer ran it, and so did the verifier, Task 0's reviewer, Task 1's
reviewer and this report's reviewer.

### QA adjudications

None: no QA ran.

## Phase 4b

One reviewer for each commit, by command, and every finding taken.

**Task 0, the ticket and the plan.** No Must, 3 Should, 10 Nits.
- Should:
  - "holds back itself" is VTT-523's and VTT-535's, not VTT-541's;
  - the plan named a machine-local scratch path;
  - Task 2 marked item 4 done before the gate that shows it had run.
- Nits:
  - a load figure the run's log does not hold;
  - "today" where `526885b` was meant;
  - G2 and D3 disagreed about a sentence or a paragraph;
  - a `--numstat` that cannot count headers;
  - the grep for a Stryker line not named;
  - `ln -sfn` for a recipe re-run into an existing copy;
  - D2 restating the `scene-plan.ts` entry rather than pointing at it;
  - D7's skip called forced when it is a judgment;
  - G4 silent on a template key that could come out empty;
  - a 91-column line.

**Task 1, the entries.** No Must, 2 Should, 4 Nits.
- Should:
  - The block said "AN EDIT TO `sending` OR `unanswered` RE-JUDGES ALL
    EIGHT", which reads as a fact and is false: SPEC-020 says a reason whose
    premise lies outside the keyed line is not judged again. The reviewer
    showed it. With `if (unanswered.size > 99) return;` added to `sending`
    in a copy, the checker still passed the file as Task 1 wrote it, and
    this report's reviewer found the same with `17febe7`'s. That edit also
    moved the input hash, so `task check` would have called the report out
    of date and judged nothing. The sentence is now an instruction, "WHOEVER
    EDITS `sending` OR `unanswered` RE-JUDGES ALL EIGHT BY HAND", and ends
    "no gate asks (SPEC-020)".
  - "TWO DISTINGUISHERS" undercounted. A spy on `has` or `delete` works as
    well as one on `add`, and the source text can be read from the file as
    well as through `toString()`. The block now says "TWO KINDS OF
    DISTINGUISHER".
- Nits:
  - the run named, rather than "the same run";
  - the trace's `requestId` said to be masked;
  - the `scene-plan.ts` precedent said to fit the spy only;
  - the owner's alternative reworded, since Add from JSON's button carries
    no `data-action`.

## The deliberate breaks

| Check | Broken by | What spoke |
|---|---|---|
| Each of the eight entries | the entry removed from a copy of the file | `tools/check-ts-mutation.py`: exit 1, one `SURVIVED` on that entry's code, no stale line |

The other direction is the committed file, green. The checker reads the
repository's tree whatever directory the file it is given sits in, which is
why a copy breaks it the way an edit to the tracked file would.

## Rule 9: how MapTool does this

It does not. Its build at `f4b7fef6c` names no mutation tool, so there is
nothing to borrow and nothing to reject.

## The plan's gaps

- **G1.** The step's own tests, `tools/check_ts_mutation_test.py` and
  `tools/check_ts_mutation_qa_test.py`, read the real file. Six of
  `RealData`'s eight failed on `526885b`. On `17febe7` all eight run and
  pass, and the two files pass whole. Nothing in them changed.
- **G2.** Taken as above: the dm.ts section's opening reads as history.
- **G3.** Taken as above: the distinguishers are in the file. The owner's
  choice named equivalence and the refactor. A test that spies on a builtin
  was not among them, and it is rejected by precedent.
- **G4.** Every reason names `sending` and `unanswered`, so `git grep -n
  unanswered` lands on all eight, and the block tells whoever edits either
  to re-judge them. A new call whose template key could come out empty is
  not caught that way. That case is near-theoretical.
- **G5.** SPEC-022's "One press, one command" names the keys' values. Read
  in Task 3 below, it stays true.
- **G6.** `tools/ts-mutation-scope.md`'s "adjudicated equivalent" column
  disagreed with the file in six rows before this ticket, `dm.ts` among
  them at 1 against 4. Its `dm.ts` row still says 1, against 12 entries
  now. Left for the owner as a
  candidate ticket.
- **G7.** This report quotes both counts and says which is which: 3337
  instrumented, 3331 counted by the checker.

## Specifications

None moved. `git grep -n -e unanswered -e add-narration -e actor-json --
docs/specifications` finds SPEC-022 alone. Its sentences on the locks say
that the console keeps its locks in `unanswered`, one per control, keyed by
the control's action, by the action and a row's id for a Describe or a
Grant, and by `actor-json` for Add from JSON. They say the Say's Send and
Enter share one lock keyed `add-narration`, and that `draft` and
`unanswered` last as long as the page. Each is still true of `dm.ts`, which
this ticket did not touch. That no test observes a key's spelling is now
written in the adjudication file, and SPEC-022 does not say otherwise.
SPEC-020 describes the form the entries take, not the entries.

## Deviations

- The plan's D2 speaks of "the two distinguishers of G3". Task 1's review
  found more routes, and the block says "two kinds".
- The plan's D3 has the section's paragraph gain "a sentence". The new
  text is the opening sentence of a block of its own, set after the
  paragraph rather than inside it.

## What could not be established

- Whether the eight survive on another machine. The stored report is
  local, and a fresh clone's `check:ts-mutation` prints THE CLIENT'S
  MUTATION WAS NOT MEASURED and passes. Item 3's hand mutations are the
  part another machine can repeat.
- The nine timed-out mutants: two in `client/src/view/canvas.ts`, six in
  `client/src/view/scene-plan.ts` and one in `client/src/wire.ts`. The
  checker counts them as killed and says its count measures the machine.
  The one-minute load average, read by hand twice during the run, was 7.6
  and 7.0; the log holds no load. They are outside this ticket.

## What was deliberately left out, and where it went

- The refactor the owner declined, which would make a button's
  `data-action` and its lock key one string. The block, the plan and this
  report record the ruling; no ticket or debt entry carries the refactor.
- G6's column in `tools/ts-mutation-scope.md`. It is a candidate ticket.

## Figures

From the `task check` on `17febe7`, the commit of the eight entries,
launched with this report written outside the tree; it exited with code 0.
No file under `client/` and no Go file differs from `526885b`.

- Go coverage: 21 packages at or above their floors; the gate notes 13 that
  clear theirs by more than 1.0pp.
- Client coverage: 24 files gated, each at 100.00%; 1 excluded.
- Go mutation: 15 packages, zero unadjudicated survivors. All 15 reused the
  verdict of their last passing run, nothing in their dependency closure
  differing. The reused verdicts carry 7 timed-out mutants, counted as
  killed and not measured: 4 in `internal/sight`, 1 each in `internal/mcp`,
  `internal/perceive` and `internal/rules`.
- Client mutation: judged, not run. `check:ts-mutation` printed "the stored
  report is current (its inputs are the tree's); it measured 526885bc0c77,
  from a clean tree. Judging it." and "3331 mutants, 3251 killed, 9 timed
  out, 71 survivors all adjudicated equivalent, zero unadjudicated
  survivors." The log holds no `task: [check:ts-mutation:full]` line and no
  line from Stryker's own logger:
  `grep -c 'INFO Instrumenter\|MutationTestExecutor'` prints 0. The
  step's own tests ran before the judgment: `check_ts_mutation_test.py`'s
  70, `check_ts_mutation_qa_test.py`'s 79, and the input and report tools'
  four test files, each OK.
- Race: both runs pass.
- Requirements chain: 541 rows, 323 test files, 17 specifications.
- Comments: 339 files, 0 comment lines added, clean. Its two notices are
  `cmd/vtt/events_tail_truncation_test.go` at 22.83 against 22.6, and
  `internal/gateway/export_test.go` at 49.18 against 45.9, the two the gate
  gave on `08917d7` in
  `docs/reports/2026-10-30-the-dm-speaks-from-the-console.md`.
- New prose, which reads Go and TypeScript only: nothing added against
  `main`.
- The tree: hashed before the launch and after the gate, the same. The
  stamp's hash and the input hash did not move.
- Disk: 34 GiB free just before the launch, and 34.2 GiB by the gate's own
  line when the Go mutation step began.
- Duration: about 965 s, from the launch at 00:15:47 to the log's last
  write at 00:31:52 on 2026-10-31.
