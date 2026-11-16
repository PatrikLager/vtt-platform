package perceive_test

import (
	"fmt"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/perceive"
)

func listedAlly(t *testing.T) *vttv1.Actor {
	t.Helper()
	a := sheetAlly(t, true, true)
	a.AbilityIds = []string{"dash", "parry", "dash"}
	return a
}

func allyShapes(t *testing.T) map[string]*vttv1.Actor {
	t.Helper()
	return map[string]*vttv1.Actor{"a list alone": abilityAlly(), "a sheet naming an id twice": listedAlly(t)}
}

func abilityChanges(out []*vttv1.Envelope, id string) []string {
	var got []string
	for _, e := range out {
		if g := e.GetAbilityGiven(); g != nil && g.GetActorId() == id {
			got = append(got, "given "+g.GetAbilityId())
		}
		if tk := e.GetAbilityTaken(); tk != nil && tk.GetActorId() == id {
			got = append(got, "taken "+tk.GetAbilityId())
		}
	}
	return got
}

func arrivals(out []*vttv1.Envelope, id string) int {
	n := 0
	for _, e := range out {
		if e.GetActorRemoved().GetActorId() == id || e.GetActorAdded().GetActor().GetActorId() == id {
			n++
		}
	}
	return n
}

func sameIDSet(a, b []string) bool {
	in := func(ids []string) map[string]bool {
		m := map[string]bool{}
		for _, id := range ids {
			m[id] = true
		}
		return m
	}
	x, y := in(a), in(b)
	if len(x) != len(y) {
		return false
	}
	for id := range x {
		if !y[id] {
			return false
		}
	}
	return true
}

func heldIDs(st *engine.State, id string) []string {
	return slices.Clone(st.Actors[id].GetAbilityIds())
}

// VTT-744
func TestALogThatSawAGiveHoldsTheAbilityAndIsNeverSentTheActorAgain(t *testing.T) {
	for name, ally := range allyShapes(t) {
		t.Run(name, func(t *testing.T) {
			tb := newSheetTable(t, ally)
			tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			want := append(heldIDs(tb.st, "ally"), "dodge")
			out := tb.step(&vttv1.AbilityGiven{ActorId: "ally", AbilityId: "dodge"})
			if got := abilityChanges(out, "ally"); !sameStrings(got, []string{"given dodge"}) {
				t.Errorf("the give in sight sent %v, want the give alone", shapes(out))
			}
			if ids := heldIDs(tb.v.shown, "ally"); !sameStrings(ids, want) {
				t.Errorf("after the give the log holds the ally naming %q, want %q", ids, want)
			}
			for _, p := range []proto.Message{
				&vttv1.NarrationAdded{Text: "later"},
				&vttv1.DoorClosed{SceneId: "s", At: sheetDoor},
				&vttv1.NarrationAdded{Text: "unseen"},
				&vttv1.DoorOpened{SceneId: "s", At: sheetDoor},
				&vttv1.NarrationAdded{Text: "seen again"},
			} {
				out = append(out, tb.step(p)...)
			}
			if n := arrivals(out, "ally"); n != 0 {
				t.Errorf("the give and the events after it sent %d ActorRemoved or ActorAdded of the ally: %v", n, shapes(out))
			}
			sheetHeldAsTheWorldHoldsIt(t, tb)
		})
	}
}

// VTT-724 VTT-744
func TestALogThatHeldAPartyMemberWholeOutOfSightIsSentNothingUntilSightBringsTheChanges(t *testing.T) {
	for name, ally := range allyShapes(t) {
		t.Run(name, func(t *testing.T) {
			tb := newSheetTable(t, ally)
			tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
			held := heldIDs(tb.v.shown, "ally")
			first := held[0]
			for _, p := range []proto.Message{
				&vttv1.AbilityGiven{ActorId: "ally", AbilityId: "dodge"},
				&vttv1.AbilityTaken{ActorId: "ally", AbilityId: first},
				&vttv1.NarrationAdded{Text: "unseen"},
				&vttv1.AbilityGiven{ActorId: "ally", AbilityId: "lunge"},
			} {
				out := tb.step(p)
				if _, told := p.(*vttv1.NarrationAdded); told && len(out) == 1 && out[0].GetNarrationAdded() != nil {
					continue
				}
				if len(out) != 0 {
					t.Errorf("%T out of sight sent %v, want nothing", p, shapes(out))
				}
			}
			if ids := heldIDs(tb.v.shown, "ally"); !sameStrings(ids, held) {
				t.Errorf("out of sight the log's ally came to name %q, want %q as it last saw it", ids, held)
			}
			sight := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
			if n := arrivals(sight, "ally"); n != 0 {
				t.Errorf("at sight the ally was sent %d ActorRemoved or ActorAdded: %v", n, shapes(sight))
			}
			want := []string{"taken " + first, "given dodge", "given lunge"}
			if got := abilityChanges(sight, "ally"); !sameStrings(got, want) {
				t.Errorf("at sight the ally was sent %q, want %q", got, want)
			}
			for _, e := range sight {
				if e.GetAbilityGiven() != nil || e.GetAbilityTaken() != nil {
					if e.GetEventId() != "" || e.GetSequence() != tb.seq || e.GetParticipantId() != "" {
						t.Errorf("a correction frame carries %v, want the event's sequence and nothing else of an envelope", e)
					}
				}
			}
			if ids, w := heldIDs(tb.v.shown, "ally"), heldIDs(tb.st, "ally"); !sameIDSet(ids, w) {
				t.Errorf("after sight the log holds the ally naming %q, the world %q", ids, w)
			}
		})
	}
}

