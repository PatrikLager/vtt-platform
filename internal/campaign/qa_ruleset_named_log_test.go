package campaign_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const (
	qarlParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qarlNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qarlWorld struct {
	t       *testing.T
	dir     string
	c       *campaign.Campaign
	n       int
	session string
}

func qarlNew(t *testing.T) *qarlWorld {
	t.Helper()
	w := &qarlWorld{t: t, dir: t.TempDir()}
	w.open()
	t.Cleanup(func() {
		if w.c != nil {
			_ = w.c.Close()
		}
	})
	return w
}

func (w *qarlWorld) open() {
	w.t.Helper()
	c, err := campaign.Open(w.dir)
	if err != nil {
		w.t.Fatalf("open: %v", err)
	}
	w.c = c
}

func (w *qarlWorld) reopen() {
	w.t.Helper()
	if err := w.c.Close(); err != nil {
		w.t.Fatalf("close: %v", err)
	}
	w.c = nil
	w.open()
}

func (w *qarlWorld) envelope() *vttv1.Envelope {
	w.n++
	return &vttv1.Envelope{
		EventId:    fmt.Sprintf("qarl-%d", w.n),
		OccurredAt: timestamppb.Now(),
		SessionId:  w.session,
	}
}

func (w *qarlWorld) append(envs ...*vttv1.Envelope) int64 {
	w.t.Helper()
	var seq int64
	var err error
	if len(envs) == 1 {
		seq, err = w.c.Append(envs[0])
	} else {
		seq, err = w.c.AppendBatch(envs)
	}
	if err != nil {
		w.t.Fatalf("append %s: %v", qarlKinds(envs), err)
	}
	return seq
}

func (w *qarlWorld) seqOf(eventID string) int64 {
	w.t.Helper()
	events, stop, head, err := w.c.Subscribe(0, 1<<12)
	if err != nil {
		w.t.Fatalf("subscribe: %v", err)
	}
	defer stop()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-events:
			if !ok {
				w.t.Fatalf("world subscription closed before %s", eventID)
			}
			if e.GetEventId() == eventID {
				return e.GetSequence()
			}
			if e.GetSequence() >= head {
				w.t.Fatalf("no world event %s", eventID)
			}
		case <-timeout:
			w.t.Fatalf("world event %s not delivered", eventID)
		}
	}
}

func (w *qarlWorld) sessionStarted(id, name string) *vttv1.Envelope {
	w.session = id
	e := w.envelope()
	e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: name}}
	return e
}

func (w *qarlWorld) sessionEnded() *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}
	w.session = ""
	return e
}

func (w *qarlWorld) sceneCreated(id string) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: id, Name: "Room " + id, GridWidth: 6, GridHeight: 6,
	}}
	return e
}

func (w *qarlWorld) actorAdded(a *vttv1.Actor) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}
	return e
}

func (w *qarlWorld) tokenPlaced(token, scene, actor string, x, y int32) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: token, SceneId: scene, ActorId: actor,
		Position: &vttv1.GridPosition{X: x, Y: y},
	}}
	return e
}

func (w *qarlWorld) granted(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: participant, Kind: kind,
	}}
	return e
}

func (w *qarlWorld) revoked(actor, participant string) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: actor, ParticipantId: participant,
	}}
	return e
}

func (w *qarlWorld) rulesetNamed(r *vttv1.RulesetNamed) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_RulesetNamed{RulesetNamed: proto.Clone(r).(*vttv1.RulesetNamed)}
	return e
}

func (w *qarlWorld) noteUpserted(key string) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: "Title " + key, Text: "Text " + key,
		Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
	}}
	return e
}

func (w *qarlWorld) narration(text string) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}
	return e
}

func (w *qarlWorld) conditionApplied(actor, condition string) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: condition,
	}}
	return e
}

func (w *qarlWorld) tokenRemoved(token string) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: token}}
	return e
}

func (w *qarlWorld) actorRemoved(actor string) *vttv1.Envelope {
	e := w.envelope()
	e.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}
	return e
}

