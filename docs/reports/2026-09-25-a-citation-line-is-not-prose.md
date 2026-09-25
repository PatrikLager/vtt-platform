# A comment line carrying only requirement ids counts in no comment share: the change

**Ticket:** `docs/superpowers/specs/2026-09-25-a-citation-line-is-not-prose-design.md`,
amended before work started on the three points its verification found.
**Plan:** `docs/superpowers/plans/2026-09-25-a-citation-line-is-not-prose.md`,
verified by `verify-ticket` (Passes with gaps); its six sign-off questions were
answered with the recommendations on 2026-09-25.
**Last commit that changes code:** `1d139e9`, on `8181d47`, `main` at the time.
Every code reference below is to that tree.

## The period, in commits

    git log --oneline 8181d47..1d139e9

    1d139e9 check:comments sets a bare citation line aside: VTT-059

`git diff --stat 8181d47..1d139e9`: 8 files changed, 940 insertions(+), 42 deletions(-).

The gate: `task check`, whole, over the tree of `1d139e9` before it was
committed: exit 0, every step printing its own verdict, `check:comments`
ending `clean`, `check:requirements-chain` reading 59 rows, `check:mutation`
reporting fourteen packages with zero unadjudicated survivors. Before it, in
the plan's order (D13): both test files, `--write-ledger` and its diff, the
gate to its completion line, `check:comments`, `check:requirements-chain`;
then the pre-commit hook's nine checks.

## Done looks like, answered

1. `[x]` In a scratch clone carrying the tree, `// VTT-042` above
   `TestCreateInviteVerifyRoundTrip` in `internal/identity/identity_test.go`
   and `python3 tools/check-comments.py main` exited 0 with
   `237 files, 0 added comment lines, 237 ledger rows; clean`; on `8181d47`
   the same run exits 1 with `comment share 4.0 is above its ceiling 4.0 and
   this change added a comment line to it`.
2. `[x]` Still refused: `// VTT-042 holds this` above that test and
   `// VTT-042 and more` above `TestTokenNotRecoverableFromDB`, in the same
   clone, exit 1 with `comment share 2.6 is above its ceiling 2.5 and this
   change added a comment line to it`; in the fixture whose row is the
   rounded-up share, one such line is refused
   (`test_a_word_beside_the_id_makes_it_a_comment_line`). A bare citation
   added to a file above its ceiling is a notice
   (`test_a_citation_added_to_a_file_above_its_ceiling_is_a_notice`), a worded
   one is refused
   (`test_a_worded_citation_added_to_a_file_above_its_ceiling_is_refused`).
3. `[x]` Eleven cases in `tools/check_comments_test.py`, named under
   "The tests" below: nine red on `8181d47`'s checker, the two still-refused
   cases green there and red under break B2. `task check:comments` runs the
   file.
4. `[x]` `python3 tools/check-comments.py --report | grep identity_test.go`
   prints `2.4%`, `ceiling   2.5` and `cites  21`; `git diff 8181d47 --
   tools/comment-ceilings.txt` shows nine rows, the ones the problem names,
   each lowered; `task check:comments` ends `clean`.
5. `[x]` `grep -c 'only requirement ids'` over
   `docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md` prints 1,
   the sentence under "Files are held by a ceiling"; the docstring's scope
   paragraph says the shape, the tag's source and the no-register run
   (`python3 tools/check-comments.py` prints it).
6. `[x]` `task check` whole was green over `1d139e9`'s tree (above).

## What the rules became

| Rule, as the ticket words it | Became |
|---|---|
| A comment line that carries only requirement ids counts in no comment share, no added-line count and no block length | VTT-059, one row, as signed off; evidence the six unit tests that hold the share, the added-line count and the block |

