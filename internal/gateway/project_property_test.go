package gateway_test

import (
	"fmt"
	"math/rand"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/eventgen"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"google.golang.org/protobuf/proto"
)

// propertyWalkEvents is how many actions one walk draws. It is shorter than
// internal/campaign's 400 because every event is projected for every seat and
// each seat's whole stream is re-folded at the end: the work here is events
// times seats, not events.
const propertyWalkEvents = 250

// seatUnderTest is one projector plus the stream it has been sent.
type seatUnderTest struct {
	name     string
	viewer   gateway.Viewer
	pr       *gateway.Projector
	received []*vttv1.Envelope
}

// TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer folds what the
// PROJECTOR chose to send a seat and compares it against what the server holds.
// A projection bug makes those two disagree; TestRebuildEqualsLiveProperty
// cannot see one at all, because it only ever folds the log.
//
// IT DOES NOT ESCAPE THE SAME-CODE CANCELLATION, and a first version of this
// comment claimed it did. Both sides here run engine.Apply too, so a fault in a
// FORWARDED arm lands identically on both and cancels exactly as it does in
// internal/campaign. Measured:
//
//	ActorRemoved made a no-op      forwarded to players   green here, green there
//	SceneCreated storing width+1   re-synthesized         RED here, green there
//
// The escape is real but narrow: it holds for the arms the projector REBUILDS
// from state — a SceneSeen it synthesizes, a TokenPlaced it emits at the square
// a token now occupies — because the fault is then applied twice, once by the
// server and once by the projector reading a state the same fault produced. For
// an envelope forwarded unchanged there is no second application and no escape,
// and on the DM/agent path there is no projection at all.
//
// WHAT SOUNDNESS MEANS, precisely, because a player is SUPPOSED to know less:
// every scene, actor and token a seat's fold holds must exist in the server's
// with an equal value. It may hold fewer — that is the whole point of the
// projection — but never something the server does not have, and never a stale
// value for something it does. A token that moved out of sight is deleted from
// the seat by a TokenHidden rather than left behind at its last known square,
// so "fewer, never different" is the invariant the projector owes.
//
// THE DM AND AGENT ARM IS A TRIPWIRE, NOT A PROOF, and calling it the sharp
// case was wrong. Project returns the SAME POINTER for those roles, so their
// fold is built from the identical envelopes the server folded — it is
// fold(log) against fold(log), the very shape this file faults
// TestRebuildEqualsLiveProperty for. It can only fail if someone edits that
// role switch, which TestTheDMReceivesEverythingUnchanged already pins by
// pointer. It is kept because a regression there would be severe and this
// notices it for free, not because it demonstrates anything.
func TestEveryProjectedSeatFoldsToSomethingSoundAgainstTheServer(t *testing.T) {
	var total walkStats
	for _, seed := range []int64{1, 2, 3, 4, 5, 6} {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			total.add(runSeatWalk(t, seed))
		})
	}

	// A VACUITY GUARD, and it is not decoration: measured before it existed,
	// a player seat ended seed 1 holding no token at all, and seed 2 held no
	// scene and no token for either player. Not "nothing at all", which an
	// earlier draft said: seed 2 still projects 62 envelopes to prop-p-0 —
	// narration, grants, actors, conditions — folding to five actors. It is the
	// SCENE and TOKEN clauses that go vacuous without a grant, not every clause.
	// Every assertion over such a seat is trivially true, so the soundness
	// check that makes this test worth running rested on one seed in three.
	//
	// The token clauses are the ones that matter, because the token position is
	// what the payoff injection moves: suppress the projector's TokenHidden and
	// a player keeps a ghost standing where a token used to be. A run where no
	// player ever holds a token cannot see that, however green it looks.
	// A SUM IS THE WRONG GRANULARITY, which the first version of this guard
	// used and review measured: eight of twelve seat-walks ended holding no
	// token, seed 2 projected no scene or token to either player, and the whole
	// run still cleared a total-based floor of one. Under the payoff injection
	// seed 2 passed while the other five failed. The floor is per-seed now, so
	// a walk that stops showing players anything reds instead of being carried.
	if total.seedsWithTokens < 4 {
		t.Errorf("only %d of 6 seeds ended with a player holding a token; the soundness "+
			"check over token positions is the one the payoff injection moves, and it "+
			"runs over an empty map on every other seed", total.seedsWithTokens)
	}
	if total.seedsWithHides < 4 {
		t.Errorf("only %d of 6 seeds withdrew a token from a player; that is the path a "+
			"stale board comes from", total.seedsWithHides)
	}
	if total.seedsWithScenes < 5 {
		t.Errorf("only %d of 6 seeds ended with a player holding a scene", total.seedsWithScenes)
	}
	t.Logf("player seats ended holding %d scenes and %d tokens; %d withdrawals projected; "+
		"seeds with tokens/hides/scenes: %d/%d/%d",
		total.playerScenes, total.playerTokens, total.hides,
		total.seedsWithTokens, total.seedsWithHides, total.seedsWithScenes)
}