// addActor adds a non-party actor, with a token on scene unless scene is "".
func (w *qarlWorld) addActor(id, scene string, x, y int32) {
	w.t.Helper()
	w.append(w.actorAdded(&vttv1.Actor{
		ActorId: id, Name: "Name " + id, ModuleId: "qa-module", Kind: qarlNonParty,
	}))
	if scene != "" {
		w.append(w.tokenPlaced("tok-"+id, scene, id, x, y))
	}
}

func (w *qarlWorld) makeParty(id string) int64 {
	w.t.Helper()
	return w.append(w.granted(id, "p-"+id, qarlParty))
}

func (w *qarlWorld) pause(id string) {
	w.t.Helper()
	w.append(w.revoked(id, "p-"+id))
	w.append(w.granted(id, "p-dm", qarlNonParty))
}

func (w *qarlWorld) log(actor string) []campaign.CharacterEntry {
	w.t.Helper()
	entries, err := w.c.CharacterLog(actor)
	if err != nil {
		w.t.Fatalf("character log %s: %v", actor, err)
	}
	return entries
}

func (w *qarlWorld) openSession() engine.Session {
	w.t.Helper()
	st := w.c.State()
	for i := len(st.Sessions) - 1; i >= 0; i-- {
		if st.Sessions[i].EndSeq == 0 {
			return st.Sessions[i]
		}
	}
	w.t.Fatalf("the world holds no open session")
	return engine.Session{}
}

func qarlKind(e *vttv1.Envelope) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", e.GetPayload()), "*vttv1.Envelope_")
}

func qarlKinds(envs []*vttv1.Envelope) string {
	ks := make([]string, len(envs))
	for i, e := range envs {
		ks[i] = qarlKind(e)
	}
	return strings.Join(ks, ",")
}

func qarlDump(entries []campaign.CharacterEntry) string {
	var b strings.Builder
	for i, e := range entries {
		fmt.Fprintf(&b, "\n  %d ws=%d %s", i, e.WorldSeq, qarlKind(e.Env))
		if r := e.Env.GetRulesetNamed(); r != nil {
			fmt.Fprintf(&b, " %s/%s/%s", r.GetRulesetId(), r.GetVersion(), r.GetName())
		}
	}
	return b.String()
}

func qarlFold(t *testing.T, entries []campaign.CharacterEntry) *engine.State {
	t.Helper()
	envs := make([]*vttv1.Envelope, len(entries))
	for i, e := range entries {
		envs[i] = e.Env
	}
	st, err := campaign.FoldPrefix(envs)
	if err != nil {
		t.Fatalf("log does not fold: %v%s", err, qarlDump(entries))
	}
	return st
}

// qarlLead is the entries no event caused at the head of entries.
func qarlLead(entries []campaign.CharacterEntry) []campaign.CharacterEntry {
	n := 0
	for n < len(entries) && entries[n].WorldSeq == 0 {
		n++
	}
	return entries[:n]
}

func qarlIndices(entries []campaign.CharacterEntry, kind string) []int {
	var out []int
	for i, e := range entries {
		if qarlKind(e.Env) == kind {
			out = append(out, i)
		}
	}
	return out
}

func qarlFoldNames(t *testing.T, st *engine.State, want *vttv1.RulesetNamed, what string) {
	t.Helper()
	got := [3]string{st.RulesetID, st.RulesetVersion, st.RulesetName}
	exp := [3]string{want.GetRulesetId(), want.GetVersion(), want.GetName()}
	if got != exp {
		t.Errorf("%s names ruleset %q, want %q", what, got, exp)
	}
}

func qarlRules(id, version, name string) *vttv1.RulesetNamed {
	return &vttv1.RulesetNamed{RulesetId: id, Version: version, Name: name}
}

