package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qarc2Wait = 10 * time.Second

func qarc2RulesetFiles(version string) map[string]string {
	return map[string]string{
		"ruleset.json": `{
  "id": "qa-rc2",
  "name": "QA RC2",
  "format_version": "3",
  "version": "` + version + `",
  "properties": [
    {"key": "tally", "type": "number", "name": "Tally",
     "description": "The zulu form's tally."},
    {"key": "span", "type": "number", "name": "Span",
     "description": "The yankee form's span."}
  ],
  "forms": [
    {"key": "zulu", "name": "Zulu", "description": "The zulu form.",
     "properties": ["tally"]},
    {"key": "yankee", "name": "Yankee", "description": "The yankee form.",
     "properties": ["span"]}
  ],
  "places": [
    {"key": "mike", "name": "Mike", "description": "Holds copies.",
     "forms": ["zulu", "yankee"], "copies": true,
     "states": ["lima", "oscar"]},
    {"key": "kilo", "name": "Kilo", "description": "Holds once.",
     "forms": ["zulu"]}
  ]
}`,
		"abilities/stay.json": `{
  "id": "stay", "name": "Stay", "usage": "at_will",
  "compose": [{"atom": "here", "bind": {}}]
}`,
		"atoms/here.json": `{
  "id": "here", "params": [], "provides": ["delivery"], "consumes": [],
  "contributes": [{"kind": "targeting", "range": 0, "max_targets": 1}]
}`,
		"catalogue/echo.json": `{
  "id": "echo", "name": "Echo", "description": "The zulu entry.",
  "form": "zulu", "values": {"tally": 1}
}`,
		"catalogue/delta.json": `{
  "id": "delta", "name": "Delta", "description": "The yankee entry.",
  "form": "yankee", "values": {"span": 2}
}`,
		"guide.md": "# QA RC2\n\nA fixture.\n",
	}
}

func qarc2Ruleset(t *testing.T, version string) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	for name, body := range qarc2RulesetFiles(version) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

type qarc2Table struct {
	c        *campaign.Campaign
	ids      *identity.DB
	tokens   map[string]string
	pids     map[string]string
	servers  map[string]string
	rulesets map[string]*rules.Ruleset
}

func qarc2Open(t *testing.T) *qarc2Table {
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
	t.Cleanup(func() {
		_ = ids.Close()
		_ = c.Close()
	})
	tb := &qarc2Table{
		c: c, ids: ids,
		tokens:   map[string]string{},
		pids:     map[string]string{},
		servers:  map[string]string{},
		rulesets: map[string]*rules.Ruleset{},
	}
	roles := []struct {
		who  string
		role identity.Role
	}{
		{"dm", identity.RoleDM},
		{"agent", identity.RoleAgent},
		{"quinn", identity.RoleSpectator},
		{"sam", identity.RoleSpectator},
	}
	for _, r := range roles {
		tok, id, err := ids.CreateInvite(r.who, r.role)
		if err != nil {
			t.Fatal(err)
		}
		tb.tokens[r.who], tb.pids[r.who] = tok, id
	}
	tb.serve(t, "none", nil)
	return tb
}

func (tb *qarc2Table) serve(t *testing.T, name string, rs *rules.Ruleset) {
	t.Helper()
	s := gateway.New(tb.c, tb.ids)
	if rs != nil {
		s = s.WithRuleset(rs)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	tb.servers[name] = "ws" + strings.TrimPrefix(srv.URL, "http")
	tb.rulesets[name] = rs
}

type qarc2Conn struct {
	ws     *websocket.Conn
	frames chan *vttv1.ServerFrame
	seen   []*vttv1.ServerFrame
	n      int
}

func (tb *qarc2Table) dial(t *testing.T, server, who string) *qarc2Conn {
	t.Helper()
	u := tb.servers[server] + "/ws?token=" + url.QueryEscape(tb.tokens[who])
	ctx, cancel := context.WithTimeout(context.Background(), qarc2Wait)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial %s as %s: %v", server, who, err)
	}
	ws.SetReadLimit(1 << 22)
	k := &qarc2Conn{ws: ws, frames: make(chan *vttv1.ServerFrame, 8192)}
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	go func() {
		defer close(k.frames)
		for {
			_, b, err := ws.Read(context.Background())
			if err != nil {
				return
			}
			f := &vttv1.ServerFrame{}
			if protojson.Unmarshal(b, f) != nil {
				return
			}
			k.frames <- f
		}
	}()
	return k
}

func (k *qarc2Conn) next(t *testing.T, until time.Time) *vttv1.ServerFrame {
	t.Helper()
	select {
	case f, ok := <-k.frames:
		if !ok {
			t.Fatal("connection closed")
		}
		k.seen = append(k.seen, f)
		return f
	case <-time.After(time.Until(until)):
		return nil
	}
}

func (k *qarc2Conn) waitFor(
	t *testing.T, what string, pred func(*vttv1.ServerFrame) bool,
) *vttv1.ServerFrame {
	t.Helper()
	until := time.Now().Add(qarc2Wait)
	for {
		f := k.next(t, until)
		if f == nil {
			t.Fatalf("timed out waiting for %s", what)
		}
		if pred(f) {
			return f
		}
	}
}

