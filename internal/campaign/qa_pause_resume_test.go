package campaign_test

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const (
	qaPauseEye         = "eye"
	qaPauseFriend      = "friend"
	qaPauseVisitor     = "visitor"
	qaPauseVisitor2    = "visitor2"
	qaPauseRoomA       = "room-a"
	qaPauseRoomB       = "room-b"
	qaPauseTokEye      = "tok-eye"
	qaPauseTokFriend   = "tok-friend"
	qaPauseTokVisitor  = "tok-visitor"
	qaPauseTokVisitor2 = "tok-visitor2"
	qaPausePlayer      = "p1"
	qaPausePM          = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaPauseNP          = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	qaPausePublic      = vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC
)

type qaPauseWorld struct {
	t   *testing.T
	dir string
	c   *campaign.Campaign
	n   int
}

func qaPauseOpen(t *testing.T) *qaPauseWorld {
	t.Helper()
	w := &qaPauseWorld{t: t, dir: t.TempDir()}
	c, err := campaign.Open(w.dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	w.c = c
	t.Cleanup(func() { _ = w.c.Close() })
	return w
}

func (w *qaPauseWorld) reopen() {
	w.t.Helper()
	if err := w.c.Close(); err != nil {
		w.t.Fatalf("Close: %v", err)
	}
	c, err := campaign.Open(w.dir)
	if err != nil {
		w.t.Fatalf("reopen: %v", err)
	}
	w.c = c
}

func (w *qaPauseWorld) stamp(e *vttv1.Envelope) *vttv1.Envelope {
	w.n++
	e.EventId = fmt.Sprintf("qa-pause-%d", w.n)
	e.OccurredAt = timestamppb.Now()
	e.ActorRole = "dm"
	return e
}

func (w *qaPauseWorld) append(e *vttv1.Envelope) int64 {
	w.t.Helper()
	seq, err := w.c.Append(w.stamp(e))
	if err != nil {
		w.t.Fatalf("Append %T: %v", e.Payload, err)
	}
	return seq
}

func (w *qaPauseWorld) batch(es ...*vttv1.Envelope) error {
	for _, e := range es {
		w.stamp(e)
	}
	_, err := w.c.AppendBatch(es)
	return err
}

func (w *qaPauseWorld) log(actor string) []campaign.CharacterEntry {
	w.t.Helper()
	got, err := w.c.CharacterLog(actor)
	if err != nil {
		w.t.Fatalf("CharacterLog(%q): %v", actor, err)
	}
	return got
}

// step appends e and returns its world sequence and what it added to actor's log.
func (w *qaPauseWorld) step(actor string, e *vttv1.Envelope) (int64, []campaign.CharacterEntry) {
	w.t.Helper()
	before := w.log(actor)
	seq := w.append(e)
	after := w.log(actor)
	qaPauseSamePrefix(w.t, before, after)
	return seq, after[len(before):]
}

func (w *qaPauseWorld) stepBatch(actor string, es ...*vttv1.Envelope) []campaign.CharacterEntry {
	w.t.Helper()
	before := w.log(actor)
	if err := w.batch(es...); err != nil {
		w.t.Fatalf("AppendBatch: %v", err)
	}
	after := w.log(actor)
	qaPauseSamePrefix(w.t, before, after)
	return after[len(before):]
}

// quiet appends e and fails unless it added nothing to actor's log.
func (w *qaPauseWorld) quiet(actor string, e *vttv1.Envelope) int64 {
	w.t.Helper()
	seq, added := w.step(actor, e)
	if len(added) != 0 {
		w.t.Fatalf("%T at world seq %d wrote %d entries to %q's log while it is not a party member: %s",
			e.Payload, seq, len(added), actor, qaPauseDump(added))
	}
	return seq
}

func (w *qaPauseWorld) world() *engine.State {
	w.t.Helper()
	st := w.c.State()
	if st == nil {
		w.t.Fatalf("State() is nil: the campaign is poisoned")
	}
	return st
}

func qaPauseScene(id string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 8, GridHeight: 8,
	}}}
}

func qaPauseActor(id, name string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: id, Name: name, Kind: kind,
		Resources: map[string]*vttv1.Resource{"pool": {Current: 10, Max: 10}},
	}}}}
}

func qaPausePlace(tok, scene, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: scene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func qaPauseMove(tok, scene string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tok, SceneId: scene, To: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func qaPauseGrant(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: participant, Kind: kind,
	}}}
}

func qaPauseRevoke(actor, participant string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: actor, ParticipantId: participant,
	}}}
}

func qaPauseRemoveToken(tok string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tok}}}
}

func qaPauseRemoveActor(actor string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}}
}

func qaPauseStart(name string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: name}}}
}

func qaPauseEnd() *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}}
}

func qaPauseNote(key, text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: key, Text: text, Visibility: qaPausePublic,
	}}}
}

func qaPauseDeleteNote(key string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteDeleted{NoteDeleted: &vttv1.NoteDeleted{Key: key}}}
}

func qaPauseNarrate(text string, from, to int64) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
		Text: text, AnchorFromSeq: from, AnchorToSeq: to,
	}}}
}

func qaPausePool(actor string, delta, newValue int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: actor, Resource: "pool", Delta: delta, NewValue: newValue,
	}}}
}

func qaPauseMark(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cond,
	}}}
}

func qaPauseAttack(attacker, target string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AttackRolled{AttackRolled: &vttv1.AttackRolled{
		AttackerId: attacker, TargetId: target, Expression: "1d20", Total: 12,
	}}}
}

