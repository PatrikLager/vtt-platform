package perceive_test

import (
	"fmt"
	"sort"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

const (
	qaSheetScene   = "qa-sheet-scene"
	qaSheetSession = "qa-sheet-session"
	qaSheetA       = "qa-aster"
	qaSheetB       = "qa-brindle"
	qaSheetC       = "qa-corvin"
	qaSheetG       = "qa-gobbo"
	qaSheetTokA    = "qa-tok-aster"
	qaSheetTokB    = "qa-tok-brindle"
	qaSheetTokC    = "qa-tok-corvin"
	qaSheetTokG    = "qa-tok-gobbo"
	qaSheetPartyK  = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaSheetNonPtyK = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

func qaSheetStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func qaSheetActor(t *testing.T, id, name string, kind vttv1.ActorKind, might int32, pool, poolMax int32) *vttv1.Actor {
	t.Helper()
	return &vttv1.Actor{
		ActorId:    id,
		Name:       name,
		ModuleId:   "mod." + id,
		Attributes: map[string]int32{"might": might, "wit": might + 1},
		Resources: map[string]*vttv1.Resource{
			"pool":  {Current: pool, Max: poolMax},
			"focus": {Current: 3, Max: 5},
		},
		ModuleData: qaSheetStruct(t, map[string]any{"lore": "secret of " + name}),
		Kind:       kind,
	}
}

func qaSheetEnv(p func(*vttv1.Envelope)) *vttv1.Envelope {
	env := &vttv1.Envelope{SessionId: qaSheetSession}
	p(env)
	return env
}

func qaSheetSessionStarted() *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "Night"}}
	})
}

func qaSheetSceneCreated() *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: qaSheetScene, Name: "Hall", GridWidth: 5, GridHeight: 3,
			Tiles: map[string]*vttv1.TileRef{
				"2,0": {Kind: "wall"}, "2,1": {Kind: "door"}, "2,2": {Kind: "wall"},
			},
		}}
	})
}

func qaSheetAdded(a *vttv1.Actor) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: proto.Clone(a).(*vttv1.Actor)}}
	})
}

func qaSheetGrant(actor, participant string, kind vttv1.ActorKind) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actor, ParticipantId: participant, Kind: kind,
		}}
	})
}

func qaSheetRevoke(actor, participant string) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorControlRevoked{ActorControlRevoked: &vttv1.ActorControlRevoked{
			ActorId: actor, ParticipantId: participant,
		}}
	})
}

func qaSheetCondOn(actor, cond string) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
			ActorId: actor, ConditionId: cond, Source: "qa-source",
		}}
	})
}

func qaSheetCondOff(actor, cond string) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ConditionRemoved{ConditionRemoved: &vttv1.ConditionRemoved{
			ActorId: actor, ConditionId: cond,
		}}
	})
}

func qaSheetPlaced(tok, actor string, x, y int32) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: tok, SceneId: qaSheetScene, ActorId: actor,
			Position: &vttv1.GridPosition{X: x, Y: y}, Name: tok,
		}}
	})
}

func qaSheetMoved(tok string, fx, fy, tx, ty int32) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
			TokenId: tok, SceneId: qaSheetScene,
			From: &vttv1.GridPosition{X: fx, Y: fy}, To: &vttv1.GridPosition{X: tx, Y: ty},
		}}
	})
}

func qaSheetTokenGone(tok string) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_TokenRemoved{TokenRemoved: &vttv1.TokenRemoved{TokenId: tok}}
	})
}

func qaSheetActorGone(actor string) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorRemoved{ActorRemoved: &vttv1.ActorRemoved{ActorId: actor}}
	})
}

func qaSheetResource(actor, res string, delta, newValue int32) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
			ActorId: actor, Resource: res, Delta: delta, NewValue: newValue,
		}}
	})
}

func qaSheetDoor(open bool) *vttv1.Envelope {
	at := &vttv1.GridPosition{X: 2, Y: 1}
	return qaSheetEnv(func(e *vttv1.Envelope) {
		if open {
			e.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{SceneId: qaSheetScene, At: at}}
			return
		}
		e.Payload = &vttv1.Envelope_DoorClosed{DoorClosed: &vttv1.DoorClosed{SceneId: qaSheetScene, At: at}}
	})
}

func qaSheetNarration(text string) *vttv1.Envelope {
	return qaSheetEnv(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}
	})
}

type qaSheetRun struct {
	events []*vttv1.Envelope
	worlds []*engine.State
	folds  []*engine.State
	frames [][]*vttv1.Envelope
	start  int
}

