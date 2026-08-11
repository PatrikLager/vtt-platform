package gateway

import (
	"sync"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// presenceConn is one live connection's slot in the registry.
//
// Identity is the POINTER, never the participant id: the same participant may
// hold several connections at once (a laptop and a phone), and they must be
// distinguishable so that closing one does not deregister the other.
type presenceConn struct {
	participantID string
	displayName   string
	// out is the connection's outCh — the same channel the command loop and
	// the event pump feed, so presence obeys the one-writer discipline that
	// keeps two writes off the wire at once, and so a snapshot cannot be
	// overtaken by a delta that belongs after it.
	//
	// LIFETIME, which is the part that bit twice. The registry holds this
	// channel but does not own it, and NOBODY CLOSES IT. That is deliberate:
	// sends now happen outside r.mu (see broadcast), so the old interlock —
	// deregister under the lock, then close — no longer holds, and no select
	// can rescue a sender from it either. Closing a channel while a goroutine
	// is parked sending on it panics THAT GOROUTINE, whatever else its select
	// was watching. The only way to make an out-of-lock send safe is for the
	// close never to happen; a frame written after teardown lands in the
	// buffer and is collected with the connection.
	out chan []byte

	// done is closed by serve when this connection is finished. A fan-out that
	// snapshotted this connection just before it left still holds the pointer;
	// without done it would spend the full budget on a connection that is
	// already unwinding, and the rest of the table would wait behind it.
	done chan struct{}
}

// presenceRegistry tracks who is currently connected.
//
// This is WIRE state, not campaign state: it is never appended to the log and
// does not survive a restart. Who is connected right now is not a fact about
// the campaign's history, and replaying a campaign must not resurrect a
// session (spec §4).
//
// Reference counting is PER PARTICIPANT, not per connection, which is the
// whole reason counts exists alongside conns. Emitting DISCONNECTED when any
// one connection closes would tell the table someone left while they are still
// sitting there on another device.
type presenceRegistry struct {
	// fanOut serialises whole fan-outs against each other. mu protects the
	// maps. They were ONE lock until #47, and separating them is the fix.
	//
	// Presence deltas are not commutative: a client re-adds a participant on
	// CONNECTED and only a snapshot can replace the list, so a CONNECTED
	// delivered after a DISCONNECTED for the same participant leaves them
	// listed for the rest of the session. One lock held across the whole walk
	// gave that ordering for free — and charged for it, because joinAndSend
	// takes the same lock and runs before the joining connection's pump
	// exists, so anyone arriving mid-fan-out waited N x the budget on a
	// stranger's slow reader. Measured at 1.9s against a 2s budget.
	//
	// So: fanOut gives the ordering, mu gives the map safety, and a joiner
	// needs only mu. Lock order is fanOut -> mu, never the reverse.
	// joinAndSend takes mu alone, which is what lets it past a fan-out in
	// progress, and is why there is no cycle to deadlock on.
	fanOut sync.Mutex

	mu     sync.Mutex
	conns  map[*presenceConn]struct{}
	counts map[string]int

	// sendBudget is presenceSendBudget, overridable by tests. A test that
	// wedges a connection deliberately would otherwise pay the full
	// production budget in wall-clock time for every such case.
	sendBudget time.Duration
}

func newPresenceRegistry() *presenceRegistry {
	return &presenceRegistry{
		conns:      make(map[*presenceConn]struct{}),
		counts:     make(map[string]int),
		sendBudget: presenceSendBudget,
	}
}

// presenceSendBudget bounds how long one connection may hold up an
// announcement to the rest of the table.
//
// It is NOT a liveness check — writeTimeout is, and it force-closes a client
// that has genuinely stopped reading. This budget exists only so that one
// slow reader cannot stall a broadcast for everyone, which is the fan-out
// wedge the store's per-subscriber queue was built to prevent (PR #18) and
// which must not be reintroduced one layer up.
//
// Why a WAIT and not an instant drop: outCh is filled by the CATCH-UP BACKLOG
// on every fresh connect, so a full channel routinely means "this client is
// replaying a large campaign", not "this client is wedged". Review measured a
// client draining 400 backlogged events miss a joiner's arrival outright under
// an instant drop, and nothing re-sends it — reconnection is manual by spec
// §3.4, so the table stayed wrong until the user acted. A few seconds covers
// a backlog hand-off with room to spare; the socket drains far faster than
// this.
//
// A frame IS still dropped if the budget expires. That client is not draining
// at all and is on its way out under writeTimeout, and its replacement
// connection opens with a fresh snapshot.
const presenceSendBudget = 3 * time.Second

// joinAndSend registers c, hands frame the resulting snapshot, and enqueues
// what it returns — all under r.mu — then reports whether this is the
// participant's FIRST live connection, the only case in which the table learns
// someone arrived.
//
// It takes r.mu and DELIBERATELY NOT fanOut, which is the whole of #47's fix.
// This runs before the joining connection's pump exists, so while it waits
// nobody is being served; holding it behind a stranger's slow reader cost a
// measured 1.9s against a 2s budget, and six seconds in production.
//
// Building and enqueueing the snapshot INSIDE the critical section is still
// the point, and still works with sends outside the lock. Registration makes c
// eligible for deltas immediately; if the snapshot were enqueued after the
// lock dropped, a delta could reach the wire first and the snapshot behind it
// would already be stale. A client applying snapshot-then-deltas would then
// show someone who has left as present, and nothing would ever correct it.
// A fan-out reads membership under r.mu too, so it either misses c entirely —
// c was not registered yet, so there is no delta to miss — or it sees c and
// necessarily sends after this returns. There is no third case: the register
// and the enqueue are not separable from outside.
//
// The snapshot lists every participant present, c INCLUDED, exactly once
// however many connections each holds: a picture of the table, not of everyone
// else. frame may return nil (encode failure), in which case nothing is sent —
// presence is soft state and a connection that can still do everything else
// should not be refused over it.
func (r *presenceRegistry) joinAndSend(c *presenceConn, frame func([]*vttv1.PresenceChanged) []byte) (first bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.conns[c] = struct{}{}
	r.counts[c.participantID]++
	first = r.counts[c.participantID] == 1

	// Deduplicated by participant id. Iterating conns alone would list a
	// participant once per device they are on.
	var present []*vttv1.PresenceChanged
	seen := make(map[string]bool, len(r.counts))
	for conn := range r.conns {
		if seen[conn.participantID] {
			continue
		}
		seen[conn.participantID] = true
		present = append(present, &vttv1.PresenceChanged{
			ParticipantId: conn.participantID,
			DisplayName:   conn.displayName,
			State:         vttv1.PresenceState_PRESENCE_STATE_CONNECTED,
		})
	}

	// THE ONE BLOCKING SEND STILL UNDER r.mu, and it is bounded by a fact
	// rather than by the budget: this runs before the connection's pump exists,
	// out holds at most the catch-up head, and gatewayBuffer is 256 — so 255
	// slots are free and this cannot park. Written down because it is not
	// visible from here, and because the alternative reading (a joiner can
	// stall the whole registry) IS true if the buffer is small: review measured
	// leave() blocked 1.95s and every fan-out behind it with buffer 0, which
	// four tests in this package deliberately set. Production never does. If
	// that ever changes, this send needs its own tight bound — the snapshot is
	// worth dropping, the registry is not worth freezing.
	if b := frame(present); b != nil {
		r.send(c, b)
	}
	return first
}

// send hands b to c, waiting at most presenceSendBudget.
//
// Safe WITHOUT r.mu, which is the whole point of #47 and needs saying because
// the previous version's safety came from the opposite claim. It rests on out
// never being closed (see presenceConn.out): a send to a connection that has
// already gone lands in a buffer nobody will read, which costs a garbage frame
// and nothing else. done is what stops it costing the budget as well.
func (r *presenceRegistry) send(c *presenceConn, b []byte) {
	timer := time.NewTimer(r.sendBudget)
	defer timer.Stop()
	select {
	case c.out <- b:
	case <-c.done:
	case <-timer.C:
	}
}

// targets snapshots the connections a fan-out should reach, so the walk itself
// can happen with r.mu released.
//
// except is excluded by CONNECTION POINTER, not by participant id: someone on
// two devices who acts on one must still see the result on the other. deny
// carries PARTICIPANT ids and may be nil, which denies nobody.
func (r *presenceRegistry) targets(except *presenceConn, deny map[string]bool) []*presenceConn {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]*presenceConn, 0, len(r.conns))
	for conn := range r.conns {
		if conn == except || deny[conn.participantID] {
			continue
		}
		out = append(out, conn)
	}
	return out
}

