package gateway_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

type rnRuleset struct{ id, version, name, fists string }

var (
	rnR        = rnRuleset{id: "ruleset-r", version: "1", name: "Ruleset R"}
	rnRNext    = rnRuleset{id: "ruleset-r", version: "2", name: "Ruleset R"}
	rnRRenamed = rnRuleset{id: "ruleset-r", version: "2", name: "Ruleset R, renamed", fists: "Another description."}
	rnQ        = rnRuleset{id: "ruleset-q", version: "2", name: "Ruleset Q"}
)

func (r rnRuleset) entry() *vttv1.RulesetNamed {
	return &vttv1.RulesetNamed{RulesetId: r.id, Version: r.version, Name: r.name}
}

func rnEdit(t *testing.T, path string, fields map[string]string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	for k, v := range fields {
		doc[k] = v
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

// load reads a copy of tavern-brawl written under r's id, version and name.
func (r rnRuleset) load(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ruleset")
	if err := os.CopyFS(dir, os.DirFS(tavernBrawlDir(t))); err != nil {
		t.Fatal(err)
	}
	rnEdit(t, filepath.Join(dir, "ruleset.json"), map[string]string{"id": r.id, "version": r.version, "name": r.name})
	if r.fists != "" {
		rnEdit(t, filepath.Join(dir, "abilities", "fists.json"), map[string]string{"description": r.fists})
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load(%s %s): %v", r.id, r.version, err)
	}
	if rs.ID != r.id || rs.Version != r.version || rs.Name != r.name {
		t.Fatalf("loaded %q %q %q, want %q %q %q", rs.ID, rs.Version, rs.Name, r.id, r.version, r.name)
	}
	if r.fists != "" && rs.Compiled["fists"].Description != r.fists {
		t.Fatalf("fists' description is %q, want %q", rs.Compiled["fists"].Description, r.fists)
	}
	return rs
}

type rnCampaign struct {
	dir                   string
	dmToken, dmID         string
	playerToken, playerID string
}

func newRNCampaign(t *testing.T) *rnCampaign {
	t.Helper()
	rc := &rnCampaign{dir: t.TempDir()}
	c, err := campaign.Open(rc.dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(campaign.LogPath(rc.dir))
	if err != nil {
		t.Fatal(err)
	}
	if rc.dmToken, rc.dmID, err = ids.CreateInvite("DM", identity.RoleDM); err != nil {
		t.Fatal(err)
	}
	if rc.playerToken, rc.playerID, err = ids.CreateInvite("Player", identity.RoleSpectator); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, c, "rn-1", &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "s"}})
	mustAppend(t, c, "rn-2", &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "room", Name: "Room", GridWidth: 5, GridHeight: 5,
	}})
	mustAppend(t, c, "rn-3", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "brawler", Name: "Brawler", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		Attributes: map[string]int32{"brawn": 3, "grit": 1}, AbilityIds: []string{"fists"},
	}}})
	mustAppend(t, c, "rn-4", &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: "brawler", ParticipantId: rc.playerID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}})
	mustAppend(t, c, "rn-5", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "patron", Name: "Patron", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
		Attributes: map[string]int32{"footing": 0}, Resources: map[string]*vttv1.Resource{"drink": {Max: 20}},
	}}})
	mustAppend(t, c, "rn-6", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "tok-brawler", SceneId: "room", ActorId: "brawler", Position: &vttv1.GridPosition{X: 0, Y: 0},
	}})
	mustAppend(t, c, "rn-7", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "tok-patron", SceneId: "room", ActorId: "patron", Position: &vttv1.GridPosition{X: 1, Y: 0},
	}})
	if err := ids.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	return rc
}

type rnServed struct {
	t     *testing.T
	c     *campaign.Campaign
	ids   *identity.DB
	hs    *httptest.Server
	conns []*websocket.Conn
}

