# A comment-gate finding prints the share it compared — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-25-a-finding-prints-the-share-it-compared-design.md`
**Verified:** 2026-09-25, by `verify-ticket`, an agent that did not write the
ticket, against `b9afde3` on `main`. Verdict: **Passes with gaps.** Every path
resolves and every arm's "today" output reproduces verbatim; one figure in the
problem statement is off by a double's representation, and the ticket's own open
question (whether two decimals are enough) is answered by measurement: they are
not. Both are under "What the verification found" and travel into this plan,
which does not edit the ticket.

**Goal, in the ticket's words:** a finding of `check:comments` prints the share
it compared, not a rounding of it that reads false against the ceiling beside it.

**MapTool (CLAUDE.md rule 9), answered in one line.** No tabletop function is
designed; the ticket says so. Nothing to look at.

## What the verification found

All at `b9afde3`, by command. The fixtures were built the way `base()` and
`change()` in `tools/check_comments_test.py` build theirs — a scratch git
repository holding copies of `tools/check-comments.py` and
`tools/check-new-prose.py`, a `main` commit with the base file, a ledger row and
a `docs/requirements.md` carrying `project: VTT`, then the change on a branch —
and `python3 -B tools/check-comments.py main` run there. A Go fixture with `c`
comment lines of `n` non-blank is one comment line above each of `c` code lines
and then plain code lines, so no block is longer than one line and only the arm
under test fires.

- **Every path and symbol resolves.** The seven files the ticket names exist;
  `gate` exists in the checker and holds the three comparisons the ticket quotes
  (`share > ceiling + EPS`, `share < ceiling - BAND - EPS`, `share > DEFAULT +
  EPS`) and the four `%.1f` sites: the ceiling finding, the notice, the band
  finding and the default finding. `--write-ledger` and `--report` are `main`'s
  two flags. Rows VTT-053, VTT-055 and VTT-057 exist with the wording the ticket
  assumes. The previous plan's "What the verification found" exists and holds
  the 54-over-1,348 case.
- **The ceiling arm, today** (base 100 of 503, row 20.0, the change adds one
  comment line; 101 of 504 is 20.0397): exit 1,
  `check:comments: internal/a/a.go: comment share 20.0 is above its ceiling 20.0
  and this change added a comment line to it (SPEC-010)`.
- **The band arm, today** (base 101 of 508, row 20.0, the change removes six
  comment lines; 95 of 502 is 18.9243): exit 1, `comment share 18.9 has fallen
  more than 1.0 under its ceiling 20.0; record it: python3
  tools/check-comments.py --write-ledger (SPEC-010)`.
- **The default arm, today** (a new file of 189 comment lines over 755, no row;
  25.0331): exit 1, `internal/a/new.go: comment share 25.0 with no row in
  tools/comment-ceilings.txt is above the default ceiling 25.0 (SPEC-010)`,
  with `189 added comment lines`.
- **The notice, today** (base 101 of 505, row 20.0, the change removes one code
  line; 101 of 504): exit 0, `check:comments: notice: internal/a/a.go is at 20.0
  above its ceiling 20.0 with no comment line added; the next change that adds
  one brings it under`, then the clean completion line.
- **A copy of the checker with those four sites at `%.2f`** prints, on the same
  four fixtures, `20.04`, `18.92`, `25.03` and `is at 20.04`: the ticket's
  items 1 and 2 word for word.
- **The ticket's `18.95` is off by a double.** 379 of 2,000 is exactly 18.95 and
  prints `18.9` today (`'%.1f' % 18.95` is `18.9`; the double is
  18.94999999999999929), and `18.95` at two decimals. 95 of 501 is 18.9621 and
  prints `19.0 has fallen more than 1.0 under its ceiling 20.0` today, the
  false-reading sentence the ticket means. The class is real; the figure that
  names it is wrong. Carried as gap G1.
- **Two decimals do not close the class the ticket's second open bullet names.**
  53 of 279 is 18.9964, compared with 19.0 and refused; at two decimals it prints
  `comment share 19.00 has fallen more than 1.0 under its ceiling 20.0`, which
  reads false. 33 of 824 is 4.00485 against a row of 4.0; at two decimals,
  `comment share 4.00 is above its ceiling 4.0`. Over the real tree today, with
  the checker's own `measure` and the ledger's rows: the first crossing of a row
  by added comment lines prints equal at two decimals for 4 of 237 files
  (`client/src/wire.ts`, `internal/gateway/project.go`,
  `internal/gateway/server_test.go`, `internal/rules/expr_test.go`), the first
  fall through the band by removed comment lines for 5, and by added code lines
  for 11; at one decimal the same three counts are 42, 59 and 114. So a fixed
  two decimals is the same defect one order of magnitude rarer. The largest file
  in scope has 2,567 non-blank lines. This forces D1 and is gap G2.
