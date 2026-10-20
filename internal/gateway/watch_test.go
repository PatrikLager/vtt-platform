package gateway_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// watchConn reads one connection's frames in arrival order. Keep it the only
// reader of its conn: frameQueue drops a ViewpointChanged.
type watchConn struct {
	t       *testing.T
	conn    *websocket.Conn
	frames  chan *vttv1.ServerFrame
	closed  chan struct{}
	pending []*vttv1.ServerFrame
	req     int
}

// dialOn is dial opening on log, as a reconnect names it (VTT-339).
func (f *gwFixture) dialOn(token string, after int64, log string) *websocket.Conn {
	f.t.Helper()
	u, err := url.Parse(f.wsURL(token, after))
	if err != nil {
		f.t.Fatal(err)
	}
	if log != "" {
		q := u.Query()
		q.Set("log", log)
		u.RawQuery = q.Encode()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, u.String(), nil)
	if err != nil {
		f.t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(200 * 1024)
	f.t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func (f *gwFixture) watch(token string, after int64, log string) *watchConn {
	f.t.Helper()
	conn := f.dialOn(token, after, log)
	w := &watchConn{t: f.t, conn: conn, frames: make(chan *vttv1.ServerFrame, 1024), closed: make(chan struct{})}
	go func() {
		defer close(w.closed)
		for {
			_, raw, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var fr vttv1.ServerFrame
			if err := protojson.Unmarshal(raw, &fr); err != nil {
				return
			}
			w.frames <- &fr
		}
	}()
	return w
}

// next returns the next frame that is not presence or a set, or nil after d.
func (w *watchConn) next(d time.Duration) *vttv1.ServerFrame {
	w.t.Helper()
	for {
		var fr *vttv1.ServerFrame
		if len(w.pending) > 0 {
			fr, w.pending = w.pending[0], w.pending[1:]
		} else {
			select {
			case fr = <-w.frames:
			case <-w.closed:
				return nil
			case <-time.After(d):
				return nil
			}
		}
		if fr.GetPresenceSnapshot() == nil && fr.GetPresenceChanged() == nil && fr.GetWatchableChanged() == nil {
			return fr
		}
	}
}

func (w *watchConn) must() *vttv1.ServerFrame {
	w.t.Helper()
	fr := w.next(3 * time.Second)
	if fr == nil {
		w.t.Fatal("no frame within 3s")
	}
	return fr
}

// opening reads the head, which must come first, then the ViewpointChanged
// before any entry (VTT-368).
func (w *watchConn) opening() (head int64, viewpoint string) {
	w.t.Helper()
	first := w.must()
	if first.GetCatchUpHead() == nil {
		w.t.Fatalf("the first frame is %v, want the catch-up head", first)
	}
	vc := w.must()
	if vc.GetViewpointChanged() == nil {
		w.t.Fatalf("the frame after the head is %v, want ViewpointChanged", vc)
	}
	return first.GetCatchUpHead().GetHeadSequence(), vc.GetViewpointChanged().GetActorId()
}

// entries reads n entries, failing on a switch or a result in between.
func (w *watchConn) entries(n int) []*vttv1.Envelope {
	w.t.Helper()
	out := make([]*vttv1.Envelope, 0, n)
	for len(out) < n {
		fr := w.must()
		if fr.GetEvent() == nil {
			w.t.Fatalf("entry %d of %d: got %v", len(out)+1, n, fr)
		}
		out = append(out, fr.GetEvent())
	}
	return out
}

// untilViewpoint reads to a ViewpointChanged: its actor, and the entries before it.
func (w *watchConn) untilViewpoint() (string, []*vttv1.Envelope) {
	w.t.Helper()
	var before []*vttv1.Envelope
	for {
		fr := w.must()
		switch {
		case fr.GetViewpointChanged() != nil:
			return fr.GetViewpointChanged().GetActorId(), before
		case fr.GetEvent() != nil:
			before = append(before, fr.GetEvent())
		}
	}
}

func (w *watchConn) quiet(d time.Duration) {
	w.t.Helper()
	if fr := w.next(d); fr != nil {
		w.t.Fatalf("want nothing within %s, got %v", d, fr)
	}
}

// command sends cmd and returns its result, keeping every other frame for next.
func (w *watchConn) command(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	w.t.Helper()
	w.req++
	cmd.RequestId = fmt.Sprintf("watch-%d", w.req)
	sendCommand(w.t, w.conn, cmd)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case fr := <-w.frames:
			if r := fr.GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
				return r
			}
			w.pending = append(w.pending, fr)
		case <-w.closed:
			w.t.Fatal("connection closed before the result")
		case <-deadline:
			w.t.Fatal("no result within 3s")
		}
	}
}

