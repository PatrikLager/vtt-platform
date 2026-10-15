package gateway_test

// Independent QA for the projection whose memory is its viewer's fold.
// Every expectation names the requirement it pins: a VTT id, or SPEC-016 by
// section and quoted sentence, or the task's requirement file by its heading.

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	qaParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

// ---------------------------------------------------------------- the log

type qafmLog struct{ envs []*vttv1.Envelope }

// emit stamps an issuer, a session, a time and an event id on every event, so
// a frame that keeps any of them is observable.
func (l *qafmLog) emit(e *vttv1.Envelope) int {
	e.Sequence = int64(len(l.envs) + 1)
	e.EventId = fmt.Sprintf("ev-%d", e.Sequence)
	e.ParticipantId = "dm-1"
	e.ActorRole = "dm"
	e.SessionId = "sess-1"
	e.OccurredAt = timestamppb.Now()
	l.envs = append(l.envs, e)
	return len(l.envs) - 1
}

func qafmSceneCreated(id string, w, h int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: id, Name: "hall", GridWidth: w, GridHeight: h}}}
}
func qafmAdded(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: proto.Clone(a).(*vttv1.Actor)}}}
}
func qafmGrant(actor, pid string, k vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actor, ParticipantId: pid, Kind: k}}}
}
func qafmRevoke(actor, pid string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: actor, ParticipantId: pid}}}
}
func qafmCondOn(actor, c string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{ActorId: actor, ConditionId: c, Source: "a-hand"}}}
}
func qafmCondOff(actor, c string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{ActorId: actor, ConditionId: c, Reason: "a-hand"}}}
}
func qafmPlace(tok, actor string, x int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: tok, SceneId: "s1", ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: 0}}}}
}
func qafmTokenOff(tok string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tok}}}
}
func qafmRemoved(actor string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}}
}
func qafmNarrate(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

// qafmBase: one scene with no terrain (every square visible, SPEC-016 "What a
// look sees": range and tolerance unlimited) and the party member p1, the
// player u1's actor and the spectator's shoulder, standing in it.
func qafmBase() *qafmLog {
	l := &qafmLog{}
	l.emit(qafmSceneCreated("s1", 8, 1))
	l.emit(qafmAdded(qafmWard()))
	l.emit(qafmGrant("p1", "u1", qaParty))
	l.emit(qafmPlace("tp", "p1", 0))
	return l
}

func qafmWard() *vttv1.Actor {
	return &vttv1.Actor{ActorId: "p1", Name: "ward", ModuleId: "mod-a", Kind: qaParty,
		Resources: map[string]*vttv1.Resource{"vigor": {Current: 10, Max: 10}}}
}

func qafmPlayer() gateway.Viewer {
	return gateway.Viewer{ParticipantID: "u1", Role: identity.RolePlayer}
}
func qafmWatcher() gateway.Viewer {
	return gateway.Viewer{ParticipantID: "w1", Role: identity.RoleSpectator, Viewpoint: "p1"}
}
func qafmViewers() []gateway.Viewer { return []gateway.Viewer{qafmPlayer(), qafmWatcher()} }

func qafmLurker() *vttv1.Actor {
	md, err := structpb.NewStruct(map[string]any{"tag": "alpha"})
	if err != nil {
		panic(err)
	}
	return &vttv1.Actor{ActorId: "x1", Name: "lurker", ModuleId: "mod-a", Kind: qaNonParty,
		Attributes: map[string]int32{"grit": 3},
		Resources:  map[string]*vttv1.Resource{"vigor": {Current: 4, Max: 9}},
		ModuleData: md}
}

// ---------------------------------------------------------------- the run

type qafmRun struct {
	envs   []*vttv1.Envelope
	per    [][]*vttv1.Envelope
	worlds []*engine.State
}

// qafmProject drives a projector as the requirement's "How the projector is
// driven" says, and checks on every event VTT-221 (the event's sequence),
// VTT-224 (every frame folds onto those before it), VTT-272/VTT-273 (no
// issuer, no reason or source), VTT-249 (an own frame carries nothing of an
// envelope but the sequence) and VTT-252 (a bare ActorRemoved only directly
// before an introduction of the same id).
func qafmProject(t *testing.T, v gateway.Viewer, envs []*vttv1.Envelope) *qafmRun {
	t.Helper()
	pr := gateway.NewProjector(v)
	r := &qafmRun{envs: envs}
	var all []*vttv1.Envelope
	for i, env := range envs {
		st, err := campaign.FoldPrefix(envs[:i+1])
		if err != nil {
			t.Fatalf("fixture: the world log does not fold at %d: %v", i+1, err)
		}
		out := pr.Project(env, st)
		for k, f := range out {
			if f.GetSequence() != env.GetSequence() {
				t.Errorf("VTT-221: event %d frame %q carries sequence %d", env.GetSequence(), qafmDescribe(f), f.GetSequence())
			}
			if f.GetParticipantId() != "" || f.GetActorRole() != "" {
				t.Errorf("VTT-272: event %d frame %q carries participant %q role %q", env.GetSequence(), qafmDescribe(f), f.GetParticipantId(), f.GetActorRole())
			}
			qafmNoCause(t, env.GetSequence(), f)
			if f.GetEventId() == "" && (f.GetSessionId() != "" || f.GetOccurredAt() != nil) {
				t.Errorf("VTT-249: event %d own frame %q carries session %q time %v", env.GetSequence(), qafmDescribe(f), f.GetSessionId(), f.GetOccurredAt())
			}
			if rm := f.GetActorRemoved(); rm != nil && f.GetEventId() == "" {
				next := (*vttv1.ActorAdded)(nil)
				if k+1 < len(out) {
					next = out[k+1].GetActorAdded()
				}
				if next == nil || next.GetActor().GetActorId() != rm.GetActorId() {
					t.Errorf("VTT-252: event %d sends a bare ActorRemoved %s not directly followed by its introduction: %v", env.GetSequence(), rm.GetActorId(), qafmDescribeAll(out))
				}
			}
		}
		all = append(all, out...)
		if _, err := campaign.FoldPrefix(all); err != nil {
			t.Fatalf("VTT-224: %s's frames stop folding at event %d (%v): %v", v.Role, env.GetSequence(), qafmDescribeAll(out), err)
		}
		r.per = append(r.per, out)
		r.worlds = append(r.worlds, st)
	}
	return r
}

func qafmNoCause(t *testing.T, seq int64, f *vttv1.Envelope) {
	t.Helper()
	if c := f.GetConditionApplied(); c != nil && c.GetSource() != "" {
		t.Errorf("VTT-273: event %d frame %q carries source %q", seq, qafmDescribe(f), c.GetSource())
	}
	if c := f.GetConditionRemoved(); c != nil && c.GetReason() != "" {
		t.Errorf("VTT-273: event %d frame %q carries reason %q", seq, qafmDescribe(f), c.GetReason())
	}
	if c := f.GetResourceChanged(); c != nil && c.GetReason() != "" {
		t.Errorf("VTT-273: event %d frame %q carries reason %q", seq, qafmDescribe(f), c.GetReason())
	}
}

// viewerFold is campaign.FoldPrefix of every frame sent for events [0..upTo].
func (r *qafmRun) viewerFold(t *testing.T, upTo int) *engine.State {
	t.Helper()
	var all []*vttv1.Envelope
	for i := 0; i <= upTo; i++ {
		all = append(all, r.per[i]...)
	}
	st, err := campaign.FoldPrefix(all)
	if err != nil {
		t.Fatalf("VTT-224: viewer fold through event %d: %v", upTo+1, err)
	}
	return st
}

// ---------------------------------------------------------------- describing frames

func qafmKind(k vttv1.ActorKind) string {
	switch k {
	case qaParty:
		return "PARTY"
	case qaNonParty:
		return "NON_PARTY"
	}
	return "UNSPEC"
}

func qafmDescribe(f *vttv1.Envelope) string {
	own := ""
	if f.GetEventId() == "" {
		own = "own:"
	}
	switch p := f.GetPayload().(type) {
	case *vttv1.Envelope_ActorRemoved:
		return own + "ActorRemoved " + p.ActorRemoved.GetActorId()
	case *vttv1.Envelope_ActorAdded:
		return own + "ActorAdded " + p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		g := p.ActorControlGranted
		return fmt.Sprintf("%sGranted %s %s %s", own, g.GetActorId(), g.GetParticipantId(), qafmKind(g.GetKind()))
	case *vttv1.Envelope_ActorControlRevoked:
		return fmt.Sprintf("%sRevoked %s %s", own, p.ActorControlRevoked.GetActorId(), p.ActorControlRevoked.GetParticipantId())
	case *vttv1.Envelope_ConditionApplied:
		return fmt.Sprintf("%sCondOn %s %s", own, p.ConditionApplied.GetActorId(), p.ConditionApplied.GetConditionId())
	case *vttv1.Envelope_ConditionRemoved:
		return fmt.Sprintf("%sCondOff %s %s", own, p.ConditionRemoved.GetActorId(), p.ConditionRemoved.GetConditionId())
	case *vttv1.Envelope_ResourceChanged:
		c := p.ResourceChanged
		return fmt.Sprintf("%sRes %s %s %+d=%d", own, c.GetActorId(), c.GetResource(), c.GetDelta(), c.GetNewValue())
	case *vttv1.Envelope_TokenPlaced:
		c := p.TokenPlaced
		return fmt.Sprintf("%sPlaced %s %s %s %d,%d", own, c.GetTokenId(), c.GetSceneId(), c.GetActorId(), c.GetPosition().GetX(), c.GetPosition().GetY())
	case *vttv1.Envelope_TokenHidden:
		return own + "Hidden " + p.TokenHidden.GetTokenId()
	case *vttv1.Envelope_TokenRemoved:
		return own + "TokenRemoved " + p.TokenRemoved.GetTokenId()
	case *vttv1.Envelope_SceneCreated:
		return own + "SceneCreated " + p.SceneCreated.GetSceneId()
	case *vttv1.Envelope_SceneSeen:
		return fmt.Sprintf("%sSceneSeen %s %d", own, p.SceneSeen.GetSceneId(), len(p.SceneSeen.GetVisible()))
	case *vttv1.Envelope_NarrationAdded:
		return own + "Narration " + p.NarrationAdded.GetText()
	}
	return fmt.Sprintf("%s%T", own, f.GetPayload())
}

func qafmDescribeAll(fs []*vttv1.Envelope) []string {
	out := []string{}
	for _, f := range fs {
		out = append(out, qafmDescribe(f))
	}
	return out
}

func qafmExpectFrames(t *testing.T, why string, r *qafmRun, idx int, want []string) {
	t.Helper()
	got := qafmDescribeAll(r.per[idx])
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s\n  event %d (%s)\n  got  %q\n  want %q", why, idx+1, qafmDescribe(r.envs[idx]), got, want)
	}
}

