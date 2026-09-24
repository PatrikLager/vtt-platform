# internal/identity carries only warnings and pointers — implementation plan

**Ticket:** `docs/superpowers/specs/2026-09-24-sweep-identity-design.md`
**Verified:** 2026-09-24, by `verify-ticket`, an agent that did not write the
ticket, against `e872467` on `chore/sweep-identity`. Verdict: **Passes with
gaps.** The gaps are listed at the end and travel with this plan. This plan does
not edit the ticket; where an item is thin, the plan decides around it and says
so.

**Goal, in the ticket's words:** `internal/identity` carries only warnings and
pointers, and its ceilings say so.

**MapTool (CLAUDE.md rule 9), in one line.** Nothing to borrow: this is a sweep
of code comments under SPEC-010, not a tabletop function, and MapTool has no
comment-content policy (the ticket says the same).

## Measurements this plan stands on

All at `e872467`, by command, re-run by the verifier.

`python3 tools/check-comments.py --report | grep internal/identity/`:

| File | Share | Ceiling | Banned | Blocks > 6 |
|---|---|---|---|---|
| `fault_internal_test.go` | 24.4 | 24.5 | 7 | 4 |
| `identity.go` | 38.2 | 38.3 | 0 | 14 |
| `identity_failure_test.go` | 29.1 | 29.2 | 2 | 1 |
| `identity_test.go` | 22.7 | 22.8 | 20 | 22 |
| `qa_joining_internal_test.go` | 12.6 | 12.7 | 0 | 0 |

Comment lines of non-blank lines, by `measure()` from `tools/check-comments.py`:
`identity.go` 259 of 678 (46 blocks, the 14 over the bound holding 150 lines);
`identity_test.go` 380 of 1,674 (22 over, 239 lines); `fault_internal_test.go`
94 of 385 (4 over, 58 lines); `identity_failure_test.go` 39 of 134 (1 over, 24
lines); `qa_joining_internal_test.go` 23 of 182. Every figure in the ticket's
table matches.

Doc blocks longer than two lines above `func Test`, by the Task 0 script:
adjacent definition 30 (29 in `identity_test.go`, 1 in
`identity_failure_test.go`), which is the ticket's figure; the gate's block
definition 32 (adds `fault_internal_test.go`'s twenty-line block and
`identity_failure_test.go`'s file header, each separated from its test by a
blank line). Lines in `identity_test.go`'s doc blocks above tests: 225
adjacent, 255 by the gate's blocks; the ticket's 253 is neither (gap 1).

`--write-ledger` run at `e872467` in a scratch clone: `237 rows written`,
`git diff --stat` empty. No row moves today, so after the sweep only the rows of
files the sweep changes can move.

Adding `// SPEC-009` between two code lines in `migrate` at `e872467`, in a
scratch clone with `main` at `e872467`: `check:comments: 237 files, 1 added
comment lines, 237 ledger rows; clean`, exit 0 (260 of 679 is 38.29, under the
ceiling 38.3). Item 4's break fails today, as a "done" item must. The same line
placed above `var joinAccessShape` joined the next block across a blank line and
was refused for the bound instead, so the break's placement matters (D13).

`grep -n identity.go tools/mutation-equivalents.txt tools/ts-mutation-equivalents.txt`
prints nothing: no adjudication key names a file in `internal/identity`, and
gremlins does not mutate test files, so no key can move. `./internal/identity/`
is the first entry of `PACKAGES` in `tools/check-mutation.py`, so the mutation
gate re-mutates it on the next run.

Citations per file, `grep -o 'VTT-[0-9]*' <file> | sort | uniq -c`:
`identity_test.go` 20 ids (VTT-015, VTT-043 and VTT-049 twice), 23 tokens on 21 lines;
`fault_internal_test.go` VTT-005, 006, 008, 045, 046; `qa_joining_internal_test.go`
VTT-008 four times; `identity.go` and `identity_failure_test.go` none. The chain
gate refuses a row whose evidence file no longer carries its id.

