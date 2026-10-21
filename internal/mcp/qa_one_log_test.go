package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/mcp"
)

const qaMOLWait = 5 * time.Second

type qaMOLConn struct {
	query url.Values
	ws    *websocket.Conn
}

type qaMOLGateway struct {
	srv      *httptest.Server
	dials    chan *qaMOLConn
	mu       sync.Mutex
	all      []*qaMOLConn
	refusing bool
	refused  []url.Values
}

func qaMOLNewGateway() *qaMOLGateway {
	g := &qaMOLGateway{dials: make(chan *qaMOLConn, 32)}
	g.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		refusing := g.refusing
		if refusing {
			g.refused = append(g.refused, r.URL.Query())
		}
		g.mu.Unlock()
		if refusing {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		c := &qaMOLConn{query: r.URL.Query(), ws: ws}
		g.mu.Lock()
		g.all = append(g.all, c)
		g.mu.Unlock()
		g.dials <- c
		for {
			if _, _, err := ws.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	return g
}

func (g *qaMOLGateway) next(t *testing.T) *qaMOLConn {
	t.Helper()
	select {
	case c := <-g.dials:
		return c
	case <-time.After(qaMOLWait):
		t.Fatalf("no dial reached the gateway within %v", qaMOLWait)
		return nil
	}
}

func (g *qaMOLGateway) refuse(on bool) {
	g.mu.Lock()
	g.refusing = on
	g.mu.Unlock()
}

func (g *qaMOLGateway) refusedDials(t *testing.T, n int) []url.Values {
	t.Helper()
	deadline := time.Now().Add(qaMOLWait)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		got := append([]url.Values(nil), g.refused...)
		g.mu.Unlock()
		if len(got) >= n {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fewer than %d redials reached the refusing gateway within %v", n, qaMOLWait)
	return nil
}

func (g *qaMOLGateway) close() {
	g.mu.Lock()
	all := append([]*qaMOLConn(nil), g.all...)
	g.mu.Unlock()
	for _, c := range all {
		_ = c.ws.CloseNow()
	}
	g.srv.Close()
}

func (c *qaMOLConn) send(t *testing.T, frames ...*vttv1.ServerFrame) {
	t.Helper()
	for _, f := range frames {
		b, err := protojson.Marshal(f)
		if err != nil {
			t.Fatalf("marshal frame: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), qaMOLWait)
		err = c.ws.Write(ctx, websocket.MessageText, b)
		cancel()
		if err != nil {
			t.Fatalf("write frame: %v", err)
		}
	}
}

func (c *qaMOLConn) drop() { _ = c.ws.CloseNow() }

func qaMOLHead(n int64) *vttv1.ServerFrame {
	return &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_CatchUpHead{CatchUpHead: &vttv1.CatchUpHead{HeadSequence: n}}}
}

func qaMOLPresence() *vttv1.ServerFrame {
	return &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_PresenceSnapshot{PresenceSnapshot: &vttv1.PresenceSnapshot{}}}
}

func qaMOLWatchable(ids ...string) *vttv1.ServerFrame {
	w := &vttv1.WatchableChanged{}
	for _, id := range ids {
		w.Actors = append(w.Actors, &vttv1.WatchableActor{ActorId: id, Name: "name-" + id})
	}
	return &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_WatchableChanged{WatchableChanged: w}}
}

func qaMOLView(actor string) *vttv1.ServerFrame {
	return &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_ViewpointChanged{ViewpointChanged: &vttv1.ViewpointChanged{ActorId: actor}}}
}

func qaMOLEntries(envs []*vttv1.Envelope) []*vttv1.ServerFrame {
	out := make([]*vttv1.ServerFrame, 0, len(envs))
	for _, e := range envs {
		out = append(out, &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: proto.Clone(e).(*vttv1.Envelope)}})
	}
	return out
}

func qaMOLTime() *timestamppb.Timestamp {
	return timestamppb.New(time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC))
}

func qaMOLActor(seq int64, id, name string) *vttv1.Envelope {
	return &vttv1.Envelope{
		Sequence:   seq,
		OccurredAt: qaMOLTime(),
		Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: id, Name: name, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
	}
}

func qaMOLNote(seq int64, key, title string) *vttv1.Envelope {
	return &vttv1.Envelope{
		Sequence:   seq,
		OccurredAt: qaMOLTime(),
		Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key: key, Title: title, Text: title, Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
		}},
	}
}