// walkStats is what one walk showed a player, so the test can refuse a run that
// asserted over an empty board.
type walkStats struct {
	playerScenes, playerTokens, hides                int
	seedsWithScenes, seedsWithTokens, seedsWithHides int
}

func (w *walkStats) add(o walkStats) {
	w.playerScenes += o.playerScenes
	w.playerTokens += o.playerTokens
	w.hides += o.hides
	if o.playerScenes > 0 {
		w.seedsWithScenes++
	}
	if o.playerTokens > 0 {
		w.seedsWithTokens++
	}
	if o.hides > 0 {
		w.seedsWithHides++
	}
}

func runSeatWalk(t *testing.T, seed int64) walkStats {
	t.Helper()
	var stats walkStats
	m := eventgen.New()
	rng := rand.New(rand.NewSource(seed))
	server := engine.NewState()

	// THE PLAYER IDS ARE THE ONES THE MODEL GRANTS TO. eventgen's grantControl
	// hands control to prop-p-0..prop-p-3, so a seat named for one of them
	// really does acquire actors as the walk runs.
	//
	// A SEAT NAMED ANYTHING ELSE IS NOT BLIND, which an earlier draft claimed:
	// measured over these six seeds, an ungranted participant still receives
	// 424 envelopes and folds 34 actors, against 569 and 45 for prop-p-0. What
	// it never gets is a scene or a token, because those are what sight decides
	// — so it is the two clauses this test exists for that would go vacuous,
	// not the whole assertion.
	seats := []*seatUnderTest{
		{name: "dm", viewer: gateway.Viewer{ParticipantID: "dm", Role: identity.RoleDM}},
		{name: "agent", viewer: gateway.Viewer{ParticipantID: "agent", Role: identity.RoleAgent}},
		{name: "player-0", viewer: gateway.Viewer{ParticipantID: "prop-p-0", Role: identity.RolePlayer}},
		{name: "player-1", viewer: gateway.Viewer{ParticipantID: "prop-p-1", Role: identity.RolePlayer}},
	}
	for _, s := range seats {
		s.pr = gateway.NewProjector(s.viewer)
	}

	var seq int64
	for i := 0; i < propertyWalkEvents; i++ {
		a := m.Step(rng, i)
		if a.Env == nil {
			continue
		}
		// VALIDATED THE WAY campaign.Append VALIDATES, and not by hand. A
		// provisional sequence goes on a CLONE, the clone is folded into a
		// Snapshot, and only a state that accepted it advances — so a refused
		// draw cannot half-mutate the server.
		//
		// THE SEQUENCE MUST EXIST BEFORE THE FOLD: engine.Apply validates a
		// narration's anchor range against the event's OWN sequence, so an
		// envelope folded while its Sequence is still zero has every backward
		// anchor rejected as "must be before this event's". Measured — every
		// anchored narration failed inside the first twenty actions of every
		// seed. campaign.Append hides this by assigning the sequence
		// itself, which is why internal/campaign's walk never meets it and why
		// a caller driving engine.Apply directly is the one that has to know.
		//
		// AND THE SNAPSHOT IS NOT OPTIONAL. A first version copied the state by
		// hand, one map at a time, which shares every nested map and slice —
		// engine.Scene carries Tiles, OpenDoors, Explored and Visible, and
		// Conditions is a map of slices — so the probe mutated the very state
		// it was protecting. It surfaced as the probe accepting an event the
		// server then refused, 168 actions later.
		probe := proto.Clone(a.Env).(*vttv1.Envelope)
		probe.Sequence = seq + 1
		if err := engine.Apply(server.Snapshot(), probe); err != nil {
			if !a.MustFail {
				t.Fatalf("action #%d (%s): the model believes this is legal and the engine "+
					"refused it: %v", i, a.Kind, err)
			}
			continue
		}
		if a.MustFail {
			t.Fatalf("action #%d (%s): the engine accepted a draw aimed at something it is "+
				"required to refuse", i, a.Kind)
		}
		seq++
		a.Env.Sequence = seq
		if err := engine.Apply(server, a.Env); err != nil {
			t.Fatalf("action #%d (%s): the snapshot accepted this and the server did not: %v",
				i, a.Kind, err)
		}
		m.Accepted(a, seq)

		// Project AFTER applying, against the state that now includes the
		// event — the ordering internal/gateway's seat.go uses, and which its
		// own comment calls what Project is specified to read.
		for _, s := range seats {
			out := s.pr.Project(a.Env, server)
			if s.viewer.Role == identity.RolePlayer {
				for _, e := range out {
					if _, ok := e.GetPayload().(*vttv1.Envelope_TokenHidden); ok {
						stats.hides++
					}
				}
			}
			s.received = append(s.received, out...)
		}
	}

	for _, s := range seats {
		seen := engine.NewState()
		for j, e := range s.received {
			if err := engine.Apply(seen, e); err != nil {
				t.Fatalf("%s: envelope %d (%T) does not fold: %v", s.name, j, e.GetPayload(), err)
			}
		}
		switch s.viewer.Role {
		case identity.RoleDM, identity.RoleAgent:
			assertSameWorld(t, s.name, server, seen)
		default:
			assertSound(t, s.name, server, seen)
			stats.playerScenes += len(seen.Scenes)
			stats.playerTokens += len(seen.Tokens)
		}
	}
	return stats
}

