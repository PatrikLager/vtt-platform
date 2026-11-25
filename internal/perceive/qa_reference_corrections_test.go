package perceive_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const (
	qarcParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qarcCreature = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qarcLog struct {
	eyes perceive.Eyes
	fold *engine.State
	pr   *perceive.Projection
	seq  int64
}

type qarcTable struct {
	t     *testing.T
	world *engine.State
	seq   int64
	logs  map[string]*qarcLog
}

func qarcNewTable(t *testing.T, shoulders ...string) *qarcTable {
	t.Helper()
	tb := &qarcTable{t: t, world: engine.NewState(), logs: map[string]*qarcLog{}}
	for _, s := range shoulders {
		eyes := perceive.Eyes{Shoulder: s}
		tb.logs[s] = &qarcLog{eyes: eyes, fold: engine.NewState(), pr: perceive.New(eyes, nil)}
	}
	return tb
}

func (tb *qarcTable) stamp(env *vttv1.Envelope) {
	tb.t.Helper()
	tb.seq++
	env.Sequence = tb.seq
	env.EventId = fmt.Sprintf("qarc-%d", tb.seq)
	env.SessionId = "qarc-session"
	env.ActorRole = "dm"
	env.ParticipantId = "qarc-issuer"
	env.OccurredAt = timestamppb.New(timestamppb.Now().AsTime())
	if err := engine.Apply(tb.world, env); err != nil {
		tb.t.Fatalf("the world refused %s: %v", qarcText(env), err)
	}
}

func (tb *qarcTable) drop(env *vttv1.Envelope) {
	tb.t.Helper()
	tb.stamp(env)
	tb.hand(env, "a")
}

func (tb *qarcTable) do(env *vttv1.Envelope) map[string][]*vttv1.Envelope {
	tb.t.Helper()
	tb.stamp(env)
	return tb.hand(env, "")
}

func (tb *qarcTable) hand(env *vttv1.Envelope, skip string) map[string][]*vttv1.Envelope {
	tb.t.Helper()
	out := map[string][]*vttv1.Envelope{}
	for _, s := range qarcSorted(tb.logs) {
		if s == skip {
			continue
		}
		l := tb.logs[s]
		frames := l.pr.Event(l.eyes, env, l.fold, tb.world)
		l.take(tb.t, s, frames)
		qarcHolds(tb.t, s, l, tb.world)
		out[s] = frames
	}
	return out
}

func (l *qarcLog) take(t *testing.T, name string, frames []*vttv1.Envelope) {
	t.Helper()
	for i, f := range frames {
		c := proto.Clone(f).(*vttv1.Envelope)
		l.seq++
		c.Sequence = l.seq
		if err := engine.Apply(l.fold, c); err != nil {
			t.Fatalf("log %s: frame %d does not fold: %v\n%s", name, i, err, qarcTexts(frames))
		}
	}
}

// VTT-1033, VTT-1068.
func qarcHolds(t *testing.T, name string, l *qarcLog, world *engine.State) {
	t.Helper()
	for _, id := range qarcSorted(world.Actors) {
		if !perceive.SeesActor(l.eyes, world, id) {
			continue
		}
		w := world.Actors[id]
		h := l.fold.Actors[id]
		if h == nil {
			t.Fatalf("log %s holds no %s, which is in its sight", name, id)
		}
		var want map[string]*vttv1.Reference
		if engine.IsPartyMember(w) {
			want = w.GetReferences()
		}
		if !qarcRefsEqual(h.GetReferences(), want) {
			t.Fatalf("log %s holds %s at %s, the world's form is %s",
				name, id, qarcRefsText(h.GetReferences()), qarcRefsText(want))
		}
	}
	for _, id := range qarcSorted(l.fold.Actors) {
		h := l.fold.Actors[id]
		if !engine.IsPartyMember(h) && len(h.GetReferences()) > 0 {
			t.Fatalf("log %s holds creature %s with references %s",
				name, id, qarcRefsText(h.GetReferences()))
		}
	}
}

func qarcSorted[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func qarcText(env *vttv1.Envelope) string {
	return prototext.MarshalOptions{}.Format(env)
}

func qarcTexts(frames []*vttv1.Envelope) string {
	var b strings.Builder
	for i, f := range frames {
		fmt.Fprintf(&b, "  [%d] %s\n", i, qarcText(f))
	}
	return b.String()
}

func qarcRef(spec string) *vttv1.Reference {
	p := strings.Split(spec, "/")
	return &vttv1.Reference{EntryId: p[0], Place: p[1], State: p[2]}
}

func qarcRefs(specs ...string) map[string]*vttv1.Reference {
	out := map[string]*vttv1.Reference{}
	for _, s := range specs {
		id, v, _ := strings.Cut(s, "=")
		out[id] = qarcRef(v)
	}
	return out
}

func qarcRefsEqual(a, b map[string]*vttv1.Reference) bool {
	if len(a) != len(b) {
		return false
	}
	for id, x := range a {
		y, ok := b[id]
		if !ok || x.GetEntryId() != y.GetEntryId() || x.GetPlace() != y.GetPlace() ||
			x.GetState() != y.GetState() {
			return false
		}
	}
	return true
}

func qarcRefsText(m map[string]*vttv1.Reference) string {
	parts := []string{}
	for _, id := range qarcSorted(m) {
		r := m[id]
		parts = append(parts, fmt.Sprintf("%s=%s/%s/%s", id, r.GetEntryId(), r.GetPlace(), r.GetState()))
	}
	return "{" + strings.Join(parts, " ") + "}"
}

func qarcScene(id string, wallX int32) *vttv1.Envelope {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 5; x++ {
		for y := int32(0); y < 3; y++ {
			kind := "floor"
			if x == wallX {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: id, Name: id, GridWidth: 5, GridHeight: 3, Tiles: tiles}}}
}

func qarcAdded(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qarcRefAdded(actor, id, spec string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceAdded{ReferenceAdded: &vttv1.ReferenceAdded{
		ActorId: actor, ReferenceId: id, Reference: qarcRef(spec)}}}
}

func qarcRefRemoved(actor, id string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceRemoved{ReferenceRemoved: &vttv1.ReferenceRemoved{
		ActorId: actor, ReferenceId: id}}}
}

func qarcRefChanged(actor, id, place, state string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceChanged{ReferenceChanged: &vttv1.ReferenceChanged{
		ActorId: actor, ReferenceId: id, Place: place, State: state}}}
}

