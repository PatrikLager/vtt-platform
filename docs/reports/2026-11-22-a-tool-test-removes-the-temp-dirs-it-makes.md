# A tool test removes every temporary directory it makes — report

Ticket: `docs/superpowers/specs/2026-11-22-a-tool-test-removes-the-temp-dirs-it-makes-design.md`.
Plan: `docs/superpowers/plans/2026-11-22-a-tool-test-removes-the-temp-dirs-it-makes.md`.
Branch `fix/the-chain-qa-test-removes-its-temp-trees` from main `1f17c17`.
Commits: `604586e` (Task 0: ticket, plan, register) and `1ae67f9` (Task 1,
the nine files; the last code commit). Every code reference below is to
`1ae67f9`.

## Done looks like

1. [x] **Each of the nine files leaves its temporary directory empty.** Run
   with `TMPDIR` set to an empty directory of its own (`env TMPDIR=<dir>/
   python3 -B tools/<file>.py -q`), each of
   `tools/check_requirements_chain_qa_test.py`,
   `tools/check_requirements_chain_test.py`, `tools/check_doc_owner_test.py`,
   `tools/check_ts_coverage_test.py`, `tools/ts_mutation_inputs_test.py`,
   `tools/check_comment_wrap_test.py`, `tools/check_coverage_test.py`,
   `tools/check_mutation_test.py` and `tools/check_ts_mutation_test.py`
   passes and leaves the directory empty; so does every other
   `tools/*_test.py`. On `604586e` each of the nine passed and left entries:
   directories (`qa-chain-*` and a `qa-chain-real-*` copy of the tree from
   the chain QA test; directories from the chain, doc-owner, ts-coverage,
   ts-mutation-inputs and mutation tests) and files (`.go` from the
   comment-wrap test, `.out` and `.txt` from the coverage test, `.txt` from
   the mutation and ts-mutation tests). The rule is VTT-945, whose evidence
   stays `**OPEN — no test yet**`: the owner ruled that no check is written
   (OD-1 A).
2. [x] **No assertion changes.** Each of the nine runs the same tests with
   the same result as on `1f17c17`, test by test (the reading review, in
   `git archive` copies of both). Paired over `git diff 604586e..1ae67f9 --
   tools/` with the inserted test arguments read away (the plan's M6), what
   is left is the cleanup lines, the `shutil` imports, the `try`/`finally`
   in `flags` and `entries`, `Tree.__init__`'s new parameter, the removed
   `os.unlink(eq)` lines, and in `check_doc_owner_test.py` `tree`'s
   docstring and the empty-scan test's `tree(self)` in place of
   `tempfile.mkdtemp()`; every line of the diff that holds an
   assertion differs only by `self` passed to a helper.
3. [x] **The gate.** `task check` is green on `1ae67f9`. `$TMPDIR` listed
   just after it started and again after it ended holds no `qa-chain-*`
   path; what it gained were `semgrep-*.rules` files, which semgrep writes
   (below).
   Its Go mutation gate reports no unadjudicated survivor in the packages it
   measures, none of which this ticket touches; it judges no client mutation
   report, the stored one having measured `b90fc1a` (SPEC-023), and
   `tools/` is not among that report's inputs.

## What the rule became

- **A Python test under `tools/` removes every temporary file and directory
  it makes, when it fails as when it passes.** VTT-945, evidence
  `**OPEN — no test yet**` (OD-1 A): a check could hold it, and the owner
  ruled none is written. The ticket's rule said "directory"; some of the
  files left files (G1), so the row says "file and directory".

Refused by the sort: "each of the nine files leaves its directory empty"
(the rule's instance today, item 1); "no assertion changes" (a property of
this diff); "`task check` is green" (the gate); "a cleanup runs after a
failure" as its own row (part of VTT-945); "no tool under `tools/` leaves a
temporary entry" (not the ticket's claim; true of the paths these tests
drive).

## Rule 9

