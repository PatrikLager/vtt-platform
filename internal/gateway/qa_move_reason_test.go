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
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaMRScene    = "scn-hall"
	qaMRDM       = "p-dm"
	qaMRAgent    = "p-agent"
	qaMRPlayer   = "p-player"
	qaMRSpec     = "p-spec"
	qaMRSpecBare = "p-spec-bare"
	qaMRIdle     = "p-idle"
	qaMRShoulder = "act-ash"
	qaMRNearAct  = "act-birch"
	qaMRFarAct   = "act-rook"
	qaMROwnTok   = "tok-ash"
	qaMRNearTok  = "tok-birch"
	qaMRFarTok   = "tok-rook"
	qaMRWallX    = 6
)

func qaMRPos(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }

func qaMRSceneCreated() *vttv1.SceneCreated {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 12; x++ {
		for y := int32(0); y < 5; y++ {
			kind := "floor"
			if x == qaMRWallX {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return &vttv1.SceneCreated{SceneId: qaMRScene, Name: "Hall", GridWidth: 12, GridHeight: 5, Tiles: tiles}
}

func qaMRActor(id, name string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: name, Kind: kind},
	}}}
}

func qaMRPlace(tok, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: qaMRScene, ActorId: actor, Position: qaMRPos(x, y),
	}}}
}

func qaMRSetupEvents(player string) []*vttv1.Envelope {
	return []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: qaMRSceneCreated()}},
		qaMRActor(qaMRShoulder, "Ash", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: qaMRShoulder, ParticipantId: player, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
		qaMRActor(qaMRNearAct, "Birch", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qaMRActor(qaMRFarAct, "Rook", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qaMRPlace(qaMROwnTok, qaMRShoulder, 1, 2),
		qaMRPlace(qaMRNearTok, qaMRNearAct, 3, 2),
		qaMRPlace(qaMRFarTok, qaMRFarAct, 9, 2),
	}
}

func qaMRMoveCommand(req, tok string, to *vttv1.GridPosition, reason *string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: tok, To: to, Reason: reason,
	}}}
}

// VTT-260
func TestQAMoveReasonToEventCarriesTheMovesReason(t *testing.T) {
	const reason = "the floor gives way by the pool"
	for _, p := range []*identity.Participant{
		{ID: qaMRDM, Name: "Dee", Role: identity.RoleDM},
		{ID: qaMRAgent, Name: "Ayo", Role: identity.RoleAgent},
		{ID: qaMRPlayer, Name: "Pim", Role: identity.RolePlayer},
	} {
		t.Run(string(p.Role), func(t *testing.T) {
			env, err := gateway.ToEvent(qaMRMoveCommand("r-1", qaMRNearTok, qaMRPos(5, 3), proto.String(reason)), p)
			if err != nil {
				t.Fatalf("ToEvent: %v", err)
			}
			tm := env.GetTokenMoved()
			if tm == nil {
				t.Fatalf("ToEvent(move_token) payload = %T, want TokenMoved", env.GetPayload())
			}
			if tm.GetTokenId() != qaMRNearTok {
				t.Errorf("TokenMoved.token_id = %q, want %q", tm.GetTokenId(), qaMRNearTok)
			}
			if !proto.Equal(tm.GetTo(), qaMRPos(5, 3)) {
				t.Errorf("TokenMoved.to = %v, want (5,3)", tm.GetTo())
			}
			if tm.GetReason() != reason {
				t.Errorf("TokenMoved.reason = %q, want %q", tm.GetReason(), reason)
			}
			if env.GetParticipantId() != p.ID || env.GetActorRole() != string(p.Role) {
				t.Errorf("envelope stamped (%q, %q), want (%q, %q)", env.GetParticipantId(), env.GetActorRole(), p.ID, p.Role)
			}
		})
	}
}

// VTT-260
func TestQAMoveReasonToEventCarriesAGrantsKind(t *testing.T) {
	dm := &identity.Participant{ID: qaMRDM, Name: "Dee", Role: identity.RoleDM}
	for _, kind := range []vttv1.ActorKind{vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY} {
		t.Run(kind.String(), func(t *testing.T) {
			cmd := &vttv1.ClientCommand{RequestId: "r-g", Command: &vttv1.ClientCommand_GrantActorControl{
				GrantActorControl: &vttv1.GrantActorControl{ActorId: qaMRShoulder, ParticipantId: qaMRPlayer, Kind: kind},
			}}
			env, err := gateway.ToEvent(cmd, dm)
			if err != nil {
				t.Fatalf("ToEvent: %v", err)
			}
			want := &vttv1.ActorControlGranted{ActorId: qaMRShoulder, ParticipantId: qaMRPlayer, Kind: kind}
			if got := env.GetActorControlGranted(); !proto.Equal(got, want) {
				t.Errorf("ActorControlGranted = %v, want %v", got, want)
			}
		})
	}
}

