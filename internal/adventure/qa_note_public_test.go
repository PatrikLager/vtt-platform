package adventure_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaAdventuresRoot = "../../adventures"

const qaRulesetsRoot = "../../rulesets"

type qaFileNote struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

func qaRulesetOf(tb testing.TB, advDir string) *rules.Ruleset {
	tb.Helper()
	raw, err := os.ReadFile(filepath.Join(advDir, "adventure.json"))
	if err != nil {
		tb.Fatalf("read manifest: %v", err)
	}
	var m struct {
		Ruleset string `json:"ruleset"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		tb.Fatalf("decode manifest: %v", err)
	}
	rs, err := rules.Load(filepath.Join(qaRulesetsRoot, m.Ruleset))
	if err != nil {
		tb.Fatalf("load ruleset %q: %v", m.Ruleset, err)
	}
	return rs
}

func qaNotesOnDisk(tb testing.TB, advDir string) map[string]qaFileNote {
	tb.Helper()
	files, err := filepath.Glob(filepath.Join(advDir, "notes", "*.json"))
	if err != nil {
		tb.Fatalf("glob notes: %v", err)
	}
	out := map[string]qaFileNote{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			tb.Fatalf("read %s: %v", f, err)
		}
		var ns []qaFileNote
		if err := json.Unmarshal(raw, &ns); err != nil {
			tb.Fatalf("decode %s: %v", f, err)
		}
		for _, n := range ns {
			out[n.Key] = n
		}
	}
	return out
}

func qaCompiledNotes(tb testing.TB, advDir string, st *engine.State) []*vttv1.NoteUpserted {
	tb.Helper()
	adv, err := adventure.Load(advDir, qaRulesetOf(tb, advDir))
	if err != nil {
		tb.Fatalf("Load(%s): %v", advDir, err)
	}
	envs, _, err := adventure.Compile(adv, st)
	if err != nil {
		tb.Fatalf("Compile(%s): %v", advDir, err)
	}
	return qaNotesIn(envs)
}

func qaNotesIn(envs []*vttv1.Envelope) []*vttv1.NoteUpserted {
	var out []*vttv1.NoteUpserted
	for _, e := range envs {
		if n := e.GetNoteUpserted(); n != nil {
			out = append(out, n)
		}
	}
	return out
}

func qaAssertAllPublicAndVerbatim(tb testing.TB, got []*vttv1.NoteUpserted, want map[string]qaFileNote) {
	tb.Helper()
	if len(got) != len(want) {
		tb.Fatalf("compiled %d NoteUpserted, the notes files declare %d", len(got), len(want))
	}
	seen := map[string]bool{}
	for i, n := range got {
		if n.GetVisibility() != vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
			tb.Errorf("note %d (%q) compiled with visibility %v, want NOTE_VISIBILITY_PUBLIC", i, n.GetKey(), n.GetVisibility())
		}
		w, ok := want[n.GetKey()]
		if !ok {
			tb.Errorf("note %d key %q is in no notes file", i, n.GetKey())
			continue
		}
		if seen[n.GetKey()] {
			tb.Errorf("note %q compiled twice", n.GetKey())
		}
		seen[n.GetKey()] = true
		if n.GetTitle() != w.Title || n.GetText() != w.Text {
			tb.Errorf("note %q compiled as (%q, %q), file says (%q, %q)", n.GetKey(), n.GetTitle(), n.GetText(), w.Title, w.Text)
		}
	}
}

func qaShippedAdventures(tb testing.TB) []string {
	tb.Helper()
	dirs, err := filepath.Glob(filepath.Join(qaAdventuresRoot, "*", "adventure.json"))
	if err != nil || len(dirs) < 2 {
		tb.Fatalf("expected the shipped adventures under %s, got %v (%v)", qaAdventuresRoot, dirs, err)
	}
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, filepath.Dir(d))
	}
	return out
}

// VTT-240
func TestQANoteAdventureEveryNoteOfEveryShippedAdventureCompilesPublic(t *testing.T) {
	for _, dir := range qaShippedAdventures(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			want := qaNotesOnDisk(t, dir)
			if len(want) == 0 {
				t.Fatalf("%s ships no note; this test needs one", dir)
			}
			qaAssertAllPublicAndVerbatim(t, qaCompiledNotes(t, dir, engine.NewState()), want)
		})
	}
}

// VTT-240
func TestQANoteAdventureEachCommittedCompiledBatchGoldenShowsEveryNotePublic(t *testing.T) {
	for _, dir := range qaShippedAdventures(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, "goldens", "compiled-batch.json"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var entries []struct {
				Type         string `json:"type"`
				NoteUpserted *struct {
					Key        string `json:"key"`
					Visibility string `json:"visibility"`
				} `json:"note_upserted"`
			}
			if err := json.Unmarshal(raw, &entries); err != nil {
				t.Fatalf("decode golden: %v", err)
			}
			var keys []string
			for _, e := range entries {
				if e.Type != "note_upserted" {
					continue
				}
				if e.NoteUpserted == nil {
					t.Fatalf("a note_upserted entry carries no note_upserted object")
				}
				if e.NoteUpserted.Visibility != "NOTE_VISIBILITY_PUBLIC" {
					t.Errorf("golden note %q has visibility %q, want NOTE_VISIBILITY_PUBLIC", e.NoteUpserted.Key, e.NoteUpserted.Visibility)
				}
				keys = append(keys, e.NoteUpserted.Key)
			}
			var want []string
			for k := range qaNotesOnDisk(t, dir) {
				want = append(want, k)
			}
			sort.Strings(keys)
			sort.Strings(want)
			if len(keys) != len(want) {
				t.Fatalf("golden holds notes %v, notes files declare %v", keys, want)
			}
			for i := range keys {
				if keys[i] != want[i] {
					t.Fatalf("golden holds notes %v, notes files declare %v", keys, want)
				}
			}
		})
	}
}

func qaCopyFile(tb testing.TB, src, dst string) {
	tb.Helper()
	raw, err := os.ReadFile(src)
	if err != nil {
		tb.Fatalf("read %s: %v", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		tb.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(dst, raw, 0o600); err != nil {
		tb.Fatalf("write %s: %v", dst, err)
	}
}

func qaAdventureWithNotes(tb testing.TB, notes map[string]string) string {
	tb.Helper()
	src := filepath.Join(qaAdventuresRoot, "cellar-rats")
	dst := tb.TempDir()
	for _, rel := range []string{"adventure.json", "guide.md", "scenes/cellar.json", "actors/hollis.json", "actors/mara.json"} {
		qaCopyFile(tb, filepath.Join(src, rel), filepath.Join(dst, rel))
	}
	if notes != nil {
		if err := os.MkdirAll(filepath.Join(dst, "notes"), 0o755); err != nil {
			tb.Fatalf("mkdir notes: %v", err)
		}
	}
	for name, body := range notes {
		if err := os.WriteFile(filepath.Join(dst, "notes", name), []byte(body), 0o600); err != nil {
			tb.Fatalf("write note file: %v", err)
		}
	}
	return dst
}

// VTT-240
func TestQANoteAdventureEveryNoteAcrossSeveralFilesCompilesPublicWhateverItsWordsSay(t *testing.T) {
	dir := qaAdventureWithNotes(t, map[string]string{
		"a-first.json": `[
  {"key": "secret", "title": "Secret", "text": "NOTE_VISIBILITY_SECRET"},
  {"key": "dm-only", "title": "DM only", "text": "visibility: secret"}
]`,
		"b-second.json": `[
  {"key": "plain-fact", "title": "Plain fact", "text": "The well is dry."}
]`,
		"c-third.json": `[
  {"key": "note-visibility-unspecified", "title": "Unspecified", "text": "NOTE_VISIBILITY_UNSPECIFIED"},
  {"key": "gm-eyes", "title": "gm-eyes", "text": "for the DM"}
]`,
	})
	want := qaNotesOnDisk(t, dir)
	if len(want) != 5 {
		t.Fatalf("fixture declares %d notes, want 5", len(want))
	}
	qaAssertAllPublicAndVerbatim(t, qaCompiledNotes(t, dir, engine.NewState()), want)
}

func TestQANoteAdventureAnAdventureWithNoNotesCompilesNoNote(t *testing.T) {
	dir := qaAdventureWithNotes(t, map[string]string{"empty.json": `[]`})
	if got := qaCompiledNotes(t, dir, engine.NewState()); len(got) != 0 {
		t.Fatalf("an adventure with no note compiled %d NoteUpserted", len(got))
	}
}

func TestQANoteAdventureANoteFileNamingAVisibilityIsNeverCompiledSecret(t *testing.T) {
	for _, vis := range []string{`"secret"`, `"NOTE_VISIBILITY_SECRET"`, `2`, `"NOTE_VISIBILITY_UNSPECIFIED"`} {
		t.Run(vis, func(t *testing.T) {
			dir := qaAdventureWithNotes(t, map[string]string{
				"n.json": `[{"key": "k1", "title": "T", "text": "x", "visibility": ` + vis + `}]`,
			})
			adv, err := adventure.Load(dir, qaRulesetOf(t, dir))
			if err != nil {
				if !strings.Contains(err.Error(), `unknown field "visibility"`) {
					t.Fatalf("Load refused it for another reason: %v", err)
				}
				return
			}
			envs, _, err := adventure.Compile(adv, engine.NewState())
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			notes := qaNotesIn(envs)
			if len(notes) != 1 {
				t.Fatalf("compiled %d notes, want 1", len(notes))
			}
			if notes[0].GetVisibility() != vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC {
				t.Fatalf("a note file naming visibility %s loaded and compiled as %v", vis, notes[0].GetVisibility())
			}
		})
	}
}

// VTT-240
func TestQANoteAdventureTheTablesExistingSecretNotesDoNotChangeALoadedNotesVisibility(t *testing.T) {
	st := engine.NewState()
	for i, v := range []vttv1.NoteVisibility{
		vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
		vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED,
	} {
		env := &vttv1.Envelope{
			Sequence: int64(i + 1),
			Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
				Key: "dm-private-" + v.String(), Title: "Private", Text: "the archer flees", Visibility: v,
			}},
		}
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("seed note: %v", err)
		}
	}
	dir := filepath.Join(qaAdventuresRoot, "goblin-ambush")
	qaAssertAllPublicAndVerbatim(t, qaCompiledNotes(t, dir, st), qaNotesOnDisk(t, dir))
}