func qaSheetWalk(t *testing.T, eyes perceive.Eyes, events []*vttv1.Envelope, start int) *qaSheetRun {
	t.Helper()
	run := &qaSheetRun{events: events, start: start}
	world := engine.NewState()
	shown := engine.NewState()
	var pr *perceive.Projection
	for i, env := range events {
		env.Sequence = int64(i + 1)
		env.EventId = fmt.Sprintf("qa-ev-%d", i+1)
		if err := engine.Apply(world, env); err != nil {
			t.Fatalf("world refuses event %d (%T): %v", i+1, env.Payload, err)
		}
		run.worlds = append(run.worlds, world.Snapshot())
		if i < start {
			run.folds = append(run.folds, nil)
			run.frames = append(run.frames, nil)
			continue
		}
		if pr == nil {
			pr = perceive.New(eyes, nil)
		}
		frames := pr.Event(eyes, env, shown, world.Snapshot())
		qaSheetFold(t, shown, frames, fmt.Sprintf("event %d (%T)", i+1, env.Payload))
		run.folds = append(run.folds, shown.Snapshot())
		run.frames = append(run.frames, frames)
	}
	return run
}

func qaSheetFold(t *testing.T, shown *engine.State, frames []*vttv1.Envelope, at string) {
	t.Helper()
	for j, f := range frames {
		if err := engine.Apply(shown, f); err != nil {
			t.Fatalf("frame %d of %s does not fold (%T): %v", j, at, f.Payload, err)
		}
	}
}

func (r *qaSheetRun) at(seq int) int { return seq - 1 }

func qaSheetHeld(t *testing.T, st *engine.State, id, at string) *vttv1.Actor {
	t.Helper()
	a, ok := st.Actors[id]
	if !ok {
		t.Fatalf("%s: the fold holds no actor %q", at, id)
	}
	return a
}

func qaSheetCondIDs(st *engine.State, id string) []string {
	var out []string
	for _, c := range st.Conditions[id] {
		out = append(out, c.ID)
	}
	sort.Strings(out)
	return out
}

func qaSheetSorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func qaSheetSameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func qaSheetResEqual(a, b map[string]*vttv1.Resource) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		w, ok := b[k]
		if !ok || !proto.Equal(v, w) {
			return false
		}
	}
	return true
}

func qaSheetAttrEqual(a, b map[string]int32) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}
	return true
}

func qaSheetNoSheet(a *vttv1.Actor) bool {
	return len(a.GetAttributes()) == 0 && len(a.GetResources()) == 0 && len(a.GetModuleData().GetFields()) == 0
}

// qaSheetAssertStripped holds VTT-658 and VTT-660.
func qaSheetAssertStripped(t *testing.T, at string, shown, world *engine.State, id string, controllers []string) {
	t.Helper()
	held := qaSheetHeld(t, shown, id, at)
	src := world.Actors[id]
	if held.GetName() != src.GetName() || held.GetModuleId() != src.GetModuleId() || held.GetKind() != qaSheetPartyK {
		t.Errorf("%s: held %q as name %q module %q kind %v, want name %q module %q kind party member",
			at, id, held.GetName(), held.GetModuleId(), held.GetKind(), src.GetName(), src.GetModuleId())
	}
	if !qaSheetNoSheet(held) {
		t.Errorf("%s: held %q with a sheet: attributes %v resources %v module data %v",
			at, id, held.GetAttributes(), held.GetResources(), held.GetModuleData())
	}
	if got := qaSheetCondIDs(shown, id); len(got) != 0 {
		t.Errorf("%s: held %q with conditions %v, want none", at, id, got)
	}
	if got := held.GetControllerIds(); !qaSheetSameStrings(got, controllers) {
		t.Errorf("%s: held %q with controllers %v, want %v", at, id, got, controllers)
	}
}

// qaSheetAssertWhole holds VTT-664 and VTT-469.
func qaSheetAssertWhole(t *testing.T, at string, shown, world *engine.State, id string) {
	t.Helper()
	held := qaSheetHeld(t, shown, id, at)
	src := world.Actors[id]
	if held.GetName() != src.GetName() || held.GetModuleId() != src.GetModuleId() || held.GetKind() != src.GetKind() {
		t.Errorf("%s: held %q as name %q module %q kind %v, want %q %q %v", at, id,
			held.GetName(), held.GetModuleId(), held.GetKind(), src.GetName(), src.GetModuleId(), src.GetKind())
	}
	if !qaSheetAttrEqual(held.GetAttributes(), src.GetAttributes()) {
		t.Errorf("%s: %q attributes %v, want %v", at, id, held.GetAttributes(), src.GetAttributes())
	}
	if !qaSheetResEqual(held.GetResources(), src.GetResources()) {
		t.Errorf("%s: %q resources %v, want %v", at, id, held.GetResources(), src.GetResources())
	}
	if !proto.Equal(held.GetModuleData(), src.GetModuleData()) {
		t.Errorf("%s: %q module data %v, want %v", at, id, held.GetModuleData(), src.GetModuleData())
	}
	if got, want := qaSheetCondIDs(shown, id), qaSheetCondIDs(world, id); !qaSheetSameStrings(got, want) {
		t.Errorf("%s: %q conditions %v, want %v", at, id, got, want)
	}
	if got, want := qaSheetSorted(held.GetControllerIds()), qaSheetSorted(src.GetControllerIds()); !qaSheetSameStrings(got, want) {
		t.Errorf("%s: %q controllers %v, want %v", at, id, got, want)
	}
}