func setViewpoint(actorID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: actorID}}}
}

func moveTo(tokenID string, x, y int32) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: tokenID, To: &vttv1.GridPosition{X: x, Y: y}}}}
}

const unseenSquare = "gateway: cannot move there — you cannot see that square"

// appendTwoRooms appends twoRoomsTiles as scene "rooms".
func appendTwoRooms(t *testing.T, f *gwFixture) {
	t.Helper()
	appendEvents(t, f.campaign, &vttv1.SceneCreated{SceneId: "rooms", Name: "Rooms", GridWidth: 7, GridHeight: 3,
		Tiles: twoRoomsTiles()})
}

func tokenAt(tokenID, actorID string, x, y int32) *vttv1.TokenPlaced {
	return &vttv1.TokenPlaced{TokenId: tokenID, SceneId: "rooms", ActorId: actorID,
		Position: &vttv1.GridPosition{X: x, Y: y}}
}

var watchEventID atomic.Int64

// appendEvents appends payloads as one batch, returning their envelopes.
func appendEvents(t *testing.T, c *campaign.Campaign, payloads ...proto.Message) []*vttv1.Envelope {
	t.Helper()
	envs := make([]*vttv1.Envelope, len(payloads))
	for i, p := range payloads {
		envs[i] = envelope(0, p)
		envs[i].EventId = fmt.Sprintf("watch-evt-%d", watchEventID.Add(1))
	}
	if _, err := c.AppendBatch(envs); err != nil {
		t.Fatalf("append %v: %v", payloads, err)
	}
	return envs
}

func partyMemberOf(actorID, participantID string) []proto.Message {
	return []proto.Message{
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: actorID, Name: actorID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		&vttv1.ActorControlGranted{ActorId: actorID, ParticipantId: participantID,
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
	}
}

func logOf(t *testing.T, f *gwFixture, actorID string) []*vttv1.Envelope {
	t.Helper()
	entries, err := f.campaign.CharacterLog(actorID)
	if err != nil {
		t.Fatalf("CharacterLog(%s): %v", actorID, err)
	}
	out := make([]*vttv1.Envelope, len(entries))
	for i, e := range entries {
		out[i] = e.Env
	}
	return out
}

func requireLog(t *testing.T, who string, got, want []*vttv1.Envelope) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d entries, the log holds %d", who, len(got), len(want))
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Fatalf("%s: entry %d is\n%v\nthe log holds\n%v", who, i+1, got[i], want[i])
		}
	}
}

func participantID(t *testing.T, f *gwFixture, token string) string {
	t.Helper()
	p, err := f.ids.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

// VTT-338 VTT-368
func TestAPlayersConnectionIsItsCharactersLog(t *testing.T) {
	f := newGWFixture(t)
	w := f.watch(f.playerToken, 0, "")
	head, vp := w.opening()
	if vp != "a1" {
		t.Fatalf("the player opens watching %q, want their party member a1", vp)
	}
	want := logOf(t, f, "a1")
	if head != int64(len(want)) {
		t.Fatalf("the head is %d, the log holds %d entries", head, len(want))
	}
	requireLog(t, "catch-up", w.entries(len(want)), want)

	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "the cave drips"})
	appendEvents(t, f.campaign, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "bat", Name: "Bat",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})
	all := logOf(t, f, "a1")
	requireLog(t, "live", w.entries(len(all)-len(want)), all[len(want):])
	w.quiet(300 * time.Millisecond)
}

