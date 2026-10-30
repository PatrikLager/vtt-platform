# Plan: the eight surviving mutants on the DM console's lock keys are adjudicated equivalent

- **Ticket:**
  `docs/superpowers/specs/2026-10-30-the-console-lock-keys-are-equivalent-design.md`.
- **Verdict:** passes, with gaps G1 to G7 (below).
  - Verified 2026-10-30 by an agent that did not write the ticket,
    read-only against HEAD `526885b` on the branch.
  - Probes ran in a scratch copy of the tree outside the repository, with
    `node_modules` linked from it; their scripts are not committed, and
    this plan carries what they measured. Nothing in the repository was
    written but this plan. The stamp,
    `reports/mutation/ts-mutation-stamp.json`, hashed the same before and
    after, and the tree's input hash is the stamp's `inputs` (P11).
  - `task check`, Stryker and gremlins were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-30-the-console-lock-keys-are-equivalent.md`.
- **Base:** main `526885b`.
- **Branch:** `fix/the-console-lock-keys-are-equivalent`.
- **Owner's rulings taken in:**
  - 2026-10-30, choosing between adjudicating the eight as equivalent and
    a refactor that makes each key its button's `data-action`: "Bedöm som
    ekvivalenta". This plan does not reopen it.
  - 2026-10-26, SPEC-023: the client's full mutation run happens when the
    owner asks. The run on `526885b` was his.
- **Signed off:** by the owner, 2026-10-30: "Ja, kör".

## The verdict, check by check

1. **Every path resolves. By command.** `reports/mutation/mutation.json`
   and `reports/mutation/ts-mutation-stamp.json` exist and are ignored by
   `.gitignore`'s `/reports/`. `tools/ts-mutation-report.py`,
   `tools/check-ts-mutation.py`, `tools/ts-mutation-equivalents.txt`,
   `client/src/view/canvas.ts`, `client/src/view/scene-plan.ts` and
   `client/src/wire.ts` exist. `sending` and `unanswered` stand in
   `client/src/view/dm.ts`, and `sending` has the ten calls the ticket
   counts: eight with a string literal for a key, in the order the ticket
   lists them, and the two template keys it quotes. The equivalents file
   has a `client/src/view/dm.ts` section. `docs/reports/` exists and the
   report the ticket names does not yet.
2. **"Done" is an observation.**
   - Item 1 fails on `526885b`, by command: the report check exits 1 and prints
     eight `SURVIVED StringLiteral` lines, each on one of the eight calls
     (P1).
   - Item 2 is an observation of the finished file. Its trial form holds,
     by command: eight entries with a placeholder reason, each removed in
     turn, red for that mutant alone (P10).
   - Item 3 holds on `526885b`, by command, for all eight (P6). It is the
     evidence the entries stand on, not a change, so holding there is
     right.
   - Item 4 fails on `526885b`, by command, for both halves of its step:
     `check:ts-mutation` runs `tools/check_ts_mutation_qa_test.py` before
     the report check, and six of its `RealData` tests fail (P11); the
     report check exits 1 (P1). `task check` itself was not run.
   - Item 5 fails on `526885b`, by command: `git diff main --
     tools/ts-mutation-equivalents.txt` prints nothing.
3. **Each rule is breakable. A reading.** The ticket puts no rule, and is
   right not to: an entry is a record the gate reads, not a rule of the
   system. VTT-541 holds the lock's behaviour and does not move. There is
   nothing to sort.
4. **The scope matches the claim. By command, then a reading.**
   - The equivalents file is read by `tools/check-ts-mutation.py`, run by
     `tools/ts-mutation-report.py check`, and by two test files of the
     same step: `tools/check_ts_mutation_test.py` (the real file's static
     check) and `tools/check_ts_mutation_qa_test.py`'s `RealData` (the real
     file against the real report). None needs an edit, and all pass with
     the eight entries in a copy (P11). The ticket does not name the two
     test files (G1).
   - `unanswered` is read nowhere but `sending`, and `dm.ts` exports only
     `DMDeps` and `renderDMConsole` (P4).
   - The reading: the section the ticket names carries a paragraph the
     entries would contradict (G2), and the entries' premise lies in
     `sending`, outside the eight lines that key them (G4).
5. **No recorded decision is contradicted silently. A reading.**
   - SPEC-020: the entries take its form, and its coordinate grep must stay
     empty (P12).
   - SPEC-023: nothing the input hash reads changes, so the report stays
     current (P14).
   - SPEC-022's "One press, one command" names the keys' values. Nothing in
     it changes, and nothing in it is contradicted (G5). The 2026-10-29 plan
     `2026-10-29-a-refused-command-keeps-what-was-typed`, D8, "a form
     button's key is its action", still holds, by two equal literals.
   - The equivalents file's own header asks for ANY observable. Two exist,
     both by reflection, and the file's `client/src/view/scene-plan.ts`
     entry is the precedent for recording such a distinguisher rather than
     hiding it (G3).
   - No gate changes (rule 2).
6. **The records it moves. By command, then a reading.** The section is
   present and says `None.`. Of `docs/specifications/`, SPEC-020 and
   SPEC-022 mention `unanswered`, adjudication or the equivalents file
   (P13). SPEC-020 describes the mechanism the entries use, not the
   entries. SPEC-022's sentences stay true of the code. `None.` is right.

## The gaps this plan inherits

- **G1. The step's own tests read the file, and the ticket does not name
  them.** `tools/check_ts_mutation_qa_test.py`'s `RealData` runs while the
  report is current. Six of its eight fail on `526885b`:
  `test_the_real_file_claims_exactly_the_real_reports_survivors`,
  `test_the_real_auth_entry_claims_only_the_mutant_it_was_written_for`,
  `test_real_entries_still_claim_after_a_comment_line_at_the_top_of_fold_ts`,
  `test_real_entries_still_claim_after_every_file_moves`,
  `test_real_survivors_still_claimed_after_every_file_is_reindented` and
  `test_the_real_report_is_matched_against_its_own_source_when_the_tree_has_moved`.
  They need no edit and pass with the entries (P11). Task 1 holds them.
- **G2. The section's paragraph says the section is empty.** It opens "166
  survivors -> 0 here", and says `dm.ts`'s four other entries sit under the
  `client/src/app.ts` header. With eight entries under it, the first reads
  wrong. Rewriting it removes lines, which item 5 forbids. D3 adds a
  sentence that turns that opening into history, and deletes no line.
- **G3. Each mutant is distinguishable, by reflection only.** By command:
  - instrumenting `Set.prototype.add` while the eight controls are pressed
    records the eight literals in order, and under each mutant `""` in that
    control's place (P8);
  - `renderDMConsole.toString()` holds `"start-session"` twice, and once
    under that mutant (P9).

  Neither is an input to the console. What the console does, its DOM, every
  box's value, the commands it sends, its notices and its repaints, is byte
  for byte the same under all eight (P7). This is the class the
  `client/src/view/scene-plan.ts` entry met with `String.prototype`, and
  recorded. So a test COULD kill these eight, by spying on a builtin and
  pinning a private key's spelling. The precedent rejects that test; D2
  records the distinguishers in the file so the rejection is visible. The
  choice put to the owner named equivalence and the refactor; this third
  route was not among them. It changes nothing in the ticket.
- **G4. The premise lies outside the keyed lines.** Each entry is keyed by
  its call's line; its reason rests on `sending` and `unanswered`. SPEC-020
  says such a reason is not judged again when that other code changes. D2
  makes every reason name both symbols, so that `git grep -n unanswered`
  from someone editing `sending` lands on all eight. A new call whose
  template key could come out empty is not caught that way; SPEC-020
  concedes the class, and the case is near-theoretical.
- **G5. SPEC-022 names the keys.** "keyed by the control's action ... by
  `actor-json` for Add from JSON ... keyed `add-narration`". That stays true
  of the code, and after this ticket the equivalents file says that no test
  observes it. The sentence is true by two equal literals, not by
  construction; the refactor the owner declined would have made it so.
  Told, not moved.
- **G6. `tools/ts-mutation-scope.md`'s "adjudicated equivalent" column is
  already out of step.** It reads 1 for `dm.ts` against 4 entries on `526885b`,
  and 12 after this ticket; five other rows disagree with the file too
  (P16). Not this ticket's; a candidate for the owner.
- **G7. Two mutant counts.** The ticket's 3337 is what Stryker
  instrumented. Six of them are `Ignored`, by `// Stryker disable`, and the
  checker's last line counts the other 3331 (P2). The report says which
  number it quotes.

