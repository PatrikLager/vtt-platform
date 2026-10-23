package gateway_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
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

const qappWait = 4 * time.Second

type qappTable struct {
	t     *testing.T
	dir   string
	c     *campaign.Campaign
	ids   *identity.DB
	srv   *httptest.Server
	dmID  string
	dmTok string
}

func qappNewTable(t *testing.T) *qappTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	tb := &qappTable{t: t, dir: dir, c: c, ids: ids, srv: srv}
	t.Cleanup(func() {
		srv.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	tb.dmTok, tb.dmID = tb.invite("Dungeon Master", identity.RoleDM)
	return tb
}

func (tb *qappTable) invite(name string, role identity.Role) (token, id string) {
	tb.t.Helper()
	token, id, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatalf("CreateInvite(%q): %v", name, err)
	}
	return token, id
}

func qappEventID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func (tb *qappTable) env(set func(e *vttv1.Envelope)) *vttv1.Envelope {
	e := &vttv1.Envelope{
		EventId:       qappEventID(),
		OccurredAt:    timestamppb.Now(),
		ActorRole:     string(identity.RoleDM),
		ParticipantId: tb.dmID,
	}
	set(e)
	return e
}

func (tb *qappTable) append(envs ...*vttv1.Envelope) {
	tb.t.Helper()
	if _, err := tb.c.AppendBatch(envs); err != nil {
		tb.t.Fatalf("AppendBatch: %v", err)
	}
}

func (tb *qappTable) actorEnv(id, name string, kind vttv1.ActorKind) *vttv1.Envelope {
	return tb.env(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: id, Name: name, ModuleId: "qa", Kind: kind,
		}}}
	})
}

func (tb *qappTable) grantEnv(actorID, pid string, kind vttv1.ActorKind) *vttv1.Envelope {
	return tb.env(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actorID, ParticipantId: pid, Kind: kind,
		}}
	})
}

func (tb *qappTable) revokeEnv(actorID, pid string) *vttv1.Envelope {
	return tb.env(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
			ActorId: actorID, ParticipantId: pid,
		}}
	})
}

func (tb *qappTable) narrationEnv(text string) *vttv1.Envelope {
	return tb.env(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}
	})
}

func (tb *qappTable) addActor(id, name string, kind vttv1.ActorKind, controllers ...string) {
	tb.t.Helper()
	envs := []*vttv1.Envelope{tb.actorEnv(id, name, kind)}
	for _, pid := range controllers {
		envs = append(envs, tb.grantEnv(id, pid, kind))
	}
	tb.append(envs...)
}

func (tb *qappTable) grant(actorID, pid string, kind vttv1.ActorKind) {
	tb.t.Helper()
	tb.append(tb.grantEnv(actorID, pid, kind))
}

func (tb *qappTable) revoke(actorID, pid string) {
	tb.t.Helper()
	tb.append(tb.revokeEnv(actorID, pid))
}

var qappTextSeq atomic.Int64

func qappText(label string) string {
	return fmt.Sprintf("qapp %s %d", label, qappTextSeq.Add(1))
}

func (tb *qappTable) barrier(conns ...*qappConn) []int {
	tb.t.Helper()
	text := qappText("barrier")
	tb.append(tb.narrationEnv(text))
	idx := make([]int, len(conns))
	for i, cn := range conns {
		idx[i] = cn.waitNarration(0, text)
	}
	return idx
}

func (tb *qappTable) logOf(actorID string) []campaign.CharacterEntry {
	tb.t.Helper()
	entries, err := tb.c.CharacterLog(actorID)
	if err != nil {
		tb.t.Fatalf("CharacterLog(%q): %v", actorID, err)
	}
	return entries
}

type qappConn struct {
	t      *testing.T
	name   string
	ws     *websocket.Conn
	mu     sync.Mutex
	frames []*vttv1.ServerFrame
	err    error
	reqSeq atomic.Int64
}

func (tb *qappTable) dial(name, token, log string) *qappConn {
	tb.t.Helper()
	q := url.Values{}
	q.Set("token", token)
	q.Set("after", "0")
	if log != "" {
		q.Set("log", log)
	}
	u := strings.Replace(tb.srv.URL, "http", "ws", 1) + "/ws?" + q.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), qappWait)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		tb.t.Fatalf("dial %s: %v", name, err)
	}
	ws.SetReadLimit(-1)
	cn := &qappConn{t: tb.t, name: name, ws: ws}
	readCtx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, data, err := ws.Read(readCtx)
			if err != nil {
				cn.mu.Lock()
				cn.err = err
				cn.mu.Unlock()
				return
			}
			f := &vttv1.ServerFrame{}
			if err := protojson.Unmarshal(data, f); err != nil {
				cn.mu.Lock()
				cn.err = fmt.Errorf("decode: %w", err)
				cn.mu.Unlock()
				return
			}
			cn.mu.Lock()
			cn.frames = append(cn.frames, f)
			cn.mu.Unlock()
		}
	}()
	tb.t.Cleanup(func() {
		_ = ws.Close(websocket.StatusNormalClosure, "")
		stop()
		<-done
	})
	return cn
}

