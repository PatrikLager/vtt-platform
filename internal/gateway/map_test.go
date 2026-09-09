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
// committed campaigns/example/maps/cellar.json (Task 3 of the 2026-09-01
// create_scene-leaves plan split what used to be one maps/cellar directory
// into a flat map file and a sibling pack tree; Task 5 of the same plan moved
// both under campaigns/example/, once maps stopped being server-wide
// --maps-dir content and became a campaign's own; 2026-09-02-art-is-a-flat-
// library Task 7 deleted the pack tree) — internal/mapdef's own tests
// already cover Load/Compile's correctness in isolation; this file proves
// the WIRING, not the loader. Mirrors adventure_test.go's own shape;
// load_adventure/handleLoadAdventure is this handler's direct template.

import (
	"context"
	"encoding/json"
	"fmt"
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

// cellarMapPath resolves the committed campaigns/example/maps/cellar.json,
// relative to this test file's own package directory — the same "../../<path>"
// convention adventure_test.go's goblinAmbushDir establishes.
//
// It had a sibling, cellarPackDir, pointing at
// campaigns/example/packs/cellar-basics, until 2026-09-02-art-is-a-flat-library
// Task 7 deleted both the pack and that directory. Art comes from cellarArtDir
// below, and from campaigns/example/art/ once that plan's Task 8 commits it.
func cellarMapPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "campaigns", "example", "maps", "cellar.json")
}

// loadCellarMap loads the real committed campaigns/example/maps/cellar.json,
// failing the test loudly if it does not load — a broken fixture here would
// silently turn every test in this file into a no-op, which is worse than a
// compile error.
func loadCellarMap(t *testing.T) *mapdef.Map {
	t.Helper()
	m, err := mapdef.Load(cellarMapPath(t))
	if err != nil {
		t.Fatalf("mapdef.Load(campaigns/example/maps/cellar.json): %v", err)
	}
	return m
}

// cellarArtDir builds, in a temp directory, exactly the art
// campaigns/example/maps/cellar.json names — the four tile pieces its
// overrides reference and the four object pictures its objects do — in the
// flat layout 2026-09-02-art-is-a-flat-library design spec §3 defines: a
// picture per piece, and beside it a sidecar for tile art only (§3.4; tile art
// with no sidecar draws plain and warns, which this fixture does not want to
// be testing by accident — this clause said "mapdef.Resolve refuses" it, true
// until Patrik's ruling of 2026-09-03).
//
// Built here rather than read from campaigns/example/art/, which does not
// exist yet: Task 8 of that plan migrates the committed fixture, and until it
// does this is the only way to assert that an override's art reaches the wire
// without weakening the assertion. The kinds and materials are copied from
// campaigns/example/packs/cellar-basics/pack.json so no square picks up a
// spurious kind-mismatch warning, taken from the manifest while it still
// existed — Task 7 deleted it along with the pack.
//
// THE SHIPPED CAMPAIGN'S OWN ART IS ASSERTED SEPARATELY, and this fixture is
// still the right one for everything else here. Task 4 could not write that
// assertion — campaigns/example/ had no art/ at all, so every override in the
// shipped campaign degraded whatever this package did — and Task 8 created the
// directory and wrote it: TestTheShippedCampaignResolvesItsOwnArt below, which
// uses shippedArtDir rather than this. What this one keeps is the ability to
// build art a committed campaign does not have, which is most of this file.
//
// KEEPING BOTH IS DELIBERATE. This directory can hold a broken piece, a
// half-installed one, or a piece that arrives mid-session; the committed one
// must stay well-formed, because it is what a DM copies. A test that needs a
// failure builds it here.
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

// shippedArtDir resolves the committed campaigns/example/art — the art the
// demo campaign actually ships, in the same "../../<path>" convention
// cellarMapPath uses.
//
// It is a REAL DIRECTORY handed to a real server, not a copy: the point of the
// one test that uses it is that the bytes in this repository resolve, so
// copying them into a temp dir first would only prove that a copy of them does.
// Nothing writes to it — installArt takes an explicit dir, and every test that
// installs art passes a temp one.
func shippedArtDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "campaigns", "example", "art")
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
		m := loadCellarMap(t)
		srv = srv.WithMaps(map[string]*mapdef.Map{m.ID: m})
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

