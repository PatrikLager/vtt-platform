package gateway_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

const qaasWait = 10 * time.Second

var qaasRulesetFiles = map[string]string{
	"ruleset.json": `{
  "id": "qa-sight", "name": "QA Sight", "format_version": "2",
  "attributes": ["knack"], "defenses": ["guard"], "resources": []
}`,
	"guide.md": "# QA Sight\n",
	"atoms/aim.json": `{
  "id": "aim",
  "params": [{ "name": "distance", "kind": "int" }, { "name": "targets", "kind": "int" }],
  "provides": ["aimed"], "consumes": [],
  "contributes": [{ "kind": "targeting", "range": "{distance}", "max_targets": "{targets}" }]
}`,
	"atoms/contest.json": `{
  "id": "contest", "params": [], "provides": ["clash"], "consumes": ["aimed"],
  "contributes": [{ "kind": "resolution", "key": "clash",
    "roll": "1d20 + @caster.knack", "vs": "@target.guard",
    "branches": ["struck", "glanced"] }]
}`,
	"abilities/shot.json": `{
  "id": "shot", "name": "Shot", "usage": "at_will",
  "compose": [
    { "atom": "aim", "bind": { "distance": 15, "targets": 1 } },
    { "atom": "contest", "bind": {} }
  ]
}`,
	"abilities/volley.json": `{
  "id": "volley", "name": "Volley", "usage": "at_will",
  "compose": [
    { "atom": "aim", "bind": { "distance": 15, "targets": 2 } },
    { "atom": "contest", "bind": {} }
  ]
}`,
}

const (
	qaasKnack = 2
	qaasGuard = 12
)

func qaasRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	for name, body := range qaasRulesetFiles {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("load ruleset: %v", err)
	}
	return rs
}

type qaasFigure struct {
	id         string
	kind       vttv1.ActorKind
	x, y       int32
	controller string
}

var qaasCast = []qaasFigure{
	{"pc-bay", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, 10, 2, "pat"},
	{"pc-ash", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, 2, 2, "pat"},
	{"npc-near", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, 4, 2, ""},
	{"npc-pet", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, 3, 3, "quin"},
	{"npc-far", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, 12, 2, ""},
	{"npc-zed", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, 13, 1, ""},
	{"npc-abe", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, 13, 3, ""},
}

type qaasTable struct {
	url    string
	camp   *campaign.Campaign
	tokens map[string]string
	pids   map[string]string
}

func qaasOpen(t *testing.T, omit ...string) *qaasTable {
	t.Helper()
	dir := t.TempDir()
	camp, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("open campaign: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("open identity: %v", err)
	}
	pids := map[string]string{}
	tbl := &qaasTable{camp: camp, tokens: map[string]string{}, pids: pids}
	for name, role := range map[string]identity.Role{
		"dm": identity.RoleDM, "agent": identity.RoleAgent,
		"pat": identity.RolePlayer, "quin": identity.RolePlayer,
	} {
		tok, id, err := ids.CreateInvite(name, role)
		if err != nil {
			t.Fatalf("invite %s: %v", name, err)
		}
		tbl.tokens[name], pids[name] = tok, id
	}
	qaasAppend(t, camp, &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{
		SceneCreated: &vttv1.SceneCreated{
			SceneId: "keep", Name: "Keep", GridWidth: 16, GridHeight: 5,
			Tiles: qaasKeepTiles(),
		},
	}})
	omitted := map[string]bool{}
	for _, id := range omit {
		omitted[id] = true
	}
	for _, f := range qaasCast {
		if omitted[f.id] {
			continue
		}
		qaasAppend(t, camp, &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{
			ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
				ActorId: f.id, Name: f.id, Kind: f.kind,
				Attributes: map[string]int32{"knack": qaasKnack, "guard": qaasGuard},
			}},
		}})
		if f.controller != "" {
			qaasAppend(t, camp, &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
				ActorControlGranted: &vttv1.ActorControlGranted{
					ActorId: f.id, ParticipantId: pids[f.controller], Kind: f.kind,
				},
			}})
		}
		qaasAppend(t, camp, &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{
			TokenPlaced: &vttv1.TokenPlaced{
				TokenId: "tok-" + f.id, SceneId: "keep", ActorId: f.id,
				Position: &vttv1.GridPosition{X: f.x, Y: f.y},
			},
		}})
	}
	srv := httptest.NewServer(gateway.New(camp, ids).WithRuleset(qaasRuleset(t)).Handler())
	t.Cleanup(func() {
		srv.Close()
		_ = camp.Close()
		_ = ids.Close()
	})
	tbl.url = srv.URL
	return tbl
}