// qaPauseBase builds room-a with eye and friend (party members) and visitor
// (not one), all in one another's view, and a public note.
func qaPauseBase(w *qaPauseWorld, session bool) {
	w.t.Helper()
	w.append(qaPauseScene(qaPauseRoomA))
	if session {
		w.append(qaPauseStart("first"))
	}
	w.append(qaPauseNote("lore", "a public fact"))
	w.append(qaPauseActor(qaPauseEye, "Asme", qaPausePM))
	w.append(qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
	w.append(qaPausePlace(qaPauseTokEye, qaPauseRoomA, qaPauseEye, 1, 1))
	w.append(qaPauseActor(qaPauseFriend, "Kit", qaPausePM))
	w.append(qaPausePlace(qaPauseTokFriend, qaPauseRoomA, qaPauseFriend, 2, 1))
	w.append(qaPauseActor(qaPauseVisitor, "Stranger", qaPauseNP))
	w.append(qaPausePlace(qaPauseTokVisitor, qaPauseRoomA, qaPauseVisitor, 3, 3))
	if len(w.log(qaPauseEye)) == 0 {
		w.t.Fatalf("fixture: eye's log is empty after the base")
	}
}

func qaPauseEnvs(es []campaign.CharacterEntry) []*vttv1.Envelope {
	out := make([]*vttv1.Envelope, len(es))
	for i, e := range es {
		out[i] = e.Env
	}
	return out
}

func qaPauseName(e *vttv1.Envelope) string {
	return fmt.Sprintf("%T", e.GetPayload())[len("*vttv1.Envelope_"):]
}

func qaPauseDump(es []campaign.CharacterEntry) string {
	s := "["
	for i, e := range es {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%d:%s(w%d)", e.Env.GetSequence(), qaPauseName(e.Env), e.WorldSeq)
		switch p := e.Env.GetPayload().(type) {
		case *vttv1.Envelope_TokenHidden:
			s += "{" + p.TokenHidden.GetTokenId() + "}"
		case *vttv1.Envelope_TokenPlaced:
			s += fmt.Sprintf("{%s@%d,%d}", p.TokenPlaced.GetTokenId(), p.TokenPlaced.GetPosition().GetX(), p.TokenPlaced.GetPosition().GetY())
		case *vttv1.Envelope_ActorAdded:
			s += "{" + p.ActorAdded.GetActor().GetActorId() + "}"
		case *vttv1.Envelope_ActorRemoved:
			s += "{" + p.ActorRemoved.GetActorId() + "}"
		case *vttv1.Envelope_SessionStarted:
			s += "{" + e.Env.GetSessionId() + "}"
		}
	}
	return s + "]"
}

// qaPauseFoldAll folds every prefix of a log (VTT-323) and returns the whole fold.
func qaPauseFoldAll(t *testing.T, es []campaign.CharacterEntry) *engine.State {
	t.Helper()
	envs := qaPauseEnvs(es)
	for i := 1; i <= len(envs); i++ {
		if _, err := campaign.FoldPrefix(envs[:i]); err != nil {
			t.Fatalf("log prefix of %d entries does not fold: %v\n%s", i, err, qaPauseDump(es))
		}
	}
	st, err := campaign.FoldPrefix(envs)
	if err != nil {
		t.Fatalf("log does not fold: %v", err)
	}
	return st
}

// qaPauseNumbered checks VTT-321: entries are numbered from 1 without gaps.
func qaPauseNumbered(t *testing.T, es []campaign.CharacterEntry) {
	t.Helper()
	for i, e := range es {
		if e.Env.GetSequence() != int64(i+1) {
			t.Fatalf("entry %d carries sequence %d, want %d", i, e.Env.GetSequence(), i+1)
		}
	}
}

// qaPauseSamePrefix checks VTT-326: what was written is never changed.
func qaPauseSamePrefix(t *testing.T, before, after []campaign.CharacterEntry) {
	t.Helper()
	if len(after) < len(before) {
		t.Fatalf("log shrank from %d to %d entries", len(before), len(after))
	}
	for i := range before {
		if before[i].WorldSeq != after[i].WorldSeq || !proto.Equal(before[i].Env, after[i].Env) {
			t.Fatalf("entry %d changed after it was written", i)
		}
	}
}

func qaPauseOpenSession(st *engine.State) (engine.Session, bool) {
	for i := len(st.Sessions) - 1; i >= 0; i-- {
		if st.Sessions[i].EndSeq == 0 {
			return st.Sessions[i], true
		}
	}
	return engine.Session{}, false
}

// qaPauseTokensStand checks that every token the log holds stands on its
// square in the world.
func qaPauseTokensStand(t *testing.T, logSt, world *engine.State) {
	t.Helper()
	for id, tok := range logSt.Tokens {
		wt, ok := world.Tokens[id]
		if !ok {
			t.Errorf("log holds token %q, which the world no longer has", id)
			continue
		}
		if wt != tok {
			t.Errorf("log holds token %q as %+v, the world as %+v", id, tok, wt)
		}
	}
}

func qaPauseIsSession(e *vttv1.Envelope) bool {
	switch e.GetPayload().(type) {
	case *vttv1.Envelope_SessionStarted, *vttv1.Envelope_SessionEnded:
		return true
	}
	return false
}

// qaPauseSplit splits a resume's entries into its leading session entries, the
// token hides that follow them, and the rest.
func qaPauseSplit(r []campaign.CharacterEntry) (session, hides, rest []campaign.CharacterEntry) {
	i := 0
	for i < len(r) && qaPauseIsSession(r[i].Env) {
		i++
	}
	j := i
	for j < len(r) && r[j].Env.GetTokenHidden() != nil {
		j++
	}
	return r[:i], r[i:j], r[j:]
}

// qaPauseCheckResume checks VTT-318 for r, the entries one resuming event
// added to a log whose entries before it were before, against world after it.
func qaPauseCheckResume(t *testing.T, before, r []campaign.CharacterEntry, world *engine.State) (rest []campaign.CharacterEntry) {
	t.Helper()
	paused := qaPauseFoldAll(t, before)
	if len(r) == 0 {
		t.Fatalf("the resuming event wrote nothing to the log")
	}
	session, hides, rest := qaPauseSplit(r)

	hidden := map[string]int{}
	for _, h := range hides {
		hidden[h.Env.GetTokenHidden().GetTokenId()]++
	}
	for id := range paused.Tokens {
		if hidden[id] != 1 {
			t.Errorf("token %q, held by the paused board, is hidden %d times in the resume's leading entries, want once: %s",
				id, hidden[id], qaPauseDump(r))
		}
	}
	for id := range hidden {
		if _, held := paused.Tokens[id]; !held {
			t.Errorf("the resume hides %q, which the paused board does not hold", id)
		}
	}
	for _, e := range rest {
		if qaPauseIsSession(e.Env) {
			t.Errorf("session entry %s after the resume's leading entries: %s", qaPauseName(e.Env), qaPauseDump(r))
		}
		if h := e.Env.GetTokenHidden(); h != nil {
			if _, held := paused.Tokens[h.GetTokenId()]; held {
				t.Errorf("token %q hidden after the first decision frame: %s", h.GetTokenId(), qaPauseDump(r))
			}
		}
	}
	removed := map[string]bool{}
	for _, e := range r {
		if e.Env.GetNoteUpserted() != nil {
			t.Errorf("the resume wrote a NoteUpserted %q, an opening's entry: %s", e.Env.GetNoteUpserted().GetKey(), qaPauseDump(r))
		}
		if ar := e.Env.GetActorRemoved(); ar != nil {
			removed[ar.GetActorId()] = true
		}
		if aa := e.Env.GetActorAdded(); aa != nil {
			id := aa.GetActor().GetActorId()
			if _, held := paused.Actors[id]; held && !removed[id] {
				t.Errorf("the resume introduces %q, which the paused log holds, without withdrawing it first", id)
			}
		}
	}

	after := qaPauseFoldAll(t, append(append([]campaign.CharacterEntry{}, before...), r...))
	ls, lOpen := qaPauseOpenSession(after)
	ws, wOpen := qaPauseOpenSession(world)
	if lOpen != wOpen || ls.ID != ws.ID {
		t.Errorf("after the resume the log's open session is %q (open %v), the world's %q (open %v)", ls.ID, lOpen, ws.ID, wOpen)
	}
	ps, pOpen := qaPauseOpenSession(paused)
	same := pOpen == wOpen && ps.ID == ws.ID
	if same && len(session) != 0 {
		t.Errorf("the paused log already had the world's open session, yet the resume wrote %d session entries", len(session))
	}
	if !same && len(session) == 0 {
		t.Errorf("the paused log's open session %q differs from the world's %q, yet the resume wrote no session entry first: %s",
			ps.ID, ws.ID, qaPauseDump(r))
	}
	for _, e := range session {
		if e.Env.GetSessionStarted() != nil && e.Env.GetSessionId() != ws.ID {
			t.Errorf("the resume starts session %q, the world's open session is %q", e.Env.GetSessionId(), ws.ID)
		}
	}
	return rest
}

func qaPauseNoDrops(t *testing.T, w *qaPauseWorld) {
	t.Helper()
	if n := w.c.DiscardedForTest(); n != 0 {
		t.Errorf("%d frame sets were discarded as not folding, in a history the spec says folds", n)
	}
	logSt := qaPauseFoldAll(t, w.log(qaPauseEye))
	mem := w.c.CharacterFoldForTest(qaPauseEye)
	if mem == nil {
		t.Fatalf("CharacterFoldForTest(eye) is nil for a non-empty log")
	}
	if fmt.Sprint(mem.Tokens) != fmt.Sprint(logSt.Tokens) || fmt.Sprint(mem.Sessions) != fmt.Sprint(logSt.Sessions) ||
		qaPauseRoster(mem) != qaPauseRoster(logSt) {
		t.Errorf("eye's memory is not the fold of its log:\nmemory tokens %v roster %s\nlog    tokens %v roster %s",
			mem.Tokens, qaPauseRoster(mem), logSt.Tokens, qaPauseRoster(logSt))
	}
}

func qaPauseRoster(st *engine.State) string {
	names := map[string]string{}
	for id, a := range st.Actors {
		names[id] = a.GetName()
	}
	return fmt.Sprint(names)
}

type qaPauseBatteryItem struct {
	name     string
	needsEye bool
	env      func() *vttv1.Envelope
}

// qaPauseBattery is events each of which reaches a party member's log in view.
func qaPauseBattery() []qaPauseBatteryItem {
	return []qaPauseBatteryItem{
		{"move a token in view", false, func() *vttv1.Envelope { return qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 3) }},
		{"move eye's own token", true, func() *vttv1.Envelope { return qaPauseMove(qaPauseTokEye, qaPauseRoomA, 1, 2) }},
		{"change eye's pool", true, func() *vttv1.Envelope { return qaPausePool(qaPauseEye, -1, 9) }},
		{"mark eye", true, func() *vttv1.Envelope { return qaPauseMark(qaPauseEye, "marked") }},
		{"attack between actors in view", true, func() *vttv1.Envelope { return qaPauseAttack(qaPauseVisitor, qaPauseEye) }},
		{"narrate", false, func() *vttv1.Envelope { return qaPauseNarrate("while away", 0, 0) }},
		{"upsert a public note", false, func() *vttv1.Envelope { return qaPauseNote("lore2", "another public fact") }},
		{"delete a public note the log holds", false, func() *vttv1.Envelope { return qaPauseDeleteNote("lore") }},
		{"end the session", false, func() *vttv1.Envelope { return qaPauseEnd() }},
		{"start a session", false, func() *vttv1.Envelope { return qaPauseStart("second") }},
		{"add a party member", false, func() *vttv1.Envelope { return qaPauseActor("late", "Late", qaPausePM) }},
		{"grant the friend", false, func() *vttv1.Envelope { return qaPauseGrant(qaPauseFriend, "p2", qaPausePM) }},
	}
}

