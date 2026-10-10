# Plan: a mutation adjudication is keyed by the code it excuses

Ticket: `docs/superpowers/specs/2026-10-10-a-mutation-adjudication-is-keyed-by-its-code-design.md`.
Verified 2026-10-10 on the tree at `ca14522` by an agent that did not write the
ticket. Verdict: **Passes with gaps**. The gaps and the questions for Patrik come
before the decisions, because Q3 and Q4 change what two tasks contain.

Every number below is a measurement at `ca14522` unless it names another commit.
None is a target.

## How the ticket was verified

| Check | How settled | Result |
|---|---|---|
| 1. Every path resolves | command | Every file, function and test the ticket names exists where it says. `read_position` returns the trimmed line as `anchor` in both gates. The quoted header phrases and the "Re-pointed 2026-08-23 from 293:15 to 324:15" account are both in `tools/mutation-equivalents.txt`. The entry counts are 36 and 67. Two Go entries share one `expr.go` line, with one mutator and two columns. Twelve TS lines carry more than one entry. In each file exactly one keyed line's trimmed text occurs again in its source file. |
| 2. Done is an observation | command | Done 1 fails today as claimed. In a scratch copy with one comment line added at the top of `internal/engine/apply.go` and `client/src/fold.ts`, both self-tests fail, at `test_every_real_adjudication_still_points_at_its_mutant` and nowhere else. The four tests Done 4 names exist and pass today: 107 and 50 tests, all OK. Done 5 is shown to be reachable: a prototype of decisions D1–D4 claims 36 of 36 and 67 of 67, the same survivors the old keys claim, with an empty difference (Appendix A). Done 6 is a reading, which is right for prose. See G4 for Done 2 and 3. |
| 3. Each rule is breakable | reading | Each of the three rules has an observation that would fail: Done 2's test, Done 3's test, and Done 4's copy and two-mutant bullets. ADR-010's decision 3 (a stale entry fails) is not among the rules (G2). |
| 4. Scope matches the claim | command, then reading | Nothing outside the two gates and their tests calls any function this changes. `Taskfile.yml`'s `check:mutation` and `check:ts-mutation` call the gates without depending on the format, so they do not change. `tools/ts_mutation_inputs_test.py` writes an old-format line only as bytes to hash. Product comments cite entries by name, not by key. The ticket's file list omits `docs/requirements.md` (G3). Inside the listed files, the test churn is larger than the list suggests: 64 Go fixtures and 42 TS fixture keys are written in the old format. |
| 5. No decision contradicted silently | reading | The ticket names what it overturns: the `THESE ENTRIES ARE KEYED BY LINE:COLUMN` block in `tools/mutation-equivalents.txt`. CLAUDE.md rule 8 already records the re-keying as agreed. ADR-010's decisions 2–4 (zero unadjudicated survivors, a stale entry fails, a reason is required) are kept, not overturned. The TS header's "Entries are keyed by file, line:col AND mutator" paragraph is overturned too. The ticket does not name it, but Done 6 rewrites it. |
| 6. Records moved are named and reachable | command, then reading | The section says `None.` A search of `docs/specifications/` for mutation, gremlins, Stryker and equivalent finds nothing, so no specification describes either gate and `None.` is accurate. Whether a new one should exist is the ticket's own open question (Q2). |

## Gaps

- **G1. Done 5's "no entry holds a line number" is ambiguous.** "Entry" could mean the key or the whole entry. Reasons hold coordinates: 25 of 36 Go reasons and 44 of 67 TS reasons contain a `line:col`, a `file.go:N` or a `line N`. So do 36 comment lines of the TS header and 1 of the Go header. If "entry" includes its reason, Done 5 cannot pass unless Q3 is answered "same change". *Fix:* the writer says which is meant, or Patrik's answer to Q3 settles it.
- **G2. Staleness is not stated as a rule.** ADR-010 decision 3 says an entry whose mutant no longer survives fails the gate. Done 4's heading covers this only implicitly. The plan keeps `test_stale_entry_fails` and `test_an_adjudication_for_a_mutant_that_no_longer_survives_fails` and gives the property a requirement id (Task 1). *Fix:* a fourth rule in the ticket, or acceptance of this note.
- **G3. "What it touches" omits `docs/requirements.md`.** The ticket's rules become requirements cited by the new tests, as tools tests already do (`# VTT-050` in `tools/check_comments_test.py`). It also omits the implementation report under `docs/reports/`, and a new specification if Q2 is answered yes. *Fix:* the writer adds them.
- **G4. Done 2 and 3 "fail on today's tree" only vacuously.** A test written in the new format fails today because today's parser refuses the entry, not because the property is missing. The plan does not rely on that. The evidence that each test bites is its deliberate break (the break tables, items B1, B2, T1 and T2).

## Questions for Patrik (sign-off, not decided here)

- **Q1. One ticket or two?** *Recommendation: one.* Done 5, Done 6 and the rule 8 edit are true only once both gates have landed. Two tickets would leave CLAUDE.md for a period in which the exception covers one file but not the other. The gates share no code, so they stay separate tasks and separate commits on one branch.
- **Q2. Does the decision get a specification?** *Recommendation: yes,* as `New:` in the next free number. The project's rule for `docs/specifications/` is "a decision somebody would otherwise re-open", and this one was a recorded design that is now being re-opened. A gate already has a specification: SPEC-010 describes `check:comments`. If yes, the ticket's `Specifications this moves` becomes `New: …`. Done 6's "each header states how an entry is matched" should then read "each header gives the format and points at the specification", or the rule lives in two places. That edit is the writer's to make.
- **Q3. Are the re-point accounts in reasons cut in this change?** *Recommendation: on this branch, in a commit of its own after the conversion,* chosen by reading and checked by command (Task 9). The conversion commit carries every reason byte for byte, which keeps Done 5 provable by command. The cut commit only removes whole sentences, and a script shows that. Whatever the answer, D13's seven present-tense sentences go in the conversion commit, because the conversion makes them false.
- **Q4. Keep the `copies` field (D4)?** *Recommendation: keep it.* It is the one addition the ticket did not foresee. Without it, one case lets a code key claim a survivor nobody judged (R1, under D4). The alternative is to accept R1 and record it in `docs/verification-debt.md` as a known gap.