func qaMOLLogA() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		qaMOLActor(1, "a1", "Alpha"),
		qaMOLActor(2, "b1", "Bravo"),
		qaMOLNote(3, "lantern", "Alpha saw the lantern"),
		qaMOLNote(4, "door", "Alpha heard the door"),
	}
}

func qaMOLLogB() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		qaMOLActor(1, "a1", "Alpha"),
		qaMOLActor(2, "b1", "Bravo"),
		qaMOLNote(3, "lantern", "Bravo saw the lantern"),
	}
}

func qaMOLWorld(from, to int64) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for s := from; s <= to; s++ {
		e := qaMOLNote(s, fmt.Sprintf("world-%d", s), fmt.Sprintf("World fact %d", s))
		e.EventId = fmt.Sprintf("ev-%d", s)
		out = append(out, e)
	}
	return out
}

func (c *qaMOLConn) openOn(t *testing.T, head int64, actor string, envs []*vttv1.Envelope) {
	t.Helper()
	c.send(t, qaMOLHead(head), qaMOLPresence(), qaMOLWatchable("a1", "b1"), qaMOLView(actor))
	c.send(t, qaMOLEntries(envs)...)
}

func (c *qaMOLConn) switchTo(t *testing.T, actor string, envs []*vttv1.Envelope) {
	t.Helper()
	c.send(t, qaMOLView(actor))
	c.send(t, qaMOLEntries(envs)...)
}

type qaMOLTable struct {
	gw   *qaMOLGateway
	sess *mcpsdk.ClientSession
}

func qaMOLOpen(t *testing.T) (*qaMOLTable, *qaMOLConn) {
	t.Helper()
	tools, err := os.ReadFile("../../contract/gen/tools/tools.json")
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	gw := qaMOLNewGateway()
	srv, err := mcp.New(mcp.Config{
		WSURL:     "ws" + strings.TrimPrefix(gw.srv.URL, "http") + "/ws",
		Token:     "qa-token",
		ToolsJSON: tools,
	})
	if err != nil {
		gw.close()
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serverT, clientT := mcpsdk.NewInMemoryTransports()
	ran := make(chan struct{})
	go func() {
		defer close(ran)
		_ = srv.Run(ctx, serverT)
	}()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qa-one-log", Version: "v0"}, nil)
	sess, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		cancel()
		gw.close()
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() {
		_ = sess.Close()
		cancel()
		select {
		case <-ran:
		case <-time.After(qaMOLWait):
			t.Errorf("Run did not return within %v of cancel", qaMOLWait)
		}
		gw.close()
	})
	tb := &qaMOLTable{gw: gw, sess: sess}
	return tb, gw.next(t)
}

func (tb *qaMOLTable) call(t *testing.T, name string, args map[string]any) map[string]json.RawMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), qaMOLWait)
	defer cancel()
	res, err := tb.sess.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("%s answered an error or nothing: %+v", name, res.Content)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("%s answered %T, not text", name, res.Content[0])
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal([]byte(text.Text), &body); err != nil {
		t.Fatalf("%s answered non-JSON %q: %v", name, text.Text, err)
	}
	return body
}

type qaMOLState struct {
	body   map[string]json.RawMessage
	notes  map[string]string
	actors []string
}

func (s qaMOLState) log() (string, bool) {
	raw, ok := s.body["log"]
	if !ok {
		return "", false
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "<" + string(raw) + ">", true
	}
	return v, true
}

func (tb *qaMOLTable) state(t *testing.T) qaMOLState {
	t.Helper()
	body := tb.call(t, "get_state", map[string]any{})
	s := qaMOLState{body: body, notes: map[string]string{}}
	var notes map[string]struct{ Title string }
	if raw, ok := body["Notes"]; ok {
		if err := json.Unmarshal(raw, &notes); err != nil {
			t.Fatalf("get_state Notes: %v", err)
		}
	}
	for k, n := range notes {
		s.notes[k] = n.Title
	}
	var actors map[string]json.RawMessage
	if raw, ok := body["Actors"]; ok {
		if err := json.Unmarshal(raw, &actors); err != nil {
			t.Fatalf("get_state Actors: %v", err)
		}
	}
	for id := range actors {
		s.actors = append(s.actors, id)
	}
	sort.Strings(s.actors)
	return s
}

