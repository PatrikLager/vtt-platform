# Only a door opens or closes: the change

**Ticket:** `docs/superpowers/specs/2026-11-03-only-a-door-opens-design.md`.
An agent that did not write it verified it and passed it with gaps.
**Plan:** `docs/superpowers/plans/2026-11-03-only-a-door-opens.md`, written
by that verifier and amended by Task 0's review.
**Where it came from:** finding A-8 of the alpha test of 2026-10-22, item 9
of the nine in the order of work of
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`: the agent opened
a stone wall square and the result was ok. The owner: "push merge sedan
A-8".
**The owner's rulings:**
- 2026-11-02, "Servern, en text för alla": the server writes a refusal in
  plain words where it refuses, one text for every reader, the agent's
  included.
- 2026-11-02: doors are the board engine's; nothing under `internal/rules`
  changes.
- 2026-11-03, at sign-off: the refusal reads `There is no door there.`, and
  the plan is approved with its reading of maps-as-geometry §6, "hard for
  players, free for the DM", as free of adjacency, not of the door's nature
  ("Godkänn planen"). The DM and the agent are refused too.

**The last code commit is `70365b0`.** Every code reference below is to
that tree unless it names another commit.

## The period, in commits

| Commit | What it holds |
|---|---|
| `d6f2b76` | Task 0: the ticket, the plan, VTT-608 and VTT-609 |
| `70365b0` | Task 1: `engine.State.IsDoor`, the door gate in `handleCommand`, `doorSquare`; the tests; QA's `internal/gateway/qa_door_square_test.go`; the comment ledger |

The comment ledger is in Task 1's commit rather than one of its own
(Deviations): `terrain.go` and `terrain_test.go` fell more than 1.0 under
their ceilings, and `--write-ledger` ran once, last, before that commit, so
the last code commit passes `check:comments` as it stands. It lowered seven
rows and added four. Besides this branch's files, it lowered ticket 15's
`client/src/app.ts`, `client/src/session.ts`, `client/src/view/spectator.ts`
and `internal/gateway/authz.go`, each under its row by less than 1.0, and
gave ticket 15's `plain_refusal_test.go` and `qa_plain_refusal_test.go`
their first rows.

## What the code does now

**The door gate.** For an `open_door` or a `close_door`, for every role,
`handleCommand` in `internal/gateway/server.go` takes the command's scene
and square from `doorSquare` and asks `engine.State.IsDoor`. A square whose
tile is not a door, a square with no tile, a square off the grid and a scene
the state does not hold are refused with `There is no door there.`, before
`ToEvent`, so nothing is appended. The gate runs after `authorize` and the
player's gates and before the validators.

**`IsDoor`.** It answers whether the tile at the square, in a scene the
state holds, has the kind `door`, through the scene map's and the tile map's
zero values, so an unknown scene and a missing tile read as no door with no
branch. It reads the tile and never `OpenDoors`, so an open door still
closes.

**A player far from the square.** `Authorize` runs `mayWorkDoor` first, so
a player's command at a square more than one square from every token they
control on its scene is refused with `mayWorkDoor`'s text whatever the
square is, and a player learns of a square's nature only for the square a
token of theirs stands on and the eight around it, but for the 2^31 edge
under QA adjudications.

**What did not change.** `engine.Apply`'s door arms, the client's fold and
door tool, and the tool descriptions, which say "Refused unless the square's
tile is a door" and are now true. `git diff 02b5874 70365b0 --
internal/rules rulesets scenarios contract client cmd` prints nothing; with
`tools` it prints only `tools/comment-ceilings.txt`, the ledger
(Deviations).

## Done looks like, answered

1. **[x]** The agent's `open_door` at a wall is refused, and nothing is
   appended. `TestOnlyADoorOpensOrClosesForEveryRole` in
   `internal/gateway/door_square_test.go`, whose first case is the agent at
   the cellar's wall (2,0); before the code it was red there ("was accepted
   at sequence 10"), and red again under K1.
2. **[x]** `open_door` and `close_door` are refused, appending nothing, at a
   wall, a floor and a square with no tile, for the DM, the agent and a
   player beside the square. The same test, both commands, with a square
   off the grid and a scene the table has not created for the DM and the
   agent; QA's `TestQADSTheDMAndTheAgentAreRefusedAtEverySquareThatIsNotADoor`
   and `TestQADSAPlayerBesideASquareThatIsNotADoorGetsTheDoorText` add
   squares on every side of the grid, a scene with tiles on one row only,
   and a floor under the player's own token.
3. **[x]** The refusal is in plain words, names no package and no id, and
   reads the same to every role. Every case of item 2's tests compares it
   whole with `There is no door there.`. K5 turned D-1 red, and the
   report's reading review re-ran K5 against QA's two, which went red too.
4. **[x]** A door square is accepted for the DM, the agent and a player
   beside it, and the tests that hold VTT-139, VTT-147 and VTT-149 pass
   unedited. `TestOnlyADoorOpensOrClosesForEveryRole`'s controls do it over
   the wire, which no test did for the agent before (the plan's G2); QA's
   `TestQADSTheDMAndTheAgentWorkADoorFromAnywhereOpenOrClosed` and
   `TestQADSAPlayerBesideADoorWorksItOpenOrClosed` alternate open and
   close on two doors. `git diff 02b5874 70365b0 --
   internal/gateway/authz_test.go internal/gateway/server_test.go
   internal/gateway/qa_move_gate_test.go`, the files those rows cite, prints
   nothing.
5. **[x]** `task check` exits 0 on the branch. It exited 0 on `70365b0`,
   2026-11-03; its figures are under Figures.

## What the rules became

- The ticket's one rule: **VTT-608**, a door command that `Authorize`
  accepts at a square that is not a door, or in a scene the state does not
  hold, is refused with `There is no door there.` for every role and
  appends nothing.
- The plan's, so the gate is no oracle across the map: **VTT-609**, a
  player far from the square is refused with one text whether it is a door
  or not. It held before this ticket, but for the 2^31 edge under QA
  adjudications; K6 shows the order that keeps it.
- **VTT-147** stands as worded: the DM and the agent work a door from
  anywhere, and what they work is a door. Its evidence gains
  `TestOnlyADoorOpensOrClosesForEveryRole` and QA's
  `TestQADSTheDMAndTheAgentWorkADoorFromAnywhereOpenOrClosed`.

## Phase 4a

The QA agent derived twelve Go tests from VTT-608, VTT-609, the kept rows
and a draft of SPEC-013's paragraph, without the source; eleven passed. Its
five fault injections, each in its own file, went red and were restored.

### QA adjudications

- `TestQADSAPlayerFarByTheWidestDistanceGetsTheFarText` failed:
  `mayWorkDoor`'s `abs` on `int32` wraps when the square is 2^31 from a
  token on one axis, so `Authorize` accepts and the gate answers `There is
  no door there.`, not `mayWorkDoor`'s text, against VTT-609's one text. The
  square is negative when the token stands on the grid, and far beyond any
  map's tiles when the DM has put the token off it (Task 1's review), so it
  is never a door; before this ticket the command was accepted and
  appended. Near-theoretical, with no consequence: by the owner's rule, no
  code, and the test was deleted. VTT-149 has had the same edge since it
  was written.
- `TestQADSADoorCommandWithNoSquarePassesTheGateOnlyAtADoorOrigin` pinned a
  command with no `at`, read as (0,0): the plan's D4 rules it
  near-theoretical, so it was deleted.
- A command with no `at`, on a scene whose (0,0) is a door, passes the gate
  and is refused by the fold with `engine: door opened without position`.
  D4; nothing is appended.
- A spectator's refusal names the command, so `open_door` and `close_door`
  read differently. VTT-139 asks for no single text; not a defect.
- The MCP door tools reach the same handler; no separate path.

## Phase 4b

| Commit | Must | Should | Nit |
|---|---|---|---|
| `d6f2b76` | 2 | 2 | 4 |
| `70365b0` | 0 | 2 | 1 |

Every finding was taken. The ones that changed what the work says:
- VTT-609 said "beside", which leaves out a token standing on the square;
  `mayWorkDoor` counts it. The row was reworded to VTT-149's measure before
  Task 0's commit.
- D-1 had no case for the DM at a wall, which ticket item 2 names; added.
- The plan's G3 said a map missing squares cannot load; one that declares
  no tiles at all can. Corrected.
- Three QA tests cited for VTT-608 passed with the gate removed (K1): one
  asks `IsDoor` alone, two show a door accepted. They are cited for VTT-147
  and VTT-149 only, and the `IsDoor` one carries no id, like E-1.
- The QA tests opened an open door and closed a closed one, which the
  ticket does not claim; their steps now alternate.
- The adjudication above first said the square is always negative; Task
  1's review moved a token off the grid and showed otherwise.

## The deliberate breaks

Each was made with a script that asserted the original text occurred once
and the broken text not at all, and undone by writing back the saved text,
checked by hash. D-1 is `TestOnlyADoorOpensOrClosesForEveryRole`, D-2
`TestAPlayerFarFromASquareIsToldTheSameWhetherItIsADoor`, E-1
`TestIsDoorIsTrueForADoorTileAlone`.

| Break | Red | Green, as expected |
|---|---|---|
| K1: the gate removed | D-1, D-2 | E-1 |
| K2: the gate a player's alone | D-1, at the agent's wall | D-2 |
| K3: `doorSquare` forgets `close_door` | D-1, at the agent's close | D-2 |
| K4: `IsDoor` asks for anything but a wall | D-1, at the floor; E-1 | D-2 |
| K5: the text loses its full stop | D-1, D-2 | |
| K6: the gate before `authorize` | D-2, the far wall answered with the gate's text | D-1 |
| K7: `IsDoor` forgets an open door | D-1, at the close; E-1 | D-2 |

## Rule 9: how MapTool does this

A door in MapTool is topology, not a square's nature. `DungeonDraftImporter`
imports a door as a `Wall` whose `DirectionModifier` for sight and light is
`SameDirection` when it is closed and `Disabled` when it is open, or, when
line of sight is imported as masks, a closed one as a stroke of VBL and MBL
and an open one as nothing; a macro (`eraseVBL`) erases topology without
asking what lies there. No command works a door at a square, so nothing
refuses one.
Borrowed: nothing. Here a door's state is folded, sent to whoever sees it,
and reasoned from by an agent, so a door opened at a wall is a false fact in
the log.

## The plan's gaps

- G1, the ticket's MapTool paragraph, and G6, two packages for one
  component, were corrected in the ticket before Task 0's commit.
- G2: the door tests that held VTT-147 ran against `Authorize` alone, and
  those that held VTT-149 did too, but for
  `TestAPlayerMayOnlyWorkADoorTheyAreNextTo`. No test worked a door over the
  wire as the agent. D-1's controls do it for each role.
- G3: corrected by Task 0's review (above).
- G4: the cases the ticket left open, decided by D2 and D4: a square off
  the grid and an unknown scene get the gate's text; a command with no `at`
  is read as (0,0).
- G5: the DM is now refused at a square that is not a door; approved at
  sign-off.
- G7: the comment ceilings were tight; the gateway tests went into a new
  file, and `server.go` gained two comment lines and stays under its
  ceiling.

## The measurements the plan stood on

The verifier's probes ran in a copy of the tree on `02b5874`. A check in
`Authorize` turned six tests red, those holding VTT-139, VTT-147 and
VTT-149 among them, since their fixtures hold no scene (P10); a check in the
fold turned six red in `perceive` and `campaign`, since `internal/eventgen`
draws door events on scenes with no tiles (P11). Over the eleven maps under
`scenarios/maps`, the example campaign's cellar and the alpha test's house,
16 of 4950 square–neighbour pairs were out of sight, each a room's corner
wall, none a door (P5), which is why the gate asks no sight. Task 1 bore
the prototype out: no existing test changed, and `git diff --numstat 02b5874
70365b0 -- '*_test.go'` shows additions only.

## Deviations

| Intended | What happened | Why |
|---|---|---|
| Task 2 runs `--write-ledger`, after Task 1's one commit; `git diff main -- internal/rules rulesets scenarios contract client cmd tools` prints nothing at every commit | `--write-ledger` ran in Task 1, before its commit, so `70365b0` holds `tools/comment-ceilings.txt` and that diff prints it | `terrain.go` and `terrain_test.go` fell more than 1.0 under their ceilings, which `check:comments` refuses, and `task check` runs on the last code commit |

## What could not be established

- Whether an agent told `There is no door there.` corrects itself. It can
  read every scene's tiles from `get_state`; no session has been played
  since.

## What was deliberately left out, and where it went

- Opening an open door and closing a closed one: accepted as before; not
  this ticket (D10, approved at sign-off). The plan and this report are its
  only record.
- `mayWorkDoor`'s words, which name the player's participant id and say
  "that door" at a wall: a candidate for the next ticket that touches it
  (D10). The plan and this report are its only record.
- The 2^31 edge in `mayWorkDoor`: no code, by the owner's rule.
- A sight check on a door command: measured near-theoretical (P5), no code.

## Figures

From `task check` on `70365b0`, 2026-11-03, 15:20 to 16:41. A first run on
the same commit, 09:32 to 09:52, stopped at `check:mutation`'s own tests,
not at the code: free disk fell from 18 to 16.0 GiB while it ran, so two of
`tools/check_mutation_test.py`'s tests met the gate's 16 GiB refusal
instead of the failure they assert. macOS had grown its swap to sixteen
1 GiB files, and Docker Desktop's virtual machine, which only
`task check:ts-mutation:full` uses, held much of the memory behind it. At
the owner's word Docker Desktop was stopped, and the second run began with
25 GiB free.

- **Go tests:** every package passed under `-race`, `internal/gateway`
  (132 s) and `cmd/vtt` (198 s) among them.
- **Go coverage:** 21 packages at or above their floors; `internal/engine`
  99.4% (floor 97.5), `internal/gateway` 94.9% (93.5).
- **Client coverage:** 24 files gated, each at its floor of 100%. This
  branch changes no client file.
- **Go mutation:** 15 packages, zero unadjudicated survivors;
  `internal/engine` 4 survivors and `internal/gateway` 2, all adjudicated
  before this branch, which adds no adjudication. Seven mutants timed out
  and were counted as killed, NOT measured: one in `internal/perceive`, four
  in `internal/sight`, one in `internal/rules` and one in `internal/mcp`,
  packages this branch does not touch.
- **Client mutation:** not measured. The stored report measured
  `526885bc0c77`; since then the tickets for the console lock keys, a name
  belonging to one participant, the DM booting a participant, whispering, a
  refusal that says why, and this one. `task check:ts-mutation:full`
  measures it when the owner asks, with Docker Desktop running.
- **Requirements chain:** 609 rows, 345 test files, 17 specifications;
  every citation resolves.
- **New prose:** 500 added lines across 5 files, clean.
- **Comments:** 355 files, 3 added comment lines, clean.