func (k *qarc2Conn) send(
	t *testing.T, cmd *vttv1.ClientCommand,
) *vttv1.CommandResult {
	t.Helper()
	k.n++
	cmd.RequestId = fmt.Sprintf("qarc2-%d", k.n)
	b, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qarc2Wait)
	defer cancel()
	if err := k.ws.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
	f := k.waitFor(t, "result "+cmd.RequestId, func(f *vttv1.ServerFrame) bool {
		return f.GetResult().GetRequestId() == cmd.RequestId
	})
	return f.GetResult()
}

func (k *qarc2Conn) watchables() int {
	n := 0
	for _, f := range k.seen {
		if f.GetWatchableChanged() != nil {
			n++
		}
	}
	return n
}

func qarc2Log(t *testing.T, c *campaign.Campaign) []*vttv1.Envelope {
	t.Helper()
	events, unsub, head, err := c.Subscribe(0, 8192)
	if err != nil {
		t.Fatal(err)
	}
	defer unsub()
	var out []*vttv1.Envelope
	if head == 0 {
		return out
	}
	until := time.After(qarc2Wait)
	for {
		select {
		case env, ok := <-events:
			if !ok {
				t.Fatal("subscription closed during catch-up")
			}
			out = append(out, env)
			if env.GetSequence() >= head {
				return out
			}
		case <-until:
			t.Fatal("timed out reading the log")
		}
	}
}

func qarc2Ref(entry, place, state string) *vttv1.Reference {
	return &vttv1.Reference{EntryId: entry, Place: place, State: state}
}

func qarc2Add(actor, id string, r *vttv1.Reference) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddReference{
		AddReference: &vttv1.AddReference{
			ActorId: actor, ReferenceId: id, Reference: r,
		},
	}}
}

func qarc2Change(actor, id, place, state string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_ChangeReference{
		ChangeReference: &vttv1.ChangeReference{
			ActorId: actor, ReferenceId: id, Place: place, State: state,
		},
	}}
}

func qarc2Remove(actor, id string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveReference{
		RemoveReference: &vttv1.RemoveReference{
			ActorId: actor, ReferenceId: id,
		},
	}}
}

func qarc2AddActor(a *vttv1.Actor) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{
		AddActor: &vttv1.AddActor{Actor: a},
	}}
}

func qarc2Grant(actor, pid string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{
		Command: &vttv1.ClientCommand_GrantActorControl{
			GrantActorControl: &vttv1.GrantActorControl{
				ActorId: actor, ParticipantId: pid,
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			},
		},
	}
}

func qarc2Take(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_TakeAbility{
		TakeAbility: &vttv1.TakeAbility{ActorId: actor, AbilityId: ability},
	}}
}

// Keep the cast added through the server serving none: twin and odd do not fit.
func qarc2Cast(t *testing.T, tb *qarc2Table) {
	t.Helper()
	dm := tb.dial(t, "none", "dm")
	party := vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	other := vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	cast := []*vttv1.Actor{
		{ActorId: "pc", Name: "Pea", Kind: party,
			AbilityIds: []string{"stay"},
			References: map[string]*vttv1.Reference{
				"ref-m": qarc2Ref("echo", "kilo", ""),
				"ref-p": qarc2Ref("echo", "mike", "lima"),
				"ref-d": qarc2Ref("delta", "mike", "oscar"),
			}},
		{ActorId: "npc", Name: "Nil", Kind: other},
		{ActorId: "kin", Name: "Kin", Kind: other,
			AbilityIds: []string{"stay"},
			References: map[string]*vttv1.Reference{
				"ref-k": qarc2Ref("echo", "mike", "lima"),
			}},
		{ActorId: "twin", Name: "Twin", Kind: other,
			References: map[string]*vttv1.Reference{
				"ref-x1": qarc2Ref("echo", "kilo", ""),
				"ref-x2": qarc2Ref("echo", "kilo", ""),
				"ref-q":  qarc2Ref("delta", "mike", "lima"),
			}},
		{ActorId: "odd", Name: "Odd", Kind: other,
			References: map[string]*vttv1.Reference{
				"ref-f": qarc2Ref("foxtrot", "mike", "lima"),
				"ref-o": qarc2Ref("delta", "mike", "lima"),
			}},
	}
	for _, a := range cast {
		if res := dm.send(t, qarc2AddActor(a)); !res.GetOk() {
			t.Fatalf("add_actor %s: %s", a.GetActorId(), res.GetError())
		}
	}
	res := dm.send(t, qarc2Grant("pc", tb.pids["quinn"]))
	if !res.GetOk() {
		t.Fatalf("grant: %s", res.GetError())
	}
}

func qarc2Setup(t *testing.T) *qarc2Table {
	t.Helper()
	tb := qarc2Open(t)
	tb.serve(t, "rs", qarc2Ruleset(t, "1"))
	qarc2Cast(t, tb)
	return tb
}

