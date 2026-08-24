package gateway_test

// map_test.go covers the load_map wiring itself (whole-branch-review C1
// remediation): mapdef.Compile -> campaign.AppendBatch, the "no maps
// available"/"unknown map" clean errors, a Compile collision (double-load of
// the SAME map) surfacing as a clean ok=false rejection rather than a
// poisoned campaign, and the whole batch reaching every connected
// participant contiguously — proof that a standalone map genuinely reaches
// campaign state, not just GET /api/maps metadata (the defect this task
// exists to close: mapdef.Compile's only production caller discarded its
// result as a boot-time dry run, so maps/cellar could be validated, listed,
// and have its art served, but never loaded). Built against the REAL
// committed maps/cellar directory — internal/mapdef's own tests already
// cover Load/Compile's correctness in isolation; this file proves the
// WIRING, not the loader. Mirrors adventure_test.go's own shape;
// load_adventure/handleLoadAdventure is this handler's direct template.

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
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

// cellarMapDir resolves the committed maps/cellar directory relative to this
// test file's own package directory — the same "../../<dir>" convention
// adventure_test.go's goblinAmbushDir establishes.
func cellarMapDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "maps", "cellar")
}

// loadCellarMap loads the real committed maps/cellar/map.json and its
// tiles/pack.json, failing the test loudly if either does not load — a
// broken fixture here would silently turn every test in this file into a
// no-op, which is worse than a compile error.
func loadCellarMap(t *testing.T) (*mapdef.Map, *mapdef.Pack) {
	t.Helper()
	dir := cellarMapDir(t)
	m, err := mapdef.Load(filepath.Join(dir, "map.json"))
	if err != nil {
		t.Fatalf("mapdef.Load(maps/cellar): %v", err)
	}
	pack, err := mapdef.LoadPack(filepath.Join(dir, "tiles"))
	if err != nil {
		t.Fatalf("mapdef.LoadPack(maps/cellar/tiles): %v", err)
	}
	return m, pack
}

// mapFixture is adventureFixture's sibling (adventure_test.go): a
// gateway.Server with the REAL committed maps/cellar map loaded via WithMaps
// when withMaps is true — the one gwFixture (server_test.go) deliberately
// does not configure.
type mapFixture struct {
	t   *testing.T
	srv *httptest.Server

	dmToken        string
	playerToken    string
	spectatorToken string
	// agentToken: the other seat that receives the log unfiltered (spec §3.1,
	// exit criterion 8), for the same reason adventureFixture grew one — a
	// load_map batch is a dungeon nobody has walked into yet, and the
	// visibility projection withholds exactly that from a player.
	agentToken string
}

func newMapFixture(t *testing.T, withMaps bool) *mapFixture {
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
	if withMaps {
		m, pack := loadCellarMap(t)
		srv = srv.WithMaps(map[string]*mapdef.Map{m.ID: m}, map[string]*mapdef.Pack{pack.ID: pack})
	}
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &mapFixture{
		t: t, srv: httpSrv,
		dmToken: dmToken, playerToken: playerToken, spectatorToken: spectatorToken,
		agentToken: agentToken,
	}
}

// wsURL/dial mirror adventureFixture's own (adventure_test.go) byte-for-byte.
func (f *mapFixture) wsURL(token string, after int64) string {
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

func (f *mapFixture) dial(token string, after int64) *websocket.Conn {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, f.wsURL(token, after), nil)
	if err != nil {
		f.t.Fatalf("dial: %v", err)
	}
	f.t.Cleanup(func() { conn.CloseNow() })
	// maps/cellar is a 10x9 grid (90 squares), comfortably inside coder/
	// websocket's default 32KB read cap — matched to adventure_test.go's own
	// 200KiB bump anyway, for consistency and headroom.
	conn.SetReadLimit(200 * 1024)
	return conn
}

// loadMapCmdFor builds a LoadMap ClientCommand for id.
func loadMapCmdFor(id string) *vttv1.ClientCommand {
	return &vttv1.ClientCommand{Command: &vttv1.ClientCommand_LoadMap{
		LoadMap: &vttv1.LoadMap{MapId: id},
	}}
}