func TestQARLAnOpeningNamesTheWorldsLastRulesetRightAfterItsSession(t *testing.T) {
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	w.append(w.sceneCreated("sc2"))
	w.append(w.noteUpserted("n1"))
	mod, err := structpb.NewStruct(map[string]any{"k": "v"})
	if err != nil {
		t.Fatal(err)
	}
	w.append(w.actorAdded(&vttv1.Actor{
		ActorId: "a2", Name: "Name a2", ModuleId: "qa-module", Kind: qarlNonParty,
		Attributes: map[string]int32{"score": 3},
		Resources:  map[string]*vttv1.Resource{"pool": {Current: 4, Max: 5}},
		ModuleData: mod,
	}))
	w.append(w.tokenPlaced("tok-a2", "sc2", "a2", 2, 2))
	w.makeParty("a2")
	w.append(w.conditionApplied("a2", "marked"))
	w.append(w.rulesetNamed(qarlRules("qa-old-rules", "1.0", "Old Rules")))
	last := qarlRules("qa-rules", "2.0", "Plain Rules")
	w.append(w.rulesetNamed(last))
	w.addActor("a1", "sc1", 1, 1)
	g := w.makeParty("a1")

	entries := w.log("a1")
	open := qarlLead(entries)
	if len(open) < 3 || len(open) == len(entries) {
		t.Fatalf("want an opening of a session, a ruleset and a party, then frames:%s", qarlDump(entries))
	}
	// VTT-981: nothing forwarded from an earlier event.
	for i, e := range entries[len(open):] {
		if e.WorldSeq != g {
			t.Errorf("entry %d after the opening has world seq %d, want %d", len(open)+i, e.WorldSeq, g)
		}
	}
	// VTT-981: the open session comes first.
	sess := w.openSession()
	if ss := open[0].Env.GetSessionStarted(); ss == nil || open[0].Env.GetSessionId() != sess.ID || ss.GetName() != sess.Name {
		t.Errorf("opening's first entry is not the open session's SessionStarted (%q %q):%s", sess.ID, sess.Name, qarlDump(entries))
	}
	// VTT-981: the RulesetNamed sits right after the SessionStarted and names
	// the world's last ruleset.
	if !proto.Equal(open[1].Env.GetRulesetNamed(), last) {
		t.Errorf("opening's second entry is not a RulesetNamed of %v:%s", last, qarlDump(entries))
	}
	// VTT-981: one RulesetNamed in the whole log.
	if got := qarlIndices(entries, "RulesetNamed"); len(got) != 1 {
		t.Errorf("log holds RulesetNamed at %v, want exactly one:%s", got, qarlDump(entries))
	}
	// VTT-981: before the party.
	adds := qarlIndices(open, "ActorAdded")
	if len(adds) == 0 {
		t.Fatalf("opening introduces no party member:%s", qarlDump(entries))
	}
	if adds[0] < 2 {
		t.Errorf("a party member is introduced at %d, before the ruleset:%s", adds[0], qarlDump(entries))
	}
	// SPEC-021, no row: an entry no event caused carries no event id, session
	// id or time.
	if r := open[1].Env; r.GetEventId() != "" || r.GetSessionId() != "" || r.GetOccurredAt() != nil {
		t.Errorf("opening's RulesetNamed carries %q %q %v, want none", r.GetEventId(), r.GetSessionId(), r.GetOccurredAt())
	}
	// VTT-981: the log's own fold names the world's last ruleset.
	qarlFoldNames(t, qarlFold(t, open), last, "the opening's fold")
	qarlFoldNames(t, qarlFold(t, entries), last, "the log's fold")

	// VTT-658: a party member its eyes do not see, without its sheet.
	st := qarlFold(t, open)
	a2 := st.Actors["a2"]
	if a2 == nil {
		t.Fatalf("opening does not introduce a2:%s", qarlDump(entries))
	}
	if a2.GetName() != "Name a2" || a2.GetModuleId() != "qa-module" || a2.GetKind() != qarlParty {
		t.Errorf("a2 introduced as %v, want its name, module id and kind", a2)
	}
	if fmt.Sprint(a2.GetControllerIds()) != "[p-a2]" {
		t.Errorf("a2's controllers = %v, want [p-a2]", a2.GetControllerIds())
	}
	if len(a2.GetAttributes()) != 0 || len(a2.GetResources()) != 0 || a2.GetModuleData() != nil || len(st.Conditions["a2"]) != 0 {
		t.Errorf("a2 introduced with its sheet: %v conditions %v", a2, st.Conditions["a2"])
	}
}

