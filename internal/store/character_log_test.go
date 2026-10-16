package store_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

func entryEnv(id string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: id, Payload: &vttv1.Envelope_NarrationAdded{
		NarrationAdded: &vttv1.NarrationAdded{Text: id},
	}}
}

func readLog(t *testing.T, s *store.Store, actorID string, after int64) []store.CharacterEntry {
	t.Helper()
	got, err := s.ReadCharacterAfter(actorID, after)
	if err != nil {
		t.Fatalf("ReadCharacterAfter(%q, %d): %v", actorID, after, err)
	}
	return got
}

// want is the sequence, world sequence and text of each entry of one log.
type want struct {
	seq, world int64
	text       string
}

func assertLog(t *testing.T, name string, got []store.CharacterEntry, ws []want) {
	t.Helper()
	if len(got) != len(ws) {
		t.Fatalf("%s: %d entries, want %d: %+v", name, len(got), len(ws), got)
	}
	for i, w := range ws {
		e := got[i]
		if e.Env.GetSequence() != w.seq || e.WorldSeq != w.world || e.Env.GetNarrationAdded().GetText() != w.text {
			t.Errorf("%s[%d] = seq %d, world %d, %q; want seq %d, world %d, %q", name, i,
				e.Env.GetSequence(), e.WorldSeq, e.Env.GetNarrationAdded().GetText(), w.seq, w.world, w.text)
		}
	}
}

