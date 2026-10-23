package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"reflect"
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

const qacnWait = 5 * time.Second

const (
	qacnParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qacnNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qacnFrame struct {
	raw []byte
	f   *vttv1.ServerFrame
	err error
}

type qacnConn struct {
	t    *testing.T
	name string
	ws   *websocket.Conn
	ch   <-chan qacnFrame
}

type qacnTable struct {
	t   *testing.T
	c   *campaign.Campaign
	ids *identity.DB
	url string
	n   int
}

func qacnNewTable(t *testing.T) *qacnTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = c.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		srv.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qacnTable{t: t, c: c, ids: ids, url: srv.URL}
}

func (tb *qacnTable) invite(name string, role identity.Role) (token, id string) {
	tb.t.Helper()
	token, id, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatalf("CreateInvite(%s): %v", name, err)
	}
	return token, id
}

func (tb *qacnTable) env() *vttv1.Envelope {
	tb.n++
	return &vttv1.Envelope{
		EventId:    fmt.Sprintf("qacn-evt-%d", tb.n),
		OccurredAt: timestamppb.Now(),
		ActorRole:  string(identity.RoleDM),
	}
}

func (tb *qacnTable) added(id, name string, kind vttv1.ActorKind, controllers ...string) *vttv1.Envelope {
	e := tb.env()
	a := &vttv1.Actor{ActorId: id, Name: name, Kind: kind}
	if len(controllers) > 0 {
		a.ControllerIds = controllers
		a.ControllerId = controllers[0]
	}
	e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}
	return e
}

func (tb *qacnTable) granted(actorID, participantID string, kind vttv1.ActorKind) *vttv1.Envelope {
	e := tb.env()
	e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actorID, ParticipantId: participantID, Kind: kind,
	}}
	return e
}

func (tb *qacnTable) revoked(actorID, participantID string) *vttv1.Envelope {
	e := tb.env()
	e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: actorID, ParticipantId: participantID,
	}}
	return e
}

func (tb *qacnTable) removed(actorID string) *vttv1.Envelope {
	e := tb.env()
	e.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actorID}}
	return e
}

func (tb *qacnTable) narrated(text string) *vttv1.Envelope {
	e := tb.env()
	e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}
	return e
}

func (tb *qacnTable) append(envs ...*vttv1.Envelope) {
	tb.t.Helper()
	var err error
	if len(envs) == 1 {
		_, err = tb.c.Append(envs[0])
	} else {
		_, err = tb.c.AppendBatch(envs)
	}
	if err != nil {
		tb.t.Fatalf("append: %v", err)
	}
}

func (tb *qacnTable) step(marker string, envs []*vttv1.Envelope, conns ...*qacnConn) [][]qacnFrame {
	tb.t.Helper()
	tb.append(append(envs, tb.narrated(marker))...)
	out := make([][]qacnFrame, len(conns))
	for i, cn := range conns {
		out[i] = cn.until("narration "+marker, qacnIsNarration(marker))
	}
	return out
}

func (tb *qacnTable) dial(name, token, log string) *qacnConn {
	t := tb.t
	t.Helper()
	u := "ws" + strings.TrimPrefix(tb.url, "http") + "/ws?token=" + url.QueryEscape(token) + "&after=0"
	if log != "" {
		u += "&log=" + url.QueryEscape(log)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qacnWait)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", name, err)
	}
	ws.SetReadLimit(1 << 20)
	ch := make(chan qacnFrame, 1024)
	done := make(chan struct{})
	go func() {
		defer close(ch)
		for {
			_, data, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			uerr := protojson.Unmarshal(data, f)
			select {
			case ch <- qacnFrame{raw: data, f: f, err: uerr}:
			case <-done:
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(done)
		_ = ws.CloseNow()
	})
	return &qacnConn{t: t, name: name, ws: ws, ch: ch}
}

func (cn *qacnConn) until(what string, match func(*vttv1.ServerFrame) bool) []qacnFrame {
	cn.t.Helper()
	timer := time.NewTimer(qacnWait)
	defer timer.Stop()
	var got []qacnFrame
	for {
		select {
		case fr, ok := <-cn.ch:
			if !ok {
				cn.t.Fatalf("%s: connection closed waiting for %s; read %s", cn.name, what, qacnDescribe(got))
			}
			if fr.err != nil {
				cn.t.Fatalf("%s: undecodable frame %s: %v", cn.name, fr.raw, fr.err)
			}
			got = append(got, fr)
			if match(fr.f) {
				return got
			}
		case <-timer.C:
			cn.t.Fatalf("%s: no %s within %v; read %s", cn.name, what, qacnWait, qacnDescribe(got))
		}
	}
}

