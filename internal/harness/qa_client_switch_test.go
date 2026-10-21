package harness_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

const qaCSNobodyFrame = `{"viewpointChanged":{}}`

type qaCSWire struct {
	frame *vttv1.ServerFrame
	raw   string
}

type qaCSScript func(ctx context.Context, conn *websocket.Conn) error

type qaCSGateway struct {
	wsURL      string
	handshakes chan url.Values
	scriptErrs chan error
	done       chan struct{}
}

func qaCSStart(t *testing.T, script qaCSScript) *qaCSGateway {
	t.Helper()
	g := &qaCSGateway{
		handshakes: make(chan url.Values, 8),
		scriptErrs: make(chan error, 8),
		done:       make(chan struct{}, 8),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { g.done <- struct{}{} }()
		g.handshakes <- r.URL.Query()
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			g.scriptErrs <- fmt.Errorf("accept: %w", err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := script(ctx, conn); err != nil {
			g.scriptErrs <- err
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "qa: script done")
	}))
	t.Cleanup(srv.Close)
	g.wsURL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	return g
}

func (g *qaCSGateway) dial(t *testing.T, after int64, log string) *harness.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := harness.Dial(ctx, g.wsURL, "qa-token", after, log)
	if err != nil {
		t.Fatalf("Dial(after=%d, log=%q): %v", after, log, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func (g *qaCSGateway) handshake(t *testing.T) url.Values {
	t.Helper()
	select {
	case q := <-g.handshakes:
		return q
	case <-time.After(5 * time.Second):
		t.Fatal("the fake gateway saw no handshake")
		return nil
	}
}

func (g *qaCSGateway) finish(t *testing.T) {
	t.Helper()
	select {
	case <-g.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the fake gateway's script did not finish")
	}
	select {
	case err := <-g.scriptErrs:
		t.Fatalf("fake gateway: %v", err)
	default:
	}
}

func qaCSSend(ctx context.Context, conn *websocket.Conn, ws []qaCSWire) error {
	for i, w := range ws {
		b := []byte(w.raw)
		if w.raw == "" {
			var err error
			if b, err = protojson.Marshal(w.frame); err != nil {
				return fmt.Errorf("frame %d: marshal: %w", i, err)
			}
		}
		if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
			return fmt.Errorf("frame %d: write: %w", i, err)
		}
	}
	return nil
}

func qaCSSendAll(ws ...[]qaCSWire) qaCSScript {
	return func(ctx context.Context, conn *websocket.Conn) error {
		for _, part := range ws {
			if err := qaCSSend(ctx, conn, part); err != nil {
				return err
			}
		}
		return nil
	}
}

func qaCSHead(n int64) qaCSWire {
	return qaCSWire{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_CatchUpHead{
		CatchUpHead: &vttv1.CatchUpHead{HeadSequence: n},
	}}}
}

func qaCSPresence() qaCSWire {
	return qaCSWire{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_PresenceSnapshot{
		PresenceSnapshot: &vttv1.PresenceSnapshot{Present: []*vttv1.PresenceChanged{
			{ParticipantId: "p-dm", DisplayName: "Dana", State: vttv1.PresenceState_PRESENCE_STATE_CONNECTED},
		}},
	}}}
}

func qaCSPresenceChanged(id string) qaCSWire {
	return qaCSWire{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_PresenceChanged{
		PresenceChanged: &vttv1.PresenceChanged{ParticipantId: id, DisplayName: id, State: vttv1.PresenceState_PRESENCE_STATE_CONNECTED},
	}}}
}

func qaCSWatchable(ids ...string) qaCSWire {
	w := &vttv1.WatchableChanged{}
	for _, id := range ids {
		w.Actors = append(w.Actors, &vttv1.WatchableActor{ActorId: id, Name: "Name of " + id})
	}
	return qaCSWire{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_WatchableChanged{WatchableChanged: w}}}
}

func qaCSView(actor string) qaCSWire {
	w := qaCSWire{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_ViewpointChanged{
		ViewpointChanged: &vttv1.ViewpointChanged{ActorId: actor},
	}}}
	if actor == "" {
		w.raw = qaCSNobodyFrame
	}
	return w
}

func qaCSEntry(env *vttv1.Envelope) qaCSWire {
	return qaCSWire{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Event{Event: env}}}
}

func qaCSNarration(seq int64, eventID, text string) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:    eventID,
		Sequence:   seq,
		OccurredAt: timestamppb.New(time.Date(2026, 10, 21, 9, 0, int(seq), 0, time.UTC)),
		SessionId:  "s-1",
		Payload:    &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}},
	}
}

