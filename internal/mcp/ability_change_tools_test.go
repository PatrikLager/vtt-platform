package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func TestGiveAndTakeAbilityReachTheWireCarryingTheirArguments(t *testing.T) {
	seen := make(chan *vttv1.ClientCommand, 2)
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		seen <- cmd
		sendResult(t, conn, cmd.GetRequestId(), true, "", 7)
	})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	for _, tool := range []string{"give_ability", "take_ability"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: tool,
			Arguments: map[string]any{"actorId": "act-fighter", "abilityId": "rally"}})
		cancel()
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v result=%+v", tool, err, res)
		}
		var cmd *vttv1.ClientCommand
		select {
		case cmd = <-seen:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s never reached the wire", tool)
		}
		var actor, ability string
		switch tool {
		case "give_ability":
			actor, ability = cmd.GetGiveAbility().GetActorId(), cmd.GetGiveAbility().GetAbilityId()
		default:
			actor, ability = cmd.GetTakeAbility().GetActorId(), cmd.GetTakeAbility().GetAbilityId()
		}
		if actor != "act-fighter" || ability != "rally" {
			t.Errorf("%s sent %v, want its arm carrying act-fighter and rally", tool, cmd.GetCommand())
		}
	}
}
