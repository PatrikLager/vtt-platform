# Plan: the console's Remove condition asks the DM first

- **Ticket:** `docs/superpowers/specs/2026-11-23-remove-condition-asks-first-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-23 by an agent that did not write the ticket, against
    `3ebb59c` on `fix/remove-condition-asks-first` in the worktree
    `vtt-platform-visibility`, stacked on the unmerged
    `fix/delete-note-asks-first`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker, `task build:client` and `requirement-id` were not
    run. M1 ran in the worktree. M2 to M7 ran in a copy of the tree made by
    `git archive HEAD` outside the repository, made a git repository of its
    own so that `check-comments.py` and `check-new-prose.py` could diff
    against its base, with the worktree's `node_modules` linked in. The rest
    are searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-23-remove-condition-asks-first.md`.
- **Base:** `3ebb59c`. **Branch:** `fix/remove-condition-asks-first`, which
  lands after `fix/delete-note-asks-first`.
- **Owner's rulings taken in:**
  - 2026-11-23: told that Remove condition does not ask and that a removed
    condition cannot be put back from the console: "ja, kan vi göra det här
    också, alltså condition, eftersom det är likadan logik?"
  - Standing: an almost theoretical edge case with a small consequence gets
    no code, no test and no debt; one source per fact, pointed at rather
    than copied; no counts or measured figures in anything written down; a
    code comment under `client/src` is a warning or a pointer; the client's
    full mutation run only when the owner asks; push and merge on his word.
- **Signed off:** 2026-11-23 by the owner: OD-1 A, and the plan "Godkänd".
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off: R1
  VTT-964, R3 VTT-965, R5 VTT-966, R6 VTT-967, R7 VTT-968, R8 VTT-969. The
  sort refused R2 and R4 as recommended below, and every candidate listed
  there.

## The verdict, check by check

1. **Every path resolves. By command.**
   - `client/src/view/dm.ts`: `renderDMConsole`'s "Remove condition" group,
     one `button` per condition with `data-action` `remove-condition`,
     labelled `${held}: ${names.condition(c.ID)} ✕`, whose press is
     `() => d.send(removeCondition(actor, c.ID))`; `sending`, the
     module-level `unanswered`; Remove actor, Remove token and Delete note
     pressing through `sending` and asking through `d.confirm`; the take
     chip keyed `take-ability:${a.actorId}:${id}`.
   - `Namer.condition` in `client/src/view/naming.ts`; the
     `conditionRemoved` arm of `client/src/fold.ts`'s fold, which throws on
     an actor it lacks and on a condition the actor does not hold.
   - `internal/engine/apply.go`'s `ConditionRemoved` arm of `Apply`, which
     refuses an unknown actor (`condition removed for unknown actor`) and a
     condition not held (`condition %q not present on actor %q`). Its
     `ConditionApplied` arm refuses one already applied, so an actor holds
     a condition once.
   - `client/e2e/conditions.spec.ts`, `cmd/vtt/webdist`,
     `docs/requirements.md`, `docs/specifications/022-a-client-holds-one-log.md`.
   - Rule 9: `ChangeStateAction` in `~/dev/RPTool/maptool`'s
     `TokenPopupMenu` sets or clears a token's state from a checkbox menu
     item and sends at once; a search of `i18n.properties` for a
     confirmation naming a state finds none. The ticket's claim holds. Not in
     the ticket, and of the same answer: the same menu's `ClearStateAction`
     ("Clear State", `defaultTool.stateAction.clear`) clears every state of
     the selected tokens at once, unasked.
2. **"Done" is an observation. By command where it could be.**
   - Item 1 names a test still to be written. M2 shows today's button asks
     nothing and sends on a `confirm` that answers no; M4's prototype of
     item 1's test fails on `3ebb59c`'s `dm.ts`.
   - Item 2 names tests still to be written. M4's held-press prototype fails
     today. Its rejection half fails today only when the test observes that
     the rejection escapes the console (G1). Its "once that command is
     answered it sends again" holds today, having no lock (G4).
   - Item 3 holds today and turns red under the change unless the test
     answers the dialog: it guards the change rather than describing today
     (G5).
   - Item 4 is the gate.
