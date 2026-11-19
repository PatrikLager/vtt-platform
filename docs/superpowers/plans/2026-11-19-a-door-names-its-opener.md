# Plan: a door's line in the Story names the character that opened or closed it

- **Ticket:** `docs/superpowers/specs/2026-11-19-a-door-names-its-opener-design.md`.
- **Verdict:** passes, with gaps (below).
  - Verified 2026-11-19 by an agent that did not write the ticket, against
    `b022434` on `feat/a-door-names-its-opener` in the worktree
    `vtt-platform-visibility`, with the ticket untracked.
  - Nothing in the repository was edited but this file. `task check`,
    `task e2e`, Stryker and `task build:client` were not run. M1 ran in the
    worktree, its `git status` the same before and after; M2 to M5 ran in a
    copy of the tree at `b022434` outside the repository; the rest are
    searches and readings.
- **To be committed as:** `docs/superpowers/plans/2026-11-19-a-door-names-its-opener.md`.
- **Base:** `b022434`. **Branch:** `feat/a-door-names-its-opener`.
- **Owner's rulings taken in:**
  - 2026-11-16: "om tobbe öppnar en dörr, så ska det stå 'tobbe öppnade
    dörren'"; a door is an event in the game that carries the story on.
  - 2026-11-19: the opener is the CHARACTER (an actor id), never the
    participant, and closing is worded the same way. A player with several
    characters opens as the one they watch, the one a board click moves;
    the door tool offers a door only when that character's token stands
    beside it. The DM and the agent may name any actor the world holds, or
    none; the DM console sends none. A player or spectator that did not see
    the opener before the event gets no line, by the rule `classify` applies
    to `AbilityUsed` (`saw`), the door's state reaching it by
    `doorTransitions`' correction, which carries no event id and so draws no
    Story line (VTT-820). No other projection rule.
  - 2026-11-19, at sign-off: an actor the world does not hold is refused by
    the gateway in plain words (OD-1 B); a check that a player's named
    character is theirs or stands beside the door is not built, "detta är
    ett teoretiskt fall, vi bygger inte kod/kontroller för det", only a
    command sent by hand naming another (D6); an e2e spec (OD-3 A); one
    ticket (OD-5 A).
  - 2026-09-04: nobody uses the product; no migration.
  - Standing: no counts or measured figures in anything written down; the
    client's full mutation run on the owner's word; push and merge on the
    owner's word.
- **Signed off:** 2026-11-19 by the owner: OD-1 B, OD-3 A, OD-5 A; OD-2
  overtaken by the ruling above; OD-4 A taken by the writer, a records
  question.
- **Ids,** allocated by `requirement-id` at Task 0: R1 VTT-866, R4
  VTT-867, R13 VTT-868, R5 VTT-869, R6 VTT-870, R7 VTT-871, R8 VTT-872, R9
  VTT-873; R10 split at the sort into VTT-874 (the click) and VTT-875 (the
  toggle); R11 VTT-876; R12 split into VTT-877 (the schema) and VTT-878 (the
  descriptions). VTT-819 is withdrawn in Task 3, whose code makes it false,
  succeeded by VTT-871.

## The verdict, check by check

1. **Every path resolves. By command.** Every file the ticket names exists,
   and every symbol is where it says, with one exception (G3):
   `OpenDoor` and `CloseDoor` in `contract/vtt/v1/commands.proto`,
   `DoorOpened` and `DoorClosed` in `contract/vtt/v1/events.proto`, each
   carrying `scene_id` and `at` alone (`go doc` of `vttv1.OpenDoor`);
   `ToEvent`'s two door arms in `internal/gateway/convert.go`; `describe`
   ("a door opened at x,y") and `tell` ("a door opened") in
   `client/src/view/spectator.ts`; `playerRules` and `mayWorkDoor` in
   `internal/gateway/authz.go`; `mayWorkDoor` and `doorCommandFor` in
   `client/src/view/doors.ts`; `openDoor` and `closeDoor` in
   `client/src/commands.ts`; `onCell` in `client/src/app.ts`; the
   `open_door` and `close_door` entries of `manifest` in
   `tools/toolgen/main.go`; the door arms of `engine.Apply`; `openDoor` and
   `closeDoor` in `internal/eventgen/model.go`; `classify`'s door arms and
   `doorTransitions` in `internal/perceive/projection.go`;
   `cmd/vtt/webdist`; `docs/requirements.md` with VTT-147, VTT-149,
   VTT-212 and VTT-819; the specifications listed. In
   `~/dev/RPTool/maptool`: `ImpersonateMacro` (its `canImpersonate` reads
   `ServerPolicy.isRestrictedImpersonation` and then `Token.isOwner`),
   `CommandPanel.TokenIdentity`, and no door in the core (doors reach it
   only as vision-blocking lines, `DungeonDraftImporter`).
2. **"Done" is an observation. By command for items 1 to 4 and 7; 5 and 6
   by reading; 8 is the gate.** The ticket names no existing test, so none
   could be run red by name. Items 1 to 3 are false today because
   `OpenDoor`, `CloseDoor`, `DoorOpened` and `DoorClosed` carry no actor
   (`go doc`), and item 7 because neither tool's schema in
   `contract/gen/tools/tools.json` has one; item 2 is false today because
   no command names an actor for the gateway to find absent; item 4's
   mechanism was probed (M3); items 5 and 6 are false today by reading
   `tell`, `describe` and `doorCommandFor`, which never name an actor.
3. **Each rule is breakable. By reading.** Each of the ticket's rules has an
   observation that fails (the "Red under" column below), with one
   reservation: its fifth rule's "otherwise it learns the door's state
   without an event id" reads as every viewer that did not see the opener,
   which is false for a viewer that does not see the square and must stay
   false (VTT-212; blueprint 17). Item 4 says it rightly: "one that sees the
   square". R6 carries item 4's words (G2).
