package contract_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func roundTrip(t *testing.T, fixture string, msg proto.Message) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := protojson.Unmarshal(raw, msg); err != nil {
		t.Fatalf("unmarshal %s: %v", fixture, err)
	}
	out, err := protojson.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s round-trip mismatch:\nwant %v\ngot  %v", fixture, want, got)
	}
}

func TestTokenMovedRoundTrip(t *testing.T) { roundTrip(t, "token_moved.json", &vttv1.TokenMoved{}) }

// TestRemoveTokenCommandRoundTrip is retraction-leaves Task 8's own fixture:
// remove_token arrived as a ClientCommand oneof arm in that task, and this
// pins its wire shape exactly as move_token_request.json pins
// MoveTokenRequest's — a bare token_id, nothing else. (It read "the newest
// arm" until 2026-09-01, one task before remove_actor below made that false;
// two comments eight lines apart then claimed the same superlative.)
func TestRemoveTokenCommandRoundTrip(t *testing.T) {
	roundTrip(t, "remove_token_command.json", &vttv1.ClientCommand{})
}

// TestRemoveActorCommandRoundTrip is retraction-leaves Task 9's own fixture:
// remove_actor arrived as a ClientCommand oneof arm one task after
// remove_token above, and this pins its wire shape — a bare actor_id, nothing
// else. The BATCH it produces is not visible
// here and is not meant to be: the wire carries a command, and the events it
// becomes are the gateway's (internal/gateway's handleRemoveActor).
func TestRemoveActorCommandRoundTrip(t *testing.T) {
	roundTrip(t, "remove_actor_command.json", &vttv1.ClientCommand{})
}
func TestAttackRolledRoundTrip(t *testing.T) {
	roundTrip(t, "attack_rolled.json", &vttv1.AttackRolled{})
}
func TestActorRoundTrip(t *testing.T) { roundTrip(t, "actor.json", &vttv1.Actor{}) }
func TestMoveTokenRequestRoundTrip(t *testing.T) {
	roundTrip(t, "move_token_request.json", &vttv1.MoveTokenRequest{})
}
func TestEnvelopeRoundTrip(t *testing.T) { roundTrip(t, "envelope.json", &vttv1.Envelope{}) }

func TestSceneEnvelopeRoundTrip(t *testing.T) { roundTrip(t, "scene_envelope.json", &vttv1.Envelope{}) }
func TestClientCommandRoundTrip(t *testing.T) {
	roundTrip(t, "client_command.json", &vttv1.ClientCommand{})
}
func TestAbilityUsedEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "ability_used_envelope.json", &vttv1.Envelope{})
}
func TestUseAbilityCommandRoundTrip(t *testing.T) {
	roundTrip(t, "use_ability_command.json", &vttv1.ClientCommand{})
}
func TestNarrationAddedEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "narration_added_envelope.json", &vttv1.Envelope{})
}
func TestUpsertNoteCommandRoundTrip(t *testing.T) {
	roundTrip(t, "upsert_note_command.json", &vttv1.ClientCommand{})
}
func TestNoteUpsertedEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "note_upserted_envelope.json", &vttv1.Envelope{})
}
func TestTokenPlacedWithTextEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "token_placed_with_text_envelope.json", &vttv1.Envelope{})
}
func TestTokenDescribedEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "token_described_envelope.json", &vttv1.Envelope{})
}
func TestDescribeTokenCommandRoundTrip(t *testing.T) {
	roundTrip(t, "describe_token_command.json", &vttv1.ClientCommand{})
}
func TestAddNarrationCommandRoundTrip(t *testing.T) {
	roundTrip(t, "add_narration_command.json", &vttv1.ClientCommand{})
}
func TestAddNarrationOutOfCharacterCommandRoundTrip(t *testing.T) {
	roundTrip(t, "add_narration_out_of_character_command.json", &vttv1.ClientCommand{})
}
func TestNarrationOutOfCharacterEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "narration_out_of_character_envelope.json", &vttv1.Envelope{})
}
func TestAbilityUsedWithOutcomesEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "ability_used_with_outcomes_envelope.json", &vttv1.Envelope{})
}
func TestTokenMovedReasonEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "token_moved_reason_envelope.json", &vttv1.Envelope{})
}
func TestAdventureLoadedEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "adventure_loaded_envelope.json", &vttv1.Envelope{})
}
func TestLoadAdventureCommandRoundTrip(t *testing.T) {
	roundTrip(t, "load_adventure_command.json", &vttv1.ClientCommand{})
}
func TestServerFrameResultRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_result.json", &vttv1.ServerFrame{})
}