// qaSheetCarriers holds VTT-660 and VTT-486.
func qaSheetCarriers(frames []*vttv1.Envelope, id string) []string {
	var out []string
	for i, f := range frames {
		switch p := f.Payload.(type) {
		case *vttv1.Envelope_ActorAdded:
			if p.ActorAdded.GetActor().GetActorId() == id && !qaSheetNoSheet(p.ActorAdded.GetActor()) {
				out = append(out, fmt.Sprintf("frame %d ActorAdded with a sheet %v", i, p.ActorAdded.GetActor()))
			}
		case *vttv1.Envelope_ResourceChanged:
			if p.ResourceChanged.GetActorId() == id {
				out = append(out, fmt.Sprintf("frame %d ResourceChanged %v", i, p.ResourceChanged))
			}
		case *vttv1.Envelope_ConditionApplied:
			if p.ConditionApplied.GetActorId() == id {
				out = append(out, fmt.Sprintf("frame %d ConditionApplied %v", i, p.ConditionApplied))
			}
		case *vttv1.Envelope_ConditionRemoved:
			if p.ConditionRemoved.GetActorId() == id {
				out = append(out, fmt.Sprintf("frame %d ConditionRemoved %v", i, p.ConditionRemoved))
			}
		}
	}
	return out
}

func qaSheetRemovals(frames []*vttv1.Envelope, id string) int {
	n := 0
	for _, f := range frames {
		if f.GetActorRemoved().GetActorId() == id {
			n++
		}
	}
	return n
}

func qaSheetIntroOf(frames []*vttv1.Envelope, id string) (*vttv1.Actor, []*vttv1.Envelope) {
	for i, f := range frames {
		if a := f.GetActorAdded().GetActor(); a.GetActorId() == id {
			return a, frames[i+1:]
		}
	}
	return nil, nil
}

func qaSheetMainLog(t *testing.T) []*vttv1.Envelope {
	t.Helper()
	return []*vttv1.Envelope{
		qaSheetSessionStarted(),
		qaSheetSceneCreated(),
		qaSheetAdded(qaSheetActor(t, qaSheetA, "Aster", qaSheetPartyK, 14, 10, 12)),
		qaSheetGrant(qaSheetA, "p-a", qaSheetPartyK),
		qaSheetCondOn(qaSheetA, "blessed"),
		qaSheetPlaced(qaSheetTokA, qaSheetA, 4, 1),
		qaSheetAdded(qaSheetActor(t, qaSheetB, "Brindle", qaSheetPartyK, 9, 7, 7)),
		qaSheetGrant(qaSheetB, "p-b", qaSheetPartyK),
		qaSheetPlaced(qaSheetTokB, qaSheetB, 0, 1),
		qaSheetResource(qaSheetA, "pool", -2, 8),
		qaSheetCondOn(qaSheetA, "hasted"),
		qaSheetGrant(qaSheetA, "p-c", qaSheetPartyK),
		qaSheetCondOff(qaSheetA, "blessed"),
		qaSheetResource(qaSheetA, "focus", 1, 4),
		qaSheetDoor(true),
		qaSheetResource(qaSheetA, "pool", -1, 7),
		qaSheetDoor(false),
		qaSheetResource(qaSheetA, "pool", -4, 3),
		qaSheetCondOn(qaSheetA, "slowed"),
		qaSheetRevoke(qaSheetA, "p-c"),
		qaSheetMoved(qaSheetTokA, 4, 1, 1, 1),
		qaSheetAdded(qaSheetActor(t, qaSheetG, "Gobbo", qaSheetNonPtyK, 6, 4, 4)),
		qaSheetGrant(qaSheetG, "p-dm", qaSheetNonPtyK),
		qaSheetCondOn(qaSheetG, "angry"),
		qaSheetPlaced(qaSheetTokG, qaSheetG, 1, 0),
		qaSheetNarration("the goblin snarls"),
		qaSheetGrant(qaSheetG, "p-dm", qaSheetPartyK),
	}
}

const qaSheetMainStart = 6

var qaSheetEyesB = perceive.Eyes{Shoulder: qaSheetB}

// VTT-658
func TestQASheetAnUnseenCompanionIsIntroducedWithoutItsSheet(t *testing.T) {
	run := qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart)
	i := run.at(7)
	frames := run.frames[i]
	// VTT-658
	intro, after := qaSheetIntroOf(frames, qaSheetA)
	if intro == nil {
		t.Fatalf("event 7: B's log is not introduced to party member A; frames %v", frames)
	}
	if intro.GetName() != "Aster" || intro.GetModuleId() != "mod."+qaSheetA || intro.GetKind() != qaSheetPartyK {
		t.Errorf("VTT-658: A introduced as name %q module %q kind %v", intro.GetName(), intro.GetModuleId(), intro.GetKind())
	}
	if !qaSheetNoSheet(intro) {
		t.Errorf("VTT-658: A introduced with a sheet: %v", intro)
	}
	var grants []string
	for _, f := range after {
		if g := f.GetActorControlGranted(); g.GetActorId() == qaSheetA {
			grants = append(grants, g.GetParticipantId())
			if g.GetKind() != qaSheetPartyK {
				t.Errorf("VTT-658: A's grant behind its introduction carries kind %v", g.GetKind())
			}
		}
		if c := f.GetConditionApplied(); c.GetActorId() == qaSheetA {
			t.Errorf("VTT-658: A introduced with condition %q", c.GetConditionId())
		}
	}
	if !qaSheetSameStrings(grants, []string{"p-a"}) {
		t.Errorf("VTT-658: A's controllers behind its introduction %v, want [p-a]", grants)
	}
	qaSheetAssertStripped(t, "VTT-658 after event 7", run.folds[i], run.worlds[i], qaSheetA, []string{"p-a"})
}

