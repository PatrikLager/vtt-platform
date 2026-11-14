package perceive_test

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const qaFwWorld = `[
  {"eventId": "w1", "sequence": "1", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sessionStarted": {"name": "Night"}},
  {"eventId": "w2", "sequence": "2", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sceneCreated": {"sceneId": "scn-1", "name": "Hall", "gridWidth": 7, "gridHeight": 1,
      "tiles": {"0,0": {"kind": "floor"}, "1,0": {"kind": "floor"}, "2,0": {"kind": "floor"},
        "3,0": {"kind": "wall"}, "4,0": {"kind": "floor"}, "5,0": {"kind": "floor"}, "6,0": {"kind": "floor"}}}},
  {"eventId": "w3", "sequence": "3", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "sceneCreated": {"sceneId": "scn-2", "name": "Cellar", "gridWidth": 2, "gridHeight": 1}},
  {"eventId": "w4", "sequence": "4", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-a", "name": "Aster", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "attributes": {"might": 3}, "abilityIds": ["dash", "parry", "dash"]}}},
  {"eventId": "w5", "sequence": "5", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-b", "name": "Bram", "kind": "ACTOR_KIND_PARTY_MEMBER",
      "abilityIds": ["parry"]}}},
  {"eventId": "w6", "sequence": "6", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-d", "name": "Dain", "kind": "ACTOR_KIND_PARTY_MEMBER"}}},
  {"eventId": "w7", "sequence": "7", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorAdded": {"actor": {"actorId": "act-c", "name": "Crag", "kind": "ACTOR_KIND_NON_PARTY",
      "abilityIds": ["claw", "claw"]}}},
  {"eventId": "w8", "sequence": "8", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "actorControlGranted": {"actorId": "act-a", "participantId": "p-a", "kind": "ACTOR_KIND_PARTY_MEMBER"}},
  {"eventId": "w9", "sequence": "9", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-a", "sceneId": "scn-1", "actorId": "act-a", "position": {"x": 0, "y": 0}}},
  {"eventId": "w10", "sequence": "10", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-c", "sceneId": "scn-1", "actorId": "act-c", "position": {"x": 1, "y": 0}}},
  {"eventId": "w11", "sequence": "11", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-b", "sceneId": "scn-1", "actorId": "act-b", "position": {"x": 2, "y": 0}}},
  {"eventId": "w12", "sequence": "12", "sessionId": "sess-1", "actorRole": "dm", "participantId": "p-dm",
    "tokenPlaced": {"tokenId": "tok-d", "sceneId": "scn-2", "actorId": "act-d", "position": {"x": 0, "y": 0}}}
]`

const qaFwMoveAway = `[
  {"eventId": "w13", "sequence": "13", "sessionId": "sess-1", "actorRole": "player", "participantId": "p-a",
    "tokenMoved": {"tokenId": "tok-a", "sceneId": "scn-1", "from": {"x": 0, "y": 0}, "to": {"x": 5, "y": 0}}}
]`

type qaFwViewer struct {
	eyes  perceive.Eyes
	pr    *perceive.Projection
	shown *engine.State
}

type qaFwTable struct {
	world   *engine.State
	viewers map[string]*qaFwViewer
}

func qaFwParse(t *testing.T, raw string) []*vttv1.Envelope {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("log: %v", err)
	}
	out := make([]*vttv1.Envelope, 0, len(items))
	for i, it := range items {
		env := &vttv1.Envelope{}
		if err := protojson.Unmarshal(it, env); err != nil {
			t.Fatalf("entry %d: %v", i, err)
		}
		out = append(out, env)
	}
	return out
}

func qaFwOpen(t *testing.T, logs ...string) *qaFwTable {
	t.Helper()
	tb := &qaFwTable{world: engine.NewState(), viewers: map[string]*qaFwViewer{}}
	for _, id := range []string{"act-a", "act-b", "act-d"} {
		eyes := perceive.Eyes{Shoulder: id}
		tb.viewers[id] = &qaFwViewer{eyes: eyes, pr: perceive.New(eyes, nil), shown: engine.NewState()}
	}
	for _, raw := range logs {
		for _, env := range qaFwParse(t, raw) {
			tb.step(t, env, true)
		}
	}
	return tb
}

func (tb *qaFwTable) step(t *testing.T, env *vttv1.Envelope, fold bool) map[string][]*vttv1.Envelope {
	t.Helper()
	if err := engine.Apply(tb.world, env); err != nil {
		t.Fatalf("world refused %s: %v", env.GetEventId(), err)
	}
	sent := map[string][]*vttv1.Envelope{}
	for id, v := range tb.viewers {
		frames := v.pr.Event(v.eyes, env, v.shown, tb.world)
		sent[id] = frames
		if !fold {
			continue
		}
		for _, f := range frames {
			if err := engine.Apply(v.shown, f); err != nil {
				t.Fatalf("log of %s refused a frame of %s: %v", id, env.GetEventId(), err)
			}
		}
	}
	return sent
}

