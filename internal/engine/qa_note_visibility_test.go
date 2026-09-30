package engine_test

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const (
	qaNVNone   = vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED
	qaNVPublic = vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC
	qaNVSecret = vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET
)

func qaNVUpsert(seq int64, key string, vis vttv1.NoteVisibility) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_NoteUpserted{
		NoteUpserted: &vttv1.NoteUpserted{Key: key, Title: "title " + key, Text: "text of " + key, Visibility: vis},
	}}
}

func qaNVDelete(seq int64, key string) *vttv1.Envelope {
	return &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_NoteDeleted{
		NoteDeleted: &vttv1.NoteDeleted{Key: key},
	}}
}

func qaNVFold(t *testing.T, envs ...*vttv1.Envelope) *engine.State {
	t.Helper()
	st := engine.NewState()
	for _, env := range envs {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("Apply(seq %d): %v", env.GetSequence(), err)
		}
	}
	return st
}

// qaNVOf fails the test when the note is absent, so a zero
// visibility read off a missing note can never pass as "none recorded".
func qaNVOf(t *testing.T, st *engine.State, key string) vttv1.NoteVisibility {
	t.Helper()
	n, ok := st.Notes[key]
	if !ok {
		t.Fatalf("note %q absent from the folded state; notes: %+v", key, st.Notes)
	}
	return n.Visibility
}

// VTT-228
func TestQANoteRecordsThePublicVisibilityItsUpsertStated(t *testing.T) {
	st := qaNVFold(t, qaNVUpsert(1, "k", qaNVPublic))
	if got := qaNVOf(t, st, "k"); got != qaNVPublic {
		t.Fatalf("visibility = %v, want %v", got, qaNVPublic)
	}
}

// VTT-228
func TestQANoteRecordsTheSecretVisibilityItsUpsertStated(t *testing.T) {
	st := qaNVFold(t, qaNVUpsert(1, "k", qaNVSecret))
	if got := qaNVOf(t, st, "k"); got != qaNVSecret {
		t.Fatalf("visibility = %v, want %v", got, qaNVSecret)
	}
}

// VTT-228
func TestQANoteRecordsNoVisibilityWhenItsUpsertStatedNone(t *testing.T) {
	st := qaNVFold(t, qaNVUpsert(1, "k", qaNVNone))
	n, ok := st.Notes["k"]
	if !ok {
		t.Fatalf("note absent; notes: %+v", st.Notes)
	}
	if n.Title != "title k" || n.Text != "text of k" {
		t.Fatalf("note not recorded from its upsert: %+v", n)
	}
	if n.Visibility != qaNVNone {
		t.Fatalf("visibility = %v, want %v", n.Visibility, qaNVNone)
	}
}

// VTT-228
func TestQANoteLaterUpsertReplacesTheVisibilityWithTheOneItStated(t *testing.T) {
	cases := []struct {
		name          string
		first, second vttv1.NoteVisibility
	}{
		{"public then secret", qaNVPublic, qaNVSecret},
		{"secret then public", qaNVSecret, qaNVPublic},
		{"none then public", qaNVNone, qaNVPublic},
		{"none then secret", qaNVNone, qaNVSecret},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := qaNVFold(t, qaNVUpsert(1, "k", c.first), qaNVUpsert(2, "k", c.second))
			if got := qaNVOf(t, st, "k"); got != c.second {
				t.Fatalf("visibility = %v, want the latest upsert's %v", got, c.second)
			}
		})
	}
}

// VTT-228
func TestQANoteLaterUpsertStatingNoneLeavesTheNoteWithNoVisibility(t *testing.T) {
	for _, first := range []vttv1.NoteVisibility{qaNVPublic, qaNVSecret} {
		t.Run(first.String()+" then none", func(t *testing.T) {
			st := qaNVFold(t, qaNVUpsert(1, "k", first), qaNVUpsert(2, "k", qaNVNone))
			if got := qaNVOf(t, st, "k"); got != qaNVNone {
				t.Fatalf("visibility = %v, want none: the latest upsert stated none", got)
			}
		})
	}
}

// VTT-228
func TestQANoteReupsertedAfterDeletionCarriesOnlyTheReupsertsVisibility(t *testing.T) {
	cases := []struct {
		name          string
		before, after vttv1.NoteVisibility
	}{
		{"public, deleted, re-upserted with none", qaNVPublic, qaNVNone},
		{"secret, deleted, re-upserted with none", qaNVSecret, qaNVNone},
		{"secret, deleted, re-upserted public", qaNVSecret, qaNVPublic},
		{"public, deleted, re-upserted secret", qaNVPublic, qaNVSecret},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st := qaNVFold(t, qaNVUpsert(1, "k", c.before), qaNVDelete(2, "k"))
			if _, ok := st.Notes["k"]; ok {
				t.Fatalf("precondition: note still present after NoteDeleted")
			}
			if err := engine.Apply(st, qaNVUpsert(3, "k", c.after)); err != nil {
				t.Fatalf("re-upsert: %v", err)
			}
			if got := qaNVOf(t, st, "k"); got != c.after {
				t.Fatalf("visibility = %v, want the re-upsert's %v", got, c.after)
			}
		})
	}
}

