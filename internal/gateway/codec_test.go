package gateway_test

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
)

func TestDecodeCommandRoundTrip(t *testing.T) {
	original := &vttv1.ClientCommand{
		RequestId: "req-1",
		Command: &vttv1.ClientCommand_MoveToken{
			MoveToken: &vttv1.MoveTokenRequest{TokenId: "t1", To: &vttv1.GridPosition{X: 3, Y: 4}},
		},
	}
	raw, err := protojson.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	got, err := gateway.DecodeCommand(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(original, got) {
		t.Fatalf("round trip mismatch: got %v, want %v", got, original)
	}
}

func TestDecodeCommandMalformedJSONErrorsCleanly(t *testing.T) {
	cmd, err := gateway.DecodeCommand([]byte("{not valid json"))
	if err == nil {
		t.Fatal("want error for malformed JSON")
	}
	// AND NOTHING ALONGSIDE IT. This half was unasserted until 2026-08-27:
	// the command was discarded into `_`, so `return &cmd, err` on the failure
	// path passed the whole suite. serve() checks only the error
	// (server.go:859), which is the correct shape and exactly what makes the
	// other half unguarded — a decoder that returned both would hand a caller
	// a half-populated command it had already refused, and protojson populates
	// as it goes, so "refused" does not mean "empty".
	if cmd != nil {
		t.Fatalf("decode refused the frame AND returned a command %v — a caller "+
			"checking only one of the two would act on a frame the decoder rejected", cmd)
	}
}

func TestEncodeFrameResultArmRoundTrips(t *testing.T) {
	frame := &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Result{
		Result: &vttv1.CommandResult{RequestId: "req-1", Ok: true, Sequence: 12},
	}}

	raw, err := gateway.EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	var back vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(frame, &back) {
		t.Fatalf("round trip mismatch: got %v, want %v", &back, frame)
	}
}

func TestEncodeFrameEventArmRoundTrips(t *testing.T) {
	frame := &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{
		Event: &vttv1.Envelope{
			EventId:       "e1",
			Sequence:      1,
			ActorRole:     "dm",
			ParticipantId: "p-1",
			OccurredAt:    timestamppb.Now(),
			Payload: &vttv1.Envelope_SessionStarted{
				SessionStarted: &vttv1.SessionStarted{Name: "n"},
			},
		},
	}}

	raw, err := gateway.EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	var back vttv1.ServerFrame
	if err := protojson.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(frame, &back) {
		t.Fatalf("round trip mismatch: got %v, want %v", &back, frame)
	}
}