func TestQARLAnOpeningWhileTheWorldNamesNoRulesetHoldsNone(t *testing.T) {
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	w.addActor("a2", "sc1", 4, 4)
	w.makeParty("a2")
	w.addActor("a1", "sc1", 1, 1)
	w.makeParty("a1")

	entries := w.log("a1")
	open := qarlLead(entries)
	// VTT-981: "when it names one".
	if got := qarlIndices(entries, "RulesetNamed"); len(got) != 0 {
		t.Errorf("log holds RulesetNamed at %v, want none:%s", got, qarlDump(entries))
	}
	if len(open) < 2 || open[0].Env.GetSessionStarted() == nil || open[1].Env.GetActorAdded() == nil {
		t.Errorf("want the session then the party right after it:%s", qarlDump(entries))
	}
	qarlFoldNames(t, qarlFold(t, entries), &vttv1.RulesetNamed{}, "the log's fold")
}

func TestQARLAnOpeningWithNoOpenSessionPutsTheRulesetBeforeTheParty(t *testing.T) {
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	w.addActor("a2", "sc1", 4, 4)
	w.makeParty("a2")
	w.append(w.sessionEnded())
	r := qarlRules("qa-rules", "1.0", "Plain Rules")
	w.append(w.rulesetNamed(r))
	w.addActor("a1", "sc1", 1, 1)
	w.makeParty("a1")

	entries := w.log("a1")
	open := qarlLead(entries)
	// VTT-981 and the placement: no session to follow, still before the party.
	if len(open) < 2 || !proto.Equal(open[0].Env.GetRulesetNamed(), r) || open[1].Env.GetActorAdded() == nil {
		t.Errorf("want the RulesetNamed first and the party after it:%s", qarlDump(entries))
	}
	if got := qarlIndices(open, "SessionStarted"); len(got) != 0 {
		t.Errorf("opening starts a session though none is open:%s", qarlDump(entries))
	}
	qarlFoldNames(t, qarlFold(t, entries), r, "the log's fold")
}

func TestQARLEveryActiveLogIsSentARulesetNamedWhateverItsEyesSee(t *testing.T) {
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	w.append(w.sceneCreated("sc2"))
	w.addActor("a1", "sc1", 1, 1)
	w.makeParty("a1")
	w.addActor("a2", "sc2", 1, 1)
	w.makeParty("a2")
	w.addActor("a3", "", 0, 0)
	w.makeParty("a3")
	w.addActor("a4", "sc1", 3, 3)
	w.addActor("a5", "sc1", 4, 4)
	w.makeParty("a5")
	w.pause("a5")
	paused := len(w.log("a5"))

	rules := []*vttv1.RulesetNamed{
		qarlRules("qa-rules", "1.0", "Plain Rules"),
		qarlRules("qa-other", "1.0", "Other Rules"),
	}
	for _, r := range rules {
		e := w.rulesetNamed(r)
		seq := w.append(e)
		for _, id := range []string{"a1", "a2", "a3"} {
			entries := w.log(id)
			var sent []campaign.CharacterEntry
			for _, en := range entries {
				if en.WorldSeq == seq && en.Env.GetRulesetNamed() != nil {
					sent = append(sent, en)
				}
			}
			// VTT-980: sent it, whatever its eyes see.
			if len(sent) != 1 || !proto.Equal(sent[0].Env.GetRulesetNamed(), r) || sent[0].Env.GetEventId() != e.GetEventId() {
				t.Errorf("%s at %d: want one RulesetNamed %v of event %s:%s", id, seq, r, e.GetEventId(), qarlDump(entries))
			}
			qarlFoldNames(t, qarlFold(t, entries), r, id+"'s fold")
		}
	}
	// SPEC-021 (paused) and VTT-316: nothing to a paused log or a non-party actor.
	if got := len(w.log("a5")); got != paused {
		t.Errorf("paused a5's log grew from %d to %d:%s", paused, got, qarlDump(w.log("a5")))
	}
	if got := w.log("a4"); len(got) != 0 {
		t.Errorf("non-party a4 has a log:%s", qarlDump(got))
	}
	for _, id := range []string{"a1", "a2", "a3"} {
		if got := qarlIndices(w.log(id), "RulesetNamed"); len(got) != len(rules) {
			t.Errorf("%s holds RulesetNamed at %v, want %d", id, got, len(rules))
		}
	}
}

