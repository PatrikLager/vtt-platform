package harness

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// collectBatchRun decides whether a participant observed a whole batch, and it
// ends the run on the SAME signal for two opposite situations: recvWithin
// returning !ok because the quiet window elapsed (the batch really is over)
// and because the Events() channel was torn down under it (the batch is
// truncated and the rest is unobservable).
//
// Treating the second as the first is not a reporting nicety. It returns
// batchOutcome with NO err — a PASS — for an observation that stopped early,
// so a scenario asserting "these five events arrived" can be satisfied by a
// participant that saw two and was then disconnected for reading too slowly.
// The harness's own verdict becomes conditional on load.
//
// This is the failure Conn.CloseErr was added to make answerable, and
// engine.go's interface doc already recorded that most Events() consumers here
// had not adopted it. These tests are that adoption for the batch path.

func envSeq(seq int64) *vttv1.Envelope { return &vttv1.Envelope{Sequence: seq} }

// collectOne runs collectBatchRun against a scripted conn with the history
// plumbing the real caller supplies.
func collectOne(conn Conn, firstSeq int64, firstTimeout, quiet time.Duration) (batchOutcome, map[string][]*vttv1.Envelope) {
	history := map[string][]*vttv1.Envelope{}
	var mu sync.Mutex
	return collectBatchRun("observer", conn, history, &mu, firstSeq, firstTimeout, quiet), history
}

func TestCollectBatchRunFailsWhenTheStreamDiesMidBatch(t *testing.T) {
	// Two events arrive, then the client tears the connection down for
	// overflow. Under the old signal this returned a clean outcome and the
	// batch read as complete at two events.
	conn := newStubConn()
	conn.send(envSeq(5))
	conn.send(envSeq(6))
	conn.err = ErrEventsOverflow
	conn.finish()

	out, history := collectOne(conn, 5, time.Second, 50*time.Millisecond)

	if out.err == "" {
		t.Fatal("want a failure: the stream died mid-batch, so 'complete' is not knowable")
	}
	if !strings.Contains(out.err, "overflow") {
		t.Fatalf("want the overflow named so the reader knows WHO was slow, got %q", out.err)
	}
	// What was observed is still ground truth for later steps — the failure is
	// the silent part, not the partial history.
	if len(history["observer"]) != 2 {
		t.Fatalf("want both observed events still recorded, got %d", len(history["observer"]))
	}
}

func TestCollectBatchRunStillPassesWhenTheQuietWindowSimplyElapses(t *testing.T) {
	// The other side, and the one that must not regress: an OPEN, silent
	// stream after a contiguous run is exactly how a finished batch looks.
	// A guard that failed here would fail every passing scenario.
	conn := newStubConn()
	conn.send(envSeq(5))
	conn.send(envSeq(6))
	// deliberately NOT closed

	out, _ := collectOne(conn, 5, time.Second, 50*time.Millisecond)

	if out.err != "" {
		t.Fatalf("a quiet window on a live stream means the batch is over, got %q", out.err)
	}
	if len(out.envs) != 2 {
		t.Fatalf("want both events in the run, got %d", len(out.envs))
	}
}

func TestCollectBatchRunNamesTheCauseWhenNothingArrivesAtAll(t *testing.T) {
	// "no event observed" is true but useless when the reason is that the
	// connection was already gone: it sends the reader looking for a missing
	// broadcast rather than a torn-down subscriber.
	conn := newStubConn()
	conn.err = ErrEventsOverflow
	conn.finish()

	out, _ := collectOne(conn, 5, time.Second, 50*time.Millisecond)

	if out.err == "" {
		t.Fatal("want a failure when no event arrives")
	}
	if !strings.Contains(out.err, "overflow") {
		t.Fatalf("want the cause named, got %q", out.err)
	}
}

func TestCollectBatchRunSaysNoEventObservedWhenTheStreamIsMerelySilent(t *testing.T) {
	// And a live-but-silent stream keeps the plain message: nothing was torn
	// down, so naming a teardown would be the mirror-image misdiagnosis.
	conn := newStubConn() // open, nothing sent

	out, _ := collectOne(conn, 5, 50*time.Millisecond, 50*time.Millisecond)

	if out.err == "" {
		t.Fatal("want a failure when no event arrives")
	}
	if strings.Contains(out.err, "overflow") || strings.Contains(out.err, "closed") {
		t.Fatalf("nothing closed; want the plain absence reported, got %q", out.err)
	}
}

