package gateway_test

// ruleset_test.go covers the use_ability/remove_condition wiring itself
// (ruleset-interpreter Task 6): rules.Resolve -> campaign.AppendBatch, the
// "no ruleset loaded" clean error, Resolve's own validation errors
// surfacing as ok=false, and the whole batch reaching every connected
// participant contiguously. Built against the REAL committed tavern-brawl
// ruleset (internal/rules/conformance's own P4 proof already covers
// Resolve's game-rules correctness in isolation) — this file proves the
// WIRING, not the rules engine.

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// tavernBrawlDir resolves the committed rulesets/tavern-brawl directory
// relative to this test file's own package directory (internal/gateway is
// two levels below the repo root — the same "../../<dir>" convention
// internal/rules/conformance/conformance_test.go and cmd/vtt's own test
// files already use for their own repo-root-relative fixtures, just one
// level shallower here since internal/gateway sits one directory closer to
// the root than internal/rules/conformance does).
func tavernBrawlDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "rulesets", "tavern-brawl")
}

func loadTavernBrawl(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(tavernBrawlDir(t))
	if err != nil {
		t.Fatalf("rules.Load(tavern-brawl): %v", err)
	}
	return rs
}

// newRulesetFixture is newGWFixture's sibling (server_test.go's fixture
// shape reused as directly as this package's unexported helpers allow):
// seeds a session/scene, a "brawler" actor (controlled by the returned
// player token, brawn=3) and a "patron" actor (footing=0, so ANY fists
// attack roll — even the crypto Roller's minimum 1d20=1 — is guaranteed to
// hit: total = 1d20 + 3 >= 4 > 0 always), tokens for both one grid cell
// apart (within fists' range=1), and returns a gateway.Server WITH the
// tavern-brawl ruleset loaded via WithRuleset — the one gwFixture (server_
// test.go) deliberately does NOT configure.
type rulesetFixture struct {
	t   *testing.T
	srv *httptest.Server

	dmToken                 string
	brawlerToken, brawlerID string
	patronToken, patronID   string
	spectatorToken          string
}

func newRulesetFixture(t *testing.T, withRuleset bool) *rulesetFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	ids, err := identity.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ids.Close() })

	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
	if err != nil {
		t.Fatal(err)
	}
	brawlerToken, brawlerID, err := ids.CreateInvite("Brawler", identity.RolePlayer, []string{"brawler"})
	if err != nil {
		t.Fatal(err)
	}
	patronToken, patronID, err := ids.CreateInvite("Patron", identity.RolePlayer, []string{"patron"})
	if err != nil {
		t.Fatal(err)
	}
	spectatorToken, _, err := ids.CreateInvite("Watcher", identity.RoleSpectator, nil)
	if err != nil {
		t.Fatal(err)
	}

	mustAppend(t, c, "rf-seed-1", &vttv1.Envelope_SessionStarted{SessionStarted: &vttv1.SessionStarted{Name: "brawl"}})
	mustAppend(t, c, "rf-seed-2", &vttv1.Envelope_SceneCreated{SceneCreated: &vttv1.SceneCreated{
		SceneId: "tavern", Name: "Tavern", GridWidth: 5, GridHeight: 5,
	}})
	mustAppend(t, c, "rf-seed-3", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{
			ActorId: "brawler", Name: "Brawler", ControllerId: brawlerID,
			Attributes: map[string]int32{"brawn": 3, "grit": 1},
		},
	}})
	mustAppend(t, c, "rf-seed-4", &vttv1.Envelope_ActorAdded{ActorAdded: &vttv1.ActorAdded{
		Actor: &vttv1.Actor{
			ActorId: "patron", Name: "Patron", ControllerId: patronID,
			Attributes: map[string]int32{"footing": 0},
			Resources:  map[string]*vttv1.Resource{"drink": {Current: 0, Max: 5}},
		},
	}})
	mustAppend(t, c, "rf-seed-5", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "tok-brawler", SceneId: "tavern", ActorId: "brawler", Position: &vttv1.GridPosition{X: 0, Y: 0},
	}})
	mustAppend(t, c, "rf-seed-6", &vttv1.Envelope_TokenPlaced{TokenPlaced: &vttv1.TokenPlaced{
		TokenId: "tok-patron", SceneId: "tavern", ActorId: "patron", Position: &vttv1.GridPosition{X: 1, Y: 0},
	}})

	srv := gateway.New(c, ids)
	if withRuleset {
		srv = srv.WithRuleset(loadTavernBrawl(t))
	}
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &rulesetFixture{
		t: t, srv: httpSrv,
		dmToken:      dmToken,
		brawlerToken: brawlerToken, brawlerID: brawlerID,
		patronToken: patronToken, patronID: patronID,
		spectatorToken: spectatorToken,
	}
}