func (tb *qaMOLTable) await(t *testing.T, what string, ok func(qaMOLState) bool) qaMOLState {
	t.Helper()
	deadline := time.Now().Add(qaMOLWait)
	var last qaMOLState
	for time.Now().Before(deadline) {
		last = tb.state(t)
		if ok(last) {
			return last
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("get_state never showed %s within %v; last notes=%v actors=%v body log=%s", what, qaMOLWait, last.notes, last.actors, last.body["log"])
	return last
}

type qaMOLPage struct {
	body   map[string]json.RawMessage
	events []*vttv1.Envelope
	more   bool
}

func (p qaMOLPage) log() (string, bool) {
	return qaMOLState{body: p.body}.log()
}

func (tb *qaMOLTable) page(t *testing.T, after int64, limit int) qaMOLPage {
	t.Helper()
	args := map[string]any{"afterSequence": after}
	if limit > 0 {
		args["limit"] = limit
	}
	body := tb.call(t, "get_events_since", args)
	p := qaMOLPage{body: body}
	var raws []json.RawMessage
	if raw, ok := body["events"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &raws); err != nil {
			t.Fatalf("get_events_since events: %v", err)
		}
	}
	for _, r := range raws {
		e := &vttv1.Envelope{}
		if err := protojson.Unmarshal(r, e); err != nil {
			t.Fatalf("get_events_since event %s: %v", r, err)
		}
		p.events = append(p.events, e)
	}
	if raw, ok := body["more"]; ok {
		_ = json.Unmarshal(raw, &p.more)
	}
	return p
}

func (tb *qaMOLTable) pageAll(t *testing.T, after int64) []*vttv1.Envelope {
	t.Helper()
	var out []*vttv1.Envelope
	for i := 0; i < 50; i++ {
		p := tb.page(t, after, 2)
		out = append(out, p.events...)
		if !p.more || len(p.events) == 0 {
			return out
		}
		after = p.events[len(p.events)-1].Sequence
	}
	t.Fatalf("get_events_since still reported more after 50 pages")
	return out
}

func qaMOLSameEntries(t *testing.T, what string, got, want []*vttv1.Envelope) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d entries %v, want %d %v", what, len(got), got, len(want), want)
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Fatalf("%s: entry %d is %v, want %v", what, i, got[i], want[i])
		}
	}
}

func qaMOLWantDial(t *testing.T, c *qaMOLConn, log string, after int64) {
	t.Helper()
	qaMOLWantQuery(t, c.query, log, after)
}

func qaMOLWantQuery(t *testing.T, q url.Values, log string, after int64) {
	t.Helper()
	if got, want := q.Get("after"), strconv.FormatInt(after, 10); got != want {
		t.Errorf("redial after = %q, want %q (query %v)", got, want, q)
	}
	if log == "" {
		if q.Has("log") {
			t.Errorf("redial names log %q, want no log (query %v)", q.Get("log"), q)
		}
		return
	}
	if got := q.Get("log"); got != log {
		t.Errorf("redial log = %q, want %q (query %v)", got, log, q)
	}
}

func qaMOLHasA(s qaMOLState) bool { return s.notes["door"] == "Alpha heard the door" }

func qaMOLHasB(s qaMOLState) bool { return s.notes["lantern"] == "Bravo saw the lantern" }

// VTT-418
func TestQAMOLSwitchToAnotherLogFoldsThatLogAlone(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)

	c.switchTo(t, "b1", qaMOLLogB())
	s := tb.await(t, "log b1's last entry", qaMOLHasB)
	if _, ok := s.notes["door"]; ok || len(s.notes) != 1 {
		t.Errorf("after the switch the fold holds notes %v, want only b1's lantern", s.notes)
	}
	qaMOLSameEntries(t, "get_events_since(0) after the switch", tb.pageAll(t, 0), qaMOLLogB())
	qaMOLSameEntries(t, "get_events_since(1) after the switch", tb.pageAll(t, 1), qaMOLLogB()[1:])
}

// VTT-419 VTT-421
func TestQAMOLSwitchNamingTheHeldLogKeepsIt(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)

	well := qaMOLNote(5, "well", "Alpha found the well")
	c.switchTo(t, "a1", []*vttv1.Envelope{well})
	s := tb.await(t, "log a1's fifth entry", func(s qaMOLState) bool { return s.notes["well"] == "Alpha found the well" })
	if !qaMOLHasA(s) || s.notes["lantern"] != "Alpha saw the lantern" {
		t.Errorf("after a ViewpointChanged naming the held log the fold holds notes %v, want a1's entries kept", s.notes)
	}
	qaMOLSameEntries(t, "get_events_since(0) after the same-log ViewpointChanged", tb.pageAll(t, 0), append(qaMOLLogA(), well))

	c.drop()
	qaMOLWantDial(t, tb.gw.next(t), "a1", 5)
}

