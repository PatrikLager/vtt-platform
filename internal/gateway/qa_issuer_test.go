package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaIssP1      = "qa-iss-player-one"
	qaIssP2      = "qa-iss-player-two"
	qaIssDM      = "qa-iss-dm"
	qaIssAgent   = "qa-iss-agent"
	qaIssSpec    = "qa-iss-spectator"
	qaIssSession = "qa-iss-session"
	qaIssDazed   = "dazed-by-ale"
)

var qaIssForwardedKinds = []string{
	"session_started", "session_ended", "narration_added", "note_upserted",
	"token_moved", "door_opened", "door_closed", "attack_rolled",
	"ability_used", "actor_control_granted", "actor_control_revoked",
	"resource_changed", "condition_applied", "condition_removed",
	"actor_removed",
}

type qaIssScript struct {
	log []*vttv1.Envelope
	ids map[string]string
}

type qaIssBuilder struct {
	t   *testing.T
	st  *engine.State
	seq int64
	out qaIssScript
}

func (b *qaIssBuilder) emit(name, pid string, role identity.Role, env *vttv1.Envelope) {
	b.t.Helper()
	b.seq++
	env.EventId = fmt.Sprintf("qa-iss-ev-%02d", b.seq)
	env.Sequence = b.seq
	env.OccurredAt = timestamppb.New(time.Unix(1_800_000_000+b.seq, 0))
	env.SessionId = qaIssSession
	env.ParticipantId = pid
	env.ActorRole = string(role)
	b.out.log = append(b.out.log, proto.Clone(env).(*vttv1.Envelope))
	if err := engine.Apply(b.st, proto.Clone(env).(*vttv1.Envelope)); err != nil {
		b.t.Fatalf("fixture event %d (%s) does not fold: %v", b.seq, name, err)
	}
	if name != "" {
		b.out.ids[name] = env.EventId
	}
}

func qaIssActor(id string, kind vttv1.ActorKind) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       id,
		Kind:       kind,
		Attributes: map[string]int32{"brawn": 3, "grit": 1, "footing": 0},
		Resources: map[string]*vttv1.Resource{
			"drink":   {Current: 0, Max: 5},
			"stamina": {Current: 2, Max: 2},
		},
	}
}

func qaIssAt(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }

func qaIssRC(actor, resource string, delta, now int32, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: actor, Resource: resource, Delta: delta, NewValue: now, Reason: reason,
	}}}
}

func qaIssCA(actor, source string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: qaIssDazed, Source: source,
	}}}
}

func qaIssCR(actor, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
		ActorId: actor, ConditionId: qaIssDazed, Reason: reason,
	}}}
}

func qaIssPlaced(tok, scene, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: scene, ActorId: actor, Position: qaIssAt(x, y),
	}}}
}

func qaIssGrant(actor, pid string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: pid, Kind: kind,
	}}}
}

func qaIssUsed(actor, ability, summary string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityUsed{AbilityUsed: &vttv1.AbilityUsed{
		ActorId: actor, AbilityId: ability, TargetIds: []string{"patron"},
		Rolls:          []*vttv1.AbilityUsed_Roll{{Expression: "1d20 + @caster.brawn", Results: []int32{9}, Total: 12}},
		OutcomeSummary: summary,
	}}}
}