// leave deregisters c and reports whether that was the participant's LAST live
// connection. Idempotent: leaving twice is not an error and does not
// double-decrement, because serve's teardown must be safe to reach by more
// than one path.
func (r *presenceRegistry) leave(c *presenceConn) (last bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.conns[c]; !ok {
		return false
	}
	delete(r.conns, c)
	// Closed HERE, behind the membership check, so "exactly once" is a
	// consequence of leave's idempotence rather than a separate promise. serve
	// reaches teardown by two paths (shutdown, and the defer that backstops the
	// returns which never get there), and closing an already-closed channel
	// panics. A sync.Once at the call site also works and is what this was
	// first written as — but a panic inside serve is RECOVERED BY net/http, so
	// nothing failed when the guard was removed and the injection proving it
	// survived. Here, TestLeaveIsIdempotent calls leave twice directly and the
	// test binary takes the panic.
	close(c.done)
	r.counts[c.participantID]--
	if r.counts[c.participantID] <= 0 {
		delete(r.counts, c.participantID)
		return true
	}
	return false
}

// announceIfPresent re-announces participantID to every connection, their own
// included, as ONE UNINTERRUPTIBLE FAN-OUT. Reports whether anything was sent.
//
// Indivisibility is the entire point, and it was learned the hard way. The
// caller used to resolve the display name, drop the lock to encode, then take
// it again to broadcast — and if that participant's last connection unwound in
// the gap, the table received DISCONNECTED and then CONNECTED, in that order.
// The client re-adds on CONNECTED and only a snapshot can replace the list, so
// the departed participant stayed listed for the rest of the session. A ghost,
// permanently, which is the exact failure presence exists to prevent. Found by
// review at 2-6 occurrences per 3000 promotions, WITHOUT injection — a real
// race between two real goroutines.
//
// WHAT PROVIDES IT NOW IS fanOut, NOT r.mu, and the ordering of the two locks
// is what makes that equivalent. Taking fanOut FIRST means a departure cannot
// interleave: leave() takes r.mu and removes the connection, but the
// DISCONNECTED that follows is a fan-out and must queue behind this one. So
// either we resolve them as present and our frame lands ahead of their
// DISCONNECTED — correct, the client shows them, then removes them — or the
// departure got here first, we find nobody by that id, and send nothing.
// Neither order resurrects anyone. Reversing the two locks reopens the ghost.
//
// False when nobody by that id is connected. Nothing is lost: they read their
// role fresh the moment they next arrive.
//
// frame is called with NO lock held (it may return nil on an encode failure,
// in which case nothing is sent) — the atomicity that matters is against other
// fan-outs, and fanOut is still held throughout.
func (r *presenceRegistry) announceIfPresent(participantID string, frame func(displayName string) []byte) bool {
	r.fanOut.Lock()
	defer r.fanOut.Unlock()

	r.mu.Lock()
	name, found := "", false
	for conn := range r.conns {
		if conn.participantID == participantID {
			name, found = conn.displayName, true
			break
		}
	}
	// Everyone, excluding nobody: the promoted participant is the one who most
	// needs this, and their other devices need it too.
	var targets []*presenceConn
	if found {
		targets = make([]*presenceConn, 0, len(r.conns))
		for conn := range r.conns {
			targets = append(targets, conn)
		}
	}
	r.mu.Unlock()

	if !found {
		return false
	}
	b := frame(name)
	if b == nil {
		return false
	}
	for _, conn := range targets {
		r.send(conn, b)
	}
	return true
}

