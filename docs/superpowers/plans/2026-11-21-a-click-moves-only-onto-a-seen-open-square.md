# Plan: a board click moves only onto a square the character sees and can enter

- **Ticket:** `docs/superpowers/specs/2026-11-21-a-click-moves-only-onto-a-seen-open-square-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-21 by an agent that did not write the ticket, against
    `b90fc1a` on `fix/a-click-moves-only-onto-a-seen-open-square` in the
    worktree `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and `task build:client` were not run. M1 ran in the
    worktree, its `git status` the same before and after; M2 ran from outside
    the repository against the worktree's source; M3 to M5 ran in a copy of
    the tree made by `git archive HEAD` outside the repository; the rest are
    searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-21-a-click-moves-only-onto-a-seen-open-square.md`.
- **Base:** `b90fc1a`. **Branch:** `fix/a-click-moves-only-onto-a-seen-open-square`.
- **Owner's rulings taken in:**
  - 2026-11-21: a player's board click moves only onto a square the watched
    character sees now (`Scene.Visible`), not a remembered one and not an
    unseen one; a click outside the grid, or on a square not seen now, does
    nothing at all, no command and no warning; a seen square the character
    cannot enter (a wall, a closed door, scenery that blocks movement) does
    nothing either. A DM's and an agent's click inside the grid stays as
    today; only off the grid does theirs become nothing. The server's move
    gate and its texts (VTT-601, VTT-602) do not change.
  - Standing: an almost theoretical edge case with a small consequence gets
    no code, no test and no debt; one source per fact, pointed at rather
    than copied; no counts or measured figures in anything written down; a
    code comment under `client/src` is a warning or a pointer; the client's
    full mutation run on the owner's word; push and merge on the owner's word.
- **Signed off:** by the owner, 2026-11-21, with OD-1 answered A: a player's
  click reads a scene the window holds no `Visible` for as every square seen.
- **Ids:** allocated at Task 0 with `requirement-id`, after sign-off: R1 is
  VTT-934, R2 VTT-935, R3 VTT-936, R4 VTT-937, R5 VTT-938, R6 VTT-939, R7
  VTT-940. The register's words bind; the table below keeps the plan's
  labels.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol is where it says: `renderGrid` and its click handler in
   `client/src/view/spectator.ts`; `cellFromPoint` and `tokensOnScene` in
   `client/src/view/grid.ts`; `worldFromScreen` (in
   `client/src/view/camera.ts`, which the ticket does not name); `onCell` in
   `client/src/app.ts`; `playerClickFor` and `moveCommandFor` in
   `client/src/view/player.ts`; `doorCommandFor` in
   `client/src/view/doors.ts`; `planFog` in `client/src/view/scene-plan.ts`;
   `Scene.Visible`, `Tiles`, `OpenDoors` and `Objects` in
   `client/src/state.ts`; `handleCommand` and both refusal texts in
   `internal/gateway/server.go`; `engine.State.Blocked` in
   `internal/engine/terrain.go`; VTT-601 and VTT-602 in the register with
   those texts; the named tests in `client/test/grid.test.ts` and
   `client/e2e/table.spec.ts`; `cmd/vtt/webdist`; SPEC-022 and SPEC-013. In
   `~/dev/RPTool/maptool`: `Zone.isPointVisible`, called by `ZoneRenderer`'s
   `renderLabels`.
2. **"Done" is an observation. By command where it could be.** Items 1 to 3
   name tests still to be written, each to fail on today's tree; M2 shows
   the behaviour each one observes is today's: a point off each side of the
   grid comes back as an edge square, and a player's click on a never-seen
   square, a remembered square, a seen wall, a seen closed door and a seen
   square of blocking scenery each sends `move_token`. Items 4 and 6 name
   existing tests, green in M1. Item 5 is an e2e, which fails today because
   the clamped click reaches the server (D11). Item 7 is the gate.
3. **Each rule is breakable. By reading.** Each of the ticket's three rules
   has an observation that fails: a command frame from a click past an
   edge; a `move_token` or `set_viewpoint` from a click on a square not seen
   now; a `move_token` onto a seen wall, closed door or blocking footprint
   (the "Red under" column below). The ticket's sentence "VTT-408 and VTT-919
   stand as worded" is not one of its rules, and it does not hold for VTT-919
   (G2).
4. **The scope matches the claim. By command, then reading.**
   - Callers, by `git grep` over `client`: `cellFromPoint` by `renderGrid`'s
     handler alone, and by `grid.test.ts`, `camera.test.ts` and
     `spectator-view.test.ts`; `playerClickFor` by `onCell` alone, and by
     `player.test.ts` and `qa-window-offers.test.ts`; `onCell` by every
     window test that clicks the board.
   - What a changed `cellFromPoint` and `playerClickFor` turn red or leave
     vacuous was measured (M3, M4), and reaches test files the ticket does
     not name (G1).
   - The server is untouched, as the ticket says; the drift guard of D6
     adds a Go test and a comment line beside `engine.State.Blocked` (G1).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-022's board-click paragraph ("Any other click, with no ability
     armed, moves the watched one's token ... (VTT-408)"; "A player's board
     click on a door square sends what the same click sends on a square with
     no door ... (VTT-884, VTT-885)"; the switch "(VTT-919)") is overturned;
     the ticket names SPEC-022.
   - The owner's ruling of 2026-11-19 that a player's board click always
     moves, or switches, taken into
     `docs/superpowers/plans/2026-11-20-a-door-has-a-button.md` and written
     into SPEC-022 and VTT-884, is overturned for a square not seen now or
     blocked by the ruling of 2026-11-21, which the ticket quotes. A plan is
     not a decision record; said here so the next reader finds both.
   - SPEC-013's move gate and its Consequences ("moves only onto a square
     the party member SPEC-015 judges the move by can see now") stand: the
     client now sends less, and the server judges what reaches it as before.
     SPEC-016 stands; D3 relies on it. Blueprint 14 (looking never acts) and
     20 (a player acts only on what their character perceives) are served,
     not contradicted. The "Bara knappar" ruling (a perching window sends
     nothing) stands.
6. **The records the work moves are named, and reachable. By command, then
   reading.** The section is present, its one path resolves, nothing is
   `New:`. A search of `docs/specifications/` for `click`, `clamp`,
   `cellFromPoint`, `Visible`, `seen now` and `see now` finds SPEC-022 (the
   board-click paragraph, which moves), SPEC-013 (the server's gate, which
   does not), SPEC-016 (the projection, which does not) and SPEC-021
   ("unclamped", a cursor, unrelated). No specification describes the clamp.

## The gaps this plan inherits

- **G1. Files and records the ticket does not name:**
  - Tests that turn red or vacuous (M3, M4, listed under "The existing
    tests that change"): `client/test/qa-door-mode.test.ts`,
    `client/test/qa-one-log.test.ts`,
    `client/test/qa-what-a-window-shows.test.ts`,
    `client/test/token-card.test.ts`, and under D3
    `client/test/qa-switch.test.ts`, `client/test/qa-console-perch.test.ts`,
    `client/test/qa-perched-player.test.ts`,
    `client/test/qa-window-offers.test.ts` and
    `client/test/qa-door-window.test.ts`.
  - New: `client/src/view/terrain.ts`, `client/test/terrain.test.ts`,
    `contract/testdata/blocked_squares_log.json`,
    `contract/testdata/blocked_squares.json`, `client/e2e/board-clicks.spec.ts`.
  - Changed outside `client/src`: `internal/engine/terrain_test.go` and a
    citation line in `internal/engine/terrain.go` (D6);
    `client/e2e/setup.ts`, which serves the cellar to the doors table alone
    (D11).
  - `docs/requirements.md`: VTT-884 and VTT-919 are withdrawn; VTT-885's
    evidence cell names a test whose name changes (D10).
- **G2. VTT-919 does not stand as worded.** Item 2 says a click on a square
  not seen now sends "neither a move nor a switch", so VTT-919's "on a cell
  holding a token of a party member ... sends `set_viewpoint`" is false for
  such a square. VTT-884 is false as worded too: a seen closed door now
  sends nothing where a square with no door moves. VTT-408 stands. Both are
  withdrawn and succeeded (D10).
- **G3. The ticket's Rule 9 answer is incomplete.** MapTool does refuse a
  player's movement on the client: `AbstractAStarWalker` blocks a step whose
  centre-to-centre line leaves the exposed fog area for a view that is not
  the GM's (`fowBlocksMovement`), and one that crosses movement-blocking
  topology (`vblBlocksMovement`, over MBL, and every VBL type when the
  server policy's `getVblBlocksMove` says so). P4 carries the answer.
- **G4. "A panned or zoomed map."** The client has no pan and no zoom: the
  margin around the map is `fitCamera` centring the scene on the axis with
  slack, and on the other axis only the board's border lies off the grid.
  A unit test can still click past each of the four edges, and the work is
  the same.
- **G5. The ticket's "What could not be established"** travels: the first
  point is D7, the second D3 and OD-1 (traced, M8), the third M7 (traced:
  the e2e stays green), the fourth D6.
- **G6. An agent's window.** Item 1 names a player's and a DM's window. An
  agent's window takes the DM's branch of `onCell` (`isDM` holds for both),
  and the off-grid answer is made before any branch (D2), so the DM's tests
  stand for it; held by reading.

## Measurements this plan stands on

- **M1. The base is green.** In the worktree,
  `cd client && env -u CLAUDECODE bun test ./test` passed whole.
- **M2. Today's behaviour.** A bun test outside the repository, importing
  the worktree's `cellFromPoint` and `playerClickFor`: a point beyond the
  left, right, top and bottom edge each answered an edge square; a player's
  click on a never-seen square, a remembered one (`Explored`, not
  `Visible`), a seen wall, a seen door never opened and a seen square of a
  `BlocksMove` object each answered `moveToken`.
- **M3. What the change turns red.** In the copy, `cellFromPoint` answered
  nothing off the grid, `renderGrid`'s handler called `onCell` only with a
  square, and `playerClickFor` checked seen-now and blocked as D3 to D5
  say; the client suite ran under two readings of an absent `Visible`.
  - Read as every square seen (reading A), the red tests were classes (a),
    (b) and (c) below, and nothing else: no existing test clicks a square
    that a fixture holding a `Visible` leaves unseen.
  - Read as no square seen (reading B), the red tests were those and
    class (e)'s red list.
- **M4. What the change leaves vacuous.** In the copy, a trace of each click
  that fell off the grid, and of each `playerClickFor` answered by the
  seen-now check, named the tests that stay green for a new reason: class
  (d), and class (e)'s vacuous list under reading B.
- **M5. Bun's `toEqual` holds `[]` equal to `[undefined]`,** and
  `toHaveLength(0)` and `toStrictEqual([])` do not. A spy test asserting
  "no call" with `toEqual([])` passes while `onCell(undefined)` is called;
  "a board click maps to the cell under the pointer, offset by the board's
  box" is green after the change only through this (class d).
- **M6. The comment ceilings.** `tools/check-comments.py --report` (run with
  `PYTHONDONTWRITEBYTECODE=1`) puts `view/grid.ts`, `view/player.ts`,
  `view/spectator.ts` and `internal/engine/terrain.go` each at its ceiling in
  `tools/comment-ceilings.txt` or just under it. `cellFromPoint`'s doc and
  `Blocked`'s doc are blocks above check:comments' block bound, so the change
  may add a line to neither unless the block ends within the bound; a `//` line
  of requirement ids alone counts in no share and belongs to no block. A new
  file is held to the default ceiling.
