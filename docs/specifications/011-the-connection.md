# SPEC-011: A WebSocket connection to the gateway is one seat with one writer

## Status

Accepted. Implemented by `internal/gateway/server.go` (`handleWS`, `Handler`,
`serve`, `writeUntilFlushed`, `deliver`, `enqueueEvents`, `credentialGone`,
`announcePresence`, `announceDeparture`, `announcePromotion`, `revoked`),
`internal/gateway/keepalive.go`, `internal/gateway/presence.go`,
`internal/gateway/codec.go`, `internal/gateway/seat.go` (`newSeat`,
`subscribeFrom`, `catchUp`) and `cmd/vtt/serve_compose.go` (`composeServer`);
pinned by `internal/gateway/server_test.go`, `server_internal_test.go`,
`presence_internal_test.go`, `keepalive_internal_test.go`,
`keepalive_conn_internal_test.go`, `static_test.go` and
`cmd/vtt/serve_e2e_test.go`.

## Principles served

vtt-platform has no blueprint. The principle this record would serve, that a
connection is a seat and not a person, is missing from the record rather than
absent from the system; the blueprint is owed a ticket of its own.

## How it works

**The handshake refuses before it upgrades.** `handleWS` reads two query
parameters from the plain HTTP request. `after`, parsed by `parseAfter`, is the
sequence the client has already seen; absent it is 0, and one that is not an
integer is answered 400 before the token is looked at. `token` is verified by
`identity.Verify`, and a token it refuses, unknown or revoked, is answered 401
with no upgrade. Only then does `websocket.Accept` run, the connection's read
limit is pinned to `maxWSFrameBytes`, and `serve` takes over with the request's
context. SPEC-009 owns what a credential is; this record owns where it is
checked.

**The routes.** `Handler` registers `/healthz`, which answers 200 and nothing
else, `/ws`, `POST /join`, and the read surface `GET /api/me`,
`GET /api/ruleset`, `GET /api/ruleset/guide`, `GET /api/join-link`,
`GET /api/participants`, `GET /api/adventures`,
`GET /api/adventures/{id}/guide`, `GET /api/maps` and `GET /api/art/{file}`.
The patterns are method-qualified, so a POST to a read route is 405. A
server given a bundle by `WithStatic` also registers `/`, last, and the mux's
most-specific-pattern rule keeps every route above out of the bundle's reach.
What each API route does is not this record's.

**One goroutine writes every frame, from a queue that is never closed.**
`serve` makes the connection's outbound queue, `outCh`, of `gatewayBuffer`
frames, and starts one writer goroutine running `writeUntilFlushed`. Every
frame the connection is sent goes through that queue: the catch-up head from
`serve` itself, results from the read loop, events and perch frames from the
pump through `deliver` and `enqueueEvents`, and presence frames from
`presenceRegistry.send`. The writer takes frames in queue order and writes
each with `conn.Write` under a context bounded by `writeTimeout`; the first
write that fails stops the writer. Nothing closes `outCh`. The writer stops on
`flush`, a separate signal closed once per connection, and before stopping it
writes what is still queued. A frame queued after the writer has stopped stays
in the buffer and is collected with the connection; `enqueueEvents` notices a
stopped writer through `writerDone` and reports `errWriterGone` instead of
parking. `stampedWrite` marks the writer busy for exactly the duration of each
write, which is what keepalive reads.

**The bounds.** `gatewayBuffer` is 256 frames, the size of `outCh` and of the
hand-off channel `campaign.SubscribeWithNoProgressTimeout` gives the pump; it
is slack for a bursty reader, not a limit on how far behind a connection may
fall. `gatewayNoProgress` is 30 seconds, the store's per-subscriber budget: a
connection that leaves a waiting frame unaccepted for that long has its
subscription closed by the store, and the pump then force-closes the socket
with `CloseNow` so the read loop's `conn.Read` fails and the connection
unwinds; the store arms that timer only while a frame waits, so an idle
connection is never timed out this way, which is what the keepalive is for.
`New` sets `writeTimeout` to the same figure, the bound on one `conn.Write`; a
write that takes longer ends the writer and, through the library's failed
connection, the read loop. `maxWSFrameBytes` is 32768 bytes, the read limit on
every inbound frame; a larger frame fails the read and ends the connection. A
frame that reads but does not decode as a `ClientCommand` runs `shutdown` and
closes with `StatusPolicyViolation` and the reason `gateway: malformed frame`.
Each of these ends the one connection it happened on and no other.