// wsURL/dial mirror gwFixture's own (server_test.go) byte-for-byte — kept
// as this fixture's own small methods rather than sharing gwFixture's
// (different receiver type) or hoisting them to free functions (would
// touch server_test.go's already-reviewed fixture for no behavioral gain).
func (f *rulesetFixture) wsURL(token string, after int64) string {
	u, err := url.Parse(f.srv.URL)
	if err != nil {
		f.t.Fatal(err)
	}
	u.Path = "/ws"
	q := u.Query()
	q.Set("token", token)
	q.Set("after", strconv.FormatInt(after, 10))
	u.RawQuery = q.Encode()
	return u.String()
}

func (f *rulesetFixture) dial(token string, after int64) *websocket.Conn {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, f.wsURL(token, after), nil)
	if err != nil {
		f.t.Fatalf("dial: %v", err)
	}
	f.t.Cleanup(func() { conn.CloseNow() })
	return conn
}

// fistsCmd builds a UseAbility ClientCommand: casterID's fists, targeting
// targetID.
func fistsCmd(casterID, targetID string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{
		UseAbility: &vttv1.UseAbility{ActorId: casterID, AbilityId: "fists", TargetIds: []string{targetID}},
	}}
}

// --- tests -----------------------------------------------------------------

// TestUseAbilityNoRulesetLoadedCleanError covers spec §7's binding: serving
// without --ruleset keeps every other command working; a use_ability
// command specifically gets a clean ok=false CommandResult naming "no
// ruleset loaded" — never a connection drop, crash, or protocol error. The
// connection stays usable afterward (a follow-up command still works).
func TestUseAbilityNoRulesetLoadedCleanError(t *testing.T) {
	f := newRulesetFixture(t, false) // withRuleset=false
	conn := f.dial(f.brawlerToken, 0)

	sendCommand(t, conn, fistsCmd("brawler", "patron"))
	res := readResult(t, conn)
	if res.Ok {
		t.Fatalf("want ok=false with no ruleset loaded, got %+v", res)
	}
	if !strings.Contains(res.Error, "no ruleset loaded") {
		t.Fatalf("error = %q, want it to contain %q", res.Error, "no ruleset loaded")
	}

	// Connection intact: an ordinary command this same player IS authorized
	// for still works (moving their own token — end_session is dm/agent
	// only, so it is not a fair "still works" probe for a player token).
	sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{
		MoveToken: &vttv1.MoveTokenRequest{TokenId: "tok-brawler", To: &vttv1.GridPosition{X: 0, Y: 1}},
	}})
	if r2 := readResult(t, conn); !r2.Ok {
		t.Fatalf("want a follow-up ordinary command to still succeed after the no-ruleset denial, got %+v", r2)
	}
}

// TestUseAbilityHitProducesBatchFirstSequence is the wiring payoff: fists
// against patron (footing=0, guaranteed hit regardless of the crypto
// Roller's actual d20 draw — see newRulesetFixture's doc comment) produces
// an ok=true CommandResult carrying the FIRST sequence of the batch
// (AbilityUsed, then the hit's ResourceChanged on patron's drink, then the
// drink threshold's ConditionApplied — tavern-brawl's own ruleset.json/
// fists.json, read verbatim, not re-derived here), and every one of those
// events reaches a second, uninvolved connection (the DM's) as broadcasts.
func TestUseAbilityHitProducesBatchFirstSequence(t *testing.T) {
	f := newRulesetFixture(t, true)
	brawlerConn := f.dial(f.brawlerToken, 0)
	// after=6 skips the 6 seeded events' catch-up replay (rf-seed-1..6):
	// this connection should observe ONLY the batch's own live broadcasts.
	dmConn := f.dial(f.dmToken, 6)

	sendCommand(t, brawlerConn, fistsCmd("brawler", "patron"))
	res := readResult(t, brawlerConn)
	if !res.Ok {
		t.Fatalf("want ok=true (footing=0 guarantees a hit), got %+v", res)
	}
	firstSeq := res.Sequence
	if firstSeq != 7 { // 6 seeded events (rf-seed-1..6) + this batch starts at 7
		t.Fatalf("result.Sequence = %d, want 7 (first seq of the batch, after 6 seeded events)", firstSeq)
	}

	// Collect the whole batch off the DM's connection (an uninvolved
	// broadcast recipient) by sequence, in order: AbilityUsed(7),
	// ResourceChanged(8), ConditionApplied(9) — tavern-brawl's fists.json
	// hit list is exactly one resource_change, and its drink threshold
	// (ruleset.json) fires unconditionally once drink becomes non-zero.
	wantKinds := []string{"abilityUsed", "resourceChanged", "conditionApplied"}
	for i, want := range wantKinds {
		env := readEvent(t, dmConn)
		if env.Sequence != firstSeq+int64(i) {
			t.Fatalf("batch event %d: sequence = %d, want %d", i, env.Sequence, firstSeq+int64(i))
		}
		got := payloadKind(env)
		if got != want {
			t.Fatalf("batch event %d: payload kind = %q, want %q", i, got, want)
		}
	}
}

