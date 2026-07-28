package store_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// This file pins the store's FAILURE behavior: an unusable file, a corrupted
// payload row, and a handle closed underneath a caller.
//
// The append-only log is the substrate every other guarantee rests on — the
// fold, retraction, replay, and the rebuild==live property all assume the
// store either returns honest events or an error. A corrupt row silently
// yielding a zero-valued Envelope would be worse than any of them: it would
// fold cleanly into a wrong state with nothing to notice.
//
// The all-or-nothing rollback invariants are already pinned elsewhere
// (TestAppendRejectsEmptyAndDuplicateEventID, and
// TestAppendBatchDuplicateEventIDRollsBackAndResetsSequences for the
// half-stamped-batch case). This file covers what those leave dark.

// openTempWithPath mirrors openTemp but also returns the file path, needed by
// the tamper test to open an independent handle on the same database.
func openTempWithPath(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestOpenRejectsFileThatIsNotADatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-db")
	if err := os.WriteFile(path, []byte("this is not a SQLite file"), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err == nil {
		s.Close()
		t.Fatal("want error opening a non-database file")
	}
	if s != nil {
		t.Errorf("want nil Store on error, got %+v", s)
	}
}

// TestReadAfterFailsLoudlyOnCorruptPayload is the important one: a payload
// blob that is not a valid Envelope must surface an error, never a
// zero-valued Envelope. proto.Unmarshal is permissive about many malformed
// inputs, so this uses bytes that cannot parse as a message.
func TestReadAfterFailsLoudlyOnCorruptPayload(t *testing.T) {
	s, path := openTempWithPath(t)
	if _, err := s.Append(newEnv("evt-1")); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	// Field number 0 is illegal in the protobuf wire format, so this cannot
	// be mistaken for a valid (or empty) message.
	if _, err := raw.Exec(`UPDATE events SET payload = ? WHERE seq = 1`, []byte{0x00, 0xFF, 0xFF}); err != nil {
		t.Fatal(err)
	}

	got, err := s.ReadAfter(0)
	if err == nil {
		t.Fatalf("want error for corrupt payload, got %d events", len(got))
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("error should name the corruption, got: %v", err)
	}
	if got != nil {
		t.Errorf("want nil events on error, got %+v", got)
	}
}

// TestAppendBatchPersistsOccurredAt is the batch counterpart to
// TestAppendPersistsOccurredAt. Both timestamp branches are exercised in ONE
// batch — a set OccurredAt must round-trip, a nil one must persist as the
// empty string, never as the Unix epoch.
//
// PROVENANCE: gremlins found the `ts != nil` guard at store.go:150 surviving
// inside AppendBatch while its Append twin (store.go:79) was killed — the
// single-envelope path was tested and the batch path assumed. Same shape as
// the at-cap gaps in internal/engine: a variant covered, its sibling not.
func TestAppendBatchPersistsOccurredAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	withTime := newEnv("batch-with-ts")
	withTime.OccurredAt = timestamppb.New(time.Date(2026, 7, 24, 10, 30, 0, 0, time.UTC))
	withoutTime := newEnv("batch-without-ts")

	if _, err := s.AppendBatch([]*vttv1.Envelope{withTime, withoutTime}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var gotWith, gotWithout string
	if err := db.QueryRow(`SELECT occurred_at FROM events WHERE event_id = ?`, "batch-with-ts").Scan(&gotWith); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT occurred_at FROM events WHERE event_id = ?`, "batch-without-ts").Scan(&gotWithout); err != nil {
		t.Fatal(err)
	}

	if want := "2026-07-24T10:30:00Z"; gotWith != want {
		t.Fatalf("occurred_at WITH OccurredAt set: got %q, want %q", gotWith, want)
	}
	if gotWithout != "" {
		t.Fatalf("occurred_at with nil OccurredAt: got %q, want empty string", gotWithout)
	}
}

// TestOperationsFailAfterClose pins that every exported operation surfaces an
// error rather than panicking once the handle is closed. This is the shape a
// caller hits during the ledgered shutdown race: composeServer's closeFn can
// close the campaign while a hijacked WebSocket connection is still live
// (cmd/vtt/serve_compose.go:31-54). That race is not fixed here, but its
// blast radius is pinned — errors, not panics.
func TestOperationsFailAfterClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Append(newEnv("evt-1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	t.Run("Append", func(t *testing.T) {
		if _, err := s.Append(newEnv("evt-2")); err == nil {
			t.Error("want error after Close")
		}
	})
	t.Run("AppendBatch", func(t *testing.T) {
		if _, err := s.AppendBatch([]*vttv1.Envelope{newEnv("evt-3")}); err == nil {
			t.Error("want error after Close")
		}
	})
	t.Run("ReadAfter", func(t *testing.T) {
		if _, err := s.ReadAfter(0); err == nil {
			t.Error("want error after Close")
		}
	})
}