- **Item 4 holds today**, held by tests that exist: the raised-row finding's
  `0.1` and `0.0` by `test_a_ledger_row_raised_above_the_base_is_refused`,
  `--write-ledger`'s tenths by `test_write_ledger_never_raises_a_row` and the
  `VALUE` pattern, `--report`'s one decimal by `report`'s `%5.1f`. The gate on
  the tree exits 0 with `237 files, 0 added comment lines, 237 ledger rows;
  clean`; `git diff --stat tools/comment-ceilings.txt` prints nothing;
  `--report` ends `237 files, 32255 comment lines of 87310 (36%), 2163 banned
  lines, 1534 blocks over 6, 102 citation lines set aside`.
- **Item 5 fails today**: the docstring says `in percent to one decimal`.
- **Item 3's tests do not exist**; 42 unit tests and 66 QA tests are green
  today, and none asserts a two-decimal figure, so each of the four cases is red
  on today's checker, as the item says.
- **The scope holds.** The checker is run by `check:comments` in `Taskfile.yml`
  and by the two test files, and by nothing else; `.lefthook.yml` names no
  comment step. `share_of` is called from `report`, `do_write_ledger` and `gate`,
  and the rounding under repair is only in `gate`'s four strings. QA's
  `CEIL_WHY`, `BAND_WHY` and `RAISE_WHY` match a figure with `[\d.]+`, which
  takes any count of decimals, and `DEFAULT_WHY` carries no figure: none needs
  widening. In the unit tests, `test_a_comment_line_added_above_the_ceiling_is_refused`
  asserts the ceiling `{top:.1f}`, `test_a_new_file_above_the_default_ceiling_with_no_row_is_refused`
  asserts `25.0`, and both figures stay tenths under D3, so both stay green.
- **Item 4 and the first open bullet pull against each other.** Item 4 lists
  "`--report` prints one decimal" among what is still true afterwards, and the
  open bullet leaves `--report` to the plan. The plan follows the done item
  (D4) and puts the bullet to sign-off.
- **No decision is overturned.** SPEC-010 names what the checker refuses and
  leaves the banned list, bound, default and band to the docstring; it says
  nothing of the precision a finding prints. The one-decimal sentence is the
  docstring's own and the ticket names it. VTT-053, VTT-055 and VTT-057 keep
  their wording. `Specifications this moves: None.` holds on the reading too:
  no refusal changes, only what a refusal prints.

## Constraints that bind every task

- **CLAUDE.md rule 2.** `EPS`, `BAND`, `DEFAULT`, `BOUND` and the three
  comparisons do not move; only what prints after a comparison changes.
- **CLAUDE.md rule 8.** Names, not `file:line`, in this plan, the tests, the
  docstring, the report and the commit message.
- **CLAUDE.md rule 10 and SPEC-010.** A test's comment is a citation or a
  one-line pointer; the docstring carries no history or measurement. The counts
  above belong to this plan and the report, never to the checker.
- **The ticket's scope.** No file under `internal/`, `cmd/` or `client/src`
  changes, so no mutation adjudication key moves and `check:drift` has nothing
  to compare. `tools/comment-ceilings.txt` does not change.
- **SPEC-008.** No id is allocated by this plan; at sign-off (question 4) the
  writer chose one row, dispensed after sign-off, which the new tests cite.
  Fixture comment lines carry no `VTT-` id, so the chain gate reads nothing
  new from them.
- **SPEC-010, "A run proves it ran".** The completion line is untouched;
  `assertClean` and `COMPLETION` keep matching.
- **`tools/check_comments_qa_test.py` is Phase 4a's.** Opened only for a case
  QA derives; its regexes are not edited by this plan.
- **Commits need a review record** (`review-gate` in `.lefthook.yml`), and the
  ledger's self-tests are seconds: run them before the gate, not after.

## Decisions this plan makes

