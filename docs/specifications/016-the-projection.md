# SPEC-016: What a player or a spectator is sent of each event is decided by the projection

## Status

Accepted. Implemented by `internal/gateway/project.go` (`Viewer`, `Projector`,
`NewProjector`, `Project`, `perchSequence`, `reperch`, `sightView`, `look`,
`eyes`, `transitions`, `sceneSeenFor`, `objectInSight`, `verdict`, `passIf`,
`classify`, `canSeeSquare`, `doorTransitions`, `doorSubject`, `squareAt`,
`squareKey`, `sortedSet`, `sortedSceneIDs`, `sortedSceneIDsUnion`,
`noteTransitions`, `introduce`, `withdraw`, `actorBelief`, `beliefOf`,
`snapshotSighted`, `correct`, `statusFrames`, `sameSet`), over
`internal/sight`'s `VisibleFrom` and `engine.IsPartyMember`; called from
`internal/gateway/seat.go` (`newSeat`, `receive`, `perch`, `canSee`); pinned
by `internal/gateway/project_test.go`, `project_internal_test.go`,
`project_property_test.go`, `keystone_test.go`, `viewpoint_internal_test.go`,
`server_visibility_test.go`, `qa_note_projection_test.go`,
`qa_testimony_eyes_test.go`, `qa_testimony_sight_test.go` and
`qa_testimony_removal_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The one
this record would cite is absent from the record rather than from the system:
a player or a spectator is sent of each event only what their eyes see, or the
table is told, decided on the server, and when the projection cannot tell, it
sends nothing.

## How it works

**Which viewers are projected, and what `Project` answers.** `Project` answers
a nil event with nothing. It answers `identity.RoleDM` and
`identity.RoleAgent` with the event itself, the same pointer, and reads no
state. It answers any role but those and `identity.RolePlayer` and
`identity.RoleSpectator` with nothing; `identity.Role` is a string, and
`identity.Verify` keeps a stored role that does not parse from reaching a seat
(SPEC-009). For a player or a spectator it answers a nil state with nothing,
and otherwise computes `look` over the state and asks `classify` for a
verdict. For `unrecognised` it sends nothing at all, `transitions`' frames
included, since it cannot tell what the event did to the world; for `withheld`
it sends `transitions`' frames; for `forwarded`, those frames and then the
event. `Project` writes to neither the event nor the state, and every frame it
builds is new (each `&vttv1.Envelope{`, and each element of a
`[]*vttv1.Envelope{{` literal, in `project.go`): a live event is one envelope,
since `store.notifyLocked` enqueues the same pointer to every subscriber.
Which seats call `Project` (`seat.receive` alone: `grep -rn '\.Project('` over
the non-test Go files) and with which state is SPEC-015's.

**Whose eyes a viewer has.** `eyes` gives a player every actor in `st.Actors`
whose `ControllerIds` holds the player's participant id, sorted, and ignores
`Viewpoint`, which comes from the client. It gives a spectator the actor its
`Viewpoint` names when `st.Actors` holds it and `engine.IsPartyMember` accepts
it, and otherwise none, the empty viewpoint included. `MayPerch` refuses a
player's perch and a non-party shoulder first (SPEC-015), and `eyes` refuses a
non-party shoulder again with the same predicate. Every other role has none.

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
for a missing position.

**What the projector remembers.** `NewProjector` gives a `Projector` its maps,
each empty. `scenes` holds the scenes introduced to the viewer and loses none
(`grep -n 'delete(pr.scenes' internal/gateway/project.go` prints nothing, and
no code under `internal/` deletes from a state's `Scenes`). `actors` holds the
actors introduced, and loses one only when `st` no longer holds it and
`sighted` does, or when `withdraw` takes it out before an introduction (`grep
-n 'delete(pr.actors' internal/gateway/project.go` prints those two lines).
`gone` marks an actor in `actors` that the world removed while `sighted` did
not hold it: the viewer was not told, so it stays in `actors` with its belief
frozen until `withdraw`, even once `st` holds the id again. `tokens` holds the
tokens on the viewer's board now. `seen` holds, per scene, the visible set
last sent. `doors` holds, per scene, the door squares the viewer believes
open. `notes` holds the keys of the notes the viewer holds; `noteTransitions`
removes a key when `st` no longer holds the note or holds it with any
visibility but `NOTE_VISIBILITY_PUBLIC`. `sighted` holds the look's `sees` as
the last `transitions` left it, by an event or by a perch. `belief` holds, per
actor in `actors`, an `actorBelief`: the current value of each resource, its
condition ids, its controllers and its kind, the four things of an actor an
event can change, since `ActorAdded` alone writes its name, attributes and
each resource's maximum. A belief is written by `beliefOf` from `st` only
where the viewer's fold equals `st` for that actor: when `introduce` sends it
or `correct` brings it up to date, and, in `snapshotSighted` after every
`transitions`, for each actor in `sighted`, every event about which was
forwarded, and each in the new `sees`, which an introduction or a correction
has just brought up to date. So an actor no eye sees keeps the belief it had
when the viewer last saw it. The maps are a function of the whole log prefix
the projector was fed, of the viewer, and of the perches applied along the
way, not of the current state: an actor seen and then hidden stays in
`actors`, which no state records. So a projector is fed the log from its first
event and never seeded from a state; the seat feeds it so (SPEC-015).

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
reported dark once; when the set last sent was already empty, nothing is sent
and the scene stays in `seen`. Each scene therefore precedes the doors, tokens
and `SceneSeen` in it, and each actor its grants, conditions and tokens, the
order `engine.Apply` and `client/src/fold.ts` both require. A token's
departure precedes any arrival, which no fold requires, so the board never
holds a departing token and an arriving one at once. Every walk that emits
frames is over a sorted set or a slice, so one log projects to one stream of
frames. Last, `noteTransitions` walks `notes` by key: a key whose note `st` no
longer holds, or holds with any visibility but `NOTE_VISIBILITY_PUBLIC`, is
forgotten and sent as a `NoteDeleted` carrying that key and the event's
sequence and nothing else; and the key of a causing `NoteUpserted` whose
visibility is `NOTE_VISIBILITY_PUBLIC` is remembered. A perch changes no note,
so it sends no note frame.

**The correction on sight.** `correct` hands `statusFrames` the actor's belief
and its present state, and sends what it returns, each frame carrying the
event's sequence and nothing else of an envelope, no event id, time, role,
participant or session, and no reason or source: one `ResourceChanged` per
resource whose current value differs, by name, with `delta` the difference and
`new_value` the present value, and, where the difference does not fit `int32`,
first one with `delta` 0 and `new_value` 0, which the fold's floor makes true
from a value below zero; one `ConditionRemoved` per condition the belief holds
and `st` does not, in the belief's order, then one `ConditionApplied` per
condition `st` holds and the belief does not, in `st`'s order; one
`ActorControlGranted` per controller `st` holds and the belief does not,
carrying the present kind, and, when the kind differs and no such grant is
sent, one re-stating the belief's first controller with the present kind; then
one `ActorControlRevoked` per controller the belief holds and `st` does not.
Conditions and controllers are compared as sets, so a change undone while the
viewer did not see it sends nothing. When the belief's resources are not the
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
board, roster and `sighted` it reads are the viewer's before the event, and
the look is the one after it. It forwards `SessionStarted`, `SessionEnded` and
`NarrationAdded` to every player and spectator; narration is addressed to the
table, and `add_narration` is open to the player, the DM and the agent
(SPEC-013). It withholds `SceneCreated`, `ActorAdded` and `TokenPlaced`, which
`transitions` introduces; `TokenRemoved`, since `transitions` sends a viewer
that held the token a `TokenHidden` once `st` no longer has it; `TokenHidden`
and `SceneSeen`, which only the projection issues (`grep -rn
'Envelope_TokenHidden{\|Envelope_SceneSeen{'` over the non-test Go under
`internal/` and `cmd/` prints only `project.go`); and `AdventureLoaded`, a
no-op for `engine.Apply` whose batch's events are each projected on their own.
It forwards `NoteUpserted` when its visibility is `NOTE_VISIBILITY_PUBLIC` and
withholds it for every other value, `NOTE_VISIBILITY_UNSPECIFIED` included, so
a note recorded without a visibility is DM-only. It withholds `NoteDeleted`:
`noteTransitions` sends a viewer that holds the note the same bare
`NoteDeleted` a note made secret gets, and a viewer that does not hold it
nothing, since its fold would refuse the deletion and a key the viewer does
not hold is the DM's. It forwards `TokenMoved` when the token was on the board
before the event and is in the look after it, since a move names both its
ends. It forwards `DoorOpened` and `DoorClosed` when `canSeeSquare` finds the
square in the look. It forwards `AttackRolled` (attacker and target),
`AbilityUsed` (actor and targets), `ActorControlGranted`,
`ActorControlRevoked`, `ResourceChanged`, `ConditionApplied` and
`ConditionRemoved` when every non-empty actor id named is in `sighted`, so
when the viewer saw each of them before the event; what an event brings into
sight arrives by introduction or by correction instead. It forwards
`ActorRemoved` on the same test, so when the viewer saw the actor before the
event; a viewer that did not see it keeps it, gone, and a fold that never held
it would refuse its removal. Every other payload, and an envelope with none,
is `unrecognised`; `TestEveryEnvelopePayloadArmHasAnExplicitRuling` walks the
envelope's oneof and fails on a payload `classify` answers `unrecognised`.

**A perch.** `reperch` sets `Viewpoint` to the actor named, answers a nil
state with nothing, and otherwise returns `transitions` with no causing event
and `perchSequence` against a fresh look. So what the new eyes see that the
projector's memory lacks is introduced, a held actor they see that `sighted`
does not hold is corrected, the new shoulder itself included, a token they do
not see leaves the board as a `TokenHidden`, a scene no eye stands in any more
is reported dark, and no terrain is withdrawn, since no frame withdraws it.
What the memory never held is served in full whenever it comes into sight, so
a shoulder a burst of perches flew past (SPEC-015) is served in full when
named again. When a perch runs, and against which state, is SPEC-015's.

**The sequence every frame carries.** `Project` hands `transitions` the
event's sequence, and every frame it builds carries it, as the forwarded event
does: one event's frames share one sequence, and a condition an introduction
carries, or a change a correction carries, bears that frame's sequence, not
the one it was applied at. `reperch` hands `transitions` `perchSequence`,
which is 0. What a resume cursor does with either is SPEC-015's.

**What this record does not decide.** Which seats are projected, the state a
seat hands `Project`, what its resume cursor drops, when a perch is taken and
against which state, and `canSee` are SPEC-015's. What a square can see
(walls, closed doors, objects that block sight, range and tolerance) is
`internal/sight`'s, which has no record. The payloads and their fields are
SPEC-007's; delivery is SPEC-011's; who may issue each command, the move
gate, and the refusal of an `upsert_note` that names no visibility are
SPEC-013's; what a party member is, is `engine.IsPartyMember`'s;
what each fold refuses is `engine.Apply`'s and `client/src/fold.ts`'s.

## Consequences

A client author, and whoever changes the code, are bound by these:

- A projected stream folds in the order it is sent, and several of its frames
  share one sequence.
- A viewer learns of a scene, an actor or a token only from the projection's
  introductions, and is never introduced to one it holds; both folds refuse a
  second. A token that leaves sight is placed again when it returns, and an
  actor id the world uses again is introduced afresh.
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
  arrival, a door's correction, a `SceneSeen` or a note's bare `NoteDeleted`.
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
  with the status it had when last seen, until its id is introduced again; a
  bare `ActorRemoved` precedes that introduction and is sent at no other time
  but a correction's re-introduction.
- The conditions and controllers a correction brings need not keep the
  server's order, and a corrected condition carries no source.
- A player's and a spectator's fold holds exactly the notes whose visibility
  is `NOTE_VISIBILITY_PUBLIC`. A note that stops being public and a note that
  is deleted reach them as the same bare `NoteDeleted`, so they cannot tell
  the two apart.
- A payload added to the contract needs an arm in `classify`, or no player or
  spectator is sent it and its event derives nothing for them.
- Nothing may write to an envelope a seat is handed: a live event is one
  envelope shared by every seat. Nor to the state `Project` reads, which is
  the seat's own fold and which `perch` reads again.
- Sight range and tolerance are not supplied. A ruleset that supplies them
  passes them as arguments to `sight.VisibleFrom`; read off
  `Actor.Attributes`, they would be game-system vocabulary in platform code.

## Requirements

VTT-194, VTT-195, VTT-196, VTT-197, VTT-198, VTT-199, VTT-200, VTT-201,
VTT-202, VTT-203, VTT-204, VTT-205, VTT-206, VTT-208, VTT-209, VTT-210,
VTT-211, VTT-212, VTT-213, VTT-214, VTT-215, VTT-216, VTT-217, VTT-218,
VTT-219, VTT-220, VTT-221, VTT-222, VTT-223, VTT-224, VTT-225, VTT-226,
VTT-227, VTT-233, VTT-234, VTT-235, VTT-236, VTT-237, VTT-238, VTT-239,
VTT-241, VTT-242, VTT-243, VTT-244, VTT-245, VTT-246, VTT-247, VTT-248,
VTT-249, VTT-250, VTT-251, VTT-252.
