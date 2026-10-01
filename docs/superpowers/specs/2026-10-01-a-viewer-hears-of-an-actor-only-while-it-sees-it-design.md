# A viewer hears of an actor only while it sees it

## The problem

`Projector.look` (`internal/gateway/project.go`) builds the look's actors from
the tokens on squares the viewer's eyes see and then adds every actor
`engine.IsPartyMember` accepts, seen or not. `classify` forwards
`ResourceChanged`, `ConditionApplied`, `ConditionRemoved` and `ActorRemoved`
when `pr.actors` held the actor before the event, and `transitions` removes an
actor from `pr.actors` only when the state no longer holds it, so a viewer that
once saw an actor goes on receiving its resource and condition changes in any
scene, behind any door, for as long as the actor exists. `classify` forwards
`AttackRolled`, `AbilityUsed`, `ActorControlGranted` and
`ActorControlRevoked` when every actor they name is in `pr.actors` or in the
look, and the look holds every party member, so every attack, ability and
control change involving a party member reaches every player and spectator
whatever they see. The 2026-08-18 visibility review recorded this as I2,
"Testimony outlives sight", still open (`docs/reports/2026-08-18-visibility.md`).
The other way round, `eyes` gives a player the actors it controls, but `look`
puts an eye among its actors only when a token of it stands on a seen square
or it is a party member, so a player granted a non-party actor with no token
on the board is never introduced to its own character, and the grant itself
is withheld.

The owner ruled on 2026-09-29 that only what a viewer sees gives them
information, for every actor, party members included
(`docs/superpowers/specs/2026-09-29-the-projection-has-a-record-design.md`,
SPEC-016's Status), and on 2026-10-01: a player's own characters and a
spectator's shoulder are always known, with their status; a party member
nobody's eyes see stays on every roster, its status frozen while unseen; an
actor seen again after changing unseen is corrected on sight, by bare frames
carrying its present status and no reason or author; and an attack, ability
or control change naming any actor the viewer does not see now is withheld
entirely.

Not in this ticket: who may attack or use an ability on an actor it does not
see (SPEC-013; an attack checks no sight today); range and light in
`internal/sight`; the DM's and the agent's streams, which stay whole; the
client, which folds what it is sent and gains no code.

## Done looks like

1. A `ResourceChanged`, `ConditionApplied` or `ConditionRemoved` for an actor
   the viewer once saw and does not see now reaches no player or spectator:
   named tests in `internal/gateway/project_test.go` for a creature behind a
   door that has closed and for a party member in another scene; both fail on
   today's tree, where `pr.actors` holding the actor forwards it.
2. An `AttackRolled`, `AbilityUsed`, `ActorControlGranted` or
   `ActorControlRevoked` that names any actor the viewer does not see now
   reaches no player or spectator, a party member out of sight included; a
   named test for an unseen party member's attack fails on today's tree.
3. The same payloads naming only actors the viewer sees now still reach it,
   and "sees now" counts a viewer's own eyes (a player's controlled actors, a
   spectator's shoulder) whether or not a token of them stands on a seen
   square: named tests show a party member's change seen across a room, and
   the player's own token-less character's resource change, arriving.
4. A player granted control of a non-party actor with no token on the board
   is introduced to it and receives the grant: a named test fails on today's
   tree.
5. A party member is still introduced to every player and spectator, seen or
   not, and an unperched spectator's fold still holds every party member, so
   the shoulder list has something to show (VTT-208 and its tests stay green).
6. When an actor comes into a viewer's sight, by an event or by a perch, after
   its resources, conditions or controllers changed while the viewer did not
   see it, the viewer is sent frames that bring its fold to the actor's
   present status (resources, conditions, controllers and kind), carrying
   that event's or the perch's sequence and no reason, source or author; a named test shows a party member's damage taken
   out of sight arriving when the viewer's eye walks into the room, and every
   projected fold in the property and keystone tests equals the server's
   status for each actor the viewer sees.
7. An actor removed from the world while the viewer does not see it is not
   reported to that viewer, and the world reusing its id afterwards still
   folds for that viewer: named tests.
8. Every projected golden under `scenarios/goldens/*/projections/` whose
   stream changes is re-derived: `state.json` by hand, `stream.json` from the
   projection's emitted output read before it is committed, `viewer.json`'s
   `why` made true; the keystone and both parity tests pass.
9. SPEC-016 describes the new behaviour: its Status carries no pending owner
   decision, its payload rulings, what the look counts as seen, the
   correction on sight, and its Consequences; requirement rows that the
   ruling makes false are withdrawn and their replacements dispensed; every
   rule the sort accepts has a row cited by a test; `task check` whole is
   green.

## Rules this puts on the system

- A resource or condition change reaches a player or spectator only while it
  sees the actor.
- An attack, an ability or a control change reaches a player or spectator only
  when it sees every actor the payload names.
- A viewer's own eyes count as seen, with or without a token on a seen square.
- A player is introduced to every actor it controls.
- A party member is introduced to every player and spectator, seen or not
  (VTT-208 holds this already).
- An actor that comes into sight is brought to its present resources,
  conditions, controllers and kind in the viewer's fold.
- A frame that brings an actor up to date carries no reason, source or author.
- An actor's removal reaches only a viewer that saw the actor before the
  removal.
- An actor id the world reuses is introduced afresh to a viewer that missed
  its removal (VTT-211 holds the case of a viewer that did not miss it).

## What it touches

In this order: the projection, its tests, the goldens, the records.

1. `internal/gateway/project.go`: `look` (what is seen against what is
   known), `classify`'s testimony arms, `transitions` (the correction on sight
   and the introduction of a player's own actors), and the memory a correction
   needs.
2. `internal/gateway/project_test.go`, `project_internal_test.go`,
   `project_property_test.go`, `keystone_test.go`,
   `viewpoint_internal_test.go`, `server_visibility_test.go`; tests there that
   pin today's testimony (`TestAConditionAppliedOutOfSightArrivesWithTheActor`,
   `TestARemovedActorReachesOnlyTheSeatThatHeldIt`,
   `TestAnActorIdUsedAgainAfterRemovalIsIntroducedAfresh`,
   `TestAnEventNamingAnUnknownActorIsWithheld`) are read and moved where the
   ruling moves them.
