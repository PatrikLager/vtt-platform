# SPEC-016: What a player or a spectator is sent of each event is decided by the projection

## Status

Accepted. Implemented by `internal/gateway/project.go` (`Viewer`,
`Projector`, `NewProjector`, `Project`, `perchSequence`, `reperch`,
`sightView`, `look`, `eyes`, `transitions`, `sceneSeenFor`,
`objectInSight`, `verdict`, `passIf`, `classify`, `canSeeSquare`,
`doorTransitions`, `doorSubject`, `squareAt`, `squareKey`, `sortedSet`,
`sortedSceneIDs`, `sortedSceneIDsUnion`, `sameSet`), over
`internal/sight`'s `VisibleFrom` and `engine.IsPartyMember`; called from
`internal/gateway/seat.go` (`newSeat`, `receive`, `perch`, `canSee`); pinned
by `internal/gateway/project_test.go`, `project_internal_test.go`,
`project_property_test.go`, `keystone_test.go`, `viewpoint_internal_test.go`
and `server_visibility_test.go`.

Two decisions the owner has taken will change this record, and neither is
implemented. A note will carry a visibility flag, public or DM-only; that
changes the notes ruling under "How each payload is ruled" and the
Consequence that a player's notes panel is empty. Only what a viewer sees
will give them information, for every actor, party members included; that
changes the rulings for a payload forwarded when the viewer knows every actor
it names or already holds the actor, whether a party member no eye sees is
introduced, and with what, and the Consequence that a party member is on
every projected roster, seen or not. Each is carried by a ticket of its own,
the notes flag first and the testimony rule second, and neither ticket is
written yet; both decisions are recorded in
`docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`.
Until each lands, every sentence below describes the code, and the ticket
that lands it rewrites this paragraph and the sentences it names.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: a player or a spectator is sent of each event only what their eyes
see, their roster already holds, or the table is told, decided on the server,
and when the projection cannot tell, it sends nothing.

## How it works

**Which viewers are projected, and what `Project` answers.** `Project`
answers a nil event with nothing. It answers `identity.RoleDM` and
`identity.RoleAgent` with the event itself, the same pointer, and reads no
state. It answers any role but those and `identity.RolePlayer` and
`identity.RoleSpectator` with nothing; `identity.Role` is a string, and
`identity.Verify` keeps a stored role that does not parse from reaching a
seat (SPEC-009). For a player or a spectator it answers a nil state with
nothing, and otherwise computes `look` over the state and asks `classify` for
a verdict. For `unrecognised` it sends nothing at all, `transitions`' frames
included, since it cannot tell what the event did to the world; for
`withheld` it sends `transitions`' frames; for `forwarded`, those frames and
then the event. `Project` writes to neither the event nor the state, and
every frame it builds is new (each `&vttv1.Envelope{` in `project.go`): a live
event is one envelope, since `store.notifyLocked` enqueues the same pointer to
every subscriber. Which seats call `Project` (`seat.receive` alone: `grep -rn
'\.Project('` over the non-test Go files) and with which state is
SPEC-015's.

**Whose eyes a viewer has.** `eyes` gives a player every actor in `st.Actors`
whose `ControllerIds` holds the player's participant id, sorted, and ignores
`Viewpoint`, which comes from the client. It gives a spectator the actor its
`Viewpoint` names when `st.Actors` holds it and `engine.IsPartyMember` accepts
it, and otherwise none, the empty viewpoint included. `MayPerch` refuses a
player's perch and a non-party shoulder first (SPEC-015), and `eyes` refuses a
non-party shoulder again with the same predicate. Every other role has none.

**What a look sees.** `look` walks each eye's tokens that stand in a scene `st`
holds. Each such scene gets an entry whether or not anything is visible from
there, so standing in a scene is what earns its board. Its squares are the
union, over those tokens, of `sight.VisibleFrom(scene, x, y,
sightRangeNotSupplied, toleranceNotSupplied)`; both are 0, which
`internal/sight` reads as unlimited range and a tolerance of one sample
point. The look's tokens are every token of `st` on one of those squares in
its scene, and its actors are those tokens' actors and every actor
`engine.IsPartyMember` accepts, seen or not. `look` runs on every
call, keeps no memo, and writes nothing to `st`. `canSeeSquare` answers
whether one square of one scene is in a look, and false for a missing
position.