func qarc2SameState(t *testing.T, want, got *engine.State) {
	t.Helper()
	if len(want.Actors) != len(got.Actors) {
		t.Fatalf("actors: %d, want %d", len(got.Actors), len(want.Actors))
	}
	for id, a := range want.Actors {
		if !proto.Equal(a, got.Actors[id]) {
			t.Fatalf("actor %s:\n got %v\nwant %v", id, got.Actors[id], a)
		}
	}
	w, g := *want, *got
	w.Actors, g.Actors = nil, nil
	if !reflect.DeepEqual(w, g) {
		t.Fatalf("state beside the actors changed:\n got %+v\nwant %+v", g, w)
	}
}

// VTT-1046, VTT-1047, VTT-1048 and VTT-1050: one envelope, stamped (SPEC-013).
func qarc2Accepted(
	t *testing.T, tb *qarc2Table, k *qarc2Conn, who string,
	cmd *vttv1.ClientCommand,
) (*vttv1.Envelope, *engine.State) {
	t.Helper()
	before, st := qarc2Log(t, tb.c), tb.c.State()
	res := k.send(t, cmd)
	if !res.GetOk() {
		t.Fatalf("%v refused: %s", cmd.GetCommand(), res.GetError())
	}
	after := qarc2Log(t, tb.c)
	if len(after) != len(before)+1 {
		t.Fatalf("appended %d envelopes, want 1", len(after)-len(before))
	}
	env := after[len(after)-1]
	if env.GetSequence() != res.GetSequence() {
		t.Fatalf("result sequence %d, event %d",
			res.GetSequence(), env.GetSequence())
	}
	if env.GetParticipantId() != tb.pids[who] || env.GetActorRole() != who {
		t.Fatalf("stamped %q %q, want %q %q", env.GetParticipantId(),
			env.GetActorRole(), tb.pids[who], who)
	}
	return env, st
}

// SPEC-013: a refusal reaches the issuer alone and appends nothing.
func qarc2Refused(
	t *testing.T, tb *qarc2Table, k *qarc2Conn, cmd *vttv1.ClientCommand,
) string {
	t.Helper()
	before, st := qarc2Log(t, tb.c), tb.c.State()
	res := k.send(t, cmd)
	if res.GetOk() {
		t.Fatalf("%v accepted, want refused", cmd.GetCommand())
	}
	if res.GetError() == "" || res.GetSequence() != 0 {
		t.Fatalf("refusal %q carries sequence %d",
			res.GetError(), res.GetSequence())
	}
	if after := qarc2Log(t, tb.c); len(after) != len(before) {
		t.Fatalf("a refusal appended %d envelopes", len(after)-len(before))
	}
	qarc2SameState(t, st, tb.c.State())
	return res.GetError()
}

func qarc2SheetIs(
	t *testing.T, tb *qarc2Table, before *engine.State, actor string,
	refs map[string]*vttv1.Reference,
) {
	t.Helper()
	want := before.Snapshot()
	a := proto.Clone(want.Actors[actor]).(*vttv1.Actor)
	a.References = refs
	want.Actors[actor] = a
	qarc2SameState(t, want, tb.c.State())
}

func qarc2With(
	refs map[string]*vttv1.Reference, id string, r *vttv1.Reference,
) map[string]*vttv1.Reference {
	out := map[string]*vttv1.Reference{}
	for k, v := range refs {
		out[k] = proto.Clone(v).(*vttv1.Reference)
	}
	if r == nil {
		delete(out, id)
	} else {
		out[id] = r
	}
	return out
}

func qarc2Refs(st *engine.State, actor string) map[string]*vttv1.Reference {
	return st.Actors[actor].GetReferences()
}

func qarc2FoldText(t *testing.T, tb *qarc2Table, env *vttv1.Envelope) string {
	t.Helper()
	err := engine.Apply(tb.c.State(), env)
	if err == nil {
		t.Fatalf("fixture: the fold accepts %v", env)
	}
	return err.Error()
}

func qarc2Contains(t *testing.T, got, want, why string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("%s: refusal %q does not carry %q", why, got, want)
	}
}

func qarc2NoRulesetNamed(t *testing.T, log []*vttv1.Envelope, after int64) {
	t.Helper()
	for _, e := range log {
		if e.GetSequence() > after && e.GetRulesetNamed() != nil {
			t.Fatalf("a RulesetNamed at %d: %v", e.GetSequence(), e)
		}
	}
}

// VTT-1046, VTT-1050.
func TestQARC2AnAddFromTheDMOrTheAgentAppendsOneReferenceAdded(t *testing.T) {
	tb := qarc2Setup(t)
	cases := []struct {
		who, actor, id string
		ref            *vttv1.Reference
	}{
		{"dm", "npc", "ref-k1", qarc2Ref("echo", "kilo", "")},
		{"agent", "npc", "ref-d1", qarc2Ref("delta", "mike", "lima")},
		{"dm", "pc", "ref-c1", qarc2Ref("echo", "mike", "oscar")},
		{"agent", "pc", "ref-c2", qarc2Ref("delta", "mike", "oscar")},
	}
	for _, c := range cases {
		k := tb.dial(t, "rs", c.who)
		cmd := qarc2Add(c.actor, c.id, c.ref)
		env, before := qarc2Accepted(t, tb, k, c.who, cmd)
		got := env.GetReferenceAdded()
		if got.GetActorId() != c.actor || got.GetReferenceId() != c.id ||
			!proto.Equal(got.GetReference(), c.ref) {
			t.Fatalf("%s: appended %v", c.id, env)
		}
		refs := qarc2With(qarc2Refs(before, c.actor), c.id, c.ref)
		qarc2SheetIs(t, tb, before, c.actor, refs)
	}
	qarc2NoRulesetNamed(t, qarc2Log(t, tb.c), 0)
}