// payloadKind names env's oneof payload variant using the same camelCase
// key protojson would use for it — good enough for this file's own
// assertions without importing reflection machinery.
func payloadKind(env *vttv1.Envelope) string {
	switch env.Payload.(type) {
	case *vttv1.Envelope_AbilityUsed:
		return "abilityUsed"
	case *vttv1.Envelope_ResourceChanged:
		return "resourceChanged"
	case *vttv1.Envelope_ConditionApplied:
		return "conditionApplied"
	case *vttv1.Envelope_ConditionRemoved:
		return "conditionRemoved"
	default:
		return fmt.Sprintf("%T", env.Payload)
	}
}

// TestUseAbilityResolveValidationErrorIsCleanOkFalse covers Resolve's own
// validation errors (unknown ability, here) surfacing as an ordinary
// ok=false CommandResult — proving handleUseAbility does not distinguish
// "no ruleset" from "ruleset loaded but Resolve rejected the command" at
// the wire level; both are clean, connection-preserving denials.
func TestUseAbilityResolveValidationErrorIsCleanOkFalse(t *testing.T) {
	f := newRulesetFixture(t, true)
	conn := f.dial(f.brawlerToken, 0)

	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_UseAbility{
		UseAbility: &vttv1.UseAbility{ActorId: "brawler", AbilityId: "no-such-ability", TargetIds: []string{"patron"}},
	}}
	sendCommand(t, conn, cmd)
	res := readResult(t, conn)
	if res.Ok {
		t.Fatalf("want ok=false for an unknown ability, got %+v", res)
	}
	if !strings.Contains(res.Error, "no-such-ability") {
		t.Fatalf("error = %q, want it to name the unknown ability", res.Error)
	}
}

// TestRemoveConditionAppliedThenRemoved exercises remove_condition end to
// end over the wire: fists first applies dazed-by-ale to patron (via the
// drink threshold), then the DM directly removes it — proving
// remove_condition's OWN path (ToEvent -> campaign.Append, not Resolve;
// see convert.go) actually reaches a real campaign. Removing it a SECOND
// time must cleanly fail (the condition is now absent) without poisoning
// the campaign — a follow-up ordinary command still succeeds.
func TestRemoveConditionAppliedThenRemoved(t *testing.T) {
	f := newRulesetFixture(t, true)
	brawlerConn := f.dial(f.brawlerToken, 0)
	dmConn := f.dial(f.dmToken, 0)

	sendCommand(t, brawlerConn, fistsCmd("brawler", "patron"))
	if r := readResult(t, brawlerConn); !r.Ok {
		t.Fatalf("want the setup fists hit to succeed, got %+v", r)
	}

	removeCmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RemoveCondition{
		RemoveCondition: &vttv1.RemoveCondition{ActorId: "patron", ConditionId: "dazed-by-ale"},
	}}
	sendCommand(t, dmConn, removeCmd)
	r1 := readResult(t, dmConn)
	if !r1.Ok {
		t.Fatalf("want ok=true removing a condition that IS present, got %+v", r1)
	}

	// Second removal: the condition is gone now — a clean ok=false, not a
	// crash or a poisoned campaign.
	sendCommand(t, dmConn, removeCmd)
	r2 := readResult(t, dmConn)
	if r2.Ok {
		t.Fatalf("want ok=false removing an already-absent condition, got %+v", r2)
	}

	// Campaign not poisoned: an ordinary follow-up command still succeeds.
	sendCommand(t, dmConn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}}})
	if r3 := readResult(t, dmConn); !r3.Ok {
		t.Fatalf("want a follow-up ordinary command to still succeed after the absent-condition denial, got %+v", r3)
	}
}
