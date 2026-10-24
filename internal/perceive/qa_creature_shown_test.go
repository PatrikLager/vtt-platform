package perceive_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const (
	qacsScene     = "cave"
	qacsHero      = "act-ayla"
	qacsFriend    = "act-bram"
	qacsBeast     = "act-gob"
	qacsRat       = "act-rat"
	qacsHeroTok   = "tok-ayla"
	qacsFriendTok = "tok-bram"
	qacsBeastTok  = "tok-gob"
	qacsRatTok    = "tok-rat"
)

var qacsSecrets = []string{
	"Gorvash the Sly", "Goblin Chief Vrask", "carries the vault key",
	"smoke hollow", "grit", "cunning",
	"Ayla (pc)", "owes the guild", "Bram (pc)", "cursed blade",
	"Squeak", "whisker",
}

var qacsGMSecrets = []string{
	"Goblin Chief Vrask", "carries the vault key",
	"Ayla (pc)", "owes the guild", "Bram (pc)", "cursed blade",
}

type qacsViewer struct {
	t     *testing.T
	eyes  perceive.Eyes
	world *engine.State
	shown *engine.State
	pr    *perceive.Projection
	seq   int64
	steps [][]*vttv1.Envelope
}

func qacsNewViewer(t *testing.T, shoulder string) *qacsViewer {
	t.Helper()
	eyes := perceive.Eyes{Shoulder: shoulder}
	return &qacsViewer{
		t: t, eyes: eyes, world: engine.NewState(), shown: engine.NewState(),
		pr: perceive.New(eyes, nil),
	}
}

func (v *qacsViewer) stamp(env *vttv1.Envelope) *vttv1.Envelope {
	v.seq++
	env.Sequence = v.seq
	env.EventId = fmt.Sprintf("ev-%d", v.seq)
	env.ParticipantId = "p-dm"
	env.ActorRole = "dm"
	env.SessionId = "sess-1"
	return env
}

func (v *qacsViewer) step(env *vttv1.Envelope) []*vttv1.Envelope {
	v.t.Helper()
	v.stamp(env)
	if err := engine.Apply(v.world, env); err != nil {
		v.t.Fatalf("world refused event %d (%v): %v", v.seq, env, err)
	}
	before := proto.Clone(env)
	frames := v.pr.Event(v.eyes, env, v.shown, v.world)
	if !proto.Equal(before, env) {
		v.t.Fatalf("SPEC-016: Event wrote to the envelope it was handed")
	}
	v.fold(frames)
	v.steps = append(v.steps, frames)
	return frames
}

func (v *qacsViewer) perch(shoulder string) []*vttv1.Envelope {
	v.t.Helper()
	v.eyes = perceive.Eyes{Shoulder: shoulder}
	frames := v.pr.Perch(v.eyes, v.shown, v.world, v.seq)
	v.fold(frames)
	v.steps = append(v.steps, frames)
	return frames
}

func (v *qacsViewer) fold(frames []*vttv1.Envelope) {
	v.t.Helper()
	for i, f := range frames {
		if err := engine.Apply(v.shown, f); err != nil {
			v.t.Fatalf("frame %d at seq %d does not fold: %v\nframe: %v", i, v.seq, err, f)
		}
	}
}

func (v *qacsViewer) move(tok string, x, y int32) []*vttv1.Envelope {
	v.t.Helper()
	cur, ok := v.world.Tokens[tok]
	if !ok {
		v.t.Fatalf("fixture: no token %s", tok)
	}
	return v.step(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: tok, SceneId: cur.SceneID,
		From:   &vttv1.GridPosition{X: cur.X, Y: cur.Y},
		To:     &vttv1.GridPosition{X: x, Y: y},
		Reason: "a shove from behind",
	}}})
}

func (v *qacsViewer) all() []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for _, s := range v.steps {
		out = append(out, s...)
	}
	return out
}

func (v *qacsViewer) noSecrets(secrets []string) {
	v.t.Helper()
	for _, f := range v.all() {
		raw, err := protojson.Marshal(f)
		if err != nil {
			v.t.Fatalf("marshal: %v", err)
		}
		for _, s := range secrets {
			if strings.Contains(string(raw), s) {
				v.t.Errorf("VTT-462 VTT-465: a frame carries %q: %s", s, raw)
			}
		}
	}
}

func qacsStruct(t *testing.T, m map[string]any) *structpb.Struct {
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
		Attributes: map[string]int32{"brawn": 3, "wit": 5},
		Resources:  map[string]*vttv1.Resource{"vigor": {Current: 7, Max: 10}},
		ModuleData: qacsStruct(t, map[string]any{"oath": "dawn"}),
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}
}

func qacsFriendActor(t *testing.T) *vttv1.Actor {
	t.Helper()
	return &vttv1.Actor{
		ActorId: qacsFriend, Name: "Bram Holt", ModuleId: "mod-warden",
		Attributes: map[string]int32{"brawn": 6},
		Resources: map[string]*vttv1.Resource{
			"vigor": {Current: 9, Max: 12}, "focus": {Current: 1, Max: 3},
		},
		ModuleData: qacsStruct(t, map[string]any{"vow": "iron"}),
		Kind:       vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
	}
}

func qacsBeastActor(t *testing.T) *vttv1.Actor {
	t.Helper()
	return &vttv1.Actor{
		ActorId: qacsBeast, Name: "Gorvash the Sly", ModuleId: "mod-goblin",
		Attributes: map[string]int32{"brawn": 2, "cunning": 7},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 4, Max: 6}},
		ModuleData: qacsStruct(t, map[string]any{"lair": "smoke hollow"}),
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}
}

func qacsRatActor(t *testing.T) *vttv1.Actor {
	t.Helper()
	return &vttv1.Actor{
		ActorId: qacsRat, Name: "Squeak", ModuleId: "mod-rat",
		Attributes: map[string]int32{"whisker": 4},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 2, Max: 3}},
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}
}

func qacsSceneEnv() *vttv1.Envelope {
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
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: qacsScene, Name: "Cave", GridWidth: 7, GridHeight: 3, Tiles: tiles,
	}}}
}

func qacsAdd(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qacsGrant(actor, who string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{
		ActorControlGranted: &vttv1.ActorControlGranted{ActorId: actor, ParticipantId: who, Kind: kind},
	}}
}

func qacsRevoke(actor, who string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlRevoked{
		ActorControlRevoked: &vttv1.ActorControlRevoked{ActorId: actor, ParticipantId: who},
	}}
}

func qacsCond(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: cond, Source: "a tripwire",
	}}}
}

func qacsUncond(actor, cond string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
		ActorId: actor, ConditionId: cond, Reason: "shrugged off",
	}}}
}

func qacsPlace(tok, actor string, x, y int32, name, notes, gmName, gmNotes string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: tok, SceneId: qacsScene, ActorId: actor,
		Position: &vttv1.GridPosition{X: x, Y: y},
		Name:     name, Notes: notes, GmName: gmName, GmNotes: gmNotes,
	}}}
}