func qarcPlaced(tok, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: "near", ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y}}}}
}

func qarcMoved(tok string, fx, fy, tx, ty int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tok, SceneId: "near", From: &vttv1.GridPosition{X: fx, Y: fy},
		To: &vttv1.GridPosition{X: tx, Y: ty}}}}
}

func qarcGranted(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: participant, Kind: kind}}}
}

func qarcGiven(actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{
		ActorId: actor, AbilityId: ability}}}
}

func qarcTaken(actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{
		ActorId: actor, AbilityId: ability}}}
}

func qarcResourceAdded(actor, name string, current, maxValue int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceAdded{ResourceAdded: &vttv1.ResourceAdded{
		ActorId: actor, Resource: name, Value: &vttv1.Resource{Current: current, Max: maxValue}}}}
}

func qarcNarration(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

func qarcSubject(f *vttv1.Envelope) (string, string) {
	switch p := f.GetPayload().(type) {
	case *vttv1.Envelope_ActorAdded:
		return "ActorAdded", p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorRemoved:
		return "ActorRemoved", p.ActorRemoved.GetActorId()
	case *vttv1.Envelope_ReferenceAdded:
		return "ReferenceAdded", p.ReferenceAdded.GetActorId()
	case *vttv1.Envelope_ReferenceRemoved:
		return "ReferenceRemoved", p.ReferenceRemoved.GetActorId()
	case *vttv1.Envelope_ReferenceChanged:
		return "ReferenceChanged", p.ReferenceChanged.GetActorId()
	case *vttv1.Envelope_AbilityGiven:
		return "AbilityGiven", p.AbilityGiven.GetActorId()
	case *vttv1.Envelope_AbilityTaken:
		return "AbilityTaken", p.AbilityTaken.GetActorId()
	case *vttv1.Envelope_ResourceAdded:
		return "ResourceAdded", p.ResourceAdded.GetActorId()
	case *vttv1.Envelope_ResourceRemoved:
		return "ResourceRemoved", p.ResourceRemoved.GetActorId()
	case *vttv1.Envelope_ResourceChanged:
		return "ResourceChanged", p.ResourceChanged.GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		return "ActorControlGranted", p.ActorControlGranted.GetActorId()
	case *vttv1.Envelope_ActorControlRevoked:
		return "ActorControlRevoked", p.ActorControlRevoked.GetActorId()
	case *vttv1.Envelope_TokenPlaced:
		return "TokenPlaced", p.TokenPlaced.GetActorId()
	}
	return fmt.Sprintf("%T", f.GetPayload()), ""
}

func qarcCount(frames []*vttv1.Envelope, kind, actor string) int {
	n := 0
	for _, f := range frames {
		if k, a := qarcSubject(f); k == kind && a == actor {
			n++
		}
	}
	return n
}

func qarcNoReintroduction(t *testing.T, frames []*vttv1.Envelope, actor string) {
	t.Helper()
	if qarcCount(frames, "ActorRemoved", actor)+qarcCount(frames, "ActorAdded", actor) != 0 {
		t.Fatalf("%s is withdrawn or introduced again:\n%s", actor, qarcTexts(frames))
	}
}

type qarcRefFrame struct {
	Kind, ID, Entry, Place, State string
}

func (r qarcRefFrame) String() string {
	return fmt.Sprintf("%s %s %s/%s/%s", r.Kind, r.ID, r.Entry, r.Place, r.State)
}

func qarcRefFramesOf(frames []*vttv1.Envelope, actor string) []qarcRefFrame {
	var out []qarcRefFrame
	for _, f := range frames {
		switch p := f.GetPayload().(type) {
		case *vttv1.Envelope_ReferenceRemoved:
			if p.ReferenceRemoved.GetActorId() == actor {
				out = append(out, qarcRefFrame{Kind: "removed", ID: p.ReferenceRemoved.GetReferenceId()})
			}
		case *vttv1.Envelope_ReferenceAdded:
			if a := p.ReferenceAdded; a.GetActorId() == actor {
				r := a.GetReference()
				out = append(out, qarcRefFrame{Kind: "added", ID: a.GetReferenceId(),
					Entry: r.GetEntryId(), Place: r.GetPlace(), State: r.GetState()})
			}
		case *vttv1.Envelope_ReferenceChanged:
			if c := p.ReferenceChanged; c.GetActorId() == actor {
				out = append(out, qarcRefFrame{Kind: "changed", ID: c.GetReferenceId(),
					Place: c.GetPlace(), State: c.GetState()})
			}
		}
	}
	return out
}

func qarcKindRank(k string) int {
	return map[string]int{"removed": 0, "added": 1, "changed": 2}[k]
}

func qarcSameFrames(got, want []qarcRefFrame) bool {
	g := append([]qarcRefFrame(nil), got...)
	w := append([]qarcRefFrame(nil), want...)
	less := func(s []qarcRefFrame) func(i, j int) bool {
		return func(i, j int) bool {
			if s[i].ID != s[j].ID {
				return s[i].ID < s[j].ID
			}
			return qarcKindRank(s[i].Kind) < qarcKindRank(s[j].Kind)
		}
	}
	sort.Slice(g, less(g))
	sort.Slice(w, less(w))
	return fmt.Sprint(g) == fmt.Sprint(w)
}

// SPEC-016.
func qarcBare(t *testing.T, frames []*vttv1.Envelope, actor string, env *vttv1.Envelope) {
	t.Helper()
	for _, f := range frames {
		k, a := qarcSubject(f)
		if a != actor || !strings.HasPrefix(k, "Reference") {
			continue
		}
		if f.GetSequence() != env.GetSequence() || f.GetEventId() != "" || f.GetOccurredAt() != nil ||
			f.GetSessionId() != "" || f.GetActorRole() != "" || f.GetParticipantId() != "" {
			t.Fatalf("correction frame is not bare at sequence %d: %s", env.GetSequence(), qarcText(f))
		}
	}
}

// Keep a token out of a's sight at x 3 or 4 and in it at x 0 or 1: near's
// wall is column 2.
func qarcStage(t *testing.T, others ...*vttv1.Actor) *qarcTable {
	t.Helper()
	tb := qarcNewTable(t, "a", "b", "d")
	tb.do(qarcScene("near", 2))
	tb.do(qarcAdded(&vttv1.Actor{ActorId: "a", Name: "alfa", Kind: qarcParty}))
	tb.do(qarcPlaced("ta", "a", 0, 1))
	for i, o := range others {
		id := o.GetActorId()
		tb.do(qarcAdded(o))
		tb.do(qarcPlaced("t"+id, id, 4, int32(i%3)))
		if perceive.SeesActor(tb.logs["a"].eyes, tb.world, id) {
			t.Fatalf("the stage shows %s to a", id)
		}
	}
	return tb
}

func qarcMoveTo(tb *qarcTable, actor string, x int32) ([]*vttv1.Envelope, *vttv1.Envelope) {
	tb.t.Helper()
	tok := tb.world.Tokens["t"+actor]
	env := qarcMoved(tok.ID, tok.X, tok.Y, x, tok.Y)
	return tb.do(env)["a"], env
}

func qarcSee(tb *qarcTable, actor string) ([]*vttv1.Envelope, *vttv1.Envelope) {
	tb.t.Helper()
	fr, env := qarcMoveTo(tb, actor, 1)
	if !perceive.SeesActor(tb.logs["a"].eyes, tb.world, actor) {
		tb.t.Fatalf("a does not see %s at x 1", actor)
	}
	return fr, env
}

func qarcHide(tb *qarcTable, actor string) {
	tb.t.Helper()
	qarcMoveTo(tb, actor, 4)
	if perceive.SeesActor(tb.logs["a"].eyes, tb.world, actor) {
		tb.t.Fatalf("a still sees %s at x 4", actor)
	}
}

func qarcReintroduced(t *testing.T, frames []*vttv1.Envelope, actor string) {
	t.Helper()
	if qarcCount(frames, "ActorRemoved", actor) != 1 || qarcCount(frames, "ActorAdded", actor) != 1 {
		t.Fatalf("%s is not withdrawn and introduced afresh:\n%s", actor, qarcTexts(frames))
	}
}

// VTT-1033.
func TestQARCAPartyMemberItsEyesDoNotSeeIsIntroducedWithNoReferences(t *testing.T) {
	refs := qarcRefs("r1=echo/mike/lima", "r2=delta/kilo/oscar")

	tb := qarcNewTable(t, "a")
	fr := tb.do(qarcAdded(&vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty, References: refs}))["a"]
	qarcIntroducedBare(t, fr, "b")

	tb = qarcStage(t)
	fr = tb.do(qarcAdded(&vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		Attributes: map[string]int32{"tango": 3}, References: refs}))["a"]
	qarcIntroducedBare(t, fr, "b")
	tb.do(qarcPlaced("tb", "b", 4, 1))
	tb.do(qarcRefAdded("b", "r3", "echo/kilo/lima"))
	if n := len(tb.logs["a"].fold.Actors["b"].GetReferences()); n != 0 {
		t.Fatalf("an unseen party member is held with %d references", n)
	}

	tb = qarcStage(t, &vttv1.Actor{ActorId: "c", Kind: qarcCreature, References: refs})
	fr = tb.do(qarcGranted("c", "qarc-player", qarcParty))["a"]
	qarcIntroducedBare(t, fr, "c")
}

