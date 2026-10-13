# A stale adjudication's line says where the same run decides its remedy

## The problem

Each mutation gate prints one line for an adjudication that claims no
surviving or timed-out mutant and that the static check does not refuse: the
`for e in stale` loop of `run` in `tools/check-mutation.py`, and of `check` in
`tools/check-ts-mutation.py`. The line says "no surviving or timed-out mutant
sits on that code" and ends with two remedies and a condition between them:
"If the code was edited, re-judge the reason against the new code and re-key
the entry; if a test now kills the mutant, remove the entry so it cannot
pre-approve a future one."

An entry reaches that line when the run reports no surviving or timed-out
mutant under its key. The line does not say why, and the ways differ in what
the reader must do.

- The mutant its reason describes did not survive the run. No other line of
  the run concerns the entry.
- Its key no longer names that mutant, though the tree does not refute the
  entry, and the mutant survives under another key.
  `test_a_wrong_span_is_not_refused_statically_and_claims_nothing` in
  `tools/check_ts_mutation_qa_test.py` builds it: the survivor sits on the
  entry's code at the entry's offset with another `span`, and the run prints
  it unadjudicated above the stale line.
- A survivor in its file is unadjudicated and is not its mutant. The run
  prints the same two lines as in the case before.
  `test_an_entry_on_one_copy_does_not_claim_another_copys_survivor`, in
  `tools/check_mutation_test.py` and in `tools/check_ts_mutation_test.py`,
  builds it with the entry's own mutant not among the survivors, killed in the
  TypeScript fixture and absent from the Go one, and a survivor on another
  copy of its code; so do
  `test_a_survivor_on_other_code_at_the_entrys_old_position_is_not_claimed`
  in `tools/check_mutation_qa_test.py` and
  `test_conditional_expression_entry_does_not_claim_edited_code_at_its_old_position`
  in `tools/check_ts_mutation_qa_test.py`, with the line edited and the
  entry's code still standing elsewhere in the file.
- A survivor in its file is one the gate cannot read (VTT-304). It is printed
  on a line of its own with no entry under it, and may be the entry's mutant.

The line is wrong in two places. Its first clause is false where a survivor
does sit on that code under another key: the `span` shape and the copy shape.
And its condition tells none of the cases apart. An entry whose own code line
was edited away never reaches the line: `entry_faults` refuses it, and both
gates drop a refused entry from the stale list (`stale = [e for e in stale if
id(e) not in faulted]`). So the code an entry names stands in its file as
often as the entry records in every case that does reach the line, and a
reader who checks whether "the code was edited" by looking at that code finds
it unedited each time. What a reader has to go on is another line of the same
output, the survivor printed unadjudicated in the entry's file with an entry
ready to paste under it, and the stale line names no other line. A reader who
then removes the entry may have deleted the reason the survivor beside it
must be judged against.

The header of `tools/ts-mutation-equivalents.txt` gives the same account of
why an entry is stale and one remedy for both causes: "A stale entry — one no
surviving or timed-out mutant sits on, because the mutant was killed or the
code was edited — is a hard error: re-judge the reason against the new code
before writing the new code line."

`tools/check-ts-mutation.py` says twice that its static check runs before the
report is read, and its test file once: the docstring of `entry_faults` opens "for every entry the
tree refutes, before the report is read", and the section comment above
`MUTATOR_TOKENS` opens "WHAT AN ENTRY'S OFFSET MUST HOLD, checked against the
tree before the report is read", wrapped over two lines. The docstring of
class `EntryCheck` in `tools/check_ts_mutation_test.py` says it a third time
and cites SPEC-020 for it. `main` reads and
parses the report, `json.loads(report_path.read_text())`, and passes it to
`check`, which calls `entry_faults` first. SPEC-020 states the order as it is:
"in TypeScript once Stryker has written its report and before the gate goes
through its mutants".

Not in this ticket: what either gate accepts, refuses or claims; a gate
pairing a stale entry with a survivor itself, which the `ADJUDICATION MOVED`
path did until SPEC-020's ticket removed it; a line of its own for an entry
whose package the Go run does not measure (`stale += [e for e in entries if
e.package not in seen]`) or whose mutant is no longer made; and a mutant one
run kills and the next does not, which reaches the same line.

## Done looks like

1. In a run where an entry is stale and a survivor in its file is
   unadjudicated, the stale line says that the run's report alone does not say
   the mutant was killed, that a survivor the same run prints unadjudicated in
   that file may be the entry's mutant under another key, that the entry is
   then re-keyed and not deleted if its reason holds for that mutant, and that
   a survivor the gate cannot read may be its mutant too. A new test in
   `tools/check_mutation_test.py` and one in `tools/check_ts_mutation_test.py`
   assert it on the stale line alone, and assert that the line does not give
   "If the code was edited" as its condition. On today's tree both fail.
2. The stale line still makes removal conditional on a test killing the
   mutant, and still carries `remove the entry`. The same two tests assert
   it.
3. What the existing tests count by stays countable. The stale line carries
   `no longer survives` once, and carries none of `not adjudicated`, in Go,
   `SURVIVED` in any form, in TypeScript, and `ADJUDICATION MOVED`, in both.
   `python3 tools/check_mutation_test.py`, `python3
   tools/check_mutation_qa_test.py`, `python3 tools/check_ts_mutation_test.py`
   and `python3 tools/check_ts_mutation_qa_test.py` pass with no test that
   exists today edited. A refused entry's line carries neither `no longer
   survives` nor `remove the entry`; no test holds the first today, and a new
   test in each of the two files does.
4. A search of `tools/check-ts-mutation.py` and
   `tools/check_ts_mutation_test.py` for the words `before the report is
   read`, whatever white space or `#` breaks them, finds nothing; today it
   finds the three places named above. That the two docstrings state the
   order SPEC-020 states is held by Phase 4b's reading.
5. `task check` whole is green.
6. `grep -c 'mutant was killed or the code was edited'
   tools/ts-mutation-equivalents.txt` prints 0; today it prints 1. That the
   header's sentence agrees with the gate's line is held by Phase 4b's
   reading. The owner ruled this item in on 2026-10-13, after the first cold
   read of the new line found the two in conflict.

## Rules this puts on the system

One rule no row states today, in its own words, unnumbered until the sort:

- The line a mutation gate prints for an adjudication that claims nothing,
  and that the tree does not refute, names where in the same run's output its
  reader finds whether its mutant survives under another key.

VTT-301, that such an adjudication fails the gate, is unchanged.

## What it touches

1. `tools/check-mutation.py`, `tools/check_mutation_test.py`
2. `tools/check-ts-mutation.py`, `tools/check_ts_mutation_test.py`, and the
   header of `tools/ts-mutation-equivalents.txt`
3. `tools/check_mutation_qa_test.py` and `tools/check_ts_mutation_qa_test.py`,
   if Phase 4a derives tests
4. `docs/requirements.md`, if the sort allocates an id
5. `docs/specifications/020-a-mutation-adjudication-names-the-code-it-excuses.md`
6. The implementation report, under `docs/reports/`

The Go and the TypeScript gate share no code, so 1 and 2 may land in either
order.

## Specifications this moves

docs/specifications/020-a-mutation-adjudication-names-the-code-it-excuses.md

## What could not be established

- Rule 9 of CLAUDE.md does not apply: no tabletop function is designed, and
  MapTool has no mutation gate.
- Whether the new line leads a reader to the right act. A test shows that
  its words are there.
- How often an entry reaches the stale line by a moved key and how often by a
  kill. A green run prints no stale line, and no red run of either real gate
  was read for this ticket.
