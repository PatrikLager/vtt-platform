# A mutation adjudication is keyed by the code it excuses, not by its line and column

## The problem

`tools/check-mutation.py` reads `tools/mutation-equivalents.txt` through
`read_equivalents` as keys of `(package, file:line:col, MUTATOR)`, and `run`
claims a survivor only when the key gremlins reports for it is equal to one of
them; gremlins reports a survivor as a position and a mutator
(`parse_survivors`). `tools/check-ts-mutation.py` reads
`tools/ts-mutation-equivalents.txt` as `(file, line:col, mutator,
replacement)`, and `check` claims a Stryker survivor only on an equal key. The
two files hold 36 and 67 entries. A line added or removed above an adjudicated
mutant moves its position and leaves its code as it was, and both gates then
fail: the entry is reported as no longer surviving and the survivor as
unadjudicated, or, where `moved_entries` or `pair_moves` can pair the two, as
`ADJUDICATION MOVED` with the new key, which fails as well. The entry is then
re-keyed by hand. `test_every_real_adjudication_still_points_at_its_mutant`, in
`tools/check_mutation_test.py` and in `tools/check_ts_mutation_test.py`, fails
on the same shift before any mutation run. The converse holds too: an entry
claims a survivor of its mutator at its position whatever code now stands
there, in the Go gate always and in the TypeScript gate whenever the
replacement does not carry the code, as with `ConditionalExpression`'s `true`
and `false`. Both gates already read the code at a key: `read_position`
returns the trimmed text of the key's line as `anchor`, and the pairing
compares it. CLAUDE.md rule 8 allows a bare coordinate nowhere but in these two
files, until this re-keying lands. The header of
`tools/mutation-equivalents.txt` gives line and column keying as the design, so
that a pre-approval does not "silently follow the code around" and start
"excusing whatever ends up on that line later"; this ticket keeps the second
half of that and drops the first.

Both cases a key without a line number must tell apart occur today. Entries
share a line: two Go entries on one line of `internal/rules`' `expr.go`, with
one mutator and two columns, and twelve TypeScript lines with more than one
entry each. A keyed line's text recurs: in each file, one keyed line's trimmed
text occurs again elsewhere in its source file.

## Done looks like

1. With a comment line inserted at the top of `internal/engine/apply.go` and
   of `client/src/fold.ts`, both of which hold adjudicated mutants, and neither
   equivalents file edited, `python3 tools/check_mutation_test.py` and
   `python3 tools/check_ts_mutation_test.py` pass. On today's tree both fail,
   at `test_every_real_adjudication_still_points_at_its_mutant`.
2. A survivor whose line moved and whose code did not is claimed by its entry,
   and the gate exits 0 without printing `ADJUDICATION MOVED`. A test in each
   self-test file shows it. Today's gates, given the same case in today's
   format, fail it, with `ADJUDICATION MOVED` or a stale entry beside an
   unadjudicated survivor.
3. A survivor at an entry's position whose code is not the entry's is not
   claimed, and the entry is reported. A test in each self-test file shows it.
   Today's gates, given the same case in today's format, claim it: the Go
   gate always, the TypeScript gate for a `ConditionalExpression` entry.
4. What is refused today is still refused, each held by a test in each
   gate's self-test file that passes before and after. A test that exists
   today is named; it may gain a source fixture and keeps its assertion:
   - a survivor of the entry's mutator on another statement of the same file
     (`test_adjudication_is_matched_on_all_three_fields`,
     `test_a_survivor_on_a_different_statement_is_not_paired`);
   - a survivor of another mutator at the entry's code
     (`test_an_adjudication_is_keyed_by_mutator_not_just_location`);
   - a TypeScript survivor with another replacement at the entry's code
     (`test_two_mutants_at_one_location_are_two_separate_claims`);
   - two mutants of one mutator on one line, as two claims;
   - a second copy of an adjudicated line in the same file: one entry claims
     one survivor, and the other survivor is reported unadjudicated;
   - an entry whose code stands nowhere in its file is reported, not kept.
5. No key in either file holds a line number, and over the survivors of one
   run of each gate the new keys claim exactly what the old keys claim at
   this ticket's base. A command the plan names prints both sets and their
   difference, and the difference is empty.
6. No reason and no header in either file carries a dated account of a key
   being re-pointed. A script shows that each reason after the cut is the
   reason before it less whole sentences. Which sentences are such accounts
   is held by Phase 4b's reading.
7. CLAUDE.md rule 8 makes no exception for mutation coordinates, a new
   specification states how an entry is matched to a survivor, and each
   equivalents file's header gives the format and points at it. Held by Phase
   4b's reading.
8. `task check` whole is green.

## Rules this puts on the system

- A survivor is claimed by the entry that names its file, its mutator, for
  TypeScript its replacement, and the code it sits on, wherever in that file
  the code now stands.
- A survivor whose code no entry names is not claimed, at whatever position an
  entry once named.
- One entry claims at most one survivor.
- An entry that claims no surviving or timed-out mutant fails the gate.
- An entry is refused when its code stands in its file a number of times
  other than the number it records.

## What it touches

1. `tools/check-mutation.py`, `tools/check_mutation_test.py`,
   `tools/mutation-equivalents.txt`
2. `tools/check-ts-mutation.py`, `tools/check_ts_mutation_test.py`,
   `tools/ts-mutation-equivalents.txt`
3. `docs/requirements.md`, with the rows the tests of 1 and 2 cite
4. The new specification, under `docs/specifications/`
5. `CLAUDE.md` rule 8, after all of the above
6. The implementation report, under `docs/reports/`

The Go and the TypeScript gate share no code, so 1 and 2 may land in either
order.

## Specifications this moves

New: how a mutation adjudication is matched to a surviving mutant, and what
the gates refuse in an entry.

## What could not be established

- Rule 9 of CLAUDE.md does not apply: no tabletop function is designed, and
  MapTool has no mutation gate.
