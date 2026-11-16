package perceive_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const (
	qaRCScene    = "scn-qarc"
	qaRCHero     = "act-qarc-hero"
	qaRCMate     = "act-qarc-mate"
	qaRCBeast    = "act-qarc-beast"
	qaRCShade    = "act-qarc-shade"
	qaRCHeroTok  = "tok-qarc-hero"
	qaRCMateTok  = "tok-qarc-mate"
	qaRCBeastTok = "tok-qarc-beast"
	qaRCShadeTok = "tok-qarc-shade"
	qaRCParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaRCNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qaRCLog struct {
	t      *testing.T
	eyes   perceive.Eyes
	world  *engine.State
	fold   *engine.State
	pr     *perceive.Projection
	seq    *int64
	logSeq int64
	sent   []*vttv1.Envelope
}

func qaRCNewLog(t *testing.T, eyes perceive.Eyes) *qaRCLog {
	t.Helper()
	var seq int64
	return &qaRCLog{
		t: t, eyes: eyes, world: engine.NewState(),
		fold: engine.NewState(), pr: perceive.New(eyes, nil), seq: &seq,
	}
}

func (l *qaRCLog) append(env *vttv1.Envelope) {
	l.t.Helper()
	*l.seq++
	env.Sequence = *l.seq
	env.EventId = fmt.Sprintf("evt-qarc-%d", *l.seq)
	env.SessionId = "sess-qarc"
	env.ActorRole = "dm"
	env.ParticipantId = "p-dm"
	if err := engine.Apply(l.world, env); err != nil {
		l.t.Fatalf("the world refused %s: %v", qaRCDescribe(env), err)
	}
}

func (l *qaRCLog) take(frames []*vttv1.Envelope) {
	l.t.Helper()
	for _, f := range frames {
		l.logSeq++
		entry := proto.Clone(f).(*vttv1.Envelope)
		entry.Sequence = l.logSeq
		if err := engine.Apply(l.fold, entry); err != nil {
			l.t.Fatalf("frame %s did not fold: %v", qaRCDescribe(f), err)
		}
		l.sent = append(l.sent, f)
	}
}

func (l *qaRCLog) emit(env *vttv1.Envelope) []*vttv1.Envelope {
	l.t.Helper()
	l.append(env)
	frames := l.pr.Event(l.eyes, env, l.fold, l.world)
	l.take(frames)
	return frames
}

func (l *qaRCLog) drop(env *vttv1.Envelope) {
	l.t.Helper()
	l.append(env)
}

func (l *qaRCLog) resume() {
	l.pr = perceive.Resume(l.eyes, l.fold, l.world)
}

func (l *qaRCLog) openFor(eyes perceive.Eyes) *qaRCLog {
	l.t.Helper()
	o := &qaRCLog{
		t: l.t, eyes: eyes, world: l.world, fold: engine.NewState(),
		seq: l.seq,
	}
	o.take([]*vttv1.Envelope{{
		SessionId: "sess-qarc",
		Payload: &vttv1.Envelope_SessionStarted{
			SessionStarted: &vttv1.SessionStarted{Name: "Night"}},
	}})
	o.take(perceive.New(perceive.Eyes{}, nil).
		Perch(perceive.Eyes{}, o.fold, o.world, 0))
	o.pr = perceive.New(eyes, nil)
	return o
}

func (l *qaRCLog) enter(a *vttv1.Actor, tok string, x int32) {
	l.t.Helper()
	l.emit(qaRCAdd(a))
	if a.GetKind() == qaRCParty {
		l.emit(qaRCGrant(a.GetActorId(), "p-2", qaRCParty))
	}
	l.emit(qaRCPlace(tok, a.GetActorId(), x))
}

func (l *qaRCLog) onBoard(tok string) bool {
	_, ok := l.fold.Tokens[tok]
	return ok
}

func qaRCTable(t *testing.T, open bool) *qaRCLog {
	t.Helper()
	l := qaRCNewLog(t, perceive.Eyes{Shoulder: qaRCHero})
	l.emit(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{
		SessionStarted: &vttv1.SessionStarted{Name: "Night"}}})
	l.emit(qaRCRooms())
	l.emit(qaRCAdd(&vttv1.Actor{
		ActorId: qaRCHero, Name: "Hero", Kind: qaRCParty,
		Resources: map[string]*vttv1.Resource{"nerve": {Current: 1, Max: 1}},
	}))
	l.emit(qaRCGrant(qaRCHero, "p-1", qaRCParty))
	l.emit(qaRCPlace(qaRCHeroTok, qaRCHero, 1))
	if open {
		l.emit(qaRCDoor(true))
	}
	return l
}

func qaRCMember(id, name string, attrs map[string]int32,
	res map[string]*vttv1.Resource) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: id, Name: name, Kind: qaRCParty, Attributes: attrs,
		Resources: res,
	}
}

func qaRCRooms() *vttv1.Envelope {
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 7; x++ {
		for y := 0; y < 3; y++ {
			kind := "floor"
			switch {
			case x == 3 && y == 1:
				kind = "door"
			case x == 3:
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{
			SceneId: qaRCScene, Name: "Two rooms", GridWidth: 7,
			GridHeight: 3, Tiles: tiles,
		}}}
}

func qaRCDoor(open bool) *vttv1.Envelope {
	at := &vttv1.GridPosition{X: 3, Y: 1}
	if open {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{
			DoorOpened: &vttv1.DoorOpened{SceneId: qaRCScene, At: at}}}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{
		DoorClosed: &vttv1.DoorClosed{SceneId: qaRCScene, At: at}}}
}

