package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaCmdBound = 128

type qaCmdTable struct {
	srv         *httptest.Server
	camp        *campaign.Campaign
	dmToken     string
	obsToken    string
	playerToken string
	playerID    string
}

func qaCmdServe(t *testing.T, configure func(*gateway.Server) *gateway.Server) *qaCmdTable {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = c.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	srv := gateway.New(c, ids)
	if configure != nil {
		srv = configure(srv)
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		hs.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	tb := &qaCmdTable{srv: hs, camp: c}
	if tb.dmToken, _, err = ids.CreateInvite("qa dm", identity.RoleDM); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if tb.obsToken, _, err = ids.CreateInvite("qa observer", identity.RoleDM); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if tb.playerToken, tb.playerID, err = ids.CreateInvite("qa player", identity.RolePlayer); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	return tb
}

type qaCmdConn struct {
	t       *testing.T
	ws      *websocket.Conn
	head    int64
	events  []*vttv1.Envelope
	results map[string]*vttv1.CommandResult
}

func qaCmdDial(t *testing.T, tb *qaCmdTable, token string) *qaCmdConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(tb.srv.URL, "http") + "/ws?token=" + token + "&after=0"
	ws, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ws.SetReadLimit(1 << 22)
	qc := &qaCmdConn{t: t, ws: ws, results: map[string]*vttv1.CommandResult{}}
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	first := qc.read()
	if first.GetCatchUpHead() == nil {
		t.Fatalf("first frame is not catch_up_head: %v", first)
	}
	qc.head = first.GetCatchUpHead().GetHeadSequence()
	return qc
}

func (qc *qaCmdConn) read() *vttv1.ServerFrame {
	qc.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, data, err := qc.ws.Read(ctx)
	if err != nil {
		qc.t.Fatalf("read: %v", err)
	}
	f := &vttv1.ServerFrame{}
	if err := protojson.Unmarshal(data, f); err != nil {
		qc.t.Fatalf("decode frame %s: %v", data, err)
	}
	if ev := f.GetEvent(); ev != nil {
		qc.events = append(qc.events, ev)
	}
	if r := f.GetResult(); r != nil {
		qc.results[r.GetRequestId()] = r
	}
	return f
}

func (qc *qaCmdConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	qc.t.Helper()
	data, err := protojson.Marshal(cmd)
	if err != nil {
		qc.t.Fatalf("encode: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := qc.ws.Write(ctx, websocket.MessageText, data); err != nil {
		qc.t.Fatalf("write: %v", err)
	}
	for {
		if r, ok := qc.results[cmd.GetRequestId()]; ok {
			return r
		}
		qc.read()
	}
}

func (qc *qaCmdConn) find(match func(*vttv1.Envelope) bool) *vttv1.Envelope {
	qc.t.Helper()
	for seen := 0; ; {
		for ; seen < len(qc.events); seen++ {
			if match(qc.events[seen]) {
				return qc.events[seen]
			}
		}
		qc.read()
	}
}

func (qc *qaCmdConn) event(seq int64) *vttv1.Envelope {
	qc.t.Helper()
	return qc.find(func(e *vttv1.Envelope) bool { return e.GetSequence() == seq })
}

func qaCmdActorCmd(req string, a *vttv1.Actor) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: a},
	}}
}

func qaCmdPlainActor(id string) *vttv1.Actor {
	return &vttv1.Actor{ActorId: id, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}
}

func qaCmdFollow(n int) *vttv1.ClientCommand {
	return qaCmdActorCmd(fmt.Sprintf("follow-%d", n), qaCmdPlainActor(fmt.Sprintf("qa-follow-%d", n)))
}

func qaCmdGrantCmd(req, actorID, participantID string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_GrantActorControl{
		GrantActorControl: &vttv1.GrantActorControl{ActorId: actorID, ParticipantId: participantID, Kind: kind},
	}}
}

func qaCmdRevokeCmd(req, actorID, participantID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{ActorId: actorID, ParticipantId: participantID},
	}}
}

func qaCmdLoadMap(req, mapID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_LoadMap{
		LoadMap: &vttv1.LoadMap{MapId: mapID},
	}}
}

func qaCmdOK(t *testing.T, qc *qaCmdConn, cmd *vttv1.ClientCommand, wantSeq int64) *vttv1.CommandResult {
	t.Helper()
	res := qc.send(cmd)
	if !res.GetOk() {
		t.Fatalf("%s refused: %q", cmd.GetRequestId(), res.GetError())
	}
	if wantSeq != 0 && res.GetSequence() != wantSeq {
		t.Fatalf("%s landed at %d, want %d", cmd.GetRequestId(), res.GetSequence(), wantSeq)
	}
	return res
}