func (cn *qappConn) snapshot() []*vttv1.ServerFrame {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	return append([]*vttv1.ServerFrame(nil), cn.frames...)
}

func (cn *qappConn) count() int {
	cn.mu.Lock()
	defer cn.mu.Unlock()
	return len(cn.frames)
}

func qappDescribe(f *vttv1.ServerFrame) string {
	switch x := f.GetFrame().(type) {
	case *vttv1.ServerFrame_ViewpointChanged:
		return fmt.Sprintf("viewpoint(%q)", x.ViewpointChanged.GetActorId())
	case *vttv1.ServerFrame_WatchableChanged:
		return fmt.Sprintf("watchable(%v)", qappWatchIDs(x.WatchableChanged))
	case *vttv1.ServerFrame_Result:
		return fmt.Sprintf("result(%s ok=%v %q)", x.Result.GetRequestId(), x.Result.GetOk(), x.Result.GetError())
	case *vttv1.ServerFrame_Event:
		e := x.Event
		if n := e.GetNarrationAdded(); n != nil {
			return fmt.Sprintf("event#%d(narration %q)", e.GetSequence(), n.GetText())
		}
		return fmt.Sprintf("event#%d(%T)", e.GetSequence(), e.GetPayload())
	case *vttv1.ServerFrame_CatchUpHead:
		return fmt.Sprintf("head(%d)", x.CatchUpHead.GetHeadSequence())
	default:
		return fmt.Sprintf("%T", x)
	}
}

func (cn *qappConn) dump() string {
	var b strings.Builder
	for i, f := range cn.snapshot() {
		fmt.Fprintf(&b, "  [%d] %s\n", i, qappDescribe(f))
	}
	cn.mu.Lock()
	if cn.err != nil {
		fmt.Fprintf(&b, "  read error: %v\n", cn.err)
	}
	cn.mu.Unlock()
	return b.String()
}

func (cn *qappConn) wait(from int, what string, pred func(*vttv1.ServerFrame) bool) (int, *vttv1.ServerFrame) {
	cn.t.Helper()
	deadline := time.Now().Add(qappWait)
	for {
		frames := cn.snapshot()
		for i := from; i < len(frames); i++ {
			if pred(frames[i]) {
				return i, frames[i]
			}
		}
		if time.Now().After(deadline) {
			cn.t.Fatalf("%s: no %s from frame %d within %v; frames:\n%s", cn.name, what, from, qappWait, cn.dump())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (cn *qappConn) waitViewpoint(from int, actorID string) int {
	cn.t.Helper()
	i, _ := cn.wait(from, fmt.Sprintf("viewpoint %q", actorID), func(f *vttv1.ServerFrame) bool {
		v := f.GetViewpointChanged()
		return v != nil && v.GetActorId() == actorID
	})
	return i
}

func (cn *qappConn) waitNarration(from int, text string) int {
	cn.t.Helper()
	i, _ := cn.wait(from, fmt.Sprintf("narration %q", text), func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetNarrationAdded().GetText() == text
	})
	return i
}

func (cn *qappConn) waitWatchable(from int) (int, *vttv1.WatchableChanged) {
	cn.t.Helper()
	i, f := cn.wait(from, "watchable", func(f *vttv1.ServerFrame) bool {
		return f.GetWatchableChanged() != nil
	})
	return i, f.GetWatchableChanged()
}

func (cn *qappConn) viewpoints(from, to int) []string {
	frames := cn.snapshot()
	if to > len(frames) {
		to = len(frames)
	}
	var out []string
	for i := from; i < to; i++ {
		if v := frames[i].GetViewpointChanged(); v != nil {
			out = append(out, v.GetActorId())
		}
	}
	return out
}

func (cn *qappConn) watchables(from, to int) []*vttv1.WatchableChanged {
	frames := cn.snapshot()
	if to > len(frames) {
		to = len(frames)
	}
	var out []*vttv1.WatchableChanged
	for i := from; i < to; i++ {
		if w := frames[i].GetWatchableChanged(); w != nil {
			out = append(out, w)
		}
	}
	return out
}

func (cn *qappConn) shown(to int) (string, bool) {
	vs := cn.viewpoints(0, to)
	if len(vs) == 0 {
		return "", false
	}
	return vs[len(vs)-1], true
}

func (cn *qappConn) send(cmd *vttv1.ClientCommand) string {
	cn.t.Helper()
	id := fmt.Sprintf("%s-req-%d", cn.name, cn.reqSeq.Add(1))
	cmd.RequestId = id
	data, err := protojson.Marshal(cmd)
	if err != nil {
		cn.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qappWait)
	defer cancel()
	if err := cn.ws.Write(ctx, websocket.MessageText, data); err != nil {
		cn.t.Fatalf("%s: write: %v", cn.name, err)
	}
	return id
}

func (cn *qappConn) result(reqID string) *vttv1.CommandResult {
	cn.t.Helper()
	_, f := cn.wait(0, "result "+reqID, func(f *vttv1.ServerFrame) bool {
		return f.GetResult().GetRequestId() == reqID
	})
	return f.GetResult()
}

func (cn *qappConn) do(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	cn.t.Helper()
	return cn.result(cn.send(cmd))
}

func (cn *qappConn) perch(actorID string) *vttv1.CommandResult {
	cn.t.Helper()
	return cn.do(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID},
	}})
}

