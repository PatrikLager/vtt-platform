# A board click moves only onto a seen, open square — report

Ticket: `docs/superpowers/specs/2026-11-21-a-click-moves-only-onto-a-seen-open-square-design.md`.
Plan: `docs/superpowers/plans/2026-11-21-a-click-moves-only-onto-a-seen-open-square.md`.
Branch `fix/a-click-moves-only-onto-a-seen-open-square` from main `b90fc1a`.
Commits: `79230f2` (Task 0: ticket, plan, register), `31e4afd` (Task 1, off
the grid), `ebed843` (Task 2, what a player's click sends), `40fe408` (Task
3, the real wire; the last code commit), `ff16cea` (the comment ledger) and
`7b5b96e` (the coverage floor of the new file). Every code reference below
is to `40fe408`.

## Done looks like

1. [x] **A click off the grid does nothing.** `cellFromPoint`
   (`client/src/view/grid.ts`) answers no square for a point whose floored
   column is below zero or at or past the grid's width, or whose row is
   below zero or at or past its height, where it clamped onto the nearest
   edge square; `renderGrid`'s click handler
   (`client/src/view/spectator.ts`) calls `onCell` only with a square, so
   the answer is made before any role's branch of `onCell`
   (`client/src/app.ts`).
   - `grid.test.ts`'s "a point off the grid is no square, on each side and
     at the far edges"; `spectator-view.test.ts`'s "a click beside the grid
     calls no onCell, on each of its four sides", which asserts the calls'
     length; `app.test.ts`'s "a player's click beside the grid sends
     nothing, on each of its four sides", "a DM's click beside the grid
     sends nothing, doors unarmed" and "a DM's click beside the grid sends
     nothing, doors armed".
   - Independently: `client/test/qa-click-off-grid.test.ts`, every role on
     fixtures drawn at a scale other than one and offset on one axis.
   - On `b90fc1a`, `grid.test.ts`'s "clicks outside the board are clamped,
     never negative or past the edge" and `spectator-view.test.ts`'s "a
     click past the last cell is clamped onto the board" pinned the clamp;
     Task 1's break restoring it turned the new tests red.
