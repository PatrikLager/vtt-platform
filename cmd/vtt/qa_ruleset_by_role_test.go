package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

type qaReadMCPTable struct {
	srv     *httptest.Server
	rs      *rules.Ruleset
	agent   string
	player  string
	session *mcpsdk.ClientSession
}

func qaReadMCPOpen(t *testing.T) *qaReadMCPTable {
	t.Helper()
	rs, err := rules.Load("../../rulesets/dnd45e-minimal")
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(filepath.Join(t.TempDir(), "identity.db"))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	agentTok, _, err := ids.CreateInvite("qa-agent", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}
	playerTok, playerID, err := ids.CreateInvite("qa-player", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	for i, env := range []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: "qa-hero", Name: "QA Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			AbilityIds: []string{"rally", "crossbow-shot"},
		}}}},
		{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: "qa-hero", ParticipantId: playerID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		}}},
	} {
		env.EventId = "qa-read-mcp-event-" + string(rune('a'+i))
		if _, err := c.Append(env); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	srv := httptest.NewServer(gateway.New(c, ids).WithRuleset(rs).Handler())
	t.Cleanup(srv.Close)
	server, err := mcp.New(mcp.Config{
		WSURL:     "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws",
		Token:     agentTok,
		ToolsJSON: toolsJSON,
	})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	serverSide, clientSide := mcpsdk.NewInMemoryTransports()
	go func() { _ = server.Run(ctx, serverSide) }()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qa-read", Version: "v0"}, nil)
	session, err := client.Connect(ctx, clientSide, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return &qaReadMCPTable{srv: srv, rs: rs, agent: agentTok, player: playerTok, session: session}
}

func (q *qaReadMCPTable) qaReadRoute(t *testing.T, token string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, q.srv.URL+"/api/ruleset", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/ruleset = %d %s (%v)", resp.StatusCode, body, err)
	}
	return string(body)
}

func qaReadAbilityIDsOf(t *testing.T, body string) []string {
	t.Helper()
	var got struct {
		Abilities []struct {
			ID string `json:"id"`
		} `json:"abilities"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	ids := []string{}
	for _, a := range got.Abilities {
		ids = append(ids, a.ID)
	}
	return ids
}

// VTT-693 VTT-621
func TestQAReadGetRulesetAsTheAgentReturnsEveryAbility(t *testing.T) {
	q := qaReadMCPOpen(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res, err := q.session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "get_ruleset", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError || len(res.Content) != 1 {
		t.Fatalf("get_ruleset: isError=%v content=%v", res.IsError, res.Content)
	}
	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("get_ruleset content is %T", res.Content[0])
	}
	var every []string
	for id := range q.rs.Compiled {
		every = append(every, id)
	}
	slices.Sort(every)
	if got := qaReadAbilityIDsOf(t, text.Text); !slices.Equal(got, every) {
		t.Errorf("get_ruleset abilities %v, want %v", got, every)
	}
	if text.Text != q.qaReadRoute(t, q.agent) {
		t.Errorf("get_ruleset is not the agent's route body verbatim")
	}
	if got := qaReadAbilityIDsOf(t, q.qaReadRoute(t, q.player)); !slices.Equal(got, []string{"crossbow-shot", "rally"}) {
		t.Errorf("fixture's player reads %v, want [crossbow-shot rally]", got)
	}
}

// VTT-694
func TestQAReadGetRulesetDescriptionSaysWhichAbilitiesEachRoleReads(t *testing.T) {
	q := qaReadMCPOpen(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tools, err := q.session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var desc string
	for _, tool := range tools.Tools {
		if tool.Name == "get_ruleset" {
			desc = tool.Description
		}
	}
	t.Logf("get_ruleset description: %s", desc)
	lower := strings.ToLower(desc)
	for _, want := range []string{"dm", "agent", "every ability", "player", "only", "sheet", "control"} {
		if !strings.Contains(lower, want) {
			t.Errorf("description does not say %q", want)
		}
	}
	if strings.Contains(lower, "as every participant can") {
		t.Errorf("description still says every participant reads the same")
	}
}