// ---------------------------------------------------------------- comparing an actor

func qafmCondIDs(st *engine.State, id string) []string {
	out := []string{}
	for _, c := range st.Conditions[id] {
		out = append(out, c.ID)
	}
	return out
}

func qafmCtl(st *engine.State, id string) []string {
	return append([]string{}, st.Actors[id].GetControllerIds()...)
}

func qafmSorted(s []string) []string {
	c := append([]string{}, s...)
	sort.Strings(c)
	return c
}

// qafmSameActorAsSets: the viewer's fold holds the actor as the world does,
// conditions and controllers compared as sets (VTT-248's grain; SPEC-016
// Consequences: "The conditions and controllers a correction brings need not
// keep the server's order").
func qafmSameActorAsSets(t *testing.T, why string, viewer, world *engine.State, id string) {
	t.Helper()
	va, wa := viewer.Actors[id], world.Actors[id]
	if va == nil || wa == nil {
		t.Errorf("%s: actor %s viewer=%v world=%v", why, id, va != nil, wa != nil)
		return
	}
	var diffs []string
	if va.GetName() != wa.GetName() {
		diffs = append(diffs, fmt.Sprintf("name %q vs %q", va.GetName(), wa.GetName()))
	}
	if va.GetModuleId() != wa.GetModuleId() {
		diffs = append(diffs, fmt.Sprintf("module %q vs %q", va.GetModuleId(), wa.GetModuleId()))
	}
	if !reflect.DeepEqual(va.GetAttributes(), wa.GetAttributes()) {
		diffs = append(diffs, fmt.Sprintf("attributes %v vs %v", va.GetAttributes(), wa.GetAttributes()))
	}
	if !proto.Equal(va.GetModuleData(), wa.GetModuleData()) {
		diffs = append(diffs, fmt.Sprintf("module data %v vs %v", va.GetModuleData(), wa.GetModuleData()))
	}
	if len(va.GetResources()) != len(wa.GetResources()) {
		diffs = append(diffs, fmt.Sprintf("resources %v vs %v", va.GetResources(), wa.GetResources()))
	} else {
		for k, r := range wa.GetResources() {
			if !proto.Equal(va.GetResources()[k], r) {
				diffs = append(diffs, fmt.Sprintf("resource %s %v vs %v", k, va.GetResources()[k], r))
			}
		}
	}
	if va.GetKind() != wa.GetKind() {
		diffs = append(diffs, fmt.Sprintf("kind %v vs %v", va.GetKind(), wa.GetKind()))
	}
	if !reflect.DeepEqual(qafmSorted(va.GetControllerIds()), qafmSorted(wa.GetControllerIds())) {
		diffs = append(diffs, fmt.Sprintf("controllers %v vs %v", va.GetControllerIds(), wa.GetControllerIds()))
	}
	if !reflect.DeepEqual(qafmSorted(qafmCondIDs(viewer, id)), qafmSorted(qafmCondIDs(world, id))) {
		diffs = append(diffs, fmt.Sprintf("conditions %v vs %v", qafmCondIDs(viewer, id), qafmCondIDs(world, id)))
	}
	if len(diffs) > 0 {
		t.Errorf("%s: viewer's fold of %s differs from the world's: %s", why, id, strings.Join(diffs, "; "))
	}
}