// VTT-1033.
func TestQARCACreatureIsIntroducedWithNoReferences(t *testing.T) {
	for _, name := range []string{"", "charlie"} {
		tb := qarcStage(t)
		tb.do(qarcAdded(&vttv1.Actor{ActorId: "c", Name: name, Kind: qarcCreature,
			References: qarcRefs("r1=echo/mike/lima")}))
		fr := tb.do(qarcPlaced("tc", "c", 1, 1))["a"]
		qarcIntroducedBare(t, fr, "c")
		for _, env := range []*vttv1.Envelope{
			qarcRefAdded("c", "r2", "delta/kilo/oscar"),
			qarcRefChanged("c", "r1", "kilo", "oscar"),
			qarcRefRemoved("c", "r2"),
		} {
			if refs := qarcRefFramesOf(tb.do(env)["a"], "c"); len(refs) != 0 {
				t.Fatalf("a creature's reference change reaches the log: %v", refs)
			}
		}
	}
}

// VTT-1068, VTT-1069.
func TestQARCReferencesChangedUnseenAreCorrectedAtSight(t *testing.T) {
	tb := qarcStage(t, &vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		Attributes: map[string]int32{"tango": 3},
		References: qarcRefs("keep=echo/mike/lima", "gone=delta/kilo/oscar", "place=echo/mike/lima",
			"state=delta/mike/lima", "swap=echo/mike/lima", "back=echo/mike/lima",
			"again=delta/kilo/lima", "Up=echo/kilo/oscar", "r10=delta/mike/oscar", "r9=echo/kilo/lima")})
	qarcSee(tb, "b")
	qarcHide(tb, "b")
	for _, env := range []*vttv1.Envelope{
		qarcRefAdded("b", "new", "echo/kilo/lima"),
		qarcRefRemoved("b", "gone"),
		qarcRefChanged("b", "place", "kilo", "lima"),
		qarcRefChanged("b", "state", "mike", "oscar"),
		qarcRefRemoved("b", "swap"),
		qarcRefAdded("b", "swap", "delta/mike/lima"),
		qarcRefAdded("b", "twin", "echo/mike/lima"),
		qarcRefAdded("b", "tmp", "delta/mike/lima"),
		qarcRefRemoved("b", "tmp"),
		qarcRefChanged("b", "back", "kilo", "oscar"),
		qarcRefChanged("b", "back", "mike", "lima"),
		qarcRefRemoved("b", "again"),
		qarcRefAdded("b", "again", "delta/kilo/lima"),
		qarcRefChanged("b", "Up", "kilo", "lima"),
		qarcRefRemoved("b", "r10"),
		qarcRefChanged("b", "r9", "mike", "lima"),
	} {
		if fr := tb.do(env)["a"]; len(fr) != 0 {
			t.Fatalf("an unseen party member's reference event reaches the log:\n%s", qarcTexts(fr))
		}
	}
	fr, env := qarcSee(tb, "b")
	qarcNoReintroduction(t, fr, "b")
	want := []qarcRefFrame{
		{Kind: "added", ID: "new", Entry: "echo", Place: "kilo", State: "lima"},
		{Kind: "removed", ID: "gone"},
		{Kind: "changed", ID: "place", Place: "kilo", State: "lima"},
		{Kind: "changed", ID: "state", Place: "mike", State: "oscar"},
		{Kind: "removed", ID: "swap"},
		{Kind: "added", ID: "swap", Entry: "delta", Place: "mike", State: "lima"},
		{Kind: "added", ID: "twin", Entry: "echo", Place: "mike", State: "lima"},
		{Kind: "changed", ID: "Up", Place: "kilo", State: "lima"},
		{Kind: "removed", ID: "r10"},
		{Kind: "changed", ID: "r9", Place: "mike", State: "lima"},
	}
	if got := qarcRefFramesOf(fr, "b"); !qarcSameFrames(got, want) {
		t.Fatalf("the correction's reference frames are\n%v\nwant\n%v", got, want)
	}
	qarcBare(t, fr, "b", env)
}

