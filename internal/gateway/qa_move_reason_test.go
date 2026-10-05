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
)

const (
	qaMRScene    = "scn-hall"
	qaMRDM       = "p-dm"
	qaMRAgent    = "p-agent"
	qaMRPlayer   = "p-player"
	qaMRSpec     = "p-spec"
	qaMRSpecBare = "p-spec-bare"
	qaMRIdle     = "p-idle"
	qaMRShoulder = "act-ash"
	qaMRNearAct  = "act-birch"
	qaMRFarAct   = "act-rook"
	qaMROwnTok   = "tok-ash"
	qaMRNearTok  = "tok-birch"
	qaMRFarTok   = "tok-rook"
	qaMRWallX    = 6
)

func qaMRPos(x, y int32) *vttv1.GridPosition { return &vttv1.GridPosition{X: x, Y: y} }

func qaMRSceneCreated() *vttv1.SceneCreated {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 12; x++ {
		for y := int32(0); y < 5; y++ {
			kind := "floor"
			if x == qaMRWallX {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return &vttv1.SceneCreated{SceneId: qaMRScene, Name: "Hall", GridWidth: 12, GridHeight: 5, Tiles: tiles}
}

func qaMRActor(id, name string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: name, Kind: kind},
	}}}
}

func qaMRPlace(tok, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: qaMRScene, ActorId: actor, Position: qaMRPos(x, y),
	}}}
}

func qaMRSetupEvents(player string) []*vttv1.Envelope {
	return []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: qaMRSceneCreated()}},
		qaMRActor(qaMRShoulder, "Ash", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: qaMRShoulder, ParticipantId: player, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
		qaMRActor(qaMRNearAct, "Birch", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qaMRActor(qaMRFarAct, "Rook", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qaMRPlace(qaMROwnTok, qaMRShoulder, 1, 2),
		qaMRPlace(qaMRNearTok, qaMRNearAct, 3, 2),
		qaMRPlace(qaMRFarTok, qaMRFarAct, 9, 2),
	}
}

func qaMRMove(tok string, from, to *vttv1.GridPosition, reason string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tok, SceneId: qaMRScene, From: from, To: to, Reason: reason,
	}}}
}

type qaMRSeat struct {
	name   string
	pr     *gateway.Projector
	frames []*vttv1.Envelope
}

type qaMRTable struct {
	t     *testing.T
	st    *engine.State
	log   []*vttv1.Envelope
	seats []*qaMRSeat
}

func qaMRNewTable(t *testing.T) *qaMRTable {
	t.Helper()
	tb := &qaMRTable{t: t, st: engine.NewState()}
	for _, v := range []struct {
		name string
		v    gateway.Viewer
	}{
		{"player", gateway.Viewer{ParticipantID: qaMRPlayer, Role: identity.RolePlayer}},
		{"perched", gateway.Viewer{ParticipantID: qaMRSpec, Role: identity.RoleSpectator, Viewpoint: qaMRShoulder}},
		{"bare", gateway.Viewer{ParticipantID: qaMRSpecBare, Role: identity.RoleSpectator}},
		{"idle", gateway.Viewer{ParticipantID: qaMRIdle, Role: identity.RolePlayer}},
		{"dm", gateway.Viewer{ParticipantID: qaMRDM, Role: identity.RoleDM}},
		{"agent", gateway.Viewer{ParticipantID: qaMRAgent, Role: identity.RoleAgent}},
	} {
		tb.seats = append(tb.seats, &qaMRSeat{name: v.name, pr: gateway.NewProjector(v.v)})
	}
	for _, env := range qaMRSetupEvents(qaMRPlayer) {
		tb.append(env, qaMRDM, identity.RoleDM)
	}
	all := []string{qaMROwnTok, qaMRNearTok, qaMRFarTok}
	tb.requireBoard("player", []string{qaMROwnTok, qaMRNearTok}, []string{qaMRFarTok})
	tb.requireBoard("perched", []string{qaMROwnTok, qaMRNearTok}, []string{qaMRFarTok})
	tb.requireBoard("bare", nil, all)
	tb.requireBoard("idle", nil, all)
	return tb
}

func (tb *qaMRTable) stamp(env *vttv1.Envelope, participant string, role identity.Role) *vttv1.Envelope {
	seq := int64(len(tb.log) + 1)
	env.EventId = fmt.Sprintf("evt-qa-%d", seq)
	env.Sequence = seq
	env.OccurredAt = timestamppb.New(time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC).Add(time.Duration(seq) * time.Second))
	env.SessionId = "sess-qa"
	env.ActorRole = string(role)
	env.ParticipantId = participant
	return env
}

func (tb *qaMRTable) append(env *vttv1.Envelope, participant string, role identity.Role) map[string][]*vttv1.Envelope {
	tb.t.Helper()
	tb.stamp(env, participant, role)
	seq := env.GetSequence()
	if err := engine.Apply(tb.st, env); err != nil {
		tb.t.Fatalf("setup: engine.Apply(seq %d): %v", seq, err)
	}
	tb.log = append(tb.log, env)
	out := map[string][]*vttv1.Envelope{}
	for _, s := range tb.seats {
		got := s.pr.Project(env, tb.st)
		s.frames = append(s.frames, got...)
		out[s.name] = got
	}
	return out
}

