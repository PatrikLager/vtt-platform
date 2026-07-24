package campaign_test

import (
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// TestSessionStampFreshIDOnSessionStarted covers case (a) of the session_id
// stamping decision (merge-gate, ADR-009 sub-project 4): a SessionStarted
// envelope appended with no caller-supplied session id comes back stamped
// with a fresh, non-empty "sess-"-prefixed id, and the live projection's
// Sessions[0].ID equals that same stamped value.
func TestSessionStampFreshIDOnSessionStarted(t *testing.T) {
	c := openTemp(t)

	env := cenv(nextID(), &vttv1.SessionStarted{Name: "n"})
	env.SessionId = ""
	must(t, c, env)

	if env.SessionId == "" {
		t.Fatal("want non-empty session id stamped onto the appended SessionStarted envelope, got empty")
	}
	if !strings.HasPrefix(env.SessionId, "sess-") {
		t.Fatalf(`want "sess-"-prefixed session id, got %q`, env.SessionId)
	}

	st := c.State()
	if len(st.Sessions) != 1 {
		t.Fatalf("want 1 session in state, got %d", len(st.Sessions))
	}
	if st.Sessions[0].ID != env.SessionId {
		t.Fatalf("State().Sessions[0].ID = %q, want %q (the stamped envelope id)", st.Sessions[0].ID, env.SessionId)
	}
}

// TestSessionStampSessionStartedOverridesCallerSuppliedID is case (a)'s
// sibling: a SessionStarted carrying a caller-supplied NON-EMPTY session id
// still gets a fresh, DIFFERENT "sess-"-prefixed id — generation is
// unconditional, not merely "fill in when empty". A "generate-only-when-empty"
// regression would pass every other SessionStamp case (none of them append a
// SessionStarted with a non-empty incoming id) and slip through undetected;
// this test is the one that pins that behavior specifically. See ADR-009 §3
// fault-injection proof in the task report: this test passes today, and was
// proven able to fail under that exact regression before being trusted.
func TestSessionStampSessionStartedOverridesCallerSuppliedID(t *testing.T) {
	c := openTemp(t)

	env := cenv(nextID(), &vttv1.SessionStarted{Name: "n"})
	env.SessionId = "caller-supplied-wrong-id"
	must(t, c, env)

	if env.SessionId == "caller-supplied-wrong-id" {
		t.Fatal("want SessionStarted's caller-supplied session id overridden with a fresh generated one, got it echoed back unchanged")
	}
	if !strings.HasPrefix(env.SessionId, "sess-") {
		t.Fatalf(`want "sess-"-prefixed generated session id, got %q`, env.SessionId)
	}

	st := c.State()
	if len(st.Sessions) != 1 {
		t.Fatalf("want 1 session in state, got %d", len(st.Sessions))
	}
	if st.Sessions[0].ID != env.SessionId {
		t.Fatalf("State().Sessions[0].ID = %q, want %q (the freshly stamped envelope id)", st.Sessions[0].ID, env.SessionId)
	}
}

// TestSessionStampPropagatesToSubsequentEvents covers case (b): once a
// session is open, later events in that session (SceneCreated, TokenMoved)
// carry the SAME session id the SessionStarted envelope was stamped with —
// even though cenv hands them a different, caller-supplied placeholder
// ("sess-1") that the campaign must overwrite.
func TestSessionStampPropagatesToSubsequentEvents(t *testing.T) {
	c := openTemp(t)

	start := cenv(nextID(), &vttv1.SessionStarted{Name: "n"})
	start.SessionId = ""
	must(t, c, start)
	sid := start.SessionId

	sceneEnv := cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	})
	must(t, c, sceneEnv)
	if sceneEnv.SessionId != sid {
		t.Fatalf("SceneCreated session id = %q, want %q (the open session's id)", sceneEnv.SessionId, sid)
	}

	actorEnv := cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
	})
	must(t, c, actorEnv)

	placeEnv := cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "scn", ActorId: "a1",
		Position: &vttv1.GridPosition{X: 1, Y: 1},
	})
	must(t, c, placeEnv)

	moveEnv := cenv(nextID(), &vttv1.TokenMoved{
		TokenId: "t1", SceneId: "scn",
		From: &vttv1.GridPosition{X: 1, Y: 1},
		To:   &vttv1.GridPosition{X: 2, Y: 2},
	})
	must(t, c, moveEnv)
	if moveEnv.SessionId != sid {
		t.Fatalf("TokenMoved session id = %q, want %q (the open session's id)", moveEnv.SessionId, sid)
	}
}