- **M7. `table.spec.ts`'s move stays green. By reading.** In Playwright's
  default viewport the board is its column's width and as tall as
  `style.css`'s clamp makes it; the camera fits the square ravine of `adventures/goblin-ambush` with a margin
  left and right, and the click's offset into `.grid` lies between the
  margins and inside the map's height, so it lands on a square of the grid.
  Every tile of `ravine.json` is `earth`, which `mapdef`'s standard table
  makes a floor, and the scene has no object, so Lera, placed at the
  console's default square, sees every square and none blocks.
- **M8. A player's window can hold a scene with no `Visible`.** SPEC-016:
  "standing in a scene is what earns its board", and a `SceneSeen` goes out
  only for a visible set that differs from the last one sent, "nothing sent
  counting as empty" (`sameSet`). So the scene's `SceneCreated` arrives
  before its first `SceneSeen`, one envelope per frame, each folded and
  painted (`Session.ingest`), and when the watched token sees no square the
  `SceneSeen` never comes. `sight.Clear` lets a viewer see its own square
  (a blocker holding the origin does not block), so that happens only for a
  token off the grid; `engine.Apply`'s `TokenPlaced` and `TokenMoved` arms
  bound no position, and the move gate is a player's alone, so a DM's or an
  agent's command can put one there.