// VTT-724, VTT-743
func TestASightCorrectionSendsTheAbilitiesAfterTheConditionsAndBeforeTheControllers(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	for _, p := range []proto.Message{
		&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-3", Kind: sheetParty},
		&vttv1.AbilityTaken{ActorId: "ally", AbilityId: "rally"},
		&vttv1.ConditionApplied{ActorId: "ally", ConditionId: "dazed"},
		&vttv1.AbilityGiven{ActorId: "ally", AbilityId: "dodge"},
	} {
		tb.step(p)
	}
	var got []string
	for _, e := range tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor}) {
		switch p := e.GetPayload().(type) {
		case *vttv1.Envelope_ConditionApplied:
			got = append(got, "applied "+p.ConditionApplied.GetConditionId())
		case *vttv1.Envelope_AbilityTaken:
			got = append(got, "taken "+p.AbilityTaken.GetAbilityId())
		case *vttv1.Envelope_AbilityGiven:
			got = append(got, "given "+p.AbilityGiven.GetAbilityId())
		case *vttv1.Envelope_ActorControlGranted:
			got = append(got, "granted "+p.ActorControlGranted.GetParticipantId())
		}
	}
	want := []string{"applied dazed", "taken rally", "given dodge", "granted p-3"}
	if !sameStrings(got, want) {
		t.Errorf("at sight the ally's correction is %q, want %q", got, want)
	}
}

// VTT-724 VTT-744
func TestATakeAndAGiveBackWhileUnseenSendNothingAtSight(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.AbilityTaken{ActorId: "ally", AbilityId: "rally"})
	tb.step(&vttv1.AbilityGiven{ActorId: "ally", AbilityId: "rally"})
	if held, w := heldIDs(tb.v.shown, "ally"), heldIDs(tb.st, "ally"); slices.Equal(held, w) {
		t.Fatalf("fixture: the log and the world both name %q, want them in another order", w)
	}
	out := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	out = append(out, tb.step(&vttv1.NarrationAdded{Text: "seen"})...)
	if got := abilityChanges(out, "ally"); len(got) != 0 {
		t.Errorf("a take and a give back while unseen sent %q at sight, want nothing", got)
	}
	if n := arrivals(out, "ally"); n != 0 {
		t.Errorf("a take and a give back while unseen sent %d ActorRemoved or ActorAdded at sight: %v", n, shapes(out))
	}
	if held, w := heldIDs(tb.v.shown, "ally"), heldIDs(tb.st, "ally"); !sameIDSet(held, w) {
		t.Errorf("the log holds the ally naming %q, the world %q", held, w)
	}
}

