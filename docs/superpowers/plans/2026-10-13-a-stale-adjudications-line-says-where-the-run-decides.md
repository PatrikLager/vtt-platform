# Plan: a stale adjudication's line says where the same run decides its remedy

Ticket: `docs/superpowers/specs/2026-10-13-a-stale-adjudications-line-says-where-the-run-decides-design.md`.
Verified 2026-10-13 by an agent that did not write the ticket, on the tree at
`5cb42fa`. Verdict: **Passes with gaps**. The verifier wrote this file and
changed nothing in the repository; every edit it tried was made in a clone.
This file is written for
`docs/superpowers/plans/2026-10-13-a-stale-adjudications-line-says-where-the-run-decides.md`.

Every number below is a measurement at `5cb42fa`. None is a target.

**Amended by the ticket's writer, 2026-10-13, after this verification.** The
text below is the verifier's and describes the ticket as it stood when it was
verified. The writer then changed the ticket on the verifier's own findings,
and no agent verified it a second time:

- F1 and F2: the ticket's account of how an entry reaches the stale line now
  names the survivor that is not the entry's mutant, and rules the unmeasured
  package, the mutant no longer made and the unreadable survivor out of scope
  by name.
- F3: the stale line's first clause is in the problem statement.
- F4 and Q3: the wrapped copy of the docstring's claim is in the problem
  statement, and Done 4 is a search that crosses line breaks, not `grep -c`.
  After the reading review it covers the gate's test file too, where the
  claim stood a third time.
