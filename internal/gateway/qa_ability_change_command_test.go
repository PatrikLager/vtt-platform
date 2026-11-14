package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

const (
	qaabRulesetDir   = "../../rulesets/dnd45e-minimal"
	qaabAdventureDir = "../../adventures/goblin-ambush"
	qaabToolsJSON    = "../../contract/gen/tools/tools.json"
)

type qaabConn struct {
	t       *testing.T
	ws      *websocket.Conn
	mu      sync.Mutex
	results map[string]*vttv1.CommandResult
	events  []*vttv1.Envelope
	wake    chan struct{}
	next    int
	name    string
}

type qaabTable struct {
	t        *testing.T
	c        *campaign.Campaign
	ids      *identity.DB
	url      string
	dm       *qaabConn
	agent    *qaabConn
	player   *qaabConn
	spect    *qaabConn
	playerID string
	dmID     string
	agentID  string
}

func qaabRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(qaabRulesetDir)
	if err != nil {
		t.Fatalf("rules.Load: %v", err)
	}
	return rs
}

func qaabNewTable(t *testing.T, rs *rules.Ruleset, withAdventure bool) *qaabTable {
	t.Helper()
	dir := t.TempDir()
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })

	srv := gateway.New(c, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	if withAdventure {
		adv, err := adventure.Load(qaabAdventureDir, rs)
		if err != nil {
			t.Fatalf("adventure.Load: %v", err)
		}
		srv = srv.WithAdventures(map[string]*adventure.Adventure{"goblin-ambush": adv})
	}
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)

	tb := &qaabTable{t: t, c: c, ids: ids, url: "ws" + strings.TrimPrefix(hs.URL, "http")}
	dmTok, dmID, err := ids.CreateInvite("Dee", identity.RoleDM)
	if err != nil {
		t.Fatalf("invite dm: %v", err)
	}
	agTok, agID, err := ids.CreateInvite("Agnes", identity.RoleAgent)
	if err != nil {
		t.Fatalf("invite agent: %v", err)
	}
	plTok, plID, err := ids.CreateInvite("Pat", identity.RolePlayer)
	if err != nil {
		t.Fatalf("invite player: %v", err)
	}
	spTok, _, err := ids.CreateInvite("Sam", identity.RoleSpectator)
	if err != nil {
		t.Fatalf("invite spectator: %v", err)
	}
	tb.playerID, tb.dmID, tb.agentID = plID, dmID, agID
	tb.dm = tb.dial(dmTok, "dm")
	tb.agent = tb.dial(agTok, "agent")
	tb.player = tb.dial(plTok, "player")
	tb.spect = tb.dial(spTok, "spectator")
	return tb
}

func (tb *qaabTable) dial(token, name string) *qaabConn {
	t := tb.t
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, tb.url+"/ws?token="+token+"&after=0", nil)
	if err != nil {
		t.Fatalf("dial %s: %v", name, err)
	}
	ws.SetReadLimit(1 << 22)
	qc := &qaabConn{t: t, ws: ws, results: map[string]*vttv1.CommandResult{}, wake: make(chan struct{}, 1), name: name}
	t.Cleanup(func() { _ = ws.Close(websocket.StatusNormalClosure, "") })
	go qc.readLoop()
	return qc
}

func (qc *qaabConn) readLoop() {
	for {
		_, raw, err := qc.ws.Read(context.Background())
		if err != nil {
			return
		}
		var f vttv1.ServerFrame
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, &f); err != nil {
			continue
		}
		qc.mu.Lock()
		switch fr := f.GetFrame().(type) {
		case *vttv1.ServerFrame_Result:
			qc.results[fr.Result.GetRequestId()] = fr.Result
		case *vttv1.ServerFrame_Event:
			qc.events = append(qc.events, fr.Event)
		}
		qc.mu.Unlock()
		select {
		case qc.wake <- struct{}{}:
		default:
		}
	}
}

func (qc *qaabConn) send(cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	t := qc.t
	t.Helper()
	qc.mu.Lock()
	qc.next++
	id := fmt.Sprintf("%s-%d", qc.name, qc.next)
	qc.mu.Unlock()
	cmd.RequestId = id
	raw, err := protojson.Marshal(cmd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := qc.ws.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("%s write: %v", qc.name, err)
	}
	deadline := time.After(10 * time.Second)
	for {
		qc.mu.Lock()
		res, ok := qc.results[id]
		qc.mu.Unlock()
		if ok {
			return res
		}
		select {
		case <-qc.wake:
		case <-time.After(50 * time.Millisecond):
		case <-deadline:
			t.Fatalf("%s: no result for %s", qc.name, id)
		}
	}
}