func qacsPool(actor, res string, delta, value int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
		ActorId: actor, Resource: res, Delta: delta, NewValue: value, Reason: "a falling rock",
	}}}
}

func qacsDescribe(tok, name, notes, gmName, gmNotes string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenDescribed{TokenDescribed: &vttv1.TokenDescribed{
		TokenId: tok, Name: name, Notes: notes, GmName: gmName, GmNotes: gmNotes,
	}}}
}

func qacsNarrate(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

func qacsRemoveToken(tok string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tok}}}
}

func qacsRemoveActor(actor string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}}
}

func qacsTable(t *testing.T) *qacsViewer {
	t.Helper()
	v := qacsNewViewer(t, qacsHero)
	v.step(qacsSceneEnv())
	v.step(qacsAdd(qacsHeroActor(t)))
	v.step(qacsGrant(qacsHero, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	v.step(qacsAdd(qacsFriendActor(t)))
	v.step(qacsGrant(qacsFriend, "p-bram", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	v.step(qacsAdd(qacsBeastActor(t)))
	v.step(qacsGrant(qacsBeast, "p-agent", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	v.step(qacsGrant(qacsBeast, "p-zed", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	v.step(qacsCond(qacsBeast, "prone"))
	v.step(qacsCond(qacsBeast, "dazed"))
	v.step(qacsPlace(qacsHeroTok, qacsHero, 1, 1, "Ayla", "a scout in green", "Ayla (pc)", "owes the guild"))
	v.step(qacsPlace(qacsFriendTok, qacsFriend, 5, 1, "Bram", "a tall warden", "Bram (pc)", "cursed blade"))
	v.step(qacsPlace(qacsBeastTok, qacsBeast, 2, 1, "a hunched figure", "it smells of smoke",
		"Goblin Chief Vrask", "carries the vault key"))
	return v
}

func qacsActorOf(f *vttv1.Envelope) string {
	switch p := f.GetPayload().(type) {
	case *vttv1.Envelope_ActorAdded:
		return p.ActorAdded.GetActor().GetActorId()
	case *vttv1.Envelope_ActorRemoved:
		return p.ActorRemoved.GetActorId()
	case *vttv1.Envelope_ActorControlGranted:
		return p.ActorControlGranted.GetActorId()
	case *vttv1.Envelope_ActorControlRevoked:
		return p.ActorControlRevoked.GetActorId()
	case *vttv1.Envelope_ResourceChanged:
		return p.ResourceChanged.GetActorId()
	case *vttv1.Envelope_ConditionApplied:
		return p.ConditionApplied.GetActorId()
	case *vttv1.Envelope_ConditionRemoved:
		return p.ConditionRemoved.GetActorId()
	case *vttv1.Envelope_TokenPlaced:
		return p.TokenPlaced.GetActorId()
	}
	return ""
}

func qacsFind(frames []*vttv1.Envelope, pred func(*vttv1.Envelope) bool) []int {
	var out []int
	for i, f := range frames {
		if pred(f) {
			out = append(out, i)
		}
	}
	return out
}

func qacsAddedOf(id string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return f.GetActorAdded() != nil && f.GetActorAdded().GetActor().GetActorId() == id
	}
}

func qacsRemovedOf(id string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return f.GetActorRemoved() != nil && f.GetActorRemoved().GetActorId() == id
	}
}

func qacsPlacedOf(tok string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return f.GetTokenPlaced() != nil && f.GetTokenPlaced().GetTokenId() == tok
	}
}

func qacsHiddenOf(tok string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return f.GetTokenHidden() != nil && f.GetTokenHidden().GetTokenId() == tok
	}
}

func qacsPoolOf(id string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return f.GetResourceChanged() != nil && f.GetResourceChanged().GetActorId() == id
	}
}

func qacsDescribedOf(tok string) func(*vttv1.Envelope) bool {
	return func(f *vttv1.Envelope) bool {
		return f.GetTokenDescribed() != nil && f.GetTokenDescribed().GetTokenId() == tok
	}
}

func qacsStripped(a *vttv1.Actor) *vttv1.Actor {
	return &vttv1.Actor{ActorId: a.GetActorId(), ModuleId: a.GetModuleId(), Kind: a.GetKind()}
}

func qacsWhole(a *vttv1.Actor) *vttv1.Actor {
	c := proto.Clone(a).(*vttv1.Actor)
	c.ControllerId = ""
	c.ControllerIds = nil
	return c
}

func qacsSorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func qacsConds(st *engine.State, id string) []string {
	var out []string
	for _, c := range st.Conditions[id] {
		out = append(out, c.ID)
	}
	return qacsSorted(out)
}

func qacsSame(a, b []string) bool {
	return strings.Join(qacsSorted(a), "|") == strings.Join(qacsSorted(b), "|")
}

func qacsHasNoStatblock(t *testing.T, label string, a *vttv1.Actor) {
	t.Helper()
	if a == nil {
		t.Fatalf("%s: the fold holds no such actor", label)
	}
	if a.GetName() != "" || len(a.GetAttributes()) != 0 || len(a.GetResources()) != 0 || a.GetModuleData() != nil {
		t.Errorf("%s: the fold holds a name, attributes, resources or module data: %v", label, a)
	}
}

// VTT-465
func TestQACSACreatureIsIntroducedByItsIdModuleAndKindWithItsGrantsAndConditionsBehind(t *testing.T) {
	v := qacsTable(t)
	frames := v.steps[len(v.steps)-1]
	added := qacsFind(frames, qacsAddedOf(qacsBeast))
	if len(added) != 1 {
		t.Fatalf("VTT-465: want one ActorAdded of the creature as its token comes into sight, got %d in %v", len(added), frames)
	}
	got := frames[added[0]].GetActorAdded().GetActor()
	want := qacsStripped(v.world.Actors[qacsBeast])
	if !proto.Equal(got, want) {
		t.Errorf("VTT-465: introduced as\n  %v\nwant\n  %v", got, want)
	}
	placed := qacsFind(frames, qacsPlacedOf(qacsBeastTok))
	if len(placed) != 1 {
		t.Fatalf("VTT-478: want the creature's token placed once, got %d", len(placed))
	}
	var grants, conds []string
	for i, f := range frames {
		if qacsActorOf(f) != qacsBeast || i == added[0] || f.GetTokenPlaced() != nil {
			continue
		}
		if i < added[0] || i > placed[0] {
			t.Errorf("VTT-465: frame %d about the creature is not behind its ActorAdded and before its token: %v", i, f)
		}
		switch {
		case f.GetActorControlGranted() != nil:
			g := f.GetActorControlGranted()
			if g.GetKind() != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
				t.Errorf("VTT-465: a grant behind the introduction carries kind %v", g.GetKind())
			}
			grants = append(grants, g.GetParticipantId())
		case f.GetConditionApplied() != nil:
			if f.GetConditionApplied().GetSource() != "" {
				t.Errorf("VTT-465: an introduced condition carries a source: %v", f)
			}
			conds = append(conds, f.GetConditionApplied().GetConditionId())
		default:
			t.Errorf("VTT-465: unexpected frame about the creature at its introduction: %v", f)
		}
	}
	if !qacsSame(grants, []string{"p-agent", "p-zed"}) {
		t.Errorf("VTT-465: controllers behind the introduction %v, want p-agent and p-zed", grants)
	}
	if !qacsSame(conds, []string{"prone", "dazed"}) {
		t.Errorf("VTT-465: conditions behind the introduction %v, want prone and dazed", conds)
	}
	held := v.shown.Actors[qacsBeast]
	qacsHasNoStatblock(t, "VTT-465", held)
	if held.GetModuleId() != "mod-goblin" || held.GetKind() != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
		t.Errorf("VTT-465: the fold holds module %q kind %v", held.GetModuleId(), held.GetKind())
	}
	if !qacsSame(held.GetControllerIds(), []string{"p-agent", "p-zed"}) {
		t.Errorf("VTT-465: the fold holds controllers %v", held.GetControllerIds())
	}
	if !qacsSame(qacsConds(v.shown, qacsBeast), []string{"prone", "dazed"}) {
		t.Errorf("VTT-465: the fold holds conditions %v", qacsConds(v.shown, qacsBeast))
	}
	v.noSecrets(qacsSecrets)
}

// VTT-485
func TestQACSAPartyMemberIsIntroducedWhole(t *testing.T) {
	v := qacsTable(t)
	for _, id := range []string{qacsHero, qacsFriend} {
		idx := -1
		var frames []*vttv1.Envelope
		for _, s := range v.steps {
			if is := qacsFind(s, qacsAddedOf(id)); len(is) > 0 {
				idx, frames = is[0], s
				break
			}
		}
		if idx < 0 {
			t.Fatalf("VTT-485: %s never introduced", id)
		}
		got := frames[idx].GetActorAdded().GetActor()
		want := qacsWhole(v.world.Actors[id])
		if !proto.Equal(got, want) {
			t.Errorf("VTT-485: %s introduced as\n  %v\nwant\n  %v", id, got, want)
		}
	}
}

// VTT-478 VTT-462
func TestQACSATokenArrivesWithItsNameAndNotesAndWithoutItsGMTexts(t *testing.T) {
	v := qacsTable(t)
	want := map[string]*vttv1.TokenPlaced{
		qacsHeroTok: {TokenId: qacsHeroTok, SceneId: qacsScene, ActorId: qacsHero,
			Position: &vttv1.GridPosition{X: 1, Y: 1}, Name: "Ayla", Notes: "a scout in green"},
		qacsBeastTok: {TokenId: qacsBeastTok, SceneId: qacsScene, ActorId: qacsBeast,
			Position: &vttv1.GridPosition{X: 2, Y: 1}, Name: "a hunched figure", Notes: "it smells of smoke"},
	}
	for tok, w := range want {
		is := qacsFind(v.all(), qacsPlacedOf(tok))
		if len(is) != 1 {
			t.Fatalf("VTT-478: token %s placed %d times", tok, len(is))
		}
		got := v.all()[is[0]].GetTokenPlaced()
		if !proto.Equal(got, w) {
			t.Errorf("VTT-478 VTT-462: %s arrived as\n  %v\nwant\n  %v", tok, got, w)
		}
		held := v.shown.Tokens[tok]
		if held.Name != w.GetName() || held.Notes != w.GetNotes() || held.GMName != "" || held.GMNotes != "" {
			t.Errorf("VTT-478 VTT-462: the fold holds %s as %+v", tok, held)
		}
	}
	if len(qacsFind(v.all(), qacsPlacedOf(qacsFriendTok))) != 0 {
		t.Errorf("VTT-478: a token out of sight was placed")
	}
	v.noSecrets(qacsGMSecrets)
}

// VTT-466 VTT-486
func TestQACSACreaturesResourceChangeIsNeverSentAndAPartyMembersIsWhileSeen(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsFriendTok, 0, 0)
	if v.shown.Tokens[qacsFriendTok].ID == "" {
		t.Fatalf("fixture: Bram's token did not come into sight")
	}
	frames := v.step(qacsPool(qacsBeast, "grit", -2, 2))
	if is := qacsFind(frames, qacsPoolOf(qacsBeast)); len(is) != 0 {
		t.Errorf("VTT-466: a seen creature's ResourceChanged was sent: %v", frames)
	}
	frames = v.step(qacsPool(qacsFriend, "vigor", -3, 6))
	is := qacsFind(frames, qacsPoolOf(qacsFriend))
	if len(is) != 1 {
		t.Fatalf("VTT-486: got %d in %v", len(is), frames)
	}
	rc := frames[is[0]].GetResourceChanged()
	if rc.GetResource() != "vigor" || rc.GetDelta() != -3 || rc.GetNewValue() != 6 || rc.GetReason() != "" {
		t.Errorf("VTT-486: sent %v", rc)
	}
	if got := v.shown.Actors[qacsFriend].GetResources()["vigor"].GetCurrent(); got != 6 {
		t.Errorf("VTT-486: the fold holds Bram's vigor at %d, want 6", got)
	}
	v.move(qacsBeastTok, 5, 0)
	if _, ok := v.shown.Tokens[qacsBeastTok]; ok {
		t.Fatalf("fixture: the creature's token is still on the board")
	}
	frames = v.step(qacsPool(qacsBeast, "grit", -1, 1))
	if is := qacsFind(frames, qacsPoolOf(qacsBeast)); len(is) != 0 {
		t.Errorf("VTT-466: an unseen creature's ResourceChanged was sent: %v", frames)
	}
	qacsHasNoStatblock(t, "VTT-466", v.shown.Actors[qacsBeast])
	v.noSecrets(qacsSecrets)
}

