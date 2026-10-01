package gateway_test

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sort"
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
	rmKeep  = "keep"
	rmYard  = "yard"
	rmPool  = "pool"
	rmParty = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	rmOther = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

var (
	rmWest     = [2]int32{2, 1}
	rmEast     = [2]int32{5, 2}
	rmDoorAt   = [2]int32{3, 1}
	rmPc1West  = [2]int32{1, 1}
	rmPc1East  = [2]int32{4, 0}
	rmPc3East  = [2]int32{5, 0}
	rmYardSpot = [2]int32{0, 0}
)

type rmStatus struct {
	name, res, conds, ctrl string
	kind                   vttv1.ActorKind
}

type rmSeat struct {
	name      string
	v         gateway.Viewer
	pr        *gateway.Projector
	fold      *engine.State
	whole     bool
	held      map[string]rmStatus
	ghost     map[string]bool
	sawBefore map[string]bool
	at        map[int64][]*vttv1.Envelope
}

type rmTable struct {
	tb     testing.TB
	world  *engine.State
	seq    int64
	seats  []*rmSeat
	byName map[string]*rmSeat
}

func rmPlayer(id string) gateway.Viewer {
	return gateway.Viewer{ParticipantID: id, Role: identity.RolePlayer}
}

func rmSpectator(id, shoulder string) gateway.Viewer {
	return gateway.Viewer{ParticipantID: id, Role: identity.RoleSpectator, Viewpoint: shoulder}
}

func rmNewTable(tb testing.TB, viewers ...gateway.Viewer) *rmTable {
	tb.Helper()
	tbl := &rmTable{tb: tb, world: engine.NewState(), byName: map[string]*rmSeat{}}
	all := append([]gateway.Viewer{
		{ParticipantID: "dm1", Role: identity.RoleDM},
		{ParticipantID: "ag1", Role: identity.RoleAgent},
	}, viewers...)
	for _, v := range all {
		s := &rmSeat{
			name: v.ParticipantID, v: v, pr: gateway.NewProjector(v), fold: engine.NewState(),
			whole: v.Role == identity.RoleDM || v.Role == identity.RoleAgent,
			held:  map[string]rmStatus{}, ghost: map[string]bool{}, sawBefore: map[string]bool{},
			at: map[int64][]*vttv1.Envelope{},
		}
		tbl.seats = append(tbl.seats, s)
		tbl.byName[s.name] = s
	}
	return tbl
}

func (tbl *rmTable) seat(name string) *rmSeat {
	s, ok := tbl.byName[name]
	if !ok {
		tbl.tb.Fatalf("no seat %q", name)
	}
	return s
}

func (tbl *rmTable) emit(env *vttv1.Envelope) *vttv1.Envelope {
	tbl.tb.Helper()
	tbl.seq++
	env.Sequence = tbl.seq
	env.EventId = fmt.Sprintf("ev-%d", tbl.seq)
	env.ActorRole = string(identity.RoleDM)
	env.ParticipantId = "dm1"
	env.SessionId = "sess-1"
	env.OccurredAt = timestamppb.New(time.Date(2026, 10, 1, 12, 0, 0, int(tbl.seq), time.UTC))
	if err := engine.Apply(tbl.world, env); err != nil {
		tbl.tb.Fatalf("the world refused seq %d %s: %v", tbl.seq, rmDescribe(nil, env), err)
	}
	snap := tbl.world.Snapshot()
	for _, s := range tbl.seats {
		frames := s.pr.Project(env, snap)
		s.at[env.GetSequence()] = frames
		for i, f := range frames {
			if err := engine.Apply(s.fold, f); err != nil {
				tbl.tb.Errorf("VTT-224: seat %s seq %d frame %d %s does not fold: %v",
					s.name, tbl.seq, i, rmDescribe(env, f), err)
			}
		}
		if s.whole {
			if len(frames) != 1 || !proto.Equal(frames[0], env) {
				tbl.tb.Errorf("VTT-176: seat %s seq %d got %v, want the event unchanged",
					s.name, tbl.seq, rmDescribeAll(env, frames))
			}
			continue
		}
		tbl.checkFrames(s, env, frames)
		tbl.advance(s, env, frames)
	}
	return env
}

func (tbl *rmTable) checkFrames(s *rmSeat, env *vttv1.Envelope, frames []*vttv1.Envelope) {
	tb := tbl.tb
	removed := env.GetActorRemoved().GetActorId()
	forwardedRemovals := 0
	for i, f := range frames {
		fwd := proto.Equal(f, env)
		where := fmt.Sprintf("seat %s seq %d frame %d %s", s.name, env.GetSequence(), i, rmDescribe(env, f))
		if f.GetSequence() != env.GetSequence() {
			tb.Errorf("VTT-221: %s carries sequence %d", where, f.GetSequence())
		}
		if !fwd && (f.GetEventId() != "" || f.GetActorRole() != "" || f.GetParticipantId() != "" ||
			f.GetSessionId() != "" || f.GetOccurredAt() != nil) {
			tb.Errorf("VTT-249: %s is the projection's own and carries metadata", where)
		}
		if !fwd && (f.GetResourceChanged().GetReason() != "" || f.GetConditionApplied().GetSource() != "" ||
			f.GetConditionRemoved().GetReason() != "") {
			tb.Errorf("VTT-249: %s is the projection's own and carries a reason or source", where)
		}
		if f.GetTokenRemoved() != nil {
			tb.Errorf("VTT-205: %s is a TokenRemoved", where)
		}
		ar := f.GetActorRemoved()
		if ar == nil {
			continue
		}
		id := ar.GetActorId()
		if fwd {
			forwardedRemovals++
			if !s.sawBefore[id] {
				tb.Errorf("VTT-251: %s forwarded, but the seat did not see %s before", where, id)
			}
			if _, ok := s.held[id]; !ok {
				tb.Errorf("VTT-215: %s forwarded, but the seat did not hold %s", where, id)
			}
			continue
		}
		if i+1 >= len(frames) || frames[i+1].GetActorAdded().GetActor().GetActorId() != id {
			tb.Errorf("VTT-252: %s is bare and not directly before ActorAdded(%s): %v",
				where, id, rmDescribeAll(env, frames))
		}
	}
	if removed == "" {
		return
	}
	if s.sawBefore[removed] && forwardedRemovals != 1 {
		tb.Errorf("SPEC-016 ruling: seat %s saw %s before seq %d and was sent %v",
			s.name, removed, env.GetSequence(), rmDescribeAll(env, frames))
	}
	if !s.sawBefore[removed] {
		for _, f := range frames {
			if rmNames(f, removed) {
				tb.Errorf("VTT-251: seat %s did not see %s, yet seq %d sent %v",
					s.name, removed, env.GetSequence(), rmDescribeAll(env, frames))
				break
			}
		}
	}
}

