// Package campaign composes the store and the engine: replay on open and the
// validate→persist→apply append path (spec §3, §7). It is the only package
// that imports both store and engine.
//
// IT NO LONGER COMPOSES AN UNDO. Spec §6 of the event-core design specified
// one, and the whole of it — Undo, the retracted set, and the fold's first
// pass — left on 2026-08-31 (spec 2026-08-30-retraction-leaves): a retraction
// exists to make something not have happened, and it cannot, because the
// table has already read the log. The log only goes forward.
package campaign

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// errPoisoned is returned by every Campaign method once poisoned is set.
// The two paths that set it are both post-persist: the log already holds an
// event that the live projection failed to fold. At that point c.state no
// longer reliably reflects the log, and continuing to serve reads/writes
// from it risks compounding the divergence. Both trigger paths are
// defensively unreachable by design — Append and AppendBatch each fold a
// snapshot clone before persisting anything — so poisoning exists as a
// fail-loud backstop, not an expected state.
var errPoisoned = errors.New("campaign: poisoned by post-persist failure; reopen required")

// Campaign composes a store.Store (the log) with an engine.State (the live
// projection). If a post-persist step ever fails — the log was written but
// the in-memory projection could not be advanced to match — the Campaign
// marks itself poisoned: every subsequent Append, AppendBatch, State, and
// Subscribe call fails (State returns nil) rather than serve state that may
// no longer match the log. There is no in-process recovery from a poisoned Campaign;
// the caller must Close and Open it again, which rebuilds the projection
// from the log from scratch.
type Campaign struct {
	mu    sync.Mutex
	log   *store.Store
	state *engine.State

	// head is the highest sequence the store has assigned so far — i.e.
	// store's SELECT MAX(seq) — maintained under c.mu on every successful
	// rebuild/append. It exists so AppendBatch can validate against clones
	// stamped with the SAME contiguous sequences the store is about to
	// assign (head+1..head+N); see AppendBatch for why that equivalence
	// holds. head counts every persisted row — nothing is ever deleted from
	// the log — exactly as store's MAX(seq) does.
	head int64

	// poisoned is set when a post-persist step fails (see errPoisoned). Once
	// true, every method fails until the Campaign is reopened.
	poisoned bool
}

func Open(path string) (*Campaign, error) {
	s, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	c := &Campaign{log: s}
	if err := c.rebuildLocked(); err != nil {
		_ = s.Close() // rebuild failed; the returned error is what matters
		return nil, err
	}
	return c, nil
}

// rebuildLocked derives state from the full log: one pass through
// foldEvents, applying every event the store returns in sequence order.
func (c *Campaign) rebuildLocked() error {
	events, err := c.log.ReadAfter(0)
	if err != nil {
		return err
	}
	st, err := foldEvents(events)
	if err != nil {
		return err
	}
	c.state = st
	// events are ordered by sequence; the last one is the log head.
	c.head = 0
	if n := len(events); n > 0 {
		c.head = events[n-1].Sequence
	}
	return nil
}

