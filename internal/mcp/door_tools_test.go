package mcp_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// The door tools exist because the door was ADVERTISED AND UNDRIVABLE (#45):
// an agent could open it and rotate it and then not tell anybody the URL, and
// could promote a participant only by supplying an id it had no way to learn.
//
// These drive the real MCP session rather than the handlers, because the
// handlers being right is not the property that was missing — the WIRING was.
// This session has already shipped three seams that nothing crossed, each
// behind a fully green suite.

// callTool runs one tool and returns its text content, or the error.
func callTool(t *testing.T, cs *mcpsdk.ClientSession, name string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: name})
	if err != nil {
		return "", err
	}
	if res.IsError {
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcpsdk.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return "", &toolError{msg: b.String()}
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), nil
}

type toolError struct{ msg string }

func (e *toolError) Error() string { return e.msg }

func TestGetJoinLinkReturnsWhatTheGatewaySays(t *testing.T) {
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})

	var (
		mu               sync.Mutex
		gotPath, gotAuth string
	)
	fs.api = func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		mu.Unlock()
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"open":true,"secret":"s3cr3t","admitted":2,"admitLimit":8}`))
	}

	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	got, err := callTool(t, cs, "get_join_link")
	if err != nil {
		t.Fatalf("get_join_link: %v", err)
	}

	mu.Lock()
	path, auth := gotPath, gotAuth
	mu.Unlock()
	if path != "/api/join-link" {
		t.Fatalf("the tool asked for %q, want /api/join-link", path)
	}
	// The TOKEN, in the header. A tool that asked without one would get a 401
	// and report "no join link" for a table that has one.
	if auth != "Bearer test-token" {
		t.Fatalf("Authorization was %q — the gateway cannot tell who is asking", auth)
	}
	// VERBATIM. Re-shaping here would be a second contract to keep in step
	// with the DM console's, which is the drift the tools.json golden exists
	// to prevent.
	if !strings.Contains(got, `"secret":"s3cr3t"`) || !strings.Contains(got, `"admitLimit":8`) {
		t.Fatalf("the tool returned %q — the gateway's answer did not survive", got)
	}
}

func TestGetParticipantsAsksTheRosterRoute(t *testing.T) {
	// The other half of #45: promote_participant needs an id, and a fresh
	// spectator controls no actor, so their id appears nowhere in get_state.
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	var (
		mu      sync.Mutex
		gotPath string
	)
	fs.api = func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		mu.Unlock()
		_, _ = w.Write([]byte(`[{"participantId":"p-1","name":"Ada","role":"spectator"}]`))
	}

	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	got, err := callTool(t, cs, "get_participants")
	if err != nil {
		t.Fatalf("get_participants: %v", err)
	}
	mu.Lock()
	path := gotPath
	mu.Unlock()
	if path != "/api/participants" {
		t.Fatalf("the tool asked for %q, want /api/participants", path)
	}
	if !strings.Contains(got, `"participantId":"p-1"`) {
		t.Fatalf("the roster did not survive: %q", got)
	}
}

func TestARefusedMetadataReadIsAnErrorNotAnEmptyAnswer(t *testing.T) {
	// The direction that matters. Returning a 403 body as CONTENT would have
	// the agent narrate "the table has no join link" to its user, which is a
	// confident wrong answer — the failure this repo keeps finding. A tool
	// ERROR makes the agent say it could not read it.
	//
	// 401 and 403 are reported DISTINCTLY, because they send whoever debugs
	// this to different places: a token the server does not accept, versus a
	// token whose role this route does not admit.
	for _, c := range []struct {
		name   string
		status int
		want   string
	}{
		{"forbidden", http.StatusForbidden, "DM or agent"},
		{"unauthorized", http.StatusUnauthorized, "not accepted"},
		{"server error", http.StatusInternalServerError, "500"},
	} {
		t.Run(c.name, func(t *testing.T) {
			fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
			fs.api = func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "gateway: not authorized", c.status)
			}
			cs, cleanup := startSession(t, fs.wsURL())
			defer cleanup()

			got, err := callTool(t, cs, "get_join_link")
			if err == nil {
				t.Fatalf("a %d was returned as content: %q", c.status, got)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("the error says %q, want it to mention %q", err.Error(), c.want)
			}
		})
	}
}

func TestAnAnswerThatIsNotJSONIsRefused(t *testing.T) {
	// A 200 carrying something that is not JSON means a proxy rewrote the
	// body. Handing that to an agent as "the roster" is how a tool reports
	// confident nonsense.
	fs := newFakeServer(t, func(conn *websocket.Conn, cmd *vttv1.ClientCommand) {})
	fs.api = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>your wifi portal</html>"))
	}
	cs, cleanup := startSession(t, fs.wsURL())
	defer cleanup()

	if got, err := callTool(t, cs, "get_join_link"); err == nil {
		t.Fatalf("a non-JSON body was accepted as the answer: %q", got)
	}
}
