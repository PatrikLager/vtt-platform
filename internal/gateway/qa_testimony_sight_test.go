package gateway_test

import (
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/sight"
)

const (
	qsParty    = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qsNonParty = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

type qsSightSeat struct {
	name   string
	v      gateway.Viewer
	pr     *gateway.Projector
	fold   *engine.State
	held   map[string]bool
	sees   map[string]bool
	frozen map[string]string
	scenes map[string]bool
	last   []*vttv1.Envelope
}

type qsSightWorld struct {
	tb    testing.TB
	st    *engine.State
	seq   int64
	seats []*qsSightSeat
	whole []*qsSightSeat
	trail []*vttv1.Envelope
	hits  map[string]int
}

func (w *qsSightWorld) seat(name string) *qsSightSeat {
	for _, s := range w.seats {
		if s.name == name {
			return s
		}
	}
	w.tb.Fatalf("no seat %q", name)
	return nil
}

func (w *qsSightWorld) at() string {
	labels := make([]string, len(w.trail))
	for i, env := range w.trail {
		labels[i] = qsSightLabel(env)
	}
	return "after [" + strings.Join(labels, "; ") + "]"
}

func qsSightNewSeat(name string, v gateway.Viewer) *qsSightSeat {
	return &qsSightSeat{
		name: name, v: v, pr: gateway.NewProjector(v), fold: engine.NewState(),
		held: map[string]bool{}, sees: map[string]bool{}, frozen: map[string]string{}, scenes: map[string]bool{},
	}
}

func qsSightNewWorld(tb testing.TB) *qsSightWorld {
	tb.Helper()
	w := &qsSightWorld{tb: tb, st: engine.NewState(), hits: map[string]int{}}
	w.whole = []*qsSightSeat{
		qsSightNewSeat("dm", gateway.Viewer{ParticipantID: "p-dm", Role: identity.RoleDM}),
		qsSightNewSeat("agent", gateway.Viewer{ParticipantID: "p-agent", Role: identity.RoleAgent}),
	}
	w.seats = []*qsSightSeat{
		qsSightNewSeat("p1", gateway.Viewer{ParticipantID: "p1", Role: identity.RolePlayer}),
		qsSightNewSeat("p2", gateway.Viewer{ParticipantID: "p2", Role: identity.RolePlayer}),
		qsSightNewSeat("p3", gateway.Viewer{ParticipantID: "p3", Role: identity.RolePlayer}),
		qsSightNewSeat("spec-hero", gateway.Viewer{ParticipantID: "s1", Role: identity.RoleSpectator, Viewpoint: "hero"}),
		qsSightNewSeat("spec-pregen", gateway.Viewer{ParticipantID: "s2", Role: identity.RoleSpectator, Viewpoint: "pregen"}),
		qsSightNewSeat("spec-orc", gateway.Viewer{ParticipantID: "s3", Role: identity.RoleSpectator, Viewpoint: "orc"}),
		qsSightNewSeat("spec-none", gateway.Viewer{ParticipantID: "s4", Role: identity.RoleSpectator}),
		qsSightNewSeat("spec-late", gateway.Viewer{ParticipantID: "s5", Role: identity.RoleSpectator, Viewpoint: "late"}),
	}
	hall := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 7; x++ {
		for y := int32(0); y < 3; y++ {
			kind := "floor"
			if x == 3 {
				kind = "wall"
				if y == 1 {
					kind = "door"
				}
			}
			hall[qsKey(x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	w.must(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "one"}}})
	w.must(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "hall", Name: "Hall", GridWidth: 7, GridHeight: 3, Tiles: hall}}})
	w.must(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{SceneId: "vault", Name: "Vault", GridWidth: 3, GridHeight: 3}}})
	w.must(qsAdd("hero", qsParty, 10, 10))
	w.must(qsAdd("ally", qsParty, 8, 8))
	w.must(qsAdd("orc", qsNonParty, 7, 7))
	w.must(qsAdd("rat", qsNonParty, 3, 3))
	w.must(qsAdd("pregen", qsParty, 5, 5))
	w.must(qsAdd("imp", qsNonParty, 4, 4))
	w.must(qsGrant("hero", "p1", qsParty))
	w.must(qsGrant("ally", "p2", qsParty))
	w.must(qsPlace("t-hero", "hall", "hero", 0, 1))
	w.must(qsPlace("t-ally", "hall", "ally", 5, 1))
	w.must(qsPlace("t-orc", "hall", "orc", 6, 0))
	w.must(qsPlace("t-rat", "hall", "rat", 1, 0))
	w.must(qsPlace("t-imp", "vault", "imp", 2, 2))
	w.trail = nil
	return w
}

func qsKey(x, y int32) string { return fmt.Sprintf("%d,%d", x, y) }

func qsAdd(id string, kind vttv1.ActorKind, cur, maxHP int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: id, Name: strings.ToUpper(id), ModuleId: "mod", Kind: kind,
		Attributes: map[string]int32{"grit": int32(len(id))},
		Resources:  map[string]*vttv1.Resource{"pool": {Current: cur, Max: maxHP}},
	}}}}
}

func qsGrant(actor, pid string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actor, ParticipantId: pid, Kind: kind}}}
}

func qsRevoke(actor, pid string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: actor, ParticipantId: pid}}}
}

func qsPlace(tok, scene, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{TokenId: tok, SceneId: scene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y}}}}
}

func qsMove(st *engine.State, tok string, x, y int32) *vttv1.Envelope {
	t, ok := st.Tokens[tok]
	if !ok {
		return nil
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{TokenId: tok, SceneId: t.SceneID, From: &vttv1.GridPosition{X: t.X, Y: t.Y}, To: &vttv1.GridPosition{X: x, Y: y}}}}
}

func qsDoor(open bool) *vttv1.Envelope {
	at := &vttv1.GridPosition{X: 3, Y: 1}
	if open {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{SceneId: "hall", At: at}}}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{DoorClosed: &vttv1.DoorClosed{SceneId: "hall", At: at}}}
}

