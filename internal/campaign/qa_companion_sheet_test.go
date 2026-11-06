package campaign_test

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

const (
	qaSheetScene   = "qa-sheet-scene"
	qaSheetSession = "qa-sheet-session"
	qaSheetA       = "qa-aster"
	qaSheetB       = "qa-brindle"
	qaSheetTokA    = "qa-tok-aster"
	qaSheetTokB    = "qa-tok-brindle"
	qaSheetPartyK  = vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER
	qaSheetNonPtyK = vttv1.ActorKind_ACTOR_KIND_NON_PARTY
)

func qaSheetActor(t *testing.T, id, name string, kind vttv1.ActorKind, might, pool, poolMax int32) *vttv1.Actor {
	t.Helper()
	md, err := structpb.NewStruct(map[string]any{"lore": "secret of " + name})
	if err != nil {
		t.Fatal(err)
	}
	return &vttv1.Actor{
		ActorId: id, Name: name, ModuleId: "mod." + id,
		Attributes: map[string]int32{"might": might, "wit": might + 1},
		Resources: map[string]*vttv1.Resource{
			"pool":  {Current: pool, Max: poolMax},
			"focus": {Current: 3, Max: 5},
		},
		ModuleData: md,
		Kind:       kind,
	}
}

func qaSheetEnv(n int, set func(*vttv1.Envelope)) *vttv1.Envelope {
	env := &vttv1.Envelope{EventId: fmt.Sprintf("qa-sheet-ev-%d", n), SessionId: qaSheetSession}
	set(env)
	return env
}

type qaSheetLog struct{ envs []*vttv1.Envelope }

func (l *qaSheetLog) add(set func(*vttv1.Envelope)) {
	l.envs = append(l.envs, qaSheetEnv(len(l.envs)+1, set))
}

func (l *qaSheetLog) session() {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "Night one"}}
	})
}

func (l *qaSheetLog) scene() {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
			SceneId: qaSheetScene, Name: "Hall", GridWidth: 5, GridHeight: 3,
			Tiles: map[string]*vttv1.TileRef{"2,0": {Kind: "wall"}, "2,1": {Kind: "door"}, "2,2": {Kind: "wall"}},
		}}
	})
}

func (l *qaSheetLog) note(key string, vis vttv1.NoteVisibility) {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_NoteUpserted{NoteUpserted: &vttv1.NoteUpserted{
			Key: key, Title: key, Text: "text of " + key, Visibility: vis,
		}}
	})
}

func (l *qaSheetLog) actor(a *vttv1.Actor) {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{Actor: a}}
	})
}

func (l *qaSheetLog) grant(actor, participant string, kind vttv1.ActorKind) {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ActorControlGranted{ActorControlGranted: &vttv1.ActorControlGranted{
			ActorId: actor, ParticipantId: participant, Kind: kind,
		}}
	})
}

func (l *qaSheetLog) condition(actor, cond string) {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ConditionApplied{ConditionApplied: &vttv1.ConditionApplied{
			ActorId: actor, ConditionId: cond, Source: "qa-source",
		}}
	})
}

func (l *qaSheetLog) token(tok, actor string, x, y int32) {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
			TokenId: tok, SceneId: qaSheetScene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y},
		}}
	})
}

func (l *qaSheetLog) resource(actor, res string, delta, newValue int32) {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_ResourceChanged{ResourceChanged: &vttv1.ResourceChanged{
			ActorId: actor, Resource: res, Delta: delta, NewValue: newValue, Reason: "qa-reason",
		}}
	})
}

func (l *qaSheetLog) door() {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_DoorOpened{DoorOpened: &vttv1.DoorOpened{
			SceneId: qaSheetScene, At: &vttv1.GridPosition{X: 2, Y: 1},
		}}
	})
}

func (l *qaSheetLog) narration(text string) {
	l.add(func(e *vttv1.Envelope) {
		e.Payload = &vttv1.Envelope_NarrationAdded{NarrationAdded: &vttv1.NarrationAdded{Text: text}}
	})
}

