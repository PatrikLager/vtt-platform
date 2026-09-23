# Adopting the process: the prerequisite

**Ticket:** `docs/superpowers/specs/2026-09-23-adopting-the-process-design.md`.
**Plan:** `docs/superpowers/plans/2026-09-23-adopting-the-process.md`, verified by
`verify-ticket` for a wider scope and narrowed to this ticket by its top block.
**Last commit that changes the tree:** `9842452`. Every path and name below is
read against that tree; the base is `1ce9731`, the branch's tip before this
work.

## The period, in commits

    git log --oneline 1ce9731..9842452

    9842452 SPEC-008: requirement ids come from the process's dispenser
    a36a90d SPEC-007: the wire contract, and contract/README.md points at it
    cc88923 The 2026-09-19 documents: eight arc reports and ADR-011, as the accounts of their periods
    257508f The dev-cycle process arrives: gate chained, register in the dispenser's shape, old id scheme out
    2d50199 Re-point one adjudication key that 48e3fe2 moved

`git diff --stat 1ce9731..9842452`: 21 files, 3635 insertions, 82 deletions.
Twenty of the files are documents and configuration; the one under `tools/` is
the adjudication key `2d50199` re-points.

The gate: `task check`, whole, run after `9842452` with nothing else in the
tree: green in every step. `check:coverage`: 20 packages at or above their floors. `check:ts-coverage`: 22 of 23 source files gated, one excluded. `check:drift` clean; `check:breaking` reporting, not enforcing, because `contract/RELEASED` is absent. `check:invariants`, `check:doc-owner`, `check:new-prose` (150 added lines across 8 files), `check:no-retraction`, `check:no-create-scene`, `check:no-pack` (1221 files each) and `check:arch` clean; `check:race` clean. `check:mutation`: 14 packages, zero unadjudicated survivors, six mutants not evaluated and counted as killed. `check:ts-mutation`: 2882 mutants, 2783 killed, 29 timed out, 70 survivors all adjudicated equivalent, zero unadjudicated. Two of its steps were also run on the working tree before
every commit, `task check:doc-owner` and `task check:new-prose`, both green.

## Done looks like, answered

1. `[x]` `git status --short` prints nothing, and
   `git rev-parse --verify origin/chore/adopt-the-process` prints `9842452`, the same commit.
   Observed after the push of `9842452`. This report is the one file added
   after that, in its own commit.
2. `[x]` The grep with the exclusion prints nothing. Observed after `9842452`,
   and before every commit from `257508f` on. Before the work: 38 lines of the
   prefixed form in four files, and seven occurrences of the bare form on six
   lines in two.
3. `[x]` `docs/requirements.md` holds `project: VTT`, the header and no row:
   `grep -cE '^\| VTT-' docs/requirements.md` prints 0. `requirement-id
   --register <copy> "Probe on a copy."` on a scratch copy prints `VTT-001` and
   writes the row with `**OPEN — no test yet**`; run twice, the second prints
   `VTT-002`. Never run on the register itself.
4. `[x]` The visibility ticket: `grep -c 'VTT-'` prints 0, `grep -c 'IDs are
   assigned'` prints 0, and each of the eight sentences the ticket lists greps
   exactly once. The paragraphs whose words did not change carry HEAD's
   wrapping again: `git diff 1ce9731 --stat` on the file is 31 insertions and
   no deletion.
5. `[x]` `docs/verification-debt.md`: `grep -c 'heal on reference'` prints 0,
   `grep -cE 'VTT-(VIS|LOG)'` prints 0, and one entry dated 2026-09-23 carries the
   labels `test asserts nothing` and `outside the tool` and names
   `TestAClosedDoorSpendsNothing` and `TestARefusedJoinWritesNothingAtAll`. Its
   recipe was measured: with the door term removed from `JoinAdmits`'s guard,
   `go test -count=1 -p 1 ./internal/identity/... ./internal/gateway/...` is
   green in both packages; the reviewer reproduced it in a worktree at HEAD.
