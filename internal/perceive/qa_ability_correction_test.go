package perceive_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type qabcViewer struct {
	name  string
	eyes  perceive.Eyes
	pr    *perceive.Projection
	shown *engine.State
}

type qabcTable struct {
	t       *testing.T
	world   *engine.State
	seq     int64
	viewers []*qabcViewer
}

func qabcNewTable(t *testing.T, shoulders ...string) *qabcTable {
	t.Helper()
	tb := &qabcTable{t: t, world: engine.NewState()}
	for _, s := range shoulders {
		eyes := perceive.Eyes{Shoulder: s}
		tb.viewers = append(tb.viewers, &qabcViewer{
			name:  s,
			eyes:  eyes,
			pr:    perceive.New(eyes, nil),
			shown: engine.NewState(),
		})
	}
	return tb
}

func (tb *qabcTable) viewer(name string) *qabcViewer {
	for _, v := range tb.viewers {
		if v.name == name {
			return v
		}
	}
	tb.t.Fatalf("no viewer %q", name)
	return nil
}

func (tb *qabcTable) stamp(env *vttv1.Envelope) {
	tb.seq++
	env.Sequence = tb.seq
	env.EventId = fmt.Sprintf("ev-%d", tb.seq)
	env.SessionId = "sess-q"
	env.ActorRole = "dm"
	env.ParticipantId = "p-dm"
	env.OccurredAt = timestamppb.New(time.Date(2026, 11, 14, 12, 0, int(tb.seq), 0, time.UTC))
}

func (tb *qabcTable) step(env *vttv1.Envelope) map[string][]*vttv1.Envelope {
	tb.t.Helper()
	tb.stamp(env)
	if err := engine.Apply(tb.world, env); err != nil {
		tb.t.Fatalf("world refuses event %d (%s): %v", env.Sequence, qabcKind(env), err)
	}
	out := map[string][]*vttv1.Envelope{}
	for _, v := range tb.viewers {
		frames := v.pr.Event(v.eyes, env, v.shown, tb.world)
		for i, f := range frames {
			if err := engine.Apply(v.shown, f); err != nil {
				tb.t.Fatalf("viewer %s: frame %d (%s) of event %d (%s) does not fold: %v\nframes: %s",
					v.name, i, qabcKind(f), env.Sequence, qabcKind(env), err, qabcDescribe(frames))
			}
		}
		out[v.name] = frames
	}
	return out
}

func (tb *qabcTable) worldOnly(env *vttv1.Envelope) {
	tb.t.Helper()
	tb.stamp(env)
	if err := engine.Apply(tb.world, env); err != nil {
		tb.t.Fatalf("world refuses event %d (%s): %v", env.Sequence, qabcKind(env), err)
	}
}

func qabcKind(f *vttv1.Envelope) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", f.GetPayload()), "*vttv1.Envelope_")
}

func qabcDescribe(frames []*vttv1.Envelope) string {
	var parts []string
	for _, f := range frames {
		parts = append(parts, qabcKind(f)+"("+qabcSubject(f)+")")
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func qabcSubject(f *vttv1.Envelope) string {
	switch p := f.GetPayload().(type) {
	case *vttv1.Envelope_AbilityGiven:
		return p.AbilityGiven.GetActorId() + ":" + p.AbilityGiven.GetAbilityId()
	case *vttv1.Envelope_AbilityTaken:
		return p.AbilityTaken.GetActorId() + ":" + p.AbilityTaken.GetAbilityId()
	case *vttv1.Envelope_ActorAdded:
		return p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorRemoved:
		return p.ActorRemoved.GetActorId()
	case *vttv1.Envelope_ConditionApplied:
		return p.ConditionApplied.GetActorId() + ":" + p.ConditionApplied.GetConditionId()
	case *vttv1.Envelope_ConditionRemoved:
		return p.ConditionRemoved.GetActorId() + ":" + p.ConditionRemoved.GetConditionId()
	case *vttv1.Envelope_ActorControlGranted:
		return p.ActorControlGranted.GetActorId() + ":" + p.ActorControlGranted.GetParticipantId()
	case *vttv1.Envelope_ActorControlRevoked:
		return p.ActorControlRevoked.GetActorId() + ":" + p.ActorControlRevoked.GetParticipantId()
	case *vttv1.Envelope_ResourceChanged:
		return p.ResourceChanged.GetActorId() + ":" + p.ResourceChanged.GetResource()
	case *vttv1.Envelope_TokenPlaced:
		return p.TokenPlaced.GetTokenId()
	case *vttv1.Envelope_TokenHidden:
		return p.TokenHidden.GetTokenId()
	case *vttv1.Envelope_TokenMoved:
		return p.TokenMoved.GetTokenId()
	}
	return ""
}

func qabcActorOf(f *vttv1.Envelope) string {
	switch p := f.GetPayload().(type) {
	case *vttv1.Envelope_AbilityGiven:
		return p.AbilityGiven.GetActorId()
	case *vttv1.Envelope_AbilityTaken:
		return p.AbilityTaken.GetActorId()
	case *vttv1.Envelope_ActorAdded:
		return p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorRemoved:
		return p.ActorRemoved.GetActorId()
	case *vttv1.Envelope_ConditionApplied:
		return p.ConditionApplied.GetActorId()
	case *vttv1.Envelope_ConditionRemoved:
		return p.ConditionRemoved.GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		return p.ActorControlGranted.GetActorId()
	case *vttv1.Envelope_ActorControlRevoked:
		return p.ActorControlRevoked.GetActorId()
	case *vttv1.Envelope_ResourceChanged:
		return p.ResourceChanged.GetActorId()
	case *vttv1.Envelope_TokenPlaced:
		return p.TokenPlaced.GetActorId()
	}
	return ""
}

func qabcAbilityFrames(frames []*vttv1.Envelope, actor string) []string {
	var out []string
	for _, f := range frames {
		switch p := f.GetPayload().(type) {
		case *vttv1.Envelope_AbilityGiven:
			if p.AbilityGiven.GetActorId() == actor {
				out = append(out, "given:"+p.AbilityGiven.GetAbilityId())
			}
		case *vttv1.Envelope_AbilityTaken:
			if p.AbilityTaken.GetActorId() == actor {
				out = append(out, "taken:"+p.AbilityTaken.GetAbilityId())
			}
		}
	}
	return out
}

func qabcCount(frames []*vttv1.Envelope, kind, actor string) int {
	n := 0
	for _, f := range frames {
		if qabcKind(f) == kind && qabcActorOf(f) == actor {
			n++
		}
	}
	return n
}

func qabcHidden(frames []*vttv1.Envelope, token string) int {
	n := 0
	for _, f := range frames {
		if h := f.GetTokenHidden(); h != nil && h.GetTokenId() == token {
			n++
		}
	}
	return n
}

func qabcSet(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

func qabcHeld(t *testing.T, st *engine.State, actor string) []string {
	t.Helper()
	a, ok := st.Actors[actor]
	if !ok {
		t.Fatalf("fold holds no actor %q", actor)
	}
	return a.GetAbilityIds()
}

func qabcSameSet(t *testing.T, label string, shown, world *engine.State, actor string) {
	t.Helper()
	got := qabcSet(qabcHeld(t, shown, actor))
	want := qabcSet(qabcHeld(t, world, actor))
	if !slices.Equal(got, want) {
		t.Fatalf("%s: log holds %q's ids %v (as a set %v), world holds %v (as a set %v)",
			label, actor, qabcHeld(t, shown, actor), got, qabcHeld(t, world, actor), want)
	}
}

func qabcBare(t *testing.T, label string, f *vttv1.Envelope, seq int64) {
	t.Helper()
	if f.GetSequence() != seq || f.GetEventId() != "" || f.GetOccurredAt() != nil ||
		f.GetSessionId() != "" || f.GetActorRole() != "" || f.GetParticipantId() != "" {
		t.Fatalf("%s: frame %s(%s) carries more than the sequence %d: seq=%d event=%q at=%v session=%q role=%q participant=%q",
			label, qabcKind(f), qabcSubject(f), seq, f.GetSequence(), f.GetEventId(), f.GetOccurredAt(),
			f.GetSessionId(), f.GetActorRole(), f.GetParticipantId())
	}
}

func qabcScene() *vttv1.Envelope {
	wall := &vttv1.TileRef{Kind: "wall"}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "s1", Name: "Hall", GridWidth: 9, GridHeight: 3,
		Tiles: map[string]*vttv1.TileRef{"4,0": wall, "4,1": wall, "4,2": wall},
	}}}
}

