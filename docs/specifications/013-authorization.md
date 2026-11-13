# SPEC-013: Who may issue a command is decided on the server, per command, against the fold

## Status

Accepted. Implemented by `internal/gateway/authz.go` (`commandRoles`,
`Authorize`, `mayVoice`, `mayWhisper`, `authorizePlayer`, `playerRules`,
`errUndecided`, `unrestricted`, `mayWorkDoor`, `authorizeTokenOwnership`,
`authorizeActorOwnership`, `controls`, `authorizeSelfRevoke`,
`authorizePromotionTarget`, `commandName`, `ErrUnauthorized`,
`plainRefusal`),
`internal/gateway/viewpoint.go` (`ownPartyMember`),
`internal/gateway/grant_validate.go`
(`validateGrantActorControl`), `internal/gateway/add_actor_validate.go`
(`validateAddActor`, `validateAddActorWith`),
`internal/gateway/note_validate.go` (`validateUpsertNote`),
`internal/gateway/convert.go` (`ToEvent`, `voice`) and
`internal/gateway/server.go` (`answerCommand`, `authorize`,
`handleSetViewpoint`, `handleCommand`, `describeBlockage`, `doorSquare`),
with `perceive.SeesActor` and `rules.UnknownTarget` for the sight gate,
`rules.Ruleset.SheetKeyFault` for `add_actor`'s keys and
`engine.State.IsDoor` for the door gate, against `internal/engine`'s
`State`; pinned by `internal/gateway/authz_test.go`,
`grant_validate_test.go`, `add_actor_validate_test.go`,
`add_actor_fit_internal_test.go`, `qa_add_actor_keys_test.go`,
`note_validate_test.go`, `convert_test.go`, `server_test.go`,
`server_visibility_test.go`, `server_internal_test.go`, `watch_test.go`,
`seat_internal_test.go`, `viewpoint_test.go`, `qa_seat_test.go`,
`qa_move_gate_test.go`, `qa_describe_token_test.go`,
`qa_ability_sight_test.go`, `narration_test.go`, `qa_speaking_test.go`,
`remove_participant_test.go`, `qa_removal_test.go`,
`qa_remove_roles_test.go`, `whisper_test.go`, `qa_whisper_test.go`,
`plain_refusal_test.go`, `qa_plain_refusal_test.go`, `door_square_test.go`
and `qa_door_square_test.go`, and by `internal/engine/terrain_test.go` and
`cmd/vtt/scenario_goldens_test.go`.

## Principles served

Principles 6, 7, 10, 13, 20 and 34 of `docs/blueprint.md`:

- **6.** A command is judged on the server, never by the window that sent it.
- **7.** It is judged per command, against the fold as it stands when the
  command arrives.
- **10.** A player speaks only as a party member they control, or out of
  character.
- **13.** The move gate's and the door gate's refusals name no package and
  no id, and the two that refuse a party member with no name name no package
  (VTT-601 to VTT-603, VTT-608); this record bounds no other refusal's text.
- **20.** A player's move onto a square their party member cannot see is
  refused, and so is an ability aimed at an actor it does not see.
- **34.** The door tools say a door command is refused unless the square's
  tile is a door, and the door gate refuses it so, for every role.

## How it works

**One function, one table.** `Authorize` decides whether a participant may
issue a command, against a snapshot of the fold, and `authorize` in
`server.go` is its one production caller (`grep -rn 'Authorize(' internal
cmd` outside the test files). `commandName` maps the `ClientCommand` oneof's
set arm to that field's proto name, and answers `""` for an unset command.
`commandRoles` is keyed by that name and holds, per command, the roles that
may issue it:

| Command | dm | agent | player | spectator |
|---|---|---|---|---|
| `move_token` | yes | yes | `authorizeTokenOwnership` | no |
| `add_actor` | yes | yes | no | no |
| `place_token` | yes | yes | no | no |
| `describe_token` | yes | yes | no | no |
| `remove_token` | yes | yes | no | no |
| `remove_actor` | yes | yes | no | no |
| `start_session` | yes | yes | no | no |
| `end_session` | yes | yes | no | no |
| `use_ability` | yes | yes | `authorizeActorOwnership` | no |
| `remove_condition` | yes | yes | `authorizeActorOwnership` | no |
| `add_narration` | yes | yes | `unrestricted` | no |
| `add_whisper` | yes | yes | `unrestricted` | no |
| `upsert_note` | yes | yes | no | no |
| `delete_note` | yes | yes | no | no |
| `load_adventure` | yes | yes | no | no |
| `load_map` | yes | yes | no | no |
| `grant_actor_control` | yes | yes | no | no |
| `revoke_actor_control` | yes | yes | `authorizeSelfRevoke` | no |
| `promote_participant` | yes | yes | no | no |
| `remove_participant` | yes | yes | no | no |
| `set_join_door` | yes | yes | no | no |
| `rotate_join_link` | yes | yes | no | no |
| `open_door` | yes | yes | `mayWorkDoor` | no |
| `close_door` | yes | yes | `mayWorkDoor` | no |
| `set_viewpoint` | no | no | `unrestricted` | yes |

