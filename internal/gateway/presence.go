package gateway

import (
	"sync"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Key the registry on the pointer, never the participant id: one participant
// may hold several connections (SPEC-011).
type presenceConn struct {
	participantID string
	displayName   string
	// Never close out: presence sends outside r.mu, and closing a channel under a
	// parked sender panics that goroutine (SPEC-011).
	out chan []byte

	done chan struct{}
}

type presenceRegistry struct {
	// Lock order is fanOut then mu, never the reverse; joinAndSend takes mu alone
	// (SPEC-011).
	fanOut sync.Mutex

	mu     sync.Mutex
	conns  map[*presenceConn]struct{}
	counts map[string]int

	sendBudget time.Duration
}

func newPresenceRegistry() *presenceRegistry {
	return &presenceRegistry{
		conns:      make(map[*presenceConn]struct{}),
		counts:     make(map[string]int),
		sendBudget: presenceSendBudget,
	}
}

// Not a liveness check, and never an instant drop: a fresh connection's outCh
// holds its catch-up backlog, and a dropped snapshot is never re-sent (SPEC-011).
const presenceSendBudget = 3 * time.Second

// Take mu alone, never fanOut: this runs before the joining pump exists, so a
// fan-out parked on a slow reader would stall the joiner (SPEC-011).
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

	// The one send under r.mu. It cannot park while gatewayBuffer leaves room
	// beside the head frame; a buffer of 0, which tests set, can block it for up
	// to the budget while the writer is parked. Bound this send before shrinking
	// the buffer.
	if b := frame(present); b != nil {
		r.send(c, b)
	}
	return first
}

// Safe without r.mu only because out is never closed (presenceConn.out).
func (r *presenceRegistry) send(c *presenceConn, b []byte) {
	timer := time.NewTimer(r.sendBudget)
	defer timer.Stop()
	select {
	case c.out <- b:
	case <-c.done:
	case <-timer.C:
	}
}

// Exclude by connection pointer and deny by participant id: a participant's
// other devices must see what one of them caused (SPEC-011).
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

func (r *presenceRegistry) leave(c *presenceConn) (last bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.conns[c]; !ok {
		return false
	}
	delete(r.conns, c)
	// Close done here, behind the membership check, and nowhere else: serve reaches
	// teardown by two paths, and net/http recovers a panic in serve, so a double
	// close there fails no test (TestLeaveIsIdempotent).
	close(c.done)
	r.counts[c.participantID]--
	if r.counts[c.participantID] <= 0 {
		delete(r.counts, c.participantID)
		return true
	}
	return false
}

// Take fanOut before mu and hold it through the sends: a departure's DISCONNECTED
// must queue behind this frame, or the table keeps a ghost (SPEC-011).
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

// Re-check under fanOut then mu: a reconnect landing between leave and this
// announcement must find its participant present and send nothing (SPEC-011).
func (r *presenceRegistry) announceIfAbsent(participantID string, deny map[string]bool, frame func() []byte) bool {
	r.fanOut.Lock()
	defer r.fanOut.Unlock()

	r.mu.Lock()
	_, present := r.counts[participantID]
	var targets []*presenceConn
	if !present {
		targets = make([]*presenceConn, 0, len(r.conns))
		for conn := range r.conns {
			// deny is resolved by the caller before fanOut is taken: an identity read per
			// connection in here would sit inside every fan-out (participantIDs).
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

// Callers resolve the deny set from this before taking fanOut, never inside a
// fan-out (announceIfAbsent, broadcast).
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

func (r *presenceRegistry) broadcast(except *presenceConn, b []byte, deny map[string]bool) {
	// fanOut for the whole walk; mu only inside targets, so a joiner never queues
	// behind these sends (SPEC-011).
	r.fanOut.Lock()
	defer r.fanOut.Unlock()

	for _, conn := range r.targets(except, deny) {
		r.send(conn, b)
	}
}