- **M9. A seen square's data in a player's window is the server's.**
  `sceneSeenFor` sends each visible square's tile and every object any
  square of whose footprint is visible, whole; `doorTransitions` corrects
  the window's belief about every door on a visible square; `Visible` holds
  only squares of the grid (`sight.VisibleFrom` walks no other). So for a
  square `Visible` names, the window's tile, `OpenDoors` entry and objects
  are what `engine.State.Blocked` reads.
- **M10. No adjudication sits on the changed code.** Neither
  `tools/ts-mutation-equivalents.txt` nor `tools/mutation-equivalents.txt`
  has an entry on `cellFromPoint`, `renderGrid`'s handler, `playerClickFor`
  or `terrain.go`; app.ts's `ui` entry reads `onCell`'s
  `ui.doorsArmed ? null : ...`, which does not change.
- **M11. A shared fixture is the house's shape.**
  `contract/testdata/resource_change_log.json` and
  `resource_change_refused.json` are held by
  `cmd/vtt/resource_change_dump_test.go` and
  `client/test/fold-resources.test.ts` alike, and
  `internal/engine/qa_ability_change_test.go` already reads
  `contract/testdata` from the engine's own tests.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened. A test whose premise
  the rulings change is rewritten to assert the new behaviour with its
  row; a fixture gains the `Visible` a player's window always holds; a
  click moved onto the grid asserts what it asserted.
- **P2. Rules 3, 4 and 5.** No contract change, no fold change; wall, door,
  floor and scenery are spatial words, not game-system ones.
- **P3. Rules 8 and 10.** Names, never line numbers. A comment under
  `client/src` or `internal/` is a warning or a pointer; a comment the
  change makes false is corrected in the commit that makes it false; M6
  binds what may be added, and no added comment line carries a banned term.
- **P4. Rule 9, answered here** (G3). MapTool refuses a player's path on the
  client, from the client's own copy of the map: through hard fog
  (`AbstractAStarWalker.fowBlocksMovement`, skipped for the GM's view) and
  through movement-blocking topology (`vblBlocksMovement`). Borrowed: the
  window refuses from what it holds, and the GM's view is exempt, as the
  DM's grid stands here; the server still judges whatever is sent (VTT-601,
  VTT-602). Not borrowed: MapTool's bound is the exposed area, everything
  ever seen, which is this platform's `Explored`; the ruling's is `Visible`.
  Nor the path: a path and a distance are the ticket's non-goals.
- **P5. The owner's rulings above.** The server's gate and texts as they
  are; the DM's and the agent's click inside the grid as it is.
- **P6. SPEC-020 and SPEC-023.** The client's stored mutation report goes out
  of date with the first client commit; `task check` names that and passes;
  a full run is the owner's to ask for.
