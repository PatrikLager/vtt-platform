# A mutation adjudication is keyed by the code it excuses: the change

**Ticket:** `docs/superpowers/specs/2026-10-10-a-mutation-adjudication-is-keyed-by-its-code-design.md`,
revised by its writer after verification and after sign-off (the plan's
section "Patrik's answers at sign-off, 2026-10-10").
**Plan:** `docs/superpowers/plans/2026-10-10-a-mutation-adjudication-is-keyed-by-its-code.md`,
written by `verify-ticket` (passes with gaps), with the owner's answers at
sign-off, the requirement sort, and the rulings after QA and review appended.
**The owner's rulings:** at sign-off, one ticket with the Go and TypeScript
gates as separate commits (Q1), a specification of its own (Q2), the dated
re-point accounts cut on this branch in a commit of their own (Q3), and
`copies` kept (Q4); after Phase 4a and the first review, `nth` in both keys and
`span` in the TypeScript key, the Go package read recursively, cross-references
by old key rewritten in the cut, and every other finding fixed; after the
second review, VTT-298's uncommitted row amended and every finding fixed; after
the cut's review, every finding fixed, the coordinates still true today
included; after this report's review, every finding fixed, the stray
citation and the pointer by a code commit on this branch.
**Last code commit:** `794d836`, which removes a stray citation that this
report's review found and a pointer to nothing that the verifier found
(Deviations). Before it, `7bb67a5` changes only comments and reasons in the
two adjudication files, `553053f` is the TypeScript gate and `ba6fe35` the
Go gate, on `ca14522`, `main` at the time. Every code reference below is to
`794d836`'s tree, except Done 5's command, which runs on `7bb67a5`'s
(Appendix A).

## The period, in commits

    git log --oneline ca14522..794d836

    794d836 The second-copy test cites one requirement, and a pointer to nothing goes
    7bb67a5 The adjudication files lose their re-point accounts
    553053f The TypeScript mutation gate keys an adjudication by its code
    ba6fe35 The Go mutation gate keys an adjudication by its code

This report, and the commit that writes SPEC-020 and edits `CLAUDE.md`
rule 8, follow.

## Done looks like, answered

