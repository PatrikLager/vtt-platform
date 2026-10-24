package campaign_test

import (
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

const (
	qaLogParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaLogNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	qaLogNoKind   = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
	qaLogPublic   = vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC
	qaLogSecret   = vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET
	qaLogUnset    = vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED
)

type qaLogGen struct{ sess string }

func (g *qaLogGen) e(id string, env *vttv1.Envelope) *vttv1.Envelope {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	env.EventId = "qalog-" + id
	env.OccurredAt = timestamppb.New(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC).Add(time.Duration(h.Sum32()%100000) * time.Second))
	env.SessionId = g.sess
	env.ActorRole = "dm"
	env.ParticipantId = "qalog-dm"
	return env
}

func (g *qaLogGen) start(id, sid, name string) *vttv1.Envelope {
	g.sess = sid
	return g.e(id, &vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: name}}})
}

func (g *qaLogGen) end(id string) *vttv1.Envelope {
	env := g.e(id, qaLogEnd())
	g.sess = ""
	return env
}

func qaLogEnd() *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}}
}

func qaLogScene(id string, w, h, wallX int32) *vttv1.Envelope {
	sc := &vttv1.SceneCreated{SceneId: id, Name: "qalog " + id, GridWidth: w, GridHeight: h}
	if wallX >= 0 {
		sc.Tiles = map[string]*vttv1.TileRef{}
		for x := int32(0); x < w; x++ {
			for y := int32(0); y < h; y++ {
				kind := "floor"
				if x == wallX {
					kind = "wall"
				}
				sc.Tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind, Material: "stone"}
			}
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: sc}}
}

func qaLogActor(id string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: id, Name: "qalog " + id, Kind: kind,
		Resources: map[string]*vttv1.Resource{"pool": {Current: 10, Max: 10}},
	}}}}
}

func qaLogGrant(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: participant, Kind: kind,
	}}}
}

func qaLogRevoke(actor, participant string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: actor, ParticipantId: participant,
	}}}
}

func qaLogPlace(token, scene, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: token, SceneId: scene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func qaLogMove(token, scene string, fx, fy, tx, ty int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: token, SceneId: scene,
		From: &vttv1.GridPosition{X: fx, Y: fy}, To: &vttv1.GridPosition{X: tx, Y: ty},
		Reason: "qalog reason",
	}}}
}

func qaLogNarrate(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text, As: "narrator"}}}
}

func qaLogNote(key, text string, vis vttv1.NoteVisibility) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: "qalog " + key, Text: text, Visibility: vis,
	}}}
}

func qaLogNoteDelete(key string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{Key: key}}}
}

func qaLogPool(actor string, delta, newValue int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: actor, Resource: "pool", Delta: delta, NewValue: newValue, Reason: "qalog reason",
	}}}
}

func qaLogMark(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cond, Source: "qalog source",
	}}}
}

func qaLogRemoveToken(token string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: token}}}
}

func qaLogRemoveActor(actor string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}}
}

func qaLogHide(token string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenHidden{TokenHidden: &vttv1.TokenHidden{TokenId: token}}}
}

func qaLogPayload(env *vttv1.Envelope) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", env.GetPayload()), "*vttv1.Envelope_")
}

func qaLogLine(e campaign.CharacterEntry) string {
	env := e.Env
	detail := ""
	switch p := env.GetPayload().(type) {
	case *vttv1.Envelope_ActorAdded:
		detail = p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		detail = p.ActorControlGranted.GetActorId() + "/" + p.ActorControlGranted.GetParticipantId()
	case *vttv1.Envelope_TokenPlaced:
		detail = p.TokenPlaced.GetTokenId()
	case *vttv1.Envelope_TokenHidden:
		detail = p.TokenHidden.GetTokenId()
	case *vttv1.Envelope_TokenMoved:
		detail = p.TokenMoved.GetTokenId()
	case *vttv1.Envelope_NoteUpserted:
		detail = p.NoteUpserted.GetKey() + "=" + p.NoteUpserted.GetText()
	case *vttv1.Envelope_NarrationAdded:
		detail = p.NarrationAdded.GetText()
	case *vttv1.Envelope_SceneCreated:
		detail = p.SceneCreated.GetSceneId()
	case *vttv1.Envelope_SessionStarted:
		detail = p.SessionStarted.GetName()
	case *vttv1.Envelope_ResourceChanged:
		detail = fmt.Sprintf("%s %+d=%d", p.ResourceChanged.GetActorId(), p.ResourceChanged.GetDelta(), p.ResourceChanged.GetNewValue())
	}
	return fmt.Sprintf("#%d w%d %s(%s) eid=%q sid=%q", env.GetSequence(), e.WorldSeq, qaLogPayload(env), detail, env.GetEventId(), env.GetSessionId())
}

func qaLogDescribe(log []campaign.CharacterEntry) string {
	var b strings.Builder
	for _, e := range log {
		b.WriteString("    " + qaLogLine(e) + "\n")
	}
	return b.String()
}

func qaLogEnvs(log []campaign.CharacterEntry) []*vttv1.Envelope {
	out := make([]*vttv1.Envelope, len(log))
	for i, e := range log {
		out[i] = e.Env
	}
	return out
}

func qaLogClone(log []campaign.CharacterEntry) []campaign.CharacterEntry {
	out := make([]campaign.CharacterEntry, len(log))
	for i, e := range log {
		out[i] = campaign.CharacterEntry{WorldSeq: e.WorldSeq, Env: proto.Clone(e.Env).(*vttv1.Envelope)}
	}
	return out
}

func qaLogFold(t *testing.T, log []campaign.CharacterEntry) *engine.State {
	t.Helper()
	st, err := campaign.FoldPrefix(qaLogEnvs(log))
	if err != nil {
		t.Fatalf("log does not fold: %v\n%s", err, qaLogDescribe(log))
	}
	return st
}

func qaLogIsParty(st *engine.State, id string) bool {
	a, ok := st.Actors[id]
	return ok && a.GetKind() == qaLogParty
}

func qaLogOpenSession(st *engine.State) (engine.Session, bool) {
	for _, s := range st.Sessions {
		if s.EndSeq == 0 {
			return s, true
		}
	}
	return engine.Session{}, false
}

func qaLogIndex(log []campaign.CharacterEntry, match func(*vttv1.Envelope) bool) int {
	for i, e := range log {
		if match(e.Env) {
			return i
		}
	}
	return -1
}

func qaLogAdds(id string) func(*vttv1.Envelope) bool {
	return func(env *vttv1.Envelope) bool { return env.GetActorAdded().GetActor().GetActorId() == id }
}

func qaLogNotes(key string) func(*vttv1.Envelope) bool {
	return func(env *vttv1.Envelope) bool {
		return env.GetNoteUpserted() != nil && env.GetNoteUpserted().GetKey() == key
	}
}

func qaLogEvent(eventID string) func(*vttv1.Envelope) bool {
	return func(env *vttv1.Envelope) bool { return env.GetEventId() == eventID }
}

func qaLogReadWorld(t *testing.T, c *campaign.Campaign, after int64) []*vttv1.Envelope {
	t.Helper()
	ch, unsub, head, err := c.Subscribe(after, 64)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	var out []*vttv1.Envelope
	for int64(len(out)) < head-after {
		select {
		case env, ok := <-ch:
			if !ok {
				t.Fatalf("world subscription closed after %d of %d events", len(out), head-after)
			}
			if env.GetSequence() != after+int64(len(out))+1 {
				t.Fatalf("world sequence %d where %d was expected", env.GetSequence(), after+int64(len(out))+1)
			}
			out = append(out, proto.Clone(env).(*vttv1.Envelope))
		case <-time.After(5 * time.Second):
			t.Fatalf("world subscription stalled after %d of %d events", len(out), head-after)
		}
	}
	return out
}

type qaLogTable struct {
	t     *testing.T
	dir   string
	c     *campaign.Campaign
	world []*vttv1.Envelope
	watch []string
	snap  map[string][]campaign.CharacterEntry
}

func qaLogAttach(t *testing.T, dir string, watch ...string) *qaLogTable {
	t.Helper()
	tb := &qaLogTable{t: t, dir: dir, watch: watch, snap: map[string][]campaign.CharacterEntry{}}
	t.Cleanup(func() {
		if tb.c != nil {
			_ = tb.c.Close()
		}
	})
	tb.open(false)
	return tb
}

func qaLogNew(t *testing.T, watch ...string) *qaLogTable {
	t.Helper()
	return qaLogAttach(t, t.TempDir(), watch...)
}

