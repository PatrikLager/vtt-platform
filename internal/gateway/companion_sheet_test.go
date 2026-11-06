package gateway_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

var sheetWireSecrets = []string{
	"bramheft", "brampool", "bramvow", "iron oath",
	"caraheft", "carapool", "caravow", "silver oath",
}

func sheetWireCompanion(t *testing.T, id, name, key, oath string) *vttv1.Actor {
	t.Helper()
	return &vttv1.Actor{
		ActorId: id, Name: name, ModuleId: "mod-" + key,
		Attributes: map[string]int32{key + "heft": 6},
		Resources:  map[string]*vttv1.Resource{key + "pool": {Current: 9, Max: 12}},
		ModuleData: qacsWireStruct(t, map[string]any{key + "vow": oath}),
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}
}

func sheetWireAdd(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func sheetWirePlace(tok, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: "cave", ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func sheetWireMove(tok string, fx, fy, tx, ty int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tok, SceneId: "cave",
		From: &vttv1.GridPosition{X: fx, Y: fy}, To: &vttv1.GridPosition{X: tx, Y: ty},
	}}}
}

func sheetWireNarrate(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

// VTT-658 VTT-659
func TestASeatIsSentNoCompanionsSheetBeforeItsCharacterSeesIt(t *testing.T) {
	w := qacsWireTable(t)
	playerTok, playerID, err := w.ids.CreateInvite("Ayla's player", identity.RolePlayer)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	watcherTok, _, err := w.ids.CreateInvite("A watcher", identity.RoleSpectator)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 7; x++ {
		for y := 0; y < 3; y++ {
			kind := "floor"
			if x == 3 {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	bram := func() *vttv1.Actor { return sheetWireCompanion(t, "act-bram", "Bram Holt", "bram", "iron oath") }
	cara := func() *vttv1.Actor { return sheetWireCompanion(t, "act-cara", "Cara Fenn", "cara", "silver oath") }
	for _, e := range []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "night one"}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "cave", Name: "Cave", GridWidth: 7, GridHeight: 3, Tiles: tiles,
		}}},
		sheetWireAdd(bram()),
		sheetWirePlace("tok-bram", "act-bram", 5, 1),
		sheetWireAdd(&vttv1.Actor{ActorId: "act-ayla", Name: "Ayla Venn", ModuleId: "mod-scout",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}),
		{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "act-ayla", ParticipantId: playerID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
		sheetWirePlace("tok-ayla", "act-ayla", 1, 1),
		sheetWireAdd(cara()),
		sheetWirePlace("tok-cara", "act-cara", 5, 2),
	} {
		w.append(e)
	}
	seats := map[string]*websocket.Conn{
		"player":    w.seat(playerTok, "act-ayla"),
		"spectator": w.seat(watcherTok, "act-ayla"),
	}
	w.append(sheetWireNarrate("qacs sheet unseen"))
	before := map[string][]*vttv1.Envelope{}
	for who, conn := range seats {
		raws, events := qacsReadUntil(t, conn, "qacs sheet unseen")
		for _, raw := range raws {
			for _, s := range sheetWireSecrets {
				if strings.Contains(raw, s) {
					t.Errorf("VTT-658: before Ayla sees them, the %s was sent a frame carrying %q: %s", who, s, raw)
				}
			}
		}
		before[who] = events
	}

	w.append(sheetWireMove("tok-bram", 5, 1, 2, 0))
	w.append(sheetWireMove("tok-cara", 5, 2, 2, 2))
	w.append(sheetWireNarrate("qacs sheet seen"))
	for who, conn := range seats {
		_, events := qacsReadUntil(t, conn, "qacs sheet seen")
		shown := engine.NewState()
		for i, ev := range append(before[who], events...) {
			if err := engine.Apply(shown, ev); err != nil {
				t.Fatalf("SPEC-021: the %s's event %d does not fold: %v\n%v", who, i, err, ev)
			}
		}
		for _, want := range []*vttv1.Actor{bram(), cara()} {
			whole := 0
			for _, ev := range events {
				a := ev.GetActorAdded().GetActor()
				if a.GetActorId() != want.GetActorId() {
					continue
				}
				if !proto.Equal(a, want) {
					t.Errorf("VTT-659: on sight the %s was introduced to %s as %v, want it whole", who, want.GetActorId(), a)
				}
				whole++
			}
			if whole != 1 {
				t.Errorf("VTT-659: on sight the %s was introduced to %s %d times, want once", who, want.GetActorId(), whole)
			}
			if held := shown.Actors[want.GetActorId()]; !proto.Equal(held, want) {
				t.Errorf("VTT-659: the %s holds %s as %v, want it whole", who, want.GetActorId(), held)
			}
		}
	}
}
