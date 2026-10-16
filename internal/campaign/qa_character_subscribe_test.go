package campaign_test

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

var qaCSubIDSeq atomic.Int64

const (
	qaCSubSession = "qa-csub-session"
	qaCSubScene   = "qa-csub-hall"
	qaCSubA       = "qa-csub-pc-a"
	qaCSubB       = "qa-csub-pc-b"
	qaCSubX       = "qa-csub-npc-x"
	qaCSubTokA    = "qa-csub-tok-a"
	qaCSubTokB    = "qa-csub-tok-b"
	qaCSubTokX    = "qa-csub-tok-x"
	qaCSubWait    = 5 * time.Second
	qaCSubHush    = 250 * time.Millisecond
)

func qaCSubStamp(env *vttv1.Envelope) *vttv1.Envelope {
	env.EventId = fmt.Sprintf("qa-csub-%d", qaCSubIDSeq.Add(1))
	env.OccurredAt = timestamppb.Now()
	env.SessionId = qaCSubSession
	return env
}

func qaCSubNarr(text string) *vttv1.Envelope {
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{
		NarrationAdded: &vttv1.NarrationAdded{Text: text},
	}})
}

func qaCSubAnchored(text string, from, to int64) *vttv1.Envelope {
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{
		NarrationAdded: &vttv1.NarrationAdded{Text: text, AnchorFromSeq: from, AnchorToSeq: to},
	}})
}

func qaCSubMove(tok string, fx, tx int32) *vttv1.Envelope {
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tok, SceneId: qaCSubScene,
		From: &vttv1.GridPosition{X: fx, Y: 0}, To: &vttv1.GridPosition{X: tx, Y: 0},
	}}})
}

func qaCSubNote(key, text string) *vttv1.Envelope {
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: key, Text: text, Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
	}}})
}

func qaCSubActor(id string, kind vttv1.ActorKind) *vttv1.Envelope {
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: kind},
	}}})
}

func qaCSubPlace(tok, actor string, x int32) *vttv1.Envelope {
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: qaCSubScene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: 0},
	}}})
}

func qaCSubGrant(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actor, ParticipantId: participant, Kind: kind},
	}})
}

// Keep the wall at x=3: TestQACSubCarriesItsOwnLogAndNothingElse needs two
// logs that differ.
func qaCSubHall() *vttv1.Envelope {
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 7; x++ {
		kind := "floor"
		if x == 3 {
			kind = "wall"
		}
		tiles[fmt.Sprintf("%d,0", x)] = &vttv1.TileRef{Kind: kind}
	}
	return qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: qaCSubScene, Name: "hall", GridWidth: 7, GridHeight: 1, Tiles: tiles,
	}}})
}

type qaCSubFix struct {
	dir    string
	c      *campaign.Campaign
	closed bool
}