func qaIssBuildScript(t *testing.T) qaIssScript {
	t.Helper()
	b := &qaIssBuilder{t: t, st: engine.NewState(), out: qaIssScript{ids: map[string]string{}}}
	dm := func(name string, env *vttv1.Envelope) { b.t.Helper(); b.emit(name, qaIssDM, identity.RoleDM, env) }
	p1 := func(name string, env *vttv1.Envelope) { b.t.Helper(); b.emit(name, qaIssP1, identity.RolePlayer, env) }
	party, other := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY

	dm("session-started", &vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "qa night"}}})
	dm("", &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "hall", Name: "Hall", GridWidth: 10, GridHeight: 10,
		Tiles: map[string]*vttv1.TileRef{"6,1": {Kind: "door"}},
	}}})
	dm("", &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "cellar", Name: "Cellar", GridWidth: 4, GridHeight: 4,
	}}})
	for _, a := range []*vttv1.Actor{qaIssActor("hero", party), qaIssActor("patron", other), qaIssActor("lurker", other), qaIssActor("familiar", other)} {
		dm("", &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}})
	}
	dm("", qaIssGrant("hero", qaIssP1, party))
	dm("", qaIssGrant("familiar", qaIssP1, other))
	dm("", qaIssPlaced("hero-tok", "hall", "hero", 1, 1))
	dm("", qaIssPlaced("patron-tok", "hall", "patron", 3, 1))
	dm("", qaIssPlaced("lurker-tok", "cellar", "lurker", 1, 1))

	p1("own-narration", &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
		Text: "knuckles crack", As: "Hero", AnchorFromSeq: 1, AnchorToSeq: 2,
	}}})
	p1("own-move", &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: "hero-tok", SceneId: "hall", From: qaIssAt(1, 1), To: qaIssAt(2, 1), Reason: "edging toward the bar",
	}}})
	p1("seen-used", qaIssUsed("hero", "fists", "Fists on patron: hit (12 vs 0)"))
	p1("seen-rc", qaIssRC("patron", "drink", 1, 1, "ability:fists:hit"))
	p1("seen-ca", qaIssCA("patron", "threshold:drink"))

	dm("unseen-used", qaIssUsed("lurker", "chair-swing", "Chair Swing on patron: hit (12 vs 0)"))
	dm("unseen-usage", qaIssRC("lurker", "stamina", -1, 1, "ability:chair-swing:usage"))
	dm("unseen-rc", qaIssRC("patron", "drink", 3, 4, "ability:chair-swing:hit"))
	dm("unseen-cr", qaIssCR("patron", "ability:splash-of-water:effect"))
	dm("unseen-ca", qaIssCA("patron", "ability:chair-swing:hit"))
	dm("by-hand-cr", qaIssCR("patron", "removed by hand"))

	b.emit("attack", qaIssAgent, identity.RoleAgent, &vttv1.Envelope{Payload: &vttv1.Envelope_AttackRolled{AttackRolled: &vttv1.AttackRolled{
		AttackerId: "hero", TargetId: "patron", Expression: "1d20+3",
		Rolls:     []*vttv1.DieRoll{{Die: 20, Result: 9}},
		Modifiers: []*vttv1.Modifier{{Source: "brawn", Value: 3}},
		Total:     12, Versus: "footing", Outcome: "hit",
	}}})
	dm("door-opened", &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{SceneId: "hall", At: qaIssAt(6, 1)}}})
	dm("door-closed", &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{DoorClosed: &vttv1.DoorClosed{SceneId: "hall", At: qaIssAt(6, 1)}}})
	dm("public-note", &vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: "rumour", Title: "Rumour", Text: "the cellar is not empty", Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC,
	}}})
	dm("grant-other", qaIssGrant("patron", qaIssP2, other))
	dm("revoke-other", &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
		ActorId: "patron", ParticipantId: qaIssP2,
	}}})
	dm("eye-removed", &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: "familiar"}}})

	dm("", &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "patron-tok"}}})
	dm("hidden-rc", qaIssRC("patron", "drink", 1, 5, "ability:fists:hit"))
	dm("hidden-ca", qaIssCA("patron", "threshold:drink"))
	dm("patron-back", qaIssPlaced("patron-tok-b", "hall", "patron", 3, 1))

	dm("", qaIssCA("lurker", "ability:chair-swing:hit"))
	dm("", &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "lurker-tok"}}})
	dm("lurker-arrives", qaIssPlaced("lurker-tok-b", "hall", "lurker", 5, 1))
	dm("session-ended", &vttv1.Envelope{Payload: &vttv1.Envelope_SessionEnded{SessionEnded: &vttv1.SessionEnded{}}})
	return b.out
}

type qaIssFrame struct {
	cause *vttv1.Envelope
	frame *vttv1.Envelope
}

func qaIssStateJSON(t *testing.T, st *engine.State) string {
	t.Helper()
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	return string(b)
}

func qaIssReplay(t *testing.T, v gateway.Viewer, log []*vttv1.Envelope) []qaIssFrame {
	t.Helper()
	pr := gateway.NewProjector(v)
	st := engine.NewState()
	var out []qaIssFrame
	for _, orig := range log {
		env := proto.Clone(orig).(*vttv1.Envelope)
		if err := engine.Apply(st, proto.Clone(orig).(*vttv1.Envelope)); err != nil {
			t.Fatalf("replay %s: %v", orig.GetEventId(), err)
		}
		envBefore := proto.Clone(env).(*vttv1.Envelope)
		stBefore := qaIssStateJSON(t, st)
		frames := pr.Project(env, st)
		if !proto.Equal(env, envBefore) {
			t.Errorf("%s: projecting %s changed the event:\nbefore %v\nafter  %v", v.Role, env.GetEventId(), envBefore, env)
		}
		if got := qaIssStateJSON(t, st); got != stBefore {
			t.Errorf("%s: projecting %s changed the state", v.Role, env.GetEventId())
		}
		for _, f := range frames {
			out = append(out, qaIssFrame{cause: env, frame: f})
		}
	}
	return out
}

