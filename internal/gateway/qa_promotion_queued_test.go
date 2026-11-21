package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const (
	qaQueuedEntries = 1500
	qaQueuedBatch   = 250
)

var qaQueuedSeq atomic.Int64

func qaQueuedStamp(env *vttv1.Envelope) {
	env.ActorRole = "dm"
	env.EventId = fmt.Sprintf("e%d", qaQueuedSeq.Add(1))
}

type qaQueuedWorld struct {
	c      *campaign.Campaign
	url    string
	tokens map[string]string
	ids    map[string]string
}

func qaQueuedAppend(t *testing.T, c *campaign.Campaign, payloads ...*vttv1.Envelope) {
	t.Helper()
	for _, env := range payloads {
		qaQueuedStamp(env)
		if _, err := c.Append(env); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
}

func qaQueuedActor(id string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: id, Name: id},
	}}}
}

func qaQueuedGrant(actor, participant string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: actor, ParticipantId: participant, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}}}
}

func qaQueuedNarrate(i int) *vttv1.Envelope {
	text := strings.Repeat("the room is quiet ", 450)
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{
		Text: text + string(rune('a'+i%26)),
	}}}
}

func qaQueuedNarrateMany(t *testing.T, c *campaign.Campaign, n int) {
	t.Helper()
	for done := 0; done < n; done += qaQueuedBatch {
		var batch []*vttv1.Envelope
		for i := done; i < done+qaQueuedBatch && i < n; i++ {
			env := qaQueuedNarrate(i)
			qaQueuedStamp(env)
			batch = append(batch, env)
		}
		if _, err := c.AppendBatch(batch); err != nil {
			t.Fatalf("append batch: %v", err)
		}
	}
}

func qaQueuedNew(t *testing.T) *qaQueuedWorld {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		ts.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	w := &qaQueuedWorld{c: c, url: "ws" + strings.TrimPrefix(ts.URL, "http"), tokens: map[string]string{}, ids: map[string]string{}}
	for name, role := range map[string]identity.Role{
		"dm": identity.RoleDM, "p1": identity.RolePlayer, "s1": identity.RoleSpectator,
	} {
		tok, id, err := ids.CreateInvite(name, role)
		if err != nil {
			t.Fatal(err)
		}
		w.tokens[name], w.ids[name] = tok, id
	}
	return w
}

type qaQueuedConn struct {
	t *testing.T
	c *websocket.Conn
}

func (w *qaQueuedWorld) dial(t *testing.T, who, log string) *qaQueuedConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	u := w.url + "/ws?token=" + w.tokens[who] + "&after=0"
	if log != "" {
		u += "&log=" + log
	}
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", who, err)
	}
	conn.SetReadLimit(1 << 24)
	t.Cleanup(func() { _ = conn.CloseNow() })
	return &qaQueuedConn{t: t, c: conn}
}

func (q *qaQueuedConn) next(d time.Duration) (*vttv1.ServerFrame, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	_, raw, err := q.c.Read(ctx)
	if err != nil {
		return nil, err
	}
	f := &vttv1.ServerFrame{}
	if err := protojson.Unmarshal(raw, f); err != nil {
		q.t.Fatalf("decode: %v", err)
	}
	return f, nil
}

func (q *qaQueuedConn) mustNext() *vttv1.ServerFrame {
	q.t.Helper()
	f, err := q.next(3 * time.Second)
	if err != nil {
		q.t.Fatalf("expected a frame: %v", err)
	}
	return f
}

func (q *qaQueuedConn) promote(w *qaQueuedWorld, participant, role string) {
	q.t.Helper()
	cmd := &vttv1.ClientCommand{RequestId: "r-promote", Command: &vttv1.ClientCommand_PromoteParticipant{
		PromoteParticipant: &vttv1.PromoteParticipant{ParticipantId: participant, Role: role},
	}}
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		q.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := q.c.Write(ctx, websocket.MessageText, raw); err != nil {
		q.t.Fatal(err)
	}
	for {
		f, err := q.next(10 * time.Second)
		if err != nil {
			q.t.Fatalf("no promotion result: %v", err)
		}
		if r := f.GetResult(); r != nil && r.GetRequestId() == "r-promote" {
			if !r.GetOk() {
				q.t.Fatalf("promotion refused: %s", r.GetError())
			}
			return
		}
	}
}

func qaQueuedPrefixEqual(t *testing.T, got []*vttv1.Envelope, log []campaign.CharacterEntry) {
	t.Helper()
	if len(got) > len(log) {
		t.Fatalf("sent %d entries of a log of %d", len(got), len(log))
	}
	for i, e := range got {
		if !proto.Equal(e, log[i].Env) {
			t.Fatalf("entry %d differs from the log:\n got %v\nwant %v", i, e, log[i].Env)
		}
	}
}

func qaQueuedSetup(t *testing.T) *qaQueuedWorld {
	t.Helper()
	w := qaQueuedNew(t)
	qaQueuedAppend(t, w.c, qaQueuedActor("a1"), qaQueuedGrant("a1", w.ids["p1"]))
	qaQueuedNarrateMany(t, w.c, qaQueuedEntries)
	return w
}

// VTT-923
func TestQAQueuedDemotedPlayerKeepsStreamingItsLog(t *testing.T) {
	w := qaQueuedSetup(t)
	log, _ := w.c.CharacterLog("a1")
	dm := w.dial(t, "dm", "")
	p := w.dial(t, "p1", "a1")
	var got []*vttv1.Envelope
	for len(got) < 5 {
		if f := p.mustNext(); f.GetEvent() != nil {
			got = append(got, f.GetEvent())
		}
	}
	dm.promote(w, w.ids["p1"], "spectator")
	time.Sleep(200 * time.Millisecond)
	for len(got) < len(log) {
		f := p.mustNext()
		if f.GetViewpointChanged() != nil {
			t.Fatalf("switched though a spectator may watch any party member: %v", f)
		}
		if e := f.GetEvent(); e != nil {
			got = append(got, e)
		}
	}
	qaQueuedPrefixEqual(t, got, log)
}
