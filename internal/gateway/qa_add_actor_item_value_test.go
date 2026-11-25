package gateway_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func qaivRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rs")
	src := filepath.Join("..", "rules", "testdata", "valid")
	if err := os.CopyFS(dir, os.DirFS(src)); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	path := filepath.Join(dir, "ruleset.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	props, _ := m["properties"].([]any)
	for _, k := range []string{"quebec", "romeo", "sierra", "tango"} {
		props = append(props, map[string]any{
			"key": k, "type": "number", "name": "N " + k,
			"description": "An invented number.",
		})
	}
	m["properties"] = props
	forms, _ := m["forms"].([]any)
	m["forms"] = append([]any{
		map[string]any{
			"key": "zulu", "name": "Lima", "description": "An invented form.",
			"properties": []string{"romeo", "quebec"},
		},
		map[string]any{
			"key": "yankee", "name": "Kilo", "description": "Another form.",
			"properties": []string{"quebec", "tango"},
		},
	}, forms...)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return rs
}

type qaivTable struct {
	c    *campaign.Campaign
	conn *websocket.Conn
	res  chan *vttv1.CommandResult
	n    int
}

func qaivOpen(t *testing.T, rs *rules.Ruleset) *qaivTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("open campaign: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("open identity: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	token, _, err := ids.CreateInvite("Dee", identity.RoleDM)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	s := gateway.New(c, ids)
	if rs != nil {
		s = s.WithRuleset(rs)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" +
		url.QueryEscape(token) + "&after=0"
	conn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.SetReadLimit(-1)
	t.Cleanup(func() { _ = conn.CloseNow() })
	tb := &qaivTable{c: c, conn: conn, res: make(chan *vttv1.CommandResult, 16)}
	go func() {
		for {
			_, b, err := conn.Read(context.Background())
			if err != nil {
				close(tb.res)
				return
			}
			var f vttv1.ServerFrame
			if protojson.Unmarshal(b, &f) == nil && f.GetResult() != nil {
				tb.res <- f.GetResult()
			}
		}
	}()
	return tb
}

func (tb *qaivTable) head(t *testing.T) int64 {
	t.Helper()
	_, stop, head, err := tb.c.Subscribe(0, 1024)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	stop()
	return head
}

func (tb *qaivTable) add(t *testing.T, a *vttv1.Actor) *vttv1.CommandResult {
	t.Helper()
	tb.n++
	id := "qaiv-" + strconv.Itoa(tb.n)
	b, err := protojson.Marshal(&vttv1.ClientCommand{
		RequestId: id,
		Command:   &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: a}},
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tb.conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("send: %v", err)
	}
	for {
		select {
		case r, ok := <-tb.res:
			if !ok {
				t.Fatal("connection closed before the result")
			}
			if r.GetRequestId() == id {
				return r
			}
		case <-ctx.Done():
			t.Fatal("no result")
		}
	}
}

func qaivActor(id string, attrs map[string]int32) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: id, Name: "A " + id, Attributes: attrs,
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}
}

func (tb *qaivTable) refused(t *testing.T, a *vttv1.Actor) string {
	t.Helper()
	before := tb.head(t)
	r := tb.add(t, a)
	if r.GetOk() {
		t.Fatalf("accepted %v", a)
	}
	if after := tb.head(t); after != before {
		t.Fatalf("log moved from %d to %d on a refusal", before, after)
	}
	if _, held := tb.c.Actors()[a.GetActorId()]; held {
		t.Fatalf("refused actor %q is held", a.GetActorId())
	}
	return r.GetError()
}

// VTT-1062
func TestQAIVAnActorHoldingAListedKeyIsRefused(t *testing.T) {
	tb := qaivOpen(t, qaivRuleset(t))
	if r := tb.add(t, qaivActor("act-zero", map[string]int32{"sierra": 1})); !r.GetOk() {
		t.Fatalf("control refused: %s", r.GetError())
	}
	msg := tb.refused(t, qaivActor("act-one", map[string]int32{"sierra": 1, "quebec": 2}))
	for _, want := range []string{`actor "act-one"`, "attributes", `"quebec"`} {
		if !strings.Contains(msg, want) {
			t.Fatalf("refusal %q does not name %s", msg, want)
		}
	}
	t.Logf("refusal: %s", msg)
}