// foldEvents folds events into a fresh engine.State, applying every envelope
// it is given in the order it is given them. Unknown variants are skipped
// with a warning (forward compatibility, spec §7); any other apply error
// means the resulting fold does not replay cleanly. This is the single fold,
// reached two ways — rebuildLocked's on-open rebuild and FoldPrefix's answer
// for the gateway — because the codebase's core principle is one fold, not
// two copies of the same loop (CLAUDE.md rule 4).
//
// NOTHING IS SKIPPED BY SEQUENCE any more (2026-08-31: the log only goes
// forward). It took a set of retracted sequences until then, and dropping
// that parameter is the whole of retraction's departure from this loop.
func foldEvents(events []*vttv1.Envelope) (*engine.State, error) {
	st := engine.NewState()
	for _, env := range events {
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

// Append validates against the projection, persists (the commit point), then
// advances the live projection. Any validation error writes nothing (spec §7).
// Returns errPoisoned without touching the log if the Campaign is poisoned
// (see the Campaign doc comment) — reopen the Campaign to recover.
func (c *Campaign) Append(env *vttv1.Envelope) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return 0, errPoisoned
	}
	// NOTHING APPENDS AN EventsRetracted. There is no longer a path that
	// produces one — the marker's only writer left with retraction on
	// 2026-08-31 — and the payload survives solely because the contract has
	// not caught up yet (Task 7 deletes the message and this guard with it).
	//
	// THE GUARD IS LOAD-BEARING, not a nicety: engine.Apply has an explicit
	// NO-OP arm for this payload ("handled by campaign rebuild, not in-line"),
	// so the validating fold below would accept it and the marker would
	// persist into a log where it now means nothing at all.
	if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
		return 0, errors.New("campaign: EventsRetracted is not an appendable event; the log only goes forward")
	}
	if env.Payload == nil {
		return 0, engine.ErrUnknownVariant
	}
	// Stamp session_id before validation, so validation (and the live apply
	// below) always see the final envelope (merge-gate decision, sub-project
	// 4): the campaign is authoritative over session_id, not the caller.
	if err := c.stampSessionID(env); err != nil {
		return 0, err
	}
	// Validate a CLONE stamped with the provisional sequence the store is
	// about to assign (c.head + 1) — the SAME fix, and the SAME
	// equivalence proof, as AppendBatch's clone-and-stamp validation below
	// (see AppendBatch's doc comment for the full argument): c.head is
	// maintained to equal store's MAX(seq) after every append/rebuild,
	// this whole call holds c.mu, and every log-appending path (Append and
	// AppendBatch, the only two) runs under c.mu, so no other append can advance
	// MAX(seq) between reading c.head here and store.Append's own
	// MAX(seq)+1 read — same lock ⇒ same head ⇒ identical sequence.
	//
	// This matters because some folds are sequence-DEPENDENT for a
	// REJECTION decision, not just a stored value: NarrationAdded's
	// anchor-sanity check (internal/engine/apply.go) rejects when
	// anchor_to_seq >= env.Sequence, so validating against an unstamped,
	// Sequence==0 envelope rejected EVERY anchored narration regardless of
	// anchor validity (world-layer sub-project 8, P11 Task 3's finding —
	// see .superpowers/sdd/p11-task-3-report.md) — the single-Append twin
	// of the F1/5c AppendBatch double-SessionEnded bug
	// (appendbatch_session_test.go). Folds that only WRITE using
	// env.Sequence (SessionStarted's StartSeq, SessionEnded's EndSeq,
	// ConditionApplied's AppliedSeq, NoteUpserted's UpdatedSeq) were never
	// observably broken by this: the throwaway validation clone's written
	// value is discarded either way, and the live env's actual stored
	// value always comes from the SECOND (post-persist) engine.Apply call
	// below, which always saw the correct store-assigned Sequence, before
	// and after this fix (see
	// TestSessionEndedEndSeqUnaffectedByValidationSequence,
	// append_sequence_validation_test.go, for the proof). The real env
	// keeps Sequence==0 (store.Append requires that and stamps it itself);
	// only the throwaway clone carries the provisional sequence, used
	// solely to drive validation — a rejection still cannot half-mutate
	// live state, since only the clone is folded here.
	clone := proto.Clone(env).(*vttv1.Envelope)
	clone.Sequence = c.head + 1
	if err := engine.Apply(c.state.Snapshot(), clone); err != nil {
		return 0, err
	}
	seq, err := c.log.Append(env) // persists; does not notify
	if err != nil {
		return 0, err
	}
	c.head = seq // the store just assigned this as the new log head
	if err := engine.Apply(c.state, env); err != nil {
		// Snapshot-validated, so this is unreachable; fail loudly if not.
		// The event is already persisted (commit point above) but the live
		// projection could not be advanced to match — poison the Campaign
		// rather than serve a projection that has silently fallen behind
		// the log. Do not notify: a poisoned campaign must not advertise an
		// event its own projection couldn't fold.
		c.poisoned = true
		return 0, fmt.Errorf("campaign: live apply diverged from validation: %w", err)
	}
	// Notify only after the live projection reflects env, so a subscriber
	// that observes this event can always read state >= it (see
	// store.Store.Notify's doc comment).
	c.log.Notify(env)
	return seq, nil
}