func qsHP(st *engine.State, actor string, delta int32) *vttv1.Envelope {
	r := st.Actors[actor].GetResources()["pool"]
	nv := int64(r.GetCurrent()) + int64(delta)
	if nv < 0 {
		nv = 0
	}
	if r.GetMax() > 0 && nv > int64(r.GetMax()) {
		nv = int64(r.GetMax())
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{ActorId: actor, Resource: "pool", Delta: delta, NewValue: int32(nv), Reason: "blow by " + actor}}}
}

func qsCond(st *engine.State, actor, cond string) *vttv1.Envelope {
	for _, c := range st.Conditions[actor] {
		if c.ID == cond {
			return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{ActorId: actor, ConditionId: cond, Reason: "cured"}}}
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{ActorId: actor, ConditionId: cond, Source: "spell of " + actor}}}
}

func qsControl(st *engine.State, actor, pid string, kind vttv1.ActorKind) *vttv1.Envelope {
	if slices.Contains(st.Actors[actor].GetControllerIds(), pid) {
		return qsRevoke(actor, pid)
	}
	return qsGrant(actor, pid, kind)
}

func qsAttack(attacker, target string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AttackRolled{AttackRolled: &vttv1.AttackRolled{AttackerId: attacker, TargetId: target, Expression: "1d20", Total: 12, Outcome: "hit"}}}
}

func qsAbility(actor string, targets ...string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_AbilityUsed{AbilityUsed: &vttv1.AbilityUsed{ActorId: actor, AbilityId: "bolt", TargetIds: targets, OutcomeSummary: "ouch"}}}
}

func (w *qsSightWorld) must(env *vttv1.Envelope) *vttv1.Envelope {
	w.tb.Helper()
	if env == nil || !w.emit(env) {
		w.tb.Fatalf("the server refused %v %s", env, w.at())
	}
	return env
}

func (w *qsSightWorld) emit(env *vttv1.Envelope) bool {
	w.tb.Helper()
	env.Sequence = w.seq + 1
	env.EventId = fmt.Sprintf("ev-%d", env.Sequence)
	env.ActorRole = "dm"
	env.ParticipantId = "p-dm"
	env.SessionId = "sess-1"
	env.OccurredAt = timestamppb.New(time.Date(2026, 10, 1, 12, 0, int(env.Sequence), 0, time.UTC))
	if err := engine.Apply(w.st, env); err != nil {
		return false
	}
	w.seq++
	w.trail = append(w.trail, env)
	orig := proto.Clone(env)
	for _, s := range w.whole {
		frames := s.pr.Project(env, w.st)
		if len(frames) != 1 || frames[0] != env {
			w.tb.Errorf("VTT-176: %s was sent %v for event %d instead of the event itself %s", s.name, frames, env.Sequence, w.at())
		}
	}
	for _, s := range w.seats {
		frames := s.pr.Project(env, w.st)
		s.last = frames
		w.check(s, env, frames)
	}
	if !proto.Equal(orig, env) {
		w.tb.Errorf("VTT-222: projecting event %d changed it %s", env.Sequence, w.at())
	}
	return true
}

func qsSightLabel(env *vttv1.Envelope) string {
	name := strings.TrimPrefix(fmt.Sprintf("%T", env.GetPayload()), "*vttv1.Envelope_")
	return fmt.Sprintf("%d:%s%v", env.GetSequence(), name, qsSightNamed(env))
}

func qsSightNamed(env *vttv1.Envelope) []string {
	var ids []string
	switch p := env.GetPayload().(type) {
	case *vttv1.Envelope_ActorAdded:
		ids = []string{p.ActorAdded.GetActor().GetActorId()}
	case *vttv1.Envelope_TokenPlaced:
		ids = []string{p.TokenPlaced.GetActorId()}
	case *vttv1.Envelope_ResourceChanged:
		ids = []string{p.ResourceChanged.GetActorId()}
	case *vttv1.Envelope_ConditionApplied:
		ids = []string{p.ConditionApplied.GetActorId()}
	case *vttv1.Envelope_ConditionRemoved:
		ids = []string{p.ConditionRemoved.GetActorId()}
	case *vttv1.Envelope_ActorControlGranted:
		ids = []string{p.ActorControlGranted.GetActorId()}
	case *vttv1.Envelope_ActorControlRevoked:
		ids = []string{p.ActorControlRevoked.GetActorId()}
	case *vttv1.Envelope_ActorRemoved:
		ids = []string{p.ActorRemoved.GetActorId()}
	case *vttv1.Envelope_AttackRolled:
		ids = []string{p.AttackRolled.GetAttackerId(), p.AttackRolled.GetTargetId()}
	case *vttv1.Envelope_AbilityUsed:
		ids = append([]string{p.AbilityUsed.GetActorId()}, p.AbilityUsed.GetTargetIds()...)
	}
	return slices.DeleteFunc(ids, func(s string) bool { return s == "" })
}

type qsSightStatus struct {
	kind  vttv1.ActorKind
	res   map[string]int32
	conds []string
	ctl   []string
	fixed string
}

func qsSightStatusOf(st *engine.State, id string) qsSightStatus {
	a := st.Actors[id]
	s := qsSightStatus{kind: a.GetKind(), res: make(map[string]int32, len(a.GetResources()))}
	var b strings.Builder
	b.WriteString(a.GetName())
	b.WriteString("|")
	b.WriteString(a.GetModuleId())
	for _, n := range slices.Sorted(maps.Keys(a.GetResources())) {
		s.res[n] = a.GetResources()[n].GetCurrent()
		b.WriteString("|" + n + "/" + strconv.Itoa(int(a.GetResources()[n].GetMax())))
	}
	for _, n := range slices.Sorted(maps.Keys(a.GetAttributes())) {
		b.WriteString("|" + n + "=" + strconv.Itoa(int(a.GetAttributes()[n])))
	}
	s.fixed = b.String()
	for _, c := range st.Conditions[id] {
		s.conds = append(s.conds, c.ID)
	}
	slices.Sort(s.conds)
	s.ctl = slices.Sorted(slices.Values(a.GetControllerIds()))
	return s
}

