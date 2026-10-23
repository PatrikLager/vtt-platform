package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaOpeningFrameWait = 3 * time.Second
	qaOpeningQuiet     = 300 * time.Millisecond
	qaOpeningRaceRuns  = 40
	qaOpeningEarly     = "the room is quiet"
	qaOpeningLate      = "the hall is still"
)

type qaOpeningClient struct {
	t      *testing.T
	conn   *websocket.Conn
	frames chan *vttv1.ServerFrame
	reqs   int
}

type qaOpeningWorld struct {
	t     *testing.T
	camp  *campaign.Campaign
	ids   *identity.DB
	ts    *httptest.Server
	ctx   context.Context
	dm    *qaOpeningClient
	token map[string]string
}

func qaOpeningNewWorld(t *testing.T) *qaOpeningWorld {
	t.Helper()
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = camp.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	ts := httptest.NewServer(gateway.New(camp, ids).Handler())
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	w := &qaOpeningWorld{t: t, camp: camp, ids: ids, ts: ts, ctx: ctx, token: map[string]string{}}
	w.invite("dm", "dm", identity.RoleDM)
	w.dm = w.dial("dm", "")
	return w
}

func (w *qaOpeningWorld) invite(id, name string, role identity.Role) {
	w.t.Helper()
	tok, pid, err := w.ids.CreateInvite(name, role)
	if err != nil {
		w.t.Fatalf("CreateInvite %s: %v", name, err)
	}
	w.token[id] = tok
	w.token["id:"+id] = pid
}

func (w *qaOpeningWorld) pid(id string) string { return w.token["id:"+id] }

func (w *qaOpeningWorld) dial(who, log string) *qaOpeningClient {
	w.t.Helper()
	url := "ws" + strings.TrimPrefix(w.ts.URL, "http") + "/ws?after=0&token=" + w.token[who]
	if log != "" {
		url += "&log=" + log
	}
	dctx, cancel := context.WithTimeout(w.ctx, qaOpeningFrameWait)
	defer cancel()
	conn, _, err := websocket.Dial(dctx, url, nil)
	if err != nil {
		w.t.Fatalf("dial %s: %v", who, err)
	}
	conn.SetReadLimit(1 << 22)
	c := &qaOpeningClient{t: w.t, conn: conn, frames: make(chan *vttv1.ServerFrame, 4096)}
	w.t.Cleanup(func() { _ = conn.CloseNow() })
	go func() {
		defer close(c.frames)
		for {
			_, data, err := conn.Read(w.ctx)
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if err := protojson.Unmarshal(data, f); err != nil {
				return
			}
			c.frames <- f
		}
	}()
	return c
}

func (c *qaOpeningClient) next() (*vttv1.ServerFrame, bool) {
	select {
	case f, ok := <-c.frames:
		return f, ok
	case <-time.After(qaOpeningFrameWait):
		return nil, false
	}
}

func (c *qaOpeningClient) send(ctx context.Context, cmd *vttv1.ClientCommand) error {
	c.reqs++
	cmd.RequestId = fmt.Sprintf("r%d", c.reqs)
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		return err
	}
	if err := c.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		return err
	}
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				return fmt.Errorf("connection closed before result")
			}
			if r := f.GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
				if !r.GetOk() {
					return fmt.Errorf("refused: %s", r.GetError())
				}
				return nil
			}
		case <-time.After(qaOpeningFrameWait):
			return fmt.Errorf("no result for %s", cmd.RequestId)
		}
	}
}

func (w *qaOpeningWorld) must(cmd *vttv1.ClientCommand) {
	w.t.Helper()
	if err := w.dm.send(w.ctx, cmd); err != nil {
		w.t.Fatalf("setup command: %v", err)
	}
}

func (w *qaOpeningWorld) partyMember(id, controller string) {
	w.t.Helper()
	w.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: id, Name: id, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
	}}})
	if controller != "" {
		w.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: id, ParticipantId: w.pid(controller), Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}})
	}
}

func (w *qaOpeningWorld) narrate(text string) {
	w.t.Helper()
	w.must(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddNarration{AddNarration: &vttv1.AddNarration{Text: text}}})
}

func (w *qaOpeningWorld) promote(who, role string) error {
	return w.dm.send(w.ctx, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PromoteParticipant{
		PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: w.pid(who), Role: role},
	}})
}

