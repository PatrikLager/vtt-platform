package gateway

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// writeActivity is the seam keepAlive's busy predicate reads: whether this
// connection's writer is mid-frame right now.
//
// A NAMED TYPE RATHER THAN A CLOSURE, for the reason writeUntilFlushed is a
// free function — a closure buried in serve() is a seam nothing can reach, and
// this repo has already shipped one seam whose justification was written down
// and never honoured. Three methods, all trivial, all directly tested.
//
// IN-FLIGHT ONLY, deliberately. An earlier design also skipped the tick when a
// write had merely COMPLETED within the last interval, on the reasoning that a
// socket carrying data recently does not need a keepalive. True, but not
// needed: the only thing that must not happen is a ping contending with a write
// that HOLDS writeFrameMu, and that is exactly what inFlight reports. The extra
// clause bought fewer pings at the price of a second piece of state and a
// clock comparison, and correctness never rested on it.
//
// The race it does NOT close is real and is covered elsewhere: busy() can
// report false a moment before the writer takes the lock, so a ping can still
// lose the five-second race. That is precisely why pingUntilStopped refuses to
// read a send failure as peer death — the skip reduces how often it happens,
// the verdict makes it harmless when it does.
type writeActivity struct{ inFlight atomic.Bool }

func (w *writeActivity) begin()     { w.inFlight.Store(true) }
func (w *writeActivity) end()       { w.inFlight.Store(false) }
func (w *writeActivity) busy() bool { return w.inFlight.Load() }

// gatewayPingInterval is how often serve pings an otherwise-silent connection.
//
// FOUND AT A TABLE, not by reasoning: in session zero a browser left idle
// through a cloudflared tunnel came back reading `closed`, and the requirement
// it produced was Patrik's — "you have to be able to be 'inactive' without
// being kicked out."
//
// Neither of our own deadlines had done it. store.SubscriberNoProgressTimeout
// is armed INSIDE the select that hands an envelope over
// (internal/store/subscribe.go:152-162; the select just above it, 145-150, is
// the wake/done one and arms no timer), so it only runs while an event is
// WAITING; on a quiet table nothing is pending and the timer never starts.
// Server.writeTimeout bounds a write, and an idle connection performs none.
// Both judge a client on whether it is READING, which is the right question
// and answers nothing about a socket where neither side has anything to say.
//
// So an idle connection carried zero bytes in either direction, indefinitely,
// and every hop between a player and the server is entitled to reap a silent
// socket: Cloudflare, carrier NAT, a home router's connection table, a
// corporate proxy. NOT a tunnel problem — removing the tunnel moves the
// number, not the failure.
//
// 20s, and the figure is not arbitrary. gatewayNoProgress's doc comment
// already cited the prior art we then failed to copy: MapTool
// (net.rptools.clientserver) runs a 20s heartbeat against a one-minute socket
// timeout. Patrik recalled the same interval from the same source
// independently.
//
// COST, and the comparison is measured because the first draft's was not. A
// ping and its pong run about a dozen bytes — coder/websocket's ping payload
// is a decimal counter and the pong echoes it back masked — so a six-player
// ninety-minute session spends roughly 18 KB of WebSocket framing on
// keepalive. That much was right. What was wrong, by a factor of twenty, was
// calling it "less than one PresenceSnapshot per hour": a six-participant
// snapshot runs around 700 bytes of protojson — measured 2026-08-26 by
// marshalling a six-entry PresenceSnapshot with 25-character participant ids
// and CONNECTED states, which came to 694 — so 18 KB is roughly twenty-six
// snapshots across the session, about seventeen an hour. Still negligible,
// which is exactly why the wrong figure survived being written. Rounded on
// purpose: the conclusion needs an order of magnitude, and protojson output in
// this repo is build-randomised, so a byte-exact figure invites a reader to
// trust a number that can shift under them.
// Counting WebSocket framing only, too: TCP, TLS and any tunnel add per-packet
// overhead that dominates at frames this small.
const gatewayPingInterval = 20 * time.Second

// gatewayPingTimeout is how long the pong may take before the peer is judged
// gone — MapTool's one-minute socket timeout against the 20s heartbeat above,
// kept at that same 3x ratio deliberately. Patrik's call, 2026-08-26, against
// the tighter alternatives.
//
// THE RATIO IS THE POINT, and it is a floor rather than a nicety. Detection is
// half of what the heartbeat is for (Patrik, on MapTool: "to check if any
// connections where down on the player side"), and detection has a direction
// that is far more dangerous than the disconnect it fixes: a deadline tight
// enough to lose a race with a phone on a slow cell reaps players who are
// perfectly fine. Three intervals means two pongs can be lost or arrive late
// before anyone is declared dead, which is what makes this safe to apply to a
// real table rather than a loopback.
//
// The cost of being generous is bounded and small: a genuinely dead peer is
// noticed within about 80s instead of instantly, and until then it holds one
// idle socket and one presence entry.
const gatewayPingTimeout = 60 * time.Second

