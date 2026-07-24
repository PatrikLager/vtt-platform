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

const schema = `
CREATE TABLE IF NOT EXISTS events (
  seq         INTEGER PRIMARY KEY,
  event_id    TEXT NOT NULL UNIQUE,
  session_id  TEXT NOT NULL,
  occurred_at TEXT NOT NULL,
  payload     BLOB NOT NULL
);`

type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	subs []*subscriber
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: init schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Append stamps the next sequence into env, persists it, and returns the
// sequence. Callers must submit sequence=0; event_id must be non-empty and
// unique (idempotency guard).
func (s *Store) Append(env *vttv1.Envelope) (int64, error) {
	if env.Sequence != 0 {
		return 0, errors.New("store: envelope sequence must be 0 on append")
	}
	if env.EventId == "" {
		return 0, errors.New("store: envelope event_id must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var next int64
	if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) + 1 FROM events`).Scan(&next); err != nil {
		return 0, err
	}
	env.Sequence = next
	blob, err := proto.Marshal(env)
	if err != nil {
		env.Sequence = 0
		return 0, fmt.Errorf("store: marshal: %w", err)
	}
	occurredAt := ""
	if ts := env.OccurredAt; ts != nil {
		occurredAt = ts.AsTime().UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	if _, err := tx.Exec(
		`INSERT INTO events (seq, event_id, session_id, occurred_at, payload) VALUES (?, ?, ?, ?, ?)`,
		next, env.EventId, env.SessionId, occurredAt, blob,
	); err != nil {
		env.Sequence = 0
		return 0, fmt.Errorf("store: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		env.Sequence = 0
		return 0, err
	}
	return next, nil
}

// Notify fans env out to subscribers. Callers invoke it AFTER the event's
// effects are observable (campaign: after live apply) so a subscriber that
// sees event N can always read state >= N. Idempotent per subscriber via
// sequence dedupe, which also closes the subscribe-between-persist-and-notify
// race.
//
// Envelopes with Sequence == 0 are silently ignored (no-op, no error — the
// signature stays stable): 0 is never a sequence Append assigns
// (COALESCE(MAX(seq),0)+1 starts at 1), so a zero-sequence envelope reaching
// Notify means a caller invoked it without the event ever having been
// persisted. Public-method hardening against that misuse; not a path any
// current caller exercises.
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

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		s.dropLocked(sub)
	}
	s.subs = nil
	return s.db.Close()
}