func qappWatchIDs(w *vttv1.WatchableChanged) []string {
	var out []string
	for _, a := range w.GetActors() {
		out = append(out, a.GetActorId())
	}
	return out
}

func (cn *qappConn) events(from, to int) []*vttv1.Envelope {
	frames := cn.snapshot()
	if to > len(frames) {
		to = len(frames)
	}
	var out []*vttv1.Envelope
	for i := from; i < to; i++ {
		if e := frames[i].GetEvent(); e != nil {
			out = append(out, e)
		}
	}
	return out
}

func qappInLog(e *vttv1.Envelope, log []campaign.CharacterEntry) bool {
	for _, entry := range log {
		if proto.Equal(e, entry.Env) {
			return true
		}
	}
	return false
}

func (tb *qappTable) sceneEnv() *vttv1.Envelope {
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 9; x++ {
		for y := 0; y < 3; y++ {
			kind := "floor"
			if x == 4 {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind, Material: "stone"}
		}
	}
	return tb.env(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "s1", Name: "Two rooms", GridWidth: 9, GridHeight: 3, Tiles: tiles,
		}}
	})
}

func (tb *qappTable) tokenEnv(tokenID, actorID string, x, y int32) *vttv1.Envelope {
	return tb.env(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: tokenID, SceneId: "s1", ActorId: actorID, Position: &vttv1.GridPosition{X: x, Y: y},
		}}
	})
}

func (cn *qappConn) move(tokenID string, x, y int32) *vttv1.CommandResult {
	cn.t.Helper()
	return cn.do(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: tokenID, To: &vttv1.GridPosition{X: x, Y: y}},
	}})
}

const (
	qappParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qappNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

func (tb *qappTable) dm() *qappConn {
	tb.t.Helper()
	return tb.dial("dm", tb.dmTok, "")
}

func (cn *qappConn) firstViewpoint() string {
	cn.t.Helper()
	_, f := cn.wait(0, "a viewpoint", func(f *vttv1.ServerFrame) bool {
		return f.GetViewpointChanged() != nil
	})
	return f.GetViewpointChanged().GetActorId()
}

func (cn *qappConn) mustOK(r *vttv1.CommandResult, what string) {
	cn.t.Helper()
	if !r.GetOk() {
		cn.t.Fatalf("%s: %s refused: %q", cn.name, what, r.GetError())
	}
}

func (cn *qappConn) mustRefuse(r *vttv1.CommandResult, what string) {
	cn.t.Helper()
	if r.GetOk() {
		cn.t.Fatalf("%s: %s accepted, want refused; frames:\n%s", cn.name, what, cn.dump())
	}
}

func (tb *qappTable) expectWatching(actorID string, from int, conns ...*qappConn) {
	tb.t.Helper()
	text := qappText("watching")
	tb.append(tb.narrationEnv(text))
	log := tb.logOf(actorID)
	for _, cn := range conns {
		idx := cn.waitNarration(from, text)
		if moved := cn.viewpoints(from, idx+1); len(moved) != 0 {
			tb.t.Fatalf("%s: ViewpointChanged %q sent after frame %d, want none; frames:\n%s", cn.name, moved, from, cn.dump())
		}
		if got, _ := cn.shown(idx); got != actorID {
			tb.t.Fatalf("%s: shows %q, want %q; frames:\n%s", cn.name, got, actorID, cn.dump())
		}
		if e := cn.snapshot()[idx].GetEvent(); !qappInLog(e, log) {
			tb.t.Fatalf("%s: narration %v is not an entry of %s's log", cn.name, e, actorID)
		}
	}
}

func qappWatchable(pairs ...string) *vttv1.WatchableChanged {
	w := &vttv1.WatchableChanged{}
	for i := 0; i+1 < len(pairs); i += 2 {
		w.Actors = append(w.Actors, &vttv1.WatchableActor{ActorId: pairs[i], Name: pairs[i+1]})
	}
	return w
}

func qappRefusalShape(r *vttv1.CommandResult, id string) string {
	if id == "" {
		return r.GetError()
	}
	return strings.ReplaceAll(r.GetError(), id, "<id>")
}

func (tb *qappTable) dmDo(dm *qappConn, cmd *vttv1.ClientCommand, what string) {
	tb.t.Helper()
	dm.mustOK(dm.do(cmd), what)
}

func qappRevokeCmd(actorID, pid string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{ActorId: actorID, ParticipantId: pid},
	}}
}

func qappPromoteCmd(pid string, role identity.Role) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PromoteParticipant{
		PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: pid, Role: string(role)},
	}}
}

func qappRemoveActorCmd(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveActor{
		RemoveActor: &vttv1.RemoveActor{ActorId: actorID},
	}}
}