// VTT-316, the pause half: a grant that makes the actor not a party member
// pauses its log, and the same events reach it when it is not paused.
// VTT-316
func TestQAPauseADemotedActorsLogReceivesNothing(t *testing.T) {
	for _, paused := range []bool{false, true} {
		name := "control, never paused"
		if paused {
			name = "paused by a non-party grant"
		}
		t.Run(name, func(t *testing.T) {
			w := qaPauseOpen(t)
			qaPauseBase(w, true)
			if paused {
				w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			}
			for _, item := range qaPauseBattery() {
				friendBefore := len(w.log(qaPauseFriend))
				seq, added := w.step(qaPauseEye, item.env())
				if paused && len(added) != 0 {
					t.Errorf("%s (world seq %d) wrote to the paused log: %s", item.name, seq, qaPauseDump(added))
				}
				if !paused && len(added) == 0 {
					t.Errorf("control: %s (world seq %d) wrote nothing to an unpaused log", item.name, seq)
				}
				if len(w.log(qaPauseFriend)) == friendBefore {
					t.Errorf("%s wrote nothing to friend's log, which no pause touches", item.name)
				}
			}
			es := w.log(qaPauseEye)
			for _, e := range es {
				if g := e.Env.GetActorControlGranted(); g != nil && g.GetActorId() == qaPauseEye && g.GetKind() == qaPauseNP {
					t.Errorf("the log holds the grant that made eye not a party member")
				}
			}
			qaPauseFoldAll(t, es)
			qaPauseNumbered(t, es)
			qaPauseFoldAll(t, w.log(qaPauseFriend))
		})
	}
}

// VTT-316, the pause half: a removal pauses the log, and the removal itself
// is not written.
// VTT-316
func TestQAPauseARemovedPartyMembersLogReceivesNothing(t *testing.T) {
	w := qaPauseOpen(t)
	qaPauseBase(w, true)
	added := w.stepBatch(qaPauseEye, qaPauseRemoveToken(qaPauseTokEye), qaPauseRemoveActor(qaPauseEye))
	for _, e := range added {
		if ar := e.Env.GetActorRemoved(); ar != nil && ar.GetActorId() == qaPauseEye {
			t.Errorf("the log holds its own actor's removal, written after it stopped being a party member: %s", qaPauseDump(added))
		}
	}
	for _, item := range qaPauseBattery() {
		if item.needsEye {
			continue
		}
		friendBefore := len(w.log(qaPauseFriend))
		seq, got := w.step(qaPauseEye, item.env())
		if len(got) != 0 {
			t.Errorf("%s (world seq %d) wrote to a removed actor's log: %s", item.name, seq, qaPauseDump(got))
		}
		if len(w.log(qaPauseFriend)) == friendBefore {
			t.Errorf("%s wrote nothing to friend's log", item.name)
		}
	}
	qaPauseFoldAll(t, w.log(qaPauseEye))
	qaPauseNumbered(t, w.log(qaPauseEye))
}

// VTT-316: only party membership pauses a log; a revocation and a kindless
// grant leave the actor a party member.
// VTT-316
func TestQAPauseRevocationAndAKindlessGrantDoNotPause(t *testing.T) {
	w := qaPauseOpen(t)
	qaPauseBase(w, true)
	w.step(qaPauseEye, qaPauseRevoke(qaPauseEye, qaPausePlayer))
	if _, added := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 3)); len(added) == 0 {
		t.Errorf("after a revocation, a move in view wrote nothing: the revocation paused the log")
	}
	w.step(qaPauseEye, qaPauseGrant(qaPauseEye, "p2", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED))
	if _, added := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 5, 3)); len(added) == 0 {
		t.Errorf("after a kindless grant, a move in view wrote nothing: the grant paused the log")
	}
	qaPauseFoldAll(t, w.log(qaPauseEye))
}

// VTT-317 against VTT-318: the opening carries the open session and the public
// notes; the resume carries neither, and hides every token the board holds.
// VTT-318
func TestQAPauseTheResumeIsNotASecondOpening(t *testing.T) {
	w := qaPauseOpen(t)
	w.append(qaPauseScene(qaPauseRoomA))
	w.append(qaPauseStart("first"))
	w.append(qaPauseNote("lore", "a public fact"))
	w.append(qaPauseActor(qaPauseEye, "Asme", qaPauseNP))
	w.append(qaPausePlace(qaPauseTokEye, qaPauseRoomA, qaPauseEye, 1, 1))
	w.append(qaPauseActor(qaPauseFriend, "Kit", qaPausePM))
	w.append(qaPausePlace(qaPauseTokFriend, qaPauseRoomA, qaPauseFriend, 2, 1))
	if n := len(w.log(qaPauseEye)); n != 0 {
		t.Fatalf("fixture: a never-party actor's log holds %d entries", n)
	}
	_, opening := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
	var openedNote, openedSession bool
	for _, e := range opening {
		openedNote = openedNote || e.Env.GetNoteUpserted().GetKey() == "lore"
		openedSession = openedSession || e.Env.GetSessionStarted() != nil
	}
	if !openedNote || !openedSession {
		t.Fatalf("fixture: the opening did not carry the public note (%v) and the session (%v): %s",
			openedNote, openedSession, qaPauseDump(opening))
	}
	w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
	w.quiet(qaPauseEye, qaPauseMove(qaPauseTokFriend, qaPauseRoomA, 2, 2))
	before := w.log(qaPauseEye)
	_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
	qaPauseCheckResume(t, before, r, w.world())
	final := qaPauseFoldAll(t, w.log(qaPauseEye))
	qaPauseTokensStand(t, final, w.world())
	qaPauseNumbered(t, w.log(qaPauseEye))
	qaPauseNoDrops(t, w)
}

