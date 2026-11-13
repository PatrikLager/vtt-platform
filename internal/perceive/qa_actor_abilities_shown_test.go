package perceive_test

import (
	"fmt"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const (
	qaAblScene = "hall"
	qaAblP1    = "p1"
	qaAblP2    = "p2"
	qaAblParty = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaAblOther = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

var qaAblHeroEyes = perceive.Eyes{Shoulder: "hero"}

type qaAblWorld struct {
	t   *testing.T
	st  *engine.State
	seq int64
}

func qaAblNewWorld(t *testing.T) *qaAblWorld {
	t.Helper()
	return &qaAblWorld{t: t, st: engine.NewState()}
}

func (w *qaAblWorld) append(env *vttv1.Envelope) *vttv1.Envelope {
	w.t.Helper()
	w.seq++
	env.Sequence = w.seq
	env.EventId = fmt.Sprintf("ev-%d", w.seq)
	env.ParticipantId = "dm-1"
	env.ActorRole = "dm"
	if err := engine.Apply(w.st, env); err != nil {
		w.t.Fatalf("world refused event %d: %v", w.seq, err)
	}
	return env
}

func (w *qaAblWorld) step(env *vttv1.Envelope, logs ...*qaAblLog) []*vttv1.Envelope {
	w.t.Helper()
	w.append(env)
	var first []*vttv1.Envelope
	for i, l := range logs {
		frames := l.event(env, w.st)
		if i == 0 {
			first = frames
		}
	}
	return first
}

type qaAblLog struct {
	t     *testing.T
	eyes  perceive.Eyes
	pr    *perceive.Projection
	shown *engine.State
	seq   int64
	all   []*vttv1.Envelope
}

func qaAblFreshLog(t *testing.T, eyes perceive.Eyes) *qaAblLog {
	t.Helper()
	return &qaAblLog{t: t, eyes: eyes, pr: perceive.New(eyes, nil), shown: engine.NewState()}
}

func (l *qaAblLog) fold(frames []*vttv1.Envelope) {
	l.t.Helper()
	for _, f := range frames {
		c, ok := proto.Clone(f).(*vttv1.Envelope)
		if !ok {
			l.t.Fatalf("clone of a frame is not an envelope")
		}
		l.seq++
		c.Sequence = l.seq
		if err := engine.Apply(l.shown, c); err != nil {
			l.t.Fatalf("the log's fold refused frame %v: %v", f, err)
		}
		l.all = append(l.all, f)
	}
}

func (l *qaAblLog) event(env *vttv1.Envelope, st *engine.State) []*vttv1.Envelope {
	l.t.Helper()
	frames := l.pr.Event(l.eyes, env, l.shown, st)
	l.fold(frames)
	return frames
}

func (l *qaAblLog) held(id string) *vttv1.Actor {
	l.t.Helper()
	a, ok := l.shown.Actors[id]
	if !ok {
		l.t.Fatalf("the log holds no actor %q", id)
	}
	return a
}

func qaAblSceneEnv() *vttv1.Envelope {
	tiles := map[string]*vttv1.TileRef{}
	for x := int32(0); x < 9; x++ {
		for y := int32(0); y < 3; y++ {
			kind := "floor"
			if x == 4 {
				kind = "wall"
				if y == 1 {
					kind = "door"
				}
			}
			tiles[fmt.Sprintf("%d,%d", x, y)] = &vttv1.TileRef{Kind: kind}
		}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: qaAblScene, Name: "Hall", GridWidth: 9, GridHeight: 3, Tiles: tiles,
	}}}
}

func qaAblAdd(a *vttv1.Actor) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}}
}

func qaAblGrant(id, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
		ActorId: id, ParticipantId: participant, Kind: kind,
	}}}
}

func qaAblPlace(token, actor string, x, y int32) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: token, SceneId: qaAblScene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y}, Name: token,
	}}}
}

func qaAblUnplace(token string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: token}}}
}

func qaAblRemove(actor string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}}
}

