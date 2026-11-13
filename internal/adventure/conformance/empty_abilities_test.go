package conformance_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/adventure/conformance"
)

// Keep an actor whose file names an empty list matching a golden that leaves
// the key out, as DumpCompiledBatch writes it.
func TestAnActorNamingAnEmptyListMatchesAGoldenWithoutTheKey(t *testing.T) {
	dir := qaDataConformanceCopy(t, "cellar-rats")
	qaDataRewrite(t, filepath.Join(dir, "actors", "hollis.json"), func(b []byte) ([]byte, error) {
		return qaDataEdit(b, []string{"ability_ids"}, func(json.RawMessage) (json.RawMessage, bool) {
			return json.RawMessage("[]"), true
		})
	})
	qaDataEditGoldenActor(t, dir, "hollis", func(json.RawMessage) (json.RawMessage, bool) { return nil, false })
	if err := conformance.Run(dir, filepath.Join(qaDataRoot(t), "rulesets")); err != nil {
		t.Fatalf("an actor naming an empty list was refused against a golden without the key: %v", err)
	}
}
