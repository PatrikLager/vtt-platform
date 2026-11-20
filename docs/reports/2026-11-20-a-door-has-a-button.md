# A player works a door by a button for that door — report

Ticket: `docs/superpowers/specs/2026-11-20-a-door-has-a-button-design.md`.
Plan: `docs/superpowers/plans/2026-11-20-a-door-has-a-button.md`.
Branch `feat/a-door-has-a-button` from main `eca70de`.
Commits: `85fac7e` (Task 0: ticket, plan, register), `6193fe3` (Task 1, the
buttons), `874ed29` (Task 2, the mode leaves), `f4449c6` (Task 3, the real
wire; the last code commit) and `1874d89` (the comment ledger). Every code
reference below is to `f4449c6`.

## Done looks like

1. [x] **A button per door.** `doorButtonsFor` (`client/src/view/doors.ts`)
   walks the current scene's door tiles and keeps those for which
   `doorDirection` answers, which it does for a player alone, from the first
   token of the asked-for actor on the current scene whose actor the player
   controls, standing at most one square from the door on each axis;
   `renderPlayerPanel` (`client/src/view/player.ts`) draws one `work-door`
   chip per button for `actingActor`, in a `.row.doors`, and no row when
   there is none.
   - `client/test/doors.test.ts`'s "a player's door buttons name each door's
     direction from the watched token, in the board's reading order", "no
     button for a door two squares off, a wall, a door of another scene, or a
     door beside another's token", "a door beside another of the player's
     characters gets no button for the one they watch" and "no scene, no door
     button"; `client/test/player.test.ts`'s "a player beside doors is offered
     a work-door button per door, below the hint and above the Say form" and
     "a player beside no door is offered no door row".
   - Independently: `client/test/qa-door-buttons.test.ts`.
   - On `eca70de` there is no `doorButtonsFor` and no `work-door` control.
2. [x] **The button says which door, and what it does.** The label is "Open
   door" or "Close door", as the window's `OpenDoors` holds the door, then
   the direction in parentheses from a table of the owner's words indexed by
   the step from the token to the door, north being a smaller y, which the
   board draws higher (`client/src/view/scene-plan.ts`), and "here" for the
   token's own square.
   - `doors.test.ts`'s every-neighbour test above (a token standing in an
     open door with a door on each neighbour, the tiles listed out of
     order) and "a scene with no OpenDoors reads its door closed";
     `qa-door-buttons.test.ts`'s "VTT-882: doorDirection names each of the
     nine squares by the eight directions, north up, and here".
3. [x] **The button works its door as the watched character.** A button's
   command is built per click: `open_door` for a closed door, `close_door`
   for an open one, at the door's square in the current scene, naming the
   actor the panel asked for.
   - `doors.test.ts`'s "a closed door's button opens it and an open one's
     closes it, at its square, naming the watched character";
     `player.test.ts`'s "a door button sends its door's command, once" and
     "each click on a door button sends a command of its own request id";
     `client/test/app.test.ts`'s "a player's door button names the character
     they watch, and a switch shows the new one's buttons".