// VTT-467 VTT-462
func TestQACSARewriteInSightIsSentWithItsNameAndNotesAlone(t *testing.T) {
	v := qacsTable(t)
	frames := v.step(qacsDescribe(qacsBeastTok, "a grinning figure", "it smells of smoke",
		"Goblin Chief Vrask", "carries the vault key"))
	is := qacsFind(frames, qacsDescribedOf(qacsBeastTok))
	if len(is) != 1 {
		t.Fatalf("VTT-467: a rewrite of a token in sight whose name changed: got %d in %v", len(is), frames)
	}
	f := frames[is[0]]
	want := &vttv1.TokenDescribed{TokenId: qacsBeastTok, Name: "a grinning figure", Notes: "it smells of smoke"}
	if !proto.Equal(f.GetTokenDescribed(), want) {
		t.Errorf("VTT-467 VTT-462: sent\n  %v\nwant\n  %v", f.GetTokenDescribed(), want)
	}
	if f.GetParticipantId() != "" || f.GetActorRole() != "" {
		t.Errorf("SPEC-016: the rewrite carries its issuer: %v", f)
	}
	frames = v.step(qacsDescribe(qacsBeastTok, "a grinning figure", "a torn red cloak",
		"Goblin Chief Vrask", "carries the vault key"))
	is = qacsFind(frames, qacsDescribedOf(qacsBeastTok))
	if len(is) != 1 {
		t.Fatalf("VTT-467: a rewrite whose notes alone changed: got %d in %v", len(is), frames)
	}
	held := v.shown.Tokens[qacsBeastTok]
	if held.Name != "a grinning figure" || held.Notes != "a torn red cloak" || held.GMName != "" || held.GMNotes != "" {
		t.Errorf("VTT-467: the fold holds the token as %+v", held)
	}
	v.noSecrets(qacsSecrets)
}

