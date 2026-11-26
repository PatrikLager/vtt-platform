package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func TestTheReferenceToolsReachTheWireCarryingTheirArguments(t *testing.T) {
	seen := make(chan *vttv1.ClientCommand, 3)
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {
		seen <- cmd
		sendResult(t, conn, cmd.GetRequestId(), true, "", 7)
	})
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	for _, c := range []struct {
		tool string
		args map[string]any
		want *vttv1.ClientCommand
	}{
		{"add_reference", map[string]any{"actorId": "act-ada", "referenceId": "ref-echo",
			"reference": map[string]any{"entryId": "echo", "place": "mike", "state": "oscar"}},
			&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddReference{AddReference: &vttv1.AddReference{
				ActorId: "act-ada", ReferenceId: "ref-echo",
				Reference: &vttv1.Reference{EntryId: "echo", Place: "mike", State: "oscar"}}}}},
		{"change_reference", map[string]any{"actorId": "act-ada", "referenceId": "ref-echo", "place": "kilo"},
			&vttv1.ClientCommand{Command: &vttv1.ClientCommand_ChangeReference{ChangeReference: &vttv1.ChangeReference{
				ActorId: "act-ada", ReferenceId: "ref-echo", Place: "kilo"}}}},
		{"remove_reference", map[string]any{"actorId": "act-ada", "referenceId": "ref-echo"},
			&vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveReference{RemoveReference: &vttv1.RemoveReference{
				ActorId: "act-ada", ReferenceId: "ref-echo"}}}},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: c.tool, Arguments: c.args})
		cancel()
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v result=%+v", c.tool, err, res)
		}
		var cmd *vttv1.ClientCommand
		select {
		case cmd = <-seen:
		case <-time.After(3 * time.Second):
			t.Fatalf("%s never reached the wire", c.tool)
		}
		cmd.RequestId = ""
		if !proto.Equal(cmd, c.want) {
			t.Errorf("%s sent %v, want %v", c.tool, cmd, c.want)
		}
	}
}
