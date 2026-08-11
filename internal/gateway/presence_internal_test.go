package gateway

import (
	"sync"
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
		// Open, never closed: these connections are live for the whole test.
		// A nil done would also work — a nil channel blocks forever, so the
		// select simply never picks it — but it would work by accident, and
		// the first test to close one would find the field it needed missing.
		done: make(chan struct{}),
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

// TestAFanOutAbandonsAConnectionThatLeftMidWalk replaces
// TestBroadcastNeverTouchesAConnectionThatHasLeft, and the replacement is the
// honest consequence of #47 rather than a relaxation.
//
// The old test simulated serve closing outCh right after leave() and asserted
// that a later broadcast did not panic on it. That ordering was the ONLY thing
// making an out-of-lock close survivable, and #47 removed the premise: sends no
// longer happen under the registry lock, so no ordering can save them. Run the
// old test against this code and it panics — not as a regression, but because
// it performs the very close the new design forbids, while a fan-out is parked
// mid-send. MEASURED, because the first draft of this said "it panics" flatly
// and that overstates it: about one run in a hundred under -race, since leave()
// now closes done first and usually wakes the parked sender through that case
// before the close lands. The narrow path is enough. Closing a channel out from
// under a parked sender panics THAT sender whatever its select is watching, so
// "guard the close" was never on the table and "never close" is the only fix.
//
// So the property moves. serve now closes done, not out, and what must hold is
// that a fan-out already holding a departed connection LETS GO of it — quickly,
// and without dropping the connections behind it in the walk. That last part is
// the original point, preserved: a fan-out that dies partway through delivers
// the departure to some connections and never to the rest, which is a permanent
// ghost at the table.
//
// Deterministic, not a race-detector test: it fails every run rather than only
// the runs where the scheduler cooperates.
func TestAFanOutAbandonsAConnectionThatLeftMidWalk(t *testing.T) {
	r := newPresenceRegistry()
	// Deliberately far longer than the assertions below. If done were ignored,
	// the fan-out would take this long and the failure would be unambiguous —
	// "abandoned" cannot be confused with "the budget happened to expire".
	r.sendBudget = 5 * time.Second

	leaver := conn("p-1", "Ada", 0) // unbuffered, never drained: the walk parks here
	// Room for BOTH announcements below. At depth 1 the second broadcast has
	// nowhere to put its frame, parks for the whole budget and drops it — which
	// reads as "the fan-out skipped it" and blames the code for the fixture.
	stays := conn("p-2", "Bo", 2)
	join(r, leaver)
	join(r, stays)

	fanOutDone := make(chan struct{})
	go func() {
		defer close(fanOutDone)
		r.broadcast(nil, []byte("frame"), nil)
	}()
	time.Sleep(20 * time.Millisecond) // let the walk reach its parked send

	// Exactly what serve's teardown does, in order, and NOT close(leaver.out):
	// nothing closes a connection's out channel any more.
	start := time.Now()
	r.leave(leaver) // closes leaver.done itself, behind its membership check
	leaveElapsed := time.Since(start)

	select {
	case <-fanOutDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the fan-out never let go of a connection that left mid-walk — done is " +
			"not being watched, so a departing client costs the whole table the budget")
	}
	if leaveElapsed > time.Second {
		t.Fatalf("leave() took %v while a fan-out was mid-walk — teardown is queued "+
			"behind a stranger's slow reader", leaveElapsed)
	}

	// The original assertion, and the one that matters: a fan-out that gives up
	// on one connection must still serve the rest of its walk.
	if len(stays.out) != 1 {
		t.Fatalf("the remaining connection holds %d frames, want 1 — a fan-out that "+
			"abandons one connection must not abandon the table with it", len(stays.out))
	}

	// And a later fan-out must not consider the departed connection at all.
	r.broadcast(nil, []byte("second"), nil)
	if len(stays.out) != 2 {
		t.Fatal("the remaining connection lost a later announcement")
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
// What forbids the reordering is now fanOut rather than one hold of the
// registry lock (#47 moved sends out from under it), but the observable
// consequence is the same and is what this asserts: once leave has returned, no
// announcement for that participant can be produced at all. Stated as an
// OUTCOME on purpose — it survived the locking being rebuilt underneath it,
// which a test phrased in terms of "the lock is held here" would not have.
func TestAnnounceIfPresentSaysNothingAboutSomebodyWhoHasLEFT(t *testing.T) {
	r := newPresenceRegistry()
	// Via the helper, not a literal: leave() closes done, so a connection built
	// without one panics on close(nil) the moment it departs.
	watcher := conn("p-watch", "Zoe", 4)
	going := conn("p-go", "Kim", 4)
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
	staying := conn("p-ok", "Zoe", 4)
	revoked := conn("p-gone", "Stranger", 4)
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

func TestAJoinerDoesNotWaitOutSomebodyElsesFanOut(t *testing.T) {
	// #47. #36 stopped a joiner waiting on its OWN announcement; this is the
	// same wait arriving by a different route, and that fix did not touch it.
	//
	// joinAndSend and broadcast take the SAME lock, and broadcast holds it for
	// its entire serial walk — up to the budget per connection. So anyone
	// arriving inside somebody else's fan-out pays for a stranger's slow
	// reader, before their own pump even exists. Measured at 1.897–1.910s
	// against a 2s budget, 3/3, by the review that found it. In production
	// (3s budget, two dead tabs) that is the same six seconds #36 was about.
	r := newPresenceRegistry()
	r.sendBudget = 500 * time.Millisecond
	wedged := conn("p-1", "Ada", 0) // unbuffered, never read: the fan-out parks here
	join(r, wedged)

	fanOutDone := make(chan struct{})
	go func() {
		defer close(fanOutDone)
		r.broadcast(nil, []byte("frame"), nil)
	}()
	// Long enough for the fan-out to be parked in its send, short enough that
	// the remaining budget dwarfs the threshold below.
	time.Sleep(50 * time.Millisecond)

	// Checked BEFORE the measurement, not after, and the difference is the
	// whole test. This is what stops it passing for the wrong reason: if the
	// fan-out had already finished, the joiner never overlapped it and the
	// timing below would pass on any implementation at all. It cannot be
	// checked afterwards — under the defect, join BLOCKS until the fan-out
	// completes, so by then fanOutDone is always closed and the guard would
	// fire on correct and broken code alike.
	select {
	case <-fanOutDone:
		t.Fatal("the fan-out finished before the joiner arrived — this run did not " +
			"exercise the overlap the test is named for, so its timing proves nothing")
	default:
	}

	joiner := conn("p-2", "Bo", 1)
	start := time.Now()
	join(r, joiner)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Fatalf("joinAndSend took %v while another connection's fan-out held the "+
			"registry lock (budget %v). A joiner must not wait out a stranger's slow reader.",
			elapsed, r.sendBudget)
	}
	<-fanOutDone
}

func TestConcurrentFanOutsReachEveryConnectionInTheSameOrder(t *testing.T) {
	// Measured by MUTUAL EXCLUSION, not by observed order, and the first draft
	// of this test is why. It ran two fan-outs at eight fast connections and
	// compared what each received — but two fan-outs that never block simply run
	// one after the other on any implementation, so it agreed with itself
	// whatever the locking did. It would have passed with fanOut deleted.
	//
	// A connection that NEVER drains makes the cost visible instead: every
	// fan-out must park on it for the whole budget, so a serialised pair costs
	// two budgets and an interleaved pair costs one. No drainer, no sleep, no
	// scheduling luck — the difference is the budget itself.
	const budget = 100 * time.Millisecond
	r := newPresenceRegistry()
	r.sendBudget = budget

	wedged := conn("p-1", "Ada", 0) // unbuffered, never drained
	other := conn("p-2", "Bo", 2)
	join(r, wedged)
	join(r, other)

	start := time.Now()
	var wg sync.WaitGroup
	for _, b := range [][]byte{[]byte("first"), []byte("second")} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.broadcast(nil, b, nil)
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	if elapsed < 2*budget-budget/5 {
		t.Fatalf("two fan-outs finished in %v, about one budget of %v rather than two. "+
			"They overlapped, so presence deltas are no longer ordered against each "+
			"other — and they are not commutative: a client re-adds a participant on "+
			"CONNECTED and only a snapshot can replace the list, so a CONNECTED "+
			"delivered after a DISCONNECTED leaves a permanent ghost at the table.",
			elapsed, budget)
	}

	// And the consequence that matters, on the connection that could receive:
	// both frames, in one order, whichever fan-out won the race.
	if len(other.out) != 2 {
		t.Fatalf("the healthy connection holds %d frames, want 2", len(other.out))
	}
	got := string(<-other.out) + "," + string(<-other.out)
	if got != "first,second" && got != "second,first" {
		t.Fatalf("healthy connection saw %q — a fan-out delivered a partial pair", got)
	}
}

func TestAPromotionInFlightIsNotOvertakenByTheDeparture(t *testing.T) {
	// The reason announceIfPresent takes fanOut BEFORE mu. Review reversed the
	// two and the entire package still passed: the invariant the whole comment
	// block above announceIfPresent hangs on — and the re-fix of 656079f's
	// second defect — was defended by nothing at all.
	//
	// Reversed, this is deterministic: resolve p-go present and release mu,
	// let the departure take mu, remove them, and fan out DISCONNECTED, then
	// deliver the CONNECTED behind it. The client re-adds on CONNECTED and only
	// a snapshot can replace the list, so p-go is a ghost for the session.
	//
	// The seam is that frame runs UNDER fanOut, so the test can hold a fan-out
	// open from inside it and drive the interleaving rather than sleeping at it.
	r := newPresenceRegistry()
	r.sendBudget = time.Second
	watcher, going := conn("p-watch", "Zoe", 4), conn("p-go", "Kim", 4)
	join(r, watcher)
	join(r, going)

	resolved, release, promoDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(promoDone)
		r.announceIfPresent("p-go", func(string) []byte {
			close(resolved)
			<-release // hold the fan-out open across the departure below
			return []byte("CONNECTED p-go")
		})
	}()
	<-resolved

	depDone := make(chan struct{})
	go func() {
		defer close(depDone)
		r.leave(going)
		r.broadcast(going, []byte("DISCONNECTED p-go"), nil)
	}()
	// Long enough for the departure to get as far as the locking lets it. With
	// fanOut taken first that is "queued"; reversed, it is "delivered".
	time.Sleep(50 * time.Millisecond)
	close(release)
	<-promoDone
	<-depDone

	var got []string
	for len(watcher.out) > 0 {
		got = append(got, string(<-watcher.out))
	}
	if len(got) == 2 && got[0] != "CONNECTED p-go" {
		t.Fatalf("the watcher saw %v — a departure overtook a promotion that was already "+
			"in flight, so the table was told p-go left and then that they arrived. The "+
			"client re-adds on CONNECTED and only a snapshot replaces the list: a "+
			"permanent ghost.", got)
	}
}

func TestADepartureIsNotAnnouncedIfTheyHaveAlreadyComeBack(t *testing.T) {
	// #55. leave() decides "that was their last connection" and the
	// announcement is a SEPARATE step, so a reconnect landing in between makes
	// the table's last word about a present participant "DISCONNECTED" — and
	// tells the fresh connection ITSELF gone, since broadcast excludes by
	// connection pointer, not by participant.
	//
	// MEASURED before fixing, mirroring exactly what serve does on each side:
	// 1 inversion in 20,000 rounds (0.8% of the 125 that produced both frames).
	// Rare because the reference count usually saves it — a reconnect that wins
	// the lock makes leave() return last=false, so no departure is announced at
	// all. It is the case where leave wins and the announcement loses that bites.
	//
	// The fix is the mirror of announceIfPresent: re-check at send time. A
	// departure is only news if they are still gone.
	r := newPresenceRegistry()
	watcher := conn("p-watch", "Zoe", 4)
	old := conn("p-1", "Ada", 4)
	join(r, watcher)
	join(r, old)

	if last := r.leave(old); !last {
		t.Fatal("removing the only connection must report last=true")
	}
	// They come back BEFORE the departure is announced. Deterministic, because
	// the race only decides whether this ordering happens — not what it means.
	fresh := conn("p-1", "Ada", 4)
	join(r, fresh)

	built := false
	if r.announceIfAbsent("p-1", nil, func() []byte {
		built = true
		return []byte("DISCONNECTED p-1")
	}) {
		t.Fatal("announced a departure for somebody who had already reconnected — the " +
			"table's last word about a present player is that they left, and only a " +
			"snapshot can correct it, which spec §3.4 makes a MANUAL act")
	}
	if built {
		t.Fatal("the frame must not even be built for a participant who is back")
	}
	if len(watcher.out) != 0 {
		t.Fatalf("the table was sent %d frame(s) about a participant who is present",
			len(watcher.out))
	}
}

func TestARealDepartureIsStillAnnounced(t *testing.T) {
	// The control, and the one that matters: a suppression rule that suppressed
	// everything would satisfy the test above while removing presence entirely.
	r := newPresenceRegistry()
	// TWO watchers, because one cannot tell "reached the table" from "reached
	// the first connection and stopped". Review injected a walk that sends to
	// targets[0] only and the whole suite stayed green — and a fan-out that
	// dies partway through is the PERMANENT GHOST 656079f was written to fix:
	// some connections learn of the departure and the rest never do.
	first := conn("p-watch-1", "Zoe", 4)
	second := conn("p-watch-2", "Kim", 4)
	going := conn("p-1", "Ada", 4)
	join(r, first)
	join(r, second)
	join(r, going)

	if last := r.leave(going); !last {
		t.Fatal("last=true expected")
	}
	if !r.announceIfAbsent("p-1", nil, func() []byte { return []byte("DISCONNECTED p-1") }) {
		t.Fatal("a genuine departure was not announced — the table keeps a ghost")
	}
	for i, w := range []*presenceConn{first, second} {
		select {
		case got := <-w.out:
			if string(got) != "DISCONNECTED p-1" {
				t.Fatalf("watcher %d got %q", i, got)
			}
		default:
			t.Fatalf("watcher %d was never told — a fan-out that stops partway leaves a "+
				"permanent ghost on every connection it skipped", i)
		}
	}
}

func TestADepartureIsWithheldFromAnybodyTheCallerHasDenied(t *testing.T) {
	// Spec §3.2. Presence is the ONE delivery path that does not run through
	// the pump, so a revoked stranger who came in on a leaked link would go on
	// watching the guest list LEAVE until the table next appended an event.
	//
	// This is a COVERAGE REGRESSION the fix introduced and review caught:
	// departures used to share broadcast's deny filter with arrivals, covered
	// by TestBroadcastSkipsAnybodyTheCallerHasDenied. announceIfAbsent
	// duplicates that filter, and the copy had no test — `_ = deny` left the
	// whole suite green.
	r := newPresenceRegistry()
	staying := conn("p-ok", "Zoe", 4)
	revoked := conn("p-gone", "Stranger", 4)
	going := conn("p-1", "Ada", 4)
	join(r, staying)
	join(r, revoked)
	join(r, going)

	if last := r.leave(going); !last {
		t.Fatal("last=true expected")
	}
	if !r.announceIfAbsent("p-1", map[string]bool{"p-gone": true},
		func() []byte { return []byte("DISCONNECTED p-1") }) {
		t.Fatal("the departure was not announced at all")
	}

	if len(revoked.out) != 0 {
		t.Fatal("a revoked participant was told who left — presence does not run through " +
			"the pump, so nothing else withholds it from them")
	}
	// The positive control on the SAME frame: a deny filter that denied
	// everybody would satisfy the assertion above.
	if len(staying.out) != 1 {
		t.Fatalf("the remaining watcher holds %d frames, want 1 — the deny set is "+
			"withholding from everyone", len(staying.out))
	}
}

func TestSuppressionIsPerParticipantNotPerConnection(t *testing.T) {
	// Absence is per PARTICIPANT, and the check must not confuse "this
	// connection is gone" with "this person is gone". A laptop closing while
	// the phone stays connected is not a departure at all — leave() already
	// says so with last=false — but the suppression must agree, or a genuine
	// last-connection departure could be suppressed by the connection that just
	// left still being visible somewhere.
	//
	// RENAMED from TestADepartureReachesTheParticipantsOtherDevices, which
	// promised something it never looked at: it reads return values only, and
	// by the final announcement p-1 has no devices left to reach. What it
	// actually pins is the sentence above.
	r := newPresenceRegistry()
	watcher := conn("p-watch", "Zoe", 4)
	laptop := conn("p-1", "Ada", 4)
	phone := conn("p-1", "Ada", 4)
	join(r, watcher)
	join(r, laptop)
	join(r, phone)

	if last := r.leave(laptop); last {
		t.Fatal("closing one of two devices must not report the participant gone")
	}
	if r.announceIfAbsent("p-1", nil, func() []byte { return []byte("DISCONNECTED p-1") }) {
		t.Fatal("announced a departure for somebody still on another device")
	}
	if last := r.leave(phone); !last {
		t.Fatal("closing the last device must report the participant gone")
	}
	if !r.announceIfAbsent("p-1", nil, func() []byte { return []byte("DISCONNECTED p-1") }) {
		t.Fatal("the real departure was suppressed")
	}
}
