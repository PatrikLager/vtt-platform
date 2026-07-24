// Package mcp is the MCP stdio server that seats an LLM at the table as the
// agent participant (docs/superpowers/specs/2026-07-24-mcp-gateway-design.md).
// It is a WIRE CLIENT — internal/harness's second consumer — and adds zero
// privilege: every tool call becomes a ClientCommand the gateway judges with
// its ordinary authz table, stamped with the agent's participant_id.
//
// Deliberate boundary (P1 rule, extended from internal/harness's original):
// this package imports ONLY contract types (vttv1), internal/harness,
// internal/engine, the official MCP SDK, and stdlib — never
// internal/gateway, internal/campaign, internal/identity, or internal/store
// (.go-arch-lint.yml's mcp component enforces this, test files included).
package mcp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// instructions is the server-level MCP Instructions string (spec §5): role,
// poll pattern, and the wire conventions every consumer of contract/README.md
// must know, summarized to ≤20 lines. Deliberately carries NO game rules —
// rule-module LLM affordances belong to a later sub-project's layer.
const instructions = `You are the agent participant at this table. Every
action you take becomes an event stamped with YOUR participant id — human
players can see everything you do. Every tool call passes through the same
authz table a human client would; a rejected call means this table's rules
do not permit that action for you right now, not that something is broken.

Poll pattern: act (call a command tool) -> read its result (ok/error) ->
call get_events_since to see what else happened, yours and others', before
acting again. There are no push notifications here; you must poll.

Wire conventions (contract/README.md is the full constitution):
 - int64 fields serialize as JSON STRINGS in results and event envelopes
   (e.g. "sequence": "42"), never bare JSON numbers.
 - An event envelope has no "type" field: its payload is one oneof key per
   event kind, e.g. {"tokenMoved": {...}}.
 - Actor.moduleData is an opaque object — do not interpret its shape.
 - A command argument is required unless the contract marks the underlying
   field "optional"; each tool's inputSchema states this per-field.`

// redialInitialBackoff/redialMaxBackoff bound the reconnect loop's simple
// exponential backoff (pump's redial). No jitter: the reconnect target is a
// single gateway process behind one WebSocket URL, not a fleet where a
// thundering herd is a real risk, so the extra complexity was judged not
// worth it for this task. Ledgered as a candidate refinement if a future
// review disagrees.
const (
	redialInitialBackoff = 100 * time.Millisecond
	redialMaxBackoff     = 2 * time.Second
)

// errUnimplemented is Step 1's compiling-stub sentinel (ADR-009: every
// method must return a real zero value/error, not panic, so the behavioral
// RED in server_test.go/tools_test.go exercises real assertions instead of
// a compile failure).
var errUnimplemented = errors.New("mcp: unimplemented")

// Config configures a Server. ToolsJSON is the raw committed contract/gen/
// tools/tools.json bytes — Server never reads the filesystem itself (Task 3
// supplies it via go:embed from cmd/vtt so a single binary needs no sidecar
// file).
type Config struct {
	WSURL     string
	Token     string
	ToolsJSON []byte
}

// Server is the MCP server core: session lifecycle plus generic command
// dispatch built from Config.ToolsJSON. The zero value is not usable; a
// Server is only ever obtained from New.
type Server struct {
	cfg      Config
	mcp      *mcpsdk.Server
	dispatch map[string]protoreflect.FieldDescriptor // tool name -> ClientCommand oneof field

	mu      sync.Mutex
	client  *harness.Client // nil while disconnected
	lastSeq int64           // highest event sequence seen so far, for redial's after= cursor
}

// New validates cfg, parses ToolsJSON, builds the tool-name -> ClientCommand
// oneof-field dispatch table (erroring if tools.json and the oneof
// disagree — buildDispatch's doc comment has the detail), and registers one
// generic MCP tool per entry. New performs no I/O: dialing happens in Run.
func New(cfg Config) (*Server, error) {
	if cfg.WSURL == "" {
		return nil, errors.New("mcp: Config.WSURL is required")
	}
	if len(cfg.ToolsJSON) == 0 {
		return nil, errors.New("mcp: Config.ToolsJSON is required")
	}

	entries, err := parseToolsJSON(cfg.ToolsJSON)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	dispatch, err := buildDispatch(names)
	if err != nil {
		return nil, err
	}

	s := &Server{cfg: cfg, dispatch: dispatch}
	s.mcp = mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: "vtt-mcp", Version: "0.1.0"},
		&mcpsdk.ServerOptions{Instructions: instructions},
	)

	for _, e := range entries {
		fd := dispatch[e.Name]
		s.mcp.AddTool(&mcpsdk.Tool{
			Name:        e.Name,
			Description: e.Description,
			InputSchema: e.InputSchema,
		}, s.handlerFor(fd))
	}

	return s, nil
}