// VTT-318 and the ticket's item 5: a resume hides every token the paused board
// holds before it places what is in view, so a token moved during the pause
// stands on its new square.
// VTT-318
func TestQAPauseAResumeHidesEveryHeldTokenThenPlacesTheWorld(t *testing.T) {
	w := qaPauseOpen(t)
	qaPauseBase(w, true)
	w.append(qaPauseActor(qaPauseVisitor2, "Other", qaPauseNP))
	w.append(qaPausePlace(qaPauseTokVisitor2, qaPauseRoomA, qaPauseVisitor2, 5, 5))

	w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
	w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 4))
	w.quiet(qaPauseEye, qaPauseRemoveToken(qaPauseTokVisitor2))
	w.quiet(qaPauseEye, qaPauseActor("visitor3", "Third", qaPauseNP))
	w.quiet(qaPauseEye, qaPausePlace("tok-visitor3", qaPauseRoomA, "visitor3", 6, 6))
	w.quiet(qaPauseEye, qaPauseMove(qaPauseTokEye, qaPauseRoomA, 1, 2))
	w.quiet(qaPauseEye, qaPauseMove(qaPauseTokFriend, qaPauseRoomA, 2, 2))
	w.quiet(qaPauseEye, qaPauseScene(qaPauseRoomB))
	w.quiet(qaPauseEye, qaPauseActor("far", "Far", qaPauseNP))
	w.quiet(qaPauseEye, qaPausePlace("tok-far", qaPauseRoomB, "far", 0, 0))
	w.quiet(qaPauseEye, qaPauseNote("lore2", "upserted during the pause"))

	before := w.log(qaPauseEye)
	held := qaPauseFoldAll(t, before).Tokens
	if len(held) < 3 {
		t.Fatalf("fixture: the paused board holds %d tokens, want at least 3", len(held))
	}
	if _, ok := held[qaPauseTokVisitor]; !ok {
		t.Fatalf("fixture: the paused board does not hold the token that moves during the pause")
	}
	_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
	world := w.world()
	qaPauseCheckResume(t, before, r, world)

	es := w.log(qaPauseEye)
	final := qaPauseFoldAll(t, es)
	qaPauseTokensStand(t, final, world)
	want := map[string][2]int32{qaPauseTokEye: {1, 2}, qaPauseTokFriend: {2, 2}, qaPauseTokVisitor: {4, 4}, "tok-visitor3": {6, 6}}
	for id, at := range want {
		tok, ok := final.Tokens[id]
		if !ok || tok.X != at[0] || tok.Y != at[1] {
			t.Errorf("after the resume the log holds %q as %+v (present %v), want it at %v", id, tok, ok, at)
		}
	}
	for _, id := range []string{qaPauseTokVisitor2, "tok-far"} {
		if _, ok := final.Tokens[id]; ok {
			t.Errorf("after the resume the log holds %q, which is not in view", id)
		}
	}
	_, lore2 := final.Notes["lore2"]
	t.Logf("observed: a public note upserted during the pause is in the resumed log: %v", lore2)
	qaPauseNumbered(t, es)
	qaPauseNoDrops(t, w)
}

// VTT-318: the resume's first entries bring the log to the world's open
// session, before any token hide, and the next session event then folds.
// VTT-318
func TestQAPauseAResumeBringsTheLogToTheWorldsOpenSession(t *testing.T) {
	cases := []struct {
		name      string
		atPause   bool
		during    []string
		wantKinds []string
	}{
		{"ended and another started", true, []string{"end", "start"}, []string{"SessionEnded", "SessionStarted"}},
		{"ended", true, []string{"end"}, []string{"SessionEnded"}},
		{"started", false, []string{"start"}, []string{"SessionStarted"}},
		{"unchanged", true, nil, nil},
		{"none throughout", false, nil, nil},
		{"two sessions passed", true, []string{"end", "start", "end", "start"}, nil},
		{"two sessions passed, none open", true, []string{"end", "start", "end"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := qaPauseOpen(t)
			qaPauseBase(w, tc.atPause)
			w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			for i, d := range tc.during {
				if d == "end" {
					w.quiet(qaPauseEye, qaPauseEnd())
				} else {
					w.quiet(qaPauseEye, qaPauseStart(fmt.Sprintf("during-%d", i)))
				}
			}
			w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 4))
			before := w.log(qaPauseEye)
			_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
			world := w.world()
			qaPauseCheckResume(t, before, r, world)
			session, hides, _ := qaPauseSplit(r)
			if len(hides) == 0 {
				t.Errorf("the resume hid no token: %s", qaPauseDump(r))
			}
			if tc.wantKinds != nil {
				got := []string{}
				for _, e := range session {
					got = append(got, qaPauseName(e.Env))
				}
				if fmt.Sprint(got) != fmt.Sprint(tc.wantKinds) {
					t.Errorf("the resume's session entries are %v, want %v", got, tc.wantKinds)
				}
			}
			if ws, ok := qaPauseOpenSession(world); ok {
				for _, e := range session {
					if e.Env.GetSessionStarted() != nil && e.Env.GetSessionStarted().GetName() != ws.Name {
						t.Logf("observed: the resume starts session %q named %q, the world names it %q",
							e.Env.GetSessionId(), e.Env.GetSessionStarted().GetName(), ws.Name)
					}
				}
			}
			var next *vttv1.Envelope
			if _, ok := qaPauseOpenSession(world); ok {
				next = qaPauseEnd()
			} else {
				next = qaPauseStart("after")
			}
			_, added := w.step(qaPauseEye, next)
			wrote := false
			for _, e := range added {
				wrote = wrote || qaPauseIsSession(e.Env)
			}
			if !wrote {
				t.Errorf("the session event after the resume was not written to the log: %s", qaPauseDump(added))
			}
			es := w.log(qaPauseEye)
			final := qaPauseFoldAll(t, es)
			ls, lOpen := qaPauseOpenSession(final)
			ws, wOpen := qaPauseOpenSession(w.world())
			if lOpen != wOpen || ls.ID != ws.ID {
				t.Errorf("after the next session event the log's open session is %q (%v), the world's %q (%v)", ls.ID, lOpen, ws.ID, wOpen)
			}
			qaPauseNumbered(t, es)
			qaPauseNoDrops(t, w)
		})
	}
}

