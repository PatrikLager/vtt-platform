package gateway

import (
	"context"
	"log/slog"
	"sync"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Leave Viewpoint empty: a spectator watches nobody until a perch names a
// shoulder (SPEC-015).
func viewerFor(p *identity.Participant) Viewer {
	return Viewer{ParticipantID: p.ID, Role: p.Role}
}

// Answer false for the DM and the agent alone: a player answered false is sent
// the whole log (TestSessionZeroCannotHappenAgain).
func projected(r identity.Role) bool {
	switch r {
	case identity.RoleDM, identity.RoleAgent:
		return false
	}
	return true
}

// Touch a seat from the pump alone once serve has drained its catch-up, and
// hand it anything else through a channel, never a lock: frames must leave in
// the order its projector changed (SPEC-011, SPEC-015).
type seat struct {
	// Keep pr nil for an unprojected seat: receive then folds nothing (SPEC-015).
	pr *Projector
	// Drop output at or below resume, never input (pastResume, SPEC-015).
	resume int64
	// Fold these with campaign.FoldPrefix, never with a loop of your own
	// (SPEC-015).
	received []*vttv1.Envelope

	// See receive, which sets it, and perch, which reads it (SPEC-015).
	world *engine.State
}

// Keep one slot, latest wins, and never block set: a queue's depth is the
// client's to choose, and a blocking hand-off stalls the table (SPEC-015,
// docs/reports/2026-08-18-visibility.md). Guard the slot with mu and nothing
// else: the pump orders frames, not this lock.
type perchBox struct {
	mu       sync.Mutex
	shoulder string
	full     bool
	// Keep wake at capacity 1: unbuffered, set's send is dropped while the pump is
	// busy and the shoulder waits for the next hop.
	wake chan struct{}
}

func newPerchBox() *perchBox { return &perchBox{wake: make(chan struct{}, 1)} }

// Never block here: the command goroutine must not wait on the pump (SPEC-015).
func (b *perchBox) set(actorID string) {
	b.mu.Lock()
	b.shoulder, b.full = actorID, true
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// Report ok=false once the slot is empty: a second wake-up must not re-apply a
// shoulder (TestARapidHopIsCoalescedToTheShoulderItEndedOn).
func (b *perchBox) take() (actorID string, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	actorID, ok = b.shoulder, b.full
	b.full = false
	return actorID, ok
}

// newSeat builds the seat SPEC-015 states for p's connection.
func newSeat(p *identity.Participant, after int64) *seat {
	if !projected(p.Role) {
		return &seat{}
	}
	return &seat{pr: NewProjector(viewerFor(p)), resume: after}
}

// subscribeFrom is where the seat's subscription starts (SPEC-011), not where
// its output starts (pastResume).
func (s *seat) subscribeFrom(after int64) int64 {
	if s.pr == nil {
		return after
	}
	return 0
}

// Return an unprojected seat's event itself, unchanged: the DM's and the
// agent's streams are the log
// (TestTheDMAndTheAgentStreamsAreUnchangedByTheProjection).
func (s *seat) receive(env *vttv1.Envelope) []*vttv1.Envelope {
	if s.pr == nil {
		return []*vttv1.Envelope{env}
	}

	s.received = append(s.received, env)
	// Hand Project the state after env, folded from what this seat received, never
	// campaign.State(): during catch-up the head is the future (SPEC-015).
	world, err := campaign.FoldPrefix(s.received)
	if err != nil {
		// Withhold the event when the fold fails: forwarding one nobody judged is the
		// leak (SPEC-015).
		// #nosec G706 -- the sequence is an int64 given as a structured attribute.
		slog.Error("gateway: projection fold failed; withholding event",
			"sequence", env.GetSequence(), "error", err)
		return nil
	}

	// Set world only after a fold succeeded: perch keeps the last good one.
	s.world = world

	return s.pastResume(s.pr.Project(env, world))
}

// Add no fast path for resume 0: the filter already keeps every envelope there,
// and the guard would be a mutant no test can kill.
func (s *seat) pastResume(out []*vttv1.Envelope) []*vttv1.Envelope {
	kept := make([]*vttv1.Envelope, 0, len(out))
	for _, e := range out {
		// Keep this strictly greater, as store.Subscribe's is: after=N means the
		// client holds N.
		if e.GetSequence() > s.resume {
			kept = append(kept, e)
		}
	}
	return kept
}

// Call perch from the pump alone, beside receive, and never pass its output
// through pastResume: its frames carry sequence 0 (SPEC-015).
func (s *seat) perch(actorID string) []*vttv1.Envelope {
	if s.pr == nil {
		// Return nothing: an unprojected seat has no projection to move
		// (TestAnUnprojectedSeatCannotBePerched).
		return nil
	}
	return s.pr.reperch(actorID, s.world)
}

// Build a fresh Projector per question: canSee must record nothing a seat was
// shown (SPEC-015).
func canSee(v Viewer, st *engine.State, sceneID string, at *vttv1.GridPosition) bool {
	pr := NewProjector(v)
	return pr.canSeeSquare(pr.look(st), sceneID, at)
}

// catchUp is the catch-up SPEC-011 states; a projected seat's head is the last
// sequence it returns, never the log's (VTT-086).
func (s *seat) catchUp(ctx context.Context, events <-chan *vttv1.Envelope, logHead int64) ([]*vttv1.Envelope, int64) {
	if s.pr == nil || logHead <= 0 {
		return nil, logHead
	}
	var out []*vttv1.Envelope
	var head int64
	for {
		select {
		case env, ok := <-events:
			if !ok {
				// Answer what this seat got: a head it cannot reach is a second failure.
				return out, head
			}
			projected := s.receive(env)
			out = append(out, projected...)
			// Take the last frame's sequence, not a max: one event's frames share its
			// sequence, and a max comparison is a mutant no test can kill.
			if n := len(projected); n > 0 {
				head = projected[n-1].GetSequence()
			}
			// Stop at the first envelope at or past the log's head: the pump sends the
			// rest.
			if env.GetSequence() >= logHead {
				return out, head
			}
		case <-ctx.Done():
			return out, head
		}
	}
}