func qaRCAdd(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
		ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qaRCPlace(tok, actor string, x int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{
		TokenPlaced: &vttv1.TokenPlaced{
			TokenId: tok, SceneId: qaRCScene, ActorId: actor,
			Position: &vttv1.GridPosition{X: x, Y: 1},
		}}}
}

func qaRCUnplace(tok string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{
		TokenRemoved: &vttv1.TokenRemoved{TokenId: tok}}}
}

func qaRCRemove(actor string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{
		ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}}
}

func qaRCGrant(actor, who string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actor, ParticipantId: who, Kind: kind,
		}}}
}

func qaRCResAdd(actor, name string, cur, top int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceAdded{
		ResourceAdded: &vttv1.ResourceAdded{
			ActorId: actor, Resource: name,
			Value: &vttv1.Resource{Current: cur, Max: top},
		}}}
}

func qaRCResRemove(actor, name string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceRemoved{
		ResourceRemoved: &vttv1.ResourceRemoved{ActorId: actor, Resource: name}}}
}

func qaRCResChange(actor, name string, delta, now int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{
		ResourceChanged: &vttv1.ResourceChanged{
			ActorId: actor, Resource: name, Delta: delta, NewValue: now,
			Reason: "qa cause",
		}}}
}

func qaRCCondition(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{
		ConditionApplied: &vttv1.ConditionApplied{
			ActorId: actor, ConditionId: cond, Source: "qa source",
		}}}
}

func qaRCGive(actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityGiven{
		AbilityGiven: &vttv1.AbilityGiven{ActorId: actor, AbilityId: ability}}}
}

func qaRCNarrate() *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{
		NarrationAdded: &vttv1.NarrationAdded{Text: "the lamp gutters"}}}
}

func qaRCSubject(f *vttv1.Envelope) string {
	switch p := f.GetPayload().(type) {
	case *vttv1.Envelope_ActorAdded:
		return p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorRemoved:
		return p.ActorRemoved.GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		return p.ActorControlGranted.GetActorId()
	case *vttv1.Envelope_ActorControlRevoked:
		return p.ActorControlRevoked.GetActorId()
	case *vttv1.Envelope_ResourceAdded:
		return p.ResourceAdded.GetActorId()
	case *vttv1.Envelope_ResourceRemoved:
		return p.ResourceRemoved.GetActorId()
	case *vttv1.Envelope_ResourceChanged:
		return p.ResourceChanged.GetActorId()
	case *vttv1.Envelope_ConditionApplied:
		return p.ConditionApplied.GetActorId()
	case *vttv1.Envelope_ConditionRemoved:
		return p.ConditionRemoved.GetActorId()
	case *vttv1.Envelope_AbilityGiven:
		return p.AbilityGiven.GetActorId()
	case *vttv1.Envelope_AbilityTaken:
		return p.AbilityTaken.GetActorId()
	}
	return ""
}

func qaRCDescribe(f *vttv1.Envelope) string {
	switch p := f.GetPayload().(type) {
	case *vttv1.Envelope_ActorAdded:
		a := p.ActorAdded.GetActor()
		return fmt.Sprintf("ActorAdded %s %q res[%s]", a.GetActorId(),
			a.GetName(), qaRCResString(a.GetResources()))
	case *vttv1.Envelope_ActorRemoved:
		return "ActorRemoved " + p.ActorRemoved.GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		g := p.ActorControlGranted
		return fmt.Sprintf("ActorControlGranted %s %s %s", g.GetActorId(),
			g.GetParticipantId(), g.GetKind())
	case *vttv1.Envelope_ResourceAdded:
		r := p.ResourceAdded
		return fmt.Sprintf("ResourceAdded %s %s %d/%d", r.GetActorId(),
			r.GetResource(), r.GetValue().GetCurrent(), r.GetValue().GetMax())
	case *vttv1.Envelope_ResourceRemoved:
		r := p.ResourceRemoved
		return fmt.Sprintf("ResourceRemoved %s %s", r.GetActorId(),
			r.GetResource())
	case *vttv1.Envelope_ResourceChanged:
		r := p.ResourceChanged
		return fmt.Sprintf("ResourceChanged %s %s %+d=%d", r.GetActorId(),
			r.GetResource(), r.GetDelta(), r.GetNewValue())
	case *vttv1.Envelope_ConditionApplied:
		c := p.ConditionApplied
		return fmt.Sprintf("ConditionApplied %s %s", c.GetActorId(),
			c.GetConditionId())
	case *vttv1.Envelope_AbilityGiven:
		a := p.AbilityGiven
		return fmt.Sprintf("AbilityGiven %s %s", a.GetActorId(), a.GetAbilityId())
	case *vttv1.Envelope_TokenHidden:
		return "TokenHidden " + p.TokenHidden.GetTokenId()
	case *vttv1.Envelope_TokenPlaced:
		return "TokenPlaced " + p.TokenPlaced.GetTokenId()
	}
	return fmt.Sprintf("%T", f.GetPayload())
}