// VTT-1068, VTT-1069.
func TestQARCAPartyMemberNeverSeenIsBroughtItsReferencesAtFirstSight(t *testing.T) {
	tb := qarcStage(t, &vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		References: qarcRefs("r1=echo/mike/lima", "r2=echo/mike/lima", "r3=delta/kilo/oscar")})
	tb.do(qarcRefChanged("b", "r3", "mike", "lima"))
	tb.do(qarcRefAdded("b", "r4", "delta/kilo/lima"))
	tb.do(qarcRefAdded("b", "r5", "echo/mike/oscar"))
	tb.do(qarcRefRemoved("b", "r5"))
	fr, env := qarcSee(tb, "b")
	qarcNoReintroduction(t, fr, "b")
	want := []qarcRefFrame{
		{Kind: "added", ID: "r1", Entry: "echo", Place: "mike", State: "lima"},
		{Kind: "added", ID: "r2", Entry: "echo", Place: "mike", State: "lima"},
		{Kind: "added", ID: "r3", Entry: "delta", Place: "mike", State: "lima"},
		{Kind: "added", ID: "r4", Entry: "delta", Place: "kilo", State: "lima"},
	}
	if got := qarcRefFramesOf(fr, "b"); !qarcSameFrames(got, want) {
		t.Fatalf("the first sight's reference frames are\n%v\nwant\n%v", got, want)
	}
	qarcBare(t, fr, "b", env)
}