func qaFwChange(give bool, actor, ability string) *vttv1.Envelope {
	env := &vttv1.Envelope{
		EventId: "evt-change", Sequence: 40, SessionId: "sess-1", ActorRole: "dm", ParticipantId: "p-dm",
		OccurredAt: &timestamppb.Timestamp{Seconds: 1794614400},
	}
	if give {
		env.Payload = &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{ActorId: actor, AbilityId: ability}}
	} else {
		env.Payload = &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{ActorId: actor, AbilityId: ability}}
	}
	return env
}

func qaFwChanges(frames []*vttv1.Envelope) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range frames {
		switch f.GetPayload().(type) {
		case *vttv1.Envelope_AbilityGiven, *vttv1.Envelope_AbilityTaken:
			out = append(out, f)
		}
	}
	return out
}

func qaFwCheckSent(t *testing.T, viewer string, frames []*vttv1.Envelope, env *vttv1.Envelope) {
	t.Helper()
	got := qaFwChanges(frames)
	if len(got) != 1 {
		t.Fatalf("log of %s was sent %d copies of the change, want 1: %v", viewer, len(got), frames)
	}
	want := proto.Clone(env).(*vttv1.Envelope)
	want.ParticipantId = ""
	want.ActorRole = ""
	if !proto.Equal(got[0], want) {
		t.Errorf("log of %s was sent %v, want %v", viewer, got[0], want)
	}
}

func qaFwCheckNone(t *testing.T, viewer string, frames []*vttv1.Envelope) {
	t.Helper()
	if got := qaFwChanges(frames); len(got) != 0 {
		t.Errorf("log of %s was sent %v, want no change", viewer, got)
	}
}

func qaFwSight(t *testing.T, tb *qaFwTable, viewer, actor string, want bool) {
	t.Helper()
	if got := perceive.SeesActor(perceive.Eyes{Shoulder: viewer}, tb.world, actor); got != want {
		t.Fatalf("fixture: %s sees %s = %v, want %v", viewer, actor, got, want)
	}
}

// VTT-741
func TestQAAPartyMembersChangeReachesTheLogsThatSeeIt(t *testing.T) {
	for _, give := range []bool{true, false} {
		ability := "dash"
		if give {
			ability = "lunge"
		}
		tb := qaFwOpen(t, qaFwWorld)
		qaFwSight(t, tb, "act-b", "act-a", true)
		qaFwSight(t, tb, "act-d", "act-a", false)
		env := qaFwChange(give, "act-a", ability)
		pristine := proto.Clone(env)
		sent := tb.step(t, env, true)
		qaFwCheckSent(t, "act-a", sent["act-a"], env)
		qaFwCheckSent(t, "act-b", sent["act-b"], env)
		qaFwCheckNone(t, "act-d", sent["act-d"])
		if !proto.Equal(env, pristine) {
			t.Errorf("the event was written to: %v, was %v", env, pristine)
		}
	}
}

// VTT-741
func TestQAACreaturesChangeReachesNoLogThatSeesIt(t *testing.T) {
	for _, give := range []bool{true, false} {
		ability := "claw"
		if give {
			ability = "bite"
		}
		tb := qaFwOpen(t, qaFwWorld)
		qaFwSight(t, tb, "act-a", "act-c", true)
		qaFwSight(t, tb, "act-b", "act-c", true)
		sent := tb.step(t, qaFwChange(give, "act-c", ability), true)
		for _, id := range []string{"act-a", "act-b", "act-d"} {
			qaFwCheckNone(t, id, sent[id])
		}
	}
}

// VTT-741
func TestQAAChangeOutOfSightReachesOnlyTheMembersOwnLog(t *testing.T) {
	for _, give := range []bool{true, false} {
		ability := "parry"
		if give {
			ability = "lunge"
		}
		tb := qaFwOpen(t, qaFwWorld, qaFwMoveAway)
		qaFwSight(t, tb, "act-b", "act-a", false)
		qaFwSight(t, tb, "act-a", "act-a", true)
		env := qaFwChange(give, "act-a", ability)
		sent := tb.step(t, env, true)
		qaFwCheckSent(t, "act-a", sent["act-a"], env)
		qaFwCheckNone(t, "act-b", sent["act-b"])
		qaFwCheckNone(t, "act-d", sent["act-d"])
	}
}

// VTT-741
func TestQAAChangeOfAnotherMemberReachesTheLogThatSeesIt(t *testing.T) {
	for _, give := range []bool{true, false} {
		ability := "parry"
		if give {
			ability = "dash"
		}
		tb := qaFwOpen(t, qaFwWorld)
		qaFwSight(t, tb, "act-a", "act-b", true)
		env := qaFwChange(give, "act-b", ability)
		sent := tb.step(t, env, true)
		qaFwCheckSent(t, "act-a", sent["act-a"], env)
		qaFwCheckSent(t, "act-b", sent["act-b"], env)
		qaFwCheckNone(t, "act-d", sent["act-d"])
	}
}