// VTT-340 VTT-341 VTT-385
func TestAPlayerWatchesTheirFirstPartyMemberByID(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, partyMemberOf("a0", f.playerID)...)

	lera := f.watch(f.playerToken, 0, "")
	if _, vp := lera.opening(); vp != "a0" {
		t.Fatalf("the player opens watching %q, want a0, their first party member by id", vp)
	}
	ivo := f.watch(f.otherPlayerToken, 0, "")
	spec := f.watch(f.spectatorToken, 0, "")
	for name, w := range map[string]*watchConn{"a player with no party member": ivo, "an unperched spectator": spec} {
		if head, vp := w.opening(); head != 0 || vp != "" {
			t.Fatalf("%s opens on %q at head %d, want nobody at 0", name, vp, head)
		}
	}
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "addressed to the table"})
	ivo.quiet(300 * time.Millisecond)
	spec.quiet(300 * time.Millisecond)

	appendEvents(t, f.campaign, partyMemberOf("b1", participantID(t, f, f.otherPlayerToken))...)
	vp, before := ivo.untilViewpoint()
	if vp != "b1" || len(before) != 0 {
		t.Fatalf("granted a first party member, the player is switched to %q after %d entries, want b1 after none", vp, len(before))
	}
	want := logOf(t, f, "b1")
	requireLog(t, "the granted log", ivo.entries(len(want)), want)
}

// VTT-342 VTT-367
func TestAPlayerSwitchesToAnotherOfTheirOwn(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, partyMemberOf("a0", f.playerID)...)
	w := f.watch(f.playerToken, 0, "")
	if _, vp := w.opening(); vp != "a0" {
		t.Fatalf("opens on %q, want a0", vp)
	}
	w.entries(len(logOf(t, f, "a0")))

	if r := w.command(setViewpoint("a1")); !r.GetOk() {
		t.Fatalf("switching to their own a1 was refused: %s", r.GetError())
	}
	vp, before := w.untilViewpoint()
	if vp != "a1" || len(before) != 0 {
		t.Fatalf("switched to %q after %d entries, want a1 after none", vp, len(before))
	}
	want := logOf(t, f, "a1")
	requireLog(t, "a1 from its first entry", w.entries(len(want)), want)

	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "both logs hear this"})
	after := logOf(t, f, "a1")
	requireLog(t, "a1's next entry", w.entries(1), after[len(want):])
	w.quiet(300 * time.Millisecond)
}

// VTT-343 VTT-346 VTT-347
func TestAPlayerMayWatchOnlyTheirOwnPartyMembers(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, partyMemberOf("b1", participantID(t, f, f.otherPlayerToken))...)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: "Gob", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "gob", ParticipantId: f.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	w := f.watch(f.playerToken, 0, "")
	if _, vp := w.opening(); vp != "a1" {
		t.Fatalf("opens on %q, want a1", vp)
	}
	w.entries(len(logOf(t, f, "a1")))

	var refusals []string
	for _, id := range []string{"b1", "gob", "ghost"} {
		r := w.command(setViewpoint(id))
		if r.GetOk() {
			t.Fatalf("a player was allowed to watch %q", id)
		}
		refusals = append(refusals, strings.ReplaceAll(r.GetError(), fmt.Sprintf("%q", id), "ID"))
	}
	if refusals[0] != refusals[1] || refusals[1] != refusals[2] {
		t.Fatalf("the refusals tell another's party member, an NPC and nobody apart: %q", refusals)
	}
	if r := w.command(setViewpoint("")); r.GetOk() {
		t.Fatal("a player was allowed to watch nobody")
	}
	w.quiet(300 * time.Millisecond)
}

