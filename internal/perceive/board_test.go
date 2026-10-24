package perceive_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// VTT-203 VTT-218
func TestAPlayerNeverReceivesATokenBehindAClosedDoor(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())

	out := pr.event(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob",
		SceneId: "s", ActorId: "goblin",
		Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)

	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			t.Fatal("the goblin is behind a closed door and must not be on this player's wire")
		}
	}
}

// VTT-221
func TestOpeningTheDoorIntroducesTheGoblinToThePlayer(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	pr.event(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}}), st)

	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.event(envelope(8, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	var sawActor, sawToken bool
	for _, e := range out {
		if a := e.GetActorAdded(); a != nil && a.GetActor().GetActorId() == "goblin" {
			sawActor = true
		}
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			sawToken = true
			if e.GetSequence() != 8 {
				t.Errorf("a synthesized introduction carries the CAUSING sequence, got %d",
					e.GetSequence())
			}
		}
	}
	if !sawActor || !sawToken {
		t.Fatalf("opening the door must introduce the goblin (actor=%v token=%v)",
			sawActor, sawToken)
	}
}

// VTT-204
func TestClosingTheDoorHidesTheGoblinAgain(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	pr.event(envelope(8, &vttv1.DoorOpened{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	mustApply(st, 9, &vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	out := pr.event(envelope(9, &vttv1.DoorClosed{SceneId: "s",
		At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	var hidden bool
	for _, e := range out {
		if h := e.GetTokenHidden(); h != nil && h.GetTokenId() == "t-gob" {
			hidden = true
		}
	}
	if !hidden {
		t.Fatal("closing the door must hide the goblin from this player")
	}
}

// VTT-196
func TestASeatWithNoActorIsToldOfNoSceneAtAll(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, perceive.Eyes{Controller: "p-nobody"})

	for _, e := range firstPlace(pr, st) {
		if e.GetSceneCreated() != nil {
			t.Fatalf("a seat with no actor must learn nothing of any scene, got %v", e)
		}
	}
}

// VTT-196
func TestAPlayerLearnsOnlyTheSceneTheirActorStandsIn(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.SceneCreated{
		SceneId: "lair", Name: "The Dragon's Lair", GridWidth: 40, GridHeight: 40})

	pr := newViewer(t, player())
	var scenes []string
	for _, e := range firstPlace(pr, st) {
		if sc := e.GetSceneCreated(); sc != nil {
			scenes = append(scenes, sc.GetSceneId())
		}
	}
	if len(scenes) != 1 || scenes[0] != "s" {
		t.Fatalf("a player must be able to enumerate exactly the scene they stand in, got %v", scenes)
	}
}

// VTT-197
func TestAnIntroducedSceneCarriesTheOutlineButNoTerrain(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())

	var found *vttv1.SceneCreated
	for _, e := range firstPlace(pr, st) {
		if sc := e.GetSceneCreated(); sc != nil {
			found = sc
		}
	}
	if found == nil {
		t.Fatal("a player standing in a scene must be given its board")
	}
	if found.GetGridWidth() != 7 || found.GetGridHeight() != 3 {
		t.Errorf("the outline must be the real one, got %dx%d",
			found.GetGridWidth(), found.GetGridHeight())
	}
	if n := len(found.GetTiles()); n != 0 {
		t.Errorf("an introduced scene must carry NO tiles, got %d", n)
	}
	if n := len(found.GetObjects()); n != 0 {
		t.Errorf("an introduced scene must carry NO objects, got %d", n)
	}
}

// Keep 1,1 in the check: it is the control that shows the set is populated.
// VTT-194 VTT-198
func TestSceneSeenCarriesOnlyTheSquaresInSight(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())

	var seen *vttv1.SceneSeen
	for _, e := range firstPlace(pr, st) {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
	}
	if seen == nil {
		t.Fatal("a player in a scene must be told what they can see of it")
	}
	if _, ok := seen.GetTiles()[key(1, 1)]; !ok {
		t.Errorf("the square the hero stands on must be in their visible set: %v", seen.GetTiles())
	}
	if _, ok := seen.GetTiles()[key(5, 1)]; ok {
		t.Error("the far room is behind a closed door and must not be in the visible set")
	}
}

// VTT-202
func TestASceneThatLeavesSightEntirelyIsReportedDark(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())

	if lit := sceneSeenIn(firstPlace(pr, st), "s"); lit == nil || len(lit.GetTiles()) == 0 {
		t.Fatalf("the control fails: the hero's room must be reported LIT first, got %v", lit)
	}

	mustApply(st, 8, &vttv1.ActorControlRevoked{ActorId: "hero", ParticipantId: "p-1"})
	out := pr.event(envelope(8, &vttv1.ActorControlRevoked{
		ActorId: "hero", ParticipantId: "p-1"}), st)

	dark := sceneSeenIn(out, "s")
	if dark == nil {
		t.Fatal("a scene this seat can no longer see anything of must be reported dark, " +
			"not left at whatever it last saw")
	}
	if n := len(dark.GetTiles()); n != 0 {
		t.Errorf("dark means the whole current visible set is EMPTY, got %d tiles", n)
	}
}

// VTT-202
func TestASceneAlreadyReportedDarkIsNotReportedDarkAgain(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.ActorControlRevoked{ActorId: "hero", ParticipantId: "p-1"})
	pr.event(envelope(8, &vttv1.ActorControlRevoked{
		ActorId: "hero", ParticipantId: "p-1"}), st)

	again := pr.event(envelope(9, &vttv1.NarrationAdded{Text: "the room falls quiet"}), st)
	if ss := sceneSeenIn(again, "s"); ss != nil {
		t.Errorf("a scene already reported dark must stay silent, got %v", ss)
	}
}

// TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain holds sameSet's
// membership test. Keep the two sets the same size, and keep the check that
// they are: a set that grows or shrinks is caught by the lengths alone.
// VTT-201
func TestAVisibleSetThatBecomesAnotherOfTheSameSizeIsSentAgain(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	before := sceneSeenIn(firstPlace(pr, st), "s")
	if before == nil {
		t.Fatal("the control fails: the hero's room must be reported lit first")
	}

	move := &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 4, Y: 1}}
	mustApply(st, 8, move)
	after := sceneSeenIn(pr.event(envelope(8, move), st), "s")
	if after == nil {
		t.Fatal("the squares the hero sees changed to as many others and no SceneSeen was sent")
	}
	if len(after.GetVisible()) != len(before.GetVisible()) || sameStrings(after.GetVisible(), before.GetVisible()) {
		t.Fatalf("fixture: want two different sets of one size, got %v then %v",
			before.GetVisible(), after.GetVisible())
	}
	want := sceneSeenIn(firstPlace(newViewer(t, player()), st), "s")
	if !proto.Equal(after, want) {
		t.Errorf("the SceneSeen carries %v, want the whole set a fresh look gives, %v",
			after.GetVisible(), want.GetVisible())
	}
}