func qaIssKind(env *vttv1.Envelope) string {
	m := env.ProtoReflect()
	fd := m.WhichOneof(m.Descriptor().Oneofs().ByName("payload"))
	if fd == nil {
		return ""
	}
	return string(fd.Name())
}

func qaIssCause(env *vttv1.Envelope) string {
	switch p := env.GetPayload().(type) {
	case *vttv1.Envelope_TokenMoved:
		return p.TokenMoved.GetReason()
	case *vttv1.Envelope_ResourceChanged:
		return p.ResourceChanged.GetReason()
	case *vttv1.Envelope_ConditionApplied:
		return p.ConditionApplied.GetSource()
	case *vttv1.Envelope_ConditionRemoved:
		return p.ConditionRemoved.GetReason()
	}
	return ""
}

func qaIssLessIssuerAndCause(env *vttv1.Envelope) *vttv1.Envelope {
	want := proto.Clone(env).(*vttv1.Envelope)
	want.ParticipantId = ""
	want.ActorRole = ""
	switch p := want.GetPayload().(type) {
	case *vttv1.Envelope_TokenMoved:
		p.TokenMoved.Reason = ""
	case *vttv1.Envelope_ResourceChanged:
		p.ResourceChanged.Reason = ""
	case *vttv1.Envelope_ConditionApplied:
		p.ConditionApplied.Source = ""
	case *vttv1.Envelope_ConditionRemoved:
		p.ConditionRemoved.Reason = ""
	}
	return want
}

func qaIssCheckFrames(t *testing.T, who string, got []qaIssFrame) map[string]*vttv1.Envelope {
	t.Helper()
	forwarded := map[string]*vttv1.Envelope{}
	for _, g := range got {
		f := g.frame
		if f.GetParticipantId() != "" || f.GetActorRole() != "" {
			t.Errorf("%s: frame %s (cause %s) carries issuer participant=%q role=%q",
				who, qaIssKind(f), g.cause.GetEventId(), f.GetParticipantId(), f.GetActorRole())
		}
		if c := qaIssCause(f); c != "" {
			t.Errorf("%s: frame %s (cause %s) carries cause %q", who, qaIssKind(f), g.cause.GetEventId(), c)
		}
		if f.GetEventId() == "" {
			continue
		}
		if f.GetEventId() != g.cause.GetEventId() {
			t.Errorf("%s: frame for %s carries event id %q", who, g.cause.GetEventId(), f.GetEventId())
		}
		if want := qaIssLessIssuerAndCause(g.cause); !proto.Equal(f, want) {
			t.Errorf("%s: forwarded %s is not the event less issuer and cause:\n got  %v\n want %v", who, f.GetEventId(), f, want)
		}
		forwarded[f.GetEventId()] = f
	}
	return forwarded
}

// SPEC-016 How it works: "No envelope `Project` sends a player or a spectator
// carries its own `participant_id` or `actor_role`".
// VTT-272 VTT-273 VTT-262 VTT-222
func TestQAIssEveryForwardedKindReachesAPlayerWithoutIssuerOrCause(t *testing.T) {
	s := qaIssBuildScript(t)
	got := qaIssReplay(t, gateway.Viewer{ParticipantID: qaIssP1, Role: identity.RolePlayer}, s.log)
	forwarded := qaIssCheckFrames(t, "player", got)

	kinds := map[string]bool{}
	for _, f := range forwarded {
		kinds[qaIssKind(f)] = true
	}
	for _, k := range qaIssForwardedKinds {
		if !kinds[k] {
			t.Errorf("player: no %s was forwarded, so its issuer was never put to the test", k)
		}
	}
	for _, name := range []string{"own-narration", "own-move", "seen-used", "seen-rc", "seen-ca"} {
		if forwarded[s.ids[name]] == nil {
			t.Errorf("player: its own %s (%s) was not echoed back", name, s.ids[name])
		}
	}
}

// SPEC-016 How each payload is ruled: "an ability's user may be an actor the
// viewer does not see".
// VTT-273 VTT-243 VTT-244
func TestQAIssAStatusChangeArrivesWithoutCauseWhetherOrNotItsUserIsSeen(t *testing.T) {
	s := qaIssBuildScript(t)
	viewers := []gateway.Viewer{
		{ParticipantID: qaIssP1, Role: identity.RolePlayer},
		{ParticipantID: qaIssSpec, Role: identity.RoleSpectator, Viewpoint: "hero"},
	}
	for _, v := range viewers {
		forwarded := qaIssCheckFrames(t, string(v.Role), qaIssReplay(t, v, s.log))
		for _, name := range []string{"seen-rc", "seen-ca", "unseen-rc", "unseen-cr", "unseen-ca", "by-hand-cr"} {
			f := forwarded[s.ids[name]]
			if f == nil {
				t.Errorf("%s: %s (%s) did not arrive", v.Role, name, s.ids[name])
				continue
			}
			if c := qaIssCause(f); c != "" {
				t.Errorf("%s: %s arrived with cause %q", v.Role, name, c)
			}
		}
		for _, name := range []string{"unseen-used", "unseen-usage", "hidden-rc", "hidden-ca"} {
			if forwarded[s.ids[name]] != nil {
				t.Errorf("%s: %s (%s) names an actor it did not see, and was forwarded", v.Role, name, s.ids[name])
			}
		}
	}
}