// D12.
func TestQARCReferenceFramesFollowTheAbilityFramesInByteOrderOfTheirIds(t *testing.T) {
	ids := []string{"r9", "r10", "Zed", "_u", "a-1", "b", "B", "~", "0", "r1"}
	initial := []string{}
	for _, id := range ids {
		initial = append(initial, id+"=echo/mike/lima")
	}
	for round := 0; round < 25; round++ {
		tb := qarcStage(t, &vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
			AbilityIds: []string{"xray"}, References: qarcRefs(initial...)})
		qarcSee(tb, "b")
		qarcHide(tb, "b")
		tb.do(qarcTaken("b", "xray"))
		tb.do(qarcGiven("b", "papa"))
		tb.do(qarcResourceAdded("b", "quebec", 3, 5))
		for i, id := range ids {
			switch i % 3 {
			case 0:
				tb.do(qarcRefRemoved("b", id))
				tb.do(qarcRefAdded("b", id, "delta/kilo/oscar"))
			case 1:
				tb.do(qarcRefChanged("b", id, "kilo", "oscar"))
			default:
				tb.do(qarcRefRemoved("b", id))
			}
			tb.do(qarcRefAdded("b", id+"+", "delta/mike/lima"))
		}
		fr, _ := qarcSee(tb, "b")
		qarcOrdered(t, fr, "b")
	}
}

func qarcOrdered(t *testing.T, fr []*vttv1.Envelope, actor string) {
	t.Helper()
	lastAbility, firstRef, lastRef, placed := -1, -1, -1, -1
	var prev qarcRefFrame
	for i, f := range fr {
		k, a := qarcSubject(f)
		if a != actor {
			continue
		}
		switch {
		case strings.HasPrefix(k, "Ability"):
			lastAbility = i
		case k == "TokenPlaced":
			placed = i
		case strings.HasPrefix(k, "Reference"):
			if firstRef < 0 {
				firstRef = i
			}
			lastRef = i
			cur := qarcRefFramesOf([]*vttv1.Envelope{f}, actor)[0]
			if prev.ID > cur.ID || (prev.ID == cur.ID && qarcKindRank(prev.Kind) >= qarcKindRank(cur.Kind)) {
				t.Fatalf("reference frame %v follows %v:\n%s", cur, prev, qarcTexts(fr))
			}
			prev = cur
		}
	}
	if lastAbility < 0 || firstRef < 0 || placed < 0 {
		t.Fatalf("the sight sends no ability, reference or token frame:\n%s", qarcTexts(fr))
	}
	if lastAbility > firstRef || lastRef > placed {
		t.Fatalf("the reference frames do not stand between the ability frames and the token:\n%s",
			qarcTexts(fr))
	}
}

