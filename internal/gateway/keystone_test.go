package gateway_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/sight"
)

// --- the keystone (visibility spec §4.3) -----------------------------------
//
// WHY THIS FILE LIVES IN internal/gateway AND NOT internal/harness, since the
// plan named `internal/harness/projection_golden_test.go`: the P1 boundary
// forbids it. .go-arch-lint.yml gives harness `mayDependOn: [contract, engine,
// harness]` — "store/campaign/gateway/identity are forbidden dependencies" —
// and go-arch-lint checks test files (four files in the tree are excluded BY
// NAME for exactly this reason). The keystone needs gateway.Projector for its
// left-hand side and internal/sight for its right-hand side, and only the
// gateway component may reach both. Moving the corpus walker here rather than
// adding a dependency edge is the direction that does not weaken a gate.
//
// The walker below is therefore a THIRD reader of scenarios/goldens/*, beside
// internal/harness's TestFoldGoldenCorpus and cmd/vtt's
// TestScenarioGoldenStreamsHaveNotDrifted, and it keeps their shape
// deliberately: DIRECTORIES, not a plain glob, because the corpus README would
// otherwise be read as a scenario (that failure has already happened once —
// see goldenDirs' own comment in either of those files).

// keystoneGolden is one corpus scenario's LOG — the events the server actually
// wrote, before any viewer's projection touched them. It is the `log` in
// §4.3's equation and it is the ONLY input this file reads from disk; both
// sides of the equation are computed from it here.
type keystoneGolden struct {
	name string
	dir  string
	log  []*vttv1.Envelope
}

// keystoneSeat is one viewer plus the name failures are reported under.
type keystoneSeat struct {
	name   string
	viewer gateway.Viewer
}

// oracleView is the RIGHT-HAND SIDE of §4.3: what the server believes this
// viewer can see, as a function of state alone.
//
// A PLAIN FUNCTION OF (state, viewer) WITH NO MEMORY, where gateway.Projector
// is a FOLD — five accumulator maps over the log prefix, by its own doc
// comment. So the two cannot share a bug in the accumulator, and the
// accumulator is where the path-dependent decisions live: what has already been
// introduced, what is on the board right now, what was last reported visible.
//
// THAT IS A NARROWER CLAIM THAN "not a second copy of the thing under test",
// which is what this comment said until a review pushed back on it. See
// visibleState's own INDEPENDENCE section for exactly how far it reaches and
// where it stops.
type oracleView struct {
	// squares is scene id -> visible square keys, with an entry for every
	// scene this viewer has an eye standing in (even one it can see nothing
	// of — spec §4.2: "you are not in a scene" and "you are in a scene you
	// cannot see" are different things).
	squares map[string]map[string]bool
	// tokens is the token ids standing on a visible square, with where each one
	// actually stands. Creatures are pure line of sight (spec §3.2); nothing
	// here is remembered.
	tokens map[string]engine.Token
	// actors is the actor ids this viewer is entitled to know of.
	actors map[string]bool
}