// --- tests -----------------------------------------------------------------

// TestLoadMapNoMapsConfiguredCleanError covers a server started with no
// --maps-dir: a load_map command gets a clean ok=false CommandResult naming
// "no maps available" — never a connection drop, crash, or protocol error.
// The connection stays usable afterward. Mirrors
// TestLoadAdventureNoAdventuresConfiguredCleanError exactly.
func TestLoadMapNoMapsConfiguredCleanError(t *testing.T) {
	f := newMapFixture(t, false) // withMaps=false
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, loadMapCmdFor("cellar"))
	res := readResult(t, conn)
	if res.Ok {
		t.Fatalf("want ok=false with no maps configured, got %+v", res)
	}
	if !strings.Contains(res.Error, "no maps available") {
		t.Fatalf("error = %q, want it to contain %q", res.Error, "no maps available")
	}

	sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: "s"},
	}})
	if r2 := readResult(t, conn); !r2.Ok {
		t.Fatalf("want a follow-up ordinary command to still succeed after the no-maps denial, got %+v", r2)
	}
}

// TestLoadMapUnknownIdCleanError covers the "unknown map" clean error,
// distinct from "no maps configured at all" — the server DOES have maps
// loaded, just not this id.
func TestLoadMapUnknownIdCleanError(t *testing.T) {
	f := newMapFixture(t, true)
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, loadMapCmdFor("no-such-map"))
	res := readResult(t, conn)
	if res.Ok {
		t.Fatalf("want ok=false for an unknown map id, got %+v", res)
	}
	if !strings.Contains(res.Error, "unknown map") || !strings.Contains(res.Error, "no-such-map") {
		t.Fatalf("error = %q, want it to name the unknown map", res.Error)
	}
}

// mapPayloadKind names env's oneof payload variant, adventure_test.go's own
// adventurePayloadKind restricted to the two variants a map's compiled batch
// can ever contain (mapdef.Compile: one SceneCreated, then one TokenPlaced
// per placement).
func mapPayloadKind(env *vttv1.Envelope) string {
	switch env.Payload.(type) {
	case *vttv1.Envelope_SceneCreated:
		return "sceneCreated"
	case *vttv1.Envelope_TokenPlaced:
		return "tokenPlaced"
	default:
		return "other"
	}
}

