# A comment in code is a warning or a pointer, and a gate holds it

## The problem

Production Go under `internal/` and `cmd/`, generated files aside, is 12,777
comment lines of 26,070 non-blank lines; the test files are 16,813 of 55,688;
`client/src` is 3,301 of 6,188. Measured 2026-09-24 at `2e24606`. Of those
comment lines, between 1,730 and 2,206 carry a date, "measured", "used to",
"review found", "spec §", a task number or an issue number, the range depending
on how the list is read; the list the plan adopts finds 2,192. That is history,
measurement and argument, which `docs/reports/` and `docs/specifications/`
exist to hold. The blocks around the read loop's `s.ids.Lookup(p.ID)` in
`internal/gateway/server.go` narrate a defect's history, and the one opening
"ANY OTHER ERROR IS OPERATIONAL" says where a sentence "used to be visible";
`internal/artlib/artlib.go` is two thirds comment. Each such line is a second
source: it describes the code as it was when written, goes false when the code
moves, and is found false only by a reader. Item 4 of
`docs/reports/2026-09-24-joining-record-and-code.md` records two false
sentences found in `internal/identity/identity.go` on the day its comments were
sorted by hand.

No check reads what a comment says or how much comment a file carries.
`check:new-prose` (`tools/check-new-prose.py`, over `main`) holds two things on
added lines, wrap width and that a cited name exists; `check:doc-owner` holds
that a doc comment sits on its own function. A comment that says "measured 35
failures in 40 trials" passes both. The cost lands on every code change: the
comments beside it are corrected by hand, in rounds, and the correction is the
larger part of the review.

## Done looks like

1. `docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md` exists,
   with the five headings, and states the rule: a comment in code is an
   imperative warning to whoever edits next, or a pointer to a record (a
   specification by number, a requirement by id, a test or symbol by name, a
   report by path), or the one-line doc sentence `go doc` prints for an
   exported symbol. History, measurements, arguments and descriptions of
   behaviour are not comments; the report and the specification hold them.
   Today: no such file.
2. `task check:comments` exists as a step of `task check`, held by
   `tools/check-comments.py` with a self-test `tools/check_comments_test.py`
   that is red before the checker exists. Over the comment lines a change ADDS
   against `main` (the scope `check:new-prose` uses, for the reason its
   docstring gives), it refuses, naming the file and line: a line carrying a
   date of the shape `20YY-MM-DD`, or the words `measured`, `used to`,
   `previously`, `an earlier version`, `turned out`, `review found`, `spec §`,
   `Task N`, or an issue number `#NNN`; and a comment block longer than a bound
   the plan sets, unless it is the package doc. Today: adding a comment line
   `// measured 2026-01-01: 35 of 40 trials` to any Go file and running
   `task check` is green.
3. A per-file ceiling on comment share ratchets down. `tools/comment-ceilings.txt`
   holds, for every Go and TypeScript file under `internal/`, `cmd/` and
   `client/src` that is not generated, the comment share measured when the
   ledger was written; `check:comments` refuses a file whose share is above its
   ceiling, naming the file and both figures, and reports the files whose share
   has fallen so the ledger can be lowered. A ceiling is lowered by the change
   that lowers the share, never raised. Today: no ledger, and no check sees
   `internal/gateway/server.go` at 64 percent.
4. `CLAUDE.md` carries the rule as its own numbered rule, 10, in one
   paragraph that names SPEC-010 and the step, and its gate paragraph names
   `check:comments` among the steps only `task check` runs. Today: rule 9 is the
   last.
5. Broken on purpose, one per check, recorded in the commit line: a banned word
   on an added comment line reds the step naming the file; a comment block one
   line over the bound reds it; a file with one comment line added above its
   ceiling reds it naming both shares; an ordinary warning comment added to a
   file under its ceiling leaves it silent.
6. Still true afterwards: `task check` whole is green on the tree that carries
   the gate. The findings already in the tree are not cleaned by this ticket:
   the ledger records today's shares, so the gate is green over them, and the
   sweep that lowers them is the next tickets' work, one per package, each
   lowering ceilings in its commit.

## Rules this puts on the system

- A comment line added to code carries no date, no measurement and no account
  of what the code used to do.
- A comment block in code is bounded in length, the package doc excepted.
- A change that adds a comment line to a file does not take that file's
  comment share above its recorded ceiling.
- A ceiling is lowered by the change that lowers the share, and never raised;
  a file with no row is held to a default ceiling until it has one.

## What it touches

1. `tools/check-comments.py`, new: the checker, in the shape of
   `tools/check-new-prose.py` (a base ref, `git diff` for added lines, the
   comment marker `tools/check-comment-wrap.py` already uses), plus a
   whole-tree mode that prints the counts the sweep will lower.
2. `tools/check_comments_test.py`, new: one red case per refusal and per pass,
   in the shape of `tools/check_requirements_chain_test.py`.
3. `tools/comment-ceilings.txt`, new: the ledger, written by the checker from
   today's tree, in the shape of `tools/coverage-thresholds.txt`.
4. `Taskfile.yml`: the `check:comments` task, listed in `check:` after
   `check:new-prose`.
5. `CLAUDE.md`: rule 10 and one name in the gate paragraph.
6. `docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md`, new.
7. `docs/requirements.md`: the rows the sort accepts, after sign-off.

Seven paths, two components: the tools and the records. No code under
`internal/`, `cmd/` or `client/` changes.

## Specifications this moves

New: a comment in code is a warning or a pointer

## What could not be established

- The block-length bound and the exact banned list. The counts above come from
  a first list; the plan sets both, and the self-test carries them.
- Whether the one-line doc sentence on an exported symbol is required or only
  allowed. No lint rule requires it today (`.golangci.yml` names no `revive`
  or `exported` rule), and `check:doc-owner` holds placement, not presence.
  Allowed, not required, until decided.
- Whether `tools/*.py` docstrings and `Taskfile.yml` `desc` blocks are in
  scope. They are prose in the same sense (`tools/check-new-prose.py` opens
  with a thirty-line narrative), but Patrik's ruling of 2026-09-24 names
  production code. Out of scope here, named so the next ticket can decide.
- Whether the sweep that follows runs per package or per arc. Per package
  matches the ledger; per arc matches the record. Not decided here.
- Rule 9 of `CLAUDE.md` does not apply: no tabletop function is designed.