// ================================================================ VTT-313: corrected

// qafmReAddHistory: x1 seen with condition "dazed" and controller u2, its token
// taken away (unseen), removed while unseen, added again as `again`, granted
// to u3, given condition "steady", then placed in sight. Returns the log and
// the indexes of the unseen stretch and of the event that brings it into sight.
func qafmReAddHistory(again *vttv1.Actor) (l *qafmLog, unseenFrom, sight int) {
	l = qafmBase()
	l.emit(qafmAdded(qafmLurker()))
	l.emit(qafmGrant("x1", "u2", qaNonParty))
	l.emit(qafmCondOn("x1", "dazed"))
	l.emit(qafmPlace("tx1", "x1", 3))
	l.emit(qafmTokenOff("tx1"))
	unseenFrom = l.emit(qafmRemoved("x1"))
	k := again.GetKind()
	l.emit(qafmAdded(again))
	l.emit(qafmGrant("x1", "u3", k))
	l.emit(qafmCondOn("x1", "steady"))
	sight = l.emit(qafmPlace("tx2", "x1", 4))
	return l, unseenFrom, sight
}

// VTT-313 VTT-248 VTT-252
func TestQAFoldMemoryAnActorAddedAgainUnchangedWhileUnseenIsCorrectedNotIntroduced(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			l, unseenFrom, sight := qafmReAddHistory(qafmLurker())
			r := qafmProject(t, v, l.envs)
			// Fixture: the viewer held x1 before the unseen removal.
			if r.viewerFold(t, unseenFrom-1).Actors["x1"] == nil {
				t.Fatal("fixture: viewer must hold x1 before its unseen removal")
			}
			for i := unseenFrom; i < sight; i++ {
				// VTT-251 (removal unseen), VTT-252 (no bare removal), VTT-242 (no
				// token in sight), VTT-245, VTT-243: nothing reaches the viewer.
				qafmExpectFrames(t, "VTT-251/VTT-252/VTT-242/VTT-245/VTT-243: nothing while x1 is unseen", r, i, []string{})
			}
			// VTT-313 "corrected otherwise"; SPEC-016 "The correction on sight".
			qafmExpectFrames(t, "VTT-313: an unchanged re-add is corrected, not introduced", r, sight, []string{
				"own:CondOff x1 dazed",
				"own:CondOn x1 steady",
				"own:Granted x1 u3 NON_PARTY",
				"own:Revoked x1 u2",
				"own:Placed tx2 s1 x1 4,0",
			})
			// VTT-248 at the grain the accepted limit allows (sets).
			qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, sight), r.worlds[sight], "x1")
		})
	}
}