// recvWithin is where the distinction has to be made, because it is the only
// place that can see the channel close.
func TestRecvWithinSeparatesAClosedStreamFromASilentOne(t *testing.T) {
	closed := newStubConn()
	closed.err = ErrEventsOverflow
	closed.finish()

	if _, ok, why := recvWithin(closed, 50*time.Millisecond); ok || why == nil {
		t.Fatalf("closed stream: want ok=false with a reason, got ok=%v why=%v", ok, why)
	} else if !errors.Is(why, ErrEventsOverflow) {
		t.Fatalf("want the overflow preserved for errors.Is, got %v", why)
	}

	silent := newStubConn() // open, nothing sent
	if _, ok, why := recvWithin(silent, 50*time.Millisecond); ok || why != nil {
		t.Fatalf("silent stream: want ok=false with NO reason (the window simply elapsed), got ok=%v why=%v", ok, why)
	}

	// A close with NO recorded cause must still report the stream ended.
	// Production *Client cannot reach this — readLoop calls teardown(err)
	// with a concrete error before the deferred close of c.events, and
	// teardown is first-write-wins — but Conn is an exported interface that
	// promises nothing of the sort, and both in-repo fakes default CloseErr
	// to nil. Left unpinned, `return errStreamEnded` could be mutated to
	// `return nil` and every caller would silently fall back to treating a
	// teardown as an ordinary quiet window: the exact false pass this change
	// exists to remove, restored fail-open.
	reasonless := newStubConn()
	reasonless.finish()
	if _, ok, why := recvWithin(reasonless, 50*time.Millisecond); ok || why == nil {
		t.Fatalf("closed with no cause: want ok=false with a reason, got ok=%v why=%v", ok, why)
	} else if !errors.Is(why, errStreamEnded) {
		t.Fatalf("want errStreamEnded even with no cause to wrap, got %v", why)
	}
}

// The denial assertion is the one that matters most, because its evidence is
// SILENCE. A denied step claims "no broadcast reached ANY participant", proved
// by drainAllForSilence hearing nothing within denialAbsenceWindow.
//
// denialAbsenceWindow's own doc comment justifies that as "a connection that
// truly never broadcasts stays silent no matter how long the window is" — true
// for a LIVE connection, and false for a dead one. A participant whose stream
// was torn down is silent for a reason that has nothing to do with the denial,
// so the proof of the negative is void and the step passes vacuously.
func TestDrainAllForSilenceReportsAnEndedStreamRatherThanCountingItSilent(t *testing.T) {
	live := newStubConn()
	dead := newStubConn()
	dead.err = ErrEventsOverflow
	dead.finish()

	history := map[string][]*vttv1.Envelope{}
	leaked, ended := drainAllForSilence(
		map[string]Conn{"live": live, "dead": dead}, history, 50*time.Millisecond)

	if len(leaked) != 0 {
		t.Fatalf("nothing was broadcast; want no leaks, got %v", leaked)
	}
	if len(ended) != 1 || ended[0] != "dead" {
		t.Fatalf("want the dead participant named as unprovable, got %v", ended)
	}
}

func TestDrainAllForSilenceStillProvesSilenceOnLiveConnections(t *testing.T) {
	// The guard against the mirror-image bug: if every live-but-quiet stream
	// were reported as "ended", no denial assertion could ever pass.
	a, b := newStubConn(), newStubConn()
	history := map[string][]*vttv1.Envelope{}
	leaked, ended := drainAllForSilence(
		map[string]Conn{"a": a, "b": b}, history, 50*time.Millisecond)

	if len(leaked) != 0 || len(ended) != 0 {
		t.Fatalf("two live silent streams prove the negative; got leaked=%v ended=%v", leaked, ended)
	}
}

func TestDrainAllForSilenceStillCatchesARealLeak(t *testing.T) {
	quiet := newStubConn()
	noisy := newStubConn()
	noisy.send(envSeq(9))

	history := map[string][]*vttv1.Envelope{}
	leaked, ended := drainAllForSilence(
		map[string]Conn{"quiet": quiet, "noisy": noisy}, history, 50*time.Millisecond)

	if len(leaked) != 1 || leaked[0] != "noisy" {
		t.Fatalf("want the broadcast caught, got leaked=%v", leaked)
	}
	if len(ended) != 0 {
		t.Fatalf("nothing ended, got %v", ended)
	}
}

