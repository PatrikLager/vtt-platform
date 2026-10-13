# A stale adjudication's line says where the same run decides its remedy: the change

**Ticket:** `docs/superpowers/specs/2026-10-13-a-stale-adjudications-line-says-where-the-run-decides-design.md`,
amended by its writer three times after it was verified: on the verifier's
findings, after the cold reads, and after the reading review.
**Plan:** `docs/superpowers/plans/2026-10-13-a-stale-adjudications-line-says-where-the-run-decides.md`,
written by `verify-ticket` (passes with gaps) and extended by the ticket's
writer; the plan names which parts are whose.
**The owner's rulings:** the work itself, chosen over correcting the docstring
alone once the first account of the defect had been withdrawn; at sign-off,
the plan and its text, one requirement id, and Phase 4a as a cold read; after
the cold reads, the header sentence of `tools/ts-mutation-equivalents.txt`
ruled in and the longer text kept; after the reading review, every Must and
Should fixed and the nits as proposed (N2, N3 and four of N8's five phrases
left), the two design observations left for a ticket of their own, and the
commit allowed once `task check` was green; after this report's review, every
finding fixed, the report committed, SPEC-020's change committed without a
reading review of its own, and the branch pushed and merged if the last gate
is green.
**Last code commit:** `7e19cde`, on `5cb42fa`, `main` at the time. Every code
reference below is to `7e19cde`'s tree.

## The period, in commits

    git log --oneline 5cb42fa..7e19cde

    7e19cde A stale adjudication's line says where the same run decides its remedy

    git diff --stat 5cb42fa..7e19cde

     docs/requirements.md                               |    1 +
     ...djudications-line-says-where-the-run-decides.md | 1069 ++++++++++++++++++++
     ...tions-line-says-where-the-run-decides-design.md |  151 +++
     tools/check-mutation.py                            |   33 +-
     tools/check-ts-mutation.py                         |   43 +-
     tools/check_mutation_test.py                       |  104 ++
     tools/check_ts_mutation_test.py                    |  105 +-
     tools/ts-mutation-equivalents.txt                  |    8 +-
     8 files changed, 1494 insertions(+), 20 deletions(-)

## Where the work started, and what was withdrawn

The item is recorded in the message of `d64cc9c`, which left it for a ticket
of its own: "both gates' stale message advises re-judging an edited entry, but
an edited entry is refused by the static check, and a refused entry is not
reported stale, so that advice almost never reaches the case it is written
for". It was carried forward as "never", and put to the owner as a line whose
advice describes a case that cannot reach it. That was false, and was
withdrawn before the ticket was written: tests that passed at `5cb42fa` build
three ways an entry reaches the stale line with a survivor printed above it,
one of them by editing the line the entry's mutant sat on.
`test_an_entry_on_one_copy_does_not_claim_another_copys_survivor`, in both
unit test files, puts the survivor on another copy of the entry's code;
`test_a_survivor_on_other_code_at_the_entrys_old_position_is_not_claimed` in
`tools/check_mutation_qa_test.py` and
`test_conditional_expression_entry_does_not_claim_edited_code_at_its_old_position`
in `tools/check_ts_mutation_qa_test.py` edit the line and leave the entry's
code standing elsewhere; and
`test_a_wrong_span_is_not_refused_statically_and_claims_nothing` in
`tools/check_ts_mutation_qa_test.py` gives the entry a `span` that is not its
mutant's. What was true is the ticket's problem statement: the line's first
clause is false where a survivor sits on the entry's code under another key,
and its condition, "If the code was edited", tells no case apart, since an
entry whose own code line was edited away is refused and the code of every
entry that does reach the line stands as often as recorded.

The verification then built twelve ways an entry reaches the line and read
what each gate printed; its table is in the plan under "What was run". Two
facts from it shaped the text. A moved key and a killed mutant beside an
unjudged survivor print the same two lines, so a line that chose a remedy from
the survivors the gate holds would assert what it had not looked up. And the
line is reached in more ways than a kill and a moved key: an entry whose
package the Go run does not measure, a mutant that is absent without a kill,
and a survivor the gate cannot read.

## Done looks like, answered

1. `[x]` The stale line says what Done 1 lists, and nothing gives "If the code
   was edited" as a condition.
   `test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file`,
   in `tools/check_mutation_test.py` (class `StaleLineTest`) and in
   `tools/check_ts_mutation_test.py` (class `StaleLine`), holds each of the
   line's eleven sentences on the stale line and on no other line of the
   output, and the line's end word for word. Run against the gates of
   `5cb42fa`, it fails with twelve failures in each file.
2. `[x]` Removal waits on a kill. The same test holds the eleventh sentence,
   "... if a test now kills the mutant, remove the entry so it cannot
   pre-approve a future survivor; if none does, leave the entry and find out
   why this run did not report its mutant." Break K3, which makes the removal
   unconditional, reds it in both files.
3. `[x]` What the existing tests count by stays countable.
   `python3 tools/check_mutation_test.py`, `tools/check_mutation_qa_test.py`,
   `tools/check_ts_mutation_test.py` and `tools/check_ts_mutation_qa_test.py`
   run 122, 85, 70 and 79 tests and pass, in the repository, where the stored
   Stryker report is; in a fresh clone the fourth skips the eight tests that
   read it. No test that existed at `5cb42fa` is
   edited: `git diff --numstat 5cb42fa..7e19cde` shows 104 lines added and none
   deleted in the first file, 103 added and 2 deleted in the third, and
   nothing in the two QA files; the two deleted lines are the opening of class
   `EntryCheck`'s docstring (Done 4). The first new test asserts `no longer
   survives` once on the stale line and, by gate, no `not adjudicated` and no
   `SURVIVED`; `test_a_survivor_on_a_different_statement_is_not_paired`, in
   Go, and
   `test_a_survivor_of_the_entrys_mutator_on_another_statement_is_not_claimed`,
   in TypeScript, existed and hold `ADJUDICATION MOVED` absent from a run that
   prints the stale line.
   `test_a_refused_entry_and_a_claimed_one_get_no_stale_line`, new in each
   file, holds that a refused entry's run prints neither `no longer survives`
   nor `remove the entry`.
4. `[x]` The words `before the report is read`, searched for across white
   space and `#`, are found nowhere in `tools/check-ts-mutation.py` or
   `tools/check_ts_mutation_test.py`; at `5cb42fa` they are found twice in the
   first and once in the second. The search is in Appendix A. That the
   docstring of `entry_faults` and the section comment above `MUTATOR_TOKENS`
   state the order SPEC-020 states is the reviewer's reading: it traced
   `main`, which reads the equivalents, parses the report and calls `check`,
   which calls `entry_faults` before its loop over `mutants(report)`. The
   docstring of class `EntryCheck` took the words the reviewer proposed.
5. `[x]` `task check` whole, run on the staged change set that `7e19cde`
   commits, ended `TASK_CHECK_EXIT=0` with no `task: Failed`.
   The Go gate reused all fourteen stored verdicts and the TypeScript task
   verified the stored report without mutating, as the plan measured it would,
   so the run exercised the new text only through the four self-tests, which
   it ran: 122, 85, 70 and 79 tests.
6. `[x]` `grep -c 'mutant was killed or the code was edited'
   tools/ts-mutation-equivalents.txt` prints 0; at `5cb42fa` it prints 1. The
   reviewer read the new sentences against the gate's line and found one word
   wrong, "refuses" where an entry whose code stands elsewhere is not refused;
   it reads "fails", as the Go file's head does.

## What the rules became

- "The line a mutation gate prints for an adjudication that claims nothing,
  and that the tree does not refute, names where in the same run's output its
  reader finds whether its mutant survives under another key." VTT-308,
  allocated by `requirement-id` after sign-off, tried on a copy of the
  register first. Held by the first new test in each file.

VTT-301 is unchanged and gained no evidence.

## The sort

One candidate accepted, above: one thing, about what and not how, false on the
tree of `5cb42fa`, and held by a test that names it. The 2026-10-10 sort
refused an id to a message's form; the owner ruled this one in, since a pasted
entry that does not parse fails at once and bad advice on this line fails
silently.

Refused, one line each:

- That removal is conditional on a kill: the same instruction's second half,
  held by the same test; not a second rule.
- That the line is the same with and without a survivor beside it: how the
  rule is met, a decision of the plan (D1).
- That a refused entry gets no stale line: SPEC-020 states it already, and it
  is not this ticket's rule.
- That the line carries `no longer survives` once and none of the survivor
  line's words: what the tests count by, not a rule on the system.
- That the docstrings state the order of calls: prose, held by a reading.
- That where to look comes before the removal: how.
- That survivors are printed before stale entries: what "Look above" rests
  on, held by the same test; scenery of the accepted rule.

## Phase 4a

A cold read, twice, each by one agent given twelve printed runs, the entry of
each as it stands in the adjudication file, and the head of each adjudication
file. The runs were built with the QA harnesses' fixtures against the gates
in the working tree, six per gate: the mutant killed and nothing beside it;
a survivor on the other copy; in Go the other mutant on the entry's line, in
TypeScript a wrong `span`; the line edited and the entry's code standing
elsewhere; a survivor on an unrelated statement; a survivor the gate cannot
read. Where the plan's D10 expects a removal, with nothing beside the entry,
both readers looked first and removed only on a kill they had seen, which is
what the committed line asks; in every other run each did what D10's pass
describes. Both said the harness
had put this project's `CLAUDE.md`, a memory index and a `git status` in
their context unasked, so neither was wholly cold.

In the tables below "closing" marks a finding the reader gave in answer to
the closing questions, with no run named.

**The first read**, of the plan's text (D2):