// SPEC-016 Consequences: "one with an event id is the event less its issuer
// and its cause".
// VTT-272 VTT-273 VTT-262
func TestQAIssAPerchedSpectatorAndABystanderAreSentNoIssuer(t *testing.T) {
	s := qaIssBuildScript(t)
	spec := qaIssCheckFrames(t, "spectator", qaIssReplay(t,
		gateway.Viewer{ParticipantID: qaIssSpec, Role: identity.RoleSpectator, Viewpoint: "hero"}, s.log))
	for _, name := range []string{"own-move", "own-narration", "seen-used", "attack", "grant-other", "session-ended"} {
		if spec[s.ids[name]] == nil {
			t.Errorf("perched spectator: %s (%s) did not arrive", name, s.ids[name])
		}
	}
	other := qaIssCheckFrames(t, "player-two", qaIssReplay(t,
		gateway.Viewer{ParticipantID: qaIssP2, Role: identity.RolePlayer}, s.log))
	for _, name := range []string{"session-started", "own-narration", "public-note"} {
		if other[s.ids[name]] == nil {
			t.Errorf("player-two: %s (%s) did not arrive", name, s.ids[name])
		}
	}
}

// SPEC-016 How it works: "every other field as the event has it"
func TestQAIssAForwardedFrameKeepsEverythingButIssuerAndCause(t *testing.T) {
	s := qaIssBuildScript(t)
	byID := map[string]*vttv1.Envelope{}
	for _, e := range s.log {
		byID[e.GetEventId()] = e
	}
	forwarded := qaIssCheckFrames(t, "player", qaIssReplay(t, gateway.Viewer{ParticipantID: qaIssP1, Role: identity.RolePlayer}, s.log))
	for _, name := range []string{"own-move", "unseen-rc", "attack", "grant-other"} {
		f, e := forwarded[s.ids[name]], byID[s.ids[name]]
		if f == nil {
			t.Fatalf("%s was not forwarded", name)
		}
		if f.GetSequence() != e.GetSequence() || f.GetSessionId() != qaIssSession ||
			!proto.Equal(f.GetOccurredAt(), e.GetOccurredAt()) || f.GetEventId() != e.GetEventId() {
			t.Errorf("%s lost its event id, sequence, time or session: %v", name, f)
		}
	}
	if m := forwarded[s.ids["attack"]].GetAttackRolled().GetModifiers(); len(m) != 1 || m[0].GetSource() != "brawn" {
		t.Errorf("an attack's modifier source is a field of the payload and was not kept: %v", m)
	}
	if p := forwarded[s.ids["grant-other"]].GetActorControlGranted().GetParticipantId(); p != qaIssP2 {
		t.Errorf("a grant's own participant_id is not the issuer and was not kept: %q", p)
	}
	if p := forwarded[s.ids["revoke-other"]].GetActorControlRevoked().GetParticipantId(); p != qaIssP2 {
		t.Errorf("a revocation's own participant_id is not the issuer and was not kept: %q", p)
	}
	if n := forwarded[s.ids["own-narration"]].GetNarrationAdded(); n.GetAs() != "Hero" || n.GetAnchorToSeq() != 2 {
		t.Errorf("a narration's own fields were not kept: %v", n)
	}
	if mv := forwarded[s.ids["own-move"]].GetTokenMoved(); mv.GetTo().GetX() != 2 || mv.GetFrom().GetX() != 1 {
		t.Errorf("a move's ends were not kept: %v", mv)
	}
}

// SPEC-016 How it works: "every frame it builds is new"
func TestQAIssAForwardedFrameIsACopyNotTheEvent(t *testing.T) {
	s := qaIssBuildScript(t)
	for _, g := range qaIssReplay(t, gateway.Viewer{ParticipantID: qaIssP1, Role: identity.RolePlayer}, s.log) {
		if g.frame.GetEventId() == "" {
			continue
		}
		if g.frame == g.cause {
			t.Errorf("forwarded %s is the event itself", g.cause.GetEventId())
		}
		if rc := g.frame.GetResourceChanged(); rc != nil && rc == g.cause.GetResourceChanged() {
			t.Errorf("forwarded %s shares its payload with the event", g.cause.GetEventId())
		}
	}
}