func qaCmdStillOpenNothingAppended(t *testing.T, tb *qaCmdTable, issuer, observer *qaCmdConn, n int, h0 int64) {
	t.Helper()
	next := issuer.send(qaCmdFollow(n))
	if !next.GetOk() {
		t.Fatalf("the connection did not take the next command: %q", next.GetError())
	}
	if next.GetSequence() != h0+1 {
		t.Fatalf("next command landed at %d, want %d: the refusal appended", next.GetSequence(), h0+1)
	}
	if observer.event(h0+1).GetEventId() != issuer.event(h0+1).GetEventId() {
		t.Fatalf("observer's event %d is not the follow-up", h0+1)
	}
	for i, e := range observer.events {
		if e.GetSequence() != int64(i+1) {
			t.Fatalf("observer received sequence %d at position %d", e.GetSequence(), i)
		}
	}
	if fresh := qaCmdDial(t, tb, tb.obsToken); fresh.head != h0+1 {
		t.Fatalf("log head is %d, want %d", fresh.head, h0+1)
	}
}

func qaCmdFoldText(t *testing.T, st *engine.State, env *vttv1.Envelope) string {
	t.Helper()
	err := engine.Apply(st, env)
	if err == nil {
		t.Fatalf("engine.Apply accepted the event the test expects it to refuse")
	}
	return err.Error()
}

func qaCmdKeys(keys ...string) map[string]*vttv1.Resource {
	out := map[string]*vttv1.Resource{}
	for _, k := range keys {
		out[k] = &vttv1.Resource{Current: 1, Max: 2}
	}
	return out
}

func qaCmdAttrs(keys ...string) map[string]int32 {
	out := map[string]int32{}
	for _, k := range keys {
		out[k] = 3
	}
	return out
}

// VTT-161 VTT-162 VTT-290 VTT-297
func TestQACmdAddActorOverTheBoundIsRefusedOnTheWireWithTheFoldsText(t *testing.T) {
	cases := []struct {
		label string
		edit  func(*vttv1.Actor)
	}{
		{"module id of 65 two-byte runes", func(a *vttv1.Actor) { a.ModuleId = strings.Repeat("é", 65) }},
		{"resource name of 129 bytes", func(a *vttv1.Actor) {
			a.Resources = qaCmdKeys("focus", strings.Repeat("😀", 32)+"a")
		}},
		{"attribute name of 129 ascii", func(a *vttv1.Actor) { a.Attributes = qaCmdAttrs("vim", strings.Repeat("a", 129)) }},
		{"empty resource name", func(a *vttv1.Actor) { a.Resources = qaCmdKeys("focus", "") }},
		{"empty attribute name", func(a *vttv1.Actor) { a.Attributes = qaCmdAttrs("", "vim") }},
	}
	tb := qaCmdServe(t, nil)
	issuer := qaCmdDial(t, tb, tb.dmToken)
	observer := qaCmdDial(t, tb, tb.obsToken)
	for n, c := range cases {
		a := qaCmdPlainActor(fmt.Sprintf("qa-actor-%d", n))
		c.edit(a)
		want := qaCmdFoldText(t, tb.camp.State(), &vttv1.Envelope{
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}},
		})
		res := issuer.send(qaCmdActorCmd(fmt.Sprintf("bad-%d", n), a))
		t.Logf("%s: %s", c.label, res.GetError())
		if res.GetOk() || res.GetError() != want {
			t.Fatalf("%s: ok=%v err=%q, want the fold's %q", c.label, res.GetOk(), res.GetError(), want)
		}
		qaCmdStillOpenNothingAppended(t, tb, issuer, observer, n, int64(n))
	}
}