3. **Each rule is breakable. By reading.**
   - "Asks the DM first, in a message naming the actor and the condition,
     and sends nothing when the DM declines": red under B1, B8, B9 and B13.
   - "While a `remove_condition` … is unanswered, pressing it again sends
     nothing": red under B7; B2 breaks its "asks nothing" consequence.
   - "Whose promise rejects frees it and is reported with `console.error`":
     red under B7 and B10, once the test observes the escape (G1).
4. **The scope matches the claim. By command, then reading.**
   - `git grep` for `remove-condition`, `removeCondition` and
     `Remove condition` over `client/src` and `client/e2e`: the one group in
     `dm.ts`, built by `removeCondition` in `client/src/commands.ts`, and the
     one spec that presses it, `conditions.spec.ts`.
   - Over `client/test`: the tests that press a removal button are
     `dm-view.test.ts`'s "a condition on an actor gets a removal button that
     sends removeCondition" and "a removal button named by the list still
     sends the actor's and the condition's ids", and
     `qa-console-condition-names.test.ts`'s "click sends removeCondition with
     ids for a named case" and "click sends ids for an unnamed case".
     `app.test.ts`, `qa-condition-window.test.ts` and
     `qa-console-condition-names.test.ts`'s other tests read the labels;
     `command-surface.test.ts` finds the control and presses nothing;
     `commands.test.ts` builds the command.
   - The agent's `remove_condition` (`tools/toolgen`, `internal/mcp`) is the
     server's path and is untouched.
   - M3: the whole client suite and the typecheck stay green under the
     change, the QA file's click tests included.
   - The ticket's "What it touches" leaves out records (G3).
5. **No recorded decision is contradicted silently. By reading.**
   - No record decided that Remove condition asks nothing. The Delete note
     report's "What was deliberately left out" put it to the owner; the
     ticket follows his answer.
   - The opening comment of `dm.ts` names a sufficient reason to ask first,
     not the only one, and stays true.
   - VTT-864 says a button "sends `remove_condition`"; D9 keeps it standing
     and says why. VTT-541 settles which presses one lock holds back (D3).
   - SPEC-013's `remove_condition` row (who may issue it) and SPEC-018's
     `ConditionRemoved.reason` describe the server and stand.
   - `tools/ts-mutation-equivalents.txt`: nothing in it goes false (D7, M7).
6. **The records the work moves are named, and reachable. By command, then
   reading.**
   - The section is present and its one path resolves.
   - A search of `docs/specifications/` for `Remove condition`,
     `remove-condition`, `removeCondition`, `remove_condition` and
     `asks first`: SPEC-022 moves ("The console's conditions", which says the
     button sends; "What a command's answer leaves typed", whose list of
     controls that clear nothing gains the button; "One press, one command",
     whose keys and VTT-541 sentence gain it; the drawing, rejection and
     refusal sentences; a paragraph after Delete note's; the Status
     sentence; the "## Requirements" list). SPEC-013 and SPEC-018 do not
     move.

## Notes on the problem statement, not failures

None changes what is built. The writer may correct them before Task 0's
commit; the report and SPEC-022 must not repeat them as they stand.

- **N1. "the only command that does is `use_ability` … sent from a player's
  panel or by the agent."** The player panel is drawn in a DM's window too
  (VTT-695), so a DM can put a condition back from their own window's panel,
  acting as an actor whose ability applies it. What the ticket says of the
  console stays true. This bears on OD-1.
- **N2. "A send whose promise rejects is caught by nothing."** True in a
  browser: the press returns `d.send`'s promise to the click listener, and a
  browser drops a listener's return value. In the client's tests it is not:
  happy-dom catches a rejected promise a listener returns, passes it to
  `console.error` and dispatches an `error` event at the window (G1).

## The gaps this plan inherits

- **G1. Item 2's rejection test passes today unless it looks past
  `console.error`.** Through happy-dom (N2), today's rejected send reaches
  `console.error` and asks no repaint, and with no lock the second press
  sends. The rejection test therefore also counts `error` events at the
  window, which today's tree dispatches and the change does not (M2, M4).
- **G2. The ticket's open points:** the words are OD-1; the naming is D2; the
  lock is D3; how the rows take the removal in is D9.
- **G3. Records the ticket does not name:**
  - `tools/ts-mutation-equivalents.txt`: read, and not edited (D7);
  - `tools/comment-ceilings.txt`: binding, and no row moves (M6, D6);
  - the Phase 4a file, a new `client/test/qa-*.test.ts`;
  - the report.