// VTT-342
func TestASpectatorWatchesThePartyMemberItPerchesOn(t *testing.T) {
	f := newGWFixture(t)
	w := f.watch(f.spectatorToken, 0, "")
	if _, vp := w.opening(); vp != "" {
		t.Fatalf("an unperched spectator opens on %q", vp)
	}
	if r := w.command(setViewpoint("a1")); !r.GetOk() {
		t.Fatalf("perching on a1 was refused: %s", r.GetError())
	}
	if vp, before := w.untilViewpoint(); vp != "a1" || len(before) != 0 {
		t.Fatalf("perched on %q after %d entries, want a1 after none", vp, len(before))
	}
	want := logOf(t, f, "a1")
	requireLog(t, "the shoulder's log", w.entries(len(want)), want)

	if r := w.command(setViewpoint("")); !r.GetOk() {
		t.Fatalf("leaving the shoulder was refused: %s", r.GetError())
	}
	if vp, before := w.untilViewpoint(); vp != "" || len(before) != 0 {
		t.Fatalf("left for %q after %d entries, want nobody after none", vp, len(before))
	}
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "nobody's to hear"})
	w.quiet(300 * time.Millisecond)
}

// VTT-340 VTT-348 VTT-386
func TestAConnectionStopsWatchingAnActorItMayNoLongerWatch(t *testing.T) {
	losses := map[string]func(f *gwFixture) []proto.Message{
		"control revoked": func(f *gwFixture) []proto.Message {
			return []proto.Message{&vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: f.playerID}}
		},
		"made a non-party actor": func(f *gwFixture) []proto.Message {
			return []proto.Message{&vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: f.playerID,
				Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}
		},
		"removed": func(*gwFixture) []proto.Message {
			return []proto.Message{&vttv1.TokenRemoved{TokenId: "t1"}, &vttv1.ActorRemoved{ActorId: "a1"}}
		},
	}
	for name, loss := range losses {
		t.Run(name, func(t *testing.T) {
			f := newGWFixture(t)
			w := f.watch(f.playerToken, 0, "")
			if _, vp := w.opening(); vp != "a1" {
				t.Fatalf("opens on %q, want a1", vp)
			}
			w.entries(len(logOf(t, f, "a1")))
			lost := appendEvents(t, f.campaign, loss(f)...)
			vp, before := w.untilViewpoint()
			if vp != "" {
				t.Fatalf("after the loss the player watches %q, want nobody", vp)
			}
			for _, e := range before {
				for _, l := range lost {
					if e.GetEventId() == l.GetEventId() {
						t.Fatalf("an entry the loss caused was sent: %v", e)
					}
				}
			}
			appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "after the loss"})
			w.quiet(300 * time.Millisecond)
		})
	}

	t.Run("to the next own party member", func(t *testing.T) {
		f := newGWFixture(t)
		appendEvents(t, f.campaign, partyMemberOf("a2", f.playerID)...)
		w := f.watch(f.playerToken, 0, "")
		if _, vp := w.opening(); vp != "a1" {
			t.Fatalf("opens on %q, want a1", vp)
		}
		w.entries(len(logOf(t, f, "a1")))
		appendEvents(t, f.campaign, &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: f.playerID})
		if vp, _ := w.untilViewpoint(); vp != "a2" {
			t.Fatalf("after losing a1 the player watches %q, want a2", vp)
		}
		want := logOf(t, f, "a2")
		requireLog(t, "a2 from its first entry", w.entries(len(want)), want)
	})

	t.Run("a spectator's shoulder leaves the party", func(t *testing.T) {
		f := newGWFixture(t)
		w := f.watch(f.spectatorToken, 0, "")
		w.opening()
		if r := w.command(setViewpoint("a1")); !r.GetOk() {
			t.Fatalf("perching on a1 was refused: %s", r.GetError())
		}
		w.untilViewpoint()
		w.entries(len(logOf(t, f, "a1")))
		appendEvents(t, f.campaign, &vttv1.ActorControlGranted{ActorId: "a1", ParticipantId: f.playerID,
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
		if vp, _ := w.untilViewpoint(); vp != "" {
			t.Fatalf("the shoulder left the party and the spectator watches %q, want nobody", vp)
		}
	})
}

// VTT-339 VTT-349
func TestACursorIsHonouredOnlyInTheLogItWasReadFrom(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, partyMemberOf("a0", f.playerID)...)
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "one"})
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "two"})
	a1 := logOf(t, f, "a1")
	n := int64(len(a1) - 2)

	w := f.watch(f.playerToken, n, "a1")
	head, vp := w.opening()
	if vp != "a1" || head != int64(len(a1)) {
		t.Fatalf("a reconnect naming a1 opened on %q at head %d, want a1 at %d", vp, head, len(a1))
	}
	requireLog(t, "a1 after the cursor", w.entries(2), a1[n:])

	a0 := logOf(t, f, "a0")
	other := f.watch(f.playerToken, n, "")
	if _, vp := other.opening(); vp != "a0" {
		t.Fatalf("a reconnect naming no log opened on %q, want the default a0", vp)
	}
	requireLog(t, "the default from its first entry", other.entries(len(a0)), a0)

	appendEvents(t, f.campaign, &vttv1.ActorControlRevoked{ActorId: "a1", ParticipantId: f.playerID})
	a0 = logOf(t, f, "a0")
	gone := f.watch(f.playerToken, n, "a1")
	if _, vp := gone.opening(); vp != "a0" {
		t.Fatalf("a reconnect naming a log no longer theirs opened on %q, want the default a0", vp)
	}
	requireLog(t, "the default from its first entry", gone.entries(len(a0)), a0)
}