// VTT-261
func TestQAMoveReasonToEventWithNoReasonRecordsNone(t *testing.T) {
	p := &identity.Participant{ID: qaMRAgent, Name: "Ayo", Role: identity.RoleAgent}
	for name, reason := range map[string]*string{"absent": nil, "empty": proto.String("")} {
		t.Run(name, func(t *testing.T) {
			env, err := gateway.ToEvent(qaMRMoveCommand("r-2", qaMRNearTok, qaMRPos(4, 1), reason), p)
			if err != nil {
				t.Fatalf("ToEvent: %v", err)
			}
			tm := env.GetTokenMoved()
			if tm == nil {
				t.Fatalf("ToEvent(move_token) payload = %T, want TokenMoved", env.GetPayload())
			}
			if tm.GetReason() != "" {
				t.Errorf("TokenMoved.reason = %q, want none", tm.GetReason())
			}
			if tm.GetTokenId() != qaMRNearTok || !proto.Equal(tm.GetTo(), qaMRPos(4, 1)) {
				t.Errorf("TokenMoved = %v, want token %s to (4,1)", tm, qaMRNearTok)
			}
		})
	}
}

type qaMRWire struct {
	raw   []byte
	frame *vttv1.ServerFrame
}

type qaMRConn struct {
	name     string
	conn     *websocket.Conn
	in       chan qaMRWire
	seen     []qaMRWire
	lookedAt int
}

func qaMRDial(t *testing.T, ctx context.Context, base, name, token string, after int64) *qaMRConn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(base, "http") + "/ws?token=" + url.QueryEscape(token)
	if after > 0 {
		u += fmt.Sprintf("&after=%d", after)
	}
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", name, err)
	}
	conn.SetReadLimit(1 << 22)
	qc := &qaMRConn{name: name, conn: conn, in: make(chan qaMRWire, 4096)}
	go func() {
		defer close(qc.in)
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if protojson.Unmarshal(raw, f) != nil {
				f = nil
			}
			qc.in <- qaMRWire{raw: raw, frame: f}
		}
	}()
	t.Cleanup(func() { _ = conn.CloseNow() })
	return qc
}

func (qc *qaMRConn) read(t *testing.T, ctx context.Context, what string) *vttv1.ServerFrame {
	t.Helper()
	select {
	case w, ok := <-qc.in:
		if !ok {
			t.Fatalf("%s: connection closed while waiting for %s", qc.name, what)
		}
		qc.seen = append(qc.seen, w)
		if w.frame == nil {
			t.Fatalf("%s: undecodable frame %s", qc.name, w.raw)
		}
		return w.frame
	case <-ctx.Done():
		t.Fatalf("%s: timed out waiting for %s", qc.name, what)
	}
	return nil
}

func (qc *qaMRConn) until(t *testing.T, ctx context.Context, what string, match func(*vttv1.ServerFrame) bool) *vttv1.ServerFrame {
	t.Helper()
	for {
		if qc.lookedAt == len(qc.seen) {
			qc.read(t, ctx, what)
		}
		f := qc.seen[qc.lookedAt].frame
		qc.lookedAt++
		if match(f) {
			return f
		}
	}
}

func (qc *qaMRConn) command(t *testing.T, ctx context.Context, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	t.Helper()
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	if err := qc.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("%s: write: %v", qc.name, err)
	}
	for {
		f := qc.read(t, ctx, "result "+cmd.GetRequestId())
		if f.GetResult().GetRequestId() == cmd.GetRequestId() {
			return f.GetResult()
		}
	}
}

func (qc *qaMRConn) eventsAt(seq int64) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, w := range qc.seen {
		if e := w.frame.GetEvent(); e != nil && e.GetSequence() == seq {
			out = append(out, e)
		}
	}
	return out
}

// caused finds the entries one world event caused by its id: a log numbers its
// entries itself (VTT-321, VTT-371).
func (qc *qaMRConn) caused(logged *vttv1.Envelope) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, w := range qc.seen {
		if e := w.frame.GetEvent(); e != nil && e.GetEventId() == logged.GetEventId() {
			out = append(out, e)
		}
	}
	return out
}