func (qc *qaabConn) eventCount() int {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	return len(qc.events)
}

func (qc *qaabConn) waitForSequence(seq int64) {
	qc.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		qc.mu.Lock()
		n := len(qc.events)
		var last int64
		if n > 0 {
			last = qc.events[n-1].GetSequence()
		}
		qc.mu.Unlock()
		if last >= seq {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	qc.t.Fatalf("%s never sent sequence %d", qc.name, seq)
}

func (qc *qaabConn) eventsFrom(n int) []*vttv1.Envelope {
	qc.mu.Lock()
	defer qc.mu.Unlock()
	return slices.Clone(qc.events[n:])
}

func (tb *qaabTable) log() []*vttv1.Envelope {
	t := tb.t
	t.Helper()
	ch, unsub, head, err := tb.c.Subscribe(0, 8192)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer unsub()
	var out []*vttv1.Envelope
	if head == 0 {
		return out
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case env, ok := <-ch:
			if !ok {
				t.Fatalf("subscription closed before head %d", head)
			}
			out = append(out, env)
			if env.GetSequence() >= head {
				return out
			}
		case <-deadline:
			t.Fatalf("log not read to head %d", head)
		}
	}
}

func (tb *qaabTable) actor(id string) *vttv1.Actor {
	st := tb.c.State()
	if st == nil {
		tb.t.Fatalf("campaign state is nil")
	}
	return st.Actors[id]
}

func (tb *qaabTable) mustOK(qc *qaabConn, cmd *vttv1.ClientCommand) *vttv1.CommandResult {
	tb.t.Helper()
	res := qc.send(cmd)
	if !res.GetOk() {
		tb.t.Fatalf("%s: command %v refused: %s", qc.name, cmd, res.GetError())
	}
	return res
}

func qaabAdd(id, name string, kind vttv1.ActorKind, abilities ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
		Actor: &vttv1.Actor{ActorId: id, Name: name, Kind: kind, AbilityIds: abilities},
	}}}
}

func qaabGive(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GiveAbility{GiveAbility: &vttv1.GiveAbility{ActorId: actor, AbilityId: ability}}}
}

func qaabTake(actor, ability string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_TakeAbility{TakeAbility: &vttv1.TakeAbility{ActorId: actor, AbilityId: ability}}}
}

func qaabGrant(actor, participant string, kind vttv1.ActorKind) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
		ActorId: actor, ParticipantId: participant, Kind: kind,
	}}}
}

func qaabUse(actor, ability string, targets ...string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{UseAbility: &vttv1.UseAbility{
		ActorId: actor, AbilityId: ability, TargetIds: targets,
	}}}
}

func qaabLoadAdventure() *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_LoadAdventure{LoadAdventure: &vttv1.LoadAdventure{AdventureId: "goblin-ambush"}}}
}

// snapshot is everything of the fold a give or a take must leave alone.
type qaabSnapshot struct {
	actors     map[string]*vttv1.Actor
	tokens     map[string]engine.Token
	conditions map[string][]engine.ActorCondition
	notes      map[string]engine.Note
	sessions   []engine.Session
	scenes     int
}

func (tb *qaabTable) snapshot() qaabSnapshot {
	st := tb.c.State()
	if st == nil {
		tb.t.Fatalf("campaign state is nil")
	}
	return qaabSnapshot{
		actors: st.Actors, tokens: st.Tokens, conditions: st.Conditions,
		notes: st.Notes, sessions: st.Sessions, scenes: len(st.Scenes),
	}
}