// VTT-467
func TestQACSARewriteOfTheGMTextsAloneIsNotSent(t *testing.T) {
	v := qacsTable(t)
	frames := v.step(qacsDescribe(qacsBeastTok, "a hunched figure", "it smells of smoke",
		"Vrask the Undying", "the key is a forgery"))
	if is := qacsFind(frames, qacsDescribedOf(qacsBeastTok)); len(is) != 0 {
		t.Errorf("VTT-467: a rewrite that leaves the name and notes as the log holds them was sent: %v", frames)
	}
	frames = v.step(qacsDescribe(qacsHeroTok, "Ayla", "a scout in green", "", ""))
	if is := qacsFind(frames, qacsDescribedOf(qacsHeroTok)); len(is) != 0 {
		t.Errorf("VTT-467: clearing the GM texts of the character's own token was sent: %v", frames)
	}
	v.noSecrets(append(qacsSecrets, "Vrask the Undying", "the key is a forgery"))
}

// VTT-467 VTT-478
func TestQACSARewriteOutOfSightIsNotSentAndTheTokenArrivesWithItsPresentTexts(t *testing.T) {
	v := qacsTable(t)
	frames := v.move(qacsBeastTok, 5, 0)
	if len(qacsFind(frames, qacsHiddenOf(qacsBeastTok))) != 1 {
		t.Fatalf("fixture: the creature's token was not hidden as it left sight: %v", frames)
	}
	frames = v.step(qacsDescribe(qacsBeastTok, "a limping figure", "blood on its boots", "Vrask", "wounded"))
	if len(frames) != 0 {
		t.Errorf("VTT-467: a rewrite of a token out of sight sent %v", frames)
	}
	frames = v.step(qacsDescribe(qacsFriendTok, "Bram the Bold", "a dented shield", "", ""))
	if len(frames) != 0 {
		t.Errorf("VTT-467: a rewrite of a token never on the board sent %v", frames)
	}
	frames = v.move(qacsBeastTok, 2, 0)
	is := qacsFind(frames, qacsPlacedOf(qacsBeastTok))
	if len(is) != 1 {
		t.Fatalf("VTT-478: the creature's token did not arrive on its return: %v", frames)
	}
	want := &vttv1.TokenPlaced{TokenId: qacsBeastTok, SceneId: qacsScene, ActorId: qacsBeast,
		Position: &vttv1.GridPosition{X: 2, Y: 0}, Name: "a limping figure", Notes: "blood on its boots"}
	if got := frames[is[0]].GetTokenPlaced(); !proto.Equal(got, want) {
		t.Errorf("VTT-478:\n  %v\nwant\n  %v", got, want)
	}
	if len(qacsFind(frames, qacsDescribedOf(qacsBeastTok))) != 0 {
		t.Errorf("VTT-467: a token not on the board before the event was sent a rewrite: %v", frames)
	}
	frames = v.move(qacsFriendTok, 0, 0)
	is = qacsFind(frames, qacsPlacedOf(qacsFriendTok))
	if len(is) != 1 {
		t.Fatalf("VTT-478: Bram's token did not arrive: %v", frames)
	}
	if got := frames[is[0]].GetTokenPlaced(); got.GetName() != "Bram the Bold" || got.GetNotes() != "a dented shield" {
		t.Errorf("VTT-478: Bram's token arrived as %v", got)
	}
	v.noSecrets(append(qacsSecrets, "wounded"))
}

// VTT-470
func TestQACSACreatureHeldInSightDoesNotArriveAgainAtEachEvent(t *testing.T) {
	v := qacsTable(t)
	introduced := len(v.steps)
	v.move(qacsHeroTok, 1, 0)
	v.move(qacsBeastTok, 2, 2)
	v.step(qacsPool(qacsBeast, "grit", -1, 3))
	v.step(qacsCond(qacsBeast, "hidden"))
	v.step(qacsUncond(qacsBeast, "dazed"))
	v.step(qacsGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	v.step(qacsRevoke(qacsBeast, "p-zed"))
	v.step(qacsDescribe(qacsBeastTok, "a crouching figure", "it smells of smoke", "", ""))
	v.step(qacsNarrate("The torches gutter."))
	v.move(qacsBeastTok, 0, 2)
	for n, s := range v.steps[introduced:] {
		bad := append(qacsFind(s, qacsAddedOf(qacsBeast)), qacsFind(s, qacsRemovedOf(qacsBeast))...)
		if len(bad) != 0 {
			t.Errorf("VTT-470: event %d of the run sent %v", n+1, s)
		}
		if len(qacsFind(s, qacsHiddenOf(qacsBeastTok))) != 0 {
			t.Errorf("VTT-470: event %d of the run hid the creature's token: %v", n+1, s)
		}
	}
	held := v.shown.Actors[qacsBeast]
	qacsHasNoStatblock(t, "VTT-465", held)
	if !qacsSame(qacsConds(v.shown, qacsBeast), []string{"prone", "hidden"}) {
		t.Errorf("SPEC-016 (forwarded while every actor named is in sighted): conditions held %v, want prone and hidden", qacsConds(v.shown, qacsBeast))
	}
	if !qacsSame(held.GetControllerIds(), []string{"p-agent", "p-ayla"}) {
		t.Errorf("SPEC-016 (forwarded while every actor named is in sighted): controllers held %v, want p-agent and p-ayla", held.GetControllerIds())
	}
	v.noSecrets(qacsSecrets)
}

func qacsCheckReintroduced(t *testing.T, label string, frames []*vttv1.Envelope, seq int64, id, tok string, want *vttv1.Actor) {
	t.Helper()
	rem := qacsFind(frames, qacsRemovedOf(id))
	add := qacsFind(frames, qacsAddedOf(id))
	if len(rem) != 1 || len(add) != 1 {
		t.Fatalf("%s: want one bare ActorRemoved and one ActorAdded of %s, got %d and %d in %v", label, id, len(rem), len(add), frames)
	}
	if seq == 0 {
		seq = frames[rem[0]].GetSequence()
	}
	bare := &vttv1.Envelope{Sequence: seq, Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: id}}}
	if !proto.Equal(frames[rem[0]], bare) {
		t.Errorf("VTT-252: the ActorRemoved is not bare: %v", frames[rem[0]])
	}
	if add[0] != rem[0]+1 {
		t.Errorf("VTT-252: the bare ActorRemoved is not directly before the introduction: %v", frames)
	}
	if got := frames[add[0]].GetActorAdded().GetActor(); !proto.Equal(got, want) {
		t.Errorf("%s: introduced afresh as\n  %v\nwant\n  %v", label, got, want)
	}
	if tok == "" {
		return
	}
	hid := qacsFind(frames, qacsHiddenOf(tok))
	placed := qacsFind(frames, qacsPlacedOf(tok))
	if len(placed) != 1 || placed[0] < add[0] {
		t.Errorf("%s: the token is not placed again after the introduction: %v", label, frames)
	} else if p := frames[placed[0]].GetTokenPlaced(); p.GetGmName() != "" || p.GetGmNotes() != "" {
		t.Errorf("VTT-462: the token placed again carries its GM texts: %v", p)
	}
	if len(hid) > 1 || (len(hid) == 1 && hid[0] > rem[0]) {
		t.Errorf("%s: the token is not hidden before the withdrawal: %v", label, frames)
	}
}