**D1 — A finding prints the share to the fewest decimals, two at least, that
put it on the side of the value it was compared with that the comparison
found.** Forced by: the measurement above (fixed two decimals leaves 4, 5 and
11 files on today's tree whose finding reads false again, and the ticket's
second open bullet asks for exactly this choice), and by the ticket's item 2,
whose strings `18.92` and `20.04` fix the floor at two: at one decimal `18.9`
already separates from `19.0` and would print, and a share printed as a tenth
looks like a ledger value. The helper is `shown(share, bound, above)` in
`tools/check-comments.py`: for `d` from 2 upward, `s = "%.*f" % (d, share)`;
return `s` when `float(s) > bound` (or `< bound` when `above` is false). It ends
by `d` of 10 because the gate only calls it after a comparison that held by
more than `EPS`. The ceiling finding and the notice call it with the row and
`above`; the band finding with `ceiling - BAND`, the value the comparison used,
and not above; the default finding with `DEFAULT` and above. On the ticket's
four fixtures it prints the ticket's strings; on 53 of 279 it prints `18.996`
and on 33 of 824 `4.005`. The alternatives, named: a fixed `%.2f` (eight fewer
lines, the residual class above); "rounds to 20.0 and is above its ceiling
20.0" (reads true and tells the reader nothing that decides). Sign-off
question 1.

**D2 — The band finding does not print the distance fallen.** A preference,
named as one: once `18.92` separates from `19.0` the sentence reads true on
its own, the ticket's item 2 string has no distance, and a clause added after
it is words the reader subtracts for themselves anyway. Sign-off question 5.

**D3 — The ceiling, the band and the default keep `%.1f`.** Forced by: they
are tenths by construction (`VALUE` admits one decimal to a row, `ceil1`
writes one, `BAND` and `DEFAULT` are constants), and the ticket's item 4 says
the raised-row finding prints the ledger's values as they stand. So the
raised-row finding, the new-row-above-default finding, the stale-row finding
and `do_write_ledger`'s stranded message are untouched; only the four sites in
`gate` that print `share` change.

**D4 — `--report` keeps one decimal.** Forced by: item 4 names it among what
is still true afterwards, and a done item is the writer's; the report is a
column read against a tenths ledger and states no verdict. Sign-off question 3
carries the open bullet.

**D5 — The notice changes with the findings.** Forced by: item 2 names it
(`is at 20.04 above its ceiling 20.0`). Same helper, same arguments as the
ceiling finding.

**D6 — The docstring's share sentence.** Forced by: item 5. The sentence "A
file's share is its comment lines over its non-blank lines, in percent to one
decimal" becomes: the share is comment lines over non-blank lines, in percent,
compared unrounded; a ledger row holds a share as a tenth, rounded up; a
finding or notice prints the share it compared to the fewest decimals, two at
least, that keep it on the side of the value it was compared with that the
comparison found. Item 5 says "to two decimals"; under D1 the docstring says
"two at least", and sign-off question 1 covers the difference. The ledger's
"with one decimal" sentence in the exit-2 paragraph stays.

**D7 — The fixtures are generated, not hand-written.** Forced by: the counts
(504, 502, 755, 279, 824 non-blank lines) and the bound: a hand-written
fixture of that size is unreadable and a block over six lines would fire the
wrong arm. `tools/check_comments_test.py` gains `go_file(comments, nonblank)`:
`package a`, a blank line, then for each comment line `// Keep N.` above `var
cN = N`, then `var pN = N` lines to the count; it asserts `nonblank - 1 - 2 *
comments >= 0`. Rows are set explicitly (`20.0`, `0.0`, `4.0`), never through
`share()`.

**D8 — Six tests, one per site and two for the precision loop.** Forced by:
item 3 (one case per arm in items 1 and 2) and D1, which adds a check of its
own that the four arm cases cannot see (they pass on a fixed `%.2f`). Each is
named below; each asserts the finding's full sentence as one needle.

**D9 — One row, chosen at sign-off; the plan had proposed evidence cells
only.** As planned: "This ticket adds no rule" and the precedent the register
sets — the completion line's content has no row of its own and is held under
VTT-058 by `test_qa_completion_line_names_files_added_comment_lines_and_rows`
— put the six tests under VTT-053, VTT-055 and VTT-057. At sign-off (question
4) the writer chose the alternative the plan named: one row, "a finding or
notice that compares a share with a ceiling prints the share on the side of
that ceiling its verdict names", breakable by B5 and held by the new tests,
dispensed after sign-off and cited by them; VTT-053, VTT-055 and VTT-057 are
unchanged. Sign-off answers: 1 separating precision, 2 the figure corrected in
the ticket, 3 `--report` unchanged, 4 one row, 5 no distance printed.