- **P7. The mechanical gates.** A commit that changes `client/src` carries
  `cmd/vtt/webdist` rebuilt by `task build:client`; bun under
  `env -u CLAUDECODE`; `task client:typecheck`;
  `python3 tools/check-new-prose.py` and
  `python3 tools/check-comments.py main` before each commit; stage and
  commit in separate calls; a client test reads no file outside
  `contract/gen` and `contract/testdata`, and
  `tools/ts_mutation_inputs_test.py` runs before a commit that adds one;
  `Array.from` over a `NodeList`; commands started with `/bin/sh`, never
  zsh; `go clean -cache` before and after a gate. Each code task gets Phase
  4a's QA and one reviewer before its commit.

## Decisions

1. **D1. Off the grid is no square.** `cellFromPoint` returns
   `Cell | undefined`: `undefined` when the floored column is below zero or
   at or past `geom.width`, or the row below zero or at or past
   `geom.height`; no clamp.
   - **Forced by** item 1 and R1, and by the clamp's own reason ("a click in
     the margin must land on the board") being the defect. `undefined`, as
     `doorDirection` answers "none".
2. **D2. The board decides it, for every window.** `renderGrid`'s handler
   calls `onCell` only with a square. `onCell` in `app.ts`, `doorCommandFor`
   and `moveCommandFor` do not change.
   - **Forced by** item 1 holding for a player's, a DM's and an agent's
     window alike, armed or not, before any role's branch, and by item 6:
     the DM's branch inside the grid stays as it is.