- F5 and F6: Done 3 names `SURVIVED` in any form and `ADJUDICATION MOVED`, and
  says a new test holds that a refused entry's line carries no `no longer
  survives`.
- G7: the ticket's list names the two QA test files.
- After the cold reads (see "After the cold reads and the reading review"
  below): the ticket's
  problem statement names the survivor the gate cannot read, its Done 1 says
  the entry is re-keyed if its reason holds for that mutant and that an
  unreadable survivor may be its mutant too, and that case leaves the list of
  what the ticket rules out.
- G5: the owner ruled the header sentence of
  `tools/ts-mutation-equivalents.txt` in on 2026-10-13, after the first cold
  reader named it as in conflict with the gate's line. It is the ticket's
  Done 6, and the constraint below that the header is not touched no longer
  holds.

One change to the tasks: Phase 3 of the cycle runs the gate before the commit,
so `task check` whole also runs on the working tree before C1. D9's run on the
tree after C3 stands as the second.

The questions for Patrik come before the decisions, because their answers
change what the tasks carry.

## How the ticket was verified

| Check | How settled | Result |
|---|---|---|
| 1. Every path resolves | command | Every file the ticket names exists. Each of the five tests is declared once in the file the ticket gives for it. `for e in stale` stands once in `run` (`tools/check-mutation.py`) and once in `check` (`tools/check-ts-mutation.py`). The quoted filter, the quoted `json.loads` call, the quoted docstring opening and the quoted SPEC-020 clause were each found by `grep`. |
| 2. Done is an observation | command; item 5 not run | Item 1's test was prototyped and fails on today's gates in each file. Item 4's `grep -c` prints 1 today. Items 2 and 3 hold today and must go on holding. Item 5 was not run (G1). |
| 3. Each rule is breakable | reading, then a command | The one rule is false on today's tree. Each clause of it has a break that reds the prototyped test (the break table). |
| 4. Scope matches the claim | command, then reading | `git grep -l "no longer survives"` names the two gates, their four test files and the 2026-10-10 plan, and nothing else. With every string constant blanked, the prototyped gates parse to the same tree as today's. |
| 5. No decision contradicted silently | reading | SPEC-020 was read whole and is the only specification on the subject. The ticket replaces the stale half of D8 of the 2026-10-10 plan and keeps its D7. ADR-010's decision 3, that a stale adjudication fails, stands. |
| 6. Records moved are named and reachable | command, then reading | The section is present and its path resolves. No sentence of SPEC-020 becomes false (see "What SPEC-020 gains"). |

## What was run

All of it in two clones of `5cb42fa` under the verifier's scratch directory,
with the ticket copied in by hand. In the repository itself the four
self-tests were run, and searches and probes that write nothing there.

**The four self-tests today:** 119, 85, 67 and 79 tests, all OK.

**What each gate prints, shape by shape.** Each row was built with the QA
harnesses' own fixtures and the output read.

| How the entry stands | What the run prints |
|---|---|
| Its own code line was edited away | The refusal line, and the survivor unadjudicated. No stale line. Both gates. |
| Its mutant is killed, nothing else survives | The stale line alone. Both gates. |
| It names copy 1; a survivor sits on copy 2 | The survivor unadjudicated with `nth=2`, then the stale line. Both gates. |
| The line was edited; the entry's code stands elsewhere | The survivor unadjudicated on the new code, then the stale line. Both gates. |
| TypeScript: its `span` is not its mutant's | The survivor unadjudicated with the same code and offset, then the stale line. |
| A survivor on an unrelated statement of its file | That survivor unadjudicated, then the stale line. Both gates. |
| Go: its package is not one the run measures | The stale line alone. |
| Go: written `p/sub  x.go`; gremlins names the file `p  sub/x.go` | The survivor unadjudicated under the other spelling, then the stale line. |
| TypeScript: it names `b.ts`; the survivor is in `a.ts` | The survivor unadjudicated in the other file, then the stale line. |
| Its mutant survives and the gate cannot read the survivor's key | The VTT-304 line, then the stale line. No unadjudicated line. Both gates. |
| Its mutant is absent or not viable | The stale line alone. Both gates. |
| TypeScript: the report is older than the tree | The survivor unadjudicated with the report's code, then the stale line. |

**The order in the TypeScript gate, traced.** `main` calls `read_equivalents`,
reads `mutation.json`, parses it, and calls `check`. `check` calls
`entry_faults`, then `mutants`. The report is read before `entry_faults` runs.

**The stores.** In the repository, `package_fingerprint` matches the skip
cache for 14 of 14 gated packages, and `reports/mutation/ts-inputs.sha256`
equals `python3 tools/ts-mutation-inputs.py`. The skip cache was written
2026-10-12 23:02 and the Stryker report 2026-10-08 15:25.

## What the tree showed that the ticket does not say

**F1. Two of the ticket's three shapes are built by tests whose own words say
the survivor is not the entry's mutant.**
`test_an_entry_on_one_copy_does_not_claim_another_copys_survivor` says, in both
files: "The judged copy's mutant is killed and another copy's survives: that
survivor is a mutant nobody judged, though its code is the same." The
TypeScript fixture reports the entry's own mutant `Killed`. The helper under
`test_conditional_expression_entry_does_not_claim_edited_code_at_its_old_position`
says the adjudicated code "still stands once, further down, with its mutant
killed". Only `test_a_wrong_span_is_not_refused_statically_and_claims_nothing`
builds a survivor on the entry's own code at its own offset.

So the ticket's "in each the same run prints the mutant as a survivor nobody
adjudicated" is stronger than the tests show. What they show is a survivor
printed unadjudicated in the entry's file above the stale line. A moved key and
a killed mutant beside an unjudged neighbour print the same two lines. This is
the fact D1 and D2 are built on.

**F2. An entry reaches the stale line in more ways than two.**

- The Go gate adds to `stale` twice: `stale += gone`, from `claims`, and
  `stale += [e for e in entries if e.package not in seen]`, for an entry whose
  package the run did not measure. `tools/mutation-scope.md` records the second
  as intended. The TypeScript gate adds once, from `claims`.
- A survivor the gate cannot read (VTT-304) is in neither list `claims` is
  given. Its entry goes stale and the survivor is printed on the VTT-304 line.
- A mutant that is absent without a kill reaches the line: not covered, not
  viable, or no longer generated.

**F3. The stale line's first clause is false in two shapes.** "no surviving or
timed-out mutant sits on that code": in the wrong-span shape a survivor sits on
that code at that offset, and in the copy shape one sits on that code. The
comment above each loop says the same. The ticket quotes only the line's
ending.

**F4. The docstring's claim stands twice.** `tools/check-ts-mutation.py` says
it again in the section comment above `MUTATOR_TOKENS`: "checked against the
tree before the report / is read", wrapped over two lines. Done 4's `grep -c`
is bound to one line and prints 0 with that sentence still there.

**F5. Three things the existing tests do not hold.** Measured by putting a
phrase into a line, or taking one out, and running the gate's two suites:

- No Go test requires the stale line to name its entry, or to carry `remove the
  entry`. Both edits leave 204 tests green.
- No test in either gate holds that a refused entry's line lacks `no longer
  survives`. Done 3 states it; it is true today by reading the printed line.
- In Go nothing holds any word of the remedy: with the line cut off after
  `no longer survives`, all 204 tests pass. In TypeScript the same cut reds
  one test, the control inside
  `test_a_refused_entry_is_not_also_told_to_remove_the_entry`.

**F6. The phrases the existing tests hold on the stale line.** This is the
whole list, found by listing every literal the four files assert absent, count
by, filter by or scan with a regex, and then appending each to the stale line.

| Phrase | Go | TypeScript |
|---|---|---|
| `no longer survives`, present | 16 tests | 18 tests |
| `no longer survives`, once only | 2 tests | 13 tests |
| `not adjudicated`, absent | 4 tests | free |
| `SURVIVED ` with a space, absent | free | 15 tests |
| `SURVIVED` in any form, absent | free | `test_two_entries_for_one_code_and_one_survivor_leave_one_stale` |
| `ADJUDICATION MOVED`, absent | `test_a_survivor_on_a_different_statement_is_not_paired` | `test_a_survivor_of_the_entrys_mutator_on_another_statement_is_not_claimed` |
| `remove the entry`, present | no test | `test_a_refused_entry_is_not_also_told_to_remove_the_entry`, its control |
| the entry's description, present | no test | `test_an_entry_whose_mutant_was_killed_fails_the_gate` |

Free in both, each tried: `unadjudicated`, `zero unadjudicated survivors`,
`do not delete`, `cannot be read`, `timed out:`, `starved`, `2 entries`,
`EXCUSES NOTHING`, `code: x`, and lower-case `survived`. On the refusal line,
`remove the entry` is held absent by three Go tests and two TypeScript tests,
one of them without regard to case.

The ticket's Done 3 names `no longer survives`, `not adjudicated` and
`SURVIVED ` with a space. It misses the bare `SURVIVED` and `ADJUDICATION
MOVED`. Its "four suites pass" half covers both.

## Rule 9: how MapTool does this

MapTool does not solve this. No tabletop function is designed here, and a
search of `~/dev/RPTool` at `63862e4` for pitest, stryker, gremlins and
mutation testing in its build and configuration files finds nothing. That is
the ticket's own statement, checked.

## The constraints

These bind every task. They are named here, not restated.

- **CLAUDE.md rule 2.** No gate is weakened. This change alters the text of
  two printed lines, three comments and one docstring; the comparison in
  Appendix B shows that and is run again on the final tree. `task check` is run
  whole at the end.
- **CLAUDE.md rule 8.** The plan, the report and the specification cite tests,
  functions and commits by name.
- **CLAUDE.md rule 10** does not reach `tools/`. No file under `internal/`,
  `cmd/` or `client/src` changes.
- **SPEC-020.** A refused entry is not also reported stale. An unadjudicated
  survivor is printed as an entry ready to paste. Neither moves.
- **The 2026-10-10 plan, D7.** Pairing was deleted on purpose. Nothing here
  brings back `stale_entry_hint` or `ADJUDICATION MOVED`.
- **SPEC-008.** An id comes from `requirement-id`, after sign-off, tried on a
  copy of the register first. The citation is the bare id on the line above
  the test. This plan holds no id the register lacks; `VTT-NNN` below was a
  placeholder that the chain does not read as a citation.
- **The ticket's own bounds.** No test that exists today is edited. Nothing a
  gate accepts, refuses or claims changes. The header of
  `tools/ts-mutation-equivalents.txt` is not touched.
- **The identifier gates.** `check:no-retraction`, `check:no-create-scene` and
  `check:no-pack` read `.py` files at code positions, so a new test's name is
  in their scope.
- **Operational.**
  - Every break is made in a scratch clone, never in the working tree, and
    undone there with `git show HEAD:<file> > <file>`.
  - Nothing is stashed or checked out while a reviewer reads; the review
    package is `git diff HEAD`.
  - `review-record.sh` refuses while a file is untracked. The ticket and this
    plan are staged before the review is recorded.
  - `git add` and `git commit` run in separate calls, with `git show --stat
    HEAD` after each commit.
  - The tree is not edited while a gate runs. `task check` is launched in its
    own process session, after reading the load with `os.getloadavg()`.
  - 35.3 GiB were free on the data volume when this was written, against the Go
    gate's 16 GiB floor. Its warning already prints.

## Questions for Patrik

- **Q1. One fixed line, or a line the gate chooses from its survivors?**
  *Recommendation: one fixed line (D1).* The other is `stale_entry_hint`'s
  same-file branch, which D7 of the 2026-10-10 plan deleted. If it is chosen
  all the same, the ticket's exclusion of pairing is its writer's to lift
  first, and D1, D2, test B and break K9 are written again.
- **Q2. Does the ticket's rule get a requirement id?** *Recommendation: yes,
  one (D7).* The 2026-10-10 sort refused an id to message design, so this is a
  ruling and not a default. If no: Task 1 falls away, the tests carry no id
  line, and SPEC-020's `Requirements` list is unchanged.
- **Q3. Is the wrapped copy of the docstring's claim corrected here (F4)?**
  *Recommendation: yes (D4).* It is the same sentence in a file the ticket
  already touches, and Done 4 is green with it still false.
- **Q4. Does Phase 4a run, and as what?** *Recommendation: yes, as a cold read
  (D10).* If it writes tests, they go in `tools/check_mutation_qa_test.py` and
  `tools/check_ts_mutation_qa_test.py`, which the ticket's list omits (G7).

The tasks below are written for the recommended answers.

## After the cold reads and the reading review: what Tasks 5 and 7 changed

Written by the ticket's writer, 2026-10-13. Task 5 ran twice, each time one
agent given only twelve printed runs and the head of each adjudication file,
and Task 7's reviewer then read the whole change. Their findings became
changes to D2's text, as Task 5 allows, and Tasks 2 to 4 were run again on
each text. What follows replaces D2's sentence, Appendix A's tests, the break
table and the draft paragraph for SPEC-020 further down; the verifier's
sections are left as they were written.

**The text as it now stands.** After "no longer survives", the same in both
gates, eleven sentences:

> under that key (the fields and code just named): this run reports no
> surviving or timed-out mutant under it, and that alone does not say the
> mutant was killed. This run found this entry's code in that file on as
> many lines as the entry records, so a survivor printed on other code is on
> another line than that code, unless the file changed between that count
> and the measuring of its mutants. Look above, in this same run's output,
> for a survivor in that file. If one is printed unadjudicated, with an
> entry ready to paste under it, compare its header and code line with this
> entry's: it may be this entry's mutant under another key. Read this
> entry's reason against that mutant where it stands in the file, not
> against the code line printed there alone. If the reason describes that
> mutant and holds there, re-key this entry instead of deleting it: give it
> the header and code line printed there. If it describes that mutant and no
> longer holds, this entry is wrong: remove it. If it does not describe that
> mutant, the survivor is another mutant. In both of those cases that
> survivor needs a test or an entry of its own. If one is printed as a
> survivor the gate cannot read (VTT-304), it may be this entry's mutant
> too: leave this entry as it is until a run can read that survivor. When no
> survivor above is this entry's mutant, make the mutation by hand and run
> the tests, since this run's output does not show a kill: if a test now
> kills the mutant, remove the entry so it cannot pre-approve a future
> survivor; if none does, leave the entry and find out why this run did not
> report its mutant.

**What each read found.**

- The first cold reader read D2's text and did the right thing in every run.
  It named five places where the words, followed literally, lead wrong: a
  reason judged "against the code printed there" passes when the code is
  identical and the offset is not (G3); "re-keyed ... and not deleted" keeps
  an entry whose reason no longer fits (T4, T5); beside a survivor the gate
  cannot read the line opens the way to removing a sound entry (G6, T6); the
  printed line is the entry's own, so the words re-key by `nth` without the
  look that decides it (G2, T2); and "Only if ... a test now kills the mutant"
  forbids removing an entry whose code is gone (G1, G5, T1).
- The second cold reader read the text that answered those and found no run
  where the whole line, read strictly, gives a wrong edit. It named five
  passages it read more than once and three that come close to a wrong act.
- The reviewer read the third text and found one clause false, "so a survivor
  printed on other code is on another line", where the source the run
  measured is not the tree the static check read; that "that survivor is not
  this entry's" says more than the reader's judgement has shown; that the
  loop comment's "cannot tell" is a "does not", since both tools report what
  became of each mutant; a third copy of the docstring's claim, in class
  `EntryCheck`; and that test A held 43% of the line, nine edits to the rest
  leaving every suite green.

The text above answers all of these but two. "no longer survives" stays, since
existing tests count by it. And the line does not name a timed-out mutant as a
place the entry's mutant may be; its last sentence still ends at "leave the
entry and find out why". The findings, one line each with what became of
them, go in the implementation report.

**What the line still does not do**, and the owner ruled a ticket of its own,
later (2026-10-13). It is one fixed line (D1), so it does not say which case
is on the screen, and it is printed whole for every stale entry. Both cold
readers asked for what the run recorded for the mutant under the entry's key,
which gremlins and Stryker both report with its place and neither gate looks
up for this line; and the reviewer measured the third text's line at 1,512
characters, of which 122 name the entry, and pointed at the timeout advice,
which each gate prints once.

**The tests, in place of Appendix A.** Same three names in both files. The
classes hold `SENTENCES`, the eleven sentences above, where Appendix A held a
`POINTER` tuple of seven phrases.

- A, `test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file`,
  asserts each sentence present on the stale line and absent from every other
  line, and then that the line ends with "no longer survives" and the eleven
  joined by a space. On the gates of `5cb42fa` it fails with twelve failures
  in each file.
- B, `test_the_stale_line_is_the_same_with_and_without_a_survivor_beside_it`,
  has a third run, beside a survivor the gate cannot read, and asserts that
  survivor's line is printed above the stale one.
- C, `test_a_refused_entry_and_a_claimed_one_get_no_stale_line`, is as it
  was, with the sentences where the phrases were.

Class `EntryCheck` in `tools/check_ts_mutation_test.py` has its docstring's
first sentence corrected; no test in it changed.

**The breaks, made again on this text** in a clone holding the working tree's
change set. K0 to K15 are the verifier's, with K2 cutting from "Look above" to
"When no survivor above" and K15 dropping "instead of deleting it"; their
counts of existing tests came out as in the verifier's table, and K9 now reds
A as well as B. Nineteen more:

| # | The edit to the working gate | Go: new, existing | TypeScript: new, existing |
|---|---|---|---|
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
| K26 | "If the reason describes that mutant and holds there, re-key" becomes "Whether or not the reason holds there, re-key" | A, 0 | A, 0 |
| K27 | "it may be this entry's mutant too: leave" becomes "it is not this entry's mutant: never leave" | A, 0 | A, 0 |
| K28 | "When no survivor above is this entry's mutant" becomes "Whatever is printed above" | A, 0 | A, 0 |
| K29 | "because a test killed it" put after "no longer survives" | A, 0 | A, 0 |
| K30 | "not against the code line printed there alone" becomes "that is against ..." | A, 0 | A, 0 |
| K31 | a note appended when the gate holds a survivor it cannot read | B, 0 | B, 0 |
| K32 | the survivors the gate cannot read printed below the stale lines | B, 0 | B, 0 |
| K33 | ", unless the file changed between that count and the measuring of its mutants" removed | A, 0 | A, 0 |
| K34 | "If it describes that mutant and no longer holds, this entry is wrong: remove it." removed | A, 0 | A, 0 |

K24 to K32 are the nine edits the reviewer found unheld. All four self-tests
pass on this text, 122, 85, 70 and 79 tests, and the comparison of Appendix B
prints `identical` for both gates.

**The comment above each stale loop**, in place of D3's: its reason reads "the
gate does not look up what became of the mutant under the key, and a survivor
beside the entry may be nobody's, so the line names where the reader looks
(VTT-308)". It cites the row and not SPEC-020, which says nothing of the line
until C3.

**The paragraph for SPEC-020**, in place of the draft under "What SPEC-020
gains", for the `specification` skill to check against the final tree:

> **A stale entry's line names where the same run settles its remedy**
> (VTT-308). The line is the same for every stale entry and in both gates, and
> picks nothing from what the gate holds. It says that the run reports no
> surviving or timed-out mutant under the entry's key and that this alone
> does not say the mutant was killed, and it sends its reader to the survivors
> the same run prints above it for the entry's file. One printed
> unadjudicated may be the entry's mutant under another key: the entry is
> re-keyed to the printed header and code line if its reason describes that
> mutant and holds where the mutant stands, and removed if the reason
> describes it and no longer holds. One the gate cannot read (VTT-304) leaves
> the entry as it is. With no survivor that is its mutant, the entry is
> removed only once a test is seen to kill the mutation made by hand.

**C1's files**, in place of Task 7's list: the two gates, the two unit test
files, `tools/ts-mutation-equivalents.txt`, `docs/requirements.md`, the
ticket and this plan. Task 5 wrote no test, so the two QA files are not among
them.

**The id, allocated 2026-10-13 after sign-off, is VTT-308.** The verifier wrote
`VTT-NNN` wherever it belongs; the writer put the id in the specification's
draft paragraph and above the two tests of Appendix A, and left the
placeholder in the sentences that speak of it.

**Answered 2026-10-13.** The owner signed the plan off with D2's text as it
stands. Q1: the fixed line; it is what he approved when he chose the work, and
the ticket rules pairing out. Q2: yes, one id. Q3: yes; the ticket now names
both places. Q4: yes, as a cold read.

## Decisions

**D1. Each gate prints one fixed stale line. It picks nothing from the
survivors it holds.**

Two designs were weighed.

- **(a) One fixed wording.** It says what the run knows, where in the same
  output to look, and what each finding means. It says nothing false in any
  row of the table above, because it asserts neither a kill nor a move. The
  cost is a longer line in every case, and the reader does the looking. Where
  the survivor is printed somewhere other than the entry's file, the line does
  not lead to it (G4).
- **(b) The gate chooses.** With an unadjudicated survivor in the entry's file
  it prints the pointer; without one it prints a plain "remove". The line is
  shorter when the entry stands alone. The cost is that "remove" then rests on
  an absence the gate cannot vouch for: the survivor may be printed under
  another package and file spelling, in another file, or on the VTT-304 line,
  and a row of the table shows each. And with a survivor in the file it points
  at a mutant that F1 shows may be nobody's.

*Forced by:*
- F1: the run cannot tell a moved key from a killed mutant beside an unjudged
  neighbour, so a gate that chooses asserts what it does not know.
- D7 of the 2026-10-10 plan and the ticket's own exclusion. Design (b) is the
  deleted hint by another name.
- Rule 2 and the ticket's bound: (a) adds no branch to a gate. With strings
  blanked the two files parse as they do today. (b) adds one on
  `unadjudicated`.
- The owner approved "the stale message is reworded so that it says how to
  tell the cases apart".

`test_the_stale_line_is_the_same_with_and_without_a_survivor_beside_it` holds
this decision. It passes today and goes red under (b) (break K9).

**D2. The text.** What follows the entry's description is the same sentence in
both gates:

> under that key: this run reports no surviving or timed-out mutant under it,
> and that alone does not say the mutant was killed. Look above, in this same
> run's output, for a survivor printed unadjudicated in that file, and compare
> the entry printed under it with this one: it may be this entry's mutant under
> another key, and then this entry is re-keyed, once its reason is judged
> against the code printed there, and not deleted. Only if no such survivor is
> its mutant, and a test now kills the mutant, remove the entry so it cannot
> pre-approve a future one.

In `run`, `tools/check-mutation.py`:

```python
        print(f"check:mutation: {equivalents_path} lists {describe(e)}, which no longer survives "
              f"under that key: this run reports no surviving or timed-out mutant under it, and "
              f"that alone does not say the mutant was killed. Look above, in this same run's "
              f"output, for a survivor printed unadjudicated in that file, and compare the entry "
              f"printed under it with this one: it may be this entry's mutant under another key, "
              f"and then this entry is re-keyed, once its reason is judged against the code "
              f"printed there, and not deleted. Only if no such survivor is its mutant, and a "
              f"test now kills the mutant, remove the entry so it cannot pre-approve a future one.",
              file=err)
