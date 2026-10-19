package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
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

const qaCreatureWait = 10 * time.Second

const (
	qaCreatureHero   = "actor-m-hero"
	qaCreatureGoblin = "actor-z-goblin"
	qaCreatureImp    = "actor-a-imp"
	qaCreatureMap    = "qa-creature-hall"
)

const qaCreatureMapJSON = `{
  "format_version": 1,
  "id": "qa-creature-hall",
  "name": "QA Creature Hall",
  "grid_width": 8,
  "grid_height": 1,
  "tiles": {
    "0,0": "stone", "1,0": "stone", "2,0": "stone", "3,0": "stone-wall",
    "4,0": "stone", "5,0": "stone", "6,0": "stone", "7,0": "stone"
  }
}`

type qaCreatureConn struct {
	t       *testing.T
	ws      *websocket.Conn
	cancel  context.CancelFunc
	mu      sync.Mutex
	frames  []*vttv1.ServerFrame
	changed chan struct{}
	readErr error
	reqN    int
}

func qaCreatureDial(t *testing.T, base, token string, query url.Values) *qaCreatureConn {
	t.Helper()
	if query == nil {
		query = url.Values{}
	}
	query.Set("token", token)
	u := "ws" + base[len("http"):] + "/ws?" + query.Encode()
	ctx, cancel := context.WithCancel(context.Background())
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		cancel()
		t.Fatalf("dial %s: %v", u, err)
	}
	ws.SetReadLimit(1 << 24)
	qc := &qaCreatureConn{t: t, ws: ws, cancel: cancel, changed: make(chan struct{})}
	go qc.readLoop(ctx)
	t.Cleanup(func() {
		_ = ws.Close(websocket.StatusNormalClosure, "")
		cancel()
	})
	return qc
}

func (qc *qaCreatureConn) readLoop(ctx context.Context) {
	for {
		_, data, err := qc.ws.Read(ctx)
		qc.mu.Lock()
		if err != nil {
			qc.readErr = err
			close(qc.changed)
			qc.changed = make(chan struct{})
			qc.mu.Unlock()
			return
		}
		f := &vttv1.ServerFrame{}
		if uerr := protojson.Unmarshal(data, f); uerr != nil {
			qc.readErr = fmt.Errorf("decode %q: %w", data, uerr)
			close(qc.changed)
			qc.changed = make(chan struct{})
			qc.mu.Unlock()
			return
		}
		qc.frames = append(qc.frames, f)
		close(qc.changed)
		qc.changed = make(chan struct{})
		qc.mu.Unlock()
	}
}

func (qc *qaCreatureConn) mark() int {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	return len(qc.frames)
}

func (qc *qaCreatureConn) snapshot(from int) []*vttv1.ServerFrame {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	out := make([]*vttv1.ServerFrame, len(qc.frames)-from)
	copy(out, qc.frames[from:])
	return out
}

func (qc *qaCreatureConn) waitFor(from int, what string, ok func([]*vttv1.ServerFrame) bool) []*vttv1.ServerFrame {
	qc.t.Helper()
	deadline := time.NewTimer(qaCreatureWait)
	defer deadline.Stop()
	for {
		qc.mu.Lock()
		fs := make([]*vttv1.ServerFrame, len(qc.frames)-from)
		copy(fs, qc.frames[from:])
		ch := qc.changed
		rerr := qc.readErr
		qc.mu.Unlock()
		if ok(fs) {
			return fs
		}
		if rerr != nil {
			qc.t.Fatalf("waiting for %s: connection ended: %v\nframes: %s", what, rerr, qaCreatureDump(fs))
		}
		select {
		case <-ch:
		case <-deadline.C:
			qc.t.Fatalf("timed out waiting for %s\nframes: %s", what, qaCreatureDump(fs))
		}
	}
}

func qaCreatureDump(fs []*vttv1.ServerFrame) string {
	s := ""
	for i, f := range fs {
		b, _ := protojson.Marshal(f)
		s += fmt.Sprintf("\n  [%d] %s", i, b)
	}
	return s
}

func (qc *qaCreatureConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	qc.t.Helper()
	qc.mu.Lock()
	qc.reqN++
	id := "qa-creature-" + strconv.Itoa(qc.reqN)
	qc.mu.Unlock()
	cmd.RequestId = id
	b, err := protojson.Marshal(cmd)
	if err != nil {
		qc.t.Fatalf("marshal: %v", err)
	}
	if err := qc.ws.Write(context.Background(), websocket.MessageText, b); err != nil {
		qc.t.Fatalf("write: %v", err)
	}
	var res *vttv1.CommandResult
	qc.waitFor(0, "result "+id, func(fs []*vttv1.ServerFrame) bool {
		for _, f := range fs {
			if r := f.GetResult(); r != nil && r.GetRequestId() == id {
				res = r
				return true
			}
		}
		return false
	})
	return res
}

func (qc *qaCreatureConn) mustOK(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	qc.t.Helper()
	r := qc.send(cmd)
	if !r.GetOk() {
		b, _ := protojson.Marshal(cmd)
		qc.t.Fatalf("command %s refused: %s", b, r.GetError())
	}
	return r
}

func qaCreatureEvents(fs []*vttv1.ServerFrame) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range fs {
		if e := f.GetEvent(); e != nil {
			out = append(out, e)
		}
	}
	return out
}

type qaCreatureTable struct {
	t      *testing.T
	c      *campaign.Campaign
	ids    *identity.DB
	base   string
	dm     *qaCreatureConn
	dmTok  string
	scene  string
	sessID string
}

