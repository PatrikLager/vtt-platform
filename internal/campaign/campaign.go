// Package campaign composes the store and the engine: replay on open, the
// validate→persist→apply append path, and compensating-marker undo (spec §3,
// §6, §7). It is the only package that imports both store and engine.
package campaign

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// errPoisoned is returned by every Campaign method once poisoned is set.
// The two paths that set it are both post-persist: the log already holds an
// event or marker that the live projection failed to fold. At that point
// c.state no longer reliably reflects the log, and continuing to serve
// reads/writes from it risks compounding the divergence. Both trigger paths
// are defensively unreachable by design (Append validates on a snapshot
// before persisting; Undo dry-runs the full fold before persisting the
// marker) — poisoning exists as a fail-loud backstop, not an expected state.
var errPoisoned = errors.New("campaign: poisoned by post-persist failure; reopen required")

// Campaign composes a store.Store (the log) with an engine.State (the live
// projection). If a post-persist step ever fails — the log was written but
// the in-memory projection could not be advanced to match — the Campaign
// marks itself poisoned: every subsequent Append, Undo, State, and Subscribe
// call fails (State returns nil) rather than serve state that may no longer
// match the log. There is no in-process recovery from a poisoned Campaign;
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
	// holds. head counts every persisted row, including retraction markers
	// and retracted events (nothing is ever deleted from the log), exactly
	// as store's MAX(seq) does.
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
	// events are ordered by sequence; the last one is the log head (markers
	// and retracted events included — they are real persisted rows the store
	// still counts in MAX(seq)).
	c.head = 0
	if n := len(events); n > 0 {
		c.head = events[n-1].Sequence
	}
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
// Returns errPoisoned without touching the log if the Campaign is poisoned
// (see the Campaign doc comment) — reopen the Campaign to recover.
func (c *Campaign) Append(env *vttv1.Envelope) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return 0, errPoisoned
	}
	if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
		return 0, errors.New("campaign: EventsRetracted must be appended via Undo")
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
	// this whole call holds c.mu, and every log-appending path (Append,
	// AppendBatch, Undo) runs under c.mu, so no other append can advance
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
// generated id; every other event — and Undo's EventsRetracted marker, which
// also routes through here — gets the currently open session's id,
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
// state's Sessions to read the open session from. Single-event Append/Undo
// always check the live c.state (via stampSessionID above). AppendBatch
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
// Rejects a zero-length batch and any EventsRetracted envelope (must go
// through Undo), exactly as Append does per-event. Returns errPoisoned
// without touching the log if the Campaign is poisoned (see the Campaign
// doc comment) — reopen the Campaign to recover. A post-persist live-apply
// failure poisons the Campaign, exactly matching Append's poison contract.
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
			return 0, errors.New("campaign: EventsRetracted must be appended via Undo")
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
	// (Append, AppendBatch, Undo) runs under c.mu, so no other append can
	// advance MAX(seq) between reading c.head here and store.AppendBatch's
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

// Undo appends an EventsRetracted marker and rebuilds the projection.
// The range must exist, contain no retraction markers (no nesting), and not
// overlap an already-retracted span (spec §6).
// Returns errPoisoned without touching the log if the Campaign is poisoned
// (see the Campaign doc comment) — reopen the Campaign to recover.
//
// actorRole and participantID stamp the marker's Envelope.ActorRole and
// Envelope.ParticipantId (spec §4: "every accepted command stamps
// actor_role AND ... participant_id"). Campaign has no identity concept of
// its own — it never sees a token or a *identity.Participant — so
// attribution is caller-supplied; the gateway is the authority on who
// issued the retraction and passes those values straight through.
//
// The marker's session_id is NOT caller-supplied (unlike actorRole and
// participantID): it is stamped the same way Append stamps every other
// event's, via stampSessionID, right before persisting — the currently open
// session's id, or cleared to "" if none is open (spec §1.1).
//
// Returns the marker's own assigned sequence on success (P6 Task 4 pre-step,
// controller decision: closes the P4 carry-forward "Undo may return it" —
// see gateway.handleRetraction, which now threads this straight into
// CommandResult.Sequence the same way Append's sequence does for every other
// command; spec §3's EXCEPTION note is updated accordingly). The returned
// sequence is 0 on every error path (poisoned, invalid range, rebuild
// failure) — callers must check the error, not assume a zero sequence means
// success.
func (c *Campaign) Undo(from, to int64, reason string, eventID, actorRole, participantID string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return 0, errPoisoned
	}
	if from < 1 || to < from {
		return 0, fmt.Errorf("campaign: invalid retraction range [%d,%d]", from, to)
	}
	events, err := c.log.ReadAfter(0)
	if err != nil {
		return 0, err
	}
	var maxSeq int64
	if n := len(events); n > 0 {
		maxSeq = events[n-1].Sequence
	}
	if to > maxSeq {
		return 0, fmt.Errorf("campaign: retraction range end %d beyond log head %d", to, maxSeq)
	}
	already := retractedSet(events)
	for _, env := range events {
		if env.Sequence < from || env.Sequence > to {
			continue
		}
		if _, isMarker := env.Payload.(*vttv1.Envelope_EventsRetracted); isMarker {
			return 0, fmt.Errorf("campaign: cannot retract a retraction (seq %d)", env.Sequence)
		}
		if already[env.Sequence] {
			return 0, fmt.Errorf("campaign: seq %d is already retracted", env.Sequence)
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
		return 0, fmt.Errorf("campaign: retraction would corrupt replay: %w", err)
	}

	marker := &vttv1.Envelope{
		EventId:       eventID,
		ActorRole:     actorRole,
		ParticipantId: participantID,
		OccurredAt:    timestamppb.Now(),
		Payload: &vttv1.Envelope_EventsRetracted{
			EventsRetracted: &vttv1.EventsRetracted{
				FromSequence: from, ToSequence: to, Reason: reason,
			},
		},
	}
	// stampSessionID never errors on a non-SessionStarted payload (the
	// generation path, which can, is only reachable for SessionStarted) —
	// the marker's payload is always EventsRetracted.
	_ = c.stampSessionID(marker)
	seq, err := c.log.Append(marker)
	if err != nil {
		return 0, err
	}
	if err := c.rebuildLocked(); err != nil {
		// The marker is already persisted (commit point above) but the
		// projection could not be rebuilt to match — poison the Campaign
		// rather than serve a projection that has silently fallen behind
		// the log. Do not notify: a poisoned campaign must not advertise an
		// event its own projection couldn't fold.
		c.poisoned = true
		return 0, err
	}
	// Notify only after the rebuilt projection reflects marker, mirroring
	// Append's post-apply notify ordering.
	c.log.Notify(marker)
	return seq, nil
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
func (c *Campaign) Subscribe(afterSeq int64, buffer int) (<-chan *vttv1.Envelope, func(), error) {
	c.mu.Lock()
	poisoned := c.poisoned
	c.mu.Unlock()
	if poisoned {
		return nil, nil, errPoisoned
	}
	return c.log.Subscribe(afterSeq, buffer)
}

func (c *Campaign) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.log.Close()
}