// VTT-741 VTT-744 VTT-698
func TestACreaturesGiveReachesNoLogAndItArrivesWithoutItsList(t *testing.T) {
	tb := newSheetTable(t, sheetAlly(t, false, false))
	tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "beast", Name: "Beast",
		Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, AbilityIds: []string{"fists"}}})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-beast", SceneId: "s", ActorId: "beast",
		Position: &vttv1.GridPosition{X: 4, Y: 1}})
	steps := []struct {
		payload   proto.Message
		silent    bool
		beastSeen bool
	}{
		{&vttv1.AbilityGiven{ActorId: "beast", AbilityId: "bite"}, true, false},
		{&vttv1.DoorOpened{SceneId: "s", At: sheetDoor}, false, true},
		{&vttv1.AbilityTaken{ActorId: "beast", AbilityId: "fists"}, true, true},
		{&vttv1.AbilityGiven{ActorId: "beast", AbilityId: "claw"}, true, true},
		{&vttv1.DoorClosed{SceneId: "s", At: sheetDoor}, false, false},
		{&vttv1.AbilityTaken{ActorId: "beast", AbilityId: "bite"}, true, false},
		{&vttv1.DoorOpened{SceneId: "s", At: sheetDoor}, false, true},
	}
	var all []*vttv1.Envelope
	for _, s := range steps {
		out := tb.step(s.payload)
		if s.silent && len(out) != 0 {
			t.Errorf("%v of the creature sent %v, want nothing", s.payload, shapes(out))
		}
		if got := perceive.SeesActor(player(), tb.st, "beast"); got != s.beastSeen {
			t.Fatalf("fixture: after %T the player sees the creature = %v, want %v", s.payload, got, s.beastSeen)
		}
		all = append(all, out...)
	}
	if got := abilityChanges(all, "beast"); len(got) != 0 {
		t.Errorf("the log was sent the creature's %q", got)
	}
	added := qacsFind(all, qacsAddedOf("beast"))
	if len(added) != 1 || arrivals(all, "beast") != 1 {
		t.Fatalf("the creature arrived as %v, want one introduction", shapes(all))
	}
	if ids := all[added[0]].GetActorAdded().GetActor().GetAbilityIds(); len(ids) != 0 {
		t.Errorf("the creature was introduced naming %q", ids)
	}
	if ids := heldIDs(tb.v.shown, "beast"); len(ids) != 0 {
		t.Errorf("the log holds the creature naming %q", ids)
	}
}

// VTT-742 VTT-744
func TestAResumeHoldsTheWorldsAbilityIdsInAnyOrderAndStartsAfreshFromALaggingList(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	eyes := player()
	attack := envelope(tb.seq+1, &vttv1.AttackRolled{AttackerId: "hero", TargetId: "ally", Total: 7})
	world := tb.st.Snapshot()
	mustFold(t, world, attack)

	for name, ids := range map[string][]string{
		"as the world holds them": {"rally", "fists"},
		"in another order":        {"fists", "rally"},
	} {
		shown := tb.v.shown.Snapshot()
		shown.Actors["ally"].AbilityIds = ids
		if out := perceive.Resume(eyes, shown, world).Event(eyes, attack, shown, world); !forwardsAttack(out) {
			t.Errorf("a fold holding the ally's ids %s does not resume: the attack is withheld: %v", name, shapes(out))
		}
	}

	for name, ids := range map[string][]string{
		"lacking one of the world's": {"rally"},
		"naming one the world lacks": {"rally", "fists", "lunge"},
		"naming none":                nil,
	} {
		shown := tb.v.shown.Snapshot()
		shown.Actors["ally"].AbilityIds = ids
		if !forwardsAttack(perceive.New(eyes, world).Event(eyes, attack, shown, world)) {
			t.Fatalf("%s: New withholds the attack too, so this case proves nothing", name)
		}
		out := perceive.Resume(eyes, shown, world).Event(eyes, attack, shown, world)
		if forwardsAttack(out) {
			t.Errorf("a fold %s resumes: the attack is forwarded before the list is corrected: %v", name, shapes(out))
		}
		if n := arrivals(out, "ally"); n != 0 {
			t.Errorf("a fold %s is sent %d ActorRemoved or ActorAdded of the ally: %v", name, n, shapes(out))
		}
		for i, e := range out {
			if err := engine.Apply(shown, e); err != nil {
				t.Fatalf("%s: frame %d (%T) does not fold: %v", name, i, e.GetPayload(), err)
			}
		}
		if got, w := heldIDs(shown, "ally"), heldIDs(world, "ally"); !sameIDSet(got, w) {
			t.Errorf("a fold %s holds the ally naming %q after the frames, the world %q", name, got, w)
		}
	}
}