// SPEC-016 The correction on sight: "no event id, time, role, participant or
// session, and no reason or source".
// VTT-249
func TestQAIssACorrectionCarriesNothingOfAnEnvelopeButItsSequence(t *testing.T) {
	s := qaIssBuildScript(t)
	viewers := []gateway.Viewer{
		{ParticipantID: qaIssP1, Role: identity.RolePlayer},
		{ParticipantID: qaIssSpec, Role: identity.RoleSpectator, Viewpoint: "hero"},
	}
	for _, v := range viewers {
		var rc, ca int
		for _, g := range qaIssReplay(t, v, s.log) {
			if g.cause.GetEventId() != s.ids["patron-back"] || g.frame.GetEventId() != "" {
				continue
			}
			f := g.frame
			switch {
			case f.GetResourceChanged().GetActorId() == "patron":
				rc++
				if f.GetResourceChanged().GetNewValue() != 5 {
					t.Errorf("%s: correction %v does not bring drink to 5", v.Role, f)
				}
			case f.GetConditionApplied().GetActorId() == "patron":
				ca++
			default:
				continue
			}
			if f.GetSequence() != g.cause.GetSequence() || f.GetOccurredAt() != nil || f.GetSessionId() != "" ||
				f.GetParticipantId() != "" || f.GetActorRole() != "" || qaIssCause(f) != "" {
				t.Errorf("%s: correction carries more than its sequence: %v", v.Role, f)
			}
		}
		if rc != 1 || ca != 1 {
			t.Errorf("%s: patron's return sent %d resource and %d condition corrections, want 1 and 1", v.Role, rc, ca)
		}
	}
}

// SPEC-016 What transitions sends: "one `ConditionApplied` per condition `st`
// holds for it, carrying its id and no source"
func TestQAIssAnIntroducedConditionCarriesNoSource(t *testing.T) {
	s := qaIssBuildScript(t)
	var n int
	for _, g := range qaIssReplay(t, gateway.Viewer{ParticipantID: qaIssP1, Role: identity.RolePlayer}, s.log) {
		ca := g.frame.GetConditionApplied()
		if g.cause.GetEventId() != s.ids["lurker-arrives"] || ca.GetActorId() != "lurker" {
			continue
		}
		n++
		if ca.GetSource() != "" || ca.GetConditionId() != qaIssDazed || g.frame.GetEventId() != "" ||
			g.frame.GetParticipantId() != "" || g.frame.GetActorRole() != "" {
			t.Errorf("introduced condition carries more than its id: %v", g.frame)
		}
	}
	if n != 1 {
		t.Errorf("lurker's introduction carried %d conditions, want 1", n)
	}
}

// SPEC-016 How it works: "It answers `identity.RoleDM` and `identity.RoleAgent`
// with the event itself, the same pointer, and reads no state".
// VTT-176 VTT-222
func TestQAIssTheDMAndTheAgentAreSentTheEventItself(t *testing.T) {
	s := qaIssBuildScript(t)
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		pr := gateway.NewProjector(gateway.Viewer{ParticipantID: "qa-iss-" + string(role), Role: role})
		var issued, caused, logged int
		for _, orig := range s.log {
			if qaIssCause(orig) != "" {
				logged++
			}
			env := proto.Clone(orig).(*vttv1.Envelope)
			got := pr.Project(env, nil)
			if len(got) != 1 || got[0] != env {
				t.Fatalf("%s: %s answered %d frames, not the event itself", role, env.GetEventId(), len(got))
			}
			if !proto.Equal(env, orig) {
				t.Errorf("%s: projecting %s changed it", role, env.GetEventId())
			}
			if got[0].GetParticipantId() != "" && got[0].GetActorRole() != "" {
				issued++
			}
			if qaIssCause(got[0]) != "" {
				caused++
			}
		}
		if issued != len(s.log) || caused != logged || logged == 0 {
			t.Errorf("%s: %d of %d events kept their issuer and %d of %d their cause", role, issued, len(s.log), caused, logged)
		}
	}
}

type qaIssConn struct {
	t      *testing.T
	ws     *websocket.Conn
	in     chan *vttv1.ServerFrame
	got    []*vttv1.ServerFrame
	reqSeq int
}

func qaIssDial(t *testing.T, base, token string) *qaIssConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(base, "http") + "/ws?token=" + url.QueryEscape(token) + "&after=0"
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ws.SetReadLimit(1 << 22)
	c := &qaIssConn{t: t, ws: ws, in: make(chan *vttv1.ServerFrame, 8192)}
	go func() {
		defer close(c.in)
		for {
			_, data, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if protojson.Unmarshal(data, f) != nil {
				f = &vttv1.ServerFrame{}
			}
			c.in <- f
		}
	}()
	t.Cleanup(func() { _ = ws.CloseNow() })
	return c
}

