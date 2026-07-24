package campaign_test

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const (
	propertySeed       = 1
	propertyEventCount = 400
	propertyCheckEvery = 50
)

// propModel tracks just enough campaign shape to generate only valid
// forward actions: which scenes/actors/tokens exist (so place/move never
// reference something that isn't there), which non-marker sequences remain
// eligible for undo, and whether a session is currently open.
//
// Undo targets ANY non-marker, un-retracted single sequence — not just
// TokenMoved. This exercises campaign.Undo's own dry-run viability check:
// Undo now folds the would-be-retracted log before persisting the marker,
// so a retraction that would corrupt replay (e.g. retracting a
// SessionEnded while a later SessionStarted exists, which would replay as
// "session already open") is rejected instead of bricking the file.
// doUndo therefore does not treat a rejection as a test failure: it counts
// it as undoRejected, leaves the model's retracted set untouched, and the
// run continues. The every-50-events close/reopen checkpoint in
// TestRebuildEqualsLiveProperty doubles as a corruption detector — a
// bricked file fails to reopen there.
type propModel struct {
	scenes []string
	actors []string

	tokenIDs   []string
	tokenScene map[string]string
	tokenPos   map[string][2]int32

	allSeqs   []int64
	retracted map[int64]bool

	sessionOpen bool

	sceneN, actorN, tokenN int
}

func newPropModel() *propModel {
	return &propModel{
		tokenScene: map[string]string{},
		tokenPos:   map[string][2]int32{},
		retracted:  map[int64]bool{},
	}
}

func (m *propModel) canPlaceToken() bool { return len(m.scenes) > 0 && len(m.actors) > 0 }
func (m *propModel) canMoveToken() bool  { return len(m.tokenIDs) > 0 }

func (m *propModel) eligibleUndoSeqs() []int64 {
	out := make([]int64, 0, len(m.allSeqs))
	for _, seq := range m.allSeqs {
		if !m.retracted[seq] {
			out = append(out, seq)
		}
	}
	return out
}

func (m *propModel) canUndo() bool { return len(m.eligibleUndoSeqs()) > 0 }

// propMust appends env and fails the test with the failing action index and
// seed on error (spec requirement: failures must be reproducible from the
// output alone).
func propMust(t *testing.T, c *campaign.Campaign, env *vttv1.Envelope, idx int, kind string) int64 {
	t.Helper()
	seq, err := c.Append(env)
	if err != nil {
		t.Fatalf("property test (seed=%d): action #%d (%s) failed: %v", propertySeed, idx, kind, err)
	}
	return seq
}

func (m *propModel) doCreateScene(t *testing.T, c *campaign.Campaign, idx int) {
	m.sceneN++
	id := fmt.Sprintf("prop-scn-%d", m.sceneN)
	seq := propMust(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 20, GridHeight: 20,
	}), idx, "createScene")
	m.scenes = append(m.scenes, id)
	m.allSeqs = append(m.allSeqs, seq)
}

func (m *propModel) doAddActor(t *testing.T, c *campaign.Campaign, idx int) {
	m.actorN++
	id := fmt.Sprintf("prop-actor-%d", m.actorN)
	seq := propMust(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: id, ModuleId: "prop-module"},
	}), idx, "addActor")
	m.actors = append(m.actors, id)
	m.allSeqs = append(m.allSeqs, seq)
}

func (m *propModel) doPlaceToken(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int) {
	m.tokenN++
	id := fmt.Sprintf("prop-tok-%d", m.tokenN)
	scene := m.scenes[rng.Intn(len(m.scenes))]
	actor := m.actors[rng.Intn(len(m.actors))]
	x, y := int32(rng.Intn(50)), int32(rng.Intn(50))
	seq := propMust(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: id, SceneId: scene, ActorId: actor,
		Position: &vttv1.GridPosition{X: x, Y: y},
	}), idx, "placeToken")
	m.tokenIDs = append(m.tokenIDs, id)
	m.tokenScene[id] = scene
	m.tokenPos[id] = [2]int32{x, y}
	m.allSeqs = append(m.allSeqs, seq)
}