func (tb *qaLogTable) open(expectSame bool) {
	tb.t.Helper()
	c, err := campaign.Open(tb.dir)
	if err != nil {
		tb.t.Fatalf("Open: %v", err)
	}
	tb.c = c
	tb.world = qaLogReadWorld(tb.t, c, 0)
	if expectSame {
		for _, id := range tb.watch {
			if got := tb.log(id); len(got) != len(tb.snap[id]) {
				tb.t.Errorf("VTT-326: after reopen %q's log reads back %d entries, it held %d\nnow:\n%sbefore:\n%s",
					id, len(got), len(tb.snap[id]), qaLogDescribe(got), qaLogDescribe(tb.snap[id]))
			}
		}
	}
	tb.check(nil)
}

func (tb *qaLogTable) close() {
	tb.t.Helper()
	if err := tb.c.Close(); err != nil {
		tb.t.Fatalf("Close: %v", err)
	}
	tb.c = nil
}

func (tb *qaLogTable) log(id string) []campaign.CharacterEntry {
	tb.t.Helper()
	log, err := tb.c.CharacterLog(id)
	if err != nil {
		tb.t.Fatalf("CharacterLog(%q): %v", id, err)
	}
	return log
}

func (tb *qaLogTable) seqOf(eventID string) int64 {
	tb.t.Helper()
	for _, w := range tb.world {
		if w.GetEventId() == eventID {
			return w.GetSequence()
		}
	}
	tb.t.Fatalf("no world event %q", eventID)
	return 0
}

func (tb *qaLogTable) try(envs ...*vttv1.Envelope) error {
	tb.t.Helper()
	var err error
	if len(envs) == 1 {
		_, err = tb.c.Append(envs[0])
	} else {
		_, err = tb.c.AppendBatch(envs)
	}
	if err != nil {
		if got := qaLogReadWorld(tb.t, tb.c, 0); len(got) != len(tb.world) {
			tb.t.Errorf("VTT-315: a failed append left the world log at %d events, it held %d", len(got), len(tb.world))
		}
		for _, id := range tb.watch {
			got := tb.log(id)
			same := len(got) == len(tb.snap[id])
			for i := 0; same && i < len(got); i++ {
				same = got[i].WorldSeq == tb.snap[id][i].WorldSeq && proto.Equal(got[i].Env, tb.snap[id][i].Env)
			}
			if !same {
				tb.t.Errorf("VTT-325: a failed append changed %q's log\nnow:\n%sbefore:\n%s",
					id, qaLogDescribe(got), qaLogDescribe(tb.snap[id]))
			}
		}
		return err
	}
	added := qaLogReadWorld(tb.t, tb.c, int64(len(tb.world)))
	if len(added) != len(envs) {
		tb.t.Fatalf("append of %d events added %d to the world", len(envs), len(added))
	}
	tb.world = append(tb.world, added...)
	tb.check(added)
	return nil
}

func (tb *qaLogTable) add(envs ...*vttv1.Envelope) {
	tb.t.Helper()
	if err := tb.try(envs...); err != nil {
		tb.t.Fatalf("append %s: %v", envs[0].GetEventId(), err)
	}
}

func (tb *qaLogTable) check(appended []*vttv1.Envelope) {
	t := tb.t
	t.Helper()
	states := map[int64]*engine.State{}
	stateAt := func(seq int64) *engine.State {
		if st, ok := states[seq]; ok {
			return st
		}
		st, err := campaign.FoldPrefix(tb.world[:seq])
		if err != nil {
			t.Fatalf("world prefix %d does not fold: %v", seq, err)
		}
		states[seq] = st
		return st
	}
	byID := map[string]*vttv1.Envelope{}
	for _, w := range tb.world {
		byID[w.GetEventId()] = w
	}
	for _, id := range tb.watch {
		log := tb.log(id)
		old := tb.snap[id]
		if len(log) < len(old) {
			t.Errorf("VTT-326: %q's log shrank from %d to %d entries", id, len(old), len(log))
		}
		for i := 0; i < len(old) && i < len(log); i++ {
			if log[i].WorldSeq != old[i].WorldSeq || !proto.Equal(log[i].Env, old[i].Env) {
				t.Errorf("VTT-326: entry %d of %q changed\nwas: %s\nnow: %s", i+1, id, qaLogLine(old[i]), qaLogLine(log[i]))
			}
		}
		for i, e := range log {
			if e.Env.GetSequence() != int64(i+1) {
				t.Errorf("VTT-321: entry %d of %q carries sequence %d", i+1, id, e.Env.GetSequence())
			}
		}
		if appended != nil && len(log) > len(old) {
			party := false
			for _, a := range appended {
				party = party || qaLogIsParty(stateAt(a.GetSequence()), id)
			}
			if !party {
				t.Errorf("VTT-316: %q's log grew at world events %d..%d, after none of which it is a party member\n%s",
					id, appended[0].GetSequence(), appended[len(appended)-1].GetSequence(), qaLogDescribe(log[len(old):]))
			}
		}
		for i := len(old); i < len(log); i++ {
			e := log[i]
			if e.WorldSeq > int64(len(tb.world)) {
				t.Errorf("%q entry %s names a world sequence the world does not hold", id, qaLogLine(e))
				continue
			}
			if e.WorldSeq != 0 && !qaLogIsParty(stateAt(e.WorldSeq), id) {
				t.Errorf("VTT-316: %q's entry %s was written with world event %d, after which it is not a party member", id, qaLogLine(e), e.WorldSeq)
			}
			tb.provenance(id, e, byID)
		}
		if _, err := campaign.FoldPrefix(qaLogEnvs(log)); err != nil {
			t.Errorf("%q's log does not fold: %v\n%s", id, err, qaLogDescribe(log))
		}
		tb.snap[id] = qaLogClone(log)
	}
}

func (tb *qaLogTable) provenance(id string, e campaign.CharacterEntry, byID map[string]*vttv1.Envelope) {
	t := tb.t
	t.Helper()
	env := e.Env
	if env.GetEventId() == "" {
		if env.GetOccurredAt() != nil {
			t.Errorf("ruling: synthesized entry of %q carries occurred_at: %s", id, qaLogLine(e))
		}
		if env.GetSessionStarted() == nil && env.GetSessionId() != "" {
			t.Errorf("ruling: synthesized entry of %q carries a session id: %s", id, qaLogLine(e))
		}
		if env.GetSessionStarted() != nil && env.GetSessionId() == "" {
			t.Errorf("ruling: an opening SessionStarted of %q carries no session id: %s", id, qaLogLine(e))
		}
		return
	}
	w, ok := byID[env.GetEventId()]
	if !ok {
		t.Errorf("%q's entry %s carries an event id no world event has", id, qaLogLine(e))
		return
	}
	if qaLogPayload(w) != qaLogPayload(env) {
		t.Errorf("%q's entry %s carries the event id of a %s", id, qaLogLine(e), qaLogPayload(w))
	}
	if env.GetSessionId() != w.GetSessionId() || !proto.Equal(env.GetOccurredAt(), w.GetOccurredAt()) {
		t.Errorf("ruling: forwarded entry %s of %q does not keep the world event's session id %q and occurred_at", qaLogLine(e), id, w.GetSessionId())
	}
	if e.WorldSeq != w.GetSequence() {
		t.Errorf("design §5: forwarded entry %s of %q names world sequence %d, its event is %d", qaLogLine(e), id, e.WorldSeq, w.GetSequence())
	}
	if env.GetParticipantId() != "" || env.GetActorRole() != "" {
		t.Errorf("SPEC-016: forwarded entry %s of %q carries its issuer (%q, %q)", qaLogLine(e), id, env.GetParticipantId(), env.GetActorRole())
	}
}

func (tb *qaLogTable) openSessionID() string {
	tb.t.Helper()
	s, ok := qaLogOpenSession(tb.c.State())
	if !ok {
		tb.t.Fatalf("no session is open in the world")
	}
	return s.ID
}

func (tb *qaLogTable) sessionOrdinals() map[string]string {
	out := map[string]string{"": ""}
	for _, w := range tb.world {
		if w.GetSessionStarted() != nil {
			out[w.GetSessionId()] = fmt.Sprintf("session-%d", len(out))
		}
	}
	return out
}

func qaLogNormal(env *vttv1.Envelope, ordinals map[string]string) *vttv1.Envelope {
	c := proto.Clone(env).(*vttv1.Envelope)
	if n, ok := ordinals[c.GetSessionId()]; ok {
		c.SessionId = n
	}
	return c
}

