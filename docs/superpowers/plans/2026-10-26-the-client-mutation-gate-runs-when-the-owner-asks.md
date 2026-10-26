# Plan: the client's full mutation run happens when the owner asks, and `task check` says when it last did

- **Ticket:**
  `docs/superpowers/specs/2026-10-26-the-client-mutation-gate-runs-when-the-owner-asks-design.md`.
- **Verdict:** passes, with gaps G1 to G6 (below).
  - Verified 2026-10-26 by an agent that did not write the ticket,
    read-only against HEAD `669c4d4`.
  - Probes ran in copies of `669c4d4` and `db81c37` made with `git
    archive`, under
    `~/.claude/projects/-Users-patriklager-dev-vtt-platform/scratch-t8/verify/`,
    none committed.
  - `task check`, Stryker and Docker were not run. `task --dry` was run in
    the copy only (P1).
- **To be committed as:**
  `docs/superpowers/plans/2026-10-26-the-client-mutation-gate-runs-when-the-owner-asks.md`.
- **Base:** main `669c4d4`.
- **Branch:** `fix/the-client-mutation-gate-runs-when-the-owner-asks`.
- **Owner's rulings taken in, all 2026-10-26:**
  - "ok, ny regel då, vi kör bara Full Stryker, när jag efterfrågar det. Du
    påminner mig och räknar upp alla ändringar (tickets) som gjorts sedan
    senaste Stryker körningen. Och jag avgör om jag vill att Stryker ska
    köras";
  - "Går igenom med varning", to what `task check` does when the client
    changed since the stored report: no Stryker; the step prints which
    commit the report is from and which tickets came since, and passes. The
    option also said the old report is not checked, since it describes
    another tree;
  - "vi kör inte CI på github längre";
  - "ja, ta det som nästa ärende".
- **Signed off:** 2026-10-26, "Ja, kör". The owner's answers: Q1 "Ja";
  Q3 "Ja"; Q4 "Ny SPEC-023". Q2 was not put to the owner: the writer of the
  ticket took the recommendation, `check:ts-mutation:full`. G1's sentence is
  corrected in the ticket, and its rule and item 5 follow Q3's answer.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists;
   `TREES`, `FILES` and `inputs_hash` stand in `tools/ts-mutation-inputs.py`,
   `_report_is_current` in `tools/check_ts_mutation_qa_test.py`, and the
   three tasks in `Taskfile.yml`. `mutation.json`'s top-level keys are the
   seven the ticket lists. Twelve files under `client/src` import
   `commands_pb` or `events_pb`. The Figures it cites say 3276 mutants and
   "Done in 86 minutes and 38 seconds". One sentence is false: G1.
2. **"Done" is an observation.** The ticket names no test.
   - Items 1 and 2, by reading: with no report, or a hash that differs,
     `check:ts-mutation`'s shell sets `NEED_REPORT=1` and goes to
     `task check:ts-mutation:docker` or `npx stryker run`.
   - Item 3 holds today, by command (P11).
   - Item 4, by reading: `check:ts-mutation:docker` writes no stamp, and the
     stamp holds a hash alone.
   - Item 5 fails today, by command (P2).
   - Item 6 fails today, by command: `git grep` finds "IN `task check`, AND
     NOT SCOPED", "now the gate runs it", CLAUDE.md's "both mutation gates"
     and SPEC-020's "run inside `task check`".
3. **Each rule is breakable. A reading.** Every rule has an observation that
   goes red, under "Requirements".
4. **The scope matches the claim. By command, then a reading.**
   - Callers of `ts-mutation-inputs.py`: `check:ts-mutation`,
     `_report_is_current`, `tools/ts_mutation_inputs_test.py`.
   - Callers of `check-ts-mutation.py`: `check:ts-mutation`, the container
     command in `check:ts-mutation:docker`, the two test files.
   - Callers of `check:ts-mutation:docker`: `check:ts-mutation` alone.
   - The reading: the hash misses files the client's tests read (G2), and
     present-tense sentences outside the four texts move (G5).
5. **No recorded decision is contradicted silently. A reading.** The ticket
   says the owner's rule overturns the gate that runs itself (the step's
   desc and banner). That is a gate change under CLAUDE.md rule 2: the
   owner's rule of 2026-10-26 is its reason, and SPEC-023 records it as its
   own decision (Q4). `check:mutation`'s "Runs as part of `check`" and ADR-010
   are the Go gate's, and stay. VTT-298 to VTT-304 and VTT-308 are rules of
   the checker and stay true of it whenever it runs.
6. **The records it moves. By command, then a reading.** The section is
   present and SPEC-020 resolves. The reading: the decision has no record of
   its own (G4).

## The gaps this plan inherits

- **G1. One sentence of the problem is false.** It says "`client/src/join.ts`
  imports `contract/testdata/join_url_format.json`". `join.ts` names that
  file only in a doc comment, above `joinSecretFrom`, and imports nothing.
  `client/test/dm-view.test.ts` and `client/test/join-url-format.test.ts`
  import it, so the file is an input and item 5 stands. The ticket's writer
  corrects the sentence before Task 0 commits it. Check 1 found it false, so
  the owner may treat it as a return.