```

In `check`, `tools/check-ts-mutation.py`:

```python
        print(f"check:ts-mutation: {describe(e)} is adjudicated equivalent but no longer survives "
              f"under that key: this run reports no surviving or timed-out mutant under it, and "
              f"that alone does not say the mutant was killed. Look above, in this same run's "
              f"output, for a survivor printed unadjudicated in that file, and compare the entry "
              f"printed under it with this one: it may be this entry's mutant under another key, "
              f"and then this entry is re-keyed, once its reason is judged against the code "
              f"printed there, and not deleted. Only if no such survivor is its mutant, and a "
              f"test now kills the mutant, remove the entry so it cannot pre-approve a future one.",
              file=err)
```

Each clause and what forced it:

| Clause | Forced by |
|---|---|
| `no longer survives under that key` | Done 3 keeps the phrase, once. "Under that key" makes it true in the moved-key rows, where the mutant does survive. |
| `this run reports no surviving or timed-out mutant under it` | It replaces "sits on that code", which F3 shows false. It is true in every row, the unmeasured package included. |
| `that alone does not say the mutant was killed` | F1 and F2. The line may not assert a kill the run did not see. |
| `Look above, in this same run's output` | The ticket's rule. Both gates print unadjudicated survivors before stale entries; the new test pins that order (break K11). |
| `a survivor printed unadjudicated in that file` | Done 1. It cannot quote the survivor line's own words: `not adjudicated` and `SURVIVED` are counted by (F6). |
| `compare the entry printed under it with this one` | SPEC-020 prints a ready entry under each unadjudicated survivor. Its header and `code:` line are what tell another copy, offset, mutator, replacement or span from other code. |
| `it may be ... under another key` | Done 1. "May", because of F1. |
| `re-keyed, once its reason is judged against the code printed there, and not deleted` | Done 1, and SPEC-020: a re-keyed entry's reason is judged again. |
| `Only if no such survivor is its mutant, and a test now kills the mutant, remove the entry so it cannot pre-approve a future one` | Done 2. From "a test now kills the mutant" to the end it is today's text, kept. |

