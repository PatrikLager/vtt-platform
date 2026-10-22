# Alpha one — what held, what broke, and the owner's rulings

On 2026-10-22, before any guest plays, the owner ran the product alone: two
party members in two browser windows side by side, the agent as DM through
`vtt mcp`, on `main` at `41ccead`. The table was a scratch campaign outside the
tree, on the `dnd45e-minimal` ruleset, with an adventure written for the test
and not committed: a 13×9 house of two rooms with a shut door between them.
Lera stood in the west room beside a sleeping goblin, and Tobbe in the east
room with a goblin cutter and a goblin archer. Neither could see the other's
room.

Each item cites what showed it: a sequence in that campaign's world log, a
window, or the owner's words. Nothing was fixed during the walk.

## What held

- **Hidden information at the start.** Lera's window showed the sleeping
  goblin, and nothing in Tobbe's mentioned it. Nothing in Lera's mentioned the
  two goblins in the east room.
- **Line of sight through the open door** (seq 22–23). Tobbe, in the doorway,
  saw Lera and not the sleeping goblin behind the wall. Lera saw Tobbe and the
  goblin cutter, and not the archer. The owner: "Helt enligt line of sight".
- **Combat kept apart** (seq 25–29). Lera saw Tobbe's hp fall twice and was not
  told the archer's arrow was one cause: the projection forwards a resource
  change to a viewer who sees the actor it changes, and clears its reason on
  everything it forwards (SPEC-016). Tobbe saw both attacks. When Lera killed the sleeping goblin (seq 32–35), Tobbe was sent
  none of it.
- **A secret note** was shown in no player's window.
- **Idle connections** survived a sixteen-minute pause: every connection
  stayed open, and the next move was accepted (seq 31).
- **A returning connection** showed its character at once, with its hp,
  conditions and whole history, attacks made while its tab was closed
  included.
- **Switching.** A player granted both characters switched between their views
  in one window (seq 43). A spectator chose a shoulder and switched.
- **A revocation** emptied the player's window at once (seq 44).
- **Walking through the open door** was accepted (seq 46–47).

## Findings

### Stopped play

- **S-1. A player can never use an ability from the browser.** Lera's
  Longsword Strike at the sleeping goblin was answered `refused: rules:
  resolve: unknown target actor "tok-sleeper"`. The player panel sends the
  target token's id, and `rules.Resolve` reads actor ids. The DM landed the
  same strike through `vtt mcp` with the actor id (seq 32). Ticket:
  `docs/superpowers/specs/2026-10-22-a-players-ability-reaches-the-creature-design.md`.

### Cost time