// VTT-228
func TestQANoteVisibilityIsHeldPerNote(t *testing.T) {
	st := qaNVFold(t,
		qaNVUpsert(1, "a", qaNVPublic),
		qaNVUpsert(2, "b", qaNVSecret),
		qaNVUpsert(3, "a", qaNVNone),
	)
	if got := qaNVOf(t, st, "a"); got != qaNVNone {
		t.Fatalf("a = %v, want none", got)
	}
	if got := qaNVOf(t, st, "b"); got != qaNVSecret {
		t.Fatalf("b = %v, want secret: a's upsert must not touch b", got)
	}
	if err := engine.Apply(st, qaNVUpsert(4, "b", qaNVPublic)); err != nil {
		t.Fatal(err)
	}
	if got := qaNVOf(t, st, "a"); got != qaNVNone {
		t.Fatalf("a = %v, want none: b's upsert must not touch a", got)
	}
	if got := qaNVOf(t, st, "b"); got != qaNVPublic {
		t.Fatalf("b = %v, want public", got)
	}
}

// VTT-228
func TestQANoteSnapshotOfTheFoldedStateCarriesEachVisibility(t *testing.T) {
	st := qaNVFold(t,
		qaNVUpsert(1, "p", qaNVPublic),
		qaNVUpsert(2, "s", qaNVSecret),
		qaNVUpsert(3, "n", qaNVNone),
	)
	snap := st.Snapshot()
	want := map[string]vttv1.NoteVisibility{"p": qaNVPublic, "s": qaNVSecret, "n": qaNVNone}
	for key, w := range want {
		if got := qaNVOf(t, snap, key); got != w {
			t.Fatalf("snapshot %q = %v, want %v", key, got, w)
		}
	}
}

// Keep this byte-identical to QA_PARITY_LOG in
// client/test/qa-note-visibility.test.ts, and qaNVParityWant equal to
// PARITY_WANT there.
const qaNVParityLog = `[
 {"sequence":"1","noteUpserted":{"key":"a","title":"A","text":"a1","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"2","noteUpserted":{"key":"b","title":"B","text":"b1","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"3","noteUpserted":{"key":"c","title":"C","text":"c1"}},
 {"sequence":"4","noteUpserted":{"key":"a","title":"A","text":"a2"}},
 {"sequence":"5","noteUpserted":{"key":"b","title":"B","text":"b2","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"6","noteUpserted":{"key":"c","title":"C","text":"c2","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"7","noteUpserted":{"key":"d","title":"D","text":"d1","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"8","noteDeleted":{"key":"d"}},
 {"sequence":"9","noteUpserted":{"key":"d","title":"D","text":"d2","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"10","noteUpserted":{"key":"e","title":"E","text":"e1","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"11","noteDeleted":{"key":"e"}},
 {"sequence":"12","noteUpserted":{"key":"e","title":"E","text":"e2"}},
 {"sequence":"13","noteUpserted":{"key":"f","title":"F","text":"f1","visibility":"NOTE_VISIBILITY_SECRET"}},
 {"sequence":"14","noteUpserted":{"key":"f","title":"F","text":"f2"}},
 {"sequence":"15","noteUpserted":{"key":"g","title":"G","text":"g1","visibility":"NOTE_VISIBILITY_PUBLIC"}},
 {"sequence":"16","noteUpserted":{"key":"h","title":"H","text":"h1","visibility":"NOTE_VISIBILITY_SECRET"}}
]`

var qaNVParityWant = map[string]vttv1.NoteVisibility{
	"a": qaNVNone, "b": qaNVPublic, "c": qaNVSecret, "d": qaNVPublic,
	"e": qaNVNone, "f": qaNVNone, "g": qaNVPublic, "h": qaNVSecret,
}

func qaNVFoldParityLog(t *testing.T) *engine.State {
	t.Helper()
	var raws []json.RawMessage
	if err := json.Unmarshal([]byte(qaNVParityLog), &raws); err != nil {
		t.Fatal(err)
	}
	st := engine.NewState()
	for i, raw := range raws {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(raw, env); err != nil {
			t.Fatalf("envelope %d: %v", i, err)
		}
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("Apply envelope %d: %v", i, err)
		}
	}
	return st
}

// VTT-228
func TestQANoteParityLogFoldsToTheStatedVisibilities(t *testing.T) {
	st := qaNVFoldParityLog(t)
	if len(st.Notes) != len(qaNVParityWant) {
		t.Fatalf("notes = %+v, want exactly the keys of %v", st.Notes, qaNVParityWant)
	}
	for key, w := range qaNVParityWant {
		if got := qaNVOf(t, st, key); got != w {
			t.Fatalf("%q = %v, want %v", key, got, w)
		}
	}
}
