// Package campaign composes the store and the engine: replay on open, the
// validate→persist→apply append path, and compensating-marker undo (spec §3,
// §6, §7). It is the only package that imports both store and engine.
package campaign

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

type Campaign struct {
	mu    sync.Mutex
	log   *store.Store
	state *engine.State
}

func Open(path string) (*Campaign, error) {
	s, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	c := &Campaign{log: s}
	if err := c.rebuildLocked(); err != nil {
		s.Close()
		return nil, err
	}
	return c, nil
}

// rebuildLocked derives state from the full log: pass 1 collects retracted
// ranges, pass 2 folds the non-retracted events via foldEvents.
func (c *Campaign) rebuildLocked() error {
	events, err := c.log.ReadAfter(0)
	if err != nil {
		return err
	}
	st, err := foldEvents(events, retractedSet(events))
	if err != nil {
		return err
	}
	c.state = st
	return nil
}

// foldEvents folds events into a fresh engine.State, skipping any sequence
// present in retracted and every EventsRetracted marker itself. Unknown
// variants are skipped with a warning (forward compatibility, spec §7); any
// other apply error means the resulting fold does not replay cleanly. This
// is the single fold shared by rebuildLocked (the live/on-open rebuild) and
// Undo's dry-run viability check — the codebase's core principle is one
// fold, not two copies of the same loop.
func foldEvents(events []*vttv1.Envelope, retracted map[int64]bool) (*engine.State, error) {
	st := engine.NewState()
	for _, env := range events {
		if retracted[env.Sequence] {
			continue
		}
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			continue
		}
		if err := engine.Apply(st, env); err != nil {
			if errors.Is(err, engine.ErrUnknownVariant) {
				slog.Warn("campaign: skipping unknown event variant during replay",
					"sequence", env.Sequence, "event_id", env.EventId)
				continue
			}
			return nil, fmt.Errorf("campaign: corrupt log at seq %d: %w", env.Sequence, err)
		}
	}
	return st, nil
}

func retractedSet(events []*vttv1.Envelope) map[int64]bool {
	out := map[int64]bool{}
	for _, env := range events {
		if r, ok := env.Payload.(*vttv1.Envelope_EventsRetracted); ok {
			for seq := r.EventsRetracted.FromSequence; seq <= r.EventsRetracted.ToSequence; seq++ {
				out[seq] = true
			}
		}
	}
	return out
}

// Append validates against the projection, persists (the commit point), then
// advances the live projection. Any validation error writes nothing (spec §7).
func (c *Campaign) Append(env *vttv1.Envelope) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
		return 0, errors.New("campaign: EventsRetracted must be appended via Undo")
	}
	if env.Payload == nil {
		return 0, engine.ErrUnknownVariant
	}
	// Validate on a snapshot so a rejection cannot half-mutate live state.
	if err := engine.Apply(c.state.Snapshot(), env); err != nil {
		return 0, err
	}
	seq, err := c.log.Append(env) // persists AND notifies subscribers
	if err != nil {
		return 0, err
	}
	if err := engine.Apply(c.state, env); err != nil {
		// Snapshot-validated, so this is unreachable; fail loudly if not.
		return 0, fmt.Errorf("campaign: live apply diverged from validation: %w", err)
	}
	return seq, nil
}

// Undo appends an EventsRetracted marker and rebuilds the projection.
// The range must exist, contain no retraction markers (no nesting), and not
// overlap an already-retracted span (spec §6).
func (c *Campaign) Undo(from, to int64, reason string, eventID, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if from < 1 || to < from {
		return fmt.Errorf("campaign: invalid retraction range [%d,%d]", from, to)
	}
	events, err := c.log.ReadAfter(0)
	if err != nil {
		return err
	}
	var maxSeq int64
	if n := len(events); n > 0 {
		maxSeq = events[n-1].Sequence
	}
	if to > maxSeq {
		return fmt.Errorf("campaign: retraction range end %d beyond log head %d", to, maxSeq)
	}
	already := retractedSet(events)
	for _, env := range events {
		if env.Sequence < from || env.Sequence > to {
			continue
		}
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			return fmt.Errorf("campaign: cannot retract a retraction (seq %d)", env.Sequence)
		}
		if already[env.Sequence] {
			return fmt.Errorf("campaign: seq %d is already retracted", env.Sequence)
		}
	}

	// Dry-run the filtered fold before persisting anything: the range shape
	// checks above (bounds, no-nesting, no-double-retraction) don't tell us
	// whether the log still replays once this range is retracted too. Build
	// the would-be retracted set (already-retracted ∪ this range) and fold
	// the full log against it through a scratch state. If that fold fails,
	// this retraction would corrupt replay — reject it and persist nothing.
	wouldBeRetracted := make(map[int64]bool, len(already)+int(to-from)+1)
	for seq, v := range already {
		wouldBeRetracted[seq] = v
	}
	for seq := from; seq <= to; seq++ {
		wouldBeRetracted[seq] = true
	}
	if _, err := foldEvents(events, wouldBeRetracted); err != nil {
		return fmt.Errorf("campaign: retraction would corrupt replay: %w", err)
	}

	marker := &vttv1.Envelope{
		EventId:   eventID,
		SessionId: sessionID,
		ActorRole: "dm",
		Payload: &vttv1.Envelope_EventsRetracted{
			EventsRetracted: &vttv1.EventsRetracted{
				FromSequence: from, ToSequence: to, Reason: reason,
			},
		},
	}
	if _, err := c.log.Append(marker); err != nil {
		return err
	}
	return c.rebuildLocked()
}

func (c *Campaign) State() *engine.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state.Snapshot()
}

func (c *Campaign) Subscribe(afterSeq int64, buffer int) (<-chan *vttv1.Envelope, func(), error) {
	return c.log.Subscribe(afterSeq, buffer)
}

func (c *Campaign) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.log.Close()
}