// TestLoadMapProducesBatchCarryingTilesAndObjects is THE C1 proof: loading
// maps/cellar through the real gateway produces a SceneCreated that carries
// its full 10x9 tile grid (both layers resolved — a wall square's masonry
// override AND a floor square's earth override both survive to the wire)
// and all six declared objects, followed by a TokenPlaced for its one
// declared placement, every envelope stamped (EventId/ActorRole/OccurredAt)
// before AppendBatch — i.e. the map genuinely reaches campaign state, not
// just GET /api/maps metadata.
//
// maps/cellar's own placement names actor "act-fighter", which the MAP
// format never creates (maps are terrain and placements only, spec §3.4 —
// there is no AddActor-shaped construct anywhere in mapdef) — so, exactly
// like a real table, the actor is added FIRST via add_actor, mirroring
// server_test.go's own seedCellar fixture (which seeds the identical
// actor/token by hand for its door/movement tests) and confirming
// load_map's real usage shape: set up your characters, then drop in a
// dungeon. Without this seed, AppendBatch would reject the whole batch at
// its TokenPlaced envelope (engine.Apply: "token placed for unknown actor")
// and NOTHING would persist — not even the SceneCreated ahead of it, since
// AppendBatch validates the whole batch atomically before persisting any of
// it. Confirmed as actual behavior while developing this test, not assumed.
func TestLoadMapProducesBatchCarryingTilesAndObjects(t *testing.T) {
	f := newMapFixture(t, true)
	dmConn := f.dial(f.dmToken, 0)
	// The AGENT seat reads the batch back, not the player: this test follows a
	// whole load_map batch envelope for envelope, and a map is terrain a player
	// has not entered — the visibility projection withholds it by design.
	agentConn := f.dial(f.agentToken, 0)

	sendCommand(t, dmConn, &vttv1.ClientCommand{
		RequestId: "seed-fighter",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-fighter", Name: "Fighter"},
		}},
	})
	if r0 := readResult(t, dmConn); !r0.Ok {
		t.Fatalf("seed AddActor act-fighter: %s", r0.Error)
	}
	// Drain the seed actor's own broadcast off the second connection before
	// reading the load_map batch below — agentConn is a participant too,
	// so it receives every broadcast, including this seed.
	readEvent(t, agentConn)

	sendCommand(t, dmConn, loadMapCmdFor("cellar"))
	res := readResult(t, dmConn)
	if !res.Ok {
		t.Fatalf("want ok=true loading maps/cellar, got %+v", res)
	}
	// Second sequence overall: seq 1 was the seed AddActor above.
	if res.Sequence != 2 {
		t.Fatalf("result.Sequence = %d, want 2 (first event of the load_map batch, after the seed AddActor)", res.Sequence)
	}

	// Read the batch from the SECOND, uninvolved connection — not dmConn,
	// which just read its own CommandResult: readResult "skips any Envelope
	// frames that race ahead of it" (server_test.go's own doc comment), so
	// reading the batch back off the SAME connection that issued the
	// command risks silently swallowing the very SceneCreated this test
	// asserts on. A second connection never sees the CommandResult frame at
	// all, so readEvent on it is race-free — the identical reasoning
	// adventure_test.go's own multi-adventure test gives for the same
	// choice.
	sceneEnv := readEvent(t, agentConn)
	if got := mapPayloadKind(sceneEnv); got != "sceneCreated" {
		t.Fatalf("first batch envelope kind = %q, want sceneCreated", got)
	}
	sc := sceneEnv.GetSceneCreated()
	if sc.GetSceneId() != "cellar" || sc.GetName() != "The Sunken Cellar" {
		t.Fatalf("SceneCreated id/name = %q/%q, want cellar/The Sunken Cellar", sc.GetSceneId(), sc.GetName())
	}
	if sc.GetGridWidth() != 10 || sc.GetGridHeight() != 9 {
		t.Fatalf("SceneCreated grid = %dx%d, want 10x9", sc.GetGridWidth(), sc.GetGridHeight())
	}
	if len(sc.GetTiles()) != 90 {
		t.Fatalf("SceneCreated carries %d tiles, want 90 (10x9)", len(sc.GetTiles()))
	}
	// Spot-check one wall square (a masonry-1 override) and one floor
	// square (an earth-1 override) — proves the whole tiles/overrides
	// two-layer resolution reached the wire (base tile's kind/material,
	// override's art), not just an empty or zero-valued Tiles map.
	wall := sc.GetTiles()["0,0"]
	if wall.GetKind() != "wall" || wall.GetMaterial() != "stone" || wall.GetArt() != "masonry-1" {
		t.Fatalf("tiles[0,0] = %+v, want kind=wall material=stone art=masonry-1", wall)
	}
	floor := sc.GetTiles()["1,1"]
	if floor.GetKind() != "floor" || floor.GetMaterial() != "earth" || floor.GetArt() != "earth-1" {
		t.Fatalf("tiles[1,1] = %+v, want kind=floor material=earth art=earth-1", floor)
	}
	if len(sc.GetObjects()) != 6 {
		t.Fatalf("SceneCreated carries %d objects, want 6", len(sc.GetObjects()))
	}
	var pillar *vttv1.SceneObject
	for _, o := range sc.GetObjects() {
		if o.GetObjectId() == "pillar-west-1" {
			pillar = o
		}
	}
	if pillar == nil {
		t.Fatalf("objects missing pillar-west-1: %+v", sc.GetObjects())
	}
	if pillar.GetAt().GetX() != 2 || pillar.GetAt().GetY() != 2 || !pillar.GetBlocksSight() || !pillar.GetBlocksMove() {
		t.Fatalf("pillar-west-1 = %+v, want at (2,2), blocks sight and move", pillar)
	}

	tokEnv := readEvent(t, agentConn)
	if got := mapPayloadKind(tokEnv); got != "tokenPlaced" {
		t.Fatalf("second batch envelope kind = %q, want tokenPlaced", got)
	}
	tp := tokEnv.GetTokenPlaced()
	if tp.GetTokenId() != "tok-fighter" || tp.GetSceneId() != "cellar" || tp.GetActorId() != "act-fighter" {
		t.Fatalf("TokenPlaced = %+v, want tok-fighter/cellar/act-fighter", tp)
	}
	if tp.GetPosition().GetX() != 2 || tp.GetPosition().GetY() != 1 {
		t.Fatalf("TokenPlaced position = %+v, want (2,1)", tp.GetPosition())
	}

	// Every envelope in the batch must be stamped by the HANDLER, not left
	// zero: mapdef.Compile itself leaves EventId/ParticipantId/ActorRole/
	// OccurredAt zero by convention (matching adventure.Compile's own
	// contract — see adventure.go's handleLoadAdventure doc comment).
	for _, env := range []*vttv1.Envelope{sceneEnv, tokEnv} {
		if env.GetEventId() == "" {
			t.Errorf("envelope %v has no EventId — mapdef.Compile's envelopes must be stamped before AppendBatch", env)
		}
		if env.GetActorRole() != string(identity.RoleDM) {
			t.Errorf("envelope ActorRole = %q, want %q", env.GetActorRole(), identity.RoleDM)
		}
		if env.GetOccurredAt() == nil {
			t.Errorf("envelope has no OccurredAt")
		}
	}
}

