# Plan: the console's Remove token asks the DM first

- **Ticket:** `docs/superpowers/specs/2026-11-22-remove-token-asks-first-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-22 by an agent that did not write the ticket, against
    `6d07bdf` on `fix/remove-token-asks-first` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and `task build:client` were not run. M1 ran in the
    worktree; `tools/check-comments.py --report` ran there too, and the
    `tools/__pycache__/` it left was removed, so `git status` holds the
    ticket and this file alone. M2 to M6 ran in a copy of the tree made by
    `git archive HEAD` outside the repository. The rest are searches and
    readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-22-remove-token-asks-first.md`.
- **Base:** `6d07bdf`. **Branch:** `fix/remove-token-asks-first`.
- **Owner's rulings taken in:**
  - 2026-11-22: told that Remove token asks nothing while MapTool asks
    before deleting a token, "kör den".
  - Standing: an almost theoretical edge case with a small consequence gets
    no code, no test and no debt; one source per fact, pointed at rather
    than copied; no counts or measured figures in anything written down; a
    code comment under `client/src` is a warning or a pointer; the client's
    full mutation run only when the owner asks; push and merge on his word.
- **Signed off:** by the owner, 2026-11-22: OD-1 = A (GM name, else the
  actor's name, else the token's name, else the id), OD-2 = A ("Remove <token>
  from the board? Its place, names and notes go with it."), OD-3 = A (no
  "don't ask again"), and the plan "Godkänd".
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off: R1 =
  VTT-952, R3 = VTT-953, R4 = VTT-954, R5 = VTT-955; R2 and R6 refused (below).

## The verdict, check by check