// announceIfAbsent tells the table a participant has gone — but only if they
// are STILL gone when the announcement is about to leave. Reports whether
// anything was sent.
//
// The mirror of announceIfPresent, and it closes the mirror-image defect (#55).
// leave() decides "that was their last connection" and the announcement is a
// SEPARATE step, so a reconnect landing in between leaves the table's last word
// about a present participant "DISCONNECTED". Worse, the fresh connection is
// told ITSELF gone: broadcast excludes by connection POINTER, so the
// participant's own new connection is a legitimate target. Only a snapshot can
// correct it, and spec §3.4 makes reconnection a MANUAL act — so it stays wrong
// until the player does something about a problem they cannot see.
//
// MEASURED before fixing, driving both sides exactly as serve does: 1 inversion
// in 20,000 rounds. Rare because the reference count usually saves it — a
// reconnect that wins the lock makes leave() return last=false and no departure
// is announced at all. The case that bites is leave winning and its
// announcement losing.
//
// The re-check is the whole fix, and it is cheap because absence is already the
// registry's own question. fanOut FIRST, then mu, exactly as announceIfPresent:
// reversing them reopens the ghost #47 closed.
//
// frame is called with NO lock held, and only when there is something to say.
// deny may be nil, which denies nobody.
func (r *presenceRegistry) announceIfAbsent(participantID string, deny map[string]bool, frame func() []byte) bool {
	r.fanOut.Lock()
	defer r.fanOut.Unlock()

	r.mu.Lock()
	// PER PARTICIPANT, not per connection: counts is the reference count leave
	// maintains, so this asks "is anybody by that id still here" rather than
	// "did this socket go away".
	_, present := r.counts[participantID]
	var targets []*presenceConn
	if !present {
		targets = make([]*presenceConn, 0, len(r.conns))
		for conn := range r.conns {
			// deny carries PARTICIPANT ids and must be resolved by the caller,
			// BEFORE fanOut is taken: it needs an identity read per
			// participant, and doing that in here would put one SQLite query
			// per connection inside the fan-out, with every other announcement
			// queued behind it. See participantIDs.
			if deny[conn.participantID] {
				continue
			}
			targets = append(targets, conn)
		}
	}
	r.mu.Unlock()

	if present {
		return false
	}
	b := frame()
	if b == nil {
		return false
	}
	for _, conn := range targets {
		r.send(conn, b)
	}
	return true
}

