package campaign_test

import (
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const (
	qacsHero     = "act-ayla"
	qacsBeast    = "act-gob"
	qacsBeastTok = "tok-gob"
)

var qacsSecrets = []string{
	"Gorvash the Sly", "Goblin Chief Vrask", "carries the vault key",
	"smoke hollow", "grit", "cunning", "Ayla (pc)", "owes the guild",
}

type qacsTable struct {
	t   *testing.T
	dir string
	c   *campaign.Campaign
	n   int
}

func (q *qacsTable) env(e *vttv1.Envelope) *vttv1.Envelope {
	q.n++
	e.EventId = fmt.Sprintf("qacs-%d", q.n)
	e.OccurredAt = timestamppb.Now()
	e.ParticipantId = "p-dm"
	e.ActorRole = "dm"
	return e
}

func (q *qacsTable) append(envs ...*vttv1.Envelope) {
	q.t.Helper()
	for _, e := range envs {
		q.env(e)
	}
	if _, err := q.c.AppendBatch(envs); err != nil {
		q.t.Fatalf("append: %v", err)
	}
}

func (q *qacsTable) log(id string) []campaign.CharacterEntry {
	q.t.Helper()
	entries, err := q.c.CharacterLog(id)
	if err != nil {
		q.t.Fatalf("CharacterLog(%s): %v", id, err)
	}
	return entries
}

func qacsStructOf(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("struct: %v", err)
	}
	return s
}

func qacsHeroActor(t *testing.T) *vttv1.Actor {
	t.Helper()
	return &vttv1.Actor{
		ActorId: qacsHero, Name: "Ayla Venn", ModuleId: "mod-scout",
		Attributes: map[string]int32{"brawn": 3},
		Resources:  map[string]*vttv1.Resource{"vigor": {Current: 7, Max: 10}},
		ModuleData: qacsStructOf(t, map[string]any{"oath": "dawn"}),
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}
}

func qacsBeastActor(t *testing.T) *vttv1.Actor {
	t.Helper()
	return &vttv1.Actor{
		ActorId: qacsBeast, Name: "Gorvash the Sly", ModuleId: "mod-goblin",
		Attributes: map[string]int32{"brawn": 2, "cunning": 7},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 4, Max: 6}},
		ModuleData: qacsStructOf(t, map[string]any{"lair": "smoke hollow"}),
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}
}

func qacsEnvGrant(actor, who string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actor, ParticipantId: who, Kind: kind},
	}}
}

func qacsEnvPool(actor, res string, delta, value int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: actor, Resource: res, Delta: delta, NewValue: value, Reason: "a falling rock",
	}}}
}

func qacsEnvNarrate(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

func qacsEnvCond(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cond, Source: "a tripwire",
	}}}
}

func qacsWorld(t *testing.T) []*vttv1.Envelope {
	t.Helper()
	tiles := map[string]*vttv1.TileRef{}
	for x := 0; x < 7; x++ {
		for y := 0; y < 3; y++ {
			kind := "floor"
			if x == 3 {
				kind = "wall"
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "night one"}}},
		{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: "cave", Name: "Cave", GridWidth: 7, GridHeight: 3, Tiles: tiles,
		}}},
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: qacsHeroActor(t)}}},
		qacsEnvGrant(qacsHero, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER),
		{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: qacsBeastActor(t)}}},
		qacsEnvGrant(qacsBeast, "p-agent", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qacsEnvCond(qacsBeast, "prone"),
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: "tok-ayla", SceneId: "cave", ActorId: qacsHero, Position: &vttv1.GridPosition{X: 1, Y: 1},
			Name: "Ayla", Notes: "a scout in green", GmName: "Ayla (pc)", GmNotes: "owes the guild",
		}}},
		{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: qacsBeastTok, SceneId: "cave", ActorId: qacsBeast, Position: &vttv1.GridPosition{X: 2, Y: 1},
			Name: "a hunched figure", Notes: "it smells of smoke",
			GmName: "Goblin Chief Vrask", GmNotes: "carries the vault key",
		}}},
	}
}

func qacsOpen(t *testing.T) *qacsTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	q := &qacsTable{t: t, dir: dir, c: c}
	t.Cleanup(func() { _ = q.c.Close() })
	q.append(qacsWorld(t)...)
	return q
}

func qacsFoldEntries(t *testing.T, entries []campaign.CharacterEntry) *engine.State {
	t.Helper()
	st := engine.NewState()
	for i, e := range entries {
		if err := engine.Apply(st, e.Env); err != nil {
			t.Fatalf("VTT-323: entry %d does not fold: %v\n%v", i, err, e.Env)
		}
	}
	return st
}

func qacsScan(t *testing.T, entries []campaign.CharacterEntry, secrets []string) {
	t.Helper()
	for i, e := range entries {
		raw, err := protojson.Marshal(e.Env)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		for _, s := range secrets {
			if strings.Contains(string(raw), s) {
				t.Errorf("VTT-462 VTT-465: entry %d carries %q: %s", i, s, raw)
			}
		}
	}
}

