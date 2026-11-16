package gateway_test

import (
	"strings"
	"testing"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

func giveAbilityCmd(actorID, abilityID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GiveAbility{
		GiveAbility: &vttv1.GiveAbility{ActorId: actorID, AbilityId: abilityID}}}
}

func takeAbilityCmd(actorID, abilityID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_TakeAbility{
		TakeAbility: &vttv1.TakeAbility{ActorId: actorID, AbilityId: abilityID}}}
}

func (f *rulesetFixture) abilityHead(t *testing.T) int64 {
	t.Helper()
	c := f.dial(f.dmToken, 0)
	defer c.Close(websocket.StatusNormalClosure, "")
	return expectCatchUpHead(t, c)
}

func (f *rulesetFixture) abilityEventAt(t *testing.T, seq int64) *vttv1.Envelope {
	t.Helper()
	env := readEvent(t, f.dial(f.dmToken, seq-1))
	if env.GetSequence() != seq {
		t.Fatalf("the DM's first event after %d is at %d", seq-1, env.GetSequence())
	}
	return env
}

func (f *rulesetFixture) tokenFor(role string) string {
	return map[string]string{"dm": f.dmToken, "agent": f.agentToken, "player": f.patronToken,
		"brawler": f.brawlerToken, "spectator": f.spectatorToken}[role]
}

// Keep rookie naming no ability until the give: the give must be the only
// reason its use resolves.
// VTT-764
func TestAGiveThenAUseResolvesForTheDMTheAgentAndThePlayer(t *testing.T) {
	for _, giver := range []string{"dm", "agent"} {
		t.Run(giver, func(t *testing.T) {
			f := newRulesetFixture(t, true)
			rookieAt(t, f)
			conn := f.dial(f.tokenFor(giver), 0)
			sendCommand(t, conn, giveAbilityCmd("rookie", "fists"))
			res := readResult(t, conn)
			if !res.GetOk() || res.GetSequence() != rfSeedHead+4 {
				t.Fatalf("the %s's give answered %+v, want ok at sequence %d", giver, res, rfSeedHead+4)
			}
			given := f.abilityEventAt(t, res.GetSequence()).GetAbilityGiven()
			if given.GetActorId() != "rookie" || given.GetAbilityId() != "fists" {
				t.Fatalf("the give appended %+v, want an AbilityGiven of fists to rookie", given)
			}
			for _, user := range []string{"dm", "agent", "player"} {
				c := f.dial(f.tokenFor(user), 0)
				sendCommand(t, c, sheetUse("rookie", "fists", "patron"))
				if r := readResult(t, c); !r.GetOk() {
					t.Errorf("the %s's use of the given ability answered %+v, want it resolved", user, r)
				}
			}
		})
	}
}

// VTT-767
func TestATakeThenAUseIsRefusedForTheDMTheAgentAndThePlayer(t *testing.T) {
	for _, taker := range []string{"dm", "agent"} {
		t.Run(taker, func(t *testing.T) {
			f := newRulesetFixture(t, true)
			conn := f.dial(f.tokenFor(taker), 0)
			sendCommand(t, conn, takeAbilityCmd("brawler", "fists"))
			res := readResult(t, conn)
			if !res.GetOk() || res.GetSequence() != rfSeedHead+1 {
				t.Fatalf("the %s's take answered %+v, want ok at sequence %d", taker, res, rfSeedHead+1)
			}
			taken := f.abilityEventAt(t, res.GetSequence()).GetAbilityTaken()
			if taken.GetActorId() != "brawler" || taken.GetAbilityId() != "fists" {
				t.Fatalf("the take appended %+v, want an AbilityTaken of fists from brawler", taken)
			}
			want := `gateway: use_ability: actor "brawler" does not name ability "fists"`
			for _, user := range []string{"dm", "agent", "brawler"} {
				c := f.dial(f.tokenFor(user), 0)
				sendCommand(t, c, sheetUse("brawler", "fists", "patron"))
				if r := readResult(t, c); r.GetOk() || r.GetError() != want {
					t.Errorf("the %s's use of the taken ability answered %+v, want the refusal %q", user, r, want)
				}
			}
			if head := f.abilityHead(t); head != rfSeedHead+1 {
				t.Errorf("log head %d after the refused uses, want %d", head, rfSeedHead+1)
			}
		})
	}
}