// The ticket's item 5, its pause case's last sentence, and the owner's ruling:
// a non-party actor the paused board held is removed during the pause and its
// id reused; the resume hides its token, withdraws it, then introduces the new one.
// VTT-318
func TestQAPauseAReusedIdIsWithdrawnBeforeItsNewActorIsIntroduced(t *testing.T) {
	for _, newTok := range []string{"tok-visitor-new", qaPauseTokVisitor} {
		t.Run("new token "+newTok, func(t *testing.T) {
			w := qaPauseOpen(t)
			qaPauseBase(w, true)
			w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			if added := w.stepBatch(qaPauseEye, qaPauseRemoveToken(qaPauseTokVisitor), qaPauseRemoveActor(qaPauseVisitor)); len(added) != 0 {
				t.Fatalf("the removal wrote to the paused log: %s", qaPauseDump(added))
			}
			w.quiet(qaPauseEye, qaPauseActor(qaPauseVisitor, "Someone Else", qaPauseNP))
			w.quiet(qaPauseEye, qaPausePlace(newTok, qaPauseRoomA, qaPauseVisitor, 5, 5))
			before := w.log(qaPauseEye)
			if old, ok := qaPauseFoldAll(t, before).Actors[qaPauseVisitor]; !ok || old.GetName() != "Stranger" {
				t.Fatalf("fixture: the paused log does not hold the old visitor")
			}
			_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
			world := w.world()
			rest := qaPauseCheckResume(t, before, r, world)
			removedAt, addedAt := -1, -1
			for i, e := range rest {
				if e.Env.GetActorRemoved().GetActorId() == qaPauseVisitor && removedAt < 0 {
					removedAt = i
				}
				if a := e.Env.GetActorAdded().GetActor(); a.GetActorId() == qaPauseVisitor && addedAt < 0 {
					addedAt = i
					if a.GetName() != "Someone Else" {
						t.Errorf("the introduced visitor is named %q, want the new actor", a.GetName())
					}
				}
			}
			if removedAt < 0 || addedAt < 0 || removedAt > addedAt {
				t.Errorf("want the old visitor withdrawn before the new one is introduced, got ActorRemoved at %d, ActorAdded at %d: %s",
					removedAt, addedAt, qaPauseDump(r))
			}
			es := w.log(qaPauseEye)
			final := qaPauseFoldAll(t, es)
			qaPauseTokensStand(t, final, world)
			if a := final.Actors[qaPauseVisitor]; a.GetName() != "Someone Else" {
				t.Errorf("after the resume the log holds visitor as %q", a.GetName())
			}
			if tok, ok := final.Tokens[newTok]; !ok || tok.X != 5 || tok.Y != 5 || tok.ActorID != qaPauseVisitor {
				t.Errorf("after the resume the log holds %q as %+v (present %v)", newTok, tok, ok)
			}
			qaPauseNumbered(t, es)
			qaPauseNoDrops(t, w)
		})
	}
}

// The ticket's TestANewActorUnderAnOldIdContinuesItsLog case and the owner's
// ruling: a removed party member's id is reused for another party member; the
// log continues, the old one is withdrawn before the new one is introduced.
// VTT-318
func TestQAPauseANewPartyMemberUnderAnOldIdContinuesItsLog(t *testing.T) {
	cases := []struct {
		name                string
		demoteFirst, reopen bool
	}{
		{"removed", false, false},
		{"removed, reopened during the pause", false, true},
		{"demoted then removed, its board still holding its token", true, false},
		{"demoted then removed, reopened during the pause", true, true},
	}
	for _, tc := range cases {
		demoteFirst, reopen := tc.demoteFirst, tc.reopen
		t.Run(tc.name, func(t *testing.T) {
			w := qaPauseOpen(t)
			qaPauseBase(w, true)
			if demoteFirst {
				w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
				if added := w.stepBatch(qaPauseEye, qaPauseRemoveToken(qaPauseTokEye), qaPauseRemoveActor(qaPauseEye)); len(added) != 0 {
					t.Fatalf("the removal wrote to the paused log: %s", qaPauseDump(added))
				}
				if _, held := qaPauseFoldAll(t, w.log(qaPauseEye)).Tokens[qaPauseTokEye]; !held {
					t.Fatalf("fixture: the paused board does not hold eye's own token")
				}
			} else {
				w.stepBatch(qaPauseEye, qaPauseRemoveToken(qaPauseTokEye), qaPauseRemoveActor(qaPauseEye))
			}
			w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 4))
			if reopen {
				w.reopen()
				w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 5, 4))
			}
			before := w.log(qaPauseEye)
			paused := qaPauseFoldAll(t, before)
			if a, ok := paused.Actors[qaPauseEye]; !ok || a.GetName() != "Asme" {
				t.Fatalf("fixture: the paused log does not hold the old eye")
			}
			_, r := w.step(qaPauseEye, qaPauseActor(qaPauseEye, "Bran", qaPausePM))
			if len(r) == 0 {
				t.Fatalf("the new party member under the old id got nothing")
			}
			rest := qaPauseCheckResume(t, before, r, w.world())
			removedAt, addedAt := -1, -1
			for i, e := range rest {
				if e.Env.GetActorRemoved().GetActorId() == qaPauseEye && removedAt < 0 {
					removedAt = i
				}
				if a := e.Env.GetActorAdded().GetActor(); a.GetActorId() == qaPauseEye && addedAt < 0 {
					addedAt = i
				}
			}
			if removedAt < 0 || addedAt < 0 || removedAt > addedAt {
				t.Errorf("want the old eye withdrawn before the new one is introduced, got ActorRemoved at %d, ActorAdded at %d: %s",
					removedAt, addedAt, qaPauseDump(r))
			}
			w.step(qaPauseEye, qaPausePlace("tok-eye-2", qaPauseRoomA, qaPauseEye, 2, 2))
			es := w.log(qaPauseEye)
			final := qaPauseFoldAll(t, es)
			world := w.world()
			qaPauseTokensStand(t, final, world)
			if a := final.Actors[qaPauseEye]; a.GetName() != "Bran" {
				t.Errorf("the log holds eye as %q, want the new actor", a.GetName())
			}
			if tok, ok := final.Tokens[qaPauseTokVisitor]; !ok || tok != world.Tokens[qaPauseTokVisitor] {
				t.Errorf("the log holds the visitor's token as %+v (present %v), the world as %+v", tok, ok, world.Tokens[qaPauseTokVisitor])
			}
			qaPauseNumbered(t, es)
			qaPauseNoDrops(t, w)
		})
	}
}

// VTT-316 and VTT-318 within one batch: the frames of an event appended while
// the actor is a party member are written, those of events while it is not are
// not, and a resume at the batch's end brings the board to the world.
// VTT-316 VTT-318
func TestQAPausePauseAndResumeInsideOneBatch(t *testing.T) {
	w := qaPauseOpen(t)
	qaPauseBase(w, true)
	added := w.stepBatch(qaPauseEye,
		qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 3),
		qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP),
		qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 5, 3),
		qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM),
	)
	movedAt, hiddenAt := -1, -1
	for i, e := range added {
		if m := e.Env.GetTokenMoved(); m != nil && m.GetTokenId() == qaPauseTokVisitor {
			if m.GetTo().GetX() == 5 {
				t.Errorf("the move made while eye was not a party member was forwarded: %s", qaPauseDump(added))
			}
			movedAt = i
		}
		if e.Env.GetTokenHidden().GetTokenId() == qaPauseTokVisitor {
			hiddenAt = i
		}
		if g := e.Env.GetActorControlGranted(); g != nil && g.GetKind() == qaPauseNP {
			t.Errorf("the demoting grant was written: %s", qaPauseDump(added))
		}
	}
	if movedAt < 0 || hiddenAt < movedAt {
		t.Errorf("want the first move forwarded, then the visitor's token hidden by the resume, got move at %d, hide at %d: %s",
			movedAt, hiddenAt, qaPauseDump(added))
	}
	final := qaPauseFoldAll(t, w.log(qaPauseEye))
	qaPauseTokensStand(t, final, w.world())
	if tok := final.Tokens[qaPauseTokVisitor]; tok.X != 5 || tok.Y != 3 {
		t.Errorf("after the batch the log holds the visitor at %d,%d, want 5,3", tok.X, tok.Y)
	}

	if got := w.stepBatch(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP), qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 3)); len(got) != 0 {
		t.Errorf("a batch that pauses first wrote: %s", qaPauseDump(got))
	}
	before := w.log(qaPauseEye)
	r := w.stepBatch(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 4), qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
	qaPauseCheckResume(t, before, r, w.world())
	final = qaPauseFoldAll(t, w.log(qaPauseEye))
	qaPauseTokensStand(t, final, w.world())
	qaPauseNumbered(t, w.log(qaPauseEye))
	qaPauseNoDrops(t, w)
}