3. **D3. A player's click reads `Visible` as the squares seen now, and no
   `Visible` as every square seen** (reading A; OD-1, the owner's answer).
   The check is `scene.Visible === undefined || scene.Visible[doorKey(x, y)]
   === true`, in `playerClickFor` alone, the reading the draw readers
   (`tokensOnScene`, `planFog`) already make (state.ts's `Scene.Visible`).
   - **Forced by** the owner's answer to OD-1. R2 is worded for a scene the
     window holds a `Visible` for.
   - **Its cost**, accepted with the answer: in M8's persistent case, a token
     off the grid, a click sends a move the server refuses with VTT-601's
     text. Class (e) is not touched: its fixtures hold no `Visible`, and
     under A their clicks send what they sent.
4. **D4. The order in `playerClickFor`:** no scene, or a square not seen
   now → nothing; a listed party member other than the watched one on the
   square → switch; the watched one's own token → nothing; a square
   `blocksMove` answers true for → nothing; else `moveCommandFor`.
   - **Forced by** item 2 ("neither a move nor a switch"), the ticket's
     third rule ("that would move the watched one's token"), so a listed
     member standing in a seen closed door still switches (R7), and VTT-408.
5. **D5. `blocksMove` lives in a new `client/src/view/terrain.ts`.**
   `blocksMove(scene, cell)` answers true for a square the window holds as
   a wall, as a door `scene.OpenDoors?.[key]` does not hold true, or as
   covered by an object with `BlocksMove` anchored at its `X`, `Y`,
   `Width` columns by `Height` rows, rotation not read: the in-grid arms of
   `engine.State.Blocked`. It has no bounds arm. Keys by `doorKey`.
   - **Forced by** one key format (`doorKey` in `client/src/fold.ts`); by a
     file named after `internal/engine/terrain.go`, so each points at the
     other; by D6's test importing it without the panel; and by M9 and D1:
     no window path hands it a square off the grid, so a bounds arm would
     be code for a case nobody reaches.
6. **D6. The client's copy of `Blocked` is held to one fixture by both
   languages. Worth it.** `contract/testdata/blocked_squares_log.json` holds
   a scene's creation and its door events; `blocked_squares.json` lists
   squares of its grid with the answer each must get. `internal/engine`'s
   `terrain_test.go` folds the log with `engine.Apply` and asks `Blocked`;
   `client/test/terrain.test.ts` folds it with `fold` and asks
   `blocksMove`. The squares: a wall; a floor; a square with no tile; a
   door never opened; a door opened; a door opened then closed; the anchor
   and the far corner of a `BlocksMove` object of more than one square on
   each axis; the floor square just past its right edge, its bottom edge,
   its left edge and its top edge; a square of an object that blocks sight
   and not movement. A citation line of R4's id stands above
   `engine.State.Blocked` and above `blocksMove`, each with the warning to
   change the other and the fixture with it.
   - **Forced by** the drift that costs most being silent: a client that
     blocks what the server allows takes away a player's only way onto that
     square, since a board click is a player's only move, and nothing else
     sees it, as the gateway's tests hold `Blocked` alone and the client's
     hold the copy alone. The fixture is one human-written statement both
     languages answer to, as `fold-parity.test.ts`'s goldens are (M11), so
     it adds no mechanism.
   - **What it does not catch:** a kind of blocker added to one side only.
     That is the citation line's, by reading (Phase 4b).
7. **D7. The window's `Visible` lagging the server's** (the ticket's first
   open point) gets no code, no test and no debt.
   - **Forced by** the owner's standing rule: the lag lasts until the
     frames of the move's batch arrive, and its consequence is one click
     that sends nothing on a newly seen square, or one that reaches the
     server and draws VTT-601's text on a square no longer seen. Told to
     the owner in one line, here.
8. **D8. The current scene is the board's.** The seen-now and blocked checks
   read `st.Scenes[currentSceneId(st)]`, the scene `playerClickFor` already
   filters tokens by and the board draws.
   - **Forced by** the click being on that scene's drawing. That the
     watched token may stand on another scene is `currentSceneId`'s, and
     not this work's.
9. **D9. Tests move; their kills stay.** Each test in "The existing tests
   that change" is rewritten, moved onto the grid or given a `Visible`, as
   its class says. A spy test asserting no `onCell` call asserts its length
   (M5). A board click in a window test is placed through `boardCamera` at
   a square's centre, as `qa-switch.test.ts`'s `clickCell` does, and a
   click on a disc carries the disc's centre, as `qa-console-perch.test.ts`'s
   `clickToken` does.
   - **Forced by** P1, and by M4: a click that falls off the grid leaves a
     green test that checks nothing, among them the one that kills `onCell`'s
     `if (cmd)` guard.
10. **D10. The register.** R1 to R7 under the sort at Task 0. VTT-884 and
    VTT-919 are withdrawn in Task 2, whose code makes them false, each
    succeeded as listed under Requirements; their tests cite the successors.
    VTT-885's evidence cell follows its renamed test.
11. **D11. The real wire is a new spec, `client/e2e/board-clicks.spec.ts`,
    on the shipped cellar.** `client/e2e/setup.ts` serves
    `campaigns/example/maps/cellar.json` to its table as to the doors
    table. The DM, as `doors.spec.ts` does: starts a session; adds
    `act-fighter` as a party member and `act-cutpurse` as not one, with the
    sheet that spec uses; grants the fighter to the player; loads the
    cellar; moves `tok-fighter` to the door's west neighbour; places the
    cutpurse on its east neighbour. The player's page records every frame
    it sends (`page.on("websocket")`, `framesent`). The DM opens the door;
    the player's window shows the cutpurse; the DM closes it; the window
    hides it. The player clicks the margin beside the grid on the fighter's
    row, then the square two east of the door, which the fighter saw while
    the door stood open and does not see now; each square's position comes
    from the fighter's disc, whose box is one square through the board's
    camera, and the spec asserts the margin is wider than a square before
    using it. The fighter's disc has not moved. Then the player clicks a
    seen floor square of the west room and waits for the disc to arrive
    there. The frames sent then hold exactly one `moveToken`, to that
    square, and the window shows no `.toast`.
    - **Forced by** item 5. A spec of its own because `setup.ts` gives each
      spec file its own table, and a test resting on another test's end
      state fails confusingly run alone; the cellar because it is the
      shipped map with walls and a door. The sent frames are the
      observation because a later successful command clears the toast, so
      a toast asserted absent after it proves nothing; no frame sent means
      no refusal can come. Today the margin click clamps onto the west
      wall and draws "You cannot move there: a wall.", so the spec is red.
    - `table.spec.ts` is unchanged (M7).
12. **D12. Comments follow the code** (P3, M6), in the commit that changes
    it: `cellFromPoint`'s doc loses the clamp's reason and ends within the
    block bound; `renderGrid`'s handler gains a citation line of R1;
    `playerClickFor`'s doc names what now sends nothing and its rows;
    `terrain.ts` carries the warning of D6; `terrain.go` gains R4's citation
    line and nothing in `Blocked`'s block. `state.ts`'s `Scene.Visible` doc
    stays true: its rule binds the readers that draw. `worldFromScreen`'s
    doc in `camera.ts` cites `cellFromPoint` by a line number; that file is
    not this work's.
13. **D13. Every client commit carries its bundle** (P7): Tasks 1 and 2.
14. **D14. The specification last,** from the report: SPEC-022's
    board-click paragraph (off the grid; seen now, D3's reading of no
    `Visible`; blocked, with `blocksMove`, the fixture and D4's order); its
    Status sentence on doors (its id range loses VTT-884); its card
    paragraph's "moves as before" (still true where the disc's square is
    seen and open); "Principles served" (20 beside 14); its
    "## Requirements" list (the new ids in, VTT-884 and VTT-919 out).

## Open decisions for the owner

- **OD-1. How a player's click reads a scene with no `Visible`** (D3).
  **Answered A by the owner, 2026-11-21.**
  - **B (recommended).** As no square seen: the ruling holds in every state,
    a token off the grid included, and the code has no branch for it.
    Class (e)'s fixtures gain a `Visible`, through each file's own table
    builder.
  - **A.** As every square seen, as the board draws it: no fixture changes
    beyond classes (a) to (d); in M8's persistent case a click draws
    VTT-601's text. R2 is then worded for a scene the window holds a
    `Visible` for.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill; the register's
words are the ones that bind. Every new row reads `**OPEN — no test yet**`
until its task cites its tests.

| R | Item | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | A board click outside the scene's grid sends no command. | a click past the left, right, top or bottom edge sends a command, in a player's window or a DM's with doors armed or not; the column at the grid's width, or the row at its height, read as inside |
| R2 | 2 | A player's board click on a square of the current scene that the window holds a `Scene.Visible` for and that it does not name sends no command. | a click on a remembered or a never-seen square sends `move_token` or `set_viewpoint`; a listed party member's token on an unseen square switches |
| R3 | 3 | A player's board click on a square of the current scene that the window holds as a wall, as a door it holds closed, or as covered by scenery that blocks movement sends no `move_token`. | a move onto a seen wall, a seen door never opened or closed after opening, or a seen square of a blocking footprint other than its anchor |
| R4 | D6 | The browser client's `blocksMove` and `engine.State.Blocked` each answer every square `contract/testdata/blocked_squares.json` lists as it says. | a door absent from `OpenDoors` read as open; a footprint one row or column too wide or too narrow; an object that blocks sight alone read as blocking; a wall read as open |
| R5 | 4 | A player's board click on a door square sends no `open_door` or `close_door`. | a door command from a player's click on an open or a closed door |
| R6 | 4 | A player's board click with no ability armed, on a square of the current scene the window's `Scene.Visible` names that holds no token of a party member the last `WatchableChanged` names and that the window holds as none of a wall, a closed door or a square covered by scenery that blocks movement, sends `move_token` of the watched one's token, when it has one, to that square. | such a click on a seen floor, a seen square with no tile, a seen open door or a seen square of an object that blocks sight alone sends nothing, or moves another token |
| R7 | 4 | A board click in a browser window whose participant's role the client last read as `player`, on a square of the current scene the window's `Scene.Visible` names that holds a token of a party member the last `WatchableChanged` names other than the one they watch, sends `set_viewpoint` naming one of those party members. | no switch on such a square, one the window holds as a closed door included |

**Candidates the sort may refuse:**
- **A row per edge of the grid.** Refused: one rule; the edges are its
  test's cases.
- **A row for what `cellFromPoint` returns.** Refused: how, not what;
  `grid.test.ts`'s test cites R1.
- **A row that a DM's and an agent's click inside the grid sends what it
  sends today** (item 6). Refused: a no-change claim, held by the existing
  tests; the armed click is VTT-888's and VTT-876's.
- **A row for D7's lag.** Refused: no code and no observation built for it.
- **A row that a seen square's tile, door and objects in a player's window
  are the server's** (M9). Refused: the projection's, held by SPEC-016's
  rows.

**Withdrawn in Task 2:**
- **VTT-884:** "False as worded once a player's click on a seen closed door
  sends nothing where the same click on a square with no door moves.
  Succeeded by R3, R5 and R6."
- **VTT-919:** "False as worded once a player's click on a square not seen
  now sends nothing, a listed party member's token on it notwithstanding.
  Succeeded by R7."

**Rows checked and left standing,** their evidence cells following any test
renamed: **VTT-408** (true as worded: a click that moves a token moves the
watched one's), **VTT-885** (its cell follows the renamed `app.test.ts` test),
**VTT-405**, **VTT-398**, **VTT-695**, **VTT-474**, **VTT-876**, **VTT-888**,
**VTT-601**, **VTT-602**, **VTT-922**.

## The existing tests that change

Measured in M3 and M4, by file and test name.

- **(a) They pin the clamp; rewritten to R1 (Task 1).**
  - `client/test/grid.test.ts`: "clicks outside the board are clamped, never
    negative or past the edge".
  - `client/test/spectator-view.test.ts`: "a click past the last cell is
    clamped onto the board".
- **(b) A seen closed door now sends nothing; rewritten to R3, R5 and R6
  (Task 2).**
  - `client/test/app.test.ts`: "a player beside a door is offered no
    arm-doors control, and a click on the door square moves the watched
    character's token" (VTT-885's half, the panel, stands); "with doors
    disarmed, the same click moves the token instead".
  - `client/test/qa-door-mode.test.ts`: "VTT-884 VTT-408: a player's click
    on a closed door square moves the watched token there, as the same
    click does where no door is".
- **(c) Their click lands beside the grid and meant a square; moved onto it
  (Task 1).**
  - `client/test/app.test.ts`: "clicking the board moves the player's
    token"; "a DM's chips pick from the fold and send nothing, and the board
    moves the pick"; "a DM's board click before any chip moves the actor
    granted to them".
  - `client/test/qa-one-log.test.ts`: "a player's command result carries a
    world sequence, which a redial's after does not take".
  - `client/test/qa-what-a-window-shows.test.ts`: "qaWS: a left click on a
    disc or its card reaches the board".
  - `client/test/token-card.test.ts`: "a click on a disc still reaches the
    board's click handler".
- **(d) Green after the change but checking nothing; moved onto the grid
  (Task 1).**
  - `client/test/app.test.ts`: "a null move command is never handed to
    send, even before the socket opens" (its click lands beside the grid,
    so `onCell` never runs and its `if (cmd)` guard goes unkilled).
  - `client/test/spectator-view.test.ts`: "a board click maps to the cell
    under the pointer, offset by the board's box" (M5); "the same pixel maps
    to a DIFFERENT cell when the board moves" (one of its two clicks lands
    beside the grid).