func qabcAdd(id, name string, kind vttv1.ActorKind, attrs map[string]int32,
	res map[string]*vttv1.Resource, list ...string) *vttv1.Envelope {
	a := &vttv1.Actor{
		ActorId: id, Name: name, ModuleId: "mod-q", Kind: kind,
		Attributes: attrs, Resources: res, AbilityIds: slices.Clone(list),
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func (tb *qabcTable) add(id, name string, kind vttv1.ActorKind, controller string, attrs map[string]int32,
	res map[string]*vttv1.Resource, list ...string) map[string][]*vttv1.Envelope {
	tb.t.Helper()
	out := tb.step(qabcAdd(id, name, kind, attrs, res, list...))
	if controller == "" {
		return out
	}
	grant := qabcGrant(id, controller)
	grant.GetActorControlGranted().Kind = kind
	for name, frames := range tb.step(grant) {
		out[name] = append(out[name], frames...)
	}
	return out
}

func qabcPlace(token, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: token, SceneId: "s1", ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func qabcMove(token string, fx, fy, tx, ty int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: token, SceneId: "s1",
		From: &vttv1.GridPosition{X: fx, Y: fy}, To: &vttv1.GridPosition{X: tx, Y: ty},
	}}}
}

func qabcGive(actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{
		ActorId: actor, AbilityId: ability,
	}}}
}

func qabcTake(actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{
		ActorId: actor, AbilityId: ability,
	}}}
}

func qabcCondition(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cond, Source: "src-q",
	}}}
}

func qabcGrant(actor, participant string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: participant, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}}}
}

func qabcResource(actor, res string, delta, value int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: actor, Resource: res, Delta: delta, NewValue: value, Reason: "why-q",
	}}}
}

func qabcNarrate(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

func qabcTokenRemoved(token string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: token}}}
}

func qabcActorRemoved(actor string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}}
}