// stampSessionID stamps env.SessionId under c.mu, before validation/persist
// (merge-gate decision, sub-project 4; corrected to match spec §1.1 during
// workflow-level final review — see docs/superpowers/specs/
// 2026-07-24-hardening-design.md): a SessionStarted event gets a fresh,
// generated id; every other event gets the currently open session's id,
// overwriting any caller-supplied value (the campaign is authoritative, not
// the caller). With no session open, the incoming value is CLEARED to ""
// rather than left as-is: a closed (or never-opened) session must never let
// a stale or caller-supplied id ride along on an out-of-session event (spec:
// "no open session → empty" — legitimate, e.g. table setup before the first
// SessionStarted).
func (c *Campaign) stampSessionID(env *vttv1.Envelope) error {
	return stampSessionIDAgainst(env, c.state)
}

// stampSessionIDAgainst is stampSessionID's logic parameterized on WHICH
// state's Sessions to read the open session from. Single-event Append
// always checks the live c.state (via stampSessionID above). AppendBatch
// instead passes an evolving snapshot clone that it folds against event by
// event, so a SessionStarted earlier in the SAME batch is already visible
// (session considered open) when a later envelope in that batch is stamped
// — matching what two sequential Append calls would produce.
func stampSessionIDAgainst(env *vttv1.Envelope, st *engine.State) error {
	if _, ok := env.Payload.(*vttv1.Envelope_SessionStarted); ok {
		id, err := newSessionID()
		if err != nil {
			return err
		}
		env.SessionId = id
		return nil
	}
	if id, ok := openSessionID(st); ok {
		env.SessionId = id
	} else {
		env.SessionId = ""
	}
	return nil
}

// openSessionID returns the id of st's currently open session (the one
// whose EndSeq is still 0), or "", false if none is open.
func openSessionID(st *engine.State) (string, bool) {
	for _, s := range st.Sessions {
		if s.EndSeq == 0 {
			return s.ID, true
		}
	}
	return "", false
}

// newSessionID returns a fresh, random "sess-"-prefixed hex session id (16
// bytes from crypto/rand — collision-negligible, mirroring
// gateway.newEventID's construction).
func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("campaign: generate session id: %w", err)
	}
	return "sess-" + hex.EncodeToString(b), nil
}

