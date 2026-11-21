# SPEC-015: A player's or a spectator's connection streams the one character log it watches

## Status

Accepted, wholly replacing the decision this number held before. Implemented by
`internal/gateway/seat.go` (`watchesALog`, `errWatchEnded`, `seat`, `openSeat`,
`watch`, `resolve`, `recheck`, `admit`, `drop`, `close`, `perchBox`,
`newPerchBox`, `set`, `take`, `judgedBy`, `canSee`) and
`internal/gateway/viewpoint.go` (`MayWatch`, `ownPartyMember`,
`controlsAPartyMember`, `defaultViewpoint`, `watchable`),
called from `internal/gateway/server.go` (`serve`, `pump`, `pumpState` with
`revoked`, `announce`, `tell`, `moved`, `onWake`, `onPerch` and `onEntry`,
`queueFrame`, `answerCommand`, `handleSetViewpoint`, `handleCommand`) and
`authz.go` (`Authorize`, `roleIn`), over `internal/campaign`'s
`SubscribeCharacter`, `SubscribeWithNoProgressTimeout`, `WatchAppends` and
`Actors` and `internal/perceive`'s `CanSee`. Pinned by
`internal/gateway/watch_test.go`, `seat_internal_test.go`, `viewpoint_test.go`,
`server_visibility_test.go`, `qa_seat_test.go`, `qa_move_gate_test.go`,
`qa_promotion_rejudge_test.go`, `qa_promotion_queued_test.go`,
`watchable_test.go`, `qa_watchable_test.go`, `watchable_abilities_test.go`,
`qa_watchable_abilities_test.go`, `qa_player_perch_test.go`,
`qa_controls_none_test.go`, `role_internal_test.go`, `make_spectator_test.go`,
`qa_role_from_log_test.go` and `authz_test.go`.

## Principles served

Principles 6, 7, 16 and 22 of `docs/blueprint.md`:

- **6.** What a connection is sent is decided on the server, per connection.
- **7.** The pump re-judges the seat, so a change of control changes what a
  connection is sent without a reconnect.
- **16.** A player's or a spectator's connection streams the one character
  log it watches, never the world.
- **22.** A reconnect names the log its cursor was read from, and continues in
  it while the participant may still watch it.

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

**Who may watch whom.** `MayWatch` tells a player from a spectator by the
actors it is handed, through `roleIn` (SPEC-013, VTT-893). No grant leaves a
player holding an actor that is not a party member, save two grants racing
(SPEC-013). `MayWatch` accepts:
- for a spectator, any actor whose kind `engine.IsPartyMember` accepts, or
  the empty id, which is nobody (VTT-181, VTT-345);
- for a player, only a party member whose controllers include them, and
  never the empty id (VTT-450, VTT-451);
- for any other role, nothing, naming the role.

A named actor that is absent, not a party member, or, for a player, one they
do not control is refused with one string, which names only the id the asker
sent (VTT-347, VTT-182). The empty id from a player is refused with a string
of its own, which names no actor.
Every refusal wraps `ErrUnauthorized` (SPEC-013). `MayWatch` judges the
actors it is handed: `Campaign.Actors()`, a copy of the live actors, in
`openSeat` and on the pump; the command's `st.Actors` in `Authorize`.

**The default.** `defaultViewpoint` is a player's first party member by id,
among those they control, or the empty id when they hold none, as two grants
racing can leave them (SPEC-013); and the empty id for every other role, the
role judged by the actors it is handed (`roleIn`, VTT-921). So a spectator whose
connection names no log it may watch watches nobody until it perches
(VTT-369, VTT-385), and a player watches one of their party members from the
next time the pump judges the seat.

**Whom a seat is told it may watch.** `watchable` is every party member
`MayWatch` accepts for the participant, sorted by id, each with its name: for
a spectator, every party member; for a player, those they control; for any
other role, none. Each party member of the set that the participant controls
(`controls`) carries a copy of the ability ids its sheet names, in the
sheet's order, and every other carries none (VTT-731, by the owner's ruling
OD-3 of 2026-11-14): a player is sent the ids of each party member of the
set, and a spectator, who controls none, is sent none. What a window does
with the ids is SPEC-022's: a window that is not a player's shows nothing for
them and reads nothing (VTT-752). Its `controls_no_party_member` is true when
the participant controls no party member (`controlsAPartyMember`, VTT-454),
which holds for every spectator and, save two grants racing (SPEC-013), for
no player. `openSeat` records the set
for a log seat, and `resolve` records it again each time it judges the seat,
by the actors then. `tell` queues it as a `WatchableChanged` unless it
equals the last one the pump queued (`proto.Equal`), and never on a world
seat, so a set whose flag alone has changed is sent again (VTT-455), and so is
one whose only change is a controlled party member's ability ids, as at a give
or a take (VTT-731). The pump calls `tell` at the opening, after its
credential lookup and before the opening `ViewpointChanged`, and in `moved`,
after every judgment and, on a switch, after the credential is re-resolved,
before the judgment's `ViewpointChanged`. So a seat is sent the set at its
opening (VTT-399) and again at each judgment that finds it changed (VTT-400,
VTT-455), before any entry of the append that changed it, since `admit` takes
a waiting wake before it sends an entry; appends whose wakes the pump takes as
one are judged once, so a grant and its revocation judged together send no
set, and neither do a give of an id and then its take. Otherwise a give or a
take to a party member the participant controls sends each of their log seats a
new set, whichever character the seat watches, before any
entry of that append, and one to any other actor sends them none
(VTT-731). A DM's or an agent's connection is sent
none (VTT-401).

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
agent's pump sends neither. A log seat's pump then judges the seat again by
the actors as they stand (`recheck`), so a seat the participant may no longer
watch moves before any entry, announced by a second `ViewpointChanged`
(VTT-921), and one they may still watch stays.

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
  default otherwise (`resolve`), judged by the actors after the append. So:
  - a player whose watched party member is removed, or is revoked, while
    they still control another actor moves to their first party member by
    id (VTT-921);
  - a player whose last actor is revoked is a spectator after it, and is not
    moved off the party member they watch while it is still a party member,
    by the owner's ruling "Stay on Tobbe" of 2026-10-23 (VTT-923);
  - a spectator who is granted an actor is a player after it and, unless
    they watch the one granted, moves onto their first party member by id
    (VTT-893, VTT-921);
  - a spectator whose shoulder leaves the party moves to nobody (VTT-386),
    and so does a player whose watched party member is removed when it was
    the last actor they controlled.