func qaSheetAppend(t *testing.T, envs []*vttv1.Envelope) (*campaign.Campaign, []int64) {
	t.Helper()
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	seqs := make([]int64, 0, len(envs))
	for i, env := range envs {
		seq, err := c.Append(proto.Clone(env).(*vttv1.Envelope))
		if err != nil {
			t.Fatalf("append %d (%T): %v", i+1, env.Payload, err)
		}
		seqs = append(seqs, seq)
	}
	return c, seqs
}

func qaSheetFoldEntries(t *testing.T, entries []campaign.CharacterEntry) *engine.State {
	t.Helper()
	st := engine.NewState()
	for i, e := range entries {
		if err := engine.Apply(st, e.Env); err != nil {
			t.Fatalf("entry %d (%T) does not fold: %v", i, e.Env.Payload, err)
		}
	}
	return st
}

func qaSheetNoSheet(a *vttv1.Actor) bool {
	return len(a.GetAttributes()) == 0 && len(a.GetResources()) == 0 && len(a.GetModuleData().GetFields()) == 0
}

func qaSheetCondIDs(st *engine.State, id string) []string {
	var out []string
	for _, c := range st.Conditions[id] {
		out = append(out, c.ID)
	}
	sort.Strings(out)
	return out
}

func qaSheetCarries(env *vttv1.Envelope, id string) bool {
	switch p := env.Payload.(type) {
	case *vttv1.Envelope_ActorAdded:
		return p.ActorAdded.GetActor().GetActorId() == id && !qaSheetNoSheet(p.ActorAdded.GetActor())
	case *vttv1.Envelope_ResourceChanged:
		return p.ResourceChanged.GetActorId() == id
	case *vttv1.Envelope_ConditionApplied:
		return p.ConditionApplied.GetActorId() == id
	case *vttv1.Envelope_ConditionRemoved:
		return p.ConditionRemoved.GetActorId() == id
	}
	return false
}

func qaSheetAssertStripped(t *testing.T, at string, st *engine.State, world *vttv1.Actor, controllers []string) {
	t.Helper()
	held, ok := st.Actors[world.GetActorId()]
	if !ok {
		t.Fatalf("%s: the log holds no %q", at, world.GetActorId())
	}
	if held.GetName() != world.GetName() || held.GetModuleId() != world.GetModuleId() || held.GetKind() != qaSheetPartyK {
		t.Errorf("%s: held as name %q module %q kind %v", at, held.GetName(), held.GetModuleId(), held.GetKind())
	}
	if !qaSheetNoSheet(held) {
		t.Errorf("%s: held with a sheet %v", at, held)
	}
	if got := qaSheetCondIDs(st, world.GetActorId()); len(got) != 0 {
		t.Errorf("%s: held with conditions %v", at, got)
	}
	if got := held.GetControllerIds(); fmt.Sprint(got) != fmt.Sprint(controllers) {
		t.Errorf("%s: held with controllers %v, want %v", at, got, controllers)
	}
}

func qaSheetAssertWhole(t *testing.T, at string, st, world *engine.State, id string) {
	t.Helper()
	held, ok := st.Actors[id]
	if !ok {
		t.Fatalf("%s: the log holds no %q", at, id)
	}
	src := world.Actors[id]
	if held.GetName() != src.GetName() || held.GetKind() != src.GetKind() || held.GetModuleId() != src.GetModuleId() {
		t.Errorf("%s: held as %q %v %q, want %q %v %q", at, held.GetName(), held.GetKind(), held.GetModuleId(),
			src.GetName(), src.GetKind(), src.GetModuleId())
	}
	if fmt.Sprint(held.GetAttributes()) != fmt.Sprint(src.GetAttributes()) {
		t.Errorf("%s: attributes %v, want %v", at, held.GetAttributes(), src.GetAttributes())
	}
	if len(held.GetResources()) != len(src.GetResources()) {
		t.Errorf("%s: resources %v, want %v", at, held.GetResources(), src.GetResources())
	}
	for k, v := range src.GetResources() {
		if !proto.Equal(held.GetResources()[k], v) {
			t.Errorf("%s: resource %q %v, want %v", at, k, held.GetResources()[k], v)
		}
	}
	if !proto.Equal(held.GetModuleData(), src.GetModuleData()) {
		t.Errorf("%s: module data %v, want %v", at, held.GetModuleData(), src.GetModuleData())
	}
	if got, want := qaSheetCondIDs(st, id), qaSheetCondIDs(world, id); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s: conditions %v, want %v", at, got, want)
	}
	got, want := append([]string(nil), held.GetControllerIds()...), append([]string(nil), src.GetControllerIds()...)
	sort.Strings(got)
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s: controllers %v, want %v", at, got, want)
	}
}

