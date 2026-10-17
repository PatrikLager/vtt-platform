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

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaSeatWait  = 5 * time.Second
	qaSeatQuiet = 300 * time.Millisecond
	qaSeatPM    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaSeatNP    = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qaSeatTable struct {
	c   *campaign.Campaign
	ids *identity.DB
	srv *httptest.Server
	tok map[identity.Role]string
	pid map[identity.Role]string
	pos map[string]*vttv1.GridPosition
	scn map[string]string
	n   int
}

func qaSeatOpen(t *testing.T, grant bool) *qaSeatTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	tb := &qaSeatTable{
		c: c, ids: ids,
		tok: map[identity.Role]string{}, pid: map[identity.Role]string{},
		pos: map[string]*vttv1.GridPosition{}, scn: map[string]string{},
	}
	for _, r := range []identity.Role{
		identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator,
	} {
		tok, id, err := ids.CreateInvite("qa-"+string(r), r)
		if err != nil {
			t.Fatalf("CreateInvite %s: %v", r, err)
		}
		tb.tok[r], tb.pid[r] = tok, id
	}
	tb.append(t, &vttv1.SessionStarted{Name: "first"})
	tb.append(t, &vttv1.SceneCreated{SceneId: "hall", Name: "hall", GridWidth: 12, GridHeight: 12})
	tb.append(t, &vttv1.SceneCreated{SceneId: "yard", Name: "yard", GridWidth: 12, GridHeight: 12})
	for _, a := range []struct {
		id    string
		kind  vttv1.ActorKind
		scene string
		at    *vttv1.GridPosition
	}{
		{"scout", qaSeatPM, "yard", &vttv1.GridPosition{X: 1, Y: 1}},
		{"ally", qaSeatPM, "yard", &vttv1.GridPosition{X: 5, Y: 5}},
		{"hero", qaSeatPM, "hall", &vttv1.GridPosition{X: 1, Y: 1}},
		{"goblin", qaSeatNP, "hall", &vttv1.GridPosition{X: 3, Y: 3}},
	} {
		tb.append(t, &vttv1.Actor{ActorId: a.id, Name: a.id, Kind: a.kind})
		tok := "t-" + a.id
		tb.append(t, &vttv1.TokenPlaced{TokenId: tok, SceneId: a.scene, ActorId: a.id, Position: a.at})
		tb.pos[tok], tb.scn[tok] = a.at, a.scene
	}
	if grant {
		tb.append(t, tb.grant("scout", qaSeatPM))
		tb.append(t, tb.grant("hero", qaSeatPM))
		tb.append(t, tb.grant("goblin", qaSeatNP))
	}
	for i := 0; i < 2; i++ {
		tb.append(t, tb.move("t-hero"))
		tb.append(t, tb.move("t-scout"))
	}
	tb.srv = httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(tb.srv.Close)
	return tb
}

func (tb *qaSeatTable) grant(actor string, kind vttv1.ActorKind) *vttv1.ActorControlGranted {
	return &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: tb.pid[identity.RolePlayer], Kind: kind,
	}
}

func (tb *qaSeatTable) move(tok string) *vttv1.TokenMoved {
	from := tb.pos[tok]
	to := &vttv1.GridPosition{X: from.X%10 + 1, Y: from.Y}
	tb.pos[tok] = to
	return &vttv1.TokenMoved{TokenId: tok, SceneId: tb.scn[tok], From: from, To: to}
}

func (tb *qaSeatTable) env(p proto.Message) *vttv1.Envelope {
	tb.n++
	e := &vttv1.Envelope{
		EventId:       fmt.Sprintf("qa-seat-%d", tb.n),
		ParticipantId: tb.pid[identity.RoleDM],
		ActorRole:     string(identity.RoleDM),
	}
	switch v := p.(type) {
	case *vttv1.SessionStarted:
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: v}
	case *vttv1.SceneCreated:
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: v}
	case *vttv1.Actor:
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: v}}
	case *vttv1.TokenPlaced:
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: v}
	case *vttv1.ActorControlGranted:
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: v}
	case *vttv1.ActorControlRevoked:
		e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: v}
	case *vttv1.TokenMoved:
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: v}
	case *vttv1.NoteUpserted:
		e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: v}
	case *vttv1.NarrationAdded:
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: v}
	default:
		panic(fmt.Sprintf("qaSeat: no envelope for %T", p))
	}
	return e
}

func (tb *qaSeatTable) append(t *testing.T, ps ...proto.Message) {
	t.Helper()
	envs := make([]*vttv1.Envelope, len(ps))
	for i, p := range ps {
		envs[i] = tb.env(p)
	}
	var err error
	if len(envs) == 1 {
		_, err = tb.c.Append(envs[0])
	} else {
		_, err = tb.c.AppendBatch(envs)
	}
	if err != nil {
		t.Fatalf("append %d envelopes: %v", len(envs), err)
	}
}