The pointer comes before the removal on purpose: a reader acts on the first
remedy a line gives (break K4).

*The check that was run on this text.* The patched gates were made to print
every kind of line in one run each, and a script asserted on what they
printed: the stale line is one line; it ends with the sentence above, word for
word; it carries `no longer survives` once and `remove the entry`; it carries
none of the other phrases any of the four files asserts absent, counts by or
filters by, in any letter case; it matches none of the three regexes the QA
files scan with; the refusal, unadjudicated and VTT-304 lines are
byte-identical to today's; and both gates print one and the same sentence
after the entry. 87 assertions, all held. Then the four suites: 122, 85, 70 and 79 tests, all
OK, the last against the real Stryker report with nothing skipped.

**D3. The comment above each stale loop becomes a warning and a pointer.** The
same four lines in both gates:

```python
    # A stale entry is one no surviving or timed-out mutant has the key of
    # (VTT-301). DO NOT have this line say why, or pick a remedy from the
    # survivors printed above it: the run cannot tell a killed mutant from a
    # moved key, so the line names where the reader looks (SPEC-020).
```

*Forced by:* today's comment reads "A stale entry names code no surviving or
timed-out mutant sits on: the mutant was killed, or the code was edited." Its
first half is F3's false clause and its second is the partition F2 shows
incomplete. The next editor of this loop is the one who will want design (b).

