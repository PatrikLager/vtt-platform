# A comment line carrying only requirement ids counts in no comment share

## The problem

`tools/check-comments.py` measures a file's comment share as comment lines over
non-blank lines, and `measure` counts every line that begins with `//` as a
comment line. A test cites the requirement it holds with a line that carries
the bare id and nothing else, `// VTT-042`, the shape `tools/check-requirements-chain.py`
requires and CLAUDE.md rule 10 names as a pointer. That line is prose to the
share. `--write-ledger` writes each ceiling as the share rounded up to a tenth,
so a file whose ledger row was written after a sweep has no room for one more
comment line: `internal/identity/identity_test.go` stands at 53 comment lines
over 1,347 non-blank, 3.93 percent, ceiling 4.0, and one added citation line
takes it to 4.006, which the gate refuses, printing the share to one decimal:
`comment share 4.0 is above its ceiling 4.0 and this change added a comment
line to it`. The same holds for
`fault_internal_test.go` (8.78, ceiling 8.8) and `identity_failure_test.go`
(6.86, ceiling 6.9). A rule that a swept package's tests already hold can
therefore not be given a row, since the row's citation is refused by the gate
the sweep lowered; and the better a sweep does, the sooner the next citation
is refused. Nine files in scope carry such lines today (measured at `8181d47`:
21 in `identity_test.go`, 10 each in `internal/gateway/join_test.go` and
`server_test.go`, 5 in `authz_test.go` and in `fault_internal_test.go`, 2 in
`internal/engine/qa_role_test.go`, 1 each in `role_test.go`,
`internal/gateway/qa_joining_test.go` and `qa_joining_internal_test.go`).

## Done looks like

1. In a scratch clone of this tree, adding the line `// VTT-042` directly
   above a `func Test` in `internal/identity/identity_test.go` and running
   `python3 tools/check-comments.py main` exits 0 with the completion line.
   Today the same run exits 1 with the finding quoted above.
2. Still refused afterwards: the line `// VTT-042 holds this`, since a word
   beside the id makes it a comment line, is refused with the ceiling finding
   in a fixture whose row sits at its share exactly (on the real tree the
   rewritten row leaves headroom for one such line, and two are refused); and
   `// VTT-042` added to a file whose share
   is already above its ceiling is not refused by the ceiling check, because
   the change added no comment line, while an added `// VTT-042 and more` is.
3. `tools/check_comments_test.py` has one case per line of item 1 and item 2;
   the cases for what passes afterwards are red on today's checker, the cases
   for what is still refused are green today and go red under a deliberate
   break of the new shape; `task check:comments` runs them.
4. `python3 tools/check-comments.py --report` prints
   `internal/identity/identity_test.go` at 2.4 percent, from 3.9, and
   `tools/comment-ceilings.txt`, rewritten by `--write-ledger` in the same
   commit as the checker, lowers the nine rows named in the problem and no
   other; `task check:comments` ends `clean`.
5. `docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md` says,
   under "Files are held by a ceiling", that a comment line carrying only
   requirement ids counts in no share and is not an added comment line; the
   checker's docstring says what such a line is, the register's tag and
   nothing but ids.
6. `task check` whole is green.

## Rules this puts on the system

A comment line that carries only requirement ids counts in no comment share,
no added-line count and no block length.

## What it touches

1. `tools/check-comments.py`
2. `tools/check_comments_test.py`
3. `tools/check_comments_qa_test.py`, if Phase 4a's QA derives a case that
   belongs beside its others
4. `tools/comment-ceilings.txt`, by `--write-ledger` only
5. `docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md`
6. `docs/requirements.md`, one row, after sign-off

One component; the checker and its test change first, the ledger last, in one
commit. No file under `internal/`, `cmd/` or `client/src` changes.

## Specifications this moves

docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md

## What could not be established

- Where the checker learns the register's tag: `tools/check-requirements-chain.py`
  reads it from the `project:` line of `docs/requirements.md`; whether the
  comment gate reads the same line or takes a generic `TAG-NNN` shape is the
  plan's decision, and so is what a line with no register counts as.
- Whether a block made only of citation lines, or a block of citations under
  a how-line, still counts toward the six-line bound. The rule above says no
  for the citation lines; the plan says how the block is measured.
- Whether `.ts` test files ever carry such a line: none does today
  (`grep -rn 'VTT-' client/src` prints nothing), and the chain gate scans
  `*.test.ts`, so the same shape is to be expected there.
- Rule 9 of CLAUDE.md does not apply: no tabletop function is designed.