func qaasKeepTiles() map[string]*vttv1.TileRef {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 16; x++ {
		for y := int32(0); y < 5; y++ {
			kind := "floor"
			if x == 6 {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return tiles
}

func qaasAppend(t *testing.T, camp *campaign.Campaign, env *vttv1.Envelope) {
	t.Helper()
	env.EventId = fmt.Sprintf("qaas-setup-%d", qaasHead(t, camp)+1)
	if _, err := camp.Append(env); err != nil {
		t.Fatalf("append %T: %v", env.GetPayload(), err)
	}
}

func qaasHead(t *testing.T, camp *campaign.Campaign) int64 {
	t.Helper()
	_, unsubscribe, head, err := camp.Subscribe(0, 1<<12)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	unsubscribe()
	return head
}

func qaasWorldAfter(t *testing.T, camp *campaign.Campaign, after int64) []*vttv1.Envelope {
	t.Helper()
	events, unsubscribe, head, err := camp.Subscribe(after, 1<<12)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsubscribe()
	var out []*vttv1.Envelope
	deadline := time.After(qaasWait)
	for len(out) == 0 || out[len(out)-1].GetSequence() < head {
		if head <= after {
			return nil
		}
		select {
		case env := <-events:
			out = append(out, env)
		case <-deadline:
			t.Fatalf("world log after %d: %d events before the head %d", after, len(out), head)
		}
	}
	return out
}

type qaasConn struct {
	t      *testing.T
	conn   *websocket.Conn
	frames chan *vttv1.ServerFrame
	events []*vttv1.Envelope
	sent   int
}

func qaasDial(t *testing.T, tbl *qaasTable, who string) *qaasConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), qaasWait)
	defer cancel()
	u := "ws" + strings.TrimPrefix(tbl.url, "http") + "/ws?token=" +
		url.QueryEscape(tbl.tokens[who]) + "&after=0"
	conn, resp, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", who, err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	conn.SetReadLimit(1 << 22)
	c := &qaasConn{t: t, conn: conn, frames: make(chan *vttv1.ServerFrame, 1<<12)}
	go func() {
		defer close(c.frames)
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			frame := &vttv1.ServerFrame{}
			if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, frame); err != nil {
				return
			}
			c.frames <- frame
		}
	}()
	t.Cleanup(func() { _ = conn.CloseNow() })
	return c
}

func (c *qaasConn) next() *vttv1.ServerFrame {
	c.t.Helper()
	select {
	case f, ok := <-c.frames:
		if !ok {
			c.t.Fatal("connection closed")
		}
		if ev := f.GetEvent(); ev != nil {
			c.events = append(c.events, ev)
		}
		return f
	case <-time.After(qaasWait):
		c.t.Fatal("no frame in time")
	}
	return nil
}

func (c *qaasConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	c.t.Helper()
	c.sent++
	cmd.RequestId = fmt.Sprintf("qaas-%d", c.sent)
	data, err := protojson.Marshal(cmd)
	if err != nil {
		c.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), qaasWait)
	defer cancel()
	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		c.t.Fatalf("write: %v", err)
	}
	for {
		if r := c.next().GetResult(); r != nil && r.GetRequestId() == cmd.RequestId {
			return r
		}
	}
}

func (c *qaasConn) waitViewpoint(actorID string) {
	c.t.Helper()
	for {
		if v := c.next().GetViewpointChanged(); v != nil && v.GetActorId() == actorID {
			return
		}
	}
}

func (c *qaasConn) waitAbilityUsed() *vttv1.AbilityUsed {
	c.t.Helper()
	for i := 0; ; i++ {
		for ; i < len(c.events); i++ {
			if used := c.events[i].GetAbilityUsed(); used != nil {
				return used
			}
		}
		c.next()
		i--
	}
}