// twoRoomsMissing is twoRooms with the terrain of square sq left out.
func twoRoomsMissing(sq string) *engine.State {
	tiles := twoRoomsTiles()
	delete(tiles, sq)
	st := engine.NewState()
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: tiles})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	mustApply(st, 7, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

// TestAVisibleSquareWithNoTerrainIsAbsentFromTheVisibleSet pins what tiles
// carry: a visible square with no Tile is absent from them. Change it only as
// a decision; the client draws from these keys (SPEC-016).
func TestAVisibleSquareWithNoTerrainIsAbsentFromTheVisibleSet(t *testing.T) {
	full := sceneSeenIn(firstPlace(newViewer(t, player()), twoRooms()), "s")
	if n := len(full.GetTiles()); n != 8 {
		t.Fatalf("the control fails: a complete room must report 8 visible squares, got %d", n)
	}
	if full.GetTiles()[key(1, 1)] == nil {
		t.Fatal("the control fails: the hero's own square must be in the complete room's set")
	}

	holed := sceneSeenIn(firstPlace(newViewer(t, player()), twoRoomsMissing(key(1, 1))), "s")
	if n := len(holed.GetTiles()); n != 7 {
		t.Errorf("removing one square's terrain must remove exactly it, got %d squares", n)
	}
	if holed.GetTiles()[key(1, 1)] != nil {
		t.Error("a square with no Tile has nothing to send, so it cannot be in the set")
	}
	if holed.GetTiles()[key(2, 1)] == nil {
		t.Error("every square that DOES declare terrain must still be reported")
	}
}

// bareCanvas is a 3x3 scene with no terrain and two tokens on it.
func bareCanvas() *engine.State {
	st := engine.NewState()
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 2, Y: 2}})
	mustApply(st, 7, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

// VTT-198
func TestSceneSeenCarriesTheVisibleSquaresEvenWithNoTerrain(t *testing.T) {
	st := bareCanvas()
	pr := newViewer(t, player())
	out := pr.event(envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 2, Y: 2}}), st)

	var sent []string
	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil {
			sent = append(sent, tp.GetTokenId())
		}
	}
	sort.Strings(sent)
	if len(sent) != 2 {
		t.Fatalf("the control fails: the server must send both tokens, got %v", sent)
	}

	seen := sceneSeenIn(out, "s")
	if seen == nil {
		t.Fatal("a player standing in a scene must be told what they can see of it")
	}
	if n := len(seen.GetTiles()); n != 0 {
		t.Errorf("there is no terrain to describe, got %d tiles", n)
	}
	if n := len(seen.GetVisible()); n != 9 {
		t.Fatalf("every square of a bare 3x3 is visible, got %d: %v", n, seen.GetVisible())
	}
	for _, sq := range []string{key(1, 1), key(2, 2)} {
		var found bool
		for _, v := range seen.GetVisible() {
			if v == sq {
				found = true
			}
		}
		if !found {
			t.Errorf("square %s carries a token the server just sent and must be visible", sq)
		}
	}
}