func (tb *qaMRTable) seat(name string) *qaMRSeat {
	for _, s := range tb.seats {
		if s.name == name {
			return s
		}
	}
	tb.t.Fatalf("no seat %q", name)
	return nil
}

func qaMRBoard(frames []*vttv1.Envelope) map[string]bool {
	board := map[string]bool{}
	for _, f := range frames {
		switch {
		case f.GetTokenPlaced() != nil:
			board[f.GetTokenPlaced().GetTokenId()] = true
		case f.GetTokenHidden() != nil:
			delete(board, f.GetTokenHidden().GetTokenId())
		case f.GetTokenRemoved() != nil:
			delete(board, f.GetTokenRemoved().GetTokenId())
		}
	}
	return board
}

func (tb *qaMRTable) requireBoard(name string, on, off []string) {
	tb.t.Helper()
	board := qaMRBoard(tb.seat(name).frames)
	for _, tok := range on {
		if !board[tok] {
			tb.t.Fatalf("setup: %s's board lacks %s after setup (board %v); the fixture's sight assumption is wrong", name, tok, board)
		}
	}
	for _, tok := range off {
		if board[tok] {
			tb.t.Fatalf("setup: %s's board holds %s after setup (board %v); the fixture's sight assumption is wrong", name, tok, board)
		}
	}
}

func qaMRText(t *testing.T, frames []*vttv1.Envelope) string {
	t.Helper()
	var b strings.Builder
	for _, f := range frames {
		raw, err := protojson.Marshal(f)
		if err != nil {
			t.Fatalf("protojson.Marshal: %v", err)
		}
		b.Write(raw)
		b.WriteByte('\n')
	}
	return b.String()
}

func qaMRTokenMoves(frames []*vttv1.Envelope) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, f := range frames {
		if f.GetTokenMoved() != nil {
			out = append(out, f)
		}
	}
	return out
}

func qaMRMoveCommand(req, tok string, to *vttv1.GridPosition, reason *string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: req, Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: tok, To: to, Reason: reason,
	}}}
}

// VTT-260
func TestQAMoveReasonToEventCarriesTheMovesReason(t *testing.T) {
	const reason = "the floor gives way by the pool"
	for _, p := range []*identity.Participant{
		{ID: qaMRDM, Name: "Dee", Role: identity.RoleDM},
		{ID: qaMRAgent, Name: "Ayo", Role: identity.RoleAgent},
		{ID: qaMRPlayer, Name: "Pim", Role: identity.RolePlayer},
	} {
		t.Run(string(p.Role), func(t *testing.T) {
			env, err := gateway.ToEvent(qaMRMoveCommand("r-1", qaMRNearTok, qaMRPos(5, 3), proto.String(reason)), p)
			if err != nil {
				t.Fatalf("ToEvent: %v", err)
			}
			tm := env.GetTokenMoved()
			if tm == nil {
				t.Fatalf("ToEvent(move_token) payload = %T, want TokenMoved", env.GetPayload())
			}
			if tm.GetTokenId() != qaMRNearTok {
				t.Errorf("TokenMoved.token_id = %q, want %q", tm.GetTokenId(), qaMRNearTok)
			}
			if !proto.Equal(tm.GetTo(), qaMRPos(5, 3)) {
				t.Errorf("TokenMoved.to = %v, want (5,3)", tm.GetTo())
			}
			if tm.GetReason() != reason {
				t.Errorf("TokenMoved.reason = %q, want %q", tm.GetReason(), reason)
			}
			if env.GetParticipantId() != p.ID || env.GetActorRole() != string(p.Role) {
				t.Errorf("envelope stamped (%q, %q), want (%q, %q)", env.GetParticipantId(), env.GetActorRole(), p.ID, p.Role)
			}
		})
	}
}

// VTT-260
func TestQAMoveReasonToEventCarriesAGrantsKind(t *testing.T) {
	dm := &identity.Participant{ID: qaMRDM, Name: "Dee", Role: identity.RoleDM}
	for _, kind := range []vttv1.ActorKind{vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, vttv1.ActorKind_ACTOR_KIND_NON_PARTY} {
		t.Run(kind.String(), func(t *testing.T) {
			cmd := &vttv1.ClientCommand{RequestId: "r-g", Command: &vttv1.ClientCommand_GrantActorControl{
				GrantActorControl: &vttv1.GrantActorControl{ActorId: qaMRShoulder, ParticipantId: qaMRPlayer, Kind: kind},
			}}
			env, err := gateway.ToEvent(cmd, dm)
			if err != nil {
				t.Fatalf("ToEvent: %v", err)
			}
			want := &vttv1.ActorControlGranted{ActorId: qaMRShoulder, ParticipantId: qaMRPlayer, Kind: kind}
			if got := env.GetActorControlGranted(); !proto.Equal(got, want) {
				t.Errorf("ActorControlGranted = %v, want %v", got, want)
			}
		})
	}
}

