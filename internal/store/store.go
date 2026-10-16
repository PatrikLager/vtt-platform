// Package store is the append-only event log. It persists protobuf-binary
// vtt.v1 Envelopes in SQLite, assigns the authoritative sequence, and knows
// nothing about game state (spec §4).
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// Keep event_id unique in events only: one world event remembered by several
// character logs keeps its id in each (VTT-319).
const schema = `
CREATE TABLE IF NOT EXISTS events (
  seq         INTEGER PRIMARY KEY,
  event_id    TEXT NOT NULL UNIQUE,
  session_id  TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  payload     BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS character_events (
  actor_id    TEXT    NOT NULL,
  seq         INTEGER NOT NULL,
  world_seq   INTEGER NOT NULL,
  event_id    TEXT    NOT NULL,
  occurred_at TEXT    NOT NULL,
  payload     BLOB    NOT NULL,
  PRIMARY KEY (actor_id, seq)
);`

// Entry is one entry for ActorID's character log, written with the world
// event at index Cause of its batch, or with none when Cause is -1.
type Entry struct {
	ActorID string
	Cause   int
	Env     *vttv1.Envelope
}

// CharacterEntry is one stored entry of a character log and the world
// sequence it was written with, 0 for none.
type CharacterEntry struct {
	WorldSeq int64
	Env      *vttv1.Envelope
}

type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	subs []*subscriber
}

func Open(path string) (*Store, error) {
	// busy_timeout(5000): a concurrent SQLITE_BUSY (another handle on the
	// same file mid-write — intra-process, e.g. Store+identity.DB both open
	// on one campaign, or cross-process) retries for up to 5s instead of
	// failing immediately (ledgered carry-forward, P6 Task 4 review).
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close() // closing a handle whose schema init just failed
		return nil, fmt.Errorf("store: init schema: %w", err)
	}
	return &Store{db: db}, nil
}

// AppendBatch stamps the next world sequences into envs and each entry's next
// sequence in its actor's log, and persists all of them in ONE transaction
// (VTT-315, VTT-321). Submit every Sequence 0, a unique event_id per envelope
// and a non-empty envs. A failure leaves every Sequence as it was submitted,
// so a caller checking Sequence != 0 never mistakes a half-stamped batch for a
// persisted one. It does not notify (Notify's doc comment).
func (s *Store) AppendBatch(envs []*vttv1.Envelope, entries []Entry) (int64, error) {
	if len(envs) == 0 {
		return 0, errors.New("store: append batch must not be empty")
	}
	for _, env := range envs {
		if env.Sequence != 0 {
			return 0, errors.New("store: envelope sequence must be 0 on append")
		}
		if env.EventId == "" {
			return 0, errors.New("store: envelope event_id must not be empty")
		}
	}
	for _, e := range entries {
		if e.Env == nil || e.Env.Sequence != 0 {
			return 0, errors.New("store: entry must carry an envelope of sequence 0")
		}
		if e.ActorID == "" {
			return 0, errors.New("store: entry actor id must not be empty")
		}
		if e.Cause < -1 || e.Cause >= len(envs) {
			return 0, fmt.Errorf("store: entry cause %d is outside its batch of %d", e.Cause, len(envs))
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	first, err := s.appendLocked(envs, entries)
	if err != nil {
		resetBatchSequences(envs, entries)
		return 0, err
	}
	return first, nil
}

func (s *Store) appendLocked(envs []*vttv1.Envelope, entries []Entry) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	var next int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM events`).Scan(&next); err != nil {
		return 0, err
	}
	first := next
	for _, env := range envs {
		env.Sequence = next
		blob, err := proto.Marshal(env)
		if err != nil {
			return 0, fmt.Errorf("store: marshal: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO events (seq, event_id, session_id, occurred_at, payload) VALUES (?, ?, ?, ?, ?)`,
			next, env.EventId, env.SessionId, occurredAt(env), blob,
		); err != nil {
			return 0, fmt.Errorf("store: insert: %w", err)
		}
		next++
	}
	heads := map[string]int64{}
	for _, e := range entries {
		seq, ok := heads[e.ActorID]
		if !ok {
			if err := tx.QueryRow(
				`SELECT COALESCE(MAX(seq), 0) + 1 FROM character_events WHERE actor_id = ?`, e.ActorID,
			).Scan(&seq); err != nil {
				return 0, err
			}
		}
		e.Env.Sequence = seq
		blob, err := proto.Marshal(e.Env)
		if err != nil {
			return 0, fmt.Errorf("store: marshal entry: %w", err)
		}
		var worldSeq int64
		if e.Cause >= 0 {
			// Count from first, never read envs back: an entry may share its envelope.
			worldSeq = first + int64(e.Cause)
		}
		if _, err := tx.Exec(
			`INSERT INTO character_events (actor_id, seq, world_seq, event_id, occurred_at, payload) VALUES (?, ?, ?, ?, ?, ?)`,
			e.ActorID, seq, worldSeq, e.Env.EventId, occurredAt(e.Env), blob,
		); err != nil {
			return 0, fmt.Errorf("store: insert entry: %w", err)
		}
		heads[e.ActorID] = seq + 1
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return first, nil
}

