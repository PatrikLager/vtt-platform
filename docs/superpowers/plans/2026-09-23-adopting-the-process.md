# Adopting the process — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-23-adopting-the-process-design.md`
**Verified:** 2026-09-23, by `verify-ticket`, an agent that did not write the
ticket. Verdict: **Passes with gaps.** The gaps are listed at the end of this
plan; they travel with it. This plan does not edit the ticket.

**Narrowed 2026-09-23, after Patrik's ruling that the work is scoped per arc.**
This ticket is the prerequisite only: the tree committed, the old id scheme out
of the documents, the register in the dispenser's shape, the id record written.
The chain gate and the register rows leave this plan for the first arc's ticket
(joining-a-table): Tasks 4, 5 and 6 and decisions D5, D6, D7, D8, D9, D12 and
D17 are NOT executed here. They stay in this file as the verified design of the
gate, for that arc's plan to start from. Task 7 keeps the specification, whose
Status says the chain check is the half not yet implemented and names the ticket
carrying it; Tasks 7 and 8 otherwise stand. The ticket was narrowed to match
after the verification, by its writer; the verification covered the superset.
The "Done item" numbers below are the superset ticket's: the narrowed ticket's
items 4 to 9 are this plan's 5 to 10, its items 1 to 3 are unchanged, and D2's
commit 5 leaves with the gate, so five commits, not six. After the reading
review, per Patrik's single-source-of-truth ruling of 2026-09-23, the register
prose and `CLAUDE.md`'s register bullet point at SPEC-008 instead of restating
it, and `contract/README.md` points at SPEC-007 instead of restating the wire
conventions; both deviate from Task 2 as written and the report records it.

**Goal, in the ticket's words:** the tree is committed and the register has one
id scheme.

**MapTool (CLAUDE.md rule 9):** MapTool keeps no requirements register and no
citation chain between rules, tests and records. There is nothing to borrow and
nothing to reject; this is process tooling, not tabletop geometry.

---

## Constraints that bind every task

The project's standing rules, by number, from `CLAUDE.md`:

- **Rule 2.** The chain gate is ADDED. No existing gate, threshold, exemption or
  scope is loosened to get a green run. The mutation position check on
  `tools/mutation-equivalents.txt` must stay clean at every commit.
- **Rule 8.** Every citation this work writes — in the register, the debt
  entry, the specification, the report, the self-test's comments — names a
  test, a file, a symbol or a dated decision. Never a line number. This plan
  obeys the same rule.
- **Rule 9.** Answered above.
- Rules 3, 4 and 5 are not engaged: no contract, fold or platform code changes.
  The vocabulary gate (`check:invariants`) scans `internal/` and `cmd/` only,
  so a Python tool under `tools/` is outside it.

The process's own rules, from the installed `dev-cycle` package (0.4.0):