func qaRCDescribeAll(frames []*vttv1.Envelope) []string {
	out := make([]string, 0, len(frames))
	for _, f := range frames {
		out = append(out, qaRCDescribe(f))
	}
	return out
}

func qaRCAbout(frames []*vttv1.Envelope, id string) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range frames {
		if qaRCSubject(f) == id {
			out = append(out, f)
		}
	}
	return out
}

func qaRCResString(m map[string]*vttv1.Resource) string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%d/%d", n, m[n].GetCurrent(),
			m[n].GetMax()))
	}
	return strings.Join(parts, " ")
}

func qaRCSameRes(a, b map[string]*vttv1.Resource) bool {
	if len(a) != len(b) {
		return false
	}
	for n, v := range a {
		w, ok := b[n]
		if !ok || v.GetCurrent() != w.GetCurrent() || v.GetMax() != w.GetMax() {
			return false
		}
	}
	return true
}

func qaRCSameData(a, b *structpb.Struct) bool {
	if len(a.GetFields()) == 0 && len(b.GetFields()) == 0 {
		return true
	}
	return proto.Equal(a, b)
}

func qaRCSameAttrs(a, b map[string]int32) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func qaRCWantFrames(t *testing.T, label string, got []*vttv1.Envelope,
	want []string) {
	t.Helper()
	g := strings.Join(qaRCDescribeAll(got), "\n  ")
	w := strings.Join(want, "\n  ")
	if g != w {
		t.Errorf("%s:\n got\n  %s\n want\n  %s", label, g, w)
	}
}

func qaRCSheetMatches(t *testing.T, l *qaRCLog, id string) {
	t.Helper()
	got, want := l.fold.Actors[id], l.world.Actors[id]
	if got == nil || want == nil {
		t.Fatalf("%s: log holds %v, world holds %v", id, got != nil,
			want != nil)
	}
	if got.GetName() != want.GetName() ||
		!qaRCSameAttrs(got.GetAttributes(), want.GetAttributes()) ||
		!qaRCSameRes(got.GetResources(), want.GetResources()) ||
		!qaRCSameData(got.GetModuleData(), want.GetModuleData()) {
		t.Errorf("%s: log holds %q %v res[%s] %v; world holds %q %v res[%s] %v",
			id, got.GetName(), got.GetAttributes(),
			qaRCResString(got.GetResources()), got.GetModuleData(),
			want.GetName(), want.GetAttributes(),
			qaRCResString(want.GetResources()), want.GetModuleData())
	}
}

func qaRCNoResources(t *testing.T, l *qaRCLog, id string) {
	t.Helper()
	if a := l.fold.Actors[id]; a == nil || len(a.GetResources()) != 0 {
		t.Errorf("%s: log holds %v with res[%s]; want it held without any",
			id, a != nil, qaRCResString(a.GetResources()))
	}
}

func qaRCCorrection(t *testing.T, f *vttv1.Envelope, seq int64) {
	t.Helper()
	if f.GetSequence() != seq || f.GetEventId() != "" ||
		f.GetSessionId() != "" || f.GetOccurredAt() != nil ||
		f.GetActorRole() != "" || f.GetParticipantId() != "" {
		t.Errorf("%s: envelope seq %d id %q session %q role %q who %q at %v;"+
			" want seq %d and nothing else", qaRCDescribe(f), f.GetSequence(),
			f.GetEventId(), f.GetSessionId(), f.GetActorRole(),
			f.GetParticipantId(), f.GetOccurredAt(), seq)
	}
	if f.GetResourceChanged().GetReason() != "" ||
		f.GetConditionApplied().GetSource() != "" {
		t.Errorf("%s carries a reason or a source", qaRCDescribe(f))
	}
}

func qaRCForwarded(t *testing.T, f, env *vttv1.Envelope) {
	t.Helper()
	want := proto.Clone(env).(*vttv1.Envelope)
	want.ParticipantId = ""
	want.ActorRole = ""
	if !proto.Equal(f, want) {
		t.Errorf("forwarded %v; want the event under its own id less its "+
			"issuer, %v", f, want)
	}
}

func qaRCNoReintro(t *testing.T, frames []*vttv1.Envelope, id string) {
	t.Helper()
	for _, f := range qaRCAbout(frames, id) {
		if f.GetActorRemoved() != nil || f.GetActorAdded() != nil {
			t.Errorf("sent %s; want no withdrawal or introduction of %s",
				qaRCDescribe(f), id)
		}
	}
}

func qaRCCarries(frames []*vttv1.Envelope, eventID string) bool {
	for _, f := range frames {
		if f.GetEventId() == eventID {
			return true
		}
	}
	return false
}

func qaRCIndex(frames []*vttv1.Envelope, desc string) int {
	for i, f := range frames {
		if strings.HasPrefix(qaRCDescribe(f), desc) {
			return i
		}
	}
	return -1
}