func occurredAt(env *vttv1.Envelope) string {
	if ts := env.OccurredAt; ts != nil {
		return ts.AsTime().UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return ""
}

func resetBatchSequences(envs []*vttv1.Envelope, entries []Entry) {
	for _, env := range envs {
		env.Sequence = 0
	}
	for _, e := range entries {
		e.Env.Sequence = 0
	}
}

// Notify fans env out to subscribers. Call it only AFTER the event's effects
// are observable (campaign: after live apply), so a subscriber that sees event
// N can always read state >= N. It ignores an envelope of Sequence 0, which no
// append assigns, so one never persisted reaches nobody.
func (s *Store) Notify(env *vttv1.Envelope) {
	if env.Sequence == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifyLocked(env)
}

// ReadAfter returns all events with seq > afterSeq, in order.
func (s *Store) ReadAfter(afterSeq int64) ([]*vttv1.Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readAfterLocked(afterSeq)
}

func (s *Store) readAfterLocked(afterSeq int64) ([]*vttv1.Envelope, error) {
	rows, err := s.db.Query(`SELECT payload FROM events WHERE seq > ? ORDER BY seq`, afterSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*vttv1.Envelope
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		env := &vttv1.Envelope{}
		if err := proto.Unmarshal(blob, env); err != nil {
			return nil, fmt.Errorf("store: corrupt event payload: %w", err)
		}
		out = append(out, env)
	}
	return out, rows.Err()
}

// ReadCharacterAfter returns actorID's character log after afterSeq, in its
// own order; an actor with no entries reads empty.
func (s *Store) ReadCharacterAfter(actorID string, afterSeq int64) ([]CharacterEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	logs, err := s.readCharactersLocked(
		`SELECT actor_id, world_seq, payload FROM character_events WHERE actor_id = ? AND seq > ? ORDER BY seq`,
		actorID, afterSeq)
	if err != nil {
		return nil, err
	}
	return logs[actorID], nil
}

// ReadCharacterLogs returns every character log, keyed by actor id.
func (s *Store) ReadCharacterLogs() (map[string][]CharacterEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readCharactersLocked(`SELECT actor_id, world_seq, payload FROM character_events ORDER BY actor_id, seq`)
}

func (s *Store) readCharactersLocked(query string, args ...any) (map[string][]CharacterEntry, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]CharacterEntry{}
	for rows.Next() {
		var actorID string
		var worldSeq int64
		var blob []byte
		if err := rows.Scan(&actorID, &worldSeq, &blob); err != nil {
			return nil, err
		}
		env := &vttv1.Envelope{}
		if err := proto.Unmarshal(blob, env); err != nil {
			return nil, fmt.Errorf("store: corrupt entry payload in %s's log: %w", actorID, err)
		}
		out[actorID] = append(out[actorID], CharacterEntry{WorldSeq: worldSeq, Env: env})
	}
	return out, rows.Err()
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		s.dropLocked(sub)
	}
	s.subs = nil
	return s.db.Close()
}