func (s qsSightStatus) String() string {
	res := make([]string, 0, len(s.res))
	for _, n := range slices.Sorted(maps.Keys(s.res)) {
		res = append(res, n+"="+strconv.Itoa(int(s.res[n])))
	}
	return "kind=" + s.kind.String() + " ctl=[" + strings.Join(s.ctl, " ") + "] conds=[" + strings.Join(s.conds, " ") +
		"] res=[" + strings.Join(res, " ") + "] fixed=" + s.fixed
}

func qsSightLook(st *engine.State, v gateway.Viewer) (map[string]bool, map[string]engine.Token, map[string]map[string]bool) {
	eyes := map[string]bool{}
	switch v.Role {
	case identity.RolePlayer:
		for id, a := range st.Actors {
			if slices.Contains(a.GetControllerIds(), v.ParticipantID) {
				eyes[id] = true
			}
		}
	case identity.RoleSpectator:
		if a, ok := st.Actors[v.Viewpoint]; ok && engine.IsPartyMember(a) {
			eyes[v.Viewpoint] = true
		}
	}
	vis := map[string]map[string]bool{}
	for _, tok := range st.Tokens {
		sc, ok := st.Scenes[tok.SceneID]
		if !ok || !eyes[tok.ActorID] {
			continue
		}
		if vis[tok.SceneID] == nil {
			vis[tok.SceneID] = map[string]bool{}
		}
		for k := range sight.VisibleFrom(sc, tok.X, tok.Y, 0, 0) {
			vis[tok.SceneID][k] = true
		}
	}
	sees := maps.Clone(eyes)
	board := map[string]engine.Token{}
	for id, tok := range st.Tokens {
		if vis[tok.SceneID][qsKey(tok.X, tok.Y)] {
			sees[tok.ActorID] = true
			board[id] = tok
		}
	}
	return sees, board, vis
}

func qsSightAll(ids []string, set map[string]bool) bool {
	for _, id := range ids {
		if !set[id] {
			return false
		}
	}
	return true
}

func qsSightIsStatusFrame(f *vttv1.Envelope) bool {
	switch f.GetPayload().(type) {
	case *vttv1.Envelope_ResourceChanged, *vttv1.Envelope_ConditionApplied, *vttv1.Envelope_ConditionRemoved,
		*vttv1.Envelope_ActorControlGranted, *vttv1.Envelope_ActorControlRevoked,
		*vttv1.Envelope_ActorRemoved, *vttv1.Envelope_ActorAdded:
		return true
	}
	return false
}

