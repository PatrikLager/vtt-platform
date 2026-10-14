# A promotion is not announced to a revoked participant

## The problem

`handlePromotion` in `internal/gateway/server.go` ends by calling
`announcePromotion`, which sends a `PresenceChanged` frame naming the promoted
participant through `presenceRegistry.announceIfPresent` in
`internal/gateway/presence.go`. `announceIfPresent` takes every connection in
the registry as its targets. The other two announcements do not:
`announcePresence` and `announceDeparture` resolve `s.revoked()` first and hand
it to `broadcast` and `announceIfAbsent` as a deny set, and both skip a denied
participant's connections. `announcePromotion` resolves nothing, and
`announceIfPresent` has no parameter to take a deny set in.

A participant is revoked by `vtt revoke` (`newRevokeCmd` in
`cmd/vtt/revoke.go`), which calls `Revoke` on the identity store
(`identity.DB`) from another process. The gateway is not told. It finds out
when it next resolves that participant through `Lookup` on the same store: in
the read loop before a command, through `credentialGone` before the backlog
and before an event is delivered, and through `revoked` before the two
announcements above. `handlePromotion` resolves its target too and refuses to
promote a revoked one, which does nothing to that participant's connection.
Until one of the first three happens the revoked participant's connection
stays in the registry, so a promotion of somebody else sends it the frame,
with the promoted participant's id and display name.
Where the promoted participant connected after the revocation,
`announcePresence` withheld that arrival from the revoked participant
(`TestARevokedWatcherIsNotEvenToldWhoElseArrives`), and the promotion's frame
names them all the same. A promotion appends no event
(`TestAPromotionAppendsNoEvent`), so it makes no pump resolve anybody either.

VTT-032 says a revoked participant is refused on their next presence frame,
and this frame is not refused. It is the one such frame. Every statement under
`internal/gateway`, outside its test files, that hands bytes to a connection's
queue (`grep -n 'outCh <-\|\.out <-\|r\.send(' internal/gateway/*.go`) is one
of these: the head frame `serve` queues before anything else; a command's
result, queued in the read loop behind its `identity.Lookup`; the events and
perch frames `enqueueEvents` queues, behind `credentialGone` or behind the
command that asked for them; the snapshot `joinAndSend` sends the joiner
itself; the sends of `broadcast` and `announceIfAbsent`, which take the deny
set; and the send of `announceIfPresent`, which reaches other participants'
connections with neither a resolution nor a deny set. The four registry
senders hand over through `send`, whose own statement the search prints too.

Three records state the gap as it stands. SPEC-009 says the promotion nudge
"sends to every connection without consulting `revoked`". SPEC-011 says
"`announcePromotion` passes no deny set". `docs/verification-debt.md` carries
it under the heading dated 2026-09-24.

Two more sentences say where the frame goes without naming the gap: SPEC-009's
paragraph on promotion, "to every connection through `announcePromotion`,
theirs included", and SPEC-011's on presence, "to every connection including
the participant's own". Two comments do the same: the one above
`announcePromotion`, and the one above `participantIDs`, which names
`announceIfAbsent` and `broadcast` as the callers that take a deny set.

MapTool, which rule 9 of CLAUDE.md asks about first, needs no such filter:
`MapToolServer.bootPlayer` calls `releaseClientConnection`, which closes the
connection and removes it from the router, and `Router.broadcastMessage` walks
the router's live set (read in `~/dev/RPTool/maptool` at `f4b7fef6`). That
answer does not fit here. Its server does the booting itself; this gateway is
never told of a revocation, which another process writes, and SPEC-009 has it
resolve the participant again instead.

## Done looks like

1. A new test in `internal/gateway/server_test.go`,
   `TestAPromotionIsNotAnnouncedToARevokedWatcher`, connects a participant and
   revokes them through `identity.Revoke`, then has a DM promote a third
   participant who is connected. A connected participant who is not revoked,
   and who has already read the promoted participant's arrival, is sent a
   further `PresenceChanged` naming them, and the revoked one is sent no
   `PresenceChanged`. A promotion's frame carries the same three fields with
   the same values as that participant's arrival, so the arrival is read
   first. On today's tree the test fails: the revoked participant is sent the
   frame.
2. `TestAPromotionIsAnnouncedToThePromotedPersonThemselves` (VTT-047) passes
   with no statement of it changed. One comment inside it, "broadcast excludes
   nobody for exactly that reason", is reworded, since the fan-out then
   excludes a revoked participant.
3. `go test -count=1 ./internal/gateway/...` passes with no assertion of a test
   that exists today changed. The three calls of `announceIfPresent` in
   `internal/gateway/presence_internal_test.go` may gain an argument.
4. `announcePromotion` resolves `revoked` before the registry's locks are
   taken. `revoked`'s own comment forbids calling it under them, and SPEC-011
   states that no identity read happens there. Held by Phase 4b's reading.
5. In `docs/requirements.md`, VTT-032's row names the new test among its
   evidence, which `check:requirements-chain` holds. The test cites VTT-032 on
   the line above it; the chain gate does not hold that in a file that cites
   the id above other tests, so `grep -B1 '^func
   TestAPromotionIsNotAnnouncedToARevokedWatcher'
   internal/gateway/server_test.go` shows it.
6. The entry dated 2026-09-24 in `docs/verification-debt.md` carries a
   `Closed by` paragraph naming the new test.
7. `task check` whole is green.

## Rules this puts on the system

This repairs VTT-032, "A revoked participant is refused on their next
command, their next delivered event and their next presence frame, without a
reconnect.", whose third clause the promotion's frame breaks today.

Two rules were added after Phase 4a, whose tests pin them and found no row to
cite. The owner ruled on 2026-10-14 that they earn ids:

- VTT-309: A promotion is announced to every connection whose participant is
  not revoked.
- VTT-310: A promotion of a participant with no connection is announced to
  nobody.

## What it touches

1. `internal/gateway/presence.go`, `internal/gateway/server.go`
2. `internal/gateway/server_test.go`,
   `internal/gateway/presence_internal_test.go`
3. A QA test file under `internal/gateway`, if Phase 4a derives tests
4. `docs/requirements.md`, `docs/verification-debt.md`
5. `docs/specifications/009-identity-and-joining.md`,
   `docs/specifications/011-the-connection.md`
6. The implementation report, under `docs/reports/`

One component, `internal/gateway`.

## Specifications this moves

docs/specifications/009-identity-and-joining.md
docs/specifications/011-the-connection.md

## What could not be established

- Whether a table has met it. It needs a revoked participant still connected,
  with no command sent and no event delivered since the revocation, at the
  moment somebody else is promoted, and no log records a presence frame.
- Whether a revoked connection should be closed by an announcement that skips
  it. `broadcast` and `announceIfAbsent` skip and close nothing, and this
  ticket follows them.
