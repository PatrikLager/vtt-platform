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
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// errPoisoned is returned by every Campaign method once poisoned is set. Only
// appendLocked sets it, after a persist, when the live state fails to fold what
// validation folded: unreachable by design, and kept as a fail-loud backstop.
var errPoisoned = errors.New("campaign: poisoned by post-persist failure; reopen required")

// ErrHeld is the refusal of an Open whose directory another Campaign holds.
var ErrHeld = errors.New("campaign: another writer holds the campaign directory")

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

	// Keep head equal to the store's MAX(seq) under c.mu: appendLocked validates
	// clones stamped head+1.., the sequences the store assigns next.
	head int64

	// poisoned is set when a post-persist step fails (see errPoisoned). Once
	// true, every method fails until the Campaign is reopened.
	poisoned bool

	logs      map[string]*characterLog
	discarded int

	// Call release once: a second close of its descriptor can close an
	// unrelated file.
	release func() error
}

// EnsureDir makes dir a campaign directory, creating it if missing and
// refusing a plain file, and takes no writer hold (SPEC-019).
func EnsureDir(dir string) error {
	info, err := os.Stat(dir)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("campaign: %s is a file; a campaign is a "+
			"directory holding log.db, maps/ and art/ — put it in one", dir)
	}
	// A read-only mount must not lose improvisation entirely (spec §12):
	// installing a map means writing a file into this directory, so a
	// campaign that cannot even be created here needs to say so in
	// campaign terms rather than surface os.MkdirAll's bare error
	// unexplained. MkdirAll can fail for reasons that are NOT about
	// writability (a dangling symlink at dir: "file exists"; a plain file
	// as an ancestor: "not a directory"), so the wrap states the
	// requirement — a campaign directory must exist and be writable — and
	// leads straight into %w for the actual cause, rather than asserting
	// that this specific failure was a permissions problem.
	//
	// 0o750, not 0o755: this directory holds identity.DB's participants and
	// token hashes (campaign.LogPath's log.db, once identity.Open is
	// pointed at it) alongside the campaign log, so it gets no more than
	// owner+group access rather than the world-readable default a plain
	// content directory would.
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("campaign: cannot make %s a usable campaign "+
			"directory — a campaign directory must exist and be writable, "+
			"since installing a map means putting a file in it: %w", dir, err)
	}
	return nil
}

// Open opens the campaign in dir as its one writer, refusing a directory
// another Campaign holds with ErrHeld (SPEC-019).
func Open(dir string) (*Campaign, error) {
	if err := EnsureDir(dir); err != nil {
		return nil, err
	}
	release, err := takeHold(dir)
	if err != nil {
		return nil, err
	}
	s, err := store.Open(LogPath(dir))
	if err != nil {
		_ = release()
		return nil, err
	}
	c := &Campaign{log: s, release: release}
	if err := c.rebuildLocked(); err != nil {
		_ = s.Close() // rebuild failed; the returned error is what matters
		_ = release()
		return nil, err
	}
	return c, nil
}

// Keep O_CLOEXEC: a child process that inherits the descriptor keeps the hold
// past Close.
func takeHold(dir string) (func() error, error) {
	fd, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("campaign: take the writer hold on %s: %w", dir, err)
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = syscall.Close(fd)
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%w %s; a campaign has one writer at a time, "+
				"so stop the vtt serve or close the Campaign that has it open", ErrHeld, dir)
		}
		return nil, fmt.Errorf("campaign: take the writer hold on %s: %w", dir, err)
	}
	return func() error { return syscall.Close(fd) }, nil
}

// LogPath names the log inside a campaign directory. Exported for callers
// that need the log file itself rather than the campaign directory — that
// is every caller of identity.Open, since it opens its own SQLite handle
// on the same file (internal/identity's package comment: "the same
// campaign file the store uses"), plus a handful of tests that reopen the
// log directly via store.Open or a raw sql.Open("sqlite", ...) to inspect
// it (e.g. row counts, schema) without going through either Open.
func LogPath(dir string) string { return filepath.Join(dir, "log.db") }

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
	return c.loadCharacterLogsLocked()
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

// Append validates env against a snapshot of the live state, persists it,
// then applies it live and notifies. A refusal writes nothing; a failure
// after the write poisons the Campaign, and every later call returns
// errPoisoned.
func (c *Campaign) Append(env *vttv1.Envelope) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return 0, errPoisoned
	}
	return c.appendLocked([]*vttv1.Envelope{env})
}

// AppendBatch is Append for a non-empty batch, all or nothing: one envelope
// that does not fold refuses the whole batch.
func (c *Campaign) AppendBatch(envs []*vttv1.Envelope) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.poisoned {
		return 0, errPoisoned
	}
	if len(envs) == 0 {
		return 0, errors.New("campaign: append batch must not be empty")
	}
	return c.appendLocked(envs)
}

// Hold c.mu for the whole call: c.head stands for the store's MAX(seq) only
// while no other append can run.
func (c *Campaign) appendLocked(envs []*vttv1.Envelope) (int64, error) {
	for _, env := range envs {
		if env.Payload == nil {
			return 0, engine.ErrUnknownVariant
		}
	}
	snap := c.state.Snapshot()
	for i, env := range envs {
		// Stamp against the evolving snapshot: a SessionStarted earlier in the
		// batch opens the session for the envelopes after it.
		if err := stampSessionIDAgainst(env, snap); err != nil {
			return 0, err
		}
		// Validate a clone stamped with the sequence the store assigns next, as
		// some folds reject by the event's own sequence
		// (TestAppendAcceptsAnchoredNarrationPointingAtRealPriorSequence). Keep
		// env unstamped: the store requires Sequence 0 and stamps it itself.
		clone := proto.Clone(env).(*vttv1.Envelope)
		clone.Sequence = c.head + int64(i) + 1
		if err := engine.Apply(snap, clone); err != nil {
			return 0, err
		}
	}
	entries, logs, dropped, err := c.fanOutLocked(envs)
	if err != nil {
		c.restartDecisionsLocked()
		return 0, err
	}
	firstSeq, err := c.log.AppendBatch(envs, entries) // persists; does not notify
	if err != nil {
		c.restartDecisionsLocked()
		return 0, err
	}
	c.head = firstSeq + int64(len(envs)) - 1
	c.logs, c.discarded = logs, c.discarded+dropped
	for _, env := range envs {
		if err := engine.Apply(c.state, env); err != nil {
			// Unreachable while validation folds the same envelopes at the same
			// sequences. Poison rather than serve a state behind the log, and
			// notify nothing.
			c.poisoned = true
			return 0, fmt.Errorf("campaign: live apply diverged from validation: %w", err)
		}
	}
	// Notify only once the live state holds the whole batch (Store.Notify).
	for _, env := range envs {
		c.log.Notify(env)
	}
	return firstSeq, nil
}

// Overwrite whatever SessionId the caller sent: a fresh id for a
// SessionStarted, else the session open in st or "". SPEC-018 leaves
// session_id unbounded because the server writes it.
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
	s, ok := openSession(st)
	return s.ID, ok
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

// Close closes the log and then ends the writer hold (SPEC-019).
func (c *Campaign) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.release == nil {
		return nil
	}
	logErr := c.log.Close()
	holdErr := c.release()
	c.release = nil
	return errors.Join(logErr, holdErr)
}
