package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	vttmcp "github.com/PatrikLager/vtt-platform/internal/mcp"
)

type qaUseMCPTable struct {
	ws    string
	dmTok string
	cs    *mcpsdk.ClientSession
}

func qaUseMCPActor(id string, kind vttv1.ActorKind, abilities ...string) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId:    id,
		Name:       id,
		Kind:       kind,
		Attributes: map[string]int32{"brawn": 3, "grit": 1, "footing": 10},
		Resources: map[string]*vttv1.Resource{
			"drink":   {Current: 0, Max: 100},
			"stamina": {Current: 2, Max: 2},
		},
		AbilityIds: abilities,
	}
}

func qaUseMCPSeed(t *testing.T, dir string) (dmTok, agentTok string) {
	t.Helper()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	dmTok, dmID, err := ids.CreateInvite("Dee", identity.RoleDM)
	if err != nil {
		t.Fatalf("CreateInvite dm: %v", err)
	}
	agentTok, _, err = ids.CreateInvite("Ada", identity.RoleAgent)
	if err != nil {
		t.Fatalf("CreateInvite agent: %v", err)
	}
	var seq int
	appendEv := func(env *vttv1.Envelope) {
		seq++
		env.EventId = fmt.Sprintf("qa-use-mcp-seed-%d", seq)
		env.OccurredAt = timestamppb.Now()
		env.ParticipantId = dmID
		env.ActorRole = string(identity.RoleDM)
		if _, err := c.Append(env); err != nil {
			t.Fatalf("seed %d: %v", seq, err)
		}
	}
	appendEv(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "tavern", Name: "tavern", GridWidth: 8, GridHeight: 8,
	}}})
	other := vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	for i, a := range []*vttv1.Actor{
		qaUseMCPActor("brawler", other, "fists"),
		qaUseMCPActor("patron", other),
		qaUseMCPActor("twin", other, "fists", "chair-swing"),
	} {
		appendEv(&vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}})
		appendEv(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-" + a.GetActorId(), SceneId: "tavern", ActorId: a.GetActorId(),
			Position: &vttv1.GridPosition{X: int32(1 + i), Y: 1},
		}}})
	}
	if err := ids.Close(); err != nil {
		t.Fatalf("identity close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("campaign close: %v", err)
	}
	return dmTok, agentTok
}

