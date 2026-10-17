package gateway_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// Bound only a wait for a frame that must come with qaPRWait.
// Never let it run out to assert that a frame did not come.
const qaPRWait = 10 * time.Second

const (
	qaPRHere = vttv1.PresenceState_PRESENCE_STATE_CONNECTED
	qaPRGone = vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED
)

const qaPRPromotionRule = "SPEC-009, Promotion is a command"

type qaPRWho struct {
	id, name, token string
	role            identity.Role
}

type qaPRTable struct {
	t    *testing.T
	ids  *identity.DB
	base string
	ctx  context.Context
	reqs int
}

func qaPRNewTable(t *testing.T) *qaPRTable {
	t.Helper()
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("open the campaign: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("open the identity store: %v", err)
	}
	routes := gateway.New(camp, ids).Handler()
	var serving sync.WaitGroup
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serving.Add(1)
		defer serving.Done()
		routes.ServeHTTP(w, r)
	}))
	ctx, cancel := context.WithCancel(context.Background())
	// Close the stores only after every connection has unwound (SPEC-011).
	t.Cleanup(func() {
		cancel()
		idle := make(chan struct{})
		go func() { serving.Wait(); close(idle) }()
		select {
		case <-idle:
		case <-time.After(qaPRWait):
			t.Errorf("a connection was still being served %v after every client hung up", qaPRWait)
		}
		srv.Close()
		if err := ids.Close(); err != nil {
			t.Errorf("close the identity store: %v", err)
		}
		if err := camp.Close(); err != nil {
			t.Errorf("close the campaign: %v", err)
		}
	})
	return &qaPRTable{t: t, ids: ids, base: srv.URL, ctx: ctx}
}

func (tb *qaPRTable) mint(name string, role identity.Role) qaPRWho {
	tb.t.Helper()
	token, id, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatalf("invite %s as %s: %v", name, role, err)
	}
	return qaPRWho{id: id, name: name, token: token, role: role}
}

// Keep the event host appends: every later connection waits for it as its
// backlog, and revoke refuses a connection that was sent none.
func (tb *qaPRTable) host() *qaPRConn {
	tb.t.Helper()
	dm := tb.connect(tb.mint("Dee", identity.RoleDM), "the DM")
	tb.narrate(dm, "the table is laid")
	return dm
}

// Keep the backlog check, and pass every open connection of who: a connection
// asks for its credential before its backlog (SPEC-011), so a revocation
// landing ahead of that first check closes it before the test has used it.
func (tb *qaPRTable) revoke(who qaPRWho, conns ...*qaPRConn) {
	tb.t.Helper()
	for _, c := range conns {
		if !c.caughtUp {
			tb.t.Fatalf("fixture: %s was sent no backlog; append an event before it connects", c.label)
		}
	}
	if err := tb.ids.Revoke(who.id); err != nil {
		tb.t.Fatalf("revoke %s: %v", who.name, err)
	}
}

// Keep both waits: for the snapshot that lists the joiner, and for the backlog
// its catch-up head names (SPEC-007).
func (tb *qaPRTable) connect(who qaPRWho, label string) *qaPRConn {
	tb.t.Helper()
	dialCtx, stop := context.WithTimeout(tb.ctx, qaPRWait)
	defer stop()
	ws, _, err := websocket.Dial(dialCtx, tb.base+"/ws?token="+url.QueryEscape(who.token), nil)
	if err != nil {
		tb.t.Fatalf("%s: dial: %v", label, err)
	}
	c := &qaPRConn{t: tb.t, label: label, ws: ws, ctx: tb.ctx, done: make(chan struct{}), tick: make(chan struct{})}
	go c.read()
	tb.t.Cleanup(c.hangUp)
	frames := c.await("snapshot listing "+who.name, func(frames []*vttv1.ServerFrame, _ bool) bool {
		return slices.ContainsFunc(frames, func(f *vttv1.ServerFrame) bool {
			return slices.ContainsFunc(f.GetPresenceSnapshot().GetPresent(), func(p *vttv1.PresenceChanged) bool {
				return p.GetParticipantId() == who.id
			})
		})
	})
	head := frames[0].GetCatchUpHead()
	if head == nil {
		tb.t.Fatalf("SPEC-007: %s: the connection opened with %s, want catch_up_head", label, qaPRDescribe(frames[:1]))
	}
	switch {
	case who.role == identity.RolePlayer || who.role == identity.RoleSpectator:
		c.await("its viewpoint", func(frames []*vttv1.ServerFrame, _ bool) bool {
			return slices.ContainsFunc(frames, func(f *vttv1.ServerFrame) bool { return f.GetViewpointChanged() != nil })
		})
		c.caughtUp = true
	case head.GetHeadSequence() > 0:
		c.await(fmt.Sprintf("backlog event %d", head.GetHeadSequence()), func(frames []*vttv1.ServerFrame, _ bool) bool {
			return slices.Contains(qaPREvents(frames), head.GetHeadSequence())
		})
		c.caughtUp = true
	}
	return c
}

