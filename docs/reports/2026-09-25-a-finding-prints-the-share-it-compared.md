# A comment-gate finding prints the share it compared: the change

**Ticket:** `docs/superpowers/specs/2026-09-25-a-finding-prints-the-share-it-compared-design.md`,
amended at sign-off on three points its verification found (a figure, the
precision item 5 asks for, and one rule where it had said none).
**Plan:** `docs/superpowers/plans/2026-09-25-a-finding-prints-the-share-it-compared.md`,
verified by `verify-ticket` (Passes with gaps); its five sign-off questions
were answered on 2026-09-26: 1, 3 and 5 as the plan proposed, 2 by amending
the ticket, 4 against the plan's proposal, with one row.
**Last commit that changes code:** `50f3f76`, on `b9afde3`, `main` at the
time; it corrects one docstring sentence of `1a1501c`, the commit the work
landed in. Every code reference below is to `50f3f76`'s tree, which differs
from `1a1501c`'s by that sentence.

## The period, in commits

    git log --oneline b9afde3..50f3f76

    50f3f76 shown's docstring carries no count
    1a1501c A comment-gate finding prints the share it compared: VTT-078

`git diff --stat b9afde3..50f3f76`: 6 files changed, 730 insertions(+), 10 deletions(-).

The gate: `task check`, whole, over the tree of `1a1501c` before it was
committed, and again over `50f3f76`'s: exit 0 on its third run, every step printing its own verdict,
`check:comments` ending `clean`, `check:requirements-chain` reading 78 rows,
`check:mutation` reporting fourteen packages with zero unadjudicated
survivors. The first two runs stopped at `check:race`, on three different
`cmd/vtt` e2e tests whose five-second subprocess deadlines expired while a
parallel session loaded the machine (`uptime` read 9 to 17 at the time);
each passed alone at low load, and the third run started behind a guard on
load and on no other `go test`. Nothing in this change touches Go code; the
deadlines are a ticket of their own. Before it, D13's steps by hand: both
test files, the gate to its completion line, the ledger diff, `--report`'s
summary, `check:comments`, `check:requirements-chain`; then the pre-commit
hook's nine checks.

## Done looks like, answered

1. `[x]` `test_a_ceiling_finding_prints_the_share_it_compared`: 101 comment
   lines of 504 against a row of 20.0, one comment line added, refused with
   `comment share 20.04 is above its ceiling 20.0 and this change added a
   comment line to it`; on `b9afde3`'s checker the same fixture prints
   `comment share 20.0 is above its ceiling 20.0 ...` (run the seven tests
   against `git show b9afde3:tools/check-comments.py` in a scratch clone).
2. `[x]` `test_a_band_finding_prints_the_share_it_compared` (95 of 502:
   `comment share 18.92 has fallen more than 1.0 under its ceiling 20.0`),
   `test_a_default_finding_prints_the_share_it_compared` (189 of 755:
   `comment share 25.03 with no row in tools/comment-ceilings.txt is above
   the default ceiling 25.0`) and `test_a_notice_prints_the_share_it_compared`
   (`is at 20.04 above its ceiling 20.0`); each printed one decimal on
   `b9afde3`'s checker.
3. `[x]` The four cases above and three more,
   `test_a_band_finding_prints_past_two_decimals_when_two_do_not_separate`
   (53 of 279: `18.996`) and
   `test_a_ceiling_finding_prints_past_two_decimals_when_two_do_not_separate`
   (33 of 824: `4.005`) and
   `test_a_band_finding_prints_past_the_band_at_a_ceiling_whose_edge_drifts`
   (17 of 109 under a ceiling of 16.6: `15.596`), in
   `tools/check_comments_test.py`; all seven red on `b9afde3`'s checker, the
   seventh alone red on the change before the review's fix, all green after;
   `task check:comments` runs the file.
