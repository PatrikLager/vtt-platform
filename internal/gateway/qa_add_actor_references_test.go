package gateway_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaRF2GwManifest = `{
 "id": "qa-rf2", "name": "QA RF2", "format_version": "3", "version": "1",
 "properties": [
  {"key": "pool_a", "type": "resource", "name": "Pool a",
   "description": "The QA pool."},
  {"key": "brawn", "type": "number", "name": "Brawn",
   "description": "The QA brawn."},
  {"key": "guard", "type": "number", "name": "Guard",
   "description": "The QA guard."}
 ],
 "forms": [
  {"key": "zulu", "name": "Zulu", "description": "The QA zulu."},
  {"key": "yankee", "name": "Yankee", "description": "The QA yankee."}
 ],
 "places": [
  {"key": "mike", "name": "Mike", "description": "The QA mike.",
   "forms": ["zulu"], "states": ["lima", "oscar"]},
  {"key": "kilo", "name": "Kilo", "description": "The QA kilo.",
   "forms": ["zulu", "yankee"], "copies": true}
 ]
}`

func qaRF2GwWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func qaRF2GwRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join("..", "rules", "testdata", "valid")
	for _, sub := range []string{"abilities", "atoms", "conditions"} {
		files, err := os.ReadDir(filepath.Join(src, sub))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			raw, err := os.ReadFile(filepath.Join(src, sub, f.Name()))
			if err != nil {
				t.Fatal(err)
			}
			qaRF2GwWrite(t, filepath.Join(dir, sub, f.Name()), raw)
		}
	}
	guide, err := os.ReadFile(filepath.Join(src, "guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	qaRF2GwWrite(t, filepath.Join(dir, "guide.md"), guide)
	qaRF2GwWrite(t, filepath.Join(dir, "ruleset.json"), []byte(qaRF2GwManifest))
	for id, form := range map[string]string{"echo": "zulu", "delta": "yankee"} {
		qaRF2GwWrite(t, filepath.Join(dir, "catalogue", id+".json"), []byte(`{"id": "`+
			id+`", "name": "N", "description": "The QA entry.", "form": "`+form+`"}`))
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return rs
}

type qaRF2Table struct {
	c    *campaign.Campaign
	conn *websocket.Conn
	n    int
}

func qaRF2Open(t *testing.T, rs *rules.Ruleset) *qaRF2Table {
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
	token, _, err := ids.CreateInvite("Dee", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	s := gateway.New(c, ids)
	if rs != nil {
		s = s.WithRuleset(rs)
	}
	srv := httptest.NewServer(s.Handler())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?token=" + token
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	t.Cleanup(func() {
		_ = conn.CloseNow()
		srv.Close()
		_ = ids.Close()
		_ = c.Close()
	})
	return &qaRF2Table{c: c, conn: conn}
}

func (tb *qaRF2Table) head(t *testing.T) int64 {
	t.Helper()
	_, stop, head, err := tb.c.Subscribe(0, 1024)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	return head
}

func (tb *qaRF2Table) add(t *testing.T, a *vttv1.Actor) *vttv1.CommandResult {
	t.Helper()
	tb.n++
	rid := "qa-" + strconv.Itoa(tb.n)
	cmd := &vttv1.ClientCommand{RequestId: rid,
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: a}}}
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := tb.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatal(err)
	}
	for {
		_, data, err := tb.conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var f vttv1.ServerFrame
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, &f); err != nil {
			t.Fatal(err)
		}
		if r := f.GetResult(); r != nil && r.GetRequestId() == rid {
			return r
		}
	}
}

func (tb *qaRF2Table) refuse(t *testing.T, a *vttv1.Actor, want string, not ...string) {
	t.Helper()
	before := tb.head(t)
	r := tb.add(t, a)
	if r.GetOk() {
		t.Fatalf("add_actor accepted, want refused with %q", want)
	}
	if want != "" && r.GetError() != want {
		t.Errorf("error = %q\nwant    %q", r.GetError(), want)
	}
	for _, n := range not {
		if strings.Contains(r.GetError(), n) {
			t.Errorf("error %q names %q", r.GetError(), n)
		}
	}
	if after := tb.head(t); after != before {
		t.Errorf("head moved from %d to %d on a refusal", before, after)
	}
	if _, ok := tb.c.Actors()[a.GetActorId()]; ok {
		t.Errorf("actor %q is in the campaign after a refusal", a.GetActorId())
	}
}