func qaAblDoor(open bool) *vttv1.Envelope {
	at := &vttv1.GridPosition{X: 4, Y: 1}
	if open {
		return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{SceneId: qaAblScene, At: at}}}
	}
	return &vttv1.Envelope{Payload: &vttv1.Envelope_DoorClosed{DoorClosed: &vttv1.DoorClosed{SceneId: qaAblScene, At: at}}}
}

func qaAblCondition(actor, condition string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
		ActorId: actor, ConditionId: condition, Source: "a trap",
	}}}
}

func qaAblForwarded(frames []*vttv1.Envelope, env *vttv1.Envelope) bool {
	for _, f := range frames {
		if f.GetEventId() == env.GetEventId() {
			return true
		}
	}
	return false
}

func qaAblHeldOnce(t *testing.T, l *qaAblLog, actor, condition string) {
	t.Helper()
	n := 0
	for _, c := range l.shown.Conditions[actor] {
		if c.ID == condition {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the log holds %q on %q %d times, want once: %v", condition, actor, n, l.shown.Conditions[actor])
	}
}

func qaAblSay(text string) *vttv1.Envelope {
	return &vttv1.Envelope{Payload: &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}}
}

func qaAblHero() *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: "hero", Name: "Hero", ModuleId: "mod-hero",
		Attributes: map[string]int32{"might": 3},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 4, Max: 6}},
		Kind:       qaAblParty,
		AbilityIds: []string{"second-wind", "fists"},
	}
}

func qaAblAlly(abilities ...string) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: "ally", Name: "Ally", ModuleId: "mod-ally",
		Attributes: map[string]int32{"agility": 2},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 3, Max: 5}},
		ModuleData: &structpb.Struct{Fields: map[string]*structpb.Value{"lore": structpb.NewStringValue("kept")}},
		Kind:       qaAblParty,
		AbilityIds: abilities,
	}
}

func qaAblBareActor(id, name string, abilities ...string) *vttv1.Actor {
	return &vttv1.Actor{ActorId: id, Name: name, ModuleId: "mod-" + id, Kind: qaAblParty, AbilityIds: abilities}
}

func qaAblGob() *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: "gob", Name: "Gob", ModuleId: "mod-gob",
		Attributes: map[string]int32{"might": 1},
		Resources:  map[string]*vttv1.Resource{"grit": {Current: 2, Max: 2}},
		ModuleData: &structpb.Struct{Fields: map[string]*structpb.Value{"lair": structpb.NewStringValue("cave")}},
		Kind:       qaAblOther,
		AbilityIds: []string{"scimitar", "bite"},
	}
}

// qaAblCreatureForm is SPEC-016's shownWith: id, module id, kind and controllers.
func qaAblCreatureForm(a *vttv1.Actor) *vttv1.Actor {
	return &vttv1.Actor{
		ActorId: a.GetActorId(), ModuleId: a.GetModuleId(), Kind: a.GetKind(),
		ControllerId: a.GetControllerId(), ControllerIds: a.GetControllerIds(),
	}
}

// qaAblUnseenForm is SPEC-016's unseenAs: the creature form with the name.
func qaAblUnseenForm(a *vttv1.Actor) *vttv1.Actor {
	f := qaAblCreatureForm(a)
	f.Name = a.GetName()
	return f
}

func qaAblIntroductions(frames []*vttv1.Envelope, id string) []*vttv1.Actor {
	var out []*vttv1.Actor
	for _, f := range frames {
		if a := f.GetActorAdded().GetActor(); a != nil && a.GetActorId() == id {
			out = append(out, a)
		}
	}
	return out
}

func qaAblWithdrawals(frames []*vttv1.Envelope, id string) int {
	n := 0
	for _, f := range frames {
		if r := f.GetActorRemoved(); r != nil && r.GetActorId() == id {
			n++
		}
	}
	return n
}

func qaAblSame(t *testing.T, what string, got, want *vttv1.Actor) {
	t.Helper()
	if !proto.Equal(got, want) {
		t.Errorf("%s:\n got  %v\n want %v", what, got, want)
	}
}

