package harness_test

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// foldEnv builds an Envelope the same way engine's own apply_test.go does
// (env helper), with a distinct EventId per call.
func foldEnv(seq int64, id string, payload any) *vttv1.Envelope {
	e := &vttv1.Envelope{EventId: id, Sequence: seq, SessionId: "s1"}
	switch p := payload.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: p}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: p}
	case *vttv1.ActorAdded:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: p}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: p}
	case *vttv1.TokenMoved:
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: p}
	case *vttv1.EventsRetracted:
		e.Payload = &vttv1.Envelope_EventsRetracted{EventsRetracted: p}
	case nil:
		// Deliberately left with a nil Payload — engine.Apply's type switch
		// falls to its default case for this (engine.ErrUnknownVariant),
		// standing in for a future event variant this client doesn't know
		// about yet (forward-compatibility skip, same as the server's fold).
	}
	return e
}

// TestFoldParityAgainstIndependentEngineApplyReplay is the ADR-009
// after-the-fact parity test: it feeds Fold the same envelope sequence used
// to independently build an "expected" state by hand-driving engine.Apply
// directly — the event-core's own documented semantics, not Fold's
// implementation — and requires Fold's derived state to match it exactly.
// The sequence is a SessionStarted/SceneCreated/ActorAdded/TokenPlaced/
// TokenMoved chain, EVERY one of which Fold must apply, in order (single
// pass, 2026-08-31-retraction-leaves task-4-brief.md — there is no marker
// here and no code path that skips by sequence), plus a trailing envelope
// with a nil Payload standing in for a future/unknown event variant,
// proving Fold skips ErrUnknownVariant rather than failing the whole fold.
func TestFoldParityAgainstIndependentEngineApplyReplay(t *testing.T) {
	events := []*vttv1.Envelope{
		foldEnv(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}),
		foldEnv(2, "ev-scene", &vttv1.SceneCreated{SceneId: "scn-1", Name: "Hall", GridWidth: 10, GridHeight: 10}),
		foldEnv(3, "ev-actor", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-1", Name: "Ursus"}}),
		foldEnv(4, "ev-place", &vttv1.TokenPlaced{
			TokenId: "tok-1", SceneId: "scn-1", ActorId: "act-1", Position: &vttv1.GridPosition{X: 2, Y: 2},
		}),
		foldEnv(5, "ev-move", &vttv1.TokenMoved{
			TokenId: "tok-1", SceneId: "scn-1", From: &vttv1.GridPosition{X: 2, Y: 2}, To: &vttv1.GridPosition{X: 9, Y: 9},
		}),
		foldEnv(6, "ev-unknown", nil),
	}

	got, err := harness.Fold(events)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}

	// Independent expectation: hand-drive engine.Apply over every event Fold
	// must apply (everything except the trailing unknown-variant envelope).
	want := engine.NewState()
	for _, i := range []int{0, 1, 2, 3, 4} { // ev-session, ev-scene, ev-actor, ev-place, ev-move
		if err := engine.Apply(want, events[i]); err != nil {
			t.Fatalf("independent replay: engine.Apply(events[%d]): %v", i, err)
		}
	}

	if len(got.Sessions) != len(want.Sessions) || len(got.Sessions) != 1 {
		t.Fatalf("Sessions = %+v, want exactly 1 matching %+v", got.Sessions, want.Sessions)
	}
	if got.Sessions[0].EndSeq != 0 {
		t.Fatalf("Sessions[0].EndSeq = %d, want 0 (open — SessionEnded never happened)", got.Sessions[0].EndSeq)
	}
	if _, ok := got.Actors["act-1"]; !ok {
		t.Fatal(`Actors["act-1"] missing, want present`)
	}
	tok, ok := got.Tokens["tok-1"]
	if !ok {
		t.Fatal(`Tokens["tok-1"] missing, want present`)
	}
	// The load-bearing assertion: the token is at its MOVED-TO position
	// (9,9) — Fold applied the TokenMoved, like every other event.
	if tok.X != 9 || tok.Y != 9 {
		t.Fatalf("Tokens[\"tok-1\"] = (%d,%d), want (9,9)", tok.X, tok.Y)
	}
	if tok.X != want.Tokens["tok-1"].X || tok.Y != want.Tokens["tok-1"].Y {
		t.Fatalf("Fold's token position (%d,%d) diverges from the independent engine.Apply replay's (%d,%d)",
			tok.X, tok.Y, want.Tokens["tok-1"].X, want.Tokens["tok-1"].Y)
	}
	if len(got.Tokens) != len(want.Tokens) {
		t.Fatalf("len(Tokens) = %d, want %d (matching the independent replay)", len(got.Tokens), len(want.Tokens))
	}
	if len(got.Scenes) != len(want.Scenes) || len(got.Actors) != len(want.Actors) {
		t.Fatalf("Scenes/Actors counts = (%d,%d), want (%d,%d)", len(got.Scenes), len(got.Actors), len(want.Scenes), len(want.Actors))
	}
}