func qaCreatureOpen(t *testing.T) *qaCreatureTable {
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
	mapsDir := filepath.Join(t.TempDir(), "maps")
	if err := os.MkdirAll(mapsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mapsDir, qaCreatureMap+".json"), []byte(qaCreatureMapJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	artDir := filepath.Join(t.TempDir(), "art")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srv := gateway.New(c, ids).WithMapsDir(mapsDir).WithArtDir(artDir)
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(func() {
		hs.CloseClientConnections()
		hs.Close()
		_ = c.Close()
		_ = ids.Close()
	})
	dmTok, _, err := ids.CreateInvite("QA DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	tb := &qaCreatureTable{t: t, c: c, ids: ids, base: hs.URL, dmTok: dmTok}
	tb.dm = qaCreatureDial(t, hs.URL, dmTok, nil)
	return tb
}

func (tb *qaCreatureTable) invite(name string, role identity.Role) (token, id string) {
	tb.t.Helper()
	tok, id, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatal(err)
	}
	return tok, id
}

func (tb *qaCreatureTable) log(actorID string) []campaign.CharacterEntry {
	tb.t.Helper()
	l, err := tb.c.CharacterLog(actorID)
	if err != nil {
		tb.t.Fatalf("CharacterLog(%s): %v", actorID, err)
	}
	return l
}

func qaCreatureAddActor(id string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: kind},
	}}}
}

func qaCreatureGrant(actorID, pid string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
		ActorId: actorID, ParticipantId: pid, Kind: kind,
	}}}
}

func qaCreatureRevoke(actorID, pid string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RevokeActorControl{RevokeActorControl: &vttv1.RevokeActorControl{
		ActorId: actorID, ParticipantId: pid,
	}}}
}

func qaCreatureRemove(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveActor{RemoveActor: &vttv1.RemoveActor{ActorId: actorID}}}
}

func qaCreatureStart(name string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: name}}}
}

func qaCreatureEnd() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}}}
}

func qaCreatureNarrate(text string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: text}}}
}

func qaCreatureNote(key string, vis vttv1.NoteVisibility) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UpsertNote{UpsertNote: &vttv1.UpsertNote{
		Key: key, Title: key, Text: "text of " + key, Visibility: vis,
	}}}
}

func qaCreatureView(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID}}}
}

func qaCreatureLoadMap() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_LoadMap{LoadMap: &vttv1.LoadMap{MapId: qaCreatureMap}}}
}