**The catch-up head is the seat's, not the log's.** `newSeat` decides what the
connection may receive and where its subscription starts: the DM and the agent
are unprojected and subscribe from `after`; a projected seat subscribes from 0
so the projector can fold the prefix, and frames at or below `after` are
dropped on the way out. `seat.catchUp` returns the store's catch-up head
untouched for an unprojected seat and drains nothing: the last sequence queued
for it, or `after` itself when the log holds nothing newer. For a projected
seat it drains the preloaded backlog, projects every envelope, and answers with
the sequence of the last envelope it sends, 0 when it sends none, which can be
below the log's head; that is the sequence this seat will actually be sent.
`serve` encodes `CatchUpHead` with that number and queues it first. If the head
cannot be encoded the connection is refused rather than served: `serve`
unsubscribes, flushes the writer, and closes with `StatusInternalError` and the
reason `gateway: encode catch-up head failed`. What the head means to a client
is SPEC-007's.

**Delivery re-resolves the credential and sends whole batches.** The pump
goroutine is the only producer of this connection's envelopes. Before the
projected backlog and before every later event it asks `credentialGone`, which
is true only when `identity.Lookup` answers `ErrInvalidToken`; a revoked
participant is closed with `StatusPolicyViolation` and the reason `gateway:
credential no longer valid`, and any other lookup failure delivers anyway. The
rule that a revoked participant is refused on delivery is SPEC-009's (VTT-032,
VTT-033); this is where it runs. Each event goes through `seat.receive`, the
projection, which yields zero, one or several envelopes; a spectator's perch,
taken from `perches` when the pump wakes, is applied and delivered here too, so
the order envelopes are sent in is the order the projector's memory changed in.
`enqueueEvents` encodes each envelope as an event frame and queues it, stopping
at the first failure; an envelope that cannot be encoded ends the connection,
closed with `StatusInternalError` and the reason `gateway: encode failed`, and
no later envelope of that batch is sent, because the order inside a projected
batch is load-bearing for the client's fold. `EncodeFrame` marshals every
frame. The head, the events and the presence frames reach it through
`encodeFrame`, a field of `Server` and never a package variable, so a test that
swaps it cannot reach another server's connections; the read loop marshals a
`CommandResult` with `EncodeFrame` directly. When `events` closes without
`shutdown` having asked for it, the pump closes the socket with `CloseNow`.

**Keepalive.** `New` sets `pingInterval` to `gatewayPingInterval`, 20 seconds,
and `pingTimeout` to `gatewayPingTimeout`, 60 seconds. `serve` starts
`pingUntilStopped` on a goroutine of its own after the arrival announcement,
and stops it through `stopPing` when `serve` returns. `keepAlive` ticks every
interval; a tick while the writer is mid-frame is skipped, because a ping is a
control frame that contends for the library's frame lock and loses that race in
five seconds; otherwise it sends a ping and waits for the pong under a context
bounded by `pingTimeout`. Only that context's deadline is a verdict:
`pingUntilStopped` returns nil for a ping that could not be sent or a
connection already closed, and the loop tries again next tick. A ping that wins
the frame lock and then cannot be written within the library's five seconds has
the connection closed by the library itself, through `setupWriteTimeout`, with
no verdict of ours consulted; the busy skip makes that rarer and cannot prevent
it. A pong that does not arrive in time reaps the peer with `CloseNow`, no
close frame, and the read loop's failed read unwinds the connection through the
ordinary path, departure included. A reap that coincides with a reasoned
`Close` from the pump or the read loop costs the peer its reason: whichever
call takes the library's closing flag first writes its frame, and the other
returns `net.ErrClosed`. `gatewayPingTimeout` is at least three
`gatewayPingInterval`s, so a pong that arrives within three intervals of its
ping never reaps a peer; `TestThePongBudgetStaysAtLeastThreeIntervals` pins the
ratio. The heartbeat is server-side, the WebSocket stack answers a ping below
the application, and no client code takes part. The pinger also returns when
the connection's context ends.