## Patrik's answers at sign-off, 2026-10-10

- **Q1:** one ticket. Go and TypeScript stay separate tasks and separate commits on one branch.
- **Q2:** yes, a new specification, SPEC-020. Task 10 runs, and the headers give the format and point at it.
- **Q3:** the re-point accounts are cut on this branch, in a commit of their own after the conversion. Task 9 runs.
- **Q4:** `copies` is kept (D4). R1 is closed.

The ticket's writer revised it after sign-off, for G1 to G4 and these answers:
- Done 5 now says no KEY holds a line number (G1).
- A new Done 6 holds the cut of the re-point accounts (Q3), so the old Done 6 and 7 are now Done 7 and 8. Done 7 names the new specification (Q2).
- Two rules were added: staleness (G2) and `copies` (Q4).
- What it touches names `docs/requirements.md`, the specification and the report (G3).
- Done 2 and 3 now name, as what fails today, today's gates given the same case in today's format (G4).

"How the ticket was verified", "Gaps" and "Questions for Patrik" above refer to the ticket as it was verified. Everything from "Evidence on disk for Done 5" on uses the revised numbering.

## The sort, after sign-off (Task 1)

Seven rules were accepted, and the dispenser allocated an id to each. Each row reads `**OPEN — no test yet**` until the tests that cite it land.

| Id | Rule | The observation that goes red |
|---|---|---|
| VTT-298 | A surviving mutant is claimed by the adjudication that names its file, its mutator, for TypeScript its replacement, the code it sits on, and which copy of that code it is, counted from the top (for TypeScript, how much code it covers), wherever in its file that code stands and however it is indented. | `test_a_survivor_whose_line_moved_is_still_claimed`, `test_a_reindented_line_is_still_claimed` (B1, B3, T1, T3) |
| VTT-299 | A surviving mutant is not claimed by an adjudication whose code it does not sit on, whatever position the adjudication once named. | `test_a_survivor_on_other_code_at_an_entrys_old_position_is_not_claimed`, `test_a_conditional_expression_entry_does_not_claim_other_code`, `test_two_mutants_of_one_mutator_on_one_line_are_two_claims` (B2, B4, T2, T4) |
| VTT-300 | One adjudication claims at most one surviving mutant. | `test_one_entry_claims_one_of_two_survivors_on_its_key` (B7, T7); `test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated` moved to VTT-299 after D15 put `nth` in the key |
| VTT-301 | An adjudication that claims no surviving or timed-out mutant fails the mutation gate. | `test_stale_entry_fails`, `test_an_adjudication_for_a_mutant_that_no_longer_survives_fails`, `test_two_entries_for_one_code_and_one_survivor_leave_one_stale` (B10, T10) |
| VTT-302 | An adjudication whose code stands in its file a number of times other than the number it records is refused before any mutation run. | the two `copies` tests and `test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported` (B14, B16, T14, T16) |
| VTT-303 | A surviving mutant is matched against the source its run measured, not the tree as it stands when the run ends. | `test_a_survivor_is_resolved_against_the_source_the_run_measured`, `test_a_survivor_is_resolved_against_the_reports_own_source` (B11, T22) |
| VTT-304 | A surviving mutant whose code cannot be read fails the mutation gate. | `test_a_survivor_whose_line_cannot_be_read_fails_the_gate`, `test_a_report_file_without_its_source_fails_the_gate`, `test_a_survivor_whose_line_is_not_in_the_source_fails_the_gate` (B13, T23) |

Rejected, one line each:
- **Done 1, a comment line inserted leaves the self-tests green.** It is an observation of VTT-298, not a rule of its own.
- **No key holds a line number.** That is how VTT-298 is achieved.
- **Done 5, the same survivors are claimed before and after.** It is a check made once, at this ticket's base, and not a standing rule.
- **Done 6 and Done 7.** Both are prose, held by Phase 4b.
- **An entry whose code stands nowhere is reported.** VTT-302 covers it, since zero is not the recorded count, and so does VTT-301 at run time.
- **Two mutants of one mutator on one line are two claims.** This is VTT-299: the code a mutant sits on includes its place in the line.
- **Survivors are matched before timeouts (D3).** This is ordering inside VTT-300 and VTT-301. The wrong order fails loudly, not silently.
- **The static comment, token and unknown-mutator rules (D6).** These are early warnings. A wrong entry is still caught at run time by VTT-299 and VTT-301. Their tests stay, without an id.
- **The parse refusals (D5).** These are validation of the file format. Their tests stay, without an id.
- **The ready-to-paste entry in a message (D8).** This is message design. Nothing breaks at the table if it changes.
- **The skip cache is unchanged (D9).** That is how, not a rule.

Deviation from Task 1's list: it named the refusal of an entry whose code stands nowhere, which is folded into VTT-302 here. It did not name D2's two properties, which are VTT-303 and VTT-304, because a gate that claims against the wrong source, or skips a line it cannot read, can pass a survivor nobody judged.

## Rule 9: how MapTool does this

MapTool does not solve this. `~/dev/RPTool` (last commit `63862e4`) has no mutation-testing configuration: a search of its Gradle, Maven, TOML and properties files for pitest, stryker and gremlins finds nothing. No tabletop function is designed here, so there is no geometry or vocabulary to borrow. That confirms the ticket's own statement.

## Evidence on disk for Done 5

**Exists, and is current for `ca14522`:**
- `reports/mutation-skip-cache.json` (gitignored, machine-local). It holds all 14 gated packages: 36 survivors and 6 timed-out mutants (`internal/sight` 4, `internal/rules` 1, `internal/mcp` 1). `package_fingerprint` recomputed for every package matches the stored fingerprint, 14 of 14. So these are positions in the `ca14522` tree, and they are exactly what the gate's next run reuses.
- `reports/mutation/mutation.json`. This is Stryker's report, with each file's `source` (22 files). It holds 2950 mutants: 2868 Killed, 67 Survived, 9 Timeout, 6 Ignored. `reports/mutation/ts-inputs.sha256` equals `python3 tools/ts-mutation-inputs.py` on the tree, and every file's `source` is byte-equal to the tree's.
- `tools/` is in no gated package's `-test` closure (checked with `go list -deps -test`). Editing the gates therefore leaves every fingerprint unchanged, and the final `task check` reuses both stores. That means Done 8's run checks the new gate against the very survivors Done 5's command uses.

