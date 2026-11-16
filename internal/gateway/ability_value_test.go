package gateway_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

func giveWith(actorID, abilityID string, resources map[string]*vttv1.Resource) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_GiveAbility{
		GiveAbility: &vttv1.GiveAbility{ActorId: actorID, AbilityId: abilityID, Resources: resources}}}
}

// poolRuleset is tavern-brawl with a second ability, haymaker, spending
// stamina as chair-swing does: no served ruleset has two abilities sharing one.
func poolRuleset(t *testing.T) *rules.Ruleset {
	t.Helper()
	dir := t.TempDir()
	src := tavernBrawlDir(t)
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), b, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
	swing, err := os.ReadFile(filepath.Join(src, "abilities", "chair-swing.json"))
	if err != nil {
		t.Fatal(err)
	}
	haymaker := strings.Replace(strings.Replace(string(swing), `"chair-swing"`, `"haymaker"`, 1), `"Chair Swing"`, `"Haymaker"`, 1)
	if err := os.WriteFile(filepath.Join(dir, "abilities", "haymaker.json"), []byte(haymaker), 0o600); err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Load(dir)
	if err != nil {
		t.Fatalf("rules.Load(pool ruleset): %v", err)
	}
	return rs
}

// addTank adds tank, standing beside the patron, naming abilities and holding
// stamina at 2 of 2.
func addTank(t *testing.T, f *rulesetFixture, abilities ...string) {
	t.Helper()
	dm := f.dial(f.dmToken, 0)
	for _, c := range []*vttv1.ClientCommand{
		{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: &vttv1.Actor{
			ActorId: "tank", Name: "Tank", Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER,
			Attributes: map[string]int32{"brawn": 2},
			Resources:  map[string]*vttv1.Resource{"stamina": {Current: 2, Max: 2}},
			AbilityIds: abilities}}}},
		{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
			TokenId: "tok-tank", SceneId: "tavern", ActorId: "tank", Position: &vttv1.GridPosition{X: 2, Y: 0}}}},
	} {
		sendCommand(t, dm, c)
		if r := readResult(t, dm); !r.GetOk() {
			t.Fatalf("fixture: %v answered %+v", c.GetCommand(), r)
		}
	}
}

// refusedAppendingNothing sends c as the DM and wants a refusal holding every
// one of want, with the log's head where it was.
func refusedAppendingNothing(t *testing.T, f *rulesetFixture, c *vttv1.ClientCommand, want ...string) {
	t.Helper()
	head := f.abilityHead(t)
	conn := f.dial(f.dmToken, 0)
	sendCommand(t, conn, c)
	res := readResult(t, conn)
	if res.GetOk() {
		t.Fatalf("%v answered ok at %d, want a refusal", c.GetCommand(), res.GetSequence())
	}
	for _, w := range want {
		if !strings.Contains(res.GetError(), w) {
			t.Errorf("%v was refused with %q, which does not name %q", c.GetCommand(), res.GetError(), w)
		}
	}
	if got := f.abilityHead(t); got != head {
		t.Errorf("%v moved the log's head from %d to %d", c.GetCommand(), head, got)
	}
}

// acceptedAppending sends c as the DM and wants it ok, appending exactly n
// events, and returns the sequence of the first.
func acceptedAppending(t *testing.T, f *rulesetFixture, c *vttv1.ClientCommand, n int64) int64 {
	t.Helper()
	head := f.abilityHead(t)
	conn := f.dial(f.dmToken, 0)
	sendCommand(t, conn, c)
	res := readResult(t, conn)
	if !res.GetOk() || res.GetSequence() != head+1 {
		t.Fatalf("%v answered %+v, want ok at %d", c.GetCommand(), res, head+1)
	}
	if got := f.abilityHead(t); got != head+n {
		t.Errorf("%v moved the log's head from %d to %d, want %d events", c.GetCommand(), head, got, n)
	}
	return res.GetSequence()
}

// VTT-760
func TestAGiveOfALimitedAbilityCarryingItsResourceAppendsTheGiveThenTheResource(t *testing.T) {
	for _, value := range []*vttv1.Resource{{Current: 1, Max: 2}, {Current: 0, Max: 2}} {
		f := newRulesetFixture(t, true)
		rookieAt(t, f)
		seq := acceptedAppending(t, f, giveWith("rookie", "chair-swing", map[string]*vttv1.Resource{"stamina": value}), 2)
		if g := f.abilityEventAt(t, seq).GetAbilityGiven(); g.GetActorId() != "rookie" || g.GetAbilityId() != "chair-swing" {
			t.Fatalf("the give's first event is %v, want the AbilityGiven of chair-swing to rookie", g)
		}
		ra := f.abilityEventAt(t, seq+1).GetResourceAdded()
		if ra.GetActorId() != "rookie" || ra.GetResource() != "stamina" ||
			ra.GetValue().GetCurrent() != value.GetCurrent() || ra.GetValue().GetMax() != value.GetMax() {
			t.Fatalf("the give's second event is %v, want stamina added to rookie at %v", ra, value)
		}
		c := f.dial(f.dmToken, 0)
		sendCommand(t, c, sheetUse("rookie", "chair-swing", "patron"))
		r := readResult(t, c)
		if value.GetCurrent() > 0 && !r.GetOk() {
			t.Errorf("a use of chair-swing given with stamina %v answered %+v, want it resolved", value, r)
		}
		if value.GetCurrent() == 0 && (r.GetOk() || !strings.Contains(r.GetError(), "insufficient")) {
			t.Errorf("a use of chair-swing given with stamina at 0 answered %+v, want it refused as too few", r)
		}
	}
}

