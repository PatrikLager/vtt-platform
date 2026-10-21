package harness_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

const (
	qaSLSMarkerID  = "qa-sls-eyes"
	qaSLSElsewhere = "qa-sls-elsewhere"
)

type qaSLSKnobs struct {
	anyPartyMember bool
	toggleWhen     func(g *qaSLSGateway, c *qaSLSConn) bool
	noSessions     bool
	restartWhen    func(g *qaSLSGateway) bool
	restartWithVC  bool
	sameVCWhen     func(g *qaSLSGateway) bool
	switchFault    func(g *qaSLSGateway, c *qaSLSConn, from, to string) int
	liveAlter      func(g *qaSLSGateway, c *qaSLSConn, e *vttv1.Envelope)
	fresh          func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope)
}

type qaSLSFreshDial struct {
	name, log, held string
	after           int64
	round           int
}

type qaSLSGateway struct {
	mu       sync.Mutex
	knobs    qaSLSKnobs
	roles    map[string]string
	pids     map[string]string
	world    []*vttv1.Envelope
	state    *engine.State
	logs     map[string][]*vttv1.Envelope
	conns    []*qaSLSConn
	live     map[string]*qaSLSConn
	dials    map[string]int
	session  string
	fresh    []qaSLSFreshDial
	switches []string
	fired    int
}

type qaSLSConn struct {
	g        *qaSLSGateway
	name     string
	role     string
	pid      string
	world    bool
	live     bool
	shown    string
	sent     int
	offset   int64
	lastSeq  int64
	ch       chan harness.Delivery
	closed   bool
	closeErr error
}

func qaSLSNewGateway(ids map[string]string, knobs qaSLSKnobs) *qaSLSGateway {
	g := &qaSLSGateway{
		knobs: knobs,
		roles: map[string]string{},
		pids:  map[string]string{},
		state: engine.NewState(),
		logs:  map[string][]*vttv1.Envelope{},
		live:  map[string]*qaSLSConn{},
		dials: map[string]int{},
	}
	for _, p := range harness.SoakParticipants() {
		g.roles[p.Name] = p.Role
		if id, ok := ids[p.Name]; ok {
			g.pids[p.Name] = id
		} else {
			g.pids[p.Name] = "unlisted-" + p.Name
		}
	}
	return g
}

func (g *qaSLSGateway) round(name string) int {
	return g.dials[name] - 1
}

