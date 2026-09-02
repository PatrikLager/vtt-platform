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
// committed campaigns/example/maps/cellar.json map and its
// campaigns/example/packs/cellar-basics pack (Task 3 of the 2026-09-01
// create_scene-leaves plan split what used to be one maps/cellar directory
// into these two; Task 5 of the same plan moved both under
// campaigns/example/, once maps stopped being server-wide --maps-dir
// content and became a campaign's own) — internal/mapdef's own tests
// already cover Load/Compile's correctness in isolation; this file proves
// the WIRING, not the loader. Mirrors adventure_test.go's own shape;
// load_adventure/handleLoadAdventure is this handler's direct template.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

// cellarMapPath and cellarPackDir resolve the committed
// campaigns/example/maps/cellar.json map and its
// campaigns/example/packs/cellar-basics pack, relative to this test file's
// own package directory — the same "../../<path>" convention
// adventure_test.go's goblinAmbushDir establishes. Two functions, not one,
// because Task 3 (the 2026-09-01 create_scene-leaves plan) split what used
// to be one maps/cellar directory into a flat file and a sibling pack
// tree; Task 5 of the same plan then moved both under campaigns/example/.
func cellarMapPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "campaigns", "example", "maps", "cellar.json")
}

func cellarPackDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "campaigns", "example", "packs", "cellar-basics")
}

// loadCellarMap loads the real committed campaigns/example/maps/cellar.json
// and its campaigns/example/packs/cellar-basics pack, failing the test
// loudly if either does not load — a broken fixture here would silently
// turn every test in this file into a no-op, which is worse than a compile
// error.
func loadCellarMap(t *testing.T) (*mapdef.Map, *mapdef.Pack) {
	t.Helper()
	m, err := mapdef.Load(cellarMapPath(t))
	if err != nil {
		t.Fatalf("mapdef.Load(campaigns/example/maps/cellar.json): %v", err)
	}
	pack, err := mapdef.LoadPack(cellarPackDir(t))
	if err != nil {
		t.Fatalf("mapdef.LoadPack(campaigns/example/packs/cellar-basics): %v", err)
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

	// mapsDir is the campaign's own maps/ directory — the place a DM
	// INSTALLS a map into (2026-09-01-create-scene-leaves design spec §4,
	// "Install, then load"). Set for every fixture; only the installable
	// one below wires it into the server, so that the older fixtures keep
	// pinning the behaviour of a server that has no maps directory at all.
	mapsDir string
}

// newMapFixture builds the pre-Task-6 shapes: a server whose map set is
// whatever WithMaps was given at boot and which cannot look anything up on
// disk. Its two states are the two this file pinned before on-demand
// loading existed — no maps configured at all, and a boot-loaded cellar.
func newMapFixture(t *testing.T, withMaps bool) *mapFixture {
	t.Helper()
	return newMapFixtureWith(t, withMaps, false)
}

// newInstallableMapFixture is the shape the 2026-09-01-create-scene-leaves
// sub-project exists for: a server booted with NOTHING installed, wired to
// the campaign's own maps/ directory, so a map written there afterwards is
// loadable with no restart (that plan's design spec §4/§5). maps/ is deliberately not created — a brand-new
// campaign has no maps directory, and creating one would make this fixture
// prove less than the real starting state does.
func newInstallableMapFixture(t *testing.T) *mapFixture {
	t.Helper()
	return newMapFixtureWith(t, false, true)
}

func newMapFixtureWith(t *testing.T, withMaps, installable bool) *mapFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "campaign.db")

	c, err := campaign.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })

	ids, err := identity.Open(campaign.LogPath(path))
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

	mapsDir := filepath.Join(path, "maps")
	srv := gateway.New(c, ids)
	if withMaps {
		m, pack := loadCellarMap(t)
		srv = srv.WithMaps(map[string]*mapdef.Map{m.ID: m}, map[string]*mapdef.Pack{pack.ID: pack})
	}
	if installable {
		srv = srv.WithMapsDir(mapsDir)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	t.Cleanup(httpSrv.Close)

	return &mapFixture{
		t: t, srv: httpSrv,
		dmToken: dmToken, playerToken: playerToken, spectatorToken: spectatorToken,
		agentToken: agentToken,
		mapsDir:    mapsDir,
	}
}

