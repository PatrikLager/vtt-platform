package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const qaNRWait = 10 * time.Second

type qaNRTable struct {
	t   *testing.T
	c   *campaign.Campaign
	ids *identity.DB
	url string
}

type qaNRSeat struct {
	t      *testing.T
	conn   *websocket.Conn
	frames chan *vttv1.ServerFrame
	seen   []*vttv1.ServerFrame
	next   int
}

func qaNRNewTable(t *testing.T) *qaNRTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = c.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	ts := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(func() {
		ts.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qaNRTable{t: t, c: c, ids: ids, url: ts.URL}
}

func (tb *qaNRTable) seat(name string, role identity.Role) *qaNRSeat {
	tb.t.Helper()
	tok, _, err := tb.ids.CreateInvite(name, role)
	if err != nil {
		tb.t.Fatalf("CreateInvite(%s): %v", role, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	u := "ws" + strings.TrimPrefix(tb.url, "http") + "/ws?token=" + url.QueryEscape(tok)
	dialCtx, dialCancel := context.WithTimeout(ctx, qaNRWait)
	conn, _, err := websocket.Dial(dialCtx, u, nil)
	dialCancel()
	if err != nil {
		cancel()
		tb.t.Fatalf("dial as %s: %v", role, err)
	}
	conn.SetReadLimit(1 << 22)
	s := &qaNRSeat{t: tb.t, conn: conn, frames: make(chan *vttv1.ServerFrame, 4096)}
	go func() {
		defer close(s.frames)
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if err := protojson.Unmarshal(data, f); err != nil {
				return
			}
			s.frames <- f
		}
	}()
	tb.t.Cleanup(func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		cancel()
	})
	return s
}

func (tb *qaNRTable) head() int64 {
	tb.t.Helper()
	_, unsubscribe, head, err := tb.c.Subscribe(0, 4096)
	if err != nil {
		tb.t.Fatalf("Subscribe: %v", err)
	}
	unsubscribe()
	return head
}

func (s *qaNRSeat) pull() *vttv1.ServerFrame {
	s.t.Helper()
	if s.next < len(s.seen) {
		f := s.seen[s.next]
		s.next++
		return f
	}
	select {
	case f, ok := <-s.frames:
		if !ok {
			s.t.Fatalf("connection closed while waiting for a frame")
		}
		s.seen = append(s.seen, f)
		s.next++
		return f
	case <-time.After(qaNRWait):
		s.t.Fatalf("no frame within %s", qaNRWait)
	}
	return nil
}

func (s *qaNRSeat) sendRaw(raw string) {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), qaNRWait)
	defer cancel()
	if err := s.conn.Write(ctx, websocket.MessageText, []byte(raw)); err != nil {
		s.t.Fatalf("write: %v", err)
	}
}

func (s *qaNRSeat) result(reqID string) *vttv1.CommandResult {
	s.t.Helper()
	for i := 0; i < len(s.seen); i++ {
		if r := s.seen[i].GetResult(); r != nil && r.GetRequestId() == reqID {
			return r
		}
	}
	for {
		f := s.pull()
		if r := f.GetResult(); r != nil && r.GetRequestId() == reqID {
			return r
		}
	}
}

func (s *qaNRSeat) upsert(reqID, key, text string, vis vttv1.NoteVisibility) *vttv1.CommandResult {
	s.t.Helper()
	cmd := &vttv1.ClientCommand{
		RequestId: reqID,
		Command: &vttv1.ClientCommand_UpsertNote{UpsertNote: &vttv1.UpsertNote{
			Key: key, Title: "title of " + key, Text: text, Visibility: vis,
		}},
	}
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		s.t.Fatalf("marshal: %v", err)
	}
	s.sendRaw(string(raw))
	return s.result(reqID)
}

func (s *qaNRSeat) noteEventsUntil(key string) []*vttv1.NoteUpserted {
	s.t.Helper()
	var got []*vttv1.NoteUpserted
	s.next = 0
	for {
		f := s.pull()
		if n := f.GetEvent().GetNoteUpserted(); n != nil {
			got = append(got, n)
			if n.GetKey() == key {
				return got
			}
		}
	}
}