func (g *qaSLSGateway) partyMembers() []string {
	var out []string
	for id, a := range g.state.Actors {
		if engine.IsPartyMember(a) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func (g *qaSLSGateway) watchable(c *qaSLSConn) []string {
	var out []string
	for _, id := range g.partyMembers() {
		if g.knobs.anyPartyMember {
			out = append(out, id)
			continue
		}
		for _, p := range g.state.Actors[id].GetControllerIds() {
			if p == c.pid {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

func (g *qaSLSGateway) mayWatch(c *qaSLSConn, actor string) bool {
	for _, id := range g.watchable(c) {
		if id == actor {
			return true
		}
	}
	return false
}

func (g *qaSLSGateway) defaultFor(c *qaSLSConn) string {
	if w := g.watchable(c); len(w) > 0 {
		return w[0]
	}
	return ""
}

func (g *qaSLSGateway) nextAfter(c *qaSLSConn, cur string) string {
	w := g.watchable(c)
	if len(w) == 0 {
		return ""
	}
	for i, id := range w {
		if id == cur {
			return w[(i+1)%len(w)]
		}
	}
	return w[0]
}

func (g *qaSLSGateway) entriesOf(actor string, after int64) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, e := range g.logs[actor] {
		if e.Sequence > after {
			out = append(out, proto.Clone(e).(*vttv1.Envelope))
		}
	}
	return out
}

func (g *qaSLSGateway) dial(name string, after int64, log string) (harness.Conn, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	role, ok := g.roles[name]
	if !ok {
		return nil, fmt.Errorf("unknown participant %q", name)
	}
	g.dials[name]++
	c := &qaSLSConn{g: g, name: name, role: role, pid: g.pids[name], ch: make(chan harness.Delivery, 4096)}
	c.world = role == "dm" || role == "agent"
	prev := g.live[name]
	c.live = prev == nil
	if c.live {
		g.live[name] = c
	}
	g.conns = append(g.conns, c)
	if c.world {
		for _, e := range g.world {
			if e.Sequence > after {
				c.push(harness.Delivery{Envelope: proto.Clone(e).(*vttv1.Envelope)})
			}
		}
		return c, nil
	}
	held, from := g.defaultFor(c), int64(0)
	if log != "" && g.mayWatch(c, log) {
		held, from = log, after
	}
	if !c.live && prev != nil {
		g.fresh = append(g.fresh, qaSLSFreshDial{name: name, log: log, held: prev.shown, after: after, round: g.round(name)})
	}
	entries := g.entriesOf(held, from)
	told := held
	if !c.live && g.knobs.fresh != nil {
		told, entries = g.knobs.fresh(g, c, held, entries)
	}
	c.shown = held
	c.push(harness.Delivery{Viewpoint: &vttv1.ViewpointChanged{ActorId: told}})
	for _, e := range entries {
		c.push(harness.Delivery{Envelope: e})
		c.lastSeq = e.Sequence
	}
	c.sent = len(g.logs[held])
	return c, nil
}

func (c *qaSLSConn) push(d harness.Delivery) {
	if c.closed {
		return
	}
	select {
	case c.ch <- d:
	default:
		c.closed = true
		c.closeErr = harness.ErrEventsOverflow
		close(c.ch)
	}
}

func (c *qaSLSConn) flush() {
	if c.shown == "" {
		return
	}
	l := c.g.logs[c.shown]
	for c.sent < len(l) && !c.closed {
		e := proto.Clone(l[c.sent]).(*vttv1.Envelope)
		e.Sequence += c.offset
		if c.live && c.g.knobs.liveAlter != nil {
			c.g.knobs.liveAlter(c.g, c, e)
		}
		c.push(harness.Delivery{Envelope: e})
		c.lastSeq = e.Sequence
		c.sent++
	}
}

func (c *qaSLSConn) switchTo(target string) {
	g := c.g
	mode := 0
	if c.live && g.knobs.switchFault != nil {
		mode = g.knobs.switchFault(g, c, c.shown, target)
	}
	if mode != 0 {
		g.fired++
	}
	if c.live {
		g.switches = append(g.switches, fmt.Sprintf("%s:%s>%s", c.name, c.shown, target))
	}
	last := c.lastSeq
	c.push(harness.Delivery{Viewpoint: &vttv1.ViewpointChanged{ActorId: target}})
	c.shown, c.sent, c.offset = target, 0, 0
	switch mode {
	case 1:
		c.sent = 1
	case 2:
		c.offset = last
	}
}

func (c *qaSLSConn) Events() <-chan harness.Delivery { return c.ch }

func (c *qaSLSConn) CloseErr() error {
	c.g.mu.Lock()
	defer c.g.mu.Unlock()
	return c.closeErr
}

func (c *qaSLSConn) Close() error {
	c.g.mu.Lock()
	defer c.g.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.closeErr = errors.New("qa fake: closed by the client")
		close(c.ch)
	}
	return nil
}

func (c *qaSLSConn) SendCommand(ctx context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error) {
	c.g.mu.Lock()
	defer c.g.mu.Unlock()
	if c.closed {
		return nil, errors.New("qa fake: connection closed")
	}
	res := &vttv1.CommandResult{RequestId: cmd.GetRequestId()}
	envs, err := c.g.decide(c, cmd)
	if err == nil && len(envs) > 0 {
		res.Sequence, err = c.g.append(c, envs)
	}
	if err != nil {
		res.Error = err.Error()
		return res, nil
	}
	res.Ok = true
	return res, nil
}

func (g *qaSLSGateway) decideMove(c *qaSLSConn, m *vttv1.MoveTokenRequest, player bool, denied error) ([]*vttv1.Envelope, error) {
	tok, ok := g.state.Tokens[m.GetTokenId()]
	if !ok {
		if player {
			return nil, denied
		}
		return nil, fmt.Errorf("unknown token %q", m.GetTokenId())
	}
	if player {
		mine := false
		for _, id := range g.state.Actors[tok.ActorID].GetControllerIds() {
			mine = mine || id == c.pid
		}
		if !mine {
			return nil, denied
		}
		if blocked, why := g.state.Blocked(tok.SceneID, m.GetTo().GetX(), m.GetTo().GetY()); blocked {
			return nil, fmt.Errorf("blocked: %s", why)
		}
	}
	return []*vttv1.Envelope{{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: m.GetTokenId(), SceneId: tok.SceneID, From: &vttv1.GridPosition{X: tok.X, Y: tok.Y}, To: m.GetTo(), Reason: m.GetReason()}}}}, nil
}

func (g *qaSLSGateway) decide(c *qaSLSConn, cmd *vttv1.ClientCommand) ([]*vttv1.Envelope, error) {
	denied := fmt.Errorf("not authorized: role %s", c.role)
	player := !c.world
	one := func(e *vttv1.Envelope) []*vttv1.Envelope { return []*vttv1.Envelope{e} }
	switch x := cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_LoadMap:
		if player {
			return nil, denied
		}
		id := x.LoadMap.GetMapId()
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: id, GridWidth: harness.SoakMapGridSize, GridHeight: harness.SoakMapGridSize}}}), nil
	case *vttv1.ClientCommand_AddActor:
		if player {
			return nil, denied
		}
		if x.AddActor.GetActor().GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
			return nil, errors.New("add_actor: kind is required")
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: x.AddActor.GetActor()}}}), nil
	case *vttv1.ClientCommand_GrantActorControl:
		if player {
			return nil, denied
		}
		gr := x.GrantActorControl
		if gr.GetKind() == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED {
			return nil, errors.New("grant_actor_control: kind is required")
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: gr.GetActorId(), ParticipantId: gr.GetParticipantId(), Kind: gr.GetKind()}}}), nil
	case *vttv1.ClientCommand_RevokeActorControl:
		rv := x.RevokeActorControl
		if player && rv.GetParticipantId() != c.pid {
			return nil, denied
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
			ActorId: rv.GetActorId(), ParticipantId: rv.GetParticipantId()}}}), nil
	case *vttv1.ClientCommand_PlaceToken:
		if player {
			return nil, denied
		}
		p := x.PlaceToken
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: p.GetTokenId(), SceneId: p.GetSceneId(), ActorId: p.GetActorId(), Position: p.GetPosition()}}}), nil
	case *vttv1.ClientCommand_MoveToken:
		return g.decideMove(c, x.MoveToken, player, denied)
	case *vttv1.ClientCommand_RemoveToken:
		if player {
			return nil, denied
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: x.RemoveToken.GetTokenId()}}}), nil
	case *vttv1.ClientCommand_RemoveActor:
		if player {
			return nil, denied
		}
		id := x.RemoveActor.GetActorId()
		var toks []string
		for tid, t := range g.state.Tokens {
			if t.ActorID == id {
				toks = append(toks, tid)
			}
		}
		sort.Strings(toks)
		var out []*vttv1.Envelope
		for _, tid := range toks {
			out = append(out, &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tid}}})
		}
		return append(out, &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: id}}}), nil
	case *vttv1.ClientCommand_StartSession:
		if player {
			return nil, denied
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: x.StartSession.GetName()}}}), nil
	case *vttv1.ClientCommand_EndSession:
		if player {
			return nil, denied
		}
		return one(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}}), nil
	default:
		if player {
			return nil, denied
		}
		return nil, fmt.Errorf("qa fake: unsupported command %T", x)
	}
}