// VTT-784 VTT-744
func TestAListOnlyCompanionAddedAgainWithASheetUnderItsNameWaitsForSight(t *testing.T) {
	tb := newSheetTable(t, abilityAlly())
	tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	tb.step(&vttv1.DoorClosed{SceneId: "s", At: sheetDoor})
	again := sheetAlly(t, true, true)
	again.AbilityIds = []string{"rally"}
	var unseen []*vttv1.Envelope
	for _, p := range []proto.Message{
		&vttv1.TokenRemoved{TokenId: "t-ally"},
		&vttv1.ActorRemoved{ActorId: "ally"},
		&vttv1.ActorAdded{Actor: again},
		&vttv1.TokenPlaced{TokenId: "t-ally", SceneId: "s", ActorId: "ally", Position: &vttv1.GridPosition{X: 5, Y: 1}},
		&vttv1.ActorControlGranted{ActorId: "ally", ParticipantId: "p-2", Kind: sheetParty},
		&vttv1.NarrationAdded{Text: "unseen"},
	} {
		unseen = append(unseen, tb.step(p)...)
	}
	if n := arrivals(unseen, "ally"); n != 0 {
		t.Errorf("the ally added again under its name out of sight was sent %v, want nothing of it until sight", shapes(unseen))
	}
	sight := tb.step(&vttv1.DoorOpened{SceneId: "s", At: sheetDoor})
	removed, added := qacsFind(sight, qacsRemovedOf("ally")), qacsFind(sight, qacsAddedOf("ally"))
	if len(removed) != 1 || len(added) != 1 || removed[0] > added[0] {
		t.Errorf("at sight the ally added again with a sheet was sent %v, want it withdrawn and introduced whole", shapes(sight))
	}
	sheetHeldAsTheWorldHoldsIt(t, tb)
}

// VTT-724 VTT-744
func TestALogThatSawAPartyMemberEarlierIsSentNothingOfItsChangesUntilItSeesItAgain(t *testing.T) {
	tb := qaFwOpen(t, qaFwWorld, qaFwMoveAway)
	qaFwSight(t, tb, "act-b", "act-a", false)
	held := heldIDs(tb.viewers["act-b"].shown, "act-a")
	seq := int64(40)
	next := func(env *vttv1.Envelope) map[string][]*vttv1.Envelope {
		seq++
		env.EventId, env.Sequence, env.SessionId = "w-next", seq, "sess-1"
		return tb.step(t, env, true)
	}
	for _, env := range []*vttv1.Envelope{
		{Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{ActorId: "act-a", AbilityId: "lunge"}}},
		{Payload: &vttv1.Envelope_AbilityTaken{AbilityTaken: &vttv1.AbilityTaken{ActorId: "act-a", AbilityId: "parry"}}},
		{Payload: &vttv1.Envelope_AbilityGiven{AbilityGiven: &vttv1.AbilityGiven{ActorId: "act-b", AbilityId: "dash"}}},
	} {
		if out := next(env)["act-b"]; arrivals(out, "act-a") != 0 || len(abilityChanges(out, "act-a")) != 0 {
			t.Errorf("act-b's log, out of sight of act-a, was sent %v at %v", shapes(out), env.GetPayload())
		}
	}
	if ids := heldIDs(tb.viewers["act-b"].shown, "act-a"); !sameStrings(ids, held) {
		t.Errorf("out of sight act-b's log came to hold act-a naming %q, want %q", ids, held)
	}
	back := next(&vttv1.Envelope{Payload: &vttv1.Envelope_TokenMoved{TokenMoved: &vttv1.TokenMoved{
		TokenId: "tok-a", SceneId: "scn-1", From: &vttv1.GridPosition{X: 5}, To: &vttv1.GridPosition{X: 0}}}})["act-b"]
	qaFwSight(t, tb, "act-b", "act-a", true)
	if n := arrivals(back, "act-a"); n != 0 {
		t.Errorf("at sight act-b's log was sent %d ActorRemoved or ActorAdded of act-a: %v", n, shapes(back))
	}
	if got, want := abilityChanges(back, "act-a"), []string{"taken parry", "given lunge"}; !sameStrings(got, want) {
		t.Errorf("at sight act-b's log was sent %q of act-a, want %q", got, want)
	}
	if got, w := heldIDs(tb.viewers["act-b"].shown, "act-a"), heldIDs(tb.world, "act-a"); !sameIDSet(got, w) {
		t.Errorf("after sight act-b's log holds act-a naming %q, the world %q", got, w)
	}
}

func namelessIn(t *testing.T, kind vttv1.ActorKind, list ...string) *sheetTable {
	t.Helper()
	tb := newSheetTable(t, sheetAlly(t, false, false))
	tb.step(&vttv1.ActorAdded{Actor: &vttv1.Actor{ActorId: "who", ModuleId: "mod-who", Kind: kind, AbilityIds: list}})
	tb.step(&vttv1.TokenPlaced{TokenId: "t-who", SceneId: "s", ActorId: "who", Position: &vttv1.GridPosition{X: 2, Y: 1}})
	if !perceive.SeesActor(player(), tb.st, "who") {
		t.Fatalf("fixture: the player does not see the nameless actor")
	}
	return tb
}