// VTT-761 VTT-800
func TestAGiveOfALimitedAbilityWithoutItsResourceIsRefused(t *testing.T) {
	f := newRulesetFixture(t, true)
	rookieAt(t, f)
	refusedAppendingNothing(t, f, giveAbilityCmd("rookie", "chair-swing"),
		`gateway: give_ability: actor "rookie": resources: `, `"chair-swing"`, `"stamina"`)
}

// VTT-762 VTT-764 VTT-800
func TestAGiveCarryingAResourceTheSheetHoldsIsRefusedAndOneCarryingNoneIsTheGiveAlone(t *testing.T) {
	f := newRulesetFixtureServing(t, poolRuleset(t))
	addTank(t, f, "chair-swing")
	refusedAppendingNothing(t, f,
		giveWith("tank", "haymaker", map[string]*vttv1.Resource{"stamina": {Current: 5, Max: 5}}),
		`gateway: give_ability: actor "tank": resources: `, `"stamina"`)
	seq := acceptedAppending(t, f, giveAbilityCmd("tank", "haymaker"), 1)
	if g := f.abilityEventAt(t, seq).GetAbilityGiven(); g.GetAbilityId() != "haymaker" {
		t.Fatalf("the give appended %v, want the AbilityGiven of haymaker", g)
	}
}

// VTT-763 VTT-800
func TestAGiveCarryingAKeyTheAbilityDoesNotSpendIsRefused(t *testing.T) {
	f := newRulesetFixture(t, true)
	rookieAt(t, f)
	for _, c := range []struct {
		ability string
		keys    map[string]*vttv1.Resource
		key     string
	}{
		{"fists", map[string]*vttv1.Resource{"stamina": {Current: 1}}, "stamina"},
		{"chair-swing", map[string]*vttv1.Resource{"drink": {Current: 1}}, "drink"},
		{"chair-swing", map[string]*vttv1.Resource{"stamina": {Current: 1}, "drink": {Current: 1}}, "drink"},
	} {
		refusedAppendingNothing(t, f, giveWith("rookie", c.ability, c.keys),
			`gateway: give_ability: actor "rookie": resources: `, `"`+c.ability+`"`, `"`+c.key+`"`)
	}
}

// VTT-764
func TestAGiveOfAnAtWillAbilityCarryingNothingAppendsTheGiveAlone(t *testing.T) {
	f := newRulesetFixture(t, true)
	rookieAt(t, f)
	acceptedAppending(t, f, giveAbilityCmd("rookie", "fists"), 1)
}

// VTT-765
func TestWithNoRulesetAGiveCarryingResourcesIsRefused(t *testing.T) {
	f := newRulesetFixture(t, false)
	rookieAt(t, f)
	refusedAppendingNothing(t, f, giveWith("rookie", "chair-swing", map[string]*vttv1.Resource{"stamina": {Current: 1}}),
		`gateway: give_ability: actor "rookie": resources: `, `"stamina"`)
}

// VTT-767 VTT-799
func TestATakeRemovesTheResourceTheTakenAbilityAloneSpends(t *testing.T) {
	f := newRulesetFixture(t, true)
	addTank(t, f, "chair-swing", "fists")
	seq := acceptedAppending(t, f, takeAbilityCmd("tank", "fists"), 1)
	if tk := f.abilityEventAt(t, seq).GetAbilityTaken(); tk.GetAbilityId() != "fists" {
		t.Fatalf("the take of fists appended %v", tk)
	}
	seq = acceptedAppending(t, f, takeAbilityCmd("tank", "chair-swing"), 2)
	if tk := f.abilityEventAt(t, seq).GetAbilityTaken(); tk.GetActorId() != "tank" || tk.GetAbilityId() != "chair-swing" {
		t.Fatalf("the take's first event is %v, want the AbilityTaken of chair-swing from tank", tk)
	}
	if rr := f.abilityEventAt(t, seq+1).GetResourceRemoved(); rr.GetActorId() != "tank" || rr.GetResource() != "stamina" {
		t.Fatalf("the take's second event is %v, want stamina removed from tank", rr)
	}
}

// VTT-767
func TestATakeKeepsAResourceAnotherAbilityOnTheSheetSpends(t *testing.T) {
	f := newRulesetFixtureServing(t, poolRuleset(t))
	addTank(t, f, "chair-swing", "haymaker")
	acceptedAppending(t, f, takeAbilityCmd("tank", "chair-swing"), 1)
	seq := acceptedAppending(t, f, takeAbilityCmd("tank", "haymaker"), 2)
	if rr := f.abilityEventAt(t, seq+1).GetResourceRemoved(); rr.GetResource() != "stamina" {
		t.Fatalf("the take of the last ability spending stamina appended %v second, want stamina removed", rr)
	}
}

// VTT-767: drink is spent by no ability of the ruleset.
func TestATakeRemovesNoResourceNoAbilitySpends(t *testing.T) {
	f := newRulesetFixture(t, true)
	acceptedAppending(t, f, giveAbilityCmd("patron", "fists"), 1)
	acceptedAppending(t, f, takeAbilityCmd("patron", "fists"), 1)
}

// VTT-768
func TestWithNoRulesetATakeRemovesNoResource(t *testing.T) {
	f := newRulesetFixture(t, false)
	addTank(t, f, "chair-swing")
	acceptedAppending(t, f, takeAbilityCmd("tank", "chair-swing"), 1)
}