func (g *qaSLSGateway) stamp(e *vttv1.Envelope, seq int64, sess *string) {
	e.Sequence = seq
	if _, ok := e.GetPayload().(*vttv1.Envelope_SessionStarted); ok {
		*sess = fmt.Sprintf("sess-%d", seq)
	}
	e.SessionId = *sess
	if _, ok := e.GetPayload().(*vttv1.Envelope_SessionEnded); ok {
		*sess = ""
	}
}

func (g *qaSLSGateway) append(c *qaSLSConn, envs []*vttv1.Envelope) (int64, error) {
	trial := g.state.Snapshot()
	sess := g.session
	first := int64(len(g.world) + 1)
	for i, e := range envs {
		probe := proto.Clone(e).(*vttv1.Envelope)
		g.stamp(probe, first+int64(i), &sess)
		if err := engine.Apply(trial, probe); err != nil {
			return 0, err
		}
	}
	for i, e := range envs {
		g.stamp(e, first+int64(i), &g.session)
		e.EventId = fmt.Sprintf("ev-%d", e.Sequence)
		e.ActorRole = c.role
		e.ParticipantId = c.pid
		if err := engine.Apply(g.state, e); err != nil {
			return 0, err
		}
		g.world = append(g.world, e)
		g.fanOut(e)
	}
	g.deliver(envs)
	return first, nil
}