// VTT-1069.
func TestQARCAReferenceEventInSightIsForwardedAndReintroducesNoActor(t *testing.T) {
	tb := qarcStage(t, &vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		References: qarcRefs("r1=echo/mike/lima")})
	qarcSee(tb, "b")
	for _, env := range []*vttv1.Envelope{
		qarcRefAdded("b", "r2", "echo/kilo/lima"),
		qarcRefChanged("b", "r1", "kilo", "oscar"),
		qarcRefChanged("b", "r2", "kilo", "lima"),
		qarcRefRemoved("b", "r1"),
		qarcRefAdded("b", "r1", "delta/mike/lima"),
		qarcRefAdded("a", "r1", "echo/mike/lima"),
		qarcRefChanged("a", "r1", "mike", "oscar"),
		qarcRefRemoved("a", "r1"),
	} {
		fr := tb.do(env)["a"]
		want := proto.Clone(env).(*vttv1.Envelope)
		want.ParticipantId, want.ActorRole = "", ""
		if len(fr) != 1 || !proto.Equal(fr[0], want) {
			t.Fatalf("%s reaches the log as\n%s", qarcText(env), qarcTexts(fr))
		}
	}
}

// D12, VTT-1033, VTT-468.
func TestQARCAKindChangeInSightReintroducesAnActorHoldingReferences(t *testing.T) {
	refs := qarcRefs("r1=echo/mike/lima", "r2=echo/kilo/oscar")
	for _, name := range []string{"", "charlie"} {
		tb := qarcStage(t)
		tb.do(qarcAdded(&vttv1.Actor{ActorId: "c", Name: name, Kind: qarcCreature, References: refs}))
		tb.do(qarcPlaced("tc", "c", 1, 1))
		qarcReintroduced(t, tb.do(qarcGranted("c", "qarc-player", qarcParty))["a"], "c")

		tb = qarcStage(t, &vttv1.Actor{ActorId: "p", Name: name, Kind: qarcParty, References: refs})
		qarcSee(tb, "p")
		qarcReintroduced(t, tb.do(qarcGranted("p", "qarc-player", qarcCreature))["a"], "p")
	}
}

// D12, VTT-468, SPEC-016.
func TestQARCAKindChangeOutOfSightWaitsForSight(t *testing.T) {
	refs := qarcRefs("r1=echo/mike/lima")
	for _, name := range []string{"", "papa"} {
		tb := qarcStage(t, &vttv1.Actor{ActorId: "p", Name: name, Kind: qarcParty, References: refs})
		qarcSee(tb, "p")
		qarcHide(tb, "p")
		tb.do(qarcRefAdded("p", "r2", "delta/kilo/lima"))
		tb.do(qarcGranted("p", "qarc-player", qarcCreature))
		if h := tb.logs["a"].fold.Actors["p"]; !engine.IsPartyMember(h) || !qarcRefsEqual(h.GetReferences(), refs) {
			t.Fatalf("out of sight the log holds p as %s", qarcText(qarcAdded(h)))
		}
		fr, _ := qarcSee(tb, "p")
		qarcReintroduced(t, fr, "p")

		tb = qarcStage(t, &vttv1.Actor{ActorId: "c", Name: name, Kind: qarcCreature, References: refs})
		qarcSee(tb, "c")
		qarcHide(tb, "c")
		tb.do(qarcGranted("c", "qarc-player", qarcParty))
		tb.do(qarcRefAdded("c", "r2", "delta/kilo/lima"))
		fr, _ = qarcSee(tb, "c")
		qarcReintroduced(t, fr, "c")
	}
}

func qarcSynced(t *testing.T) *qarcTable {
	t.Helper()
	tb := qarcStage(t,
		&vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
			References: qarcRefs("r1=echo/mike/lima", "r2=echo/kilo/oscar")},
		&vttv1.Actor{ActorId: "d", Name: "dora", Kind: qarcParty,
			References: qarcRefs("r1=delta/mike/lima")})
	tb.do(qarcRefAdded("a", "own", "echo/mike/lima"))
	qarcSee(tb, "d")
	qarcHide(tb, "d")
	qarcSee(tb, "b")
	tb.do(qarcAdded(&vttv1.Actor{ActorId: "c", Name: "charlie", Kind: qarcCreature,
		References: qarcRefs("r1=echo/mike/lima")}))
	tb.do(qarcPlaced("tc", "c", 0, 0))
	return tb
}

func qarcResume(tb *qarcTable) {
	tb.t.Helper()
	l := tb.logs["a"]
	l.pr = perceive.Resume(l.eyes, l.fold, tb.world)
}