// TestLoadMapDoubleLoadCollisionRejectedCleanNotPoisoned proves AppendBatch's
// atomicity for load_map specifically, the same proof
// TestLoadAdventureDoubleLoadCollisionRejectedCleanNotPoisoned gives
// load_adventure: loading maps/cellar a SECOND time collides on its own
// scene id ("cellar" already exists — engine.Apply's SceneCreated arm,
// internal/engine/apply.go) and is cleanly rejected — never a poisoned
// campaign, proven by a follow-up ordinary command still succeeding on the
// same connection.
//
// Unlike adventure.Compile (which pre-checks collisions itself before
// building any envelope), mapdef.Compile does no such check — the ONLY
// thing that catches this collision is campaign.AppendBatch's own
// snapshot-fold validation, so this test also proves that backstop actually
// engages for the standalone-map path, not just the adventure path.
func TestLoadMapDoubleLoadCollisionRejectedCleanNotPoisoned(t *testing.T) {
	f := newMapFixture(t, true)
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, &vttv1.ClientCommand{
		RequestId: "seed-fighter",
		Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{
			Actor: &vttv1.Actor{ActorId: "act-fighter", Name: "Fighter"},
		}},
	})
	if r0 := readResult(t, conn); !r0.Ok {
		t.Fatalf("seed AddActor act-fighter: %s", r0.Error)
	}

	sendCommand(t, conn, loadMapCmdFor("cellar"))
	if r1 := readResult(t, conn); !r1.Ok {
		t.Fatalf("want the first load to succeed, got %+v", r1)
	}
	// Drain the first load's whole 2-envelope batch (SceneCreated,
	// TokenPlaced) — conn is a participant, so it receives its own
	// broadcast too. Leftover, un-drained event frames would otherwise
	// still be queued ahead of the SECOND command's own CommandResult
	// below, overflowing readResult's bounded frame scan.
	for i := 0; i < 2; i++ {
		readEvent(t, conn)
	}

	sendCommand(t, conn, loadMapCmdFor("cellar"))
	r2 := readResult(t, conn)
	if r2.Ok {
		t.Fatalf("want ok=false on a second load of the same map (scene id collision), got %+v", r2)
	}
	if !strings.Contains(r2.Error, "already exists") {
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