type qarlResumeCase struct {
	name        string
	atOpening   *vttv1.RulesetNamed
	whileActive *vttv1.RulesetNamed
	duringPause []*vttv1.RulesetNamed
	session     string // "same", "new" or "ended" across the pause
	byRemoval   bool
	want        *vttv1.RulesetNamed
}

func TestQARLAPausedLogResumesToTheWorldsRuleset(t *testing.T) {
	r1 := qarlRules("qa-rules", "1.0", "Plain Rules")
	r2 := qarlRules("qa-rules", "2.0", "Plain Rules")
	cases := []qarlResumeCase{
		{name: "another version", atOpening: r1, duringPause: []*vttv1.RulesetNamed{r2}, session: "new", want: r2},
		{name: "another id at the same version", atOpening: r1,
			duringPause: []*vttv1.RulesetNamed{qarlRules("qa-other", "1.0", "Other Rules")}, session: "new",
			want: qarlRules("qa-other", "1.0", "Other Rules")},
		{name: "nothing changed", atOpening: r1, session: "new"},
		{name: "same id and version under another name", atOpening: r1,
			duringPause: []*vttv1.RulesetNamed{qarlRules("qa-rules", "1.0", "Renamed Rules")}, session: "new"},
		{name: "the log names none and the world one", duringPause: []*vttv1.RulesetNamed{r1}, session: "new", want: r1},
		{name: "neither names one", session: "new"},
		{name: "named away and back", atOpening: r1,
			duringPause: []*vttv1.RulesetNamed{qarlRules("qa-rules", "2.0", "Two Rules"), r1}, session: "new"},
		{name: "named twice, the last counts", atOpening: r1,
			duringPause: []*vttv1.RulesetNamed{qarlRules("qa-rules", "2.0", "Two Rules"), qarlRules("qa-rules", "3.0", "Three Rules")},
			session:     "new", want: qarlRules("qa-rules", "3.0", "Three Rules")},
		{name: "a forwarded one, world unchanged", whileActive: r1, session: "new"},
		{name: "a forwarded one, world back to the opening's", atOpening: r1, whileActive: r2,
			duringPause: []*vttv1.RulesetNamed{r1}, session: "new", want: r1},
		{name: "no session change", atOpening: r1, duringPause: []*vttv1.RulesetNamed{r2}, session: "same", want: r2},
		{name: "no session open at the resume", atOpening: r1, duringPause: []*vttv1.RulesetNamed{r2}, session: "ended", want: r2},
		{name: "paused by removal", atOpening: r1, duringPause: []*vttv1.RulesetNamed{r2}, session: "new", byRemoval: true, want: r2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { qarlResume(t, tc) })
	}
}