// VTT-657
func TestQASheetTheViewersOwnCharacterIsIntroducedWhole(t *testing.T) {
	run := qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart)
	i := run.at(7)
	// VTT-657
	intro, _ := qaSheetIntroOf(run.frames[i], qaSheetB)
	if intro == nil {
		t.Fatalf("event 7: B's log is not introduced to B; frames %v", run.frames[i])
	}
	src := run.worlds[i].Actors[qaSheetB]
	if intro.GetName() != src.GetName() || !qaSheetAttrEqual(intro.GetAttributes(), src.GetAttributes()) ||
		!qaSheetResEqual(intro.GetResources(), src.GetResources()) || !proto.Equal(intro.GetModuleData(), src.GetModuleData()) {
		t.Errorf("VTT-657: B introduced as %v, want its whole sheet %v", intro, src)
	}
}

// VTT-660 VTT-486 VTT-661
func TestQASheetChangesToAnUnseenCompanionSendNothing(t *testing.T) {
	run := qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart)
	for seq := 8; seq <= 14; seq++ {
		i := run.at(seq)
		// VTT-660, VTT-486
		if got := qaSheetCarriers(run.frames[i], qaSheetA); len(got) != 0 {
			t.Errorf("VTT-660, VTT-486: event %d: B's log, which never saw A, is sent A's values: %v", seq, got)
		}
		// VTT-661
		if n := qaSheetRemovals(run.frames[i], qaSheetA); n != 0 {
			t.Errorf("VTT-661: event %d sends %d bare ActorRemoved of A before B sees it", seq, n)
		}
		// VTT-660
		qaSheetAssertStripped(t, fmt.Sprintf("VTT-660 after event %d", seq), run.folds[i], run.worlds[i], qaSheetA, []string{"p-a"})
	}
}

// VTT-664 VTT-661 VTT-486
func TestQASheetTheSheetArrivesWithTheFirstSight(t *testing.T) {
	run := qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart)
	i := run.at(15)
	if _, ok := run.folds[i].Tokens[qaSheetTokA]; !ok {
		t.Fatalf("fixture: A's token is not on B's board after the door opens")
	}
	// VTT-664
	qaSheetAssertWhole(t, "VTT-664 after event 15", run.folds[i], run.worlds[i], qaSheetA)
	held := run.folds[i].Actors[qaSheetA]
	if got := held.GetResources()["pool"].GetCurrent(); got != 8 {
		t.Errorf("VTT-664: A's pool after first sight %d, want 8, its value changed while unseen", got)
	}
	if got := qaSheetCondIDs(run.folds[i], qaSheetA); !qaSheetSameStrings(got, []string{"hasted"}) {
		t.Errorf("VTT-664: A's conditions after first sight %v, want [hasted]", got)
	}
	// VTT-661
	if n := qaSheetRemovals(run.frames[i], qaSheetA); n > 1 {
		t.Errorf("VTT-661: first sight sends %d bare ActorRemoved of A", n)
	}
	// VTT-486
	j := run.at(16)
	var sent *vttv1.ResourceChanged
	for _, f := range run.frames[j] {
		if rc := f.GetResourceChanged(); rc.GetActorId() == qaSheetA {
			sent = rc
		}
	}
	if sent == nil || sent.GetNewValue() != 7 || sent.GetResource() != "pool" {
		t.Errorf("VTT-486: event 16 sends A's ResourceChanged %v, want pool to 7", sent)
	}
	qaSheetAssertWhole(t, "VTT-486 after event 16", run.folds[j], run.worlds[j], qaSheetA)
}