// VTT-223
func TestTheVisibleSetIsSentInAStableOrder(t *testing.T) {
	for i := 0; i < 20; i++ {
		seen := sceneSeenIn(pr20(t), "s")
		got := seen.GetVisible()
		// Keep the length check: sort.StringsAreSorted(nil) is true, so without
		// it an absent visible set passes as sorted.
		if len(got) != 9 {
			t.Fatalf("the control fails: a bare 3x3 has nine visible squares, got %v", got)
		}
		if !sort.StringsAreSorted(got) {
			t.Fatalf("the visible set must be emitted sorted, got %v", got)
		}
	}
}

// pr20 is one fresh projection of bareCanvas, a fresh draw against Go's
// randomised map iteration each call.
func pr20(tb testing.TB) []*vttv1.Envelope {
	tb.Helper()
	st := bareCanvas()
	return newViewer(tb, player()).event(
		envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
			ActorId: "goblin", Position: &vttv1.GridPosition{X: 2, Y: 2}}), st)
}

// TestVisibleAndExploredComeFromDifferentSourcesAndMayDiffer pins the pair a
// reader will assume matches: Explored is terrain remembered, Visible is
// sight's own answer, and on a bare canvas they are wholly apart.
func TestVisibleAndExploredComeFromDifferentSourcesAndMayDiffer(t *testing.T) {
	seen := sceneSeenIn(pr20(t), "s")

	viewer := engine.NewState()
	mustApply(viewer, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(viewer, 2, &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 3, GridHeight: 3})
	must3(t, engine.Apply(viewer, envelope(3, seen)))

	sc := viewer.Scenes["s"]
	if len(sc.Visible) != 9 {
		t.Errorf("every square is visible on a bare canvas, got %d", len(sc.Visible))
	}
	if len(sc.Explored) != 0 {
		t.Errorf("there is no terrain to remember, so nothing is explored, got %v", sc.Explored)
	}
}

func must3(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("fold: %v", err)
	}
}

// VTT-195
func TestASpectatorRidesTheShoulderTheyPerchOn(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, perceive.Eyes{Shoulder: "hero"})
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	var sawGoblin bool
	for _, e := range pr.event(envelope(8, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st) {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			sawGoblin = true
		}
	}
	if !sawGoblin {
		t.Fatal("a spectator must see what the shoulder they ride sees")
	}
}

// TestAPerchOnAnNpcYieldsNoSightAtAll refuses a non-party shoulder in the
// projection too (SPEC-015). Assert scene, terrain and tokens only: the party
// roster still reaches a spectator, who needs it to choose a shoulder.
func TestAPerchOnAnNpcYieldsNoSightAtAll(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, perceive.Eyes{Shoulder: "goblin"})

	for _, e := range firstPlace(pr, st) {
		switch {
		case e.GetSceneCreated() != nil, e.GetSceneSeen() != nil, e.GetTokenPlaced() != nil:
			t.Fatalf("a perch on an NPC must yield no sight at all, got %v", e)
		}
	}
}

// Keep SceneCreated first: it is the payload the projection redacts, the one
// most tempting to edit where it stands.
// VTT-222
func TestProjectingChangesNeitherTheEventNorTheState(t *testing.T) {
	ins := []*vttv1.Envelope{
		envelope(2, &vttv1.SceneCreated{SceneId: "s", Name: "S",
			GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()}),
		envelope(4, &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "goblin", Name: "Goblin"}}),
		envelope(6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
			ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}}),
	}
	for _, in := range ins {
		st := twoRooms()
		pr := newViewer(t, player())
		inBefore := proto.Clone(in)
		stBefore, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}

		pr.event(in, st)

		if !proto.Equal(in, inBefore) {
			t.Errorf("the projection must not touch the envelope it was handed:\n got %v\nwant %v",
				in, inBefore)
		}
		stAfter, err := json.Marshal(st)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		if string(stAfter) != string(stBefore) {
			t.Errorf("the projection must not touch state:\n got %s\nwant %s", stAfter, stBefore)
		}
	}
}