func (tbl *rmTable) advance(s *rmSeat, env *vttv1.Envelope, frames []*vttv1.Envelope) {
	tb := tbl.tb
	sees, known, board := rmLook(s.v, tbl.world)
	if id := env.GetActorRemoved().GetActorId(); id != "" {
		if s.sawBefore[id] {
			delete(s.held, id)
			delete(s.ghost, id)
		} else if _, ok := s.held[id]; ok {
			s.ghost[id] = true
		}
	}
	for _, id := range rmKeys(known) {
		_, holds := s.held[id]
		switch {
		case !holds:
			if rmRemovalIndex(frames, id) >= 0 {
				tb.Errorf("VTT-252: seat %s seq %d removes %s it never held: %v",
					s.name, env.GetSequence(), id, rmDescribeAll(env, frames))
			}
			s.held[id] = rmStatusOf(tbl.world, id)
		case s.ghost[id]:
			i := rmRemovalIndex(frames, id)
			if i < 0 || frames[i].GetEventId() != "" || i+1 >= len(frames) ||
				frames[i+1].GetActorAdded().GetActor().GetActorId() != id {
				tb.Errorf("VTT-211 VTT-252: seat %s seq %d meets %s again without a bare removal first: %v",
					s.name, env.GetSequence(), id, rmDescribeAll(env, frames))
			}
			delete(s.ghost, id)
			s.held[id] = rmStatusOf(tbl.world, id)
		}
	}
	for _, id := range rmKeys(s.held) {
		if !s.ghost[id] && (s.sawBefore[id] || sees[id]) {
			s.held[id] = rmStatusOf(tbl.world, id)
		}
	}
	if got, want := rmKeys(s.fold.Actors), rmKeys(s.held); !slices.Equal(got, want) {
		tb.Errorf("VTT-247 VTT-251 VTT-211: seat %s after seq %d holds %v, want %v",
			s.name, env.GetSequence(), got, want)
	}
	for _, id := range rmKeys(s.held) {
		if s.fold.Actors[id] == nil {
			continue
		}
		if got, want := rmStatusOf(s.fold, id), s.held[id]; got != want {
			req := "VTT-247"
			if sees[id] {
				req = "VTT-248"
			}
			tb.Errorf("%s: seat %s after seq %d holds %s as %+v, want %+v",
				req, s.name, env.GetSequence(), id, got, want)
		}
	}
	if got, want := rmBoard(s.fold.Tokens), rmBoard(board); !slices.Equal(got, want) {
		tb.Errorf("VTT-203 VTT-204 VTT-205: seat %s after seq %d has board %v, want %v",
			s.name, env.GetSequence(), got, want)
	}
	s.sawBefore = sees
}

func rmLook(v gateway.Viewer, st *engine.State) (sees, known map[string]bool, board map[string]engine.Token) {
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
	visible := map[string]map[string]bool{}
	for _, tok := range st.Tokens {
		sc, ok := st.Scenes[tok.SceneID]
		if !eyes[tok.ActorID] || !ok {
			continue
		}
		if visible[tok.SceneID] == nil {
			visible[tok.SceneID] = map[string]bool{}
		}
		for k := range sight.VisibleFrom(sc, tok.X, tok.Y, 0, 0) {
			visible[tok.SceneID][k] = true
		}
	}
	sees, known, board = map[string]bool{}, map[string]bool{}, map[string]engine.Token{}
	for id := range eyes {
		sees[id] = true
	}
	for id, tok := range st.Tokens {
		if visible[tok.SceneID][fmt.Sprintf("%d,%d", tok.X, tok.Y)] {
			board[id] = tok
			sees[tok.ActorID] = true
		}
	}
	for id, a := range st.Actors {
		if sees[id] || engine.IsPartyMember(a) {
			known[id] = true
		}
	}
	return sees, known, board
}

func rmStatusOf(st *engine.State, id string) rmStatus {
	a := st.Actors[id]
	var res []string
	for n, r := range a.GetResources() {
		res = append(res, fmt.Sprintf("%s=%d/%d", n, r.GetCurrent(), r.GetMax()))
	}
	conds := map[string]bool{}
	for _, c := range st.Conditions[id] {
		conds[c.ID] = true
	}
	ctrl := map[string]bool{}
	for _, c := range a.GetControllerIds() {
		ctrl[c] = true
	}
	sort.Strings(res)
	return rmStatus{
		name: a.GetName(), kind: a.GetKind(), res: strings.Join(res, ","),
		conds: strings.Join(rmKeys(conds), ","), ctrl: strings.Join(rmKeys(ctrl), ","),
	}
}

func rmBoard(tokens map[string]engine.Token) []string {
	var out []string
	for id, t := range tokens {
		out = append(out, fmt.Sprintf("%s:%s:%s:%d,%d", id, t.SceneID, t.ActorID, t.X, t.Y))
	}
	sort.Strings(out)
	return out
}

func rmKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func rmRemovalIndex(frames []*vttv1.Envelope, id string) int {
	for i, f := range frames {
		if f.GetActorRemoved() != nil && f.GetActorRemoved().GetActorId() == id {
			return i
		}
	}
	return -1
}

func rmNames(f *vttv1.Envelope, id string) bool {
	names := []string{
		f.GetActorAdded().GetActor().GetActorId(), f.GetActorRemoved().GetActorId(),
		f.GetTokenPlaced().GetActorId(), f.GetActorControlGranted().GetActorId(),
		f.GetActorControlRevoked().GetActorId(), f.GetResourceChanged().GetActorId(),
		f.GetConditionApplied().GetActorId(), f.GetConditionRemoved().GetActorId(),
		f.GetAttackRolled().GetAttackerId(), f.GetAttackRolled().GetTargetId(),
		f.GetAbilityUsed().GetActorId(),
	}
	names = append(names, f.GetAbilityUsed().GetTargetIds()...)
	return slices.Contains(names, id)
}

func rmDescribe(env, f *vttv1.Envelope) string {
	mark := ""
	if env != nil && proto.Equal(f, env) {
		mark = "event:"
	}
	switch {
	case f.GetActorRemoved() != nil:
		return mark + "ActorRemoved(" + f.GetActorRemoved().GetActorId() + ")"
	case f.GetActorAdded() != nil:
		return mark + "ActorAdded(" + f.GetActorAdded().GetActor().GetActorId() + ")"
	case f.GetTokenHidden() != nil:
		return mark + "TokenHidden(" + f.GetTokenHidden().GetTokenId() + ")"
	case f.GetTokenPlaced() != nil:
		return mark + "TokenPlaced(" + f.GetTokenPlaced().GetTokenId() + ")"
	case f.GetTokenRemoved() != nil:
		return mark + "TokenRemoved(" + f.GetTokenRemoved().GetTokenId() + ")"
	case f.GetActorControlGranted() != nil:
		return mark + "ActorControlGranted(" + f.GetActorControlGranted().GetActorId() + ")"
	case f.GetActorControlRevoked() != nil:
		return mark + "ActorControlRevoked(" + f.GetActorControlRevoked().GetActorId() + ")"
	case f.GetResourceChanged() != nil:
		return mark + "ResourceChanged(" + f.GetResourceChanged().GetActorId() + ")"
	case f.GetConditionApplied() != nil:
		return mark + "ConditionApplied(" + f.GetConditionApplied().GetActorId() + ")"
	case f.GetConditionRemoved() != nil:
		return mark + "ConditionRemoved(" + f.GetConditionRemoved().GetActorId() + ")"
	case f.GetSceneSeen() != nil:
		return mark + "SceneSeen(" + f.GetSceneSeen().GetSceneId() + ")"
	}
	return mark + strings.TrimPrefix(fmt.Sprintf("%T", f.GetPayload()), "*vttv1.Envelope_")
}

func rmDescribeAll(env *vttv1.Envelope, frames []*vttv1.Envelope) []string {
	out := make([]string, 0, len(frames))
	for _, f := range frames {
		out = append(out, rmDescribe(env, f))
	}
	return out
}

func rmScene(id string, w, h int32, walled bool) *vttv1.Envelope {
	sc := &vttv1.SceneCreated{SceneId: id, Name: id, GridWidth: w, GridHeight: h}
	if walled {
		sc.Tiles = map[string]*vttv1.TileRef{}
		for x := int32(0); x < w; x++ {
			for y := int32(0); y < h; y++ {
				kind := "floor"
				if x == rmDoorAt[0] {
					kind = "wall"
					if y == rmDoorAt[1] {
						kind = "door"
					}
				}
				sc.Tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
			}
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: sc}}
}

func rmAdd(id, name string, kind vttv1.ActorKind, pool int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: id, Name: name, Kind: kind,
		Resources: map[string]*vttv1.Resource{rmPool: {Current: pool, Max: pool}},
	}}}}
}

func rmGrant(id, who string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{ActorId: id, ParticipantId: who, Kind: kind}}}
}

func rmRevoke(id, who string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{
		ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: id, ParticipantId: who}}}
}

func rmPlace(tok, scene, id string, at [2]int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: scene, ActorId: id, Position: &vttv1.GridPosition{X: at[0], Y: at[1]}}}}
}

func rmMove(tok, scene string, from, to [2]int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tok, SceneId: scene,
		From: &vttv1.GridPosition{X: from[0], Y: from[1]}, To: &vttv1.GridPosition{X: to[0], Y: to[1]}}}}
}

func rmTokenGone(tok string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tok}}}
}

func rmActorGone(id string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: id}}}
}

func rmDoor(open bool) *vttv1.Envelope {
	at := &vttv1.GridPosition{X: rmDoorAt[0], Y: rmDoorAt[1]}
	if open {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{SceneId: rmKeep, At: at}}}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{DoorClosed: &vttv1.DoorClosed{SceneId: rmKeep, At: at}}}
}

func rmCondition(id, cond string, on bool) *vttv1.Envelope {
	if on {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{
			ConditionApplied: &vttv1.ConditionApplied{ActorId: id, ConditionId: cond, Source: "src"}}}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{
		ConditionRemoved: &vttv1.ConditionRemoved{ActorId: id, ConditionId: cond, Reason: "why"}}}
}

func rmNarration(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

func rmSecretNote(key string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
		Key: key, Title: key, Text: key, Visibility: vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET}}}
}

func (tbl *rmTable) change(id string, delta int32) *vttv1.Envelope {
	next := max(tbl.world.Actors[id].GetResources()[rmPool].GetCurrent()+delta, 0)
	return tbl.emit(&vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: id, Resource: rmPool, Delta: delta, NewValue: next, Reason: "why"}}})
}