func (f *qaCSubFix) close(t *testing.T) {
	t.Helper()
	if f.closed {
		return
	}
	f.closed = true
	if err := f.c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func (f *qaCSubFix) reopen(t *testing.T) {
	t.Helper()
	f.close(t)
	c, err := campaign.Open(f.dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	f.c, f.closed = c, false
}

func (f *qaCSubFix) append(t *testing.T, env *vttv1.Envelope) {
	t.Helper()
	if _, err := f.c.Append(env); err != nil {
		t.Fatalf("Append %T: %v", env.Payload, err)
	}
}

func (f *qaCSubFix) batch(t *testing.T, envs ...*vttv1.Envelope) {
	t.Helper()
	if _, err := f.c.AppendBatch(envs); err != nil {
		t.Fatalf("AppendBatch: %v", err)
	}
}

// Keep pc-a at x=1 and pc-b at x=6 on return: every test's moves start there.
func qaCSubTable(t *testing.T) *qaCSubFix {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	f := &qaCSubFix{dir: dir, c: c}
	t.Cleanup(func() {
		if !f.closed {
			f.closed = true
			_ = f.c.Close()
		}
	})
	f.append(t, qaCSubStamp(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{
		SessionStarted: &vttv1.SessionStarted{Name: "evening"},
	}}))
	f.append(t, qaCSubHall())
	f.batch(t,
		qaCSubActor(qaCSubA, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaCSubActor(qaCSubB, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaCSubActor(qaCSubX, vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qaCSubPlace(qaCSubTokA, qaCSubA, 0),
		qaCSubPlace(qaCSubTokB, qaCSubB, 6),
		qaCSubPlace(qaCSubTokX, qaCSubX, 2),
	)
	f.append(t, qaCSubNarr("torches gutter"))
	f.append(t, qaCSubMove(qaCSubTokA, 0, 1))
	f.append(t, qaCSubNote("qa-map", "the hall runs east"))
	return f
}

func qaCSubLog(t *testing.T, c *campaign.Campaign, actor string) []*vttv1.Envelope {
	t.Helper()
	entries, err := c.CharacterLog(actor)
	if err != nil {
		t.Fatalf("CharacterLog(%s): %v", actor, err)
	}
	out := make([]*vttv1.Envelope, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Env)
	}
	return out
}

func qaCSubWorldHead(t *testing.T, c *campaign.Campaign) int64 {
	t.Helper()
	ch, unsub, head, err := c.Subscribe(0, 1)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	unsub()
	_ = ch
	return head
}

// Do not compare with qaCSubSession: the campaign stamps its own session id.
func qaCSubOpenSession(t *testing.T, c *campaign.Campaign) string {
	t.Helper()
	ch, unsub, head, err := c.Subscribe(0, 1)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	id := ""
	for _, e := range qaCSubTake(t, "world", ch, int(head)) {
		if e.GetSessionStarted() != nil {
			id = e.GetSessionId()
		}
	}
	if id == "" {
		t.Fatalf("fixture: the world holds no SessionStarted")
	}
	return id
}

type qaCSubSub struct {
	ch    <-chan *vttv1.Envelope
	unsub func()
	head  int64
}

func qaCSubSubscribe(t *testing.T, c *campaign.Campaign, actor string, after int64, buffer int, noProgress time.Duration) qaCSubSub {
	t.Helper()
	ch, unsub, head, err := c.SubscribeCharacter(actor, after, buffer, noProgress)
	if err != nil {
		t.Fatalf("SubscribeCharacter(%s, %d): %v", actor, after, err)
	}
	var once sync.Once
	s := qaCSubSub{ch: ch, unsub: func() { once.Do(unsub) }, head: head}
	t.Cleanup(s.unsub)
	return s
}

func qaCSubDesc(env *vttv1.Envelope) string {
	return fmt.Sprintf("#%d %T id=%q", env.GetSequence(), env.GetPayload(), env.GetEventId())
}

func qaCSubTake(t *testing.T, label string, ch <-chan *vttv1.Envelope, n int) []*vttv1.Envelope {
	t.Helper()
	got := make([]*vttv1.Envelope, 0, n)
	deadline := time.After(qaCSubWait)
	for len(got) < n {
		select {
		case env, ok := <-ch:
			if !ok {
				t.Fatalf("%s: channel closed after %d of %d entries", label, len(got), n)
			}
			got = append(got, env)
		case <-deadline:
			t.Fatalf("%s: %d of %d entries after %v", label, len(got), n, qaCSubWait)
		}
	}
	return got
}

func qaCSubQuiet(t *testing.T, label string, ch <-chan *vttv1.Envelope) {
	t.Helper()
	select {
	case env, ok := <-ch:
		if !ok {
			t.Fatalf("%s: channel closed, wanted it open and silent", label)
		}
		t.Fatalf("%s: unexpected delivery %s", label, qaCSubDesc(env))
	case <-time.After(qaCSubHush):
	}
}

func qaCSubDrainToClose(t *testing.T, label string, ch <-chan *vttv1.Envelope, d time.Duration) int {
	t.Helper()
	n := 0
	deadline := time.After(d)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return n
			}
			n++
		case <-deadline:
			t.Fatalf("%s: still open after %v (%d entries read)", label, d, n)
		}
	}
}

func qaCSubSame(t *testing.T, label string, got, want []*vttv1.Envelope) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d entries, want %d", label, len(got), len(want))
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Fatalf("%s: entry %d is %s, want %s", label, i, qaCSubDesc(got[i]), qaCSubDesc(want[i]))
		}
	}
}

func qaCSubContiguous(t *testing.T, label string, envs []*vttv1.Envelope, first int64) {
	t.Helper()
	for i, e := range envs {
		if e.GetSequence() != first+int64(i) {
			t.Fatalf("%s: entry %d has sequence %d, want %d", label, i, e.GetSequence(), first+int64(i))
		}
	}
}

func qaCSubRaw(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func qaCSubExec(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("raw exec %q: %v", stmt, err)
	}
}