Baselines green at `e872467`: `python3 tools/check-doc-owner.py .` ("79 files,
every doc comment sits on its own function"), `python3
tools/check-requirements-chain.py .` ("58 rows, 191 test files, 4
specifications"), `python3 tools/check_mutation_test.py -q` (OK, under two
seconds), `gofmt -l internal/identity/` (nothing).

## Constraints that bind every task

- `CLAUDE.md` rule 10 and SPEC-010: a comment is an imperative warning, a
  pointer, or the one-line doc sentence of an exported symbol. VTT-051 is held
  by the Phase 4b reading; VTT-050, VTT-052, VTT-053 and VTT-055 by
  `check:comments`.
- `CLAUDE.md` rule 2: no gate is weakened. The ledger only goes down, and only
  through `--write-ledger`.
- `CLAUDE.md` rule 8, narrowed by rule 10 for code: a comment points at a
  specification by number, a requirement by id, a test or symbol by name, or a
  report by its `docs/` path; never a line number, a date, a commit hash, a plan
  or a task.
- SPEC-008: every `VTT-NNN` a test file carries today stays in that file, and no
  id is chosen by hand.
- The ticket: no code line changes. The Task 0 token comparison is the check.
- `qa_joining_internal_test.go` is not touched; `docs/reports/` gains only this
  ticket's report.

## Decisions this plan makes

**D1. The sort has three kinds and nothing else.** Every comment line in the
four files is read and ends up as exactly one of:

- **A warning**, written imperatively to whoever edits next: a verb first (`Do
  not`, `Keep`, `Read`, `Close`, `Set`), then a colon and the consequence of not
  doing it, in the present tense. At most three lines. No account of how it was
  learned, no figure measured, no earlier version.
- **A pointer**: `SPEC-009`, `VTT-NNN`, a test name (`TestX`), a symbol name
  (`migrateLocked`, `store.Open`), or `docs/verification-debt.md`. A pointer may
  close a warning or a doc sentence in parentheses, as SPEC-009 is cited today.
  A plan, a commit hash, a date, a review, `§` and `#NNN` are not targets.
- **The doc sentence of an exported symbol**: the first sentence `go doc`
  prints, kept as it stands where it is true, cut to the fact where it is not
  one sentence.

Anything else is deleted: history, measurements, arguments, descriptions of what
the code does, section banners naming a plan's task. Forced by rule 10's wording
and SPEC-010's "How it works", which name these three and exclude the rest.

**D2. Unexported symbols keep no doc sentence.** Rule 10 allows the doc
sentence of an EXPORTED symbol. The doc blocks on `schema`, `migrate`,
`migrationPending`, `tableShape`, `columnNames`, `shapeReader`,
`migrateLocked`, `driverName`, `ensureJoinRow`, `newSecret` and the test
helpers (`withFaultDriver`, `preBudgetCampaign`, `tamperRow`; `openTemp` has
none) keep only their warnings and pointers; a block that holds neither goes.
`check:doc-owner` fires only when a doc's first word names another function, so
a deleted doc cannot trip it. Forced by rule 10; a sign-off question in case the
reading is thought too literal (Q3).

**D3. The package doc is one sentence and a pointer.** `// Package identity
...` is exempt from the bound but not from rule 10. It keeps what the package
holds and `(SPEC-009)`; "deliberately NOT event-sourced" is SPEC-009's first
paragraph.

**D4. A doc sentence is one sentence, on one line where it fits.** SPEC-010 says
"one-line". A sentence that cannot be cut to one line without losing a fact may
wrap to a second physical line, and Phase 4b names each such case. Forced by the
wrap band `check:new-prose` holds on added lines (`SHORT, LONG = 55, 85` in
`tools/check-comment-wrap.py`), which a crammed line would breach. Q8.

**D5. The test-file rule.** Above each `func Test`: an optional line saying how
the test observes what its name states, then the `VTT-NNN` line (one or more ids
on it, exactly as today), directly above `func`. Nothing else. The `VTT-NNN`
line is never dropped, moved to another test, or invented for a test with none:
the multiset of ids per file is identical before and after (Task 0's
`citations` command). Placement follows the file's own convention, where the id
line sits directly above `func` in all 21 citation lines of `identity_test.go`.

A warning in a test's doc block, one that says what nothing else holds (the
mutation gate cannot mutate SQL text, so this case is the only guard on
`ORDER BY display_name, id`), moves into the body, above the assertion it
guards, as a D1 warning of at most two lines. It does not stay above `func`.

Comments inside test bodies get the D1 sort with no doc-sentence kind.