const (
	qabcParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qabcCreature = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

func qabcForwarded(t *testing.T, label string, frames []*vttv1.Envelope, env *vttv1.Envelope) {
	t.Helper()
	n := 0
	for _, f := range frames {
		if f.GetEventId() != env.GetEventId() {
			continue
		}
		n++
		if qabcKind(f) != qabcKind(env) || qabcSubject(f) != qabcSubject(env) {
			t.Fatalf("%s: forwarded %s(%s), want %s(%s)", label, qabcKind(f), qabcSubject(f), qabcKind(env), qabcSubject(env))
		}
		if f.GetParticipantId() != "" || f.GetActorRole() != "" {
			t.Fatalf("%s: forwarded frame names its issuer: participant=%q role=%q", label, f.GetParticipantId(), f.GetActorRole())
		}
		if f.GetSequence() != env.GetSequence() {
			t.Fatalf("%s: forwarded frame carries sequence %d, want %d", label, f.GetSequence(), env.GetSequence())
		}
	}
	if n != 1 {
		t.Fatalf("%s: %d frames carry the event id %q, want the event forwarded once; frames %s",
			label, n, env.GetEventId(), qabcDescribe(frames))
	}
}

func qabcNotForwarded(t *testing.T, label string, frames []*vttv1.Envelope, env *vttv1.Envelope) {
	t.Helper()
	for _, f := range frames {
		if f.GetEventId() == env.GetEventId() {
			t.Fatalf("%s: the event was forwarded; frames %s", label, qabcDescribe(frames))
		}
	}
	if got := qabcAbilityFrames(frames, qabcActorOf(env)); len(got) != 0 {
		t.Fatalf("%s: sent %v", label, got)
	}
}

// VTT-741
func TestQAAbilityCorrectionAGiveOrTakeReachesExactlyTheLogsThatSawTheMember(t *testing.T) {
	tb := qabcNewTable(t, "A", "B", "C")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y")
	tb.add("C", "Cy", qabcParty, "p3", map[string]int32{"s": 3}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.step(qabcPlace("tB", "B", 2, 1))
	tb.step(qabcPlace("tC", "C", 7, 1))

	give := qabcGive("B", "w")
	got := tb.step(give)
	qabcForwarded(t, "A sees B, give", got["A"], give)
	qabcForwarded(t, "B's own log, give", got["B"], give)
	qabcNotForwarded(t, "C does not see B, give", got["C"], give)

	take := qabcTake("B", "x")
	got = tb.step(take)
	qabcForwarded(t, "A sees B, take", got["A"], take)
	qabcForwarded(t, "B's own log, take", got["B"], take)
	qabcNotForwarded(t, "C does not see B, take", got["C"], take)

	tb.step(qabcMove("tB", 2, 1, 6, 1))
	give = qabcGive("B", "v")
	got = tb.step(give)
	qabcNotForwarded(t, "A no longer sees B, give", got["A"], give)
	qabcForwarded(t, "B's own log out of A's sight, give", got["B"], give)
	qabcForwarded(t, "C now sees B, give", got["C"], give)

	take = qabcTake("B", "y")
	got = tb.step(take)
	qabcNotForwarded(t, "A no longer sees B, take", got["A"], take)
	qabcForwarded(t, "B's own log out of A's sight, take", got["B"], take)
	qabcForwarded(t, "C now sees B, take", got["C"], take)

	for _, v := range []string{"B", "C"} {
		qabcSameSet(t, "log "+v+" that saw every change", tb.viewer(v).shown, tb.world, "B")
	}
	if held := qabcHeld(t, tb.viewer("B").shown, "B"); !slices.Equal(held, qabcHeld(t, tb.world, "B")) {
		t.Fatalf("B's own log, forwarded every change in order, holds %v, world %v", held, qabcHeld(t, tb.world, "B"))
	}
}

// VTT-724, VTT-743
func TestQAAbilityCorrectionSightBringsTakesThenGivesBetweenConditionsAndControllers(t *testing.T) {
	tb := qabcNewTable(t, "A", "B")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2},
		map[string]*vttv1.Resource{"r": {Current: 5, Max: 5}}, "z", "y", "x")
	tb.step(qabcPlace("tB", "B", 2, 1))
	if held := qabcHeld(t, tb.viewer("A").shown, "B"); !slices.Equal(held, []string{"z", "y", "x"}) {
		t.Fatalf("A, which sees B, holds %v, want [z y x]", held)
	}
	tb.step(qabcMove("tB", 2, 1, 6, 1))

	for _, env := range []*vttv1.Envelope{
		qabcCondition("B", "c1"),
		qabcTake("B", "x"),
		qabcTake("B", "z"),
		qabcGive("B", "v"),
		qabcGive("B", "u"),
		qabcGrant("B", "p9"),
		qabcResource("B", "r", -2, 3),
	} {
		got := tb.step(env)
		if ab := qabcAbilityFrames(got["A"], "B"); len(ab) != 0 {
			t.Fatalf("A, which does not see B, was sent %v at %s", ab, qabcKind(env))
		}
	}
	if held := qabcHeld(t, tb.viewer("A").shown, "B"); !slices.Equal(held, []string{"z", "y", "x"}) {
		t.Fatalf("A holds %v of B before sight, want the list it last saw [z y x]", held)
	}

	sight := qabcMove("tB", 6, 1, 3, 1)
	got := tb.step(sight)["A"]
	want := []string{"taken:z", "taken:x", "given:v", "given:u"}
	if ab := qabcAbilityFrames(got, "B"); !slices.Equal(ab, want) {
		t.Fatalf("at sight A was sent %v of B, want %v; frames %s", ab, want, qabcDescribe(got))
	}
	lastCond, firstCtl, firstAb, lastAb := -1, -1, -1, -1
	for i, f := range got {
		if qabcActorOf(f) != "B" {
			continue
		}
		switch qabcKind(f) {
		case "ConditionApplied", "ConditionRemoved":
			lastCond = i
		case "ActorControlGranted", "ActorControlRevoked":
			if firstCtl < 0 {
				firstCtl = i
			}
		case "AbilityGiven", "AbilityTaken":
			if firstAb < 0 {
				firstAb = i
			}
			lastAb = i
			qabcBare(t, "a correction's ability frame", f, sight.GetSequence())
		}
	}
	if lastCond < 0 || firstCtl < 0 {
		t.Fatalf("the sight correction carries no condition or no controller of B; frames %s", qabcDescribe(got))
	}
	if firstAb <= lastCond || lastAb >= firstCtl {
		t.Fatalf("ability frames at %d..%d, last condition at %d, first controller at %d; frames %s",
			firstAb, lastAb, lastCond, firstCtl, qabcDescribe(got))
	}
	for _, kind := range []string{"ActorRemoved", "ActorAdded"} {
		if n := qabcCount(got, kind, "B"); n != 0 {
			t.Fatalf("a list change sent %d %s of B; frames %s", n, kind, qabcDescribe(got))
		}
	}
	if n := qabcHidden(got, "tB"); n != 0 {
		t.Fatalf("a list change hid B's token; frames %s", qabcDescribe(got))
	}
	qabcSameSet(t, "A after sight", tb.viewer("A").shown, tb.world, "B")
}