- **(e) Their fixture models a player's window with no `Visible`** (would
  have gained one under OD-1 B; untouched under the answer A).
  - Red: `client/test/app.test.ts`: "a player switches by a button and by a
    click on the token"; "a player's move moves the character they watch".
    `client/test/player.test.ts`: "a click on a listed party member's token
    switches to it"; "two listed party members on one cell switch to the
    lowest id"; "a click on a square with no listed party member moves the
    watched character"; "a token the set does not name moves the watched
    character onto its square"; "a listed party member on the same row or
    column, not the same square, does not switch"; "a listed party member's
    token in another scene does not switch".
    `client/test/qa-console-perch.test.ts`: "a player's board click >
    VTT-919: on a listed party member's token switches to it" (flag false
    and flag true); "VTT-919: on an unlisted party member's token does not
    switch to it". `client/test/qa-door-mode.test.ts`: "VTT-408: a player's
    click on a plain square moves the watched character's token there (the
    baseline the door cases compare with)"; "VTT-884 VTT-408: a player's
    click on an open door square moves the watched token there, as the same
    click does where no door is"; "VTT-884 VTT-919: a player's click on an
    open door square holding a listed party member's token switches to them,
    as the same click does where no door is".
    `client/test/qa-perched-player.test.ts`: "qaNP: a player whose set says
    they control no party member switches or moves by a board click as a
    listed player does"; "qaNP: a listed player's click on a cell of set
    members switches to one of them, never to one the set does not name";
    "qaNP: a listed player's click on an empty cell moves the watched
    character's token". `client/test/qa-switch.test.ts`: "player: a click on
    a listed party member's token sends set_viewpoint naming it"; "player: a
    click on an empty square moves the watched character, which is not the
    first by id"; "player: the watched character and a listed party member on
    one square switch to the other"; "player: two listed party members on one
    square send one set_viewpoint naming one of them"; "player: after a
    switch, a move moves the newly watched character"; "player: a click on a
    listed party member's token standing in an open door switches to them,
    and works no door"; "player: with an ability armed and doors not, a
    click on a listed party member's token in range still switches".
    `client/test/qa-window-offers.test.ts`: "the panel offers what the
    selected actor's sheet names > a player's board click with a carried-over
    ability the sheet does not name moves the watched token".
  Under OD-1's answer A none of class (e) changes; the list stands as the
  measurement of what reading B would have cost.
  - Green but answered by the seen-now check first, under reading B only:
    `player.test.ts`: "a click on the watched character's token sends
    nothing".
    `qa-switch.test.ts`: "player: a click on the watched character's own
    token alone sends no set_viewpoint, and moves nothing else"; "player: a
    token of an actor the set does not name switches to nothing, and any
    move is the watched character's". `qa-console-perch.test.ts`: "VTT-919:
    on the watched one's own token sends no switch".
    `qa-door-window.test.ts`: "a player watching the character beside the
    door works it by its button; switching to the far one removes the
    button" (its board click is on a closed door, which R3 now answers too).
  - Tests naming VTT-884 or VTT-919 cite their successors.

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 to R7 and the
  candidates, as OD-1's answer leaves them; allocate ids with
  `requirement-id`; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: a click off the grid is no square (R1; D1, D2, D9; item 1)