func (qc *qaMRConn) events() []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, w := range qc.seen {
		if e := w.frame.GetEvent(); e != nil {
			out = append(out, e)
		}
	}
	return out
}

func (qc *qaMRConn) rawContains(text string) bool {
	for _, w := range qc.seen {
		if strings.Contains(string(w.raw), text) {
			return true
		}
	}
	return false
}

type qaMRLive struct {
	ctx                          context.Context
	c                            *campaign.Campaign
	base                         string
	tokens                       map[identity.Role]string
	dmID, agentID, playerID      string
	dm, agent, player, spectator *qaMRConn
	conns                        []*qaMRConn
	barriers                     int
}

func qaMRStartLive(t *testing.T) *qaMRLive {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	tokens := map[identity.Role]string{}
	l := &qaMRLive{ctx: ctx, c: c, tokens: tokens}
	for _, inv := range []struct {
		name string
		role identity.Role
		id   *string
	}{
		{"Dee", identity.RoleDM, &l.dmID},
		{"Ayo", identity.RoleAgent, &l.agentID},
		{"Pim", identity.RolePlayer, &l.playerID},
		{"Sol", identity.RoleSpectator, new(string)},
	} {
		tok, id, err := db.CreateInvite(inv.name, inv.role)
		if err != nil {
			t.Fatalf("CreateInvite: %v", err)
		}
		tokens[inv.role], *inv.id = tok, id
	}
	for i, env := range qaMRSetupEvents(l.playerID) {
		env.EventId = fmt.Sprintf("evt-seed-%d", i+1)
		env.OccurredAt = timestamppb.Now()
		env.ActorRole = string(identity.RoleDM)
		env.ParticipantId = l.dmID
		if _, err := c.Append(env); err != nil {
			t.Fatalf("seed %d: campaign.Append: %v", i+1, err)
		}
	}
	srv := httptest.NewServer(gateway.New(c, db).Handler())
	t.Cleanup(srv.Close)
	l.base = srv.URL
	l.dm = qaMRDial(t, ctx, srv.URL, "dm", tokens[identity.RoleDM], 0)
	l.agent = qaMRDial(t, ctx, srv.URL, "agent", tokens[identity.RoleAgent], 0)
	l.player = qaMRDial(t, ctx, srv.URL, "player", tokens[identity.RolePlayer], 0)
	l.spectator = qaMRDial(t, ctx, srv.URL, "spectator", tokens[identity.RoleSpectator], 0)
	l.conns = []*qaMRConn{l.dm, l.agent, l.player, l.spectator}
	for _, qc := range l.conns {
		qc.until(t, ctx, "catch-up head", func(f *vttv1.ServerFrame) bool { return f.GetCatchUpHead() != nil })
	}
	placed := func(tok string) func(*vttv1.ServerFrame) bool {
		return func(f *vttv1.ServerFrame) bool { return f.GetEvent().GetTokenPlaced().GetTokenId() == tok }
	}
	l.player.until(t, ctx, "the near token placed", placed(qaMRNearTok))
	res := l.spectator.command(t, ctx, &vttv1.ClientCommand{RequestId: "perch", Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: qaMRShoulder},
	}})
	if !res.GetOk() {
		t.Fatalf("setup: set_viewpoint refused: %s", res.GetError())
	}
	l.spectator.until(t, ctx, "the near token placed after the perch", placed(qaMRNearTok))
	return l
}

func (l *qaMRLive) barrier(t *testing.T) int64 {
	t.Helper()
	l.barriers++
	text := fmt.Sprintf("barrier %d", l.barriers)
	res := l.dm.command(t, l.ctx, &vttv1.ClientCommand{RequestId: text, Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: text},
	}})
	if !res.GetOk() {
		t.Fatalf("barrier narration refused: %s", res.GetError())
	}
	for _, qc := range l.conns {
		qc.until(t, l.ctx, text, func(f *vttv1.ServerFrame) bool { return f.GetEvent().GetNarrationAdded().GetText() == text })
	}
	return res.GetSequence()
}

