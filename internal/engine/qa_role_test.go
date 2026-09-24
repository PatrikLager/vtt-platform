package engine_test

import (
	"reflect"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// qaRoles are the roles an envelope can carry: the four identity roles, the
// empty string an unstamped envelope carries, and a value no role parses to.
var qaRoles = []string{"dm", "agent", "player", "spectator", "", "not-a-role"}

// qaScript builds a fresh event history each call, so no two folds share a
// payload pointer. It touches the state a role could plausibly colour:
// control grants and revocations, visibility, and notes.
func qaScript() []*vttv1.Envelope {
	pos := func(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }
	payloads := []func(*vttv1.Envelope){
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "qa"}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "s1", Name: "Hall", GridWidth: 10, GridHeight: 10}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
				ActorId: "a1", Name: "Ada", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
				Resources: map[string]*vttv1.Resource{"vigor": {Current: 10, Max: 10}},
			}}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
				ActorId: "a2", Name: "Bob", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			}}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: "p1"}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: "a2", ParticipantId: "p2"}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t1", SceneId: "s1", ActorId: "a1", Position: pos(1, 1)}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t2", SceneId: "s1", ActorId: "a2", Position: pos(4, 4)}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: "t1", SceneId: "s1", From: pos(1, 1), To: pos(2, 3)}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{ActorId: "a1", ConditionId: "prone", Source: "qa"}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{ActorId: "a1", Resource: "vigor", Delta: -3, NewValue: 7}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{Key: "n1", Title: "Secret", Text: "dm eyes only"}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_TokenHidden{TokenHidden: &vttv1.TokenHidden{TokenId: "t2"}}
		},
		func(e *vttv1.Envelope) {
			e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: "a2", ParticipantId: "p2"}}
		},
	}
	out := make([]*vttv1.Envelope, len(payloads))
	for i, set := range payloads {
		e := &vttv1.Envelope{EventId: "e" + string(rune('a'+i)), Sequence: int64(i + 1), SessionId: "sess", ParticipantId: "p1"}
		set(e)
		out[i] = e
	}
	return out
}

// qaFold folds the script with each event's ActorRole set by roleOf.
func qaFold(t *testing.T, roleOf func(i int) string) *engine.State {
	t.Helper()
	st := engine.NewState()
	for i, env := range qaScript() {
		env.ActorRole = roleOf(i)
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("role %q: Apply event %d (%T): %v", env.ActorRole, i+1, env.Payload, err)
		}
	}
	return st
}

// VTT-039
func TestQAFoldingIgnoresTheEnvelopesRole(t *testing.T) {
	want := qaFold(t, func(int) string { return "dm" })
	if reflect.DeepEqual(want, engine.NewState()) {
		t.Fatal("fixture: the script left the state empty; the comparison would prove nothing")
	}
	for _, role := range qaRoles {
		got := qaFold(t, func(int) string { return role })
		if !reflect.DeepEqual(got, want) {
			t.Errorf("folding with actor_role %q gives a different state than with %q", role, "dm")
		}
	}
	// Mixed: each event carries a different role than its neighbour.
	mixed := qaFold(t, func(i int) string { return qaRoles[i%len(qaRoles)] })
	if !reflect.DeepEqual(mixed, want) {
		t.Errorf("folding with a role that changes per event gives a different state")
	}
}

// VTT-039 control: the comparison sees a one-field payload difference, so its
// silence above is evidence.
func TestQAFoldComparisonSeesAPayloadDifference(t *testing.T) {
	want := qaFold(t, func(int) string { return "dm" })
	st := engine.NewState()
	for i, env := range qaScript() {
		env.ActorRole = "dm"
		if tm := env.GetTokenMoved(); tm != nil {
			tm.To = &vttv1.GridPosition{X: 5, Y: 5}
		}
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("Apply event %d: %v", i+1, err)
		}
	}
	if reflect.DeepEqual(st, want) {
		t.Fatal("a different TokenMoved destination folded to an equal state: the comparison is blind")
	}
}