func (tb *qaSeatTable) world(t *testing.T, after int64) ([]*vttv1.Envelope, int64) {
	t.Helper()
	ch, unsub, head, err := tb.c.Subscribe(after, 4096)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	var out []*vttv1.Envelope
	deadline := time.After(qaSeatWait)
	for len(out) < int(head-after) {
		select {
		case e := <-ch:
			out = append(out, e)
		case <-deadline:
			t.Fatalf("world catch-up stalled at %d of %d", len(out), head-after)
		}
	}
	return out, head
}

func (tb *qaSeatTable) head(t *testing.T) int64 {
	t.Helper()
	_, h := tb.world(t, 0)
	return h
}

func (tb *qaSeatTable) log(t *testing.T, actor string) []*vttv1.Envelope {
	t.Helper()
	return tb.caused(t, actor, -1)
}

func (tb *qaSeatTable) caused(t *testing.T, actor string, fromWorld int64) []*vttv1.Envelope {
	t.Helper()
	entries, err := tb.c.CharacterLog(actor)
	if err != nil {
		t.Fatalf("CharacterLog %s: %v", actor, err)
	}
	var out []*vttv1.Envelope
	for _, e := range entries {
		if fromWorld < 0 || e.WorldSeq >= fromWorld {
			out = append(out, e.Env)
		}
	}
	return out
}

type qaSeatConn struct {
	ws   *websocket.Conn
	in   chan *vttv1.ServerFrame
	err  error
	done bool
	got  []*vttv1.ServerFrame
	req  int
}

func (tb *qaSeatTable) dial(t *testing.T, r identity.Role, query string) *qaSeatConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), qaSeatWait)
	defer cancel()
	u := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" +
		url.QueryEscape(tb.tok[r]) + "&" + query
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial %s %q: %v", r, query, err)
	}
	ws.SetReadLimit(512 << 10)
	q := &qaSeatConn{ws: ws, in: make(chan *vttv1.ServerFrame, 8192)}
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		for {
			_, b, err := ws.Read(context.Background())
			if err == nil {
				f := &vttv1.ServerFrame{}
				if err = protojson.Unmarshal(b, f); err == nil {
					q.in <- f
					continue
				}
			}
			q.err = err
			close(q.in)
			return
		}
	}()
	t.Cleanup(func() {
		_ = ws.CloseNow()
		select {
		case <-ended:
		case <-time.After(qaSeatWait):
		}
	})
	return q
}

func (q *qaSeatConn) pull(d time.Duration) {
	if q.done {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case f, ok := <-q.in:
		if !ok {
			q.done = true
			return
		}
		q.got = append(q.got, f)
	case <-timer.C:
	}
}

func (q *qaSeatConn) settle(d time.Duration) {
	deadline := time.Now().Add(d)
	for !q.done && time.Now().Before(deadline) {
		q.pull(time.Until(deadline))
	}
}

func (q *qaSeatConn) await(t *testing.T, from int, what string, pred func(*vttv1.ServerFrame) bool) int {
	t.Helper()
	deadline := time.Now().Add(qaSeatWait)
	for i := from; ; i++ {
		for i >= len(q.got) {
			left := time.Until(deadline)
			if q.done || left <= 0 {
				t.Fatalf("no %s from frame %d (closed %v): %s", what, from, q.done, q.describe())
			}
			q.pull(left)
		}
		if pred(q.got[i]) {
			return i
		}
	}
}

func (q *qaSeatConn) describe() string {
	var b strings.Builder
	for i, f := range q.got {
		fmt.Fprintf(&b, "\n  %d: %s", i, qaSeatJSON(f))
	}
	if q.done {
		fmt.Fprintf(&b, "\n  closed: %v", q.err)
	}
	return b.String()
}

func qaSeatJSON(m proto.Message) string {
	b, _ := protojson.Marshal(m)
	if len(b) > 240 {
		return string(b[:240]) + "..."
	}
	return string(b)
}

func qaSeatShown(f *vttv1.ServerFrame) bool {
	return f.GetEvent() != nil || f.GetViewpointChanged() != nil
}

func qaSeatIsView(f *vttv1.ServerFrame) bool { return f.GetViewpointChanged() != nil }

func (q *qaSeatConn) opening(t *testing.T) (int64, string, int) {
	t.Helper()
	i := q.await(t, 0, "opening ViewpointChanged", qaSeatShown)
	if q.got[i].GetViewpointChanged() == nil {
		t.Fatalf("an entry before the opening ViewpointChanged: %s", q.describe())
	}
	if i < 2 || q.got[0].GetCatchUpHead() == nil || q.got[1].GetPresenceSnapshot() == nil {
		t.Fatalf("opening is not CatchUpHead, PresenceSnapshot: %s", q.describe())
	}
	for _, f := range q.got[2:i] {
		if f.GetCatchUpHead() != nil || f.GetPresenceSnapshot() != nil {
			t.Fatalf("a second opening frame: %s", q.describe())
		}
	}
	return q.got[0].GetCatchUpHead().GetHeadSequence(), q.got[i].GetViewpointChanged().GetActorId(), i + 1
}