func qaasUse(caster, ability string, targets ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{
		UseAbility: &vttv1.UseAbility{
			ActorId: caster, AbilityId: ability, TargetIds: targets,
		},
	}}
}

type qaasReply struct {
	tbl            *qaasTable
	conn           *qaasConn
	res            *vttv1.CommandResult
	before, after  int64
	appendedEvents []*vttv1.Envelope
}

func qaasIssue(t *testing.T, omit []string, who, perch string, cmd *vttv1.ClientCommand) qaasReply {
	t.Helper()
	tbl := qaasOpen(t, omit...)
	c := qaasDial(t, tbl, who)
	if perch != "" {
		res := c.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
			SetViewpoint: &vttv1.SetViewpoint{ActorId: perch},
		}})
		if !res.GetOk() {
			t.Fatalf("set_viewpoint %s: %s", perch, res.GetError())
		}
		c.waitViewpoint(perch)
	}
	before := qaasHead(t, tbl.camp)
	res := c.send(cmd)
	after := qaasHead(t, tbl.camp)
	return qaasReply{
		tbl: tbl, conn: c, res: res, before: before, after: after,
		appendedEvents: qaasWorldAfter(t, tbl.camp, before),
	}
}

func qaasRefusedAsUnknown(t *testing.T, who, perch string, cmd *vttv1.ClientCommand, unseen ...string) string {
	t.Helper()
	seen := qaasIssue(t, nil, who, perch, cmd)
	absent := qaasIssue(t, unseen, who, perch, cmd)
	if seen.res.GetOk() {
		t.Fatalf("%v by %s was resolved, want it refused", cmd.GetUseAbility(), who)
	}
	if seen.after != seen.before {
		t.Errorf("refused %v appended %d events", cmd.GetUseAbility(), seen.after-seen.before)
	}
	if absent.res.GetOk() {
		t.Fatalf("%v by %s with %v absent was resolved", cmd.GetUseAbility(), who, unseen)
	}
	if seen.res.GetError() != absent.res.GetError() {
		t.Errorf("%v by %s: answer %q, but with %v naming no actor %q",
			cmd.GetUseAbility(), who, seen.res.GetError(), unseen, absent.res.GetError())
	}
	t.Logf("%v by %s: %q", cmd.GetUseAbility(), who, seen.res.GetError())
	return absent.res.GetError()
}

func qaasIsUnknownTarget(t *testing.T, answer string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if answer == rules.UnknownTarget(id).Error() {
			return
		}
	}
	t.Errorf("answer %q is no unknown-target refusal of %v", answer, ids)
}

func qaasResolved(t *testing.T, r qaasReply, cmd *vttv1.ClientCommand) *vttv1.AbilityUsed {
	t.Helper()
	if !r.res.GetOk() {
		t.Fatalf("%v was refused: %s", cmd.GetUseAbility(), r.res.GetError())
	}
	for _, env := range r.appendedEvents {
		if used := env.GetAbilityUsed(); used != nil {
			return used
		}
	}
	t.Fatalf("%v resolved and appended no AbilityUsed: %v", cmd.GetUseAbility(), r.appendedEvents)
	return nil
}

func qaasCheckOutcomes(t *testing.T, used *vttv1.AbilityUsed, targets ...string) {
	t.Helper()
	got, rolls := used.GetOutcomes(), used.GetRolls()
	if len(got) != len(targets) || len(rolls) != len(targets) {
		t.Fatalf("outcomes = %v, rolls = %v, want one of each for each of %v", got, rolls, targets)
	}
	for i, o := range got {
		if o.GetTotal() != rolls[i].GetTotal() {
			t.Errorf("outcome %d total %d, want its resolution roll's %d", i, o.GetTotal(), rolls[i].GetTotal())
		}
		want := "glanced"
		if o.GetTotal() >= qaasGuard {
			want = "struck"
		}
		if o.GetTargetId() != targets[i] || o.GetResult() != want ||
			o.GetTotal() < 1+qaasKnack || o.GetTotal() > 20+qaasKnack {
			t.Errorf("outcome %d = %v, want %s %s with a total of 1d20+%d",
				i, o, targets[i], want, qaasKnack)
		}
	}
}

