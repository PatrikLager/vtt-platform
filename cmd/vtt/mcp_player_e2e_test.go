package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/harness"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func mintParticipant(t *testing.T, campaignPath string, role identity.Role, name string) (token, id string) {
	t.Helper()
	ids, err := identity.Open(campaign.LogPath(campaignPath))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	defer ids.Close()
	token, id, err = ids.CreateInvite(name, role)
	if err != nil {
		t.Fatalf("CreateInvite(%s): %v", role, err)
	}
	return token, id
}

func freshDialOf(t *testing.T, wsURL, token, log string) []*vttv1.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := harness.Dial(ctx, wsURL, token, 0, log)
	if err != nil {
		t.Fatalf("harness.Dial log=%s: %v", log, err)
	}
	defer c.Close()
	head, err := c.CatchUpHead(ctx)
	if err != nil {
		t.Fatalf("a fresh dial of %s: catch-up head: %v", log, err)
	}
	envs, held, reached, why := drainToHead(c.Events(), c.CloseErr, 0, head, 200*time.Millisecond, 10*time.Second)
	if !reached || len(envs) == 0 || held == nil || *held != log {
		t.Fatalf("a fresh dial of %s read %d entries of head %d, holding %v: %v", log, len(envs), head, held, why)
	}
	return envs
}

type eventsSinceLog struct {
	Events       []json.RawMessage `json:"events"`
	HeadSequence int64             `json:"headSequence"`
	Log          *string           `json:"log"`
}

func readEventsSince(t *testing.T, cs *mcpsdk.ClientSession) eventsSinceLog {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_events_since", Arguments: map[string]any{"afterSequence": 0, "limit": 200}})
	if err != nil || res.IsError {
		t.Fatalf("get_events_since: err=%v result=%+v", err, res)
	}
	var out eventsSinceLog
	if err := json.Unmarshal([]byte(res.Content[0].(*mcpsdk.TextContent).Text), &out); err != nil {
		t.Fatalf("get_events_since: %v", err)
	}
	return out
}

func waitForMCPLog(t *testing.T, cs *mcpsdk.ClientSession, log string, atLeast int64) eventsSinceLog {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var last eventsSinceLog
	for time.Now().Before(deadline) {
		last = readEventsSince(t, cs)
		if last.Log != nil && *last.Log == log && last.HeadSequence >= atLeast {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("get_events_since never named log %q at head %d or above (last log=%v head %d)", log, atLeast, last.Log, last.HeadSequence)
	return last
}

// VTT-418 VTT-422
func TestMCPSeatedAsAPlayerFollowsItsSetViewpoint(t *testing.T) {
	fx := startMCPFixture(t)
	dmToken := mintInviteToken(t, fx.campaignPath, identity.RoleDM, "dm")
	playerToken, playerID := mintParticipant(t, fx.campaignPath, identity.RolePlayer, "player")
	dm, closeDM := startMCPSession(t, fx.wsURL, dmToken)
	defer closeDM()
	player, closePlayer := startMCPSession(t, fx.wsURL, playerToken)
	defer closePlayer()

	for _, id := range []string{"act-ash", "act-bram"} {
		mustCallToolOK(t, dm, "add_actor", map[string]any{"actor": map[string]any{"actorId": id, "name": id, "kind": "ACTOR_KIND_PARTY_MEMBER"}})
		mustCallToolOK(t, dm, "grant_actor_control", map[string]any{"actorId": id, "participantId": playerID, "kind": "ACTOR_KIND_PARTY_MEMBER"})
		if id == "act-ash" {
			mustCallToolOK(t, dm, "add_narration", map[string]any{"text": "Only act-ash's log was open to hear this."})
		}
	}
	ash := freshDialOf(t, fx.wsURL, playerToken, "act-ash")
	waitForMCPLog(t, player, "act-ash", ash[len(ash)-1].GetSequence())

	mustCallToolOK(t, player, "set_viewpoint", map[string]any{"actorId": "act-bram"})
	want := freshDialOf(t, fx.wsURL, playerToken, "act-bram")
	since := waitForMCPLog(t, player, "act-bram", want[len(want)-1].GetSequence())
	if len(since.Events) != len(want) {
		t.Fatalf("get_events_since from 0 after set_viewpoint act-bram = %d entries, a fresh dial of act-bram's log %d", len(since.Events), len(want))
	}
	for i, raw := range since.Events {
		var got vttv1.Envelope
		if err := protojson.Unmarshal(raw, &got); err != nil || !proto.Equal(&got, want[i]) {
			t.Fatalf("get_events_since entry %d = %s, a fresh dial of act-bram's log has %v (err %v)", i, raw, want[i], err)
		}
	}

	actors, _ := callGetStateGeneric(t, player)["Actors"].(map[string]any)
	bram, _ := actors["act-bram"].(map[string]any)
	if controllers, _ := bram["controller_ids"].([]any); len(controllers) != 1 || controllers[0] != playerID {
		t.Fatalf("get_state after set_viewpoint act-bram folds act-bram as %v, want it controlled by the player", bram)
	}
}