// qaSheetOpening holds VTT-663.
func qaSheetOpening(entries []campaign.CharacterEntry) []campaign.CharacterEntry {
	for i, e := range entries {
		if e.WorldSeq != 0 {
			return entries[:i]
		}
	}
	return entries
}

func qaSheetThrough(entries []campaign.CharacterEntry, seq int64) []campaign.CharacterEntry {
	for i, e := range entries {
		if e.WorldSeq > seq {
			return entries[:i]
		}
	}
	return entries
}

func qaSheetWorld(t *testing.T, c *campaign.Campaign) []*vttv1.Envelope {
	t.Helper()
	ch, stop, head, err := c.Subscribe(0, 256)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	var out []*vttv1.Envelope
	for int64(len(out)) < head {
		select {
		case env := <-ch:
			out = append(out, env)
		case <-time.After(5 * time.Second):
			t.Fatalf("world subscription delivered %d of %d events", len(out), head)
		}
	}
	return out
}

func qaSheetBuildUnseen(t *testing.T) *qaSheetLog {
	t.Helper()
	l := &qaSheetLog{}
	l.session()
	l.scene()
	l.note("qa-public", vttv1.NoteVisibility_NOTE_VISIBILITY_PUBLIC)
	l.note("qa-secret", vttv1.NoteVisibility_NOTE_VISIBILITY_SECRET)
	l.actor(qaSheetActor(t, qaSheetA, "Aster", qaSheetPartyK, 14, 10, 12))
	l.grant(qaSheetA, "p-a", qaSheetPartyK)
	l.condition(qaSheetA, "blessed")
	l.token(qaSheetTokA, qaSheetA, 4, 1)
	l.narration("before Brindle")
	l.actor(qaSheetActor(t, qaSheetB, "Brindle", qaSheetPartyK, 9, 7, 7))
	l.grant(qaSheetB, "p-b", qaSheetPartyK)
	l.token(qaSheetTokB, qaSheetB, 0, 1)
	l.resource(qaSheetA, "pool", -2, 8)
	l.condition(qaSheetA, "hasted")
	l.door()
	l.resource(qaSheetA, "pool", -1, 7)
	return l
}

const (
	qaSheetUnseenBAdded   = 10
	qaSheetUnseenDoorOpen = 15
)

