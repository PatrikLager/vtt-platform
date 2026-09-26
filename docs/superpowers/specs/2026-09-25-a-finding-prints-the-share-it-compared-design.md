# A comment-gate finding prints the share it compared

## The problem

`gate` in `tools/check-comments.py` compares a file's unrounded comment share
with its ceiling (`share > ceiling + EPS`, `share < ceiling - BAND - EPS`,
`share > DEFAULT + EPS`) and prints the share with `%.1f` in the finding and
in the notice it emits. A share of 20.04 against a ceiling of 20.0 is refused
with `comment share 20.0 is above its ceiling 20.0 and this change added a
comment line to it`, which reads false; a share of 18.92 against a ceiling of
20.0 is refused with `comment share 18.9 has fallen more than 1.0 under its
ceiling 20.0`, where the printed figures are 1.1 apart and the sentence
happens to read true, while 18.96 (95 of 501) prints `19.0 has fallen more
than 1.0 under its ceiling 20.0`, which reads false; the default arm and the notice
(`is at 20.0 above its ceiling 20.0`) do the same. The verification of the
citation-line ticket found the first case on the real tree
(`docs/superpowers/plans/2026-09-25-a-citation-line-is-not-prose.md`, "What
the verification found": 54 over 1,348 is 4.006, printed `4.0` against `4.0`)
and carried it as a gap. The ledger's own values, which the raised-row finding
and `--write-ledger` print, are tenths and read true as printed.

## Done looks like

1. In a scratch clone, a Go file whose share is 20.04 percent (101 comment
   lines of 504 non-blank) against a ledger row of 20.0, with the change
   adding a comment line, is refused with `comment share 20.04 is above its
   ceiling 20.0 and this change added a comment line to it`; today the same
   run prints `comment share 20.0 is above its ceiling 20.0 ...`.
2. The same for the band arm (a share of 18.92, 95 of 502, against 20.0:
   `comment share 18.92 has fallen more than 1.0 under its ceiling 20.0`),
   the default arm (25.03, 189 of 755, with no row: `comment share 25.03
   with no row in tools/comment-ceilings.txt is above the default ceiling
   25.0`) and the notice (`is at 20.04 above its ceiling 20.0`); today each
   prints the share to one decimal.
3. `tools/check_comments_test.py` has one case per arm in items 1 and 2, each
   red on today's checker and green afterwards, and `task check:comments`
   runs them.
4. Still true afterwards: the raised-row finding prints the ledger's values
   as they stand (`0.1`, `0.0`), `--write-ledger` writes tenths, and
   `--report` prints one decimal; `task check:comments` ends `clean` and
   `git diff --stat tools/comment-ceilings.txt` prints nothing.
5. The checker's docstring says a finding prints the share it compared, to
   the fewest decimals, two at least, that put it on the side of the compared
   value the verdict names, and that ledger values are tenths.
6. `task check` whole is green.

## Rules this puts on the system

A finding or notice that compares a share with a ceiling prints the share on
the side of that ceiling its verdict names.

The findings corrected are VTT-053's, VTT-055's and VTT-057's; the rule above
is the one the new cases hold, and those rows are unchanged.

## What it touches

1. `tools/check-comments.py`
2. `tools/check_comments_test.py`
3. `tools/check_comments_qa_test.py`, if Phase 4a's QA derives a case that
   belongs beside its others
4. `docs/requirements.md`, one row after sign-off, by the dispenser, its
   evidence cell by hand

One component, one commit. No file under `internal/`, `cmd/` or `client/src`
changes; `tools/comment-ceilings.txt` does not change.

## Specifications this moves

None.

## What could not be established

- Whether `--report`'s per-file share should print two decimals as well. It
  is the sweep's reading aid, not a finding, and the ledger it is read
  against holds tenths; the plan decides.
- Two decimals are not enough: 53 of 279 is 18.9964 and prints `19.00` under
  a ceiling of 20.0 with the band at 1.0; the verification measured four files
  on today's tree whose first crossing prints equal at two decimals. The
  comparison stays unrounded and the finding prints the fewest decimals, two
  at least, that separate the share from the value it was compared with.
- Rule 9 of CLAUDE.md does not apply: no tabletop function is designed.
