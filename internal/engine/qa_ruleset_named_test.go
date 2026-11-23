package engine_test

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const (
	qarnID      = "qa-rules"
	qarnVersion = "2.5.0"
	qarnName    = "Plain Rules"
)

func qarnEnv(seq int64, id, version, name string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:       "ev-rn",
		Sequence:      seq,
		SessionId:     "s-1",
		ParticipantId: "p-dm",
		ActorRole:     "dm",
		Payload: &vttv1.Envelope_RulesetNamed{RulesetNamed: &vttv1.RulesetNamed{
			RulesetId: id, Version: version, Name: name,
		}},
	}
}

// Keep every map of the state written here: VTT-979's comparison reads them.
func qarnPrefix() []*vttv1.Envelope {
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	return []*vttv1.Envelope{
		{EventId: "ev-1", Sequence: 1, SessionId: "s-1", Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "Evening"}}},
		{EventId: "ev-2", Sequence: 2, SessionId: "s-1", Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "sc-1", Name: "Hall", GridWidth: 6, GridHeight: 5}}},
		{EventId: "ev-3", Sequence: 3, SessionId: "s-1", Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "a-1", Name: "Ada", Kind: party,
			Attributes: map[string]int32{"k1": 3},
			Resources:  map[string]*vttv1.Resource{"r1": {Current: 2, Max: 4}},
			AbilityIds: []string{"ab-1"},
		}}}},
		{EventId: "ev-4", Sequence: 4, SessionId: "s-1", Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "b-1", Name: "Bram", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}}}},
		{EventId: "ev-5", Sequence: 5, SessionId: "s-1", Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: "a-1", ParticipantId: "p-1", Kind: party}}},
		{EventId: "ev-6", Sequence: 6, SessionId: "s-1", Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-1", SceneId: "sc-1", ActorId: "a-1", Position: &vttv1.GridPosition{X: 1, Y: 2}, Name: "Ada"}}},
		{EventId: "ev-7", Sequence: 7, SessionId: "s-1", Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-2", SceneId: "sc-1", ActorId: "b-1", Position: &vttv1.GridPosition{X: 4, Y: 3}}}},
		{EventId: "ev-8", Sequence: 8, SessionId: "s-1", Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{ActorId: "a-1", ConditionId: "cond-1", Source: "manual"}}},
		{EventId: "ev-9", Sequence: 9, SessionId: "s-1", Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{Key: "n-1", Title: "Title", Text: "Body", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC}}},
	}
}

func qarnFold(t *testing.T, envs ...*vttv1.Envelope) *engine.State {
	t.Helper()
	st := engine.NewState()
	for _, e := range envs {
		if err := engine.Apply(st, e); err != nil {
			t.Fatalf("fixture event %d does not fold: %v", e.Sequence, err)
		}
	}
	return st
}

func qarnSameButRuleset(t *testing.T, want, got *engine.State) {
	t.Helper()
	wv, gv := reflect.ValueOf(want).Elem(), reflect.ValueOf(got).Elem()
	for i := 0; i < wv.NumField(); i++ {
		f := wv.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		switch f.Name {
		case "RulesetID", "RulesetVersion", "RulesetName":
			continue
		case "Actors":
			if len(want.Actors) != len(got.Actors) {
				t.Errorf("Actors: want %d actors, got %d", len(want.Actors), len(got.Actors))
			}
			for id, a := range want.Actors {
				if !proto.Equal(a, got.Actors[id]) {
					t.Errorf("Actors[%q]: want %v, got %v", id, a, got.Actors[id])
				}
			}
		default:
			if !reflect.DeepEqual(wv.Field(i).Interface(), gv.Field(i).Interface()) {
				t.Errorf("%s changed: want %#v, got %#v", f.Name, wv.Field(i).Interface(), gv.Field(i).Interface())
			}
		}
	}
}

func qarnRequireNames(t *testing.T, st *engine.State, id, version, name string) {
	t.Helper()
	if st.RulesetID != id || st.RulesetVersion != version || st.RulesetName != name {
		t.Errorf("state names (%q, %q, %q), want (%q, %q, %q)",
			st.RulesetID, st.RulesetVersion, st.RulesetName, id, version, name)
	}
}

var qarnWordSplit = regexp.MustCompile(`[A-Z]?[a-z]+|[A-Z]+(?:[a-z]+)?|[0-9]+`)

// Split at a camelCase hump and at punctuation: "ruleset_named" is no "name".
func qarnWords(msg string) map[string]bool {
	words := map[string]bool{}
	for _, part := range regexp.MustCompile(`[^A-Za-z0-9]+`).Split(msg, -1) {
		for _, w := range qarnWordSplit.FindAllString(part, -1) {
			words[strings.ToLower(w)] = true
		}
	}
	return words
}

