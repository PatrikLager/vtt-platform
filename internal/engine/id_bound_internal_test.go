package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// VTT-281
func TestTheToolsStateTheFoldsIDBound(t *testing.T) {
	raw, err := os.ReadFile("../../contract/gen/tools/tools.json")
	if err != nil {
		t.Fatal(err)
	}
	type prop struct {
		Description string          `json:"description"`
		Properties  map[string]prop `json:"properties"`
	}
	var tools []struct {
		Name        string `json:"name"`
		InputSchema prop   `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("At most %d bytes", maxIDBytes)
	found := 0
	for _, tl := range tools {
		var desc string
		switch tl.Name {
		case "add_actor":
			desc = tl.InputSchema.Properties["actor"].Properties["actorId"].Description
		case "place_token":
			desc = tl.InputSchema.Properties["tokenId"].Description
		default:
			continue
		}
		found++
		if !strings.Contains(desc, want) {
			t.Errorf("%s's id description = %q, want it to state %q", tl.Name, desc, want)
		}
	}
	if found != 2 {
		t.Fatalf("tools.json holds %d of add_actor and place_token, want both", found)
	}
}
