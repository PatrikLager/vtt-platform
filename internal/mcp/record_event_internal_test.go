package mcp

// Mutation-driven tests for the parts of the MCP server that reconnects and
// history de-duplication depend on. Each targets a mutant that SURVIVED a
// gremlins run, i.e. behavior the existing suite could not distinguish from
// broken.

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

func seqEnv(seq int64) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:  "evt",
		Sequence: seq,
		Payload:  &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}},
	}
}

// TestRecordEventIsIdempotentAtTheBoundary kills the `<=` at server.go:339.
//
// recordEvent's guard is `env.GetSequence() <= s.lastSeq`. Mutated to `<`, an
// event whose sequence EQUALS lastSeq is appended a second time. That is not a
// hypothetical path: it is precisely what a redial replays. `redial` reconnects
// with after=lastSeen (harness.Dial's `after` parameter), and the server's
// catch-up is inclusive of nothing above that — but any overlap, or a
// reconnect racing an in-flight event, re-delivers what was already recorded.
//
// A duplicate in history is not cosmetic: get_events_since paginates over that
// slice, so the agent would see the same event twice, with headSequence
// unchanged. The existing suite never fed the same sequence twice.
func TestRecordEventIsIdempotentAtTheBoundary(t *testing.T) {
	s := &Server{}

	s.recordEvent(seqEnv(1))
	s.recordEvent(seqEnv(2))
	before, head := s.historySnapshot()
	if len(before) != 2 || head != 2 {
		t.Fatalf("setup: got %d events head=%d, want 2 and 2", len(before), head)
	}

	// Exactly at the boundary — the redial-overlap case.
	s.recordEvent(seqEnv(2))

	after, headAfter := s.historySnapshot()
	if len(after) != 2 {
		t.Errorf("re-recording sequence 2 appended a duplicate: history has %d events, want 2 "+
			"— a reconnect that re-delivers an event the server already has would show the "+
			"agent the same event twice", len(after))
	}
	if headAfter != 2 {
		t.Errorf("head = %d, want 2", headAfter)
	}
}

// TestRecordEventIgnoresStaleSequences is the below-boundary half. Both
// directions are needed: an assertion on only one side leaves the other
// mutation alive, which is the recurring "one variant tested, its sibling
// assumed" shape in this codebase.
func TestRecordEventIgnoresStaleSequences(t *testing.T) {
	s := &Server{}
	s.recordEvent(seqEnv(5))
	s.recordEvent(seqEnv(3))

	events, head := s.historySnapshot()
	if len(events) != 1 {
		t.Errorf("a stale (lower) sequence was recorded: history has %d events, want 1", len(events))
	}
	if head != 5 {
		t.Errorf("head = %d, want 5 — a stale event must not move it backwards", head)
	}
}

// TestRecordEventAdvancesOnNewSequences pins the accept path, so the guard
// cannot be satisfied by rejecting everything.
func TestRecordEventAdvancesOnNewSequences(t *testing.T) {
	s := &Server{}
	for _, seq := range []int64{1, 2, 7} {
		s.recordEvent(seqEnv(seq))
	}
	events, head := s.historySnapshot()
	if len(events) != 3 {
		t.Errorf("history has %d events, want 3", len(events))
	}
	if head != 7 {
		t.Errorf("head = %d, want 7", head)
	}
}

// TestDescribeArgsDecodeErrorNamesAnUnknownArgument kills the `m != nil` at
// read_tools.go:182.
//
// Strict decoding rejects unknown fields with a message an LLM caller cannot
// act on ("json: unknown field ..."); describeArgsDecodeError rewrites it to
// name the argument. Mutated to `m == nil`, the rewrite is skipped and the raw
// encoding/json text is returned — the tool still errors, so nothing in the
// suite noticed, but the agent loses the one hint that tells it which argument
// to drop.
func TestDescribeArgsDecodeErrorNamesAnUnknownArgument(t *testing.T) {
	var dst struct {
		Known string `json:"known"`
	}
	dec := json.NewDecoder(strings.NewReader(`{"nope": 1}`))
	dec.DisallowUnknownFields()
	err := dec.Decode(&dst)
	if err == nil {
		t.Fatal("setup: want a strict-decode error")
	}

	got := describeArgsDecodeError(err)
	if !strings.Contains(got.Error(), `unknown argument "nope"`) {
		t.Errorf("got %q, want it to name the unknown argument — an LLM caller cannot act on "+
			"encoding/json's raw wording", got)
	}
}

// TestDescribeArgsDecodeErrorNamesATypeMismatch pins the sibling branch, whose
// mutant was killed only because this path happened to be exercised elsewhere.
// Pinning it here keeps that true if the other test moves.
func TestDescribeArgsDecodeErrorNamesATypeMismatch(t *testing.T) {
	var dst struct {
		Limit int `json:"limit"`
	}
	err := json.Unmarshal([]byte(`{"limit": "many"}`), &dst)
	if err == nil {
		t.Fatal("setup: want a type error")
	}

	got := describeArgsDecodeError(err).Error()
	if !strings.Contains(got, "limit") || !strings.Contains(got, "must be a") {
		t.Errorf("got %q, want it to name the field and the expected type", got)
	}
}

// TestDescribeArgsDecodeErrorPassesThroughUnrecognisedErrors pins the fallback:
// an error matching neither branch must be returned unchanged, not swallowed
// or rewritten into something misleading.
func TestDescribeArgsDecodeErrorPassesThroughUnrecognisedErrors(t *testing.T) {
	sentinel := errors.New("something else entirely")
	if got := describeArgsDecodeError(sentinel); !errors.Is(got, sentinel) {
		t.Errorf("got %v, want the original error unchanged", got)
	}
}

