package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mcp"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const qaAbTavernDir = "../../rulesets/tavern-brawl"

func qaAbSheet(id string, abilities ...string) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       "Qa " + id,
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		Attributes: map[string]int32{"brawn": 1, "footing": 2},
		Resources:  map[string]*vttv1.Resource{"stamina": {Current: 2, Max: 2}},
		AbilityIds: abilities,
	}
}

func qaAbCampaign(t *testing.T, actors ...*vttv1.Actor) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	for i, a := range actors {
		env := &vttv1.Envelope{
			EventId: fmt.Sprintf("qa-ab-event-%d", i),
			Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}},
		}
		if _, err := c.Append(env); err != nil {
			_ = c.Close()
			t.Fatalf("append ActorAdded %q: %v", a.GetActorId(), err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatalf("campaign.Close: %v", err)
	}
	return dir
}

func qaAbBootRefusal(t *testing.T, dir string) string {
	t.Helper()
	_, closer, err := composeServer(dir, "127.0.0.1:0", qaAbTavernDir, "")
	if err == nil {
		_ = closer()
		t.Fatal("composeServer with the ruleset started a campaign whose actors name unheld ids")
	}
	msg := err.Error()
	t.Logf("refusal: %s", msg)
	if !strings.HasPrefix(msg, "vtt serve: ruleset ") {
		t.Fatalf("refusal %q does not begin %q", msg, "vtt serve: ruleset ")
	}
	const lead = ": the campaign's actors do not fit it: "
	i := strings.Index(msg, lead)
	if i < 0 {
		t.Fatalf("refusal %q does not contain %q", msg, lead)
	}
	return msg[i+len(lead):]
}

func qaAbMustBoot(t *testing.T, dir, rulesetDir string) {
	t.Helper()
	_, closer, err := composeServer(dir, "127.0.0.1:0", rulesetDir, "")
	if err != nil {
		t.Fatalf("composeServer(ruleset %q): %v", rulesetDir, err)
	}
	if err := closer(); err != nil {
		t.Fatalf("closer: %v", err)
	}
}

func qaAbUnheldPart(actor, id string) string {
	return `actor "` + actor + `": ability_ids: names ability "` + id + `", which the ruleset does not hold`
}

// VTT-688
func TestQAAbServeRefusesACampaignNamingEachActorAndItsFirstFault(t *testing.T) {
	keyMisfit := qaAbSheet("Z-keys", "zz-unheld")
	keyMisfit.Attributes["qa-undeclared"] = 1
	dir := qaAbCampaign(t,
		qaAbSheet("c-fits", "fists", "fists", "chair-swing"),
		qaAbSheet("d-late", "splash-of-water", "zz-late"),
		qaAbSheet("b-two", "zz-unheld", "fists", "beta-unheld", "Zed-unheld"),
		keyMisfit,
		qaAbSheet("e-none"),
	)
	got := qaAbBootRefusal(t, dir)
	want := strings.Join([]string{
		`actor "Z-keys": attributes: references undeclared property "qa-undeclared"`,
		qaAbUnheldPart("b-two", "Zed-unheld"),
		qaAbUnheldPart("d-late", "zz-late"),
	}, "; ")
	if got != want {
		t.Fatalf("refused actors = %q, want %q", got, want)
	}
}

// VTT-688
func TestQAAbServeRefusesACampaignWithOneUnfitActorAmongFittingOnes(t *testing.T) {
	dir := qaAbCampaign(t,
		qaAbSheet("a-fits", "sober-up", "fists"),
		qaAbSheet("m-one", "fists", "Fists"),
		qaAbSheet("z-fits"),
	)
	if got, want := qaAbBootRefusal(t, dir), qaAbUnheldPart("m-one", "Fists"); got != want {
		t.Fatalf("refused actors = %q, want %q", got, want)
	}
}

// VTT-688
func TestQAAbARefusedBootLeavesTheCampaignOpenable(t *testing.T) {
	dir := qaAbCampaign(t, qaAbSheet("x-bad", "zz-unheld"))
	qaAbBootRefusal(t, dir)
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open after a refused boot: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("campaign.Close: %v", err)
	}
	qaAbMustBoot(t, dir, "")
}

// VTT-688
func TestQAAbServeStartsACampaignWhoseActorsNameOnlyHeldIds(t *testing.T) {
	dir := qaAbCampaign(t,
		qaAbSheet("a-held", "splash-of-water", "chair-swing", "fists", "sober-up"),
		qaAbSheet("b-repeat", "fists", "fists"),
		qaAbSheet("c-none"),
	)
	qaAbMustBoot(t, dir, qaAbTavernDir)
}

// VTT-689
func TestQAAbServeWithNoRulesetRefusesNoCampaignForItsAbilityIds(t *testing.T) {
	dir := qaAbCampaign(t,
		qaAbSheet("a-free", "zz-unheld", "Zed-unheld", ""),
		qaAbSheet("b-free", "fists", "not-a-thing"),
	)
	qaAbMustBoot(t, dir, "")
}