**D4. The docstring states the order, and so does the comment that repeats
it.** `entry_faults` in `tools/check-ts-mutation.py` opens:

```python
    """[(Entry, fault)] for every entry the tree refutes. It reads the tree, not the report.

    check() calls it with Stryker's report already loaded and before it goes
    through the report's mutants (SPEC-020).
```

and the section comment above `MUTATOR_TOKENS` opens:

```python
# WHAT AN ENTRY'S OFFSET MUST HOLD, checked against the tree before check()
# goes through the report's mutants. An entry on a comment, or on code its
# mutator cannot rewrite, names no mutant Stryker makes, so it could only ever
# go stale.
```

*Forced by:* the trace above, SPEC-020's own sentence, and F4. The check for
this decision is the search in Appendix B, which crosses line breaks. It finds
2 today and must find 0. Done 4's `grep -c` is run as well.

The Go docstring, "before any mutation run", is true and does not change.

**D5. Three new tests in each file, in a new class at the end.** `StaleLineTest`
in `tools/check_mutation_test.py`, `StaleLine` in
`tools/check_ts_mutation_test.py`, each placed before `if __name__`. The names
are the same in both files.

- **A.** `test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file`.
  Done 1, Done 2 and the stale line's half of Done 3. Red today.
- **B.** `test_the_stale_line_is_the_same_with_and_without_a_survivor_beside_it`.
  D1. Green today.
- **C.** `test_a_refused_entry_and_a_claimed_one_get_no_stale_line`. The other
  direction, and the half of Done 3 that F5 found unheld. Green today.

*Forced by:* Done 1 names one new test per file; that is A. B exists because
nothing else would notice design (b) arriving. C exists because the direction
"a correct input leaves it silent" needs a test of its own.

No existing test is edited. `git diff --numstat` shows no deleted line in
either test file, and the two QA files are untouched unless Q4 adds to them.

**D6. How a test isolates the stale line.** The stale line is the one line of
the output that carries `no longer survives`. The test splits the output into
that line and every other line joined. Each phrase is asserted present in the
first and absent from the second. So a phrase that another line happened to
carry would fail the test, and a phrase asserted on the whole output would
not. Test A also requires exactly one stale line.

*Forced by:* Done 1's "on the stale line alone". The stale line is a single
line with no line break in it; the unadjudicated message is four.

**D7. One requirement, stated as the ticket states it.**

> The line a mutation gate prints for an adjudication that claims nothing, and
> that the tree does not refute, names where in the same run's output its
> reader finds whether its mutant survives under another key.

The observation that goes red is test A, and the row's evidence cell is:

    tools/check_mutation_test.py#test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file, tools/check_ts_mutation_test.py#test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file

Both are red on today's tree.

It passes the sort: one thing, what and not how, breakable, and no word in it
that cannot be observed. *Against it:* the 2026-10-10 sort refused "the
ready-to-paste entry in a message" as message design, with "nothing breaks at
the table if it changes". *For it:* that format fails loudly, since a pasted
entry does not parse. This rule fails silently, and the gates' own history
records what that costs. The docstring of `stale_entry_hint`, as it stood at
`ca14522`, says of the advice to remove a stale entry: "obeying it throws away
reasoning somebody did and leaves the survivor unexplained. That happened four
times in one day." The verifier allocated nothing.

**D8. SPEC-020 gains one paragraph and one id.** See "What SPEC-020 gains".

**D9. One branch, three commits, one whole gate run.** C1 is the gates, the
tests, the row, the ticket and this plan. C2 is the report. C3 is SPEC-020.
`task check` runs once, on the tree after C3.

*Forced by:* the last arc's shape (`b0e4172`, `1c31423`, `5cb42fa`), and the
measurement below: this change gives no gate anything new to measure, so one
run on the final tree is the whole of it.

**D10. Phase 4a is a cold read.** One agent is given the printed output of six
runs per gate, and for each the entry as it stands in the equivalents file.
It is given nothing else: not the gates, not the tests, not this plan. It says
what it would do with each entry and why. The six runs are: killed alone; the
other copy; the wrong span, or in Go the other mutant on the line; the edited
line with the code elsewhere; an unrelated survivor; an unreadable survivor.

*Forced by:* the rule is about what a reader is told, and a phrase test shows
only that words are present (G2). The pass is: it re-keys and does not delete
where the survivor is the entry's mutant; it removes where nothing is beside
the entry; it does not move the entry onto the unrelated survivor. The
implementer builds the six runs and hands over only what they printed. A test
the agent derives as well comes from the rule and SPEC-020 alone.

**D11. The breaks are made again on the committed text.** The table below is
of the prototype. Task 4 repeats every row.

## The breaks

Each was made on the prototyped gate in a clone, the gate's two suites run,
and the gate put back. A, B and C are the new tests. "Existing" counts tests
that exist today.