// VTT-290
func TestQACmdAddActorAtTheBoundIsRecordedWhole(t *testing.T) {
	tb := qaCmdServe(t, nil)
	dm := qaCmdDial(t, tb, tb.dmToken)
	observer := qaCmdDial(t, tb, tb.obsToken)
	module, resource, attribute := strings.Repeat("😀", 32), strings.Repeat("é", 64), strings.Repeat("€", 42)+"ab"
	a := qaCmdPlainActor("qa-edge")
	a.ModuleId, a.Resources, a.Attributes = module, qaCmdKeys("focus", resource), qaCmdAttrs(attribute, "vim")
	res := qaCmdOK(t, dm, qaCmdActorCmd("edge", a), 1)
	got := observer.event(res.GetSequence()).GetActorAdded().GetActor()
	if got.GetModuleId() != module {
		t.Errorf("recorded module id is %d bytes, sent %d", len(got.GetModuleId()), len(module))
	}
	if _, ok := got.GetResources()[resource]; !ok || len(got.GetResources()) != 2 {
		t.Errorf("recorded resources do not hold the %d-byte name", len(resource))
	}
	if _, ok := got.GetAttributes()[attribute]; !ok || len(got.GetAttributes()) != 2 {
		t.Errorf("recorded attributes do not hold the %d-byte name", len(attribute))
	}
}

func qaCmdControlEvent(grant bool, actorID, participantID string) *vttv1.Envelope {
	if grant {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
			ActorControlGranted: &vttv1.ActorControlGranted{
				ActorId: actorID, ParticipantId: participantID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			},
		}}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{
		ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: actorID, ParticipantId: participantID},
	}}
}

// VTT-161 VTT-162 VTT-289
func TestQACmdControlOverTheBoundIsRefusedOnTheWireWithTheFoldsText(t *testing.T) {
	cases := []struct {
		label, participant string
		grant              bool
	}{
		{"grant of 129 ascii", strings.Repeat("a", 129), true},
		{"grant of 65 two-byte runes", strings.Repeat("é", 65), true},
		{"revoke of 32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a", false},
		{"revoke of 43 three-byte runes", strings.Repeat("€", 43), false},
	}
	tb := qaCmdServe(t, nil)
	issuer := qaCmdDial(t, tb, tb.dmToken)
	observer := qaCmdDial(t, tb, tb.obsToken)
	qaCmdOK(t, issuer, qaCmdActorCmd("actor", qaCmdPlainActor("qa-actor")), 1)
	for n, c := range cases {
		want := qaCmdFoldText(t, tb.camp.State(), qaCmdControlEvent(c.grant, "qa-actor", c.participant))
		cmd := qaCmdRevokeCmd(fmt.Sprintf("bad-%d", n), "qa-actor", c.participant)
		if c.grant {
			cmd = qaCmdGrantCmd(fmt.Sprintf("bad-%d", n), "qa-actor", c.participant, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)
		}
		res := issuer.send(cmd)
		t.Logf("%s: %s", c.label, res.GetError())
		if res.GetOk() || res.GetError() != want {
			t.Fatalf("%s: ok=%v err=%q, want the fold's %q", c.label, res.GetOk(), res.GetError(), want)
		}
		qaCmdStillOpenNothingAppended(t, tb, issuer, observer, n, int64(n+1))
	}
}

// VTT-289
func TestQACmdControlAtTheBoundIsRecordedWhole(t *testing.T) {
	tb := qaCmdServe(t, nil)
	dm := qaCmdDial(t, tb, tb.dmToken)
	observer := qaCmdDial(t, tb, tb.obsToken)
	qaCmdOK(t, dm, qaCmdActorCmd("actor", qaCmdPlainActor("qa-actor")), 1)
	for n, p := range []string{"S" + strings.Repeat("a", 126) + "E", strings.Repeat("é", 64), strings.Repeat("😀", 32)} {
		seq := int64(2*n + 2)
		res := qaCmdOK(t, dm, qaCmdGrantCmd(fmt.Sprintf("grant-%d", n), "qa-actor", p,
			vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER), seq)
		if got := observer.event(res.GetSequence()).GetActorControlGranted().GetParticipantId(); got != p {
			t.Fatalf("recorded grant's participant id is %d bytes, sent %d", len(got), len(p))
		}
		if !slices.Contains(tb.camp.State().Actors["qa-actor"].GetControllerIds(), p) {
			t.Fatalf("the campaign's actor is not controlled by the %d-byte participant", len(p))
		}
		res = qaCmdOK(t, dm, qaCmdRevokeCmd(fmt.Sprintf("revoke-%d", n), "qa-actor", p), seq+1)
		if got := observer.event(res.GetSequence()).GetActorControlRevoked().GetParticipantId(); got != p {
			t.Fatalf("recorded revoke's participant id is %d bytes, sent %d", len(got), len(p))
		}
	}
}