type qaAbMCPTable struct {
	c  *campaign.Campaign
	cs *mcpsdk.ClientSession
}

func qaAbMCP(t *testing.T) *qaAbMCPTable {
	t.Helper()
	rs, err := rules.Load(qaAbTavernDir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	tok, _, err := ids.CreateInvite("Qa Agent", identity.RoleAgent)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	hs := httptest.NewServer(gateway.New(c, ids).WithRuleset(rs).Handler())
	t.Cleanup(hs.Close)
	toolsJSON, err := os.ReadFile("../../contract/gen/tools/tools.json")
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	srv, err := mcp.New(mcp.Config{WSURL: "ws" + strings.TrimPrefix(hs.URL, "http") + "/ws", Token: tok, ToolsJSON: toolsJSON})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serverT, clientT := mcpsdk.NewInMemoryTransports()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Run(ctx, serverT)
	}()
	cs, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qa-ab", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		cancel()
		t.Fatalf("mcp client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	})
	return &qaAbMCPTable{c: c, cs: cs}
}

func (m *qaAbMCPTable) addActor(t *testing.T, actor map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := m.cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: "add_actor", Arguments: map[string]any{"actor": actor}})
	if err != nil {
		t.Fatalf("CallTool add_actor: %v", err)
	}
	return res
}

func qaAbText(res *mcpsdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// VTT-683
func TestQAAbTheMCPToolAppendsAnActorNamingItsAbilityIdsInTheirOrder(t *testing.T) {
	m := qaAbMCP(t)
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"qa-mcp-ordered", []string{"sober-up", "fists", "splash-of-water", "chair-swing"}},
		{"qa-mcp-repeat", []string{"fists", "chair-swing", "fists"}},
	} {
		res := m.addActor(t, map[string]any{
			"actorId":    tc.id,
			"kind":       "ACTOR_KIND_PARTY_MEMBER",
			"attributes": map[string]any{"brawn": 2},
			"abilityIds": tc.want,
		})
		if res.IsError {
			t.Fatalf("add_actor %q refused: %s", tc.id, qaAbText(res))
		}
		folded := m.c.State().Actors[tc.id]
		if folded == nil {
			t.Fatalf("fold holds no actor %q after the tool answered", tc.id)
		}
		if got := folded.GetAbilityIds(); !slices.Equal(got, tc.want) {
			t.Fatalf("folded ability_ids = %q, want %q", got, tc.want)
		}
	}
}

// VTT-685
func TestQAAbTheMCPToolIsRefusedForAnUnheldIdAndAppendsNothing(t *testing.T) {
	m := qaAbMCP(t)
	before := len(m.c.State().Actors)
	res := m.addActor(t, map[string]any{
		"actorId":    "qa-mcp-unheld",
		"kind":       "ACTOR_KIND_NON_PARTY",
		"abilityIds": []string{"zz-unheld", "fists", "Zed-unheld"},
	})
	text := qaAbText(res)
	t.Logf("tool answer: %s", text)
	if !res.IsError {
		t.Fatalf("add_actor naming unheld ids was not an error: %s", text)
	}
	var answer struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatalf("tool answer %q is not a result: %v", text, err)
	}
	want := `gateway: add_actor: actor "qa-mcp-unheld": ability_ids: names ability "Zed-unheld", which the ruleset does not hold`
	if answer.Ok || answer.Error != want {
		t.Fatalf("tool answer = ok %v %q, want refusal %q", answer.Ok, answer.Error, want)
	}
	if _, ok := m.c.State().Actors["qa-mcp-unheld"]; ok || len(m.c.State().Actors) != before {
		t.Fatal("the refused add_actor appended an actor")
	}
}

// VTT-700
func TestQAAbTheServedAddActorSchemaDocumentsAbilityIds(t *testing.T) {
	m := qaAbMCP(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	list, err := m.cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var schema any
	for _, tool := range list.Tools {
		if tool.Name == "add_actor" {
			schema = tool.InputSchema
		}
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	var s struct {
		Properties struct {
			Actor struct {
				Properties map[string]struct {
					Type        string `json:"type"`
					Description string `json:"description"`
					Items       struct {
						Type string `json:"type"`
					} `json:"items"`
				} `json:"properties"`
			} `json:"actor"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	p, ok := s.Properties.Actor.Properties["abilityIds"]
	if !ok {
		t.Fatalf("served add_actor schema has no actor.abilityIds: %s", raw)
	}
	if p.Type != "array" || p.Items.Type != "string" {
		t.Fatalf("abilityIds is %q of %q, want an array of strings", p.Type, p.Items.Type)
	}
	t.Logf("abilityIds: %s", p.Description)
	for _, phrase := range []string{"abilities", "id", "served ruleset", "with a ruleset served", "the ruleset does not hold", "refuses"} {
		if !strings.Contains(strings.ToLower(p.Description), phrase) {
			t.Errorf("served abilityIds description %q does not say %q", p.Description, phrase)
		}
	}
}
