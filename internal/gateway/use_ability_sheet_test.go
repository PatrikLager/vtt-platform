package gateway_test

import (
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func sheetUse(caster, ability, target string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{
		UseAbility: &vttv1.UseAbility{ActorId: caster, AbilityId: ability, TargetIds: []string{target}}}}
}

// Keep rookie holding the brawn fists rolls with and naming no ability: the
// sheet must be the only reason its use is refused.
func rookieAt(t *testing.T, f *rulesetFixture) {
	t.Helper()
	dm := f.dial(f.dmToken, 0)
	for _, c := range []*vttv1.ClientCommand{
		{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{
			ActorId: "rookie", Name: "Rookie", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Attributes: map[string]int32{"brawn": 3}}}}},
		{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "tok-rookie", SceneId: "tavern", ActorId: "rookie", Position: &vttv1.GridPosition{X: 1, Y: 1}}}},
		{Command: &vttv1.ClientCommand_GrantActorControl{GrantActorControl: &vttv1.GrantActorControl{
			ActorId: "rookie", ParticipantId: f.patronID, Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER}}},
	} {
		sendCommand(t, dm, c)
		if r := readResult(t, dm); !r.Ok {
			t.Fatalf("fixture: %v answered %+v", c.GetCommand(), r)
		}
	}
}

// Keep the probe after the refusal: its sequence is how the test sees that the
// refused use appended nothing.
// VTT-690
func TestAUseTheCastersSheetDoesNotNameIsRefusedForEveryRole(t *testing.T) {
	for _, role := range []string{"dm", "agent", "player"} {
		t.Run(role, func(t *testing.T) {
			f := newRulesetFixture(t, true)
			rookieAt(t, f)
			token := map[string]string{"dm": f.dmToken, "agent": f.agentToken, "player": f.patronToken}[role]
			conn := f.dial(token, 0)
			sendCommand(t, conn, sheetUse("rookie", "fists", "patron"))
			res := readResult(t, conn)
			want := `gateway: use_ability: actor "rookie" does not name ability "fists"`
			if res.Ok || res.Error != want {
				t.Fatalf("the %s's use the sheet does not name answered %+v, want the refusal %q", role, res, want)
			}
			sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{
				MoveToken: &vttv1.MoveTokenRequest{TokenId: "tok-rookie", To: &vttv1.GridPosition{X: 2, Y: 1}}}})
			if r := readResult(t, conn); !r.Ok || r.Sequence != rfSeedHead+4 {
				t.Errorf("the next command answered %+v, want ok at sequence %d: the refused use appended", r, rfSeedHead+4)
			}
		})
	}
}

// VTT-690
func TestAUseByAnActorTheCampaignDoesNotHoldIsLeftToResolve(t *testing.T) {
	f := newRulesetFixture(t, true)
	conn := f.dial(f.dmToken, 0)
	sendCommand(t, conn, sheetUse("ghost", "fists", "patron"))
	res := readResult(t, conn)
	if res.Ok || !strings.Contains(res.Error, `unknown actor "ghost"`) {
		t.Fatalf("a use by an actor the campaign does not hold answered %+v, want Resolve's unknown actor", res)
	}
}