2. [x] **A player's click on a square they do not see does nothing.**
   `playerClickFor` (`client/src/view/player.ts`) answers nothing for a
   square of the current scene that the window holds a `Visible` for and
   that it does not name, before it looks for a token to switch to.
   - `player.test.ts`'s "a click on a remembered square or a never-seen one
     sends nothing" and "a listed party member's token on a square not seen
     now switches to nothing".
   - Independently: `qa-seen-open-square.test.ts`'s "a player's click on
     each square of a scene with a Visible answers by what is seen now and
     held there" and "a player's window sends nothing for an unseen or a
     blocked seen square and one move for an open seen one".
   - On `b90fc1a` a click on a remembered or a never-seen square sent
     `move_token` (the plan's M2), and `playerClickFor` switched to a
     listed party member's token wherever it stood, seen or not.
3. [x] **A player's click on a seen square the character cannot enter does
   nothing.** `blocksMove` (`client/src/view/terrain.ts`, new) answers true
   for a square the window holds as a wall, as a door its `OpenDoors` does
   not hold open, or as covered by an object whose `BlocksMove` is set;
   `playerClickFor` sends no move there; a listed party member standing
   on such a square is still switched to (item 4).
   - `player.test.ts`'s "a click on a seen wall, a seen closed door or seen
     scenery that blocks movement sends nothing"; `app.test.ts`'s "a player
     beside a seen closed door is offered no arm-doors control, and a click
     on the door square sends nothing", which delivers a `sceneSeen`
     through the window's socket; `qa-door-mode.test.ts`'s "VTT-936
     VTT-938: a player's click on a closed door square sends nothing, where
     the same click on a square with no door moves".
   - On `b90fc1a` the same app test asserted the move ("... and a click on
     the door square moves the watched character's token").
4. [x] **A seen square the character can enter still moves.**
   - `player.test.ts`'s "a click on a seen open door, a seen square with no
     tile or a seen curtain moves the watched token there", which clicks a
     seen floor square first, the curtain being an object that blocks
     sight alone; `app.test.ts`'s "once the seen door is open, the same
     click moves the watched character's token and works no door",
     rewritten from that file's "with doors disarmed, the same click moves
     the token instead", for the open door in a window.
   - The switch: the row is VTT-940 now, VTT-919 withdrawn (below).
     `player.test.ts`'s "a click on a listed party member's token switches
     to it" and "a listed party member standing in a seen closed door still
     switches to them"; `qa-switch.test.ts`'s player tests, green unchanged
     but for their citations.
   - The existing move tests (VTT-408) are green unchanged: their fixtures
     hold no `Visible`, which a player's click reads as every square seen
     (OD-1, below).
5. [x] **No warning appears.** Against the real server, through
   `task e2e`:
   - `client/e2e/board-margin.spec.ts`'s "a player's click beside the
     ravine's grid sends nothing, and one on a seen square of it moves", on
     `adventures/goblin-ambush`'s ravine, whose edge is open ground: after
     the margin click no `.toast` is drawn, and the frames the page sent
     are exactly one `moveToken`, the later one onto a seen square.
   - `client/e2e/board-clicks.spec.ts`'s "a player's click on a square they
     no longer see sends nothing, and one on a seen floor moves", on the
     shipped cellar: the DM opens and closes the door, the player clicks
     the square beyond it, seen while the door stood open, and no `.toast`
     is drawn; the frames sent are exactly one `moveToken`, to a seen floor
     of the fighter's own room.
   - The frames sent, asserted whole, hold the item: a click that sends
     no frame draws no refusal. Each `.toast` check runs before the move,
     whose answer would clear a toast an earlier click drew.
6. [x] **The DM's grid stands.** `onCell`'s DM branch, `doorCommandFor` and
   `moveCommandFor` are unchanged.
   - `app.test.ts`'s "a DM's chips pick from the fold and send nothing, and
     the board moves the pick" and "a DM's board click before any chip
     moves the actor granted to them", whose click had landed beside the
     grid and now lands on a square of it (Task 1); "with doors armed, a
     DM's click on a door square opens it and sends no move", "with doors
     armed, a DM's click on a plain square sends nothing at all, though
     unarmed it moves" and "the DM's armed door click names no actor,
     though a chip selects an actor of theirs", unchanged.
   - Independently: `qa-seen-open-square.test.ts`'s "a DM's window moves
     its selected actor onto a wall, scenery and a closed door, doors
     unarmed"; `qa-click-off-grid.test.ts`'s DM and agent windows, inside
     each edge, armed and unarmed.
7. [x] **The gate.**
   - `task check` is green on `7b5b96e` from a clean tree, the Go mutation
     gate among it. Its first run, on `ff16cea`, was red at
     `check:ts-coverage`, which holds a new file under `client/src` to a
     floor it did not yet have; `7b5b96e` gives `terrain.ts` its floor at
     the coverage it earns, the whole file.
   - The embedded bundle under `cmd/vtt/webdist` is the build of the last
     commit to change `client/src`, `ebed843`; `check:drift`, a step of
     `task check`, rebuilds it and fails on any difference.
   - `task e2e` is green on `7b5b96e`, both new specs and `table.spec.ts`'s
     "a player sees their own panel and can move their token" among it.
   - The client's stored mutation report measured `b90fc1a`; this ticket
     changed the client after it, so `task check` judges none of it and
     names the ticket (SPEC-023). A full run is the owner's to ask for.

## What the rules became

- A board click outside the grid sends no command, in any window:
  VTT-934.
- A player's click on a square not seen now sends no command: VTT-935,
  worded for a scene the window holds a `Visible` for, as OD-1's answer
  leaves it.
- "A player's board click that would move the watched one's token onto a
  seen square the window holds as a wall, a closed door, or covered by
  scenery that blocks movement sends no command": VTT-936, worded as no
  `move_token`, since a listed party member on such a square is still
  switched to (D4, VTT-940).
- Added by the plan:
  - the client's `blocksMove` and `engine.State.Blocked` each answer the
    shared fixture as it says: VTT-937 (D6);
  - a player's click on a door square sends no door command: VTT-938;
  - the move a player's click sends, and where: VTT-939;
  - the switch, now on a square seen: VTT-940.
- Withdrawn at Task 2: VTT-884, a seen closed door now sending nothing
  where a square with no door moves (succeeded by VTT-936, VTT-938 and
  VTT-939); VTT-919, a square not seen now sending no switch (succeeded by
  VTT-940).

Refused at Task 0: a row per edge of the grid (one rule; the edges are its
test's cases); a row for what `cellFromPoint` returns (how, not what); a row
that the DM's and the agent's click inside the grid sends what it sent (a
no-change claim, held by the existing tests, the armed click being
VTT-888's and VTT-876's); a row for D7's lag (nothing built for it); a row
that a seen square's tile, door and objects in a player's window are the
server's (the projection's, held by SPEC-016's rows). "No warning" got no
row: it follows from no command being sent, and the window and e2e tests
above hold it. VTT-408, VTT-885, VTT-601 and VTT-602 stand as worded,
VTT-885's evidence cell following its renamed app test.

## The deliberate breaks

Each was made by an edit, run, and restored; each turned its tests red.

- **Task 1:** the clamp restored; the far edge read as inside; `onCell`
  called whatever `cellFromPoint` answers (the board's test and the DM's
  unarmed window, the QA board test among them); the board's box offset
  added instead of taken away; `onCell`'s `if (cmd)` dropped (the
  null-command test, which had clicked beside the grid and so never reached
  the guard).
- **Task 2:** the seen-now check dropped; no `Visible` read as no square
  seen; the seen-now check asked after the switch; `blocksMove` asked
  before the switch; the no-scene guard dropped; the wall, door and object
  arms each dropped; an object that blocks sight read as blocking; an
  absent `OpenDoors` key read as open; each bound of the footprint off by
  one; an open door returning early before the objects; rotation read, in
  the client and in Go; in `engine.State.Blocked`, an absent door key read
  as open, which turned the Go fixture test red.
- **Task 3** (against the rebuilt binary, the bundle restored after and
  compared with the committed one): the clamp restored turned
  `board-margin.spec.ts` red at its sent frames and left
  `board-clicks.spec.ts` green; the seen-now check dropped turned
  `board-clicks.spec.ts` red at its toast check, the server's refusal
  drawn, and left `board-margin.spec.ts` green.

## Rule 9: how MapTool does this

MapTool's map has no edge to click off. It does refuse a player's movement
on the client, from the client's copy of the map: its path walker
(`AbstractAStarWalker`) refuses a step out of the exposed fog for a view
that is not the GM's (`fowBlocksMovement`), and one across
movement-blocking topology (`vblBlocksMovement`). Borrowed: the window
refuses from what it holds, and the GM's view is exempt, as the DM's grid
stands here; the server still judges whatever is sent (VTT-601, VTT-602).
Not borrowed: MapTool's bound is the exposed area, every square ever seen,
which is this platform's `Explored`; the owner's is `Visible`. Nor the
path: a path and a distance are the ticket's non-goals. The ticket's first
answer missed the walker; the verification found it (G3), and the ticket
was corrected before the plan was signed.

## The plan's gaps and the owner's answers

- **OD-1, a scene with no `Visible`:** the owner answered A, every square
  seen, as the board draws it (D3). The plan recommended B. The cost,
  accepted with the answer: a player's window holds a scene with no
  `Visible` until its first `sceneSeen`, and for good while the watched
  token stands off the grid, which only a DM's or an agent's command can
  place (M8); a click then sends a move the server refuses with VTT-601's
  text. The `Visible`-free fixtures of the existing player tests stayed as
  they were.
- **G1, files the ticket did not name:** changed as the plan listed but
  `client/test/qa-window-offers.test.ts`, which OD-1's answer left as it
  was; `client/e2e/board-margin.spec.ts`, `client/src/view/camera.ts`,
  `tools/comment-ceilings.txt` and `tools/ts-coverage-thresholds.txt`
  besides (Deviations).
- **G2, VTT-919 and VTT-884:** withdrawn and succeeded (above).
- **G3, Rule 9:** above.
- **G4, "a panned or zoomed map":** the client has no pan and no zoom;
  `fitCamera` centres the scene, so a margin lies on the axis with slack
  and only the board's border off the grid on the other. The tests click
  past each edge.
- **G5, the ticket's open points:** the lag is D7, below; a player's scene
  with no `Visible` is OD-1; `table.spec.ts`'s move stays green (M7, and
  `task e2e`); the drift between the client's reading of a blocked square
  and the server's is D6, the shared fixture.
- **G6, an agent's window:** it takes the DM's branch of `onCell`, and the
  off-grid answer is made before any branch; `qa-click-off-grid.test.ts`
  drives an agent's window too.
- **D7, the window's `Visible` lagging the server's:** no code, no test and
  no debt, by the owner's standing rule. Until the frames of an accepted
  move's batch arrive, a click on a newly seen square sends nothing, and
  one on a square no longer seen reaches the server and draws VTT-601's
  text.

## The existing tests that changed

- **They pinned the clamp:** `grid.test.ts`'s and `spectator-view.test.ts`'s
  clamp tests now pin VTT-934 (item 1).
- **Their click landed beside the grid and meant a square:** `app.test.ts`'s
  "clicking the board moves the player's token", the DM's chip and
  before-any-chip moves, and "a null move command is never handed to send,
  even before the socket opens"; `qa-one-log.test.ts`'s command result;
  `qa-what-a-window-shows.test.ts`'s and `token-card.test.ts`'s disc
  clicks, now at the disc's centre; `spectator-view.test.ts`'s pair on the
  board's box offset. Each window test now clicks a square's centre
  through `boardCamera`, each disc click the disc's centre, and the
  box-offset pair a pixel inside the grid; each asserts what it asserted,
  some the square the click lands on besides.
- **A seen closed door now sends nothing:** `app.test.ts`'s door clicks,
  "a player beside a door is offered no arm-doors control, and a click on
  the door square moves the watched character's token" and "with doors
  disarmed, the same click moves the token instead", became "a player beside a seen closed door is offered no
  arm-doors control, and a click on the door square sends nothing" and
  "once the seen door is open, the same click moves the watched
  character's token and works no door"; `qa-door-mode.test.ts`'s loop over
  a closed and an open door became a test per door state, the closed one
  asserting nothing sent, and its header names the door-has-a-button ticket's Done
  item 6 by its file instead of "the ticket's".
- **Citations:** `git grep -e VTT-884 -e VTT-919 -- client internal cmd`
  finds nothing at `40fe408`. On `b90fc1a` it finds `onCell`'s comment in
  `client/src/app.ts`, `playerClickFor`'s doc and tests in `app.test.ts`,
  `player.test.ts`, `qa-console-perch.test.ts`, `qa-door-mode.test.ts`,
  `qa-door-window.test.ts`, `qa-perched-player.test.ts` and
  `qa-switch.test.ts`; each now names the successors.
- **Spy assertions of no call** that this work touched assert a length, as
  Bun's `toEqual` holds `[]` equal to `[undefined]` (M5).
- `camera.test.ts`: a comment citing `grid.ts` by a line number now cites
  no line.

## Phase 4a adjudications

Task 1 (`client/test/qa-click-off-grid.test.ts`), no failure:

- QA finding 1: spec gap — a DM's or an agent's unarmed click inside the
  grid, and the chip selection before any chip click — SPEC-022's existing
  prose, not this ticket's — nothing changed.
- QA finding 2: spec gap — how `clientX` becomes a board point — pinned by
  `spectator-view.test.ts`'s "a board click maps to the cell under the
  pointer, offset by the board's box", moved onto the grid in Task 1 —
  nothing changed.
- QA finding 3: note — on the axis without slack, only the board's border
  lies off the grid — nothing changed.
- QA finding 4: spec gap — a click within float rounding of the far edge —
  almost theoretical with a small consequence: no code, no test, no debt.
- QA finding 5: spec gap — "no warning" has no row — refused in the sort,
  held by Task 3's e2e — nothing changed.
- Brief notes (the page's "null" origin, the standard pack's fetch,
  `boardCamera` in squares): no finding.

Task 2 (`client/test/qa-seen-open-square.test.ts`), no failure:

- QA finding 1: spec ambiguity — SPEC-022's board-click paragraph (a switch
  with no sight condition, "any other click moves", a door square sending
  what a square with no door sends) disagrees with VTT-935, VTT-936 and
  VTT-940 — SPEC-022 is rewritten after this report.