4. [x] **A player's board click never works a door.** `onCell`
   (`client/src/app.ts`) sends a player's click to `playerClickFor` before
   anything reads `ui.doorsArmed`; the panel draws no arm-doors toggle.
   - `app.test.ts`'s "a player beside a door is offered no arm-doors
     control, and a click on the door square moves the watched character's
     token" and "with doors disarmed, the same click moves the token
     instead"; `client/test/qa-switch.test.ts`'s "player: a click on a listed
     party member's token standing in an open door switches to them, and
     works no door"; `player.test.ts`'s "a player's panel draws no arm-doors
     toggle, whether doors are armed or not".
   - Independently: `client/test/qa-door-mode.test.ts`.
   - On `eca70de` the player panel offers the arm-doors toggle beside a door
     (`renderPlayerPanel`'s `data-action="arm-doors"`), so the first and the
     last of these fail there.
   - "Whatever the window's arming bit holds" is shown at the panel only. At
     the board click no window can show it: only the DM console's toggle
     sets the bit true, the console is drawn only for a DM or an agent, and
     the server never makes either of them a player (`handlePromotion`
     refuses a dm or agent target, VTT-026). `onCell`'s branch order holds
     it, and Task 2's review read it.
5. [x] **The buttons follow the watched character.** `app.test.ts`'s "a
   player's door button names the character they watch, and a switch shows
   the new one's buttons" and "a player's switch to a character beside no
   door leaves no door button"; `qa-door-window.test.ts`'s "a player watching
   the character beside the door works it by its button; switching to the
   far one removes the button".
6. [x] **The DM's arming stands.** `client/test/dm-view.test.ts`'s "the
   doors toggle exists and flips the shared armed bit on each click" and
   "arming doors sends nothing over the wire — it is a local mode, not a
   command", and `app.test.ts`'s "the DM's armed door click names no actor,
   though a chip selects an actor of theirs", pass with their bodies
   unchanged; `app.test.ts`'s "the DM's own console toggle arms doors,
   board-wide" holds the board's armed label, and Task 2's break handing the
   board `false` turned it red; the DM's armed click on a door
   and on a plain square moved to a DM's table (`app.test.ts`'s "with doors
   armed, a DM's click on a door square opens it and sends no move" and "with
   doors armed, a DM's click on a plain square sends nothing at all, though
   unarmed it moves").
7. [x] **The real wire.** `client/e2e/doors.spec.ts`: the player, whose
   fighter stands on the cellar door's west neighbour, clicks "Close door
   (east)", the page offers no arm-doors control, and both seats' Story read
   "Brannoc closed a door".
8. [x] **The gate.** `task check` passed whole on `1874d89`, the comment
   ledger's commit, which changes no code after `f4449c6`; it judged no
   client mutation report, the stored one having measured `eca70de`.
   `task build:client` on that tree leaves `cmd/vtt/webdist` unchanged, the
   bundle being `874ed29`'s build, the last commit to change `client/src`.
   `task e2e` passed whole on `1874d89`.

## What the rules became

- One `work-door` button per door beside the watched character's token:
  VTT-881.
- The label: VTT-882.
- The command a button sends: VTT-883.
- A player's board click on a door square: VTT-884.
- No arm-doors toggle in a player's panel: VTT-885.
- No door button in a DM's or an agent's window: VTT-886.
- The buttons in the board's reading order: VTT-887, accepted at Task 0
  once the owner answered OD-1.
- The DM's and the agent's armed click: VTT-888, added at Task 0's review;
  no row held it, VTT-876 saying only that the command names no actor.
- Withdrawn at Task 2: VTT-873 (succeeded by VTT-883), VTT-874 (VTT-881 and
  VTT-884), VTT-875 (VTT-881 and VTT-885), VTT-879 (VTT-881).

Refused at Task 0: a row for the buttons following a switch (VTT-881's "the
character they watch" holds it); a row for the label being measured from
the token a click moves (theoretical, D6); a row that a player's window
never holds the bit true (the gateway holds it, VTT-026, and no window test
can break it). VTT-149, VTT-460, VTT-609 and VTT-876 stand as worded.

## The deliberate breaks

Each was made by an edit, run, and restored; each turned its tests red.

- **Task 1** (`doors.test.ts`, `player.test.ts`, `app.test.ts`): north and
  south swapped; the direction measured from the door; a hyphen dropped; the
  own square worded otherwise; the reach widened to two squares; Manhattan
  reach; a DM answered; the controller check dropped; any of the player's
  characters asked; the door-tile filter dropped; sorted by column first; the
  verb inverted; no actor named; `OpenDoors` read without its guard; the row
  drawn with no button; the button sending nothing; the panel asking the
  first controlled actor instead of `actingActor`; one command minted per
  button and reused on every click.
- **Task 2** (red in those files, and also in `qa-door-window.test.ts` and
  `qa-switch.test.ts`): the
  toggle drawn again for a player; the DM's armed plain click falling
  through to a move; `doorCommandFor` naming an actor; the board handed
  `false`; `currentSceneId`'s fallback changed; a player's click handled as a
  DM's; the arming bit ignored. A player's click reaching `doorCommandFor`
  armed could not be broken through a window (item 4).
- **Task 3** (the e2e, against the rebuilt binary): the button's command
  naming no actor; east and west swapped.

## Rule 9: how MapTool does this

MapTool's core has no door: a door is vision-blocking geometry
(`DungeonDraftImporter` strokes one into VBL) that a macro changes through
`Topology_Functions`, run from a macro button (`MacroButtonProperties`), not
from a mode of a click on the map. Borrowed: the act is a button in a panel.
Not borrowed: the macro; the window derives the buttons from the board, and
the server judges each command as before (VTT-149).

## The plan's gaps and the owner's answers

- **OD-1, the buttons' order**: the owner answered A, the board's reading
  order, row then column (VTT-887).
- **G1, files the ticket did not name**: `qa-switch.test.ts`,
  `tools/ts-mutation-equivalents.txt`, `spectator-view.test.ts`,
  `commands.test.ts` and `client/src/commands.ts` changed as the plan said;
  `command-surface.test.ts` did not change (D15), its warning about the join
  door's actions making the button's action `work-door`.
- **G4, item 4's last clause**: held at the panel and by reading, as above.
- **G5, `currentSceneId`'s fallback**: its kill, `player.test.ts`'s
  prototype test, now reaches it through `doorButtonsFor`; Task 2's break
  of the fallback turned it red.
- **D6, two tokens of one actor on one scene**: no code and no test. The
  direction is measured from the first such token beside the door. A scan
  of every `placements` list under `campaigns/` finds no actor placed twice.

## The existing tests that changed

- `doors.test.ts`: `doorCommandFor` lost its role and actor; the
  `mayWorkDoor` tests became `doorDirection` tests asserting a direction or
  none; the DM's and the agent's now assert none with a token of theirs
  beside the door; the player's armed command test left, its cases in
  `doorButtonsFor`'s tests.
- `qa-door-window.test.ts`: the player's module tests moved to
  `doorButtonsFor` and `doorDirection`, the toggle test to the button, the
  window test to a button click.
- `player.test.ts`: the toggle tests became the door button's, their kills
  kept; the toggle's click and label tests left with it.
- `app.test.ts`: the player's armed tests left, their premise unbuildable;
  the DM's armed click on a door and on a plain square moved to a DM's table
  (`dmTableWithDoor`), the second now also showing the unarmed move.
- `qa-switch.test.ts`: the armed switch test became a switch onto a party
  member standing in an open door.
- `commands.test.ts` and `client/e2e/doors.spec.ts` cite VTT-883 for the
  withdrawn VTT-873.
- `spectator-view.test.ts`: a comment that named `mayWorkAnyDoor` now names
  the scene pick `view/player.ts` once held.

## Phase 4a adjudications

- QA finding 1: spec gap — with no character selected a window acts as the
  first controlled one; whether buttons appear for it — no case: a player's
  selection follows the viewpoint, and a player watching nobody is shown no
  board (SPEC-015, SPEC-022) — nothing changed.
- QA finding 2: spec gap — two tokens of the watched character beside one
  door — D6, theoretical — nothing changed.
- QA finding 3: spec gap — where the buttons stand — D5, pinned by
  `player.test.ts`'s placement test; SPEC-022 is to say it (D18) — nothing
  changed.
- QA finding 4: spec gap — `doorDirection`'s return is not itself required —
  its words are VTT-882's — nothing changed.
- QA finding 5: needs an id — the buttons following a switch — refused at
  Task 0, held by VTT-881 — nothing changed.
- QA finding 6: spec gap — the current scene in a DM's window — the greatest
  scene id, as for a player; SPEC-022 is to say it (D18) — nothing changed
  in code.
- QA finding 7: spec ambiguity — VTT-888 does not map a door's state to the
  verb — the row allows either; `doors.test.ts`'s "armed, a closed door
  opens" and "armed, an open door closes — one control, both verbs" pin it,
  cited in VTT-888; SPEC-022 is to say it (D18) — nothing changed.
- QA finding 8: spec gap — the board-click side of "whatever the bit holds"
  — unbuildable (item 4) — nothing changed.
- QA finding 9: spec gap — SPEC-022 still described the player's arming —
  rewritten after this report — nothing changed in code.
- QA finding 10: brief defect — the surface handed to Task 2's QA lacked the
  fake socket's `OPEN` and the page's "null" origin — later briefs — nothing
  changed.

## Phase 4b

- **Task 0**: no Must; D5's claim about the tiles' order, M6's scope claim,
  the e2e's citation, the equivalents file's header block and verdict check
  6's search were corrected; VTT-882's and VTT-886's words made to stand
  alone; VTT-888 added.
- **Task 1**: no Must. A button's command was built when the panel was
  drawn, so every click before a repaint reused one request id, which
  `client/src/wire.ts` matches answers on; it is now built per click, with a
  test. The two-squares-off case was off on x only; one off on y was added.
- **Task 2**: one Must — `qa-door-mode.test.ts`'s fake fetch read paths only
  while the page's origin was "null", so it failed in the full suite after a
  file that sets the page URL; it now sets the URL and parses the path.
  The comments on the console's door group (`client/src/view/dm.ts`), on
  `toggleDoors` and on the board's doors bit (`client/src/app.ts`) pointed at
  SPEC-022 for what it does not say; they now point at the tests that hold
  them.
- **Task 3**: no Must or Should.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| A `DoorButton` carrying its command (D4) | It carries a function that builds the command per click | Built once at render, every click before a repaint reused one request id (Task 1's review) |
| D16 and Task 3 name VTT-883 for the e2e | The spec cites VTT-882 too | Its exact-label click is what the east/west break turned red |
| D14's list | `currentSceneId`'s doc and `doors.ts`'s header cut to warnings and pointers; `spectator.ts`'s `extras.doorsArmed` doc corrected; one `spectator.ts` comment left naming "whichever panel armed it" | `check:comments` bounds a touched comment block, and that comment sits in a block over the bound |
| — | A cut between two markers in `app.test.ts` removed Task 1's window tests while Task 2 was in the tree | Restored from `6193fe3` before Task 2's commit; the test names were then diffed against HEAD for every test file Task 2 touched |

## What could not be established

- **A click left of the drawn grid** moved the token to column 0 in Task 2's
  QA run. The board's click mapping is older than this ticket and outside
  it; whether that is intended was not traced.
- **`qa-door-mode.test.ts`'s cases for VTT-885, VTT-886 and VTT-888** are
  named with template literals (`${role}`, `${doorsArmed}`), which the
  requirements chain cannot cite, so those rows do not name them; they run
  in the suite all the same.
- **The client's stored mutation report** measured `eca70de`; this ticket
  changed the client after it, so `task check` judges none of it until a
  full Stryker run, which is the owner's to ask for (SPEC-023).

## What was deliberately left out

- The DM console's arming, and its board label.
- A door drawn differently on the board, or a button marking its door there.
- A lock, a key, or a door a token stands in being refused a close: "Close
  door (here)" closes the door the token stands in, as the server allows.
- A second token of one actor on a scene (D6).