// openFloor is a w x h room of plain floor with no walls at all.
func openFloor(w, h int32) map[string]*vttv1.TileRef {
	tiles := map[string]*vttv1.TileRef{}
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			tiles[key(x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	return tiles
}

// aWholeFight is a log with every transition the projection must get right.
// Keep two of everything: an ordered loop is observably ordered only when one
// event makes it emit more than one frame.
func aWholeFight() []step {
	return []step{
		{1, &vttv1.SessionStarted{Name: "n"}},
		{2, &vttv1.SceneCreated{SceneId: "s", Name: "S",
			GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()}},
		{3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}},
		{4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}}},
		{5, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "ogre", Name: "Ogre"}}},
		{6, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{7, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin",
			Position: &vttv1.GridPosition{X: 5, Y: 1}}},
		{8, &vttv1.TokenPlaced{TokenId: "t-ogre", SceneId: "s", ActorId: "ogre",
			Position: &vttv1.GridPosition{X: 4, Y: 1}}},
		{9, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		{10, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}},
		{11, &vttv1.TokenMoved{TokenId: "t-ogre", SceneId: "s",
			From: &vttv1.GridPosition{X: 4, Y: 1}, To: &vttv1.GridPosition{X: 5, Y: 1}}},
		{12, &vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}},
		{13, &vttv1.NarrationAdded{Text: "The door slams.", As: "DM"}},

		// Keep the scout's two tokens in two scenes: it is the one shape that
		// puts a viewer into two scenes on one event.
		{14, &vttv1.SceneCreated{SceneId: "vault", Name: "Vault",
			GridWidth: 3, GridHeight: 3, Tiles: openFloor(3, 3)}},
		{15, &vttv1.SceneCreated{SceneId: "crypt", Name: "Crypt",
			GridWidth: 3, GridHeight: 3, Tiles: openFloor(3, 3)}},
		{16, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "scout", Name: "Scout"}}},
		{17, &vttv1.TokenPlaced{TokenId: "t-sv", SceneId: "vault", ActorId: "scout",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{18, &vttv1.TokenPlaced{TokenId: "t-sc", SceneId: "crypt", ActorId: "scout",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{19, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-1"}},

		// Keep this second grant: it mutates an actor already on the wire, which
		// is what lets TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed see
		// an introduction that aliases live state rather than cloning it.
		{20, &vttv1.ActorControlGranted{ActorId: "scout", ParticipantId: "p-2"}},

		// Keep the middle step: without the condition on the introduction, the
		// removal at the end is a fold error.
		{21, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "ghoul", Name: "Ghoul"}}},
		{22, &vttv1.ConditionApplied{ActorId: "ghoul", ConditionId: "marked", Source: "the DM"}},
		{23, &vttv1.ActorControlGranted{ActorId: "ghoul", ParticipantId: "p-1"}},
		{24, &vttv1.ConditionRemoved{ActorId: "ghoul", ConditionId: "marked"}},
	}
}

// Keep twenty runs: two runs of a two-element set agree on its order half the
// time, so a deleted sort would survive.
// VTT-223
func TestTheSameLogProjectsTheSameStreamEveryTime(t *testing.T) {
	const runs = 20
	first := runLog(t, player(), aWholeFight())
	for r := 1; r < runs; r++ {
		again := runLog(t, player(), aWholeFight())
		if len(first) != len(again) {
			t.Fatalf("the same log must project the same stream: %d envelopes then %d",
				len(first), len(again))
		}
		for i := range first {
			if !proto.Equal(first[i], again[i]) {
				t.Fatalf("run %d, envelope %d differs from the first run of the same log:\n %v\n %v",
					r, i, first[i], again[i])
			}
		}
	}

	// Keep the non-vacuity check: two empty streams satisfy the loop above.
	var placed, hidden int
	for _, e := range first {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			placed++
		}
		if h := e.GetTokenHidden(); h != nil && h.GetTokenId() == "t-gob" {
			hidden++
		}
	}
	if placed != 1 || hidden != 1 {
		t.Fatalf("the goblin must arrive once and leave once, got %d arrivals and %d departures",
			placed, hidden)
	}
}

// TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed: the cut loop cannot
// fail on its own, since a projection fed from the beginning is deterministic.
// Keep the isolated run and the naive loop: they are the ones that can.
// VTT-222
func TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed(t *testing.T) {
	steps := aWholeFight()
	live := runLogSteps(t, player(), steps, false)
	isolated := runLogSteps(t, player(), steps, true)

	for i := range steps {
		if len(live[i]) != len(isolated[i]) {
			t.Fatalf("step %d: the same log projected %d envelopes against live state and %d against an isolated copy",
				i, len(live[i]), len(isolated[i]))
		}
		for j := range live[i] {
			if !proto.Equal(live[i][j], isolated[i][j]) {
				t.Fatalf("step %d, envelope %d: the projection retained a reference into the state it was handed —\n live     %v\n isolated %v",
					i, j, live[i][j], isolated[i][j])
			}
		}
	}

	foldAll := func(t *testing.T, st *engine.State, batches [][]*vttv1.Envelope, from, to int, what string) {
		t.Helper()
		for i := from; i < to; i++ {
			for _, e := range batches[i] {
				if err := engine.Apply(st, e); err != nil {
					t.Fatalf("%s: step %d (%T) does not fold: %v", what, i, e.GetPayload(), err)
				}
			}
		}
	}

	want := engine.NewState()
	foldAll(t, want, live, 0, len(steps), "the live stream")
	if _, ok := want.Tokens["t-hero"]; !ok {
		t.Fatal("the live stream never delivered the player their own character, so nothing below is proved")
	}

	for cut := 1; cut < len(steps); cut++ {
		resumed := runLogSteps(t, player(), steps, false)

		got := engine.NewState()
		foldAll(t, got, live, 0, cut, fmt.Sprintf("cut %d: what the seat folded before it dropped", cut))
		foldAll(t, got, resumed, cut, len(steps), fmt.Sprintf("cut %d: the catch-up", cut))

		gotJSON, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		wantJSON, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal state: %v", err)
		}
		if string(gotJSON) != string(wantJSON) {
			t.Fatalf("dropping after step %d and catching up must land the seat where it would have been:\n got %s\nwant %s",
				cut, gotJSON, wantJSON)
		}
	}

	// Require the naive construction to break somewhere: a projection started
	// empty at the resume point must corrupt the seat it catches up.
	foldErr := func(st *engine.State, batches [][]*vttv1.Envelope, from, to int) error {
		for i := from; i < to; i++ {
			for _, e := range batches[i] {
				if err := engine.Apply(st, e); err != nil {
					return err
				}
			}
		}
		return nil
	}
	var broke int
	for cut := 1; cut < len(steps); cut++ {
		naive := runLogStepsResumingAt(t, player(), steps, cut)

		got := engine.NewState()
		err := foldErr(got, live, 0, cut)
		if err == nil {
			err = foldErr(got, naive, cut, len(steps))
		}
		if err != nil {
			broke++
			continue
		}
		gotJSON, mErr := json.Marshal(got)
		if mErr != nil {
			t.Fatalf("marshal state: %v", mErr)
		}
		wantJSON, mErr := json.Marshal(want)
		if mErr != nil {
			t.Fatalf("marshal state: %v", mErr)
		}
		if string(gotJSON) != string(wantJSON) {
			broke++
		}
	}
	if broke == 0 {
		t.Fatal("a projection started empty at the resume point must corrupt the seat it catches up — " +
			"a resumed projection needs the viewer's fold (VTT-312)")
	}
}