func (tb *qaPRTable) request() string {
	tb.reqs++
	return fmt.Sprintf("qa-pr-%d", tb.reqs)
}

func (tb *qaPRTable) promote(by *qaPRConn, who qaPRWho, role identity.Role) *vttv1.CommandResult {
	tb.t.Helper()
	return by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_PromoteParticipant{
		PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: who.id, Role: string(role)},
	}})
}

func (tb *qaPRTable) mustPromote(by *qaPRConn, who qaPRWho, role identity.Role) {
	tb.t.Helper()
	if res := tb.promote(by, who, role); !res.GetOk() {
		tb.t.Fatalf("%s: promoting %s to %s was refused: %q", by.label, who.name, role, res.GetError())
	}
}

func (tb *qaPRTable) narrate(by *qaPRConn, text string) *vttv1.CommandResult {
	tb.t.Helper()
	res := by.ask(&vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: text},
	}})
	if !res.GetOk() {
		tb.t.Fatalf("%s: narrating %q was refused: %q", by.label, text, res.GetError())
	}
	return res
}

func (tb *qaPRTable) viewpoint() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: tb.request(), Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{},
	}}
}

// Call fence after the announcement under test, and read a revoked connection
// to its close only after it: fan-outs are serialised (SPEC-011), so whatever
// that announcement queued for any connection is queued once a watcher holds
// the newcomer's arrival.
// Keep the command it asks first one that neither appends nor announces.
func (tb *qaPRTable) fence(by *qaPRConn, watchers ...*qaPRConn) qaPRWho {
	tb.t.Helper()
	by.ask(tb.viewpoint())
	who := tb.mint(fmt.Sprintf("Fen %d", tb.reqs), identity.RoleSpectator)
	tb.connect(who, who.name)
	for _, w := range watchers {
		w.awaitTold(who, qaPRHere, 1)
	}
	return who
}

// Count what a revoked connection was sent from what refuse or end returns: the
// stream read to its close, never a stream still open.
func (tb *qaPRTable) refuse(c *qaPRConn) []*vttv1.ServerFrame {
	tb.t.Helper()
	if err := c.send(tb.viewpoint()); err != nil {
		tb.t.Fatalf("%s: the connection could not take a command: %v", c.label, err)
	}
	frames, err := c.end()
	if got := websocket.CloseStatus(err); got != websocket.StatusPolicyViolation {
		tb.t.Fatalf("VTT-032: %s: the next command ended the connection with %v (%v), want a policy-violation close", c.label, got, err)
	}
	return frames
}

type qaPRConn struct {
	t     *testing.T
	label string
	ws    *websocket.Conn
	ctx   context.Context
	done  chan struct{}

	caughtUp bool

	mu    sync.Mutex
	seen  []*vttv1.ServerFrame
	ended bool
	err   error
	tick  chan struct{}
}

