package campaign_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

var idCounter int

// nextID returns a fresh, unique event id. The store enforces event_id
// uniqueness, so every envelope built for these tests needs one.
func nextID() string {
	idCounter++
	return fmt.Sprintf("evt-%d", idCounter)
}

// cenv builds an Envelope with EventId set (campaign.Append requires it)
// wrapping payload in the correct oneof variant. Mirrors Task 4's env()
// helper style in internal/engine/apply_test.go.
func cenv(id string, payload any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: id, SessionId: "sess-1", ActorRole: "dm"}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SessionEnded:
		e.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: p}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.AttackRolled:
		e.Payload = &vttv1.Envelope_AttackRolled{AttackRolled: p}
	case *vttv1.EventsRetracted:
		e.Payload = &vttv1.Envelope_EventsRetracted{EventsRetracted: p}
	}
	return e
}

func openTemp(t *testing.T) *campaign.Campaign {
	t.Helper()
	c, err := campaign.Open(filepath.Join(t.TempDir(), "campaign.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// must appends env and fails the test on error.
func must(t *testing.T, c *campaign.Campaign, env *vttv1.Envelope) int64 {
	t.Helper()
	seq, err := c.Append(env)
	if err != nil {
		t.Fatalf("append %s: %v", env.EventId, err)
	}
	return seq
}

func TestOpenAppendLifecycleStateCorrect(t *testing.T) {
	c := openTemp(t)

	seq1 := must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	seq2 := must(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	}))
	seq3 := must(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
	}))
	seq4 := must(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 3, Y: 7},
	}))
	seq5 := must(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 3, Y: 7},
		To:   &vttv1.GridPosition{X: 5, Y: 8},
	}))

	if seq1 != 1 || seq2 != 2 || seq3 != 3 || seq4 != 4 || seq5 != 5 {
		t.Fatalf("want sequential sequences 1..5, got %d %d %d %d %d", seq1, seq2, seq3, seq4, seq5)
	}

	st := c.State()

	tok, ok := st.Tokens["t1"]
	if !ok {
		t.Fatal("want token t1 present")
	}
	if tok.X != 5 || tok.Y != 8 {
		t.Fatalf("token position: got (%d,%d), want (5,8)", tok.X, tok.Y)
	}
	if _, ok := st.Scenes["scn"]; !ok {
		t.Fatal("want scene scn present")
	}
	if _, ok := st.Actors["a1"]; !ok {
		t.Fatal("want actor a1 present")
	}
	if len(st.Sessions) != 1 || st.Sessions[0].EndSeq != 0 {
		t.Fatalf("want 1 open session, got %+v", st.Sessions)
	}
}

// TestAppendValidationFailurePersistsNothing verifies that a rejected append
// (TokenMoved for an unknown token) writes nothing to the log: close the
// campaign, reopen the raw store on the same file, and confirm ReadAfter(0)
// still returns exactly the events appended before the failed call.
func TestAppendValidationFailurePersistsNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	}))

	_, err = c.Append(cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "missing-token", SceneId: "scn",
		From: &vttv1.GridPosition{X: 0, Y: 0},
		To:   &vttv1.GridPosition{X: 1, Y: 1},
	}))
	if err == nil {
		t.Fatal("want error for TokenMoved on unknown token")
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	events, err := s.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("ReadAfter(0) after rejected append: got %d events, want 2 (failed append must persist nothing)", len(events))
	}
}

func TestAppendRejectsEventsRetractedEnvelope(t *testing.T) {
	c := openTemp(t)

	_, err := c.Append(cenv(nextID(), &vttv1.EventsRetracted{
		FromSequence: 1, ToSequence: 1, Reason: "x",
	}))
	if err == nil {
		t.Fatal("want error appending an EventsRetracted envelope directly")
	}
	if !strings.Contains(err.Error(), "Undo") {
		t.Fatalf("want error message to mention Undo, got %q", err.Error())
	}
}

func TestCloseReopenStateDeepEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	must(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "n"}))
	must(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	}))
	must(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
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

	before := c.State()
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	c2, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c2.Close() })

	after := c2.State()
	if !statesEqual(before, after) {
		t.Fatalf("state mismatch after close/reopen\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func TestSubscriberSeesAppendedEvents(t *testing.T) {
	c := openTemp(t)

	ch, cancel, err := c.Subscribe(0, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	env := cenv(nextID(), &vttv1.SessionStarted{Name: "n"})
	if _, err := c.Append(env); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		if got.EventId != env.EventId {
			t.Fatalf("subscriber event id: got %s, want %s", got.EventId, env.EventId)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscriber to see appended event")
	}
}