// serve opens the campaign directory under rs, or under none when nil; close
// it before the next serve, since a directory has one writer (SPEC-019).
func (rc *rnCampaign) serve(t *testing.T, rs *rules.Ruleset) *rnServed {
	t.Helper()
	c, err := campaign.Open(rc.dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := identity.Open(campaign.LogPath(rc.dir))
	if err != nil {
		t.Fatal(err)
	}
	srv := gateway.New(c, ids)
	if rs != nil {
		srv = srv.WithRuleset(rs)
	}
	s := &rnServed{t: t, c: c, ids: ids, hs: httptest.NewServer(srv.Handler())}
	t.Cleanup(s.close)
	return s
}

func (s *rnServed) close() {
	if s.hs == nil {
		return
	}
	for _, conn := range s.conns {
		conn.CloseNow()
	}
	s.hs.Close()
	s.hs = nil
	if err := s.ids.Close(); err != nil {
		s.t.Error(err)
	}
	if err := s.c.Close(); err != nil {
		s.t.Error(err)
	}
}

func (s *rnServed) dial(token string, after int64) *websocket.Conn {
	s.t.Helper()
	f := &rulesetFixture{t: s.t, srv: s.hs}
	conn := f.dial(token, after)
	s.conns = append(s.conns, conn)
	return conn
}

func (s *rnServed) logAfter(after int64) []*vttv1.Envelope {
	s.t.Helper()
	return qaUseEventsAfter(s.t, s.c, after)
}

func (s *rnServed) head() int64 {
	s.t.Helper()
	return qaUseHead(s.t, s.c)
}

// use sends fists as the holder of token and returns its result and what it
// appended.
func (s *rnServed) use(token string) (*vttv1.CommandResult, []*vttv1.Envelope) {
	s.t.Helper()
	before := s.head()
	conn := s.dial(token, before)
	sendCommand(s.t, conn, fistsCmd("brawler", "patron"))
	res := readResult(s.t, conn)
	if !res.GetOk() {
		s.t.Fatalf("use refused: %q", res.GetError())
	}
	return res, s.logAfter(before)
}

func rnWantEntry(t *testing.T, label string, res *vttv1.CommandResult, batch []*vttv1.Envelope, want *vttv1.RulesetNamed) {
	t.Helper()
	if len(batch) < 2 {
		t.Fatalf("%s: the use appended %d entries, want the ruleset's and then the outcome", label, len(batch))
	}
	if got := batch[0].GetRulesetNamed(); !proto.Equal(got, want) {
		t.Fatalf("%s: the batch begins with %v, want RulesetNamed %v", label, batch[0].GetPayload(), want)
	}
	if res.GetSequence() != batch[0].GetSequence() {
		t.Fatalf("%s: the result answers %d, want the entry's %d", label, res.GetSequence(), batch[0].GetSequence())
	}
	if batch[1].GetAbilityUsed() == nil {
		t.Fatalf("%s: the entry is followed by %v, want the AbilityUsed", label, batch[1].GetPayload())
	}
	for _, env := range batch[1:] {
		if env.GetRulesetNamed() != nil {
			t.Fatalf("%s: a second RulesetNamed at %d", label, env.GetSequence())
		}
	}
}

func rnWantNoEntry(t *testing.T, label string, res *vttv1.CommandResult, batch []*vttv1.Envelope) {
	t.Helper()
	if len(batch) == 0 || batch[0].GetAbilityUsed() == nil {
		t.Fatalf("%s: the batch does not begin with the AbilityUsed: %v", label, batch)
	}
	if res.GetSequence() != batch[0].GetSequence() {
		t.Fatalf("%s: the result answers %d, want the AbilityUsed's %d", label, res.GetSequence(), batch[0].GetSequence())
	}
	for _, env := range batch {
		if env.GetRulesetNamed() != nil {
			t.Fatalf("%s: a RulesetNamed %v at %d", label, env.GetRulesetNamed(), env.GetSequence())
		}
	}
}

// VTT-975
func TestAUseNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother(t *testing.T) {
	rc := newRNCampaign(t)
	for _, step := range []struct {
		label  string
		served rnRuleset
	}{
		{"the log names none", rnR},
		{"another version", rnRNext},
		{"another id at the version the log names", rnQ},
	} {
		s := rc.serve(t, step.served.load(t))
		res, batch := s.use(rc.dmToken)
		rnWantEntry(t, step.label, res, batch, step.served.entry())
		s.close()
	}
}

// VTT-978
func TestAUseUnderTheRulesetTheLogNamesAppendsNoEntry(t *testing.T) {
	rc := newRNCampaign(t)
	s := rc.serve(t, rnR.load(t))
	res, batch := s.use(rc.dmToken)
	rnWantEntry(t, "the first use", res, batch, rnR.entry())
	res, batch = s.use(rc.dmToken)
	rnWantNoEntry(t, "a second use", res, batch)
	s.close()
	for _, step := range []struct {
		label  string
		served rnRuleset
		want   *vttv1.RulesetNamed
	}{
		{"reopened under the same", rnR, nil},
		{"reopened under another version", rnRNext, rnRNext.entry()},
		{"reopened renamed, with fists described otherwise", rnRRenamed, nil},
	} {
		s := rc.serve(t, step.served.load(t))
		res, batch := s.use(rc.dmToken)
		if step.want != nil {
			rnWantEntry(t, step.label, res, batch, step.want)
		} else {
			rnWantNoEntry(t, step.label, res, batch)
		}
		s.close()
	}
}

// VTT-976 VTT-272
func TestTheEntryCarriesItsIssuerToTheDMAndNoneToAPlayer(t *testing.T) {
	rc := newRNCampaign(t)
	for _, step := range []struct {
		served         rnRuleset
		issuer, player bool
	}{
		{rnR, true, true},
		{rnRNext, false, false},
	} {
		s := rc.serve(t, step.served.load(t))
		dm := s.dial(rc.dmToken, s.head())
		token, wantID, wantRole := rc.dmToken, rc.dmID, string(identity.RoleDM)
		if step.issuer {
			token, wantID, wantRole = rc.playerToken, rc.playerID, string(identity.RolePlayer)
		}
		var player *websocket.Conn
		if step.player {
			player = s.dial(rc.playerToken, 0)
		}
		issuer := s.dial(token, 0)
		sendCommand(t, issuer, fistsCmd("brawler", "patron"))
		res := readResult(t, issuer)
		if !res.GetOk() {
			t.Fatalf("use as %s refused: %q", wantRole, res.GetError())
		}
		got := readEvent(t, dm)
		if got.GetRulesetNamed() == nil || got.GetSequence() != res.GetSequence() {
			t.Fatalf("the DM's stream opens the batch with %v at %d, want the entry at %d", got.GetPayload(), got.GetSequence(), res.GetSequence())
		}
		if got.GetParticipantId() != wantID || got.GetActorRole() != wantRole || got.GetEventId() == "" || got.GetOccurredAt() == nil {
			t.Errorf("issued as %s: the DM's entry carries participant %q, role %q, event id %q, time %v; want %q, %q, an id and a time",
				wantRole, got.GetParticipantId(), got.GetActorRole(), got.GetEventId(), got.GetOccurredAt(), wantID, wantRole)
		}
		if player != nil {
			for {
				env := readEvent(t, player)
				if env.GetRulesetNamed() == nil {
					continue
				}
				if env.GetParticipantId() != "" || env.GetActorRole() != "" {
					t.Errorf("the player's entry carries participant %q, role %q, want neither", env.GetParticipantId(), env.GetActorRole())
				}
				break
			}
		}
		s.close()
	}
}

// VTT-132
func TestAUseWithNoRulesetServedIsRefusedAndTheLogIsUnchanged(t *testing.T) {
	rc := newRNCampaign(t)
	s := rc.serve(t, rnR.load(t))
	res, batch := s.use(rc.dmToken)
	rnWantEntry(t, "the first use", res, batch, rnR.entry())
	s.close()

	s = rc.serve(t, nil)
	before := s.head()
	conn := s.dial(rc.dmToken, before)
	sendCommand(t, conn, fistsCmd("brawler", "patron"))
	if r := readResult(t, conn); r.GetOk() || r.GetError() != "gateway: no ruleset loaded" {
		t.Fatalf("with no ruleset served the use answered ok=%v %q, want the refusal", r.GetOk(), r.GetError())
	}
	if after := s.head(); after != before {
		t.Fatalf("the refused use moved the head %d -> %d", before, after)
	}
	if st := s.c.State(); st.RulesetID != rnR.id || st.RulesetVersion != rnR.version {
		t.Fatalf("the log names %q %q, want %q %q", st.RulesetID, st.RulesetVersion, rnR.id, rnR.version)
	}
	sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}}})
	if r := readResult(t, conn); !r.GetOk() {
		t.Fatalf("the connection did not take the next command: %q", r.GetError())
	}
}

