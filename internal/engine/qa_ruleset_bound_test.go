package engine_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const qaRuleBound = 128

const (
	qaRuleCondition = "ConditionApplied"
	qaRuleAbility   = "AbilityUsed"
	qaRuleActorID   = "qa-actor"
)

var qaRuleKinds = []string{qaRuleCondition, qaRuleAbility}

var qaRuleField = map[string]*regexp.Regexp{
	qaRuleCondition: regexp.MustCompile(`(?i)condition[ _.]?id`),
	qaRuleAbility:   regexp.MustCompile(`(?i)ability[ _.]?id`),
}

var qaRuleZero = regexp.MustCompile(`\b0\b`)

type qaRuleCase struct {
	label string
	id    string
}

func qaRuleASCII(n int) string { return strings.Repeat("a", n) }

func qaRuleAccepted() []qaRuleCase {
	return []qaRuleCase{
		{"one byte", "a"},
		{"127 ascii", qaRuleASCII(127)},
		{"128 ascii", "S" + qaRuleASCII(126) + "E"},
		{"64 two-byte runes", strings.Repeat("é", 64)},
		{"42 three-byte runes and two ascii", strings.Repeat("€", 42) + "ab"},
		{"32 four-byte runes", strings.Repeat("😀", 32)},
	}
}

func qaRuleRefused() []qaRuleCase {
	return []qaRuleCase{
		{"129 ascii", qaRuleASCII(129)},
		{"70000 ascii", qaRuleASCII(70000)},
		{"65 two-byte runes", strings.Repeat("é", 65)},
		{"one ascii then 64 two-byte runes", "a" + strings.Repeat("é", 64)},
		{"43 three-byte runes", strings.Repeat("€", 43)},
		{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
	}
}

func qaRuleEvent(kind, id, actorID string) *vttv1.Envelope {
	env := &vttv1.Envelope{EventId: "qa-rule-event", Sequence: 2}
	switch kind {
	case qaRuleCondition:
		env.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
			ActorId: actorID, ConditionId: id, Source: "qa-source",
		}}
	case qaRuleAbility:
		env.Payload = &vttv1.Envelope_AbilityUsed{AbilityUsed: &vttv1.AbilityUsed{
			ActorId: actorID, AbilityId: id, TargetIds: []string{actorID}, OutcomeSummary: "qa",
		}}
	}
	return env
}

func qaRuleState(t *testing.T) *engine.State {
	t.Helper()
	st := engine.NewState()
	actor := &vttv1.Envelope{EventId: "qa-rule-actor", Sequence: 1, Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: qaRuleActorID, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}},
	}}
	if err := engine.Apply(st, actor); err != nil {
		t.Fatalf("setup ActorAdded refused: %v", err)
	}
	return st
}

func qaRuleHeld(st *engine.State, actorID, id string) bool {
	for _, c := range st.Conditions[actorID] {
		if c.ID == id {
			return true
		}
	}
	return false
}

func qaRuleRefusal(t *testing.T, st *engine.State, env *vttv1.Envelope) string {
	t.Helper()
	err := engine.Apply(st, env)
	if err == nil {
		t.Fatalf("engine.Apply accepted the event the test expects it to refuse")
	}
	return err.Error()
}

// VTT-285
func TestQARuleTheGoFoldAcceptsAnIDUpToTheBound(t *testing.T) {
	for _, kind := range qaRuleKinds {
		for _, c := range qaRuleAccepted() {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				if len(c.id) > qaRuleBound {
					t.Fatalf("fixture %q is %d bytes, over the bound", c.label, len(c.id))
				}
				st := qaRuleState(t)
				if err := engine.Apply(st, qaRuleEvent(kind, c.id, qaRuleActorID)); err != nil {
					t.Fatalf("%s id of %d bytes refused: %v", kind, len(c.id), err)
				}
				if kind == qaRuleCondition && !qaRuleHeld(st, qaRuleActorID, c.id) {
					t.Fatalf("condition id of %d bytes accepted but not held whole", len(c.id))
				}
			})
		}
	}
}