**D10 — The deliberate breaks, one per check, in a scratch clone.** Forced by:
Phase 4a's rule and the shape of D8: a test green before and after proves
nothing about itself, and T5 and T6 must be shown to fail on a fixed `%.2f`
and not on the four arm sites. `git clone --no-hardlinks` into the scratchpad,
`main` at the base, the working tree's `git diff HEAD` applied with `git
apply`; each break is one edit to `tools/check-comments.py` in the clone, then
`python3 -B tools/check_comments_test.py` there (no bytecode cache), the red
names recorded, the inverse edit, `git diff --stat` printing nothing before the
next.

| # | Break, one edit in the clone | Expected red |
|---|---|---|
| B1 | the ceiling finding's site back to `%.1f` | T1, T6 |
| B2 | the band finding's site back to `%.1f` | T2, T5 |
| B3 | the default finding's site back to `%.1f` | T3 |
| B4 | the notice's site back to `%.1f` | T4 |
| B5 | `shown` returns `"%.2f" % share` unconditionally | T5, T6 only |
| B6 | `shown` starts at one decimal | T2 only (`18.9` separates; `20.0` does not) |

**D11 — Phase 4a's QA dispatch, from the docstring.** Forced by: the change
has behaviour (what prints), and the cycle's QA reads the record, not the
source. The QA agent receives `python3 tools/check-comments.py` with no
argument (the docstring, the Python equivalent of `go doc`), SPEC-010, and
rows VTT-053, VTT-055 and VTT-057 — no source, no unit test, no plan. If it
derives a case, it adds it beside its others in
`tools/check_comments_qa_test.py`; its regexes already admit any count of
decimals, and widening one is its call, not this plan's. Each finding goes to
the report under its own heading, one line each; a failing QA case is a defect
in the checker or a false sentence in the docstring, and the report says which.

**D12 — One commit for the change, the report in its own commit.** Forced by:
the ticket ("One component, one commit") and Phase 5's pattern (`1d139e9` then
`0dd0e65`). The commit carries the ticket (untracked today), this plan, the
checker, the unit tests, the QA file if Phase 4a added to it, and the new
row with its evidence cell (D9). No ledger row, no specification, no file under `internal/`,
`cmd/` or `client/src`.

**D13 — The gate steps before the commit, in order, after the review has
settled** (a run the review still edits under is wasted, twice over in this
repository):

1. `python3 -B tools/check_comments_test.py`: green, 48 tests.
2. `python3 -B tools/check_comments_qa_test.py`: green; 66 plus whatever
   Phase 4a added.
3. `python3 tools/check-comments.py main`: exit 0, `237 files, 0 added comment
   lines, 237 ledger rows; clean`.
4. `git diff --stat tools/comment-ceilings.txt`: nothing.
5. `python3 tools/check-comments.py --report | tail -1`: the summary line
   above, unchanged, since no share moves.
6. `task check:comments`: the same three commands as the Taskfile runs them.
7. `task check:requirements-chain`: green; the new row's evidence resolves
   to the new tests (D9).
8. `task check` whole, item 6, launched detached in its own session (a
   background run in this repository dies at thirty-eight minutes otherwise),
   the tree untouched while it runs. The mutation gates run over Go and
   TypeScript that did not change.
9. Phase 4b's reading review, the review record, `git add` in its own call and
   `git commit` in the next.
10. `git push`: the pre-push hook runs about three minutes and is not killed
    under five.

## The sort

The ticket states no rule and says so. One candidate the reading found, for
the writer at sign-off (D9, question 4): a finding's printed share lies on the
side of the compared value that the comparison found. Observations: B5 and the
six tests. Refused as rows: "the docstring says two decimals" (the presence of
a sentence, held by Task 2's command); "`--report` prints one decimal" (item 4,
a status held by `report`'s format string).

## Tasks, in dependency order

### Task 1 — The unit tests, red

File: `tools/check_comments_test.py`. `go_file` per D7, then six tests under a
new section heading, each citing its row on the line above, rows explicit.

- T1 `test_a_ceiling_finding_prints_the_share_it_compared` (VTT-053): base
  `go_file(100, 503)`, row 20.0; change `go_file(101, 504)`; refused, needle
  `comment share 20.04 is above its ceiling 20.0 and this change added a
  comment line to it`.
- T2 `test_a_band_finding_prints_the_share_it_compared` (VTT-055): base
  `go_file(101, 508)`, row 20.0; change `go_file(95, 502)`; refused, needle
  `comment share 18.92 has fallen more than 1.0 under its ceiling 20.0`.
- T3 `test_a_default_finding_prints_the_share_it_compared` (VTT-057): base
  `internal/a/a.go` at `go_file(0, 12)`, row 0.0; write `internal/a/new.go` at
  `go_file(189, 755)`; refused, needle `comment share 25.03 with no row in
  tools/comment-ceilings.txt is above the default ceiling 25.0`.
- T4 `test_a_notice_prints_the_share_it_compared` (VTT-053): base
  `go_file(101, 505)`, row 20.0; change `go_file(101, 504)`; clean, and the
  output holds `is at 20.04 above its ceiling 20.0`.
- T5 `test_a_band_finding_prints_past_two_decimals_when_two_do_not_separate`
  (VTT-055): base `go_file(59, 285)`, row 20.0; change `go_file(53, 279)`;
  refused, needle `comment share 18.996 has fallen more than 1.0 under its
  ceiling 20.0`.
- T6 `test_a_ceiling_finding_prints_past_two_decimals_when_two_do_not_separate`
  (VTT-053): base `go_file(32, 823)`, row 4.0; change `go_file(33, 824)`;
  refused, needle `comment share 4.005 is above its ceiling 4.0`.

**Done when:** `python3 -B tools/check_comments_test.py` reports exactly T1 to
T6 failed and the 42 existing tests passed; the six failing names and each
failure's printed finding are recorded for the report.

### Task 2 — The checker

File: `tools/check-comments.py`. `shown` per D1; the four sites in `gate` per
D1, D3 and D5; the docstring per D6. Nothing else in the file moves.
**Done when:** `python3 -B tools/check_comments_test.py` is green, 48 tests;
`python3 -B tools/check_comments_qa_test.py` is green, 66 tests; `python3
tools/check-comments.py main` exits 0 with the completion line of D13 step 3;
`git diff --stat tools/comment-ceilings.txt` prints nothing; `python3
tools/check-comments.py | grep -c 'two at least'` prints 1 and `grep -c 'to
one decimal' tools/check-comments.py` prints 0.

