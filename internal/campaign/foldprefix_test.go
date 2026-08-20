package campaign_test

import (
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

// TestFoldPrefixHonoursARetractionInsideThePrefix is the half a streaming
// re-application of events cannot do, and the reason FoldPrefix exists at all
// rather than "apply each envelope as it arrives".
//
// A retraction is retroactive: the marker lands at head and removes a range
// that was folded long before it. Only a fold that sees the WHOLE prefix
// before it applies anything can honour that, which is exactly the two-pass
// shape foldEvents already has.
func TestFoldPrefixHonoursARetractionInsideThePrefix(t *testing.T) {
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
	moved := must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 7},
		To:   &vttv1.GridPosition{X: 5, Y: 8},
	}))
	if _, err := c.Undo(moved, moved, "misclick", nextID(), "dm", "p-dm"); err != nil {
		t.Fatalf("undo the move: %v", err)
	}

	log := drainLog(t, c, 6)
	whole, err := campaign.FoldPrefix(log)
	if err != nil {
		t.Fatalf("FoldPrefix over a log with a retraction: %v", err)
	}
	if tok := whole.Tokens["t1"]; tok.X != 3 || tok.Y != 7 {
		t.Fatalf("a retracted move must not be folded: t1 at (%d,%d), want (3,7)", tok.X, tok.Y)
	}
}