func qaCSSceneSeen(seq int64, scene string) *vttv1.Envelope {
	return &vttv1.Envelope{
		Sequence: seq,
		Payload:  &vttv1.Envelope_SceneSeen{SceneSeen: &vttv1.SceneSeen{SceneId: scene}},
	}
}

func qaCSMoved(seq int64, eventID, token string, x int32) *vttv1.Envelope {
	return &vttv1.Envelope{
		EventId:  eventID,
		Sequence: seq,
		Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: token, SceneId: "sc-cellar",
			From: &vttv1.GridPosition{X: x - 1, Y: 2}, To: &vttv1.GridPosition{X: x, Y: 2},
		}},
	}
}

func qaCSWanted(ws ...[]qaCSWire) []harness.Delivery {
	var out []harness.Delivery
	for _, part := range ws {
		for _, w := range part {
			switch f := w.frame.GetFrame().(type) {
			case *vttv1.ServerFrame_Event:
				out = append(out, harness.Delivery{Envelope: f.Event})
			case *vttv1.ServerFrame_ViewpointChanged:
				out = append(out, harness.Delivery{Viewpoint: f.ViewpointChanged})
			}
		}
	}
	return out
}

func qaCSCollect(t *testing.T, c *harness.Client) []harness.Delivery {
	t.Helper()
	deadline := time.After(10 * time.Second)
	var got []harness.Delivery
	for {
		select {
		case d, ok := <-c.Events():
			if !ok {
				if errors.Is(c.CloseErr(), harness.ErrEventsOverflow) {
					t.Fatalf("Events() overflowed after %d deliveries", len(got))
				}
				return got
			}
			got = append(got, d)
		case <-deadline:
			t.Fatalf("Events() was not closed within 10s; %d deliveries so far: %s", len(got), qaCSDescribe(got))
		}
	}
}

func qaCSDescribeOne(d harness.Delivery) string {
	switch {
	case d.Envelope != nil && d.Viewpoint != nil:
		return "BOTH"
	case d.Viewpoint != nil:
		if d.Viewpoint.GetActorId() == "" {
			return "switch(nobody)"
		}
		return "switch(" + d.Viewpoint.GetActorId() + ")"
	case d.Envelope != nil:
		return fmt.Sprintf("entry(seq=%d,id=%q)", d.Envelope.GetSequence(), d.Envelope.GetEventId())
	default:
		return "EMPTY"
	}
}

