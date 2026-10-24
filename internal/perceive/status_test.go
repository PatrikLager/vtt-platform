package perceive_test

import (
	"fmt"
	"math"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

// testimonyLog is twoRooms as a log, with a party member of another player,
// the rogue, granted in the hero's room, and extra appended from sequence 11.
func testimonyLog(extra ...proto.Message) []step {
	steps := []step{
		{1, &vttv1.SessionStarted{Name: "n"}},
		{2, &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3,
			Tiles: twoRoomsTiles()}},
		{3, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "hero", Name: "Hero",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}},
		{4, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "goblin", Name: "Goblin",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}}},
		{5, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero",
			Position: &vttv1.GridPosition{X: 1, Y: 1}}},
		{6, &vttv1.TokenPlaced{TokenId: "t-gob", SceneId: "s", ActorId: "goblin",
			Position: &vttv1.GridPosition{X: 5, Y: 1}}},
		{7, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
		{8, &vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "rogue", Name: "Rogue",
			Kind:      vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Resources: map[string]*vttv1.Resource{"pool": {Current: 5, Max: 5}}}}},
		{9, &vttv1.TokenPlaced{TokenId: "t-rogue", SceneId: "s", ActorId: "rogue",
			Position: &vttv1.GridPosition{X: 2, Y: 1}}},
		{10, &vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-2",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}},
	}
	for i, p := range extra {
		steps = append(steps, step{int64(11 + i), p})
	}
	return steps
}

var testimonyDoor = &vttv1.GridPosition{X: 3, Y: 1}

// rogueLeaves takes the rogue behind the shut door.
var rogueLeaves = &vttv1.TokenMoved{TokenId: "t-rogue", SceneId: "s", To: &vttv1.GridPosition{X: 4, Y: 1}}

// runTestimony projects steps for eyes as a seat does, each event stamped with
// the metadata a log carries, so a forwarded frame can be told from a built one.
func runTestimony(tb testing.TB, eyes perceive.Eyes, steps []step) [][]*vttv1.Envelope {
	tb.Helper()
	st := engine.NewState()
	pr := newViewer(tb, eyes)
	out := make([][]*vttv1.Envelope, len(steps))
	for i, s := range steps {
		env := envelope(s.seq, s.payload)
		env.EventId = fmt.Sprintf("evt-%d", s.seq)
		env.ActorRole = "dm"
		env.ParticipantId = "p-dm"
		env.SessionId = "sess-1"
		if err := engine.Apply(st, env); err != nil {
			panic(fmt.Sprintf("runTestimony seq %d: %v", s.seq, err))
		}
		out[i] = pr.event(env, st)
	}
	return out
}

// foldSteps folds every frame a seat was sent, failing on the first refused.
func foldSteps(t *testing.T, frames [][]*vttv1.Envelope) *engine.State {
	t.Helper()
	viewer := engine.NewState()
	for i, batch := range frames {
		for _, e := range batch {
			if err := engine.Apply(viewer, e); err != nil {
				t.Fatalf("step %d: projected %T does not fold: %v", i, e.GetPayload(), err)
			}
		}
	}
	return viewer
}

// VTT-243
func TestAConditionOnACreatureBehindAShutDoorReachesNoPlayer(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
		&vttv1.DoorClosed{SceneId: "s", At: testimonyDoor},
		&vttv1.ConditionApplied{ActorId: "goblin", ConditionId: "marked"},
	))
	viewer := foldSteps(t, frames)
	if _, ok := viewer.Actors["goblin"]; !ok {
		t.Fatal("fixture check: the player must have seen the goblin while the door stood open")
	}
	if out := frames[12]; len(out) != 0 {
		t.Errorf("a condition on a goblin behind a shut door reached the player: %v", out)
	}
}

