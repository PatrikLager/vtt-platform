// Package harness is the wire client core for the simulation harness (P1
// proof): a hand-rolled WebSocket client that speaks the gateway's wire
// protocol (contract/README.md's wire conventions, docs/superpowers/specs/
// 2026-07-23-api-gateway-design.md §3) directly, from OUTSIDE the platform.
//
// Deliberate boundary: this package imports ONLY contract types (vttv1),
// internal/engine, coder/websocket, and stdlib — never internal/gateway,
// internal/campaign, internal/identity, or internal/store. Task 1's binding
// rule is that a headless client can drive a live table using nothing but
// the wire constitution, the same way a human client or the LLM's MCP tools
// eventually will. protojson framing (ClientCommand out, ServerFrame in) is
// therefore re-implemented here rather than reused from internal/gateway's
// codec.go — a deliberate, documented duplication, not an oversight.
package harness

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// eventBuffer bounds Events(): the same 256 the gateway itself uses for its
// per-connection subscription (internal/gateway/server.go's gatewayBuffer),
// re-declared here rather than imported (the whole point of this package's
// boundary — see the package comment). Unlike the gateway, which reacts to
// ITS producer (the store) overflowing by closing just that connection,
// THIS side has no way to signal the server to slow down; a client that
// can't keep its own Events() consumer fed within this much slack has
// fallen behind in a way the wire protocol can't recover from, so the
// client tears itself down (see deliverEvent) and the caller must Dial
// again with a fresh `after` cursor, the same recovery story the gateway's
// own overflow gives every other consumer.
const eventBuffer = 256

// ErrEventsOverflow is the error a Client tears itself down with when a
// caller isn't draining Events() fast enough to keep up with the server's
// broadcast stream (see eventBuffer's doc comment).
var ErrEventsOverflow = errors.New("harness: events buffer overflow — caller too slow draining Events()")

// errClientClosed is the fallback teardown error, used when a connection
// ends locally (Close) or the underlying error is otherwise unavailable.
var errClientClosed = errors.New("harness: client closed")

// Client is a connected wire client: one WebSocket to the gateway, demuxing
// ServerFrame results (correlated to SendCommand callers by request_id) from
// the broadcast event stream (Events()). The zero value is not usable; a
// Client is only ever obtained from Dial.
type Client struct {
	conn *websocket.Conn

	// readCtx bounds the reader goroutine's Read calls; canceled by Close.
	// It is independent from any per-call context passed to SendCommand.
	readCtx context.Context
	cancel  context.CancelFunc

	events chan *vttv1.Envelope

	mu      sync.Mutex
	pending map[string]chan *vttv1.CommandResult
	closed  bool
	// closeErr is the reason the connection ended: the read error that
	// tripped teardown, ErrEventsOverflow, or errClientClosed for a local
	// Close with no prior read error. Every pending/future SendCommand call
	// reports this as its error once set.
	closeErr error

	reqSeq uint64

	// readerDone closes once the reader goroutine has fully exited — Close
	// waits on it so a caller never observes a "closed" Client whose
	// goroutine is still unwinding.
	readerDone chan struct{}
}

// Dial connects to the gateway's /ws endpoint at wsURL with the given invite
// token and catch-up cursor (after), and starts demuxing inbound frames.
// ctx bounds only the connection handshake; the returned Client's lifetime
// is independent of ctx (mirroring the gateway's own connection lifecycle —
// see internal/gateway/server.go's serve, which does the same against
// r.Context()).
func Dial(ctx context.Context, wsURL, token string, after int64) (*Client, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("harness: parse wsURL: %w", err)
	}
	q := u.Query()
	q.Set("token", token)
	q.Set("after", strconv.FormatInt(after, 10))
	u.RawQuery = q.Encode()

	conn, _, err := websocket.Dial(ctx, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("harness: dial: %w", err)
	}

	readCtx, cancel := context.WithCancel(context.Background())
	c := &Client{
		conn:       conn,
		readCtx:    readCtx,
		cancel:     cancel,
		events:     make(chan *vttv1.Envelope, eventBuffer),
		pending:    make(map[string]chan *vttv1.CommandResult),
		readerDone: make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

// SendCommand assigns cmd a fresh request_id, sends it, and blocks until the
// matching CommandResult arrives — events keep flowing to Events() meanwhile
// (they are demuxed on the same reader goroutine that resolves results, so
// neither stream blocks the other).
//
// cmd must not be shared across concurrent SendCommand calls: RequestId is
// mutated in place, so two goroutines calling SendCommand with the SAME cmd
// value would race on that assignment. Distinct *vttv1.ClientCommand values
// per call (even for logically-identical commands) is the safe pattern —
// see TestConcurrentSendCommandsCorrelateByRequestID for exactly that shape.
//
// Canceling ctx while the write is still in flight tears down the WHOLE
// connection, not just this call: coder/websocket's own doc comment on Conn
// says a context expiration is treated like any other error — "the
// connection is closed with an appropriate reason ... This applies to
// context expirations as well unfortunately" (see
// https://github.com/nhooyr/websocket/issues/242#issuecomment-633182220).
// So a canceled write's error propagates to every other in-flight
// SendCommand and to Events() the next time the reader goroutine's Read
// fails, and this Client is unusable afterward — teardown handles that
// uniformly, the same as a genuine network failure; there is no
// special-case recovery for "just this call's" cancellation.
func (c *Client) SendCommand(ctx context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	reqID := c.nextRequestID()
	cmd.RequestId = reqID

	ch := make(chan *vttv1.CommandResult, 1)

	c.mu.Lock()
	if c.closed {
		err := c.closeErr
		c.mu.Unlock()
		return nil, fmt.Errorf("harness: send command: %w", closeErrOrDefault(err))
	}
	c.pending[reqID] = ch
	c.mu.Unlock()

	raw, err := protojson.Marshal(cmd)
	if err != nil {
		c.dropPending(reqID)
		return nil, fmt.Errorf("harness: marshal command: %w", err)
	}

	if err := c.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		c.dropPending(reqID)
		return nil, fmt.Errorf("harness: write command: %w", err)
	}

	select {
	case res, ok := <-ch:
		if !ok {
			c.mu.Lock()
			err := c.closeErr
			c.mu.Unlock()
			return nil, fmt.Errorf("harness: connection closed waiting for result: %w", closeErrOrDefault(err))
		}
		return res, nil
	case <-ctx.Done():
		c.dropPending(reqID)
		return nil, ctx.Err()
	}
}

