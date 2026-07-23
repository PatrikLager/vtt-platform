package campaign_test

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

const (
	propertySeed       = 1
	propertyEventCount = 400
	propertyCheckEvery = 50
)

// propModel tracks just enough campaign shape to generate only valid
// actions: which scenes/actors/tokens exist (so place/move never reference
// something that isn't there), which TokenMoved sequences remain eligible
// for undo, and whether a session is currently open.
//
// Undo is deliberately restricted to TokenMoved sequences (see canUndo /
// doUndo). Nothing else in engine.Apply's validation depends on a token's
// move history, so retracting a TokenMoved can never invalidate a later
// event on replay. Retracting a SceneCreated, ActorAdded, TokenPlaced,
// SessionStarted, or SessionEnded, by contrast, can: e.g. retracting a
// SessionEnded while a later SessionStarted exists makes the rebuild replay
// "session already open" and fail outright. Reaching that state isn't
// something a "valid actions only" generator should do to itself, so the
// model never offers those events as undo targets.
type propModel struct {
	scenes []string
	actors []string

	tokenIDs   []string
	tokenScene map[string]string
	tokenPos   map[string][2]int32

	moveSeqs  []int64
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
	out := make([]int64, 0, len(m.moveSeqs))
	for _, seq := range m.moveSeqs {
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
	propMust(t, c, cenv(nextID(), &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 20, GridHeight: 20,
	}), idx, "createScene")
	m.scenes = append(m.scenes, id)
}

func (m *propModel) doAddActor(t *testing.T, c *campaign.Campaign, idx int) {
	m.actorN++
	id := fmt.Sprintf("prop-actor-%d", m.actorN)
	propMust(t, c, cenv(nextID(), &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: id, ModuleId: "prop-module"},
	}), idx, "addActor")
	m.actors = append(m.actors, id)
}

func (m *propModel) doPlaceToken(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int) {
	m.tokenN++
	id := fmt.Sprintf("prop-tok-%d", m.tokenN)
	scene := m.scenes[rng.Intn(len(m.scenes))]
	actor := m.actors[rng.Intn(len(m.actors))]
	x, y := int32(rng.Intn(50)), int32(rng.Intn(50))
	propMust(t, c, cenv(nextID(), &vttv1.TokenPlaced{
		TokenId: id, SceneId: scene, ActorId: actor,
		Position: &vttv1.GridPosition{X: x, Y: y},
	}), idx, "placeToken")
	m.tokenIDs = append(m.tokenIDs, id)
	m.tokenScene[id] = scene
	m.tokenPos[id] = [2]int32{x, y}
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
	m.moveSeqs = append(m.moveSeqs, seq)
}

func (m *propModel) doUndo(t *testing.T, c *campaign.Campaign, rng *rand.Rand, idx int) {
	t.Helper()
	eligible := m.eligibleUndoSeqs()
	seq := eligible[rng.Intn(len(eligible))]
	if err := c.Undo(seq, seq, "property-test undo", nextID(), "sess-1"); err != nil {
		t.Fatalf("property test (seed=%d): action #%d (undo [%d,%d]) failed: %v", propertySeed, idx, seq, seq, err)
	}
	m.retracted[seq] = true
}

func (m *propModel) doStartSession(t *testing.T, c *campaign.Campaign, idx int) {
	propMust(t, c, cenv(nextID(), &vttv1.SessionStarted{Name: "prop-session"}), idx, "startSession")
	m.sessionOpen = true
}

func (m *propModel) doEndSession(t *testing.T, c *campaign.Campaign, idx int) {
	propMust(t, c, cenv(nextID(), &vttv1.SessionEnded{}), idx, "endSession")
	m.sessionOpen = false
}

// step picks one random VALID action given the current model and applies it,
// per the action mix in the brief: create scene ~5%, add actor ~10%, place
// token ~15% (when scene+actor exist), move token ~55% (when tokens exist),
// undo ~10% (when an eligible move exists), start/end session ~5% (start if
// none open; end if one's open, gated further to rand<0.05 so sessions stay
// open across most of the run). Any bucket whose precondition isn't met
// falls through to the next check using the same draw, and the session
// bucket's own "session open, but the 0.05 gate didn't fire" branch falls
// back to addActor — always valid — so every iteration guarantees forward
// progress toward the requested event count.
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
		m.doUndo(t, c, rng, idx)
		counts["undo"]++
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
	for _, kind := range []string{"createScene", "addActor", "placeToken", "moveToken", "startSession"} {
		if counts[kind] == 0 {
			t.Errorf("property test (seed=%d): action type %q was never exercised", propertySeed, kind)
		}
	}
}