// VTT-243
func TestAPartyMemberInAnotherSceneIsNotHeardOf(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		&vttv1.SceneCreated{SceneId: "far", Name: "Far", GridWidth: 3, GridHeight: 3},
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.TokenPlaced{TokenId: "t-rogue-far", SceneId: "far", ActorId: "rogue",
			Position: &vttv1.GridPosition{X: 1, Y: 1}},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "marked"},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3},
		&vttv1.ConditionRemoved{ActorId: "rogue", ConditionId: "marked"},
	))
	viewer := foldSteps(t, frames)
	if _, ok := viewer.Actors["rogue"]; !ok {
		t.Fatal("fixture check: a party member stays on every roster (VTT-208)")
	}
	for i := 13; i <= 15; i++ {
		if len(frames[i]) != 0 {
			t.Errorf("step %d about a party member in another scene reached the player: %v", i, frames[i])
		}
	}
	if got := viewer.Actors["rogue"].GetResources()["pool"].GetCurrent(); got != 5 {
		t.Errorf("the player's rogue holds pool %d, want the 5 it was introduced with", got)
	}
}

// VTT-244
func TestAnUnseenPartyMembersAttackReachesNoPlayer(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		rogueLeaves,
		&vttv1.AttackRolled{AttackerId: "rogue", TargetId: "hero", Total: 12, Outcome: "hit"},
		&vttv1.AbilityUsed{ActorId: "rogue", AbilityId: "taunt", TargetIds: []string{"hero"}},
		&vttv1.AttackRolled{AttackerId: "hero", TargetId: "goblin", Total: 9, Outcome: "miss"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
		&vttv1.AttackRolled{AttackerId: "hero", TargetId: "goblin", Total: 15, Outcome: "hit"},
	))
	foldSteps(t, frames)
	for i, what := range map[int]string{11: "the rogue's attack on the hero", 12: "the rogue's ability on the hero",
		13: "the hero's attack on a goblin it does not see"} {
		if len(frames[i]) != 0 {
			t.Errorf("%s reached the player: %v", what, frames[i])
		}
	}
	if out := frames[15]; len(out) != 1 || out[0].GetAttackRolled() == nil {
		t.Errorf("the hero's attack on a goblin it sees did not reach the player, got %v", out)
	}
}

// VTT-245
func TestAControlChangeOnAnUnseenPartyMemberReachesNoPlayer(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		rogueLeaves,
		&vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-3",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		&vttv1.ActorControlRevoked{ActorId: "rogue", ParticipantId: "p-2"},
	))
	viewer := foldSteps(t, frames)
	for i := 11; i <= 12; i++ {
		if len(frames[i]) != 0 {
			t.Errorf("a control change on the unseen rogue reached the player: %v", frames[i])
		}
	}
	if ids := viewer.Actors["rogue"].GetControllerIds(); len(ids) != 1 || ids[0] != "p-2" {
		t.Errorf("the player's rogue is held by %v, want the [p-2] it was introduced with", ids)
	}
}

// VTT-365
func TestAPlayersOwnTokenlessCharacterIsHeardOf(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "familiar", Name: "Familiar",
			Kind:      vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Resources: map[string]*vttv1.Resource{"pool": {Current: 2, Max: 2}}}},
		&vttv1.ActorControlGranted{ActorId: "familiar", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		&vttv1.ResourceChanged{ActorId: "familiar", Resource: "pool", Delta: -1, NewValue: 1},
		&vttv1.ConditionApplied{ActorId: "familiar", ConditionId: "tired"},
	))
	viewer := foldSteps(t, frames)
	for i := 12; i <= 13; i++ {
		if len(frames[i]) != 1 || frames[i][0].GetEventId() == "" {
			t.Errorf("step %d about the player's own token-less familiar was not forwarded: %v", i, frames[i])
		}
	}
	if got := viewer.Actors["familiar"].GetResources()["pool"].GetCurrent(); got != 1 {
		t.Errorf("the player's familiar holds pool %d, want 1", got)
	}
}

// correctionFrames returns the status frames of out about actorID, failing on
// any that carries something a correction must not.
func correctionFrames(t *testing.T, out []*vttv1.Envelope, actorID string) []*vttv1.Envelope {
	t.Helper()
	var fix []*vttv1.Envelope
	for _, e := range out {
		var about, extra string
		switch p := e.GetPayload().(type) {
		case *vttv1.Envelope_ResourceChanged:
			about, extra = p.ResourceChanged.GetActorId(), p.ResourceChanged.GetReason()
		case *vttv1.Envelope_ConditionApplied:
			about, extra = p.ConditionApplied.GetActorId(), p.ConditionApplied.GetSource()
		case *vttv1.Envelope_ConditionRemoved:
			about, extra = p.ConditionRemoved.GetActorId(), p.ConditionRemoved.GetReason()
		case *vttv1.Envelope_ActorControlGranted:
			about = p.ActorControlGranted.GetActorId()
		case *vttv1.Envelope_ActorControlRevoked:
			about = p.ActorControlRevoked.GetActorId()
		default:
			continue
		}
		if about != actorID {
			continue
		}
		if e.GetEventId() != "" || e.GetActorRole() != "" || e.GetParticipantId() != "" ||
			e.GetSessionId() != "" || e.GetOccurredAt() != nil || extra != "" {
			t.Errorf("a frame bringing %s up to date carries more than its status: %v", actorID, e)
		}
		fix = append(fix, e)
	}
	return fix
}