1. **Every path resolves. By command.**
   - `client/src/view/dm.ts`, with the `remove-token` action and its press
     sending `removeToken(tokId.value.trim())`, the notice
     `name the token to remove`, and the comment above it carrying
     "retraction-leaves Task 8, spec §5.1" and "this branch's own Task 3".
   - `Token` in `client/src/state.ts` with `ActorID`, `Name`, `Notes`,
     `GMName` and `GMNotes`.
   - The server: `remove_token` becomes one `TokenRemoved`
     (`internal/gateway/convert.go`); the `TokenRemoved` arm of `Apply`
     (`internal/engine/apply.go`) deletes the token and refuses an unknown
     one (`engine: removed unknown token`). In
     `internal/perceive/projection.go` the `TokenRemoved` arm is withheld,
     and a viewer that held the token is sent a `TokenHidden`
     (`TestARemovedTokenReachesAPlayerOnlyAsHidden`, in
     `internal/perceive/board_test.go`).
   - VTT-950 and SPEC-022's "The console's Remove actor asks first";
     New link, Boot and Make spectator calling `DMDeps.confirm`.
   - `client/test/dm-view.test.ts`, `docs/requirements.md`,
     `cmd/vtt/webdist`, `docs/specifications/022-a-client-holds-one-log.md`,
     and `CLAUDE.md` rule 10.
   - In `~/dev/RPTool/maptool`: `MapTool.confirmTokenDelete`,
     `msg.confirm.deleteToken` ("Are you sure you want to delete the
     selected tokens?") in `i18n.properties`, and
     `AppPreferences.tokensWarnWhenDeleted`, defined true. The dialog's
     third answer, "Yes, don't show again", turns that preference off, and
     `msg.confirm.deleteToken.removed` points to Preferences to turn it
     back on.
2. **"Done" is an observation. By command where it could be.**
   - Item 1 names a test still to be written. M2 shows today's Remove token
     sends with a `confirm` that answers no, and asks nothing, so that test
     fails today; M4 ran prototypes of it.
   - Item 2 holds today, which asks nothing at all. It guards the change
     (B2) rather than describing today. Its evidence is partial (G1).
   - Item 3 is a reading, backed by a search of the comment above Remove
     token for the banned terms `check:comments` names.
   - Item 4 is the gate.
3. **Each rule is breakable. By reading.** The ticket's one rule fails on a
   Remove token that sends after a no, and on a question that does not hold
   the token's name. M5's B1 and B4 turn it red. "Naming the token" is not
   defined until OD-1 is answered: the ticket says so itself.
4. **The scope matches the claim. By command, then reading.**
   - `git grep` for `remove-token`, `removeToken` and `remove_token` over
     `client/`: the one control in `dm.ts`, built by `removeToken` in
     `client/src/commands.ts`. The tests that press it:
     - `client/test/dm-view.test.ts`: "each guard refuses with its own exact
       wording", "removing a token sends its trimmed id and clears the
       field", "a whitespace-only id is refused everywhere one is
       required", and the `FORMS` table's "Remove token", which the
       refused, accepted, held and rejected `FORMS` tests press;
     - `client/test/qa-refused-command.test.ts`'s "Remove token".
     `client/test/command-surface.test.ts` names the action and presses
     nothing; `client/test/commands.test.ts` builds the command and renders
     no console; `client/test/removal-batch-parity.test.ts` folds and
     renders no console. Every presser's `confirm` answers yes: the
     `harness` default and `qa-refused-command.test.ts`'s
     `confirm: () => true`. M3: the whole client suite stays green under the
     change.
   - `client/e2e`: `git grep` for `remove-token`, `Remove` and `dialog`
     finds no spec that presses Remove token. Its `dialog` handlers are in
     `booting.spec.ts` (Boot), `handover.spec.ts` (Make spectator) and
     `joining.spec.ts` (`rotate-link`). `refusals.spec.ts` types into
     `token-id` and presses Place. So no Playwright test meets this dialog.
   - The ticket's "What it touches" leaves out records (G3).
5. **No recorded decision is contradicted silently. By reading.**
   - No record decided that Remove token asks nothing. The precedent's
     report (`docs/reports/2026-11-22-remove-asks-first.md`, "What was
     deliberately left out") records that it still asks nothing, where
     MapTool asks, and that this was raised with the owner. The ticket
     follows his answer.
   - The opening comment of `dm.ts` names when to ask first. It names a
     sufficient reason, not the only one, and stays true.
   - SPEC-013 (who may issue `remove_token`), SPEC-016 (the projection) and
     SPEC-022's "Remove token by `token-id` alone" stand. VTT-484 (how a DM's
     window names a token) stands; D2 follows it.
6. **The records the work moves are named, and reachable. By command, then
   reading.**
   - The section is present and its one path resolves.
   - A search of `docs/specifications/` for `Remove token`, `remove-token`,
     `removeToken`, `remove_token`, `confirm` and `asks first`:
     - SPEC-022 moves (a paragraph beside Remove actor's, the Status
       sentence, the "## Requirements" list). Its form-clearing paragraph
       and "One press, one command" stay true.
     - SPEC-007, SPEC-013, SPEC-016 and SPEC-017 describe the server and do
       not move; SPEC-009's Boot does not move.

## Notes on the problem statement, not failures

Neither changes what is built. The writer may correct them before Task 0's
commit; the report and SPEC-022 must not repeat them as they stand.

- **"Remove token is the one removal left that does not [ask]"** is false
  as a scope claim. A search of `dm.ts` for `d.confirm(` finds Remove actor,
  Make spectator, Boot and New link alone; Delete (a note), the "Remove
  condition" buttons, an ability's take chip and a held actor's revoke
  button also remove without asking. Whether any of them should ask is not
  this ticket's.
- **"A player whose character saw the piece is sent that it went out of
  sight"** is true of the stream, a `TokenHidden`. The player's window
  draws no Story line for it (VTT-823), so in words they are told nothing;
  the piece leaves their board.

## The gaps this plan inherits

- **G1. Item 2's evidence observes "asks nothing" only for the empty field.**
  "each guard refuses with its own exact wording" asserts no question for
  every guard, Remove token's included. In "a whitespace-only id is refused
  everywhere one is required", the Remove token arm asserts the notice and
  that nothing was sent, but not that nothing was asked. With the question
  moved before the empty check (B2), that arm stays green (M5). D7 adds the
  assertion.
- **G2. Item 1 does not say how the token is named.** That is the ticket's
  own first open point: D2 and OD-1.
- **G3. Records the ticket does not name:**
  - `tools/comment-ceilings.txt`: M6 puts `dm.ts` more than the band under
    its ceiling after the change, so its row is lowered (D8);
  - `tools/ts-mutation-equivalents.txt`: untouched but binding (P5);
  - the Phase 4a file, a new `client/test/qa-*.test.ts`;
  - the report.
- **G4. The ticket's "What could not be established"** travels: the naming
  is D2 and OD-1; the words are D3 and OD-2; a press the lock holds back is
  D1.

## Measurements this plan stands on

- **M1. The base is green.** In the worktree, from `client/`,
  `env -u CLAUDECODE bun test test/dm-view.test.ts
  test/qa-refused-command.test.ts test/command-surface.test.ts
  test/qa-remove-asks-first.test.ts` passed whole.
- **M2. Today's Remove token.** A probe in the copy rendered
  `renderDMConsole` over a state holding a token with a GM name, with
  `confirm` answering no. Remove token with that id typed asked nothing and
  sent one `removeToken`.
- **M3. The change turns no existing test red.** In the copy, Remove token's
  press kept its empty check first, then read the trimmed id, then asked
  through `d.confirm`, naming the token by OD-1's recommended chain with
  placeholder words, and returned nothing on a no. The Remove line named in
  P5 was left as it is. The whole client suite (`bun test client/test`) and
  `bunx tsc --noEmit -p client/tsconfig.json` (what
  `task client:typecheck` runs) both passed.
- **M4. The new tests fail today.** Prototype tests in the copy's
  `dm-view.test.ts` covered:
  - a no, over tokens named each way D2 distinguishes, with a padded id and
    an unknown id;
  - a no keeping the field at the next paint;
  - a yes sending the trimmed id after one question;
  - a second press while held;
  - an empty and a blank field.
  All passed on the change. On `6d07bdf`'s `dm.ts`, the no, the
  field-kept and the yes tests failed. The held-press and empty-or-blank
  tests passed, because they guard B3 and B2, not today's behaviour.
- **M5. The breaks the tests catch.** Each was applied to the change in the
  copy, run, and restored from a saved copy, compared byte for byte. Each
  turned a prototype or existing test red:
  - B1 drop the question;
  - B2 ask before the empty check ("each guard refuses with its own exact
    wording" and the blank-field prototype red; the whitespace test's Remove
    token arm stayed green, which is G1);
  - B3 ask in a wrapper outside `sending` (a second press while held asks
    again);
  - B4 show the id always;
  - B5 `??` for `||`, inside the chain and at its end (a fold's empty name
    is `""`, never `undefined`, so a token with no GM name is shown as
    nothing);
  - B6 look up and show the untrimmed field;
  - B7 clear the field's draft on a no;
  - B8 put the players' name before the actor's;
  - B9 drop the `?.` on the actor lookup (a token whose actor the state
    does not hold throws);
  - B10 send the untrimmed id on a yes;
  - B11 drop the players' name from the chain.
- **M6. The comment ceiling.** `tools/check-comments.py --report` puts
  `client/src/view/dm.ts` within the band under its ceiling at `6d07bdf`.
  In the copy, measured with the script's own `measure`: the change, with
  the comment above Remove token replaced by a citation line and the
  warning Remove actor carries, falls more than the band under that
  ceiling. A replacement no shorter than the comment it replaces is the
  only way it would not, and padding a comment to dodge a ledger row is not
  a reason.
- **M7. The adjudications Remove token's change must keep true.**
  `tools/ts-mutation-equivalents.txt` keys a `StringLiteral` on the
  `"remove-token"` key of
  `button("Remove", sending(d, "remove-token", () => {`. It belongs to the
  block of literal lock keys whose shared premise is `sending` and
  `unanswered`. Another entry, on
  `const names = namer(d.st, [], true, d.abilities, d.properties,
  d.conditions);`, rests on the premise that the console reads nothing of
  the namer but `resource` and `condition` (D2).
  `tools/ts-coverage-thresholds.txt` holds `dm.ts` at full coverage.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened or dropped. The
  existing Remove token test and the whitespace test each gain assertions.
- **P2. Rules 3, 4 and 5.** No contract, server or fold change; "token",
  "actor" and "board" are the platform's words.
- **P3. Rules 8 and 10.** Names, never line numbers. A comment under
  `client/src` is a warning or a pointer, and no added comment line carries
  a banned term. The history in the comment above Remove token moves to the
  report.
- **P4. Rule 9, answered in the ticket and checked** (check 1). MapTool asks
  before deleting tokens unless `tokensWarnWhenDeleted` is off, and the
  dialog offers to turn it off. Borrowed: asking before a token is removed.
  Not borrowed: MapTool's unnamed "the selected tokens", since the console
  removes one token named by a typed id. The switch is OD-3.
- **P5. SPEC-020 and SPEC-023.**
  - The line `button("Remove", sending(d, "remove-token", () => {` stays
    byte for byte, and `sending` and `unanswered` are not edited, so the
    adjudication keyed there holds without re-judging.
  - The console does not read `names.token`, `names.shown` or `names.actor`,
    so the adjudication on the console's `namer(...)` line holds.
  - The client's stored mutation report goes out of date with the code
    commit. `task check` names that and passes; a full run is the owner's to
    ask for.
- **P6. The mechanical gates.**
  - The code commit carries `cmd/vtt/webdist` rebuilt by `task build:client`
    (`check:drift` compares it).
  - bun runs under `env -u CLAUDECODE`, and `task client:typecheck` passes.
  - `python3 tools/check-new-prose.py main` (it holds test files to its
    wrap band) and `python3 tools/check-comments.py main` run before each
    commit.
  - Stage and commit in separate calls.
  - `Array.from` over a `NodeList`.
  - Commands are started with `/bin/sh`, never zsh.
  - `go clean -cache` before and after the gate.
  - Python run from the worktree leaves a `tools/__pycache__/`. Remove it
    before `review-record`, which refuses untracked files.
  - The code task gets Phase 4a's QA and one reviewer before its commit.

## Decisions

1. **D1. The question is asked inside the press `sending` hands, after the
   empty check, before the command is returned.** A no returns nothing.
   - **Forced by** item 2 (an empty or blank field asks nothing, so the
     check comes first); by the lock (`sending` stops a press while its
     command is unanswered before it runs the press, so a press it holds
     back asks nothing, which B3 breaks); and by P5 (the keyed line stays
     byte for byte, and `sending` and `unanswered` stay untouched). The
     precedent's D1, for the same reasons.
2. **D2. The token is named from the console's own state, by OD-1's
   chain.** Recommended (OD-1 A): its `GMName`, else the name `d.st.Actors`
   holds for its `ActorID`, else its `Name`, else the trimmed id. As
   prototyped:
   `(tok && (tok.GMName || d.st.Actors[tok.ActorID]?.name || tok.Name)) || id`,
   where `tok` is `d.st.Tokens[id]` and `id` is the trimmed field.
   - **Forced by:**
     - VTT-484: a DM's window names a token, in its disc's initial and
       title, its card, the panel's target buttons and the Story, by the GM
       name, else the actor's name. The question names the piece as the
       DM's board names it.
     - Blueprint principle 13: no identifier where a name exists. So the
       players' name, which the DM's card shows as "players see:" (VTT-490),
       comes before the id.
     - The console's convention of a name, else the id (VTT-902, VTT-950,
       the "Remove condition" buttons). This puts the id last, in place of
       the board's `a creature`, because the question must say which piece
       the typed id named; it is the id, too, when the window holds no
       token of it (D4).
   - **Read from `d.st`, not through `namer`.** `namer`'s `token` answers
     `a creature` both for an unknown id and for a nameless token, so it
     cannot fall back to the id. Reading it would also falsify the
     adjudication on the console's `namer(...)` line (M7, P5).
   - `?.` on the actor lookup, because the state can hold a token whose
     actor it does not hold (`grid.test.ts`'s "a token whose actor is gone
     still draws, as a creature"); B9 breaks it. `||`, not `??`, because a
     fold's empty name is `""`; B5 breaks it.
3. **D3. The words are OD-2's.** Recommended: `Remove <token> from the
   board? Its place, names and notes go with it.`
   - **Forced by** the console's shape for a question, "<verb> <name>?
     <what is lost>.", as in Boot's, Make spectator's and Remove actor's.
     The ticket rules out "Nothing brings it back", since the token can be
     placed again as a new one.
4. **D4. An id the window holds no token of is asked by that id and sent on
   a yes;** the console does not refuse it.
   - **Forced by** item 1, which asks it, and by the server being the
     authority. The fold refuses an unknown token in its own words, shown as
     the toast (VTT-604). A window that has not yet folded a token the agent
     just placed would otherwise refuse a real one. The precedent's D4.
5. **D5. Comments.**
   - The comment above Remove token in `dm.ts` becomes a citation line of
     the new ids and a warning naming D1's placement, in the words Remove
     actor's warning uses. Its history and its description ("Takes a piece
     off the board for good (retraction-leaves Task 8, spec §5.1) — the DM
     control this branch's own Task 3 left without one when the Undo group
     came out. Shares tokId with Place rather than a field of its own:
     naming a token to remove needs nothing else.") move to the report
     under "History cut from the code (rule 10)".
   - `dm.ts`'s opening comment stays (check 5).
   - Above "removing a token sends its trimmed id and clears the field",
     one line is added: the harness answers Remove token's question yes,
     and the question's own tests follow. The precedent left the history in
     that file's comments: rule 10 binds `client/src`, and item 3 names
     only the comment above the control.
   - **Forced by** item 3, P3 and M6.
6. **D6. No e2e.**
   - **Forced by** check 4: no spec presses Remove token, so the suite
     meets no new dialog. And `confirm: (m) => window.confirm(m)` in
     `client/src/app.ts` is one wiring shared with Remove actor, Boot, Make
     spectator and New link, each but Remove actor driven through a real
     dialog by `booting.spec.ts`, `handover.spec.ts` and
     `joining.spec.ts`. The precedent's D6.
7. **D7. Existing tests: assertions added, none changed.**
   - "removing a token sends its trimmed id and clears the field" asserts
     one question and one send (R3).
   - "a whitespace-only id is refused everywhere one is required" asserts
     no question on its Remove token arm (G1, R5).
   - "each guard refuses with its own exact wording" already asserts no
     question for every guard. It gains R5's id on its citation line, and so
     does the whitespace test.
   - Every other presser keeps answering yes and needs no change (M3).
   - **Forced by** G1 and P1.
8. **D8. One code commit, with its bundle and the lowered ceiling row.**
   After the reading review's fixes are settled, as the last edit before
   staging, `python3 tools/check-comments.py --write-ledger` runs once,
   then `check-comments.py main` runs clean. The lowered row for `dm.ts` is
   committed with the code.
   - **Forced by** M6 and P6. `check-comments.py main` refuses a file more
     than the band under its ceiling, and P6 runs it before the commit. So
     the row cannot wait for the gate task, where the precedent's plan
     placed its ledger step (that change stayed within its band and needed
     none). A lowered row cannot be raised again, so it is written once and
     last.
9. **D9. The question has no "don't ask again".** Pending OD-3.

## Open decisions for the owner

- **OD-1. How does the question name the token?** (The ticket's first open
  point. It changes what the DM is told, so it is his.)
  - **A (recommended). GM name, else the actor's name, else the name
    players see, else the id.** The GM name and the actor's name are what
    the DM's board titles the piece by (VTT-484). The players' name comes next so the id appears only
    where the token carries no name at all (principle 13). The id stands in
    for the board's `a creature`, which would not say which piece was
    typed.
  - **B. GM name, else the name players see, else the actor's name, else
    the id.** A token described for players but given no GM name would be
    asked by the players' name, while the DM's board titles it by the
    actor's name. B8 is this order.
  - **C. As A, without the players' name.** A token whose actor is nameless
    and whose only name is the players' would be asked by its id while a
    name exists, against principle 13. B11 is this order.
  - Under any of these orders, several tokens of one actor with no GM name read
    alike. The question then tells them apart by nothing. A GM name, which
    the console's "Describe tokens" sets per token, is what tells them
    apart. Adding the token's square was considered and is not recommended:
    the board labels no square.
- **OD-2. The question's words.** (The ticket's second open point.)
  - **A (recommended).** `Remove <token> from the board? Its place, names
    and notes go with it.`
    - "from the board" is what the DM's Events panel says of it afterwards
      ("taken off the board", `describe` in `client/src/view/spectator.ts`;
      the Story draws no `tokenRemoved`, VTT-818).
    - "names", plural, because a token holds the name players see and the
      GM name, and both go.
    - "go with it" says what is lost without claiming that nothing restores
      it: placing again makes a new token, at whatever square is typed,
      with no names or notes until it is described.
  - **B.** `Remove <token> from the board? Its place, name and notes go with
    it.` Singular, as the brief drafted it; it leaves the GM name to be
    read into "name".
  - **C.** `Remove <token> from the board? Nothing brings back its place,
    names or notes.` It echoes Remove actor's words. But the DM can place a
    token on the same square and describe it with the same names, so it
    overstates, which the ticket's own open point warns against.
- **OD-3. Borrow MapTool's "don't ask again"?**
  - **A (recommended). No.**
    - `DMDeps.confirm` is `window.confirm`, which `app.ts` keeps "deliberate
      for a destructive action: it is modal and unmissable", and it has no
      third answer. A switch needs a dialog of the client's own, a stored
      setting the console does not have (the client stores the invite
      token and the viewpoint, nothing else), and a way back on, which
      MapTool keeps in a Preferences dialog the console has no counterpart
      to.
    - Remove actor borrowed none, and the console's other questions have
      none.
    - MapTool deletes what is visibly selected. Here the question is the one
      place a typed id is matched to a name, so turning it off removes more
      than it does there.
  - **B. Yes.** A third answer that stops Remove token asking in that
    browser. It costs a dialog in place of `window.confirm`, a stored
    setting, a way to turn it back on, a row (the R7 candidate below) and
    its tests.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the register's
words are the ones that bind. Every new row reads `**OPEN — no test yet**`
until Task 1 cites its tests. Rows that a press the console's lock can hold
back carry "while no `remove_token` it sent is unanswered", the clause whose
absence withdrew VTT-946 and VTT-949.

| R | Item | Rule (as drafted for OD-1 A; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | The console's Remove token, pressed with a token id that is not blank while no `remove_token` it sent is unanswered, asks the DM first, in a message holding the GM name the console's state holds for that token, else the name it holds for the token's actor, else the token's name, else the id, and sends nothing when the DM declines. | B1; B4, B5, B6, B8, B9, B11 |
| R2 | 1 | The console's Remove token question reads OD-2's words. | a reworded question |
| R3 | 1 | A Remove token the DM accepts sends one `remove_token` naming the trimmed token id. | B10 |
| R4 | 1 | A Remove token the DM declines leaves the token field as typed, at the next paint too. | B7 |
| R5 | 2 | The console's Remove token, pressed with the token field empty or only white space while no `remove_token` it sent is unanswered, shows `name the token to remove`, asks the DM nothing and sends nothing. | B2 |
| R6 | D1 | While a `remove_token` the console's Remove token sent is unanswered, pressing Remove token again asks the DM nothing. | B3 |

**Recommended to the sort, as the precedent's sort ruled:** refuse R2
(presentation: VTT-950 pins a name and no words, and the words are pinned by
the test) and R6 (a consequence of D1's place behind the existing lock,
whose VTT-523 holds that a second press sends nothing; B3 still runs as a
deliberate break, and the held-press test cites VTT-523).

**Candidates the sort may refuse:**
- **R7, the "don't ask again" switch.** Only if OD-3 is answered B.
- **A row that the agent's `remove_token` over MCP asks nothing.** Refused:
  the ticket's non-goal.
- **A row that the lookup is by exact id.** Refused: how, not what; R1's
  test, with a padded id, holds it.
- **A row that a GM name of white space alone is shown as written.**
  Refused: an almost theoretical edge case with a small consequence.

**The sort at Task 0** kept R1, R3, R4 and R5 as drafted, as VTT-952, VTT-953,
VTT-954 and VTT-955, and refused R2 and R6 as recommended above, with the
candidates below.

**No existing row becomes false.** Checked and left standing:
- **VTT-523**, **VTT-524**, **VTT-526**, **VTT-538** and **VTT-541**, which
  name Remove token among their controls. A press the DM declines sends
  nothing and takes no lock; a press that sends is as before.
- **VTT-517** and **VTT-533**: a decline clears nothing; an accepted
  removal clears `token-id` as before.
- **VTT-475**, **VTT-484** and **VTT-490**, which list the surfaces a
  window's naming covers. The console's question is not among them, and D2
  follows VTT-484's order.
- **VTT-604** (the toast of an unknown token's refusal), **VTT-823**, and
  the server's **VTT-344**.
- The precedents **VTT-569**, **VTT-901**, **VTT-902**, **VTT-947**,
  **VTT-948**, **VTT-950** and **VTT-951**.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R6 and the
  candidates, as OD-1 to OD-3's answers leave them. Allocate ids with
  `requirement-id`. Commit the ticket, this plan (its sign-off and ids
  filled in) and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: Remove token asks first (R1, R3 to R5; D1 to D5, D7, D8; items 1 to 3)

- **Files:**
  - `client/src/view/dm.ts`: Remove token's press and the comment above it;
  - `client/test/dm-view.test.ts`: a fixture of tokens, new tests beside
    "removing a token sends its trimmed id and clears the field", that
    test's added line and assertions, and D7's assertions;
  - `docs/requirements.md`: the rows' cells;
  - `tools/comment-ceilings.txt`: D8;
  - a rebuilt `cmd/vtt/webdist`.
- **Tests first, seen red where today differs:**
  - A fixture over `newState()` with a named actor and a nameless one. It
    holds a token for each case D2 distinguishes: a GM name set; no GM name
    over the named actor, with a players' name set; no names over the
    nameless actor; a players' name alone over the nameless actor; and an
    actor the state does not hold. Every empty name is `""`, as the fold
    writes it.
  - Through `harness` with `setConfirm(false)`: each token, a padded id and
    an id the state does not hold are each asked once, `confirmMessages()`
    exactly the question with the name D2 gives, and nothing is sent or
    shown (R1). Red today.
  - Typed through `input`: after a no, the field holds what was typed, in a
    fresh `harness()` too (R4). Red today.
  - With the default yes: "removing a token sends its trimmed id and clears
    the field" asserts one question and one send (R3). Red today.
  - With `answer: "held"`: a second press before `release` asks nothing and
    sends nothing (D1, citing VTT-523). Green today, which asks nothing at
    all; it holds B3.
  - D7's assertions (R5). Green today; they hold B2.
  - Every test that types into `token-id` empties it in a `finally`:
    `draft` outlives the harness.
- **True when:**
  - the new tests pass, and those marked red failed before;
  - the whole client suite, `task client:typecheck` and P6's checks pass;
  - the comment above Remove token holds none of the banned terms
    `check:comments` names, no history and no description of the control;
  - the Remove line named in P5 is unchanged;
  - `task check:requirements-chain` passes.
- **Deliberate breaks** (M5), each turning a named test red, then restored:
  B1 to B11, less whichever OD-1's answer makes the code (B8 under B, B11
  under C).
- **The state here:** the DM is asked before the console removes a token,
  by the name the DM's board gives it; a no leaves everything as typed.

### Task 2: the gate (item 4)

- **Do:**
  - `go clean -cache`; check free space and the load
    (`os.getloadavg()`).
  - Run `task check` whole on the code commit, detached under `/bin/sh`,
    output to a file, requiring its completion line.
  - Run `task e2e`.
  - `go clean -cache`.
  - Tell the owner the client's stored mutation report is out of date from
    Task 1, and which commit it measured (SPEC-023).
  - If `check:comments` reports a share more than the band under a ceiling
    D8 did not cover, the gate is red: back to Task 1, and the row is
    written there, never on a green-seeking edit here.
- **True when:** `task check` and `task e2e` are green, and the embedded
  bundle is the code commit's.

### Task 3: the report, then the specification

- **The report:** `docs/reports/2026-11-22-remove-token-asks-first.md`,
  with the `implementation-report` skill and its reading review. It holds:
  - each "Done looks like" item with its observation;
  - the rows, and the break that turned each red;
  - P4 and OD-3's answer;
  - G1 to G4 and the notes on the problem statement;
  - OD-1's and OD-2's answers;
  - the history cut under D5;
  - Phase 4a's adjudications under their own heading;
  - the ledger row D8 lowered;
  - the gate.
- **SPEC-022,** with the `specification` skill, from the report:
  - a paragraph "The console's Remove token asks first", beside Remove
    actor's (D1 to D4 and the rows);
  - the Status sentence beside Remove actor's, naming
    `client/test/dm-view.test.ts` and the QA file;
  - "Principles served" weighed for 13;
  - the "## Requirements" list gains the new ids.
  - Optional: whether "One press, one command" should name a press the DM
    declines beside a press the console's checks refuse. Both send nothing
    and take no lock. The precedent's plan intended that sentence, and it
    is not in SPEC-022.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the specification;
  push and merge on the owner's word.

## For Phase 4a

- **Hand the QA agent:**
  - the rows and SPEC-022's paragraph as drafted;
  - the surface as signatures, never the source:
    `renderDMConsole(d: DMDeps): HTMLElement`, the `DMDeps` interface
    (`st`, `send`, `notify`, `confirm`, `rerender`, `roster`,
    `participants`), `State.Tokens`, `Token` and `State.Actors`, and
    `newState` from `client/src/state.ts`;
  - `ok`, `refused`, `held` and `settle` from
    `client/test/support/answers.ts`, and `client/test/support/dom`.
- **Tell it:**
  - never run `task check`, `task e2e` or Stryker;
  - its file is a new `client/test/qa-*.test.ts`;
  - tests run under `env -u CLAUDECODE bun test`;
  - `Array.from` over a `NodeList`;
  - the console's typed text outlives a render in module state, so empty
    every field it types in a `finally`;
  - an empty name in a real state is `""`;
  - the tree is hashed before and after it.
- **Have it try:**
  - names with spaces and punctuation;
  - a GM name, a players' name and an actor name in every combination;
  - a token whose actor the state does not hold;
  - an id padded with white space;
  - a token removed from the state between render and press;
  - a refused and a rejected send after a yes.
- **Known, and not findings:**
  - an id the window does not hold is asked by id and sent (D4);
  - the agent's MCP removal asks nothing (non-goal);
  - several tokens of one actor with no GM name read alike (OD-1);
  - a GM name of white space alone is shown as written (refused candidate).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it to check:

- the Remove line named in P5 is unchanged, `sending` and `unanswered` are
  untouched, and the console reads nothing new of `namer`;
- the press's order against D1, and the chain against OD-1's answer;
- each comment under `client/src` is a warning or a pointer, and the
  comment above Remove token keeps no history and no description (item 3);
- the ceiling row D8 lowered matches `--write-ledger`'s output, and no
  comment was padded to avoid it;
- no assertion was weakened or dropped (rule 2), and D7's assertions are
  there;
- every test that types into `token-id` empties it;
- each scope claim in new prose names the search that bounds it.