// runLogStepsResumingAt folds the world from the beginning but starts the
// projection empty at step from; batches before from are nil.
func runLogStepsResumingAt(tb testing.TB, eyes perceive.Eyes, steps []step, from int) [][]*vttv1.Envelope {
	tb.Helper()
	st := engine.NewState()
	out := make([][]*vttv1.Envelope, len(steps))
	var pr *testViewer
	for i, s := range steps {
		env := envelope(s.seq, s.payload)
		if err := engine.Apply(st, env); err != nil {
			panic(fmt.Sprintf("runLogStepsResumingAt seq %d: %v", s.seq, err))
		}
		if i == from {
			pr = newViewer(tb, eyes)
		}
		if pr != nil {
			out[i] = pr.event(env, st)
		}
	}
	return out
}

// VTT-218 VTT-224
func TestAProjectedStreamFoldsCleanly(t *testing.T) {
	out := runLog(t, player(), aWholeFight())
	if len(out) == 0 {
		t.Fatal("nothing was projected, so nothing was proved")
	}
	viewerState := engine.NewState()
	for i, e := range out {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	if _, ok := viewerState.Tokens["t-gob"]; ok {
		t.Error("the goblin is behind a closed door again and must be gone from the folded view")
	}
	if _, ok := viewerState.Tokens["t-hero"]; !ok {
		t.Error("a player must still be able to see their own character")
	}
}

// TestARemovedTokenReachesAPlayerOnlyAsHidden pins classify's TokenRemoved
// ruling: forwarding the raw event behind the TokenHidden is a second removal
// of one token, which both folds refuse (SPEC-016).
// VTT-205
func TestARemovedTokenReachesAPlayerOnlyAsHidden(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	intro := firstPlace(pr, st)

	mustApply(st, 8, &vttv1.TokenRemoved{TokenId: "t-hero"})
	out := pr.event(envelope(8, &vttv1.TokenRemoved{TokenId: "t-hero"}), st)

	var hidden bool
	for _, e := range out {
		if e.GetTokenRemoved() != nil {
			t.Fatal("a removed token must reach a player as TokenHidden, never the raw TokenRemoved")
		}
		if h := e.GetTokenHidden(); h != nil && h.GetTokenId() == "t-hero" {
			hidden = true
		}
	}
	if !hidden {
		t.Fatal("want a synthesized TokenHidden for the player's own removed token")
	}

	viewerState := engine.NewState()
	for i, e := range append(append([]*vttv1.Envelope{}, intro...), out...) {
		if err := engine.Apply(viewerState, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
}

// TestAMoveIsWithheldWhileTheMoverIsOutOfSight: a move names both ends, so
// forwarding one whose start the player never saw hands them a hidden square.
func TestAMoveIsWithheldWhileTheMoverIsOutOfSight(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 4, Y: 1}})
	in := envelope(8, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 4, Y: 1}})

	for _, e := range pr.event(in, st) {
		if e.GetTokenMoved() != nil {
			t.Fatalf("a move by a token behind a closed door must not reach this player: %v", e)
		}
	}
}

// VTT-206
func TestSteppingIntoViewArrivesRatherThanMoves(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})
	in := envelope(8, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s",
		From: &vttv1.GridPosition{X: 5, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})

	var arrived bool
	for _, e := range pr.event(in, st) {
		if e.GetTokenMoved() != nil {
			t.Error("a move OUT of the dark names the dark square it started in")
		}
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			arrived = true
			if tp.GetPosition().GetX() != 2 || tp.GetPosition().GetY() != 1 {
				t.Errorf("the goblin arrives where it now stands, got %v", tp.GetPosition())
			}
		}
	}
	if !arrived {
		t.Error("the goblin walked into the room and must appear")
	}
}

