# Comments are warnings or pointers: the rule, and the gate that holds it

**Ticket:** `docs/superpowers/specs/2026-09-24-comments-are-warnings-or-pointers-design.md`.
**Plan:** `docs/superpowers/plans/2026-09-24-comments-are-warnings-or-pointers.md`,
verified by `verify-ticket` (Passes with gaps); its seven sign-off questions
were answered with the recommendations on 2026-09-24.
**Last commit that changes code:** `8f1f586`. The base is `2e24606`, `main` at the
time, to which `chore/adopt-the-process` had been merged that afternoon so
that this gate's first run would judge only what this branch adds.

## The period, in commits

    git log --oneline 2e24606..8f1f586

    8f1f586 CLAUDE.md rule 10: a comment in code is a warning or a pointer
    a7e8626 check:comments holds SPEC-010 on added lines and on every file's share
    701f27e The comment rule: SPEC-010, the ticket, the plan and nine rows

`git diff --stat 2e24606..8f1f586`: `10 files changed, 2257 insertions(+), 2
deletions(-)`.

The gate: `task check`, whole, over the tree of `8f1f586` less two SPEC-010
edits made after the run, before that commit: green, exit 0, every step,
`check:comments` included with its completion line; the Go mutation gate
reported 14 packages with zero unadjudicated survivors (6 mutants timed out and
are counted as killed without being measured; most packages reused a cached
verdict, nothing in their closure having changed), the TS mutation gate 2882
mutants, 2783 killed, 29 timed out, 70 survivors all adjudicated equivalent.
The tree it ran over was that of `8f1f586` less two SPEC-010 edits made after
the run, edits no step's verdict depends on. Each commit ran the steps that
reach its files before it: `check:requirements-chain` for all three;
`check:comments` itself, the self-test and the QA tests for the second; the
commit gate's nine pre-commit checks on all three.

## Done looks like, answered

1. `[x]` `docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md`
   exists with the five headings and states the rule as the ticket words it: an
   imperative warning, a pointer to a record, or the doc sentence of an
   exported symbol, with history, measurements, arguments and descriptions of
   behaviour named as the report's and the specification's. Its Status read
   "Accepted, and not yet implemented" at `701f27e` and "Accepted. Implemented
   by ..." from `a7e8626`.
2. `[x]` `task check:comments` is a step of `task check`, held by
   `tools/check-comments.py` with `tools/check_comments_test.py`, which was red
   at import before the checker existed (`FileNotFoundError` on the checker's
   path) and green with it. Each refusal the item lists is a case of QA's file
   (`test_qa_each_banned_term_on_an_added_go_line_is_refused` walks the whole
   banned list); the self-test carries `measured` and the date. The banned
   list, the bound of six and the exemptions are the checker's docstring's. The
   item's own observation, a `// measured 2026-01-01: 35 of 40 trials` line
   added to a Go file, is break B1 of `a7e8626`'s message, which records the
   file and the term; the checker names the line as well, which
   `test_an_added_line_with_a_banned_term_is_refused` asserts.
3. `[~]` `tools/comment-ceilings.txt` holds one row per in-scope file, 237 at
   `a7e8626`, each the file's share rounded up to a tenth. Two of the item's
   sentences changed shape: a file above its ceiling is refused when the change
   added a comment line to it and is a notice otherwise
   (`test_a_rise_from_removed_code_alone_is_a_notice_not_a_refusal`), since a
   share also rises when code leaves; and a share that fell is not reported but
   refused past the band until `--write-ledger` records it
   (`test_a_share_fallen_more_than_the_band_under_its_ceiling_is_refused`),
   since a report alone left the ticket's lowering rule unheld. Break B3 reds
   the first, B6 the second, B4 a row raised above the base's, and
   `--write-ledger` never raises a row (a self-test case).
   `internal/gateway/server.go` sits at 64.9 in the ledger, which is the item's
   "no check sees" answered.
4. `[x]` `grep -n '^10\. \*\*' CLAUDE.md` prints one line, and the rule names
   SPEC-010 and the step; the gate paragraph lists `check:comments` among the
   steps only `task check` runs (`grep -c check:comments CLAUDE.md` prints 2).
