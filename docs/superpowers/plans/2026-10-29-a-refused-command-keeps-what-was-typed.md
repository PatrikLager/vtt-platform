# Plan: a refused command keeps what was typed, and the DM's speaker stays until changed

- **Ticket:**
  `docs/superpowers/specs/2026-10-29-a-refused-command-keeps-what-was-typed-design.md`.
- **Verdict:** passes, with gaps G1 to G8 (below).
  - Verified 2026-10-29 by an agent that did not write the ticket,
    read-only against HEAD `83d9661`.
  - Probes ran under
    `~/.claude/projects/-Users-patriklager-dev-vtt-platform/scratch-t9/verify/`,
    importing the client by absolute path. The owner's textContent probe ran
    as he ran it: copied into `client/`, run, the copy removed.
  - `task check`, `task e2e`, Stryker and gremlins were not run.
- **To be committed as:**
  `docs/superpowers/plans/2026-10-29-a-refused-command-keeps-what-was-typed.md`.
- **Base:** main `83d9661`.
- **Branch:** `fix/a-refused-command-keeps-what-was-typed`.
- **Owner's rulings taken in, all 2026-10-29:**
  - selects and buttons losing the focus at a repaint: "Nej, låt VTT-492
    stå". Out of scope;
  - the DM's "in character as" box: "Ja, behåll namnet". Only the text
    empties at Send; the name stays until it is changed;
  - refused text: "Ja, Say och alla formulär", to the option "Töm bara vid
    ok, överallt där konsolen eller panelen tömmer i dag. Vid vägran står
    texten kvar bredvid felmeddelandet.";
  - the false el() comments: "Rätta alla sju, behåll disable". All seven are
    corrected, the three `// Stryker disable` lines are kept with a true
    reason, and `tools/ts-mutation-equivalents.txt` gets no new row;
  - layout: "Ett ärende, en gren", the comments first.

  The ticket is consistent with each of them; it quotes only the layout
  ruling.
- **Signed off:** 2026-10-29, "Ja, kör", with Q1 "Töm ändå", Q2 "Nej,
  osynligt lås" and Q3 "Ja, med de sju". C8 is not allocated.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   `cmd/vtt/webdist` and `client/e2e/` included. Every symbol it names
   stands where it says: `renderDMConsole`, `clearDraft`, `DMDeps.send`
   in `client/src/view/dm.ts`; `renderPlayerPanel`, `PlayerUIState`, `submit`
   and `speaksFreely` in `client/src/view/player.ts`; `act` in
   `client/src/app.ts`; `Wire.send` in `client/src/wire.ts`; el() and its
   directive in three views. The tests it quotes exist by those names, and so
   do the debt entry and the two paragraphs. Two citations were off: the
   MapTool commit (G1), and where one of the two paragraphs begins (G6).
   The ticket's writer corrected both before Task 0's commit.
2. **"Done" is an observation.** The tests the items describe do not exist
   yet.
   - Items 1 and 5 fail today, by command (P5). Item 4 fails today, by
     command: "qaSpk: Send empties the box" passes with a `send` that never
     answers (P7).
   - Item 7 fails today, by reading: `act` paints after the refusal, and
     the draft was already cleared at the press.
   - Item 8 fails today, by command: the grep printed seven lines, one in
     each place (P1). The ticket's corrected grep, with `renders the word`,
     prints eight, `join.ts`'s header among them.
   - Item 2, item 6 and the accepted halves of items 3 and 4 hold today
     (P5, P7). They guard the change rather than fail before it (G4).
3. **Each rule is breakable. A reading.** Every rule has an observation
   that goes red; see "Requirements". The fourth rule's words, "one press
   … sends one command, however often it is pressed", read as a
   contradiction, and item 6 fixes what they mean. The VTT-493 move was
   wider than the ticket first said (G3); the ticket now withdraws it.
4. **The scope matches the claim. By command, then a reading.**
   - `renderPlayerPanel` has 21 calls in eight test files, and one in
     app.ts.
     The ticket names three of those files (P6).
   - A `DMDeps` is built 12 times in seven test files, plus app.ts. The
     ticket names one of those files.
   - `clearDraft` has nine calls, all inside `renderDMConsole`.
   - The reading: typing the panel's `send` as returning the answer
     touches all 22 calls, and D6 adds a member to `DMDeps`
     (G5). `client/e2e/speaking.spec.ts`, which the ticket does not name,
     depends on timing that the one-press rule changes, and so may any spec
     that refills a box before its command's answer (D12).
5. **No recorded decision is contradicted silently. A reading.**
   - The 2026-10-25 plan's D6 kept the console's "cleared when they send,
     refused or not" and an emptied speaker box. Its report raised both to
     the owner. The ticket overturns both under the owner's rulings of
     2026-10-29, and says so through VTT-493 and SPEC-022. A plan is not a
     decision record, so not naming D6 hides nothing.
   - VTT-492 stands, by ruling.
   - No gate changes (rule 2), and the contract is untouched (rule 3).
6. **The records it moves. By command, then a reading.**
   - The section is present, and SPEC-022 resolves.
   - The reading: SPEC-022 moves in the places Task 6 lists.
   - No other specification describes the console's or the panel's
     clearing (`git grep` over `docs/specifications` for draft, Say,
     `refused:` and clearing).
   - SPEC-010 and SPEC-020 do not move, because no row is added.

## The gaps this plan inherits

- **G1. The MapTool commit names the wrong repository.** `63862e4` is the
  HEAD of `~/dev/RPTool`, which tracks no file of `maptool`. `maptool` is
  its own repository, at `f4b7fef6c`, and `git cat-file` finds no
  `63862e4` there. The claims hold at `f4b7fef6c` (P10). The ticket's writer
  corrected the commit before Task 0's commit.