| # | Run(s) | Finding | What became of it |
|---|---|---|---|
| R1 | closing | "no longer survives under that key" first read as "killed"; "key" is never defined | Text: the key is glossed as "(the fields and code just named)". The phrase itself stays, since existing tests count by it. |
| R2 | closing | The passive "is re-keyed" read as something the gate does | Text: an imperative, "re-key this entry instead of deleting it: give it the header and code line printed there". |
| R3 | T4, T5 | "once its reason is judged ... and not deleted" has no branch for a reason that fails, and keeps an entry whose reason no longer fits | Text: "If the reason describes that mutant and holds there, ..." and a sentence for a reason that does not; the review's S1 then made that sentence three. |
| R4 | G4, T4, T5 | "re-keyed" sent the reader to the header; only the code line differs | Text: "the header and code line printed there". |
| R5 | closing | "pre-approve a future one": a future what; "such survivor" points back two sentences | Text: "a future survivor"; "no survivor above". |
| R6 | G3 | A reason judged "against the code printed there" passes because the code is identical; what differs is the offset | Text: "compare its header and code line with this entry's"; "Read this entry's reason against that mutant where it stands in the file, not against the code line printed there alone." |
| R7 | G2, T2 | The same: the printed line is the entry's own, so the look that decides is at the survivor's place in the file | The same text as R6. |
| R8 | G6, T6 | Beside a survivor the gate cannot read, the line opens the way to removing a sound entry, and "compare the entry printed under it" has nothing to work on | Text: "If one is printed as a survivor the gate cannot read (VTT-304), it may be this entry's mutant too: leave this entry as it is until a run can read that survivor." The ticket's exclusion of that case was narrowed. |
| R9 | G1, G5, T1 | The reader could not tell whether the entry's code still stands, and read "Only if ... a test now kills" as forbidding removal of an entry whose code is gone | Text: what the run found of the entry's code, and a last sentence that says what to do when no test kills. An entry whose code is gone is refused and gets no stale line (the third new test). |
| R10 | T2 | The TypeScript survivor line never says "unadjudicated" | Text: "printed unadjudicated, with an entry ready to paste under it". |
| R11 | G1, T1 | `ok  p: 0 survivor(s)` on a run that exits 1; no line in TypeScript says the file was measured | No change: other lines of the output, outside the ticket. |
| R12 | G3 | The head of the Go file does not say whether `offset` counts from 0 or 1 | No change: the head is outside the ticket. SPEC-020 says 1-based. |
| R13 | G6, T6 | The VTT-304 line does not say which of its "or" arms happened | No change: another line, outside the ticket. |
| R14 | G1, T1 | The reader wanted what the run recorded for the mutant: killed, not covered, not made | No change here; the owner ruled it a ticket of its own. |
| R15 | T1 | The head of `tools/ts-mutation-equivalents.txt` says an entry is stale "because the mutant was killed or the code was edited", which the gate's line contradicts | The owner ruled it in: the sentence is corrected (Done 6). |
| R16 | G3 | The entry's reason, if true, would excuse the line's other mutant as well | An artefact of the fixture: the reasons were written for the read. No change. |
| R17 | before the runs | The long line names the entry without `copies` | No change: `copies` is not in the key a survivor is claimed by (SPEC-020), and the line describes the key. |
| R18 | G2, T3 | `nth` differs while `copies` is equal, which the head's rule does not produce; an entry with `span=6` that once matched | Artefacts of the fixtures, as R16. |

**The second read**, of the text that answered the first:

| # | Run(s) | Finding | What became of it |
|---|---|---|---|
| S1 | closing | "no longer survives" first read as "was killed"; "the fields and code just named" read as a new clause | Text: the gloss is in brackets. "no longer survives" stays. |
| S2 | G4, T4 | "The static check found this entry's code in that file as often as the entry records" read as "nothing changed", then seen as in conflict with a survivor printed on other code with the same header | Text: "This run found this entry's code in that file on as many lines as the entry records, so a survivor printed on other code is on another line"; the review's M1 then added the exception. |
| S3 | closing | "the printed line" taken to be the long line itself | Text: "the code line printed there". |
| S4 | T4, T5 | "this entry does not move to that survivor, which is then a mutant nobody has judged": no act for the entry, and untrue of a renamed operand | Text, and then the review's S1: the three sentences for a reason that does not hold. |
| S5 | G1, T1 | "Only if ... a test now kills the mutant" read as something the gate saw; with no kill shown, no act is permitted | Text: "make the mutation by hand and run the tests, since this run's output does not show a kill: if a test now kills the mutant, remove the entry ...; if none does, leave the entry and find out why this run did not report its mutant." |
| S6 | T4 | "keep its reason" leaves a reason naming the old operand above the new code | Text: the words are gone. |
| S7 | G4, T4, T5 | "compare ... field by field": by the line's own words the code is not a field, and only the code differs | Text: "compare its header and code line with this entry's". |
| S8 | G4, T4 | The reader wanted the line number where the entry's code was found | No change here; it belongs with R14. |
| S9 | G6, T6 | "until a run can read that survivor" names no next step; VTT-304 is not explained | No change: the VTT-304 line above says why the survivor cannot be read. |
| S10 | all | The line is the same every time and never says which case is on the screen or which field differs | No change here; the owner ruled it a ticket of its own. |
| S11 | G2, T2 | Re-keying assumes the reason has one home; if it holds at both copies the first is left uncovered | No change: the first copy's mutant did not survive, so nothing there needs an entry. |
| S12 | T2 | "where it stands in the file" is too narrow for a reason about a caller, which may be in another file | Left: the words stand in the committed line. |
| S13 | G1 | The head of the Go file has no sentence saying a stale entry fails the gate | No change: the head is outside the ticket. |
| S14 | G4, T5 | Whether the gate accepts the printed entry pasted beside the old one, two entries with one header | No change: not this line's. Two entries that differ in their code line are two keys (SPEC-020). |
| S15 | G6 | "the source the run measured" implies a source other than the reader's, and nothing says when | No change: the VTT-304 line, outside the ticket. |
| S16 | G2 | The long line names the entry without `copies` | As R17. |
| S17 | G2 | `nth` differs while `copies` is equal | An artefact of the fixture, as R18. |
| S18 | T1, T3 | The conflict with the head of `tools/ts-mutation-equivalents.txt` again | As R15: the runs of both cold reads were built before the head was corrected. |

## The verification and Phase 4b

**The verification** passed the ticket with gaps and found five things false
or too strong in it: that in each of its three shapes the run
prints "the mutant" unadjudicated, where two of the three tests say in their
own words that the survivor is a mutant nobody judged; that two things leave
an entry stale; a phrase list in Done 3 that missed `SURVIVED` in any form and
`ADJUDICATION MOVED`; a `grep` in Done 4 that one line break defeats, with a
second copy of the claim standing behind it; and the stale line's own first
clause, which the ticket had not named. The writer amended the ticket on all
five. No agent verified the amended ticket.

**Phase 4b**, one reviewer over `git diff HEAD` with the third text in the
tree. It read the line literally against twelve runs and found the reader
right in every one. It built twenty-three shapes of its own, re-ran the
twenty-three breaks then in the plan and made nine of its own, and showed by
token stream and by bytecode, each with a control, that nothing but strings
and comments changed in the two gates.

| # | Finding | What became of it |
|---|---|---|
| M1 | "so a survivor printed on other code is on another line" is false where the source the run measured is not the tree the static check read: a Stryker report older than the tree, or a Go file edited after the check | The clause gained "than that code, unless the file changed between that count and the measuring of its mutants", and the test holds it (K33). |
| S1 | "that survivor is not this entry's" says more than the reader's judgement has shown: a reason that describes the mutant and no longer holds leaves a refuted entry standing | Three sentences in its place: a reason that describes the mutant and no longer holds removes the entry; one that does not describe it makes the survivor another mutant; in both the survivor needs a test or an entry of its own. |
| S2 | The loop comment and two class docstrings say the run "cannot tell" a killed mutant from a moved key; both tools report what became of each mutant, and the gate does not look it up | They say "does not look up", and cite VTT-308. |
| S3 | A third copy of "before the report is read", in the docstring of class `EntryCheck`, citing SPEC-020 | Corrected; the ticket's Done 4 searches both files. |
| S4 | The plan said the draft paragraph for SPEC-020 was replaced and gave no replacement | The plan's writer-written section carries the paragraph. |
| S5 | The first test held 43% of the line; nine edits to the rest left every suite green, among them "Look above" made "Look below" and the re-keying made unconditional | The test holds every sentence and the line's end; the second test has a run beside an unreadable survivor. The nine edits are K24 to K32, and each reds a new test. |
| N1 | The header's "and the gate refuses it" is false where the entry's code stands elsewhere as often as recorded | "and the gate fails", as the Go head says. |
| N2 | A timed-out mutant is not named as a place the entry's mutant may be | Left. The line's last sentence still ends at "leave the entry and find out why this run did not report its mutant". |
| N3 | The first test's name says the line "names the survivor"; the line names the place | Left: the name is cited by the register, the plan and this report, and VTT-308's sentence is exact. |
| N4 | The ticket said the Go fixture of the copy test has the entry's own mutant killed; it has no line for that mutant at all | The ticket says "not among the survivors, killed in the TypeScript fixture and absent from the Go one". |
| N5 | The plan's list of C1's files omitted the adjudication file | The writer's section lists C1's files. |
| N6 | "A fails in each file with seven failures" was the count for the first tests | The writer's section gives twelve, for the tests as committed. |
| N7 | The writer's list of the first reader's five places was not the reader's | Corrected from the reader's report. |
| N8 | Phrases that read two ways: "this run", "this entry's mutant", "on another line", "no longer survives", "until a run can read that survivor" | "on another line" is "on another line than that code". The others are left. |

The fourth text, the one committed, was read by nobody but its writer. What
changed after the review is M1's clause and S1's three sentences. `7e19cde`'s
message says all findings are fixed but two nits; with N8's four phrases the
nits left are three.

**This report's review**, one reviewer over this file as staged, against
`7e19cde`, `5cb42fa` and the session's evidence. It reproduced every number in
"Done looks like, answered", the break table and this appendix's printed
results, and found three Must, nine Should and seven nits, none in the code.
The Musts were three sentences this report's own tables contradicted: that
every finding of the reading review was fixed, that the second cold reader
doubted two runs, and that the verification built every way to the line. The
Shoulds: a break runner that stops at K1 on a clone at `7e19cde`; its log cut at
112 columns; a quotation found nowhere durable; unnamed tests behind the
opening section's claim; a gate run written as if two had been made;
SPEC-020 missing from what is still to come; seven of the cold readers'
findings with no row; the second and third texts kept nowhere; and
instruments given without what they printed, or not given. All are fixed in
this report, the last by saying which of the reviewer's instruments are not
given.

## The breaks

Each made on the working gate in a clone holding the change set, the gate's
two suites run, and the gate put back; the runner asserts that the edit
changed the file. A, B and C are the three new tests in the order Done 1, the
plan's D1, and the other direction. "Existing" counts tests that existed at
`5cb42fa`. The runner, the text it reads and its output are in Appendix A.