func qarlResume(t *testing.T, tc qarlResumeCase) {
	t.Helper()
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	w.addActor("a2", "sc1", 4, 4)
	w.makeParty("a2")
	if tc.atOpening != nil {
		w.append(w.rulesetNamed(tc.atOpening))
	}
	w.addActor("a1", "sc1", 1, 1)
	w.makeParty("a1")
	if tc.whileActive != nil {
		w.append(w.rulesetNamed(tc.whileActive))
	}
	if tc.byRemoval {
		w.append(w.tokenRemoved("tok-a1"), w.actorRemoved("a1"))
	} else {
		w.pause("a1")
	}
	before := w.log("a1")
	held := qarlFold(t, before)
	switch tc.session {
	case "new":
		w.append(w.sessionEnded())
		w.append(w.sessionStarted("s2", "Second Evening"))
	case "ended":
		w.append(w.sessionEnded())
	}
	for _, r := range tc.duringPause {
		w.append(w.rulesetNamed(r))
	}
	if got := w.log("a1"); len(got) != len(before) {
		t.Fatalf("paused log grew from %d to %d:%s", len(before), len(got), qarlDump(got))
	}
	var rs int64
	if tc.byRemoval {
		rs = w.append(w.actorAdded(&vttv1.Actor{ActorId: "a1", Name: "Name a1", ModuleId: "qa-module", Kind: qarlParty}))
	} else {
		rs = w.makeParty("a1")
	}

	entries := w.log("a1")
	after := entries[len(before):]
	resume := qarlLead(after)
	if len(resume) == 0 || len(resume) == len(after) {
		t.Fatalf("want resume entries then the resuming event's frames:%s", qarlDump(after))
	}
	for i, e := range after[len(resume):] {
		if e.WorldSeq != rs {
			t.Errorf("entry %d after the resume entries has world seq %d, want %d", i, e.WorldSeq, rs)
		}
	}
	// VTT-982: no second opening; only session, ruleset and hiding entries.
	sessionAt, hiddenAt := []int{}, []int{}
	hidden := map[string]bool{}
	for i, e := range resume {
		switch {
		case e.Env.GetSessionStarted() != nil || e.Env.GetSessionEnded() != nil:
			sessionAt = append(sessionAt, i)
		case e.Env.GetTokenHidden() != nil:
			hiddenAt = append(hiddenAt, i)
			hidden[e.Env.GetTokenHidden().GetTokenId()] = true
		case e.Env.GetRulesetNamed() != nil:
		default:
			t.Errorf("resume entry %d is a %s:%s", i, qarlKind(e.Env), qarlDump(after))
		}
	}
	// VTT-982: every token it holds is hidden.
	var heldIDs, hiddenIDs []string
	for id := range held.Tokens {
		heldIDs = append(heldIDs, id)
	}
	for id := range hidden {
		hiddenIDs = append(hiddenIDs, id)
	}
	sort.Strings(heldIDs)
	sort.Strings(hiddenIDs)
	// A removal's own frames hide every token first, so that log holds none.
	if len(heldIDs) == 0 && !tc.byRemoval || fmt.Sprint(heldIDs) != fmt.Sprint(hiddenIDs) {
		t.Errorf("resume hides %v, the log holds %v", hiddenIDs, heldIDs)
	}
	if tc.session == "same" && len(sessionAt) != 0 || tc.session != "same" && len(sessionAt) == 0 {
		t.Errorf("session entries at %v for a %q session:%s", sessionAt, tc.session, qarlDump(after))
	}

	named := qarlIndices(resume, "RulesetNamed")
	// VTT-982: exactly the changed id or version brings a RulesetNamed, and no
	// frame does.
	if got := qarlIndices(after, "RulesetNamed"); len(got) != len(named) {
		t.Errorf("a RulesetNamed outside the resume entries:%s", qarlDump(after))
	}
	if tc.want == nil {
		if len(named) != 0 {
			t.Errorf("resume names a ruleset though the log's matches the world's:%s", qarlDump(after))
		}
		want := &vttv1.RulesetNamed{RulesetId: held.RulesetID, Version: held.RulesetVersion, Name: held.RulesetName}
		qarlFoldNames(t, qarlFold(t, entries), want, "the resumed log's fold")
		return
	}
	if len(named) != 1 || !proto.Equal(resume[named[0]].Env.GetRulesetNamed(), tc.want) {
		t.Fatalf("want one RulesetNamed of %v in the resume:%s", tc.want, qarlDump(after))
	}
	// The placement: after the session entries, before the TokenHidden entries.
	for _, i := range sessionAt {
		if i > named[0] {
			t.Errorf("session entry %d after the RulesetNamed at %d:%s", i, named[0], qarlDump(after))
		}
	}
	for _, i := range hiddenAt {
		if i < named[0] {
			t.Errorf("TokenHidden %d before the RulesetNamed at %d:%s", i, named[0], qarlDump(after))
		}
	}
	if tc.session == "same" && named[0] != 0 {
		t.Errorf("with no session entries the RulesetNamed is at %d, want 0:%s", named[0], qarlDump(after))
	}
	qarlFoldNames(t, qarlFold(t, entries), tc.want, "the resumed log's fold")
}