// VTT-472
func TestQAASAPlayersTargetBehindAWallIsRefusedAsIfItNamedNoActor(t *testing.T) {
	cmd := qaasUse("pc-ash", "shot", "npc-far")
	qaasIsUnknownTarget(t, qaasRefusedAsUnknown(t, "pat", "", cmd, "npc-far"), "npc-far")
}

// VTT-487 VTT-473 VTT-488
func TestQAASAPlayersTargetInSightAndInRangeIsResolved(t *testing.T) {
	cmd := qaasUse("pc-ash", "shot", "npc-near")
	r := qaasIssue(t, nil, "pat", "", cmd)
	if !r.res.GetOk() {
		t.Fatalf("VTT-487: %v was refused: %s", cmd.GetUseAbility(), r.res.GetError())
	}
	qaasCheckOutcomes(t, qaasResolved(t, r, cmd), "npc-near")
	frame := r.conn.waitAbilityUsed()
	qaasCheckOutcomes(t, frame, "npc-near")
	if !strings.Contains(frame.GetOutcomeSummary(), fmt.Sprintf("vs %d", qaasGuard)) {
		t.Errorf("VTT-488: player's outcome_summary = %q, want it whole with the compared %d",
			frame.GetOutcomeSummary(), qaasGuard)
	}
}

// VTT-472 VTT-473
func TestQAASTwoTargetsInSightAreResolvedAndOneUnseenRefusesBoth(t *testing.T) {
	both := qaasUse("pc-bay", "volley", "npc-zed", "npc-far")
	r := qaasIssue(t, nil, "pat", "pc-bay", both)
	qaasCheckOutcomes(t, qaasResolved(t, r, both), "npc-zed", "npc-far")

	for _, targets := range [][]string{{"npc-near", "npc-far"}, {"npc-far", "npc-near"}} {
		cmd := qaasUse("pc-ash", "volley", targets...)
		qaasIsUnknownTarget(t, qaasRefusedAsUnknown(t, "pat", "", cmd, "npc-far"), "npc-far")
	}
}

// VTT-472
func TestQAASTheRefusalNamesWhatAnUnknownIdWouldHaveBeenRefusedFor(t *testing.T) {
	cases := []struct {
		targets, unseen []string
	}{
		{[]string{"npc-zed", "npc-abe"}, []string{"npc-zed", "npc-abe"}},
		{[]string{"npc-abe", "npc-zed"}, []string{"npc-zed", "npc-abe"}},
		{[]string{"npc-ghost", "npc-far"}, []string{"npc-far"}},
		{[]string{"npc-far", "npc-ghost"}, []string{"npc-far"}},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.targets, "+"), func(t *testing.T) {
			cmd := qaasUse("pc-ash", "volley", c.targets...)
			answer := qaasRefusedAsUnknown(t, "pat", "", cmd, c.unseen...)
			qaasIsUnknownTarget(t, answer, c.targets...)
		})
	}
}

// VTT-472
func TestQAASTheRefusalIsAnUnknownIdsWhateverElseTheCommandGetsWrong(t *testing.T) {
	cases := map[string]*vttv1.ClientCommand{
		"unknown ability":  qaasUse("pc-ash", "nonesuch", "npc-far"),
		"too many targets": qaasUse("pc-ash", "shot", "npc-near", "npc-far"),
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			qaasRefusedAsUnknown(t, "pat", "", cmd, "npc-far")
		})
	}
}

// VTT-472 VTT-453
func TestQAASAPerchedPlayerIsJudgedByThePartyMemberTheConnectionShows(t *testing.T) {
	far := qaasUse("pc-ash", "shot", "npc-far")
	qaasResolved(t, qaasIssue(t, nil, "pat", "pc-bay", far), far)
	near := qaasUse("pc-ash", "shot", "npc-near")
	qaasIsUnknownTarget(t, qaasRefusedAsUnknown(t, "pat", "pc-bay", near, "npc-near"), "npc-near")
}