// keepAlive pings until ctx is cancelled, stop closes, or a ping fails —
// calling fail exactly once, and only on that last one. It SKIPS a tick
// entirely while busy reports that the connection's writer is working.
//
// A free function with injected busy/ping/fail, matching writeUntilFlushed in
// server.go and for the same reason: the alternative is a closure nothing can
// reach. That justification was made once before and not honoured — nothing
// called this directly, and a reviewer showed the seam was inert by mutating
// the stop arm into a failure and watching every connection-level test still
// pass. keepalive_internal_test.go now drives it directly, which is what makes
// the sentence above true rather than merely intended.
//
// THE SKIP IS NOT AN OPTIMISATION. A socket with data flowing on it does not
// need a keepalive — this exists to stop an IDLE connection looking dead to
// intermediaries, and a connection mid-write is not idle. Pinging anyway means
// contending for coder/websocket's writeFrameMu against a data write that may
// hold it for up to Server.writeTimeout, and a control frame is given only
// five seconds to win that race (write.go:277, pinned v1.8.15). Since the ping
// interval is SHORTER than the write timeout, any write using its full budget
// straddles a tick by construction, so this was not a corner case: it reaped
// healthy clients at five seconds.
//
// Skipping also NARROWS — it does not close — a window the verdict below
// cannot reach. If a ping wins the lock and then blocks past those five
// seconds, coder/websocket's setupWriteTimeout calls c.close() directly
// (conn.go:171-182), so the connection dies without fail() ever being
// consulted. Note what that case actually is: busy() reports that the WRITER
// holds the lock, and here the PING holds it while the socket's send buffer
// refuses to drain. busy() is false in exactly that moment, so skipping cannot
// prevent it — fewer pings simply mean fewer chances to land in it. Claiming
// more than that would be claiming a defence we do not have.
//
// Detection is not weakened. While the writer works, writeTimeout already asks
// whether the peer is reading; when the writer goes quiet, the next tick pings.
// The two cover disjoint halves of one question and neither leaves a gap.
//
// ITS OWN GOROUTINE, not the writer's, and that is forced rather than chosen.
// coder/websocket's Conn.Ping blocks until the pong comes back, and the pong
// is only read by a concurrent Reader call ("Ping must be called concurrently
// with Reader as it does not read from the connection"). Run from the writer,
// a ping would stall every queued event for a whole round trip, and a dead
// peer would stall them for gatewayPingTimeout.
//
// That does NOT break serve's single-writer discipline, which cost three
// attempts to get right (656079f). That discipline orders DATA frames through
// outCh; a ping is a control frame that never enters it, and coder/websocket
// serialises every frame write behind writeFrameMu, so the two cannot
// interleave on the wire.
//
// TICKER, not a sleep loop: a ping that takes a while to fail must not push
// the next one out by however long it took. Missed ticks coalesce, so at most
// one ping is ever in flight — which is what keeps a failing peer from
// accumulating a queue of pending pings while we decide it is gone.
//
// THREE EXITS, ONE OF WHICH IS NOT THE CALLER'S TO FORGET. The loop returns on
// ctx, on stop, and on a ping failure. Only the middle one is the caller's
// responsibility, and that used to be the whole list: once a send failure
// stopped being a verdict, the only SELF-terminating path disappeared with it.
// Before, ANY error ended this loop, including the net.ErrClosed a closed
// connection returns; after, a closed-but-not-yet-stopped connection would be
// pinged once per interval forever. So ctx is honoured here directly rather
// than resting on the caller closing stop on every path, panics included. That
// is precisely the honour-system arrangement this repo keeps finding broken,
// and it costs one select arm to not have.
func keepAlive(
	ctx context.Context,
	interval time.Duration,
	stop <-chan struct{},
	busy func() bool,
	ping func() error,
	fail func(),
) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-t.C:
			if busy() {
				continue
			}
			if ping() != nil {
				fail()
				return
			}
		}
	}
}