1. `[x]` With a comment line inserted at the top of `internal/engine/apply.go`
   and of `client/src/fold.ts` and neither adjudication file edited, both
   self-tests pass. Run on a copy of `794d836`'s tree, the four files print

       tools/check_mutation_test.py        Ran 119 tests ... OK
       tools/check_ts_mutation_test.py     Ran 67 tests ... OK
       tools/check_mutation_qa_test.py     Ran 85 tests ... OK
       tools/check_ts_mutation_qa_test.py  Ran 79 tests ... OK (skipped=8)

   the eight skipped being QA's RealData tests, which skip when Stryker's
   report is absent or older than the client, as on a copy. On a copy of `ca14522`'s tree
   the same insertion fails both self-tests at
   `test_every_real_adjudication_still_points_at_its_mutant` and nowhere else
   (the plan's "How the ticket was verified", row 2).
2. `[x]` A survivor whose line moved and whose code did not is claimed, and
   the gate exits 0 without `ADJUDICATION MOVED`:
   `tools/check_mutation_test.py#test_a_survivor_whose_line_moved_is_still_claimed`
   and `tools/check_ts_mutation_test.py#test_a_survivor_whose_line_moved_is_still_claimed`.
   `ca14522`'s gates failed the same case. Its Go
   `test_a_move_the_source_can_prove_is_reported_as_a_move` asserted exit 1
   with `ADJUDICATION MOVED` for an entry and a survivor on identical code ten
   lines apart; its TypeScript
   `test_the_anchor_separates_two_statements_sharing_a_replacement` asserted
   `ADJUDICATION MOVED`, `90:7` and `91:7` for an entry on
   `label("other");` whose survivor stands on the identical line below, and
   exit 1, which an unpairable `send("tag")` pair in the same fixture also
   causes.
3. `[x]` A survivor at an entry's position on other code is not claimed and
   the entry is reported:
   `tools/check_mutation_test.py#test_a_survivor_on_other_code_at_an_entrys_old_position_is_not_claimed`
   and `tools/check_ts_mutation_test.py#test_a_conditional_expression_entry_does_not_claim_other_code`.
   `ca14522`'s gates, given the same case in their own format, claimed it:
   a Go entry written for `if a<b {` at `a.go:10:5` with `if a<c {` standing
   there, and a TypeScript `ConditionalExpression false` entry with its clause
   edited, both exited 0 with "zero unadjudicated survivors" (run on
   2026-10-11 against `git show ca14522:tools/check-mutation.py` and
   `check-ts-mutation.py`).
4. `[~]` What was refused is still refused, held in both self-test files
   except the replacement, which only the TypeScript key has. The ticket asks
   for tests that pass before and after; that holds for the four tests that
   existed at `ca14522`, and a fifth renamed, whose assertions this report's
   review diffed and found unchanged. The others are new and have no before:
   what shows they bite is their breaks (the plan's G4).
   - Another statement of the file: Go
     `tools/check_mutation_test.py#test_adjudication_is_matched_on_all_three_fields`
     and `#test_a_survivor_on_a_different_statement_is_not_paired`, both kept;
     TypeScript, new,
     `tools/check_ts_mutation_test.py#test_a_survivor_of_the_entrys_mutator_on_another_statement_is_not_claimed`.
   - Another mutator: Go `#test_adjudication_is_matched_on_all_three_fields`,
     its `ARITHMETIC_BASE` case; TypeScript
     `#test_an_adjudication_is_keyed_by_mutator_not_just_location`, kept.
   - Another replacement, TypeScript only:
     `#test_two_mutants_at_one_location_are_two_separate_claims`, kept, and
     `#test_a_survivor_with_another_replacement_at_the_entrys_code_is_not_claimed`,
     `ca14522`'s `test_a_different_replacement_is_not_a_move` renamed, its
     assertions unchanged and its survivor moved onto the entry's code.
   - New in both files: two mutants of one mutator on one line
     (`#test_two_mutants_of_one_mutator_on_one_line_are_two_claims`); a second
     copy of an adjudicated line
     (`#test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated`,
     `#test_an_entry_on_one_copy_does_not_claim_another_copys_survivor`); an
     entry whose code stands nowhere
     (`#test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported`).
5. `[x]` No key holds a line number, and over one run of each gate the new
   keys claim what the old keys claimed: Appendix A's command, run on
   `7bb67a5` over the stores frozen at Task 0, printed

       GO live=42 old_claimed=36 new_claimed=36 old-new=[] new-old=[]
       TS live=76 old_claimed=67 new_claimed=67 old-new=[] new-old=[]

   and both gates, run on `7bb67a5` over the same stores, report zero
   unadjudicated survivors. The command first asserts that `internal/` and
   `client/src` equal `ca14522`'s, which holds at `7bb67a5` and not at
   `794d836`, whose `internal/adventure/compile_test.go` differs.
6. `[x]` No reason and no header carries a dated account of a re-point:
   `7bb67a5` cut 304 sentences from 20 Go reasons and 505 from 44 TypeScript
   reasons, and from the TypeScript header 13 paragraphs of re-keying history
   and, beside them, the green-run record of 2026-08-25.
   Appendix B's check prints "not whole sentences: 0" over every reason, once
   the listed rewrites are undone; which sentences were such accounts was
   held by Phase 4b's third review, which found no Must.
7. `[ ]` Not done at this report's commit, by design, and the item stands:
   the process writes the specification after the report. One part is
   already true: both headers give the format and name
   `docs/specifications/020-a-mutation-adjudication-names-the-code-it-excuses.md`.
   The commit that follows writes that file and edits `CLAUDE.md` rule 8, and
   Phase 4b reads Done 7 there.
8. `[x]` `task check` whole is green on `794d836`: run detached on
   2026-10-11, it ended `TASK_CHECK_EXIT=0` with no `task: Failed` line.
   `check:mutation` re-measured with gremlins the three packages whose test
   closure holds `internal/adventure`, which `794d836` touches
   (`internal/adventure`, `internal/adventure/conformance` and
   `internal/gateway`), reused the other eleven's stored verdicts ("Nothing in
   its dependency closure differs"), and matched all fourteen anew against
   the new keys: "14 packages, zero unadjudicated survivors", 36 survivors
   claimed. `check:ts-mutation` verified the stored report's input stamp and
   recomputed the verdict: "2944 mutants, 2868 killed, 9 timed out, 67
   survivors all adjudicated equivalent, zero unadjudicated survivors".
   Neither gate reported a stale entry. The same gate was green on `7bb67a5`
   earlier the same day, reusing every stored verdict.

## What the rules became

| Id | Rule | Held by |
|---|---|---|
| VTT-298 | A survivor is claimed by the entry naming its file, mutator, TS replacement, code, copy and TS extent, wherever the code stands and however indented. | 4 Go and 6 TS tests |
| VTT-299 | A survivor is not claimed by an entry whose code it does not sit on. | 6 Go and 8 TS tests |
| VTT-300 | One entry claims at most one survivor. | `#test_one_entry_claims_one_of_two_survivors_on_its_key`, both files |
| VTT-301 | An entry that claims no surviving or timed-out mutant fails the gate. | 3 Go and 3 TS tests |
| VTT-302 | An entry whose code stands other than `copies` times is refused before any run. | 3 Go and 3 TS tests, the real-file check among them |
| VTT-303 | A survivor is matched against the source its run measured. | 1 Go and 1 TS test |
| VTT-304 | A survivor whose code cannot be read fails the gate. | 2 Go and 4 TS tests |

VTT-298's statement was amended in its uncommitted row after the second
review, on the owner's ruling: the first wording named neither the copy nor
the extent, which D15 and D16 put in the key. The ticket's rules, five after
sign-off, map onto VTT-298 to VTT-302; VTT-303 and VTT-304 came from the plan's
D2. The rows name every self-test that cites them. The QA files cite the
same ids above their tests, 38 in Go and 36 in TypeScript, and no row names
a QA test. The chain check passes over 304 rows.

## The sort

Seven accepted, each with the observation that goes red, and eleven
rejected, one line each, in the plan's section "The sort, after sign-off".
After D15, `#test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated`
no longer held VTT-300 (two copies have two keys) and was moved to VTT-299;
VTT-300 is held by a direct test of `claims`. The Go copy kept a stray
`# VTT-300` until `794d836`.

## Phase 4a: QA adjudications

Two independent QA agents, one per gate, each given the requirement rows,
the signed-off decisions restated and the gate's exported surface, never its
source, its tests, the plan's test lists or a diff. Each ran twice: once on the
gate as first written, once after the owner's rulings amended the decisions.
Their tests are `tools/check_mutation_qa_test.py` (85) and
`tools/check_ts_mutation_qa_test.py` (79), run by `check:mutation` and
`check:ts-mutation` beside the gates' own.

- QA finding 1 (Go, round 1): spec ambiguity — a survivor whose column falls
  in its line's indentation or past the trimmed code was printed as an entry
  its own parser or check refused — ruled unreadable, failing the gate (D18) —
  `resolve` returns no key; `#test_a_survivor_outside_its_trimmed_line_fails_the_gate`.
- QA finding 2 (Go, round 1): spec ambiguity — a survivor on an empty line —
  unreadable, accepted — no change.
- QA finding 3 (Go, round 1): spec ambiguity — `copies=0` refused with the
  generic message — accepted; the message now says each number is at least 1.
- QA finding 4 (Go, round 1): spec gap — which stream a failure goes to — none
  needed — no change.
- QA finding 5 (Go, round 1): spec ambiguity — an entry for a package outside
  the run is stale — accepted, unchanged from `ca14522` — no change.
- QA finding 6 (Go, round 1): none — `trimmed`'s doc concerns one line
  without its terminator — no change.
- QA finding 7 (Go, round 1): spec gap — a token inside a longer operator
  (`+=`, `<<`) is not refused — the limitation `MUTATOR_TOKENS`' comment
  records — no change.
- QA finding 8 (TS, round 1, G1): behaviour defect — two mutants sharing
  start, mutator and replacement but not extent shared one key; 31 groups in
  the real report, one pairing a survivor with a killed mutant in
  `client/src/auth.ts` — owner ruled `span` into the key (D16).
- QA finding 9 (TS, round 1, G2): spec gap — the operator rule compares the
  expression as text, so two killed real mutants (`client/src/session.ts`,
  `client/src/view/scene-plan.ts`) could not be adjudicated if they survived
  — written into SPEC-020's Consequences by the commit after this report —
  no change.
- QA finding 10 (TS, round 1, G3): behaviour defect — the printed entry did
  not parse when pasted — header in column 1 (D19), both gates.
- QA finding 11 (TS, round 1, G4): spec gap — U+2028 is not a line break for
  the gate; none in `client/src` — written into SPEC-020 by the commit after
  this report — no change.
- QA finding 12 (TS, round 1, G5): none — an unreadable timeout passes, as a
  timeout never becomes unadjudicated — no change.
- QA finding 13 (TS, round 1, G6): none — Done 5 is the implementer's to show,
  not QA's — no change.
- QA finding 14 (TS, round 1, G7): spec gap — three mutators in neither
  table fail closed and cannot be adjudicated, as at `ca14522` — written into
  SPEC-020's Consequences by the commit after this report; a ticket if a
  survivor ever meets one.
- QA finding 15 (TS, round 1, G8): none — lines are 1-based — no change.
- QA finding 16 (Go, round 2): behaviour defect — the ready entry pasted
  unedited claimed its survivor, excusing a mutant nobody judged — the
  placeholder is refused as a reason in both gates (D20).
- QA finding 17 (Go, round 2): spec gap — D18 and D19 existed only in the QA
  brief — recorded in the plan.
- QA finding 18 (TS, round 2, G9): behaviour defect — a multi-line `span`
  counted continuation indentation, so a re-indent staled three real entries
  against VTT-298 — `span` leaves each line's indentation and trailing blanks
  out (D16, amended); the three entries were respanned; QA's `span_between`
  helper and its multi-line test's expected value were updated by the
  implementer to the amended definition, and its re-indent test lost its
  multi-line exemption after the second review.
- QA finding 19 (TS, round 2, G10): none — an unreadable survivor gets no
  ready entry — no change.
- QA finding 20 (TS, round 2, G11): none — an end past its own line is
  unreadable — no change.

The implementer also changed QA's TypeScript harness so that its RealData
tests read `reports/mutation/mutation.json` and skip unless its input stamp
matches the tree, rather than the frozen store; QA's 65 injections, re-run on
the committed file after one was re-aimed at the amended span, turn 60
targets red and leave 5 controls green.

## Phase 4b: the reviews

Four reading reviews, one reviewer each; the fourth, of this report, led to
a code commit.

- **The Go commit, first version.** Two Must: one entry on code standing
  twice claimed a survivor on the copy nobody judged, and the real
  `internal/rules` `expr.go` entry on `if p.depth > maxExprDepth {` was
  exposed, its own reason saying the other copy is not equivalent; and the
  commit message's count of re-pointed keys did not reproduce. Six Should
  (three more sentences the conversion made false, QA's "each reddened",
  quotations from the QA brief, the package not read recursively, a comment
  naming a deleted function, cross-references by old key) and five nits. The
  owner ruled `nth` (D15) and the recursive read (D17), the cross-references
  into the cut, and the rest fixed.
- **The Go fixes and the whole TypeScript commit.** No path found where either
  gate passes a survivor nobody judged in realistic use. Must: both counts of
  re-pointed keys (40 in 13 commits for Go, 83 in 7 for TypeScript, from
  2026-09-23 on, pairing removed and added keys of one file and mutator per
  commit with each commit's own parser); QA's re-indent test still describing
  the old `span`; the injection count measured before the amendment. Should:
  VTT-300 named a test that no longer held it, and VTT-298's statement named
  neither copy nor extent. Nits on trimming symmetry, header wording and
  comments. All fixed; VTT-298 amended in its uncommitted row.
- **The cut.** No Must. Four Should (a dangling "this same arc", a false
  "the entry above", a dropped record of a withdrawn adjudication, an
  ambiguous "this file's own header") and five nits, all fixed, the source
  coordinates still true included, on the owner's ruling.
- **This report.** Three Must: the Go second-copy test still carried
  `# VTT-300`, which the plan had moved; the rows were said to name every
  test citing them, and no row names a QA test; Done 2 cited an old
  TypeScript test whose entry and survivor stood on different code. Eight
  Should and ten nits, among them the pointer in `compile_test.go` given no
  destination. All fixed, the first and the pointer by `794d836`; a second
  pass over the fixes found five more Should and five nits, also fixed.

## The breaks

Each break was made in a scratch copy of the gate, one at a time, and each
named test went red; the restored suite passed. Which tests each break must
turn red is in `breaks_go.py` and `breaks_ts.py`, Appendix B; this report's
review re-ran both on `7bb67a5`'s tree and each reported every break ok.

**Go, 31 (`tools/check-mutation.py`):** the survivor's line put in the key;
the code, the offset, the mutator, the package and `nth` each taken out of the
key; the offset taken from the raw column instead of the trimmed line; one
entry claiming every survivor on its key; timeouts matched before survivors;
a timeout not holding its entry; leftover entries not reported stale; the
source read after the runner returns; a reused verdict's survivors dropped;
an unreadable survivor skipped; a column outside the line read; a zero count
passed; the other direction, a sound entry refused; `copies` and the `nth`
bound ignored; a duplicate entry passed; the comment rule removed; the token
rule removed; the other direction, every token refused; an unknown mutator
passed; a missing code line accepted, `#` read as a comment mid-line, the
package spelling normalised silently; the placeholder accepted as a reason;
the code line trimmed otherwise than source; the package read without its
subdirectories; the ready entry indented; and its code line dropped.

**TypeScript, 37 (`tools/check-ts-mutation.py`):** the same checks where the
gate has them, plus the replacement and `span` in the key, a missing end read
as a one-unit span, a multi-line span counting indentation, the report's
source replaced by the tree and by the tree as a fallback, UTF-16 offsets
counted in bytes, the sibling-operand comparison reduced to the left operand,
the string-opener rule removed, and entries split at a form feed. The skip
cache has no TypeScript counterpart.

## Rule 9: how MapTool does this

MapTool does not solve this. `~/dev/RPTool` has no mutation-testing
configuration; the verifier searched its build files for pitest, stryker and
gremlins and found nothing. No tabletop function is designed here.

## Deviations

| Intended | Happened | Why |
|---|---|---|
| Task 5: TypeScript tests before the gate | The TypeScript gate was written first; the red step was shown afterwards by running the new tests against `ca14522`'s gate, 44 of 59 failing: 5 failures and 39 errors, 17 of the errors because that gate has no `entry_faults`, so that much of the red is the empty kind the plan's G4 describes | An ordering slip by the implementer, found before QA; the breaks are the evidence that each test bites |
| D4: `copies` closes "the one way" a code key could claim a survivor nobody judged | It did not: one entry on code standing twice claimed the other copy's survivor; `nth` joined the key (D15) | `copies` says how many copies of the code stand, not which one the entry judged, so once the judged copy's mutant was killed its entry claimed the other copy's survivor; `internal/rules`' entry on `if p.depth > maxExprDepth {` in `expr.go` was exposed to exactly that |
| D1: the TypeScript key is code and offset | `span` joined it (D16), then left out continuation indentation (D16 amended) | QA found mutants sharing a start but not an extent, then that a re-indent changed a multi-line span |
| D2: the Go source is the package's directory | Read recursively (D17) | gremlins names files in ungated subdirectories relative to the package |
| D8: a ready-to-paste entry | Header in column 1 (D19); the placeholder refused as a reason (D20); a column outside the line unreadable (D18) | Printed indented, the entry did not parse when pasted; pasted unedited, its placeholder reason claimed the survivor; a column outside the trimmed code printed an entry the gate's own parser or check refused (QA findings 10, 16 and 1) |
| D3: two identical entries are legal | Withdrawn: two entries on one key are a duplicate, refused | With `nth` in the key, identical entries name one mutant |
| The Go and TypeScript commits from one working tree | The first Go commit attempt stopped in lefthook, which could not stash a partly staged `Taskfile.yml` beside an intent-to-add file; the TypeScript hunk was taken out of the working tree, the review re-recorded, and put back after | lefthook stashes partly staged files before its hooks |
| Task 9 keeps coordinates still true | They were named by code too | The owner ruled on the cut's review that every finding be fixed, these included; the ruling is recorded in `7bb67a5`'s message and in no section of the plan. A coordinate true today goes stale at the next edit above it |
| Task 9: every removed sentence is listed in the report | Not listed; `git diff 553053f 7bb67a5 -- tools/mutation-equivalents.txt tools/ts-mutation-equivalents.txt` holds them, and Appendix B's check shows each reason lost only whole sentences | 809 sentences would outrun the rest of this report, and the diff holds them exactly, at a commit the report names |
| Task 3: reasons carried byte for byte apart from D13's sentences | `ba6fe35` also cut, by hand and with no script in Appendix B, two Go reason passages, "Re-measured as the last action before committing — …" and "THE FIX ROUND'S OWN INSTRUCTION SAID NO KEY WOULD MOVE, …", and the `internal/sight` header paragraph opening "RE-POINTED THREE TIMES" | Each says a key moves when the file above it is edited, which the conversion made false |
| The plan's `test_an_entry_claims_a_survivor_before_a_timed_out_copy`, in both files, as B8's target | Not written; `#test_claims_takes_survivors_before_timeouts`, a direct test of `claims` in both files, holds the property and is B8's target | The order is decided inside `claims`, so the test calls it directly |
| D7 deletes `_by_position` from both gates | Both gates define a new `_by_position`: file, line, column | `claims` takes survivors in the order given, and that order is the one it gives |
| The ticket's last code commit is `553053f`, and `internal/adventure/compile_test.go` lies outside it | `794d836` removes the Go second-copy test's stray `# VTT-300`, and the pointer in `compile_test.go` to "the campaign entries in tools/mutation-equivalents.txt", which hold no `internal/campaign` entry here or at `ca14522`; the pointer's block of 30 comment lines became six, and the file's comment ceiling fell from 28.5 to 24.5. Its message says 29 lines and credits this report's review with both findings | This report's review found the stray citation; the verifier found the pointer, and this report's review found that the report gave it no destination; the owner ruled both fixed on this branch. `check:comments` (SPEC-010) refuses a comment block over six lines to which a change adds a line, and removing the pointer meant rewording lines of the block, so it was cut to six. The change to `internal/adventure` puts every package whose test closure holds it outside the gate's stored verdicts, so `794d836`'s gate re-measured those with gremlins |

## What could not be established

- **A quiet case the second review constructed:** copies swap places, and in
  the same change the unjudged copy's mutant starts surviving while the judged
  copy's times out. Not reachable without both at once; not closed.
- Whether gremlins or Stryker ever report a column in a line's indentation;
  D18 fails such a survivor closed either way.

## What was deliberately left out, and where it went

- The TypeScript operator rule's textual comparison and the three mutators in
  neither table: unchanged from `ca14522`, written into SPEC-020's
  Consequences by the commit after this report; a ticket if a survivor ever
  meets them.
- U+2028 as a line break: written into SPEC-020 by the commit after this
  report; nothing in `client/src` carries one.
- **R2.** A reason whose premise lies outside the keyed line is not judged
  again when that other code changes. Position keys caught some of these by
  accident; code keys catch none. Written into SPEC-020's Consequences by the
  commit after this report.
- **R3.** A TypeScript mutant that spans lines is keyed by its first line and
  its `span`; an edit to a later line that keeps the span's length keeps the
  key. Written into SPEC-020's Consequences by the same commit.
- Comment lines over 80 characters in `tools/ts-mutation-equivalents.txt`
  at `794d836`: three. The one in the header is this branch's own Format
  line, 110 characters because it names SPEC-020's path; the two among the
  entries stood at `ca14522` too. Left.

## Appendix A: the frozen evidence and Done 5's command

Task 0 copied the stores the gates had last written into
`reports/mutkeys-base-ca14522/` (gitignored) before any gate code changed:

    f3698f983d5182a25d2f685d23887c7223ba4c45eceed496bd7e22690d639913  mutation-skip-cache.json
    0fc3d319ab3270e1e8e3eba961b40d0a759f70780aef36056094881104c92b6c  mutation.json
    7e2a346b235a501463dbfd045e44915aab2bf9037168776c91e06f268eb9d0f3  ts-inputs.sha256

All 14 skip-cache fingerprints matched `ca14522`'s packages, holding 36
survivors and 6 timed-out mutants, and the Stryker report's input stamp
matched its tree. `tools/` is in no gated package's closure, so editing the
gates left every fingerprint unchanged, and the gate run on `7bb67a5` reuses
both stores.

The comparison, run on `7bb67a5`'s tree as `python3 appendix_a.py <repo>
ca14522 reports/mutkeys-base-ca14522 both`:

    """Done 5: over the frozen stores, the new keys claim exactly what BASE's keys claim.
    Usage: python3 appendix_a.py <repo> <BASE> <SNAP> [go|ts|both]"""
    import importlib.util, json, os, subprocess, sys, tempfile
    repo, base, snap = sys.argv[1], sys.argv[2], sys.argv[3]
    which = sys.argv[4] if len(sys.argv) > 4 else "both"
    os.chdir(repo)
    assert subprocess.run(["git", "diff", "--quiet", base, "--", "internal", "client/src"]).returncode == 0, \
        "the source the survivors were measured on is not the branch's"

    def load(name, path, rev=None):
        if rev:
            data = subprocess.run(["git", "show", f"{rev}:{path}"], capture_output=True, check=True).stdout
            fh = tempfile.NamedTemporaryFile("wb", suffix=os.path.splitext(path)[1], delete=False)
            fh.write(data); fh.close(); path = fh.name
        spec = importlib.util.spec_from_file_location(name, path)
        m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
        return m, path

    def base_file(path):
        data = subprocess.run(["git", "show", f"{base}:{path}"], capture_output=True, check=True).stdout
        fh = tempfile.NamedTemporaryFile("wb", suffix=".txt", delete=False); fh.write(data); fh.close()
        return fh.name

    bad = 0
    if which in ("go", "both"):
        old, _ = load("old_go", "tools/check-mutation.py", base)
        new, _ = load("new_go", "tools/check-mutation.py")
        cache = json.load(open(os.path.join(snap, "mutation-skip-cache.json")))
        old_entries = old.read_equivalents(base_file("tools/mutation-equivalents.txt"))
        new_entries = new.read_equivalents("tools/mutation-equivalents.txt")
        live = [(pkg, loc, mut) for pkg, e in cache.items() for loc, mut in e["survivors"] + e["timed_out"]]
        old_claimed = {m for m in live if m in old_entries}
        new_claimed = set()
        for pkg, e in cache.items():
            source = new.package_source(".", pkg)
            res = lambda ms: [(new.resolve(source, pkg, loc, mut), (pkg, loc, mut)) for loc, mut in ms]
            lived = [x for x in res(e["survivors"]) if x[0] is not None]
            hung = [x for x in res(e["timed_out"]) if x[0] is not None]
            assert len(lived) == len(e["survivors"]), f"{pkg}: a survivor's line cannot be read"
            mine = [x for x in new_entries if x.package == pkg]
            claimed, _, _ = new.claims(mine, sorted(lived, key=lambda x: x[1]), sorted(hung, key=lambda x: x[1]))
            new_claimed |= set(claimed)
        print(f"GO live={len(live)} old_claimed={len(old_claimed)} new_claimed={len(new_claimed)} "
              f"old-new={sorted(old_claimed - new_claimed)} new-old={sorted(new_claimed - old_claimed)}")
        bad += bool(old_claimed ^ new_claimed)
    if which in ("ts", "both"):
        old, _ = load("old_ts", "tools/check-ts-mutation.py", base)
        new, _ = load("new_ts", "tools/check-ts-mutation.py")
        rep = json.load(open(os.path.join(snap, "mutation.json")))
        old_entries = old.read_equivalents(base_file("tools/ts-mutation-equivalents.txt"))
        new_entries = new.read_equivalents("tools/ts-mutation-equivalents.txt")
        surv = [(p_, w, m, r, e) for p_, w, m, r, s, e in new.mutants(rep) if s == "Survived"]
        hung = [(p_, w, m, r, e) for p_, w, m, r, s, e in new.mutants(rep) if s == "Timeout"]
        live = surv + hung
        old_claimed = {m for m in live if m[:4] in old_entries}
        sources = {path: f.get("source") for path, f in rep.get("files", {}).items()}
        lived = [(new.resolve(sources, *m), m) for m in surv]
        assert all(k is not None for k, _ in lived), "a survivor's line cannot be read"
        hung_k = [(k, m) for m in hung if (k := new.resolve(sources, *m)) is not None]
        claimed, _, _ = new.claims(new_entries, sorted(lived, key=new._by_position),
                                   sorted(hung_k, key=new._by_position))
        new_claimed = set(claimed)
        print(f"TS live={len(live)} old_claimed={len(old_claimed)} new_claimed={len(new_claimed)} "
              f"old-new={sorted(old_claimed - new_claimed)} new-old={sorted(new_claimed - old_claimed)}")
        bad += bool(old_claimed ^ new_claimed)
    sys.exit(1 if bad else 0)

## Appendix B: the scripts

The six editing scripts below, `convert_go.py` to `task9_apply.py`, each
assert before they write, and each was run once, in the order given, on the
branch with `internal/` and `client/src` equal to `ca14522`'s. Done 6's check
and the two break scripts follow them; those write nothing in the tree. The cut lists Task 9 read (`cuts.json`, `rewrites.json`,
`headers.json`, and the second round's corrections) lived in the session's
scratch directory and are not kept: `7bb67a5`'s diff is the durable record of
what was cut.

**`convert_go.py`**, the Go conversion to code, offset and copies:

    """D11: convert tools/mutation-equivalents.txt from file:line:col keys to code keys.

    Reads the entries with BASE's own parser, reads each entry's line in the tree
    (whose internal/ must equal BASE's), and rewrites only the entry header lines:
    each becomes the new header plus a `code:` line. Reason lines are carried byte
    for byte except for the D13 sentences, which are cut whole. Asserts before it
    writes. Usage: python3 convert_go.py <repo> <BASE>
    """
    import importlib.util, os, subprocess, sys, tempfile

    repo, base = sys.argv[1], sys.argv[2]
    os.chdir(repo)
    assert subprocess.run(["git", "diff", "--quiet", base, "--", "internal"]).returncode == 0, "internal/ differs from BASE"

    src = subprocess.run(["git", "show", f"{base}:tools/check-mutation.py"], capture_output=True, check=True).stdout
    with tempfile.NamedTemporaryFile("wb", suffix=".py", delete=False) as fh:
        fh.write(src)
    spec = importlib.util.spec_from_file_location("old", fh.name)
    old = importlib.util.module_from_spec(spec); spec.loader.exec_module(old)

    path = "tools/mutation-equivalents.txt"
    base_text = subprocess.run(["git", "show", f"{base}:{path}"], capture_output=True, check=True).stdout.decode()
    with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as fh:
        fh.write(base_text)
    old_entries = old.read_equivalents(fh.name)
    assert old.suspect_positions(old_entries, root=".") == [], "BASE keys do not all point at their mutants"

    D13 = [
        " A comment or a var added ANYWHERE above a key moves it, in BOTH adjudication files, and neither pre-commit hook looks.",
        " Re-keying an entry is not done until every file:line INSIDE its body has been rendered against the tree, not just the key at the top.",
    ]
    cut_count = {s: 0 for s in D13}

    def trimmed(raw):
        return raw.strip(b" \t\r"), len(raw) - len(raw.lstrip(b" \t\r"))

    out, n_entries = [], 0
    lines = base_text.split("\n")
    for i, line in enumerate(lines):
        if not line.strip() or line.startswith("#") or line[0].isspace():
            if line[:1].isspace() and line.strip() and not line.lstrip().startswith("#"):
                for s in D13:
                    if s in line:
                        cut_count[s] += 1
                        line = line.replace(s, "", 1)
            out.append(line)
            continue
        pkg, loc, mut = line.split()
        assert pkg == pkg.strip("./").rstrip("/"), pkg
        assert (pkg, loc, mut) in old_entries, line
        file_, ln, col = loc.rsplit(":", 2)
        with open(os.path.join(pkg, file_), "rb") as f:
            flines = f.read().split(b"\n")
        raw = flines[int(ln) - 1]
        code, indent = trimmed(raw)
        offset = int(col) - indent
        assert offset >= 1 and code, line
        copies = sum(1 for r in flines if trimmed(r)[0] == code)
        assert copies >= 1
        nxt = lines[i + 1]
        assert nxt[:1].isspace() and nxt.strip(), f"no reason under {line}"
        indent_str = nxt[: len(nxt) - len(nxt.lstrip())]
        out.append(f"{pkg}  {file_}  {mut}  offset={offset}  copies={copies}")
        out.append(f"{indent_str}code: {code.decode('utf-8')}")
        n_entries += 1

    assert n_entries == len(old_entries) == 36, (n_entries, len(old_entries))
    assert cut_count[D13[0]] == 4 and cut_count[D13[1]] == 1, cut_count
    with open(path, "w") as f:
        f.write("\n".join(out))
    print("converted", n_entries, "entries; D13 cuts:", cut_count)

**`add_nth_go.py`**, `nth` for the Go entries (D15):

    """D15 for Go: add nth to each entry of the converted tools/mutation-equivalents.txt.

    Pairs each entry, in file order, with BASE's entry in the same place (the
    conversion kept the order), and counts the copies of its code at or above
    BASE's line in the tree, whose internal/ equals BASE's. Asserts before it
    writes; touches only header lines. Usage: python3 add_nth_go.py <repo> <BASE>
    """
    import importlib.util, os, subprocess, sys, tempfile
    repo, base = sys.argv[1], sys.argv[2]
    os.chdir(repo)
    assert subprocess.run(["git", "diff", "--quiet", base, "--", "internal"]).returncode == 0
    src = subprocess.run(["git", "show", f"{base}:tools/check-mutation.py"], capture_output=True, check=True).stdout
    with tempfile.NamedTemporaryFile("wb", suffix=".py", delete=False) as fh:
        fh.write(src)
    spec = importlib.util.spec_from_file_location("old", fh.name)
    old = importlib.util.module_from_spec(spec); spec.loader.exec_module(old)
    with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as fh:
        fh.write(subprocess.run(["git", "show", f"{base}:tools/mutation-equivalents.txt"], capture_output=True, check=True).stdout.decode())
    base_keys = list(old.read_equivalents(fh.name))
    path = "tools/mutation-equivalents.txt"
    lines = open(path).read().split("\n")
    heads = [i for i, l in enumerate(lines) if l.strip() and not l.startswith("#") and not l[0].isspace()]
    assert len(heads) == len(base_keys) == 36
    trim = lambda b: b.strip(b" \t\r")
    for i, (pkg, loc, mut) in zip(heads, base_keys):
        parts = lines[i].split()
        assert parts[0] == pkg and parts[2] == mut and parts[1] == loc.rsplit(":", 2)[0], (lines[i], loc)
        assert len(parts) == 5 and parts[3].startswith("offset=") and parts[4].startswith("copies="), lines[i]
        file_, ln, _ = loc.rsplit(":", 2)
        with open(os.path.join(pkg, file_), "rb") as f:
            flines = f.read().split(b"\n")
        code = trim(flines[int(ln) - 1])
        assert lines[i + 1].strip() == "code: " + code.decode("utf-8"), lines[i + 1]
        nth = sum(1 for r in flines[: int(ln)] if trim(r) == code)
        assert 1 <= nth <= int(parts[4].split("=")[1])
        lines[i] = f"{parts[0]}  {parts[1]}  {parts[2]}  {parts[3]}  nth={nth}  {parts[4]}"
    open(path, "w").write("\n".join(lines))
    print("nth added to", len(heads), "entries;", sum(1 for i in heads if "nth=1 " not in lines[i] + " "), "name a copy other than the first")

**`convert_ts.py`**, the TypeScript conversion:

    """D11: convert tools/ts-mutation-equivalents.txt from line:col keys to code keys.

    Reads the entries with BASE's own parser, reads each entry's line in the tree
    (whose client/src must equal BASE's), and rewrites only the entry header lines:
    each becomes the new header plus a `code:` line. Reason lines are carried byte
    for byte except for the D13 sentence, which is cut whole. Asserts before it
    writes. Usage: python3 convert_ts.py <repo> <BASE>
    """
    import importlib.util, os, subprocess, sys, tempfile

    repo, base = sys.argv[1], sys.argv[2]
    os.chdir(repo)
    assert subprocess.run(["git", "diff", "--quiet", base, "--", "client/src"]).returncode == 0, "client/src differs from BASE"
    src = subprocess.run(["git", "show", f"{base}:tools/check-ts-mutation.py"], capture_output=True, check=True).stdout
    with tempfile.NamedTemporaryFile("wb", suffix=".py", delete=False) as fh:
        fh.write(src)
    spec = importlib.util.spec_from_file_location("old", fh.name)
    old = importlib.util.module_from_spec(spec); spec.loader.exec_module(old)

    path = "tools/ts-mutation-equivalents.txt"
    base_text = subprocess.run(["git", "show", f"{base}:{path}"], capture_output=True, check=True).stdout.decode()
    with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as fh:
        fh.write(base_text)
    old_entries = old.read_equivalents(fh.name)
    assert old.suspect_positions(old_entries, root=".") == [], "BASE keys do not all point at their mutants"

    D13 = [" A COMMENT-ONLY EDIT MOVES EVERY KEY BELOW IT, and there are TWO adjudication files — checking "
           "tools/mutation-equivalents.txt and not this one is how check:ts-mutation came to report this key "
           "naming a comment line."]
    cut_count = {s: 0 for s in D13}
    INDENT = " \t\r"
    out, n = [], 0
    lines = base_text.split("\n")
    for i, line in enumerate(lines):
        if not line.strip() or line.startswith("#") or line[0].isspace():
            if line[:1].isspace() and line.strip():
                for s in D13:
                    if s in line:
                        cut_count[s] += 1
                        line = line.replace(s, "", 1)
            out.append(line)
            continue
        file_, where, mut, rep = line.split(maxsplit=3)
        rep = rep.rstrip()
        assert (file_, where, mut, rep) in old_entries, line
        ln, col = (int(x) for x in where.split(":"))
        with open(file_, encoding="utf-8") as f:
            flines = f.read().split("\n")
        raw = flines[ln - 1]
        code = raw.strip(INDENT)
        offset = col - (len(raw) - len(raw.lstrip(INDENT)))
        assert offset >= 1 and code, line
        copies = sum(1 for r in flines if r.strip(INDENT) == code)
        nxt = lines[i + 1]
        assert nxt[:1].isspace() and nxt.strip(), f"no reason under {line}"
        ind = nxt[: len(nxt) - len(nxt.lstrip())]
        out.append(f"{file_}  {mut}  offset={offset}  copies={copies}  {rep}")
        out.append(f"{ind}code: {code}")
        n += 1
    assert n == len(old_entries) == 67, (n, len(old_entries))
    assert cut_count[D13[0]] == 2, cut_count
    with open(path, "w") as f:
        f.write("\n".join(out))
    print("converted", n, "entries; D13 cuts:", cut_count)

**`add_span_nth_ts.py`**, `span` and `nth` for the TypeScript entries (D15,
D16):

    """D15 and D16 for TypeScript: add span and nth to each entry of the converted
    tools/ts-mutation-equivalents.txt.

    Pairs each entry, in file order, with BASE's entry in the same place, finds the
    one live mutant (Survived or Timeout) that BASE's key claimed in the frozen
    Stryker report, and takes span and nth from the branch gate's resolve() over
    the report's own source. Asserts before it writes; touches only header lines.
    Usage: python3 add_span_nth_ts.py <repo> <BASE> <frozen mutation.json>
    """
    import importlib.util, json, os, subprocess, sys, tempfile
    repo, base, snap = sys.argv[1], sys.argv[2], sys.argv[3]
    os.chdir(repo)
    assert subprocess.run(["git", "diff", "--quiet", base, "--", "client/src"]).returncode == 0
    def load(name, data_or_path):
        if isinstance(data_or_path, bytes):
            fh = tempfile.NamedTemporaryFile("wb", suffix=".py", delete=False); fh.write(data_or_path); fh.close(); path = fh.name
        else:
            path = data_or_path
        spec = importlib.util.spec_from_file_location(name, path)
        m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m); return m
    old = load("old", subprocess.run(["git", "show", f"{base}:tools/check-ts-mutation.py"], capture_output=True, check=True).stdout)
    new = load("new", "tools/check-ts-mutation.py")
    with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as fh:
        fh.write(subprocess.run(["git", "show", f"{base}:tools/ts-mutation-equivalents.txt"], capture_output=True, check=True).stdout.decode())
    base_keys = list(old.read_equivalents(fh.name))
    rep = json.load(open(snap))
    sources = {p: f.get("source") for p, f in rep["files"].items()}
    live = [m for m in new.mutants(rep) if m[4] in ("Survived", "Timeout")]
    path = "tools/ts-mutation-equivalents.txt"
    lines = open(path).read().split("\n")
    heads = [i for i, l in enumerate(lines) if l.strip() and not l.startswith("#") and not l[0].isspace()]
    assert len(heads) == len(base_keys) == 67
    for i, (f, where, mut, rp) in zip(heads, base_keys):
        parts = lines[i].split(maxsplit=4)
        assert parts[0] == f and parts[1] == mut and " ".join(parts[4].split()) == rp, (lines[i], (f, where, mut, rp))
        hits = [m for m in live if (m[0], m[1], m[2], m[3]) == (f, where, mut, rp)]
        assert len(hits) == 1, (f, where, mut, rp, hits)
        m = hits[0]
        key = new.resolve(sources, m[0], m[1], m[2], m[3], m[5])
        assert key is not None, m
        _, _, _, offset, span, nth, code = key
        assert parts[2] == f"offset={offset}" and lines[i + 1].strip() == "code: " + code, (lines[i], key)
        lines[i] = f"{parts[0]}  {parts[1]}  {parts[2]}  span={span}  nth={nth}  {parts[3]}  {parts[4]}"
    open(path, "w").write("\n".join(lines))
    print("span and nth added to", len(heads), "entries;",
          sum(1 for i in heads if " nth=1 " not in lines[i]), "name a copy other than the first")

**`respan_ts.py`**, the three multi-line spans recomputed (D16 amended):

    """D16 amended: recompute each TS entry's span with the gate's resolve() after
    multi-line spans left out indentation. Same pairing as add_span_nth_ts.py;
    asserts before it writes; touches only header lines.
    Usage: python3 respan_ts.py <repo> <BASE> <frozen mutation.json>"""
    import importlib.util, json, os, subprocess, sys, tempfile
    repo, base, snap = sys.argv[1], sys.argv[2], sys.argv[3]
    os.chdir(repo)
    def load(name, src):
        fh = tempfile.NamedTemporaryFile("wb", suffix=".py", delete=False); fh.write(src); fh.close()
        spec = importlib.util.spec_from_file_location(name, fh.name); m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m); return m
    old = load("old", subprocess.run(["git", "show", f"{base}:tools/check-ts-mutation.py"], capture_output=True, check=True).stdout)
    new = load("new", open("tools/check-ts-mutation.py", "rb").read())
    with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as fh:
        fh.write(subprocess.run(["git", "show", f"{base}:tools/ts-mutation-equivalents.txt"], capture_output=True, check=True).stdout.decode())
    base_keys = list(old.read_equivalents(fh.name))
    rep = json.load(open(snap)); sources = {p: f.get("source") for p, f in rep["files"].items()}
    live = [m for m in new.mutants(rep) if m[4] in ("Survived", "Timeout")]
    path = "tools/ts-mutation-equivalents.txt"
    lines = open(path).read().split("\n")
    heads = [i for i, l in enumerate(lines) if l.strip() and not l.startswith("#") and not l[0].isspace()]
    assert len(heads) == len(base_keys) == 67
    changed = []
    for i, (f, where, mut, rp) in zip(heads, base_keys):
        parts = lines[i].split(maxsplit=6)
        hits = [m for m in live if (m[0], m[1], m[2], m[3]) == (f, where, mut, rp)]
        assert len(hits) == 1 and parts[0] == f and parts[1] == mut
        key = new.resolve(sources, hits[0][0], hits[0][1], hits[0][2], hits[0][3], hits[0][5])
        assert key is not None and parts[2] == f"offset={key[3]}" and parts[4] == f"nth={key[5]}"
        if parts[3] != f"span={key[4]}":
            changed.append((lines[i][:70], parts[3], key[4]))
            parts[3] = f"span={key[4]}"
            lines[i] = "  ".join(parts[:6]) + "  " + parts[6]
    open(path, "w").write("\n".join(lines))
    print("respanned", len(changed), "entries:", changed)

**`task9_apply.py`**, the cut:

    """Task 9: cut the dated re-point accounts (whole sentences) and header paragraphs,
    and rewrite cross-references by old coordinate, in both adjudication files, from
    task9/{cuts,rewrites,headers}.json reviewed by the implementer. Asserts before
    it writes. Usage: python3 task9_apply.py <repo> <task9-dir>"""
    import json, os, sys
    repo, T = sys.argv[1], sys.argv[2]
    os.chdir(repo)
    cuts = json.load(open(f"{T}/cuts.json")); rewrites = json.load(open(f"{T}/rewrites.json"))
    headers = json.load(open(f"{T}/headers.json"))
    extra = {"file": "tools/ts-mutation-equivalents.txt",
             "first_line": "# CONFIRMED BY A GREEN RUN, 2026-08-25 14:08 (recorded in 76e45c2's message):"}
    for path in ("tools/mutation-equivalents.txt", "tools/ts-mutation-equivalents.txt"):
        lines = open(path).read().split("\n")
        orig = list(lines)
        # reasons: the line after each `code:` line, keyed by the header's line number (1-based)
        my_cuts = [c for c in cuts if c["file"] == path]
        my_rw = [r for r in rewrites if r["file"] == path]
        is_header = lambda r: not r.get("entry_line") or str(r.get("entry", "")).startswith("(header")
        for c in my_cuts + [r for r in my_rw if not is_header(r)]:
            h = c["entry_line"] - 1
            assert lines[h] == c["entry"], (path, c["entry_line"], lines[h][:60], c["entry"][:60])
        reason_idx = {}
        for c in my_cuts:
            h = c["entry_line"] - 1
            assert lines[h + 1].strip() == "code: " + c["code"], (path, c["entry_line"])
            reason_idx.setdefault(h + 2, []).append(c["sentence"])
        for i, sents in reason_idx.items():
            r = lines[i]
            for s in sents:
                n, listed = r.count(s), sents.count(s)
                assert n >= 1 and orig[i].count(s) == listed, (path, i + 1, n, listed, s[:80])
                if (" " + s) in r:
                    r = r.replace(" " + s, "", 1)
                elif (s + " ") in r:
                    r = r.replace(s + " ", "", 1)
                else:
                    r = r.replace(s, "", 1)
            assert r.strip(), (path, i + 1, "empty")
            lines[i] = r
        # rewrites in reasons (entry_line given) or header paragraphs (no entry)
        for rw in my_rw:
            if not is_header(rw):
                i = rw["entry_line"] + 1
                assert lines[i].count(rw["old"]) == 1, (path, i + 1, rw["old"][:60])
                lines[i] = lines[i].replace(rw["old"], rw["new"], 1)
            else:
                hits = [k for k, l in enumerate(lines) if l.startswith("#") and rw["old"] in l]
                assert len(hits) == 1, (path, rw["old"][:60], hits)
                lines[hits[0]] = lines[hits[0]].replace(rw["old"], rw["new"], 1)
        # header paragraphs: from first_line through last_line, plus the "#" separator before it
        paras = [h for h in headers if h["file"] == path]
        if path.endswith("ts-mutation-equivalents.txt"):
            paras.append(extra)
        drop = set()
        for h in paras:
            starts = [k for k, l in enumerate(lines) if l == h["first_line"]]
            assert len(starts) == 1, (path, h["first_line"][:60], starts)
            a = starts[0]
            if "last_line" in h:
                ends = [k for k, l in enumerate(lines) if l == h["last_line"] and k >= a]
                b = ends[0]
            else:
                b = a
                while b + 1 < len(lines) and lines[b + 1].startswith("#") and lines[b + 1].strip() != "#":
                    b += 1
            assert all(lines[k].startswith("#") for k in range(a, b + 1)), (path, a + 1, b + 1)
            drop.update(range(a, b + 1))
            if a > 0 and lines[a - 1].strip() == "#" and (a - 1) not in drop:
                drop.add(a - 1)
        lines = [l for k, l in enumerate(lines) if k not in drop]
        open(path, "w").write("\n".join(lines))
        print(path, "reasons cut in:", len(reason_idx), "sentences:", sum(len(v) for v in reason_idx.values()),
              "rewrites:", len(my_rw), "header lines dropped:", len(drop))

**`task9_check.py`**, Done 6's check:

    """Done 6: each reason after the Task 9 cut is the reason before it less whole
    sentences (once the listed coordinate rewrites are undone). Compares HEAD with
    the working tree. Usage: python3 task9_check.py <repo> <task9-dir>"""
    import json, os, re, subprocess, sys
    repo, T = sys.argv[1], sys.argv[2]
    os.chdir(repo)
    rewrites = json.load(open(f"{T}/rewrites.json"))
    ENDS = tuple('.!?)"`\'')
    bad = 0
    for path in ("tools/mutation-equivalents.txt", "tools/ts-mutation-equivalents.txt"):
        before = subprocess.run(["git", "show", f"HEAD:{path}"], capture_output=True, check=True).stdout.decode().split("\n")
        after = open(path).read().split("\n")
        rb = [l for l in before if l[:1].isspace() and l.strip() and not l.strip().startswith("code: ")]
        ra = [l for l in after if l[:1].isspace() and l.strip() and not l.strip().startswith("code: ")]
        assert len(rb) == len(ra), (path, len(rb), len(ra))
        undo = [(r["new"], r["old"]) for r in rewrites if r["file"] == path]
        # The second review's corrections, also rewrites rather than cuts.
        round2 = json.load(open(os.path.join(T, "..", "task9_round2.json")))
        undo += [(new, old) for old, new in round2.get(path, [])]
        if path.endswith("ts-mutation-equivalents.txt"):
            # One more rewrite, of a source position, made while reading the cut.
            undo.append(("the ELSE of `label(a, a.actorId) === label(b, b.actorId)` — so",
                         "the ELSE of `label(a, a.actorId) === label(b, b.actorId)` on the line above — so"))
        changed = 0
        for b, a in zip(rb, ra):
            for new, old in undo:
                if new in a:
                    a = a.replace(new, old, 1)
            if a == b:
                continue
            changed += 1
            # walk b, consuming a; every skipped stretch of b must be whole sentences
            i = j = 0
            while i < len(b):
                if j < len(a) and b[i] == a[j]:
                    i += 1; j += 1; continue
                # skip in b to the next point where the rest of a resumes
                k = b.find(a[j:j + 40], i) if j < len(a) else len(b)
                if k < 0:
                    bad += 1; print("MISMATCH", path, b[:60]); break
                gap = b[i:k].strip()
                prev = b[:i].rstrip()
                if not gap or not gap.endswith(ENDS) or (prev and not prev.endswith(ENDS)) or gap[0].islower():
                    bad += 1; print("NOT A WHOLE SENTENCE", path, repr(gap[:80])); break
                i = k
            if j < len(a):
                pass
        print(path, "reasons:", len(rb), "changed:", changed)
    print("not whole sentences:", bad)
    sys.exit(1 if bad else 0)

**`breaks_go.py`**, the Go gate's 31 breaks, each with the tests it must turn
red:

    """Task 4: break the Go gate one way at a time in a scratch copy; each named test must fail.
    Usage: python3 breaks_go.py <repo> <scratch-dir>"""
    import os, re, shutil, subprocess, sys
    repo, work = sys.argv[1], sys.argv[2]
    shutil.rmtree(work, ignore_errors=True)
    os.makedirs(work)
    subprocess.run(f"git -C {repo} archive HEAD internal | tar -x -C {work}", shell=True, check=True)
    os.makedirs(os.path.join(work, "tools"))
    for f in ("check-mutation.py", "check_mutation_test.py", "mutation-equivalents.txt"):
        shutil.copy(os.path.join(repo, "tools", f), os.path.join(work, "tools", f))
    gate = os.path.join(work, "tools", "check-mutation.py")
    clean = open(gate).read()

    def run_tests(names):
        out = {}
        for n in names:
            p = subprocess.run([sys.executable, "-B", "check_mutation_test.py", "-k", n], cwd=os.path.join(work, "tools"),
                               capture_output=True, text=True)
            ran = re.search(r"Ran (\d+) test", p.stderr)
            assert ran and int(ran.group(1)) >= 1, (n, p.stderr[-300:])
            out[n] = "FAIL" if p.returncode != 0 else "pass"
        return out

    BREAKS = [
     ("B1 position-free match", [("    return (name, file_, mutator, col - indent, nth, text)", "    return (name, file_, mutator, col - indent, nth, f\"{line}@{text}\")")],
      ["test_a_survivor_whose_line_moved_is_still_claimed"]),
     ("B2 code in the key", [("    return (e.package, e.file, e.mutator, e.offset, e.nth, e.code)", "    return (e.package, e.file, e.mutator, e.offset, e.nth)"),
                             ("    return (name, file_, mutator, col - indent, nth, text)", "    return (name, file_, mutator, col - indent, nth)")],
      ["test_a_survivor_on_other_code_at_an_entrys_old_position_is_not_claimed", "test_a_survivor_on_a_different_statement_is_not_paired"]),
     ("B3 offset from the trimmed line", [("    return (name, file_, mutator, col - indent, nth, text)", "    return (name, file_, mutator, col, nth, text)")],
      ["test_a_reindented_line_is_still_claimed", "test_a_tab_indented_survivor_resolves_to_its_offset_in_the_trimmed_line"]),
     ("B4 offset in the key", [("    return (e.package, e.file, e.mutator, e.offset, e.nth, e.code)", "    return (e.package, e.file, e.mutator, e.nth, e.code)"),
                               ("    return (name, file_, mutator, col - indent, nth, text)", "    return (name, file_, mutator, nth, text)")],
      ["test_two_mutants_of_one_mutator_on_one_line_are_two_claims"]),
     ("B5 mutator in the key", [("    return (e.package, e.file, e.mutator, e.offset, e.nth, e.code)", "    return (e.package, e.file, None, e.offset, e.nth, e.code)"),
                                ("    return (name, file_, mutator, col - indent, nth, text)", "    return (name, file_, None, col - indent, nth, text)")],
      ["test_adjudication_is_matched_on_all_three_fields"]),
     ("B6 package in the match", [("        mine = [e for e in entries if e.package == name]", "        mine = list(entries)"),
                                  ("    return (e.package, e.file, e.mutator, e.offset, e.nth, e.code)", "    return (None, e.file, e.mutator, e.offset, e.nth, e.code)"),
                                  ("    return (name, file_, mutator, col - indent, nth, text)", "    return (None, file_, mutator, col - indent, nth, text)")],
      ["test_survivor_in_a_different_package_is_not_pre_approved"]),
     ("B7 counted match", [("        n = min(len(named), len(lived))", "        n = len(lived) if named else 0")],
      ["test_one_entry_claims_one_of_two_survivors_on_its_key"]),
     ("B8 survivors before timeouts", [("        n = min(len(named), len(lived))\n        claimed += lived[:n]\n        unadjudicated += [(key, m) for m in lived[n:]]\n        k = min(len(named) - n, len(hung))\n        claimed += hung[:k]\n",
                                        "        k = min(len(named), len(hung))\n        claimed += hung[:k]\n        n = min(len(named) - k, len(lived))\n        claimed += lived[:n]\n        unadjudicated += [(key, m) for m in lived[n:]]\n")],
      ["test_claims_takes_survivors_before_timeouts"]),
     ("B9 a timeout keeps its entry live", [("        k = min(len(named) - n, len(hung))", "        k = 0")],
      ["test_an_adjudication_whose_mutant_timed_out_is_not_called_stale"]),
     ("B10 leftover entries are stale", [("        stale += named[n + k:]", "        pass")],
      ["test_stale_entry_fails", "test_two_entries_for_one_code_and_one_survivor_leave_one_stale"]),
     ("B11 resolve against the measured source", [("        lived, cannot = [], []\n", "        source = package_source(root, pkg)\n        lived, cannot = [], []\n")],
      ["test_a_survivor_is_resolved_against_the_source_the_run_measured"]),
     ("B12 a reused verdict is still matched", [("            survivors, timed_out = reused\n", "            survivors, timed_out = [], reused[1]\n")],
      ["test_a_reused_survivor_still_faces_the_adjudication_check"]),
     ("B13 an unreadable survivor fails closed", [("                cannot.append((name, location, mutator))", "                pass")],
      ["test_a_survivor_whose_line_cannot_be_read_fails_the_gate"]),
     ("B14 code that stands nowhere is reported", [("    if n != e.copies:", "    if n and n != e.copies:")],
      ["test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported"]),
     ("B15 other direction: a sound entry stays silent", [("    if n != e.copies:", "    if n <= e.copies:")],
      ["test_a_sound_file_of_keys_does_not_red_the_gate", "test_every_real_adjudication_still_points_at_its_mutant"]),
     ("B16 copies checked", [("    if n != e.copies:", "    if False:"), ("    if e.nth > e.copies:", "    if False:")],
      ["test_an_entry_whose_copies_differ_from_the_tree_is_reported", "test_an_entry_whose_nth_exceeds_its_copies_is_reported"]),
     ("B17 comment rule", [("    if code.startswith(COMMENT_PREFIXES):", "    if False:")],
      ["test_a_key_on_a_line_comment_is_rejected", "test_a_key_on_a_block_comment_is_rejected", "test_a_key_on_a_doc_comment_continuation_is_rejected"]),
     ("B18 token rule", [("    if not token_at(code, e.offset, tokens):", "    if False:")],
      ["test_a_key_whose_column_holds_a_token_the_mutator_cannot_apply_to"]),
     ("B19 other direction: a right token is accepted", [("    if not token_at(code, e.offset, tokens):", "    if True:")],
      ["test_the_same_column_is_accepted_for_the_mutator_that_does_live_there"]),
     ("B20 unknown mutator fails closed", [("    if tokens is None:\n        return (", "    if tokens is None:\n        return \"\"\n        return (")],
      ["test_a_mutator_with_no_rule_is_fatal_rather_than_silently_skipped"]),
     ("B21a parser: a missing code line", [("                    if not text.startswith(\"code: \"):", "                    if False:")],
      ["test_an_entry_without_a_code_line_is_fatal"]),
     ("B21b parser: # mid-line is a comment", [("            line = raw.rstrip(\"\\r\\n\")", "            line = raw.split(\"#\", 1)[0].rstrip(\"\\r\\n\") if not raw.startswith(\"#\") else raw.rstrip(\"\\r\\n\")")],
      ["test_a_hash_inside_code_is_not_a_comment"]),
     ("B21c parser: normalise the package silently", [("            if parts[0] != _norm_pkg(parts[0]):\n                raise", "            parts[0] = _norm_pkg(parts[0])\n            if False:\n                raise")],
      ["test_a_package_not_written_as_survivors_carry_it_is_refused"]),
     ("B24 nth in the key", [("    return (e.package, e.file, e.mutator, e.offset, e.nth, e.code)", "    return (e.package, e.file, e.mutator, e.offset, e.code)"),
                             ("    return (name, file_, mutator, col - indent, nth, text)", "    return (name, file_, mutator, col - indent, text)")],
      ["test_an_entry_on_one_copy_does_not_claim_another_copys_survivor"]),
     ("B25 two entries on one key refused", [("    if sharing > 1:", "    if False:")],
      ["test_two_entries_for_one_copy_are_reported"]),
     ("B26 the package is read recursively", [("    for dirpath, _, names in os.walk(d):", "    for dirpath, _, names in [(d, None, os.listdir(d) if os.path.isdir(d) else [])]:")],
      ["test_a_survivor_in_an_ungated_subdirectory_is_claimed"]),
     ("B27 the ready entry starts in column 1", [("              f\"{name}  {key[1]}  {mutator}  offset=", "              f\"    {name}  {key[1]}  {mutator}  offset=")],
      ["test_an_unadjudicated_survivor_is_printed_as_a_ready_entry"]),
     ("B23 a column outside the line fails closed", [("    if not text or not 1 <= col - indent <= len(code):", "    if not text:")],
      ["test_a_survivor_outside_its_trimmed_line_fails_the_gate"]),
     ("B28 the placeholder is refused as a reason", [("                if text == PLACEHOLDER:", "                if False:")],
      ["test_the_printed_placeholder_is_refused_as_a_reason"]),
     ("B29 the code line is trimmed as source is", [("                    text = line.lstrip(\" \\t\").rstrip(\" \\t\\r\")", "                    text = line.strip()")],
      ["test_a_code_line_keeps_a_trailing_no_break_space"]),
     ("B22 ready entry in the message", [("              f\"    code: {key[5]}\\n\"\n", "")],
      ["test_an_unadjudicated_survivor_is_printed_as_a_ready_entry"]),
    ]

    assert all(v == "pass" for v in run_tests(sorted({n for _, _, ns in BREAKS for n in ns})).values()), "a named test fails on the clean gate"
    bad = 0
    for label, edits, names in BREAKS:
        broken = clean
        for old, new in edits:
            assert broken.count(old) == 1, (label, old)
            broken = broken.replace(old, new)
        assert broken != clean
        open(gate, "w").write(broken)
        res = run_tests(names)
        ok = all(v == "FAIL" for v in res.values())
        bad += not ok
        print(f"{'ok ' if ok else 'BAD'} {label}: " + ", ".join(f"{n} {v}" for n, v in res.items()))
    open(gate, "w").write(clean)
    p = subprocess.run([sys.executable, "-B", "check_mutation_test.py"], cwd=os.path.join(work, "tools"), capture_output=True, text=True)
    print("restored, whole suite:", p.stderr.strip().splitlines()[-1])
    sys.exit(1 if bad else 0)

**`breaks_ts.py`**, the TypeScript gate's 37 breaks:

    """Task 7: break the TS gate one way at a time in a scratch copy; each named test must fail.
    Usage: python3 breaks_ts.py <repo> <scratch-dir>"""
    import os, re, shutil, subprocess, sys
    repo, work = sys.argv[1], sys.argv[2]
    shutil.rmtree(work, ignore_errors=True)
    os.makedirs(work)
    subprocess.run(f"git -C {repo} archive HEAD client/src | tar -x -C {work}", shell=True, check=True)
    os.makedirs(os.path.join(work, "tools"))
    for f in ("check-ts-mutation.py", "check_ts_mutation_test.py", "ts-mutation-equivalents.txt"):
        shutil.copy(os.path.join(repo, "tools", f), os.path.join(work, "tools", f))
    gate = os.path.join(work, "tools", "check-ts-mutation.py")
    clean = open(gate).read()

    def run_tests(names):
        out = {}
        for n in names:
            p = subprocess.run([sys.executable, "-B", "check_ts_mutation_test.py", "-k", n],
                               cwd=os.path.join(work, "tools"), capture_output=True, text=True)
            ran = re.search(r"Ran (\d+) test", p.stderr)
            assert ran and int(ran.group(1)) >= 1, (n, p.stderr[-300:])
            out[n] = "FAIL" if p.returncode != 0 else "pass"
        return out

    KEY = "    return (e.file, e.mutator, e.replacement, e.offset, e.span, e.nth, e.code)"
    RES = "    return (path, mutator, replacement, offset, span, nth, code)"
    SRC = '    sources = {path: entry.get("source") for path, entry in report.get("files", {}).items()}'
    BREAKS = [
     ("T1 position-free match", [(RES, RES.replace("code)", 'f"{line}@{code}")'))],
      ["test_a_survivor_whose_line_moved_is_still_claimed"]),
     ("T2 code in the key", [(KEY, KEY.replace(", e.code)", ")")), (RES, RES.replace(", code)", ")"))],
      ["test_a_conditional_expression_entry_does_not_claim_other_code",
       "test_a_survivor_of_the_entrys_mutator_on_another_statement_is_not_claimed"]),
     ("T3 offset from the trimmed line", [("    offset = col - (len(raw) - len(raw.lstrip(INDENT)))\n", "    offset = col\n")],
      ["test_a_reindented_line_is_still_claimed"]),
     ("T4 offset in the key", [(KEY, KEY.replace(" e.offset,", "")), (RES, "    return (path, mutator, replacement, code)")],
      ["test_two_mutants_of_one_mutator_on_one_line_are_two_claims"]),
     ("T5 mutator in the key", [(KEY, KEY.replace("e.mutator", "None")), (RES, RES.replace("(path, mutator,", "(path, None,"))],
      ["test_an_adjudication_is_keyed_by_mutator_not_just_location"]),
     ("T6 file in the key", [(KEY, KEY.replace("(e.file,", "(None,")), (RES, RES.replace("(path, mutator,", "(None, mutator,"))],
      ["test_the_same_code_in_another_file_is_not_claimed"]),
     ("T7 counted match", [("        n = min(len(named), len(lived))", "        n = len(lived) if named else 0")],
      ["test_one_entry_claims_one_of_two_survivors_on_its_key"]),
     ("T8 survivors before timeouts", [("        n = min(len(named), len(lived))\n        claimed += lived[:n]\n        unadjudicated += [(key, m) for m in lived[n:]]\n        k = min(len(named) - n, len(hung))\n        claimed += hung[:k]\n",
                                        "        k = min(len(named), len(hung))\n        claimed += hung[:k]\n        n = min(len(named) - k, len(lived))\n        claimed += lived[:n]\n        unadjudicated += [(key, m) for m in lived[n:]]\n")],
      ["test_claims_takes_survivors_before_timeouts"]),
     ("T9 a timeout keeps its entry live", [("        k = min(len(named) - n, len(hung))", "        k = 0")],
      ["test_an_adjudication_whose_mutant_timed_out_is_not_called_stale"]),
     ("T10 leftover entries are stale", [("        stale += named[n + k:]", "        pass")],
      ["test_an_adjudication_for_a_mutant_that_no_longer_survives_fails",
       "test_two_entries_for_one_code_and_one_survivor_leave_one_stale"]),
     ("T13 an unreadable survivor fails closed", [("            unreadable.append(m)", "            pass")],
      ["test_a_survivor_whose_line_is_not_in_the_source_fails_the_gate"]),
     ("T14 code that stands nowhere is reported", [("    if n != e.copies:", "    if n and n != e.copies:")],
      ["test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported"]),
     ("T15 other direction: a sound entry stays silent", [("    if n != e.copies:", "    if n <= e.copies:")],
      ["test_two_entries_on_two_identical_lines_claim_both", "test_every_real_adjudication_still_points_at_its_mutant"]),
     ("T16 copies checked", [("    if n != e.copies:", "    if False:"), ("    if e.nth > e.copies:", "    if False:")],
      ["test_an_entry_whose_copies_differ_from_the_tree_is_reported", "test_an_entry_whose_nth_exceeds_its_copies_is_reported"]),
     ("T17 comment rule", [("    if code.startswith(COMMENT_PREFIXES):", "    if False:")],
      ["test_a_key_on_a_line_comment_is_rejected", "test_a_key_on_a_doc_comment_is_rejected",
       "test_a_key_on_a_doc_comment_continuation_is_rejected"]),
     ("T18 token rule", [("    if not operator_at(code[off:], e.replacement.encode(), tokens):", "    if False:")],
      ["test_an_operator_key_is_checked_against_its_own_replacement_text"]),
     ("T19 other direction: a right token is accepted", [("    if not operator_at(code[off:], e.replacement.encode(), tokens):", "    if True:")],
      ["test_an_operator_key_is_checked_against_its_own_replacement_text", "test_a_column_is_counted_the_way_stryker_counts_it"]),
     ("T20 unknown mutator fails closed", [("    if tokens is None:\n        return (", "    if tokens is None:\n        return \"\"\n        return (")],
      ["test_an_unknown_mutator_is_fatal_rather_than_silently_skipped"]),
     ("T21 replacement in the key", [(KEY, KEY.replace(" e.replacement,", "")), (RES, RES.replace(" replacement,", ""))],
      ["test_two_mutants_at_one_location_are_two_separate_claims",
       "test_a_survivor_with_another_replacement_at_the_entrys_code_is_not_claimed"]),
     ("T22 code from the report's source", [(SRC, '    sources = {path: ((Path(root) / path).read_text() if (Path(root) / path).exists() else None) for path in report.get("files", {})}')],
      ["test_a_survivor_is_resolved_against_the_reports_own_source"]),
     ("T23 missing source fails closed", [(SRC, '    sources = {path: entry.get("source", (Path(root) / path).read_text() if (Path(root) / path).exists() else None) for path, entry in report.get("files", {}).items()}')],
      ["test_a_report_file_without_its_source_fails_the_gate"]),
     ("T24 UTF-16 offset", [("    off = byte_offset(code, e.offset)", "    off = e.offset - 1")],
      ["test_a_column_is_counted_the_way_stryker_counts_it"]),
     ("T25 operator and sibling rules", [("        if was and now and tail[j + len(was):].startswith(replacement[j + len(now):]):", "        if was and now:")],
      ["test_a_key_on_the_sibling_comparison_is_rejected"]),
     ("T26 string-opener rule", [("        if opener not in (b'\"', b\"'\", b\"`\"):", "        if False:")],
      ["test_a_string_key_whose_column_does_not_open_a_string_is_rejected"]),
     ("T27 # is text", [("        if not line.strip() or line.startswith(\"#\"):\n            continue\n        if line[0].isspace():",
                         "        line = line if line.startswith(\"#\") else line.split(\"#\", 1)[0].rstrip()\n        if not line.strip() or line.startswith(\"#\"):\n            continue\n        if line[0].isspace():")],
      ["test_a_hash_in_a_replacement_or_reason_is_kept"]),
     ("T28 parser: a missing code line", [("                if not text.startswith(\"code: \"):", "                if False:")],
      ["test_an_entry_without_a_code_line_is_fatal"]),
     ("T32 nth in the key", [(KEY, KEY.replace(" e.nth,", "")), (RES, RES.replace(" nth,", ""))],
      ["test_an_entry_on_one_copy_does_not_claim_another_copys_survivor"]),
     ("T33 span in the key", [(KEY, KEY.replace(" e.span,", "")), (RES, RES.replace(" span,", ""))],
      ["test_two_mutants_at_one_start_with_different_extents_are_two_claims"]),
     ("T34 a missing end fails closed", [("    if not isinstance(text, str) or end is None:", "    end = end or where.split(\":\")[0] + \":\" + str(int(where.split(\":\")[1]) + 1)\n    if not isinstance(text, str):")],
      ["test_a_survivor_without_an_end_fails_the_gate"]),
     ("T35 two entries on one key refused", [("    if sharing > 1:", "    if False:")],
      ["test_two_entries_for_one_copy_are_reported"]),
     ("T36 a multi-line span leaves out indentation", [("        span = _units(\"\\n\".join(covered))", "        span = (_units(raw) - (col - 1) + 1 + sum(_units(x) + 1 for x in lines[line:eline - 1]) + ecol - 1)")],
      ["test_a_reindented_multi_line_mutant_is_still_claimed"]),
     ("T37 the placeholder is refused as a reason", [("            if text == PLACEHOLDER:", "            if False:")],
      ["test_the_printed_placeholder_is_refused_as_a_reason"]),
     ("T38 the code line is trimmed as source is", [("                text = line.lstrip(INDENT).rstrip(INDENT)", "                text = line.strip()")],
      ["test_a_code_line_keeps_a_trailing_form_feed_and_no_break_space"]),
     ("T39 entries split on line feeds only", [("    for lineno, line in enumerate(Path(path).read_text().split(\"\\n\"), 1):", "    for lineno, line in enumerate(Path(path).read_text().splitlines(), 1):")],
      ["test_a_code_line_keeps_a_trailing_form_feed_and_no_break_space"]),
     ("T30 a column outside the line fails closed", [("    if not code or not 1 <= offset <= _units(code):", "    if not code:")],
      ["test_a_survivor_outside_its_trimmed_line_fails_the_gate"]),
     ("T31 the ready entry starts in column 1", [("              f\"{path}  {mutator}  offset=", "              f\"    {path}  {mutator}  offset=")],
      ["test_an_unadjudicated_survivor_is_printed_as_a_ready_entry"]),
     ("T29 ready entry in the message", [("              f\"    code: {key[6]}\\n\"\n", "")],
      ["test_an_unadjudicated_survivor_is_printed_as_a_ready_entry"]),
    ]
    assert all(v == "pass" for v in run_tests(sorted({n for _, _, ns in BREAKS for n in ns})).values()), "a named test fails on the clean gate"
    bad = 0
    for label, edits, names in BREAKS:
        broken = clean
        for old, new in edits:
            assert broken.count(old) == 1, (label, old)
            broken = broken.replace(old, new)
        open(gate, "w").write(broken)
        res = run_tests(names)
        ok = all(v == "FAIL" for v in res.values())
        bad += not ok
        print(f"{'ok ' if ok else 'BAD'} {label}: " + ", ".join(f"{n} {v}" for n, v in res.items()))
    open(gate, "w").write(clean)
    p = subprocess.run([sys.executable, "-B", "check_ts_mutation_test.py"], cwd=os.path.join(work, "tools"), capture_output=True, text=True)
    print("restored, whole suite:", p.stderr.strip().splitlines()[-1])
    sys.exit(1 if bad else 0)