// VTT-779, VTT-781.
func TestQARCResourceAddedAndRemovedInSightAreForwardedAlone(t *testing.T) {
	l := qaRCTable(t, true)
	l.enter(qaRCMember(qaRCMate, "Mate", map[string]int32{"knack": 2},
		map[string]*vttv1.Resource{
			"focus": {Current: 1, Max: 3}, "pool": {Current: 2, Max: 2},
		}), qaRCMateTok, 5)
	if !l.onBoard(qaRCMateTok) {
		t.Fatal("fixture: the mate's token is not in the log's sight")
	}
	start := len(l.sent)
	for _, env := range []*vttv1.Envelope{
		qaRCResAdd(qaRCMate, "zeal", 0, 4),
		qaRCResRemove(qaRCMate, "focus"),
		qaRCResAdd(qaRCMate, "focus", 2, 2),
		qaRCResAdd(qaRCMate, "void", 0, 0),
		qaRCResRemove(qaRCMate, "pool"),
	} {
		frames := l.emit(env)
		if len(frames) != 1 {
			t.Fatalf("%s: sent %v; want the event alone", qaRCDescribe(env),
				qaRCDescribeAll(frames))
		}
		qaRCForwarded(t, frames[0], env)
		qaRCSheetMatches(t, l, qaRCMate)
	}
	l.emit(qaRCDoor(false))
	if l.onBoard(qaRCMateTok) {
		t.Fatal("fixture: the closed door leaves the mate in sight")
	}
	back := l.emit(qaRCDoor(true))
	qaRCWantFrames(t, "back in sight, nothing changed", qaRCAbout(back,
		qaRCMate), nil)
	l.emit(qaRCResChange(qaRCMate, "zeal", 3, 3))
	l.emit(qaRCNarrate())
	qaRCSheetMatches(t, l, qaRCMate)
	qaRCNoReintro(t, l.sent[start:], qaRCMate)
}

// VTT-780, VTT-796, VTT-781, VTT-793; VTT-779 for the frames out of sight.
func TestQARCChangesOutOfSightAreCorrectedInByteOrder(t *testing.T) {
	l := qaRCTable(t, true)
	l.enter(qaRCMember(qaRCMate, "Mate", map[string]int32{"knack": 2},
		map[string]*vttv1.Resource{
			"Zeal": {Current: 2, Max: 5}, "edge": {Current: 3, Max: 3},
			"focus": {Current: 1, Max: 3}, "kin": {Current: 2, Max: 2},
			"mist": {Current: 0, Max: 0}, "nerve": {Current: 2, Max: 4},
			"ward": {Current: 1, Max: 1},
		}), qaRCMateTok, 5)
	l.emit(qaRCDoor(false))
	if l.onBoard(qaRCMateTok) {
		t.Fatal("fixture: the closed door leaves the mate in sight")
	}
	start := len(l.sent)
	for _, env := range []*vttv1.Envelope{
		qaRCResAdd(qaRCMate, "pool", 0, 2),
		qaRCResRemove(qaRCMate, "focus"),
		qaRCResRemove(qaRCMate, "Zeal"),
		qaRCResAdd(qaRCMate, "Zeal", 3, 4),
		qaRCResChange(qaRCMate, "nerve", -1, 1),
		qaRCResRemove(qaRCMate, "edge"),
		qaRCResAdd(qaRCMate, "edge", 1, 3),
		qaRCResRemove(qaRCMate, "kin"),
		qaRCResAdd(qaRCMate, "kin", 2, 2),
		qaRCResRemove(qaRCMate, "mist"),
		qaRCResAdd(qaRCMate, "void", 0, 0),
		qaRCCondition(qaRCMate, "dazed"),
		qaRCGive(qaRCMate, "lunge"),
	} {
		frames := l.emit(env)
		if len(qaRCAbout(frames, qaRCMate)) != 0 ||
			qaRCCarries(frames, env.GetEventId()) {
			t.Errorf("out of sight, %s sent %v; want nothing of the mate",
				qaRCDescribe(env), qaRCDescribeAll(frames))
		}
	}
	open := qaRCDoor(true)
	frames := l.emit(open)
	about := qaRCAbout(frames, qaRCMate)
	qaRCWantFrames(t, "the correction at sight", about, []string{
		"ResourceRemoved act-qarc-mate Zeal",
		"ResourceAdded act-qarc-mate Zeal 3/4",
		"ResourceChanged act-qarc-mate edge -2=1",
		"ResourceRemoved act-qarc-mate focus",
		"ResourceRemoved act-qarc-mate mist",
		"ResourceChanged act-qarc-mate nerve -1=1",
		"ResourceAdded act-qarc-mate pool 0/2",
		"ResourceAdded act-qarc-mate void 0/0",
		"ConditionApplied act-qarc-mate dazed",
		"AbilityGiven act-qarc-mate lunge",
	})
	for _, f := range about {
		qaRCCorrection(t, f, open.GetSequence())
	}
	qaRCSheetMatches(t, l, qaRCMate)
	qaRCNoReintro(t, l.sent[start:], qaRCMate)
}