// VTT-212
func TestADoorInARoomYouAreNotInStaysSilent(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.SceneCreated{
		SceneId: "lair", Name: "The Dragon's Lair", GridWidth: 5, GridHeight: 5,
		Tiles: map[string]*vttv1.TileRef{key(2, 2): {Kind: "door"}}})
	pr := newViewer(t, player())
	firstPlace(pr, st)

	mustApply(st, 9, &vttv1.DoorOpened{SceneId: "lair", At: &vttv1.GridPosition{X: 2, Y: 2}})
	in := envelope(9, &vttv1.DoorOpened{SceneId: "lair", At: &vttv1.GridPosition{X: 2, Y: 2}})
	for _, e := range pr.event(in, st) {
		if e.GetDoorOpened() != nil {
			t.Error("a door in a scene this player has never entered must not reach them")
		}
	}
}

// TestADoorYouCanSeeDoesReachThePlayer is the control for
// TestADoorInARoomYouAreNotInStaysSilent. Keep the count at exactly one: a
// duplicate from doorTransitions folds harmlessly, so nothing else reports it.
// VTT-214
func TestADoorYouCanSeeDoesReachThePlayer(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	in := envelope(8, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})

	var opened int
	for _, e := range pr.event(in, st) {
		if e.GetDoorOpened() != nil {
			opened++
		}
	}
	if opened != 1 {
		t.Fatalf("a player must watch the door in their own room swing open, exactly once, got %d", opened)
	}
}

// Keep this apart from the opening test: the two directions run through
// separate arms of doorSubject.
// VTT-214
func TestADoorYouCanSeeSwingShutReachesThePlayerOnce(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	pr.event(envelope(8, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st)

	mustApply(st, 9, &vttv1.DoorClosed{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})
	var closed int
	for _, e := range pr.event(envelope(9, &vttv1.DoorClosed{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st) {
		if e.GetDoorClosed() != nil {
			closed++
		}
	}
	if closed != 1 {
		t.Fatalf("a player must watch the door in their own room swing shut, exactly once, got %d", closed)
	}
}

// TestAVisibleTokensMoveDoesReachThePlayer is the control for
// TestAMoveIsWithheldWhileTheMoverIsOutOfSight.
// VTT-206
func TestAVisibleTokensMoveDoesReachThePlayer(t *testing.T) {
	st := twoRooms()
	pr := newViewer(t, player())
	firstPlace(pr, st)

	mustApply(st, 8, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})
	in := envelope(8, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 1, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}})

	var moved bool
	for _, e := range pr.event(in, st) {
		if e.GetTokenMoved() != nil {
			moved = true
		}
	}
	if !moved {
		t.Fatal("a player must watch their own character walk")
	}
}

// oneRoomWithAPillar is twoRooms' corridor with the wall replaced by scenery.
// Keep the ghost's zero width beside the hero: nothing may ever reveal it.
//
//	 x: 0    1     2    3       4      5    6
//	y=1 wall hero  flr  pillar  crate  gob  wall
func oneRoomWithAPillar() *engine.State {
	st := engine.NewState()
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		tiles[key(x, 0)] = &vttv1.TileRef{Kind: "wall"}
		tiles[key(x, 2)] = &vttv1.TileRef{Kind: "wall"}
		tiles[key(x, 1)] = &vttv1.TileRef{Kind: "floor"}
	}
	tiles[key(0, 1)] = &vttv1.TileRef{Kind: "wall"}
	tiles[key(6, 1)] = &vttv1.TileRef{Kind: "wall"}

	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: tiles,
		Objects: []*vttv1.SceneObject{
			{ObjectId: "pillar", Kind: "pillar", At: &vttv1.GridPosition{X: 3, Y: 1},
				Width: 1, Height: 1, BlocksSight: true, BlocksMove: true},
			{ObjectId: "crate", Kind: "crate", At: &vttv1.GridPosition{X: 4, Y: 1},
				Width: 1, Height: 1},
			{ObjectId: "ghost", Kind: "smudge", At: &vttv1.GridPosition{X: 2, Y: 1},
				Width: 0, Height: 1},
		}})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin"}})
	mustApply(st, 5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s",
		ActorId: "goblin", Position: &vttv1.GridPosition{X: 5, Y: 1}})
	mustApply(st, 7, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

// VTT-199 VTT-203
func TestSightBlockingSceneryHidesWhatStandsBehindIt(t *testing.T) {
	st := oneRoomWithAPillar()
	pr := newViewer(t, player())
	out := firstPlace(pr, st)

	for _, e := range out {
		if tp := e.GetTokenPlaced(); tp != nil && tp.GetTokenId() == "t-gob" {
			t.Error("the goblin is behind a pillar — scenery blocks sight exactly as a wall does")
		}
	}

	var seen *vttv1.SceneSeen
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
	}
	if seen == nil {
		t.Fatal("a player in a scene must be told what they can see of it")
	}
	var ids []string
	for _, o := range seen.GetObjects() {
		ids = append(ids, o.GetObjectId())
	}
	if len(ids) != 1 || ids[0] != "pillar" {
		t.Errorf("the pillar is in plain sight and the crate is in its shadow, got %v", ids)
	}
}