// VTT-261
func TestQAMoveReasonToEventWithNoReasonRecordsNone(t *testing.T) {
	p := &identity.Participant{ID: qaMRAgent, Name: "Ayo", Role: identity.RoleAgent}
	for name, reason := range map[string]*string{"absent": nil, "empty": proto.String("")} {
		t.Run(name, func(t *testing.T) {
			env, err := gateway.ToEvent(qaMRMoveCommand("r-2", qaMRNearTok, qaMRPos(4, 1), reason), p)
			if err != nil {
				t.Fatalf("ToEvent: %v", err)
			}
			tm := env.GetTokenMoved()
			if tm == nil {
				t.Fatalf("ToEvent(move_token) payload = %T, want TokenMoved", env.GetPayload())
			}
			if tm.GetReason() != "" {
				t.Errorf("TokenMoved.reason = %q, want none", tm.GetReason())
			}
			if tm.GetTokenId() != qaMRNearTok || !proto.Equal(tm.GetTo(), qaMRPos(4, 1)) {
				t.Errorf("TokenMoved = %v, want token %s to (4,1)", tm, qaMRNearTok)
			}
		})
	}
}

func qaMRRequireStrippedCopy(t *testing.T, tb *qaMRTable, seat string) {
	t.Helper()
	const reason = "the floor gives way by the pool"
	env := qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(4, 3), reason)
	got := tb.append(env, qaMRAgent, identity.RoleAgent)[seat]
	want := forwardedOf(env)
	if len(got) != 1 {
		t.Fatalf("%s is sent %d frames for a move in sight, want exactly the move:\n%s", seat, len(got), qaMRText(t, got))
	}
	if !proto.Equal(got[0], want) {
		t.Errorf("%s is sent\n%v\nwant the event with its reason and issuer cleared and every other field kept\n%v", seat, got[0], want)
	}
	if got[0] == env {
		t.Errorf("%s is sent the shared event itself for a move with a reason, want a copy", seat)
	}
}

// VTT-262
func TestQAMoveReasonPlayerIsSentTheMoveWithItsReasonCleared(t *testing.T) {
	qaMRRequireStrippedCopy(t, qaMRNewTable(t), "player")
}

// VTT-262
func TestQAMoveReasonPerchedSpectatorIsSentTheMoveWithItsReasonCleared(t *testing.T) {
	qaMRRequireStrippedCopy(t, qaMRNewTable(t), "perched")
}

// VTT-176
func TestQAMoveReasonDMAndAgentAreSentEveryEventAsTheLogHoldsIt(t *testing.T) {
	tb := qaMRNewTable(t)
	moves := []*vttv1.Envelope{
		qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(4, 3), "the floor gives way by the pool"),
		qaMRMove(qaMRFarTok, qaMRPos(9, 2), qaMRPos(10, 3), "it slides toward the hidden pool"),
		qaMRMove(qaMRNearTok, qaMRPos(4, 3), qaMRPos(3, 1), ""),
		qaMRMove(qaMRNearTok, qaMRPos(3, 1), qaMRPos(9, 1), "it is dragged beyond the wall"),
		qaMRMove(qaMRFarTok, qaMRPos(10, 3), qaMRPos(2, 3), "it climbs out of the pool"),
	}
	for _, env := range moves {
		before := proto.Clone(tb.stamp(env, qaMRAgent, identity.RoleAgent)).(*vttv1.Envelope)
		got := tb.append(env, qaMRAgent, identity.RoleAgent)
		for _, seat := range []string{"dm", "agent"} {
			frames := got[seat]
			if len(frames) != 1 || frames[0] != env {
				t.Errorf("seq %d: %s is sent %d frames, want exactly the shared event:\n%s", env.GetSequence(), seat, len(frames), qaMRText(t, frames))
				continue
			}
			if !proto.Equal(frames[0], before) {
				t.Errorf("seq %d: %s is sent\n%v\nwant the event as appended\n%v", env.GetSequence(), seat, frames[0], before)
			}
		}
	}
	for _, seat := range []string{"dm", "agent"} {
		frames := tb.seat(seat).frames
		if len(frames) != len(tb.log) {
			t.Fatalf("%s holds %d frames over a log of %d events", seat, len(frames), len(tb.log))
		}
		for i := range frames {
			if frames[i] != tb.log[i] {
				t.Errorf("%s frame %d is not the log's event %d", seat, i, tb.log[i].GetSequence())
			}
		}
	}
}

// VTT-206
func TestQAMoveReasonAHiddenMoveIsSentToNoPlayerOrSpectator(t *testing.T) {
	tb := qaMRNewTable(t)
	const reason = "it slides toward the hidden pool"
	got := tb.append(qaMRMove(qaMRFarTok, qaMRPos(9, 2), qaMRPos(10, 3), reason), qaMRAgent, identity.RoleAgent)
	for _, seat := range []string{"player", "perched", "bare", "idle"} {
		if len(got[seat]) != 0 {
			t.Errorf("%s is sent %d frames for a move it saw neither end of, want none:\n%s", seat, len(got[seat]), qaMRText(t, got[seat]))
		}
		if text := qaMRText(t, tb.seat(seat).frames); strings.Contains(text, reason) {
			t.Errorf("%s's stream carries the reason %q", seat, reason)
		}
	}
	if dm := got["dm"]; len(dm) != 1 || dm[0].GetTokenMoved().GetReason() != reason {
		t.Errorf("dm is sent %v, want the move with its reason", dm)
	}
}

