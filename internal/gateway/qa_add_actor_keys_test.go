package gateway_test

import (
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type qaKeysTable struct {
	camp *campaign.Campaign
	conn *websocket.Conn
}

func qaKeysRuleset() *rules.Ruleset {
	return &rules.Ruleset{
		ID:            "qa-keys",
		Name:          "QA keys",
		FormatVersion: "3",
		Properties: []rules.Property{
			{Key: "qa_num", Type: rules.PropertyNumber, Name: "QA number", Description: "A number."},
			{Key: "qa_num_two", Type: rules.PropertyNumber, Name: "QA number two", Description: "Another number."},
			{Key: "qa_pool", Type: rules.PropertyResource, Name: "QA pool", Description: "A resource.", Resource: &rules.ResourceDef{}},
			{Key: "qa_pool_two", Type: rules.PropertyResource, Name: "QA pool two", Description: "Another resource.", Resource: &rules.ResourceDef{}},
		},
		Conditions: map[string]*rules.Condition{},
		Atoms:      map[string]*rules.AtomDef{},
		Compiled:   map[string]*rules.CompiledPower{},
	}
}

func qaKeysLoadedRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	manifest := `{
  "id": "qa-keys",
  "name": "QA keys",
  "format_version": "3",
  "properties": [
    {"key": "qa_num", "type": "number", "name": "QA number", "description": "A number."},
    {"key": "qa_num_two", "type": "number", "name": "QA number two", "description": "Another number."},
    {"key": "qa_pool", "type": "resource", "name": "QA pool", "description": "A resource."},
    {"key": "qa_pool_two", "type": "resource", "name": "QA pool two", "description": "Another resource."}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "ruleset.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "guide.md"), []byte("QA guide.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"abilities", "atoms", "conditions"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaKeysServe(t *testing.T, rs *rules.Ruleset) *qaKeysTable {
	t.Helper()
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		_ = camp.Close()
		t.Fatalf("identity.Open: %v", err)
	}
	token, _, err := ids.CreateInvite("qa-dm", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	srv := gateway.New(camp, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	hs := httptest.NewServer(srv.Handler())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws?token=" + url.QueryEscape(token) + "&after=0"
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		hs.Close()
		t.Fatalf("websocket.Dial: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		hs.Close()
		_ = ids.Close()
		_ = camp.Close()
	})
	return &qaKeysTable{camp: camp, conn: conn}
}

func (tb *qaKeysTable) addActor(t *testing.T, a *vttv1.Actor) *vttv1.CommandResult {
	t.Helper()
	sendCommand(t, tb.conn, &vttv1.ClientCommand{
		RequestId: "qa-req-" + a.GetActorId(),
		Command:   &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: a}},
	})
	return readResult(t, tb.conn)
}

func qaNonParty(id string, attrs map[string]int32, res map[string]*vttv1.Resource) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       "QA " + id,
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		Attributes: attrs,
		Resources:  res,
	}
}

func qaPool(current, ceiling int32) *vttv1.Resource {
	return &vttv1.Resource{Current: current, Max: ceiling}
}

func (tb *qaKeysTable) requireKeyRefusal(t *testing.T, res *vttv1.CommandResult, actorID, field, key string) {
	t.Helper()
	if res.GetOk() {
		t.Fatalf("add_actor of %q was accepted; want it refused naming %s key %q", actorID, field, key)
	}
	msg := res.GetError()
	t.Logf("refusal: %s", msg)
	for _, want := range []string{actorID, field, key} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q does not name %q", msg, want)
		}
	}
	if _, held := tb.camp.Actors()[actorID]; held {
		t.Errorf("the campaign holds actor %q after its add_actor was refused", actorID)
	}
}

func (tb *qaKeysTable) requireAccepted(t *testing.T, res *vttv1.CommandResult, actorID string) *vttv1.Actor {
	t.Helper()
	if !res.GetOk() {
		t.Fatalf("add_actor of %q was refused: %q", actorID, res.GetError())
	}
	got, held := tb.camp.Actors()[actorID]
	if !held {
		t.Fatalf("add_actor of %q answered ok but the campaign holds no such actor", actorID)
	}
	return got
}

// VTT-677
func TestQAKeysAnUndeclaredAttributeKeyIsRefusedNamingActorMapAndKey(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	res := tb.addActor(t, qaNonParty("qa-actor-ua", map[string]int32{"qa_num": 3, "qa_stray": 4}, nil))
	tb.requireKeyRefusal(t, res, "qa-actor-ua", "attributes", "qa_stray")
}

// VTT-677
func TestQAKeysAnUndeclaredResourceKeyIsRefusedNamingActorMapAndKey(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	res := tb.addActor(t, qaNonParty("qa-actor-ur", map[string]int32{"qa_num": 3},
		map[string]*vttv1.Resource{"qa_pool": qaPool(2, 4), "qa_stray": qaPool(1, 1)}))
	tb.requireKeyRefusal(t, res, "qa-actor-ur", "resources", "qa_stray")
}

// VTT-677
func TestQAKeysAResourceKeyUnderAttributesIsRefused(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	res := tb.addActor(t, qaNonParty("qa-actor-ra", map[string]int32{"qa_pool": 2}, nil))
	tb.requireKeyRefusal(t, res, "qa-actor-ra", "attributes", "qa_pool")
}

// VTT-677
func TestQAKeysANumberKeyUnderResourcesIsRefused(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	res := tb.addActor(t, qaNonParty("qa-actor-nr", nil,
		map[string]*vttv1.Resource{"qa_num": qaPool(3, 5)}))
	tb.requireKeyRefusal(t, res, "qa-actor-nr", "resources", "qa_num")
}

// VTT-677
func TestQAKeysAKeyDeclaredInAnotherCaseIsNotDeclared(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	res := tb.addActor(t, qaNonParty("qa-actor-case", map[string]int32{"QA_NUM": 1}, nil))
	tb.requireKeyRefusal(t, res, "qa-actor-case", "attributes", "QA_NUM")
}

// VTT-677
func TestQAKeysAttributesAreReadBeforeResources(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	// Keep the resource key sorting first: one merged order would name it.
	for i := 0; i < 6; i++ {
		id := "qa-actor-ab" + string(rune('a'+i))
		res := tb.addActor(t, qaNonParty(id,
			map[string]int32{"qa_zz_stray": 1},
			map[string]*vttv1.Resource{"qa_aa_stray": qaPool(1, 1)}))
		tb.requireKeyRefusal(t, res, id, "attributes", "qa_zz_stray")
		if strings.Contains(res.GetError(), "qa_aa_stray") {
			t.Errorf("refusal %q names the resource key; attributes are read first and only the first key is named", res.GetError())
		}
	}
}

// Keep an upper-case, a "_" and a lower-case key here: a case-insensitive
// order names another key, and only byte order names qaOrderFirst.
var qaOrderStrays = []string{"qa_stray_b", "QA_STRAY_Z", "qa_stray_a", "_qa_stray", "Qa_stray"}

const qaOrderFirst = "QA_STRAY_Z"

// VTT-677
func TestQAKeysTheFirstAttributeInByteOrderIsNamed(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	// Keep the repeats: map order is random, and one attempt can pass unsorted.
	for i := 0; i < 8; i++ {
		id := "qa-actor-ao" + string(rune('a'+i))
		attrs := map[string]int32{"qa_num": 1}
		for _, k := range qaOrderStrays {
			attrs[k] = 2
		}
		res := tb.addActor(t, qaNonParty(id, attrs, nil))
		tb.requireKeyRefusal(t, res, id, "attributes", qaOrderFirst)
		for _, k := range qaOrderStrays {
			if k != qaOrderFirst && strings.Contains(res.GetError(), k) {
				t.Errorf("refusal %q names %q; only the first key in byte order, %q, is named", res.GetError(), k, qaOrderFirst)
			}
		}
	}
}

// VTT-677
func TestQAKeysTheFirstResourceInByteOrderIsNamed(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	for i := 0; i < 8; i++ {
		id := "qa-actor-ro" + string(rune('a'+i))
		pools := map[string]*vttv1.Resource{"qa_pool": qaPool(1, 2)}
		for _, k := range qaOrderStrays {
			pools[k] = qaPool(1, 1)
		}
		res := tb.addActor(t, qaNonParty(id, map[string]int32{"qa_num": 1}, pools))
		tb.requireKeyRefusal(t, res, id, "resources", qaOrderFirst)
		for _, k := range qaOrderStrays {
			if k != qaOrderFirst && strings.Contains(res.GetError(), k) {
				t.Errorf("refusal %q names %q; only the first key in byte order, %q, is named", res.GetError(), k, qaOrderFirst)
			}
		}
	}
}

// VTT-677
func TestQAKeysTheFirstKeyThatDoesNotFitIsNamedNotTheFirstKey(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	for i := 0; i < 6; i++ {
		id := "qa-actor-ff" + string(rune('a'+i))
		res := tb.addActor(t, qaNonParty(id,
			map[string]int32{"qa_num": 1, "qa_num_two": 2, "qa_stray": 3}, nil))
		tb.requireKeyRefusal(t, res, id, "attributes", "qa_stray")
		if strings.Contains(res.GetError(), "qa_num") {
			t.Errorf("refusal %q names a declared key", res.GetError())
		}
	}
}

// VTT-677
func TestQAKeysAnUndeclaredKeyAndAKeyOfTheOtherTypeShareOneKeyOrder(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	cases := []struct {
		id, field, first, later string
		attrs                   map[string]int32
		pools                   map[string]*vttv1.Resource
	}{
		{"qa-actor-mx1", "attributes", "qa_pool", "qa_zstray",
			map[string]int32{"qa_pool": 1, "qa_zstray": 1}, nil},
		{"qa-actor-mx2", "attributes", "qa_astray", "qa_pool",
			map[string]int32{"qa_astray": 1, "qa_pool": 1}, nil},
		{"qa-actor-mx3", "resources", "qa_num", "qa_zstray",
			nil, map[string]*vttv1.Resource{"qa_num": qaPool(1, 1), "qa_zstray": qaPool(1, 1)}},
		{"qa-actor-mx4", "resources", "qa_astray", "qa_num",
			nil, map[string]*vttv1.Resource{"qa_astray": qaPool(1, 1), "qa_num": qaPool(1, 1)}},
	}
	for _, c := range cases {
		for i := 0; i < 4; i++ {
			id := c.id + string(rune('a'+i))
			res := tb.addActor(t, qaNonParty(id, c.attrs, c.pools))
			tb.requireKeyRefusal(t, res, id, c.field, c.first)
			if strings.Contains(res.GetError(), c.later) {
				t.Errorf("refusal %q names %q, which sorts after %q", res.GetError(), c.later, c.first)
			}
		}
	}
}

// VTT-677
func TestQAKeysDeclaredKeysEachInItsOwnMapAreAccepted(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	res := tb.addActor(t, qaNonParty("qa-actor-ok",
		map[string]int32{"qa_num": 0, "qa_num_two": 7},
		map[string]*vttv1.Resource{"qa_pool": qaPool(0, 0), "qa_pool_two": qaPool(3, 5)}))
	got := tb.requireAccepted(t, res, "qa-actor-ok")
	if v, ok := got.GetAttributes()["qa_num_two"]; !ok || v != 7 {
		t.Errorf("attributes = %v; want qa_num_two 7", got.GetAttributes())
	}
	if _, ok := got.GetAttributes()["qa_num"]; !ok {
		t.Errorf("attributes = %v; want qa_num held at 0", got.GetAttributes())
	}
	if r := got.GetResources()["qa_pool_two"]; r.GetCurrent() != 3 || r.GetMax() != 5 {
		t.Errorf("resources = %v; want qa_pool_two 3 of 5", got.GetResources())
	}
	if _, ok := got.GetResources()["qa_pool"]; !ok {
		t.Errorf("resources = %v; want qa_pool held", got.GetResources())
	}
}

// VTT-677
func TestQAKeysASheetOfSomeOrNoneOfTheDeclaredPropertiesIsAccepted(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	tb.requireAccepted(t, tb.addActor(t, qaNonParty("qa-actor-bare", nil, nil)), "qa-actor-bare")
	tb.requireAccepted(t, tb.addActor(t, qaNonParty("qa-actor-part", nil,
		map[string]*vttv1.Resource{"qa_pool_two": qaPool(1, 0)})), "qa-actor-part")
}

// VTT-152, VTT-677
func TestQAKeysAControllerIsToldBeforeAnyKey(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	for _, ctl := range []struct {
		name string
		set  func(a *vttv1.Actor)
	}{
		{"controller_id", func(a *vttv1.Actor) { a.ControllerId = "someone" }},
		{"controller_ids", func(a *vttv1.Actor) { a.ControllerIds = []string{"someone", "another"} }},
		{"empty entry", func(a *vttv1.Actor) { a.ControllerIds = []string{""} }},
	} {
		clean := qaNonParty("qa-actor-ctl", nil, nil)
		ctl.set(clean)
		want := tb.addActor(t, clean)
		if want.GetOk() || !strings.Contains(want.GetError(), "grant_actor_control") {
			t.Fatalf("%s: a controller alone answered %v; want refused naming grant_actor_control", ctl.name, want)
		}
		stray := qaNonParty("qa-actor-ctl", map[string]int32{"qa_stray": 1},
			map[string]*vttv1.Resource{"qa_num": qaPool(1, 1)})
		ctl.set(stray)
		got := tb.addActor(t, stray)
		if got.GetOk() {
			t.Fatalf("%s: a controller with a stray key was accepted", ctl.name)
		}
		if got.GetError() != want.GetError() {
			t.Errorf("%s: refusal = %q; want the controller's refusal %q, told before any key", ctl.name, got.GetError(), want.GetError())
		}
	}
	if len(tb.camp.Actors()) != 0 {
		t.Errorf("the campaign holds %d actors after only refusals", len(tb.camp.Actors()))
	}
}

// VTT-153, VTT-677
func TestQAKeysAMissingKindIsToldBeforeAnyKey(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	clean := qaNonParty("qa-actor-kind", nil, nil)
	clean.Kind = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
	want := tb.addActor(t, clean)
	if want.GetOk() {
		t.Fatalf("an actor stating no kind was accepted")
	}
	stray := qaNonParty("qa-actor-kind", map[string]int32{"qa_stray": 1},
		map[string]*vttv1.Resource{"qa_num": qaPool(1, 1)})
	stray.Kind = vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED
	got := tb.addActor(t, stray)
	if got.GetOk() {
		t.Fatalf("an actor stating no kind with a stray key was accepted")
	}
	if got.GetError() != want.GetError() {
		t.Errorf("refusal = %q; want the kind's refusal %q, told before any key", got.GetError(), want.GetError())
	}
}

// VTT-677
func TestQAKeysAPartyMemberIsReadAgainstTheRulesetToo(t *testing.T) {
	tb := qaKeysServe(t, qaKeysRuleset())
	a := qaNonParty("qa-actor-pm", map[string]int32{"qa_stray": 1}, nil)
	a.Kind = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	tb.requireKeyRefusal(t, tb.addActor(t, a), "qa-actor-pm", "attributes", "qa_stray")
}

// VTT-677
func TestQAKeysALoadedRulesetRefusesAndAcceptsAsDeclared(t *testing.T) {
	tb := qaKeysServe(t, qaKeysLoadedRuleset(t))
	tb.requireKeyRefusal(t, tb.addActor(t, qaNonParty("qa-actor-l1", map[string]int32{"qa_pool": 1}, nil)),
		"qa-actor-l1", "attributes", "qa_pool")
	tb.requireKeyRefusal(t, tb.addActor(t, qaNonParty("qa-actor-l2", nil,
		map[string]*vttv1.Resource{"qa_stray": qaPool(1, 1)})), "qa-actor-l2", "resources", "qa_stray")
	tb.requireAccepted(t, tb.addActor(t, qaNonParty("qa-actor-l3", map[string]int32{"qa_num": 1},
		map[string]*vttv1.Resource{"qa_pool": qaPool(0, 0)})), "qa-actor-l3")
}

// VTT-675
func TestQAKeysWithNoRulesetNoKeyIsReadAgainstOne(t *testing.T) {
	tb := qaKeysServe(t, nil)
	got := tb.requireAccepted(t, tb.addActor(t, qaNonParty("qa-actor-free",
		map[string]int32{"qa_stray": 1, "qa_pool": 2, "QA odd key!": 3},
		map[string]*vttv1.Resource{"qa_num": qaPool(1, 2), "qa_other": qaPool(0, 0)})), "qa-actor-free")
	for _, k := range []string{"qa_stray", "qa_pool", "QA odd key!"} {
		if _, ok := got.GetAttributes()[k]; !ok {
			t.Errorf("attributes = %v; want %q held", got.GetAttributes(), k)
		}
	}
	for _, k := range []string{"qa_num", "qa_other"} {
		if _, ok := got.GetResources()[k]; !ok {
			t.Errorf("resources = %v; want %q held", got.GetResources(), k)
		}
	}
}

// VTT-675
func TestQAKeysWithNoRulesetOneKeyInBothMapsIsAccepted(t *testing.T) {
	tb := qaKeysServe(t, nil)
	tb.requireAccepted(t, tb.addActor(t, qaNonParty("qa-actor-both",
		map[string]int32{"qa_both": 1},
		map[string]*vttv1.Resource{"qa_both": qaPool(1, 1)})), "qa-actor-both")
}

// VTT-675
func TestQAKeysWithNoRulesetAKeyAtTheLengthBoundIsAccepted(t *testing.T) {
	tb := qaKeysServe(t, nil)
	long := strings.Repeat("k", 128)
	tb.requireAccepted(t, tb.addActor(t, qaNonParty("qa-actor-long",
		map[string]int32{long: 1},
		map[string]*vttv1.Resource{long: qaPool(1, 1)})), "qa-actor-long")
}
