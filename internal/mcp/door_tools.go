package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// The two READ tools that make the join door drivable by an agent (#45).
//
// Before them the door was advertised and undrivable, which is the failure T6
// exists to catch, one layer up. An agent could OPEN the door and ROTATE the
// link and then not tell anybody the URL, because rotate_join_link deliberately
// returns no secret — a secret must not travel the frame channel every
// participant reads — and no tool could read one either. It could promote a
// participant only by supplying an id it had no way to learn: a fresh spectator
// controls no actor, so their id appears nowhere in engine.State.
//
// NO NEW AUTHORITY. Both routes are already gated to dm/agent by the gateway
// (metadata.go's joinLinkRoles and participantRoles), and the reasoning for
// admitting an agent to the shared secret is recorded there. The MCP server
// presents the SAME token, so the gateway makes the same decision it already
// makes for the DM console. This closes an asymmetry rather than opening a door.
//
// HTTP rather than the WebSocket, because that is where the answers live: the
// roster and the link are identity state, not campaign events, and neither has
// a frame. Reading them over the socket would mean inventing one, and a
// PresenceSnapshot-shaped frame carrying a shared secret is exactly what
// rotate_join_link declines to do.

// joinLinkPath and participantsPath are the gateway routes these tools read.
const (
	joinLinkPath     = "/api/join-link"
	participantsPath = "/api/participants"
)

// metadataTimeout bounds a metadata read.
//
// Short on purpose: these are single-row SQLite reads behind a local HTTP
// handler, and an agent left waiting on one has no way to tell a slow table
// from a hung one. Failing in seconds with a message beats hanging in a
// conversation.
const metadataTimeout = 10 * time.Second

// httpOriginFrom turns the WebSocket endpoint into the HTTP origin beside it.
//
// Derived rather than configured, so the two cannot drift: a separate flag
// would be one more thing to get wrong, and wrong here means an agent politely
// reporting that a table has no join link when it is simply asking the wrong
// host.
//
// The QUERY IS DROPPED, which is the part that matters. The URL the CLI passes
// carries the token, and carrying it into every metadata request would put a
// credential in the request line — where it reaches logs and proxies. It
// travels in the Authorization header instead, which is what that header is
// for.
//
// An untranslatable scheme is an ERROR rather than a passthrough: answering
// "http://…" for an ftp:// input would send the token somewhere nobody meant.
func httpOriginFrom(wsURL string) (string, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("mcp: parse server url %q: %w", wsURL, err)
	}
	var scheme string
	switch u.Scheme {
	case "ws", "http":
		scheme = "http"
	case "wss", "https":
		scheme = "https"
	default:
		return "", fmt.Errorf("mcp: server url %q has scheme %q, want ws or wss", wsURL, u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("mcp: server url %q names no host", wsURL)
	}
	return scheme + "://" + u.Host, nil
}

func (s *Server) registerDoorTools() {
	s.mcp.AddTool(&mcpsdk.Tool{
		Name:        "get_join_link",
		Description: getJoinLinkDescription,
		InputSchema: emptyObjectSchema,
	}, s.handleGetJoinLink)

	s.mcp.AddTool(&mcpsdk.Tool{
		Name:        "get_participants",
		Description: getParticipantsDescription,
		InputSchema: emptyObjectSchema,
	}, s.handleGetParticipants)
}

const getJoinLinkDescription = "Read the table's shared join link, whether the door is open, and how many " +
	"admissions this opening has left. DM/agent only — the secret admits ANYBODY who holds it. " +
	"Use this after set_join_door or rotate_join_link, which deliberately return nothing: a secret " +
	"must not travel the channel every participant reads. The share URL is the server's own address " +
	"with `/?join=<secret>` — this returns the secret, not the URL, because only the operator knows " +
	"which address their players can reach. `admitted` and `admitLimit` bound what an open door can " +
	"mint; when they are equal the door admits nobody until it is opened again, and a joiner turned " +
	"away then sees exactly what a stranger sees."

const getParticipantsDescription = "List everyone at the table: participant id, display name, and role. " +
	"DM/agent only. This is how to learn the participantId that promote_participant, " +
	"grant_actor_control and revoke_actor_control all require — somebody who joined through the " +
	"shared link arrives as a spectator controlling no actor, so their id appears nowhere in " +
	"get_state. Revoked participants are omitted. Roles are \"dm\", \"agent\", \"player\" or " +
	"\"spectator\"."

// emptyObjectSchema is the input schema for a tool that takes no arguments.
var emptyObjectSchema = map[string]any{
	"type":       "object",
	"properties": map[string]any{},
	"required":   []string{},
}

func (s *Server) handleGetJoinLink(ctx context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	return s.readMetadata(ctx, joinLinkPath)
}

func (s *Server) handleGetParticipants(ctx context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	return s.readMetadata(ctx, participantsPath)
}

// readMetadata GETs one of the gateway's metadata routes and returns its JSON
// body verbatim.
//
// VERBATIM, and not re-shaped: the gateway already decided what these routes
// say, the DM console renders exactly this, and a second shape here would be a
// second contract to keep in step — the drift the tools.json golden exists to
// prevent, reintroduced by hand.
//
// A refusal is returned as a TOOL ERROR rather than as content, so the agent
// sees a failure instead of narrating an empty roster to its user. 401 and 403
// are reported distinctly from a transport failure, because they mean different
// things to whoever is debugging: the wrong token, versus a token whose role
// this route does not admit.
func (s *Server) readMetadata(ctx context.Context, path string) (*mcpsdk.CallToolResult, error) {
	origin, err := httpOriginFrom(s.cfg.WSURL)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+path, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: build request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: read %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcp: read %s: %w", path, err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("mcp: read %s: this server's token was not accepted", path)
	case http.StatusForbidden:
		return nil, fmt.Errorf("mcp: read %s: this server's token is not a DM or agent, and "+
			"only those may read it", path)
	default:
		return nil, fmt.Errorf("mcp: read %s: server said %s: %s",
			path, resp.Status, strings.TrimSpace(string(body)))
	}

	// Validated, not merely forwarded. A 200 carrying something that is not
	// JSON means a proxy rewrote the body, and handing that to an agent as
	// "the roster" is how a tool reports confident nonsense.
	if !json.Valid(body) {
		return nil, fmt.Errorf("mcp: read %s: the answer was not JSON", path)
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(body)}},
	}, nil
}