// requireOnlyAbilitiesChanged fails unless before and after differ at most in
// the ability list of the actor named.
func (tb *qaabTable) requireOnlyAbilitiesChanged(before, after qaabSnapshot, actorID string) {
	t := tb.t
	t.Helper()
	if len(before.actors) != len(after.actors) {
		t.Fatalf("actor count changed: %d -> %d", len(before.actors), len(after.actors))
	}
	for id, b := range before.actors {
		a := after.actors[id]
		if a == nil {
			t.Fatalf("actor %q disappeared", id)
		}
		bc, ac := proto.Clone(b).(*vttv1.Actor), proto.Clone(a).(*vttv1.Actor)
		if id == actorID {
			bc.AbilityIds, ac.AbilityIds = nil, nil
		}
		if !proto.Equal(bc, ac) {
			t.Fatalf("actor %q changed beyond its ability list:\nbefore %v\nafter  %v", id, b, a)
		}
	}
	if !reflect.DeepEqual(before.tokens, after.tokens) {
		t.Fatalf("tokens changed:\nbefore %v\nafter  %v", before.tokens, after.tokens)
	}
	if !reflect.DeepEqual(before.conditions, after.conditions) {
		t.Fatalf("conditions changed:\nbefore %v\nafter  %v", before.conditions, after.conditions)
	}
	if !reflect.DeepEqual(before.notes, after.notes) {
		t.Fatalf("notes changed")
	}
	if !reflect.DeepEqual(before.sessions, after.sessions) {
		t.Fatalf("sessions changed")
	}
	if before.scenes != after.scenes {
		t.Fatalf("scene count changed")
	}
}

// requireRefusedAppendingNothing sends cmd from qc and fails unless it is
// refused, its text holds every want, the log does not grow and the fold is
// unchanged.
func (tb *qaabTable) requireRefusedAppendingNothing(qc *qaabConn, cmd *vttv1.ClientCommand, want ...string) *vttv1.CommandResult {
	t := tb.t
	t.Helper()
	logBefore := len(tb.log())
	before := tb.snapshot()
	res := qc.send(cmd)
	if res.GetOk() {
		t.Fatalf("%s: %v accepted, want refused", qc.name, cmd)
	}
	if res.GetError() == "" {
		t.Fatalf("%s: refusal of %v carries no text", qc.name, cmd)
	}
	t.Logf("%s refused: %s", qc.name, res.GetError())
	for _, w := range want {
		if !strings.Contains(res.GetError(), w) {
			t.Fatalf("%s: refusal %q does not contain %q", qc.name, res.GetError(), w)
		}
	}
	if res.GetSequence() != 0 {
		t.Fatalf("refusal carries sequence %d", res.GetSequence())
	}
	if got := len(tb.log()); got != logBefore {
		t.Fatalf("refused %v appended: log %d -> %d", cmd, logBefore, got)
	}
	after := tb.snapshot()
	tb.requireOnlyAbilitiesChanged(before, after, "")
	return res
}

// requireAppendsOne sends cmd from qc, requires it accepted with exactly one
// new log entry, and returns that entry.
func (tb *qaabTable) requireAppendsOne(qc *qaabConn, cmd *vttv1.ClientCommand) *vttv1.Envelope {
	t := tb.t
	t.Helper()
	before := tb.log()
	res := qc.send(cmd)
	if !res.GetOk() {
		t.Fatalf("%s: %v refused: %s", qc.name, cmd, res.GetError())
	}
	after := tb.log()
	if len(after) != len(before)+1 {
		t.Fatalf("%s: %v appended %d entries, want 1", qc.name, cmd, len(after)-len(before))
	}
	last := after[len(after)-1]
	if res.GetSequence() != last.GetSequence() {
		t.Fatalf("result sequence %d, appended entry's %d", res.GetSequence(), last.GetSequence())
	}
	return last
}

// VTT-713
func TestQAGiveAbilityFromTheDMAndTheAgentAppendsOneAbilityGivenAndEndsTheList(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally", "longsword-strike", "rally"))

			for _, step := range []struct {
				who     *qaabConn
				ability string
				id      string
				role    string
			}{{tb.dm, "crossbow-shot", tb.dmID, "dm"}, {tb.agent, "staggering-blow", tb.agentID, "agent"}} {
				prior := slices.Clone(tb.actor("a1").GetAbilityIds())
				env := tb.requireAppendsOne(step.who, qaabGive("a1", step.ability))
				g := env.GetAbilityGiven()
				if g == nil {
					t.Fatalf("%s: appended %T, want AbilityGiven", step.who.name, env.GetPayload())
				}
				if g.GetActorId() != "a1" || g.GetAbilityId() != step.ability {
					t.Fatalf("AbilityGiven = %v, want a1/%s", g, step.ability)
				}
				if env.GetParticipantId() != step.id || env.GetActorRole() != step.role {
					t.Fatalf("AbilityGiven stamped %q/%q, want %q/%q", env.GetParticipantId(), env.GetActorRole(), step.id, step.role)
				}
				got := tb.actor("a1").GetAbilityIds()
				want := append(prior, step.ability)
				if !slices.Equal(got, want) {
					t.Fatalf("after %s's give: ability_ids %v, want %v", step.who.name, got, want)
				}
			}
		})
	}
}