// newRNTakeCampaign adds an actor per step holding chair-swing and the stamina it
// spends, so a take of it removes the resource the ruleset chooses.
func newRNTakeCampaign(t *testing.T) *rnCampaign {
	t.Helper()
	rc := newRNCampaign(t)
	c, err := campaign.Open(rc.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"s1", "s2", "s3"} {
		mustAppend(t, c, "rn-take-"+id, &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: &vttv1.Actor{
			ActorId: id, Name: "Swinger " + id, Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			AbilityIds: []string{"chair-swing"}, Resources: map[string]*vttv1.Resource{"stamina": {Current: 2, Max: 2}},
		}}})
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	return rc
}

// command sends cmd as the holder of token and returns its result and what
// it appended; a refusal fails the test.
func (s *rnServed) command(token string, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, []*vttv1.Envelope) {
	s.t.Helper()
	before := s.head()
	conn := s.dial(token, before)
	sendCommand(s.t, conn, cmd)
	res := readResult(s.t, conn)
	if !res.GetOk() {
		s.t.Fatalf("%v refused: %q", cmd.GetCommand(), res.GetError())
	}
	return res, s.logAfter(before)
}

func rnTake(actorID, abilityID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_TakeAbility{
		TakeAbility: &vttv1.TakeAbility{ActorId: actorID, AbilityId: abilityID}}}
}