// VTT-313 VTT-248
func TestQAFoldMemoryAnActorAddedAgainDifferingOnlyInStatusIsCorrected(t *testing.T) {
	cases := []struct {
		name  string
		again func(a *vttv1.Actor)
		want  []string
	}{
		{"current value", func(a *vttv1.Actor) { a.Resources["vigor"].Current = 7 }, []string{
			"own:Res x1 vigor +3=7",
			"own:CondOff x1 dazed",
			"own:CondOn x1 steady",
			"own:Granted x1 u3 NON_PARTY",
			"own:Revoked x1 u2",
			"own:Placed tx2 s1 x1 4,0",
		}},
		{"kind", func(a *vttv1.Actor) { a.Kind = qaParty }, []string{
			"own:CondOff x1 dazed",
			"own:CondOn x1 steady",
			"own:Granted x1 u3 PARTY",
			"own:Revoked x1 u2",
			"own:Placed tx2 s1 x1 4,0",
		}},
	}
	for _, c := range cases {
		for _, v := range qafmViewers() {
			t.Run(c.name+"/"+string(v.Role), func(t *testing.T) {
				again := qafmLurker()
				c.again(again)
				l, unseenFrom, sight := qafmReAddHistory(again)
				r := qafmProject(t, v, l.envs)
				for i := unseenFrom; i < sight; i++ {
					qafmExpectFrames(t, "VTT-252/VTT-242/SPEC-016 Consequences \"a viewer that holds an actor keeps the kind it last saw until it sees it again\"", r, i, []string{})
				}
				qafmExpectFrames(t, "VTT-313: a re-add differing in no ActorAdded-only field is corrected", r, sight, c.want)
				qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, sight), r.worlds[sight], "x1")
			})
		}
	}
}

// ================================================================ VTT-313: introduced afresh