func (cn *qacnConn) open() []qacnFrame {
	cn.t.Helper()
	got := cn.until("the opening ViewpointChanged", func(f *vttv1.ServerFrame) bool { return f.GetViewpointChanged() != nil })
	head := got[0].f.GetCatchUpHead()
	if head == nil {
		cn.t.Fatalf("%s: first frame is not catch_up_head: %s", cn.name, got[0].raw)
	}
	if hs := head.GetHeadSequence(); hs > 0 {
		got = append(got, cn.until("the catch-up head", func(f *vttv1.ServerFrame) bool {
			return f.GetEvent() != nil && f.GetEvent().GetSequence() >= hs
		})...)
	}
	return got
}

func (cn *qacnConn) openWorld() []qacnFrame {
	cn.t.Helper()
	got := cn.until("the presence snapshot", func(f *vttv1.ServerFrame) bool { return f.GetPresenceSnapshot() != nil })
	head := got[0].f.GetCatchUpHead()
	if head == nil {
		cn.t.Fatalf("%s: first frame is not catch_up_head: %s", cn.name, got[0].raw)
	}
	if hs := head.GetHeadSequence(); hs > 0 {
		got = append(got, cn.until("the catch-up head", func(f *vttv1.ServerFrame) bool {
			return f.GetEvent() != nil && f.GetEvent().GetSequence() >= hs
		})...)
	}
	return got
}

func (cn *qacnConn) send(cmd *vttv1.ClientCommand) {
	cn.t.Helper()
	data, err := protojson.Marshal(cmd)
	if err != nil {
		cn.t.Fatalf("marshal command: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qacnWait)
	defer cancel()
	if err := cn.ws.Write(ctx, websocket.MessageText, data); err != nil {
		cn.t.Fatalf("%s: write: %v", cn.name, err)
	}
}

func qacnIsNarration(text string) func(*vttv1.ServerFrame) bool {
	return func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetNarrationAdded() != nil && f.GetEvent().GetNarrationAdded().GetText() == text
	}
}

func qacnDescribe(frames []qacnFrame) string {
	parts := make([]string, 0, len(frames))
	for _, fr := range frames {
		parts = append(parts, string(fr.raw))
	}
	return "[" + strings.Join(parts, " | ") + "]"
}

func qacnSet(none bool, pairs ...string) *vttv1.WatchableChanged {
	w := &vttv1.WatchableChanged{ControlsNoPartyMember: none}
	for i := 0; i+1 < len(pairs); i += 2 {
		w.Actors = append(w.Actors, &vttv1.WatchableActor{ActorId: pairs[i], Name: pairs[i+1]})
	}
	return w
}

func qacnWatchables(frames []qacnFrame) []qacnFrame {
	var out []qacnFrame
	for _, fr := range frames {
		if fr.f.GetWatchableChanged() != nil {
			out = append(out, fr)
		}
	}
	return out
}

func qacnFirst(frames []qacnFrame, pred func(*vttv1.ServerFrame) bool) int {
	for i, fr := range frames {
		if pred(fr.f) {
			return i
		}
	}
	return -1
}

func qacnIsWatchable(f *vttv1.ServerFrame) bool { return f.GetWatchableChanged() != nil }
func qacnIsEvent(f *vttv1.ServerFrame) bool     { return f.GetEvent() != nil }
func qacnIsViewpoint(f *vttv1.ServerFrame) bool { return f.GetViewpointChanged() != nil }

func qacnWire(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("frame is not JSON: %s: %v", raw, err)
	}
	return top
}

func qacnRawFlag(t *testing.T, raw []byte) bool {
	t.Helper()
	wc, ok := qacnWire(t, raw)["watchableChanged"].(map[string]any)
	if !ok {
		t.Fatalf("frame has no watchableChanged object: %s", raw)
	}
	if _, snake := wc["controls_no_party_member"]; snake {
		t.Errorf("frame names the field in snake case: %s", raw)
	}
	v, present := wc["controlsNoPartyMember"]
	if !present {
		return false
	}
	b, isBool := v.(bool)
	if !isBool {
		t.Errorf("controlsNoPartyMember is not a JSON bool: %s", raw)
	}
	return b
}

