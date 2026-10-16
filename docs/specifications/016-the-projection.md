# SPEC-016: What a player or a spectator is sent of each event is decided by the projection

## Status

Accepted. Implemented by `internal/perceive/projection.go` (`Eyes`,
`Projection`, `New`, `Resume`, `Event`, `Perch`, `CanSee`, `recall`,
`sameIncarnation`,
`trueKeys`, `sightView`, `look`, `eyes`, `transitions`, `sceneSeenFor`,
`objectInSight`, `verdict`, `passIf`, `classify`, `canSeeSquare`,
`doorTransitions`, `doorSubject`, `squareAt`, `squareKey`, `sortedSet`,
`sortedSceneIDs`, `sortedSceneIDsUnion`, `noteTransitions`, `introduce`,
`withdraw`, `actorBelief`, `beliefOf`, `correct`, `statusFrames`, `sameSet`,
`forwardable`), over `internal/sight`'s `VisibleFrom` and
`engine.IsPartyMember`, and by `internal/gateway/project.go` (`Viewer`,
`Projector`, `NewProjector`, `eyesFor`, `Project`, `perchSequence`,
`reperch`), over `campaign.FoldPrefix`; called from
`internal/gateway/seat.go` (`newSeat`, `receive`, `perch`, `canSee`) and,
for the character logs, from `internal/campaign/fanout.go`
(`loadCharacterLogsLocked`, `fanOutLocked`, `restartDecisionsLocked`,
`opening`; SPEC-021); pinned
by `internal/perceive/board_test.go`, `roster_test.go`, `status_test.go`,
`history_test.go`, `unseen_test.go`, `resume_from_fold_test.go`,
`deps_test.go`,
`projection_internal_test.go` and `qa_testimony_eyes_test.go`, through the
harness in `viewer_test.go`, and by
`internal/gateway/project_test.go`, `project_internal_test.go`,
`project_property_test.go`, `keystone_test.go`, `viewpoint_internal_test.go`,
`server_visibility_test.go`, `resume_test.go`, `history_fold_test.go`,
`qa_fold_memory_test.go`, `qa_note_projection_test.go`,
`qa_note_adventure_test.go`,
`qa_testimony_eyes_test.go`, `qa_testimony_sight_test.go`,
`qa_testimony_removal_test.go`, `qa_move_reason_test.go` and
`qa_issuer_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The one
this record would cite is absent from the record rather than from the system:
a player or a spectator is sent of each event only what their eyes see, or the
table is told, decided on the server, and when the projection cannot tell, it
sends nothing.

## How it works

**Which viewers are projected, and what `Project` answers.** `Project` answers
a nil event with nothing. It answers `identity.RoleDM` and `identity.RoleAgent`
with the event itself, the same pointer, and reads no state. It answers any
role but those and `identity.RolePlayer` and `identity.RoleSpectator` with
nothing; `identity.Role` is a string, and `identity.Verify` keeps a stored role
that does not parse from reaching a seat (SPEC-009). For a player or a
spectator it answers a nil state with nothing; otherwise it folds the frames
it has sent with `campaign.FoldPrefix`, which is the viewer's fold, answers
nothing when that fold fails (below), and hands `Projection.Event` the
viewer's eyes, the event, that fold and the state, and adds what it returns to
the frames it has sent. `Event` rebuilds its memory from the viewer's fold
(`recall`), computes `look` over the state and asks `classify` for a verdict.
For `unrecognised` it sends nothing at all, `transitions`' frames included,
since it cannot tell what the event did to the world; for `withheld` it sends
`transitions`' frames; for `forwarded`, those frames and then
`forwardable`'s answer: a copy of the event with its `participant_id` and
`actor_role` cleared, a `TokenMoved`'s or a
`ResourceChanged`'s `reason`, a `ConditionApplied`'s `source` and a
`ConditionRemoved`'s `reason` cleared, and every other field as the event has
it. No envelope `Project` sends a player or a spectator carries its own
`participant_id` or `actor_role`, which name who issued the event; a control
event's `participant_id` is its subject and is sent. Presence names every
connected participant (SPEC-011). `Event` writes to none of the event, the
viewer's fold and the state, and every frame it builds is new (each
`&vttv1.Envelope{`, each element of a `[]*vttv1.Envelope{{` literal, and
`forwardable`'s `proto.Clone`, in `projection.go`): a live event is one
envelope, since `store.notifyLocked` enqueues the same pointer to every
subscriber. Which seats call `Project` (`seat.receive` alone: `grep -rn
'\.Project('` over the non-test Go files) and with which state is SPEC-015's.

**Whose eyes a viewer has.** `eyesFor` turns a `Viewer` into `perceive.Eyes`:
a player's participant id as `Controller`, ignoring `Viewpoint`, which comes
from the client; a spectator's `Viewpoint` as `Shoulder`; and for every other
role neither. `eyes` gives a `Controller` every actor in `st.Actors` whose
`ControllerIds` holds it, sorted. Without one, it gives a `Shoulder` the actor
it names when `st.Actors` holds it and `engine.IsPartyMember` accepts it, and
otherwise none, the empty shoulder included. `MayPerch` refuses a player's
perch and a non-party shoulder first (SPEC-015), and `eyes` refuses a
non-party shoulder again with the same predicate. `perceive` knows no role:
it does not import `internal/identity` (VTT-311).

**What a look sees.** `look` walks each eye's tokens that stand in a scene
`st` holds. Each such scene gets an entry whether or not anything is visible
from there, so standing in a scene is what earns its board. Its squares are
the union, over those tokens, of `sight.VisibleFrom(scene, x, y,
sightRangeNotSupplied, toleranceNotSupplied)`; both are 0, which
`internal/sight` reads as unlimited range and a tolerance of one sample point.
The look's tokens are every token of `st` on one of those squares in its
scene, and its actors are the viewer's eyes, those tokens' actors and every
actor `engine.IsPartyMember` accepts, seen or not, so a player is introduced
to every actor it controls, with a token in sight or none. Its `sees` are the
viewer's eyes and those tokens' actors, the party members no eye sees left
out. `look` runs on every call, keeps no memo, and writes nothing to `st`.
`canSeeSquare` answers whether one square of one scene is in a look, and false
for a missing position. `CanSee` asks it of a fresh `Projection`'s look, so it
records nothing.

**What the projection remembers.** The `Projector` keeps `sent`, every frame
`Project` and `reperch` have returned to a player or a spectator and nothing
else (`grep -n 'pr.sent =' internal/gateway/project.go` prints their two
appends), and folds it at every event and perch it projects; that fold, the
viewer's, is the projection's memory. `Event` and `Perch` start with `recall`, which rebuilds every map but
`sighted` from it: `scenes` holds its scenes; `seen`, per scene, the squares
its `Visible` marks, a scene with none left out; `doors`, per scene, the
squares its `OpenDoors` marks open; `tokens` its tokens; `notes` its notes'
keys; `actors` its actors; and `belief`, per actor, `beliefOf`'s
`actorBelief`: the current value of each resource, its condition ids and its
controllers in the order the fold holds them, and its kind, the four things of
an actor an event can change, since `ActorAdded` alone writes its name, module
id, attributes, module data, and its resources' names and maxima. `recall` marks
`gone` each actor the fold holds that `st` holds as another, one that differs
in a field only `ActorAdded` writes (`sameIncarnation`); the viewer never saw
it removed. Within one call, `transitions` forgets an actor the fold holds and
`st` does not when `sighted` holds it, since `classify` forwarded its
`ActorRemoved`, and marks every other such actor gone; `withdraw` takes an
actor out of `actors`, `belief` and `gone`, and `introduce` puts one into
`actors`, as each sends. So an actor no eye sees keeps, in the fold, the
status it had when the viewer last saw it, and an
actor removed while the viewer did not see it stays there until `withdraw`
sends its bare `ActorRemoved`, which happens only before its id is introduced
again; when `st` adds it again as the same actor, it is not gone, and it is
corrected when it comes into sight. `sighted` alone is carried from call to
call: the look's `sees` as the last `transitions` left it, by an event or by a
perch. `New` starts it empty, or, handed a world, with the `sees` of a look at
that world. `Resume`, handed the viewer's fold and a world, starts it as
`New` does with that world when the fold holds every actor in that look's
`sees` as the world holds it, the same incarnation and one for which
`statusFrames` answers no frame and true, and empty otherwise: a fold that
lags the world for an actor in sight is then corrected rather than forwarded
what it would refuse, which a character log relies on after a dropped frame
set, a failed write and a reopen (SPEC-021). So what a projection sends is a function of the viewer's fold, the
world, the eyes and the last look's `sees`, and a `Projection` built by `New`
from the world at any prefix of the log, and handed the fold of the frames
sent to that point, sends from then on what one fed from the first event
sends (VTT-312). The `Projector` starts from nothing, and the seat feeds it
from the log's first event (SPEC-015).

**When the viewer's fold fails.** When `campaign.FoldPrefix` refuses `sent`,
`Project` logs the event's sequence and the error with `slog.Error`, and
`reperch` the error, and each sends nothing (VTT-314). The frames that broke
the fold were returned by the call that produced them, and every later fold
includes them, so the seat is sent no further event or perch frame on that
connection, as SPEC-015 says of the world's fold.

**What `transitions` sends, in order.** `transitions` first forgets each actor
`actors` holds that `st` does not and `sighted` does, and marks gone each
other one `st` does not hold, sending nothing for either. Then, walking each
set by sorted id, it sends: for each scene in the look not yet introduced, a
`SceneCreated` of its id, name, width and height, with no tile and no object;
`doorTransitions`' frames; for each actor in the look that is gone,
`withdraw`'s bare `ActorRemoved`, carrying the actor's id and the event's
sequence and nothing else, and then, as for each actor in the look not yet
introduced, `introduce`'s frames: an `ActorAdded` of a copy of it with
`ControllerId` and `ControllerIds` cleared, then one `ActorControlGranted` per
controller, in the actor's order, carrying the actor's kind, then one
`ConditionApplied` per condition `st` holds for it, carrying its id and no
source; for each actor in `actors` that the look `sees` and `sighted` does
not, `correct`'s frames, below; a `TokenHidden` for each token on the board
that is not in the look; a `TokenPlaced` of scene, actor and position for each
token in the look that is not on the board; and a `SceneSeen` for each scene
in `seen` or in the look whose visible set differs from the one last sent,
nothing sent counting as empty (`sameSet`). `sceneSeenFor` builds that
`SceneSeen`: the visible squares, sorted; the tile of each visible square that
has one; and every object of the scene any square of whose footprint is
visible, so an object is sent whole, and one with a width or height below one
is never sent, as `sight.Blockers` casts no shadow for it. A scene no eye
stands in any more is sent an empty `SceneSeen` and leaves `seen`, so it is
reported dark once; a scene whose last set sent was empty is not in `seen`,
which `recall` leaves it out of, so nothing is sent. Each scene therefore
precedes the doors, tokens
and `SceneSeen` in it, and each actor its grants, conditions and tokens, the
order `engine.Apply` and `client/src/fold.ts` both require. A token's
departure precedes any arrival, which no fold requires, so the board never
holds a departing token and an arriving one at once. Every walk that emits
frames is over a sorted set or a slice, so one log projects to one stream of
frames. Last, `noteTransitions` walks `notes` by key: a key whose note `st` no
longer holds, or holds with any visibility but `NOTE_VISIBILITY_PUBLIC`, is
forgotten and sent as a `NoteDeleted` carrying that key and the event's
sequence and nothing else. A `NoteUpserted` `classify` forwards reaches
`notes` through the viewer's fold at the next call. A perch changes no note,
so it sends no note frame.

**The correction on sight.** `correct` hands `statusFrames` the actor's belief
and its present state, and sends what it returns, each frame carrying the
event's sequence and nothing else of an envelope, no event id, time, role,
participant or session, and no reason or source. First, one `ResourceChanged`
per resource whose current value differs, in sorted name order, with `delta`
the difference and `new_value` the present value, and, where the difference
does not fit `int32`, first one with `delta` 0 and `new_value` 0, which the
fold's floor makes true from a value below zero. Then one `ConditionRemoved`
per condition the belief holds and `st` does not, in the order the viewer's
fold holds them. Then one `ConditionApplied` per condition `st` holds and the
belief does not, in `st`'s order. Then one `ActorControlGranted` per
controller `st` holds and the belief does not, in `st`'s order, carrying the
present kind, and, when the kind differs and no such grant is sent, one
re-stating the first controller the viewer's fold holds, with the present
kind. Then one `ActorControlRevoked` per controller the belief holds and `st`
does not, in the fold's order. Conditions and controllers are compared as
sets, so a change undone while the viewer did not see it sends nothing, and
the viewer's fold can hold an actor's conditions and controllers, and so the
first controller `engine.Apply` mirrors into `controller_id`, in another order
than `st` holds them. When the belief's resources are not the
actor's, or the kind differs, no grant carries it, and the belief holds no
controller or the present kind is unspecified, `correct` sends `withdraw`'s
bare `ActorRemoved` and then `introduce`'s frames instead; both folds accept
it, since no token of an actor entering sight is on the viewer's board.

**Doors.** For each scene in the look, by id, and each visible square, by key,
where `OpenDoors` in `st` differs from the viewer's belief, `doorTransitions`
sends a `DoorOpened` or a `DoorClosed` at that square and records the new
belief. The square of the door event being projected is recorded and sent no
frame, since `classify` forwards that event when its square is visible. A
perch has no causing event, so `doorSubject` reports no square and every such
square is sent. A key `squareAt` cannot read as two whole integers is skipped.
A door the viewer does not see keeps the viewer's belief, as its terrain does.
`OpenDoors` travels in neither the introduced `SceneCreated` nor `SceneSeen`,
so these frames are how a door opened before a viewer had eyes reaches its
board. `squareKey` builds the key `sight.VisibleFrom` builds, column then row.

**How each payload is ruled.** `classify` runs before `transitions`, so the
board, roster and `sighted` it reads are the viewer's before the event, and the
look is the one after it. It forwards `SessionStarted`, `SessionEnded` and
`NarrationAdded` to every player and spectator; narration is addressed to the
table, and `add_narration` is open to the player, the DM and the agent
(SPEC-013). It withholds `SceneCreated`, `ActorAdded` and `TokenPlaced`, which
`transitions` introduces; `TokenRemoved`, since `transitions` sends a viewer
that held the token a `TokenHidden` once `st` no longer has it; `TokenHidden`
and `SceneSeen`, which only the projection issues (`grep -rn
'Envelope_TokenHidden{\|Envelope_SceneSeen{'` over the non-test Go under
`internal/` and `cmd/` prints only `projection.go`); and `AdventureLoaded`, a
no-op for `engine.Apply` whose batch's events are each projected on their own.
It forwards `NoteUpserted` when its visibility is `NOTE_VISIBILITY_PUBLIC` and
withholds it for every other value, `NOTE_VISIBILITY_UNSPECIFIED` included, so
a note recorded without a visibility is DM-only. It withholds `NoteDeleted`:
`noteTransitions` sends a viewer that holds the note the same bare
`NoteDeleted` a note made secret gets, and a viewer that does not hold it
nothing, since its fold would refuse the deletion and a key the viewer does not
hold is the DM's. It forwards `TokenMoved` when the token was on the board
before the event and is in the look after it, since a move names both its ends.
What `Project` sends of it carries no `reason` (`forwardable`), which its
issuer wrote and the DM and the agent read in the log. It forwards `DoorOpened`
and `DoorClosed` when `canSeeSquare` finds the square in the look. It forwards
`AttackRolled` (attacker and target), `AbilityUsed` (actor and targets),
`ActorControlGranted`, `ActorControlRevoked`, `ResourceChanged`,
`ConditionApplied` and `ConditionRemoved` when every non-empty actor id named
is in `sighted`, so when the viewer saw each of them before the event; what an
event brings into sight arrives by introduction or by correction instead. What
`Project` sends of a `ResourceChanged`, a `ConditionApplied` or a
`ConditionRemoved` carries no `reason` or `source` (`forwardable`): the cause
names an ability, a threshold or a hand, and an ability's user may be an actor
the viewer does not see. It forwards `ActorRemoved` on the same test, so when
the viewer saw the actor before the event; a viewer that did not see it keeps
it, gone, and a fold that never held it would refuse its removal. Every other
payload, and an envelope with none, is `unrecognised`;
`TestEveryEnvelopePayloadArmHasAnExplicitRuling` walks the envelope's oneof and
fails on a payload `classify` answers `unrecognised`.

**A perch.** `reperch` sets `Viewpoint` to the actor named, answers a nil
state with nothing, and otherwise folds `sent` and hands `Projection.Perch`
the new eyes, that fold, the state and `perchSequence`; `Perch` rebuilds its
memory as `Event` does and returns `transitions` with no causing event against
a fresh look. So what the new eyes see that the viewer's fold lacks is
introduced, a held actor they see that `sighted`
does not hold is corrected, the new shoulder itself included, a token they do
not see leaves the board as a `TokenHidden`, a scene no eye stands in any more
is reported dark, and no terrain is withdrawn, since no frame withdraws it.
What the fold never held is served in full whenever it comes into sight, so
a shoulder a burst of perches flew past (SPEC-015) is served in full when
named again. When a perch runs, and against which state, is SPEC-015's.

**The sequence every frame carries.** `Event` hands `transitions` the
event's sequence, and every frame it builds carries it, as the forwarded event
does: one event's frames share one sequence, and a condition an introduction
carries, or a change a correction carries, bears that frame's sequence, not
the one it was applied at. `reperch` hands `Perch` `perchSequence`, which is
0. What a resume cursor does with either is SPEC-015's.

**What this record does not decide.** Which seats are projected, the state a
seat hands `Project`, what its resume cursor drops, when a perch is taken and
against which state, and `canSee` are SPEC-015's. What a square can see
(walls, closed doors, objects that block sight, range and tolerance) is
`internal/sight`'s, which has no record. The payloads and their fields are
SPEC-007's; delivery is SPEC-011's; who may issue each command, the move
gate, and the refusal of an `upsert_note` that names no visibility are
SPEC-013's; what a party member is, is `engine.IsPartyMember`'s;
what each fold refuses is `engine.Apply`'s and `client/src/fold.ts`'s. Which
character logs exist, when the decision runs at append, and what an entry of
one carries are SPEC-021's.

## Consequences

A client author, and whoever changes the code, are bound by these:

- A projected stream folds in the order it is sent, and several of its frames
  share one sequence.
- A viewer learns of a scene, an actor or a token only from the projection's
  introductions, and is never introduced to one it holds; both folds refuse a
  second. A token that leaves sight is placed again when it returns. An actor
  the world adds again is introduced afresh when the viewer no longer holds
  its id, or holds one that differs in a field only `ActorAdded` writes, and
  is corrected otherwise; a party member added again as another is introduced
  at once, seen or not, since it is on every roster.
- A scene once introduced is never withdrawn, and its terrain arrives only as
  it is seen.
- The newest `SceneSeen` of a scene is the viewer's visible set there; an
  empty one means dark and forgets no terrain.
- A party member is on every projected roster, seen or not, with the status
  it had when the viewer last saw it or at its introduction.
- An actor's kind is status like the rest: a viewer that holds an actor keeps
  the kind it last saw until it sees it again, while an actor that becomes a
  party member is introduced, with its present status and controllers, to
  every viewer that does not hold it. Two viewers' rosters can therefore
  differ, and a spectator can be offered a shoulder that is no longer a party
  member, which `MayPerch` refuses (SPEC-015).
- A condition an introduction carries bears the introduction's sequence.
- A viewer hears of a resource, a condition, a control change, an attack or
  an ability only while it sees every actor it names; what changed meanwhile
  arrives as one correction when the actor comes into sight.
- A frame with no event id is the projection's own: an introduction and the
  bare `ActorRemoved` before one, a correction, a token's departure or
  arrival, a door's correction, a `SceneSeen` or a note's bare `NoteDeleted`;
  one with an event id is the event less its issuer and its cause.
- A change undone while the viewer did not see the actor sends nothing when
  it comes into sight.
- A party member removed while a spectator did not see it stays on that
  spectator's shoulder list; `MayPerch` refuses a perch on it as an absent
  actor (SPEC-015).
- A forwarded `ActorRemoved` reaches only a viewer whose eye the actor is: a
  fold refuses the removal of an actor with a token on the board, so
  `remove_actor` takes the tokens first, and an actor no token stands for is
  seen only as an eye.
- An actor removed while the viewer did not see it stays in the viewer's fold,
  with the status it had when last seen. When the world adds its id again as
  another actor, a bare `ActorRemoved` precedes the introduction; added again
  as the same actor, it is corrected when it comes into sight. A bare
  `ActorRemoved` is sent at no other time than these two: before that
  introduction, and before a correction's re-introduction.
- A viewer's fold need not hold an actor's conditions and controllers, nor so
  its `controller_id`, in the server's order, and a corrected condition
  carries no source.
- A player's and a spectator's fold holds exactly the notes whose visibility
  is `NOTE_VISIBILITY_PUBLIC`. A note that stops being public and a note that
  is deleted reach them as the same bare `NoteDeleted`, so they cannot tell
  the two apart.
- A payload added to the contract needs an arm in `classify`, or no player or
  spectator is sent it and its event derives nothing for them.
- Nothing may write to an envelope a seat is handed: a live event is one
  envelope shared by every seat. Nor to the state `Project` reads, which is
  the seat's own fold and which `perch` reads again.
- Every frame a `Projector` returns to a player or a spectator is added to
  `sent`, and nothing else is: its fold stands for what the viewer holds, and
  is the projection's memory.
- A projection can be started at any prefix of the log from the world and the
  fold of what its viewer was sent, and sends what one fed from the first
  event sends.
- A seat whose sent frames stop folding is sent no further event or perch
  frame on that connection.
- No file of `internal/perceive`, test files included, imports a package of
  this module but its own, `contract`, `engine` and `sight`;
  `.go-arch-lint.yml`'s `perceive` component allows the same.
- No envelope a player or a spectator is sent carries its own
  `participant_id` or `actor_role`, and no `TokenMoved`, `ResourceChanged`,
  `ConditionApplied` or `ConditionRemoved` it is sent carries a `reason` or a
  `source`; the DM and the agent are sent each event as the log holds it.
- Sight range and tolerance are not supplied. A ruleset that supplies them
  passes them as arguments to `sight.VisibleFrom`; read off
  `Actor.Attributes`, they would be game-system vocabulary in platform code.

## Requirements

VTT-194, VTT-195, VTT-196, VTT-197, VTT-198, VTT-199, VTT-200, VTT-201,
VTT-202, VTT-203, VTT-204, VTT-205, VTT-206, VTT-208, VTT-209, VTT-210,
VTT-212, VTT-213, VTT-214, VTT-215, VTT-216, VTT-217, VTT-218, VTT-219,
VTT-220, VTT-221, VTT-222, VTT-223, VTT-224, VTT-225, VTT-226, VTT-227,
VTT-233, VTT-234, VTT-235, VTT-236, VTT-237, VTT-238, VTT-239, VTT-241,
VTT-242, VTT-243, VTT-244, VTT-245, VTT-246, VTT-247, VTT-248, VTT-249,
VTT-250, VTT-251, VTT-252, VTT-262, VTT-272, VTT-273, VTT-311, VTT-312,
VTT-313, VTT-314.
