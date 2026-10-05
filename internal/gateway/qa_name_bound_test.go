package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
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

const qaNameBound = 256

type qaNameCase struct {
	label string
	name  string
}

func qaNameASCII(n int) string { return strings.Repeat("a", n) }

func qaNameAccepted() []qaNameCase {
	return []qaNameCase{
		{"empty", ""},
		{"one byte", "a"},
		{"255 ascii", qaNameASCII(255)},
		{"256 ascii", qaNameASCII(256)},
		{"128 two-byte runes", strings.Repeat("é", 128)},
		{"85 three-byte runes and one ascii", strings.Repeat("€", 85) + "a"},
		{"64 four-byte runes", strings.Repeat("😀", 64)},
	}
}

func qaNameRefused() []qaNameCase {
	return []qaNameCase{
		{"257 ascii", qaNameASCII(257)},
		{"1000 ascii", qaNameASCII(1000)},
		{"129 two-byte runes", strings.Repeat("é", 129)},
		{"one ascii then 128 two-byte runes", "a" + strings.Repeat("é", 128)},
		{"86 three-byte runes", strings.Repeat("€", 86)},
		{"64 four-byte runes and one ascii", strings.Repeat("😀", 64) + "a"},
	}
}

var qaNameKinds = []string{"SessionStarted", "SceneCreated", "ActorAdded", "AdventureLoaded"}

func qaNameEvent(kind, name string) *vttv1.Envelope {
	env := &vttv1.Envelope{EventId: "qa-name-event", Sequence: 1}
	switch kind {
	case "SessionStarted":
		env.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: name}}
	case "SceneCreated":
		env.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "qa-scene", Name: name, GridWidth: 2, GridHeight: 2,
		}}
	case "ActorAdded":
		env.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "qa-actor", Name: name, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}}}
	case "AdventureLoaded":
		env.Payload = &vttv1.Envelope_AdventureLoaded{AdventureLoaded: &vttv1.AdventureLoaded{
			AdventureId: "qa-adventure", Name: name,
		}}
	}
	return env
}

func qaNameFoldText(t *testing.T, env *vttv1.Envelope) string {
	t.Helper()
	err := engine.Apply(engine.NewState(), env)
	if err == nil {
		t.Fatalf("engine.Apply accepted the event the wire was meant to refuse")
	}
	return err.Error()
}

// SPEC-018 How it works: the table's "May be empty" column, yes for a name.
// VTT-274
func TestQANameTheGoFoldAcceptsEveryNameUpToTheBound(t *testing.T) {
	for _, kind := range qaNameKinds {
		for _, c := range qaNameAccepted() {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				if len(c.name) > qaNameBound {
					t.Fatalf("fixture %q is %d bytes, over the bound", c.label, len(c.name))
				}
				if err := engine.Apply(engine.NewState(), qaNameEvent(kind, c.name)); err != nil {
					t.Fatalf("%s name of %d bytes refused: %v", kind, len(c.name), err)
				}
			})
		}
	}
}

// SPEC-018 How it works: "refuses a field over its bound ... with an error
// naming the field, the bound and the length".
// VTT-274
func TestQANameTheGoFoldRefusesEveryNameOverTheBound(t *testing.T) {
	for _, kind := range qaNameKinds {
		for _, c := range qaNameRefused() {
			t.Run(kind+"/"+c.label, func(t *testing.T) {
				err := engine.Apply(engine.NewState(), qaNameEvent(kind, c.name))
				if err == nil {
					t.Fatalf("%s name of %d bytes (%d runes) accepted", kind, len(c.name),
						len([]rune(c.name)))
				}
				msg := strings.ReplaceAll(err.Error(), c.name, "<name>")
				t.Logf("refusal: %s", msg)
				for _, want := range []string{"name", strconv.Itoa(qaNameBound), strconv.Itoa(len(c.name))} {
					if !strings.Contains(msg, want) {
						t.Errorf("refusal %q does not contain %q", msg, want)
					}
				}
			})
		}
	}
}

func qaNameSecond(t *testing.T, first, second *vttv1.Envelope) error {
	t.Helper()
	st := engine.NewState()
	if err := engine.Apply(st, first); err != nil {
		t.Fatalf("setup event refused: %v", err)
	}
	return engine.Apply(st, second)
}