func qacsCount(entries []campaign.CharacterEntry, pred func(*vttv1.Envelope) bool) int {
	n := 0
	for _, e := range entries {
		if pred(e.Env) {
			n++
		}
	}
	return n
}

func qacsArrival(id string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return (f.GetActorAdded() != nil && f.GetActorAdded().GetActor().GetActorId() == id) ||
			(f.GetActorRemoved() != nil && f.GetActorRemoved().GetActorId() == id)
	}
}

func qacsPoolOf(id string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return f.GetResourceChanged() != nil && f.GetResourceChanged().GetActorId() == id
	}
}

// VTT-465 VTT-462 VTT-478
func TestQACSAPlayersLogHoldsACreatureWithoutItsStatblockAndTheWorldHoldsItWhole(t *testing.T) {
	q := qacsOpen(t)
	entries := q.log(qacsHero)
	var added []*vttv1.Actor
	for _, e := range entries {
		if a := e.Env.GetActorAdded().GetActor(); a.GetActorId() == qacsBeast {
			added = append(added, a)
		}
	}
	want := &vttv1.Actor{ActorId: qacsBeast, ModuleId: "mod-goblin", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}
	if len(added) != 1 || !proto.Equal(added[0], want) {
		t.Errorf("VTT-465: the hero's log introduces the creature as %v, want once as %v", added, want)
	}
	qacsScan(t, entries, qacsSecrets)
	fold := q.c.CharacterFoldForTest(qacsHero)
	if a := fold.Actors[qacsBeast]; a == nil || a.GetName() != "" || len(a.GetResources()) != 0 ||
		len(a.GetAttributes()) != 0 || a.GetModuleData() != nil {
		t.Errorf("VTT-465: the log's fold holds the creature as %v", a)
	}
	if tok := fold.Tokens[qacsBeastTok]; tok.Name != "a hunched figure" || tok.GMName != "" || tok.GMNotes != "" {
		t.Errorf("VTT-478 VTT-462: the log's fold holds the token as %+v", tok)
	}
	if !proto.Equal(qacsFoldEntries(t, entries).Actors[qacsBeast], fold.Actors[qacsBeast]) {
		t.Errorf("SPEC-021: the stored log and the campaign's fold disagree on the creature")
	}
	world := q.c.State()
	if a := world.Actors[qacsBeast]; a.GetName() != "Gorvash the Sly" || a.GetResources()["grit"].GetCurrent() != 4 {
		t.Errorf("the world holds the creature as %v, want it whole", a)
	}
	if tok := world.Tokens[qacsBeastTok]; tok.GMName != "Goblin Chief Vrask" || tok.GMNotes != "carries the vault key" {
		t.Errorf("the world holds the token as %+v, want its four texts", tok)
	}
}

// VTT-466 VTT-486
func TestQACSAPlayersLogIsWrittenNoCreatureResourceChange(t *testing.T) {
	q := qacsOpen(t)
	before := len(q.log(qacsHero))
	q.append(qacsEnvPool(qacsBeast, "grit", -3, 1))
	q.append(qacsEnvPool(qacsHero, "vigor", -2, 5))
	after := q.log(qacsHero)[before:]
	if n := qacsCount(after, qacsPoolOf(qacsBeast)); n != 0 {
		t.Errorf("VTT-466: the hero's log was written %d ResourceChanged of the creature", n)
	}
	if n := qacsCount(after, qacsPoolOf(qacsHero)); n != 1 {
		t.Errorf("VTT-486: written %d of the hero's", n)
	}
	if got := q.c.CharacterFoldForTest(qacsHero).Actors[qacsBeast]; len(got.GetResources()) != 0 {
		t.Errorf("VTT-466: the log's fold holds the creature's resources %v", got.GetResources())
	}
}

// VTT-467 VTT-462
func TestQACSAPlayersLogIsWrittenARewriteWithoutItsGMTexts(t *testing.T) {
	q := qacsOpen(t)
	before := len(q.log(qacsHero))
	q.append(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenDescribed{TokenDescribed: &vttv1.TokenDescribed{
		TokenId: qacsBeastTok, Name: "a grinning figure", Notes: "it smells of smoke",
		GmName: "Vrask Unmasked", GmNotes: "the key is a forgery",
	}}})
	after := q.log(qacsHero)[before:]
	var got []*vttv1.TokenDescribed
	for _, e := range after {
		if d := e.Env.GetTokenDescribed(); d != nil {
			got = append(got, d)
		}
	}
	want := &vttv1.TokenDescribed{TokenId: qacsBeastTok, Name: "a grinning figure", Notes: "it smells of smoke"}
	if len(got) != 1 || !proto.Equal(got[0], want) {
		t.Errorf("VTT-467 VTT-462: the log was written %v, want %v", got, want)
	}
	qacsScan(t, after, []string{"Vrask Unmasked", "the key is a forgery"})
	if tok := q.c.State().Tokens[qacsBeastTok]; tok.GMName != "Vrask Unmasked" {
		t.Errorf("the world holds the token as %+v, want its GM name", tok)
	}
}

