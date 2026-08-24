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

// TestAPerchCarriesNoSequenceAtAll pins the number every frame on this path
// carries, and the answer is that it carries none: zero.
//
// THIS TEST PINNED THE OPPOSITE UNTIL REVIEW. It required the seat's last
// folded sequence, which is what spec §4.2 says for a synthesized envelope —
// correct for every frame an EVENT caused, and wrong here, because no event
// caused this one. Borrowing a live number put perch frames inside a range that
// `vtt undo` can name: retracting that sequence deleted the watcher's board
// with no message, and left the party's next move dangling against a token the
// watcher no longer had. The test moved with the contract rather than the
// contract with the test.
//
// Zero is unreachable by any retraction on either side of the wire — see
// perchSequence, which names the two guards — so this assertion is the whole of
// what protects a perched board from an unrelated undo.
func TestAPerchCarriesNoSequenceAtAll(t *testing.T) {
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	for _, env := range perchFixtureLog() {
		s.receive(env)
	}

	out := s.perch("hero")
	if len(out) == 0 {
		t.Fatal("perching on the hero must show the hero's board")
	}
	for _, e := range out {
		if e.GetSequence() != 0 {
			t.Errorf("a perch frame must carry no sequence, got %d: %v — a live number is "+
				"one an undo can name", e.GetSequence(), e.GetPayload())
		}
	}
}

// TestAPerchIsNotFilteredByTheResumeCursor is the seat-level half of the
// "answered yes and sent nothing" defect.
//
// A perch used to pass through pastResume, which drops output at or below the
// cursor a client resumed from. That question only means something for replayed
// output — and now that a perch carries no sequence at all, asking it would
// discard EVERY perch frame ever, at every cursor. The two halves are one fix,
// so this test holds them together.
func TestAPerchIsNotFilteredByTheResumeCursor(t *testing.T) {
	log := perchFixtureLog()
	// Resumed from the very head of the log: the strictest cursor a client can
	// present, and the one that produced zero frames before the fix.
	head := log[len(log)-1].GetSequence()

	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, head)
	for _, env := range log {
		s.receive(env)
	}
	if got := len(s.perch("hero")); got == 0 {
		t.Fatal("a spectator who resumed at the head must still be shown the shoulder " +
			"they just climbed onto: a perch is not replay")
	}
}