func qaLogSameLogs(t *testing.T, got, want *qaLogTable, ids ...string) {
	t.Helper()
	gn, wn := got.sessionOrdinals(), want.sessionOrdinals()
	for _, id := range ids {
		g, w := got.log(id), want.log(id)
		same := len(g) == len(w)
		for i := 0; same && i < len(g); i++ {
			same = proto.Equal(qaLogNormal(g[i].Env, gn), qaLogNormal(w[i].Env, wn))
		}
		if !same {
			t.Errorf("%q's log differs from the control's\ngot:\n%swant:\n%s", id, qaLogDescribe(g), qaLogDescribe(w))
			continue
		}
		for i := range g {
			if g[i].WorldSeq != w[i].WorldSeq {
				t.Errorf("%q entry %d names world sequence %d, the control's names %d", id, i+1, g[i].WorldSeq, w[i].WorldSeq)
			}
		}
	}
}

func qaLogRefuseCharacterWrites(t *testing.T, dir string) func() {
	t.Helper()
	db, err := sql.Open("sqlite", campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`CREATE TRIGGER qalog_refuse BEFORE INSERT ON character_events BEGIN SELECT RAISE(ABORT, 'qalog injected failure'); END`); err != nil {
		_ = db.Close()
		t.Fatalf("create trigger: %v", err)
	}
	return func() {
		if _, err := db.Exec(`DROP TRIGGER qalog_refuse`); err != nil {
			t.Errorf("drop trigger: %v", err)
		}
		_ = db.Close()
	}
}

func qaLogCraft(t *testing.T, dir string, world *vttv1.Envelope, entries ...store.Entry) {
	t.Helper()
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	if _, err := s.AppendBatch([]*vttv1.Envelope{world}, entries); err != nil {
		t.Fatalf("crafted store append: %v", err)
	}
}

func qaLogWalled(g *qaLogGen, xKind vttv1.ActorKind) []*vttv1.Envelope {
	return []*vttv1.Envelope{
		g.start("s1", "qalog-s1", "one"),
		g.e("scene", qaLogScene("a", 7, 3, 3)),
		g.e("p", qaLogActor("p", qaLogParty)),
		g.e("pg", qaLogGrant("p", "alice", qaLogParty)),
		g.e("pt", qaLogPlace("tp", "a", "p", 0, 1)),
		g.e("x", qaLogActor("x", xKind)),
		g.e("xt", qaLogPlace("tx", "a", "x", 5, 1)),
		g.e("q", qaLogActor("q", qaLogParty)),
		g.e("qg", qaLogGrant("q", "bob", qaLogParty)),
		g.e("qt", qaLogPlace("tq", "a", "q", 6, 1)),
	}
}

// TestQALogANonPartyActorNeverHasALog pins VTT-316.
// VTT-316
func TestQALogANonPartyActorNeverHasALog(t *testing.T) {
	tb := qaLogNew(t, "p", "x", "u", "y", "nobody")
	g := &qaLogGen{}
	for _, env := range []*vttv1.Envelope{
		g.start("s1", "qalog-s1", "one"),
		g.e("scene", qaLogScene("a", 6, 6, -1)),
		g.e("p", qaLogActor("p", qaLogParty)),
		g.e("pg", qaLogGrant("p", "alice", qaLogParty)),
		g.e("pt", qaLogPlace("tp", "a", "p", 0, 0)),
		g.e("x", qaLogActor("x", qaLogNonParty)),
		g.e("xt", qaLogPlace("tx", "a", "x", 1, 1)),
		g.e("u", qaLogActor("u", qaLogNoKind)),
		g.e("ut", qaLogPlace("tu", "a", "u", 2, 2)),
		g.e("ug", qaLogGrant("u", "carol", qaLogNoKind)),
		g.e("y", qaLogActor("y", qaLogNonParty)),
		g.e("yg", qaLogGrant("y", "bob", qaLogNonParty)),
		g.e("yt", qaLogPlace("ty", "a", "y", 3, 3)),
		g.e("xm", qaLogMove("tx", "a", 1, 1, 4, 1)),
		g.e("ym", qaLogMove("ty", "a", 3, 3, 4, 4)),
		g.e("xr", qaLogPool("x", -2, 8)),
		g.e("ur", qaLogPool("u", -1, 9)),
		g.e("yc", qaLogMark("y", "marked")),
		g.e("n1", qaLogNarrate("qalog narration one")),
		g.e("note", qaLogNote("k1", "public text", qaLogPublic)),
		g.e("yrev", qaLogRevoke("y", "bob")),
		g.end("end"),
	} {
		tb.add(env)
	}
	tb.close()
	tb.open(true)
	tb.add(g.e("n2", qaLogNarrate("qalog narration two")))
	for _, id := range []string{"x", "u", "y", "nobody"} {
		if got := tb.log(id); len(got) != 0 {
			t.Errorf("VTT-316: %q is never a party member and has a log:\n%s", id, qaLogDescribe(got))
		}
	}
	held := qaLogFold(t, tb.log("p"))
	for _, id := range []string{"x", "u", "y"} {
		if _, ok := held.Actors[id]; !ok {
			t.Errorf("fixture: p never saw %q, so the fan-out never had it in view\n%s", id, qaLogDescribe(tb.log("p")))
		}
	}
}

// TestQALogAnEventReachesALogOnlyWhileItsActorIsAPartyMember pins VTT-316.
// VTT-316
func TestQALogAnEventReachesALogOnlyWhileItsActorIsAPartyMember(t *testing.T) {
	tb := qaLogNew(t, "p", "q", "r", "s")
	g := &qaLogGen{}
	tb.add(g.start("s1", "qalog-s1", "one"))
	tb.add(g.e("scene", qaLogScene("a", 6, 6, -1)))
	tb.add(g.e("p", qaLogActor("p", qaLogNonParty)))
	tb.add(g.e("pt", qaLogPlace("tp", "a", "p", 0, 0)))
	tb.add(g.e("n0", qaLogNarrate("qalog before the grant")))
	tb.add(g.e("pr", qaLogPool("p", -1, 9)))
	if got := tb.log("p"); len(got) != 0 {
		t.Fatalf("VTT-316: p is not a party member yet and has a log:\n%s", qaLogDescribe(got))
	}

	tb.add(g.e("pg", qaLogGrant("p", "alice", qaLogParty)))
	if len(tb.log("p")) == 0 {
		t.Fatalf("VTT-317: the grant made p a party member and its log did not open")
	}
	tb.add(g.e("n1", qaLogNarrate("qalog while a party member")))
	if i := qaLogIndex(tb.log("p"), qaLogEvent("qalog-n1")); i < 0 {
		t.Fatalf("fixture: a narration while p is a party member did not reach p\n%s", qaLogDescribe(tb.log("p")))
	}

	tb.add(g.e("pd", qaLogGrant("p", "alice", qaLogNonParty)))
	paused := len(tb.log("p"))
	for _, env := range []*vttv1.Envelope{
		g.e("n2", qaLogNarrate("qalog while demoted")),
		g.e("pm", qaLogMove("tp", "a", 0, 0, 1, 0)),
		g.e("note", qaLogNote("k1", "public text", qaLogPublic)),
		g.e("pr2", qaLogPool("p", -1, 8)),
		g.end("end1"),
		g.start("s2", "qalog-s2", "two"),
	} {
		tb.add(env)
	}
	tb.close()
	tb.open(true)
	tb.add(g.e("n3", qaLogNarrate("qalog after reopen while demoted")))
	if got := len(tb.log("p")); got != paused {
		t.Errorf("VTT-316: p's log grew from %d to %d while p was not a party member\n%s", paused, got, qaLogDescribe(tb.log("p")))
	}

	tb.add(g.e("q", qaLogActor("q", qaLogParty)))
	tb.add(g.e("qg", qaLogGrant("q", "bob", qaLogParty)))
	tb.add(g.e("qt", qaLogPlace("tq", "a", "q", 2, 2)))
	tb.add(g.e("qtr", qaLogRemoveToken("tq")), g.e("qr", qaLogRemoveActor("q")))
	removed := len(tb.log("q"))
	if removed == 0 {
		t.Fatalf("fixture: q never had a log")
	}
	tb.add(g.e("n4", qaLogNarrate("qalog after q left")))
	if got := len(tb.log("q")); got != removed {
		t.Errorf("VTT-316: q's log grew from %d to %d after q left the world", removed, got)
	}

	tb.add(g.e("r", qaLogActor("r", qaLogParty)), g.e("rd", qaLogGrant("r", "dave", qaLogNonParty)))
	if len(tb.log("r")) == 0 {
		t.Errorf("VTT-316/317: r was a party member after the first event of its batch and has no log")
	}
	for _, e := range tb.log("r") {
		if e.WorldSeq == tb.seqOf("qalog-rd") {
			t.Errorf("VTT-316: r's entry %s was written with the grant after which r is not a party member", qaLogLine(e))
		}
	}

	tb.add(g.e("s", qaLogActor("s", qaLogNonParty)), g.e("sg", qaLogGrant("s", "erin", qaLogParty)))
	if len(tb.log("s")) == 0 {
		t.Errorf("VTT-317: the grant made s a party member and its log did not open")
	}
	for _, e := range tb.log("s") {
		if e.WorldSeq == tb.seqOf("qalog-s") {
			t.Errorf("VTT-316: s's entry %s was written with its ActorAdded, after which s is not a party member", qaLogLine(e))
		}
	}
}