// VTT-469 VTT-249 VTT-471
func TestDamageTakenOutOfSightArrivesWhenTheEyeWalksIn(t *testing.T) {
	steps := testimonyLog(
		rogueLeaves,
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3, Reason: "ability:stab:hit"},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "bleeding", Source: "ability:stab:hit"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	)
	frames := runTestimony(t, player(), steps)
	viewer := foldSteps(t, frames)

	fix := correctionFrames(t, frames[13], "rogue")
	if len(fix) != 2 || fix[0].GetResourceChanged() == nil || fix[1].GetConditionApplied() == nil {
		t.Fatalf("walking in on the rogue sent %v, want its pool and its condition", fix)
	}
	if rc := fix[0].GetResourceChanged(); rc.GetDelta() != -2 || rc.GetNewValue() != 3 {
		t.Errorf("the rogue's pool arrived as delta %d new %d, want -2 and 3", rc.GetDelta(), rc.GetNewValue())
	}
	for _, e := range fix {
		if e.GetSequence() != 14 {
			t.Errorf("a correction carries sequence %d, want the door's 14", e.GetSequence())
		}
	}
	if got := viewer.Actors["rogue"].GetResources()["pool"].GetCurrent(); got != 3 {
		t.Errorf("the player's rogue holds pool %d after seeing it, want 3", got)
	}
	if c := viewer.Conditions["rogue"]; len(c) != 1 || c[0].ID != "bleeding" {
		t.Errorf("the player's rogue carries %v after seeing it, want [bleeding]", c)
	}
}

// VTT-469 VTT-471 VTT-481
func TestAKindChangedOutOfSightIsBroughtToItsPresentOnSight(t *testing.T) {
	charm := &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "p-3",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}
	for _, tc := range []struct {
		name  string
		after []proto.Message
		want  []string
	}{
		{"with a controller", nil, []string{"p-3"}},
		{"with none", []proto.Message{&vttv1.ActorControlRevoked{ActorId: "goblin", ParticipantId: "p-3"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := []proto.Message{
				&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
				&vttv1.DoorClosed{SceneId: "s", At: testimonyDoor},
				charm,
			}
			log = append(log, tc.after...)
			log = append(log, &vttv1.DoorOpened{SceneId: "s", At: testimonyDoor})
			frames := runTestimony(t, player(), testimonyLog(log...))
			viewer := foldSteps(t, frames)
			for i := 12; i < len(frames)-1; i++ {
				if len(frames[i]) != 0 {
					t.Errorf("step %d, a control change on the unseen goblin, reached the player: %v", i, frames[i])
				}
			}
			g := viewer.Actors["goblin"]
			if g.GetKind() != vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER {
				t.Errorf("the player's goblin is %v after seeing it, want a party member", g.GetKind())
			}
			if ids := g.GetControllerIds(); !sameStrings(ids, tc.want) {
				t.Errorf("the player's goblin is held by %v after seeing it, want %v", ids, tc.want)
			}
		})
	}
}

