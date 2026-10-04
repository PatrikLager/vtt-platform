package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// VTT-265
func TestTheMoveToolStatesTheFoldsReasonBound(t *testing.T) {
	raw, err := os.ReadFile("../../contract/gen/tools/tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var tools []struct {
		Name        string `json:"name"`
		InputSchema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("At most %d bytes", maxMoveReasonBytes)
	for _, tl := range tools {
		if tl.Name != "move_token" {
			continue
		}
		if desc := tl.InputSchema.Properties["reason"].Description; !strings.Contains(desc, want) {
			t.Fatalf("move_token's reason description = %q, want it to state %q", desc, want)
		}
		return
	}
	t.Fatal("tools.json has no move_token tool")
}
