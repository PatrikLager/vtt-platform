package gateway_test

// adventure_test.go covers the load_adventure wiring itself (adventure-
// format Task 4): adventure.Compile -> campaign.AppendBatch, the "no
// adventures available"/"unknown adventure" clean errors, a Compile
// collision (double-load of the SAME adventure) surfacing as a clean
// ok=false rejection rather than a poisoned campaign, and the whole batch
// reaching every connected participant contiguously. Built against the REAL
// committed adventures/goblin-ambush directory and rulesets/dnd45e-minimal
// ruleset (internal/adventure/conformance's own P4 proof already covers
// Load/Compile's own correctness in isolation) — this file proves the
// WIRING, not the loader. Mirrors ruleset_test.go's own shape (that file's
// doc comment) — use_ability is this handler's direct template
// (task-12-4-brief.md).

import (
	"context"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/adventure"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// dnd45eMinimalDir/goblinAmbushDir resolve the committed rulesets/
// dnd45e-minimal and adventures/goblin-ambush directories relative to this
// test file's own package directory — the same "../../<dir>" convention
// tavernBrawlDir (ruleset_test.go) already establishes.
func dnd45eMinimalDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "rulesets", "dnd45e-minimal")
}

func goblinAmbushDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "adventures", "goblin-ambush")
}

func loadDnd45eMinimal(t *testing.T) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Load(dnd45eMinimalDir(t))
	if err != nil {
		t.Fatalf("rules.Load(dnd45e-minimal): %v", err)
	}
	return rs
}

func loadGoblinAmbush(t *testing.T, rs *rules.Ruleset) *adventure.Adventure {
	t.Helper()
	adv, err := adventure.Load(goblinAmbushDir(t), rs)
	if err != nil {
		t.Fatalf("adventure.Load(goblin-ambush): %v", err)
	}
	return adv
}

// cellarRatsDir resolves the committed adventures/cellar-rats directory —
// goblinAmbushDir's sibling, used together with it (fix-wave F3) to prove
// the gateway's load_adventure id-selection contract against a server with
// TWO adventures loaded, not just the degenerate single-entry case every
// other test in this file exercises. cellar-rats declares ruleset
// "tavern-brawl" (loadTavernBrawl, ruleset_test.go), deliberately a
// DIFFERENT ruleset than goblin-ambush's "dnd45e-minimal" — the gateway
// itself never re-validates an already-loaded Adventure against a served
// ruleset at request time (that happened once, at boot, via
// adventure.Load), so two adventures loaded under different rulesets is a
// legal WithAdventures configuration and a stronger proof than two
// same-ruleset adventures would be.
func cellarRatsDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "adventures", "cellar-rats")
}

func loadCellarRats(t *testing.T, rs *rules.Ruleset) *adventure.Adventure {
	t.Helper()
	adv, err := adventure.Load(cellarRatsDir(t), rs)
	if err != nil {
		t.Fatalf("adventure.Load(cellar-rats): %v", err)
	}
	return adv
}

// adventureFixture is newRulesetFixture's (ruleset_test.go) sibling: a
// gateway.Server with the REAL goblin-ambush adventure loaded via
// WithAdventures when withAdventures is true — the one gwFixture
// (server_test.go) deliberately does NOT configure.
type adventureFixture struct {
	t   *testing.T
	srv *httptest.Server

	dmToken        string
	playerToken    string
	spectatorToken string
	// agentToken is the OTHER seat that receives the log unfiltered (spec
	// §3.1, exit criterion 8). Tests here that need "a second, uninvolved
	// connection" to read a whole batch off use it rather than the player:
	// since the visibility projection landed, a player receives what their
	// actors can see, and an adventure's batch is precisely a pile of world
	// the player has not walked into yet.
	agentToken string
}

