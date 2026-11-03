# Plan: only a door opens or closes

- **Ticket:** `docs/superpowers/specs/2026-11-03-only-a-door-opens-design.md`
  (A-8 of `docs/superpowers/notes/2026-10-22-alpha-one-findings.md`).
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-03 by an agent that did not write the ticket, read-only
    against main `02b5874` and the uncommitted ticket.
  - Probes ran in a copy of the tree outside the repository. P4, P10 and P11
    are prototypes in that copy, measured and then deleted; nothing in the
    repository was edited but this file.
  - `task check`, `task e2e`, Stryker and gremlins were not run.
- **To be committed as:** `docs/superpowers/plans/2026-11-03-only-a-door-opens.md`.
- **Base:** main `02b5874`.
- **Branch:** `fix/only-a-door-opens`.
- **Owner's rulings taken in:**
  - 2026-11-02, "Servern, en text för alla": the server writes a refusal in
    plain words where it refuses, one text for every reader, the agent's
    included; the window shows it with nothing before it (VTT-604). The owner
    approves a refusal's exact words at sign-off (Q1).
  - 2026-11-02: doors are the board engine's. Nothing under `internal/rules`
    changes.
  - 2026-10-21: a near-theoretical edge with a small consequence gets no code,
    no test, no debt entry and no spec sentence. Applied in D3 and D4 after
    measuring (P5), not assumed.
  - 2026-09-04: nobody uses the product; no migration, no phased removal.
  - 2026-08-12, "hard for players, free for the DM" (maps-as-geometry design
    §6): read as free of adjacency, not of the door's nature (G5).
  - 2026-10-20, "two-step stays" (kind and control are separate): a player
    can still control a creature that is not a party member (D3).
  - 2026-09-23, one source for every fact (D2).
- **Signed off:** 2026-11-03 by the owner: Q1 "There is no door there.",
  and the plan approved with G5's reading ("Godkänn planen").
- **Ids:** R1 = VTT-608, R2 = VTT-609.

## The verdict, check by check

1. **Every path resolves. By command** (P1). All eleven paths the ticket
   names exist, and every symbol it names is in the file it names:
   `Authorize` and `mayWorkDoor` in `authz.go`, `handleCommand` in
   `server.go`, `ToEvent` in `convert.go`, the `DoorOpened` and `DoorClosed`
   arms of `engine.Apply`, `engine.State.Blocked` in `terrain.go`,
   `doorCommandFor` in `client/src/view/doors.ts`, the text "Refused unless
   the square's tile is a door" in `tools/toolgen/main.go`, and
   `DungeonDraftImporter` in MapTool.
2. **"Done" is an observation. By command.** Items 1 to 3 name tests that do
   not exist, and the behaviour they pin is absent today (P2): an `open_door`
   or `close_door` at a wall, a floor, a square with no tile and a square off
   the grid is answered ok and the log's head moves, for the agent, the DM
   and a player beside the square. `git grep -n "There is no door there"`
   prints nothing. Item 4's tests pass today (P3) and pass unchanged under
   the prototype (P4). Item 5 is the gate.
3. **Each rule is breakable. A reading.** The ticket's one rule is broken by
   removing the check (D-1 red), by keeping it a player's alone (D-1's DM and
   agent cases red), and by appending anyway (`refusedUnmoved` reads the
   head). K1 to K7 below name each.
4. **The scope matches the claim. By command, then a reading.**
   `handleCommand` has one caller, `answerCommand`. P4 ran every Go package
   that folds, projects or relays a door event, `cmd/vtt`'s scenario library
   included, against the prototype: no existing test changes. The client is
   untouched (`doorCommandFor` builds a command for a door tile alone). Two
   packages change, not one (G6), and the tool descriptions do not (D6).
5. **No recorded decision is contradicted silently. A reading.**
   - SPEC-013's "What the DM and the agent are free of" keeps "they work a
     door from anywhere"; its Consequences sentence "The DM and the agent are
     bound by the table's cells, by promotion's bound, by `mayVoice` and by
     `mayWhisper`, and by no player rule" gains the door gate. The ticket's
     "VTT-147 stands as worded" and its naming of SPEC-013 cover it.
   - VTT-147, VTT-149, VTT-139, VTT-158, VTT-379 and VTT-601 to VTT-603 stay
     true as worded (P3, P4).
   - VTT-156, "A player's refusal for a square they cannot see is the same
     string whatever stands on the square", is worded for every refusal, not
     the move gate's alone. It stays true only if the door refusal is one
     string; a text naming the tile would break it at an unseen square
     (D5, Q1).
   - Maps-as-geometry §6, "DM and agent unrestricted": G5.