// VTT-1062
func TestQAIVTheRefusalIsShapedAsAnUndeclaredKeys(t *testing.T) {
	tb := qaivOpen(t, qaivRuleset(t))
	listed := tb.refused(t, qaivActor("act-one", map[string]int32{"tango": 1}))
	undeclared := tb.refused(t, qaivActor("act-one", map[string]int32{"xray": 1}))
	pre := `gateway: add_actor: actor "act-one": attributes: `
	if !strings.HasPrefix(listed, pre) || !strings.HasPrefix(undeclared, pre) {
		t.Fatalf("refusals %q and %q, want each to open %q", listed, undeclared, pre)
	}
	if !strings.Contains(listed, `"tango"`) || !strings.Contains(listed, `"yankee"`) {
		t.Fatalf("refusal %q does not name tango and the form that lists it", listed)
	}
}

// VTT-1062
func TestQAIVTheFirstListedKeyInByteOrderIsNamed(t *testing.T) {
	tb := qaivOpen(t, qaivRuleset(t))
	msg := tb.refused(t, qaivActor("act-one",
		map[string]int32{"tango": 1, "romeo": 1, "quebec": 1, "sierra": 1}))
	if !strings.Contains(msg, `"quebec"`) || strings.Contains(msg, `"romeo"`) {
		t.Fatalf("refusal %q, want quebec", msg)
	}
}

// VTT-1062
func TestQAIVAListedKeyIsToldBeforeTheAbilityIds(t *testing.T) {
	tb := qaivOpen(t, qaivRuleset(t))
	empty := qaivActor("act-one", map[string]int32{"romeo": 1})
	empty.AbilityIds = []string{""}
	unheld := qaivActor("act-two", map[string]int32{"romeo": 1})
	unheld.AbilityIds = []string{"no-such-ability"}
	for _, a := range []*vttv1.Actor{empty, unheld} {
		msg := tb.refused(t, a)
		if !strings.Contains(msg, "attributes") || !strings.Contains(msg, `"romeo"`) {
			t.Fatalf("refusal %q, want the listed key first", msg)
		}
	}
}

// VTT-1062
func TestQAIVAKindRefusalIsToldBeforeAListedKey(t *testing.T) {
	tb := qaivOpen(t, qaivRuleset(t))
	a := qaivActor("act-one", map[string]int32{"quebec": 1})
	a.Kind = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
	msg := tb.refused(t, a)
	if !strings.Contains(msg, "kind") || strings.Contains(msg, "quebec") {
		t.Fatalf("refusal %q, want the kind first", msg)
	}
}

// VTT-1062
func TestQAIVAnActorOfNumbersNoFormListsIsAdded(t *testing.T) {
	tb := qaivOpen(t, qaivRuleset(t))
	before := tb.head(t)
	if r := tb.add(t, qaivActor("act-one", map[string]int32{"sierra": 1})); !r.GetOk() {
		t.Fatalf("refused: %s", r.GetError())
	}
	if after := tb.head(t); after <= before {
		t.Fatalf("log did not move from %d", before)
	}
	if got := tb.c.Actors()["act-one"].GetAttributes()["sierra"]; got != 1 {
		t.Fatalf("held sierra %d, want 1", got)
	}
}

// VTT-675
func TestQAIVWithNoRulesetAListedKeyIsAdded(t *testing.T) {
	tb := qaivOpen(t, nil)
	if r := tb.add(t, qaivActor("act-one", map[string]int32{"quebec": 1})); !r.GetOk() {
		t.Fatalf("refused with no ruleset: %s", r.GetError())
	}
	if _, held := tb.c.Actors()["act-one"]; !held {
		t.Fatal("actor not held")
	}
}