**Does not exist:** any committed record of a run's survivors. Both stores are gitignored and are overwritten by the next run that measures. gremlins' raw output is not kept: `run_package` parses it and discards it. Nothing on disk records a run older than these two stores.

**Consequence:** Task 0 freezes both stores before anything else and records their sha256 in the implementation report. Done 5 needs no mutation run beyond the final `task check`. If Task 0 finds either store no longer current, the evidence for Done 5 is gone. Then the only way to get it is a fresh measuring run at `ca14522` (`VTT_MUTATION_NO_SKIP=1`, and the Stryker container), and that is Patrik's call, not the implementer's.

## The constraints

These bind every task. They are named here, not restated.

- **CLAUDE.md rule 2.** This plan is the reviewed decision that changes two gates. Nothing that fails today may pass afterwards except as D1–D4 state, and Done 4 and Done 5 hold that. `task check` is run whole at the end. Most of the gates it holds run at no other time.
- **CLAUDE.md rule 8.** No stored key holds a line number. The plan, the report and the specification cite by name. A position the gate prints in a message is output, not stored prose.
- **CLAUDE.md rule 10** does not reach `tools/`. No comment under `internal/`, `cmd/` or `client/src` is touched, and the final diff shows that.
- **ADR-010, decisions 2–4:** zero unadjudicated survivors, a stale entry fails, and every entry has a reason. All three stay.
- **Patrik's rulings:** a timed-out mutant counts as detected and is still printed (2026-07-28); delete the old solution first; rank by what breaks; equivalence is a claim about specific code; every number is a measurement.
- **SPEC-008:** requirement ids come only from `requirement-id`. A row in `docs/requirements.md` names tests that carry its id, and lands in the same commit as those tests.
- **Operational:**
  - No mutation run beyond the final `task check`.
  - Never edit the tree while a gate runs.
  - Every deliberate break is made in a scratch copy, never in the working tree.
  - A conversion script asserts before it writes, and its result is grepped before anything is recorded or committed.
  - A header edit replaces only the paragraphs named in D12. No file is rewrapped wholesale.
  - Review packages are built with `git diff HEAD`.
  - The lefthook `review-gate` needs a review record for each commit.

## Decisions

**D1. What a key is.**
- Go: `(package, file, MUTATOR, offset, code)`.
- TypeScript: `(file, mutator, replacement, offset, code)`.
- `code` is the source line where the mutant starts. The line terminator is dropped, leading and trailing spaces, tabs and CRs are removed, and the interior is kept byte for byte.
- `offset` is the mutant's 1-based column within `code`. It is the tool's column minus the width of the removed indentation. gremlins counts in bytes and Stryker in UTF-16 units; indentation is ASCII, so the two widths agree.

How it separates the ticket's two collision cases:
- **Entries that share a line** differ in `offset`, and in TS also in `replacement`.
- **A keyed line whose text recurs in its file** gives identical keys. D3 and D4 handle that case.

Re-indenting a line leaves its key unchanged, so it is still claimed. Changing whitespace inside the line, or editing a trailing comment on it, changes the key and forces a re-judgment. That over-forces, in the safe direction. At `ca14522` one keyed line has a trailing comment (the `fold.ts` line `if (res.max > 0 && computed > res.max) …`), and no keyed line has a run of interior whitespace.

*Forced by:*
- gremlins gives a mutant only as `{type, status, line, column}`, with no replacement and no end. The code therefore has to be read from source.
- Without `offset`, 1 Go and 6 TS entries would collide at `ca14522`. The Go case is the `expr.go` line `if a%b != 0 && (a < 0) != (b < 0) {`, which carries two CONDITIONALS_BOUNDARY entries. The TS cases include the two `StringLiteral ""` entries on `grid.ts`'s `initial:` line.
- The offset is relative to the trimmed line because re-indenting is a move, and moves must not force a re-judgment.
- Stryker's start-to-end span was considered as the TS `code`, and rejected. Go has no span, so the two files would mean different things by "code". A span such as `""` recurs throughout a file and has lost its line.

The residual, **R3**: a TS mutant on a node spanning several lines is keyed by the node's first line only.

**D2. A survivor's code is read from the source the run measured.**
- TS reads it from the report's own `files[path].source`.
- Go reads it from the package's `.go` files, read immediately before gremlins is invoked for that package. For a reused verdict, the tree is read at the moment its fingerprint matched.
- A survivor whose file or line cannot be read fails the gate (fail closed). A report file with no `source` fails it too.

*Forced by:* positions name the source the tool mutated. The TS checker runs every time against a stored report. A gremlins run takes minutes, during which the tree has been edited before in this repository. Resolving after the run could give a survivor the code of whatever line now stands at its position. Usually that leaves it unclaimed and loud, but it can also hand it to an entry nobody wrote for it.

**D3. Matching counts.** Take the entries, survivors and timed-out mutants that share one key. Then:
- `min(entries, survivors)` survivors are claimed.
- Any survivors left over are unadjudicated.
- Entries left over claim up to that many timed-out mutants.
- Any entries still left over are stale.

When there are more survivors than entries, the claimed ones are chosen in position order. The message then names every position that shares the key and how many entries name it.

Matching runs per package (the key holds the package), so the skip cache's per-package write condition does not change. Two identical entries are legal, so the TS parser's "listed twice" refusal goes. An accidental duplicate is still refused: statically by D4 and at run time as stale.

*Forced by:*
- The rule "one entry claims at most one survivor".
- Done 4's copy bullet.
- Precedent: the TS header records two identical `history.replaceState` statements that each carried their own entry.
- Survivors are matched before timeouts because a timeout is not an evaluation (2026-07-28).