// VTT-418
func TestQAMOLSwitchBackToALogFoldsItAfresh(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)
	c.switchTo(t, "b1", qaMOLLogB())
	tb.await(t, "log b1's last entry", func(s qaMOLState) bool { return qaMOLHasB(s) && !qaMOLHasA(s) })

	c.switchTo(t, "a1", qaMOLLogA())
	s := tb.await(t, "log a1's last entry again", qaMOLHasA)
	if s.notes["lantern"] != "Alpha saw the lantern" || len(s.notes) != 2 {
		t.Errorf("after switching back the fold holds notes %v, want a1's lantern and door alone", s.notes)
	}
	qaMOLSameEntries(t, "get_events_since(0) after switching back", tb.pageAll(t, 0), qaMOLLogA())

	c.drop()
	qaMOLWantDial(t, tb.gw.next(t), "a1", 4)
}

// VTT-418 VTT-422
func TestQAMOLSwitchToNobodyHoldsNoEntry(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)

	c.switchTo(t, "", nil)
	s := tb.await(t, `log "" for nobody`, func(s qaMOLState) bool {
		v, ok := s.log()
		return ok && v == ""
	})
	if len(s.notes) != 0 || len(s.actors) != 0 {
		t.Errorf("on nobody the fold holds notes %v actors %v, want nothing", s.notes, s.actors)
	}
	p := tb.page(t, 0, 0)
	if len(p.events) != 0 {
		t.Errorf("on nobody get_events_since(0) pages %v, want none", p.events)
	}
	if v, ok := p.log(); !ok || v != "" {
		t.Errorf("on nobody get_events_since log = %q present=%v, want \"\" present", v, ok)
	}
}

// VTT-422
func TestQAMOLReadToolsNameTheCharacterLog(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	s := tb.await(t, "log a1's last entry", qaMOLHasA)
	if v, ok := s.log(); !ok || v != "a1" {
		t.Errorf("get_state log = %q present=%v, want a1", v, ok)
	}
	if v, ok := tb.page(t, 0, 0).log(); !ok || v != "a1" {
		t.Errorf("get_events_since log = %q present=%v, want a1", v, ok)
	}

	c.switchTo(t, "b1", qaMOLLogB())
	s = tb.await(t, "log b1's last entry", qaMOLHasB)
	if v, ok := s.log(); !ok || v != "b1" {
		t.Errorf("after the switch get_state log = %q present=%v, want b1", v, ok)
	}
	if v, ok := tb.page(t, 0, 0).log(); !ok || v != "b1" {
		t.Errorf("after the switch get_events_since log = %q present=%v, want b1", v, ok)
	}
}

// VTT-422
func TestQAMOLReadToolsOnTheWorldNameNoLog(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.send(t, qaMOLHead(3), qaMOLPresence())
	c.send(t, qaMOLEntries(qaMOLWorld(1, 3))...)
	s := tb.await(t, "the world's third entry", func(s qaMOLState) bool { return s.notes["world-3"] == "World fact 3" })
	if v, ok := s.log(); ok {
		t.Errorf("on the world get_state names log %q, want no log key", v)
	}
	p := tb.page(t, 0, 0)
	if len(p.events) != 3 {
		t.Errorf("on the world get_events_since(0) pages %d entries, want 3", len(p.events))
	}
	if v, ok := p.log(); ok {
		t.Errorf("on the world get_events_since names log %q, want no log key", v)
	}
}

// VTT-419 VTT-420 VTT-421
func TestQAMOLRedialNamesTheHeldLogAtItsCursor(t *testing.T) {
	tb, c := qaMOLOpen(t)
	logA := qaMOLLogA()
	c.openOn(t, 3, "a1", logA[:3])
	tb.await(t, "log a1's third entry", func(s qaMOLState) bool { return s.notes["lantern"] == "Alpha saw the lantern" })

	c.drop()
	r := tb.gw.next(t)
	qaMOLWantDial(t, r, "a1", 3)

	well := qaMOLNote(5, "well", "Alpha found the well")
	r.openOn(t, 5, "a1", []*vttv1.Envelope{logA[3], well})
	s := tb.await(t, "log a1's fifth entry", func(s qaMOLState) bool { return s.notes["well"] == "Alpha found the well" })
	if !qaMOLHasA(s) || s.notes["lantern"] != "Alpha saw the lantern" || strings.Join(s.actors, ",") != "a1,b1" {
		t.Errorf("after a redial on the held log the fold holds notes %v actors %v, want entries 1 to 5", s.notes, s.actors)
	}
	qaMOLSameEntries(t, "get_events_since(0) after the redial", tb.pageAll(t, 0), append(qaMOLLogA(), well))

	r.drop()
	qaMOLWantDial(t, tb.gw.next(t), "a1", 5)
}