// VTT-313 VTT-252
func TestQAFoldMemoryAnActorAddedAgainDifferingInAnAddOnlyFieldIsIntroducedAfresh(t *testing.T) {
	cases := []struct {
		name  string
		again func(a *vttv1.Actor)
	}{
		{"name", func(a *vttv1.Actor) { a.Name = "skulker" }},
		{"module id", func(a *vttv1.Actor) { a.ModuleId = "mod-b" }},
		{"attribute value", func(a *vttv1.Actor) { a.Attributes["grit"] = 4 }},
		{"attribute added", func(a *vttv1.Actor) { a.Attributes["poise"] = 1 }},
		{"module data", func(a *vttv1.Actor) {
			md, _ := structpb.NewStruct(map[string]any{"tag": "beta"})
			a.ModuleData = md
		}},
		{"resource name", func(a *vttv1.Actor) {
			a.Resources = map[string]*vttv1.Resource{"verve": {Current: 4, Max: 9}}
		}},
		{"resource maximum", func(a *vttv1.Actor) { a.Resources["vigor"].Max = 12 }},
	}
	for _, c := range cases {
		for _, v := range qafmViewers() {
			t.Run(c.name+"/"+string(v.Role), func(t *testing.T) {
				again := qafmLurker()
				c.again(again)
				l, unseenFrom, sight := qafmReAddHistory(again)
				r := qafmProject(t, v, l.envs)
				for i := unseenFrom; i < sight; i++ {
					qafmExpectFrames(t, "VTT-252: no bare ActorRemoved but directly before an introduction; VTT-242", r, i, []string{})
				}
				// VTT-313 "introduced afresh"; requirement: "a bare ActorRemoved of
				// the id, then the ActorAdded and the frames an introduction carries"
				// (SPEC-016 "What transitions sends": grants in the actor's order
				// carrying its kind, then one ConditionApplied per condition).
				qafmExpectFrames(t, "VTT-313: a re-add differing in an ActorAdded-only field is introduced afresh", r, sight, []string{
					"own:ActorRemoved x1",
					"own:ActorAdded x1",
					"own:Granted x1 u3 NON_PARTY",
					"own:CondOn x1 steady",
					"own:Placed tx2 s1 x1 4,0",
				})
				frames := r.per[sight]
				if len(frames) >= 2 {
					// SPEC-016 "What transitions sends": the bare ActorRemoved carries
					// "the actor's id and the event's sequence and nothing else".
					want := &vttv1.Envelope{Sequence: l.envs[sight].GetSequence(), Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: "x1"}}}
					if !proto.Equal(frames[0], want) {
						t.Errorf("SPEC-016 bare ActorRemoved: got %v", frames[0])
					}
					// SPEC-016: "an ActorAdded of a copy of it with ControllerId and
					// ControllerIds cleared".
					wantActor := proto.Clone(r.worlds[sight].Actors["x1"]).(*vttv1.Actor)
					wantActor.ControllerId, wantActor.ControllerIds = "", nil
					if got := frames[1].GetActorAdded().GetActor(); !proto.Equal(got, wantActor) {
						t.Errorf("SPEC-016 introduction: ActorAdded carries %v, want %v", got, wantActor)
					}
				}
				vf := r.viewerFold(t, sight)
				qafmSameActorAsSets(t, "VTT-313/VTT-248", vf, r.worlds[sight], "x1")
				// Introduced afresh: the fold holds exactly the world's conditions in
				// the world's order (no condition crosses an introduction).
				if got, want := qafmCondIDs(vf, "x1"), qafmCondIDs(r.worlds[sight], "x1"); !reflect.DeepEqual(got, want) {
					t.Errorf("SPEC-016 introduction \"one ConditionApplied per condition st holds\": fold %v world %v", got, want)
				}
			})
		}
	}
}

// The viewer never held the id: no bare removal (VTT-252), a plain introduction.
// VTT-313 VTT-252
func TestQAFoldMemoryAnActorTheViewerNeverHeldIsIntroducedWithoutABareRemoval(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			l := qafmBase()
			l.emit(qafmAdded(qafmLurker()))
			l.emit(qafmRemoved("x1"))
			again := qafmLurker()
			again.Name = "skulker"
			l.emit(qafmAdded(again))
			sight := l.emit(qafmPlace("tx2", "x1", 4))
			r := qafmProject(t, v, l.envs)
			qafmExpectFrames(t, "VTT-313 \"no longer holds the id\" / VTT-252 / VTT-242", r, sight, []string{
				"own:ActorAdded x1",
				"own:Placed tx2 s1 x1 4,0",
			})
			qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, sight), r.worlds[sight], "x1")
		})
	}
}

// ================================================================ neighbour: removed while SEEN