// TestPumpReturnsWhenContextCancelledAfterConnectionLoss kills the
// `ctx.Err() != nil` at server.go:260.
//
// When Events() closes, the pump clears the client and then checks whether the
// context is done: cancelled means shut down, live means redial. Mutated to
// `== nil`, those swap — a cancelled context would fall through to redial, and
// a live one would return, abandoning the wire permanently on the first
// dropped connection.
func TestPumpReturnsWhenContextCancelledAfterConnectionLoss(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	dialed := make(chan struct{}, 4)
	stubDial(t, func(context.Context, string, string, int64) (*harness.Client, error) {
		dialed <- struct{}{}
		return nil, errors.New("connection refused")
	})

	s := &Server{cfg: Config{WSURL: "ws://unused/ws", Token: "tok"}}
	cancel()

	// With the context already cancelled, redial must not dial at all.
	if s.redial(ctx) {
		t.Fatal("redial: want false with a cancelled context")
	}
	select {
	case <-dialed:
		t.Error("redial dialed despite a cancelled context — the guard's sense is inverted")
	default:
	}
}

// TestRedialBackoffGrowsExponentially kills BOTH negation mutants in the
// backoff arithmetic (server.go:298:15 and :300:16), which no assertion on
// redial's RETURN VALUE can reach — it returns false on every failed attempt
// regardless of how long it waited.
//
//	if backoff < redialMaxBackoff {   // :298 — negate: never doubles
//	    backoff *= 2
//	    if backoff > redialMaxBackoff {  // :300 — negate: jumps to max at once
//	        backoff = redialMaxBackoff
//	    }
//	}
//
// Elapsed time is the only observable. With correct growth, three failed
// attempts wait 100 + 200 + 400 = ~700ms. Negating :298 removes the doubling
// entirely (~300ms); negating :300 clamps to max immediately (~100 + 2000 +
// 2000). The window below excludes both while leaving generous headroom for a
// loaded machine.
//
// Why this matters beyond the mutants: the backoff is what stops a server
// whose table has gone away from hammering reconnects, and what keeps a brief
// blip from costing the agent a full 2s stall. Neither end was pinned.
func TestRedialBackoffGrowsExponentially(t *testing.T) {
	const attempts = 4 // 3 waits between them: 100 + 200 + 400

	attemptCh := make(chan struct{}, attempts+2)
	stubDial(t, func(context.Context, string, string, int64) (*harness.Client, error) {
		attemptCh <- struct{}{}
		return nil, errors.New("connection refused")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &Server{cfg: Config{WSURL: "ws://unused/ws", Token: "tok"}}

	done := make(chan bool, 1)
	start := time.Now()
	go func() { done <- s.redial(ctx) }()

	for i := 0; i < attempts; i++ {
		select {
		case <-attemptCh:
		// Deliberately tight. A generous deadline here would let a mutant
		// that breaks the backoff HANG the test instead of failing it, and a
		// hung test is a gremlins TIMEOUT — never evaluated, invisible to the
		// gate. Correct behaviour reaches four attempts in ~700ms.
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d dial attempts within 3s, want %d", i, attempts)
		}
	}
	elapsed := time.Since(start)
	cancel()
	<-done

	// Correct: ~700ms. Flat (no doubling): ~300ms. Immediate max: ~4100ms.
	const lower, upper = 500 * time.Millisecond, 1500 * time.Millisecond
	if elapsed < lower {
		t.Errorf("%d attempts took %v, under %v — the backoff is not growing, so a server "+
			"whose table has gone away would be hammered at a flat interval", attempts, elapsed, lower)
	}
	if elapsed > upper {
		t.Errorf("%d attempts took %v, over %v — the backoff jumped to its maximum instead of "+
			"ramping, so a brief blip costs the agent a full %v stall",
			attempts, elapsed, upper, redialMaxBackoff)
	}
}

// TestPumpRedialsWhenConnectionDropsWithLiveContext kills the negation at
// server.go:260 — `if ctx.Err() != nil { return }` after Events() closes.
//
// Inverted, the two cases swap: a LIVE context returns (the pump abandons the
// wire permanently after the first dropped connection, and the agent silently
// stops receiving events with no error anywhere), while a CANCELLED one falls
// through to redial. The live-context half is the damaging one and the one
// asserted here — the pump must try to reconnect.
func TestPumpRedialsWhenConnectionDropsWithLiveContext(t *testing.T) {
	wsURL := fakeWSServer(t)

	redialled := make(chan struct{}, 4)
	real := harnessDial
	var dials int
	stubDial(t, func(dctx context.Context, u, tok string, after int64) (*harness.Client, error) {
		dials++
		if dials == 1 {
			return real(dctx, u, tok, after) // the connection that will drop
		}
		redialled <- struct{}{}
		return nil, errors.New("refused") // enough to prove the attempt happened
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &Server{cfg: Config{WSURL: wsURL, Token: "tok"}}
	if !s.redial(ctx) {
		t.Fatal("setup: first dial should succeed")
	}
	client := s.currentClient()

	go s.pump(ctx)

	// Drop the connection out from under the pump while ctx is still live.
	client.Close()

	select {
	case <-redialled:
	// Same reasoning: fail fast so a broken mutant fails rather than hangs.
	case <-time.After(3 * time.Second):
		t.Fatal("the pump did not attempt a redial after the connection dropped with a live " +
			"context — it returned instead, which abandons the wire silently: the agent stops " +
			"receiving events and nothing reports an error")
	}
}