// VTT-744
func TestQAAbilityCorrectionNoListChangeWithdrawsOrIntroducesAMemberAgain(t *testing.T) {
	tb := qabcNewTable(t, "A", "B")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y")
	tb.step(qabcPlace("tB", "B", 2, 1))

	removals := 0
	introductions := 0
	count := func(frames []*vttv1.Envelope) {
		removals += qabcCount(frames, "ActorRemoved", "B")
		introductions += qabcCount(frames, "ActorAdded", "B")
	}
	script := []*vttv1.Envelope{
		qabcGive("B", "w"),
		qabcTake("B", "x"),
		qabcMove("tB", 2, 1, 6, 1),
		qabcGive("B", "x"),
		qabcTake("B", "y"),
		qabcTake("B", "w"),
		qabcMove("tB", 6, 1, 3, 1),
		qabcTake("B", "x"),
		qabcMove("tA", 1, 1, 5, 1),
		qabcGive("B", "q"),
		qabcMove("tA", 5, 1, 0, 1),
	}
	for _, env := range script {
		for _, frames := range tb.step(env) {
			count(frames)
		}
		for _, v := range tb.viewers {
			if _, ok := v.shown.Actors["B"]; !ok {
				t.Fatalf("log %s holds no B after %s", v.name, qabcKind(env))
			}
		}
	}
	if removals != 0 || introductions != 0 {
		t.Fatalf("list changes sent %d ActorRemoved and %d ActorAdded of B", removals, introductions)
	}
	qabcSameSet(t, "A at the end", tb.viewer("A").shown, tb.world, "B")
}

// VTT-724
func TestQAAbilityCorrectionATakeAndAGiveBackUnseenSendNothingAtSight(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y", "x", "z")
	tb.step(qabcPlace("tB", "B", 2, 1))
	tb.step(qabcMove("tB", 2, 1, 6, 1))
	tb.step(qabcTake("B", "x"))
	tb.step(qabcGive("B", "x"))
	if world := qabcHeld(t, tb.world, "B"); slices.Equal(world, qabcHeld(t, tb.viewer("A").shown, "B")) {
		t.Fatalf("the world's order %v equals the log's; the case needs another order", world)
	}
	got := tb.step(qabcMove("tB", 6, 1, 3, 1))["A"]
	if ab := qabcAbilityFrames(got, "B"); len(ab) != 0 {
		t.Fatalf("a take and a give back sent %v at sight; frames %s", ab, qabcDescribe(got))
	}
	qabcSameSet(t, "A after sight", tb.viewer("A").shown, tb.world, "B")
}

// VTT-724
func TestQAAbilityCorrectionAnIdHeldTwiceIsTakenOnceAndAnIdTheWorldHoldsTwiceGivenOnce(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y", "x")
	tb.step(qabcPlace("tB", "B", 2, 1))
	tb.step(qabcMove("tB", 2, 1, 6, 1))
	tb.step(qabcTake("B", "x"))
	sight := qabcMove("tB", 6, 1, 3, 1)
	got := tb.step(sight)["A"]
	if ab := qabcAbilityFrames(got, "B"); !slices.Equal(ab, []string{"taken:x"}) {
		t.Fatalf("A holding [x y x] against the world's [y] was sent %v, want [taken:x]", ab)
	}
	qabcSameSet(t, "A after the take", tb.viewer("A").shown, tb.world, "B")

	tb.add("E", "Eli", qabcParty, "p5", nil, nil, "m", "n", "m")
	got = tb.step(qabcPlace("tE", "E", 0, 0))["A"]
	if ab := qabcAbilityFrames(got, "E"); !slices.Equal(ab, []string{"given:m", "given:n"}) {
		t.Fatalf("A holding no list against the world's [m n m] was sent %v, want [given:m given:n]", ab)
	}
	qabcSameSet(t, "A after the gives", tb.viewer("A").shown, tb.world, "E")
}

// VTT-738
func TestQAAbilityCorrectionAListOnlyMemberFirstHeldUnseenIsBroughtByGives(t *testing.T) {
	tb := qabcNewTable(t, "A", "C")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("C", "Cy", qabcParty, "p3", nil, nil, "b", "a", "b")
	if held := qabcHeld(t, tb.viewer("A").shown, "C"); len(held) != 0 {
		t.Fatalf("A, which does not see C, holds its list %v", held)
	}
	tb.step(qabcPlace("tC", "C", 7, 1))
	unseen := qabcGive("C", "d")
	qabcNotForwarded(t, "A does not see C", tb.step(unseen)["A"], unseen)

	sight := qabcMove("tC", 7, 1, 2, 1)
	got := tb.step(sight)["A"]
	if ab := qabcAbilityFrames(got, "C"); !slices.Equal(ab, []string{"given:b", "given:a", "given:d"}) {
		t.Fatalf("A was sent %v at first sight of C, want [given:b given:a given:d]; frames %s", ab, qabcDescribe(got))
	}
	for _, f := range got {
		if qabcKind(f) == "AbilityGiven" {
			qabcBare(t, "a first-sight give", f, sight.GetSequence())
		}
	}
	for _, kind := range []string{"ActorRemoved", "ActorAdded"} {
		if n := qabcCount(got, kind, "C"); n != 0 {
			t.Fatalf("first sight of a list-only member sent %d %s; frames %s", n, kind, qabcDescribe(got))
		}
	}
	qabcSameSet(t, "A after first sight", tb.viewer("A").shown, tb.world, "C")
	qabcSameSet(t, "C's own log", tb.viewer("C").shown, tb.world, "C")
}