4. **The scope matches the claim. By command, then reading.**
   - Callers, by `git grep`: the Go `mayWorkDoor` is called by `playerRules`
     alone; `doorTransitions` by `transitions` alone, which `Event` and
     `Perch` call; `classify` by `Event`; the client's `mayWorkDoor` by
     `doorCommandFor` and by `mayWorkAnyDoor` in `client/src/view/player.ts`;
     `doorCommandFor` by `onCell` in `client/src/app.ts`; `openDoor` and
     `closeDoor` by `doorCommandFor`; `describe` by `tell` and the Events
     panel, `tell` by the Story; `moveCommandFor` by `onCell` and
     `playerClickFor`.
   - The work reaches files the ticket does not name (G4):
     `client/src/view/player.ts` (`mayWorkAnyDoor`, and `moveCommandFor`'s
     choice of actor, which the ruling makes the door's), `client/src/fold.ts`
     (blueprint 11: an entry refused where the record is kept is refused by
     every reader), `contract/testdata` (round-trip fixtures and
     `expected_tools.json`), `contract/roundtrip_test.go`,
     `contract/events.test.ts`, `cmd/vtt/tools.json`, and, under OD-3 A,
     `client/e2e/setup.ts`. The gateway's existence check sits in
     `handleCommand`'s door gate (D5).
5. **No recorded decision is contradicted silently. By reading.**
   - SPEC-013's door gate gains the check that a named actor exists (D5);
     its `mayWorkDoor` paragraph ("a token on the command's scene, whose
     actor the player `controls`") stands (D6); the ticket names SPEC-013.
   - SPEC-016's "It forwards `DoorOpened` and `DoorClosed` when
     `canSeeSquare` finds the square in the look" and "The square of the
     door event being projected is recorded and sent no frame, since
     `classify` forwards that event when its square is visible" are both
     overturned; named.
   - SPEC-022's `tell`, "which words a door 'a door opened' or 'a door
     closed', naming no square (VTT-819)", is overturned for a door naming an
     actor; named.
   - The first row of SPEC-018's second table ("`engine.Apply` refuses a value that names
     nothing it has folded") gains the two new fields; named.
   - VTT-717 settled that an absent actor named by the DM in a `give_ability`
     is "the fold's to refuse" (`giveSheetFault`); the owner's answer to
     OD-1 puts a gateway check in front of the fold's refusal (D5).
   - The 2026-11-16 ticket, `2026-11-16-the-story-writes-nothing-from-the-sheet-design.md`,
     recorded that no door command or event carried an opener; this is the
     ticket it foresaw.
   - Blueprint 10 (whoever is sent an act is sent who did it, as their
     character knows them) is served by the window; 17 (an act outside
     perception reaches the player as its effect, without its cause) holds
     by R6; 13 holds by R13's plain words. For a command sent by hand,
     outside the window, two are not held: 10, whose Effect says nobody
     speaks as a character they do not play, since such a command may
     name any actor the world holds and the Story then names it; and 20
     (an action at what the character does not perceive is answered as if
     it did not exist), since R13 refuses an absent actor and the player
     rule accepts a present one. The owner ruled that case theoretical at
     sign-off (D6).
6. **The records the work moves are named, and reachable. By command, then
   reading.** The section is present, its paths resolve, nothing is
   `New:`. By search over `docs/specifications/` for `door`, `DoorOpened`,
   `open_door`, `mayWorkDoor` and `toolgen`: SPEC-013, SPEC-016, SPEC-018 and
   SPEC-022 are right, and SPEC-007 is missing. SPEC-007 records each field
   added to the contract and each tool's text ("Whom a line speaks as",
   "Whom a whisper is addressed to"), and this adds a field to `OpenDoor`,
   `CloseDoor`, `DoorOpened` and `DoorClosed` and changes `open_door` and
   `close_door` (G1, OD-4). SPEC-015 (who a player watches), SPEC-021 (each
   party member's log) and SPEC-014 (loading a map) describe nothing this
   changes.

**Split: one ticket,** the owner's answer to OD-5. The gateway requires no
actor of a player (D6), so each commit leaves a working table whatever the
order of Tasks 4 and 5; they stay as numbered.

## The gaps this plan inherits

G1 to G4 were amended in the ticket by its writer after this verdict and
before Task 0's commit: SPEC-007 named, the fifth rule split in two with
item 4's words, the citation corrected, and the files G4 lists named.

- **G1. SPEC-007 is moved and not named.** OD-4.
- **G2. The ticket's fifth rule overclaims.** "Otherwise it learns the
  door's state without an event id" would hand a door's state to a viewer
  that does not see its square. Item 4 is right; R6 takes item 4's words.
- **G3. One false citation.** The problem places `doorCommandFor` in
  `client/src/commands.ts`; `git grep -n "function doorCommandFor" -- client/src`
  finds it in `client/src/view/doors.ts` alone, as "What it touches" says.
- **G4. Files and tests the ticket does not name** (check 4), and the tests
  that change (below).
- **G5. A DM's or an agent's absent actor.** Answered OD-1 B: the door
  gate refuses it in plain words (D5), the fold refusing it too (D3).
- **G6. No test classifies every string field of every event against
  SPEC-018's tables.** Searched: no Go, TypeScript or Python test walks the
  events' string fields against the specification; the closest,
  `TestEveryEnvelopeFieldIsKeptOrClearedByForwardable`, walks the
  envelope's own fields. The new row is held by R1's tests and by reading.
- **G7. How `<actor>` is named** is not said. D8: as the window names an
  actor (VTT-475), so a player reads the token's name, else a party
  member's name, else "a creature", and never an id or a participant.
- **G8. The ticket's "What could not be established" travels:** the first
  is OD-1, the third OD-3; the second is answered by M6.

## Measurements this plan stands on

- **M1. The base is green.** In the worktree, `go test` over
  `./internal/perceive/`, `./internal/engine/`, `./internal/gateway/`,
  `./tools/toolgen/`, `./contract/` and `./internal/eventgen/` passed, and
  `env -u CLAUDECODE bun test` over `doors.test.ts`,
  `spectator-view.test.ts`, `commands.test.ts`, `player.test.ts`,
  `feed.test.ts`, `fold-rejections.test.ts`, `fold-parity.test.ts`,
  `dm-view.test.ts` and `command-surface.test.ts` passed.
- **M2. The field alone turns one test red.** In a copy at `b022434` with
  `string actor_id = 3` added to `OpenDoor`, `CloseDoor`, `DoorOpened` and
  `DoorClosed` and `generate:contract`'s steps run (`buf lint`,
  `buf generate`, toolgen, the copy to `cmd/vtt/tools.json`): `go build ./...` and
  `bunx tsc --noEmit -p client/tsconfig.json` passed; the generated
  TypeScript gives each message `actorId: string`; toolgen put `actorId` in
  both tools' `required`. `TestToolsMatchGolden` turned red, and nothing
  else in `./contract/`, `./tools/toolgen/`, `./internal/engine/`,
  `./internal/perceive/`, `./internal/eventgen/`, `./internal/mcp/`,
  `./internal/campaign/`, `./internal/harness/`, the gateway's `ToEvent`,
  `Converts`, door and `Authorize` tests, or
  `bun test client/test contract/`. Nothing turns red if `ToEvent` drops
  the field: `TestEveryClientCommandConverts` sets the oneof arm alone, and
  the door tests of `convert_test.go` read the scene and the square.
- **M3. Withholding a door event needs the skip made conditional.** In the
  copy, with `classify`'s door arms also asking `saw` of the actor, a
  probe on `twoRooms` (the hero's room sees the door square, not the
  goblin behind it) projected a `DoorOpened` carrying an event id:
  - naming the goblin, with the skip as it stands: the hero was sent no door
    frame, and its fold kept the door closed while the world's was open;
  - naming the goblin, with `Event` handing `transitions` the event as its
    cause only when `classify` forwarded it, and nil otherwise: the hero was
    sent one `DoorOpened` with no event id and folded the door open;
  - naming the hero, or none: forwarded once, with its event id, under both.
  `./internal/perceive/`, `./internal/campaign/` and `./internal/harness/`
  passed under both, with M5's fold and walk in place: no existing test
  sees the wrong belief, so the new test of Task 2 alone holds it.
  `doorSubject(nil)` reports no square, so a nil cause changes nothing for
  any event but a withheld door event.
- **M4. A player rule on the named actor would turn these gateway tests
  red, each for naming no actor;** moot under D6, the rule being today's. In the copy, with `mayWorkDoor` reading the named actor alone
  (its text unchanged) and `ToEvent` copying the field:
  `TestAuthorizeTableAllCommandsAllRoles` (its `open_door/player` and
  `close_door/player` cells), `TestAuthorizePlayerMayWorkAdjacentDoor`,
  `TestAuthorizePlayerMayWorkAdjacentDoorAwayFromTheOrigin`,
  `TestAuthorizePlayerMayWorkDiagonallyAdjacentDoor`,
  `TestAuthorizePlayerMayNotWorkADoorTwoSquaresOffOnOneAxis` (its
  fixture's accepted control), `TestOnlyADoorOpensOrClosesForEveryRole`
  (its player cases), `TestAPlayerFarFromASquareIsToldTheSameWhetherItIsADoor`
  (its beside-a-wall control), `TestQADSAPlayerBesideASquareThatIsNotADoorGetsTheDoorText`,
  `TestQADSAPlayerBesideADoorWorksItOpenOrClosed` and
  `TestAPlayerMayOnlyWorkADoorTheyAreNextTo`. `cmd/vtt`'s scenario, golden
  and corpus tests stayed green: `scenarios/door-watch.json`'s door
  commands are the DM's, naming none, and protojson omits an empty
  `actor_id`, so `scenarios/goldens` do not move.
- **M5. The fold and the walk.** In the copy, with `engine.Apply`'s door
  arms refusing a non-empty `actor_id` the state does not hold, after the
  scene and the position, and `eventgen`'s door actions naming, on even
  indices, the model's actor at the index modulo the model's actors,
  `./internal/eventgen/`, `./internal/engine/`, `./internal/perceive/`,
  `./internal/campaign/` and `./internal/harness/` passed.
- **M6. The set a player watches never holds a character they do not
  control while their window can act** (the ticket's second open point).
  `watchable` lists what `MayWatch` accepts, and for a player who controls a
  party member `MayWatch` accepts only party members they `controls`
  (`ownOnly`). A seat re-resolves on every wake (`seat.recheck`,
  `seat.resolve`), and a shown actor `MayWatch` refuses gives way to
  `defaultViewpoint`. A player who controls no party member perches
  (`mayPerch`), `canAct` is false and the board has no `onCell`. Only a
  window holding a `ViewpointChanged` older than a revocation can name a
  character no longer theirs; its fold holds the revocation (`classify`
  forwards an `ActorControlRevoked` of an actor it saw, and its own
  character is its eyes), so under D9 the window offers no door.
- **M7. A refusal reaches its issuer.** `campaign.Append` folds before it
  persists and returns the fold's error, and `handleCommand` answers it
  ok=false with that text (SPEC-013's command path; VTT-161, VTT-162).
  `authorize` answers `Authorize`'s error text the same way. The DM
  console never names an actor (the ruling), so an absent actor's refusal
  reaches only an agent, a player's or the DM's raw socket.
- **M8. The comment ceilings.** `python3 tools/check-comments.py --report`
  puts every Go and TypeScript file this plan changes at its ceiling in
  `tools/comment-ceilings.txt` or just under it: `authz.go` (unchanged
  under D6), `projection.go`, `apply.go`, `convert.go`, `model.go`, `doors.ts`,
  `player.ts`, `spectator.ts`, `app.ts`, `commands.ts` and `fold.ts`. A
  prose comment line fits only beside added code or in place of a removed
  one; a line of requirement ids alone counts in no share. (The run left a
  `tools/__pycache__`, removed.)
- **M9. No adjudication sits on a changed line.** By search of
  `tools/mutation-equivalents.txt` and `tools/ts-mutation-equivalents.txt`
  by file: `authz.go`'s entry is on `abs` (`if n < 0 {`), which nothing
  here changes; none is on a door arm of `apply.go` or `fold.ts`,
  `classify`, `Event`, `doorTransitions`, `ToEvent`, `doors.ts`,
  `moveCommandFor`, `tell` or `describe`; `app.ts`'s entries on the `ui`
  literal stand if that line is left as it is.
- **M10. How a tool's text and schema are made.** `toolgen` derives
  `required` from the fields that are not proto3 `optional`, unless the
  manifest's `fieldOverride.requiredOverride` replaces it, and a field's
  description reaches the schema only through `fieldOverride.fieldDocs`.
  `generate:contract` writes `contract/gen/tools/tools.json` and copies it
  to `cmd/vtt/tools.json`, both under `check:drift`;
  `TestToolsMatchGolden` compares `buildTools()` with
  `contract/testdata/expected_tools.json`. The MCP server decodes a tool's
  arguments with protojson into the command's own message
  (`commandFromArgs`), so a new field reaches the gateway with no MCP code.
- **M11. The real wire can be driven.** No spec under `client/e2e` sends a
  door command (search). The e2e server serves `rulesets/dnd45e-minimal`
  and `adventures/` (of which `goblin-ambush` alone fits that ruleset) and
  no map; `vtt serve` reads maps from `<campaign>/maps`, the campaign being
  the directory `setup.ts` passes as `--campaign`. The shipped
  `campaigns/example/maps/cellar.json` holds a door tile (`wood-door`)
  in the wall between two walled rooms, a floor square on each side of it on
  its row free of objects, and a placement `tok-fighter` of `act-fighter`.
  `goblin-ambush`'s ravine places a `tok-fighter` too, and `currentSceneId`
  takes the lexicographically greatest scene id, so a spec that loads the
  adventure and the map collides on the token and boards the ravine. A
  closed door blocks sight (`sight.VisibleFrom`). `asDM` sends a DM's
  commands on a raw socket; `table.spec.ts`'s move test clicks a board
  square from `.grid`'s bounding box.

## Constraints that bind every task

- **P1. CLAUDE.md rule 2.** No assertion is weakened; a test M4 names moves
  by naming the actor its fixture already places beside the square, and
  asserts what it asserted.
- **P2. Rule 3.** The contract change is additive: a new field number on
  each of `OpenDoor`, `CloseDoor`, `DoorOpened` and `DoorClosed`, nothing
  renamed or renumbered. `check:breaking` reports and does not enforce
  (`contract/RELEASED` is absent); the rule binds anyway.
- **P3. Rule 4.** `engine.Apply` stays the one fold; the actor check is a
  refusal in its door arms, and `fold.ts` mirrors it.
- **P4. Rule 5.** Platform code names actors, doors and squares; no
  game-system word ("hp" is refused by semgrep).
- **P5. Rules 8 and 10.** Names, never line numbers. A comment under
  `internal/`, `cmd/` or `client/src` is a warning or a pointer; a comment a
  change makes false is corrected or removed in the commit that makes it
  false (D13); M8 binds what may be added.
- **P6. Rule 9,** answered below.
- **P7. Blueprint 10, 11, 13, 17, 33, 34.** Who opened is sent as the
  viewer's character knows them; both folds refuse alike; refusals in the
  table's words (D5); the effect without the cause; one description for
  every window and tool; a tool's text true when it lands. Blueprint 20 is
  not held for a command sent by hand (check 5, D6).
- **P8. SPEC-020 and SPEC-023.** The client's stored mutation report goes
  out of date with the first client commit; `task check` names that and
  passes; a full run is the owner's to ask for.
- **P9. The mechanical gates.** A commit that changes `client/src` carries
  `cmd/vtt/webdist` rebuilt by `task build:client`; a commit that changes a
  `.proto` or `tools/toolgen` carries `contract/gen` and `cmd/vtt/tools.json`
  regenerated by `task generate:contract` (`check:drift`), and
  `contract/testdata/expected_tools.json`, which no task writes, brought to
  the generated schema; bun under `env -u CLAUDECODE`;
  `task client:typecheck`; `python3 tools/check-new-prose.py` and
  `python3 tools/check-comments.py` before each commit; stage and commit in
  separate calls; a client test reads no file outside `contract/gen` and
  `contract/testdata`; Go commands started with `/bin/sh`, never zsh. Each
  task gets Phase 4a's QA and one reviewer before its commit.

## Rule 9: how MapTool does this

The ticket's paragraph holds against `~/dev/RPTool/maptool`:
- MapTool's core has no door. A door is drawn geometry that blocks vision
  (`DungeonDraftImporter` imports a map's portals as vision-blocking
  lines), and nothing in the core opens one or says who did.
- What a macro says in the chat speaks as the token the user impersonates:
  `ImpersonateMacro` sets `CommandPanel`'s `TokenIdentity`, and its
  `canImpersonate` lets anyone impersonate any token unless
  `ServerPolicy.isRestrictedImpersonation` is set, then a GM any token and a
  player only one whose owner they are (`Token.isOwner`).
- **Borrowed:** the act is spoken as the character the player is acting as,
  and the GM free to speak as anyone or as nobody.
- **Not borrowed:** MapTool's identity is a chat setting chosen apart from
  the board; here the opener is the character the board click moves (the
  ruling). Nor the restricted policy's server check of whom a player may
  name: the window offers only the character the player watches, and the
  gateway asks only that a named actor exists (D5, D6).
- **Our own:** that a viewer that did not see the opener is told nothing of
  them. MapTool sends every client the whole chat; its answer to who sees
  what is not ours (CLAUDE.md rule 9's own warning).

## Decisions

1. **D1. The field.** `string actor_id = 3` on `OpenDoor`, `CloseDoor`,
   `DoorOpened` and `DoorClosed`, not proto3 `optional`; empty is none, on
   the command and on the event alike. Each message's proto comment gains
   one line saying so and naming SPEC-013.
   - **Forced by** P2; `AddNarration.actor_id` and
     `AddWhisper.from_actor_id`, plain fields an empty value of which means
     none (SPEC-007, `TestQANarrNewFieldsAreNotProto3Optional`); an
     `optional` event field would split absent from empty for nothing.
2. **D2. The tools leave the actor optional from the start, and say who may
   name one when that is true.** Task 1 gives both manifest entries
   `requiredOverride: ["sceneId", "at"]`; Task 6 adds `fieldDocs` for
   `actorId` and rewrites the two descriptions.
   - **Forced by** M2 (the derived list requires `actorId`, telling the DM
     and the agent they must name one) and blueprint 34 (the descriptions
     state the refusal's plain words, which are the gateway's from Task 5).
3. **D3. Both folds refuse an absent opener, last.** `engine.Apply`'s door
   arms check, after the scene and the position, that a non-empty
   `actor_id` names an actor the state holds, refusing as `engine: door
   opened names unknown actor "<id>"` and `engine: door closed names
   unknown actor "<id>"`; `fold.ts`'s `doorOpened` and `doorClosed` arms do
   the same through `st.Actors`, a map with no prototype (`emptyMap`),
   throwing `door opened names unknown actor "<id>"` and its twin.
   - **Forced by** item 1, blueprint 11, and the whisper arms' form
     (VTT-586) in both folds; checking last leaves every refusal the arms
     give today as it is.
4. **D4. `ToEvent` copies `actor_id`** in both door arms, and VTT-503's
   `TestToEventOpenDoorProducesDoorOpened` and
   `TestToEventCloseDoorProducesDoorClosed` send a command naming an actor
   and assert it on the event.
   - **Forced by** M2: nothing else turns red when the copy is dropped.
5. **D5. The door gate refuses an absent actor, for every role** (OD-1 B).
   In `handleCommand`, after `IsDoor` accepts the square, a door command
   whose `actor_id` is not empty and names no actor in the snapshot's
   `Actors` is answered ok=false with `There is no such character.`, and
   nothing is appended. `doorSquare` reports the actor beside the scene and
   the square. It runs after `Authorize`, so a player's command
   `mayWorkDoor` refuses keeps that text, and after the door gate, so a
   square that is not a door keeps `There is no door there.`
   - **Forced by** the owner's answer to OD-1, and the door gate being
     where a door command's form is checked against the snapshot for every
     role (VTT-608); the fold's own refusal (D3) stays behind it.
6. **D6. The player rule stays as it is.** `mayWorkDoor` and its text are
   unchanged (VTT-149, VTT-609): a player works a door when a token on its
   scene whose actor they control stands beside it. The actor a player's
   command names is checked for nothing but D5's existence, so a command
   sent by hand may name another character, one of theirs elsewhere, or
   one their character does not see, and the event names it.
   - **Forced by** the owner's ruling at sign-off: "detta är ett teoretiskt
     fall, vi bygger inte kod/kontroller för det". The window names only
     the character the player watches, offered only when it stands beside
     the door (D9).
7. **D7. The projection.** `classify`'s door arms forward when
   `canSeeSquare` finds the square in the look and `saw` the event's actor;
   `saw` passes an empty id, so a door event naming none is ruled as
   today. `Event` hands
   `transitions` the event as its cause only when `classify` forwarded it,
   and nil otherwise, so `doorTransitions` skips the square only of a door
   event the viewer is sent. `Perch` is unchanged.
   - **Forced by** the ruling (`AbilityUsed`'s `saw`), and M3: with the skip
     unconditional, a viewer that sees the square and not the opener keeps a
     wrong belief (VTT-213).
8. **D8. The lines.** `tell`'s door arms read `${names.actor(id)} opened a
   door` and `${names.actor(id)} closed a door` when the payload names an
   actor, and today's words when it names none; `describe`'s read the same
   followed by ` at x,y`.
   - **Forced by** item 5 and VTT-475 (an actor named as the window names
     it, never by an id; a participant is never an actor's name).
9. **D9. The window's opener is the actor a board click moves.** One
   exported function in `client/src/view/player.ts` answers it:
   `ui.selectedActorId`, else the first actor the participant controls by
   id, as `moveCommandFor` chooses today; `moveCommandFor` calls it. The
   client's `mayWorkDoor(st, me, actorId, cell)` answers, for a player,
   whether that actor is controlled by `me` and has a token on the current
   scene within one square on each axis, for the DM and the agent true, for
   every other role false; `doorCommandFor(st, me, armed, cell, actorId)`
   names `actorId` for a player and none for the DM and the agent;
   `mayWorkAnyDoor` takes the same actor; `onCell` and `renderPlayerPanel`
   hand them that function's answer.
   - **Forced by** the ruling ("the one they watch, the one a board click
     moves"; `ui.selectedActorId` follows `session.viewpoint` in a player's
     `paint`), and by the offer and the send having to agree: one choice
     feeds both. Before a player's first `ViewpointChanged`, the panel's
     heading may name another actor than the door; the door follows the
     move, as the ruling words it.
10. **D10. The builders take an optional actor.** `openDoor(sceneId, at,
    actorId?)` and `closeDoor(...)` leave it empty when not given, which
    protojson omits, so the DM's door command keeps today's wire shape.
    - **Forced by** item 6's DM half and the commands file's own protojson
      note (empty scalars are omitted).
11. **D11. The opener's JSON name is a cross-language claim.** Task 1 adds
    `contract/testdata/open_door_command.json` and
    `contract/testdata/door_opened_envelope.json`, each naming an actor,
    round-tripped by `contract/roundtrip_test.go` and
    `contract/events.test.ts`; Task 4 asserts `openDoor` naming an actor
    against the command fixture in `client/test/commands.test.ts`, and
    corrects the commands file's header, which puts `openDoor` and
    `closeDoor` among the builders held by a hand-written literal.
    - **Forced by** the actor becoming the field the Story names a
      player's character by: a builder whose shape the server parses
      otherwise names nobody, while today the two builders' shapes are
      held only by a literal the client's own author wrote (the commands
      file's header says so); `contract/testdata` is hashed into the
      client's mutation inputs (`ts-mutation-inputs.py`).
12. **D12. The walk names openers.** `eventgen`'s `openDoor` and
    `closeDoor` name, on even indices, the actor at the index modulo the
    model's actors when it holds any, and none otherwise; `closeDoor` takes
    the index. No new `rng` draw, and no absent actor drawn.
    - **Forced by** `Step`'s rule that an action keeps its count of draws,
      `openDoor`'s own warning that a guard nobody trips is untested, and
      M5; the character-log oracle then compares door frames carrying an
      actor through each log.
13. **D13. Comments follow the code.** `doors.ts`'s header and its
    `mayWorkDoor` and `doorCommandFor` docs, `doorTransitions`' doc
    ("classify forwards that event"),
    `Event`'s new branch, and `tell`'s pointer (VTT-819) are made true or
    removed in the task that changes the code beneath them; a new line
    there is a warning or a pointer, by requirement id where a ceiling binds
    (M8).
    - **Forced by** rule 10 and M8.
14. **D14. Every client commit carries its bundle** (P9), Tasks 1, 3 and 4.
15. **D15. The real wire** (OD-3 A): `client/e2e/doors.spec.ts`, its own
    table, whose campaign directory `setup.ts` gives a copy of
    `campaigns/example/maps/cellar.json` under `maps/` before the server
    starts, for that table alone.
    - The spec reads the door's square and its two neighbours on its row
      from the map file, writing no coordinate of its own, and asserts each
      neighbour is floor with no object.
    - Through `asDM`, the DM starts a session; adds `act-fighter` (the map's
      placement), a party member with a name no participant has (here
      "Brannoc") and a creature, each with the properties
      `dnd45e-minimal` requires, built as `conditions.spec.ts` builds its
      actors; loads the map, and no adventure (M11); grants the party member
      to the fixture's player; places it on the door's west neighbour and the
      creature on its east; then opens the door naming the creature.
    - The player's page, watching the party member, shows the creature's
      token and no Story line holding "opened a door".
    - The player arms doors and clicks the door square (as `table.spec.ts`'s
      move test clicks a square): the player's Story reads
      "Brannoc closed a door" and no line names the participant.
    - The DM's page reads, in its Story, "<the creature as the DM's window
      names it> opened a door" and "Brannoc closed a door", and in its
      Events panel the same with " at x,y".
    - `afterAll` ends the session.
    - **Forced by** M11 (no spec works a door, and the opener's JSON name
      is otherwise held by fixtures alone) and blueprint 33.
16. **D16. The register.** R1, R4 to R13 under the sort at Task 0; VTT-819 is
    withdrawn and succeeded by R7; every other row the work reaches stands
    (below), its evidence cells following the tests that move.
17. **D17. The specifications last,** from the report: SPEC-013, SPEC-016,
    SPEC-018, SPEC-022 and, under OD-4 A, SPEC-007.

## Open decisions for the owner

The owner's answers, 2026-11-19: OD-1 B, with the text `There is no such
character.`; OD-2 overtaken, no player check being built (D6), so
`mayWorkDoor`'s text stays; OD-3 A; OD-4 A, taken by the writer as a
records question; OD-5 A.

- **OD-1. An actor the world does not hold, named by the DM or the agent.**
  - **A (recommended).** The fold refuses it (D3, D5): ok=false with
    `engine: door opened names unknown actor "<id>"`, nothing appended. As
    VTT-717 does for a give; no code beyond the fold's; read only by an agent
    or a raw socket, since the console names no actor.
  - **B.** A gateway check for the DM and the agent before the fold, as
    `mayVoice` runs for every role, refusing with a `plainRefusal` that names
    no package and no id ("There is no such character."). One more check
    and one more row.
- **OD-2. The words of a player's refusal** (D6; one text for every cause).
  - **A (recommended).** A `plainRefusal`, naming no package and no id:
    "You open or close a door as a character of yours standing beside it."
    True for every cause (no actor, absent, not theirs, not beside), as the
    move gate's and the door gate's are (VTT-601, VTT-608); SPEC-013's
    principle 13 then names it.
  - **B.** Today's text, `gateway: not authorized: participant "<id>" has no
    token adjacent to that door`. Still one text for each player, but it
    names a package and an id, and it misreads a command naming no actor.
- **OD-3. A check on the real wire** (the ticket's third open point).
  - **A (recommended).** D15: `client/e2e/doors.spec.ts` on the shipped
    cellar map, deterministic by a closed door between two walled rooms.
  - **B.** No e2e check. The fixtures of D11 hold the opener's JSON name; no
    test then drives a door through a browser and a server together.
- **OD-4. SPEC-007** (G1).
  - **A (recommended).** It gains a section beside "Whom a whisper is
    addressed to": the door commands' and events' `actor_id` (field 3),
    empty for none, not proto3 `optional`, the event copying the command's,
    and what the two tools require and say; its Status names the new
    round-trip tests and the tools' test.
  - **B.** Left as it is; SPEC-007 describes no door message today.
- **OD-5. One ticket** (the ticket asks).
  - **A (recommended).** One ticket, the tasks below its commits (the split
    paragraph above).
  - **B.** Two: the contract, the folds, the projection and the lines first;
    the window and the player rule together second.

## Requirements

Sorted and allocated at Task 0 with the `requirements` skill, under the
signed-off answers; the register's words are the ones that bind. Every new
row reads `**OPEN — no test yet**` until its task cites its tests.

| R | Item | Rule (as drafted; the register's row binds) | Red under |
|---|---|---|---|
| R1 | 1 | `engine.Apply` and the client's fold refuse a `DoorOpened` or a `DoorClosed` whose `actor_id` is not empty and names no actor they hold, after its scene and its position, so the command that would append it appends nothing; an empty `actor_id` folds as it did. | an absent id folded; an empty id refused; the two folds apart on an id such as `hasOwnProperty`; an unknown scene answered with the actor's text |
| R13 | 2 | An `open_door` or `close_door` that `Authorize` accepts at a door square, naming an actor the state does not hold, is refused with `There is no such character.`, whatever the issuer's role, and appends nothing. | accepted; another text, or one naming a package or the id; a square that is not a door answered with this text; anything appended |
| R4 | 3 | A DM's or an agent's `open_door` or `close_door` at a door square, naming an actor the state holds or none, is accepted wherever that actor's tokens stand, a player's character and an actor with no token included, and the event names that actor or none. | refused for an actor far away, with no token or a player's; the id dropped |
| R5 | 4 | A player's or a spectator's log is sent a `DoorOpened` or a `DoorClosed` that names an actor, with its event id, only when its eyes saw that actor before the event. | forwarded to eyes that come to see the opener by the opening itself, or never see it |
| R6 | 4 | Eyes that see a door's square, and are not forwarded its door event, are sent the door's new state once, in a frame that carries no event id. | the log keeping the old state; the frame carrying an event id; two frames |
| R7 | 5 | The Story draws a `doorOpened` that carries an event id as "<actor> opened a door" when it names an actor, the actor named as the window names it, and as "a door opened" when it names none, and a `doorClosed` alike with "closed". Succeeds VTT-819. | an id, a participant's name or the square in the line; the no-actor line changed |
| R8 | 5 | The Events panel words a door event as the Story's words for it would read, then " at " and its square as "x,y". | the square or the actor missing; the panel naming the actor otherwise than the Story |
| R9 | 6 | With doors armed in a player's window, a click on a door square of the current scene sends `open_door` or `close_door` naming the actor a board click moves. | none named; the first controlled actor named while another is watched; the participant's id |
| R10 | 6 | A player's window offers the arm-doors toggle, and with doors armed sends a door command, only for a door of the current scene at most one square on each axis from a token of the actor a board click moves. | offered or sent because another character of theirs stands beside |
| R11 | 6 | A DM's or an agent's window's door command names no actor. | an actor named |
| R12 | 7 | The `open_door` and `close_door` tools take `actorId` and do not require it, and their descriptions say it names the actor that opens or closes the door, any actor the campaign holds, or none, and that an actor the campaign does not hold is refused. | required; absent from the schema; a description saying the actor must stand beside the door |

**Candidates the sort may refuse:**
- **A row for the event copying the command's actor.** VTT-503 says it of
  every field the event shares with its command; refused, its door tests
  extended (D4).
- **R2 and R3, a player's door command checked on the actor it names.**
  Refused at sign-off: the owner ruled the case theoretical (D6).
- **VTT-149 and VTT-609 withdrawn.** Refused: the player rule is today's
  (D6), true as worded.

**Withdrawn:** VTT-819, "False as worded once a door event naming an actor
is drawn naming it. Succeeded by R7."

**Rows checked and left standing,** their evidence cells following any test
renamed:
- **VTT-147** (the DM and the agent work a door from anywhere); R4 says
  what they may name.
- **VTT-149** and **VTT-609** (above).
- **VTT-212** (forwarded only when its square is in sight): R5 adds a
  condition, and "only when" stays true.
- **VTT-213** (a door a viewer sees is shown as the table has it) and
  **VTT-214** (reaches it once): D7 is what keeps both true; R6's tests cite
  them.
- **VTT-503**, **VTT-608**, **VTT-820** (no line for a door with no event
  id, which R6 rests on), **VTT-822**, **VTT-460**, **VTT-408**, **VTT-441**
  (D9 keeps `moveCommandFor`'s choice), **VTT-161**, **VTT-162**,
  **VTT-506** and **VTT-589**.

## The existing tests that change

- **Task 1:** `TestToEventOpenDoorProducesDoorOpened` and
  `TestToEventCloseDoorProducesDoorClosed` name an actor and assert it
  (D4); `TestToolsMatchGolden`'s golden, `contract/testdata/expected_tools.json`,
  takes the regenerated schema (M2).
- **Task 4, by reading, measured before its commit:** in
  `client/test/doors.test.ts`, every test calling `mayWorkDoor` or
  `doorCommandFor` passes the actor its fixture places (`a1`, or for the
  DM's tests the empty id), asserting what it asserted.
- **Task 5:** none; the tests M4 names stand unchanged, the player rule
  being today's (D6).

## Tasks

### Task 0: the records, the ids

- **Do:** after sign-off, run the `requirements` sort over R1 and R4 to
  R13 and the candidates, as the answers leave them; allocate ids with
  `requirement-id`; commit the ticket, this plan and `docs/requirements.md`.
- **True when:** `git status` is clean and `task check:requirements-chain`
  passes.

### Task 1: the contract and the folds (R1; D1 to D4, D11's fixtures, D12; item 1)

- **Files:** `contract/vtt/v1/commands.proto`,
  `contract/vtt/v1/events.proto`; `contract/gen/**` and
  `cmd/vtt/tools.json` by `task generate:contract`; `tools/toolgen/main.go`
  (`requiredOverride` alone) and a test in `tools/toolgen/main_test.go`
  that both tools offer `actorId` and do not require it;
  `contract/testdata/expected_tools.json`;
  `contract/testdata/open_door_command.json`,
  `contract/testdata/door_opened_envelope.json`,
  `contract/roundtrip_test.go`, `contract/events.test.ts`;
  `internal/engine/apply.go` and a new test file in `internal/engine`;
  `client/src/fold.ts` and `client/test/fold-rejections.test.ts`;
  `internal/gateway/convert.go` and `internal/gateway/convert_test.go`;
  `internal/eventgen/model.go`; `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **Tests first, seen red:** both folds refuse a `DoorOpened` and a
  `DoorClosed` naming `ghost`, and `hasOwnProperty` in the client's fold,
  with the texts D3 gives; fold one naming an actor they hold and one naming
  none; an event naming an absent actor in an unknown scene is refused with
  the scene's text; the round trips; the conversion tests (D4); the tools'
  schema.
- **True when:** the new tests pass and failed before; `go test` over
  `./contract/ ./internal/engine/ ./internal/gateway/ ./internal/eventgen/
  ./internal/perceive/ ./internal/campaign/ ./internal/harness/
  ./internal/mcp/ ./tools/toolgen/ ./cmd/vtt/` passes; `env -u CLAUDECODE bun test client/test
  contract/` and `task client:typecheck` pass; `scenarios/goldens` is
  unchanged; P9's checks pass.
- **Deliberate breaks:**
  - Drop the actor check from `DoorOpened`'s arm in `engine.Apply`: R1's Go
    test turns red.
  - Check the actor before the scene: the unknown-scene test turns red.
  - Refuse an empty `actor_id`: the no-opener tests turn red, and so do
    `scenarios/door-watch.json`'s goldens.
  - Drop the check from `fold.ts`: R1's client test turns red.
  - Look the actor up with `in` or on a plain object in `fold.ts`: the
    `hasOwnProperty` case turns red.
  - Drop `ActorId` from either arm of `ToEvent`: D4's tests turn red.
  - Remove the `requiredOverride`: the tools' test turns red.
  - Spell the fixture's key `actor_id` or `opener`: both round trips turn
    red.
  - Name an id the model does not hold in `eventgen`: the walks' "landed …
    the model expected" assertions turn red.
- **The state here:** an event can carry its opener and both folds agree on
  it; nothing but an agent sends one; the player rule and the lines are
  today's.

### Task 2: the projection (R5, R6; D7; item 4)

- **Files:** `internal/perceive/projection.go` (`classify`'s door arms,
  `Event`, `doorTransitions`' doc); a new test file in `internal/perceive`;
  `docs/requirements.md`.
- **Tests first, seen red,** on `twoRooms` and through `newViewer`'s fold,
  each event carrying an event id: a `DoorOpened` naming the goblin behind
  the door reaches the hero's eyes as one `DoorOpened` with no event id,
  their fold showing the door open, the goblin introduced; the same for a
  `DoorClosed` naming an actor the eyes do not see; a door event naming
  the hero, or none, is forwarded once with its event id; a door event
  naming an actor the eyes saw, at a square they do not see, sends nothing;
  a spectator's shoulder (`Eyes{Shoulder: …}`) the same as a player's.
- **True when:** the new tests pass and failed before; `go test
  ./internal/perceive/ ./internal/campaign/ ./internal/harness/` passes,
  `TestEveryEnvelopePayloadArmHasAnExplicitRuling`, VTT-212's, VTT-213's and
  VTT-214's tests among them, unchanged; `go-arch-lint` and P9's checks
  pass.
- **Deliberate breaks:**
  - Leave out `saw` in either door arm: R5's test turns red.
  - Ask `now.sees` in place of `saw`: the goblin-behind-the-door test turns
    red (the opening itself brings it into sight).
  - Hand `transitions` the event whatever the verdict: R6's test turns red,
    the hero's fold keeping the door shut (M3).
  - Hand it nil whatever the verdict: VTT-214's
    `TestADoorYouCanSeeDoesReachThePlayer` turns red, the door arriving
    twice.
- **The state here:** a viewer that did not see an opener is sent the door's
  state without a line; nothing but an agent names one yet.

### Task 3: the lines (R7, R8; D8; item 5)

- **Files:** `client/src/view/spectator.ts` (`tell`, `describe`);
  `client/test/spectator-view.test.ts`; `docs/requirements.md` (VTT-819
  withdrawn, succeeded by VTT-871, its tests' citations moved); a rebuilt
  `cmd/vtt/webdist`.
- **Tests first, seen red:** a player's window names the opener by the
  token's name, by the party member's name, and as "a creature", in the
  Story and never with the square; a DM's window by its GM chain, in the
  Story and on the Events panel with " at x,y"; a door event naming none
  reads as today; a line never holds the opener's id or a participant's
  display name; a door event with no event id draws no line, opener or not.
- **True when:** the new tests pass and failed before; the whole client
  suite, `task client:typecheck` and P9's checks pass;
  `qa-what-a-window-shows.test.ts`'s door rows and VTT-820's tests pass
  unchanged.
- **Deliberate breaks:**
  - Name no actor in `tell`: R7's test turns red.
  - Name the actor by `p.value.actorId`: the token-name and "a creature"
    tests turn red.
  - Drop " at x,y" from `describe`'s new arms: R8's test turns red.
  - Read "someone opened a door" for none: the no-opener test turns red.
- **The state here:** a door event naming an opener is told so; only an
  agent sends one.

### Task 4: the window (R9 to R11; D9, D10, D11's client half; item 6)

- **Files:** `client/src/view/player.ts` (the acting actor's function,
  `moveCommandFor`, `mayWorkAnyDoor`, `renderPlayerPanel`'s toggle);
  `client/src/view/doors.ts` (`mayWorkDoor`, `doorCommandFor`, their docs);
  `client/src/app.ts` (`onCell`); `client/src/commands.ts` (`openDoor`,
  `closeDoor`, the header); `client/test/doors.test.ts`,
  `client/test/player.test.ts`, `client/test/app.test.ts`,
  `client/test/commands.test.ts`; `docs/requirements.md`; a rebuilt
  `cmd/vtt/webdist`.
- **Tests first, seen red:** a player controlling two characters, the
  watched one beside a door and the other not, then the other way round:
  the toggle is offered, and an armed click sends a command naming the
  watched one, only while the watched one stands beside; a booted player's
  window, switched by a `ViewpointChanged`, sends the new one; with no
  watched id, the door names the actor a click moves; a DM's and an agent's
  window's armed click sends a door command naming none, wherever its own
  chips point; `openDoor` naming an actor matches
  `contract/testdata/open_door_command.json`, and with none matches today's
  shape.
- **True when:** the new tests pass and failed before; the whole client
  suite, `task client:typecheck`, `ts_mutation_inputs_test.py` and P9's
  checks pass; VTT-408's, VTT-441's and VTT-460's tests pass unchanged.
- **Deliberate breaks:**
  - Name none for a player: R9's test turns red.
  - Name the first controlled actor in place of the watched one: the
    two-character test turns red.
  - Ask any controlled token in `mayWorkDoor`, as today: R10's tests turn
    red.
  - Name the acting actor for the DM: R11's test turns red.
  - Gate the toggle on any controlled token: the toggle's test turns red.
- **The state here:** a player's door click names their character; the
  gateway still accepts by today's rule, and the event names the character.

### Task 5: the gateway (R4, R13; D5; items 2 and 3; OD-1 B)

- **Files:** `internal/gateway/server.go` (the door gate in
  `handleCommand`, `doorSquare`); a new test file in `internal/gateway`;
  `docs/requirements.md`.
- **Tests first, seen red:** the DM and the agent naming an actor far from
  the door, an actor with no token, a player's character, or none:
  accepted, the event naming it or none; the DM, the agent and a player
  beside the door each naming an absent id: refused with `There is no such
  character.`, nothing appended, no other connection sent anything; an
  absent id at a square that is not a door: `There is no door there.`; a
  player far from the door naming an absent id: `mayWorkDoor`'s text.
- **True when:** the new tests pass and failed before; `go test
  ./internal/gateway/ ./cmd/vtt/` passes, `scenarios/goldens` unchanged,
  VTT-149's, VTT-608's and VTT-609's tests unchanged; P9's checks pass.
- **Deliberate breaks:**
  - Drop the check: R13's tests turn red (the fold's text answers).
  - Check before `IsDoor`: the not-a-door test turns red.
  - Check for the DM and the agent alone: the player's absent-id test turns
    red.
  - Name the id in the text: the text test turns red.
- **The state here:** a door command names an actor the world holds, or
  none.

### Task 6: the tools' texts (R12; D2; item 7)

- **Files:** `tools/toolgen/main.go` (`fieldDocs` for `actorId`, the two
  descriptions); `tools/toolgen/main_test.go`; `contract/gen/tools/tools.json`
  and `cmd/vtt/tools.json` by `task generate:contract`;
  `contract/testdata/expected_tools.json`; `docs/requirements.md`.
- **Tests first, seen red:** each description and `actorId`'s doc say what
  R12 says, read as `TestUseAbilitySaysAPlayersTargetMustBeSeen` reads its
  tool; `actorId` stays out of `required`; every sentence is true against
  Task 5's tests.
- **True when:** the new tests pass and failed before; `go test
  ./tools/toolgen/ ./internal/mcp/ ./cmd/vtt/` passes; P9's checks pass;
  every sentence of the two texts is run against Task 5's tests by the
  reviewer (blueprint 34).
- **Deliberate breaks:**
  - Say the actor must stand beside the door: R12's test turns red.
  - Put `actorId` in `requiredOverride`: the schema test turns red.
- **The state here:** an agent is told who may name an opener.

### Task 7: the real wire (D15; OD-3 A)

- **Files:** `client/e2e/doors.spec.ts`; `client/e2e/setup.ts` (the cellar
  map in the door spec's campaign directory alone).
- **True when:** `task e2e` passes, the spec among it; the spec reads the
  square from `campaigns/example/maps/cellar.json`.
- **Deliberate breaks:**
  - Name none in `doorCommandFor` for a player: the spec turns red (the
    Story reads "a door closed").
  - Leave out `saw` in `classify`: the spec turns red (the player's Story
    tells of the creature's opening).
- **The state here:** the whole table, through a browser and a server.

### Task 8: the gate (item 8)

- **Do:** `python3 tools/check-comments.py main`, and `--write-ledger`
  once, last, if a ceiling can come down; check free space and the load
  (`os.getloadavg()`); run `task check` whole on the last code commit,
  detached under `/bin/sh`, output to a file, requiring its completion
  line; run `task e2e`. Tell the owner the client's stored mutation report
  is out of date from Task 1 and which commit it measured (SPEC-023); a
  full Stryker run is his to ask for.
- **True when:** `task check` and `task e2e` are green; the embedded bundle
  is the last client commit's.

### Task 9: the report, then the specifications

- **The report:** `docs/reports/2026-11-19-a-door-names-its-opener.md`, with
  the `implementation-report` skill and its reading review: each "Done looks
  like" item with its observation; the rows and what turned each red; Rule
  9; G1 to G8 and each OD's answer; M3's probe as the reason for D7; Phase
  4a's adjudications under their own heading; the gate.
- **The specifications,** with the `specification` skill, from the report,
  each "## Requirements" list gaining its rows:
  - **SPEC-013:** the door gate's check that a named actor exists, and its
    text (D5); what the DM and the agent may name; that a player's named
    actor is checked for nothing more (D6).
  - **SPEC-016:** the door arms' `saw`; `Event`'s conditional cause and
    `doorTransitions`' skip of a forwarded event's square alone.
  - **SPEC-018:** `DoorOpened.actor_id` and `DoorClosed.actor_id` join the
    row `engine.Apply` holds by refusing what it has not folded.
  - **SPEC-022:** the Story's and the Events panel's door lines; the door
    tool's actor, offer and send.
  - **SPEC-007** (OD-4 A): the section D17 names.
- **True when:** each changed sentence has been run against the code by
  someone other than its writer. Commit the report, then the
  specifications; push and merge on the owner's word.

## For Phase 4a

Hand the QA agent the rows, the specifications as drafted, and the surfaces
as signatures, never the source: `go doc` of `vttv1.OpenDoor`,
`vttv1.CloseDoor`, `vttv1.DoorOpened`, `vttv1.DoorClosed`,
`./internal/engine Apply`, `./internal/gateway Authorize` and `ToEvent`,
`./internal/perceive Projection.Event` and `New`; on the client,
`doorCommandFor`, `mayWorkDoor`, the acting actor's function,
`moveCommandFor`, `describe`, `renderSpectator`, `fold`, `openDoor`,
`closeDoor`, and `boot` with its fetch and socket seams as `app.test.ts`
uses them. Tell it: never run `task check`, `task e2e` or Stryker; its files
are new `qa_*` or `qa-*` test files; client tests under
`env -u CLAUDECODE bun test`; a client test reads no file outside
`contract/gen` and `contract/testdata`; `internal/perceive` imports only the
contract, `engine` and `sight` (`go-arch-lint`); semgrep refuses "hp" in
platform code; fixtures with two characters and two doors, the one asked
for not first; the tree is hashed before and after it. Known and not
findings: the DM console names no actor; a door event naming none reads as
today; the panel's heading and the door's actor may differ before a
player's first `ViewpointChanged` (D9); the agent's read tools return the
opener's id; a player's command sent by hand naming a character not theirs,
or not beside the door, is accepted when the player rule is met (D6, the
owner's ruling).

## For Phase 4b

Hand the reviewer `git diff HEAD` and tell it: check each scope claim
("every", "only", "no other") names the search that bounds it; check that
the contract diff only adds; check each comment under `internal/`, `cmd/`
and `client/src` is a warning or a pointer, and that no comment D13 names
still describes the old rule; check every assertion this work changed for
being weakened rather than moved (rule 2); read R13's text against every
role and every square, and its order after `Authorize` and `IsDoor`; read
the two tools' texts
against Task 5's tests (blueprint 34); and read `Event`'s cause against
`doorSubject` for every payload.