// VTT-1047, VTT-1050, D8, D9 and D14.
func TestQARC2AChangeFromTheDMOrTheAgentAppendsOneReferenceChanged(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	cases := []struct {
		who, actor, id, place, state, entry string
	}{
		{"dm", "pc", "ref-p", "mike", "oscar", "echo"},
		{"agent", "kin", "ref-k", "kilo", "", "echo"},
		{"dm", "pc", "ref-m", "kilo", "", "echo"},
		{"agent", "pc", "ref-d", "mike", "lima", "delta"},
	}
	for _, c := range cases {
		k := tb.dial(t, "rs", c.who)
		cmd := qarc2Change(c.actor, c.id, c.place, c.state)
		env, before := qarc2Accepted(t, tb, k, c.who, cmd)
		got := env.GetReferenceChanged()
		want := &vttv1.ReferenceChanged{
			ActorId: c.actor, ReferenceId: c.id,
			Place: c.place, State: c.state,
		}
		if !proto.Equal(got, want) {
			t.Fatalf("%s: appended %v", c.id, env)
		}
		ref := qarc2Ref(c.entry, c.place, c.state)
		refs := qarc2With(qarc2Refs(before, c.actor), c.id, ref)
		qarc2SheetIs(t, tb, before, c.actor, refs)
	}
	qarc2NoRulesetNamed(t, qarc2Log(t, tb.c), 0)
}

// VTT-1048, VTT-1050 and D14.
func TestQARC2ARemoveFromTheDMOrTheAgentAppendsOneReferenceRemoved(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	cases := []struct{ server, who, actor, id string }{
		{"rs", "dm", "pc", "ref-d"},
		{"rs", "agent", "odd", "ref-f"},
		{"none", "dm", "kin", "ref-k"},
		{"none", "agent", "pc", "ref-p"},
	}
	for _, c := range cases {
		k := tb.dial(t, c.server, c.who)
		env, before := qarc2Accepted(t, tb, k, c.who, qarc2Remove(c.actor, c.id))
		want := &vttv1.ReferenceRemoved{ActorId: c.actor, ReferenceId: c.id}
		if !proto.Equal(env.GetReferenceRemoved(), want) {
			t.Fatalf("%s: appended %v", c.id, env)
		}
		refs := qarc2With(qarc2Refs(before, c.actor), c.id, nil)
		qarc2SheetIs(t, tb, before, c.actor, refs)
	}
	qarc2NoRulesetNamed(t, qarc2Log(t, tb.c), 0)
}

// VTT-1047, VTT-1061, D8 and D9.
func TestQARC2WithNoRulesetAChangeKeepsNothingOfTheOldPlace(t *testing.T) {
	tb := qarc2Setup(t)
	cases := []struct {
		who, actor, id, place, state, entry string
	}{
		{"dm", "pc", "ref-p", "november", "papa", "echo"},
		{"agent", "pc", "ref-d", "mike", "", "delta"},
		{"dm", "pc", "ref-p", "kilo", "", "echo"},
		{"agent", "kin", "ref-k", "kilo", "lima", "echo"},
	}
	for _, c := range cases {
		k := tb.dial(t, "none", c.who)
		cmd := qarc2Change(c.actor, c.id, c.place, c.state)
		_, before := qarc2Accepted(t, tb, k, c.who, cmd)
		ref := qarc2Ref(c.entry, c.place, c.state)
		refs := qarc2With(qarc2Refs(before, c.actor), c.id, ref)
		qarc2SheetIs(t, tb, before, c.actor, refs)
	}
}

func qarc2AddFaults() []struct {
	id  string
	ref *vttv1.Reference
} {
	return []struct {
		id  string
		ref *vttv1.Reference
	}{
		{"ref-e", qarc2Ref("foxtrot", "mike", "lima")},
		{"ref-n", qarc2Ref("echo", "november", "")},
		{"ref-y", qarc2Ref("delta", "kilo", "")},
		{"ref-s", qarc2Ref("echo", "mike", "")},
		{"ref-u", qarc2Ref("echo", "mike", "papa")},
		{"ref-v", qarc2Ref("echo", "kilo", "lima")},
	}
}

