# SPEC-015: A player's or a spectator's connection streams the one character log it watches

## Status

Accepted, wholly replacing the decision this number held before. Implemented by
`internal/gateway/seat.go` (`watchesALog`, `errWatchEnded`, `seat`, `openSeat`,
`watch`, `resolve`, `recheck`, `rejudge`, `admit`, `drop`, `close`, `perchBox`,
`newPerchBox`, `set`, `take`, `judgedBy`, `canSee`) and
`internal/gateway/viewpoint.go` (`MayWatch`, `ownPartyMember`,
`controlsAPartyMember`, `defaultViewpoint`, `watchable`),
called from `internal/gateway/server.go` (`serve`, `pump`, `pumpState` with
`revoked`, `announce`, `tell`, `moved`, `onWake`, `onPerch` and `onEntry`,
`queueFrame`, `answerCommand`, `handleSetViewpoint`, `handleCommand`,
`handlePromotion`), `presence.go` (`presenceRegistry.rejudge`) and `authz.go`
(`Authorize`), over `internal/campaign`'s `SubscribeCharacter`,
`SubscribeWithNoProgressTimeout`, `WatchAppends` and `Actors` and
`internal/perceive`'s `CanSee`. Pinned by `internal/gateway/watch_test.go`,
`seat_internal_test.go`, `presence_internal_test.go`, `viewpoint_test.go`,
`server_visibility_test.go`, `qa_seat_test.go`, `qa_move_gate_test.go`,
`qa_promotion_rejudge_test.go`, `qa_promotion_opening_test.go`,
`qa_promotion_queued_test.go`, `watchable_test.go`, `qa_watchable_test.go`,
`qa_player_perch_test.go`, `qa_controls_none_test.go` and `authz_test.go`.

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

**Who may watch whom.** A participant controls a party member when an actor
`engine.IsPartyMember` accepts lists them among its controllers
(`controlsAPartyMember`, over the actors `MayWatch` is handed). `MayWatch`
accepts:
- for a spectator, any actor whose kind `engine.IsPartyMember` accepts, or
  the empty id, which is nobody (VTT-181, VTT-345);
- for a player who controls a party member, only a party member whose
  controllers include them, and never the empty id (VTT-450, VTT-451);