// VTT-469
func TestAResourceStartedBelowZeroIsCorrectedOnSight(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wraith", Name: "Wraith",
			Kind:      vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Resources: map[string]*vttv1.Resource{"pool": {Current: math.MinInt32}}}},
		&vttv1.TokenPlaced{TokenId: "t-wraith", SceneId: "s", ActorId: "wraith",
			Position: &vttv1.GridPosition{X: 5, Y: 1}},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: 1, NewValue: 0},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: math.MaxInt32, NewValue: math.MaxInt32},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	viewer := foldSteps(t, frames)
	fix := correctionFrames(t, frames[14], "wraith")
	if len(fix) != 2 {
		t.Fatalf("seeing the wraith sent %v, want two frames through zero", fix)
	}
	first, second := fix[0].GetResourceChanged(), fix[1].GetResourceChanged()
	if first.GetDelta() != 0 || first.GetNewValue() != 0 ||
		second.GetDelta() != math.MaxInt32 || second.GetNewValue() != math.MaxInt32 {
		t.Errorf("the wraith's pool arrived as %v then %v, want 0/0 then %d/%d",
			first, second, math.MaxInt32, math.MaxInt32)
	}
	if got := viewer.Actors["wraith"].GetResources()["pool"].GetCurrent(); got != math.MaxInt32 {
		t.Errorf("the player's wraith holds pool %d, want %d", got, math.MaxInt32)
	}
}

// VTT-250
func TestAChangeUndoneOutOfSightSendsNothingOnSight(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		rogueLeaves,
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "marked"},
		&vttv1.ConditionRemoved{ActorId: "rogue", ConditionId: "marked"},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: 2, NewValue: 5},
		&vttv1.ActorControlGranted{ActorId: "rogue", ParticipantId: "p-3",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		&vttv1.ActorControlRevoked{ActorId: "rogue", ParticipantId: "p-3"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	foldSteps(t, frames)
	if fix := correctionFrames(t, frames[17], "rogue"); len(fix) != 0 {
		t.Errorf("seeing the rogue, whose changes were all undone unseen, sent %v", fix)
	}
}

// VTT-250
func TestConditionsReorderedOutOfSightSendNothingOnSight(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "a"},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "b"},
		rogueLeaves,
		&vttv1.ConditionRemoved{ActorId: "rogue", ConditionId: "a"},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "a"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	foldSteps(t, frames)
	if fix := correctionFrames(t, frames[15], "rogue"); len(fix) != 0 {
		t.Errorf("seeing the rogue, whose conditions were only reordered unseen, sent %v", fix)
	}
}

// VTT-469
func TestADifferenceOfExactlyMaxInt32IsOneCorrection(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "wraith", Name: "Wraith",
			Kind:      vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Resources: map[string]*vttv1.Resource{"pool": {Current: -1}}}},
		&vttv1.TokenPlaced{TokenId: "t-wraith", SceneId: "s", ActorId: "wraith",
			Position: &vttv1.GridPosition{X: 5, Y: 1}},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: 1, NewValue: 0},
		&vttv1.ResourceChanged{ActorId: "wraith", Resource: "pool", Delta: math.MaxInt32 - 1, NewValue: math.MaxInt32 - 1},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
	))
	foldSteps(t, frames)
	fix := correctionFrames(t, frames[14], "wraith")
	if len(fix) != 1 || fix[0].GetResourceChanged().GetDelta() != math.MaxInt32 {
		t.Errorf("a difference of exactly MaxInt32 arrived as %v, want one frame carrying it", fix)
	}
}

// VTT-251
func TestARemovalOutOfSightIsNotReported(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		rogueLeaves,
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.ActorRemoved{ActorId: "rogue"},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "familiar", Name: "Familiar",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}},
		&vttv1.ActorControlGranted{ActorId: "familiar", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		&vttv1.ActorRemoved{ActorId: "familiar"},
	))
	viewer := foldSteps(t, frames)
	if out := frames[12]; len(out) != 0 {
		t.Errorf("the removal of the rogue, unseen behind the door, reached the player: %v", out)
	}
	if _, ok := viewer.Actors["rogue"]; !ok {
		t.Error("the player's fold dropped the rogue it never saw removed")
	}
	if out := frames[15]; len(out) != 1 || out[0].GetActorRemoved().GetActorId() != "familiar" {
		t.Errorf("the removal of the player's own familiar, which it sees, did not reach it: %v", out)
	}
}

