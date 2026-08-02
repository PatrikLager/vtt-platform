package contract_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

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

func TestTokenMovedRoundTrip(t *testing.T)  { roundTrip(t, "token_moved.json", &vttv1.TokenMoved{}) }
func TestAttackRolledRoundTrip(t *testing.T) { roundTrip(t, "attack_rolled.json", &vttv1.AttackRolled{}) }
func TestActorRoundTrip(t *testing.T)        { roundTrip(t, "actor.json", &vttv1.Actor{}) }
func TestMoveTokenRequestRoundTrip(t *testing.T) {
	roundTrip(t, "move_token_request.json", &vttv1.MoveTokenRequest{})
}
func TestEnvelopeRoundTrip(t *testing.T) { roundTrip(t, "envelope.json", &vttv1.Envelope{}) }

func TestSceneEnvelopeRoundTrip(t *testing.T) { roundTrip(t, "scene_envelope.json", &vttv1.Envelope{}) }
func TestRetractionEnvelopeRoundTrip(t *testing.T) {
	roundTrip(t, "retraction_envelope.json", &vttv1.Envelope{})
}
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
