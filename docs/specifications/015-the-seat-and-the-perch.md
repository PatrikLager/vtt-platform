# SPEC-015: A player's or a spectator's connection streams the one character log it watches

## Status

Accepted, wholly replacing the decision this number held before. Implemented
by `internal/gateway/seat.go` (`watchesALog`, `errWatchEnded`, `seat`,
`openSeat`, `watch`, `resolve`, `recheck`, `admit`, `drop`, `close`,
`perchBox`, `newPerchBox`, `set`, `take`, `judgedBy`, `canSee`) and
`internal/gateway/viewpoint.go` (`MayWatch`, `defaultViewpoint`), called from
`internal/gateway/server.go` (`serve`, `pump`, `pumpState` with `revoked`,
`announce`, `moved`, `onWake`, `onPerch` and `onEntry`, `sendViewpoint`,
`answerCommand`, `handleSetViewpoint`, `handleCommand`) and `authz.go`
(`Authorize`), over `internal/campaign`'s `SubscribeCharacter`,
`SubscribeWithNoProgressTimeout`, `WatchAppends` and `Actors` and
`internal/perceive`'s `CanSee`. Pinned by `internal/gateway/watch_test.go`,
`seat_internal_test.go`, `viewpoint_test.go`, `server_visibility_test.go`,
`qa_seat_test.go`, `qa_move_gate_test.go` and `authz_test.go`.

## Principles served

This project has no blueprint, so no principle can be named as served. The
one this record would cite: what a person is sent is decided on the server,
per connection, and a player or a spectator is sent what one party member
experienced, never the world.

## How it works