func (q *qaSeatConn) expect(t *testing.T, from int, what string, want []*vttv1.Envelope) int {
	t.Helper()
	for k, w := range want {
		i := q.await(t, from, fmt.Sprintf("%s entry %d of %d", what, k+1, len(want)), qaSeatShown)
		got := q.got[i].GetEvent()
		if got == nil {
			t.Fatalf("%s: ViewpointChanged where entry %d (%s) was due: %s",
				what, k+1, qaSeatJSON(w), q.describe())
		}
		if !proto.Equal(got, w) {
			t.Fatalf("%s: frame %d is %s, want entry %d %s", what, i, qaSeatJSON(got), k+1, qaSeatJSON(w))
		}
		from = i + 1
	}
	return from
}

func (q *qaSeatConn) view(t *testing.T, from int, want string) int {
	t.Helper()
	i := q.await(t, from, fmt.Sprintf("ViewpointChanged %q", want), qaSeatShown)
	vc := q.got[i].GetViewpointChanged()
	if vc == nil {
		t.Fatalf("entry %s where ViewpointChanged %q was due: %s", qaSeatJSON(q.got[i]), want, q.describe())
	}
	if vc.GetActorId() != want {
		t.Fatalf("ViewpointChanged %q, want %q: %s", vc.GetActorId(), want, q.describe())
	}
	return i + 1
}

func (q *qaSeatConn) nothingShown(t *testing.T, from int, why string) {
	t.Helper()
	q.settle(qaSeatQuiet)
	for i := from; i < len(q.got); i++ {
		if qaSeatShown(q.got[i]) {
			t.Fatalf("%s: frame %d %s: %s", why, i, qaSeatJSON(q.got[i]), q.describe())
		}
	}
}

func (q *qaSeatConn) send(t *testing.T, cmd *vttv1.ClientCommand) string {
	t.Helper()
	q.req++
	cmd.RequestId = fmt.Sprintf("qa-r%d", q.req)
	b, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaSeatWait)
	defer cancel()
	if err := q.ws.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("write: %v", err)
	}
	return cmd.RequestId
}

func (q *qaSeatConn) result(t *testing.T, id string) *vttv1.CommandResult {
	t.Helper()
	i := q.await(t, 0, "result "+id, func(f *vttv1.ServerFrame) bool {
		return f.GetResult().GetRequestId() == id
	})
	return q.got[i].GetResult()
}

func qaSeatPerch(actor string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{
		Command: &vttv1.ClientCommand_SetViewpoint{SetViewpoint: &vttv1.SetViewpoint{ActorId: actor}},
	}
}

func (q *qaSeatConn) perch(t *testing.T, actor string) *vttv1.CommandResult {
	t.Helper()
	return q.result(t, q.send(t, qaSeatPerch(actor)))
}

func (q *qaSeatConn) closeStatus(t *testing.T) websocket.StatusCode {
	t.Helper()
	deadline := time.Now().Add(qaSeatWait)
	for !q.done && time.Now().Before(deadline) {
		q.pull(time.Until(deadline))
	}
	if !q.done {
		t.Fatalf("connection still open after %v: %s", qaSeatWait, q.describe())
	}
	return websocket.CloseStatus(q.err)
}

func qaSeatMinLen(t *testing.T, what string, log []*vttv1.Envelope, n int) {
	t.Helper()
	if len(log) < n {
		t.Fatalf("fixture: %s holds %d entries, the test needs %d", what, len(log), n)
	}
}

// VTT-340 VTT-338 VTT-368 VTT-349 VTT-086
// VTT-378
func TestQASeatAPlayerOpensOnTheirFirstPartyMemberByID(t *testing.T) {
	tb := qaSeatOpen(t, true)
	hero := tb.log(t, "hero")
	qaSeatMinLen(t, "hero", hero, 3)
	q := tb.dial(t, identity.RolePlayer, "after=0")
	head, view, next := q.opening(t)
	if view != "hero" {
		t.Fatalf("opened on %q, want hero", view)
	}
	if head != int64(len(hero)) {
		t.Fatalf("CatchUpHead %d, want hero's head %d", head, len(hero))
	}
	next = q.expect(t, next, "hero history", hero)
	if got := q.got[next-1].GetEvent().GetSequence(); got != head {
		t.Fatalf("last history entry has sequence %d, CatchUpHead said %d", got, head)
	}
	tb.append(t, tb.move("t-scout"))
	tb.append(t, &vttv1.NoteUpserted{
		Key: "vault", Title: "vault", Text: "sealed",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
	})
	if n := len(tb.log(t, "hero")); n != len(hero) {
		t.Fatalf("fixture: the scout's move and a secret note wrote %d hero entries", n-len(hero))
	}
	tb.append(t, tb.move("t-hero"))
	live := tb.log(t, "hero")[len(hero):]
	qaSeatMinLen(t, "hero live", live, 1)
	next = q.expect(t, next, "hero live", live)
	q.nothingShown(t, next, "after the live entries")
}