// getAs issues an authenticated GET against this fixture's server, so a
// test can read /api/maps back. Mirrors mapsFixture.getAs
// (metadata_test.go), which belongs to a different fixture in a different
// file.
func (f *mapFixture) getAs(path, token string) (int, []byte) {
	f.t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.srv.URL+path, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp.StatusCode, body
}

// installMap writes one map file into dir, the way a DM (or a script, or a
// future editor) installs a place mid-session: outside the platform, by
// putting a file where the campaign keeps its maps. body is the map's whole
// JSON, so a test can install a BROKEN map as easily as a good one.
func installMap(t *testing.T, dir, id, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// oneSquareMap is a map that passes every check mapdef.Load makes: a
// declared format_version this server understands, and "stone", a real name
// in mapdef's standard vocabulary (standard.go). Both matter — a fixture
// broken in some unrelated way would make a refusal test pass for a reason
// that has nothing to do with what it claims to pin.
func oneSquareMap(id string) string {
	return `{"format_version":1,"id":"` + id + `","name":"Level Two",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"}}`
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

// TestLoadMapNoMapsConfiguredCleanError covers a server for a campaign
// whose maps/ is absent, or which has no maps installed yet
// (2026-09-01-create-scene-leaves Task 5): a load_map command gets a
// clean ok=false CommandResult naming "no maps available" — never a
// connection drop, crash, or protocol error. The connection stays usable
// afterward. Mirrors TestLoadAdventureNoAdventuresConfiguredCleanError
// exactly.
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
			Actor: &vttv1.Actor{ActorId: "act-fighter", Name: "Fighter",
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
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
	// which has its own seed broadcast queued ahead of the batch, so reading
	// positionally off it would assert against the seed AddActor rather than
	// against the load_map batch. agentConn was drained of that one frame
	// above and holds nothing else.
	//
	// CORRECTED, fix round 1 of retraction-leaves Task 9: this comment used
	// to justify the second connection by quoting readResult as one that
	// "skips any Envelope frames that race ahead of it". That sentence left
	// server_test.go when the per-connection demultiplexer landed, and
	// readResult's doc comment now says the opposite — "Events that arrive
	// first are queued, not discarded, so a later readEvent still sees them".
	// The choice of connection is still right; only the reason was wrong.
	// adventure_test.go's multi-adventure test carried the same borrowed
	// claim and is corrected too.
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
			Actor: &vttv1.Actor{ActorId: "act-fighter", Name: "Fighter",
				Kind: vttv1.ActorKind_ACTOR_KIND_PARTY_MEMBER},
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

// --- installed after boot ---------------------------------------------------

// TestAMapInstalledAfterBootIsLoadable is the whole point of the
// 2026-09-01-create-scene-leaves sub-project (design spec §4, exit
// criterion 3): create_scene left the platform, and what replaces
// improvisation is two separate acts — install a map file into the
// campaign's own maps/, then load it. The server here booted with NOTHING
// installed and no maps/ directory at all; the map appears while it is
// serving; no restart, no reconnect.
//
// ok=true alone would not prove it: the assertion follows the batch onto a
// second connection and reads the SceneCreated back, so the map has to have
// reached campaign state, not merely satisfied a lookup.
func TestAMapInstalledAfterBootIsLoadable(t *testing.T) {
	f := newInstallableMapFixture(t)
	dmConn := f.dial(f.dmToken, 0)
	// The AGENT seat reads the batch, not a player's: a map is terrain
	// nobody has walked into yet, and the visibility projection withholds
	// exactly that from a player (this file's own note on
	// TestLoadMapProducesBatchCarryingTilesAndObjects).
	agentConn := f.dial(f.agentToken, 0)

	installMap(t, f.mapsDir, "level-2", oneSquareMap("level-2"))

	sendCommand(t, dmConn, loadMapCmdFor("level-2"))
	res := readResult(t, dmConn)
	if !res.Ok {
		t.Fatalf("load_map after install: %s", res.Error)
	}

	env := readEvent(t, agentConn)
	if got := mapPayloadKind(env); got != "sceneCreated" {
		t.Fatalf("first batch envelope kind = %q, want sceneCreated", got)
	}
	sc := env.GetSceneCreated()
	if sc.GetSceneId() != "level-2" || sc.GetName() != "Level Two" {
		t.Fatalf("SceneCreated id/name = %q/%q, want level-2/Level Two", sc.GetSceneId(), sc.GetName())
	}
	if len(sc.GetTiles()) != 1 {
		t.Fatalf("SceneCreated carries %d tiles, want 1 (the map's whole 1x1 grid)", len(sc.GetTiles()))
	}
}

// TestAMapInstalledAfterBootJoinsTheListing proves the loaded map genuinely
// joined the server's map set rather than being compiled once and
// discarded: /api/maps is how a client discovers what this table has, and a
// map loaded mid-session has to appear there the same as one present at
// boot.
func TestAMapInstalledAfterBootJoinsTheListing(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	if code, body := f.getAs("/api/maps", f.dmToken); code != http.StatusOK || !strings.Contains(string(body), `"maps":[]`) {
		t.Fatalf("before install: status %d body %s, want 200 and an empty list", code, body)
	}

	installMap(t, f.mapsDir, "level-2", oneSquareMap("level-2"))
	sendCommand(t, conn, loadMapCmdFor("level-2"))
	if res := readResult(t, conn); !res.Ok {
		t.Fatalf("load_map after install: %s", res.Error)
	}

	code, body := f.getAs("/api/maps", f.dmToken)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", code, body)
	}
	var got struct {
		Maps []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"maps"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if len(got.Maps) != 1 || got.Maps[0].ID != "level-2" || got.Maps[0].Name != "Level Two" {
		t.Fatalf("/api/maps = %+v, want the one map installed during the session", got.Maps)
	}
}

// TestAnUnknownMapIsRefusedByName pins the refusal on the other side of the
// probe: a server that CAN look on disk, asked for something that is not
// there, answers a clean ok=false naming the map — not a torn connection,
// and not the raw filesystem error the probe actually got. The negative
// half is the load-bearing one: os.Open's own error carries the server's
// absolute path, and handing that to whoever asked would leak the layout of
// the machine to every DM seat.
func TestAnUnknownMapIsRefusedByName(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, loadMapCmdFor("nowhere"))
	res := readResult(t, conn)
	if res.Ok {
		t.Fatalf("want an unknown map refused, got %+v", res)
	}
	if !strings.Contains(res.Error, "unknown map") || !strings.Contains(res.Error, "nowhere") {
		t.Fatalf("error = %q, want it to name the unknown map", res.Error)
	}
	if strings.Contains(res.Error, "no such file") || strings.Contains(res.Error, f.mapsDir) {
		t.Fatalf("error = %q, want no filesystem path or os error in what a client is told", res.Error)
	}

	sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: "s"},
	}})
	if r2 := readResult(t, conn); !r2.Ok {
		t.Fatalf("want a follow-up ordinary command to still succeed after the refusal, got %+v", r2)
	}
}