func (tb *qaRF2Table) accept(t *testing.T, a *vttv1.Actor) {
	t.Helper()
	r := tb.add(t, a)
	if !r.GetOk() {
		t.Fatalf("add_actor refused: %s", r.GetError())
	}
	got, ok := tb.c.Actors()[a.GetActorId()]
	if !ok {
		t.Fatalf("actor %q not in the campaign", a.GetActorId())
	}
	if len(got.GetReferences()) != len(a.GetReferences()) {
		t.Fatalf("references = %v, want %v", got.GetReferences(), a.GetReferences())
	}
	for id, ref := range a.GetReferences() {
		if !proto.Equal(got.GetReferences()[id], ref) {
			t.Errorf("reference %q = %v, want %v", id, got.GetReferences()[id], ref)
		}
	}
}

func qaRF2GwRef(entry, place, state string) *vttv1.Reference {
	return &vttv1.Reference{EntryId: entry, Place: place, State: state}
}

func qaRF2Actor(id string, refs map[string]*vttv1.Reference) *vttv1.Actor {
	return &vttv1.Actor{ActorId: id, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		References: refs}
}

func qaRF2RefErr(t *testing.T, rs *rules.Ruleset, actor string,
	refs map[string]*vttv1.Reference) string {
	t.Helper()
	f := rs.SheetReferenceFault(refs)
	if f == nil {
		t.Fatal("fixture: SheetReferenceFault = nil")
	}
	return `gateway: add_actor: actor "` + actor + `": references: ` + f.Reason
}

// VTT-1039, VTT-1070.
func TestQARF2AddActorRefusesEachMisfit(t *testing.T) {
	rs := qaRF2GwRuleset(t)
	tb := qaRF2Open(t, rs)
	cases := []struct {
		name string
		refs map[string]*vttv1.Reference
		ref  string
	}{
		{"entry not held", map[string]*vttv1.Reference{
			"romeo": qaRF2GwRef("sierra", "mike", "lima")}, "romeo"},
		{"place not declared", map[string]*vttv1.Reference{
			"romeo": qaRF2GwRef("echo", "papa", "")}, "romeo"},
		{"form the place does not hold", map[string]*vttv1.Reference{
			"romeo": qaRF2GwRef("delta", "mike", "lima")}, "romeo"},
		{"state not named", map[string]*vttv1.Reference{
			"romeo": qaRF2GwRef("echo", "mike", "quebec")}, "romeo"},
		{"no state where some", map[string]*vttv1.Reference{
			"romeo": qaRF2GwRef("echo", "mike", "")}, "romeo"},
		{"state where none", map[string]*vttv1.Reference{
			"romeo": qaRF2GwRef("echo", "kilo", "lima")}, "romeo"},
		{"copy in a place that holds once", map[string]*vttv1.Reference{
			"a": qaRF2GwRef("echo", "mike", "lima"),
			"b": qaRF2GwRef("echo", "mike", "oscar")}, "b"},
		{"lowest id first", map[string]*vttv1.Reference{
			"alpha": qaRF2GwRef("sierra", "mike", "lima"),
			"Zed":   qaRF2GwRef("echo", "mike", "")}, "Zed"},
		{"fit fault before copies fault", map[string]*vttv1.Reference{
			"a": qaRF2GwRef("echo", "mike", "lima"),
			"b": qaRF2GwRef("echo", "mike", "lima"),
			"c": qaRF2GwRef("sierra", "kilo", "")}, "c"},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actor := "act-" + strconv.Itoa(i)
			want := qaRF2RefErr(t, rs, actor, c.refs)
			if !strings.Contains(want, `"`+c.ref+`"`) {
				t.Fatalf("fixture: %q does not name %q", want, c.ref)
			}
			tb.refuse(t, qaRF2Actor(actor, c.refs), want)
		})
	}
}

// VTT-1038, VTT-1039.
func TestQARF2AddActorAcceptsFittingReferences(t *testing.T) {
	tb := qaRF2Open(t, qaRF2GwRuleset(t))
	tb.accept(t, qaRF2Actor("act-fit", map[string]*vttv1.Reference{
		"a": qaRF2GwRef("echo", "mike", "lima"),
		"b": qaRF2GwRef("echo", "kilo", ""),
		"c": qaRF2GwRef("echo", "kilo", ""),
		"d": qaRF2GwRef("delta", "kilo", ""),
	}))
	tb.accept(t, qaRF2Actor("act-none", nil))
}

