package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func removeCmd(requestID, participantID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: requestID, Command: &vttv1.ClientCommand_RemoveParticipant{
		RemoveParticipant: &vttv1.RemoveParticipant{ParticipantId: participantID}}}
}

func (f *gwFixture) removeAs(t *testing.T, token, participantID string) *vttv1.CommandResult {
	t.Helper()
	conn := f.dial(token, f.head(t))
	sendCommand(t, conn, removeCmd("remove-"+participantID, participantID))
	return readResult(t, conn)
}

func (f *gwFixture) invite(t *testing.T, name string, role identity.Role) (token, id string) {
	t.Helper()
	token, id, err := f.ids.CreateInvite(name, role)
	if err != nil {
		t.Fatal(err)
	}
	return token, id
}

func waitClosed(t *testing.T, conn *websocket.Conn) websocket.CloseError {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			var ce websocket.CloseError
			if !errors.As(err, &ce) {
				t.Fatalf("the connection ended with %v, not a websocket close", err)
			}
			return ce
		}
	}
}

func credentialClose(t *testing.T) websocket.CloseError {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "testdata", "credential_close.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	return websocket.CloseError{Code: websocket.StatusCode(want.Code), Reason: want.Reason}
}

// VTT-557 VTT-034 VTT-074
func TestTheDMRemovesASpectatorWhoIsNotConnected(t *testing.T) {
	f := newGWFixture(t)
	id := f.playerIDFor(t, f.spectatorToken)
	if r := f.removeAs(t, f.dmToken, id); !r.GetOk() {
		t.Fatalf("the removal was refused: %s", r.GetError())
	}
	if _, err := f.ids.Lookup(id); !errors.Is(err, identity.ErrInvalidToken) {
		t.Fatalf("Lookup of the removed spectator: %v, want ErrInvalidToken", err)
	}
	list, err := f.ids.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p.ID == id {
			t.Fatalf("List still names the removed spectator: %+v", p)
		}
	}
	if _, err := f.ids.Lookup(f.playerID); err != nil {
		t.Fatalf("Lookup of a player nobody removed: %v", err)
	}
}

// VTT-558 VTT-080 VTT-104 VTT-376
func TestEveryConnectionOfARemovedParticipantClosesAtOnce(t *testing.T) {
	f := newGWFixture(t)
	id := f.playerIDFor(t, f.spectatorToken)
	watcher := f.dial(f.dmToken, 0)
	expectCatchUpHead(t, watcher)
	expectPresenceSnapshot(t, watcher)
	one, two := f.dial(f.spectatorToken, 0), f.dial(f.spectatorToken, 0)
	expectPresenceChanged(t, watcher, id, vttv1.PresenceState_PRESENCE_STATE_CONNECTED)
	before := f.head(t)

	if r := f.removeAs(t, f.dmToken, id); !r.GetOk() {
		t.Fatalf("the removal was refused: %s", r.GetError())
	}
	want := credentialClose(t)
	for name, conn := range map[string]*websocket.Conn{"first": one, "second": two} {
		if got := waitClosed(t, conn); got.Code != want.Code || got.Reason != want.Reason {
			t.Fatalf("the %s connection closed with %d %q, want %d %q",
				name, got.Code, got.Reason, want.Code, want.Reason)
		}
	}
	expectPresenceChanged(t, watcher, id, vttv1.PresenceState_PRESENCE_STATE_DISCONNECTED)
	if after := f.head(t); after != before {
		t.Fatalf("the head moved %d -> %d for a spectator who controls nothing", before, after)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, f.wsURL(f.spectatorToken, 0), nil)
	if err == nil {
		conn.CloseNow()
		t.Fatal("a dial with the removed spectator's credential was accepted")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a dial with the removed spectator's credential: %v, want 401", err)
	}
}