// SPEC-018 How it works: "A SceneCreated is checked for a duplicate, then its
// name; an ActorAdded for an actor with an id, a duplicate and a declared
// controller, then its name; a SessionStarted for an open session, then its
// name".
// VTT-274
func TestQANameTheGoFoldChecksTheEarlierFaultBeforeTheName(t *testing.T) {
	long := qaNameASCII(qaNameBound + 1)
	actor := func(id, name string, controllers ...string) *vttv1.Envelope {
		env := qaNameEvent("ActorAdded", name)
		a := env.GetActorAdded().GetActor()
		a.ActorId = id
		if len(controllers) > 0 {
			a.ControllerId = controllers[0]
			a.ControllerIds = controllers
		}
		return env
	}
	cases := []struct {
		label         string
		first         *vttv1.Envelope
		short, faulty *vttv1.Envelope
	}{
		{"duplicate scene", qaNameEvent("SceneCreated", "x"),
			qaNameEvent("SceneCreated", "y"), qaNameEvent("SceneCreated", long)},
		{"actor without an id", qaNameEvent("SceneCreated", "x"),
			actor("", "y"), actor("", long)},
		{"duplicate actor", actor("qa-actor", "x"),
			actor("qa-actor", "y"), actor("qa-actor", long)},
		{"actor naming a controller", qaNameEvent("SceneCreated", "x"),
			actor("qa-other", "y", "qa-participant"), actor("qa-other", long, "qa-participant")},
		{"session already open", qaNameEvent("SessionStarted", "x"),
			qaNameEvent("SessionStarted", "y"), qaNameEvent("SessionStarted", long)},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			want := qaNameSecond(t, c.first, c.short)
			if want == nil {
				t.Fatalf("the earlier fault with a short name was accepted")
			}
			got := qaNameSecond(t, c.first, c.faulty)
			if got == nil || got.Error() != want.Error() {
				t.Fatalf("with a %d-byte name the refusal is %v, want the earlier fault's %q",
					qaNameBound+1, got, want.Error())
			}
			if c.label == "duplicate scene" && !errors.Is(got, engine.ErrSceneExists) {
				t.Errorf("duplicate scene refusal %v does not wrap engine.ErrSceneExists", got)
			}
		})
	}
}

type qaNameTable struct {
	srv      *httptest.Server
	dmToken  string
	obsToken string
}

func qaNameServe(t *testing.T, configure func(*gateway.Server) *gateway.Server) *qaNameTable {
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
	return &qaNameTable{srv: hs, dmToken: dmToken, obsToken: obsToken}
}

type qaNameConn struct {
	t       *testing.T
	ws      *websocket.Conn
	head    int64
	events  []*vttv1.Envelope
	results map[string]*vttv1.CommandResult
}

func qaNameDial(t *testing.T, tb *qaNameTable, token string) *qaNameConn {
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
	qc := &qaNameConn{t: t, ws: ws, results: map[string]*vttv1.CommandResult{}}
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	first := qc.read()
	if first.GetCatchUpHead() == nil {
		t.Fatalf("first frame is not catch_up_head: %v", first)
	}
	qc.head = first.GetCatchUpHead().GetHeadSequence()
	return qc
}

func (qc *qaNameConn) read() *vttv1.ServerFrame {
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

func (qc *qaNameConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
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

func (qc *qaNameConn) event(seq int64) *vttv1.Envelope {
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

func qaNameStart(id, name string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: id, Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: name},
	}}
}

func qaNameAdd(id, actorID, name string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: id, Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{
			ActorId: actorID, Name: name, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		}},
	}}
}

func qaNameLoadMap(id, mapID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: id, Command: &vttv1.ClientCommand_LoadMap{
		LoadMap: &vttv1.LoadMap{MapId: mapID},
	}}
}

func qaNameLoadAdventure(id, advID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: id, Command: &vttv1.ClientCommand_LoadAdventure{
		LoadAdventure: &vttv1.LoadAdventure{AdventureId: advID},
	}}
}