// TestAnInstalledButBrokenMapIsRefusedAndStaysUnloaded proves the probe
// runs the same validation boot runs and that a refusal leaves the map set
// untouched. The installed file declares an id that disagrees with its
// filename — design spec §6's refusal, the one that stops a rename from
// putting a second scene in the world for the same place — and the error
// has to name both strings, exactly as it does at boot
// (cmd/vtt's TestLoadMapsDirRefusesAFilenameThatDisagreesWithTheID).
//
// Asking twice is not repetition: a probe that cached before validating
// would answer the second attempt differently, and /api/maps below would
// list a map nobody could load.
func TestAnInstalledButBrokenMapIsRefusedAndStaysUnloaded(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	installMap(t, f.mapsDir, "level-2", oneSquareMap("level-3"))

	for attempt := 1; attempt <= 2; attempt++ {
		sendCommand(t, conn, loadMapCmdFor("level-2"))
		res := readResult(t, conn)
		if res.Ok {
			t.Fatalf("attempt %d: a map whose filename disagrees with its id loaded", attempt)
		}
		for _, want := range []string{`declares id "level-3"`, "level-2"} {
			if !strings.Contains(res.Error, want) {
				t.Fatalf("attempt %d: error = %q, want it to contain %q", attempt, res.Error, want)
			}
		}
	}

	code, body := f.getAs("/api/maps", f.dmToken)
	if code != http.StatusOK || !strings.Contains(string(body), `"maps":[]`) {
		t.Fatalf("/api/maps = %d %s, want 200 and an empty list — a map that "+
			"failed to load must not join the set", code, body)
	}
}