func (c *qaIssConn) until(desc string, pred func(*vttv1.ServerFrame) bool) {
	c.t.Helper()
	for _, f := range c.got {
		if pred(f) {
			return
		}
	}
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				c.t.Fatalf("connection closed while waiting for %s", desc)
			}
			if f.GetFrame() == nil {
				c.t.Fatalf("undecodable frame while waiting for %s", desc)
			}
			c.got = append(c.got, f)
			if pred(f) {
				return
			}
		case <-time.After(10 * time.Second):
			c.t.Fatalf("no %s within 10s", desc)
		}
	}
}

func (c *qaIssConn) do(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	c.reqSeq++
	cmd.RequestId = fmt.Sprintf("qa-iss-req-%d", c.reqSeq)
	b, err := protojson.Marshal(cmd)
	if err != nil {
		c.t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, b); err != nil {
		c.t.Fatalf("write: %v", err)
	}
	var res *vttv1.CommandResult
	c.until("result "+cmd.RequestId, func(f *vttv1.ServerFrame) bool {
		if r := f.GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
			res = r
			return true
		}
		return false
	})
	return res
}

func (c *qaIssConn) must(cmd *vttv1.ClientCommand) int64 {
	c.t.Helper()
	r := c.do(cmd)
	if !r.GetOk() {
		c.t.Fatalf("command %v refused: %s", cmd, r.GetError())
	}
	return r.GetSequence()
}

func (c *qaIssConn) untilNarration(text string) {
	c.t.Helper()
	c.until("narration "+text, func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetNarrationAdded().GetText() == text
	})
}

func (c *qaIssConn) events() []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range c.got {
		if e := f.GetEvent(); e != nil {
			out = append(out, e)
		}
	}
	return out
}

type qaIssTable struct {
	t      *testing.T
	c      *campaign.Campaign
	url    string
	tokens map[identity.Role]string
	ids    map[identity.Role]string
}

func qaIssOpenTable(t *testing.T) *qaIssTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	rs, err := rules.Load("../../rulesets/tavern-brawl")
	if err != nil {
		t.Fatalf("ruleset: %v", err)
	}
	hs := httptest.NewServer(gateway.New(c, ids).WithRuleset(rs).Handler())
	t.Cleanup(hs.Close)
	tb := &qaIssTable{t: t, c: c, url: hs.URL, tokens: map[identity.Role]string{}, ids: map[identity.Role]string{}}
	for _, r := range []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator} {
		tok, id, err := ids.CreateInvite("qa-iss-"+string(r), r)
		if err != nil {
			t.Fatalf("invite: %v", err)
		}
		tb.tokens[r], tb.ids[r] = tok, id
	}
	return tb
}

func (tb *qaIssTable) dial(r identity.Role) *qaIssConn {
	tb.t.Helper()
	return qaIssDial(tb.t, tb.url, tb.tokens[r])
}

func (tb *qaIssTable) appendAsDM(id string, env *vttv1.Envelope) {
	tb.t.Helper()
	env.EventId = id
	env.OccurredAt = timestamppb.Now()
	env.ParticipantId = tb.ids[identity.RoleDM]
	env.ActorRole = string(identity.RoleDM)
	if _, err := tb.c.Append(env); err != nil {
		tb.t.Fatalf("append %s: %v", id, err)
	}
}

func (tb *qaIssTable) readLog() []*vttv1.Envelope {
	tb.t.Helper()
	ch, unsub, head, err := tb.c.Subscribe(0, 8192)
	if err != nil {
		tb.t.Fatalf("subscribe: %v", err)
	}
	defer unsub()
	var out []*vttv1.Envelope
	for head > 0 {
		select {
		case e := <-ch:
			out = append(out, proto.Clone(e).(*vttv1.Envelope))
			if e.GetSequence() >= head {
				return out
			}
		case <-time.After(10 * time.Second):
			tb.t.Fatalf("log catch-up stalled at %d of %d", len(out), head)
		}
	}
	return out
}

func qaIssCmdAbility(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{UseAbility: &vttv1.UseAbility{
		ActorId: actor, AbilityId: ability, TargetIds: []string{"patron"},
	}}}
}

func qaIssCmdNarrate(text string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: text}}}
}

func qaIssCmdPlace(tok, scene, actor string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
		TokenId: tok, SceneId: scene, ActorId: actor, Position: qaIssAt(x, y),
	}}}
}

func qaIssCmdPerch(actor string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{SetViewpoint: &vttv1.SetViewpoint{ActorId: actor}}}
}