// VTT-340 VTT-349
func TestQASeatAPlayerIgnoresTheCursorWithoutAWatchableLog(t *testing.T) {
	tb := qaSeatOpen(t, true)
	hero := tb.log(t, "hero")
	qaSeatMinLen(t, "hero", hero, 6)
	for _, query := range []string{
		"after=5", "after=5&log=", "after=5&log=ally", "after=5&log=goblin", "after=5&log=nosuch",
	} {
		t.Run(query, func(t *testing.T) {
			q := tb.dial(t, identity.RolePlayer, query)
			head, view, next := q.opening(t)
			if view != "hero" {
				t.Fatalf("opened on %q, want hero", view)
			}
			if head != int64(len(hero)) {
				t.Fatalf("CatchUpHead %d, want %d", head, len(hero))
			}
			q.expect(t, next, "hero from its first entry", hero)
		})
	}
}

// VTT-339 VTT-349 VTT-338 VTT-086
func TestQASeatANamedLogOpensAfterItsCursor(t *testing.T) {
	tb := qaSeatOpen(t, true)
	scout, hero := tb.log(t, "scout"), tb.log(t, "hero")
	qaSeatMinLen(t, "scout", scout, 6)
	qaSeatMinLen(t, "hero", hero, 7)
	for _, c := range []struct {
		role  identity.Role
		query string
		actor string
		want  []*vttv1.Envelope
	}{
		{identity.RolePlayer, "after=4&log=scout", "scout", scout[4:]},
		{identity.RolePlayer, "after=0&log=scout", "scout", scout},
		{identity.RolePlayer, "after=3&log=hero", "hero", hero[3:]},
		{identity.RoleSpectator, "after=5&log=hero", "hero", hero[5:]},
		{identity.RoleSpectator, "after=2&log=ally", "ally", tb.log(t, "ally")[2:]},
	} {
		t.Run(string(c.role)+" "+c.query, func(t *testing.T) {
			q := tb.dial(t, c.role, c.query)
			head, view, next := q.opening(t)
			if view != c.actor {
				t.Fatalf("opened on %q, want %s", view, c.actor)
			}
			full := tb.log(t, c.actor)
			if head != int64(len(full)) {
				t.Fatalf("CatchUpHead %d, want %d", head, len(full))
			}
			next = q.expect(t, next, c.actor+" after the cursor", c.want)
			q.nothingShown(t, next, "after the history")
		})
	}
}

// VTT-349 VTT-086
func TestQASeatACursorAtTheHeadSendsNoHistory(t *testing.T) {
	tb := qaSeatOpen(t, true)
	scout := tb.log(t, "scout")
	q := tb.dial(t, identity.RolePlayer, fmt.Sprintf("after=%d&log=scout", len(scout)))
	head, view, next := q.opening(t)
	if view != "scout" || head != int64(len(scout)) {
		t.Fatalf("opened on %q at head %d, want scout at %d", view, head, len(scout))
	}
	q.nothingShown(t, next, "a cursor at the head")
	tb.append(t, tb.move("t-scout"))
	q.expect(t, next, "scout live", tb.log(t, "scout")[len(scout):])
}

// VTT-341 VTT-180 VTT-369 VTT-342
// VTT-374
func TestQASeatASpectatorWatchesNobodyUntilItPerches(t *testing.T) {
	tb := qaSeatOpen(t, true)
	for _, query := range []string{
		"after=0", "after=3", "after=3&log=goblin", "after=0&log=nosuch", "after=0&log=",
	} {
		t.Run(query, func(t *testing.T) {
			q := tb.dial(t, identity.RoleSpectator, query)
			head, view, next := q.opening(t)
			if view != "" || head != 0 {
				t.Fatalf("opened on %q at head %d, want nobody at 0", view, head)
			}
			before := len(tb.log(t, "hero"))
			tb.append(t, tb.move("t-hero"))
			tb.append(t, tb.move("t-scout"))
			tb.append(t, &vttv1.NarrationAdded{Text: "dusk"})
			tb.append(t, &vttv1.NoteUpserted{
				Key: "gate", Title: "gate", Text: "open",
				Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
			})
			if len(tb.log(t, "hero")) == before {
				t.Fatalf("fixture: the appends wrote no hero entry")
			}
			q.nothingShown(t, next, "watching nobody")
			if r := q.perch(t, "hero"); !r.GetOk() {
				t.Fatalf("perch on hero refused: %s", r.GetError())
			}
			next = q.view(t, next, "hero")
			q.expect(t, next, "hero from its first entry", tb.log(t, "hero"))
		})
	}
}