**D4. Each entry records `copies`,** the number of lines in its file whose code equals its own. The static check (D6) refuses an entry when the tree's count differs, and refuses more entries for one key than its `copies`.

*Forced by* **R1**, the one way a code key could claim a survivor nobody judged. The adjudicated copy is deleted and an identical line elsewhere in the same file starts surviving, both in one change. Under position keys that fails, by accident. Under D3 alone it would pass in silence.

Exposure at `ca14522` is two entries, one per file, on code that stands twice in its file:
- `internal/rules/expr.go`'s `if p.depth > maxExprDepth {`
- `client/src/fold.ts`'s `const list = st.Conditions[v.actorId] ?? [];`

The cost is that an entry is re-judged whenever a line identical to its code is added to or removed from its file. For example, a second `default:` in `fold.ts` would flag the `ConditionalExpression default:` entry. Subject to Q4.

**D5. File format.**

```
Go:  <package>  <file>  <MUTATOR>  offset=<n>  copies=<n>
         code: <code>
         <reason>
TS:  <file>  <MUTATOR>  offset=<n>  copies=<n>  <replacement, to end of line>
         code: <code>
         <reason>
```

- A comment is a line whose first character is `#`. A `#` anywhere else is text.
- An entry without its `code:` line, or without its reason, is a parse error.
- A Go package must be written as survivors carry it (`internal/x`, not `./internal/x/`), or parsing refuses it and names the spelling to use.

*Forced by:*
- Code and the TS replacement are both free text containing spaces, so code gets its own line.
- The TS parser drops everything after a `#`. That would cut code, and today it already silently cuts two reasons (measured).
- `offset` and `copies` are labelled so that the two numbers cannot be swapped silently.
- A misspelled package today matches nothing. `stale_entry_hint` diagnoses that only after a full mutation run. Refusing at parse finds it in milliseconds, and the hint branch can be deleted.

**D6. A static check replaces `suspect_positions`.** It runs before the mutation run and, like today, it is reported and carried rather than returned on. It refuses an entry when:
- its file cannot be read;
- its code stands in the file a number of times other than `copies`;
- more entries share its key than its `copies`;
- its code starts like a comment (`COMMENT_PREFIXES`);
- (Go) the code at `offset` is not a token its mutator applies to (`MUTATOR_TOKENS`, `token_at`);
- (TS) `operator_at`, the string-opener rule or `NO_POSITION_RULE` fails on the code at `offset` (via `byte_offset`);
- its mutator is unknown (fail closed).

Every rule is applied to the entry's own code at its own offset. `read_position`'s anchor half goes.

`test_every_real_adjudication_still_points_at_its_mutant` keeps its name and asserts that this check, run over the real file and the real tree, finds nothing.

*Forced by:*
- Done 4's last bullet.
- Done 1, which names that test.
- The comment and token rules still guard an entry written by hand.

A prototype of this check finds no fault in any of the 103 converted entries: none on the `ca14522` tree, and none with Done 1's comment line inserted.

**D7. Pairing is deleted.**
- From `check-mutation.py`: `moved_entries`, `pair_signature`, `_by_position`, `line_col`, `location_file`, `stale_entry_hint` (both branches) and the `ADJUDICATION MOVED` message.
- From `check-ts-mutation.py`: `pair_moves`, `_pair`, `_by_position` and its `ADJUDICATION MOVED` message.
- The long comments explaining all of these go with them.

*Forced by:*
- A mutant that moved is now claimed, so there is nothing to pair.
- The hint's same-file branch existed only for the shift case.
- D5 replaces the hint's spelling branch.
- The ruling to delete the old solution first.

**D8. Messages.**
- An unadjudicated survivor is printed with the entry header and `code:` line ready to paste, `copies` included.
- A stale entry says no mutant on its code survives. If the code was edited, it says to re-judge the reason against the new code and re-key it.
- Substrings that kept tests assert stay unchanged: Go `not adjudicated` and `no longer survives`; TS `SURVIVED <mutator> at <path>:<line:col>`, `no longer survives` and `-> false`.

*Forced by:*
- Offsets worked out by hand are how keys went wrong (the reasons record re-points that claimed to have re-resolved what they had not).
- Done 4 keeps the assertions of the tests it names.

**D9. The skip cache does not change.** It stores gremlins' positions, and at reuse they are resolved to code against the tree whose fingerprint matched (D2).

*Forced by:*
- The fingerprint covers every file in the package's `-test` closure, its own directory included, so a stored position names the same bytes now.
- Neither gate script nor equivalents file is in the fingerprint, so the cache the old gate wrote stays valid. Measured: 14 of 14 fingerprints match, and `tools/` is in no closure.

**D10. Matching is one pure function per gate.** `claims(entries, live)` returns the claimed live mutants, identified as the tool reported them: position, mutator and, for TS, replacement. `run()` and `check()` call it, and Done 5's command imports it.

*Forced by:* Done 5 must exercise the gate's own matching. A re-implementation would prove only itself.

**D11. The conversion is mechanical.** A one-shot script reads `ca14522`'s entries with `ca14522`'s parser and reads each entry's line in the tree. It writes the D5 header and `code:` line, and carries the reason line byte for byte (except D13). Before writing, it asserts that each entry resolves to exactly one line. The script is kept verbatim in the implementation report, not committed as a tool.

*Forced by:*
- The ruling that each file is converted in the same change as its gate.
- The repository's practice that a script which fails must stop the chain before anything is recorded.

**D12. Header paragraphs replaced in the conversion commit.** These are the paragraphs that state the format or the matching, or that tell a reader to re-point. They become false the moment D1 lands.
- Go: the `Format:` lines, and the block from `THESE ENTRIES ARE KEYED BY LINE:COLUMN` to the rule after it.
- TS:
  - the `Format:` paragraph;
  - the paragraph beginning "Entries are keyed by file, line:col AND mutator";
  - the instruction paragraph beginning "WRITE THAT CHECK CAREFULLY IF YOU RE-RUN IT";
  - the sentence beginning "Anyone repeating that comparison should pair the entries POSITIONALLY";
  - the instruction ending "grep this header for its old line before you stop".