**D6. The sweep dispenses no requirement row.** A test's doc block that states
a rule no row states is a candidate for a new row (the ones visible now are in
"Candidates the reading starts from"). The sweep refuses to allocate: the ticket
says it adds no rule, the `requirements` skill allocates only after a sort and
a sign-off, and a row brings a `VTT-NNN` line into a test file, which D5
forbids inventing. Each candidate goes into the report, one line with the test
that would be its evidence and the observation that would fail, for a later
ticket to sort. Q1.

**D7. A decision only a comment holds moves to SPEC-009, in the same commit.**
When the reading of `identity.go` finds a sentence that states how identity
works and SPEC-009 does not say it, the sentence is checked against the code,
written into SPEC-009's "How it works" in the present tense, and the comment
becomes a pointer or goes. Checked means: the reading names the code line's
symbol and what it does, and Phase 4b re-reads the sentence against it. The
comment and its new home change in one commit, so no tree exists where the fact
is in neither place. Forced by ticket item 5 and SPEC-010's "the specification
holds how the system works now". SPEC-009's Status and Requirements do not
change. Q2.

**D8. What the sweep leaves alone, and why.**

- The SQL `--` lines inside the `schema` string literal (15 lines). They are
  Go string content, not comments; the gate does not count them; editing them
  changes the statement `Open` executes and the token comparison would show it.
  The ticket's "no code line changes" rules them out. Gap 5.
- `// #nosec G202 -- column is a test-supplied literal, never external input.`
  in `identity_failure_test.go`: gosec's suppression, machine input, kept
  verbatim.
- The warning at `ensureJoinRow` about the upsert blocking for the full
  `busy_timeout(5000)` and failing `SQLITE_BUSY`, in imperative form:
  `TestAnUnreadableIdentityRefusesTheCommandWithoutKickingAnybody`'s doc block in
  `internal/gateway/server_test.go` points at "identity.go's own comment" for
  exactly that fact. If the reading finds that warning cannot stay, the sweep
  stops and asks, because the fix is in a file the ticket does not name.

Trailing comments (`secret := newSecret() // used only if ...`, `_ = db.Close()
// closing a handle ...`) are not comment lines to the gate and move no share.
They get the D1 sort anyway; the report counts them separately.

**D9. Order of files: `identity.go` first.** Test comments point into it
(`TestOpeningACurrentCampaignTakesNoWriteLock` names "ensureJoinRow's own
comment", `Participant`'s doc names "schema's note", `fault_internal_test.go`
names "the reason stated at that arm"). Sorting the target first fixes what
survives, and each pointer in the tests is then rewritten to a symbol, a test or
SPEC-009, or deleted. Then `identity_test.go`, `fault_internal_test.go`,
`identity_failure_test.go`, SPEC-009 if D7 applies, and the ledger last.

**D10. One commit for the sweep.** It carries the ticket, this plan, the four
files, the ledger, and SPEC-009 if D7 adds to it. Forced by three things: the
band refuses a file more than 1.0 under its ceiling, so each file's deletions
and its row must land together, which one commit does for all four at once; D7
needs the comment and its SPEC-009 sentence in one commit; and the review gate
wants a record per commit while Phase 4b is one reading over the package, whose
cross-file pointers (D9) are only checkable together. Per-file commits would buy
smaller diffs and cost four review records over one reading. The report is a
second commit. Q4.

**D11. The comment-stripped comparison is a token stream.** Task 0 builds a
twenty-line Go program from the listing below with `go/scanner`, which drops
comments when its mode is 0 and prints each remaining token and its literal. For
each file, the stream at `e872467` (`git show e872467:<path>`) and the stream of
the working tree compare identical with `cmp`. Tokens, not `gofmt` output: a
printer re-derives blank lines from positions, so deleting a comment line
between two statements changes `gofmt` output while changing no code, and would
be a false difference. The verifier measured the program: deleting every `//`
line from `identity.go` gives an identical stream (2,742 tokens), and changing
`admitted >= budget` to `>` gives exactly one differing line. A run proves it
ran by printing each file's token count, which must be non-zero: a first
attempt, run as `go run` on a path outside its directory, printed nothing for
both sides and "compared equal".