// oneRoomWithAGapInTheWall is a 5x5 room cut by a wall at y=2 with a gap at
// 2,2; from the hero at 2,4 the cache at 0,1 is dark with lit squares beside.
//
//	 x: 0      1     2     3     4
//	y=1 CACHE  flr   flr   flr   flr
//	y=2 wall   wall  gap   wall  wall
func oneRoomWithAGapInTheWall() *engine.State {
	st := engine.NewState()
	tiles := map[string]*vttv1.TileRef{}
	for y := int32(0); y < 5; y++ {
		for x := int32(0); x < 5; x++ {
			tiles[key(x, y)] = &vttv1.TileRef{Kind: "floor"}
		}
	}
	for _, x := range []int32{0, 1, 3, 4} {
		tiles[key(x, 2)] = &vttv1.TileRef{Kind: "wall"}
	}
	mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
	mustApply(st, 2, &vttv1.SceneCreated{
		SceneId: "s", Name: "S", GridWidth: 5, GridHeight: 5, Tiles: tiles,
		Objects: []*vttv1.SceneObject{
			// Keep the cache from blocking sight: it must not cast the shadow it
			// hides in.
			{ObjectId: "cache", Kind: "chest", At: &vttv1.GridPosition{X: 0, Y: 1},
				Width: 1, Height: 1},
		}})
	mustApply(st, 3, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}})
	mustApply(st, 4, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 2, Y: 4}})
	mustApply(st, 5, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

// TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn holds both bounds of
// objectInSight's half-open walk. Keep the three fixture checks: an unlit
// neighbour reveals nothing, and the test then passes for the wrong reason.
// VTT-199
func TestAnObjectIsRevealedOnlyByTheSquaresItStandsOn(t *testing.T) {
	st := oneRoomWithAGapInTheWall()
	pr := newViewer(t, player())
	out := pr.event(envelope(4, &vttv1.TokenPlaced{TokenId: "t-hero",
		SceneId: "s", ActorId: "hero", Position: &vttv1.GridPosition{X: 2, Y: 4}}), st)

	var seen *vttv1.SceneSeen
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
	}
	if seen == nil {
		t.Fatal("a player in a scene must be told what they can see of it")
	}
	lit := seen.GetTiles() // every square carries terrain, so this IS the visible set

	if _, ok := lit[key(0, 1)]; ok {
		t.Fatalf("fixture: the cache's own square must be DARK or nothing below is proved, lit %v", keysOf(lit))
	}
	if _, ok := lit[key(0, 2)]; !ok {
		t.Fatalf("fixture: the square just BELOW the cache must be LIT to catch the y bound, lit %v", keysOf(lit))
	}
	if _, ok := lit[key(1, 1)]; !ok {
		t.Fatalf("fixture: the square just RIGHT of the cache must be LIT to catch the x bound, lit %v", keysOf(lit))
	}

	for _, o := range seen.GetObjects() {
		if o.GetObjectId() == "cache" {
			t.Error("the cache stands on a square this player cannot see — " +
				"a lit square past its edge must not reveal it")
		}
	}
}

// keysOf is a tile map's squares in a stable order, for failure messages.
func keysOf(m map[string]*vttv1.TileRef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestAnIntroducedSceneArrivesWithItsDoorsAlreadyOpen: open doors ride on
// neither the outline nor SceneSeen, so the introduction must correct them.
// VTT-213
func TestAnIntroducedSceneArrivesWithItsDoorsAlreadyOpen(t *testing.T) {
	st := twoRooms()
	mustApply(st, 8, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}})

	pr := newViewer(t, perceive.Eyes{Controller: "p-2"})
	before := pr.event(envelope(8, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 3, Y: 1}}), st)
	for _, e := range before {
		if e.GetDoorOpened() != nil {
			t.Fatal("a seat with no actor is in no scene and must not hear a door")
		}
	}

	mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-2"})
	out := pr.event(envelope(9, &vttv1.ActorControlGranted{
		ActorId: "hero", ParticipantId: "p-2"}), st)

	viewer := engine.NewState()
	for i, e := range append(append([]*vttv1.Envelope{}, before...), out...) {
		if err := engine.Apply(viewer, e); err != nil {
			t.Fatalf("projected envelope %d (%T) does not fold: %v", i, e.GetPayload(), err)
		}
	}
	got := viewer.Scenes["s"].OpenDoors
	if !got[key(3, 1)] {
		t.Fatalf("the introduced scene must arrive with its open doors open, got %v", got)
	}
	if len(got) != len(st.Scenes["s"].OpenDoors) {
		t.Errorf("the viewer's doors must agree with the table's, got %v want %v",
			got, st.Scenes["s"].OpenDoors)
	}
}