func qaUseMCPBoot(t *testing.T) *qaUseMCPTable {
	t.Helper()
	dir := t.TempDir()
	dmTok, agentTok := qaUseMCPSeed(t, dir)
	srv, closeFn, err := composeServer(dir, "127.0.0.1:0", filepath.Join("..", "..", "rulesets", "tavern-brawl"), "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(l) }()
	wsURL := "ws://" + l.Addr().String() + "/ws"

	s, err := vttmcp.New(vttmcp.Config{WSURL: wsURL, Token: agentTok, ToolsJSON: toolsJSON})
	if err != nil {
		t.Fatalf("mcp.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	clientT, serverT := mcpsdk.NewInMemoryTransports()
	ran := make(chan error, 1)
	go func() { ran <- s.Run(ctx, serverT) }()
	cs, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "qa-use", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		cancel()
		t.Fatalf("mcp connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		cancel()
		select {
		case <-ran:
		case <-time.After(10 * time.Second):
			t.Errorf("mcp Run did not return")
		}
		_ = srv.Close()
		if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("serve: %v", err)
		}
		if err := closeFn(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	tb := &qaUseMCPTable{ws: wsURL, dmTok: dmTok, cs: cs}
	deadline := time.Now().Add(15 * time.Second)
	for {
		res, err := cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "get_state", Arguments: map[string]any{}})
		if err == nil && !res.IsError {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the agent never reached the gateway: %v %v", err, qaUseMCPText(res))
		}
		time.Sleep(50 * time.Millisecond)
	}
	return tb
}

func qaUseMCPText(res *mcpsdk.CallToolResult) string {
	if res == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func qaUseMCPRefusal(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	r := &vttv1.CommandResult{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal([]byte(qaUseMCPText(res)), r); err != nil {
		t.Errorf("tool content %q is not a CommandResult: %v", qaUseMCPText(res), err)
		return ""
	}
	if r.GetOk() || r.GetSequence() != 0 {
		t.Errorf("tool content %q: ok=%v sequence=%d on a refusal", qaUseMCPText(res), r.GetOk(), r.GetSequence())
	}
	return r.GetError()
}

func (tb *qaUseMCPTable) head(t *testing.T) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, tb.ws+"?token="+url.QueryEscape(tb.dmTok)+"&after=0", nil)
	if err != nil {
		t.Fatalf("dm dial: %v", err)
	}
	defer func() { _ = ws.Close(websocket.StatusNormalClosure, "") }()
	ws.SetReadLimit(1 << 22)
	_, data, err := ws.Read(ctx)
	if err != nil {
		t.Fatalf("dm read: %v", err)
	}
	f := &vttv1.ServerFrame{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, f); err != nil {
		t.Fatalf("dm frame: %v", err)
	}
	if f.GetCatchUpHead() == nil {
		t.Fatalf("first frame is not the catch-up head: %v", f)
	}
	return f.GetCatchUpHead().GetHeadSequence()
}

func (tb *qaUseMCPTable) use(t *testing.T, actor, ability string, targets ...string) *mcpsdk.CallToolResult {
	t.Helper()
	res, err := tb.cs.CallTool(context.Background(), &mcpsdk.CallToolParams{Name: "use_ability", Arguments: map[string]any{
		"actorId": actor, "abilityId": ability, "targetIds": targets,
	}})
	if err != nil {
		t.Fatalf("use_ability %s %s: protocol error %v", actor, ability, err)
	}
	return res
}

func qaUseServedTools(t *testing.T, cs *mcpsdk.ClientSession) map[string]*mcpsdk.Tool {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	out := map[string]*mcpsdk.Tool{}
	for _, tool := range res.Tools {
		out[tool.Name] = tool
	}
	return out
}

func qaUseAbilityIdsDoc(t *testing.T, schema any) string {
	t.Helper()
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("schema: %v", err)
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
				Required []string `json:"required"`
			} `json:"actor"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("schema decode: %v", err)
	}
	p, ok := s.Properties.Actor.Properties["abilityIds"]
	if !ok {
		t.Fatalf("add_actor's actor has no abilityIds property: %s", raw)
	}
	if p.Type != "array" || p.Items.Type != "string" {
		t.Errorf("abilityIds is %q of %q, want array of string", p.Type, p.Items.Type)
	}
	for _, r := range s.Properties.Actor.Required {
		if r == "abilityIds" {
			t.Errorf("abilityIds is required; a sheet naming none must be accepted")
		}
	}
	return p.Description
}

// VTT-690
func TestQAUseTheAgentsToolReturnsTheRefusalAsAToolError(t *testing.T) {
	tb := qaUseMCPBoot(t)
	before := tb.head(t)
	for _, c := range []struct{ actor, ability, target string }{
		{"brawler", "chair-swing", "patron"},
		{"brawler", "kick", "patron"},
		{"patron", "fists", "brawler"},
		{"brawler", "Fists", "patron"},
	} {
		res := tb.use(t, c.actor, c.ability, c.target)
		want := fmt.Sprintf("gateway: use_ability: actor %q does not name ability %q", c.actor, c.ability)
		if !res.IsError {
			t.Errorf("use_ability %s %s: isError=false, text %q", c.actor, c.ability, qaUseMCPText(res))
		}
		if got := qaUseMCPRefusal(t, res); got != want {
			t.Errorf("use_ability %s %s: error %q, want %q", c.actor, c.ability, got, want)
		}
	}
	if after := tb.head(t); after != before {
		t.Errorf("head moved %d -> %d across refused uses", before, after)
	}
	if res := tb.use(t, "twin", "chair-swing", "patron"); res.IsError {
		t.Errorf("twin chair-swing: %q, want resolved", qaUseMCPText(res))
	}
	if res := tb.use(t, "brawler", "fists", "patron"); res.IsError {
		t.Errorf("brawler fists: %q, want resolved", qaUseMCPText(res))
	}
	if after := tb.head(t); after <= before {
		t.Errorf("head %d after two named uses, want past %d", after, before)
	}
}

// Keep an actor the campaign does not hold to the rules engine's own refusal.
func TestQAUseTheAgentsToolLeavesAnActorTheCampaignDoesNotHoldToTheRulesEngine(t *testing.T) {
	tb := qaUseMCPBoot(t)
	before := tb.head(t)
	res := tb.use(t, "ghost", "fists", "patron")
	want := `rules: resolve: unknown actor "ghost"`
	if !res.IsError {
		t.Errorf("ghost fists: isError=false, text %q", qaUseMCPText(res))
	}
	if got := qaUseMCPRefusal(t, res); got != want {
		t.Errorf("ghost fists: error %q, want %q", got, want)
	}
	if after := tb.head(t); after != before {
		t.Errorf("head moved %d -> %d", before, after)
	}
}

// VTT-701
func TestQAUseTheServedUseAbilityDescriptionSaysAnUnnamedUseIsRefusedForEveryRole(t *testing.T) {
	tb := qaUseMCPBoot(t)
	tool, ok := qaUseServedTools(t, tb.cs)["use_ability"]
	if !ok {
		t.Fatalf("use_ability is not served")
	}
	d := tool.Description
	for _, frag := range []string{"sheet", "abilityIds", "refused", "whatever your role"} {
		if !strings.Contains(d, frag) {
			t.Errorf("use_ability description lacks %q: %q", frag, d)
		}
	}
}

// VTT-703
func TestQAUseTheServedAddActorSchemaSaysAnActorUsesOnlyWhatItsAbilityIdsName(t *testing.T) {
	tb := qaUseMCPBoot(t)
	tool, ok := qaUseServedTools(t, tb.cs)["add_actor"]
	if !ok {
		t.Fatalf("add_actor is not served")
	}
	d := qaUseAbilityIdsDoc(t, tool.InputSchema)
	for _, frag := range []string{"use_ability", "not named here", "names none can use none"} {
		if !strings.Contains(d, frag) {
			t.Errorf("abilityIds description lacks %q: %q", frag, d)
		}
	}
}

// VTT-701 VTT-703
func TestQAUseTheCommittedToolTextsAreTheServedOnes(t *testing.T) {
	tb := qaUseMCPBoot(t)
	served := qaUseServedTools(t, tb.cs)
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "gen", "tools", "tools.json"))
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	var committed []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		InputSchema any    `json:"inputSchema"`
	}
	if err := json.Unmarshal(raw, &committed); err != nil {
		t.Fatalf("decode tools.json: %v", err)
	}
	seen := 0
	for _, c := range committed {
		switch c.Name {
		case "use_ability":
			seen++
			if served[c.Name].Description != c.Description {
				t.Errorf("use_ability served %q, committed %q", served[c.Name].Description, c.Description)
			}
		case "add_actor":
			seen++
			if got, want := qaUseAbilityIdsDoc(t, served[c.Name].InputSchema), qaUseAbilityIdsDoc(t, c.InputSchema); got != want {
				t.Errorf("abilityIds served %q, committed %q", got, want)
			}
		}
	}
	if seen != 2 {
		t.Errorf("tools.json names %d of use_ability and add_actor", seen)
	}
}