func (g *qaSLSGateway) deliver(envs []*vttv1.Envelope) {
	restart := g.knobs.restartWhen != nil && g.knobs.restartWhen(g)
	sameVC := g.knobs.sameVCWhen != nil && g.knobs.sameVCWhen(g)
	for _, c := range g.conns {
		if c.closed {
			continue
		}
		if c.world {
			for _, e := range envs {
				c.push(harness.Delivery{Envelope: proto.Clone(e).(*vttv1.Envelope)})
			}
			continue
		}
		target := c.shown
		if target == "" || !g.mayWatch(c, target) {
			target = g.defaultFor(c)
		}
		if c.live && g.knobs.toggleWhen != nil && g.knobs.toggleWhen(g, c) {
			target = g.nextAfter(c, target)
		}
		switch {
		case target != c.shown:
			c.switchTo(target)
		case restart && c.live && c.shown != "":
			if g.knobs.restartWithVC {
				c.push(harness.Delivery{Viewpoint: &vttv1.ViewpointChanged{ActorId: c.shown}})
			}
			c.sent, c.offset = 0, 0
			g.fired++
		case sameVC && c.live && c.shown != "":
			c.push(harness.Delivery{Viewpoint: &vttv1.ViewpointChanged{ActorId: c.shown}})
			g.fired++
		}
		c.flush()
	}
}

func qaSLSIsSession(e *vttv1.Envelope) bool {
	switch e.GetPayload().(type) {
	case *vttv1.Envelope_SessionStarted, *vttv1.Envelope_SessionEnded:
		return true
	}
	return false
}

func (g *qaSLSGateway) entry(e *vttv1.Envelope) *vttv1.Envelope {
	out := proto.Clone(e).(*vttv1.Envelope)
	out.EventId, out.ActorRole, out.ParticipantId = "", "", ""
	return out
}

func (g *qaSLSGateway) fanOut(e *vttv1.Envelope) {
	keep := func(w *vttv1.Envelope) bool { return !g.knobs.noSessions || !qaSLSIsSession(w) }
	ids := g.partyMembers()
	for id := range g.logs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	done := map[string]bool{}
	for _, id := range ids {
		if done[id] {
			continue
		}
		done[id] = true
		l, has := g.logs[id]
		if !has {
			l = append(l, &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
				ActorId: qaSLSMarkerID, Name: "eyes of " + id, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}}})
			for _, w := range g.world {
				if keep(w) {
					l = append(l, g.entry(w))
				}
			}
		} else if keep(e) {
			l = append(l, g.entry(e))
		}
		for i := range l {
			l[i].Sequence = int64(i + 1)
		}
		g.logs[id] = l
	}
}

func (g *qaSLSGateway) closeAll() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, c := range g.conns {
		if !c.closed {
			c.closed = true
			c.closeErr = errors.New("qa fake: closed at the end of the test")
			close(c.ch)
		}
	}
}

type qaSLSOutcome struct {
	rep  *harness.SoakReport
	err  error
	text string
	g    *qaSLSGateway
}

func qaSLSConfig() harness.SoakConfig {
	return harness.SoakConfig{Seed: 1, Events: 200, CheckEvery: 25,
		IDs: map[string]string{"player1": "pid-player1", "player2": "pid-player2"}}
}

func qaSLSRun(t *testing.T, cfg harness.SoakConfig, knobs qaSLSKnobs) qaSLSOutcome {
	t.Helper()
	var out qaSLSOutcome
	synctest.Test(t, func(t *testing.T) {
		g := qaSLSNewGateway(cfg.IDs, knobs)
		var buf bytes.Buffer
		rep, err := harness.RunSoak(context.Background(), cfg, g.dial, &buf)
		g.closeAll()
		out = qaSLSOutcome{rep: rep, err: err, text: buf.String(), g: g}
	})
	return out
}