func (w *qsSightWorld) check(s *qsSightSeat, env *vttv1.Envelope, frames []*vttv1.Envelope) {
	tb := w.tb
	tb.Helper()
	where := func() string { return fmt.Sprintf("%s, event %d %s", s.name, env.GetSequence(), w.at()) }
	sees, board, vis := qsSightLook(w.st, s.v)
	for id := range vis {
		s.scenes[id] = true
	}
	held := map[string]bool{}
	for a := range s.held {
		if w.st.Actors[a] != nil {
			held[a] = true
		}
	}
	for a := range sees {
		held[a] = true
	}
	for id, a := range w.st.Actors {
		if engine.IsPartyMember(a) {
			held[id] = true
		}
	}
	before := map[string]qsSightStatus{}
	for a := range sees {
		if s.held[a] && !s.sees[a] && s.fold.Actors[a] != nil {
			before[a] = qsSightStatusOf(s.fold, a)
		}
	}

	fwd := -1
	for i, f := range frames {
		if f.GetSequence() != env.GetSequence() {
			tb.Errorf("VTT-221: %s: frame %d carries sequence %d", where(), i, f.GetSequence())
		}
		if f == env || (f.GetEventId() != "" && proto.Equal(f, env)) {
			fwd = i
			continue
		}
		if f.GetEventId() != "" || f.GetActorRole() != "" || f.GetParticipantId() != "" || f.GetSessionId() != "" {
			tb.Errorf("VTT-249: %s: the projection's own frame %v carries the event's metadata", where(), f)
		}
		if f.GetResourceChanged().GetReason() != "" || f.GetConditionApplied().GetSource() != "" || f.GetConditionRemoved().GetReason() != "" {
			tb.Errorf("VTT-249: %s: the projection's own frame %v carries a reason or source", where(), f)
		}
		for _, a := range qsSightNamed(f) {
			if !s.sees[a] && !sees[a] && (!held[a] || s.held[a]) {
				tb.Errorf("VTT-243 VTT-245 VTT-247: %s: frame %v names %s, which the viewer neither saw before nor sees nor is introduced to", where(), f, a)
			}
		}
	}
	if fwd >= 0 && fwd != len(frames)-1 {
		tb.Errorf("%s: the forwarded event is frame %d of %d, not the last", where(), fwd, len(frames))
	}
	forwarded := fwd >= 0
	names := qsSightNamed(env)
	switch env.GetPayload().(type) {
	case *vttv1.Envelope_ResourceChanged, *vttv1.Envelope_ConditionApplied, *vttv1.Envelope_ConditionRemoved:
		want := s.sees[names[0]]
		w.hits[fmt.Sprint("change forwarded=", forwarded)]++
		if forwarded != want {
			tb.Errorf("VTT-243: %s: forwarded=%v, saw %s before the event=%v", where(), forwarded, names[0], want)
		}
		if !want && len(frames) != 0 {
			tb.Errorf("VTT-243: %s: a change to unseen %s produced frames %v", where(), names[0], frames)
		}
	case *vttv1.Envelope_AttackRolled, *vttv1.Envelope_AbilityUsed:
		want := qsSightAll(names, s.sees)
		w.hits[fmt.Sprint("deed forwarded=", forwarded)]++
		if forwarded != want {
			tb.Errorf("VTT-244: %s: forwarded=%v, saw every one of %v before the event=%v", where(), forwarded, names, want)
		}
		if !want && len(frames) != 0 {
			tb.Errorf("VTT-244: %s: a deed naming an unseen actor produced frames %v", where(), frames)
		}
	case *vttv1.Envelope_DoorOpened, *vttv1.Envelope_DoorClosed:
		at := env.GetDoorOpened().GetAt()
		if at == nil {
			at = env.GetDoorClosed().GetAt()
		}
		if want := vis["hall"][qsKey(at.GetX(), at.GetY())]; forwarded != want {
			tb.Errorf("VTT-212: %s: forwarded=%v, door square in sight=%v", where(), forwarded, want)
		}
	case *vttv1.Envelope_ActorControlGranted, *vttv1.Envelope_ActorControlRevoked:
		want := s.sees[names[0]]
		w.hits[fmt.Sprint("control forwarded=", forwarded)]++
		if forwarded != want {
			tb.Errorf("VTT-245: %s: forwarded=%v, saw %s before the event=%v", where(), forwarded, names[0], want)
		}
		if !want && !sees[names[0]] && maps.Equal(held, s.held) && len(frames) != 0 {
			tb.Errorf("VTT-245: %s: a control change on unseen %s produced frames %v", where(), names[0], frames)
		}
	}

	for i, f := range frames {
		if err := engine.Apply(s.fold, f); err != nil {
			tb.Fatalf("VTT-224: %s: frame %d %v does not fold: %v", where(), i, f, err)
		}
	}

	if got, want := slices.Sorted(maps.Keys(s.fold.Actors)), slices.Sorted(maps.Keys(held)); !slices.Equal(got, want) {
		tb.Errorf("VTT-208 VTT-241 VTT-242: %s: roster %v, want %v", where(), got, want)
	}
	for a := range held {
		if s.fold.Actors[a] == nil {
			continue
		}
		got := qsSightStatusOf(s.fold, a).String()
		server := qsSightStatusOf(w.st, a).String()
		switch {
		case sees[a]:
			if got != server {
				tb.Errorf("VTT-243 VTT-245 VTT-248: %s: sees %s as\n  %s\nserver has\n  %s", where(), a, got, server)
			}
			s.frozen[a] = server
		case s.held[a] && !s.sees[a]:
			if got != server {
				w.hits["held unseen and stale"]++
			}
			if got != s.frozen[a] {
				tb.Errorf("VTT-247: %s: holds unseen %s as\n  %s\nlast seen as\n  %s", where(), a, got, s.frozen[a])
			}
		default:
			if got != server {
				tb.Errorf("VTT-247: %s: holds %s, seen before the event or introduced at it, as\n  %s\nserver has\n  %s", where(), a, got, server)
			}
			s.frozen[a] = server
		}
		if !s.held[a] && !slices.Equal(s.fold.Actors[a].GetControllerIds(), w.st.Actors[a].GetControllerIds()) {
			tb.Errorf("VTT-209: %s: %s introduced with controllers %v, server order %v", where(), a, s.fold.Actors[a].GetControllerIds(), w.st.Actors[a].GetControllerIds())
		}
	}
	for a, b := range before {
		w.checkCorrection(where, env, a, b, qsSightStatusOf(w.st, a), frames)
	}
	if !maps.Equal(s.fold.Tokens, board) {
		tb.Errorf("VTT-203 VTT-204: %s: board %v, want %v", where(), s.fold.Tokens, board)
	}
	if got, want := slices.Sorted(maps.Keys(s.fold.Scenes)), slices.Sorted(maps.Keys(s.scenes)); !slices.Equal(got, want) {
		tb.Errorf("VTT-196: %s: scenes %v, want %v", where(), got, want)
	}
	for id, sc := range s.fold.Scenes {
		got := maps.Clone(sc.Visible)
		maps.DeleteFunc(got, func(_ string, v bool) bool { return !v })
		if !maps.Equal(got, vis[id]) && len(got)+len(vis[id]) > 0 {
			tb.Errorf("VTT-198 VTT-202: %s: %s visible %v, want %v", where(), id, slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(vis[id])))
		}
		for k := range vis[id] {
			if sc.OpenDoors[k] != w.st.Scenes[id].OpenDoors[k] {
				tb.Errorf("VTT-213: %s: door %s of %s open=%v, server %v", where(), k, id, sc.OpenDoors[k], w.st.Scenes[id].OpenDoors[k])
			}
		}
	}
	s.held, s.sees = held, sees
}

func qsSightMinus(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

func qsSightSameSet(x, y []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(x)), slices.Sorted(slices.Values(y)))
}