func qaLogCheckOpening(t *testing.T, log []campaign.CharacterEntry, sid, name string) {
	t.Helper()
	if len(log) == 0 {
		t.Fatalf("VTT-317: the log did not open")
	}
	first := log[0].Env
	if first.GetSessionStarted() == nil {
		t.Fatalf("VTT-317: the log does not open with the open session's SessionStarted\n%s", qaLogDescribe(log))
	}
	if first.GetSessionId() != sid {
		t.Errorf("VTT-317: the opening's SessionStarted carries session %q, the open session is %q", first.GetSessionId(), sid)
	}
	if first.GetSessionStarted().GetName() != name {
		t.Errorf("VTT-317: the opening's SessionStarted is named %q, the open session is %q", first.GetSessionStarted().GetName(), name)
	}
	if first.GetEventId() != "" || first.GetOccurredAt() != nil {
		t.Errorf("ruling: the opening's SessionStarted is synthesized and carries an event id or a time: %s", qaLogLine(log[0]))
	}
}

func qaLogNothingFrom(t *testing.T, log []campaign.CharacterEntry, earlier []*vttv1.Envelope) {
	t.Helper()
	for _, w := range earlier {
		if i := qaLogIndex(log, qaLogEvent(w.GetEventId())); i >= 0 {
			t.Errorf("VTT-317: the log holds entry %s forwarded from an event appended before it opened", qaLogLine(log[i]))
		}
	}
	for _, e := range log {
		if strings.HasPrefix(e.Env.GetNarrationAdded().GetText(), "qalog-old") {
			t.Errorf("ruling: the log holds narration appended before it opened: %s", qaLogLine(e))
		}
	}
}

// TestQALogALateLogOpensWithThePresent pins VTT-317.
// VTT-317
func TestQALogALateLogOpensWithThePresent(t *testing.T) {
	tb := qaLogNew(t, "p", "q", "x")
	g := &qaLogGen{}
	for _, env := range []*vttv1.Envelope{
		g.start("s1", "qalog-s1", "first"),
		g.e("old1", qaLogNarrate("qalog-old one")),
		g.end("end1"),
		g.start("s2", "qalog-s2", "second"),
		g.e("scene", qaLogScene("a", 6, 6, -1)),
		g.e("q", qaLogActor("q", qaLogParty)),
		g.e("qg", qaLogGrant("q", "bob", qaLogParty)),
		g.e("qt", qaLogPlace("tq", "a", "q", 0, 0)),
		g.e("qr", qaLogPool("q", -4, 6)),
		g.e("x", qaLogActor("x", qaLogNonParty)),
		g.e("xt", qaLogPlace("tx", "a", "x", 5, 5)),
		g.e("pub1", qaLogNote("pub", "old text", qaLogPublic)),
		g.e("pub2", qaLogNote("pub", "new text", qaLogPublic)),
		g.e("sec", qaLogNote("sec", "secret text", qaLogSecret)),
		g.e("uns", qaLogNote("uns", "unset text", qaLogUnset)),
		g.e("gone1", qaLogNote("gone", "gone text", qaLogPublic)),
		g.e("gone2", qaLogNoteDelete("gone")),
		g.e("hid1", qaLogNote("hid", "was public", qaLogPublic)),
		g.e("hid2", qaLogNote("hid", "now secret", qaLogSecret)),
		g.e("old2", qaLogNarrate("qalog-old two")),
	} {
		tb.add(env)
	}
	earlier := append([]*vttv1.Envelope(nil), tb.world...)
	tb.add(g.e("p", qaLogActor("p", qaLogParty)))
	log := tb.log("p")
	sid := tb.openSessionID()
	qaLogCheckOpening(t, log, sid, "second")
	qaLogNothingFrom(t, log, earlier)
	self := qaLogIndex(log, qaLogAdds("p"))
	party := qaLogIndex(log, qaLogAdds("q"))
	note := qaLogIndex(log, qaLogNotes("pub"))
	if party < 0 || note < 0 {
		t.Fatalf("VTT-317: the opening lacks the party (q at %d) or the public note (pub at %d)\n%s", party, note, qaLogDescribe(log))
	}
	if self >= 0 && (party > self || note > self) {
		t.Errorf("VTT-317: the opening is not before the event's frames: p's own ActorAdded is entry %d, q's %d, the note %d\n%s", self+1, party+1, note+1, qaLogDescribe(log))
	}
	for _, key := range []string{"sec", "uns", "gone", "hid"} {
		if i := qaLogIndex(log, qaLogNotes(key)); i >= 0 {
			t.Errorf("VTT-317: the opening holds note %q, which is not public: %s", key, qaLogLine(log[i]))
		}
	}
	held := qaLogFold(t, log)
	if s, ok := qaLogOpenSession(held); !ok || s.ID != sid || len(held.Sessions) != 1 {
		t.Errorf("VTT-317: the opened log holds sessions %+v, the present is %s open", held.Sessions, sid)
	}
	if n, ok := held.Notes["pub"]; !ok || n.Text != "new text" || len(held.Notes) != 1 {
		t.Errorf("VTT-317: the opened log holds notes %+v, the present is pub=new text", held.Notes)
	}
	if a, ok := held.Actors["q"]; !ok || a.GetResources()["pool"].GetCurrent() != 6 {
		t.Errorf("VTT-317: the opened log holds q as %v, the world holds its pool at 6", a)
	}
	if _, ok := held.Actors["x"]; ok {
		t.Errorf("VTT-317: the opening introduces x, which is neither in the party nor in p's sight")
	}

	tb.add(g.e("new", qaLogNarrate("qalog-new")))
	if got := tb.log("p"); got[len(got)-1].Env.GetEventId() != "qalog-new" {
		t.Errorf("ruling: a narration after the opening is not the log's last entry\n%s", qaLogDescribe(got))
	}
	tb.add(g.end("end2"))
	held = qaLogFold(t, tb.log("p"))
	if _, ok := qaLogOpenSession(held); ok {
		t.Errorf("VTT-317: the session open at the opening is still open in p's log after it ended\n%s", qaLogDescribe(tb.log("p")))
	}
}

// TestQALogALateLogOpensWithoutASessionWhenNoneIsOpen pins VTT-317.
// VTT-317
func TestQALogALateLogOpensWithoutASessionWhenNoneIsOpen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start bool
	}{{"a session started and ended before", true}, {"no session ever", false}} {
		t.Run(tc.name, func(t *testing.T) {
			tb := qaLogNew(t, "p")
			g := &qaLogGen{}
			if tc.start {
				tb.add(g.start("s1", "qalog-s1", "first"))
				tb.add(g.e("old1", qaLogNarrate("qalog-old one")))
				tb.add(g.end("end1"))
			}
			tb.add(g.e("pub", qaLogNote("pub", "public", qaLogPublic)))
			tb.add(g.e("p", qaLogActor("p", qaLogParty)))
			log := tb.log("p")
			if len(log) == 0 {
				t.Fatalf("VTT-317: the log did not open")
			}
			for _, e := range log {
				if e.Env.GetSessionStarted() != nil || e.Env.GetSessionEnded() != nil {
					t.Errorf("VTT-317: no session is open, and the opening holds %s", qaLogLine(e))
				}
			}
			if qaLogIndex(log, qaLogNotes("pub")) < 0 {
				t.Errorf("VTT-317: the opening lacks the public note\n%s", qaLogDescribe(log))
			}
			tb.add(g.start("s2", "qalog-s2", "second"))
			tb.add(g.end("end2"))
			if i := qaLogIndex(tb.log("p"), qaLogEvent("qalog-s2")); i < 0 {
				t.Errorf("a session started after the opening did not reach the log\n%s", qaLogDescribe(tb.log("p")))
			}
		})
	}
}