// Events returns the channel of broadcast Envelopes, in wire order. It is
// closed when the connection ends (server close, local Close, or a
// protocol/overflow error).
func (c *Client) Events() <-chan *vttv1.Envelope {
	return c.events
}

// Close ends the connection and releases the client's resources. It is safe
// to call more than once (coder/websocket's own Close is documented as a
// no-op after the first) and safe to call after the server has already
// closed the connection.
func (c *Client) Close() error {
	// Close's own return error (e.g. "already closed" on a connection the
	// server or readLoop already tore down) is not the caller's concern —
	// Close's job is "make sure this connection is gone and resources are
	// released", which happens either way.
	_ = c.conn.Close(websocket.StatusNormalClosure, "harness: client closing")
	<-c.readerDone // wait for readLoop's teardown to finish unblocking pending callers
	c.teardown(errClientClosed)
	return nil
}

// readLoop is the single demuxing goroutine: every inbound ServerFrame is
// decoded here and routed to either deliverResult (result → the caller
// blocked in SendCommand, matched by request_id) or deliverEvent (event →
// Events(), in arrival order). It owns the only conn.Read call for this
// connection's lifetime (required by coder/websocket: "All methods may be
// called concurrently except for Reader and Read.").
func (c *Client) readLoop() {
	defer close(c.readerDone)
	defer close(c.events)
	defer c.cancel()

	for {
		_, raw, err := c.conn.Read(c.readCtx)
		if err != nil {
			c.teardown(err)
			return
		}

		var frame vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &frame); err != nil {
			wrapped := fmt.Errorf("harness: malformed server frame: %w", err)
			c.teardown(wrapped)
			c.conn.Close(websocket.StatusUnsupportedData, "harness: malformed frame")
			return
		}

		switch f := frame.GetFrame().(type) {
		case *vttv1.ServerFrame_Result:
			c.deliverResult(f.Result)
		case *vttv1.ServerFrame_Event:
			if !c.deliverEvent(f.Event) {
				return // overflow: deliverEvent already tore the connection down
			}
		default:
			c.teardown(errors.New("harness: server frame has neither result nor event"))
			c.conn.Close(websocket.StatusUnsupportedData, "harness: empty frame")
			return
		}
	}
}

// deliverResult routes a CommandResult to its SendCommand caller by
// request_id. A result for an unknown/already-resolved request_id is
// dropped — it can only happen if the server double-replies or replies
// after this client already gave up waiting (ctx.Done()), neither of which
// is this client's fault to report.
func (c *Client) deliverResult(res *vttv1.CommandResult) {
	c.mu.Lock()
	ch, ok := c.pending[res.GetRequestId()]
	if ok {
		delete(c.pending, res.GetRequestId())
	}
	c.mu.Unlock()
	if ok {
		ch <- res
	}
}

// deliverEvent pushes env onto Events(). It returns false if the buffer was
// full (eventBuffer's overflow policy: tear the connection down with
// ErrEventsOverflow rather than block the reader goroutine — a blocked
// reader would also stall result delivery for every in-flight SendCommand).
func (c *Client) deliverEvent(env *vttv1.Envelope) bool {
	select {
	case c.events <- env:
		return true
	default:
		c.teardown(ErrEventsOverflow)
		c.conn.Close(websocket.StatusPolicyViolation, "harness: events buffer overflow")
		return false
	}
}

// teardown marks the client closed (idempotent: only the first call takes
// effect) and unblocks every pending SendCommand call with err.
func (c *Client) teardown(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.closeErr = err
	pending := c.pending
	c.pending = nil
	c.mu.Unlock()

	for _, ch := range pending {
		close(ch)
	}
}

// dropPending removes a registered-but-not-yet-answered request (write
// failed, or the caller's ctx was canceled first) so a late server reply
// for it is simply dropped by deliverResult instead of leaking the channel.
func (c *Client) dropPending(reqID string) {
	c.mu.Lock()
	if c.pending != nil {
		delete(c.pending, reqID)
	}
	c.mu.Unlock()
}

// nextRequestID returns a fresh, client-instance-unique request id.
// atomic.AddUint64 gives concurrent SendCommand callers distinct ids without
// a lock.
func (c *Client) nextRequestID() string {
	n := atomic.AddUint64(&c.reqSeq, 1)
	return "harness-" + strconv.FormatUint(n, 10)
}

// closeErrOrDefault reports errClientClosed when a teardown recorded no
// specific reason (should not happen in practice — teardown is always
// called with a concrete error — but keeps every error path nil-safe).
func closeErrOrDefault(err error) error {
	if err == nil {
		return errClientClosed
	}
	return err
}