- QA finding 2: spec gap — no specification says what blocks a move in the
  window — SPEC-022 is to say it, with `blocksMove` and the fixture.
- QA finding 3: spec gap — VTT-939's "when it has one": a watched one with
  no token sends nothing (`moveCommandFor`) — nothing changed.
- QA finding 4: spec gap — the shared fixture had no one-square object, no
  rotation and no scenery over a square with no tile — the fixture gained
  a rotated statue whose second column lies over a square with no tile;
  Go and the client both answer it.
- QA finding 5: "no warning" has no row — as Task 1's finding 5; the
  window test's no-`.toast` check and Task 3's e2e hold it.
- A `Visible` naming no square sends nothing for every click (the fold
  keeps it as an empty set): consistent with VTT-935 — nothing changed.

Task 3 added tests only, and Phase 4a did not run (Deviations).

## Phase 4b

- **Task 0:** the ticket was verified by an agent that did not write it
  (passes with gaps G1 to G6); its MapTool answer, its "panned or zoomed"
  and its sentence that VTT-919 stands were corrected by its writer before
  the plan was signed.
- **Task 1:** the QA file cited VTT-934 for clicks inside the grid and
  moved a player onto doors its window held closed (now opened first,
  citing VTT-408); a spy could not count `onCell(undefined)`; the QA file
  quoted a scratch file; `cellFromPoint`'s doc said board pixels for world
  pixels; `worldFromScreen`'s doc in `camera.ts` and a comment in
  `camera.test.ts` cited `grid.ts` by a line number. All fixed, and
  `worldFromScreen`'s doc cut to the block bound. The comment ledger was
  left to the gate, as the plan says.