func (o qaSLSOutcome) failLines() string {
	var keep []string
	for _, l := range strings.Split(o.text, "\n") {
		if strings.Contains(l, "checkpoint") || strings.Contains(strings.ToLower(l), "fail") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

func qaSLSOther(g *qaSLSGateway, held string) string {
	for _, id := range g.partyMembers() {
		if id != held {
			return id
		}
	}
	return ""
}

func qaSLSPassed(t *testing.T, o qaSLSOutcome) {
	t.Helper()
	if o.err != nil {
		t.Fatalf("RunSoak returned a framework error: %v\n%s", o.err, o.failLines())
	}
	if o.rep == nil || !o.rep.Pass || o.rep.Checkpoints == 0 {
		t.Fatalf("a correct gateway must pass every checkpoint; got %+v\n%s", o.rep, o.failLines())
	}
}

func qaSLSFailed(t *testing.T, o qaSLSOutcome) {
	t.Helper()
	if o.g.fired == 0 {
		t.Fatalf("degenerate fixture: the fault never fired")
	}
	if o.err != nil {
		t.Fatalf("an assertion failure must be reported through Pass, not as a framework error: %v\n%s", o.err, o.failLines())
	}
	if o.rep == nil || o.rep.Pass {
		t.Fatalf("a soak whose character seat was sent a faulty stream must not pass; got %+v\n%s", o.rep, o.failLines())
	}
	for _, l := range strings.Split(o.text, "\n") {
		if strings.Contains(l, "FAIL") {
			t.Log(l)
		}
	}
}

func qaSLSToggling() qaSLSKnobs {
	return qaSLSKnobs{
		anyPartyMember: true,
		toggleWhen: func(g *qaSLSGateway, c *qaSLSConn) bool {
			return (c.name == "player1" && len(g.world)%9 == 0) || (c.name == "player2" && len(g.world)%14 == 0)
		},
	}
}

func qaSLSBetweenLogs(g *qaSLSGateway) int {
	n := 0
	for _, s := range g.switches {
		parts := strings.SplitN(s, ":", 2)
		ends := strings.SplitN(parts[1], ">", 2)
		if ends[0] != "" && ends[1] != "" {
			n++
		}
	}
	return n
}

// VTT-411 VTT-416 VTT-429
func TestQASLSTwinEachPlayerOnTheLogItWasGrantedPasses(t *testing.T) {
	o := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{})
	qaSLSPassed(t, o)
	held := map[string]string{}
	for _, f := range o.g.fresh {
		held[f.name] = f.held
	}
	if held["player1"] == "" || held["player2"] == "" || held["player1"] == held["player2"] {
		t.Fatalf("degenerate fixture: the players must end on two different logs, got %v", held)
	}
	a, b := o.g.logs[held["player1"]], o.g.logs[held["player2"]]
	if len(a) < 2 || len(b) < 2 || proto.Equal(a[0], b[0]) {
		t.Fatalf("degenerate fixture: the two logs must differ in a payload and hold more than one entry")
	}
}

// VTT-411 VTT-416 VTT-429
func TestQASLSTwinSeatsSwitchingBetweenLogsPass(t *testing.T) {
	for _, noSessions := range []bool{false, true} {
		t.Run(fmt.Sprintf("noSessions=%v", noSessions), func(t *testing.T) {
			k := qaSLSToggling()
			k.noSessions = noSessions
			o := qaSLSRun(t, qaSLSConfig(), k)
			qaSLSPassed(t, o)
			if n := qaSLSBetweenLogs(o.g); n < 6 {
				t.Fatalf("degenerate fixture: want at least 6 switches between two logs, got %d (%v)", n, o.g.switches)
			}
		})
	}
}

// VTT-416 VTT-411
func TestQASLSTwinRunRestartingAtSequenceOnePasses(t *testing.T) {
	for _, withVC := range []bool{false, true} {
		t.Run(fmt.Sprintf("sameLogViewpoint=%v", withVC), func(t *testing.T) {
			o := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{restartWhen: func(g *qaSLSGateway) bool { return len(g.world)%13 == 0 }, restartWithVC: withVC})
			qaSLSPassed(t, o)
			if o.g.fired == 0 {
				t.Fatalf("degenerate fixture: no restart was sent")
			}
		})
	}
}

