package campaign_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

type qabrTable struct {
	t   *testing.T
	dir string
	c   *campaign.Campaign
	n   int
}

func qabrOpen(t *testing.T) *qabrTable {
	t.Helper()
	tb := &qabrTable{t: t, dir: t.TempDir()}
	tb.reopen()
	return tb
}

func (tb *qabrTable) reopen() {
	tb.t.Helper()
	if tb.c != nil {
		if err := tb.c.Close(); err != nil {
			tb.t.Fatalf("close: %v", err)
		}
	}
	c, err := campaign.Open(tb.dir)
	if err != nil {
		tb.t.Fatalf("open: %v", err)
	}
	tb.c = c
	tb.t.Cleanup(func() { _ = c.Close() })
}

func (tb *qabrTable) append(env *vttv1.Envelope) *vttv1.Envelope {
	tb.t.Helper()
	tb.n++
	env.EventId = fmt.Sprintf("qabr-%d", tb.n)
	env.ActorRole = "dm"
	env.ParticipantId = "p-dm"
	seq, err := tb.c.Append(env)
	if err != nil {
		tb.t.Fatalf("append %d (%T): %v", tb.n, env.GetPayload(), err)
	}
	env.Sequence = seq
	return env
}

func (tb *qabrTable) log(actor string) []campaign.CharacterEntry {
	tb.t.Helper()
	entries, err := tb.c.CharacterLog(actor)
	if err != nil {
		tb.t.Fatalf("character log %s: %v", actor, err)
	}
	return entries
}

func (tb *qabrTable) fold(actor string) *engine.State {
	tb.t.Helper()
	st := engine.NewState()
	for i, e := range tb.log(actor) {
		if err := engine.Apply(st, e.Env); err != nil {
			tb.t.Fatalf("log %s entry %d (%T) does not fold: %v", actor, i, e.Env.GetPayload(), err)
		}
	}
	return st
}

func (tb *qabrTable) caused(actor string, worldSeq int64) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, e := range tb.log(actor) {
		if e.WorldSeq == worldSeq {
			out = append(out, e.Env)
		}
	}
	return out
}

func qabrKind(f *vttv1.Envelope) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", f.GetPayload()), "*vttv1.Envelope_")
}

func qabrDescribe(frames []*vttv1.Envelope) string {
	var parts []string
	for _, f := range frames {
		parts = append(parts, qabrKind(f)+"["+f.GetEventId()+"]")
	}
	return strings.Join(parts, " ")
}

func qabrSet(ids []string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return slices.Compact(out)
}

func qabrScene() *vttv1.Envelope {
	wall := &vttv1.TileRef{Kind: "wall"}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "s1", Name: "Hall", GridWidth: 9, GridHeight: 3,
		Tiles: map[string]*vttv1.TileRef{"4,0": wall, "4,1": wall, "4,2": wall},
	}}}
}

func (tb *qabrTable) addMember(id, name, controller string, attrs map[string]int32, list ...string) {
	tb.t.Helper()
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: id, Name: name, ModuleId: "mod-q", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		Attributes: attrs, AbilityIds: slices.Clone(list),
	}}}})
	tb.append(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: id, ParticipantId: controller, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}}})
}

func qabrPlace(token, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: token, SceneId: "s1", ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y},
	}}}
}

func qabrMove(token string, fx, fy, tx, ty int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: token, SceneId: "s1",
		From: &vttv1.GridPosition{X: fx, Y: fy}, To: &vttv1.GridPosition{X: tx, Y: ty},
	}}}
}

func qabrGive(actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{
		ActorId: actor, AbilityId: ability,
	}}}
}

func qabrTake(actor, ability string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{
		ActorId: actor, AbilityId: ability,
	}}}
}

func qabrCondition(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cond,
	}}}
}

func qabrForwarded(t *testing.T, label string, frames []*vttv1.Envelope, env *vttv1.Envelope) {
	t.Helper()
	n := 0
	for _, f := range frames {
		if f.GetEventId() == env.GetEventId() && qabrKind(f) == qabrKind(env) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%s: %d entries forward event %s (%s); entries %s",
			label, n, env.GetEventId(), qabrKind(env), qabrDescribe(frames))
	}
	for _, f := range frames {
		if f.GetEventId() == "" && (qabrKind(f) == "AbilityGiven" || qabrKind(f) == "AbilityTaken") {
			t.Fatalf("%s: a correction beside the forwarded event, as after a fresh start; entries %s",
				label, qabrDescribe(frames))
		}
	}
}