// VTT-714
func TestQATakeAbilityFromTheDMAndTheAgentRemovesEveryOccurrenceAndKeepsTheOrder(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
				"rally", "longsword-strike", "rally", "crossbow-shot", "staggering-blow", "rally"))

			env := tb.requireAppendsOne(tb.dm, qaabTake("a1", "rally"))
			tk := env.GetAbilityTaken()
			if tk == nil || tk.GetActorId() != "a1" || tk.GetAbilityId() != "rally" {
				t.Fatalf("appended %v, want AbilityTaken a1/rally", env.GetPayload())
			}
			if got, want := tb.actor("a1").GetAbilityIds(), []string{"longsword-strike", "crossbow-shot", "staggering-blow"}; !slices.Equal(got, want) {
				t.Fatalf("after DM's take: %v, want %v", got, want)
			}

			env = tb.requireAppendsOne(tb.agent, qaabTake("a1", "crossbow-shot"))
			tk = env.GetAbilityTaken()
			if tk == nil || tk.GetActorId() != "a1" || tk.GetAbilityId() != "crossbow-shot" {
				t.Fatalf("appended %v, want AbilityTaken a1/crossbow-shot", env.GetPayload())
			}
			if got, want := tb.actor("a1").GetAbilityIds(), []string{"longsword-strike", "staggering-blow"}; !slices.Equal(got, want) {
				t.Fatalf("after agent's take: %v, want %v", got, want)
			}
		})
	}
}

// VTT-713, VTT-714: nothing else of the actor or of the world changes.
func TestQAGiveAndTakeChangeNothingButTheActorsAbilityList(t *testing.T) {
	rs := qaabRuleset(t)
	tb := qaabNewTable(t, rs, true)
	tb.mustOK(tb.dm, qaabLoadAdventure())
	tb.mustOK(tb.dm, qaabGrant("act-fighter", tb.playerID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	tb.mustOK(tb.dm, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "s1"}}})

	before := tb.snapshot()
	tb.requireAppendsOne(tb.dm, qaabGive("act-fighter", "goblin-scimitar"))
	mid := tb.snapshot()
	tb.requireOnlyAbilitiesChanged(before, mid, "act-fighter")
	if !slices.Contains(mid.actors["act-fighter"].GetAbilityIds(), "goblin-scimitar") {
		t.Fatalf("give did not reach the sheet: %v", mid.actors["act-fighter"].GetAbilityIds())
	}

	tb.requireAppendsOne(tb.agent, qaabTake("act-fighter", "rally"))
	after := tb.snapshot()
	tb.requireOnlyAbilitiesChanged(mid, after, "act-fighter")
	if slices.Contains(after.actors["act-fighter"].GetAbilityIds(), "rally") {
		t.Fatalf("take did not reach the sheet: %v", after.actors["act-fighter"].GetAbilityIds())
	}
}

// VTT-716
func TestQAGiveAndTakeFromAPlayerOrASpectatorAreRefusedAsNotAuthorized(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally"))
			tb.mustOK(tb.dm, qaabAdd("a2", "Beta", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "rally"))
			tb.mustOK(tb.dm, qaabGrant("a1", tb.playerID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))

			for _, qc := range []*qaabConn{tb.player, tb.spect} {
				for _, actor := range []string{"a1", "a2"} {
					tb.requireRefusedAppendingNothing(qc, qaabGive(actor, "crossbow-shot"), "not authorized")
					tb.requireRefusedAppendingNothing(qc, qaabTake(actor, "rally"), "not authorized")
				}
			}
			if got := tb.actor("a1").GetAbilityIds(); !slices.Equal(got, []string{"rally"}) {
				t.Fatalf("a1's sheet changed: %v", got)
			}
		})
	}
}

// VTT-717
func TestQAGiveAndTakeNamingAnActorTheCampaignDoesNotHoldAreRefused(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally"))
			for _, qc := range []*qaabConn{tb.dm, tb.agent} {
				tb.requireRefusedAppendingNothing(qc, qaabGive("nobody", "crossbow-shot"))
				tb.requireRefusedAppendingNothing(qc, qaabTake("nobody", "rally"))
				tb.requireRefusedAppendingNothing(qc, qaabGive("", "crossbow-shot"))
				tb.requireRefusedAppendingNothing(qc, qaabTake("", "rally"))
			}
		})
	}
}