// VTT-325 at a resume: a refused batch that would have resumed the log leaves
// it and the memory as they were, so the next resume still hides the board.
// VTT-325
func TestQAPauseARefusedResumeChangesNoMemory(t *testing.T) {
	refusals := map[string][]func() *vttv1.Envelope{
		"grant then a bad move": {
			func() *vttv1.Envelope { return qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM) },
			func() *vttv1.Envelope { return qaPauseMove("no-such-token", qaPauseRoomA, 1, 1) },
		},
		"grant, a good move, then a bad removal": {
			func() *vttv1.Envelope { return qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM) },
			func() *vttv1.Envelope { return qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 6) },
			func() *vttv1.Envelope { return qaPauseRemoveActor("no-such-actor") },
		},
	}
	for name, build := range refusals {
		t.Run(name, func(t *testing.T) {
			w := qaPauseOpen(t)
			qaPauseBase(w, true)
			w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 4))
			before := w.log(qaPauseEye)
			memBefore := w.c.CharacterFoldForTest(qaPauseEye)
			if memBefore == nil {
				t.Fatalf("CharacterFoldForTest is nil for a non-empty log")
			}
			envs := []*vttv1.Envelope{}
			for _, b := range build {
				envs = append(envs, b())
			}
			if err := w.batch(envs...); err == nil {
				t.Fatalf("fixture: the batch was accepted")
			}
			after := w.log(qaPauseEye)
			qaPauseSamePrefix(t, before, after)
			if len(after) != len(before) {
				t.Errorf("a refused batch added %d entries to the log", len(after)-len(before))
			}
			memAfter := w.c.CharacterFoldForTest(qaPauseEye)
			if memAfter == nil || fmt.Sprint(memAfter.Tokens) != fmt.Sprint(memBefore.Tokens) ||
				fmt.Sprint(memAfter.Sessions) != fmt.Sprint(memBefore.Sessions) || len(memAfter.Actors) != len(memBefore.Actors) {
				t.Errorf("a refused batch changed the log's memory: tokens %v -> %v", memBefore.Tokens, memAfter.Tokens)
			}
			_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
			qaPauseCheckResume(t, before, r, w.world())
			final := qaPauseFoldAll(t, w.log(qaPauseEye))
			qaPauseTokensStand(t, final, w.world())
			if tok := final.Tokens[qaPauseTokVisitor]; tok.X != 4 || tok.Y != 4 {
				t.Errorf("after the resume the log holds the visitor at %d,%d, want 4,4", tok.X, tok.Y)
			}
			qaPauseNoDrops(t, w)
		})
	}
}

// VTT-324 and VTT-318: a log paused across a reopen resumes without a second
// opening and hides every token its board holds.
// VTT-318 VTT-324
func TestQAPauseALogPausedAcrossAReopenResumes(t *testing.T) {
	for _, sessionChanges := range []bool{false, true} {
		t.Run(fmt.Sprintf("the session changes during the pause %v", sessionChanges), func(t *testing.T) {
			w := qaPauseOpen(t)
			qaPauseBase(w, true)
			w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 4))
			if sessionChanges {
				w.quiet(qaPauseEye, qaPauseEnd())
			}
			before := w.log(qaPauseEye)
			w.reopen()
			reread := w.log(qaPauseEye)
			qaPauseSamePrefix(t, before, reread)
			if len(reread) != len(before) {
				t.Fatalf("the log has %d entries after the reopen, %d before", len(reread), len(before))
			}
			mem := w.c.CharacterFoldForTest(qaPauseEye)
			if mem == nil || fmt.Sprint(mem.Tokens) != fmt.Sprint(qaPauseFoldAll(t, before).Tokens) {
				t.Errorf("after the reopen the log's memory is not the fold of its log")
			}
			w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 5, 4))
			w.quiet(qaPauseEye, qaPauseMove(qaPauseTokFriend, qaPauseRoomA, 2, 3))
			if sessionChanges {
				w.quiet(qaPauseEye, qaPauseStart("after the reopen"))
			}
			_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
			world := w.world()
			qaPauseCheckResume(t, before, r, world)
			if session, _, _ := qaPauseSplit(r); sessionChanges && len(session) == 0 {
				t.Errorf("the session changed during the pause and the resume wrote no session entry: %s", qaPauseDump(r))
			}
			es := w.log(qaPauseEye)
			final := qaPauseFoldAll(t, es)
			qaPauseTokensStand(t, final, world)
			if tok := final.Tokens[qaPauseTokVisitor]; tok.X != 5 || tok.Y != 4 {
				t.Errorf("after the resume the log holds the visitor at %d,%d, want 5,4", tok.X, tok.Y)
			}
			if _, added := w.step(qaPauseEye, qaPauseEnd()); len(added) == 0 || added[len(added)-1].Env.GetSessionEnded() == nil {
				t.Errorf("the session end after the resume was not written: %s", qaPauseDump(added))
			}
			qaPauseFoldAll(t, w.log(qaPauseEye))
			qaPauseNumbered(t, w.log(qaPauseEye))
			qaPauseNoDrops(t, w)
		})
	}
}

