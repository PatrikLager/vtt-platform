# SPEC-015: What a connection is sent is decided by its seat, whose viewpoint only a spectator may set

## Status

Accepted. Implemented by `internal/gateway/seat.go` (`viewerFor`, `projected`,
`seat`, `perchBox`, `newPerchBox`, `set`, `take`, `newSeat`, `subscribeFrom`,
`receive`, `pastResume`, `perch`, `canSee`, `catchUp`) and
`internal/gateway/viewpoint.go` (`MayPerch`), over `internal/campaign`'s
`FoldPrefix` and the `Projector` in `internal/gateway/project.go`; called from
`internal/gateway/server.go` (`serve`, `handleSetViewpoint`, `handleCommand`)
and `authz.go` (`Authorize`); pinned by `internal/gateway/viewpoint_test.go`,
`viewpoint_internal_test.go`, `server_visibility_test.go` and
`authz_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite is absent from the record rather than from the
system: what a person is sent is decided on the server, per connection, from
the log that connection has been fed, and a watcher sees through one party
member's eyes at a time and never through the DM's.

## How it works

**Which seats are projected.** `projected` answers false for `identity.RoleDM`
and `identity.RoleAgent` and true for every other role; a stored role that
does not parse never reaches it, since `identity.Verify` refuses it
(SPEC-009). `newSeat` gives an unprojected seat no `Projector` and a projected
one `NewProjector(viewerFor(p))`, with `resume` set to the cursor the
connection asked for. An unprojected seat's `receive` returns the event it was
handed, unchanged, and folds nothing. `Project` itself answers the DM and the
agent with the event unchanged (SPEC-016), so what `projected` decides for
them is where their subscription starts and the head they are told (SPEC-011),
whether each event costs a fold, and so whether a failed fold can withhold an
event from them; a player or a spectator answered false would be sent the
whole log.

**How a projected seat is fed, and against which state.** `subscribeFrom`
answers 0 for a projected seat (SPEC-011), so it is fed the log from its start
whatever cursor it asked for. `receive` appends each envelope to `received`,
folds the whole slice with `campaign.FoldPrefix`, hands `Project` the event and
that state, and keeps the state as `world`: each event is judged against the
state after it, from what this seat has received, and never against
`campaign.State()`, which during catch-up is ahead of the event being judged.
Every event folds the whole prefix received so far, with campaign's fold; the
gateway keeps none of its own. That a projector must be fed from the start of
the log is the `Projector`'s (SPEC-016).

**What a projected seat drops.** `pastResume` keeps the frames whose sequence
is strictly greater than `resume`, as `store.Subscribe` keeps `seq >
afterSeq`; input at or below `resume` is still folded. A seat resumed at a
cursor is therefore sent what its projection produces after the cursor,
departures from its view during its absence included, and no frame of an event
at or below it.

**A fold that fails withholds the event.** When `FoldPrefix` fails, `receive`
logs the event's sequence and the error with `slog.Error`, never the event id,
returns no frame for the event, and leaves `world` at the last state that
folded. `received` keeps the envelope, so every later event's fold fails too
and the seat is sent no further event frame on that connection. While one
`Campaign` is the only writer of a log, every prefix a seat receives folds:
`internal/campaign` is the only non-test importer of `internal/store` (`grep
-rln 'vtt-platform/internal/store"'` over the non-test Go files),
`campaign.Append` and `campaign.AppendBatch` are its only callers of the
store's appends (`grep -rn 'c.log.Append'` over `internal/campaign`), and each
validates its envelopes against a snapshot of the live state under that
`Campaign`'s mutex before it persists; `campaign.Open` refuses a log that does
not fold. `campaign.Open` takes the directory's writer hold before it opens
the log (SPEC-019), so a second `Campaign` on one directory, in this process
or another, is refused before it can append; a seat reaches this arm only on a log written
without the hold, and the next `Open` refuses that log too.

**The viewpoint a connection opens with.** `viewerFor` gives the
participant's id and role and an empty `Viewpoint`. `eyes` reads an empty
viewpoint as no eyes, and ignores `Viewpoint` for a player (SPEC-016), so a
spectator's connection is shown no board until a perch names a shoulder, and no
shoulder is chosen for them. The viewpoint lives in the seat, which `serve`
makes per connection.

**Who may perch, on whom, and the one refusal.** `MayPerch` refuses every role
but spectator, naming the role; accepts an empty actor id; and otherwise
accepts an actor `st.Actors` holds whose kind `engine.IsPartyMember` accepts,
reading no controller, so controlling an actor does not make it a shoulder. It
answers an absent actor and a non-party one with one string, which names only
the id the asker sent. Every refusal wraps `ErrUnauthorized` (SPEC-013).
`Authorize` runs `MayPerch` for every `set_viewpoint` after the role table,
which holds a cell for the spectator alone (SPEC-013), so through `Authorize`
the role arm never refuses, and `Authorize` is its one production caller
(`grep -rn 'MayPerch('` over the non-test files); `eyes` refuses a non-party
viewpoint a second time (SPEC-016).

**How a perch travels to the pump.** `handleSetViewpoint` calls
`perchBox.set` after `authorize` (SPEC-013). `set` stores the shoulder and
marks the slot full under `mu`, then signals `wake`, whose capacity is one,
dropping the signal when one is already waiting, and never blocks. `take`
returns the slot's shoulder and whether it was full, and empties it. So the
pump is handed the last shoulder set before its `take`, the empty id included;
a shoulder replaced before then is never applied; and a second wake-up with
nothing new applies nothing. `mu` guards the slot and orders nothing on the
wire.

**How a perch is applied.** `serve`'s pump takes the shoulder when `wake`
fires and calls `perch` at once, so the new view is sent without waiting for
an event, and delivers its frames as it delivers an event's (SPEC-011),
without re-checking the credential (SPEC-009). `perch` returns nothing for an
unprojected seat. Otherwise it returns `Projector.reperch(actorID, world)`:
the new eyes judged against `world`, the state this seat last folded, never
the campaign's head, and nothing when the seat has folded nothing yet. Every
frame of a perch carries `perchSequence`, which is 0, and a perch's output
never passes `pastResume`, so no resume cursor filters it. Neither
`handleSetViewpoint` nor `perch` appends to the log (SPEC-007 names
`set_viewpoint` among the commands that append nothing). What `reperch` sends,
and that a shoulder named again is served in full, since the projector's
memory never held it, are SPEC-016's.

**`canSee`.** `canSee` builds a fresh `Projector` for the viewer, asks `look`
once and answers `canSeeSquare` for one square, so no seat's projector is
touched. The move gate in `handleCommand` is its one production caller
(`grep -rn 'canSee('` over the non-test files; SPEC-013).

**A projected seat's catch-up.** For a projected seat `catchUp` runs each
backlog envelope through `receive`, stops at the first envelope at or past the
log's head, and answers what it has when `events` closes or the context ends;
the head it answers is SPEC-011's (VTT-086). `serve` makes `perches` after
`catchUp` returns, so no perch frame is part of a catch-up.

**What this record does not decide.** Where a subscription starts, the
catch-up head, the pump and the one writer goroutine are SPEC-011's; the
command path to `MayPerch` and `perchBox.set` and the role table are
SPEC-013's; that `set_viewpoint` appends nothing and what `CatchUpHead` means
to a client are SPEC-007's; re-resolution, and that a perch's frames are not
re-checked, are SPEC-009's; what a projection computes (`Project`, `reperch`,
`eyes`, `look`, `canSeeSquare`, `perchSequence`, the `Viewer` type) is
SPEC-016's; what a party member is, is
`engine.IsPartyMember`'s.

## Consequences

A client author, and whoever changes the code, are bound by these:

- A spectator's connection shows no board until it perches.
- A perch's frames carry sequence 0 and move no resume cursor.
- Of a burst of perches, only the last one set before the pump takes one is
  applied; naming a shoulder again restores it.
- A perch refusal says nothing about whether the actor exists.
- The DM's and the agent's streams are the log.
- Every connection of a player or a spectator replays the log from its start
  on the server, one fold of the prefix per event.
- A seat is touched from the pump alone once its catch-up is drained, and
  never from a second goroutine through a lock.
- Append to `log.db` only through a `Campaign`: `campaign.Open` alone takes
  the writer hold (SPEC-019), so a process that appends without one is not
  stopped, and the log it writes can stop folding.

## Requirements

VTT-176, VTT-177, VTT-178, VTT-179, VTT-180, VTT-181, VTT-182, VTT-183,
VTT-184, VTT-185, VTT-186, VTT-187, VTT-188, VTT-189, VTT-190, VTT-191,
VTT-192, VTT-193.