// VTT-468
func TestQACSACreatureMadeAPartyMemberInSightArrivesWithItsNameAndStatblock(t *testing.T) {
	v := qacsTable(t)
	frames := v.step(qacsGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	if len(qacsFind(frames, qacsHiddenOf(qacsBeastTok))) != 1 {
		t.Errorf("VTT-468: the creature's token on the board was not hidden before the withdrawal: %v", frames)
	}
	qacsCheckReintroduced(t, "VTT-468", frames, v.seq, qacsBeast, qacsBeastTok, qacsWhole(v.world.Actors[qacsBeast]))
	held := v.shown.Actors[qacsBeast]
	w := v.world.Actors[qacsBeast]
	if held.GetName() != w.GetName() || !proto.Equal(held.GetModuleData(), w.GetModuleData()) ||
		len(held.GetResources()) != 1 || held.GetResources()["grit"].GetCurrent() != 4 ||
		held.GetAttributes()["cunning"] != 7 || held.GetKind() != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
		t.Errorf("VTT-468: the fold holds %v", held)
	}
	if !qacsSame(held.GetControllerIds(), []string{"p-agent", "p-zed", "p-ayla"}) {
		t.Errorf("VTT-468: controllers held %v", held.GetControllerIds())
	}
	if !qacsSame(qacsConds(v.shown, qacsBeast), []string{"prone", "dazed"}) {
		t.Errorf("VTT-468: conditions held %v", qacsConds(v.shown, qacsBeast))
	}
	if _, ok := v.shown.Tokens[qacsBeastTok]; !ok {
		t.Errorf("VTT-468: the token is not on the board after the event")
	}
	frames = v.step(qacsPool(qacsBeast, "grit", -1, 3))
	if len(qacsFind(frames, qacsPoolOf(qacsBeast))) != 1 {
		t.Errorf("VTT-486: got %v", frames)
	}
	v.noSecrets(qacsGMSecrets)
}

