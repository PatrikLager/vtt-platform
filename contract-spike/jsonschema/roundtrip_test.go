package jsonschema_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	jsgen "github.com/PatrikLager/vtt-platform/contract-spike/jsonschema/gen/go"
)

func roundTrip[T any](t *testing.T, fixture string) {
	t.Helper()
	raw, err := os.ReadFile("../fixtures/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	var typed T
	if err := json.Unmarshal(raw, &typed); err != nil {
		t.Fatalf("unmarshal %s: %v", fixture, err)
	}
	out, err := json.Marshal(typed)
	if err != nil {
		t.Fatal(err)
	}
	var want, got map[string]any
	_ = json.Unmarshal(raw, &want)
	_ = json.Unmarshal(out, &got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("%s mismatch:\nwant %v\ngot  %v", fixture, want, got)
	}
}

func TestTokenMoved(t *testing.T)       { roundTrip[jsgen.TokenMoved](t, "token_moved.json") }
func TestAttackRolled(t *testing.T)     { roundTrip[jsgen.AttackRolled](t, "attack_rolled.json") }
func TestActor(t *testing.T)            { roundTrip[jsgen.Actor](t, "actor.json") }
func TestMoveTokenRequest(t *testing.T) { roundTrip[jsgen.MoveTokenRequest](t, "move_token_request.json") }
