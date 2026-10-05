# A viewer is told no issuer and no unseen cause

## The problem

`Projector.Project` in `internal/gateway/project.go` answers a player's or a
spectator's `forwarded` verdict with `forwardable(env)`: the event itself, or,
for a `TokenMoved` that carries a `reason`, a copy with the reason cleared.
Every other field of a forwarded event reaches the seat as the log holds it.
Every envelope the gateway appends carries the issuer: `ToEvent` in
`internal/gateway/convert.go`, and the batch paths in `server.go`,
`adventure.go`, `ruleset.go` and `map.go`, set `participant_id` and
`actor_role` from the issuing participant. So every forwarded frame names the
participant who issued its event and that participant's role. The presence
frames of `internal/gateway/presence.go` tell every connection each connected
participant's id and display name, so a participant id reads as a name.

`classify` forwards a `ResourceChanged`, `ConditionApplied` or
`ConditionRemoved` when the viewer saw the actor it names before the event,
and withholds an `AttackRolled` or `AbilityUsed` that names an actor the
viewer did not see. The status events carry a cause the server writes:
`internal/rules/resolve.go` sets `ResourceChanged.reason`,
`ConditionApplied.source` and `ConditionRemoved.reason` to
`ability:<id>:usage`, `ability:<id>:<phase>` or `threshold:<resource>`, and
`ToEvent` sets a `remove_condition`'s `ConditionRemoved.reason` to `"manual"`.
So a player whose character an unseen creature hurts with an ability is sent
nothing of the `AbilityUsed`, and the `ResourceChanged` on its own character
with the ability's id in `reason` and the issuer's participant id and role on
the envelope: it learns which ability hit it and who issued it. The owner's
ruling of 2026-09-29 is that only what a viewer sees gives it information; of
2026-10-01, that an actor seen again after changing unseen is corrected by
bare frames carrying its present status and no reason or author, and that an
attack or ability naming any actor the viewer does not see now is withheld
entirely (the rulings paragraph of
`docs/superpowers/plans/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it.md`).
`docs/reports/2026-10-01-a-viewer-hears-of-an-actor-only-while-it-sees-it.md`
records this as raised and not yet a ticket; the move-reason sign-off of
2026-10-03 (Q3) added every forwarded frame's issuer to it.

Nothing outside tests reads an envelope's `participant_id` or `actor_role`:
`grep -rn` over the non-test Go files for `.ParticipantId`, `.ActorRole`,
`GetParticipantId()` and `GetActorRole()` finds only their writers and the
fields of the same name on commands and control events, and `client/src`
reads neither. The client's feed (`describe` in
`client/src/view/spectator.ts`) shows no status event's reason or source;
`client/src/fold.ts` keeps a condition's source in the actor's state. The DM
and the agent are sent the log unchanged (VTT-176), and a frame that brings an
actor up to date already carries no issuer, reason or source (VTT-249).

## Done looks like

1. A player whose own character an ability changes, used by an actor the
   player does not see, is sent the change with nothing that names the
   ability or the participant who issued it: a named test under
   `internal/gateway/`, for a `ResourceChanged` and a `ConditionApplied`,
   fails on today's tree and passes after.
2. A player or a spectator is sent no frame whose envelope carries a
   `participant_id` or an `actor_role`: a named test under `internal/gateway/`
   that walks a stream for each fails on today's tree and passes after. A
   control event's own `participant_id`, the participant it grants or revokes,
   is its subject and still reaches a viewer that sees the actor.
3. What a player or a spectator is sent of a status change's cause that it
   did see, an ability whose user it sees or a `"manual"` removal, follows the
   owner's ruling at sign-off, and a named test under `internal/gateway/`
   observes it.
4. The DM and the agent are still sent every event as the log holds it, issuer
   and cause included: `TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection`
   passes before and after, and the log the gateway appends is unchanged.
5. The projected golden streams under `scenarios/goldens/*/projections/`
   change only by the fields the rules above remove, and `task check` whole is
   green.

## Rules this puts on the system

- A player or a spectator is never sent an envelope's `participant_id` or
  `actor_role`, the issuer of its event.
- A player or a spectator is never sent the cause of a status change whose
  cause it did not see.
- Projecting an event writes nothing to the event (VTT-222, which this cites
  rather than restates).

## What it touches

1. `internal/gateway/project.go` (`forwardable`, `Project`) and its tests:
   `project_test.go`, `project_internal_test.go`,
   `project_property_test.go`, `server_visibility_test.go`, and the QA files
   that recognise a forwarded frame as the event itself:
   `qa_move_reason_test.go`, `qa_note_projection_test.go`,
   `qa_testimony_removal_test.go`, `qa_testimony_sight_test.go`
2. `scenarios/goldens/*/projections/*/stream.json`, the twelve projected
   golden streams, which carry issuers and three of which carry a status
   cause
3. `docs/specifications/016-the-projection.md`; `docs/requirements.md`;
   `tools/comment-ceilings.txt`
4. `contract/vtt/v1/events.proto`'s comment on `participant_id`, only if the
   plan rules it should say who is sent it

One component, the gateway's projection; the goldens follow it.

## Specifications this moves

docs/specifications/016-the-projection.md

## What could not be established

- Whether a cause the viewer did see is kept, Done item 3: the ruling of
  2026-10-01 put it as a question ("stripping them changes what a viewer is
  told of events it does see, which no ruling has decided").
- Whether a status change can be told apart by the projection as caused by
  an event the viewer was or was not sent: a `ResourceChanged` names an
  ability, not the event or the actor that used it, and the batch it shares
  with its `AbilityUsed` is projected one envelope at a time.
- Whether `event_id`, `session_id` or `occurred_at` on a forwarded frame names
  anything a viewer did not see; this ticket takes none of them.
- Whether the client needs anything: it reads no issuer and shows no cause,
  so none is expected; the plan confirms it.
