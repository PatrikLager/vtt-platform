# A promotion re-judges every seat of the participant it promotes

## The problem

`handlePromotion` in `internal/gateway/server.go` changes a participant's role
through `identity.DB.SetRole` and then calls `announcePromotion`, which sends a
`PresenceChanged` frame through `presenceRegistry.announceIfPresent`. It
appends nothing to the campaign, so `Campaign.WatchAppends` raises no wake, and
nothing else reaches the promoted participant's connections. A log seat's pump
(`Server.pump`) re-judges what the seat shows only in `onWake`, in `onPerch`,
and in `admit` when a wake is already waiting; `pumpState.revoked` replaces the
seat's participant on every step, but replacing it re-judges nothing. So the
connection of a spectator who is promoted to player keeps the log it shows
until the next append anywhere in the campaign, and the entries its
subscription already holds are still sent.

`authorizePromotionTarget` allows a promotion to player or to spectator only.
Two cases therefore move a seat, both from spectator to player:

- the spectator watches a party member they do not control. A player may watch
  only a party member they control (`MayWatch`), so the seat must move to the
  player's default (`defaultViewpoint`): their first party member by id, or
  nobody;
- the spectator watches nobody but controls a party member. The grant path in
  `handleCommand` checks the command's form (`validateGrantActorControl`) and
  not the target's role, so a spectator can hold one. VTT-340 says a player
  watches their first party member whenever they watch nothing they may
  watch, so the seat must move onto it.

A promotion to spectator moves no seat. A spectator may watch any party member,
which covers what a player watched, and a spectator watching nobody keeps
watching nobody (VTT-385).

VTT-340 says "whenever", and today the move waits for an append. The gap is
recorded in `docs/verification-debt.md` under Open debt, in the entry
beginning "A promotion re-judges a seat only at the next append's wake". The
owner chose on 2026-10-18 that a promotion re-judges the seat itself.

The same gap exists while a connection opens. `handleWS` reads the participant
through `identity.DB.Verify` before `serve` runs, and `openSeat` judges by
that role. `pump` re-resolves the
participant when it starts and then announces what the seat shows without
re-judging it. A promotion that lands between the read and the pump's start is
seen only at the next wake.

How MapTool solves this, per CLAUDE.md rule 9: it does not. A client's role is
set at the handshake, from the server's connection-successful message
(`ClientHandshake`). `Players.setRole` changes the stored player, and its doc
comment says it "will not change the role of a currently logged in player,
they will have to log out and log back in for it to take effect". MapTool's
view is a client-side filter over a campaign every client holds, which this
platform refuses (rule 9). Nothing is borrowed.

## Done looks like

1. A test in `internal/gateway` perches a spectator on a party member another
   player controls, and promotes the spectator to player with
   `promote_participant` from the DM's connection. Nothing is appended after
   the promotion. The spectator's connection is sent a `ViewpointChanged`
   naming nobody, and no entry of the party member's log after it. It fails
   today: no `ViewpointChanged` arrives.
2. A test in `internal/gateway` grants a spectator, who watches nobody, a party
   member, then promotes them to player with nothing appended after. The
   connection is sent a `ViewpointChanged` naming that party member, then that
   log from its first entry. It fails today.
3. A test in `internal/gateway` holds two connections of one spectator, both
   perched on a party member another player controls, and promotes the
   spectator. Both connections are sent a `ViewpointChanged` naming nobody. It
   fails today.
4. A test in `internal/gateway` promotes a spectator after their connection's
   seat has opened on a party member another player controls and before that
   connection's pump starts. Before a `ViewpointChanged` naming nobody, the
   connection is sent no entry of that party member's log, and after it a
   narration appended to the campaign sends nothing. It fails today: the pump
   announces the party member and sends its log.
5. A test in `internal/gateway` promotes a player who watches their own party
   member to spectator. The connection is sent no `ViewpointChanged` and goes
   on receiving that party member's log. It passes today and must still pass.
6. The entry in `docs/verification-debt.md` beginning "A promotion re-judges a
   seat only at the next append's wake" leaves Open debt and is closed by a
   paragraph naming the tests of items 1 to 4. `grep -n 'A promotion re-judges
   a seat only at the next append' docs/verification-debt.md` prints a line
   that is above the `## Open debt` heading.
7. `docs/specifications/015-the-seat-and-the-perch.md` no longer says a
   promotion wakes no seat. `grep -n 'wakes no seat'
   docs/specifications/015-the-seat-and-the-perch.md` prints nothing.
8. `task check` whole is green.
9. A test in `internal/gateway` promotes a spectator who controls a party
   member and watches nobody, after their connection's seat has opened and
   before its pump starts. The connection is sent a `ViewpointChanged` naming
   that party member, then that log from its first entry. It fails today: the
   connection is sent one `ViewpointChanged` naming nobody, then nothing.
10. A test in `internal/gateway` opens a spectator's seat on a party member
    another player controls, promotes the spectator to player, and lets the
    pump decide that log's next entry with no append wake waiting. The entry
    is not sent, and the seat moves to nobody. It fails today: the entry is
    sent and the seat goes on showing the party member.

## Rules this puts on the system

- A promotion re-judges, with no append, what every connection of the promoted
  participant shows, by the new role. This is VTT-340 holding without an
  append, and it takes no row of its own.
- Every entry a player's or a spectator's connection queues after its pump
  has re-resolved its participant to a new role is of a log that role may
  watch. A connection whose participant is promoted while it opens is one
  instance.
- A change of a participant's role leaves each of their connections watching
  what it watches while the new role may watch it. No row says this today:
  VTT-340 is about a player's connection and VTT-385 about a spectator who
  watches nobody.

## What it touches

1. `internal/gateway/server.go`: `handlePromotion`, `serve`, `pump`,
   `pumpState` with `onEntry`
2. `internal/gateway/seat.go`: `seat`, `watch`, `recheck`
3. `internal/gateway/presence.go`: `presenceConn` and `presenceRegistry`
4. `internal/gateway/watch_test.go`
5. `internal/gateway/seat_internal_test.go`
6. `internal/gateway/presence_internal_test.go`
7. `docs/requirements.md`
8. `docs/verification-debt.md`
9. `docs/specifications/015-the-seat-and-the-perch.md`
10. `docs/specifications/009-identity-and-joining.md`
11. `docs/specifications/011-the-connection.md`

One component, `internal/gateway`, and the records it moves. Eleven files,
five of them records; the split the `ticket` skill asks about was weighed and
not taken, since the code is three files of one package.

## Specifications this moves

docs/specifications/015-the-seat-and-the-perch.md
docs/specifications/009-identity-and-joining.md
docs/specifications/011-the-connection.md

## What could not be established

- Whether an entry already handed on can be withheld. The DM's command
  goroutine and the promoted connection's pump share no order. An entry the
  pump judged before it read the new role, or a frame already queued for the
  connection's writer, may still be sent after `handlePromotion` returns. No
  test can order the two from outside. Item 10 observes an entry judged after
  the new role is read, and VTT-367 already holds that no entry of the log
  left follows a `ViewpointChanged`.
- Whether a revocation needs the same signal. `vtt revoke` (`cmd/vtt/revoke.go`)
  runs in another process, so no in-process signal reaches it. A revoked
  participant's connection is closed at the next append or command (VTT-352).
  This ticket leaves revocation as it is.