// VTT-262
func TestQAMoveReasonAReasonNamingAnUnseenActorReachesNoPlayerOrSpectator(t *testing.T) {
	tb := qaMRNewTable(t)
	for _, seat := range []string{"player", "perched"} {
		if strings.Contains(qaMRText(t, tb.seat(seat).frames), qaMRFarAct) {
			t.Fatalf("setup: %s already holds %s before the move", seat, qaMRFarAct)
		}
	}
	reason := qaMRFarAct + " shoves it from beyond the wall"
	got := tb.append(qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(4, 3), reason), qaMRAgent, identity.RoleAgent)
	for _, seat := range []string{"player", "perched"} {
		if len(qaMRTokenMoves(got[seat])) != 1 {
			t.Fatalf("%s is sent %d moves for a move in sight, want 1", seat, len(qaMRTokenMoves(got[seat])))
		}
		if strings.Contains(qaMRText(t, tb.seat(seat).frames), qaMRFarAct) {
			t.Errorf("%s's stream names %s, which it does not see", seat, qaMRFarAct)
		}
	}
	for _, seat := range []string{"dm", "agent"} {
		if g := got[seat]; len(g) != 1 || g[0].GetTokenMoved().GetReason() != reason {
			t.Errorf("%s is sent %v, want the move with its reason", seat, g)
		}
	}
}

// VTT-206
func TestQAMoveReasonAMoveWithNoReasonIsForwardedLessItsIssuer(t *testing.T) {
	tb := qaMRNewTable(t)
	env := qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(3, 1), "")
	before := proto.Clone(tb.stamp(env, qaMRAgent, identity.RoleAgent)).(*vttv1.Envelope)
	got := tb.append(env, qaMRAgent, identity.RoleAgent)
	for _, seat := range []string{"player", "perched"} {
		frames := got[seat]
		if len(frames) != 1 {
			t.Fatalf("%s is sent %d frames for a move in sight, want exactly the move:\n%s", seat, len(frames), qaMRText(t, frames))
		}
		if !proto.Equal(frames[0], forwardedOf(before)) {
			t.Errorf("%s is sent\n%v\nwant the event as appended less its issuer\n%v", seat, frames[0], forwardedOf(before))
		}
	}
}

// SPEC-016, How it works: "a copy of the event with its `participant_id` and
// `actor_role` cleared".
func TestQAMoveReasonAMoveWithNoReasonIsACopyForAPlayer(t *testing.T) {
	tb := qaMRNewTable(t)
	env := qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(3, 1), "")
	got := tb.append(env, qaMRAgent, identity.RoleAgent)
	for _, seat := range []string{"player", "perched"} {
		if len(got[seat]) != 1 || got[seat][0] == env || !proto.Equal(got[seat][0], forwardedOf(env)) {
			t.Errorf("%s is not sent a copy of the event less its issuer for a move with no reason", seat)
		}
	}
}

// VTT-206
func TestQAMoveReasonAMoveOutOfSightSendsAHidingAndNoReason(t *testing.T) {
	tb := qaMRNewTable(t)
	const reason = "it is dragged beyond the wall"
	got := tb.append(qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(9, 1), reason), qaMRAgent, identity.RoleAgent)
	for _, seat := range []string{"player", "perched"} {
		frames := got[seat]
		if n := len(qaMRTokenMoves(frames)); n != 0 {
			t.Errorf("%s is sent %d moves for a token that left its sight, want 0", seat, n)
		}
		hidden := false
		for _, f := range frames {
			if f.GetTokenHidden().GetTokenId() == qaMRNearTok {
				hidden = true
			}
		}
		if !hidden {
			t.Errorf("%s is sent no TokenHidden for %s:\n%s", seat, qaMRNearTok, qaMRText(t, frames))
		}
		if strings.Contains(qaMRText(t, tb.seat(seat).frames), reason) {
			t.Errorf("%s's stream carries the reason %q", seat, reason)
		}
	}
}

// VTT-206
func TestQAMoveReasonAMoveIntoSightSendsAPlacingAndNoReason(t *testing.T) {
	tb := qaMRNewTable(t)
	const reason = "it climbs out of the pool"
	got := tb.append(qaMRMove(qaMRFarTok, qaMRPos(9, 2), qaMRPos(2, 3), reason), qaMRAgent, identity.RoleAgent)
	for _, seat := range []string{"player", "perched"} {
		frames := got[seat]
		if n := len(qaMRTokenMoves(frames)); n != 0 {
			t.Errorf("%s is sent %d moves for a token that was not on its board, want 0", seat, n)
		}
		placed := false
		for _, f := range frames {
			if tp := f.GetTokenPlaced(); tp.GetTokenId() == qaMRFarTok && proto.Equal(tp.GetPosition(), qaMRPos(2, 3)) {
				placed = true
			}
		}
		if !placed {
			t.Errorf("%s is sent no TokenPlaced for %s at (2,3):\n%s", seat, qaMRFarTok, qaMRText(t, frames))
		}
		if strings.Contains(qaMRText(t, tb.seat(seat).frames), reason) {
			t.Errorf("%s's stream carries the reason %q", seat, reason)
		}
	}
}