func TestQARLABatchNamingARulesetThenGrantingOpensTheLogWithIt(t *testing.T) {
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	w.addActor("a2", "sc1", 4, 4)
	w.makeParty("a2")
	w.addActor("a1", "sc1", 1, 1)
	r := qarlRules("qa-rules", "1.0", "Plain Rules")
	eg := w.granted("a1", "p-a1", qarlParty)
	w.append(w.rulesetNamed(r), eg)
	g := w.seqOf(eg.GetEventId())

	entries := w.log("a1")
	open := qarlLead(entries)
	// VTT-981: the world before the grant names it.
	if len(open) < 2 || open[0].Env.GetSessionStarted() == nil || !proto.Equal(open[1].Env.GetRulesetNamed(), r) {
		t.Errorf("want the session then the RulesetNamed:%s", qarlDump(entries))
	}
	if got := qarlIndices(entries, "RulesetNamed"); len(got) != 1 {
		t.Errorf("log holds RulesetNamed at %v, want one:%s", got, qarlDump(entries))
	}
	for _, e := range entries[len(open):] {
		if e.WorldSeq != g {
			t.Errorf("entry at world seq %d, want only %d after the opening:%s", e.WorldSeq, g, qarlDump(entries))
			break
		}
	}
}

func TestQARLABatchGrantingThenNamingARulesetSendsItAfterTheOpening(t *testing.T) {
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	w.addActor("a1", "sc1", 1, 1)
	r := qarlRules("qa-rules", "1.0", "Plain Rules")
	er := w.rulesetNamed(r)
	w.append(w.granted("a1", "p-a1", qarlParty), er)
	rseq := w.seqOf(er.GetEventId())

	entries := w.log("a1")
	open := qarlLead(entries)
	// VTT-981: the world before the grant names none.
	if got := qarlIndices(open, "RulesetNamed"); len(got) != 0 {
		t.Errorf("opening names a ruleset the world named after the grant:%s", qarlDump(entries))
	}
	// VTT-980: the now-active log is sent the batch's RulesetNamed.
	named := qarlIndices(entries, "RulesetNamed")
	if len(named) != 1 || entries[named[0]].WorldSeq != rseq || !proto.Equal(entries[named[0]].Env.GetRulesetNamed(), r) {
		t.Errorf("want one RulesetNamed %v at world seq %d:%s", r, rseq, qarlDump(entries))
	}
	qarlFoldNames(t, qarlFold(t, entries), r, "the log's fold")
}

func TestQARLABatchResumingAfterNamingARulesetResumesWithIt(t *testing.T) {
	r1 := qarlRules("qa-rules", "1.0", "Plain Rules")
	r2 := qarlRules("qa-rules", "2.0", "Plain Rules")
	for _, rulesetFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset first %v", rulesetFirst), func(t *testing.T) {
			w := qarlNew(t)
			w.append(w.sessionStarted("s1", "First Evening"))
			w.append(w.sceneCreated("sc1"))
			w.append(w.rulesetNamed(r1))
			w.addActor("a1", "sc1", 1, 1)
			w.makeParty("a1")
			w.pause("a1")
			before := len(w.log("a1"))
			er, eg := w.rulesetNamed(r2), w.granted("a1", "p-a1", qarlParty)
			if rulesetFirst {
				w.append(er, eg)
			} else {
				w.append(eg, er)
			}
			rseq := w.seqOf(er.GetEventId())
			entries := w.log("a1")
			after := entries[before:]
			resume := qarlLead(after)
			named := qarlIndices(after, "RulesetNamed")
			if len(named) != 1 || !proto.Equal(after[named[0]].Env.GetRulesetNamed(), r2) {
				t.Fatalf("want one RulesetNamed %v after the pause:%s", r2, qarlDump(after))
			}
			if rulesetFirst {
				// VTT-982: the world before the resuming grant names r2.
				if named[0] >= len(resume) {
					t.Errorf("RulesetNamed is not a resume entry:%s", qarlDump(after))
				}
			} else if after[named[0]].WorldSeq != rseq {
				// VTT-982 and VTT-980: resumed on r1, then sent r2 at its own event.
				t.Errorf("RulesetNamed at world seq %d, want %d:%s", after[named[0]].WorldSeq, rseq, qarlDump(after))
			}
			qarlFoldNames(t, qarlFold(t, entries), r2, "the log's fold")
		})
	}
}

