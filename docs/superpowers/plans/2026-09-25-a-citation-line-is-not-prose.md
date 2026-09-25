# A comment line carrying only requirement ids counts in no comment share — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-25-a-citation-line-is-not-prose-design.md`
**Verified:** 2026-09-25, by `verify-ticket`, an agent that did not write the
ticket, against `8181d47` on `main`. Verdict: **Passes with gaps.** Three of the
ticket's stated observations are inaccurate as written, each established by
command and listed under "What the verification found"; the plan holds the
corrected forms and does not edit the ticket. The gaps travel with this plan.

**Goal, in the ticket's words:** a comment line that carries only requirement
ids counts in no comment share, no added-line count and no block length.

**MapTool (CLAUDE.md rule 9), answered in one line.** No tabletop function is
designed; the ticket says so, and the previous plan
(`docs/superpowers/plans/2026-09-24-comments-are-warnings-or-pointers.md`) found
that MapTool reads nothing a comment says. Nothing to import.

## What the verification found

All at `8181d47`, by command, with the checker's own `measure`, `share_of` and
`ceil1` from `tools/check-comments.py`, and a scratch clone for the gate runs.

- **The figures reproduce.** `internal/identity/identity_test.go` is 53 comment
  lines over 1,347 non-blank, 3.93 percent, `ceil1` 4.0, and the ledger row is
  4.0. `fault_internal_test.go` 28 over 319, 8.78, row 8.8.
  `identity_failure_test.go` 7 over 102, 6.86, row 6.9.
- **The refusal reproduces, and the ticket's quoted finding does not.** In a
  scratch clone on a branch off `main`, `// VTT-042` directly above
  `TestCreateInviteVerifyRoundTrip` and `python3 tools/check-comments.py main`
  exits 1 with `internal/identity/identity_test.go: comment share 4.0 is above
  its ceiling 4.0 and this change added a comment line to it (SPEC-010)`. One
  added line takes the share to 54 over 1,348, 4.006 percent: the gate compares
  the unrounded value against the row and refuses, and the finding prints the
  share to one decimal, `4.0`. The ticket's `4.1` is wrong; the ticket's
  observation (exit 1, the ceiling finding) is right. `// VTT-042 holds this`
  in the same place: the same finding, exit 1.
- **The count reproduces exactly.** A regex for a `//` line carrying nothing
  but ids of the register's tag, run over `scan_tree()`'s 237 files: nine files,
  56 lines, the ticket's per-file numbers to the line (21, 10, 10, 5, 5, 2, 1,
  1, 1). Seven of the 56 carry two ids separated by a space
  (`// VTT-048 VTT-049` in `identity_test.go` and six more in `join_test.go`
  and `server_test.go`); none carries a comma. Seventeen further comment lines in
  scope carry an id beside words (`// VTT-039 control: ...`); those are comment
  lines under the ticket's rule and stay so.
- **`grep -rn 'VTT-' client/src` prints nothing**, and `client/src` holds no
  `.test.ts`; the client's tests live under `client/test/`, outside the comment
  gate's roots and inside the chain gate's `*.test.ts` glob.
- **`--write-ledger` today would change no row**: every row equals `ceil1` of
  its file's share or sits under it, so the only rows the changed checker lowers
  are the nine, computed under D3 below.
- **The ceiling has headroom after the change that it lacks today.** After the
  change `identity_test.go` is 32 over 1,326, 2.413 percent, row 2.5; one added
  worded line is 33 over 1,327, 2.487, under the row. So the ticket's item 2,
  first clause, read on the real tree after the change, does not hold: the
  refusal today rests on 4.0 leaving 0.07 of headroom against 0.074 per line,
  a rounding accident. The fixture holds it (T2 below), where the row is set to
  the share exactly.
- **"Each red on today's checker" is false for the still-refused cases.** A
  test that a worded line is refused is green on today's checker, which refuses
  every `//` line. Those two cases, T2 and T4, guard against over-reach and go
  red under the deliberate break B2, not on today's tree.
- The checker is called from `check:comments` in `Taskfile.yml` and from its
  two test files, and from nowhere else (`grep` over the tree; `.lefthook.yml`
  names no comment step). `measure` is called by `measure_all` only; a doc
  snippet in the sweep-identity plan calls it with two arguments, so a third
  parameter takes a default.
- Both test files and the chain gate are green today: 31 tests, 48 tests,
  `58 rows, 191 test files, 4 specifications`.

