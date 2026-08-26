package gateway

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// FOUND IN SESSION ZERO (findings note, entry 7). Patrik's browser sat idle
// through the tunnel while play happened elsewhere, and came back reading
// `closed`. His requirement, verbatim: "you have to be able to be 'inactive'
// without being kicked out."
//
// Neither of our own deadlines did it, and both were checked rather than
// assumed. store.SubscriberNoProgressTimeout is armed INSIDE the select that
// hands an envelope over (internal/store/subscribe.go:145-163), so it only
// runs while an event is waiting; Server.writeTimeout bounds a write, and an
// idle connection performs none. Nothing pinged anywhere, so an idle
// connection carried ZERO BYTES in either direction and any intermediary —
// Cloudflare, carrier NAT, a home router's connection table — was free to
// reap it.
//
// These tests observe the fix AT THE SOCKET, which is the only place it is
// real: coder/websocket's DialOptions.OnPingReceived reports a ping frame as
// it arrives, and returning false from it suppresses the pong — a genuine
// fault injection for the half that detects a dead peer, rather than a stub.

// keepAliveFixture is one campaign, one identity DB, and a server whose ping
// budgets are small enough to observe. Everything else is production shape.
type keepAliveFixture struct {
	srv  *Server
	http *httptest.Server
	ids  *identity.DB
}

func newKeepAliveFixture(t *testing.T, interval, timeout time.Duration) *keepAliveFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")
	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	ids, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })

	srv := New(c, ids)
	srv.pingInterval = interval
	srv.pingTimeout = timeout
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &keepAliveFixture{srv: srv, http: httpSrv, ids: ids}
}

// TestAnIdleConnectionIsPinged is the keepalive half.
//
// The connection sends no command and the table broadcasts no event, so
// EVERY byte this test observes is one the server sent for no reason but to
// keep the socket from looking dead. That is the whole property: before this,
// an idle connection was indistinguishable from an abandoned one to every hop
// between a player and the server.
func TestAnIdleConnectionIsPinged(t *testing.T) {
	f := newKeepAliveFixture(t, 20*time.Millisecond, 5*time.Second)
	token, _, err := f.ids.CreateInvite("Idle", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pinged := make(chan struct{}, 1)
	conn, _, err := websocket.Dial(ctx, f.http.URL+"/ws?token="+token+"&after=0",
		&websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool {
				select {
				case pinged <- struct{}{}:
				default:
				}
				return true // pong normally; this test is only about the ping
			},
		})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// A reader must be running for control frames to be processed at all —
	// coder/websocket handles ping/pong inside Read, so an unread connection
	// would never invoke the callback no matter what the server sent. This
	// also matches what a real client does: it sits in a read loop.
	go func() {
		for {
			if _, _, rerr := conn.Read(ctx); rerr != nil {
				return
			}
		}
	}()

	select {
	case <-pinged:
	case <-time.After(5 * time.Second):
		t.Fatal("an idle connection was never pinged — it carries zero bytes indefinitely, " +
			"so every intermediary between a player and the server is free to reap it, and " +
			"the player discovers it by acting and having nothing happen")
	}
}

// TestAClientThatStopsPongingIsAnnouncedGone is the detection half, and it is
// the reason the ping is not fire-and-forget.
//
// This is what the prior art actually used a heartbeat for. Patrik, recalling
// MapTool: "to check if any connections were down on the player side."
//
// Without it a peer that has silently gone — laptop lid closed, phone off a
// dead cell, a NAT entry evicted — stays CONNECTED at the table forever,
// because departure hangs off serve() returning and serve() is parked in
// conn.Read on a socket nobody has told it is gone. The DM sees a full table
// and narrates to a ghost. The write deadline cannot cover this: it only fires
// when there is something to write, and on a quiet table there is not.
//
// THE INJECTION IS REAL. The victim receives the ping and refuses to answer
// it, which is precisely a half-open connection: bytes still flow toward it,
// nothing comes back. A stubbed-out "pretend the pong failed" would prove
// nothing about the wire.
func TestAClientThatStopsPongingIsAnnouncedGone(t *testing.T) {
	f := newKeepAliveFixture(t, 20*time.Millisecond, 100*time.Millisecond)
	dmToken, _, err := f.ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	muteToken, muteID, err := f.ids.CreateInvite("Mute", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	watcher, _, err := websocket.Dial(ctx, f.http.URL+"/ws?token="+dmToken+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial watcher: %v", err)
	}
	defer watcher.CloseNow()

	gone := make(chan struct{})
	go func() {
		for {
			_, raw, rerr := watcher.Read(ctx)
			if rerr != nil {
				return
			}
			var frame vttv1.ServerFrame
			if protojson.Unmarshal(raw, &frame) != nil {
				continue
			}
			pc := frame.GetPresenceChanged()
			if pc.GetParticipantId() == muteID &&
				pc.GetState() == vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED {
				close(gone)
				return
			}
		}
	}()

	mute, _, err := websocket.Dial(ctx, f.http.URL+"/ws?token="+muteToken+"&after=0",
		&websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool {
				return false // heard, deliberately unanswered
			},
		})
	if err != nil {
		t.Fatalf("dial mute: %v", err)
	}
	defer mute.CloseNow()
	go func() {
		for {
			if _, _, rerr := mute.Read(ctx); rerr != nil {
				return
			}
		}
	}()

	select {
	case <-gone:
	case <-time.After(10 * time.Second):
		t.Fatal("a client that stopped answering pings was never announced gone — it stays " +
			"listed at the table indistinguishable from someone genuinely there, and the DM " +
			"narrates to a ghost")
	}
}