func (l *qaMRLive) logThrough(t *testing.T, seq int64) []*vttv1.Envelope {
	t.Helper()
	events, unsubscribe, _, err := l.c.Subscribe(0, 256)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsubscribe()
	var out []*vttv1.Envelope
	for {
		select {
		case env, ok := <-events:
			if !ok {
				t.Fatalf("log subscription closed before seq %d", seq)
			}
			out = append(out, env)
			if env.GetSequence() >= seq {
				return out
			}
		case <-l.ctx.Done():
			t.Fatalf("timed out reading the log through seq %d", seq)
		}
	}
}

func qaMRLogged(t *testing.T, log []*vttv1.Envelope, seq int64) *vttv1.Envelope {
	t.Helper()
	for _, env := range log {
		if env.GetSequence() == seq {
			return env
		}
	}
	t.Fatalf("the log holds no seq %d", seq)
	return nil
}

func (l *qaMRLive) move(t *testing.T, issuer *qaMRConn, req, tok string, to *vttv1.GridPosition, reason *string) int64 {
	t.Helper()
	res := issuer.command(t, l.ctx, qaMRMoveCommand(req, tok, to, reason))
	if !res.GetOk() {
		t.Fatalf("%s's move_token refused: %s", issuer.name, res.GetError())
	}
	return res.GetSequence()
}

func qaMRRequireSentWithoutReason(t *testing.T, qc *qaMRConn, seq int64, logged *vttv1.Envelope, reason string) {
	t.Helper()
	got := qc.caused(logged)
	if len(got) != 1 {
		t.Fatalf("%s is sent %d frames for the move at seq %d, want exactly the move", qc.name, len(got), seq)
	}
	want := forwardedOf(logged)
	want.Sequence = got[0].GetSequence()
	if !proto.Equal(got[0], want) {
		t.Errorf("%s is sent\n%v\nwant the logged move with its reason and issuer cleared\n%v", qc.name, got[0], want)
	}
	if qc.rawContains(reason) {
		t.Errorf("%s's wire carries the reason %q", qc.name, reason)
	}
}

// VTT-176
func TestQAMoveReasonOverTheWireDMAndAgentHearAnAgentsReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	head := l.barrier(t)
	log := l.logThrough(t, head)
	logged := qaMRLogged(t, log, seq)
	want := &vttv1.TokenMoved{TokenId: qaMRNearTok, SceneId: qaMRScene, From: qaMRPos(3, 2), To: qaMRPos(4, 3), Reason: reason}
	if !proto.Equal(logged.GetTokenMoved(), want) {
		t.Errorf("the log holds %v, want %v", logged.GetTokenMoved(), want)
	}
	if logged.GetParticipantId() != l.agentID || logged.GetActorRole() != string(identity.RoleAgent) {
		t.Errorf("the logged move is stamped (%q, %q), want (%q, agent)", logged.GetParticipantId(), logged.GetActorRole(), l.agentID)
	}
	for _, qc := range []*qaMRConn{l.dm, l.agent} {
		got := qc.events()
		if len(got) != len(log) {
			t.Fatalf("%s is sent %d events through seq %d, the log holds %d", qc.name, len(got), head, len(log))
		}
		for i := range got {
			if !proto.Equal(got[i], log[i]) {
				t.Errorf("%s event %d is\n%v\nwant the log's\n%v", qc.name, i, got[i], log[i])
			}
		}
	}
}

// VTT-262
func TestQAMoveReasonOverTheWirePlayerAndSpectatorDoNotHearAnAgentsReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	for _, qc := range []*qaMRConn{l.player, l.spectator} {
		qaMRRequireSentWithoutReason(t, qc, seq, logged, reason)
	}
}

// VTT-262
func TestQAMoveReasonOverTheWireAPlayerIsNotSentBackItsOwnReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "it steps around the pool"
	seq := l.move(t, l.player, "mv-player", qaMROwnTok, qaMRPos(2, 2), proto.String(reason))
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	if got := logged.GetTokenMoved().GetReason(); got != reason {
		t.Errorf("the log holds reason %q for the player's move, want %q", got, reason)
	}
	for _, qc := range []*qaMRConn{l.player, l.spectator} {
		qaMRRequireSentWithoutReason(t, qc, seq, logged, reason)
	}
	for _, qc := range []*qaMRConn{l.dm, l.agent} {
		if got := qc.eventsAt(seq); len(got) != 1 || !proto.Equal(got[0], logged) {
			t.Errorf("%s is sent %v at seq %d, want the logged move %v", qc.name, got, seq, logged)
		}
	}
}