Dated history stays, pending Q3.

**D13. Seven present-tense sentences inside reasons are cut in the conversion commit, whatever Q3's answer.**
- Four Go reasons: "A comment or a var added ANYWHERE above a key moves it, in BOTH adjudication files, and neither pre-commit hook looks."
- One Go reason: "Re-keying an entry is not done until every file:line INSIDE its body has been rendered against the tree, not just the key at the top."
- Two TS reasons (the `view/dm.ts` comparator pair): the sentence beginning "A COMMENT-ONLY EDIT MOVES EVERY KEY BELOW IT".

*Forced by:* each becomes false when D1 lands, and false prose is this repository's dominant defect. Each is cut as a whole sentence and listed in the report.

**D14. Order.** The gates do not depend on each other. Go goes first, as a preference: it has more fixtures and it has the skip cache. CLAUDE.md is edited last, because Done 7 is true only once both gates have landed.

### Residual risks, named rather than fixed

- **R1** is closed by D4. If Q4 drops D4, R1 is open.
- **R2.** A reason whose premise lives outside the keyed line is not re-judged when that other code is edited. Position keys caught this only by accident, when the edit changed the number of lines above the key. The ruling holds that only an edit to the code itself forces a re-judgment.
- **R3.** See D1.

## Tests

Kept tests keep their assertions, and their fixtures are rewritten into the D5 format with the same code and offset. Each test in "deleted" names the property it held and where that property is held now.

### `tools/check_mutation_test.py`

- **Kept, fixture converted.** Every `MutationGateTest` and `SkipCacheTest` test not listed below; `test_adjudication_is_matched_on_all_three_fields` and `test_survivor_in_a_different_package_is_not_pre_approved` (Done 4); `test_stale_entry_fails` (G2); and `test_a_survivor_on_a_different_statement_is_not_paired` (Done 4, moved to `MutationGateTest` when its class goes).
- **Converted into the static check (D6):**
  - the three comment tests;
  - `test_a_key_whose_column_holds_a_token_the_mutator_cannot_apply_to` and its control `test_the_same_column_is_accepted_for_the_mutator_that_does_live_there`;
  - `test_a_key_one_column_off_is_rejected` (offset one off);
  - `test_a_sound_key_is_accepted`;
  - `test_a_mutator_with_no_rule_is_fatal_rather_than_silently_skipped`;
  - `test_the_gate_itself_fails_and_names_the_key`;
  - `test_a_suspect_key_is_not_also_told_to_delete_itself`;
  - `test_a_sound_file_of_keys_does_not_red_the_gate`;
  - `test_every_real_adjudication_still_points_at_its_mutant`.

  Two are renamed because their names would become false:
  - `test_the_column_is_a_byte_offset_so_a_tab_counts_as_one` becomes `test_a_tab_indented_survivor_resolves_to_its_offset_in_the_trimmed_line`;
  - `test_a_key_naming_a_line_that_is_not_there_is_rejected` becomes `test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported` (Done 4).
- **Deleted:**
  - `test_a_timeout_at_a_moved_entrys_old_key_costs_the_move_pairing`: the cost of pairing, which no longer exists.
  - The four hint tests, `test_stale_entry_hints_at_a_same_file_survivor`, `test_the_hint_names_every_candidate`, `test_no_hint_when_nothing_could_have_moved` and `test_no_hint_across_packages`: the hint goes (D7).
  - `test_the_hint_does_not_excuse_anything`: its two assertions are a subset of `test_a_survivor_on_a_different_statement_is_not_paired`'s.
  - `test_a_package_written_differently_is_diagnosed_as_such`: replaced by the parse refusal.
  - `test_a_move_the_source_can_prove_is_reported_as_a_move`: replaced by Done 2's test.
  - `test_a_move_is_never_advice_to_delete`: there is no move message any more.
- **Converted with a new name:**
  - `test_two_entries_and_two_survivors_pair_by_position_order` becomes `test_two_entries_on_two_identical_lines_claim_both`.
  - `test_mismatched_counts_still_refuse_to_guess` becomes `test_two_entries_for_one_code_and_one_survivor_leave_one_stale`.
- **New:**
  - `test_a_survivor_whose_line_moved_is_still_claimed` (Done 2: two runs, the second after a line is inserted at the top);
  - `test_a_survivor_on_other_code_at_an_entrys_old_position_is_not_claimed` (Done 3);
  - `test_a_reindented_line_is_still_claimed`;
  - `test_two_mutants_of_one_mutator_on_one_line_are_two_claims` (Done 4);
  - `test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated` (Done 4);
  - `test_an_entry_claims_a_survivor_before_a_timed_out_copy`;
  - `test_a_survivor_is_resolved_against_the_source_the_run_measured` (the fake runner edits the tree);
  - `test_a_survivor_whose_line_cannot_be_read_fails_the_gate`;
  - `test_an_entry_whose_copies_differ_from_the_tree_is_reported`;
  - `test_more_entries_than_copies_is_reported`;
  - `test_an_entry_without_a_code_line_is_fatal`;
  - `test_a_hash_inside_code_is_not_a_comment`;
  - `test_a_package_not_written_as_survivors_carry_it_is_refused`;
  - `test_an_unadjudicated_survivor_is_printed_as_a_ready_entry`.

### `tools/check_ts_mutation_test.py`

`report()` gains each file's `source`, built from `TS_LINES`.

