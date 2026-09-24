# internal/identity carries only warnings and pointers, and its ceilings say so

## The problem

`internal/identity` is the first package swept under SPEC-010. Measured at
`e872467` with `python3 tools/check-comments.py --report`:

| File | Comment lines | Blocks over the bound | Banned lines |
|---|---|---|---|
| `identity.go` | 259 of 678 | 14, holding 150 lines | 0 |
| `identity_test.go` | 380 of 1,674 | 22, holding 239 lines | 20 |
| `fault_internal_test.go` | 94 of 385 | 4, holding 58 lines | 7 |
| `identity_failure_test.go` | 39 of 134 | 1, holding 24 lines | 2 |
| `qa_joining_internal_test.go` | 23 of 182 | 0 | 0 |

`identity.go`'s comments were sorted by hand in `9920a67` against the rule
"what the code cannot say about itself"; what remains still describes behaviour
in paragraphs, and the reading review of that commit found two of those
sentences false on the day they were written. In `identity_test.go`, 225 of the
380 comment lines sit directly above a `func Test...`, narrating what a review
measured, what the mutation gate found and what the code did before, above
names that already state the rule; `fault_internal_test.go`'s longest block,
twenty lines above its first migration test, recounts a fixture and four tests
that no longer exist. Every such block is a second source that goes false when
the code moves, and the gate now refuses any change that touches one without
cutting it to the bound.

The decisions those comments explain are SPEC-009's; the history is in
`docs/reports/2026-08-09-joining-a-table.md` and
`docs/reports/2026-09-24-joining-record-and-code.md`. Nothing that goes has
to be moved, and nothing that stays may be more than a warning, a pointer or
a doc sentence.

## Done looks like

1. `python3 tools/check-comments.py --report | grep internal/identity/` prints
   `banned 0` and `blocks>6 0` on every line. Today: 29 banned lines and 41
   blocks over the bound across the five files.
2. Above every `func Test...` in the package's four test files there is at most
   the `VTT-NNN` citation line and one line saying how the test observes what
   its name states; `grep -c` of doc blocks longer than two lines above a test,
   by the script the plan writes, prints 0. Today: 30 such blocks, 29 of them
   in `identity_test.go`, whose doc blocks above tests hold 225 lines in all.
3. Every comment block left in `identity.go` is an imperative warning to
   whoever edits next, a pointer (SPEC-009, a VTT id, a test or symbol name),
   or the doc sentence of an exported symbol, by the reading Phase 4b holds
   for VTT-051; and `--report` shows `identity.go` at or under 20.0 percent,
   from 38.2. Today: 46 blocks, of which 14 run past the bound.
4. `tools/comment-ceilings.txt`'s rows for the four files that change are
   lowered by `python3 tools/check-comments.py --write-ledger` in the same
   commit as the deletions, and `task check:comments` is green with them;
   adding one comment line back to `identity.go` afterwards reds it naming the
   file and both shares (the break the commit line records). Today: the rows
   stand at the shares measured when the ledger was written.
5. Still true afterwards: `go test -count=1 ./internal/identity/...` is green;
   `task check` whole is green; SPEC-009 is unchanged unless the reading in
   item 3 finds a decision only a comment held, in which case SPEC-009 gains
   the sentence and the report names which.
6. Still true afterwards: `git diff --stat e872467 -- docs/reports/` prints
   nothing but this ticket's own report; the two reports that hold this
   package's history are not revised.

## Rules this puts on the system

This ticket adds no rule. It works under VTT-050, VTT-052, VTT-053 and VTT-055,
which the gate holds, and under VTT-051, which the reading holds.

## What it touches

1. `internal/identity/identity.go`
2. `internal/identity/identity_test.go`
3. `internal/identity/fault_internal_test.go`
4. `internal/identity/identity_failure_test.go`
5. `tools/comment-ceilings.txt`, by `--write-ledger` only
6. `docs/specifications/009-identity-and-joining.md`, only if item 3's reading
   finds a decision it lacks

`qa_joining_internal_test.go` is clean and is not touched. One component; the
comments change first, the ledger last, in one commit. No code line changes:
a comment-stripped comparison of each file before and after prints nothing.

## Specifications this moves

docs/specifications/009-identity-and-joining.md — only if a deleted comment
holds a decision the record lacks; otherwise it is unchanged, and the report
says so.

## What could not be established

- Whether any comment in `identity.go` holds a decision SPEC-009 lacks. That
  is the reading the sweep consists of; it is done block by block and its
  outcome is the report's.
- The share `identity.go` lands at. 20 percent is the bound the ticket sets
  from a model of what survives (a doc sentence per exported symbol, one to
  three warning lines per block that has one); the reading decides the real
  figure, and the ledger records it.
- Whether the mutation gate re-mutates `internal/identity` for a change that
  moves no code line. Its skip cache keys on the package's dependency closure,
  which includes test files, so it will; `identity.go` carries no mutation
  adjudication key, so none moves.
- Rule 9 of `CLAUDE.md` does not apply: no tabletop function is designed.