const qaIssMarker = "qa-iss-marker"

type qaIssPlay struct {
	tb     *qaIssTable
	dm, ag *qaIssConn
	p1, sp *qaIssConn
	actsAt int64
}

func qaIssPlayTable(t *testing.T) *qaIssPlay {
	t.Helper()
	tb := qaIssOpenTable(t)
	pl := &qaIssPlay{tb: tb}
	pl.dm, pl.ag = tb.dial(identity.RoleDM), tb.dial(identity.RoleAgent)
	pl.p1, pl.sp = tb.dial(identity.RolePlayer), tb.dial(identity.RoleSpectator)

	tb.appendAsDM("qa-iss-hall", &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "hall", Name: "Hall", GridWidth: 10, GridHeight: 10,
	}}})
	tb.appendAsDM("qa-iss-cellar", &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "cellar", Name: "Cellar", GridWidth: 4, GridHeight: 4,
	}}})
	pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "qa night"}}})
	party, other := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	for _, a := range []*vttv1.Actor{qaIssActor("hero", party), qaIssActor("patron", other), qaIssActor("lurker", other)} {
		pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: a}}})
	}
	pl.dm.must(qaIssCmdPlace("hero-tok", "hall", "hero", 1, 1))
	pl.dm.must(qaIssCmdPlace("patron-tok", "hall", "patron", 3, 1))
	pl.dm.must(qaIssCmdPlace("patron-cellar-tok", "cellar", "patron", 2, 1))
	pl.dm.must(qaIssCmdPlace("lurker-tok", "cellar", "lurker", 1, 1))
	pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
		ActorId: "hero", ParticipantId: tb.ids[identity.RolePlayer], Kind: party,
	}}})
	pl.sp.must(qaIssCmdPerch("hero"))
	pl.dm.must(qaIssCmdNarrate("perched"))
	pl.sp.untilNarration("perched")

	reason := "edging toward the bar"
	pl.actsAt = pl.p1.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: "hero-tok", To: qaIssAt(2, 1), Reason: &reason,
	}}})
	pl.p1.must(qaIssCmdNarrate("knuckles crack"))
	pl.dm.must(qaIssCmdAbility("lurker", "chair-swing"))
	pl.dm.must(qaIssCmdAbility("lurker", "splash-of-water"))
	pl.p1.must(qaIssCmdAbility("hero", "chair-swing"))
	pl.dm.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveCondition{RemoveCondition: &vttv1.RemoveCondition{
		ActorId: "patron", ConditionId: qaIssDazed,
	}}})
	pl.dm.must(qaIssCmdAbility("lurker", "fists"))
	pl.ag.must(qaIssCmdNarrate("the barkeep sighs"))
	pl.dm.must(qaIssCmdNarrate(qaIssMarker))
	for _, c := range []*qaIssConn{pl.dm, pl.ag, pl.p1, pl.sp} {
		c.untilNarration(qaIssMarker)
	}
	return pl
}

func qaIssNames(e *vttv1.Envelope) []string {
	switch {
	case e.GetAbilityUsed() != nil:
		return append([]string{e.GetAbilityUsed().GetActorId()}, e.GetAbilityUsed().GetTargetIds()...)
	case e.GetResourceChanged() != nil:
		return []string{e.GetResourceChanged().GetActorId()}
	case e.GetConditionApplied() != nil:
		return []string{e.GetConditionApplied().GetActorId()}
	case e.GetConditionRemoved() != nil:
		return []string{e.GetConditionRemoved().GetActorId()}
	}
	return nil
}