// observeOnAll answers "who did NOT see this event". A participant whose
// stream died did not see it either, but for a different reason and with a
// different fix, and reporting both as "not observed matching" sends the
// reader hunting for a broadcast bug.
func TestObserveOnAllSeparatesADeadStreamFromAMissedEvent(t *testing.T) {
	saw := newStubConn()
	saw.send(envSeq(7))
	missed := newStubConn() // live, never receives
	dead := newStubConn()
	dead.err = ErrEventsOverflow
	dead.finish()

	history := map[string][]*vttv1.Envelope{}
	// No projected seats: this pins the UNFILTERED contract, which is the one
	// that answers "who did not see this event" at all. What a projected seat
	// does here has its own test below.
	missing, ended := observeOnAll(
		map[string]Conn{"saw": saw, "missed": missed, "dead": dead}, nil, history, 7,
		50*time.Millisecond, 20*time.Millisecond)

	if len(missing) != 1 || missing[0] != "missed" {
		t.Fatalf("want only the live-but-silent participant reported missing, got %v", missing)
	}
	if len(ended) != 1 || ended[0] != "dead" {
		t.Fatalf("want the dead stream named separately, got %v", ended)
	}
}

// drainPreExisting proves a campaign is FRESH by hearing nothing during the
// window. Same shape as the denial assertion, same hole: a connection that was
// already torn down is silent, returns 0, and the run proceeds on the
// assumption that absolute sequence numbers start at 1 — the very assumption
// errFreshCampaignRequired exists to protect. The later failure then lands
// somewhere unrelated.
func TestDrainPreExistingReportsADeadStreamRatherThanCallingItFresh(t *testing.T) {
	dead := newStubConn()
	dead.err = ErrEventsOverflow
	dead.finish()

	n, err := drainPreExisting(dead, 50*time.Millisecond)
	if n != 0 {
		t.Fatalf("nothing arrived, want n=0, got %d", n)
	}
	if err == nil {
		t.Fatal("want the dead stream reported: its silence does not prove the campaign is fresh")
	}
	if !errors.Is(err, ErrEventsOverflow) {
		t.Fatalf("want the cause preserved, got %v", err)
	}
}

func TestDrainPreExistingStillProvesFreshnessOnALiveStream(t *testing.T) {
	live := newStubConn() // open, silent: a genuinely fresh campaign
	n, err := drainPreExisting(live, 50*time.Millisecond)
	if n != 0 || err != nil {
		t.Fatalf("want (0, nil) for a live silent stream, got (%d, %v)", n, err)
	}
}

func TestDrainPreExistingStillCountsRealBacklog(t *testing.T) {
	conn := newStubConn()
	conn.send(envSeq(1))
	conn.send(envSeq(2))
	n, err := drainPreExisting(conn, 50*time.Millisecond)
	if n != 2 || err != nil {
		t.Fatalf("want (2, nil) for a replayed backlog, got (%d, %v)", n, err)
	}
}

// The same hole as drainAllForSilence, in RunSoak's own evidence base.
//
// soakHistories.start drains each participant with `for env := range
// c.Events()`, and that loop simply ENDS when the stream is torn down. Its
// history then stops growing, so grewSince — which proves "no broadcast
// reached anyone" by comparing lengths across denialAbsenceWindow — sees no
// growth and reports no leak. The trailing-denial assertion passes on the
// silence of a participant that could not have heard anything.
//
// engine_test.go's fakeConn.Close doc already names these drain goroutines as
// the consumer class worth getting right; this is the assertion they feed.
func TestSoakHistoriesReportAStreamThatEndedRatherThanCallingItQuiet(t *testing.T) {
	live := newStubConn()
	dead := newStubConn()
	dead.err = ErrEventsOverflow

	h := newSoakHistories()
	h.start("live", live)
	h.start("dead", dead)

	// Explicit: lengths() lists only participants that already have history,
	// and these fixtures start empty.
	before := map[string]int{"live": 0, "dead": 0}
	dead.finish() // torn down mid-run

	grew, ended := h.grewSince(before, 50*time.Millisecond)

	if len(grew) != 0 {
		t.Fatalf("nothing was broadcast; want no growth, got %v", grew)
	}
	if len(ended) != 1 || ended[0] != "dead" {
		t.Fatalf("want the torn-down participant named, got %v", ended)
	}
}