// VTT-472 VTT-453
func TestQAASAPerchOnAPartyMemberNoLongerTheirsNoLongerJudges(t *testing.T) {
	tbl := qaasOpen(t)
	pat := qaasDial(t, tbl, "pat")
	if res := pat.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_SetViewpoint{
		SetViewpoint: &vttv1.SetViewpoint{ActorId: "pc-bay"},
	}}); !res.GetOk() {
		t.Fatalf("set_viewpoint: %s", res.GetError())
	}
	pat.waitViewpoint("pc-bay")
	far := qaasUse("pc-ash", "shot", "npc-far")
	if res := pat.send(far); !res.GetOk() {
		t.Fatalf("perched on pc-bay, %v was refused: %s", far.GetUseAbility(), res.GetError())
	}
	dm := qaasDial(t, tbl, "dm")
	if res := dm.send(&vttv1.ClientCommand{Command: &vttv1.ClientCommand_RevokeActorControl{
		RevokeActorControl: &vttv1.RevokeActorControl{
			ActorId: "pc-bay", ParticipantId: tbl.pids["pat"],
		},
	}}); !res.GetOk() {
		t.Fatalf("revoke: %s", res.GetError())
	}
	before := qaasHead(t, tbl.camp)
	res := pat.send(far)
	if res.GetOk() || res.GetError() != rules.UnknownTarget("npc-far").Error() {
		t.Errorf("after pc-bay was revoked: ok %v, answer %q, want %q",
			res.GetOk(), res.GetError(), rules.UnknownTarget("npc-far").Error())
	}
	if after := qaasHead(t, tbl.camp); after != before {
		t.Errorf("refused command appended %d events", after-before)
	}
}

// VTT-472 VTT-453
func TestQAASACasterOutOfTheShouldersSightCannotAimEvenAtItself(t *testing.T) {
	far := qaasUse("pc-bay", "shot", "npc-far")
	qaasIsUnknownTarget(t, qaasRefusedAsUnknown(t, "pat", "", far, "npc-far"), "npc-far")
	r := qaasIssue(t, nil, "pat", "", qaasUse("pc-bay", "shot", "pc-bay"))
	if r.res.GetOk() || r.res.GetError() != rules.UnknownTarget("pc-bay").Error() {
		t.Errorf("pc-bay on itself, judged by pc-ash: ok %v, answer %q, want %q",
			r.res.GetOk(), r.res.GetError(), rules.UnknownTarget("pc-bay").Error())
	}
	if r.after != r.before {
		t.Errorf("refused command appended %d events", r.after-r.before)
	}
}

// VTT-365 VTT-472
func TestQAASThePlayersShoulderIsSeenAsATarget(t *testing.T) {
	self := qaasUse("pc-ash", "shot", "pc-ash")
	qaasCheckOutcomes(t, qaasResolved(t, qaasIssue(t, nil, "pat", "", self), self), "pc-ash")
}

// VTT-472 VTT-453
func TestQAASAPlayerWhoControlsNoPartyMemberHasNoEyesToAimBy(t *testing.T) {
	near := qaasUse("npc-pet", "shot", "npc-near")
	qaasIsUnknownTarget(t, qaasRefusedAsUnknown(t, "quin", "", near, "npc-near"), "npc-near")
	r := qaasIssue(t, nil, "quin", "", qaasUse("npc-pet", "shot", "npc-pet"))
	if r.res.GetOk() || r.res.GetError() != rules.UnknownTarget("npc-pet").Error() {
		t.Errorf("npc-pet on itself: ok %v, answer %q, want %q",
			r.res.GetOk(), r.res.GetError(), rules.UnknownTarget("npc-pet").Error())
	}
}

// VTT-482
func TestQAASTheDMsAndTheAgentsAbilitiesAreNotJudgedBySight(t *testing.T) {
	for _, who := range []string{"dm", "agent"} {
		for _, cmd := range []*vttv1.ClientCommand{
			qaasUse("pc-ash", "shot", "npc-far"),
			qaasUse("npc-pet", "volley", "npc-zed", "npc-abe"),
		} {
			r := qaasIssue(t, nil, who, "", cmd)
			used := qaasResolved(t, r, cmd)
			qaasCheckOutcomes(t, used, cmd.GetUseAbility().GetTargetIds()...)
			qaasCheckOutcomes(t, r.conn.waitAbilityUsed(), cmd.GetUseAbility().GetTargetIds()...)
		}
	}
}