func (tbl *rmTable) removeActor(id string) (tokens []*vttv1.Envelope, actor *vttv1.Envelope) {
	var ids []string
	for tid, t := range tbl.world.Tokens {
		if t.ActorID == id {
			ids = append(ids, tid)
		}
	}
	sort.Strings(ids)
	for _, tid := range ids {
		tokens = append(tokens, tbl.emit(rmTokenGone(tid)))
	}
	return tokens, tbl.emit(rmActorGone(id))
}

func (tbl *rmTable) addPartyMember(id, who string, at [2]int32, scene string) {
	tbl.emit(rmAdd(id, id, rmParty, 10))
	tbl.emit(rmGrant(id, who, rmParty))
	tbl.emit(rmPlace("t-"+id, scene, id, at))
}

func rmStandard(tb testing.TB) *rmTable {
	tb.Helper()
	tbl := rmNewTable(tb,
		rmPlayer("p1"), rmPlayer("p2"), rmPlayer("p3"),
		rmSpectator("sp1", "pc1"), rmSpectator("sp2", "pc2"), rmSpectator("sp3", "pc3"))
	tbl.emit(&vttv1.Envelope{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}}})
	tbl.emit(rmScene(rmKeep, 7, 3, true))
	tbl.emit(rmScene(rmYard, 3, 3, false))
	tbl.addPartyMember("pc1", "p1", rmPc1West, rmKeep)
	tbl.addPartyMember("pc2", "p2", [2]int32{1, 1}, rmYard)
	tbl.addPartyMember("pc3", "p3", rmPc3East, rmKeep)
	return tbl
}

func (tbl *rmTable) frames(seat string, env *vttv1.Envelope) []string {
	return rmDescribeAll(env, tbl.seat(seat).at[env.GetSequence()])
}

func (tbl *rmTable) want(req, seat string, env *vttv1.Envelope, want ...string) {
	tbl.tb.Helper()
	if got := tbl.frames(seat, env); !slices.Equal(got, want) {
		tbl.tb.Errorf("%s: seat %s at seq %d %s got %v, want %v",
			req, seat, env.GetSequence(), rmDescribe(nil, env), got, want)
	}
}

func (tbl *rmTable) wantSees(req, seat, id string, sees bool) {
	tbl.tb.Helper()
	if tbl.seat(seat).sawBefore[id] != sees {
		tbl.tb.Fatalf("%s: setup: seat %s sees %s = %v, want %v", req, seat, id, !sees, sees)
	}
}

func (tbl *rmTable) wantHeld(req, seat, id string, want rmStatus) {
	tbl.tb.Helper()
	a := tbl.seat(seat).fold.Actors[id]
	if a == nil {
		tbl.tb.Errorf("%s: seat %s does not hold %s", req, seat, id)
		return
	}
	if got := rmStatusOf(tbl.seat(seat).fold, id); got != want {
		tbl.tb.Errorf("%s: seat %s holds %s as %+v, want %+v", req, seat, id, got, want)
	}
}

func (tbl *rmTable) wantBareBefore(req, seat string, env *vttv1.Envelope, id string) {
	tbl.tb.Helper()
	frames := tbl.seat(seat).at[env.GetSequence()]
	i := rmRemovalIndex(frames, id)
	if i < 0 || i+1 >= len(frames) || frames[i+1].GetActorAdded().GetActor().GetActorId() != id {
		tbl.tb.Errorf("%s: seat %s at seq %d got %v, want ActorRemoved(%s) directly before ActorAdded(%s)",
			req, seat, env.GetSequence(), rmDescribeAll(env, frames), id, id)
		return
	}
	bare := frames[i]
	want := &vttv1.Envelope{Sequence: env.GetSequence(),
		Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: id}}}
	if !proto.Equal(bare, want) {
		tbl.tb.Errorf("%s: seat %s at seq %d sent the removal of %s as %v, want only its id and seq %d",
			req, seat, env.GetSequence(), id, bare, env.GetSequence())
	}
}

func rmMob(name string, pool int32, conds string, kind vttv1.ActorKind, ctrl string) rmStatus {
	return rmStatus{name: name, res: fmt.Sprintf("%s=%d/%d", rmPool, pool, rmMax(name)), conds: conds, ctrl: ctrl, kind: kind}
}

func rmMax(name string) int32 {
	switch name {
	case "mob-2":
		return 4
	case "mob-3":
		return 6
	}
	return 10
}

// VTT-251 VTT-205 VTT-247 VTT-215 VTT-224 VTT-176 VTT-203 VTT-204
func TestQATestimonyRemovalBatchReachesNoSeatThatWatchedOnlyTheToken(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmDoor(true))
	tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
	tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
	tbl.change("mob", -3)
	tbl.emit(rmCondition("mob", "marked", true))
	for _, s := range []string{"p1", "sp1", "p3", "sp3"} {
		tbl.wantSees("VTT-251", s, "mob", true)
	}
	tbl.wantSees("VTT-251", "p2", "mob", false)
	toks, gone := tbl.removeActor("mob")
	for _, s := range []string{"p1", "sp1", "p3", "sp3"} {
		tbl.want("VTT-205", s, toks[0], "TokenHidden(t-mob)")
		tbl.wantHeld("VTT-251 VTT-247", s, "mob", rmMob("mob-1", 7, "marked", rmOther, ""))
	}
	for _, s := range []string{"p2", "sp2"} {
		tbl.want("VTT-205", s, toks[0])
	}
	for _, s := range []string{"p1", "p2", "p3", "sp1", "sp2", "sp3"} {
		tbl.want("VTT-251", s, gone)
	}
	tbl.want("VTT-176", "dm1", gone, "event:ActorRemoved(mob)")
	tbl.want("VTT-176", "ag1", gone, "event:ActorRemoved(mob)")
}

