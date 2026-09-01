package campaign_test

import (
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// drainLog subscribes from the beginning and returns the whole log, sequences
// and all. Used rather than a second store handle because it is the same
// delivery the gateway's pump reads, which is what FoldPrefix's caller folds.
func drainLog(t *testing.T, c *campaign.Campaign, want int) []*vttv1.Envelope {
	t.Helper()
	events, unsubscribe, _, err := c.Subscribe(0, 256)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	var out []*vttv1.Envelope
	for len(out) < want {
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatalf("subscription closed after %d of %d events", len(out), want)
			}
			out = append(out, e)
		case <-time.After(3 * time.Second):
			t.Fatalf("only %d of %d events arrived", len(out), want)
		}
	}
	return out
}

// seqLog stamps a hand-built slice with contiguous sequences 1..n, the way
// the store would have. The fold reads env.Sequence for its error messages
// and several arms write it into state (SessionStarted's StartSeq,
// SessionEnded's EndSeq), so an unstamped slice is not the log this test
// claims to be folding.
func seqLog(envs ...*vttv1.Envelope) []*vttv1.Envelope {
	for i, e := range envs {
		e.Sequence = int64(i + 1)
	}
	return envs
}

// TestFoldPrefixIsTheStateThatPrefixProduced pins the contract the gateway's
// per-seat projection rests on: for any PREFIX of the log, FoldPrefix returns
// the state that prefix produced — not the state at head.
//
// The visibility projection is a function of (log-so-far, viewer), and
// "so-far" is the whole point: a reconnecting seat is replayed from the
// beginning, and every event has to be judged against the world as it stood
// when that event happened. Answering with head state instead would forward a
// move judged against positions from the future.
func TestFoldPrefixIsTheStateThatPrefixProduced(t *testing.T) {
	c := openTemp(t)
	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	}))
	must(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero"},
	}))
	must(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 3, Y: 7},
	}))
	must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 7},
		To:   &vttv1.GridPosition{X: 5, Y: 8},
	}))

	log := drainLog(t, c, 5)

	// The prefix that ends BEFORE the move sees the token where it was placed.
	before, err := campaign.FoldPrefix(log[:4])
	if err != nil {
		t.Fatalf("FoldPrefix over 4 events: %v", err)
	}
	if tok := before.Tokens["t1"]; tok.X != 3 || tok.Y != 7 {
		t.Fatalf("prefix of 4 must place t1 at (3,7), got (%d,%d)", tok.X, tok.Y)
	}

	// And the whole log agrees with the live projection, which is the claim
	// that makes this the SAME fold rather than a second opinion.
	whole, err := campaign.FoldPrefix(log)
	if err != nil {
		t.Fatalf("FoldPrefix over the whole log: %v", err)
	}
	live := c.State()
	if tok := whole.Tokens["t1"]; tok.X != live.Tokens["t1"].X || tok.Y != live.Tokens["t1"].Y {
		t.Fatalf("whole-log fold must equal the live projection: got (%d,%d), live (%d,%d)",
			tok.X, tok.Y, live.Tokens["t1"].X, live.Tokens["t1"].Y)
	}
	if tok := whole.Tokens["t1"]; tok.X != 5 || tok.Y != 8 {
		t.Fatalf("whole-log fold must place t1 at (5,8), got (%d,%d)", tok.X, tok.Y)
	}
}

// TestFoldPrefixRefusesALogThatDoesNotReplay pins the fold's OTHER answer, and
// the one a seat's safety rests on: an envelope that engine.Apply rejects for
// any reason other than an unknown variant aborts the whole fold with an error
// naming the sequence, and returns NO state.
//
// NO STATE IS THE LOAD-BEARING HALF. internal/gateway/seat.go's receive FAILS
// CLOSED on this error — a fold that does not replay means the connection
// cannot be told what it may see, so nothing is forwarded. A partial state
// returned alongside the error would let a caller that checks only the state
// judge a viewer against a world built from half a log.
//
// AFTER-THE-FACT, so each of its three load-bearing assertions was proven by a
// separate injection into foldEvents' corrupt-log return (ADR-009 §3), run and
// reverted 2026-08-31:
//
//   - replaced by `continue` — fails on "want an error folding a log whose
//     second event moves a token nothing placed";
//   - `return st, ...` instead of `return nil, ...` — fails on "want no state
//     alongside the error", printing the half-built state;
//   - the sequence dropped from the message — fails on "the error must name the
//     sequence that failed", printing `campaign: corrupt log: engine: moved
//     unknown token "ghost"`.
//
// It replaces the coverage TestUndoRejectsRetractionThatBreaksReplay gave this
// branch: that test reached it through Undo's dry run, and went with Undo.
func TestFoldPrefixRefusesALogThatDoesNotReplay(t *testing.T) {
	log := seqLog(
		cenv(nextID(), &vttv1.SessionStarted{Name: "n"}),
		cenv(nextID(), &vttv1.TokenMoved{
			TokenId: "ghost", SceneId: "scn",
			From: &vttv1.GridPosition{X: 1, Y: 1},
			To:   &vttv1.GridPosition{X: 2, Y: 2},
		}),
	)

	st, err := campaign.FoldPrefix(log)
	if err == nil {
		t.Fatal("want an error folding a log whose second event moves a token nothing placed")
	}
	if st != nil {
		t.Fatalf("want no state alongside the error, got %+v", st)
	}
	if !strings.Contains(err.Error(), "seq 2") {
		t.Errorf("the error must name the sequence that failed, got %q", err.Error())
	}
}