// VTT-660 VTT-469
func TestQASheetASeenCompanionOutOfSightIsHeldAsLastSeen(t *testing.T) {
	run := qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart)
	lastSeen := run.worlds[run.at(16)]
	for seq := 17; seq <= 20; seq++ {
		i := run.at(seq)
		if _, ok := run.folds[i].Tokens[qaSheetTokA]; ok {
			t.Fatalf("fixture: A's token is still on B's board at event %d", seq)
		}
		// VTT-660, VTT-469
		qaSheetAssertWhole(t, fmt.Sprintf("VTT-660 after event %d", seq), run.folds[i], lastSeen, qaSheetA)
		if got := qaSheetCarriers(run.frames[i], qaSheetA); len(got) != 0 {
			t.Errorf("VTT-660: event %d sends A's values while out of sight: %v", seq, got)
		}
	}
	// VTT-469
	i := run.at(21)
	if _, ok := run.folds[i].Tokens[qaSheetTokA]; !ok {
		t.Fatalf("fixture: A's token is not on B's board after it moves into sight")
	}
	qaSheetAssertWhole(t, "VTT-469 after event 21", run.folds[i], run.worlds[i], qaSheetA)
	if got := run.folds[i].Actors[qaSheetA].GetResources()["pool"].GetCurrent(); got != 3 {
		t.Errorf("VTT-469: A's pool back in sight %d, want 3", got)
	}
}

// VTT-465 VTT-657
func TestQASheetACreatureArrivesWithoutNameOrValues(t *testing.T) {
	run := qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart)
	if _, ok := run.folds[run.at(24)].Actors[qaSheetG]; ok {
		t.Errorf("VTT-465: B's log holds creature G before it sees it")
	}
	i := run.at(25)
	// VTT-465
	intro, after := qaSheetIntroOf(run.frames[i], qaSheetG)
	if intro == nil {
		t.Fatalf("VTT-465: no introduction of G when its token comes into sight; frames %v", run.frames[i])
	}
	if intro.GetName() != "" || !qaSheetNoSheet(intro) || intro.GetModuleId() != "mod."+qaSheetG || intro.GetKind() != qaSheetNonPtyK {
		t.Errorf("VTT-465: G introduced as %v", intro)
	}
	var grants, conds []string
	for _, f := range after {
		if g := f.GetActorControlGranted(); g.GetActorId() == qaSheetG {
			grants = append(grants, g.GetParticipantId())
		}
		if c := f.GetConditionApplied(); c.GetActorId() == qaSheetG {
			conds = append(conds, c.GetConditionId())
		}
	}
	if !qaSheetSameStrings(grants, []string{"p-dm"}) || !qaSheetSameStrings(conds, []string{"angry"}) {
		t.Errorf("VTT-465: G's controllers %v and conditions %v behind it, want [p-dm] and [angry]", grants, conds)
	}
	// VTT-657
	k := run.at(27)
	qaSheetAssertWhole(t, "VTT-657 after G becomes a party member in sight", run.folds[k], run.worlds[k], qaSheetG)
}

// VTT-312
func TestQASheetARestartedDecisionSendsWhatTheFirstSends(t *testing.T) {
	qaSheetAssertRestarts(t, qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart))
}

// qaSheetAssertRestarts holds VTT-312.
func qaSheetAssertRestarts(t *testing.T, run *qaSheetRun) {
	t.Helper()
	eyes := qaSheetEyesB
	for k := run.start; k < len(run.events)-1; k++ {
		for _, how := range []string{"New", "Resume"} {
			var pr *perceive.Projection
			if how == "New" {
				pr = perceive.New(eyes, run.worlds[k].Snapshot())
			} else {
				pr = perceive.Resume(eyes, run.folds[k].Snapshot(), run.worlds[k].Snapshot())
			}
			shown := run.folds[k].Snapshot()
			for i := k + 1; i < len(run.events); i++ {
				got := pr.Event(eyes, run.events[i], shown, run.worlds[i].Snapshot())
				// VTT-312
				if !qaSheetSameFrames(got, run.frames[i]) {
					t.Errorf("VTT-312: %s at sequence %d sends at event %d\n%v\nwant\n%v", how, k+1, i+1, got, run.frames[i])
					break
				}
				qaSheetFold(t, shown, got, fmt.Sprintf("%s restart event %d", how, i+1))
			}
		}
	}
}

func qaSheetSameFrames(a, b []*vttv1.Envelope) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !proto.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// VTT-657
func TestQASheetAViewerStartedInSightOfACompanionIsIntroducedToItWhole(t *testing.T) {
	events := qaSheetMainLog(t)[:16]
	// VTT-657
	run := qaSheetWalk(t, qaSheetEyesB, events, 14)
	i := run.at(15)
	intro, _ := qaSheetIntroOf(run.frames[i], qaSheetA)
	if intro == nil {
		t.Fatalf("event 15: a viewer sent nothing yet is not introduced to A; frames %v", run.frames[i])
	}
	src := run.worlds[i].Actors[qaSheetA]
	if !qaSheetAttrEqual(intro.GetAttributes(), src.GetAttributes()) || !qaSheetResEqual(intro.GetResources(), src.GetResources()) ||
		!proto.Equal(intro.GetModuleData(), src.GetModuleData()) || intro.GetName() != src.GetName() {
		t.Errorf("VTT-657: A, seen, introduced as %v, want %v", intro, src)
	}
	qaSheetAssertWhole(t, "VTT-657 after event 15", run.folds[i], run.worlds[i], qaSheetA)
}

