package mapdef_test

// apidoc_test.go pins the two facts Task 10 exists to guarantee (maps-as-
// geometry spec §1.5 — "An LLM handed the format document and a pack
// manifest, with no other help, authors a map that loads and plays"):
//
//  1. docs/map-format.md — the API document itself — actually names every
//     tile in the standard vocabulary. If it drifts from standard.go, the
//     LLM that reads it authors maps referencing tiles it cannot see are
//     real, or never learns about ones that are — and nothing else in this
//     suite would notice, because the doc is prose, not code.
//  2. maps/cellar — the fixture the NEXT arc (line of sight) depends on —
//     actually loads AND actually has cover. goblin-ambush's failure (spec
//     §1.3) was a 32x32 field with nothing to hide behind; a map with no
//     blocking object would repeat it silently.

import (
	"os"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

func TestEveryStandardTileIsDocumented(t *testing.T) {
	doc, err := os.ReadFile("../../docs/map-format.md")
	if err != nil {
		t.Fatal(err)
	}
	for name := range mapdef.StandardTileNames() {
		if !strings.Contains(string(doc), "`"+name+"`") {
			t.Fatalf("standard tile %q is not in docs/map-format.md — an author "+
				"reading the docs cannot know it exists", name)
		}
	}
}

func TestTheDemoMapLoadsAndHasCover(t *testing.T) {
	m, err := mapdef.Load("../../maps/cellar/map.json")
	if err != nil {
		t.Fatalf("the demo map does not load: %v", err)
	}
	var blockers int
	for _, o := range m.Objects {
		if o.BlocksSight {
			blockers++
		}
	}
	if blockers == 0 {
		t.Fatal("the demo map has no cover — the visibility arc would have " +
			"nothing to hide behind, which is how goblin-ambush failed")
	}
}
