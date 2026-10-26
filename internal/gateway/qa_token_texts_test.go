package gateway_test

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

type qattTable struct {
	c       *campaign.Campaign
	srv     *httptest.Server
	dmToken string
}

func qattEnv(id string) *vttv1.Envelope {
	return &vttv1.Envelope{EventId: id}
}

func qattWorld() []*vttv1.Envelope {
	sc := qattEnv("qatt-e1")
	sc.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "s1", Name: "Crypt", GridWidth: 10, GridHeight: 10}}
	ac := qattEnv("qatt-e2")
	ac.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "a1", Name: "Goblin"}}}
	t1 := qattEnv("qatt-e3")
	t1.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "t1", SceneId: "s1", ActorId: "a1", Position: &vttv1.GridPosition{X: 1, Y: 2},
		Name: "Grub", Notes: "Limps.", GmName: "Grub the Spy", GmNotes: "Reports to the lich.",
	}}
	return []*vttv1.Envelope{sc, ac, t1}
}

func qattOpen(t *testing.T, world []*vttv1.Envelope) *qattTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	for _, e := range world {
		if _, err := c.Append(e); err != nil {
			t.Fatalf("Append %s: %v", e.GetEventId(), err)
		}
	}
	ids, err := identity.Open(filepath.Join(dir, "qatt-identity.db"))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	tok, _, err := ids.CreateInvite("Dee", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		srv.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qattTable{c: c, srv: srv, dmToken: tok}
}

func (tb *qattTable) wsURL() string {
	return "ws" + strings.TrimPrefix(tb.srv.URL, "http")
}

type qattConn struct {
	t    *testing.T
	ws   *websocket.Conn
	head int64
	got  []*vttv1.ServerFrame
}

func (tb *qattTable) dial(t *testing.T) *qattConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, tb.wsURL()+"/ws?token="+tb.dmToken+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	qc := &qattConn{t: t, ws: ws}
	first := qc.next()
	h, ok := first.GetFrame().(*vttv1.ServerFrame_CatchUpHead)
	if !ok {
		t.Fatalf("first frame is %T, want catch_up_head", first.GetFrame())
	}
	qc.head = h.CatchUpHead.GetHeadSequence()
	return qc
}

func (qc *qattConn) next() *vttv1.ServerFrame {
	qc.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, raw, err := qc.ws.Read(ctx)
	if err != nil {
		qc.t.Fatalf("read: %v", err)
	}
	f := &vttv1.ServerFrame{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, f); err != nil {
		qc.t.Fatalf("decode frame %s: %v", raw, err)
	}
	qc.got = append(qc.got, f)
	return f
}

func (qc *qattConn) send(cmd *vttv1.ClientCommand) {
	qc.t.Helper()
	b, err := protojson.Marshal(cmd)
	if err != nil {
		qc.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := qc.ws.Write(ctx, websocket.MessageText, b); err != nil {
		qc.t.Fatalf("write: %v", err)
	}
}

func (qc *qattConn) result(requestID string) *vttv1.CommandResult {
	qc.t.Helper()
	for _, f := range qc.got {
		if r, ok := f.GetFrame().(*vttv1.ServerFrame_Result); ok && r.Result.GetRequestId() == requestID {
			return r.Result
		}
	}
	for {
		f := qc.next()
		if r, ok := f.GetFrame().(*vttv1.ServerFrame_Result); ok && r.Result.GetRequestId() == requestID {
			return r.Result
		}
	}
}

func (qc *qattConn) event(seq int64) *vttv1.Envelope {
	qc.t.Helper()
	for _, f := range qc.got {
		if e, ok := f.GetFrame().(*vttv1.ServerFrame_Event); ok && e.Event.GetSequence() == seq {
			return e.Event
		}
	}
	for {
		f := qc.next()
		if e, ok := f.GetFrame().(*vttv1.ServerFrame_Event); ok && e.Event.GetSequence() == seq {
			return e.Event
		}
	}
}

func qattDescribeCmd(reqID, tokenID, name, notes, gmName, gmNotes string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: reqID, Command: &vttv1.ClientCommand_DescribeToken{DescribeToken: &vttv1.DescribeToken{
		TokenId: tokenID, Name: name, Notes: notes, GmName: gmName, GmNotes: gmNotes,
	}}}
}

// VTT-479
func TestQATTALiveDescribeAppendsOneEventAndChangesTheToken(t *testing.T) {
	tb := qattOpen(t, qattWorld())
	dm := tb.dial(t)
	if dm.head != 3 {
		t.Fatalf("catch-up head = %d, want 3", dm.head)
	}
	dm.send(qattDescribeCmd("qatt-r1", "t1", "", "Now runs.", "", "Turned."))
	res := dm.result("qatt-r1")
	if !res.GetOk() {
		t.Fatalf("describe refused: %q", res.GetError())
	}
	if res.GetSequence() != 4 {
		t.Fatalf("result sequence = %d, want 4", res.GetSequence())
	}
	got := tb.c.State().Tokens["t1"]
	want := engine.Token{ID: "t1", SceneID: "s1", ActorID: "a1", X: 1, Y: 2, Notes: "Now runs.", GMNotes: "Turned."}
	if got != want {
		t.Fatalf("token after describe = %+v, want %+v", got, want)
	}
	if head := tb.dial(t).head; head != 4 {
		t.Fatalf("log head after one describe = %d, want 4", head)
	}
}

