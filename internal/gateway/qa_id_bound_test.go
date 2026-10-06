package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

const qaIDBound = 128

const (
	qaIDScene     = "SceneCreated"
	qaIDActor     = "ActorAdded"
	qaIDToken     = "TokenPlaced"
	qaIDAdventure = "AdventureLoaded"
)

var qaIDKinds = []string{qaIDScene, qaIDActor, qaIDToken, qaIDAdventure}

var qaIDField = map[string]*regexp.Regexp{
	qaIDScene:     regexp.MustCompile(`(?i)scene[ _.]?id`),
	qaIDActor:     regexp.MustCompile(`(?i)actor[ _.]?id`),
	qaIDToken:     regexp.MustCompile(`(?i)token[ _.]?id`),
	qaIDAdventure: regexp.MustCompile(`(?i)adventure[ _.]?id`),
}

const qaIDActorWithoutID = "engine: actor_added requires an actor with an id"

type qaIDCase struct {
	label string
	id    string
}

func qaIDASCII(n int) string { return strings.Repeat("a", n) }

func qaIDAccepted() []qaIDCase {
	return []qaIDCase{
		{"one byte", "a"},
		{"127 ascii", qaIDASCII(127)},
		{"128 ascii", "S" + qaIDASCII(126) + "E"},
		{"64 two-byte runes", strings.Repeat("é", 64)},
		{"42 three-byte runes and two ascii", strings.Repeat("€", 42) + "ab"},
		{"32 four-byte runes", strings.Repeat("😀", 32)},
	}
}

func qaIDRefused() []qaIDCase {
	return []qaIDCase{
		{"129 ascii", qaIDASCII(129)},
		{"1000 ascii", qaIDASCII(1000)},
		{"65 two-byte runes", strings.Repeat("é", 65)},
		{"one ascii then 64 two-byte runes", "a" + strings.Repeat("é", 64)},
		{"43 three-byte runes", strings.Repeat("€", 43)},
		{"32 four-byte runes and one ascii", strings.Repeat("😀", 32) + "a"},
	}
}

func qaIDEvent(kind, id string) *vttv1.Envelope {
	env := &vttv1.Envelope{EventId: "qa-id-event", Sequence: 1}
	switch kind {
	case qaIDScene:
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: id, Name: "qa", GridWidth: 3, GridHeight: 3,
		}}
	case qaIDActor:
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: id, Name: "qa", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}}}
	case qaIDToken:
		env.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: id, SceneId: "qa-scene", ActorId: "qa-actor", Position: &vttv1.GridPosition{X: 1, Y: 1},
		}}
	case qaIDAdventure:
		env.Payload = &vttv1.Envelope_AdventureLoaded{AdventureLoaded: &vttv1.AdventureLoaded{
			AdventureId: id, Name: "qa",
		}}
	}
	return env
}

func qaIDState(t *testing.T, kind string) *engine.State {
	t.Helper()
	st := engine.NewState()
	if kind != qaIDToken {
		return st
	}
	for _, env := range []*vttv1.Envelope{qaIDEvent(qaIDScene, "qa-scene"), qaIDEvent(qaIDActor, "qa-actor")} {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("setup event refused: %v", err)
		}
	}
	return st
}

func qaIDHolds(st *engine.State, kind, id string) bool {
	switch kind {
	case qaIDScene:
		_, ok := st.Scenes[id]
		return ok
	case qaIDActor:
		_, ok := st.Actors[id]
		return ok
	case qaIDToken:
		_, ok := st.Tokens[id]
		return ok
	}
	return true
}

func qaIDFoldText(t *testing.T, st *engine.State, env *vttv1.Envelope) string {
	t.Helper()
	err := engine.Apply(st, env)
	if err == nil {
		t.Fatalf("engine.Apply accepted the event the test expects it to refuse")
	}
	return err.Error()
}

// VTT-277
func TestQAIDTheGoFoldAcceptsEveryIDUpToTheBound(t *testing.T) {
	for _, kind := range qaIDKinds {
		for _, c := range qaIDAccepted() {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				if len(c.id) > qaIDBound {
					t.Fatalf("fixture %q is %d bytes, over the bound", c.label, len(c.id))
				}
				st := qaIDState(t, kind)
				if err := engine.Apply(st, qaIDEvent(kind, c.id)); err != nil {
					t.Fatalf("%s id of %d bytes refused: %v", kind, len(c.id), err)
				}
				if !qaIDHolds(st, kind, c.id) {
					t.Fatalf("%s id of %d bytes accepted but not held whole", kind, len(c.id))
				}
			})
		}
	}
}