// VTT-321
func TestACharacterLogNumbersItsEntriesFromOne(t *testing.T) {
	s, path := openTempWithPath(t)
	// The world's first two sequences go to events that reach no log, so a
	// log numbered from the world's sequence would start at 3.
	if _, err := s.AppendBatch(batchEnvs(2, "before"), nil); err != nil {
		t.Fatal(err)
	}
	w := batchEnvs(2, "w")
	a1, b1, a2, a0 := entryEnv("a1"), entryEnv("b1"), entryEnv("a2"), entryEnv("a0")
	first, err := s.AppendBatch(w, []store.Entry{
		{ActorID: "a", Cause: -1, Env: a0},
		{ActorID: "a", Cause: 0, Env: a1},
		{ActorID: "b", Cause: 1, Env: b1},
		{ActorID: "a", Cause: 1, Env: a2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first != 3 {
		t.Fatalf("first world sequence = %d, want 3", first)
	}
	for e, seq := range map[*vttv1.Envelope]int64{a0: 1, a1: 2, a2: 3, b1: 1} {
		if e.Sequence != seq {
			t.Errorf("entry %s was stamped %d, want %d", e.EventId, e.Sequence, seq)
		}
	}
	assertLog(t, "a", readLog(t, s, "a", 0), []want{{1, 0, "a0"}, {2, 3, "a1"}, {3, 4, "a2"}})
	assertLog(t, "b", readLog(t, s, "b", 0), []want{{1, 4, "b1"}})
	assertLog(t, "a after 1", readLog(t, s, "a", 1), []want{{2, 3, "a1"}, {3, 4, "a2"}})
	if got := readLog(t, s, "nobody", 0); len(got) != 0 {
		t.Errorf("an actor with no entries read %d", len(got))
	}

	// A failed batch leaves no gap in either log.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`CREATE TRIGGER refuse_last BEFORE INSERT ON character_events
		WHEN NEW.actor_id = 'z' BEGIN SELECT RAISE(ABORT, 'refused'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendBatch(batchEnvs(1, "failed"), []store.Entry{
		{ActorID: "a", Cause: 0, Env: entryEnv("lost-a")},
		{ActorID: "b", Cause: 0, Env: entryEnv("lost-b")},
		{ActorID: "z", Cause: 0, Env: entryEnv("lost-z")},
	}); err == nil {
		t.Fatal("the trigger did not refuse the batch")
	}
	if _, err := raw.Exec(`DROP TRIGGER refuse_last`); err != nil {
		t.Fatal(err)
	}

	// A reopened store continues each log where it stopped, in the order
	// given: an entry of no cause after a caused one, and a later cause first.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s2.Close() })
	if _, err := s2.AppendBatch(batchEnvs(2, "later"), []store.Entry{
		{ActorID: "b", Cause: 1, Env: entryEnv("b2")},
		{ActorID: "a", Cause: 0, Env: entryEnv("a3")},
		{ActorID: "b", Cause: 0, Env: entryEnv("b3")},
		{ActorID: "a", Cause: -1, Env: entryEnv("a4")},
	}); err != nil {
		t.Fatal(err)
	}
	logs, err := s2.ReadCharacterLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("ReadCharacterLogs holds %d logs, want 2", len(logs))
	}
	assertLog(t, "a reopened", logs["a"], []want{{1, 0, "a0"}, {2, 3, "a1"}, {3, 4, "a2"}, {4, 5, "a3"}, {5, 0, "a4"}})
	assertLog(t, "b reopened", logs["b"], []want{{1, 4, "b1"}, {2, 6, "b2"}, {3, 5, "b3"}})
	assertLog(t, "a after 3", readLog(t, s2, "a", 3), []want{{4, 5, "a3"}, {5, 0, "a4"}})
}

func TestAnEntryKeepsTheIdAndTimeItWasGiven(t *testing.T) {
	s, path := openTempWithPath(t)
	w := batchEnvs(1, "w")
	w[0].OccurredAt = timestamppb.New(time.Date(2026, 10, 16, 9, 30, 0, 0, time.UTC))
	// A forwarded entry carries the world event's id, in every log it reaches;
	// an entry the decision makes carries none, more than once in one log.
	forwarded := func() *vttv1.Envelope {
		e := proto.Clone(w[0]).(*vttv1.Envelope)
		e.Sequence = 0
		return e
	}
	if _, err := s.AppendBatch(w, []store.Entry{
		{ActorID: "a", Cause: 0, Env: forwarded()},
		{ActorID: "b", Cause: 0, Env: forwarded()},
		{ActorID: "a", Cause: 0, Env: entryEnv("")},
		{ActorID: "a", Cause: 0, Env: entryEnv("")},
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	rows, err := raw.Query(`SELECT actor_id, seq, event_id, occurred_at FROM character_events ORDER BY actor_id, seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var actor, id, at string
		var seq int64
		if err := rows.Scan(&actor, &seq, &id, &at); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %d %q %q", actor, seq, id, at))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantRows := []string{
		`a 1 "wa" "2026-10-16T09:30:00Z"`,
		`a 2 "" ""`,
		`a 3 "" ""`,
		`b 1 "wa" "2026-10-16T09:30:00Z"`,
	}
	if strings.Join(got, "\n") != strings.Join(wantRows, "\n") {
		t.Fatalf("character_events rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantRows, "\n"))
	}
}

// VTT-315
func TestAFailedCharacterWriteLeavesNoWorldEvent(t *testing.T) {
	s, path := openTempWithPath(t)
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	// Fail the second entry, after both world rows and the first entry are
	// written in the same transaction.
	if _, err := raw.Exec(`CREATE TRIGGER refuse_b BEFORE INSERT ON character_events
		WHEN NEW.actor_id = 'b' BEGIN SELECT RAISE(ABORT, 'refused by the test'); END`); err != nil {
		t.Fatal(err)
	}
	w := batchEnvs(2, "w")
	entries := []store.Entry{
		{ActorID: "a", Cause: 0, Env: entryEnv("a1")},
		{ActorID: "b", Cause: 1, Env: entryEnv("b1")},
	}
	if _, err := s.AppendBatch(w, entries); err == nil || !strings.Contains(err.Error(), "refused by the test") {
		t.Fatalf("AppendBatch error = %v, want the trigger's refusal", err)
	}
	if got, err := s.ReadAfter(0); err != nil || len(got) != 0 {
		t.Fatalf("world log after a failed entry: %d events, err %v; want none", len(got), err)
	}
	if logs, err := s.ReadCharacterLogs(); err != nil || len(logs) != 0 {
		t.Fatalf("character logs after a failed entry: %v, err %v; want none", logs, err)
	}
	for _, env := range w {
		if env.Sequence != 0 {
			t.Errorf("world envelope %s kept sequence %d", env.EventId, env.Sequence)
		}
	}
	for _, e := range entries {
		if e.Env.Sequence != 0 {
			t.Errorf("entry %s kept sequence %d", e.Env.EventId, e.Env.Sequence)
		}
	}

	if _, err := raw.Exec(`DROP TRIGGER refuse_b`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AppendBatch(w, entries); err != nil {
		t.Fatalf("the same batch after the trigger is gone: %v", err)
	}
	assertLog(t, "a", readLog(t, s, "a", 0), []want{{1, 1, "a1"}})
	assertLog(t, "b", readLog(t, s, "b", 0), []want{{1, 2, "b1"}})
}

func TestAnEntryTheStoreCannotWriteIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	stamped := entryEnv("stamped")
	stamped.Sequence = 7
	for _, tc := range []struct {
		name  string
		entry store.Entry
		want  string
	}{
		{"no envelope", store.Entry{ActorID: "a", Cause: 0}, "envelope of sequence 0"},
		{"a stamped envelope", store.Entry{ActorID: "a", Cause: 0, Env: stamped}, "envelope of sequence 0"},
		{"no actor", store.Entry{Cause: 0, Env: entryEnv("x")}, "actor id must not be empty"},
		{"a cause before the batch", store.Entry{ActorID: "a", Cause: -2, Env: entryEnv("x")}, "outside its batch"},
		{"a cause after the batch", store.Entry{ActorID: "a", Cause: 1, Env: entryEnv("x")}, "outside its batch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := openTemp(t)
			w := batchEnvs(1, "w")
			if _, err := s.AppendBatch(w, []store.Entry{tc.entry}); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("AppendBatch error = %v, want %q", err, tc.want)
			}
			if w[0].Sequence != 0 {
				t.Errorf("the world envelope was stamped %d", w[0].Sequence)
			}
			if got, _ := s.ReadAfter(0); len(got) != 0 {
				t.Errorf("the world log holds %d events", len(got))
			}
		})
	}
}

func TestACorruptEntryFailsItsRead(t *testing.T) {
	s, path := openTempWithPath(t)
	if _, err := s.AppendBatch(batchEnvs(1, "w"), []store.Entry{{ActorID: "a", Cause: 0, Env: entryEnv("a1")}}); err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	// Field number 0 is illegal in the protobuf wire format.
	if _, err := raw.Exec(`UPDATE character_events SET payload = ?`, []byte{0x00, 0xFF, 0xFF}); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ReadCharacterAfter("a", 0); err == nil || !strings.Contains(err.Error(), "corrupt entry payload in a's log") {
		t.Fatalf("ReadCharacterAfter = %d entries, err %v; want the corruption named", len(got), err)
	}
	if _, err := s.ReadCharacterLogs(); err == nil {
		t.Fatal("ReadCharacterLogs read a corrupt entry without error")
	}
}