## Constraints that bind every task

- **CLAUDE.md rule 2.** `task check` is never weakened. The exemption is one
  named shape, held by tests that go red when it widens; no threshold, band,
  bound or default moves.
- **CLAUDE.md rule 8.** Names, not `file:line`, in the plan, the tests, the
  docstring, SPEC-010 and the commit message.
- **CLAUDE.md rule 10 and SPEC-010.** The tests' own comments are citations and
  one-line pointers; the docstring carries no history or measurement.
- **The ticket's scope.** No file under `internal/`, `cmd/` or `client/src`
  changes, so no mutation adjudication key moves and `check:drift` has nothing
  to compare.
- **SPEC-008.** Only the register's own tag reads as a citation; a fixture's
  other tag is data. Ids come from the dispenser, after sign-off; this plan
  allocates none.
- **SPEC-010, "A run proves it ran".** The completion line names files, added
  comment lines and ledger rows, and two regexes pin its head: `assertClean` in
  `tools/check_comments_test.py` (a prefix match) and `COMPLETION` in
  `tools/check_comments_qa_test.py` (which allows a `;`-led tail after `clean`).
- **VTT-054 and VTT-055.** A row is never raised; a share more than `BAND`
  under its row is refused until `--write-ledger` runs.
- **`tools/check_comments_qa_test.py` is Phase 4a's**, derived from SPEC-010
  alone; the ticket opens it only for a case QA derives. Its fixtures copy two
  named files into the scratch repository, `check-comments.py` and
  `check-new-prose.py`, and hold no register.
- **Commits need a review record** (`review-gate` in `.lefthook.yml`).

## Decisions this plan makes