**Presence is a per-participant count, announced after the joiner's own pump
runs.** `presenceRegistry` holds every live connection by pointer and a count
per participant id. `serve` registers the connection with `joinAndSend` after
the head frame and before the pump: under `mu` the registry adds the
connection, increments the count, builds a `PresenceSnapshot` of every present
participant once, the joiner included, and queues it, so no presence delta can
reach the wire ahead of the snapshot that describes the table. `joinAndSend`
reports whether this is the participant's first connection; only then, and only
after the pump goroutine is running, does `serve` call `announcePresence`
synchronously, which broadcasts `CONNECTED` to every other connection. A second
connection of the same participant is not an arrival. `leave` is idempotent,
closes the connection's `done`, decrements the count, and reports the last
connection gone; `announceDeparture` then runs `announceIfAbsent`, which under
`fanOut` and then `mu` checks that no connection of that participant exists,
takes its targets in the same critical section, and otherwise sends nothing; a
participant who reconnected before that check is never announced gone, and one
who reconnects after it is put right by their own `CONNECTED`, whose broadcast
queues behind this fan-out on `fanOut`. `announcePromotion` runs
`announceIfPresent`, one fan-out under `fanOut`, to every connection including
the participant's own. Every fan-out takes `fanOut` first and `mu` only to read
the membership, so announcements reach every connection in one order and a
joiner, who takes `mu` alone, never waits for a fan-out in progress. `send`
hands a frame to a connection and waits at most `presenceSendBudget`, 3
seconds, or until that connection's `done`, then drops the frame for that
connection; presence is repaired by the next snapshot, never by the log.
`broadcast` excludes by connection pointer, so a participant's other devices
receive what one of them caused, and skips the participants in `revoked`, which
`announcePresence` and `announceDeparture` resolve through `identity.Lookup`
before taking `fanOut`, so no identity read happens under the registry's locks;
`announcePromotion` passes no deny set, the debt `docs/verification-debt.md`
records under 2026-09-24. SPEC-007 owns that presence frames are not envelopes
and that `DISCONNECTED` is per participant.

**Teardown runs in dependency order.** `shutdown` leaves presence first, so a
departure is announced before this connection's writer stops and a fan-out
holding this connection abandons it; then sets `closing`, unsubscribes, waits
for the pump to drain, closes `flush`, and waits for the writer. It runs when
the read loop ends, whatever ended it: a failed read, a malformed frame, a
revoked credential, or a result the writer could not take. The pump and the
pinger never call `shutdown`, since it waits on the pump; they close the socket
so the read fails. A deferred `CloseNow` backstops the returns before
`shutdown` exists; the deferred `leavePresence` follows the join with no return
between it and `shutdown`, so it backstops only a panic unwinding through
net/http.

**The static client.** `WithStatic` takes an `fs.FS`, the bundle `cmd/vtt`
embeds, and `Handler` serves it at `/` through `http.FileServerFS`, without
authentication: the program is public, and every route it then calls is
authenticated. A server without a bundle serves the API and answers `/` with
404.

**Process shutdown does not wait for connections.** `composeServer` returns
the server and a `closeFn` that closes the identity handle and then the
campaign. `vtt serve`'s `RunE`, on SIGINT or SIGTERM, calls `srv.Shutdown`
under `serveShutdownTimeout`, then `srv.Close`, then `closeFn`, whether or not
a connection is still unwinding. `http.Server.Shutdown` waits for HTTP
handlers and not for hijacked connections, which every WebSocket connection
is, so a connection mid-teardown can see its handles closed under it. That
gap is recorded in `docs/verification-debt.md`; `cmd/vtt/serve_e2e_test.go`
closes its one connection before `Shutdown`.

## Consequences

- A credential that does not verify is refused with 401 before any frame, so a
  client sees no partial connection.
- A malformed or oversized inbound frame ends the connection that sent it; a
  client that reconnects continues from its `after`.
- A pong not answered within the budget ends the connection with no close
  frame, and a ping is not a clock: nothing may be inferred from a ping that
  does not arrive while frames are flowing.
- A presence frame may be dropped for a connection that does not drain within
  the budget; the next snapshot repairs it, so a client applies snapshots as
  replacements and deltas in the order received.
- The head a connection opens with is always reachable by that seat, and a
  client waiting for it waits for its own seat's last sequence, not the log's.
- Every frame a connection receives was queued by one goroutine per source and
  written by one writer, so a client may rely on queue order and never on
  arrival timing across kinds (SPEC-007).
- A process shutdown may close the store under a connection still unwinding;
  until the debt above is closed, a caller of `closeFn` closes every
  connection first or accepts the race.

## Requirements

VTT-080, VTT-081, VTT-082, VTT-083, VTT-084, VTT-085, VTT-086, VTT-087,
VTT-088, VTT-089, VTT-090, VTT-091, VTT-092, VTT-093, VTT-094, VTT-095,
VTT-096, VTT-097, VTT-098, VTT-099, VTT-100, VTT-101, VTT-102, VTT-103,
VTT-104, VTT-105, VTT-106, VTT-107, VTT-108, VTT-109, VTT-110.