// VTT-262
func TestQAMoveReasonSeatsWithoutEyesAreSentNoMoveAndNoReason(t *testing.T) {
	tb := qaMRNewTable(t)
	reasons := []string{"the floor gives way by the pool", "it slides toward the hidden pool", "it is dragged beyond the wall"}
	tb.append(qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(4, 3), reasons[0]), qaMRAgent, identity.RoleAgent)
	tb.append(qaMRMove(qaMRFarTok, qaMRPos(9, 2), qaMRPos(10, 3), reasons[1]), qaMRAgent, identity.RoleAgent)
	tb.append(qaMRMove(qaMRNearTok, qaMRPos(4, 3), qaMRPos(9, 1), reasons[2]), qaMRAgent, identity.RoleAgent)
	for _, seat := range []string{"bare", "idle"} {
		frames := tb.seat(seat).frames
		if n := len(qaMRTokenMoves(frames)); n != 0 {
			t.Errorf("%s, with no eyes, is sent %d moves", seat, n)
		}
		text := qaMRText(t, frames)
		for _, r := range reasons {
			if strings.Contains(text, r) {
				t.Errorf("%s's stream carries the reason %q", seat, r)
			}
		}
	}
}

// SPEC-016, How it works: "It answers any role but those and
// identity.RolePlayer and identity.RoleSpectator with nothing".
func TestQAMoveReasonAnUnknownRoleOrAMissingStateIsSentNothing(t *testing.T) {
	tb := qaMRNewTable(t)
	odd := gateway.NewProjector(gateway.Viewer{ParticipantID: "p-odd", Role: identity.Role("observer")})
	for _, env := range tb.log {
		if got := odd.Project(env, tb.st); len(got) != 0 {
			t.Fatalf("an unknown role is sent %d frames for seq %d", len(got), env.GetSequence())
		}
	}
	env := qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(4, 3), "the floor gives way by the pool")
	tb.append(env, qaMRAgent, identity.RoleAgent)
	if got := odd.Project(env, tb.st); len(got) != 0 {
		t.Errorf("an unknown role is sent %d frames for a move with a reason", len(got))
	}
	for _, v := range []gateway.Viewer{
		{ParticipantID: qaMRPlayer, Role: identity.RolePlayer},
		{ParticipantID: qaMRSpec, Role: identity.RoleSpectator, Viewpoint: qaMRShoulder},
	} {
		if got := gateway.NewProjector(v).Project(env, nil); len(got) != 0 {
			t.Errorf("%s with a nil state is sent %d frames, want none", v.Role, len(got))
		}
	}
}

// VTT-222
func TestQAMoveReasonProjectingToEverySeatLeavesTheEventAndStateUnchanged(t *testing.T) {
	tb := qaMRNewTable(t)
	moves := []*vttv1.Envelope{
		qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(4, 3), "the floor gives way by the pool"),
		qaMRMove(qaMRFarTok, qaMRPos(9, 2), qaMRPos(10, 3), "it slides toward the hidden pool"),
		qaMRMove(qaMRNearTok, qaMRPos(4, 3), qaMRPos(3, 1), ""),
		qaMRMove(qaMRNearTok, qaMRPos(3, 1), qaMRPos(9, 1), "it is dragged beyond the wall"),
		qaMRMove(qaMRFarTok, qaMRPos(10, 3), qaMRPos(2, 3), "it climbs out of the pool"),
	}
	for _, env := range moves {
		before := proto.Clone(tb.stamp(env, qaMRAgent, identity.RoleAgent)).(*vttv1.Envelope)
		tb.append(env, qaMRAgent, identity.RoleAgent)
		if !proto.Equal(env, before) {
			t.Errorf("seq %d: the shared event after projecting to six seats is\n%v\nwant\n%v", env.GetSequence(), env, before)
		}
	}
	twin := engine.NewState()
	for _, env := range tb.log {
		if err := engine.Apply(twin, proto.Clone(env).(*vttv1.Envelope)); err != nil {
			t.Fatalf("twin fold: %v", err)
		}
	}
	got, err := json.Marshal(tb.st)
	if err != nil {
		t.Fatalf("json.Marshal(state): %v", err)
	}
	want, err := json.Marshal(twin)
	if err != nil {
		t.Fatalf("json.Marshal(twin): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("the state projected against differs from an unprojected fold of the same log:\n got %s\nwant %s", got, want)
	}
}

// SPEC-016, How it works: "every frame it builds is new".
func TestQAMoveReasonWritingToAPlayersCopyDoesNotReachTheSharedEvent(t *testing.T) {
	tb := qaMRNewTable(t)
	env := qaMRMove(qaMRNearTok, qaMRPos(3, 2), qaMRPos(4, 3), "the floor gives way by the pool")
	got := tb.append(env, qaMRAgent, identity.RoleAgent)
	before := proto.Clone(env).(*vttv1.Envelope)
	perched := proto.Clone(got["perched"][0]).(*vttv1.Envelope)
	cp := got["player"][0]
	if cp == env || cp.GetTokenMoved() == env.GetTokenMoved() || cp.GetTokenMoved().GetTo() == env.GetTokenMoved().GetTo() {
		t.Fatalf("the player's copy shares a message with the shared event")
	}
	cp.GetTokenMoved().GetTo().X = 11
	cp.GetTokenMoved().GetFrom().Y = 4
	cp.GetTokenMoved().Reason = "written by a seat"
	cp.Sequence = 99
	if !proto.Equal(env, before) {
		t.Errorf("writing to the player's copy changed the shared event:\n%v\nwant\n%v", env, before)
	}
	if !proto.Equal(got["perched"][0], perched) {
		t.Errorf("writing to the player's copy changed the spectator's frame:\n%v\nwant\n%v", got["perched"][0], perched)
	}
}