// visibleState is `visibleState(fold(log), viewer)` — the oracle §4.3 requires,
// DERIVED FROM engine.State AND internal/sight AND NOTHING ELSE.
//
// INDEPENDENCE IS THE WHOLE POINT, and it is worth stating exactly what is and
// is not independent here, because "independent" is the claim a reader a year
// from now has to be able to check:
//
//   - It calls NOTHING in internal/gateway's projection. Not look, not
//     classify, not transitions, not sceneSeenFor, not Projector's maps. If it
//     did, the equation would be a tautology that holds however wrong the
//     projection is: two derivations sharing an implementation agree about
//     their shared bug.
//   - It DOES call internal/sight, deliberately, because §4.3 says the
//     right-hand side is computed "with the sight test over engine.State".
//     sight is pure geometry with its own boundary tests
//     (internal/sight/sight_test.go); sharing it means this test measures the
//     PROJECTION rather than re-litigating the geometry. sight.VisibleFrom
//     consults BOTH blocker sources — wall/closed-door entries in Tiles AND
//     objects carrying blocks_sight (see sight.Blockers) — so a scene with no
//     terrain at all can still be shadowed, and this oracle inherits that for
//     free by asking VisibleFrom rather than reasoning about tiles.
//   - THE RESIDUAL IS BIGGER THAN "a rule both sides read wrong", and pretending
//     otherwise would be the exact self-deception §4.3 warns about. The rules
//     below ARE, statement for statement, what project.go's look() and eyes()
//     do — same eye loop, same nil-map-index idiom, same unconditional
//     player-controlled pass. Nothing calls into the projection, so §4.3's rule
//     is kept; but a bug INSIDE look() would move both sides of the equation
//     together and this test would stay green. What is genuinely independent is
//     the Projector's fold — its five maps, every introduction decision,
//     classify, transitions, sceneSeenFor, and both language folds — and that is
//     the great majority of the code the arc added.
//
//     WHERE THE REST OF THE INDEPENDENCE LIVES is the hand-derived
//     scenarios/goldens/*/projections/*/state.json, which no machine produced
//     and which look() had no hand in: a human wrote 36 squares down from the
//     scene's geometry and both folds have to land on them.
//     TestTheProjectedGoldensAreWhatTheProjectionActuallySends is what holds the
//     projection to those files, and it is the check that survives a wrong
//     look(). Rewriting the rules below to be structurally UNLIKE look() would
//     make this comment easier to write and the test no stronger — the honest
//     answer is that the oracle covers the fold and the fixtures cover the
//     geometry.
//
//     Each rule cites the spec section it comes from, so the check a reader can
//     perform is against the spec and not against project.go.
//
// The rules, each from the spec and not from the implementation:
//
//   - §3.1 whose eyes: a player sees through the union of the actors they
//     control; a spectator through the one shoulder they are riding, and that
//     shoulder must be a PLAYER-CONTROLLED actor (§3.1.1 — "a spectator
//     perched on the Goblin Archer would watch the ambush from inside it").
//   - §3.4 sight range is an INPUT and is not supplied by the platform, so the
//     oracle asks for the same "not supplied" the projection does: unlimited
//     range, one exposed sample point of nine.
//   - §3.2 creatures are not remembered: a token is visible iff it stands on a
//     currently visible square.
//   - §5 the actor roster: an NPC becomes knowable on first sight, and actors
//     controlled by ANY player are always known ("you know your party exists
//     when the rogue is two rooms away").
func visibleState(st *engine.State, v gateway.Viewer) oracleView {
	out := oracleView{
		squares: map[string]map[string]bool{},
		tokens:  map[string]engine.Token{},
		actors:  map[string]bool{},
	}
	if st == nil {
		return out
	}

	for _, eye := range oracleEyes(st, v) {
		for _, tok := range st.Tokens {
			if tok.ActorID != eye {
				continue
			}
			sc, ok := st.Scenes[tok.SceneID]
			if !ok {
				continue
			}
			seen := out.squares[tok.SceneID]
			if seen == nil {
				seen = map[string]bool{}
				out.squares[tok.SceneID] = seen
			}
			for sq := range sight.VisibleFrom(sc, tok.X, tok.Y, 0, 0) {
				seen[sq] = true
			}
		}
	}

	for id, tok := range st.Tokens {
		if out.squares[tok.SceneID][oracleSquareKey(tok.X, tok.Y)] {
			out.tokens[id] = tok
			out.actors[tok.ActorID] = true
		}
	}
	for id, a := range st.Actors {
		if len(a.GetControllerIds()) > 0 {
			out.actors[id] = true
		}
	}
	return out
}

// oracleEyes are the actors this viewer looks through, per spec §3.1/§3.1.1.
//
// The DM and the agent are absent on purpose: they receive the log itself, so
// "what can they see" is not a sight question and this file answers it with a
// different comparison entirely (see the identity arm of the keystone).
func oracleEyes(st *engine.State, v gateway.Viewer) []string {
	switch v.Role {
	case identity.RolePlayer:
		var ids []string
		for id, a := range st.Actors {
			for _, c := range a.GetControllerIds() {
				if c == v.ParticipantID {
					ids = append(ids, id)
					break
				}
			}
		}
		sort.Strings(ids)
		return ids
	case identity.RoleSpectator:
		a, ok := st.Actors[v.Viewpoint]
		if !ok || len(a.GetControllerIds()) == 0 {
			// An unknown actor, the empty id (nobody perched yet), and an NPC
			// all mean the same thing: no eyes at all. §3.1.1's refusal.
			return nil
		}
		return []string{v.Viewpoint}
	}
	return nil
}