- **Files:** `client/src/view/grid.ts` (`cellFromPoint`, its doc);
  `client/src/view/spectator.ts` (`renderGrid`'s handler);
  `client/test/grid.test.ts`, `client/test/spectator-view.test.ts`,
  `client/test/app.test.ts`, `client/test/qa-one-log.test.ts`,
  `client/test/qa-what-a-window-shows.test.ts`,
  `client/test/token-card.test.ts` (classes a, c, d);
  `docs/requirements.md` (R1's cell); a rebuilt `cmd/vtt/webdist`.
- **Tests first, seen red:**
  - `cellFromPoint`: a point past each of the four edges is no square, the
    column at exactly the grid's width and the row at exactly its height
    included, and a point a fraction left of or above the origin; the last
    pixel inside the far edge is the last square.
  - The board: a click past each edge calls no `onCell`, asserted by length.
  - The window, through `boot`: a player's click past each edge, and a DM's
    with doors armed and unarmed, sends no frame.
- **True when:** the new tests pass and failed before; classes (a), (c) and
  (d) pass, moved as D9 says; the whole client suite,
  `task client:typecheck` and P7's checks pass.
- **Deliberate breaks:**
  - Restore the clamp: the off-edge tests turn red.
  - `x >= geom.width` → `x > geom.width`, and the same on the row: the
    exact-edge cases turn red.
  - Call `onCell` whatever `cellFromPoint` answers: the board's length
    assertion and the DM's window test turn red.
  - `ev.clientX - r.left` → `+ r.left`: the moved "offset by the board's
    box" test turns red.
  - Drop `onCell`'s `if (cmd)` in `app.ts`: the moved "a null move command
    is never handed to send" turns red.
- **The state here:** no window sends anything for a click beside the map;
  inside it, every window sends what it sent before.

### Task 2: what a player's click sends (R2 to R7; D3 to D6, D8, D10; items 2 to 4)

- **Files:** `client/src/view/terrain.ts` (new, `blocksMove`);
  `client/src/view/player.ts` (`playerClickFor`, its doc);
  `contract/testdata/blocked_squares_log.json` and
  `contract/testdata/blocked_squares.json` (new);
  `internal/engine/terrain_test.go`, and R4's citation line in
  `internal/engine/terrain.go`; `client/test/terrain.test.ts` (new);
  `client/test/player.test.ts`, `client/test/app.test.ts`,
  `client/test/qa-door-mode.test.ts`;
  `docs/requirements.md` (the withdrawals, the cells); a rebuilt
  `cmd/vtt/webdist`.
- **Tests first, seen red:**
  - `playerClickFor`, on a scene holding a `Visible`: a remembered square
    and a never-seen square send nothing; a listed party member's token on
    an unseen square sends nothing; on a scene with no `Visible` a click
    moves or switches as on a seen square (R2, D3). A seen wall, a seen door never opened, a seen door opened and
    closed again, and a seen square of a two-by-two `BlocksMove` object
    other than its anchor send nothing (R3). A seen open door, a seen square
    with no tile and a seen square of an object that blocks sight alone move
    the watched one's token (R6, R5). A listed party member standing in a
    seen closed door switches (R7, D4).
  - The window, through `boot` with a `sceneSeen` in the stream: a click on
    a seen closed door sends nothing; the same square after a `doorOpened`
    moves the watched one's token and sends no door command (class b).
  - The fixture: `terrain_test.go` and `terrain.test.ts` each answer every
    listed square as it says, and each asserts the list is not empty (R4).
- **True when:** the new tests pass and the client's failed before (the
  Go test passes on today's `Blocked`, which is the point of it); class
  (b) passes as rewritten and class (e) unchanged; the whole client suite, `go test
  ./internal/engine/...`, `task client:typecheck` and P7's checks pass;
  `task check:requirements-chain` passes.
- **Deliberate breaks:**
  - Drop the seen-now check: R2's tests turn red.
  - Read no `Visible` as no square seen: the no-`Visible` test and class (e)
    turn red.
  - Ask seen-now after the switch: the unseen listed member's test turns red.
  - Ask `blocksMove` before the switch: the closed-door switch turns red.
  - Drop the wall arm, the door arm or the object arm of `blocksMove`: R3's
    case and the client's fixture test turn red.
  - Read `OpenDoors?.[key] === false` as closed: the never-opened door
    reads open and turns red.
  - `cell.x < o.X + o.Width` → `<=`, and each of the footprint's other
    bounds: the fixture's edge squares turn red.
  - In `engine.State.Blocked`, read a door absent from `OpenDoors` as open:
    the Go fixture test turns red.
- **The state here:** a player moves only onto a seen square they can
  enter, and switches only by a seen token; the DM's grid as before.

### Task 3: the real wire (D11; item 5)

- **Files:** `client/e2e/board-clicks.spec.ts` (new); `client/e2e/setup.ts`
  (the cellar served to its table, and the comment naming the doors spec
  alone); `docs/requirements.md` (R1's and R2's cells gain the spec).
- **True when:** `task e2e` passes, the new spec and `table.spec.ts` among
  it.
- **Deliberate breaks:**
  - Restore the clamp: the margin click sends a `moveToken` and draws "You
    cannot move there: a wall."; the spec turns red.
  - Drop the seen-now check: the remembered square's click sends a
    `moveToken`; the spec turns red.
- **The state here:** the whole table, through a browser and a server.

### Task 4: the gate (item 7)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger` once,
  last, if a ceiling must come down (M6); `go clean -cache`; check free
  space and the load (`os.getloadavg()`); run `task check` whole on the
  last code commit, detached under `/bin/sh`, output to a file, requiring
  its completion line; run `task e2e`; `go clean -cache`. Tell the owner
  the client's stored mutation report is out of date from Task 1 and which
  commit it measured (SPEC-023); a full run is his to ask for.
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the last client commit's.

### Task 5: the report, then the specification

- **The report:** `docs/reports/2026-11-21-a-click-moves-only-onto-a-seen-open-square.md`,
  with the `implementation-report` skill and its reading review: each "Done
  looks like" item with its observation; the rows and what turned each red;
  Rule 9 as P4 answers it; G1 to G6 and OD-1's answer; D7 told to the
  owner; Phase 4a's adjudications under their own heading; the gate.
- **SPEC-022,** with the `specification` skill, from the report, as D14
  lists.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the specification;
  push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, SPEC-022 as drafted, and the surfaces as
signatures, never the source: `cellFromPoint`, `playerClickFor`,
`blocksMove`, `moveCommandFor`, `currentSceneId`, `doorKey`, `boardCamera`,
`renderSpectator` with `ViewExtras.onCell`, and `boot` with its fetch and
socket seams as `app.test.ts` uses them. Tell it: never run `task check`,
`task e2e` or Stryker; its files are new `qa-*` test files; client tests
under `env -u CLAUDECODE bun test ./test`; a client test reads no file
outside `contract/gen` and `contract/testdata`; `Array.from` over a
`NodeList`; assert "no call" by length, never `toEqual([])` (M5); fixtures
with a `Visible` that names some squares and not others, a remembered square
among the others, scenery more than one square wide and tall, and doors in
each state; the tree is hashed before and after it. Known and not findings:
D7's lag; a player's board on the scene whose id sorts greatest while the
watched token stands on another (D8); a token placed off the grid by a DM
or an agent (M8); MapTool's exposed-area bound, not borrowed (P4); a DM's
and an agent's window reading no `Visible`.

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check each
comment under `client/src` and `internal/` is a warning or a pointer, within
M6's ceilings and block bound, and that `cellFromPoint`'s doc no longer
gives the clamp's reason; check every assertion this work changed for being
weakened rather than moved (rule 2), nothing losing an assertion; read
`playerClickFor`'s order
against D4; read `blocksMove` against `engine.State.Blocked` arm by arm, and
the fixture's squares against each arm and each footprint bound; and check
that no spy test asserts "no call" with `toEqual`.