// VTT-340 VTT-341 VTT-342 VTT-338
// VTT-374
func TestQASeatAPlayerWithNoPartyMemberWatchesNobodyUntilGranted(t *testing.T) {
	tb := qaSeatOpen(t, false)
	tb.append(t, tb.grant("goblin", qaSeatNP))
	q := tb.dial(t, identity.RolePlayer, "after=0&log=goblin")
	head, view, next := q.opening(t)
	if view != "" || head != 0 {
		t.Fatalf("opened on %q at head %d, want nobody at 0", view, head)
	}
	tb.append(t, tb.move("t-hero"))
	tb.append(t, tb.move("t-goblin"))
	q.nothingShown(t, next, "a player controlling no party member")
	tb.append(t, tb.grant("scout", qaSeatPM), tb.grant("hero", qaSeatPM))
	next = q.view(t, next, "hero")
	hero := tb.log(t, "hero")
	next = q.expect(t, next, "hero from its first entry", hero)
	tb.append(t, tb.move("t-scout"))
	tb.append(t, tb.move("t-hero"))
	q.expect(t, next, "hero live", tb.log(t, "hero")[len(hero):])
}

// VTT-342 VTT-343 VTT-367 VTT-184
// VTT-375
func TestQASeatSetViewpointSwitchesToTheNewLogFromItsFirstEntry(t *testing.T) {
	tb := qaSeatOpen(t, true)
	q := tb.dial(t, identity.RolePlayer, "after=3&log=hero")
	_, _, next := q.opening(t)
	next = q.expect(t, next, "hero after the cursor", tb.log(t, "hero")[3:])
	worldHead := tb.head(t)
	if r := q.perch(t, "scout"); !r.GetOk() {
		t.Fatalf("perch on scout refused: %s", r.GetError())
	}
	if h := tb.head(t); h != worldHead {
		t.Fatalf("set_viewpoint moved the world head from %d to %d", worldHead, h)
	}
	next = q.view(t, next, "scout")
	scout := tb.log(t, "scout")
	next = q.expect(t, next, "scout from its first entry", scout)
	heroBefore := len(tb.log(t, "hero"))
	tb.append(t, tb.move("t-hero"))
	if len(tb.log(t, "hero")) == heroBefore {
		t.Fatalf("fixture: the hero's move wrote no hero entry")
	}
	if len(tb.log(t, "scout")) != len(scout) {
		t.Fatalf("fixture: the hero's move wrote a scout entry")
	}
	tb.append(t, tb.move("t-scout"))
	next = q.expect(t, next, "scout live", tb.log(t, "scout")[len(scout):])
	if r := q.perch(t, "hero"); !r.GetOk() {
		t.Fatalf("perch back on hero refused: %s", r.GetError())
	}
	next = q.view(t, next, "hero")
	hero := tb.log(t, "hero")
	next = q.expect(t, next, "hero from its first entry", hero)
	q.perch(t, "hero")
	tb.append(t, tb.move("t-hero"))
	q.expect(t, next, "hero live after a perch on what is shown", tb.log(t, "hero")[len(hero):])
}

func (q *qaSeatConn) follow(t *testing.T, tb *qaSeatTable, next int, actor string, seen *int) int {
	t.Helper()
	cur := tb.log(t, actor)
	next = q.expect(t, next, actor+" still watched", cur[*seen:])
	*seen = len(cur)
	return next
}

func (q *qaSeatConn) refused(t *testing.T, actor, want string) string {
	t.Helper()
	r := q.perch(t, actor)
	if r.GetOk() || r.GetError() == "" {
		t.Fatalf("perch on %q: ok %v error %q, want a refusal", actor, r.GetOk(), r.GetError())
	}
	if want != "" && r.GetError() != want {
		t.Fatalf("perch on %q refused with %q, earlier with %q", actor, r.GetError(), want)
	}
	return r.GetError()
}

// VTT-343 VTT-346 VTT-347 VTT-193
func TestQASeatARefusedPlayerPerchLeavesTheWatcherWhereItWas(t *testing.T) {
	tb := qaSeatOpen(t, true)
	q := tb.dial(t, identity.RolePlayer, "after=0")
	_, _, next := q.opening(t)
	seen := 0
	next = q.follow(t, tb, next, "hero", &seen)
	worldHead := tb.head(t)
	for _, a := range []string{"goblin", "ally", "nosuch"} {
		t.Logf("refusal for %s: %q", a, q.refused(t, a, ""))
	}
	if r := q.perch(t, ""); r.GetOk() {
		t.Fatalf("a player's perch on nobody was accepted")
	}
	if h := tb.head(t); h != worldHead {
		t.Fatalf("refused perches moved the world head from %d to %d", worldHead, h)
	}
	player := tb.pid[identity.RolePlayer]
	refusal := q.refused(t, "squire", "")
	tb.append(t, &vttv1.Actor{ActorId: "squire", Name: "squire", Kind: qaSeatNP})
	next = q.follow(t, tb, next, "hero", &seen)
	q.refused(t, "squire", refusal)
	tb.append(t, &vttv1.ActorControlGranted{ActorId: "squire", ParticipantId: player, Kind: qaSeatNP})
	next = q.follow(t, tb, next, "hero", &seen)
	q.refused(t, "squire", refusal)
	tb.append(t, &vttv1.ActorControlRevoked{ActorId: "squire", ParticipantId: player},
		&vttv1.ActorControlGranted{
			ActorId: "squire", ParticipantId: tb.pid[identity.RoleSpectator], Kind: qaSeatPM,
		})
	next = q.follow(t, tb, next, "hero", &seen)
	q.refused(t, "squire", refusal)
	tb.append(t, tb.move("t-hero"))
	q.follow(t, tb, next, "hero", &seen)
}