// VTT-717
func TestQAEngineApplyRefusesAbilityEventsForAnActorItDoesNotHold(t *testing.T) {
	st := engine.NewState()
	add := &vttv1.Envelope{EventId: "e1", Sequence: 1, Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, AbilityIds: []string{"x"}},
	}}}
	if err := engine.Apply(st, add); err != nil {
		t.Fatalf("ActorAdded: %v", err)
	}
	give := &vttv1.Envelope{EventId: "e2", Sequence: 2, Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{ActorId: "nobody", AbilityId: "y"}}}
	if err := engine.Apply(st, give); err == nil {
		t.Fatalf("AbilityGiven for an unknown actor applied")
	}
	take := &vttv1.Envelope{EventId: "e3", Sequence: 3, Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{ActorId: "nobody", AbilityId: "x"}}}
	if err := engine.Apply(st, take); err == nil {
		t.Fatalf("AbilityTaken for an unknown actor applied")
	}
	if _, ok := st.Actors["nobody"]; ok {
		t.Fatalf("a refused event created an actor")
	}
	if got := st.Actors["a1"].GetAbilityIds(); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("a1 changed: %v", got)
	}
}

// VTT-718
func TestQAGiveOfAnIdTheServedRulesetDoesNotHoldIsRefusedNamingTheActorAndTheId(t *testing.T) {
	tb := qaabNewTable(t, qaabRuleset(t), false)
	tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally"))
	for _, qc := range []*qaabConn{tb.dm, tb.agent} {
		for _, id := range []string{"no-such-ability", "Rally", "rally ", "goblin-scimitar-x"} {
			tb.requireRefusedAppendingNothing(qc, qaabGive("a1", id), `"a1"`, `"`+id+`"`)
		}
	}
}

// VTT-720
func TestQAGiveOfAnIdTheSheetAlreadyNamesIsRefused(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally", "longsword-strike"))
			for _, qc := range []*qaabConn{tb.dm, tb.agent} {
				tb.requireRefusedAppendingNothing(qc, qaabGive("a1", "rally"))
				tb.requireRefusedAppendingNothing(qc, qaabGive("a1", "longsword-strike"))
			}
			tb.requireAppendsOne(tb.dm, qaabGive("a1", "crossbow-shot"))
			tb.requireRefusedAppendingNothing(tb.agent, qaabGive("a1", "crossbow-shot"))
		})
	}
}

// VTT-721
func TestQATakeOfAnIdTheSheetDoesNotNameIsRefused(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally"))
			tb.mustOK(tb.dm, qaabAdd("a2", "Beta", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
			for _, qc := range []*qaabConn{tb.dm, tb.agent} {
				tb.requireRefusedAppendingNothing(qc, qaabTake("a1", "crossbow-shot"))
				tb.requireRefusedAppendingNothing(qc, qaabTake("a1", "no-such-ability"))
				tb.requireRefusedAppendingNothing(qc, qaabTake("a1", ""))
				tb.requireRefusedAppendingNothing(qc, qaabTake("a2", "rally"))
			}
			tb.requireAppendsOne(tb.dm, qaabTake("a1", "rally"))
			tb.requireRefusedAppendingNothing(tb.agent, qaabTake("a1", "rally"))
		})
	}
}

// VTT-722
func TestQAGiveOfAnEmptyIdIsRefusedWithOrWithoutARuleset(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally"))
			for _, qc := range []*qaabConn{tb.dm, tb.agent} {
				tb.requireRefusedAppendingNothing(qc, qaabGive("a1", ""))
			}
		})
	}
}

// VTT-722
func TestQAEngineApplyRefusesAnAbilityGivenWithAnEmptyId(t *testing.T) {
	st := engine.NewState()
	add := &vttv1.Envelope{EventId: "e1", Sequence: 1, Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{ActorId: "a1", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
	}}}
	if err := engine.Apply(st, add); err != nil {
		t.Fatalf("ActorAdded: %v", err)
	}
	give := &vttv1.Envelope{EventId: "e2", Sequence: 2, Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{ActorId: "a1"}}}
	if err := engine.Apply(st, give); err == nil {
		t.Fatalf("AbilityGiven with an empty id applied")
	}
	if got := st.Actors["a1"].GetAbilityIds(); len(got) != 0 {
		t.Fatalf("a1 changed: %v", got)
	}
}

