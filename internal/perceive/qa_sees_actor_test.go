package perceive_test

import (
	"encoding/json"
	"fmt"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

type qaasBuilder struct {
	t   *testing.T
	st  *engine.State
	seq int64
}

func qaasNew(t *testing.T) *qaasBuilder {
	t.Helper()
	return &qaasBuilder{t: t, st: engine.NewState()}
}

func (b *qaasBuilder) apply(env *vttv1.Envelope) *qaasBuilder {
	b.t.Helper()
	b.seq++
	env.Sequence = b.seq
	if err := engine.Apply(b.st, env); err != nil {
		b.t.Fatalf("apply %d: %v", b.seq, err)
	}
	return b
}

func (b *qaasBuilder) scene(id string, w, h int32, tiles map[string]*vttv1.TileRef) *qaasBuilder {
	b.t.Helper()
	return b.apply(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: id, GridWidth: w, GridHeight: h, Tiles: tiles,
		},
	}})
}

func (b *qaasBuilder) actor(id string, kind vttv1.ActorKind) *qaasBuilder {
	b.t.Helper()
	return b.apply(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: id, Name: id, Kind: kind,
		}},
	}})
}

func (b *qaasBuilder) grant(actorID, participant string, kind vttv1.ActorKind) *qaasBuilder {
	b.t.Helper()
	return b.apply(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actorID, ParticipantId: participant, Kind: kind,
		},
	}})
}

func (b *qaasBuilder) token(tokenID, sceneID, actorID string, x, y int32) *qaasBuilder {
	b.t.Helper()
	return b.apply(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{
		TokenPlaced: &vttv1.TokenPlaced{
			TokenId: tokenID, SceneId: sceneID, ActorId: actorID,
			Position: &vttv1.GridPosition{X: x, Y: y},
		},
	}})
}

func (b *qaasBuilder) openDoor(sceneID string, x, y int32) *qaasBuilder {
	b.t.Helper()
	return b.apply(&vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{
			SceneId: sceneID, At: &vttv1.GridPosition{X: x, Y: y},
		},
	}})
}