func qacnCheckSet(t *testing.T, who string, fr qacnFrame, want *vttv1.WatchableChanged) {
	t.Helper()
	got := fr.f.GetWatchableChanged()
	if !proto.Equal(got, want) {
		t.Errorf("%s: WatchableChanged = %s, want %s", who, fr.raw, protojson.Format(want))
	}
	if flag := qacnRawFlag(t, fr.raw); flag != want.GetControlsNoPartyMember() {
		t.Errorf("%s: wire controlsNoPartyMember = %v, want %v: %s", who, flag, want.GetControlsNoPartyMember(), fr.raw)
	}
}

func qacnCheckOpening(t *testing.T, who string, frames []qacnFrame, want *vttv1.WatchableChanged) {
	t.Helper()
	sets := qacnWatchables(frames)
	if len(sets) != 1 {
		t.Fatalf("%s: opening sent %d WatchableChanged, want 1: %s", who, len(sets), qacnDescribe(frames))
	}
	w := qacnFirst(frames, qacnIsWatchable)
	v := qacnFirst(frames, qacnIsViewpoint)
	if v < w {
		t.Errorf("%s: opening ViewpointChanged came before the WatchableChanged: %s", who, qacnDescribe(frames))
	}
	qacnCheckSet(t, who, sets[0], want)
}

func qacnCheckChange(t *testing.T, who string, frames []qacnFrame, want *vttv1.WatchableChanged) {
	t.Helper()
	sets := qacnWatchables(frames)
	if len(sets) != 1 {
		t.Fatalf("%s: the append sent %d WatchableChanged, want 1: %s", who, len(sets), qacnDescribe(frames))
	}
	w := qacnFirst(frames, qacnIsWatchable)
	if e := qacnFirst(frames, qacnIsEvent); e >= 0 && e < w {
		t.Errorf("%s: an entry of the append came before its WatchableChanged: %s", who, qacnDescribe(frames))
	}
	qacnCheckSet(t, who, sets[0], want)
}

func qacnCheckNoSet(t *testing.T, who string, frames []qacnFrame) {
	t.Helper()
	if sets := qacnWatchables(frames); len(sets) != 0 {
		t.Errorf("%s: sent %d WatchableChanged, want none: %s", who, len(sets), qacnDescribe(frames))
	}
}

func qacnCheckNoMove(t *testing.T, who string, frames []qacnFrame) {
	t.Helper()
	if v := qacnFirst(frames, qacnIsViewpoint); v >= 0 {
		t.Errorf("%s: the append moved the connection: %s", who, qacnDescribe(frames))
	}
}

// VTT-454 VTT-399
func TestQACNAPlayerWhoControlsAPartyMemberIsToldSo(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty),
		tb.added("pm-2", "Birch", qacnParty), tb.granted("pm-2", a, qacnParty),
		tb.added("pm-3", "Cedar", qacnParty),
		tb.added("np-1", "Goblin", qacnNonParty), tb.granted("np-1", a, qacnNonParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(false, "pm-2", "Birch"))
}

// VTT-454 VTT-452 VTT-399
func TestQACNAPlayerWhoControlsOnlyANonPartyActorIsToldTheyControlNone(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty),
		tb.added("pm-2", "Birch", qacnParty),
		tb.added("np-1", "Goblin", qacnNonParty), tb.granted("np-1", a, qacnNonParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(true, "pm-1", "Ash", "pm-2", "Birch"))
}

// VTT-454 VTT-452 VTT-399
func TestQACNAPlayerWhoControlsNothingIsToldTheyControlNone(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, _ := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty),
		tb.added("pm-2", "Birch", qacnParty),
		tb.added("np-1", "Goblin", qacnNonParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(true, "pm-1", "Ash", "pm-2", "Birch"))
}

// VTT-454 VTT-399
func TestQACNASpectatorWhoControlsNothingIsToldTheyControlNone(t *testing.T) {
	tb := qacnNewTable(t)
	sTok, _ := tb.invite("Sol", identity.RoleSpectator)
	_, a := tb.invite("Ada", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty),
		tb.added("pm-2", "Birch", qacnParty),
	)
	cn := tb.dial("Sol", sTok, "")
	qacnCheckOpening(t, "Sol", cn.open(), qacnSet(true, "pm-1", "Ash", "pm-2", "Birch"))
}