// VTT-781, VTT-784, VTT-780, VTT-793, VTT-658.
func TestQARCHeldUnseenDifferingOnlyInResourcesIsCorrected(t *testing.T) {
	l := qaRCTable(t, false)
	start := len(l.sent)
	l.enter(qaRCMember(qaRCMate, "Mate", nil, map[string]*vttv1.Resource{
		"focus": {Current: 1, Max: 3}, "Pool": {Current: 2, Max: 2},
		"nerve": {Current: 0, Max: 0},
	}), qaRCMateTok, 5)
	if l.onBoard(qaRCMateTok) {
		t.Fatal("fixture: the closed door leaves the mate in sight")
	}
	qaRCNoResources(t, l, qaRCMate)
	l.emit(qaRCCondition(qaRCMate, "dazed"))
	open := qaRCDoor(true)
	about := qaRCAbout(l.emit(open), qaRCMate)
	qaRCWantFrames(t, "the first sight", about, []string{
		"ResourceAdded act-qarc-mate Pool 2/2",
		"ResourceAdded act-qarc-mate focus 1/3",
		"ResourceAdded act-qarc-mate nerve 0/0",
		"ConditionApplied act-qarc-mate dazed",
		"ActorControlGranted act-qarc-mate p-2 ACTOR_KIND_PARTY_MEMBER",
	})
	for _, f := range about {
		qaRCCorrection(t, f, open.GetSequence())
	}
	qaRCSheetMatches(t, l, qaRCMate)
	if got := qaRCAbout(l.sent[start:], qaRCMate); len(got) < 1 ||
		got[0].GetActorAdded() == nil {
		t.Fatalf("fixture: the mate was not introduced unseen first: %v",
			qaRCDescribeAll(got))
	}
	qaRCNoReintro(t, qaRCAbout(l.sent[start:], qaRCMate)[1:], qaRCMate)
}

// VTT-784, VTT-664, VTT-793.
func TestQARCHeldUnseenWithASheetIsWithdrawnOnlyAtFirstSight(t *testing.T) {
	mood, err := structpb.NewStruct(map[string]any{"mood": "calm"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		attrs map[string]int32
		data  *structpb.Struct
	}{
		{"attributes", map[string]int32{"knack": 2}, nil},
		{"module data", nil, mood},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := qaRCTable(t, false)
			mate := qaRCMember(qaRCMate, "Mate", c.attrs,
				map[string]*vttv1.Resource{"focus": {Current: 1, Max: 3}})
			mate.ModuleData = c.data
			l.enter(mate, qaRCMateTok, 5)
			open := qaRCDoor(true)
			about := qaRCAbout(l.emit(open), qaRCMate)
			if len(about) < 2 || about[0].GetActorRemoved() == nil ||
				about[1].GetActorAdded() == nil {
				t.Fatalf("first sight sent %v; want a bare ActorRemoved, "+
					"then the ActorAdded", qaRCDescribeAll(about))
			}
			qaRCCorrection(t, about[0], open.GetSequence())
			if !proto.Equal(about[0].GetActorRemoved(),
				&vttv1.ActorRemoved{ActorId: qaRCMate}) {
				t.Errorf("ActorRemoved %v is not bare", about[0])
			}
			for _, f := range about {
				if strings.HasPrefix(qaRCDescribe(f), "Resource") {
					t.Errorf("re-introduction also sent %s", qaRCDescribe(f))
				}
			}
			qaRCSheetMatches(t, l, qaRCMate)
			l.emit(qaRCDoor(false))
			start := len(l.sent)
			l.emit(qaRCResAdd(qaRCMate, "zeal", 1, 1))
			l.emit(qaRCResRemove(qaRCMate, "focus"))
			again := qaRCAbout(l.emit(qaRCDoor(true)), qaRCMate)
			qaRCWantFrames(t, "the second sight", again, []string{
				"ResourceRemoved act-qarc-mate focus",
				"ResourceAdded act-qarc-mate zeal 1/1",
			})
			qaRCSheetMatches(t, l, qaRCMate)
			qaRCNoReintro(t, l.sent[start:], qaRCMate)
		})
	}
}

// VTT-783, VTT-781, VTT-780, VTT-793.
func TestQARCPartyMemberAddedAgainWithOtherResourcesIsCorrected(
	t *testing.T) {
	sheet := func(res map[string]*vttv1.Resource) *vttv1.Actor {
		return qaRCMember(qaRCMate, "Mate", map[string]int32{"knack": 2}, res)
	}
	want := []string{
		"ResourceRemoved act-qarc-mate focus",
		"ResourceAdded act-qarc-mate focus 1/5",
		"ResourceRemoved act-qarc-mate pool",
		"ResourceAdded act-qarc-mate zeal 0/1",
	}
	for _, inSight := range []bool{false, true} {
		t.Run(fmt.Sprintf("in sight %v", inSight), func(t *testing.T) {
			l := qaRCTable(t, true)
			l.enter(sheet(map[string]*vttv1.Resource{
				"focus": {Current: 1, Max: 3}, "pool": {Current: 2, Max: 2},
			}), qaRCMateTok, 5)
			if !inSight {
				l.emit(qaRCDoor(false))
			}
			start := len(l.sent)
			l.emit(qaRCUnplace(qaRCMateTok))
			l.emit(qaRCRemove(qaRCMate))
			l.emit(qaRCAdd(sheet(map[string]*vttv1.Resource{
				"focus": {Current: 1, Max: 5}, "zeal": {Current: 0, Max: 1},
			})))
			l.emit(qaRCGrant(qaRCMate, "p-2", qaRCParty))
			place := qaRCPlace(qaRCMateTok+"-2", qaRCMate, 5)
			frames := l.emit(place)
			if !inSight {
				if len(qaRCAbout(frames, qaRCMate)) != 0 {
					t.Errorf("out of sight the new token sent %v",
						qaRCDescribeAll(frames))
				}
				place = qaRCDoor(true)
				frames = l.emit(place)
			}
			about := qaRCAbout(frames, qaRCMate)
			qaRCWantFrames(t, "the correction", about, want)
			for _, f := range about {
				qaRCCorrection(t, f, place.GetSequence())
			}
			qaRCSheetMatches(t, l, qaRCMate)
			qaRCNoReintro(t, l.sent[start:], qaRCMate)
		})
	}
}