6. `[x]` `grep -c 'holds no such commands' CLAUDE.md` and `grep -c 'fourteen
   rows' CLAUDE.md` both print 0; the register bullet names the tag, the header
   and the dispenser, and `grep -c 'Its tag is' CLAUDE.md` prints 1.
7. `[x]` `test -e docs/ADOPTING-THE-PROCESS.md` and
   `test -e docs/reports/DERIVED-contract-experiment.md` both fail. Neither was
   ever committed; copies were taken outside the repository on 2026-09-23, and
   what the handoff measured is in this report.
8. `[x]` Still refused: in a scratch repository with a staged file and this
   tree's `.lefthook.yml`, `lefthook run pre-commit --command review-gate` exits
   1 with the gate's refusal, exits 1 with `CLAUDE_REVIEW_DONE=1` and no record,
   and exits 0 with `CLAUDE_REVIEW_SKIP` set. Observed before the work and by
   the reviewer after the edits, and the five commits above each passed the same
   gate with a record. In a working tree with nothing staged the same command
   reports `(skip) no matching staged files`: lefthook skipping a command that
   has no staged files, which is not the gate passing.
9. `[x]` `docs/specifications/008-requirement-ids-come-from-the-dispenser.md`
   exists; its Status reads "Accepted, and partly implemented" and says "No
   ticket carries it yet".

## What the rules became

The ticket puts no rule on the system, so the `requirements` skill's sort was
not run and no id was allocated. The three rules of the citation chain and the
rule the commit gate holds travel to the first arc's ticket, with the gate that
will hold them. The register holds zero rows at `9842452`.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| The ticket covered the chain gate, its self-test, the Taskfile wiring, the register rows and a specification of the whole chain: plan Tasks 4 to 6 and decisions D5 to D9, D12 and D17. | Narrowed to the prerequisite before any Phase 3 work; those parts moved to the first arc's ticket. | Patrik's ruling, 2026-09-23: the work is scoped per arc, and a gate needs rows to hold; the rows come from the arcs. The verified design stays in the plan for that ticket to start from. |
| Plan Task 2: the register's prose and `CLAUDE.md`'s register bullet restate the evidence values and the replacement of the old rows. | Both point at SPEC-008; the register's prose is two sentences and a pointer; the replacement is recorded here, below. | Patrik's single-source-of-truth ruling, 2026-09-23, after the reading review found the same facts in four places. |
| SPEC-007 committed with one line changed. | Two false sentences corrected first, one paragraph made present-tense, one fact added, and `contract/README.md` trimmed to a pointer. | The review found by command that both generator plugins are local and pinned by go.mod and bun.lock, and that `revoke_actor_control` admits a player revoking their own control; the README carried the same four conventions, a second home. |
| SPEC-008's first draft, before its reading review: titled with an "and", filed as `008-requirement-ids-and-the-chain.md`, restating the dispenser's rules. Not a plan intention; the plan was committed already carrying the final name. | Renamed and retitled without the "and", pointing at the package's `requirements` skill and the dispenser's own header for what they enforce; the review record taken before `9842452` says so. | The specification skill's catches list reads an "and" in a title as two decisions in one file; the restated rules were the package's own, a copy. |
| The shut-door entry, written from the register's own section, said `TestARefusedJoinWritesNothingAtAll` counts rows and so cannot see a touch on an existing row. | It says the test's fixture holds no row at all, so `JoinAdmits` returns at `sql.ErrNoRows` before the guard the recipe edits. | The reviewer read the test: its first assertion is that `join_access` is empty. The register's account was wrong, and copying it forward would have kept it wrong. |
| Six commits, plan D2. | Five. | The fifth, the gate and the rows, left with the narrowing. |
| The two handoff files deleted in the last commit. | Deleted from the working tree before the second commit. | Both were untracked, so no commit could carry a deletion; the recorder refuses a tree with untracked files, and each had to be either staged or gone. |

## What could not be established

- Whether the seven reports committed unchanged in `cc88923` are right about
  their own periods. They were not read; each is read when its arc's ticket
  runs (Patrik, 2026-09-23). The bounded check found no test name or path in
  them that resolves nowhere in the tree or in history; one,
  `internal/perceive/oracle_test.go`, names a file that was never committed.