// TestFoldAppliesEveryEnvelopeEvenAcrossAnEventsRetractedMarker is the
// removal RED for 2026-08-31-retraction-leaves task-4-brief.md (ADR-009's
// inverted form for a deletion: the test that proves the thing is gone must
// fail while it is still present). Fold's two passes exist ONLY because
// retraction is retroactive, so the honest behavioral RED is a log that
// CONTAINS an EventsRetracted marker: with the two-pass Fold this predates,
// the marker's covered range (the TokenMoved at sequence 5) is skipped and
// the token stays at its PLACED position (2,2); once Fold collapses to a
// single pass, nothing skips by sequence anymore, so the same TokenMoved
// applies like any other event and the token ends at the MOVED-TO position
// (9,9). engine.Apply already treats an EventsRetracted payload itself as a
// no-op ("handled by campaign rebuild, not in-line" — apply.go), so the
// marker envelope needs no special casing either way; asserting where the
// TOKEN ends up is what makes this a behavioral RED rather than a
// name-only one.
//
// TRANSITIONAL BY DESIGN: this test constructs an EventsRetracted message,
// which Task 7 of 2026-08-31-retraction-leaves deletes from the contract.
// It cannot survive that deletion, and it is not meant to — once the
// message cannot exist, "Fold applies everything" stops being a behavior to
// prove against a marker and becomes structural (there is nothing left that
// could skip). This test dies at Task 7; that is correct, not a debt.
func TestFoldAppliesEveryEnvelopeEvenAcrossAnEventsRetractedMarker(t *testing.T) {
	events := []*vttv1.Envelope{
		foldEnv(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}),
		foldEnv(2, "ev-scene", &vttv1.SceneCreated{SceneId: "scn-1", Name: "Hall", GridWidth: 10, GridHeight: 10}),
		foldEnv(3, "ev-actor", &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "act-1", Name: "Ursus"}}),
		foldEnv(4, "ev-place", &vttv1.TokenPlaced{
			TokenId: "tok-1", SceneId: "scn-1", ActorId: "act-1", Position: &vttv1.GridPosition{X: 2, Y: 2},
		}),
		foldEnv(5, "ev-move", &vttv1.TokenMoved{
			TokenId: "tok-1", SceneId: "scn-1", From: &vttv1.GridPosition{X: 2, Y: 2}, To: &vttv1.GridPosition{X: 9, Y: 9},
		}),
		foldEnv(6, "ev-retract", &vttv1.EventsRetracted{FromSequence: 5, ToSequence: 5, Reason: "undo the move"}),
	}
	got, err := harness.Fold(events)
	if err != nil {
		t.Fatalf("Fold: %v", err)
	}
	tok, ok := got.Tokens["tok-1"]
	if !ok {
		t.Fatal(`Tokens["tok-1"] missing, want present`)
	}
	if tok.X != 9 || tok.Y != 9 {
		t.Fatalf(`Tokens["tok-1"] = (%d,%d), want (9,9) — Fold must apply every envelope in order, `+
			`including one an EventsRetracted marker used to cover; no code path skips by sequence anymore`,
			tok.X, tok.Y)
	}
}

// TestFoldRejectsGenuinelyCorruptSequence proves Fold does not silently
// swallow a REAL apply error (as opposed to ErrUnknownVariant) — a
// TokenMoved for a token that was never placed.
func TestFoldRejectsGenuinelyCorruptSequence(t *testing.T) {
	events := []*vttv1.Envelope{
		foldEnv(1, "ev-session", &vttv1.SessionStarted{Name: "s1"}),
		foldEnv(2, "ev-move", &vttv1.TokenMoved{TokenId: "tok-ghost", To: &vttv1.GridPosition{X: 1, Y: 1}}),
	}
	if _, err := harness.Fold(events); err == nil {
		t.Fatal("Fold: want error for a TokenMoved on a token that was never placed, got nil")
	}
}
