package gateway_test

import (
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/sight"
)

const qaStripID = "strip"

var (
	qaDM    = gateway.Viewer{ParticipantID: "dm", Role: identity.RoleDM}
	qaAgent = gateway.Viewer{ParticipantID: "agent", Role: identity.RoleAgent}
)

func qaSpectatorOn(actorID string) gateway.Viewer {
	return gateway.Viewer{ParticipantID: "s-" + actorID, Role: identity.RoleSpectator, Viewpoint: actorID}
}

type qaEyesSeat struct {
	v    gateway.Viewer
	pr   *gateway.Projector
	fold *engine.State
	sent []*vttv1.Envelope
}

type qaEyesTable struct {
	world *engine.State
	seq   int64
	order []string
	seats map[string]*qaEyesSeat
}

func newQAEyesTable(viewers map[string]gateway.Viewer) *qaEyesTable {
	tt := &qaEyesTable{world: engine.NewState(), seats: map[string]*qaEyesSeat{}}
	for name, v := range viewers {
		tt.order = append(tt.order, name)
		tt.seats[name] = &qaEyesSeat{v: v, pr: gateway.NewProjector(v), fold: engine.NewState()}
	}
	sort.Strings(tt.order)
	return tt
}

func (tt *qaEyesTable) apply(tb testing.TB, envs ...*vttv1.Envelope) {
	tb.Helper()
	for _, env := range envs {
		tt.seq++
		env.Sequence = tt.seq
		if err := engine.Apply(tt.world, proto.Clone(env).(*vttv1.Envelope)); err != nil {
			tb.Fatalf("fixture: the log refused event %d: %v", env.GetSequence(), err)
		}
		for _, name := range tt.order {
			tt.project(tb, name, env)
		}
	}
}

func (tt *qaEyesTable) project(tb testing.TB, name string, env *vttv1.Envelope) {
	tb.Helper()
	s := tt.seats[name]
	want := proto.Clone(env)
	var frames []*vttv1.Envelope
	switch s.v.Role {
	case identity.RoleDM, identity.RoleAgent:
		frames = s.pr.Project(env, nil)
		if len(frames) != 1 || frames[0] != env {
			tb.Fatalf("VTT-176: %s was not sent event %d itself: %d frames", name, env.GetSequence(), len(frames))
		}
	default:
		st := tt.world.Snapshot()
		frames = s.pr.Project(env, st)
		qaEyesUnchanged(tb, name, st, tt.world)
	}
	if !proto.Equal(want, env) {
		tb.Fatalf("VTT-222: projecting event %d for %s changed the event", env.GetSequence(), name)
	}
	for _, f := range frames {
		if f.GetSequence() != env.GetSequence() {
			tb.Errorf("VTT-221: %s was sent a frame of event %d with sequence %d", name, env.GetSequence(), f.GetSequence())
		}
		if err := engine.Apply(s.fold, proto.Clone(f).(*vttv1.Envelope)); err != nil {
			tb.Fatalf("VTT-224: %s's stream does not fold at event %d: %v", name, env.GetSequence(), err)
		}
		s.sent = append(s.sent, f)
	}
}

func qaEyesUnchanged(tb testing.TB, name string, got, want *engine.State) {
	tb.Helper()
	if len(got.Actors) != len(want.Actors) || len(got.Tokens) != len(want.Tokens) || len(got.Scenes) != len(want.Scenes) {
		tb.Fatalf("VTT-222: projecting for %s changed the state's size", name)
	}
	for id, a := range want.Actors {
		if !proto.Equal(a, got.Actors[id]) {
			tb.Fatalf("VTT-222: projecting for %s changed actor %s to %v", name, id, got.Actors[id])
		}
	}
	for id, tok := range want.Tokens {
		if got.Tokens[id] != tok {
			tb.Fatalf("VTT-222: projecting for %s changed token %s", name, id)
		}
	}
}

func qaEyesHolds(tb testing.TB, tt *qaEyesTable, name, actorID, rule string) *vttv1.Actor {
	tb.Helper()
	a := tt.seats[name].fold.Actors[actorID]
	if a == nil {
		tb.Fatalf("%s: %s was never introduced to %s", rule, name, actorID)
	}
	return a
}

func qaEyesOnBoard(tb testing.TB, tt *qaEyesTable, name, tokenID string, want bool, rule string) {
	tb.Helper()
	if _, on := tt.seats[name].fold.Tokens[tokenID]; on != want {
		tb.Errorf("%s: %s has token %s on its board: %v, want %v", rule, name, tokenID, on, want)
	}
}