func qaCSDescribe(ds []harness.Delivery) string {
	parts := make([]string, len(ds))
	for i, d := range ds {
		parts[i] = qaCSDescribeOne(d)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func qaCSAssertDeliveries(t *testing.T, got, want []harness.Delivery) {
	t.Helper()
	if len(want) == 0 {
		t.Fatal("test fixture wants no deliveries")
	}
	if len(got) != len(want) {
		t.Fatalf("got %d deliveries, want %d\n got: %s\nwant: %s", len(got), len(want), qaCSDescribe(got), qaCSDescribe(want))
	}
	for i := range want {
		g, w := got[i], want[i]
		if (g.Envelope == nil) == (g.Viewpoint == nil) {
			t.Errorf("delivery %d is %s, want exactly one of an entry or a switch", i, qaCSDescribeOne(g))
			continue
		}
		if w.Viewpoint != nil {
			if g.Viewpoint == nil || !proto.Equal(g.Viewpoint, w.Viewpoint) {
				t.Errorf("delivery %d is %s, want %s", i, qaCSDescribeOne(g), qaCSDescribeOne(w))
			}
			continue
		}
		if g.Envelope == nil || !proto.Equal(g.Envelope, w.Envelope) {
			t.Errorf("delivery %d is %s, want %s (%v)", i, qaCSDescribeOne(g), qaCSDescribeOne(w), w.Envelope)
		}
	}
	if t.Failed() {
		t.Logf(" got: %s\nwant: %s", qaCSDescribe(got), qaCSDescribe(want))
	}
}

func qaCSCountSwitches(ds []harness.Delivery) int {
	n := 0
	for _, d := range ds {
		if d.Viewpoint != nil {
			n++
		}
	}
	return n
}

func qaCSOpening() []qaCSWire {
	return []qaCSWire{qaCSHead(3), qaCSPresence(), qaCSWatchable("pc-ayla", "pc-bram")}
}

// VTT-410
func TestQACSDialNamesTheLogBesideAfter(t *testing.T) {
	g := qaCSStart(t, qaCSSendAll(qaCSOpening()))
	c := g.dial(t, 42, "pc-ayla")
	q := g.handshake(t)
	if got := q["log"]; len(got) != 1 || got[0] != "pc-ayla" {
		t.Errorf("handshake log = %q, want exactly [\"pc-ayla\"] (query %q)", got, q.Encode())
	}
	if got := q["after"]; len(got) != 1 || got[0] != "42" {
		t.Errorf("handshake after = %q, want exactly [\"42\"] (query %q)", got, q.Encode())
	}
	qaCSCollect(t, c)
	g.finish(t)
}

// VTT-410
func TestQACSDialGivenNoLogSendsNoLog(t *testing.T) {
	g := qaCSStart(t, qaCSSendAll(qaCSOpening()))
	c := g.dial(t, 42, "")
	q := g.handshake(t)
	if q.Has("log") {
		t.Errorf("handshake carries log %q, want no log parameter at all (query %q)", q["log"], q.Encode())
	}
	if got := q["after"]; len(got) != 1 || got[0] != "42" {
		t.Errorf("handshake after = %q, want exactly [\"42\"] (query %q)", got, q.Encode())
	}
	qaCSCollect(t, c)
	g.finish(t)
}

// VTT-410
func TestQACSDialNamesTheLogAtCursorZero(t *testing.T) {
	g := qaCSStart(t, qaCSSendAll(qaCSOpening()))
	c := g.dial(t, 0, "pc-bram")
	q := g.handshake(t)
	if got := q["log"]; len(got) != 1 || got[0] != "pc-bram" {
		t.Errorf("handshake log = %q, want exactly [\"pc-bram\"] (query %q)", got, q.Encode())
	}
	if q.Has("after") && q.Get("after") != "0" {
		t.Errorf("handshake after = %q, want 0 or absent (query %q)", q["after"], q.Encode())
	}
	qaCSCollect(t, c)
	g.finish(t)
}

// VTT-410
func TestQACSDialNamesALogWhoseIdNeedsEscaping(t *testing.T) {
	const awkward = "pc a&log=other+x/é?#%41"
	g := qaCSStart(t, qaCSSendAll(qaCSOpening()))
	c := g.dial(t, 7, awkward)
	q := g.handshake(t)
	if got := q["log"]; len(got) != 1 || got[0] != awkward {
		t.Errorf("handshake log = %q, want exactly [%q] (query %q)", got, awkward, q.Encode())
	}
	if got := q["after"]; len(got) != 1 || got[0] != "7" {
		t.Errorf("handshake after = %q, want exactly [\"7\"] (query %q)", got, q.Encode())
	}
	qaCSCollect(t, c)
	g.finish(t)
}

// VTT-409
func TestQACSOpeningViewpointIsHandedBeforeTheEntries(t *testing.T) {
	opening := qaCSEntry(qaCSSceneSeen(1, "sc-cellar"))
	log := []qaCSWire{
		qaCSView("pc-ayla"),
		opening,
		qaCSEntry(qaCSNarration(2, "ev-0007", "The door creaks.")),
		qaCSEntry(qaCSMoved(3, "ev-0009", "tok-ayla", 4)),
	}
	g := qaCSStart(t, qaCSSendAll(qaCSOpening(), log))
	c := g.dial(t, 0, "pc-ayla")
	got := qaCSCollect(t, c)
	g.finish(t)
	want := []harness.Delivery{
		{Viewpoint: &vttv1.ViewpointChanged{ActorId: "pc-ayla"}},
		{Envelope: qaCSSceneSeen(1, "sc-cellar")},
		{Envelope: qaCSNarration(2, "ev-0007", "The door creaks.")},
		{Envelope: qaCSMoved(3, "ev-0009", "tok-ayla", 4)},
	}
	qaCSAssertDeliveries(t, got, want)
}

// VTT-409
func TestQACSNobodysViewpointIsHanded(t *testing.T) {
	steps := []qaCSWire{
		qaCSView(""),
		qaCSWatchable("pc-ayla"),
		qaCSView("pc-ayla"),
		qaCSEntry(qaCSSceneSeen(1, "sc-cellar")),
		qaCSEntry(qaCSNarration(2, "ev-0003", "Torchlight.")),
		qaCSView(""),
	}
	g := qaCSStart(t, qaCSSendAll([]qaCSWire{qaCSHead(0), qaCSPresence(), qaCSWatchable()}, steps))
	c := g.dial(t, 0, "")
	got := qaCSCollect(t, c)
	g.finish(t)
	want := []harness.Delivery{
		{Viewpoint: &vttv1.ViewpointChanged{}},
		{Viewpoint: &vttv1.ViewpointChanged{ActorId: "pc-ayla"}},
		{Envelope: qaCSSceneSeen(1, "sc-cellar")},
		{Envelope: qaCSNarration(2, "ev-0003", "Torchlight.")},
		{Viewpoint: &vttv1.ViewpointChanged{}},
	}
	qaCSAssertDeliveries(t, got, want)
	for _, i := range []int{0, 4} {
		if i < len(got) && (got[i].Viewpoint == nil || got[i].Viewpoint.GetActorId() != "") {
			t.Errorf("delivery %d is %s, want a switch to nobody", i, qaCSDescribeOne(got[i]))
		}
	}
}

// VTT-409 VTT-427
func TestQACSEverySwitchIsHandedInWireOrderAmongTheEntries(t *testing.T) {
	shared := qaCSNarration(2, "ev-0040", "A bell tolls across the town.")
	sharedInB := proto.Clone(shared).(*vttv1.Envelope)
	sharedInB.Sequence = 3
	steps := []qaCSWire{
		qaCSView("pc-ayla"),
		qaCSEntry(qaCSSceneSeen(1, "sc-cellar")),
		qaCSEntry(shared),
		qaCSPresenceChanged("p-bram"),
		qaCSEntry(qaCSMoved(3, "ev-0041", "tok-ayla", 5)),
		qaCSWatchable("pc-ayla", "pc-bram", "pc-cora"),
		qaCSView("pc-bram"),
		qaCSEntry(qaCSSceneSeen(1, "sc-cellar")),
		qaCSEntry(qaCSSceneSeen(2, "sc-attic")),
		qaCSEntry(sharedInB),
		qaCSView(""),
		qaCSPresenceChanged("p-cora"),
		qaCSView("pc-cora"),
		qaCSView("pc-ayla"),
		qaCSEntry(qaCSSceneSeen(1, "sc-cellar")),
		qaCSEntry(shared),
		qaCSWatchable("pc-ayla", "pc-cora"),
		qaCSView("pc-cora"),
		qaCSEntry(qaCSSceneSeen(1, "sc-yard")),
		qaCSEntry(qaCSNarration(2, "", "Cora hums.")),
		qaCSEntry(qaCSMoved(3, "ev-0052", "tok-cora", 9)),
		qaCSEntry(qaCSMoved(4, "ev-0053", "tok-cora", 10)),
		qaCSView(""),
		qaCSView("pc-ayla"),
		qaCSEntry(qaCSSceneSeen(1, "sc-cellar")),
	}
	g := qaCSStart(t, qaCSSendAll(qaCSOpening(), steps))
	c := g.dial(t, 0, "pc-ayla")
	got := qaCSCollect(t, c)
	g.finish(t)
	want := qaCSWanted(steps)
	if n := qaCSCountSwitches(want); n != 8 {
		t.Fatalf("fixture has %d switches, want 8", n)
	}
	qaCSAssertDeliveries(t, got, want)
}

// VTT-409
func TestQACSAViewpointRepeatingTheHeldActorIsHandedEachTime(t *testing.T) {
	steps := []qaCSWire{
		qaCSView("pc-bram"),
		qaCSEntry(qaCSSceneSeen(1, "sc-attic")),
		qaCSEntry(qaCSNarration(2, "ev-0012", "Dust settles.")),
		qaCSView("pc-bram"),
		qaCSEntry(qaCSNarration(3, "ev-0013", "A rat skitters.")),
		qaCSView("pc-bram"),
		qaCSView("pc-bram"),
	}
	g := qaCSStart(t, qaCSSendAll(qaCSOpening(), steps))
	c := g.dial(t, 0, "pc-bram")
	got := qaCSCollect(t, c)
	g.finish(t)
	want := []harness.Delivery{
		{Viewpoint: &vttv1.ViewpointChanged{ActorId: "pc-bram"}},
		{Envelope: qaCSSceneSeen(1, "sc-attic")},
		{Envelope: qaCSNarration(2, "ev-0012", "Dust settles.")},
		{Viewpoint: &vttv1.ViewpointChanged{ActorId: "pc-bram"}},
		{Envelope: qaCSNarration(3, "ev-0013", "A rat skitters.")},
		{Viewpoint: &vttv1.ViewpointChanged{ActorId: "pc-bram"}},
		{Viewpoint: &vttv1.ViewpointChanged{ActorId: "pc-bram"}},
	}
	qaCSAssertDeliveries(t, got, want)
}

// VTT-409
func TestQACSASwitchAroundItsCommandResultIsHanded(t *testing.T) {
	for _, order := range []string{"result-first", "result-among-entries", "result-last"} {
		t.Run(order, func(t *testing.T) {
			before := []qaCSWire{
				qaCSView("pc-ayla"),
				qaCSEntry(qaCSSceneSeen(1, "sc-cellar")),
				qaCSEntry(qaCSNarration(2, "ev-0021", "Ayla waits.")),
			}
			after := []qaCSWire{
				qaCSView("pc-bram"),
				qaCSEntry(qaCSSceneSeen(1, "sc-attic")),
				qaCSEntry(qaCSNarration(2, "ev-0022", "Bram listens.")),
			}
			script := func(ctx context.Context, conn *websocket.Conn) error {
				if err := qaCSSend(ctx, conn, append(qaCSOpening(), before...)); err != nil {
					return err
				}
				_, data, err := conn.Read(ctx)
				if err != nil {
					return fmt.Errorf("read command: %w", err)
				}
				cmd := &vttv1.ClientCommand{}
				if err := protojson.Unmarshal(data, cmd); err != nil {
					return fmt.Errorf("decode command %s: %w", data, err)
				}
				if cmd.GetSetViewpoint().GetActorId() != "pc-bram" {
					return fmt.Errorf("command is %s, want set_viewpoint pc-bram", data)
				}
				result := qaCSWire{frame: &vttv1.ServerFrame{Frame: &vttv1.ServerFrame_Result{
					Result: &vttv1.CommandResult{RequestId: cmd.GetRequestId(), Ok: true},
				}}}
				var rest []qaCSWire
				switch order {
				case "result-first":
					rest = append([]qaCSWire{result}, after...)
				case "result-among-entries":
					rest = []qaCSWire{after[0], after[1], result, after[2]}
				default:
					rest = append(append([]qaCSWire{}, after...), result)
				}
				return qaCSSend(ctx, conn, rest)
			}
			g := qaCSStart(t, script)
			c := g.dial(t, 0, "pc-ayla")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			res, err := c.SendCommand(ctx, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
				SetViewpoint: &vttv1.SetViewpoint{ActorId: "pc-bram"},
			}})
			if err != nil {
				t.Fatalf("SendCommand(set_viewpoint): %v", err)
			}
			if !res.GetOk() {
				t.Fatalf("SendCommand(set_viewpoint) result = %v, want ok", res)
			}
			got := qaCSCollect(t, c)
			g.finish(t)
			qaCSAssertDeliveries(t, got, qaCSWanted(before, after))
		})
	}
}