MapTool (`~/dev/RPTool`) has no Python test tooling. Its JUnit tests use
the framework's `@TempDir` in `PackedFileTest`, and an end-of-body `delete()`
in `AssetTransferTest`, the form that does not run after a failure (the
plan's P4). Nothing borrowed.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| D6: files go with `os.remove` | The tabs test in `check_comment_wrap_test.py` registers `Path(f.name).unlink` as its cleanup | D3 prescribes `Path(path).unlink()` for `flags` in that file, which has no `import os`; both call unlink and both raise if the file is gone |

## The deliberate breaks

Each was applied, its file run with an empty `TMPDIR`, and restored from a
saved copy compared byte for byte. Taking out the cleanup of
`RealTreeInjected`'s copy left a `qa-chain-real-*` directory, also with the
copy made to fail; of `Tree`, `qa-chain-*` directories; of `equivalents`,
`.txt` files; of `entries`, the removal in its `finally`, `.txt` files; of
`flags`, the removal in its `finally`, `.go` files; of `profile`, `.out` files; of
`SkipCacheTest.cache_file`, directories; of `tree` in the doc-owner, chain
and ts-mutation-inputs tests, directories; of
`SourceWalk.test_the_extension_rule`'s directory, that directory. On the
repaired tree every `tools/*_test.py` left its directory empty.

## Phase 4a adjudications

Run once, in a `git archive` copy with the change applied, and committing
nothing (the owner's ruling, D10). No findings. Every `tools/*_test.py`
passed and left its `TMPDIR` empty. Each of the nine was made to fail
without being read: every assertion replaced by one that fails, every
function of the tool it drives replaced by one that raises, and, for the
chain QA test, its copy of the tree made to fail partway; each run's
runner line shows its tests ran and failed or errored, and each directory
was empty.
The tool stand-in had no effect on the chain QA test, which runs its tool
as a subprocess; every subprocess call made to raise stood in for it, and
left nothing. Not exercised: a failure inside a `setUp` or `tearDown`
alone.

## The plan's gaps

- **G1.** Some files left files as well as or instead of directories
  (`NamedTemporaryFile` with `delete=False`): the comment-wrap, coverage and
  ts-mutation tests left files only, the mutation test files and
  directories. Item 1's measurement catches both, and VTT-945 says "file and
  directory".
- **G2.** The ticket's open points are settled: a cleanup registered with
  `addCleanup` or `addClassCleanup` runs after a failure (Phase 4a); no tool
  leaves a path of its own on the paths these tests drive (the plan's M2,
  and item 1's measurement after the change).
- **G3.** The pre-commit hook runs `task check:doc-owner`, so
  `check_doc_owner_test.py` leaked on every commit, not only in `task
  check`.
- **G4.** Item 2's diff holds the inserted test arguments, the removed
  `os.unlink(eq)` lines (left in, they would make the new cleanup raise),
  `tree`'s docstring and the empty-scan test's `tree(self)`, as well as the
  cleanup lines.

## What could not be established

Nothing was left open: the ticket's open points are settled (G2).

## What was deliberately left out

- **A write that fails before its cleanup is registered.** In `flags`
  (`tools/check_comment_wrap_test.py`) and in `entries`
  (`tools/check_ts_mutation_test.py`) the temporary file is written and
  closed before the `try` whose `finally` removes it, so a write that fails
  at the disk leaves that one small file, and VTT-945 as worded is not met
  in that window. Near-theoretical, with a small consequence: no code (the
  owner's ruling of 2026-10-21 on such edge cases).
- A check in `task check` that a test leaves `$TMPDIR` as it found it: the
  owner's ruling.
- The tools under test, `Taskfile.yml` and the hooks (D9). `tools/check-coverage.py`
  emits a `ResourceWarning` for a file it leaves open, as it did on
  `1f17c17`; it leaves nothing in `$TMPDIR`.
- The Go tests and the client's tests.
- Semgrep's own temporary rule files (`semgrep-*.rules` in `$TMPDIR`), which
  a `task check` run leaves: semgrep is a tool the gate runs, not a test
  under `tools/`.
