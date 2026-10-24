package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

var qacsWireSecrets = []string{
	"Gorvash the Sly", "Goblin Chief Vrask", "carries the vault key",
	"smoke hollow", "grit", "cunning", "Ayla (pc)", "owes the guild",
}

type qacsWire struct {
	t   *testing.T
	c   *campaign.Campaign
	ids *identity.DB
	srv *httptest.Server
	n   int
}

func (w *qacsWire) append(envs ...*vttv1.Envelope) {
	w.t.Helper()
	for _, e := range envs {
		w.n++
		e.EventId = fmt.Sprintf("qacs-wire-%d", w.n)
		e.OccurredAt = timestamppb.Now()
		e.ParticipantId = "p-dm"
		e.ActorRole = "dm"
	}
	if _, err := w.c.AppendBatch(envs); err != nil {
		w.t.Fatalf("append: %v", err)
	}
}

func qacsWireTable(t *testing.T) *qacsWire {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("open campaign: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("open identity: %v", err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		srv.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qacsWire{t: t, c: c, ids: ids, srv: srv}
}

func (w *qacsWire) seat(token, log string) *websocket.Conn {
	w.t.Helper()
	u := "ws" + strings.TrimPrefix(w.srv.URL, "http") + "/ws?token=" + url.QueryEscape(token) + "&log=" + url.QueryEscape(log)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, u, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		w.t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(1 << 22)
	w.t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })
	return conn
}

func qacsReadUntil(t *testing.T, conn *websocket.Conn, sentinel string) ([]string, []*vttv1.Envelope) {
	t.Helper()
	var raws []string
	var events []*vttv1.Envelope
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		_, data, err := conn.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("read before %q arrived: %v", sentinel, err)
		}
		raws = append(raws, string(data))
		var f vttv1.ServerFrame
		if err := protojson.Unmarshal(data, &f); err != nil {
			t.Fatalf("decode %s: %v", data, err)
		}
		if ev := f.GetEvent(); ev != nil {
			events = append(events, ev)
			if ev.GetNarrationAdded().GetText() == sentinel {
				return raws, events
			}
		}
	}
	t.Fatalf("%q never arrived", sentinel)
	return nil, nil
}

func qacsWireStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("struct: %v", err)
	}
	return s
}

func qacsWireWorld(t *testing.T, player string) []*vttv1.Envelope {
	t.Helper()
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
	hero := &vttv1.Actor{
		ActorId: "act-ayla", Name: "Ayla Venn", ModuleId: "mod-scout",
		Attributes: map[string]int32{"brawn": 3},
		Resources:  map[string]*vttv1.Resource{"vigor": {Current: 7, Max: 10}},
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}
	beast := &vttv1.Actor{
		ActorId: "act-gob", Name: "Gorvash the Sly", ModuleId: "mod-goblin",
		Attributes: map[string]int32{"brawn": 2, "cunning": 7},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 4, Max: 6}},
		ModuleData: qacsWireStruct(t, map[string]any{"lair": "smoke hollow"}),
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}
	return []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "night one"}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "cave", Name: "Cave", GridWidth: 7, GridHeight: 3, Tiles: tiles,
		}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: hero}}},
		{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "act-ayla", ParticipantId: player, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: beast}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-ayla", SceneId: "cave", ActorId: "act-ayla", Position: &vttv1.GridPosition{X: 1, Y: 1},
			Name: "Ayla", Notes: "a scout in green", GmName: "Ayla (pc)", GmNotes: "owes the guild",
		}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-gob", SceneId: "cave", ActorId: "act-gob", Position: &vttv1.GridPosition{X: 2, Y: 1},
			Name: "a hunched figure", Notes: "it smells of smoke",
			GmName: "Goblin Chief Vrask", GmNotes: "carries the vault key",
		}}},
	}
}

// VTT-465 VTT-466 VTT-462
func TestQACSAPlayerAndASpectatorWhoseCharacterSeesACreatureAreSentNoFrameWithItsNameOrResources(t *testing.T) {
	w := qacsWireTable(t)
	playerTok, playerID, err := w.ids.CreateInvite("Ayla's player", identity.RolePlayer)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	watcherTok, _, err := w.ids.CreateInvite("A watcher", identity.RoleSpectator)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	w.append(qacsWireWorld(t, playerID)...)
	seats := map[string]*websocket.Conn{
		"player":    w.seat(playerTok, "act-ayla"),
		"spectator": w.seat(watcherTok, "act-ayla"),
	}
	w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: "act-gob", Resource: "grit", Delta: -3, NewValue: 1, Reason: "a falling rock",
	}}})
	w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
		Text: "qacs sentinel",
	}}})
	for who, conn := range seats {
		raws, events := qacsReadUntil(t, conn, "qacs sentinel")
		for _, raw := range raws {
			for _, s := range qacsWireSecrets {
				if strings.Contains(raw, s) {
					t.Errorf("VTT-465 VTT-466 VTT-462: the %s was sent a frame carrying %q: %s", who, s, raw)
				}
			}
		}
		shown := engine.NewState()
		var introduced, placed int
		for i, ev := range events {
			if err := engine.Apply(shown, ev); err != nil {
				t.Fatalf("SPEC-021: the %s's event %d does not fold: %v\n%v", who, i, err, ev)
			}
			if a := ev.GetActorAdded().GetActor(); a.GetActorId() == "act-gob" {
				introduced++
				want := &vttv1.Actor{ActorId: "act-gob", ModuleId: "mod-goblin", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}
				if !proto.Equal(a, want) {
					t.Errorf("VTT-465: the %s was introduced to the creature as %v", who, a)
				}
			}
			if p := ev.GetTokenPlaced(); p.GetTokenId() == "tok-gob" {
				placed++
				if p.GetName() != "a hunched figure" || p.GetNotes() != "it smells of smoke" {
					t.Errorf("VTT-478: the %s was sent the creature's token as %v", who, p)
				}
			}
			if rc := ev.GetResourceChanged(); rc.GetActorId() == "act-gob" {
				t.Errorf("VTT-466: the %s was sent %v", who, rc)
			}
		}
		if introduced != 1 || placed != 1 {
			t.Errorf("the %s was introduced to the creature %d times and its token placed %d times, want once each", who, introduced, placed)
		}
	}
}

// VTT-176
func TestQACSTheDMIsSentTheCreatureAndTheTokenWhole(t *testing.T) {
	w := qacsWireTable(t)
	dmTok, _, err := w.ids.CreateInvite("The DM", identity.RoleDM)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	w.append(qacsWireWorld(t, "p-nobody")...)
	conn := w.seat(dmTok, "act-ayla")
	w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: "act-gob", Resource: "grit", Delta: -3, NewValue: 1, Reason: "a falling rock",
	}}})
	w.append(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
		Text: "qacs sentinel",
	}}})
	raws, _ := qacsReadUntil(t, conn, "qacs sentinel")
	all := strings.Join(raws, "\n")
	for _, s := range qacsWireSecrets {
		if !strings.Contains(all, s) {
			t.Errorf("VTT-176: the DM was never sent %q", s)
		}
	}
}