// VTT-454 VTT-399
func TestQACNASpectatorWhoControlsAPartyMemberIsToldSo(t *testing.T) {
	tb := qacnNewTable(t)
	sTok, s := tb.invite("Sol", identity.RoleSpectator)
	_, a := tb.invite("Ada", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty),
		tb.added("pm-2", "Birch", qacnParty), tb.granted("pm-2", s, qacnParty),
		tb.added("pm-3", "Cedar", qacnParty),
	)
	cn := tb.dial("Sol", sTok, "")
	qacnCheckOpening(t, "Sol", cn.open(), qacnSet(false, "pm-1", "Ash", "pm-2", "Birch", "pm-3", "Cedar"))
}

// VTT-399 VTT-454
func TestQACNAnEmptySetSaysOnTheWireThatTheyControlNone(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	sTok, _ := tb.invite("Sol", identity.RoleSpectator)
	tb.append(tb.added("np-1", "Goblin", qacnNonParty), tb.granted("np-1", a, qacnNonParty))
	want := map[string]any{"watchableChanged": map[string]any{"controlsNoPartyMember": true}}
	for _, who := range []struct{ name, token string }{{"Ada", aTok}, {"Sol", sTok}} {
		frames := tb.dial(who.name, who.token, "").open()
		sets := qacnWatchables(frames)
		if len(sets) != 1 {
			t.Fatalf("%s: opening sent %d WatchableChanged, want 1: %s", who.name, len(sets), qacnDescribe(frames))
		}
		if got := qacnWire(t, sets[0].raw); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: empty set on the wire = %s, want {\"watchableChanged\":{\"controlsNoPartyMember\":true}}", who.name, sets[0].raw)
		}
	}
}

// VTT-455 VTT-454 VTT-456
func TestQACNRevokingATablesOnlyPartyMemberSendsTheSetAgain(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty),
		tb.added("np-1", "Goblin", qacnNonParty), tb.granted("np-1", a, qacnNonParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(false, "pm-1", "Ash"))
	got := tb.step("after-revoke", []*vttv1.Envelope{tb.revoked("pm-1", a)}, cn)[0]
	qacnCheckChange(t, "Ada", got, qacnSet(true, "pm-1", "Ash"))
	qacnCheckNoMove(t, "Ada", got)
}

// VTT-455 VTT-454
func TestQACNGrantingTheOnlyPartyMemberToAPlayerWhoControlsNoneSendsTheSetAgain(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	tb.append(tb.added("pm-1", "Ash", qacnParty))
	cn := tb.dial("Ada", aTok, "pm-1")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(true, "pm-1", "Ash"))
	got := tb.step("after-grant", []*vttv1.Envelope{tb.granted("pm-1", a, qacnParty)}, cn)[0]
	qacnCheckChange(t, "Ada", got, qacnSet(false, "pm-1", "Ash"))
}

// VTT-455 VTT-454
func TestQACNASpectatorsControlChangeSendsTheSetAgain(t *testing.T) {
	tb := qacnNewTable(t)
	sTok, s := tb.invite("Sol", identity.RoleSpectator)
	_, a := tb.invite("Ada", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty),
		tb.added("pm-2", "Birch", qacnParty),
	)
	cn := tb.dial("Sol", sTok, "pm-1")
	qacnCheckOpening(t, "Sol", cn.open(), qacnSet(true, "pm-1", "Ash", "pm-2", "Birch"))
	got := tb.step("after-grant", []*vttv1.Envelope{tb.granted("pm-2", s, qacnParty)}, cn)[0]
	qacnCheckChange(t, "Sol after the grant", got, qacnSet(false, "pm-1", "Ash", "pm-2", "Birch"))
	got = tb.step("after-revoke", []*vttv1.Envelope{tb.revoked("pm-2", s)}, cn)[0]
	qacnCheckChange(t, "Sol after the revocation", got, qacnSet(true, "pm-1", "Ash", "pm-2", "Birch"))
}