- **Task 2:** no Must. The fixture was blind to an open door under
  blocking scenery and to rotation being read: both added, and both drifts
  seen red in Go and in the client. The Go side carried no warning: one
  added in `Blocked`'s body. `playerClickFor`'s doc misread a scene with no
  `Visible` and miscited the own-token case; evidence cells in the
  register named tests that do not hold their row. All fixed, with the nits, but the older
  `toEqual([])` assertions the reviewer found harmless.
- **Task 3:** no Must. The clamp restored had left `board-clicks.spec.ts`
  green, the cellar's edge being wall, so the margin case had moved to
  `board-margin.spec.ts`; the review found `board-clicks.spec.ts` still
  making its margin click and citing VTT-934 for it: removed. The toast was checked after the move whose answer
  clears it: now before. The sent frames are asserted whole, not filtered
  by kind.
- **The ledger and the floor:** read for no ceiling raised and no floor
  touched but the new file's.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| One e2e spec on the cellar, the margin click beside the fighter's row (D11) | The margin click has `board-margin.spec.ts`, on the goblin-ambush ravine; `board-clicks.spec.ts` keeps the remembered square and the seen floor | The cellar's whole edge is wall, and a seen wall already sends nothing (VTT-936), so with the clamp restored the cellar's margin click still sent nothing and the spec stayed green |
