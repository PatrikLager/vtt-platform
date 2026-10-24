package gateway_test

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func qadtState(controller string) *engine.State {
	st := engine.NewState()
	st.Actors["qadt-actor"] = &vttv1.Actor{
		ActorId:       "qadt-actor",
		Name:          "Lera",
		ControllerId:  controller,
		ControllerIds: []string{controller},
		Kind:          vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}
	st.Actors["qadt-other"] = &vttv1.Actor{
		ActorId: "qadt-other",
		Name:    "Goblin",
		Kind:    vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}
	st.Tokens["qadt-tok"] = engine.Token{ID: "qadt-tok", SceneID: "qadt-scene", ActorID: "qadt-actor", X: 2, Y: 3}
	st.Tokens["qadt-gob"] = engine.Token{ID: "qadt-gob", SceneID: "qadt-scene", ActorID: "qadt-other", X: 5, Y: 5}
	return st
}

func qadtDescribe(tokenID, name, notes, gmName, gmNotes string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{
		RequestId: "qadt-req",
		Command: &vttv1.ClientCommand_DescribeToken{DescribeToken: &vttv1.DescribeToken{
			TokenId: tokenID, Name: name, Notes: notes, GmName: gmName, GmNotes: gmNotes,
		}},
	}
}

// SPEC-013 "One function, one table": "The DM and the agent hold every row but `set_viewpoint`".
func TestQADTDMAndAgentMayDescribeAToken(t *testing.T) {
	st := qadtState("p-player")
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := &identity.Participant{ID: "p-" + string(role), Name: "Boss", Role: role}
		for _, tok := range []string{"qadt-tok", "qadt-gob"} {
			cmd := qadtDescribe(tok, "a sleeping creature", "it snores", "Grukk", "has the key")
			if err := gateway.Authorize(p, cmd, st); err != nil {
				t.Errorf("%s describe_token %s: Authorize = %v, want nil", role, tok, err)
			}
		}
	}
}

// VTT-463
// SPEC-013 "Refusal by default": "the message names the role and the command".
func TestQADTPlayerIsRefusedEvenForATokenWhoseActorTheyControl(t *testing.T) {
	st := qadtState("p-player")
	p := &identity.Participant{ID: "p-player", Name: "Ann", Role: identity.RolePlayer}

	move := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: "qadt-tok", To: &vttv1.GridPosition{X: 3, Y: 3}},
	}}
	if err := gateway.Authorize(p, move, st); err != nil {
		t.Fatalf("fixture: the player must control qadt-tok's actor, move_token refused: %v", err)
	}

	for _, tok := range []string{"qadt-tok", "qadt-gob", "qadt-missing"} {
		err := gateway.Authorize(p, qadtDescribe(tok, "n", "no", "g", "gn"), st)
		if err == nil {
			t.Errorf("player describe_token %s: Authorize = nil, want a refusal", tok)
			continue
		}
		if !errors.Is(err, gateway.ErrUnauthorized) {
			t.Errorf("player describe_token %s: %v does not wrap ErrUnauthorized", tok, err)
		}
		if !strings.Contains(err.Error(), "describe_token") || !strings.Contains(err.Error(), "player") {
			t.Errorf("player describe_token %s: message %q names not both the role and the command", tok, err)
		}
	}
}

// VTT-463
// SPEC-013 "Refusal by default": "the message names the role and the command".
func TestQADTSpectatorIsRefused(t *testing.T) {
	st := qadtState("p-spec")
	p := &identity.Participant{ID: "p-spec", Name: "Sam", Role: identity.RoleSpectator}
	for _, tok := range []string{"qadt-tok", "qadt-gob"} {
		err := gateway.Authorize(p, qadtDescribe(tok, "n", "no", "g", "gn"), st)
		if err == nil {
			t.Errorf("spectator describe_token %s: Authorize = nil, want a refusal", tok)
			continue
		}
		if !errors.Is(err, gateway.ErrUnauthorized) {
			t.Errorf("spectator describe_token %s: %v does not wrap ErrUnauthorized", tok, err)
		}
		if !strings.Contains(err.Error(), "describe_token") || !strings.Contains(err.Error(), "spectator") {
			t.Errorf("spectator describe_token %s: message %q names not both the role and the command", tok, err)
		}
	}
}