// TestTwoRacingLoadsOfANewlyInstalledMapProduceOneScene is the concurrency
// case the design spec calls out by name (§5, "Two load_maps racing on the
// same new id must not both compile and cache it"), pinned where a client
// can see it: two seats issue load_map for the same freshly installed map at
// once. Whichever order they arrive in, exactly one puts the scene in the
// world and the other is cleanly refused for the reason a second load is
// always refused — the scene id already exists — never a torn connection, a
// crash, or two scenes for one place.
//
// The insert-once property itself is pinned in map_internal_test.go, which
// can observe the map set directly; this test is the boundary half.
func TestTwoRacingLoadsOfANewlyInstalledMapProduceOneScene(t *testing.T) {
	f := newInstallableMapFixture(t)
	first := f.dial(f.dmToken, 0)
	second := f.dial(f.dmToken, 0)

	installMap(t, f.mapsDir, "level-2", oneSquareMap("level-2"))

	start := make(chan struct{})
	results := make(chan *vttv1.CommandResult, 2)
	for _, conn := range []*websocket.Conn{first, second} {
		go func() {
			<-start
			sendCommand(t, conn, loadMapCmdFor("level-2"))
			results <- readResult(t, conn)
		}()
	}
	close(start)

	var ok, refused int
	for i := 0; i < 2; i++ {
		res := <-results
		if res.Ok {
			ok++
			continue
		}
		refused++
		if !strings.Contains(res.Error, "already exists") {
			t.Errorf("the losing load was refused with %q, want the ordinary "+
				"scene-collision refusal", res.Error)
		}
	}
	if ok != 1 || refused != 1 {
		t.Fatalf("got %d ok and %d refused, want exactly one of each", ok, refused)
	}
}

// --- fix round 1 (2026-09-01-create-scene-leaves Task 6) ---------------------