// pingUntilStopped runs keepAlive for one connection: it bounds each ping with
// pingTimeout, derived from the connection's own ctx so a cancelled request
// unwinds a ping that is still waiting for a pong.
//
// A PING IS ONLY EVER A VERDICT ABOUT THE PONG. conn.Ping reports failures of
// three different kinds, and the first draft of this treated them alike:
//
//   - the pong did not come back inside OUR budget — pctx hit its deadline.
//     The peer is unreachable and this is the one case that is both evidence
//     about the peer and ours to act on. Reap it;
//   - the ping frame could not be SENT, or the connection is already closed:
//     writeControl's own hard five-second context expired while ours had
//     barely started, or Ping returned net.ErrClosed off c.closed. Neither
//     says anything about the peer, and the writer — or the teardown already
//     under way — owns that case;
//   - the raw socket write failed, ECONNRESET or EPIPE, and writeFrame's
//     deferred wrapper passes it through — wrapped in its own message twice,
//     but with the underlying error's identity intact — because neither the
//     context nor c.isClosed() explains it. THIS ONE IS GENUINE EVIDENCE OF
//     PEER DEATH AND WE DISCARD IT ANYWAY. Not because it is harmless: a
//     socket that resets on write also fails on read, so serve's blocked
//     conn.Read returns, shutdown() runs, and leavePresence tells the table —
//     the departure a player actually notices — within moments. We trade a
//     marginally faster reap for never guessing.
//
//     THAT READ PATH IS THE WHOLE SAFETY ARGUMENT, so do not weaken it without
//     replacing this. An earlier draft of this paragraph also credited
//     coder/websocket's read.go deferred c.close(); that defer lives inside
//     CloseRead, which this repo never calls, so it named unreachable code.
//     Anyone tempted to reap on this error should first confirm serve still
//     does its half.
//
// So the test is the state of pctx, not the presence of an error. A send
// failure returns nil and the loop simply tries again next interval, by which
// time the writer will usually have finished. context.Canceled is excluded
// along with the rest: a cancelled connection ctx means we are tearing down
// anyway, and calling that peer death would be a guess dressed as a finding.
//
// The failure path FORCE-CLOSES rather than closing with a reason, unlike the
// revocation path in serve's pump. There is no one left to tell: a peer that
// did not answer a ping will not read a close frame either, and Close would
// sit through its own handshake timeout first. CloseNow makes the command
// loop's conn.Read error out, and the connection then unwinds through the
// ordinary shutdown path — including leavePresence, which is the half a table
// actually notices. Before this, a peer that had silently gone stayed
// CONNECTED forever, because departure hangs off serve() returning and serve()
// was parked in conn.Read on a socket nobody had told it was gone.
//
// IT CAN ALSO PRE-EMPT SOMEBODY ELSE'S CLOSE REASON, which the paragraph above
// does not cover because it is about a different thing. Close and CloseNow both
// open by swapping coder/websocket's `closing` flag; whoever wins, the loser
// returns net.ErrClosed without writing its close frame. So a pinger that reaps
// at the same moment the pump is closing with "gateway: credential no longer
// valid" costs that peer its reason — the told-why-SOMETIMES failure recorded
// in server.go. The window is narrow and the case is benign: reaching it takes
// a peer that has already missed a 60s pong, and such a peer will not read a
// close frame either. Recorded rather than guarded, because the guard the other
// two sites use keys on `closing`, which is only set inside shutdown().
func pingUntilStopped(
	ctx context.Context,
	interval, timeout time.Duration,
	stop <-chan struct{},
	busy func() bool,
	ping func(context.Context) error,
	closeNow func(),
) {
	keepAlive(ctx, interval, stop, busy, func() error {
		pctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		err := ping(pctx)
		if err == nil || !errors.Is(pctx.Err(), context.DeadlineExceeded) {
			return nil
		}
		return err
	}, closeNow)
}

// stampedWrite wraps a connection's write func so its writeActivity reports
// in-flight for exactly the duration of each call.
//
// A NAMED HELPER rather than two statements inside serve()'s closure, and the
// reason is measured rather than stylistic: with the stamping inline, DELETING
// the begin() call left the entire gateway suite green. The busy-skip is the
// whole purpose of writeActivity, and nothing anywhere observed that serve()
// actually stamps the real writer around the real write — the type was tested
// in isolation and keepAlive was tested with an injected predicate, so the one
// join between them was the one thing unpinned. That is the same shape this
// file already warns about twice, arriving one level up.
//
// defer, not a trailing call: a panic between begin and end would strand
// inFlight true, which disables that connection's keepalive permanently and
// silently. Unreachable today — the writer goroutine has no recover, so a panic
// there takes the process — but the deferred form costs nothing and removes the
// reasoning step.
func stampedWrite(a *writeActivity, write func([]byte) bool) func([]byte) bool {
	return func(b []byte) bool {
		a.begin()
		defer a.end()
		return write(b)
	}
}