// oracleSquareKey formats a square the way the wire does (maps-as-geometry spec
// §4.1). Spelled out rather than imported because every package that needs it
// keeps its own — engine.gridKey, sight.squareKey and gateway.squareKey are all
// unexported, and this package's tests are an EXTERNAL package by design.
func oracleSquareKey(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

// TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees is the keystone
// (visibility spec §4.3):
//
//	fold(project(log, viewer)) == visibleState(fold(log), viewer)
//
// Disagreement in EITHER direction is a defect: a player seeing a goblin they
// should not, or missing one they should. Both directions are asserted, and both
// were proved by fault injection — four over-send faults and two under-send
// ones, each applied to project.go, run, and reverted, with the transcripts in
// .superpowers/sdd/2026-08-18-visibility/task-8-report.md §5. ADR-009 requires
// that evidence for a keystone written after the code, and a doc comment is not
// where it lives.
//
// IT RUNS AT EVERY PREFIX (§4.3 as amended 2026-08-22), not only over the final
// state. A leak that appears and is then covered by later events is invisible to
// a final-state check and is exactly what a prefix-wise one catches. The left
// side is built the way a real seat builds it — one Projector fed the log from
// the beginning, judging each event against campaign.FoldPrefix of the log so
// far, which is what internal/gateway/seat.go's receive does — so retraction,
// introductions and the projector's memory are all exercised rather than
// stepped around.
//
// EXPLORED IS EXCLUDED FROM THE COMPARISON, AND IMPLIED IN ONE DIRECTION — the
// one that matters. Stated here and not only in the spec, because "excluded
// because it is path-dependent and implied by the prefix-wise result" and
// "excluded because we could not make it pass" look identical in a diff a year
// from now:
//
//   - Explored is terrain MEMORY. It is unioned from each SceneSeen's TILES
//     KEYS — not from its `visible` set; different parts of the message, see
//     the SceneSeen arms in internal/engine/apply.go and client/src/fold.ts —
//     and SceneSeen exists only in projections. So folding a real log leaves it
//     empty on every scene while folding that log's projection leaves it
//     populated, and no final-state oracle can close that gap.
//   - sceneSeenFor only ever builds `tiles` from the squares it has just
//     decided are visible, so per message `tiles ⊆ visible`. That SUBSET
//     relation — not an identity, and an earlier draft of the spec's amendment
//     had it as one — is what the exclusion rests on: NOTHING CAN BE REMEMBERED
//     THAT WAS NEVER VISIBLE. Verifying Visible at every prefix therefore bounds
//     Explored from ABOVE, and a wrongly remembered square must first have
//     leaked as a currently-visible one at some prefix, where this test catches
//     it.
//   - It does NOT bound Explored from below, and that is correct rather than a
//     gap: a visible square carrying no terrain is deliberately never
//     remembered, because there is nothing there to remember. On a bare-canvas
//     scene Explored stays empty however much is visible — which is why the
//     corpus carries one of each (scenarios/goldens/session-zero has a tiled
//     scene and a terrain-free one side by side).
//
// TWO FIELDS ARE COMPARED AS BOUNDS RATHER THAN EQUALITIES, and for the same
// path-dependence reason rather than for convenience:
//
//   - THE ACTOR ROSTER NEVER FORGETS. An NPC seen at seq 7 and hidden at seq 8
//     is still in the viewer's roster at 8 (TokenHidden removes a token, not an
//     actor), while any function of state-at-8 says it is unknown —
//     internal/gateway/seat.go's own doc comment says so and Task 4 proved it.
//     So the oracle's answer is the LOWER bound (nothing the viewer is entitled
//     to may be missing) and the union of every prefix's answer is the UPPER
//     bound (nothing may appear that the viewer was never entitled to at any
//     point). An introduction one event early fails the upper bound.
//   - A SCENE IS NEVER UN-INTRODUCED either, for the same reason plus one more:
//     a scene the viewer has walked out of stays on their board, dimmed, under
//     §3.2's terrain memory. Its Visible must be EMPTY there — the projection
//     emits an empty SceneSeen when a scene goes dark — and the scene itself
//     must be one an eye stood in at some prefix.
func TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees(t *testing.T) {
	for _, g := range keystoneCorpus(t) {
		for _, seat := range keystoneSeats(t, g) {
			t.Run(g.name+"/"+seat.name, func(t *testing.T) {
				walkKeystone(t, g, seat)
			})
		}
	}
}

// walkKeystone folds one golden through one seat, checking §4.3 after every
// event.
func walkKeystone(t *testing.T, g keystoneGolden, seat keystoneSeat) {
	t.Helper()

	pr := gateway.NewProjector(seat.viewer)
	identityProjection := seat.viewer.Role == identity.RoleDM || seat.viewer.Role == identity.RoleAgent

	var received, projected []*vttv1.Envelope
	everKnownActors := map[string]bool{}
	everSeenScenes := map[string]bool{}
	everSeenSquares := map[string]map[string]bool{}

	for i, env := range g.log {
		received = append(received, env)
		// The state AFTER env, which is what Project is specified to read, and
		// the same call seat.receive makes for the same reason: State() is head,
		// and during a replay head is the future.
		world, err := campaign.FoldPrefix(received)
		if err != nil {
			t.Fatalf("prefix %d (seq %d): folding the LOG failed, so the corpus is not a valid input: %v",
				i+1, env.GetSequence(), err)
		}

		out := pr.Project(env, world)
		if identityProjection {
			// Exit criterion 8: the DM's stream is byte-for-byte unchanged and
			// the agent seat notices nothing. Pointer identity is the strongest
			// available statement of that and is what Project promises.
			if len(out) != 1 || out[0] != env {
				t.Fatalf("prefix %d (seq %d): the identity projection must return the event itself, "+
					"by pointer; got %d envelope(s)", i+1, env.GetSequence(), len(out))
			}
		}
		projected = append(projected, out...)

		// campaign.FoldPrefix is engine.Apply under the same two-pass retraction
		// semantics internal/harness.Fold and client/src/fold.ts implement (see
		// FoldPrefix's own doc comment: it exists so the gateway does not grow a
		// second event-application loop). Folding the PROJECTION with it is the
		// left-hand side of §4.3.
		got, err := campaign.FoldPrefix(projected)
		if err != nil {
			t.Fatalf("prefix %d (seq %d): folding this seat's PROJECTION failed — a stream the "+
				"server sent that its own recipient cannot fold: %v", i+1, env.GetSequence(), err)
		}

		if identityProjection {
			// NOT A DUMP COMPARISON, deliberately. Comparing FoldPrefix(projected)
			// against FoldPrefix(received) here would be a tautology: the pointer
			// check above has just established that the two slices hold the same
			// envelopes, so folding them cannot disagree. What IS worth asserting
			// is the consequence for the two fields this arc added — the DM's
			// board must stay untouched by all of it (exit criterion 8).
			//
			// NIL, NOT EMPTY, and the distinction is the one engine.Scene's own
			// doc comment calls load-bearing: nil is "no projection ever arrived",
			// empty is "a projection arrived and this seat can see nothing here".
			// A renderer that conflated them would blank the DM's board.
			for id, sc := range got.Scenes {
				if sc.Visible != nil || sc.Explored != nil {
					t.Fatalf("prefix %d (seq %d): scene %q on an UNPROJECTED seat's board carries "+
						"Visible=%v Explored=%v; both must stay nil, because nothing in a log "+
						"produces SceneSeen and the DM receives the log",
						i+1, env.GetSequence(), id, sc.Visible, sc.Explored)
				}
			}
			if i == len(g.log)-1 {
				// AND AT THE END, THE CORPUS'S OWN HAND-DERIVED STATE. §4.3:
				// "today's scenarios/goldens/ corpus becomes the viewer = DM case,
				// where projection is identity" — so this is that sentence as an
				// assertion, and the DM's board is measured against a file a human
				// wrote from the scenario rather than against another fold.
				//
				// IT IS ALSO THE ONLY THING KEEPING keystoneDump AND
				// internal/harness's dumpJSON BYTE-IDENTICAL. They are duplicated
				// (see this file's header on why harness is out of reach) and
				// nothing structural ties them together; what ties them is that
				// this line and TestFoldGoldenCorpus now compare their output to
				// the SAME committed <golden>/state.json. Without it the two could
				// drift and neither would notice — the projected fixtures alone
				// would not catch it, because only keystoneDump is ever applied to
				// those.
				gotDump := keystoneDump(t, got, headSequenceOf(projected))
				wantDump, err := os.ReadFile(filepath.Join(g.dir, "state.json"))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(bytes.TrimSpace(gotDump), bytes.TrimSpace(wantDump)) {
					t.Errorf("the identity projection did not fold to the corpus's own hand-derived "+
						"state\n--- folded ---\n%s\n--- state.json ---\n%s", gotDump, wantDump)
				}
			}
			continue
		}

		want := visibleState(world, seat.viewer)
		for id := range want.actors {
			everKnownActors[id] = true
		}
		for id, squares := range want.squares {
			everSeenScenes[id] = true
			ever := everSeenSquares[id]
			if ever == nil {
				ever = map[string]bool{}
				everSeenSquares[id] = ever
			}
			for sq := range squares {
				ever[sq] = true
			}
		}

		for _, diff := range keystoneDiff(got, want, everKnownActors, everSeenScenes, everSeenSquares) {
			t.Errorf("prefix %d (seq %d): %s", i+1, env.GetSequence(), diff)
		}
		if t.Failed() {
			// The first divergent prefix is the informative one; everything
			// after it is that same divergence carried forward.
			return
		}
	}
}

// keystoneDiff is the comparison §4.3 specifies, in both directions. It returns
// one line per disagreement and an empty slice when the equation holds.
func keystoneDiff(got *engine.State, want oracleView, everKnownActors, everSeenScenes map[string]bool,
	everSeenSquares map[string]map[string]bool) []string {
	var out []string

	// --- tokens: an exact set, and an exact position ------------------------
	//
	// Creatures are pure line of sight (spec §3.2), so this is an equality in
	// both directions with nothing remembered on either side. The POSITION is
	// compared too: a token left on a stale square is a board that lies about
	// where a creature stands, which TokenMoved's visible-before-and-after rule
	// exists to prevent.
	for id, stands := range want.tokens {
		held, onBoard := got.Tokens[id]
		if !onBoard {
			out = append(out, fmt.Sprintf("token %q is visible to this seat but MISSING from its board "+
				"(the projection withheld a sighting the viewer is entitled to)", id))
			continue
		}
		if held != stands {
			out = append(out, fmt.Sprintf("token %q stands at %+v on this seat's board and at %+v in the world",
				id, held, stands))
		}
	}
	for id := range got.Tokens {
		if _, visible := want.tokens[id]; !visible {
			out = append(out, fmt.Sprintf("token %q is on this seat's board but NOT visible to it "+
				"(a creature leaked — this is session zero's defect)", id))
		}
	}

	// --- actors: bounded above and below, never equal ------------------------
	for id := range want.actors {
		if _, held := got.Actors[id]; !held {
			out = append(out, fmt.Sprintf("actor %q is knowable to this seat but MISSING from its roster", id))
		}
	}
	for id := range got.Actors {
		if !everKnownActors[id] {
			out = append(out, fmt.Sprintf("actor %q is on this seat's roster and has never been knowable "+
				"at any prefix (a roster leak — finding 14 one layer up)", id))
		}
	}

	// --- Visible: an exact set per scene, both directions --------------------
	for id, squares := range want.squares {
		sc, held := got.Scenes[id]
		if !held {
			out = append(out, fmt.Sprintf("scene %q is one this seat has an eye in but it holds no board for it", id))
			continue
		}
		if d := squareSetDiff(sc.Visible, squares); d != "" {
			out = append(out, fmt.Sprintf("scene %q Visible: %s", id, d))
		}
	}
	for id, sc := range got.Scenes {
		if _, inSight := want.squares[id]; inSight {
			continue
		}
		if !everSeenScenes[id] {
			out = append(out, fmt.Sprintf("scene %q is on this seat's board and no eye of its has ever "+
				"stood in it (exit criterion 6: a scene a player has never entered is absent entirely)", id))
			continue
		}
		if len(sc.Visible) != 0 {
			out = append(out, fmt.Sprintf("scene %q is out of this seat's sight but still reports %d visible "+
				"square(s) — the room stays lit forever", id, len(sc.Visible)))
		}
	}

	// --- terrain: bounded from ABOVE, which is §4.3's Explored argument turned
	// --- from prose into an assertion -----------------------------------------
	//
	// Explored and Tiles are NOT compared for equality — Explored is excluded,
	// and Tiles rides with it, because both are memory and no final state can
	// reconstruct either. But the amendment's justification for excluding them
	// is a BOUND, and a bound is checkable: `tiles ⊆ visible` per SceneSeen
	// message, so nothing can be remembered that was never visible. Both maps
	// are therefore held to the union of every visible set this seat's oracle
	// has produced so far.
	//
	// WITHOUT THIS THE EXCLUSION WOULD BE AN ARGUMENT AND NOT A TEST, and that
	// was MEASURED rather than reasoned. A projection that sent a scene's WHOLE
	// tile map on the viewer's first look changes no square of Visible, no token
	// and no actor — it is the §4.2 leak in its purest form, a player handed the
	// map of a room they have not walked into — and with this block disabled the
	// keystone passes it. Every other comparison above lets it through.
	//
	// The keystone is NOT the only thing that catches it, and saying so is the
	// difference between a measurement and a boast: making sceneSeenFor walk
	// sc.Tiles instead of the visible squares also reds seven existing tests in
	// this package, TestSceneSeenCarriesOnlyTheSquaresInSight among them. What
	// this block adds is that the keystone's own equation stops having a hole in
	// it.
	//
	// One direction only, deliberately. The lower bound does not hold and must
	// not be asserted: a visible square carrying no terrain is never remembered,
	// so on a bare canvas these stay empty however much is visible.
	for id, sc := range got.Scenes {
		ever := everSeenSquares[id]
		for _, remembered := range []struct {
			what string
			keys []string
		}{
			{"Tiles", sortedKeys(boolKeys(sc.Tiles))},
			{"Explored", sortedKeys(sc.Explored)},
		} {
			var leaked []string
			for _, sq := range remembered.keys {
				if !ever[sq] {
					leaked = append(leaked, sq)
				}
			}
			if len(leaked) > 0 {
				out = append(out, fmt.Sprintf("scene %q %s holds %d square(s) no eye of this seat has ever "+
					"seen %s — terrain it was never entitled to", id, remembered.what,
					len(leaked), keystoneSample(leaked)))
			}
		}
	}

	sort.Strings(out)
	return out
}

// boolKeys re-keys a tile map as a set, so the two remembered maps above can be
// walked by one loop rather than two that could drift apart.
func boolKeys(tiles map[string]engine.Tile) map[string]bool {
	out := make(map[string]bool, len(tiles))
	for k := range tiles {
		out[k] = true
	}
	return out
}

// squareSetDiff reports how two square sets differ, naming a bounded number of
// squares from each direction: a whole 32x32 board in an error message is not a
// message anyone reads.
func squareSetDiff(got, want map[string]bool) string {
	var extra, missing []string
	for sq := range got {
		if !want[sq] {
			extra = append(extra, sq)
		}
	}
	for sq := range want {
		if !got[sq] {
			missing = append(missing, sq)
		}
	}
	if len(extra) == 0 && len(missing) == 0 {
		return ""
	}
	sort.Strings(extra)
	sort.Strings(missing)
	var parts []string
	if len(extra) > 0 {
		parts = append(parts, fmt.Sprintf("%d square(s) this seat can see and should not %s",
			len(extra), keystoneSample(extra)))
	}
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("%d square(s) this seat should see and does not %s",
			len(missing), keystoneSample(missing)))
	}
	return strings.Join(parts, "; ")
}