5. `[~]` Eight breaks and two silent cases, one per check, in a scratch clone
   carrying the gate, each finding line in `a7e8626`'s message: B1 the banned
   term, B2 a block over the bound, B3 a file above its ceiling, B4 a raised
   row, B5 a stale row, B6 a fallen share, B7 a new file above the default, B8
   nothing scanned; P1 a warning and P2 a `docs/` path with a date, both silent
   with the completion line. The item's "one line over the bound" is held by
   the self-test (`test_a_block_of_exactly_the_bound_passes` beside
   `test_a_block_over_the_bound_that_the_change_touches_is_refused`), not by
   B2: in the real tree the seven added lines joined an existing block and a
   fourteen-line block was refused, which is the rule and not the case the item
   names. Five of the ten were run again: B4 needed a base that carries the
   ledger, since `main` has none; B8's first run did not capture its exit code;
   P1 and P2 placed beside an existing block joined it and were refused for
   that, first an eight-line block and then a thirteen-line one, before they
   were placed between two code lines; and B5 was run again after the rename
   fix changed its message.
6. `[x]` `task check` whole was green over the tree of `8f1f586` less two
   SPEC-010 edits made after the run. The findings in the tree are not cleaned:
   `--report` at `a7e8626` counts 2,192 banned lines and 1,576 blocks over the
   bound in 237 files, the plan's figures, and the ledger records the shares as
   they are, so the gate is green over them and every later change that adds a
   comment line is answerable for its file.

## What the rules became

| Rule, as the ticket words it | Became |
|---|---|
| A comment line added to code carries no date, no measurement and no account of what the code used to do | VTT-050, narrowed to what the check holds: none of the banned terms the gate names. The wider sentence is VTT-051, held by a reading |
| A comment block in code is bounded in length, the package doc excepted | VTT-052, "longer than the bound ... when a change adds a line to it", the figure the checker's |
| A change that adds a comment line to a file does not take that file's share above its ceiling | VTT-053 |
| A ceiling is lowered by the change that lowers the share, and never raised; a file with no row is held to a default | VTT-054 (never raised), VTT-055 (the band, which is what makes "lowered by the change" a refusal), VTT-056 (a row names a file that exists), VTT-057 (the default) |
| — | VTT-051, the ruling itself, `**READING — Phase 4b**`; VTT-058, a run that scans nothing, has no base or finds no ledger fails |

