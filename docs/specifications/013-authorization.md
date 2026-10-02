# SPEC-013: Who may issue a command is decided on the server, per command, against the fold

## Status

Accepted. Implemented by `internal/gateway/authz.go` (`commandRoles`,
`Authorize`, `authorizePlayer`, `playerRules`, `errUndecided`, `unrestricted`,
`mayWorkDoor`, `authorizeTokenOwnership`, `authorizeActorOwnership`,
`controls`, `authorizeSelfRevoke`, `authorizePromotionTarget`, `commandName`,
`ErrUnauthorized`), `internal/gateway/grant_validate.go`
(`validateGrantActorControl`), `internal/gateway/add_actor_validate.go`
(`validateAddActor`), `internal/gateway/note_validate.go`
(`validateUpsertNote`), `internal/gateway/convert.go` (`ToEvent`) and
`internal/gateway/server.go` (`answerCommand`, `authorize`,
`handleSetViewpoint`, `handleCommand`, `describeBlockage`), against
`internal/engine`'s `State`; pinned by `internal/gateway/authz_test.go`,
`grant_validate_test.go`, `add_actor_validate_test.go`,
`note_validate_test.go`, `convert_test.go`, `server_test.go`,
`server_visibility_test.go` and `server_internal_test.go`, and by
`cmd/vtt/scenario_goldens_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: who may do what is decided on the server, per command, against the
fold as it stands when the command arrives, never by a client and never once
at connection time.

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
| `remove_token` | yes | yes | no | no |
| `remove_actor` | yes | yes | no | no |
| `start_session` | yes | yes | no | no |
| `end_session` | yes | yes | no | no |
| `use_ability` | yes | yes | `authorizeActorOwnership` | no |
| `remove_condition` | yes | yes | `authorizeActorOwnership` | no |
| `add_narration` | yes | yes | `unrestricted` | no |
| `upsert_note` | yes | yes | no | no |
| `delete_note` | yes | yes | no | no |
| `load_adventure` | yes | yes | no | no |
| `load_map` | yes | yes | no | no |
| `grant_actor_control` | yes | yes | no | no |
| `revoke_actor_control` | yes | yes | `authorizeSelfRevoke` | no |
| `promote_participant` | yes | yes | no | no |
| `set_join_door` | yes | yes | no | no |
| `rotate_join_link` | yes | yes | no | no |
| `open_door` | yes | yes | `mayWorkDoor` | no |
| `close_door` | yes | yes | `mayWorkDoor` | no |
| `set_viewpoint` | no | no | no | yes |

A player cell names the `playerRules` entry that decides it; every other
"yes" is the whole of the role's answer from the table. The DM and the agent
hold every row but `set_viewpoint`; `set_viewpoint` is the spectator's only
cell. Removal authors the board and the cast, so `remove_token` holds
`place_token`'s cells and `remove_actor` holds `add_actor`'s: a player may
remove neither a token nor an actor they control. Granting control,
promotion and the join door are the DM's and the agent's alone, since each
confers standing at a table whose shared link admits spectators (SPEC-009,
VTT-028). Every refusal `Authorize` returns wraps `ErrUnauthorized`: every
`fmt.Errorf` in `authz.go` and `viewpoint.go` wraps it with `%w`.
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

**Two checks for every role.** After the table and before the player half,
`Authorize` runs `authorizePromotionTarget` for `promote_participant`, which
accepts a target role of player or spectator and refuses every other, for
the DM's promotions as for anyone's; SPEC-009 states that bound and
`handlePromotion`'s bound on the target's current role. For `set_viewpoint`
it runs `MayPerch`, whose rule is SPEC-015's. Both run for every role,
so the DM is bound by promotion's bound and a spectator, whom the player half
never reaches, is bound by `MayPerch`.

**The player half.** For every role but player, `Authorize` returns after
those two checks. For a player it calls `authorizePlayer` with the name it
derived and the command together, and a mismatched pair would judge a command
by another's rule; `Authorize` is its one production caller (`grep -rn
'authorizePlayer(' internal`). `authorizePlayer` looks the name up in
`playerRules`, a table keyed like `commandRoles` whose entries are
`playerRule` functions run against the participant, the command and the
snapshot. A player cell with no entry is refused by `errUndecided`, whose
message names the command. `unrestricted` is the entry for `add_narration`,
so a command that needs no rule says so in the table rather than by its
absence. `TestEveryPlayerCommandHasARule` holds that every player cell has an
entry and every entry a player cell; while the two agree no command reaches
the missing-rule arm through `Authorize`, and `AuthorizeUndecidedForTest` in
`export_test.go` reaches it directly.

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
- `add_narration`: `unrestricted` accepts.

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
every role but player, and `handleCommand`'s move gate runs for a player
only. They act on, and grant and revoke control of, an actor a player
controls, and acting on it leaves its `controller_ids` as they were; they
work a door from anywhere; they move a token onto any square, a wall and a
square no player can see included. The table's cells and the two checks
for every role bind them.

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
2. `validateGrantActorControl`, for a `grant_actor_control`;
3. `validateAddActor`, for an `add_actor`;
4. `validateUpsertNote`, for an `upsert_note`;
5. the dispatch of `use_ability`, `load_adventure`, `load_map`,
   `remove_actor`, `promote_participant`, `set_join_door` and
   `rotate_join_link` to their handlers;
6. `ToEvent`, which makes every other command one envelope stamped with the
   participant's id and role;
7. for a `TokenMoved`, the backfill of `SceneId` and `From` from the
   snapshot's token;
8. `campaign.Append`, which folds a clone of the envelope and returns the
   fold's refusal before anything is persisted;
9. an ok=true result carrying the sequence `campaign.Append` assigned.

Every refusal is an ok=false `CommandResult` whose `Error` is the refusal's
text. The read loop queues it on the issuing connection and reads the next
command: no refusal on this path closes the connection, and a refused command
appends nothing, so no other connection sees it. `handleCommand` never closes
the connection or writes to the wire: neither it nor `answerCommand` is
handed the connection.

**The player's move gate.** For a player's `move_token` whose token
`st.Tokens` holds, `handleCommand` first asks `canSee(viewerFor(p), st,
tok.SceneID, to)` and, when it answers false, refuses with `gateway: cannot
move there — you cannot see that square`, whatever stands on the square. Only
then does it ask `st.Blocked(tok.SceneID, x, y)`, and refuses a blocked
square with `gateway: cannot move there — ` followed by
`describeBlockage(why)`. `engine.State.Blocked` answers for every square of a
scene, seen or not, so the order is what keeps an unseen square's refusal one
string that names no wall, door or scenery. A square out of the player's sight
is refused even when they remember its terrain. A token the snapshot does not
hold never reaches the gate, since `authorizeTokenOwnership` has refused it.
`canSee` and `viewerFor` are SPEC-015's; what a projection sees is SPEC-016's.

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
told about the controller. `engine.Apply` refuses an `ActorAdded` that names
a controller as well, and accepts one with no kind, which on a recorded event
reads as not a party member; the kind refusal exists at the command boundary
alone. `validateUpsertNote` refuses an `upsert_note` whose `visibility` is
`NOTE_VISIBILITY_UNSPECIFIED`, with a message naming `visibility`, and accepts
every other value, so the enum may grow without a note being refused.
`engine.Apply` accepts a `NoteUpserted` with no visibility, which the
projection sends to no player or spectator (SPEC-016), so this refusal too
exists at the command boundary alone.

**What is dispatched, and what becomes one envelope.** `use_ability`
(`handleUseAbility`) and `load_adventure` (`handleLoadAdventure`) are
SPEC-012's; `load_map` (`handleLoadMap`) is SPEC-014's; `remove_actor`
(`handleRemoveActor`) is SPEC-017's; and SPEC-007 states what the
`load_map` and `remove_actor` batches carry. All four append a whole batch
through `campaign.AppendBatch`. `promote_participant` (`handlePromotion`),
`set_join_door` (`handleJoinDoor`) and `rotate_join_link`
(`handleRotateJoinLink`) are SPEC-009's and append nothing (SPEC-007), so
their ok=true result carries no sequence. Every other command reaches
`ToEvent` and becomes one envelope, carrying every field the command gave
that the event has a field for, a grant's `kind` included; `move_token`'s
`reason` has none in `TokenMoved` and is dropped. For a `TokenMoved`, `handleCommand` sets
`SceneId` and `From` from the snapshot's token before `campaign.Append`, so
the log records the scene and the square the token left; `engine.Apply`'s
`TokenMoved` arm reads only `TokenId` and `To`.

**`set_viewpoint`.** `answerCommand` routes it to `handleSetViewpoint`, which
runs `authorize` (the spectator's cell and `MayPerch`), then
`perches.set(actorID)`, and answers ok=true. It appends nothing (SPEC-007)
and projects nothing itself: `perchBox.set` records the shoulder and wakes
the pump without blocking, and the pump takes the shoulder and applies it
(SPEC-011). An ok result means the shoulder is recorded, not that its frames
are sent, and a shoulder replaced before the pump takes it is never applied.
`serve` makes `perches` per connection, so a perch ends with its connection.

**What this record does not decide.** Promotion's bounds and the
re-resolution before every command are SPEC-009's; the perch's rule and what
a seat is sent are SPEC-015's; the batch handlers are SPEC-012's, SPEC-014's
and SPEC-017's; the join door and promotion handlers are
SPEC-009's; what the fold refuses is `engine.Apply`'s.

## Consequences

A client or tool author is bound by these:

- A spectator's only command is `set_viewpoint`.
- A player acts only through an actor that counts them among its
  controllers; an actor nobody controls is the DM's and the agent's.
- A player works a door from a controlled token beside it, and moves only
  onto a square they can see now; a square they remember and cannot see is
  refused.
- A refusal is an ok=false result on a connection that stays open, seen by
  the issuer alone. A move refusal's text names nothing the issuer has not
  been sent (VTT-156, VTT-159); no other refusal's text is bounded by this
  record.
- A `grant_actor_control` and an `add_actor` state what the actor is; an
  `add_actor` never confers control, and control is granted afterwards.
- An `upsert_note` states who may read the note.
- The DM and the agent are bound by the table's cells and by promotion's
  bound, and by no player rule.
- A perch does not survive a reconnect; a client re-sends it after
  redialling.

And whoever changes the code:

- A command added to the contract needs a row in `commandRoles`, and a player
  cell needs a `playerRules` entry, or the command is refused.
- A `Blocked` reason that interpolates text from a map file needs the clip
  `describeBlockage` gives the scenery kind.

## Requirements

VTT-138, VTT-139, VTT-140, VTT-141, VTT-142, VTT-143, VTT-144, VTT-145,
VTT-146, VTT-147, VTT-148, VTT-149, VTT-150, VTT-151, VTT-152, VTT-153,
VTT-154, VTT-155, VTT-156, VTT-157, VTT-158, VTT-159, VTT-160, VTT-161,
VTT-162, VTT-231, VTT-257.