func keystoneSample(sqs []string) string {
	const most = 6
	if len(sqs) <= most {
		return "[" + strings.Join(sqs, " ") + "]"
	}
	return "[" + strings.Join(sqs[:most], " ") + " ...]"
}

// keystoneDump renders a state the way cmd/vtt's writeDump and every
// scenarios/goldens/*/state.json do: the state's own fields plus headSequence,
// two-space indented.
//
// DUPLICATED from internal/harness's dumpJSON rather than shared, for this
// file's header reason — the P1 boundary keeps harness out of reach — and the
// two must stay byte-identical. What keeps them so is that the keystone's
// identity arm compares this function's output against the SAME committed
// <golden>/state.json that TestFoldGoldenCorpus holds dumpJSON to. That
// cross-check is deliberate rather than incidental: without it the projected
// fixtures would exercise only this copy, and a drift between the two would
// pass both packages.
func keystoneDump(t *testing.T, st *engine.State, head int64) []byte {
	t.Helper()
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	headRaw, err := json.Marshal(head)
	if err != nil {
		t.Fatal(err)
	}
	fields["headSequence"] = headRaw
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(fields); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// keystoneCorpus reads every golden's LOG. See this file's header for why the
// walker is here rather than shared with internal/harness.
func keystoneCorpus(t *testing.T) []keystoneGolden {
	t.Helper()
	entries, err := filepath.Glob("../../scenarios/goldens/*")
	if err != nil {
		t.Fatal(err)
	}
	var out []keystoneGolden
	for _, e := range entries {
		info, err := os.Stat(e)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			continue
		}
		out = append(out, keystoneGolden{
			name: filepath.Base(e), dir: e,
			log: readEnvelopes(t, filepath.Join(e, "stream.json")),
		})
	}
	if len(out) == 0 {
		t.Fatal("no golden scenario directories found — an empty corpus must fail rather than vacuously pass")
	}
	return out
}