// VTT-450 VTT-347 VTT-340 VTT-193
func TestQAPPAPlayerWithAPartyMemberWatchesOnlyTheirOwn(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-d", "Dora", qappParty, pID)
	tb.addActor("pm-b", "Bert", qappParty, pID)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-c", "Cato", qappParty)
	tb.addActor("np-g", "Gobo", qappNonParty, pID)
	tb.addActor("np-m", "Mold", qappNonParty)

	cn := tb.dial("p", pTok, "")
	if got := cn.firstViewpoint(); got != "pm-b" {
		t.Fatalf("opening viewpoint %q, want pm-b, the first by id the player controls", got)
	}
	cn.mustOK(cn.perch("pm-d"), "perch on own pm-d")
	atD := cn.waitViewpoint(0, "pm-d")
	cn.mustOK(cn.perch("pm-b"), "perch on own pm-b")
	from := cn.waitViewpoint(atD+1, "pm-b") + 1

	shapes := map[string]string{}
	for _, id := range []string{"pm-a", "pm-c", "np-g", "np-m", "zz-nope"} {
		r := cn.perch(id)
		cn.mustRefuse(r, "perch on "+id)
		shapes[id] = qappRefusalShape(r, id)
	}
	for id, shape := range shapes {
		if shape != shapes["zz-nope"] {
			t.Fatalf("refusal for %s is %q, for an absent actor %q: want one string", id, shape, shapes["zz-nope"])
		}
	}
	tb.expectWatching("pm-b", from, cn)
}

// VTT-451 VTT-193
func TestQAPPAPlayerWithAPartyMemberIsRefusedNobody(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	tb.addActor("pm-b", "Bert", qappParty, pID)
	tb.addActor("pm-a", "Alma", qappParty)

	cn := tb.dial("p", pTok, "")
	from := cn.waitViewpoint(0, "pm-b") + 1
	cn.mustRefuse(cn.perch(""), "perch on nobody")
	tb.expectWatching("pm-b", from, cn)
}

// VTT-452 VTT-347 VTT-340 VTT-193
func TestQAPPAPlayerWithoutAPartyMemberWatchesAnyPartyMemberOrNobody(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-c", "Cato", qappParty)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("np-g", "Gobo", qappNonParty, pID)
	tb.addActor("np-m", "Mold", qappNonParty)

	cn := tb.dial("p", pTok, "")
	if got := cn.firstViewpoint(); got != "" {
		t.Fatalf("opening viewpoint %q, want nobody", got)
	}
	cn.mustOK(cn.perch("pm-a"), "perch on another player's pm-a")
	atA := cn.waitViewpoint(0, "pm-a") + 1
	tb.expectWatching("pm-a", atA, cn)
	logA := tb.logOf("pm-a")
	end := cn.count()
	for _, e := range cn.events(atA, end) {
		if !qappInLog(e, logA) {
			t.Fatalf("entry %v sent while watching pm-a is not an entry of pm-a's log", e)
		}
	}

	cn.mustOK(cn.perch("pm-c"), "perch on the unassigned pm-c")
	atC := cn.waitViewpoint(atA, "pm-c") + 1

	shapes := map[string]string{}
	for _, id := range []string{"np-g", "np-m", "zz-nope"} {
		r := cn.perch(id)
		cn.mustRefuse(r, "perch on "+id)
		shapes[id] = qappRefusalShape(r, id)
	}
	for id, shape := range shapes {
		if shape != shapes["zz-nope"] {
			t.Fatalf("refusal for %s is %q, for an absent actor %q: want one string", id, shape, shapes["zz-nope"])
		}
	}
	tb.expectWatching("pm-c", atC, cn)

	cn.mustOK(cn.perch(""), "perch on nobody")
	cn.waitViewpoint(atC, "")
}

// VTT-339 VTT-452 VTT-340
func TestQAPPAPlayerWithoutAPartyMemberOpensOnTheLogItNames(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	qTok, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-c", "Cato", qappParty)
	tb.addActor("pm-a", "Alma", qappParty)
	tb.addActor("pm-b", "Bert", qappParty, qID)
	tb.addActor("np-g", "Gobo", qappNonParty, pID)
	tb.addActor("np-m", "Mold", qappNonParty)

	onC := tb.dial("p-log-pm-c", pTok, "pm-c")
	if got := onC.firstViewpoint(); got != "pm-c" {
		t.Fatalf("log=pm-c opened on %q, want pm-c", got)
	}
	tb.expectWatching("pm-c", onC.waitViewpoint(0, "pm-c")+1, onC)
	onB := tb.dial("p-log-pm-b", pTok, "pm-b")
	if got := onB.firstViewpoint(); got != "pm-b" {
		t.Fatalf("log=pm-b, another player's, opened on %q, want pm-b", got)
	}
	for _, log := range []string{"np-g", "np-m", "zz-nope"} {
		cn := tb.dial("p-log-"+log, pTok, log)
		if got := cn.firstViewpoint(); got != "" {
			t.Fatalf("log=%s opened on %q, want nobody", log, got)
		}
	}
	qn := tb.dial("q-log-pm-c", qTok, "pm-c")
	if got := qn.firstViewpoint(); got != "pm-b" {
		t.Fatalf("a player who controls pm-b naming log=pm-c opened on %q, want pm-b", got)
	}
}

