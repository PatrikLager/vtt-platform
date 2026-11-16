# The Story draws what happens in the game, and no line that is mechanical

## The problem

**The Story draws a line for every entry of the log.** Every role's window
draws the Story: `renderSpectator` in `client/src/view/spectator.ts` passes
the log the window holds to `buildFeed` (`client/src/view/feed.ts`) and
`renderFeed` draws each entry `buildFeed` keeps with `describe`. `buildFeed`
leaves out only the runs `reintroductions` names, a withdrawal and
introduction again of one actor with the grants and conditions that restate
it. So the Story draws, in every window whose log holds the entry:

- a `resourceChanged` as "<actor> <resource> → <new value>";
- an `abilityGiven` as "<actor> can now use <ability>" and an `abilityTaken`
  as "<actor> can no longer use <ability>" (VTT-736);
- a `resourceAdded` as "<actor> now has <resource> at <current>" and a
  `resourceRemoved` as "<actor> no longer has <resource>" (VTT-790);
- an `actorControlGranted` as "control of <actor> given" and an
  `actorControlRevoked` as "control of <actor> taken back";
- a `tokenPlaced` as "<token> is at <x>,<y>", a `tokenMoved` as "<token>
  moved to <x>,<y>" with its reason when its frame carries one (VTT-263), a
  `tokenHidden` as "<token> went out of sight", a `tokenRemoved` as "<token>
  taken off the board" and a `tokenDescribed` as "<token> described";
- a `sceneCreated` as `scene "<name>"` and a `sceneSeen` as "the view
  changed";
- a `noteUpserted` as `note "<key>" updated` and a `noteDeleted` as
  `note "<key>" withdrawn`;
- an `adventureLoaded` as "adventure <id> loaded";
- a `doorOpened` as "a door opened at <x>,<y>" and a `doorClosed` as "a door
  closed at <x>,<y>".

In a character log some of these entries are the projection's corrections at
a sight, written with no event id (SPEC-016: in a character log an entry with
no event id is the projection's own). `statusFrames`
(`internal/perceive/projection.go`) writes the sheet's and control's entries
to bring the log's belief of a party member to its present sheet: a party
member held unseen whose sheet holds no attributes or module data is
corrected rather than introduced again at its first sight, so the Story draws
a line for each resource it holds and each ability it names (SPEC-022,
Consequences). The same projection writes a `doorOpened` or a `doorClosed`
for a door it sees whose state differs from what the log believed, so a door
opened out of the character's sight is drawn as "a door opened at <x>,<y>"
when it comes into sight.

A `tokenHidden` is never an event of the table: no command writes one, and
every one carries no event id. In `internal/perceive/projection.go`,
`transitions` writes one for each token the character no longer sees, and
`hideTokensOf` one for each token of an actor the projection withdraws to
introduce again, in sight or not. A character log that resumes, when
its actor becomes a party member again, hides every token it holds before
the projection places what the character sees (`resume` in
`internal/campaign/fanout.go`, VTT-318). A token coming back into sight is
a `tokenPlaced`.

The owner ruled on 2026-11-16 that the Story is the game's story, and
mechanical information goes to the log:
- The Story writes nothing from a sheet: what a character holds is looked up
  on the sheet, and lines such as "Tobbe has 2 flurry uses left" make the
  Story unreadable. An ability's use is written: the line "<actor> used
  <ability> on <target>: <result>, rolled <total>" stays. A condition gained
  or lost belongs to the story and stays.
- A grant and a taking back of control are mechanical and leave the Story.
- A token's placing, moving, removal and description, a scene, a change of
  view, a note and an adventure's loading are mechanical and leave the Story.
  A session's start and end, a character coming into sight, joining the
  party and leaving, and a roll stay.
- A token's going out of sight leaves the Story too. Without its placing, a
  resumed log would draw "<token> went out of sight" for tokens still on
  the board, and the Story would never draw their return.
- A door opened or closed advances the story and is an event in the game: it
  stays, worded as an event rather than as a square. The owner's wording,
  "Tobbe opened the door", names who opened it; no door command or event
  carries that today (`OpenDoor` and `DoorOpened` in `contract/vtt/v1` carry a
  scene and a square), and naming the opener is a ticket of its own.

This overturns the owner's ruling OD-4 of 2026-11-14, by which the Story
draws a give and a take in every window, a correction's included (SPEC-022),
and OD-4 of `docs/superpowers/plans/2026-11-15-an-ability-is-given-with-its-value.md`,
signed off by the owner, by which the Story words a resource added and a
resource removed.

The same rulings say no view replaces the sheet's lines in this work. A
player's panel that holds the character's sheet is the next ticket, by the
owner's ruling of 2026-11-16.

**What MapTool does.** Changing a token's property posts nothing to the
chat: neither `TokenPropertyFunctions` (`setProperty` and its kin) nor
`EditTokenDialog` calls `MapTool.addMessage` or `MapTool.addLocalMessage`.
Moving a token posts no chat line of its own; what reaches the chat on a
move is what an event macro the campaign runs returns (`EventMacroUtil`).
Near a token's arrival, `ZoneRenderer` sends `TextMessage.gm`
"Token.dropped.byPlayer" to the GM alone when a player drops a token, and
the paster sees its own error ("Token.error.unableToPaste"). MapTool has no door in its core: a
door is a macro's, and what reaches the chat is what the macro prints, under
the token the player speaks as. The chat holds what is said and what a macro
prints. The precedent fits and is borrowed: a use of an ability is a line, a
sheet's change and a token's move are not, and what is mechanical goes to the
GM's view.

## Done looks like

1. **The Story draws no sheet or control entry.** In a window of any role, a
   log holding a `resourceChanged`, an `abilityGiven`, an `abilityTaken`, a
   `resourceAdded`, a `resourceRemoved`, an `actorControlGranted` or an
   `actorControlRevoked` draws no Story line for it, whether the entry is
   forwarded from the world log or is a correction with no event id. A client
   test that fails on today's tree, where each is drawn.
2. **A first sight draws none either.** A party member held unseen whose
   sheet holds no attributes or module data, coming into a character log's
   sight, draws no "now has" and no "can now use" line in that window's
   Story. A client test that fails on today's tree.
3. **The Story draws no board, scene, note or adventure entry.** In a window
   of any role, a log holding a `tokenPlaced`, a `tokenMoved`, a
   `tokenHidden`, a `tokenRemoved`, a `tokenDescribed`, a `sceneCreated`, a
   `sceneSeen`, a `noteUpserted`, a `noteDeleted` or an `adventureLoaded`
   draws no Story line for it, with an event id or none. A client test that
   fails on today's tree.
4. **A door is an event, not a square.** A `doorOpened` or a `doorClosed`
   that carries an event id is drawn in the Story as "a door opened" or "a
   door closed", naming no square; one with no event id draws no Story line.
   Client tests that fail on today's tree.
5. **The Events panel is unchanged.** A DM's or an agent's window still draws
   a row in the Events panel for each entry, worded as today, a move's reason
   and a door's square included (VTT-447; a move's reason by VTT-263, which
   VTT-821 succeeds). A client test that passes
   on today's tree and after.