func TestQARLAReopenedCampaignWritesNoSecondRulesetNamed(t *testing.T) {
	w := qarlNew(t)
	w.append(w.sessionStarted("s1", "First Evening"))
	w.append(w.sceneCreated("sc1"))
	r1 := qarlRules("qa-rules", "1.0", "Plain Rules")
	r2 := qarlRules("qa-rules", "2.0", "Plain Rules")
	w.append(w.rulesetNamed(r1))
	for i, id := range []string{"a1", "a2", "a3", "a4"} {
		w.addActor(id, "sc1", int32(i), 0)
		w.makeParty(id)
	}
	w.pause("a2")
	w.pause("a3")
	w.append(w.rulesetNamed(r2))
	w.makeParty("a2")
	w.pause("a4")

	ids := []string{"a1", "a2", "a3", "a4"}
	lens := map[string]int{}
	for _, id := range ids {
		lens[id] = len(w.log(id))
	}
	countAll := func(id string) int { return len(qarlIndices(w.log(id), "RulesetNamed")) }
	wantCounts := map[string]int{"a1": 2, "a2": 2, "a3": 1, "a4": 2}
	for id, n := range wantCounts {
		if got := countAll(id); got != n {
			t.Fatalf("%s holds %d RulesetNamed before the reopen, want %d:%s", id, got, n, qarlDump(w.log(id)))
		}
	}

	w.reopen()
	for _, id := range ids {
		if got := len(w.log(id)); got != lens[id] {
			t.Errorf("reopen changed %s's log from %d to %d entries", id, lens[id], got)
		}
	}
	// VTT-981, VTT-982: an active log is not opened or resumed again.
	n := w.append(w.narration("qa line"))
	for _, id := range []string{"a1", "a2"} {
		after := w.log(id)[lens[id]:]
		if len(after) == 0 || after[len(after)-1].WorldSeq != n {
			t.Errorf("%s was not written the narration:%s", id, qarlDump(after))
		}
		if got := qarlIndices(after, "RulesetNamed"); len(got) != 0 {
			t.Errorf("%s written a RulesetNamed after the reopen:%s", id, qarlDump(after))
		}
	}
	// VTT-982: a3 paused across the reopen, the world moved on; a4 paused, unchanged.
	w.makeParty("a3")
	w.makeParty("a4")
	a3 := w.log("a3")[lens["a3"]:]
	if named := qarlIndices(qarlLead(a3), "RulesetNamed"); len(named) != 1 || !proto.Equal(a3[named[0]].Env.GetRulesetNamed(), r2) {
		t.Errorf("a3's resume after the reopen: want one RulesetNamed %v:%s", r2, qarlDump(a3))
	}
	if got := qarlIndices(w.log("a4")[lens["a4"]:], "RulesetNamed"); len(got) != 0 {
		t.Errorf("a4's resume after the reopen names a ruleset its fold already names:%s", qarlDump(w.log("a4")[lens["a4"]:]))
	}
	// VTT-981: a log opened after the reopen opens once with the world's ruleset.
	w.addActor("a5", "sc1", 4, 4)
	w.makeParty("a5")
	a5 := w.log("a5")
	if open := qarlLead(a5); len(open) < 2 || !proto.Equal(open[1].Env.GetRulesetNamed(), r2) {
		t.Errorf("a5's opening: want the session then RulesetNamed %v:%s", r2, qarlDump(a5))
	}

	w.reopen()
	w.append(w.narration("qa line two"))
	ids = append(ids, "a5")
	wantCounts["a3"] = 2
	wantCounts["a5"] = 1
	for _, id := range ids {
		entries := w.log(id)
		if got := len(qarlIndices(entries, "RulesetNamed")); got != wantCounts[id] {
			t.Errorf("%s holds %d RulesetNamed after two reopens, want %d:%s", id, got, wantCounts[id], qarlDump(entries))
		}
		qarlFoldNames(t, qarlFold(t, entries), r2, id+"'s fold")
	}
}