type qaMRWire struct {
	raw   []byte
	frame *vttv1.ServerFrame
}

type qaMRConn struct {
	name string
	conn *websocket.Conn
	in   chan qaMRWire
	seen []qaMRWire
}

func qaMRDial(t *testing.T, ctx context.Context, base, name, token string, after int64) *qaMRConn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(base, "http") + "/ws?token=" + url.QueryEscape(token)
	if after > 0 {
		u += fmt.Sprintf("&after=%d", after)
	}
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", name, err)
	}
	conn.SetReadLimit(1 << 22)
	qc := &qaMRConn{name: name, conn: conn, in: make(chan qaMRWire, 4096)}
	go func() {
		defer close(qc.in)
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if protojson.Unmarshal(raw, f) != nil {
				f = nil
			}
			qc.in <- qaMRWire{raw: raw, frame: f}
		}
	}()
	t.Cleanup(func() { _ = conn.CloseNow() })
	return qc
}

func (qc *qaMRConn) until(t *testing.T, ctx context.Context, what string, match func(*vttv1.ServerFrame) bool) *vttv1.ServerFrame {
	t.Helper()
	for {
		select {
		case w, ok := <-qc.in:
			if !ok {
				t.Fatalf("%s: connection closed while waiting for %s", qc.name, what)
			}
			qc.seen = append(qc.seen, w)
			if w.frame == nil {
				t.Fatalf("%s: undecodable frame %s", qc.name, w.raw)
			}
			if match(w.frame) {
				return w.frame
			}
		case <-ctx.Done():
			t.Fatalf("%s: timed out waiting for %s", qc.name, what)
		}
	}
}

func (qc *qaMRConn) command(t *testing.T, ctx context.Context, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	t.Helper()
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal command: %v", err)
	}
	if err := qc.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("%s: write: %v", qc.name, err)
	}
	return qc.until(t, ctx, "result "+cmd.GetRequestId(), func(f *vttv1.ServerFrame) bool {
		return f.GetResult().GetRequestId() == cmd.GetRequestId()
	}).GetResult()
}

func (qc *qaMRConn) eventsAt(seq int64) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, w := range qc.seen {
		if e := w.frame.GetEvent(); e != nil && e.GetSequence() == seq {
			out = append(out, e)
		}
	}
	return out
}

func (qc *qaMRConn) events() []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, w := range qc.seen {
		if e := w.frame.GetEvent(); e != nil {
			out = append(out, e)
		}
	}
	return out
}

func (qc *qaMRConn) rawContains(text string) bool {
	for _, w := range qc.seen {
		if strings.Contains(string(w.raw), text) {
			return true
		}
	}
	return false
}

type qaMRLive struct {
	ctx                          context.Context
	c                            *campaign.Campaign
	base                         string
	tokens                       map[identity.Role]string
	dmID, agentID, playerID      string
	dm, agent, player, spectator *qaMRConn
	conns                        []*qaMRConn
	barriers                     int
}

func qaMRStartLive(t *testing.T) *qaMRLive {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	tokens := map[identity.Role]string{}
	l := &qaMRLive{ctx: ctx, c: c, tokens: tokens}
	for _, inv := range []struct {
		name string
		role identity.Role
		id   *string
	}{
		{"Dee", identity.RoleDM, &l.dmID},
		{"Ayo", identity.RoleAgent, &l.agentID},
		{"Pim", identity.RolePlayer, &l.playerID},
		{"Sol", identity.RoleSpectator, new(string)},
	} {
		tok, id, err := db.CreateInvite(inv.name, inv.role)
		if err != nil {
			t.Fatalf("CreateInvite: %v", err)
		}
		tokens[inv.role], *inv.id = tok, id
	}
	for i, env := range qaMRSetupEvents(l.playerID) {
		env.EventId = fmt.Sprintf("evt-seed-%d", i+1)
		env.OccurredAt = timestamppb.Now()
		env.ActorRole = string(identity.RoleDM)
		env.ParticipantId = l.dmID
		if _, err := c.Append(env); err != nil {
			t.Fatalf("seed %d: campaign.Append: %v", i+1, err)
		}
	}
	srv := httptest.NewServer(gateway.New(c, db).Handler())
	t.Cleanup(srv.Close)
	l.base = srv.URL
	l.dm = qaMRDial(t, ctx, srv.URL, "dm", tokens[identity.RoleDM], 0)
	l.agent = qaMRDial(t, ctx, srv.URL, "agent", tokens[identity.RoleAgent], 0)
	l.player = qaMRDial(t, ctx, srv.URL, "player", tokens[identity.RolePlayer], 0)
	l.spectator = qaMRDial(t, ctx, srv.URL, "spectator", tokens[identity.RoleSpectator], 0)
	l.conns = []*qaMRConn{l.dm, l.agent, l.player, l.spectator}
	for _, qc := range l.conns {
		qc.until(t, ctx, "catch-up head", func(f *vttv1.ServerFrame) bool { return f.GetCatchUpHead() != nil })
	}
	placed := func(tok string) func(*vttv1.ServerFrame) bool {
		return func(f *vttv1.ServerFrame) bool { return f.GetEvent().GetTokenPlaced().GetTokenId() == tok }
	}
	l.player.until(t, ctx, "the near token placed", placed(qaMRNearTok))
	res := l.spectator.command(t, ctx, &vttv1.ClientCommand{RequestId: "perch", Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: qaMRShoulder},
	}})
	if !res.GetOk() {
		t.Fatalf("setup: set_viewpoint refused: %s", res.GetError())
	}
	l.spectator.until(t, ctx, "the near token placed after the perch", placed(qaMRNearTok))
	return l
}