// VTT-328, with VTT-322 as its control: a narration's anchor in a character
// log names only the entries after the latest resume in its world range.
// VTT-328
func TestQAPauseAnAnchorAcrossAResumeNamesOnlyWhatFollowsTheLatestResume(t *testing.T) {
	type qaPauseRange struct{ from, to int64 }
	results := map[bool]map[string]qaPauseRange{}
	readings := map[string]string{}
	for _, reopen := range []bool{false, true} {
		t.Run(fmt.Sprintf("reopened before narrating %v", reopen), func(t *testing.T) {
			w := qaPauseOpen(t)
			first := func(es []campaign.CharacterEntry) int64 { return es[0].Env.GetSequence() }
			last := func(es []campaign.CharacterEntry) int64 { return es[len(es)-1].Env.GetSequence() }
			nonEmpty := func(what string, es []campaign.CharacterEntry) []campaign.CharacterEntry {
				if len(es) == 0 {
					t.Fatalf("fixture: %s wrote nothing to the log", what)
				}
				return es
			}
			w.append(qaPauseScene(qaPauseRoomA))
			w.append(qaPauseActor(qaPauseEye, "Asme", qaPausePM))
			w.append(qaPausePlace(qaPauseTokEye, qaPauseRoomA, qaPauseEye, 1, 1))
			w.append(qaPauseActor(qaPauseVisitor, "Stranger", qaPauseNP))
			sPlace, ePlace := w.step(qaPauseEye, qaPausePlace(qaPauseTokVisitor, qaPauseRoomA, qaPauseVisitor, 3, 3))
			nonEmpty("the visitor's placing", ePlace)
			sPre, ePre := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 3))
			nonEmpty("the move before the pause", ePre)
			sPause1 := w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			sPaused1 := w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 5, 3))
			before := w.log(qaPauseEye)
			sResume1, r1 := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
			rest1 := nonEmpty("the first resume's decision", qaPauseCheckResume(t, before, r1, w.world()))
			sPost1a, ePost1a := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 3))
			nonEmpty("the first move after the first resume", ePost1a)
			_, ePost1b := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 4))
			nonEmpty("the second move after the first resume", ePost1b)
			w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			sPaused2 := w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 7, 4))
			before = w.log(qaPauseEye)
			_, r2 := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
			rest2 := nonEmpty("the second resume's decision", qaPauseCheckResume(t, before, r2, w.world()))
			sPost2a, ePost2a := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 7, 5))
			nonEmpty("the first move after the second resume", ePost2a)
			_, ePost2b := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 7, 6))
			nonEmpty("the second move after the second resume", ePost2b)
			if reopen {
				w.reopen()
			}

			narrate := func(text string, from, to int64) qaPauseRange {
				_, added := w.step(qaPauseEye, qaPauseNarrate(text, from, to))
				for _, e := range added {
					if n := e.Env.GetNarrationAdded(); n != nil && n.GetText() == text {
						return qaPauseRange{n.GetAnchorFromSeq(), n.GetAnchorToSeq()}
					}
				}
				t.Fatalf("narration %q did not reach the log: %s", text, qaPauseDump(added))
				return qaPauseRange{}
			}
			got := map[string]qaPauseRange{}
			fixed := []struct {
				name     string
				from, to int64
				want     qaPauseRange
			}{
				{"no resume, before the pause", sPlace, sPre, qaPauseRange{first(ePlace), last(ePre)}},
				{"only the pause", sPause1, sPaused1, qaPauseRange{0, 0}},
				{"a pause and no resume", sPost1a, sPaused2, qaPauseRange{first(ePost1a), last(ePost1b)}},
			}
			for _, c := range fixed {
				g := narrate(c.name, c.from, c.to)
				got[c.name] = g
				if g != c.want {
					t.Errorf("%s: world range [%d,%d] anchors the log at %v, want %v", c.name, c.from, c.to, g, c.want)
				}
			}
			// Each case accepts two readings of "caused after the latest resume":
			// A counts the resuming event's decision frames, B starts at the next event.
			ambiguous := []struct {
				name     string
				from, to int64
				a, b     qaPauseRange
			}{
				{"spans one resume", sPre, sPost1a, qaPauseRange{first(rest1), last(ePost1a)}, qaPauseRange{first(ePost1a), last(ePost1a)}},
				{"spans two resumes", sPre, sPost2a, qaPauseRange{first(rest2), last(ePost2a)}, qaPauseRange{first(ePost2a), last(ePost2a)}},
				{"starts at a resume", sResume1, sPost1a, qaPauseRange{first(rest1), last(ePost1a)}, qaPauseRange{first(ePost1a), last(ePost1a)}},
				{"ends at a resume", sPaused1, sResume1, qaPauseRange{first(rest1), last(rest1)}, qaPauseRange{0, 0}},
			}
			for _, c := range ambiguous {
				g := narrate(c.name, c.from, c.to)
				got[c.name] = g
				switch g {
				case c.a:
					readings[c.name] = "A (the resuming event's decision frames included)"
				case c.b:
					readings[c.name] = "B (only events after the resuming one)"
				default:
					t.Errorf("%s: world range [%d,%d] anchors the log at %v, want %v (reading A) or %v (reading B); log %s",
						c.name, c.from, c.to, g, c.a, c.b, qaPauseDump(w.log(qaPauseEye)))
				}
			}
			results[reopen] = got
			es := w.log(qaPauseEye)
			qaPauseFoldAll(t, es)
			qaPauseNumbered(t, es)
			qaPauseNoDrops(t, w)
		})
	}
	for name, g := range results[false] {
		if r, ok := results[true][name]; ok && r != g {
			t.Errorf("%s: the anchor is %v live and %v after a reopen", name, g, r)
		}
	}
	for name, reading := range readings {
		t.Logf("observed: %s follows reading %s", name, reading)
	}
}

// VTT-325 with VTT-316: a refused batch that would have paused, or paused and
// resumed, the log leaves it unpaused, so the next event is forwarded as is.
// VTT-325
func TestQAPauseARefusedPauseChangesNoMemory(t *testing.T) {
	refusals := map[string][]func() *vttv1.Envelope{
		"pause then a bad move": {
			func() *vttv1.Envelope { return qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP) },
			func() *vttv1.Envelope { return qaPauseMove("no-such-token", qaPauseRoomA, 1, 1) },
		},
		"pause, a move, resume, then a bad move": {
			func() *vttv1.Envelope { return qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP) },
			func() *vttv1.Envelope { return qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 6) },
			func() *vttv1.Envelope { return qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM) },
			func() *vttv1.Envelope { return qaPauseMove("no-such-token", qaPauseRoomA, 1, 1) },
		},
	}
	for name, build := range refusals {
		t.Run(name, func(t *testing.T) {
			w := qaPauseOpen(t)
			qaPauseBase(w, true)
			before := w.log(qaPauseEye)
			envs := []*vttv1.Envelope{}
			for _, b := range build {
				envs = append(envs, b())
			}
			if err := w.batch(envs...); err == nil {
				t.Fatalf("fixture: the batch was accepted")
			}
			if after := w.log(qaPauseEye); len(after) != len(before) {
				t.Errorf("a refused batch added %d entries", len(after)-len(before))
			}
			_, added := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 3))
			if len(added) != 1 || added[0].Env.GetTokenMoved() == nil {
				t.Errorf("after a refused batch, a move in view was written as %s, want the forwarded move alone", qaPauseDump(added))
			}
			final := qaPauseFoldAll(t, w.log(qaPauseEye))
			qaPauseTokensStand(t, final, w.world())
			qaPauseNoDrops(t, w)
		})
	}
}

// VTT-318: a resume where the eye now stands in a scene created during the
// pause hides the old board, and the log holds the new scene and its token.
// VTT-318
func TestQAPauseAResumeIntoASceneCreatedDuringThePause(t *testing.T) {
	w := qaPauseOpen(t)
	qaPauseBase(w, true)
	w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
	w.quiet(qaPauseEye, qaPauseScene(qaPauseRoomB))
	w.quiet(qaPauseEye, qaPauseRemoveToken(qaPauseTokEye))
	w.quiet(qaPauseEye, qaPausePlace("tok-eye-b", qaPauseRoomB, qaPauseEye, 0, 0))
	w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 6))
	before := w.log(qaPauseEye)
	_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
	world := w.world()
	qaPauseCheckResume(t, before, r, world)
	final := qaPauseFoldAll(t, w.log(qaPauseEye))
	qaPauseTokensStand(t, final, world)
	if _, ok := final.Scenes[qaPauseRoomB]; !ok {
		t.Errorf("after the resume the log does not hold the scene the eye stands in")
	}
	if tok, ok := final.Tokens["tok-eye-b"]; !ok || tok.SceneID != qaPauseRoomB {
		t.Errorf("after the resume the log does not hold the eye's token in room-b: %+v", tok)
	}
	for _, id := range []string{qaPauseTokEye, qaPauseTokVisitor, qaPauseTokFriend} {
		if _, ok := final.Tokens[id]; ok {
			t.Errorf("after the resume the log holds %q, in a scene the eye no longer stands in", id)
		}
	}
	qaPauseNumbered(t, w.log(qaPauseEye))
	qaPauseNoDrops(t, w)
}