var qarnFieldWord = map[string]string{"id": "id", "version": "version", "name": "name"}

func qarnRequireNamesField(t *testing.T, err error, field string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want a refusal naming the %s, got none", field)
	}
	words := qarnWords(err.Error())
	for f, w := range qarnFieldWord {
		if f == field && !words[w] {
			t.Errorf("refusal %q does not name the %s", err, field)
		}
		if f != field && words[w] {
			t.Errorf("refusal of the %s %q also names the %s", field, err, f)
		}
	}
}

func qarnWith(field, value string) (string, string, string) {
	return qarnPut(field, value, qarnID, qarnVersion, qarnName)
}

func qarnPut(field, value, id, version, name string) (string, string, string) {
	switch field {
	case "id":
		id = value
	case "version":
		version = value
	case "name":
		name = value
	}
	return id, version, name
}

// Keep each multi-byte value's rune count and UTF-16 length under the bound.
var (
	qarnAt128 = map[string]string{
		"ascii":  strings.Repeat("r", 128),
		"3-byte": strings.Repeat("€", 42) + "rr",
		"4-byte": strings.Repeat("😀", 32),
	}
	qarnVersionAt128 = map[string]string{
		"ascii":  strings.Repeat("v", 128),
		"3-byte": "vv" + strings.Repeat("€", 42),
		"4-byte": strings.Repeat("😃", 32),
	}
	qarnPast128 = map[string]string{
		"ascii":  strings.Repeat("r", 129),
		"3-byte": strings.Repeat("€", 43),
		"4-byte": strings.Repeat("😀", 32) + "r",
	}
	qarnAt256 = map[string]string{
		"ascii":  strings.Repeat("n", 256),
		"3-byte": strings.Repeat("€", 85) + "n",
		"4-byte": strings.Repeat("😀", 64),
	}
	qarnPast256 = map[string]string{
		"ascii":  strings.Repeat("n", 257),
		"3-byte": strings.Repeat("€", 86),
		"4-byte": strings.Repeat("😀", 64) + "n",
	}
)

func qarnAt(field string) map[string]string {
	if field == "name" {
		return qarnAt256
	}
	return qarnAt128
}

func qarnPast(field string) map[string]string {
	if field == "name" {
		return qarnPast256
	}
	return qarnPast128
}

func TestQARNEachFieldExactlyAtItsBoundIsFolded(t *testing.T) {
	for _, field := range []string{"id", "version", "name"} {
		for enc, v := range qarnAt(field) {
			t.Run(field+"/"+enc, func(t *testing.T) {
				id, version, name := qarnWith(field, v)
				st := engine.NewState()
				// VTT-974
				if err := engine.Apply(st, qarnEnv(1, id, version, name)); err != nil {
					t.Fatalf("a %s of %d bytes is refused: %v", field, len(v), err)
				}
				// VTT-979
				qarnRequireNames(t, st, id, version, name)
			})
		}
	}
}

func TestQARNAllThreeAtTheirBoundsTogetherAreFolded(t *testing.T) {
	for _, enc := range []string{"ascii", "3-byte", "4-byte"} {
		t.Run(enc, func(t *testing.T) {
			id, version, name := qarnAt128[enc], qarnVersionAt128[enc], qarnAt256[enc]
			st := engine.NewState()
			// VTT-974
			if err := engine.Apply(st, qarnEnv(1, id, version, name)); err != nil {
				t.Fatalf("id, version and name at their bounds are refused: %v", err)
			}
			// VTT-979
			qarnRequireNames(t, st, id, version, name)
		})
	}
}

func TestQARNEachFieldEmptyOrPastItsBoundIsRefusedNamingIt(t *testing.T) {
	for _, field := range []string{"id", "version", "name"} {
		cases := map[string]string{"empty": ""}
		for enc, v := range qarnPast(field) {
			cases["past/"+enc] = v
		}
		for label, v := range cases {
			t.Run(field+"/"+label, func(t *testing.T) {
				id, version, name := qarnWith(field, v)
				st := qarnFold(t, qarnPrefix()...)
				err := engine.Apply(st, qarnEnv(10, id, version, name))
				// VTT-974
				qarnRequireNamesField(t, err, field)
				// VTT-974: a refusal leaves the state as it was (SPEC-018).
				qarnSameButRuleset(t, qarnFold(t, qarnPrefix()...), st)
				qarnRequireNames(t, st, "", "", "")
			})
		}
	}
}