// VTT-399 VTT-452 VTT-401
func TestQAPPAPlayerWithoutAPartyMemberIsToldEveryPartyMember(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	qTok, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-c", "Cato", qappParty)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-b", "Bert", qappParty)
	tb.addActor("np-g", "Gobo", qappNonParty, pID)
	tb.addActor("np-m", "Mold", qappNonParty)

	_, got := tb.dial("p", pTok, "").waitWatchable(0)
	if want := qappWatchable("pm-a", "Alma", "pm-b", "Bert", "pm-c", "Cato"); !proto.Equal(got, want) {
		t.Fatalf("a player with no party member is told %v, want %v", got, want)
	}
	_, got = tb.dial("q", qTok, "").waitWatchable(0)
	if want := qappWatchable("pm-a", "Alma"); !proto.Equal(got, want) {
		t.Fatalf("a player who controls pm-a is told %v, want %v", got, want)
	}
	dm := tb.dm()
	idx := tb.barrier(dm)[0]
	if ws := dm.watchables(0, idx+1); len(ws) != 0 {
		t.Fatalf("the DM's connection was sent %d WatchableChanged, want none", len(ws))
	}

	bare := qappNewTable(t)
	rTok, rID := bare.invite("Rune", identity.RolePlayer)
	bare.addActor("np-g", "Gobo", qappNonParty, rID)
	_, got = bare.dial("r", rTok, "").waitWatchable(0)
	if len(got.GetActors()) != 0 {
		t.Fatalf("with no party member at the table the player is told %v, want none", got)
	}
}

// VTT-400 VTT-452 VTT-456
func TestQAPPTheWatchableSetFollowsThePlayersControl(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-c", "Cato", qappParty)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-b", "Bert", qappParty, pID)
	tb.addActor("np-g", "Gobo", qappNonParty, pID)

	cn := tb.dial("p", pTok, "")
	_, got := cn.waitWatchable(0)
	if want := qappWatchable("pm-b", "Bert"); !proto.Equal(got, want) {
		t.Fatalf("opening set %v, want %v", got, want)
	}
	step := func(what string, want *vttv1.WatchableChanged, envs ...*vttv1.Envelope) int {
		t.Helper()
		from := cn.count()
		tb.append(envs...)
		i, got := cn.waitWatchable(from)
		if !proto.Equal(got, want) {
			t.Fatalf("after %s the set is %v, want %v; frames:\n%s", what, got, want, cn.dump())
		}
		return i
	}
	at := step("revoking pm-b, the last party member", qappWatchable("pm-a", "Alma", "pm-b", "Bert", "pm-c", "Cato"),
		tb.revokeEnv("pm-b", pID))
	tb.expectWatching("pm-b", at+1, cn)

	cn.mustOK(cn.perch("pm-a"), "perch on pm-a")
	cn.waitViewpoint(at, "pm-a")
	tb.revoke("pm-a", qID)
	idx := tb.barrier(cn)[0]
	if ws := cn.watchables(at+1, idx+1); len(ws) != 0 {
		t.Fatalf("an unchanged set was sent again: %v; frames:\n%s", ws, cn.dump())
	}

	step("adding pm-e", qappWatchable("pm-a", "Alma", "pm-b", "Bert", "pm-c", "Cato", "pm-e", "Edda"),
		tb.actorEnv("pm-e", "Edda", qappParty))
	step("making pm-c non-party", qappWatchable("pm-a", "Alma", "pm-b", "Bert", "pm-e", "Edda"),
		tb.grantEnv("pm-c", qID, qappNonParty))
	step("granting pm-e", qappWatchable("pm-e", "Edda"),
		tb.grantEnv("pm-e", pID, qappParty))
}

// VTT-456 VTT-339 VTT-340
func TestQAPPRevokingAPlayersOnlyPartyMemberLeavesThemWatchingIt(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-t", "Tobbe", qappParty, pID)
	dm := tb.dm()

	plain := tb.dial("p-plain", pTok, "")
	named := tb.dial("p-named", pTok, "pm-t")
	fromPlain := plain.waitViewpoint(0, "pm-t") + 1
	fromNamed := named.waitViewpoint(0, "pm-t") + 1
	tb.expectWatching("pm-t", fromPlain, plain)
	tb.expectWatching("pm-t", fromNamed, named)

	tb.dmDo(dm, qappRevokeCmd("pm-t", pID), "revoke pm-t")
	tb.expectWatching("pm-t", fromPlain, plain)
	tb.expectWatching("pm-t", fromNamed, named)

	again := tb.dial("p-again", pTok, "pm-t")
	if got := again.firstViewpoint(); got != "pm-t" {
		t.Fatalf("reconnecting with log=pm-t after the revocation opened on %q, want pm-t", got)
	}
	fresh := tb.dial("p-fresh", pTok, "")
	if got := fresh.firstViewpoint(); got != "" {
		t.Fatalf("reconnecting with no log after the revocation opened on %q, want nobody", got)
	}
}