// VTT-503
func TestQATTALiveDescribeEventCarriesEveryFieldTheCommandGave(t *testing.T) {
	tb := qattOpen(t, qattWorld())
	dm := tb.dial(t)
	dm.send(qattDescribeCmd("qatt-r2", "t1", "N", "No", "G", "Gn"))
	if res := dm.result("qatt-r2"); !res.GetOk() {
		t.Fatalf("describe refused: %q", res.GetError())
	}
	ev := dm.event(4).GetTokenDescribed()
	want := &vttv1.TokenDescribed{TokenId: "t1", Name: "N", Notes: "No", GmName: "G", GmNotes: "Gn"}
	if !proto.Equal(ev, want) {
		t.Fatalf("event = %v, want %v", ev, want)
	}
}

// VTT-480
func TestQATTALiveDescribeNamingNoTokenIsRefusedAndAppendsNothing(t *testing.T) {
	tb := qattOpen(t, qattWorld())
	dm := tb.dial(t)
	before := tb.c.State().Tokens
	dm.send(qattDescribeCmd("qatt-r3", "ghost", "X", "X", "X", "X"))
	res := dm.result("qatt-r3")
	if res.GetOk() {
		t.Fatal("describe of no token was accepted")
	}
	if res.GetError() == "" {
		t.Fatal("refusal carries no error")
	}
	if res.GetSequence() != 0 {
		t.Fatalf("refusal carries sequence %d", res.GetSequence())
	}
	if head := tb.dial(t).head; head != 3 {
		t.Fatalf("log head after a refused describe = %d, want 3", head)
	}
	after := tb.c.State().Tokens
	if len(after) != len(before) || after["t1"] != before["t1"] {
		t.Fatalf("tokens changed: before %+v after %+v", before, after)
	}
}

// VTT-503
func TestQATTToEventCarriesEveryFieldOfADescribe(t *testing.T) {
	dm := &identity.Participant{ID: "p-dm", Name: "Dee", Role: identity.RoleDM}
	env, err := gateway.ToEvent(qattDescribeCmd("r", "t1", "N", "No", "G", "Gn"), dm)
	if err != nil {
		t.Fatalf("ToEvent: %v", err)
	}
	want := &vttv1.TokenDescribed{TokenId: "t1", Name: "N", Notes: "No", GmName: "G", GmNotes: "Gn"}
	if !proto.Equal(env.GetTokenDescribed(), want) {
		t.Fatalf("event = %v, want %v", env.GetTokenDescribed(), want)
	}
}

// VTT-503
func TestQATTToEventCarriesThePlacementsFourTexts(t *testing.T) {
	dm := &identity.Participant{ID: "p-dm", Name: "Dee", Role: identity.RoleDM}
	cmd := &vttv1.ClientCommand{RequestId: "r", Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
		TokenId: "t9", SceneId: "s1", ActorId: "a1", Position: &vttv1.GridPosition{X: 4, Y: 5},
		Name: "N", Notes: "No", GmName: "G", GmNotes: "Gn",
	}}}
	env, err := gateway.ToEvent(cmd, dm)
	if err != nil {
		t.Fatalf("ToEvent: %v", err)
	}
	want := &vttv1.TokenPlaced{TokenId: "t9", SceneId: "s1", ActorId: "a1", Position: &vttv1.GridPosition{X: 4, Y: 5},
		Name: "N", Notes: "No", GmName: "G", GmNotes: "Gn"}
	if !proto.Equal(env.GetTokenPlaced(), want) {
		t.Fatalf("event = %v, want %v", env.GetTokenPlaced(), want)
	}
}

// VTT-463
func TestQATTAPlayersAndASpectatorsDescribeAreRefused(t *testing.T) {
	st := engine.NewState()
	for _, e := range qattWorld() {
		if err := engine.Apply(st, e); err != nil {
			t.Fatal(err)
		}
	}
	cmd := qattDescribeCmd("r", "t1", "N", "", "", "")
	for _, role := range []identity.Role{identity.RolePlayer, identity.RoleSpectator} {
		if err := gateway.Authorize(&identity.Participant{ID: "p", Name: "P", Role: role}, cmd, st); err == nil {
			t.Errorf("%s's describe_token was authorized", role)
		}
	}
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		if err := gateway.Authorize(&identity.Participant{ID: "p", Name: "P", Role: role}, cmd, st); err != nil {
			t.Errorf("%s's describe_token was refused: %v", role, err)
		}
	}
}