// VTT-1046, VTT-1061 and D14.
func TestQARC2EachFitFaultRefusesAnAddAndNoneIsReadWithNoRuleset(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	rs := tb.rulesets["rs"]
	for _, who := range []string{"dm", "agent"} {
		k := tb.dial(t, "rs", who)
		for _, c := range qarc2AddFaults() {
			fault := rs.ReferenceFault(c.id, c.ref)
			if fault == nil {
				t.Fatalf("fixture: %s fits", c.id)
			}
			for _, actor := range []string{"npc", "ghost"} {
				got := qarc2Refused(t, tb, k, qarc2Add(actor, c.id, c.ref))
				qarc2Contains(t, got, fault.Reason, c.id+" on "+actor)
			}
		}
	}
	for _, who := range []string{"dm", "agent"} {
		k := tb.dial(t, "none", who)
		for _, c := range qarc2AddFaults() {
			id := c.id + "-" + who
			_, before := qarc2Accepted(t, tb, k, who, qarc2Add("npc", id, c.ref))
			refs := qarc2With(qarc2Refs(before, "npc"), id, c.ref)
			qarc2SheetIs(t, tb, before, "npc", refs)
		}
	}
	k := tb.dial(t, "none", "dm")
	cmd := qarc2Add("pc", "ref-w", qarc2Ref("echo", "kilo", ""))
	qarc2Accepted(t, tb, k, "dm", cmd)
}

// VTT-1047 and D9.
func TestQARC2EachFitFaultRefusesAChange(t *testing.T) {
	tb := qarc2Setup(t)
	rs := tb.rulesets["rs"]
	cases := []struct{ actor, id, place, state string }{
		{"pc", "ref-p", "november", ""},
		{"pc", "ref-d", "kilo", ""},
		{"pc", "ref-p", "mike", ""},
		{"pc", "ref-p", "mike", "papa"},
		{"kin", "ref-k", "kilo", "lima"},
	}
	for _, who := range []string{"dm", "agent"} {
		k := tb.dial(t, "rs", who)
		for _, c := range cases {
			entry := qarc2Refs(tb.c.State(), c.actor)[c.id].GetEntryId()
			fault := rs.ReferenceFault(c.id, qarc2Ref(entry, c.place, c.state))
			if fault == nil {
				t.Fatalf("fixture: %s to %s fits", c.id, c.place)
			}
			cmd := qarc2Change(c.actor, c.id, c.place, c.state)
			got := qarc2Refused(t, tb, k, cmd)
			qarc2Contains(t, got, fault.Reason, c.id+" to "+c.place)
		}
	}
}

// VTT-1046.
func TestQARC2TheCopiesRuleRefusesAnAddNamingAnEntryItsPlaceHoldsOnce(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	k := tb.dial(t, "rs", "dm")
	for _, id := range []string{"ref-a", "ref-z"} {
		got := qarc2Refused(t, tb, k, qarc2Add("pc", id, qarc2Ref("echo", "kilo", "")))
		t.Logf("add %s: %s", id, got)
		qarc2Contains(t, got, id, "the new reference is named")
	}
	for _, id := range []string{"ref-a", "ref-z"} {
		cmd := qarc2Add("pc", id, qarc2Ref("echo", "mike", "oscar"))
		qarc2Accepted(t, tb, k, "dm", cmd)
	}
}

// VTT-1047 and D14.
func TestQARC2TheCopiesRuleRefusesAChangeIntoAPlaceHoldingItsEntry(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	k := tb.dial(t, "rs", "agent")
	qarc2Accepted(t, tb, k, "agent",
		qarc2Add("pc", "ref-z", qarc2Ref("echo", "mike", "lima")))
	for _, id := range []string{"ref-p", "ref-z"} {
		got := qarc2Refused(t, tb, k, qarc2Change("pc", id, "kilo", ""))
		t.Logf("change %s: %s", id, got)
		qarc2Contains(t, got, id, "the changed reference is named")
	}
	got := qarc2Refused(t, tb, k, qarc2Change("twin", "ref-x1", "kilo", ""))
	t.Logf("change ref-x1 kept in kilo beside ref-x2: %s", got)
	qarc2Accepted(t, tb, k, "agent", qarc2Change("pc", "ref-m", "kilo", ""))
	qarc2Accepted(t, tb, k, "agent", qarc2Change("pc", "ref-z", "mike", "oscar"))
}

// VTT-1046 and VTT-1047.
func TestQARC2ASheetOutOfFitRefusesNoOtherReference(t *testing.T) {
	tb := qarc2Setup(t)
	k := tb.dial(t, "rs", "dm")
	qarc2Accepted(t, tb, k, "dm", qarc2Change("twin", "ref-q", "mike", "oscar"))
	qarc2Accepted(t, tb, k, "dm",
		qarc2Add("twin", "ref-n", qarc2Ref("delta", "mike", "oscar")))
	qarc2Accepted(t, tb, k, "dm", qarc2Change("odd", "ref-o", "mike", "oscar"))
	qarc2Accepted(t, tb, k, "dm",
		qarc2Add("odd", "ref-n", qarc2Ref("echo", "kilo", "")))
}

// D14.
func TestQARC2AChangeOfAReferenceWhoseEntryTheCatalogueLacksIsRefused(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	k := tb.dial(t, "rs", "dm")
	fault := tb.rulesets["rs"].ReferenceFault("ref-f",
		qarc2Ref("foxtrot", "mike", "oscar"))
	got := qarc2Refused(t, tb, k, qarc2Change("odd", "ref-f", "mike", "oscar"))
	t.Logf("change ref-f: %s", got)
	qarc2Contains(t, got, fault.Reason, "entry from the sheet")
}