// VTT-340 VTT-456
func TestQAPPLosingOneOfTwoPartyMembersMovesLosingTheLastDoesNot(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-d", "Dora", qappParty, pID)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-b", "Bert", qappParty, pID)

	cn := tb.dial("p", pTok, "")
	cn.waitViewpoint(0, "pm-b")
	cn.mustOK(cn.perch("pm-d"), "perch on pm-d")
	from := cn.waitViewpoint(0, "pm-d") + 1

	tb.revoke("pm-d", pID)
	from = cn.waitViewpoint(from, "pm-b") + 1
	if moved := cn.viewpoints(from-1, cn.count()); len(moved) != 1 {
		t.Fatalf("revoking pm-d sent viewpoints %q, want only pm-b", moved)
	}
	tb.expectWatching("pm-b", from, cn)

	tb.revoke("pm-b", pID)
	tb.expectWatching("pm-b", from, cn)
}

// VTT-340 VTT-452
func TestQAPPAWatchedActorThatLeavesThePartyMovesThePlayerToNobody(t *testing.T) {
	setup := func(t *testing.T) (*qappTable, *qappConn, string, string) {
		t.Helper()
		tb := qappNewTable(t)
		pTok, pID := tb.invite("Pia", identity.RolePlayer)
		_, qID := tb.invite("Quinn", identity.RolePlayer)
		tb.addActor("pm-a", "Alma", qappParty, qID)
		tb.addActor("pm-c", "Cato", qappParty)
		tb.addActor("pm-t", "Tobbe", qappParty, pID)
		return tb, tb.dial("p", pTok, ""), pID, qID
	}
	t.Run("their own made non-party", func(t *testing.T) {
		tb, cn, pID, _ := setup(t)
		from := cn.waitViewpoint(0, "pm-t") + 1
		tb.grant("pm-t", pID, qappNonParty)
		cn.waitViewpoint(from, "")
	})
	t.Run("their own removed", func(t *testing.T) {
		tb, cn, _, _ := setup(t)
		from := cn.waitViewpoint(0, "pm-t") + 1
		tb.dmDo(tb.dm(), qappRemoveActorCmd("pm-t"), "remove pm-t")
		cn.waitViewpoint(from, "")
	})
	t.Run("another's made non-party", func(t *testing.T) {
		tb, cn, pID, qID := setup(t)
		cn.waitViewpoint(0, "pm-t")
		tb.revoke("pm-t", pID)
		cn.mustOK(cn.perch("pm-a"), "perch on pm-a")
		from := cn.waitViewpoint(0, "pm-a") + 1
		tb.grant("pm-a", qID, qappNonParty)
		cn.waitViewpoint(from, "")
	})
}

// VTT-340 VTT-348
func TestQAPPAGrantTakesAWatcherOffAnotherPartyMembersLog(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("np-g", "Gobo", qappNonParty, pID)
	for i := 0; i < 12; i++ {
		tb.append(tb.narrationEnv(qappText("history")))
	}
	tb.addActor("pm-y", "Ylva", qappParty)

	cn := tb.dial("p", pTok, "")
	cn.mustOK(cn.perch("pm-a"), "perch on pm-a")
	cn.waitViewpoint(0, "pm-a")
	tb.barrier(cn)

	before := len(tb.logOf("pm-a"))
	mark := cn.count()
	after := qappText("after-grant")
	tb.append(tb.grantEnv("pm-y", pID, qappParty), tb.narrationEnv(after))
	atY := cn.waitViewpoint(mark, "pm-y")
	cn.waitNarration(atY, after)
	tb.barrier(cn)

	late := tb.logOf("pm-a")[before:]
	logY := tb.logOf("pm-y")
	var lateNarration *vttv1.Envelope
	for _, entry := range late {
		if entry.Env.GetNarrationAdded().GetText() == after {
			lateNarration = entry.Env
		}
	}
	if lateNarration == nil {
		t.Fatalf("fixture: pm-a's log holds no entry of the batch")
	}
	if qappInLog(lateNarration, logY) {
		t.Fatalf("fixture: pm-a's and pm-y's copies of the narration are equal, so the test cannot tell them apart")
	}
	for _, e := range cn.events(mark, cn.count()) {
		for _, entry := range late {
			if proto.Equal(e, entry.Env) {
				t.Fatalf("entry %v of pm-a's log, caused by the grant or later, was sent; frames:\n%s", e, cn.dump())
			}
		}
	}
	for _, e := range cn.events(atY, cn.count()) {
		if !qappInLog(e, logY) {
			t.Fatalf("entry %v sent after the switch is not of pm-y's log", e)
		}
	}
}

// VTT-453
func TestQAPPAMoveIsJudgedByThePartyMemberTheConnectionShows(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	tb.addActor("pm-l", "Lena", qappParty, pID)
	tb.addActor("pm-r", "Rolf", qappParty, pID)
	tb.append(tb.sceneEnv(), tb.tokenEnv("t-l", "pm-l", 1, 1), tb.tokenEnv("t-r", "pm-r", 7, 1))

	left := tb.dial("p-left", pTok, "")
	right := tb.dial("p-right", pTok, "pm-r")
	left.waitViewpoint(0, "pm-l")
	right.waitViewpoint(0, "pm-r")

	left.mustRefuse(left.move("t-r", 7, 2), "a right-room move judged by pm-l")
	right.mustOK(right.move("t-r", 7, 2), "a right-room move judged by pm-r")
	right.mustRefuse(right.move("t-l", 1, 2), "a left-room move judged by pm-r")
	left.mustOK(left.move("t-l", 1, 2), "a left-room move judged by pm-l")

	at := left.count()
	left.mustOK(left.perch("pm-r"), "perch on pm-r")
	left.waitViewpoint(at, "pm-r")
	left.mustOK(left.move("t-r", 7, 1), "a right-room move after perching on pm-r")
	left.mustRefuse(left.move("t-l", 1, 1), "a left-room move after perching on pm-r")
}