const (
	qaasParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaasNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

func qaasHallTiles() map[string]*vttv1.TileRef {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 12; x++ {
		for y := int32(0); y < 5; y++ {
			kind := "floor"
			if x == 5 {
				kind = "wall"
				if y == 2 {
					kind = "door"
				}
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return tiles
}

func qaasHall(t *testing.T) *qaasBuilder {
	t.Helper()
	b := qaasNew(t).
		scene("hall", 12, 5, qaasHallTiles()).
		scene("cellar", 5, 5, nil)
	for _, a := range []struct {
		id   string
		kind vttv1.ActorKind
	}{
		{"hero", qaasParty}, {"scout", qaasParty}, {"ally", qaasParty},
		{"near", qaasNonParty}, {"beyond", qaasNonParty}, {"twin", qaasNonParty},
		{"lodger", qaasNonParty}, {"rumour", qaasNonParty}, {"goblin", qaasNonParty},
	} {
		b.actor(a.id, a.kind)
	}
	return b.
		grant("hero", "p1", qaasParty).
		grant("scout", "p1", qaasParty).
		token("tok-hero", "hall", "hero", 1, 2).
		token("tok-scout", "hall", "scout", 10, 2).
		token("tok-ally", "hall", "ally", 9, 0).
		token("tok-near", "hall", "near", 3, 2).
		token("tok-goblin", "hall", "goblin", 2, 2).
		token("tok-beyond", "hall", "beyond", 8, 2).
		token("tok-twin-far", "hall", "twin", 10, 4).
		token("tok-twin-near", "hall", "twin", 2, 4).
		token("tok-lodger", "cellar", "lodger", 3, 2)
}

func qaasExpect(t *testing.T, eyes perceive.Eyes, st *engine.State, want map[string]bool, why string) {
	t.Helper()
	for id, w := range want {
		if got := perceive.SeesActor(eyes, st, id); got != w {
			t.Errorf("SeesActor(%+v, %q) = %v, want %v (%s)", eyes, id, got, w, why)
		}
	}
}

const qaasSeen = "an actor counts as seen when it is one of the eyes, or a token " +
	"of it stands on a square the eyes see"

// VTT-472 VTT-365
func TestQAASAShoulderSeesItselfAndWhatStandsInItsSight(t *testing.T) {
	st := qaasHall(t).st
	qaasExpect(t, perceive.Eyes{Shoulder: "hero"}, st, map[string]bool{
		"hero":   true,
		"near":   true,
		"goblin": true,
		"twin":   true,
		"beyond": false,
		"ally":   false,
		"scout":  false,
		"lodger": false,
		"rumour": false,
		"nobody": false,
	}, qaasSeen)
}

// VTT-472
func TestQAASAnOpenedDoorLetsTheShoulderSeeWhatStandsBeyondIt(t *testing.T) {
	b := qaasHall(t)
	if perceive.SeesActor(perceive.Eyes{Shoulder: "hero"}, b.st, "beyond") {
		t.Fatal("beyond is seen through the closed door")
	}
	b.openDoor("hall", 5, 2)
	if !perceive.SeesActor(perceive.Eyes{Shoulder: "hero"}, b.st, "beyond") {
		t.Error("beyond is not seen through the open door")
	}
}

// VTT-365
func TestQAASAShoulderWithNoTokenStillSeesItself(t *testing.T) {
	st := qaasNew(t).
		scene("hall", 12, 5, qaasHallTiles()).
		actor("hero", qaasParty).
		actor("near", qaasNonParty).
		token("tok-near", "hall", "near", 3, 2).st
	qaasExpect(t, perceive.Eyes{Shoulder: "hero"}, st, map[string]bool{
		"hero": true,
		"near": false,
	}, qaasSeen)
}

// VTT-472
func TestQAASASceneWithNoTilesHidesNothingAtAnyDistance(t *testing.T) {
	st := qaasNew(t).
		scene("field", 40, 40, nil).
		actor("hero", qaasParty).
		actor("far", qaasNonParty).
		token("tok-hero", "field", "hero", 0, 0).
		token("tok-far", "field", "far", 39, 39).st
	qaasExpect(t, perceive.Eyes{Shoulder: "hero"}, st, map[string]bool{"far": true},
		"a scene with no tiles blocks nothing")
}

// VTT-472 VTT-453
func TestQAASAnotherShoulderSeesFromItsOwnSquare(t *testing.T) {
	st := qaasHall(t).st
	qaasExpect(t, perceive.Eyes{Shoulder: "scout"}, st, map[string]bool{
		"scout":  true,
		"beyond": true,
		"ally":   true,
		"twin":   true,
		"near":   false,
		"hero":   false,
		"goblin": false,
	}, qaasSeen)
}

// SPEC-016
func TestQAASNoEyesAndANonPartyShoulderSeeNobody(t *testing.T) {
	st := qaasHall(t).st
	for _, eyes := range []perceive.Eyes{{}, {Shoulder: "goblin"}, {Shoulder: "nobody"}} {
		qaasExpect(t, eyes, st, map[string]bool{
			"goblin": false,
			"near":   false,
			"hero":   false,
		}, "SPEC-016, Whose eyes a viewer has: a Shoulder is the actor it names "+
			"when st.Actors holds it and engine.IsPartyMember accepts it, "+
			"and otherwise none")
	}
}

// SPEC-016
func TestQAASControllerEyesSeeThroughEveryActorTheParticipantControls(t *testing.T) {
	st := qaasHall(t).st
	qaasExpect(t, perceive.Eyes{Controller: "p1"}, st, map[string]bool{
		"hero":   true,
		"scout":  true,
		"near":   true,
		"beyond": true,
		"ally":   true,
		"lodger": false,
		"rumour": false,
	}, "SPEC-016, Whose eyes a viewer has: eyes give a Controller every actor "+
		"in st.Actors whose ControllerIds holds it")
}

// SPEC-016
func TestQAASSeesActorWritesNothingToTheState(t *testing.T) {
	st := qaasHall(t).st
	before, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	first := perceive.SeesActor(perceive.Eyes{Shoulder: "hero"}, st, "beyond")
	perceive.SeesActor(perceive.Eyes{Shoulder: "scout"}, st, "beyond")
	perceive.SeesActor(perceive.Eyes{Controller: "p1"}, st, "near")
	again := perceive.SeesActor(perceive.Eyes{Shoulder: "hero"}, st, "beyond")
	after, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("SeesActor changed the state (SPEC-016, What a look sees: " +
			"look keeps no memo and writes nothing to st)")
	}
	if first || again {
		t.Errorf("hero sees beyond: first %v, again %v", first, again)
	}
}