// An eye's removal is forwarded (VTT-251, SPEC-016 Consequences "A forwarded
// ActorRemoved reaches only a viewer whose eye the actor is"), so the viewer no
// longer holds the id and an identical re-add is a plain introduction (VTT-313
// "no longer holds the id").
func TestQAFoldMemoryAPlayersOwnActorRemovedAndAddedAgainIsIntroducedPlainly(t *testing.T) {
	l := qafmBase()
	l.emit(qafmAdded(qafmLurker()))
	l.emit(qafmGrant("x1", "u1", qaNonParty))
	l.emit(qafmCondOn("x1", "dazed"))
	removal := l.emit(qafmRemoved("x1"))
	readd := l.emit(qafmAdded(qafmLurker()))
	regrant := l.emit(qafmGrant("x1", "u1", qaNonParty))
	r := qafmProject(t, qafmPlayer(), l.envs)
	qafmExpectFrames(t, "VTT-251/VTT-215: the eye's removal is forwarded", r, removal, []string{"ActorRemoved x1"})
	if r.viewerFold(t, removal).Actors["x1"] != nil {
		t.Errorf("fixture: the player still holds x1 after its forwarded removal")
	}
	qafmExpectFrames(t, "VTT-242: not an eye, no token: nothing", r, readd, []string{})
	qafmExpectFrames(t, "VTT-313 \"no longer holds the id\"/VTT-241/VTT-209/VTT-252", r, regrant, []string{
		"own:ActorAdded x1",
		"own:Granted x1 u1 NON_PARTY",
	})
	qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, regrant), r.worlds[regrant], "x1")
}

func TestQAFoldMemoryAShoulderRemovedAndAddedAgainIsIntroducedPlainly(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			l := qafmBase()
			l.emit(qafmCondOn("p1", "dazed"))
			l.emit(qafmTokenOff("tp"))
			removal := l.emit(qafmRemoved("p1"))
			readd := l.emit(qafmAdded(qafmWard()))
			r := qafmProject(t, v, l.envs)
			qafmExpectFrames(t, "VTT-251/VTT-246: the eye's removal is forwarded", r, removal, []string{"ActorRemoved p1"})
			qafmExpectFrames(t, "VTT-208/VTT-313 \"no longer holds the id\"/VTT-252: introduced without a bare removal", r, readd, []string{
				"own:ActorAdded p1",
			})
			qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, readd), r.worlds[readd], "p1")
		})
	}
}

// ================================================================ neighbour: a party member

func TestQAFoldMemoryAPartyMemberRemovedUnseenAndAddedAgain(t *testing.T) {
	kin := func(name string) *vttv1.Actor {
		return &vttv1.Actor{ActorId: "y1", Name: name, ModuleId: "mod-a", Kind: qaParty,
			Resources: map[string]*vttv1.Resource{"vigor": {Current: 6, Max: 6}}}
	}
	for _, differs := range []bool{false, true} {
		for _, v := range qafmViewers() {
			t.Run(fmt.Sprintf("differs=%v/%s", differs, v.Role), func(t *testing.T) {
				l := qafmBase()
				intro := l.emit(qafmAdded(kin("kin")))
				removal := l.emit(qafmRemoved("y1"))
				name := "kin"
				if differs {
					name = "kith"
				}
				readd := l.emit(qafmAdded(kin(name)))
				sight := l.emit(qafmPlace("ty", "y1", 5))
				r := qafmProject(t, v, l.envs)
				qafmExpectFrames(t, "VTT-208: a party member is introduced, seen or not", r, intro, []string{"own:ActorAdded y1"})
				qafmExpectFrames(t, "VTT-251: a removal the viewer did not see is not forwarded", r, removal, []string{})
				if differs {
					// VTT-208 ("seen or not") with VTT-313 ("differs in a field only
					// ActorAdded writes"): introduced afresh at the re-add.
					qafmExpectFrames(t, "VTT-208+VTT-313: a different party member is introduced afresh, seen or not", r, readd, []string{
						"own:ActorRemoved y1", "own:ActorAdded y1",
					})
				} else {
					qafmExpectFrames(t, "VTT-313 corrected (on sight) / VTT-252: nothing at an identical re-add", r, readd, []string{})
				}
				qafmExpectFrames(t, "VTT-203/VTT-248: placed; nothing to correct", r, sight, []string{"own:Placed ty s1 y1 5,0"})
				qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, sight), r.worlds[sight], "y1")
			})
		}
	}
}

// ================================================================ correction order

// qafmTwoConditionOrders arranges for x1's conditions to sit in the viewer's
// fold in one order and in the world in the other, then hides x1 again.
func qafmTwoConditionOrders() (*qafmLog, int) {
	l := qafmBase()
	l.emit(qafmAdded(qafmLurker()))
	l.emit(qafmGrant("x1", "u2", qaNonParty))
	l.emit(qafmCondOn("x1", "a"))
	l.emit(qafmPlace("tx1", "x1", 3))
	l.emit(qafmTokenOff("tx1"))
	l.emit(qafmCondOff("x1", "a"))
	l.emit(qafmCondOn("x1", "b"))
	l.emit(qafmCondOn("x1", "a"))
	seen := l.emit(qafmPlace("tx2", "x1", 4))
	l.emit(qafmTokenOff("tx2"))
	return l, seen
}