**D12. The ledger.** After Phase 4b has settled and no comment will change
again, `python3 tools/check-comments.py --write-ledger`, then `git diff
tools/comment-ceilings.txt`. It must show exactly four changed rows, each
lowered, each `internal/identity/`: `fault_internal_test.go`, `identity.go`,
`identity_failure_test.go`, `identity_test.go`. `qa_joining_internal_test.go`'s
row stays at 12.7 because its file is untouched; the ticket's "five rows" reads
as four that move and one that stays (gap 2). The command rewrites the whole
file, but lowers a row only to `min(old, measured)`, and at `e872467` it moves
none, so a fifth changed row means another file changed after `e872467`; if one
appears, stop, name it, and ask before committing it. Last because every comment
edit moves a share, and a row written before the last edit is either refused by
the band or leaves headroom the next change can spend.

**D13. The deliberate break.** In a scratch clone with the final diff applied
and committed there, and that clone's `main` pointing at that commit: first
`python3 tools/check-comments.py main` exits 0 with its completion line; then
one pointer line, `// SPEC-009`, is added to `identity.go` between two code
lines, inside a function body, where no block can absorb it. The gate must exit 1
with the finding `internal/identity/identity.go: comment share X is above its
ceiling Y and this change added a comment line to it (SPEC-010)`, naming the
file and both shares. The added line carries no banned term and joins no block,
so that finding is the only one. It is guaranteed to fire: `--write-ledger`
writes `ceil1(share)`, at most 0.1 of headroom, and one comment line on a file of
about 500 non-blank lines at about 16 percent raises the share by about 0.17.
The commit message records the finding line verbatim. Forced by ticket item 4.