**Who is sent what.** `watchesALog` answers false for `identity.RoleDM` and
`identity.RoleAgent` and true for every other role. `openSeat` subscribes a
DM's or an agent's connection to the world after its cursor (SPEC-011) and
ignores `log` (VTT-176, VTT-377). Every other connection is a log seat: it
holds an append watch (`Campaign.WatchAppends`, SPEC-021) and at most one
character subscription (`Campaign.SubscribeCharacter`), and no world
subscription, since the package's one `SubscribeWithNoProgressTimeout` call
sits behind `!watchesALog` (`grep -rn 'Subscribe[A-Za-z]*(' --include='*.go'
internal/gateway | grep -v _test.go` prints `seat.go`'s two calls; VTT-366).
A log seat is sent the entries of the log it watches, each as that log holds
it and numbered by that log's own sequence, and no world event (VTT-338).

**Who may watch whom.** `MayWatch` accepts, for a spectator, any actor whose
kind `engine.IsPartyMember` accepts, or the empty id, which is nobody
(VTT-181, VTT-345); for a player, a party member whose controllers include
them, and never the empty id (VTT-343, VTT-346); for any other role,
nothing, naming the role. A named actor that is absent, not a party member,
or, for a player, one they do not control is refused with one string, which
names only the id the asker sent (VTT-347, VTT-182). Every refusal wraps
`ErrUnauthorized` (SPEC-013). `MayWatch` judges the actors it is handed:
`Campaign.Actors()`, a copy of the live actors, in `openSeat` and on the
pump; the command's `st.Actors` in `Authorize` and the move gate.

**The default.** `defaultViewpoint` is a player's first party member by id,
over the ids `MayWatch` would accept for them, and the empty id for a player
who controls none and for every other role (VTT-340). So a spectator whose
connection names no log it may watch watches nobody until it perches
(VTT-369, VTT-385), and a player watches a party member whenever they
control one, from the next time the pump re-judges the seat.

**The open.** `openSeat` registers the append watch before it reads the
actors, so an append that lands while the seat opens wakes it. When `log`
names an actor the participant may watch, the seat opens on that log after
the connection's cursor (VTT-339, VTT-349); otherwise on the default from its
first entry, the cursor ignored. A spectator whose `log` names no party
member opens on nobody (VTT-369). For a log seat, the head `serve` queues
first is what `watch` returns: the catch-up head of the log the seat opened
on, or 0 for nobody (VTT-374, VTT-384, SPEC-011). The pump re-resolves the
credential and then queues a `ViewpointChanged` naming what the seat shows,
empty for nobody, before any entry (VTT-368); a DM's or an agent's pump sends
none.

**`watch`'s settle loop.** `watch(actorID, cursor)` drops the seat's
subscription, records `actorID` as shown and stores it in `told`, and
subscribes to that log after `cursor`, or to nothing for the empty id, so a
seat that watches nobody is sent no event (VTT-341). If a wake is then
waiting it reads it and resolves the actors again: the same actor ends the
loop, and a different one starts it again from 0. So the head `watch`
returns is that of the log the seat shows, and a wake that arrived while it
subscribed is not lost. A wake channel closed by the campaign's `Close` ends
the watch with `errWatchEnded`.

**What moves a seat, all on the pump.**
- An append's wake: `onWake` re-resolves the credential, and `recheck`
  keeps what is shown while the participant may watch it and moves to the
  default otherwise (`resolve`). A player whose watched party member is
  revoked, made a non-party actor or removed moves to their first party
  member by id, or to nobody, and a player watching nobody who is granted a
  party member moves onto it (VTT-340); a spectator whose shoulder leaves
  the party moves to nobody (VTT-386).
- A perch: `handleSetViewpoint` calls `perchBox.set` after `authorize`
  (SPEC-013). The pump takes it, and `onPerch` applies nothing when it names
  what the seat shows (VTT-375) or when `MayWatch` refuses it against the
  current actors, and otherwise calls `watch(actorID, 0)`. A refused perch
  leaves the seat where it was (VTT-193).
- An entry's admission, below.

A promotion appends nothing (SPEC-007) and wakes no seat: `handlePromotion`
calls `identity.SetRole` and `announcePromotion` alone. So a promoted
participant's seat keeps what it shows until the next append's wake
re-judges it, and the entries its subscription already holds are still
sent; no entry written after the promotion reaches it first, since an append
raises its wake before any of its entries is delivered (VTT-355). A move that
changes what the seat shows is announced by one `ViewpointChanged`, queued
after the move and before any entry of the new log, which is sent from its
first entry (VTT-342, VTT-192); no entry of the previous log follows it
(VTT-367).

**No entry is sent after the right is lost.** An append raises its wake
after its live apply and before any of its entries is delivered (VTT-355,
SPEC-021). `admit` reads a waiting wake before it lets the pump send an
entry it has taken, and when the recheck moves the seat the entry, which
belongs to the log just left, is not sent. So no entry caused by the event
that took away the right to watch a log, or by a later event, is sent on
that connection (VTT-348). The entries still queued on the old subscription
are dropped with it, a batch's earlier entries included.

**The credential.** The pump re-resolves the participant at the moments
SPEC-009 lists, after `admit` among them since `admit` may have taken the
entry's wake; a successful lookup replaces the seat's participant, so
`MayWatch` and the default judge by the current role.

**How a perch travels to the pump.** `set` stores the shoulder and marks the
slot full under `mu`, then signals `wake`, whose capacity is one, dropping
the signal when one is already waiting, and never blocks, so a perch stalls
no other participant's command (VTT-189). `take` returns the slot's shoulder
and whether it was full, and empties it. So the pump is handed the last
shoulder set before its `take`, the empty id included (VTT-188), a perch set
while the pump is busy is still handed to it (VTT-306), and a second wake-up
with nothing new applies nothing. `mu` guards the slot and orders nothing on
the wire.

**The move gate's viewpoint.** `told` is an `atomic.Pointer[string]`, the
one field of a seat read from another goroutine. `watch` stores it before
any frame naming the actor is queued, so a command sent after a
`ViewpointChanged` is never judged by an older viewpoint. `judgedBy` answers
the told viewpoint while `MayWatch` accepts it at the command's snapshot,
and the default at that snapshot otherwise (VTT-351). `canSee` asks
`perceive.CanSee` with `Eyes{Shoulder: viewpoint}` for one square and
records nothing; the player's move gate in `handleCommand` is its one
production caller (`grep -rn 'canSee(' --include='*.go' internal cmd | grep
-v _test.go` prints its definition and that call; SPEC-013, VTT-350). A
player who controls no party member has no eyes, so every move of a token
they control is refused as unseen; `Authorize` refuses any other first
(VTT-370).

**What this record does not decide.** Where a world subscription starts,
what the catch-up head means, the pump's delivery and the one writer are
SPEC-011's; the role table and the command path are SPEC-013's;
`ViewpointChanged`'s shape, what a client does at one, and that
`set_viewpoint` appends nothing are SPEC-007's; re-resolution is SPEC-009's;
what a log holds and the append watch are SPEC-021's; what a character
perceives is SPEC-016's; what a party member is, is `engine.IsPartyMember`'s.

## Consequences

A client author, and whoever changes the code, are bound by these:

- A spectator's connection that names no log it may watch is sent no entry
  until it perches; a player's watches a party member whenever they control
  one, from the pump's next re-judging of the seat.
- A switch sends the new log from its first entry; what a client does at a
  `ViewpointChanged` is SPEC-007's.
- A player's or a spectator's reconnect names the log its cursor was read
  from (`log`); a cursor with no `log`, or with one the participant may no
  longer watch, is ignored.
- Of a burst of perches, only the last one set before the pump takes one is
  applied; naming what the seat shows sends nothing.
- A refusal says nothing about whether the actor exists, is a party member,
  or whose it is.
- The DM's and the agent's streams are the world.
- A seat is touched from the pump alone once `serve` has queued its head,
  `told` aside.
- A switch reads its history under the campaign's lock, so an append waits
  for it (SPEC-021).
- A player who controls no party member moves nothing, and watches nobody
  from the pump's next re-judging of the seat.

## Requirements

VTT-176, VTT-181, VTT-182, VTT-188, VTT-189, VTT-192, VTT-193, VTT-306,
VTT-338, VTT-339, VTT-340, VTT-341, VTT-342, VTT-343, VTT-345, VTT-346,
VTT-347, VTT-348, VTT-349, VTT-350, VTT-351, VTT-366, VTT-367, VTT-368,
VTT-369, VTT-370, VTT-374, VTT-375, VTT-377, VTT-385, VTT-386.