func qaNROfferedVisibilities() []vttv1.NoteVisibility {
	var out []vttv1.NoteVisibility
	for n := range vttv1.NoteVisibility_name {
		if n != int32(vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED) {
			out = append(out, vttv1.NoteVisibility(n))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func qaNRAssertVisibilityRefusal(t *testing.T, label string, r *vttv1.CommandResult) {
	t.Helper()
	if r.GetOk() {
		t.Fatalf("%s: ok=true, want a refusal", label)
	}
	t.Logf("%s: refused with %q", label, r.GetError())
	if !strings.Contains(strings.ToLower(r.GetError()), "visibility") {
		t.Errorf("%s: refusal %q does not name visibility", label, r.GetError())
	}
	if strings.Contains(r.GetError(), gateway.ErrUnauthorized.Error()) {
		t.Errorf("%s: refusal %q reads as an authorization refusal, want a refusal of the command's form", label, r.GetError())
	}
}

// VTT-231
func TestQAUpsertNoteStatingNoVisibilityIsRefusedForTheDMAndTheAgent(t *testing.T) {
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		t.Run(string(role), func(t *testing.T) {
			tb := qaNRNewTable(t)
			issuer := tb.seat("issuer", role)
			observer := tb.seat("observer", identity.RoleDM)
			before := tb.head()

			omitted := issuer.upsert("omitted", "refused-note", "should not land", vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED)
			qaNRAssertVisibilityRefusal(t, "visibility omitted", omitted)

			issuer.sendRaw(`{"requestId":"named-zero","upsertNote":{"key":"refused-note","title":"t","text":"x","visibility":"NOTE_VISIBILITY_UNSPECIFIED"}}`)
			qaNRAssertVisibilityRefusal(t, "visibility named UNSPECIFIED", issuer.result("named-zero"))

			issuer.sendRaw(`{"requestId":"numeric-zero","upsertNote":{"key":"refused-note","title":"t","text":"x","visibility":0}}`)
			qaNRAssertVisibilityRefusal(t, "visibility given as 0", issuer.result("numeric-zero"))

			if after := tb.head(); after != before {
				t.Fatalf("log head moved from %d to %d across three refused upserts", before, after)
			}
			if _, ok := tb.c.State().Notes["refused-note"]; ok {
				t.Fatalf("the fold holds refused-note after every upsert of it was refused")
			}

			sentinel := issuer.upsert("sentinel", "sentinel-note", "after the refusals", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC)
			if !sentinel.GetOk() {
				t.Fatalf("the connection could not issue a command after the refusals: %q", sentinel.GetError())
			}
			for _, n := range observer.noteEventsUntil("sentinel-note") {
				if n.GetKey() == "refused-note" {
					t.Fatalf("another connection was sent a NoteUpserted for a refused upsert: %v", n)
				}
			}
		})
	}
}

// VTT-231
func TestQAAnUpsertWithoutVisibilityLeavesAnExistingNoteAsItWas(t *testing.T) {
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		t.Run(string(role), func(t *testing.T) {
			tb := qaNRNewTable(t)
			issuer := tb.seat("issuer", role)
			first := issuer.upsert("first", "kept", "original text", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET)
			if !first.GetOk() {
				t.Fatalf("setup upsert refused: %q", first.GetError())
			}
			before := tb.head()
			qaNRAssertVisibilityRefusal(t, "re-upsert without visibility",
				issuer.upsert("again", "kept", "replacement text", vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED))
			if after := tb.head(); after != before {
				t.Fatalf("log head moved from %d to %d on a refused re-upsert", before, after)
			}
			note := tb.c.State().Notes["kept"]
			if note.Text != "original text" || note.Visibility != vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET {
				t.Fatalf("existing note changed by a refused upsert: %+v", note)
			}
		})
	}
}

// VTT-231
func TestQAUpsertNoteStatingAnyOfferedVisibilityIsAccepted(t *testing.T) {
	offered := qaNROfferedVisibilities()
	if len(offered) < 2 {
		t.Fatalf("the contract offers %d note visibilities, want at least public and DM-only", len(offered))
	}
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		for _, vis := range offered {
			t.Run(fmt.Sprintf("%s/%s", role, vis), func(t *testing.T) {
				tb := qaNRNewTable(t)
				issuer := tb.seat("issuer", role)
				before := tb.head()
				r := issuer.upsert("accepted", "stated", "a stated visibility", vis)
				if !r.GetOk() {
					t.Fatalf("upsert_note with %s refused: %q", vis, r.GetError())
				}
				if r.GetSequence() != before+1 {
					t.Errorf("result sequence %d, want %d (the next sequence after the head)", r.GetSequence(), before+1)
				}
				if after := tb.head(); after != before+1 {
					t.Fatalf("log head %d after an accepted upsert, want %d", after, before+1)
				}
				note, ok := tb.c.State().Notes["stated"]
				if !ok {
					t.Fatalf("accepted note is not in the fold")
				}
				if note.Visibility != vis {
					t.Errorf("fold records visibility %s, want %s", note.Visibility, vis)
				}
			})
		}
	}
}

// VTT-231
func TestQAUpsertNoteVisibilityByNameOnTheWireIsAccepted(t *testing.T) {
	for _, vis := range qaNROfferedVisibilities() {
		t.Run(vis.String(), func(t *testing.T) {
			tb := qaNRNewTable(t)
			issuer := tb.seat("issuer", identity.RoleAgent)
			issuer.sendRaw(fmt.Sprintf(`{"requestId":"by-name","upsertNote":{"key":"named","text":"x","visibility":%q}}`, vis.String()))
			if r := issuer.result("by-name"); !r.GetOk() {
				t.Fatalf("upsert_note naming %s refused: %q", vis, r.GetError())
			}
			if got := tb.c.State().Notes["named"].Visibility; got != vis {
				t.Fatalf("fold records %s, want %s", got, vis)
			}
		})
	}
}

// SPEC-013, The three validators.
func TestQAAPlayersUpsertWithoutVisibilityIsRefusedByAuthorizationFirst(t *testing.T) {
	for _, role := range []identity.Role{identity.RolePlayer, identity.RoleSpectator} {
		t.Run(string(role), func(t *testing.T) {
			tb := qaNRNewTable(t)
			issuer := tb.seat("issuer", role)
			before := tb.head()
			r := issuer.upsert("unauth", "k", "x", vttv1.NoteVisibility_NOTE_VISIBILITY_UNSPECIFIED)
			if r.GetOk() {
				t.Fatalf("a %s's upsert_note was accepted", role)
			}
			if !strings.Contains(r.GetError(), gateway.ErrUnauthorized.Error()) {
				t.Errorf("refusal %q is not the authorization refusal; the validator must run after Authorize", r.GetError())
			}
			if after := tb.head(); after != before {
				t.Fatalf("log head moved from %d to %d", before, after)
			}
		})
	}
}