6. **What stays in the Story.** An ability's use, a condition gained and a
   condition lost, a narration, a whisper, a session started and ended, an
   actor coming into sight or joining the party, an actor leaving, and a roll
   are still drawn as today. Client tests that
   pass on today's tree and after.
7. **A narration is never dropped.** A narration anchored only to entries the
   Story leaves out is still drawn, in a beat of its own. A client test.
8. **The player is still told.** A player's toast on a give and a take of
   their party member's ability (`sheetNews`, `client/src/view/naming.ts`;
   VTT-733) is unchanged. Its tests stay green.
9. **The gate.** `task check` is green, the embedded bundle under
   `cmd/vtt/webdist` is rebuilt from the changed client, and `task e2e` is
   green.

## Rules this puts on the system

- The Story draws no line for an entry that changes what a sheet holds: a
  resource's value, a resource's presence or an ability's presence.
- The Story draws no line for a grant or a taking back of a character's
  control.
- The Story draws no line for a token's placing, move, going out of sight,
  removal or description, a scene's creation, a change of view, a note's change or an
  adventure's loading.
- The Story draws a door's opening or closing that carries an event id
  without naming its square, and draws no line for one with no event id.

The Events panel's row for each such entry is an existing rule, VTT-447,
held and not added.

## What it touches

- `client/src/view/feed.ts` (`buildFeed`): where the Story leaves the entries
  out. `client/src/view/spectator.ts` (`renderFeed`): the Story's words for a
  door. `describe` still words every entry for the Events panel
  (`renderTicker`).
- Client tests that assert a Story line for one of these entries, among them
  `client/test/spectator-view.test.ts`, `client/test/feed.test.ts`,
  `client/test/app.test.ts`, `client/test/qa-companion-first-sight.test.ts`,
  `client/test/qa-events-panel.test.ts`, `client/test/qa-move-reason.test.ts`,
  `client/test/qa-older-answer.test.ts`, `client/test/qa-property-names.test.ts`,
  `client/test/qa-sheet-news.test.ts`, `client/test/sheet-news.test.ts` and
  `client/test/qa-what-a-window-shows.test.ts`. Which of their assertions
  read the Story and which read `describe` or the Events panel is the plan's
  to sort.
- `client/e2e/table.spec.ts`, whose player's Story expects a token's placing.
- `cmd/vtt/webdist`, the committed bundle `task build:client` writes.
- `docs/requirements.md`: the rows that say the Story or the feed words one
  of these entries or names a resource in such a line (VTT-662, VTT-736,
  VTT-790, VTT-622, VTT-623, VTT-756, VTT-263), withdrawn or succeeded as the
  plan decides.

## Specifications this moves

- docs/specifications/022-a-client-holds-one-log.md

## What could not be established

- **Who opened a door.** The Story's door line names no one: `OpenDoor` and
  `DoorOpened` carry a scene and a square, the envelope's participant is
  never forwarded to a player (SPEC-016), a player may play more than one
  character, and the DM works a door with none. Naming the opener is a ticket
  of its own, by the owner's ruling of 2026-11-16.
- **How a player learns of a grant.** A taking back is told to the player by
  the toast "You no longer play …" (`onPartyMembersLeft` in `client/src/app.ts`),
  which blueprint principle 12 names. A grant has no message: once the Story
  draws no control line, the player learns of it as the window starts to
  offer the character. The owner ruled on 2026-11-16 that none is added.
- **Where a value is still seen.** After this, a player sees a party
  member's resource values only on its disc's chips on the board. A party
  member with no token on the board shows them nowhere in that window. The
  player's panel that holds the sheet is the next ticket.
- **What an ability's use leaves unsaid.** The ability's line names each
  target's result and roll, never the damage or the cost. The resource change
  behind it, a wound among them, then reaches the window only as data: the
  chip, and the DM's Events panel.
- **What the board alone now shows.** A token's arrival, move, going out of
  sight and removal, a scene and a note reach a player's window as the board and the notes panel
  show them, and as no line. A move's reason reached no player before this
  (SPEC-016 strips it from a forwarded move) and still reaches the DM's
  Events panel.
- **The register.** Whether VTT-736 and VTT-790 are reworded for the Events
  panel or withdrawn, and how VTT-622, VTT-623, VTT-756, VTT-263 and VTT-662
  lose their Story halves, is the plan's.