**D14. Gate steps before the commit, in order.** `gofmt -l internal/identity/`
prints nothing; `go vet ./internal/identity/`; `go test -count=1
./internal/identity/...`; `task check:comments`; `task check:doc-owner` (doc
comments move); `task check:requirements-chain` (the `VTT-NNN` lines are
comments that move); `task check:new-prose` (citations and wrap on the added
comment lines); `python3 tools/check_mutation_test.py -q` and `python3
tools/check_ts_mutation_test.py -q` (fixture-only self-tests; no identity key
exists to move); `task lint`, since the `whitespace` linter refuses a blank line
left at the top of a block when a comment there goes. Then `task check` whole,
once, on the final tree, launched in its own session (`start_new_session=True`,
or the run dies at the parent's teardown), and only after Phase 4b has settled,
so no edit lands mid-run. The pre-commit hook then runs its own set, the review
gate among them.

**D15. Phase 4a is skipped, with its reason.** Independent QA derives tests
from a specification to find behaviour the implementer got wrong. This change has
no behaviour: the token comparison shows every file's code identical to
`e872467`. What can be wrong is a sentence, and a reading holds that: VTT-051 is
`**READING — Phase 4b**`.

**D16. Phase 4b is the check for VTT-051, block by block.** The reviewer gets
the diff (`git diff HEAD`, which shows staged deletions), SPEC-010, SPEC-009, the
register rows VTT-005 to VTT-058, and the four files. For every surviving block
it names the kind (D1), and for a warning, the code it guards and whether the
consequence stated is true of that code; for a pointer, that the target
resolves (`grep` for the test or symbol, the row in `docs/requirements.md`); for
a doc sentence, that the symbol is exported and the sentence is true. For every
deleted block it answers one question: did it hold a fact that is now in no
record? If yes, that fact goes to SPEC-009 (D7), becomes a D6 candidate, or is
named in the report as dropped with the reason. For every SPEC-009 sentence
added, it re-reads the sentence against the named code.

**D17. What the report records.** Per file: comment lines and share before and
after, blocks deleted, blocks kept by kind, trailing comments removed; the
sentences moved into SPEC-009, each with the symbol it describes; the candidate
rows (D6); the four ledger rows old and new; the token counts from D11; the
break's finding line (D13); the gaps below as found or closed. The report does
not revise `docs/reports/2026-08-09-joining-a-table.md` or
`docs/reports/2026-09-24-joining-record-and-code.md`, which hold this package's
history.

## Candidates the reading starts from

Read by the verifier at `e872467`; the reading in Tasks 1 to 4 confirms or
overrides each, and Phase 4b checks the outcome. A starting point, not a
verdict.

**`identity.go`: facts SPEC-009 does not state (D7 candidates).**

1. `JoinBudget` and `JoinSecret` read before they write, and `JoinBudget` mints
   nothing: the DM console polls them, and a poll takes no write lock on the file
   `internal/store` appends to (`ensureJoinRow`, `JoinBudget`).
2. `SetJoinOpen` writes the limit on every call, closing included, and
   `JoinBudget` reports it until the next opening.
3. A stored role that does not parse is refused by `Verify`, `Lookup` and
   `List`, never skipped or defaulted.
4. `Verify` finds the row by plain equality on the SHA-256 hash, which is safe
   because the hash is not secret to someone without the token, and confirms the
   match with `subtle.ConstantTimeCompare`.
5. `JoinOpen` answers false when the database cannot be read (VTT-048 holds it;
   SPEC-009's "How it works" does not name `JoinOpen`).

**`identity.go`: warnings to keep, imperative.** The `controls` column is not to
be dropped by a migration; `migrate` stays idempotent and takes no write lock
before `migrationPending` answers; `BEGIN IMMEDIATE`, not `db.Begin()`;
`tableShape`'s pragma stays a literal; `rows` is closed before
`migrateLocked`'s `ALTER TABLE`; the re-read under the lock, with its
`docs/verification-debt.md` pointer; the door-budget `UPDATE` keyed on state;
`driverName` set only by internal tests and restored; `JoinAdmits`' refusals
before the write, the empty-secret term, no compensating decrement;
`RotateJoinSecret`'s `DO UPDATE` leaving `open` and `admit_limit` and resetting
`admitted`; `ensureJoinRow`'s read first (D8) and `SET secret = secret`.

**Test files: rules no row states (D6 candidates).**
`TestMigratingTwiceIsNotAnError`, `TestOpeningACurrentCampaignTakesNoWriteLock`,
`TestAnAlreadyMigratedReadOnlyCampaignStillOpens`,
`TestMigrationSurvivesConcurrentFirstOpens`,
`TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying`,
`TestOpeningAnUnreadableCampaignFailsLoudly`,
`TestACampaignPredatingTheAdmissionBudgetStillWorks`,
`TestTheDoorOpensOnACampaignThatAlreadyHasALink`,
`TestOpeningTheDoorFirstStillMintsARealSecret`,
`TestJoinBudgetReportsWhatHasBeenSpent`,
`TestCreateInviteRefusesARoleThatIsNotOne`,
`TestTheIdentityStoreReportsFailuresRatherThanPretending`,
`TestVerifyFailsClosedOnInvalidStoredRole`, `TestOperationsFailAfterClose`, and
the `TestAMigrationThatCannot...RefusesTheCampaign` family in
`fault_internal_test.go`. Each carries no `VTT-NNN` today.

**Text that goes on sight.** Every section banner naming "joining-a-table T1",
"J3" or "J4" (a plan); "spec §2", "spec §3.1", "§3.2"; "#40", "#42"; every
date; "measured", "review found", "used to", "the first version"; the
`withControlColumnCampaign` block in `fault_internal_test.go` (twenty lines
about a fixture and four tests that no longer exist; confirmed by reading);
`identity_test.go`'s 31-line block about two deleted tests above
`TestMigratingTwiceIsNotAnError`; `identity_failure_test.go`'s two bare line ranges, one into `identity.go` and
one into `cmd/vtt/serve_compose.go` (rule 8's forbidden shape) and "the enforcement plan's note" (a plan).

Projection, not a target: keeping about 82 comment lines in `identity.go` (the
doc sentences of its 20 documented exported symbols, the package doc, the
warnings above at one to three lines each) puts it near 16 percent against 419
code lines, under the ticket's 20.0.

## Tasks, in dependency order

### Task 0 — Instruments and baselines

**Files:** none in the repository. Everything goes in the session's scratchpad
directory, `$S` below.

1. Write `$S/codetokens/main.go` from this listing and build it with `(cd
   "$S/codetokens" && go build -o codetokens main.go)`:

   ```go
   // Print a Go file's token stream with comments dropped, one token per line.
   package main

   import (
   	"fmt"
   	"go/scanner"
   	"go/token"
   	"os"
   )

   func main() {
   	src, err := os.ReadFile(os.Args[1])
   	if err != nil {
   		fmt.Fprintln(os.Stderr, err)
   		os.Exit(2)
   	}
   	fset := token.NewFileSet()
   	var s scanner.Scanner
   	errs := 0
   	s.Init(fset.AddFile(os.Args[1], -1, len(src)), src,
   		func(_ token.Position, msg string) { errs++; fmt.Fprintln(os.Stderr, msg) }, 0)
   	for {
   		_, tok, lit := s.Scan()
   		if tok == token.EOF {
   			break
   		}
   		fmt.Printf("%s %q\n", tok, lit)
   	}
   	if errs > 0 {
   		os.Exit(2)
   	}
   }
   ```

   The comparison, run from the repository root:

   ```sh
   for f in internal/identity/identity.go internal/identity/identity_test.go \
            internal/identity/fault_internal_test.go \
            internal/identity/identity_failure_test.go \
            internal/identity/qa_joining_internal_test.go; do
     git show "e872467:$f" > "$S/before.go"
     "$S/codetokens/codetokens" "$S/before.go" > "$S/before.tok" || echo "SCAN FAILED $f"
     "$S/codetokens/codetokens" "$f" > "$S/after.tok" || echo "SCAN FAILED $f"
     printf '%s %s/%s tokens ' "$f" "$(wc -l < "$S/before.tok")" "$(wc -l < "$S/after.tok")"
     cmp -s "$S/before.tok" "$S/after.tok" && echo same || echo DIFFERS
   done
   ```

2. Write `$S/test-doc-blocks.py` from this listing:

   ```python
   """Count comment blocks longer than two lines above `func Test` lines.

   Two definitions, both printed: ADJACENT is the run of comment lines directly
   above the func line (a blank line ends it); GATE is the block
   tools/check-comments.py measures, which a blank line does not end.
   Usage: python3 test-doc-blocks.py <file>...   (run from the repository root)
   """
   import importlib.util, re, sys

   spec = importlib.util.spec_from_file_location("cc", "tools/check-comments.py")
   cc = importlib.util.module_from_spec(spec); spec.loader.exec_module(cc)

   total_adj = total_gate = 0
   for path in sys.argv[1:]:
       lines = cc.read(path)
       _, _, blocks = cc.measure(lines, False)
       end_of = {blk[-1]: blk for blk, _ in blocks}
       adj = gate = 0
       for i, line in enumerate(lines, 1):
           if not re.match(r"func Test", line):
               continue
           n, j = 0, i - 1
           while j >= 1 and lines[j - 1].strip().startswith("//"):
               n, j = n + 1, j - 1
           j = i - 1
           while j >= 1 and not lines[j - 1].strip():
               j -= 1
           g = len(end_of.get(j, []))
           name = line.split("(")[0][5:]
           if n > 2:
               adj += 1
           if g > 2:
               gate += 1
           if n > 2 or g > 2:
               print("  %s %s adjacent=%d gate=%d" % (path, name, n, g))
       print("%s adjacent>2: %d  gate>2: %d" % (path, adj, gate))
       total_adj += adj; total_gate += gate
   print("test-doc-blocks: %d files, adjacent>2: %d, gate>2: %d"
         % (len(sys.argv) - 1, total_adj, total_gate))
   ```

3. Record the baselines:
   - `python3 tools/check-comments.py --report | grep internal/identity/`
   - `python3 "$S/test-doc-blocks.py" internal/identity/*_test.go`
   - `for f in internal/identity/*.go; do echo "$f $(grep -o 'VTT-[0-9]*' "$f" | sort | tr '\n' ' ')"; done > "$S/citations.before"`
   - the comparison loop above, on the unchanged tree.

**Done when:** the report shows the table under "Measurements"; the counter
ends `test-doc-blocks: 4 files, adjacent>2: 30, gate>2: 32`; the comparison
prints `same` for all five files with token counts 2742, 8974, 1883, 692 and
1200 in the loop's order.

### Task 1 — `identity.go`

**Files:** `internal/identity/identity.go`.

Read each of the 46 blocks and each trailing comment under D1 to D4 and D8,
starting from "Candidates the reading starts from". For each D7 candidate the
reading confirms, write down the sentence and the symbol it describes, for
Task 5. Keep `ensureJoinRow`'s `busy_timeout` warning (D8).

**Done when:** the comparison prints `same` for `identity.go` with 2742 tokens;
`--report` shows `identity.go` with `banned 0`, `blocks>6 0` and a share at or
under 20.0; `gofmt -l internal/identity/` prints nothing; `go doc
./internal/identity` lists every exported symbol it listed at `e872467`, each
with its doc sentence; `grep -nE '[a-z_]+\.go:[0-9]' internal/identity/identity.go`
prints nothing.

### Task 2 — `identity_test.go`

**Files:** `internal/identity/identity_test.go`.

D5 above every test; D1 inside bodies; pointers into `identity.go` re-aimed at
what Task 1 left. D6 candidates listed as found.

**Done when:** `test-doc-blocks.py` prints `adjacent>2: 0  gate>2: 0` for the
file; `--report` shows `banned 0` and `blocks>6 0`; the file's citation line in
`citations.before` matches the working tree's; the comparison prints `same`
with 8974 tokens; `go test -count=1 ./internal/identity/...` is green.

### Task 3 — `fault_internal_test.go` and `identity_failure_test.go`

**Files:** those two.

The same rules. `withControlColumnCampaign`'s block and
`TestVerifyFailsClosedOnCorruptControls`'s block go whole; the `#nosec` line
stays verbatim; `identity_failure_test.go`'s file header keeps at most the one
warning a reader needs (which paths are not tested, if the reading finds that a
warning at all) and no line range.

**Done when:** as Task 2, for these two files (1883 and 692 tokens), and the
counter's total line reads `test-doc-blocks: 4 files, adjacent>2: 0, gate>2: 0`.

### Task 4 — Local gates, first pass

**Files:** none changed.

Run D14's list up to and not including `task check` whole.

**Done when:** each step exits 0 and prints its own completion line where it has
one (`check:comments: ... clean`, `check:doc-owner: ... every doc comment sits
on its own function`, `check:requirements-chain: ... every citation resolves`).
`check:comments` at this point may refuse the four identity files for having
fallen more than the band under their ceilings; that finding is expected here
and is cleared by Task 7, not by any other means.

### Task 5 — SPEC-009, if D7 applies

**Files:** `docs/specifications/009-identity-and-joining.md`.

Each sentence confirmed in Task 1 goes into "How it works", in the paragraph
whose subject it is, in the present tense, naming the symbol. Nothing else in
the record changes.

**Done when:** `git diff docs/specifications/009-identity-and-joining.md` shows
only added or extended sentences in "How it works", each of which Task 1's notes
tie to a symbol; `task check:requirements-chain` is green. If Task 1 confirmed
none, the file is unchanged and the report says so.

### Task 6 — Phase 4b, the reading review

**Files:** whatever its findings touch among the above.

Per D16. Findings are fixed and the affected Task's "done" is re-run. The
review settles before Task 7 starts.

**Done when:** the review record names every surviving block's kind and every
deleted block's outcome, and reports no open finding; the Task 0 comparison
prints `same` for all five files.

### Task 7 — The ledger

**Files:** `tools/comment-ceilings.txt`, by `--write-ledger` only.

Per D12.

**Done when:** `git diff tools/comment-ceilings.txt` shows exactly the four
identity rows, each lowered; `task check:comments` ends `clean`; `--report |
grep internal/identity/` prints `banned   0` and `blocks>6   0` on all five lines
and a ceiling equal to each measured share rounded up to a tenth.

### Task 8 — The break and the whole gate

**Files:** none in the repository.

D13 in a scratch clone; then `task check` whole, once, per D14.

**Done when:** the clone's clean run exits 0 with the completion line, and the
broken run exits 1 with the one finding D13 names; `task check` exits 0 with
every step, `check:comments` and `check:mutation` among them, printing its own
verdict.

### Task 9 — Commit, then the report

**Files:** the commit's, per D10; then `docs/reports/2026-09-24-sweep-identity.md`.

The sweep commit's message lists the four ledger rows old and new, the token
counts, and D13's finding line verbatim. After it: the report per D17 and the
`implementation-report` skill, in its own commit.

**Done when:** `git show --stat HEAD~1` (the sweep) lists the ticket, the plan,
the four identity files, the ledger, and SPEC-009 only if Task 5 changed it;
`git diff --stat e872467 -- docs/reports/` lists only the new report; `git diff
--quiet e872467 -- internal/identity/qa_joining_internal_test.go` exits 0.

## Commits

| Commit | Carries | Gate steps it runs |
|---|---|---|
| C1 | the ticket, this plan, the four files, the ledger, SPEC-009 if Task 5 changed it; the message records the ledger rows, the token counts and the break's finding line | D14's list by hand, `task check` whole (Task 8), then the pre-commit hook (lint, vet, tier-1, arch, vocabulary, doc-owner, secrets, typecheck, review gate) |
| C2 | the implementation report | pre-commit hook |

Push after C2: pre-push runs tiers 2 and 3 and the contract gates, about three
minutes; let it finish. No Go code changes, so no mutation key moves and
`check:drift` has no client change to compare.

## Gaps that travel with this plan

1. **The ticket's 253 lines.** `identity_test.go`'s doc blocks above tests hold
   225 lines by the adjacent definition and 255 by the gate's; 253 is neither.
   The "done" observation (the counter prints 0) does not depend on it. The
   report states the measured figures.
2. **Item 4's "five rows ... are lowered".** `qa_joining_internal_test.go` is
   not touched, so `--write-ledger` cannot lower its row: four rows move and one
   stays at 12.7 (D12). The plan plans to the four.
3. **The problem's "found two of those paragraphs false".** The joining report
   says two false SENTENCES, both corrected before `9920a67`. Nothing done
   depends on it.
4. **`TestVerifyUsesConstantTimeCompare` reads `identity.go` as text,
   comments included.** It passes while the file contains
   `subtle.ConstantTimeCompare` anywhere: today in `Verify`'s doc, in `Verify`'s
   code and in `JoinAdmits`' code. So a comment can satisfy it, and removing the
   call from `Verify` alone leaves it green because `JoinAdmits` still carries
   one. The sweep may remove the doc mention and the test stays green on code;
   the weakness is not introduced here and fixing it changes a code line, which
   the ticket forbids. It goes to the report, and is proposed for
   `docs/verification-debt.md` (Q6).
5. **SQL comments inside the `schema` literal.** Fifteen `--` lines describe
   behaviour in paragraphs; neither the gate nor SPEC-010 reaches them, and the
   ticket's no-code-change rule keeps them (D8). A later ticket decides whether
   SPEC-010 extends to them (Q7).
6. **Trailing comments are invisible to the gate.** They are read by the sweep
   and by Phase 4b; nothing holds them afterwards.
7. **Item 2 is an exit state, not a standing rule.** After the sweep,
   `check:comments` allows a block of up to six lines above a test, so nothing
   keeps the "id line plus one line" shape. The verifier agrees the ticket adds
   no rule; holding that shape would be a new rule, and a different ticket.
8. **The 20.0 bound is a model.** The projection is about 16; if the reading
   keeps more true warnings than that, Q5 applies.
9. **The pointer from `internal/gateway/server_test.go`** into `identity.go`'s
   `busy_timeout` warning holds only while D8 keeps that warning.

## Questions for sign-off

1. **May the sweep allocate rows for rules it finds in test doc blocks?**
   Recommend no (D6): record each candidate in the report with its test and its
   observation; a later ticket sorts them with the `requirements` skill.
2. **Do facts only a comment held go into SPEC-009 in the same commit?**
   Recommend yes (D7), each sentence checked against a named symbol and re-read
   by Phase 4b, the five candidates listed as the starting point.
3. **Unexported symbols lose their descriptive doc sentences (D2)?** Recommend
   yes; rule 10 allows the doc sentence of an exported symbol only, and a
   warning on an unexported symbol stays.
4. **One commit for the sweep, the report separately (D10)?** Recommend yes.
5. **If `identity.go` lands above 20.0 with every surviving line a true
   warning, pointer or doc sentence?** Recommend the reading governs: stop,
   report the figure and the lines, and let the ticket's writer amend item 3,
   rather than cut a true warning to meet a number.
6. **Record gap 4 (`TestVerifyUsesConstantTimeCompare` passes on text a comment
   can supply) in `docs/verification-debt.md`?** Recommend yes, as a known
   coverage gap in C2 beside the report, with the fix left to the next ticket
   that touches the package's tests.
7. **SQL comments inside the `schema` literal (gap 5).** Recommend leaving them,
   as the ticket requires, and raising whether SPEC-010 reaches SQL in string
   literals as its own question, not decided inside a sweep.
8. **A doc sentence that cannot fit one line (D4).** Recommend one sentence,
   one line where it fits, a second physical line only where cutting would lose
   a fact, each case named by Phase 4b.