// participantIDs returns the distinct participants holding a live connection.
//
// Exists so a caller can resolve identity for all of them BEFORE broadcasting
// and come back with the answer: a Lookup inside broadcast's loop would put
// one SQLite read per connection inside the fan-out, serialised behind fanOut
// with every other announcement waiting on it.
func (r *presenceRegistry) participantIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	seen := make(map[string]bool, len(r.counts))
	out := make([]string, 0, len(r.counts))
	for conn := range r.conns {
		if seen[conn.participantID] {
			continue
		}
		seen[conn.participantID] = true
		out = append(out, conn.participantID)
	}
	return out
}

// broadcast hands b to every registered connection EXCEPT except and except
// anyone in deny, bounded per connection by presenceSendBudget so one slow
// reader cannot stall the table.
//
// Excluding by CONNECTION POINTER, not by participant id: someone on two
// devices who acts on one must still see the result on the other.
//
// deny carries PARTICIPANT ids and is how revocation reaches this path. A
// revoked participant stops receiving events on the pump, but presence frames
// never travel that channel — they are written straight into each connection's
// out — so without this they went on watching the guest list arrive and leave
// until the table next appended something. deny may be nil, which denies
// nobody. Resolved by the caller, never here: see participantIDs.
func (r *presenceRegistry) broadcast(except *presenceConn, b []byte, deny map[string]bool) {
	// fanOut for the whole walk, r.mu only to read the membership. A joiner
	// needs r.mu alone, so it no longer queues behind these sends; other
	// fan-outs still queue behind this one, which is what keeps the table
	// agreeing about the order presence changed in.
	r.fanOut.Lock()
	defer r.fanOut.Unlock()

	for _, conn := range r.targets(except, deny) {
		r.send(conn, b)
	}
}