// VTT-740
func TestQAAbilityCorrectionAHeldMemberWithoutValuesIsWithdrawnOnlyAtItsFirstSight(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y")
	tb.step(qabcPlace("tB", "B", 7, 1))
	tb.step(qabcTake("B", "y"))
	if n := qabcCount(tb.step(qabcGive("B", "z"))["A"], "ActorRemoved", "B"); n != 0 {
		t.Fatalf("a give out of sight sent %d ActorRemoved", n)
	}
	got := tb.step(qabcMove("tB", 7, 1, 2, 1))["A"]
	if n := qabcCount(got, "ActorRemoved", "B"); n != 1 {
		t.Fatalf("first sight of B, held without its sheet, sent %d ActorRemoved, want 1; frames %s", n, qabcDescribe(got))
	}
	if n := qabcCount(got, "ActorAdded", "B"); n != 1 {
		t.Fatalf("first sight of B sent %d ActorAdded, want 1; frames %s", n, qabcDescribe(got))
	}
	if a := tb.viewer("A").shown.Actors["B"]; a.GetAttributes()["s"] != 2 {
		t.Fatalf("B introduced at first sight with attributes %v", a.GetAttributes())
	}
	qabcSameSet(t, "A after first sight", tb.viewer("A").shown, tb.world, "B")

	tb.step(qabcMove("tB", 2, 1, 7, 1))
	tb.step(qabcTake("B", "x"))
	tb.step(qabcGive("B", "y"))
	got = tb.step(qabcMove("tB", 7, 1, 2, 1))["A"]
	if n := qabcCount(got, "ActorRemoved", "B"); n != 0 {
		t.Fatalf("a second sight sent %d ActorRemoved; frames %s", n, qabcDescribe(got))
	}
	if ab := qabcAbilityFrames(got, "B"); !slices.Equal(ab, []string{"taken:x", "given:y"}) {
		t.Fatalf("second sight sent %v, want [taken:x given:y]", ab)
	}
}

// VTT-740
func TestQAAbilityCorrectionAListOnlyMemberAddedAgainWithValuesIsWithdrawnOnlyAtSight(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("E", "Eli", qabcParty, "p5", nil, nil, "a")
	tb.step(qabcPlace("tE", "E", 2, 1))
	if held := qabcHeld(t, tb.viewer("A").shown, "E"); !slices.Equal(held, []string{"a"}) {
		t.Fatalf("A sees E and holds %v, want [a]", held)
	}
	tb.step(qabcTokenRemoved("tE"))
	tb.step(qabcActorRemoved("E"))
	got := tb.add("E", "Eli", qabcParty, "p5", map[string]int32{"s": 4}, nil, "a", "q")["A"]
	if n := qabcCount(got, "ActorRemoved", "E"); n != 0 {
		t.Fatalf("E added again with its name and module id sent %d ActorRemoved before sight; frames %s", n, qabcDescribe(got))
	}
	got = tb.step(qabcPlace("tE", "E", 2, 1))["A"]
	if n := qabcCount(got, "ActorRemoved", "E"); n != 1 {
		t.Fatalf("first sight of E added again with values sent %d ActorRemoved, want 1; frames %s", n, qabcDescribe(got))
	}
	if a := tb.viewer("A").shown.Actors["E"]; a.GetAttributes()["s"] != 4 {
		t.Fatalf("E held with attributes %v after its first sight, want s=4", a.GetAttributes())
	}
	qabcSameSet(t, "A after first sight", tb.viewer("A").shown, tb.world, "E")
	got = tb.step(qabcGive("E", "r"))["A"]
	if n := qabcCount(got, "ActorRemoved", "E"); n != 0 {
		t.Fatalf("a give in sight after the introduction sent %d ActorRemoved", n)
	}
}

// VTT-740
func TestQAAbilityCorrectionAMemberAddedAgainUnderAnotherNameIsWithdrawnAtOnce(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("E", "Eli", qabcParty, "p5", nil, nil, "a")
	tb.step(qabcPlace("tE", "E", 2, 1))
	tb.step(qabcTokenRemoved("tE"))
	tb.step(qabcActorRemoved("E"))
	got := tb.add("E", "Other", qabcParty, "p5", nil, nil, "a")["A"]
	if n := qabcCount(got, "ActorRemoved", "E"); n != 1 {
		t.Fatalf("E added again under another name sent %d ActorRemoved at the addition, want 1; frames %s", n, qabcDescribe(got))
	}
	if a := tb.viewer("A").shown.Actors["E"]; a.GetName() != "Other" {
		t.Fatalf("A holds E as %q after the addition, want Other", a.GetName())
	}
}

// VTT-739
func TestQAAbilityCorrectionAMemberAddedAgainWithAnotherListIsCorrected(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	res := func() map[string]*vttv1.Resource { return map[string]*vttv1.Resource{"r": {Current: 4, Max: 6}} }
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, res(), "x", "y")
	tb.step(qabcPlace("tB", "B", 2, 1))
	tb.step(qabcTokenRemoved("tB"))
	tb.step(qabcActorRemoved("B"))
	removed, added := 0, 0
	got := tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, res(), "y", "w")["A"]
	removed += qabcCount(got, "ActorRemoved", "B")
	added += qabcCount(got, "ActorAdded", "B")
	got = tb.step(qabcPlace("tB", "B", 2, 1))["A"]
	removed += qabcCount(got, "ActorRemoved", "B")
	added += qabcCount(got, "ActorAdded", "B")
	if removed != 0 || added != 0 {
		t.Fatalf("B added again differing only in its list sent %d ActorRemoved and %d ActorAdded; frames %s",
			removed, added, qabcDescribe(got))
	}
	if ab := qabcAbilityFrames(got, "B"); !slices.Equal(ab, []string{"taken:x", "given:w"}) {
		t.Fatalf("B added again with [y w] over a held [x y] was sent %v at sight, want [taken:x given:w]", ab)
	}
	qabcSameSet(t, "A after sight", tb.viewer("A").shown, tb.world, "B")
}

