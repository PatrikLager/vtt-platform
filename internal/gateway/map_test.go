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
//
// The pack is still loaded because GET /api/packs/{pack}/{file} still serves
// from it; nothing RESOLVES against it any more (2026-09-02-art-is-a-flat-library
// Task 3). Art comes from cellarArtDir below.
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

// cellarArtDir builds, in a temp directory, exactly the art
// campaigns/example/maps/cellar.json names — the four tile pieces its
// overrides reference and the four object pictures its objects do — in the
// flat layout 2026-09-02-art-is-a-flat-library design spec §3 defines: a
// picture per piece, and beside it a sidecar for tile art only (§3.4, and
// mapdef.Resolve refuses tile art with no sidecar).
//
// Built here rather than read from campaigns/example/art/, which does not
// exist yet: Task 8 of that plan migrates the committed fixture, and until it
// does this is the only way to assert that an override's art reaches the wire
// without weakening the assertion. The kinds and materials are copied from
// campaigns/example/packs/cellar-basics/pack.json so no square picks up a
// spurious kind-mismatch warning.
//
// WHAT IS THEREFORE UNTESTED, AND IT BELONGS TO TASK 8: nothing anywhere
// asserts that the SHIPPED campaign's own art reaches the wire.
// TestLoadMapProducesBatchCarryingTilesAndObjects below loads the real
// campaigns/example/maps/cellar.json and resolves it against this synthetic
// directory, so it proves the wiring and not the fixture — and every override
// in the shipped campaign degrades today whatever this package does, because
// campaigns/example/ has no art/ at all. Task 4 considered writing that
// assertion and could not: the fixture it needs is the one Task 8 creates, and
// a test built against art that does not exist yet would either be skipped or
// be this same synthetic directory under another name. Task 8 commits the art
// and owns the assertion that a map from campaigns/example/ loads with its own
// art resolved and no warnings.
//
// The .png files hold the string "fake-png" rather than image bytes: nothing
// in internal/artlib reads a picture's contents, only whether the entry
// exists, so real images would add bytes and prove nothing.
func cellarArtDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tile := range []struct{ id, sidecar string }{
		{"masonry-1", `{"format_version":1,"kind":"wall","material":"stone"}`},
		{"earth-1", `{"format_version":1,"kind":"floor","material":"earth"}`},
		{"flagstone-1", `{"format_version":1,"kind":"floor","material":"stone"}`},
	} {
		write(tile.id+".json", tile.sidecar)
		write(tile.id+".png", "fake-png")
	}
	// A door has TWO pictures and no third (spec §3.4), named by its sidecar.
	write("cellar-door.json", `{"format_version":1,"kind":"door","material":"wood",
		"open":"cellar-door-open.png","closed":"cellar-door-closed.png"}`)
	write("cellar-door-open.png", "fake-png")
	write("cellar-door-closed.png", "fake-png")
	// Object art needs no sidecar (spec §3.4's asymmetry).
	for _, obj := range []string{"pillar-stone", "crate-wood", "barrel", "brazier"} {
		write(obj+".png", "fake-png")
	}
	return dir
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

	// artDir is the campaign's flat art/ directory, wired into every fixture
	// (see newMapFixtureWith) and exposed here so a test can install a piece
	// of art DURING the session — which is the whole of
	// 2026-09-02-art-is-a-flat-library design spec §3.6, and the only way to
	// exercise an art failure that arrives after boot.
	artDir string
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
	return newMapFixtureAt(t, withMaps, installable, cellarArtDir(t))
}

