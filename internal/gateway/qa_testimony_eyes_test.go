package gateway_test

import (
	"fmt"
	"slices"
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
	qaP1    = gateway.Viewer{ParticipantID: "p1", Role: identity.RolePlayer}
	qaP2    = gateway.Viewer{ParticipantID: "p2", Role: identity.RolePlayer}
	qaP3    = gateway.Viewer{ParticipantID: "p3", Role: identity.RolePlayer}
	qaDM    = gateway.Viewer{ParticipantID: "dm", Role: identity.RoleDM}
	qaAgent = gateway.Viewer{ParticipantID: "agent", Role: identity.RoleAgent}

	qaKindNames = map[vttv1.ActorKind]string{
		vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER: "party member",
		vttv1.ActorKind_ACTOR_KIND_NON_PARTY:    "non-party",
		vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED:  "kind unspecified",
	}
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

func qaEyesNames(f *vttv1.Envelope, actorID string) bool {
	named := []string{
		f.GetActorAdded().GetActor().GetActorId(),
		f.GetActorControlGranted().GetActorId(),
		f.GetActorControlRevoked().GetActorId(),
		f.GetTokenPlaced().GetActorId(),
		f.GetConditionApplied().GetActorId(),
		f.GetConditionRemoved().GetActorId(),
		f.GetResourceChanged().GetActorId(),
		f.GetActorRemoved().GetActorId(),
		f.GetAttackRolled().GetAttackerId(),
		f.GetAttackRolled().GetTargetId(),
		f.GetAbilityUsed().GetActorId(),
	}
	named = append(named, f.GetAbilityUsed().GetTargetIds()...)
	return slices.Contains(named, actorID)
}

func (s *qaEyesSeat) told(actorID string) bool {
	return slices.ContainsFunc(s.sent, func(f *vttv1.Envelope) bool { return qaEyesNames(f, actorID) })
}

func qaEyesNotIntroduced(tb testing.TB, tt *qaEyesTable, name, actorID, rule string) {
	tb.Helper()
	s := tt.seats[name]
	if s.told(actorID) || s.fold.Actors[actorID] != nil {
		tb.Errorf("%s: %s was introduced to %s", rule, name, actorID)
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

func qaEyesControllers(tb testing.TB, tt *qaEyesTable, rule, name, actorID string, want ...string) {
	tb.Helper()
	got := qaEyesHolds(tb, tt, name, actorID, rule).GetControllerIds()
	if !slices.Equal(got, want) {
		tb.Errorf("VTT-209: %s holds %s with controllers %v, want %v", name, actorID, got, want)
	}
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

func qaMove(tokenID string, from, to int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tokenID, SceneId: qaStripID, From: &vttv1.GridPosition{X: from}, To: &vttv1.GridPosition{X: to},
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

// VTT-241 VTT-209 VTT-208 VTT-242 VTT-194 VTT-196
func TestQATestimonyEyesAPlayerIsIntroducedToEveryTokenlessActorItControls(t *testing.T) {
	qaEyesStripGuard(t)
	for _, kind := range []vttv1.ActorKind{
		vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED,
	} {
		for _, first := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s, granted before the table is set: %v", qaKindNames[kind], first), func(t *testing.T) {
				tt := newQAEyesTable(map[string]gateway.Viewer{
					"p1": qaP1, "p2": qaP2, "spectator": qaSpectatorOn("pc2"), "dm": qaDM, "agent": qaAgent,
				})
				if first {
					tt.apply(t, qaActor("x", kind), qaGrant("x", "p1", kind))
				}
				tt.apply(t, qaWithP2OnTheNearEnd()...)
				if !first {
					tt.apply(t, qaActor("x", kind), qaGrant("x", "p1", kind))
				}
				qaEyesControllers(t, tt, "VTT-241", "p1", "x", "p1")
				if got := tt.seats["p1"].fold.Actors["x"].GetKind(); got != kind {
					t.Errorf("VTT-241: p1 holds x as %v, want %v", got, kind)
				}
				if n := len(tt.seats["p1"].fold.Scenes); n != 0 {
					t.Errorf("VTT-196 VTT-194: p1, whose only eye has no token, was introduced to %d scenes", n)
				}
				qaEyesHolds(t, tt, "dm", "x", "VTT-176")
				for _, other := range []string{"p2", "spectator"} {
					if kind == vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
						qaEyesControllers(t, tt, "VTT-208", other, "x", "p1")
					} else {
						qaEyesNotIntroduced(t, tt, other, "x", "VTT-242")
					}
				}
			})
		}
	}
}

// VTT-241 VTT-209 VTT-242
func TestQATestimonyEyesAnActorGrantedToTwoPlayersIsIntroducedToBothWithBothControllers(t *testing.T) {
	for _, kind := range []vttv1.ActorKind{vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED} {
		t.Run(qaKindNames[kind], func(t *testing.T) {
			tt := newQAEyesTable(map[string]gateway.Viewer{"p1": qaP1, "p2": qaP2, "p3": qaP3})
			tt.apply(t, qaActor("x", kind), qaGrant("x", "p1", kind))
			qaEyesNotIntroduced(t, tt, "p3", "x", "VTT-242")
			tt.apply(t, qaGrant("x", "p3", kind))
			qaEyesControllers(t, tt, "VTT-241", "p1", "x", "p1", "p3")
			qaEyesControllers(t, tt, "VTT-241", "p3", "x", "p1", "p3")
			qaEyesNotIntroduced(t, tt, "p2", "x", "VTT-242")
		})
	}
}

// VTT-242 VTT-203 VTT-204 VTT-206 VTT-194 VTT-195
func TestQATestimonyEyesAnotherPlayersActorIsIntroducedOnlyOnceItsTokenComesIntoSight(t *testing.T) {
	qaEyesStripGuard(t)
	for _, kind := range []vttv1.ActorKind{vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED} {
		t.Run(qaKindNames[kind], func(t *testing.T) {
			watchers := []string{"p2", "spectator"}
			tt := newQAEyesTable(map[string]gateway.Viewer{"p1": qaP1, "p2": qaP2, "spectator": qaSpectatorOn("pc2")})
			tt.apply(t, qaWithP2OnTheNearEnd()...)
			tt.apply(t, qaActor("x", kind), qaGrant("x", "p1", kind), qaPlace("t-x", "x", 4))
			qaEyesOnBoard(t, tt, "p1", "t-x", true, "VTT-194 VTT-203")
			for _, w := range watchers {
				qaEyesNotIntroduced(t, tt, w, "x", "VTT-242")
			}
			tt.apply(t, qaMove("t-x", 4, 3))
			for _, w := range watchers {
				qaEyesNotIntroduced(t, tt, w, "x", "VTT-242")
			}
			tt.apply(t, qaMove("t-x", 3, 1))
			for _, w := range watchers {
				qaEyesControllers(t, tt, "VTT-242", w, "x", "p1")
				qaEyesOnBoard(t, tt, w, "t-x", true, "VTT-242 VTT-203")
			}
			tt.apply(t, qaMove("t-x", 1, 4))
			for _, w := range watchers {
				qaEyesOnBoard(t, tt, w, "t-x", false, "VTT-204")
				qaEyesHolds(t, tt, w, "x", `SPEC-016 "What the projector remembers": "an actor seen and then hidden stays in actors"`)
			}
			tt.apply(t, qaMove("t-x", 4, 1))
			for _, w := range watchers {
				qaEyesOnBoard(t, tt, w, "t-x", true, "VTT-203")
			}
		})
	}
}

// VTT-242 VTT-203
func TestQATestimonyEyesAnotherPlayersActorPlacedInSightIsIntroducedOnThatEvent(t *testing.T) {
	qaEyesStripGuard(t)
	tt := newQAEyesTable(map[string]gateway.Viewer{"p1": qaP1, "p2": qaP2, "spectator": qaSpectatorOn("pc2")})
	tt.apply(t, qaWithP2OnTheNearEnd()...)
	tt.apply(t, qaActor("x", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	qaEyesNotIntroduced(t, tt, "p2", "x", "VTT-242")
	tt.apply(t, qaPlace("t-x", "x", 1))
	for _, w := range []string{"p2", "spectator"} {
		qaEyesControllers(t, tt, "VTT-242", w, "x", "p1")
		qaEyesOnBoard(t, tt, w, "t-x", true, "VTT-203")
	}
}

// VTT-194 VTT-196 VTT-203 VTT-241 VTT-208
func TestQATestimonyEyesAPlayerSeesThroughItsActorsTokenWhereverItStands(t *testing.T) {
	qaEyesStripGuard(t)
	tt := newQAEyesTable(map[string]gateway.Viewer{"p1": qaP1, "p3": qaP3})
	tt.apply(t, qaWithP2OnTheNearEnd()...)
	tt.apply(t, qaActor("x", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaPlace("t-x", "x", 4))
	if n := len(tt.seats["p1"].fold.Scenes); n != 0 {
		t.Fatalf("VTT-196: p1 controls nothing yet and was introduced to %d scenes", n)
	}
	qaEyesNotIntroduced(t, tt, "p1", "x", "VTT-242")
	tt.apply(t, qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	qaEyesControllers(t, tt, "VTT-241", "p1", "x", "p1")
	qaEyesOnBoard(t, tt, "p1", "t-x", true, "VTT-194 VTT-203")
	qaEyesOnBoard(t, tt, "p1", "t-pc2", false, "VTT-203")
	qaEyesHolds(t, tt, "p1", "pc2", "VTT-208")
	sc, ok := tt.seats["p1"].fold.Scenes[qaStripID]
	if !ok {
		t.Fatalf("VTT-196: p1 was not introduced to the scene its eye stands in")
	}
	if !sc.Visible["4,0"] || sc.Visible["0,0"] {
		t.Errorf("VTT-194: p1 sees %v, want the far end through x and not the near end", sc.Visible)
	}
	for _, name := range []string{"x", "pc2"} {
		if name == "pc2" {
			qaEyesHolds(t, tt, "p3", name, "VTT-208")
		} else {
			qaEyesNotIntroduced(t, tt, "p3", name, "VTT-242")
		}
	}
	if n := len(tt.seats["p3"].fold.Scenes); n != 0 {
		t.Errorf("VTT-194 VTT-196: p3 controls nothing and was introduced to %d scenes", n)
	}
}

// VTT-194 VTT-204 VTT-202 VTT-203
func TestQATestimonyEyesARevokedPlayerStopsSeeingThroughTheActorAndKeepsIt(t *testing.T) {
	qaEyesStripGuard(t)
	held := `SPEC-016 "What the projector remembers": "transitions removes one only when st no longer holds it"`
	t.Run("with a token", func(t *testing.T) {
		tt := newQAEyesTable(map[string]gateway.Viewer{"p1": qaP1, "p2": qaP2})
		tt.apply(t, qaWithP2OnTheNearEnd()...)
		tt.apply(t, qaActor("x", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaPlace("t-x", "x", 4))
		qaEyesOnBoard(t, tt, "p1", "t-x", true, "VTT-194")
		tt.apply(t, qaRevoke("x", "p1"))
		qaEyesOnBoard(t, tt, "p1", "t-x", false, "VTT-194 VTT-204")
		if a := qaEyesHolds(t, tt, "p1", "x", held); len(a.GetControllerIds()) != 0 {
			t.Errorf(`SPEC-016 "How each payload is ruled" (ActorControlRevoked forwarded when known): p1 holds x with controllers %v`, a.GetControllerIds())
		}
		sc, ok := tt.seats["p1"].fold.Scenes[qaStripID]
		if !ok || sc.Visible == nil || len(sc.Visible) != 0 {
			t.Errorf("VTT-202: p1's scene after the revocation is %v (held %v), want it dark", sc.Visible, ok)
		}
		qaEyesNotIntroduced(t, tt, "p2", "x", "VTT-242")
		tt.apply(t, qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
		qaEyesOnBoard(t, tt, "p1", "t-x", true, "VTT-194 VTT-203")
		qaEyesControllers(t, tt, "VTT-241", "p1", "x", "p1")
	})
	t.Run("token-less", func(t *testing.T) {
		tt := newQAEyesTable(map[string]gateway.Viewer{"p1": qaP1})
		tt.apply(t, qaActor("x", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaRevoke("x", "p1"))
		if a := qaEyesHolds(t, tt, "p1", "x", held); len(a.GetControllerIds()) != 0 {
			t.Errorf("p1 holds x with controllers %v after its revocation", a.GetControllerIds())
		}
	})
}

// VTT-195 VTT-208 VTT-242 VTT-196
func TestQATestimonyEyesASpectatorOnAPartyMemberIsIntroducedToThePartyAndToWhatItSees(t *testing.T) {
	qaEyesStripGuard(t)
	tt := newQAEyesTable(map[string]gateway.Viewer{"spectator": qaSpectatorOn("pc2"), "p1": qaP1})
	tt.apply(t, qaWithP2OnTheNearEnd()...)
	tt.apply(t,
		qaActor("y", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		qaActor("x", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qaActor("z", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaPlace("t-z", "z", 1),
		qaActor("w", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaPlace("t-w", "w", 4),
	)
	qaEyesHolds(t, tt, "spectator", "y", "VTT-208")
	qaEyesHolds(t, tt, "spectator", "z", "VTT-195 VTT-242")
	qaEyesOnBoard(t, tt, "spectator", "t-z", true, "VTT-195")
	qaEyesNotIntroduced(t, tt, "spectator", "x", "VTT-242")
	qaEyesNotIntroduced(t, tt, "spectator", "w", "VTT-242")
	if _, ok := tt.seats["spectator"].fold.Scenes[qaStripID]; !ok {
		t.Errorf("VTT-195 VTT-196: the spectator was not introduced to the scene its shoulder stands in")
	}
	qaEyesHolds(t, tt, "p1", "x", "VTT-241")
	qaEyesNotIntroduced(t, tt, "p1", "z", "VTT-242")
}

// VTT-195 VTT-242 VTT-196 VTT-208
func TestQATestimonyEyesASpectatorWhoseViewpointNamesANonPartyActorSeesThroughNothing(t *testing.T) {
	for _, kind := range []vttv1.ActorKind{vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED} {
		t.Run(qaKindNames[kind], func(t *testing.T) {
			tt := newQAEyesTable(map[string]gateway.Viewer{"on-x": qaSpectatorOn("x"), "on-pc": qaSpectatorOn("pc")})
			tt.apply(t,
				qaStrip(),
				qaActor("x", kind), qaGrant("x", "p1", kind), qaPlace("t-x", "x", 0),
				qaActor("pc", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER), qaPlace("t-pc", "pc", 1),
			)
			qaEyesNotIntroduced(t, tt, "on-x", "x", "VTT-242")
			if s := tt.seats["on-x"].fold; len(s.Scenes) != 0 || len(s.Tokens) != 0 {
				t.Errorf("VTT-195 VTT-196: a spectator on a non-party actor holds %d scenes and %d tokens", len(s.Scenes), len(s.Tokens))
			}
			qaEyesHolds(t, tt, "on-x", "pc", "VTT-208")
			qaEyesHolds(t, tt, "on-pc", "x", "VTT-195 VTT-242")
			qaEyesOnBoard(t, tt, "on-pc", "t-x", true, "VTT-195 VTT-203")
		})
	}
}

// VTT-208 VTT-242
func TestQATestimonyEyesAnActorAGrantMakesAPartyMemberIsIntroducedToEveryone(t *testing.T) {
	tt := newQAEyesTable(map[string]gateway.Viewer{"p1": qaP1, "p2": qaP2, "spectator": qaSpectatorOn("x")})
	tt.apply(t, qaActor("x", vttv1.ActorKind_ACTOR_KIND_NON_PARTY), qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	qaEyesNotIntroduced(t, tt, "p2", "x", "VTT-242")
	qaEyesNotIntroduced(t, tt, "spectator", "x", "VTT-242")
	tt.apply(t, qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	for _, w := range []string{"p2", "spectator"} {
		if got := qaEyesHolds(t, tt, w, "x", "VTT-208").GetKind(); got != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
			t.Errorf("VTT-208: %s holds x as %v", w, got)
		}
	}
	tt.apply(t, qaGrant("x", "p1", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
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