func TestQARNARefusalLeavesTheRulesetAStateNamesStanding(t *testing.T) {
	named := append(qarnPrefix(), qarnEnv(10, qarnID, qarnVersion, qarnName))
	for _, field := range []string{"id", "version", "name"} {
		for label, v := range map[string]string{"empty": "", "past": qarnPast(field)["4-byte"]} {
			t.Run(field+"/"+label, func(t *testing.T) {
				id, version, name := qarnPut(field, v, "other-rules", "2.5.1", "Plain Rules II")
				st := qarnFold(t, named...)
				err := engine.Apply(st, qarnEnv(11, id, version, name))
				// VTT-974
				qarnRequireNamesField(t, err, field)
				// VTT-974
				qarnSameButRuleset(t, qarnFold(t, named...), st)
				qarnRequireNames(t, st, qarnID, qarnVersion, qarnName)
			})
		}
	}
}

func TestQARNNamesItsRulesetAndChangesNothingElse(t *testing.T) {
	for label, prefix := range map[string][]*vttv1.Envelope{"empty": nil, "populated": qarnPrefix()} {
		t.Run(label, func(t *testing.T) {
			before := qarnFold(t, prefix...)
			after := qarnFold(t, prefix...)
			if err := engine.Apply(after, qarnEnv(10, qarnID, qarnVersion, qarnName)); err != nil {
				t.Fatalf("refused: %v", err)
			}
			// VTT-979
			qarnRequireNames(t, after, qarnID, qarnVersion, qarnName)
			// VTT-979
			qarnSameButRuleset(t, before, after)
		})
	}
}

func TestQARNALaterOneReplacesEachFieldTheStateNames(t *testing.T) {
	steps := []struct{ id, version, name string }{
		{qarnID, qarnVersion, qarnName},
		{qarnID, "2.6.0", "Plain Rules Revised"},
		{"other-rules", qarnVersion, qarnName},
		{"other-rules", qarnVersion, "Other Rules"},
	}
	st := qarnFold(t, qarnPrefix()...)
	for i, s := range steps {
		if err := engine.Apply(st, qarnEnv(int64(10+i), s.id, s.version, s.name)); err != nil {
			t.Fatalf("step %d refused: %v", i, err)
		}
		// VTT-979
		qarnRequireNames(t, st, s.id, s.version, s.name)
	}
	// VTT-979
	qarnSameButRuleset(t, qarnFold(t, qarnPrefix()...), st)
}

func qarnDump(t *testing.T, st *engine.State) map[string]json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestQARNTheDumpNamesTheRulesetAndNothingElseChanges(t *testing.T) {
	name := strings.Repeat("€", 85) + "n"
	before := qarnFold(t, qarnPrefix()...)
	after := qarnFold(t, append(qarnPrefix(), qarnEnv(10, qarnID, qarnVersion, name))...)
	b, a := qarnDump(t, before), qarnDump(t, after)
	want := map[string]string{"RulesetID": qarnID, "RulesetVersion": qarnVersion, "RulesetName": name}
	for k, v := range want {
		var got string
		if err := json.Unmarshal(a[k], &got); err != nil || got != v {
			// VTT-979
			t.Errorf("dump %s = %s, want %q", k, a[k], v)
		}
		delete(a, k)
	}
	// VTT-979
	if !reflect.DeepEqual(b, a) {
		t.Errorf("the dump changed beyond the ruleset:\nbefore %s\nafter  %s", qarnRaw(b), qarnRaw(a))
	}
}

func qarnRaw(m map[string]json.RawMessage) string {
	raw, _ := json.Marshal(m)
	return string(raw)
}

func TestQARNTheDumpOfALogThatNamesNoRulesetWritesNothingOfOne(t *testing.T) {
	refused := qarnFold(t, qarnPrefix()...)
	if err := engine.Apply(refused, qarnEnv(10, qarnID, "", qarnName)); err == nil {
		t.Fatalf("an empty version was folded")
	}
	for label, st := range map[string]*engine.State{
		"new":       engine.NewState(),
		"populated": qarnFold(t, qarnPrefix()...),
		"refused":   refused,
	} {
		raw, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		// VTT-987
		if strings.Contains(strings.ToLower(string(raw)), "ruleset") {
			t.Errorf("%s: the dump writes a ruleset: %s", label, raw)
		}
	}
}

func TestQARNASnapshotNamesTheRulesetItsStateNames(t *testing.T) {
	st := qarnFold(t, append(qarnPrefix(), qarnEnv(10, qarnID, qarnVersion, qarnName))...)
	// VTT-979
	qarnRequireNames(t, st.Snapshot(), qarnID, qarnVersion, qarnName)
}