// VTT-1046, VTT-1047, VTT-1048, VTT-1061 and D14.
func TestQARC2TheFoldsRefusalsReachTheIssuerAndAppendNothing(t *testing.T) {
	tb := qarc2Setup(t)
	fit := qarc2Ref("echo", "mike", "oscar")
	added := func(actor, id string, r *vttv1.Reference) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceAdded{
			ReferenceAdded: &vttv1.ReferenceAdded{
				ActorId: actor, ReferenceId: id, Reference: r,
			},
		}}
	}
	changed := func(actor, id, place, state string) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceChanged{
			ReferenceChanged: &vttv1.ReferenceChanged{
				ActorId: actor, ReferenceId: id, Place: place, State: state,
			},
		}}
	}
	removed := func(actor, id string) *vttv1.Envelope {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_ReferenceRemoved{
			ReferenceRemoved: &vttv1.ReferenceRemoved{
				ActorId: actor, ReferenceId: id,
			},
		}}
	}
	cases := []struct {
		cmd *vttv1.ClientCommand
		env *vttv1.Envelope
	}{
		{qarc2Add("ghost", "ref-g", fit), added("ghost", "ref-g", fit)},
		{qarc2Add("pc", "ref-d", fit), added("pc", "ref-d", fit)},
		{qarc2Change("ghost", "ref-p", "mike", "oscar"),
			changed("ghost", "ref-p", "mike", "oscar")},
		{qarc2Change("ghost", "ref-p", "november", "papa"),
			changed("ghost", "ref-p", "november", "papa")},
		{qarc2Change("pc", "ref-none", "mike", "oscar"),
			changed("pc", "ref-none", "mike", "oscar")},
		{qarc2Change("pc", "ref-none", "november", "papa"),
			changed("pc", "ref-none", "november", "papa")},
		{qarc2Change("pc", "", "mike", "oscar"),
			changed("pc", "", "mike", "oscar")},
		{qarc2Remove("ghost", "ref-p"), removed("ghost", "ref-p")},
		{qarc2Remove("pc", "ref-none"), removed("pc", "ref-none")},
		{qarc2Remove("pc", ""), removed("pc", "")},
	}
	for _, server := range []string{"rs", "none"} {
		for _, who := range []string{"dm", "agent"} {
			k := tb.dial(t, server, who)
			for _, c := range cases {
				want := qarc2FoldText(t, tb, c.env)
				got := qarc2Refused(t, tb, k, c.cmd)
				qarc2Contains(t, got, want, server+" "+who)
			}
		}
	}
	k := tb.dial(t, "none", "dm")
	got := qarc2Refused(t, tb, k, qarc2Add("pc", "ref-nil", nil))
	qarc2Contains(t, got, qarc2FoldText(t, tb, added("pc", "ref-nil", nil)),
		"a reference left out")
	k = tb.dial(t, "rs", "dm")
	t.Logf("rs, left out: %s",
		qarc2Refused(t, tb, k, qarc2Add("pc", "ref-nil", nil)))
}

// D14.
func TestQARC2AnEmptyReferenceIdRefusesAnAddWithOrWithoutARuleset(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	for _, server := range []string{"rs", "none"} {
		for _, who := range []string{"dm", "agent"} {
			k := tb.dial(t, server, who)
			for _, actor := range []string{"npc", "ghost"} {
				cmd := qarc2Add(actor, "", qarc2Ref("echo", "mike", "lima"))
				t.Logf("%s %s %s: %s", server, who, actor,
					qarc2Refused(t, tb, k, cmd))
			}
		}
	}
}

// VTT-1046.
func TestQARC2TwoSheetsMayUseOneReferenceId(t *testing.T) {
	tb := qarc2Setup(t)
	k := tb.dial(t, "rs", "agent")
	cmd := qarc2Add("kin", "ref-m", qarc2Ref("echo", "kilo", ""))
	_, before := qarc2Accepted(t, tb, k, "agent", cmd)
	refs := qarc2With(qarc2Refs(before, "kin"), "ref-m",
		qarc2Ref("echo", "kilo", ""))
	qarc2SheetIs(t, tb, before, "kin", refs)
}

// VTT-1049 and SPEC-013.
func TestQARC2APlayerAndASpectatorAreRefusedEveryReferenceCommand(
	t *testing.T,
) {
	tb := qarc2Setup(t)
	cmds := map[string]func(actor string) *vttv1.ClientCommand{
		"add_reference": func(a string) *vttv1.ClientCommand {
			return qarc2Add(a, "ref-n", qarc2Ref("delta", "mike", "lima"))
		},
		"change_reference": func(a string) *vttv1.ClientCommand {
			return qarc2Change(a, "ref-p", "mike", "oscar")
		},
		"remove_reference": func(a string) *vttv1.ClientCommand {
			return qarc2Remove(a, "ref-d")
		},
	}
	seats := []struct{ who, role string }{
		{"quinn", "player"}, {"sam", "spectator"},
	}
	for _, server := range []string{"rs", "none"} {
		for _, s := range seats {
			k := tb.dial(t, server, s.who)
			for name, mk := range cmds {
				for _, actor := range []string{"pc", "npc"} {
					got := qarc2Refused(t, tb, k, mk(actor))
					qarc2Contains(t, got, name, s.who)
					qarc2Contains(t, got, s.role, s.who)
				}
			}
		}
	}
}