func qaNameNameOf(env *vttv1.Envelope) string {
	switch {
	case env.GetSessionStarted() != nil:
		return env.GetSessionStarted().GetName()
	case env.GetActorAdded() != nil:
		return env.GetActorAdded().GetActor().GetName()
	case env.GetSceneCreated() != nil:
		return env.GetSceneCreated().GetName()
	case env.GetAdventureLoaded() != nil:
		return env.GetAdventureLoaded().GetName()
	}
	return ""
}

func qaNameNameOfCommand(cmd *vttv1.ClientCommand) string {
	if s := cmd.GetStartSession(); s != nil {
		return s.GetName()
	}
	return cmd.GetAddActor().GetActor().GetName()
}

func qaNameHeadIs(t *testing.T, tb *qaNameTable, want int64) {
	t.Helper()
	fresh := qaNameDial(t, tb, tb.obsToken)
	if fresh.head != want {
		t.Fatalf("log head is %d, want %d", fresh.head, want)
	}
	for seq := int64(1); seq <= want; seq++ {
		if n := qaNameNameOf(fresh.event(seq)); len(n) > qaNameBound {
			t.Fatalf("event %d carries a %d-byte name", seq, len(n))
		}
	}
}

func qaNameRefusedOnTheWire(t *testing.T, refused, follow *vttv1.ClientCommand, want string) {
	t.Helper()
	tb := qaNameServe(t, nil)
	issuer := qaNameDial(t, tb, tb.dmToken)
	observer := qaNameDial(t, tb, tb.obsToken)
	h0 := issuer.head
	res := issuer.send(refused)
	t.Logf("refusal: %s", strings.ReplaceAll(res.GetError(), qaNameNameOfCommand(refused), "<name>"))
	if res.GetOk() {
		t.Fatalf("over-long name accepted at sequence %d", res.GetSequence())
	}
	if res.GetError() != want {
		t.Fatalf("refusal text\n got %q\nwant %q (the fold's)", res.GetError(), want)
	}
	next := issuer.send(follow)
	if !next.GetOk() {
		t.Fatalf("the connection did not take the next command: %q", next.GetError())
	}
	if next.GetSequence() != h0+1 {
		t.Fatalf("next command landed at %d, want %d: the refusal appended", next.GetSequence(), h0+1)
	}
	seen := observer.event(h0 + 1)
	if len(observer.events) != 1 || seen.GetEventId() != issuer.event(h0+1).GetEventId() {
		t.Fatalf("observer received %d events before the follow-up's", len(observer.events)-1)
	}
	qaNameHeadIs(t, tb, h0+1)
}

// VTT-161 VTT-162 VTT-274
func TestQANameStartSessionOverTheBoundIsRefusedOnTheWire(t *testing.T) {
	for _, c := range qaNameRefused() {
		t.Run(c.label, func(t *testing.T) {
			want := qaNameFoldText(t, qaNameEvent("SessionStarted", c.name))
			qaNameRefusedOnTheWire(t, qaNameStart("r1", c.name), qaNameStart("r2", "qa follow"), want)
		})
	}
}

// VTT-161 VTT-162 VTT-274
func TestQANameAddActorOverTheBoundIsRefusedOnTheWire(t *testing.T) {
	for _, c := range qaNameRefused() {
		t.Run(c.label, func(t *testing.T) {
			want := qaNameFoldText(t, qaNameEvent("ActorAdded", c.name))
			qaNameRefusedOnTheWire(t, qaNameAdd("r1", "qa-actor", c.name),
				qaNameAdd("r2", "qa-actor", "qa follow"), want)
		})
	}
}

// VTT-274
func TestQANameStartSessionAndAddActorAtTheBoundAreAcceptedWhole(t *testing.T) {
	for _, c := range qaNameAccepted()[1:] {
		t.Run(c.label, func(t *testing.T) {
			tb := qaNameServe(t, nil)
			dm := qaNameDial(t, tb, tb.dmToken)
			for _, cmd := range []*vttv1.ClientCommand{
				qaNameStart("r1", c.name), qaNameAdd("r2", "qa-actor", c.name),
			} {
				res := dm.send(cmd)
				if !res.GetOk() {
					t.Fatalf("%s of a %d-byte name refused: %q", cmd.GetRequestId(), len(c.name), res.GetError())
				}
				if got := qaNameNameOf(dm.event(res.GetSequence())); got != c.name {
					t.Fatalf("recorded name is %d bytes, sent %d", len(got), len(c.name))
				}
			}
		})
	}
}