| D6's list of fixture squares | It also holds the last column of the anchor's row and the last row of the anchor's column of the blocking object, a rotated object wider than one square, a square with no tile under its second column, the square below its anchor, and an opened door under blocking scenery | Task 2's QA found no rotation and no scenery over a square with no tile; its review found the fixture blind to an open door under scenery and to rotation being read |
| R4's citation line above `engine.State.Blocked`, nothing in its block (D6, D12) | The warning stands as the first lines of `Blocked`'s body, and the door arm's comment was shortened | Task 2's review found the Go side carrying no warning; `Blocked`'s doc is a block over `check:comments`' bound, so the warning went into the body, and the warning alone would have put `terrain.go` over its ceiling |
| `worldFromScreen`'s doc left alone, "that file is not this work's" (D12) | Its pointer "`cellFromPoint` (grid.ts:52)" lost the line number, and the block is cut to the bound | Task 1's review found the line-number citation; `check:comments` refuses a change to a block over its bound, so touching the block meant cutting it under the bound |
| Task 4 runs `task check` on the last code commit, the ledger written last | The gate is green on `7b5b96e`, a commit after the ledger that gives `terrain.ts` its coverage floor; the code is `40fe408`'s | `check:ts-coverage` refuses a new file under `client/src` with no floor, and the plan listed none |
| Phase 4a's QA for each code task (P7) | Task 3's e2e got none | The task added test files alone, no code a QA agent could derive tests against; its deliberate breaks were made instead, and its reading review ran |

## What could not be established

- **`qa-click-off-grid.test.ts`'s tests** are named with template literals
  (`${f.label}`), which the requirements chain cannot cite, so VTT-934's
  row does not name them; they run in the suite all the same.
- **The client's stored mutation report** measured `b90fc1a`; this ticket
  changed `client/src` after it, so no mutation run has judged
  `cellFromPoint`'s new bounds, `playerClickFor`'s order or `blocksMove`.
  A full run is the owner's to ask for (SPEC-023).
- **A blocker kind added on one side only** — a new arm in
  `engine.State.Blocked` with no fixture square — fails no test; the
  warnings in `Blocked` and above `blocksMove` hold it, by reading (D6).

## What was deliberately left out

- A path, a distance, or a token standing in the way (the ticket's
  non-goals).
- The server's move gate and its refusal texts: a move that reaches the
  server is judged as before.
- A DM's or an agent's click inside the grid, the door buttons, the
  spectator's board and how the board is drawn.
- D7's lag, and a token placed off the grid in a player's window (OD-1's
  accepted cost).
- A click within float rounding of the far edge (Task 1's QA finding 4).