// VTT-200 VTT-213
func TestADoorOpenedOutOfSightArrivesWhenTheSquareComesIntoView(t *testing.T) {
	st := oneRoomWithAGapInTheWall()
	pr := newViewer(t, player())
	pr.event(envelope(4, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s",
		ActorId: "hero", Position: &vttv1.GridPosition{X: 2, Y: 4}}), st)

	mustApply(st, 6, &vttv1.DoorOpened{SceneId: "s", At: &vttv1.GridPosition{X: 0, Y: 1}})
	for _, e := range pr.event(envelope(6, &vttv1.DoorOpened{
		SceneId: "s", At: &vttv1.GridPosition{X: 0, Y: 1}}), st) {
		if e.GetDoorOpened() != nil {
			t.Fatal("a door on a square this viewer cannot see must stay silent")
		}
	}

	mustApply(st, 8, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 2, Y: 4}, To: &vttv1.GridPosition{X: 1, Y: 1}})
	out := pr.event(envelope(8, &vttv1.TokenMoved{TokenId: "t-hero", SceneId: "s",
		From: &vttv1.GridPosition{X: 2, Y: 4}, To: &vttv1.GridPosition{X: 1, Y: 1}}), st)

	var seen *vttv1.SceneSeen
	var opened bool
	for _, e := range out {
		if ss := e.GetSceneSeen(); ss != nil {
			seen = ss
		}
		if do := e.GetDoorOpened(); do != nil && do.GetAt().GetX() == 0 && do.GetAt().GetY() == 1 {
			opened = true
		}
	}
	if seen == nil {
		t.Fatal("moving must re-report what the viewer can see")
	}
	if _, lit := seen.GetTiles()[key(0, 1)]; !lit {
		t.Fatalf("fixture: 0,1 must be VISIBLE from 1,1 or this proves nothing, lit %v", keysOf(seen.GetTiles()))
	}
	if !opened {
		t.Fatal("a door that comes into view must arrive in the state it is actually in, " +
			"or the client draws it shut for the rest of the session")
	}
}

// VTT-462 VTT-478
func TestATokenArrivesWithItsNameAndNotesAlone(t *testing.T) {
	st := creatureWorld()
	v := newViewer(t, heroEyes())
	var tp *vttv1.TokenPlaced
	for _, e := range next(v, st, 10, &vttv1.NarrationAdded{Text: "look"}) {
		if p := e.GetTokenPlaced(); p.GetTokenId() == "t-gob" {
			tp = p
		}
	}
	if tp.GetName() != "a hunched shape" || tp.GetNotes() != "asleep" {
		t.Fatalf("the token arrived as %v, want its name and notes", tp)
	}
	if tp.GetGmName() != "" || tp.GetGmNotes() != "" {
		t.Fatalf("the token arrived as %v, with a GM text", tp)
	}
}

// VTT-467 VTT-462
func TestATokensRewriteReachesALogOnlyInSightAndOnlyWhenItsTextChanged(t *testing.T) {
	st := creatureWorld()
	v := newViewer(t, heroEyes())
	next(v, st, 10, &vttv1.NarrationAdded{Text: "look"})

	described := func(out []*vttv1.Envelope) []*vttv1.TokenDescribed {
		var got []*vttv1.TokenDescribed
		for _, e := range out {
			if d := e.GetTokenDescribed(); d != nil {
				got = append(got, d)
			}
		}
		return got
	}
	if got := described(next(v, st, 11, &vttv1.TokenDescribed{TokenId: "t-gob",
		Name: "a hunched shape", Notes: "asleep", GmName: "the chief", GmNotes: "lies"})); len(got) != 0 {
		t.Fatalf("a rewrite of the GM texts alone was sent: %v", got)
	}
	got := described(next(v, st, 12, &vttv1.TokenDescribed{TokenId: "t-gob",
		Name: "an angry goblin", Notes: "asleep", GmName: "the chief", GmNotes: "lies"}))
	want := &vttv1.TokenDescribed{TokenId: "t-gob", Name: "an angry goblin", Notes: "asleep"}
	if len(got) != 1 || !proto.Equal(got[0], want) {
		t.Fatalf("a rewrite of the name was sent as %v, want one %v", got, want)
	}

	next(v, st, 13, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", To: &vttv1.GridPosition{X: 5, Y: 1}})
	if got := described(next(v, st, 14, &vttv1.TokenDescribed{TokenId: "t-gob",
		Name: "a fleeing goblin", GmName: "the chief"})); len(got) != 0 {
		t.Fatalf("a rewrite out of sight was sent: %v", got)
	}
	next(v, st, 15, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", To: &vttv1.GridPosition{X: 2, Y: 1}})
	if tok := v.shown.Tokens["t-gob"]; tok.Name != "a fleeing goblin" || tok.Notes != "" || tok.GMName != "" {
		t.Fatalf("the token came back into sight as %+v, want its present name and no GM text", tok)
	}
}