// VTT-988 VTT-799 VTT-992
func TestATakeNamesTheServedRulesetFirstWhenTheLogNamesNoneOrAnother(t *testing.T) {
	rc := newRNTakeCampaign(t)
	for _, step := range []struct {
		label  string
		served rnRuleset
		actor  string
	}{
		{"the log names none", rnR, "s1"},
		{"another version", rnRNext, "s2"},
		{"another id at the version the log names", rnQ, "s3"},
	} {
		s := rc.serve(t, step.served.load(t))
		res, batch := s.command(rc.dmToken, rnTake(step.actor, "chair-swing"))
		if len(batch) != 3 {
			t.Fatalf("%s: the take appended %v, want the entry, the AbilityTaken and the ResourceRemoved", step.label, batch)
		}
		if got := batch[0].GetRulesetNamed(); !proto.Equal(got, step.served.entry()) {
			t.Fatalf("%s: the batch begins with %v, want RulesetNamed %v", step.label, batch[0].GetPayload(), step.served.entry())
		}
		if res.GetSequence() != batch[0].GetSequence() {
			t.Fatalf("%s: the result answers %d, want the entry's %d", step.label, res.GetSequence(), batch[0].GetSequence())
		}
		if tk := batch[1].GetAbilityTaken(); tk.GetActorId() != step.actor || tk.GetAbilityId() != "chair-swing" {
			t.Fatalf("%s: the entry is followed by %v, want the AbilityTaken", step.label, batch[1].GetPayload())
		}
		if rr := batch[2].GetResourceRemoved(); rr.GetActorId() != step.actor || rr.GetResource() != "stamina" {
			t.Fatalf("%s: then %v, want the ResourceRemoved of stamina", step.label, batch[2].GetPayload())
		}
		if batch[0].GetParticipantId() != rc.dmID || batch[0].GetActorRole() != "dm" {
			t.Fatalf("%s: the entry names %q %q, want the issuer", step.label, batch[0].GetParticipantId(), batch[0].GetActorRole())
		}
		s.close()
	}
}

// VTT-989 VTT-992
func TestATakeUnderTheRulesetTheLogNamesOrWithNoneServedAppendsNoEntry(t *testing.T) {
	rc := newRNTakeCampaign(t)
	s := rc.serve(t, rnR.load(t))
	_, batch := s.use(rc.dmToken)
	if batch[0].GetRulesetNamed() == nil {
		t.Fatalf("the first use wrote no entry: %v", batch)
	}
	for _, step := range []struct {
		label  string
		served func() *rules.Ruleset
		actor  string
	}{
		{"under the ruleset the log names", func() *rules.Ruleset { return rnR.load(t) }, "s1"},
		{"with no ruleset served", func() *rules.Ruleset { return nil }, "s2"},
	} {
		s.close()
		s = rc.serve(t, step.served())
		res, batch := s.command(rc.dmToken, rnTake(step.actor, "chair-swing"))
		if len(batch) == 0 || batch[0].GetAbilityTaken() == nil || res.GetSequence() != batch[0].GetSequence() {
			t.Fatalf("%s: the take appended %v answering %d, want it to begin with the AbilityTaken", step.label, batch, res.GetSequence())
		}
		for _, env := range batch {
			if env.GetRulesetNamed() != nil {
				t.Fatalf("%s: a RulesetNamed %v at %d", step.label, env.GetRulesetNamed(), env.GetSequence())
			}
		}
	}
	s.close()
}

// VTT-990
func TestAGiveOnALogNamingNoneAppendsNoEntryAndTheNextUseDoes(t *testing.T) {
	rc := newRNCampaign(t)
	s := rc.serve(t, rnR.load(t))
	give := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GiveAbility{
		GiveAbility: &vttv1.GiveAbility{ActorId: "brawler", AbilityId: "splash-of-water"}}}
	_, batch := s.command(rc.dmToken, give)
	if len(batch) != 1 || batch[0].GetAbilityGiven() == nil {
		t.Fatalf("the give appended %v, want its AbilityGiven alone", batch)
	}
	res, batch := s.use(rc.dmToken)
	rnWantEntry(t, "the use after the give", res, batch, rnR.entry())
	s.close()
}