// TestServerFrameErrorRoundTrip covers the non-empty error path, distinct
// from server_frame_result.json's ok=true/sequence-set success shape: ok and
// sequence are both proto3 zero values (false, 0) on a rejected command, so
// protojson omits them from the wire form entirely — the fixture has no
// "ok" or "sequence" key at all. A consumer must treat an absent "ok" as
// false and an absent "sequence" as unset/0, not fail to parse.
func TestServerFrameErrorRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_error.json", &vttv1.ServerFrame{})
}

// TestServerFrameCatchUpHeadRoundTrip covers the third ServerFrame case, the
// boundary marker every connection opens with. head_sequence is an int64, so
// protojson carries it as a STRING on the wire ("12", not 12) — the same
// convention Envelope.sequence uses, and the thing a hand-written client is
// most likely to get wrong about this frame.
func TestServerFrameCatchUpHeadRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_catch_up_head.json", &vttv1.ServerFrame{})
}

// TestCatchUpHeadZeroSerializesAsAnEmptyMessage pins the encoding of the empty
// log, which is load-bearing rather than incidental: the gateway sends this
// frame UNCONDITIONALLY, head 0 included, precisely so that a client never has
// to interpret "no frame yet". head_sequence 0 is a proto3 zero value, so
// protojson omits the field and the frame goes out as {"catchUpHead":{}}.
//
// A consumer that treats the absent key as "not a catch-up-head frame" —
// rather than as head 0 — reintroduces the guess the frame was added to
// remove, and does it exactly in the empty-campaign case where nothing else
// will ever arrive to correct it.
func TestCatchUpHeadZeroSerializesAsAnEmptyMessage(t *testing.T) {
	raw, err := protojson.Marshal(&vttv1.ServerFrame{
		Frame: &vttv1.ServerFrame_CatchUpHead{CatchUpHead: &vttv1.CatchUpHead{HeadSequence: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var back vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	head := back.GetCatchUpHead()
	if head == nil {
		t.Fatalf("want the catch_up_head case to survive a zero head, got %s", raw)
	}
	if head.GetHeadSequence() != 0 {
		t.Fatalf("head_sequence = %d, want 0", head.GetHeadSequence())
	}
}

func TestServerFrameViewpointChangedRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_viewpoint_changed.json", &vttv1.ServerFrame{})
}

// Keep the empty case: the arm's presence is the frame, and an empty
// actor_id is nobody, at the open or on a switch (SPEC-007).
func TestServerFrameViewpointChangedToNobodyRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_viewpoint_changed_nobody.json", &vttv1.ServerFrame{})
}

// Keep the nameless actor: a client labels it by its id (VTT-404).
func TestServerFrameWatchableChangedRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_watchable_changed.json", &vttv1.ServerFrame{})
}

// Keep the empty case: the arm's presence is the frame (VTT-399).
func TestServerFrameWatchableChangedToNoneRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_watchable_changed_none.json", &vttv1.ServerFrame{})
}

// VTT-454
func TestServerFrameWatchableChangedControlsNoneRoundTrip(t *testing.T) {
	roundTrip(t, "server_frame_watchable_changed_controls_none.json", &vttv1.ServerFrame{})
}

// protojson omits empty fields, so an empty message serializes as
// {"watchableChanged":{}}; a reader that takes the absent "actors" key for no
// frame keeps the last set it was told (VTT-399).
func TestAnEmptyWatchableSetStillSerializesAsItsFrame(t *testing.T) {
	frame := &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_WatchableChanged{
		WatchableChanged: &vttv1.WatchableChanged{},
	}}
	raw, err := protojson.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	set, ok := got["watchableChanged"]
	if !ok {
		t.Fatalf("an empty set must still identify itself as one, got %s", raw)
	}
	if m, isMap := set.(map[string]any); !isMap || len(m) != 0 {
		t.Fatalf("want an empty object for an empty message, got %v", set)
	}
	var back vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.GetWatchableChanged() == nil {
		t.Fatal("round-trip lost the watchable_changed arm — an empty set must not decode as no frame")
	}
	if n := len(back.GetWatchableChanged().GetActors()); n != 0 {
		t.Fatalf("want no actor, got %d", n)
	}
}

func TestEnvelopePayloadIsCompilerDiscriminated(t *testing.T) {
	raw, _ := os.ReadFile("testdata/envelope.json")
	var env vttv1.Envelope
	if err := protojson.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	switch p := env.Payload.(type) {
	case *vttv1.Envelope_TokenMoved:
		if p.TokenMoved.TokenId != "tok-ursus" {
			t.Fatalf("wrong token: %s", p.TokenMoved.TokenId)
		}
	default:
		t.Fatalf("expected TokenMoved payload, got %T", p)
	}
}

