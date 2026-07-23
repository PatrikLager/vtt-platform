package proto_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	contractv1 "github.com/PatrikLager/vtt-platform/contract-spike/proto/gen/go/contract/v1"
)

// roundTrip unmarshals a fixture into msg via protojson, marshals it back,
// and compares semantically (protojson output ordering is not stable text).
func roundTrip(t *testing.T, fixture string, msg proto.Message) {
	t.Helper()
	raw, err := os.ReadFile("../fixtures/" + fixture)
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

func TestTokenMovedRoundTrip(t *testing.T)  { roundTrip(t, "token_moved.json", &contractv1.TokenMoved{}) }
func TestAttackRolledRoundTrip(t *testing.T) { roundTrip(t, "attack_rolled.json", &contractv1.AttackRolled{}) }
func TestActorRoundTrip(t *testing.T)        { roundTrip(t, "actor.json", &contractv1.Actor{}) }
func TestMoveTokenRequestRoundTrip(t *testing.T) {
	roundTrip(t, "move_token_request.json", &contractv1.MoveTokenRequest{})
}