// VTT-468
func TestQACSAPartyMemberMadeNonPartyInSightIsHeldWithoutItsStatblock(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsFriendTok, 0, 0)
	if v.shown.Actors[qacsFriend].GetName() != "Bram Holt" {
		t.Fatalf("fixture: Bram is not held whole before the grant")
	}
	mark := len(v.steps)
	frames := v.step(qacsGrant(qacsFriend, "p-bram", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	if len(qacsFind(frames, qacsHiddenOf(qacsFriendTok))) != 1 {
		t.Errorf("VTT-468: Bram's token on the board was not hidden before the withdrawal: %v", frames)
	}
	qacsCheckReintroduced(t, "VTT-468", frames, v.seq, qacsFriend, qacsFriendTok, qacsStripped(v.world.Actors[qacsFriend]))
	qacsHasNoStatblock(t, "VTT-468", v.shown.Actors[qacsFriend])
	frames = v.step(qacsPool(qacsFriend, "vigor", -2, 7))
	if len(qacsFind(frames, qacsPoolOf(qacsFriend))) != 0 {
		t.Errorf("VTT-466: a ResourceChanged of the actor made non-party was sent: %v", frames)
	}
	v.move(qacsHeroTok, 2, 2)
	v.step(qacsNarrate("Bram's eyes go dark."))
	qacsHasNoStatblock(t, "VTT-468", v.shown.Actors[qacsFriend])
	for _, s := range v.steps[mark:] {
		for _, f := range s {
			raw, _ := protojson.Marshal(f)
			if strings.Contains(string(raw), "Bram Holt") || strings.Contains(string(raw), "focus") {
				t.Errorf("VTT-465: Bram's name or statblock sent after the grant: %s", raw)
			}
		}
	}
}

// VTT-481 VTT-471
func TestQACSAKindChangeOutOfSightWaitsUntilTheActorComesIntoSight(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsBeastTok, 5, 0)
	frames := v.step(qacsGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	if bad := append(qacsFind(frames, qacsAddedOf(qacsBeast)), qacsFind(frames, qacsRemovedOf(qacsBeast))...); len(bad) != 0 {
		t.Errorf("VTT-481: the creature made a party member out of sight arrived at once: %v", frames)
	}
	held := v.shown.Actors[qacsBeast]
	qacsHasNoStatblock(t, "VTT-471", held)
	if held.GetKind() != vttv1.ActorKind_ACTOR_KIND_NON_PARTY || !qacsSame(held.GetControllerIds(), []string{"p-agent", "p-zed"}) {
		t.Errorf("VTT-471: the unseen creature is held as %v", held)
	}
	v.step(qacsPool(qacsBeast, "grit", -1, 3))
	frames = v.move(qacsBeastTok, 2, 0)
	qacsCheckReintroduced(t, "VTT-481", frames, v.seq, qacsBeast, qacsBeastTok, qacsWhole(v.world.Actors[qacsBeast]))
	if got := v.shown.Actors[qacsBeast].GetResources()["grit"].GetCurrent(); got != 3 {
		t.Errorf("VTT-469: a party member coming into sight holds grit %d, want 3", got)
	}

	frames = v.step(qacsGrant(qacsFriend, "p-bram", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	if bad := append(qacsFind(frames, qacsAddedOf(qacsFriend)), qacsFind(frames, qacsRemovedOf(qacsFriend))...); len(bad) != 0 {
		t.Errorf("VTT-481: the party member made non-party out of sight was re-sent at once: %v", frames)
	}
	if b := v.shown.Actors[qacsFriend]; b.GetName() != "Bram Holt" || b.GetKind() != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER ||
		b.GetResources()["vigor"].GetCurrent() != 9 {
		t.Errorf("VTT-471: the unseen party member is not held as at its introduction: %v", b)
	}
	frames = v.move(qacsFriendTok, 0, 1)
	qacsCheckReintroduced(t, "VTT-481", frames, v.seq, qacsFriend, qacsFriendTok, qacsStripped(v.world.Actors[qacsFriend]))
	qacsHasNoStatblock(t, "VTT-481", v.shown.Actors[qacsFriend])
	v.noSecrets(qacsGMSecrets)
}

func qacsEnvelopeIsBare(t *testing.T, label string, f *vttv1.Envelope) {
	t.Helper()
	if f.GetEventId() != "" || f.GetActorRole() != "" || f.GetParticipantId() != "" || f.GetSessionId() != "" {
		t.Errorf("%s: an up-to-date frame carries an event id, role, participant or session: %v", label, f)
	}
	if f.GetResourceChanged().GetReason() != "" || f.GetConditionApplied().GetSource() != "" ||
		f.GetConditionRemoved().GetReason() != "" {
		t.Errorf("%s: an up-to-date frame carries a reason or source: %v", label, f)
	}
}

// VTT-471 VTT-469 VTT-249
func TestQACSAnUnseenCreatureIsHeldAsLastSeenAndBroughtToItsPresentOnSight(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsBeastTok, 5, 0)
	for _, env := range []*vttv1.Envelope{
		qacsUncond(qacsBeast, "prone"),
		qacsCond(qacsBeast, "hidden"),
		qacsRevoke(qacsBeast, "p-zed"),
		qacsGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qacsPool(qacsBeast, "grit", -3, 1),
	} {
		if frames := v.step(env); len(frames) != 0 {
			t.Errorf("VTT-243, SPEC-016: an unseen creature's change sent %v", frames)
		}
		held := v.shown.Actors[qacsBeast]
		if !qacsSame(qacsConds(v.shown, qacsBeast), []string{"prone", "dazed"}) ||
			!qacsSame(held.GetControllerIds(), []string{"p-agent", "p-zed"}) ||
			held.GetKind() != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
			t.Errorf("VTT-471: the unseen creature is held as %v with %v", held, qacsConds(v.shown, qacsBeast))
		}
		qacsHasNoStatblock(t, "VTT-471", held)
	}
	frames := v.move(qacsBeastTok, 2, 0)
	if bad := append(qacsFind(frames, qacsAddedOf(qacsBeast)), qacsFind(frames, qacsRemovedOf(qacsBeast))...); len(bad) != 0 {
		t.Errorf("VTT-469: a creature of the same kind was re-introduced rather than brought up to date: %v", frames)
	}
	if len(qacsFind(frames, qacsPoolOf(qacsBeast))) != 0 {
		t.Errorf("VTT-466: the creature's correction carries a ResourceChanged: %v", frames)
	}
	for _, f := range frames {
		if qacsActorOf(f) == qacsBeast && f.GetTokenPlaced() == nil {
			qacsEnvelopeIsBare(t, "VTT-249", f)
		}
	}
	for _, f := range frames {
		if g := f.GetActorControlGranted(); g != nil && g.GetActorId() == qacsBeast &&
			g.GetKind() != vttv1.ActorKind_ACTOR_KIND_NON_PARTY {
			t.Errorf("VTT-469: a corrected grant carries kind %v, want the present one", g.GetKind())
		}
	}
	held := v.shown.Actors[qacsBeast]
	if !qacsSame(qacsConds(v.shown, qacsBeast), []string{"dazed", "hidden"}) {
		t.Errorf("VTT-469: conditions on sight %v, want dazed and hidden", qacsConds(v.shown, qacsBeast))
	}
	if !qacsSame(held.GetControllerIds(), []string{"p-agent", "p-ayla"}) {
		t.Errorf("VTT-469: controllers on sight %v, want p-agent and p-ayla", held.GetControllerIds())
	}
	qacsHasNoStatblock(t, "VTT-469", held)
	v.noSecrets(qacsSecrets)
}

// VTT-471 VTT-469 VTT-249
func TestQACSAnUnseenPartyMemberIsHeldAtItsResourcesThenAndBroughtToThemOnSight(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsFriendTok, 0, 0)
	v.move(qacsFriendTok, 5, 2)
	if _, ok := v.shown.Tokens[qacsFriendTok]; ok {
		t.Fatalf("fixture: Bram's token is still on the board")
	}
	v.step(qacsPool(qacsFriend, "vigor", -4, 5))
	v.step(qacsCond(qacsFriend, "weary"))
	if got := v.shown.Actors[qacsFriend].GetResources()["vigor"].GetCurrent(); got != 9 {
		t.Errorf("VTT-471: unseen Bram is held at vigor %d, want 9", got)
	}
	if len(qacsConds(v.shown, qacsFriend)) != 0 {
		t.Errorf("VTT-471: unseen Bram is held with %v", qacsConds(v.shown, qacsFriend))
	}
	frames := v.move(qacsFriendTok, 0, 0)
	is := qacsFind(frames, qacsPoolOf(qacsFriend))
	if len(is) != 1 {
		t.Fatalf("VTT-469: want one ResourceChanged of Bram on sight, got %v", frames)
	}
	rc := frames[is[0]].GetResourceChanged()
	if rc.GetResource() != "vigor" || rc.GetDelta() != -4 || rc.GetNewValue() != 5 {
		t.Errorf("VTT-469: the correction is %v", rc)
	}
	for _, f := range frames {
		if qacsActorOf(f) == qacsFriend && f.GetTokenPlaced() == nil {
			qacsEnvelopeIsBare(t, "VTT-249", f)
		}
	}
	if got := v.shown.Actors[qacsFriend].GetResources()["vigor"].GetCurrent(); got != 5 {
		t.Errorf("VTT-469: Bram on sight is held at vigor %d, want 5", got)
	}
	if !qacsSame(qacsConds(v.shown, qacsFriend), []string{"weary"}) {
		t.Errorf("VTT-469: Bram on sight holds %v", qacsConds(v.shown, qacsFriend))
	}
}

// VTT-469 VTT-465
func TestQACSAPerchBringsACreatureToItsPresentAndIntroducesAnUnseenOneStripped(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsBeastTok, 4, 1)
	v.step(qacsCond(qacsBeast, "hidden"))
	v.step(qacsAdd(qacsRatActor(t)))
	v.step(qacsPlace(qacsRatTok, qacsRat, 6, 2, "a small shape", "", "Squeak", "whisker twitch"))
	if _, ok := v.shown.Actors[qacsRat]; ok {
		t.Fatalf("fixture: Ayla's log holds the rat before the perch")
	}
	frames := v.perch(qacsFriend)
	if bad := append(qacsFind(frames, qacsAddedOf(qacsBeast)), qacsFind(frames, qacsRemovedOf(qacsBeast))...); len(bad) != 0 {
		t.Errorf("VTT-469: the held creature was re-introduced by the perch: %v", frames)
	}
	if !qacsSame(qacsConds(v.shown, qacsBeast), []string{"prone", "dazed", "hidden"}) {
		t.Errorf("VTT-469: after the perch the creature holds %v", qacsConds(v.shown, qacsBeast))
	}
	qacsHasNoStatblock(t, "VTT-469", v.shown.Actors[qacsBeast])
	add := qacsFind(frames, qacsAddedOf(qacsRat))
	if len(add) != 1 {
		t.Fatalf("VTT-465: the perch did not introduce the rat: %v", frames)
	}
	if got, want := frames[add[0]].GetActorAdded().GetActor(), qacsStripped(v.world.Actors[qacsRat]); !proto.Equal(got, want) {
		t.Errorf("VTT-465: the perch introduced the rat as %v, want %v", got, want)
	}
	if is := qacsFind(frames, qacsPlacedOf(qacsBeastTok)); len(is) != 1 {
		t.Errorf("VTT-478: the creature's token not placed by the perch: %v", frames)
	} else if p := frames[is[0]].GetTokenPlaced(); p.GetName() != "a hunched figure" || p.GetGmName() != "" {
		t.Errorf("VTT-478 VTT-462: placed by the perch as %v", p)
	}
	v.noSecrets(qacsSecrets)
}