// VTT-453 VTT-370 VTT-456
func TestQAPPAPlayerWithoutAPartyMemberMovesNothingWhateverTheyWatch(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	tb.addActor("pm-x", "Xena", qappParty)
	tb.addActor("np-g", "Gobo", qappNonParty, pID)
	tb.append(tb.sceneEnv(), tb.tokenEnv("t-x", "pm-x", 7, 1), tb.tokenEnv("t-g", "np-g", 6, 1))

	cn := tb.dial("p", pTok, "")
	cn.waitViewpoint(0, "")
	cn.mustOK(cn.perch("pm-x"), "perch on pm-x")
	from := cn.waitViewpoint(0, "pm-x") + 1
	cn.mustRefuse(cn.move("t-g", 6, 2), "a move while controlling no party member")

	tb.grant("pm-x", pID, qappParty)
	cn.mustOK(cn.move("t-g", 6, 2), "the same move once pm-x is theirs")

	tb.revoke("pm-x", pID)
	cn.mustRefuse(cn.move("t-g", 6, 0), "a move after pm-x is revoked")
	tb.expectWatching("pm-x", from, cn)
	cn.mustRefuse(cn.move("t-g", 6, 0), "a move after pm-x is revoked, the seat settled")
}

// VTT-453 VTT-340
func TestQAPPAMoveRightAfterControlChangesIsJudgedByTheFirstPartyMember(t *testing.T) {
	t.Run("a grant while watching another's", func(t *testing.T) {
		tb := qappNewTable(t)
		pTok, pID := tb.invite("Pia", identity.RolePlayer)
		tb.addActor("pm-x", "Xena", qappParty)
		tb.addActor("pm-z", "Zack", qappParty)
		tb.addActor("pm-y", "Ylva", qappParty)
		tb.addActor("np-g", "Gobo", qappNonParty, pID)
		tb.addActor("np-h", "Hobb", qappNonParty, pID)
		tb.append(tb.sceneEnv(),
			tb.tokenEnv("t-x", "pm-x", 7, 1), tb.tokenEnv("t-z", "pm-z", 7, 0), tb.tokenEnv("t-y", "pm-y", 1, 1),
			tb.tokenEnv("t-g", "np-g", 6, 1), tb.tokenEnv("t-h", "np-h", 2, 1))

		cn := tb.dial("p", pTok, "")
		cn.mustOK(cn.perch("pm-x"), "perch on pm-x")
		mark := cn.waitViewpoint(0, "pm-x") + 1

		tb.append(tb.grantEnv("pm-z", pID, qappParty), tb.grantEnv("pm-y", pID, qappParty))
		cn.mustRefuse(cn.move("t-g", 6, 2), "a right-room move once pm-z and pm-y are theirs")
		cn.mustOK(cn.move("t-h", 2, 2), "a left-room move once pm-z and pm-y are theirs")
		cn.waitViewpoint(mark, "pm-y")
		if moved := cn.viewpoints(mark, cn.count()); moved[0] != "pm-y" {
			t.Fatalf("the grant moved the seat to %q, want pm-y first", moved)
		}
	})
	t.Run("a revocation of the one shown", func(t *testing.T) {
		tb := qappNewTable(t)
		pTok, pID := tb.invite("Pia", identity.RolePlayer)
		tb.addActor("pm-c", "Cato", qappParty, pID)
		tb.addActor("pm-b", "Bert", qappParty, pID)
		tb.addActor("pm-s", "Sune", qappParty, pID)
		tb.addActor("np-g", "Gobo", qappNonParty, pID)
		tb.addActor("np-h", "Hobb", qappNonParty, pID)
		tb.append(tb.sceneEnv(),
			tb.tokenEnv("t-b", "pm-b", 1, 1), tb.tokenEnv("t-c", "pm-c", 7, 0), tb.tokenEnv("t-s", "pm-s", 7, 1),
			tb.tokenEnv("t-g", "np-g", 6, 1), tb.tokenEnv("t-h", "np-h", 2, 1))

		cn := tb.dial("p", pTok, "pm-s")
		mark := cn.waitViewpoint(0, "pm-s") + 1

		tb.revoke("pm-s", pID)
		cn.mustRefuse(cn.move("t-g", 6, 2), "a right-room move once pm-s is revoked")
		cn.mustOK(cn.move("t-h", 2, 2), "a left-room move once pm-s is revoked")
		cn.waitViewpoint(mark, "pm-b")
		if moved := cn.viewpoints(mark, cn.count()); moved[0] != "pm-b" {
			t.Fatalf("the revocation moved the seat to %q, want pm-b first", moved)
		}
	})
}