// SPEC-013 "The player half": "For every role but player, `Authorize` returns after those two checks."
func TestQADTAuthorizeLeavesAnUnknownTokenToTheFoldForTheDM(t *testing.T) {
	st := qadtState("p-player")
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := &identity.Participant{ID: "p-" + string(role), Role: role}
		if err := gateway.Authorize(p, qadtDescribe("qadt-missing", "n", "", "", ""), st); err != nil {
			t.Errorf("%s describe_token of an unknown token: Authorize = %v, want nil", role, err)
		}
	}
}

// VTT-260
func TestQADTDescribeTokenBecomesTokenDescribedCarryingEveryField(t *testing.T) {
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent} {
		p := &identity.Participant{ID: "p-" + string(role), Name: "Boss", Role: role}
		cmd := qadtDescribe("qadt-tok", "a hooded figure", "it watches the door\nand hums", "Mirela the spy", "works for the duke; knows the password")
		env, err := gateway.ToEvent(cmd, p)
		if err != nil {
			t.Fatalf("%s: ToEvent = %v", role, err)
		}
		arm, ok := env.GetPayload().(*vttv1.Envelope_TokenDescribed)
		if !ok {
			t.Fatalf("%s: payload is %T, want *Envelope_TokenDescribed", role, env.GetPayload())
		}
		want := &vttv1.TokenDescribed{
			TokenId: "qadt-tok", Name: "a hooded figure", Notes: "it watches the door\nand hums",
			GmName: "Mirela the spy", GmNotes: "works for the duke; knows the password",
		}
		if !proto.Equal(arm.TokenDescribed, want) {
			t.Errorf("%s: TokenDescribed = %v, want %v", role, arm.TokenDescribed, want)
		}
		if env.GetParticipantId() != p.ID || env.GetActorRole() != string(role) {
			t.Errorf("%s: stamped participant %q role %q, want %q %q", role, env.GetParticipantId(), env.GetActorRole(), p.ID, role)
		}
	}
}

// VTT-260
func TestQADTEmptiedFieldsReachTheEventEmpty(t *testing.T) {
	p := &identity.Participant{ID: "p-dm", Role: identity.RoleDM}
	cases := []*vttv1.TokenDescribed{
		{TokenId: "qadt-tok", Name: "kept", Notes: "", GmName: "", GmNotes: ""},
		{TokenId: "qadt-tok", Name: "", Notes: "", GmName: "", GmNotes: "only this"},
		{TokenId: "qadt-tok"},
	}
	for _, want := range cases {
		env, err := gateway.ToEvent(qadtDescribe(want.TokenId, want.Name, want.Notes, want.GmName, want.GmNotes), p)
		if err != nil {
			t.Fatalf("ToEvent(%v) = %v", want, err)
		}
		got := env.GetTokenDescribed()
		if got == nil {
			t.Fatalf("ToEvent(%v): no TokenDescribed, payload %T", want, env.GetPayload())
		}
		if !proto.Equal(got, want) {
			t.Errorf("TokenDescribed = %v, want %v", got, want)
		}
	}
}