// VTT-559 VTT-560 VTT-562 VTT-563 VTT-392
func TestARemovalRevokesEveryControlTheParticipantHolds(t *testing.T) {
	f := newGWFixture(t)
	ivo := f.playerIDFor(t, f.otherPlayerToken)
	dmID := f.playerIDFor(t, f.dmToken)
	for _, id := range []string{"a3", "a5", "a2", "a4"} {
		kind := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
		if id == "a3" {
			kind = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
		}
		appendEvents(t, f.campaign,
			&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: kind}},
			&vttv1.ActorControlGranted{ActorId: id, ParticipantId: f.playerID, Kind: kind})
	}
	appendEvents(t, f.campaign,
		&vttv1.ActorControlGranted{ActorId: "a2", ParticipantId: ivo,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	appendEvents(t, f.campaign, partyMemberOf("a6", ivo)...)
	kinds := map[string]vttv1.ActorKind{}
	for id, a := range f.campaign.State().Actors {
		kinds[id] = a.GetKind()
	}

	before := f.head(t)
	dm := f.dial(f.dmToken, before)
	sendCommand(t, dm, removeCmd("remove-lera", f.playerID))
	r := readResult(t, dm)
	if !r.GetOk() || r.GetSequence() != before+1 {
		t.Fatalf("result %v, want ok at sequence %d", r, before+1)
	}
	for _, want := range []string{"a1", "a2", "a3", "a4", "a5"} {
		ev := readEvent(t, dm)
		rv := ev.GetActorControlRevoked()
		if rv.GetActorId() != want || rv.GetParticipantId() != f.playerID {
			t.Fatalf("event %d: %v, want the revocation of %s from Lera", ev.GetSequence(), ev, want)
		}
		if ev.GetParticipantId() != dmID || ev.GetActorRole() != string(identity.RoleDM) {
			t.Fatalf("event %d is stamped %q/%q, want the DM's id and role",
				ev.GetSequence(), ev.GetParticipantId(), ev.GetActorRole())
		}
	}
	assertNoFrameWithin(t, dm, 300*time.Millisecond)

	st := f.campaign.State()
	for id, a := range st.Actors {
		if slices.Contains(a.GetControllerIds(), f.playerID) {
			t.Fatalf("%s still lists Lera among its controllers", id)
		}
		if a.GetKind() != kinds[id] {
			t.Fatalf("%s's kind went %v -> %v", id, kinds[id], a.GetKind())
		}
	}
	for _, id := range []string{"a2", "a6"} {
		if !slices.Contains(st.Actors[id].GetControllerIds(), ivo) {
			t.Fatalf("%s lost Ivo, whom nobody removed: %v", id, st.Actors[id].GetControllerIds())
		}
	}
}

// VTT-561
func TestARemovalOfSomebodyWhoControlsNothingAppendsNothing(t *testing.T) {
	f := newGWFixture(t)
	before := f.head(t)
	r := f.removeAs(t, f.dmToken, f.playerIDFor(t, f.spectatorToken))
	if !r.GetOk() || r.GetSequence() != 0 {
		t.Fatalf("result %v, want ok at sequence 0", r)
	}
	if after := f.head(t); after != before {
		t.Fatalf("the head moved %d -> %d", before, after)
	}
}

func drainQuiet(w *watchConn) {
	for w.next(150*time.Millisecond) != nil {
	}
}

func framesAfterClose(w *watchConn) (n int) {
	frames := w.pending
	for done := false; !done; {
		select {
		case fr := <-w.frames:
			frames = append(frames, fr)
		default:
			done = true
		}
	}
	for _, fr := range frames {
		if fr.GetPresenceSnapshot() == nil && fr.GetPresenceChanged() == nil {
			n++
		}
	}
	return n
}

// VTT-566
func TestARemovedPlayerIsSentNothingTheRemovalCaused(t *testing.T) {
	f := newGWFixture(t)
	type seat struct {
		id, token, actor string
		w                *watchConn
	}
	var seats []*seat
	for i := range 10 {
		token, id := f.invite(t, "Player "+string(rune('A'+i)), identity.RolePlayer)
		actor := "pm" + string(rune('a'+i))
		appendEvents(t, f.campaign, partyMemberOf(actor, id)...)
		seats = append(seats, &seat{id: id, token: token, actor: actor})
	}
	for _, s := range seats {
		s.w = f.watch(s.token, 0, "")
		if _, vp := s.w.opening(); vp != s.actor {
			t.Fatalf("%s opens watching %q, want %s", s.id, vp, s.actor)
		}
	}
	dm := f.dial(f.dmToken, f.head(t))
	for _, s := range seats {
		drainQuiet(s.w)
		s.w.pending = nil
		sendCommand(t, dm, removeCmd("remove-"+s.id, s.id))
		if r := readResult(t, dm); !r.GetOk() {
			t.Fatalf("removing %s: %s", s.id, r.GetError())
		}
		select {
		case <-s.w.closed:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s's connection is still open", s.id)
		}
		if n := framesAfterClose(s.w); n != 0 {
			t.Fatalf("%s was sent %d frames after their removal", s.id, n)
		}
	}
}

// VTT-564 VTT-162
func TestARemovalOfADMAnAgentTheIssuerAnUnknownOrARevokedTargetIsRefused(t *testing.T) {
	f := newGWFixture(t)
	_, dm2 := f.invite(t, "Second DM", identity.RoleDM)
	agent := f.playerIDFor(t, f.agentToken)
	self := f.playerIDFor(t, f.dmToken)
	_, gone := f.invite(t, "Gone", identity.RolePlayer)
	appendEvents(t, f.campaign, partyMemberOf("ag", gone)...)
	if err := f.ids.Revoke(gone); err != nil {
		t.Fatal(err)
	}
	agentConn := f.watch(f.agentToken, f.head(t), "")
	agentConn.must()
	before := f.head(t)

	for name, id := range map[string]string{"a dm": dm2, "the agent": agent, "the issuer": self,
		"an unknown id": "never-minted", "a revoked player": gone} {
		if r := f.removeAs(t, f.dmToken, id); r.GetOk() {
			t.Fatalf("removing %s was accepted", name)
		}
	}
	for _, id := range []string{dm2, agent, self} {
		if _, err := f.ids.Lookup(id); err != nil {
			t.Fatalf("Lookup(%s) after a refused removal: %v", id, err)
		}
	}
	if after := f.head(t); after != before {
		t.Fatalf("the head moved %d -> %d", before, after)
	}
	if !slices.Contains(f.campaign.State().Actors["ag"].GetControllerIds(), gone) {
		t.Fatal("the revoked player's control was revoked by a refused removal")
	}
	select {
	case <-agentConn.closed:
		t.Fatal("a refused removal of the agent closed the agent's connection")
	default:
	}
	if r := f.removeAs(t, f.dmToken, f.playerID); !r.GetOk() {
		t.Fatalf("control: the DM removing a player was refused: %s", r.GetError())
	}
}

// VTT-565 VTT-162
func TestAPlayerAndASpectatorMayNotRemoveAnybody(t *testing.T) {
	f := newGWFixture(t)
	spectator := f.playerIDFor(t, f.spectatorToken)
	before := f.head(t)
	for _, c := range []struct{ token, target string }{
		{f.playerToken, spectator}, {f.spectatorToken, f.playerID},
	} {
		if r := f.removeAs(t, c.token, c.target); r.GetOk() {
			t.Fatalf("a removal of %s was accepted", c.target)
		}
	}
	for _, id := range []string{spectator, f.playerID} {
		if _, err := f.ids.Lookup(id); err != nil {
			t.Fatalf("Lookup(%s): %v", id, err)
		}
	}
	if after := f.head(t); after != before {
		t.Fatalf("the head moved %d -> %d", before, after)
	}
}

// VTT-544
func TestARemovedParticipantsNameIsFreeAtTheShareLink(t *testing.T) {
	f := newGWFixture(t)
	_, kim := f.invite(t, "Kim", identity.RoleSpectator)
	if r := f.removeAs(t, f.dmToken, kim); !r.GetOk() {
		t.Fatalf("the removal was refused: %s", r.GetError())
	}
	secret, err := f.ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.ids.SetJoinOpen(true, 5); err != nil {
		t.Fatal(err)
	}
	if status, body := f.postJoin(t, secret, "kim"); status != http.StatusOK {
		t.Fatalf("join as kim after Kim's removal: %d %q", status, body)
	}
}