// VTT-277
func TestQAIDTheGoFoldRefusesEveryIDOverTheBoundNamingFieldBoundAndLength(t *testing.T) {
	for _, kind := range qaIDKinds {
		for _, c := range qaIDRefused() {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				err := engine.Apply(qaIDState(t, kind), qaIDEvent(kind, c.id))
				if err == nil {
					t.Fatalf("%s id of %d bytes (%d runes) accepted", kind, len(c.id), len([]rune(c.id)))
				}
				msg := strings.ReplaceAll(err.Error(), c.id, "<id>")
				t.Logf("refusal: %s", msg)
				if !qaIDField[kind].MatchString(msg) {
					t.Errorf("refusal %q does not name the %s id field", msg, kind)
				}
				for _, want := range []string{strconv.Itoa(qaIDBound), strconv.Itoa(len(c.id))} {
					if !strings.Contains(msg, want) {
						t.Errorf("refusal %q does not contain %q", msg, want)
					}
				}
			})
		}
	}
}

var qaIDZero = regexp.MustCompile(`\b0\b`)

// VTT-278
func TestQAIDTheGoFoldRefusesAnEmptyID(t *testing.T) {
	for _, kind := range qaIDKinds {
		t.Run(kind, func(t *testing.T) {
			err := engine.Apply(qaIDState(t, kind), qaIDEvent(kind, ""))
			if err == nil {
				t.Fatalf("%s with an empty id accepted", kind)
			}
			msg := err.Error()
			t.Logf("refusal: %s", msg)
			if kind == qaIDActor {
				if msg != qaIDActorWithoutID {
					t.Fatalf("refusal %q, want %q", msg, qaIDActorWithoutID)
				}
				return
			}
			if !qaIDField[kind].MatchString(msg) {
				t.Errorf("refusal %q does not name the %s id field", msg, kind)
			}
			if !strings.Contains(msg, strconv.Itoa(qaIDBound)) || !qaIDZero.MatchString(msg) {
				t.Errorf("refusal %q does not carry the bound %d and the length 0", msg, qaIDBound)
			}
		})
	}
}

func qaIDWith(env *vttv1.Envelope, edit func(*vttv1.Envelope)) *vttv1.Envelope {
	edit(env)
	return env
}

// VTT-277 VTT-278
func TestQAIDTheGoFoldChecksTheIDBeforeTheLaterFaults(t *testing.T) {
	long := qaIDASCII(qaIDBound + 1)
	longName := qaIDASCII(257)
	withScene := func(id string) func(*engine.State) {
		return func(st *engine.State) { st.Scenes[id] = engine.Scene{ID: id, GridWidth: 3, GridHeight: 3} }
	}
	withActor := func(id string) func(*engine.State) {
		return func(st *engine.State) { st.Actors[id] = &vttv1.Actor{ActorId: id} }
	}
	withToken := func(id string) func(*engine.State) {
		return func(st *engine.State) {
			st.Tokens[id] = engine.Token{ID: id, SceneID: "qa-scene", ActorID: "qa-actor", X: 1, Y: 1}
		}
	}
	controlled := func(e *vttv1.Envelope) {
		a := e.GetActorAdded().GetActor()
		a.ControllerId = "qa-participant"
		a.ControllerIds = []string{"qa-participant"}
	}
	nameScene := func(e *vttv1.Envelope) { e.GetSceneCreated().Name = longName }
	nameActor := func(e *vttv1.Envelope) { e.GetActorAdded().GetActor().Name = longName }
	nameAdventure := func(e *vttv1.Envelope) { e.GetAdventureLoaded().Name = longName }
	stranger := func(e *vttv1.Envelope) {
		tp := e.GetTokenPlaced()
		tp.SceneId, tp.ActorId, tp.Position = "qa-nowhere", "qa-nobody", nil
	}
	cases := []struct {
		label  string
		kind   string
		id     string
		inject func(*engine.State)
		edit   func(*vttv1.Envelope)
	}{
		{"scene id before its name", qaIDScene, long, nil, nameScene},
		{"scene id before a duplicate", qaIDScene, long, withScene(long), nameScene},
		{"empty scene id before a duplicate", qaIDScene, "", withScene(""), nameScene},
		{"actor id before a controller", qaIDActor, long, nil, controlled},
		{"actor id before its name", qaIDActor, long, nil, nameActor},
		{"actor id before a duplicate", qaIDActor, long, withActor(long), nameActor},
		{"missing actor id before a controller", qaIDActor, "", nil, func(e *vttv1.Envelope) { controlled(e); nameActor(e) }},
		{"token id before unknown scene actor and position", qaIDToken, long, nil, stranger},
		{"token id before a duplicate", qaIDToken, long, withToken(long), func(*vttv1.Envelope) {}},
		{"empty token id before a duplicate", qaIDToken, "", withToken(""), func(*vttv1.Envelope) {}},
		{"empty token id before an unknown scene", qaIDToken, "", nil, stranger},
		{"adventure id before its name", qaIDAdventure, long, nil, nameAdventure},
		{"empty adventure id before its name", qaIDAdventure, "", nil, nameAdventure},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			want := qaIDFoldText(t, qaIDState(t, c.kind), qaIDEvent(c.kind, c.id))
			st := qaIDState(t, c.kind)
			if c.kind == qaIDToken && c.inject == nil {
				st = engine.NewState()
			}
			if c.inject != nil {
				c.inject(st)
			}
			got := qaIDFoldText(t, st, qaIDWith(qaIDEvent(c.kind, c.id), c.edit))
			if got != want {
				t.Fatalf("refusal\n got %q\nwant %q (the id's own)", got, want)
			}
		})
	}
}