// VTT-252 VTT-470
func TestAnIdReusedAfterAnUnseenRemovalFoldsAgain(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		rogueLeaves,
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.ActorRemoved{ActorId: "rogue"},
		&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "rogue", Name: "Second Rogue", ModuleId: "other",
			Kind:      vttv1.ActorKind_ACTOR_KIND_NON_PARTY,
			Resources: map[string]*vttv1.Resource{"pool": {Current: 9, Max: 9}}}},
		&vttv1.TokenPlaced{TokenId: "t-rogue-2", SceneId: "s", ActorId: "rogue",
			Position: &vttv1.GridPosition{X: 2, Y: 1}},
	))
	viewer := foldSteps(t, frames)
	out := frames[14]
	if len(out) < 2 || out[0].GetActorRemoved().GetActorId() != "rogue" || out[0].GetEventId() != "" ||
		out[1].GetActorAdded().GetActor().GetActorId() != "rogue" {
		t.Fatalf("the reused rogue arrived as %v, want a bare ActorRemoved and then its introduction", out)
	}
	want := &vttv1.Actor{ActorId: "rogue", ModuleId: "other", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY}
	if a := viewer.Actors["rogue"]; !proto.Equal(a, want) {
		t.Errorf("the player holds rogue as %v, want the second incarnation as a log is sent it, %v", a, want)
	}
}

// VTT-252
func TestARemovedActorWhoseIdNeverReturnsStaysInTheFold(t *testing.T) {
	frames := runTestimony(t, player(), testimonyLog(
		rogueLeaves,
		&vttv1.TokenRemoved{TokenId: "t-rogue"},
		&vttv1.ActorRemoved{ActorId: "rogue"},
		&vttv1.DoorOpened{SceneId: "s", At: testimonyDoor},
		&vttv1.ConditionApplied{ActorId: "hero", ConditionId: "relieved"},
	))
	viewer := foldSteps(t, frames)
	for i, batch := range frames {
		for _, e := range batch {
			if e.GetActorRemoved().GetActorId() == "rogue" {
				t.Errorf("step %d sent the player a removal of the rogue, whose id never returned: %v", i, e)
			}
		}
	}
	if _, ok := viewer.Actors["rogue"]; !ok {
		t.Error("the player's fold dropped the rogue, whose removal it never saw")
	}
}

// VTT-272 VTT-273 VTT-222 VTT-243
func TestAChangeByAnUnseenUserNamesNoAbilityAndNoIssuer(t *testing.T) {
	const cause = "ability:claw:hit"
	steps := testimonyLog(
		&vttv1.AbilityUsed{ActorId: "goblin", AbilityId: "claw", TargetIds: []string{"rogue"}},
		&vttv1.ResourceChanged{ActorId: "rogue", Resource: "pool", Delta: -2, NewValue: 3, Reason: cause},
		&vttv1.ConditionApplied{ActorId: "rogue", ConditionId: "bleeding", Source: cause},
	)
	for _, v := range []struct {
		role string
		eyes perceive.Eyes
	}{
		{"player", perceive.Eyes{Controller: "p-2"}},
		{"spectator", perceive.Eyes{Shoulder: "rogue"}},
	} {
		st := engine.NewState()
		pr := newViewer(t, v.eyes)
		for _, s := range steps {
			in := envelope(s.seq, s.payload)
			in.EventId, in.ActorRole, in.ParticipantId, in.SessionId = fmt.Sprintf("evt-%d", s.seq), "player", "p-3", "sess-1"
			before := proto.Clone(in)
			if err := engine.Apply(st, in); err != nil {
				t.Fatalf("seq %d: %v", s.seq, err)
			}
			out := pr.event(in, st)
			if !proto.Equal(in, before) {
				t.Errorf("%s: projecting seq %d changed the event", v.role, s.seq)
			}
			if s.seq < 11 {
				continue
			}
			var sent *vttv1.Envelope
			for _, e := range out {
				if e.GetAbilityUsed() != nil {
					t.Errorf("%s: sent the ability of a goblin it does not see", v.role)
				}
				if e.GetEventId() != "" {
					sent = e
				}
			}
			if s.seq == 11 {
				continue
			}
			if sent == nil {
				t.Fatalf("%s: the change at seq %d on the rogue was not forwarded", v.role, s.seq)
			}
			if sent.GetParticipantId() != "" || sent.GetActorRole() != "" ||
				sent.GetResourceChanged().GetReason() != "" || sent.GetConditionApplied().GetSource() != "" {
				t.Errorf("%s: seq %d names its issuer or its cause: %v", v.role, s.seq, sent)
			}
			if !proto.Equal(sent, forwardedOf(in)) {
				t.Errorf("%s: forwarded %v, want the event less its issuer and its cause", v.role, sent)
			}
		}
	}
}