// VTT-1035.
func TestQARCResumeResumesAFoldAtTheWorldsReferences(t *testing.T) {
	for _, probe := range []func() *vttv1.Envelope{
		func() *vttv1.Envelope { return qarcRefChanged("b", "r1", "kilo", "lima") },
		func() *vttv1.Envelope { return qarcRefRemoved("b", "r2") },
		func() *vttv1.Envelope { return qarcRefAdded("b", "r3", "echo/mike/lima") },
		func() *vttv1.Envelope { return qarcRefAdded("a", "own2", "delta/kilo/oscar") },
	} {
		tb := qarcSynced(t)
		tb.do(qarcRefChanged("d", "r1", "kilo", "oscar"))
		tb.do(qarcRefAdded("d", "r2", "echo/mike/lima"))
		tb.do(qarcRefChanged("c", "r1", "kilo", "oscar"))
		qarcResume(tb)
		env := probe()
		fr := tb.do(env)["a"]
		if len(fr) != 1 || fr[0].GetEventId() != env.GetEventId() {
			t.Fatalf("a resumed log is not forwarded %s:\n%s", qarcText(env), qarcTexts(fr))
		}
	}
}

// VTT-1035.
func TestQARCResumeStartsAfreshFromAFoldLaggingTheWorldsReferences(t *testing.T) {
	cases := []struct {
		name string
		drop []*vttv1.Envelope
		next *vttv1.Envelope
	}{
		{"lacks an id", []*vttv1.Envelope{qarcRefAdded("b", "r3", "delta/mike/lima")},
			qarcRefChanged("b", "r3", "kilo", "oscar")},
		{"holds an id the world does not", []*vttv1.Envelope{qarcRefRemoved("b", "r2")},
			qarcRefAdded("b", "r2", "delta/kilo/lima")},
		{"holds an id as another entry",
			[]*vttv1.Envelope{qarcRefRemoved("b", "r1"), qarcRefAdded("b", "r1", "delta/mike/lima")},
			qarcRefChanged("b", "r1", "kilo", "lima")},
		{"holds a reference at another place", []*vttv1.Envelope{qarcRefChanged("b", "r1", "kilo", "lima")},
			qarcRefChanged("b", "r2", "mike", "oscar")},
		{"holds a reference at another state", []*vttv1.Envelope{qarcRefChanged("b", "r1", "mike", "oscar")},
			qarcRefChanged("b", "r2", "kilo", "lima")},
		{"holds an entry once the world holds twice", []*vttv1.Envelope{qarcRefAdded("b", "r4", "echo/mike/lima")},
			qarcNarration("qarc")},
		{"lags for its own character", []*vttv1.Envelope{qarcRefChanged("a", "own", "kilo", "oscar")},
			qarcRefAdded("a", "own2", "delta/kilo/oscar")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := qarcSynced(t)
			for _, env := range c.drop {
				tb.drop(env)
			}
			qarcResume(tb)
			fr := tb.do(c.next)["a"]
			qarcNoReintroduction(t, fr, "a")
			qarcNoReintroduction(t, fr, "b")
			if c.next.GetNarrationAdded() != nil {
				if got := qarcRefFramesOf(fr, "b"); len(got) != 1 || got[0].ID != "r4" {
					t.Fatalf("the narration after a resume brings %v", got)
				}
				return
			}
			for _, f := range fr {
				if f.GetEventId() == c.next.GetEventId() {
					t.Fatalf("a lagging log is forwarded %s", qarcText(f))
				}
			}
		})
	}
}

// VTT-1033, VTT-1068, VTT-1069, SPEC-021.
func TestQARCALogOpenedThroughNoEyesIsBroughtTheReferencesAtItsFirstEvent(t *testing.T) {
	tb := qarcNewTable(t)
	tb.do(qarcScene("near", 2))
	tb.do(qarcAdded(&vttv1.Actor{ActorId: "a", Name: "alfa", Kind: qarcParty,
		References: qarcRefs("r1=echo/mike/lima")}))
	tb.do(qarcPlaced("ta", "a", 0, 1))
	tb.do(qarcAdded(&vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		References: qarcRefs("r1=delta/kilo/oscar", "r2=delta/kilo/oscar")}))
	tb.do(qarcPlaced("tb", "b", 1, 1))
	tb.do(qarcAdded(&vttv1.Actor{ActorId: "c", Name: "charlie", Kind: qarcCreature,
		References: qarcRefs("r1=echo/mike/lima")}))
	tb.do(qarcPlaced("tc", "c", 1, 0))

	fold := engine.NewState()
	frames := perceive.New(perceive.Eyes{}, nil).Perch(perceive.Eyes{}, fold, tb.world, 0)
	qarcIntroducedBare(t, frames, "a")
	qarcIntroducedBare(t, frames, "b")
	eyes := perceive.Eyes{Shoulder: "a"}
	l := &qarcLog{eyes: eyes, fold: fold, pr: perceive.New(eyes, nil)}
	l.take(t, "a", frames)
	tb.logs["a"] = l
	fr := tb.do(qarcNarration("qarc"))["a"]
	qarcNoReintroduction(t, fr, "a")
	qarcNoReintroduction(t, fr, "b")
}