// VTT-1039, VTT-1040.
func TestQARF2AddActorReadsReferencesLast(t *testing.T) {
	tb := qaRF2Open(t, qaRF2GwRuleset(t))
	bad := map[string]*vttv1.Reference{"whiskey": qaRF2GwRef("sierra", "mike", "lima")}
	empty := map[string]*vttv1.Reference{
		"":        qaRF2GwRef("echo", "mike", "lima"),
		"whiskey": qaRF2GwRef("sierra", "mike", "lima"),
	}
	key := func(refs map[string]*vttv1.Reference) *vttv1.Actor {
		a := qaRF2Actor("act-key", refs)
		a.Attributes = map[string]int32{"tango": 1}
		return a
	}
	ability := func(refs map[string]*vttv1.Reference) *vttv1.Actor {
		a := qaRF2Actor("act-ability", refs)
		a.AbilityIds = []string{"uniform"}
		return a
	}
	emptyAbility := func(refs map[string]*vttv1.Reference) *vttv1.Actor {
		a := qaRF2Actor("act-empty-ability", refs)
		a.AbilityIds = []string{""}
		return a
	}
	spend := func(refs map[string]*vttv1.Reference) *vttv1.Actor {
		a := qaRF2Actor("act-spend", refs)
		a.Resources = map[string]*vttv1.Resource{"pool_a": {Current: 1}}
		return a
	}
	for _, refs := range []map[string]*vttv1.Reference{bad, empty} {
		tb.refuse(t, key(refs), `gateway: add_actor: actor "act-key": attributes: `+
			`references undeclared property "tango"`, "whiskey", "sierra")
		tb.refuse(t, ability(refs), "", "whiskey", "sierra", "references:")
		tb.refuse(t, emptyAbility(refs), "", "whiskey", "sierra", "references:")
		tb.refuse(t, spend(refs), "", "whiskey", "sierra", "references:")
	}
	for _, a := range []*vttv1.Actor{ability(bad), emptyAbility(bad), spend(bad)} {
		r := tb.add(t, a)
		t.Logf("%s: %s", a.GetActorId(), r.GetError())
		if !strings.Contains(r.GetError(), `actor "`+a.GetActorId()+`"`) {
			t.Errorf("error %q does not name the actor", r.GetError())
		}
	}
}

// VTT-1040.
func TestQARF2AddActorRefusesAnEmptyReferenceId(t *testing.T) {
	cases := []struct {
		name string
		rs   *rules.Ruleset
	}{{"ruleset served", qaRF2GwRuleset(t)}, {"none served", nil}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := qaRF2Open(t, c.rs)
			for _, refs := range []map[string]*vttv1.Reference{
				{"": qaRF2GwRef("echo", "mike", "lima")},
				{"": qaRF2GwRef("", "", "")},
				{"": qaRF2GwRef("echo", "mike", "lima"),
					"whiskey": qaRF2GwRef("sierra", "mike", "lima")},
				{"": qaRF2GwRef("echo", "mike", "lima"),
					"a": qaRF2GwRef("echo", "mike", "lima"),
					"b": qaRF2GwRef("echo", "mike", "lima")},
			} {
				r := tb.add(t, qaRF2Actor("act-empty", refs))
				t.Logf("%s: %s", c.name, r.GetError())
				tb.refuse(t, qaRF2Actor("act-empty", refs), "", "whiskey", "sierra",
					`"b"`, "already")
				if !strings.HasPrefix(r.GetError(),
					`gateway: add_actor: actor "act-empty": references: `) {
					t.Errorf("error %q does not name the actor and the field", r.GetError())
				}
			}
		})
	}
}

// VTT-1040.
func TestQARF2AddActorNoRulesetEmptyIdOrder(t *testing.T) {
	tb := qaRF2Open(t, nil)
	refs := map[string]*vttv1.Reference{"": qaRF2GwRef("echo", "mike", "lima")}
	a := qaRF2Actor("act-both", refs)
	a.AbilityIds = []string{""}
	tb.refuse(t, a, `gateway: add_actor: actor "act-both": ability_ids: `+
		`must not hold an empty id`)
	k := qaRF2Actor("act-keyed", refs)
	k.Attributes = map[string]int32{"tango": 1}
	r := tb.add(t, k)
	if r.GetOk() || !strings.Contains(r.GetError(), `actor "act-keyed": references: `) {
		t.Errorf("result = %v, want the empty reference id refused", r)
	}
}

// VTT-1041.
func TestQARF2AddActorNoRulesetReadsNoReference(t *testing.T) {
	tb := qaRF2Open(t, nil)
	tb.accept(t, qaRF2Actor("act-free", map[string]*vttv1.Reference{
		"a": qaRF2GwRef("sierra", "papa", "quebec"),
		"b": qaRF2GwRef("echo", "mike", ""),
		"c": qaRF2GwRef("echo", "mike", ""),
		"d": qaRF2GwRef("delta", "mike", "lima"),
		"e": qaRF2GwRef("echo", "kilo", "lima"),
		"f": qaRF2GwRef("", "", ""),
	}))
	k := qaRF2Actor("act-free-keys", map[string]*vttv1.Reference{
		"a": qaRF2GwRef("sierra", "papa", "quebec")})
	k.Attributes = map[string]int32{"tango": 1}
	k.AbilityIds = []string{"uniform"}
	tb.accept(t, k)
}