func newAdventureFixture(t *testing.T, withAdventures bool) *adventureFixture {
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

	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	playerToken, _, err := ids.CreateInvite("Player", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	spectatorToken, _, err := ids.CreateInvite("Watcher", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	agentToken, _, err := ids.CreateInvite("Agent", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}

	srv := gateway.New(c, ids)
	if withAdventures {
		rs := loadDnd45eMinimal(t)
		adv := loadGoblinAmbush(t, rs)
		srv = srv.WithAdventures(map[string]*adventure.Adventure{adv.ID: adv})
	}
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &adventureFixture{
		t: t, srv: httpSrv,
		dmToken: dmToken, playerToken: playerToken, spectatorToken: spectatorToken,
		agentToken: agentToken,
	}
}

// newMultiAdventureFixture is newAdventureFixture(t, true)'s sibling (fix-
// wave F3): a gateway.Server with BOTH goblin-ambush and cellar-rats loaded
// via a single WithAdventures call — the multi-adventure --adventures-dir
// production shape (cmd/vtt's loadAdventuresDir loads every subdirectory
// and boot-errors on duplicate ids specifically to support this) that no
// existing test configures. Every load_adventure gateway test before this
// one used a single-entry map, so a lookup regression that serves a
// DIFFERENT loaded adventure's content than requested (s.adventures[cmd.
// GetAdventureId()] returning the wrong entry) would pass unnoticed.
func newMultiAdventureFixture(t *testing.T) *adventureFixture {
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

	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	playerToken, _, err := ids.CreateInvite("Player", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	spectatorToken, _, err := ids.CreateInvite("Watcher", identity.RoleSpectator)
	if err != nil {
		t.Fatal(err)
	}
	agentToken, _, err := ids.CreateInvite("Agent", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}

	goblinRS := loadDnd45eMinimal(t)
	goblinAdv := loadGoblinAmbush(t, goblinRS)
	cellarRS := loadTavernBrawl(t)
	cellarAdv := loadCellarRats(t, cellarRS)

	srv := gateway.New(c, ids).WithAdventures(map[string]*adventure.Adventure{
		goblinAdv.ID: goblinAdv,
		cellarAdv.ID: cellarAdv,
	})
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &adventureFixture{
		t: t, srv: httpSrv,
		dmToken: dmToken, playerToken: playerToken, spectatorToken: spectatorToken,
		agentToken: agentToken,
	}
}

// wsURL/dial mirror rulesetFixture's own (ruleset_test.go) byte-for-byte.
func (f *adventureFixture) wsURL(token string, after int64) string {
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

func (f *adventureFixture) dial(token string, after int64) *websocket.Conn {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, f.wsURL(token, after), nil)
	if err != nil {
		f.t.Fatalf("dial: %v", err)
	}
	f.t.Cleanup(func() { conn.CloseNow() })
	// maps-as-geometry Task 4: SceneCreated now carries one TileRef per grid
	// square, so loading a real adventure (goblin-ambush's scene alone is
	// ~1024 squares) can exceed coder/websocket's default 32KB read cap —
	// see server_internal_test.go's bigSceneName fixture
	// (TestAWedgedConnectionIsTornDownAndOthersKeepServing), the precedent
	// this 200KiB matches exactly.
	conn.SetReadLimit(200 * 1024)
	return conn
}

// loadAdventureCmdFor builds a LoadAdventure ClientCommand for id. Distinct
// from authz_test.go's argument-less loadAdventureCmd, whose name this doc
// carried until check:doc-owner was pointed at test files.
func loadAdventureCmdFor(id string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_LoadAdventure{
		LoadAdventure: &vttv1.LoadAdventure{AdventureId: id},
	}}
}

// --- tests -----------------------------------------------------------------

// TestLoadAdventureNoAdventuresConfiguredCleanError covers spec §7's
// binding: serving without --adventures-dir keeps every other command
// working; a load_adventure command specifically gets a clean ok=false
// CommandResult naming "no adventures available" — never a connection drop,
// crash, or protocol error. The connection stays usable afterward.
func TestLoadAdventureNoAdventuresConfiguredCleanError(t *testing.T) {
	f := newAdventureFixture(t, false) // withAdventures=false
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, loadAdventureCmdFor("goblin-ambush"))
	res := readResult(t, conn)
	if res.Ok {
		t.Fatalf("want ok=false with no adventures configured, got %+v", res)
	}
	if !strings.Contains(res.Error, "no adventures available") {
		t.Fatalf("error = %q, want it to contain %q", res.Error, "no adventures available")
	}

	sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: "s"},
	}})
	if r2 := readResult(t, conn); !r2.Ok {
		t.Fatalf("want a follow-up ordinary command to still succeed after the no-adventures denial, got %+v", r2)
	}
}

// TestLoadAdventureUnknownIdCleanError covers the "unknown adventure" clean
// error, distinct from "no adventures configured at all" — the server DOES
// have adventures loaded, just not this id.
func TestLoadAdventureUnknownIdCleanError(t *testing.T) {
	f := newAdventureFixture(t, true)
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, loadAdventureCmdFor("no-such-adventure"))
	res := readResult(t, conn)
	if res.Ok {
		t.Fatalf("want ok=false for an unknown adventure id, got %+v", res)
	}
	if !strings.Contains(res.Error, "unknown adventure") || !strings.Contains(res.Error, "no-such-adventure") {
		t.Fatalf("error = %q, want it to name the unknown adventure", res.Error)
	}
}