// TestAClientThatKeepsAnsweringIsNotReaped is the direction that would be
// catastrophic to get wrong, and it is the one a keepalive tempts you to skip.
//
// The two tests above both push toward tearing connections down. Nothing in
// them would notice a failure path wired backwards, a pong deadline too tight
// for a phone on a slow cell, or a ping whose success is read as a failure —
// and every one of those reaps PLAYERS WHO ARE FINE. That is strictly worse
// than the bug being fixed: entry 7 cost one person their connection, this
// would cost everyone theirs, repeatedly, and the symptom would look identical.
//
// So: a client that answers every ping survives many intervals. Deliberately
// run with a pong deadline (100ms) far tighter than production's, and for
// enough intervals that a systematically wrong verdict cannot hide in a
// margin. Presence is watched rather than just the socket, because a spurious
// reap announces DISCONNECTED and that is what the table would actually see.
func TestAClientThatKeepsAnsweringIsNotReaped(t *testing.T) {
	const interval = 20 * time.Millisecond
	f := newKeepAliveFixture(t, interval, 100*time.Millisecond)
	dmToken, _, err := f.ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	politeToken, politeID, err := f.ids.CreateInvite("Polite", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	watcher, _, err := websocket.Dial(ctx, f.http.URL+"/ws?token="+dmToken+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial watcher: %v", err)
	}
	defer watcher.CloseNow()

	reaped := make(chan struct{})
	go func() {
		for {
			_, raw, rerr := watcher.Read(ctx)
			if rerr != nil {
				return
			}
			var frame vttv1.ServerFrame
			if protojson.Unmarshal(raw, &frame) != nil {
				continue
			}
			pc := frame.GetPresenceChanged()
			if pc.GetParticipantId() == politeID &&
				pc.GetState() == vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED {
				close(reaped)
				return
			}
		}
	}()

	pings := make(chan struct{}, 64)
	polite, _, err := websocket.Dial(ctx, f.http.URL+"/ws?token="+politeToken+"&after=0",
		&websocket.DialOptions{
			OnPingReceived: func(context.Context, []byte) bool {
				select {
				case pings <- struct{}{}:
				default:
				}
				return true
			},
		})
	if err != nil {
		t.Fatalf("dial polite: %v", err)
	}
	defer polite.CloseNow()

	readErr := make(chan error, 1)
	go func() {
		for {
			if _, _, rerr := polite.Read(ctx); rerr != nil {
				readErr <- rerr
				return
			}
		}
	}()

	// Long enough for many round trips, so "survived" is a result rather than
	// a race that has not been given time to lose.
	const rounds = 25
	deadline := time.After(rounds * interval * 8)
	seen := 0
	for seen < rounds {
		select {
		case <-pings:
			seen++
		case <-reaped:
			t.Fatalf("a client answering every ping was announced gone after %d pings — "+
				"the keepalive is reaping live players, which is worse than the disconnect "+
				"it was built to fix", seen)
		case rerr := <-readErr:
			t.Fatalf("a client answering every ping had its connection closed after %d "+
				"pings: %v", seen, rerr)
		case <-deadline:
			t.Fatalf("only %d of %d pings arrived — the ping is not running at its "+
				"interval", seen, rounds)
		}
	}
}