func (w *qsSightWorld) checkCorrection(where func() string, env *vttv1.Envelope, a string, b, p qsSightStatus, frames []*vttv1.Envelope) {
	tb := w.tb
	tb.Helper()
	var af []*vttv1.Envelope
	for _, f := range frames {
		if f.GetEventId() == "" && qsSightIsStatusFrame(f) && slices.Contains(qsSightNamed(f), a) {
			af = append(af, f)
		}
	}
	if b.String() == p.String() {
		w.hits["into sight unchanged"]++
		if len(af) != 0 {
			tb.Errorf("VTT-250: %s: %s came into sight unchanged, yet was sent %v", where(), a, af)
		}
		return
	}
	newCtl := qsSightMinus(p.ctl, b.ctl)
	reintro := !slices.Equal(slices.Sorted(maps.Keys(b.res)), slices.Sorted(maps.Keys(p.res))) ||
		(b.kind != p.kind && len(newCtl) == 0 && (len(b.ctl) == 0 || p.kind == vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED))
	if reintro {
		w.hits["into sight introduced again"]++
		if len(af) != 2+len(p.ctl)+len(p.conds) || af[0].GetActorRemoved() == nil || af[1].GetActorAdded() == nil {
			tb.Errorf("VTT-248: %s: %s needed introducing again, got %v", where(), a, af)
		}
		return
	}
	stage := 0
	resFrames := map[string]int{}
	var removed, applied, granted, revoked []string
	restates := 0
	for _, f := range af {
		var at int
		switch {
		case f.GetResourceChanged() != nil:
			at = 1
			resFrames[f.GetResourceChanged().GetResource()]++
		case f.GetConditionRemoved() != nil:
			at = 2
			removed = append(removed, f.GetConditionRemoved().GetConditionId())
		case f.GetConditionApplied() != nil:
			at = 3
			applied = append(applied, f.GetConditionApplied().GetConditionId())
		case f.GetActorControlGranted() != nil:
			at = 4
			g := f.GetActorControlGranted()
			if g.GetKind() != p.kind {
				tb.Errorf("VTT-248: %s: correcting grant %v does not carry the present kind %v", where(), g, p.kind)
			}
			if slices.Contains(b.ctl, g.GetParticipantId()) {
				restates++
			} else {
				granted = append(granted, g.GetParticipantId())
			}
		case f.GetActorControlRevoked() != nil:
			at = 5
			revoked = append(revoked, f.GetActorControlRevoked().GetParticipantId())
		default:
			tb.Errorf("VTT-248: %s: unexpected correction frame for %s: %v", where(), a, f)
		}
		if at < stage {
			tb.Errorf("VTT-248: %s: correction frames for %s out of the recorded order: %v", where(), a, af)
		}
		stage = at
	}
	for n, cur := range p.res {
		want := 0
		if diff := int64(cur) - int64(b.res[n]); diff != 0 {
			want = 1
			if diff != int64(int32(diff)) {
				want = 2
			}
		}
		if resFrames[n] != want {
			tb.Errorf("VTT-248 VTT-250: %s: %s's %s went %d -> %d, sent %d ResourceChanged frames, want %d", where(), a, n, b.res[n], cur, resFrames[n], want)
		}
	}
	if !qsSightSameSet(removed, qsSightMinus(b.conds, p.conds)) || !qsSightSameSet(applied, qsSightMinus(p.conds, b.conds)) {
		tb.Errorf("VTT-248 VTT-250: %s: %s conditions %v -> %v, sent removed %v applied %v", where(), a, b.conds, p.conds, removed, applied)
	}
	if !qsSightSameSet(granted, newCtl) || !qsSightSameSet(revoked, qsSightMinus(b.ctl, p.ctl)) {
		tb.Errorf("VTT-248 VTT-250: %s: %s controllers %v -> %v, sent granted %v revoked %v", where(), a, b.ctl, p.ctl, granted, revoked)
	}
	wantRestate := 0
	if b.kind != p.kind && len(newCtl) == 0 {
		wantRestate = 1
		w.hits["into sight kind restated"]++
	}
	if restates != wantRestate {
		tb.Errorf("VTT-248 VTT-250: %s: %s kind %v -> %v, sent %d re-stating grants, want %d", where(), a, b.kind, p.kind, restates, wantRestate)
	}
}

func (w *qsSightWorld) sent(seat string, keep func(*vttv1.Envelope) bool) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range w.seat(seat).last {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}

func (w *qsSightWorld) wantSilent(seats ...string) {
	w.tb.Helper()
	for _, name := range seats {
		if got := w.seat(name).last; len(got) != 0 {
			w.tb.Errorf("%s was sent %v %s", name, got, w.at())
		}
	}
}

func (w *qsSightWorld) wantEventAlone(seats ...string) {
	w.tb.Helper()
	for _, name := range seats {
		got := w.seat(name).last
		if len(got) != 1 || got[0].GetEventId() != fmt.Sprintf("ev-%d", w.seq) {
			w.tb.Errorf("%s was sent %v, want the event alone %s", name, got, w.at())
		}
	}
}

func (w *qsSightWorld) wantSees(seat string, actors ...string) {
	w.tb.Helper()
	for _, a := range actors {
		if !w.seat(seat).sees[a] {
			w.tb.Fatalf("fixture: %s does not see %s %s", seat, a, w.at())
		}
	}
}

func (w *qsSightWorld) wantUnseen(seat string, actors ...string) {
	w.tb.Helper()
	for _, a := range actors {
		if w.seat(seat).sees[a] {
			w.tb.Fatalf("fixture: %s sees %s %s", seat, a, w.at())
		}
	}
}

func qsIsResource(f *vttv1.Envelope) bool { return f.GetResourceChanged() != nil }

// VTT-243 VTT-245 VTT-247 VTT-248 VTT-249 VTT-221
func TestQATestimonySightAnUnseenPartyMemberIsFrozenThenCorrectedOnSight(t *testing.T) {
	w := qsSightNewWorld(t)
	w.wantUnseen("p1", "ally")
	w.must(qsHP(w.st, "ally", -3))
	w.wantSilent("p1", "p3", "spec-hero", "spec-pregen", "spec-none")
	w.must(qsCond(w.st, "ally", "prone"))
	w.wantSilent("p1", "p3", "spec-hero", "spec-pregen", "spec-none")
	w.must(qsGrant("ally", "p3", qsParty))
	w.wantSilent("p1", "spec-hero", "spec-pregen", "spec-none")
	if got := qsSightStatusOf(w.seat("p1").fold, "ally").String(); !strings.Contains(got, "ctl=[] conds=[] res=[pool=8]") {
		t.Errorf("VTT-247: p1 holds ally as %s", got)
	}
	door := w.must(qsDoor(true))
	w.wantSees("p1", "ally")
	rc := w.sent("p1", qsIsResource)
	ca := w.sent("p1", func(f *vttv1.Envelope) bool { return f.GetConditionApplied() != nil })
	gr := w.sent("p1", func(f *vttv1.Envelope) bool { return f.GetActorControlGranted() != nil })
	if len(rc) != 1 || rc[0].GetResourceChanged().GetActorId() != "ally" || rc[0].GetResourceChanged().GetDelta() != -3 || rc[0].GetResourceChanged().GetNewValue() != 5 {
		t.Errorf("VTT-248: p1's resource correction %v", rc)
	}
	if len(ca) != 1 || ca[0].GetConditionApplied().GetConditionId() != "prone" {
		t.Errorf("VTT-248: p1's condition correction %v", ca)
	}
	if len(gr) != 2 || gr[0].GetActorControlGranted().GetParticipantId() != "p2" || gr[1].GetActorControlGranted().GetParticipantId() != "p3" || gr[1].GetActorControlGranted().GetKind() != qsParty {
		t.Errorf("VTT-248: p1's control correction %v", gr)
	}
	for _, f := range slices.Concat(rc, ca, gr) {
		if f.GetSequence() != door.GetSequence() || f.GetEventId() != "" || f.GetParticipantId() != "" || f.GetActorRole() != "" || f.GetSessionId() != "" {
			t.Errorf("VTT-221 VTT-249: correction %v", f)
		}
		if f.GetOccurredAt() != nil {
			t.Logf("observation: a correction frame carries occurred_at %v", f.GetOccurredAt().AsTime())
		}
	}
}