// VTT-1068, VTT-1069.
func TestQARCAReferenceTheWorldHoldsWithNoValueIsBroughtByAFoldableFrame(t *testing.T) {
	tb := qarcStage(t, &vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		References: map[string]*vttv1.Reference{"r1": nil, "r2": {}}})
	fr, _ := qarcSee(tb, "b")
	qarcNoReintroduction(t, fr, "b")
}

func qarcIntroducedBare(t *testing.T, frames []*vttv1.Envelope, actor string) {
	t.Helper()
	n := 0
	for _, f := range frames {
		if a := f.GetActorAdded().GetActor(); a != nil && a.GetActorId() == actor {
			n++
			if len(a.GetReferences()) != 0 {
				t.Fatalf("%s is introduced with references: %s", actor, qarcText(f))
			}
		}
	}
	if n != 1 {
		t.Fatalf("%s is introduced %d times:\n%s", actor, n, qarcTexts(frames))
	}
}

// VTT-1068, VTT-1069, VTT-783.
func TestQARCAPartyMemberAddedAgainWithOtherReferencesIsCorrected(t *testing.T) {
	p := &vttv1.Actor{ActorId: "p", Name: "papa", Kind: qarcParty,
		Attributes: map[string]int32{"tango": 3}, References: qarcRefs("r1=echo/mike/lima", "r2=echo/mike/lima")}
	tb := qarcStage(t, p)
	qarcSee(tb, "p")
	qarcHide(tb, "p")
	tb.do(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "tp"}}})
	tb.do(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: "p"}}})
	again := proto.Clone(p).(*vttv1.Actor)
	again.References = qarcRefs("r1=delta/mike/lima", "r3=echo/kilo/oscar", "r2=echo/kilo/lima")
	tb.do(qarcAdded(again))
	tb.do(qarcPlaced("tp", "p", 4, 1))
	fr, env := qarcSee(tb, "p")
	qarcNoReintroduction(t, fr, "p")
	want := []qarcRefFrame{
		{Kind: "removed", ID: "r1"},
		{Kind: "added", ID: "r1", Entry: "delta", Place: "mike", State: "lima"},
		{Kind: "changed", ID: "r2", Place: "kilo", State: "lima"},
		{Kind: "added", ID: "r3", Entry: "echo", Place: "kilo", State: "oscar"},
	}
	if got := qarcRefFramesOf(fr, "p"); !qarcSameFrames(got, want) {
		t.Fatalf("the sight's reference frames are\n%v\nwant\n%v", got, want)
	}
	qarcBare(t, fr, "p", env)
}

// VTT-1069, SPEC-016.
func TestQARCAPerchOntoAPartyMemberBringsItsReferences(t *testing.T) {
	tb := qarcNewTable(t)
	tb.do(qarcScene("near", 2))
	tb.do(qarcAdded(&vttv1.Actor{ActorId: "a", Name: "alfa", Kind: qarcParty,
		References: qarcRefs("r1=echo/mike/lima")}))
	tb.do(qarcPlaced("ta", "a", 0, 1))
	tb.do(qarcAdded(&vttv1.Actor{ActorId: "b", Name: "bravo", Kind: qarcParty,
		References: qarcRefs("r1=delta/kilo/oscar", "r2=delta/kilo/oscar")}))
	tb.do(qarcPlaced("tb", "b", 1, 1))
	fold := engine.NewState()
	pr := perceive.New(perceive.Eyes{}, nil)
	l := &qarcLog{eyes: perceive.Eyes{}, fold: fold, pr: pr}
	l.take(t, "opening", pr.Perch(perceive.Eyes{}, fold, tb.world, 0))
	l.eyes = perceive.Eyes{Shoulder: "b"}
	fr := pr.Perch(l.eyes, fold, tb.world, 7)
	l.take(t, "b", fr)
	qarcHolds(t, "b", l, tb.world)
	qarcNoReintroduction(t, fr, "a")
	qarcNoReintroduction(t, fr, "b")
	if got := qarcRefFramesOf(fr, "a"); len(got) != 1 {
		t.Fatalf("the perch brings a %v", got)
	}
	for _, f := range fr {
		if f.GetSequence() != 7 {
			t.Fatalf("a perch's frame carries sequence %d: %s", f.GetSequence(), qarcText(f))
		}
	}
}
