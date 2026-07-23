package store_test

import (
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

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