func qaNameWriteMap(t *testing.T, dir, id, name string) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"format_version": 1, "id": id, "name": name, "grid_width": 2, "grid_height": 2,
	})
	if err != nil {
		t.Fatalf("marshal map: %v", err)
	}
	path := filepath.Join(dir, id+".json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write map: %v", err)
	}
	return path
}

// SPEC-014 What reaches a client from a refusal: "LoadInstalled's refusals,
// which name the file as maps/<id>.json and never by the path opened".
// VTT-161 VTT-162 VTT-275
func TestQANameLoadMapOfAnInstalledMapWithALongNameIsRefusedNamingFileAndField(t *testing.T) {
	mapsDir := filepath.Join(t.TempDir(), "maps")
	if err := os.MkdirAll(mapsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	edge := strings.Repeat("é", 128)
	qaNameWriteMap(t, mapsDir, "qa-long", "a"+edge)
	qaNameWriteMap(t, mapsDir, "qa-edge", edge)
	tb := qaNameServe(t, func(s *gateway.Server) *gateway.Server { return s.WithMapsDir(mapsDir) })
	dm := qaNameDial(t, tb, tb.dmToken)
	res := dm.send(qaNameLoadMap("r1", "qa-long"))
	if res.GetOk() {
		t.Fatalf("a map named in %d bytes loaded", len(edge)+1)
	}
	msg := res.GetError()
	t.Logf("refusal: %s", msg)
	if !strings.Contains(msg, "maps/qa-long.json") {
		t.Errorf("refusal %q does not name maps/qa-long.json", msg)
	}
	if !strings.Contains(strings.ReplaceAll(msg, "maps/qa-long.json", ""), "name") {
		t.Errorf("refusal %q does not name the field", msg)
	}
	if strings.Contains(msg, mapsDir) {
		t.Errorf("refusal %q discloses the server's maps directory", msg)
	}
	ok := dm.send(qaNameLoadMap("r2", "qa-edge"))
	if !ok.GetOk() || ok.GetSequence() != 1 {
		t.Fatalf("map at the bound: ok=%v seq=%d err=%q", ok.GetOk(), ok.GetSequence(), ok.GetError())
	}
	scene := dm.event(1).GetSceneCreated()
	if scene == nil || scene.GetName() != edge {
		t.Fatalf("first event after the load is not the whole-named scene: %v", dm.event(1))
	}
}

// SPEC-014 What reaches a client from a refusal: "Everything else is
// forwarded as written: ... Compile's, newEventID's and AppendBatch's".
// VTT-161 VTT-162 VTT-274
func TestQANameAHeldMapOverTheBoundIsRefusedByTheFold(t *testing.T) {
	path := qaNameWriteMap(t, t.TempDir(), "qa-held", qaNameASCII(qaNameBound))
	m, err := mapdef.Load(path)
	if err != nil {
		t.Fatalf("map at the bound refused by its loader: %v", err)
	}
	m.Name = qaNameASCII(qaNameBound + 1)
	envs, _, err := mapdef.Compile(m, "")
	if err != nil {
		t.Fatalf("mapdef.Compile: %v", err)
	}
	st := engine.NewState()
	var foldErr error
	for _, env := range envs {
		if foldErr = engine.Apply(st, env); foldErr != nil {
			break
		}
	}
	if foldErr == nil {
		t.Fatalf("the fold accepted a scene named in %d bytes", qaNameBound+1)
	}
	tb := qaNameServe(t, func(s *gateway.Server) *gateway.Server {
		return s.WithMaps(map[string]*mapdef.Map{m.ID: m})
	})
	dm := qaNameDial(t, tb, tb.dmToken)
	res := dm.send(qaNameLoadMap("r1", m.ID))
	t.Logf("refusal: %s", res.GetError())
	if res.GetOk() || !strings.Contains(res.GetError(), foldErr.Error()) {
		t.Fatalf("load_map: ok=%v err=%q, want refusal carrying %q", res.GetOk(), res.GetError(), foldErr.Error())
	}
	if next := dm.send(qaNameStart("r2", "qa follow")); !next.GetOk() || next.GetSequence() != 1 {
		t.Fatalf("follow-up: ok=%v seq=%d err=%q", next.GetOk(), next.GetSequence(), next.GetError())
	}
	qaNameHeadIs(t, tb, 1)
}

func qaNameServeAdventure(t *testing.T, name string) (*qaNameTable, *adventure.Adventure) {
	t.Helper()
	rs, err := rules.Load(filepath.Join("..", "adventure", "testdata", "ruleset"))
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	adv, err := adventure.Load(filepath.Join("..", "adventure", "testdata", "valid"), rs)
	if err != nil {
		t.Fatalf("adventure.Load: %v", err)
	}
	adv.Name = name
	tb := qaNameServe(t, func(s *gateway.Server) *gateway.Server {
		return s.WithRuleset(rs).WithAdventures(map[string]*adventure.Adventure{adv.ID: adv})
	})
	return tb, adv
}

// VTT-274
func TestQANameAnAdventureNamedAtTheBoundLoadsWhole(t *testing.T) {
	name := strings.Repeat("😀", 64)
	tb, adv := qaNameServeAdventure(t, name)
	dm := qaNameDial(t, tb, tb.dmToken)
	res := dm.send(qaNameLoadAdventure("r1", adv.ID))
	if !res.GetOk() {
		t.Fatalf("adventure named in %d bytes refused: %q", len(name), res.GetError())
	}
	var got *vttv1.AdventureLoaded
	for seq := res.GetSequence(); got == nil; seq++ {
		got = dm.event(seq).GetAdventureLoaded()
	}
	if got.GetName() != name {
		t.Fatalf("AdventureLoaded name is %d bytes, want %d", len(got.GetName()), len(name))
	}
}

// VTT-161 VTT-162 VTT-274
func TestQANameAHeldAdventureOverTheBoundIsRefusedByTheFold(t *testing.T) {
	name := "a" + strings.Repeat("é", 128)
	tb, adv := qaNameServeAdventure(t, name)
	want := qaNameFoldText(t, qaNameEvent("AdventureLoaded", name))
	dm := qaNameDial(t, tb, tb.dmToken)
	res := dm.send(qaNameLoadAdventure("r1", adv.ID))
	t.Logf("refusal: %s", res.GetError())
	if res.GetOk() || !strings.Contains(res.GetError(), want) {
		t.Fatalf("load_adventure: ok=%v err=%q, want refusal carrying %q", res.GetOk(), res.GetError(), want)
	}
	if next := dm.send(qaNameStart("r2", "qa follow")); !next.GetOk() || next.GetSequence() != 1 {
		t.Fatalf("follow-up: ok=%v seq=%d err=%q", next.GetOk(), next.GetSequence(), next.GetError())
	}
	qaNameHeadIs(t, tb, 1)
}

func qaNameToolDescription(t *testing.T, tool string, path ...string) string {
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

var qaNameStatedBound = regexp.MustCompile(`At most (\d+) bytes of UTF-8`)

// VTT-276
func TestQANameTheToolsStateTheBoundTheFoldEnforces(t *testing.T) {
	cases := []struct {
		tool, kind string
		path       []string
	}{
		{"add_actor", "ActorAdded", []string{"actor", "name"}},
		{"start_session", "SessionStarted", []string{"name"}},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			desc := qaNameToolDescription(t, c.tool, c.path...)
			m := qaNameStatedBound.FindStringSubmatch(desc)
			if m == nil {
				t.Fatalf("%s name description states no bound: %q", c.tool, desc)
			}
			n, _ := strconv.Atoi(m[1])
			if n != qaNameBound {
				t.Fatalf("%s states %d bytes, want %d", c.tool, n, qaNameBound)
			}
			if err := engine.Apply(engine.NewState(), qaNameEvent(c.kind, qaNameASCII(n))); err != nil {
				t.Fatalf("the fold refuses the stated bound: %v", err)
			}
			if engine.Apply(engine.NewState(), qaNameEvent(c.kind, qaNameASCII(n+1))) == nil {
				t.Fatalf("the fold accepts one byte over the stated bound")
			}
		})
	}
}