- Whether an `UPDATE` matching no rows takes SQLite's write lock, the premise
  ADR-011 and the shut-door entry rest on. Asserted by both; not instrumented.
- Whether the visibility report changed in anything but its two headings. The
  file has no committed prior to diff against.
- The plan's task and decision text below its top block still describes the
  wider scope, with the superset ticket's item numbers; the block maps them. It
  is left as the verified record rather than rewritten.

## What was deliberately left out, and where it went

- The chain gate, its self-test, the Taskfile step and the register rows: the
  first arc's ticket, joining-a-table; the design is the plan's D5 to D9 and
  D12.
- Every arc's requirements, re-derived from its exit criteria: one ticket per
  arc.
- The shut-door refusal's missing test: `docs/verification-debt.md`, entry dated
  2026-09-23, and the joining-a-table ticket.
- The full reading of the seven unchanged reports: each arc's ticket.
- A blueprint: none exists (`CLAUDE.md`, "Where the blueprint is"); its own
  ticket.
- A home for Phase 4a adjudications: the first QA run decides (plan D17).
- The three `docs/verification-debt.md` entries that describe
  `internal/perceive`, rolled back uncommitted on 2026-09-20: kept as records of
  their period, with a note above them saying so.
- The proto comment reduction the handoff measured, below: not applied, because
  changing comments regenerates the committed Go and TypeScript; the contract
  arc's ticket.

## The rows that were replaced

Before `257508f` the register held fourteen rows of the shape
`VTT-<SUBJECT>-NNN`, a per-subject series chosen by hand, across three subjects.
The dispenser reads rows of the shape `VTT-NNN` only, so a copy of that register
allocated `VTT-001` beside them. Patrik's ruling, 2026-09-23: replaced, not
migrated. Their rule text was not lost. The visibility rules stay as sentences
in the visibility ticket; the join rules are ADR-011's own sentences and the
tests the ADR names; the two character-log rules are sentences in the
per-character-logs ticket. Each arc's own ticket re-derives its rules from its
exit criteria and allocates them.

## What the handoff measured, kept here because the handoff is gone

`docs/ADOPTING-THE-PROCESS.md` was written 2026-09-23 in a session of the
process's own repository, as the brief for this work, and deleted by this
ticket. What it measured against this tree on that date, before any commit
above:

- Twenty-two tickets under `docs/superpowers/specs/`, thirteen with an Exit
  criteria heading; twenty-five plans; eight reports; eleven ADRs; one
  specification; a register of fourteen rows.
- A calibration run on the contract arc: an agent given the `.proto` files, the
  tests, the fixtures and two consumers, and forbidden the README, the ADR and
  `docs/`, produced a 299-line derivation that could not determine whether three
  absent field numbers may be reused. The specification written from it,
  SPEC-007, was 192 lines, 100 of them rules that do not move when a field is
  added. It took three attempts: one stalled surveying 173 consumer files, one
  hit a spurious API refusal.
- The proto comments went from 58% of the file to 13%, 419 lines to 199, in a
  candidate held outside the tree and not applied.
- A signature for narration in comments (`used to `, `an earlier version`,
  `previously`, `once shipped`, `it cost`, `turned out`, `stopped being true`,
  `was true while`, `keeps rediscovering`, `has hit repeatedly`) found five
  narrations in `commands.proto`, one in `events.proto`, and no false positive
  against the warnings worth keeping. `check:new-prose` is scoped to added
  lines, so wiring the signature there would start green on that tree.

## Starting state, measured before the first edit (plan Task 0)

Free space on the temp volume was between 16 and 17 GiB by `df -g`, at the
mutation gate's 16 GiB guard; `go clean -cache` was run before the self-test
and raised it to 29, so whether the self-test would have passed without the
clean was not measured. `python3 tools/check_mutation_test.py -q`: 107 tests
OK. The position check over `tools/mutation-equivalents.txt`: 36 entries, none
suspect. lefthook 2.1.5, and the plugin path in `.lefthook.yml` present. Eleven
entries in `git status`, plus the ticket once written.