| # | The edit to the working gate | Go: new, existing | TypeScript: new, existing |
|---|---|---|---|
| K0 | the stale `print` rebuilt unchanged (the runner's own control) | none, 0 | none, 0 |
| K1 | the stale text of `5cb42fa` put back | A, 0 | A, 0 |
| K2 | every sentence from "Look above" to "When no survivor above" removed | A, 0 | A, 0 |
| K3 | the removal no longer conditional on a kill | A, 0 | A, 0 |
| K4 | the removal sentence moved before the pointers | A, 0 | A, 0 |
| K5 | "that alone does not say the mutant was killed" becomes "so the mutant was killed" | A, 0 | A, 0 |
| K6 | "If the code was edited, re-key it." appended | A, 0 | A, 0 |
| K7 | a phrase the survivor line is counted by, added | A, 4 | A, 15 |
| K8 | the stale line names no entry | A, 0 | A, 1 |
| K9 | a note appended when the gate holds an unadjudicated survivor | A and B, 0 | A and B, 0 |
| K10 | `stale = [e for e in stale if id(e) not in faulted]` removed | C, 3 | C, 2 |
| K11 | the stale loop moved above the unadjudicated loop | A, 0 | A, 0 |
| K12 | a second `no longer survives` on the line | A, 2 | A, 13 |
| K13 | every entry called stale | C, 40 | C, 42 |
| K14 | "in that file" removed | A, 0 | A, 0 |
| K15 | "instead of deleting it" removed | A, 0 | A, 0 |
| K16 | the sentence on a survivor the gate cannot read removed | A, 0 | A, 0 |
| K17 | what the run found of the entry's code removed | A, 0 | A, 0 |
| K18 | the three sentences for a reason that does not hold removed | A, 0 | A, 0 |
| K19 | "Read this entry's reason against that mutant ..." removed | A, 0 | A, 0 |
| K20 | "printed unadjudicated, with an entry ready to paste under it" becomes "printed" | A, 0 | A, 0 |
| K21 | "if none does, leave the entry ..." removed | A, 0 | A, 0 |
| K22 | "make the mutation by hand and run the tests ..." removed | A, 0 | A, 0 |
| K23 | "compare its header and code line with this entry's" becomes "compare it with this one" | A, 0 | A, 0 |
| K24 | "Look above" becomes "Look below" | A, 0 | A, 0 |
| K25 | "it may be this entry's mutant under another key" becomes "it is never ..." | A, 0 | A, 0 |
| K26 | the re-keying made unconditional on the reason | A, 0 | A, 0 |
| K27 | an unreadable survivor "is not this entry's mutant: never leave" | A, 0 | A, 0 |
| K28 | "When no survivor above is this entry's mutant" becomes "Whatever is printed above" | A, 0 | A, 0 |
| K29 | "because a test killed it" put after "no longer survives" | A, 0 | A, 0 |
| K30 | "not against the code line printed there alone" becomes "that is against ..." | A, 0 | A, 0 |
| K31 | a note appended when the gate holds a survivor it cannot read | B, 0 | B, 0 |
| K32 | the survivors the gate cannot read printed below the stale lines | B, 0 | B, 0 |
| K33 | the exception for a file that changed removed | A, 0 | A, 0 |
| K34 | "If it describes that mutant and no longer holds, this entry is wrong: remove it." removed | A, 0 | A, 0 |

Twenty-nine of the thirty-four are caught by a new test and by nothing that
existed, in both gates: K1 to K6, K9, K11 and K14 to K34. The other
direction, on the repaired gate: the third test, in which a refused
entry and an entry that claims its survivor each print no stale line; and
every test that existed still passes.

That only strings changed: with every string constant blanked, each gate
parses to the tree `5cb42fa`'s parses to (Appendix A), and with `not in
faulted` made `in faulted` the same comparison prints `DIFFERENT`.

## Rule 9: how MapTool does this

MapTool does not solve this: no tabletop function is designed, and the
verifier's search of `~/dev/RPTool` for a mutation tool in its build and
configuration found none.

## Deviations

| What was intended | What happened | Why |
|---|---|---|
| The plan's D2: one sentence after the entry's description, as signed off | Eleven sentences, the fourth text | Task 5 allows a finding to become a change to the text. The first cold read found five places where D2's words lead wrong; the second, five passages read twice; the review, one false clause and one that said too much. |
| Appendix A: test A holds seven phrases | It holds eleven whole sentences and the line's end; test B has a third run | The reviewer made nine edits to the unheld part of the line and every suite stayed green. |
| The ticket ruled out the header sentence of `tools/ts-mutation-equivalents.txt` | It is corrected; the ticket's Done 6 | The first cold reader named it as in conflict with the gate's line, and the owner ruled it in. |
| The ticket ruled out the survivor the gate cannot read | The line names it | The first cold reader showed the line opening the way to removing a sound entry beside one. |
| D10: the cold reader is given the printed runs and the entries, "nothing else" | It was also given the head of each adjudication file, and the harness gave it project context unasked | Without the head, `offset`, `nth`, `copies` and `span` mean nothing to a reader. The harness's part could not be withheld. |
| Task 5 once | Twice | The text the first read produced had been read by nobody. |
| Fifteen breaks | Thirty-four | Each sentence added to the line got its own, and the reviewer's nine were added. |
| D3's comment, citing SPEC-020 | Another reason, citing VTT-308 | The reviewer showed "cannot tell" false, and SPEC-020 says nothing of the line until it is next written. |
| No test file but the two unit files gains anything; no existing line of them changes | The docstring of class `EntryCheck` lost its first two lines and gained two | The claim Done 4 is about stood there a third time. |
| D9: `task check` once, on the final tree | Run before `7e19cde`, on the staged change set; D9's run on the tree after SPEC-020's commit is still to come when this is written | Phase 3 of the cycle runs the gate before the commit. |
| Task 8: the report carries the table of shapes and F1 to F6 | It points at the plan's table and gives the verification's five items | The table stands in the plan, which rides in `7e19cde`; one home. |

## What could not be established

- Whether the committed text leads a reader right. Two cold readers and the
  reviewer read three earlier texts; M1's clause and S1's three sentences
  have been read by their writer alone.
- How cold the cold readers were. Each reported the project context the
  harness gave it. The first said its caution about deleting may be the
  higher for it, and the second could not rule out its influence on four
  runs, G1, T1, G4 and T4.
- How often an entry reaches the stale line by a moved key and how often by a
  kill. No red run of either real gate was read; the line is known from
  fixtures, and a green `task check` prints no stale line.
- Whether "no longer survives" can be kept from reading as "was killed". Both
  cold readers took it so at first. Existing tests count by the phrase.

## What was deliberately left out, and where it went

- **SPEC-020's paragraph and its id.** The plan's Task 9, a commit of its own
  after this report; the draft is in the plan's writer-written section.
- **A gate that says what the run recorded.** Both cold readers asked for the
  status of the mutant under the entry's key, and the second for the line to
  say which case is on the screen. gremlins and Stryker both report a killed
  mutant with its place, and neither gate looks it up for this line. The
  owner ruled it a ticket of its own, later; no ticket is written. It is
  recorded here and in the plan's writer-written section.
- **The advice printed once.** The reviewer measured the third text's line at
  1,512 characters, 122 of them naming the entry, and three stale entries at
  4,536 of 5,211 characters of the output. Each gate prints its timeout
  advice once. The same ruling, the same ticket.
- **A timed-out mutant as a place the entry's mutant may be** (N2). Left; no
  ticket.
- **Other lines of the output**, which the cold readers found wanting and the
  ticket does not touch: `ok  p: 0 survivor(s)` on a run that exits 1; no
  line in the TypeScript gate saying a file was measured; the VTT-304 line
  not saying which of its "or" arms happened. No ticket.
- **The head of `tools/mutation-equivalents.txt` does not say that `offset`
  counts from 1.** SPEC-020 does. No ticket.
- **An entry whose package the Go run does not measure, and a mutant no
  longer made.** The ticket rules a line of their own out. The committed
  line's last sentence covers them without naming them: the mutation is
  made by hand, and if no test kills it the entry is left and the reader
  finds out why the run did not report its mutant.
- **SPEC-020 does not say that a Go entry whose package the run does not
  measure is stale**; `tools/mutation-scope.md` does. Noticed by the
  verifier, outside the ticket. No ticket.

## Appendix A: the instruments and what they printed

The session's scratch directory does not outlive the session, so each is
given whole.

**The text, in one place** (`texts2.py`): the gate edit, the tests' sentences
and the break runner all read it from here.

    """The stale line after the cold read, in one place: the gate edit, the tests'
    phrases and the break runner all read it from here."""
    import re

    # What follows "no longer survives " on the stale line, identical in both gates,
    # sentence by sentence. The tests hold every one of them.
    SENTENCES = (
        "under that key (the fields and code just named): this run reports no surviving or "
        "timed-out mutant under it, and that alone does not say the mutant was killed.",
        "This run found this entry's code in that file on as many lines as the entry records, so "
        "a survivor printed on other code is on another line than that code, unless the file "
        "changed between that count and the measuring of its mutants.",
        "Look above, in this same run's output, for a survivor in that file.",
        "If one is printed unadjudicated, with an entry ready to paste under it, compare its "
        "header and code line with this entry's: it may be this entry's mutant under another key.",
        "Read this entry's reason against that mutant where it stands in the file, not against "
        "the code line printed there alone.",
        "If the reason describes that mutant and holds there, re-key this entry instead of "
        "deleting it: give it the header and code line printed there.",
        "If it describes that mutant and no longer holds, this entry is wrong: remove it.",
        "If it does not describe that mutant, the survivor is another mutant.",
        "In both of those cases that survivor needs a test or an entry of its own.",
        "If one is printed as a survivor the gate cannot read (VTT-304), it may be this entry's "
        "mutant too: leave this entry as it is until a run can read that survivor.",
        "When no survivor above is this entry's mutant, make the mutation by hand and run the "
        "tests, since this run's output does not show a kill: if a test now kills the mutant, "
        "remove the entry so it cannot pre-approve a future survivor; if none does, leave the "
        "entry and find out why this run did not report its mutant.",
    )
    SUFFIX = " ".join(SENTENCES)

    HEAD = {"go": "check:mutation: {equivalents_path} lists {describe(e)}, which no longer survives ",
            "ts": "check:ts-mutation: {describe(e)} is adjudicated equivalent but no longer survives "}

    POINTER = SENTENCES

    START = re.compile(r'        print\(f"check:(?:ts-)?mutation: [^\n]*no longer survives[^\n]*\n(?:              f"[^\n]*\n)*              file=err\)\n')


    def render(gate, suffix=SUFFIX, head=None, extra=""):
        """The stale print statement, wrapped the one way this function wraps it."""
        assert "{" not in suffix and '"' not in suffix and "\\" not in suffix
        head = HEAD[gate] if head is None else head
        words = suffix.split(" ")
        lines, cur = [], ""
        for w in words:
            if cur and len(cur) + 1 + len(w) > 78:
                lines.append(cur + " ")
                cur = w
            else:
                cur = (cur + " " + w) if cur else w
        lines.append(cur)
        out = '        print(f"' + head + '"\n'
        for i, l in enumerate(lines):
            last = i == len(lines) - 1
            out += '              f"' + l + '"' + ((extra + ",") if last else "") + "\n"
        return out + "              file=err)\n"


    def stale_block(src):
        m = START.findall(src)
        assert len(m) == 1, len(m)
        return m[0]

**The break runner** (`breaks2.py <clone>`). The clone's `HEAD` must be
`5cb42fa` with the change set in its working tree: K1 reads the old text from
`HEAD`, so on a clone at `7e19cde` it changes nothing and the runner stops at its
own assertion.

    """Each deliberate break of the gates as they stand in the clone, and which tests it reds.
    The gate file is rewritten from its own text each time and put back afterwards.
    Usage: breaks2.py <clone>   (the clone holds the working tree's change set)"""
    import os, re, subprocess, sys
    sys.dont_write_bytecode = True
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    import texts2 as T
    clone = sys.argv[1]
    env = dict(os.environ, PYTHONDONTWRITEBYTECODE="1")
    G = {"go": ("tools/check-mutation.py", ["tools/check_mutation_test.py", "tools/check_mutation_qa_test.py"]),
         "ts": ("tools/check-ts-mutation.py", ["tools/check_ts_mutation_test.py", "tools/check_ts_mutation_qa_test.py"])}
    NEW = {"test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file": "A",
           "test_the_stale_line_is_the_same_with_and_without_a_survivor_beside_it": "B",
           "test_a_refused_entry_and_a_claimed_one_get_no_stale_line": "C"}

    def reds(tests):
        out = set()
        for t in tests:
            p = subprocess.run([sys.executable, t], cwd=clone, capture_output=True, text=True, env=env)
            assert re.search(r"^Ran (\d+) tests", p.stderr, re.M), p.stderr[-600:]
            out |= {a for a, _ in re.findall(r"^(?:FAIL|ERROR): (\S+) \((\S+?)\)", p.stderr, re.M)}
        return out

    def stale_print(src, gate, head=None, suffix=None, extra=""):
        block = T.stale_block(src)
        assert block == T.render(gate), "the clone's stale print is not the rendered text"
        new = T.render(gate, T.SUFFIX if suffix is None else suffix, head, extra)
        return src.replace(block, new)

    def sfx(old, new):
        assert T.SUFFIX.count(old) == 1, old
        return T.SUFFIX.replace(old, new)

    def between(a, b):
        return T.SUFFIX[T.SUFFIX.index(a):T.SUFFIX.index(b)]

    def old_text(gate):
        path = G[gate][0]
        src = subprocess.run(["git", "show", "HEAD:" + path], cwd=clone, capture_output=True, text=True, check=True).stdout
        return T.stale_block(src)

    def k_swap_loops(src, gate):
        start = src.index("    # A stale entry is one no surviving or timed-out mutant has the key of")
        end = src.index("    if failed:\n        return 1" if gate == "go" else "    if fail:\n        return 1")
        block = src[start:end]
        rest = src[:start] + src[end:]
        anchor = ("    for name, (location, mutator), key, sharing, named, copies in unadjudicated:" if gate == "go"
                  else "    for key, (path, where, mutator, replacement, _) in unclaimed:")
        assert rest.count(anchor) == 1
        return rest.replace(anchor, block + anchor)

    def k_unreadable_last(src, gate):
        head = ("    for name, location, mutator in unreadable:\n" if gate == "go"
                else "    for path, where, mutator, replacement, _ in unreadable:\n")
        nxt = ("    for name, (location, mutator), key, sharing, named, copies in unadjudicated:" if gate == "go"
               else "    for key, (path, where, mutator, replacement, _) in unclaimed:")
        assert src.count(head) == 1 and src.count(nxt) == 1
        a, b = src.index(head), src.index(nxt)
        block, rest = src[a:b], src[:a] + src[b:]
        end = "    if failed:\n        return 1" if gate == "go" else "    if fail:\n        return 1"
        assert rest.count(end) >= 1
        i = rest.index(end, rest.index("    for e in stale:"))
        return rest[:i] + block + rest[i:]

    def once(src, old, new):
        assert src.count(old) == 1, (src.count(old), old)
        return src.replace(old, new)

    POINTERS = between("Look above", "When no survivor above")
    REMOVE = T.SUFFIX[T.SUFFIX.index("When no survivor above"):]
    STATIC = between("This run found this entry's code", "Look above")
    READ = between("Read this entry's reason", "If the reason describes")
    UNREADABLE = between("If one is printed as a survivor the gate cannot read", "When no survivor above")
    FAILING = " " + between("If it describes that mutant and no longer holds", "If one is printed as a survivor").rstrip()
    BY_HAND = "make the mutation by hand and run the tests, since this run's output does not show a kill: "
    BREAKS = [
     ("K0  rebuilt unchanged (the harness itself)", lambda s, g: stale_print(s, g)),
     ("K1  the stale text of HEAD put back", lambda s, g: once(s, T.stale_block(s), old_text(g))),
     ("K2  every sentence from 'Look above' to 'When no survivor' removed", lambda s, g: stale_print(s, g, suffix=sfx(POINTERS, ""))),
     ("K3  removal no longer conditional on a kill", lambda s, g: stale_print(s, g, suffix=sfx(BY_HAND + "if a test now kills the mutant, remove the entry", "remove the entry"))),
     ("K4  the removal sentence moved before the pointers", lambda s, g: stale_print(s, g, suffix=sfx(POINTERS + REMOVE, REMOVE + " " + POINTERS.rstrip()))),
     ("K5  the line asserts a kill", lambda s, g: stale_print(s, g, suffix=sfx("and that alone does not say the mutant was killed.", "so the mutant was killed."))),
     ("K6  the old condition added back", lambda s, g: stale_print(s, g, suffix=T.SUFFIX + " If the code was edited, re-key it.")),
     ("K7  a counted phrase of the survivor line added", lambda s, g: stale_print(s, g, suffix=sfx("Look above,", ("A survivor not adjudicated may stand above. " if g == "go" else "A SURVIVED line may stand above. ") + "Look above,"))),
     ("K8  the stale line names no entry", lambda s, g: stale_print(s, g, head=("check:mutation: {equivalents_path} lists an entry, which no longer survives " if g == "go" else "check:ts-mutation: an entry is adjudicated equivalent but no longer survives "))),
     ("K9  the gate picks the text from its survivors", lambda s, g: stale_print(s, g, extra=' + (" NOTE: a survivor stands unadjudicated in that file." if ' + ("unadjudicated" if g == "go" else "unclaimed") + ' else "")')),
     ("K10 refused entries left in the stale list", lambda s, g: once(s, "    stale = [e for e in stale if id(e) not in faulted]\n", "")),
     ("K11 stale lines printed before the survivors", k_swap_loops),
     ("K12 a second 'no longer survives' on the line", lambda s, g: stale_print(s, g, suffix=sfx("Look above,", "If it no longer survives, look above,"))),
     ("K13 every entry is called stale", lambda s, g: once(s, "    stale = [e for e in stale if id(e) not in faulted]\n", "    stale = list(entries)\n")),
     ("K14 'in that file' dropped from the pointer", lambda s, g: stale_print(s, g, suffix=sfx("for a survivor in that file.", "for a survivor."))),
     ("K15 'instead of deleting it' dropped", lambda s, g: stale_print(s, g, suffix=sfx("re-key this entry instead of deleting it:", "re-key this entry:"))),
     ("K16 the sentence on an unreadable survivor removed", lambda s, g: stale_print(s, g, suffix=sfx(UNREADABLE, ""))),
     ("K17 what the run found of the entry's code removed", lambda s, g: stale_print(s, g, suffix=sfx(STATIC, ""))),
     ("K18 the branch for a reason that does not hold removed", lambda s, g: stale_print(s, g, suffix=sfx(FAILING, ""))),
     ("K19 'read the reason against that mutant' removed", lambda s, g: stale_print(s, g, suffix=sfx(READ, ""))),
     ("K20 'printed unadjudicated, with an entry ...' becomes 'printed'", lambda s, g: stale_print(s, g, suffix=sfx("If one is printed unadjudicated, with an entry ready to paste under it, compare", "If one is printed, compare"))),
     ("K21 'if none does, leave the entry ...' removed", lambda s, g: stale_print(s, g, suffix=sfx("; if none does, leave the entry and find out why this run did not report its mutant.", "."))),
     ("K22 'make the mutation by hand ...' removed", lambda s, g: stale_print(s, g, suffix=sfx(BY_HAND, ""))),
     ("K23 'compare its header and code line' becomes 'compare it'", lambda s, g: stale_print(s, g, suffix=sfx("compare its header and code line with this entry's:", "compare it with this one:"))),
     ("K24 'Look above' becomes 'Look below'", lambda s, g: stale_print(s, g, suffix=sfx("Look above,", "Look below,"))),
     ("K25 'it may be ... under another key' becomes 'it is never'", lambda s, g: stale_print(s, g, suffix=sfx("it may be this entry's mutant under another key", "it is never this entry's mutant under another key"))),
     ("K26 re-key whether or not the reason holds", lambda s, g: stale_print(s, g, suffix=sfx("If the reason describes that mutant and holds there, re-key", "Whether or not the reason holds there, re-key"))),
     ("K27 an unreadable survivor 'is not this entry's mutant'", lambda s, g: stale_print(s, g, suffix=sfx("it may be this entry's mutant too: leave", "it is not this entry's mutant: never leave"))),
     ("K28 'When no survivor above is ...' becomes 'Whatever is printed above'", lambda s, g: stale_print(s, g, suffix=sfx("When no survivor above is this entry's mutant", "Whatever is printed above"))),
     ("K29 the opening words assert a kill", lambda s, g: stale_print(s, g, head=T.HEAD[g] + "because a test killed it ")),
     ("K30 'not against the code line' becomes 'that is against'", lambda s, g: stale_print(s, g, suffix=sfx("not against the code line printed there alone", "that is against the code line printed there alone"))),
     ("K31 a note appended when the gate holds an unreadable survivor", lambda s, g: stale_print(s, g, extra=' + (" NOTE: a survivor above could not be read." if unreadable else "")')),
     ("K32 the unreadable survivors printed below the stale lines", k_unreadable_last),
     ("K33 the exception for a file that changed removed", lambda s, g: stale_print(s, g, suffix=sfx(", unless the file changed between that count and the measuring of its mutants", ""))),
     ("K34 the branch for a reason that no longer holds removed", lambda s, g: stale_print(s, g, suffix=sfx(" If it describes that mutant and no longer holds, this entry is wrong: remove it.", ""))),
    ]
    for gate in ("go", "ts"):
        path, tests = G[gate]
        full = os.path.join(clone, path)
        patched = open(full).read()
        assert reds(tests) == set(), "baseline not green"
        print("%s: baseline green" % gate, flush=True)
        for label, edit in BREAKS:
            try:
                broken = edit(patched, gate)
                assert broken != patched or label.startswith("K0"), "the edit changed nothing: " + label
                open(full, "w").write(broken)
                r = reds(tests)
            finally:
                open(full, "w").write(patched)
            new = "".join(sorted(NEW[t] for t in r if t in NEW)) or "-"
            old = sorted(t for t in r if t not in NEW)
            print("  %-62s new: %-4s existing: %d%s" % (label, new, len(old), "" if not old else "  (" + "; ".join(old[:2]) + (" ..." if len(old) > 2 else "") + ")"), flush=True)
        assert reds(tests) == set()
    print("BREAKS_DONE", flush=True)

**Its output**, whole, on a clone at `5cb42fa` holding `7e19cde`'s files:

    go: baseline green
      K0  rebuilt unchanged (the harness itself)                     new: -    existing: 0
      K1  the stale text of HEAD put back                            new: A    existing: 0
      K2  every sentence from 'Look above' to 'When no survivor' removed new: A    existing: 0
      K3  removal no longer conditional on a kill                    new: A    existing: 0
      K4  the removal sentence moved before the pointers             new: A    existing: 0
      K5  the line asserts a kill                                    new: A    existing: 0
      K6  the old condition added back                               new: A    existing: 0
      K7  a counted phrase of the survivor line added                new: A    existing: 4  (test_a_timeout_on_the_other_copy_leaves_the_entry_stale; test_an_entry_left_over_after_the_timeouts_is_stale ...)
      K8  the stale line names no entry                              new: A    existing: 0
      K9  the gate picks the text from its survivors                 new: AB   existing: 0
      K10 refused entries left in the stale list                     new: C    existing: 3  (test_a_duplicated_entry_fails_the_run_and_is_not_told_to_remove_itself; test_a_refused_entry_is_not_also_told_to_remove_the_entry ...)
      K11 stale lines printed before the survivors                   new: A    existing: 0
      K12 a second 'no longer survives' on the line                  new: A    existing: 2  (test_an_entry_left_over_after_the_timeouts_is_stale; test_entries_on_two_copies_and_one_survivor_leave_one_stale)
      K13 every entry is called stale                                new: C    existing: 40  (test_a_copies_refusal_is_reported_before_the_run_and_the_run_goes_ahead; test_a_duplicated_entry_fails_the_run_and_is_not_told_to_remove_itself ...)
      K14 'in that file' dropped from the pointer                    new: A    existing: 0
      K15 'instead of deleting it' dropped                           new: A    existing: 0
      K16 the sentence on an unreadable survivor removed             new: A    existing: 0
      K17 what the run found of the entry's code removed             new: A    existing: 0
      K18 the branch for a reason that does not hold removed         new: A    existing: 0
      K19 'read the reason against that mutant' removed              new: A    existing: 0
      K20 'printed unadjudicated, with an entry ...' becomes 'printed' new: A    existing: 0
      K21 'if none does, leave the entry ...' removed                new: A    existing: 0
      K22 'make the mutation by hand ...' removed                    new: A    existing: 0
      K23 'compare its header and code line' becomes 'compare it'    new: A    existing: 0
      K24 'Look above' becomes 'Look below'                          new: A    existing: 0
      K25 'it may be ... under another key' becomes 'it is never'    new: A    existing: 0
      K26 re-key whether or not the reason holds                     new: A    existing: 0
      K27 an unreadable survivor 'is not this entry's mutant'        new: A    existing: 0
      K28 'When no survivor above is ...' becomes 'Whatever is printed above' new: A    existing: 0
      K29 the opening words assert a kill                            new: A    existing: 0
      K30 'not against the code line' becomes 'that is against'      new: A    existing: 0
      K31 a note appended when the gate holds an unreadable survivor new: B    existing: 0
      K32 the unreadable survivors printed below the stale lines     new: B    existing: 0
      K33 the exception for a file that changed removed              new: A    existing: 0
      K34 the branch for a reason that no longer holds removed       new: A    existing: 0
    ts: baseline green
      K0  rebuilt unchanged (the harness itself)                     new: -    existing: 0
      K1  the stale text of HEAD put back                            new: A    existing: 0
      K2  every sentence from 'Look above' to 'When no survivor' removed new: A    existing: 0
      K3  removal no longer conditional on a kill                    new: A    existing: 0
      K4  the removal sentence moved before the pointers             new: A    existing: 0
      K5  the line asserts a kill                                    new: A    existing: 0
      K6  the old condition added back                               new: A    existing: 0
      K7  a counted phrase of the survivor line added                new: A    existing: 15  (test_a_wrong_span_is_not_refused_statically_and_claims_nothing; test_an_entry_on_a_copy_whose_mutant_was_killed_is_stale ...)
      K8  the stale line names no entry                              new: A    existing: 1  (test_an_entry_whose_mutant_was_killed_fails_the_gate)
      K9  the gate picks the text from its survivors                 new: AB   existing: 0
      K10 refused entries left in the stale list                     new: C    existing: 2  (test_a_refused_entry_is_not_also_told_to_remove_the_entry; test_a_suspect_key_is_not_also_told_to_delete_itself)
      K11 stale lines printed before the survivors                   new: A    existing: 0
      K12 a second 'no longer survives' on the line                  new: A    existing: 13  (test_a_wrong_span_is_not_refused_statically_and_claims_nothing; test_an_entry_on_a_copy_whose_mutant_was_killed_is_stale ...)
      K13 every entry is called stale                                new: C    existing: 42  (test_a_form_feed_in_a_comment_does_not_move_the_line; test_a_hash_inside_code_replacement_and_reason_is_text ...)
      K14 'in that file' dropped from the pointer                    new: A    existing: 0
      K15 'instead of deleting it' dropped                           new: A    existing: 0
      K16 the sentence on an unreadable survivor removed             new: A    existing: 0
      K17 what the run found of the entry's code removed             new: A    existing: 0
      K18 the branch for a reason that does not hold removed         new: A    existing: 0
      K19 'read the reason against that mutant' removed              new: A    existing: 0
      K20 'printed unadjudicated, with an entry ...' becomes 'printed' new: A    existing: 0
      K21 'if none does, leave the entry ...' removed                new: A    existing: 0
      K22 'make the mutation by hand ...' removed                    new: A    existing: 0
      K23 'compare its header and code line' becomes 'compare it'    new: A    existing: 0
      K24 'Look above' becomes 'Look below'                          new: A    existing: 0
      K25 'it may be ... under another key' becomes 'it is never'    new: A    existing: 0
      K26 re-key whether or not the reason holds                     new: A    existing: 0
      K27 an unreadable survivor 'is not this entry's mutant'        new: A    existing: 0
      K28 'When no survivor above is ...' becomes 'Whatever is printed above' new: A    existing: 0
      K29 the opening words assert a kill                            new: A    existing: 0
      K30 'not against the code line' becomes 'that is against'      new: A    existing: 0
      K31 a note appended when the gate holds an unreadable survivor new: B    existing: 0
      K32 the unreadable survivors printed below the stale lines     new: B    existing: 0
      K33 the exception for a file that changed removed              new: A    existing: 0
      K34 the branch for a reason that no longer holds removed       new: A    existing: 0
    BREAKS_DONE

**Only strings changed** (`only-strings.py`, run from the repository root
before the commit, with the change in the working tree, so that `HEAD` was
`5cb42fa`):

    """Appendix B of the plan: do the two gates differ from HEAD in anything but string constants?"""
    import ast, subprocess

    class Blank(ast.NodeTransformer):
        def visit_Constant(self, node):
            return ast.copy_location(ast.Constant(value=""), node) if isinstance(node.value, str) else node

    def shape(src):
        return ast.dump(Blank().visit(ast.parse(src)), include_attributes=False)

    for path in ("tools/check-mutation.py", "tools/check-ts-mutation.py"):
        old = subprocess.run(["git", "show", "HEAD:" + path], capture_output=True, text=True, check=True).stdout
        new = open(path).read()
        print(path, "changed" if old != new else "UNCHANGED", "identical" if shape(old) == shape(new) else "DIFFERENT")

    tools/check-mutation.py changed identical
    tools/check-ts-mutation.py changed identical

**The two gates print one sentence** (`same-sentence.py`):

    """Do the two gates' stale lines carry one and the same sentence after the entry's description?"""
    import ast, re
    def stale_text(path):
        tree = ast.parse(open(path).read())
        hits = []
        for node in ast.walk(tree):
            if isinstance(node, ast.JoinedStr):
                text = "".join(v.value if isinstance(v, ast.Constant) else "{}" for v in node.values)
                if "no longer survives" in text:
                    hits.append(text)
        assert len(hits) == 1, (path, len(hits))
        return hits[0]
    go, ts = stale_text("tools/check-mutation.py"), stale_text("tools/check-ts-mutation.py")
    tail = lambda t: t[t.index("no longer survives"):]
    print("go :", go[:70].replace("\n", " "))
    print("ts :", ts[:70].replace("\n", " "))
    print("same sentence from 'no longer survives' on:", tail(go) == tail(ts))
    print("one line each:", "\n" not in go and "\n" not in ts)
    print(tail(go))

What it prints at `7e19cde`, less the sentence itself:

    same sentence from 'no longer survives' on: True
    one line each: True

**Done 4's search**, over both files, run before the commit, when `HEAD` was
`5cb42fa`:

    import re, subprocess
    pat = re.compile(r"before\s+(?:#\s*)?the\s+(?:#\s*)?report\s+(?:#\s*)?is\s+(?:#\s*)?read")
    for path in ("tools/check-ts-mutation.py", "tools/check_ts_mutation_test.py"):
        old = subprocess.run(["git", "show", "HEAD:" + path], capture_output=True, text=True, check=True).stdout
        print(path, "at HEAD:", len(pat.findall(old)), "now:", len(pat.findall(open(path).read())))

    tools/check-ts-mutation.py at HEAD: 2 now: 0
    tools/check_ts_mutation_test.py at HEAD: 1 now: 0

**The cold read's runs** (`build-runs.py <repo> <out.md>`):

    """Phase 4a, the cold read: six runs per gate, built with the QA harnesses'
    fixtures against the gates in the working tree. Writes runs.md: for each run the
    entry as it stands in the adjudication file and everything the gate printed.
    Temporary directory names are replaced by the real files' names; nothing else
    of the output is touched. Usage: build-runs.py <repo> <out.md>"""
    import importlib.util, os, re, sys, tempfile
    sys.dont_write_bytecode = True
    REPO, OUT = sys.argv[1], sys.argv[2]
    TOOLS = os.path.join(REPO, "tools")

    def load(name, fname):
        spec = importlib.util.spec_from_file_location(name, os.path.join(TOOLS, fname))
        mod = importlib.util.module_from_spec(spec)
        sys.modules[name] = mod
        spec.loader.exec_module(mod)
        return mod

    gq = load("goqa", "check_mutation_qa_test.py")
    tq = load("tsqa", "check_ts_mutation_qa_test.py")
    CB, CODE, OTHER, FIRST, SECOND = gq.CB, gq.CODE, gq.OTHER, gq.FIRST, gq.SECOND
    doc = []

    def head(path, upto):
        lines = open(os.path.join(REPO, path)).read().split("\n")
        n = next(i for i, l in enumerate(lines) if upto in l)
        while lines[n].startswith("#") and not set(lines[n]) <= set("# -"):
            n += 1
        return "\n".join(lines[:n])

    def block(text):
        return "```\n" + text.rstrip("\n") + "\n```\n"

    def emit(title, entry_text, printed, exit_code):
        doc.append("### %s\n\nThe gate exited %d.\n\nThe entry, as it stands in the adjudication file:\n\n%s\n"
                   "Everything the gate printed:\n\n%s" % (title, exit_code, block(entry_text), block(printed)))

    GO_R1 = "`a > b` -> `>=`: when a equals b the branch adds i to x and the else arm adds the same i, so x is the same either way."
    GO_R2 = "`c > d` -> `>=`: when c equals d the branch adds i to x and the else arm adds the same i, so x is the same either way."

    def go(title, files, eq, plan, prep=None):
        with tempfile.TemporaryDirectory() as root:
            for rel, data in files.items():
                gq.write(root, rel, data)
            if prep:
                plan = prep(root)
            r = gq.gate(root, eq, plan)
            printed = (r.out + r.err).replace(os.path.join(root, "equivalents.txt"), "tools/mutation-equivalents.txt")
            assert root not in printed, printed
        emit(title, eq, printed, r.code)

    doc.append("## The Go gate, `check:mutation`\n\nThe head of `tools/mutation-equivalents.txt`, which every entry stands under:\n\n"
               + block(head("tools/mutation-equivalents.txt", "AN ENTRY NAMES CODE, NOT A LINE")))
    e1 = gq.entry("p", "f.go", CB, FIRST, 1, CODE, reason=GO_R1)
    go("Run G1", {"p/f.go": gq.f_go()}, e1, {"./p/": {"killed": [(CB, gq.at("f.go", CODE, FIRST))]}})
    go("Run G2", {"p/f.go": gq.f_go(inner=CODE, outer=CODE)}, gq.entry("p", "f.go", CB, FIRST, 2, CODE, reason=GO_R1, nth=1),
       {"./p/": {"killed": [(CB, gq.at("f.go", CODE, FIRST, 1))], "lived": [(CB, gq.at("f.go", CODE, FIRST, 2))]}})
    go("Run G3", {"p/f.go": gq.f_go()}, gq.entry("p", "f.go", CB, SECOND, 1, CODE, reason=GO_R2),
       {"./p/": {"lived": [(CB, gq.at("f.go", CODE, FIRST))]}})
    def moved(root):
        old = gq._locate(root, "./p/", gq.at("f.go", CODE, FIRST))
        gq.write(root, "p/f.go", gq.f_go(inner=OTHER, outer=CODE))
        return {"./p/": {"lived": [(CB, old)]}}
    go("Run G4", {"p/f.go": gq.f_go(inner=CODE)}, e1, None, prep=moved)
    loop = "for i := 0; i < 3; i++ {"
    go("Run G5", {"p/f.go": gq.f_go()}, e1, {"./p/": {"lived": [(CB, gq.at("f.go", loop, loop.index("<") + 1))]}})
    go("Run G6", {"p/f.go": gq.f_go()}, e1, {"./p/": {"lived": [(CB, "f.go:5:99")]}})

    A, BASE, CMP, CMP_CODE, DUP = tq.A, tq.BASE, tq.CMP, tq.CMP_CODE, tq.DUP
    TS_EQ = "`a > b` -> `a >= b`: when a equals b the function returns 1 either way, since the caller clamps 0 to 1."
    TS_CE = "`a > b` -> `true`: the caller only ever passes a greater than b, so the condition is already always true."

    def ts(title, s, entry_text):
        r = s.run()
        assert not re.search(r"/(var|tmp|private)/", r.text), r.text
        emit(title, entry_text, r.text, r.rc)

    def scn(lines, muts, ent):
        s = tq.Scn().file(A, lines)
        for m in muts:
            s.mut(A, *m[:5], **(m[5] if len(m) > 5 else {}))
        mutator, offset, copies, replacement, code, kw = ent
        s.ent(A, mutator, offset, copies, replacement, code, **kw)
        text = tq.entry_block(A, mutator, offset, copies, replacement, code, **kw)
        return s, text

    doc.append("## The TypeScript gate, `check:ts-mutation`\n\nThe head of `tools/ts-mutation-equivalents.txt`, which every entry stands under:\n\n"
               + block(head("tools/ts-mutation-equivalents.txt", "AN ENTRY NAMES CODE, NOT A LINE")))
    EQ = "EqualityOperator"; CE = "ConditionalExpression"
    ts("Run T1", *scn(BASE, [(EQ, "Killed", "a >= b", 2, 7)], (EQ, 5, 1, "a >= b", CMP_CODE, {"reason": TS_EQ})))
    ts("Run T2", *scn(DUP, [(EQ, "Killed", "a >= b", 2, 7), (EQ, "Survived", "a >= b", 5, 7)],
                      (EQ, 5, 2, "a >= b", CMP_CODE, {"reason": TS_EQ, "nth": 1})))
    ts("Run T3", *scn(BASE, [(EQ, "Survived", "a >= b", 2, 7)], (EQ, 5, 1, "a >= b", CMP_CODE, {"reason": TS_EQ, "span": 6})))
    edited = "  if (a > c) {"
    lines = BASE[:1] + [edited] + BASE[2:] + [CMP]
    ts("Run T4", *scn(lines, [(CE, "Survived", "true", 2, 7), (CE, "Killed", "true", len(lines), tq.col(CMP, "a > b"))],
                      (CE, 5, 1, "true", CMP_CODE, {"reason": TS_CE})))
    ts("Run T5", *scn(BASE, [(CE, "Survived", "true", 5, 7), (CE, "Killed", "true", 2, 7)],
                      (CE, 5, 1, "true", CMP_CODE, {"reason": TS_CE})))
    ts("Run T6", *scn(BASE, [(EQ, "Survived", "a >= b", 2, 7, {"end": tq.NO_END})], (EQ, 5, 1, "a >= b", CMP_CODE, {"reason": TS_EQ})))

    open(OUT, "w").write("\n".join(doc))
    print("wrote", OUT, sum(d.startswith("### ") for d in doc), "runs")

**The brief each cold reader was given**, less the paths:

    You are a cold reader. Your whole job is to read one file and say what you
    would do, as the person who maintains two "adjudication files" for a
    project's mutation-testing gates.

    The file holds twelve gate runs, six of a Go gate (G1 to G6) and six of a
    TypeScript gate (T1 to T6). Before each group stands the head of the
    adjudication file, which explains the format of an entry. For each run you
    are shown one entry as it stands in the adjudication file, and everything
    the gate printed when it failed.

    Read that file and nothing else: no source code, no tests, no
    documentation, no git history, no web. You have no access to the source
    files the runs are about, and that is deliberate: if you would need to look
    at a source file or at history to decide, say exactly what you would look at
    and what each possible finding would make you do. You are not being asked
    whether the gate is right or how it works inside. You are being asked what a
    careful maintainer does next when this is what the screen shows. Do not try
    to guess what answer is wanted. If the printed text leaves you unsure, say
    so plainly.

    For each of the twelve runs: (1) the act: leave the entry as it is, edit it
    (name each field or line and to what), delete it, or "look first" (name
    what you look at and, for each thing you might find, the act that follows);
    (2) why, quoting the words of the output or of the file head that led you
    there; (3) what you could not tell, and anything the output told you that
    you believe is wrong, misleading, or in conflict with another line of the
    same output or with the file head.

    Then, once: which sentence or phrase of the long line did you have to read
    more than once, and what did you first take it to mean? Is there any run
    where following that line's words literally would have led you to an act
    you think is wrong? Is there any run where that line did not help you at
    all, and what would you have needed?

**The second text**, read by the second cold reader, after "no longer
survives":

    under that key, the fields and code just named: this run reports no
    surviving or timed-out mutant under it, and that alone does not say the
    mutant was killed. The static check found this entry's code in that file as
    often as the entry records. Look above, in this same run's output, for a
    survivor in that file. If one is printed unadjudicated, with an entry ready
    to paste under it, compare that entry with this one field by field: it may
    be this entry's mutant under another key. Read this entry's reason against
    that mutant where it stands in the file, not against the printed line alone.
    If the reason describes that mutant and holds there, re-key this entry: give
    it the header and code line printed there and keep its reason, instead of
    deleting it; if it does not, this entry does not move to that survivor,
    which is then a mutant nobody has judged. If one is printed as a survivor
    the gate cannot read (VTT-304), it may be this entry's mutant too: leave
    this entry as it is until a run can read that survivor. Only if no survivor
    above is this entry's mutant, and a test now kills the mutant, remove the
    entry so it cannot pre-approve a future survivor.

**The third text**, read by the reviewer:

    under that key (the fields and code just named): this run reports no
    surviving or timed-out mutant under it, and that alone does not say the
    mutant was killed. This run found this entry's code in that file on as many
    lines as the entry records, so a survivor printed on other code is on
    another line. Look above, in this same run's output, for a survivor in that
    file. If one is printed unadjudicated, with an entry ready to paste under
    it, compare its header and code line with this entry's: it may be this
    entry's mutant under another key. Read this entry's reason against that
    mutant where it stands in the file, not against the code line printed there
    alone. If the reason describes that mutant and holds there, re-key this
    entry instead of deleting it: give it the header and code line printed
    there. If it does not, that survivor is not this entry's and needs a test or
    an entry of its own. If one is printed as a survivor the gate cannot read
    (VTT-304), it may be this entry's mutant too: leave this entry as it is
    until a run can read that survivor. When no survivor above is this entry's
    mutant, make the mutation by hand and run the tests, since this run's output
    does not show a kill: if a test now kills the mutant, remove the entry so it
    cannot pre-approve a future survivor; if none does, leave the entry and find
    out why this run did not report its mutant.

The first text is the plan's D2; the fourth is in `texts2.py` above.

**The reviewer's report**, whole:

    # Reading review: a stale adjudication's line says where the same run decides its remedy

    Reviewed: `git diff HEAD` on `fix/a-stale-adjudications-line-says-where-the-run-decides`, base `5cb42fa`, eight files.
    All experiments ran in a clone at `.../scratchpad/stale/review/rc`; scripts and logs are in `.../scratchpad/stale/review/work`.

    ## Verdict

    One Must, five Should, eight Nit. The change does what it claims for rule 2 and for the tests it names. The Must is one clause of the printed line, added after the second cold read, that is false in a row of the plan's own table.

    In all twelve runs of `qa3/runs-body.md` a maintainer following the final wording literally does the right thing.

    ## Must

    ### M1. "so a survivor printed on other code is on another line" is false when the measured source is not the tree

    - **Where:** the stale `print` in `run` (`tools/check-mutation.py`) and in `check` (`tools/check-ts-mutation.py`): "This run found this entry's code in that file on as many lines as the entry records, so a survivor printed on other code is on another line."
    - **What is wrong:** the first half is the static check, which reads the tree. The survivor's code is read from the source the run measured (VTT-303; SPEC-020, "A survivor's key is read from the source the run measured"). The "so" joins two reads that SPEC-020 says may differ.
    - **TypeScript, report older than the tree** (`work/shapes.py`, shape "TS a"): the tree has the entry's `if (a > b) {` on line 2, the report's source has `if (a > c) {` on line 2. The gate printed `SURVIVED EqualityOperator at client/src/a.ts:2:7 (-> a >= c)` with `code: if (a > c) {`, then the stale line. The survivor on other code is on the same line.
    - **Go, tree edited after the static check and before a later package is read** (shape "GO G", the QA harness's `during` hook): printed `q f.go:6:8 ... SURVIVED and is not adjudicated` with `code: if a > b || c > d {`, then the stale line for the entry on `if a > b && c > d {`, which stood on line 6 when the static check ran.
    - **Why it matters:** this is the plan's own row "TypeScript: the report is older than the tree", and D1 rests on the line saying "nothing false in any row of the table". Followed literally in the TypeScript case, the line leads the reader to re-key a correctly re-keyed entry back onto code that no longer stands; the next run refuses it. Nothing is deleted. It is reachable by running the checker directly on the stored report, as the plan's Task 6 does.
    - **Smallest fix** (keeps the pinned phrase): "..., so a survivor printed on other code is on another line, unless that file changed after its mutants were measured." Pin the new clause.

    ## Should

    ### S1. "that survivor is not this entry's" asserts what the reader's judgement has not shown

    - **Where:** same line: "If it does not, that survivor is not this entry's and needs a test or an entry of its own."
    - **What is wrong:** "it does not" covers two cases: the reason does not describe the mutant, or it describes it and no longer holds. In the second the survivor is this entry's mutant and the entry is wrong. This is the untruth the second cold reader named on T4 ("calls the survivor 'a mutant nobody has judged', which in that case is untrue"), reworded and not removed.
    - **Consequence, by reading:** in the wrong-span shape with a reason that has stopped holding, the reader is sent to "make the mutation by hand"; no test kills it; "leave the entry and find out why this run did not report its mutant". The run did report it, and a refuted entry is left.
    - **Smallest fix:** "If it does not, do not re-key: that survivor needs a test or an entry of its own."

    ### S2. "the run cannot tell a killed mutant from a moved key" is a "does not", not a "cannot"

    - **Where:** the comment above each stale loop, and the class docstrings of `StaleLineTest` and `StaleLine` ("The run cannot tell a killed mutant from one whose key moved").
    - **Shown by command** (`work`, inline script): in the T1 fixture the report's mutant at `client/src/a.ts 2:7` has status `Killed` and `resolve` returns exactly the stale entry's key. In the G1 fixture gremlins' own line `KILLED CONDITIONALS_BOUNDARY at f.go:6:8` resolves to the stale entry's key. The real stored report records `{'Ignored': 6, 'Killed': 2868, 'Survived': 67, 'Timeout': 9}`, each with a location.
    - **Why it matters:** the comment is a DO NOT aimed at the next editor, and its premise is false. Both cold readers asked for exactly this fact ("what the run recorded for that mutant"). The printed line's own sentences ("that alone does not say", "this run's output does not show a kill") are true.
    - **Smallest fix:** "the gate does not read what became of the mutant under the key, and a survivor beside the entry may be nobody's, so the line names where the reader looks (SPEC-020)". Same change in the two class docstrings.

    ### S3. A third copy of "before the report is read" stands, and cites SPEC-020 for it

    - **Where:** `tools/check_ts_mutation_test.py`, docstring of class `EntryCheck`: "The static check of each entry against the tree, before the report is read (SPEC-020)."
    - **Shown by:** `git grep -n -i "before the report" -- . ':!docs/superpowers'` prints that one line. The ticket's "says twice" is true of `tools/check-ts-mutation.py` (regex: 2 at `5cb42fa`, 0 now), and Done 4 searches only that file.
    - **Smallest fix:** "..., before check() goes through the report's mutants (SPEC-020)." It costs one deleted line in `--numstat` of a file whose tests are otherwise untouched, so it is the owner's call; otherwise record it in the report as left.

    ### S4. The plan says the SPEC-020 draft paragraph is replaced, and nothing replaces it

    - **Where:** plan, writer's section: "What follows replaces D2's sentence, Appendix A's `POINTER` tuple, the break table and the draft paragraph for SPEC-020 further down". The section gives the text, the phrases and the breaks, and no paragraph.
    - **What stands:** the draft under "What SPEC-020 gains" still says "re-keyed once its reason is judged against the code printed there, and not deleted". The final line says the opposite ("not against the code line printed there alone"), and the first cold reader showed the old wording moves an entry onto the wrong mutant (G3). Task 9 says "The paragraph and the id below".
    - **Also:** the loop comment and both class docstrings point at SPEC-020 for what the stale line tells its reader. SPEC-020 says nothing of it until C3 (`grep -n -i stale` on it: two lines, neither about the line's content).
    - **Smallest fix:** write the replacement paragraph from the final text, or say in the writer's section that the draft is void and Task 9 drafts from the gate's line.

    ### S5. Test A pins 43% of the line, and the unpinned part carries the conditions

    - **Shown by** `work/mybreaks.py` on both gates, with three positive controls that red as they should (`work/mycontrols.py`). Each of these left all four suites green, new tests included:
      - "Look above" becomes "Look below" (the "where" of VTT-308).
      - "it may be this entry's mutant under another key" becomes "it is never ...".
      - "If the reason describes that mutant and holds there, re-key" becomes "Whether or not the reason holds there, re-key". This is the edit that moves an entry onto a mutant its reason does not describe.
      - "it may be this entry's mutant too: leave" becomes "it is not this entry's mutant: never leave".
      - "When no survivor above is this entry's mutant" becomes "Whatever is printed above".
      - "no longer survives because a test killed it under that key" (the line asserts a kill).
      - "not against the code line printed there alone" becomes "that is against the code line printed there alone".
      - The gate appends a note when an unreadable survivor exists: test B stays green, so it holds D1 only for an unadjudicated survivor.
      - The VTT-304 loop moved below the stale loop: green, though the line says "Look above ... a survivor the gate cannot read".
    - **Smallest fix (not run):** add to `POINTER` "Look above", "it may be this entry's mutant under another key", "If the reason describes that mutant and holds there", "it may be this entry's mutant too", "When no survivor above is this entry's mutant"; give test B a third run with an unreadable survivor; assert the VTT-304 line's index is below the stale line's.

    ## Nit

    1. **`tools/ts-mutation-equivalents.txt`, "and the gate refuses it".** False where the entry's code stands elsewhere as often as recorded: shape "TS e" printed the stale line with `faults 0`. The Go head says "the gate fails", which is true in both cases.
    2. **Timed-out mutants are not candidates in the line.** With a wrong `span` and the mutant timed out (shape "TS c3"), and with copy 2 timed out (shape "GO D"), the run prints `timed out: ...` and the stale line sends the reader to "a survivor". The ending is safe (by hand, then "leave the entry and find out").
    3. **Test A's name** says the line "names the same run's unadjudicated survivor". The line names the place; test B forbids naming the survivor. VTT-308's own sentence is exact.
    4. **Ticket, "builds it with the entry's own mutant killed"** for `test_an_entry_on_one_copy_does_not_claim_another_copys_survivor`. True of the TypeScript fixture (`Killed` at 30:6). The Go fixture has no line for `a.go:30:7` at all: absent, which is the distinction the ticket is about.
    5. **Plan, Task 7's file list** omits `tools/ts-mutation-equivalents.txt`; its "lists those files and no other" will be false for C1. The writer's G5 note lifts the constraint but not this list.
    6. **Plan, Appendix A, "A fails in each file with seven failures".** With the final tests on `5cb42fa`'s gates it is 15 in each file (run in `work/head`). B and C pass there, as claimed.
    7. **Plan, "named five places where the words, followed literally, lead wrong".** The reader's five are G3; T4 and T5; G6 and T6; G2 and T2; G1, G5 and T1. The writer's list drops G2/T2 and puts in "is re-keyed reads as something the gate does", which is from the reader's other list.
    8. **Phrases that read two ways:**
       - "this run": the measurement, or this invocation of the checker (a reused Go verdict, a stored Stryker report).
       - "this entry's mutant": the one its key names, or the one its reason was written about. They differ in G4 and T4.
       - "on another line": another than which.
       - "no longer survives": first read as "was killed" by both cold readers; kept because existing tests count by it.
       - "until a run can read that survivor": no next step, as the second reader said of G6.

    ## The twelve runs of `qa3/runs-body.md`, read literally

    | Run | What the literal reader does | Right? |
    |---|---|---|
    | G1, T1 | No survivor above; mutates by hand; removes on a kill, else leaves and looks for why | Yes |
    | G2, T2 | Compares: `nth` 2 against 1; reads the reason at copy 2; it does not describe it; the survivor gets its own remedy; by hand on copy 1 | Yes |
    | G3 | Compares: offset 6 against 15; the reason names `c > d`; not re-keyed; by hand | Yes; only "describes" stops the move |
    | G4 | Header equal, code differs; reads the reason at line 6; if it holds, takes the printed header and code line | Yes; "on another line" pulls the other way first |
    | G5, T5 | Other code, reason does not describe it; by hand | Yes |
    | G6, T6 | Leaves the entry until a run can read the survivor | Yes |
    | T3 | `span` 5 against 6, same mutant; re-keys | Yes |
    | T4 | Reason names `a > b`, code is `a > c`; not re-keyed; by hand on the entry's code, killed, removed | Yes for the fixture; see S1 if `c` is `b` renamed |

    ## What I checked, and how

    | Point | How | Result |
    |---|---|---|
    | 1. The stale line | Command: 23 shapes of my own (`work/shapes.py`), old gate against new | M1, S1, Nit 2. Every other sentence held in every shape, the unmeasured package, the absent mutant and the other spelling included |
    | 2. Literal read | Reading | Table above; Nit 8 |
    | 3. Other prose | Command and reading | S2, S3, Nit 1. The `entry_faults` docstring and the section comment are true: `main` reads the equivalents, parses the report, calls `check`, which calls `entry_faults` before the loop over `mutants(report)`; `check` is its only caller in the gate |
    | 4. Tests | Command | Each of A, B, C can fail (controls, and the plan's K0 to K23 re-run in my clone: every count identical to `breaks-3.log`). S5, Nit 3. `# VTT-308` stands directly above test A in both files. No existing test edited: each file is its `5cb42fa` self plus one appended class (119 to 122, 67 to 70), QA files untouched |
    | 5. Rule 2 | Command, three instruments of my own | Token streams without comments and with strings blanked: identical. Full bytecode with string constants blanked: identical, both gates. Each instrument has a control that prints DIFFERENT. Every line of all 23 shapes but the stale tail is byte-identical between old and new gate |
    | 6. Ticket and plan | Command | S4, Nits 4 to 7. Held: the quoted text equals the gate's tail, the 15 phrases equal `POINTER` in both files, the ticket's rule equals the VTT-308 row, 122/85/70/79 in the repository with nothing skipped, Appendix B prints `identical` twice, Done 4 regex 2 then 0, Done 6 grep 1 then 0, A red and B, C green on `5cb42fa`'s gates |
    | 7. VTT-308 | Command | `check-requirements-chain.py .` passes (308 rows). One breakable rule; the named tests red on its removal, but not on "Look below" (S5) |
    | 8. Design | Reading and one measurement | Below |

    Not reached: nothing of the eight. Not done: `task check` and both real gates, as instructed.

    One observation outside this change: with `# VTT-308` moved above test B, the chain still passes. Placement is held by reading only.

    ## Design

    I see no sound reason against the fixed line inside this ticket. Two things D1 does not weigh:

    - **The gate holds the fate of the mutant under the entry's own key** (S2). Printing it is not pairing with a survivor and is not design (b). It changes code, so it is a ticket of its own; the comment should not foreclose it with "cannot".
    - **Advice once, a short line per entry.** Both gates already do this for timeouts, with the reason in a comment ("ONCE, not once per mutant — 322 copies of it in the 2026-08-28 run"). Measured with three stale entries: each line is 1512 characters, 122 of them (8%) name the entry, and the three lines are 4536 of 5211 characters of stderr.

    On length: the order is right. "that alone does not say the mutant was killed" is at word 24 of 262, "Look above" at 61, "remove the entry" at 236. The sentence that stands between the reader and "Look above" is the one in M1.

    ## Repository state

    `git status --short` at the end is the same eight lines as at the start (same hash of the listing, `61590db9`), `HEAD` is `5cb42fa`, and no `.pyc` was written. `git stash list` shows one entry; I ran no stash command.

**The reviewer's shapes** (`shapes.py <clone>`), on which its M1 rests. Its
three other instruments, for its nine breaks, their controls and the
comparison by token stream and bytecode, are not given here and do not
outlive the session; its report above says what each printed.

    """Reviewer's own shapes. Loads gates + QA harnesses from the clone given as argv[1]."""
    import importlib.util, io, os, sys, tempfile, json
    sys.dont_write_bytecode = True
    CLONE = sys.argv[1]
    TOOLS = os.path.join(CLONE, "tools")
    def load(name, fname):
        spec = importlib.util.spec_from_file_location(name, os.path.join(TOOLS, fname))
        mod = importlib.util.module_from_spec(spec); sys.modules[name] = mod
        spec.loader.exec_module(mod); return mod
    gq = load("goqa", "check_mutation_qa_test.py")
    tq = load("tsqa", "check_ts_mutation_qa_test.py")
    CB, CN, CODE, OTHER, FIRST, SECOND = gq.CB, gq.CN, gq.CODE, gq.OTHER, gq.FIRST, gq.SECOND
    TAILS = set()
    def show(title, text, root=None):
        print("===", title)
        for line in text.splitlines():
            if not line.strip(): continue
            if root: line = line.replace(root, "<root>")
            if "no longer survives" in line:
                i = line.index("no longer survives")
                TAILS.add(line[i:])
                line = line[:i] + "<STALE-TAIL>"
            print("   |", line)
        print()
    def go(title, files, eq, plan, during=None, prep=None):
        with tempfile.TemporaryDirectory() as root:
            for rel, data in files.items(): gq.write(root, rel, data)
            if prep: plan = prep(root)
            r = gq.gate(root, eq, plan, during=during)
            show("GO " + title + " [exit %d]" % r.code, r.out + r.err, root)
        return r
    e1 = gq.entry("p", "f.go", CB, FIRST, 1, CODE)
    # A. unmeasured package, alone
    go("A unmeasured package q; p measured clean", {"p/f.go": gq.f_go("p"), "q/f.go": gq.f_go("q")},
       gq.entry("q", "f.go", CB, FIRST, 1, CODE), {"./p/": {}})
    # A2. unmeasured package q with a survivor in p's same-named file f.go
    go("A2 unmeasured package q; survivor in p f.go same code", {"p/f.go": gq.f_go("p"), "q/f.go": gq.f_go("q")},
       gq.entry("q", "f.go", CB, FIRST, 1, CODE), {"./p/": {"lived": [(CB, gq.at("f.go", CODE, FIRST))]}})
    # B. spelling p/sub x.go vs p sub/x.go
    guard = "if a > b {"
    sub = "package sub\n\nfunc x(a, b int) bool {\n\t\t" + guard + "\n\t\treturn true\n\t}\n\treturn false\n}\n"
    go("B entry `p/sub  x.go`, survivor `p  sub/x.go`", {"p/f.go": gq.f_go(), "p/sub/x.go": sub},
       gq.entry("p/sub", "x.go", CB, 6, 1, guard), {"./p/": {"lived": [(CB, gq.at("sub/x.go", guard, 6))]}})
    # C. absent entirely
    go("C mutant absent from gremlins output", {"p/f.go": gq.f_go()}, e1, {"./p/": {}})
    # D. timed out under another key: entry on copy1 (killed), copy2 timed out
    go("D copy1 entry, copy 2 TIMED OUT (no survivor)", {"p/f.go": gq.f_go(inner=CODE, outer=CODE)},
       gq.entry("p", "f.go", CB, FIRST, 2, CODE, nth=1),
       {"./p/": {"killed": [(CB, gq.at("f.go", CODE, FIRST, 1))], "timed_out": [(CB, gq.at("f.go", CODE, FIRST, 2))]}})
    # D2. entry's own mutant timed out but unreadable location
    go("D2 a timed-out mutant whose location cannot be read", {"p/f.go": gq.f_go()}, e1,
       {"./p/": {"timed_out": [(CB, "f.go:5:99")]}})
    # D3. own mutant timed out under own key: claimed (control)
    go("D3 own mutant timed out under own key (control: no stale)", {"p/f.go": gq.f_go()}, e1,
       {"./p/": {"timed_out": [(CB, gq.at("f.go", CODE, FIRST))]}})
    # E. edited line, entry's code elsewhere
    def moved(root):
        old = gq._locate(root, "./p/", gq.at("f.go", CODE, FIRST))
        gq.write(root, "p/f.go", gq.f_go(inner=OTHER, outer=CODE))
        return {"./p/": {"lived": [(CB, old)]}}
    go("E line edited; entry code stands elsewhere", {"p/f.go": gq.f_go(inner=CODE)}, e1, None, prep=moved)
    # F. other mutator same offset
    go("F CN survivor at entry's CB offset; CB killed", {"p/f.go": gq.f_go()}, e1,
       {"./p/": {"killed": [(CB, gq.at("f.go", CODE, FIRST))], "lived": [(CN, gq.at("f.go", CODE, FIRST))]}})
    # G. tree edited between the static check and package q's source read (during p's run)
    def during(pkg):
        if pkg == "./p/":
            gq.write(ROOT[0], "q/f.go", gq.f_go("q", inner=OTHER))
    ROOT = [None]
    with tempfile.TemporaryDirectory() as root:
        ROOT[0] = root
        gq.write(root, "p/f.go", gq.f_go("p")); gq.write(root, "q/f.go", gq.f_go("q"))
        r = gq.gate(root, gq.entry("q", "f.go", CB, FIRST, 1, CODE),
                    {"./p/": {}, "./q/": {"lived": [(CB, gq.at("f.go", OTHER, FIRST))]}}, during=during)
        show("GO G tree edited after static check, before q is read [exit %d]" % r.code, r.out + r.err, root)
    # H. unreadable survivor + stale
    go("H unreadable survivor", {"p/f.go": gq.f_go()}, e1, {"./p/": {"lived": [(CB, "f.go:5:99")]}})
    # I. two packages, survivor in other package's same-named file
    # ---------------- TypeScript
    A, B, BASE, CMP, CMP_CODE, DUP = tq.A, tq.B, tq.BASE, tq.CMP, tq.CMP_CODE, tq.DUP
    EQ = "EqualityOperator"; CE = "ConditionalExpression"
    def ts(title, s):
        r = s.run(); show("TS " + title + " [exit %d, faults %d]" % (r.rc, len(r.faults)), r.text); return r
    # report older than tree: tree line 2 is the entry's code; report's source line 2 is older code
    s = tq.Scn().file(A, BASE[:1] + ["  if (a > c) {"] + BASE[2:], tree_lines=BASE)
    s.mut(A, EQ, "Survived", "a >= c", 2, 7); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE)
    ts("a report older than tree: SAME line 2, report has `a > c`, tree has entry's `a > b`", s)
    # report older than tree, survivor on entry's code in the report but tree has moved it (line shift only) -> claimed (control)
    s = tq.Scn().file(A, BASE, tree_lines=["// new"] + BASE)
    s.mut(A, EQ, "Survived", "a >= b", 2, 7); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE)
    ts("a2 report older than tree, only a line added above (control)", s)
    # b. names b.ts, survivor in a.ts
    s = tq.Scn().file(A, BASE).file(B, BASE)
    s.mut(A, CE, "Survived", "true", 2, 7); s.mut(B, CE, "Killed", "true", 2, 7); s.ent(B, CE, 5, 1, "true", CMP_CODE)
    ts("b entry names b.ts, survivor in a.ts", s)
    # c. not viable / NoCoverage / Timeout under another key
    s = tq.Scn().file(A, BASE); s.mut(A, EQ, "CompileError", "a >= b", 2, 7); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE)
    ts("c CompileError", s)
    s = tq.Scn().file(A, BASE); s.mut(A, EQ, "NoCoverage", "a >= b", 2, 7); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE)
    ts("c2 NoCoverage", s)
    s = tq.Scn().file(A, BASE); s.mut(A, EQ, "Timeout", "a >= b", 2, 7); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE, span=6)
    ts("c3 wrong span and the mutant TIMED OUT (not survived)", s)
    s = tq.Scn().file(A, BASE); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE)
    ts("c4 mutant absent from report", s)
    # d. replacement differs
    s = tq.Scn().file(A, BASE); s.mut(A, CE, "Survived", "false", 2, 7); s.mut(A, CE, "Killed", "true", 2, 7); s.ent(A, CE, 5, 1, "true", CMP_CODE)
    ts("d entry -> true killed, -> false survives same place", s)
    # e. T4 shape
    edited = "  if (a > c) {"; lines = BASE[:1] + [edited] + BASE[2:] + [CMP]
    s = tq.Scn().file(A, lines); s.mut(A, CE, "Survived", "true", 2, 7); s.mut(A, CE, "Killed", "true", len(lines), tq.col(CMP, "a > b")); s.ent(A, CE, 5, 1, "true", CMP_CODE)
    ts("e line edited, entry code stands elsewhere", s)
    # f. unreadable
    s = tq.Scn().file(A, BASE); s.mut(A, EQ, "Survived", "a >= b", 2, 7, end=tq.NO_END); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE)
    ts("f unreadable survivor", s)
    # g. report has no source for the file at all
    s = tq.Scn().file(A, BASE, with_source=False); s.mut(A, EQ, "Survived", "a >= b", 2, 7); s.ent(A, EQ, 5, 1, "a >= b", CMP_CODE)
    ts("g report carries no source for the file", s)
    print("distinct stale tails:", len(TAILS))
    for t in TAILS: print(t)