// VTT-739
func TestQAAbilityCorrectionAMemberAddedAgainWithOtherAttributesIsIntroducedAfresh(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y")
	tb.step(qabcPlace("tB", "B", 2, 1))
	tb.step(qabcTokenRemoved("tB"))
	tb.step(qabcActorRemoved("B"))
	added := 0
	added += qabcCount(tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 5}, nil, "x", "y")["A"], "ActorAdded", "B")
	added += qabcCount(tb.step(qabcPlace("tB", "B", 2, 1))["A"], "ActorAdded", "B")
	if added == 0 {
		t.Fatalf("B added again with other attributes was never introduced afresh")
	}
	if a := tb.viewer("A").shown.Actors["B"]; a.GetAttributes()["s"] != 5 {
		t.Fatalf("A holds B with attributes %v in sight, want s=5", a.GetAttributes())
	}
	qabcSameSet(t, "A after sight", tb.viewer("A").shown, tb.world, "B")
}

// VTT-741
func TestQAAbilityCorrectionACreaturesListReachesNoLogAndArrivesWithoutIt(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("D", "Dax", qabcCreature, "", map[string]int32{"s": 7}, nil, "k", "l")
	tb.step(qabcPlace("tD", "D", 7, 1))
	tb.step(qabcGive("D", "m"))
	got := tb.step(qabcMove("tD", 7, 1, 2, 1))["A"]
	if n := qabcCount(got, "ActorAdded", "D"); n != 1 {
		t.Fatalf("D coming into sight was introduced %d times; frames %s", n, qabcDescribe(got))
	}
	for _, f := range got {
		if a := f.GetActorAdded().GetActor(); a != nil && a.GetActorId() == "D" && len(a.GetAbilityIds()) != 0 {
			t.Fatalf("D introduced with its list %v", a.GetAbilityIds())
		}
	}
	if ab := qabcAbilityFrames(got, "D"); len(ab) != 0 {
		t.Fatalf("D's introduction sent %v", ab)
	}
	for _, env := range []*vttv1.Envelope{qabcGive("D", "n"), qabcTake("D", "k")} {
		got = tb.step(env)["A"]
		qabcNotForwarded(t, "A sees creature D", got, env)
	}
	tb.step(qabcMove("tD", 2, 1, 7, 1))
	tb.step(qabcTake("D", "l"))
	got = tb.step(qabcMove("tD", 7, 1, 3, 1))["A"]
	if ab := qabcAbilityFrames(got, "D"); len(ab) != 0 {
		t.Fatalf("D coming back into sight sent %v", ab)
	}
	if held := qabcHeld(t, tb.viewer("A").shown, "D"); len(held) != 0 {
		t.Fatalf("A holds creature D's list %v", held)
	}
}

func qabcSeenAndReordered(t *testing.T) *qabcTable {
	t.Helper()
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y", "x", "z")
	tb.step(qabcPlace("tB", "B", 2, 1))
	tb.add("D", "Dax", qabcCreature, "", nil, nil, "k")
	tb.step(qabcPlace("tD", "D", 3, 0))
	tb.step(qabcMove("tB", 2, 1, 6, 1))
	tb.step(qabcTake("B", "x"))
	tb.step(qabcGive("B", "x"))
	tb.step(qabcMove("tB", 6, 1, 3, 1))
	return tb
}

// VTT-742
func TestQAAbilityCorrectionResumeResumesAFoldHoldingTheIdsInAnotherOrder(t *testing.T) {
	tb := qabcSeenAndReordered(t)
	v := tb.viewer("A")
	if slices.Equal(qabcHeld(t, v.shown, "B"), qabcHeld(t, tb.world, "B")) {
		t.Fatalf("the log holds B's ids in the world's order %v; the case needs another", qabcHeld(t, tb.world, "B"))
	}
	qabcSameSet(t, "before the resume", v.shown, tb.world, "B")
	v.pr = perceive.Resume(v.eyes, v.shown, tb.world)

	give := qabcGive("B", "n")
	got := tb.step(give)["A"]
	qabcForwarded(t, "a resumed log sees B", got, give)
	if ab := qabcAbilityFrames(got, "B"); !slices.Equal(ab, []string{"given:n"}) {
		t.Fatalf("a resumed log was sent %v, want the give alone", ab)
	}
	cond := qabcCondition("D", "c7")
	got = tb.step(cond)["A"]
	qabcForwarded(t, "a resumed log sees creature D, its list not held", got, cond)
}

// VTT-742
func TestQAAbilityCorrectionResumeStartsAfreshFromAFoldThatLacksAnId(t *testing.T) {
	tb := qabcSeenAndReordered(t)
	v := tb.viewer("A")
	tb.worldOnly(qabcGive("B", "n"))
	v.pr = perceive.Resume(v.eyes, v.shown, tb.world)

	narr := qabcNarrate("then")
	got := tb.step(narr)["A"]
	if ab := qabcAbilityFrames(got, "B"); !slices.Equal(ab, []string{"given:n"}) {
		t.Fatalf("a log lacking n was sent %v at the next event, want [given:n]; frames %s", ab, qabcDescribe(got))
	}
	for _, f := range got {
		if qabcKind(f) == "AbilityGiven" {
			qabcBare(t, "a fresh start's correction", f, narr.GetSequence())
		}
	}
	qabcForwarded(t, "the narration", got, narr)
	qabcSameSet(t, "after the correction", v.shown, tb.world, "B")
}

// VTT-742, VTT-741
func TestQAAbilityCorrectionResumeStartsAfreshFromAFoldThatNamesAnIdTheWorldDoesNot(t *testing.T) {
	tb := qabcSeenAndReordered(t)
	v := tb.viewer("A")
	tb.worldOnly(qabcTake("B", "z"))
	v.pr = perceive.Resume(v.eyes, v.shown, tb.world)

	give := qabcGive("B", "n")
	got := tb.step(give)["A"]
	for _, f := range got {
		if f.GetEventId() == give.GetEventId() {
			t.Fatalf("a log that lags B was forwarded the give as if it saw B; frames %s", qabcDescribe(got))
		}
	}
	if ab := qabcAbilityFrames(got, "B"); !slices.Equal(ab, []string{"taken:z", "given:n"}) {
		t.Fatalf("a log naming z was sent %v, want [taken:z given:n]; frames %s", ab, qabcDescribe(got))
	}
	qabcSameSet(t, "after the correction", v.shown, tb.world, "B")
}