## Measurements this plan stands on

All on 2026-10-30 at `526885b`, at a load of 1.4 to 2.0, outside the
repository unless named.

| # | Probe | Result |
|---|---|---|
| P1 | `python3 tools/ts-mutation-report.py check`, in the repository | "the stored report is current (its inputs are the tree's); it measured 526885bc0c77, from a clean tree"; eight `SURVIVED StringLiteral` on the eight literal-keyed `sending` calls of `client/src/view/dm.ts`; exit 1 |
| P2 | The statuses in `reports/mutation/mutation.json` | 3337: Killed 3251, Survived 71, Timeout 9, Ignored 6. `dm.ts`: 12 survived, 4 of them claimed |
| P3 | The report's mutants on the lines of `sending` and its calls | the eight literal keys to `""` Survived; both template keys to an empty template literal Killed; `unanswered.has(key)` forced either way Killed; the `.finally` arrow Killed |
| P4 | `grep -rnw unanswered client/src`; `grep -n '^export' client/src/view/dm.ts` | `dm.ts` alone: the declaration, `.has`, `.add`, `.delete` (one prose word in `view/player.ts`'s doc). Exports `DMDeps`, `renderDMConsole` |
| P5 | `env -u CLAUDECODE bun test client/test`, unmutated copy | 1773 pass, 0 fail, 58 files |
| P6 | The same, with `sending(d, "<key>",` made `sending(d, "",` once per run, restored and compared after each | `add-narration`, `start-session`, `add-actor`, `remove-actor`, `actor-json`, `place-token`, `remove-token`, `upsert-note`: each 1773 pass, 0 fail |
| P7 | A scratch trace: 13 controls (Send, Enter, the six forms, Add from JSON, two Describes, two Grants) pressed while held, again on the old and a fresh console, refused, pressed, accepted, then with an immediate ok; each step records a fresh paint's `outerHTML`, every box's `.value`, notices, repaints, and the commands sent with `requestId` masked | 345,611 bytes; byte-identical under all eight mutants and over two re-runs of the original |
| P8 | `Set.prototype.add` instrumented while the eight literal-keyed controls are pressed | original: the eight literals in order; each mutant: `""` in its control's place |
| P9 | `renderDMConsole.toString()` | `"start-session"` twice; once under that mutant |
| P10 | `python3 tools/check-ts-mutation.py <scratch file>`, the scratch file being `526885b`'s equivalents plus the eight entries P1 prints, each with a placeholder reason | "3331 mutants, 3251 killed, 9 timed out, 71 survivors all adjudicated equivalent, zero unadjudicated survivors", exit 0. With each entry removed in turn: exit 1, one `SURVIVED` on that entry's code, no stale line |
| P11 | The copy with the report, the stamp and P10's file in place | `tools/ts-mutation-inputs.py` prints the stamp's `inputs`, `1de3fc01949080af…`, as the repository does; report check exit 0, no `SURVIVED`; `check_ts_mutation_qa_test.py` 79 OK, `RealData` 8 OK; `check_ts_mutation_test.py` 70 OK. In the repository on `526885b`, `RealData` fails 6 of 8 |
| P12 | SPEC-020's coordinate grep over both equivalents files | no output |
| P13 | `grep -l` for `unanswered`, `adjudicat` and `ts-mutation-equivalents` in `docs/specifications/` | SPEC-020, SPEC-022 |
| P14 | `TREES` and `FILES` in `tools/ts-mutation-inputs.py` | neither `tools/` nor `docs/` hashed; its docstring: "NOT the checker and NOT the adjudication file" |
| P15 | `~/dev/RPTool/maptool` at `f4b7fef6c`: its Gradle files for `pitest` or `stryker` | none |
| P16 | `tools/ts-mutation-scope.md`'s column against the file's entries | `dm.ts` 1 / 4, `fold.ts` 14 / 15, `view/spectator.ts` 1 / 2, `view/grid.ts` 7 / 5, `app.ts` 1 / 0, `session.ts` 0 / 2 |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. The entries are adjudications, recorded where the
    record says they go; no threshold, exemption or scope changes.
  - 8: cite names. The checker prints each survivor with a
    `file:line:column`; no committed text, commit messages included,
    copies one. A mutant is named by its code.
  - 9: answered below.
  - 10: no comment under `client/src` is touched.
- **Nothing the input hash reads changes, on the branch or in the working
  tree** (SPEC-023): not `client/src`, `client/test`, `contract/gen/ts`,
  `contract/testdata`, `scenarios/goldens`, `campaigns/example/art`, nor
  the single files in `FILES`. An untracked `.ts` under `client/src` or
  `client/test` counts too, so no probe is written there. Otherwise the
  report reads out of date and the step passes without judging.
- **Nothing under `reports/` is written, moved or deleted.** Before and
  after each task: `sha256sum reports/mutation/ts-mutation-stamp.json`
  unchanged, and `python3 tools/ts-mutation-inputs.py` prints the stamp's
  `inputs`.
- **Nobody starts Stryker** (SPEC-023).
- **Every hand mutation and probe runs in a scratch copy outside the
  repository** (D5).
- `bun test` is measured with `env -u CLAUDECODE`.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo an edit. Use the inverse edit, checked
  with grep.
- **The review gate refuses untracked files.** Park scratch files outside
  the tree with `cp` and `cmp`.

## Rule 9: how MapTool does this

It does not. MapTool's build names no mutation tool (P15); the gate and its
adjudications are this project's own. Nothing to borrow and nothing to
reject.

## Design questions for the owner

None. The ruling of 2026-10-30 settled the one question the ticket had. G3
and G6 are told to him, not asked.

## Decisions

1. **D1. The eight headers and code lines are pasted as the checker
   prints them.** Forced by SPEC-020: the key is the checker's, and P10
   shows the printed form claims each survivor. Hand-built keys are how a
   `span` or `offset` goes wrong.
2. **D2. One `#` block holds what the eight share; each reason holds its
   own premise.**
   - Forced by the owner's rule of 2026-09-23, one source for a fact, and
     by G4.
   - The block, at the head of the eight, says: the eight are one mutation,
     `sending`'s key emptied at each literal-keyed call, and the
     template-keyed calls' mutants were killed in the same run; the
     hand-applied mutants leave `bun test client/test` passing at
     `526885b`, and a trace of the DOM, every box's value, the commands
     sent, notices and repaints is the same byte for byte (P6, P7); the two
     distinguishers of G3, both by reflection, and why no test is written
     for them, pointing at the `client/src/view/scene-plan.ts` entry on
     `scene.Objects ?? []` rather than restating its reasoning; that the
     premise lies in `sending` and `unanswered`, so an edit to either
     re-judges all eight; and the owner's ruling of 2026-10-30.
   - Each reason, one line, names its control and says: `sending` uses its
     key only as the member `unanswered.has`, `.add` and `.delete` name;
     `unanswered` is module-private, never iterated and never sized; `""`
     is no other call's key, since the seven other literals are non-empty
     and differ, and both template keys open with a non-empty prefix; so
     the control still holds back itself (VTT-523 for the seven forms,
     VTT-535 for the Say) and no other control (VTT-541). The
     `add-narration` reason adds that Send and Enter share the one `say`,
     so they stay one control. Each points at the block by its place in the
     file.
   - Measured counts, of tests and bytes, go to the report, not the file:
     a count rots by addition.
3. **D3. The section's paragraph gains a sentence and loses none.** Forced
   by G2 and item 5. The eight go under the `client/src/view/dm.ts` header,
   after its paragraph. The four older `dm.ts` entries stay under the
   `client/src/app.ts` header: moving them deletes lines.
4. **D4. The entries go in the order of their code in `dm.ts`.** A
   preference: it is the order the checker prints, so the diff reads
   against P1.
5. **D5. Item 3 runs in a copy outside the repository.** Forced by
   SPEC-023 and the constraint above. The recipe, from a new directory
   `$S`:

   ```
   rsync -a --exclude=/.git --exclude=/node_modules --exclude=/reports \
     ~/dev/vtt-platform/ "$S"/
   ln -sfn ~/dev/vtt-platform/node_modules "$S"/node_modules
   ```

   For each key: copy `client/src/view/dm.ts` aside; replace the one
   `sending(d, "<key>",` with `sending(d, "",`, asserting the text stood
   exactly once before and `sending(d, "",` exactly once after; run
   `env -u CLAUDECODE bun test client/test` from `$S`; restore, and `cmp`
   against the repository's `dm.ts`. Also run it unmutated first.
6. **D6. Item 2 runs the checker over scratch copies of the file.** Forced
   by the constraint against editing a tracked file to break it:
   `ts-mutation-report.py check` reads only the tracked path, and what it
   runs, once it finds the report current, is
   `tools/check-ts-mutation.py` with that path, against the repository's
   own tree whatever directory the file is in (P10). For each entry: a
   copy of the committed file with that entry's three lines removed;
   `python3 tools/check-ts-mutation.py <copy>`; exit 1, exactly one
   `SURVIVED`, on that entry's code, and no "no longer survives" line.
   This is Phase 3's deliberate break, eight of eight; the other direction
   is the committed file, green.
7. **D7. Phase 4a does not run.** A judgment under the cycle's skip rule,
   whose list does not name an adjudication file: the ticket puts no rule
   and moves no specification, so no sentence describes what is
   delivered, and an agent that may not read the source
   cannot test a claim about the source. What executes against the claim
   is item 3, run by the implementer, and again by the reviewer (Phase 4b).
   The commit says so.
8. **D8. The report quotes the run as the owner's, on `526885b`, 73
   minutes, with the counts of P2.** Its log lives on the machine that ran
   it; the report carries the numbers, not the path.

## Requirements

None. The ticket puts no rule (verdict, check 3), and an adjudication earns
no row: it is the gate's record, where CLAUDE.md says adjudications go.
VTT-541 holds the behaviour and is cited, not changed.

## Tasks

### Task 0: sign-off, and the records

- **Do:**
  - Get the owner's sign-off on this plan.
  - Commit the ticket and this plan, with a reading review per the cycle.
- **True when:** `git status --porcelain` is clean; the stamp's hash and
  the input hash are as before.

### Task 1: the eight entries (items 1, 2, 3 and 5; D1 to D6)

- **Files:** `tools/ts-mutation-equivalents.txt` only.
- **Do:**
  1. Run `python3 tools/ts-mutation-report.py check`; take the eight
     headers and `code:` lines it prints (D1).
  2. Write the block, the eight reasons and the section's added sentence
     (D2, D3, D4), each reason against its own code.
  3. Run item 3 by D5's recipe, all eight and the original.
  4. Run item 2 by D6, all eight.
- **True when:**
  - `python3 tools/ts-mutation-report.py check` exits 0, prints that the
    report is current and measured `526885bc0c77` from a clean tree, ends
    "71 survivors all adjudicated equivalent, zero unadjudicated
    survivors.", and prints no `SURVIVED`;
  - `python3 tools/check_ts_mutation_test.py -q` and
    `python3 tools/check_ts_mutation_qa_test.py -q` pass, and
    `python3 tools/check_ts_mutation_qa_test.py RealData -v` runs its
    eight tests, none skipped (G1);
  - SPEC-020's coordinate grep over both equivalents files prints nothing;
  - `git diff main --numstat -- tools/ts-mutation-equivalents.txt` shows
    no deleted line, and `git diff main -- tools/ts-mutation-equivalents.txt
    | grep -c '^+client/src/view/dm.ts  StringLiteral'` prints 8;
  - item 3: eight mutants and the original, each with no failing test;
  - item 2: eight of eight red for their own mutant alone;
  - `git status --porcelain` names only the equivalents file; the stamp's
    hash and the input hash are as before.
- **Commit:** after Phase 4b. The message says what was broken and which
  check spoke (D6), that Phase 4a did not run and why (D7), and names no
  coordinate.

### Task 2: the report

- **Do:** with the `implementation-report` skill, write
  `docs/reports/2026-10-30-the-console-lock-keys-are-equivalent.md`.
- **It covers:** the five Done items in the ticket's numbering, each with
  its observation; item 3's table as Task 1 measured it; P7 to P9, the
  trace and the two distinguishers; G1 to G7, each with what became of
  it; the run's figures (D8, G7) and its nine timed-out mutants, out of
  scope; Rule 9; a heading for Phase 4a's adjudications that says none,
  and why (D7). Leave room for the gate's figures.
- **True when:** Done items 1, 2, 3 and 5 are `[x]` with their
  observations, item 4 waits for Task 4, and every number in the report was
  measured on the branch, not copied from this plan.

### Task 3: the specification

- **Do:** none is written. Run `git grep -n -e unanswered -e add-narration
  -e actor-json -- docs/specifications`, read the hits in SPEC-022, and
  confirm each is still true of `dm.ts`. Write the result in the report.
- **True when:** the report says which specifications were read and that
  none moved.

### Task 4: the gate, whole, and the last commit

- **Do:**
  - Check free space and the machine's load.
  - Run `task check` whole on the branch, after Task 1's commit. Launch it
    with `start_new_session=True`, send all output to a file, and require
    the gate's own completion line.
  - Write the figures into the report, then commit it.
  - Tell the owner: the last full client run is `526885b`; the stored
    report stays current until something the hash reads changes; G3's
    third route and G6's stale column are his to know about.
- **True when:**
  - `task check` exits 0, and the report's item 4 is `[x]` with it;
  - `check:ts-mutation` printed that the stored report is current, that it
    measured `526885bc0c77` from a clean tree, and "zero unadjudicated
    survivors", and did not print THE CLIENT'S MUTATION WAS NOT MEASURED;
  - the log holds no line from Stryker's own logger: `grep -c
    'INFO Instrumenter\|MutationTestExecutor' <log>` prints 0;
  - the stamp's hash is unchanged.
- Push and merge on the owner's word.

## For Phase 4b

One reviewer, per task. For Task 1, hand it the diff, SPEC-020, the
equivalents file's header, `sending` and its calls, and this plan's D2.
Tell it:
- to check each reason BY COMMAND: re-run D5's recipe for all eight; bound
  "module-private, never iterated, never sized" with
  `grep -rnw unanswered client/src` and `grep -n '^export'` on `dm.ts`;
  bound "no other call's key" with `grep -n 'sending(d, ' client/src/view/dm.ts`;
- to re-run P8, or its own instrumentation, if it doubts G3;
- to run SPEC-020's coordinate grep;
- to hash the stamp and print the input hash before and after it reads;
- never to run Stryker, `task check` or anything that starts Docker, and
  never to write under the repository.