// VTT-745
func TestQAWithNoRulesetServedAGiveOfAnyNonEmptyIdIsAccepted(t *testing.T) {
	tb := qaabNewTable(t, nil, false)
	tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	want := []string{}
	for i, id := range []string{"not-in-any-ruleset", " ", "ä", "Rally", strings.Repeat("z", 300), "\x00"} {
		qc := tb.dm
		if i%2 == 1 {
			qc = tb.agent
		}
		env := tb.requireAppendsOne(qc, qaabGive("a1", id))
		if g := env.GetAbilityGiven(); g == nil || g.GetAbilityId() != id {
			t.Fatalf("give %q appended %v", id, env.GetPayload())
		}
		want = append(want, id)
		if got := tb.actor("a1").GetAbilityIds(); !slices.Equal(got, want) {
			t.Fatalf("after give %q: %q, want %q", id, got, want)
		}
	}
}

// VTT-713 VTT-718
func TestQAAGiveRefusalReachesTheIssuerAndTheConnectionGoesOn(t *testing.T) {
	tb := qaabNewTable(t, qaabRuleset(t), false)
	added := tb.mustOK(tb.dm, qaabAdd("a1", "Alpha", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "rally"))
	tb.agent.waitForSequence(added.GetSequence())
	mark := tb.agent.eventCount()
	tb.requireRefusedAppendingNothing(tb.dm, qaabGive("a1", "no-such-ability"), "no-such-ability")
	env := tb.requireAppendsOne(tb.dm, qaabGive("a1", "crossbow-shot"))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && tb.agent.eventCount() < mark+1 {
		time.Sleep(20 * time.Millisecond)
	}
	got := tb.agent.eventsFrom(mark)
	if len(got) != 1 || got[0].GetAbilityGiven() == nil || got[0].GetSequence() != env.GetSequence() {
		t.Fatalf("agent was sent %v after the refusal, want the one accepted AbilityGiven", got)
	}
}

// VTT-713 VTT-714
func TestQAAUseFollowsTheSheetAfterAGiveAndATake(t *testing.T) {
	rs := qaabRuleset(t)
	tb := qaabNewTable(t, rs, true)
	tb.mustOK(tb.dm, qaabLoadAdventure())
	tb.mustOK(tb.dm, qaabGrant("act-fighter", tb.playerID, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))

	if res := tb.dm.send(qaabUse("act-cutter", "rally", "act-fighter")); res.GetOk() {
		t.Fatalf("fixture: act-cutter already uses rally")
	}
	tb.mustOK(tb.dm, qaabGive("act-cutter", "rally"))
	tb.mustOK(tb.dm, qaabUse("act-cutter", "rally", "act-fighter"))

	if res := tb.agent.send(qaabUse("act-cutter", "crossbow-shot", "act-fighter")); res.GetOk() {
		t.Fatalf("fixture: act-cutter already uses crossbow-shot")
	}
	tb.mustOK(tb.agent, qaabGive("act-cutter", "crossbow-shot"))
	tb.mustOK(tb.agent, qaabUse("act-cutter", "crossbow-shot", "act-fighter"))

	tb.mustOK(tb.dm, qaabUse("act-fighter", "longsword-strike", "act-cutter"))
	tb.mustOK(tb.dm, qaabTake("act-fighter", "longsword-strike"))
	tb.requireRefusedAppendingNothing(tb.dm, qaabUse("act-fighter", "longsword-strike", "act-cutter"), "longsword-strike")
	tb.requireRefusedAppendingNothing(tb.agent, qaabUse("act-fighter", "longsword-strike", "act-cutter"), "longsword-strike")
	tb.requireRefusedAppendingNothing(tb.player, qaabUse("act-fighter", "longsword-strike", "act-cutter"), "longsword-strike")

	tb.mustOK(tb.player, qaabUse("act-fighter", "rally", "act-cutter"))
	tb.mustOK(tb.agent, qaabTake("act-fighter", "rally"))
	tb.requireRefusedAppendingNothing(tb.player, qaabUse("act-fighter", "rally", "act-cutter"), "rally")
	tb.requireRefusedAppendingNothing(tb.agent, qaabUse("act-fighter", "rally", "act-cutter"), "rally")

	if res := tb.player.send(qaabUse("act-fighter", "goblin-scimitar", "act-cutter")); res.GetOk() {
		t.Fatalf("fixture: act-fighter already uses goblin-scimitar")
	}
	tb.mustOK(tb.dm, qaabGive("act-fighter", "goblin-scimitar"))
	tb.mustOK(tb.player, qaabUse("act-fighter", "goblin-scimitar", "act-cutter"))
}