// VTT-466 VTT-486
func TestACreaturesResourceChangeReachesNoLog(t *testing.T) {
	st := creatureWorld()
	v := newViewer(t, heroEyes())
	next(v, st, 10, &vttv1.NarrationAdded{Text: "look"})
	resourceFrames := func(out []*vttv1.Envelope) int {
		n := 0
		for _, e := range out {
			if e.GetResourceChanged() != nil {
				n++
			}
		}
		return n
	}
	if n := resourceFrames(next(v, st, 11, &vttv1.ResourceChanged{
		ActorId: "goblin", Resource: "vigor", Delta: -2, NewValue: 3})); n != 0 {
		t.Fatalf("a seen creature's resource change was sent (%d frames)", n)
	}
	next(v, st, 12, &vttv1.TokenMoved{TokenId: "t-gob", SceneId: "s", To: &vttv1.GridPosition{X: 5, Y: 1}})
	if n := resourceFrames(next(v, st, 13, &vttv1.ResourceChanged{
		ActorId: "goblin", Resource: "vigor", Delta: -1, NewValue: 2})); n != 0 {
		t.Fatalf("an unseen creature's resource change was sent (%d frames)", n)
	}
	if n := resourceFrames(next(v, st, 14, &vttv1.ResourceChanged{
		ActorId: "hero", Resource: "vigor", Delta: -1, NewValue: 8})); n != 1 {
		t.Fatalf("the party member's own resource change was sent as %d frames, want 1", n)
	}
}

// reshapeWorld is creatureWorld with a second party member, the ally, by the hero.
func reshapeWorld() *engine.State {
	st := creatureWorld()
	mustApply(st, 20, &vttv1.ActorAdded{Actor: &vttv1.Actor{
		ActorId: "ally", Name: "Ally", ModuleId: "m", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
		Resources: map[string]*vttv1.Resource{"vigor": {Current: 4, Max: 4}}}})
	mustApply(st, 21, &vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s",
		ActorId: "ally", Position: &vttv1.GridPosition{X: 1, Y: 1}})
	mustApply(st, 22, &vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	return st
}

