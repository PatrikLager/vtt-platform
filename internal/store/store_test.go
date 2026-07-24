package store_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func newEnv(id string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:   id,
		SessionId: "sess-1",
		ActorRole: "dm",
		Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "test"},
		},
	}
}

func openTemp(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "campaign.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAppendAssignsSequentialSequence(t *testing.T) {
	s := openTemp(t)
	for want := int64(1); want <= 3; want++ {
		env := newEnv(string(rune('a' + want)))
		got, err := s.Append(env)
		if err != nil {
			t.Fatal(err)
		}
		if got != want || env.Sequence != want {
			t.Fatalf("seq: got %d (env %d), want %d", got, env.Sequence, want)
		}
	}
}

func TestAppendRejectsNonZeroSequence(t *testing.T) {
	s := openTemp(t)
	env := newEnv("x")
	env.Sequence = 9
	if _, err := s.Append(env); err == nil {
		t.Fatal("want error for non-zero sequence")
	}
}

func TestAppendRejectsEmptyAndDuplicateEventID(t *testing.T) {
	s := openTemp(t)
	if _, err := s.Append(newEnv("")); err == nil {
		t.Fatal("want error for empty event_id")
	}
	if _, err := s.Append(newEnv("dup")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(newEnv("dup")); err == nil {
		t.Fatal("want error for duplicate event_id")
	}
}

// TestAppendPersistsOccurredAt pins the occurred_at column round-trip:
// present when the envelope carries an OccurredAt timestamp, empty string
// when it doesn't. The column is write-only through the public API (Store
// never selects it back), so the on-disk row is the only place this
// boundary is observable.
func TestAppendPersistsOccurredAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	withTime := newEnv("with-ts")
	withTime.OccurredAt = timestamppb.New(time.Date(2026, 7, 24, 10, 30, 0, 0, time.UTC))
	if _, err := s.Append(withTime); err != nil {
		t.Fatal(err)
	}

	withoutTime := newEnv("without-ts")
	if _, err := s.Append(withoutTime); err != nil {
		t.Fatal(err)
	}
	s.Close()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var gotWith, gotWithout string
	if err := db.QueryRow(`SELECT occurred_at FROM events WHERE event_id = ?`, "with-ts").Scan(&gotWith); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT occurred_at FROM events WHERE event_id = ?`, "without-ts").Scan(&gotWithout); err != nil {
		t.Fatal(err)
	}

	if want := "2026-07-24T10:30:00Z"; gotWith != want {
		t.Fatalf("occurred_at for envelope WITH OccurredAt set: got %q, want %q", gotWith, want)
	}
	if gotWithout != "" {
		t.Fatalf("occurred_at for envelope with nil OccurredAt: got %q, want empty string", gotWithout)
	}
}

func TestReadAfterAndPersistenceAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first := newEnv("e1")
	if _, err := s.Append(first); err != nil {
		t.Fatal(err)
	}
	second := newEnv("e2")
	if _, err := s.Append(second); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, err := s2.ReadAfter(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !proto.Equal(got[0], second) {
		t.Fatalf("ReadAfter(1): got %v", got)
	}
	all, _ := s2.ReadAfter(0)
	if len(all) != 2 {
		t.Fatalf("ReadAfter(0): got %d events", len(all))
	}
}