// TestNoRefusalTellsAClientWhereTheCampaignLives is the wire-level half of
// the path-disclosure fix. Round 1 of this task had exactly this assertion
// — and only for a map that is not installed, which is the ONE refusal the
// handler translated. Every other broken map forwarded mapdef's error
// verbatim, and mapdef named the file by the path it opened, so an
// ordinary agent seat could read the server's absolute campaign directory
// out of a CommandResult. The 260-character id below is the case review
// fired: it returns ENAMETOOLONG, which is not ENOENT, so nothing about
// round 1's translation caught it.
//
// Every case here is refused for a DIFFERENT reason (missing, unopenable,
// unreadable, unparseable, invalid, unresolvable, mismatched), because the
// defect was never about one error — it was about which errors were
// forwarded, and that is all of them but two.
func TestNoRefusalTellsAClientWhereTheCampaignLives(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	installMap(t, f.mapsDir, "mismatch", oneSquareMap("some-other-id"))
	installMap(t, f.mapsDir, "typo", `{"format_version":1,"id":"typo","name":"Level Two",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stoen"}}`)
	installMap(t, f.mapsDir, "truncated", `{"format_version":1,"id":"truncated",`)
	installMap(t, f.mapsDir, "nopack", `{"format_version":1,"id":"nopack","name":"Level Two",
		"grid_width":1,"grid_height":1,"pack":"cave-basics","tiles":{"0,0":"stone"},
		"overrides":{"0,0":"cave-floor-1"}}`)
	if err := os.MkdirAll(filepath.Join(f.mapsDir, "adir.json"), 0o750); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, id string }{
		{"not installed", "nowhere"},
		{"name too long", strings.Repeat("z", 260)},
		{"filename disagrees with id", "mismatch"},
		{"tile name typo", "typo"},
		{"truncated json", "truncated"},
		{"pack not loaded", "nopack"},
		{"a directory where a map should be", "adir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sendCommand(t, conn, loadMapCmdFor(tc.id))
			res := readResult(t, conn)
			if res.Ok {
				t.Fatalf("id %q loaded; every fixture here is broken", tc.id)
			}
			if strings.Contains(res.Error, f.mapsDir) || strings.Contains(res.Error, os.TempDir()) {
				t.Errorf("a client was told where the campaign lives:\n  %s", res.Error)
			}
			if !strings.Contains(res.Error, tc.id) {
				t.Errorf("error = %q, want it to name the map that was asked for", res.Error)
			}
		})
	}

	// The connection is still usable after all of it: none of these is a
	// protocol error, whatever the filesystem said.
	sendCommand(t, conn, &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{
		StartSession: &vttv1.StartSession{Name: "s"},
	}})
	if r := readResult(t, conn); !r.Ok {
		t.Fatalf("want an ordinary command to still succeed afterwards, got %+v", r)
	}
}

// TestAPackInstalledAfterBootSaysToRestart is I2: the pack seam is real and
// stays — the design spec asks for maps on demand and is silent about packs
// — but round 1 explained it with mapdef's "no pack was given to resolve
// it", which is true of the function call and false about the table: the
// pack IS there, installed and valid, one restart away. A DM reading that
// goes and checks the pack field on a map that is already correct.
//
// The fixture installs BOTH a map and the pack it names, so the only reason
// the load fails is that packs are read at startup — and the message has to
// say that, name the pack, and not claim none was given.
func TestAPackInstalledAfterBootSaysToRestart(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	packDir := filepath.Join(filepath.Dir(f.mapsDir), "packs", "cave-basics")
	if err := os.MkdirAll(packDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "pack.json"), []byte(`{
		"format_version": 1, "id": "cave-basics", "name": "Cave Basics", "cell_px": 64,
		"tiles": [{"name":"cave-floor-1","file":"cave_01.png","kind":"floor","material":"stone"}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	installMap(t, f.mapsDir, "level-5", `{"format_version":1,"id":"level-5","name":"Level Five",
		"grid_width":1,"grid_height":1,"pack":"cave-basics","tiles":{"0,0":"stone"},
		"overrides":{"0,0":"cave-floor-1"}}`)

	sendCommand(t, conn, loadMapCmdFor("level-5"))
	res := readResult(t, conn)
	if res.Ok {
		t.Fatal("a map resolving against a pack installed after boot loaded; packs are " +
			"still boot-time only, and half-loading one would be worse than refusing")
	}
	for _, want := range []string{`declares pack "cave-basics"`, "restart"} {
		if !strings.Contains(res.Error, want) {
			t.Errorf("error = %q, want it to contain %q", res.Error, want)
		}
	}
	if strings.Contains(res.Error, "no pack was given") {
		t.Errorf("error = %q — a pack WAS given, and is sitting installed in this "+
			"campaign; saying otherwise sends the DM to check a correct map", res.Error)
	}
}