- Requirement ids come from the package's dispenser, `requirement-id`, and from
  nothing else (process SPEC-007; Patrik's ruling 2026-09-23). They are
  allocated AFTER Phase 2 sign-off of this plan, never on the ticket
  (`requirements` skill). Never run the dispenser against the real register to
  see what it does: copy it to the scratchpad first. A burned id is permanent.
- Tests before code (ADR-009, and the package): the chain gate's self-test is
  red before the checker exists.
- One review per commit, recorded with the package's `review-record.sh`; the
  commit gate (`review-gate` in `.lefthook.yml`) refuses without it. Nothing
  reaches history before Phase 4 sign-off.
- The report is written per the `implementation-report` skill, after the
  commits, against the ticket's ten "Done looks like" items.

Patrik's rulings, taken as given for this work:

- The fourteen existing rows are **replaced, not migrated.** The register
  restarts at `VTT-001`.
- **Delete the old solution first.** No phased removal, no migration route.
- The commit gate is chained in `.lefthook.yml` as one line (`review-gate`).
  Done and verified; this plan commits it and does not redesign it.

Two records that constrain HOW documents are edited, engaged by decision 4:

- `CLAUDE.md`: `docs/adr/` is frozen evidence, never rewritten.
- The `requirements` skill: a report is edited for exactly two reasons — it was
  wrong about its own ticket, or it names code in a way the rules forbid.

An environmental precondition, measured 2026-09-23: `task check`'s mutation
gate refuses below 16 GiB free on the temp volume, and its self-test's disk
guard fires before the tested code runs (two failures today at 15.3 GiB free,
both the guard). `go clean -cache` is the remedy the guard prints. Task 0
measures it before anything else.

---

## Decisions this plan makes

Each with the constraint or measurement that forced it. Two are preferences and
say so.

**D1. The register is cut to its shape, then refilled only by the dispenser,
only after sign-off.** Forced by: the ruling (replaced, not migrated); the
`requirements` skill (ids after sign-off); and the measurement that the
dispenser, run on a copy of today's register, allocates `VTT-001` beside the
fourteen rows and sees none of them. The shape is `project: VTT`, short prose
stating what the evidence column may hold, and the `| Id | Requirement |
Verified by |` header. The dispenser was run on a copy of that shape and
allocated `VTT-001`, then `VTT-002`; with prose below the table it still
appended to the table.

**D2. Six reviewed commits, in this order.** Forced by the ticket's ordering
("the ids leave together, so that no committed tree cites an id its register
does not define"), by the commit gate (one record per tree), and by the
four-commit sequence already presented to Patrik for the document half.

1. `tools/mutation-equivalents.txt` alone.
2. The process's arrival: `.lefthook.yml`, `.claude/settings.json`,
   `CLAUDE.md`, `docs/requirements.md` (shape only), `docs/verification-debt.md`,
   the visibility ticket, this ticket and this plan.
3. The 2026-09-19 documents: the eight arc reports (the visibility report minus
   two parentheticals) and ADR-011 (closing paragraph reworded).
4. `docs/specifications/007-the-wire-contract.md`.
5. The chain gate, its self-test, the Taskfile wiring, and the register rows
   with their evidence — one commit, test and implementation together.
6. The new specification, the two deletions, and the report.

After commit 4 the only untracked paths are `docs/ADOPTING-THE-PROCESS.md` and
`docs/reports/DERIVED-contract-experiment.md`; `ADOPTING` holds the in-tree
record of the ruling until the specification exists, so it goes last, as the
ticket orders.

**D3. Done item 2 is observed with the ticket, this plan and the report
excluded.** Forced by measurement: the ticket's own problem statement carries
three lines matching its item-2 pattern (the prefixed form, and the bare form
in the same sentences), so the command as written can never print
nothing while the ticket sits under `docs/`. The observation this plan uses:

    grep -rnE '\bVTT-[A-Z]+-[0-9]{3}\b|(^|[^A-Z-])(VIS|LOG|JOIN)-[0-9]{3}\b' \
      --exclude='2026-09-23-adopting-the-process*' docs CLAUDE.md tools

The report is named so the same exclusion covers it, and this plan and the
report describe the old forms by shape (`VTT-<SUBJECT>-NNN`), never by
instance. Whether the ticket's own item gains the exclusion is its writer's
call; a question at the end.

**D4. ADR-011 and the visibility report are edited before their first commit,
and never after.** Forced by: the ticket lists both among the files the ids
leave from; both are untracked, so no committed form exists to freeze; and the
`requirements` skill's second permitted reason applies anyway — each names an
id that will resolve to nothing. From commit 3 on, both are frozen as
`CLAUDE.md` says.

**D5. The chain gate reads citations from test files only, under every root
that holds tests.** Test files are `*_test.go`, `*.test.ts` and `*_test.py`
under `internal/`, `cmd/`, `client/`, `tools/` and `contract/`, skipping
`node_modules`, `gen`, `contract-spike`, `.git`, `.superpowers`, `.stryker-tmp`
and `.tscov`. Specifications are `docs/specifications/*.md`. Rows come from
`docs/requirements.md`. Forced by the ticket's rule ("cited by a test") and the
process's own chain test, which bounds its scan because "cited anywhere" sweeps
in the tickets' prose and fixture data. `contract/` is added to the ticket's
four roots because it holds `contract_test.go`, `roundtrip_test.go` and
`events.test.ts` (measured), and the rule says "a test", not "a test under
four directories". The ticket's item-4 observations are unchanged by the
addition.

**D6. The gate also refuses a row whose id is not `<tag>-NNN` after the
dispenser's markup set is stripped, a duplicated row id, and a blank evidence
cell.** Forced by: Done item 3's shape is held by nothing once this ticket
lands unless the gate holds it; the dispenser strips backticks, bold and
underscores when it counts, so the gate reads the same way; and process
SPEC-007 says a blank evidence cell "is the one state the column may not be
in". `**OPEN — no test yet**` and `**READING — <review>**` are skipped BY NAME;
anything else is read as comma-separated repo-relative paths, so a sentence in
the evidence cell fails as a missing file, which is the catch.

**D7. Nothing scanned is a refusal, an empty table is a pass.** Exit 2 when the
register is missing, declares no `project:` tag, or has no `| Id | Requirement
|` header, and when zero test files are found. Zero rows is silent. Forced by
this repository's tools convention (`check_doc_owner_test.py`'s
`test_an_empty_scan_is_a_failure_not_a_pass`) and the process chain test's
header: a check that reds on an empty register "enforces having requirements,
which is a step and not a defect".

**D8. The self-test's fixtures carry a tag that is not the project's, and the
self-test cites the real ids in comments.** Forced by D5: the gate scans
`tools/*_test.py`, so a literal `VTT-999` in a fixture string would be a
citation the gate reds on. Fixtures use `project: TT` and `TT-NNN`; process
SPEC-007 names the tag as what keeps a fixture apart from a citation. The
ticket's `VTT-999` cases are demonstrated on the real tree by fault injection
in Task 5 and recorded in the report, not encoded as literals. The real ids
are cited the way the spec says a test cites: the bare id in a comment on the
line above the case it holds.

**D9. The gate is wired into `check:` only, after `check:doc-owner`; the
pre-commit hook is unchanged.** A PREFERENCE, named as one. The ticket asks
for a `task check` step; `CLAUDE.md`'s gate paragraph enumerates the hook's
steps, and adding a hook step is a second change to that record beyond the
ticket. It runs in well under a second and would fit the hook; a question at
the end. `check:fast`'s enumerated skip list gains the name, because that desc
says its names "are what rots" and has rotted twice.

**D10. The new specification is `docs/specifications/008-requirement-ids-come-from-the-dispenser.md`.**
A PREFERENCE. The `specification` skill says "next free number, never reuse";
only `007` exists, so `001` is free and `008` is the next above the highest.
A number below an existing one reads as having siblings. A question at the
end.

**D11. The commit gate's wiring is recorded in `CLAUDE.md`'s gate paragraph and
gets no specification of its own in this ticket.** Forced by the ticket's
`Specifications this moves`, which names exactly one `New:` record. A second
record (one decision, one file) is offered as a question.

**D12. What this plan fixes about evidence is where citations can go; what each
row's evidence IS is the sort's to decide after sign-off.** The two chain
rules and the shape half of the id rule can be held by
`tools/check_requirements_chain_test.py`; the "written by the dispenser and by
nothing else" half is a reading, because a hand-typed flat id is
indistinguishable from a dispensed one; the commit-gate rule has no in-tree
test and no test is planned by the ticket, so its row is `**OPEN — no test
yet**` and the item-9 observation is repeated in the report. Forced by: the
register may not hold a blank (D6), and the `requirements` skill names exactly
three outcomes for a rule no check holds.

**D13. Spec 007's `Requirements` line becomes a sentence that stays true at
every row count.** Forced by the ticket: "carries `project: VTT` and no rows"
is false today, true between commits 2 and 5, and false again after the first
allocation. The replacement: none allocated for this record; the contract
arc's rules are re-derived from its ticket's exit criteria by a later ticket.

**D14. The shut-door entry in `docs/verification-debt.md` is written from the
register's own section and the joining report's "One open item", without
ids.** Forced by Done items 6 and 2. Labels: `test asserts nothing` (the
ticket's) and `outside the tool` (the register's section measured that
removing the shut-door term leaves the Go suite green, and the mutation gate
cannot express a whole term's disappearance).

**D15. The visibility ticket keeps the rule prose its uncommitted diff added and
loses everything about ids.** Forced by Done item 5 (the eight sentences stay,
once each) and Done item 2. Measured: `HEAD`'s copy of the file carries no id
at all; the diff adds eight bracketed tags, five inline ids inside the
one-code-path block and the id paragraphs, and two rule passages (the roster
refusal, the one code path). The ticket says "a paragraph on how ids are
assigned"; there are four consecutive ones, and all four go.

**D16. `tools/mutation-equivalents.txt` is committed alone and first.** Forced
by the standing lesson that any source edit moves keys, and measured today:
`read_equivalents` reads 36 entries and `suspect_positions` reports none over
the working tree. The re-pointed key is consistent with the tree it sits in.

**D17. The first QA run under the process happens in Task 5, and its
adjudications have no home.** `CLAUDE.md` says so and says the first run must
give them one. This plan does not choose the home; a question at the end, with
a recommendation.

---

## Tasks, in dependency order

Every task ends with what must be true, as a command or a reading, and names
the files it touches. A task touching no file says so.

### Task 0 — Measure before editing

**Files:** none.

**Do:**
- Free space on the temp volume (`df -h "$TMPDIR"`): at least 16 GiB, or
  `task check` whole and `python3 tools/check_mutation_test.py -q` cannot run
  to the end. `go clean -cache` if not.
- `python3 tools/check_mutation_test.py -q` passes.
- The position check over `tools/mutation-equivalents.txt` reports no suspect
  entry. It runs inside `tools/check-mutation.py` before gremlins; a scratch
  script that loads the module, calls `read_equivalents` on the file and
  `suspect_positions` on the result is how it was run today.
- `lefthook version` prints; the plugin path in `.lefthook.yml`'s `review-gate`
  line exists.
- `git status --short` shows the eleven entries the ticket lists plus the
  ticket itself and, once written, this plan.

**True when done:** all five hold, recorded in the report as the starting
state.

### Task 1 — Commit the mutation re-point alone

**Files:** `tools/mutation-equivalents.txt`.

**Do:** review the diff (one key moved by the branch's own `48e3fe2`, the
entry's own paragraph explains it), record the review, commit that one file
through the gate.

**True when done:** `git show --stat HEAD` names exactly that file; the
position check is still clean; Task 0's self-test still passes.

### Task 2 — The old scheme leaves the documents

**Files:** `docs/requirements.md`, `docs/verification-debt.md`,
`docs/superpowers/specs/2026-08-18-visibility-design.md`,
`docs/adr/011-identity-and-authorization.md`,
`docs/reports/2026-08-18-visibility.md`,
`docs/specifications/007-the-wire-contract.md`, `CLAUDE.md`.

**Do, per file:**

- `docs/requirements.md` — cut to D1's shape. The prose above the table says
  what the process's register says: ids come from `requirement-id` and nothing
  else, none reused, none renumbered once cited, a withdrawn row stays; the
  evidence column holds repo-relative test paths, comma-separated, or one of
  the two named answers. No rows. The sections "Why the shut-door row is open"
  and "Two numbers that are not requirements" go (the first moves to the debt
  file below). Do NOT name the chain gate here yet — it does not exist until
  Task 5, and a sentence naming it would be false at commit 2.
- `docs/verification-debt.md` — under `Open debt`, remove the passage from
  "Requirement IDs heal on reference" through "paid at that moment, by whoever
  is citing", including both commands and the next-number paragraph; keep "The
  oracle corpus contains no non-party viewpoint". In the two 2026-09-16
  entries, replace each bracketed id with the rule sentence it names (the
  roster rule, the roster-refusal rule, the one-code-path rule, the
  character-log rule), so each recipe still says which rule it is about. Add
  a new entry dated 2026-09-23 for the shut-door refusal, per D14: what the
  gap is, both labels, the recipe (remove the shut-door term from
  `JoinAdmits`'s guard, per the joining report), why
  `TestAClosedDoorSpendsNothing` sees only the effect and
  `TestARefusedJoinWritesNothingAtAll` sees only a birth, and what closes it
  (the armed database fault the wrong-secret and spent-budget tests use, with
  a shut door). Closing it is not this ticket's.
- The visibility ticket — per D15. Remove the eight bracketed tags, keeping the
  bold sentences. In the one-code-path block, replace the two inline
  references to the roster rule with "the roster rule in §5" and the one to
  the character-log rule with the rule's own words. Remove the four id
  paragraphs ("IDs are assigned on REFERENCE", "An ID is opaque", "A reference
  is not only a test", "Adding an ID to a sentence"). Keep the roster-refusal
  paragraph.
- ADR-011 — reword the closing paragraph, per D4: the rule that a refusal
  writes nothing is stated once per refusal; name the two armed-fault tests
  and the indistinguishability test by name; say the shut-door refusal has no
  test that can see its write and that `docs/verification-debt.md` records
  what the two tests aiming at it can and cannot see. No ids, no claim about
  rows in the register.
- The visibility report — drop the two parenthetical ids from the two bold
  headings ("Exactly one code path introduces an actor", "Fail closed on an
  unrecognised payload"). Nothing else changes.
- Spec 007 — the `Requirements` section, per D13.
- `CLAUDE.md` — the register bullet names the tag `VTT`, the header, the
  dispenser (`requirement-id`, on the path while the package is installed, by
  its path under the plugin cache otherwise), the three evidence values, and
  that the rows written before 2026-09-23 under a per-subject scheme were
  replaced, each arc's own ticket re-deriving its rules. Keep the sentence
  "Nothing checks the chain here today" — it is true until Task 5, which
  replaces it. Remove the "TWO ID SCHEMES" passage and the "holds no such
  commands" passage. In the gate paragraph, add `review-gate` to the list of
  what the pre-commit hook runs. Do NOT touch the adjudications bullet (D17).

**True when done**, each by command:
- Done item 2 with D3's exclusion prints nothing.
- `grep -E '^\| VTT' docs/requirements.md` prints nothing (no rows yet); the
  dispenser on a copy allocates `VTT-001`.
- Done item 5: `grep -c '\[VTT-'` on the visibility ticket prints 0; no
  paragraph on id assignment; each of the eight sentences once.
- Done item 6: no register passage, no hardcoded series in a command, and one
  entry labelled `test asserts nothing` naming both tests.
- Done item 7: both counts 0; the register bullet names tag, header,
  dispenser.
- A reading: every sentence written is true at commit 2 AND at commit 5.

### Task 3 — Commit the documents, three commits

**Files:** as D2's commits 2, 3 and 4. Commit 2 adds `.lefthook.yml`,
`.claude/settings.json` (the local file is ignored by the global gitignore,
measured), `CLAUDE.md`, `docs/requirements.md`, `docs/verification-debt.md`,
the visibility ticket, this ticket and this plan. Commit 3 adds the eight
reports and ADR-011. Commit 4 adds `docs/specifications/`.

**Do:** one reading review per commit, recorded; each commit through the gate.
Item 9 is re-observed in a scratch repository after commit 2 lands (the
scratch procedure is in the ticket; lefthook 2.1.5 takes `--command`, not
`--commands`).

**True when done:** `git status --short` prints only `?? docs/ADOPTING-THE-PROCESS.md`
and `?? docs/reports/DERIVED-contract-experiment.md`; `git log --oneline
main..HEAD` shows the four new commits above the three the branch already
carries; item 9 holds.

### Task 4 — Sort the rules and allocate their ids (after sign-off only)

**Files:** `docs/requirements.md` (rows appended by the dispenser; evidence
cells filled by hand afterwards).

**Do:** per the `requirements` skill, sort the ticket's four rules; report
what was accepted and rejected and why. For each accepted rule, from the repo
root, `requirement-id "<the sentence>"`. Then set each row's evidence per the
sort's outcome (D12): `tools/check_requirements_chain_test.py` for a rule that
file will hold, `**READING — Phase 4b**` for the reading-held clause,
`**OPEN — no test yet**` left as the dispenser wrote it for the commit-gate
rule. Afterwards confirm `docs/requirements.md.lock` does not exist.

**True when done:** every row's id matches `^VTT-[0-9]{3}$`; the dispenser on a
copy allocates the number after the highest row (Done item 3); no evidence
cell is blank.

### Task 5 — The chain gate, test first

**Files:** new `tools/check_requirements_chain_test.py`, new
`tools/check-requirements-chain.py`, `Taskfile.yml`.

**Do, in this order:**
1. Write the self-test in the shape of `tools/check_doc_owner_test.py`: a
   `tree(**files)` helper building a fixture repo in a temp dir with
   `project: TT`, and one case per direction and per refusal — a test citing
   `TT-999` with no row; a specification naming `TT-999` with no row; a row
   whose evidence names a missing file; a row whose evidence names a file
   that does not carry the id; `OPEN` and `READING` pass as answers; a blank
   evidence cell is refused; a row id of another shape (a per-subject one) is
   refused; a duplicated row id is refused; markup around a row id is read
   through; a `TT` fixture is never read as a `VTT` claim; no register is
   exit 2; no test files is exit 2; an empty table passes; a clean fixture
   passes and prints the completion line; the real tree is clean, asserting
   the completion line with at least one row and at least one test file
   (nothing scanned is nothing proven). Each case carries the id of the rule
   it holds, per D8. Run it: RED, because the module does not exist.
2. Write the checker in the shape of `tools/check-doc-owner.py`: read the tag
   and rows from the register; collect test files per D5; collect ids from
   tests and specifications; apply D6 and D7; one finding per line on stderr
   prefixed `check:requirements-chain:`, naming the file and the id or the
   row; a completion line on stdout stating rows, test files and
   specifications scanned (the `check:new-prose` convention: proof a checker
   ran to the end, because an exit code is not). Exit 1 on findings, 2 on
   nothing scanned, 0 otherwise. Run the self-test: GREEN.
3. `Taskfile.yml`: a `check:requirements-chain` task whose cmds are the
   self-test with `-q` then the checker over `.`, a desc that states what it
   refuses and nothing about how it was arrived at; add it to `check:`
   directly after `check:doc-owner`; add its name to `check:fast`'s enumerated
   skip list.
4. Fault-inject the ticket's three item-4 cases on the real tree — a `VTT-999`
   comment in one test file, one in spec 007, one row's evidence pointed at a
   missing file — each as a temporary edit: `task check:requirements-chain`
   exits non-zero naming the offender each time. Revert each; run green.
5. Replace `CLAUDE.md`'s "Nothing checks the chain here today" with the gate's
   name, and add "the requirements chain" to the gate paragraph's list of what
   runs only under `task check`.

**True when done:** step 4's three refusals and the green run afterwards, all
recorded in the report; `python3 tools/check_requirements_chain_test.py -q`
passes; `task check:requirements-chain` prints the completion line; Done item
4 holds except its last clause ("exits zero on the committed tree"), which
Task 6 closes.

### Task 6 — Commit the gate

**Files:** those of Tasks 4 and 5 (`docs/requirements.md`, the two new tools,
`Taskfile.yml`, `CLAUDE.md`).

**Do:** Phase 4a's independent QA on the checker (its usage line and self-test
are what the QA agent gets; there is no `go doc` surface), adjudications per
D17's answer; Phase 4b's reading review; record; one commit, test and
implementation together, carrying the line Phase 3 requires (what was broken
and which check spoke — Task 5 step 4).

**True when done:** `task check:requirements-chain` is green on `HEAD`; the
commit is the fifth above `main`.

### Task 7 — The specification, the deletions, the whole gate

**Files:** new `docs/specifications/008-requirement-ids-come-from-the-dispenser.md`
(D10); delete `docs/ADOPTING-THE-PROCESS.md` and
`docs/reports/DERIVED-contract-experiment.md`.

**Do:** per the `specification` skill, five headings. `Status`: accepted,
implemented by `docs/requirements.md`, `tools/check-requirements-chain.py` and
`Taskfile.yml`'s `check:requirements-chain`, pinned by
`tools/check_requirements_chain_test.py`. `Principles served`: this project has
no blueprint (`CLAUDE.md`, "Where the blueprint is"); the principle this record
would serve — a citation resolves to exactly one thing — is missing, not
absent, and the blueprint is its own ticket. `How it works`: the tag and where
it is declared; the dispenser and that nothing else writes an id; the row
shape and the three evidence values; how a test cites (the bare id in a
comment on the line above the check it holds); what the gate reads (D5), what
it refuses (D6, D7); that a fixture uses another tag. `Consequences`: never
reused, never renumbered, a withdrawn row stays; a specification names only
allocated ids; the rows written before 2026-09-23 were replaced and each arc's
ticket re-derives its own. `Requirements`: the ids Task 4 allocated, copied
from the register, none invented. Then `catches.md` beside the skill, two
passes — in particular no path outside the project (the dispenser is named by
the package's name, not by its cache path; `CLAUDE.md` is the record that may
carry the path), no measurement, no "only/never/every" without the search
named, no old text beside new. Then delete the two files. Then `task check`
WHOLE, locally, in the foreground or launched the way the memory on background
runs prescribes; Task 0's disk precondition applies.

**True when done:** Done items 8 and 10 hold; `task check:requirements-chain`
is green with the specification in place (it names only rows that exist);
`task check` whole is green.

### Task 8 — Commit, report, push

**Files:** new `docs/reports/2026-09-23-adopting-the-process.md`.

**Do:** review and record; commit the specification and the deletions (the
sixth commit). Write the report per the `implementation-report` skill against
the ticket's ten items, each with the observation that shows it — including
Task 5 step 4's refusals, the item-9 scratch observation with the exact
command, Task 0's starting state, and what the sort rejected. The report
names the old id forms by shape only (D3). Review, record, commit the report.
Fetch and check the branch against `origin/main` per Phase 5; then
`git push -u origin chore/adopt-the-process`.

**True when done:** Done item 1 holds — `git status --short` prints nothing and
`git rev-parse --verify origin/chore/adopt-the-process` succeeds; Done item 2
with D3's exclusion prints nothing over the final tree, the report included;
item 9 re-observed once more after the push.

---

## Gaps that travel with this plan

From the verification, each one either answered by a decision above or left
for the user:

1. Done item 2's command matches the ticket's own problem statement (four
   lines). D3 reads it with the ticket, plan and report excluded. The ticket's
   writer may add the exclusion; this plan did not edit the ticket.
2. Done item 2's "Today" count for the bare form is six lines in two files by
   measurement, not seven. The observation itself is unaffected.
3. The ticket says two rows name tests in a package that does not exist; three
   rows name that package (two name tests, one names a non-test file). The
   rows go either way.
4. The id rule's "written by the dispenser and by nothing else" clause has no
   command that can fail; D12 sends it to the sort as a reading.
5. The commit-gate rule has an observation (item 9) and no in-tree test; D12
   leaves its row `OPEN`.
6. The ticket edits an ADR and a report, which two records say are not
   rewritten, without saying so; D4 resolves it (never committed, so not yet
   frozen; and the second permitted reason applies).
7. "The branch has no remote" in the ticket means no upstream ref; the
   repository has `origin`. Item 1's observation is exact and is what this
   plan uses.
8. Three tests named in `docs/verification-debt.md`'s 2026-09-16 and 2026-09-17
   entries are not in the tree (`TestARefusedLookStillFillsTheRoster`,
   `TestLookRefusesEyesToAnythingButAPartyMember`,
   `TestQAActorAddedIsWithheldEvenForAPartyMember`): they left with the
   per-character-logs rollback. Outside this ticket; the entries are records
   of their period. Noted so the report can say it.

## Questions for sign-off

One line each; the plan proceeds on the defaults named.

- Split the ticket at the document/gate seam (Tasks 1–3 as one ticket, Tasks
  4–8 as another)? Default: no — the seam is already the commit boundary, the
  second half's rows are what make the first half's register non-empty, and
  one report covers both.
- Should the chain gate also run in the pre-commit hook (D9)? Default: `check:`
  only.
- Specification number `008` (D10)? Default: yes.
- A second specification for the commit gate's wiring, one decision one file
  (D11)? Default: no, `CLAUDE.md`'s gate paragraph is the record.
- Where do Phase 4a's QA adjudications live (D17)? Recommendation: in the
  arc's implementation report, under their own heading, since the report is
  where the road there is kept and nothing else is read for it; and
  `CLAUDE.md`'s adjudications bullet is updated in Task 6 to say so.
- Should the ticket's item 2 gain D3's exclusion, by its writer? Default: the
  plan's reading stands either way.