| # | The edit to the working gate | Go: new, existing | TypeScript: new, existing |
|---|---|---|---|
| K0 | the stale `print` rebuilt unchanged (the harness's own control) | none, 0 | none, 0 |
| K1 | today's stale text put back | A, 0 | A, 0 |
| K2 | the sentence from "Look above" to "not deleted." removed | A, 0 | A, 0 |
| K3 | "Only if ... and a test now kills the mutant, remove" becomes "Otherwise remove" | A, 0 | A, 0 |
| K4 | the removal sentence moved before the pointer | A, 0 | A, 0 |
| K5 | "that alone does not say the mutant was killed" becomes "so the mutant was killed" | A, 0 | A, 0 |
| K6 | "If the code was edited, re-key it." appended | A, 0 | A, 0 |
| K7 | a phrase the survivor line is counted by, added | A, 4 | A, 15 |
| K8 | the stale line names no entry | A, 0 | A, 1 |
| K9 | a note appended when the gate holds an unadjudicated survivor | B, 0 | B, 0 |
| K10 | `stale = [e for e in stale if id(e) not in faulted]` removed | C, 3 | C, 2 |
| K11 | the stale loop moved above the unadjudicated loop | A, 0 | A, 0 |
| K12 | a second `no longer survives` on the line | A, 2 | A, 13 |
| K13 | every entry called stale | C, 40 | C, 42 |
| K14 | "in that file" removed | A, 0 | A, 0 |
| K15 | ", and not deleted" removed | A, 0 | A, 0 |

Ten of the fifteen breaks are caught by a new test and by nothing that exists
today, in both gates: K1 to K6, K9, K11, K14 and K15.

The other direction, on the repaired gate: test C, in which a refused entry
and an entry that claims its survivor each print no stale line. And every test
that exists today still passes.

## Tasks, in dependency order

### Task 0: baselines

No file changes. Run the four self-tests; record the counts. Recompute the 14
fingerprints against `reports/mutation-skip-cache.json` and the Stryker stamp
against `python3 tools/ts-mutation-inputs.py`.

*True when:* all four pass and their counts are written down; every gated
package's stored verdict is reusable and the stamp matches. If a store is not
current, say so to Patrik before anything else: the final gate run would then
measure, for a reason outside this ticket.

### Task 1: the sort, after sign-off (Q2)

`dev-cycle:requirements` over D7. `requirement-id`, tried on a copy of the
register first, then on `docs/requirements.md`.

*True when:* the row exists with `**OPEN — no test yet**`, and its id is
written into this plan in place of `VTT-NNN`.

### Task 2: the tests, first

Add Appendix A's two classes. Replace `# VTT-NNN` with the id.

*Files:* `tools/check_mutation_test.py`, `tools/check_ts_mutation_test.py`.

*True when:* on the unchanged gates, A fails in each file and B and C pass;
every test Task 0 counted still passes; `git diff --numstat` shows no deleted
line in either file.

### Task 3: the two gates

D2's two `print` statements, D3's comment twice, D4's docstring and comment.

*Files:* `tools/check-mutation.py`, `tools/check-ts-mutation.py`.

*True when:*
- the four self-tests pass, the two changed files each with three tests more
  than Task 0 counted;
- Appendix B's comparison prints `identical` for both gates;
- Appendix B's search finds 0, and `grep -c 'before the report is read'
  tools/check-ts-mutation.py` prints 0;
- the two gates print the same sentence after the entry's description, shown
  by a command and not by eye.

### Task 4: the breaks

K1 to K15 on the committed text, in a scratch clone, one at a time, with K0
first. Assert each edit landed before trusting what the suites say.

*True when:* each break reds the new test its row names, and the result is
kept for the report and for C1's message.

### Task 5: Phase 4a (Q4)

D10. Its findings are adjudicated one line each in the report, under their own
heading.

*True when:* the reader's six answers per gate are recorded, and each is
either as D10's pass describes or has become a change to the text, with Tasks
2 to 4 run again on the changed text.

### Task 6: the steps of `task check` that read these files

`python3 tools/check-requirements-chain.py .`, `python3
tools/check-no-retraction.py .`, `python3 tools/check-no-create-scene.py .`,
`python3 tools/check-no-pack.py .`, and `python3 tools/check-ts-mutation.py
tools/ts-mutation-equivalents.txt` against the stored report.

*True when:* each prints its own clean line. The Go gate is not run on its
own: it clears the Go test cache before it starts, and `task check` runs it.

### Task 7: Phase 4b, then C1

One reviewer, on `git diff HEAD` with the ticket and this plan staged. Its
brief is the prose: is every sentence of D2's text true in every row of the
table of shapes, and does the docstring state the order SPEC-020 states. Write
D7's evidence cell into the row. Record the review. Commit.

*Files in C1:* the two gates, the two test files, the QA files if Task 5 wrote
to them, `docs/requirements.md`, the ticket, this plan.

*True when:* `git show --stat HEAD` lists those files and no other, and the
message says which break reddened which test.

### Task 8: the report, then C2

`dev-cycle:implementation-report`, under `docs/reports/`, answering Done 1 to
5 in the ticket's numbering. It carries the table of shapes, F1 to F6, the
break results, Task 5's adjudications, and the header sentence the ticket left
(G5). Its own reading review, its own commit.

### Task 9: SPEC-020, then C3

`dev-cycle:specification`. The paragraph and the id below. Its own reading
review, its own commit.

*True when:* `python3 tools/check-requirements-chain.py .` passes with the id
in SPEC-020's `Requirements` list.

### Task 10: the gate, the push, the merge

`task check` whole, on the tree after C3. Then the push and the merge, on
Patrik's word.

*True when:* Done 5 is green and, unless Task 0 found a store out of date, the
Go gate measured no package and the TypeScript task verified the stored report
without mutating.

## What `task check` re-measures because of this change

Nothing.

- **Go.** `package_fingerprint` hashes the Go version, the gremlins argument
  list, `go.mod`, `go.sum`, and every file under each directory of the
  package's `go list -deps -test` closure. No such directory is the module
  root, `tools/` or anything under `docs/`. Measured in a clone: with a line
  appended to both gates, all four test files, both equivalents files,
  `docs/requirements.md` and SPEC-020, and with a new plan and a new report
  added, 14 of 14 fingerprints are unmoved.
- **TypeScript.** `tools/ts-mutation-inputs.py` hashes `client/src`,
  `client/test`, `stryker.conf.json`, `package.json`, the lockfile and
  `client/tsconfig.json`. Its docstring says "NOT the checker and NOT the
  adjudication file". Measured with the same edits: the hash is unmoved.
- **So** the run reuses 14 Go verdicts and the stored Stryker report, and
  computes both verdicts again with the new checker. Prototyped: the patched Go
  gate over the repository's skip cache reuses 14 and prints "14 packages, zero
  unadjudicated survivors"; the patched TypeScript gate over the stored report
  prints "2944 mutants, 2868 killed, 9 timed out, 67 survivors all adjudicated
  equivalent, zero unadjudicated survivors". Neither prints a stale line, so
  the real run does not exercise the new text. The self-tests do.

**The steps that read the two gates or the four test files:**

- `check:mutation` runs `tools/check_mutation_test.py` and
  `tools/check_mutation_qa_test.py`, then the gate.
- `check:ts-mutation` runs `tools/check_ts_mutation_test.py`,
  `tools/check_ts_mutation_qa_test.py` and `tools/ts_mutation_inputs_test.py`,
  then the gate.
- `check:requirements-chain` reads the test files for ids, with
  `docs/requirements.md` and every file under `docs/specifications/`.
- `check:no-retraction`, `check:no-create-scene` and `check:no-pack` read all
  six `.py` files at code positions. Over the prototype each printed `clean`.
- `check:new-prose` reads `tools/*.py` only as an index of names, and only
  when a change adds a line to a `.go` file. This one adds none.

No pre-commit or pre-push hook runs any of the four self-tests. They run when
somebody types `task check`.

**The steps that read the ticket, this plan or the report:** none. No step of
`task check` reads Markdown under `docs/superpowers/` or `docs/reports/`. The
pre-commit hook's `secrets` and `review-gate` see them as staged content.

## What SPEC-020 gains

No sentence of SPEC-020 becomes false. It says nothing today of what the stale
line tells its reader. These are the sentences the work touches or rests on:

- "VTT-298, VTT-299, VTT-300, VTT-301, VTT-302, VTT-303, VTT-304." The list
  gains the id, if Q2 is answered yes.
- "**An unadjudicated survivor is printed as an entry ready to paste:** the
  header in column 1, the `code:` line and a placeholder reason indented four
  spaces." Unchanged. The new paragraph goes after it, since the stale line
  now sends its reader to that printed entry.
- "in TypeScript once Stryker has written its report and before the gate goes
  through its mutants". Unchanged. The docstring is brought to it.
- "A refused entry fails the gate, matching still runs, and the entry is not
  also reported stale." Unchanged. Test C holds it for an entry whose own code
  was edited away.
- "An edit to the mutant's own code changes its key. The static check refuses
  the entry, whose code no longer stands as often as it records". Unchanged.
  The ticket's problem statement rests on it.

The paragraph, as a draft for the `specification` skill to check against the
final tree:

> **A stale entry's line names where the same run settles its remedy**
> (VTT-308). It says that the run reports no surviving or timed-out mutant
> under the entry's key and that this alone does not say the mutant was
> killed; that a survivor the same run prints unadjudicated in the entry's
> file may be its mutant under another key, in which case the entry is
> re-keyed once its reason is judged against the code printed there, and not
> deleted; and that the entry is removed only if no such survivor is its
> mutant and a test now kills the mutant. The line is the same whether or not
> a survivor is printed beside it. Neither gate pairs a stale entry with a
> survivor.

## Gaps that travel with this plan

- **G1. `task check` whole was not run.** Done 5 is open until Task 10. What
  is known: both stores are current, and the prototyped gates exit 0 over
  them.
- **G2. Test A pins phrases the plan chose.** It shows the words are there.
  That they tell a reader what Done 1 says is held by Task 5's cold read and
  Task 7's review.
- **G3. Done 1 does not say what the line reads when no survivor is beside
  it.** It admits both designs. D1 takes the fixed one; Q1 puts it to the
  owner.
- **G4. Three ways to the stale line are named and not mended.** An unreadable
  survivor is printed on the VTT-304 line, which the stale line does not name;
  so is a survivor printed for another file than the entry names. An entry for
  a package the run does not measure, and a mutant absent without a kill, are
  given no remedy: neither condition the line offers is theirs. The ticket
  rules one neighbour of these out of scope, a mutant one run kills and the
  next does not.
- **G5. The header of `tools/ts-mutation-equivalents.txt` keeps its
  sentence:** "A stale entry — one no surviving or timed-out mutant sits on,
  because the mutant was killed or the code was edited — is a hard error".
  Leaving it is safe: no gate and no test reads a comment line of that file,
  and its remedy does not contradict the new line. It is then the one place
  that still gives "the code was edited" as a cause, and its first clause is
  F3's.
- **G6. The id is open until Task 1.** Every `VTT-NNN` in this plan and in
  Appendix A is a placeholder.
- **G7. The ticket's list omits the two QA test files.** Only Q4 can bring
  them in.
- **G8. No red run of either real gate was read.** The stale line is known
  from fixtures.
- **G9. Noticed, outside this ticket.** SPEC-020 does not say that a Go entry
  whose package the run does not measure is stale; `tools/mutation-scope.md`
  does.

## Appendix A: the tests as prototyped

For `tools/check_mutation_test.py`, before `if __name__`:

```python
class StaleLineTest(unittest.TestCase):
    """What the stale line tells its reader (SPEC-020). The run cannot tell a
    killed mutant from one whose key moved, so the line names where to look
    and picks no remedy itself."""

    POINTER = ("that alone does not say the mutant was killed",
               "in this same run's output",
               "a survivor printed unadjudicated in that file",
               "under another key",
               "re-keyed",
               "not deleted",
               "a test now kills the mutant, remove the entry")

    def gate(self, eq_path, mapping, lines=GO_LINES):
        out, err = io.StringIO(), io.StringIO()
        with tempfile.TemporaryDirectory() as root:
            source_tree(root, lines)
            code = cm.run(eq_path, ["./p/"], runner_for(mapping), out=out, err=err, root=root,
                          free_bytes=500 * 1024**3, cache_bytes=0)
        return code, out.getvalue(), err.getvalue()

    def split(self, err):
        """(the stale lines, every other line of the output joined)."""
        lines = err.splitlines()
        return ([l for l in lines if "no longer survives" in l],
                "\n".join(l for l in lines if "no longer survives" not in l))

    # VTT-308
    def test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file(self):
        eq = equivalents(entry("p", "a.go:30:7", "CONDITIONALS_BOUNDARY"))
        code, _, err = self.gate(eq, {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:40:7"))})
        self.assertEqual(code, 1)
        [stale], rest = self.split(err)
        self.assertIn("a.go:40:7 CONDITIONALS_BOUNDARY SURVIVED and is not adjudicated", rest)
        self.assertLess(err.index("a.go:40:7"), err.index(stale), "the survivor is printed above")
        self.assertIn("p a.go CONDITIONALS_BOUNDARY offset=7 nth=1 on `if dup<lim {`", stale)
        for phrase in self.POINTER:
            with self.subTest(phrase):
                self.assertIn(phrase, stale)
                self.assertNotIn(phrase, rest)
        self.assertNotIn("If the code was edited", stale)
        self.assertLess(stale.index("in this same run's output"), stale.index("remove the entry"),
                        "where to look comes before the removal")
        self.assertEqual(stale.count("no longer survives"), 1)
        self.assertNotIn("not adjudicated", stale)

    def test_the_stale_line_is_the_same_with_and_without_a_survivor_beside_it(self):
        """The gate picks no message from its own survivors: beside a survivor
        or alone, the entry gets one line."""
        eq = equivalents(entry("p", "a.go:30:7", "CONDITIONALS_BOUNDARY"))
        _, _, beside = self.gate(eq, {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:40:7"))})
        _, _, alone = self.gate(eq, {"./p/": gremlins_output()})
        [with_survivor], _ = self.split(beside)
        [without], rest = self.split(alone)
        self.assertEqual(with_survivor, without)
        self.assertEqual(rest, "")

    def test_a_refused_entry_and_a_claimed_one_get_no_stale_line(self):
        """The other direction. An entry whose own code was edited away is
        refused and told to correct itself; an entry that claims its survivor
        is told nothing."""
        eq = equivalents(entry("p", "a.go:10:5", "CONDITIONALS_BOUNDARY"))
        survivor = {"./p/": gremlins_output(("CONDITIONALS_BOUNDARY", "a.go:10:5"))}
        code, out, err = self.gate(eq, survivor, lines={**GO_LINES, 10: "if a<c {"})
        self.assertEqual(code, 1)
        self.assertIn("stands 0 time(s)", err)
        code_ok, out_ok, err_ok = self.gate(eq, survivor)
        self.assertEqual(code_ok, 0, err_ok)
        for text in (out + err, out_ok + err_ok):
            for phrase in ("no longer survives", "remove the entry") + self.POINTER:
                with self.subTest(phrase):
                    self.assertNotIn(phrase, text)
```

For `tools/check_ts_mutation_test.py`, before `if __name__`:

```python
class StaleLine(unittest.TestCase):
    """What the stale line tells its reader (SPEC-020). The run cannot tell a
    killed mutant from one whose key moved, so the line names where to look
    and picks no remedy itself."""

    POINTER = ("that alone does not say the mutant was killed",
               "in this same run's output",
               "a survivor printed unadjudicated in that file",
               "under another key",
               "re-keyed",
               "not deleted",
               "a test now kills the mutant, remove the entry")

    def split(self, msg):
        """(the stale lines, every other line of the output joined)."""
        lines = msg.splitlines()
        return ([l for l in lines if "no longer survives" in l],
                "\n".join(l for l in lines if "no longer survives" not in l))

    # VTT-308
    def test_a_stale_line_names_the_same_runs_unadjudicated_survivor_in_its_file(self):
        code, msg = run(
            report(("client/src/a.ts", 30, 6, "StringLiteral", "Killed", '""'),
                   ("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""')),
            entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "reasoned")))
        self.assertEqual(code, 1)
        [stale], rest = self.split(msg)
        self.assertIn("SURVIVED StringLiteral at client/src/a.ts:40:6", rest)
        self.assertLess(msg.index("client/src/a.ts:40:6"), msg.index(stale),
                        "the survivor is printed above")
        self.assertIn('client/src/a.ts StringLiteral (-> "") offset=6 span=1 nth=1 on `send("tag");`', stale)
        for phrase in self.POINTER:
            with self.subTest(phrase):
                self.assertIn(phrase, stale)
                self.assertNotIn(phrase, rest)
        self.assertNotIn("If the code was edited", stale)
        self.assertLess(stale.index("in this same run's output"), stale.index("remove the entry"),
                        "where to look comes before the removal")
        self.assertEqual(stale.count("no longer survives"), 1)
        self.assertNotIn("SURVIVED", stale)

    def test_the_stale_line_is_the_same_with_and_without_a_survivor_beside_it(self):
        """The gate picks no message from its own survivors: beside a survivor
        or alone, the entry gets one line."""
        eq = entries(entry("client/src/a.ts", "30:6", "StringLiteral", '""', "reasoned"))
        _, beside = run(report(("client/src/a.ts", 30, 6, "StringLiteral", "Killed", '""'),
                               ("client/src/a.ts", 40, 6, "StringLiteral", "Survived", '""')), eq)
        _, alone = run(report(("client/src/a.ts", 30, 6, "StringLiteral", "Killed", '""')), eq)
        [with_survivor], _ = self.split(beside)
        [without], rest = self.split(alone)
        self.assertEqual(with_survivor, without)
        self.assertEqual(rest, "")

    def test_a_refused_entry_and_a_claimed_one_get_no_stale_line(self):
        """The other direction. An entry whose own code was edited away is
        refused and told to correct itself; an entry that claims its survivor
        is told nothing."""
        eq = entries(entry("client/src/a.ts", "10:5", "EqualityOperator", "a >= b"))
        edited = {**TS_LINES, 10: "if (a > q2) c();"}
        code, msg = run(report(("client/src/a.ts", 10, 5, "EqualityOperator", "Survived", "a >= q2"),
                               lines=edited), eq, lines=edited)
        self.assertEqual(code, 1)
        self.assertIn("stands 0 time(s)", msg)
        code_ok, msg_ok = run(report(("client/src/a.ts", 10, 5, "EqualityOperator", "Survived", "a >= b")), eq)
        self.assertEqual(code_ok, 0, msg_ok)
        for text in (msg, msg_ok):
            for phrase in ("no longer survives", "remove the entry") + self.POINTER:
                with self.subTest(phrase):
                    self.assertNotIn(phrase, text)
```

On today's gates, A fails in each file with seven failures: the first six
phrases of `POINTER` are absent, and `If the code was edited` is present. The
seventh phrase is in today's line already. B and C pass.

## Appendix B: two instruments, to be rebuilt as written

The verifier's scratch directory does not outlive its session.

**Only strings changed.** Run from the repository root with the change in the
working tree. Comments are not in a syntax tree, so `identical` means the same
statements, conditions, calls and interpolated expressions.

```python
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
```

Its control was run: with `not in faulted` changed to `in faulted` it prints
`DIFFERENT`.

**The docstring's claim, across line breaks.** Prints 2 at `5cb42fa` and must
print 0.

```python
import re
src = open("tools/check-ts-mutation.py").read()
print(len(re.findall(r"before the report\s+(?:#\s*)?is read", src)))
```