// VTT-783, VTT-658: another name is another actor, introduced at its add.
func TestQARCPartyMemberAddedAgainUnderAnotherNameIsIntroduced(
	t *testing.T) {
	l := qaRCTable(t, true)
	l.enter(qaRCMember(qaRCMate, "Mate", map[string]int32{"knack": 2},
		map[string]*vttv1.Resource{"focus": {Current: 1, Max: 3}}),
		qaRCMateTok, 5)
	l.emit(qaRCDoor(false))
	l.emit(qaRCUnplace(qaRCMateTok))
	l.emit(qaRCRemove(qaRCMate))
	about := qaRCAbout(l.emit(qaRCAdd(qaRCMember(qaRCMate, "Other",
		map[string]int32{"knack": 2},
		map[string]*vttv1.Resource{"focus": {Current: 1, Max: 3}}))), qaRCMate)
	if len(about) < 2 || about[0].GetActorRemoved() == nil ||
		qaRCDescribe(about[1]) != `ActorAdded act-qarc-mate "Other" res[]` {
		t.Errorf("the add under another name sent %v; want a bare "+
			"ActorRemoved, then Other without its sheet", qaRCDescribeAll(about))
	}
}

// VTT-784: held with resources and no attributes, re-added with attributes.
func TestQARCHeldWithResourcesOnlyIsWithdrawnAtItsFirstSight(t *testing.T) {
	for _, c := range []struct {
		name, newName, module string
		atAdd                 bool
	}{
		{"same name and module", "Mate", "", false},
		{"another name", "Other", "", true},
		{"another module", "Mate", "mod-other", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := qaRCTable(t, true)
			l.enter(qaRCMember(qaRCMate, "Mate", nil,
				map[string]*vttv1.Resource{"focus": {Current: 1, Max: 3}}),
				qaRCMateTok, 5)
			qaRCSheetMatches(t, l, qaRCMate)
			l.emit(qaRCDoor(false))
			start := len(l.sent)
			l.emit(qaRCUnplace(qaRCMateTok))
			l.emit(qaRCRemove(qaRCMate))
			again := qaRCMember(qaRCMate, c.newName, map[string]int32{"knack": 2},
				map[string]*vttv1.Resource{
					"focus": {Current: 1, Max: 3}, "zeal": {Current: 2, Max: 2},
				})
			again.ModuleId = c.module
			added := qaRCAbout(l.emit(qaRCAdd(again)), qaRCMate)
			removedAtAdd := len(added) > 0 && added[0].GetActorRemoved() != nil
			if removedAtAdd != c.atAdd {
				t.Errorf("the add sent %v; want a bare ActorRemoved %v",
					qaRCDescribeAll(added), c.atAdd)
			}
			l.emit(qaRCGrant(qaRCMate, "p-2", qaRCParty))
			l.emit(qaRCPlace(qaRCMateTok+"-2", qaRCMate, 5))
			if !c.atAdd {
				qaRCNoReintro(t, l.sent[start:], qaRCMate)
			}
			about := qaRCAbout(l.emit(qaRCDoor(true)), qaRCMate)
			if len(about) < 2 || about[0].GetActorRemoved() == nil ||
				about[1].GetActorAdded() == nil {
				t.Fatalf("first sight sent %v; want a bare ActorRemoved, "+
					"then the ActorAdded", qaRCDescribeAll(about))
			}
			qaRCSheetMatches(t, l, qaRCMate)
		})
	}
}

// VTT-779: a creature's resources reach no log, forwarded or corrected.
func TestQARCCreatureResourcesNeverReachTheLog(t *testing.T) {
	for _, openAtEntry := range []bool{true, false} {
		t.Run(fmt.Sprintf("seen at entry %v", openAtEntry), func(t *testing.T) {
			l := qaRCTable(t, openAtEntry)
			l.enter(&vttv1.Actor{
				ActorId: qaRCBeast, Name: "Beast", Kind: qaRCNonParty,
				Resources: map[string]*vttv1.Resource{
					"focus": {Current: 2, Max: 3}},
			}, qaRCBeastTok, 6)
			if !openAtEntry {
				l.emit(qaRCResAdd(qaRCBeast, "nerve", 1, 1))
				l.emit(qaRCDoor(true))
			}
			if !l.onBoard(qaRCBeastTok) {
				t.Fatal("fixture: the beast's token is not in sight")
			}
			qaRCNoResources(t, l, qaRCBeast)
			for _, env := range []*vttv1.Envelope{
				qaRCResAdd(qaRCBeast, "zeal", 1, 1),
				qaRCResChange(qaRCBeast, "focus", -1, 1),
				qaRCResRemove(qaRCBeast, "focus"),
				qaRCResAdd(qaRCBeast, "focus", 0, 4),
				qaRCDoor(false),
				qaRCResRemove(qaRCBeast, "zeal"),
				qaRCResAdd(qaRCBeast, "pool", 1, 2),
				qaRCResChange(qaRCBeast, "focus", 2, 2),
				qaRCDoor(true),
			} {
				frames := l.emit(env)
				for _, f := range qaRCAbout(frames, qaRCBeast) {
					if strings.HasPrefix(qaRCDescribe(f), "Resource") {
						t.Errorf("%s sent %s", qaRCDescribe(env), qaRCDescribe(f))
					}
				}
				qaRCNoResources(t, l, qaRCBeast)
			}
			if !l.onBoard(qaRCBeastTok) {
				t.Fatal("fixture: the beast did not come back into sight")
			}
		})
	}
}