// VTT-329, VTT-331.
func TestQACSubHistoryAfterACursorThenLiveEachOnceInOrder(t *testing.T) {
	f := qaCSubTable(t)
	log := qaCSubLog(t, f.c, qaCSubA)
	if len(log) < 5 {
		t.Fatalf("fixture: pc-a's log holds %d entries, want at least 5", len(log))
	}
	qaCSubContiguous(t, "pc-a's log", log, 1)
	const cursor = 2
	sub := qaCSubSubscribe(t, f.c, qaCSubA, cursor, 2, 0)
	if want := log[len(log)-1].GetSequence(); sub.head != want {
		t.Fatalf("catch-up head %d, want %d (last entry queued)", sub.head, want)
	}
	hist := qaCSubTake(t, "history", sub.ch, len(log)-cursor)
	qaCSubSame(t, "history", hist, log[cursor:])
	qaCSubContiguous(t, "history", hist, cursor+1)
	qaCSubQuiet(t, "after history", sub.ch)

	worldHead := qaCSubWorldHead(t, f.c)
	f.append(t, qaCSubMove(qaCSubTokA, 1, 0))
	move := qaCSubMove(qaCSubTokA, 0, 1)
	f.batch(t,
		qaCSubNarr("a draught from the east"),
		move,
		qaCSubAnchored("the stone shifts underfoot", worldHead+3, worldHead+3),
		qaCSubNote("qa-map", "the hall runs east, then north"),
	)
	after := qaCSubLog(t, f.c, qaCSubA)
	fresh := after[len(log):]
	if len(fresh) < 5 {
		t.Fatalf("fixture: the appends wrote %d entries to pc-a's log, want at least 5", len(fresh))
	}
	live := qaCSubTake(t, "live", sub.ch, len(fresh))
	qaCSubSame(t, "live", live, fresh)
	qaCSubContiguous(t, "live", live, int64(len(log))+1)
	qaCSubQuiet(t, "after live", sub.ch)

	var narr *vttv1.Envelope
	for _, e := range live {
		if na := e.GetNarrationAdded(); na != nil && na.GetText() == "the stone shifts underfoot" {
			narr = e
		}
	}
	if narr == nil {
		t.Fatalf("the anchored narration did not reach pc-a's subscription")
	}
	from, to := narr.GetNarrationAdded().GetAnchorFromSeq(), narr.GetNarrationAdded().GetAnchorToSeq()
	if from == 0 || to >= narr.GetSequence() {
		t.Fatalf("delivered narration anchored %d..%d at entry %d, want a range of earlier entries", from, to, narr.GetSequence())
	}
	found := false
	for _, e := range after {
		if e.GetSequence() >= from && e.GetSequence() <= to && e.GetEventId() == move.GetEventId() {
			found = true
		}
	}
	if !found {
		t.Fatalf("delivered narration's anchor %d..%d does not hold pc-a's entry for the move", from, to)
	}
}

// VTT-331, VTT-329.
func TestQACSubCatchUpHeadIsTheCursorWhenNothingIsQueued(t *testing.T) {
	f := qaCSubTable(t)
	log := qaCSubLog(t, f.c, qaCSubA)
	last := log[len(log)-1].GetSequence()

	zero := qaCSubSubscribe(t, f.c, qaCSubA, 0, 4, 0)
	if zero.head != last {
		t.Fatalf("cursor 0: catch-up head %d, want %d", zero.head, last)
	}
	qaCSubSame(t, "cursor 0", qaCSubTake(t, "cursor 0", zero.ch, len(log)), log)

	atHead := qaCSubSubscribe(t, f.c, qaCSubA, last, 4, 0)
	if atHead.head != last {
		t.Fatalf("cursor at head: catch-up head %d, want %d", atHead.head, last)
	}
	qaCSubQuiet(t, "cursor at head", atHead.ch)

	const ahead = 4
	beyond := qaCSubSubscribe(t, f.c, qaCSubA, last+ahead, 4, 0)
	if beyond.head != last+ahead {
		t.Fatalf("cursor beyond head: catch-up head %d, want %d", beyond.head, last+ahead)
	}
	qaCSubQuiet(t, "cursor beyond head", beyond.ch)

	for i := 0; ; i++ {
		if i == 20 {
			t.Fatalf("fixture: twenty narrations did not take pc-a's log past %d", last+ahead+1)
		}
		f.append(t, qaCSubNarr(fmt.Sprintf("drip %d", i)))
		if n := len(qaCSubLog(t, f.c, qaCSubA)); int64(n) > last+ahead+1 {
			break
		}
	}
	now := qaCSubLog(t, f.c, qaCSubA)
	tail := now[len(log):]
	qaCSubSame(t, "cursor at head, live", qaCSubTake(t, "cursor at head", atHead.ch, len(tail)), tail)
	qaCSubSame(t, "cursor 0, live", qaCSubTake(t, "cursor 0", zero.ch, len(tail)), tail)
	past := now[last+ahead:]
	got := qaCSubTake(t, "cursor beyond head", beyond.ch, len(past))
	qaCSubSame(t, "cursor beyond head, live", got, past)
	qaCSubContiguous(t, "cursor beyond head, live", got, last+ahead+1)
	qaCSubQuiet(t, "cursor beyond head, after", beyond.ch)
}