// VTT-260
func TestQADTPlaceTokenTextsReachTokenPlaced(t *testing.T) {
	p := &identity.Participant{ID: "p-agent", Role: identity.RoleAgent}
	cases := []*vttv1.TokenPlaced{
		{
			TokenId: "qadt-new", SceneId: "qadt-scene", ActorId: "qadt-other",
			Position: &vttv1.GridPosition{X: 7, Y: 4},
			Name:     "a sleeping creature", Notes: "curled by the fire",
			GmName: "Grukk the goblin chief", GmNotes: "wakes at any noise",
		},
		{
			TokenId: "qadt-new", SceneId: "qadt-scene", ActorId: "qadt-other",
			Position: &vttv1.GridPosition{X: 1, Y: 9},
			GmNotes:  "only the DM knows",
		},
		{
			TokenId: "qadt-plain", SceneId: "qadt-scene", ActorId: "qadt-actor",
			Position: &vttv1.GridPosition{X: 2, Y: 2},
		},
	}
	for _, want := range cases {
		cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: want.TokenId, SceneId: want.SceneId, ActorId: want.ActorId,
			Position: &vttv1.GridPosition{X: want.Position.X, Y: want.Position.Y},
			Name:     want.Name, Notes: want.Notes, GmName: want.GmName, GmNotes: want.GmNotes,
		}}}
		env, err := gateway.ToEvent(cmd, p)
		if err != nil {
			t.Fatalf("ToEvent(%v) = %v", want, err)
		}
		got := env.GetTokenPlaced()
		if got == nil {
			t.Fatalf("payload %T, want TokenPlaced", env.GetPayload())
		}
		if !proto.Equal(got, want) {
			t.Errorf("TokenPlaced = %v, want %v", got, want)
		}
	}
}

// SPEC-018
func TestQADTTheFourTextsAreNotBoundedByAuthorizeOrToEvent(t *testing.T) {
	st := qadtState("p-player")
	p := &identity.Participant{ID: "p-dm", Role: identity.RoleDM}
	big := strings.Repeat("é", 50_000)
	describe := qadtDescribe("qadt-tok", big, big+"n", big+"g", big+"gn")
	place := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
		TokenId: "qadt-big", SceneId: "qadt-scene", ActorId: "qadt-other", Position: &vttv1.GridPosition{X: 1, Y: 1},
		Name: big, Notes: big + "n", GmName: big + "g", GmNotes: big + "gn",
	}}}
	for _, cmd := range []*vttv1.ClientCommand{describe, place} {
		if err := gateway.Authorize(p, cmd, st); err != nil {
			t.Fatalf("Authorize of 100 KB texts = %v, want nil", err)
		}
		env, err := gateway.ToEvent(cmd, p)
		if err != nil {
			t.Fatalf("ToEvent of 100 KB texts = %v", err)
		}
		if d := env.GetTokenDescribed(); d != nil && (d.GetName() != big || d.GetGmNotes() != big+"gn") {
			t.Errorf("TokenDescribed altered a 100 KB text")
		}
		if pl := env.GetTokenPlaced(); pl != nil && (pl.GetNotes() != big+"n" || pl.GetGmName() != big+"g") {
			t.Errorf("TokenPlaced altered a 100 KB text")
		}
		if env.GetTokenDescribed() == nil && env.GetTokenPlaced() == nil {
			t.Errorf("payload %T, want TokenDescribed or TokenPlaced", env.GetPayload())
		}
	}
}

// SPEC-007 "The event envelope has no `type` field": "The `oneof payload` inlines one key per variant".
func TestQADTDescribeTokenDecodesFromTheWire(t *testing.T) {
	raw := []byte(`{"requestId":"r1","describeToken":{"tokenId":"qadt-tok","name":"n1","notes":"no1","gmName":"g1","gmNotes":"gn1"}}`)
	cmd, err := gateway.DecodeCommand(raw)
	if err != nil {
		t.Fatalf("DecodeCommand = %v", err)
	}
	want := &vttv1.DescribeToken{TokenId: "qadt-tok", Name: "n1", Notes: "no1", GmName: "g1", GmNotes: "gn1"}
	if !proto.Equal(cmd.GetDescribeToken(), want) {
		t.Errorf("decoded %v, want %v", cmd.GetDescribeToken(), want)
	}
}

type qadtTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema struct {
		Properties map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		} `json:"properties"`
		Required []string `json:"required"`
	} `json:"inputSchema"`
}