// TestQALogALogOpensWhenAGrantMakesAPartyMember pins VTT-317 and VTT-316.
// VTT-317 VTT-316
func TestQALogALogOpensWhenAGrantMakesAPartyMember(t *testing.T) {
	tb := qaLogNew(t, "p", "q", "x")
	g := &qaLogGen{}
	for _, env := range []*vttv1.Envelope{
		g.start("s1", "qalog-s1", "one"),
		g.e("scene", qaLogScene("a", 6, 6, -1)),
		g.e("q", qaLogActor("q", qaLogParty)),
		g.e("qg", qaLogGrant("q", "bob", qaLogParty)),
		g.e("qt", qaLogPlace("tq", "a", "q", 0, 0)),
		g.e("pub", qaLogNote("pub", "public", qaLogPublic)),
		g.e("sec", qaLogNote("sec", "secret", qaLogSecret)),
		g.e("x", qaLogActor("x", qaLogNonParty)),
		g.e("xt", qaLogPlace("tx", "a", "x", 5, 5)),
		g.e("p", qaLogActor("p", qaLogNonParty)),
		g.e("pt", qaLogPlace("tp", "a", "p", 3, 3)),
		g.e("old1", qaLogNarrate("qalog-old before the grant")),
		g.e("pr", qaLogPool("p", -2, 8)),
		g.e("pm", qaLogMove("tp", "a", 3, 3, 3, 4)),
		g.e("pg0", qaLogGrant("p", "frank", qaLogNonParty)),
	} {
		tb.add(env)
	}
	if got := tb.log("p"); len(got) != 0 {
		t.Fatalf("VTT-316: p is not a party member and has a log:\n%s", qaLogDescribe(got))
	}
	earlier := append([]*vttv1.Envelope(nil), tb.world...)
	tb.add(g.e("pg", qaLogGrant("p", "frank", qaLogParty)))
	log := tb.log("p")
	qaLogCheckOpening(t, log, tb.openSessionID(), "one")
	qaLogNothingFrom(t, log, earlier)
	grant := tb.seqOf("qalog-pg")
	for _, e := range log {
		if e.WorldSeq != 0 && e.WorldSeq != grant {
			t.Errorf("VTT-317: the log opens at the grant (world %d) and holds %s", grant, qaLogLine(e))
		}
	}
	party := qaLogIndex(log, qaLogAdds("q"))
	note := qaLogIndex(log, qaLogNotes("pub"))
	self := qaLogIndex(log, qaLogAdds("p"))
	if party < 0 || note < 0 {
		t.Fatalf("VTT-317: the opening lacks the party (q at %d) or the public note (pub at %d)\n%s", party, note, qaLogDescribe(log))
	}
	if self >= 0 && (party > self || note > self) {
		t.Errorf("VTT-317: p was not a party member before the grant, yet the opening's party includes it\n%s", qaLogDescribe(log))
	}
	if qaLogIndex(log, qaLogNotes("sec")) >= 0 {
		t.Errorf("VTT-317: the opening holds the secret note\n%s", qaLogDescribe(log))
	}
	held := qaLogFold(t, log)
	if a, ok := held.Actors["p"]; !ok || a.GetKind() != qaLogParty {
		t.Errorf("the log opened at the grant does not hold p as a party member: %v\n%s", a, qaLogDescribe(log))
	}
}