func (tb *qabcTable) open(name string) {
	tb.t.Helper()
	shown := engine.NewState()
	opening := perceive.New(perceive.Eyes{}, nil)
	for i, f := range opening.Perch(perceive.Eyes{}, shown, tb.world, 0) {
		if err := engine.Apply(shown, f); err != nil {
			tb.t.Fatalf("opening frame %d (%s) does not fold: %v", i, qabcKind(f), err)
		}
	}
	eyes := perceive.Eyes{Shoulder: name}
	tb.viewers = append(tb.viewers, &qabcViewer{name: name, eyes: eyes, pr: perceive.New(eyes, nil), shown: shown})
}

// VTT-738
func TestQAAbilityCorrectionALogOpenedForAListOnlyMemberBringsItsOwnListByGives(t *testing.T) {
	tb := qabcNewTable(t)
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("C", "Cy", qabcParty, "p3", nil, nil, "b", "a")
	tb.step(qabcPlace("tC", "C", 2, 1))
	tb.open("C")
	if held := qabcHeld(t, tb.viewer("C").shown, "C"); len(held) != 0 {
		t.Fatalf("the opening introduced C with its list %v", held)
	}
	got := tb.step(qabcNarrate("begin"))["C"]
	if ab := qabcAbilityFrames(got, "C"); !slices.Equal(ab, []string{"given:b", "given:a"}) {
		t.Fatalf("C's own log was sent %v of C, want [given:b given:a]; frames %s", ab, qabcDescribe(got))
	}
	for _, kind := range []string{"ActorRemoved", "ActorAdded"} {
		if n := qabcCount(got, kind, "C"); n != 0 {
			t.Fatalf("C's own log was sent %d %s of C; frames %s", n, kind, qabcDescribe(got))
		}
	}
	if n := qabcCount(got, "ActorAdded", "A"); n != 1 {
		t.Fatalf("C's log, seeing A with values for the first time, was sent %d ActorAdded of A; frames %s", n, qabcDescribe(got))
	}
	qabcSameSet(t, "C's own log", tb.viewer("C").shown, tb.world, "C")
}

func qabcKindGrant(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	g := qabcGrant(actor, participant)
	g.GetActorControlGranted().Kind = kind
	return g
}

// VTT-741
func TestQAAbilityCorrectionAMemberMadeACreatureOutOfSightIsSentNoTakeAtSight(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y")
	tb.step(qabcPlace("tB", "B", 2, 1))
	tb.step(qabcMove("tB", 2, 1, 6, 1))
	tb.step(qabcKindGrant("B", "p2", qabcCreature))
	give := qabcGive("B", "w")
	qabcNotForwarded(t, "A does not see creature B", tb.step(give)["A"], give)
	got := tb.step(qabcMove("tB", 6, 1, 3, 1))["A"]
	if ab := qabcAbilityFrames(got, "B"); len(ab) != 0 {
		t.Fatalf("B, a creature now, brought into sight sent %v; frames %s", ab, qabcDescribe(got))
	}
	b := tb.viewer("A").shown.Actors["B"]
	if b.GetKind() != qabcCreature || len(b.GetAbilityIds()) != 0 {
		t.Fatalf("A holds B as kind %v with list %v, want a creature without its list", b.GetKind(), b.GetAbilityIds())
	}
}

// VTT-741
func TestQAAbilityCorrectionAMemberMadeACreatureInSightIsSentNoFurtherGiveOrTake(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "x", "y")
	tb.step(qabcPlace("tB", "B", 2, 1))
	got := tb.step(qabcKindGrant("B", "p2", qabcCreature))["A"]
	if ab := qabcAbilityFrames(got, "B"); len(ab) != 0 {
		t.Fatalf("B made a creature in sight sent %v; frames %s", ab, qabcDescribe(got))
	}
	for _, env := range []*vttv1.Envelope{qabcGive("B", "w"), qabcTake("B", "x")} {
		qabcNotForwarded(t, "A sees creature B", tb.step(env)["A"], env)
	}
	if held := qabcHeld(t, tb.viewer("A").shown, "B"); len(held) != 0 {
		t.Fatalf("A holds creature B's list %v", held)
	}
}

// VTT-738
func TestQAAbilityCorrectionACreatureMadeAMemberOutOfSightIsSeenWithItsList(t *testing.T) {
	tb := qabcNewTable(t, "A")
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("D", "Dax", qabcCreature, "", nil, nil, "k", "l")
	tb.step(qabcPlace("tD", "D", 2, 1))
	tb.step(qabcMove("tD", 2, 1, 6, 1))
	tb.step(qabcKindGrant("D", "p4", qabcParty))
	give := qabcGive("D", "m")
	qabcNotForwarded(t, "A does not see member D", tb.step(give)["A"], give)
	tb.step(qabcTake("D", "k"))
	tb.step(qabcMove("tD", 6, 1, 3, 1))
	qabcSameSet(t, "A after sight of member D", tb.viewer("A").shown, tb.world, "D")
	take := qabcTake("D", "l")
	got := tb.step(take)["A"]
	qabcForwarded(t, "A sees member D", got, take)
}

// VTT-742
func TestQAAbilityCorrectionResumeStartsAfreshFromAnOpeningThatHoldsAListOnlyMemberWithoutItsList(t *testing.T) {
	tb := qabcNewTable(t)
	tb.step(qabcScene())
	tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil)
	tb.step(qabcPlace("tA", "A", 1, 1))
	tb.add("C", "Cy", qabcParty, "p3", nil, nil, "b", "a")
	tb.step(qabcPlace("tC", "C", 2, 1))
	tb.open("C")
	v := tb.viewer("C")
	v.pr = perceive.Resume(v.eyes, v.shown, tb.world)
	got := tb.step(qabcNarrate("begin"))["C"]
	if ab := qabcAbilityFrames(got, "C"); !slices.Equal(ab, []string{"given:b", "given:a"}) {
		t.Fatalf("a resume over a fold holding C without its list sent %v, want [given:b given:a]; frames %s",
			ab, qabcDescribe(got))
	}
	qabcSameSet(t, "C's own log", v.shown, tb.world, "C")
}

