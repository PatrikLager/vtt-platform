# Plan: a player works a door by its button, and a board click always moves

- **Ticket:** `docs/superpowers/specs/2026-11-20-a-door-has-a-button-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-20 by an agent that did not write the ticket, against
    `eca70de` on `feat/a-door-has-a-button` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and `task build:client` were not run. M1 ran in the
    worktree, its `git status` the same before and after; M2 ran in a copy
    of `client/` and `contract/gen/ts` outside the repository; the rest are
    searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-20-a-door-has-a-button.md`.
- **Base:** `eca70de`. **Branch:** `feat/a-door-has-a-button`.
- **Owner's rulings taken in:**
  - 2026-11-19: no arming mode for a player; while the character they watch
    stands beside a door, the panel shows one button per door, "beside"
    being VTT-149's geometry (at most one square on each axis, diagonals and
    the token's own square included); a player's board click always moves,
    or switches (VTT-460).
  - 2026-11-20: every button names the door's direction from the character,
    even with one door: "Open door (east)", "Close door (north)"; north is up
    on the board; the door the token stands in is "here" ("Close door
    (here)").
  - The DM console's "Arm doors" stays as it is; the ruling names players
    only. VTT-876 stands.
  - Standing: an almost theoretical edge case with a small consequence gets
    no code, no test and no debt; equivalent, for a mutant, means no
    observation at all in the DOM; no counts or measured figures in anything
    written down; a code comment under `client/src` is a warning or a
    pointer; the client's full mutation run on the owner's word; push and
    merge on the owner's word.
- **Signed off:** by the owner, 2026-11-20, OD-1 answered A ("Brädets
  läsordning, kör").
- **Ids:** allocated by `requirement-id` at Task 0: R1 VTT-881, R2 VTT-882,
  R3 VTT-883, R4 VTT-884, R5 VTT-885, R7 VTT-886, R8 VTT-887, R9 VTT-888.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol is where it says: `renderPlayerPanel`, `mayWorkAnyDoor`,
   `actingActor`, `PlayerUIState.doorsArmed` and the `arm-doors` toggle in
   `client/src/view/player.ts`; `mayWorkDoor` and `doorCommandFor` in
   `client/src/view/doors.ts`; `ui`, `paint`, `onCell` and `ui.doorsArmed` in
   `client/src/app.ts`; `renderGrid` and "Doors armed — click a door to open
   or close it." in `client/src/view/spectator.ts`; `renderDMConsole` and
   `DMDeps.doorsArmed` in `client/src/view/dm.ts`; `openDoor` and `closeDoor`
   in `client/src/commands.ts`; `Blocked` ("a closed door") in
   `internal/engine/terrain.go`; the test files it names,
   `client/e2e/doors.spec.ts`, `cmd/vtt/webdist`, `docs/requirements.md`
   and SPEC-022. In
   `~/dev/RPTool/maptool`: `DungeonDraftImporter` (a door's portal stroked
   into VBL, `DOOR_VBL_STROKE`), `Topology_Functions` and
   `MacroButtonProperties`.
2. **"Done" is an observation. By reading; no test is named, so none could
   be run red by name.** Items 1 to 3 are false today: `renderPlayerPanel`
   draws no button per door, only the toggle. Item 4 is false today: the
   toggle is offered to a player beside a door (`player.test.ts`'s "a player
   with a controlled token adjacent to a door is offered the toggle", green
   in M1). Item 5 follows from 1. Item 6 names existing tests, green in M1.
   Items 7 and 8 are the e2e and the gate.
3. **Each rule is breakable. By reading.** Each rule has an observation that
   fails (the "Red under" column below). One reservation: item 4's
   "whatever the window's arming bit holds" cannot be observed through a
   window, since no window of a player can hold the bit true (M3); it is
   observable at the panel, by `renderPlayerPanel` handed the bit true (R5),
   and held at the board click by the order of `onCell`'s branches, by
   reading (G4, D9).
4. **The scope matches the claim. By command, then reading.**
   - Callers, by `git grep` over `client/src`: `doorCommandFor` by `onCell`
     alone; the client's `mayWorkDoor` by `doorCommandFor` and
     `mayWorkAnyDoor`; `mayWorkAnyDoor` by `renderPlayerPanel` alone;
     `actingActor` by `onCell`, `renderPlayerPanel` and `moveCommandFor`;
     `openDoor` and `closeDoor` by `doorCommandFor`; `currentSceneId` by
     `mayWorkDoor`, `doorCommandFor`, `mayWorkAnyDoor` and `playerClickFor`.
   - The work stays in the client, as the ticket says: the gateway's
     `mayWorkDoor` (VTT-149, VTT-609, VTT-880) and SPEC-013 are untouched.
   - It reaches files the ticket does not name (G1).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-022's board-click paragraph ("With doors armed, a board click works
     a door or sends nothing: a player's names the actor ... and the
     arm-doors toggle is offered only while one stands so (VTT-873 to
     VTT-875) ... A player's switch ... disarms the door tool (VTT-879)"),
     its perch sentence (`canAct && ui.doorsArmed`), its "Outside the page"
     bullet and its Status paragraph are overturned; the ticket names
     SPEC-022.
   - The 2026-08-30 ticket `2026-08-30-dm-hands-and-retraction-design.md`
     §4.1 decided "Players get this control too"; the owner's ruling of
     2026-11-19, which the ticket quotes, overturns it for players. A
     ticket is not a decision record; said here so the next reader finds
     both.
   - Blueprint 13 (the label in the table's words, never an id), 14 (a
     board click and a button each send only what the participant meant),
     20 (a button only for a door the window holds, beside the watched
     character; the server judges, VTT-149) hold.
6. **The records the work moves are named, and reachable. By command, then
   reading.** The section is present, its path resolves, nothing is `New:`.
   A search of `docs/specifications/` for `arm-doors`, `doorsArmed`,
   `Arm doors`, `door tool` and `armed` finds SPEC-022, SPEC-013 ("door
   tools", the agent's) and SPEC-021 ("charmed"); SPEC-013's hit and the
   gateway's rule it describes do not change.

## The gaps this plan inherits

- **G1. Files and records the ticket does not name:**
  - `client/test/qa-switch.test.ts`: "player: with doors armed, a click on a
    listed party member's token sends nothing, and the door still works"
    arms a player's doors and is cited in VTT-460's evidence cell.
  - `tools/ts-mutation-equivalents.txt`: the reasons of app.ts's `ui`
    ObjectLiteral entry and of player.ts's `actingActor` entry name readers
    and rules that leave, and the comment block under its
    `client/src/view/player.ts` header tells of `mayWorkAnyDoor` (M5).
  - `client/test/command-surface.test.ts`: its warning about the join
    door's `open-door` and `close-door` actions binds the button's action
    (M8, P8).
  - `client/test/spectator-view.test.ts`: a comment names `mayWorkAnyDoor`.
  - `client/test/commands.test.ts`, `client/src/commands.ts` and
    `client/e2e/doors.spec.ts` cite VTT-873, which this work withdraws; the
    e2e is in its evidence cell.
- **G2. "The watched character."** The ticket's rules say it; the code's
  actor is `actingActor`, the one a board click moves, which is the watched
  one once a `ViewpointChanged` has arrived. The rows say "the character
  they watch", as VTT-408 does for the move, and the code uses
  `actingActor`, as the move does.
- **G3. The ticket's "What could not be established"** travels: the first
  is D6, the second D5 and OD-1, the third D11.
- **G4. Item 4's "whatever the window's arming bit holds"** is held at the
  panel by R5's test and at the board click by reading (D9): no window test
  can hold a player's bit true (M3).
- **G5. `currentSceneId`'s `?? ""`** is killed today only through
  `mayWorkAnyDoor`, by `player.test.ts`'s "a prototype-injected empty-string
  scene defeats the `?? \"\"` equivalence claim -- the entry is WITHDRAWN,
  not filed"; removing `mayWorkAnyDoor`
  without porting that test leaves the mutant to survive the next full run
  (D13).

## Measurements this plan stands on

- **M1. The base is green.** In the worktree,
  `env -u CLAUDECODE bun test client/test contract contract-spike` passed
  whole, and `task client:typecheck` passed.
- **M2. What moving `doorsArmed` out of `PlayerUIState` costs.** In a copy of
  `client/` and `contract/gen/ts` with the field deleted from the interface,
  `tsc -p client/tsconfig.json` refused, besides `app.ts` and `player.ts`,
  these test files, each for a `PlayerUIState` literal naming the field:
  `command-surface`, `panel-offers`, `panel-sheet`, `player`,
  `qa-ability-target`, `qa-ability-title`, `qa-door-window`, `qa-panel-say`,
  `qa-panel-sheet`, `qa-property-names`, `qa-refused-line`, `qa-say-boxes`,
  `qa-speaking`, `qa-survivor-rules`, `qa-what-a-window-shows`,
  `qa-window-offers` and `spectator-view`. The `ui` literal in `app.ts`
  holds the code of M5's `app.ts` entries, whose keys move with it.
- **M3. A player's window never holds the arming bit true.** `ui.doorsArmed`
  is set true only by `toggleDoors` (the DM console's) and the player
  panel's toggle; the console is drawn only when `isDM`. `identity.SetRole` is the
  one writer of a role (`git grep 'SET role'`), its one caller is
  `handlePromotion`, which refuses a dm or agent target (VTT-026), and
  `CreateInvite` mints a new participant. So once the panel's toggle is gone
  the bit is true only in a DM's or an agent's window, and such a window
  never becomes a player's. Neither ever perches (`mayPerch`), so `canAct`
  is true wherever the bit can be.
- **M4. The comment ceilings.** `tools/check-comments.py --report` (run with
  `PYTHONDONTWRITEBYTECODE=1`, leaving no file) puts `app.ts`, `commands.ts`,
  `view/dm.ts`, `view/doors.ts`, `view/player.ts` and `view/spectator.ts`
  each at its ceiling in `tools/comment-ceilings.txt` or just under it. A
  comment line fits only beside added code or in place of a removed one; a
  line of requirement ids alone counts in no share. Task 2 removes comment
  lines, which may leave a file more than the band under its ceiling, refused
  until `--write-ledger` runs (Task 4).
- **M5. Adjudications on the code this changes.** None on `view/doors.ts`.
  On `app.ts`, the ObjectLiteral `{}` and StringLiteral entries whose code is
  `const ui: PlayerUIState = { selectedActorId: "", selectedAbilityId: "",
  doorsArmed: false };`; the first's reason lists the bit's readers, among
  them player.ts's toggle (`ui.doorsArmed ? ...`, `!ui.doorsArmed`) and
  `canAct && ui.doorsArmed`. On `view/player.ts`, the StringLiteral entry on
  `actingActor`'s fallback, whose reason says "mayWorkDoor accepts a DM and
  an agent whatever actor is passed" and "doorCommandFor and mayWorkAnyDoor
  ask mayWorkDoor". The code of each stands under D10; their reasons
  must be judged again.
- **M6. An actor may have two tokens on one scene.** `engine.Apply`'s
  `TokenPlaced` arm refuses a token id already held, an unknown scene and an
  unknown actor, and nothing else about the actor; `tokenForActor` moves the
  first token `Object.values(st.Tokens)` finds, so a second token of a
  player's character never moves by a click. A map's or an adventure's
  placements may name one actor twice, since `mapdef` and `loadPlacements`
  refuse only a duplicate token id, and so may the DM's Place token; a scan
  of every `placements` list under `campaigns/` finds no actor placed twice.
- **M7. A door a player's window holds is keyed by whole squares.** A
  player's tiles arrive by `SceneSeen`, which `sceneSeenFor` builds from the
  visible squares (SPEC-016), so a door within one square of a token is one
  of the eight neighbours or the token's own square, and its direction one
  of the ruling's words.
- **M8. `open-door` and `close-door` are taken.** `renderDMConsole`'s join-door
  buttons carry them (`setJoinDoor`), `client/e2e/joining.spec.ts` selects on
  them, and `command-surface.test.ts` warns that a board door command given
  either action would be found reachable by the join door's button.
- **M9. The real wire's geometry.** `doors.spec.ts` moves `tok-fighter` to the
  door's west neighbour on its row, read from
  `campaigns/example/maps/cellar.json`; the door is then east of the fighter,
  and open after the DM's `openDoor` naming the cutpurse, so the player's
  button reads "Close door (east)".

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened. A test of the removed
  mode is deleted only with the row it holds withdrawn (D12); a test whose
  call changes (`doorCommandFor`'s parameters, `mayWorkDoor` becoming
  `doorDirection`) asserts what it asserted, or more.
- **P2. Rules 3, 4 and 5.** No contract change, no fold change; door and
  direction words are not game-system words (semgrep refuses "hp").
- **P3. Rules 8 and 10.** Names, never line numbers. A comment under
  `client/src` is a warning or a pointer; a comment the change makes false is
  corrected or removed in the commit that makes it false (D14); M4 binds
  what may be added.
- **P4. Rule 9,** answered by the ticket and checked (check 1): MapTool has no
  door but VBL, changed by macros run from macro buttons. Borrowed: the act
  is a button in a panel. Not borrowed: the macro; here the window derives
  the buttons from the board and the server judges each command (VTT-149).
- **P5. The owner's rulings above.** No player arming; the DM's arming as it
  is (VTT-876).
- **P6. SPEC-020 and SPEC-023.** The client's stored mutation report goes out
  of date with the first client commit; `task check` names that and passes;
  a full run is the owner's to ask for. An adjudication whose code stands
  but whose reason a change makes false is corrected in that change (D14).
- **P7. The mechanical gates.** A commit that changes `client/src` carries
  `cmd/vtt/webdist` rebuilt by `task build:client`; bun under
  `env -u CLAUDECODE`; `task client:typecheck`;
  `python3 tools/check-new-prose.py` and
  `python3 tools/check-comments.py main` before each commit; stage and
  commit in separate calls; a client test reads no file outside
  `contract/gen` and `contract/testdata`; `Array.from` over a `NodeList`;
  commands started with `/bin/sh`, never zsh. Each task gets Phase 4a's QA
  and one reviewer before its commit.
- **P8. The button's action is `work-door`,** never `open-door` or
  `close-door` (M8).

## Decisions

1. **D1. Which doors.** A player's panel draws a button for each door tile of
   the current scene for which `doorDirection` (D2) answers, asked for
   `actingActor(st, me, ui)`.
   - **Forced by** the ruling (beside the watched character, VTT-149's
     geometry) and by the move: `moveCommandFor` moves `actingActor`'s token,
     so the door and the move name one character (G2).
2. **D2. One search answers whether, and which way.** `mayWorkDoor` becomes
   `doorDirection(st, me, actorId, cell)`: the direction from the first token,
   in `Object.values(st.Tokens)`'s order, of `actorId` on the current scene,
   whose actor lists `me.participantId` among its controllers, standing at
   most one square from `cell` on each axis, to `cell`; undefined when none
   stands so, and for every role but player (D7).
   - **Forced by** the label needing the token the predicate found: a second
     search could choose another token only in M6's case, which no test can
     pin without building it (D6). The geometry tests, which assert true or
     false today, assert a direction or undefined: more, never less (P1).
3. **D3. The words.** A table from the step `(dx, dy)` to "north-west",
   "north", "north-east", "west", "here", "east", "south-west", "south",
   "south-east", with dy below zero north; the label is `Open door
   (<direction>)` for a door `scene.OpenDoors?.[key]` holds closed and
   `Close door (<direction>)` for one it holds open, read as `doorCommandFor`
   reads it. No fallback for a step outside the table.
   - **Forced by** the ruling's words; north up because the board draws row
     `gy` at `gy * cell * scale + offsetY` (`client/src/view/scene-plan.ts`),
     so a smaller y is higher; a table because each entry is one string a
     test names, where computed words add boundary operators for mutants to
     sit on; no fallback by M7.
4. **D4. What a button sends.** `openDoor(sceneId, at, actorId)` or
   `closeDoor(...)`, the builders unchanged, `actorId` being `actingActor`'s.
   `doorButtonsFor(st, me, actorId)` in `client/src/view/doors.ts` returns,
   per door, its label and its command, so the panel draws and sends and
   decides nothing.
   - **Forced by** item 3, and by the module holding the rule as a tested
     pure function, as `doorCommandFor` does for the DM.
5. **D5. Where they stand, and in what order.** In a `div.row.doors` after the
   target buttons or "Click the board to move.", where the toggle stands
   today, and before the Say form, on every page; the row is drawn only when
   it holds a button; each button is a `.chip` with `data-action="work-door"`;
   ordered by the door's row, top to bottom, then its column, left to right
   (OD-1 A).
   - **Forced by** SPEC-022's "Outside the page" (an act that is not on a
     sheet's page stands below it on every page, as the toggle does), P8,
     and the `Tiles` map's order being the wire's (`map<string, TileRef>`,
     and a player's tiles arrive in pieces by `SceneSeen`), which tells a
     player nothing. The order itself is a preference, put to the owner as OD-1.
6. **D6. An actor with two tokens on one scene** (the ticket's first open
   point) is theoretical (M6): the direction is measured from the first of
   them standing beside the door in `st.Tokens`' order, which is the token a
   click moves whenever that token stands beside it, and a door gets one
   button because the function walks doors, not tokens. No code and no test
   for the case.
   - **Forced by** the owner's standing rule; the consequence is a label
     measured from the other token beside the same door, and the button works
     the door either way (the gateway asks for any controlled token beside
     it, VTT-149).
7. **D7. No buttons in a DM's or an agent's panel.** `doorDirection` refuses
   every role but player, so the player panel a DM's or an agent's window
   draws beside its console (`canAct` admits them) holds no door button,
   whatever its chips select.
   - **Forced by** the ruling (players only) and VTT-876: a button there
     would name the chip's actor.
8. **D8. `doorCommandFor` is the DM's and the agent's alone.** It becomes
   `doorCommandFor(st, armed, cell)`: null unarmed, with no scene, or off a
   door, otherwise `open_door` or `close_door` at `cell` naming no actor. It
   asks nothing of the role or an actor.
   - **Forced by** D9: only `onCell`'s branch for a window that is not a
     player's calls it, and that window is a DM's or an agent's (`canAct`
     excludes a spectator and a perching player), for which `mayWorkDoor`
     answers true and the opener is empty; the player branch and the actor
     parameter would be dead. Nobody uses the product, so the old shape is
     deleted, not kept beside the new.
9. **D9. A player's board click never reads the bit.** `onCell` sends
   `isPlayer ? playerClickFor(...) : doorCommandFor(st, ui.doorsArmed, cell)
   ?? (ui.doorsArmed ? null : moveCommandFor(...))`.
   - **Forced by** item 4 and the ruling. The order of the branches is what
     holds "whatever the bit holds" (G4); `actingActor`'s import into
     `app.ts` goes with the call that used it.
10. **D10. The bit stays in `PlayerUIState`.** Its doc says it is the DM's and
    the agent's: whether their board click works a door instead of moving a
    token (SPEC-022).
    - **Forced by** M2 (the field named in the literals of every test file
      listed there, and in the `app.ts` line M5's entries key on) for a
      move that changes no behaviour, and by the type already being the
      window's for every role that acts: a DM's chips select through
      `ui.selectedActorId`.
11. **D11. The role change** (the ticket's third open point) cannot happen
    (M3). `paint`'s `ui.doorsArmed = false` on a player's switch (VTT-879) is
    removed, the bit being false in every player's window; the board is
    handed `ui.doorsArmed` in place of `canAct && ui.doorsArmed`, since
    `canAct` is true wherever the bit can be true and the guard would leave a
    mutant no DOM can tell apart.
    - **Forced by** M3 and the owner's rule on equivalence.
12. **D12. The register.** R1 to R5, R7, R8 and R9 under the sort at Task 0
    (R6 refused). VTT-873, VTT-874, VTT-875 and VTT-879 are
    withdrawn in Task 2, whose code removes the mode they describe, each
    succeeded as listed under Requirements. VTT-876 and VTT-460 stand, their
    evidence cells following the tests that move.
13. **D13. Tests move; their kills stay.** Each test of `mayWorkAnyDoor` and
    of the player branch of `mayWorkDoor` and `doorCommandFor` that kills a
    mutant the new code still carries is ported to `doorDirection`,
    `doorButtonsFor` or the panel, asserting what it asserted: the current
    scene as the greatest id; no scene; a tile that is not a door; a token
    on another scene; another participant's token; a token whose actor is
    missing; a spectator; two squares off on one axis; a scene with no
    `OpenDoors`; and the prototype test of G5, against `doorButtonsFor`.
    The armed click on a plain square sending nothing moves from a player's
    table to a DM's, since only a DM's or an agent's window still arms.
    - **Forced by** P1, and by a deleted test taking with it coverage nobody
      named.
14. **D14. Comments and reasons follow the code.** In the commit that makes
    them false: `doors.ts`'s header (the predicate "shared by the DM console
    and the player panel"; "a later task (arming a board mode)") and the docs
    of `mayWorkDoor` and `doorCommandFor`; player.ts's toggle comment,
    `mayWorkAnyDoor`'s and `actingActor`'s docs; `app.ts`'s `toggleDoors` doc
    (which names the player panel), `paint`'s pointer to VTT-879 and the
    door tool, `onCell`'s comment; `dm.ts`'s `DMDeps.doorsArmed` doc ("the
    player panel offers the same toggle") and the doors group's comment
    (`mayWorkAnyDoor`, "unlike a player, who is offered the control");
    `commands.ts`'s pointer to VTT-873; M5's reasons, the `view/player.ts`
    block of `tools/ts-mutation-equivalents.txt` and its file-header block
    (the "Fix round 1's own review" paragraph), which points to the
    `player.test.ts` toggle tests Task 2 deletes or rewrites. Each new
    line is a warning or a pointer, by requirement id where a ceiling binds
    (M4).
    - **Forced by** rule 10, SPEC-020's "nothing checks that it was" and M4.
15. **D15. `command-surface.test.ts`'s rows stay `board`.**
    - **Forced by** the DM's and the agent's armed board click, which still
      sends both commands (VTT-876), so the table's one surface per command
      stays true; its warning holds because the button's action is
      `work-door` (P8).
16. **D16. The real wire.** `doors.spec.ts` keeps its table and its DM's
    `openDoor` naming the cutpurse; the player, in place of arming and
    clicking the door square, clicks the button "Close door (east)" (M9), and
    the spec asserts the player's page offers no `arm-doors` control.
    `between` goes with the board click. Every other assertion stands. Its
    `// VTT-873` becomes `// VTT-883`, and VTT-883's evidence cell gains the
    spec, VTT-873 being withdrawn in Task 2.
    - **Forced by** item 7.
17. **D17. Every client commit carries its bundle** (P7): Tasks 1 and 2.
18. **D18. The specification last,** from the report: SPEC-022's Status
    paragraph, its board-click paragraph and perch sentence, its "Outside
    the page" bullet and its "## Requirements" list.

## Open decisions for the owner

- **OD-1. The buttons' order** (the ticket's second open point).
  - **A (recommended).** The board's reading order: the door's row top to
    bottom, then its column left to right, so the buttons stand as the doors
    stand on the board, "here" between west and east.
  - **B.** Compass order clockwise from north, "here" first.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the register's
words are the ones that bind. Every new row reads `**OPEN — no test yet**`
until its task cites its tests.

| R | Item | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | A player's panel draws one `work-door` button for each door of the current scene that a token on that scene of the character they watch stands at most one square from on each axis, its own square included, and no other. | a door two squares off on one axis offered; a diagonal door or the token's own square missing; a door beside another of their characters offered; a door of another scene; a wall tile; two buttons for one door; the previous character's doors after a switch |
| R2 | 2 | A door button reads "Open door" for a door the window holds closed and "Close door" for one it holds open, followed in parentheses by the door's direction from that token, one of north, north-east, east, south-east, south, south-west, west and north-west, north being toward the top of the board, or "here" for the square the token stands on. | the verb inverted; north and south, or east and west, swapped; the direction measured from the door; "here" missing; the hyphen dropped |
| R3 | 3 | A click on a door button sends `open_door` for a closed door and `close_door` for an open one, at that door's square in the current scene, naming the character the player watches. | no actor named; another of theirs named; the wrong verb; another scene; x and y transposed |
| R4 | 4 | A player's board click on a door square never sends `open_door` or `close_door`, and sends what it sends on a square with no door (VTT-408, VTT-460). | a door command sent; the move or the switch withheld on a door square |
| R5 | 4 | A player's panel draws no arm-doors toggle, whatever `PlayerUIState.doorsArmed` holds. | the toggle, or "Doors armed", drawn with the bit true or false |
| R7 | ruling | A DM's or an agent's window draws no door button, whatever actor its chips select. | a button for the chip's actor beside a door |
| R8 | OD-1 | A player's panel orders its door buttons by the door's row, top to bottom, then by its column, left to right. | sorted by column first; in the `Tiles` map's order; in compass order |
| R9 | ruling | With doors armed in a DM's or an agent's window, a board click on a door square of the current scene sends `open_door` or `close_door` at that square, and a click on any other square sends nothing. | the armed click on a plain square sends a move; the armed click on a door sends nothing |

**Candidates the sort may refuse:**
- **R6 (item 5), the buttons follow a switch.** Recommended refused: R1 says
  "the character they watch", and VTT-407 says the window shows as watched
  the actor the last `ViewpointChanged` named; the window test after a switch
  cites R1.
- **A row for the label being measured from the token a click moves.**
  Refused: the case where it matters is D6's, theoretical.
- **A row for the buttons' order.** Accepted at Task 0 as R8, once the
  owner answered OD-1: the order is a decision a player sees, and Task 1's
  break "sort by column first" needs a row for its test to cite.
- **A row for the DM's and the agent's armed click** (R9). Accepted at Task
  0 on review: the tests that pin it are in no row, VTT-876 says only that
  the command names no actor, and Task 2's break on
  `?? (ui.doorsArmed ? null : ...)` needs a row its test can cite.
- **A row that a player's window never holds the bit true.** Refused: M3
  holds it in the gateway (VTT-026), and no window test can break it.

**Withdrawn in Task 2:**
- **VTT-873**: "No door mode remains in a player's window: a player works a
  door by its button. Succeeded by VTT-883."
- **VTT-874**: the same reason; "Succeeded by VTT-881 and VTT-884."
- **VTT-875**: "No arm-doors toggle remains in a player's panel. Succeeded
  by VTT-881 and VTT-885."
- **VTT-879**: "No door tool remains in a player's window to disarm.
  Succeeded by VTT-881."

**Rows checked and left standing,** their evidence cells following any test
renamed or removed:
- **VTT-876** (a DM's or an agent's door command names no actor): its
  `doors.test.ts` and `qa-door-window.test.ts` tests lose "whatever actor it
  is handed" from their names, since `doorCommandFor` is handed none (D8).
- **VTT-460**: true as worded ("With doors not armed" now always holds in a
  player's window); its cell loses `qa-switch.test.ts`'s armed test and
  gains its rewrite (Task 2).
- **VTT-408**, **VTT-407**, **VTT-461**, **VTT-147**, **VTT-149**,
  **VTT-609**, **VTT-880**, **VTT-026**.

## The existing tests that change

- **Task 1:** none.
- **Task 2, by reading, measured before its commit:**
  - `client/test/doors.test.ts`: every `doorCommandFor` call drops the role
    and the actor; the `mayWorkDoor` tests become `doorDirection` tests (D2,
    D13), "DM and agent both pass unconditionally" asserting undefined (R7);
    "a player's door names the actor a click moves ..." (VTT-873, VTT-874)
    deleted, its cases ported to Task 1's `doorButtonsFor` tests; VTT-876's
    test renamed.
  - `client/test/qa-door-window.test.ts`: the module-level player tests
    ported to `doorButtonsFor` (R1, R3), the toggle test to the buttons (R1),
    VTT-876's renamed; the window test rewritten with buttons; the DM's
    window test unchanged.
  - `client/test/player.test.ts`: the toggle section becomes the buttons'
    (D13); "the toggle's class and label ..." deleted, succeeded by R5's
    test.
  - `client/test/app.test.ts`: "a player's armed door click names ...",
    "a switch to another character beside a door leaves the toggle ...",
    "with doors armed, a player's click on a door ..." and "a player who
    perches while doors are armed ..." deleted, their premise unbuildable;
    "with doors armed, a click on a door square opens it ..." and "with doors
    armed, a click on a plain square sends nothing at all" move to a DM's
    table and cite R9; "with doors disarmed, the same click moves the token instead"
    stands and cites R4; the comment naming the player panel's toggle is
    corrected.
  - `client/test/qa-switch.test.ts`: the armed test rewritten as a click on a
    listed party member's token standing in an open door, which switches
    (R4, VTT-460).
  - `client/test/commands.test.ts`: the fixture test cites R3.
  - `client/test/spectator-view.test.ts`: the comment naming
    `mayWorkAnyDoor` corrected.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R7 and the
  candidates, as the answers leave them; allocate ids with `requirement-id`;
  commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: the buttons (R1, R2, R3, R7, R8; D1 to D7; items 1 to 3 and 5)

- **Files:** `client/src/view/doors.ts` (`doorDirection`, `DoorButton`,
  `doorButtonsFor`, beside `mayWorkDoor`, which Task 2 removes);
  `client/src/view/player.ts` (the row, after the toggle, which stays for
  now); `client/test/doors.test.ts`, `client/test/player.test.ts`,
  `client/test/app.test.ts`; `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **Tests first, seen red:**
  - `doorButtonsFor`: a token with a door on each of its neighbours and an
    open one under it reads each of the ruling's words once, "Close door
    (here)" for its own square, in OD-1's order; a closed door's button
    opens and an open one's closes, at its square, naming the actor; a door
    two squares off on one axis, a wall, a door on another scene, a door
    beside another character of the player's or beside another
    participant's token get none; a scene with no `OpenDoors` reads closed;
    a DM and an agent get none.
  - The panel: one `.chip[data-action="work-door"]` per button with its
    label, in the `.doors` row below the hint and above the Say form; no
    `.doors` row with none; a click sends the button's command once; a DM's
    panel with a chip on an actor beside a door draws none.
  - The window, through `boot`: a player watching the character beside a
    door clicks its button and `open_door` names that character; after a
    switch to one beside another door, the buttons are that door's, and to
    one beside none, there are none.
- **True when:** the new tests pass and failed before; the whole client
  suite, `task client:typecheck` and P7's checks pass.
- **Deliberate breaks:**
  - Swap "north" and "south" in the table: the every-neighbour test turns
    red.
  - Measure from the door to the token: the same test turns red.
  - Drop a hyphen, or word the own square otherwise than "here": the same.
  - Widen the reach to two squares, or measure Manhattan: the two-off and
    diagonal cases turn red.
  - Answer for a DM: R7's tests turn red.
  - Drop the controller check: the other participant's token case turns red.
  - Ask any of the player's characters, not `actingActor`'s: the other
    character case and the switch test turn red.
  - Drop the door-tile filter: the wall case turns red.
  - Sort by column first: the order assertion turns red.
  - Invert the verb, or name no actor: R2's and R3's tests turn red.
  - Read `scene.OpenDoors![key]`: the no-`OpenDoors` case throws.
  - Draw the `.doors` row with no button: the no-row case turns red.
- **The state here:** a player sees a button per door beside the watched
  character and can use it; the toggle and the armed click still stand.

### Task 2: the mode leaves (R4, R5, R9; D8 to D15; item 4; withdrawals)

- **Files:** `client/src/view/player.ts` (the toggle and `mayWorkAnyDoor`
  removed, `actingActor`'s doc); `client/src/view/doors.ts` (`mayWorkDoor`
  removed, `doorCommandFor(st, armed, cell)`, the header and docs);
  `client/src/app.ts` (`onCell`, `paint`'s disarm, the board's bit, the
  import, the comments); `client/src/view/dm.ts` and `client/src/commands.ts`
  (comments); `tools/ts-mutation-equivalents.txt` (M5's reasons, the
  `view/player.ts` block); the tests listed under "The existing tests that
  change"; `docs/requirements.md` (the withdrawals and evidence cells); a
  rebuilt `cmd/vtt/webdist`.
- **Tests first, seen red:** `renderPlayerPanel` for a player beside a door
  with `doorsArmed` true and false draws no `arm-doors` control and no
  "Doors armed" (R5); in a player's window beside a door, no `arm-doors`
  control is offered and a click on the door square sends `move_token` of
  the watched character's token (R4); a click on a listed party member's
  token standing in an open door sends `set_viewpoint` (R4, VTT-460); the
  ported prototype test against `doorButtonsFor` (G5).
- **True when:** the new and ported tests pass and the new ones failed
  before; the whole client suite, `task client:typecheck` and P7's checks
  pass; VTT-876's, VTT-460's, VTT-408's and VTT-461's tests and the DM's
  console and board-label tests (`dm-view.test.ts`, `app.test.ts`'s "the
  DM's own console toggle arms doors, board-wide", `spectator-view.test.ts`'s
  armed-label tests) pass; each code line of M5's entries is found in
  its file as often as `copies` says; `task check:requirements-chain` passes.
- **Deliberate breaks:**
  - Draw the toggle again for a player: R5's test turns red.
  - Hand a player's click to `doorCommandFor` armed: not observable through a
    window (M3); R5's test holds the panel half and the branch order is read
    by the reviewer (G4).
  - Drop the DM's `?? (ui.doorsArmed ? null : ...)`: the DM's armed click on
    a plain square sends a move and its moved test turns red.
  - Name an actor in `doorCommandFor`: VTT-876's tests turn red.
  - Hand the board `false`: "the DM's own console toggle arms doors,
    board-wide" turns red.
  - Change `currentSceneId`'s `?? ""` to another string: the ported
    prototype test turns red.
- **The state here:** a player works a door only by its button; their board
  click moves or switches; the DM arms as before.

### Task 3: the real wire (D16; item 7)

- **Files:** `client/e2e/doors.spec.ts`; `docs/requirements.md` (VTT-883's
  evidence cell).
- **True when:** `task e2e` passes, the spec among it.
- **Deliberate breaks:**
  - Name no actor in `doorButtonsFor`'s command: the spec turns red (the
    Story reads "a door closed").
  - Swap east and west: the spec finds no "Close door (east)".
- **The state here:** the whole table, through a browser and a server.

### Task 4: the gate (item 8)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once,
  last, if a ceiling must come down (M4); check free space and the load
  (`os.getloadavg()`); run `task check` whole on the last code commit,
  detached under `/bin/sh`, output to a file, requiring its completion line;
  run `task e2e`. Tell the owner the client's stored mutation report is out
  of date from Task 1 and which commit it measured (SPEC-023); a full
  Stryker run is his to ask for.
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the last client commit's.

### Task 5: the report, then the specification

- **The report:** `docs/reports/2026-11-20-a-door-has-a-button.md`, with the
  `implementation-report` skill and its reading review: each "Done looks
  like" item with its observation; the rows and what turned each red; Rule
  9; G1 to G5 and OD-1's answer; M3 as the reason item 4's last clause is
  held by reading; Phase 4a's adjudications under their own heading; the
  gate.
- **SPEC-022,** with the `specification` skill, from the report: the Status
  paragraph's door sentence and its tests; the board-click paragraph (a
  player's click never works a door; the door buttons; the DM's and the
  agent's armed click naming none); the perch sentence (the bit true only
  in a DM's or an agent's window, which never perches); the "Outside the
  page" bullet (the door buttons in place of the toggle); the
  "## Requirements" list (the new ids; the withdrawn ones out).
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the specification;
  push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, SPEC-022 as drafted, and the surfaces as
signatures, never the source: `doorButtonsFor`, `doorDirection`,
`doorCommandFor`, `currentSceneId`, `actingActor`, `renderPlayerPanel`,
`PlayerUIState`, `openDoor`, `closeDoor`, and `boot` with its fetch and socket
seams as `app.test.ts` uses them. Tell it: never run `task check`, `task e2e`
or Stryker; its files are new `qa-*` test files; client tests under
`env -u CLAUDECODE bun test`; a client test reads no file outside
`contract/gen` and `contract/testdata`; `Array.from` over a `NodeList`;
fixtures with two characters and doors on several sides, the one asked for
not first; the tree is hashed before and after it. Known and not findings:
a DM's and an agent's armed click and its label (VTT-876); the bit cannot be
true in a player's window (M3), so a test that builds that window by a fake
`/api/me` turning a DM into a player tests a state the gateway cannot
produce; an actor with two tokens on one scene (D6); a closed door a token
stands on, which a DM can make, reads "Open door (here)" (R2 as worded).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check each
comment under `client/src` is a warning or a pointer, and that none D14
names still describes the toggle or the player's armed click; check every
assertion this work changed for being weakened rather than moved (rule 2),
the ported kills of D13 among them; read `onCell`'s branch order against
item 4 and M3; read M5's reasons against the code they now describe;
and check the button's action is `work-door`.
