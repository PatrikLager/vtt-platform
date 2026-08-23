package campaign

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// FoldPrefix returns the state a PREFIX of the log produced.
//
// It is foldEvents — the one fold rebuildLocked and Undo's viability check
// already share — reached from outside the package, and it exists so that the
// gateway does NOT grow a second event-application loop (CLAUDE.md rule 4).
// The visibility projection needs the state each event produced, one event at
// a time, and State() cannot answer that: State() is head, and a seat being
// replayed from the beginning must judge every event against the world as it
// stood then.
//
// TWO PASSES, and that is the whole reason this is a function over a slice
// rather than an Apply-as-you-go. A retraction is retroactive: the marker
// lands at head and removes a range folded long before it, so a fold that has
// already applied the retracted event cannot take it back. foldEvents collects
// the retracted set from the whole slice first and only then applies anything;
// TestFoldPrefixHonoursARetractionInsideThePrefix fails against the one-pass
// version.
//
// COST, stated rather than hidden: a caller folding every prefix in turn does
// O(n^2) work over a log of n events. That is deliberate — it buys exact
// agreement with the live projection, retraction included, for no new
// algorithm — and it is not the dominant term in the caller that needed it:
// the gateway recomputes line of sight per event too, at 15-176 ms per eye
// (visibility spec §8) against roughly a microsecond per engine.Apply. If it
// ever does dominate, the shape that replaces it must still answer retraction
// by re-folding, which is what the test above is really pinning.
func FoldPrefix(events []*vttv1.Envelope) (*engine.State, error) {
	return foldEvents(events, retractedSet(events))
}