func qaAblNoListIntroduced(t *testing.T, l *qaAblLog, id string) {
	t.Helper()
	intros := qaAblIntroductions(l.all, id)
	if len(intros) == 0 {
		t.Fatalf("the log was never introduced to %q", id)
	}
	for _, a := range intros {
		if len(a.GetAbilityIds()) != 0 {
			t.Errorf("an introduction of %q carries ability_ids %v", id, a.GetAbilityIds())
		}
	}
}

func qaAblListGuard(t *testing.T, a *vttv1.Actor) {
	t.Helper()
	if len(a.GetAbilityIds()) < 2 || slices.IsSorted(a.GetAbilityIds()) {
		t.Fatalf("fixture: %q's list %v must hold two or more ids out of sorted order", a.GetActorId(), a.GetAbilityIds())
	}
}

// qaAblHall builds the hall with the hero at 1,1 west of a closed door at 4,1.
func qaAblHall(w *qaAblWorld, logs ...*qaAblLog) {
	w.t.Helper()
	w.step(qaAblSceneEnv(), logs...)
	w.step(qaAblAdd(qaAblHero()), logs...)
	w.step(qaAblGrant("hero", qaAblP1, qaAblParty), logs...)
	w.step(qaAblPlace("tok-hero", "hero", 1, 1), logs...)
}

// qaAblAddCompanion grants the companion to qaAblP2 unless it is the list-only one.
func qaAblAddCompanion(w *qaAblWorld, a *vttv1.Actor, x, y int32, logs ...*qaAblLog) {
	w.t.Helper()
	w.step(qaAblAdd(a), logs...)
	if len(a.GetAttributes()) > 0 {
		w.step(qaAblGrant(a.GetActorId(), qaAblP2, qaAblParty), logs...)
	}
	w.step(qaAblPlace("tok-"+a.GetActorId(), a.GetActorId(), x, y), logs...)
}

// VTT-698
func TestQAAblACreatureInSightIsIntroducedWithNoAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	w.step(qaAblAdd(qaAblGob()), l)
	w.step(qaAblPlace("tok-gob", "gob", 2, 1), l)

	qaAblListGuard(t, w.st.Actors["gob"])
	qaAblNoListIntroduced(t, l, "gob")
	qaAblSame(t, "the creature in sight", l.held("gob"), qaAblCreatureForm(w.st.Actors["gob"]))
}

// VTT-698
func TestQAAblACreatureComingIntoSightIsIntroducedWithNoAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	w.step(qaAblAdd(qaAblGob()), l)
	w.step(qaAblPlace("tok-gob", "gob", 7, 1), l)
	if _, ok := l.shown.Actors["gob"]; ok {
		t.Fatalf("fixture: the creature behind the closed door is held before it is seen: %v", l.shown.Actors["gob"])
	}

	w.step(qaAblDoor(true), l)

	qaAblNoListIntroduced(t, l, "gob")
	qaAblSame(t, "the creature once seen", l.held("gob"), qaAblCreatureForm(w.st.Actors["gob"]))
}

// VTT-698
func TestQAAblACreatureSeenAgainIsNotIntroducedAgain(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	w.step(qaAblAdd(qaAblGob()), l)
	w.step(qaAblPlace("tok-gob", "gob", 7, 1), l)
	w.step(qaAblDoor(true), l)
	w.step(qaAblDoor(false), l)

	again := w.step(qaAblDoor(true), l)

	if n := len(qaAblIntroductions(again, "gob")) + qaAblWithdrawals(again, "gob"); n != 0 {
		t.Errorf("seeing the unchanged creature again sent %d withdrawal or introduction frames: %v", n, again)
	}
	qaAblSame(t, "the creature seen again", l.held("gob"), qaAblCreatureForm(w.st.Actors["gob"]))
}

// VTT-698
func TestQAAblACompanionNeverSeenIsIntroducedWithNoAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 7, 1, l)
	qaAblAddCompanion(w, qaAblBareActor("bare", "Bare", "shove", "dodge"), 7, 0, l)
	w.step(qaAblSay("the hall is quiet"), l)

	asAdded := map[string]*vttv1.Actor{"ally": qaAblAlly("rally", "fists"), "bare": qaAblBareActor("bare", "Bare", "shove", "dodge")}
	for id, added := range asAdded {
		qaAblListGuard(t, w.st.Actors[id])
		qaAblNoListIntroduced(t, l, id)
		qaAblSame(t, "the companion never seen", l.held(id), qaAblUnseenForm(added))
	}
}