// readEnvelopes decodes a committed stream.json into envelopes.
func readEnvelopes(t *testing.T, path string) []*vttv1.Envelope {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []json.RawMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := make([]*vttv1.Envelope, 0, len(msgs))
	for i, m := range msgs {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(m, env); err != nil {
			t.Fatalf("%s[%d]: %v", path, i, err)
		}
		out = append(out, env)
	}
	return out
}

// keystoneSeats are the viewers one golden is checked through, DERIVED FROM THE
// LOG rather than listed, so a golden added later is covered without anyone
// remembering to name its participants here.
//
// EVERY GOLDEN GETS A PLAYER SEAT AND A SPECTATOR SEAT, including the ones whose
// logs contain no player-controlled actor at all. Spec §7: "the DM sees
// everything, so no test exercising the DM can catch a projection bug." A seat
// with no eyes is not a degenerate case to skip — it is onboarding (you log in
// with no character and the DM assigns one afterwards), and its correct answer
// is "no scene at all, not its name, not its size" (exit criterion 4).
func keystoneSeats(t *testing.T, g keystoneGolden) []keystoneSeat {
	t.Helper()

	controllers := map[string]bool{}
	controlled := map[string]bool{}
	uncontrolled := map[string]bool{}
	for _, env := range g.log {
		switch p := env.GetPayload().(type) {
		case *vttv1.Envelope_ActorAdded:
			a := p.ActorAdded.GetActor()
			ids := a.GetControllerIds()
			if len(ids) == 0 && a.GetControllerId() != "" {
				ids = []string{a.GetControllerId()}
			}
			if len(ids) == 0 {
				uncontrolled[a.GetActorId()] = true
				continue
			}
			controlled[a.GetActorId()] = true
			for _, c := range ids {
				controllers[c] = true
			}
		case *vttv1.Envelope_ActorControlGranted:
			controlled[p.ActorControlGranted.GetActorId()] = true
			delete(uncontrolled, p.ActorControlGranted.GetActorId())
			controllers[p.ActorControlGranted.GetParticipantId()] = true
		}
	}

	seats := []keystoneSeat{
		{name: "dm", viewer: gateway.Viewer{ParticipantID: "p-dm", Role: identity.RoleDM}},
		{name: "agent", viewer: gateway.Viewer{ParticipantID: "p-agent", Role: identity.RoleAgent}},
		// The onboarding seat: a player who has been granted no character yet.
		// Exit criterion 4 — "a seat with no actor sees no scene at all".
		{name: "player-unassigned", viewer: gateway.Viewer{ParticipantID: "p-nobody", Role: identity.RolePlayer}},
		// A spectator who has not named a shoulder. seat.go: "a connection opens
		// perched on nobody", which is the fail-closed direction.
		{name: "spectator-unperched", viewer: gateway.Viewer{ParticipantID: "p-watcher", Role: identity.RoleSpectator}},
	}
	for _, id := range sortedKeys(controllers) {
		seats = append(seats, keystoneSeat{
			name:   "player/" + id,
			viewer: gateway.Viewer{ParticipantID: id, Role: identity.RolePlayer},
		})
	}
	for _, id := range sortedKeys(controlled) {
		seats = append(seats, keystoneSeat{
			name:   "spectator-on/" + id,
			viewer: gateway.Viewer{ParticipantID: "p-watcher", Role: identity.RoleSpectator, Viewpoint: id},
		})
	}
	for _, id := range sortedKeys(uncontrolled) {
		// §3.1.1's refusal: "a spectator perched on the Goblin Archer would
		// watch the ambush from inside it, and the arc would be undone in a
		// single click." The oracle gives such a perch no eyes; the projection
		// must agree.
		seats = append(seats, keystoneSeat{
			name:   "spectator-on-npc/" + id,
			viewer: gateway.Viewer{ParticipantID: "p-watcher", Role: identity.RoleSpectator, Viewpoint: id},
		})
	}
	return seats
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// projectedSeat is scenarios/goldens/<golden>/projections/<seat>/viewer.json:
// which seat a committed projected stream belongs to.
//
// THE PERCH IS NOT A LOG FACT, which is why this file exists at all. A
// spectator's shoulder arrives over SetViewpoint on their own connection and
// nothing about it is ever written to the log (seat.go: the perch "is
// connection state, like the catch-up point"), so a projected golden has to
// declare it beside the stream rather than derive it from one.
type projectedSeat struct {
	Seat          string `json:"seat"`
	ParticipantID string `json:"participantId"`
	Role          string `json:"role"`
	Viewpoint     string `json:"viewpoint,omitempty"`
	Why           string `json:"why"`
}

// TestTheProjectedGoldensAreWhatTheProjectionActuallySends pins the projected
// halves of the corpus.
//
// WHY THEY ARE COMMITTED AT ALL, given the keystone above recomputes the
// projection anyway: TypeScript cannot. There is no TS projector and there is
// deliberately never going to be one — "server decides what you may see; the
// client draws it" (spec §6.2) — so the only way client/src/fold.ts can be held
// to the same bytes the Go fold sees is for those bytes to exist on disk.
// client/test/projection-parity.test.ts folds exactly these files.
//
// THIS IS NOT AN INDEPENDENT RECORDING, and saying so plainly matters because
// the corpus README's division of labour rests on the difference. Each
// projections/<seat>/stream.json is DERIVED — it is what gateway.Projector
// emits for that seat over the parent golden's own recorded stream.json — and
// this test is what keeps it honest, by recomputing it and requiring the bytes
// to match. What is NOT derived is the state.json beside it: that is
// hand-derived from the scenario, exactly as every other state.json in the
// corpus is, and three separate things are held to it — the Go fold here, the
// TypeScript fold in client/test, and (for everything but Explored) the
// independent sight oracle in the keystone above.
func TestTheProjectedGoldensAreWhatTheProjectionActuallySends(t *testing.T) {
	corpus := map[string]keystoneGolden{}
	for _, g := range keystoneCorpus(t) {
		corpus[g.name] = g
	}

	seatDirs, err := filepath.Glob("../../scenarios/goldens/*/projections/*")
	if err != nil {
		t.Fatal(err)
	}
	var checked int
	for _, dir := range seatDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			continue
		}
		goldenName := filepath.Base(filepath.Dir(filepath.Dir(dir)))
		g, ok := corpus[goldenName]
		if !ok {
			t.Fatalf("%s: no golden named %q", dir, goldenName)
		}

		t.Run(goldenName+"/"+filepath.Base(dir), func(t *testing.T) {
			checked++
			raw, err := os.ReadFile(filepath.Join(dir, "viewer.json"))
			if err != nil {
				t.Fatal(err)
			}
			var seat projectedSeat
			if err := json.Unmarshal(raw, &seat); err != nil {
				t.Fatalf("viewer.json: %v", err)
			}
			viewer := gateway.Viewer{
				ParticipantID: seat.ParticipantID,
				Role:          identity.Role(seat.Role),
				Viewpoint:     seat.Viewpoint,
			}

			projected := projectWholeLog(t, g, viewer)
			gotStream := marshalStream(t, projected)
			wantStream, err := os.ReadFile(filepath.Join(dir, "stream.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bytes.TrimSpace(gotStream), bytes.TrimSpace(wantStream)) {
				t.Errorf("the projection no longer emits the committed stream\n--- emitted ---\n%s\n--- committed ---\n%s",
					gotStream, wantStream)
			}

			st, err := campaign.FoldPrefix(projected)
			if err != nil {
				t.Fatalf("folding this seat's projection failed: %v", err)
			}
			gotState := keystoneDump(t, st, headSequenceOf(projected))
			wantState, err := os.ReadFile(filepath.Join(dir, "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(bytes.TrimSpace(gotState), bytes.TrimSpace(wantState)) {
				t.Errorf("folding the projection did not reproduce the hand-derived state\n--- folded ---\n%s\n--- hand-derived ---\n%s",
					gotState, wantState)
			}
		})
	}

	if checked == 0 {
		t.Fatal("no projected seats found under scenarios/goldens/*/projections/*, so the corpus " +
			"pins Visible and Explored being ABSENT and nothing about the populated case " +
			"(visibility spec §4.3: \"the corpus must gain projected streams\")")
	}
}

// projectWholeLog is `project(log, viewer)` the way a real seat computes it:
// ONE Projector fed the log from the beginning, each event judged against the
// state that event produced. internal/gateway/seat.go's receive is the original;
// this is the same three lines with the socket taken off.
func projectWholeLog(t *testing.T, g keystoneGolden, v gateway.Viewer) []*vttv1.Envelope {
	t.Helper()
	pr := gateway.NewProjector(v)
	var received, out []*vttv1.Envelope
	for _, env := range g.log {
		received = append(received, env)
		world, err := campaign.FoldPrefix(received)
		if err != nil {
			t.Fatalf("%s: folding the log failed: %v", g.name, err)
		}
		out = append(out, pr.Project(env, world)...)
	}
	return out
}

// marshalStream renders envelopes as a protojson array, two-space indented.
//
// IT IS NOT BYTE-FOR-BYTE WHAT cmd/vtt's captureNormalizedStream PRODUCES, and
// an earlier version of this comment claimed it was. Both write valid protojson
// and both indent the same way, but the KEY ORDER differs: the recorder appends
// protojson's own bytes as a json.RawMessage and so keeps PROTO FIELD ORDER
// (`eventId, sequence, sessionId, …`), while the round-trip below decodes into
// `any` and re-encodes, which SORTS keys (`actorRole, eventId, participantId,
// …`). You can see both conventions side by side in the corpus:
// session-zero/stream.json is the recorder's and
// session-zero/projections/*/stream.json is this function's.
//
// That is tolerable here and would not be there. A recorded stream is testimony
// compared against a re-recording, so its shape must match what the server
// emits; a projected stream is only ever compared against THIS function's own
// output and folded by both languages, neither of which reads key order. What
// it does need is to be STABLE, and that is exactly what the round-trip buys:
// protojson deliberately injects random whitespace to discourage byte-comparing
// its output, so its bytes cannot be committed directly.
func marshalStream(t *testing.T, envs []*vttv1.Envelope) []byte {
	t.Helper()
	out := make([]json.RawMessage, 0, len(envs))
	for _, e := range envs {
		raw, err := protojson.MarshalOptions{Multiline: false}.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		var stable any
		if err := json.Unmarshal(raw, &stable); err != nil {
			t.Fatal(err)
		}
		compact, err := json.Marshal(stable)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, compact)
	}
	buf, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(buf, '\n')
}

