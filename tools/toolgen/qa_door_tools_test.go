package main

import (
	"encoding/json"
	"os"
	"testing"
)

func qaDoorTool(t *testing.T, name string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../cmd/vtt/tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	for _, tl := range list {
		if tl["name"] == name {
			return tl["inputSchema"].(map[string]any)
		}
	}
	t.Fatalf("tool %s not served", name)
	return nil
}

func qaDoorCheck(t *testing.T, name string) {
	t.Helper()
	schema := qaDoorTool(t, name)
	props := schema["properties"].(map[string]any)
	a, ok := props["actorId"].(map[string]any)
	if !ok || a["type"] != "string" {
		t.Errorf("%s: actorId property = %v", name, props["actorId"])
	}
	req := map[string]bool{}
	for _, r := range schema["required"].([]any) {
		req[r.(string)] = true
	}
	if req["actorId"] {
		t.Errorf("%s: actorId is required", name)
	}
	if !req["sceneId"] || !req["at"] {
		t.Errorf("%s: required = %v, want sceneId and at", name, req)
	}
}

// VTT-877
func TestQADoorOpenDoorToolTakesAnActorIdWithoutRequiringIt(t *testing.T) {
	qaDoorCheck(t, "open_door")
}

// VTT-877
func TestQADoorCloseDoorToolTakesAnActorIdWithoutRequiringIt(t *testing.T) {
	qaDoorCheck(t, "close_door")
}