// VTT-161
func TestQACmdTheValidatorsRefuseBeforeTheFoldMeasures(t *testing.T) {
	tb := qaCmdServe(t, nil)
	dm := qaCmdDial(t, tb, tb.dmToken)
	qaCmdOK(t, dm, qaCmdActorCmd("actor", qaCmdPlainActor("qa-actor")), 1)
	long := strings.Repeat("a", qaCmdBound+1)
	unkinded := func(req string, edit func(*vttv1.Actor)) *vttv1.ClientCommand {
		a := qaCmdPlainActor("qa-new")
		a.Kind = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
		edit(a)
		return qaCmdActorCmd(req, a)
	}
	controlled := func(req string, edit func(*vttv1.Actor)) *vttv1.ClientCommand {
		a := qaCmdPlainActor("qa-new")
		a.ControllerIds = []string{"qa-participant"}
		edit(a)
		return qaCmdActorCmd(req, a)
	}
	none := func(*vttv1.Actor) {}
	cases := []struct {
		label       string
		short, long *vttv1.ClientCommand
	}{
		{"grant with no kind", qaCmdGrantCmd("g1", "qa-actor", "qa-p", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED),
			qaCmdGrantCmd("g2", "qa-actor", long, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)},
		{"add_actor with no kind", unkinded("k1", none),
			unkinded("k2", func(a *vttv1.Actor) { a.ModuleId = long })},
		{"add_actor naming a controller", controlled("c1", none),
			controlled("c2", func(a *vttv1.Actor) { a.Resources = qaCmdKeys(long) })},
		{"add_actor naming a controller and an empty attribute", controlled("c3", none),
			controlled("c4", func(a *vttv1.Actor) { a.Attributes = qaCmdAttrs("") })},
	}
	for _, c := range cases {
		want := dm.send(c.short)
		got := dm.send(c.long)
		t.Logf("%s: %s", c.label, got.GetError())
		if want.GetOk() || got.GetOk() || got.GetError() != want.GetError() {
			t.Errorf("%s: ok=%v %q, want the validator's %q", c.label, got.GetOk(), got.GetError(), want.GetError())
		}
	}
	qaCmdOK(t, dm, qaCmdFollow(1), 2)
}

func qaCmdObjects(ids []string) []map[string]any {
	objs := make([]map[string]any, 0, len(ids))
	for i, id := range ids {
		objs = append(objs, map[string]any{
			"id": id, "kind": "crate", "at": []int{i % 4, i / 4}, "size": []int{1, 1}, "rot": 0,
			"blocks_sight": false, "blocks_move": true, "art": "qa-crate",
		})
	}
	return objs
}