// VTT-663 VTT-658
func TestQASheetALogOpensWithEachCompanionWithoutItsSheet(t *testing.T) {
	l := qaSheetBuildUnseen(t)
	c, seqs := qaSheetAppend(t, l.envs)
	entries, err := c.CharacterLog(qaSheetB)
	if err != nil {
		t.Fatal(err)
	}
	world := qaSheetWorld(t, c)
	bAdded := seqs[qaSheetUnseenBAdded-1]
	opening := qaSheetOpening(entries)
	if len(opening) == 0 {
		t.Fatalf("VTT-663: B's log has no opening; entries %v", entries)
	}
	if len(entries) <= len(opening) || entries[len(opening)].WorldSeq != bAdded {
		t.Errorf("VTT-663: the entries after the opening are not B's ActorAdded's frames at world sequence %d", bAdded)
	}
	// VTT-663
	first := opening[0].Env
	if first.GetSessionStarted() == nil || first.GetSessionStarted().GetName() != "Night one" || first.GetSessionId() != world[0].GetSessionId() {
		t.Errorf("VTT-663: the opening starts with %v, want the open session's SessionStarted", first)
	}
	earlier := map[string]bool{}
	for _, w := range world[:qaSheetUnseenBAdded-1] {
		earlier[w.GetEventId()] = true
	}
	var notes []string
	for i, e := range opening {
		if e.WorldSeq != 0 {
			t.Errorf("VTT-663: opening entry %d carries world sequence %d", i, e.WorldSeq)
		}
		if e.Env.GetEventId() != "" && earlier[e.Env.GetEventId()] {
			t.Errorf("VTT-663: opening entry %d is forwarded from earlier event %q", i, e.Env.GetEventId())
		}
		if n := e.Env.GetNoteUpserted(); n != nil {
			notes = append(notes, n.GetKey())
		}
		if e.Env.GetNarrationAdded() != nil {
			t.Errorf("VTT-663: the opening holds an earlier narration %v", e.Env)
		}
		// VTT-658
		if qaSheetCarries(e.Env, qaSheetA) {
			t.Errorf("VTT-658: opening entry %d carries A's values: %v", i, e.Env)
		}
	}
	for i, e := range entries {
		if id := e.Env.GetEventId(); id != "" && earlier[id] {
			t.Errorf("VTT-663: entry %d is forwarded from event %q appended before B's log opened", i, id)
		}
	}
	if fmt.Sprint(notes) != "[qa-public]" {
		t.Errorf("VTT-663: the opening's notes %v, want [qa-public]", notes)
	}
	opened := qaSheetFoldEntries(t, opening)
	if _, ok := opened.Actors[qaSheetB]; ok {
		t.Errorf("VTT-663: the opening introduces B, who was no party member before the event")
	}
	worldA := l.envs[4].GetActorAdded().GetActor()
	// VTT-658
	qaSheetAssertStripped(t, "VTT-658 in the opening", opened, worldA, []string{"p-a"})
}

// VTT-658 VTT-660 VTT-661 VTT-664 VTT-486
func TestQASheetACampaignLogIsWrittenNoCompanionSheetBeforeSight(t *testing.T) {
	l := qaSheetBuildUnseen(t)
	c, seqs := qaSheetAppend(t, l.envs)
	entries, err := c.CharacterLog(qaSheetB)
	if err != nil {
		t.Fatal(err)
	}
	doorSeq := seqs[qaSheetUnseenDoorOpen-1]
	before := qaSheetThrough(entries, doorSeq-1)
	// VTT-658, VTT-660
	for i, e := range before {
		if qaSheetCarries(e.Env, qaSheetA) {
			t.Errorf("entry %d (world sequence %d) carries A's values before B sees A: %v", i, e.WorldSeq, e.Env)
		}
		if e.Env.GetActorRemoved().GetActorId() == qaSheetA {
			t.Errorf("VTT-661: entry %d (world sequence %d) is a bare ActorRemoved of A before B sees A", i, e.WorldSeq)
		}
	}
	worldA := l.envs[4].GetActorAdded().GetActor()
	qaSheetAssertStripped(t, "VTT-660 before the door opens", qaSheetFoldEntries(t, before), worldA, []string{"p-a"})
	// VTT-664
	through := qaSheetThrough(entries, doorSeq)
	atDoor, err := campaign.FoldPrefix(qaSheetWorld(t, c)[:qaSheetUnseenDoorOpen])
	if err != nil {
		t.Fatal(err)
	}
	qaSheetAssertWhole(t, "VTT-664 at the door's opening", qaSheetFoldEntries(t, through), atDoor, qaSheetA)
	// VTT-486
	qaSheetAssertWhole(t, "VTT-486 at the end", qaSheetFoldEntries(t, entries), c.State(), qaSheetA)
}