// VTT-329, VTT-331.
func TestQACSubALogThatDoesNotExistYetIsDeliveredFromItsOpening(t *testing.T) {
	f := qaCSubTable(t)
	const pcC, tokC = "qa-csub-pc-c", "qa-csub-tok-c"
	sub := qaCSubSubscribe(t, f.c, pcC, 0, 2, 0)
	if sub.head != 0 {
		t.Fatalf("catch-up head %d for an actor with no log, want the cursor 0", sub.head)
	}
	qaCSubQuiet(t, "before the actor exists", sub.ch)

	f.batch(t, qaCSubActor(pcC, vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaCSubPlace(tokC, pcC, 5))
	f.append(t, qaCSubNarr("someone waits in the dark"))
	if n := len(qaCSubLog(t, f.c, pcC)); n != 0 {
		t.Fatalf("fixture: a non-party actor's log holds %d entries", n)
	}
	qaCSubQuiet(t, "while not a party member", sub.ch)

	f.append(t, qaCSubGrant(pcC, "qa-csub-player-c", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	log := qaCSubLog(t, f.c, pcC)
	if len(log) < 2 {
		t.Fatalf("fixture: the opening wrote %d entries, want at least 2", len(log))
	}
	got := qaCSubTake(t, "opening", sub.ch, len(log))
	qaCSubSame(t, "opening", got, log)
	qaCSubContiguous(t, "opening", got, 1)
	open := qaCSubOpenSession(t, f.c)
	if got[0].GetSessionStarted() == nil || got[0].GetSessionId() != open {
		t.Fatalf("first delivered entry is %s of session %q, want the opening's SessionStarted of the open session %q", qaCSubDesc(got[0]), got[0].GetSessionId(), open)
	}
	qaCSubQuiet(t, "after the opening", sub.ch)
}

// VTT-329.
func TestQACSubPauseAndResumeAreDeliveredAsTheLogHoldsThem(t *testing.T) {
	f := qaCSubTable(t)
	log := qaCSubLog(t, f.c, qaCSubA)
	sub := qaCSubSubscribe(t, f.c, qaCSubA, int64(len(log)), 4, 0)

	f.append(t, qaCSubGrant(qaCSubA, "qa-csub-player-a", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	f.append(t, qaCSubNarr("said while paused"))
	f.append(t, qaCSubMove(qaCSubTokA, 1, 0))
	if n := len(qaCSubLog(t, f.c, qaCSubA)); n != len(log) {
		t.Fatalf("fixture: a paused log grew from %d to %d entries", len(log), n)
	}
	qaCSubQuiet(t, "paused", sub.ch)

	f.append(t, qaCSubGrant(qaCSubA, "qa-csub-player-a", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	now := qaCSubLog(t, f.c, qaCSubA)
	tail := now[len(log):]
	if len(tail) == 0 {
		t.Fatalf("fixture: the resume wrote no entry")
	}
	got := qaCSubTake(t, "resume", sub.ch, len(tail))
	qaCSubSame(t, "resume", got, tail)
	qaCSubContiguous(t, "resume", got, int64(len(log))+1)
	for _, e := range got {
		if e.GetNarrationAdded().GetText() == "said while paused" {
			t.Fatalf("the narration made during the pause was delivered: %s", qaCSubDesc(e))
		}
	}
	qaCSubQuiet(t, "after resume", sub.ch)
}

// VTT-330.
func TestQACSubCarriesItsOwnLogAndNothingElse(t *testing.T) {
	f := qaCSubTable(t)
	a1 := qaCSubSubscribe(t, f.c, qaCSubA, 0, 4, 0)
	a2 := qaCSubSubscribe(t, f.c, qaCSubA, 0, 4, 0)
	b := qaCSubSubscribe(t, f.c, qaCSubB, 0, 4, 0)
	worldCh, worldUnsub, worldHead, err := f.c.Subscribe(0, 4)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(worldUnsub)
	qaCSubTake(t, "world history", worldCh, int(worldHead))

	appended := []*vttv1.Envelope{qaCSubMove(qaCSubTokA, 1, 0), qaCSubMove(qaCSubTokB, 6, 5)}
	f.append(t, appended[0])
	f.append(t, appended[1])
	batch := []*vttv1.Envelope{
		qaCSubMove(qaCSubTokA, 0, 1), qaCSubNarr("both hear it"), qaCSubMove(qaCSubTokB, 5, 4), qaCSubNote("qa-key", "brass"),
	}
	f.batch(t, batch...)
	appended = append(appended, batch...)

	logA, logB := qaCSubLog(t, f.c, qaCSubA), qaCSubLog(t, f.c, qaCSubB)
	idsIn := func(envs []*vttv1.Envelope) map[string]bool {
		m := map[string]bool{}
		for _, e := range envs {
			if e.GetEventId() != "" {
				m[e.GetEventId()] = true
			}
		}
		return m
	}
	inA, inB := idsIn(logA), idsIn(logB)
	onlyA, onlyB := 0, 0
	for id := range inA {
		if !inB[id] {
			onlyA++
		}
	}
	for id := range inB {
		if !inA[id] {
			onlyB++
		}
	}
	if onlyA == 0 || onlyB == 0 {
		t.Fatalf("fixture: pc-a holds %d events pc-b does not, pc-b %d; want both non-zero", onlyA, onlyB)
	}

	qaCSubSame(t, "pc-a reader 1", qaCSubTake(t, "pc-a reader 1", a1.ch, len(logA)), logA)
	qaCSubSame(t, "pc-a reader 2", qaCSubTake(t, "pc-a reader 2", a2.ch, len(logA)), logA)
	qaCSubSame(t, "pc-b", qaCSubTake(t, "pc-b", b.ch, len(logB)), logB)
	world := qaCSubTake(t, "world live", worldCh, len(appended))
	for i, e := range world {
		if e.GetEventId() != appended[i].GetEventId() || e.GetSequence() != worldHead+int64(i)+1 {
			t.Fatalf("world entry %d is %s, want %q at %d", i, qaCSubDesc(e), appended[i].GetEventId(), worldHead+int64(i)+1)
		}
	}
	qaCSubQuiet(t, "pc-a reader 1 after", a1.ch)
	qaCSubQuiet(t, "pc-b after", b.ch)
	qaCSubQuiet(t, "world after", worldCh)
}

// VTT-332.
func TestQACSubAFailedAppendDeliversNoEntry(t *testing.T) {
	f := qaCSubTable(t)
	logA, logB := qaCSubLog(t, f.c, qaCSubA), qaCSubLog(t, f.c, qaCSubB)
	a := qaCSubSubscribe(t, f.c, qaCSubA, int64(len(logA)), 4, 0)
	b := qaCSubSubscribe(t, f.c, qaCSubB, int64(len(logB)), 4, 0)
	worldHead := qaCSubWorldHead(t, f.c)
	worldCh, worldUnsub, _, err := f.c.Subscribe(worldHead, 4)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(worldUnsub)

	if _, err := f.c.Append(qaCSubMove("qa-csub-no-such-token", 0, 1)); err == nil {
		t.Fatalf("a move of an unknown token was accepted")
	}
	if _, err := f.c.AppendBatch([]*vttv1.Envelope{qaCSubNarr("half a batch"), qaCSubMove("qa-csub-no-such-token", 0, 1)}); err == nil {
		t.Fatalf("a batch ending in a move of an unknown token was accepted")
	}

	db := qaCSubRaw(t, f.dir)
	qaCSubExec(t, db, `CREATE TRIGGER qa_csub_refuse_entry BEFORE INSERT ON character_events BEGIN SELECT RAISE(ABORT, 'refused'); END`)
	if _, err := f.c.Append(qaCSubNarr("never written")); err == nil {
		t.Fatalf("an append whose entry write fails succeeded")
	}
	if _, err := f.c.AppendBatch([]*vttv1.Envelope{qaCSubNarr("never written either"), qaCSubMove(qaCSubTokA, 1, 0)}); err == nil {
		t.Fatalf("a batch whose entry write fails succeeded")
	}
	qaCSubExec(t, db, `DROP TRIGGER qa_csub_refuse_entry`)
	qaCSubExec(t, db, `CREATE TRIGGER qa_csub_refuse_event BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT, 'refused'); END`)
	if _, err := f.c.Append(qaCSubNarr("no world row")); err == nil {
		t.Fatalf("an append whose world write fails succeeded")
	}
	qaCSubExec(t, db, `DROP TRIGGER qa_csub_refuse_event`)

	qaCSubQuiet(t, "pc-a after failures", a.ch)
	qaCSubQuiet(t, "pc-b after failures", b.ch)
	qaCSubQuiet(t, "world after failures", worldCh)
	if f.c.State() == nil {
		t.Fatalf("a failed write poisoned the campaign")
	}

	f.append(t, qaCSubNarr("afterwards"))
	for _, tc := range []struct {
		name  string
		actor string
		sub   qaCSubSub
		was   int
	}{{"pc-a", qaCSubA, a, len(logA)}, {"pc-b", qaCSubB, b, len(logB)}} {
		now := qaCSubLog(t, f.c, tc.actor)
		tail := now[tc.was:]
		if len(tail) == 0 {
			t.Fatalf("fixture: the good append wrote nothing to %s", tc.name)
		}
		got := qaCSubTake(t, tc.name, tc.sub.ch, len(tail))
		qaCSubSame(t, tc.name, got, tail)
		qaCSubContiguous(t, tc.name, got, int64(tc.was)+1)
		if txt := got[len(got)-1].GetNarrationAdded().GetText(); txt != "afterwards" {
			t.Fatalf("%s: last delivered entry is %s, want the narration \"afterwards\"", tc.name, qaCSubDesc(got[len(got)-1]))
		}
		qaCSubQuiet(t, tc.name+" after the good append", tc.sub.ch)
	}
}

// VTT-332, VTT-333, VTT-334.
func TestQACSubAPoisoningAppendDeliversNothingAndRefusesSubscriptions(t *testing.T) {
	f := qaCSubTable(t)
	logA := qaCSubLog(t, f.c, qaCSubA)
	a := qaCSubSubscribe(t, f.c, qaCSubA, int64(len(logA)), 4, 0)
	worldHead := qaCSubWorldHead(t, f.c)
	worldCh, worldUnsub, _, err := f.c.Subscribe(worldHead, 4)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Cleanup(worldUnsub)

	// Do not drop the DELETE: it is what makes the live apply diverge and poison.
	db := qaCSubRaw(t, f.dir)
	qaCSubExec(t, db, fmt.Sprintf(`DELETE FROM events WHERE seq = %d`, worldHead))
	if _, err := f.c.Append(qaCSubAnchored("the poison", 1, worldHead)); err == nil {
		t.Fatalf("fixture: the diverging append succeeded")
	}
	if f.c.State() != nil {
		t.Fatalf("fixture: the diverging append did not poison the campaign")
	}
	var stored int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_events WHERE actor_id = ? AND seq > ?`, qaCSubA, len(logA)).Scan(&stored); err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if stored == 0 {
		t.Fatalf("fixture: the diverging append persisted no entry for pc-a, so this does not show an unapplied entry")
	}

	qaCSubQuiet(t, "pc-a after poison", a.ch)
	qaCSubQuiet(t, "world after poison", worldCh)

	for _, actor := range []string{qaCSubA, qaCSubB, "qa-csub-nobody"} {
		for _, after := range []int64{0, int64(len(logA))} {
			if _, unsub, _, err := f.c.SubscribeCharacter(actor, after, 4, 0); err == nil {
				unsub()
				t.Fatalf("a poisoned campaign accepted a character subscription to %s after %d", actor, after)
			}
		}
	}
}

// VTT-335.
func TestQACSubClosingTheCampaignEndsEveryCharacterSubscription(t *testing.T) {
	f := qaCSubTable(t)
	logA := qaCSubLog(t, f.c, qaCSubA)
	logB := qaCSubLog(t, f.c, qaCSubB)
	drained := qaCSubSubscribe(t, f.c, qaCSubA, 0, 4, 0)
	qaCSubTake(t, "drained", drained.ch, len(logA))
	undrained := qaCSubSubscribe(t, f.c, qaCSubA, 0, 1, 0)
	other := qaCSubSubscribe(t, f.c, qaCSubB, int64(len(logB)), 4, 0)
	ghost := qaCSubSubscribe(t, f.c, "qa-csub-nobody", 0, 4, 0)

	f.close(t)
	const within = 3 * time.Second
	if n := qaCSubDrainToClose(t, "drained", drained.ch, within); n != 0 {
		t.Fatalf("drained: %d entries after Close, want none", n)
	}
	if n := qaCSubDrainToClose(t, "undrained", undrained.ch, within); n > len(logA) {
		t.Fatalf("undrained: %d entries, more than the log's %d", n, len(logA))
	}
	qaCSubDrainToClose(t, "pc-b", other.ch, within)
	qaCSubDrainToClose(t, "no log", ghost.ch, within)

	if _, unsub, _, err := f.c.SubscribeCharacter(qaCSubA, 0, 4, 0); err == nil {
		unsub()
		t.Fatal("SubscribeCharacter after Close returned no error")
	}
}

// VTT-336, VTT-337.
func TestQACSubAStalledReaderIsClosedAndDelaysNoOther(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := qaCSubTable(t)
		logA, logB := qaCSubLog(t, f.c, qaCSubA), qaCSubLog(t, f.c, qaCSubB)
		const budget = 2 * time.Second
		stalled := qaCSubSubscribe(t, f.c, qaCSubA, int64(len(logA)), 1, budget)
		active := qaCSubSubscribe(t, f.c, qaCSubA, int64(len(logA)), 1, 0)
		otherLog := qaCSubSubscribe(t, f.c, qaCSubB, int64(len(logB)), 1, 0)
		worldHead := qaCSubWorldHead(t, f.c)
		worldCh, worldUnsub, _, err := f.c.SubscribeWithNoProgressTimeout(worldHead, 1, 0)
		if err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
		t.Cleanup(worldUnsub)

		type tally struct {
			mu   sync.Mutex
			n    int
			last time.Time
		}
		read := func(ch <-chan *vttv1.Envelope, tl *tally, stop <-chan struct{}) {
			for {
				select {
				case _, ok := <-ch:
					if !ok {
						return
					}
					tl.mu.Lock()
					tl.n++
					tl.last = time.Now()
					tl.mu.Unlock()
				case <-stop:
					return
				}
			}
		}
		stop := make(chan struct{})
		var ta, tb, tw tally
		go read(active.ch, &ta, stop)
		go read(otherLog.ch, &tb, stop)
		go read(worldCh, &tw, stop)
		defer close(stop)

		const appends = 8
		start := time.Now()
		for i := 0; i < appends; i++ {
			f.append(t, qaCSubNarr(fmt.Sprintf("echo %d", i)))
		}
		appended := time.Since(start)
		if appended >= budget/2 {
			t.Fatalf("eight appends took %v with a stalled reader, want well under its %v budget", appended, budget)
		}
		wantA := len(qaCSubLog(t, f.c, qaCSubA)) - len(logA)
		wantB := len(qaCSubLog(t, f.c, qaCSubB)) - len(logB)
		if wantA <= 2 {
			t.Fatalf("fixture: %d entries for pc-a, want more than the stalled reader's buffer", wantA)
		}
		deadline := start.Add(budget * 3 / 4)
		for {
			ta.mu.Lock()
			na, lastA := ta.n, ta.last
			ta.mu.Unlock()
			tb.mu.Lock()
			nb := tb.n
			tb.mu.Unlock()
			tw.mu.Lock()
			nw := tw.n
			tw.mu.Unlock()
			if na == wantA && nb == wantB && nw == appends {
				if lastA.Sub(start) >= budget/2 {
					t.Fatalf("the active reader's last entry came %v after the first append, want well under %v", lastA.Sub(start), budget)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("before the stalled reader's budget ran out: active %d/%d, pc-b %d/%d, world %d/%d", na, wantA, nb, wantB, nw, appends)
			}
			time.Sleep(10 * time.Millisecond)
		}

		time.Sleep(budget + budget/2)
		n := qaCSubDrainToClose(t, "stalled", stalled.ch, qaCSubWait)
		if n >= wantA {
			t.Fatalf("stalled: read %d of %d entries before close, want it cut off", n, wantA)
		}

		f.append(t, qaCSubNarr("still here"))
		wantA2 := len(qaCSubLog(t, f.c, qaCSubA)) - len(logA)
		until := time.Now().Add(qaCSubWait)
		for {
			ta.mu.Lock()
			na := ta.n
			ta.mu.Unlock()
			if na == wantA2 {
				break
			}
			if time.Now().After(until) {
				t.Fatalf("after the stalled reader was dropped, the active reader has %d of %d entries", na, wantA2)
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// VTT-329.
func TestQACSubASlowReaderWithinItsBudgetIsKept(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := qaCSubTable(t)
		logA := qaCSubLog(t, f.c, qaCSubA)
		const budget = 900 * time.Millisecond
		slow := qaCSubSubscribe(t, f.c, qaCSubA, int64(len(logA)), 1, budget)
		for i := 0; i < 6; i++ {
			f.append(t, qaCSubNarr(fmt.Sprintf("slowly %d", i)))
		}
		tail := qaCSubLog(t, f.c, qaCSubA)[len(logA):]
		got := make([]*vttv1.Envelope, 0, len(tail))
		for len(got) < len(tail) {
			time.Sleep(budget / 3)
			select {
			case env, ok := <-slow.ch:
				if !ok {
					t.Fatalf("a reader accepting an entry every %v was closed after %d of %d", budget/3, len(got), len(tail))
				}
				got = append(got, env)
			case <-time.After(qaCSubWait):
				t.Fatalf("slow reader: %d of %d entries", len(got), len(tail))
			}
		}
		qaCSubSame(t, "slow reader", got, tail)
		qaCSubQuiet(t, "slow reader after", slow.ch)
	})
}

// VTT-333.
func TestQACSubAnArrivingEntryIsAlreadyPersistedAndApplied(t *testing.T) {
	f := qaCSubTable(t)
	logA := qaCSubLog(t, f.c, qaCSubA)
	sub := qaCSubSubscribe(t, f.c, qaCSubA, int64(len(logA)), 4, 0)
	db := qaCSubRaw(t, f.dir)

	x := int32(1)
	for i := 0; i < 4; i++ {
		to := 1 - x
		move := qaCSubMove(qaCSubTokA, x, to)
		x = to
		done := make(chan string, 1)
		go func(want *vttv1.Envelope, at int32) {
			for {
				select {
				case env, ok := <-sub.ch:
					if !ok {
						done <- "channel closed"
						return
					}
					var rows int
					if err := db.QueryRow(`SELECT COUNT(*) FROM character_events WHERE actor_id = ? AND seq = ?`, qaCSubA, env.GetSequence()).Scan(&rows); err != nil || rows != 1 {
						done <- fmt.Sprintf("entry %s arrived with %d committed rows (err %v)", qaCSubDesc(env), rows, err)
						return
					}
					if env.GetEventId() == "" {
						continue
					}
					if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE event_id = ?`, env.GetEventId()).Scan(&rows); err != nil || rows != 1 {
						done <- fmt.Sprintf("entry %s arrived before its world event was committed (%d rows, err %v)", qaCSubDesc(env), rows, err)
						return
					}
					if env.GetEventId() != want.GetEventId() {
						continue
					}
					st := f.c.State()
					if st == nil {
						done <- "campaign poisoned"
						return
					}
					if tok, ok := st.Tokens[qaCSubTokA]; !ok || tok.X != at {
						done <- fmt.Sprintf("entry %s arrived with the token at x=%d, want the move to x=%d applied", qaCSubDesc(env), tok.X, at)
						return
					}
					done <- ""
					return
				case <-time.After(qaCSubWait):
					done <- "the move's entry never arrived"
					return
				}
			}
		}(move, to)
		f.append(t, move)
		if msg := <-done; msg != "" {
			t.Fatalf("move %d: %s", i, msg)
		}
	}
}

// VTT-329.
func TestQACSubReadersRacingAppendsSeeEachEntryOnce(t *testing.T) {
	f := qaCSubTable(t)
	var wg sync.WaitGroup
	wg.Add(1)
	errs := make(chan error, 1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			var err error
			if i%5 == 4 {
				_, err = f.c.AppendBatch([]*vttv1.Envelope{qaCSubNarr(fmt.Sprintf("race %d a", i)), qaCSubNarr(fmt.Sprintf("race %d b", i))})
			} else {
				_, err = f.c.Append(qaCSubNarr(fmt.Sprintf("race %d", i)))
			}
			if err != nil {
				errs <- err
				return
			}
		}
	}()
	type reader struct {
		sub    qaCSubSub
		cursor int64
	}
	var readers []reader
	for i := 0; i < 6; i++ {
		cursor := int64(i % 3)
		readers = append(readers, reader{qaCSubSubscribe(t, f.c, qaCSubA, cursor, 2, 0), cursor})
		time.Sleep(3 * time.Millisecond)
	}
	wg.Wait()
	select {
	case err := <-errs:
		t.Fatalf("append: %v", err)
	default:
	}
	final := qaCSubLog(t, f.c, qaCSubA)
	raced := 0
	for _, r := range readers {
		if r.sub.head < int64(len(final)) {
			raced++
		}
	}
	if raced == 0 {
		t.Fatalf("fixture: every reader subscribed after the last append, so none raced one")
	}
	for i, r := range readers {
		label := fmt.Sprintf("reader %d after %d", i, r.cursor)
		got := qaCSubTake(t, label, r.sub.ch, len(final)-int(r.cursor))
		qaCSubSame(t, label, got, final[r.cursor:])
		qaCSubQuiet(t, label, r.sub.ch)
	}
}

// VTT-329.
func TestQACSubAReopenedCampaignDeliversHistoryThenLive(t *testing.T) {
	f := qaCSubTable(t)
	log := qaCSubLog(t, f.c, qaCSubA)
	f.reopen(t)
	sub := qaCSubSubscribe(t, f.c, qaCSubA, 1, 2, 0)
	if want := int64(len(log)); sub.head != want {
		t.Fatalf("catch-up head %d after reopen, want %d", sub.head, want)
	}
	qaCSubSame(t, "history after reopen", qaCSubTake(t, "history after reopen", sub.ch, len(log)-1), log[1:])
	f.batch(t, qaCSubNarr("morning"), qaCSubMove(qaCSubTokA, 1, 0))
	tail := qaCSubLog(t, f.c, qaCSubA)[len(log):]
	if len(tail) < 2 {
		t.Fatalf("fixture: the batch wrote %d entries to pc-a's log", len(tail))
	}
	got := qaCSubTake(t, "live after reopen", sub.ch, len(tail))
	qaCSubSame(t, "live after reopen", got, tail)
	qaCSubContiguous(t, "live after reopen", got, int64(len(log))+1)
	qaCSubQuiet(t, "after reopen", sub.ch)
}
