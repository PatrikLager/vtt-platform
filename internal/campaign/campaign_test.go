package campaign_test

import (
	"fmt"
	"os"
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
	case *vttv1.ResourceChanged:
		e.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: p}
	case *vttv1.ConditionApplied:
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: p}
	case *vttv1.ConditionRemoved:
		e.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: p}
	case *vttv1.NarrationAdded:
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: p}
	case *vttv1.NoteUpserted:
		e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: p}
	case *vttv1.NoteDeleted:
		e.Payload = &vttv1.Envelope_NoteDeleted{NoteDeleted: p}
	case *vttv1.AdventureLoaded:
		e.Payload = &vttv1.Envelope_AdventureLoaded{AdventureLoaded: p}
	case *vttv1.DoorOpened:
		e.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: p}
	case *vttv1.DoorClosed:
		e.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: p}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: p}
	case *vttv1.ActorControlRevoked:
		e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: p}
	case *vttv1.TokenRemoved:
		e.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: p}
	case *vttv1.ActorRemoved:
		e.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: p}
	default:
		// AN UNHANDLED PAYLOAD USED TO RETURN A NIL-PAYLOAD ENVELOPE, and the
		// failure then surfaced as "unknown event variant" — which reads as a
		// missing ARM in the fold rather than a missing case in a test helper.
		// Measured 2026-09-12: the property walk gained four add/remove pairs
		// and this switch gained SIX cases for them (two were already here),
		// and every engine arm was present the whole time.
		//
		// THE ERROR DOES NOT COME FROM engine.Apply, which a first draft of
		// this comment said: a nil payload never reaches it. campaign.Append
		// returns engine.ErrUnknownVariant from its own env.Payload == nil
		// guard before folding, and AppendBatch does the same in its pre-loop.
		// The text a reader sees is identical either way, which is exactly how
		// the wrong thing gets blamed — the failure this comment is about.
		panic(fmt.Sprintf("cenv: no case for payload %T — add one; a nil-payload "+
			"envelope fails later and blames the engine", payload))
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

	// path is now the campaign DIRECTORY (2026-09-01-create-scene-leaves Task 4); the log itself lives at
	// campaign.LogPath(path) inside it.
	s, err := store.Open(campaign.LogPath(path))
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

	ch, unsubscribe, _, err := c.Subscribe(0, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

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

// writeFile writes content at path, creating no parent directories (the
// caller's tempdir already exists). Mirrors cmd/vtt/maps_test.go's helper of
// the same name and shape — that one lives in package main and cannot be
// imported here.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestOpenTakesACampaignDirectory pins the new contract (spec
// 2026-09-01-create-scene-leaves-design.md §3): Open takes a DIRECTORY and
// creates the log inside it, so the same directory can go on to hold maps/
// and art/ (Task 5+) without a second top-level path for the campaign. It
// said "maps/ and packs/" until 2026-09-02-art-is-a-flat-library Task 7
// deleted the pack; Open's own refusal message names the same two.
func TestOpenTakesACampaignDirectory(t *testing.T) {
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("Open on a directory: %v", err)
	}
	defer c.Close()
	if _, err := os.Stat(filepath.Join(dir, "log.db")); err != nil {
		t.Fatalf("want log.db created inside the campaign directory: %v", err)
	}
}

// TestOpenRefusesABareLogFile is the "no implicit fallback" half (spec §3,
// "A bare log file is refused, not adopted"): treating a lone log file as a
// campaign with no maps is the same reasoning that makes terrain mandatory
// and gives the console's selects a blank first option.
//
// The assertion pins "put it in one" rather than the plan's own suggested
// "directory" (docs/superpowers/plans/2026-09-01-create-scene-leaves.md,
// Task 4 Step 1): os.MkdirAll's OWN fallback error for this exact fixture
// (an existing regular file where a directory is wanted) is "mkdir <path>:
// not a directory" — which also contains "directory" — so that substring
// is satisfied by BOTH the deliberate refusal and by silently falling
// through to MkdirAll and letting its bare OS error surface. Worse: this
// package's OWN read-only-mount wrap (TestOpenOnAReadOnlyMountGivesAClearError)
// also contains the word "directory", so even a platform that words ENOTDIR
// differently would still satisfy a "directory"-only assertion by accident.
// With the refusal branch deleted, that assertion stays GREEN on the wrong
// error; "put it in one" goes RED, because only the deliberate message
// contains it — reproducible directly: delete the `!info.IsDir()` branch
// in Open and run this test.
func TestOpenRefusesABareLogFile(t *testing.T) {
	dir := t.TempDir()
	lone := filepath.Join(dir, "old.db")
	writeFile(t, lone, "")

	_, err := campaign.Open(lone)
	if err == nil {
		t.Fatal("want a bare log file refused rather than adopted")
	}
	if !strings.Contains(err.Error(), "put it in one") {
		t.Fatalf("error = %q, want it to say what to do", err.Error())
	}
}

// TestOpenOnAReadOnlyMountGivesAClearError is the spec §12 hazard ("Nothing
// can create a place if the campaign directory is read-only... worth a clear
// error rather than a confusing one"): installing a map means writing a file
// into the campaign directory, so an operator who cannot even open one loses
// improvisation entirely and needs to be told why in campaign terms, not in
// bare os.MkdirAll terms.
//
// The probe-then-skip pattern mirrors
// internal/identity/identity_test.go's
// TestMigratingAReadOnlyCampaignFailsRatherThanHalfApplying: running as a
// user for whom permission bits are decorative (root, some CI sandboxes)
// must skip rather than falsely pass or fail.
func TestOpenOnAReadOnlyMountGivesAClearError(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "campaign")

	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	if f, err := os.CreateTemp(parent, "probe-*"); err == nil {
		f.Close()
		_ = os.Remove(f.Name())
		t.Skip("running with rights that make a read-only directory writable")
	}

	_, err := campaign.Open(dir)
	if err == nil {
		t.Fatal("want campaign.Open on a read-only mount to fail rather than silently succeed")
	}
	if !strings.Contains(err.Error(), "writable") {
		t.Fatalf("error = %q, want it to say the campaign directory must be writable", err.Error())
	}
}