- **G2. An eighth place carries the same false claim.** The header comment
  of `client/src/view/join.ts` says happy-dom drops an undefined
  `textContent` while "a real browser renders the word / "undefined"". The
  line break hid it from the ticket's first grep (P2); the corrected grep
  finds it. The owner's "alla sju"
  counted the ticket's list. Q3.
- **G3. VTT-493 moves further than "since … becomes since the server last
  accepted one".** Its subject is "the player panel's Say boxes", both of
  them. A speaker box that keeps its text after Send makes it false for
  `say-as` as well. Per SPEC-008 it is withdrawn and succeeded by C2, C4,
  C5 and C6 (Task 0). The ticket now says it
  withdraws VTT-493.
- **G4. Some done items hold today.**
  - Item 6 holds because emptying the box at Send is itself the guard: a
    second press finds the box empty (P5).
  - The rule's other half, "or of a form's button", is broken today: Start
    session pressed twice sends two commands (P5). No done item observes
    it, so Task 2 adds the observation.
  - Items 2, 3 and 4 hold today for an accepted answer.
  - "a REFUSED start still clears the name — the text is gone with the
    command" answers `ok: true`. It pins clearing at the press, which needs
    no answer.
- **G5. The test files the ticket does not name (P6).**
  - The `send` type reaches `client/test/player.test.ts`,
    `command-surface.test.ts`, `qa-ability-target.test.ts`,
    `qa-what-a-window-shows.test.ts` and `token-card.test.ts`.
  - D6's `rerender` reaches `command-surface.test.ts`, `focus.test.ts`,
    `qa-describe-token.test.ts`, `qa-note-console.test.ts`,
    `qa-repaint-focus.test.ts` and `qa-token-texts.test.ts`.
  - D11 adds tests to `client/test/app.test.ts`, and D12 changes
    `client/e2e/speaking.spec.ts`.
  - Each change in those files is mechanical, except the timing changes.
- **G6. Where one of the two paragraphs begins.** "Two further mutants,
  el()'s" begins a sentence inside the paragraph under
  `client/src/view/dm.ts`'s header. That paragraph begins "166 survivors ->
  0 here". The ticket's writer corrected "paragraph" to "sentence".
