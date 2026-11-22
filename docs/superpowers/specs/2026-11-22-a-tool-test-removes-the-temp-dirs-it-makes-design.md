# A tool test removes every temporary directory it makes

## The problem

**Nine of the Python tests under `tools/` leave temporary directories
behind.** Each makes directories with `tempfile.mkdtemp` and never removes
them, so every run adds to `$TMPDIR`:

- `tools/check_requirements_chain_qa_test.py`: `Tree` makes a `qa-chain-*`
  directory per synthetic tree, and `RealTreeInjected.setUpClass` copies
  `docs`, `internal`, `cmd`, `client` (without `node_modules`), `tools` and
  `contract` into a `qa-chain-real-*` directory; neither is removed (its
  `tearDown` restores the copied register and deletes added files, and there
  is no `tearDownClass`).
- `tools/check_requirements_chain_test.py` (`tree`),
  `tools/check_doc_owner_test.py`, `tools/check_ts_coverage_test.py`,
  `tools/ts_mutation_inputs_test.py`, `tools/check_comment_wrap_test.py`,
  `tools/check_coverage_test.py`, `tools/check_mutation_test.py` and
  `tools/check_ts_mutation_test.py`.

Shown by running each file with `TMPDIR` set to an empty directory of its own
(`env TMPDIR=<dir>/ python3 -B tools/<file>.py -q`) and listing that
directory afterwards: for these nine it is not empty, for every other
`tools/*_test.py` it is. `task check` runs all nine, in its
`check:requirements-chain`, `check:doc-owner`, `check:ts-coverage`,
`check:ts-mutation`, `check:new-prose`, `check:coverage` and `check:mutation`
steps (`Taskfile.yml`), so the directories pile up with each run. On
2026-11-21 the `qa-chain-real-*` copies alone had filled enough of the disk
to put `check:mutation` under its free-space floor, which showed as failing
`check_mutation_test` cases rather than as a disk refusal; the copies have
since been deleted by hand before and after each gate.

**The owner's rulings, 2026-11-22.** Asked what the chain test is and offered
its fix: "ok kör det". Shown that eight more tool tests leak: all nine in one
ticket ("Alla nio"), and no new check to stop a future leak ("Nej, bara
rättningen").

**What MapTool does.** Nothing to compare: MapTool has no Python gate
tooling, and how a test cleans its scratch space is not a tabletop question.

**What this is not.** A new gate or a check that a test leaves `$TMPDIR` as it
found it (the owner's ruling). A change to what any of these tests asserts,
or to the tools they test. The Go tests (`t.TempDir` removes itself) and the
client's tests.

## Done looks like

1. **Each of the nine files leaves its temporary directory empty.** Run with
   `TMPDIR` set to an empty directory of its own, each passes and the
   directory is empty afterwards; on today's tree each leaves entries.
2. **No assertion changes.** Each file's tests pass, and each test asserts
   what it asserted: the diff touches only how temporary directories are made
   and removed.
3. **The gate.** `task check` is green.

## Rules this puts on the system

- A Python test under `tools/` removes every temporary directory it makes.

## What it touches

The nine files named above, all under `tools/`. No other component.

## Specifications this moves

None.

## What could not be established

- **Whether a test removes its directory when it fails.** A cleanup made by
  `addCleanup` or `tearDownClass` runs after a failure too; one written at the
  end of a test body does not. Which form each file needs was not traced.
- **Whether any of the tools these tests run makes temporary directories of
  its own** (rather than the test). The measurement counts both; a tool that
  leaks would need its own fix, which this ticket would then name.
