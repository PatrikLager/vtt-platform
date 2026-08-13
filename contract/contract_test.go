package contract_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func TestSceneCarriesTerrainAndDoorsExist(t *testing.T) {
	sc := &vttv1.SceneCreated{
		SceneId: "s1", Name: "Cellar", GridWidth: 3, GridHeight: 3,
		Tiles: map[string]*vttv1.TileRef{
			"0,0": {Kind: "wall", Material: "stone", Art: ""},
			"1,1": {Kind: "floor", Material: "wood", Art: "planks-3"},
		},
		Objects: []*vttv1.SceneObject{{
			ObjectId: "o1", Kind: "boulder",
			At:              &vttv1.GridPosition{X: 1, Y: 1},
			Width:           1,
			Height:          1,
			RotationDegrees: 90,
			BlocksSight:     true,
			BlocksMove:      true,
			Art:             "boulder-02",
		}},
	}

	// A real wire round trip, not just a struct-field read: this is what
	// would actually catch a field silently dropped by the generated
	// marshalling code, which a bare struct literal read cannot.
	sceneWire, err := proto.Marshal(sc)
	if err != nil {
		t.Fatalf("marshal SceneCreated: %v", err)
	}
	gotScene := &vttv1.SceneCreated{}
	if err := proto.Unmarshal(sceneWire, gotScene); err != nil {
		t.Fatalf("unmarshal SceneCreated: %v", err)
	}
	wall := gotScene.GetTiles()["0,0"]
	if wall.GetKind() != "wall" || wall.GetMaterial() != "stone" {
		t.Fatalf("wall tile did not survive the round trip: %+v", wall)
	}
	if gotScene.GetTiles()["1,1"].GetArt() != "planks-3" {
		t.Fatal("art did not survive the round trip")
	}
	obj := gotScene.GetObjects()[0]
	if obj.GetObjectId() != "o1" || obj.GetKind() != "boulder" {
		t.Fatalf("SceneObject identity did not survive the round trip: %+v", obj)
	}
	if obj.GetAt().GetX() != 1 || obj.GetAt().GetY() != 1 {
		t.Fatal("SceneObject.At did not survive the round trip")
	}
	if obj.GetWidth() != 1 || obj.GetHeight() != 1 {
		t.Fatal("SceneObject footprint did not survive the round trip")
	}
	if obj.GetRotationDegrees() != 90 {
		t.Fatal("SceneObject.RotationDegrees did not survive the round trip")
	}
	if !obj.GetBlocksSight() || !obj.GetBlocksMove() {
		t.Fatal("SceneObject blocking flags did not survive the round trip")
	}
	if obj.GetArt() != "boulder-02" {
		t.Fatal("SceneObject.Art did not survive the round trip")
	}

	// Doors are a nature plus folded state, never two natures (spec §3.3).
	// Round-tripped on the wire too, for the same reason as SceneCreated
	// above: a struct-field read cannot catch a field the generated
	// marshalling code silently drops.
	doorOpenedWire, err := proto.Marshal(&vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
		DoorOpened: &vttv1.DoorOpened{SceneId: "s1", At: &vttv1.GridPosition{X: 0, Y: 1}}}})
	if err != nil {
		t.Fatalf("marshal DoorOpened envelope: %v", err)
	}
	gotDoorOpened := &vttv1.Envelope{}
	if err := proto.Unmarshal(doorOpenedWire, gotDoorOpened); err != nil {
		t.Fatalf("unmarshal DoorOpened envelope: %v", err)
	}
	if gotDoorOpened.GetDoorOpened().GetSceneId() != "s1" || gotDoorOpened.GetDoorOpened().GetAt().GetY() != 1 {
		t.Fatal("DoorOpened lost its scene or position on the wire")
	}

	doorClosedWire, err := proto.Marshal(&vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
		DoorClosed: &vttv1.DoorClosed{SceneId: "s1", At: &vttv1.GridPosition{X: 0, Y: 1}}}})
	if err != nil {
		t.Fatalf("marshal DoorClosed envelope: %v", err)
	}
	gotDoorClosed := &vttv1.Envelope{}
	if err := proto.Unmarshal(doorClosedWire, gotDoorClosed); err != nil {
		t.Fatalf("unmarshal DoorClosed envelope: %v", err)
	}
	if gotDoorClosed.GetDoorClosed().GetSceneId() != "s1" || gotDoorClosed.GetDoorClosed().GetAt().GetY() != 1 {
		t.Fatal("DoorClosed lost its scene or position on the wire")
	}

	openDoorWire, err := proto.Marshal(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_OpenDoor{
		OpenDoor: &vttv1.OpenDoor{SceneId: "s1", At: &vttv1.GridPosition{X: 0, Y: 1}}}})
	if err != nil {
		t.Fatalf("marshal OpenDoor command: %v", err)
	}
	gotOpenDoor := &vttv1.ClientCommand{}
	if err := proto.Unmarshal(openDoorWire, gotOpenDoor); err != nil {
		t.Fatalf("unmarshal OpenDoor command: %v", err)
	}
	if gotOpenDoor.GetOpenDoor().GetSceneId() != "s1" || gotOpenDoor.GetOpenDoor().GetAt().GetY() != 1 {
		t.Fatal("OpenDoor lost its scene or position on the wire")
	}

	closeDoorWire, err := proto.Marshal(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_CloseDoor{
		CloseDoor: &vttv1.CloseDoor{SceneId: "s1", At: &vttv1.GridPosition{X: 0, Y: 1}}}})
	if err != nil {
		t.Fatalf("marshal CloseDoor command: %v", err)
	}
	gotCloseDoor := &vttv1.ClientCommand{}
	if err := proto.Unmarshal(closeDoorWire, gotCloseDoor); err != nil {
		t.Fatalf("unmarshal CloseDoor command: %v", err)
	}
	if gotCloseDoor.GetCloseDoor().GetSceneId() != "s1" || gotCloseDoor.GetCloseDoor().GetAt().GetY() != 1 {
		t.Fatal("CloseDoor lost its scene or position on the wire")
	}
}