// VTT-328: a resume that writes no resume entry, because the board is empty
// and the session unchanged, still bounds an anchor, live and after a reopen.
// Two such resumes: a demoted actor with nothing on its board, and a removed
// party member whose id is reused (the owner's ruling).
// VTT-328
func TestQAPauseAnAnchorAcrossAResumeWithNoResumeEntries(t *testing.T) {
	type qaPauseHistory struct {
		sPre, sPost       int64
		ePre, rest, ePost []campaign.CharacterEntry
	}
	histories := map[string]func(t *testing.T, w *qaPauseWorld) qaPauseHistory{
		"demoted with an empty board": func(t *testing.T, w *qaPauseWorld) qaPauseHistory {
			t.Helper()
			var h qaPauseHistory
			w.append(qaPauseScene(qaPauseRoomA))
			w.append(qaPauseActor(qaPauseEye, "Asme", qaPausePM))
			h.sPre, h.ePre = w.step(qaPauseEye, qaPauseActor("m1", "One", qaPausePM))
			w.step(qaPauseEye, qaPauseActor("m2", "Two", qaPausePM))
			w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
			w.quiet(qaPauseEye, qaPauseActor("m3", "Three", qaPausePM))
			before := w.log(qaPauseEye)
			_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
			h.rest = qaPauseCheckResume(t, before, r, w.world())
			if len(h.rest) != len(r) {
				t.Fatalf("fixture: the resume wrote %d resume entries, want none", len(r)-len(h.rest))
			}
			h.sPost, h.ePost = w.step(qaPauseEye, qaPauseActor("m4", "Four", qaPausePM))
			w.step(qaPauseEye, qaPauseActor("m5", "Five", qaPausePM))
			return h
		},
		"removed and re-added under its id": func(t *testing.T, w *qaPauseWorld) qaPauseHistory {
			t.Helper()
			var h qaPauseHistory
			qaPauseBase(w, true)
			h.sPre, h.ePre = w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 3))
			w.stepBatch(qaPauseEye, qaPauseRemoveToken(qaPauseTokEye), qaPauseRemoveActor(qaPauseEye))
			w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 5, 3))
			before := w.log(qaPauseEye)
			_, r := w.step(qaPauseEye, qaPauseActor(qaPauseEye, "Bran", qaPausePM))
			h.rest = qaPauseCheckResume(t, before, r, w.world())
			if len(h.rest) != len(r) {
				t.Fatalf("fixture: the resume wrote %d resume entries, want none", len(r)-len(h.rest))
			}
			h.sPost, h.ePost = w.step(qaPauseEye, qaPausePlace("tok-eye-2", qaPauseRoomA, qaPauseEye, 2, 2))
			w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 3))
			return h
		},
	}
	for name, build := range histories {
		for _, reopen := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, reopened before narrating %v", name, reopen), func(t *testing.T) {
				w := qaPauseOpen(t)
				h := build(t, w)
				if len(h.ePre) == 0 || len(h.rest) == 0 || len(h.ePost) == 0 {
					t.Fatalf("fixture: an event around the resume wrote nothing")
				}
				if reopen {
					w.reopen()
				}
				_, added := w.step(qaPauseEye, qaPauseNarrate("across", h.sPre, h.sPost))
				var n *vttv1.NarrationAdded
				for _, e := range added {
					if e.Env.GetNarrationAdded() != nil {
						n = e.Env.GetNarrationAdded()
					}
				}
				if n == nil {
					t.Fatalf("the narration did not reach the log")
				}
				to := h.ePost[len(h.ePost)-1].Env.GetSequence()
				a, b := h.rest[0].Env.GetSequence(), h.ePost[0].Env.GetSequence()
				if n.GetAnchorToSeq() != to || (n.GetAnchorFromSeq() != a && n.GetAnchorFromSeq() != b) {
					t.Errorf("world range [%d,%d] anchors the log at [%d,%d], want [%d or %d, %d]; the entries before the pause start at %d\n%s",
						h.sPre, h.sPost, n.GetAnchorFromSeq(), n.GetAnchorToSeq(), a, b, to, h.ePre[0].Env.GetSequence(), qaPauseDump(w.log(qaPauseEye)))
				}
				qaPauseFoldAll(t, w.log(qaPauseEye))
				qaPauseNumbered(t, w.log(qaPauseEye))
			})
		}
	}
}

// VTT-318 with VTT-320 and VTT-323: status that changed during the pause, on an
// actor in view and on the eye itself, does not stop the next events about
// them from being written after the resume.
// VTT-318
func TestQAPauseStatusChangedDuringThePauseStillFoldsAfterTheResume(t *testing.T) {
	w := qaPauseOpen(t)
	qaPauseBase(w, true)
	w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
	w.quiet(qaPauseEye, qaPauseMark(qaPauseVisitor, "marked"))
	w.quiet(qaPauseEye, qaPausePool(qaPauseVisitor, -3, 7))
	w.quiet(qaPauseEye, qaPausePool(qaPauseEye, -4, 6))
	w.quiet(qaPauseEye, qaPauseMark(qaPauseEye, "slowed"))
	before := w.log(qaPauseEye)
	_, r := w.step(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM))
	qaPauseCheckResume(t, before, r, w.world())
	next := []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{ActorId: qaPauseVisitor, ConditionId: "marked"}}},
		qaPausePool(qaPauseVisitor, 1, 8),
		qaPausePool(qaPauseEye, 1, 7),
		{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{ActorId: qaPauseEye, ConditionId: "slowed"}}},
	}
	for _, e := range next {
		name := qaPauseName(e)
		if _, added := w.step(qaPauseEye, e); len(added) == 0 {
			t.Errorf("%s after the resume was not written to the log (discarded so far: %d)", name, w.c.DiscardedForTest())
		}
	}
	final := qaPauseFoldAll(t, w.log(qaPauseEye))
	world := w.world()
	for _, id := range []string{qaPauseVisitor, qaPauseEye} {
		if got, want := final.Actors[id].GetResources()["pool"].GetCurrent(), world.Actors[id].GetResources()["pool"].GetCurrent(); got != want {
			t.Errorf("the log holds %s's pool at %d, the world at %d", id, got, want)
		}
		if got, want := len(final.Conditions[id]), len(world.Conditions[id]); got != want {
			t.Errorf("the log holds %d conditions on %s, the world %d", got, id, want)
		}
	}
	qaPauseNoDrops(t, w)
}

// VTT-328 inside one batch: a narration in the same batch as the resume it
// spans names only what follows that resume.
// VTT-328
func TestQAPauseAnAnchorAcrossAResumeInsideOneBatch(t *testing.T) {
	w := qaPauseOpen(t)
	qaPauseBase(w, true)
	sPre, ePre := w.step(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 4, 3))
	w.quiet(qaPauseEye, qaPauseGrant(qaPauseEye, qaPausePlayer, qaPauseNP))
	head := w.quiet(qaPauseEye, qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 5, 3))
	sResume, sMove := head+1, head+2
	added := w.stepBatch(qaPauseEye,
		qaPauseGrant(qaPauseEye, qaPausePlayer, qaPausePM),
		qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 3),
		qaPauseMove(qaPauseTokVisitor, qaPauseRoomA, 6, 4),
		qaPauseNarrate("in the batch", sPre, sMove),
	)
	var n *vttv1.NarrationAdded
	var moveEntries []campaign.CharacterEntry
	_, hides, rest := qaPauseSplit(added)
	if len(hides) == 0 || len(rest) == 0 {
		t.Fatalf("fixture: the batch's resume hid nothing: %s", qaPauseDump(added))
	}
	firstAfterHides := rest[0].Env.GetSequence()
	for _, e := range added {
		if e.WorldSeq == sMove {
			moveEntries = append(moveEntries, e)
		}
		if e.Env.GetNarrationAdded() != nil {
			n = e.Env.GetNarrationAdded()
		}
	}
	if n == nil || len(moveEntries) == 0 {
		t.Fatalf("fixture: the narration or the move did not reach the log: %s", qaPauseDump(added))
	}
	to := moveEntries[len(moveEntries)-1].Env.GetSequence()
	a, b := firstAfterHides, moveEntries[0].Env.GetSequence()
	if n.GetAnchorToSeq() != to || (n.GetAnchorFromSeq() != a && n.GetAnchorFromSeq() != b) {
		t.Errorf("world range [%d,%d] spanning the resume at %d anchors the log at [%d,%d], want [%d or %d, %d]; before the pause it starts at %d\n%s",
			sPre, sMove, sResume, n.GetAnchorFromSeq(), n.GetAnchorToSeq(), a, b, to, ePre[0].Env.GetSequence(), qaPauseDump(w.log(qaPauseEye)))
	}
	qaPauseFoldAll(t, w.log(qaPauseEye))
	qaPauseNoDrops(t, w)
}