// VTT-340 VTT-369 VTT-339
func TestALogNamingAnActorItMayNotWatchOpensOnTheDefault(t *testing.T) {
	f := newGWFixture(t)
	appendEvents(t, f.campaign, partyMemberOf("b1", participantID(t, f, f.otherPlayerToken))...)
	appendEvents(t, f.campaign, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "gob", Name: "Gob",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}})

	w := f.watch(f.playerToken, 3, "b1")
	if _, vp := w.opening(); vp != "a1" {
		t.Fatalf("naming another player's party member opened on %q, want the default a1", vp)
	}
	a1 := logOf(t, f, "a1")
	requireLog(t, "the default from its first entry", w.entries(len(a1)), a1)

	s := f.watch(f.spectatorToken, 3, "gob")
	if head, vp := s.opening(); vp != "" || head != 0 {
		t.Fatalf("a spectator naming an NPC opened on %q at %d, want nobody at 0", vp, head)
	}
	perched := f.watch(f.spectatorToken, 0, "a1")
	if _, vp := perched.opening(); vp != "a1" {
		t.Fatalf("a spectator naming a party member opened on %q, want a1", vp)
	}
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "only a1 hears"})
	s.quiet(300 * time.Millisecond)
}

// VTT-193
func TestARefusedViewpointLeavesTheConnectionWhereItWas(t *testing.T) {
	f := newGWFixture(t)
	w := f.watch(f.playerToken, 0, "")
	w.opening()
	before := logOf(t, f, "a1")
	w.entries(len(before))
	if r := w.command(setViewpoint("ghost")); r.GetOk() {
		t.Fatal("watching an absent actor was accepted")
	}
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "still here"})
	after := logOf(t, f, "a1")
	requireLog(t, "a1's next entry", w.entries(1), after[len(before):])
}

func TestAClosedCampaignClosesAWatchersConnection(t *testing.T) {
	f := newGWFixture(t)
	w := f.watch(f.spectatorToken, 0, "")
	w.opening()
	if err := f.campaign.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.closed:
	case <-time.After(3 * time.Second):
		t.Fatal("a connection watching nobody outlived the campaign")
	}
}

