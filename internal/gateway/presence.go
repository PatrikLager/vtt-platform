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
	// LIFETIME, which is the part that bit: the registry holds this channel
	// but does NOT own it. serve owns its close. That is only safe because
	// serve deregisters (taking r.mu) BEFORE closing it, and every send here
	// happens under the same lock — so a connection still in the registry
	// cannot have had its channel closed underneath it. Reordering those two
	// steps reintroduces "send on closed channel".
	out chan []byte
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
// what it returns — all under one lock — then reports whether this is the
// participant's FIRST live connection, the only case in which the table learns
// someone arrived.
//
// Building and enqueueing the snapshot INSIDE the critical section is the
// point. Registration makes c eligible for deltas immediately; if the snapshot
// were enqueued after the lock dropped, a delta could reach the wire first and
// the snapshot behind it would already be stale. A client applying
// snapshot-then-deltas would then show someone who has left as present, and
// nothing would ever correct it.
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

	if b := frame(present); b != nil {
		r.send(c, b)
	}
	return first
}

// send hands b to c, waiting at most presenceSendBudget.
//
// Callers hold r.mu, and that is what makes the send safe rather than merely
// convenient: serve deregisters a connection (taking this same lock) BEFORE it
// closes outCh, so a connection still in the registry cannot have had its
// channel closed underneath us.
func (r *presenceRegistry) send(c *presenceConn, b []byte) {
	timer := time.NewTimer(r.sendBudget)
	defer timer.Stop()
	select {
	case c.out <- b:
	case <-timer.C:
	}
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
	r.counts[c.participantID]--
	if r.counts[c.participantID] <= 0 {
		delete(r.counts, c.participantID)
		return true
	}
	return false
}

// announceIfPresent re-announces participantID to every connection, their own
// included, under ONE HOLD of the lock. Reports whether anything was sent.
//
// The single hold is the entire point, and it was learned the hard way. The
// caller used to resolve the display name, drop the lock to encode, then take
// it again to broadcast — and if that participant's last connection unwound in
// the gap, the table received DISCONNECTED and then CONNECTED, in that order.
// The client re-adds on CONNECTED and only a snapshot can replace the list, so
// the departed participant stayed listed for the rest of the session. A ghost,
// permanently, which is the exact failure presence exists to prevent. Found by
// review at 2-6 occurrences per 3000 promotions, WITHOUT injection — a real
// race between two real goroutines.
//
// False when nobody by that id is connected. Nothing is lost: they read their
// role fresh the moment they next arrive.
//
// frame is called with the lock HELD, like joinAndSend's, and may return nil
// on an encode failure — in which case nothing is sent.
func (r *presenceRegistry) announceIfPresent(participantID string, frame func(displayName string) []byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	name, found := "", false
	for conn := range r.conns {
		if conn.participantID == participantID {
			name, found = conn.displayName, true
			break
		}
	}
	if !found {
		return false
	}
	b := frame(name)
	if b == nil {
		return false
	}
	// Everyone, excluding nobody: the promoted participant is the one who most
	// needs this, and their other devices need it too.
	for conn := range r.conns {
		r.send(conn, b)
	}
	return true
}

// participantIDs returns the distinct participants holding a live connection.
//
// Exists so a caller can resolve identity for all of them OUTSIDE the lock and
// come back with the answer: a Lookup inside broadcast's loop would put one
// SQLite read per connection under the registry's global mutex, which is the
// fan-out stall presenceSendBudget exists to prevent.
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
	r.mu.Lock()
	defer r.mu.Unlock()

	for conn := range r.conns {
		if conn == except || deny[conn.participantID] {
			continue
		}
		r.send(conn, b)
	}
}