// VTT-181 VTT-182 VTT-345 VTT-342 VTT-193 VTT-341
// VTT-375
func TestQASeatASpectatorPerchesOnlyOnAPartyMember(t *testing.T) {
	tb := qaSeatOpen(t, true)
	q := tb.dial(t, identity.RoleSpectator, "after=0&log=hero")
	_, view, next := q.opening(t)
	if view != "hero" {
		t.Fatalf("opened on %q, want hero", view)
	}
	seen := 0
	next = q.follow(t, tb, next, "hero", &seen)
	t.Logf("refusal for goblin: %q", q.refused(t, "goblin", ""))
	refusal := q.refused(t, "squire", "")
	tb.append(t, &vttv1.Actor{ActorId: "squire", Name: "squire", Kind: qaSeatNP})
	next = q.follow(t, tb, next, "hero", &seen)
	q.refused(t, "squire", refusal)
	tb.append(t, tb.grant("squire", qaSeatNP))
	next = q.follow(t, tb, next, "hero", &seen)
	q.refused(t, "squire", refusal)
	tb.append(t, tb.move("t-hero"))
	next = q.follow(t, tb, next, "hero", &seen)
	if r := q.perch(t, "ally"); !r.GetOk() {
		t.Fatalf("perch on ally refused: %s", r.GetError())
	}
	next = q.view(t, next, "ally")
	next = q.expect(t, next, "ally from its first entry", tb.log(t, "ally"))
	if r := q.perch(t, ""); !r.GetOk() {
		t.Fatalf("a spectator's perch on nobody refused: %s", r.GetError())
	}
	next = q.view(t, next, "")
	tb.append(t, tb.move("t-ally"))
	tb.append(t, tb.move("t-hero"))
	q.nothingShown(t, next, "after a perch on nobody")
	q.perch(t, "")
	q.perch(t, "scout")
	next = q.view(t, next, "scout")
	q.expect(t, next, "scout from its first entry", tb.log(t, "scout"))
}

// VTT-348 VTT-340 VTT-342 VTT-367
func TestQASeatLosingControlSwitchesAPlayerToTheNextPartyMember(t *testing.T) {
	tb := qaSeatOpen(t, true)
	onHero := tb.dial(t, identity.RolePlayer, "after=0")
	_, view, nh := onHero.opening(t)
	if view != "hero" {
		t.Fatalf("default opened on %q, want hero", view)
	}
	nh = onHero.expect(t, nh, "hero history", tb.log(t, "hero"))
	onScout := tb.dial(t, identity.RolePlayer, "after=0&log=scout")
	_, _, ns := onScout.opening(t)
	ns = onScout.expect(t, ns, "scout history", tb.log(t, "scout"))
	revokeSeq := tb.head(t) + 1
	tb.append(t, &vttv1.ActorControlRevoked{ActorId: "hero", ParticipantId: tb.pid[identity.RolePlayer]},
		tb.move("t-hero"))
	tb.append(t, tb.move("t-hero"))
	leaked := tb.caused(t, "hero", revokeSeq)
	qaSeatMinLen(t, "hero entries caused by the revocation or later", leaked, 2)
	nh = onHero.view(t, nh, "scout")
	scout := tb.log(t, "scout")
	nh = onHero.expect(t, nh, "scout from its first entry", scout)
	tb.append(t, tb.move("t-scout"))
	live := tb.log(t, "scout")[len(scout):]
	onHero.expect(t, nh, "scout live", live)
	onScout.expect(t, ns, "scout live on a watched log", live)
	for _, q := range []*qaSeatConn{onHero, onScout} {
		q.settle(qaSeatQuiet)
		for i, f := range q.got {
			for _, l := range leaked {
				if proto.Equal(f.GetEvent(), l) {
					t.Fatalf("frame %d is a hero entry caused at or after the revocation: %s", i, qaSeatJSON(l))
				}
			}
		}
	}
	for i, f := range onScout.got[ns:] {
		if qaSeatIsView(f) {
			t.Fatalf("the scout watcher switched at frame %d: %s", ns+i, onScout.describe())
		}
	}
}