// VTT-470
func TestQACSACreatureAddedAgainWithItsModuleIsCorrectedNotReintroduced(t *testing.T) {
	v := qacsTable(t)
	v.step(qacsRemoveToken(qacsBeastTok))
	if frames := v.step(qacsRemoveActor(qacsBeast)); len(frames) != 0 {
		t.Errorf("VTT-251: an unseen actor's removal sent %v", frames)
	}
	again := &vttv1.Actor{
		ActorId: qacsBeast, Name: "Gorvash Reborn", ModuleId: "mod-goblin",
		Attributes: map[string]int32{"brawn": 9},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 1, Max: 9}},
		Kind:       vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
	}
	if frames := v.step(qacsAdd(again)); len(frames) != 0 {
		t.Errorf("VTT-470: an unseen creature added again sent %v", frames)
	}
	frames := v.step(qacsPlace("tok-gob-2", qacsBeast, 2, 1, "a hunched figure", "", "", ""))
	if bad := append(qacsFind(frames, qacsAddedOf(qacsBeast)), qacsFind(frames, qacsRemovedOf(qacsBeast))...); len(bad) != 0 {
		t.Errorf("VTT-470: a creature added again under the same module was re-introduced: %v", frames)
	}
	held := v.shown.Actors[qacsBeast]
	qacsHasNoStatblock(t, "VTT-470", held)
	if len(qacsConds(v.shown, qacsBeast)) != 0 || len(held.GetControllerIds()) != 0 {
		t.Errorf("VTT-470: the corrected creature holds %v and %v", qacsConds(v.shown, qacsBeast), held.GetControllerIds())
	}
	v.noSecrets(append(qacsSecrets, "Gorvash Reborn"))
}

// VTT-470 VTT-252
func TestQACSACreatureAddedAgainWithAnotherModuleIsIntroducedAfresh(t *testing.T) {
	v := qacsTable(t)
	v.step(qacsRemoveToken(qacsBeastTok))
	v.step(qacsRemoveActor(qacsBeast))
	again := qacsBeastActor(t)
	again.ModuleId = "mod-ogre"
	if frames := v.step(qacsAdd(again)); len(frames) != 0 {
		t.Errorf("VTT-470: an unseen creature added again sent %v", frames)
	}
	frames := v.step(qacsPlace("tok-gob-2", qacsBeast, 2, 1, "a looming shape", "", "", ""))
	qacsCheckReintroduced(t, "VTT-470", frames, v.seq, qacsBeast, "tok-gob-2", qacsStripped(v.world.Actors[qacsBeast]))
	if got := v.shown.Actors[qacsBeast].GetModuleId(); got != "mod-ogre" {
		t.Errorf("VTT-470: the fold holds module %q", got)
	}
	v.noSecrets(qacsSecrets)
}

// VTT-470
func TestQACSAPartyMemberAddedAgainIsIntroducedAfreshOrCorrected(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsFriendTok, 0, 0)
	v.step(qacsRemoveToken(qacsFriendTok))
	v.step(qacsRemoveActor(qacsFriend))
	same := qacsFriendActor(t)
	same.Resources["vigor"].Current = 2
	v.step(qacsAdd(same))
	frames := v.step(qacsPlace("tok-bram-2", qacsFriend, 0, 0, "Bram", "", "", ""))
	all := append(append([]*vttv1.Envelope{}, v.steps[len(v.steps)-2]...), frames...)
	if bad := append(qacsFind(all, qacsAddedOf(qacsFriend)), qacsFind(all, qacsRemovedOf(qacsFriend))...); len(bad) != 0 {
		t.Errorf("VTT-470: a party member added again as the same was re-introduced: %v", all)
	}
	if got := v.shown.Actors[qacsFriend].GetResources()["vigor"].GetCurrent(); got != 2 {
		t.Errorf("VTT-470: the corrected party member holds vigor %d, want 2", got)
	}

	v.step(qacsRemoveToken("tok-bram-2"))
	v.step(qacsRemoveActor(qacsFriend))
	other := qacsFriendActor(t)
	other.Name = "Bram the Second"
	v.step(qacsAdd(other))
	addFrames := v.steps[len(v.steps)-1]
	frames = v.step(qacsPlace("tok-bram-3", qacsFriend, 0, 0, "Bram", "", "", ""))
	both := append(append([]*vttv1.Envelope{}, addFrames...), frames...)
	qacsCheckReintroduced(t, "VTT-470", both, 0, qacsFriend, "", qacsWhole(v.world.Actors[qacsFriend]))
	if got := v.shown.Actors[qacsFriend].GetName(); got != "Bram the Second" {
		t.Errorf("VTT-470: the fold holds %q", got)
	}
}

// VTT-312 VTT-470
func TestQACSAResumedProjectionHoldingACreatureSendsWhatTheFedOneSends(t *testing.T) {
	v := qacsTable(t)
	shown := v.shown.Snapshot()
	resumed := perceive.Resume(v.eyes, shown, v.world.Snapshot())
	prefix := perceive.New(v.eyes, v.world.Snapshot())
	shownNew := v.shown.Snapshot()
	run := []*vttv1.Envelope{
		qacsCond(qacsBeast, "hidden"),
		qacsPool(qacsBeast, "grit", -1, 3),
		qacsGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_NON_PARTY),
		qacsNarrate("Smoke drifts."),
	}
	for n, env := range run {
		fed := v.step(env)
		got := resumed.Event(v.eyes, env, shown, v.world)
		gotNew := prefix.Event(v.eyes, env, shownNew, v.world)
		for _, pair := range []struct {
			label  string
			frames []*vttv1.Envelope
			fold   *engine.State
		}{{"Resume", got, shown}, {"VTT-312 New", gotNew, shownNew}} {
			for i, f := range pair.frames {
				if err := engine.Apply(pair.fold, f); err != nil {
					t.Fatalf("%s: frame %d does not fold: %v", pair.label, i, err)
				}
			}
			if bad := append(qacsFind(pair.frames, qacsAddedOf(qacsBeast)), qacsFind(pair.frames, qacsRemovedOf(qacsBeast))...); len(bad) != 0 {
				t.Errorf("%s, VTT-470: event %d sent %v", pair.label, n+1, pair.frames)
			}
			if len(pair.frames) != len(fed) {
				t.Errorf("%s: event %d sent %d frames, the fed projection %d:\n%v\nvs\n%v", pair.label, n+1, len(pair.frames), len(fed), pair.frames, fed)
				continue
			}
			for i := range fed {
				if !proto.Equal(pair.frames[i], fed[i]) {
					t.Errorf("%s: event %d frame %d is %v, the fed projection's %v", pair.label, n+1, i, pair.frames[i], fed[i])
				}
			}
		}
	}
}