// VTT-388 VTT-387 VTT-452 VTT-340
func TestQAPPPromotingASpectatorToAPlayerWithoutAPartyMemberKeepsThePerch(t *testing.T) {
	tb := qappNewTable(t)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	s1Tok, s1ID := tb.invite("Sami", identity.RoleSpectator)
	s2Tok, s2ID := tb.invite("Sven", identity.RoleSpectator)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-c", "Cato", qappParty)
	for i := 0; i < 12; i++ {
		tb.append(tb.narrationEnv(qappText("history")))
	}
	tb.addActor("pm-b", "Bert", qappParty, s2ID)
	dm := tb.dm()

	s1 := tb.dial("s1", s1Tok, "")
	s1.mustOK(s1.perch("pm-a"), "s1 perch on pm-a")
	from1 := s1.waitViewpoint(0, "pm-a") + 1
	s2 := tb.dial("s2", s2Tok, "")
	s2.mustOK(s2.perch("pm-a"), "s2 perch on pm-a")
	from2 := s2.waitViewpoint(0, "pm-a") + 1
	tb.expectWatching("pm-a", from1, s1)
	tb.expectWatching("pm-a", from2, s2)
	watchFrom := s1.count()

	tb.dmDo(dm, qappPromoteCmd(s1ID, identity.RolePlayer), "promote s1")
	tb.expectWatching("pm-a", from1, s1)
	if ws := s1.watchables(watchFrom, s1.count()); len(ws) != 0 {
		t.Fatalf("the promotion re-sent an unchanged set: %v", ws)
	}

	tb.dmDo(dm, qappPromoteCmd(s2ID, identity.RolePlayer), "promote s2")
	text := qappText("after-promotion")
	tb.append(tb.narrationEnv(text))
	idx := s2.waitNarration(from2, text)
	if got, _ := s2.shown(idx); got != "pm-b" {
		t.Fatalf("s2, now a player who controls pm-b, shows %q at the narration, want pm-b; frames:\n%s", got, s2.dump())
	}
	e := s2.snapshot()[idx].GetEvent()
	if !qappInLog(e, tb.logOf("pm-b")) || qappInLog(e, tb.logOf("pm-a")) {
		t.Fatalf("the narration after the promotion is not pm-b's copy: %v", e)
	}
}

// VTT-388
func TestQAPPDemotingAPlayerToSpectatorKeepsThePerch(t *testing.T) {
	tb := qappNewTable(t)
	pTok, pID := tb.invite("Pia", identity.RolePlayer)
	oTok, oID := tb.invite("Olle", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-o", "Otto", qappParty, oID)
	dm := tb.dm()

	p := tb.dial("p", pTok, "")
	p.mustOK(p.perch("pm-a"), "perch on pm-a")
	fromP := p.waitViewpoint(0, "pm-a") + 1
	o := tb.dial("o", oTok, "")
	fromO := o.waitViewpoint(0, "pm-o") + 1

	tb.dmDo(dm, qappPromoteCmd(pID, identity.RoleSpectator), "demote p")
	tb.dmDo(dm, qappPromoteCmd(oID, identity.RoleSpectator), "demote o")
	tb.expectWatching("pm-a", fromP, p)
	tb.expectWatching("pm-o", fromO, o)
}

// VTT-181 VTT-345 VTT-386
func TestQAPPTheSpectatorsPerchIsUnchanged(t *testing.T) {
	tb := qappNewTable(t)
	sTok, sID := tb.invite("Sami", identity.RoleSpectator)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("np-g", "Gobo", qappNonParty, sID)

	cn := tb.dial("s", sTok, "")
	cn.waitViewpoint(0, "")
	cn.mustRefuse(cn.perch("np-g"), "perch on a non-party actor the spectator controls")
	cn.mustOK(cn.perch("pm-a"), "perch on pm-a")
	from := cn.waitViewpoint(0, "pm-a") + 1
	cn.mustOK(cn.perch(""), "perch on nobody")
	from = cn.waitViewpoint(from, "") + 1
	cn.mustOK(cn.perch("pm-a"), "perch on pm-a again")
	from = cn.waitViewpoint(from, "pm-a") + 1
	tb.grant("pm-a", qID, qappNonParty)
	cn.waitViewpoint(from, "")
}

// VTT-452 VTT-340
func TestQAPPAPlayerWatchingAnotherPartyMemberStaysAsItChangesHands(t *testing.T) {
	tb := qappNewTable(t)
	pTok, _ := tb.invite("Pia", identity.RolePlayer)
	_, qID := tb.invite("Quinn", identity.RolePlayer)
	_, rID := tb.invite("Rune", identity.RolePlayer)
	tb.addActor("pm-a", "Alma", qappParty, qID)
	tb.addActor("pm-b", "Bert", qappParty)

	cn := tb.dial("p", pTok, "")
	cn.mustOK(cn.perch("pm-b"), "perch on pm-b")
	from := cn.waitViewpoint(0, "pm-b") + 1
	tb.grant("pm-b", rID, qappParty)
	tb.expectWatching("pm-b", from, cn)
	tb.revoke("pm-b", rID)
	tb.grant("pm-b", qID, qappParty)
	tb.expectWatching("pm-b", from, cn)
}