**What the projector remembers.** `NewProjector` gives a `Projector` its maps,
each empty. `scenes` holds the scenes introduced to the viewer and loses none
(`grep -n 'delete(pr.scenes' internal/gateway/project.go` prints nothing, and
no code under `internal/` deletes from a state's `Scenes`). `actors` holds the
actors introduced, and `transitions` removes one only when `st` no longer
holds it (`grep -n 'delete(pr.actors' internal/gateway/project.go` prints its
one line). `tokens` holds the tokens on the viewer's board now. `seen` holds,
per scene, the visible set last sent. `doors` holds, per scene, the door
squares the viewer believes open. The maps are a function of the whole log
prefix the projector was fed, of the viewer, and of the perches applied along
the way, not of the current state: an actor seen and then hidden stays in
`actors`, which no state records. So a
projector is fed the log from its first event and never seeded from a state;
the seat feeds it so (SPEC-015).

**What `transitions` sends, in order.** `transitions` first forgets each actor
`actors` holds that `st` does not, and sends nothing for it. Then, walking each
set by sorted id, it sends: for each scene in the look not yet introduced, a
`SceneCreated` of its id, name, width and height, with no tile and no object;
`doorTransitions`' frames; for each actor in the look not yet introduced, an
`ActorAdded` of a copy of it with `ControllerId` and `ControllerIds` cleared,
then one `ActorControlGranted` per controller, in the actor's order, carrying
the actor's kind, then one `ConditionApplied` per condition `st` holds for it;
a `TokenHidden` for each token on the board that is not in the look; a
`TokenPlaced` of scene, actor and position for each token in the look that is
not on the board; and a `SceneSeen` for each scene in `seen` or in the look
whose visible set differs from the one last sent, nothing sent counting as
empty (`sameSet`). `sceneSeenFor` builds that `SceneSeen`: the visible
squares, sorted; the tile of each visible square that has one; and every
object of the scene any square of whose footprint is visible, so an object is
sent whole, and one with a width or height below one is never sent, as
`sight.Blockers` casts no shadow for it. A scene no eye stands in any more is
sent an empty `SceneSeen` and leaves `seen`, so it is reported dark once; when
the set last sent was already empty, nothing is sent and the scene stays in
`seen`. Each scene therefore precedes the doors, tokens and `SceneSeen` in it,
and each actor its grants, conditions and tokens, the order `engine.Apply` and
`client/src/fold.ts` both require. A token's departure precedes any arrival,
which no fold requires, so the board never holds a departing token and an
arriving one at once. Every walk that emits frames is over a sorted set or a
slice, so one log projects to one stream of frames.

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
board and roster it reads are the viewer's before the event, and the look is
the one after it. It forwards `SessionStarted`, `SessionEnded` and
`NarrationAdded` to every player and spectator; narration is addressed to the
table, and `add_narration` is open to the player, the DM and the agent
(SPEC-013). It withholds `SceneCreated`, `ActorAdded` and `TokenPlaced`, which
`transitions` introduces; `TokenRemoved`, since `transitions` sends a viewer
that held the token a `TokenHidden` once `st` no longer has it; `TokenHidden`
and `SceneSeen`, which only the projection issues (`grep -rn
'Envelope_TokenHidden{\|Envelope_SceneSeen{'` over the non-test Go under
`internal/` and `cmd/` prints only `project.go`); `NoteUpserted` and
`NoteDeleted`, which only the DM and the agent may issue (SPEC-013); and
`AdventureLoaded`, a no-op for `engine.Apply` whose batch's events are each
projected on their own. It forwards `TokenMoved` when the token was on the
board before the event and is in the look after it, since a move names both
its ends. It forwards `DoorOpened` and `DoorClosed` when `canSeeSquare` finds
the square in the look. It forwards `AttackRolled` (attacker and target),
`AbilityUsed` (actor and targets), `ActorControlGranted` and
`ActorControlRevoked` when every non-empty actor id named is in `actors` or in
the look (`knows`). It forwards `ResourceChanged`, `ConditionApplied` and
`ConditionRemoved` when `actors` held the actor before the event, since an
introduction already carries the actor's resources and conditions, and
`ActorRemoved` on the same test, since a fold that never held the actor
refuses its removal. Every other payload, and an envelope with none, is
`unrecognised`; `TestEveryEnvelopePayloadArmHasAnExplicitRuling` walks the
envelope's oneof and fails on a payload `classify` answers `unrecognised`.

**A perch.** `reperch` sets `Viewpoint` to the actor named, answers a nil state
with nothing, and otherwise returns `transitions` with no causing event and
`perchSequence` against a fresh look. So what the new eyes see that the
projector's memory lacks is introduced, a token they do not see leaves the
board as a `TokenHidden`, a scene no eye stands in any more is reported dark,
and no terrain is withdrawn, since no frame withdraws it. What the memory
never held is served in full whenever it comes into sight, so a shoulder a
burst of perches flew past (SPEC-015) is served in full when named again. When
a perch runs, and against which state, is SPEC-015's.

**The sequence every frame carries.** `Project` hands `transitions` the
event's sequence, and every frame it builds carries it, as the forwarded event
does: one event's frames share one sequence, and a condition an introduction
carries bears the introduction's sequence, not the one it was applied at.
`reperch` hands `transitions` `perchSequence`, which is 0. What a resume
cursor does with either is SPEC-015's.

**What this record does not decide.** Which seats are projected, the state a
seat hands `Project`, what its resume cursor drops, when a perch is taken and
against which state, and `canSee` are SPEC-015's. What a square can see
(walls, closed doors, objects that block sight, range and tolerance) is
`internal/sight`'s, which has no record. The payloads and their fields are
SPEC-007's; delivery is SPEC-011's; who may issue each command, and the move
gate, are SPEC-013's; what a party member is, is `engine.IsPartyMember`'s;
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
- A party member is on every projected roster, seen or not.
- A condition an introduction carries bears the introduction's sequence.
- A viewer present at a grant that introduces an actor receives the grant
  twice, which both folds accept.
- A player's and a spectator's notes panel is empty.
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
VTT-202, VTT-203, VTT-204, VTT-205, VTT-206, VTT-207, VTT-208, VTT-209,
VTT-210, VTT-211, VTT-212, VTT-213, VTT-214, VTT-215, VTT-216, VTT-217,
VTT-218, VTT-219, VTT-220, VTT-221, VTT-222, VTT-223, VTT-224, VTT-225,
VTT-226, VTT-227.