// VTT-261
func TestQAMoveReasonOverTheWireAMoveWithNoReasonAppendsNone(t *testing.T) {
	l := qaMRStartLive(t)
	seq := l.move(t, l.dm, "mv-plain", qaMRNearTok, qaMRPos(3, 1), nil)
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	want := &vttv1.TokenMoved{TokenId: qaMRNearTok, SceneId: qaMRScene, From: qaMRPos(3, 2), To: qaMRPos(3, 1)}
	if !proto.Equal(logged.GetTokenMoved(), want) {
		t.Errorf("the log holds %v, want %v with no reason", logged.GetTokenMoved(), want)
	}
	for _, qc := range l.conns {
		want, got := logged, qc.eventsAt(seq)
		if qc == l.player || qc == l.spectator {
			want, got = forwardedOf(logged), qc.caused(logged)
			if len(got) == 1 {
				want.Sequence = got[0].GetSequence()
			}
		}
		if len(got) != 1 || !proto.Equal(got[0], want) {
			t.Errorf("%s is sent %v at seq %d, want %v", qc.name, got, seq, want)
		}
	}
}

// VTT-206
func TestQAMoveReasonOverTheWireAHiddenMoveReachesNoPlayerOrSpectator(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "it slides toward the hidden pool"
	seq := l.move(t, l.dm, "mv-hidden", qaMRFarTok, qaMRPos(10, 3), proto.String(reason))
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	for _, qc := range []*qaMRConn{l.player, l.spectator} {
		if got := qc.caused(logged); len(got) != 0 {
			t.Errorf("%s is sent %d frames for the move at seq %d, which it saw neither end of", qc.name, len(got), seq)
		}
		if qc.rawContains(reason) {
			t.Errorf("%s's wire carries the reason %q", qc.name, reason)
		}
	}
	for _, qc := range []*qaMRConn{l.dm, l.agent} {
		if got := qc.eventsAt(seq); len(got) != 1 || !proto.Equal(got[0], logged) {
			t.Errorf("%s is sent %v at seq %d, want the logged move %v", qc.name, got, seq, logged)
		}
	}
}

// VTT-262
func TestQAMoveReasonOverTheWireAPlayerCatchingUpIsSentNoReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	head := l.barrier(t)
	logged := qaMRLogged(t, l.logThrough(t, head), seq)
	for _, after := range []int64{0, seq - 1} {
		qc := qaMRDial(t, l.ctx, l.base, fmt.Sprintf("player after %d", after), l.tokens[identity.RolePlayer], after)
		qc.until(t, l.ctx, "the barrier in the backlog", func(f *vttv1.ServerFrame) bool {
			return f.GetEvent().GetNarrationAdded().GetText() == "barrier 1"
		})
		qaMRRequireSentWithoutReason(t, qc, seq, logged, reason)
	}
}

// VTT-176
func TestQAMoveReasonOverTheWireADMCatchingUpIsSentEveryEventAfterItsCursorUnchanged(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	head := l.barrier(t)
	var want []*vttv1.Envelope
	for _, env := range l.logThrough(t, head) {
		if env.GetSequence() > seq-1 {
			want = append(want, env)
		}
	}
	qc := qaMRDial(t, l.ctx, l.base, "dm after cursor", l.tokens[identity.RoleDM], seq-1)
	qc.until(t, l.ctx, "the barrier in the backlog", func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetNarrationAdded().GetText() == "barrier 1"
	})
	got := qc.events()
	if len(got) != len(want) {
		t.Fatalf("dm after cursor %d is sent %d events, want %d", seq-1, len(got), len(want))
	}
	for i := range got {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("dm after cursor event %d is\n%v\nwant the log's\n%v", i, got[i], want[i])
		}
	}
	if got[0].GetTokenMoved().GetReason() != reason {
		t.Errorf("dm after cursor is sent reason %q, want %q", got[0].GetTokenMoved().GetReason(), reason)
	}
}

// forwardedOf is env as a player or spectator gets it; keep it equal to perceive's.
func forwardedOf(env *vttv1.Envelope) *vttv1.Envelope {
	c := proto.Clone(env).(*vttv1.Envelope)
	c.ParticipantId, c.ActorRole = "", ""
	if m := c.GetTokenMoved(); m != nil {
		m.Reason = ""
	}
	if r := c.GetResourceChanged(); r != nil {
		r.Reason = ""
	}
	if a := c.GetConditionApplied(); a != nil {
		a.Source = ""
	}
	if r := c.GetConditionRemoved(); r != nil {
		r.Reason = ""
	}
	return c
}
