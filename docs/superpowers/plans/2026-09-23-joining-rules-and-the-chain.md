# Joining rules and the chain — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-23-joining-rules-and-the-chain-design.md`
**Verified:** 2026-09-23, by `verify-ticket`, an agent that did not write the
ticket. Verdict: **Passes with gaps.** The gaps are listed at the end and
travel with this plan. This plan does not edit the ticket.

**Goal, in the ticket's words:** the joining-a-table arc's rules are in the
register, cited by their tests, and a gate holds the chain.

**Scope, from Patrik's ruling of 2026-09-23:** this is the FIRST HALF of the
arc. The identity/joining specification, the sort of `identity.go`'s comments,
the removal of `JoinAllows` and the shut-door test are the second ticket's and
are not planned here.

**Design reused:** the chain gate's design is the one verified in
`docs/superpowers/plans/2026-09-23-adopting-the-process.md`, decisions D5 to
D9 and D12 and its Task 5. Where this plan departs from it, the departure is a
numbered decision below with the measurement that forced it.

**MapTool (CLAUDE.md rule 9):** MapTool keeps no requirements register and no
citation chain between rules, tests and records; the gate is process tooling,
not tabletop geometry, so there is nothing to borrow and nothing to reject.
The joining rules themselves were designed by the arc that built them, and
that arc's answer stands: MapTool's join model (every client receives the
whole campaign; a server password, no door, no per-person credential) was
checked and rejected as a security model (`internal/gateway/seat.go`,
CLAUDE.md rule 9's own text). This ticket registers rules that exist; it does
not reopen their design.

---

## Constraints that bind every task

The project's standing rules, `CLAUDE.md`, by number:

- **Rule 2.** The chain gate is ADDED. No existing gate, threshold, exemption
  or scope is loosened to get a green run.
- **Rule 8.** Every citation this work writes — register rows, test comments,
  SPEC-008, the report, the self-test — names a test, a file, a symbol or a
  dated decision. Never a line number. The checker's own stderr may print a
  path and a line for a finding, as `check:doc-owner` does; that is tool
  output, not prose.
- **Rule 9.** Answered above.
- Rules 3, 4 and 5 are not engaged: no contract, fold or platform code
  changes. `check:invariants` scans `internal/` and `cmd/` only (its
  `semgrep scan` line in `Taskfile.yml`), so a Python tool under `tools/` is
  outside it, and the register's rows use platform vocabulary (spectator, DM,
  player), which rule 5 permits.

The process's rules, from the installed `dev-cycle` package (0.4.0):