// VTT-741
func TestANamelessPartyMemberMadeACreatureInSightIsHeldAndSentWithoutItsList(t *testing.T) {
	tb := namelessIn(t, sheetParty, "x", "y")
	if ids := heldIDs(tb.v.shown, "who"); !sameIDSet(ids, []string{"x", "y"}) {
		t.Fatalf("fixture: the log holds the nameless party member naming %q, want [x y]", ids)
	}
	var all []*vttv1.Envelope
	for _, p := range []proto.Message{
		&vttv1.ActorControlGranted{ActorId: "who", ParticipantId: "p-3", Kind: vttv1.ActorKind_ACTOR_KIND_NON_PARTY},
		&vttv1.TokenMoved{TokenId: "t-who", SceneId: "s", From: &vttv1.GridPosition{X: 2, Y: 1}, To: &vttv1.GridPosition{X: 4, Y: 1}},
		&vttv1.TokenMoved{TokenId: "t-who", SceneId: "s", From: &vttv1.GridPosition{X: 4, Y: 1}, To: &vttv1.GridPosition{X: 2, Y: 1}},
		&vttv1.NarrationAdded{Text: "seen"},
	} {
		all = append(all, tb.step(p)...)
		if ids := heldIDs(tb.v.shown, "who"); len(ids) != 0 {
			t.Errorf("after %T the log holds the creature naming %q", p, ids)
		}
	}
	if got := abilityChanges(all, "who"); len(got) != 0 {
		t.Errorf("the log was sent the creature's %q", got)
	}
}

// VTT-738
func TestANamelessCreatureMadeAPartyMemberInSightIsHeldWithItsList(t *testing.T) {
	tb := namelessIn(t, vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "k", "l")
	if ids := heldIDs(tb.v.shown, "who"); len(ids) != 0 {
		t.Fatalf("fixture: the log holds the nameless creature naming %q", ids)
	}
	for _, p := range []proto.Message{
		&vttv1.ActorControlGranted{ActorId: "who", ParticipantId: "p-3", Kind: sheetParty},
		&vttv1.NarrationAdded{Text: "seen"},
	} {
		tb.step(p)
		if ids := heldIDs(tb.v.shown, "who"); !sameIDSet(ids, []string{"k", "l"}) {
			t.Errorf("after %T the log holds the party member naming %q, want [k l]", p, ids)
		}
	}
}

// VTT-738 VTT-741
func TestANamelessCreatureMadeAPartyMemberInSightIsForwardedATakeThatFolds(t *testing.T) {
	tb := namelessIn(t, vttv1.ActorKind_ACTOR_KIND_NON_PARTY, "k", "l")
	tb.step(&vttv1.ActorControlGranted{ActorId: "who", ParticipantId: "p-3", Kind: sheetParty})
	out := tb.step(&vttv1.AbilityTaken{ActorId: "who", AbilityId: "k"})
	if got := abilityChanges(out, "who"); !sameStrings(got, []string{"taken k"}) {
		t.Errorf("the take in sight sent %q, want it forwarded", got)
	}
	if ids := heldIDs(tb.v.shown, "who"); !sameIDSet(ids, []string{"l"}) {
		t.Errorf("after the take the log holds the party member naming %q, want [l]", ids)
	}
}

// VTT-755
func TestANamelessActorWithNoListChangingKindInSightIsForwardedTheGrantAlone(t *testing.T) {
	for from, to := range map[vttv1.ActorKind]vttv1.ActorKind{
		sheetParty: vttv1.ActorKind_ACTOR_KIND_NON_PARTY, vttv1.ActorKind_ACTOR_KIND_NON_PARTY: sheetParty,
	} {
		tb := namelessIn(t, from)
		out := tb.step(&vttv1.ActorControlGranted{ActorId: "who", ParticipantId: "p-3", Kind: to})
		if got, want := shapes(out), []string{"*vttv1.Envelope_ActorControlGranted who"}; !slices.Equal(got, want) {
			t.Errorf("%v made %v in sight sent %v, want the grant alone %v", from, to, got, want)
		}
		if len(out) == 1 && out[0].GetEventId() != fmt.Sprintf("evt-%d", tb.seq) {
			t.Errorf("%v made %v in sight sent a built grant, event id %q", from, to, out[0].GetEventId())
		}
		if got := tb.v.shown.Actors["who"].GetKind(); got != to {
			t.Errorf("%v made %v in sight is held as %v", from, to, got)
		}
	}
}