func qaIssCheckWire(t *testing.T, who string, c *qaIssConn, log []*vttv1.Envelope, from int64) {
	t.Helper()
	byID := map[string]*vttv1.Envelope{}
	for _, e := range log {
		byID[e.GetEventId()] = e
	}
	sent := map[string]bool{}
	for _, f := range c.events() {
		if f.GetParticipantId() != "" || f.GetActorRole() != "" {
			t.Errorf("%s: sent %s seq %d with issuer participant=%q role=%q", who, qaIssKind(f), f.GetSequence(), f.GetParticipantId(), f.GetActorRole())
		}
		if cause := qaIssCause(f); cause != "" {
			t.Errorf("%s: sent %s seq %d with cause %q", who, qaIssKind(f), f.GetSequence(), cause)
		}
		if f.GetEventId() == "" {
			continue
		}
		e := byID[f.GetEventId()]
		if e == nil {
			t.Errorf("%s: sent event id %q the log does not hold", who, f.GetEventId())
			continue
		}
		if want := qaIssLessIssuerAndCause(e); !proto.Equal(f, want) {
			t.Errorf("%s: sent %s that is not the logged event less issuer and cause:\n got  %v\n want %v", who, f.GetEventId(), f, want)
		}
		sent[f.GetEventId()] = true
	}
	var reached, kept, issued int
	for _, e := range log {
		if e.GetSequence() < from {
			continue
		}
		if e.GetParticipantId() != "" && e.GetActorRole() != "" {
			issued++
		}
		names := qaIssNames(e)
		if names == nil && e.GetTokenMoved() == nil && e.GetNarrationAdded() == nil {
			continue
		}
		unseen := false
		for _, n := range names {
			unseen = unseen || n == "lurker"
		}
		switch {
		case unseen && sent[e.GetEventId()]:
			t.Errorf("%s: %s seq %d names lurker, whom it never saw, and was sent", who, qaIssKind(e), e.GetSequence())
		case !unseen && !sent[e.GetEventId()]:
			t.Errorf("%s: %s seq %d (cause %q) was not sent", who, qaIssKind(e), e.GetSequence(), qaIssCause(e))
		case !unseen:
			reached++
			if qaIssCause(e) != "" {
				kept++
			}
		}
	}
	if reached < 14 || kept < 10 || issued == 0 {
		t.Errorf("%s: only %d events reached it, %d with a logged cause, %d issued: the scenario proves nothing", who, reached, kept, issued)
	}
}

// VTT-272 VTT-273 VTT-262 VTT-243 VTT-244
func TestQAIssALivePlayerAndAPerchedSpectatorAreSentNoIssuerOrCause(t *testing.T) {
	pl := qaIssPlayTable(t)
	log := pl.tb.readLog()
	qaIssCheckWire(t, "live player", pl.p1, log, pl.actsAt)
	qaIssCheckWire(t, "live perched spectator", pl.sp, log, pl.actsAt)
}

// VTT-272 VTT-273 VTT-262 VTT-249
func TestQAIssACatchUpIsSentNoIssuerOrCause(t *testing.T) {
	pl := qaIssPlayTable(t)
	log := pl.tb.readLog()
	p1 := pl.tb.dial(identity.RolePlayer)
	p1.untilNarration(qaIssMarker)
	qaIssCheckWire(t, "catch-up player", p1, log, pl.actsAt)

	sp := pl.tb.dial(identity.RoleSpectator)
	sp.untilNarration(qaIssMarker)
	sp.must(qaIssCmdPerch("hero"))
	sp.until("hero's stamina correction", func(f *vttv1.ServerFrame) bool {
		e := f.GetEvent()
		return e.GetEventId() == "" && e.GetResourceChanged().GetActorId() == "hero"
	})
	sp.until("patron's introduced condition", func(f *vttv1.ServerFrame) bool {
		e := f.GetEvent()
		return e.GetEventId() == "" && e.GetConditionApplied().GetActorId() == "patron"
	})
	for _, e := range sp.events() {
		if e.GetParticipantId() != "" || e.GetActorRole() != "" || qaIssCause(e) != "" {
			t.Errorf("catch-up spectator: %s seq %d carries issuer or cause: %v", qaIssKind(e), e.GetSequence(), e)
		}
		correction := e.GetResourceChanged() != nil || e.GetConditionRemoved() != nil
		if e.GetEventId() == "" && correction && (e.GetSessionId() != "" || e.GetOccurredAt() != nil) {
			t.Errorf("catch-up spectator: correction %s carries session or time: %v", qaIssKind(e), e)
		}
	}
}

// SPEC-016 Consequences: "the DM and the agent are sent each event as the log
// holds it".
// VTT-176 VTT-222
func TestQAIssTheDMAndTheAgentAreSentEachEventAsLogged(t *testing.T) {
	pl := qaIssPlayTable(t)
	log := pl.tb.readLog()
	late := pl.tb.dial(identity.RoleDM)
	late.untilNarration(qaIssMarker)
	var issued, caused int
	for _, e := range log {
		if e.GetParticipantId() != "" && e.GetActorRole() != "" {
			issued++
		}
		if qaIssCause(e) != "" {
			caused++
		}
	}
	if issued != len(log) || caused < 10 {
		t.Fatalf("log premise: %d of %d events issued, %d caused", issued, len(log), caused)
	}
	for who, c := range map[string]*qaIssConn{"live dm": pl.dm, "live agent": pl.ag, "catch-up dm": late} {
		got := c.events()
		if len(got) != len(log) {
			t.Errorf("%s: sent %d events, the log holds %d", who, len(got), len(log))
			continue
		}
		for i := range log {
			if !proto.Equal(got[i], log[i]) {
				t.Errorf("%s: event %d differs from the log:\n got  %v\n want %v", who, i, got[i], log[i])
			}
		}
	}
}