// VTT-698
func TestQAAblALogsOpeningIntroducesEveryPartyMemberWithNoAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	qaAblHall(w)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 2, 1)
	qaAblAddCompanion(w, qaAblBareActor("bare", "Bare", "shove", "dodge"), 2, 0)

	l := qaAblFreshLog(t, perceive.Eyes{})
	l.fold(l.pr.Perch(perceive.Eyes{}, l.shown, w.st, 0))

	for _, id := range []string{"hero", "ally", "bare"} {
		qaAblNoListIntroduced(t, l, id)
		qaAblSame(t, "a party member in the opening", l.held(id), qaAblUnseenForm(w.st.Actors[id]))
	}
}

// VTT-699
func TestQAAblACompanionSeenFromItsIntroductionIsHeldWithItsAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	qaAblHall(w)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 2, 1)
	qaAblAddCompanion(w, qaAblBareActor("bare", "Bare", "shove", "dodge"), 2, 0)

	l := qaAblFreshLog(t, qaAblHeroEyes)
	frames := w.step(qaAblSay("a late log"), l)

	for _, id := range []string{"hero", "ally", "bare"} {
		qaAblListGuard(t, w.st.Actors[id])
		intros := qaAblIntroductions(frames, id)
		if len(intros) != 1 || qaAblWithdrawals(frames, id) != 0 {
			t.Fatalf("%q: want one introduction and no withdrawal, got %v", id, frames)
		}
		if !slices.Equal(intros[0].GetAbilityIds(), w.st.Actors[id].GetAbilityIds()) {
			t.Errorf("%q is introduced with ability_ids %v, want %v", id, intros[0].GetAbilityIds(), w.st.Actors[id].GetAbilityIds())
		}
		qaAblSame(t, "a party member seen from its introduction", l.held(id), w.st.Actors[id])
	}
}

// VTT-699
func TestQAAblACompanionWithASheetFirstIntroducedUnseenIsBroughtItsAbilityIdsAtFirstSight(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 7, 1, l)
	qaAblSame(t, "fixture: the companion before sight", l.held("ally"), qaAblUnseenForm(qaAblAlly("rally", "fists")))

	w.step(qaAblDoor(true), l)

	qaAblSame(t, "the companion at first sight", l.held("ally"), w.st.Actors["ally"])
}

// VTT-699
func TestQAAblACompanionWhoseSheetNamesOnlyAbilitiesIsBroughtThemAtFirstSight(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblBareActor("bare", "Bare", "shove", "dodge"), 7, 1, l)
	world := w.st.Actors["bare"]
	qaAblListGuard(t, world)
	onlyTheList := qaAblUnseenForm(world)
	onlyTheList.AbilityIds = world.GetAbilityIds()
	if !proto.Equal(onlyTheList, world) {
		t.Fatalf("fixture: the companion must differ from its unseen form in its list alone: %v", world)
	}
	qaAblSame(t, "fixture: the companion before sight", l.held("bare"), qaAblUnseenForm(world))

	w.step(qaAblDoor(true), l)

	qaAblSame(t, "the list-only companion at first sight", l.held("bare"), w.st.Actors["bare"])
}

// VTT-699
func TestQAAblAnOwnCharacterWhoseSheetNamesOnlyAbilitiesIsBroughtThemAfterTheOpening(t *testing.T) {
	w := qaAblNewWorld(t)
	qaAblHall(w)
	w.append(qaAblAdd(qaAblBareActor("solo", "Solo", "sprint", "climb")))
	w.append(qaAblPlace("tok-solo", "solo", 2, 1))
	qaAblListGuard(t, w.st.Actors["solo"])
	before := w.st.Snapshot()

	l := qaAblFreshLog(t, perceive.Eyes{Shoulder: "solo"})
	l.fold(l.pr.Perch(perceive.Eyes{}, l.shown, before, 0))
	qaAblSame(t, "fixture: the own character in the opening", l.held("solo"), qaAblUnseenForm(before.Actors["solo"]))

	w.step(qaAblGrant("solo", qaAblP1, qaAblParty), l)

	qaAblSame(t, "the own character after its first event", l.held("solo"), w.st.Actors["solo"])
}