func qaStrip() *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: qaStripID, Name: "Strip", GridWidth: 5, GridHeight: 1,
		Tiles: map[string]*vttv1.TileRef{"2,0": {Kind: "wall"}},
	}}}
}

func qaEyesStripGuard(tb testing.TB) {
	tb.Helper()
	st := engine.NewState()
	if err := engine.Apply(st, qaStrip()); err != nil {
		tb.Fatalf("fixture: %v", err)
	}
	near := sight.VisibleFrom(st.Scenes[qaStripID], 0, 0, 0, 0)
	far := sight.VisibleFrom(st.Scenes[qaStripID], 4, 0, 0, 0)
	if !near["0,0"] || !near["1,0"] || near["3,0"] || near["4,0"] || far["0,0"] || far["1,0"] || !far["3,0"] || !far["4,0"] {
		tb.Fatalf("fixture: the wall does not split the strip: from 0,0 %v, from 4,0 %v", near, far)
	}
}

func qaActor(id string, kind vttv1.ActorKind) *vttv1.Envelope {
	a := &vttv1.Actor{ActorId: id, Name: "Actor " + id, Kind: kind}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qaGrant(actorID, pid string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actorID, ParticipantId: pid, Kind: kind,
	}}}
}

func qaRevoke(actorID, pid string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: actorID, ParticipantId: pid,
	}}}
}

func qaPlace(tokenID, actorID string, x int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tokenID, SceneId: qaStripID, ActorId: actorID, Position: &vttv1.GridPosition{X: x},
	}}}
}

func qaWithP2OnTheNearEnd() []*vttv1.Envelope {
	return []*vttv1.Envelope{
		qaStrip(),
		qaActor("pc2", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaGrant("pc2", "p2", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaPlace("t-pc2", "pc2", 0),
	}
}

// VTT-176
func TestQATestimonyEyesTheDMAndTheAgentAreSentEveryEventItself(t *testing.T) {
	tt := newQAEyesTable(map[string]gateway.Viewer{"dm": qaDM, "agent": qaAgent})
	tt.apply(t, qaWithP2OnTheNearEnd()...)
	tt.apply(t, qaActor("x", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaPlace("t-x", "x", 4), qaRevoke("x", "p1"))
	for _, name := range []string{"dm", "agent"} {
		s := tt.seats[name]
		if int64(len(s.sent)) != tt.seq {
			t.Errorf("VTT-176: %s was sent %d frames for %d events", name, len(s.sent), tt.seq)
		}
		for id, a := range tt.world.Actors {
			if !proto.Equal(a, s.fold.Actors[id]) {
				t.Errorf("VTT-176: %s holds actor %s as %v, the log as %v", name, id, s.fold.Actors[id], a)
			}
		}
		if len(s.fold.Tokens) != len(tt.world.Tokens) {
			t.Errorf("VTT-176: %s holds %d tokens, the log %d", name, len(s.fold.Tokens), len(tt.world.Tokens))
		}
	}
}

// VTT-194 VTT-180 VTT-195 VTT-208
func TestQATestimonyEyesAViewpointGivesAPlayerNoEyesAndAnEmptyOneGivesASpectatorNone(t *testing.T) {
	qaEyesStripGuard(t)
	p2On := gateway.Viewer{ParticipantID: "p2", Role: identity.RolePlayer, Viewpoint: "q"}
	tt := newQAEyesTable(map[string]gateway.Viewer{"p2": p2On, "unperched": qaSpectatorOn(""), "on-q": qaSpectatorOn("q")})
	tt.apply(t, qaWithP2OnTheNearEnd()...)
	tt.apply(t, qaActor("q", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER), qaPlace("t-q", "q", 4))
	qaEyesOnBoard(t, tt, "on-q", "t-q", true, "VTT-195")
	qaEyesOnBoard(t, tt, "p2", "t-q", false, "VTT-194")
	if tt.seats["p2"].fold.Scenes[qaStripID].Visible["4,0"] {
		t.Errorf("VTT-194: p2 sees through q, which it does not control")
	}
	if s := tt.seats["unperched"].fold; len(s.Scenes) != 0 || len(s.Tokens) != 0 {
		t.Errorf("VTT-180: an unperched spectator holds %d scenes and %d tokens", len(s.Scenes), len(s.Tokens))
	}
	qaEyesHolds(t, tt, "unperched", "q", "VTT-208")
	qaEyesHolds(t, tt, "unperched", "pc2", "VTT-208")
}