func qarc2IsEntry(kind string) func(*vttv1.ServerFrame) bool {
	return func(f *vttv1.ServerFrame) bool {
		e := f.GetEvent()
		switch kind {
		case "added":
			return e.GetReferenceAdded() != nil
		case "changed":
			return e.GetReferenceChanged() != nil
		case "removed":
			return e.GetReferenceRemoved() != nil
		}
		return false
	}
}

// VTT-1031, D22 and SPEC-015.
func TestQARC2NoReferenceCommandSendsAnyoneANewWatchableSet(t *testing.T) {
	tb := qarc2Setup(t)
	q := tb.dial(t, "rs", "quinn")
	s := tb.dial(t, "rs", "sam")
	perch := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: "pc"},
	}}
	if res := s.send(t, perch); !res.GetOk() {
		t.Fatalf("perch: %s", res.GetError())
	}
	s.waitFor(t, "perch on pc", func(f *vttv1.ServerFrame) bool {
		return f.GetViewpointChanged().GetActorId() == "pc"
	})
	dm := tb.dial(t, "rs", "dm")
	steps := []struct {
		kind string
		cmd  *vttv1.ClientCommand
	}{
		{"added", qarc2Add("pc", "ref-n", qarc2Ref("delta", "mike", "lima"))},
		{"changed", qarc2Change("pc", "ref-n", "mike", "oscar")},
		{"removed", qarc2Remove("pc", "ref-n")},
	}
	for _, st := range steps {
		qarc2Accepted(t, tb, dm, "dm",
			qarc2Add("npc", "ref-"+st.kind, qarc2Ref("delta", "mike", "lima")))
		qarc2Accepted(t, tb, dm, "dm", st.cmd)
		for name, k := range map[string]*qarc2Conn{"quinn": q, "sam": s} {
			k.waitFor(t, name+" sees "+st.kind, qarc2IsEntry(st.kind))
			if n := k.watchables(); n != 1 {
				t.Fatalf("%s after %s: %d watchable sets", name, st.kind, n)
			}
		}
	}
	if res := dm.send(t, qarc2Take("pc", "stay")); !res.GetOk() {
		t.Fatalf("control take: %s", res.GetError())
	}
	q.waitFor(t, "the control's set", func(f *vttv1.ServerFrame) bool {
		return f.GetWatchableChanged() != nil
	})
}

// VTT-1050 and SPEC-026.
func TestQARC2NoReferenceCommandAppendsARulesetNamedAfterASwitch(
	t *testing.T,
) {
	tb := qarc2Open(t)
	tb.serve(t, "rs", qarc2Ruleset(t, "1"))
	tb.serve(t, "rs2", qarc2Ruleset(t, "2"))
	qarc2Cast(t, tb)
	dm1 := tb.dial(t, "rs", "dm")
	if res := dm1.send(t, qarc2Take("kin", "stay")); !res.GetOk() {
		t.Fatalf("take: %s", res.GetError())
	}
	log := qarc2Log(t, tb.c)
	first := log[len(log)-2]
	if first.GetRulesetNamed().GetVersion() != "1" {
		t.Fatalf("control: the take opened with %v", first)
	}
	mark := log[len(log)-1].GetSequence()
	dm2 := tb.dial(t, "rs2", "dm")
	ag2 := tb.dial(t, "rs2", "agent")
	qarc2Accepted(t, tb, dm2, "dm",
		qarc2Add("pc", "ref-n", qarc2Ref("delta", "mike", "lima")))
	qarc2Accepted(t, tb, ag2, "agent",
		qarc2Change("pc", "ref-n", "mike", "oscar"))
	qarc2Accepted(t, tb, dm2, "dm", qarc2Remove("pc", "ref-n"))
	qarc2NoRulesetNamed(t, qarc2Log(t, tb.c), mark)
	res := ag2.send(t, qarc2Take("pc", "stay"))
	if !res.GetOk() {
		t.Fatalf("take: %s", res.GetError())
	}
	log = qarc2Log(t, tb.c)
	if log[len(log)-2].GetRulesetNamed().GetVersion() != "2" {
		t.Fatalf("control: the take after the switch opened with %v",
			log[len(log)-2])
	}
}