A player cell names the `playerRules` entry that decides it; every other
"yes" is the whole of the role's answer from the table. The DM and the agent
hold every row but `set_viewpoint`, since they are sent the world and watch
no character log (SPEC-015); `set_viewpoint` is the spectator's only cell and
one of the player's. Removal authors the board and the cast, so
`remove_token` holds `place_token`'s cells and `remove_actor` holds
`add_actor`'s: a player may remove neither a token nor an actor they control.
`describe_token` holds `place_token`'s cells too: what a token is called and
what its notes say are the DM's and the agent's to write (VTT-463; "Ja, ett
eget kommando", the owner's ruling of 2026-10-24).
Granting control, promotion, removing a participant and the join door are
the DM's and the agent's alone, since each confers or takes away standing at
a table whose shared link admits spectators (SPEC-009, VTT-028, VTT-565).
Every refusal `Authorize` returns wraps `ErrUnauthorized`: every `fmt.Errorf`
in `authz.go` and `viewpoint.go` wraps it with `%w`, and `plainRefusal`, a
refusal whose text names no package, returns it from `Unwrap`.
`TestEveryClientCommandHasRoleCells` walks the oneof and requires every arm
to resolve through `commandName` to a name with a row, and
`TestAuthorizeTableAllCommandsAllRoles` asserts every cell of the table
above. The role maps the HTTP routes consult are not cells of this
table (SPEC-012).

**Refusal by default.** `Authorize` refuses when `commandRoles` holds no row
for the name or the row holds no cell for the participant's role
(`!known || !roles[p.Role]`), and the message names the role and the command.
A command added to the oneof and not to the table, and an unset command, are
refused for every role the same way.

**Four checks for every role.** After the table and before the player half,
`Authorize` runs `authorizePromotionTarget` for `promote_participant`, which
accepts a target role of player or spectator and refuses every other, for
the DM's promotions as for anyone's; SPEC-009 states that bound and
`handlePromotion`'s bound on the target's current role. For `set_viewpoint`
it runs `MayWatch` against the snapshot's actors, whose rule is SPEC-015's.
For `add_narration` it runs `mayVoice` against the snapshot's actors, which
decides whom a line may speak as:
- a line out of character that names an actor is refused, for every role;
- any other line out of character is accepted (VTT-495, VTT-497);
- the DM and the agent name no actor (VTT-499), and voice anyone by `as`;
- a player's line in character must name, in `actor_id`, a party member the
  player controls (`ownPartyMember`), and is refused with one text whether the
  id is absent, not theirs, or not a party member (VTT-506), then refused if
  that party member has no name (VTT-495), with a `plainRefusal` whose text
  is `This character has no name. Ask the DM for one, or speak out of
  character with /ooc.` (VTT-603).

For `add_whisper` it runs `mayWhisper` against the snapshot's actors, which
decides whom a whisper is to and from, by the owner's ruling B-2 of
2026-10-22:
- the DM's and the agent's names party members in `to_actor_ids`, at least
  one, and no `from_actor_id`; one that names no actor, an actor that is not
  a party member, or a whisperer is refused (VTT-587);
- a player's names no `to_actor_ids`, and names in `from_actor_id` a party
  member the player controls (`ownPartyMember`), refused with one text
  whether the id is empty, absent, not theirs, or not a party member
  (VTT-589), then refused if that party member has no name (VTT-588), with
  a `plainRefusal` whose text is `This character has no name. Ask the DM for
  one with /ooc, then whisper.` (VTT-603).

All four run for every role, so the DM is bound by promotion's bound, by
`mayVoice` and by `mayWhisper`, and a spectator, whom the player half never
reaches, is bound by `MayWatch`; a spectator has no `add_whisper` cell
(VTT-590).

**The player half.** For every role but player, `Authorize` returns after
those four checks. For a player it calls `authorizePlayer` with the name it
derived and the command together, and a mismatched pair would judge a command
by another's rule; `Authorize` is its one production caller (`grep -rn
'authorizePlayer(' internal`). `authorizePlayer` looks the name up in
`playerRules`, a table keyed like `commandRoles` whose entries are
`playerRule` functions run against the participant, the command and the
snapshot. A player cell with no entry is refused by `errUndecided`, whose
message names the command. `unrestricted` is the entry for `add_narration`,
whose rule `mayVoice` has already applied, for `add_whisper`, whose rule
`mayWhisper` has, and for `set_viewpoint`, whose rule
`MayWatch` has, so a command that needs no rule here says so in the table
rather than by its absence. `TestEveryPlayerCommandHasARule` holds that every
player cell has an entry and every entry a player cell; while the two agree no
command reaches the missing-rule arm through `Authorize`, and
`AuthorizeUndecidedForTest` in `export_test.go` reaches it directly.

**The rules.** Each reads the snapshot's `Tokens` and `Actors` and nothing
else, and `Authorize` performs no I/O: `authz.go` and `viewpoint.go` reach
`internal/identity` for its types and role constants and `internal/engine`
for `State` and `IsPartyMember`, and call nothing that reads a file, the
network or the identity database.

- `move_token`: `authorizeTokenOwnership` refuses a token the snapshot does
  not hold, and a token whose actor the snapshot does not hold or whose actor
  the player does not `controls`.
- `use_ability` and `remove_condition`: `authorizeActorOwnership` on the
  command's `actor_id` refuses unless the snapshot holds that actor and the
  player `controls` it.
- `revoke_actor_control`: `authorizeSelfRevoke` refuses a `participant_id`
  other than the player's own, then asks `authorizeActorOwnership`, so a
  player revokes only control they hold, and a revoke of control never held
  is refused here rather than appended by the fold as a change to nothing.
- `open_door` and `close_door`: `mayWorkDoor` accepts when a token on the
  command's scene, whose actor the player `controls`, stands at most one
  square from the door on each axis (`abs(dx) <= 1 && abs(dy) <= 1`), so the
  eight neighbouring squares count and a token on another scene does not. It
  asks where a controlled token stands and nothing about reach, movement cost
  or a ruleset.
- `add_narration`: `unrestricted` accepts what `mayVoice` accepted.
- `add_whisper`: `unrestricted` accepts what `mayWhisper` accepted.

**Control is membership.** `controls` answers whether the participant id is
among the actor's `controller_ids`, through `GetControllerIds()`, and never
reads the `controller_id` mirror, which carries one controller of several. An
actor whose set is empty is controlled by nobody, so no player rule that
asks about an actor accepts it, and it is the DM's and the agent's alone. An
empty participant id controls nothing: `controls` answers false for it before
reading the set, and on the revoke path, where `authorizeSelfRevoke`'s
comparison of two empty ids passes, that guard is the refusal. Control is an
`ActorControlGranted` in the log (SPEC-009); an `add_actor` confers none
(below).

**What the DM and the agent are free of.** No player rule runs for them:
`Authorize` returns before `authorizePlayer`, `mayWorkDoor` returns nil for
every role but player, and `handleCommand`'s move gate and sight gate run for
a player only. They act on, and grant and revoke control of, an actor a player
controls, and acting on it leaves its `controller_ids` as they were; they
work a door from anywhere, though only a door; they move a token onto any
square, a wall and a square no player can see included (VTT-158, VTT-379);
they aim an ability at an actor no character sees (VTT-482). The table's
cells, the four checks for every role, the door gate and the three
validators bind them.

**The command path.** `serve`'s read loop decodes each frame, re-resolves the
participant through `identity.Lookup` (SPEC-009) and hands the command to
`answerCommand` (SPEC-011 owns the loop). `answerCommand` has three arms: a
lookup error that is not a revocation refuses the command with `gateway:
identity unavailable` (SPEC-009); `set_viewpoint` goes to
`handleSetViewpoint`; every other command goes to `handleCommand`.
`answerCommand` queues no frame: it returns the result, and the read loop
queues it. `authorize`, which both handlers call first, takes
`campaign.State()` once and refuses with `gateway: campaign unavailable` when
it is nil, which it is for a poisoned campaign; otherwise it asks `Authorize`
against that snapshot and refuses with the error's text. `handleCommand` then
runs, in this order, and stops at the first refusal:

1. for a player's `move_token` whose token the snapshot holds, the move gate
   below;
2. for a player's `use_ability`, the sight gate below;
3. for an `open_door` or a `close_door`, from any role, the door gate below;
4. `validateGrantActorControl`, for a `grant_actor_control`;
5. `validateAddActorWith`, for an `add_actor`: `validateAddActor`, then the
   actor's keys against the served ruleset;
6. `validateUpsertNote`, for an `upsert_note`;
7. the dispatch of `use_ability`, `load_adventure`, `load_map`,
   `remove_actor`, `remove_participant`, `promote_participant`,
   `set_join_door` and `rotate_join_link` to their handlers;
8. `ToEvent`, which makes every other command one envelope stamped with the
   participant's id and role;
9. for a `TokenMoved`, the backfill of `SceneId` and `From` from the
   snapshot's token; for a `NarrationAdded`, `voice`, which writes `as`: the
   named actor's name from the same snapshot for a player's line in character
   (VTT-496), and the sender's display name for any line out of character
   (VTT-497); otherwise it leaves the `as` the DM or the agent gave
   (VTT-498);
10. `campaign.Append`, which folds a clone of the envelope and returns the
   fold's refusal before anything is persisted;
11. an ok=true result carrying the sequence `campaign.Append` assigned.

Every refusal is an ok=false `CommandResult` whose `Error` is the refusal's
text. The read loop queues it on the issuing connection and reads the next
command: no refusal on this path closes the connection, and a refused command
appends nothing, so no other connection sees it. `handleCommand` never closes
the connection or writes to the wire: neither it nor `answerCommand` is
handed the connection.

**The player's move gate.** For a player's `move_token` whose token
`st.Tokens` holds, `handleCommand` first asks `canSee(sub.judgedBy(p,
st.Actors), st, tok.SceneID, to)`, through the eyes of the character
SPEC-015 says the move is judged by, and when it answers false, it refuses
with `You cannot move there: you cannot see that square.`, whatever stands on
the square (VTT-601). Only then does it ask `st.Blocked(tok.SceneID, x, y)`,
and refuses a blocked square with `You cannot move there: `, then
`describeBlockage(why)`, then `.` (VTT-602). A square off the grid is one
`canSee` answers false for, since `sight.VisibleFrom` walks the grid's
squares alone, and the move gate is `Blocked`'s one caller outside the tests,
so a player's move refusal never reads `outside the grid`
(`TestQAPRPlainASquareOffTheGridIsRefusedInPlainWords`).
`engine.State.Blocked` answers for every square of a scene, seen or not, so
the order is what keeps an unseen square's refusal one string that names no
wall, door or scenery. A square out of the player's sight
is refused even when they remember its terrain. A token the snapshot does not
hold never reaches the gate, since
`authorizeTokenOwnership` has refused it. `canSee` and `judgedBy` are
SPEC-015's; what a character sees is SPEC-016's.

**The player's sight gate.** For a player's `use_ability`, `handleCommand`
asks of each target id, in the command's order,
`perceive.SeesActor(perceive.Eyes{Shoulder: sub.judgedBy(p, st.Actors)}, st,
id)`, and at the first it answers false refuses with
`rules.UnknownTarget(id).Error()`, the answer `Resolve` gives a target id that
names no actor, and appends nothing (VTT-472; A-10 in
`docs/superpowers/notes/2026-10-22-alpha-one-findings.md`). It runs before
`Resolve`, and an absent id is not seen either, so an unseen target and an
absent one get the same answer whatever else the command carries: an unknown
ability, a target named twice, more targets than the ability allows. A command
whose every target is seen goes on to `handleUseAbility` (VTT-487). The eyes
are the ones the move gate judges by, so a party member of the player's out of
that character's sight cannot be aimed at, not even by itself as caster, and a
player who controls only a creature has no eyes and names no target that is
not refused.

**The door gate.** For an `open_door` or a `close_door` that `Authorize`
accepts, from any role, `handleCommand` takes the command's scene id and
square from `doorSquare` and asks `engine.State.IsDoor` of them, against the
snapshot `authorize` took. `IsDoor` answers whether the tile at that square,
in a scene the state holds, has the kind `door`; it reads the tile and never
`OpenDoors`, so a door open or closed is a door. Where it answers false (a
wall, a floor, a square with no tile, a square off the grid, a scene the
state does not hold) the gate refuses with `There is no door there.`, for
every role, and appends nothing (VTT-608). It runs after `Authorize`, so a
player's command that `mayWorkDoor` refuses never reaches it and is answered
with `mayWorkDoor`'s text, one text whatever the square is (VTT-609). It
asks nothing about sight. `engine.Apply`'s door arms refuse
only a scene the state does not hold and a missing position, so the gate is
what holds a door command to a door.

**`describeBlockage`.** It rewrites the reason `engine.State.Blocked` gives.
`scenery: <kind>` becomes `something (a <kind>) is in the way`, the kind
passed through `artlib.Clip` at `artlib.MaxFragment`; a reason beginning
`unknown scene ` becomes `that destination is not part of any scene this
table has created`, naming no id; every other reason, each a literal
`Blocked` writes (`a wall`, `a closed door`, `outside the grid`), passes
through unchanged. The scenery kind is a map object's `kind`, free text from
the map file, and the one author-written string `describeBlockage` renders.

**The three validators.** They run after `Authorize`, for every role, before
anything is written, and their refusals do not wrap `ErrUnauthorized`: they
refuse a command's form, not its issuer. `validateGrantActorControl` refuses
a grant whose `kind` is `ACTOR_KIND_UNSPECIFIED` and accepts every other
value, so the enum may grow without a grant being refused; SPEC-007 states
the requirement as the wire's promise. `engine.Apply` accepts an
`ActorControlGranted` with no kind and leaves the actor's kind as it was, so
this refusal too exists at the command boundary alone. `validateAddActor`
first refuses an actor that names a controller, by a non-empty
`controller_id` or by any entry in `controller_ids`, an empty string
included, with a message naming `grant_actor_control`; then leaves an
`add_actor` with no actor or no `actor_id` to the fold, whose refusal says
the id is missing; then refuses a `kind` of `ACTOR_KIND_UNSPECIFIED` and
accepts every other value, so an `add_actor` may declare a party member that
nobody controls yet. A command wrong in both the controller and the kind is
told about the controller. `validateAddActorWith` runs `validateAddActor`
and then, with a ruleset served, reads an actor with an id against it,
refusing a key the ruleset does not declare, or declares as the other type,
naming the actor, the map and the key (VTT-677, SPEC-024); with none served
it reads no key (VTT-675). `engine.Apply` refuses an `ActorAdded` that names
a controller as well, and accepts one with no kind, which on a recorded event
reads as not a party member; the kind refusal exists at the command boundary
alone. `validateUpsertNote` refuses an `upsert_note` whose `visibility` is
`NOTE_VISIBILITY_UNSPECIFIED`, with a message naming `visibility`, and accepts
every other value, so the enum may grow without a note being refused.
`engine.Apply` accepts a `NoteUpserted` with no visibility, which the
projection writes to no character log (SPEC-016), so this refusal too
exists at the command boundary alone.

**What is dispatched, and what becomes one envelope.** `use_ability`
(`handleUseAbility`) and `load_adventure` (`handleLoadAdventure`) are
SPEC-012's; `load_map` (`handleLoadMap`) is SPEC-014's; `remove_actor`
(`handleRemoveActor`) is SPEC-017's; `remove_participant` (`handleRemoval`)
is SPEC-009's; and SPEC-007 states what the `load_map`, `remove_actor` and
`remove_participant` batches carry. All five append a whole batch through
`campaign.AppendBatch`, but a removal of a participant who controls no actor
appends nothing, and its ok=true result carries no sequence.
`promote_participant` (`handlePromotion`),
`set_join_door` (`handleJoinDoor`) and `rotate_join_link`
(`handleRotateJoinLink`) are SPEC-009's and append nothing (SPEC-007), so
their ok=true result carries no sequence. Every other command reaches
`ToEvent` and becomes one envelope, carrying every field of the command that
the event also has, a grant's `kind` and a move's `reason` included, but a
narration's `as` where `voice` writes it (VTT-503); a move that gave no
`reason` records none, and `AddNarration.actor_id` has no field in the
event. For a `TokenMoved`, `handleCommand` sets
`SceneId` and `From` from the snapshot's token before `campaign.Append`, so
the log records the scene and the square the token left; `engine.Apply`'s
`TokenMoved` arm reads `TokenId`, `To` and the lengths of `Reason` and
`SceneId` (SPEC-018), and never `From`.

**`set_viewpoint`.** `answerCommand` routes it to `handleSetViewpoint`, which
runs `authorize` (the cell and `MayWatch`), hands the choice to the pump with
`perches.set(actorID)` without waiting, and answers ok=true; what the pump
does with it is SPEC-015's. It appends nothing (SPEC-007). An ok result means
the choice is recorded, not that the switch is sent. `serve` makes `perches`
per connection, so a choice ends with its connection.

**What this record does not decide.** Promotion's bounds and the
re-resolution before every command are SPEC-009's; who may watch whom, the
viewpoint the move gate judges by, and what a seat is sent are SPEC-015's;
the batch handlers are SPEC-012's, SPEC-014's
and SPEC-017's; the join door and promotion handlers are
SPEC-009's; what the fold refuses is `engine.Apply`'s, and its byte bounds
SPEC-018's.

## Consequences

A client or tool author is bound by these:

- A spectator's only command is `set_viewpoint`. A player who controls a
  party member chooses by it which of their party members they watch, and a
  player who controls none chooses as a spectator does (SPEC-015, the
  owner's ruling B-4 of 2026-10-22). The DM and the agent may not issue it.
- A player acts only through an actor that counts them among its
  controllers; an actor nobody controls is the DM's and the agent's.
- A door command names a door: an `open_door` or a `close_door` at a square
  whose tile is not a door, or in a scene the table has not created, is
  refused for every role, the DM's and the agent's included.
- A player works a door from a controlled token on it or beside it, and moves
  only onto a square the party member SPEC-015 judges the move by can see now;
  a square they remember and cannot see is refused.
- A player aims an ability only at actors that party member sees; a target it
  does not see is answered as a target that names no actor.
- A token's texts are written by the DM and the agent alone.
- A player speaks as a named party member they control, or out of
  character; the server writes the name either line is shown under. The DM
  and the agent voice anyone in character by `as`, name no actor by id, and
  out of character are shown under their display name.
- A refusal is an ok=false result on a connection that stays open, seen by
  the issuer alone. A move refusal's text names nothing the character the
  move was judged by has not seen (VTT-156, VTT-159), and a player's
  `use_ability` at a target that character does not see is answered with the
  text an absent id gets (VTT-472), and a player's line in character naming an
  actor that is absent, not theirs, or not a party member is answered with
  one text (VTT-506), as is a player's whisper from an actor that is absent,
  not theirs, or not a party member (VTT-589), and a player's door command
  that `mayWorkDoor` refuses, whatever its square (VTT-609). The move gate's
  two texts name no package and no id (VTT-601, VTT-602), and the two a party
  member with no name is refused with begin `This character has no name.` and
  name no package (VTT-603). The door gate's text, `There is no door there.`,
  names no package and no id (VTT-608); no other refusal's text is bounded by
  this record.
- A `grant_actor_control` and an `add_actor` state what the actor is; an
  `add_actor` never confers control, and control is granted afterwards.
- An `upsert_note` states who may read the note.
- A player whispers to the DM alone, from a named party member they
  control; the DM and the agent whisper to party members alone, never from
  an actor. No command whispers to a participant: a whisper to a player
  names that player's party members, which the DM's console finds from a
  name (SPEC-022) and the agent names itself, as `add_whisper`'s tool tells
  it (SPEC-007).
- The DM and the agent are bound by the table's cells, by promotion's bound,
  by `mayVoice`, by `mayWhisper`, by the door gate and by the three
  validators, and by no player rule.
- What a reconnect opens on is SPEC-015's.

And whoever changes the code:

- A command added to the contract needs a row in `commandRoles`, and a player
  cell needs a `playerRules` entry, or the command is refused.
- A `Blocked` reason that interpolates text from a map file needs the clip
  `describeBlockage` gives the scenery kind.

## Requirements

VTT-138, VTT-139, VTT-140, VTT-142, VTT-143, VTT-144, VTT-145, VTT-146,
VTT-147, VTT-148, VTT-149, VTT-150, VTT-151, VTT-152, VTT-153, VTT-154,
VTT-156, VTT-157, VTT-158, VTT-159, VTT-160, VTT-161, VTT-162, VTT-231,
VTT-261, VTT-344, VTT-379, VTT-463, VTT-472, VTT-482, VTT-487, VTT-495,
VTT-496, VTT-497, VTT-498, VTT-499, VTT-503, VTT-506, VTT-565, VTT-587,
VTT-588, VTT-589, VTT-590, VTT-601, VTT-602, VTT-603, VTT-608, VTT-609,
VTT-675, VTT-677.