type qaIDTable struct {
	srv      *httptest.Server
	camp     *campaign.Campaign
	dmToken  string
	obsToken string
}

func qaIDServe(t *testing.T, configure func(*gateway.Server) *gateway.Server) *qaIDTable {
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
	dmToken, _, err := ids.CreateInvite("qa dm", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	obsToken, _, err := ids.CreateInvite("qa observer", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	return &qaIDTable{srv: hs, camp: c, dmToken: dmToken, obsToken: obsToken}
}

type qaIDConn struct {
	t       *testing.T
	ws      *websocket.Conn
	head    int64
	events  []*vttv1.Envelope
	results map[string]*vttv1.CommandResult
}

func qaIDDial(t *testing.T, tb *qaIDTable, token string) *qaIDConn {
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
	qc := &qaIDConn{t: t, ws: ws, results: map[string]*vttv1.CommandResult{}}
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	first := qc.read()
	if first.GetCatchUpHead() == nil {
		t.Fatalf("first frame is not catch_up_head: %v", first)
	}
	qc.head = first.GetCatchUpHead().GetHeadSequence()
	return qc
}

func (qc *qaIDConn) read() *vttv1.ServerFrame {
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

func (qc *qaIDConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
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

func (qc *qaIDConn) event(seq int64) *vttv1.Envelope {
	qc.t.Helper()
	for {
		for _, e := range qc.events {
			if e.GetSequence() == seq {
				return e
			}
		}
		qc.read()
	}
}

func qaIDAdd(req, actorID string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{ActorId: actorID, Name: "qa", Kind: kind}},
	}}
}

func qaIDPlace(req, tokenID, sceneID, actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_PlaceToken{
		PlaceToken: &vttv1.PlaceToken{
			TokenId: tokenID, SceneId: sceneID, ActorId: actorID, Position: &vttv1.GridPosition{X: 1, Y: 1},
		},
	}}
}

func qaIDLoadMap(req, mapID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_LoadMap{
		LoadMap: &vttv1.LoadMap{MapId: mapID},
	}}
}

func qaIDLoadAdventure(req, advID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_LoadAdventure{
		LoadAdventure: &vttv1.LoadAdventure{AdventureId: advID},
	}}
}

func qaIDFollow(n int) *vttv1.ClientCommand {
	return qaIDAdd(fmt.Sprintf("follow-%d", n), fmt.Sprintf("qa-follow-%d", n), vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
}

func qaIDOK(t *testing.T, qc *qaIDConn, cmd *vttv1.ClientCommand, wantSeq int64) *vttv1.CommandResult {
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

func qaIDStillOpenNothingAppended(t *testing.T, tb *qaIDTable, issuer, observer *qaIDConn, n int, h0 int64) {
	t.Helper()
	next := issuer.send(qaIDFollow(n))
	if !next.GetOk() {
		t.Fatalf("the connection did not take the next command: %q", next.GetError())
	}
	if next.GetSequence() != h0+1 {
		t.Fatalf("next command landed at %d, want %d: the refusal appended", next.GetSequence(), h0+1)
	}
	seen := observer.event(h0 + 1)
	if seen.GetEventId() != issuer.event(h0+1).GetEventId() {
		t.Fatalf("observer's event %d is not the follow-up", h0+1)
	}
	for i, e := range observer.events {
		if e.GetSequence() != int64(i+1) {
			t.Fatalf("observer received sequence %d at position %d", e.GetSequence(), i)
		}
	}
	if fresh := qaIDDial(t, tb, tb.obsToken); fresh.head != h0+1 {
		t.Fatalf("log head is %d, want %d", fresh.head, h0+1)
	}
}

func qaIDWriteMap(t *testing.T, dir, file, id, tokenID string) {
	t.Helper()
	doc := map[string]any{
		"format_version": 1, "id": id, "name": "qa", "grid_width": 3, "grid_height": 3,
	}
	if tokenID != "-" {
		doc["placements"] = []map[string]any{{"token_id": tokenID, "actor_id": "qa-actor", "x": 1, "y": 1}}
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal map: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), body, 0o600); err != nil {
		t.Fatalf("write map: %v", err)
	}
}

func qaIDMapsDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "maps")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func qaIDBoard(t *testing.T, dm *qaIDConn) string {
	t.Helper()
	res := qaIDOK(t, dm, qaIDLoadMap("board", "qa-board"), 1)
	scene := dm.event(res.GetSequence()).GetSceneCreated()
	if scene == nil {
		t.Fatalf("load_map's first event is not a SceneCreated")
	}
	qaIDOK(t, dm, qaIDAdd("actor", "qa-actor", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), 2)
	return scene.GetSceneId()
}