// VTT-342 VTT-348 VTT-340 VTT-341
func TestQASeatAShoulderThatStopsBeingAPartyMemberSwitchesItsWatchers(t *testing.T) {
	tb := qaSeatOpen(t, true)
	spec := tb.dial(t, identity.RoleSpectator, "after=0&log=hero")
	_, _, ns := spec.opening(t)
	ns = spec.expect(t, ns, "hero history", tb.log(t, "hero"))
	play := tb.dial(t, identity.RolePlayer, "after=0")
	_, _, np := play.opening(t)
	np = play.expect(t, np, "hero history", tb.log(t, "hero"))
	tb.append(t, tb.grant("hero", qaSeatNP), tb.move("t-hero"))
	ns = spec.view(t, ns, "")
	np = play.view(t, np, "scout")
	np = play.expect(t, np, "scout from its first entry", tb.log(t, "scout"))
	tb.append(t, tb.move("t-hero"))
	spec.nothingShown(t, ns, "a spectator whose shoulder left the party")
	play.nothingShown(t, np, "a player whose default left the party")
	spec.perch(t, "scout")
	ns = spec.view(t, ns, "scout")
	spec.expect(t, ns, "scout from its first entry", tb.log(t, "scout"))
}

// VTT-352
// VTT-376
func TestQASeatARevokedConnectionClosesAtTheNextAppend(t *testing.T) {
	for _, c := range []struct {
		name  string
		role  identity.Role
		query string
		entry bool
	}{
		{"player, no entry", identity.RolePlayer, "after=0", false},
		{"player, an entry", identity.RolePlayer, "after=0", true},
		{"spectator on hero, no entry", identity.RoleSpectator, "after=0&log=hero", false},
		{"spectator on nobody", identity.RoleSpectator, "after=0", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			tb := qaSeatOpen(t, true)
			q := tb.dial(t, c.role, c.query)
			_, view, next := q.opening(t)
			if view == "hero" {
				q.expect(t, next, "hero history", tb.log(t, "hero"))
			}
			if err := tb.ids.Revoke(tb.pid[c.role]); err != nil {
				t.Fatalf("Revoke: %v", err)
			}
			before := len(tb.log(t, "hero"))
			if c.entry {
				tb.append(t, tb.move("t-hero"))
			} else {
				tb.append(t, &vttv1.NoteUpserted{
					Key: "vault", Title: "vault", Text: "sealed",
					Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET,
				})
			}
			if wrote := len(tb.log(t, "hero")) > before; wrote != c.entry {
				t.Fatalf("fixture: the append wrote a hero entry: %v, want %v", wrote, c.entry)
			}
			if got := q.closeStatus(t); got != websocket.StatusPolicyViolation {
				t.Fatalf("closed with %v, want %v", got, websocket.StatusPolicyViolation)
			}
		})
	}
}

// VTT-176 VTT-140
// VTT-377
func TestQASeatTheDMAndTheAgentAreSentTheWorldAndIgnoreLog(t *testing.T) {
	tb := qaSeatOpen(t, true)
	for _, c := range []struct {
		role  identity.Role
		after int64
	}{{identity.RoleDM, 5}, {identity.RoleAgent, 0}} {
		t.Run(string(c.role), func(t *testing.T) {
			q := tb.dial(t, c.role, fmt.Sprintf("after=%d&log=hero", c.after))
			world, head := tb.world(t, c.after)
			i := q.await(t, 0, "CatchUpHead", func(f *vttv1.ServerFrame) bool { return true })
			if q.got[i].GetCatchUpHead().GetHeadSequence() != head {
				t.Fatalf("first frame %s, want CatchUpHead %d", qaSeatJSON(q.got[i]), head)
			}
			next := 1
			for k, w := range world {
				i := q.await(t, next, fmt.Sprintf("world event %d", w.GetSequence()), func(f *vttv1.ServerFrame) bool {
					return f.GetEvent() != nil
				})
				if !proto.Equal(q.got[i].GetEvent(), w) {
					t.Fatalf("event %d is %s, want %s", k, qaSeatJSON(q.got[i]), qaSeatJSON(w))
				}
				next = i + 1
			}
			r := q.perch(t, "hero")
			if r.GetOk() {
				t.Fatalf("a %s set_viewpoint was accepted", c.role)
			}
		})
	}
}