// VTT-251 VTT-246 VTT-215 VTT-247 VTT-205 VTT-224 VTT-176 VTT-211 VTT-252 VTT-248
func TestQATestimonyRemovalBatchOfAControlledActorReachesItsOwnEyes(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmDoor(true))
	tbl.change("pc1", -2)
	tbl.wantSees("VTT-251", "p3", "pc1", true)
	tbl.wantSees("VTT-251", "p2", "pc1", false)
	toks, gone := tbl.removeActor("pc1")
	tbl.want("VTT-205", "p3", toks[0], "TokenHidden(t-pc1)")
	for _, s := range []string{"p1", "sp1"} {
		tbl.want("VTT-251", s, gone, "event:ActorRemoved(pc1)")
		if tbl.seat(s).fold.Actors["pc1"] != nil {
			t.Errorf("VTT-251: seat %s still holds pc1 after its forwarded removal", s)
		}
	}
	for _, s := range []string{"p2", "p3", "sp2", "sp3"} {
		tbl.want("VTT-251", s, gone)
	}
	tbl.wantHeld("VTT-251 VTT-247", "p3", "pc1", rmStatus{name: "pc1", res: "pool=8/10", ctrl: "p1", kind: rmParty})
	tbl.wantHeld("VTT-251 VTT-247", "p2", "pc1", rmStatus{name: "pc1", res: "pool=10/10", kind: rmParty})
	back := tbl.emit(rmAdd("pc1", "pc1-again", rmParty, 5))
	for _, s := range []string{"p2", "p3", "sp2", "sp3"} {
		tbl.wantBareBefore("VTT-211 VTT-252", s, back, "pc1")
	}
	for _, s := range []string{"p1", "sp1"} {
		if got := tbl.frames(s, back); len(got) == 0 || got[0] != "ActorAdded(pc1)" {
			t.Errorf("VTT-211 VTT-252: seat %s at the return of pc1 got %v, want a plain introduction", s, got)
		}
	}
	for _, s := range []string{"p1", "p2", "p3", "sp1", "sp2", "sp3"} {
		tbl.wantHeld("VTT-211 VTT-248", s, "pc1", rmStatus{name: "pc1-again", res: "pool=5/5", kind: rmParty})
	}
}

// VTT-251 VTT-246 VTT-215 VTT-247 VTT-224 VTT-176 VTT-243
func TestQATestimonyRemovalOfATokenlessOwnActorReachesOnlyItsEyes(t *testing.T) {
	tbl := rmNewTable(t, rmPlayer("p1"), rmPlayer("p2"), rmSpectator("sp4", "pc4"), rmSpectator("sp1", "pc1"))
	tbl.emit(rmScene(rmKeep, 7, 3, true))
	tbl.addPartyMember("pc1", "p1", rmPc1West, rmKeep)
	tbl.emit(rmAdd("pc4", "pc4", rmParty, 10))
	tbl.emit(rmGrant("pc4", "p1", rmParty))
	tbl.emit(rmCondition("pc4", "marked", true))
	tbl.wantSees("VTT-246", "p1", "pc4", true)
	tbl.wantSees("VTT-246", "sp4", "pc4", true)
	gone := tbl.emit(rmActorGone("pc4"))
	tbl.want("VTT-251", "p1", gone, "event:ActorRemoved(pc4)")
	tbl.want("VTT-251", "sp4", gone, "event:ActorRemoved(pc4)")
	for _, s := range []string{"p2", "sp1"} {
		tbl.want("VTT-251", s, gone)
		tbl.wantHeld("VTT-251 VTT-247", s, "pc4", rmStatus{name: "pc4", res: "pool=10/10", kind: rmParty})
	}
}

// VTT-251 VTT-205 VTT-204 VTT-247 VTT-224 VTT-176 VTT-215 VTT-243
func TestQATestimonyRemovalAfterTheTokenLeftByOtherMeansIsNotTold(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
	tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
	tbl.emit(rmAdd("imp", "imp", rmOther, 10))
	tbl.emit(rmDoor(true))
	tbl.emit(rmPlace("t-imp", rmKeep, "imp", rmEast))
	tbl.wantSees("VTT-251", "p1", "mob", true)
	tbl.wantSees("VTT-251", "p1", "imp", true)
	tokGone := tbl.emit(rmTokenGone("t-mob"))
	tbl.want("VTT-205", "p1", tokGone, "TokenHidden(t-mob)")
	tbl.change("mob", -4)
	tbl.emit(rmNarration("a pause"))
	gone := tbl.emit(rmActorGone("mob"))
	shut := tbl.emit(rmDoor(false))
	tbl.want("VTT-204", "p1", shut, "TokenHidden(t-imp)", "TokenHidden(t-pc3)", "SceneSeen(keep)", "event:DoorClosed")
	toks, impGone := tbl.removeActor("imp")
	for _, s := range []string{"p1", "p2", "p3", "sp1", "sp2", "sp3"} {
		tbl.want("VTT-251", s, gone)
		tbl.want("VTT-251", s, impGone)
	}
	tbl.want("VTT-205", "p1", toks[0])
	tbl.want("VTT-205", "p3", toks[0], "TokenHidden(t-imp)")
	tbl.wantHeld("VTT-251 VTT-247", "p1", "mob", rmMob("mob-1", 10, "", rmOther, ""))
	tbl.wantHeld("VTT-251 VTT-247", "p1", "imp", rmStatus{name: "imp", res: "pool=10/10", kind: rmOther})
}