// Keep every frame, whatever its kind: a later assertion counts them (SPEC-007).
// Never give Read a context that expires: the library closes the connection
// when it does.
func (c *qaPRConn) read() {
	defer close(c.done)
	for {
		_, data, err := c.ws.Read(c.ctx)
		frame := &vttv1.ServerFrame{}
		if err == nil {
			if err = protojson.Unmarshal(data, frame); err != nil {
				err = fmt.Errorf("frame %q does not decode: %w", data, err)
			}
		}
		c.mu.Lock()
		if err != nil {
			c.ended, c.err = true, err
		} else {
			c.seen = append(c.seen, frame)
		}
		close(c.tick)
		c.tick = make(chan struct{})
		c.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (c *qaPRConn) hangUp() {
	_ = c.ws.CloseNow()
	<-c.done
}

func (c *qaPRConn) await(what string, ok func(frames []*vttv1.ServerFrame, ended bool) bool) []*vttv1.ServerFrame {
	c.t.Helper()
	limit := time.After(qaPRWait)
	for {
		c.mu.Lock()
		frames, ended, err, tick := slices.Clip(c.seen), c.ended, c.err, c.tick
		c.mu.Unlock()
		if ok(frames, ended) {
			return frames
		}
		if ended {
			c.t.Fatalf("%s: the connection ended (%v) with no %s; it was sent %s", c.label, err, what, qaPRDescribe(frames))
		}
		select {
		case <-tick:
		case <-limit:
			c.t.Fatalf("%s: no %s within %v; it was sent %s", c.label, what, qaPRWait, qaPRDescribe(frames))
		}
	}
}

func (c *qaPRConn) awaitTold(who qaPRWho, state vttv1.PresenceState, n int) []*vttv1.ServerFrame {
	c.t.Helper()
	return c.await(fmt.Sprintf("frame %d giving %s as %s", n, who.name, state), func(frames []*vttv1.ServerFrame, _ bool) bool {
		count := 0
		for _, s := range qaPRTold(frames, who) {
			if s == state {
				count++
			}
		}
		return count >= n
	})
}

func (c *qaPRConn) ahead(fence qaPRWho) []*vttv1.ServerFrame {
	c.t.Helper()
	frames := c.awaitTold(fence, qaPRHere, 1)
	return frames[:slices.IndexFunc(frames, func(f *vttv1.ServerFrame) bool {
		return f.GetPresenceChanged().GetParticipantId() == fence.id
	})]
}

func (c *qaPRConn) end() ([]*vttv1.ServerFrame, error) {
	c.t.Helper()
	frames := c.await("close", func(_ []*vttv1.ServerFrame, ended bool) bool { return ended })
	c.mu.Lock()
	defer c.mu.Unlock()
	return frames, c.err
}

func (c *qaPRConn) send(cmd *vttv1.ClientCommand) error {
	data, err := protojson.Marshal(cmd)
	if err != nil {
		return err
	}
	return c.ws.Write(c.ctx, websocket.MessageText, data)
}

// Match a result by its request id, never by its position (SPEC-007).
func (c *qaPRConn) ask(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	if err := c.send(cmd); err != nil {
		c.t.Fatalf("%s: send %s: %v", c.label, cmd.GetRequestId(), err)
	}
	var res *vttv1.CommandResult
	c.await("result for "+cmd.GetRequestId(), func(frames []*vttv1.ServerFrame, _ bool) bool {
		for _, f := range frames {
			if r := f.GetResult(); r != nil && r.GetRequestId() == cmd.GetRequestId() {
				res = r
			}
		}
		return res != nil
	})
	return res
}

func qaPRNaming(frames []*vttv1.ServerFrame, who qaPRWho) []*vttv1.PresenceChanged {
	var named []*vttv1.PresenceChanged
	for _, f := range frames {
		if pc := f.GetPresenceChanged(); pc != nil && pc.GetParticipantId() == who.id {
			named = append(named, pc)
		}
	}
	return named
}

func qaPRTold(frames []*vttv1.ServerFrame, who qaPRWho) []vttv1.PresenceState {
	var told []vttv1.PresenceState
	for _, pc := range qaPRNaming(frames, who) {
		told = append(told, pc.GetState())
	}
	return told
}

func qaPREvents(frames []*vttv1.ServerFrame) []int64 {
	var seqs []int64
	for _, f := range frames {
		if ev := f.GetEvent(); ev != nil {
			seqs = append(seqs, ev.GetSequence())
		}
	}
	return seqs
}

func qaPRDescribe(frames []*vttv1.ServerFrame) string {
	parts := make([]string, 0, len(frames))
	for _, f := range frames {
		parts = append(parts, protojson.MarshalOptions{}.Format(f))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func qaPRWantTold(t *testing.T, rule string, c *qaPRConn, frames []*vttv1.ServerFrame, who qaPRWho, want ...vttv1.PresenceState) {
	t.Helper()
	if got := qaPRTold(frames, who); !slices.Equal(got, want) {
		t.Errorf("%s: %s was sent %v naming %s, want %v", rule, c.label, got, who.name, want)
	}
}

// VTT-032
func TestQAPromoRevokedARevokedWatcherIsNotSentThePromotion(t *testing.T) {
	for _, role := range []identity.Role{identity.RoleSpectator, identity.RolePlayer, identity.RoleAgent, identity.RoleDM} {
		t.Run("revoked "+string(role), func(t *testing.T) {
			tb := qaPRNewTable(t)
			dm := tb.host()
			revoked := tb.mint("Rook", role)
			lost := tb.connect(revoked, "the revoked watcher")
			witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
			target := tb.mint("Tam", identity.RoleSpectator)
			own := tb.connect(target, "the promoted participant")
			for _, c := range []*qaPRConn{dm, lost, witness} {
				c.awaitTold(target, qaPRHere, 1)
			}

			tb.revoke(revoked, lost)
			tb.mustPromote(dm, target, identity.RolePlayer)
			fence := tb.fence(dm, witness, own)

			qaPRWantTold(t, qaPRPromotionRule, witness, witness.ahead(fence), target, qaPRHere, qaPRHere)
			qaPRWantTold(t, qaPRPromotionRule, dm, dm.ahead(fence), target, qaPRHere, qaPRHere)
			qaPRWantTold(t, "VTT-047", own, own.ahead(fence), target, qaPRHere)
			qaPRWantTold(t, "VTT-032", lost, tb.refuse(lost), target, qaPRHere)
		})
	}
}

// VTT-032
func TestQAPromoRevokedNoDeviceOfARevokedParticipantIsSentThePromotion(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	target := tb.mint("Tam", identity.RoleSpectator)
	tb.connect(target, "the promoted participant")
	// Keep this wait ahead of the connects below: they must find the target in
	// their snapshot and be sent no arrival that reads as a promotion.
	dm.awaitTold(target, qaPRHere, 1)
	witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
	revoked := tb.mint("Rook", identity.RolePlayer)
	first := tb.connect(revoked, "the revoked participant's first device")
	second := tb.connect(revoked, "the revoked participant's second device")

	tb.revoke(revoked, first, second)
	tb.mustPromote(dm, target, identity.RolePlayer)
	fence := tb.fence(dm, witness)

	qaPRWantTold(t, qaPRPromotionRule, witness, witness.ahead(fence), target, qaPRHere)
	qaPRWantTold(t, "VTT-032", first, tb.refuse(first), target)
	qaPRWantTold(t, "VTT-032", second, tb.refuse(second), target)
}

// VTT-032
func TestQAPromoRevokedARevocationBetweenTwoPromotionsWithholdsOnlyTheSecond(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	revoked := tb.mint("Rook", identity.RoleSpectator)
	lost := tb.connect(revoked, "the watcher revoked between the promotions")
	witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
	target := tb.mint("Tam", identity.RoleSpectator)
	own := tb.connect(target, "the promoted participant")
	for _, c := range []*qaPRConn{dm, lost, witness} {
		c.awaitTold(target, qaPRHere, 1)
	}

	tb.mustPromote(dm, target, identity.RolePlayer)
	// Keep this wait ahead of the revocation: the watcher is owed the first
	// promotion, and nothing orders a result against the frame it caused.
	lost.awaitTold(target, qaPRHere, 2)

	tb.revoke(revoked, lost)
	tb.mustPromote(dm, target, identity.RoleSpectator)
	fence := tb.fence(dm, witness, own)

	qaPRWantTold(t, qaPRPromotionRule, witness, witness.ahead(fence), target, qaPRHere, qaPRHere, qaPRHere)
	qaPRWantTold(t, "VTT-047", own, own.ahead(fence), target, qaPRHere, qaPRHere)
	qaPRWantTold(t, "VTT-032", lost, tb.refuse(lost), target, qaPRHere, qaPRHere)
}

// VTT-032
func TestQAPromoRevokedPromotingARevokedParticipantSendsThemNothing(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
	target := tb.mint("Tam", identity.RoleSpectator)
	own := tb.connect(target, "the revoked participant being promoted")
	dm.awaitTold(target, qaPRHere, 1)
	witness.awaitTold(target, qaPRHere, 1)

	tb.revoke(target, own)
	// Assert nothing about this result or about what the witness is sent: no
	// record says whether a revoked participant can be promoted.
	tb.promote(dm, target, identity.RolePlayer)
	tb.fence(dm, witness)

	qaPRWantTold(t, "VTT-032", own, tb.refuse(own), target)
}

// VTT-032 VTT-352
func TestQAPromoRevokedTheNextEventClosesARevokedWatcherWhoWasSentNoPromotion(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	revoked := tb.mint("Rook", identity.RoleSpectator)
	lost := tb.connect(revoked, "the revoked watcher")
	witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
	target := tb.mint("Tam", identity.RoleSpectator)
	tb.connect(target, "the promoted participant")
	for _, c := range []*qaPRConn{dm, lost, witness} {
		c.awaitTold(target, qaPRHere, 1)
	}

	tb.revoke(revoked, lost)
	tb.mustPromote(dm, target, identity.RolePlayer)
	fence := tb.fence(dm, witness)
	qaPRWantTold(t, qaPRPromotionRule, witness, witness.ahead(fence), target, qaPRHere, qaPRHere)
	narrated := tb.narrate(dm, "the lamps gutter")

	frames, err := lost.end()
	var closed websocket.CloseError
	if !errors.As(err, &closed) {
		t.Fatalf("VTT-032: the revoked watcher's connection ended with %v, want a close frame", err)
	}
	if closed.Code != websocket.StatusPolicyViolation || closed.Reason != "gateway: credential no longer valid" {
		t.Errorf("VTT-032: the next event closed the revoked watcher with %v %q, want %v %q",
			closed.Code, closed.Reason, websocket.StatusPolicyViolation, "gateway: credential no longer valid")
	}
	if slices.Contains(qaPREvents(frames), narrated.GetSequence()) {
		t.Errorf("VTT-032: the revoked watcher was delivered event %d", narrated.GetSequence())
	}
	qaPRWantTold(t, "VTT-032", lost, frames, target, qaPRHere)
}

// VTT-032
func TestQAPromoRevokedARevokedWatcherIsToldOfNoArrivalAndNoDeparture(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	revoked := tb.mint("Rook", identity.RolePlayer)
	lost := tb.connect(revoked, "the revoked watcher")
	witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")

	tb.revoke(revoked, lost)
	visitor := tb.mint("Vee", identity.RoleSpectator)
	visiting := tb.connect(visitor, "the visitor")
	witness.awaitTold(visitor, qaPRHere, 1)
	visiting.hangUp()
	witness.awaitTold(visitor, qaPRGone, 1)
	fence := tb.fence(dm, witness)

	qaPRWantTold(t, "SPEC-011, Presence", witness, witness.ahead(fence), visitor, qaPRHere, qaPRGone)
	qaPRWantTold(t, "VTT-032", lost, tb.refuse(lost), visitor)
}

// VTT-047
func TestQAPromoRevokedThePromotedParticipantIsToldOnTheirOwnConnection(t *testing.T) {
	tb := qaPRNewTable(t)
	dm := tb.host()
	revoked := tb.mint("Rook", identity.RolePlayer)
	lost := tb.connect(revoked, "the revoked watcher")
	target := tb.mint("Tam", identity.RoleSpectator)
	own := tb.connect(target, "the promoted participant")
	dm.awaitTold(target, qaPRHere, 1)

	tb.revoke(revoked, lost)
	tb.mustPromote(dm, target, identity.RolePlayer)
	fence := tb.fence(dm, own)

	named := qaPRNaming(own.ahead(fence), target)
	want := &vttv1.PresenceChanged{ParticipantId: target.id, DisplayName: target.name, State: qaPRHere}
	if len(named) != 1 || !proto.Equal(named[0], want) {
		t.Errorf("VTT-047: the promoted participant's own connection was sent %v naming them, want exactly %v", named, want)
	}
}

// VTT-309
func TestQAPromoRevokedEveryUnrevokedConnectionIsSentThePromotion(t *testing.T) {
	for _, by := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		t.Run("promoted by "+string(by), func(t *testing.T) {
			other := identity.RoleAgent
			if by == identity.RoleAgent {
				other = identity.RoleDM
			}
			tb := qaPRNewTable(t)
			promoter := tb.connect(tb.mint("Dee", by), "the promoter")
			tb.narrate(promoter, "the table is laid")
			player := tb.mint("Wren", identity.RolePlayer)
			before := []*qaPRConn{
				promoter,
				tb.connect(tb.mint("Bo", other), "the other "+string(other)),
				tb.connect(player, "a player's first device"),
				tb.connect(player, "a player's second device"),
				tb.connect(tb.mint("Sy", identity.RoleSpectator), "a spectator"),
			}
			revoked := tb.mint("Rook", identity.RolePlayer)
			lost := tb.connect(revoked, "the revoked watcher")
			target := tb.mint("Tam", identity.RoleSpectator)
			first := tb.connect(target, "the promoted participant's first device")
			for _, c := range append(slices.Clone(before), lost) {
				c.awaitTold(target, qaPRHere, 1)
			}
			// Keep the second device behind those waits: connected sooner, it can
			// be sent the arrival its own participant caused (SPEC-011), and that
			// frame reads as a promotion.
			own := []*qaPRConn{first, tb.connect(target, "the promoted participant's second device")}

			tb.revoke(revoked, lost)
			tb.mustPromote(promoter, target, identity.RolePlayer)
			fence := tb.fence(promoter, append(slices.Clone(before), own...)...)

			want := &vttv1.PresenceChanged{ParticipantId: target.id, DisplayName: target.name, State: qaPRHere}
			for _, c := range before {
				named := qaPRNaming(c.ahead(fence), target)
				if len(named) != 2 || !proto.Equal(named[1], want) {
					t.Errorf("%s: %s was sent %v naming the promoted participant, want their arrival and then %v", qaPRPromotionRule, c.label, named, want)
				}
			}
			for _, c := range own {
				named := qaPRNaming(c.ahead(fence), target)
				if len(named) != 1 || !proto.Equal(named[0], want) {
					t.Errorf("%s: %s was sent %v naming them, want exactly %v", qaPRPromotionRule, c.label, named, want)
				}
			}
			qaPRWantTold(t, "VTT-032", lost, tb.refuse(lost), target, qaPRHere)
		})
	}
}

// VTT-310
func TestQAPromoRevokedPromotingAnAbsentParticipantAnnouncesNothing(t *testing.T) {
	t.Run("never connected", func(t *testing.T) {
		tb := qaPRNewTable(t)
		dm := tb.host()
		witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
		target := tb.mint("Tam", identity.RoleSpectator)

		tb.mustPromote(dm, target, identity.RolePlayer)
		fence := tb.fence(dm, dm, witness)

		qaPRWantTold(t, qaPRPromotionRule, dm, dm.ahead(fence), target)
		qaPRWantTold(t, qaPRPromotionRule, witness, witness.ahead(fence), target)
	})
	t.Run("connected and left", func(t *testing.T) {
		tb := qaPRNewTable(t)
		dm := tb.host()
		witness := tb.connect(tb.mint("Wren", identity.RolePlayer), "the witness")
		target := tb.mint("Tam", identity.RoleSpectator)
		tb.connect(target, "the participant who leaves").hangUp()
		dm.awaitTold(target, qaPRGone, 1)
		witness.awaitTold(target, qaPRGone, 1)

		tb.mustPromote(dm, target, identity.RolePlayer)
		fence := tb.fence(dm, dm, witness)

		qaPRWantTold(t, qaPRPromotionRule, dm, dm.ahead(fence), target, qaPRHere, qaPRGone)
		qaPRWantTold(t, qaPRPromotionRule, witness, witness.ahead(fence), target, qaPRHere, qaPRGone)
	})
}

// VTT-036
func TestQAPromoRevokedAPromotionAppendsNoEvent(t *testing.T) {
	tb := qaPRNewTable(t)
	host := tb.mint("Dee", identity.RoleDM)
	dm := tb.connect(host, "the DM")
	target := tb.mint("Tam", identity.RoleSpectator)
	tb.connect(target, "the promoted participant")
	dm.awaitTold(target, qaPRHere, 1)

	first := tb.narrate(dm, "before the promotion")
	tb.mustPromote(dm, target, identity.RolePlayer)
	second := tb.narrate(dm, "after the promotion")
	want := []int64{first.GetSequence(), second.GetSequence()}

	arrived := func(frames []*vttv1.ServerFrame, _ bool) bool {
		return slices.Contains(qaPREvents(frames), second.GetSequence())
	}
	if got := qaPREvents(dm.await("event of the second narration", arrived)); !slices.Equal(got, want) {
		t.Errorf("VTT-036: the DM was delivered events %v across a promotion, want the two narrations %v", got, want)
	}
	if got := qaPREvents(tb.connect(host, "the DM's second device").await("backlog to its head", arrived)); !slices.Equal(got, want) {
		t.Errorf("VTT-036: the log replays as %v after a promotion, want the two narrations %v", got, want)
	}
}