// TestSessionStampUsesSecondSessionAfterRestart covers case (c): after a
// SessionEnded followed by a new SessionStarted, subsequent events carry the
// SECOND session's id, not the first's.
func TestSessionStampUsesSecondSessionAfterRestart(t *testing.T) {
	c := openTemp(t)

	start1 := cenv(nextID(), &vttv1.SessionStarted{Name: "one"})
	start1.SessionId = ""
	must(t, c, start1)
	sid1 := start1.SessionId
	if sid1 == "" {
		t.Fatal("want non-empty session id after first SessionStarted")
	}

	must(t, c, cenv(nextID(), &vttv1.SessionEnded{}))

	start2 := cenv(nextID(), &vttv1.SessionStarted{Name: "two"})
	start2.SessionId = ""
	must(t, c, start2)
	sid2 := start2.SessionId
	if sid2 == "" {
		t.Fatal("want non-empty session id after second SessionStarted")
	}
	if sid1 == sid2 {
		t.Fatal("want the second session to receive a fresh id distinct from the first")
	}

	sceneEnv := cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	})
	must(t, c, sceneEnv)
	if sceneEnv.SessionId != sid2 {
		t.Fatalf("post-restart event session id = %q, want %q (the second, currently-open session)", sceneEnv.SessionId, sid2)
	}
}

// TestSessionStampLeavesIDVerbatimWithNoOpenSession covers case (d): with no
// session currently open, an appended event's incoming session id (whatever
// the caller supplied, including empty) is left untouched — there is no open
// session to stamp with.
func TestSessionStampLeavesIDVerbatimWithNoOpenSession(t *testing.T) {
	c := openTemp(t)

	env := cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Name: "Hero", ModuleId: "m"},
	})
	env.SessionId = "caller-supplied-value"
	must(t, c, env)

	if env.SessionId != "caller-supplied-value" {
		t.Fatalf("session id with no open session: got %q, want unchanged %q", env.SessionId, "caller-supplied-value")
	}
}

// TestSessionStampOverwritesWrongIncomingID covers case (e): a
// non-SessionStarted event submitted with a WRONG non-empty session id (not
// merely a missing one) is overwritten to the open session's id — the
// campaign is authoritative over session_id, not the caller.
func TestSessionStampOverwritesWrongIncomingID(t *testing.T) {
	c := openTemp(t)

	start := cenv(nextID(), &vttv1.SessionStarted{Name: "n"})
	start.SessionId = ""
	must(t, c, start)
	sid := start.SessionId

	sceneEnv := cenv(nextID(), &vttv1.SceneCreated{
		SceneId: "scn", Name: "S", GridWidth: 10, GridHeight: 10,
	})
	sceneEnv.SessionId = "wrong-session-id"
	must(t, c, sceneEnv)

	if sceneEnv.SessionId != sid {
		t.Fatalf("wrong incoming session id was not overwritten: got %q, want %q (the open session's id)", sceneEnv.SessionId, sid)
	}
}

// TestSessionStampUndoMarkerCarriesOpenSessionID covers case (f): the
// EventsRetracted marker Undo persists (and broadcasts) carries the open
// session's id.
//
// Step 2 dropped Undo's sessionID parameter entirely (the marker's
// session_id is now stamped by campaign, not caller-supplied) — this call
// site uses the new 6-arg signature, having been mechanically updated
// alongside every other Undo caller.
func TestSessionStampUndoMarkerCarriesOpenSessionID(t *testing.T) {
	c, moveSeq := seedMovedToken(t)
	sid := c.State().Sessions[0].ID

	ch, cancel, err := c.Subscribe(moveSeq, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	if err := c.Undo(moveSeq, moveSeq, "mistake", nextID(), "dm", "test-participant"); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		if _, ok := got.Payload.(*vttv1.Envelope_EventsRetracted); !ok {
			t.Fatalf("want EventsRetracted payload, got %T", got.Payload)
		}
		if got.SessionId != sid {
			t.Fatalf("undo marker session id = %q, want %q (the open session's id)", got.SessionId, sid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscriber to receive retraction marker")
	}
}