func qaCreaturePlace(tokenID, sceneID, actorID string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
		TokenId: tokenID, SceneId: sceneID, ActorId: actorID, Position: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func qaCreatureMove(tokenID string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: tokenID, To: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func qaCreatureDeleteNote(key string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_DeleteNote{DeleteNote: &vttv1.DeleteNote{Key: key}}}
}

const (
	qaCreatureRat      = "actor-q-rat"
	qaCreatureTokHero  = "tok-hero"
	qaCreatureTokGob   = "tok-goblin"
	qaCreatureTokRat   = "tok-a-rat"
	qaCreatureOpenName = "S2"
)

func qaCreatureSetup(t *testing.T) (tb *qaCreatureTable, pTok, pID string) {
	t.Helper()
	tb = qaCreatureOpen(t)
	tb.dm.mustOK(qaCreatureStart("S1"))
	tb.dm.mustOK(qaCreatureAddActor(qaCreatureHero, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	tb.dm.mustOK(qaCreatureAddActor(qaCreatureGoblin, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	tb.dm.mustOK(qaCreatureAddActor(qaCreatureRat, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	tb.dm.mustOK(qaCreatureLoadMap())
	tb.dm.mustOK(qaCreatureNote("qa-b-lore", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	tb.dm.mustOK(qaCreatureNote("qa-a-lore", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	tb.dm.mustOK(qaCreatureNote("qa-c-gone", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
	tb.dm.mustOK(qaCreatureDeleteNote("qa-c-gone"))
	tb.dm.mustOK(qaCreatureNote("qa-secret", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET))
	tb.dm.mustOK(qaCreatureNarrate("in S1"))
	tb.dm.mustOK(qaCreatureEnd())
	last := tb.dm.mustOK(qaCreatureStart(qaCreatureOpenName))
	pTok, pID = tb.invite("QA Player", identity.RolePlayer)
	tb.dm.mustOK(qaCreatureGrant(qaCreatureHero, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	w := tb.worldThrough(last.GetSequence())
	for _, e := range w {
		if sc := e.GetSceneCreated(); sc != nil {
			tb.scene = sc.GetSceneId()
		}
	}
	if tb.scene == "" {
		t.Fatal("no SceneCreated in the world")
	}
	tb.dm.mustOK(qaCreaturePlace(qaCreatureTokHero, tb.scene, qaCreatureHero, 0, 0))
	tb.dm.mustOK(qaCreaturePlace(qaCreatureTokGob, tb.scene, qaCreatureGoblin, 6, 0))
	tb.dm.mustOK(qaCreaturePlace(qaCreatureTokRat, tb.scene, qaCreatureRat, 4, 0))
	tb.sessID = tb.sessionID(qaCreatureOpenName)
	return tb, pTok, pID
}

func (tb *qaCreatureTable) worldThrough(seq int64) []*vttv1.Envelope {
	tb.t.Helper()
	fs := tb.dm.waitFor(0, "world event "+strconv.FormatInt(seq, 10), func(fs []*vttv1.ServerFrame) bool {
		for _, e := range qaCreatureEvents(fs) {
			if e.GetSequence() >= seq {
				return true
			}
		}
		return false
	})
	return qaCreatureEvents(fs)
}

func (tb *qaCreatureTable) sessionID(name string) string {
	tb.t.Helper()
	for _, e := range qaCreatureEvents(tb.dm.snapshot(0)) {
		if ss := e.GetSessionStarted(); ss != nil && ss.GetName() == name {
			return e.GetSessionId()
		}
	}
	tb.t.Fatalf("no world SessionStarted %q", name)
	return ""
}

func (tb *qaCreatureTable) narrate(text string) int64 {
	tb.t.Helper()
	return tb.dm.mustOK(qaCreatureNarrate(text)).GetSequence()
}

func qaCreatureLastVC(fs []*vttv1.ServerFrame) int {
	idx := -1
	for i, f := range fs {
		if f.GetViewpointChanged() != nil {
			idx = i
		}
	}
	return idx
}

func qaCreatureVCs(fs []*vttv1.ServerFrame) []string {
	var out []string
	for _, f := range fs {
		if vc := f.GetViewpointChanged(); vc != nil {
			out = append(out, vc.GetActorId())
		}
	}
	return out
}

func (qc *qaCreatureConn) expectSwitch(from int, actor string, log []campaign.CharacterEntry, req string) {
	qc.t.Helper()
	fs := qc.waitFor(from, "switch to "+strconv.Quote(actor)+" ("+req+")", func(fs []*vttv1.ServerFrame) bool {
		i := qaCreatureLastVC(fs)
		if i < 0 || fs[i].GetViewpointChanged().GetActorId() != actor {
			return false
		}
		return len(qaCreatureEvents(fs[i+1:])) >= len(log)
	})
	if vcs := qaCreatureVCs(fs); len(vcs) != 1 {
		qc.t.Errorf("%s: want one ViewpointChanged naming %q, got %q", req, actor, vcs)
	}
	i := qaCreatureLastVC(fs)
	evs := qaCreatureEvents(fs[i+1:])
	if len(evs) != len(log) {
		qc.t.Errorf("%s: after ViewpointChanged %q want %d entries, got %d%s", req, actor, len(log), len(evs), qaCreatureDump(fs))
		return
	}
	for k := range log {
		if !proto.Equal(evs[k], log[k].Env) {
			a, _ := protojson.Marshal(evs[k])
			b, _ := protojson.Marshal(log[k].Env)
			qc.t.Errorf("%s: entry %d after ViewpointChanged %q:\n got  %s\n want %s", req, k, actor, a, b)
		}
	}
}

func (qc *qaCreatureConn) expectStay(from int, log []campaign.CharacterEntry, req string) {
	qc.t.Helper()
	if len(log) == 0 {
		qc.t.Fatalf("%s: expectStay needs a log with an entry", req)
	}
	lastEnv := log[len(log)-1].Env
	fs := qc.waitFor(from, "the watched log's last entry ("+req+")", func(fs []*vttv1.ServerFrame) bool {
		for _, e := range qaCreatureEvents(fs) {
			if proto.Equal(e, lastEnv) {
				return true
			}
		}
		return false
	})
	if vcs := qaCreatureVCs(fs); len(vcs) != 0 {
		qc.t.Errorf("%s: want the seat to stay, got ViewpointChanged %q", req, vcs)
	}
	for _, e := range qaCreatureEvents(fs) {
		found := false
		for _, le := range log {
			if proto.Equal(e, le.Env) {
				found = true
				break
			}
		}
		if !found {
			b, _ := protojson.Marshal(e)
			qc.t.Errorf("%s: sent an entry that is not of the watched log: %s", req, b)
		}
	}
}

func qaCreatureContiguous(t *testing.T, log []campaign.CharacterEntry, req string) {
	t.Helper()
	for i, e := range log {
		if e.Env.GetSequence() != int64(i+1) {
			t.Errorf("%s: entry %d carries sequence %d, want %d", req, i, e.Env.GetSequence(), i+1)
		}
	}
}

func qaCreaturePrefix(t *testing.T, before, log []campaign.CharacterEntry, req string) {
	t.Helper()
	if len(log) < len(before) {
		t.Fatalf("%s: log shrank from %d to %d entries", req, len(before), len(log))
	}
	for i := range before {
		if before[i].WorldSeq != log[i].WorldSeq || !proto.Equal(before[i].Env, log[i].Env) {
			a, _ := protojson.Marshal(log[i].Env)
			b, _ := protojson.Marshal(before[i].Env)
			t.Errorf("%s: entry %d changed:\n now  ws=%d %s\n was  ws=%d %s", req, i, log[i].WorldSeq, a, before[i].WorldSeq, b)
		}
	}
}

func qaCreatureHeld(log []campaign.CharacterEntry) []string {
	held := map[string]bool{}
	for _, e := range log {
		switch {
		case e.Env.GetTokenPlaced() != nil:
			held[e.Env.GetTokenPlaced().GetTokenId()] = true
		case e.Env.GetTokenHidden() != nil:
			delete(held, e.Env.GetTokenHidden().GetTokenId())
		case e.Env.GetTokenRemoved() != nil:
			delete(held, e.Env.GetTokenRemoved().GetTokenId())
		}
	}
	out := make([]string, 0, len(held))
	for id := range held {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func qaCreatureLine(e campaign.CharacterEntry) string {
	b, _ := protojson.Marshal(e.Env)
	return fmt.Sprintf("ws=%d %s", e.WorldSeq, b)
}

// VTT-316 VTT-317 VTT-321
func TestQACreatureGrantOpensTheCreaturesOwnLog(t *testing.T) {
	tb, _, pID := qaCreatureSetup(t)
	tb.dm.mustOK(qaCreatureMove(qaCreatureTokGob, 5, 0))
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	tb.narrate("a monster the player holds")

	// VTT-316
	if l := tb.log(qaCreatureGoblin); len(l) != 0 {
		t.Errorf("VTT-316: a goblin that was never a party member has %d log entries", len(l))
	}

	// VTT-151
	if r := tb.dm.send(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED)); r.GetOk() {
		t.Errorf("VTT-151: a grant_actor_control with no kind was accepted")
	}

	g := tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)).GetSequence()
	after := tb.narrate("after the grant")
	world := tb.worldThrough(after)
	log := tb.log(qaCreatureGoblin)
	for _, e := range log {
		t.Logf("goblin %s", qaCreatureLine(e))
	}

	// VTT-321
	qaCreatureContiguous(t, log, "VTT-321")

	// VTT-317
	k := -1
	for i, e := range log {
		if e.WorldSeq != 0 {
			k = i
			break
		}
	}
	if k <= 0 {
		t.Fatalf("VTT-317: want opening entries before the grant's frames, first caused entry at %d", k)
	}
	if log[k].WorldSeq != g {
		t.Errorf("VTT-317: the first caused entry is of world event %d, want the grant %d", log[k].WorldSeq, g)
	}
	earlierIDs := map[string]bool{}
	for _, e := range world {
		if e.GetSequence() < g {
			earlierIDs[e.GetEventId()] = true
		}
	}
	for i, e := range log {
		if e.WorldSeq != 0 && e.WorldSeq < g {
			t.Errorf("VTT-317: entry %d forwarded from world event %d, appended before the grant", i, e.WorldSeq)
		}
		if id := e.Env.GetEventId(); id != "" && earlierIDs[id] {
			t.Errorf("VTT-317: entry %d carries the id of an event appended before the grant: %s", i, qaCreatureLine(e))
		}
	}
	opening := log[:k]
	ss := opening[0].Env.GetSessionStarted()
	if ss == nil || ss.GetName() != qaCreatureOpenName || opening[0].Env.GetSessionId() != tb.sessID {
		t.Errorf("VTT-317: the opening's first entry is %s, want SessionStarted %q of session %s", qaCreatureLine(opening[0]), qaCreatureOpenName, tb.sessID)
	}
	var noteKeys []string
	heroIntroduced := false
	for i, e := range opening {
		if i > 0 && (e.Env.GetSessionStarted() != nil || e.Env.GetSessionEnded() != nil) {
			t.Errorf("VTT-317: the opening holds a second session entry: %s", qaCreatureLine(e))
		}
		if n := e.Env.GetNoteUpserted(); n != nil {
			noteKeys = append(noteKeys, n.GetKey())
		}
		if a := e.Env.GetActorAdded(); a != nil {
			switch a.GetActor().GetActorId() {
			case qaCreatureHero:
				heroIntroduced = true
			case qaCreatureGoblin:
				t.Errorf("VTT-317: the opening introduces the goblin, which was not of the party before the grant: %s", qaCreatureLine(e))
			}
		}
	}
	if !heroIntroduced {
		t.Errorf("VTT-317: the opening does not introduce the party's hero")
	}
	if fmt.Sprint(noteKeys) != fmt.Sprint([]string{"qa-a-lore", "qa-b-lore"}) {
		t.Errorf("VTT-317: the opening's public notes are %q, want [qa-a-lore qa-b-lore]", noteKeys)
	}
	for _, e := range log {
		if n := e.Env.GetNoteUpserted(); n != nil && (n.GetKey() == "qa-secret" || n.GetKey() == "qa-c-gone") {
			t.Errorf("VTT-317: the log holds note %q, which was not a public note before the grant", n.GetKey())
		}
		if e.Env.GetSessionStarted() != nil && e.Env.GetSessionStarted().GetName() == "S1" {
			t.Errorf("VTT-317: the log holds the ended session S1")
		}
	}

	// VTT-319
	sawAfter := false
	for _, e := range log {
		if e.WorldSeq == after {
			sawAfter = true
		}
	}
	if !sawAfter {
		t.Errorf("VTT-319: the goblin, a party member after the narration, was written nothing for it")
	}
}

// VTT-343 VTT-342 VTT-391 VTT-340
func TestQACreatureSetViewpointNeedsAPartyMemberTheyControl(t *testing.T) {
	tb, pTok, pID := qaCreatureSetup(t)
	pl := qaCreatureDial(t, tb.base, pTok, nil)
	tb.narrate("sync")
	pl.expectSwitch(0, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340 open")

	// VTT-343
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	m := pl.mark()
	nonParty := pl.send(qaCreatureView(qaCreatureGoblin))
	if nonParty.GetOk() {
		t.Errorf("VTT-343: set_viewpoint on a non-party goblin the player controls was accepted")
	}
	absent := pl.send(qaCreatureView("actor-absent"))
	// VTT-347
	if absent.GetOk() || strings.ReplaceAll(nonParty.GetError(), qaCreatureGoblin, "actor-absent") != absent.GetError() {
		t.Errorf("VTT-347: refusals differ: non-party %q, absent %q", nonParty.GetError(), absent.GetError())
	}

	// VTT-391
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	if k := tb.c.Actors()[qaCreatureGoblin].GetKind(); k != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Errorf("VTT-391: a re-grant as party member left the goblin %v", k)
	}
	tb.narrate("sync after the party grant")
	// VTT-340 VTT-193
	pl.expectStay(m, tb.log(qaCreatureHero), "VTT-340 stay")

	// VTT-343
	p2Tok, _ := tb.invite("QA Other Player", identity.RolePlayer)
	p2 := qaCreatureDial(t, tb.base, p2Tok, nil)
	notTheirs := p2.send(qaCreatureView(qaCreatureGoblin))
	if notTheirs.GetOk() {
		t.Errorf("VTT-343: set_viewpoint on a goblin the player does not control was accepted")
	}
	// VTT-347
	if strings.ReplaceAll(notTheirs.GetError(), qaCreatureGoblin, "actor-absent") != absent.GetError() {
		t.Errorf("VTT-347: refusals differ: not theirs %q, absent %q", notTheirs.GetError(), absent.GetError())
	}

	// VTT-343 VTT-342
	m = pl.mark()
	if r := pl.send(qaCreatureView(qaCreatureGoblin)); !r.GetOk() {
		t.Fatalf("VTT-343: set_viewpoint on the goblin granted as a party member refused: %s", r.GetError())
	}
	tb.narrate("sync on the goblin")
	pl.expectSwitch(m, qaCreatureGoblin, tb.log(qaCreatureGoblin), "VTT-342 perch on the goblin")

	m = pl.mark()
	if r := pl.send(qaCreatureView(qaCreatureHero)); !r.GetOk() {
		t.Fatalf("VTT-343: set_viewpoint back to the hero refused: %s", r.GetError())
	}
	n := tb.narrate("sync back on the hero")
	pl.expectSwitch(m, qaCreatureHero, tb.log(qaCreatureHero), "VTT-342 perch back on the hero")

	if k := tb.c.Actors()[qaCreatureGoblin].GetKind(); k != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Errorf("ruling: watching the hero turned the goblin %v", k)
	}
	written := false
	for _, e := range tb.log(qaCreatureGoblin) {
		if e.WorldSeq == n {
			written = true
		}
	}
	if !written {
		t.Errorf("VTT-319: the goblin's log was not written while the player watched the hero")
	}
}

// VTT-350
func TestQACreatureMoveGateJudgesByTheWatchedCharactersEyes(t *testing.T) {
	tb, pTok, pID := qaCreatureSetup(t)
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	pl := qaCreatureDial(t, tb.base, pTok, nil)
	tb.narrate("sync")
	pl.expectSwitch(0, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340 open")

	if r := pl.send(qaCreatureMove(qaCreatureTokGob, 5, 0)); r.GetOk() || r.GetError() != unseenSquare {
		t.Errorf("VTT-350: watching the hero, a move onto 5,0, which only the goblin sees: ok=%v %q", r.GetOk(), r.GetError())
	}
	if r := pl.send(qaCreatureMove(qaCreatureTokHero, 1, 0)); !r.GetOk() {
		t.Errorf("VTT-350: watching the hero, a move onto 1,0, which the hero sees, was refused: %s", r.GetError())
	}

	m := pl.mark()
	if r := pl.send(qaCreatureView(qaCreatureGoblin)); !r.GetOk() {
		t.Fatalf("VTT-343: set_viewpoint on the goblin refused: %s", r.GetError())
	}
	tb.narrate("sync on the goblin")
	pl.expectSwitch(m, qaCreatureGoblin, tb.log(qaCreatureGoblin), "VTT-342")

	if r := pl.send(qaCreatureMove(qaCreatureTokHero, 2, 0)); r.GetOk() || r.GetError() != unseenSquare {
		t.Errorf("VTT-350: watching the goblin, a move onto 2,0, which only the hero sees: ok=%v %q", r.GetOk(), r.GetError())
	}
	if r := pl.send(qaCreatureMove(qaCreatureTokGob, 5, 0)); !r.GetOk() {
		t.Errorf("VTT-350: watching the goblin, a move onto 5,0, which the goblin sees, was refused: %s", r.GetError())
	}
}

func qaCreatureRunning(t *testing.T) (tb *qaCreatureTable, pl *qaCreatureConn, pTok, pID string) {
	t.Helper()
	tb, pTok, pID = qaCreatureSetup(t)
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	tb.narrate("the goblin joins")
	pl = qaCreatureDial(t, tb.base, pTok, nil)
	tb.narrate("sync")
	pl.expectSwitch(0, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340 open")
	m := pl.mark()
	if r := pl.send(qaCreatureView(qaCreatureGoblin)); !r.GetOk() {
		t.Fatalf("VTT-343: set_viewpoint on the goblin refused: %s", r.GetError())
	}
	tb.narrate("sync on the goblin")
	pl.expectSwitch(m, qaCreatureGoblin, tb.log(qaCreatureGoblin), "VTT-342")
	return tb, pl, pTok, pID
}

// VTT-391 VTT-316 VTT-326 VTT-340 VTT-342 VTT-343 VTT-350
func TestQACreatureANonPartyGrantEndsTheRun(t *testing.T) {
	tb, pl, _, pID := qaCreatureRunning(t)
	before := tb.log(qaCreatureGoblin)

	m := pl.mark()
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	tb.narrate("the run is over")
	pl.expectSwitch(m, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340, VTT-342 run ends by a grant")

	// VTT-391
	if k := tb.c.Actors()[qaCreatureGoblin].GetKind(); k != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Errorf("VTT-391: a non-party grant to the goblin's own controller left it %v", k)
	}
	// VTT-316
	after := tb.log(qaCreatureGoblin)
	if len(after) != len(before) {
		t.Errorf("VTT-316: the goblin's log grew from %d to %d entries after it left the party", len(before), len(after))
		for _, e := range after[len(before):] {
			t.Logf("  %s", qaCreatureLine(e))
		}
	}
	qaCreaturePrefix(t, before, after, "VTT-326")
	// VTT-343
	if r := pl.send(qaCreatureView(qaCreatureGoblin)); r.GetOk() {
		t.Errorf("VTT-343: set_viewpoint on the goblin after its non-party grant was accepted")
	}
	// VTT-350
	if r := pl.send(qaCreatureMove(qaCreatureTokGob, 5, 0)); r.GetOk() || r.GetError() != unseenSquare {
		t.Errorf("VTT-350: after the run, a move onto 5,0, which the hero cannot see: ok=%v %q", r.GetOk(), r.GetError())
	}

	m = pl.mark()
	tb.dm.mustOK(qaCreatureRevoke(qaCreatureGoblin, pID))
	tb.narrate("revoked")
	pl.expectStay(m, tb.log(qaCreatureHero), "VTT-340 stay after the revocation")
	if l := tb.log(qaCreatureGoblin); len(l) != len(before) {
		t.Errorf("VTT-316: the goblin's log grew from %d to %d entries after the revocation", len(before), len(l))
	}
	for _, id := range tb.c.Actors()[qaCreatureGoblin].GetControllerIds() {
		if id == pID {
			t.Errorf("the revocation left the player a controller of the goblin")
		}
	}
}

// VTT-316 VTT-326 VTT-340 VTT-342
func TestQACreatureRemoveActorEndsTheRun(t *testing.T) {
	tb, pl, _, _ := qaCreatureRunning(t)
	before := tb.log(qaCreatureGoblin)

	m := pl.mark()
	r := tb.dm.mustOK(qaCreatureRemove(qaCreatureGoblin)).GetSequence()
	n := tb.narrate("the goblin is gone")
	pl.expectSwitch(m, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340, VTT-342 run ends by removal")

	var removedAt int64
	for _, e := range tb.worldThrough(n) {
		if a := e.GetActorRemoved(); a != nil && a.GetActorId() == qaCreatureGoblin {
			removedAt = e.GetSequence()
		}
	}
	if removedAt < r {
		t.Fatalf("no ActorRemoved for the goblin at or after %d", r)
	}
	after := tb.log(qaCreatureGoblin)
	qaCreaturePrefix(t, before, after, "VTT-326")
	for _, e := range after[len(before):] {
		t.Logf("goblin after removal %s", qaCreatureLine(e))
		// VTT-316
		if e.WorldSeq == 0 || e.WorldSeq >= removedAt {
			t.Errorf("VTT-316: the goblin's log was written after its removal: %s", qaCreatureLine(e))
		}
	}
}

// VTT-392 VTT-319 VTT-181 VTT-340 VTT-342 VTT-348
func TestQACreatureRevokingTheLastControllerLeavesAPartyMember(t *testing.T) {
	tb, pl, _, pID := qaCreatureRunning(t)

	m := pl.mark()
	v := tb.dm.mustOK(qaCreatureRevoke(qaCreatureGoblin, pID)).GetSequence()
	n := tb.narrate("nobody runs the goblin")
	pl.expectSwitch(m, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340, VTT-342 run ends by revocation")

	a := tb.c.Actors()[qaCreatureGoblin]
	// VTT-392
	if a.GetKind() != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Errorf("VTT-392: revoking the goblin's last controller left it %v", a.GetKind())
	}
	if len(a.GetControllerIds()) != 0 {
		t.Errorf("the goblin still has controllers %q", a.GetControllerIds())
	}
	// VTT-319
	log := tb.log(qaCreatureGoblin)
	written := false
	for _, e := range log {
		if e.WorldSeq == n {
			written = true
		}
	}
	if !written {
		t.Errorf("VTT-319: the goblin's log was not written for the narration after the revocation")
	}
	// VTT-181
	if err := gateway.MayWatch(&identity.Participant{ID: "qa-spectator", Role: identity.RoleSpectator}, qaCreatureGoblin, tb.c.Actors()); err != nil {
		t.Errorf("VTT-181: a spectator may not watch the goblin after the revocation: %v", err)
	}
	// VTT-348
	fs := pl.snapshot(m)
	vc := qaCreatureLastVC(fs)
	for _, e := range qaCreatureEvents(fs[:vc]) {
		for _, le := range log {
			if le.WorldSeq >= v && proto.Equal(e, le.Env) {
				t.Errorf("VTT-348: an entry of the revocation was sent before the switch: %s", qaCreatureLine(le))
			}
		}
	}
}

func qaCreatureResumeEntries(endHeld bool, startName, startID string, held []string) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	if endHeld {
		out = append(out, &vttv1.Envelope{Payload: &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}})
	}
	if startName != "" {
		out = append(out, &vttv1.Envelope{SessionId: startID, Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: startName}}})
	}
	for _, id := range held {
		out = append(out, &vttv1.Envelope{Payload: &vttv1.Envelope_TokenHidden{TokenHidden: &vttv1.TokenHidden{TokenId: id}}})
	}
	return out
}

func qaCreatureExpectResume(t *testing.T, paused, log []campaign.CharacterEntry, want []*vttv1.Envelope, g int64) {
	t.Helper()
	for _, e := range log {
		t.Logf("goblin %s", qaCreatureLine(e))
	}
	qaCreaturePrefix(t, paused, log, "VTT-326")
	qaCreatureContiguous(t, log, "VTT-321")
	rest := log[len(paused):]
	k := len(rest)
	for i, e := range rest {
		if e.WorldSeq != 0 {
			k = i
			break
		}
	}
	got := rest[:k]
	if len(got) != len(want) {
		t.Errorf("VTT-318: the resume wrote %d entries before the decision's frames, want %d", len(got), len(want))
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		w := proto.Clone(want[i]).(*vttv1.Envelope)
		w.Sequence = got[i].Env.GetSequence()
		if !proto.Equal(got[i].Env, w) {
			b, _ := protojson.Marshal(w)
			t.Errorf("VTT-318: resume entry %d is %s, want %s", i, qaCreatureLine(got[i]), b)
		}
	}
	if k == len(rest) || rest[k].WorldSeq != g {
		t.Errorf("VTT-318: the resume entries are not followed by the resuming event %d's frames", g)
	}
}

// VTT-318 VTT-316 VTT-321 VTT-326 VTT-340 VTT-342
func TestQACreatureASecondPartyGrantResumesTheOldLog(t *testing.T) {
	cases := []struct {
		name      string
		pause     func(tb *qaCreatureTable)
		endHeld   bool
		startName string
	}{
		{name: "the session it holds is still open", pause: func(tb *qaCreatureTable) {}},
		{name: "another session is open", endHeld: true, startName: "S4", pause: func(tb *qaCreatureTable) {
			tb.dm.mustOK(qaCreatureEnd())
			tb.dm.mustOK(qaCreatureStart("S3"))
			tb.narrate("in S3")
			tb.dm.mustOK(qaCreatureEnd())
			tb.dm.mustOK(qaCreatureStart("S4"))
		}},
		{name: "no session is open", endHeld: true, pause: func(tb *qaCreatureTable) {
			tb.dm.mustOK(qaCreatureEnd())
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tb, pl, _, pID := qaCreatureRunning(t)
			tb.dm.mustOK(qaCreatureMove(qaCreatureTokGob, 5, 0))
			tb.dm.mustOK(qaCreatureAddActor("actor-b-bat", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
			tb.dm.mustOK(qaCreaturePlace("tok-0-bat", tb.scene, "actor-b-bat", 7, 0))
			tb.narrate("still running")

			m := pl.mark()
			g2 := tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_NON_PARTY)).GetSequence()
			paused := tb.log(qaCreatureGoblin)
			held := qaCreatureHeld(paused)
			if fmt.Sprint(held) != fmt.Sprint([]string{"tok-0-bat", qaCreatureTokRat, qaCreatureTokGob}) {
				t.Fatalf("degenerate fixture: the paused log holds tokens %q", held)
			}
			during := tb.narrate("during the pause")
			tb.dm.mustOK(qaCreatureNote("qa-d-pause", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC))
			tb.dm.mustOK(qaCreatureMove(qaCreatureTokGob, 6, 0))
			tc.pause(tb)
			g3 := tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)).GetSequence()
			n := tb.narrate("resumed")
			pl.expectSwitch(m, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340, VTT-342 the pause moves the seat")

			startID := ""
			if tc.startName != "" {
				startID = tb.sessionID(tc.startName)
			}
			log := tb.log(qaCreatureGoblin)
			qaCreatureExpectResume(t, paused, log, qaCreatureResumeEntries(tc.endHeld, tc.startName, startID, held), g3)
			// VTT-318 VTT-319
			placed := false
			for _, e := range log {
				if tp := e.Env.GetTokenPlaced(); e.WorldSeq == g3 && tp != nil && tp.GetTokenId() == qaCreatureTokGob {
					placed = tp.GetPosition().GetX() == 6 && tp.GetPosition().GetY() == 0
				}
			}
			if !placed {
				t.Errorf("VTT-318: the resuming grant's frames do not place the goblin's token on its present square 6,0")
			}
			for _, e := range log {
				// VTT-316
				if e.WorldSeq >= g2 && e.WorldSeq < g3 {
					t.Errorf("VTT-316: the paused log was written for world event %d: %s", e.WorldSeq, qaCreatureLine(e))
				}
				if e.WorldSeq == during {
					t.Errorf("VTT-316: the paused log holds the narration made during the pause")
				}
				if nu := e.Env.GetNoteUpserted(); nu != nil && nu.GetKey() == "qa-d-pause" {
					t.Errorf("VTT-316: the log holds the public note made during the pause: %s", qaCreatureLine(e))
				}
				if ss := e.Env.GetSessionStarted(); ss != nil && ss.GetName() == "S3" {
					t.Errorf("VTT-318: the log holds S3, which opened and closed during the pause")
				}
			}
			resumedWritten := false
			for _, e := range log {
				if e.WorldSeq == n {
					resumedWritten = true
				}
			}
			if !resumedWritten {
				t.Errorf("VTT-319: the resumed log was not written for the narration after the resume")
			}

			// VTT-342
			m = pl.mark()
			if r := pl.send(qaCreatureView(qaCreatureGoblin)); !r.GetOk() {
				t.Fatalf("VTT-343: set_viewpoint on the resumed goblin refused: %s", r.GetError())
			}
			tb.narrate("sync on the resumed goblin")
			pl.expectSwitch(m, qaCreatureGoblin, tb.log(qaCreatureGoblin), "VTT-342 the resumed log")
		})
	}
}

// VTT-318 VTT-316 VTT-321 VTT-326 VTT-343
func TestQACreatureAnActorAddedAgainUnderItsIdContinuesItsLog(t *testing.T) {
	tb, pl, _, pID := qaCreatureRunning(t)
	m := pl.mark()
	tb.dm.mustOK(qaCreatureRemove(qaCreatureGoblin))
	paused := tb.log(qaCreatureGoblin)
	held := qaCreatureHeld(paused)
	tb.dm.mustOK(qaCreatureEnd())
	tb.dm.mustOK(qaCreatureStart("S3"))
	tb.dm.mustOK(qaCreatureAddActor(qaCreatureGoblin, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	tb.dm.mustOK(qaCreaturePlace("tok-goblin-again", tb.scene, qaCreatureGoblin, 5, 0))
	if l := tb.log(qaCreatureGoblin); len(l) != len(paused) {
		t.Errorf("VTT-316: the log of a goblin added again as non-party grew from %d to %d", len(paused), len(l))
	}
	g := tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER)).GetSequence()
	tb.narrate("the goblin is back")
	pl.expectSwitch(m, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340, VTT-342 the removal moves the seat")

	log := tb.log(qaCreatureGoblin)
	qaCreatureExpectResume(t, paused, log, qaCreatureResumeEntries(true, "S3", tb.sessionID("S3"), held), g)

	m = pl.mark()
	if r := pl.send(qaCreatureView(qaCreatureGoblin)); !r.GetOk() {
		t.Fatalf("VTT-343: set_viewpoint on the goblin added again refused: %s", r.GetError())
	}
	tb.narrate("sync on the goblin added again")
	pl.expectSwitch(m, qaCreatureGoblin, tb.log(qaCreatureGoblin), "VTT-342 the continued log")
}

// VTT-339 VTT-349 VTT-368 VTT-384
func TestQACreatureAReconnectNamingTheCreaturesLog(t *testing.T) {
	tb, _, pTok, pID := qaCreatureRunning(t)
	tb.narrate("one more")
	tb.narrate("and another")
	log := tb.log(qaCreatureGoblin)
	cursor := len(log) - 3
	if cursor < 1 {
		t.Fatalf("degenerate fixture: the goblin's log has %d entries", len(log))
	}

	re := qaCreatureDial(t, tb.base, pTok, url.Values{"after": {strconv.Itoa(cursor)}, "log": {qaCreatureGoblin}})
	fs := re.waitFor(0, "the catch-up after the cursor", func(fs []*vttv1.ServerFrame) bool {
		return len(qaCreatureEvents(fs)) >= len(log)-cursor
	})
	if h := fs[0].GetCatchUpHead(); h == nil || h.GetHeadSequence() != int64(len(log)) {
		t.Errorf("VTT-384: the first frame is %s, want CatchUpHead %d", qaCreatureDump(fs[:1]), len(log))
	}
	if vcs := qaCreatureVCs(fs); len(vcs) != 1 || vcs[0] != qaCreatureGoblin {
		t.Errorf("VTT-339: a reconnect naming the goblin's log was told %q", vcs)
	}
	evs := qaCreatureEvents(fs)
	if len(evs) != len(log)-cursor {
		t.Errorf("VTT-339: want the %d entries after the cursor, got %d", len(log)-cursor, len(evs))
	}
	for i := 0; i < len(evs) && cursor+i < len(log); i++ {
		if !proto.Equal(evs[i], log[cursor+i].Env) {
			b, _ := protojson.Marshal(evs[i])
			t.Errorf("VTT-339: entry %d after the cursor is %s, want %s", i, b, qaCreatureLine(log[cursor+i]))
		}
	}
	if i := qaCreatureLastVC(fs); i >= 0 {
		for _, f := range fs[:i] {
			if f.GetEvent() != nil {
				t.Errorf("VTT-368: an event came before the opening ViewpointChanged")
			}
		}
	}

	// VTT-349
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	tb.dm.mustOK(qaCreatureRevoke(qaCreatureGoblin, pID))
	tb.narrate("over")
	hero := tb.log(qaCreatureHero)
	late := qaCreatureDial(t, tb.base, pTok, url.Values{"after": {strconv.Itoa(cursor)}, "log": {qaCreatureGoblin}})
	late.expectSwitch(0, qaCreatureHero, hero, "VTT-349: a cursor of a log the player may no longer watch is ignored")
	if h := late.snapshot(0)[0].GetCatchUpHead(); h == nil || h.GetHeadSequence() != int64(len(hero)) {
		t.Errorf("VTT-384: the first frame is %s, want CatchUpHead %d", qaCreatureDump(late.snapshot(0)[:1]), len(hero))
	}
}

// VTT-340 VTT-342
func TestQACreatureTheDefaultIsTheFirstPartyMemberById(t *testing.T) {
	tb, pTok, pID := qaCreatureSetup(t)
	tb.dm.mustOK(qaCreatureAddActor(qaCreatureImp, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	pl := qaCreatureDial(t, tb.base, pTok, nil)
	tb.narrate("sync")
	pl.expectSwitch(0, qaCreatureHero, tb.log(qaCreatureHero), "VTT-340 open")

	// VTT-340
	m := pl.mark()
	agTok, _ := tb.invite("QA Agent", identity.RoleAgent)
	ag := qaCreatureDial(t, tb.base, agTok, nil)
	ag.mustOK(qaCreatureGrant(qaCreatureImp, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	tb.narrate("the imp joins")
	pl.expectStay(m, tb.log(qaCreatureHero), "VTT-340 stays on the hero")

	// VTT-340
	qTok, qID := tb.invite("QA Empty-handed Player", identity.RolePlayer)
	q := qaCreatureDial(t, tb.base, qTok, nil)
	q.expectSwitch(0, "", nil, "VTT-340 nobody")
	if h := q.snapshot(0)[0].GetCatchUpHead(); h == nil || h.GetHeadSequence() != 0 {
		t.Errorf("VTT-374: a seat on nobody opened with %s, want CatchUpHead 0", qaCreatureDump(q.snapshot(0)[:1]))
	}
	m = q.mark()
	ag.mustOK(qaCreatureGrant(qaCreatureGoblin, qID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	tb.narrate("the goblin is Q's")
	q.expectSwitch(m, qaCreatureGoblin, tb.log(qaCreatureGoblin), "VTT-340, VTT-342 nobody to the goblin")
	if r := q.send(qaCreatureMove(qaCreatureTokGob, 5, 0)); !r.GetOk() {
		t.Errorf("VTT-350: watching the goblin it runs, a move onto 5,0, which the goblin sees, was refused: %s", r.GetError())
	}

	m = q.mark()
	ag.mustOK(qaCreatureGrant(qaCreatureGoblin, qID, vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	pm := pl.mark()
	tb.narrate("the goblin's run is over")
	pl.expectStay(pm, tb.log(qaCreatureHero), "sync")
	q.expectSwitch(m, "", nil, "VTT-342 the goblin to nobody")
	// VTT-370
	if r := q.send(qaCreatureMove(qaCreatureTokGob, 6, 0)); r.GetOk() {
		t.Errorf("VTT-370: a player who runs no party member moved the goblin")
	}

	// VTT-342
	pm = pl.mark()
	tb.narrate("nobody hears this")
	pl.expectStay(pm, tb.log(qaCreatureHero), "sync")
	time.Sleep(300 * time.Millisecond)
	fs := q.snapshot(m)
	if evs := qaCreatureEvents(fs[qaCreatureLastVC(fs)+1:]); len(evs) != 0 {
		t.Errorf("VTT-342: a seat switched to nobody was sent %d entries%s", len(evs), qaCreatureDump(fs))
	}
}

// VTT-313 VTT-318 VTT-321 VTT-326
func TestQACreatureAnActorAddedAgainAsItIsNow(t *testing.T) {
	tb, _, _, pID := qaCreatureRunning(t)
	tb.dm.mustOK(qaCreatureRemove(qaCreatureGoblin))
	paused := tb.log(qaCreatureGoblin)
	held := qaCreatureHeld(paused)
	a := tb.dm.mustOK(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: qaCreatureGoblin, Name: "Goblin Reborn", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, Attributes: map[string]int32{"str": 9}},
	}}}).GetSequence()
	tb.dm.mustOK(qaCreaturePlace("tok-goblin-again", tb.scene, qaCreatureGoblin, 5, 0))
	tb.dm.mustOK(qaCreatureGrant(qaCreatureGoblin, pID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	log := tb.log(qaCreatureGoblin)
	qaCreatureExpectResume(t, paused, log, qaCreatureResumeEntries(false, "", "", held), a)
	var last *vttv1.Actor
	for _, e := range log[len(paused):] {
		if aa := e.Env.GetActorAdded(); aa != nil && aa.GetActor().GetActorId() == qaCreatureGoblin {
			last = aa.GetActor()
		}
	}
	if last.GetName() != "Goblin Reborn" || last.GetAttributes()["str"] != 9 {
		t.Errorf("VTT-313: after its return the log last introduced the goblin as %v, want the actor added again", last)
	}
}