// VTT-340
func TestAPromotedSpectatorWatchesTheirFirstPartyMember(t *testing.T) {
	f := newGWFixture(t)
	w := f.watch(f.spectatorToken, 0, "")
	if _, vp := w.opening(); vp != "" {
		t.Fatalf("an unperched spectator opens on %q", vp)
	}
	id := participantID(t, f, f.spectatorToken)
	if err := f.ids.SetRole(id, identity.RolePlayer); err != nil {
		t.Fatal(err)
	}
	appendEvents(t, f.campaign, partyMemberOf("c1", id)...)
	if vp, before := w.untilViewpoint(); vp != "c1" || len(before) != 0 {
		t.Fatalf("promoted and granted a party member, the connection watches %q after %d entries, want c1", vp, len(before))
	}
	want := logOf(t, f, "c1")
	requireLog(t, "the granted log", w.entries(len(want)), want)
}

// VTT-350
func TestAPlayersMoveIsJudgedByTheCharacterTheyWatch(t *testing.T) {
	f := newGWFixture(t)
	ivo := participantID(t, f, f.otherPlayerToken)
	appendTwoRooms(t, f)
	appendEvents(t, f.campaign, partyMemberOf("ana", ivo)...)
	appendEvents(t, f.campaign, partyMemberOf("bo", ivo)...)
	appendEvents(t, f.campaign, tokenAt("tok-ana", "ana", 1, 1), tokenAt("tok-bo", "bo", 5, 1))

	w := f.watch(f.otherPlayerToken, 0, "")
	if _, vp := w.opening(); vp != "ana" {
		t.Fatalf("opens on %q, want ana", vp)
	}
	if r := w.command(moveTo("tok-bo", 4, 1)); r.GetOk() || r.GetError() != unseenSquare {
		t.Fatalf("watching ana, a move onto a square only bo sees: ok=%v %q, want %q",
			r.GetOk(), r.GetError(), unseenSquare)
	}
	if r := w.command(setViewpoint("bo")); !r.GetOk() {
		t.Fatalf("switching to their own bo was refused: %s", r.GetError())
	}
	if vp, _ := w.untilViewpoint(); vp != "bo" {
		t.Fatalf("switched to %q, want bo", vp)
	}
	if r := w.command(moveTo("tok-bo", 4, 1)); !r.GetOk() {
		t.Fatalf("watching bo, the same move was refused: %s", r.GetError())
	}
}

// VTT-370
func TestAPlayerWithNoPartyMemberMovesNothing(t *testing.T) {
	f := newGWFixture(t)
	ivo := participantID(t, f, f.otherPlayerToken)
	appendTwoRooms(t, f)
	appendEvents(t, f.campaign,
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wolf", Name: "Wolf", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "wolf", ParticipantId: ivo, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		tokenAt("tok-wolf", "wolf", 1, 1))

	w := f.watch(f.otherPlayerToken, 0, "")
	if _, vp := w.opening(); vp != "" {
		t.Fatalf("opens on %q, want nobody", vp)
	}
	for _, to := range [][2]int32{{2, 1}, {1, 0}, {4, 1}} {
		if r := w.command(moveTo("tok-wolf", to[0], to[1])); r.GetOk() || r.GetError() != unseenSquare {
			t.Fatalf("the wolf's controller moved it to %v: ok=%v %q, want %q", to, r.GetOk(), r.GetError(), unseenSquare)
		}
	}
}

func promote(t *testing.T, dm *watchConn, participantID string, role identity.Role) {
	t.Helper()
	r := dm.command(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_PromoteParticipant{
		PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: participantID, Role: string(role)}}})
	if !r.GetOk() {
		t.Fatalf("the promotion to %s was refused: %s", role, r.GetError())
	}
}