// VTT-251 VTT-215 VTT-247 VTT-205 VTT-224 VTT-176 VTT-246
func TestQATestimonyRemovalFarAwayOrInAnotherSceneIsNotReported(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmAdd("imp", "imp", rmOther, 10))
	tbl.emit(rmPlace("t-imp", rmYard, "imp", rmYardSpot))
	tbl.change("pc2", -1)
	impToks, impGone := tbl.removeActor("imp")
	tbl.want("VTT-205", "p2", impToks[0], "TokenHidden(t-imp)")
	tbl.want("VTT-205", "sp2", impToks[0], "TokenHidden(t-imp)")
	toks, gone := tbl.removeActor("pc2")
	for _, s := range []string{"p1", "p3", "sp1", "sp3"} {
		tbl.want("VTT-251", s, impToks[0])
		tbl.want("VTT-251", s, impGone)
		tbl.want("VTT-251", s, toks[0])
		tbl.want("VTT-251", s, gone)
		tbl.wantHeld("VTT-251 VTT-247", s, "pc2", rmStatus{name: "pc2", res: "pool=10/10", kind: rmParty})
		if tbl.seat(s).fold.Actors["imp"] != nil {
			t.Errorf("VTT-215: seat %s holds imp, which it never saw", s)
		}
	}
	tbl.want("VTT-251", "p2", impGone)
	tbl.want("VTT-251", "p2", gone, "event:ActorRemoved(pc2)")
	tbl.want("VTT-251", "sp2", gone, "event:ActorRemoved(pc2)")
}

// VTT-211 VTT-252 VTT-251 VTT-247 VTT-248 VTT-221 VTT-249 VTT-224 VTT-176 VTT-215
func TestQATestimonyRemovalThenReuseWaitsUnseenThenComesAfterABareRemoval(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
	tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
	tbl.change("mob", -3)
	tbl.emit(rmCondition("mob", "marked", true))
	tbl.removeActor("mob")
	add := tbl.emit(rmAdd("mob", "mob-2", rmOther, 4))
	place := tbl.emit(rmPlace("t-mob2", rmKeep, "mob", rmEast))
	cond := tbl.emit(rmCondition("mob", "other", true))
	for _, s := range []string{"p1", "sp1", "p2", "sp2"} {
		tbl.want("VTT-252", s, add)
		tbl.want("VTT-252", s, place)
		tbl.want("VTT-243", s, cond)
	}
	tbl.wantHeld("VTT-247", "p1", "mob", rmMob("mob-1", 7, "marked", rmOther, ""))
	for _, s := range []string{"p3", "sp3"} {
		if got := tbl.frames(s, place); len(got) == 0 || got[0] != "ActorAdded(mob)" {
			t.Errorf("VTT-252: seat %s never held mob and got %v, want a plain introduction", s, got)
		}
	}
	open := tbl.emit(rmDoor(true))
	for _, s := range []string{"p1", "sp1"} {
		tbl.wantBareBefore("VTT-211 VTT-252 VTT-221 VTT-249", s, open, "mob")
		tbl.wantHeld("VTT-211 VTT-248", s, "mob", rmMob("mob-2", 4, "other", rmOther, ""))
	}
	tbl.want("VTT-251", "p2", open)
}

// VTT-211 VTT-252 VTT-251 VTT-247 VTT-248 VTT-224 VTT-176 VTT-215
func TestQATestimonyRemovalThenReuseAsAPartyMemberIsIntroducedAtOnce(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
	tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
	tbl.removeActor("mob")
	back := tbl.emit(rmAdd("mob", "mob-3", rmParty, 6))
	for _, s := range []string{"p1", "sp1"} {
		tbl.wantBareBefore("VTT-211 VTT-252", s, back, "mob")
	}
	for _, s := range []string{"p2", "p3", "sp2", "sp3"} {
		if got := tbl.frames(s, back); len(got) == 0 || got[0] != "ActorAdded(mob)" {
			t.Errorf("VTT-252: seat %s never held mob and got %v, want a plain introduction", s, got)
		}
	}
	for _, s := range []string{"p1", "p2", "p3", "sp1", "sp2", "sp3"} {
		tbl.wantHeld("VTT-211", s, "mob", rmMob("mob-3", 6, "", rmParty, ""))
	}
}

// VTT-211 VTT-252 VTT-251 VTT-247 VTT-248 VTT-224 VTT-176 VTT-215
func TestQATestimonyRemovalThenReuseTwiceIsIntroducedAfreshEachTime(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
	tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
	tbl.removeActor("mob")
	tbl.emit(rmAdd("mob", "mob-2", rmOther, 4))
	second := tbl.emit(rmPlace("t-mob2", rmKeep, "mob", rmWest))
	tbl.wantBareBefore("VTT-211 VTT-252", "p1", second, "mob")
	tbl.change("mob", -1)
	tbl.removeActor("mob")
	tbl.emit(rmAdd("mob", "mob-x", rmOther, 9))
	gone := tbl.emit(rmActorGone("mob"))
	tbl.want("VTT-251", "p1", gone)
	tbl.wantHeld("VTT-247", "p1", "mob", rmMob("mob-2", 3, "", rmOther, ""))
	tbl.emit(rmAdd("mob", "mob-3", rmOther, 6))
	fourth := tbl.emit(rmPlace("t-mob4", rmKeep, "mob", rmWest))
	tbl.wantBareBefore("VTT-211 VTT-252", "p1", fourth, "mob")
	if n := strings.Count(strings.Join(tbl.frames("p1", fourth), " "), "ActorRemoved(mob)"); n != 1 {
		t.Errorf("VTT-252: seat p1 got %d removals of mob at its fourth arrival, want 1", n)
	}
	tbl.wantHeld("VTT-211 VTT-248", "p1", "mob", rmMob("mob-3", 6, "", rmOther, ""))
}