Refused by the sort: "the ledger's nine rows are lowered" (a one-time event),
"`task check` whole is green" (a status), "the docstring says what a citation
line is" (the presence of a sentence), "the tag is read from the register"
(how the rule is held; `test_the_tag_is_the_registers` pins it as the
checker's shape).

## The tests

Eleven in `tools/check_comments_test.py`, each preceded by `# VTT-059`:

- `test_a_bare_citation_line_is_not_an_added_comment_line`: a file at its
  row gains `// VTT-042`; clean, `0 added comment lines`.
- `test_a_word_beside_the_id_makes_it_a_comment_line`: `// VTT-042 holds
  this` in the same place; refused at the ceiling.
- `test_a_citation_added_to_a_file_above_its_ceiling_is_a_notice`.
- `test_a_worded_citation_added_to_a_file_above_its_ceiling_is_refused`.
- `test_citation_lines_leave_the_share_where_it_was`: three citations added
  to a file at its row; clean, which a citation counted as non-blank would
  push under the band.
- `test_a_citation_line_adds_no_length_to_a_block`: six lines, a citation, a
  function; clean.
- `test_a_citation_line_does_not_split_a_block`: four lines, a citation,
  three lines; refused as a block of seven.
- `test_the_tag_is_the_registers`: under `project: ABC`, `// ABC-001` is set
  aside in a file at a row of 0.0 and `// VTT-001` is counted in another.
- `test_two_ids_on_one_line_are_one_citation_line`.
- `test_no_tag_counts_a_citation_line_as_a_comment_and_says_so`: no
  register; refused at the ceiling, the output naming
  `docs/requirements.md`.
- `test_write_ledger_lowers_a_row_for_citation_lines`: two citations and one
  comment, the row at the share counting all three; `--write-ledger` writes
  the share counting one.


The breaks, one edit each to `tools/check-comments.py` in a scratch clone
carrying the change, the tests that went red:

| Break | Red |
|---|---|
| B1 the pattern never matches | the eight that pass only with the shape (all but the two worded-line refusals and the no-tag case) |
| B2 any `//` line containing an id is set aside | the two worded-line refusals |
| B3 a citation line counted as non-blank | the share case, the block-length case and the ledger case |
| B4 a citation line appended to its block | both block cases |
| B5 a citation line ends a block as code does | the split case |
| B6 the tag is the literal `VTT` | the register's-tag case |
| B7 the no-tag tail not printed | the no-tag case |

Three counts differ from plan D12's expectation. B1 redded eight, not nine:
the no-tag case stays green with the pattern dead, since its refusal and its
tail do not depend on the pattern. B3 redded three where the plan named one:
a citation counted as non-blank lowers the share in every case, which reds
the two whose added citations take the share more than the band under its
row (the share case and the block-length case) and the ledger case, which
asserts the written value; the other cases at their row stay inside the
band. B4 redded both block cases where the plan named one: a citation
appended to the block makes the split case's block eight lines, and the test
asserts seven.

## QA adjudications

QA derived eighteen cases from SPEC-010 and the checker's docstring alone;
sixteen passed on first run and two failed; fifteen of the sixteen were
shown to fail under a mutated fixture, and the word-beside-the-id case was
not injected.

- `/* VTT-042 */` in Go counted as a comment line, QA expected a citation line:
  spec ambiguity. The record said "a comment line carrying only requirement
  ids" without a shape; the plan's D2 had ruled only a `//` line. SPEC-010 and
  the docstring now say so, and QA's case pins the ruling
  (`test_qa_a_go_slash_star_line_carrying_only_ids_is_a_comment_line`).
- ` * VTT-042` inside a TypeScript `/** */` block counted as a block line, QA
  expected a citation line: the same ambiguity, the same ruling
  (`test_qa_a_star_line_inside_a_typescript_block_carrying_only_ids_is_a_block_line`).
- A comma between two ids makes a comment line: QA's strict reading matched
  the tool's; the record now says "spaces between".
- "The completion line says so" left the tail's wording unstated: left so;
  QA's cases hold that a tail exists after `clean` and mentions citations or
  the register; the unit test `test_no_tag_counts_a_citation_line_as_a_comment_and_says_so`
  pins that it names `docs/requirements.md`.
- Whether an id must resolve to a row: the tag alone; the record now says
  resolution is the chain gate's.
- Not derived by QA and not held: a lowercase tag, and the first run of the
  gate over a ledger written while citation lines still counted, which is
  what the ledger change in this commit is.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Plan T8: one file, `// ABC-001` set aside and `// VTT-001` counted, the completion line saying `1 added comment lines`. | Two files, the `ABC-001` file at a row of 0.0. | With one file, a checker whose tag were the literal `VTT` would count the other line instead and print the same figure; break B6 needs a file that is refused when the wrong line counts. Found while planning the breaks. |
| The ticket's rule, the row as dispensed and SPEC-010's first draft: "A comment line that carries only requirement ids", with no shape; plan D2 had the `//` shape all along. | The row reads "A `//` comment line that carries only requirement ids of the register's tag", SPEC-010 states the shape, and the ticket's line keeps its wording. | QA's adjudication narrowed the shape and the reading review found the row broader than the code, with two QA cases citing VTT-059 while asserting the opposite of its broad reading; a register that states a rule the code does not hold is the failure the `requirements` skill names. The row was uncommitted, so its text moved; a ticket is not edited after the fact. |
| Plan D12: B1 reds "the nine that are red today"; B3 reds T5; B4 reds T6. | B1 redded eight, B3 three, B4 two. | The no-tag case runs with no pattern to kill; a citation counted as non-blank takes two cases under the band and changes the ledger case's written value; a citation appended to the block lengthens the split case's block to eight. |
| Plan D6: `measure` gains a `cite` parameter, and the default keeps the two-argument call in the sweep-identity plan's snippet working. | It also returns a fourth element, the citation line numbers, for `--report`'s column; the parameter's default holds, the fourth value does not: that snippet unpacks three and now raises `ValueError`. | The count of set-aside lines has to come from the same pass that sets them aside, or a second pass would copy the shape; the snippet is a landed plan's documentation of a one-time measurement, and no gate runs it (`grep -rln sweep-identity` outside `docs/` finds nothing). |

## What could not be established

- A finding prints a rounded share against an unrounded comparison (`4.0 is
  above its ceiling 4.0`; plan gap). Outside this ticket; a ticket of its own.
- No `.ts` file in scope carries a citation line (`grep -rn 'VTT-' client/src`
  prints nothing), and `client/src` holds no test file; the shape is held
  there by one QA case only
  (`test_qa_a_typescript_slash_line_carrying_only_ids_is_a_citation_line`).
- A lowercase tag and the first run over a ledger written while citation
  lines still counted (this commit's own ledger change) were not derived by QA.
- `//VTT-042`, with no space after the marker, is set aside, and the module
  docstring does not say so; a tab between ids or a comma makes a comment
  line, which it does say, and a non-ASCII digit does too, which only
  `cite_pattern`'s own docstring says.
- The register is read from the working tree, not the base: a change that
  removes the `project:` line only makes the gate stricter, and the tail
  says so; a change to the tag is the chain gate's to refuse.

## What was deliberately left out, and where it went

- Rows for the identity rules the sweep report listed as candidates (D6 of
  the sweep plan): the next ticket, which this change unblocks.
- The rounded-share finding: its own ticket, above.
- No file under `internal/`, `cmd/` or `client/src` changed; no mutation key
  moved.

## The sort

One candidate, accepted as one row with the observations named above; the
four refusals are listed under "What the rules became".