func (l *qaMRLive) barrier(t *testing.T) int64 {
	t.Helper()
	l.barriers++
	text := fmt.Sprintf("barrier %d", l.barriers)
	res := l.dm.command(t, l.ctx, &vttv1.ClientCommand{RequestId: text, Command: &vttv1.ClientCommand_AddNarration{
		AddNarration: &vttv1.AddNarration{Text: text},
	}})
	if !res.GetOk() {
		t.Fatalf("barrier narration refused: %s", res.GetError())
	}
	for _, qc := range l.conns {
		qc.until(t, l.ctx, text, func(f *vttv1.ServerFrame) bool { return f.GetEvent().GetNarrationAdded().GetText() == text })
	}
	return res.GetSequence()
}

func (l *qaMRLive) logThrough(t *testing.T, seq int64) []*vttv1.Envelope {
	t.Helper()
	events, unsubscribe, _, err := l.c.Subscribe(0, 256)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsubscribe()
	var out []*vttv1.Envelope
	for {
		select {
		case env, ok := <-events:
			if !ok {
				t.Fatalf("log subscription closed before seq %d", seq)
			}
			out = append(out, env)
			if env.GetSequence() >= seq {
				return out
			}
		case <-l.ctx.Done():
			t.Fatalf("timed out reading the log through seq %d", seq)
		}
	}
}

func qaMRLogged(t *testing.T, log []*vttv1.Envelope, seq int64) *vttv1.Envelope {
	t.Helper()
	for _, env := range log {
		if env.GetSequence() == seq {
			return env
		}
	}
	t.Fatalf("the log holds no seq %d", seq)
	return nil
}

func (l *qaMRLive) move(t *testing.T, issuer *qaMRConn, req, tok string, to *vttv1.GridPosition, reason *string) int64 {
	t.Helper()
	res := issuer.command(t, l.ctx, qaMRMoveCommand(req, tok, to, reason))
	if !res.GetOk() {
		t.Fatalf("%s's move_token refused: %s", issuer.name, res.GetError())
	}
	return res.GetSequence()
}

func qaMRRequireSentWithoutReason(t *testing.T, qc *qaMRConn, seq int64, logged *vttv1.Envelope, reason string) {
	t.Helper()
	got := qc.eventsAt(seq)
	if len(got) != 1 {
		t.Fatalf("%s is sent %d frames at seq %d, want exactly the move", qc.name, len(got), seq)
	}
	if want := forwardedOf(logged); !proto.Equal(got[0], want) {
		t.Errorf("%s is sent\n%v\nwant the logged move with its reason and issuer cleared\n%v", qc.name, got[0], want)
	}
	if qc.rawContains(reason) {
		t.Errorf("%s's wire carries the reason %q", qc.name, reason)
	}
}

// VTT-176
func TestQAMoveReasonOverTheWireDMAndAgentHearAnAgentsReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	head := l.barrier(t)
	log := l.logThrough(t, head)
	logged := qaMRLogged(t, log, seq)
	want := &vttv1.TokenMoved{TokenId: qaMRNearTok, SceneId: qaMRScene, From: qaMRPos(3, 2), To: qaMRPos(4, 3), Reason: reason}
	if !proto.Equal(logged.GetTokenMoved(), want) {
		t.Errorf("the log holds %v, want %v", logged.GetTokenMoved(), want)
	}
	if logged.GetParticipantId() != l.agentID || logged.GetActorRole() != string(identity.RoleAgent) {
		t.Errorf("the logged move is stamped (%q, %q), want (%q, agent)", logged.GetParticipantId(), logged.GetActorRole(), l.agentID)
	}
	for _, qc := range []*qaMRConn{l.dm, l.agent} {
		got := qc.events()
		if len(got) != len(log) {
			t.Fatalf("%s is sent %d events through seq %d, the log holds %d", qc.name, len(got), head, len(log))
		}
		for i := range got {
			if !proto.Equal(got[i], log[i]) {
				t.Errorf("%s event %d is\n%v\nwant the log's\n%v", qc.name, i, got[i], log[i])
			}
		}
	}
}

// VTT-262
func TestQAMoveReasonOverTheWirePlayerAndSpectatorDoNotHearAnAgentsReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	for _, qc := range []*qaMRConn{l.player, l.spectator} {
		qaMRRequireSentWithoutReason(t, qc, seq, logged, reason)
	}
}

