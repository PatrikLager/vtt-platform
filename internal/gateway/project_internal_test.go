package gateway

import (
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// TestEveryEnvelopePayloadArmHasAnExplicitRuling walks the Envelope oneof
// itself and demands a decision for every arm in it.
//
// Spec §8 names the failure this exists to prevent: "a channel nobody
// projected is the most likely real bug". The projection fails closed, so a
// forgotten arm does not leak — but it does go SILENT, and a payload that
// silently stops reaching players is discovered at a table, not in CI.
//
// Driven off the descriptor rather than a hand-written list on purpose: the
// list would be the thing that goes stale. Add a oneof arm to the contract and
// this test reds until project.go rules on it, which is the only mechanism
// that survives a contract this arc is still adding to (visibility Task 2
// added two arms; more will come).
func TestEveryEnvelopePayloadArmHasAnExplicitRuling(t *testing.T) {
	pr := NewProjector(Viewer{ParticipantID: "p-1", Role: identity.RolePlayer})
	now := pr.look(engine.NewState())

	// The control, and it is what stops the loop below being vacuous: an
	// Envelope with NO payload must be unrecognised. If classify's default
	// ever started forwarding, every assertion below would still pass.
	if v := pr.classify(&vttv1.Envelope{Sequence: 1}, now); v != unrecognised {
		t.Fatalf("an Envelope with no payload must be unrecognised, got %v", v)
	}

	oneof := (&vttv1.Envelope{}).ProtoReflect().Descriptor().Oneofs().ByName("payload")
	if oneof == nil {
		t.Fatal("the Envelope has no oneof named payload — this test is pointed at the wrong thing")
	}
	if oneof.Fields().Len() == 0 {
		t.Fatal("the payload oneof has no arms — this test would prove nothing")
	}
	for i := range oneof.Fields().Len() {
		fd := oneof.Fields().Get(i)
		env := &vttv1.Envelope{Sequence: 1}
		m := env.ProtoReflect()
		m.Set(fd, m.NewField(fd))
		if v := pr.classify(env, now); v == unrecognised {
			t.Errorf("payload %q (field %d) has no explicit ruling in classify — "+
				"spec §4.4 requires the switch to be exhaustive over the oneof",
				fd.Name(), fd.Number())
		}
	}
}

// VTT-272
func TestEveryEnvelopeFieldIsKeptOrClearedByForwardable(t *testing.T) {
	kept := map[protoreflect.Name]bool{"event_id": true, "sequence": true, "occurred_at": true, "session_id": true}
	cleared := map[protoreflect.Name]bool{"actor_role": true, "participant_id": true}

	fields := (&vttv1.Envelope{}).ProtoReflect().Descriptor().Fields()
	ruled := 0
	for i := range fields.Len() {
		fd := fields.Get(i)
		if fd.ContainingOneof() != nil {
			continue
		}
		if !kept[fd.Name()] && !cleared[fd.Name()] {
			t.Errorf("Envelope field %q has no ruling: keep it or clear it in forwardable (SPEC-016)", fd.Name())
			continue
		}
		ruled++
		env := &vttv1.Envelope{}
		m := env.ProtoReflect()
		switch fd.Kind() {
		case protoreflect.StringKind:
			m.Set(fd, protoreflect.ValueOfString("x"))
		case protoreflect.Int64Kind:
			m.Set(fd, protoreflect.ValueOfInt64(7))
		case protoreflect.MessageKind:
			m.Set(fd, m.NewField(fd))
		default:
			t.Fatalf("Envelope field %q is of kind %v, which this test cannot set", fd.Name(), fd.Kind())
		}
		got := forwardable(env).ProtoReflect()
		if kept[fd.Name()] && !got.Has(fd) {
			t.Errorf("forwardable dropped %q, which a forwarded frame keeps", fd.Name())
		}
		if cleared[fd.Name()] && got.Has(fd) {
			t.Errorf("forwardable keeps %q, which names who issued the event", fd.Name())
		}
	}
	if ruled != len(kept)+len(cleared) {
		t.Errorf("ruled %d Envelope fields, want %d: a ruled field left the contract", ruled, len(kept)+len(cleared))
	}
}