- A perch: `handleSetViewpoint` calls `perchBox.set` after `authorize`
  (SPEC-013). The pump takes it, and `onPerch` applies nothing when it names
  what the seat shows (VTT-375) or when `MayWatch` refuses it against the
  current actors, and otherwise calls `watch(actorID, 0)`. A refused perch
  leaves the seat where it was (VTT-193).
- An entry's admission, below.

A promotion to spectator appends a revocation of each actor its participant
controls, as one batch (SPEC-007, VTT-896), so it reaches a seat by that
append's wake, as every change of control does, and leaves the seat on the
party member it watches (VTT-923). A move that
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
it, a batch's earlier entries included. A participant's role follows the
actors they control (`roleIn`), and the actors change only by an append, so
a change of role is judged at that append's wake, before any entry it
caused, like any other change of control (VTT-348).

**The credential.** The pump re-resolves the participant's credential at the
moments SPEC-009 lists, after `admit` among them since `admit` may have taken
the entry's wake, and closes the connection when identity no longer holds it
(`revoked`, VTT-032). The lookup changes nothing else about the seat:
`MayWatch` and the default tell a player from a spectator by the actors they
are handed (`roleIn`), never by the role identity stores.

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
otherwise (VTT-922). So a participant whose connection still shows another's
party member when they are granted one of their own is judged by their own
first. `canSee` asks `perceive.CanSee` with
`Eyes{Shoulder: viewpoint}` for one square and records nothing; the player's
move gate in `handleCommand` is its one production caller (`grep -rn
'canSee(' --include='*.go' internal cmd | grep -v _test.go` prints its
definition and that call; SPEC-013, VTT-350), and the square it asks about is
the one the token would move to, the command's `to`.

**What this record does not decide.** Where a world subscription starts, what
the catch-up head means, the pump's delivery and the one writer are SPEC-011's;
the role table, how a role follows control (`roleIn`) and the command path are
SPEC-013's; `ViewpointChanged`'s and
`WatchableChanged`'s shapes and that `set_viewpoint` appends nothing are
SPEC-007's; what a client does at either and what it offers are SPEC-022's;
re-resolution is SPEC-009's; what a log holds and the append watch are
SPEC-021's; what a character perceives is SPEC-016's; what a party member is,
is `engine.IsPartyMember`'s.

## Consequences

A client author, and whoever changes the code, are bound by these:

- A spectator's connection that names no log it may watch is sent no entry
  until it perches; a player's watches one of their party members from the
  pump's next judgment of the seat.
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
- A grant makes a spectator a player, and the last revocation makes a
  player a spectator, with no reconnect: the seat is judged again at the
  append's wake. The event that ends a participant's control of their last
  actor does not move them off a party member they watch that is still one
  (VTT-923), and a grant after which they control a party member, but not
  the one they watch, moves them to their first by id (VTT-921).
- A promotion to spectator moves a seat only by the revocations it appends;
  a client learns of a move by its `ViewpointChanged`.
- A client learns whom it may watch, whether its participant controls a
  party member, and which abilities each party member it controls names, by
  `WatchableChanged`, never from its log's roster. A set that has not
  changed, in its actors, their ability ids or its flag, is not sent again,
  a switch included.

## Requirements

VTT-176, VTT-181, VTT-182, VTT-188, VTT-189, VTT-192, VTT-193, VTT-306,
VTT-338, VTT-339, VTT-341, VTT-342, VTT-345, VTT-347, VTT-348, VTT-349,
VTT-350, VTT-366, VTT-367, VTT-368, VTT-369, VTT-374, VTT-375, VTT-377,
VTT-385, VTT-386, VTT-399, VTT-400, VTT-401, VTT-450, VTT-451, VTT-454,
VTT-455, VTT-731, VTT-893, VTT-921, VTT-922, VTT-923.