- **Kept, fixture converted.** Every `Gate` test, including `test_an_adjudication_is_keyed_by_mutator_not_just_location`, `test_two_mutants_at_one_location_are_two_separate_claims` (both Done 4) and `test_an_adjudication_for_a_mutant_that_no_longer_survives_fails` (G2). Also the `Equivalents` parse tests, except one: `test_a_duplicate_entry_is_an_error` keeps its fixture (an entry listed twice for code that stands once), but its refusal now comes from the static check rather than the parser.
- **Converted into the static check:** every `KeyPosition` test. `test_a_column_is_counted_the_way_stryker_counts_it` keeps its UTF-16 assertions, now applied to the offset within the code.
- **Converted with a new name:**
  - `test_a_moved_adjudication_is_reported_as_a_move_with_its_new_key` becomes `test_a_survivor_of_the_entrys_mutator_on_another_statement_is_not_claimed` (Done 4). Its fixture already puts entry and survivor on different code.
  - `test_a_different_replacement_is_not_a_move` becomes `test_a_survivor_with_another_replacement_at_the_entrys_code_is_not_claimed`.
  - `test_a_move_across_files_is_not_a_move` becomes `test_the_same_code_in_another_file_is_not_claimed`.
  - `test_two_candidates_at_once_are_not_paired` becomes `test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated` (Done 4).
  - `test_two_entries_and_two_survivors_pair_by_position_order` becomes `test_two_entries_on_two_identical_lines_claim_both`.
  - `test_mismatched_counts_still_refuse_to_guess` becomes `test_two_entries_for_one_code_and_one_survivor_leave_one_stale`.
  - `test_the_anchor_separates_two_statements_sharing_a_replacement` becomes `test_a_string_literal_entry_claims_only_its_own_code`.
- **Deleted:**
  - `test_a_move_still_fails_the_gate` and `test_a_pairing_is_never_advice_to_delete`: there is no move any more.
  - `test_an_anchorless_ordered_pairing_does_not_claim_a_shared_source_line`: the pairing message goes.
  - `test_a_timeout_at_a_moved_entrys_old_key_costs_the_move_pairing`: the cost of pairing.
- **New:**
  - `test_a_survivor_whose_line_moved_is_still_claimed` (Done 2);
  - `test_a_conditional_expression_entry_does_not_claim_other_code` (Done 3);
  - `test_a_reindented_line_is_still_claimed`;
  - `test_an_entry_claims_a_survivor_before_a_timed_out_copy`;
  - `test_a_survivor_is_resolved_against_the_reports_own_source` (the tree differs from the report);
  - `test_a_report_file_without_its_source_fails_the_gate`;
  - `test_a_survivor_whose_line_is_not_in_the_source_fails_the_gate`;
  - `test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported` (Done 4);
  - the two `copies` tests;
  - `test_an_entry_without_a_code_line_is_fatal`;
  - `test_a_hash_in_a_replacement_or_reason_is_kept`;
  - `test_an_unadjudicated_survivor_is_printed_as_a_ready_entry`.

### Breaks: one per check relied on, made in a scratch copy, each named test must fail

| # | Check (gate) | Deliberate break | Must fail |
|---|---|---|---|
| B1 | position-free match (Go) | add the survivor's line number to its key | `test_a_survivor_whose_line_moved_is_still_claimed` |
| B2 | code in the key (Go) | match on package, file, mutator and offset only | `test_a_survivor_on_other_code_at_an_entrys_old_position_is_not_claimed`, `test_a_survivor_on_a_different_statement_is_not_paired` |
| B3 | offset relative to the trimmed line (Go) | use the raw column | `test_a_reindented_line_is_still_claimed`, the tab test |
| B4 | offset in the key (Go) | drop it | `test_two_mutants_of_one_mutator_on_one_line_are_two_claims` |
| B5 | mutator in the key (Go) | drop it | `test_adjudication_is_matched_on_all_three_fields` |
| B6 | package in the key (Go) | drop it | `test_survivor_in_a_different_package_is_not_pre_approved` |
| B7 | counted match (Go) | one entry claims every survivor with its key | `test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated` |
| B8 | survivors before timeouts (Go) | match timeouts first | `test_an_entry_claims_a_survivor_before_a_timed_out_copy` |
| B9 | a timeout keeps its entry live (Go) | stop counting timeouts as claims | `test_an_adjudication_whose_mutant_timed_out_is_not_called_stale` |
| B10 | leftover entries are stale (Go) | stop reporting them | `test_stale_entry_fails`, `test_two_entries_for_one_code_and_one_survivor_leave_one_stale` |
| B11 | resolve against the measured source (Go) | resolve after the runner returns | `test_a_survivor_is_resolved_against_the_source_the_run_measured` |
| B12 | a reused verdict is still matched (Go) | treat reused survivors as claimed | `test_a_reused_survivor_still_faces_the_adjudication_check` |
| B13 | an unreadable survivor fails closed (Go) | skip it | `test_a_survivor_whose_line_cannot_be_read_fails_the_gate` |
| B14 | code stands nowhere is reported (Go) | skip the count when it is zero | `test_an_entry_whose_code_stands_nowhere_in_its_file_is_reported` |
| B15 | other direction: a sound entry stays silent (Go) | require the count to exceed `copies` | `test_a_sound_file_of_keys_does_not_red_the_gate`, `test_every_real_adjudication_still_points_at_its_mutant` |
| B16 | `copies` checked (Go) | ignore it | the two `copies` tests |
| B17 | comment rule (Go) | delete the `COMMENT_PREFIXES` test | the three comment tests |
| B18 | token rule (Go) | delete it | `test_a_key_whose_column_holds_a_token_the_mutator_cannot_apply_to` |
| B19 | other direction: a right token is accepted (Go) | refuse every token | `test_the_same_column_is_accepted_for_the_mutator_that_does_live_there` |
| B20 | unknown mutator fails closed (Go) | return no fault | `test_a_mutator_with_no_rule_is_fatal_rather_than_silently_skipped` |
| B21 | parser (Go) | accept a missing `code:` line; treat `#` mid-line as a comment; normalise the package silently | the three parse tests, one each |
| B22 | ready entry in the message (Go) | omit the `code:` line | `test_an_unadjudicated_survivor_is_printed_as_a_ready_entry` |
| T1–T20 | the same twenty checks (TS) | the same breaks | the TS tests of the same names |
| T21 | replacement in the key (TS) | drop it | `test_two_mutants_at_one_location_are_two_separate_claims` |
| T22 | code from the report's source (TS) | read the tree instead | `test_a_survivor_is_resolved_against_the_reports_own_source` |
| T23 | missing `source` fails closed (TS) | fall back to the tree | `test_a_report_file_without_its_source_fails_the_gate` |
| T24 | UTF-16 offset (TS) | count bytes | `test_a_column_is_counted_the_way_stryker_counts_it` |
| T25 | operator and sibling rules (TS) | compare the left operand only | `test_a_key_on_the_sibling_comparison_is_rejected` |
| T26 | string-opener rule (TS) | accept any character | `test_a_string_key_whose_column_does_not_open_a_string_is_rejected` |
| T27 | `#` is text (TS) | restore `split("#", 1)` | `test_a_hash_in_a_replacement_or_reason_is_kept` |