// VTT-660
func TestQAAblACompanionSeenThenOutOfSightKeepsItsAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 7, 1, l)
	qaAblAddCompanion(w, qaAblBareActor("bare", "Bare", "shove", "dodge"), 7, 0, l)
	w.step(qaAblDoor(true), l)
	seen := map[string]*vttv1.Actor{}
	for _, id := range []string{"ally", "bare"} {
		seen[id] = proto.Clone(w.st.Actors[id]).(*vttv1.Actor)
		qaAblSame(t, "fixture: the companion in sight", l.held(id), seen[id])
	}

	closing := w.step(qaAblDoor(false), l)
	reopening := w.step(qaAblDoor(true), l)

	for _, id := range []string{"ally", "bare"} {
		for _, frames := range [][]*vttv1.Envelope{closing, reopening} {
			if n := len(qaAblIntroductions(frames, id)) + qaAblWithdrawals(frames, id); n != 0 {
				t.Errorf("%q: leaving or re-entering sight sent %d withdrawal or introduction frames: %v", id, n, frames)
			}
		}
		qaAblSame(t, "the companion seen, lost and seen again", l.held(id), seen[id])
	}
}

// VTT-660
func TestQAAblACompanionOutOfSightIsHeldWithItsAbilityIdsAsLastSeen(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 7, 1, l)
	w.step(qaAblDoor(true), l)
	w.step(qaAblDoor(false), l)
	w.step(qaAblSay("time passes"), l)

	if got, want := l.held("ally").GetAbilityIds(), w.st.Actors["ally"].GetAbilityIds(); !slices.Equal(got, want) {
		t.Errorf("the companion out of sight is held with ability_ids %v, want %v as last seen", got, want)
	}
}

// VTT-699
func TestQAAblAResumeOfALogHoldingAListOnlyCompanionUnseenBringsItsAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	lagging := qaAblFreshLog(t, qaAblHeroEyes)
	current := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, lagging, current)
	qaAblAddCompanion(w, qaAblBareActor("bare", "Bare", "shove", "dodge"), 7, 1, lagging, current)
	w.step(qaAblDoor(true), current)
	qaAblSame(t, "fixture: the lagging log's companion", lagging.held("bare"), qaAblUnseenForm(w.st.Actors["bare"]))
	qaAblSame(t, "fixture: the current log's companion", current.held("bare"), w.st.Actors["bare"])

	lagging.pr = perceive.Resume(qaAblHeroEyes, lagging.shown, w.st)
	current.pr = perceive.Resume(qaAblHeroEyes, current.shown, w.st)
	dazed := w.append(qaAblCondition("bare", "dazed"))
	laggingFrames := lagging.event(dazed, w.st)
	currentFrames := current.event(dazed, w.st)

	qaAblSame(t, "the lagging log's companion after the resume", lagging.held("bare"), w.st.Actors["bare"])
	qaAblHeldOnce(t, lagging, "bare", "dazed")
	if qaAblForwarded(laggingFrames, dazed) {
		t.Errorf("the lagging log was forwarded the event, so its held companion was taken as current: %v", laggingFrames)
	}
	if len(currentFrames) != 1 || !qaAblForwarded(currentFrames, dazed) {
		t.Errorf("control: a resume of a log holding the companion as the world does sent %v, want the event forwarded alone", currentFrames)
	}
}