type qabcWalk struct {
	t       *testing.T
	tb      *qabcTable
	pos     map[string][2]int32
	members map[string]bool
	valued  map[string]bool
	seen    map[string]bool
	fixes   *int
}

func (w *qabcWalk) randomEvent(r *rand.Rand) *vttv1.Envelope {
	actors := []string{"A", "B", "C", "D"}
	switch r.IntN(3) {
	case 0:
		tok := "t" + actors[r.IntN(len(actors))]
		from := w.pos[tok]
		var to [2]int32
		for {
			to = [2]int32{int32(r.IntN(9)), int32(r.IntN(3))}
			if to[0] != 4 && to != from {
				break
			}
		}
		w.pos[tok] = to
		return qabcMove(tok, from[0], from[1], to[0], to[1])
	case 1:
		actor := actors[r.IntN(len(actors))]
		held := w.tb.world.Actors[actor].GetAbilityIds()
		for _, id := range r.Perm(6) {
			ability := string(rune('a' + id))
			if !slices.Contains(held, ability) {
				return qabcGive(actor, ability)
			}
		}
	default:
		actor := actors[r.IntN(len(actors))]
		if held := w.tb.world.Actors[actor].GetAbilityIds(); len(held) > 0 {
			return qabcTake(actor, held[r.IntN(len(held))])
		}
	}
	return qabcNarrate("pause")
}

func (w *qabcWalk) check(env *vttv1.Envelope, before map[string]bool, got map[string][]*vttv1.Envelope) {
	t := w.t
	t.Helper()
	subject := qabcActorOf(env)
	isChange := qabcKind(env) == "AbilityGiven" || qabcKind(env) == "AbilityTaken"
	for _, v := range w.tb.viewers {
		frames := got[v.name]
		for _, f := range frames {
			k := qabcKind(f)
			x := qabcActorOf(f)
			if (k == "AbilityGiven" || k == "AbilityTaken") && !w.members[x] {
				t.Fatalf("event %d: log %s sent %s of creature %s", env.Sequence, v.name, k, x)
			}
			if (k == "AbilityGiven" || k == "AbilityTaken") && f.GetEventId() == "" {
				*w.fixes++
			}
			if k == "ActorRemoved" || k == "ActorAdded" {
				first := !w.seen[v.name+"/"+x] && w.valued[x]
				if isChange || !first {
					t.Fatalf("event %d (%s): log %s sent %s of %s (first sight with values: %v); frames %s",
						env.Sequence, qabcKind(env), v.name, k, x, first, qabcDescribe(frames))
				}
			}
		}
		if isChange && w.members[subject] {
			forwarded := false
			for _, f := range frames {
				if f.GetEventId() == env.GetEventId() {
					forwarded = true
				}
			}
			if forwarded != before[v.name] {
				t.Fatalf("event %d (%s of %s): log %s forwarded=%v, saw before=%v; frames %s",
					env.Sequence, qabcKind(env), subject, v.name, forwarded, before[v.name], qabcDescribe(frames))
			}
		}
		for x := range w.tb.world.Actors {
			if !perceive.SeesActor(v.eyes, w.tb.world, x) {
				continue
			}
			w.seen[v.name+"/"+x] = true
			if w.members[x] {
				qabcSameSet(t, fmt.Sprintf("event %d, log %s, in sight", env.Sequence, v.name), v.shown, w.tb.world, x)
			} else if held := qabcHeld(t, v.shown, x); len(held) != 0 {
				t.Fatalf("event %d: log %s holds creature %s's list %v", env.Sequence, v.name, x, held)
			}
		}
	}
}

// VTT-724
func TestQAAbilityCorrectionAWalkKeepsEveryLogInSightOfEachListAsASet(t *testing.T) {
	fixes := 0
	for seed := uint64(1); seed <= 40; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			r := rand.New(rand.NewPCG(seed, 7))
			tb := qabcNewTable(t, "A", "B", "C")
			tb.step(qabcScene())
			tb.add("A", "Ada", qabcParty, "p1", map[string]int32{"s": 1}, nil, "a")
			tb.add("B", "Bo", qabcParty, "p2", map[string]int32{"s": 2}, nil, "b", "c", "b")
			tb.add("C", "Cy", qabcParty, "p3", nil, nil, "d", "a")
			tb.add("D", "Dax", qabcCreature, "", map[string]int32{"s": 3}, nil, "e")
			w := &qabcWalk{
				t: t, tb: tb, pos: map[string][2]int32{},
				members: map[string]bool{"A": true, "B": true, "C": true},
				valued:  map[string]bool{"A": true, "B": true, "D": true},
				seen:    map[string]bool{},
				fixes:   &fixes,
			}
			for _, p := range []struct {
				tok, actor string
				x, y       int32
			}{{"tA", "A", 1, 1}, {"tB", "B", 7, 1}, {"tC", "C", 6, 0}, {"tD", "D", 2, 2}} {
				env := qabcPlace(p.tok, p.actor, p.x, p.y)
				w.pos[p.tok] = [2]int32{p.x, p.y}
				before := w.sawBefore(p.actor)
				w.check(env, before, tb.step(env))
			}
			for i := 0; i < 250; i++ {
				env := w.randomEvent(r)
				before := w.sawBefore(qabcActorOf(env))
				w.check(env, before, tb.step(env))
			}
		})
	}
	if fixes == 0 {
		t.Fatalf("no walk reached a sight correction of a list")
	}
}

func (w *qabcWalk) sawBefore(actor string) map[string]bool {
	out := map[string]bool{}
	for _, v := range w.tb.viewers {
		out[v.name] = actor != "" && w.tb.world.Actors[actor] != nil && perceive.SeesActor(v.eyes, w.tb.world, actor)
	}
	return out
}