func qaSheetPerchLog(t *testing.T) []*vttv1.Envelope {
	t.Helper()
	return []*vttv1.Envelope{
		qaSheetSessionStarted(),
		qaSheetSceneCreated(),
		qaSheetAdded(qaSheetActor(t, qaSheetA, "Aster", qaSheetPartyK, 14, 10, 12)),
		qaSheetGrant(qaSheetA, "p-a", qaSheetPartyK),
		qaSheetAdded(qaSheetActor(t, qaSheetC, "Corvin", qaSheetPartyK, 11, 9, 9)),
		qaSheetGrant(qaSheetC, "p-c", qaSheetPartyK),
		qaSheetAdded(qaSheetActor(t, qaSheetB, "Brindle", qaSheetPartyK, 9, 7, 7)),
		qaSheetGrant(qaSheetB, "p-b", qaSheetPartyK),
		qaSheetPlaced(qaSheetTokA, qaSheetA, 4, 1),
		qaSheetPlaced(qaSheetTokC, qaSheetC, 3, 1),
		qaSheetPlaced(qaSheetTokB, qaSheetB, 0, 1),
		qaSheetResource(qaSheetA, "pool", -5, 5),
		qaSheetCondOn(qaSheetA, "hasted"),
		qaSheetGrant(qaSheetA, "p-x", qaSheetPartyK),
		qaSheetResource(qaSheetC, "pool", -1, 8),
	}
}

// VTT-469
func TestQASheetAPerchOntoEyesThatSeeACompanionBringsItToItsPresent(t *testing.T) {
	events := qaSheetPerchLog(t)
	run := qaSheetWalk(t, qaSheetEyesB, events, 6)
	last := len(events) - 1
	shown := run.folds[last].Snapshot()
	world := run.worlds[last]
	qaSheetAssertStripped(t, "before the perch", shown, world, qaSheetA, []string{"p-a"})
	eyesC := perceive.Eyes{Shoulder: qaSheetC}
	pr := perceive.New(qaSheetEyesB, world.Snapshot())
	frames := pr.Perch(eyesC, shown, world.Snapshot(), int64(len(events)))
	qaSheetFold(t, shown, frames, "the perch onto C")
	// VTT-469
	for _, id := range []string{qaSheetA, qaSheetC} {
		held := qaSheetHeld(t, shown, id, "after the perch")
		src := world.Actors[id]
		if !qaSheetResEqual(held.GetResources(), src.GetResources()) {
			t.Errorf("VTT-469: %s resources after the perch %v, want %v", id, held.GetResources(), src.GetResources())
		}
		if got, want := qaSheetCondIDs(shown, id), qaSheetCondIDs(world, id); !qaSheetSameStrings(got, want) {
			t.Errorf("VTT-469: %s conditions after the perch %v, want %v", id, got, want)
		}
		if got, want := qaSheetSorted(held.GetControllerIds()), qaSheetSorted(src.GetControllerIds()); !qaSheetSameStrings(got, want) {
			t.Errorf("VTT-469: %s controllers after the perch %v, want %v", id, got, want)
		}
		if held.GetKind() != src.GetKind() {
			t.Errorf("VTT-469: %s kind after the perch %v, want %v", id, held.GetKind(), src.GetKind())
		}
	}
}

// VTT-661 VTT-660 VTT-664 VTT-312
func TestQASheetAReAddUnderTheSameNameSendsNoRemoval(t *testing.T) {
	base := qaSheetMainLog(t)[:14]
	reborn := qaSheetActor(t, qaSheetA, "Aster", qaSheetPartyK, 20, 30, 30)
	events := append(base,
		qaSheetTokenGone(qaSheetTokA),
		qaSheetActorGone(qaSheetA),
		qaSheetAdded(reborn),
		qaSheetGrant(qaSheetA, "p-a", qaSheetPartyK),
		qaSheetPlaced(qaSheetTokA, qaSheetA, 4, 1),
		qaSheetDoor(true),
	)
	run := qaSheetWalk(t, qaSheetEyesB, events, qaSheetMainStart)
	qaSheetAssertRestarts(t, run)
	for seq := 15; seq <= 19; seq++ {
		i := run.at(seq)
		// VTT-661
		if n := qaSheetRemovals(run.frames[i], qaSheetA); n != 0 {
			t.Errorf("VTT-661: event %d sends %d bare ActorRemoved of A, re-added under the same name and module id", seq, n)
		}
		if got := qaSheetCarriers(run.frames[i], qaSheetA); len(got) != 0 {
			t.Errorf("VTT-660: event %d sends A's values unseen: %v", seq, got)
		}
	}
	// VTT-664
	i := run.at(20)
	qaSheetAssertWhole(t, "VTT-664 after the re-added A is first seen", run.folds[i], run.worlds[i], qaSheetA)
	if got := run.folds[i].Actors[qaSheetA].GetAttributes()["might"]; got != 20 {
		t.Errorf("VTT-664: re-added A's might %d, want 20", got)
	}
}