4. `[x]` `test_a_ledger_row_raised_above_the_base_is_refused` still asserts
   `0.1` and `0.0`; `test_write_ledger_never_raises_a_row` still reads
   `5.0`; `report`'s format string is unchanged (`git diff b9afde3..1a1501c --
   tools/check-comments.py` touches `shown`, the four sites and the
   docstring); `task check:comments` ends `clean` and `git diff --stat
   tools/comment-ceilings.txt` prints nothing.
5. `[x]` `python3 tools/check-comments.py | grep -c 'two at least'` prints 1
   and `grep -c 'to one decimal' tools/check-comments.py` prints 0.
6. `[x]` `task check` whole was green over `1a1501c`'s tree (above).

## What the rules became

| Rule, as the ticket words it after sign-off | Became |
|---|---|
| A finding or notice that compares a share with a ceiling prints the share on the side of that ceiling its verdict names | VTT-078, the seven tests as evidence; "or notice" added at review, since the checker keeps notices apart from findings and `test_a_notice_prints_the_share_it_compared` and QA's `test_qa_a_notice_prints_the_share_above_the_ceiling` hold it |

Refused by the sort: "the docstring says two decimals" (the presence of a
sentence, held by item 5's command), "`--report` prints one decimal" (a
status, held by `report`'s format string). VTT-053, VTT-055 and VTT-057 are
unchanged: what they refuse is the same, only what the refusal prints moved.

## The breaks

One edit each to `tools/check-comments.py` in a scratch clone carrying the
change, the tests that went red:

| Break | Red |
|---|---|
| B1 the ceiling site back to `%.1f` | the ceiling case and its past-two-decimals case |
| B2 the band site back to `%.1f` | the band case, its past-two-decimals case and the drifting-edge case |
| B3 the default site back to `%.1f` | the default case |
| B4 the notice site back to `%.1f` | the notice case |
| B5 `shown` fixed at two decimals | the two past-two-decimals cases and the drifting-edge case |
| B6 `shown` starting at one decimal | the band case only: `18.9` separates from `19.0`, `20.0` does not from `20.0` |
| B7 the band bound unrounded (`ceiling - BAND`) | the drifting-edge case only |

B1, B3, B4 and B6 match plan D10's expectation; B2 and B5 each add the
drifting-edge case, which D10 predates; B7 is the review's addition.

## QA adjudications

QA derived ten cases from SPEC-010, the docstring and rows VTT-053, VTT-055,
VTT-057 and VTT-078; all ten passed on first run, and each was shown to fail
under an injected output regression (tenths, fixed two, fixed three, the
share moved onto the compared value, hundredths in the ceiling, two decimals
in the ledger) or a mutated fixture.

- "Fewest decimals" did not say rounded or truncated: spec ambiguity in the
  docstring; it now says "rounded to the fewest decimals". Nine of QA's ten
  fixtures give the same count under both readings; 11 of 57 prints three
  decimals only under rounding, the reading the docstring names.
- `--report` prints `20.0%  ceiling  20.0` for a file the gate refuses as
  above its ceiling (81 of 404 is 20.0495): a true observation and not
  VTT-078's, whose finding names a verdict; sign-off question 3 kept
  `--report` at one decimal. Recorded under "What could not be established"
  for a later ticket.
- The ceiling, the band and the default printed as tenths is inferred from
  the docstring's spelling of them, not stated: QA's case pins the inference
  and is kept; the docstring is not extended.
- Two docstring sentences no row states, "a ledger row holds a share as a
  tenth, rounded up" and "compared unrounded", are pinned by QA's cases under
  VTT-078; candidates for a later sort, not dispensed here.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Ticket: "This ticket adds no rule." | One row, VTT-078. | The verification's reading found the rule the six tests hold; cited under VTT-053, VTT-055 and VTT-057 they would have held something those rows do not say. Sign-off question 4. |
| Ticket item 5: "to two decimals". | The fewest decimals, two at least, that separate the share from the compared value. | Two decimals leave 53 of 279 printing `19.00 has fallen more than 1.0 under its ceiling 20.0`, and four files on `b9afde3`'s tree whose first crossing would print equal; measured by the verification. Sign-off question 1. |
| Ticket: 18.95 prints `19.0`. | 18.96 (95 of 501) does; 18.95 is stored as 18.9499... and prints `18.9`. | The verification's command; the ticket was corrected at sign-off. |
| Plan D1: the band site passes `ceiling - BAND`. | It passes `round(ceiling - BAND, 1)`. | `ceiling - BAND` drifts by an ulp for 30 of the 990 tenth-valued ceilings and lands above the decimal edge for 15 of them, 16.6 and 64.9 among the ledger's rows at `1a1501c`; a two-decimal string reading exactly the edge then passed `float(s) < bound`, printing `15.60 has fallen more than 1.0 under its ceiling 16.6`. The gate's verdict was right (`EPS` absorbs the ulp); the print was the defect the ticket exists to remove. Found by the reading review, which had QA's Fraction check to hand and saw QA's two band fixtures (21.1, 20.3) sit on exact subtractions. |
| Plan D9, D12, D13 step 7, Tasks 3 and 5: evidence cells on VTT-053, VTT-055 and VTT-057, no new row. | One row, VTT-078; the plan's five passages now say so and record the five sign-off answers. | Sign-off question 4 chose the row; the plan was unlanded and its sentences were made true rather than recorded as false. |
| `shown`'s fallback returns the share at ten decimals. | It raises. | Unreachable from `gate` (the comparison held by more than `EPS`; a fuzz of 200,000 shares never reached it), and if reached it would print the share at the bound, the output the helper exists to prevent. |

## What could not be established

- `shown` stops at ten decimals; a share within 1e-10 of its bound would
  print equal there. The gate compares with `EPS` of 1e-9 first, so no
  finding reaches that branch, and no test holds it.
- The precision a finding prints is not stated in SPEC-010, which leaves the
  checker's figures to its docstring; whether the record should name it is
  left to the next reading of SPEC-010.
- `--report`'s per-file line shows share and ceiling side by side to one
  decimal, so a file just above its ceiling reads level with it (QA's
  observation). Kept by sign-off; a mark on the line, or the finding's
  precision, is a ticket of its own.
- The `64.9` row's drift case is held by arithmetic only: `go_file` cannot
  build a share above 50 percent, and the test at 16.6 holds a bound with
  the same defect, an ulp above its edge.
- Python's `%.*f` breaks exact ties half-even; a tie needs a dyadic share and
  every bound is a tenth, so no tie can sit on a bound and no claimed figure
  is one.

## What was deliberately left out, and where it went

- `--report`'s one decimal: kept, sign-off question 3; it is read against a
  ledger of tenths.
- The distance fallen in the band finding: not printed, sign-off question 5.
- No file under `internal/`, `cmd/` or `client/src` changed; no mutation key
  moved; no ledger row moved.

## The sort

One candidate, accepted as VTT-078 with the seven tests; the two refusals are
under "What the rules became".