- **S-2. A player who loses their token has no way back to their character.**
  Tobbe's window, closed and reopened at its address, said `No invite token.
  Open the link your DM sent you.` The client keeps its token in
  `localStorage`, which survives a tab close, and never clears it (`Auth.clear`
  is called nowhere). What discarded it was not established; a private window
  or a setting that clears site data would. It was the window's only tab. Reopening the
  join link admitted a NEW participant also named "Tobbe". The DM granted Tobbe
  to the new one (seq 42), while the old one stayed in the roster, still
  controlling him, with a credential nobody holds. The agent's tools cannot
  revoke a participant. `vtt revoke` can: it takes them off the roster, and
  the characters they control keep them as a controller.
- **A-5. A click to see what something is moves your character there.** The
  owner clicked the sleeping goblin to learn what it was, and Lera was moved
  onto its square (seq 20), then back (seq 21). Both moves are in the log for
  good. Unless the door tool is armed, `playerClickFor` in
  `client/src/view/player.ts` makes a board click a switch when it lands on
  another party member the player may watch, nothing when it lands on the
  watched character's own token, and a move everywhere else. Nothing asks the
  board what a token is. In MapTool a
  click selects a token and shows its details, and a move is a drag.
- **A-9. Reloading the page forgets whose eyes you look through.** A spectator
  on Tobbe reloaded and came back on nobody. A player holding both characters,
  on Tobbe, reloaded and came back on Lera, the first party member by id.
  `Wire` in `client/src/wire.ts` keeps the log it holds in memory only, so a new
  page dials with no `log` and the server opens the default (SPEC-015).

### Awkward

- **A-1. Every character is offered every ability in the ruleset**, the
  goblins' included: Lera's panel offered Goblin Scimitar and Goblin Shortbow.
  Nothing in the ruleset format says which abilities a character has.
- **A-2. "actor Sleeping Goblin joined."** `describe` in
  `client/src/view/spectator.ts` words an actor arriving in a character's log as
  if someone had joined the table.
- **A-3. Refusals speak developer.** A move into a wall read `refused: gateway:
  cannot move there — a wall`. `gateway:` is a package name.
- **A-7. A player is not told why their window emptied** when their character
  was revoked (seq 44).

### False prose

- **A-8. `open_door` on a wall is accepted, though the tool says it is
  refused.** The DM opened a stone wall square (seq 45), and the result was ok.
  `open_door`'s tool description says "Refused unless the square's tile is a
  door", but no check reads the square's nature: not `mayWorkDoor` in
  `internal/gateway/authz.go`, not the gateway's conversion of the command, and
  not `engine.Apply`'s `DoorOpened` arm. The wall still stops movement and
  sight (`engine.State.Blocked` and `sight.Blockers` read `wall` before
  `OpenDoors`). The log gains a door opened where none exists, and the agent
  relies on a description that is not true.

### Found while working on the findings

- **A-6. A player is sent a creature's true name and its whole statblock as
  soon as their character sees it.** `Projection.introduce` in
  `internal/perceive/projection.go` sends a clone of the actor: the name
  "Sleeping Goblin", its defenses and its hp. The name showed in Lera's window
  (A-2), and the rest is in the frames the client receives.
- **A-10. The server does not check that a player's character sees the target
  of an ability.** `use_ability`'s authorization (`authorizeActorOwnership` in
  `internal/gateway/authz.go`) checks only that the player controls the caster,
  and `rules.Resolve` checks the range. A player who knows an unseen creature's
  id can aim at it, and the answer tells them whether that id exists.

### Observations

- **A-4. Two tokens may share a square.** `engine.State.Blocked` reads terrain
  and objects, not tokens. MapTool allows it too.
- **D-4. Granting a character to a second player does not take it from the
  first.** Control is a set (`grant_actor_control`; MapTool's
  `Token.addOwner` is a set too), so moving a character is a grant and then a
  revocation. The owner expected the grant to move it. Nothing in the window
  shows that two players share a character.
- **Before the walk:** `vtt join-link show` still prints `share:
  <your-server-url>/?join=…`, as
  `docs/superpowers/notes/2026-08-12-session-zero-backlog.md` found (A1).

## The owner's rulings

Asked during and after the walk, 2026-10-22.

- **On A-6: "servern ska inte skicka det".** The server sends a player no
  non-party actor's name and no statblock. What it sends instead is K-1.
- **K-1. Hovering over or right-clicking a token shows what the character can
  perceive of it**, "a sleeping creature" for example. "Första delen är det
  viktiga": a monster check that reveals more on a success ("looks like a
  sleeping goblin warrior") comes later.
- **B-5. That information lives on the token, as in MapTool, and MapTool's
  split is borrowed whole** ("Låna MapTools uppdelning rakt av"). A MapTool
  `Token` carries `name` and `notes`, which players see, and `gmName` and
  `gmNotes`, which only the GM sees, and MapTool's hover sheet drops `gmOnly`
  properties for a player. MapTool sends everything to every client and
  filters there; here the server sends a player only what is theirs to see.
  Stats stay on the actor.
- **B-1. Two channels.** Event data, meaning moves, rolls, hits, damage, hp and
  conditions, follows sight strictly: a player whose character does not see
  something is sent no event data about it, not even that it happened. The
  ruling of 2026-09-29 ("endast det man ser, får man information om") stands.
  Text is the DM's tool: the DM decides whether someone hears an attack ("Det
  är ju en bra uppdelning").
- **B-2. Whispering, as in MapTool.** The DM can whisper to a character, and a
  character to the DM, with nobody else in the party seeing it, "även när alla
  står i samma rum".
  - A whisper is addressed to a CHARACTER and is shown only in that character's
    log ("Bara till Tobbe, det var han som hörde det"). A whisper to a PLAYER is
    shown in every character that player holds.
  - When a player whispers to the DM, the character whispers ("Tobbe viskar"),
    and it is shown only in that character's log and to the DM.
  - MapTool's `TextMessage` has the channels ALL, SAY, GM, ME, GROUP,
    WHISPER, GMME, NOTME, NOTGM and NOTGMME, and a whisper names its `target`.
    But `MapTool.addServerMessage` drops a whisper for anyone but its target on
    the client, after every client has received it. The channels are borrowed, and
    that distribution is not.
- **B-3. The "Events" panel is the DM's alone.** Players and spectators do not
  have it. "Story" stays as it is ("kändes bra, just nu iaf"). A macro button
  whose attack carries both text and mechanics, as in MapTool, comes later.
- **B-4. A player whose character the DM has taken watches as a spectator
  does.** The role stays player. What changes is whom they may watch while they
  control no party member.
- **Deferred: the character sheet.** What an actor holds, its sheet's shape for
  each ruleset, and its equipment, items and magic are a conversation for later
  ("vi går vidare med stats samtalet vid en annan tidpunkt"). A-1 belongs
  there.

## The order of work

The owner agreed this order ("ok, ordningen stämmer, kör"), one ticket and
one branch each:

1. S-1: a player's ability reaches the creature.
2. A-9: a reload keeps whose eyes the window looks through.
3. B-3: the Events panel is the DM's alone.
4. B-4: a player without a character watches as a spectator.
5. B-5, K-1 and A-6, with A-2 and A-10: a token carries what players and the DM
   may see of it, and the server sends a player nothing of a creature beyond
   it.
6. B-2: whispering to and from characters.
7. S-2: a player who lost their token comes back without a duplicate. Its
   shape is the owner's to decide.
8. A-3 and A-7: refusals in plain words.
9. A-8: `open_door` refuses a square that is not a door.

Still to decide when they come up: whether a left click still moves once a
hover shows information (A-5), whether two tokens may share a square (A-4), and
whether the window shows that a character is shared (D-4).
