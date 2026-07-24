package harness

import (
	"errors"
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// Fold derives client-side state from a sequence of received envelopes,
// using the SAME two-pass retraction semantics as the server's own fold
// (internal/campaign/campaign.go's foldEvents/rebuildLocked) — this is
// deliberately not a second implementation of that algorithm: it is the
// published derivation (docs/superpowers/specs/2026-07-24-simulation-
// harness-design.md §3) that any client, this harness today and the future
// TS client eventually, reconstructs identical state with, consuming only
// contract types plus engine.Apply — never internal/campaign or
// internal/store (the P1 boundary this whole package is bound by).
//
// Pass 1 collects every sequence covered by an EventsRetracted marker's
// [FromSequence, ToSequence] range (inclusive) into a set. Pass 2 applies
// engine.Apply to every envelope NOT in that set, in order, skipping (never
// applying in-line) EventsRetracted markers themselves — a marker changes
// history's SHAPE, not live state, by design; see engine.Apply's own doc
// comment — and skipping (not failing on) any envelope whose payload
// engine.Apply reports as engine.ErrUnknownVariant, the same forward-
// compatibility behavior rebuildLocked gives the server's replay.
func Fold(events []*vttv1.Envelope) (*engine.State, error) {
	retracted := map[int64]bool{}
	for _, env := range events {
		if r, ok := env.Payload.(*vttv1.Envelope_EventsRetracted); ok {
			for seq := r.EventsRetracted.GetFromSequence(); seq <= r.EventsRetracted.GetToSequence(); seq++ {
				retracted[seq] = true
			}
		}
	}

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
				continue
			}
			return nil, fmt.Errorf("harness: fold: corrupt event at sequence %d: %w", env.Sequence, err)
		}
	}
	return st, nil
}
