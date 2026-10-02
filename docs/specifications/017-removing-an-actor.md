# SPEC-017: An actor leaves the world in one batch that only the fold checks

## Status

Accepted. Implemented by `internal/gateway/server.go` (`handleRemoveActor`,
dispatched from `handleCommand`), over `internal/campaign`'s `AppendBatch` and
`internal/engine`'s `Apply`; pinned by `internal/gateway/remove_actor_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: a fact about the world is checked once, by the fold.

## How it works

**The batch is built from the snapshot.** `handleCommand` hands
`handleRemoveActor` the state `authorize` took through `Campaign.State`, after
authorization, which is SPEC-013's. The handler collects the ids of the
tokens in `st.Tokens` whose `ActorID` is the command's `actor_id`, sorts them
with `sort.Strings`, and builds one `TokenRemoved` per id in that order and
then the `ActorRemoved`: the batch SPEC-007 states. An actor with no token
yields the `ActorRemoved` alone.

**The stamping.** Each envelope gets an `EventId` from `newEventID`, the
issuer's `ParticipantId` and `ActorRole`, and one `OccurredAt` shared by the
batch; `store.AppendBatch` refuses an envelope with no `EventId`. A failure to
generate an id answers ok=false and appends nothing.

**One append.** `handleRemoveActor` calls `campaign.AppendBatch` once with the
whole batch. `AppendBatch` takes the campaign's lock, folds every envelope
against a snapshot of the state with `engine.Apply`, and persists none of them
unless every one folds, so the batch is appended whole or not at all, the
atomicity SPEC-007 states. An ok result carries `Sequence`, the first sequence
`AppendBatch` assigned; a refusal answers ok=false with the fold's message as
`Error`.

**The fold refuses in the handler's place.** `handleRemoveActor` checks
nothing about the actor. `engine.Apply`'s `ActorRemoved` arm refuses an actor
the state does not hold (`engine: removed unknown actor "<id>"`) and an actor
with a token still on the board, naming the first such token in id order. The
handler reads the snapshot before `AppendBatch` takes the lock, so a token
placed for the actor in between stands when the `ActorRemoved` folds, and the
whole batch is refused.

**Control ends with the actor.** An actor's controllers are its
`controller_ids`, a field of the `Actor` that the `ActorRemoved` arm deletes
with it, so no event revokes a grant on a removed actor. An actor added later
under the same id is controlled by nobody: `engine.Apply` refuses an
`ActorAdded` that names a controller, whichever command carries it
(SPEC-013).

**What this record does not decide.** The batch's wire shape and its
atomicity are SPEC-007's; who may remove an actor is SPEC-013's (VTT-141);
what a viewer is sent of a removal is SPEC-016's; what the fold refuses in
general, and that an actor's conditions leave with it, is `engine.Apply`'s.

## Consequences

- The DM and the agent, whose seats are unprojected (SPEC-015), receive a
  removal as contiguous `TokenRemoved` events, then the `ActorRemoved`; what
  a player or spectator receives is SPEC-016's.
- Removing an actor that does not exist is refused with the fold's message,
  and nothing is appended.
- A removal refused because a token was placed for the actor meanwhile can be
  sent again; it then reads the new snapshot.
- Control need not be revoked before an actor is removed.
- Whoever changes `handleRemoveActor` adds no check the fold already makes,
  and appends the batch in one call.

## Requirements

VTT-254, VTT-255, VTT-258, VTT-259.
