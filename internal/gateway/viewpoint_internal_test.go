package gateway

import (
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// The two ways perch can be asked for a re-projection it cannot make. Neither
// is reachable over the wire — Authorize denies set_viewpoint to the
// unprojected roles, and a spectator can only perch on an actor, which implies
// a log that introduced one — so they are pinned HERE, from inside the package,
// rather than left as guards nobody ever runs.

func TestAnUnprojectedSeatCannotBePerched(t *testing.T) {
	// The DM and the agent receive the log itself, by pointer (exit criterion
	// 8). There is no projector to move, and a perch must not invent one:
	// their stream stays byte-for-byte what it is today.
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		s := newSeat(&identity.Participant{ID: "p-1", Role: role}, 0)
		if got := s.perch("a1"); len(got) != 0 {
			t.Errorf("%s seat produced %d frame(s) from a perch; it has no projection to re-run",
				role, len(got))
		}
	}
}

func TestASeatPerchesOnlyAgainstAWorldItHasSeen(t *testing.T) {
	// A seat that has folded no event has no world to look at, and the answer
	// is silence rather than a guess (spec §4.4). The projector's own Project
	// takes the same nil state the same way.
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	if got := s.perch("a1"); len(got) != 0 {
		t.Fatalf("a seat shown nothing has nothing to re-show, got %d frame(s)", len(got))
	}
}

// TestAPerchIsStampedWithTheSequenceItsSeatLastFolded pins the number every
// synthesized envelope on this path carries.
//
// No event causes a perch, so there is no "causing sequence" in spec §4.2's
// sense; the honest answer is the last event this seat was judged against,
// which is exactly the world the new eyes are being shown. A HIGHER number
// would name a sequence this seat has not reached, and the resume cursor is
// keyed on that number.
func TestAPerchIsStampedWithTheSequenceItsSeatLastFolded(t *testing.T) {
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	for _, env := range perchFixtureLog() {
		s.receive(env)
	}

	out := s.perch("hero")
	if len(out) == 0 {
		t.Fatal("perching on the hero must show the hero's board")
	}
	for _, e := range out {
		if e.GetSequence() != 6 {
			t.Errorf("a perch frame carries the sequence its seat last folded (6), got %d: %v",
				e.GetSequence(), e.GetPayload())
		}
	}
}

// TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen is Task 4's I-1 defect one
// seat over, and it is the claim reperch's own comment makes: no door is
// SKIPPED on the perch path, because the skip exists only for the door event
// classify is forwarding alongside, and a perch forwards no event.
//
// Without it a watcher who sits down after the party opened a door sees THROUGH
// that door — SceneSeen lights the room beyond — while their board still draws
// it shut. It never self-corrects and never throws: engine.Scene.OpenDoors
// travels in neither the redacted SceneCreated nor SceneSeen, and both folds
// treat a repeat as idempotent. A demo-gate find, not a CI one, which is why it
// is pinned here rather than trusted.
func TestAPerchArrivesWithTheDoorsItCanSeeAlreadyOpen(t *testing.T) {
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	log := perchFixtureLog()
	// The party opened the door at (3,1) before this watcher chose a shoulder.
	log = append(log, &vttv1.Envelope{Sequence: 7, EventId: "e",
		Payload: &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{
			SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}}})
	for _, env := range log {
		s.receive(env)
	}

	var sawOpen bool
	for _, e := range s.perch("hero") {
		if d := e.GetDoorOpened(); d != nil && d.GetSceneId() == "s" &&
			d.GetAt().GetX() == 3 && d.GetAt().GetY() == 1 {
			sawOpen = true
		}
	}
	if !sawOpen {
		t.Fatal("the door was opened before this watcher perched, and they can see its square: " +
			"their board must not be left drawing it shut")
	}
}

// TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain pins what the empty
// actor id DOES, rather than only that MayPerch permits it.
//
// A bird between shoulders sees nothing, so the creatures it was shown go: they
// are pure line of sight (spec §3.2). The TERRAIN does not, and cannot — there
// is no message that un-explores a square, which is the same structural fact
// that makes memory accumulate across a hop. So an un-perch emits TokenHidden
// and no SceneSeen at all.
func TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain(t *testing.T) {
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	for _, env := range perchFixtureLog() {
		s.receive(env)
	}
	if len(s.perch("hero")) == 0 {
		t.Fatal("the watcher must be on the hero's shoulder before they can leave it")
	}

	var hidden []string
	for _, e := range s.perch("") {
		if th := e.GetTokenHidden(); th != nil {
			hidden = append(hidden, th.GetTokenId())
		}
		if e.GetSceneSeen() != nil {
			t.Error("leaving a shoulder must not re-describe terrain: what was explored stays explored")
		}
	}
	if len(hidden) != 1 || hidden[0] != "t-hero" {
		t.Fatalf("the hero's own token must leave a board with no eyes on it, got %v", hidden)
	}
}

// perchFixtureLog is twoRooms' history as a LOG rather than as a folded state:
// a session, a two-room scene split by a closed door, a hero west of it and an
// NPC east of it. Sequences 1..6, so the last one a seat folds is 6.
func perchFixtureLog() []*vttv1.Envelope {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		tiles[squareKey(x, 0)] = &vttv1.TileRef{Kind: "wall"}
		tiles[squareKey(x, 2)] = &vttv1.TileRef{Kind: "wall"}
	}
	tiles[squareKey(0, 1)] = &vttv1.TileRef{Kind: "wall"}
	tiles[squareKey(6, 1)] = &vttv1.TileRef{Kind: "wall"}
	for _, x := range []int32{1, 2, 4, 5} {
		tiles[squareKey(x, 1)] = &vttv1.TileRef{Kind: "floor"}
	}
	tiles[squareKey(3, 1)] = &vttv1.TileRef{Kind: "door"}

	payloads := []struct {
		seq int64
		p   any
	}{
		{1, &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "n"}}},
		{2, &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: tiles}}},
		{3, &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "hero", Name: "Hero", ControllerIds: []string{"p-1"}}}}},
		{4, &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "goblin", Name: "Goblin"}}}},
		{5, &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-hero",
			SceneId: "s", ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}}}},
		{6, &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-gob",
			SceneId: "s", ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}}}},
	}

	out := make([]*vttv1.Envelope, 0, len(payloads))
	for _, e := range payloads {
		env := &vttv1.Envelope{Sequence: e.seq, EventId: "e"}
		switch p := e.p.(type) {
		case *vttv1.Envelope_SessionStarted:
			env.Payload = p
		case *vttv1.Envelope_SceneCreated:
			env.Payload = p
		case *vttv1.Envelope_ActorAdded:
			env.Payload = p
		case *vttv1.Envelope_TokenPlaced:
			env.Payload = p
		}
		out = append(out, env)
	}
	return out
}