// VTT-661 VTT-658 VTT-664 VTT-312
func TestQASheetAReAddUnderAnotherNameIsIntroducedAfreshWithoutItsSheet(t *testing.T) {
	base := qaSheetMainLog(t)[:14]
	reborn := qaSheetActor(t, qaSheetA, "Aster Reborn", qaSheetPartyK, 20, 30, 30)
	events := append(base,
		qaSheetTokenGone(qaSheetTokA),
		qaSheetActorGone(qaSheetA),
		qaSheetAdded(reborn),
		qaSheetPlaced(qaSheetTokA, qaSheetA, 4, 1),
		qaSheetDoor(true),
	)
	run := qaSheetWalk(t, qaSheetEyesB, events, qaSheetMainStart)
	qaSheetAssertRestarts(t, run)
	i := run.at(17)
	// VTT-661
	if n := qaSheetRemovals(run.frames[i], qaSheetA); n != 1 {
		t.Errorf("VTT-661: the re-add under another name sends %d bare ActorRemoved of A, want 1", n)
	}
	// VTT-658
	if got := qaSheetCarriers(run.frames[i], qaSheetA); len(got) != 0 {
		t.Errorf("VTT-658: the re-add sends A's values unseen: %v", got)
	}
	qaSheetAssertStripped(t, "VTT-658 after the re-add under another name", run.folds[i], run.worlds[i], qaSheetA, nil)
	// VTT-664
	k := run.at(19)
	qaSheetAssertWhole(t, "VTT-664 after the renamed A is first seen", run.folds[k], run.worlds[k], qaSheetA)
}

// VTT-660 VTT-481 VTT-465 VTT-469 VTT-312
func TestQASheetAnUnseenCompanionMadeACreatureArrivesAsOne(t *testing.T) {
	base := qaSheetMainLog(t)[:14]
	events := append(base,
		qaSheetGrant(qaSheetA, "p-dm", qaSheetNonPtyK),
		qaSheetDoor(true),
	)
	run := qaSheetWalk(t, qaSheetEyesB, events, qaSheetMainStart)
	qaSheetAssertRestarts(t, run)
	i := run.at(15)
	// VTT-660
	if got := qaSheetCarriers(run.frames[i], qaSheetA); len(got) != 0 {
		t.Errorf("VTT-660: event 15 sends A's values unseen: %v", got)
	}
	qaSheetAssertStripped(t, "VTT-660 after A turns creature unseen", run.folds[i], run.worlds[i], qaSheetA, []string{"p-a"})
	// VTT-481, VTT-465
	k := run.at(16)
	held := qaSheetHeld(t, run.folds[k], qaSheetA, "after the door opens")
	if held.GetKind() != qaSheetNonPtyK || held.GetName() != "" || !qaSheetNoSheet(held) {
		t.Errorf("VTT-481: A, a creature now in sight, held as %v", held)
	}
	if got, want := qaSheetCondIDs(run.folds[k], qaSheetA), qaSheetCondIDs(run.worlds[k], qaSheetA); !qaSheetSameStrings(got, want) {
		t.Errorf("VTT-465: A's conditions in sight %v, want %v", got, want)
	}
	if got, want := qaSheetSorted(held.GetControllerIds()), qaSheetSorted(run.worlds[k].Actors[qaSheetA].GetControllerIds()); !qaSheetSameStrings(got, want) {
		t.Errorf("VTT-469: A's controllers in sight %v, want %v", got, want)
	}
}

// VTT-658 VTT-664 VTT-312
func TestQASheetAnUnseenCreatureMadeAPartyMemberIsIntroducedWithoutItsSheet(t *testing.T) {
	base := qaSheetMainLog(t)[:14]
	events := append(base,
		qaSheetAdded(qaSheetActor(t, qaSheetG, "Gobbo", qaSheetNonPtyK, 6, 4, 4)),
		qaSheetGrant(qaSheetG, "p-dm", qaSheetNonPtyK),
		qaSheetCondOn(qaSheetG, "angry"),
		qaSheetPlaced(qaSheetTokG, qaSheetG, 3, 1),
		qaSheetGrant(qaSheetG, "p-g", qaSheetPartyK),
		qaSheetDoor(true),
	)
	run := qaSheetWalk(t, qaSheetEyesB, events, qaSheetMainStart)
	qaSheetAssertRestarts(t, run)
	if _, ok := run.folds[run.at(18)].Actors[qaSheetG]; ok {
		t.Fatalf("fixture: B's log holds creature G before it is a party member or seen")
	}
	i := run.at(19)
	// VTT-658
	if got := qaSheetCarriers(run.frames[i], qaSheetG); len(got) != 0 {
		t.Errorf("VTT-658: G, made a party member unseen, is sent its values: %v", got)
	}
	qaSheetAssertStripped(t, "VTT-658 after G is made a party member unseen", run.folds[i], run.worlds[i], qaSheetG, []string{"p-dm", "p-g"})
	// VTT-664
	k := run.at(20)
	qaSheetAssertWhole(t, "VTT-664 after G is first seen", run.folds[k], run.worlds[k], qaSheetG)
}