// --- presence and actor control (spec 2026-08-06) -----------------------------
//
// Actor control becomes a SET while `controller_id` stays exactly where it is:
// ADR-007 is additive-only, and there are ActorAdded events in real campaign
// logs already carrying that field. You cannot reinterpret history you have
// written, which is a property of event sourcing rather than a rule anyone
// chose.
//
// The fixture pins the MIRROR RULE on the wire: controller_id is
// controller_ids[0], so a two-controller actor still shows one real controller
// to a reader that predates the set. It is deliberately NOT empty — an empty
// controller_id already means DM/agent-only, and protojson omits empty
// strings, so blanking it for a shared actor would be byte-identical to
// "nobody controls this" and would drop the actor from its own player's list.

func TestActorWithMultipleControllersRoundTrip(t *testing.T) {
	roundTrip(t, "actor_multi_controller.json", &vttv1.Actor{})
}

func TestActorControlGrantedEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "actor_control_granted_envelope.json", &vttv1.Envelope{})
}

func TestActorControlRevokedEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "actor_control_revoked_envelope.json", &vttv1.Envelope{})
}

// Presence rides the ServerFrame, NEVER the Envelope. Who happened to be
// online is not campaign history: appending it would make every replay
// reconstruct session noise and change what `vtt state dump` prints.
//
// These fixtures prove ServerFrame CARRIES presence; they do not prove the
// Envelope cannot. That guarantee is structural — Envelope's payload oneof has
// no presence arm — and adding one tomorrow would leave these green. The
// absence of that arm is the real protection.

// An EMPTY snapshot is the case the contract's prose promises ("sent
// unconditionally, including when empty") and the one a fixture cannot
// express: protojson omits empty repeated fields, so the frame goes out as
// {"presenceSnapshot":{}}. This is the mirror of
// TestCatchUpHeadZeroSerializesAsAnEmptyMessage above, and exists for the same
// reason — a consumer that reads the absent "present" key as "not a snapshot"
// reintroduces exactly the guess the snapshot was added to remove, in the
// empty-table case where nothing later will correct it.
func TestEmptyPresenceSnapshotStillSerializesAsASnapshotFrame(t *testing.T) {
	frame := &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_PresenceSnapshot{
		PresenceSnapshot: &vttv1.PresenceSnapshot{},
	}}
	raw, err := protojson.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	snap, ok := got["presenceSnapshot"]
	if !ok {
		t.Fatalf("an empty snapshot must still identify itself as one, got %s", raw)
	}
	if m, isMap := snap.(map[string]any); !isMap || len(m) != 0 {
		t.Fatalf("want an empty object for a table with nobody on it, got %v", snap)
	}

	var back vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.GetPresenceSnapshot() == nil {
		t.Fatal("round-trip lost the snapshot arm — an empty snapshot must not decode as no frame")
	}
	if n := len(back.GetPresenceSnapshot().GetPresent()); n != 0 {
		t.Fatalf("want nobody present, got %d", n)
	}
}

func TestPresenceSnapshotFrameRoundTrip(t *testing.T) {
	roundTrip(t, "presence_snapshot_frame.json", &vttv1.ServerFrame{})
}

func TestPresenceChangedFrameRoundTrip(t *testing.T) {
	roundTrip(t, "presence_changed_frame.json", &vttv1.ServerFrame{})
}

// Phase 4a's tests of decision D1 of
// docs/superpowers/plans/2026-10-26-a-player-speaks-as-their-character.md;
// "D1" below is that decision. Keep them in this file: .go-arch-lint.yml
// attaches no component to another *_test.go in contract/.

