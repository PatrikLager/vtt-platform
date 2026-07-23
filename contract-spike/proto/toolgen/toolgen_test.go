package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestToolgenMatchesExpectedTool(t *testing.T) {
	got := buildTool()
	raw, err := os.ReadFile("../../fixtures/expected_tool.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(got)
	var gotMap map[string]any
	_ = json.Unmarshal(gotJSON, &gotMap)
	if !reflect.DeepEqual(want, gotMap) {
		t.Fatalf("tool mismatch:\nwant %v\ngot  %v", want, gotMap)
	}
}