- **G7. Firefox and WebKit (the ticket's first unestablished point).** Only
  Chromium is installed for Playwright (P13). Installing the others changes
  the machine, so it is not done.
- **G8. The directive also disables the guard forced false.**
  - `// Stryker disable next-line ConditionalExpression` covers both of
    the mutator's replacements.
  - Forced false, el() never writes a label. Every button would read
    blank, so that mutant is not equivalent. This is by reading, not
    measured.
  - The ruling stands. Task 1's paragraphs say what the directive covers,
    and the report says it again.

## Measurements this plan stands on

All on 2026-10-29 at `83d9661`, outside the repository unless named.

| # | Probe | Result |
|---|---|---|
| P1 | Item 8's `git grep` | 7 lines: el() in `dm.ts`, `player.ts`, `spectator.ts`; the comments of "a group's row holds only its elements, with no stray text" and "a group carries no stray text of its own"; the two paragraphs |
| P2 | `git grep -i` for happy-dom beside undefined, textContent, stringify, coerce or word, over the tracked tree less reports, plans and tickets | P1's seven, `join.ts`'s header, and the debt entry. Two className lines in the tests ("stringifies") are true and unrelated |
| P3 | The owner's probe, copied to `client/`, `env -u CLAUDECODE bun`, copy removed and seen gone | HeadlessChrome 143.0.7499.4: a div holding a span given `textContent = undefined` reads `""` with 0 child nodes; `null` reads `""` |
| P4 | happy-dom 20.11.1 by absolute import, from the scratch directory | the same `""` and 0 nodes; a new div given `undefined` reads `""`; `click()` on a disabled button delivers no click. `null` on an empty div reads `""` with 0 nodes, run by Task 0's second reviewer |
| P5 | A scratch `bun test` driving `renderDMConsole` and `renderPlayerPanel` as they stand | refused Start session: next render's `session-name` is `""`. Start session clicked twice: 2 sent. Send twice: 1 sent. Send then Enter: 1 sent. A DM's `say-as` after Send: `""`; the second line's `as`: `""` |
| P6 | `grep` for `renderPlayerPanel(`, for `toggleDoors` in `DMDeps` literals, and for `clearDraft(` | 21 panel calls in 8 test files, and 1 in app.ts; 12 `DMDeps` literals in 7 test files plus app.ts; 9 `clearDraft` calls |
| P7 | `env -u CLAUDECODE bun test` over the 14 test files P6 names, `dm-view` and `spectator-view` among them, and `app.test.ts`: 15 files | 583 pass, 0 fail |
| P8 | `entry_faults(read_equivalents(…))` from `tools/check-ts-mutation.py` | 63 entries, no fault. Rows on the touched views: `dm.ts` 4 (three comparators and the grant's `target.value = draft[field] ?? "";`), `spectator.ts` 2 comparators |
| P9 | Reading `Wire.send`, `connect`'s close handler and the gateway's read loop | `send` resolves at its result frame or at the close of the socket it was written to, with no timeout; `act` paints inside its own `.then`, before a caller's continuation; the gateway writes a result from the read loop and events from the pump, so their order is not fixed, and skips the result when `EncodeFrame` fails |
| P10 | `CommandPanel.java` in `~/dev/RPTool/maptool` at `f4b7fef6c` | `commitCommand()` trims, adds to `commandHistory` unless it repeats the last, calls `commitCommand(command, null)` and sets `commandTextArea` to `""`, also when that call drops the line for its cheater pattern or div count; `globalIdentity`, a `TokenIdentity`, stays until set or cleared |
| P11 | `tools/comment-ceilings.txt`; `tools/ts-coverage-thresholds.txt` | ceilings: `app.ts` 48.7, `view/dm.ts` 38.6, `view/join.ts` 33.9, `view/player.ts` 22.1, `view/spectator.ts` 40.8. Line-coverage floor 100.00 for all five |
| P12 | `python3 tools/ts-mutation-report.py check`; `shutil.disk_usage` | the stored report is current and measured `db81c37`; 31.3 GiB free, over the Go gate's 16 GiB `MIN_FREE_BYTES` |
| P13 | `~/Library/Caches/ms-playwright` | chromium and its headless shell only |
| P14 | `go build ./...` | builds |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone (D14). Nothing is raised, widened or skipped.
  - 3: no contract change.
  - 8: cite names, never `file:line` or `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `client/src` is a warning or a pointer. An added
    comment line carries no banned term (a date, "measured", "used to",
    "turned out", "task N" and the rest of `check-comments.py`'s list). A
    block that gains a line ends at six lines or fewer. No file's comment
    share rises above its ceiling.
- **The coverage floors are 100% for every source file this touches**
  (P11). Each new branch, the refused answer, the accepted one, the press
  while unanswered and a rejected promise, has a test that reaches it.
- **Nobody starts Stryker.** Any edit under `client/src` makes the stored
  report out of date (SPEC-023). From Task 1 on, `task check` prints that
  the client's mutation was not measured, names `db81c37` and the tickets
  since, and passes.
- **The adjudication rows stay true.** No line P8 names is edited or
  copied. `entry_faults` is run after each code task and finds no fault.
- **Module state survives tests.** `draft` and D8's set live for the whole
  bun process, so every held answer is released inside its test.
- **An answer is awaited, never assumed.** A test that reads a field after
  an answer awaits it, with `settle()` (D10).
- **A negative assertion needs its positive.** "One command was sent"
  comes with a press after the answer that sends.
- `bun test` is measured with `env -u CLAUDECODE`.
- **Before every commit:**
  - `python3 tools/check-new-prose.py main`. The 85-column band and its
    SHORT rule hold for test files too;
  - `bunx tsc --noEmit -p client/tsconfig.json`;
  - `python3 tools/check-requirements-chain.py .`.
- **`cmd/vtt/webdist` is rebuilt by `task build:client`** after the last
  `client/src` change and committed. The commit hook does not catch a
  stale bundle.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break. Use the inverse edit, checked
  with grep.
- **The review gate refuses untracked files.** Park scratch files outside
  the tree with `cp` and `cmp`.
- **Hash the tree before and after any reviewer.**
- **A new test goes after a closing brace**, or it inherits its
  neighbour's comment.

## Rule 9: how MapTool does this

In `~/dev/RPTool/maptool` at `f4b7fef6c` (P10):

- **The chat box empties at send.** `CommandPanel.commitCommand()` stores
  the line in `commandHistory`, executes it and empties
  `commandTextArea`.
- **Its local refusals do the same.** When its own checks drop the line
  (the cheater pattern, the div count), the box still empties, and the
  arrow keys recall the line from the history. No server refuses a chat
  line, so MapTool has no answer for one.
- **A second Enter is harmless**, because the box is already empty and
  `commitCommand(String, …)` drops an empty line.
- **The speaker is `globalIdentity`**, kept until it is changed or
  cleared.

Borrowed: the speaker that stays (C5). Rejected: the box that empties at
send, because here a server can refuse a line, and by the owner's ruling
the text stays. The history is not taken up; it is a ticket of its own.
MapTool's dialogs were not examined.

The project's own precedent for a press while an answer is awaited is the
join form. `JoinViewState.busy` keeps the Join button disabled and labelled
"Joining…", and `submit` in `runJoin` returns early for Enter. Q2 asks
whether to follow it.

## Design questions for the owner

1. **Q1. Something is typed into a box after Send and before the answer.
   What does an accepted answer do to it?**
   - Recommended: **it empties the box anyway**: "Töm vid ok", taken as
     written.
     - It is one rule (C3, C4) and needs no comparison.
     - What it loses is what was typed during one round trip. The length
       of that gap was not measured.
   - Alternative A: empty a box only if it still holds what was sent, and
     otherwise leave it as it stands. Nothing is lost, but the sent line
     stays in front of what was added, and Enter would send both. It adds
     C8.
   - Alternative B: remove the sent line from the front of `say-text`,
     and for the console use A. Nothing is lost and nothing is resent,
     but it adds the most code and a rule.
   - Alternative C: the boxes stay read-only until the answer. Keystrokes
     in the gap are dropped, visibly.
   - **Answered 2026-10-29:** "Töm ändå", the recommendation.
2. **Q2. While a command is unanswered, does the screen show it?**
   - Recommended: **no.**
     - A further press of the control that sent it sends nothing (C7).
       Each control has its own lock, so Remove actor still sends while
       Add is awaited.
     - Nothing is drawn differently: no repaint at the press, no new
       style.
     - The refusal's toast, or the emptied box, follows the answer.
   - Alternative: as the join form does, the control is drawn disabled,
     and Send reads "Sending…".
     - It needs a repaint at the press.
     - It needs a `.dm .chip:disabled` rule, since `style.css` has one
       for `.player` alone.
     - happy-dom does not deliver a click to a disabled button (P4), so
       the button's guard would be its disabled state, as in the join
       form. Enter in `say-text` keeps its guard in `submit`.
   - Either way, a lock ends at the answer, which `Wire.send` gives at a
     result or at the socket's close (P9). A result the gateway fails to
     encode holds its one control until the socket closes.
   - **Answered 2026-10-29:** "Nej, osynligt lås", the recommendation.
3. **Q3. Is `client/src/view/join.ts`'s header corrected with the seven
   (G2)?**
   - Recommended: **yes**, in Task 1, by cutting the false parenthetical.
     The header keeps its reason for not using el(): every node there has
     text or does not, decided where it is built.
     - It is the same false claim, in the same kind of place.
     - The edit is under `client/src`, which Task 1 already moves.
   - If no: it goes to `docs/verification-debt.md` as an open entry,
     labelled `test asserts nothing`.
   - **Answered 2026-10-29:** "Ja, med de sju", the recommendation.

## Decisions

1. **D1. The comments are Task 1, alone in one commit, before any code.**
   Forced by the owner's "Ett ärende, en gren", the comments first.
2. **D2. Each kept `// Stryker disable next-line ConditionalExpression`
   takes as its reason an imperative warning.**
   - In substance: forced true is equivalent only because `n` is new and
     empty; drop the directive if el() is ever handed an element that has
     children.
   - It is at most two lines. The wording is the implementer's, read in
     Phase 4b.
   - Forced by SPEC-010:
     - a code comment may point only at a specification, a requirement,
       a test or symbol, or a report under `docs/`, and
       `tools/ts-mutation-equivalents.txt` is none of these;
     - "measured" and dates are banned;
     - `dm.ts`'s block is nine lines and must end at six.
   - The measurement goes to the paragraphs (D4) and the report.
   - The warning is true: on an element with children, `textContent =
     null` removes them, so forced true would differ there.
   - It also says that the directive hides the guard forced false, which is
     not equivalent (G8): the directive's first line becomes the reason the
     stored report records for both mutants.
3. **D3. The two test comments in `client/test/dm-view.test.ts` say what
   each test holds.**
   - "a group's row holds only its elements, with no stray text" holds the
     `const row` sentinel. That is the debt entry's own closing condition.
   - "a group carries no stray text of its own" holds that a group's own
     children are elements.
   - Neither claims to see el()'s guard. Both tests stay.
4. **D4. The two paragraphs of `tools/ts-mutation-equivalents.txt` state
   the fact.**
   - The guard forced true is equivalent: an undefined `textContent`
     reaches the nullable DOMString as null and leaves a new element empty,
     in Chromium 143 and happy-dom 20.11.1 (P3, P4).
   - Both replacements are disabled in the source by the owner's ruling of
     2026-10-29. Forced false is not equivalent (G8).
   - No row is added. The file's header style holds: dated history is
     allowed there, as the neighbouring entries show.
5. **D5. The debt entry is closed.** It leaves "Open debt" for a dated
   section with a **Closed by** paragraph, the file's shape for a closed
   entry. That paragraph names the corrected comments and Task 1's commit.
   Under Q3's "no", the new open entry of Q3 is added.
6. **D6. The console waits for the answer, and repaints after clearing.**
   - Each handler the ticket names sends, and when the answer is `ok`,
     clears the drafts it clears today and calls a new required
     `DMDeps.rerender`. Any other answer changes nothing.
   - app.ts passes `paint`. The name follows `renderPlayerPanel`'s
     `rerender`.
   - Forced by P9:
     - `act` paints inside its own `.then`, so a caller's continuation
       always runs after that paint;
     - the result and the command's events reach the client in no fixed
       order, so no later paint is guaranteed;
     - without the console's own repaint, an accepted command's text stays
       in view until something unrelated paints. Item 2 asks for it gone
       at the paint after the answer.
   - Required, not optional, so the type makes app.ts pass it. That costs
     G5's six files a line each.
7. **D7. Save note no longer sets its live select to `""`.** The accepted
   answer's repaint (D6) draws a new select from the cleared draft, so
   setting the old one changes nothing a person sees.
   - "a choice is not carried to the next note, re-rendered or not" loses
     its "or not" half and is renamed. VTT-229's evidence follows the new
     name.
   - The select keeps the choice until the answer, as the rule wants.
8. **D8. One press, one command.**
   - `dm.ts` keeps, beside `draft`, a set of the controls whose command is
     unanswered. A form button's key is its action; a Describe or Grant
     key is its action and the token or actor id. Add from JSON, which
     carries no action, is keyed by its field, `actor-json`.
   - `PlayerUIState` gains an optional field for Send and Enter.
   - A press whose key is held sends nothing. The key is taken only when a
     command is sent, so a press the client's own guard refuses ("a
     session needs a name") takes none.
   - The key is released when the send's promise settles, resolved or
     rejected. A rejection from a throwing `paint` must not hold a control
     for good.
   - Forced by:
     - the console is rebuilt at every paint, so its state lives where
       `draft` does;
     - `PlayerUIState` is the panel's home across paints;
     - an optional field spares the `PlayerUIState` literals, as the
       2026-10-25 plan's D6 did for `sayAs` and `sayText`.
   - Under Q2's alternative, the held control is also drawn disabled.
9. **D9. The panel.**
   - `renderPlayerPanel`'s `send` is typed `(c: ClientCommand) =>
     Promise<CommandResult>`.
   - `submit` reads the line, takes the lock and sends. It neither empties
     nor repaints at the press.
   - On the answer it releases the lock. On `ok` it empties `ui.sayText`
     and the live `say-text` box (per Q1) and calls `rerender`.
   - Send never touches `ui.sayAs`.
   - The ability buttons keep ignoring their answer.
   - Forced by items 4 and 5, by "Ja, behåll namnet", and by a
     `void`-typed `send`, which cannot carry an answer.
10. **D10. A shared test helper, `client/test/support/answers.ts`.**
    - It holds `ok()`, `refused(error)` and `held()`, a `send` whose answer
      the test releases, and `settle()`, which awaits a macrotask.
    - It replaces `okResult()` in `dm-view.test.ts`.
    - Forced by more than 30 call sites needing a `send` that answers
      (P6), and by keeping one source for the result's shape. The name is
      a preference.
11. **D11. Items 1, 2 and 5 are observed in the composed app as well**, in
    `client/test/app.test.ts`, through its `FakeSocket`, which delivers a
    result with no event.
    - Forced by D6: the console's harness renders once, and only the
      composed app shows `act`'s paint and the console's in their real
      order.
    - K3 is red there, and in T4, which counts the console's repaints.
12. **D12. The browser tests.**
    - The new tests go in a new spec file, `client/e2e/refusals.spec.ts`
      (the name is a preference). Forced by `startFixtures`: one server and
      campaign per spec file. These tests open a session and give the DM a
      character, which no other file expects.
    - `client/e2e/speaking.spec.ts` waits for `say-text` to read `""` after
      its first line, before it fills the second. Forced by D8 and C4:
      today it waits only for the other page's feed. A second Enter inside
      the first line's round trip sends nothing, and a late `ok` empties
      the second line.
    - Any other spec that fills a box its previous command fed, before that
      command's answer, is made to wait for the answer the same way.
      `task e2e` runs whole.
    - Each spec that opens a session closes it in `test.afterAll`, as
      `client/e2e/setup.ts` requires.
13. **D13. VTT-493 is withdrawn and succeeded at Task 0, per SPEC-008.**
    - The register's form: `WITHDRAWN 2026-10-29: <its text>. <why>.
      Succeeded by …`, with the evidence `**READING — verify-ticket check 5
      of 2026-10-29; withdrawn, held by nothing**`, as VTT-155's row has
      it.
    - Its citations move to the successor whose observation each test
      makes, in Tasks 3 and 4. So does `PlayerUIState`'s doc comment.
    - SPEC-022 drops it from its list (Task 6), as other specifications
      dropped VTT-155 and VTT-180.
14. **D14. `python3 tools/check-comments.py --write-ledger` runs once,
    after the last code commit, and its rows are committed.**
    - Forced by `check-comments.py`, which refuses a file more than 1.0
      point under its ceiling. Task 1 shortens comments in `dm.ts`,
      `spectator.ts`, `player.ts` and `join.ts`, and Tasks 2 and 3 add code
      to `dm.ts` and `player.ts`, which lowers their shares.
    - A lowered row cannot be raised again, so it runs last.
    - Mid-branch, `--report` reads the shares, and `check-comments.py main`
      is read for added-line refusals only.
15. **D15. No Stryker run.** At Task 7 the owner is told that the stored
    report is from `db81c37`, with the tickets since as the step lists
    them, and that `task check:ts-mutation:full` is his to ask for.

## Requirements

Allocated at Task 0, 2026-10-29: C1 VTT-517, C2 VTT-518, C3 VTT-519, C4
VTT-520, C5 VTT-521, C6 VTT-522, C7 VTT-523. C8 was not, by Q1's answer.

Candidates for the sort. Task 0 allocates them after sign-off with
`requirement-id`, each `**OPEN — no test yet**` until its tests land. Each
names the observation that goes red. The C, K and test labels are this
plan's.

- **C1.** "A command the server refuses clears no DM console field that
  fed it." T1 to T3, A1, E1.
  Red under K1, K2.
- **C2.** "A line the server refuses does not empty the player panel's
  `say-text`." U1, U2. Red under K5.
- **C3.** "A command the server accepts leaves each DM console field that
  fed it as it is drawn when nothing has been typed in it, except the notes
  form's key, title and text."
  - T4, T5, A2. Red under K3, K4.
  - "as it is drawn when nothing has been typed in it", not "empty": a
    Describe box with no draft shows the token's present text (VTT-464).
  - Existing behaviour, moved from the press to the answer.
- **C4.** "A line the server accepts leaves the player panel's `say-text`
  empty." U3. Red under K6, K12.
- **C5.** "Send leaves a DM's or an agent's `say-as` holding what it held."
  U4, A3, E2. Red under K7.
- **C6.** "After a repaint, a DM's or an agent's `say-as` shows what it
  showed before it, and the player panel's `say-text` shows what was typed
  in it since the server last accepted a line from the panel."
  - The typed half is VTT-493's repaint half, existing behaviour, held
    today by "what a DM typed in Say survives a repaint, with the as box
    focused", "what a player typed in Say survives a repaint, with the text
    box focused", "qaSay: the DM's narration repaints and both boxes keep
    what was typed" and `client/e2e/typing.spec.ts`'s "what a player types
    in Say survives a repaint, focus and all". Red when the panel stops
    reading `ui.sayAs` or `ui.sayText`.
  - The "since the server last accepted" half is new: U1, U2, U3. Red
    under K5, K6.
- **C7.** "While a command that Start session, Add actor, Remove actor,
  Add from JSON, Place token, Remove token, a row's Describe, Save note or
  a row's Grant sent is unanswered, pressing that control again sends
  nothing; while a line the player panel sent is unanswered, Send or Enter
  in `say-text` sends nothing." T6, T7, U5, U6. Red under K8 to K11.
- **C8**, not allocated (Q1 was answered "Töm ändå"), only under Q1's
  alternative A: "An accepted answer leaves a field
  typed in after its command was sent as it stands." It needs a test with a
  held answer and a keystroke before the release.

The sort, and what it rejected:
- "No connection leaves it as typed": the client cannot tell this from a
  refusal, since `Wire.send` answers `ok: false` (P9). One T1 case uses
  "wire: not connected". It is not a rule of its own.
- "The refusal shows beside the toast `refused: …`": `act`'s existing
  behaviour, not this ticket's rule. A1 and E1 assert it.
- Item 8, the corrected comments: the ticket says they add no rule. Task
  1's greps and Phase 4b hold them. No id.
- The console's repaint after an accepted command: how (D6), observed under
  C3 by A2.
- Save note's key, title and text after either answer: covered by C1 and
  C3's exception.
- How the lock is keyed and where it lives: how (D8).
- A DM with no actor has no Say form: existing behaviour, out of scope.
- MapTool's history: not taken up.

Withdrawn: VTT-493, succeeded by C2, C4, C5 and C6. Kept, wording still
true:
- VTT-229, its evidence renamed (D7);
- VTT-464;
- VTT-491 and VTT-492 (by ruling);
- VTT-500 and VTT-504.

| Ticket item | Observed by |
|---|---|
| 1 | T1, A1 |
| 2 | T4, A2 |
| 3 | T2, T3, T5 |
| 4 | U1, U2, U3 |
| 5 | U4, A3 |
| 6 | U5, U6, and T6 for the console's buttons |
| 7 | E1, E2 |
| 8 | Task 1's greps, and Phase 4b |

## The existing tests that change

- **`client/test/dm-view.test.ts`:**
  - the harness's `send` takes an answer from D10, it gains `rerender`
    counting repaints, and it returns `settle` and `release`;
  - "a REFUSED start still clears the name — the text is gone with the
    command" goes, and T1 takes its place;
  - these await the answer before reading the next render: "a submitted
    session name is not restored on the next render", "removing a token
    sends its trimmed id and clears the field", "removing an actor sends
    its trimmed id and clears the field", "adding an actor clears exactly
    the actor fields", "placing a token clears exactly the token fields",
    "adding an actor from JSON clears the paste box", "a describe field
    with no draft shows the token's present text", and "a typed describe
    field survives a repaint until sent, and Describe clears only its own
    row", which is renamed, since "until sent" becomes false;
  - "a choice is not carried to the next note, re-rendered or not" (D7);
  - Task 1's two comments.
- **`client/test/spectator-view.test.ts`:**
  - `panel()` and `sayTable()` take a `send` that answers;
  - "Send empties both Say boxes for the paints after it" becomes C5 and
    C4's observation and is renamed;
  - "sending narration requires text, and clears the box afterwards" and
    "Send narrates the trimmed text, clears the box and repaints" read the
    box after the answer, and `repaints()` after the answer;
  - "only /ooc as the line's first word steps out of character" awaits the
    first Enter's answer before the second (C7).
  - "a player's switch of party member keeps the Say text, and Send speaks
    as the new one" builds its own panel with `(c) => sent.push(c)`; its
    `send` answers.
- **`client/test/qa-say-boxes.test.ts`:**
  - `table()`'s `send` answers;
  - "qaSay: after Send both boxes are empty at the paints that follow",
    "qaSay: a second Send sends only what was typed since the first" and
    "qaSay: Enter in the text box leaves the boxes as its sending decides"
    turn red on `say-as` and are made C5's: `say-as` keeps its text, and
    `say-text` empties after the answer;
  - "qaSay: a second Send sends only what was typed since the first" also
    turns red on C7 and Q1: its second Send waits for the first's answer;
  - "qaSay: Enter's repaint leaves the focus in the new text box" stays
    green, but Enter no longer repaints at the press: it awaits the answer
    before its first assertion, or that assertion holds nothing;
  - each cites the successor whose observation it makes.
- **`client/test/qa-speaking.test.ts`:**
  - `rig()`'s `send` answers;
  - "qaSpk: Send empties the box" and "qaSpk: an /ooc Send empties the
    box" await the answer;
  - the DM and agent "qaSpk: … Send empties both boxes" becomes C5's;
  - each cites a successor.
- **`player.test.ts`, `command-surface.test.ts`,
  `qa-ability-target.test.ts`, `qa-what-a-window-shows.test.ts`,
  `token-card.test.ts`:** each `send` argument answers (D9). No assertion
  changes.
- **`command-surface.test.ts`, `focus.test.ts`, `qa-describe-token.test.ts`,
  `qa-note-console.test.ts`, `qa-repaint-focus.test.ts`,
  `qa-token-texts.test.ts`:** each `DMDeps` gains `rerender`. Their `send`
  stubs already resolve a result, and the QA files already await a
  macrotask after each click.
- **`client/e2e/typing.spec.ts`:** its `// VTT-491 VTT-493` line names
  C6's id.
- **`client/e2e/speaking.spec.ts`:** D12.

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Get the owner's sign-off, with answers to Q1 to Q3.
  - The ticket's writer corrects G1's commit. G6 is the writer's to
    correct or leave.
  - Allocate C1 to C7, and C8 under Q1's alternative A, with
    `requirement-id`.
  - Withdraw VTT-493 in the register's form (D13).
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean, the rows exist, VTT-493 reads
  `WITHDRAWN 2026-10-29` and names its successors, and
  `python3 tools/check-requirements-chain.py .` passes.

### Task 1: the comments (item 8; D1 to D5, Q3)

- **Files:**
  - el()'s comment in `client/src/view/dm.ts`, `client/src/view/spectator.ts`
    and `client/src/view/player.ts`;
  - under Q3's yes, `client/src/view/join.ts`'s header;
  - the two comments in `client/test/dm-view.test.ts`;
  - the two paragraphs in `tools/ts-mutation-equivalents.txt`;
  - `docs/verification-debt.md`.
- **No code line changes.**
- **True when:**
  - item 8's `git grep` prints nothing;
  - `git grep -n -E 'harness cannot observe|renders the word' --
    client/src` prints nothing, under Q3's yes;
  - `git grep -c 'Stryker disable next-line ConditionalExpression' --
    client/src` counts one in each of the three views, each still followed,
    after its comment lines, by `if (text !== undefined) n.textContent =
    text;`;
  - `entry_faults` finds none;
  - `python3 tools/check-comments.py main` reports no banned term and no
    block over six lines on an added line. A file under its ceiling is
    expected until D14;
  - the client suite is green;
  - the debt entry stands under a dated heading with **Closed by**, and no
    longer under "Open debt";
  - Phase 4b has read each rewritten comment against the code and the
    measurement.

### Task 2: the console (items 1 to 3, the console's half of 6; D6 to D8, D10, D11)

- **Files:**
  - `client/src/view/dm.ts`: the nine handlers, `DMDeps.rerender`, the
    lock;
  - `client/src/app.ts`: `rerender: paint`;
  - new `client/test/support/answers.ts`;
  - `client/test/dm-view.test.ts`;
  - `client/test/app.test.ts`;
  - G5's six `DMDeps` files.
- **New tests:**
  - **T1** (item 1): `session-name` typed through an `input` event, Start
    session, a refused answer, `settle()`: the next render's
    `session-name` reads `Night One`. The same with "wire: not connected".
  - **T2** (item 3, refused): for each form the console draws, Add actor,
    Remove actor, Add from JSON, Place, Remove token, a row's Describe,
    Save note and a row's Grant: every field it fed reads as typed after a
    refused answer. Selects are set through `change`.
  - **T3:** Save note refused or accepted: key, title and text read as
    typed.
  - **T4** (item 2): T1 with an `ok` answer: `session-name` reads `""`,
    and the harness counted one repaint after the answer.
  - **T5** (item 3, accepted): T2's forms with `ok`. Each field it fed
    reads as with no draft: `""` for a box, the blank option for a select,
    the token's present text for a Describe box. Another form's typed
    field is untouched.
  - **T6** (C7): each of the nine controls, over T2's forms, pressed twice
    with a held answer sends one command; after the release a press sends
    again. Add from JSON is among them, under its `actor-json` key. The
    locks of a Grant row and a Describe row are per row: the other row's
    button still sends.
  - **T7:** a press the client's guard refuses takes no lock: an empty
    name, then a typed name, sends one command. A `send` whose promise
    rejects releases the lock.
  - **A1** (item 1, composed): app.test's DM, Start session with
    `Night One`, the socket answers `ok: false`: the box reads `Night One`
    and the page shows `refused: …`.
  - **A2** (item 2, composed): the socket answers `ok: true` with no
    event: the box reads `""`.
- **Order:**
  1. Write the tests and run them on Task 1's commit. T1, T2, T6 and A1
     are red. T4 is red on its repaint count. T3, T5, T7 and A2 hold
     today, so check that each reads the next render.
  2. Then write the code.
  3. Then make the breaks.
- **True when:**
  - the 15 files of P7 and the new support file are green, with no test
    skipped;
  - the typecheck is green;
  - `entry_faults` finds none.
- **Breaks**, each undone by its inverse edit:
  - **K1:** the `ok` check removed, so drafts clear on any answer. T1, T2
    and A1 red.
  - **K2:** `clearDraft` moved back before `send`. T1, T2 and A1 red.
  - **K3:** `d.rerender()` removed. A2 and T4 red.
  - **K4:** the accepted branch's `clearDraft` removed. T4 and T5 red.
  - **K8:** the lock check removed. T6 red.
  - **K10:** the lock never released. T6's second half red.
  - **K11:** the lock taken before the guard. T7 red.

### Task 3: the panel (items 4 to 6; D8, D9, D11, Q1)

- **Files:**
  - `client/src/view/player.ts`: `submit`, `PlayerUIState`, `send`'s type
    and `sayAs`'s doc comment;
  - `client/test/spectator-view.test.ts`, `qa-say-boxes.test.ts`,
    `qa-speaking.test.ts`, `app.test.ts`;
  - G5's five panel files.
- **New tests**, through a repainting table like `sayTable()`:
  - **U1** (item 4; C2, C6): a player types `I draw my sword`, Send,
    refused: the next paint shows it in `say-text`.
  - **U2:** the same for `/ooc where is the pizza`.
  - **U3** (item 4; C4, C6): U1 and U2 answered `ok`: `say-text` reads
    `""`, and
    the panel asked for one repaint after the answer.
  - **U4** (item 5): a DM types `Grimble` in `say-as` and `Hello`, Send,
    `ok`: `say-as` reads `Grimble` and `say-text` reads `""`. `Bye` sent
    next carries `as` `Grimble`. Refused, both read as typed.
  - **U5** (item 6): Send twice, Send then Enter, and Enter twice, with a
    held answer: one command. After the release, Send sends again.
  - **U6:** a blank line takes no lock: Send on `"   "`, then a typed line,
    sends one command.
  - **A3** (item 5, composed): app.test's DM controlling an actor, `say-as`
    `Grimble`, Enter, `ok` with no event: `say-as` reads `Grimble`, and
    `say-text` reads `""`.
  - **Under Q1's alternative A, C8's test:** a held answer, a keystroke,
    the release with `ok`: the box reads what was typed.
- **Order:**
  1. Write the tests first, on Task 2's commit. U1, U2 and U4 are red, and
     A3 is red on `say-as`. U3, U5 and U6 hold today (G4); check that each
     reads the next paint.
  2. Then write the code.
  3. Then make the breaks.
- **True when:**
  - green as Task 2's, the typecheck included;
  - `git grep -n 'VTT-493' -- client/src client/test` prints nothing; the
    citation in `client/e2e/typing.spec.ts` moves in Task 4.
- **Breaks:**
  - **K5:** `say-text` emptied on any answer. U1 and U2 red.
  - **K6:** not emptied on `ok`. U3 red.
  - **K7:** `ui.sayAs = ""` put back in `submit`. U4 and A3 red.
  - **K9:** `submit`'s lock check removed. U5 red.
  - **K12:** `rerender` after the `ok` emptying removed. U3 red.

### Task 4: the browser (item 7; D12)

- **Files:**
  - new `client/e2e/refusals.spec.ts`;
  - `client/e2e/speaking.spec.ts`;
  - `client/e2e/typing.spec.ts`'s citation;
  - `cmd/vtt/webdist`.
- **New tests**, each with its id line above it:
  - **E1:** the DM fills Place token with `tok-x`, `nowhere`, `nobody`, `2`
    and `3`, and clicks Place. The toast starts `refused: `, and each of
    the five boxes holds what was typed.
  - **E2:** `asDM` starts a session, loads `goblin-ambush` and grants one
    of its actors, such as `act-cutter`, to `ids.dm`, as
    `ACTOR_KIND_NON_PARTY`. No target-role check in
    `validateGrantActorControl` refuses a DM; `asDM` fails the test on any
    refusal, which settles it. The DM types `Grimble` in `say-as` and `Hello`
    in `say-text` and presses Enter. The feed shows `Grimble: Hello`,
    `say-text` reads `""` and `say-as` reads `Grimble`. `Bye` then shows
    as `Grimble: Bye`.
- **Order:**
  1. `task build:client`.
  2. `task e2e` whole, after checking the machine's load.
  3. Commit `cmd/vtt/webdist` with the specs.
- **True when:**
  - every spec passes, E1 and E2 among them;
  - the last screenshot shows E1's fields beside the toast.

### Task 5: the report

- **Do:** write `docs/reports/2026-10-29-a-refused-command-keeps-what-was-typed.md`
  and give it its reading review.
- **It covers:**
  - each "Done looks like" item with its observation;
  - K1 to K12, with what went red;
  - Rule 9;
  - G1 to G8, and Q1 to Q3's answers;
  - P3 and P4's measurement, the one the comments no longer carry;
  - Phase 4a's adjudications, under their own heading.

  Leave room for the gate's figures.
- **True when:** it answers the ticket's eight items in its numbering, each
  `[x]` naming its observation.

### Task 6: the specification

With the `specification` skill, from the report, SPEC-022:
- in the paragraph that lists what holds the client, "until a narration is
  sent" becomes: `say-text` keeps what was typed until the server accepts a
  line, and `say-as` until it is changed; and that sentence's range
  "(VTT-491 to VTT-494)", which spans the withdrawn VTT-493, becomes VTT-491,
  VTT-492, VTT-494, VTT-518 and VTT-520 to VTT-522;
- in the repaint paragraph, `sayAs` and `sayText`, "which Send, or Enter
  in `say-text`, empties when it sends a narration (VTT-493)", is restated
  per C4 to C6;
- the console's draft clears when the server accepts the command it fed,
  and a refusal leaves it (C1, C3). `rerender` is named;
- "Saying something" gains that `say-as` stays after Send (C5);
- the one-press rule (C7), for the console's controls and for Send and
  Enter;
- "The console's rewrite": Describe clears its row's drafts when the
  server accepts it;
- the Requirements list loses VTT-493 and gains C1 to C7.

**True when:** each changed sentence has been run against the code by
someone other than its writer; `git grep -n "VTT-493\|when it sends a
narration" docs/specifications` prints nothing; and the ids of the sentence
that named "(VTT-491 to VTT-494)" are read, since a range split across a
line break hides VTT-493 from that grep.

### Task 7: the ledger, the gate, whole, and the commits

- **Do:**
  - Run `python3 tools/check-comments.py --write-ledger` once, then
    `python3 tools/check-comments.py main`. Commit the ledger (D14).
  - Check free space against `MIN_FREE_BYTES` (P12), and the machine's
    load.
  - Run `task check` whole on the last code commit. Launch it with
    `start_new_session=True`, send all output to a file, and require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Write the figures into the report. The client's mutation was not run,
    by the owner's rule of 2026-10-26. Say what the step printed.
  - Commit the report, then the specification.
  - Tell the owner D15's reminder.
- **True when:**
  - the gate is green, `check:mutation`, `check:comments`,
    `check:new-prose` and `check:requirements-chain` included;
  - `check:ts-mutation` printed that the client's mutation was not
    measured, naming `db81c37` and this ticket;
  - the log holds no Stryker line.
- Push and merge on the owner's word.

## For Phase 4a

Hand the QA agent:
- the allocated rules' texts, C1 to C7 (and C8 under Q1's A);
- SPEC-022's changed paragraphs;
- the surfaces, as signatures and names, never the source:
  - `renderDMConsole(d: DMDeps): HTMLElement`, with `DMDeps`'s members
    (`send` resolving to a `CommandResult`, and `rerender`);
  - `renderPlayerPanel(st, me, abilities, ui, send, rerender, switcher?)`,
    and `PlayerUIState`;
  - `renderSpectator(root, st, log, status, extras)`, for a repaint;
  - `CommandResultSchema` and `create` for an answer;
  - each form's `data-field` and `data-action` names, as SPEC-022 and the
    console draw them;
  - `client/test/support/dom` and `client/test/support/answers.ts`.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- measure with `env -u CLAUDECODE bun test <file>`;
- the console's typed text and its locks persist across renders in one
  bun process: release every held answer inside its test, and never
  leave a draft another test could read;
- an answer is awaited, never assumed, and a "nothing was sent" assertion
  comes with a press that sends;
- its tests go in `client/test/qa-refused-command.test.ts`, each citing
  its rule's id. The tree is hashed before and after it.
