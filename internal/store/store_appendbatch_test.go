package store_test

import (
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

// batchEnvs builds n fresh envelopes with distinct, unique event ids.
func batchEnvs(n int, prefix string) []*vttv1.Envelope {
	out := make([]*vttv1.Envelope, n)
	for i := 0; i < n; i++ {
		out[i] = newEnv(prefix + string(rune('a'+i)))
	}
	return out
}

// TestAppendBatchAssignsContiguousSequences covers the happy path (N>=2):
// sequences are contiguous starting at MAX(seq)+1, stamped into every
// envelope, and the returned firstSeq is the first of the run.
func TestAppendBatchAssignsContiguousSequences(t *testing.T) {
	s := openTemp(t)

	// A prior one-envelope batch occupies seq 1, so this batch must start at 2.
	if _, err := appendOne(s, newEnv("solo")); err != nil {
		t.Fatal(err)
	}

	envs := batchEnvs(3, "batch1-")
	first, err := s.AppendBatch(envs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first != 2 {
		t.Fatalf("firstSeq: got %d, want 2", first)
	}
	for i, env := range envs {
		want := int64(2 + i)
		if env.Sequence != want {
			t.Fatalf("envs[%d].Sequence: got %d, want %d", i, env.Sequence, want)
		}
	}

	all, err := s.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 {
		t.Fatalf("ReadAfter(0): got %d events, want 4 (1 solo + 3 batch)", len(all))
	}
}

// TestAppendBatchRejectsEmptyBatch covers the empty-batch posture: a
// zero-length batch is a caller bug and must be rejected with a clean
// error rather than silently succeeding with nothing persisted.
func TestAppendBatchRejectsEmptyBatch(t *testing.T) {
	s := openTemp(t)
	if _, err := s.AppendBatch(nil, nil); err == nil {
		t.Fatal("want error for empty (nil) batch")
	}
	if _, err := s.AppendBatch([]*vttv1.Envelope{}, nil); err == nil {
		t.Fatal("want error for empty (zero-length) batch")
	}
}

// TestAppendBatchRejectsNonZeroSequence: one envelope with a caller-supplied
// sequence refuses the batch, and nothing in it is persisted.
func TestAppendBatchRejectsNonZeroSequence(t *testing.T) {
	s := openTemp(t)
	envs := batchEnvs(3, "bad-seq-")
	envs[1].Sequence = 9
	if _, err := s.AppendBatch(envs, nil); err == nil {
		t.Fatal("want error for non-zero sequence on a batch member")
	}
	all, err := s.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("ReadAfter(0) after rejected batch: got %d events, want 0", len(all))
	}
}

// TestAppendBatchRejectsEmptyEventID applies the event_id guard across the
// whole batch.
func TestAppendBatchRejectsEmptyEventID(t *testing.T) {
	s := openTemp(t)
	envs := batchEnvs(3, "empty-id-")
	envs[2].EventId = ""
	if _, err := s.AppendBatch(envs, nil); err == nil {
		t.Fatal("want error for empty event_id on a batch member")
	}
}

// TestAppendBatchDuplicateEventIDRollsBackAndResetsSequences: a third
// envelope's duplicate event_id fails the transaction after two inserts. The
// whole batch rolls back, every Sequence is reset to 0, and the store stays
// usable.
func TestAppendBatchDuplicateEventIDRollsBackAndResetsSequences(t *testing.T) {
	s := openTemp(t)
	if _, err := appendOne(s, newEnv("existing")); err != nil {
		t.Fatal(err)
	}

	envs := batchEnvs(2, "dup-")
	envs = append(envs, newEnv("existing")) // collides with the row above

	if _, err := s.AppendBatch(envs, nil); err == nil {
		t.Fatal("want error for a batch containing a duplicate event_id")
	}
	for i, env := range envs {
		if env.Sequence != 0 {
			t.Fatalf("envs[%d].Sequence after rolled-back batch: got %d, want 0 (reset on every failure path)", i, env.Sequence)
		}
	}

	all, err := s.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("ReadAfter(0) after rolled-back batch: got %d events, want 1 (only the pre-existing row)", len(all))
	}

	// Store remains usable: a normal append still works.
	if _, err := appendOne(s, newEnv("after-rollback")); err != nil {
		t.Fatalf("append after rolled-back batch: %v", err)
	}
}

// TestAppendBatchPersistsAcrossReopen proves the batch's rows survive a
// close/reopen with the same contiguous sequences, as
// TestReadAfterAndPersistenceAcrossReopen does for single appends.
func TestAppendBatchPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	envs := batchEnvs(3, "reopen-")
	first, err := s.AppendBatch(envs, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	s2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	got, err := s2.ReadAfter(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("ReadAfter(0) after reopen: got %d events, want 3", len(got))
	}
	for i, env := range got {
		want := first + int64(i)
		if env.Sequence != want {
			t.Fatalf("reopened event[%d].Sequence: got %d, want %d", i, env.Sequence, want)
		}
	}
}

// TestAppendBatchThenNotifyDeliversContiguousToSubscriber mirrors the
// pattern a caller (campaign.AppendBatch) uses: persist via AppendBatch,
// then Notify each envelope in order. A subscriber caught up before the
// batch must see exactly the batch's events, in sequence order, with no
// gaps.
func TestAppendBatchThenNotifyDeliversContiguousToSubscriber(t *testing.T) {
	s := openTemp(t)

	ch, unsubscribe, _, err := s.Subscribe(0, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	envs := batchEnvs(3, "notify-")
	if _, err := s.AppendBatch(envs, nil); err != nil {
		t.Fatal(err)
	}
	for _, env := range envs {
		s.Notify(env)
	}

	for i, want := range envs {
		got := recv(t, ch)
		if got.EventId != want.EventId {
			t.Fatalf("delivery[%d]: got event id %s, want %s", i, got.EventId, want.EventId)
		}
		if got.Sequence != want.Sequence {
			t.Fatalf("delivery[%d]: got sequence %d, want %d", i, got.Sequence, want.Sequence)
		}
	}
}

// TestAppendBatchDoesNotNotify mirrors TestAppendDoesNotNotify: AppendBatch
// persists but does not notify — a subscriber established before the call
// must see nothing until the caller explicitly calls Notify.
func TestAppendBatchDoesNotNotify(t *testing.T) {
	s := openTemp(t)
	ch, unsubscribe, _, err := s.Subscribe(0, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()

	envs := batchEnvs(2, "silent-")
	if _, err := s.AppendBatch(envs, nil); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-ch:
		t.Fatalf("want no delivery from AppendBatch alone, got %v", got)
	default:
	}
}