func qaCmdWriteJSON(t *testing.T, path string, doc any) {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func qaCmdWriteMap(t *testing.T, dir, id string, objectIDs []string) {
	t.Helper()
	qaCmdWriteJSON(t, filepath.Join(dir, id+".json"), map[string]any{
		"format_version": 1, "id": id, "name": "qa", "grid_width": 4, "grid_height": 4,
		"objects": qaCmdObjects(objectIDs),
	})
}

// VTT-161 VTT-162 VTT-294 VTT-295
func TestQACmdLoadMapOfAnInstalledMapWithABadObjectIDIsRefusedWithLoadInstalledsText(t *testing.T) {
	mapsDir := filepath.Join(t.TempDir(), "maps")
	edge := []string{strings.Repeat("😀", 32), "qa-2", strings.Repeat("é", 63) + "xy"}
	qaCmdWriteMap(t, mapsDir, "qa-long", []string{"qa-1", strings.Repeat("é", 65)})
	qaCmdWriteMap(t, mapsDir, "qa-empty", []string{"qa-1", "", "qa-3"})
	qaCmdWriteMap(t, mapsDir, "qa-repeat", []string{"qa-1", "qa-2", "qa-1"})
	qaCmdWriteMap(t, mapsDir, "qa-edge", edge)
	tb := qaCmdServe(t, func(s *gateway.Server) *gateway.Server { return s.WithMapsDir(mapsDir) })
	issuer := qaCmdDial(t, tb, tb.dmToken)
	observer := qaCmdDial(t, tb, tb.obsToken)
	for n, id := range []string{"qa-long", "qa-empty", "qa-repeat"} {
		_, lerr := mapdef.LoadInstalled(mapsDir, id, "")
		if lerr == nil {
			t.Fatalf("LoadInstalled accepted %s", id)
		}
		res := issuer.send(qaCmdLoadMap(fmt.Sprintf("map-%d", n), id))
		t.Logf("%s: %s", id, strings.ReplaceAll(res.GetError(), strings.Repeat("é", 65), "<v>"))
		if res.GetOk() || res.GetError() != lerr.Error() {
			t.Fatalf("load_map %s: ok=%v err=%q, want LoadInstalled's %q", id, res.GetOk(), res.GetError(), lerr.Error())
		}
		if strings.Contains(res.GetError(), mapsDir) {
			t.Errorf("refusal %q names the server's maps directory", res.GetError())
		}
		qaCmdStillOpenNothingAppended(t, tb, issuer, observer, n, int64(n))
	}
	res := qaCmdOK(t, issuer, qaCmdLoadMap("edge", "qa-edge"), 4)
	var got []string
	for _, o := range observer.event(res.GetSequence()).GetSceneCreated().GetObjects() {
		got = append(got, o.GetObjectId())
	}
	if !slices.Equal(got, edge) {
		t.Fatalf("loaded scene carries %d object ids, not the %d installed whole", len(got), len(edge))
	}
}

// VTT-161 VTT-162 VTT-291 VTT-292
func TestQACmdLoadMapOfAHeldMapWithABadObjectIDIsRefused(t *testing.T) {
	cases := []struct {
		label, id string
	}{
		{"over the bound", strings.Repeat("😀", 32) + "a"},
		{"empty", ""},
		{"repeated", "qa-1"},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			dir := t.TempDir()
			qaCmdWriteMap(t, dir, "qa-held", []string{"qa-1", "qa-2", "qa-3"})
			m, err := mapdef.Load(filepath.Join(dir, "qa-held.json"))
			if err != nil {
				t.Fatalf("mapdef.Load: %v", err)
			}
			m.Objects[2].ID = c.id
			want := ""
			envs, _, cerr := mapdef.Compile(m, "")
			if cerr != nil {
				want = cerr.Error()
			} else {
				st := engine.NewState()
				for _, env := range envs {
					if ferr := engine.Apply(st, env); ferr != nil {
						want = ferr.Error()
						break
					}
				}
			}
			if want == "" {
				t.Fatalf("neither Compile nor the fold refused an object id that is %s", c.label)
			}
			tb := qaCmdServe(t, func(s *gateway.Server) *gateway.Server {
				return s.WithMaps(map[string]*mapdef.Map{m.ID: m})
			})
			issuer := qaCmdDial(t, tb, tb.dmToken)
			observer := qaCmdDial(t, tb, tb.obsToken)
			res := issuer.send(qaCmdLoadMap("held", m.ID))
			t.Logf("refusal: %s", res.GetError())
			if res.GetOk() || !strings.Contains(res.GetError(), want) {
				t.Fatalf("load_map: ok=%v err=%q, want a refusal carrying %q", res.GetOk(), res.GetError(), want)
			}
			qaCmdStillOpenNothingAppended(t, tb, issuer, observer, 1, 0)
		})
	}
}