// TestQALogALogOpensForAPartyMemberThatHadNone pins VTT-317 and VTT-324.
// VTT-317 VTT-324
func TestQALogALogOpensForAPartyMemberThatHadNone(t *testing.T) {
	dir := t.TempDir()
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	s, err := store.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	g := &qaLogGen{}
	world := []*vttv1.Envelope{
		g.start("s1", "qalog-s1", "one"),
		g.e("old1", qaLogNarrate("qalog-old one")),
		g.e("p", qaLogActor("p", qaLogParty)),
		g.e("pg", qaLogGrant("p", "alice", qaLogParty)),
		g.e("pub", qaLogNote("pub", "public", qaLogPublic)),
		g.e("sec", qaLogNote("sec", "secret", qaLogSecret)),
		g.e("q", qaLogActor("q", qaLogParty)),
		g.e("qr", qaLogPool("q", -3, 7)),
		g.e("old2", qaLogNarrate("qalog-old two")),
	}
	if _, err := s.AppendBatch(world, nil); err != nil {
		t.Fatalf("store append: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("store close: %v", err)
	}
	tb := qaLogAttach(t, dir, "p", "q")
	for _, id := range []string{"p", "q"} {
		if got := tb.log(id); len(got) != 0 {
			t.Fatalf("%q's log opened before any append:\n%s", id, qaLogDescribe(got))
		}
	}
	earlier := append([]*vttv1.Envelope(nil), tb.world...)
	tb.add(g.e("next", qaLogNarrate("qalog-next")))
	for _, id := range []string{"p", "q"} {
		log := tb.log(id)
		qaLogCheckOpening(t, log, "qalog-s1", "one")
		qaLogNothingFrom(t, log, earlier)
		held := qaLogFold(t, log)
		for _, a := range []string{"p", "q"} {
			if !qaLogIsParty(held, a) {
				t.Errorf("VTT-317: %q's opening lacks party member %q\n%s", id, a, qaLogDescribe(log))
			}
		}
		if held.Actors["q"].GetResources()["pool"].GetCurrent() != 7 {
			t.Errorf("VTT-317: %q's opening holds q's pool at %d, the world at 7", id, held.Actors["q"].GetResources()["pool"].GetCurrent())
		}
		if _, ok := held.Notes["pub"]; !ok || len(held.Notes) != 1 {
			t.Errorf("VTT-317: %q's opening holds notes %+v, the public notes are {pub}", id, held.Notes)
		}
		if last := log[len(log)-1].Env; last.GetEventId() != "qalog-next" {
			t.Errorf("the event that opened %q's log is not its last entry\n%s", id, qaLogDescribe(log))
		}
	}
	opened := len(tb.log("p"))
	tb.close()
	tb.open(true)
	tb.add(g.e("again", qaLogNarrate("qalog-again")))
	log := tb.log("p")
	for _, e := range log[opened:] {
		if e.Env.GetEventId() == "" {
			t.Errorf("VTT-324: after reopen p's log received synthesized entry %s for a narration", qaLogLine(e))
		}
	}
}

func qaLogTwoParty(g *qaLogGen) []*vttv1.Envelope {
	return []*vttv1.Envelope{
		g.start("s1", "qalog-s1", "one"),
		g.e("scene", qaLogScene("a", 6, 6, -1)),
		g.e("p", qaLogActor("p", qaLogParty)),
		g.e("pg", qaLogGrant("p", "alice", qaLogParty)),
		g.e("pt", qaLogPlace("tp", "a", "p", 0, 0)),
		g.e("q", qaLogActor("q", qaLogParty)),
		g.e("qg", qaLogGrant("q", "bob", qaLogParty)),
		g.e("qt", qaLogPlace("tq", "a", "q", 5, 5)),
		g.e("x", qaLogActor("x", qaLogNonParty)),
		g.e("xt", qaLogPlace("tx", "a", "x", 2, 2)),
	}
}

// TestQALogFramesThatWouldNotFoldAreNotWritten pins VTT-320.
// VTT-320
func TestQALogFramesThatWouldNotFoldAreNotWritten(t *testing.T) {
	tb := qaLogNew(t, "p", "q", "x")
	g := &qaLogGen{}
	for _, env := range qaLogTwoParty(g) {
		tb.add(env)
	}
	if _, ok := qaLogFold(t, tb.log("p")).Tokens["tx"]; !ok {
		t.Fatalf("fixture: p does not see tx\n%s", qaLogDescribe(tb.log("p")))
	}
	tb.close()
	qaLogCraft(t, tb.dir, g.e("crafted", qaLogNarrate("qalog crafted")),
		store.Entry{ActorID: "p", Cause: -1, Env: qaLogHide("tx")},
		store.Entry{ActorID: "p", Cause: -1, Env: qaLogEnd()})
	tb.open(false)
	if got := tb.c.DiscardedForTest(); got != 0 {
		t.Fatalf("a fresh Campaign reports %d discards", got)
	}
	pBefore, qBefore := tb.log("p"), tb.log("q")

	tb.add(g.end("end"))
	if s, ok := qaLogOpenSession(tb.c.State()); ok {
		t.Errorf("VTT-320: the world event was not applied, session %q is still open", s.ID)
	}
	if tb.world[len(tb.world)-1].GetEventId() != "qalog-end" {
		t.Errorf("VTT-320: the world log does not end with the SessionEnded")
	}
	if got := tb.log("p"); len(got) != len(pBefore) {
		t.Errorf("VTT-320: p's frames for the SessionEnded do not fold onto its log, and it received\n%s", qaLogDescribe(got[len(pBefore):]))
	}
	q := tb.log("q")
	if i := qaLogIndex(q[len(qBefore):], qaLogEvent("qalog-end")); i < 0 {
		t.Errorf("VTT-320: q's log did not receive the SessionEnded\n%s", qaLogDescribe(q))
	}
	if got := tb.c.DiscardedForTest(); got != 1 {
		t.Errorf("VTT-320: %d frame sets discarded, one did not fold", got)
	}

	tb.add(g.e("after", qaLogNarrate("qalog after")))
	p := tb.log("p")
	if len(p) == len(pBefore) || p[len(p)-1].Env.GetEventId() != "qalog-after" {
		t.Errorf("VTT-320: the next event that reaches p is not written to its log\n%s", qaLogDescribe(p))
	}
	if qaLogIndex(p[len(pBefore):], func(env *vttv1.Envelope) bool { return env.GetTokenPlaced().GetTokenId() == "tx" }) < 0 {
		t.Fatalf("fixture: the discarded frames held no TokenPlaced that would have folded\n%s", qaLogDescribe(p))
	}
	tb.add(g.e("mv", qaLogMove("tx", "a", 2, 2, 3, 2)))
	tb.add(g.start("s2", "qalog-s2", "two"))
	tb.add(g.end("end2"))
	if got := tb.c.DiscardedForTest(); got != 1 {
		t.Errorf("VTT-320: %d frame sets discarded after the log recovered, one did not fold", got)
	}
	tb.close()
	tb.open(true)
}

// TestQALogFramesThatWouldNotFoldAreNotWrittenWithinABatch pins VTT-320.
// VTT-320 VTT-371 VTT-372
func TestQALogFramesThatWouldNotFoldAreNotWrittenWithinABatch(t *testing.T) {
	tb := qaLogNew(t, "p", "q")
	g := &qaLogGen{}
	for _, env := range qaLogTwoParty(g) {
		tb.add(env)
	}
	tb.close()
	qaLogCraft(t, tb.dir, g.e("crafted", qaLogNarrate("qalog crafted")),
		store.Entry{ActorID: "p", Cause: -1, Env: qaLogEnd()})
	tb.open(false)
	pBefore := len(tb.log("p"))
	tb.add(g.end("end"), g.e("nb", qaLogNarrate("qalog in the same batch")))
	p := tb.log("p")[pBefore:]
	if i := qaLogIndex(p, qaLogEvent("qalog-end")); i >= 0 {
		t.Errorf("VTT-320: p received the SessionEnded that does not fold onto its log: %s", qaLogLine(p[i]))
	}
	if qaLogIndex(p, qaLogEvent("qalog-nb")) < 0 {
		t.Errorf("VTT-320: p did not receive the next event of the same batch\n%s", qaLogDescribe(p))
	}
	q := tb.log("q")
	if qaLogIndex(q, qaLogEvent("qalog-end")) < 0 || qaLogIndex(q, qaLogEvent("qalog-nb")) < 0 {
		t.Errorf("VTT-320: q did not receive both events of the batch\n%s", qaLogDescribe(q))
	}
	if got := tb.c.DiscardedForTest(); got != 1 {
		t.Errorf("VTT-320: %d frame sets discarded, one did not fold", got)
	}
}

// TestQALogAFailedFanOutDoesNotAdvanceACharactersMemory pins VTT-325.
// VTT-325
func TestQALogAFailedFanOutDoesNotAdvanceACharactersMemory(t *testing.T) {
	type fail func(t *testing.T, tb *qaLogTable, g *qaLogGen) error
	refused := func(t *testing.T, tb *qaLogTable, envs ...*vttv1.Envelope) error {
		t.Helper()
		restore := qaLogRefuseCharacterWrites(t, tb.dir)
		defer restore()
		err := tb.try(envs...)
		if err != nil && !strings.Contains(err.Error(), "qalog injected") {
			t.Errorf("the append failed, but not at the injected store write: %v", err)
		}
		return err
	}
	for _, tc := range []struct {
		name  string
		xKind vttv1.ActorKind
		fail  fail
	}{
		{"store write fails, unseen non-party actor", qaLogNonParty, func(t *testing.T, tb *qaLogTable, g *qaLogGen) error {
			t.Helper()
			return refused(t, tb, g.e("m1", qaLogMove("tx", "a", 5, 1, 2, 1)))
		}},
		{"store write fails, unseen party member", qaLogParty, func(t *testing.T, tb *qaLogTable, g *qaLogGen) error {
			t.Helper()
			return refused(t, tb, g.e("m1", qaLogMove("tx", "a", 5, 1, 2, 1)))
		}},
		{"store write fails for a batch", qaLogParty, func(t *testing.T, tb *qaLogTable, g *qaLogGen) error {
			t.Helper()
			return refused(t, tb, g.e("nb", qaLogNarrate("qalog in the failed batch")), g.e("m1", qaLogMove("tx", "a", 5, 1, 2, 1)))
		}},
		{"batch refused after the move", qaLogParty, func(t *testing.T, tb *qaLogTable, g *qaLogGen) error {
			t.Helper()
			return tb.try(g.e("m1", qaLogMove("tx", "a", 5, 1, 2, 1)), g.e("bad", qaLogMove("ghost", "a", 0, 0, 1, 1)))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			watch := []string{"p", "q", "x"}
			control := qaLogNew(t, watch...)
			cg := &qaLogGen{}
			for _, env := range qaLogWalled(cg, tc.xKind) {
				control.add(env)
			}
			control.add(cg.e("r", qaLogPool("x", -3, 7)))
			control.add(cg.e("m2", qaLogMove("tx", "a", 5, 1, 2, 1)))

			failed := qaLogNew(t, watch...)
			fg := &qaLogGen{}
			for _, env := range qaLogWalled(fg, tc.xKind) {
				failed.add(env)
			}
			if _, ok := qaLogFold(t, failed.log("p")).Tokens["tx"]; ok {
				t.Fatalf("fixture: p sees tx before the move\n%s", qaLogDescribe(failed.log("p")))
			}
			if err := tc.fail(t, failed, fg); err == nil {
				t.Fatalf("the failing append succeeded")
			}
			if tok := failed.c.State().Tokens["tx"]; tok.X != 5 {
				t.Errorf("the failed move reached the world: tx at %d,%d", tok.X, tok.Y)
			}
			pAt := len(failed.log("p"))
			failed.add(fg.e("r", qaLogPool("x", -3, 7)))
			if got := failed.log("p"); len(got) != pAt {
				t.Errorf("VTT-325: an event on x, which p never saw, reached p after the failed move\n%s", qaLogDescribe(got[pAt:]))
			}
			if got := failed.c.DiscardedForTest(); got != 0 {
				t.Errorf("VTT-325: %d frame sets were discarded after the failed append; p's memory moved past its log", got)
			}
			failed.add(fg.e("m2", qaLogMove("tx", "a", 5, 1, 2, 1)))
			if _, ok := qaLogFold(t, control.log("p")).Tokens["tx"]; !ok {
				t.Fatalf("fixture: the move does not bring tx into p's sight\n%s", qaLogDescribe(control.log("p")))
			}
			if qaLogIndex(control.log("p"), qaLogEvent("qalog-r")) >= 0 {
				t.Fatalf("fixture: the event on unseen x reached p in the control")
			}
			qaLogSameLogs(t, failed, control, watch...)
			if got := failed.c.DiscardedForTest(); got != 0 {
				t.Errorf("VTT-325: %d frame sets discarded in the failed campaign", got)
			}
		})
	}
}

// TestQALogAFailedOpeningLeavesTheLogToOpenAgain pins VTT-325 and VTT-317.
// VTT-325 VTT-317
func TestQALogAFailedOpeningLeavesTheLogToOpenAgain(t *testing.T) {
	watch := []string{"p", "q", "late"}
	control := qaLogNew(t, watch...)
	cg := &qaLogGen{}
	for _, env := range qaLogWalled(cg, qaLogNonParty) {
		control.add(env)
	}
	control.add(cg.e("pub", qaLogNote("pub", "public", qaLogPublic)))
	control.add(cg.e("between", qaLogNarrate("qalog between")))
	control.add(cg.e("late2", qaLogActor("late", qaLogParty)))
	control.add(cg.e("after", qaLogNarrate("qalog after")))

	failed := qaLogNew(t, watch...)
	fg := &qaLogGen{}
	for _, env := range qaLogWalled(fg, qaLogNonParty) {
		failed.add(env)
	}
	failed.add(fg.e("pub", qaLogNote("pub", "public", qaLogPublic)))
	restore := qaLogRefuseCharacterWrites(t, failed.dir)
	err := failed.try(fg.e("late1", qaLogActor("late", qaLogParty)))
	restore()
	if err == nil {
		t.Fatalf("the opening append succeeded although every character write was refused")
	}
	failed.add(fg.e("between", qaLogNarrate("qalog between")))
	failed.add(fg.e("late2", qaLogActor("late", qaLogParty)))
	failed.add(fg.e("after", qaLogNarrate("qalog after")))
	qaLogCheckOpening(t, control.log("late"), control.openSessionID(), "one")
	qaLogCheckOpening(t, failed.log("late"), failed.openSessionID(), "one")
	qaLogSameLogs(t, failed, control, watch...)
}

func qaLogIntroducesNothingHeld(t *testing.T, id string, log []campaign.CharacterEntry, from int) {
	t.Helper()
	for k := from; k < len(log); k++ {
		env := log[k].Env
		if env.GetEventId() != "" {
			continue
		}
		held := qaLogFold(t, log[:k])
		bad := false
		switch {
		case env.GetSessionStarted() != nil:
			_, bad = qaLogOpenSession(held)
		case env.GetSceneCreated() != nil:
			_, bad = held.Scenes[env.GetSceneCreated().GetSceneId()]
		case env.GetActorAdded() != nil:
			_, bad = held.Actors[env.GetActorAdded().GetActor().GetActorId()]
		case env.GetTokenPlaced() != nil:
			_, bad = held.Tokens[env.GetTokenPlaced().GetTokenId()]
		case env.GetNoteUpserted() != nil:
			n, ok := held.Notes[env.GetNoteUpserted().GetKey()]
			bad = ok && n.Text == env.GetNoteUpserted().GetText()
		case env.GetActorControlGranted() != nil:
			g := env.GetActorControlGranted()
			for _, c := range held.Actors[g.GetActorId()].GetControllerIds() {
				bad = bad || (c == g.GetParticipantId() && held.Actors[g.GetActorId()].GetKind() == g.GetKind())
			}
		}
		if bad {
			t.Errorf("VTT-324: %q's entry %s introduces what the log already holds", id, qaLogLine(log[k]))
		}
	}
}

func qaLogReopenPartOne(g *qaLogGen) []*vttv1.Envelope {
	return []*vttv1.Envelope{
		g.start("s1", "qalog-s1", "one"),
		g.e("scene", qaLogScene("a", 7, 3, 3)),
		g.e("p", qaLogActor("p", qaLogParty)),
		g.e("pg", qaLogGrant("p", "alice", qaLogParty)),
		g.e("pt", qaLogPlace("tp", "a", "p", 0, 1)),
		g.e("x", qaLogActor("x", qaLogNonParty)),
		g.e("xt", qaLogPlace("tx", "a", "x", 1, 1)),
		g.e("q", qaLogActor("q", qaLogParty)),
		g.e("qg", qaLogGrant("q", "bob", qaLogParty)),
		g.e("qt", qaLogPlace("tq", "a", "q", 6, 1)),
		g.e("y", qaLogActor("y", qaLogNonParty)),
		g.e("yt", qaLogPlace("ty", "a", "y", 5, 1)),
		g.e("pub", qaLogNote("pub", "public", qaLogPublic)),
		g.e("n1", qaLogNarrate("qalog one")),
		g.e("rx1", qaLogPool("x", -1, 9)),
		g.e("rq1", qaLogPool("q", -1, 9)),
	}
}

func qaLogReopenPartTwo(g *qaLogGen) []*vttv1.Envelope {
	return []*vttv1.Envelope{
		g.e("cx2", qaLogMark("x", "winded")),
		g.e("rq2", qaLogPool("q", -1, 8)),
		g.e("ry2", qaLogPool("y", -4, 6)),
		g.e("mx", qaLogMove("tx", "a", 1, 1, 2, 1)),
		g.e("n2", qaLogNarrate("qalog two")),
		g.e("pub2", qaLogNote("pub", "public again", qaLogPublic)),
		g.e("cx", qaLogMark("x", "marked")),
		g.e("mq", qaLogMove("tq", "a", 6, 1, 2, 0)),
		g.e("my", qaLogMove("ty", "a", 5, 1, 4, 1)),
		g.end("end"),
	}
}

// TestQALogACharacterLogContinuesAfterReopen pins VTT-324, VTT-326 and VTT-321.
// VTT-324 VTT-326 VTT-321
func TestQALogACharacterLogContinuesAfterReopen(t *testing.T) {
	watch := []string{"p", "q", "x", "y"}
	control := qaLogNew(t, watch...)
	cg := &qaLogGen{}
	for _, env := range append(qaLogReopenPartOne(cg), qaLogReopenPartTwo(cg)...) {
		control.add(env)
	}

	tb := qaLogNew(t, watch...)
	g := &qaLogGen{}
	for _, env := range qaLogReopenPartOne(g) {
		tb.add(env)
	}
	at := map[string]int{}
	for _, id := range watch {
		at[id] = len(tb.log(id))
	}
	tb.close()
	tb.open(true)
	tb.close()
	tb.open(true)
	for _, env := range qaLogReopenPartTwo(g) {
		tb.add(env)
	}
	if got := tb.c.DiscardedForTest(); got != 0 {
		t.Errorf("VTT-324: %d frame sets discarded after reopen; the memory rebuilt on open is not the log's", got)
	}
	for _, id := range []string{"p", "q"} {
		qaLogIntroducesNothingHeld(t, id, tb.log(id), at[id])
		if len(tb.log(id)) == at[id] {
			t.Errorf("fixture: %q's log received nothing after reopen", id)
		}
	}
	if qaLogIndex(control.log("p"), qaLogEvent("qalog-cx2")) < 0 {
		t.Fatalf("fixture: the change on seen x did not reach p in the control\n%s", qaLogDescribe(control.log("p")))
	}
	if qaLogIndex(control.log("p"), qaLogEvent("qalog-rq2")) >= 0 {
		t.Fatalf("fixture: the change on unseen q reached p in the control")
	}
	qaLogSameLogs(t, tb, control, watch...)
}

// TestQALogALogReopenedRightAfterItsOpeningContinues pins VTT-324.
// VTT-324
func TestQALogALogReopenedRightAfterItsOpeningContinues(t *testing.T) {
	part1 := func(g *qaLogGen) []*vttv1.Envelope {
		return []*vttv1.Envelope{
			g.start("s1", "qalog-s1", "one"),
			g.e("pub", qaLogNote("pub", "public", qaLogPublic)),
			g.e("q", qaLogActor("q", qaLogParty)),
			g.e("p", qaLogActor("p", qaLogParty)),
		}
	}
	part2 := func(g *qaLogGen) []*vttv1.Envelope {
		return []*vttv1.Envelope{
			g.e("n1", qaLogNarrate("qalog one")),
			g.e("scene", qaLogScene("a", 6, 6, -1)),
			g.e("pt", qaLogPlace("tp", "a", "p", 0, 0)),
			g.e("qt", qaLogPlace("tq", "a", "q", 1, 1)),
			g.e("pub2", qaLogNote("pub", "public again", qaLogPublic)),
			g.end("end"),
		}
	}
	watch := []string{"p", "q"}
	control := qaLogNew(t, watch...)
	cg := &qaLogGen{}
	for _, env := range append(part1(cg), part2(cg)...) {
		control.add(env)
	}
	tb := qaLogNew(t, watch...)
	g := &qaLogGen{}
	for _, env := range part1(g) {
		tb.add(env)
	}
	at := len(tb.log("p"))
	tb.close()
	tb.open(true)
	for _, env := range part2(g) {
		tb.add(env)
	}
	qaLogIntroducesNothingHeld(t, "p", tb.log("p"), at)
	qaLogSameLogs(t, tb, control, watch...)
}

// TestQALogOpenRefusesACharacterLogThatDoesNotFold pins VTT-327.
// VTT-327
func TestQALogOpenRefusesACharacterLogThatDoesNotFold(t *testing.T) {
	setup := func(t *testing.T) (string, *qaLogGen) {
		t.Helper()
		tb := qaLogNew(t, "pia", "quinn")
		g := &qaLogGen{}
		tb.add(g.start("s1", "qalog-s1", "one"))
		tb.add(g.e("pia", qaLogActor("pia", qaLogParty)))
		tb.add(g.e("quinn", qaLogActor("quinn", qaLogParty)))
		tb.add(g.e("n1", qaLogNarrate("qalog one")))
		tb.close()
		return tb.dir, g
	}
	refuses := func(t *testing.T, dir, actor, other string) {
		t.Helper()
		for attempt := 1; attempt <= 2; attempt++ {
			c, err := campaign.Open(dir)
			if err == nil {
				_ = c.Close()
				t.Fatalf("VTT-327: Open accepted a campaign whose log for %q does not fold (attempt %d)", actor, attempt)
			}
			if errors.Is(err, campaign.ErrHeld) {
				t.Errorf("VTT-327: attempt %d was refused as held, not for the log: %v", attempt, err)
			}
			if !strings.Contains(err.Error(), actor) {
				t.Errorf("VTT-327: the refusal does not name %q: %v", actor, err)
			}
			if other != "" && strings.Contains(err.Error(), other) {
				t.Errorf("VTT-327: the refusal names %q, whose log folds: %v", other, err)
			}
		}
	}
	ghost := func() *vttv1.Envelope { return qaLogMove("qalog-ghost-token", "a", 0, 0, 1, 1) }
	t.Run("an unknown token moved", func(t *testing.T) {
		dir, g := setup(t)
		qaLogCraft(t, dir, g.e("crafted", qaLogNarrate("qalog crafted")), store.Entry{ActorID: "quinn", Cause: -1, Env: ghost()})
		refuses(t, dir, "quinn", "pia")
	})
	t.Run("a bad entry that is not the last", func(t *testing.T) {
		dir, g := setup(t)
		qaLogCraft(t, dir, g.e("crafted", qaLogNarrate("qalog crafted")),
			store.Entry{ActorID: "pia", Cause: -1, Env: ghost()},
			store.Entry{ActorID: "pia", Cause: -1, Env: qaLogNarrate("qalog fine")})
		refuses(t, dir, "pia", "quinn")
	})
	t.Run("a log for an actor the world never held", func(t *testing.T) {
		dir, g := setup(t)
		qaLogCraft(t, dir, g.e("crafted", qaLogNarrate("qalog crafted")), store.Entry{ActorID: "stray", Cause: -1, Env: qaLogEnd()})
		refuses(t, dir, "stray", "")
	})
	t.Run("a second session opened", func(t *testing.T) {
		dir, g := setup(t)
		second := &vttv1.Envelope{SessionId: "qalog-s9", Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "nine"}}}
		qaLogCraft(t, dir, g.e("crafted", qaLogNarrate("qalog crafted")), store.Entry{ActorID: "quinn", Cause: 0, Env: second})
		refuses(t, dir, "quinn", "pia")
	})
	t.Run("control: a crafted entry that folds opens", func(t *testing.T) {
		dir, g := setup(t)
		qaLogCraft(t, dir, g.e("crafted", qaLogNarrate("qalog crafted")), store.Entry{ActorID: "quinn", Cause: -1, Env: qaLogNarrate("qalog fine")})
		c, err := campaign.Open(dir)
		if err != nil {
			t.Fatalf("Open refused a campaign whose logs fold: %v", err)
		}
		defer func() { _ = c.Close() }()
		log, err := c.CharacterLog("quinn")
		if err != nil || len(log) == 0 || log[len(log)-1].Env.GetNarrationAdded().GetText() != "qalog fine" {
			t.Errorf("the crafted entry is not quinn's last: %v\n%s", err, qaLogDescribe(log))
		}
	})
}

