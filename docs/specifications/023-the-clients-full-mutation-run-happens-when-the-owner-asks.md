# SPEC-023: The client's full mutation run happens when the owner asks

## Status

Accepted. Implemented by `tools/ts-mutation-report.py` (`check`, `snapshot`,
`stamp`, `report_state`, `tickets_since`), by `tools/ts-mutation-inputs.py`
(`TREES`, `FILES`, `inputs_hash`), and by `Taskfile.yml`'s
`check:ts-mutation`, a step of `task check`, and `check:ts-mutation:full`,
which is not one; pinned by `tools/ts_mutation_report_test.py` and
`tools/ts_mutation_inputs_test.py`, and by QA's
`tools/ts_mutation_report_qa_test.py` and
`tools/ts_mutation_inputs_qa_test.py`, derived from the rows and the plan's
decisions.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record serves would read: a gate that did not measure says so.

## How it works

**The full run happens when the owner asks for it** (the owner's rule of
2026-10-26: "vi kör bara Full Stryker, när jag efterfrågar det"). It is `task
check:ts-mutation:full`, and in order it:
1. snapshots the tree with `ts-mutation-report.py snapshot`;
2. runs Stryker over `stryker.conf.json`'s `mutate` list, every `.ts`, `.tsx`,
   `.mts` and `.cts` file under `client/src` but `client/src/main.ts`: in a
   Linux container on Darwin 25 and later unless `VTT_FORCE_TS_MUTATION=1`,
   natively otherwise;
3. turns the snapshot into the stamp with `ts-mutation-report.py stamp`;
4. judges the report with `tools/check-ts-mutation.py`, whose rules are
   SPEC-020's.

`stamp` writes nothing and exits 1 unless `reports/mutation/mutation.json` is
newer than the snapshot's start (VTT-514). The report is stamped before it is
judged, so a survivor does not make it out of date: while the tree hashes as
it did when the run began, each `task check` judges the report red again
(VTT-515).

**The stamp** is `reports/mutation/ts-mutation-stamp.json`, one JSON object,
written whole through a temporary file and a rename:
- `inputs`, the input hash of the tree when the run began;
- `commit`, the full SHA checked out when the run began (VTT-512);
- `clean`, whether no file the hash reads differed from that commit, in the
  index or the working tree, or stood untracked, an ignored one included
  (VTT-513), or null when that was not recorded;
- `started`, the run's start in epoch seconds, which a stamp written for a
  report no snapshot preceded may lack.

A stamp whose fields are not of their kind reads as unreadable, and `stamp`
refuses a snapshot whose fields are not.

**The input hash** (`tools/ts-mutation-inputs.py`) is a SHA-256 over the name
and version range of the first `package.json` dependency naming Stryker, and
over every file below, by path and content (VTT-516):
- under `client/src` and `client/test`, the `.ts`, `.tsx`, `.mts` and `.cts`
  files;
- under `contract/gen/ts`, the `.ts` files, which the client imports;
- under `contract/testdata` and `scenarios/goldens`, the `.json` files, which
  the client's tests read;
- under `campaigns/example/art`, the `.png` and `.json` files, the suffixes of
  `internal/artlib`;
- the files `stryker.conf.json`, `package.json`, `bun.lock`, `bun.lockb`,
  `client/tsconfig.json`, `contract/gen/tools/tools.json` and
  `campaigns/example/maps/cellar.json`.

Each tree must exist, or the hasher prints no hash, only an error on stderr,
and exits 1. It reads the working tree, so an untracked file with a hashed
suffix counts too.

**`task check`'s step, `check:ts-mutation`,** runs the tests of the checker,
the hasher and `ts-mutation-report.py`, QA's among them, then
`ts-mutation-report.py check`. It starts no Stryker run and no container
(VTT-507). `check` reads the report as:
- CURRENT when `mutation.json` exists, the stamp is readable, and the stamp's
  `inputs` is the tree's hash. The step prints the commit the report measured
  and how clean its tree was, runs the checker, and exits with its verdict
  (VTT-511).
- MISSING when there is no `mutation.json`. The step prints THE CLIENT'S
  MUTATION WAS NOT MEASURED and that there is no stored report, and exits 0
  (VTT-510).
- OUT OF DATE otherwise, an absent or unreadable stamp and an unhashable tree
  included. `check` runs no part of the checker and starts no process but
  `git`; the step prints THE CLIENT'S MUTATION WAS NOT MEASURED with the
  reason, the commit the report measured, how clean its tree was and the
  tickets added since, and exits 0 (VTT-508, VTT-509). Both paths end by
  naming the full run.

**The tickets added since** are the files named `*-design.md` directly in
`docs/superpowers/specs/`, in the working tree, that the tree of the stamp's
commit lacks, sorted by name. It is a tree difference, not a history walk:
it counts a ticket not yet committed, and answers for a commit that is not an
ancestor of `HEAD`, which the step then says. With no commit recorded, or one
the repository does not hold, the step says the tickets cannot be listed, and
why.

`_report_is_current` in `tools/check_ts_mutation_qa_test.py` asks
`report_state`, so the tests of the real report run while it is current and
skip otherwise; with `QA_TS_MUTATION_REPORT` naming a report that exists,
they run over that report whatever its state.

## Consequences

- A green `task check` says nothing about the client's mutants unless its
  step printed that the report is current. THE CLIENT'S MUTATION WAS NOT
  MEASURED is the line that says it did not.
- What the full run would find gathers between runs: a survivor in new or
  changed code, or a test that no longer kills a mutant, is seen at the next
  full run and not before. The tickets the step lists are the work added
  since the measured commit, whatever each touched.
- An edit to any file the hash reads, a comment included, reads the report
  out of date until the next full run, or until the file is restored byte for
  byte.
- Go mutants are judged at every `task check` (`check:mutation`); the
  client's only over a current report.

## Requirements

VTT-507, VTT-508, VTT-509, VTT-510, VTT-511, VTT-512, VTT-513, VTT-514,
VTT-515, VTT-516.