// VTT-244
func TestQATestimonySightADeedNamingAnUnseenActorIsWithheldWhole(t *testing.T) {
	w := qsSightNewWorld(t)
	w.wantSees("p1", "hero", "rat")
	w.wantUnseen("p1", "orc", "ally")
	w.wantSees("p2", "ally", "orc")
	w.must(qsAttack("orc", "hero"))
	w.wantSilent("p1", "p2", "spec-hero", "spec-orc", "spec-none")
	w.must(qsAttack("hero", "orc"))
	w.wantSilent("p1", "p2", "spec-hero")
	w.must(qsAttack("hero", "rat"))
	w.wantEventAlone("p1", "spec-hero")
	w.wantSilent("p2", "p3", "spec-none")
	w.must(qsAbility("hero", "rat", "orc"))
	w.wantSilent("p1", "p2", "spec-hero")
	w.must(qsAbility("hero", "rat"))
	w.wantEventAlone("p1", "spec-hero")
	w.must(qsAbility("ally"))
	w.wantSilent("p1", "spec-hero")
	w.wantEventAlone("p2")
	w.must(qsAbility("ally", "orc", "hero"))
	w.wantSilent("p1", "p2")
	w.must(qsAttack("orc", "ally"))
	w.wantEventAlone("p2")
	w.wantSilent("p1")
}

// VTT-246 VTT-243 VTT-241 VTT-242
func TestQATestimonySightOwnActorsAndTheShoulderAreHeardWithoutAToken(t *testing.T) {
	w := qsSightNewWorld(t)
	w.must(qsHP(w.st, "pregen", -1))
	w.wantEventAlone("spec-pregen")
	w.wantSilent("p1", "p2", "p3", "spec-hero", "spec-none")
	w.must(qsGrant("pregen", "p3", qsParty))
	w.wantEventAlone("spec-pregen")
	w.must(qsCond(w.st, "pregen", "hidden"))
	w.wantEventAlone("spec-pregen", "p3")
	w.wantSilent("p1", "p2", "spec-none")
	w.must(qsAdd("familiar", qsNonParty, 2, 2))
	w.wantSilent("p1", "p2", "p3", "spec-hero", "spec-pregen", "spec-none")
	w.must(qsGrant("familiar", "p2", qsNonParty))
	w.wantSilent("p1", "p3", "spec-hero", "spec-pregen", "spec-none")
	w.must(qsHP(w.st, "familiar", -1))
	w.wantEventAlone("p2")
	w.wantSilent("p1", "p3", "spec-hero", "spec-pregen", "spec-none")
}

// VTT-250 VTT-247 VTT-248
func TestQATestimonySightAChangeUndoneOutOfSightSendsNothingOnSight(t *testing.T) {
	w := qsSightNewWorld(t)
	w.must(qsMove(w.st, "t-hero", 4, 2))
	w.wantSees("p1", "ally", "orc")
	w.must(qsMove(w.st, "t-hero", 0, 1))
	w.wantUnseen("p1", "ally", "orc")
	for _, a := range []string{"ally", "orc"} {
		w.must(qsHP(w.st, a, -3))
		w.must(qsHP(w.st, a, 3))
		w.must(qsCond(w.st, a, "prone"))
		w.must(qsCond(w.st, a, "prone"))
		w.must(qsGrant(a, "p3", w.st.Actors[a].GetKind()))
		w.must(qsRevoke(a, "p3"))
	}
	w.must(qsRevoke("ally", "p2"))
	w.must(qsGrant("ally", "p2", qsNonParty))
	w.must(qsGrant("ally", "p2", qsParty))
	w.must(qsGrant("orc", "p2", qsParty))
	w.must(qsGrant("orc", "p2", qsNonParty))
	w.must(qsRevoke("orc", "p2"))
	w.must(qsDoor(true))
	w.wantSees("p1", "ally", "orc")
	if got := w.sent("p1", qsSightIsStatusFrame); len(got) != 0 {
		t.Errorf("VTT-250: p1 was sent %v", got)
	}
	if got := w.sent("p1", func(f *vttv1.Envelope) bool { return f.GetTokenPlaced() != nil }); len(got) != 2 {
		t.Errorf("fixture: p1 placed %v", got)
	}
}

// VTT-248 VTT-224 VTT-247
func TestQATestimonySightACorrectionTooWideForOneDeltaStillFolds(t *testing.T) {
	w := qsSightNewWorld(t)
	w.must(qsAdd("ghost", qsNonParty, math.MinInt32, 0))
	w.must(qsPlace("t-ghost", "hall", "ghost", 2, 2))
	w.wantSees("p1", "ghost")
	w.must(qsMove(w.st, "t-ghost", 6, 2))
	w.wantUnseen("p1", "ghost")
	w.must(qsHP(w.st, "ghost", math.MaxInt32))
	w.must(qsHP(w.st, "ghost", math.MaxInt32))
	w.wantSilent("p1")
	if got := w.st.Actors["ghost"].GetResources()["pool"].GetCurrent(); got != math.MaxInt32 {
		t.Fatalf("fixture: the server's ghost has %d", got)
	}
	w.must(qsMove(w.st, "t-ghost", 2, 2))
	rc := w.sent("p1", qsIsResource)
	if len(rc) != 2 || rc[0].GetResourceChanged().GetDelta() != 0 || rc[0].GetResourceChanged().GetNewValue() != 0 || rc[1].GetResourceChanged().GetNewValue() != math.MaxInt32 {
		t.Errorf("VTT-248: p1's correction %v", rc)
	}
	if got := w.seat("p1").fold.Actors["ghost"].GetResources()["pool"].GetCurrent(); got != math.MaxInt32 {
		t.Errorf("VTT-248: p1 holds the ghost at %d", got)
	}
}