// Keep each command on the brawler player's own party member: the role table
// must be the only reason that player is refused.
// VTT-716
func TestAGiveOrATakeFromAPlayerOrASpectatorIsRefusedAndAppendsNothing(t *testing.T) {
	f := newRulesetFixture(t, true)
	for _, who := range []string{"brawler", "spectator"} {
		for _, cmd := range []*vttv1.ClientCommand{giveAbilityCmd("brawler", "chair-swing"), takeAbilityCmd("brawler", "fists")} {
			c := f.dial(f.tokenFor(who), 0)
			sendCommand(t, c, cmd)
			if r := readResult(t, c); r.GetOk() || !strings.Contains(r.GetError(), "not authorized") {
				t.Errorf("the %s's %T answered %+v, want it refused as not authorized", who, cmd.GetCommand(), r)
			}
		}
	}
	if head := f.abilityHead(t); head != rfSeedHead {
		t.Errorf("log head %d after the refused commands, want %d", head, rfSeedHead)
	}
}

// VTT-718 VTT-800
func TestWithARulesetAGiveOfAnIdItDoesNotHoldIsRefusedNamingTheActorAndTheId(t *testing.T) {
	f := newRulesetFixture(t, true)
	for _, c := range []struct{ giver, actor, id string }{
		{"dm", "patron", "no-such-move"},
		{"agent", "patron", "no-such-move"},
		{"dm", "patron", ""},
		{"dm", "ghost", "no-such-move"},
	} {
		conn := f.dial(f.tokenFor(c.giver), 0)
		sendCommand(t, conn, giveAbilityCmd(c.actor, c.id))
		want := `gateway: give_ability: actor "` + c.actor + `": ability_ids: names ability "` + c.id +
			`", which the ruleset does not hold`
		if r := readResult(t, conn); r.GetOk() || r.GetError() != want {
			t.Errorf("the %s's give of %q to %s answered %+v, want %q", c.giver, c.id, c.actor, r, want)
		}
	}
	if head := f.abilityHead(t); head != rfSeedHead {
		t.Errorf("log head %d after the refused gives, want %d", head, rfSeedHead)
	}
}

// VTT-717 VTT-720 VTT-721
func TestAGiveOrATakeTheFoldRefusesReachesTheIssuerAndAppendsNothing(t *testing.T) {
	f := newRulesetFixture(t, true)
	dm := f.dial(f.dmToken, 0)
	for _, c := range []struct {
		cmd  *vttv1.ClientCommand
		want string
	}{
		{giveAbilityCmd("ghost", "fists"), `engine: ability_given names unknown actor "ghost"`},
		{takeAbilityCmd("ghost", "fists"), `engine: ability_taken names unknown actor "ghost"`},
		{giveAbilityCmd("brawler", "fists"), `engine: actor "brawler" already names ability "fists"`},
		{takeAbilityCmd("brawler", "chair-swing"), `engine: actor "brawler" does not name ability "chair-swing"`},
		{takeAbilityCmd("patron", "fists"), `engine: actor "patron" does not name ability "fists"`},
	} {
		sendCommand(t, dm, c.cmd)
		if r := readResult(t, dm); r.GetOk() || r.GetError() != c.want {
			t.Errorf("%v answered %+v, want %q", c.cmd.GetCommand(), r, c.want)
		}
	}
	if head := f.abilityHead(t); head != rfSeedHead {
		t.Errorf("log head %d after the refused commands, want %d", head, rfSeedHead)
	}
}

// VTT-722 VTT-766
func TestWithNoRulesetAGiveOfAnyNonEmptyIdIsAcceptedAndAnEmptyOneRefused(t *testing.T) {
	f := newRulesetFixture(t, false)
	dm := f.dial(f.dmToken, 0)
	sendCommand(t, dm, giveAbilityCmd("patron", "no-such-move"))
	res := readResult(t, dm)
	if !res.GetOk() || res.GetSequence() != rfSeedHead+1 {
		t.Fatalf("a give with no ruleset served answered %+v, want ok at sequence %d", res, rfSeedHead+1)
	}
	given := f.abilityEventAt(t, res.GetSequence()).GetAbilityGiven()
	if given.GetActorId() != "patron" || given.GetAbilityId() != "no-such-move" {
		t.Fatalf("the give appended %+v, want an AbilityGiven of no-such-move to patron", given)
	}
	sendCommand(t, dm, giveAbilityCmd("patron", ""))
	if r, want := readResult(t, dm), "engine: ability_given requires an ability id"; r.GetOk() || r.GetError() != want {
		t.Errorf("a give of an empty id answered %+v, want %q", r, want)
	}
	if head := f.abilityHead(t); head != rfSeedHead+1 {
		t.Errorf("log head %d after the refused give, want %d", head, rfSeedHead+1)
	}
}