## Tasks, in dependency order

**Task 0. Freeze the evidence.** No repository file changes.
- Copy `reports/mutation-skip-cache.json` and `reports/mutation/{mutation.json,ts-inputs.sha256}` into a gitignored folder under `reports/`, so that they outlive the session and the final `task check`.
- Record their sha256 in the report draft.
- Re-run the freshness checks from "Evidence on disk".

*True when:* the copies exist, their hashes are recorded, and 14 of 14 fingerprints and the stamp still match. If they do not match, stop and ask Patrik.

**Task 1. Requirements** (`dev-cycle:requirements`). Allocate ids with `requirement-id` for:
- the three rules in the ticket;
- staleness (G2);
- the static refusal of an entry whose code stands nowhere;
- `copies`, if Q4 is answered yes.

*True when:* the ids are allocated and their statements are written. The rows land later, with the tests that carry the ids.

**Task 2. Go tests first.** Change `tools/check_mutation_test.py` as the Go test list says. Each new test carries its `# VTT-NNN`.

*True when:* every kept test still passes, every new test fails, and the file runs.

**Task 3. Go gate, conversion and header.**
- Implement D1–D10 for Go in `tools/check-mutation.py`, deleting D7's functions and rewriting the module docstring's "IT DOES READ GO SOURCE" paragraph.
- Convert the 36 entries with D11.
- Make D12's and D13's edits to `tools/mutation-equivalents.txt`.
- Add the Go rows to `docs/requirements.md`.

*True when:*
- `python3 tools/check_mutation_test.py` passes;
- the static check finds nothing in the real file;
- the converted file parses to 36 entries;
- a command shows each reason byte-identical to `ca14522`'s apart from D13's listed sentences;
- `python3 tools/check-requirements-chain.py .` passes.

**Task 4. Go breaks.** Make B1–B22 in a scratch copy, one at a time.

*True when:* each break made its named tests fail, and the result is recorded in the report.

**Task 5. TS tests first.** The same as Task 2, for `tools/check_ts_mutation_test.py`.

**Task 6. TS gate, conversion and header.** The same as Task 3, for `tools/check-ts-mutation.py` and `tools/ts-mutation-equivalents.txt`, with 67 entries.

**Task 7. TS breaks.** T1–T27, as in Task 4.

**Task 8. Done 1 and Done 5, by command.**
- Done 1: copy the working tree with `git ls-files -z | tar --null -T - -cf - | tar -x -C <scratch>`. Insert one comment line at the top of `internal/engine/apply.go` and `client/src/fold.ts`. Run both self-tests there. Both must pass.
- Done 5: run Appendix A against Task 0's frozen copies.
- Both outputs go verbatim into the report.

*True when:* the self-tests pass on the shifted copy, and Appendix A prints an empty difference for each gate.

**Task 9 (Done 6; Q3 answered "this branch").** Cut the dated re-point accounts from reasons and headers, by reading, in a commit of its own.

*True when:* a script shows every reason after the cut equals its reason before minus whole sentences. Every removed sentence is listed in the report, and the self-tests and the static check still pass.

**Task 10 (Q2 answered yes).** Write the specification with `dev-cycle:specification`. It covers D1–D6 and D15–D17 in the present tense and lists the requirement ids. Point both equivalents headers at it.

*True when:* `docs/specifications/020-a-mutation-adjudication-names-the-code-it-excuses.md` exists at exactly that path, which both equivalents headers already name, and `python3 tools/check-requirements-chain.py .` passes.

**Task 11. CLAUDE.md rule 8.**
- Replace the paragraph from "THERE IS NO EXCEPTION. One stood here until 2026-09-17" to "it ends when the re-keying does." The new text says there is no exception, and that the mutation adjudication files name a mutant by its code, mutator and offset (pointing at the specification if Task 10 ran).
- Check the rest of the rule for any sentence that still assumes coordinate keys.

*True when:* Phase 4b reads Done 7 as met.

**Task 12. The gate and the report.**
- Run `task check` whole: launch it detached from the shell's session, wait for low load, and do not edit the tree while it runs.
- It should reuse all 14 Go verdicts and the stored Stryker report.
- Write the implementation report under `docs/reports/`. It carries the D11 and Appendix A scripts verbatim, the break results, the D13 cuts, and Phase 4a's adjudications.

*True when:* Done 8 is green and the report answers every Done item with its observation.

## Noticed, outside this ticket

- `internal/adventure/compile_test.go` refers to "the campaign entries in `tools/mutation-equivalents.txt`". At `ca14522` the file has no `internal/campaign` entry. This is a pointer that resolves to nothing.
- Today's TS parser silently cuts two reasons at a `#`. D5 fixes this as a side effect.

## Appendix A: Done 5's command, to be rebuilt against the final API

Run from the repository root on the branch, with `BASE=ca14522` and `SNAP=<Task 0's folder>`. It does the following:

1. `git diff --quiet $BASE -- internal client/src`, so the source the survivors were measured on is the branch's source.
2. Load `check-mutation.py`, `check-ts-mutation.py` and both equivalents files as they stood at `$BASE` (`git show $BASE:<path>` into a temp folder). Load the branch's versions too.
3. **Go.** The live mutants are every `(package, location, mutator)` in `$SNAP/mutation-skip-cache.json`, survivors and timed-out mutants alike.
   - **old** = those whose tuple is a key of the base `read_equivalents`.
   - **new** = the branch's `claims(read_equivalents(<branch file>), live)`, with code resolved against the branch tree.