// doMoveToken uses the model's last known position as From. That tracked
// position can go stale relative to live state after an undo retracts an
// earlier move on the same token (the model doesn't unwind on retraction),
// but engine.Apply never validates From against current position — only
// that the token exists and To is set — so a stale From cannot turn this
// into an invalid action; it only means From/To are not always contiguous,
// which doesn't matter for what this test verifies.
func (m *propModel) doMoveToken(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int) {
	id := m.tokenIDs[rng.Intn(len(m.tokenIDs))]
	from := m.tokenPos[id]
	to := [2]int32{int32(rng.Intn(50)), int32(rng.Intn(50))}
	seq := propMust(t, c, cenv(nextID(), &vttv1.TokenMoved{
		TokenId: id, SceneId: m.tokenScene[id],
		From: &vttv1.GridPosition{X: from[0], Y: from[1]},
		To:   &vttv1.GridPosition{X: to[0], Y: to[1]},
	}), idx, "moveToken")
	m.tokenPos[id] = to
	m.allSeqs = append(m.allSeqs, seq)
}

// doUndo picks a random eligible (non-marker, un-retracted) sequence and
// attempts to retract it alone. Unlike the other actions, a rejection here
// is not a test failure: campaign.Undo is now expected to reject ranges
// that would corrupt replay (e.g. a SessionEnded a later SessionStarted
// depends on), so doUndo counts the rejection as undoRejected, leaves the
// model's retracted set untouched, and lets the run continue.
//
// A successful retraction is counted as undo and marks the sequence
// retracted so it isn't offered again. Because eligibility is no longer
// restricted to TokenMoved, a successful retraction can remove a scene,
// actor, token, or session boundary from live state (campaign.Undo's own
// dry-run guarantees nothing still in the log depended on it, or the
// retraction would have been rejected). The model's bookkeeping for future
// action generation is resynced from live state afterward so it never
// offers a follow-up action — e.g. placing a token on a just-retracted
// actor — that only looks valid because the model forgot the retraction
// happened.
func (m *propModel) doUndo(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	t.Helper()
	eligible := m.eligibleUndoSeqs()
	seq := eligible[rng.Intn(len(eligible))]
	if err := c.Undo(seq, seq, "property-test undo", nextID(), "dm", "test-participant"); err != nil {
		counts["undoRejected"]++
		return
	}
	m.retracted[seq] = true
	counts["undo"]++
	m.resyncFromState(c.State())
}

// resyncFromState rebuilds the model's scene/actor/token/session bookkeeping
// from live campaign state. Called after a successful Undo (see doUndo)
// since a retraction can make the model's tracked entities stale.
//
// Go map iteration order is randomized per run, so each rebuilt slice is
// sorted before use: downstream generation (doPlaceToken, doMoveToken, …)
// indexes into these slices with rng.Intn, and an unsorted, iteration-order
// slice would let the same rng draw sequence pick a different entity from
// run to run — silently breaking the seed-1 run's reproducibility even
// though every individual action stays valid.
func (m *propModel) resyncFromState(st *engine.State) {
	m.scenes = m.scenes[:0]
	for id := range st.Scenes {
		m.scenes = append(m.scenes, id)
	}
	sort.Strings(m.scenes)

	m.actors = m.actors[:0]
	for id := range st.Actors {
		m.actors = append(m.actors, id)
	}
	sort.Strings(m.actors)

	m.tokenIDs = m.tokenIDs[:0]
	for id, tok := range st.Tokens {
		m.tokenIDs = append(m.tokenIDs, id)
		m.tokenScene[id] = tok.SceneID
		m.tokenPos[id] = [2]int32{tok.X, tok.Y}
	}
	sort.Strings(m.tokenIDs)
	for id := range m.tokenScene {
		if _, ok := st.Tokens[id]; !ok {
			delete(m.tokenScene, id)
			delete(m.tokenPos, id)
		}
	}

	m.sessionOpen = false
	for _, s := range st.Sessions {
		if s.EndSeq == 0 {
			m.sessionOpen = true
			break
		}
	}
}

