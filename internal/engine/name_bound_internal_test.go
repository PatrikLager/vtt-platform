package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// VTT-276
func TestTheToolsStateTheFoldsNameBound(t *testing.T) {
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
	want := fmt.Sprintf("At most %d bytes", maxNameBytes)
	found := 0
	for _, tl := range tools {
		var desc string
		switch tl.Name {
		case "add_actor":
			desc = tl.InputSchema.Properties["actor"].Properties["name"].Description
		case "start_session":
			desc = tl.InputSchema.Properties["name"].Description
		default:
			continue
		}
		found++
		if !strings.Contains(desc, want) {
			t.Errorf("%s's name description = %q, want it to state %q", tl.Name, desc, want)
		}
	}
	if found != 2 {
		t.Fatalf("tools.json holds %d of add_actor and start_session, want both", found)
	}
}