// TestARapidHopIsCoalescedToTheShoulderItEndedOn pins perchBox's latest-wins
// slot, which is what keeps a hopping spectator from taxing the whole table
// (forty hops over a blocking one-slot handoff time a DM's own command out in
// roughly two runs in three, and never once over the box that is here; see
// perchBox for the measurement and for what coalescing costs).
//
// IT ENDS SOMEWHERE THE BURST DID NOT START, and that is the design of the
// sequence rather than an incidental. It ran hero → goblin → hero and required
// "hero" until review, which a FIRST-WINS box passes identically — measured, by
// injecting `if !b.full { b.shoulder, b.full = actorID, true }` into set and
// watching this test and the whole package stay green. A first-wins box parks a
// spectator on the shoulder they hopped AWAY from, so the direction that matters
// had nothing pinning it. Ending on a value that is neither the first set nor
// the second refuses first-wins and "keeps the second" alike.
//
// THE LAST SET IS THE EMPTY SHOULDER on purpose: un-perching travels through
// this slot as a VALUE, and `full` is the only thing that says the slot is
// occupied. A box that read "" as "nothing here" would strand a bird trying to
// leave, and the ok below is what catches that.
func TestARapidHopIsCoalescedToTheShoulderItEndedOn(t *testing.T) {
	b := newPerchBox()
	b.set("hero")
	b.set("goblin")
	b.set("hero")
	b.set("") // and off again, between shoulders

	got, ok := b.take()
	if !ok || got != "" {
		t.Fatalf("the pump must see the shoulder the spectator ended on — here, none at all: "+
			"got %q (ok=%v)", got, ok)
	}
	// And the slot is empty afterwards: a second wake-up with nothing new in it
	// must not re-apply the last shoulder.
	if _, ok := b.take(); ok {
		t.Error("taking twice must not hand the same shoulder out again")
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
// that makes memory accumulate across a hop.
//
// RE-DERIVED FOR TASK 7, and the earlier form of this test was the reason the
// hole it closes went unnoticed: it asserted `e.GetSceneSeen() != nil` was an
// error, which is STRONGER than the sentence above it and pinned the defect.
// "Un-explores a square" and "reports that you can see nothing right now" are
// different claims, and only the first is forbidden. Leaving a shoulder must
// darken the board — the watcher's eyes are gone, so their current visible set
// is empty — while every square they have already mapped stays mapped. An
// EMPTY SceneSeen says exactly that and no more: it withdraws nothing, because
// both folds union its tiles into Explored and it has none.
func TestLeavingAShoulderTakesTheCreaturesAndNotTheTerrain(t *testing.T) {
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	for _, env := range perchFixtureLog() {
		s.receive(env)
	}
	if len(s.perch("hero")) == 0 {
		t.Fatal("the watcher must be on the hero's shoulder before they can leave it")
	}

	var hidden []string
	var darkened int
	for _, e := range s.perch("") {
		if th := e.GetTokenHidden(); th != nil {
			hidden = append(hidden, th.GetTokenId())
		}
		if ss := e.GetSceneSeen(); ss != nil {
			darkened++
			if n := len(ss.GetTiles()); n != 0 {
				t.Errorf("leaving a shoulder must not re-describe terrain, got %d tiles", n)
			}
			if n := len(ss.GetObjects()); n != 0 {
				t.Errorf("leaving a shoulder must not re-describe scenery, got %d objects", n)
			}
		}
	}
	if darkened != 1 {
		t.Errorf("the one room this watcher was shown must go dark exactly once, got %d", darkened)
	}
	if len(hidden) != 1 || hidden[0] != "t-hero" {
		t.Fatalf("the hero's own token must leave a board with no eyes on it, got %v", hidden)
	}
}

// TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt is the premise
// perchBox's whole design rests on, and until this test nothing held it.
//
// Coalescing a burst of hops to the one it ended on DROPS FRAMES — measured, and
// written up at perchBox: the rooms passed through never reach the board. That
// is affordable for exactly one reason, which is that nothing dropped is lost.
// reperch computes a diff against MEMORY and current sight, and it keeps no
// record of which shoulders it has been asked for, so a shoulder a burst flew
// past is served in full the moment somebody asks for it again.
//
// EVERY ASSERTION BELOW WAS INJECTED AGAINST, one fault each, all four
// realistic rather than arithmetic — this test was written after the code it
// guards, so nothing in it is trusted for looking reasonable:
//
//   - "only the room it ended in" fails when reperch hands the new eyes the
//     whole scene list (`for id := range st.Scenes` folded into look's squares);
//     it reports r-a.
//   - "must be shown its room" fails when the projector's memory is
//     fast-forwarded over every scene in st inside transitions — r-c is marked
//     introduced during catch-up, before anybody perches, so the burst's own
//     room never arrives either.
//   - "must introduce its room" fails when reperch marks every scene introduced
//     AFTER computing its frames ("we coalesced past those rooms, so count them
//     as shown"). That is the dangerous form of this optimisation, and the one
//     the empty assertion above cannot see: the burst's own room still arrives,
//     the board is silently wrong only for the rooms flown past, and hopping
//     back cannot repair it.
//   - "the whole of the room" fails when introducing a room is taken to have
//     described it (`pr.seen[id] = now.squares[id]` in the introduction loop);
//     the room arrives with 0 of its 9 tiles.
//
// A FIFTH INJECTION DID NOT BITE and is recorded because it was the one this
// comment first claimed: giving reperch a `served` set and returning nil for a
// repeat changes nothing here, since no shoulder below is asked for twice. It
// was written down before it was run, which is the exact defect this round of
// review exists to fix — and the two entries above it are the second instance
// of the same thing, since the first draft of this list gave ONE fault for
// "must introduce its room" and review found it landed on a different
// assertion once the empty-perch check was added.
//
// WHAT THIS PINS IS THE PER-SCENE FORM, and the fifth injection is exactly why
// that has to be said out loud: a reperch that skipped by SHOULDER passes here.
// The property held is that reperch's output is a function of memory and current
// sight — so an optimisation that touches the MEMORY is caught, and one that
// short-circuits on the request is not. Coalescing's premise needs the former,
// since the memory is the only thing that can lose a room; a shoulder asked for
// twice is a different question, and no test in this package asks it.
//
// If this property ever goes, coalescing stops being defensible and perchBox has
// to become the FIFO its comment explains away.
func TestAShoulderABurstFlewPastIsRestoredByHoppingBackToIt(t *testing.T) {
	s := newSeat(&identity.Participant{ID: "s-1", Role: identity.RoleSpectator}, 0)
	for _, env := range threeRoomLog() {
		s.receive(env)
	}

	// The seat as a coalesced burst leaves it: perchBox dropped the hops naming
	// a-a and a-b, so the pump applied a-c and only a-c. This calls the seat
	// directly — the box is pinned one test up — so what is asserted here is the
	// consequence: r-a and r-b were never introduced.
	var sawEnd bool
	for _, e := range s.perch("a-c") {
		sc := e.GetSceneCreated()
		if sc == nil {
			continue
		}
		if sc.GetSceneId() != "r-c" {
			t.Fatalf("a coalesced burst must show only the room it ended in, got %q",
				sc.GetSceneId())
		}
		sawEnd = true
	}
	// ONLY r-c is half a claim; the other half is that r-c ARRIVED. Without this
	// the loop above passes on an empty perch, and it measurably does: injecting
	// the whole-scene-list fault into look() rather than into reperch introduces
	// every room during catch-up instead, and this loop then falls silent.
	if !sawEnd {
		t.Fatal("the shoulder the burst ended on must be shown its room")
	}

	// And now they ask for one of the shoulders the burst flew past.
	var room *vttv1.SceneCreated
	var seen *vttv1.SceneSeen
	for _, e := range s.perch("a-b") {
		if sc := e.GetSceneCreated(); sc != nil && sc.GetSceneId() == "r-b" {
			room = sc
		}
		if ss := e.GetSceneSeen(); ss != nil && ss.GetSceneId() == "r-b" {
			seen = ss
		}
	}
	if room == nil {
		t.Fatal("hopping back to a shoulder a burst flew past must introduce its room: " +
			"coalescing is only affordable because nothing it drops is lost")
	}
	if got := len(seen.GetTiles()); got != 9 {
		t.Fatalf("the whole of the room this shoulder can see must arrive with it, "+
			"got %d of 9 tiles", got)
	}
}

// threeRoomLog is three separate 3x3 rooms with one player-controlled actor
// standing in the middle of each, so hopping between shoulders means changing
// SCENE rather than changing view of one. THREE and not two, because a burst
// needs a shoulder to start on, one to fly past, and one to end on.
//
// It is the bench perchBox's FRAME COUNTS were taken on — the 11-against-3 and
// the recoverability figures. Its STALL numbers are from somewhere else and
// could not have come from here: a blocking handoff timing a DM's own command
// out needs a server, a second connection and two goroutines, none of which this
// fixture has. That bench is TestHoppingWhileTheTableIsBusyKeepsOneOrder.
func threeRoomLog() []*vttv1.Envelope {
	rooms := []string{"r-a", "r-b", "r-c"}
	actors := []string{"a-a", "a-b", "a-c"}

	tiles := func() map[string]*vttv1.TileRef {
		t := map[string]*vttv1.TileRef{}
		for x := int32(0); x < 3; x++ {
			for y := int32(0); y < 3; y++ {
				t[squareKey(x, y)] = &vttv1.TileRef{Kind: "floor"}
			}
		}
		return t
	}

	out := []*vttv1.Envelope{{Sequence: 1, EventId: "e",
		Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "n"}}}}
	seq := int64(1)
	add := func(p any) {
		seq++
		env := &vttv1.Envelope{Sequence: seq, EventId: "e"}
		switch v := p.(type) {
		case *vttv1.Envelope_SceneCreated:
			env.Payload = v
		case *vttv1.Envelope_ActorAdded:
			env.Payload = v
		case *vttv1.Envelope_TokenPlaced:
			env.Payload = v
		}
		out = append(out, env)
	}
	for _, id := range rooms {
		add(&vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: id, GridWidth: 3, GridHeight: 3, Tiles: tiles()}})
	}
	// Party members, declared. A perch is refused on anything else (MayPerch,
	// and eyes() a second time), and an absent kind is not a party member —
	// there is no longer a branch that reads control instead, so these three
	// have to SAY what they are or the whole fixture becomes unperchable.
	for _, id := range actors {
		add(&vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: id, Name: id, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}})
	}
	for i, id := range actors {
		add(&vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "t-" + id, SceneId: rooms[i], ActorId: id,
			Position: &vttv1.GridPosition{X: 1, Y: 1}}})
	}
	return out
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
			ActorId: "hero", Name: "Hero",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}}},
		{4, &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "goblin", Name: "Goblin"}}}},
		{5, &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-hero",
			SceneId: "s", ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}}}},
		{6, &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: "t-gob",
			SceneId: "s", ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}}}},
		// Control is a SEPARATE EVENT now, and the last one here: creation
		// makes a character, a grant hands it over (spec §5.1, 2026-08-24).
		// It is last so the head sequence this file's resume-cursor test takes
		// is still the head of the whole fixture.
		{7, &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "hero", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}},
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