// VTT-411
func TestQASLSTwinSameLogViewpointThenContinuingEntriesPasses(t *testing.T) {
	o := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{sameVCWhen: func(g *qaSLSGateway) bool { return len(g.world)%11 == 0 }})
	qaSLSPassed(t, o)
	if o.g.fired == 0 {
		t.Fatalf("degenerate fixture: no same-log viewpoint was sent")
	}
}

// VTT-416 VTT-429
func TestQASLSTwinSeatOnNobodyPasses(t *testing.T) {
	cfg := qaSLSConfig()
	cfg.IDs = map[string]string{"player1": "pid-player1"}
	o := qaSLSRun(t, cfg, qaSLSKnobs{})
	qaSLSPassed(t, o)
	nobody := 0
	for _, f := range o.g.fresh {
		if f.name == "player2" && f.held == "" {
			nobody++
		}
	}
	if nobody == 0 {
		t.Fatalf("degenerate fixture: no checkpoint dialled player2 while it held nobody")
	}
}

// VTT-411
func TestQASLSSwitchFromNobodyNotStartingAtOneFails(t *testing.T) {
	o := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{switchFault: func(g *qaSLSGateway, c *qaSLSConn, from, to string) int {
		if c.name == "player1" && from == "" && to != "" {
			return 1
		}
		return 0
	}})
	qaSLSFailed(t, o)
}

// VTT-411
func TestQASLSSwitchBetweenLogsNotStartingAtOneFails(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mode       int
		noSessions bool
	}{
		{"new log from its second entry", 1, false},
		{"new log numbered on from the old cursor", 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := qaSLSToggling()
			k.noSessions = tc.noSessions
			k.switchFault = func(g *qaSLSGateway, c *qaSLSConn, from, to string) int {
				if c.name == "player1" && from != "" && to != "" {
					return tc.mode
				}
				return 0
			}
			qaSLSFailed(t, qaSLSRun(t, qaSLSConfig(), k))
		})
	}
}

// VTT-411
func TestQASLSSwitchFaultAfterTheLastPeriodicCheckpointFailsTheFinalOne(t *testing.T) {
	twin := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{})
	qaSLSPassed(t, twin)
	last := twin.rep.Checkpoints - 1
	for _, faulty := range []bool{false, true} {
		t.Run(fmt.Sprintf("faulty=%v", faulty), func(t *testing.T) {
			toggled := false
			k := qaSLSKnobs{anyPartyMember: true, noSessions: true}
			k.toggleWhen = func(g *qaSLSGateway, c *qaSLSConn) bool {
				if c.name != "player1" || toggled || g.round("player1") != last {
					return false
				}
				toggled = true
				return true
			}
			if faulty {
				k.switchFault = func(g *qaSLSGateway, c *qaSLSConn, from, to string) int {
					if toggled && c.name == "player1" && from != "" && to != "" {
						return 2
					}
					return 0
				}
			}
			o := qaSLSRun(t, qaSLSConfig(), k)
			if !toggled || qaSLSBetweenLogs(o.g) == 0 {
				t.Fatalf("degenerate fixture: no switch after checkpoint %d (%v)", last, o.g.switches)
			}
			if faulty {
				qaSLSFailed(t, o)
			} else {
				qaSLSPassed(t, o)
			}
		})
	}
}

func qaSLSAlterLast(entries []*vttv1.Envelope, pick func(e *vttv1.Envelope) bool, alter func(e *vttv1.Envelope)) bool {
	for i := len(entries) - 1; i >= 0; i-- {
		if pick(entries[i]) {
			alter(entries[i])
			return true
		}
	}
	return false
}

func qaSLSAlterFirst(entries []*vttv1.Envelope, pick func(e *vttv1.Envelope) bool, alter func(e *vttv1.Envelope)) bool {
	for _, e := range entries {
		if pick(e) {
			alter(e)
			return true
		}
	}
	return false
}