func TestQAFoldMemoryACorrectionRemovesConditionsInTheViewersFoldOrder(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			l, seen := qafmTwoConditionOrders()
			l.emit(qafmCondOff("x1", "b"))
			l.emit(qafmCondOff("x1", "a"))
			l.emit(qafmCondOn("x1", "c"))
			sight := l.emit(qafmPlace("tx3", "x1", 5))
			r := qafmProject(t, v, l.envs)
			// Fixture, non-degenerate: when the viewer last saw x1 the world held
			// [b a] and the viewer's fold [a b].
			if got := qafmCondIDs(r.worlds[seen], "x1"); !reflect.DeepEqual(got, []string{"b", "a"}) {
				t.Fatalf("fixture: world order at last sight %v, want [b a]", got)
			}
			if got := qafmCondIDs(r.viewerFold(t, seen), "x1"); !reflect.DeepEqual(got, []string{"a", "b"}) {
				t.Fatalf("fixture: viewer fold order at last sight %v, want [a b]", got)
			}
			// Requirement, "Where SPEC-016 is now out of date": ConditionRemoved in
			// the order the VIEWER'S FOLD holds the conditions; SPEC-016 "then one
			// ConditionApplied per condition st holds and the belief does not".
			qafmExpectFrames(t, "requirement: removals in the viewer's fold order", r, sight, []string{
				"own:CondOff x1 a",
				"own:CondOff x1 b",
				"own:CondOn x1 c",
				"own:Placed tx3 s1 x1 5,0",
			})
			qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, sight), r.worlds[sight], "x1")
		})
	}
}

func qafmTwoControllerOrders() (*qafmLog, int) {
	l := qafmBase()
	l.emit(qafmAdded(qafmLurker()))
	l.emit(qafmGrant("x1", "u2", qaNonParty))
	l.emit(qafmPlace("tx1", "x1", 3))
	l.emit(qafmTokenOff("tx1"))
	l.emit(qafmRevoke("x1", "u2"))
	l.emit(qafmGrant("x1", "u3", qaNonParty))
	l.emit(qafmGrant("x1", "u2", qaNonParty))
	seen := l.emit(qafmPlace("tx2", "x1", 4))
	l.emit(qafmTokenOff("tx2"))
	return l, seen
}

func qafmCheckControllerFixture(t *testing.T, r *qafmRun, seen int) {
	t.Helper()
	if got := qafmCtl(r.worlds[seen], "x1"); !reflect.DeepEqual(got, []string{"u3", "u2"}) {
		t.Fatalf("fixture: world controllers at last sight %v, want [u3 u2]", got)
	}
	if got := qafmCtl(r.viewerFold(t, seen), "x1"); !reflect.DeepEqual(got, []string{"u2", "u3"}) {
		t.Fatalf("fixture: viewer fold controllers at last sight %v, want [u2 u3]", got)
	}
}

func TestQAFoldMemoryACorrectionRevokesInTheViewersFoldOrder(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			l, seen := qafmTwoControllerOrders()
			l.emit(qafmGrant("x1", "u4", qaNonParty))
			l.emit(qafmRevoke("x1", "u3"))
			l.emit(qafmRevoke("x1", "u2"))
			sight := l.emit(qafmPlace("tx3", "x1", 5))
			r := qafmProject(t, v, l.envs)
			qafmCheckControllerFixture(t, r, seen)
			// Requirement: "its ActorControlRevoked frames in the order the
			// viewer's fold holds the controllers"; SPEC-016: grants, "then" revokes.
			qafmExpectFrames(t, "requirement: revocations in the viewer's fold order", r, sight, []string{
				"own:Granted x1 u4 NON_PARTY",
				"own:Revoked x1 u2",
				"own:Revoked x1 u3",
				"own:Placed tx3 s1 x1 5,0",
			})
			qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, sight), r.worlds[sight], "x1")
		})
	}
}