// VTT-454 VTT-400 VTT-456
func TestQACNRevokingAPlayersPartyMemberKeepsThemWatchingItAndSaysTheyControlNone(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty),
		tb.added("pm-2", "Birch", qacnParty), tb.granted("pm-2", a, qacnParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(false, "pm-2", "Birch"))
	got := tb.step("after-revoke", []*vttv1.Envelope{tb.revoked("pm-2", a)}, cn)[0]
	qacnCheckChange(t, "Ada", got, qacnSet(true, "pm-1", "Ash", "pm-2", "Birch"))
	qacnCheckNoMove(t, "Ada", got)
}

// VTT-454 VTT-400
func TestQACNLosingOneOfTwoPartyMembersStillSaysTheyControlOne(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty),
		tb.added("pm-2", "Birch", qacnParty), tb.granted("pm-2", a, qacnParty),
		tb.added("pm-3", "Cedar", qacnParty), tb.granted("pm-3", b, qacnParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(false, "pm-1", "Ash", "pm-2", "Birch"))
	got := tb.step("after-revoke", []*vttv1.Envelope{tb.revoked("pm-2", a)}, cn)[0]
	qacnCheckChange(t, "Ada", got, qacnSet(false, "pm-1", "Ash"))
}

// VTT-454
func TestQACNANonPartyGrantLeavesAPlayerControllingNone(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty))
	cn := tb.dial("Ada", aTok, "pm-1")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(true, "pm-1", "Ash"))
	got := tb.step("after-grant", []*vttv1.Envelope{
		tb.added("np-1", "Goblin", qacnNonParty), tb.granted("np-1", a, qacnNonParty),
	}, cn)[0]
	for _, fr := range qacnWatchables(got) {
		qacnCheckSet(t, "Ada after the non-party grant", fr, qacnSet(true, "pm-1", "Ash"))
	}
	qacnCheckNoSet(t, "Ada after the non-party grant", got)
	again := tb.dial("Ada again", aTok, "pm-1")
	qacnCheckOpening(t, "Ada again", again.open(), qacnSet(true, "pm-1", "Ash"))
}

// VTT-454 VTT-400 VTT-340
func TestQACNMakingTheirPartyMemberNonPartySaysTheyControlNone(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty),
		tb.added("pm-2", "Birch", qacnParty), tb.granted("pm-2", a, qacnParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(false, "pm-2", "Birch"))
	tb.append(tb.granted("pm-2", a, qacnNonParty))
	got := cn.until("the move off pm-2", qacnIsViewpoint)
	qacnCheckChange(t, "Ada", got, qacnSet(true, "pm-1", "Ash"))
	if id := got[len(got)-1].f.GetViewpointChanged().GetActorId(); id != "" {
		t.Errorf("Ada moved to %q, want nobody", id)
	}
}

// VTT-454 VTT-400
func TestQACNRemovingTheirPartyMemberSaysTheyControlNone(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty),
		tb.added("pm-2", "Birch", qacnParty), tb.granted("pm-2", a, qacnParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(false, "pm-2", "Birch"))
	tb.append(tb.removed("pm-2"))
	got := cn.until("the move off pm-2", qacnIsViewpoint)
	qacnCheckChange(t, "Ada", got, qacnSet(true, "pm-1", "Ash"))
}

// VTT-454 VTT-400 VTT-340
func TestQACNARegrantMakingTheirNonPartyActorAPartyMemberSaysTheyControlOne(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	_, b := tb.invite("Bo", identity.RolePlayer)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", b, qacnParty),
		tb.added("cr-1", "Wolf", qacnNonParty), tb.granted("cr-1", a, qacnNonParty),
	)
	cn := tb.dial("Ada", aTok, "pm-1")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(true, "pm-1", "Ash"))
	tb.append(tb.granted("cr-1", a, qacnParty))
	got := cn.until("the move onto cr-1", qacnIsViewpoint)
	qacnCheckChange(t, "Ada", got, qacnSet(false, "cr-1", "Wolf"))
	if id := got[len(got)-1].f.GetViewpointChanged().GetActorId(); id != "cr-1" {
		t.Errorf("Ada moved to %q, want cr-1", id)
	}
}