// adventurePayloadKind names env's oneof payload variant using the same camelCase
// key protojson would use for it (ruleset_test.go's own precedent, extended
// with the adventure-format Compile batch's variants).
func adventurePayloadKind(env *vttv1.Envelope) string {
	switch env.Payload.(type) {
	case *vttv1.Envelope_AdventureLoaded:
		return "adventureLoaded"
	case *vttv1.Envelope_SceneCreated:
		return "sceneCreated"
	case *vttv1.Envelope_ActorAdded:
		return "actorAdded"
	case *vttv1.Envelope_TokenPlaced:
		return "tokenPlaced"
	case *vttv1.Envelope_NoteUpserted:
		return "noteUpserted"
	case *vttv1.Envelope_NarrationAdded:
		return "narrationAdded"
	default:
		return "other"
	}
}

// TestLoadAdventureProducesBatchFirstSequenceReachesAllParticipants is the
// wiring payoff: loading goblin-ambush against a fresh campaign produces an
// ok=true CommandResult carrying the FIRST sequence of the batch (sequence
// 1 — nothing else has been appended yet), and the WHOLE 10-envelope batch
// (AdventureLoaded, one SceneCreated, three ActorAdded, three TokenPlaced,
// one NoteUpserted, one NarrationAdded — internal/adventure/conformance's
// own hand-derived goblin-ambush golden, .superpowers/sdd/p12-task-3-
// report.md) reaches a second, uninvolved connection as broadcasts, in
// Compile's own binding order.
func TestLoadAdventureProducesBatchFirstSequenceReachesAllParticipants(t *testing.T) {
	f := newAdventureFixture(t, true)
	dmConn := f.dial(f.dmToken, 0)
	// The AGENT is the second connection, not the player. The batch this test
	// follows envelope for envelope is a whole adventure — a scene the player
	// has not entered and three actors they have not met — and the visibility
	// projection withholds exactly that. What is under test here is that a
	// BATCH fans out whole and in Compile's order, which the agent seat proves
	// (spec §3.1: agent and DM receive the log unchanged); what a player
	// receives of an adventure is the projection's business and is pinned in
	// project_test.go and server_visibility_test.go.
	agentConn := f.dial(f.agentToken, 0)

	sendCommand(t, dmConn, loadAdventureCmdFor("goblin-ambush"))
	res := readResult(t, dmConn)
	if !res.Ok {
		t.Fatalf("want ok=true loading goblin-ambush against a fresh campaign, got %+v", res)
	}
	if res.Sequence != 1 {
		t.Fatalf("result.Sequence = %d, want 1 (first event of a fresh campaign)", res.Sequence)
	}

	wantKinds := []string{
		"adventureLoaded", "sceneCreated",
		"actorAdded", "actorAdded", "actorAdded",
		"tokenPlaced", "tokenPlaced", "tokenPlaced",
		"noteUpserted", "narrationAdded",
	}
	for i, want := range wantKinds {
		env := readEvent(t, agentConn)
		if env.Sequence != res.Sequence+int64(i) {
			t.Fatalf("batch event %d: sequence = %d, want %d", i, env.Sequence, res.Sequence+int64(i))
		}
		if got := adventurePayloadKind(env); got != want {
			t.Fatalf("batch event %d: payload kind = %q, want %q", i, got, want)
		}
	}
}

// TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned is the
// double-load proof (spec §5: ids "must NOT collide with existing campaign
// state at load time... rejection, not overwrite"): loading goblin-ambush a
// SECOND time collides on its own scene/actor/token ids (checked by
// adventure.Compile's checkCollisions against the live snapshot, and
// independently re-validated by campaign.AppendBatch's own atomic
// snapshot-fold — see adventure.go's handleLoadAdventure doc comment on the
// TOCTOU race posture) and is cleanly rejected — never a poisoned campaign,
// proven by a follow-up ordinary command still succeeding on the same
// connection.
func TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned(t *testing.T) {
	f := newAdventureFixture(t, true)
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, loadAdventureCmdFor("goblin-ambush"))
	if r1 := readResult(t, conn); !r1.Ok {
		t.Fatalf("want the first load to succeed, got %+v", r1)
	}
	// Drain the first load's whole 10-envelope batch (this file's own
	// wantKinds golden above) — conn is a participant, so it receives its
	// own broadcast too. Leftover, un-drained event frames would otherwise
	// still be queued ahead of the SECOND command's own CommandResult
	// below, overflowing readResult's bounded frame scan (server_test.go).
	for i := 0; i < 10; i++ {
		readEvent(t, conn)
	}

	sendCommand(t, conn, loadAdventureCmdFor("goblin-ambush"))
	r2 := readResult(t, conn)
	if r2.Ok {
		t.Fatalf("want ok=false on a second load of the same adventure (id collision), got %+v", r2)
	}
	if !strings.Contains(r2.Error, "collides") {
		t.Fatalf("error = %q, want it to name a collision", r2.Error)
	}

	// Campaign not poisoned: an ordinary follow-up command still succeeds.
	sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: "s"},
	}})
	if r3 := readResult(t, conn); !r3.Ok {
		t.Fatalf("want a follow-up ordinary command to still succeed after the collision denial, got %+v", r3)
	}
}