// VTT-468, VTT-779: a party member whose sheet is resources alone.
func TestQARCKindChangeInSightOfAPartyMemberWithResources(t *testing.T) {
	l := qaRCTable(t, true)
	l.enter(qaRCMember(qaRCShade, "", nil,
		map[string]*vttv1.Resource{"focus": {Current: 1, Max: 2}}),
		qaRCShadeTok, 4)
	qaRCSheetMatches(t, l, qaRCShade)
	grant := qaRCGrant(qaRCShade, "p-2", qaRCNonParty)
	frames := l.emit(grant)
	hid := qaRCIndex(frames, "TokenHidden "+qaRCShadeTok)
	removed := qaRCIndex(frames, "ActorRemoved "+qaRCShade)
	added := qaRCIndex(frames, "ActorAdded "+qaRCShade)
	placed := qaRCIndex(frames, "TokenPlaced "+qaRCShadeTok)
	if hid < 0 || removed < hid || added < removed || placed < added ||
		frames[len(frames)-1].GetEventId() != grant.GetEventId() {
		t.Fatalf("the kind change sent %v; want TokenHidden, ActorRemoved, "+
			"ActorAdded, TokenPlaced, then the grant", qaRCDescribeAll(frames))
	}
	if len(frames[added].GetActorAdded().GetActor().GetResources()) != 0 {
		t.Errorf("the creature was introduced with %s", qaRCDescribe(frames[added]))
	}
	qaRCNoResources(t, l, qaRCShade)
	if k := l.fold.Actors[qaRCShade].GetKind(); k != qaRCNonParty {
		t.Errorf("log holds kind %s", k)
	}
	for _, f := range l.emit(qaRCResChange(qaRCShade, "focus", -1, 0)) {
		if strings.HasPrefix(qaRCDescribe(f), "Resource") {
			t.Errorf("the creature's change sent %s", qaRCDescribe(f))
		}
	}
}

// VTT-468, VTT-793: a creature whose sheet is resources alone.
func TestQARCKindChangeInSightOfACreatureWithResources(t *testing.T) {
	l := qaRCTable(t, true)
	l.enter(&vttv1.Actor{ActorId: qaRCShade, Kind: qaRCNonParty,
		Resources: map[string]*vttv1.Resource{"focus": {Current: 1, Max: 2}}},
		qaRCShadeTok, 4)
	qaRCNoResources(t, l, qaRCShade)
	frames := l.emit(qaRCGrant(qaRCShade, "p-2", qaRCParty))
	if qaRCIndex(frames, "ActorRemoved "+qaRCShade) < 0 ||
		qaRCIndex(frames, "ActorAdded "+qaRCShade) < 0 {
		t.Errorf("the kind change sent %v; want a withdrawal and an "+
			"introduction", qaRCDescribeAll(frames))
	}
	qaRCSheetMatches(t, l, qaRCShade)
	change := qaRCResChange(qaRCShade, "focus", -1, 0)
	if got := l.emit(change); len(got) != 1 ||
		got[0].GetEventId() != change.GetEventId() {
		t.Errorf("the party member's change sent %v; want it forwarded",
			qaRCDescribeAll(got))
	}
}

// VTT-481, VTT-779, VTT-793: both directions while the door is closed.
func TestQARCKindChangeOutOfSightOfAnActorWithResources(t *testing.T) {
	for _, c := range []struct {
		name     string
		from, to vttv1.ActorKind
	}{
		{"party member made a creature", qaRCParty, qaRCNonParty},
		{"creature made a party member", qaRCNonParty, qaRCParty},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := qaRCTable(t, true)
			l.enter(&vttv1.Actor{ActorId: qaRCShade, Kind: c.from,
				Resources: map[string]*vttv1.Resource{
					"focus": {Current: 1, Max: 2}},
			}, qaRCShadeTok, 4)
			l.emit(qaRCDoor(false))
			unseen := l.emit(qaRCGrant(qaRCShade, "p-2", c.to))
			qaRCNoReintro(t, unseen, qaRCShade)
			about := qaRCAbout(l.emit(qaRCDoor(true)), qaRCShade)
			if len(about) < 2 || about[0].GetActorRemoved() == nil ||
				about[1].GetActorAdded() == nil {
				t.Fatalf("next sight sent %v; want a withdrawal and an "+
					"introduction", qaRCDescribeAll(about))
			}
			if k := l.fold.Actors[qaRCShade].GetKind(); k != c.to {
				t.Errorf("log holds kind %s; want %s", k, c.to)
			}
			if c.to == qaRCParty {
				qaRCSheetMatches(t, l, qaRCShade)
			} else {
				qaRCNoResources(t, l, qaRCShade)
			}
		})
	}
}

