# Plan: the console's Delete note asks the DM first

- **Ticket:** `docs/superpowers/specs/2026-11-23-delete-note-asks-first-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-23 by an agent that did not write the ticket, against
    `d1ff728` on `fix/delete-note-asks-first` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker, `task build:client` and `requirement-id` were not
    run. M1 ran in the worktree. M2 to M7 ran in a copy of the tree made by
    `git archive HEAD` outside the repository, made a git repository of its
    own so that `check-comments.py` and `check-new-prose.py` could diff
    against its base. `git status` holds the ticket and this file alone. The
    rest are searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-23-delete-note-asks-first.md`.
- **Base:** `d1ff728`. **Branch:** `fix/delete-note-asks-first`.
- **Owner's rulings taken in:**
  - 2026-11-23: told that Delete note is the one console removal that takes
    away something the DM wrote, and that it could ask first, naming the
    note, and send through `sending`: "ok, kör den".
  - Standing: an almost theoretical edge case with a small consequence gets
    no code, no test and no debt; one source per fact, pointed at rather
    than copied; no counts or measured figures in anything written down; a
    code comment under `client/src` is a warning or a pointer; the client's
    full mutation run only when the owner asks; push and merge on his word.
- **Signed off:** 2026-11-23 by the owner: OD-1 A ("Titeln, annars
  nyckeln"), OD-2 A ("…its title and text go"), and the plan ("Godkänd").
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off: R1
  VTT-956, R3 VTT-957, R4 VTT-958, R5 VTT-959, R7 VTT-960, R8 VTT-961, R9
  VTT-962, R10 VTT-963. The sort refused R2 and R6 as recommended below, and
  each candidate under "Candidates the sort may refuse".

## The verdict, check by check

1. **Every path resolves. By command.**
   - `client/src/view/dm.ts`: the Notes group, `noteKey` (`note-key`), Save
     through `sending(d, "upsert-note", …)` and Delete as a plain press,
     `button("Delete", () => {`, calling `d.send(deleteNote(…))` with the
     notice `name the note to delete`; `sending`, `unanswered`, `clearDraft`;
     `d.confirm` called by Remove actor, Remove token, Make spectator, Boot and
     New link.
   - `Note` (`Title`, `Text`, `UpdatedSeq`, `Visibility`) and `State.Notes` in
     `client/src/state.ts`; `noteDeleted` in `client/src/fold.ts`, which
     deletes the key and throws on one it does not hold.
   - The server: `delete_note` becomes one `NoteDeleted`
     (`internal/gateway/convert.go`); the `NoteDeleted` arm of `Apply`
     (`internal/engine/apply.go`) refuses a key no note holds
     (`engine: note %q not present`).
   - VTT-523, VTT-524, VTT-525 (withdrawn, naming Delete note among the
     controls that do not catch a rejected send), VTT-526, VTT-538, VTT-950,
     VTT-952 in `docs/requirements.md`; `client/test/dm-view.test.ts`;
     `cmd/vtt/webdist`; `docs/specifications/022-a-client-holds-one-log.md`.
   - In `~/dev/RPTool/maptool`'s `i18n.properties`:
     `LookupTablePanel.confirm.delete` (`Delete table "{0}"?`), used by
     `LookupTablePanel` with the table's name, and `confirm.macro.delete`
     (`You are about to delete the {0} macro.`), used by
     `MacroButtonPopupMenu` with the button's label. Neither sits behind a
     preference: a search of `AppPreferences` for `WarnWhenDeleted` finds
     `tokensWarnWhenDeleted` and `drawingsWarnWhenDeleted` alone. A search of
     the language file's keys for `note` finds notes only on a token's edit
     dialog (`EditTokenDialog.tab.notes`, `EditTokenDialog.tab.gmnotes`) and
     in campaign-export warnings.
2. **"Done" is an observation. By command where it could be.**
   - Item 1 names a test still to be written. M2 shows today's Delete note
     asks nothing and sends with a `confirm` that answers no; M4's prototypes
     of item 1's tests fail on `d1ff728`'s `dm.ts`.
   - Item 2 holds today, which asks nothing at all. It guards the change (B2)
     rather than describing today, and its evidence is partial (G1).
   - Item 3 names tests still to be written. M2 shows a second press while
     held sends again and a rejected send goes unhandled; M4's held-press and
     rejection prototypes fail today. Its "once that command is answered it
     sends again" holds today, having no lock to free (G5).
   - Item 4 is the gate.
3. **Each rule is breakable. By reading.**
   - "Asks the DM first, in a message naming the note, and sends nothing when
     the DM declines": red under B1, and under B4 to B6 and B8 once "naming
     the note" is defined, which waits on OD-1 (the ticket says so itself).
   - "While a `delete_note` … is unanswered, pressing it again sends
     nothing": red under B10 and B14.
   - "A `delete_note` … whose promise rejects frees it and is reported with
     `console.error`": red under B10 and B13.
4. **The scope matches the claim. By command, then reading.**
   - `git grep` for `delete-note`, `deleteNote`, `delete_note` and the label
     `"Delete"` over `client/`: the one control in `dm.ts`, built by
     `deleteNote` in `client/src/commands.ts`. The tests that press it are all
     in `client/test/dm-view.test.ts`: "each guard refuses with its own exact
     wording", "deleting a note trims its key" and "a whitespace-only id is
     refused everywhere one is required". `client/test/command-surface.test.ts`
     names the action and presses nothing; `client/test/commands.test.ts`
     builds the command and renders no console. No `FORMS` entry presses it.
   - `client/e2e` (less `client/e2e/.report`): `git grep` for `delete-note`,
     `deleteNote`, `"Delete"`, `note-key` and `dialog` finds no spec that
     presses Delete note. `handover.spec.ts` types into `note-key` and presses
     Save. The `dialog` handlers are in `booting.spec.ts`, `handover.spec.ts`
     and `joining.spec.ts`.
   - M3: the whole client suite and the typecheck stay green under the change.
   - The ticket's "What it touches" leaves out records (G3).
5. **No recorded decision is contradicted silently. By reading.**
   - VTT-533 holds an accepted command's notes-form key, title and text as
     typed. D3 keeps `note-key` as typed after an accepted Delete note, as
     today, so the row stands.
   - VTT-525's withdrawal named Delete note as a control that does not catch
     a rejected send. The change makes it catch one; the withdrawn row stays
     the record of the past, and R9 states the new rule.
   - The opening comment of `dm.ts` names a sufficient reason to ask first,
     not the only one, and stays true.
   - SPEC-022's "What a command's answer leaves typed" and the paragraphs
     after it list the controls that send through `sending`; the ticket names
     SPEC-022. SPEC-013 (who may issue `delete_note`) and SPEC-016 (which
     windows hold a note) stand.
   - `tools/ts-mutation-equivalents.txt`: no entry is keyed on a line the
     change edits. The entries on the literal lock keys of `sending` (the
     block headed by the full run of 2026-10-30) each argue that "the seven
     other literal keys are non-empty and differ from each other"; the change
     adds a literal key, so that clause goes false as a count while its
     argument holds (D7).
6. **The records the work moves are named, and reachable. By command, then
   reading.**
   - The section is present and its one path resolves.
   - A search of `docs/specifications/` for `Delete note`, `delete-note`,
     `deleteNote`, `delete_note`, `note-key`, `Save note`, `confirm` and
     `asks first`: SPEC-022 moves (the Status sentence; "What a command's
     answer leaves typed", whose list and whose notes-form exception gain
     Delete note; "One press, one command", the drawing and the rejection
     paragraphs, which gain the new ids; a paragraph beside Remove token's;
     the "## Requirements" list). SPEC-013's `delete_note` row and SPEC-012's
     note-key residue (adventure compilation) describe the server and do not
     move; SPEC-009's Boot does not move.

## Notes on the problem statement, not failures

None changes what is built. The writer may correct them before Task 0's
commit; the report and SPEC-022 must not repeat them as they stand.

- **N1. "Removing a condition, taking an ability and revoking control … each
  is put back with a press or two"** is false for a condition. The console has
  no control that applies one, and `ClientCommand` in
  `contract/vtt/v1/commands.proto` has `remove_condition` and no command that
  applies a condition but `use_ability`. Putting a removed condition back
  takes an actor that holds an ability applying it, using it on the right
  target. Taking an ability (Give) and revoking control (Grant) are put back
  from the console. Whether Remove condition should ask is not this ticket's;
  it is worth putting to the owner.
- **N2. "The rows that name the controls `sending` holds, VTT-523, VTT-524,
  VTT-526 and VTT-538, do not name it"** is true of Delete note, but those
  lists never named every control `sending` holds: a row's Give
  (`give-ability:`) and an ability's take chip (`take-ability:`) send through
  `sending` today and are in none of the four, though SPEC-022's "What a
  command's answer leaves typed" names both. Not this ticket's; recorded so
  that the rows below are not read as completing those lists.
- **N3. "Nothing in the client shows them again"** is true of the client. The
  log keeps the title and text, and the agent reads them back with MCP's
  `get_events_since` (`internal/mcp/read_tools.go`). This bears on OD-2.

## The gaps this plan inherits

- **G1. Item 2's evidence observes "asks nothing" only for the empty
  field.** "each guard refuses with its own exact wording" asserts no
  question for every guard, Delete note's included. In "a whitespace-only id
  is refused everywhere one is required" the Delete note arm asserts only
  that nothing was sent; under B2 it stays green. D8 adds the notice and the
  question count.
- **G2. Item 1 does not say how the note is named.** The ticket's first open
  point: D2 and OD-1.
- **G3. Records the ticket does not name:**
  - `tools/ts-mutation-equivalents.txt`: D7;
  - `tools/comment-ceilings.txt`: binding, and no row moves (M6, D6);
  - the Phase 4a file, a new `client/test/qa-*.test.ts`;
  - the report.
- **G4. The ticket's "What could not be established"** travels: the naming
  is D2 and OD-1; the words are D4 and OD-2; how the rows take Delete note
  in is D9.
- **G5. Item 3's "once that command is answered it sends again"** holds
  today, which has no lock. The held-press test asserts it after the release
  all the same, so that it is red under a lock that is never freed.

## Measurements this plan stands on

- **M1. The base is green.** In the worktree, from `client/`,
  `env -u CLAUDECODE bun test test/dm-view.test.ts
  test/qa-remove-token-asks-first.test.ts test/qa-remove-asks-first.test.ts
  test/command-surface.test.ts test/qa-refused-command.test.ts
  test/qa-note-console.test.ts` passed whole.
- **M2. Today's Delete note.** A probe in the copy rendered
  `renderDMConsole` over a state holding a titled note, with `confirm`
  answering no. Delete note with that key typed asked nothing and sent one
  `deleteNote`. With the send held, a second press sent a second
  `deleteNote`. With the send rejecting, nothing reached `console.error`, and
  bun reported the rejection as an unhandled error.
- **M3. The change turns no existing test red.** In the copy, Delete note's
  press was moved into `sending(d, "delete-note", …)`: the empty check
  first, then the lookup, then the question through `d.confirm` naming the
  note by OD-1's recommended chain in OD-2's recommended words, a no
  returning nothing, a yes returning `{ command: deleteNote(key), clears:
  [] }`; one warning line and a citation line above it. The whole client
  suite (`env -u CLAUDECODE bun test test` from `client/`) and
  `bunx tsc --noEmit -p client/tsconfig.json` (what `task client:typecheck`
  runs) both passed.
- **M4. The new tests fail today.** Prototype tests in the copy's
  `dm-view.test.ts` covered:
  - a no, over a titled note, a note with an empty title, a padded key and a
    key the state holds no note of;
  - a no keeping `note-key` at the next paint;
  - a yes sending the trimmed key after one question;
  - a second press while held, then a press after the answer;
  - a rejected send;
  - a `FORMS` entry for Delete note, walked by the existing tests;
  - D8's assertions.
  All passed on the change. On `d1ff728`'s `dm.ts`, these failed: the no,
  the field-kept, the yes (its question count), the held-press and the
  rejection tests, and "a control whose command is unanswered sends nothing
  when pressed again, each row its own". Today's unhandled rejection also
  failed a neighbouring test, "the guide button shows the guide it fetched".
  The other walks and D8's assertions passed today: they guard the change.
  With the prototypes on the change, `bun test --coverage` held `dm.ts`
  whole, as `tools/ts-coverage-thresholds.txt` requires.
- **M5. The breaks the tests catch.** Each was applied to the change in the
  copy, run, and restored from a saved copy compared byte for byte. Each
  turned a named test red:
  - B1 drop the question;
  - B2 ask before the empty check (the guard test, and the whitespace test
    through D8's assertion);
  - B3 ask in a wrapper outside `sending` (the held-press test: a held press
    asks again);
  - B4 show the key always;
  - B5 `??` for `||` (a fold's empty title is `""`, so a titleless note is
    shown as nothing);
  - B6 look up the untrimmed field;
  - B7 clear the draft on a no;
  - B8 drop the `?.` on the lookup (a key the state holds no note of throws);
  - B9 send the untrimmed key on a yes;
  - B10 send outside `sending` (the held-press and rejection tests, and the
    lock walk);
  - B11 clear `note-key` on an accepted answer (the walk "an accepted command
    clears the fields of its own form and of no other", VTT-533);
  - B12 key Delete note's lock as Save's (the lock walk and the drawing
    walk: VTT-541);
  - B13 ask for a repaint at the press (the drawing and refusal walks, and
    the rejection test);
  - B14 send outside `sending`, repainting at every answer and reporting a
    rejection (the refusal walk, the lock walk and the held-press test);
  - B15 check the untrimmed field for empty (the whitespace test).
- **M6. The comment ceiling.** `client/src/view/dm.ts` measured with
  `check-comments.py`'s own `measure`, and `check-comments.py` run against
  the copy's base: with M3's one warning line, the file stays within the band
  under its ceiling and the run is clean, so no row moves. With the two-line
  warning Remove actor and Remove token carry, the file reaches its ceiling
  with no room left: it is clean only while the press keeps the number of
  code lines M3 gave it, and one code line fewer puts it above the ceiling
  with an added comment line, which `check:comments` refuses.
  `check-new-prose.py` was clean on the change.
- **M7. The adjudications the change touches.**
  `tools/ts-mutation-equivalents.txt` keys no entry on Delete note's lines.
  The entry on `button("Save", sending(d, "upsert-note", () => {` names a line
  the change leaves byte for byte. The entries on the literal lock keys share
  the premise that `sending` and `unanswered` are what they are, and each
  holds the clause "the seven other literal keys are non-empty and differ
  from each other". The entry on the console's `namer(...)` line rests on the
  console reading nothing of the namer but `resource` and `condition`; the
  question reads `d.st.Notes` directly. The entry on the take chip's
  `clears: [],` rests on the spelling of every draft key, and the change adds
  none. `tools/ts-coverage-thresholds.txt` holds `dm.ts` at full coverage.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened or dropped; the existing
  Delete note tests gain assertions. No ledger row is written to dodge a
  finding, and no comment or code line is added or removed to dodge one.
- **P2. Rules 3, 4 and 5.** No contract, server or fold change; "note",
  "key" and "title" are the platform's words.
- **P3. Rules 8 and 10.** Names, never line numbers. A comment under
  `client/src` is a warning or a pointer, and no added comment line carries a
  banned term.
- **P4. Rule 9, answered in the ticket and checked** (check 1). MapTool has
  no campaign notes. Its two named pieces of user writing, a lookup table and
  a macro, ask before deletion, naming the table by its name and the macro by
  its label, behind no preference. Borrowed: asking before a named piece of
  writing is deleted, naming it by what the user sees it called, and asking
  every time. Not borrowed: the macro's "Do you wish to continue?", which the
  console's other questions do without.
- **P5. SPEC-020 and SPEC-023.**
  - `sending` and `unanswered` are not edited, and Save's line
    `button("Save", sending(d, "upsert-note", () => {` stays byte for byte, so
    every entry keyed in the Notes group holds without re-judging.
  - The console reads nothing new of `namer`.
  - The change brings two survivors the next full run will report, each of
    a kind already adjudicated: the key `"delete-note"` emptied, as for the
    other literal lock keys, and the `[]` of Delete note's `clears` filled,
    as for the take chip's. Neither is adjudicated before a run has produced
    it: an entry excuses a survivor, and one written ahead of the run that
    names it widens an exemption no run has shown is needed (rule 2). The
    report names both for whoever answers that run.
  - The client's stored mutation report goes out of date with the code
    commit; `task check` names that and passes; a full run is the owner's to
    ask for.
- **P6. The mechanical gates.**
  - The code commit carries `cmd/vtt/webdist` rebuilt by `task build:client`
    (`check:drift` compares it).
  - bun runs under `env -u CLAUDECODE`, from `client/`, as `bun test test`;
    a bare `bun test` loads the Playwright specs. `task client:typecheck`
    passes.
  - `python3 tools/check-new-prose.py main` and
    `python3 tools/check-comments.py main` run before each commit.
  - Stage and commit in separate calls.
  - `Array.from` over a `NodeList`.
  - Commands are started with `/bin/sh`, never zsh.
  - `go clean -cache` before and after the gate.
  - Python run from the worktree leaves a `tools/__pycache__/`. Remove it
    before `review-record`, which refuses untracked files.
  - The code task gets Phase 4a's QA and one reviewer before its commit.

## Decisions

1. **D1. Delete note sends through `sending`, keyed `delete-note`, its own
   lock.** The question is asked inside the press `sending` hands, after the
   empty check, before the command is returned; a no returns nothing.
   - **Forced by:**
     - item 3 and the ticket's two lock rules, which `sending` already keeps
       for its other controls: a lock taken when the command is sent and
       freed at any answer or a rejection, and `console.error` for a
       rejection;
     - item 2 (a blank key asks nothing, so the check comes first);
     - the lock (`sending` stops a held press before it runs the press, so a
       held press asks nothing; B3 breaks it);
     - VTT-541 (one control's command holds back no other): its own key,
       never Save's, though the two share `note-key` (B12);
     - P5 (`sending` and `unanswered` untouched). The precedents' D1.
   - **A consequence named, not a choice:** an accepted Delete note now asks
     for one repaint, as every control behind `sending` does. `app.ts`'s
     `act` already paints at every answer, so nothing the DM sees changes.
2. **D2. The note is named from the console's own state, by OD-1's chain.**
   Recommended (OD-1 A): its title, else the trimmed key. As prototyped:
   `d.st.Notes[key]?.Title || key`, where `key` is the trimmed field.
   - **Forced by:**
     - the window's own naming of a note: `renderNotes` in
       `client/src/view/spectator.ts` heads each note by its title, else its
       key;
     - blueprint principle 13: no identifier where a name exists;
     - the console's convention of a name, else the id (VTT-950, VTT-952);
     - MapTool naming a macro by its label and a table by its name (P4).
   - `?.`, because the key typed may name no note the state holds (B8). `||`,
     not `??`, because a fold's empty title is `""` (B5).
3. **D3. A yes clears nothing: `clears: []`.** `note-key` stays as typed
   after an accepted Delete note, as today.
   - **Forced by** VTT-533 (an accepted command leaves the notes form's key
     as typed) and the ticket ("as today"). B11 breaks it.
4. **D4. The words are OD-2's.** Recommended: `Delete the note "<note>"? Its
   title and text go with it.`
   - **Forced by** the console's shape for a question, "<verb> <name>? <what
     is lost>.", as in Boot's, Make spectator's, Remove actor's and Remove
     token's; and by N3: the log keeps the title and text and the agent reads
     them back, so a question saying nothing brings them back overstates.
5. **D5. A key the window holds no note of is asked by that key and sent on a
   yes;** the console does not refuse it.
   - **Forced by** item 1, which asks it, and by the server being the
     authority: `Apply` refuses an unknown key and the refusal is shown as the
     toast (VTT-604). A window that has not yet folded a note the agent just
     wrote would otherwise refuse a real one. The precedents' D4.
6. **D6. The comment above Delete note is one warning line and one citation
   line:** `// Ask inside the press \`sending\` runs, after the empty check
   (SPEC-022).`, then the new ids.
   - **Forced by** M6: the two-line warning Remove actor and Remove token
     carry would fit under `dm.ts`'s ceiling only while the press keeps its
     prototyped length, so the comment's length would be settled by the
     code's. One line keeps the warning (where the question sits, which B2
     and B3 break) and points to SPEC-022's paragraph for why; a citation line
     counts in no share. This is the file's whole comment budget for this
     change: a further comment line in `dm.ts` must be paid for by removing
     one, never by adding code.
   - No ledger row moves (M6). If the final tree, after the review's fixes,
     falls more than the band under the ceiling, `python3
     tools/check-comments.py --write-ledger` runs once, as the last edit
     before staging, and the lowered row is committed with the code.
7. **D7. The reasons on the literal lock keys drop their count.** In each
   entry of `tools/ts-mutation-equivalents.txt` that holds it, "the seven
   other literal keys are non-empty and differ from each other" becomes "the
   other literal keys are non-empty and differ from each other". No key, code
   line, header or other word of the file changes, and no entry is added
   (P5).
   - **Forced by** check 5 and M7: the change adds a literal key, so the
     count goes false the moment it lands, while the argument it carries (no
     other call's key is `""`) still holds; and by the owner's standing rule
     to state the invariant, not the count. The checker reads keys, not
     reasons, so no gate catches the false count.
8. **D8. Existing tests: assertions added, none changed.**
   - "deleting a note trims its key" asserts one question, and settles its
     send so that its lock is freed inside the test (`support/answers.ts`:
     locks outlive the test) (R3).
   - "a whitespace-only id is refused everywhere one is required" asserts the
     notice and no question on its Delete note arm (G1, R5).
   - "each guard refuses with its own exact wording" already asserts no
     question for every guard; it and the whitespace test gain R5's id on
     their citation lines.
   - `FORMS` gains a "Delete note" entry after "Save note": it types
     `note-key` with the value Save note's entry types, clears nothing, and
     presses `delete-note`. The walks then take it in without change: the
     refusal walk (VTT-517, VTT-538, R10), the acceptance walk (VTT-533, B11),
     the lock walk (VTT-523, VTT-541, R7), the drawing walk (VTT-524, R8) and
     the guard walk.
     - **One value in the shared box,** because the lock walk and the
       acceptance walk enter every form's typed values into one console. With
       two values `note-key` would hold whichever was entered last, and Save's
       press in the lock walk would send Delete's key: still green, but
       dependent on the table's order.
   - The walks' citation lines gain the new ids beside the old.
   - **Forced by** G1, P1 and D9.
9. **D9. Delete note earns rows of its own; VTT-523, VTT-524, VTT-526 and
   VTT-538 stand as worded.**
   - **Forced by:**
     - the four rows stay true: each states its property for the controls it
       lists and claims no control beyond them (N2 shows they never listed
       every control `sending` holds);
     - withdrawal and succession would move every citation of the four.
       VTT-523 alone is cited by comments in `client/src/view/dm.ts`,
       `client/src/view/player.ts`, three e2e specs
       (`handover.spec.ts`, `speaking.spec.ts`, `whispering.spec.ts`),
       `dm-view.test.ts`, `spectator-view.test.ts` and the QA files of the
       panel's Say, the refused command, the refused line and both Removes,
       by SPEC-022 and by the reasons on the literal lock keys; and its
       successor would have to carry the player panel's clause with them.
       VTT-524, VTT-526 and VTT-538 are cited in several of the same files.
       Each would have to be re-pointed, or left naming a withdrawn row;
     - new rows move no citation: the walks' comment lines gain an id;
     - precedent: the console's Say has rows of its own (VTT-535 to VTT-537)
       beside VTT-523, VTT-524 and VTT-526, and SPEC-022 cites each pair.
   - **Its cost, named:** the lock's rule is worded once for the listed
     controls and once for Delete note. SPEC-022 states it once and cites
     both, as it does for the Say.
10. **D10. No e2e test.**
    - **Forced by** check 4: no spec presses Delete note, so the suite meets
      no new dialog. `confirm: (m) => window.confirm(m)` in
      `client/src/app.ts` is one wiring shared with Remove actor, Remove
      token, Boot, Make spectator and New link; `booting.spec.ts`,
      `handover.spec.ts` and `joining.spec.ts` drive it through a real
      dialog. The precedents' D6.
11. **D11. The question has no "don't ask again".**
    - **Forced by** P4: MapTool asks before deleting a table or a macro
      behind no preference; and by the precedent's OD-3 A: `window.confirm`
      has no third answer and the client keeps no settings to turn a question
      back on.

## Open decisions for the owner

- **OD-1. How does the question name the note?** (The ticket's first open
  point. It changes what the DM is told, so it is his.)
  - **A (recommended). Its title, else the key.** The DM's Notes panel heads
    a note by its title, else its key (`renderNotes`), so the question names
    the note as the window does. The key stands in where the note has no
    title and where the window holds no note of that key, so the question
    always says which note the typed key named. Principle 13 puts the title
    before the key.
  - **B. The key alone.** Exactly what was typed, and what the Events panel
    names a deleted note by (`note "<key>" withdrawn`, `describe` in
    `client/src/view/spectator.ts`). But it shows the identifier where the
    note has a title, and does not show that the key matched the note the DM
    meant: the question is the one place the typed key meets the note. B4 is
    this order.
  - **C. Its title and its key, when it has a title.** For example
    `"The Ravine" (ravine)`. It tells apart two notes of one title, which A
    reads alike (titles are not unique; keys are). It puts an identifier
    beside a name, against principle 13's letter, and lengthens every
    question for a case the typed key already settles.
- **OD-2. The question's words.** (The ticket's second open point.)
  - **A (recommended).** `Delete the note "<note>"? Its title and text go
    with it.`
    - The console's "<verb> <name>? <what is lost>." shape; "go with it" as in
      Remove token's question.
    - It says what is lost and claims nothing about getting it back: the log
      keeps both, the agent reads them with `get_events_since`, and a Save
      under the same key makes the note again.
    - The quotes mark where a title begins and ends, since a title is a
      phrase or a sentence ("Who killed the baron"), as MapTool's
      `Delete table "{0}"?` and the Events panel's `note "<key>"` do.
  - **B.** `Delete the note "<note>"? Every window stops showing it, the
    players' too when they may read it.` It names who loses it, as Boot's and
    New link's questions name who is affected; longer, and it leaves what is
    lost to be inferred.
  - **C.** `Delete the note "<note>"? Nothing brings back its title and
    text.` It echoes Remove actor's words, but overstates (N3), which the
    ticket's own open point weighs.
  - Under any of these, the quotes may be dropped, as Remove actor's and
    Remove token's questions have none; then a title reads run into the
    sentence.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the register's
words are the ones that bind. Every new row reads `**OPEN — no test yet**`
until Task 1 cites its tests. Rows that a press the console's lock can hold
back carry "while no `delete_note` it sent is unanswered", the clause whose
absence withdrew VTT-946 and VTT-949: a held press never runs the press
body.

| R | Item | Rule (as drafted for OD-1 A; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | The console's Delete note, pressed with a key in `note-key` that is not blank while no `delete_note` it sent is unanswered, asks the DM first, in a message holding the title the console's state holds for the note of that key, or the key when it holds no title for it, and sends nothing when the DM declines. | B1; B4, B5, B6, B8 |
| R2 | 1 | The console's Delete note question reads OD-2's words. | a reworded question |
| R3 | 1 | A Delete note the DM accepts sends one `delete_note` naming the trimmed key. | B9; B1 |
| R4 | 1 | A Delete note the DM declines leaves `note-key` as typed, at the next paint too. | B7 |
| R5 | 2 | The console's Delete note, pressed with `note-key` empty or only white space while no `delete_note` it sent is unanswered, shows `name the note to delete`, asks the DM nothing and sends nothing. | B2, B15 |
| R6 | D1 | While a `delete_note` the console's Delete note sent is unanswered, pressing Delete note again asks the DM nothing. | B3 |
| R7 | 3 | While a `delete_note` the console's Delete note sent is unanswered, pressing Delete note again sends nothing. | B10, B14 |
| R8 | D1 | A press of the console's Delete note that sends a `delete_note` changes nothing the console draws until that command is answered. | B13 |
| R9 | 3 | A `delete_note` the console's Delete note sent, whose promise rejects, clears nothing, asks for no repaint, frees Delete note, and is reported with `console.error`. | B10, B13 |
| R10 | D1 | When the server refuses a `delete_note` the console's Delete note sent, the console asks for no repaint of its own. | B13, B14 |

**Recommended to the sort, as the precedents' sorts ruled:** refuse R2
(presentation: VTT-950 and VTT-952 pin a name and no words, and the words
are pinned by the tests) and R6 (a consequence of D1's place behind the lock;
B3 still runs as a deliberate break, and the held-press test cites R7). Keep
R8 and R10 beside R7 and R9: SPEC-022's paragraphs on drawing and on a
refusal name "those controls", which will include Delete note, and each cites
the rows that hold it, as for the Say (VTT-536).

**Candidates the sort may refuse:**
- **A row that the agent's `delete_note` over MCP asks nothing.** Refused: the
  ticket's non-goal.
- **A row that an accepted Delete note keeps `note-key`.** Refused: VTT-533
  states it for the notes form's key whichever control fed it, and the
  acceptance walk holds it for Delete note (B11).
- **A row that the lookup is by exact key.** Refused: how, not what; R1's
  test, with a padded key, holds it.
- **A row that Delete note holds back no other control.** Refused: VTT-541
  states it for every console control, and the lock walk holds it (B12).
- **A row that a title of white space alone, or one holding quotes, is shown
  as written.** Refused: an almost theoretical edge case with a small
  consequence.

**No existing row becomes false.** Checked and left standing:
- **VTT-523**, **VTT-524**, **VTT-526** and **VTT-538** (D9).
- **VTT-517**, **VTT-533** and **VTT-541**, which state their rule for every
  console control and gain Delete note through the walks.
- **VTT-229** (Save sends no `upsert_note` until who may read it is chosen):
  Save is untouched.
- **VTT-604** (a refusal shown as the toast) and the server's **VTT-344**.
- The withdrawn **VTT-525**, which stays the record of the past.
- The precedents **VTT-569**, **VTT-901**, **VTT-902** and **VTT-947** to
  **VTT-955**.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R10 and the
  candidates, as OD-1's and OD-2's answers leave them. Allocate ids with
  `requirement-id`. Fill in this plan's sign-off and ids. Commit the ticket,
  this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: Delete note asks first (R1, R3 to R5, R7 to R10; D1 to D9; items 1 to 3)

- **Files:**
  - `client/src/view/dm.ts`: Delete note's press and the two comment lines
    above it;
  - `client/test/dm-view.test.ts`: a fixture of notes, the new tests beside
    "deleting a note trims its key", D8's assertions, the `FORMS` entry and
    the citation lines;
  - `tools/ts-mutation-equivalents.txt`: D7;
  - `docs/requirements.md`: the rows' cells;
  - a rebuilt `cmd/vtt/webdist`;
  - `tools/comment-ceilings.txt` only if D6's last step writes it.
- **Tests first, seen red where today differs:**
  - A fixture over `newState()` holding a note with a title and a note whose
    title is `""`, as the fold writes it.
  - Through `harness` with `setConfirm(false)`: the titled note's key, the
    same key padded, the titleless note's key and a key the state holds no
    note of are each asked once, `confirmMessages()` exactly the question
    with the name D2 gives, and nothing is sent or shown (R1). Red today.
  - Typed through `input`: after a no, `note-key` holds what was typed, in a
    fresh `harness()` too (R4). Red today.
  - With the default yes: "deleting a note trims its key" asserts one
    question and settles (R3). Red today.
  - With `answer: "held"`: a second press before `release` asks nothing and
    sends nothing; after the answer and a settle, a press asks and sends
    again (R7). Red today.
  - With a send that rejects, the shape of "a send that rejects lets its
    control go, clears nothing, and reports the error": two presses, each
    settled, send twice, ask for no repaint, leave `note-key` as typed in a
    fresh harness, and reach `console.error` once each (R9). Red today.
  - The `FORMS` entry (D8). The lock walk is red today; the other walks are
    green today and hold B11 to B14.
  - D8's assertions (R5). Green today; they hold B2 and B15.
  - Every test that types into `note-key` empties it in a `finally`: `draft`
    outlives the harness.
- **Code:** D1 to D6, as M3 prototyped.
- **True when:**
  - the new tests pass, and those marked red failed before;
  - the whole client suite, `task client:typecheck` and P6's checks pass;
  - the comment above Delete note is D6's two lines and carries none of the
    banned terms `check:comments` names;
  - Save's line named in P5 is unchanged, and `sending` and `unanswered` are
    untouched;
  - `python3 tools/check_ts_mutation_test.py -q` and
    `python3 tools/check_ts_mutation_qa_test.py -q` pass after D7's edit;
  - `task check:requirements-chain` passes.
- **Deliberate breaks** (M5), each turning a named test red, then restored:
  B1 to B15, less whichever OD-1's answer makes the code (B4 under B).
- **The state here:** the DM is asked before the console deletes a note, by
  the name the Notes panel gives it; a no leaves everything as typed; a
  second press while the first is unanswered does nothing; a rejected send is
  reported.

### Task 2: the gate (item 4)

- **Do:**
  - `go clean -cache`; check free space and the load (`os.getloadavg()`).
  - Run `task check` whole on the code commit, detached under `/bin/sh`,
    output to a file, requiring its completion line.
  - Run `task e2e`.
  - `go clean -cache`.
  - Tell the owner the client's stored mutation report is out of date from
    Task 1, which commit it measured (SPEC-023), and that the next full run
    should find P5's two survivors.
  - If `check:comments` reports a share more than the band under a ceiling
    D6 did not cover, the gate is red: back to Task 1, and the row is written
    there, never on a green-seeking edit here.
- **True when:** `task check` and `task e2e` are green, and the embedded
  bundle is the code commit's.

### Task 3: the report, then the specification

- **The report:** `docs/reports/2026-11-23-delete-note-asks-first.md`, with
  the `implementation-report` skill and its reading review. It holds:
  - each "Done looks like" item with its observation;
  - the rows, and the break that turned each red;
  - P4 (rule 9);
  - G1 to G5 and the notes on the problem statement;
  - OD-1's and OD-2's answers;
  - P5's two expected survivors, each with the entry it would mirror;
  - D7's edit, and that no key changed;
  - Phase 4a's adjudications under their own heading;
  - whether D6's ledger step ran;
  - the gate.
- **SPEC-022,** with the `specification` skill, from the report:
  - "What a command's answer leaves typed": Delete note joins the controls
    that send through `sending`, beside the take chip as one that clears
    nothing; the notes-form exception names the notes form's key, title and
    text whichever of Save and Delete note fed it; Delete note is fed by
    `note-key` alone;
  - "One press, one command": its rows gain R7, and Save note beside Delete
    note joins the controls that share a box and not a lock;
  - the drawing and rejection paragraphs, and the refusal sentence, gain R8,
    R9 and R10;
  - a paragraph "The console's Delete note asks first", after Remove
    token's (D1 to D5 and the rows);
  - the Status sentence, after Remove token's, naming
    `client/test/dm-view.test.ts` and the QA file;
  - "Principles served" weighed for 13;
  - the "## Requirements" list gains the new ids.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the specification;
  push and merge on the owner's word.

## For Phase 4a

- **Hand the QA agent:**
  - the rows as the register words them, and D1 to D5;
  - the surface as signatures, never the source:
    `renderDMConsole(d: DMDeps): HTMLElement`, the `DMDeps` interface
    (`st`, `send`, `notify`, `confirm`, `rerender`, `roster`,
    `participants`), `State.Notes` and `Note`, and `newState` from
    `client/src/state.ts`;
  - `ok`, `refused`, `held` and `settle` from
    `client/test/support/answers.ts`, and `client/test/support/dom`.
- **Tell it:**
  - never run `task check`, `task e2e` or Stryker;
  - its file is a new `client/test/qa-*.test.ts`;
  - tests run from `client/` under `env -u CLAUDECODE bun test test/<file>`;
  - `Array.from` over a `NodeList`;
  - the console's typed text and its locks outlive a render in module state,
    so empty every field it types and release every held send in a
    `finally`;
  - a test named by a template cannot be cited by name in the register;
  - an empty title in a real state is `""`;
  - the tree is hashed before and after it.
- **Have it try:**
  - titles with spaces, punctuation and quotes;
  - a note with a title, one with none, and a key the state holds no note of;
  - a key padded with white space;
  - a note removed from the state between render and press;
  - Save and Delete note pressed in turn while each is held;
  - a blank press while a `delete_note` is held;
  - a refused and a rejected send after a yes.
- **Known, and not findings:**
  - a key the window holds no note of is asked by that key and sent (D5);
  - the agent's MCP `delete_note` asks nothing (non-goal);
  - two notes of one title read alike (OD-1);
  - `note-key` keeps its text after an accepted delete (VTT-533, D3);
  - a title of white space alone, or one holding quotes, is shown as
    written (refused candidate);
  - a blank press while held shows nothing (the lock clause in R5).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it to check:

- Save's line named in P5 is unchanged, `sending` and `unanswered` are
  untouched, and the console reads nothing new of `namer`;
- the press's order against D1, the chain against OD-1's answer, the words
  against OD-2's, and `clears: []` (D3);
- each comment under `client/src` is a warning or a pointer; the comment
  above Delete note is D6's two lines; no comment or code line was added or
  removed to fit the ceiling;
- D7 changed only the count clause in the reasons that held it, and no key,
  code line or header;
- no assertion was weakened or dropped (rule 2), and D8's assertions are
  there;
- the `FORMS` entry types the value Save note's entry types;
- every test that types into `note-key` empties it, and every held send is
  released;
- each scope claim in new prose names the search that bounds it.