// VTT-251 VTT-252 VTT-247 VTT-224 VTT-176 VTT-215 VTT-205
func TestQATestimonyRemovalWhoseIdNeverReturnsIsNeverNamedAgain(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
	tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
	tbl.emit(rmCondition("mob", "marked", true))
	_, gone := tbl.removeActor("mob")
	after := gone.GetSequence()
	tbl.emit(rmDoor(true))
	tbl.emit(rmMove("t-pc1", rmKeep, rmPc1West, rmPc1East))
	tbl.change("pc3", -2)
	tbl.emit(rmNarration("later"))
	tbl.emit(rmSecretNote("n1"))
	tbl.emit(rmMove("t-pc1", rmKeep, rmPc1East, rmPc1West))
	tbl.emit(rmDoor(false))
	for _, s := range []string{"p1", "sp1", "p3", "sp3", "p2", "sp2"} {
		for seq := after; seq <= tbl.seq; seq++ {
			for _, f := range tbl.seat(s).at[seq] {
				if rmNames(f, "mob") {
					t.Errorf("VTT-251 VTT-252: seat %s at seq %d was sent %s", s, seq, rmDescribe(nil, f))
				}
			}
		}
	}
	tbl.wantHeld("VTT-247", "p1", "mob", rmMob("mob-1", 10, "marked", rmOther, ""))
}

// VTT-251 VTT-243 VTT-247 VTT-224 VTT-205 VTT-215 VTT-221
func TestQATestimonyRemovalUnseenCannotBeToldFromNoRemoval(t *testing.T) {
	run := func(tb testing.TB, remove bool) *rmTable {
		tb.Helper()
		tbl := rmStandard(tb)
		tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
		tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
		tbl.emit(rmTokenGone("t-mob"))
		tbl.emit(rmTokenGone("t-pc2"))
		if remove {
			tbl.emit(rmActorGone("mob"))
			tbl.emit(rmActorGone("pc2"))
		} else {
			tbl.change("mob", -1)
			tbl.change("pc2", -1)
		}
		tbl.emit(rmDoor(true))
		tbl.emit(rmMove("t-pc1", rmKeep, rmPc1West, rmPc1East))
		tbl.emit(rmNarration("later"))
		tbl.emit(rmSecretNote("n1"))
		tbl.emit(rmMove("t-pc3", rmKeep, rmPc3East, [2]int32{6, 2}))
		tbl.emit(rmDoor(false))
		return tbl
	}
	a, b := run(t, true), run(t, false)
	for _, s := range []string{"p1", "p3", "sp1", "sp3"} {
		for seq := int64(1); seq <= a.seq; seq++ {
			fa, fb := a.seat(s).at[seq], b.seat(s).at[seq]
			same := len(fa) == len(fb)
			for i := 0; same && i < len(fa); i++ {
				same = proto.Equal(fa[i], fb[i])
			}
			if !same {
				t.Errorf("VTT-251: seat %s at seq %d tells a removal from none: %v vs %v",
					s, seq, rmDescribeAll(nil, fa), rmDescribeAll(nil, fb))
			}
		}
	}
}

// VTT-252 VTT-221 VTT-249 VTT-211 VTT-251 VTT-224 VTT-247 VTT-248
func TestQATestimonyRemovalTimeIsNotToldWhenTheIdReturns(t *testing.T) {
	run := func(tb testing.TB, early bool) (*rmTable, *vttv1.Envelope) {
		tb.Helper()
		tbl := rmStandard(tb)
		tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
		tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmWest))
		tbl.emit(rmTokenGone("t-mob"))
		if early {
			tbl.emit(rmActorGone("mob"))
		}
		tbl.emit(rmSecretNote("one"))
		tbl.emit(rmSecretNote("two"))
		if !early {
			tbl.emit(rmActorGone("mob"))
		}
		tbl.emit(rmAdd("mob", "mob-2", rmOther, 4))
		return tbl, tbl.emit(rmPlace("t-mob2", rmKeep, "mob", rmWest))
	}
	a, placed := run(t, true)
	b, _ := run(t, false)
	a.wantBareBefore("VTT-252 VTT-221 VTT-249", "p1", placed, "mob")
	for _, s := range []string{"p1", "sp1"} {
		for seq := int64(1); seq <= a.seq; seq++ {
			fa, fb := a.seat(s).at[seq], b.seat(s).at[seq]
			same := len(fa) == len(fb)
			for i := 0; same && i < len(fa); i++ {
				same = proto.Equal(fa[i], fb[i])
			}
			if !same {
				t.Errorf("VTT-252: seat %s at seq %d tells when mob left: %v vs %v",
					s, seq, rmDescribeAll(nil, fa), rmDescribeAll(nil, fb))
			}
		}
	}
}

// VTT-252 VTT-248 VTT-249 VTT-247 VTT-245 VTT-224 VTT-176 VTT-211
func TestQATestimonyRemovalBareAlsoPrecedesACorrectionsReintroduction(t *testing.T) {
	tbl := rmStandard(t)
	tbl.emit(rmDoor(true))
	tbl.emit(rmAdd("mob", "mob-1", rmOther, 10))
	tbl.emit(rmPlace("t-mob", rmKeep, "mob", rmEast))
	tbl.wantSees("VTT-248", "p1", "mob", true)
	tbl.emit(rmDoor(false))
	tbl.wantSees("VTT-248", "p1", "mob", false)
	grant := tbl.emit(rmGrant("mob", "p9", rmParty))
	revoke := tbl.emit(rmRevoke("mob", "p9"))
	tbl.want("VTT-245", "p1", grant)
	tbl.want("VTT-245", "p1", revoke)
	open := tbl.emit(rmDoor(true))
	tbl.wantBareBefore("VTT-252", "p1", open, "mob")
	tbl.wantHeld("VTT-248", "p1", "mob", rmMob("mob-1", 10, "", rmParty, ""))
}

type rmModel struct {
	present, party, cond, p1 bool
	token                    int
	door, pc1East            bool
	inc                      int
}

type rmOp struct {
	name string
	ok   func(m rmModel) bool
	emit func(tbl *rmTable, m rmModel)
}

var rmSpots = [][2]int32{{}, rmWest, rmEast, rmYardSpot}

func rmKindOf(party bool) vttv1.ActorKind {
	if party {
		return rmParty
	}
	return rmOther
}