func (tb *qabrTable) sameSet(label, logActor, actor string) {
	tb.t.Helper()
	held := tb.fold(logActor).Actors[actor].GetAbilityIds()
	world := tb.c.State().Actors[actor].GetAbilityIds()
	if !slices.Equal(qabrSet(held), qabrSet(world)) {
		tb.t.Fatalf("%s: log %s holds %s's ids %v, the world %v", label, logActor, actor, held, world)
	}
}

func qabrHall(t *testing.T) *qabrTable {
	t.Helper()
	tb := qabrOpen(t)
	tb.append(qabrScene())
	tb.addMember("A", "Ada", "p1", map[string]int32{"s": 1})
	tb.append(qabrPlace("tA", "A", 1, 1))
	tb.addMember("B", "Bo", "p2", map[string]int32{"s": 2}, "z", "y", "x")
	tb.append(qabrPlace("tB", "B", 2, 1))
	return tb
}

// VTT-742
func TestQAAbilityReopenAfterAGiveResumesTheLogs(t *testing.T) {
	tb := qabrHall(t)
	give := tb.append(qabrGive("B", "n"))
	qabrForwarded(t, "A sees B before the reopen", tb.caused("A", give.GetSequence()), give)
	tb.sameSet("before the reopen", "A", "B")

	tb.reopen()
	give = tb.append(qabrGive("B", "m"))
	qabrForwarded(t, "A's log after the reopen", tb.caused("A", give.GetSequence()), give)
	qabrForwarded(t, "B's log after the reopen", tb.caused("B", give.GetSequence()), give)
	tb.sameSet("after the reopen", "A", "B")
	tb.sameSet("B's own log after the reopen", "B", "B")
}

// VTT-742
func TestQAAbilityReopenResumesALogHoldingTheIdsInAnotherOrder(t *testing.T) {
	tb := qabrHall(t)
	tb.append(qabrMove("tB", 2, 1, 6, 1))
	tb.append(qabrTake("B", "z"))
	tb.append(qabrGive("B", "z"))
	sight := tb.append(qabrMove("tB", 6, 1, 3, 1))
	for _, f := range tb.caused("A", sight.GetSequence()) {
		if k := qabrKind(f); k == "AbilityGiven" || k == "AbilityTaken" || k == "ActorRemoved" {
			t.Fatalf("a take and a give back sent %s at sight; entries %s",
				k, qabrDescribe(tb.caused("A", sight.GetSequence())))
		}
	}
	held := tb.fold("A").Actors["B"].GetAbilityIds()
	if slices.Equal(held, tb.c.State().Actors["B"].GetAbilityIds()) {
		t.Fatalf("A's log holds B's ids %v in the world's order; the case needs another", held)
	}
	tb.sameSet("before the reopen", "A", "B")

	tb.reopen()
	give := tb.append(qabrGive("B", "n"))
	qabrForwarded(t, "A's log resumed over another order", tb.caused("A", give.GetSequence()), give)
	tb.sameSet("after the reopen", "A", "B")
}

// VTT-742
func TestQAAbilityReopenResumesALogThatLagsOnlyAMemberOutOfSight(t *testing.T) {
	tb := qabrHall(t)
	tb.append(qabrMove("tB", 2, 1, 6, 1))
	tb.append(qabrGive("B", "w"))
	tb.append(qabrTake("B", "y"))

	tb.reopen()
	cond := tb.append(qabrCondition("A", "c1"))
	qabrForwarded(t, "A's own condition after the reopen", tb.caused("A", cond.GetSequence()), cond)

	sight := tb.append(qabrMove("tB", 6, 1, 3, 1))
	var ab []string
	for _, f := range tb.caused("A", sight.GetSequence()) {
		switch p := f.GetPayload().(type) {
		case *vttv1.Envelope_AbilityGiven:
			ab = append(ab, "given:"+p.AbilityGiven.GetAbilityId())
		case *vttv1.Envelope_AbilityTaken:
			ab = append(ab, "taken:"+p.AbilityTaken.GetAbilityId())
		case *vttv1.Envelope_ActorRemoved, *vttv1.Envelope_ActorAdded:
			t.Fatalf("sight after the reopen sent %s; entries %s",
				qabrKind(f), qabrDescribe(tb.caused("A", sight.GetSequence())))
		}
	}
	if !slices.Equal(ab, []string{"taken:y", "given:w"}) {
		t.Fatalf("sight after the reopen sent %v, want [taken:y given:w]", ab)
	}
	tb.sameSet("after sight", "A", "B")
}