func qaSLSFreshFaults() map[string]func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
	only := func(f func(g *qaSLSGateway, held string, entries []*vttv1.Envelope) bool) func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
		return func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
			if c.name == "player1" && held != "" && f(g, held, entries) {
				g.fired++
			}
			return held, entries
		}
	}
	return map[string]func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope){
		"an actor's name": only(func(g *qaSLSGateway, held string, entries []*vttv1.Envelope) bool {
			return qaSLSAlterFirst(entries, func(e *vttv1.Envelope) bool { return e.GetActorAdded() != nil },
				func(e *vttv1.Envelope) { e.GetActorAdded().Actor.Name += " (altered)" })
		}),
		"a scene's name": only(func(g *qaSLSGateway, held string, entries []*vttv1.Envelope) bool {
			return qaSLSAlterFirst(entries, func(e *vttv1.Envelope) bool { return e.GetSceneCreated() != nil },
				func(e *vttv1.Envelope) { e.GetSceneCreated().Name += " (altered)" })
		}),
		"a session's name": only(func(g *qaSLSGateway, held string, entries []*vttv1.Envelope) bool {
			return qaSLSAlterFirst(entries, func(e *vttv1.Envelope) bool { return e.GetSessionStarted() != nil },
				func(e *vttv1.Envelope) { e.GetSessionStarted().Name += " (altered)" })
		}),
		"a token's square": only(func(g *qaSLSGateway, held string, entries []*vttv1.Envelope) bool {
			return qaSLSAlterLast(entries, func(e *vttv1.Envelope) bool { return e.GetTokenMoved() != nil || e.GetTokenPlaced() != nil },
				func(e *vttv1.Envelope) {
					if m := e.GetTokenMoved(); m != nil {
						m.To = &vttv1.GridPosition{X: (m.GetTo().GetX() + 1) % harness.SoakMapGridSize, Y: m.GetTo().GetY()}
						return
					}
					p := e.GetTokenPlaced()
					p.Position = &vttv1.GridPosition{X: (p.GetPosition().GetX() + 1) % harness.SoakMapGridSize, Y: p.GetPosition().GetY()}
				})
		}),
		"the other player's log": func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
			other := qaSLSOther(g, held)
			if c.name != "player1" || held == "" || other == "" {
				return held, entries
			}
			swapped := g.entriesOf(other, 0)
			if len(swapped) != len(entries) {
				return held, entries
			}
			for i := range swapped {
				if swapped[i].Sequence != entries[i].Sequence || swapped[i].EventId != "" {
					return held, entries
				}
			}
			g.fired++
			return held, swapped
		},
		"one entry short": func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
			if c.name != "player1" || len(entries) < 2 {
				return held, entries
			}
			g.fired++
			return held, entries[:len(entries)-1]
		},
	}
}

// VTT-416
func TestQASLSFreshDialFoldingOtherwiseThanTheHeldLogFails(t *testing.T) {
	faults := qaSLSFreshFaults()
	names := make([]string, 0, len(faults))
	for n := range faults {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		t.Run(n, func(t *testing.T) {
			qaSLSFailed(t, qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{fresh: faults[n]}))
		})
	}
}

// VTT-416
func TestQASLSLiveEntryDifferingOnlyInPayloadFails(t *testing.T) {
	o := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{liveAlter: func(g *qaSLSGateway, c *qaSLSConn, e *vttv1.Envelope) {
		if c.name == "player1" && e.GetActorAdded().GetActor().GetActorId() == qaSLSMarkerID {
			e.GetActorAdded().Actor.Name = "eyes of " + qaSLSOther(g, c.shown)
			g.fired++
		}
	}})
	qaSLSFailed(t, o)
}

// VTT-416
func TestQASLSFreshFoldFaultOnlyAtTheFinalCheckpointFails(t *testing.T) {
	twin := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{})
	qaSLSPassed(t, twin)
	final := twin.rep.Checkpoints
	inner := qaSLSFreshFaults()["the other player's log"]
	o := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{fresh: func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
		if g.round(c.name) != final {
			return held, entries
		}
		return inner(g, c, held, entries)
	}})
	qaSLSFailed(t, o)
}

func qaSLSTold(pick func(g *qaSLSGateway, held string) string) func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
	return func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
		if c.name != "player1" || held == "" {
			return held, entries
		}
		told := pick(g, held)
		if told == held {
			return held, entries
		}
		g.fired++
		return told, entries
	}
}