type qarc2Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"inputSchema"`
}

func qarc2Tools(t *testing.T) map[string]qarc2Tool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "cmd", "vtt", "tools.json"))
	if err != nil {
		t.Fatal(err)
	}
	var list []qarc2Tool
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	out := map[string]qarc2Tool{}
	for _, tl := range list {
		out[tl.Name] = tl
	}
	return out
}

// VTT-1051 and D15, read from cmd/vtt/tools.json.
func TestQARC2TheReferenceToolsSayWhatTheyChangeRefuseAndTell(
	t *testing.T,
) {
	tools := qarc2Tools(t)
	common := []string{
		"No player is sent a message of the change",
		"whoever is sent the sheet sees it there",
		"DM/agent only",
		"Refused, appending nothing",
		"an actor the campaign does not hold",
	}
	own := map[string][]string{
		"add_reference": {
			"Only the sheet's references change",
			"a reference id the sheet holds already",
			"an empty reference id", "a reference left out",
			"with a ruleset served", "with no ruleset served",
			"an entry the catalogue does not hold",
			"a place the ruleset does not declare",
			"an entry of a form its place does not hold",
			"no state in a place that names its states",
			"a state its place does not name",
			"any state in a place that names none",
			"an entry another of the sheet's references names in the same " +
				"place, unless that place holds copies",
		},
		"change_reference": {
			"Only that reference's place and state change",
			"The reference keeps its entry",
			"a reference id the sheet does not hold",
			"with a ruleset served", "with no ruleset served",
			"a place the ruleset does not declare",
			"a place that does not hold the entry's form",
			"no state in a place that names its states",
			"a state its place does not name",
			"any state in a place that names none",
			"a place that holds an entry once and holds the reference's " +
				"entry already under another reference id",
		},
		"remove_reference": {
			"Only the sheet's references change",
			"with a ruleset served or none",
			"a reference id the sheet does not hold",
		},
	}
	for name, phrases := range own {
		tl, ok := tools[name]
		if !ok {
			t.Fatalf("no %s in tools.json", name)
		}
		for _, p := range append(append([]string{}, common...), phrases...) {
			if !strings.Contains(tl.Description, p) {
				t.Errorf("%s's description lacks %q", name, p)
			}
		}
		if strings.Contains(tl.Description, "is told") {
			t.Errorf("%s's description says a player is told", name)
		}
	}
	var actor struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(
		tools["add_actor"].InputSchema.Properties["actor"], &actor,
	); err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Required []string `json:"required"`
	}
	raw := tools["add_reference"].InputSchema.Properties["reference"]
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ref.Required, []string{"entryId", "place"}) {
		t.Errorf("add_reference's reference requires %v", ref.Required)
	}
	attrs := actor.Properties["attributes"].Description
	if !strings.Contains(attrs, "so does a key a catalogue form lists") {
		t.Errorf("add_actor's attributes does not refuse a listed key")
	}
	refs := actor.Properties["references"].Description
	for _, p := range []string{
		"With a ruleset served, a reference refuses the whole command",
		"an entry the catalogue does not hold",
		"unless that place holds copies",
		"A reference whose id is empty refuses the whole command, with a " +
			"ruleset served or none",
	} {
		if !strings.Contains(refs, p) {
			t.Errorf("add_actor's references lacks %q", p)
		}
	}
}

// VTT-1061.
func TestQARC2WithNoRulesetAnEntryPlaceAndStateAreNotRead(t *testing.T) {
	tb := qarc2Setup(t)
	k := tb.dial(t, "none", "dm")
	long := qarc2Ref(strings.Repeat("e", 2000), strings.Repeat("p", 2000),
		strings.Repeat("s", 2000))
	cmd := qarc2Add("npc", "ref-long", long)
	_, before := qarc2Accepted(t, tb, k, "dm", cmd)
	qarc2SheetIs(t, tb, before, "npc",
		qarc2With(qarc2Refs(before, "npc"), "ref-long", long))
	cmd = qarc2Change("npc", "ref-long", strings.Repeat("q", 2000),
		strings.Repeat("r", 2000))
	qarc2Accepted(t, tb, k, "dm", cmd)
	qarc2Accepted(t, tb, k, "dm", qarc2Add("npc", strings.Repeat("i", 2000),
		qarc2Ref("echo", "kilo", "")))
}

// D14 and SPEC-013's order of handleCommand.
func TestQARC2RefusalsComeInD14sOrder(t *testing.T) {
	tb := qarc2Setup(t)
	rs := tb.rulesets["rs"]
	k := tb.dial(t, "rs", "dm")
	empty := qarc2Refused(t, tb, k,
		qarc2Add("pc", "", qarc2Ref("echo", "mike", "lima")))
	bad := qarc2Ref("foxtrot", "november", "papa")
	got := qarc2Refused(t, tb, k, qarc2Add("pc", "", bad))
	if got != empty {
		t.Fatalf("empty id with a misfit: %q, want %q", got, empty)
	}
	both := qarc2Ref("echo", "kilo", "lima")
	got = qarc2Refused(t, tb, k, qarc2Add("pc", "ref-a", both))
	qarc2Contains(t, got, rs.ReferenceFault("ref-a", both).Reason, "fit first")
	got = qarc2Refused(t, tb, k, qarc2Change("pc", "ref-p", "kilo", "lima"))
	qarc2Contains(t, got, rs.ReferenceFault("ref-p", both).Reason, "fit first")
	got = qarc2Refused(t, tb, k,
		qarc2Add("pc", "ref-m", qarc2Ref("echo", "kilo", "")))
	t.Logf("an id held already, naming what it holds: %s", got)
}