4. **TS.** The live mutants are every Survived or Timeout mutant in `$SNAP/mutation.json`, as `(file, line:col, mutator, replacement)`.
   - **old** = those that are keys of the base `read_equivalents`.
   - **new** = the branch's `claims`, with code read from the report's `source`.
5. Print, for each gate, old, new, `old - new` and `new - old`. Exit 1 if either difference is non-empty.

A prototype of steps 3–4, re-implementing D1–D3 because the branch API does not exist yet, printed this at `ca14522`:

```
GO survivors+timeouts=42 old_claimed=36 new_claimed=36 old-new=[] new-old=[] entries_unclaimed={}
TS survived+timeout=76 old_claimed=67 new_claimed=67 old-new=[] new-old=[] entries_unclaimed={}
```

Two pairs of live mutants share a code key. Both are unadjudicated timeouts on identical lines: `internal/sight`'s two `for` loops (INCREMENT_DECREMENT), and `client/src/view/scene-plan.ts`'s `gx--`/`gy--` (UpdateOperator). Under D3 they change no verdict, because a timeout never becomes unadjudicated.

## Changes after Phase 4a and the Go review, ruled by Patrik 2026-10-10

The Go review showed that D4 does not close what it claimed to. Its R1 is not "the one way a code key could claim a survivor nobody judged". With two copies of a line and one entry, the gate claims a survivor on the copy nobody judged when the judged copy's mutant is killed, and no check fires. The real file has that case: the `internal/rules` `expr.go` entry on `if p.depth > maxExprDepth {` has `copies=2`, and its own reason says the other copy is not equivalent. TypeScript QA found the same class in the TS key: mutants that share file, mutator, replacement and start but differ in extent get one key. The real report holds 31 such groups, all ConditionalExpression: 30 pairs and one group of four. One of them pairs a survivor with a killed mutant, in `client/src/auth.ts`. Both are holes in "an adjudication excuses only the mutant it was written for", and the TS one held under position keys too.

**D15. Each entry records `nth`, which copy of its code it names, counted from the top of the file, and `nth` is part of the key.** It changes only when a copy is added or removed above the entry, which already changes `copies`, or when copies swap places. In a swap the entry names the other copy, and that copy's mutant goes unclaimed and the entry goes stale, unless that mutant survives too. D3's counting stays. Two entries now share a key only by duplication, and the static check refuses that in place of "more entries than copies". It also refuses an `nth` above `copies`. The sentence "identical statements have interchangeable reasons" is withdrawn.

**D16. A TypeScript entry records `span`, the length in UTF-16 code units of the code its mutant covers, from Stryker's `location.start` to `location.end` in the report's own source, and `span` is part of the TypeScript key.** gremlins reports no end, and a Go mutant rewrites one token at its offset, so the Go key has no `span`. A TypeScript survivor whose report gives no end, or an end outside the source, fails the gate (VTT-304). The static check cannot verify `span` against the tree without a parser. A wrong `span` matches nothing and fails loudly, as stale beside unadjudicated.

**D17. The Go gate reads a package's source recursively, naming each file by its path relative to the package.** gremlins recurses into a package's ungated subdirectories and names their files that way (`gremlins_args`). At `ca14522` no gated package has an ungated subdirectory holding Go source, so nothing changes today. Before D17 a correct entry for such a file could never be claimed.

**Formats after D15 and D16:**

```
Go:  <package>  <file>  <MUTATOR>  offset=<n>  nth=<n>  copies=<n>
TS:  <file>  <MUTATOR>  offset=<n>  span=<n>  nth=<n>  copies=<n>  <replacement, to end of line>
```

The conversion adds `nth` from the tree and, for TypeScript, `span` from the frozen report. It does this by matching each entry to the one survivor the old key claimed, which Done 5's comparison shows is a one-to-one map.

**Also ruled:**
- the review's mechanical findings are fixed in place;
- S6, reasons that point at other entries by old keys, becomes part of Task 9's commit, where each such pointer names its entry instead;
- the printed entry is ready to paste, with the header in column 1.

**D18. A survivor whose column falls in its line's indentation, or past the end of the trimmed line, names no code and fails the gate as unreadable (VTT-304).** Go QA found that the gate printed, for such a column, an entry its own parser or static check then refused (`offset=-1`, or an offset past the code). gremlins and Stryker are not expected to report such a column. If one did, a printed entry that cannot be used would hide it.

**D19. The ready entry the gate prints for an unadjudicated survivor has its header in column 1, and its `code:` line and placeholder reason indented four spaces.** Pasted as printed, with its reason written, it parses and claims the survivor. TypeScript QA found that the earlier printing, indented four and eight spaces, failed to parse when pasted.

**D20. The printed placeholder, `<the observable that makes it equivalent>`, is refused as a reason, in both files.** Go QA found that a ready entry pasted unedited parsed and claimed its survivor. That excused a mutant nobody judged, which is the same class D15 closes. ADR-010's decision 4, that every entry has a reason, is what this keeps true.

**D16, amended after TypeScript QA's second round.** For a mutant that spans lines, `span` counts the covered code with each line's indentation and trailing blanks left out and each line break as one unit. QA found that a span with the indentation in it changed when a block was re-indented. Three real entries went stale that way, which contradicted VTT-298's "however it is indented": `fold.ts`'s and `wire.ts`'s ConditionalExpression `default:` and `view/feed.ts`'s MethodExpression. Their spans were recomputed from the frozen report with the gate's own `resolve()`. QA's `span_between` helper and its multi-line test's expected value follow the amended definition, and a CRLF break now counts as one unit, the same as LF.

**After the second review, 2026-10-10.** VTT-298's statement was amended in its uncommitted row, on Patrik's ruling, to name the copy and, for TypeScript, the extent. `test_a_second_copy_of_an_adjudicated_line_is_reported_unadjudicated` now cites VTT-299, since with `nth` in the key it no longer separates one entry from two survivors on one key. The code line is trimmed as the gate trims source, `" \t\r"`, so a printed entry pasted back names the same code, and the TypeScript parser splits on line feeds only.