// TestQALogACharacterEntryIsNeverChangedOrRemoved pins VTT-326.
// VTT-326
func TestQALogACharacterEntryIsNeverChangedOrRemoved(t *testing.T) {
	watch := []string{"p", "q", "r", "x"}
	tb := qaLogNew(t, watch...)
	g := &qaLogGen{}
	for _, env := range qaLogTwoParty(g) {
		tb.add(env)
	}
	tb.add(g.e("r", qaLogActor("r", qaLogParty)))
	tb.add(g.e("rt", qaLogPlace("tr", "a", "r", 4, 0)))
	tb.add(g.e("n1", qaLogNarrate("qalog one")))
	tb.add(g.e("rx", qaLogPool("x", -1, 9)))
	kept := map[string][]campaign.CharacterEntry{}
	for _, id := range watch {
		kept[id] = qaLogClone(tb.log(id))
	}

	restore := qaLogRefuseCharacterWrites(t, tb.dir)
	if err := tb.try(g.e("lost", qaLogNarrate("qalog lost"))); err == nil {
		t.Errorf("the append succeeded although every character write was refused")
	}
	restore()
	tb.add(g.e("pd", qaLogGrant("p", "alice", qaLogNonParty)))
	tb.add(g.e("n2", qaLogNarrate("qalog two")))
	tb.add(g.e("qtr", qaLogRemoveToken("tq")), g.e("qr", qaLogRemoveActor("q")))
	qGone := qaLogClone(tb.log("q"))
	tb.add(g.e("n3", qaLogNarrate("qalog three")))
	tb.close()
	qaLogCraft(t, tb.dir, g.e("crafted", qaLogNarrate("qalog crafted")), store.Entry{ActorID: "r", Cause: -1, Env: qaLogEnd()})
	tb.open(false)
	tb.add(g.end("end"))
	if got := tb.c.DiscardedForTest(); got != 1 {
		t.Errorf("fixture: %d discards, the crafted log was to refuse one", got)
	}
	tb.add(g.e("n4", qaLogNarrate("qalog four")))
	tb.close()
	tb.open(true)
	for _, id := range watch {
		now := tb.log(id)
		if len(now) < len(kept[id]) {
			t.Errorf("VTT-326: %q's log shrank from %d to %d", id, len(kept[id]), len(now))
			continue
		}
		for i, e := range kept[id] {
			if now[i].WorldSeq != e.WorldSeq || !proto.Equal(now[i].Env, e.Env) {
				t.Errorf("VTT-326: %q's entry %d changed\nwas: %s\nnow: %s", id, i+1, qaLogLine(e), qaLogLine(now[i]))
			}
		}
	}
	if now := tb.log("q"); len(now) != len(qGone) || len(qGone) == 0 {
		t.Errorf("VTT-326: q's log held %d entries when q left and reads %d now", len(qGone), len(now))
	}
}