// VTT-699
func TestQAAblAResumeOfALogHoldingACompanionWholeWithoutItsListBringsIt(t *testing.T) {
	without := qaAblNewWorld(t)
	with := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(without, l)
	qaAblHall(with)
	qaAblAddCompanion(without, qaAblAlly(), 2, 1, l)
	qaAblAddCompanion(with, qaAblAlly("rally", "fists"), 2, 1)
	qaAblSame(t, "fixture: the log's companion", l.held("ally"), without.st.Actors["ally"])
	onlyTheList := proto.Clone(without.st.Actors["ally"]).(*vttv1.Actor)
	onlyTheList.AbilityIds = with.st.Actors["ally"].GetAbilityIds()
	qaAblSame(t, "fixture: the two worlds differ in the list alone", onlyTheList, with.st.Actors["ally"])
	control := qaAblLog{t: t, eyes: qaAblHeroEyes, shown: l.shown.Snapshot()}

	l.pr = perceive.Resume(qaAblHeroEyes, l.shown, with.st)
	dazed := with.append(qaAblCondition("ally", "dazed"))
	frames := l.event(dazed, with.st)
	control.pr = perceive.Resume(qaAblHeroEyes, control.shown, without.st)
	controlDazed := without.append(qaAblCondition("ally", "dazed"))
	controlFrames := control.event(controlDazed, without.st)

	qaAblSame(t, "the companion after a resume against a world that names its abilities", l.held("ally"), with.st.Actors["ally"])
	qaAblHeldOnce(t, l, "ally", "dazed")
	if qaAblForwarded(frames, dazed) {
		t.Errorf("the log was forwarded the event, so its held companion was taken as current: %v", frames)
	}
	if len(controlFrames) != 1 || !qaAblForwarded(controlFrames, controlDazed) {
		t.Errorf("control: a resume of a log holding the companion as the world does sent %v, want the event forwarded alone", controlFrames)
	}
}

// VTT-699
func TestQAAblACreatureMadeAPartyMemberInSightIsHeldWithItsAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	w.step(qaAblAdd(qaAblGob()), l)
	w.step(qaAblPlace("tok-gob", "gob", 2, 1), l)

	w.step(qaAblGrant("gob", qaAblP2, qaAblParty), l)

	qaAblSame(t, "the creature made a party member in sight", l.held("gob"), w.st.Actors["gob"])
}

// VTT-698
func TestQAAblAPartyMemberMadeACreatureInSightIsHeldWithNoAbilityIds(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 2, 1, l)
	qaAblSame(t, "fixture: the companion in sight", l.held("ally"), w.st.Actors["ally"])

	frames := w.step(qaAblGrant("ally", qaAblP2, qaAblOther), l)

	for _, a := range qaAblIntroductions(frames, "ally") {
		if len(a.GetAbilityIds()) != 0 {
			t.Errorf("the re-introduction carries ability_ids %v", a.GetAbilityIds())
		}
	}
	qaAblSame(t, "the companion made a creature in sight", l.held("ally"), qaAblCreatureForm(w.st.Actors["ally"]))
}

// VTT-698
func TestQAAblAPartyMemberMadeACreatureOutOfSightLosesItsAbilityIdsAtSight(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 7, 1, l)
	w.step(qaAblDoor(true), l)
	lastSeen := proto.Clone(w.st.Actors["ally"]).(*vttv1.Actor)
	w.step(qaAblDoor(false), l)
	w.step(qaAblGrant("ally", qaAblP2, qaAblOther), l)
	// VTT-481
	qaAblSame(t, "the companion made a creature out of sight", l.held("ally"), lastSeen)

	w.step(qaAblDoor(true), l)

	qaAblSame(t, "the creature it became, at sight", l.held("ally"), qaAblCreatureForm(w.st.Actors["ally"]))
}

// VTT-699
func TestQAAblACompanionAddedAgainWithAbilityIdsIsHeldWithThemAtSight(t *testing.T) {
	w := qaAblNewWorld(t)
	l := qaAblFreshLog(t, qaAblHeroEyes)
	qaAblHall(w, l)
	qaAblAddCompanion(w, qaAblAlly(), 7, 1, l)
	w.step(qaAblDoor(true), l)
	w.step(qaAblDoor(false), l)
	w.step(qaAblUnplace("tok-ally"), l)
	w.step(qaAblRemove("ally"), l)
	qaAblAddCompanion(w, qaAblAlly("rally", "fists"), 7, 1, l)

	w.step(qaAblDoor(true), l)

	qaAblSame(t, "the companion added again with a list, at sight", l.held("ally"), w.st.Actors["ally"])
}