// VTT-429
func TestQASLSFreshDialToldAnotherLogFails(t *testing.T) {
	for _, tc := range []struct {
		name  string
		knobs func() qaSLSKnobs
	}{
		{"the other player's party member", func() qaSLSKnobs {
			return qaSLSKnobs{fresh: qaSLSTold(func(g *qaSLSGateway, held string) string { return qaSLSOther(g, held) })}
		}},
		{"an actor nobody holds", func() qaSLSKnobs {
			return qaSLSKnobs{fresh: qaSLSTold(func(g *qaSLSGateway, held string) string { return qaSLSElsewhere })}
		}},
		{"nobody", func() qaSLSKnobs {
			return qaSLSKnobs{fresh: qaSLSTold(func(g *qaSLSGateway, held string) string { return "" })}
		}},
		{"the default, the named log ignored", func() qaSLSKnobs {
			k := qaSLSToggling()
			k.fresh = func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
				def := g.defaultFor(c)
				if def == held {
					return held, entries
				}
				g.fired++
				return def, g.entriesOf(def, 0)
			}
			return k
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qaSLSFailed(t, qaSLSRun(t, qaSLSConfig(), tc.knobs()))
		})
	}
}

// VTT-429
func TestQASLSFreshDialOfASeatOnNobodyToldALogFails(t *testing.T) {
	for _, withEntries := range []bool{false, true} {
		t.Run(fmt.Sprintf("withEntries=%v", withEntries), func(t *testing.T) {
			cfg := qaSLSConfig()
			cfg.IDs = map[string]string{"player1": "pid-player1"}
			o := qaSLSRun(t, cfg, qaSLSKnobs{fresh: func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
				if c.name != "player2" || held != "" {
					return held, entries
				}
				other := qaSLSOther(g, "")
				if other == "" {
					return held, entries
				}
				g.fired++
				if withEntries {
					return other, g.entriesOf(other, 0)
				}
				return other, nil
			}})
			qaSLSFailed(t, o)
		})
	}
}

// VTT-429
func TestQASLSFreshDialToldAnotherLogOnlyAtTheFinalCheckpointFails(t *testing.T) {
	twin := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{})
	qaSLSPassed(t, twin)
	final := twin.rep.Checkpoints
	inner := qaSLSTold(func(g *qaSLSGateway, held string) string { return qaSLSOther(g, held) })
	o := qaSLSRun(t, qaSLSConfig(), qaSLSKnobs{fresh: func(g *qaSLSGateway, c *qaSLSConn, held string, entries []*vttv1.Envelope) (string, []*vttv1.Envelope) {
		if g.round(c.name) != final {
			return held, entries
		}
		return inner(g, c, held, entries)
	}})
	qaSLSFailed(t, o)
}

// VTT-428
func TestQASLSFreshDialNamesTheLogTheSeatHolds(t *testing.T) {
	nobody := qaSLSConfig()
	nobody.IDs = map[string]string{"player1": "pid-player1"}
	for _, tc := range []struct {
		name  string
		cfg   harness.SoakConfig
		knobs qaSLSKnobs
	}{
		{"granted", qaSLSConfig(), qaSLSKnobs{}},
		{"switching", qaSLSConfig(), qaSLSToggling()},
		{"one on nobody", nobody, qaSLSKnobs{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := qaSLSRun(t, tc.cfg, tc.knobs)
			seen := map[string]map[string]bool{}
			for _, f := range o.g.fresh {
				if f.log != f.held {
					t.Errorf("checkpoint %d dialled %s naming log %q while its seat held %q", f.round, f.name, f.log, f.held)
				}
				if seen[f.name] == nil {
					seen[f.name] = map[string]bool{}
				}
				seen[f.name][f.held] = true
			}
			if len(seen["player1"]) == 0 || len(seen["player2"]) == 0 {
				t.Fatalf("degenerate fixture: a player was never dialled fresh (%v)", seen)
			}
			if tc.name == "switching" && len(seen["player1"]) < 2 {
				t.Fatalf("degenerate fixture: player1 held one log at every checkpoint (%v)", seen)
			}
			if tc.name == "one on nobody" && !seen["player2"][""] {
				t.Fatalf("degenerate fixture: player2 never held nobody at a checkpoint (%v)", seen)
			}
		})
	}
}