func TestSoakHistoriesStillProveSilenceWhenEveryStreamIsLive(t *testing.T) {
	// The guard: if a live quiet stream were reported as ended, no trailing
	// denial in any soak could ever pass.
	a, b := newStubConn(), newStubConn()
	h := newSoakHistories()
	h.start("a", a)
	h.start("b", b)

	grew, ended := h.grewSince(map[string]int{"a": 0, "b": 0}, 50*time.Millisecond)
	if len(grew) != 0 || len(ended) != 0 {
		t.Fatalf("two live silent streams prove the negative; got grew=%v ended=%v", grew, ended)
	}
}

func TestSoakHistoriesStillCatchARealLeak(t *testing.T) {
	quiet, noisy := newStubConn(), newStubConn()
	h := newSoakHistories()
	h.start("quiet", quiet)
	h.start("noisy", noisy)

	before := map[string]int{"quiet": 0, "noisy": 0}
	noisy.send(envSeq(42))

	grew, ended := h.grewSince(before, 50*time.Millisecond)
	if len(grew) != 1 || grew[0] != "noisy" {
		t.Fatalf("want the leak caught, got grew=%v", grew)
	}
	if len(ended) != 0 {
		t.Fatalf("nothing ended, got %v", ended)
	}
}

// TestObserveOnAllNeverRequiresAProjectedSeatToSeeAnything is the visibility
// arc's amendment to the contract above, and both halves matter.
//
// A projected seat receives what its participant may see, so silence is a
// legitimate answer for an event about a room they are not in — requiring one
// event each would fail every honest projection (visibility spec §4.2). But
// what it DOES receive still has to reach history, because the denial checks
// count what arrived since (leakedSince) and the reconnect step compares
// catch-up against what was seen live; a seat drained by nobody makes one
// assertion vacuous and the other wrong.
func TestObserveOnAllNeverRequiresAProjectedSeatToSeeAnything(t *testing.T) {
	silent := newStubConn() // a projected seat that may see nothing
	watching := newStubConn()
	watching.send(envSeq(7))
	watching.send(envSeq(7)) // one event, two projected envelopes
	dm := newStubConn()
	dm.send(envSeq(7))

	history := map[string][]*vttv1.Envelope{}
	missing, ended := observeOnAll(
		map[string]Conn{"silent": silent, "watching": watching, "dm": dm},
		map[string]bool{"silent": true, "watching": true},
		history, 7, 50*time.Millisecond, 30*time.Millisecond)

	if len(missing) != 0 {
		t.Fatalf("a projected seat must never be reported missing, got %v", missing)
	}
	if len(ended) != 0 {
		t.Fatalf("nothing ended, got %v", ended)
	}
	if len(history["silent"]) != 0 {
		t.Fatalf("the silent seat recorded %d envelopes, want 0", len(history["silent"]))
	}
	if len(history["watching"]) != 2 {
		t.Fatalf("a projected seat given two envelopes for one event recorded %d, want 2 — "+
			"the denial and reconnect assertions read this", len(history["watching"]))
	}
	if len(history["dm"]) != 1 {
		t.Fatalf("the unprojected seat recorded %d envelopes, want 1", len(history["dm"]))
	}
}

// TestObserveOnAllStillNamesAProjectedSeatWhoseStreamDied keeps the half that
// silence must not swallow: a seat entitled to receive nothing is not the same
// as a seat that can no longer receive at all.
func TestObserveOnAllStillNamesAProjectedSeatWhoseStreamDied(t *testing.T) {
	dead := newStubConn()
	dead.err = ErrEventsOverflow
	dead.finish()
	dm := newStubConn()
	dm.send(envSeq(7))

	history := map[string][]*vttv1.Envelope{}
	missing, ended := observeOnAll(
		map[string]Conn{"dead": dead, "dm": dm},
		map[string]bool{"dead": true},
		history, 7, 50*time.Millisecond, 30*time.Millisecond)

	if len(missing) != 0 {
		t.Fatalf("a projected seat must never be reported missing, got %v", missing)
	}
	if len(ended) != 1 || ended[0] != "dead" {
		t.Fatalf("want the dead projected stream named, got %v", ended)
	}
}