// TestQALogMutatingAReadEntryChangesNoStoredEntry pins VTT-326.
// VTT-326
func TestQALogMutatingAReadEntryChangesNoStoredEntry(t *testing.T) {
	tb := qaLogNew(t, "p")
	g := &qaLogGen{}
	tb.add(g.start("s1", "qalog-s1", "one"))
	tb.add(g.e("p", qaLogActor("p", qaLogParty)))
	tb.add(g.e("n1", qaLogNarrate("qalog one")))
	read := tb.log("p")
	last := read[len(read)-1].Env
	if last.GetNarrationAdded() == nil {
		t.Fatalf("fixture: p's last entry is not the narration\n%s", qaLogDescribe(read))
	}
	last.GetNarrationAdded().Text = "qalog rewritten by a reader"
	last.Sequence = 99
	read[0].Env.Payload = &vttv1.Envelope_TokenHidden{TokenHidden: &vttv1.TokenHidden{TokenId: "qalog-nothing"}}
	again := tb.log("p")
	if got := again[len(again)-1].Env; got.GetNarrationAdded().GetText() != "qalog one" || got.GetSequence() != int64(len(again)) {
		t.Errorf("VTT-326: a reader's write to a read entry reached the log: %s", qaLogLine(again[len(again)-1]))
	}
	if again[0].Env.GetSessionStarted() == nil {
		t.Errorf("VTT-326: a reader's write to a read entry reached the log: %s", qaLogLine(again[0]))
	}
	tb.add(g.e("n2", qaLogNarrate("qalog two")))
	tb.close()
	tb.open(true)
}