- **G4. Item 2's "once that command is answered it sends again"** holds
  today, which has no lock. The held-press test asserts it after the
  release all the same, so that a lock never freed turns it red.
- **G5. Item 3** passes today and its evidence is `task e2e` at Task 2; no
  run of it before then.

## Measurements this plan stands on

- **M1. The base is green.** In the worktree, from `client/`,
  `env -u CLAUDECODE bun test test/dm-view.test.ts` passed whole.
- **M2. Today's button.** In the copy, on `3ebb59c`'s `dm.ts`: with
  `confirm` answering no, a press asked nothing and sent; with the send
  held, a second press of the same button sent again; with the send
  rejecting, `console.error` received the error and the window received an
  `error` event for each press.
- **M3. The change turns no existing test red.** In the copy, each button's
  press was moved into `sending(d, \`remove-condition:${actor}:${c.ID}\`, …)`:
  the question through `d.confirm` in OD-1 A's words, naming the label's two
  strings, a no returning nothing, a yes returning
  `{ command: removeCondition(actor, c.ID), clears: [] }`; one warning line
  and a citation line above it. The whole client suite
  (`env -u CLAUDECODE bun test test` from `client/`) and
  `bunx tsc --noEmit -p client/tsconfig.json` passed, and
  `check-new-prose.py` was clean.
- **M4. The new tests fail today.** Prototypes in the copy's
  `dm-view.test.ts` covered: a no over a named actor, an actor named `""`,
  an actor the fold lacks, a listed and an unlisted condition, and no list;
  the two existing press tests asserting one question and settling; a held
  press again, at a fresh paint, beside another condition of that actor and
  that condition on another actor, then after the answer; a rejected send,
  with the window's `error` events counted; and a `FORMS` entry (D8). All
  passed on the change. On `3ebb59c`'s `dm.ts` these failed: the no, the two
  press tests, the held-press test, the rejection test (by its window count
  alone) and the walk "a control whose command is unanswered sends nothing
  when pressed again, each row its own". The other walks pass today: they
  guard the change.