// VTT-729: an actor that names none can use none until give_ability gives it one.
func TestQAAnActorNamingNoAbilityUsesNoneUntilOneIsGiven(t *testing.T) {
	rs := qaabRuleset(t)
	tb := qaabNewTable(t, rs, true)
	tb.mustOK(tb.dm, qaabLoadAdventure())
	tb.mustOK(tb.dm, qaabAdd("bare", "Bare", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	tb.mustOK(tb.dm, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
		TokenId: "tok-bare", SceneId: "ravine", ActorId: "bare", Position: &vttv1.GridPosition{X: 0, Y: 1},
	}}})
	for _, qc := range []*qaabConn{tb.dm, tb.agent} {
		tb.requireRefusedAppendingNothing(qc, qaabUse("bare", "rally", "act-fighter"), `"bare"`, "rally")
	}
	tb.mustOK(tb.agent, qaabGive("bare", "rally"))
	tb.mustOK(tb.dm, qaabUse("bare", "rally", "act-fighter"))
	tb.mustOK(tb.agent, qaabUse("bare", "rally", "act-fighter"))
	tb.requireRefusedAppendingNothing(tb.dm, qaabUse("bare", "longsword-strike", "act-fighter"), "longsword-strike")
}

// VTT-746
func TestQAWithNoRulesetServedAddActorRefusesNoNonEmptyAbilityIds(t *testing.T) {
	tb := qaabNewTable(t, nil, false)
	cases := [][]string{
		{"not-in-any-ruleset"},
		{"dup", "dup", "Dup"},
		{" ", "ä", strings.Repeat("y", 300)},
		{},
	}
	for i, ids := range cases {
		id := fmt.Sprintf("n%d", i)
		tb.requireAppendsOne(tb.dm, qaabAdd(id, "", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, ids...))
		if got := tb.actor(id).GetAbilityIds(); !slices.Equal(got, ids) {
			t.Fatalf("%s: ability_ids %q, want %q", id, got, ids)
		}
	}
}

// VTT-747
func TestQAAddActorRefusesAnEmptyAbilityIdWithOrWithoutARuleset(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)
			for i, ids := range [][]string{{""}, {"rally", ""}, {"", "rally"}, {"rally", "", "rally"}} {
				id := fmt.Sprintf("e%d", i)
				for _, qc := range []*qaabConn{tb.dm, tb.agent} {
					tb.requireRefusedAppendingNothing(qc, qaabAdd(id, "E", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, ids...),
						`"`+id+`"`, "ability_ids")
				}
				if tb.actor(id) != nil {
					t.Fatalf("%s was added", id)
				}
			}
		})
	}
}

// VTT-747: the order of add_actor's refusals around the empty id.
func TestQAAddActorRefusesAnEmptyAbilityIdAfterItsControllerKindAndKeyRefusals(t *testing.T) {
	for _, served := range []bool{true, false} {
		t.Run(fmt.Sprintf("ruleset served %v", served), func(t *testing.T) {
			var rs *rules.Ruleset
			if served {
				rs = qaabRuleset(t)
			}
			tb := qaabNewTable(t, rs, false)

			ctl := qaabAdd("o1", "O", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "")
			ctl.GetAddActor().GetActor().ControllerId = tb.playerID
			res := tb.requireRefusedAppendingNothing(tb.dm, ctl, "grant_actor_control")
			if strings.Contains(res.GetError(), "ability_ids") {
				t.Fatalf("controller refusal names ability_ids: %q", res.GetError())
			}

			ctls := qaabAdd("o2", "O", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "")
			ctls.GetAddActor().GetActor().ControllerIds = []string{""}
			res = tb.requireRefusedAppendingNothing(tb.dm, ctls, "grant_actor_control")
			if strings.Contains(res.GetError(), "ability_ids") {
				t.Fatalf("controller refusal names ability_ids: %q", res.GetError())
			}

			kind := qaabAdd("o3", "O", vttv1.ActorKind_ACTOR_KIND_UNSPECIFIED, "")
			res = tb.requireRefusedAppendingNothing(tb.dm, kind, "kind")
			if strings.Contains(res.GetError(), "ability_ids") {
				t.Fatalf("kind refusal names ability_ids: %q", res.GetError())
			}

			keys := qaabAdd("o4", "O", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "")
			keys.GetAddActor().GetActor().Attributes = map[string]int32{"qa-undeclared-key": 1}
			if served {
				res = tb.requireRefusedAppendingNothing(tb.dm, keys, "qa-undeclared-key")
				if strings.Contains(res.GetError(), "ability_ids") {
					t.Fatalf("key refusal names ability_ids: %q", res.GetError())
				}
			} else {
				tb.requireRefusedAppendingNothing(tb.dm, keys, `"o4"`, "ability_ids")
			}

			noID := qaabAdd("", "O", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "")
			res = tb.requireRefusedAppendingNothing(tb.dm, noID)
			if strings.Contains(res.GetError(), "ability_ids") {
				t.Fatalf("an actor with no id is refused for its ability ids: %q", res.GetError())
			}

			if served {
				before := qaabAdd("o5", "O", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "zzz-no-such-ability", "")
				res = tb.requireRefusedAppendingNothing(tb.dm, before, `"o5"`, "ability_ids")
				if strings.Contains(res.GetError(), "zzz-no-such-ability") || strings.Contains(res.GetError(), "does not hold") {
					t.Fatalf("empty id refused after the ruleset read: %q", res.GetError())
				}
			}
		})
	}
}

type qaabTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func qaabTools(t *testing.T) map[string]qaabTool {
	t.Helper()
	raw, err := os.ReadFile(qaabToolsJSON)
	if err != nil {
		t.Fatalf("read tools: %v", err)
	}
	var list []qaabTool
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode tools: %v", err)
	}
	out := map[string]qaabTool{}
	for _, tl := range list {
		out[tl.Name] = tl
	}
	return out
}

func qaabRequireAll(t *testing.T, what, text string, phrases ...string) {
	t.Helper()
	for _, p := range phrases {
		if !strings.Contains(text, p) {
			t.Errorf("%s does not say %q:\n%s", what, p, text)
		}
	}
}

// VTT-728
func TestQAGiveAndTakeToolsSayWhoMayWhatEachRefusesAndThatNothingElseChanges(t *testing.T) {
	tools := qaabTools(t)
	give, ok := tools["give_ability"]
	if !ok {
		t.Fatalf("no give_ability tool")
	}
	take, ok := tools["take_ability"]
	if !ok {
		t.Fatalf("no take_ability tool")
	}
	qaabRequireAll(t, "give_ability", give.Description,
		"DM/agent only",
		"Only its ability list changes",
		"tokens, controllers, conditions and resources stay as they are",
		"an actor the campaign does not hold",
		"an ability its sheet already names",
		"an empty ability id",
		"with a ruleset served, an id the ruleset does not hold",
		"Refused, appending nothing",
	)
	qaabRequireAll(t, "take_ability", take.Description,
		"DM/agent only",
		"Only its ability list changes",
		"an actor the campaign does not hold",
		"an ability its sheet does not name",
		"Refused, appending nothing",
	)
	for _, tl := range []qaabTool{give, take} {
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(tl.InputSchema, &schema); err != nil {
			t.Fatalf("%s schema: %v", tl.Name, err)
		}
		if !slices.Contains(schema.Required, "actorId") || !slices.Contains(schema.Required, "abilityId") {
			t.Errorf("%s requires %v, want actorId and abilityId", tl.Name, schema.Required)
		}
	}
}

// VTT-729, VTT-749
func TestQAAddActorSchemaSaysWhatAnAbilityListDoesInPlay(t *testing.T) {
	tools := qaabTools(t)
	add, ok := tools["add_actor"]
	if !ok {
		t.Fatalf("no add_actor tool")
	}
	var schema struct {
		Properties struct {
			Actor struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
			} `json:"actor"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(add.InputSchema, &schema); err != nil {
		t.Fatalf("add_actor schema: %v", err)
	}
	doc := schema.Properties.Actor.Properties["abilityIds"].Description
	qaabRequireAll(t, "add_actor's abilityIds (VTT-729)", doc,
		"use_ability refuses this actor any ability its sheet does not name",
		"an actor that names none can use none until give_ability gives it one",
		"give_ability and take_ability change the list in play",
	)
	qaabRequireAll(t, "add_actor's abilityIds (VTT-749)", doc,
		"an empty id refuses it with or without one",
	)
}