// VTT-285
func TestQARuleTheGoFoldRefusesAnIDOverTheBoundNamingFieldBoundAndLength(t *testing.T) {
	for _, kind := range qaRuleKinds {
		for _, c := range qaRuleRefused() {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				if len(c.id) <= qaRuleBound {
					t.Fatalf("fixture %q is %d bytes, within the bound", c.label, len(c.id))
				}
				msg := qaRuleElide(qaRuleRefusal(t, qaRuleState(t), qaRuleEvent(kind, c.id, qaRuleActorID)), c.id)
				t.Logf("refusal: %s", msg)
				if !qaRuleField[kind].MatchString(msg) {
					t.Errorf("refusal %q does not name the %s id field", msg, kind)
				}
				for _, want := range []string{strconv.Itoa(qaRuleBound), strconv.Itoa(len(c.id))} {
					if !strings.Contains(msg, want) {
						t.Errorf("refusal %q does not contain %q", msg, want)
					}
				}
			})
		}
	}
}

// VTT-286
func TestQARuleTheGoFoldRefusesAnEmptyID(t *testing.T) {
	for _, kind := range qaRuleKinds {
		t.Run(kind, func(t *testing.T) {
			msg := qaRuleRefusal(t, qaRuleState(t), qaRuleEvent(kind, "", qaRuleActorID))
			t.Logf("refusal: %s", msg)
			if !qaRuleField[kind].MatchString(msg) {
				t.Errorf("refusal %q does not name the %s id field", msg, kind)
			}
			if !strings.Contains(msg, strconv.Itoa(qaRuleBound)) || !qaRuleZero.MatchString(msg) {
				t.Errorf("refusal %q does not carry the bound %d and the length 0", msg, qaRuleBound)
			}
		})
	}
}

// VTT-285 VTT-286
func TestQARuleTheGoFoldChecksAConditionIDBeforeItsActorAndADuplicate(t *testing.T) {
	long := qaRuleASCII(qaRuleBound + 1)
	duplicate := func(id string) func(*engine.State) {
		return func(st *engine.State) {
			st.Conditions[qaRuleActorID] = append(st.Conditions[qaRuleActorID], engine.ActorCondition{ID: id, AppliedSeq: 1})
		}
	}
	cases := []struct {
		label   string
		id      string
		noActor bool
		inject  func(*engine.State)
	}{
		{"long id before an unknown actor", long, true, nil},
		{"empty id before an unknown actor", "", true, nil},
		{"long id before a duplicate", long, false, duplicate(long)},
		{"empty id before a duplicate", "", false, duplicate("")},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			want := qaRuleRefusal(t, qaRuleState(t), qaRuleEvent(qaRuleCondition, c.id, qaRuleActorID))
			st := qaRuleState(t)
			if c.noActor {
				st = engine.NewState()
			}
			if c.inject != nil {
				c.inject(st)
			}
			got := qaRuleRefusal(t, st, qaRuleEvent(qaRuleCondition, c.id, qaRuleActorID))
			if got != want {
				t.Fatalf("refusal\n got %q\nwant %q (the id's own)", qaRuleElide(got, c.id), qaRuleElide(want, c.id))
			}
		})
	}
}

// VTT-285 VTT-286
func TestQARuleTheGoFoldChecksAnAbilityUsedForItsIDAlone(t *testing.T) {
	for _, c := range qaRuleAccepted()[2:] {
		t.Run("accepts "+c.label+" naming an unknown actor", func(t *testing.T) {
			if err := engine.Apply(engine.NewState(), qaRuleEvent(qaRuleAbility, c.id, "qa-nobody")); err != nil {
				t.Fatalf("AbilityUsed of a %d-byte id naming an unknown actor refused: %v", len(c.id), err)
			}
		})
	}
	for _, c := range []qaRuleCase{{"long", qaRuleASCII(qaRuleBound + 1)}, {"empty", ""}} {
		t.Run("refuses "+c.label+" naming an unknown actor with the id's own text", func(t *testing.T) {
			want := qaRuleRefusal(t, qaRuleState(t), qaRuleEvent(qaRuleAbility, c.id, qaRuleActorID))
			got := qaRuleRefusal(t, engine.NewState(), qaRuleEvent(qaRuleAbility, c.id, qaRuleActorID))
			if got != want {
				t.Fatalf("refusal\n got %q\nwant %q", qaRuleElide(got, c.id), qaRuleElide(want, c.id))
			}
		})
	}
}

func qaRuleElide(msg, id string) string {
	if id == "" {
		return msg
	}
	return strings.ReplaceAll(msg, id, "<id>")
}