func qaSeatCommands(player string) []*vttv1.ClientCommand {
	pos := &vttv1.GridPosition{X: 6, Y: 6}
	return []*vttv1.ClientCommand{
		{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{
			ActorId: "squire", Name: "squire", Kind: qaSeatPM, ControllerIds: []string{player},
		}}}},
		{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "t-hero-2", SceneId: "hall", ActorId: "hero", Position: pos,
		}}},
		{Command: &vttv1.ClientCommand_RemoveToken{RemoveToken: &vttv1.RemoveToken{TokenId: "t-hero"}}},
		{Command: &vttv1.ClientCommand_RemoveActor{RemoveActor: &vttv1.RemoveActor{ActorId: "hero"}}},
		{Command: &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "second"}}},
		{Command: &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}}},
		{Command: &vttv1.ClientCommand_UpsertNote{UpsertNote: &vttv1.UpsertNote{
			Key: "gate", Title: "gate", Text: "open",
			Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		}}},
		{Command: &vttv1.ClientCommand_DeleteNote{DeleteNote: &vttv1.DeleteNote{Key: "gate"}}},
		{Command: &vttv1.ClientCommand_LoadAdventure{LoadAdventure: &vttv1.LoadAdventure{AdventureId: "keep"}}},
		{Command: &vttv1.ClientCommand_LoadMap{LoadMap: &vttv1.LoadMap{MapId: "hall"}}},
		{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: "hero", ParticipantId: player, Kind: qaSeatPM,
		}}},
	}
}

// VTT-344 VTT-140
func TestQASeatAPlayerIssuesNoneOfTheTableCommands(t *testing.T) {
	tb := qaSeatOpen(t, true)
	tb.append(t, &vttv1.NoteUpserted{
		Key: "gate", Title: "gate", Text: "shut",
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
	})
	who := map[identity.Role]*identity.Participant{}
	for _, r := range []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer} {
		p, err := tb.ids.Lookup(tb.pid[r])
		if err != nil {
			t.Fatalf("Lookup %s: %v", r, err)
		}
		who[r] = p
	}
	st := tb.c.State()
	for _, cmd := range qaSeatCommands(tb.pid[identity.RolePlayer]) {
		name := fmt.Sprintf("%T", cmd.GetCommand())
		if err := gateway.Authorize(who[identity.RolePlayer], cmd, st); err == nil {
			t.Errorf("player authorized for %s", name)
		}
		for _, r := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
			if err := gateway.Authorize(who[r], cmd, st); err != nil {
				t.Errorf("%s refused %s: %v", r, name, err)
			}
		}
	}
	for _, r := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		if gateway.Authorize(who[r], qaSeatPerch("hero"), st) == nil {
			t.Errorf("%s authorized for set_viewpoint", r)
		}
	}
	worldHead := tb.head(t)
	q := tb.dial(t, identity.RolePlayer, "after=0")
	q.opening(t)
	for _, cmd := range qaSeatCommands(tb.pid[identity.RolePlayer]) {
		name := fmt.Sprintf("%T", cmd.GetCommand())
		if r := q.result(t, q.send(t, cmd)); r.GetOk() {
			t.Errorf("player's %s accepted", name)
		}
	}
	if h := tb.head(t); h != worldHead {
		t.Fatalf("the player's commands moved the world head from %d to %d", worldHead, h)
	}
}

// VTT-188 VTT-306 VTT-342 VTT-367
func TestQASeatTheLastOfABurstOfPerchesWins(t *testing.T) {
	tb := qaSeatOpen(t, true)
	q := tb.dial(t, identity.RoleSpectator, "after=0")
	_, _, next := q.opening(t)
	burst := []string{"hero", "scout", "ally", "", "hero", "ally", "scout"}
	var ids []string
	for _, a := range burst {
		ids = append(ids, q.send(t, qaSeatPerch(a)))
	}
	for k, id := range ids {
		if r := q.result(t, id); !r.GetOk() {
			t.Fatalf("perch %d on %q refused: %s", k, burst[k], r.GetError())
		}
	}
	logs := map[string][]*vttv1.Envelope{"": nil}
	for _, a := range []string{"hero", "scout", "ally"} {
		logs[a] = tb.log(t, a)
	}
	final := burst[len(burst)-1]
	walk := func() (string, int) {
		last, seg := "", 0
		for i := next; i < len(q.got); i++ {
			f := q.got[i]
			if vc := f.GetViewpointChanged(); vc != nil {
				last, seg = vc.GetActorId(), 0
				continue
			}
			if e := f.GetEvent(); e != nil {
				if seg >= len(logs[last]) || !proto.Equal(e, logs[last][seg]) {
					t.Fatalf("frame %d is not entry %d of %q's log: %s", i, seg+1, last, q.describe())
				}
				seg++
			}
		}
		return last, seg
	}
	deadline := time.Now().Add(qaSeatWait)
	for last, seg := walk(); last != final || seg != len(logs[final]); last, seg = walk() {
		if q.done || time.Now().After(deadline) {
			t.Fatalf("settled on %q after %d entries, want %q after %d: %s",
				last, seg, final, len(logs[final]), q.describe())
		}
		q.pull(qaSeatQuiet)
	}
	q.settle(qaSeatQuiet)
	if last, seg := walk(); last != final || seg != len(logs[final]) {
		t.Fatalf("moved on to %q after %d entries, want %q after %d", last, seg, final, len(logs[final]))
	}
	var views []string
	for _, f := range q.got[next:] {
		if qaSeatIsView(f) {
			views = append(views, f.GetViewpointChanged().GetActorId())
		}
	}
	t.Logf("burst %q announced %q", burst, views)
}