**D1 — The tag is the register's, read from the `project:` line of
`docs/requirements.md`; with no tag, no line is a citation line and the run
says so.** Forced by: SPEC-008 ("only the register's own tag reads as one"; a
fixture's tag is data), and the QA-fixture constraint. A generic `TAG-NNN`
shape would set aside a Go test's fixture lines that SPEC-008 calls data.
Importing `read_register` from `tools/check-requirements-chain.py` by path, as
`check-new-prose.py` is imported, would need a third file copied into both
test fixtures, and the QA file is not this ticket's to reshape. So the checker
carries its own copy of one regex, the chain checker's `TAG_RE`, and a
`REGISTER` constant with the same value; the docstring points at the chain
checker as the regex's origin. With no register, or a register with no
`project:` line, the checker sets no line aside and appends to its completion
line and to its finding-count line the tail `; no tag in
docs/requirements.md, citation lines counted as comment lines`, joined to the
existing base-ledger tail when both apply. Not exit 2: the thirty-one existing
fixtures have no register and are not this ticket's to rewrite, and a run that
counted every `//` line is what the gate did until now, stated rather than
silent.

**D2 — The shape of a citation line.** Forced by: the ticket ("the register's
tag and nothing but ids") and the seven two-id lines. A `//` line whose text
after the marker is one or more ids `TAG-NNN`, three or more digits (the chain
gate's shape), separated by whitespace, and nothing else. No comma, no
trailing period, no word: any of those makes it a comment line. A line inside a
`/* ... */` block is a block line whatever it says. The same shape applies in a
`.ts` file; a `* VTT-042` line inside a JSDoc block is not a citation line. The
docstring states the shape under its scope paragraph, as the ticket's item 5
asks.

**D3 — A citation line is out of both the numerator and the denominator.**
Forced by: VTT-055 and the ticket's own mechanism. If the line stayed
non-blank, every added citation would lower the share a little, and a sweep
that cites twenty-one tests in one file would push it more than `BAND` under
its row, refuse the change, and lower the ledger by `--write-ledger` for no
comment removed, which is the problem the ticket states, running the other
way. So a citation line neither counts nor is counted against: the share of a
file does not move when one is added or removed. Under this, `--report` prints
`identity_test.go` at 2.4 (32 over 1,326 is 2.413), the ticket's figure, and
`--write-ledger` writes its row at 2.5. The computed rows, old to new, all nine
and no other:

| File | Citation lines | Row today | Row after |
|---|---|---|---|
| `internal/engine/qa_role_test.go` | 2 | 8.2 | 7.4 |
| `internal/engine/role_test.go` | 1 | 9.6 | 7.4 |
| `internal/gateway/authz_test.go` | 5 | 38.5 | 38.2 |
| `internal/gateway/join_test.go` | 10 | 27.5 | 25.7 |
| `internal/gateway/qa_joining_test.go` | 1 | 5.9 | 4.8 |
| `internal/gateway/server_test.go` | 10 | 29.1 | 28.8 |
| `internal/identity/fault_internal_test.go` | 5 | 8.8 | 7.4 |
| `internal/identity/identity_test.go` | 21 | 4.0 | 2.5 |
| `internal/identity/qa_joining_internal_test.go` | 1 | 12.7 | 12.2 |

Five of the nine fall more than `BAND` under their row once the checker
changes (`role_test.go` by 2.28, `join_test.go` 1.83, `identity_test.go` 1.59,
`fault_internal_test.go` 1.48, `qa_joining_test.go` 1.14), so the checker
without the ledger is red on the gate, which is why D10 is one commit. The
`--report` summary's comment-line total falls by 56 from 32,311 and its
non-blank total by 56 from 87,366.

**D4 — A citation line belongs to no block: it neither counts in a block's
length nor ends one, the way a blank line behaves in `measure`.** Forced by:
the ticket's rule ("no block length") and the previous plan's D3, which closed
the insert-a-blank-line evasion; a citation line that ended a block would be
the new way to split a twelve-line block into two of six. So a six-line
how-block followed by a citation line passes, and four how-lines, a citation
line and three more how-lines are one block of seven. A citation line is not
in any block's line list, so adding one under a legacy block over the bound
does not "touch" that block and is not refused, consistent with "no added-line
count". The ticket's gap, answered: a block made only of citation lines is no
block; a block of citations under a how-line is a block of one line.

**D5 — The completion line's head is unchanged; the citation count goes to
`--report`.** Forced by: SPEC-010's "A run proves it ran" names three figures,
and the two regexes that pin them are in files this ticket does not open for
this. `--report` gains a per-file column `cites` and the summary line gains
`, N citation lines set aside`; nothing asserts the report's format. A
preference, named as one: the count could stay unprinted, and the sweep's
reader is better served seeing what was set aside.

**D6 — `measure(lines, ts, cite=None)`; the pattern is built once in `main`
and passed through `measure_all`, `report`, `do_write_ledger` and `gate`.**
Forced by: the two-argument call in the sweep-identity plan's doc snippet, and
the QA test's black-box constraint (no module-level state to reach in). A
module-level compiled pattern read at import would also make the unit-test
fixtures depend on the cwd at import time, which they do not today.

**D7 — The unit-test fixtures gain a register.** Forced by: D1 and the
fixtures' shape. `base()` in `tools/check_comments_test.py` takes `tag="VTT"`
and writes `docs/requirements.md` with a `project:` line before the base commit;
`tag=None` writes none. Existing tests keep their text and gain a register they
never read. The new tests set their ledger rows explicitly from the non-citation
lines rather than through the `share()` helper, which counts every `//` line
and would otherwise be a second copy of the shape under test.

**D8 — What SPEC-010 says, and where.** Forced by: the ticket's item 5 and the
rule's three parts. Under "Files are held by a ceiling", one sentence: a
comment line carrying only requirement ids counts in no comment share, is not
an added comment line and has no place in a block, and the checker's docstring
says what such a line is. Under "Added lines are held by `check:comments`", the
clause that the bound counts no citation line, so the block paragraph and the
ceiling paragraph agree. "What is outside" keeps "A test's `VTT-NNN` citation
is a pointer (SPEC-008)", which stays true. The Status paragraph and the
Requirements line gain the new id once allocated. Nothing in CLAUDE.md moves:
rule 10's sentence about `check:comments` remains true.

**D9 — The ticket's three inaccurate observations, held in corrected form.**
Forced by: the verification's commands, and the skill's rule that a verifier
does not edit the ticket. (a) Item 1's "today" finding prints `4.0`, not `4.1`;
T1's fixture asserts the finding's words and the ceiling, not a share figure.
(b) Item 3's "each red on today's checker" holds for nine of the eleven, T1,
T3, T5, T6, T7, T8, T9, T10 and T11; T2 and T4 are green today and red under
B2 only. (c)
Item 2's first clause is a fixture observation (T2), not a real-tree one; the
plan's real-tree observation after the change is that `// VTT-042` passes with
`0 added comment lines` and two worded lines above two tests in
`identity_test.go` are refused. Sign-off question 1 asks whether the writer
amends the ticket or the report records the corrected forms.

**D10 — One commit for the change; the implementation report follows in its
own commit, per Phase 5.** Forced by: D3's five band refusals (the checker
without the ledger is red), VTT-054 read the other way (the ledger without the
checker passes with notices while stating rows under the measured share, a
ledger that lies), and the chain gate (the row's evidence names tests that must
exist in the same tree). The commit carries the ticket file (untracked today),
this plan, the checker, the unit tests, the QA file if Phase 4a adds to it, the
ledger, SPEC-010 and the register row. The pattern is `8ac4913` then
`8181d47`.

**D11 — Phase 4a's QA dispatch.** Forced by: the change has behaviour, and the
dev-cycle's QA reads the record, not the source. The QA agent is handed the
amended SPEC-010 and the checker's docstring (`python3 tools/check-comments.py`
with no argument prints it, the Python equivalent of `go doc`), and no source,
no unit test and no plan. It derives its cases from the amended paragraphs
alone, adds them to `tools/check_comments_qa_test.py` beside its others, and
reports each finding for adjudication in the implementation report under its
own heading, one line per finding. A QA case that fails is a defect in the
checker or a false sentence in SPEC-010, and the report says which.

**D12 — The deliberate breaks, one per check the change adds, in a scratch
clone.** Forced by: the ticket's item 3 ("red on today's checker") holds only
half the cases, and a test that is green before and after proves nothing about
itself. `git clone --no-hardlinks` of the repository into the scratchpad, `main` at
the base, the working tree's `git diff HEAD` applied there with `git apply`
(every file the change touches is tracked); each break is one edit to `tools/check-comments.py`
there, then `python3 -B tools/check_comments_test.py` (no bytecode cache: a
stale `.pyc` has turned a red proof green in this repository before), the red
test names recorded, then the inverse edit and `git diff --stat` printing
nothing before the next break.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | the citation pattern never matches | the nine that are red today |
| B2 | the pattern matches any `//` line that contains an id | T2, T4 |
| B3 | a citation line is counted as non-blank | T5 |
| B4 | a citation line is appended to its block's line list | T6 |
| B5 | a citation line ends a block as a code line does | T7 |
| B6 | the tag is the literal `VTT` instead of the register's | T8 |
| B7 | the no-tag tail is not printed | T10 |

And the real-tree observations in the same clone, unbroken, on a branch off
`main`: `// VTT-042` above `TestCreateInviteVerifyRoundTrip` in
`internal/identity/identity_test.go`, `python3 -B tools/check-comments.py main`
exits 0 and its completion line says `0 added comment lines`; then
`// VTT-042 holds this` above that test and `// VTT-042 and more` above
`TestTokenNotRecoverableFromDB`, exit 1 with the ceiling finding naming
`identity_test.go` (two lines, because one sits under the 2.5 row's headroom,
per "What the verification found").

**D13 — The gate steps before the commit, in order, on the working tree after
the review has settled** (a gate run that the review still edits under is
wasted; this repository has lost two forty-minute runs that way):

1. `python3 -B tools/check_comments_test.py`: green, T1 to T11 included.
2. `python3 -B tools/check_comments_qa_test.py`: green; the file is unchanged
   unless Phase 4a added to it.
3. `python3 tools/check-comments.py --write-ledger`, then
   `git diff --stat tools/comment-ceilings.txt`: one file, nine rows changed,
   the values of D3's table.
4. `python3 tools/check-comments.py main`: exit 0, `237 files, 0 added comment
   lines, 237 ledger rows; clean`.
5. `task check:comments`: the same three commands as the Taskfile runs them.
6. `task check:requirements-chain`: the new row's evidence resolves to the new
   tests and every citation of the new id resolves to the row.
7. `task check` whole, item 6, launched detached with its own session (a
   background run in this repository dies at thirty-eight minutes otherwise),
   the tree untouched while it runs. `check:new-prose` and `check:drift` have
   nothing in scope; the mutation gates run over Go and TypeScript that did
   not change.
8. Phase 4b's reading review, the review record, `git add` in its own call and
   `git commit` in the next (a hook that blocks a combined call leaves the
   staging half applied). The pre-commit hook runs lint, vet, `test:unit`,
   arch, invariants, doc-owner, staged secrets, typecheck and the review gate.
9. `git push`: the pre-push hook runs tiers 2 and 3, drift and breaking, about
   three minutes, and is not killed under five.

## The sort's candidate, and what the reading found

One candidate, the ticket's rule, for the `requirements` skill after sign-off;
no id is allocated here.

1. **A comment line that carries only requirement ids counts in no comment
   share, no added-line count and no block length.** Observations: B1, B3, B4
   and the real-tree run in D12. The ticket asks for one row; the skill's sort
   may split the sentence into its three parts, each with its own tests, and
   the ticket's "one row" is then the writer's to defend. Evidence, once the
   tests exist: T1, T3, T5, T6 and T11 for the share and the added-line count,
   T6 and T7 for the block.

Refused: "the ledger's nine rows are lowered" (a one-time event, held by
Task 5's command); "`task check` whole is green" (a status); "the docstring
says what a citation line is" (the presence of a sentence, held by Task 4's
command); "the tag is read from the register" (how the rule is held, not a
rule of the system; T8 pins it as the checker's shape).

## Tasks, in dependency order

### Task 1 — The id, after sign-off

Files: `docs/requirements.md`, by the dispenser only. Run `requirement-id
"<the rule, one sentence>"` once per row the sort produces, from the repository
root; the evidence cell starts `**OPEN — no test yet**`.
**Done when:** `task check:requirements-chain` is green and prints one more row
than 58; `git diff docs/requirements.md` shows only appended rows.

### Task 2 — The unit tests, red

File: `tools/check_comments_test.py`. `base()` gains `tag="VTT"` (D7). Eleven
tests, each citing the allocated id on the line above it, under a new section
heading; ledger rows set explicitly.

- T1 `test_a_bare_citation_line_is_not_an_added_comment_line`: base with one
  comment line, row at that share; add `// VTT-042` above a function; clean,
  `0 added comment lines`.
- T2 `test_a_word_beside_the_id_makes_it_a_comment_line`: same base; add
  `// VTT-042 holds this`; refused, `ceiling`, `added a comment line`.
- T3 `test_a_citation_added_to_a_file_above_its_ceiling_is_a_notice`: base
  with one comment line, row at that share; remove a function so the share
  rises; add `// VTT-042`; clean, `notice`.
- T4 `test_a_worded_citation_added_to_a_file_above_its_ceiling_is_refused`:
  as T3 with `// VTT-042 and more`; refused, `ceiling`.
- T5 `test_citation_lines_leave_the_share_where_it_was`: base with one
  comment line among ten code lines, row at that share; add three citation
  lines above three functions; clean.
- T6 `test_a_citation_line_adds_no_length_to_a_block`: a six-line block, then
  `// VTT-042`, then a function; clean.
- T7 `test_a_citation_line_does_not_split_a_block`: four how-lines,
  `// VTT-042`, three how-lines, a function; refused, `block`, `7 lines`.
- T8 `test_the_tag_is_the_registers`: `base(..., tag="ABC")`; `// ABC-001`
  above one function is not a comment line and `// VTT-001` above another is:
  the completion line says `1 added comment lines`.
- T9 `test_two_ids_on_one_line_are_one_citation_line`: `// VTT-048 VTT-049`;
  clean, `0 added comment lines`.
- T10 `test_no_tag_counts_a_citation_line_as_a_comment_and_says_so`:
  `base(..., tag=None)`; add `// VTT-042` to a file at its row; refused,
  `ceiling`, and the output carries `no tag in docs/requirements.md`.
- T11 `test_write_ledger_lowers_a_row_for_citation_lines`: base with two
  citation lines and one comment line, row at the share counting all three;
  `--write-ledger`; the row equals `ceil1` of the share counting one.

**Done when:** `python3 -B tools/check_comments_test.py` reports T1, T3, T5,
T6, T7, T8, T9, T10 and T11 failed (T7 because today's checker counts the
citation line and says `8 lines`) and T2, T4 and the thirty-one existing tests
passed; the failing names are recorded for the report.

### Task 3 — The checker

File: `tools/check-comments.py`. Per D1 to D6: `REGISTER`, a `register_tag()`
reading the `project:` line with the chain checker's regex, a `cite_pattern(tag)`
building D2's shape, `measure(lines, ts, cite=None)` skipping a matching line
outside a `/* */` block as it skips a blank one, the pattern threaded through
`measure_all`, `report`, `do_write_ledger` and `gate`, the no-tag tail, the
`cites` column and the summary count. Docstring: the scope paragraph gains the
shape of a citation line, where the tag comes from and what a run with no tag
does; the refusal list is unchanged.
**Done when:** `python3 -B tools/check_comments_test.py` is green, 42 tests;
`python3 -B tools/check_comments_qa_test.py` is green, 48 tests;
`python3 tools/check-comments.py --report | grep identity_test.go` prints
`2.4%` and `ceiling   4.0` and a `cites` of 21; `python3 tools/check-comments.py
main` on the branch exits 1 with five band findings, each naming
`--write-ledger`.

### Task 4 — SPEC-010

File: `docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md`, per
D8, with the `specification` skill's catches list read against it.
**Done when:** `grep -c 'only requirement ids' <the file>` prints at least 1
under "Files are held by a ceiling"; the Requirements line names the new id;
`task check:requirements-chain` is green.

### Task 5 — The ledger

File: `tools/comment-ceilings.txt`, by `--write-ledger` only.
**Done when:** `git diff --stat tools/comment-ceilings.txt` shows nine rows
changed and `git diff tools/comment-ceilings.txt` shows exactly D3's table;
`python3 tools/check-comments.py main` exits 0 with `237 files, 0 added comment
lines, 237 ledger rows; clean`.

### Task 6 — The register's evidence

File: `docs/requirements.md`, the evidence cell only, by hand, from `OPEN` to
`tools/check_comments_test.py#<test>` entries per the sort.
**Done when:** `task check:requirements-chain` prints every row's evidence
holds; `grep -c 'OPEN' docs/requirements.md` is unchanged from before Task 1.

### Task 7 — Phase 4a, the breaks and QA

Per D11 and D12, in a scratch clone carrying the tree Tasks 1 to 6 produce:
`git clone` reads commits, so the clone is made at `main` and the working
tree's `git diff HEAD` is applied in it; the gate there runs against the
clone's `main`, as the real run will.
**Done when:** D12's table has a recorded red list per break matching the
expected column, the real-tree observations are recorded verbatim, and QA's
cases are green or adjudicated in writing.

### Task 8 — The gate, the review, the commit

Per D13. The commit message names the ticket, the plan, SPEC-010, the id, the
nine rows and the QA outcome, and ends with the attribution lines.
**Done when:** `task check` exited 0 with `check:comments`'s completion line in
its output; the review record matches the committed tree; `git log -1
--stat` lists the files D10 names and no file under `internal/`, `cmd/` or
`client/src`.

## Sign-off questions

1. **The ticket's three inaccuracies (D9).** The writer amends the ticket
   before work starts (`4.1` to `4.0`; "each red on today's checker" narrowed
   to the nine; item 2's first clause marked as a fixture observation), or
   the plan proceeds as written and the implementation report records the
   corrected forms against the ticket's items. Which?
2. **Denominator (D3).** A citation line is out of both numerator and
   denominator, so the share does not move when one is added. The alternative,
   numerator only, prints the same `2.4` today and lowers the ledger for every
   sweep that adds citations. Agreed?
3. **Blocks (D4).** A citation line is transparent to blocks, as a blank line
   is: it does not split a long block. The alternative, ending the block, opens
   the evasion the previous plan closed for blank lines. Agreed?
4. **No tag (D1).** A run with no register, or one with no `project:` line,
   counts every `//` line and says so in its completion line, rather than
   exiting 2. Agreed?
5. **`--report` (D5).** The `cites` column and the summary count are a
   preference. Keep, or leave the report's format alone?
6. **The sort (candidate 1).** The ticket asks for one row; the `requirements`
   skill may split it three ways. Does the writer's "one row" stand?

## Gaps carried from the verification

- **A finding prints a rounded share against an unrounded comparison.** `4.0
  is above its ceiling 4.0` is what the gate says today when 4.006 is refused
  against 4.0, and `2.5 is above its ceiling 2.5` is what it will say. Outside
  this ticket; a candidate for a ticket of its own (print two decimals, or
  "rounds to").
- **`.ts` citation lines are untested on the real tree**, since none exists
  and `client/src` holds no test file; D2 applies the shape there by symmetry
  and T-cases are Go. A `.ts` case would be Phase 4a's to derive if SPEC-010's
  sentence is read as language-neutral, which it is.
- **The chain gate's `*_test.py` glob** reads `tools/check_comments_test.py`,
  so the fixture strings `// VTT-042` and `// VTT-048 VTT-049` in the new tests
  are citations of VTT-042, VTT-048 and VTT-049, which exist. A fixture id
  that no row defines (`VTT-001` exists; `VTT-999` does not) would fail the
  chain gate; T-cases use ids the register holds, and T8's `ABC-001` carries
  another tag by SPEC-008's own rule.
