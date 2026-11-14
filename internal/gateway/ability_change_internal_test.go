package gateway

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func sheetGive(actorID, abilityID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "give", Command: &vttv1.ClientCommand_GiveAbility{
		GiveAbility: &vttv1.GiveAbility{ActorId: actorID, AbilityId: abilityID}}}
}

func sheetTake(actorID, abilityID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{RequestId: "take", Command: &vttv1.ClientCommand_TakeAbility{
		TakeAbility: &vttv1.TakeAbility{ActorId: actorID, AbilityId: abilityID}}}
}

func appendedAt(t *testing.T, camp *campaign.Campaign, seq int64) *vttv1.Envelope {
	t.Helper()
	events, unsubscribe, head, err := camp.Subscribe(seq-1, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer unsubscribe()
	if head != seq {
		t.Fatalf("the log's head is %d, want %d: the command appended other than one event", head, seq)
	}
	return <-events
}

func sameStateBut(t *testing.T, before, after *engine.State, id string, want []string) {
	t.Helper()
	for name, pair := range map[string][2]any{
		"scenes": {before.Scenes, after.Scenes}, "tokens": {before.Tokens, after.Tokens},
		"sessions": {before.Sessions, after.Sessions}, "conditions": {before.Conditions, after.Conditions},
		"notes": {before.Notes, after.Notes},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("the state's %s changed: %v, then %v", name, pair[0], pair[1])
		}
	}
	if ids, now := slices.Sorted(maps.Keys(before.Actors)), slices.Sorted(maps.Keys(after.Actors)); !slices.Equal(ids, now) {
		t.Errorf("the state's actors changed: %v, then %v", ids, now)
	}
	for aid, a := range before.Actors {
		wantActor := proto.Clone(a).(*vttv1.Actor)
		if aid == id {
			wantActor.AbilityIds = want
		}
		if !proto.Equal(after.Actors[aid], wantActor) {
			t.Errorf("actor %q is %v, want %v", aid, after.Actors[aid], wantActor)
		}
	}
}

func abilityChangeServer(t *testing.T) (*Server, *campaign.Campaign, *identity.Participant) {
	t.Helper()
	s, camp, dm := fitServer(t, abilityFitRuleset("fists", "rally", "chair-swing"))
	for _, cmd := range []*vttv1.ClientCommand{
		abilityAddActor("fit", map[string]int32{"brace": 2}, "rally", "fists", "rally"),
		abilityAddActor("other", nil, "fists"),
	} {
		if r := s.handleCommand(dm, cmd, nil); !r.GetOk() {
			t.Fatalf("fixture: %s", r.GetError())
		}
	}
	return s, camp, dm
}

// VTT-713
func TestAGiveAppendsOneAbilityGivenAndChangesOnlyTheActorsList(t *testing.T) {
	s, camp, dm := abilityChangeServer(t)
	before := camp.State().Snapshot()
	r := s.handleCommand(dm, sheetGive("fit", "chair-swing"), nil)
	if !r.GetOk() {
		t.Fatalf("give_ability refused: %s", r.GetError())
	}
	g := appendedAt(t, camp, r.GetSequence()).GetAbilityGiven()
	if g.GetActorId() != "fit" || g.GetAbilityId() != "chair-swing" {
		t.Fatalf("the give appended %+v, want an AbilityGiven of chair-swing to fit", g)
	}
	sameStateBut(t, before, camp.State(), "fit", []string{"rally", "fists", "rally", "chair-swing"})
}

// VTT-714
func TestATakeAppendsOneAbilityTakenAndLeavesNoOccurrenceTheOthersInOrder(t *testing.T) {
	s, camp, dm := abilityChangeServer(t)
	if r := s.handleCommand(dm, sheetGive("fit", "chair-swing"), nil); !r.GetOk() {
		t.Fatalf("give_ability refused: %s", r.GetError())
	}
	before := camp.State().Snapshot()
	r := s.handleCommand(dm, sheetTake("fit", "rally"), nil)
	if !r.GetOk() {
		t.Fatalf("take_ability refused: %s", r.GetError())
	}
	tk := appendedAt(t, camp, r.GetSequence()).GetAbilityTaken()
	if tk.GetActorId() != "fit" || tk.GetAbilityId() != "rally" {
		t.Fatalf("the take appended %+v, want an AbilityTaken of rally from fit", tk)
	}
	sameStateBut(t, before, camp.State(), "fit", []string{"fists", "chair-swing"})
}

// VTT-747
func TestAddActorRefusesAnEmptyAbilityIdWithOrWithoutARuleset(t *testing.T) {
	for name, rs := range map[string]*rules.Ruleset{"no ruleset": nil, "a ruleset": abilityFitRuleset("fists")} {
		t.Run(name, func(t *testing.T) {
			s, camp, dm := fitServer(t, rs)
			r := s.handleCommand(dm, abilityAddActor("fit", nil, "fists", ""), nil)
			want := `gateway: add_actor: actor "fit": ability_ids: must not hold an empty id`
			if r.GetOk() || r.GetError() != want {
				t.Errorf("add_actor answered %+v, want %q", r, want)
			}
			if _, added := camp.State().Actors["fit"]; added {
				t.Error("the refused actor was appended")
			}
		})
	}
}

// VTT-747
func TestAddActorReadsAnEmptyAbilityIdAfterItsKeysAndBeforeTheRuleset(t *testing.T) {
	s, _, dm := fitServer(t, abilityFitRuleset("fists"))
	for _, c := range []struct {
		name string
		cmd  *vttv1.ClientCommand
		want string
	}{
		{"a controller", func() *vttv1.ClientCommand {
			cmd := abilityAddActor("fit", nil, "")
			cmd.GetAddActor().GetActor().ControllerId = "someone"
			return cmd
		}(), "gateway: add_actor: controller"},
		{"an undeclared key", abilityAddActor("fit", map[string]int32{"nope": 1}, ""),
			`gateway: add_actor: actor "fit": attributes: references undeclared property "nope"`},
		{"an id the ruleset does not hold", abilityAddActor("fit", nil, "nope", ""),
			`gateway: add_actor: actor "fit": ability_ids: must not hold an empty id`},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := s.handleCommand(dm, c.cmd, nil)
			if r.GetOk() || !strings.HasPrefix(r.GetError(), c.want) {
				t.Errorf("add_actor answered %+v, want an error starting %q", r, c.want)
			}
		})
	}
}
