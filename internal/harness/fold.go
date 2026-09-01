package harness

import (
	"errors"
	"fmt"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// Fold derives client-side state from a sequence of received envelopes by
// applying engine.Apply to each one, once, in order — this is deliberately
// not a second implementation of the event-core's semantics: it is the
// published derivation (docs/superpowers/specs/2026-07-24-simulation-
// harness-design.md §3) that any client, this harness today and the future
// TS client eventually, reconstructs identical state with, consuming only
// contract types plus engine.Apply — never internal/campaign or
// internal/store (the P1 boundary this whole package is bound by).
//
// Single pass, since Task 4 of docs/superpowers/plans/
// 2026-08-31-retraction-leaves.md (commit 92f1284): Fold no
// longer collects a skip-set from an EventsRetracted marker, because there is
// no code path left that skips by sequence at all. Task 7 of the same spec
// then deleted the message itself, so there is no longer a payload any fold
// could be asked to skip on. Any envelope whose payload engine.Apply reports
// as engine.ErrUnknownVariant is skipped (not failed on) — the same
// forward-compatibility behavior the server's own replay gives an event
// variant it doesn't recognize yet.
func Fold(events []*vttv1.Envelope) (*engine.State, error) {
	st := engine.NewState()
	for _, env := range events {
		if err := engine.Apply(st, env); err != nil {
			if errors.Is(err, engine.ErrUnknownVariant) {
				continue
			}
			return nil, fmt.Errorf("harness: fold: corrupt event at sequence %d: %w", env.Sequence, err)
		}
	}
	return st, nil
}