- **G2. The client's tests read files outside `client/` that no import
  reaches (P3).** The rule names "imports". `fold-parity.test.ts` compares
  the fold against `scenarios/goldens`; `commands.test.ts`,
  `fold-unit.test.ts` and `removal-batch-parity.test.ts` read
  `contract/testdata`; two QA tests read `contract/gen/tools/tools.json`;
  `art-assets.test.ts` reads `campaigns/example/art` and
  `campaigns/example/maps/cellar.json`. Each decides whether a mutant is
  killed, and P2 shows a golden edited with the hash unmoved. D8 hashes them,
  under Q3.
- **G3. Item 4 cannot be observed for real without the owner's word.** It
  needs a full run. Task 1 observes it with stand-ins for Stryker and a
  report written by the test (T4). The first real observation is the
  owner's first requested run.
- **G4. The decision has no record of its own.** SPEC-020's subject is the
  adjudication key. When Stryker runs, what the stamp records and what the
  step prints when the report is out of date is a decision the step's desc
  argued the opposite of until today. SPEC-020 also moves twice, not once:
  its Status, and its sentence that the TypeScript static check runs "once
  Stryker has written its report", since the stale path runs no checker
  (D4). Q4.
- **G5. Present-tense sentences outside the four texts (the ticket's third
  unestablished point).** Searched with `git grep -n -i -E
  "stryker|ts-mutation|mutation gate|unasked|runs it"` over the tracked
  tree, less `docs/reports`, `docs/superpowers`, `docs/adr` and
  `contract-spike`, and over `client/` and `contract/` for who runs the
  gate. They become false:
  - `main()` in `tools/check-ts-mutation.py`: "run stryker first (task
    check:ts-mutation does)";
  - the docstrings of `tools/ts-mutation-inputs.py` ("re-produce only when
    they differ") and `tools/ts_mutation_inputs_test.py` ("decides whether
    62 minutes are spent");
  - `check:`'s own comment above `- task: check:ts-mutation` ("A CONTAINER
    THE TASK STARTS ITSELF"), which is not a desc or a banner, and its
    comment above `- task: check:race`, "BEFORE the mutation gates: ~53s
    against their minutes";
  - `docs/verification-debt.md`, the recipe of a test printing over 1 MiB:
    "every mutant `check:ts-mutation` runs";
  - `client/src/join.ts`'s doc comment above `joinSecretFrom`: "The
    container target is the only thing that runs it unasked". It already
    says "the gate skips itself on macOS", false since `334eae7`. D10 leaves
    it: an edit there moves the hash, and its history belongs in a report
    by rule 10. A ticket of its own.

  Still true, unchanged: `contract/events.test.ts`'s "No mutation gate
  scores this file"; `tools/ts-mutation-scope.md` and
  `tools/mutation-scope.md`, dated history; `check:fast`'s list, in which
  `check:ts-mutation` is still a step it skips. No report is revised.

  Also false once D6 removes `check:ts-mutation:docker`: the header of
  `.github/workflows/ci.yml` says the gate that cannot run on macOS "runs in
  a Linux container via `task check:ts-mutation:docker`, locally". D12
  changes that sentence alone; CI itself is out of scope by the owner's
  ruling.
- **G6. The commit of the stored report (the ticket's first unestablished
  point).** P4 to P7 establish what can be: the report measured
  `db81c37`'s client sources and the old hash's inputs, and every file the
  new hash adds is unchanged from `db81c37` to HEAD. What was not recorded is
  whether those files stood uncommitted during the run. Q1.

## Measurements this plan stands on

All on 2026-10-26 at `669c4d4`, outside the repository unless named.

| # | Probe | Result |
|---|---|---|
| P1 | `task --dry check` in a copy; `task --dry check:fast` first, to see that a dry run executes nothing | `check:fast` printed its nested `lint` and `test:unit` commands and ran none. `check`'s printed commands hold `task check:ts-mutation:docker` and `npx stryker run stryker.conf.json`, inside `check:ts-mutation`'s shell; `docker run` is not printed, being inside the docker task's own shell |
| P2 | Today's hasher on a copy, after appending to `events_pb.ts`, editing `join_url_format.json`, then `scenarios/goldens/adventure-night/state.json` | `7a6971ce…` before and after each edit |
| P3 | `grep` of `client/src` and `client/test` for paths that leave `client/` | imports: `contract/gen/ts/vtt/v1/commands_pb`, `events_pb`, `contract/testdata/join_url_format.json`. Read with `readFileSync`: the other paths of G2. No client test spawns a process |
| P4 | Each `source` in `reports/mutation/mutation.json` against `git show <commit>:<path>` | 24 files; all equal at `db81c37` and at `b1944ce`; 5 differ at `466cfbe`. `client/src` at `db81c37`, less `main.ts` and `style.css`, is those 24 |
| P5 | Today's hasher on a copy of `db81c37` | `7a6971ce…`, equal to `reports/mutation/ts-inputs.sha256` and to the working tree |
| P6 | `git diff --quiet db81c37 HEAD --` every path D8 hashes; `git status --porcelain --untracked-files=all --` the same | exit 0; no output |
| P7 | The Figures of `docs/reports/2026-10-26-a-player-speaks-as-their-character.md` | "From the `task check` on `db81c37`, launched with this report and the changes to SPEC-013, SPEC-007, SPEC-016 and SPEC-022 in the tree, uncommitted". Docs only, no input. A record, not a command |
| P8 | Tickets by tree difference: `*-design.md` in the working tree less `git ls-tree --name-only db81c37 -- docs/superpowers/specs/` | one, the ticket itself, untracked. `git merge-base --is-ancestor db81c37 HEAD` exits 0. Every file there ends `-design.md` |
| P9 | `node_modules/@stryker-mutator/core`'s `json-reporter.js`; `stryker.conf.json` | the report is written in `onMutationTestReportReady`, at the run's end. No `thresholds`, so a run with survivors exits 0 |
| P10 | `shutil.disk_usage` | 15.5 GiB free, under `MIN_FREE_BYTES`, 16 GiB, in `tools/check-mutation.py` |
| P11 | `python3 tools/<x>_test.py -q`, and `-v` for QA | 12, 70 and 79 green; `RealData`'s 8 ran, none skipped |
| P12 | `grep -li 'pitest\|stryker'` over the Gradle files of `~/dev/RPTool/maptool` | exit 1, none |
| P13 | `git status --porcelain -- bun.lockb nonexistent/dir`; `git ls-tree` of an unknown commit | exit 0, no output; exit 128 |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: this is a gate change, `task check` no longer producing the client's
    report, and rule 2 makes that its own reviewed decision with its own
    reason: the owner's rule of 2026-10-26, recorded in SPEC-023 (Q4).
    Nothing else moves: `mutate`, the thresholds, both adjudication files,
    the Go gate and `MIN_FREE_BYTES` are untouched.
  - 3: no contract change.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: no comment under `internal/`, `cmd/` or `client/src` changes. The
    plan touches none of them, so `tools/comment-ceilings.txt` does not
    move and `--write-ledger` is not run.
- **Nobody starts Stryker or its container without the owner's word**: not
  an implementer, not a test, not QA. A test that runs a command puts
  stand-ins for `docker`, `npx`, `bunx` and `stryker` first on `PATH`, and
  for `task` unless the test runs the real `task --dry`, and asserts none
  was called. The Taskfile is read with `task --dry`, which executes nothing
  for `check` and `check:ts-mutation:full` (P1); the Taskfile's one dynamic
  var, `audit:mutation`'s `sh: go env GOPATH`, is evaluated by a dry run of
  that task alone.
- **A test that runs a Taskfile step cannot be a command of that step.**
  The wiring tests run `task --dry`; nothing they start runs a step.
- **An exit code is not proof a tool ran.** Each test asserts the script's
  own line as well as its status.
- **No edit under a hashed path** (D10).
- **Never `git checkout --`** to undo a break: the inverse edit, or a saved
  copy restored and compared with `cmp`.
- **Hash the tree before and after any reviewer.**
- **Tests appended to an existing Python test file follow its style.**
- **Fixture repositories** are temporary directories, never under the
  repository, and every `git` call in them passes `-c core.hooksPath=/dev/null
  -c commit.gpgsign=false -c user.name=t -c user.email=t@t`, so a fixture
  commit runs no hook of this machine.
- **The review gate refuses untracked files.** Park scratch files outside
  the tree with `cp` and `cmp`.

## Rule 9: how MapTool does this

It does not. MapTool's build names no mutation tool (P12); the gate is this
project's own tooling. Nothing to borrow and nothing to reject.

## Design questions for the owner

1. **Q1. May the report made on `db81c37` be stamped as current under the
   new hash, naming `db81c37`?** The new hash cannot equal the old stamp.
   - Recommended: **yes**, written once in Task 3, with `clean` as `null`:
     "not recorded".
     - P4: the report's 24 sources are `db81c37`'s.
     - P5: the old stamp is `db81c37`'s old hash, so its tests, config and
       lockfile were too.
     - P6: every file the new hash adds is the same from `db81c37` to HEAD.
     - P7 says the run's uncommitted changes were docs. That is prose, so
       the stamp does not claim a clean tree.
     - The gate run then exercises the current path, and `RealData`'s 8
       tests keep running.
   - If no: the step reads the report as out of date and says it records
     no commit, until the owner asks for a run. `RealData` skips.
2. **Q2. What is the on-request task called?**
   - Recommended: **`check:ts-mutation:full`**, after "Full Stryker", beside
     the step it feeds. `check:ts-mutation:docker` goes (D6).
   - Alternatives: `stryker:full`, or keeping `check:ts-mutation:docker`
     for the container run alone, which D6 argues against.
3. **Q3. Does the hash cover what the client's tests read by path (G2)?**
   - Recommended: **yes**.
     - A golden changed by Go work changes what `fold-parity.test.ts`
       kills, and today the report would still read current.
     - Too much now costs a warning, not an hour.
     - Measured since 2026-09-26: 8 commits touched those paths and not
       `client/src`, `client/test` or `contract/gen/ts`. Each would make
       the report read out of date.
   - If no: D8 hashes `contract/gen/ts` and `join_url_format.json` alone,
     C10 says "imports", and G2 goes to `docs/verification-debt.md`.
4. **Q4. Where is the decision recorded (G4)?**
   - Recommended: **a new specification**,
     `docs/specifications/023-the-clients-full-mutation-run-happens-when-the-owner-asks.md`.
     It carries C1 to C10. SPEC-020 changes its two sentences and points at
     it. One decision, one file, and its title is where a reader looks.
   - If no: SPEC-020 gains a section, and its title no longer says what it
     holds.

## Decisions

1. **D1. The step's decision lives in a script, `tools/ts-mutation-report.py`,
   and the Taskfile holds no branch.** `check:ts-mutation`'s last command is
   `python3 tools/ts-mutation-report.py check`. The script takes `--root
   <dir>`, default the repository, and three subcommands:
   - `check`, the step;
   - `snapshot`, which prints the would-be stamp as one JSON line, or
     nothing and exit 1;
   - `stamp <snapshot>`, which writes it.

   Its functions take the root and, for `check`, the runner of the checker,
   so unit tests inject one. The command it runs is
   `<root>/tools/check-ts-mutation.py`, given
   `<root>/tools/ts-mutation-equivalents.txt`: root-relative, so a fixture
   can stand a checker in.
   - Forced by: items 1 to 3 need a test that runs the decision with no
     Stryker and no Docker. Shell in the Taskfile is reached only by running
     the step, whose commands include the tests.
   - The script's name is a preference.
2. **D2. The stamp is `reports/mutation/ts-mutation-stamp.json`, one JSON
   object:** `inputs`, the hash; `commit`, the full SHA checked out when the
   run began; `clean`, `true`, `false` or `null`. `snapshot` adds
   `started`, the run's start in epoch seconds, and `stamp` writes the
   object, `started` included, atomically: a temporary file in the same
   directory, then `os.replace`.
   - Forced by: the rule "the full run records which commit's tree it
     measured"; one file, so a hash and a commit cannot disagree; the old
     name says SHA-256 of a file that would hold more.
   - `clean` is whether no file the input hash reads differed from the
     commit, or was untracked, when the run began: `git status --porcelain
     --untracked-files=all` over the hashed paths, kept to the files the
     hasher reads. A dirty tree is
     recorded, not refused. The hash decides currency, so such a report is
     exact for the tree it measured. Refusing would forbid measuring before
     a commit, which the ticket does not ask. The flag keeps the stamp from
     claiming a commit's tree it did not measure. `null` is written by
     Task 3 alone.
   - A changed file under a hashed tree that the hasher does not read, such
     as `client/src/style.css` or `scenarios/goldens/README.md`, leaves
     `clean` true, as VTT-513 states it.
   - `reports/mutation/ts-inputs.sha256` is read by nothing afterwards.
     Task 3 parks it.
3. **D3. Currency.**
   - **Current**: `mutation.json` exists, the stamp parses, and its `inputs`
     equals the hash of the tree now.
   - **Missing**: no `mutation.json`.
   - **Stale**: anything else, an unreadable stamp and an unhashable tree
     included. An unknown never reads current.

   `_report_is_current` calls the script's `report_state` rather than
   reading the stamp itself; `QA_TS_MUTATION_REPORT` keeps its meaning.
   Forced by one reader for one format. The helper's `read().strip()` would
   read a JSON stamp as never current, and `RealData` would skip silently.
4. **D4. The stale and missing paths run no part of `check-ts-mutation.py`,
   start no process but `git`, and exit 0.**
   - Forced by the owner's option: "the old report is not checked".
     `entry_faults` reads the tree, not the report. But it fails on an
     adjudicated line edited since, and the re-key SPEC-020 asks for needs a
     survivor list from a full run. Running it would turn the path the owner
     chose to pass red over what only his run can resolve.
   - The checker's own tests and QA's still run as commands of the step.
5. **D5. "Tickets added since" is a tree difference.** The `*-design.md`
   files under `docs/superpowers/specs/` in the working tree that
   `git ls-tree --name-only <commit> -- docs/superpowers/specs/` does not
   list, sorted by name.
   - Forced by the ticket's second unestablished point. A tree difference
     needs no ancestry, so it answers for a commit on another branch as for
     an ancestor. It counts a ticket not yet committed, which is how a
     ticket stands while its own work is checked (P8).
   - When `git merge-base --is-ancestor <commit> HEAD` fails, the warning
     says the commit is not an ancestor of HEAD.
   - When no commit is recorded, or `git ls-tree` fails (P13), the warning
     says the tickets cannot be listed, and why.
   - None found prints that none was added.
6. **D6. One on-request task, `check:ts-mutation:full` (Q2), not a step of
   `check:`.** In order, as commands of one shell block under `set -e`:
   1. `SNAP="$(python3 tools/ts-mutation-report.py snapshot)"`;
   2. Stryker: in the container on Darwin 25 and later unless
      `VTT_FORCE_TS_MUTATION=1`, as `check:ts-mutation` chooses today, or
      `npx stryker run stryker.conf.json`;
   3. `python3 tools/ts-mutation-report.py stamp "$SNAP"`;
   4. `python3 tools/check-ts-mutation.py tools/ts-mutation-equivalents.txt`,
      natively.

   `check:ts-mutation:docker` goes. Its `docker run` moves into
   `check:ts-mutation:full` without the checker it ran inside the container.
   - Forced by:
     - item 4;
     - D7: the stamp sits between Stryker and the judge, so the container
       cannot judge;
     - a container task typed alone would run Stryker and record no
       commit, a second way to run it against the ticket's rule.
   - The container's `apt-get` line keeps `python3`. A preference: the
     container's setup stays as it ran.
7. **D7. The full run stamps before it judges, and stamps only a report
   newer than its snapshot.** `stamp` refuses, writes nothing and exits 1
   when `mutation.json` is absent or its mtime is not after `started`.
   - Forced by:
     - a red verdict must stay red. Unstamped, the next `task check` would
       call the new report stale and pass, hiding a measured survivor;
     - P9: Stryker writes the report at its end, so an older report is one
       this run did not write, and stamping it would call an old report
       current.
   - The snapshot is taken before Stryker starts, as `$WANT` is today, so an
     edit made during the run reads stale afterwards.
8. **D8. What the hash covers (item 5, G2, Q3).** `TREES` gains
   `contract/gen/ts` (`.ts`), `contract/testdata` (`.json`),
   `scenarios/goldens` (`.json`) and `campaigns/example/art` (`.png`,
   `.json`). `FILES` gains `contract/gen/tools/tools.json` and
   `campaigns/example/maps/cellar.json`. Every tree must exist, as today.
   - Forced by item 5, G2, the hasher's own "deliberately generous", and a
     cost that is now a warning.
   - Directories, because `commands.test.ts` reads `contract/testdata` by a
     name passed in.
   - The art's two suffixes are `internal/artlib`'s. All files would hash
     a Finder `.DS_Store`, which is gitignored and comes and goes.
   - A wiring test holds the next read: every path literal in `client/src`
     and `client/test` that leaves `client/` resolves under a hashed tree or
     to a hashed file. It misses a path built from pieces. Accepted.
   - The ticket's own changes touch no hashed path, so the input hash stays
     put through it.
9. **D9. The step stops starting Stryker (Task 1) before the hash moves
   (Task 2).** Forced by the owner's rule. A commit where the hash had moved
   and the step still started Stryker would start it at the next
   `task check`.
10. **D10. No edit under a hashed path.** Forced by D8: an edit there moves
    the hash, makes the stored report stale and leaves the gate's current
    path unexercised. `client/src/join.ts`'s comment stays (G5).
11. **D11. What the step prints.** The tests assert these by substring:
    - stale: `THE CLIENT'S MUTATION WAS NOT MEASURED`, why (inputs changed,
      stamp unreadable or absent, tree unhashable), the commit as `git
      rev-parse --short=12` prints it, how clean it was, the tickets, and
      that `task check:ts-mutation:full` measures it when the owner asks;
    - missing: the same first words, and that there is no stored report;
    - current: that the report is current, and its commit.

    No duration in a message, since it would rot. The wording is a
    preference.
12. **D12. The sentences that change (item 6, G5).**
    - `Taskfile.yml`:
      - `check:ts-mutation`'s desc keeps zero unadjudicated survivors, what
        coverage cannot see, the command runner, and the unscoped `mutate`
        glob. It loses "IN `task check`", the runtime trade, CI, the
        2026-08-25 correction and the skip banner's sentence, and says the
        step judges a current report and otherwise reports it was not
        measured;
      - its shell block, banners and comments go with D1;
      - `check:ts-mutation:full`'s desc takes the kernel-bug paragraphs,
        Node and Bun, the anonymous `node_modules` volume and the Docker
        daemon from the two old descs, and says it runs when the owner
        asks. Their CI sentences go;
      - `check:`'s two comments from G5, above the step and above
        `- task: check:race`.
    - `CLAUDE.md`, "The gate": "and both mutation gates" becomes "the Go
      mutation gate and the judging of the client's stored mutation
      report". One sentence follows: the client's full run is `task
      check:ts-mutation:full`, run only when the owner asks, and `task
      check` names the commit the stored report measured and the tickets
      added since.
    - SPEC-020's Status: `check:ts-mutation` runs the TypeScript gate inside
      `task check` only over a current report, and `check:ts-mutation:full`
      after each full run. Its static-check sentence gains that the
      TypeScript gate does not run while the report is out of date. Both
      point at Q4's record.
    - `main()` in `check-ts-mutation.py`: `task check:ts-mutation:full`.
    - Both hasher docstrings: what the hash decides now. And four comments
      in `tools/ts_mutation_inputs_test.py` that say a stale hash produces
      the report: in `test_the_same_tree_hashes_the_same` ("would re-mutate
      every run"), `test_a_file_that_is_not_an_input_does_NOT_move_it`
      ("re-mutating … 62 minutes"),
      `test_a_missing_mutated_tree_is_not_UNCHANGED` ("Fail towards
      producing") and
      `test_main_refuses_rather_than_printing_nothing` ("'produce the
      report'").
    - The header of `.github/workflows/ci.yml`: the container run is `task
      check:ts-mutation:full`, when the owner asks.
    - `docs/verification-debt.md`'s recipe: "every mutant
      `check:ts-mutation:full` runs".

## Requirements

Candidates for the sort. Task 0 allocates them after sign-off, with
`requirement-id`, each row `**OPEN — no test yet**`. Each names the
observation that goes red. Allocated 2026-10-26, all ten, Q3 being "Ja":
C1 is VTT-507, C2 VTT-508, C3 VTT-509, C4 VTT-510, C5 VTT-511, C6 VTT-512,
C7 VTT-513, C8 VTT-514, C9 VTT-515 and C10 VTT-516. C7 and C10 were
reworded in place by Task 0's review, before its commit.

- **C1.** "`task check` starts no Stryker run and no container." W1, R1.
  Red under K1, K2, K4.
- **C2.** "With the stored client mutation report out of date,
  `check:ts-mutation` exits 0 and does not judge the report." S1, S2, S8.
  Red under K3, K5.
- **C3.** "With the stored client mutation report out of date,
  `check:ts-mutation` prints that the client's mutation was not measured,
  the commit the report measured, and each ticket under
  `docs/superpowers/specs/` that the commit's tree lacks." S1 to S5. Red
  under K7, K8.
- **C4.** "With no stored client mutation report, `check:ts-mutation` exits
  0 and prints that there is none." S6. Red under K6.
- **C5.** "With the stored client mutation report current,
  `check:ts-mutation` exits with the verdict of `tools/check-ts-mutation.py`
  over it." S7. Red under K9. Existing behaviour, checked against the code:
  today the shell's last command runs the checker on every path.
- **C6.** "A full run's stamp names the commit checked out when the run
  began." T1. Red under K10.
- **C7.** "A full run's stamp says whether any file the report's input hash
  reads differed from that commit, or was untracked, when the run began."
  T2. Red under K11.
- **C8.** "A full run that leaves no report newer than its start writes no
  stamp." T3. Red under K12.
- **C9.** "A full run's report is current for `check:ts-mutation` whatever
  the full run's verdict." W2, T4. Red under K13.
- **C10.** "A change to a tracked file that a client source or test imports, or
  that a client test reads by a path written in it, moves the report's
  input hash." H1 to H7. Red under K14 to K16. Under Q3's "no", it ends at
  "imports".

The sort, and what it rejected:
- "The full run mutates the `mutate` list unchanged": nothing changes, and
  `test_the_hashed_trees_match_what_stryker_actually_mutates` already pins
  the hashed trees against it. No id.
- Item 6, no sentence says `task check` runs Stryker: a done observation,
  held by Phase 4b and Task 4's grep, not a rule of the system. No id.
- An unhashable tree never reads current: a case of "out of date" (D3),
  under C2 by S8.
- The stamp's JSON keys, `_report_is_current`'s delegation, the sort order
  of tickets, the not-an-ancestor note and the message's wording: how, or
  scenery.

Kept, wording still true: VTT-298 to VTT-304 and VTT-308, rules of the
checker. No row is withdrawn.

| Ticket item | Observed by |
|---|---|
| 1 | S1, R1 |
| 2 | S6, R1 |
| 3 | S7, and `check_ts_mutation_test.py` |
| 4 | W2, T1, T4 (G3) |
| 5 | H1, H2 |
| 6 | Task 4's grep, and Phase 4b |

## The existing tests that change

- `tools/ts_mutation_inputs_test.py`: `tree()` gains a file in each new tree,
  since every tree must exist (D8). Its docstring and the four comments D12
  names change. No test changes its assertion.
- `tools/check_ts_mutation_qa_test.py`: `_report_is_current` (D3).
- No test reads `main()`'s message in `check-ts-mutation.py`.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off, with answers to Q1 to Q4.
  - The ticket's writer corrects G1's sentence.
  - Allocate C1 to C10, less those an answer removes, with
    `requirement-id`.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean, the rows exist, and `python3
  tools/check-requirements-chain.py` passes.

### Task 1: the step and the full run (items 1 to 4; D1 to D7, D11)

- **Files:**
  - new `tools/ts-mutation-report.py` and `tools/ts_mutation_report_test.py`;
  - `tools/check_ts_mutation_qa_test.py`: `_report_is_current`;
  - `Taskfile.yml`: `check:ts-mutation`'s commands, the new test file among
    them, and its desc; `check:ts-mutation:full`, new;
    `check:ts-mutation:docker`, removed; `check:`'s two comments (D12);
  - `tools/check-ts-mutation.py`: `main()`'s message.
- **New tests**, each on a fixture repository committed at a commit A, with
  the trees D8 hashes, a `mutation.json` and a stamp naming A:
  - **S1** (item 1): edit `client/src`; add one ticket committed at B and
    one untracked. `check` exits 0 and prints the stale line, A's short SHA
    and both tickets, not a ticket A's tree holds. The runner is not called.
  - **S2:** a stamp with no `commit`, then one that is not JSON: says the
    tickets cannot be listed; exit 0.
  - **S3:** a stamp naming a SHA the fixture does not hold: the same.
  - **S4:** a stamp naming a commit on another branch: says it is not an
    ancestor, and lists by tree difference.
  - **S5:** `clean` `false` and `null` each print their note.
  - **S6** (item 2): no `mutation.json`: exit 0, says there is no stored
    report; the runner is not called.
  - **S7** (item 3): a current stamp: the runner is called with the root's
    checker and equivalents file, and its 0 and its 1 each come back as the
    exit status; the commit is printed.
  - **S8:** a report present and `client/src` missing: stale, exit 0, the
    runner not called.
  - **R1** (items 1, 2): the CLI as a subprocess on S1's and S6's fixtures,
    with the stand-ins of the constraints first on `PATH`, each appending
    its name to a log: the log is empty, the script's line is printed, and
    the status is 0.
  - **T1:** `snapshot`, then a commit, then a newer report and `stamp`: the
    stamp names the commit of the snapshot.
  - **T2:** `snapshot` on a clean fixture says `clean` true; with an
    untracked `.ts` file under `client/src`, false; with only
    `client/src/style.css` changed, true.
  - **T3:** `stamp` with no report, then with a report whose mtime is not
    after `started`: exit 1, and the stamp file is unchanged, compared by
    content. With a newer report it writes.
  - **T4** (item 4): `snapshot`, a newer report, `stamp`, then `check` with
    a stand-in checker at the root's path that exits 1: `check` exits 1 and
    names the commit.
  - **T5:** `snapshot` on an unhashable tree, and outside a git repository:
    exit 1, nothing on stdout.
  - **W1** (C1): `task --dry check` in the repository: none of `stryker
    run`, `docker run`, `check:ts-mutation:docker`, `check:ts-mutation:full`;
    a `[check:ts-mutation]` line with `ts-mutation-report.py check`. With no
    `task` on `PATH` it fails, not skips.
  - **W2** (C9): `task --dry check:ts-mutation:full`: `snapshot` before
    `stryker run` and `docker run`, these before `stamp`, and `stamp`
    before `check-ts-mutation.py`.
- **Order:**
  1. Write the tests and run them on `669c4d4`: the script is absent, and
     W1 is red on P1's two lines.
  2. Write the script, then the Taskfile and the QA helper.
  3. Make the breaks.
  4. Run the four Python suites, with `-v` for QA.
- **True when:**
  - all green;
  - `python3 tools/ts-mutation-report.py check` in the repository prints
    the stale line, says the report records no commit, and exits 0, since
    the new stamp does not exist yet;
  - `task --dry check` prints none of W1's four strings.
- **Breaks**, each undone by its inverse edit:
  - **K1:** `- task: check:ts-mutation:full` added to `check:`. W1 red.
  - **K2:** `npx stryker run stryker.conf.json` added to
    `check:ts-mutation`'s commands. W1 red.
  - **K3:** the stale path calls the runner. S1 red.
  - **K4:** the stale path runs `docker info`. R1 red.
  - **K5:** the stale path returns 1. S1 red.
  - **K6:** the missing path returns 1. S6 red.
  - **K7:** the tickets taken from `git log <commit>..HEAD`, which misses
    the untracked one. S1 red.
  - **K8:** the commit left out of the stale line. S1 red.
  - **K9:** the current path returns 0 whatever the runner said. S7 red.
  - **K10:** `stamp` reads `HEAD` again instead of the snapshot's commit.
    T1 red.
  - **K11:** `clean` always true. T2 red.
  - **K12:** the mtime check removed. T3 red.
  - **K13:** in `check:ts-mutation:full`, the checker moved above `stamp`.
    W2 red.

### Task 2: the hash (item 5; D8)

- **Files:** `tools/ts-mutation-inputs.py` (`TREES`, `FILES`, docstring);
  `tools/ts_mutation_inputs_test.py`.
- **New tests**, appended in the file's style:
  - **H1** (item 5): an edit to `contract/gen/ts/vtt/v1/events_pb.ts` moves
    the hash;
  - **H2** (item 5): an edit to `contract/testdata/join_url_format.json`
    moves it;
  - **H3:** `commands_pb.ts`; **H4:** a golden; **H5:** an art PNG;
    **H6:** `tools.json` and `cellar.json`. Each moves it. Under Q3's "no",
    H4 to H6 go;
  - **H7:** the wiring test of D8, over the real `client/src` and
    `client/test`;
  - **H8:** a fixture lacking `contract/gen/ts` hashes to `None`.
- **Order:** tests first, each red on Task 1's commit; then the change; then
  the breaks.
- **True when:**
  - the suite is green;
  - P2's probe, repeated on a copy, prints a new hash after each edit;
  - `python3 tools/ts-mutation-report.py check` still exits 0.
- **Breaks:**
  - **K14:** `contract/gen/ts` out of `TREES`. H1, H7 red.
  - **K15:** `contract/testdata` out. H2, H7 red.
  - **K16:** `campaigns/example/art` out. H5, H7 red.

### Task 3: the stored report, locally (Q1)

Nothing in Task 3 is committed.
- **Do, under Q1's "yes":**
  - Re-run P4 to P6 on Task 2's commit. P6 runs over Task 2's paths. Stop if
    any differs.
  - Write `{"inputs": <inputs_hash of the repository>, "commit":
    "db81c37aa307c3583f6eba7cf18764eaecdd57ca", "clean": null}` through the
    script's writer, called with `python3 -c`, never typed by hand.
  - Copy `reports/mutation/ts-inputs.sha256` to
    `~/.claude/projects/-Users-patriklager-dev-vtt-platform/scratch-t8/`,
    `cmp` it, then remove it. Its hash goes into the report.
- **Under "no":** only the parking.
- **True when:**
  - under "yes": `check` prints current and `db81c37`, the checker prints
    zero unadjudicated survivors, and the QA suite's `-v` shows no test
    skipped;
  - under "no": `check` prints the stale line and no commit.

### Task 4: the other texts (item 6; D12)

- **Files:** `CLAUDE.md`, `docs/verification-debt.md`.
- **True when:** `git grep -n -E "both mutation gates|IN .task check.|now
  the gate runs it"` over `CLAUDE.md` and `Taskfile.yml` prints nothing, and
  Phase 4b has read each changed sentence against the code.

### Task 5: the report

- **Do:** write
  `docs/reports/2026-10-26-the-client-mutation-gate-runs-when-the-owner-asks.md`
  and give it its reading review. It covers:
  - each "Done looks like" item with its observation;
  - K1 to K16, with what went red;
  - Rule 9;
  - G1 to G6, and Q1 to Q4's answers;
  - Task 3's evidence and the old stamp's hash;
  - Phase 4a's adjudications, under their own heading.

  Leave room for the gate's figures.
- **True when:** it answers the ticket's six items in its numbering, each
  `[x]` naming its observation.

### Task 6: the specifications

With the `specification` skill, from the report:
- Q4's record, new: when the full run happens and who asks; what the stamp
  holds; currency; the three paths and what each prints; the tickets by tree
  difference; what the hash covers; C1 to C10 in its Requirements.
- SPEC-020: its Status and its static-check sentence (D12).

**True when:** each new sentence has been run against the code by someone
other than its writer, and `git grep -n "run inside .task check."
docs/specifications` prints nothing.

### Task 7: the gate, whole, and the commits

- **Do:**
  - Check free space first. P10 put it under the Go gate's floor, which
    refuses the run. Tell the owner; the floor is not lowered (rule 2), and
    `go clean -cache` is machine-wide, so it is his call.
  - Run `task check` whole on the last code commit. Launch it with
    `start_new_session=True`, send all output to a file, and require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Write the figures into the report. Client mutation: not run, by the
    owner's rule of 2026-10-26. The stored report is from `db81c37`, and
    the figures say what the step did with it.
  - Commit the report, then the specifications.
- **True when:**
  - the gate is green, `check:mutation` included;
  - its log holds no `[check:ts-mutation:full]` line and none of Stryker's
    own lines;
  - under Q1's "yes", `check:ts-mutation` judged the report from `db81c37`
    with zero unadjudicated survivors; under "no", it printed the stale
    line;
  - `check:requirements-chain` is green.
- Push and merge on the owner's word.

## For Phase 4a

Hand the QA agent:
- the allocated rules' texts;
- Q4's record, or SPEC-020's new section;
- the surfaces:
  - `python3 tools/ts-mutation-report.py [--root DIR] check | snapshot |
    stamp <snapshot>`;
  - the stamp's path, keys and value types; `reports/mutation/mutation.json`;
  - `python3 tools/ts-mutation-inputs.py [ROOT]`, and the hashed paths as
    the record states them;
  - that `check` on a current report runs `<root>/tools/check-ts-mutation.py
    <root>/tools/ts-mutation-equivalents.txt`, so a fixture may stand a
    checker there;
  - `task --dry check` and `task --dry check:ts-mutation:full`.

Tell it:
- never run `task check`, `task check:ts-mutation:full`, Stryker or
  Docker. Any command it runs has the stand-ins for `docker`, `npx`,
  `bunx` and `stryker` first on `PATH`, and for `task` unless it runs the
  real `task --dry`, and it checks they were not called;
- fixtures are temporary git repositories, with the constraints' `-c`
  flags;
- never write the repository's `reports/mutation/`. `stamp` and `check` run
  with `--root` on a fixture;
- an exit code is not proof: assert the script's own line;
- its tests go in `tools/ts_mutation_report_qa_test.py`, added to
  `check:ts-mutation`'s commands. The tree is hashed before and after it.