// VTT-324
func TestQACSAReopenedLogDoesNotIntroduceAHeldCreatureAgain(t *testing.T) {
	q := qacsOpen(t)
	if err := q.c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	c, err := campaign.Open(q.dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	q.c = c
	before := len(q.log(qacsHero))
	q.append(qacsEnvNarrate("The torches gutter."))
	q.append(qacsEnvCond(qacsBeast, "hidden"))
	q.append(qacsEnvPool(qacsBeast, "grit", -1, 3))
	after := q.log(qacsHero)[before:]
	if n := qacsCount(after, qacsArrival(qacsBeast)); n != 0 {
		t.Errorf("VTT-470: after a reopen, %d arrivals", n)
	}
	cond := 0
	for _, e := range after {
		if a := e.Env.GetConditionApplied(); a != nil && a.GetActorId() == qacsBeast && a.GetConditionId() == "hidden" {
			cond++
		}
	}
	if cond != 1 {
		t.Errorf("SPEC-016 (forwarded while every actor named is in sighted): the seen creature's condition was written %d times after a reopen", cond)
	}
	qacsScan(t, q.log(qacsHero), qacsSecrets)
	qacsFoldEntries(t, q.log(qacsHero))
}

// VTT-468 VTT-316
func TestQACSAKindChangeInSightRewritesTheCreatureInAPlayersLog(t *testing.T) {
	q := qacsOpen(t)
	q.append(qacsEnvGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	fold := q.c.CharacterFoldForTest(qacsHero)
	if a := fold.Actors[qacsBeast]; a.GetName() != "Gorvash the Sly" || a.GetResources()["grit"].GetCurrent() != 4 {
		t.Errorf("VTT-468: %v", a)
	}
	if _, ok := fold.Tokens[qacsBeastTok]; !ok {
		t.Errorf("VTT-468: the creature's token is not on the log's board after the grant")
	}
	if len(q.log(qacsBeast)) == 0 {
		t.Errorf("VTT-317: the creature made a party member has no log of its own")
	}
	own := len(q.log(qacsBeast))
	before := len(q.log(qacsHero))
	q.append(qacsEnvGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	fold = q.c.CharacterFoldForTest(qacsHero)
	if a := fold.Actors[qacsBeast]; a.GetName() != "" || len(a.GetResources()) != 0 || a.GetModuleData() != nil {
		t.Errorf("VTT-468: %v", a)
	}
	if n := len(q.log(qacsBeast)); n != own {
		t.Errorf("VTT-316: the creature's own log was written %d entries by the grant that made it non-party", n-own)
	}
	q.append(qacsEnvPool(qacsBeast, "grit", -1, 3))
	after := q.log(qacsHero)[before:]
	if n := qacsCount(after, qacsPoolOf(qacsBeast)); n != 0 {
		t.Errorf("VTT-466: %d ResourceChanged of the creature after it left the party", n)
	}
	qacsScan(t, after, qacsSecrets)
	qacsFoldEntries(t, q.log(qacsHero))
	qacsFoldEntries(t, q.log(qacsBeast))
}

// VTT-481 VTT-318
func TestQACSAResumedLogMeetsACreatureThatJoinedThePartyDuringItsPause(t *testing.T) {
	q := qacsOpen(t)
	q.append(qacsEnvGrant(qacsHero, "p-ayla", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	paused := len(q.log(qacsHero))
	q.append(qacsEnvGrant(qacsBeast, "p-agent", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	q.append(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenDescribed{TokenDescribed: &vttv1.TokenDescribed{
		TokenId: qacsBeastTok, Name: "a sworn ally", Notes: "a new oath", GmName: "Vrask Turned", GmNotes: "a spy still",
	}}})
	if n := len(q.log(qacsHero)); n != paused {
		t.Fatalf("VTT-316: the paused log was written %d entries", n-paused)
	}
	q.append(qacsEnvGrant(qacsHero, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	after := q.log(qacsHero)[paused:]
	fold := qacsFoldEntries(t, q.log(qacsHero))
	if a := fold.Actors[qacsBeast]; a.GetName() != "Gorvash the Sly" || a.GetResources()["grit"].GetCurrent() != 4 ||
		a.GetKind() != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Errorf("VTT-481: on its resume the log holds the creature as %v", a)
	}
	if tok := fold.Tokens[qacsBeastTok]; tok.Name != "a sworn ally" || tok.Notes != "a new oath" || tok.GMName != "" {
		t.Errorf("VTT-478 VTT-462: on its resume the log holds the token as %+v", tok)
	}
	qacsScan(t, after, []string{"Vrask Turned", "a spy still", "Goblin Chief Vrask", "Ayla (pc)"})
	if !proto.Equal(fold.Actors[qacsBeast], q.c.CharacterFoldForTest(qacsHero).Actors[qacsBeast]) {
		t.Errorf("SPEC-021: the stored log and the campaign's fold disagree on the creature")
	}
	qacsScan(t, q.log(qacsBeast), []string{"Goblin Chief Vrask", "carries the vault key", "Vrask Turned", "a spy still", "Ayla (pc)"})
}