// VTT-782.
func TestQARCResumeComparesThePartyInSightByResources(t *testing.T) {
	for _, c := range []struct {
		name    string
		closed  bool
		drop    []*vttv1.Envelope
		subject string
		resumes bool
		want    []string
	}{
		{name: "equal", subject: qaRCMate, resumes: true},
		{name: "lags a name", subject: qaRCMate,
			drop: []*vttv1.Envelope{qaRCResAdd(qaRCMate, "zeal", 1, 2)},
			want: []string{"ResourceAdded act-qarc-mate zeal 1/2"}},
		{name: "lags a name held at zero", subject: qaRCMate,
			drop: []*vttv1.Envelope{qaRCResAdd(qaRCMate, "void", 0, 0)},
			want: []string{"ResourceAdded act-qarc-mate void 0/0"}},
		{name: "holds a name the world removed", subject: qaRCMate,
			drop: []*vttv1.Envelope{qaRCResRemove(qaRCMate, "pool")},
			want: []string{"ResourceRemoved act-qarc-mate pool"}},
		{name: "lags a max", subject: qaRCMate,
			drop: []*vttv1.Envelope{qaRCResRemove(qaRCMate, "focus"),
				qaRCResAdd(qaRCMate, "focus", 1, 4)},
			want: []string{"ResourceRemoved act-qarc-mate focus",
				"ResourceAdded act-qarc-mate focus 1/4"}},
		{name: "lags a current", subject: qaRCMate,
			drop: []*vttv1.Envelope{qaRCResChange(qaRCMate, "pool", -1, 1)},
			want: []string{"ResourceChanged act-qarc-mate pool -1=1"}},
		{name: "the eye lags a current", subject: qaRCHero,
			drop: []*vttv1.Envelope{qaRCResChange(qaRCHero, "nerve", -1, 0)},
			want: []string{"ResourceChanged act-qarc-hero nerve -1=0"}},
		{name: "a creature in sight lags", subject: qaRCBeast, resumes: true,
			drop: []*vttv1.Envelope{qaRCResAdd(qaRCBeast, "zeal", 1, 1),
				qaRCResChange(qaRCBeast, "focus", -1, 1)}},
		{name: "a party member out of sight lags", subject: qaRCHero,
			closed: true, resumes: true,
			drop: []*vttv1.Envelope{qaRCResAdd(qaRCMate, "zeal", 1, 2)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			l := qaRCTable(t, true)
			l.enter(qaRCMember(qaRCMate, "Mate", map[string]int32{"knack": 2},
				map[string]*vttv1.Resource{
					"focus": {Current: 1, Max: 3}, "pool": {Current: 2, Max: 2},
				}), qaRCMateTok, 5)
			l.enter(&vttv1.Actor{ActorId: qaRCBeast, Name: "Beast",
				Kind: qaRCNonParty, Resources: map[string]*vttv1.Resource{
					"focus": {Current: 2, Max: 3}},
			}, qaRCBeastTok, 6)
			if c.closed {
				l.emit(qaRCDoor(false))
			}
			for _, env := range c.drop {
				l.drop(env)
			}
			l.resume()
			next := qaRCCondition(c.subject, "dazed")
			frames := l.emit(next)
			if got := qaRCCarries(frames, next.GetEventId()); got != c.resumes {
				t.Fatalf("the next event forwarded %v, want %v: %v", got,
					c.resumes, qaRCDescribeAll(frames))
			}
			if !c.resumes {
				want := append(append([]string{}, c.want...),
					"ConditionApplied "+c.subject+" dazed")
				qaRCWantFrames(t, "the correction after a fresh start",
					qaRCAbout(frames, c.subject), want)
				qaRCSheetMatches(t, l, c.subject)
				qaRCNoReintro(t, frames, c.subject)
			}
			if c.closed {
				back := qaRCAbout(l.emit(qaRCDoor(true)), qaRCMate)
				qaRCWantFrames(t, "the out-of-sight member at sight", back,
					[]string{"ResourceAdded act-qarc-mate zeal 1/2"})
				qaRCSheetMatches(t, l, qaRCMate)
			}
		})
	}
}

// VTT-658, VTT-781, VTT-784, VTT-793: a log opened as the fan-out opens one.
func TestQARCAnOpenedLogIsBroughtItsResourcesByCorrection(t *testing.T) {
	l := qaRCTable(t, true)
	l.enter(qaRCMember(qaRCMate, "Mate", nil, map[string]*vttv1.Resource{
		"focus": {Current: 1, Max: 3}, "Pool": {Current: 0, Max: 0},
	}), qaRCMateTok, 5)
	own := l.openFor(perceive.Eyes{Shoulder: qaRCMate})
	qaRCNoResources(t, own, qaRCMate)
	qaRCNoResources(t, own, qaRCHero)
	start := len(own.sent)
	frames := own.emit(qaRCNarrate())
	qaRCWantFrames(t, "its own character", qaRCAbout(frames, qaRCMate),
		[]string{
			"ResourceAdded act-qarc-mate Pool 0/0",
			"ResourceAdded act-qarc-mate focus 1/3",
		})
	qaRCWantFrames(t, "the hero it sees", qaRCAbout(frames, qaRCHero),
		[]string{"ResourceAdded act-qarc-hero nerve 1/1"})
	qaRCSheetMatches(t, own, qaRCMate)
	qaRCSheetMatches(t, own, qaRCHero)
	qaRCNoReintro(t, own.sent[start:], qaRCMate)
	qaRCNoReintro(t, own.sent[start:], qaRCHero)
}