func (w *qaOpeningWorld) worldHead() int64 {
	w.t.Helper()
	_, stop, head, err := w.camp.Subscribe(0, 1)
	if err != nil {
		w.t.Fatalf("Subscribe: %v", err)
	}
	stop()
	return head
}

type qaOpeningWatch struct {
	t       *testing.T
	c       *qaOpeningClient
	gotHead bool
	sawVC   bool
	cur     string
	vcs     []string
	sinceVC []*vttv1.Envelope
}

func (s *qaOpeningWatch) feed(f *vttv1.ServerFrame) {
	s.t.Helper()
	switch {
	case f.GetCatchUpHead() != nil:
		if s.gotHead || s.sawVC {
			s.t.Fatalf("catch-up head was not the first frame, or came twice")
		}
		s.gotHead = true
	case f.GetViewpointChanged() != nil:
		if !s.gotHead {
			s.t.Fatalf("ViewpointChanged before the catch-up head")
		}
		s.sawVC = true
		s.cur = f.GetViewpointChanged().GetActorId()
		s.vcs = append(s.vcs, s.cur)
		s.sinceVC = nil
	case f.GetEvent() != nil:
		e := f.GetEvent()
		if !s.sawVC {
			s.t.Fatalf("entry %d sent before any ViewpointChanged", e.GetSequence())
		}
		if s.cur == "" {
			s.t.Fatalf("entry %d sent while the connection watches nobody; viewpoints so far %q", e.GetSequence(), s.vcs)
		}
		s.sinceVC = append(s.sinceVC, e)
		if want := int64(len(s.sinceVC)); e.GetSequence() != want {
			s.t.Fatalf("after ViewpointChanged %q entry %d arrived where %d was due: the log is sent from its first entry", s.cur, e.GetSequence(), want)
		}
	}
}

func (s *qaOpeningWatch) until(done func() bool, what string) {
	s.t.Helper()
	for !done() {
		f, ok := s.c.next()
		if !ok {
			s.t.Fatalf("connection never %s; viewpoints so far %q, entries since the last %d", what, s.vcs, len(s.sinceVC))
		}
		s.feed(f)
	}
}

func (s *qaOpeningWatch) quiet() {
	s.t.Helper()
	deadline := time.After(qaOpeningQuiet)
	for {
		select {
		case f, ok := <-s.c.frames:
			if !ok {
				return
			}
			if f.GetPresenceChanged() != nil || f.GetPresenceSnapshot() != nil {
				continue
			}
			if f.GetViewpointChanged() != nil {
				s.t.Fatalf("a second ViewpointChanged %q arrived after the connection settled; so far %q", f.GetViewpointChanged().GetActorId(), s.vcs)
			}
			s.feed(f)
		case <-deadline:
			return
		}
	}
}

type qaOpeningMode int

const (
	qaOpeningBefore qaOpeningMode = iota
	qaOpeningSettled
	qaOpeningRace
)

func qaOpeningLaunch(w *qaOpeningWorld, mode qaOpeningMode, delay time.Duration, who, role, log string) (*qaOpeningWatch, func() error) {
	w.t.Helper()
	promoteErr := make(chan error, 1)
	run := func() error { return w.promote(who, role) }
	var watch *qaOpeningWatch
	switch mode {
	case qaOpeningBefore:
		if err := run(); err != nil {
			w.t.Fatalf("promote: %v", err)
		}
		promoteErr <- nil
		watch = &qaOpeningWatch{t: w.t, c: w.dial(who, log)}
	case qaOpeningSettled:
		watch = &qaOpeningWatch{t: w.t, c: w.dial(who, log)}
		watch.until(func() bool { return watch.sawVC }, "opened")
		if err := run(); err != nil {
			w.t.Fatalf("promote: %v", err)
		}
		promoteErr <- nil
	default:
		promoteDelay, dialDelay := delay, time.Duration(0)
		if delay < 0 {
			promoteDelay, dialDelay = 0, -delay
		}
		go func() {
			time.Sleep(promoteDelay)
			promoteErr <- run()
		}()
		time.Sleep(dialDelay)
		watch = &qaOpeningWatch{t: w.t, c: w.dial(who, log)}
	}
	return watch, func() error { return <-promoteErr }
}