// newMapFixtureAt is newMapFixtureWith with the art directory chosen by the
// caller, for the one thing no well-formed art directory can express: an art
// ROOT that exists and cannot be opened. That is not one piece failing, it is
// every piece failing, and it is the third answer artlib.ErrArtDirUnreadable
// exists to carry.
func newMapFixtureAt(t *testing.T, withMaps, installable bool, artDir string) *mapFixture {
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
	// Wired unconditionally, present or not, for the reason the maps
	// directory is: a server that only has an art directory when something
	// else is also configured is the boot-order shape this sub-project exists
	// to delete. An empty/absent art/ resolves nothing and refuses nothing.
	srv = srv.WithArtDir(artDir)
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
		artDir:     artDir,
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

// installArt writes one piece of art into dir DURING a session, the way a DM
// installs art: by putting files where the campaign keeps them, with no
// restart and nothing told to reload (2026-09-02-art-is-a-flat-library design
// spec §3.6). sidecar is the whole JSON, so a test can install a BROKEN piece
// as easily as a good one; the picture is always written, because a sidecar
// with no picture beside it is a different failure (absence) and would make a
// refusal test pass for the wrong reason.
func installArt(t *testing.T, dir, id, sidecar string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(sidecar), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".png"), []byte("fake-png"), 0o600); err != nil {
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
// unreadable, unparseable, invalid, mismatched, art that cannot be parsed, art
// that cannot be read past its open),
// because the defect was never about one error — it was about which errors
// were forwarded, and that is all of them but one.
//
// THE ART CASE IS THE NEWEST AND WAS THE MOST NEARLY MISSED. The old "pack not
// loaded" row left with the mechanism that produced it
// (2026-09-02-art-is-a-flat-library Task 3), and round 1 of that task removed
// it without a substitute — reasoning, in a comment that named a function
// (artDirNotWiredYet) it had itself already deleted, that no art refusal was
// reachable through this handler. It was: mapByID and handleLoadMap both
// resolve against s.artDir, which newMapFixtureWith wires unconditionally.
// The row below is the substitute, and f.artDir joins the paths this test
// forbids in an answer, because an art error is exactly as capable of carrying
// the server's layout as a map error is.
func TestNoRefusalTellsAClientWhereTheCampaignLives(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	installMap(t, f.mapsDir, "mismatch", oneSquareMap("some-other-id"))
	installMap(t, f.mapsDir, "typo", `{"format_version":1,"id":"typo","name":"Level Two",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stoen"}}`)
	installMap(t, f.mapsDir, "truncated", `{"format_version":1,"id":"truncated",`)
	// An art case, restored. A sidecar declaring a format this server does not
	// understand is the art failure that still REFUSES (an absent piece, and
	// an art directory that cannot be opened, both degrade), and it is the one
	// that carries an artlib error all the way to a client.
	installArt(t, f.artDir, "broken-stone", `{"format_version":99,"kind":"wall"}`)
	installMap(t, f.mapsDir, "brokenart", `{"format_version":1,"id":"brokenart","name":"Level Two",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"broken-stone"}}`)
	// The READ phase, on a real seat. A DIRECTORY named <id>.json: openat
	// succeeds, read(2) returns EISDIR, and Root.ReadFile's error after a
	// successful open carries the ABSOLUTE path (internal/artlib's bareCause
	// has the per-phase table). This exact shape reached a DM verbatim through
	// this handler until review round 3 of art-is-a-flat-library Task 3, and
	// load_adventure hands the same thing to a RoleAgent seat. The unit tests
	// in internal/artlib and internal/mapdef both guard it now; this row is
	// the end-to-end one, because that is where it was actually observed.
	if err := os.MkdirAll(filepath.Join(f.artDir, "dir-stone.json"), 0o750); err != nil {
		t.Fatal(err)
	}
	installMap(t, f.mapsDir, "unreadableart", `{"format_version":1,"id":"unreadableart","name":"Level Two",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"dir-stone"}}`)
	if err := os.MkdirAll(filepath.Join(f.mapsDir, "adir.json"), 0o750); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, id string }{
		{"not installed", "nowhere"},
		{"name too long", strings.Repeat("z", 260)},
		{"filename disagrees with id", "mismatch"},
		{"tile name typo", "typo"},
		{"truncated json", "truncated"},
		{"art whose sidecar cannot be read", "brokenart"},
		{"art whose sidecar cannot be read past its open", "unreadableart"},
		{"a directory where a map should be", "adir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sendCommand(t, conn, loadMapCmdFor(tc.id))
			res := readResult(t, conn)
			if res.Ok {
				t.Fatalf("id %q loaded; every fixture here is broken", tc.id)
			}
			if strings.Contains(res.Error, f.mapsDir) || strings.Contains(res.Error, f.artDir) ||
				strings.Contains(res.Error, os.TempDir()) {
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

// TestNoWarningTellsAClientWhereTheCampaignLives is the same promise as the
// test above, on the channel that did not exist when that promise was written.
// Task 2 of 2026-09-02-art-is-a-flat-library added CommandResult.warnings
// (field 5), and Task 3 turned three art failures into warnings — so an ok=true
// result now carries text produced by exactly the code whose errors the test
// above forbids from naming a path, to exactly the same seats.
//
// EVERY WARNING SENTENCE THE ART PATH CAN PRODUCE IS DRIVEN HERE, across two
// subtests, and it took a review finding to get there (F3, 2026-09-03): the
// first version drove only the unopenable-root sentence and left the other four
// — artNotInstalled, the no-sidecar sentence, the kind-mismatch sentence and
// ResolveObjectArt's not-installed sentence — with no guard at the wire at all.
// None of them can leak today. Nothing pinned that they would not.
//
// TWO SUBTESTS BECAUSE ONE ART DIRECTORY CANNOT PRODUCE BOTH SETS: against an
// unopenable root EVERY lookup returns ErrArtDirUnreadable, so the four
// resolve-time sentences are unreachable until the root opens.
//
// THE ART ROOT IS THE SHAPE THE REFUSAL TEST ABOVE CANNOT COVER. Task 4's brief
// asked for an unopenable art root to be restored as a refusal row there; it
// cannot be one, because Patrik's ruling of 2026-09-03 made that condition
// DEGRADE at request time (internal/mapdef's Resolve — a DM cannot chmod a
// directory from a browser). The disclosure surface moved with the verdict: an
// unopenable root is now the loudest thing a load_map can say without refusing,
// and os.OpenRoot's own error is an *fs.PathError holding the absolute path.
// internal/artlib's artDirUnreadableWarning is a constant with nothing
// interpolated into it precisely so that path cannot ride out.
//
// FAULT-INJECTION PROOF (these assertions are after-the-fact, per CLAUDE.md
// rule 1). Appending artDir to the two warnings mapdef.Resolve and
// mapdef.ResolveObjectArt build for an unreadable root — the change that
// reintroduces the leak — fails the first subtest. It also fails
// internal/mapdef's TestNoArtFailureNamesTheDirectoryItRead, which is the
// unit-level guard on the same sentences and NOT what this duplicates: what
// only this can see is the sentence arriving on a CommandResult that a real
// seat reads, which is where the leak was actually observed in Task 3 and the
// reason the refusal test above keeps its own end-to-end art rows. Recorded in
// the Task 4 report.
func TestNoWarningTellsAClientWhereTheCampaignLives(t *testing.T) {
	t.Run("the art root cannot be opened", func(t *testing.T) {
		root := t.TempDir()
		// A plain FILE where art/ belongs, rather than a mode: a permissions
		// fixture passes trivially for a process running as root, and CI
		// containers often do.
		artDir := filepath.Join(root, "art")
		if err := os.WriteFile(artDir, []byte("a plain file where art/ belongs"), 0o600); err != nil {
			t.Fatal(err)
		}
		f := newMapFixtureAt(t, false, true, artDir)
		conn := f.dial(f.dmToken, 0)

		// Two overrides and an object: BOTH producers that can fire against an
		// unreadable root, so a fix that closes one sentence and leaves the
		// other is caught.
		installMap(t, f.mapsDir, "cellar", `{"format_version":1,"id":"cellar","name":"Cellar",
			"grid_width":2,"grid_height":1,"tiles":{"0,0":"stone","1,0":"stone"},
			"overrides":{"0,0":"masonry-1","1,0":"earth-1"},
			"objects":[{"id":"p1","kind":"pillar","art":"pillar-stone","at":[0,0],"size":[1,1]}]}`)

		sendCommand(t, conn, loadMapCmdFor("cellar"))
		res := readResult(t, conn)
		if !res.GetOk() {
			t.Fatalf("load_map: %s — an art root that cannot be opened degrades every square "+
				"at request time, it does not refuse the map (Patrik's ruling, 2026-09-03)",
				res.GetError())
		}
		if len(res.GetWarnings()) == 0 {
			t.Fatal("no warnings: every reference in this map dropped, and a silent degrade " +
				"is the failure spec §4 designs the warning to prevent")
		}
		assertNoWarningNamesAPath(t, res.GetWarnings(), artDir, f.mapsDir, root)
	})

	t.Run("art that resolves — every other warning the art path has", func(t *testing.T) {
		f := newInstallableMapFixture(t)
		conn := f.dial(f.dmToken, 0)

		// One square per producer, against the REAL cellarArtDir:
		//   masonry-1    declares kind "wall" over a "stone" floor -> mismatch
		//   no-such-art  is installed nowhere                      -> not installed
		//   pillar-stone is a picture with no sidecar              -> no sidecar
		//   the object's art is installed nowhere                  -> object not installed
		installMap(t, f.mapsDir, "cellar", `{"format_version":1,"id":"cellar","name":"Cellar",
			"grid_width":4,"grid_height":1,
			"tiles":{"0,0":"stone","1,0":"stone","2,0":"stone","3,0":"stone"},
			"overrides":{"0,0":"masonry-1","1,0":"no-such-art","2,0":"pillar-stone"},
			"objects":[{"id":"o1","kind":"pillar","art":"no-such-object","at":[3,0],"size":[1,1]}]}`)

		sendCommand(t, conn, loadMapCmdFor("cellar"))
		res := readResult(t, conn)
		if !res.GetOk() {
			t.Fatalf("load_map: %s — every one of these degrades", res.GetError())
		}
		// EACH PRODUCER MUST ACTUALLY HAVE FIRED. Without this the path
		// assertion below goes vacuous the moment a sentence stops being
		// produced, which is the degenerate-fixture failure this repo keeps
		// finding: a guard over an empty set passes.
		said := strings.Join(res.GetWarnings(), "\n")
		for _, want := range []string{"masonry-1", "no-such-art", "pillar-stone", "no-such-object"} {
			if !strings.Contains(said, want) {
				t.Fatalf("warnings %q, want one naming %q — this test guards the sentence "+
					"that reference produces, and cannot guard one nobody built",
					res.GetWarnings(), want)
			}
		}
		assertNoWarningNamesAPath(t, res.GetWarnings(), f.artDir, f.mapsDir)
	})
}

// assertNoWarningNamesAPath is the promise itself, in one place because two
// subtests make it and a third will. os.TempDir() is always forbidden and is
// the broad net: every fixture path in this package sits under it, so a path
// this call site forgot to name is still caught.
func assertNoWarningNamesAPath(t *testing.T, warnings []string, forbidden ...string) {
	t.Helper()
	for _, w := range warnings {
		for _, path := range append(forbidden, os.TempDir()) {
			if strings.Contains(w, path) {
				t.Errorf("a client was told where the campaign lives:\n  %s", w)
				break
			}
		}
	}
}

// TestAMapWithUnreadableArtIsRefusedOnDemandExactlyAsAtBoot pins the argument
// mapByID passes to mapdef.LoadInstalled, and it exists because nothing did:
// fault injection during review of art-is-a-flat-library Task 3 replaced
// s.artDir with "" at that call site and the ENTIRE gateway suite still
// passed.
//
// WHAT "" COSTS IS THE SHAPE OF THE ANSWER, which is subtler than it first
// looks and is why this test asserts the message rather than just the refusal.
// handleLoadMap compiles again, against the real s.artDir, immediately after
// mapByID returns — so the map is still refused either way and a test that
// checked only ok=false would pass under the mutant. What changes is WHO
// refuses it. Measured under the injection: the DM receives
//
//	artlib: art/cave-floor-1.json: field "format_version": declares 99; this server understands 1
//
// with no map named anywhere in it, because LoadInstalled is the layer that
// wraps every failure as `map "level-5" (maps/level-5.json): …` and its dry run
// was handed nothing to fail on. mapByID has also cached, as loadable, a map
// that is not. Boot says the first sentence and on-demand says the second:
// design spec §12 asks for one function so the two cannot disagree, and handing
// that one function two different art roots defeats it as thoroughly as writing
// two functions would.
//
// The art is installed AFTER boot on purpose — that is the case a boot-time
// check can never cover, and the one §3.6 exists for.
func TestAMapWithUnreadableArtIsRefusedOnDemandExactlyAsAtBoot(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	installArt(t, f.artDir, "cave-floor-1", `{"format_version":99,"kind":"floor"}`)
	installMap(t, f.mapsDir, "level-5", `{"format_version":1,"id":"level-5","name":"Level Five",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"cave-floor-1"}}`)

	sendCommand(t, conn, loadMapCmdFor("level-5"))
	res := readResult(t, conn)
	if res.GetOk() {
		t.Fatal("a map whose art sidecar cannot be read loaded on demand; the boot walk " +
			"refuses it, so this would let a map load at the table and then refuse to boot")
	}
	if !strings.Contains(res.GetError(), "cave-floor-1") {
		t.Errorf("error = %q, want it to name the art that could not be read", res.GetError())
	}
	// The boot-identical shape: mapdef.LoadInstalled is the layer that names
	// the map, and it can only name it if it was given the art root to fail on.
	for _, want := range []string{`map "level-5"`, "maps/level-5.json"} {
		if !strings.Contains(res.GetError(), want) {
			t.Errorf("error = %q, want it to contain %q — the on-demand refusal must be the "+
				"one mapdef.LoadInstalled produces at boot, not a bare artlib error with no "+
				"map in it", res.GetError(), want)
		}
	}
}

// TestArtInstalledAfterBootDrawsWithoutARestart is the positive half, and
// without it the test above is satisfied by a server that refuses everything.
// It is deliberately NOT the composeServer-driven proof Task 4 owns
// (TestArtInstalledAfterBootIsFoundWithoutARestart, which is what can actually
// see a boot-order defect): this one pins only that nothing is cached between
// loads at this layer — the art did not exist when the fixture booted, and the
// wire carries it anyway.
func TestArtInstalledAfterBootDrawsWithoutARestart(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)
	agentConn := f.dial(f.agentToken, 0)

	installArt(t, f.artDir, "late-stone", `{"format_version":1,"kind":"wall","material":"stone"}`)
	installMap(t, f.mapsDir, "hall", `{"format_version":1,"id":"hall","name":"Hall",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone-wall"},
		"overrides":{"0,0":"late-stone"}}`)

	sendCommand(t, conn, loadMapCmdFor("hall"))
	res := readResult(t, conn)
	if !res.GetOk() {
		t.Fatalf("load_map: %s", res.GetError())
	}
	if len(res.GetWarnings()) != 0 {
		t.Fatalf("warnings %q: the art WAS installed, before the load and after the boot",
			res.GetWarnings())
	}
	sceneEnv := readEvent(t, agentConn)
	sc := sceneEnv.GetSceneCreated()
	if sc == nil {
		t.Fatalf("first batch envelope = %v, want a SceneCreated", sceneEnv)
	}
	if got := sc.GetTiles()["0,0"]; got.GetArt() != "late-stone" {
		t.Fatalf("tiles[0,0] = %+v, want art late-stone", got)
	}
}

// TestAPackInstalledAfterBootSaysToRestart is GONE, and its absence is the
// requirement now. It pinned I2: packs were read once at startup, so a map
// naming a pack installed since then was refused, and this handler was the
// only layer that knew a restart was the remedy — so the message had to say
// so, name the pack, and not claim none was given.
//
// 2026-09-02-art-is-a-flat-library deleted the seam rather than the message.
// Art is read from a directory when the map is loaded (that plan's design
// spec §3.6), so nothing is ever "installed but not loaded" and no answer
// here has a restart to suggest. The property that replaces it is the exact
// inversion — art installed after boot is FOUND, without a restart — and it
// is Task 4's TestArtInstalledAfterBootIsFoundWithoutARestart, driven through
// composeServer rather than a constructed Server, because the boot-order
// defect this whole sub-project removes was invisible to a bare Server value.

// --- 2026-09-02-art-is-a-flat-library Task 2 --------------------------------

// TestALoadMapWarningReachesTheIssuer pins the channel Task 2 of
// 2026-09-02-art-is-a-flat-library added: a command that SUCCEEDED can still
// carry non-fatal facts back to whoever issued it (CommandResult.warnings,
// field 5).
//
// ITS DOC COMMENT WAS WRITTEN BEFORE TASK 3 AND SURVIVED IT UNCORRECTED, which
// is the failure it now records. It said Task 3 "has not landed", that Resolve
// "still takes a *Pack today", and that an override with no pack REFUSES via a
// `p == nil` arm — all three untrue since Task 3, and the last one names a
// branch that no longer exists. Found in review, 2026-09-03.
//
// WHAT IT DRIVES IS UNCHANGED, and Task 3's plan kept the mechanism on purpose:
// an override whose ART declares a kind disagreeing with its square's base tile
// WARNS rather than refuses (resolve.go, "an illusory wall is legitimate
// dungeon craft ... and refusing it would forbid a feature one arc away"). It
// is now one of four warning producers rather than the only one — art that is
// not installed, art with no sidecar, and an unreadable art directory all warn
// too, and any of them would do here.
//
// The mechanism now depends on cellarArtDir (above) declaring masonry-1 as
// kind "wall", which is what campaigns/example/packs/cellar-basics/pack.json
// declared and what campaigns/example/art/masonry-1.json must declare when
// Task 8 commits it. If that fixture's kinds are ever softened, this test goes
// quietly green-for-nothing rather than failing.
func TestALoadMapWarningReachesTheIssuer(t *testing.T) {
	// An installable fixture is enough now: art comes from WithArtDir, which
	// newMapFixtureWith wires unconditionally, so nothing has to be loaded at
	// boot for "shrine" to resolve. This used to be newMapFixtureWith(t, true,
	// true) so that cellar-basics was loaded as a PACK at boot — packs were
	// boot-time only and an override could not resolve without one. Neither
	// half of that is true any more.
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	// "stone" is a floor (standard.go's standardTiles); masonry-1 is a wall
	// (cellarArtDir's sidecar for it). The override supplies ART, never NATURE
	// (resolve.go's own doc comment on Resolve), so this square keeps being a
	// floor and loads anyway — it only warns that its art disagrees.
	installMap(t, f.mapsDir, "shrine", `{"format_version":1,"id":"shrine","name":"Shrine",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"masonry-1"}}`)

	sendCommand(t, conn, loadMapCmdFor("shrine"))
	res := readResult(t, conn)
	if !res.Ok {
		t.Fatalf("load_map refused: %s — a kind mismatch warns, it does not refuse", res.Error)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("no warnings: the mismatch was accepted silently and nobody was told why")
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "masonry-1") {
		t.Fatalf("warnings %q must name the reference that mismatched", res.Warnings)
	}
}