- for a player who controls none, what it accepts for a spectator: any party
  member, or nobody (VTT-452, by the owner's ruling B-4 of 2026-10-22, "A
  player whose character the DM has taken watches as a spectator does");
- for any other role, nothing, naming the role.

A named actor that is absent, not a party member, or, for a player who
controls a party member, one they do not control is refused with one string,
which names only the id the asker sent (VTT-347, VTT-182). The empty id from
such a player is refused with a string of its own, which names no actor.
Every refusal wraps `ErrUnauthorized` (SPEC-013). `MayWatch` judges the
actors it is handed: `Campaign.Actors()`, a copy of the live actors, in
`openSeat` and on the pump; the command's `st.Actors` in `Authorize`.

**The default.** `defaultViewpoint` is a player's first party member by id,
among those they control, and the empty id for a player who controls none and
for every other role (VTT-340). So a spectator, or a player who controls no
party member, whose connection names no log it may watch watches nobody until
it perches (VTT-369, VTT-385, VTT-340), and a player watches a party member
whenever they control one, from the next time the pump re-judges the seat.

**Whom a seat is told it may watch.** `watchable` is every party member
`MayWatch` accepts for the participant, sorted by id, each with its name: for
a spectator and for a player who controls none, every party member; for a
player who controls one, those they control; for any other role, none. Its
`controls_no_party_member` is true when the participant controls no party
member, whatever their role (VTT-454). `openSeat` records the set for a log
seat, and `resolve` records it again each time it judges the seat, by the
participant's role then. `tell` queues it as a `WatchableChanged` unless it
equals the last one the pump queued (`proto.Equal`), and never on a world
seat, so a set whose flag alone has changed is sent again (VTT-455). The pump
calls `tell` at the opening, after its credential lookup and before the
opening `ViewpointChanged`, and in `moved`, after every judgment and, on a
switch, after the credential is re-resolved, before the judgment's
`ViewpointChanged`. So a seat is sent the set at its opening (VTT-399) and
again at each judgment that finds it changed (VTT-400, VTT-455), before any
entry of the append that changed it, since `admit` takes a waiting wake
before it sends an entry; appends whose wakes the pump takes as one are
judged once, so a grant and its revocation judged together send no set. A
DM's or an agent's connection is sent none (VTT-401).

**The open.** `openSeat` registers the append watch before it reads the actors,
so an append that lands while the seat opens wakes it. When `log` names an
actor the participant may watch, the seat opens on that log after the
connection's cursor (VTT-339, VTT-349); otherwise on the default from its first
entry, the cursor ignored. A spectator whose `log` names no party member opens
on nobody (VTT-369). For a log seat, the head `serve` queues first is what
`watch` returns: the catch-up head of the log the seat opened on, or 0 for
nobody (VTT-374, VTT-384, SPEC-011). The pump re-resolves the credential and
then queues the set the seat opened with and a `ViewpointChanged` naming what
the seat shows, empty for nobody, both before any entry (VTT-368); a DM's or an
agent's pump sends neither. A log seat's pump then re-judges the seat by the
role that lookup returned (`recheck`), so a seat that role may not watch moves
before any entry, announced by a second `ViewpointChanged` (VTT-340, VTT-387),
and one it may still watch stays (VTT-388).

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
  default otherwise (`resolve`). So:
  - a player whose watched party member is made a non-party actor or
    removed, or is revoked while they still control another, moves to their
    first party member by id, or to nobody when they control none (VTT-340);
  - a player whose watched party member is revoked, and was the last party
    member they controlled, is not moved off it by that revocation when it
    is still a party member after it, by the owner's ruling "Stay on Tobbe"
    of 2026-10-23 (VTT-456);
  - a player who controls none, watching nobody or a party member they do
    not control, who is granted a party member, and does not then control
    the one they watch, moves onto their first by id (VTT-340);
  - a spectator whose shoulder leaves the party moves to nobody (VTT-386).
- A promotion: after `identity.SetRole`, `handlePromotion` calls
  `presenceRegistry.rejudge`, which puts a signal on each of the promoted
  participant's connections (SPEC-011). The pump takes it and runs `onWake`,
  so the seat is re-judged by the new role with nothing appended (VTT-340),
  and a seat the new role may still watch stays where it is (VTT-388).
- A perch: `handleSetViewpoint` calls `perchBox.set` after `authorize`
  (SPEC-013). The pump takes it, and `onPerch` applies nothing when it names
  what the seat shows (VTT-375) or when `MayWatch` refuses it against the
  current actors, and otherwise calls `watch(actorID, 0)`. A refused perch
  leaves the seat where it was (VTT-193).
- An entry's admission, below.

A promotion appends nothing (SPEC-007, VTT-036) and so raises no wake; it
reaches a seat by its signal, by the pump's opening `recheck`, by another
append's wake or by an entry's `rejudge`, whichever comes first. A move that
changes what the seat shows is announced by one `ViewpointChanged`, queued
after the move, after the judgment's `WatchableChanged` when the set changed,
and before any entry of the new log, which is sent from its first entry
(VTT-342, VTT-192); no entry of the previous log follows it
(VTT-367).

**No entry is sent after the right is lost.** An append raises its wake after
its live apply and before any of its entries is delivered (VTT-355, SPEC-021).
`admit` reads a waiting wake before it lets the pump send an entry it has
taken, and when the recheck moves the seat the entry, which belongs to the log
just left, is not sent. So no entry caused by the event that took away the
right to watch a log, or by a later event, is sent on that connection
(VTT-348). The entries still queued on the old subscription are dropped with
it, a batch's earlier entries included. A seat records the role it was last
judged by (`judgedAs`, set by `openSeat`, `watch` and `recheck`), and
`onEntry`, after its last lookup, re-judges a seat whose participant now holds
another (`rejudge`) and does not send the entry when that moves the seat. So
no entry the pump queues after it has read a new role is of a log that role
may not watch (VTT-387). An entry the pump judged before it read the new role,
or a frame already queued for the writer, may still be sent after the
promotion returns.

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
the told viewpoint while it is a party member the participant controls at
the command's snapshot (`ownPartyMember`), and the default at that snapshot
otherwise (VTT-453). So a player whose connection still shows another's
party member when they are granted one of their own is judged by their own
first. `canSee` asks `perceive.CanSee` with
`Eyes{Shoulder: viewpoint}` for one square and records nothing; the player's
move gate in `handleCommand` is its one production caller (`grep -rn
'canSee(' --include='*.go' internal cmd | grep -v _test.go` prints its
definition and that call; SPEC-013, VTT-350), and the square it asks about is
the one the token would move to, the command's `to`. A player who controls no
party member has no eyes, whomever they watch, so every move of a token they
control is refused as unseen; `Authorize` refuses any other first (VTT-370).

**What this record does not decide.** Where a world subscription starts, what
the catch-up head means, the pump's delivery and the one writer are SPEC-011's;
the role table and the command path are SPEC-013's; `ViewpointChanged`'s and
`WatchableChanged`'s shapes and that `set_viewpoint` appends nothing are
SPEC-007's; what a client does at either and what it offers are SPEC-022's;
re-resolution is SPEC-009's; what a log holds and the append watch are
SPEC-021's; what a character perceives is SPEC-016's; what a party member is,
is `engine.IsPartyMember`'s; the presence registry's locks, which a
promotion's signal takes, are SPEC-011's.

## Consequences

A client author, and whoever changes the code, are bound by these:

- A spectator's connection, or that of a player who controls no party
  member, that names no log it may watch is sent no entry until it perches;
  a player's watches a party member whenever they control one, from the
  pump's next re-judging of the seat.
- A switch sends the new log from its first entry; what a client does at a
  `ViewpointChanged` is SPEC-022's.
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
- A player who controls no party member moves nothing (VTT-370). They may
  watch any party member, or nobody, as a spectator may (VTT-452). The event
  that leaves them controlling no party member does not move them off a
  party member they watch that is still one (VTT-456), and a grant after which
  they control a party member, but not the one they watch, moves them to
  their first by id (VTT-340).
- A promotion re-judges the promoted participant's seats with nothing
  appended and moves each one the new role may not watch; a client learns of
  a move by its `ViewpointChanged`.
- A client learns whom it may watch, and whether its participant controls a
  party member, by `WatchableChanged`, never from its log's roster. A set
  that has not changed, in its actors or its flag, is not sent again, a
  switch included.

## Requirements

VTT-176, VTT-181, VTT-182, VTT-188, VTT-189, VTT-192, VTT-193, VTT-306,
VTT-338, VTT-339, VTT-340, VTT-341, VTT-342, VTT-345, VTT-347, VTT-348,
VTT-349, VTT-350, VTT-366, VTT-367, VTT-368, VTT-369, VTT-370, VTT-374,
VTT-375, VTT-377, VTT-385, VTT-386, VTT-387, VTT-388, VTT-399, VTT-400,
VTT-401, VTT-450, VTT-451, VTT-452, VTT-453, VTT-454, VTT-455, VTT-456.