func qaOpeningCases() []struct {
	name  string
	mode  qaOpeningMode
	delay time.Duration
} {
	cases := []struct {
		name  string
		mode  qaOpeningMode
		delay time.Duration
	}{
		{"before the dial", qaOpeningBefore, 0},
		{"after it settled", qaOpeningSettled, 0},
	}
	for i := 0; i < qaOpeningRaceRuns; i++ {
		d := time.Duration(i/2) * 100 * time.Microsecond
		if i%2 == 1 {
			d = -d
		}
		cases = append(cases, struct {
			name  string
			mode  qaOpeningMode
			delay time.Duration
		}{fmt.Sprintf("race %d", i), qaOpeningRace, d})
	}
	return cases
}

// VTT-340 VTT-341 VTT-342 VTT-367 VTT-036
func TestQAOpeningAPromotedSpectatorOnAnothersPartyMemberEndsWatchingTheirOwn(t *testing.T) {
	for _, tc := range qaOpeningCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := qaOpeningNewWorld(t)
			w.invite("p1", "p1", identity.RolePlayer)
			w.invite("s1", "s1", identity.RoleSpectator)
			w.partyMember("a1", "p1")
			w.partyMember("c1", "s1")
			w.narrate(qaOpeningEarly)
			w.narrate(qaOpeningEarly)
			before := w.worldHead()

			watch, promoted := qaOpeningLaunch(w, tc.mode, tc.delay, "s1", "player", "a1")
			if err := promoted(); err != nil {
				t.Fatalf("promote: %v", err)
			}
			if got := w.worldHead(); got != before {
				t.Fatalf("a promotion appended: world head %d, was %d", got, before)
			}
			own, err := w.camp.CharacterLog("c1")
			if err != nil {
				t.Fatal(err)
			}
			watch.until(func() bool { return watch.sawVC && watch.cur == "c1" && len(watch.sinceVC) >= len(own) },
				"came to watch c1 from its first entry")
			w.narrate(qaOpeningLate)
			watch.until(func() bool { return len(watch.sinceVC) > len(own) }, "was sent c1's next entry")
			watch.quiet()
			t.Logf("viewpoints %q", watch.vcs)
			want, err := w.camp.CharacterLog("c1")
			if err != nil || len(watch.sinceVC) != len(want) {
				t.Fatalf("c1's log holds %d entries (err %v), %d were sent after the last ViewpointChanged",
					len(want), err, len(watch.sinceVC))
			}
			for i, e := range watch.sinceVC {
				if !proto.Equal(e, want[i].Env) {
					t.Fatalf("entry %d is %v, c1's log holds %v there", i+1, e, want[i].Env)
				}
			}
			if last := watch.vcs[len(watch.vcs)-1]; last != "c1" {
				t.Fatalf("last ViewpointChanged names %q, want c1", last)
			}
		})
	}
}

// VTT-340 VTT-341 VTT-342 VTT-036
func TestQAOpeningAPromotedSpectatorHoldingAPartyMemberWatchesItFromItsFirstEntry(t *testing.T) {
	for _, tc := range qaOpeningCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := qaOpeningNewWorld(t)
			w.invite("s1", "s1", identity.RoleSpectator)
			w.partyMember("c1", "s1")
			w.narrate(qaOpeningEarly)
			w.narrate(qaOpeningEarly)
			want, err := w.camp.CharacterLog("c1")
			if err != nil || len(want) < 2 {
				t.Fatalf("setup: character log of c1 holds %d entries, err %v", len(want), err)
			}
			before := w.worldHead()

			watch, promoted := qaOpeningLaunch(w, tc.mode, tc.delay, "s1", "player", "")
			if err := promoted(); err != nil {
				t.Fatalf("promote: %v", err)
			}
			if got := w.worldHead(); got != before {
				t.Fatalf("a promotion appended: world head %d, was %d", got, before)
			}
			watch.until(func() bool { return watch.sawVC && watch.cur == "c1" && len(watch.sinceVC) >= len(want) }, "came to watch c1 from its first entry")
			watch.quiet()
			t.Logf("viewpoints %q", watch.vcs)
			if len(watch.sinceVC) != len(want) {
				t.Fatalf("c1's log has %d entries, %d were sent after the last ViewpointChanged", len(want), len(watch.sinceVC))
			}
			for i, e := range watch.sinceVC {
				if !proto.Equal(e, want[i].Env) {
					t.Fatalf("entry %d is %v, c1's log holds %v there", i+1, e, want[i].Env)
				}
			}
			if last := watch.vcs[len(watch.vcs)-1]; last != "c1" {
				t.Fatalf("last ViewpointChanged names %q, want c1", last)
			}
		})
	}
}