func qaCmdAdventureOnDisk(t *testing.T) (*rules.Ruleset, *adventure.Adventure) {
	t.Helper()
	root := t.TempDir()
	rsDir, advDir := filepath.Join(root, "ruleset"), filepath.Join(root, "adventure")
	qaCmdWriteJSON(t, filepath.Join(rsDir, "ruleset.json"), map[string]any{
		"id": "qa-rules", "name": "QA Rules", "format_version": "2",
		"attributes": []string{"vim"}, "defenses": []string{}, "resources": []any{},
	})
	qaCmdWriteJSON(t, filepath.Join(advDir, "adventure.json"), map[string]any{
		"id": "qa-adv", "name": "QA", "format_version": "1", "ruleset": "qa-rules", "opening_narration": "qa",
	})
	qaCmdWriteJSON(t, filepath.Join(advDir, "scenes", "qa-scene.json"), map[string]any{
		"id": "qa-scene", "name": "QA", "grid_width": 4, "grid_height": 4,
		"objects": qaCmdObjects([]string{"qa-1", "qa-2", "qa-3"}),
	})
	qaCmdWriteJSON(t, filepath.Join(advDir, "actors", "qa-hero.json"), map[string]any{
		"actor_id": "qa-hero", "name": "QA", "kind": "non_party", "attributes": map[string]int{"vim": 1},
	})
	if err := os.MkdirAll(filepath.Join(advDir, "art"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(advDir, "art", "qa-crate.png"), []byte("fake-png"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{rsDir, advDir} {
		if err := os.WriteFile(filepath.Join(d, "guide.md"), []byte("qa\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(rsDir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	adv, err := adventure.Load(advDir, rs)
	if err != nil {
		t.Fatalf("adventure.Load: %v", err)
	}
	return rs, adv
}

// VTT-161 VTT-162 VTT-291 VTT-292
func TestQACmdLoadAdventureWithABadObjectIDIsRefused(t *testing.T) {
	for _, c := range []struct{ label, id string }{
		{"over the bound", strings.Repeat("€", 43)},
		{"empty", ""},
		{"repeated", "qa-1"},
	} {
		t.Run(c.label, func(t *testing.T) {
			rs, adv := qaCmdAdventureOnDisk(t)
			adv.Scenes[0].Objects[2].ID = c.id
			tb := qaCmdServe(t, func(s *gateway.Server) *gateway.Server {
				return s.WithRuleset(rs).WithAdventures(map[string]*adventure.Adventure{adv.ID: adv})
			})
			issuer := qaCmdDial(t, tb, tb.dmToken)
			observer := qaCmdDial(t, tb, tb.obsToken)
			res := issuer.send(&vttv1.ClientCommand{RequestId: "adv", Command: &vttv1.ClientCommand_LoadAdventure{
				LoadAdventure: &vttv1.LoadAdventure{AdventureId: adv.ID},
			}})
			t.Logf("refusal: %s", strings.ReplaceAll(res.GetError(), strings.Repeat("€", 43), "<v>"))
			if res.GetOk() {
				t.Fatalf("load_adventure with an object id that is %s accepted at %d", c.label, res.GetSequence())
			}
			qaCmdStillOpenNothingAppended(t, tb, issuer, observer, 1, 0)
		})
	}
}

// VTT-293 VTT-297
func TestQACmdASessionAndAMoveCarryTheirIDsToEverySeat(t *testing.T) {
	mapsDir := filepath.Join(t.TempDir(), "maps")
	qaCmdWriteMap(t, mapsDir, "qa-board", []string{"qa-crate"})
	tb := qaCmdServe(t, func(s *gateway.Server) *gateway.Server { return s.WithMapsDir(mapsDir) })
	dm := qaCmdDial(t, tb, tb.dmToken)
	player := qaCmdDial(t, tb, tb.playerToken)
	res := qaCmdOK(t, dm, &vttv1.ClientCommand{RequestId: "session", Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: "qa"},
	}}, 1)
	session := dm.event(res.GetSequence()).GetSessionId()
	if session == "" || len(session) > qaCmdBound {
		t.Fatalf("recorded session id is %d bytes", len(session))
	}
	qaCmdOK(t, dm, qaCmdLoadMap("board", "qa-board"), 2)
	qaCmdOK(t, dm, qaCmdActorCmd("actor", qaCmdPlainActor("qa-hero")), 0)
	qaCmdOK(t, dm, qaCmdGrantCmd("grant", "qa-hero", tb.playerID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER), 0)
	qaCmdOK(t, dm, &vttv1.ClientCommand{RequestId: "place", Command: &vttv1.ClientCommand_PlaceToken{
		PlaceToken: &vttv1.PlaceToken{
			TokenId: "qa-tok", SceneId: "qa-board", ActorId: "qa-hero", Position: &vttv1.GridPosition{X: 2, Y: 2},
		},
	}}, 0)
	res = qaCmdOK(t, dm, &vttv1.ClientCommand{RequestId: "move", Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: "qa-tok", To: &vttv1.GridPosition{X: 3, Y: 2}},
	}}, 0)
	if got := dm.event(res.GetSequence()).GetTokenMoved().GetSceneId(); got != "qa-board" {
		t.Fatalf("recorded move's scene id is %q, want the token's scene", got)
	}
	seen := player.find(func(e *vttv1.Envelope) bool { return e.GetSessionStarted() != nil })
	if seen.GetSessionId() != session {
		t.Fatalf("the player's session started carries session id %q, want %q", seen.GetSessionId(), session)
	}
	moved := player.find(func(e *vttv1.Envelope) bool { return e.GetTokenMoved() != nil })
	if moved.GetTokenMoved().GetSceneId() != "qa-board" {
		t.Fatalf("the player's move carries scene id %q, want the token's scene", moved.GetTokenMoved().GetSceneId())
	}
	st := engine.NewState()
	for i, env := range player.events {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("the player's frame %d (sequence %d) does not fold: %v", i, env.GetSequence(), err)
		}
	}
}