// AppendBatch validates and persists a batch of events atomically (spec
// §6, data-integrity-core review depth): the WHOLE batch is folded
// sequentially against ONE snapshot clone before anything persists — any
// single event failing rejects the entire batch, writing nothing — then
// persisted via store.AppendBatch (one transaction, contiguous sequences),
// then live-applied to the live projection, then notified in order. All of
// this happens under ONE acquisition of c.mu — the SAME mutex Append uses,
// held for the whole call with no unlock between validate and persist — so
// no other Campaign operation can run concurrently with it, let alone
// interleave a foreign event into this batch's notify run.
//
// Session ids are stamped envelope by envelope against the evolving
// snapshot (not the live state) as it is folded, so a SessionStarted
// earlier in the SAME batch is already visible to a later envelope's
// session-open check — exactly as two sequential Append calls would behave
// (see stampSessionIDAgainst).
//
// Rejects a zero-length batch and any EventsRetracted envelope — nothing
// produces one at all now (see Append's own guard) — exactly as Append does
// per-event. Returns errPoisoned without touching the log if the Campaign is
// poisoned (see the Campaign doc comment) — reopen the Campaign to recover.
// A post-persist live-apply failure poisons the Campaign, exactly matching
// Append's poison contract.
func (c *Campaign) AppendBatch(envs []*vttv1.Envelope) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return 0, errPoisoned
	}
	if len(envs) == 0 {
		return 0, errors.New("campaign: append batch must not be empty")
	}
	for _, env := range envs {
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			return 0, errors.New("campaign: EventsRetracted is not an appendable event; the log only goes forward")
		}
		if env.Payload == nil {
			return 0, engine.ErrUnknownVariant
		}
	}
	// Stamp session_id and validate each envelope IN ORDER against one
	// evolving snapshot clone: any envelope failing to fold rejects the
	// entire batch untouched (nothing stamped survives past this loop's
	// error return except in the caller's own envelope objects — matching
	// Append's own behavior, where a rejected event's stamped session_id
	// is not reverted either).
	//
	// The validation fold MUST see the same sequence each envelope will
	// carry when it is later live-applied, because some folds are
	// sequence-dependent (SessionEnded writes EndSeq = env.Sequence, and
	// EndSeq==0 is the "session still open" sentinel — folding an unstamped,
	// Sequence==0 SessionEnded would leave the session looking open and
	// validate differently than it live-applies). We fold CLONES stamped
	// with provisional contiguous sequences head+1..head+N.
	//
	// Those provisional values are PROVABLY identical to what store.AppendBatch
	// will assign below: the store assigns MAX(seq)+1..MAX(seq)+N, and c.head
	// is maintained to equal store's MAX(seq) after every append/rebuild.
	// The whole call holds c.mu, and every path that appends to the log
	// (Append and AppendBatch, the only two) runs under c.mu, so no other
	// append can advance MAX(seq) between reading c.head here and store.AppendBatch's
	// own MAX(seq) read — same lock ⇒ same head ⇒ identical sequences. The
	// real envelopes keep Sequence==0 (store.AppendBatch requires that and
	// stamps them itself); only the throwaway clones carry the provisional
	// sequence, used solely to drive validation.
	snap := c.state.Snapshot()
	for i, env := range envs {
		if err := stampSessionIDAgainst(env, snap); err != nil {
			return 0, err
		}
		clone := proto.Clone(env).(*vttv1.Envelope)
		clone.Sequence = c.head + int64(i) + 1
		if err := engine.Apply(snap, clone); err != nil {
			return 0, err
		}
	}
	firstSeq, err := c.log.AppendBatch(envs) // persists; does not notify
	if err != nil {
		return 0, err
	}
	c.head = firstSeq + int64(len(envs)) - 1 // store just assigned this run
	for _, env := range envs {
		if err := engine.Apply(c.state, env); err != nil {
			// Genuinely unreachable now: validation folded clones carrying
			// the exact sequences store.AppendBatch just assigned (proven
			// above), so a fold that passed validation cannot fail here.
			// Kept as a fail-loud backstop. The batch is already persisted
			// (commit point above) but the live projection could not be
			// advanced to match — poison the Campaign rather than serve a
			// projection that has silently fallen behind the log. Do not
			// notify anything: a poisoned campaign must not advertise events
			// its own projection couldn't fold.
			c.poisoned = true
			return 0, fmt.Errorf("campaign: live apply diverged from validation: %w", err)
		}
	}
	// Notify only after the live projection reflects the WHOLE batch, so a
	// subscriber that observes any one of these events can always read
	// state >= it (see store.Store.Notify's doc comment).
	for _, env := range envs {
		c.log.Notify(env)
	}
	return firstSeq, nil
}

// State returns a snapshot of the live projection, or nil if the Campaign is
// poisoned (see the Campaign doc comment) — reopen the Campaign to recover.
func (c *Campaign) State() *engine.State {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return nil
	}
	return c.state.Snapshot()
}

// Subscribe delegates to the underlying store.Store. Returns errPoisoned if
// the Campaign is poisoned (see the Campaign doc comment) — reopen the
// Campaign to recover. The log itself remains intact and readable through a
// fresh Campaign even while poisoned; only this in-process projection is
// suspect.
//
// catchUpHead is the sequence catch-up ended at — see Store.Subscribe.
func (c *Campaign) Subscribe(afterSeq int64, buffer int) (events <-chan *vttv1.Envelope, unsubscribe func(), catchUpHead int64, err error) {
	return c.SubscribeWithNoProgressTimeout(afterSeq, buffer, 0)
}

// SubscribeWithNoProgressTimeout is Subscribe with an explicit liveness budget
// — see store.Store.SubscribeWithNoProgressTimeout. A non-positive noProgress
// means the store's default.
func (c *Campaign) SubscribeWithNoProgressTimeout(afterSeq int64, buffer int, noProgress time.Duration) (events <-chan *vttv1.Envelope, unsubscribe func(), catchUpHead int64, err error) {
	c.mu.Lock()
	poisoned := c.poisoned
	c.mu.Unlock()
	if poisoned {
		return nil, nil, 0, errPoisoned
	}
	return c.log.SubscribeWithNoProgressTimeout(afterSeq, buffer, noProgress)
}

func (c *Campaign) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.log.Close()
}