// VTT-340
func TestAPromotionMovesASpectatorOffAShoulderThePlayerMayNotWatch(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	spec := f.watch(f.spectatorToken, 0, "")
	if _, vp := spec.opening(); vp != "" {
		t.Fatalf("an unperched spectator opens on %q", vp)
	}
	if r := spec.command(setViewpoint("a1")); !r.GetOk() {
		t.Fatalf("perching on a1 was refused: %s", r.GetError())
	}
	if vp, _ := spec.untilViewpoint(); vp != "a1" {
		t.Fatalf("perched on %q, want a1", vp)
	}
	spec.entries(len(logOf(t, f, "a1")))

	promote(t, dm, participantID(t, f, f.spectatorToken), identity.RolePlayer)
	if vp, before := spec.untilViewpoint(); vp != "" || len(before) != 0 {
		t.Fatalf("promoted with nothing appended, the connection moved to %q after %d entries, want nobody after none",
			vp, len(before))
	}
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "a1 hears this"})
	spec.quiet(300 * time.Millisecond)
}

// VTT-340
func TestAPromotedSpectatorMovesOntoThePartyMemberTheyHold(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	id := participantID(t, f, f.spectatorToken)
	appendEvents(t, f.campaign, partyMemberOf("c1", id)...)
	spec := f.watch(f.spectatorToken, 0, "")
	if head, vp := spec.opening(); head != 0 || vp != "" {
		t.Fatalf("a spectator who holds c1 opens on %q at head %d, want nobody at 0", vp, head)
	}

	promote(t, dm, id, identity.RolePlayer)
	if vp, before := spec.untilViewpoint(); vp != "c1" || len(before) != 0 {
		t.Fatalf("promoted with nothing appended, the connection moved to %q after %d entries, want c1 after none",
			vp, len(before))
	}
	want := logOf(t, f, "c1")
	requireLog(t, "c1 from its first entry", spec.entries(len(want)), want)
}

// VTT-340
func TestAPromotionMovesEveryConnectionOfTheParticipant(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	devices := []*watchConn{f.watch(f.spectatorToken, 0, "a1"), f.watch(f.spectatorToken, 0, "a1")}
	for i, w := range devices {
		if _, vp := w.opening(); vp != "a1" {
			t.Fatalf("device %d opens on %q, want a1", i+1, vp)
		}
		w.entries(len(logOf(t, f, "a1")))
	}

	promote(t, dm, participantID(t, f, f.spectatorToken), identity.RolePlayer)
	for i, w := range devices {
		if vp, before := w.untilViewpoint(); vp != "" || len(before) != 0 {
			t.Fatalf("device %d moved to %q after %d entries, want nobody after none", i+1, vp, len(before))
		}
	}
}

// VTT-388
func TestAPlayerMadeASpectatorKeepsWatchingTheirPartyMember(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	w := f.watch(f.playerToken, 0, "")
	if _, vp := w.opening(); vp != "a1" {
		t.Fatalf("opens on %q, want a1", vp)
	}
	before := logOf(t, f, "a1")
	w.entries(len(before))

	promote(t, dm, f.playerID, identity.RoleSpectator)
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "a1 still hears this"})
	requireLog(t, "a1's next entry", w.entries(1), logOf(t, f, "a1")[len(before):])
	w.quiet(300 * time.Millisecond)
}

// VTT-388
func TestAPromotedSpectatorKeepsWatchingAPartyMemberTheyHold(t *testing.T) {
	f := newGWFixture(t)
	dm := f.watch(f.dmToken, 0, "")
	id := participantID(t, f, f.spectatorToken)
	appendEvents(t, f.campaign, partyMemberOf("c1", id)...)
	appendEvents(t, f.campaign, partyMemberOf("c2", id)...)
	w := f.watch(f.spectatorToken, 0, "c2")
	if _, vp := w.opening(); vp != "c2" {
		t.Fatalf("opens on %q, want c2", vp)
	}
	before := logOf(t, f, "c2")
	w.entries(len(before))

	promote(t, dm, id, identity.RolePlayer)
	appendEvents(t, f.campaign, &vttv1.NarrationAdded{Text: "c2 still hears this"})
	requireLog(t, "c2's next entry", w.entries(1), logOf(t, f, "c2")[len(before):])
	w.quiet(300 * time.Millisecond)
}