func TestQAFoldMemoryAKindRestatementNamesTheViewersFirstController(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			l, seen := qafmTwoControllerOrders()
			regrant := l.emit(qafmGrant("x1", "u3", qaParty))
			sight := l.emit(qafmPlace("tx3", "x1", 5))
			r := qafmProject(t, v, l.envs)
			qafmCheckControllerFixture(t, r, seen)
			if got := r.worlds[regrant].Actors["x1"]; got.GetKind() != qaParty || !reflect.DeepEqual(got.GetControllerIds(), []string{"u3", "u2"}) {
				t.Fatalf("fixture: after the re-grant world x1 kind %v controllers %v", got.GetKind(), got.GetControllerIds())
			}
			qafmExpectFrames(t, "SPEC-016 Consequences: a held actor keeps the kind it last saw until seen", r, regrant, []string{})
			// Requirement: "the correction restates the FIRST CONTROLLER THE
			// VIEWER'S FOLD HOLDS, with the present kind".
			qafmExpectFrames(t, "requirement: kind restated on the viewer's first controller", r, sight, []string{
				"own:Granted x1 u2 PARTY",
				"own:Placed tx3 s1 x1 5,0",
			})
			qafmSameActorAsSets(t, "VTT-248", r.viewerFold(t, sight), r.worlds[sight], "x1")
		})
	}
}

// ================================================================ accepted limit

// Conditions and controllers that cross an unchanged re-add: sets only.
func TestQAFoldMemoryConditionsCrossingAnUnchangedReAddArriveAsSets(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			l := qafmBase()
			l.emit(qafmAdded(qafmLurker()))
			l.emit(qafmGrant("x1", "u2", qaNonParty))
			l.emit(qafmCondOn("x1", "a"))
			l.emit(qafmCondOn("x1", "b"))
			l.emit(qafmPlace("tx1", "x1", 3))
			l.emit(qafmTokenOff("tx1"))
			l.emit(qafmRemoved("x1"))
			l.emit(qafmAdded(qafmLurker()))
			l.emit(qafmCondOn("x1", "b"))
			l.emit(qafmCondOn("x1", "a"))
			l.emit(qafmGrant("x1", "u3", qaNonParty))
			l.emit(qafmGrant("x1", "u2", qaNonParty))
			sight := l.emit(qafmPlace("tx2", "x1", 4))
			r := qafmProject(t, v, l.envs)
			// VTT-313 corrected; VTT-250 "A change an actor undid while the viewer
			// did not see it sends nothing": only the new controller u3.
			qafmExpectFrames(t, "VTT-313/VTT-250", r, sight, []string{
				"own:Granted x1 u3 NON_PARTY",
				"own:Placed tx2 s1 x1 4,0",
			})
			qafmSameActorAsSets(t, "VTT-248 (requirement: accepted limit, sets)", r.viewerFold(t, sight), r.worlds[sight], "x1")
		})
	}
}

// ================================================================ VTT-314

// VTT-314
func TestQAFoldMemoryAViewerWhoseFoldFailsIsSentNothing(t *testing.T) {
	for _, v := range qafmViewers() {
		t.Run(string(v.Role), func(t *testing.T) {
			world := qafmBase()
			base := len(world.envs)
			world.emit(qafmNarrate("one"))
			world.emit(qafmCondOn("p1", "dazed"))
			world.emit(qafmNarrate("two"))

			drive := func(withBad bool) [][]*vttv1.Envelope {
				pr := gateway.NewProjector(v)
				var sent []*vttv1.Envelope
				var st *engine.State
				for i := 0; i < base; i++ {
					st, _ = campaign.FoldPrefix(world.envs[:i+1])
					sent = append(sent, pr.Project(world.envs[i], st)...)
				}
				if withBad {
					// A frame the viewer's fold refuses: the eye's removal while its
					// token stands on the board, judged against a world that still
					// holds both, so the projector forwards it (VTT-251).
					bad := &vttv1.Envelope{EventId: "ev-bad", Sequence: int64(base + 1), Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: "p1"}}}
					if err := engine.Apply(st.Snapshot(), bad); err == nil {
						t.Fatal("fixture: engine.Apply accepts removing an actor whose token is on the board")
					}
					out := pr.Project(bad, st)
					if got := qafmDescribeAll(out); !reflect.DeepEqual(got, []string{"ActorRemoved p1"}) {
						t.Fatalf("fixture: the inconsistent removal was not forwarded: %q", got)
					}
					sent = append(sent, out...)
					if _, err := campaign.FoldPrefix(sent); err == nil {
						t.Fatal("fixture: the viewer's fold should fail now")
					}
				}
				var after [][]*vttv1.Envelope
				for i := base; i < len(world.envs); i++ {
					st, _ = campaign.FoldPrefix(world.envs[:i+1])
					after = append(after, pr.Project(world.envs[i], st))
				}
				return after
			}
			control := drive(false)
			if len(control[0]) == 0 || len(control[1]) == 0 || len(control[2]) == 0 {
				t.Fatalf("control: a healthy viewer is sent narration and its eye's condition: %v", control)
			}
			for i, out := range drive(true) {
				if len(out) != 0 {
					t.Errorf("VTT-314: event after the failing fold (#%d) sent %q", i+1, qafmDescribeAll(out))
				}
			}
		})
	}
}