func qadtTools(t *testing.T) map[string]qadtTool {
	t.Helper()
	raw, err := os.ReadFile("../../contract/gen/tools/tools.json")
	if err != nil {
		t.Fatalf("read tools.json: %v", err)
	}
	var list []qadtTool
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("parse tools.json: %v", err)
	}
	out := map[string]qadtTool{}
	for _, tool := range list {
		out[tool.Name] = tool
	}
	return out
}

func qadtSorted(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

func TestQADTPlaceTokenToolKeepsTheFourTextsOptional(t *testing.T) {
	tool, ok := qadtTools(t)["place_token"]
	if !ok {
		t.Fatal("tools.json offers no place_token")
	}
	want := qadtSorted([]string{"tokenId", "sceneId", "actorId", "position"})
	if got := qadtSorted(tool.InputSchema.Required); !slices.Equal(got, want) {
		t.Errorf("place_token required = %v, want %v", got, want)
	}
	for _, f := range []string{"name", "notes", "gmName", "gmNotes"} {
		prop, ok := tool.InputSchema.Properties[f]
		if !ok {
			t.Errorf("place_token offers no %s", f)
			continue
		}
		if prop.Type != "string" {
			t.Errorf("place_token %s type = %q, want string", f, prop.Type)
		}
	}
}

func TestQADTPlaceTokenToolDocsTheFourTexts(t *testing.T) {
	tool := qadtTools(t)["place_token"]
	want := map[string]string{
		"name":    "Optional; what every player and spectator whose character sees this token is told it is, such as \"a sleeping creature\". Never the creature's true name unless the characters know it.",
		"notes":   "Optional; what they are told of it beside its name.",
		"gmName":  "Optional; the DM's and the agent's alone: no player or spectator is ever sent it.",
		"gmNotes": "Optional; the DM's and the agent's alone: no player or spectator is ever sent it.",
	}
	for f, doc := range want {
		if got := tool.InputSchema.Properties[f].Description; got != doc {
			t.Errorf("place_token %s description = %q, want %q", f, got, doc)
		}
	}
}

func TestQADTDescribeTokenToolRequiresAllFiveAndSaysSo(t *testing.T) {
	tool, ok := qadtTools(t)["describe_token"]
	if !ok {
		t.Fatal("tools.json offers no describe_token")
	}
	five := qadtSorted([]string{"tokenId", "name", "notes", "gmName", "gmNotes"})
	if got := qadtSorted(tool.InputSchema.Required); !slices.Equal(got, five) {
		t.Errorf("describe_token required = %v, want %v", got, five)
	}
	var props []string
	for name, prop := range tool.InputSchema.Properties {
		props = append(props, name)
		if prop.Type != "string" {
			t.Errorf("describe_token %s type = %q, want string", name, prop.Type)
		}
	}
	if got := qadtSorted(props); !slices.Equal(got, five) {
		t.Errorf("describe_token properties = %v, want %v", got, five)
	}
	const desc = "Rewrite a placed token's name, notes, GM name and GM notes, all four at once: a field sent empty is cleared, so send the present value of any you keep. Players and spectators whose character sees the token are sent its new name and notes; the GM name and GM notes reach only the DM and the agent. DM/agent only."
	if tool.Description != desc {
		t.Errorf("describe_token description = %q, want %q", tool.Description, desc)
	}
}

func TestQADTUseAbilityTargetDocSaysAPlayersTargetMustBeSeen(t *testing.T) {
	tool, ok := qadtTools(t)["use_ability"]
	if !ok {
		t.Fatal("tools.json offers no use_ability")
	}
	const sentence = "A player's every target must be an actor the character judging their moves sees; any other is refused exactly as an id that names no actor is."
	if got := tool.InputSchema.Properties["targetIds"].Description; !strings.Contains(got, sentence) {
		t.Errorf("use_ability targetIds description = %q, want it to contain %q", got, sentence)
	}
}