func TestQACSAWorldSeatIsHandedNoSwitch(t *testing.T) {
	entries := []qaCSWire{
		qaCSEntry(qaCSNarration(6, "ev-0006", "Session six.")),
		qaCSPresenceChanged("p-ayla"),
		qaCSEntry(qaCSMoved(7, "ev-0007", "tok-ayla", 3)),
		qaCSEntry(qaCSMoved(8, "ev-0008", "tok-bram", 4)),
	}
	g := qaCSStart(t, qaCSSendAll([]qaCSWire{qaCSHead(8), qaCSPresence()}, entries))
	c := g.dial(t, 5, "")
	got := qaCSCollect(t, c)
	g.finish(t)
	qaCSAssertDeliveries(t, got, []harness.Delivery{
		{Envelope: qaCSNarration(6, "ev-0006", "Session six.")},
		{Envelope: qaCSMoved(7, "ev-0007", "tok-ayla", 3)},
		{Envelope: qaCSMoved(8, "ev-0008", "tok-bram", 4)},
	})
}

// VTT-427
func TestQACSALogOpenedFromItsFirstEntryBelowTheCursorIsHanded(t *testing.T) {
	steps := []qaCSWire{
		qaCSView("pc-bram"),
		qaCSEntry(qaCSSceneSeen(1, "sc-attic")),
		qaCSEntry(qaCSNarration(2, "ev-0030", "Bram wakes.")),
		qaCSEntry(qaCSMoved(3, "ev-0031", "tok-bram", 6)),
	}
	g := qaCSStart(t, qaCSSendAll([]qaCSWire{qaCSHead(3), qaCSPresence(), qaCSWatchable("pc-bram")}, steps))
	c := g.dial(t, 9, "pc-ayla")
	got := qaCSCollect(t, c)
	g.finish(t)
	qaCSAssertDeliveries(t, got, []harness.Delivery{
		{Viewpoint: &vttv1.ViewpointChanged{ActorId: "pc-bram"}},
		{Envelope: qaCSSceneSeen(1, "sc-attic")},
		{Envelope: qaCSNarration(2, "ev-0030", "Bram wakes.")},
		{Envelope: qaCSMoved(3, "ev-0031", "tok-bram", 6)},
	})
}