// TestTheShippedCampaignResolvesItsOwnArt is the assertion Tasks 4 and 8 passed
// between them, and the first time anything in this tree has run the demo
// campaign's own art through the code that serves it.
//
// EVERY OTHER ART TEST BUILDS ITS FIXTURE. cellarArtDir writes a synthetic
// directory whose kinds and materials were copied out of the pack manifest by
// hand, so TestLoadMapProducesBatchCarryingTilesAndObjects proves the WIRING and
// says nothing about the files this repository ships. Until Task 8 there were no
// such files: campaigns/example/ had maps/ and nothing else, so every square of
// the demo drew from the built-in vocabulary and the shipped `overrides` were
// ninety warnings waiting to happen. Nobody would have found that out from a
// green suite.
//
// NO WARNINGS IS THE WHOLE ASSERTION, and it is stronger than it looks. spec §4
// makes every art failure degrade, so a campaign whose art is missing, misnamed,
// wrong-cased, half-installed or wrong about a square's nature still loads with
// ok=true — the ONLY difference between that and a working demo is this list
// being empty. An assertion on the tiles alone would pass with the art deleted,
// because the kind and material come from m.Tiles either way.
//
// IT ALSO COVERS THE DOOR, which has never met real traffic: cellar-door is the
// one piece with two pictures and no <id>.png, and every test that has exercised
// that path used a hand-written sidecar. Here the sidecar is the one
// tools/genmappack wrote and the pictures are the ones it drew.
func TestTheShippedCampaignResolvesItsOwnArt(t *testing.T) {
	f := newMapFixtureAt(t, true, false, shippedArtDir(t))
	dmConn := f.dial(f.dmToken, 0)
	// The agent seat, for the reason TestLoadMapProducesBatchCarryingTilesAndObjects
	// gives: a map is terrain a player has not walked into, and the visibility
	// projection withholds it.
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
	readEvent(t, agentConn)

	sendCommand(t, dmConn, loadMapCmdFor("cellar"))
	res := readResult(t, dmConn)
	if !res.Ok {
		t.Fatalf("load_map cellar against campaigns/example/art: %s", res.Error)
	}
	if len(res.GetWarnings()) != 0 {
		t.Fatalf("the shipped campaign warns on its own art: %q. Every one of these is a "+
			"square that draws plain at the table, and the demo is the thing anybody "+
			"looks at first", res.GetWarnings())
	}

	sceneEnv := readEvent(t, agentConn)
	sc := sceneEnv.GetSceneCreated()
	if sc.GetSceneId() != "cellar" {
		t.Fatalf("first batch envelope is %q, want the cellar SceneCreated", mapPayloadKind(sceneEnv))
	}

	// Read what the map file itself declares rather than a second copy of it:
	// cellar.json overrides every one of its 90 squares today, and a hardcoded
	// count here would be a number that rots the first time somebody edits the
	// map.
	m := loadCellarMap(t)
	if len(m.Overrides) == 0 || len(m.Objects) == 0 {
		t.Fatalf("cellar.json declares %d overrides and %d objects; with either at zero "+
			"this test asserts nothing", len(m.Overrides), len(m.Objects))
	}
	for square, want := range m.Overrides {
		got := sc.GetTiles()[square]
		if got.GetArt() != want {
			t.Errorf("tiles[%s].art = %q, want %q — the map names it and art/ has it",
				square, got.GetArt(), want)
		}
	}
	for _, o := range m.Objects {
		var got *vttv1.SceneObject
		for _, candidate := range sc.GetObjects() {
			if candidate.GetObjectId() == o.ID {
				got = candidate
			}
		}
		if got == nil {
			t.Errorf("objects missing %s", o.ID)
			continue
		}
		if got.GetArt() != o.Art {
			t.Errorf("objects[%s].art = %q, want %q", o.ID, got.GetArt(), o.Art)
		}
	}

	// The door, named on its own because it is the piece with two pictures and
	// no <id>.png, and because its NATURE must still come from m.Tiles: art
	// never decides what a square is.
	door := sc.GetTiles()["5,4"]
	if door.GetArt() != "cellar-door" || door.GetKind() != "door" || door.GetMaterial() != "wood" {
		t.Errorf("tiles[5,4] = %+v, want art=cellar-door kind=door material=wood", door)
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
	// The DM-FACING TRANSLATION of engine.ErrSceneExists. handleLoadMap emits
	// this sentence only on errors.Is against that sentinel, and only the fold
	// produces it — so asserting the translated wording still pins that the
	// BACKSTOP refused, which is what this test exists to prove, rather than
	// merely that something somewhere said no.
	if !strings.Contains(r2.Error, "already in play") {
		t.Fatalf("error = %q, want the collision refusal", r2.Error)
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
// always refused — the scene id is taken, surfaced to the DM as "already in
// play" by handleLoadMap's engine.ErrSceneExists arm — never a torn connection, a
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
		if !strings.Contains(res.Error, "already in play") {
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
// unreadable, unparseable, invalid, mismatched, and art written for a format
// this server does not understand), because the defect was never about one
// error — it was about which errors were forwarded, and that is all of them
// but one.
//
// THE READ-PHASE ART ROW LEFT ON 2026-09-04 AND DID NOT STOP MATTERING. A
// DIRECTORY named <id>.json — openat succeeds, read(2) returns EISDIR, and
// Root.ReadFile's error after a successful open carries the ABSOLUTE path
// (internal/artlib's bareCause has the per-phase table) — was a row here until
// Patrik's ruling of that day made a sidecar this server cannot read DEGRADE
// rather than refuse. The disclosure surface moved with the verdict, exactly as
// the unopenable root's did a day earlier: that shape now travels on
// CommandResult.warnings, to the same seats, and
// TestNoWarningTellsAClientWhereTheCampaignLives below carries it. It is the
// same fixture and the same absolute path; only the channel changed.
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
	// The one art case left. A sidecar declaring a format this server does not
	// understand is the art failure that still REFUSES (an absent piece, an art
	// directory that cannot be opened, and since 2026-09-04 every sidecar this
	// server simply cannot read, all degrade), and it is the one that carries
	// an artlib error all the way to a client.
	installArt(t, f.artDir, "broken-stone", `{"format_version":99,"kind":"wall"}`)
	installMap(t, f.mapsDir, "brokenart", `{"format_version":1,"id":"brokenart","name":"Level Two",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"broken-stone"}}`)
	if err := os.MkdirAll(filepath.Join(f.mapsDir, "adir.json"), 0o750); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ name, id string }{
		{"not installed", "nowhere"},
		{"name too long", strings.Repeat("z", 260)},
		{"filename disagrees with id", "mismatch"},
		{"tile name typo", "typo"},
		{"truncated json", "truncated"},
		{"art written for a format this server does not understand", "brokenart"},
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
// subtests, and it has taken two review findings to make that sentence true
// (F3 on 2026-09-03, F1 on 2026-09-05). The first version drove only the
// unopenable-root sentence and left four others with no guard at the wire at
// all; the second added artCannotBeUsed on the TILE arm and missed that
// mapdef.ResolveObjectArt builds the same sentence from its own switch.
//
// THERE ARE EIGHT TEMPLATES, and the count is worth stating because a reader
// consults exactly this sentence before deciding whether to widen the table.
// mapdef.Resolve builds five — not installed, no sidecar, the unreadable art
// root, a kind mismatch, and cannot-be-used — and mapdef.ResolveObjectArt
// builds three, its own not-installed, unreadable-root and cannot-be-used.
// Only the two cannot-be-used sentences interpolate an artlib ERROR; the
// unreadable-root pair is a constant with nothing interpolated at all (which
// is why "the others carry an art name and nothing else" was false of it); the
// rest carry an art name, and the mismatch also carries two kinds.
//
// THE TWO cannot-be-used SENTENCES ARE THE DANGEROUS ONES, because they carry
// text produced by the same code whose errors the refusal test above forbids
// from naming a path. Both dir-stone rows below are the READ phase: a DIRECTORY
// named <id>.json, where openat succeeds, read(2) returns EISDIR, and
// Root.ReadFile's error carries the ABSOLUTE campaign path. That exact shape
// reached a DM verbatim through this handler until review round 3 of
// art-is-a-flat-library Task 3; it was a REFUSAL row in the test above until
// Patrik's ruling of 2026-09-04 made it degrade, and it is here because the
// leak moved channel rather than closing.
//
// A TILE ROW DOES NOT COVER THE OBJECT ARM. Measured 2026-09-05: appending
// artDir to ONLY ResolveObjectArt's tail left both of this file's path tests
// green while internal/mapdef's unit test caught it — and this test exists
// precisely because a unit test cannot see the sentence arriving on a
// CommandResult that a real seat reads. load_adventure puts these same
// warnings on a result for RoleAgent seats, which no refusal test reaches at
// all. So the object arm gets its own dir-stone row.
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
// rule 1). For the dir-stone rows: dropping bareCause from artlib's read-phase
// arm — `fmt.Errorf("artlib: art/%s%s: %w", id, sidecarExt, err)` instead of
// bareCause(err) in lookupIn's default branch, which is the one-token form of
// the leak that actually shipped — fails the second subtest with "a client was
// told where the campaign lives". Measured 2026-09-04, Task 4b.
//
// And for the OBJECT row specifically, which is the one F1 found missing:
// appending artDir to the tail mapdef.ResolveObjectArt passes artCannotBeUsed,
// and to nothing else, fails this subtest. Before that row existed the same
// injection left both of this file's path tests green. Measured 2026-09-05.
//
// Appending artDir to the two warnings mapdef.Resolve and
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

		// A DIRECTORY where a sidecar belongs: the read phase, and the one
		// artlib error whose *fs.PathError holds the absolute path.
		if err := os.MkdirAll(filepath.Join(f.artDir, "dir-stone.json"), 0o750); err != nil {
			t.Fatal(err)
		}

		// One producer per square or object, against the REAL cellarArtDir:
		//   masonry-1    declares kind "wall" over a "stone" floor -> mismatch
		//   no-such-art  is installed nowhere                      -> not installed
		//   pillar-stone is a picture with no sidecar              -> no sidecar
		//   dir-stone    is a sidecar that cannot be read          -> cannot be used
		//   o1's art is installed nowhere                          -> object not installed
		//   o2's art is dir-stone                                  -> object cannot be used
		//
		// o2 is the row F1 added. The two cannot-be-used sentences are built by
		// two different switches with nothing forcing them to agree, so the
		// tile row proves nothing about the object one.
		installMap(t, f.mapsDir, "cellar", `{"format_version":1,"id":"cellar","name":"Cellar",
			"grid_width":6,"grid_height":1,
			"tiles":{"0,0":"stone","1,0":"stone","2,0":"stone","3,0":"stone","4,0":"stone",
				"5,0":"stone"},
			"overrides":{"0,0":"masonry-1","1,0":"no-such-art","2,0":"pillar-stone",
				"3,0":"dir-stone"},
			"objects":[{"id":"o1","kind":"pillar","art":"no-such-object","at":[4,0],"size":[1,1]},
				{"id":"o2","kind":"pillar","art":"dir-stone","at":[5,0],"size":[1,1]}]}`)

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
		for _, want := range []string{"masonry-1", "no-such-art", "pillar-stone", "dir-stone",
			"no-such-object"} {
			if !strings.Contains(said, want) {
				t.Fatalf("warnings %q, want one naming %q — this test guards the sentence "+
					"that reference produces, and cannot guard one nobody built",
					res.GetWarnings(), want)
			}
		}
		// BY NAME IS NOT ENOUGH FOR dir-stone, which two producers now name: a
		// square and an object. Only the object tail tells them apart, so
		// without this the object arm could stop producing anything and the
		// loop above would still pass on the tile sentence alone — the
		// degenerate-fixture failure this repo keeps finding.
		var objectCannotBeUsed bool
		for _, w := range res.GetWarnings() {
			if strings.Contains(w, "cannot be used") && strings.Contains(w, "the object stays") {
				objectCannotBeUsed = true
			}
		}
		if !objectCannotBeUsed {
			t.Fatalf("warnings %q carry no OBJECT cannot-be-used sentence; "+
				"mapdef.ResolveObjectArt builds it from its own switch and this is the "+
				"only test that sees it on a CommandResult", res.GetWarnings())
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

// TestAMapWithArtFromALaterFormatIsRefusedOnDemandExactlyAsAtBoot pins the argument
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
//
// THE FIXTURE IS NARROWER THAN THE NAME USED TO SAY. It was
// TestAMapWithUnreadableArtIsRefusedOnDemandExactlyAsAtBoot, and a sidecar
// that cannot be read stopped refusing anything on 2026-09-04 (Task 4b) — it
// degrades one square now, so this test would have gone green-for-nothing
// under any other broken sidecar. format_version 99 was already the fixture,
// so the mechanism this pins is unchanged; only the name and the sentences
// around it were describing a wider class than still exists.
func TestAMapWithArtFromALaterFormatIsRefusedOnDemandExactlyAsAtBoot(t *testing.T) {
	f := newInstallableMapFixture(t)
	conn := f.dial(f.dmToken, 0)

	installArt(t, f.artDir, "cave-floor-1", `{"format_version":99,"kind":"floor"}`)
	installMap(t, f.mapsDir, "level-5", `{"format_version":1,"id":"level-5","name":"Level Five",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"cave-floor-1"}}`)

	sendCommand(t, conn, loadMapCmdFor("level-5"))
	res := readResult(t, conn)
	if res.GetOk() {
		t.Fatal("a map naming art written for a later format loaded on demand; the boot " +
			"walk refuses it, so this would let a map load at the table and then refuse " +
			"to boot")
	}
	if !strings.Contains(res.GetError(), "cave-floor-1") {
		t.Errorf("error = %q, want it to name the art it refused", res.GetError())
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
// is now one of five warning producers rather than the only one — art that is
// not installed, art with no sidecar, art that is installed and cannot be read,
// and an unreadable art directory all warn too, and any of them would do here.
//
// The mechanism now depends on cellarArtDir (above) declaring masonry-1 as
// kind "wall", which is what the deleted cellar-basics manifest declared and
// what campaigns/example/art/masonry-1.json must declare when Task 8 commits
// it. If that fixture's kinds are ever softened, this test goes
// quietly green-for-nothing rather than failing.
func TestALoadMapWarningReachesTheIssuer(t *testing.T) {
	// An installable fixture is enough now: art comes from WithArtDir, which
	// newMapFixtureWith wires unconditionally, so nothing has to be loaded at
	// boot for "shrine" to resolve. This used to be newMapFixtureWith(t, true,
	// true) so that cellar-basics was loaded as a PACK at boot — packs were
	// boot-time only and an override could not resolve without one. Neither
	// half of that is true any more, and since Task 7 there is no pack to
	// load.
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

// TestASecondLoadTellsTheDMTheMapIsLoadedAndHowToReload is the message half of
// the collision TestLoadMapDoubleLoadCollisionRejectedCleanNotPoisoned proves
// the mechanics of. The refusal is correct and stays correct; what this pins is
// that it is ADDRESSED TO A DM.
//
// The path that makes this matter is ordinary, not exotic. Art that is not
// installed DEGRADES the square and lets the load commit (design spec §4), so a
// DM who mistypes an override, or copies a map before its pictures, gets a
// warning rather than a refusal — and the map is now in the log. Installing the
// picture and loading again is the obvious next move, and it cannot work: the
// scene id is taken, by that same first load. Handing them
// `engine: scene "cellar" already exists` at that moment names a layer they do
// not work in and a word ("scene") they did not type, while the thing they
// changed was art. The remedy — load a copy under a new map id — is not
// guessable from it.
//
// Asserted on the DM-facing sentence rather than on any substring the engine
// happens to share, so that a future change to the fold's own wording cannot
// quietly make this pass while the DM reads something else.
func TestASecondLoadTellsTheDMTheMapIsLoadedAndHowToReload(t *testing.T) {
	f := newMapFixture(t, true)
	conn := f.dial(f.dmToken, 0)

	// The cellar map places a token for this actor, and engine.Apply refuses a
	// TokenPlaced naming an actor the world does not have — so the FIRST load
	// only succeeds once the actor exists.
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
	// Drain the first load's own broadcast; see the double-load test above for
	// why an un-drained batch would starve the next CommandResult.
	for i := 0; i < 2; i++ {
		readEvent(t, conn)
	}

	sendCommand(t, conn, loadMapCmdFor("cellar"))
	r2 := readResult(t, conn)
	if r2.Ok {
		t.Fatalf("want ok=false on a second load of the same map, got %+v", r2)
	}
	// THE MAP ID, because that is what the DM typed and what they will look for.
	if !strings.Contains(r2.Error, `"cellar"`) {
		t.Errorf("error = %q, want it to name the map the DM asked for", r2.Error)
	}
	// THE REMEDY. A refusal a DM cannot act on sends them hunting a duplicate
	// that does not exist.
	if !strings.Contains(r2.Error, "already in play") {
		t.Errorf("error = %q, want it to say the scene id is already in play", r2.Error)
	}
	// AND NOT OVER-CLAIM. The sentinel knows a scene id is taken; it does not
	// know a MAP was loaded, and a loaded adventure can claim a scene id too —
	// adventures/cellar-rats and campaigns/example/maps/cellar.json both use
	// "cellar", so a first draft of this message was false on shipped content.
	if strings.Contains(r2.Error, "map \"cellar\" is already loaded") {
		t.Errorf("error = %q claims the MAP was loaded, which the sentinel does "+
			"not establish — a loaded adventure can take the scene id", r2.Error)
	}
	if !strings.Contains(r2.Error, "new id") {
		t.Errorf("error = %q, want it to name the remedy: load a copy under a new id", r2.Error)
	}
	// NOT the fold's internal phrasing. "scene" is a word the DM never typed.
	if strings.Contains(r2.Error, "engine:") {
		t.Errorf("error = %q, leaks the fold's own wording to a DM", r2.Error)
	}
}

// TestANonCollisionFailureKeepsItsOwnMessage is the discriminating half of
// handleLoadMap's engine.ErrSceneExists arm, and without it that arm is pinned
// only to fire — never to STOP firing.
//
// Measured with the arm mutated to `errors.Is(...) || err != nil`: the whole
// repository still passes. Nothing observed the difference, and check:mutation
// cannot: gremlins has no mutator for a bare boolean call in an `if`, the same
// blind spot that once hid fourteen mutants behind 100% line coverage.
//
// What the gap costs is a misdirection at the worst moment. A poisoned
// campaign, a store write that failed, or an actor collision inside the map's
// own batch would all reach the DM as "install a copy under a new id" — sending
// them to duplicate a map over a disk error.
//
// The vehicle is the cellar map's own TokenPlaced, which names act-fighter.
// Loading without seeding that actor makes engine.Apply refuse for a reason
// that has nothing to do with scene ids, on the same AppendBatch call.
func TestANonCollisionFailureKeepsItsOwnMessage(t *testing.T) {
	f := newMapFixture(t, true)
	conn := f.dial(f.dmToken, 0)

	sendCommand(t, conn, loadMapCmdFor("cellar"))
	r := readResult(t, conn)
	if r.Ok {
		t.Fatalf("want ok=false: the map places a token for an actor nobody added, got %+v", r)
	}
	// The fold's OWN sentence, untranslated.
	if !strings.Contains(r.Error, "unknown actor") {
		t.Errorf("error = %q, want the fold's own reason for this failure", r.Error)
	}
	// And emphatically not the scene-collision translation.
	if strings.Contains(r.Error, "already in play") || strings.Contains(r.Error, "new id") {
		t.Errorf("error = %q translates a NON-collision failure into the "+
			"scene-id refusal, sending a DM to copy a map over an unrelated fault", r.Error)
	}
}

// bigSidecarArtDir installs n pieces whose sidecars each carry an oversized
// author-controlled value, in the shape that reaches CommandResult.warnings:
// a kind that disagrees with the door fields beside it, which artlib reports by
// quoting the kind back.
func bigSidecarArtDir(t *testing.T, n, valueBytes int) string {
	t.Helper()
	dir := t.TempDir()
	huge := strings.Repeat("A", valueBytes)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("piece-%02d", i)
		if err := os.WriteFile(filepath.Join(dir, id+".png"), []byte("p"), 0o600); err != nil {
			t.Fatal(err)
		}
		// NO DOOR FIELDS. With them the huge kind takes artlib's door-mismatch
		// arm, which was already bounded — so the fixture proved the bound it
		// happened to route through rather than the one under test. Without
		// them the sidecar decodes cleanly and the kind travels out as
		// Piece.Kind, which resolve.go renders into its own sentence.
		body := fmt.Sprintf(`{"format_version":1,"kind":%q}`, huge)
		if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestBrokenArtCannotPushAResultPastTheReadLimit is the boundary this whole
// bounding exercise exists for, and it is stated as a frame the client can
// actually read rather than as a byte count in a comment.
//
// artlib.clip bounds author-controlled sidecar text to forty characters, and it
// was called from ONE place: the format_version arm. Every other interpolation
// passed the file's bytes through whole — measured, a single 20 KB `kind` value
// produced 20,191 bytes of warnings. Warnings collapse per art NAME, so twelve
// broken pieces is twelve of those, and adventure.Compile does not collapse them
// across scenes at all.
//
// WHAT GOES WRONG IS WORSE THAN A BIG MESSAGE. handleLoadMap has already
// committed and broadcast the scene by the time the result is built, so every
// other seat's board changes and the ISSUER's socket closes with "message too
// big" — reconnecting into a campaign that silently changed, never told why.
// Go clients only: the MCP agent seat and cmd/vtt, which set
// internal/harness's readLimit. A browser sets none and renders the block.
//
// Not remotely triggerable: authz.go gates load_map to DM and agent, and no
// route writes map or art files. This is a broken-content and third-party-bundle
// footgun, which is why it is a bound rather than a refusal.
func TestBrokenArtCannotPushAResultPastTheReadLimit(t *testing.T) {
	const pieces, valueBytes = 12, 20000 // ~240 KB unbounded, against a 200 KiB limit
	f := newMapFixtureAt(t, false, true, bigSidecarArtDir(t, pieces, valueBytes))
	conn := f.dial(f.dmToken, 0)

	tiles := make([]string, 0, pieces)
	overrides := make([]string, 0, pieces)
	for i := 0; i < pieces; i++ {
		tiles = append(tiles, fmt.Sprintf(`"%d,0":"stone-wall"`, i))
		overrides = append(overrides, fmt.Sprintf(`"%d,0":"piece-%02d"`, i, i))
	}
	installMap(t, f.mapsDir, "wide", fmt.Sprintf(
		`{"format_version":1,"id":"wide","name":"Wide","grid_width":%d,"grid_height":1,`+
			`"tiles":{%s},"overrides":{%s}}`,
		pieces, strings.Join(tiles, ","), strings.Join(overrides, ",")))

	sendCommand(t, conn, loadMapCmdFor("wide"))
	// The assertion IS that this read succeeds. Unbounded, the frame exceeds the
	// client's own read limit and the connection is torn down instead — which is
	// the failure a DM cannot see, because the scene has already been broadcast.
	r := readResult(t, conn)

	// Every square degrades and the map still loads: bounding the message must
	// not turn a degrade into a refusal.
	if !r.Ok {
		t.Fatalf("want ok=true — broken art degrades, it does not refuse: %s", r.Error)
	}
	if len(r.Warnings) == 0 {
		t.Fatal("want the DM told which pieces failed; bounding must not silence them")
	}
	total := 0
	for _, w := range r.Warnings {
		total += len(w)
	}
	// A real bound, not merely "smaller": twelve warnings that each still name
	// their piece cost a few hundred bytes apiece, nowhere near the limit.
	if total > 32*1024 {
		t.Errorf("warnings total %d bytes from %d pieces — author bytes are still "+
			"passing through substantially unbounded", total, pieces)
	}
	t.Logf("%d warnings, %d bytes total (unbounded this was ~%d)",
		len(r.Warnings), total, pieces*valueBytes)
}