// VTT-248 VTT-249 VTT-247 VTT-208 VTT-209
func TestQATestimonySightAnActorWhoseKindChangedUnseenIsIntroducedAgainOnSight(t *testing.T) {
	w := qsSightNewWorld(t)
	w.wantSees("p1", "rat")
	w.must(qsMove(w.st, "t-rat", 5, 2))
	w.wantUnseen("p1", "rat")
	w.must(qsGrant("rat", "p2", qsParty))
	w.wantSilent("p1", "spec-hero")
	for _, seat := range []string{"p3", "spec-none", "spec-orc"} {
		if got := w.seat(seat).fold.Actors["rat"].GetControllerIds(); !slices.Equal(got, []string{"p2"}) {
			t.Errorf("VTT-208 VTT-209: %s holds rat with controllers %v", seat, got)
		}
	}
	if got := w.seat("p1").fold.Actors["rat"].GetKind(); got != qsNonParty {
		t.Errorf("VTT-247: p1 holds rat as %v", got)
	}
	w.must(qsRevoke("rat", "p2"))
	w.wantSilent("p1", "spec-hero", "p3", "spec-none")
	move := w.must(qsMove(w.st, "t-rat", 1, 0))
	frames := w.seat("p1").last
	kinds := make([]string, len(frames))
	for i, f := range frames {
		kinds[i] = strings.TrimPrefix(fmt.Sprintf("%T", f.GetPayload()), "*vttv1.Envelope_")
		if f.GetSequence() != move.GetSequence() || f.GetEventId() != "" || f.GetParticipantId() != "" {
			t.Errorf("VTT-249: %v", f)
		}
	}
	removed, added, placed := slices.Index(kinds, "ActorRemoved"), slices.Index(kinds, "ActorAdded"), slices.Index(kinds, "TokenPlaced")
	if removed < 0 || added < removed || placed < added {
		t.Errorf("VTT-248: p1 was sent %v", kinds)
	}
	if got := w.seat("p1").fold.Actors["rat"]; got.GetKind() != qsParty || len(got.GetControllerIds()) != 0 {
		t.Errorf("VTT-248: p1 holds rat as %v", got)
	}
	if got := w.seat("p3").fold.Actors["rat"].GetControllerIds(); !slices.Equal(got, []string{"p2"}) {
		t.Errorf("VTT-247: p3, which never saw rat, holds it with controllers %v", got)
	}
}

// VTT-246 VTT-245 VTT-247 VTT-248 VTT-249
func TestQATestimonySightAShoulderThatLeavesThePartyStopsBeingEyesUntilItReturns(t *testing.T) {
	w := qsSightNewWorld(t)
	w.wantUnseen("p2", "hero")
	w.must(qsGrant("hero", "p3", qsNonParty))
	if got := w.seat("spec-hero").last; len(got) == 0 || got[len(got)-1].GetEventId() == "" {
		t.Errorf("VTT-245: spec-hero, which saw hero, was sent %v", got)
	}
	w.wantSilent("p2")
	w.wantUnseen("spec-hero", "hero", "rat")
	if got := w.seat("p2").fold.Actors["hero"].GetKind(); got != qsParty {
		t.Errorf("VTT-247: p2 holds hero as %v", got)
	}
	w.must(qsHP(w.st, "hero", -1))
	w.wantSilent("spec-hero", "p2")
	w.wantEventAlone("p1", "p3")
	w.must(qsCond(w.st, "hero", "blessed"))
	w.wantSilent("spec-hero", "p2")
	w.must(qsGrant("hero", "p3", qsParty))
	w.wantSees("spec-hero", "hero", "rat")
	gr := w.sent("spec-hero", func(f *vttv1.Envelope) bool { return f.GetActorControlGranted() != nil })
	if len(gr) != 1 || gr[0].GetEventId() != "" || gr[0].GetActorControlGranted().GetKind() != qsParty {
		t.Errorf("VTT-248 VTT-249: spec-hero's kind correction %v", gr)
	}
	if got := w.sent("spec-hero", qsIsResource); len(got) != 1 || got[0].GetResourceChanged().GetDelta() != -1 {
		t.Errorf("VTT-248: spec-hero's resource correction %v", got)
	}
}

type qsSightStep struct {
	name string
	f    func(st *engine.State) *vttv1.Envelope
}

func qsToggle(st *engine.State, tok string, ax, ay, bx, by int32) *vttv1.Envelope {
	t, ok := st.Tokens[tok]
	if !ok {
		return nil
	}
	if t.X == ax && t.Y == ay {
		return qsMove(st, tok, bx, by)
	}
	return qsMove(st, tok, ax, ay)
}