// assertSameWorld is the DM and agent case. It checks the three counts and then
// every soundness clause, which together are NOT equality — see assertSound for
// what goes uncompared. Equality of the whole state would need a comparison
// internal/campaign already owns (statesEqual), and reaching for it here would
// pin the DM path twice rather than once.
func assertSameWorld(t *testing.T, name string, server, seen *engine.State) {
	t.Helper()
	if len(seen.Scenes) != len(server.Scenes) {
		t.Errorf("%s holds %d scenes and the server holds %d — this seat receives the "+
			"identity projection and must reproduce the server exactly",
			name, len(seen.Scenes), len(server.Scenes))
	}
	if len(seen.Actors) != len(server.Actors) {
		t.Errorf("%s holds %d actors and the server holds %d", name, len(seen.Actors), len(server.Actors))
	}
	if len(seen.Tokens) != len(server.Tokens) {
		t.Errorf("%s holds %d tokens and the server holds %d", name, len(seen.Tokens), len(server.Tokens))
	}
	assertSound(t, name, server, seen)
}

// assertSound is the player case: fewer is allowed, different is not.
//
// WHAT IT DOES NOT COMPARE, stated because the sentence above promises more
// than the code delivers otherwise: a scene's Tiles, Objects, Explored and
// Visible, an Actor's fields beyond existence, and State.Sessions. OpenDoors is
// checked one way only — every door a seat believes open really is open — and
// cannot be checked the other, because a player legitimately holds FEWER open
// doors when the projector withheld one it could not see. Review demonstrated
// the gap: neutering classify's DoorOpened arm leaves a player permanently
// stale about a door they watched open, and this stays green. The package's own
// TestADoorYouCanSeeDoesReachThePlayer is what catches that; a soundness
// property structurally cannot.
func assertSound(t *testing.T, name string, server, seen *engine.State) {
	t.Helper()
	for id, sc := range seen.Scenes {
		held, ok := server.Scenes[id]
		if !ok {
			t.Errorf("%s holds scene %q, which the server does not have — a projection may "+
				"withhold, it may not invent", name, id)
			continue
		}
		if sc.Name != held.Name || sc.GridWidth != held.GridWidth || sc.GridHeight != held.GridHeight {
			t.Errorf("%s holds scene %q as %q %dx%d, the server has %q %dx%d",
				name, id, sc.Name, sc.GridWidth, sc.GridHeight, held.Name, held.GridWidth, held.GridHeight)
		}
	}
	for id, tk := range seen.Tokens {
		held, ok := server.Tokens[id]
		if !ok {
			t.Errorf("%s holds token %q, which the server does not have", name, id)
			continue
		}
		// THE STALE-POSITION CASE, which is the one worth having. A token that
		// moves out of view is withdrawn by a TokenHidden rather than left at
		// its last known square, so a seat that still holds one must hold it
		// where the server does.
		//
		// THE HIDE PATH IS THE COVERED ONE. A token can also go stale by moving
		// WITHIN view and the forward being lost, and this walk barely reaches
		// that: measured over six seeds, prop-p-0 received exactly one
		// TokenMoved, against dozens of withdrawals. The assertion covers both,
		// the generated histories only really exercise one.
		if tk.X != held.X || tk.Y != held.Y || tk.SceneID != held.SceneID {
			t.Errorf("%s holds token %q at %s(%d,%d), the server has it at %s(%d,%d) — a seat "+
				"that still holds a token must hold it where it really is",
				name, id, tk.SceneID, tk.X, tk.Y, held.SceneID, held.X, held.Y)
		}
	}
	for id := range seen.Actors {
		if _, ok := server.Actors[id]; !ok {
			t.Errorf("%s holds actor %q, which the server does not have", name, id)
		}
	}
	for id, sc := range seen.Scenes {
		held, ok := server.Scenes[id]
		if !ok {
			continue // already reported above
		}
		for at := range sc.OpenDoors {
			if !held.OpenDoors[at] {
				t.Errorf("%s believes the door at %s in scene %q is open and the server does "+
					"not — a seat may know of fewer open doors, never of one that is shut",
					name, at, id)
			}
		}
	}
	for actor, carried := range seen.Conditions {
		held := server.Conditions[actor]
		for _, c := range carried {
			found := false
			for _, h := range held {
				if h.ID == c.ID {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s believes actor %q carries condition %q and the server does not",
					name, actor, c.ID)
			}
		}
	}
	for key, n := range seen.Notes {
		held, ok := server.Notes[key]
		if !ok {
			t.Errorf("%s holds note %q, which the server does not have", name, key)
			continue
		}
		if n.Title != held.Title || n.Text != held.Text {
			t.Errorf("%s holds note %q with different contents than the server", name, key)
		}
	}
}
