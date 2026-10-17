package campaign

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// FoldPrefix returns the state a prefix of a log produced: foldEvents, the one
// fold, for a caller outside the package. Add no second loop that applies
// events anywhere instead (CLAUDE.md rule 4, VTT-323).
func FoldPrefix(events []*vttv1.Envelope) (*engine.State, error) {
	return foldEvents(events, nil)
}