6. **The records it moves. By command, then a reading.** The section names
   SPEC-013, which resolves. `git grep -il door docs/specifications` finds
   SPEC-007 and SPEC-009 (the join door, not a map's), SPEC-013, SPEC-016
   (which door events a seat is sent), SPEC-018 (the byte bound on
   `DoorOpened.scene_id`), SPEC-021 (the doors in a character's sight) and
   SPEC-022 (the window's door tool). The fold, the projection, a character's
   log and the window do not change, so SPEC-013 alone moves.

## The gaps this plan inherits

- **G1. The ticket's MapTool paragraph is half the picture.**
  `DungeonDraftImporter` imports a door two ways, and only one is a stroke
  of VBL; see Rule 9. Its conclusion, that no command works a door at a
  square, stands.
- **G2. Item 4's tests cannot see the change.** The door tests that hold
  VTT-147 and VTT-149 are `Authorize`-level, but for
  `TestAPlayerMayOnlyWorkADoorTheyAreNextTo`; VTT-139's spectator is refused
  by the role table before the gate. They pass by construction. No test
  today has the agent work a door over the wire. D-1's controls add the
  agent's, the DM's and an adjacent player's acceptance at a door, over the
  wire.
- **G3. "A square of the scene with no tile" is a scene that declares no
  tiles.** `mapdef.CheckEverySquarePresent` refuses a map or an adventure
  scene missing some squares, on both load paths, but accepts one that
  declares none; such a scene loads, and no shipped map, campaign or
  adventure is one. `create_scene` left the platform on 2026-09-02.
  `newGWFixture`'s `scn1` has no tiles. A scene without tiles holds no door,
  so every door command in it is refused; it is tested as one table row.
- **G4. Cases the ticket leaves open:** a square off the grid, an unknown
  scene, a command with no `at`. D2 and D4 decide them.
- **G5. The DM is now refused something.** Maps-as-geometry §6 says "DM and
  agent unrestricted" under "hard for players, free for the DM". Its table
  reads `yes` for the DM and `yes (adjacent)` for a player, so the freedom it
  grants is from adjacency; the tool description of the same arc says
  "Refused unless the square's tile is a door" for every reader. Noted for
  the owner, not asked.
- **G6. Two packages, though the ticket says one component.** The gate is in
  `internal/gateway` and the question it asks is a new query in
  `internal/engine` (D1, D2). The ticket allows it ("or `internal/engine` if
  the plan asks the engine the question"); the board engine is one component
  in the owner's sense.
- **G7. The comment ceilings are tight** (P7): `server.go` is at 21.31 of
  21.4, `terrain.go` at 52.24 of 52.3, `server_test.go` at 23.89 of 23.9. New
  tests go in a new file.

## Measurements this plan stands on

All on 2026-11-03 at `02b5874`.

| # | Probe | Result |
|---|---|---|
| P1 | `test -e` on the ticket's paths; `grep -n` on each symbol; MapTool's source | all resolve |
| P2 | Over the wire, `newGWFixture` and `seedCellar`: `open_door` then `close_door` from the agent, the DM and the player (whose `tok-fighter` stands at (2,1) and `t1` at (3,7) of `scn1`), at the cellar's walls (0,0), (1,0), (2,0), its floors (1,1), (2,1), `scn1`'s tile-less (5,5), (4,4), (3,6), the cellar's (9,9) off the grid, scene `nowhere`, no `at`, and the door (0,1) | wall, floor, no tile, off the grid: ok, the head moves, every role. `nowhere`: `engine: door opened in unknown scene "nowhere"` (and `closed`); no `at`: `engine: door opened without position`; the player two squares from (0,1), and at `nowhere`: `gateway: not authorized: participant "<their id>" has no token adjacent to that door` |
| P3 | `go test ./internal/gateway/ -run` the tests VTT-139, VTT-147, VTT-149 and the door tests cite; `go test ./internal/engine/ -run Door`, on the real tree | pass |
| P4 | Prototype of D1 to D5 (`IsDoor`, the gate, `There is no door there.`); P2 again; `go test ./internal/gateway/ ./internal/engine/ ./internal/mcp/ ./cmd/vtt/ ./internal/perceive/ ./internal/campaign/ -count=1` | every case that P2 found ok, `nowhere` and no `at` read `There is no door there.`, the head unmoved; the far player keeps `mayWorkDoor`'s text; the agent's and the DM's at (0,1) are ok. All six packages pass, `cmd/vtt` in 180 s |
| P5 | `sight.VisibleFrom` (tolerance and range not supplied, as `perceive` asks) from every square `Blocked` lets a token stand on, to each of its eight neighbours in the grid, doors closed: the 11 maps under `scenarios/maps`, `campaigns/example/maps/cellar.json`, and the alpha test's `house` scene | 737 squares, 4950 pairs, 16 unseen (0.32%): each the corner wall of a room, diagonal behind two walls. None is a door |
| P6 | `git grep` the fold's door refusals in every test | no Go test pins the gateway's answer to a door command at an unknown scene; `client/test/fold-rejections.test.ts` pins the client fold's own |
| P7 | `measure` and `share_of` from `tools/check-comments.py` against `tools/comment-ceilings.txt` | `server.go` 21.31 (21.4), 976 non-blank lines, 208 comments; `terrain.go` 52.24 (52.3), 67 and 35; `terrain_test.go` 29.36 (29.4); `server_test.go` 23.89 (23.9); `authz.go` 11.69 (11.9) |
| P8 | MapTool at `f4b7fef6c` (the RPTool checkout rule 9 names) | see Rule 9 |
| P9 | Writers of `DoorOpened` and `DoorClosed` outside tests | `ToEvent`; `perceive`'s `doorTransitions`, a projection of folded state; `internal/eventgen`, the property tests' model |
| P10 | Prototype: the check in `Authorize`, for every role | six red: `TestAuthorizeTableAllCommandsAllRoles` (VTT-139), `TestAuthorizeDMMayWorkDoorRegardlessOfTokenPosition` (VTT-147), `TestAuthorizePlayerMayWorkAdjacentDoor`, `TestAuthorizePlayerMayWorkAdjacentDoorAwayFromTheOrigin`, `TestAuthorizePlayerMayWorkDiagonallyAdjacentDoor`, `TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis` (VTT-149). `ownershipFixture` and `doorFixture` hold no scene |
| P11 | Prototype: both fold arms refuse a square whose tile is not a door; `go test` on `engine`, `perceive`, `campaign`, `eventgen`, `sight`, `rules/...` | six red: `TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed`, `TestAnIntroducedSceneArrivesWithItsDoorsAlreadyOpen`, `TestADoorOpenedOutOfSightArrivesWhenTheSquareComesIntoView` (`perceive`); `TestACharacterLogIsWhatItsCharactersEyesAreGiven`, `TestEveryCharacterLogFoldsAtEveryPrefix`, `TestRebuildEqualsLiveProperty` (`campaign`) |

## Constraints that bind every task

- **CLAUDE.md rules:**
  - 2: no gate moves. `tools/comment-ceilings.txt` is lowered by
    `--write-ledger` alone.
  - 3: the contract does not change. No field, no message, no regenerated
    file (D6).
  - 4: one fold. `engine.Apply` does not change; `IsDoor` is a query, as
    `Blocked` is.
  - 5: no game-system vocabulary, tests included. "door", "wall" and "floor"
    are the board's.
  - 8: cite names; never `file:line`, never `.superpowers/`.
  - 9: answered below.
  - 10: a comment under `internal/` is a warning, a pointer, or an exported
    symbol's one-line doc. The words of the refusal are SPEC-013's, pointed
    to, not explained.
- **Nothing outside the board engine changes:** `git diff main --
  internal/rules rulesets scenarios contract client cmd tools` prints nothing
  at every commit.
- **The comment ceilings** (P7). In `server.go` each comment line needs about
  four code lines beside it; `share_of` measures each touched file before its
  commit. A test cites its rule on a citation-only line.
- **`check:new-prose`** holds added comment lines to an 85-column band,
  tests included.
- **`go-arch-lint`.** `internal/gateway` already imports `internal/engine`;
  a `gateway_test` test imports what its neighbours do.
- **Coverage floors:** `internal/engine` 97.5, `internal/gateway` 93.5.
- **The Go mutation gate runs in `task check`.** Each new condition has a
  test that goes red when it is negated. gremlins does not mutate a `case`,
  so `doorSquare`'s two arms are held by K3. No adjudication in
  `tools/mutation-equivalents.txt` sits in `server.go` or `terrain.go`.
- **A new function or test goes after a closing brace** (`check:doc-owner`).
- **A test the register cites has a literal name.**
- **Before every commit:** `python3 tools/check-new-prose.py main`; `python3
  tools/check-requirements-chain.py .`; `python3 tools/check-comments.py
  main`, after staging.
- Stage and commit in separate calls, and verify what landed.
- Never `git checkout --` to undo a break: the inverse edit, checked with
  grep, after asserting the broken text is not already in the file.
- **The review gate refuses untracked files.** Park scratch files outside the
  tree with `cp` and `cmp`.
- **Hash the tree before and after any reviewer.**
- **The pre-push hook takes minutes.** 600 000 ms, its whole output to a
  file, the machine's load checked first.

## Rule 9: how MapTool does this

In MapTool at `f4b7fef6c` (P8):

- **A door is not a square's nature, and no command works one at a square.**
  It is topology. `DungeonDraftImporter` imports a Dungeondraft portal one of
  two ways. With the option to import line of sight as walls, it becomes a
  `Wall` in the zone's `WallTopology`, whose `DirectionModifier` is
  `SameDirection` when the portal is closed and `Disabled` when it is open.
  Otherwise a closed portal becomes a one-pixel stroke of `WALL_VBL` and
  `MBL`, and an open one imports nothing. A table also draws a door as a
  token carrying its own VBL (`ZoneRenderer`'s remark on a "door" stamp).
- **What opens one is a macro.** `eraseVBL` (`Topology_Functions`) erases the
  topology inside the shape it is given and asks nothing about what lies
  there; `setTokenVBL` changes a token's.
- **So nothing refuses a door command at a wall.** There is no such command.

Borrowed: nothing. Checked and rejected: `eraseVBL`'s "erase whatever is
there". Here a door's state is folded, sent to whoever sees it, and reasoned
from by an agent; a door opened at a wall is a false fact in the log (A-8).

## Design questions for the owner

1. **Q1. The words.** One text for both commands and every square that is
   not a door.
   - **Answered 2026-11-03: `There is no door there.`**
   - Recommended: **`There is no door there.`**
     - The agent needs to know its square was wrong. It named the square, and
       `get_state` gives it every scene's `Tiles`, so naming the tile tells it
       nothing it cannot read. A-8's failure was an ok, not a refusal that
       said too little.
     - A player learns nothing they lack. In the 13 maps measured, 99.68% of
       the squares beside a square a token can stand on are in sight from it
       (P5), and the rest are corner walls (D3).
     - VTT-156 stays true as worded: one string at any square, seen or not.
     - One text needs no words for a square with no tile, a square off the
       grid, or a scene the table has not created (D4).
   - Alternative: **name the tile**, `There is no door there: a wall.`
     - Five texts: a wall, a floor, no tile, off the grid, no such scene.
     - Breaks VTT-156 at an unseen square unless sight is asked first, which
       is a sixth text and the sight check D3 rejects.
   - Alternative: **the move gate's form**, `You cannot open that: there is no
     door there.` and `You cannot close that: there is no door there.`
     - Two texts. The sender knows which command it sent.

**Noted for the owner, not asked.**
- **The DM is now refused at a square that is not a door** (G5). A DM who
  wants a door where the map has a wall has no command for it: terrain is
  fixed once a scene is created (`ErrSceneExists`).
- **An agent that mistypes a scene id is told `There is no door there.`**,
  not that the scene is unknown (D4).
- **The alpha campaign keeps its door opened at a wall** (A-8). The fold does
  not change, so the campaign still opens; its `OpenDoors` holds the wall's
  key, which neither `Blocked` nor `sight` reads for a wall.
- **`mayWorkDoor`'s refusal keeps its words** (VTT-149): a player not beside
  a square reads `gateway: not authorized: participant "<their id>" has no
  token adjacent to that door`, an id and "door" at a wall included. A
  candidate for the next ticket that touches it.

## Decisions

1. **D1. The gate lives in `handleCommand`, for every role, after
   `authorize` and the player's gates, before the validators.** It refuses
   before `ToEvent`, so nothing is appended.
   - **Not in `Authorize`.** `Authorize` decides who may issue a command, and
     its refusals wrap `ErrUnauthorized`; a wall is not about the issuer, the
     same reason `handleCommand`'s own comment gives for the validators. And
     it is measured (P10): `ownershipFixture` and `doorFixture` hold no scene,
     so a check there turns six tests red, among them those that hold VTT-139,
     VTT-147 and VTT-149. Item 4 forbids it.
   - **Not in the fold.** Measured (P11) and read:
     - six tests red in `perceive` and `campaign`. `internal/eventgen`'s
       `openDoor` and `closeDoor` draw door events at random squares of
       scenes with no tiles, and the property tests fold them; the model
       would have to learn tiles;
     - `client/src/fold.ts` mirrors each refusal of the Go fold
       (`client/test/fold-rejections.test.ts`): a client change, a webdist
       rebuild, and the client's mutation report out of date;
     - a fold refusal is replay's too: `campaign.Open` returns the rebuild's
       error, so a log holding one such event no longer opens. The alpha
       campaign's does. Ruling 4 makes that acceptable, not wanted;
     - the refusal would reach the window as `engine: …`, against the
       owner's plain words;
     - `engine.State.Blocked`'s own doc: the engine answers, the gateway
       enforces.
   - What the fold would buy is that no writer could log a door at a wall.
     `ToEvent` is the one production writer of door events (P9), and it sits
     behind the gate.
2. **D2. The gate asks the engine: `func (st *State) IsDoor(sceneID string,
   x, y int32) bool`, in `terrain.go` after `Blocked`.** It returns whether
   the tile at the square, in a scene the state holds, is a door: `Kind ==
   "door"`, through `gridKey`.
   - Forced by one source. `gridKey` is unexported, and the square's key is
     already formatted four times (`gridKey`, `sight`'s and `perceive`'s
     `squareKey`, `mapdef`'s); `sight.squareKey`'s comment warns that another
     copy which disagrees fails silently. The gateway formats no key today.
   - `Blocked` cannot answer it: an open door and a floor both read `false,
     ""`.
   - An unknown scene and a square with no tile read as no door through Go's
     zero values, with no branch. A square off the grid has no tile in any
     scene `load_map` or `load_adventure` built: `mapdef.CheckTilesInsideGrid`
     refuses one on both load paths.
   - **No grid-bounds check.** Only a `SceneCreated` appended by hand could
     record a door tile off its grid; `mapdef` refuses one on both load paths.
     Near-theoretical, and the consequence is one event that `Blocked` and
     `sight` ignore: ruling 3.
   - It reads the tile, never `OpenDoors`: an open door is still a door, so
     `close_door` passes (K7).
   - The gate reads the snapshot `authorize` took. A scene's tiles never
     change once created (`ErrSceneExists`), so only a scene created between
     the snapshot and the append is answered as unknown, and a retry passes.
3. **D3. For a player, the gate runs after `mayWorkDoor`, and asks no sight
   first.**
   - **After `mayWorkDoor`, forced.** Before it, a player could ask of every
     square of every scene whether it is a door. After it, a square more than
     one square from every token of theirs is refused with `mayWorkDoor`'s one
     text, door or not (R2, K6), and they learn something only of the square a
     token they control stands on and the eight around it.
   - **No sight check, measured.** A square beside a token the player
     controls can be one their eyes do not see in three ways:
     - **a corner:** diagonal, behind two blockers. P5: 16 of 4950 pairs, each
       a corner wall of a room, none a door;
     - **another party member of theirs, not the one they watch:** `MayWatch`
       lets them watch it and see the same squares;
     - **a creature that is not a party member and that they still control:**
       the end of a run under the two-step ruling, "rare at the table"
       (2026-10-20); `scenarios/door-watch.json`'s `act-hidden`, granted `ACTOR_KIND_NON_PARTY`, is one. It has no
       eyes; its player watches a party member as a spectator does (B-4), and
       the squares beside the creature may be out of that one's sight.
   - In each case the answer is one bit, door or not, about a square beside a
     token they control. The window never asks it: `doorCommandFor` builds a
     command only for a door tile in the window's own fold. Today the same
     command is answered ok, door or not. Near-theoretical with a small
     consequence: no code, no test, no spec sentence (ruling 3). VTT-156 holds
     because the text is one string (D5).
4. **D4. The gate answers a scene the state does not hold, and a command
   with no `at`.**
   - **An unknown scene** gets the gate's text. Only the DM and the agent
     reach it: a player has no token on a scene that does not exist, so
     `mayWorkDoor` refuses first.
     - Forced by the owner's plain words: today the agent is answered
       `engine: door opened in unknown scene "…"`, a package's words. The
       window never sends it: `doorCommandFor` names the scene the window
       shows.
     - It is true: a scene that does not exist holds no door.
     - It costs no branch (D2), and no Go test pins the old answer (P6).
     - Cost: an agent that mistypes a scene id is told there is no door there,
       not that the scene is unknown. It reads `get_state`'s `Scenes`.
     - The fold keeps its refusal for replay
       (`TestDoorCommandsRejectUnknownSceneAndMissingPosition`).
   - **No `at`:** the getters read it as (0,0). The gate refuses it unless
     (0,0) is a door, and then the fold refuses it (`door opened without
     position`); nothing is appended either way. The tool marks `at` required
     and the window always sends it: no branch (ruling 3).
5. **D5. One text, the same for both commands, every role, and every square
   that is not a door** (Q1). The literal sits in `handleCommand`, where the
   move gate's sit (D5 of `docs/superpowers/plans/2026-11-02-a-refusal-says-why.md`).
6. **D6. The tool descriptions do not change.** "Refused unless the square's
   tile is a door" becomes true as written. Changing it regenerates
   `contract/gen/tools/tools.json`, `cmd/vtt/tools.json` and
   `contract/testdata/expected_tools.json`; the first and the last are
   inputs `tools/ts-mutation-inputs.py` hashes, so the client's stored
   mutation report would go out of date for no reader's gain.
7. **D7. The client does not change.** `doorCommandFor` sends only at a door
   tile, and a refusal shows as its error with nothing before it (VTT-604).
   No webdist rebuild, no Stryker.
8. **D8. The tests go in a new file, `internal/gateway/door_square_test.go`**,
   package `gateway_test`, held to the default ceiling, 25.0
   (`server_test.go` is at 23.89 of 23.9, G7). The engine's goes in
   `internal/engine/terrain_test.go`, beside `TestBlockedOnUnknownSceneBlocks`;
   code lines only lower its share. They use `newGWFixture`, `seedCellar`,
   `refusedUnmoved`, `accepted` and `stateWithCellar` as they stand.
9. **D9. One comment on the gate**, a warning with a pointer, e.g. "Keep this
   after authorize: before it, a player asks of any square whether it is a
   door (SPEC-013)." `doorSquare` is unexported and gets none; `IsDoor`
   gets its one-line doc. Each is measured with `share_of` (P7).
10. **D10. Not this ticket:** opening an open door, closing a closed one,
    `mayWorkDoor`'s words.

## Requirements

Allocated at Task 0 with `requirement-id`, each `**OPEN — no test yet**`
until its test lands. R1 and R2 are this plan's labels; D-1, D-2 and E-1 name
tests.

- **R1.** "An `open_door` or `close_door` that `Authorize` accepts, naming a
  scene the state does not hold or a square whose tile is not a door, a
  square with no tile included, is refused with `There is no door there.`,
  whatever the issuer's role, and appends nothing." D-1. Red under K1 to K5
  and K7.
  - Its controls: what `Authorize` refuses is VTT-139's and VTT-149's;
    opening an open door is not claimed.
- **R2.** "A player's `open_door` or `close_door` naming a square more than
  one square, on some axis, from every token they control on its scene is
  refused with one text, whether that square is a door, is not one, or lies
  in a scene the state does not hold." D-2. Red under K6.
  - Measured as VTT-149 measures it: a token standing on the square is at
    distance 0 and is not far from it.
  - The ticket does not state it; the plan does (D3), because it is what keeps
    the gate from being a terrain oracle across the map.

Kept, wording still true: VTT-139, VTT-147, VTT-149, VTT-156 (D5), VTT-158,
VTT-379, VTT-601 to VTT-603, VTT-604. VTT-147's evidence gains D-1, the
over-the-wire half it lacks (G2).

| Ticket item | Observed by |
|---|---|
| 1 | D-1's first case: the agent's `open_door` at the cellar's wall (2,0) |
| 2 | D-1 |
| 3 | D-1, the text compared whole |
| 4 | D-1's controls; the VTT-139, VTT-147 and VTT-149 tests passing unedited |
| 5 | Task 2 |

## The existing tests that change

None (P4).

## Tasks

### Task 0: sign-off, the records, the ids

- **Do:**
  - Sign-off is given, with Q1 answered.
  - Allocate R1 and R2 with `requirement-id`.
  - Commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean; the rows exist; the chain passes.

### Task 1: the door gate (R1, R2; D1 to D5, D8, D9; items 1 to 4)

- **Files:** `internal/engine/terrain.go` (`IsDoor`),
  `internal/engine/terrain_test.go` (E-1), `internal/gateway/server.go`
  (`handleCommand`, `doorSquare`), `internal/gateway/door_square_test.go`,
  new; `docs/requirements.md`.
- **The code:**
  - `IsDoor` as D2 states it.
  - `doorSquare(cmd)` returns the scene id, the `at` and true for an
    `open_door` or a `close_door`, and false for any other command. After
    `handleCommand`'s closing brace.
  - In `handleCommand`, after the player's gates and before the validators'
    comment: `if sceneID, at, ok := doorSquare(cmd); ok &&
    !st.IsDoor(sceneID, at.GetX(), at.GetY())`, refuse with the text Q1
    settles.
- **New tests**, each with a literal name and its id line:
  - **D-1** (R1, VTT-147), `TestOnlyADoorOpensOrClosesForEveryRole`:
    `newGWFixture` and `seedCellar`. `refusedUnmoved` with the exact text,
    for `open_door` and `close_door` each:
    - the agent at the wall (2,0) first, item 1's case; then the DM at the
      wall (2,0); then the agent and the DM at the floor (1,1), at `scn1`'s tile-less (5,5), at the cellar's
      (5,5) off the grid, and at scene `nowhere`;
    - the player at the wall (2,0) and the floor (1,1), beside
      `tok-fighter`, and at `scn1`'s (3,6), beside `t1`.
    - Controls, each `accepted`: the agent opens and closes the door (0,1);
      the DM does the same; the DM moves `tok-fighter` to (1,1), and the
      player opens and closes (0,1).
  - **D-2** (R2), `TestAPlayerFarFromASquareIsToldTheSameWhetherItIsADoor`:
    from (2,1) the player's `open_door` at the wall (0,0), the door (0,1) and
    scene `nowhere` are refused and unmoved, with one text, which is not
    `There is no door there.`. Control: at the wall (2,0), beside, the gate's
    text.
  - **E-1**, `TestIsDoorIsTrueForADoorTileAlone`: on `stateWithCellar`, true
    at (0,1), and still true after a `DoorOpened` there; false at the wall
    (0,0), the floor (1,1), (5,5) off the grid and in scene `nowhere`; on
    `seedScene`'s `scn`, which has no tiles, false. No id: it pins the query
    for the mutation gate.
- **Order:** the tests, red (D-1 and E-1 red; D-2 red at its control alone,
  since its far cases hold today); the code; the breaks.
- **True when:** `go test ./internal/gateway/ ./internal/engine/
  ./internal/mcp/ ./cmd/vtt/ ./internal/perceive/ ./internal/campaign/`
  passes; `go vet` is clean; `share_of` holds every touched file; the
  `git diff main` of the constraints prints nothing.
- **Breaks**, each undone by its inverse edit:
  - **K1:** the gate removed. D-1 red at its first case; D-2's control red.
  - **K2:** the gate moved inside `if p.Role == identity.RolePlayer`. D-1's
    agent and DM cases red.
  - **K3:** `doorSquare`'s `CloseDoor` arm removed. D-1's `close_door` cases
    red.
  - **K4:** `IsDoor` compares `!= "wall"`. D-1's floor, no-tile and `nowhere`
    cases red; E-1 red.
  - **K5:** the text loses its `.`. D-1 and D-2's control red.
  - **K6:** the gate moved before `s.authorize`. D-2 red: the far wall reads
    the gate's text, the far door `mayWorkDoor`'s.
  - **K7:** `IsDoor` returns `Kind == "door" && !sc.OpenDoors[key]`. D-1's
    `close_door` controls red; E-1's opened door red.
- **Then:** Phase 4a, Phase 4b, the fixes, one commit.

### Task 2: the gate, whole

- **Do:**
  - `python3 tools/check-comments.py main`; `--write-ledger` once if a file
    is more than 1.0 under its ceiling.
  - Check free space and the load. Run `task check` whole on the last code
    commit, with `start_new_session=True`, all output to a file; require the
    gate's own completion line and `check:ts-mutation`'s own line.
  - Tell the owner which commit the client's stored mutation report measured
    and the tickets since; the full run is his to ask for.
- **True when:** the gate is green.

### Task 3: the report

- **Do:** write `docs/reports/2026-11-03-only-a-door-opens.md` and give it
  its reading review.
- **It covers:** each "Done looks like" item with its observation; K1 to K7
  with what went red; Rule 9; the gaps and Q1's answer; D1's measurements
  (P10, P11) and D3's (P5); Phase 4a's adjudications under their own
  heading; the gate's figures.
- **True when:** it answers the ticket's five items in their numbering.

### Task 4: the specification

With the `specification` skill, from the report, SPEC-013:
- **Status:** `doorSquare` among `server.go`'s names; `engine.State.IsDoor`
  for the door gate; `door_square_test.go` and `qa_door_square_test.go`.
- **The command path:** a step after the player's gates: for an `open_door`
  or a `close_door`, for every role, the door gate.
- **The door gate**, a new paragraph: it asks `IsDoor` of the command's scene
  and square and refuses a square whose tile is not a door, and a scene the
  state does not hold, with the text; it runs after `Authorize`, so a player
  is told of a square beside their tokens alone (R2); it asks no sight.
- **What the DM and the agent are free of:** they work a door from anywhere,
  and the door gate binds them.
- **Consequences:** the bullet on what binds the DM and the agent gains the
  door gate; a bullet that a door command names a door, for every role; the
  refusal bullet names R1's text and says it names no package and no id.
- **Requirements:** R1, R2.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer; every id resolves; the move gate is still
  `Blocked`'s one caller outside the tests.

Commit the report, then the specification. Push and merge on the owner's
word.

## For Phase 4a

Hand the QA agent:
- R1, R2, and the texts of VTT-139, VTT-147, VTT-149 and VTT-156;
- SPEC-013's changed paragraphs as drafted from D1 to D5;
- the surfaces as signatures and names, never the source:
  - the wire: a `ClientCommand` with `openDoor` or `closeDoor`, and its
    `CommandResult`;
  - `go doc ./internal/gateway`: `Authorize`, `ErrUnauthorized`, `New`;
  - `go doc ./internal/engine`: `State`, `State.IsDoor`, `State.Blocked`,
    `Apply`, `Scene`, `Tile`;
  - `newGWFixture`, `seedCellar`, `dial`, `dialOn`, `sendCommand`,
    `readResult`, `refusedUnmoved`, `accepted`, `head` in
    `internal/gateway`'s tests.

Tell it:
- never run `task check`, `task e2e`, Stryker or gremlins;
- its file: `internal/gateway/qa_door_square_test.go`, package
  `gateway_test`, held to the default ceiling, 25.0. `go-arch-lint` runs in
  the hook;
- `semgrep` bans game words in every Go file under `internal/` and `cmd/`,
  tests included;
- known and not findings: a command with no `at` read as (0,0) (D4); a door
  tile recorded off the grid (D2); the three ways a square beside a token goes
  unseen (D3); opening an open door; `mayWorkDoor`'s words; a scene created between
  the snapshot and the append (D2);
- give each test a literal name and its rule's id;
- the tree is hashed before and after it.

## For Phase 4b

Hand the reviewer the diff with `git diff HEAD` and tell it:
- check each scope claim ("only", "never", "every") names the search that
  bounds it;
- check each comment under `internal/` is a warning or a pointer (rule 10),
  and each citation a name (rule 8);
- check the text against Q1's answer and R1, word for word, and that
  `git diff main -- internal/rules rulesets scenarios contract client cmd
  tools` prints nothing.