// VTT-663 VTT-658 VTT-664 VTT-657
func TestQASheetALogOpenedInSightOfACompanionBringsItsSheetAtTheOpeningEvent(t *testing.T) {
	l := &qaSheetLog{}
	l.session()
	l.scene()
	l.actor(qaSheetActor(t, qaSheetA, "Aster", qaSheetPartyK, 14, 10, 12))
	l.grant(qaSheetA, "p-a", qaSheetPartyK)
	l.condition(qaSheetA, "blessed")
	l.token(qaSheetTokA, qaSheetA, 0, 1)
	l.actor(qaSheetActor(t, qaSheetB, "Brindle", qaSheetNonPtyK, 9, 7, 7))
	l.token(qaSheetTokB, qaSheetB, 1, 1)
	l.resource(qaSheetA, "pool", -2, 8)
	l.grant(qaSheetB, "p-b", qaSheetPartyK)
	c, seqs := qaSheetAppend(t, l.envs)
	entries, err := c.CharacterLog(qaSheetB)
	if err != nil {
		t.Fatal(err)
	}
	grantSeq := seqs[len(seqs)-1]
	opening := qaSheetOpening(entries)
	if len(entries) <= len(opening) || entries[len(opening)].WorldSeq != grantSeq {
		t.Errorf("VTT-663: the entries after the opening are not the grant's frames at world sequence %d", grantSeq)
	}
	// VTT-663, VTT-658
	worldA := proto.Clone(l.envs[2].GetActorAdded().GetActor()).(*vttv1.Actor)
	qaSheetAssertStripped(t, "VTT-663 in the opening", qaSheetFoldEntries(t, opening), worldA, []string{"p-a"})
	// VTT-664
	qaSheetAssertWhole(t, "VTT-664 at the event that opened the log", qaSheetFoldEntries(t, entries), c.State(), qaSheetA)
	// VTT-657
	qaSheetAssertWhole(t, "VTT-657 B's own sheet", qaSheetFoldEntries(t, entries), c.State(), qaSheetB)
}

// VTT-176
func TestQASheetTheWorldLogKeepsEveryEventUnchanged(t *testing.T) {
	l := qaSheetBuildUnseen(t)
	c, _ := qaSheetAppend(t, l.envs)
	world := qaSheetWorld(t, c)
	if len(world) != len(l.envs) {
		t.Fatalf("VTT-176: the world log holds %d events, want %d", len(world), len(l.envs))
	}
	// VTT-176
	for i, w := range world {
		in, out := proto.Clone(l.envs[i]).(*vttv1.Envelope), proto.Clone(w).(*vttv1.Envelope)
		in.Sequence, out.Sequence = 0, 0
		in.OccurredAt, out.OccurredAt = nil, nil
		in.SessionId, out.SessionId = "", ""
		if !proto.Equal(in, out) {
			t.Errorf("VTT-176: world event %d is %v, appended as %v", i+1, w, l.envs[i])
		}
	}
}

// VTT-660 VTT-312 VTT-664 VTT-486
func TestQASheetAReopenedCampaignBringsTheSheetAtTheFirstSight(t *testing.T) {
	l := qaSheetBuildUnseen(t)
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, env := range l.envs[:qaSheetUnseenDoorOpen-1] {
		if _, err := c.Append(proto.Clone(env).(*vttv1.Envelope)); err != nil {
			t.Fatalf("append %d: %v", i+1, err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	c, err = campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	var doorSeq int64
	for i, env := range l.envs[qaSheetUnseenDoorOpen-1:] {
		seq, err := c.Append(proto.Clone(env).(*vttv1.Envelope))
		if err != nil {
			t.Fatalf("append after reopen %d: %v", i+1, err)
		}
		if i == 0 {
			doorSeq = seq
		}
	}
	entries, err := c.CharacterLog(qaSheetB)
	if err != nil {
		t.Fatal(err)
	}
	worldA := l.envs[4].GetActorAdded().GetActor()
	// VTT-660
	qaSheetAssertStripped(t, "VTT-660 before the door, across a reopen", qaSheetFoldEntries(t, qaSheetThrough(entries, doorSeq-1)), worldA, []string{"p-a"})
	// VTT-312, VTT-664
	atDoor, err := campaign.FoldPrefix(qaSheetWorld(t, c)[:qaSheetUnseenDoorOpen])
	if err != nil {
		t.Fatal(err)
	}
	qaSheetAssertWhole(t, "VTT-664 at the door after a reopen", qaSheetFoldEntries(t, qaSheetThrough(entries, doorSeq)), atDoor, qaSheetA)
	// VTT-486
	qaSheetAssertWhole(t, "VTT-486 at the end after a reopen", qaSheetFoldEntries(t, entries), c.State(), qaSheetA)
}