func qaIDBoardServer(t *testing.T) *qaIDTable {
	t.Helper()
	mapsDir := qaIDMapsDir(t)
	qaIDWriteMap(t, mapsDir, "qa-board.json", "qa-board", "-")
	return qaIDServe(t, func(s *gateway.Server) *gateway.Server { return s.WithMapsDir(mapsDir) })
}

// VTT-161 VTT-162 VTT-277
func TestQAIDAddActorOverTheBoundIsRefusedOnTheWireWithTheFoldsText(t *testing.T) {
	for _, c := range qaIDRefused() {
		t.Run(c.label, func(t *testing.T) {
			tb := qaIDServe(t, nil)
			issuer := qaIDDial(t, tb, tb.dmToken)
			observer := qaIDDial(t, tb, tb.obsToken)
			cmd := qaIDAdd("long", c.id, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
			want := qaIDFoldText(t, tb.camp.State(), qaIDEvent(qaIDActor, c.id))
			res := issuer.send(cmd)
			t.Logf("refusal: %s", strings.ReplaceAll(res.GetError(), c.id, "<id>"))
			if res.GetOk() {
				t.Fatalf("add_actor of a %d-byte id accepted at %d", len(c.id), res.GetSequence())
			}
			if res.GetError() != want {
				t.Fatalf("refusal\n got %q\nwant %q (the fold's)", res.GetError(), want)
			}
			qaIDStillOpenNothingAppended(t, tb, issuer, observer, 1, 0)
		})
	}
}

// VTT-161 VTT-162 VTT-277
func TestQAIDPlaceTokenOverTheBoundIsRefusedOnTheWireWithTheFoldsText(t *testing.T) {
	for _, c := range qaIDRefused() {
		t.Run(c.label, func(t *testing.T) {
			tb := qaIDBoardServer(t)
			issuer := qaIDDial(t, tb, tb.dmToken)
			observer := qaIDDial(t, tb, tb.obsToken)
			sceneID := qaIDBoard(t, issuer)
			cmd := qaIDPlace("long", c.id, sceneID, "qa-actor")
			env := qaIDEvent(qaIDToken, c.id)
			env.GetTokenPlaced().SceneId = sceneID
			want := qaIDFoldText(t, tb.camp.State(), env)
			res := issuer.send(cmd)
			t.Logf("refusal: %s", strings.ReplaceAll(res.GetError(), c.id, "<id>"))
			if res.GetOk() {
				t.Fatalf("place_token of a %d-byte id accepted at %d", len(c.id), res.GetSequence())
			}
			if res.GetError() != want {
				t.Fatalf("refusal\n got %q\nwant %q (the fold's)", res.GetError(), want)
			}
			qaIDStillOpenNothingAppended(t, tb, issuer, observer, 1, 2)
		})
	}
}

// VTT-161 VTT-162 VTT-278
func TestQAIDAnEmptyIDIsRefusedOnTheWireWithTheFoldsText(t *testing.T) {
	tb := qaIDBoardServer(t)
	issuer := qaIDDial(t, tb, tb.dmToken)
	observer := qaIDDial(t, tb, tb.obsToken)
	sceneID := qaIDBoard(t, issuer)
	res := issuer.send(qaIDAdd("empty-actor", "", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	if res.GetOk() || res.GetError() != qaIDActorWithoutID {
		t.Fatalf("add_actor with no id: ok=%v err=%q, want %q", res.GetOk(), res.GetError(), qaIDActorWithoutID)
	}
	qaIDStillOpenNothingAppended(t, tb, issuer, observer, 1, 2)
	env := qaIDEvent(qaIDToken, "")
	env.GetTokenPlaced().SceneId = sceneID
	want := qaIDFoldText(t, tb.camp.State(), env)
	res = issuer.send(qaIDPlace("empty-token", "", sceneID, "qa-actor"))
	t.Logf("refusal: %s", res.GetError())
	if res.GetOk() || res.GetError() != want {
		t.Fatalf("place_token with no id: ok=%v err=%q, want %q", res.GetOk(), res.GetError(), want)
	}
	qaIDStillOpenNothingAppended(t, tb, issuer, observer, 2, 3)
}

// VTT-277
func TestQAIDAddActorAndPlaceTokenAtTheBoundAreRecordedWhole(t *testing.T) {
	for _, c := range qaIDAccepted()[2:] {
		t.Run(c.label, func(t *testing.T) {
			tb := qaIDBoardServer(t)
			dm := qaIDDial(t, tb, tb.dmToken)
			sceneID := qaIDBoard(t, dm)
			res := qaIDOK(t, dm, qaIDAdd("edge-actor", c.id, vttv1.ActorKind_ACTOR_KIND_NON_PARTY), 3)
			if got := dm.event(res.GetSequence()).GetActorAdded().GetActor().GetActorId(); got != c.id {
				t.Fatalf("recorded actor id is %d bytes, sent %d", len(got), len(c.id))
			}
			res = qaIDOK(t, dm, qaIDPlace("edge-token", c.id, sceneID, c.id), 4)
			if got := dm.event(res.GetSequence()).GetTokenPlaced().GetTokenId(); got != c.id {
				t.Fatalf("recorded token id is %d bytes, sent %d", len(got), len(c.id))
			}
		})
	}
}

// VTT-161
func TestQAIDAddActorsValidatorRefusesBeforeTheFoldMeasuresTheID(t *testing.T) {
	tb := qaIDServe(t, nil)
	dm := qaIDDial(t, tb, tb.dmToken)
	long := qaIDASCII(qaIDBound + 1)
	controlled := func(req, id string) *vttv1.ClientCommand {
		cmd := qaIDAdd(req, id, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)
		cmd.GetAddActor().GetActor().ControllerIds = []string{"qa-participant"}
		return cmd
	}
	cases := []struct {
		label       string
		short, long *vttv1.ClientCommand
	}{
		{"unspecified kind", qaIDAdd("k1", "qa-short", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED),
			qaIDAdd("k2", long, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)},
		{"a named controller", controlled("c1", "qa-short"), controlled("c2", long)},
	}
	for _, c := range cases {
		want := dm.send(c.short)
		got := dm.send(c.long)
		if want.GetOk() || got.GetOk() || got.GetError() != want.GetError() {
			t.Errorf("%s: with a %d-byte id the refusal is ok=%v %q, want the validator's %q",
				c.label, len(long), got.GetOk(), got.GetError(), want.GetError())
		}
	}
	qaIDOK(t, dm, qaIDFollow(1), 1)
}

// VTT-161 VTT-162 VTT-279 VTT-280
func TestQAIDLoadMapOfAnInstalledMapOverTheBoundIsRefusedWithLoadInstalledsText(t *testing.T) {
	mapsDir := qaIDMapsDir(t)
	longID := "q" + strings.Repeat("é", 64)
	longToken := strings.Repeat("😀", 32) + "a"
	edge := strings.Repeat("€", 42) + "ab"
	qaIDWriteMap(t, mapsDir, longID+".json", longID, "qa-tok")
	qaIDWriteMap(t, mapsDir, "qa-long-token.json", "qa-long-token", longToken)
	qaIDWriteMap(t, mapsDir, "qa-empty-token.json", "qa-empty-token", "")
	qaIDWriteMap(t, mapsDir, edge+".json", edge, edge)
	tb := qaIDServe(t, func(s *gateway.Server) *gateway.Server { return s.WithMapsDir(mapsDir) })
	issuer := qaIDDial(t, tb, tb.dmToken)
	observer := qaIDDial(t, tb, tb.obsToken)
	qaIDOK(t, issuer, qaIDAdd("actor", "qa-actor", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), 1)
	h := int64(1)
	for n, id := range []string{longID, "qa-long-token", "qa-empty-token"} {
		_, lerr := mapdef.LoadInstalled(mapsDir, id, "")
		if lerr == nil {
			t.Fatalf("LoadInstalled accepted %q", id)
		}
		res := issuer.send(qaIDLoadMap(fmt.Sprintf("map-%d", n), id))
		t.Logf("refusal: %s", res.GetError())
		if res.GetOk() || res.GetError() != lerr.Error() {
			t.Fatalf("load_map %d: ok=%v err=%q, want LoadInstalled's %q", n, res.GetOk(), res.GetError(), lerr.Error())
		}
		if strings.Contains(res.GetError(), mapsDir) {
			t.Errorf("refusal %q names the server's maps directory", res.GetError())
		}
		qaIDStillOpenNothingAppended(t, tb, issuer, observer, n, h)
		h++
	}
	res := qaIDOK(t, issuer, qaIDLoadMap("edge", edge), h+1)
	if issuer.event(res.GetSequence()).GetSceneCreated() == nil {
		t.Fatalf("load_map at the bound did not start with a SceneCreated")
	}
	if got := issuer.event(res.GetSequence() + 1).GetTokenPlaced().GetTokenId(); got != edge {
		t.Fatalf("placed token id is %d bytes, want %d", len(got), len(edge))
	}
}

func qaIDHeldMap(t *testing.T, id, tokenID string) *mapdef.Map {
	t.Helper()
	dir := t.TempDir()
	qaIDWriteMap(t, dir, "qa-held.json", "qa-held", "qa-tok")
	m, err := mapdef.Load(filepath.Join(dir, "qa-held.json"))
	if err != nil {
		t.Fatalf("mapdef.Load: %v", err)
	}
	m.ID = id
	m.Placements[0].TokenID = tokenID
	return m
}

func qaIDFoldBatch(t *testing.T, setup []*vttv1.Envelope, envs []*vttv1.Envelope) error {
	t.Helper()
	st := engine.NewState()
	for _, env := range setup {
		if err := engine.Apply(st, env); err != nil {
			t.Fatalf("setup refused: %v", err)
		}
	}
	for _, env := range envs {
		if err := engine.Apply(st, env); err != nil {
			return err
		}
	}
	return nil
}

// VTT-161 VTT-162 VTT-277
func TestQAIDAHeldMapWhoseIDsCrossTheBoundIsRefusedByTheFold(t *testing.T) {
	atBound := strings.Repeat("é", 64)
	over := "a" + atBound
	cases := []struct {
		label, id, token string
		refused          bool
	}{
		{"map id over", over, "qa-tok", true},
		{"token id over", "qa-held", over, true},
		{"both at the bound", atBound, atBound, false},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			m := qaIDHeldMap(t, c.id, c.token)
			envs, _, err := mapdef.Compile(m, "")
			if err != nil {
				t.Fatalf("mapdef.Compile: %v", err)
			}
			foldErr := qaIDFoldBatch(t, []*vttv1.Envelope{qaIDEvent(qaIDActor, "qa-actor")}, envs)
			if (foldErr != nil) != c.refused {
				t.Fatalf("local fold of the compiled map: %v, want refused=%v", foldErr, c.refused)
			}
			tb := qaIDServe(t, func(s *gateway.Server) *gateway.Server {
				return s.WithMaps(map[string]*mapdef.Map{m.ID: m})
			})
			issuer := qaIDDial(t, tb, tb.dmToken)
			observer := qaIDDial(t, tb, tb.obsToken)
			qaIDOK(t, issuer, qaIDAdd("actor", "qa-actor", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), 1)
			res := issuer.send(qaIDLoadMap("held", m.ID))
			if !c.refused {
				if !res.GetOk() || res.GetSequence() != 2 {
					t.Fatalf("load_map at the bound: ok=%v seq=%d err=%q", res.GetOk(), res.GetSequence(), res.GetError())
				}
				return
			}
			t.Logf("refusal: %s", strings.ReplaceAll(res.GetError(), over, "<id>"))
			if res.GetOk() || !strings.Contains(res.GetError(), foldErr.Error()) {
				t.Fatalf("load_map: ok=%v err=%q, want a refusal carrying %q", res.GetOk(), res.GetError(), foldErr.Error())
			}
			qaIDStillOpenNothingAppended(t, tb, issuer, observer, 1, 1)
		})
	}
}

func qaIDWriteJSON(t *testing.T, path string, doc any) {
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

func qaIDAdventureOnDisk(t *testing.T) (*rules.Ruleset, *adventure.Adventure) {
	t.Helper()
	root := t.TempDir()
	rsDir, advDir := filepath.Join(root, "ruleset"), filepath.Join(root, "adventure")
	qaIDWriteJSON(t, filepath.Join(rsDir, "ruleset.json"), map[string]any{
		"id": "qa-rules", "name": "QA Rules", "format_version": "2",
		"attributes": []string{"vim"}, "defenses": []string{}, "resources": []any{},
	})
	qaIDWriteJSON(t, filepath.Join(advDir, "adventure.json"), map[string]any{
		"id": "qa-adv", "name": "QA", "format_version": "1", "ruleset": "qa-rules", "opening_narration": "qa",
	})
	qaIDWriteJSON(t, filepath.Join(advDir, "scenes", "qa-scene.json"), map[string]any{
		"id": "qa-scene", "name": "QA", "grid_width": 3, "grid_height": 3,
		"placements": []map[string]any{{"token_id": "qa-tok", "actor_id": "qa-hero", "x": 1, "y": 1}},
	})
	for _, id := range []string{"qa-hero", "qa-extra"} {
		qaIDWriteJSON(t, filepath.Join(advDir, "actors", id+".json"), map[string]any{
			"actor_id": id, "name": "QA", "kind": "non_party", "attributes": map[string]int{"vim": 1},
		})
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

func qaIDAdventureEdits() map[string]func(*adventure.Adventure, string) {
	return map[string]func(*adventure.Adventure, string){
		"adventure id": func(a *adventure.Adventure, id string) { a.ID = id },
		"scene id":     func(a *adventure.Adventure, id string) { a.Scenes[0].ID = id },
		"actor id": func(a *adventure.Adventure, id string) {
			for i := range a.Actors {
				if a.Actors[i].ID == "qa-extra" {
					a.Actors[i].ID = id
				}
			}
		},
		"token id": func(a *adventure.Adventure, id string) { a.Scenes[0].Placements[0].TokenID = id },
	}
}

func qaIDBatchCarries(t *testing.T, qc *qaIDConn, from, to int64, field, id string) bool {
	t.Helper()
	for seq := from; seq < to; seq++ {
		e := qc.event(seq)
		got := map[string]string{
			"adventure id": e.GetAdventureLoaded().GetAdventureId(),
			"scene id":     e.GetSceneCreated().GetSceneId(),
			"actor id":     e.GetActorAdded().GetActor().GetActorId(),
			"token id":     e.GetTokenPlaced().GetTokenId(),
		}[field]
		if got == id {
			return true
		}
	}
	return false
}

// VTT-161 VTT-162 VTT-277
func TestQAIDAHeldAdventureWhoseIDsCrossTheBoundIsRefusedByTheFold(t *testing.T) {
	atBound := strings.Repeat("😀", 32)
	over := atBound + "a"
	for field, edit := range qaIDAdventureEdits() {
		for _, c := range []qaIDCase{{"over", over}, {"at the bound", atBound}} {
			t.Run(field+"/"+c.label, func(t *testing.T) {
				rs, adv := qaIDAdventureOnDisk(t)
				edit(adv, c.id)
				envs, _, err := adventure.Compile(adv, engine.NewState())
				if err != nil {
					t.Fatalf("adventure.Compile: %v", err)
				}
				foldErr := qaIDFoldBatch(t, nil, envs)
				if refused := c.id == over; (foldErr != nil) != refused {
					t.Fatalf("local fold of the compiled adventure: %v, want refused=%v", foldErr, refused)
				}
				tb := qaIDServe(t, func(s *gateway.Server) *gateway.Server {
					return s.WithRuleset(rs).WithAdventures(map[string]*adventure.Adventure{adv.ID: adv})
				})
				issuer := qaIDDial(t, tb, tb.dmToken)
				observer := qaIDDial(t, tb, tb.obsToken)
				res := issuer.send(qaIDLoadAdventure("adv", adv.ID))
				if foldErr != nil {
					t.Logf("refusal: %s", strings.ReplaceAll(res.GetError(), over, "<id>"))
					if res.GetOk() {
						t.Fatalf("load_adventure with a %d-byte %s accepted", len(c.id), field)
					}
					qaIDStillOpenNothingAppended(t, tb, issuer, observer, 1, 0)
					return
				}
				if !res.GetOk() || res.GetSequence() != 1 {
					t.Fatalf("load_adventure at the bound: ok=%v seq=%d err=%q", res.GetOk(), res.GetSequence(), res.GetError())
				}
				next := qaIDOK(t, issuer, qaIDFollow(1), 0)
				if !qaIDBatchCarries(t, issuer, 1, next.GetSequence(), field, c.id) {
					t.Fatalf("no event of the loaded batch carries the %d-byte %s", len(c.id), field)
				}
			})
		}
	}
}

func qaIDToolDescription(t *testing.T, tool string, path ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"))
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	var tools []struct {
		Name        string         `json:"name"`
		InputSchema map[string]any `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatalf("decode tools.json: %v", err)
	}
	for _, tl := range tools {
		if tl.Name != tool {
			continue
		}
		node := tl.InputSchema
		for _, p := range path {
			props, _ := node["properties"].(map[string]any)
			node, _ = props[p].(map[string]any)
		}
		desc, _ := node["description"].(string)
		return desc
	}
	t.Fatalf("no %s tool in tools.json", tool)
	return ""
}

var qaIDStatedBound = regexp.MustCompile(`At most (\d+) bytes of UTF-8`)

// VTT-281
func TestQAIDTheToolsStateTheIDBoundTheFoldEnforces(t *testing.T) {
	cases := []struct {
		tool, kind string
		path       []string
	}{
		{"add_actor", qaIDActor, []string{"actor", "actorId"}},
		{"place_token", qaIDToken, []string{"tokenId"}},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			desc := qaIDToolDescription(t, c.tool, c.path...)
			m := qaIDStatedBound.FindStringSubmatch(desc)
			if m == nil {
				t.Fatalf("%s id description states no bound: %q", c.tool, desc)
			}
			n, _ := strconv.Atoi(m[1])
			if n != qaIDBound {
				t.Fatalf("%s states %d bytes, want %d", c.tool, n, qaIDBound)
			}
			for _, id := range []string{qaIDASCII(n), strings.Repeat("é", n/2)} {
				if err := engine.Apply(qaIDState(t, c.kind), qaIDEvent(c.kind, id)); err != nil {
					t.Fatalf("the fold refuses an id of the stated %d bytes: %v", n, err)
				}
				if engine.Apply(qaIDState(t, c.kind), qaIDEvent(c.kind, id+"a")) == nil {
					t.Fatalf("the fold accepts an id one byte over the stated %d", n)
				}
			}
		})
	}
}

type qaIDMapCase struct {
	label string
	id    string
}

func qaIDMapAccepted() []qaIDMapCase {
	return []qaIDMapCase{
		{"128 ascii", "S" + strings.Repeat("a", 126) + "E"},
		{"64 two-byte runes", strings.Repeat("é", 64)},
		{"42 three-byte runes and two ascii", strings.Repeat("€", 42) + "ab"},
		{"32 four-byte runes", strings.Repeat("😀", 32)},
	}
}

func qaIDMapWrite(t *testing.T, dir, file, id, tokenID string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"format_version": 1, "id": id, "name": "qa", "grid_width": 2, "grid_height": 2,
		"placements": []map[string]any{{"token_id": tokenID, "actor_id": "qa-actor", "x": 1, "y": 1}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, file)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// VTT-279 VTT-277
func TestQAIDAMapWhoseIDsAreAtTheBoundLoadsCompilesAndFolds(t *testing.T) {
	cases := qaIDMapAccepted()
	for i, c := range cases {
		tokenID := cases[(i+1)%len(cases)].id
		t.Run(c.label, func(t *testing.T) {
			dir := t.TempDir()
			qaIDMapWrite(t, dir, c.id+".json", c.id, tokenID)
			loaded, err := mapdef.Load(filepath.Join(dir, c.id+".json"))
			if err != nil {
				t.Fatalf("Load refused a %d-byte id: %v", len(c.id), err)
			}
			installed, err := mapdef.LoadInstalled(dir, c.id, "")
			if err != nil {
				t.Fatalf("LoadInstalled refused a %d-byte id: %v", len(c.id), err)
			}
			for _, m := range []*mapdef.Map{loaded, installed} {
				if m.ID != c.id || len(m.Placements) != 1 || m.Placements[0].TokenID != tokenID {
					t.Fatalf("loaded map does not hold its ids whole: id %d bytes, placements %v", len(m.ID), m.Placements)
				}
			}
			envs, _, err := mapdef.Compile(installed, "")
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			st := engine.NewState()
			actor := &vttv1.Envelope{EventId: "qa-actor", Sequence: 1, Payload: &vttv1.Envelope_ActorAdded{
				ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "qa-actor", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
			}}
			for i, env := range append([]*vttv1.Envelope{actor}, envs...) {
				if err := engine.Apply(st, env); err != nil {
					t.Fatalf("the fold refused event %d of the compiled map: %v", i, err)
				}
			}
			if _, ok := st.Tokens[tokenID]; !ok {
				t.Fatalf("the folded state holds no token %d bytes long", len(tokenID))
			}
		})
	}
}