// TestTheKeystoneCorpusCanTellAProjectionFromAPassthrough is the guard that
// keeps the keystone from being a test of the identity projection wearing the
// name of the general one (spec §4.3, "the corpus must gain projected
// streams").
//
// THE FAILURE THIS CATCHES IS A CORPUS FAILURE, NOT A CODE ONE, and it is the
// reason it exists as its own test rather than as a line inside the keystone: a
// corpus in which nothing is ever hidden cannot distinguish a correct projection
// from one that forwards the whole log to everybody. The keystone would go green
// on day one against such a corpus, which is the tell.
//
// What it demands is the smallest thing that cannot be faked: SOME golden, SOME
// non-DM seat, SOME prefix at which a creature stands in a scene that seat has
// an eye in and the seat still cannot see it. A creature in another scene does
// not count — that is redaction by scene, not by sight — and neither does a seat
// with no eyes, which sees nothing for a reason that has nothing to do with
// geometry.
func TestTheKeystoneCorpusCanTellAProjectionFromAPassthrough(t *testing.T) {
	var witnesses []string

	for _, g := range keystoneCorpus(t) {
		for _, seat := range keystoneSeats(t, g) {
			if seat.viewer.Role == identity.RoleDM || seat.viewer.Role == identity.RoleAgent {
				continue
			}
			var received []*vttv1.Envelope
			for _, env := range g.log {
				received = append(received, env)
				world, err := campaign.FoldPrefix(received)
				if err != nil {
					t.Fatalf("%s: folding the log failed: %v", g.name, err)
				}
				want := visibleState(world, seat.viewer)
				if len(want.squares) == 0 {
					continue // no eyes: nothing geometric is being proven here
				}
				for id, tok := range world.Tokens {
					if _, inScene := want.squares[tok.SceneID]; !inScene {
						continue
					}
					if _, visible := want.tokens[id]; visible {
						continue
					}
					witnesses = append(witnesses,
						fmt.Sprintf("%s / %s: %s at %s in scene %s, at sequence %d",
							g.name, seat.name, id, oracleSquareKey(tok.X, tok.Y), tok.SceneID, env.GetSequence()))
				}
			}
		}
	}

	if len(witnesses) == 0 {
		t.Fatal("the golden corpus contains no creature that any seat's own scene hides from it, so " +
			"TestFoldingAProjectionEqualsWhatTheServerThinksTheViewerSees cannot tell a correct " +
			"projection from one that forwards everything: every scene in the corpus is either " +
			"untiled or entirely floor, with no wall, no closed door and no blocks_sight object " +
			"anywhere in it. Add a golden with a sight blocker and a creature behind it " +
			"(visibility spec §4.3, §9 exit criterion 7).")
	}
	t.Logf("the corpus hides %d creature-sighting(s) that the keystone therefore proves; first: %s",
		len(witnesses), witnesses[0])
}

// headSequenceOf is the largest sequence in a stream — `headSequence` in the
// dump shape. Written as a max rather than "the last one" because a projection
// can end on a frame that carries no sequence of its own (perchSequence is 0).
func headSequenceOf(envs []*vttv1.Envelope) int64 {
	var head int64
	for _, e := range envs {
		if e.GetSequence() > head {
			head = e.GetSequence()
		}
	}
	return head
}
