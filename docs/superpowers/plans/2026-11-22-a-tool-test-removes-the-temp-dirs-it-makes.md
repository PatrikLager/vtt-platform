# Plan: a tool test removes every temporary file and directory it makes

- **Ticket:** `docs/superpowers/specs/2026-11-22-a-tool-test-removes-the-temp-dirs-it-makes-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-22 by an agent that did not write the ticket, against
    `1f17c17` on `fix/the-chain-qa-test-removes-its-temp-trees` in the
    worktree `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e` and Stryker were not run. The measurement on the base (M1) ran
    in the worktree, each test with its own `TMPDIR` outside it, and
    `tools/__pycache__` was removed afterwards; every run that changed code
    (M2 to M5) ran in a `git clone` of the branch outside the repository. The
    worktree's `git status` was the same before and after.
- **To be committed as:** `docs/superpowers/plans/2026-11-22-a-tool-test-removes-the-temp-dirs-it-makes.md`.
- **Base:** `1f17c17`. **Branch:** `fix/the-chain-qa-test-removes-its-temp-trees`.
- **Owner's rulings taken in (2026-11-22):**
  - All nine leaking tool test files in one ticket ("Alla nio").
  - No new gate or check that a test leaves `$TMPDIR` as it found it ("Nej,
    bara rättningen").
  - Standing: no counts or measured figures in anything written down; one
    source per fact, pointed at; `go clean -cache` before and after every
    gate; push and merge on the owner's word.
- **Signed off:** by the owner, 2026-11-22: OD-1 = A (an id whose evidence
  reads `**OPEN — no test yet**`), and the plan "Godkänd".
- **Ids:** R1 = VTT-945, allocated at Task 0 after sign-off.

## The verdict, check by check

1. **Every path resolves. By command.** Each of the nine files exists, and
   each symbol the ticket names is where it says: `Tree` and
   `RealTreeInjected.setUpClass` in `tools/check_requirements_chain_qa_test.py`,
   with the roots and the `node_modules` exclusion as described, a `tearDown`
   that restores the register and deletes added files, and no
   `tearDownClass`; `tree` in `tools/check_requirements_chain_test.py`. Each
   `task check` step the ticket names runs the file it says (`Taskfile.yml`:
   `check:requirements-chain`, `check:doc-owner`, `check:ts-coverage`,
   `check:ts-mutation`, `check:new-prose`, `check:coverage`,
   `check:mutation`).
2. **"Done" is an observation. By command.** Item 1 fails today: M1. Item 2
   is held by the commands under Task 1's "True when" (the diff less its
   inserted test arguments, and each file's runner line against the base's).
   Item 3 is the gate.
3. **Each rule is breakable. By reading.** The rule fails the moment a test
   under `tools/` leaves something in an empty `TMPDIR` it ran under, whether
   it passed or failed (M4 shows each cleanup form's break). As worded the
   rule names directories only; see G1.
4. **The scope matches the claim. By command, then reading.**
   - The nine, and only the nine, leave entries (M1).
   - What leaves them is the test, never the tool it drives: with only the
     tests changed, every run leaves its directory empty (M2). No tool needs
     naming.
   - Every helper that makes a temporary path is local to its file: each
     `importlib` load in `tools/*_test.py` loads a tool, never another test
     file, and every call of a helper sits in a method of a `TestCase`.
   - No `Taskfile.yml` line, tool, hook or Go or client file changes.
   - The pre-commit hook also runs `task check:doc-owner` (`.lefthook.yml`),
     so `tools/check_doc_owner_test.py` leaks on every commit as well as in
     `task check` (G3).
5. **No recorded decision is contradicted silently. By reading.**
   SPEC-008, SPEC-020 and SPEC-023 name some of the nine as the tests that
   pin them; none says how a test handles a temporary path, and none
   changes. No ADR, specification or register row speaks of temporary files
   or `$TMPDIR` (searched: `docs/adr`, `docs/specifications`,
   `docs/requirements.md`). `tools/` is not among the inputs
   `tools/ts-mutation-inputs.py` hashes (`TREES`, `FILES`), so the client's
   stored mutation report is not moved by this work (SPEC-023).
6. **The records the work moves are named, and reachable. By command, then
   reading.** The section is present and says `None.`; a search of
   `docs/specifications/` for the nine file names and for temporary paths
   finds only the citations above, which stay true.

## The gaps this plan inherits

- **G1. The ticket says directories; some of the nine leave files.** Its
  problem says each of the nine "makes directories with `tempfile.mkdtemp`",
  and its rule says "every temporary directory". `tools/check_comment_wrap_test.py`,
  `tools/check_coverage_test.py` and `tools/check_ts_mutation_test.py` leave
  only files, and `tools/check_mutation_test.py` leaves files beside
  `cache_file`'s directories, all from `tempfile.NamedTemporaryFile(delete=False)`
  (M1). Done item 1 observes `$TMPDIR`, which holds both, so this plan works
  to item 1 and reads the rule as "every temporary file and directory"
  (the Requirements below). The ticket's wording is the writer's to correct.
- **G2. The ticket's "What could not be established"** is established here
  by command: which cleanup form each site needs (M3, D2 to D5), and that no
  tool under test makes a temporary entry of its own (M2).
- **G3. Where the nine run.** The ticket names `task check`; the pre-commit
  hook's `task check:doc-owner` runs `tools/check_doc_owner_test.py` on
  every commit too. The fix is the same; the leak was wider than stated.
- **G4. Item 2's "only how temporary directories are made and removed"**
  is read to include what the fix needs beside the cleanup lines: a test
  argument added to every call of a helper whose path outlives it (D2), the
  `os.unlink(eq)` lines removed from
  `test_the_gate_fails_and_explains_rather_than_reporting_a_clean_run` and
  `test_the_gate_fails_and_names_the_symlink_rather_than_measuring` (D6),
  and `tree`'s docstring in `tools/check_doc_owner_test.py`, whose "(caller
  cleans up)" is false today (D7). M6 shows nothing else moves.

## Measurements this plan stands on

- **M1. The base.** Each `tools/*_test.py` run as
  `env TMPDIR=<an empty directory of its own>/ PYTHONDONTWRITEBYTECODE=1 python3 -B tools/<file>.py -q`,
  then the directory listed. Every file passes. The nine the ticket names
  leave entries, and every other leaves its directory empty. What each
  leaves, and the code that makes it:
  - `check_requirements_chain_qa_test.py`: `qa-chain-*` directories from
    `Tree.__init__` (reached through `baseline` and the `NothingScanned`
    tests), and a `qa-chain-real-*` copy from `RealTreeInjected.setUpClass`.
  - `check_requirements_chain_test.py`: directories from `tree`, reached
    directly and through `clean_fixture`.
  - `check_doc_owner_test.py`: directories from `tree`, and from the inline
    `tempfile.mkdtemp()` in `test_an_empty_scan_is_a_failure_not_a_pass`.
  - `check_ts_coverage_test.py`: directories from the inline
    `tempfile.mkdtemp()` in `SourceWalk.test_the_extension_rule` and
    `SourceWalk.test_a_symlinked_directory_is_not_descended_into`.
  - `ts_mutation_inputs_test.py`: directories from `tree`, reached directly
    and through `OutsideTheClient.moved`.
  - `check_comment_wrap_test.py`: `.go` files from `flags` and from the
    inline `NamedTemporaryFile` in `test_tabs_are_measured_raw_not_expanded`.
  - `check_coverage_test.py`: `.out` and `.txt` files from `profile`,
    `expectfile` and `thresholds`, and from the inline `NamedTemporaryFile`
    in `test_malformed_profile_line_is_fatal`.
  - `check_mutation_test.py`: `.txt` files from `equivalents`, and
    directories from `SkipCacheTest.cache_file`. The `os.unlink(eq)` in the
    tests named in G4 runs at the end of the body, so not after a
    failure.
  - `check_ts_mutation_test.py`: `.txt` files from `equivalents_file`, which
    only `entries` calls.
  - No test inspects its temporary path after its run, and no helper is
    called outside a test method.
- **M2. The fix, prototyped** in a clone of the branch outside the
  repository, applied by a script that asserted each edit landed (D2 to D7).
  Each of the nine passes, its runner's `Ran` line equals the base's, and
  its directory is empty afterwards. Only test code changed, so no tool these
  tests drive leaves a temporary entry on the paths they drive.
- **M3. After a failure.** In the clone, each of the nine run through a
  wrapper that loads the file under a name other than `__main__`, replaces
  every `unittest.TestCase.assert*` with one that fails, and runs
  `unittest.main(module=...)`: each reports failures, and each directory is
  empty. With `ccw.check` raising, `flags` errors and leaves nothing; with
  `ctm.read_equivalents` raising, `entries` the same; with `shutil.copytree`
  raising, `RealTreeInjected.setUpClass` errors and leaves nothing. A
  variant with a `tearDownClass` in place of D5's `addClassCleanup` leaves
  its `qa-chain-real-*` directory under the same raising copy, and nothing
  on a clean run.
- **M4. The breaks.** In the clone, each with one cleanup taken out:
  `equivalents`' `addCleanup`, `entries`' removal in its `finally`, the real
  tree's `addClassCleanup`, and `tree`'s `addCleanup` in
  `check_doc_owner_test.py`. Each run passes and leaves entries.
- **M5. The mechanical gates on the prototype's diff.**
  `python3 -B tools/check-new-prose.py origin/main`: nothing to hold, since
  it reads `.go` and `.ts` alone (`CHECKED_EXT`).
  `PYTHONDONTWRITEBYTECODE=1 python3 -B tools/check-comments.py origin/main`:
  no added comment line, clean; its scope is Go and TypeScript under
  `internal/`, `cmd/` and `client/src` (`ROOTS`), and its notices on
  `internal/rules` test files predate this work. `tools/check-requirements-chain.py`
  over the clone: clean. No added or removed line carries a `VTT-` id.
- **M6. The diff less its inserted test arguments.** Pairing each removed
  line with the added line that equals it once `(self, ` or `(test, ` is read
  as `(`, and `(self)` or `(test)` as `()`, the residue is: each cleanup line, the
  `shutil` imports, the `try`/`finally` bodies of `flags` and `entries`, the
  `Tree.__init__` signature, the removed `os.unlink(eq)` lines, `tree`'s
  docstring and the empty-scan line in `check_doc_owner_test.py`.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion changes, no test is removed or
  skipped, no `Taskfile.yml` step or threshold changes. Each file's runner
  line equals the base's.
- **P2. Rule 8.** Any prose the change writes names a function, test or
  file, never a line.
- **P3. Rule 10 and SPEC-010.** Their scope is code under `internal/`,
  `cmd/` and `client/src`; `check:comments` and `check:new-prose` do not
  read Python under `tools/` (M5). The change adds no comment regardless
  (D8).
- **P4. Rule 9, answered in the ticket, checked here.** MapTool
  (`~/dev/RPTool`) has no Python test tooling. Its JUnit tests use the
  framework's `@TempDir` in `PackedFileTest`, and an end-of-body `delete()`
  in `AssetTransferTest`, the form that does not run after a failure.
  Nothing borrowed.
- **P5. The owner's rulings above.** All nine; no gate, check or committed
  QA test about `$TMPDIR`.
- **P6. The ticket's non-goals.** No tool under `tools/`, no Go test, no
  client test changes.
- **P7. The mechanics.** Python runs with `PYTHONDONTWRITEBYTECODE=1` and
  `-B`, then `rm -rf tools/__pycache__`; the gate started with `/bin/sh`,
  never zsh; `go clean -cache` before and after the gate; stage and commit
  in separate calls; the review recorded before the commit. No `client/src`
  change, so no webdist rebuild. Rules 3, 4 and 5 are untouched: no Go,
  contract or client code changes.

## Decisions

1. **D1. One code task.** All nine files in one commit.
   - **Forced by** no file depending on another (check 4), and by M6: the
     diff less its inserted test arguments is a short residue that a command
     isolates, so one QA run and one review cover it. A task per file would
     be a review per file of the same forms.
2. **D2. A helper whose path outlives it takes the test, and registers
   `test.addCleanup` on the line after the path is made.** `Tree(test)` and
   `baseline(test)` in `check_requirements_chain_qa_test.py`; `tree(test, **files)`
   in `check_requirements_chain_test.py`, `check_doc_owner_test.py` and
   `ts_mutation_inputs_test.py`, with `clean_fixture(test)`;
   `profile(test, ...)`, `expectfile(test, ...)` and `thresholds(test, ...)`
   in `check_coverage_test.py`; `equivalents(test, text)` in
   `check_mutation_test.py`. Every call passes `self`.
   - **Forced by** the path being used after the helper returns, so only the
     test's end can remove it; by `addCleanup` running after a failure (M3);
     and by the form already in `tools/`: `fixture(test)` and
     `stand_ins(test)` in `tools/ts_mutation_report_test.py`.
   - **Rejected:** one scratch directory per module made in `setUpModule`.
     Smaller diff, but a helper reached before it exists passes `dir=None`
     and writes to `$TMPDIR` without a sound, and no file in `tools/` does
     it. `atexit`: not the test's lifetime.
3. **D3. A helper whose path ends inside it removes it in a `finally`:**
   `flags` in `check_comment_wrap_test.py` (`Path(path).unlink()`) and
   `entries` in `check_ts_mutation_test.py` (`os.remove(path)`).
   - **Forced by** both reading the file eagerly before they return
     (`ccw.check` builds a list; `read_equivalents` reads the whole file),
     and by the `finally` running when the tool raises (M3). No call of
     either changes.
4. **D4. A path made inline in a test method gets `self.addCleanup` on the
   next line:** `SourceWalk.test_the_extension_rule` and
   `SourceWalk.test_a_symlinked_directory_is_not_descended_into`,
   `test_tabs_are_measured_raw_not_expanded`,
   `test_malformed_profile_line_is_fatal` (inside its `subTest` loop) and
   `SkipCacheTest.cache_file`. `test_an_empty_scan_is_a_failure_not_a_pass`
   calls `tree(self)` in place of `tempfile.mkdtemp()`.
   - **Forced by** the same lifetime and failure reasons as D2; the
     empty-scan test needs an empty directory, which `tree(self)` is.
5. **D5. The real-tree copy registers `cls.addClassCleanup(shutil.rmtree,
   cls.root, True)` straight after its `mkdtemp`, before the copy.**
   - **Forced by** M3: `setUpClass` can raise after the `mkdtemp` (the
     copy, the tag assertion), and a `tearDownClass` does not run then; the
     variant left its copy behind.
6. **D6. Files go with `os.remove`, directories with
   `shutil.rmtree(path, True)`.** The `os.unlink(eq)` lines in the tests
   named in G4 are removed.
   - **Forced by** the directory form already in `tools/` (D2's precedent),
     and by a file cleanup that finds its file gone raising: left in, those
     `os.unlink` lines would turn both tests into errors.
7. **D7. `tree`'s docstring in `check_doc_owner_test.py` loses "(caller
   cleans up)".**
   - **Forced by** it being false today and staying false: the helper
     cleans up, on the line below.
8. **D8. No comment is added.** Each cleanup line says what it does; there
   is nothing for a next editor to be warned of.
9. **D9. No tool, `Taskfile.yml` or hook changes.**
   - **Forced by** the ticket's non-goals and M2.
10. **D10. Phase 4a runs once, on the task, and commits nothing** (For Phase
    4a, below).
    - **Forced by** the change having behaviour a run observes (what a test
      leaves, after a pass and after a failure), which is where the cycle
      says QA runs; and by the owner's ruling, which refuses the permanent
      check its tests would be. Its runs and adjudications go in the report.
11. **D11. No entry in `docs/verification-debt.md`.**
    - **Forced by** the owner's ruling: no gate is meant to catch a future
      leak, so none let this one through. The rule's absence of a check is
      recorded once, by OD-1's answer and the report.
12. **D12. No specification moves, and the comment ledger is not written.**
    - **Forced by** check 6, and by M5: no share in `check:comments`' scope
      moves.

## Open decisions for the owner

- **OD-1. What the register holds for the ticket's rule.** The rule can
  fail and a check could hold it; the owner has ruled that none will.
  - **A (recommended). An id, its evidence `**OPEN — no test yet**`.** The
    `requirements` skill's "gap worth recording", in SPEC-008's words "a gap
    worth recording keeps its row, marked `OPEN`", as `VTT-077` and
    `VTT-258` stand today. The register then states the rule and that
    nothing runs it. Cost: a row that stays open.
  - **B. An id, its evidence `**READING — Phase 4b**`.** Cost: the skill
    keeps `READING` for a rule no check could hold, and this one a check
    could; and no later review is briefed to read for it.
  - **C. Refused, no id.** The rule lives in the ticket and the report
    alone. Cost: the skill refuses only what cannot fail, and this can; a
    later reader of the register does not meet the rule.

## Requirements

Sorted at Task 0 with the `requirements` skill, as OD-1 leaves it; the
register's words bind.

| R | Item | Rule (as drafted) | Red under |
|---|---|---|---|
| R1 | 1 | A Python test under `tools/` removes every temporary file and directory it makes, when it fails as when it passes. | the file run with `TMPDIR` set to an empty directory, as it stands and with its assertions made to fail, leaves an entry there |

**Candidates the sort refuses:**
- **"Each of the nine files leaves its directory empty"** (item 1). The
  rule's instance today, observed in the report; not a second rule.
- **"No assertion changes"** (item 2). A property of this diff, held by the
  review's commands; not a rule of the system.
- **"`task check` is green"** (item 3). The gate, not a rule.
- **"A cleanup runs after a failure", as its own row.** Part of R1.
- **"No tool under `tools/` leaves a temporary entry".** Not the ticket's
  claim; true of the paths these tests drive (M2), recorded in the report.

**Rows that become false:** none. No row speaks of temporary paths.

**The sort at Task 0** kept R1 as drafted, as VTT-945 with its evidence
`**OPEN — no test yet**` (OD-1 A), and refused the candidates above.

## Tasks

### Task 0: the records

- **Do:** after sign-off, run the sort over R1 as OD-1 leaves it; under A or
  B allocate its id with `requirement-id`, and under B set its evidence to
  `**READING — Phase 4b**`; commit the ticket, this plan and, under A or B,
  `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: every tool test removes what it makes (R1; D2 to D8; items 1 and 2)

- **Files:** `tools/check_requirements_chain_qa_test.py`,
  `tools/check_requirements_chain_test.py`, `tools/check_doc_owner_test.py`,
  `tools/check_ts_coverage_test.py`, `tools/ts_mutation_inputs_test.py`,
  `tools/check_comment_wrap_test.py`, `tools/check_coverage_test.py`,
  `tools/check_mutation_test.py`, `tools/check_ts_mutation_test.py`.
- **Seen red first:** M1's command on the unchanged tree, each of the nine
  leaving entries; each file's runner line (`Ran ... tests`, and `OK` or its
  skips) kept from this run.
- **True when:**
  - M1's command leaves every `tools/*_test.py`'s directory empty, and each
    passes.
  - M3's assertion-failure wrapper over each of the nine reports failures
    and leaves its directory empty; with `shutil.copytree` raising,
    `RealTreeInjected.setUpClass` errors and leaves nothing.
  - Each file's runner line equals the one kept from the red run.
  - M6's pairing over `git diff HEAD -- tools/` leaves only the residue M6
    names.
  - P7's checks pass, and `tools/__pycache__` is gone.
- **Deliberate breaks** (each followed by M1's command on that file, then
  repaired and re-run clean):
  - Take out `RealTreeInjected`'s `addClassCleanup`: a `qa-chain-real-*`
    directory remains.
  - Take out `Tree.__init__`'s `addCleanup`: `qa-chain-*` directories
    remain.
  - Take out `equivalents`' `addCleanup` in `check_mutation_test.py`: `.txt`
    files remain.
  - Take out `entries`' removal in its `finally`: `.txt` files remain.
  - Take out `flags`' removal in its `finally`: `.go` files remain.
  - Take out `profile`'s `addCleanup`: `.out` files remain.
  - Take out `SkipCacheTest.cache_file`'s `addCleanup`: directories remain.
  - Take out `tree`'s `addCleanup` in `check_doc_owner_test.py`:
    directories remain.
  - Take out `tree`'s `addCleanup` in `check_requirements_chain_test.py`,
    and in `ts_mutation_inputs_test.py`: directories remain.
  - Take out the `addCleanup` in `SourceWalk.test_the_extension_rule`: a
    directory remains.
  - The other direction, on the repaired tree: M1's command over every
    `tools/*_test.py` leaves every directory empty.
- **Phase 4a:** applies, once (D10).
- **The state here:** no tool test leaves anything in `$TMPDIR`, after a
  pass or a failure.

### Task 2: the gate (item 3)

- **Do:** `go clean -cache`; check free space and the load
  (`os.getloadavg()`); list `$TMPDIR`'s `qa-chain-*` entries; run
  `task check` whole on the last code commit, detached under `/bin/sh`,
  output to a file, requiring its completion line; list `$TMPDIR`'s
  `qa-chain-*` entries again; `go clean -cache`. No `--write-ledger` (D12).
- **True when:** `task check` is green, and the gate added no `qa-chain-*`
  entry to `$TMPDIR`.

### Task 3: the report

- **The report:** `docs/reports/2026-11-22-a-tool-test-removes-the-temp-dirs-it-makes.md`,
  with the `implementation-report` skill and its reading review: each "Done
  looks like" item with its observation; M1 per file before, and after
  Task 1; the breaks and what each left; OD-1's answer and R1's outcome; G1
  to G4; Rule 9 as P4 answers it; Phase 4a's adjudications under their own
  heading; the gate.
- **Specifications:** none moves (D12).
- **True when:** each sentence has been run against the tree by someone
  other than its writer. Commit the report; push and merge on the owner's
  word.

## For Phase 4a

Hand the QA agent the ticket, R1 as signed off, the list of the nine files,
the `Taskfile.yml` lines that run them, and each tool's top-level names and
signatures (`inspect.signature`, no bodies). Not the diff and not the nine
files' source. It works in a copy of the tree outside the repository and
never edits the nine files. Tell it:
- Run each file with `TMPDIR` set to an empty directory, and list it.
- Make each file fail without reading it: through a wrapper that replaces
  `unittest.TestCase`'s assertions with failing ones; by replacing the tool
  a file drives with a stand-in that raises; and, for
  `RealTreeInjected.setUpClass`, by making its copy fail in the copy of the
  tree. After each, the directory must be empty.
- A run that fails at import makes nothing and proves nothing: require the
  runner's own line naming the tests that ran, and that it reports
  failures.
- Commit nothing (the owner's ruling); its findings go in the report.
- Never run `task check`, `task e2e` or Stryker; remove `tools/__pycache__`
  after its runs.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: run M6's pairing and read
only what it leaves; check that each cleanup is registered on the line after
its path is made, before anything that can raise; check that nothing reads a
path after the cleanup that removes it (`flags` and `entries` return what
they read); check each file's runner line against the base's, run in a copy
of `1f17c17` made by `git archive`; check that
`tree`'s docstring in `check_doc_owner_test.py` is true; check every scope
claim in the report ("every", "only", "no other") names the search that
bounds it.