func (m *propModel) doStartSession(t *testing.T, c *campaign.Campaign, idx int) {
	seq := propMust(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "prop-session"}), idx, "startSession")
	m.sessionOpen = true
	m.allSeqs = append(m.allSeqs, seq)
}

func (m *propModel) doEndSession(t *testing.T, c *campaign.Campaign, idx int) {
	seq := propMust(t, c, cenv(nextID(), &vttv1.SessionEnded{}), idx, "endSession")
	m.sessionOpen = false
	m.allSeqs = append(m.allSeqs, seq)
}

// step picks one random VALID action given the current model and applies it,
// per the action mix in the brief: create scene ~5%, add actor ~10%, place
// token ~15% (when scene+actor exist), move token ~55% (when tokens exist),
// undo ~10% (when an eligible non-marker, un-retracted sequence exists —
// see doUndo for why a rejected undo is not a test failure), start/end
// session ~5% (start if none open; end if one's open, gated further to
// rand<0.05 so sessions stay open across most of the run). Any bucket whose
// precondition isn't met falls through to the next check using the same
// draw, and the session bucket's own "session open, but the 0.05 gate
// didn't fire" branch falls back to addActor — always valid — so every
// iteration guarantees forward progress toward the requested event count.
func (m *propModel) step(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int, counts map[string]int) {
	r := rng.Float64()
	switch {
	case r < 0.05:
		m.doCreateScene(t, c, idx)
		counts["createScene"]++
	case r < 0.15:
		m.doAddActor(t, c, idx)
		counts["addActor"]++
	case r < 0.30 && m.canPlaceToken():
		m.doPlaceToken(t, c, rng, idx)
		counts["placeToken"]++
	case r < 0.85 && m.canMoveToken():
		m.doMoveToken(t, c, rng, idx)
		counts["moveToken"]++
	case r < 0.95 && m.canUndo():
		m.doUndo(t, c, rng, idx, counts)
	default:
		switch {
		case !m.sessionOpen:
			m.doStartSession(t, c, idx)
			counts["startSession"]++
		case rng.Float64() < 0.05:
			m.doEndSession(t, c, idx)
			counts["endSession"]++
		default:
			m.doAddActor(t, c, idx)
			counts["addActor"]++
		}
	}
}

// TestRebuildEqualsLiveProperty is the keystone property (spec §9): for a
// long, varied, valid event history — including undo — the state rebuilt
// from a full log replay always equals the live, incrementally-folded
// projection. It's checked by closing and reopening the campaign every 50
// events (single-writer: the SQLite file must be closed before it can be
// reopened) and comparing State() before and after.
func TestRebuildEqualsLiveProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(propertySeed))

	dir := t.TempDir()
	path := filepath.Join(dir, "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatalf("property test (seed=%d): initial open: %v", propertySeed, err)
	}
	t.Cleanup(func() {
		if c != nil {
			c.Close()
		}
	})

	m := newPropModel()
	counts := map[string]int{}

	for i := 0; i < propertyEventCount; i++ {
		m.step(t, c, rng, i, counts)

		if (i+1)%propertyCheckEvery == 0 {
			snapshot := c.State()

			if err := c.Close(); err != nil {
				t.Fatalf("property test (seed=%d): close after action #%d: %v", propertySeed, i, err)
			}
			c, err = campaign.Open(path)
			if err != nil {
				t.Fatalf("property test (seed=%d): reopen after action #%d: %v", propertySeed, i, err)
			}

			live := c.State()
			if !statesEqual(snapshot, live) {
				t.Fatalf("property test (seed=%d): rebuild != live after action #%d\nsnapshot: %+v\nlive:     %+v",
					propertySeed, i, snapshot, live)
			}
		}
	}

	t.Logf("property test (seed=%d): %d events, action counts: %+v", propertySeed, propertyEventCount, counts)

	if counts["undo"] == 0 {
		t.Fatalf("property test (seed=%d): undo was never exercised — this run proves nothing about retraction", propertySeed)
	}
	for _, kind := range []string{"createScene", "addActor", "placeToken", "moveToken", "startSession", "endSession"} {
		if counts[kind] == 0 {
			t.Errorf("property test (seed=%d): action type %q was never exercised", propertySeed, kind)
		}
	}
}