// VTT-420 VTT-421
func TestQAMOLRedialAfterASwitchNamesTheNewLog(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)
	c.switchTo(t, "b1", qaMOLLogB())
	tb.await(t, "log b1's last entry", qaMOLHasB)

	c.drop()
	qaMOLWantDial(t, tb.gw.next(t), "b1", 3)
}

// VTT-420 VTT-421
func TestQAMOLRedialAfterASwitchToAnEmptyLogDialsZero(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)
	c.switchTo(t, "b1", nil)
	tb.await(t, "log b1 named", func(s qaMOLState) bool {
		v, ok := s.log()
		return ok && v == "b1"
	})

	c.drop()
	qaMOLWantDial(t, tb.gw.next(t), "b1", 0)
}

// VTT-420 VTT-421
func TestQAMOLRedialOnNobodyNamesNoLog(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)
	c.switchTo(t, "", nil)
	tb.await(t, `log "" for nobody`, func(s qaMOLState) bool {
		v, ok := s.log()
		return ok && v == ""
	})

	c.drop()
	qaMOLWantDial(t, tb.gw.next(t), "", 0)
}

// VTT-420 VTT-421
func TestQAMOLRedialOnTheWorldNamesNoLog(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.send(t, qaMOLHead(3), qaMOLPresence())
	c.send(t, qaMOLEntries(qaMOLWorld(1, 3))...)
	tb.await(t, "the world's third entry", func(s qaMOLState) bool { return s.notes["world-3"] == "World fact 3" })

	c.drop()
	r := tb.gw.next(t)
	qaMOLWantDial(t, r, "", 3)

	r.send(t, qaMOLHead(5), qaMOLPresence())
	r.send(t, qaMOLEntries(qaMOLWorld(4, 5))...)
	tb.await(t, "the world's fifth entry", func(s qaMOLState) bool { return s.notes["world-5"] == "World fact 5" })

	r.drop()
	qaMOLWantDial(t, tb.gw.next(t), "", 5)
}

// VTT-418 VTT-420 VTT-421
func TestQAMOLRedialRefusedTheHeldLogFoldsTheLogSent(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	tb.await(t, "log a1's last entry", qaMOLHasA)

	c.drop()
	r := tb.gw.next(t)
	qaMOLWantDial(t, r, "a1", 4)

	r.send(t, qaMOLHead(3), qaMOLPresence(), qaMOLWatchable("b1"), qaMOLView("b1"))
	r.send(t, qaMOLEntries(qaMOLLogB())...)
	s := tb.await(t, "log b1's last entry", qaMOLHasB)
	if _, ok := s.notes["door"]; ok || len(s.notes) != 1 {
		t.Errorf("after a redial sent another log the fold holds notes %v, want only b1's lantern", s.notes)
	}
	qaMOLSameEntries(t, "get_events_since(0) after a redial sent another log", tb.pageAll(t, 0), qaMOLLogB())

	r.drop()
	qaMOLWantDial(t, tb.gw.next(t), "b1", 3)
}

// VTT-420 VTT-421 VTT-422
func TestQAMOLRedialsThatFailKeepNamingTheHeldLog(t *testing.T) {
	tb, c := qaMOLOpen(t)
	c.openOn(t, 4, "a1", qaMOLLogA())
	c.switchTo(t, "b1", qaMOLLogB())
	tb.await(t, "log b1's last entry", qaMOLHasB)

	tb.gw.refuse(true)
	c.drop()
	for _, q := range tb.gw.refusedDials(t, 2) {
		qaMOLWantQuery(t, q, "b1", 3)
	}
	s := tb.state(t)
	if v, ok := s.log(); !ok || v != "b1" || !qaMOLHasB(s) {
		t.Errorf("while redialling get_state log = %q present=%v notes %v, want b1's fold", v, ok, s.notes)
	}
	if v, ok := tb.page(t, 0, 0).log(); !ok || v != "b1" {
		t.Errorf("while redialling get_events_since log = %q present=%v, want b1", v, ok)
	}

	tb.gw.refuse(false)
	qaMOLWantDial(t, tb.gw.next(t), "b1", 3)
}