Refused by the sort: the presence of SPEC-010 and of rule 10 (held by the
plan's done commands), "`task check` whole is green" (a status), "the ledger
records today's shares" (a one-time event), "the doc sentence is allowed, not
required" (undecided), and "the sweep lowers ceilings one package at a time"
(the next tickets' work).

## QA adjudications

QA derived 48 tests in `tools/check_comments_qa_test.py` from SPEC-010, the
nine rows and the checker's usage text, without reading the checker or the
self-test; every refusal test pins the reason text, and QA reports redding 36
assertions by injection into its own inputs. Three failed on the first run, all
on lines that are not prose. Committed with the task and run by
`check:comments` beside the self-test.

- QA finding 1: behaviour defect — a `//go:` directive carrying a banned word
  was refused; SPEC-010 says a directive is not refused — the checker now
  matches the banned list on nothing of a directive line but a trailing `//
  reason`; QA's test passes unchanged.
- QA finding 2: spec ambiguity — `//nolint:errcheck // measured`; QA read the
  exemption as covering the reason — ruled the other way: the reason is prose
  and is read; SPEC-010 says so, and QA's test was set to the ruling
  (`test_qa_a_nolint_directive_with_a_banned_reason_is_refused`).
- QA finding 3: spec ambiguity — an `[anchor:measured-share-guard]` marker was
  refused for its spelling — the marker is removed before matching, as `docs/`
  paths are; SPEC-010 says so; QA's test passes unchanged.
- QA finding 4: usage-text defect — the docstring named the bound, the default
  ceiling and the band without their values, while SPEC-010 says the docstring
  carries them — the values are in the docstring now, with the share's
  definition, how renames are detected, what `--write-ledger` adds, and that an
  untracked file counts whole.
- QA finding 5: spec gap — VTT-058 named no-scan and no-base but not a missing
  ledger, which the checker also fails on — the row, not yet committed at the
  time, names all three.
- QA finding 6: overtaken — QA's test of the old reading, now
  `test_qa_a_go_slash_star_block_is_read_as_comment`, held the plan's D2 (a Go
  `/*` line is code); the reading review of the gate found that D2 let history
  hide inside a Go block comment, and the gate now reads every line of a `/*
  ... */` block in both languages; QA's test is set to that ruling.
- QA's remaining gaps (the share's definition, block and package-doc
  definitions, the completion line's suffix, rename detection): no change
  beyond the docstring; SPEC-010 defers those to the checker by decision D16.
- Requirements QA cited by section (the `docs/` exemption, only added lines
  read, the scope roots, the notice for a rise without an added comment line,
  the hand-lowered row, `--write-ledger`'s three actions, the completion line's
  content): held by the rows they refine, VTT-050, VTT-052 to VTT-056 and
  VTT-058, which QA's tests cite; no new row.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Plan D2: a `//go:` or `//nolint` directive is not refused. | The directive part is not read; a trailing `// reason` is; an `[anchor:...]` marker is removed before matching. | QA's three failures: the checker read a directive's own text and an anchor's name as prose. The nolint reason was ruled prose, against QA's reading, and QA's test was set to the ruling. |
| Plan D16: the checker's docstring carries the bound, the default and the band. | It named them without their values. | QA could not read them anywhere it was allowed to; the values are in the docstring now, with the share's definition. |
| Plan D14, P1 and P2: a warning and a `docs/` path in a file under its ceiling. | Placed between two code lines. | Beside an existing seven-line block, the added line joins it and the block is refused; that is D3 working, not the silent case. |
| Plan D6/D7: a `git mv` carries its row. | A renamed-away row is stale until `--write-ledger` moves it, and `--write-ledger` refuses to strand a plainly moved file. | The reading review of the gate showed the branch clean after a `git mv` with the ledger unchanged, and `main` red for good once merged, because the merge base then pairs nothing; and that `--write-ledger` after a plain `mv` dropped the old row while the new file, above the default, could get none. |
| Plan D2: a Go comment line starts with `//`. | Every line of a `/* ... */` block counts, in Go as in TypeScript. | The same review put history inside a Go block comment and the gate read nothing; no such block exists in the tree, so the ledger did not move. QA's test of the old reading was set to the new one. |
| The ledger parser takes any float. | It takes one decimal from 0.0 to 100.0 and refuses a path twice. | `nan` in a row passed the raise check and `--write-ledger` kept it; a second decimal printed as its neighbour. |
| Plan Task 6: `CLAUDE.md` alone in the third commit. | With two sentences of SPEC-010. | SPEC-010 said `CLAUDE.md` did not yet carry rule 10 and left the narrowing's holder unnamed; both become true only with rule 10. |
| Plan D14, B2: a new seven-line block in a file under its ceiling. | The seven lines joined the block above the function and a fourteen-line block was refused. | Blank lines do not split a block, so a block placed above a function with a doc block joins it; the one-line-over case is the self-test's. |
| Plan Task 7: the whole gate after the review has settled and the tree is final. | It ran over the tree before two SPEC-010 sentences were made true for the third commit. | The sentences say rule 10 exists and holds the narrowing, which is true only from that commit; no step's verdict reads them. |
| Plan D14, B1 in `internal/artlib/artlib.go`. | In `internal/gateway/server.go`. | The implementer read `artlib.go` as having no headroom for one added line; the ledger shows it has (a share of 69.4 against a ceiling of 69.5). The break holds the same in either file. |
| VTT-058 as the sort worded it. | Names a missing ledger as well. | QA found the checker exits 2 on it and no row said so; the row was uncommitted. |

## What could not be established

- Whether the sweep should run per package or per arc. Carried from the ticket;
  the ledger is per file either way.
- Whether `tools/*.py` docstrings and `Taskfile.yml` `desc` blocks come into
  scope. The new tool's docstring and the step's `desc` obey the rule by hand;
  nothing checks them.
- Whether the one-line doc sentence on an exported symbol is required. Allowed.
- The band of one point, signed off from `internal/gateway/server.go`, trips on
  code-only changes to small files: over the 237 rows at `8f1f586`, taking for
  each file the fewest added non-blank code lines that put its measured share
  more than a point under its ceiling, the median is 8 lines and 93 files trip
  at 5 or fewer. Patrik kept the signed-off band on 2026-09-24; the remedy is
  `--write-ledger` in the same change, and the sweep lowers every ceiling
  anyway.
- Rename detection in the added-line scope is `tools/check-new-prose.py`'s,
  imported unchanged, which does not pass `-M`; with `diff.renames=false` a
  `git mv` makes the whole file count as added. The ledger half passes `-M`.
- An unreadable or non-UTF-8 file in scope is skipped by the measurement; none
  exists today.
- `testdata` directories are in scope; no Go or TypeScript file under one
  exists today.

## What was deliberately left out, and where it went

- The comment already in the tree: the sweep, one ticket per package, each
  lowering `tools/comment-ceilings.txt` with `--write-ledger` in its commit.
  `--report` is written for it.
- A pre-commit or pre-push seat for the step: a later decision, per D11.
- A hatch: none, per D13; a warning that cannot be written without a banned
  term is a reviewed decision under rule 2.

