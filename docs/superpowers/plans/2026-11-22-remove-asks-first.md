# Plan: the console's Remove asks the DM first

- **Ticket:** `docs/superpowers/specs/2026-11-22-remove-asks-first-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-22 by an agent that did not write the ticket, against
    `009db12` on `fix/remove-asks-first` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and `task build:client` were not run. M1 ran in the
    worktree, its `git status` the same before and after; M2 ran from
    outside the repository against the worktree's source; M3 to M5 ran in a
    copy of the tree made by `git archive HEAD` outside the repository; the
    rest are searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-22-remove-asks-first.md`.
- **Base:** `009db12`. **Branch:** `fix/remove-asks-first`.
- **Owner's rulings taken in:**
  - 2026-11-22: Remove always asks, like Boot and Make spectator ("Fråga
    alltid"). The option chosen carried the example "Remove Goblin? Nothing
    brings it back, and its tokens leave the board."
  - Standing: an almost theoretical edge case with a small consequence gets
    no code, no test and no debt; one source per fact, pointed at rather
    than copied; no counts or measured figures in anything written down; a
    code comment under `client/src` is a warning or a pointer; the client's
    full mutation run only when the owner asks; push and merge on his word.
- **Signed off:** by the owner, 2026-11-22: OD-1 = A (the question names the
  actor alone), and the plan "Godkänd".
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off: R1 =
  VTT-946, R3 = VTT-947, R4 = VTT-948, R5 = VTT-949; R2 and R6 refused (below).

## The verdict, check by check

1. **Every path resolves. By command.** Every file and symbol the ticket
   names is where it says: `client/src/view/dm.ts` with the `remove-actor`
   and `remove-token` actions, its opening comment in the words quoted, the
   comment above Remove with "exactly one control asks first" and "A removal
   lands on the log every seat at this table can see", `DMDeps.confirm`, and
   the "New link", "Boot" and "Make spectator" controls calling it;
   `client/test/dm-view.test.ts` with the same reasons above "removing an
   actor sends its trimmed id and clears the field"; `handleRemoveActor` in
   `internal/gateway/server.go`; the `TokenHidden` a seen removed token is
   withheld as in `internal/perceive/projection.go`, which `STORY` in
   `client/src/view/feed.ts` marks undrawn; SPEC-015, SPEC-017, SPEC-021,
   SPEC-022; `cmd/vtt/webdist`. In `~/dev/RPTool/maptool`:
   `MapTool.confirmTokenDelete`, `msg.confirm.deleteToken` in
   `i18n.properties`, and `AppPreferences.tokensWarnWhenDeleted`, defined
   true.
2. **"Done" is an observation. By command where it could be.** Item 1 names
   a test still to be written; M2 shows today's Remove sends with a `confirm`
   that answers no and asks nothing, so that test fails today (M4 ran it).
   Item 2 names existing tests that do not observe "asks nothing" (G1).
   Item 3 is a search of `dm.ts` and `dm-view.test.ts` for the false
   reasons. Item 4 is the gate.
3. **Each rule is breakable. By reading.** The ticket's one rule fails on a
   Remove that sends after a no, and on a question that does not hold the
   actor's name; M5's breaks B1, B4 and B6 turn it red.
4. **The scope matches the claim. By command, then reading.**
   - `git grep` for `remove-actor` and `removeActor` over `client`: the one
     control in `dm.ts`; presses in `client/test/dm-view.test.ts` (the
     guard tests, "removing an actor sends its trimmed id and clears the
     field", the `FORMS` table's "Remove actor", and "a grant, a revocation
     and a removal of an actor each read the roster again once accepted") and
     `client/test/qa-refused-command.test.ts`'s "Remove actor" control;
     `client/test/command-surface.test.ts` names the action and presses
     nothing. Every presser's `confirm` answers yes: the `harness` default,
     `rosterConsole(true)`, and `qa-refused-command.test.ts`'s
     `confirm: () => true`. M3: the whole client suite stays green under
     the change.
   - `client/e2e`: no spec presses `remove-actor`. Its `dialog`
     handlers are `booting.spec.ts` (Boot), `handover.spec.ts` (Make
     spectator) and `joining.spec.ts` (`rotate-link`); `as-dm.ts` presses no
     Remove. So no Playwright test meets this dialog, which Playwright would
     dismiss.
   - Records the ticket's "What it touches" leaves out (G3):
     `docs/requirements.md`, and the adjudication in
     `tools/ts-mutation-equivalents.txt` keyed on Remove's
     `button("Remove", sending(d, "remove-actor", () => {` line, which binds
     where the question goes (D1). `tools/comment-ceilings.txt` holds
     `dm.ts` at its ceiling (M6). No Go file, no contract file.
5. **No recorded decision is contradicted silently. By reading.**
   - The "no confirmation" for Remove was recorded only in the comments
     the ticket quotes, and their premise ("exactly one control asks
     first") was already false once Boot asked: the Boot report
     (`docs/reports/2026-10-31-the-dm-boots-a-participant.md`, "History cut
     from the code") rewrote the head comment and left this one. The ticket
     overturns it by the owner's ruling and says so.
   - SPEC-017 (what a removal does) stands; SPEC-009's Boot and SPEC-022's
     Make spectator stand. Blueprint principle 2 (the record only grows) is
     why the question exists; none is contradicted.
6. **The records the work moves are named, and reachable. By command, then
   reading.** The section is present and its one path resolves. A search of
   `docs/specifications/` for `confirm`, `asks first`, `Remove actor` and
   `remove-actor` finds SPEC-022 (the controls' answer and lock paragraphs,
   and the roster paragraph's "asks the DM first"), which moves, and
   SPEC-009's Boot, which does not. SPEC-017 states no console control: its
   Status names the gateway, the campaign and the engine, and "What this
   record does not decide" names no client. It does not move. That settles
   the ticket's first open point.

## The gaps this plan inherits

- **G1. Item 2's evidence does not observe "asks nothing."** "each guard
  refuses with its own exact wording" and "a whitespace-only id is refused
  everywhere one is required" assert the notice and that nothing was sent.
  With the question asked before the empty check, and the harness answering
  yes, both stay green (M5, B2). D7 adds the assertion; the harness counts
  confirms for exactly this ("some guards must run BEFORE the dialog
  opens").
- **G2. Item 1 does not say how an actor with an empty name is shown.** D2:
  by its id, as the console shows a nameless actor everywhere else.
- **G3. Records the ticket does not name:** `docs/requirements.md` (the new
  rows); `tools/ts-mutation-equivalents.txt`, untouched but binding (D1);
  `tools/comment-ceilings.txt`, touched only if Task 2 finds `dm.ts` more
  than the band under its ceiling.
- **G4. The ticket's "What could not be established"** travels: SPEC-017 does
  not move (check 6); the words are D3 and OD-1.
- **Notes on the problem statement, not failures.** Neither changes what is
  built; the writer may correct them.
  - "an actor added again under a new id" cites SPEC-017, which says an
    actor may be added again under the same id, controlled by nobody.
  - "The people it reaches are not told": a connected player whose party
    member is removed is shown "You no longer play" and its name (VTT-605).
    What they are not told is that it was removed; one not connected is told
    nothing.

## Measurements this plan stands on

- **M1. The base is green.** In the worktree,
  `env -u CLAUDECODE bun test client/test/dm-view.test.ts
  client/test/qa-refused-command.test.ts client/test/command-surface.test.ts`
  passed whole.
- **M2. Today's Remove.** A bun test outside the repository, rendering the
  worktree's `renderDMConsole` over a state holding `gob-1` named Goblin,
  with `confirm` answering no: Remove with `gob-1` typed asked nothing and
  sent one `removeActor`. The DM's fold holds the actor's name as
  `st.Actors[id].name`.
- **M3. The change turns no existing test red.** In the copy, Remove's press checked the
  trimmed id, then asked `Remove <name, else id>? Nothing brings it back,
  and its tokens leave the board.` through `d.confirm`, and returned nothing
  on a no. The whole client suite passed; `bunx tsc --noEmit -p
  client/tsconfig.json` passed.
- **M4. The new tests fail today.** In the copy, prototype tests of a no
  (named actor, padded id, field kept at the next render), of a nameless
  actor and an unknown id, of an empty and a blank field, and of a yes and a
  second press while held, passed on the change; on `009db12`'s `dm.ts` the
  no, the nameless-and-unknown and the held-press tests failed, and the
  empty-and-blank test passed (it guards B2, not today's behaviour).
- **M5. The breaks the tests catch,** each applied to the change in the copy
  and reverted, each turning a prototype test red:
  - B1 drop the question; B2 ask before the empty check; B3 ask in a wrapper
    outside `sending` (a second press while held asks again); B4 show the id
    always; B5 `??` for `||` (a nameless actor shown as nothing); B6 look up
    and show the untrimmed field; B7 clear the field's draft on a no.
- **M6. The comment ceiling.** `tools/check-comments.py --report` puts
  `client/src/view/dm.ts` at its ceiling. The comment above Remove is a
  block over check:comments' bound. In the copy, the prototype's one-line
  warning in place of that block left `dm.ts` within the band, near its
  floor; with no comment there at all it would fall under.
- **M7. No other adjudication sits on Remove's code.**
  `tools/ts-mutation-equivalents.txt`'s one entry there is the
  `StringLiteral` on the `"remove-actor"` key, whose line the change leaves
  as it is, and whose premise is `sending` and `unanswered`, which it does
  not touch. `tools/ts-coverage-thresholds.txt` holds `dm.ts` at full
  coverage.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened or dropped. The guard
  tests gain an assertion; the existing Remove test keeps all of its.
- **P2. Rules 3, 4 and 5.** No contract, server or fold change; "actor",
  "token" and "board" are the platform's words.
- **P3. Rules 8 and 10.** Names, never line numbers. A comment under
  `client/src` is a warning or a pointer; no added comment line carries a
  banned term; M6 binds what may be added: a line added to the block above
  Remove must leave the block within the bound, so it is replaced, not
  appended to.
- **P4. Rule 9, answered in the ticket and checked** (check 1). MapTool asks
  before deleting tokens (`MapTool.confirmTokenDelete`,
  `msg.confirm.deleteToken`) unless `tokensWarnWhenDeleted` is off, and the
  dialog offers to switch it off. Borrowed: asking before an irreversible
  removal. Not borrowed: the switch, which "always" rules out; nor MapTool's
  unnamed "the selected tokens", since the console names one actor. MapTool's
  question covers deleting a token, which here is `remove-token`; the
  ticket leaves it out, and so does this plan.
- **P5. SPEC-020 and SPEC-023.** The line
  `button("Remove", sending(d, "remove-actor", () => {` stays as it is, and
  `sending` and `unanswered` are not edited, so the adjudication keyed there
  holds without re-judging. The client's stored mutation report goes out of
  date with the code commit; `task check` names that and passes; a full run
  is the owner's to ask for.
- **P6. The mechanical gates.** The code commit carries `cmd/vtt/webdist`
  rebuilt by `task build:client` (`check:drift` compares it); bun under
  `env -u CLAUDECODE`; `task client:typecheck`;
  `python3 tools/check-new-prose.py main` and
  `python3 tools/check-comments.py main` before each commit; stage and
  commit in separate calls; `Array.from` over a `NodeList`; commands
  started with `/bin/sh`, never zsh; `go clean -cache` before and after the
  gate. The code task gets Phase 4a's QA and one reviewer before its commit.

## Decisions

1. **D1. The question is asked inside the press `sending` hands, after the
   empty check, before the command is returned.** A no returns nothing.
   - **Forced by** item 2 (an empty or blank field asks nothing, so the
     check comes first); by the lock: `sending` stops a press while its
     command is unanswered before it runs the press, so a stopped press
     asks nothing (R6, B3); and by P5: the line that keys the adjudication
     stays byte for byte, and `sending` and `unanswered` stay untouched.
     The Give button's wrapper shape, a closure calling `sending(...)()`,
     would break each of these.
2. **D2. The actor is shown by `d.st.Actors[id]?.name || id`,** `id` the
   trimmed field, the same trimmed id the command carries.
   - **Forced by** item 1 (its id when the window holds no actor of that
     id), and, for an empty name (G2), by the console's one convention:
     Make spectator's question (VTT-902), the Abilities and "Who controls
     what" rows and the condition buttons each show a name, else the id. A
     blank would read "Remove ?". B4, B5 and B6 hold it.
3. **D3. The words are the owner's example:** `Remove <actor>? Nothing brings
   it back, and its tokens leave the board.`
   - **Forced by** the option the owner chose carrying them, and by the
     console's shape for a question: "<verb> <name>? <what is lost>.", as in
     Boot's and Make spectator's. "Its tokens leave the board" is what
     `handleRemoveActor` appends; for an actor with no token it says
     nothing false. Whether to name the controllers is OD-1.
4. **D4. An id the window holds no actor of is asked by that id and sent on
   a yes;** the console does not refuse it.
   - **Forced by** item 1, which asks it, and by the server being the
     authority: the fold refuses an unknown actor with its message
     (SPEC-017), shown as the toast (VTT-604), while a window that has not
     yet folded an actor the agent just added would refuse a real one.
5. **D5. Comments.** The block above Remove in `dm.ts` becomes a citation
   line of the new ids and a short warning naming D1's placement (ask
   inside the press, after the empty check, so a press the lock or the
   check stops asks nothing); its history ("retraction-leaves Task 9",
   "spec §5.2", "the one command in this arc that emits a batch") moves to
   the report under "History cut from the code (rule 10)", as the Boot
   report did. `dm.ts`'s opening comment stays: Remove now obeys its rule,
   and item 3 does not name it. The comment above the Remove test in
   `dm-view.test.ts` loses its "NO CONFIRMATION" paragraph and says the
   harness answers the question yes, the question's own tests following.
   - **Forced by** item 3, P3 and M6.
6. **D6. No e2e.**
   - **Forced by** check 4: no spec presses Remove, so the suite meets no
     new dialog; and the `confirm: (m) => window.confirm(m)` field in
     `client/src/app.ts` is one wiring shared with Boot, Make spectator and
     New link, each driven through a real dialog by `booting.spec.ts`,
     `handover.spec.ts` and `joining.spec.ts`. The ticket names no e2e.
7. **D7. Existing tests: one comment, one assertion.** The pressers keep
   answering yes and need no change (M3). "each guard refuses with its own
   exact wording" asserts `confirmCount()` is 0 for every guard case, and
   "a whitespace-only id is refused everywhere one is required" asserts it
   for its Remove actor arm.
   - **Forced by** G1 and P1.
8. **D8. One code commit, with its bundle** (P6).

## Open decisions for the owner

- **OD-1. Does the question name who controls the actor?** (the ticket's
  second open point; it changes what the DM is told, so it is his.)
  - **A (recommended). No:** the example's words as they stand. The name
    already tells the DM which actor, which is the slip a question catches
    (a mistyped id); "Who controls what", further down the same console,
    shows each actor's holders; and the example is the option he chose.
  - **B. Yes,** as Make spectator names what a player loses: "Remove Gobo?
    Nothing brings it back, its tokens leave the board, and Lera no longer
    controls it." Holders named as "Who controls what" names them (the
    roster's name, else presence's, else the id), sorted as VTT-902 sorts,
    joined by " and ", the clause left out when nobody controls it. Costs a
    row (the R2b candidate below) and tests for none, one, several and an
    unnamed holder.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the register's
words are the ones that bind. Every new row reads `**OPEN — no test yet**`
until Task 1 cites its tests.

| R | Item | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | The console's Remove, pressed with an actor id that is not blank, asks the DM first, in a message holding the actor's name, and sends nothing when the DM declines. | B1; B4 |
| R2 | 1 | The console's Remove question reads `Remove <actor>? Nothing brings it back, and its tokens leave the board.`, the actor shown by the name the console's state holds for the trimmed id, or by that id when the state holds no actor of it or that actor's name is empty. | B4, B5, B6; a reworded question |
| R3 | 1 | A Remove the DM accepts sends one `remove_actor` naming the trimmed id. | the question asked and nothing sent on a yes; the untrimmed id sent |
| R4 | 1 | A Remove the DM declines leaves the actor field as typed, at the next paint too. | B7 |
| R5 | 2 | The console's Remove, pressed with the actor field empty or only white space, tells `name the actor to remove`, asks the DM nothing and sends nothing. | B2 |
| R6 | D1 | While a `remove_actor` the console's Remove sent is unanswered, pressing Remove again asks the DM nothing. | B3 |

**Candidates the sort may refuse:**
- **R2b, the holders in the question.** Only if OD-1 is answered B.
- **A row that the agent's `remove_actor` over MCP asks nothing,** or that
  `remove-token` asks nothing. Refused: the ticket's non-goals; no code
  and no observation built for them.
- **A row that the lookup is by exact id.** Refused: how, not what; R2's
  test, with a padded id, holds it.
- **A row for the opening comment's rule.** Refused: a comment, not
  behaviour.

**The sort at Task 0** kept R1 (with R2's fallback to the id folded in), R3, R4
and R5, and refused R2's exact wording (presentation: the precedent rows,
VTT-569 and VTT-901, pin a name and no words, and R1 holds the name; the
question's words are pinned by the test, not a row) and R6 (a consequence of
D1's place for the question behind the existing lock, whose rows hold that a
second press sends nothing; B3 still runs as a deliberate break), with the
candidates below.

**No existing row becomes false.** Checked and left standing:
**VTT-523**, **VTT-524**, **VTT-526**, **VTT-538** and **VTT-541** (Remove
actor among their controls: a press the DM declines sends nothing and takes
no lock, a press that sends is as before); **VTT-933** (its press answers
yes); **VTT-569**, **VTT-901**, **VTT-902** (the precedents); **VTT-254**,
**VTT-255**, **VTT-259**, **VTT-307**, **VTT-344**, **VTT-906**,
**VTT-924** (the server's).

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R6 and the
  candidates, as OD-1's answer leaves them; allocate ids with
  `requirement-id`; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: Remove asks first (R1 to R6; D1 to D5, D7; items 1 to 3)

- **Files:** `client/src/view/dm.ts` (Remove's press and the comment above
  it); `client/test/dm-view.test.ts` (new tests beside "removing an actor
  sends its trimmed id and clears the field", that test's comment and an
  assertion of one question, and D7's assertions); `docs/requirements.md`
  (the rows' cells); a rebuilt `cmd/vtt/webdist`.
- **Tests first, seen red where today differs:**
  - Over a state holding a named actor and a nameless one, through
    `harness` with `setConfirm(false)`: a padded id of the named actor asks
    once, `confirmMessages()` exactly R2's question with its name, and sends
    nothing; the nameless actor and an id the state does not hold are each
    asked by id; typed through `input`, the field holds what was typed in a
    fresh `harness()` after the no (R1, R2, R4). Red today.
  - With the default yes: "removing an actor sends its trimmed id and clears
    the field" also asserts one question (R3). Red today.
  - With `answer: "held"`: a yes sends one `removeActor`; a second press
    before `release` asks nothing and sends nothing (R6). Green today,
    which asks nothing at all; it holds B3.
  - D7's assertions (R5). Green today; they hold B2.
  - Every test that types into `actor-id` empties it in a `finally`:
    `draft` outlives the harness.
- **True when:** the new tests pass and those marked red failed before;
  the whole client suite, `task client:typecheck` and P6's checks pass; a
  search of `dm.ts` and `dm-view.test.ts` finds neither "exactly one control
  asks first" nor "every seat at this table can see"; the Remove line named
  in P5 is unchanged; `task check:requirements-chain` passes.
- **Deliberate breaks** (M5), each turning a named test red, then reverted:
  B1, B2, B3, B4, B5, B6, B7.
- **The state here:** the DM is asked before any removal the console sends,
  by the actor's name; a no leaves everything as typed.

### Task 2: the gate (item 4)

- **Do:** `python3 tools/check-comments.py --report`, and `--write-ledger`
  once, last, only if `dm.ts` is more than the band under its ceiling (M6);
  `go clean -cache`; check free space and the load (`os.getloadavg()`); run
  `task check` whole on the code commit, detached under `/bin/sh`, output
  to a file, requiring its completion line; run `task e2e`; `go clean
  -cache`. Tell the owner the client's stored mutation report is out of date
  from Task 1 and which commit it measured (SPEC-023).
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the code commit's.

### Task 3: the report, then the specification

- **The report:** `docs/reports/2026-11-22-remove-asks-first.md`, with the
  `implementation-report` skill and its reading review: each "Done looks
  like" item with its observation; the rows and the break that turned each
  red; P4; G1 to G4 and the notes; OD-1's answer; the history cut under
  D5; Phase 4a's adjudications under their own heading; the gate.
- **SPEC-022,** with the `specification` skill, from the report: a paragraph
  on Remove's question (D1 to D4, R1 to R6), beside the console's other
  controls; in "One press, one command", a press the DM declines beside a
  press the console's checks refuse (nothing sent, no lock), and a press a
  lock stops asking nothing; the Status sentence beside Make spectator's,
  naming `client/test/dm-view.test.ts` and the QA file; "Principles served"
  weighed for 2; the "## Requirements" list gains the new ids.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the specification;
  push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, SPEC-022's paragraph as drafted, and the surface
as signatures, never the source: `renderDMConsole(d: DMDeps): HTMLElement`,
the `DMDeps` interface (`st`, `send`, `notify`, `confirm`, `rerender`,
`roster`, `participants`), `State.Actors` and `newState` from
`client/src/state.ts`, `ok`, `refused`, `held` and `settle` from
`client/test/support/answers.ts`, and `client/test/support/dom`. Tell it:
never run `task check`, `task e2e` or Stryker; its file is a new
`client/test/qa-*.test.ts`; tests under `env -u CLAUDECODE bun test`;
`Array.from` over a `NodeList`; the console's typed text outlives a render
in module state, so empty every field it types in a `finally`; try names
with spaces and punctuation, an id padded with white space, an actor
removed from the state between render and press, and a refused and a
rejected send after a yes; the tree is hashed before and after it. Known and
not findings: an id the window does not hold is asked by id and sent (D4);
the agent's MCP removal and `remove-token` ask nothing (non-goals); the
question does not name the holders unless OD-1 is answered B.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check the Remove line named
in P5 is unchanged and `sending` and `unanswered` untouched; read the press's
order against D1; check each comment under `client/src` is a warning or a
pointer, within M6's ceiling and the block bound, and that neither file
keeps a false reason (item 3); check no assertion was weakened or dropped
(rule 2) and that D7's assertions are there; check every test that types
into `actor-id` empties it; check each scope claim in new prose names the
search that bounds it.