- **M5. The breaks the tests catch.** Each was applied to the change in the
  copy, `dm-view.test.ts` run, and the file restored from a saved copy
  compared byte for byte. Each turned a named test red:
  - B1 drop the question (the no test, the held-press test, both press
    tests);
  - B2 ask in a wrapper outside `sending` (the held-press test: a held press
    asks again);
  - B3 one key for the group, `"remove-condition"` (the held-press test and
    the lock walk);
  - B4 key by the condition alone (the held-press test and the lock walk);
  - B5 key by the actor alone (the held-press test);
  - B6 the key emptied, as Stryker's string mutant does (the held-press test
    and the lock walk);
  - B7 send outside `sending` (the held-press, rejection and lock-walk
    tests, and a neighbouring test, "a submitted session name is not
    restored on the next render");
  - B8 the actor by its id in the question, B9 the condition by its id (the
    no test);
  - B10 a repaint at the press (the drawing and refusal walks and the
    rejection test);
  - B11 `clears: ["note-key"]` (the walk "an accepted command clears the
    fields of its own form and of no other");
  - B12 the ids swapped in the command (both press tests and the held-press
    test);
  - B13 yes and no reversed (every test above that presses).
- **M6. The comment ceiling.** `client/src/view/dm.ts` measured with
  `check-comments.py`'s own `measure`, and `check-comments.py` run against
  the copy's base: with M3's one warning line the file stays at or under its
  ceiling and within the band, and the run is clean, so no row moves. A
  second warning line would put the file above its ceiling with an added
  comment line, which `check:comments` refuses.
- **M7. The adjudications the change touches.** For every entry of
  `tools/ts-mutation-equivalents.txt` keyed in `dm.ts`, its code line occurs
  in the changed file as often as its `copies` says, as on the base. No
  entry is keyed on a line the change edits.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened or dropped; the existing
  press tests gain assertions. No ledger row is written to dodge a finding,
  and no comment or code line is added or removed to dodge one.
- **P2. Rules 3, 4 and 5.** No contract, server or fold change;
  "condition" is the platform's word.
- **P3. Rules 8 and 10.** Names, never line numbers. A comment under
  `client/src` is a warning or a pointer, and no added comment line carries a
  banned term.
- **P4. Rule 9, answered in the ticket and checked** (check 1). MapTool sets
  and clears a token's state from its popup menu, one checkbox per state
  (`ChangeStateAction`), and clears them all with "Clear State"
  (`ClearStateAction`), asking nothing; `i18n.properties` holds no
  confirmation for a state. Its answer does not fit: its menu sets a cleared
  state again with one press, and the console has no control that applies a
  condition. Borrowed: nothing.
- **P5. SPEC-020 and SPEC-023.**
  - `sending` and `unanswered` are not edited.
  - The console reads nothing of `namer` but `resource` and `condition`.
  - The change brings one survivor the next full run will report, of a kind
    already adjudicated: the `[]` of the button's `clears` filled, as for the
    take chip's `clears: [],`. It is not adjudicated before a run has
    produced it: an entry excuses a survivor, and one written ahead of the
    run that names it widens an exemption no run has shown is needed
    (rule 2). The report names it for whoever answers that run.
  - The client's stored mutation report goes out of date with the code
    commit; `task check` names that and passes; a full run is the owner's to
    ask for.
- **P6. The mechanical gates.**
  - The code commit carries `cmd/vtt/webdist` rebuilt by `task build:client`
    (`check:drift` compares it).
  - bun runs under `env -u CLAUDECODE`, from `client/`, as `bun test test`;
    a bare `bun test` loads the Playwright specs. `task client:typecheck`
    passes; the harness's options take no `conditions: undefined`
    (`exactOptionalPropertyTypes`).
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

1. **D1. Each button sends through `sending`; the question is asked inside
   the press `sending` hands, before the command is returned; a no returns
   nothing.**
   - **Forced by:** item 2 and the ticket's two lock rules, which `sending`
     already keeps for its other controls (a lock taken when the command is
     sent and freed at any answer or a rejection, `console.error` for a
     rejection); the lock (`sending` stops a held press before it runs the
     press, so a held press asks nothing; B2 breaks it); P5. The precedents'
     D1.
   - **A consequence named, not a choice:** an accepted removal now asks for
     one repaint, as every control behind `sending` does. `app.ts`'s `act`
     already paints at every answer, so nothing the DM sees changes.
2. **D2. The question names the actor and the condition by the two strings
   the button's label is built from,** computed once per paint: the actor's
   name in the fold, else its id, and `names.condition(c.ID)`.
   - **Forced by:** item 1 ("naming the actor and the condition as the
     button does"): the DM decides on what the button pressed reads, so the
     question names exactly that; principle 13 (a name where one exists);
     and P5 (the namer read through `condition` alone).
   - Not a choice left open by the ticket, so not the owner's.
3. **D3. One lock per button, keyed `remove-condition:${actor}:${c.ID}`.** A
   button's unanswered removal holds back that button, at any paint, and no
   other: not another condition of that actor, nor that condition on another
   actor.
   - **Forced by:** VTT-541 (a command a console control sent holds back no
     control but that one; each button is a control, as each take chip and
     each row's Describe is); SPEC-022's "One press, one command", which keys
     a give and a take by the action, the actor's id and the ability's id;
     and an actor holding a condition once (`Apply` and the fold refuse a
     second), so the key names one button. B3 to B6 break it.
   - This answers the ticket's open point from the register, so it is not
     the owner's.
   - **Refused:** an actor id or a condition id holding `:` so that two
     buttons share a key. An almost theoretical edge case: no code, no test,
     no debt.
4. **D4. The words are OD-1's.** Recommended: `Remove <condition> from
   <actor>? Nothing in the console puts it back.`
5. **D5. A yes clears nothing: `clears: []`.** No field feeds the button.
   - **Forced by** VTT-533 and the walk "an accepted command clears the
     fields of its own form and of no other" (B11).
6. **D6. The comment above the button is one warning line and one citation
   line:** `// Ask inside the press \`sending\` runs (SPEC-022).`, then the
   new ids.
   - **Forced by** M6: one warning line keeps the file under its ceiling and
     a second does not. The line keeps the warning (where the question sits,
     which B2 breaks) and points to SPEC-022 for why. This is the file's
     whole comment budget for this change: a further comment line in `dm.ts`
     must be paid for by removing one, never by adding code.
   - No ledger row moves (M6). If the final tree, after the review's fixes,
     falls more than the band under the ceiling, `python3
     tools/check-comments.py --write-ledger` runs once, as the last edit
     before staging, and the lowered row is committed with the code.
7. **D7. `tools/ts-mutation-equivalents.txt` is not edited.**
   - **Forced by** M7 and a reading of the console's lock-key block:
     - its header is already in the past tense, naming the calls whose key
       was a string literal on `526885b` and the two whose key was a
       template literal there (the Delete note report's deviation), so a
       further template key makes nothing in it false;
     - D3 adds no literal key, so each reason's "the other literal keys are
       non-empty and differ from each other" holds; `remove-condition:` is a
       non-empty prefix, so "every template key opens with a non-empty
       prefix" holds;
     - the `namer(...)` entry's premise holds (D2 reads `condition` alone),
       and its "every removal button names the condition" stays true;
     - the take chip's `clears: [],` entry's premise, the spelling of every
       draft key, holds: the change adds none.
   - **What the next full run should find** from this change: the `[]` of
     the button's `clears` filled (P5). The emptied key is killed by the lock
     walk's second row (B6), as the run of `526885b` killed it on the two
     template keys of its day; so it needs no entry.
8. **D8. Existing tests: assertions added, none changed.**
   - "a condition on an actor gets a removal button that sends
     removeCondition" and "a removal button named by the list still sends
     the actor's and the condition's ids" each assert one question and one
     send, and settle so that the lock is freed inside the test
     (`support/answers.ts`: locks outlive the test) (R3).
   - `tableForEveryForm` gives `act-r1` and `act-r2` each the condition
     `dazed`, so every walk's console draws the group. A helper beside
     `grantForm` builds a Remove condition form for an actor: nothing typed,
     nothing cleared, pressing the button labelled `<actor>: dazed ✕`.
     `FORMS` gains the one for `act-r1` before `grantForm("act-r1")`; the lock
     walk's controls gain the one for `act-r2`, beside `describeForm("tok-r2")`
     and `grantForm("act-r2")`. The refusal walk (VTT-517, VTT-538, R8), the
     acceptance walk (VTT-533, B11), the lock walk (VTT-523, VTT-541, R5,
     B3, B4, B6) and the drawing walk (VTT-524, R6, B10) then take it in.
   - "a press the console's own guard refuses holds nothing back" leaves the
     Remove condition entry out, as it leaves out Describe: neither has a
     check of its own. Its assertions do not change.
   - The walks' citation lines gain the new ids beside the old.
   - `qa-console-condition-names.test.ts` is not edited: its click tests
     answer yes and stay green (M3).
   - **Forced by** P1, D9 and VTT-541.
9. **D9. The removal earns rows of its own; VTT-523, VTT-524, VTT-526,
   VTT-538 and VTT-864 stand as worded.**
   - **Forced by:**
     - the four list rows stay true: each states its property for the
       controls it lists and claims no control beyond them; the Delete note
       plan's D9 and N2;
     - VTT-864's "and sends `remove_condition` naming the actor's id and the
       condition's id" names what a press sends, not that every press sends:
       the register reads VTT-730's "sends `give_ability` or `take_ability`"
       so, its own evidence holding "qaDMAb: an unanswered give or take holds
       back only itself: keyed per actor and per ability", a held press that
       sends nothing. R3 states the yes, and VTT-864's tests and its e2e
       answer yes;
     - withdrawal and succession would move every citation. VTT-864 alone is
       cited by `dm.ts`'s doc line on `DMDeps.conditions`, `dm-view.test.ts`,
       `app.test.ts`, `qa-condition-window.test.ts`,
       `qa-console-condition-names.test.ts`, `conditions.spec.ts`, SPEC-022
       and the `namer(...)` entry's reason; the list rows' citations are the
       Delete note plan's D9;
     - precedent: the Say's rows and Delete note's rows of their own.
   - **Its cost, named:** the lock's rule is worded once for the listed
     controls and once for the removal. SPEC-022 states it once and cites
     both.
10. **D10. The e2e test answers the question.** In `conditions.spec.ts`,
    before `remove.click()`, `page.once("dialog", …)` keeps the message and
    accepts, as `handover.spec.ts` does for Make spectator; after the feed
    shows `Dummy lost …` and the dot is gone, the message is asserted to be
    OD-1's question naming `Dummy` and `dazed.name`. Its citation line gains
    R1's and R3's ids.
    - **Forced by** item 3, and by Playwright dismissing a dialog no handler
      answers: `window.confirm` then returns false, nothing is sent, and the
      feed never shows the loss. The message assertion holds R1 through the
      real `confirm: (m) => window.confirm(m)` wiring in `app.ts`.
11. **D11. The question has no "don't ask again".**
    - **Forced by** the precedents' OD-3 A: `window.confirm` has no third
      answer and the client keeps no settings to turn a question back on.

## Open decisions for the owner

- **OD-1. The question's words.** (The ticket's first open point. It changes
  what the DM is told, so it is his.) The console's questions take the shape
  "<verb> <name>? <what follows>.", as Remove actor's, Remove token's and
  Delete note's do. Facts: no console control sends a command that applies a
  condition; only `use_ability` does, from the player panel of any window
  that offers the ability, a DM's included (N1), or from the agent.
  - **A (recommended).** `Remove Dazed from Lera? Nothing in the console puts
    it back.`
    - True as said, and its claim is about the console, whose controls sit
      in the file that holds the question: whoever adds a control that
      applies a condition edits the same group's file.
    - "Remove … from …" as Remove token's "Remove <token> from the board?".
  - **B.** `Remove Dazed from Lera? Only an ability that applies it puts it
    back.`
    - Tells the DM the way back. Its claim is about the whole platform
      (`ClientCommand` has no other command that applies a condition), which
      nothing near the question would show going false; and an ability that
      applies a condition may need a hit, so "puts it back" promises more
      than a use gives.
  - **C.** `Remove Dazed from Lera? Nothing brings it back.` Remove actor's
    words, and false here: an ability does. Not recommended (principle 34).
  - Under any of these the names are shown without quotes, as Remove actor's
    and Remove token's are: a condition's name is a word or two from its
    ruleset, not a sentence.
  - **The owner's answer, 2026-11-23: A.**

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the register's
words are the ones that bind. Every new row reads `**OPEN — no test yet**`
until Task 1 cites its tests. Rows that a press the console's lock can hold
back carry "while no `remove_condition` it sent is unanswered", the clause
whose absence withdrew VTT-946 and VTT-949: a held press never runs the press
body.

| R | Item | Rule (as drafted for OD-1 A; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | A button of the console's "Remove condition" group, pressed while no `remove_condition` it sent is unanswered, asks the DM first, in a message holding the actor and the condition as the button reads them, and sends nothing when the DM declines. | B1, B8, B9, B13 |
| R2 | 1 | The console's Remove condition question reads OD-1's words. | a reworded question |
| R3 | 1 | A Remove condition button the DM accepts sends one `remove_condition` naming the actor's id and the condition's id. | B1, B12, B13 |
| R4 | D1 | While a `remove_condition` a button of the group sent is unanswered, pressing the button for that actor and condition again asks the DM nothing. | B2 |
| R5 | 2 | While a `remove_condition` a button of the group sent is unanswered, pressing the button for that actor and condition again sends nothing. | B7, B13 |
| R6 | D1 | A press of a Remove condition button that sends a `remove_condition` changes nothing the console draws until that command is answered. | B10 |
| R7 | 2 | A `remove_condition` a button of the group sent, whose promise rejects, clears nothing, asks for no repaint, frees that button, and is reported with `console.error`. | B7, B10 |
| R8 | D1 | When the server refuses a `remove_condition` a button of the group sent, the console asks for no repaint of its own. | B10, B13 |

**Recommended to the sort, as the precedents' sorts ruled:** refuse R2
(presentation: VTT-950, VTT-952 and VTT-956 pin a name and no words, and the
tests pin the words) and R4 (a consequence of D1's place behind the lock; B2
still runs as a deliberate break, and the held-press test cites R5). Keep R6
and R8 beside R5 and R7: SPEC-022's drawing and refusal sentences name
"those controls", which will include the buttons, and each cites the rows
that hold it, as for Delete note (VTT-961, VTT-963).

**Candidates the sort may refuse:**
- **A row that a button's removal holds back no other button.** Refused:
  VTT-541 states it for every console control, and the held-press test and
  the lock walk hold it (B3 to B6). The held-press test joins VTT-541's
  evidence at Task 1.
- **A row that an accepted removal clears no field.** Refused: VTT-533 and the
  acceptance walk (B11).
- **A row that the agent's `remove_condition` over MCP asks nothing.**
  Refused: the ticket's non-goal.
- **A row that the question names what the label read at its paint.**
  Refused: how, not what (D2); R1's test holds what it names.
- **A row that ids holding `:` keep apart.** Refused: an almost theoretical
  edge case (D3).

**No existing row becomes false.** Checked and left standing:
- **VTT-523**, **VTT-524**, **VTT-526**, **VTT-538** and **VTT-864** (D9).
- **VTT-517**, **VTT-533** and **VTT-541**, which state their rule for every
  console control and gain the buttons through the walks.
- **VTT-862**, **VTT-863** and **VTT-865**, which name a condition where the
  window draws it; the label is unchanged.
- **VTT-730** (the take chip and the give), untouched.
- **VTT-604** (a refusal shown as the toast).
- The precedents **VTT-569**, **VTT-901**, **VTT-902** and **VTT-947** to
  **VTT-963**.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R8 and the
  candidates, as OD-1's answer leaves them. Allocate ids with
  `requirement-id`. Fill in this plan's sign-off and ids. Commit the ticket,
  this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: Remove condition asks first (R1, R3, R5 to R8; D1 to D10; items 1 to 3)

- **Files:**
  - `client/src/view/dm.ts`: the "Remove condition" group's press and the
    two comment lines above it;
  - `client/test/dm-view.test.ts`: the new tests beside the conditions
    tests, D8's assertions, fixture, helper and entries, and the citation
    lines;
  - `client/e2e/conditions.spec.ts`: D10;
  - `docs/requirements.md`: the rows' cells, and VTT-541's evidence;
  - a rebuilt `cmd/vtt/webdist`;
  - `tools/comment-ceilings.txt` only if D6's last step writes it.
- **Tests first, seen red where today differs:**
  - Over `conditionsOn` and `CONSOLE_CONDITIONS`, through `harness` with
    `setConfirm(false)`: a named actor's listed and unlisted conditions, an
    actor named `""`, an actor the fold lacks, and a console with no list
    are each asked once, `confirmMessages()` exactly OD-1's question with
    D2's names, and nothing is sent or shown (R1). Red today.
  - D8's press tests (R3). Red today.
  - With `answer: "held"`: a second press of the same button, and a press of
    it at a fresh `harness`, ask nothing and send nothing; another condition
    of that actor and that condition on another actor each send; after the
    answer and a settle, the first button asks and sends again (R5,
    VTT-541). Red today. Every held send is released in a `finally`.
  - With a send that rejects: two presses, each settled, send twice, ask for
    no repaint, reach `console.error` once each, and dispatch no `error`
    event at the window, the listener removed in a `finally` (R7, G1). Red
    today by the window's count.
  - D8's fixture, helper and entries. The lock walk is red today; the other
    walks are green today and hold B10 and B11.
  - D10's e2e change, seen at Task 2.
- **Code:** D1 to D6, as M3 prototyped.
- **True when:**
  - the new tests pass, and those marked red failed before;
  - the whole client suite, `task client:typecheck` and P6's checks pass;
  - the comment above the press is D6's two lines and carries none of the
    banned terms `check:comments` names;
  - `sending` and `unanswered` are untouched, and
    `tools/ts-mutation-equivalents.txt` is unchanged (D7);
  - `task check:requirements-chain` passes.
- **Deliberate breaks** (M5), each turning a named test red, then restored
  from a saved copy compared byte for byte: B1 to B13.
- **The state here:** the DM is asked before the console removes a
  condition, by the names its button shows; a no sends nothing; a second
  press of that button while the first is unanswered does nothing, and every
  other button still works; a rejected send is reported.

### Task 2: the gate (item 4)

- **Do:**
  - `go clean -cache`; check free space and the load (`os.getloadavg()`).
  - Run `task check` whole on the code commit, detached under `/bin/sh`,
    output to a file, requiring its completion line.
  - Run `task e2e`; `conditions.spec.ts` must pass with D10's dialog.
  - `go clean -cache`.
  - Tell the owner the client's stored mutation report is out of date from
    Task 1, which commit it measured (SPEC-023), and that the next full run
    should find P5's survivor beside Delete note's.
  - If `check:comments` reports a share more than the band under a ceiling
    D6 did not cover, the gate is red: back to Task 1, and the row is written
    there, never on a green-seeking edit here.
- **True when:** `task check` and `task e2e` are green, and the embedded
  bundle is the code commit's.

### Task 3: the report, then the specification

- **The report:** `docs/reports/2026-11-23-remove-condition-asks-first.md`,
  with the `implementation-report` skill and its reading review. It holds:
  - each "Done looks like" item with its observation;
  - the rows, and the break that turned each red;
  - P4 (rule 9), with `ClearStateAction`;
  - G1 to G5 and the notes on the problem statement;
  - OD-1's answer;
  - P5's expected survivor, with the entry it would mirror, and that D7
    left the equivalents file unedited, with why;
  - Phase 4a's adjudications under their own heading;
  - whether D6's ledger step ran;
  - the gate.
- **SPEC-022,** with the `specification` skill, from the report:
  - "The console's conditions": the button asks first (the new paragraph)
    and sends `removeCondition` with the two ids on a yes;
  - "What a command's answer leaves typed": the buttons join the take chip
    and Delete note as controls that send through `sending` and clear
    nothing;
  - "One press, one command": a key by the action, the actor's id and the
    condition's id for a Remove condition button, and in the VTT-541
    sentence, a removal holding back only that button; its rows gain R5;
  - the drawing, rejection and refusal sentences gain R6, R7 and R8;
  - a paragraph "The console's Remove condition asks first", after Delete
    note's (D1 to D5 and the rows);
  - the Status sentence, after Delete note's, naming
    `client/test/dm-view.test.ts`, the QA file and
    `client/e2e/conditions.spec.ts`;
  - "Principles served" weighed for 13 and 34;
  - the "## Requirements" list gains the new ids.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the specification;
  push and merge on the owner's word, after `fix/delete-note-asks-first`.

## For Phase 4a

- **Hand the QA agent:**
  - the rows as the register words them, D1 to D5 and OD-1's answer;
  - the surface as signatures, never the source:
    `renderDMConsole(d: DMDeps): HTMLElement`, the `DMDeps` interface (`st`,
    `send`, `notify`, `confirm`, `rerender`, `conditions`), `State.Actors`,
    `State.Conditions`, `ConditionDef`, and `newState` from
    `client/src/state.ts`;
  - `ok`, `refused`, `held` and `settle` from
    `client/test/support/answers.ts`, and `client/test/support/dom`.
- **Tell it:**
  - never run `task check`, `task e2e` or Stryker;
  - its file is a new `client/test/qa-*.test.ts`;
  - tests run from `client/` under `env -u CLAUDECODE bun test test/<file>`;
  - `Array.from` over a `NodeList`;
  - the console's locks outlive a render in module state, so release every
    held send and settle every sent one in the test that sent it;
  - happy-dom passes a rejected promise a click listener returns to
    `console.error` and an `error` event at the window, so a test of a
    rejection observes the window too (G1);
  - a comment in its file cites a row by its id or this plan's decision by
    number, never the dispatch brief, which is in no record (the Delete note
    report's "Task 1's reading review, on the QA file");
  - a test named by a template cannot be cited by name in the register;
  - the tree is hashed before and after it.
- **Have it try:**
  - actor names and condition names with spaces and punctuation, an actor
    named `""`, an actor the fold lacks, a condition the list does not
    declare, and no list;
  - two conditions on one actor and one condition on two actors, each held
    while the others are pressed;
  - a press at a later paint while held;
  - a declined press followed by an accepted one;
  - a refused and a rejected send after a yes;
  - what is drawn until the answer, and the repaint after an accepted one.
- **Known, and not findings:**
  - ids holding `:` sharing a key (D3, refused);
  - the agent's MCP `remove_condition` asks nothing (non-goal);
  - the question names what the label read at its paint (D2);
  - no "don't ask again" (D11).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it to check:

- `sending` and `unanswered` are untouched, the console reads nothing of
  `namer` but `resource` and `condition`, and
  `tools/ts-mutation-equivalents.txt` is unchanged;
- the key against D3, the names against D2, the words against OD-1's
  answer, and `clears: []` (D5);
- each comment under `client/src` is a warning or a pointer; the comment
  above the press is D6's two lines; no comment or code line was added or
  removed to fit the ceiling;
- no assertion was weakened or dropped (rule 2), D8's assertions are there,
  and the guard walk's assertions are unchanged;
- every held send is released and every sent one settled;
- the e2e accepts the dialog and asserts its message (D10);
- each scope claim in new prose names the search that bounds it.