var rmOps = []rmOp{
	rmAddOp("add", false), rmAddOp("addparty", true),
	rmPlaceOp("west", 1), rmPlaceOp("east", 2), rmPlaceOp("yard", 3),
	{"remove", func(m rmModel) bool { return m.present }, func(tbl *rmTable, _ rmModel) {
		tbl.removeActor("mob")
	}},
	{"untoken", func(m rmModel) bool { return m.token != 0 }, func(tbl *rmTable, m rmModel) {
		tbl.emit(rmTokenGone(fmt.Sprintf("t-mob%d", m.inc)))
	}},
	{"door", func(rmModel) bool { return true }, func(tbl *rmTable, m rmModel) {
		tbl.emit(rmDoor(!m.door))
	}},
	{"change", func(m rmModel) bool { return m.present }, func(tbl *rmTable, _ rmModel) {
		tbl.change("mob", -1)
	}},
	{"cond", func(m rmModel) bool { return m.present }, func(tbl *rmTable, m rmModel) {
		tbl.emit(rmCondition("mob", "marked", !m.cond))
	}},
	{"grant", func(m rmModel) bool { return m.present }, func(tbl *rmTable, m rmModel) {
		if m.p1 {
			tbl.emit(rmRevoke("mob", "p1"))
			return
		}
		tbl.emit(rmGrant("mob", "p1", rmKindOf(m.party)))
	}},
	{"standing", func(m rmModel) bool { return m.present }, func(tbl *rmTable, m rmModel) {
		tbl.emit(rmGrant("mob", "p9", rmKindOf(!m.party)))
		tbl.emit(rmRevoke("mob", "p9"))
	}},
	{"walk", func(rmModel) bool { return true }, func(tbl *rmTable, m rmModel) {
		from, to := rmPc1West, rmPc1East
		if m.pc1East {
			from, to = to, from
		}
		tbl.emit(rmMove("t-pc1", rmKeep, from, to))
	}},
}

func rmAddOp(name string, party bool) rmOp {
	return rmOp{name, func(m rmModel) bool { return !m.present }, func(tbl *rmTable, m rmModel) {
		tbl.emit(rmAdd("mob", fmt.Sprintf("mob-%d", m.inc+1), rmKindOf(party), int32(4+m.inc)))
	}}
}

func rmPlaceOp(name string, spot int) rmOp {
	return rmOp{name, func(m rmModel) bool { return m.present && m.token == 0 }, func(tbl *rmTable, m rmModel) {
		tbl.emit(rmPlace(fmt.Sprintf("t-mob%d", m.inc), rmSceneOf(spot), "mob", rmSpots[spot]))
	}}
}

func rmSceneOf(spot int) string {
	if spot == 3 {
		return rmYard
	}
	return rmKeep
}

func rmRunOps(tb testing.TB, ops []int) {
	tb.Helper()
	tbl := rmNewTable(tb, rmPlayer("p1"), rmPlayer("p3"), rmPlayer("p4"), rmSpectator("sp1", "pc1"))
	tbl.emit(rmScene(rmKeep, 7, 3, true))
	tbl.emit(rmScene(rmYard, 3, 3, false))
	tbl.addPartyMember("pc1", "p1", rmPc1West, rmKeep)
	tbl.addPartyMember("pc3", "p3", rmPc3East, rmKeep)
	m := rmModel{}
	for _, op := range ops {
		rmOps[op].emit(tbl, m)
		m = rmApplyModel(rmOps[op].name, m)
	}
}

// VTT-251 VTT-252 VTT-211 VTT-247 VTT-248 VTT-224 VTT-176 VTT-215 VTT-205 VTT-221
func TestQATestimonyRemovalHoldsOverEverySmallSequence(t *testing.T) {
	var seqs [][]int
	var walk func(m rmModel, prefix []int, depth int)
	walk = func(m rmModel, prefix []int, depth int) {
		if depth == 0 {
			seqs = append(seqs, slices.Clone(prefix))
			return
		}
		for i, op := range rmOps {
			if op.ok(m) {
				walk(rmApplyModel(op.name, m), append(prefix, i), depth-1)
			}
		}
	}
	for _, start := range [][]string{{"add", "west"}, {"add", "west", "remove"}, {"door", "add", "east", "door", "remove"}} {
		m, prefix := rmModel{}, []int{}
		for _, name := range start {
			i := slices.IndexFunc(rmOps, func(op rmOp) bool { return op.name == name })
			m, prefix = rmApplyModel(name, m), append(prefix, i)
		}
		walk(m, prefix, 3)
	}
	r := rand.New(rand.NewPCG(7, 11))
	for range 80 {
		m, seq := rmModel{}, []int{}
		for len(seq) < 16 {
			i := r.IntN(len(rmOps))
			if rmOps[i].ok(m) {
				m, seq = rmApplyModel(rmOps[i].name, m), append(seq, i)
			}
		}
		seqs = append(seqs, seq)
	}
	for _, seq := range seqs {
		rmRunOps(t, seq)
		if t.Failed() {
			names := make([]string, 0, len(seq))
			for _, i := range seq {
				names = append(names, rmOps[i].name)
			}
			t.Fatalf("first failing sequence: %v", names)
		}
	}
	t.Logf("%d sequences", len(seqs))
}

func rmApplyModel(name string, m rmModel) rmModel {
	switch name {
	case "add", "addparty":
		return rmModel{present: true, party: name == "addparty", door: m.door, pc1East: m.pc1East, inc: m.inc + 1}
	case "west":
		m.token = 1
	case "east":
		m.token = 2
	case "yard":
		m.token = 3
	case "remove":
		m.present, m.token, m.p1, m.cond = false, 0, false, false
	case "untoken":
		m.token = 0
	case "door":
		m.door = !m.door
	case "cond":
		m.cond = !m.cond
	case "grant":
		m.p1 = !m.p1
	case "standing":
		m.party = !m.party
	case "walk":
		m.pc1East = !m.pc1East
	}
	return m
}