// VTT-039: a REFUSED event is refused whatever role it carries, and leaves
// the state as it found it under every role.
func TestQAARefusedEventIsRefusedUnderEveryRole(t *testing.T) {
	base := func() *engine.State { return qaFold(t, func(int) string { return "dm" }) }
	want := base()
	for _, role := range qaRoles {
		st := base()
		// t9 does not exist.
		err := engine.Apply(st, &vttv1.Envelope{
			EventId: "bad", Sequence: 99, ActorRole: role,
			Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: "t9", SceneId: "s1", To: &vttv1.GridPosition{X: 1, Y: 1}}},
		})
		if err == nil {
			t.Errorf("role %q: moving an unknown token was accepted", role)
		}
		if !reflect.DeepEqual(st, want) {
			t.Errorf("role %q: a refused event changed the state", role)
		}
	}
}

// qaRoleNamed reports the full names of every field, reachable from the
// descriptor's fields (restricted to the oneof when oneof != ""), whose name
// names a role, and every enum value that names one of the four roles.
func qaRoleNamed(md protoreflect.MessageDescriptor, oneof string) []string {
	roleWords := []string{"DM", "AGENT", "PLAYER", "SPECTATOR"}
	var found []string
	seen := map[protoreflect.FullName]bool{}
	walkEnum := func(ed protoreflect.EnumDescriptor) {
		if seen[ed.FullName()] {
			return
		}
		seen[ed.FullName()] = true
		if strings.Contains(strings.ToLower(string(ed.Name())), "role") {
			found = append(found, string(ed.FullName()))
		}
		vals := ed.Values()
		for i := 0; i < vals.Len(); i++ {
			for _, w := range strings.Split(string(vals.Get(i).Name()), "_") {
				for _, r := range roleWords {
					if w == r {
						found = append(found, string(vals.Get(i).FullName()))
					}
				}
			}
		}
	}
	var walkField func(fd protoreflect.FieldDescriptor)
	var walkMsg func(m protoreflect.MessageDescriptor, only string)
	walkField = func(fd protoreflect.FieldDescriptor) {
		if strings.Contains(strings.ToLower(string(fd.Name())), "role") ||
			strings.Contains(strings.ToLower(fd.JSONName()), "role") {
			found = append(found, string(fd.FullName()))
		}
		if fd.IsMap() {
			walkField(fd.MapKey())
			walkField(fd.MapValue())
			return
		}
		switch fd.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			walkMsg(fd.Message(), "")
		case protoreflect.EnumKind:
			walkEnum(fd.Enum())
		}
	}
	walkMsg = func(m protoreflect.MessageDescriptor, only string) {
		if only == "" {
			if seen[m.FullName()] {
				return
			}
			seen[m.FullName()] = true
			if strings.Contains(strings.ToLower(string(m.Name())), "role") {
				found = append(found, string(m.FullName()))
			}
		}
		fields := m.Fields()
		for i := 0; i < fields.Len(); i++ {
			fd := fields.Get(i)
			if only != "" && (fd.ContainingOneof() == nil || string(fd.ContainingOneof().Name()) != only) {
				continue
			}
			walkField(fd)
		}
	}
	walkMsg(md, oneof)
	return found
}

// VTT-038
func TestQANoEventPayloadNamesARole(t *testing.T) {
	env := (&vttv1.Envelope{}).ProtoReflect().Descriptor()
	payload := env.Oneofs().ByName("payload")
	if payload == nil || payload.Fields().Len() == 0 {
		t.Fatal("Envelope has no payload oneof with fields; the walk would prove nothing")
	}
	if got := qaRoleNamed(env, "payload"); len(got) != 0 {
		t.Errorf("event payloads name a role: %v", got)
	}
}

// VTT-038 control: the same walk finds the role it is looking for where the
// contract does carry one, so its silence above is evidence.
func TestQARoleWalkFindsTheRolesTheContractDoesCarry(t *testing.T) {
	env := (&vttv1.Envelope{}).ProtoReflect().Descriptor()
	if got := qaRoleNamed(env, ""); !reflect.DeepEqual(got, []string{"vtt.v1.Envelope.actor_role"}) {
		t.Errorf("walking the whole Envelope found %v, want exactly its actor_role", got)
	}
	cmd := (&vttv1.PromoteParticipant{}).ProtoReflect().Descriptor()
	if got := qaRoleNamed(cmd, ""); len(got) != 1 || got[0] != "vtt.v1.PromoteParticipant.role" {
		t.Errorf("walking PromoteParticipant found %v, want its role field", got)
	}
}