3. The twelve projected seats under `scenarios/goldens/*/projections/`
   (`adventure-night/act-fighter`, `door-watch/act-latecomer`,
   `door-watch/act-watcher`, `session-zero/act-fighter`, `session-zero/player`,
   `session-zero/spectator`, `shared-control/act-scout`,
   `shared-control/act-warden`, `story-table/act-hero`,
   `three-role-exit/act-lera`, `toy-brawl/act-brawler`, `toy-brawl/act-patron`).
4. `contract/testdata/removal_batch_projected_stream.json`, which
   `TestARemovalBatchProjectsToTheBytesBothFoldsRead` and
   `client/test/removal-batch-parity.test.ts` read; `internal/eventgen`
   (`model.go`, `model_test.go`), whose generated actors the property test
   walks.
5. `docs/specifications/016-the-projection.md`; `docs/requirements.md`
   (VTT-207 and VTT-215 are read against the ruling);
   `tools/mutation-equivalents.txt` and `tools/comment-ceilings.txt` as the
   edits move them.

## Specifications this moves

docs/specifications/016-the-projection.md

## What could not be established

- Which register rows the ruling makes false. VTT-207 says an actor that is
  not a party member is introduced only once a token of it is seen, "whoever
  controls it", which the first 2026-10-01 ruling contradicts for a player's
  own actor; VTT-215 says a removal reaches a viewer that held the actor. Both
  are read against the final rules in the sort; how a row is withdrawn is
  SPEC-008's.
- What the correction needs to remember per viewer and actor, and whether a
  resource's maximum can change: `ResourceChanged` carries `new_value` and no
  maximum.
- How a viewer that missed a removal is told when the id returns: the fold
  refuses a second `ActorAdded` for an id it holds.
- A player may name an actor it does not see as an attack's target, since no
  command checks sight but a move; under this ticket such an attack's
  `AttackRolled` reaches the attacker's player only if it sees the target.
  Whether that is refused at the command is SPEC-013's and not this ticket's.
- Whether every golden moves: those whose seats see every actor they hear of
  may not.