### Task 3 — The register's evidence

File: `docs/requirements.md`, per D9 as signed off: one row by the
dispenser, its evidence cell by hand.
**Done when:** `task check:requirements-chain` is green; `git diff
docs/requirements.md` shows one appended row and no other change.

### Task 4 — Phase 4a: the breaks and QA

Per D10 and D11, in a scratch clone carrying the tree Tasks 1 to 3 produce.
**Done when:** D10's table has a recorded red list per break matching the
expected column, six times over with an empty `git diff --stat` between; QA's
cases, if any, are green in the tree or adjudicated in writing for the report.

### Task 5 — The gate, the review, the commit

Per D13. The commit message names the ticket, the plan, the new row and the
QA outcome, and ends with the attribution lines.
**Done when:** `task check` exited 0 with `check:comments`'s completion line
in its output; the review record matches the committed tree; `git log -1
--stat` lists the files D12 names and no ledger, no specification and no file
under `internal/`, `cmd/` or `client/src`.

### Task 6 — The implementation report

File: `docs/reports/2026-09-25-a-finding-prints-the-share-it-compared.md`, its
own commit, per the `implementation-report` skill: each of the ticket's six
items answered with the observation that shows it, the four verbatim runs,
gaps G1 and G2 and how each was settled, the QA adjudications under their own
heading, and D10's recorded red lists.
**Done when:** the report is committed after the change's commit; `git log -2
--stat` shows the change then the report.

## Sign-off questions

1. **Precision (D1, D6).** Fewest decimals, two at least, that separate the
   share from the value it was compared with — or a fixed two decimals, which
   is the ticket's literal item 5 and leaves 4, 5 and 11 files on today's
   tree whose finding can read false again? The plan is written for the
   first; the second drops T5, T6, B5 and B6 and changes nothing else.
2. **The `18.95` figure (G1).** The writer amends the problem statement (95 of
   501, 18.96) or the report records the corrected form. Which?
3. **`--report` (D4).** Stays at one decimal, as item 4 says. Agreed, given the
   first open bullet?
4. **The sort (D9).** No new row, the six tests under VTT-053, VTT-055 and
   VTT-057 as the ticket says — or one row for the printed side of a finding?
5. **The band distance (D2).** Not printed. Agreed?

## Gaps carried from the verification

- **G1.** The ticket's problem statement says 18.95 prints `19.0`; it prints
  `18.9`. The false-reading band case begins above 18.95 (95 of 501 at 18.96
  is one), and T2's `18.92` and T5's `18.996` are the plan's fixtures either
  way. Question 2.
- **G2.** Two decimals do not close the class; D1 does, by construction, and
  T5, T6, B5 and B6 hold it. If sign-off chooses a fixed two decimals, the
  report records the residual counts (4, 5, 11 today) as a known gap in
  `docs/verification-debt.md`'s sense: a defect the gate reports in words that
  read false, with the recipe being any of the four files named above.
- **The ticket's item 5 and D1 differ by two words** ("two decimals" against
  "two at least"); question 1 settles it and the report records which.
