package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func qaT6ToolOf(t *testing.T, name string) (desc, actorDesc string, required []string, hasActor bool) {
	t.Helper()
	raw, err := os.ReadFile("../../cmd/vtt/tools.json")
	if err != nil {
		t.Fatal(err)
	}
	var tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema struct {
			Properties map[string]struct {
				Type        string `json:"type"`
				Description string `json:"description"`
			} `json:"properties"`
			Required []string `json:"required"`
		} `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatal(err)
	}
	for _, tl := range tools {
		if tl.Name == name {
			p, ok := tl.InputSchema.Properties["actorId"]
			if ok && p.Type != "string" {
				t.Fatalf("%s actorId type %q", name, p.Type)
			}
			return tl.Description, p.Description, tl.InputSchema.Required, ok
		}
	}
	t.Fatalf("tool %s not served", name)
	return
}

// VTT-877
func TestQADoorToolTextSchemaHasOptionalActorId(t *testing.T) {
	for _, n := range []string{"open_door", "close_door"} {
		_, _, req, has := qaT6ToolOf(t, n)
		if !has {
			t.Errorf("%s has no actorId", n)
		}
		for _, r := range req {
			if r == "actorId" {
				t.Errorf("%s requires actorId", n)
			}
		}
	}
}

// VTT-878
func TestQADoorToolTextDescriptionsSayWhoMayBeNamed(t *testing.T) {
	for _, n := range []string{"open_door", "close_door"} {
		d, a, _, _ := qaT6ToolOf(t, n)
		for _, want := range []string{"actorId names the actor", "any actor the campaign holds", "or none",
			"An actor the campaign does not hold is refused", "There is no such character."} {
			if !strings.Contains(d, want) {
				t.Errorf("%s description lacks %q", n, want)
			}
		}
		for _, want := range []string{"Optional", "omit for none", "any actor the campaign holds",
			"does not hold refuses the command"} {
			if !strings.Contains(a, want) {
				t.Errorf("%s actorId description lacks %q", n, want)
			}
		}
	}
	o, _, _, _ := qaT6ToolOf(t, "open_door")
	c, _, _, _ := qaT6ToolOf(t, "close_door")
	if !strings.Contains(o, "opens it") || !strings.Contains(c, "closes it") {
		t.Errorf("verbs: open %q close %q", o, c)
	}
}
