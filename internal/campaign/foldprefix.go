package campaign

import (
	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// FoldPrefix returns the state a PREFIX of the log produced.
//
// It is foldEvents — the one fold rebuildLocked runs on open — reached from
// outside the package, and that is the whole of why it exists: so the gateway
// does NOT grow a second event-application loop (CLAUDE.md rule 4). The
// visibility projection needs the state each event produced, one event at a
// time, and State() cannot answer that: State() is head, and a seat being
// replayed from the beginning must judge every event against the world as it
// stood then.
//
// ONE PASS SINCE 2026-08-31, and the second pass is worth naming because it
// carried the old argument for this shape. A retraction was retroactive — the
// marker landed at head and removed a range folded long before it — so the
// fold had to learn the whole retracted set before applying anything, and a
// function over a SLICE was the only way to do that. Retraction has left the
// platform (spec 2026-08-30-retraction-leaves) and no envelope changes what an
// earlier PREFIX produced, so the slice is no longer load-bearing for
// correctness. Rule 4 is what is left holding this function up, and it holds it
// alone.
//
// COST, STATED RATHER THAN HIDDEN, because a hidden one is the kind nobody
// notices starting to matter: a caller that folds every prefix in turn does
// O(n^2) work over a log of n events, and seat.receive is exactly that caller.
// What the O(n^2) buys is now one thing — the gateway reads the state after
// each event without keeping its own copy of the loop that produces it,
// unknown-variant tolerance and corrupt-log wrapping included. It is not the
// dominant term in that caller: the gateway recomputes line of sight per event
// too, at 15-176 ms per eye (visibility spec §8) against roughly a microsecond
// per engine.Apply.
//
// THE CHEAP SHAPE IS AVAILABLE NOW AND IS DELIBERATELY NOT TAKEN, which is a
// different sentence from the one this comment used to end on. With nothing
// retroactive, a seat could hold one engine.State and advance it by a single
// engine.Apply per event — O(n), and in exact agreement with this function.
// That is a change to how the gateway keeps a seat rather than anything in this
// file, it is out of scope for retraction's removal, and it is likely moot: the
// per-character-logs design (spec 2026-08-30-per-character-logs §7) deletes the
// read-time projection and the seat's projector, which between them are this
// function's only non-test caller.
func FoldPrefix(events []*vttv1.Envelope) (*engine.State, error) {
	return foldEvents(events)
}