// VTT-468
func TestACreatureAGrantMakesAPartyMemberInSightArrivesWhole(t *testing.T) {
	st := reshapeWorld()
	v := newViewer(t, heroEyes())
	next(v, st, 30, &vttv1.NarrationAdded{Text: "look"})

	out := next(v, st, 31, &vttv1.ActorControlGranted{ActorId: "goblin", ParticipantId: "p-3",
		Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
	want := []string{
		"*vttv1.Envelope_TokenHidden t-gob",
		"*vttv1.Envelope_ActorRemoved goblin",
		"*vttv1.Envelope_ActorAdded goblin",
		"*vttv1.Envelope_ActorControlGranted goblin",
		"*vttv1.Envelope_ActorControlGranted goblin",
		"*vttv1.Envelope_ConditionApplied goblin",
		"*vttv1.Envelope_TokenPlaced t-gob",
		"*vttv1.Envelope_ActorControlGranted goblin",
	}
	if got := shapes(out); !sameStrings(got, want) {
		t.Fatalf("the reshaping grant was sent as\n%v\nwant\n%v", got, want)
	}
	if a := v.shown.Actors["goblin"]; a.GetName() != "Goblin Cutter" || a.GetResources()["vigor"].GetMax() != 5 {
		t.Fatalf("the new party member is held as %v, want it whole", a)
	}
	for _, e := range next(v, st, 32, &vttv1.NarrationAdded{Text: "after"}) {
		if e.GetActorAdded() != nil || e.GetActorRemoved() != nil || e.GetTokenHidden() != nil {
			t.Fatalf("the next event sent the reshaped actor again: %v", e)
		}
	}
}

// VTT-468
func TestAPartyMemberAGrantMakesACreatureInSightIsHeldStripped(t *testing.T) {
	st := reshapeWorld()
	v := newViewer(t, heroEyes())
	next(v, st, 30, &vttv1.NarrationAdded{Text: "look"})

	out := next(v, st, 31, &vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-agent",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
	want := []string{
		"*vttv1.Envelope_TokenHidden t-ally",
		"*vttv1.Envelope_ActorRemoved ally",
		"*vttv1.Envelope_ActorAdded ally",
		"*vttv1.Envelope_ActorControlGranted ally",
		"*vttv1.Envelope_ActorControlGranted ally",
		"*vttv1.Envelope_TokenPlaced t-ally",
		"*vttv1.Envelope_ActorControlGranted ally",
	}
	if got := shapes(out); !sameStrings(got, want) {
		t.Fatalf("the reshaping grant was sent as\n%v\nwant\n%v", got, want)
	}
	held := v.shown.Actors["ally"]
	if held.GetName() != "" || len(held.GetResources()) != 0 {
		t.Fatalf("the new creature is held as %v, want it stripped", held)
	}
	for _, e := range next(v, st, 32, &vttv1.NarrationAdded{Text: "after"}) {
		if e.GetActorAdded() != nil || e.GetActorRemoved() != nil || e.GetTokenHidden() != nil {
			t.Fatalf("the next event sent the reshaped actor again: %v", e)
		}
	}
}

// VTT-481
func TestAReshapingKindChangeOutOfSightWaitsForSight(t *testing.T) {
	// Give no actor here a resource, and each a controller: else correct's own
	// re-introduction, which a differing resource set forces, hides the reshape.
	build := func() *engine.State {
		st := engine.NewState()
		mustApply(st, 1, &vttv1.SessionStarted{Name: "n"})
		mustApply(st, 2, &vttv1.SceneCreated{
			SceneId: "s", Name: "S", GridWidth: 7, GridHeight: 3, Tiles: twoRoomsTiles()})
		for i, a := range []*vttv1.Actor{
			{ActorId: "hero", Name: "Hero", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
			{ActorId: "imp", Name: "Imp", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
			{ActorId: "ally", Name: "Ally", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
		} {
			mustApply(st, int64(3+i), &vttv1.ActorAdded{Actor: a})
		}
		mustApply(st, 6, &vttv1.TokenPlaced{TokenId: "t-hero", SceneId: "s", ActorId: "hero",
			Position: &vttv1.GridPosition{X: 1, Y: 1}})
		mustApply(st, 7, &vttv1.TokenPlaced{TokenId: "t-imp", SceneId: "s", ActorId: "imp",
			Position: &vttv1.GridPosition{X: 2, Y: 1}})
		mustApply(st, 8, &vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally",
			Position: &vttv1.GridPosition{X: 2, Y: 1}})
		mustApply(st, 9, &vttv1.ActorControlGranted{ActorId: "hero", ParticipantId: "p-1",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
		mustApply(st, 10, &vttv1.ActorControlGranted{ActorId: "imp", ParticipantId: "p-agent",
			Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY})
		mustApply(st, 11, &vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2",
			Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER})
		return st
	}
	for _, c := range []struct {
		name, actor, token string
		kind               vttv1.ActorKind
		wantName           string
	}{
		{"a creature made a party member", "imp", "t-imp", vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER, "Imp"},
		{"a party member made a creature", "ally", "t-ally", vttv1.ActorKind_ACTOR_KIND_NON_PARTY, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := build()
			v := newViewer(t, heroEyes())
			next(v, st, 12, &vttv1.NarrationAdded{Text: "look"})
			next(v, st, 13, &vttv1.TokenMoved{TokenId: c.token, SceneId: "s", To: &vttv1.GridPosition{X: 5, Y: 1}})
			for _, e := range next(v, st, 14, &vttv1.ActorControlGranted{ActorId: c.actor,
				ParticipantId: "p-3", Kind: c.kind}) {
				t.Fatalf("an unseen reshaping grant sent %v", shapes([]*vttv1.Envelope{e}))
			}
			out := next(v, st, 15, &vttv1.TokenMoved{TokenId: c.token, SceneId: "s",
				To: &vttv1.GridPosition{X: 2, Y: 1}})
			removed, added := -1, -1
			for i, e := range out {
				if e.GetActorRemoved().GetActorId() == c.actor {
					removed = i
				}
				if e.GetActorAdded().GetActor().GetActorId() == c.actor {
					added = i
				}
			}
			if removed < 0 || added < removed {
				t.Fatalf("coming into sight sent %v, want a withdrawal and then an introduction", shapes(out))
			}
			if got := v.shown.Actors[c.actor].GetName(); got != c.wantName {
				t.Fatalf("the log holds %s named %q, want %q", c.actor, got, c.wantName)
			}
		})
	}
}