var qsSightSteps = []qsSightStep{
	{"orc hurt", func(st *engine.State) *vttv1.Envelope { return qsHP(st, "orc", -2) }},
	{"orc healed", func(st *engine.State) *vttv1.Envelope { return qsHP(st, "orc", 2) }},
	{"orc stunned", func(st *engine.State) *vttv1.Envelope { return qsCond(st, "orc", "stunned") }},
	{"ally hurt", func(st *engine.State) *vttv1.Envelope { return qsHP(st, "ally", -3) }},
	{"ally healed", func(st *engine.State) *vttv1.Envelope { return qsHP(st, "ally", 3) }},
	{"ally prone", func(st *engine.State) *vttv1.Envelope { return qsCond(st, "ally", "prone") }},
	{"hero hurt", func(st *engine.State) *vttv1.Envelope { return qsHP(st, "hero", -1) }},
	{"hero blessed", func(st *engine.State) *vttv1.Envelope { return qsCond(st, "hero", "blessed") }},
	{"imp hurt", func(st *engine.State) *vttv1.Envelope { return qsHP(st, "imp", -1) }},
	{"pregen hurt", func(st *engine.State) *vttv1.Envelope { return qsHP(st, "pregen", -1) }},
	{"orc to p3", func(st *engine.State) *vttv1.Envelope { return qsControl(st, "orc", "p3", qsNonParty) }},
	{"ally to p1", func(st *engine.State) *vttv1.Envelope { return qsControl(st, "ally", "p1", qsParty) }},
	{"pregen to p3", func(st *engine.State) *vttv1.Envelope { return qsControl(st, "pregen", "p3", qsParty) }},
	{"rat to p2", func(st *engine.State) *vttv1.Envelope { return qsControl(st, "rat", "p2", qsParty) }},
	{"rat restated", func(st *engine.State) *vttv1.Envelope { return qsGrant("rat", "p2", qsNonParty) }},
	{"hero flips", func(st *engine.State) *vttv1.Envelope {
		if st.Actors["hero"].GetKind() == qsParty {
			return qsGrant("hero", "p3", qsNonParty)
		}
		return qsGrant("hero", "p3", qsParty)
	}},
	{"door", func(st *engine.State) *vttv1.Envelope { return qsDoor(!st.Scenes["hall"].OpenDoors[qsKey(3, 1)]) }},
	{"orc steps", func(st *engine.State) *vttv1.Envelope { return qsToggle(st, "t-orc", 6, 0, 4, 1) }},
	{"hero steps", func(st *engine.State) *vttv1.Envelope { return qsToggle(st, "t-hero", 0, 1, 4, 2) }},
	{"rat steps", func(st *engine.State) *vttv1.Envelope { return qsToggle(st, "t-rat", 1, 0, 5, 2) }},
	{"pregen token", func(st *engine.State) *vttv1.Envelope {
		if _, ok := st.Tokens["t-pregen"]; ok {
			return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "t-pregen"}}}
		}
		return qsPlace("t-pregen", "vault", "pregen", 1, 1)
	}},
	{"orc flips", func(st *engine.State) *vttv1.Envelope {
		if st.Actors["orc"].GetKind() == qsParty {
			return qsGrant("orc", "p1", qsNonParty)
		}
		return qsGrant("orc", "p1", qsParty)
	}},
	{"late arrives", func(*engine.State) *vttv1.Envelope { return qsAdd("late", qsParty, 6, 6) }},
	{"late token", func(st *engine.State) *vttv1.Envelope {
		if _, ok := st.Tokens["t-late"]; ok {
			return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: "t-late"}}}
		}
		return qsPlace("t-late", "hall", "late", 5, 0)
	}},
	{"late hurt", func(st *engine.State) *vttv1.Envelope {
		if st.Actors["late"] == nil {
			return nil
		}
		return qsHP(st, "late", -2)
	}},
	{"orc attacks hero", func(*engine.State) *vttv1.Envelope { return qsAttack("orc", "hero") }},
	{"hero attacks rat", func(*engine.State) *vttv1.Envelope { return qsAttack("hero", "rat") }},
	{"ally bolts", func(*engine.State) *vttv1.Envelope { return qsAbility("ally", "orc", "rat") }},
	{"pregen bolts", func(*engine.State) *vttv1.Envelope { return qsAbility("pregen", "imp") }},
}

func qsSightRun(w *qsSightWorld, step qsSightStep) {
	if env := step.f(w.st); env != nil {
		w.emit(env)
	}
}

var qsSightPaths = []string{
	"change forwarded=true", "change forwarded=false", "deed forwarded=true", "deed forwarded=false",
	"control forwarded=true", "control forwarded=false", "held unseen and stale",
	"into sight unchanged", "into sight kind restated",
}

func qsSightWantPaths(tb testing.TB, hits map[string]int, extra ...string) {
	tb.Helper()
	for _, p := range append(slices.Clone(qsSightPaths), extra...) {
		if hits[p] == 0 {
			tb.Errorf("fixture: no event took the path %q", p)
		}
	}
}

// VTT-176 VTT-196 VTT-198 VTT-203 VTT-204 VTT-208 VTT-209 VTT-212 VTT-221 VTT-224
// VTT-241 VTT-242 VTT-243 VTT-244 VTT-245 VTT-246 VTT-247 VTT-248 VTT-249 VTT-250
func TestQATestimonySightEveryPairOfStepsKeepsEachSeatToWhatItSees(t *testing.T) {
	hits := map[string]int{}
	base := qsSightNewWorld(t)
	for _, a := range qsSightSteps {
		if env := a.f(base.st); env == nil || env.GetAttackRolled() != nil || env.GetAbilityUsed() != nil {
			continue
		}
		for _, b := range qsSightSteps {
			w := qsSightNewWorld(t)
			clear(w.hits)
			qsSightRun(w, a)
			qsSightRun(w, b)
			if t.Failed() {
				t.Fatalf("first failure on %s then %s", a.name, b.name)
			}
			for k, v := range w.hits {
				hits[k] += v
			}
		}
	}
	qsSightWantPaths(t, hits)
}

// VTT-176 VTT-196 VTT-198 VTT-203 VTT-204 VTT-208 VTT-209 VTT-212 VTT-221 VTT-224
// VTT-241 VTT-242 VTT-243 VTT-244 VTT-245 VTT-246 VTT-247 VTT-248 VTT-249 VTT-250
func TestQATestimonySightEveryRandomWalkKeepsEachSeatToWhatItSees(t *testing.T) {
	hits := map[string]int{}
	for seed := uint64(1); seed <= 40; seed++ {
		rng := rand.New(rand.NewPCG(seed, 0x51647))
		w := qsSightNewWorld(t)
		for range 40 {
			qsSightRun(w, qsSightSteps[rng.IntN(len(qsSightSteps))])
		}
		if t.Failed() {
			t.Fatalf("first failure on seed %d", seed)
		}
		for k, v := range w.hits {
			hits[k] += v
		}
	}
	qsSightWantPaths(t, hits, "into sight introduced again")
}