// VTT-262
func TestQAMoveReasonOverTheWireAPlayerIsNotSentBackItsOwnReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "it steps around the pool"
	seq := l.move(t, l.player, "mv-player", qaMROwnTok, qaMRPos(2, 2), proto.String(reason))
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	if got := logged.GetTokenMoved().GetReason(); got != reason {
		t.Errorf("the log holds reason %q for the player's move, want %q", got, reason)
	}
	for _, qc := range []*qaMRConn{l.player, l.spectator} {
		qaMRRequireSentWithoutReason(t, qc, seq, logged, reason)
	}
	for _, qc := range []*qaMRConn{l.dm, l.agent} {
		if got := qc.eventsAt(seq); len(got) != 1 || !proto.Equal(got[0], logged) {
			t.Errorf("%s is sent %v at seq %d, want the logged move %v", qc.name, got, seq, logged)
		}
	}
}

// VTT-261
func TestQAMoveReasonOverTheWireAMoveWithNoReasonAppendsNone(t *testing.T) {
	l := qaMRStartLive(t)
	seq := l.move(t, l.dm, "mv-plain", qaMRNearTok, qaMRPos(3, 1), nil)
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	want := &vttv1.TokenMoved{TokenId: qaMRNearTok, SceneId: qaMRScene, From: qaMRPos(3, 2), To: qaMRPos(3, 1)}
	if !proto.Equal(logged.GetTokenMoved(), want) {
		t.Errorf("the log holds %v, want %v with no reason", logged.GetTokenMoved(), want)
	}
	for _, qc := range l.conns {
		want := logged
		if qc == l.player || qc == l.spectator {
			want = forwardedOf(logged)
		}
		if got := qc.eventsAt(seq); len(got) != 1 || !proto.Equal(got[0], want) {
			t.Errorf("%s is sent %v at seq %d, want %v", qc.name, got, seq, want)
		}
	}
}

// VTT-206
func TestQAMoveReasonOverTheWireAHiddenMoveReachesNoPlayerOrSpectator(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "it slides toward the hidden pool"
	seq := l.move(t, l.dm, "mv-hidden", qaMRFarTok, qaMRPos(10, 3), proto.String(reason))
	logged := qaMRLogged(t, l.logThrough(t, l.barrier(t)), seq)
	for _, qc := range []*qaMRConn{l.player, l.spectator} {
		if got := qc.eventsAt(seq); len(got) != 0 {
			t.Errorf("%s is sent %d frames at seq %d for a move it saw neither end of", qc.name, len(got), seq)
		}
		if qc.rawContains(reason) {
			t.Errorf("%s's wire carries the reason %q", qc.name, reason)
		}
	}
	for _, qc := range []*qaMRConn{l.dm, l.agent} {
		if got := qc.eventsAt(seq); len(got) != 1 || !proto.Equal(got[0], logged) {
			t.Errorf("%s is sent %v at seq %d, want the logged move %v", qc.name, got, seq, logged)
		}
	}
}

// VTT-262
func TestQAMoveReasonOverTheWireAPlayerCatchingUpIsSentNoReason(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	head := l.barrier(t)
	logged := qaMRLogged(t, l.logThrough(t, head), seq)
	for _, after := range []int64{0, seq - 1} {
		qc := qaMRDial(t, l.ctx, l.base, fmt.Sprintf("player after %d", after), l.tokens[identity.RolePlayer], after)
		qc.until(t, l.ctx, "the barrier in the backlog", func(f *vttv1.ServerFrame) bool {
			return f.GetEvent().GetNarrationAdded().GetText() == "barrier 1"
		})
		qaMRRequireSentWithoutReason(t, qc, seq, logged, reason)
	}
}

// VTT-176
func TestQAMoveReasonOverTheWireADMCatchingUpIsSentEveryEventAfterItsCursorUnchanged(t *testing.T) {
	l := qaMRStartLive(t)
	const reason = "the floor gives way by the pool"
	seq := l.move(t, l.agent, "mv-agent", qaMRNearTok, qaMRPos(4, 3), proto.String(reason))
	head := l.barrier(t)
	var want []*vttv1.Envelope
	for _, env := range l.logThrough(t, head) {
		if env.GetSequence() > seq-1 {
			want = append(want, env)
		}
	}
	qc := qaMRDial(t, l.ctx, l.base, "dm after cursor", l.tokens[identity.RoleDM], seq-1)
	qc.until(t, l.ctx, "the barrier in the backlog", func(f *vttv1.ServerFrame) bool {
		return f.GetEvent().GetNarrationAdded().GetText() == "barrier 1"
	})
	got := qc.events()
	if len(got) != len(want) {
		t.Fatalf("dm after cursor %d is sent %d events, want %d", seq-1, len(got), len(want))
	}
	for i := range got {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("dm after cursor event %d is\n%v\nwant the log's\n%v", i, got[i], want[i])
		}
	}
	if got[0].GetTokenMoved().GetReason() != reason {
		t.Errorf("dm after cursor is sent reason %q, want %q", got[0].GetTokenMoved().GetReason(), reason)
	}
}