// VTT-455
func TestQACNEveryConnectionOfThePlayerIsSentTheChange(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	tb.append(tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty))
	one := tb.dial("Ada's first", aTok, "")
	qacnCheckOpening(t, "Ada's first", one.open(), qacnSet(false, "pm-1", "Ash"))
	two := tb.dial("Ada's second", aTok, "pm-1")
	qacnCheckOpening(t, "Ada's second", two.open(), qacnSet(false, "pm-1", "Ash"))
	got := tb.step("after-revoke", []*vttv1.Envelope{tb.revoked("pm-1", a)}, one, two)
	qacnCheckChange(t, "Ada's first", got[0], qacnSet(true, "pm-1", "Ash"))
	qacnCheckChange(t, "Ada's second", got[1], qacnSet(true, "pm-1", "Ash"))
}

// VTT-401 VTT-455
func TestQACNTheDMAndTheAgentAreSentNoSetAsControlChanges(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	dTok, _ := tb.invite("Dee", identity.RoleDM)
	gTok, _ := tb.invite("Gus", identity.RoleAgent)
	tb.append(tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty))
	player := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", player.open(), qacnSet(false, "pm-1", "Ash"))
	dm := tb.dial("Dee", dTok, "")
	agent := tb.dial("Gus", gTok, "")
	dmSeen := dm.openWorld()
	agentSeen := agent.openWorld()
	got := tb.step("after-revoke", []*vttv1.Envelope{tb.revoked("pm-1", a)}, player, dm, agent)
	qacnCheckChange(t, "Ada", got[0], qacnSet(true, "pm-1", "Ash"))
	dmSeen = append(dmSeen, got[1]...)
	agentSeen = append(agentSeen, got[2]...)
	got = tb.step("after-grant", []*vttv1.Envelope{tb.granted("pm-1", a, qacnParty)}, player, dm, agent)
	qacnCheckChange(t, "Ada", got[0], qacnSet(false, "pm-1", "Ash"))
	qacnCheckNoSet(t, "Dee", append(dmSeen, got[1]...))
	qacnCheckNoSet(t, "Gus", append(agentSeen, got[2]...))
}

// VTT-455 VTT-454
func TestQACNAGrantAndItsRevocationInOneAppendSendNoSet(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	tb.append(tb.added("pm-1", "Ash", qacnParty))
	cn := tb.dial("Ada", aTok, "pm-1")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(true, "pm-1", "Ash"))
	got := tb.step("after-both", []*vttv1.Envelope{tb.granted("pm-1", a, qacnParty), tb.revoked("pm-1", a)}, cn)[0]
	qacnCheckNoSet(t, "Ada after a grant and its revocation", got)
	got = tb.step("after-grant", []*vttv1.Envelope{tb.granted("pm-1", a, qacnParty)}, cn)[0]
	qacnCheckChange(t, "Ada after the grant alone", got, qacnSet(false, "pm-1", "Ash"))
}

// VTT-454 VTT-400
func TestQACNMakingAPlayerASpectatorKeepsSayingTheyControlOne(t *testing.T) {
	tb := qacnNewTable(t)
	aTok, a := tb.invite("Ada", identity.RolePlayer)
	dTok, _ := tb.invite("Dee", identity.RoleDM)
	tb.append(
		tb.added("pm-1", "Ash", qacnParty), tb.granted("pm-1", a, qacnParty),
		tb.added("pm-2", "Birch", qacnParty),
	)
	cn := tb.dial("Ada", aTok, "")
	qacnCheckOpening(t, "Ada", cn.open(), qacnSet(false, "pm-1", "Ash"))
	dm := tb.dial("Dee", dTok, "")
	dm.openWorld()
	dm.send(&vttv1.ClientCommand{
		RequestId: "qacn-promote",
		Command: &vttv1.ClientCommand_PromoteParticipant{PromoteParticipant: &vttv1.PromoteParticipant{
			ParticipantId: a, Role: string(identity.RoleSpectator),
		}},
	})
	res := dm.until("the promotion's result", func(f *vttv1.ServerFrame) bool {
		return f.GetResult().GetRequestId() == "qacn-promote"
	})
	if r := res[len(res)-1].f.GetResult(); !r.GetOk() {
		t.Fatalf("promotion refused: %s", r.GetError())
	}
	got := cn.until("the spectator's set", qacnIsWatchable)
	qacnCheckSet(t, "Ada as a spectator", got[len(got)-1], qacnSet(false, "pm-1", "Ash", "pm-2", "Birch"))
}