// VTT-658 VTT-465
func TestQASheetAPerchThroughNoEyesIntroducesEachPartyMemberWithoutItsSheet(t *testing.T) {
	run := qaSheetWalk(t, qaSheetEyesB, qaSheetMainLog(t), qaSheetMainStart)
	world := run.worlds[run.at(25)]
	shown := engine.NewState()
	frames := perceive.New(perceive.Eyes{}, nil).Perch(perceive.Eyes{}, shown, world.Snapshot(), 0)
	qaSheetFold(t, shown, frames, "a perch through no eyes")
	// VTT-658
	for _, id := range []string{qaSheetA, qaSheetB} {
		if got := qaSheetCarriers(frames, id); len(got) != 0 {
			t.Errorf("VTT-658: a perch through no eyes sends %s's values: %v", id, got)
		}
		qaSheetAssertStripped(t, "VTT-658 after a perch through no eyes", shown, world, id, world.Actors[id].GetControllerIds())
	}
	// VTT-465
	if _, ok := shown.Actors[qaSheetG]; ok {
		t.Errorf("VTT-465: a perch through no eyes introduces creature G")
	}
}

func TestQASheetAFirstSightWhoseFramesWereDroppedBringsTheSheetNext(t *testing.T) {
	events := qaSheetMainLog(t)
	run := qaSheetWalk(t, qaSheetEyesB, events, qaSheetMainStart)
	lagging := run.folds[run.at(14)].Snapshot()
	pr := perceive.Resume(qaSheetEyesB, lagging.Snapshot(), run.worlds[run.at(15)].Snapshot())
	i := run.at(16)
	frames := pr.Event(qaSheetEyesB, events[i], lagging, run.worlds[i].Snapshot())
	qaSheetFold(t, lagging, frames, "event 16 after a dropped first sight")
	qaSheetAssertWhole(t, "after a dropped first sight and event 16", lagging, run.worlds[i], qaSheetA)
}

// VTT-658 VTT-664 VTT-661 VTT-486 VTT-312
func TestQASheetTheSheetArrivesWithTheFirstSightHoweverItComes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token bool
		sight *vttv1.Envelope
	}{
		{"the door opens", true, qaSheetDoor(true)},
		{"the companion steps into view", true, qaSheetMoved(qaSheetTokA, 4, 1, 1, 1)},
		{"the viewer steps into view", true, qaSheetMoved(qaSheetTokB, 0, 1, 3, 1)},
		{"the companion is placed in view", false, qaSheetPlaced(qaSheetTokA, qaSheetA, 1, 2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := qaSheetMainLog(t)[:14]
			if !tc.token {
				events[5] = qaSheetNarration("no token for Aster yet")
			}
			events = append(events, tc.sight, qaSheetResource(qaSheetA, "pool", -1, 7))
			run := qaSheetWalk(t, qaSheetEyesB, events, qaSheetMainStart)
			qaSheetAssertRestarts(t, run)
			before := run.at(14)
			qaSheetAssertStripped(t, "VTT-658 before the first sight", run.folds[before], run.worlds[before], qaSheetA, []string{"p-a"})
			i := run.at(15)
			if _, ok := run.folds[i].Tokens[qaSheetTokA]; !ok {
				t.Fatalf("fixture: A's token is not on B's board at the first sight")
			}
			// VTT-664
			qaSheetAssertWhole(t, "VTT-664 at the first sight", run.folds[i], run.worlds[i], qaSheetA)
			// VTT-661
			if n := qaSheetRemovals(run.frames[i], qaSheetA); n > 1 {
				t.Errorf("VTT-661: the first sight sends %d bare ActorRemoved of A", n)
			}
			// VTT-486
			j := run.at(16)
			qaSheetAssertWhole(t, "VTT-486 after the first change in sight", run.folds[j], run.worlds[j], qaSheetA)
		})
	}
}

// VTT-660 VTT-470 VTT-312
func TestQASheetASeenCompanionReAddedUnseenIsIntroducedAfreshOnSight(t *testing.T) {
	base := qaSheetMainLog(t)[:17]
	reborn := qaSheetActor(t, qaSheetA, "Aster", qaSheetPartyK, 20, 30, 30)
	events := append(base,
		qaSheetTokenGone(qaSheetTokA),
		qaSheetActorGone(qaSheetA),
		qaSheetAdded(reborn),
		qaSheetPlaced(qaSheetTokA, qaSheetA, 4, 1),
		qaSheetDoor(true),
	)
	run := qaSheetWalk(t, qaSheetEyesB, events, qaSheetMainStart)
	qaSheetAssertRestarts(t, run)
	for seq := 18; seq <= 21; seq++ {
		if got := qaSheetCarriers(run.frames[run.at(seq)], qaSheetA); len(got) != 0 {
			t.Errorf("VTT-660: event %d sends the re-added A's values unseen: %v", seq, got)
		}
	}
	// VTT-470
	i := run.at(22)
	qaSheetAssertWhole(t, "VTT-470 after the re-added A comes into sight", run.folds[i], run.worlds[i], qaSheetA)
}