// TestLoadAdventureWithMultipleAdventuresLoadedServesRequestedContent covers
// the id-selection contract itself (fix-wave F3, task-12-wf-final-review
// finding): every OTHER load_adventure gateway test loads a server with a
// SINGLE adventure (newAdventureFixture(t, true) always maps just
// goblin-ambush), so a lookup regression at handleLoadAdventure
// (adventure.go:76, `s.adventures[cmd.GetAdventureId()]`) that compiles a
// DIFFERENT loaded adventure than the one requested — whenever more than
// one is loaded, with ok still true — passes the entire existing suite
// undetected. newMultiAdventureFixture loads TWO adventures (goblin-ambush,
// cellar-rats); each subtest requests one id and asserts the compiled
// batch's FIRST envelope (AdventureLoaded testimony) names exactly that
// adventure_id/name, plus a distinguishing envelope only the requested
// adventure could have produced (goblin-ambush's scene id is "ravine",
// cellar-rats' is "cellar" — the two never collide) — so serving the
// OTHER loaded adventure's content would fail both assertions.
//
// The batch is read from a SECOND, uninvolved connection (agentConn), the
// same choice TestLoadAdventureProducesBatchFirstSequenceReachesAllParticipants
// makes — not the commanding dmConn, whose own queue holds earlier broadcasts
// this test would then read positionally instead of the batch it asserts on.
//
// CORRECTED, fix round 1 of retraction-leaves Task 9: this comment used to
// justify the second connection by quoting readResult as one that "skips any
// Envelope frames that race ahead of it". That sentence left server_test.go
// when the per-connection demultiplexer landed, and readResult's doc comment
// now says the opposite — "Events that arrive first are queued, not discarded,
// so a later readEvent still sees them". The choice of connection is still
// right; only the reason was wrong. map_test.go carried the same borrowed
// claim and is corrected too.
func TestLoadAdventureWithMultipleAdventuresLoadedServesRequestedContent(t *testing.T) {
	cases := []struct {
		requestID       string
		wantName        string
		wantSceneID     string
		wantBatchLength int
	}{
		{requestID: "goblin-ambush", wantName: "Goblin Ambush", wantSceneID: "ravine", wantBatchLength: 10},
		{requestID: "cellar-rats", wantName: "Cellar Rats", wantSceneID: "cellar", wantBatchLength: 8},
	}

	for _, c := range cases {
		t.Run(c.requestID, func(t *testing.T) {
			f := newMultiAdventureFixture(t)
			dmConn := f.dial(f.dmToken, 0)
			// The agent seat, for the same reason as the test above: this
			// follows a whole adventure batch, which is world a player has
			// not walked into.
			agentConn := f.dial(f.agentToken, 0)

			sendCommand(t, dmConn, loadAdventureCmdFor(c.requestID))
			res := readResult(t, dmConn)
			if !res.Ok {
				t.Fatalf("want ok=true loading %q from a server with two adventures loaded, got %+v", c.requestID, res)
			}

			loaded := readEvent(t, agentConn)
			al := loaded.GetAdventureLoaded()
			if al == nil {
				t.Fatalf("first batch envelope = %+v, want AdventureLoaded", loaded)
			}
			if al.GetAdventureId() != c.requestID {
				t.Fatalf("AdventureLoaded.AdventureId = %q, want the REQUESTED id %q (not the other loaded adventure's)", al.GetAdventureId(), c.requestID)
			}
			if al.GetName() != c.wantName {
				t.Fatalf("AdventureLoaded.Name = %q, want %q", al.GetName(), c.wantName)
			}

			// Distinguishing envelope: the SceneCreated that only the
			// REQUESTED adventure declares — proof the whole batch, not just
			// the AdventureLoaded testimony's own id field, matches the
			// requested content.
			sceneEnv := readEvent(t, agentConn)
			sc := sceneEnv.GetSceneCreated()
			if sc == nil {
				t.Fatalf("second batch envelope = %+v, want SceneCreated", sceneEnv)
			}
			if sc.GetSceneId() != c.wantSceneID {
				t.Fatalf("SceneCreated.SceneId = %q, want %q (the requested adventure %q's own scene, not the other loaded adventure's)", sc.GetSceneId(), c.wantSceneID, c.requestID)
			}

			// Drain the rest of the batch so this subtest's connection
			// teardown doesn't leave unread frames behind.
			for i := 2; i < c.wantBatchLength; i++ {
				readEvent(t, agentConn)
			}
		})
	}
}
