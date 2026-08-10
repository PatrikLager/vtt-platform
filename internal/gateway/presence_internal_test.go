package gateway

import (
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// The registry is the piece spec §4 is most specific about, and the piece
// whose defects are invisible from outside: a wrong reference count shows up
// as a player who is at the table but listed as gone, or gone and listed as
// present, with nothing failing anywhere.
//
// These are internal tests because the registry is unexported and its
// behaviour is a property of THIS package, not of the WS surface. The
// connection-lifecycle wiring is tested through the real server separately.

// join adapts the tests to joinAndSend, capturing the snapshot the registry
// builds so the assertions below stay about BEHAVIOUR rather than plumbing.
func join(r *presenceRegistry, c *presenceConn) (snapshot []*vttv1.PresenceChanged, first bool) {
	first = r.joinAndSend(c, func(present []*vttv1.PresenceChanged) []byte {
		snapshot = present
		return nil // nothing enqueued: these tests assert the snapshot's CONTENT
	})
	return snapshot, first
}

func conn(participantID, displayName string, buffer int) *presenceConn {
	return &presenceConn{
		participantID: participantID,
		displayName:   displayName,
		out:           make(chan []byte, buffer),
	}
}

func TestJoinReportsFirstConnectionOnlyOnce(t *testing.T) {
	// The reference count is PER PARTICIPANT (spec §4). One person on a
	// laptop and a phone is ONE arrival at the table, not two — announcing
	// twice would show them joining while they are already sitting there.
	r := newPresenceRegistry()

	if _, first := join(r, conn("p-1", "Ada", 1)); !first {
		t.Fatal("the first connection for a participant must report first=true")
	}
	if _, first := join(r, conn("p-1", "Ada", 1)); first {
		t.Fatal("a SECOND connection for the same participant must NOT report first=true")
	}
	if _, first := join(r, conn("p-2", "Bo", 1)); !first {
		t.Fatal("a different participant's first connection must report first=true")
	}
}

func TestLeaveReportsLastOnlyWhenEveryConnectionIsGone(t *testing.T) {
	// The mirror image, and the one that matters more: telling the table
	// someone left while they are still connected elsewhere is the failure
	// the per-participant count exists to prevent.
	r := newPresenceRegistry()
	laptop := conn("p-1", "Ada", 1)
	phone := conn("p-1", "Ada", 1)
	join(r, laptop)
	join(r, phone)

	if last := r.leave(laptop); last {
		t.Fatal("closing ONE of two connections must not report the participant gone")
	}
	if last := r.leave(phone); !last {
		t.Fatal("closing the LAST connection must report the participant gone")
	}
}

func TestLeaveIsIdempotent(t *testing.T) {
	// serve's teardown must be safe to reach by more than one path (a clean
	// quit and a force-close), so a second leave must not double-decrement —
	// which would make a LATER connection's close report "last" early and
	// announce a departure that never happened.
	r := newPresenceRegistry()
	c := conn("p-1", "Ada", 1)
	join(r, c)

	if last := r.leave(c); !last {
		t.Fatal("first leave must report last=true")
	}
	if last := r.leave(c); last {
		t.Fatal("leaving twice must not report last=true a second time")
	}
}

func TestLeaveOfAnUnknownConnectionIsNotADeparture(t *testing.T) {
	r := newPresenceRegistry()
	join(r, conn("p-1", "Ada", 1))

	if last := r.leave(conn("p-1", "Ada", 1)); last {
		t.Fatal("a connection that never joined must not evict the participant who did")
	}
}

func TestSnapshotListsEachParticipantOnceIncludingTheJoiner(t *testing.T) {
	// A picture of the TABLE, not of everyone else: a client should see
	// itself. And once per participant regardless of device count —
	// iterating connections alone would list a two-device participant twice.
	r := newPresenceRegistry()
	join(r, conn("p-1", "Ada", 1))
	join(r, conn("p-1", "Ada", 1)) // same person, second device

	snapshot, _ := join(r, conn("p-2", "Bo", 1))

	names := map[string]string{}
	for _, e := range snapshot {
		if _, dup := names[e.GetParticipantId()]; dup {
			t.Fatalf("participant %q appears twice in the snapshot", e.GetParticipantId())
		}
		names[e.GetParticipantId()] = e.GetDisplayName()
		if e.GetState() != vttv1.PresenceState_PRESENCE_STATE_CONNECTED {
			t.Fatalf("snapshot entry %q has state %v, want CONNECTED", e.GetParticipantId(), e.GetState())
		}
	}
	if len(names) != 2 {
		t.Fatalf("snapshot has %d participants, want 2: %v", len(names), names)
	}
	if names["p-2"] != "Bo" {
		t.Fatalf("the JOINER must be in its own snapshot, got %v", names)
	}
	if names["p-1"] != "Ada" {
		t.Fatalf("display names must be carried, got %v", names)
	}
}

func TestBroadcastReachesEveryoneButTheExcludedConnection(t *testing.T) {
	r := newPresenceRegistry()
	self := conn("p-1", "Ada", 1)
	other := conn("p-2", "Bo", 1)
	join(r, self)
	join(r, other)

	r.broadcast(self, []byte("frame"), nil)

	if len(self.out) != 0 {
		t.Fatal("the excluded connection must not receive its own announcement")
	}
	if len(other.out) != 1 {
		t.Fatalf("every other connection must receive it, got %d frames", len(other.out))
	}
}

func TestBroadcastReachesEverySecondDeviceOfTheSameParticipant(t *testing.T) {
	// Excluding by CONNECTION, not by participant id. A person on two devices
	// who acts on one must still see the result on the other.
	r := newPresenceRegistry()
	laptop := conn("p-1", "Ada", 1)
	phone := conn("p-1", "Ada", 1)
	join(r, laptop)
	join(r, phone)

	r.broadcast(laptop, []byte("frame"), nil)

	if len(phone.out) != 1 {
		t.Fatal("the same participant's OTHER connection must still receive the announcement")
	}
}

func TestBroadcastIsBoundedByAWedgedConnectionNotStalledByIt(t *testing.T) {
	// The property that keeps one stalled client from stalling the table.
	//
	// Named for what the code actually does: the send is BOUNDED, not
	// non-blocking. An instant drop was the first implementation and it was
	// wrong — outCh is filled by the CATCH-UP BACKLOG on every fresh connect,
	// so a full channel usually means "replaying a large campaign", not
	// "wedged", and review measured a catching-up client miss a joiner's
	// arrival outright with nothing to re-send it.
	//
	// What must still hold is that a client which never drains cannot hold the
	// announcement hostage: the budget expires, that frame is dropped, and
	// everyone else is served.
	r := newPresenceRegistry()
	r.sendBudget = 20 * time.Millisecond
	wedged := conn("p-1", "Ada", 0) // unbuffered and never read: cannot accept
	healthy := conn("p-2", "Bo", 1)
	join(r, wedged)
	join(r, healthy)

	start := time.Now()
	r.broadcast(nil, []byte("frame"), nil)
	elapsed := time.Since(start)

	if len(healthy.out) != 1 {
		t.Fatal("a wedged connection must not cost a healthy one its announcement")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("broadcast took %v — a wedged connection must not stall the table", elapsed)
	}
}

func TestBroadcastWaitsForAConnectionThatIsMerelyBusy(t *testing.T) {
	// The other half, and the one an instant drop got wrong: a client that is
	// briefly full but still draining must RECEIVE the announcement, not lose
	// it. This is the catching-up client from spec §3.4 — reconnection is
	// manual, so a dropped frame leaves the table wrong until the user acts.
	r := newPresenceRegistry()
	r.sendBudget = 2 * time.Second
	busy := conn("p-1", "Ada", 1)
	busy.out <- []byte("backlog") // full at the moment of broadcast

	join(r, busy)
	go func() {
		time.Sleep(20 * time.Millisecond)
		<-busy.out // drains, as a healthy reader does
	}()

	r.broadcast(nil, []byte("frame"), nil)

	// CONTENT, not depth. `len(busy.out) == 1` is satisfied by the leftover
	// "backlog" frame under an instant drop, so a depth assertion passes under
	// the exact defect this test is named for — measured: the drop leaves all
	// ten registry tests green and only the runtime changes, 0.02s to 0.00s.
	select {
	case got := <-busy.out:
		if string(got) != "frame" {
			t.Fatalf("channel holds %q, want the announcement — it was dropped, and the "+
				"backlog frame satisfied a depth check in its place", got)
		}
	default:
		t.Fatal("a connection that drains a moment later must still receive the announcement")
	}
}

func TestLeaveStopsDelivery(t *testing.T) {
	r := newPresenceRegistry()
	gone := conn("p-1", "Ada", 1)
	stays := conn("p-2", "Bo", 1)
	join(r, gone)
	join(r, stays)
	r.leave(gone)

	r.broadcast(nil, []byte("frame"), nil)

	if len(gone.out) != 0 {
		t.Fatal("a departed connection must not still be written to")
	}
	if len(stays.out) != 1 {
		t.Fatal("the remaining connection must still receive announcements")
	}
}

// TestBroadcastNeverTouchesAConnectionThatHasLeft pins the ORDERING that
// serve's teardown depends on: leave() must complete before outCh is closed.
//
// Nothing pinned it before. Moving leavePresence() back below <-writerDone —
// the exact pre-fix state — left the whole Go suite green across 96 executions
// while producing 16 "send on closed channel" panics, because `go test` hides a
// passing package's output and net/http's handler recovers the panic. The
// consequence is not the crash: map iteration order is random, so a panic
// mid-broadcast delivers the departure to SOME connections and never to the
// rest, leaving a permanent ghost.
//
// This works at the registry level, where the ordering actually lives: a
// broadcast that is holding the lock must not still be handing frames to a
// connection whose leave() has returned. Deliberately not a race-detector
// test, and the reason has CHANGED for the better: the gate ran no -race at
// all when this was written, so a pin that only failed under -race would not
// have failed anything. check:race fixed that on 2026-08-08. This stays as
// it is on the stronger argument — it is DETERMINISTIC where a race pin is
// probabilistic, failing every run rather than only the runs where the
// scheduler happens to cooperate.
func TestBroadcastNeverTouchesAConnectionThatHasLeft(t *testing.T) {
	r := newPresenceRegistry()
	r.sendBudget = time.Second

	// `leaver` never drains, so a broadcast to it parks under the lock — the
	// window in which serve would otherwise be closing its channel.
	leaver := conn("p-1", "Ada", 0)
	stays := conn("p-2", "Bo", 1)
	join(r, leaver)
	join(r, stays)

	left := make(chan struct{})
	go func() {
		defer close(left)
		r.leave(leaver) // blocks until the broadcast releases r.mu
		// Standing in for serve's close(outCh): only reached once leave has
		// returned, and by then no broadcast may hold this connection.
		close(leaver.out)
	}()

	r.broadcast(nil, []byte("frame"), nil)

	select {
	case <-left:
	case <-time.After(5 * time.Second):
		t.Fatal("leave never returned — broadcast is holding the registry lock indefinitely")
	}

	// A second broadcast must not send to the departed connection. Without the
	// leave-before-close ordering this is a send on a closed channel, which
	// panics rather than failing, and takes the announcement to `stays` with it.
	r.broadcast(nil, []byte("second"), nil)

	if len(stays.out) == 0 {
		t.Fatal("the remaining connection lost its announcement — a panic mid-broadcast " +
			"delivers to some connections and never to the rest")
	}
}

// TestAnnounceIfPresentSaysNothingAboutSomebodyWhoHasLEFT pins the ordering
// guarantee that makes a promotion announcement safe.
//
// Resolving the name, encoding, and sending used to be three steps with the
// lock released between them. If the participant's last connection unwound in
// that gap, the table received DISCONNECTED and then CONNECTED — in that order
// — and the client re-adds on CONNECTED. The departed participant stayed in
// everybody's list for the rest of the session: a permanent ghost, which is
// the precise failure presence exists to prevent.
//
// Under one hold that reordering is impossible, and this is the observable
// consequence: once leave has returned, no announcement for that participant
// can be produced at all.
func TestAnnounceIfPresentSaysNothingAboutSomebodyWhoHasLEFT(t *testing.T) {
	r := newPresenceRegistry()
	watcher := &presenceConn{participantID: "p-watch", displayName: "Zoe", out: make(chan []byte, 4)}
	going := &presenceConn{participantID: "p-go", displayName: "Kim", out: make(chan []byte, 4)}
	r.joinAndSend(watcher, func([]*vttv1.PresenceChanged) []byte { return nil })
	r.joinAndSend(going, func([]*vttv1.PresenceChanged) []byte { return nil })

	if !r.announceIfPresent("p-go", func(name string) []byte {
		if name != "Kim" {
			t.Errorf("announced name = %q, want Kim", name)
		}
		return []byte("promoted")
	}) {
		t.Fatal("a connected participant must be announceable")
	}
	if got := <-watcher.out; string(got) != "promoted" {
		t.Fatalf("the table got %q", got)
	}

	// They leave. Any announcement AFTER this would arrive as a CONNECTED for
	// somebody the table has already been told is gone.
	r.leave(going)
	built := false
	if r.announceIfPresent("p-go", func(string) []byte {
		built = true
		return []byte("ghost")
	}) {
		t.Fatal("announced somebody who has left — the table will re-add them and never " +
			"hear otherwise, because only a snapshot can replace the list")
	}
	if built {
		t.Fatal("the frame must not even be built for a participant who is gone")
	}
	select {
	case got := <-watcher.out:
		t.Fatalf("the table was sent %q about a participant who had left", got)
	default:
	}
}

func TestBroadcastSkipsAnybodyTheCallerHasDenied(t *testing.T) {
	// How revocation reaches presence. Frames written here never travel the
	// pump, so the per-event re-resolution cannot see them: a revoked stranger
	// on a leaked link went on watching the guest list until the table next
	// appended an event.
	r := newPresenceRegistry()
	staying := &presenceConn{participantID: "p-ok", displayName: "Zoe", out: make(chan []byte, 4)}
	revoked := &presenceConn{participantID: "p-gone", displayName: "Stranger", out: make(chan []byte, 4)}
	r.joinAndSend(staying, func([]*vttv1.PresenceChanged) []byte { return nil })
	r.joinAndSend(revoked, func([]*vttv1.PresenceChanged) []byte { return nil })

	r.broadcast(nil, []byte("someone arrived"), map[string]bool{"p-gone": true})

	if got := <-staying.out; string(got) != "someone arrived" {
		t.Fatalf("a participant in good standing got %q", got)
	}
	select {
	case got := <-revoked.out:
		t.Fatalf("a revoked participant was sent %q — they are meant to see nothing", got)
	default:
	}

	// And a nil deny denies nobody, or every ordinary broadcast would stop.
	r.broadcast(nil, []byte("second"), nil)
	if got := <-revoked.out; string(got) != "second" {
		t.Fatalf("nil deny must deny nobody, got %q", got)
	}
}
