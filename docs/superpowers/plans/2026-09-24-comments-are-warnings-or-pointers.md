# A comment in code is a warning or a pointer — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-24-comments-are-warnings-or-pointers-design.md`
**Verified:** 2026-09-24, by `verify-ticket`, an agent that did not write the
ticket, against `2e24606` on `chore/adopt-the-process`. Verdict: **Passes with
gaps.** The gaps are listed at the end and travel with this plan. This plan does
not edit the ticket; where an item is thin, the plan decides around it and says
so.

**Goal, in the ticket's words:** a comment in code is a warning or a pointer,
and a gate holds it.

**Patrik's ruling of 2026-09-24, which the ticket implements:** no history in
code comments; the implementation report holds what happened during
development; a comment may only be a warning about how one must do it for it to
work, or a pointer to a document.

**MapTool (CLAUDE.md rule 9), answered in one line.** MapTool has no
comment-content policy to borrow: its Gradle builds run Spotless for formatting
and a mandated licence header (`~/dev/RPTool/dice/build.gradle.kts`, its `spotless {}` block), and
nothing reads what a comment says; this is not a tabletop function, so rule 9
has nothing to import.

## Measurements this plan stands on

All at `2e24606`, by script over `git ls-files`. A comment line is one whose
stripped text starts with `//` in Go, or with `//`, `*` or `/*` in TypeScript.
Production = Go under `internal/` and `cmd/`, not `_test.go`, not `.pb.go`, not
under a `gen/` directory; tests = `_test.go` under the same roots; client =
`client/src/**/*.ts`, not `.d.ts`, not `gen/`. 237 files in all: 76 production,
138 test, 23 client. None of them is generated or under `testdata/` today; the
exclusions are defensive.

| Set | Comment lines | Non-blank lines | Ticket says |
|---|---|---|---|
| production | 12,777 | 26,070 | same |
| tests | 16,813 | 55,688 | same |
| client | 3,301 | 6,188 | same |

`internal/gateway/server.go` is 1,098 of 1,694 (64.8 %); `internal/artlib/artlib.go`
is 844 of 1,216 (69.4 %); `internal/identity/identity.go` 259 of 678 (38.2 %).

Per-file share, median: production 48.2 %, tests 27.7 %, client 50.2 %.
Files at or under 20 %: 2 production, 29 test, 0 client. At or under 30 %: 13,
82, 0. Highest: `internal/engine/actorkind.go` 89.4 %.

The ticket's 1,888 banned-vocabulary lines could not be reproduced (gap G6).
Per term, whole tree, word-bounded, case-sensitive / case-insensitive:

| Term | Lines |
|---|---|
| a date `20YY-MM-DD` | 638 |
| `spec §` | 764 / 802 |
| `Task N` | 542 / 552 |
| `used to` | 153 / 164 |
| `measured` | 118 / 199 |
| `#NNN` | 42 |
| `previously` | 13 |
| `an earlier version` | 11 / 16 |
| `review found` | 8 |
| `turned out` | 3 |
| **any, the ticket's full list** | **2,083 / 2,206** |
| any, the ticket's first seven terms | 2,066 / 2,179 |
| any, the list D1 adopts (case-insensitive, `docs/` paths exempt) | **2,192** (production 952, tests 1,033, client 207) |

Comment blocks (a run of comment lines; D3 says what ends one): 4,318 in all,
24 of them Go package docs. Non-package blocks longer than the bound, by bound:

| Bound | Blocks over, blank lines split | Blocks over, blank lines do not split |
|---|---|---|
| 5 | 1,840 | 1,831 |
| 6 | 1,580 | **1,576** |
| 8 | 1,168 | 1,171 |
| 10 | 907 | 905 |
| 12 | 697 | 695 |
| 15 | 488 | — |
| 20 | 277 | — |

Median block 5 lines; 90th percentile 16; longest non-package block 154
(`internal/rules/expr.go`, the grammar block after its imports);
longest package doc 117 (`internal/artlib/artlib.go`).

**What this branch already adds against `main`** (both local `main`, `a908649`,
and `origin/main`, `fc368ca`; local `main` is behind `origin/main`): 406 comment
lines, of which 13 carry a term from D1's list, in six files —
`internal/engine/actorkind.go` 5, `internal/gateway/metadata_test.go` 3,
`internal/mapdef/compile.go` 2, `internal/gateway/join.go` 1,
`internal/gateway/project.go` 1, `internal/gateway/viewpoint.go` 1. Block
findings on the same diff at bound 6: 43 touched blocks over the bound, or 13
blocks with more than six added lines. Decision D15 exists because of these.