// VTT-468
func TestQACSAKindChangeInSightHidesAndPlacesAgainEveryTokenOfTheActorOnTheBoard(t *testing.T) {
	v := qacsTable(t)
	v.step(qacsPlace("tok-gob-b", qacsBeast, 0, 2, "a second shadow", "", "", ""))
	v.step(&vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "attic", Name: "Attic", GridWidth: 2, GridHeight: 2,
	}}})
	v.step(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "tok-gob-c", SceneId: "attic", ActorId: qacsBeast, Position: &vttv1.GridPosition{X: 0, Y: 0},
	}}})
	if _, ok := v.shown.Tokens["tok-gob-b"]; !ok {
		t.Fatalf("fixture: the creature's second token is not on the board")
	}
	frames := v.step(qacsGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	rem := qacsFind(frames, qacsRemovedOf(qacsBeast))
	add := qacsFind(frames, qacsAddedOf(qacsBeast))
	if len(rem) != 1 || len(add) != 1 {
		t.Fatalf("VTT-468: want one withdrawal and one introduction, got %v", frames)
	}
	for _, tok := range []string{qacsBeastTok, "tok-gob-b"} {
		hid := qacsFind(frames, qacsHiddenOf(tok))
		placed := qacsFind(frames, qacsPlacedOf(tok))
		if len(hid) != 1 || hid[0] > rem[0] {
			t.Errorf("VTT-468: %s not hidden before the withdrawal: %v", tok, frames)
		}
		if len(placed) != 1 || placed[0] < add[0] {
			t.Errorf("VTT-468: %s not placed again after the introduction: %v", tok, frames)
		}
	}
	if n := len(qacsFind(frames, qacsPlacedOf("tok-gob-c"))) + len(qacsFind(frames, qacsHiddenOf("tok-gob-c"))); n != 0 {
		t.Errorf("VTT-468: a token of the actor never on the board was hidden or placed: %v", frames)
	}
}

// VTT-481 VTT-466
func TestQACSAPartyMemberWhoseResourcesChangedAndWhoLeftThePartyUnseenArrivesStripped(t *testing.T) {
	v := qacsTable(t)
	v.step(qacsPool(qacsFriend, "vigor", -4, 5))
	v.step(qacsGrant(qacsFriend, "p-bram", vttv1.ActorKind_ACTOR_KIND_NON_PARTY))
	v.step(qacsPool(qacsFriend, "focus", 1, 2))
	frames := v.move(qacsFriendTok, 0, 0)
	if is := qacsFind(frames, qacsPoolOf(qacsFriend)); len(is) != 0 {
		t.Errorf("VTT-466: an actor that is not a party member after the event was sent a ResourceChanged: %v", frames)
	}
	qacsCheckReintroduced(t, "VTT-481", frames, v.seq, qacsFriend, qacsFriendTok, qacsStripped(v.world.Actors[qacsFriend]))
	qacsHasNoStatblock(t, "VTT-481", v.shown.Actors[qacsFriend])
}

// VTT-481 VTT-469
func TestQACSAPerchOntoACreatureWhoseKindChangedUnseenWithdrawsAndIntroducesIt(t *testing.T) {
	v := qacsTable(t)
	v.move(qacsBeastTok, 4, 1)
	v.step(qacsGrant(qacsBeast, "p-ayla", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER))
	if v.shown.Actors[qacsBeast].GetName() != "" {
		t.Fatalf("VTT-481: the creature arrived whole before it came into sight")
	}
	frames := v.perch(qacsFriend)
	qacsCheckReintroduced(t, "VTT-481", frames, v.seq, qacsBeast, qacsBeastTok, qacsWhole(v.world.Actors[qacsBeast]))
	if got := v.shown.Actors[qacsBeast].GetName(); got != "Gorvash the Sly" {
		t.Errorf("VTT-481: after the perch the fold names the creature %q", got)
	}
}

// VTT-470
func TestQACSAnIdAddedAgainUnderTheOtherKindIsIntroducedAfreshWhenSeen(t *testing.T) {
	v := qacsTable(t)
	v.step(qacsRemoveToken(qacsBeastTok))
	v.step(qacsRemoveActor(qacsBeast))
	again := qacsBeastActor(t)
	again.Kind = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	addFrames := v.step(qacsAdd(again))
	frames := v.step(qacsPlace("tok-gob-2", qacsBeast, 2, 1, "a hunched figure", "", "", ""))
	both := append(append([]*vttv1.Envelope{}, addFrames...), frames...)
	qacsCheckReintroduced(t, "VTT-470", both, 0, qacsBeast, "", qacsWhole(v.world.Actors[qacsBeast]))
	if got := v.shown.Actors[qacsBeast].GetName(); got != "Gorvash the Sly" {
		t.Errorf("VTT-470: the fold names the party member added again %q", got)
	}

	v.move(qacsFriendTok, 0, 0)
	v.step(qacsRemoveToken(qacsFriendTok))
	v.step(qacsRemoveActor(qacsFriend))
	creature := qacsFriendActor(t)
	creature.Kind = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
	addFrames = v.step(qacsAdd(creature))
	frames = v.step(qacsPlace("tok-bram-2", qacsFriend, 0, 0, "a tall shape", "", "", ""))
	both = append(append([]*vttv1.Envelope{}, addFrames...), frames...)
	qacsCheckReintroduced(t, "VTT-470", both, 0, qacsFriend, "", qacsStripped(v.world.Actors[qacsFriend]))
	qacsHasNoStatblock(t, "VTT-470", v.shown.Actors[qacsFriend])
}

// VTT-312 VTT-468
func TestQACSAResumeAfterADroppedKindChangeOfACreatureInSightRewritesIt(t *testing.T) {
	label := "VTT-312 VTT-468"
	for _, tc := range []struct {
		name, id, tok string
		kind          vttv1.ActorKind
		whole         bool
	}{
		{"creature to party member", qacsBeast, qacsBeastTok, vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, true},
		{"party member to creature", qacsFriend, qacsFriendTok, vttv1.ActorKind_ACTOR_KIND_NON_PARTY, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := qacsTable(t)
			v.move(qacsFriendTok, 0, 0)
			dropped := v.stamp(qacsGrant(tc.id, "p-ayla", tc.kind))
			if err := engine.Apply(v.world, dropped); err != nil {
				t.Fatalf("world refused the grant: %v", err)
			}
			v.pr = perceive.Resume(v.eyes, v.shown, v.world)
			frames := v.step(qacsNarrate("The fire crackles."))
			want := qacsStripped(v.world.Actors[tc.id])
			if tc.whole {
				want = qacsWhole(v.world.Actors[tc.id])
			}
			qacsCheckReintroduced(t, label, frames, v.seq, tc.id, tc.tok, want)
		})
	}
}