// Run dials the gateway via harness.Dial and serves the MCP protocol over
// transport for one session (one connection per Run lifetime), reconnecting
// the underlying wire client on connection loss: redial uses after=<last
// seen event sequence>, and tool calls made while disconnected return clean
// MCP errors (never a crash — see handlerFor/currentClient). Run blocks
// until the client disconnects or ctx is canceled, mirroring
// mcpsdk.Server.Run's own contract.
func (s *Server) Run(ctx context.Context, transport mcpsdk.Transport) error {
	client, err := harness.Dial(ctx, s.cfg.WSURL, s.cfg.Token, 0)
	if err != nil {
		return fmt.Errorf("mcp: initial dial: %w", err)
	}
	s.setClient(client)

	pumpCtx, cancelPump := context.WithCancel(ctx)
	defer cancelPump()
	go s.pump(pumpCtx)

	defer func() {
		if c := s.currentClient(); c != nil {
			c.Close()
		}
	}()

	return s.mcp.Run(ctx, transport)
}

// pump owns the wire client's Events() stream for the lifetime of Run: it
// must be drained continuously (harness.Client tears itself down if its
// event buffer overflows), and it is the only place lastSeq advances, which
// is what redial's after= cursor is built from. On connection loss it hands
// off to redial and resumes draining once (if) a new client is installed.
func (s *Server) pump(ctx context.Context) {
	for {
		client := s.currentClient()
		if client == nil {
			if !s.redial(ctx) {
				return
			}
			client = s.currentClient()
		}

		for env := range client.Events() {
			// Dedupe by sequence (harness client semantics): a redial's
			// catch-up history plus this connection's live stream are
			// already deduped server-side per connection (store.Store's
			// notifyLocked), but a connection drop can leave events
			// buffered-and-undrained at teardown; only ever advance
			// forward, never regress or reprocess a sequence already seen.
			if seq := env.GetSequence(); seq > s.lastSeen() {
				s.setLastSeen(seq)
			}
		}

		// Events() closed: the connection is gone.
		s.setClient(nil)
		if ctx.Err() != nil {
			return
		}
	}
}

// redial retries harness.Dial with after=<lastSeen> until it succeeds or ctx
// is canceled, using a simple capped exponential backoff. It installs the
// new client via setClient on success. Returns false if ctx was canceled
// before a connection could be established.
func (s *Server) redial(ctx context.Context) bool {
	backoff := redialInitialBackoff
	for {
		if ctx.Err() != nil {
			return false
		}
		client, err := harness.Dial(ctx, s.cfg.WSURL, s.cfg.Token, s.lastSeen())
		if err == nil {
			s.setClient(client)
			return true
		}
		select {
		case <-time.After(backoff):
			if backoff < redialMaxBackoff {
				backoff *= 2
				if backoff > redialMaxBackoff {
					backoff = redialMaxBackoff
				}
			}
		case <-ctx.Done():
			return false
		}
	}
}

func (s *Server) currentClient() *harness.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

func (s *Server) setClient(c *harness.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = c
}

func (s *Server) lastSeen() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSeq
}

func (s *Server) setLastSeen(seq int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastSeq = seq
}

// handlerFor returns the ONE generic tool handler shape used for every
// command tool: fd identifies which ClientCommand oneof field this tool
// name maps to (from buildDispatch), and everything else — building the
// submessage, unmarshaling arguments into it, wiring it into the oneof,
// sending it, and shaping the result — is identical regardless of WHICH
// command this is. There is deliberately no per-command switch (self-review
// requirement, task-1-brief.md): grep this file for "switch" to confirm.
func (s *Server) handlerFor(fd protoreflect.FieldDescriptor) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		client := s.currentClient()
		if client == nil {
			return nil, fmt.Errorf("mcp: not connected to the gateway (wire is down)")
		}

		mt, err := protoregistry.GlobalTypes.FindMessageByName(fd.Message().FullName())
		if err != nil {
			// Would mean vttv1 itself doesn't register a type the SAME
			// vttv1 build's descriptor just named — an internal
			// inconsistency, not a caller mistake.
			return nil, fmt.Errorf("mcp: no registered Go type for %s: %w", fd.Message().FullName(), err)
		}
		sub := mt.New().Interface()

		args := []byte(req.Params.Arguments)
		if len(args) == 0 {
			args = []byte("{}")
		}
		if err := protojson.Unmarshal(args, sub); err != nil {
			return nil, fmt.Errorf("mcp: invalid arguments for %s: %w", fd.Name(), err)
		}

		cmd := &vttv1.ClientCommand{}
		cmd.ProtoReflect().Set(fd, protoreflect.ValueOfMessage(sub.ProtoReflect()))

		result, err := client.SendCommand(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("mcp: command failed: %w", err)
		}

		raw, err := protojson.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("mcp: marshal command result: %w", err)
		}

		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(raw)}},
			IsError: !result.GetOk(),
		}, nil
	}
}