## Constraints that bind every task

- **CLAUDE.md rule 2.** `task check` is never weakened to pass it. The new step
  is added whole; no existing step's scope, threshold or exemption moves.
- **CLAUDE.md rule 8.** Everything written here, in SPEC-010, in the tool's
  docstring, in the Taskfile `desc` and in commit messages cites names, never a
  bare `file:line`, never a `.superpowers/` path, never "this task".
- **The ticket's scope.** No file under `internal/`, `cmd/` or `client/`
  changes. That also means no mutation adjudication key moves
  (`tools/mutation-equivalents.txt`, `tools/ts-mutation-equivalents.txt` key on
  `file:line:col`, per CLAUDE.md rule 8's standing exception).
- **The gate's shape.** A checker proves it ran to the end by a completion line,
  not by its exit code (`findings` in `tools/check-new-prose.py`); a run that
  scans nothing fails (`check-requirements-chain.py`'s exit 2, SPEC-008); a
  base ref that does not resolve fails loudly (`resolve_base`).
- **The self-test is red before the checker exists** (the shape of
  `tools/check_requirements_chain_test.py`, which loads its checker by path with
  `importlib.util.spec_from_file_location`, so a missing checker is an
  import failure, not a skip).
- **Ids come from the dispenser, after sign-off** (SPEC-008). This plan allocates
  none. The sort below names candidates in words.
- **Commits need a review record** (`review-gate` in `.lefthook.yml`).
- **The new tool, its test and its Taskfile entry obey the ruling themselves**,
  though `tools/*.py` and `Taskfile.yml` are outside the checker's scope (ticket,
  "What could not be established"): the docstring and the `desc` say what the
  step does and point at SPEC-010, and carry no history or measurement.

## Decisions this plan makes

**D1 — The banned list.** Forced by: Done 2's list, Done 1's allowed pointers,
and CLAUDE.md rule 8.
Case-insensitive, word-bounded, matched on a comment line's text after every
`docs/…` path has been removed from it:

    \b20\d\d-\d\d-\d\d\b      a date
    \bmeasured\b
    \bused to\b
    \bpreviously\b
    \ban earlier version\b
    \bturned out\b
    \breview found\b
    \bspec §                  a section of a ticket
    \btask \d+\b              a plan's task
    (?<![\w&])#\d+\b          an issue or PR number

Measured: 2,192 comment lines whole-tree (table above). Case-insensitive because
a sentence can open with the word: case-sensitive `measured` finds 118,
insensitive 199; the extra 81 were not read one by one, and the self-test
carries the choice.
**The `docs/` path exemption** resolves a contradiction inside the ticket (gap
G1): Done 1 allows a pointer to "a report by path", and every report path
carries a date (`docs/reports/2026-09-24-joining-record-and-code.md`), so Done
2's date term as written refuses the pointer Done 1 allows. 22 comment lines
carry a date inside a `docs/` path today. **`Task N` stays banned** although
CLAUDE.md rule 8 says "naming a plan and a task within it is fine": rule 8 is
about how to cite, SPEC-010 narrows what a code comment may point at (a
specification, a requirement, a test or symbol, a report), and a plan is not
among them. SPEC-010 and rule 10 say this narrowing in words, so it is not
silent (verify-ticket check 5). Known false positives, accepted because the
scope is added lines and the remedy is a rewording: `internal/rules/resolve_test.go`'s
"roll #1 (vs c) misses"; `client/src/view/camera.ts`'s "the same units `cell`
is measured in".

**D2 — What a comment line is.** Forced by: the ticket's measurement definition
and a measurement. Go: stripped text starts with `//`. TypeScript: `//`, `*` or
`/*`. NOT the `^\s*(//+|\*|#)` marker in `tools/check-comment-wrap.py` that the
ticket's "What it touches" item 1 names: in Go it reads three code lines as
comments today (`*u = Usage{…}` twice in `internal/rules/load.go`, a
`*vttv1.ClientCommand_LoadMap:` case in `internal/harness/engine.go`), and `#`
is not a comment in either language. No TypeScript line outside a block comment
starts with `*` today (checked by script over `client/src`). Directives count as
comment lines (`//go:` 3, `//nolint` 4, `// Complexity:` 4 today); the self-test
holds that a directive on its own is not refused.

**D3 — The block, and its bound: 6 lines.** Forced by: Done 2 leaves the bound
to the plan; the counts above.
A block is a maximal run of comment lines in which a blank line does not end the
run, only a code line does. Measured cost: 1,576 blocks over 6 when blanks do not
split, 1,580 when they do — four blocks — so closing the "insert a blank line"
evasion is free. The package doc is exempt: in Go, a block whose next non-blank
line starts with `package`; TypeScript has no package doc and nothing is exempt
there. Why 6: a warning (the imperative, and the one reason it binds) plus a
pointer is three to four lines at the wrap band; the median block is 5; 6 leaves
room for a doc sentence above a warning on an exported symbol. The alternatives
and their whole-tree counts are in the table above; the self-test carries the
number, as the ticket says.
**Attribution: a block longer than 6 is refused when at least one of its lines
is added.** Not "when more than 6 of its lines are added": that lets a legacy
40-line block grow by five lines per change for ever, which is the unbounded
growth the rule exists to stop, in exactly the files that already hold most of
the text. The cost is real and is sign-off question 3: correcting one sentence
in a long legacy block means cutting that block to 6 in the same change.

**D4 — The ceiling ledger is per file.** Forced by: Done 3 ("for every Go and
TypeScript file"). `tools/comment-ceilings.txt`, one row per in-scope file,
`<repo-relative path>  <share>`, the share in percent to one decimal, **rounded
up** to the next tenth so the file measured when the row was written sits at or
under it. A short header states what the file is, which tool writes it and
points at SPEC-010; unlike `tools/coverage-thresholds.txt`'s header, it carries
no history and no measured figures.

**D5 — A lowered ceiling is written by `tools/check-comments.py --write-ledger`,
never by hand.** Forced by: the ticket's rule "a ceiling is lowered by the change
that lowers the share" is otherwise unheld (gap G3), and a hand edit is where a
typo raises a ceiling. `--write-ledger` writes `min(ceiling, measured share)`
for every row, drops rows whose file no longer exists, and never raises a row;
on a tree with no ledger it writes one from the measured shares (the landing
commit). The gate itself refuses a file whose share has fallen **more than 1.0
percentage point** under its ceiling, naming the file, both figures and the
command to run. Unlike `check:coverage`'s "consider ratcheting" advice
(`SUGGEST_RAISE_BAND` in `tools/check-coverage.py`), which prints and passes,
this refuses: advice only would leave the ticket's rule with no failing
observation. Why a band: adding ten code lines to `internal/gateway/server.go`
moves its share 0.4 points, and a change that only adds code should not have to
touch the ledger. Sign-off question 4.

**D6 — A ceiling is never raised, and the gate checks it.** Forced by: the
ticket's rule, which is otherwise held by nobody. The checker reads the base's
ledger (`git show <base>:tools/comment-ceilings.txt`) and refuses any row whose
value is above the base row for the same path. A row for a path the base's
ledger does not hold is allowed only at or under the default ceiling (D8), or
when `git diff --merge-base -M --name-status <base>` pairs it as a rename of a
path whose base row is at or above it. When the base has no ledger (the landing
change, whose base `main` has none), the comparison is skipped and the run says
so in its completion line; it does not pass silently.

**D7 — A deleted or renamed file.** Forced by: D4's per-file rows and D6.
A row whose file does not exist is refused ("stale row; `--write-ledger` drops
it"), the set-equality `check:coverage` applies to packages. A `git mv` moves the
row by D6's rename pairing, and `--write-ledger` rewrites it under the new path
at the old value or lower. A plain `mv` shows as a deletion plus an untracked
file: the old row is stale and refused, the new file has no row and is held to
D8 — the refusal names both, so the remedy is legible.

**D8 — A file with no row is held to a default ceiling of 25 %.** Forced by: the
ticket does not say what ceiling a file created after the ledger has (gap G4);
without one a new file is unbounded, or enters the ledger at whatever share it
was written with. 25 % is under the test median (27.7 %) and under every
production file but two; a new 60-line file with a three-line package doc and a
doc sentence on five exported symbols sits near 20 %. `--write-ledger` may add a
row for a new file, and only at or under 25 %. Sign-off question 5.

**D9 — Attribution of a rise.** Forced by: the ticket's rule "a file's share
never rises above its ceiling" reds a change that only DELETES code (gap G5),
and this repository deletes old solutions first. A file whose share is above its
ceiling is refused **when the change added at least one comment line to it**
(the added-lines scope of D12). A file above its ceiling that received no added
comment line — its share rose because code left — is printed as a notice and
not refused; the next change that adds a comment line to it is answerable for
bringing it under. The ceiling is not raised for it (D6).

**D10 — How the checker proves it ran.** Forced by: the gate's shape
(constraints). On success it prints, last,
`check:comments: <F> files, <A> added comment lines, <R> ledger rows; clean`
(plus `; no ledger at <base>, raise check skipped` when D6 skips). On findings
it prints each finding and then `check:comments: <N> finding(s) …`. A run that
finds no in-scope file exits 2 with "nothing was scanned, so nothing is proven".
A missing or unparseable ledger exits 2. The self-test asserts the completion
line on the clean fixture, and that a tree with no in-scope file is not a pass.

**D11 — `task check` only, after `check:new-prose`; not pre-commit.** Forced by:
the ticket (Done 2, "a step of `task check`"), and two measurements. The scope
is "added against `main`", so on any branch carrying earlier commits the hook
would judge lines the commit being made did not write (this branch: 13 lines,
D15). And the hook runs on the working tree, not the staged set (the same reason
`check:new-prose` is not a hook). The step costs well under a second: 237 files
and one `git diff`. A pre-commit or pre-push seat is a later decision and would
change CLAUDE.md's gate paragraph; it is not taken here.

**D12 — The added-line scope is `tools/check-new-prose.py`'s, imported, not
copied.** Forced by: that reader already carries the hardening a copy would lose
(`--merge-base`, the `diff --git` header rule, the quoted-path refusal,
untracked files counted whole). `tools/check-comments.py` loads it with
`importlib.util.spec_from_file_location` and calls `resolve_base` and
`added_lines` unchanged, then narrows to D2's roots and files. Their refusals
are prefixed `check:new-prose:`; `check-comments.py` follows any `None` from
them with its own line naming itself, so the reader knows which step failed.
`tools/check-new-prose.py` is not edited (ticket scope).

**D13 — No hatch.** Forced by: the ruling allows no exception, and D1's false
positives have a rewording remedy on added lines. `check:new-prose`'s hatches
(`citations:ok`, `wrap:ok`) are not honoured here. If one proves necessary, it
is its own reviewed decision under rule 2.

**D14 — The deliberate breaks, one per check, run in a scratch clone.**
Forced by: Done 5, and the constraint that the working tree is not edited while
a gate reads it. `git clone --shared` of the repository into the scratchpad, at
the commit that carries the gate; each break is one edit, `python3 -B
tools/check-comments.py main` (no bytecode cache), the output's finding line
recorded, then `git checkout -- .`:

| # | Break | Expected |
|---|---|---|
| B1 | append `// measured 2026-01-01: 35 of 40 trials` above a function in `internal/artlib/artlib.go` | red, naming the file and line, the term `measured` |
| B2 | add a new 7-line comment block above a function in a file under its ceiling | red, block over 6, naming the file and the block's first line |
| B3 | add one comment line to a file whose share sits within one line of its ceiling | red, naming the file and both shares |
| B4 | raise one ledger row by 0.1 | red, row above base (needs a base that carries the ledger: run it with a local commit of the ledger as base) |
| B5 | delete a file that has a row | red, stale row |
| B6 | delete comment lines from one file until its share falls more than 1.0 under its ceiling, ledger untouched | red, naming the `--write-ledger` command |
| B7 | a new file above 25 % with no row | red, naming the default |
| B8 | move the in-scope roots away | exit 2, "nothing was scanned" |
| P1 | add `// Hold mu before calling; SPEC-009 says why.` to a file under its ceiling | silent, exit 0, completion line printed |
| P2 | add `// See docs/reports/2026-09-24-joining-record-and-code.md.` | silent (D1's `docs/` exemption) |

On this branch before D15's merge, B-runs against `main` also report the 13
pre-existing lines; the recorded finding is the break's own line.

**D15 — Landing: `chore/adopt-the-process` merges to `main` before this work
branches.** Forced by: the table "what this branch already adds" — the step's
first run on this branch is red on 13 banned lines and 13 to 43 blocks the
branch already added, and the ticket's two claims "no code under `internal/`
changes" and "`task check` whole is green" (Done 6) cannot both hold here. The
alternative, cleaning those lines here, edits six files under `internal/`, moves
mutation keys in them, and is the sweep's work arriving early. Local `main` must
also be brought to `origin/main` (`git fetch`, fast-forward): `resolve_base`
prefers the local branch, and local `main` is behind. Sign-off question 1.

**D16 — Specification number and requirements.** SPEC-010 at
`docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md`, in
SPEC-008's five headings (Status, Principles served, How it works,
Consequences, Requirements). It states the rule; the banned list, the bound, the
default ceiling and the band as the checker's own to carry (as SPEC-008 leaves
file sets to its checker's docstring); that it narrows CLAUDE.md rule 8 for code
comments (D1); that `contract/*.proto` doc comments are outside it and are
already warnings by SPEC-007's "Consequences"; and that a test's `VTT-NNN`
citation (SPEC-008) is a pointer. Its Requirements line names the ids the sort
below produces, allocated after sign-off.

## The sort's candidates, and what the reading found

Candidates from the ticket's rules and "Done looks like", sorted by the
`requirements` skill. No id is allocated here.

Accepted, each with the observation that fails:

1. **An added comment line carries none of SPEC-010's banned terms.** Observation:
   B1. The ticket's wording ("no date, no measurement and no account of what the
   code used to do") is wider than any check: "35 of 40 trials" with no banned
   word passes. The row states what the check holds; the wider sentence is
   candidate 2.
2. **A comment is a warning, a pointer, or the one-line doc sentence of an
   exported symbol.** No command settles it. `**READING — Phase 4b**`.
3. **A comment block of more than six lines that a change touches is refused,
   the package doc excepted.** Observation: B2. Its row depends on sign-off
   question 3.
4. **A change that adds a comment line to a file leaves that file at or under its
   ceiling.** Observation: B3. The ticket's "a file's share never rises above
   its recorded ceiling" is refused as written: it would red a pure code
   deletion (D9), and no check holds it in that form.
5. **A ledger row is never raised above the base's.** Observation: B4. The
   ticket joins this with lowering in one sentence; the sort splits them.
6. **A share more than 1.0 point under its ceiling is refused until the ceiling
   is lowered.** Observation: B6. Depends on sign-off question 4; under
   advice-only it becomes `**READING — Phase 4b**` or is dropped.
7. **Every ledger row names a file that exists.** Observation: B5.
8. **A file with no row is held to the default ceiling.** Observation: B7.
9. **A run that scans nothing, or cannot establish its base, fails.**
   Observation: B8, and a missing base ref.

Refused:

- "SPEC-010 exists with five headings" and "CLAUDE.md carries rule 10": the
  presence of a document is not a rule of the system. Held by this plan's
  "done" commands.
- "`task check` whole is green": a status, and the run is the status.
- "The ledger records today's shares": a one-time event.
- "The one-line doc sentence is allowed, not required": the ticket leaves it
  undecided; nothing can fail.
- "The sweep lowers ceilings one package at a time": the next tickets' work.

## Tasks, in dependency order

### Task 0 — Precondition (D15)

After sign-off. `chore/adopt-the-process` is merged to `main` by Patrik's call;
local `main` fast-forwarded to `origin/main`; a branch for this ticket cut from
it. The ticket file (untracked today) moves with the work.
**Done when:** `git rev-parse main origin/main` prints one hash, and
`git diff --unified=0 --merge-base main -- 'internal/*.go' 'cmd/*.go' 'client/src/*.ts'`
prints nothing.

### Task 1 — Ids, then SPEC-010

Files: `docs/requirements.md` (by the dispenser only),
`docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md` (new).
Run `requirement-id "<sentence>"` once per accepted candidate, as signed off.
Each row starts `**OPEN — no test yet**`, except candidate 2, whose evidence is
set to `**READING — Phase 4b**`. Write SPEC-010 per D16, its Requirements line
naming the allocated ids.
**Done when:** `task check:requirements-chain` is green;
`grep -c '^## ' docs/specifications/010-a-comment-is-a-warning-or-a-pointer.md`
prints 5; every id the SPEC names is a row.

### Task 2 — The self-test, red

File: `tools/check_comments_test.py` (new). Synthetic git repositories in
temporary directories, as `tools/check_new_prose_test.py` builds them (`git
init`, a base commit, then a change), one test per refusal and per pass in D14's
table (B1 to B8, P1, P2), plus: the completion line on a clean fixture; a missing
base ref fails; a directive (`//go:embed`, `//nolint:gocyclo`) is not refused;
a Go line starting with `*` is not a comment; a blank line does not end a block;
the package doc is exempt and a TS file's opening block is not; a `git mv`
carries its row; `--write-ledger` never raises and drops a stale row. Each test
cites its id on the line above it (SPEC-008).
**Done when:** `python3 tools/check_comments_test.py` fails at import because
`tools/check-comments.py` does not exist. The output's last lines are recorded
for C2's message.

### Task 3 — The checker

File: `tools/check-comments.py` (new). Per D1–D13: `check-comments.py <base-ref>`
for the gate, `--write-ledger` for D5, `--report` for the whole-tree counts the
sweep lowers (per file: share, ceiling, banned lines, blocks over the bound).
Docstring: what it refuses and a pointer to SPEC-010; no history.
**Done when:** `python3 tools/check_comments_test.py` is green, and
`python3 tools/check-comments.py --report | tail -1` prints the whole-tree totals,
which match this plan's measurements (2,192 banned lines, 1,576 blocks over 6)
or the difference is explained in the report.

### Task 4 — The ledger and the wiring

Files: `tools/comment-ceilings.txt` (new, by `--write-ledger` on a tree with no
ledger), `Taskfile.yml`. A `check:comments` task: `desc` one short paragraph
ending in a pointer to SPEC-010; `cmds` run `python3
tools/check_comments_test.py -q` then `python3 tools/check-comments.py main`.
Listed in `check:`'s `cmds` directly after `task: check:new-prose`.
**Done when:** `wc -l` of the ledger minus its header equals 237 (or the in-scope
file count at that commit); `task check:comments` is green and its last line is
the completion line; `task --list-all | grep -c 'check:comments'` prints 1;
`grep -n 'check:comments' .lefthook.yml` prints nothing.

### Task 5 — The deliberate breaks (D14)

In a scratch clone at the commit that carries Tasks 1–4, never the working tree.
**Done when:** every B row reds with the expected finding and every P row is
silent; the finding lines are in C2's message, one per break.

### Task 6 — CLAUDE.md

File: `CLAUDE.md`. Rule 10, one paragraph: a comment in code is a warning to
whoever edits next, a pointer (SPEC-010's list), or the doc sentence of an
exported symbol; it narrows rule 8's citable targets for code comments; held by
`check:comments` on added lines and by the ceiling ledger; SPEC-010 is the
record. The gate paragraph's "everything else — …" list names `check:comments`.
Rule 10 itself carries no date other than the ruling's, as rule 9 does.
**Done when:** `grep -n '^10\. \*\*' CLAUDE.md` prints one line;
`grep -c 'check:comments' CLAUDE.md` prints at least 2.

### Task 7 — The gate, whole, once

After review has settled and the tree is final. `task check` launched in its own
session (it outlives a process-group teardown otherwise).
**Done when:** it exits 0 and its `check:comments` step printed the completion
line.

### Task 8 — The rows' evidence, the report

Files: `docs/requirements.md` (evidence cells for the accepted rows, from OPEN to
`tools/check_comments_test.py#<test>`; by hand only in the evidence column, as
the previous arc did), `docs/reports/2026-09-24-comments-are-warnings-or-pointers.md`
(new, Phase 5, the `implementation-report` skill), SPEC-010 re-read against the
tree.
**Done when:** `task check:requirements-chain` is green with no OPEN row among
this arc's except any sign-off left open.

The evidence cells change after the test names exist, so in practice Task 8's
register half lands in C2 with the tests. Only the report waits.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, SPEC-010, the allocated rows (OPEN) | pre-commit hook (lint, vet, tier-1, arch, invariants, doc-owner, secrets, typecheck, review gate); by hand before it: `task check:requirements-chain` |
| C2 | the self-test, the checker, the ledger, the Taskfile step, the rows' evidence; the message records the red self-test output and one finding line per break | pre-commit hook; by hand: `task check:comments`, `task check:requirements-chain`, `task check:new-prose` |
| C3 | CLAUDE.md rule 10 and the gate paragraph | pre-commit hook |
| — | `task check` whole, once, on C3's tree (Task 7) | every step |
| C4 | the implementation report | pre-commit hook |

Push after C4: pre-push runs tiers 2–3, `check:drift`, `check:breaking` (about
three minutes; let it run). No commit touches Go or TypeScript, so no mutation
key moves and `check:drift` has nothing to compare.

Phase 4a (independent QA, from SPEC-010 and the checker's usage alone) runs
before C2 per the dev-cycle package; if it writes tests they land as
`tools/check_comments_qa_test.py`, an eighth path the ticket does not list, and
are added to the `check:comments` task's `cmds` beside the self-test, as
`check:requirements-chain` runs its QA file.

## Gaps that travel with this plan

- **G1 — Done 1 and Done 2 contradict each other, and the ticket narrows
  CLAUDE.md rule 8 without saying so.** A report path carries a date, so a
  pointer Done 1 allows is a line Done 2 refuses; and rule 8 names "a dated
  decision" and "a plan and a task within it" as citable, which the date and
  `Task N` terms refuse in code comments. D1 resolves the first by exempting
  `docs/` paths; SPEC-010 and rule 10 name the second.
- **G2 — Done 6 cannot hold on this branch as the ticket scopes it** (13 banned
  lines and 13–43 blocks the branch already adds against `main`). D15, sign-off
  question 1.
- **G3 — The ticket's "a ceiling is lowered by the change that lowers the
  share" has no failing observation in Done 3,** which only reports a fall. D5
  makes it a refusal with a band.
- **G4 — A file created after the ledger has no ceiling in the ticket.** D8.
- **G5 — "A file's share never rises above its ceiling" reds a pure code
  deletion.** D9 attributes a rise to added comment lines.
- **G6 — The ticket's 1,888 cannot be re-derived:** it does not record its list's
  exact form, and every reasonable reading gives 1,730 to 2,206. The argument
  does not depend on the figure; the report quotes D1's 2,192 with its commit.
- **G7 — The ticket's "What it touches" item 1 names `check-comment-wrap.py`'s
  comment marker,** which misreads Go code lines and `#`. D2 uses the ticket's
  own measurement definition instead.
- **G8 — The ticket locates the "used to be visible" sentence in "the block
  above the read loop's `s.ids.Lookup(p.ID)`".** That block narrates history
  too, but the quoted sentence is in the next block below the call, the one
  opening "ANY OTHER ERROR IS OPERATIONAL". It also cites "the reading review of
  joining ticket 2", which is no durable name; the source is item 4 of
  `docs/reports/2026-09-24-joining-record-and-code.md`. Neither changes the
  work.
- **G9 — The Phase 4a QA file and the report are not among the ticket's seven
  paths.** Both are the process's; named above.
- Carried from the ticket, not decided here: whether the doc sentence on an
  exported symbol is required; whether `tools/*.py` docstrings and Taskfile
  `desc` blocks come into scope; whether the sweep runs per package or per arc.

## Questions for sign-off

1. **Merge `chore/adopt-the-process` to `main` before this work, and cut a fresh
   branch (D15)?** Recommend yes. The alternative is cleaning 13 lines in six
   `internal/` files here, which breaks the ticket's scope and moves mutation
   keys.
2. **The banned list as D1 states it, case-insensitive, with `docs/` paths
   exempt and `Task N` banned despite rule 8?** Recommend yes; SPEC-010 and rule
   10 say the narrowing out loud.
3. **The block bound, 6 lines, and a touched block answerable for its whole
   length (D3)?** Recommend yes to both. If correcting a sentence in a legacy
   block must not force cutting it, the alternative is "more than 6 added lines
   in one block" — at the cost of legacy blocks that can grow without limit.
4. **Refuse a share that has fallen more than 1.0 point under its ceiling, and
   lower ceilings only with `--write-ledger` (D5)?** Recommend yes. Advice only,
   as `check:coverage` does, leaves the ticket's lowering rule unheld.
5. **A default ceiling of 25 % for a file with no row (D8)?** Recommend yes.
6. **A rise caused only by removed code is a notice, not a refusal (D9)?**
   Recommend yes; this repository deletes old solutions first, and a gate that
   reds a deletion for its neighbours' comments is one that gets bypassed.
7. **No hatch (D13)?** Recommend yes; revisit only as its own reviewed decision
   if a real warning cannot be written without a banned term.