- Requirement ids come from `requirement-id` and from nothing else, allocated
  AFTER Phase 2 sign-off (`requirements` skill; Patrik's ruling). A dry run
  goes on a copy in the scratchpad, never on `docs/requirements.md`. Measured
  today on a copy: the dispenser allocates `VTT-001` then `VTT-002`, writes
  `| <id> | <sentence> | **OPEN — no test yet** |`, and leaves no `.lock`.
- Tests before code, then the deliberate break: ONE break per check relied
  on, an act and not an artifact, recorded as a line in the commit.
- Phase 4a's QA runs on the gate, before the reading review, and receives the
  requirement and the specification, never the diff, the source or the
  existing tests. It is skipped for the rows and citations (prose) and for
  the report.
- One reading review per commit, recorded with the package's
  `review-record.sh --summary`. The recorder fingerprints `git diff HEAD` over
  the WHOLE working tree plus every untracked file, and the commit gate
  recomputes the same, so a commit's working tree must be exactly its diff:
  nothing else modified, nothing untracked.
- The implementation report follows the last code commit, per the
  `implementation-report` skill, against the ticket's six items in the
  ticket's numbering.

Patrik's rulings, taken as given:

- Per-arc scope, one ticket at a time; this is the first half.
- Single source of truth: point at the source rather than restate it.

Records that constrain how documents are edited:

- `docs/adr/011-identity-and-authorization.md` is frozen and is not touched.
- `docs/reports/2026-08-09-joining-a-table.md` is a period record and is not
  revised for this work.

Environment, measured 2026-09-23:

- `python3` is Python 3.9.6. The checker uses no 3.10 syntax.
- `task check` whole takes over an hour; a background run must be launched
  with `start_new_session` (memory: background gate runs die at 38 minutes),
  and the mutation gate refuses below 16 GiB free on the temp volume.
- Mutation adjudication keys are source-file coordinates
  (`tools/mutation-equivalents.txt` holds none in a `_test.go` file), so a
  comment line added to a test file moves no key.

---

## Decisions this plan makes

**D1. What the gate scans** (previous plan D5, reused). Test files are
`*_test.go`, `*.test.ts` and `*_test.py` under `internal/`, `cmd/`, `client/`,
`tools/` and `contract/`, skipping `node_modules`, `gen`, `contract-spike`,
`.git`, `.superpowers`, `.stryker-tmp` and `.tscov`. Specifications are
`docs/specifications/*.md`, read whole. Rows come from the table under the
`| Id | Requirement |` header in `docs/requirements.md`; the tag comes from
its `project:` line. Measured today: 183 test files across the five roots
(116 Go under `internal/`, 18 under `cmd/`, 30 TS under `client/`, 12 Python
and 4 Go under `tools/`, 3 under `contract/`) and no `VTT-` followed by three
digits in any of them.

**D2. What the gate refuses** (previous plan D6 and D7, reused, plus one).
Exit 1, one finding per line on stderr prefixed `check:requirements-chain:`,
naming the file and the id or the row:

- a test file cites an id no row defines;
- a specification names an id no row defines;
- an evidence entry names a file that does not exist;
- an evidence entry names a file that does not carry the row's id;
- NEW — an evidence entry names a check that is not in that file: `func
  <name>(` in a `.go` file, `def <name>(` in a `.py` file, the name as a
  quoted string in a `.ts` file. Forced by Done item 6, which claims every
  named test exists as `func <name>(`; without this the claim is checked once,
  in the report, and rots afterwards;
- a row id that is not `<tag>-NNN` after backticks, bold and underscores are
  stripped (the dispenser strips the same set when it counts);
- two rows with one id;
- a blank evidence cell;
- a `READING` cell with nothing after the dash (a reading nobody can name is
  one of the other two outcomes, per the `requirements` skill).

Passes by name: a cell that is exactly `**OPEN — no test yet**`, or opens
`**READING — ` with a review named. Anything else is read as evidence entries.
Exit 2 when the register is missing, declares no tag, has no header, or when
zero test files are found; zero rows is silent (an empty register is a step,
not a defect). A completion line on stdout states rows, test files and
specifications scanned — proof the checker ran to the end, because an exit
code is not (`check:new-prose`'s convention).

**D3. The evidence entry is `<repo-relative path>#<check name>`, and entries
are comma-separated.** Forced by: Done item 6 names tests, not only files; the
`requirements` skill asks for "the file and the check's own name"; rule 8
makes the name the durable half. `#` is the separator because no tracked path
contains one (measured: `git ls-files | grep -c '#'` prints 0) and no
tracked path contains a comma. `OPEN` and `READING` cells keep the shape the
dispenser and the skill write. A sentence in the cell parses as a path with
spaces, fails as a missing file, and that is the catch.

**D4. A citation is the bare id on the LAST line of the comment block directly
above the check, and a check holding several rules puts their ids on that one
line, space-separated.** Forced by measurement, 2026-09-23, running
`tools/check-comment-wrap.py` over a scratch file: an id line as the last line
of a block is not flagged; an id line followed by more comment is flagged
`SHORT`; two ids on separate lines flag the first; two ids on one line are not
flagged. `check:new-prose` promotes that finding to a failing gate for lines a
change adds to `.go` files. `check:doc-owner` skips test files and reads only
a block's first word, so an id there changes nothing it sees. For the
Python self-test the same shape, with `#`.

**D5. SPEC-008's example id becomes the shape.** Its `How it works` says the
id is "`VTT-001` upward"; the gate reads specifications whole, so that is a
citation, and on the empty register of the first commit it is a dangling one:
the gate would red on the commit that introduces it. The sentence says the
shape (`VTT-NNN`, numbered from one) and no instance. Reading only a
specification's `Requirements` section was rejected: a dangling id in a
specification's prose is exactly what the specification scan is for.

**D6. Fixtures carry `project: TT` and ids `TT-NNN`** (previous plan D8,
reused). The gate scans `tools/*_test.py`, so a literal `VTT-999` in a fixture
would be a citation. The ticket's `VTT-999` cases are shown on the real tree by
injection (Task 1) and recorded in the commit line and the report. QA's test
file is under the same rule, and SPEC-008 says so, which is the document QA
reads.

**D7. Wiring** (previous plan D9, reused). A `check:requirements-chain` task
in `Taskfile.yml`: its `cmds` run the self-test with `-q`, QA's test with
`-q`, then the checker over `.`; a `desc` that says what it refuses and
nothing about how it was arrived at. It goes into `check:` directly after
`check:doc-owner`, and its name goes into `check:fast`'s enumerated skip list
(that desc says the names are what rots, and they have rotted twice).
`.lefthook.yml` is unchanged: the ticket asks for a `task check` step, and a
hook seat is a second change to `CLAUDE.md`'s gate paragraph. A preference,
named as one; a question at the end. CI runs `task check` whole and picks the
step up with no change of its own.

**D8. QA's tests are wired into the step, not only committed.** Forced by the
`check:new-prose` desc's own record: two checkers "whose tests nothing ran"
rotted, and the reason to wire them is that an unrun test rots.

**D9. Two code commits, then the report.** Commit 1: the gate, its self-test,
QA's test, `Taskfile.yml`, `CLAUDE.md`, `SPEC-008`. Commit 2: the register
rows with their evidence, the citations in the five test files and the
self-test, and SPEC-008's `Requirements` section. Forced by: every sentence
must be true at every commit, and two flip with the gate (`CLAUDE.md`'s
"Nothing checks the chain here today"; SPEC-008's Status) while one flips with
the rows (SPEC-008's `Requirements`); the recorder's fingerprint covers the
whole working tree, so the rows cannot sit in the tree while commit 1 is
recorded; and the process reviews one task's diff at a time. The ticket's
"rows and citations land together" holds: both are commit 2.

**D10. The dispenser writes at the start of Task 3, after commit 1, not at the
end of Phase 2.** Forced by D9 and the recorder. The SORT runs in Phase 2,
after sign-off, and is reported there with its rejections; only the writes to
`docs/requirements.md` move, and they are still after sign-off.

**D11. What the sort is handed, from the verification's reading of every
named test.** The sort is the `requirements` skill's, run in Phase 2; this
plan does not run it. What it will face:

- `TestCheckingTheDoorMintsNothing` exercises `JoinAllows`, which has no
  production caller and which the second ticket removes; the live-path twin
  `TestARefusedJoinWritesNothingAtAll` holds the same property through
  `handleJoin`. A row citing the former goes red the day the second ticket
  lands.
- `TestEveryClientCommandConverts` SKIPS `promote_participant` by name in its
  `notConverted` map; it holds nothing about whether a promotion appends an
  event. No test reads the log head across a promotion (`f.head(t)` is used by
  `TestRevokingRemovesSomebodyWhoIsStillConnected` only). The rule that a role
  never enters the log is held by a search of `internal/engine` (measured:
  no `Role` in any file there, tests included) and by nothing that runs under
  `task check`; its outcome is OPEN or a split.
- `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody` in
  `internal/gateway/server_test.go` holds "only an invalid credential ends a
  connection": it renames the participants table under a live connection,
  asserts the command is refused, the connection stays, and a narration
  appended meanwhile still arrives. The ticket's first "could not be
  established" is settled.
- The shut-door rule is two: the refusal (held by `TestAClosedDoorMintsNobody`,
  `TestJoinIsClosedOnAFreshCampaign`, `TestAClosedDoorSpendsNothing`) and its
  inertness (held by nothing that can see the write; the debt entry dated
  2026-09-23; OPEN).
- The budget-default rule's "non-positive" is exercised at zero only
  (`TestADoorOpenedWithNoStatedBudgetStillAdmits`; no test passes a negative
  budget anywhere under `internal/` or `cmd/`).
- "The joiner chooses no role" is held by `joinRequest`'s shape in
  `internal/gateway/join.go` (two fields, `secret` and `displayName`), a
  reading; the named test asserts the minted role only.
- One §5 rule the ticket does not list: opening the door and rotating the
  link are DM-and-agent only, held by `authzCases` under
  `TestAuthorizeTableAllCommandsAllRoles` in `internal/gateway/authz_test.go`.

**D12. Phase 4a's adjudications go in the arc's implementation report, under
their own heading, one line per finding in the shape `qa-prompt.md` gives,
and `CLAUDE.md`'s adjudications bullet says so from commit 1.** Forced by that
bullet's own sentence — the first QA run here has to give them a home — and
this is the first QA run. The report is where the road there is kept. A
question at the end.

**D13. `task check` whole runs once, before commit 2, over the tree that
carries both commits' changes; commit 1 runs the steps its files can reach.**
Forced by measurement: commit 1 adds no `.go` or `.ts` source, so no tier of
the suite, no coverage floor and no mutation key is affected by it; the steps
it can reach are `check:requirements-chain`, `check:fast`, `check:new-prose`,
`check:doc-owner` and `check:secrets:staged`, each seconds. The whole gate is
over an hour. A question at the end.

**D14. SPEC-008's Status changes in the commit that lands the gate, not after
the report.** Phase 5 says the record is updated last; the `specification`
skill's `catches.md` says a Status that lags the tree is the failure its
section exists for, and Done item 5 makes the Status a deliverable. The Phase
5 step then finds nothing left to move, or moves it.

**D15. The arc's rows carry no specification yet.** The identity/joining
specification is the second ticket's. The gate checks that a specification's
ids resolve to rows, not that every row is named by a specification, so the
tree is green with rows that no record names. SPEC-008's `Requirements`
names the three chain rows in commit 2.

---

## Tasks, in dependency order

Every task ends with what must be true, by command or by a reading, and names
the files it touches.

### Task 0 — Measure before editing

**Files:** none.

**Do:**
- `python3 --version` prints 3.9.x; `df -h "$TMPDIR"` shows at least 16 GiB
  free, or `go clean -cache` first.
- `python3 tools/check_doc_owner_test.py -q` passes (the convention the new
  tool copies).
- `git status --short` prints the ticket and this plan and nothing else;
  `git log --oneline -1` prints `ad68765`.
- `lefthook version` prints; the path in `.lefthook.yml`'s `review-gate` line
  exists.
- On a scratchpad copy of `docs/requirements.md`, `requirement-id --register
  <copy> "<any sentence>"` allocates `VTT-001` and leaves no `.lock`.

**True when done:** all five hold, recorded in the report as the starting
state.

### Task 1 — The gate, test first

**Files:** new `tools/check_requirements_chain_test.py`, new
`tools/check-requirements-chain.py`, `Taskfile.yml`, `CLAUDE.md`,
`docs/specifications/008-requirement-ids-come-from-the-dispenser.md`.

**Do, in this order:**

1. Write the self-test in the shape of `tools/check_doc_owner_test.py`: the
   module loaded by path, a `tree(**files)` helper writing a fixture
   repository to a temp dir, `project: TT`. One case per refusal in D2, in
   each direction, and the passes: a test citing `TT-999` with no row; a
   specification naming `TT-999` with no row; an evidence entry naming a
   missing file; one naming a file that does not carry the id; one naming a
   check that is not in the file, for `.go`, `.py` and `.ts`; `OPEN` and a
   named `READING` pass; a `READING` with nothing after the dash is refused;
   a blank cell is refused; a row id of another shape (a per-subject one) is
   refused; a duplicated id is refused; markup around a row id is read
   through; two ids on one comment line both resolve; a `TT` fixture is not
   read as a `VTT` claim; no register, no tag and no header each exit 2; no
   test files exits 2; an empty table passes; a clean fixture passes and
   prints the completion line; and the real tree is clean, asserting the
   completion line with at least one hundred test files and two
   specifications (nothing scanned is nothing proven — the count is a floor,
   not a figure). Run it: RED, because the module does not exist.
2. Write the checker in the shape of `tools/check-doc-owner.py`: a module
   docstring that says what it catches and carries the `Run:` line and the
   exit codes (QA reads that docstring and nothing else of the source); D1's
   scan; D2's refusals; D3's entry parser. Python 3.9. Run the self-test:
   GREEN.
3. `Taskfile.yml` per D7 (QA's line is added in Task 2 once the file exists).
4. `CLAUDE.md`: the register bullet's last sentence names
   `check:requirements-chain` as what checks the chain; the gate paragraph's
   list of what runs only under `task check` gains "the requirements chain";
   the adjudications bullet says where Phase 4a's adjudications go (D12).
   Nothing else in the file moves.
5. SPEC-008: Status becomes `Accepted. Implemented by ...` naming the
   checker, the Taskfile step and the register, `pinned by` the self-test and
   QA's test; `How it works` gains a paragraph on how the chain is checked
   (D1, D2, in the present tense, without the road there) and the evidence
   entry shape (D3); the example id becomes the shape (D5); the
   `Consequences` sentence about "until the chain gate exists" goes, because
   the old text does not stay beside the new. Two passes against
   `catches.md`; in particular no measurement, no path outside the project,
   no "only/never/every" without the search named.
6. BREAK IT, one act per refusal relied on, each on the real tree, each
   reverted before the next, `task check:requirements-chain` run after each
   and after the last revert:
   - a `// VTT-999` line in `internal/identity/identity_test.go`;
   - a `VTT-999` in `docs/specifications/007-the-wire-contract.md`;
   - a hand-typed row `| VTT-999 | x | internal/nowhere_test.go#TestNope |`
     in `docs/requirements.md` (typed, never dispensed: a dispensed row is a
     number spent), then the same row pointed at a real file without the id,
     then at a real file with a wrong check name, then with a blank cell,
     then a `| VTT-9 |` shape, then two rows `VTT-999`;
   Assert each edit landed before trusting the red; each run exits non-zero
   naming the offender; the final run on the restored tree prints the
   completion line with zero rows. Then the other direction: an ordinary
   correct citation of a real row cannot be tried yet (no rows), so the
   self-test's clean-fixture case stands in and the report says so.
7. `task check:fast`, `task check:new-prose`, `task check:doc-owner`,
   `task check:secrets:staged` (after staging) — the steps commit 1 can reach
   (D13).

**True when done:** `python3 tools/check_requirements_chain_test.py -q`
passes; `task check:requirements-chain` prints the completion line with 0
rows, at least 183 test files and 2 specifications; every injection in step 6
redded and is written down for the commit line; `grep -c 'Nothing checks the
chain here today' CLAUDE.md` prints 0; `grep -nE '\bVTT-[0-9]{3}\b'
docs/specifications/008-*.md` prints nothing; SPEC-008's Status names the
step and the self-test (Done item 5 except the QA test, added in Task 2).

### Task 2 — QA on the gate, review, commit 1

**Files:** new `tools/check_requirements_chain_qa_test.py` (QA writes it),
`Taskfile.yml` (its line).

**Do:**
1. Extract the checker's docstring to a scratchpad file (the way Task 0's
   convention check loads a module by path). Write the requirement file: the
   ticket's Done item 3 and its three chain rules, verbatim.
2. Dispatch ONE QA agent per `qa-prompt.md`: the requirement file, SPEC-008
   whole, the ticket whole, the docstring; the test file
   `tools/check_requirements_chain_qa_test.py`; the command `python3
   tools/check_requirements_chain_qa_test.py`. It does not receive the diff,
   the checker's source or the self-test. SPEC-008 tells it a fixture carries
   another tag.
3. Adjudicate each failure (behaviour defect or specification ambiguity),
   fix, re-run QA's tests; write the adjudications down for the report (D12).
   A defect that reached QA past the self-test gets a recipe in
   `docs/verification-debt.md`, which is then part of this commit.
4. Add QA's test to the Taskfile step (D8). `task check:requirements-chain`
   green.
5. Phase 4b: one reviewer at high effort on the diff; verify every sentence
   in `CLAUDE.md`, SPEC-008 and the Taskfile desc BY COMMAND; present
   findings; apply approved fixes; `review-record.sh --summary`.
6. Stage everything; `git status --short` shows nothing unstaged and nothing
   untracked except this plan and the ticket, which are staged too (they are
   part of this commit, as the previous arc's were); commit with the line
   Phase 3 requires (what was broken, which check spoke).

**True when done:** `git show --stat HEAD` names exactly the files of Tasks 1
and 2 plus the ticket and this plan; `task check:requirements-chain` is green
on `HEAD`; the working tree is clean.

### Task 3 — The rows, the evidence, the citations

**Files:** `docs/requirements.md`; `internal/identity/identity_test.go`,
`internal/identity/fault_internal_test.go`, `internal/gateway/join_test.go`,
`internal/gateway/authz_test.go`, `internal/gateway/server_test.go`;
`tools/check_requirements_chain_test.py`;
`docs/specifications/008-requirement-ids-come-from-the-dispenser.md`.

**Do:**
1. For each rule the Phase 2 sort accepted, in the sort's order, from the
   repository root: `requirement-id "<the sentence>"`. Once each, on the real
   register, never twice for one rule. Afterwards `ls docs/requirements.md.lock`
   fails.
2. Set each row's evidence per the sort's outcome and D3: `path#Check`
   entries for a held rule; `**OPEN — no test yet**` left as written for a
   gap (the shut-door inertness; the role-never-in-the-log rule unless split);
   `**READING — <review>**` where the sort ruled so (the "written by the
   dispenser and by nothing else" half of the id rule, per the previous plan's
   D12).
3. Add each citation per D4: the id, or the ids, on the last comment line
   above `func TestX(` in the file the row names; in the self-test, above the
   case that holds each chain rule.
4. SPEC-008's `Requirements` lists the three chain ids, copied from the
   register.
5. `task check:requirements-chain` green with rows. BREAK IT: change one
   citation to an id one higher than the highest row — red, naming the file;
   revert. Rename one cited test in its evidence entry — red, naming the row;
   revert. Green.
6. `go test ./internal/identity/... ./internal/gateway/...` green (comments
   only, but run). `task check:new-prose` green on the added lines (D4).
7. `task check` WHOLE, locally, launched per the memory on background runs,
   after Task 0's disk precondition. Green.

**True when done:** Done item 1 by its command prints the number of accepted
rules and no cell is blank; Done item 2's grep over `internal` lists exactly
the five test files, and `grep -rlE '\bVTT-[0-9]{3}\b' tools` lists exactly
the self-test (gap 1 below); every row's every entry resolves (the gate
says so, and Done item 6's `func <name>(` check by command agrees); the two
tests item 6 names pass unchanged; `task check` whole is green.

### Task 4 — Review, commit 2

**Files:** those of Task 3.

**Do:** Phase 4a is skipped (prose and comments; no behaviour to derive — say
so in the report). Phase 4b: one reviewer; each row's sentence checked
against the test it names BY READING THE TEST, not the ticket; each `OPEN`
and `READING` justified in one line. `review-record.sh --summary`; commit
with the Phase 3 line (step 5's two breaks).

**True when done:** the commit is the second above `ad68765`; `task
check:requirements-chain` green on `HEAD`; the working tree is clean.

### Task 5 — The report, then Phase 5

**Files:** new `docs/reports/2026-09-23-joining-rules-and-the-chain.md`.

**Do:** per the `implementation-report` skill against the ticket's six
`Done looks like` items in the ticket's numbering, each `[x]` with the
observation; `What the rules became` from the sort (each rule's id or the one
line refusing it, the readings named); the QA adjudications under their own
heading (D12); the injections of Task 1 step 6 and Task 3 step 5; Task 0's
starting state; deviations (D5's SPEC-008 edit, D10's timing, anything the
sort changed); what could not be established. Names the last code commit.
Reading review; record; own commit. Then fetch, check the branch against
`origin/main`, merge if behind, push `chore/adopt-the-process`.

**True when done:** three commits above `ad68765`; `git status --short`
prints nothing; the report answers items 1 to 6.

---

## Gaps that travel with this plan

1. Done item 2's grep is over `internal` only, while the three chain rows name
   `tools/check_requirements_chain_test.py`, which then carries ids too. The
   plan reads the item per root (the five test files under `internal/`, the
   self-test under `tools/`). Whether the item gains `tools` is its writer's
   call.
2. Done item 3's "exits zero on the committed tree" holds only with D5's
   edit to SPEC-008's example id, which the ticket's list of edits to SPEC-008
   (Status only) does not name. The report records it as a deviation.
3. `TestCheckingTheDoorMintsNothing`, named by the ticket, exercises
   `JoinAllows`, which the second ticket removes (D11).
4. `TestEveryClientCommandConverts`, named by the ticket, skips
   `promote_participant` and holds nothing about it (D11).
5. The ticket's first "could not be established" is settled:
   `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody` holds the
   connection rule (D11).
6. The budget-default rule is exercised at zero, not at a negative number.
7. One §5 rule is absent from the ticket's list (door and rotation are
   DM-and-agent only); the sort meets it in the pre-ticket and ADR-011's
   "How this is enforced" does not name it either. Its test is named in D11.
8. "The joiner chooses no role" is a reading of `joinRequest`, not the named
   test's assertion.
9. Whether Phase 5's "record last" applies when a Status change is itself a
   Done item (D14).
10. The previous plan's open questions on the hook seat (D7) and the
    adjudications' home (D12) are still open; this plan answers both and asks.

## Questions for sign-off

One line each; the plan proceeds on the defaults named.

- Is this half-ticket right-sized (nine existing files, two new, four areas)?
  Recommendation: yes, as one ticket in two code commits, and do not split
  again — rows without the gate are held by nothing, and the gate without rows
  holds nothing; the second-half ticket is the next split.
- Evidence entries as `path#Check` with the check's name verified by the gate
  (D2's added refusal, D3)? Default: yes.
- QA adjudications in the report, and `CLAUDE.md`'s bullet updated to say so
  (D12)? Default: yes.
- `task check` whole once, before commit 2 (D13)? Default: yes; the
  alternative is a second hour-long run after commit 1 over a tree no tier
  reads differently.
- SPEC-008's Status in the gate's commit rather than after the report (D14)?
  Default: yes.
- A pre-commit seat for `check:requirements-chain` (D7)? Default: no.