// D1: AddNarration.actor_id = 5, out_of_character = 6 (fields).
func TestQANarrCommandFieldNumbersOnTheWire(t *testing.T) {
	b, err := proto.Marshal(&vttv1.AddNarration{
		ActorId: "a", OutOfCharacter: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// tag 5 string = 0x2a, tag 6 varint = 0x30
	want := []byte{0x2a, 0x01, 'a', 0x30, 0x01}
	if !bytes.Equal(b, want) {
		t.Fatalf("got % x want % x", b, want)
	}
}

// D1: NarrationAdded.out_of_character = 5 (fields).
func TestQANarrEventFieldNumberOnTheWire(t *testing.T) {
	b, err := proto.Marshal(&vttv1.NarrationAdded{OutOfCharacter: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x28, 0x01} // tag 5 varint
	if !bytes.Equal(b, want) {
		t.Fatalf("got % x want % x", b, want)
	}
}

// D1: the command names actorId and outOfCharacter on the wire.
func TestQANarrCommandJSONNames(t *testing.T) {
	an := &vttv1.AddNarration{Text: "hi", ActorId: "ada", OutOfCharacter: true}
	m := &vttv1.ClientCommand{RequestId: "r",
		Command: &vttv1.ClientCommand_AddNarration{AddNarration: an}}
	j, err := protojson.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	got := &vttv1.ClientCommand{}
	if err := protojson.Unmarshal([]byte(`{"requestId":"r","addNarration":`+
		`{"text":"hi","actorId":"ada","outOfCharacter":true}}`),
		got); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(m, got) {
		t.Fatalf("json %s decoded to %v", j, got)
	}
	for _, k := range []string{`"actorId"`, `"outOfCharacter"`} {
		if !bytes.Contains(j, []byte(k)) {
			t.Fatalf("%s missing from %s", k, j)
		}
	}
}

// D1: "protojson omitting a false bool".
func TestQANarrFalseAndEmptyAreOmitted(t *testing.T) {
	cases := []struct {
		m    proto.Message
		keys []string
	}{
		{&vttv1.AddNarration{Text: "x"}, []string{"outOfCharacter", "actorId"}},
		{&vttv1.NarrationAdded{Text: "x", As: "Ada"}, []string{"outOfCharacter"}},
	}
	for _, c := range cases {
		j, err := protojson.Marshal(c.m)
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range c.keys {
			if bytes.Contains(j, []byte(k)) {
				t.Fatalf("%s present in %s", k, j)
			}
		}
	}
}

// D1: every NarrationAdded already in a log "reads as not out of
// character"; the event is a fixture recorded before the field existed.
func TestQANarrOldMessagesDecodeAsNotOutOfCharacter(t *testing.T) {
	raw, err := os.ReadFile("testdata/narration_added_envelope.json")
	if err != nil {
		t.Fatal(err)
	}
	env := &vttv1.Envelope{}
	if err := protojson.Unmarshal(raw, env); err != nil {
		t.Fatal(err)
	}
	n := env.GetNarrationAdded()
	if n.GetOutOfCharacter() || n.GetAs() != "Goblin Cutter" {
		t.Fatalf("got %v", n)
	}
	cmd := &vttv1.AddNarration{}
	if err := protojson.Unmarshal([]byte(`{"text":"t","as":"x"}`), cmd); err != nil {
		t.Fatal(err)
	}
	if cmd.ActorId != "" || cmd.OutOfCharacter {
		t.Fatalf("got %v", cmd)
	}
}

// D1: the speaker's name stays in `as`; the event carries the mark
// and the name.
func TestQANarrEventCarriesMarkAndNameInAs(t *testing.T) {
	in := `{"narrationAdded":{"text":"brb","as":"Ada",` +
		`"outOfCharacter":true}}`
	env := &vttv1.Envelope{}
	if err := protojson.Unmarshal([]byte(in), env); err != nil {
		t.Fatal(err)
	}
	n := env.GetNarrationAdded()
	if !n.GetOutOfCharacter() || n.GetAs() != "Ada" {
		t.Fatalf("got %v", n)
	}
}

// VTT-505
func TestQANarrEventHasNoActorId(t *testing.T) {
	fields := (&vttv1.NarrationAdded{}).ProtoReflect().Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		if name := string(fields.Get(i).Name()); strings.Contains(name, "actor") {
			t.Fatalf("NarrationAdded carries %s", name)
		}
	}
	in := []byte(`{"text":"x","actorId":"a"}`)
	if err := protojson.Unmarshal(in, &vttv1.NarrationAdded{}); err == nil {
		t.Fatal("event accepted actorId")
	}
}

// D1: "None is proto3 `optional`".
func TestQANarrNewFieldsAreNotProto3Optional(t *testing.T) {
	cf := (&vttv1.AddNarration{}).ProtoReflect().Descriptor().Fields()
	ef := (&vttv1.NarrationAdded{}).ProtoReflect().Descriptor().Fields()
	for _, f := range []protoreflect.FieldDescriptor{
		cf.ByName("actor_id"), cf.ByName("out_of_character"),
		ef.ByName("out_of_character")} {
		if f.HasOptionalKeyword() {
			t.Fatalf("field %d is optional", f.Number())
		}
	}
}
