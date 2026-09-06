package main

// maps_e2e_test.go proves composeServer's real map-serving lifecycle end to
// end (maps-as-geometry Task 7; the --maps-dir flag itself is gone as of
// 2026-09-01-create-scene-leaves Task 5 — "the kernel serves maps, it does
// not make them" — maps now come from the campaign directory's own maps/,
// so these tests write straight into campaignPath instead of a
// separate operator-pointed directory) — the same concern serve_e2e_test.go's
// TestComposeServerFailsLoudlyOnAnUnreadableAdventureGuide covers for
// --adventures-dir, and the reason a wiring change like Task 5's
// (docs/superpowers/plans/2026-09-01-create-scene-leaves.md's own Task 5
// section) needs an end-to-end test of its own rather than trusting
// loadMapsDir's unit coverage alone: a change to composeServer's own call
// site is exactly the kind of coupling a package's own unit tests, run in
// isolation, cannot see. maps_test.go proves loadMapsDir's OWN logic in
// isolation; this file proves the WIRING — a real composeServer, a real
// gateway.Handler(), a real listener — actually connects a campaign's maps/
// to GET /api/maps. It connected a sibling packs/ to
// GET /api/packs/{pack}/{file} too, until 2026-09-02-art-is-a-flat-library
// Task 7 deleted both; nothing serves art bytes over the wire until Task 6 of
// that plan builds GET /api/art/{file}, and this file is where its own
// end-to-end proof belongs.

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// TestCampaignsMapsServeEndToEnd boots composeServer against a campaign
// directory holding one real map (Task 3 layout: maps are flat files under
// campaignPath/maps/, named by their own id) and drives GET /api/maps over an
// actual HTTP listener — not the gateway-package fixture, which never goes
// through composeServer/the real wiring at all.
//
// IT WAS TestCampaignsMapsAndPacksServeEndToEnd, and drove GET /api/packs/{pack}/{file}
// against a real pack directory in the same campaign, until
// 2026-09-02-art-is-a-flat-library Task 7. That half asserted the campaign's
// own installed BYTES came back over a real listener, and no test asserts that
// about anything today, because no route serves bytes: Task 6 of that plan
// builds GET /api/art/{file} and owes this file the same round trip.
func TestCampaignsMapsServeEndToEnd(t *testing.T) {
	campaignPath := t.TempDir()

	mapsSubDir := filepath.Join(campaignPath, "maps")
	if err := os.MkdirAll(mapsSubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mapsSubDir, "shrine.json"), []byte(`{
		"format_version": 1,
		"id": "shrine", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1,
		"tiles": {"0,0":"wood"},
		"overrides": {"0,0":"wood-planks-split-3"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer with maps/packs installed in the campaign: %v", err)
	}
	t.Cleanup(func() {
		if err := closeFn(); err != nil {
			t.Errorf("closeFn: %v", err)
		}
	})

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	base := "http://" + ln.Addr().String()
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	if err := waitForHealthz(base, 3*time.Second); err != nil {
		t.Fatalf("healthz never became ready: %v", err)
	}

	// campaignPath is the campaign DIRECTORY (2026-09-01-create-scene-leaves
	// Task 4); composeServer above has already created log.db inside it.
	ids, err := identity.Open(campaign.LogPath(campaignPath))
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}

	get := func(path string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, body
	}

	code, body := get("/api/maps")
	if code != http.StatusOK {
		t.Fatalf("/api/maps status = %d, want 200 (body %s)", code, body)
	}
	// Decoded into map[string]any rather than a typed shape, so the assertion
	// can see a "pack" key that should no longer be there: encoding/json
	// silently discards a field a struct has nowhere to put, so a typed decode
	// without a Pack field would pass whether the server sent one or not. This
	// asserted `pack.id == "mossy-keep"` until Task 5 of
	// 2026-09-02-art-is-a-flat-library deleted mapdef.Map.Pack — with no field
	// to key the lookup by, /api/maps has no pack reference to build.
	var got struct {
		Maps []map[string]any `json:"maps"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode /api/maps: %v (body %s)", err, body)
	}
	if len(got.Maps) != 1 || got.Maps[0]["id"] != "shrine" {
		t.Fatalf("/api/maps = %+v, want exactly one shrine map", got.Maps)
	}
	if _, present := got.Maps[0]["pack"]; present {
		t.Fatalf("/api/maps entry carries a pack reference (%v); no map declares a pack any more",
			got.Maps[0]["pack"])
	}
}

// TestComposeServerFailsLoudlyOnABrokenMap covers the OTHER end of task-7-
// brief.md's boot posture through the real composeServer path (not just
// LoadMapsDir directly, which maps_test.go already covers): one broken map
// under the campaign's own maps/ must stop composeServer from returning a
// server at all. The fixture (testdata/maps-with-one-broken) is copied into
// a fresh campaign directory via os.CopyFS rather than used as campaignPath
// directly — campaign.Open/identity.Open both WRITE into campaignPath
// (log.db, at minimum), and testdata is checked into git; writing into it
// from a test run would dirty a committed fixture.
func TestComposeServerFailsLoudlyOnABrokenMap(t *testing.T) {
	campaignPath := t.TempDir()
	if err := os.CopyFS(filepath.Join(campaignPath, "maps"), os.DirFS("testdata/maps-with-one-broken/maps")); err != nil {
		t.Fatal(err)
	}

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err == nil {
		if closeFn != nil {
			_ = closeFn()
		}
		_ = srv
		t.Fatal("composeServer succeeded with a broken map in the campaign's maps/; " +
			"the table would find out instead of us")
	}
	// "broken.json", not the weaker "broken": composeServer's own wrap
	// ("vtt serve: load maps %s: %w", campaignPath, err) embeds campaignPath
	// itself (a t.TempDir() path) in EVERY error this call can ever return,
	// so a bare "broken" substring would pass even if the cause had nothing
	// to do with the broken map. The full filename can only appear because
	// the inner error actually named that file.
	if !strings.Contains(err.Error(), "broken.json") {
		t.Errorf("error should name the offending file broken.json, got: %v", err)
	}
}

// --- 2026-09-02-art-is-a-flat-library Task 4 --------------------------------
//
// The tests below drive art through the REAL composeServer, and that is the
// whole point rather than a preference for realism. The defect design spec §1
// describes — "composeServer gates the pack load on maps/ existing, so a
// campaign with art and no map yet boots with no art at all" — was invisible to
// internal/gateway's own fixtures, because a constructed gateway.Server has
// whatever the test handed it and no boot order at all. §8 says so outright:
// "Both are exercised against a running server, not a constructed Server value
// — the §1 defect existed precisely because its test never went through
// composeServer."

// artCampaign is one composeServer-booted campaign with two seats on it: a DM
// to issue load_map, and an agent to watch the batch land. Two connections
// rather than one because the CommandResult and the SceneCreated broadcast race
// each other on a single socket, and a reader that stops at the result drops
// the envelope it was about to read (internal/gateway's map_test.go splits them
// the same way, for the same reason).
type artCampaign struct {
	t    *testing.T
	path string
	dm   *websocket.Conn
	view *websocket.Conn
}

// startArtCampaign boots campaignPath exactly as `vtt serve` does. campaignPath
// is created if it does not exist, but maps/ and art/ are NOT: whether either
// exists at boot is the variable most of these tests turn on.
func startArtCampaign(t *testing.T, campaignPath string) *artCampaign {
	t.Helper()
	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		if err := closeFn(); err != nil {
			t.Errorf("closeFn: %v", err)
		}
	})
	base := "http://" + ln.Addr().String()
	if err := waitForHealthz(base, 5*time.Second); err != nil {
		t.Fatalf("healthz never became ready: %v", err)
	}

	ids, err := identity.Open(campaign.LogPath(campaignPath))
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	dmToken, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	// RoleAgent, not a second DM: an agent seat receives the log unfiltered
	// (visibility spec §3.1), which a player seat deliberately does not — a
	// load_map batch is a room nobody has walked into yet.
	viewToken, _, err := ids.CreateInvite("Agent", identity.RoleAgent)
	if err != nil {
		t.Fatal(err)
	}

	c := &artCampaign{t: t, path: campaignPath}
	dial := func(token string) *websocket.Conn {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _, err := websocket.Dial(ctx, "ws://"+ln.Addr().String()+"/ws?token="+token+"&after=0", nil)
		if err != nil {
			t.Fatalf("ws dial: %v", err)
		}
		t.Cleanup(func() { conn.CloseNow() })
		conn.SetReadLimit(200 * 1024)
		return conn
	}
	c.dm, c.view = dial(dmToken), dial(viewToken)
	return c
}

// installArt writes one piece of tile art into the campaign's art/ DURING the
// session — the way a DM installs art, by putting files where the campaign
// keeps them, with no restart and nothing told to reload (design spec §3.6).
// Overwriting an existing stem is the same call, which is what makes it the
// "drop a better masonry-1.png over the old one" of §3.3.
func (c *artCampaign) installArt(id, sidecar string) {
	c.t.Helper()
	dir := filepath.Join(c.path, "art")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		c.t.Fatal(err)
	}
	writeFile(c.t, filepath.Join(dir, id+".json"), sidecar)
	writeFile(c.t, filepath.Join(dir, id+".png"), "fake-png")
}

// installMap writes one map file into the campaign's maps/, the same way.
func (c *artCampaign) installMap(id, body string) {
	c.t.Helper()
	dir := filepath.Join(c.path, "maps")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		c.t.Fatal(err)
	}
	writeFile(c.t, filepath.Join(dir, id+".json"), body)
}

// oneOverrideMap is a one-square map whose single square is a standard "stone"
// floor carrying art. Kept in one place so a fixture cannot quietly differ
// between tests in a way that changes which warning is produced.
func oneOverrideMap(id, art string) string {
	return `{"format_version":1,"id":"` + id + `","name":"Hall",
		"grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},
		"overrides":{"0,0":"` + art + `"}}`
}

// loadMap issues load_map as the DM and returns the result.
func (c *artCampaign) loadMap(id string) *vttv1.CommandResult {
	c.t.Helper()
	raw, err := protojson.Marshal(&vttv1.ClientCommand{
		RequestId: "load-" + id,
		Command:   &vttv1.ClientCommand_LoadMap{LoadMap: &vttv1.LoadMap{MapId: id}},
	})
	if err != nil {
		c.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.dm.Write(ctx, websocket.MessageText, raw); err != nil {
		c.t.Fatalf("ws write: %v", err)
	}
	res, err := readCommandResult(c.dm, 5*time.Second)
	if err != nil {
		c.t.Fatalf("read result: %v", err)
	}
	return res
}

// scene reads the next SceneCreated off the watching seat, so a test can assert
// what the wire actually carries rather than only what the DM was told.
func (c *artCampaign) scene() *vttv1.SceneCreated {
	c.t.Helper()
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, raw, err := c.view.Read(ctx)
		cancel()
		if err != nil {
			c.t.Fatalf("ws read: %v", err)
		}
		var frame vttv1.ServerFrame
		if err := protojson.Unmarshal(raw, &frame); err != nil {
			c.t.Fatal(err)
		}
		if sc := frame.GetEvent().GetSceneCreated(); sc != nil {
			return sc
		}
	}
	c.t.Fatal("no SceneCreated within 10 frames")
	return nil
}

// TestArtInstalledAfterBootIsFoundWithoutARestart is the defect sub-project 15
// shipped, inverted into a requirement (design spec §3.6, exit criterion 4):
// art appears in the campaign's art/ AFTER the server is up, and the very next
// load_map draws with it — no restart, and no answer anywhere that suggests one.
//
// IT MUST GO THROUGH composeServer. internal/gateway's own
// TestArtInstalledAfterBootDrawsWithoutARestart pins the same property against a
// constructed Server, and by construction it cannot see a boot-order defect:
// its fixture wires WithArtDir itself.
//
// THE "no art directory at boot" SUBTEST IS THE PLACEMENT PROOF. Neither
// subtest has a maps/ at boot either, so a WithArtDir call placed inside
// composeServer's os.Stat(mapsDir) guard fails both — and a call placed inside
// a guard on art/ existing fails the second one alone. That pair is the whole
// of "wired unconditionally", and it is the improvisation case: a brand-new
// campaign has neither directory.
func TestArtInstalledAfterBootIsFoundWithoutARestart(t *testing.T) {
	for _, tc := range []struct {
		name         string
		artDirAtBoot bool
	}{
		{"an empty art directory at boot", true},
		{"no art directory at boot — a brand-new campaign", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaignPath := filepath.Join(t.TempDir(), "campaign")
			if tc.artDirAtBoot {
				if err := os.MkdirAll(filepath.Join(campaignPath, "art"), 0o750); err != nil {
					t.Fatal(err)
				}
			}
			c := startArtCampaign(t, campaignPath)

			c.installArt("late-stone", `{"format_version":1,"kind":"floor","material":"stone"}`)
			c.installMap("hall", oneOverrideMap("hall", "late-stone"))

			res := c.loadMap("hall")
			if !res.GetOk() {
				t.Fatalf("load_map: %s", res.GetError())
			}
			if len(res.GetWarnings()) != 0 {
				t.Fatalf("warnings %q: the art WAS installed, before the load and after the boot",
					res.GetWarnings())
			}
			if got := c.scene().GetTiles()["0,0"].GetArt(); got != "late-stone" {
				t.Fatalf("tiles[0,0].art = %q, want late-stone on the wire — a silent "+
					"degrade would satisfy the two assertions above on a server that "+
					"resolves nothing at all", got)
			}
		})
	}
}

// TestAMapNamingArtThatIsNotInstalledLoadsAndNamesTheReference is design spec
// §4's keystone at the wire, and exit criterion 3: "A map whose art is entirely
// missing loads, plays, and renders from the built-in vocabulary, warning once
// per unresolved reference."
//
// IT CARRIES THE ASSERTION TASK 2 HAD TO RETIRE. That task's brief drove
// load_map with a map naming absent art and asserted ok=true plus warnings
// naming the reference; Task 3 had not landed, absent art still refused on that
// tree, and Task 2 correctly substituted a kind-mismatch fixture. The positive
// case was then tested nowhere, and the negative ones around it — "warnings must
// be empty" — would all pass on a server that never warns at all.
//
// ONE PIECE IS INSTALLED AND ONE IS NOT, deliberately. A fixture where
// everything is missing warns identically on a server wired to no art directory
// whatsoever, which is exactly the state this task found the tree in: the
// warning has to name the reference that DROPPED and leave the one that
// resolved out of it.
func TestAMapNamingArtThatIsNotInstalledLoadsAndNamesTheReference(t *testing.T) {
	c := startArtCampaign(t, filepath.Join(t.TempDir(), "campaign"))

	c.installArt("installed-stone", `{"format_version":1,"kind":"floor","material":"stone"}`)
	c.installMap("hall", `{"format_version":1,"id":"hall","name":"Hall",
		"grid_width":2,"grid_height":1,"tiles":{"0,0":"stone","1,0":"stone"},
		"overrides":{"0,0":"installed-stone","1,0":"absent-stone"}}`)

	res := c.loadMap("hall")
	if !res.GetOk() {
		t.Fatalf("load_map: %s — art that is not installed degrades its square, it does "+
			"not refuse the map (design spec §4)", res.GetError())
	}
	said := strings.Join(res.GetWarnings(), "\n")
	if !strings.Contains(said, "absent-stone") {
		t.Fatalf("warnings %q, want the dropped reference named: a square that silently "+
			"draws plain is a typo nobody ever finds", res.GetWarnings())
	}
	if strings.Contains(said, "installed-stone") {
		t.Fatalf("warnings %q name installed-stone, which resolved: a warning about every "+
			"reference is a warning about none", res.GetWarnings())
	}
	sc := c.scene()
	if got := sc.GetTiles()["0,0"].GetArt(); got != "installed-stone" {
		t.Errorf("tiles[0,0].art = %q, want installed-stone", got)
	}
	if got := sc.GetTiles()["1,0"]; got.GetArt() != "" || got.GetKind() != "floor" {
		t.Errorf("tiles[1,0] = %+v, want no art and the kind its map declared — the square "+
			"keeps being a floor, it just stops being a PARTICULAR floor", got)
	}
}

// TestArtOverwrittenInPlaceChangesWhatAReloadDraws is Patrik's scenario of
// 2026-09-02: "i find a better art ... i should be able to overwrite it ... And
// then when I reload the map. It will use the new art." Nothing is cached
// across a load, and this is what pins it (design spec §3.3, §3.6).
//
// IT LOADS TWO MAPS RATHER THAN THE SAME MAP TWICE, and that is a fact about
// the platform rather than a convenience: a scene id can only be created once,
// so a second load_map of "hall" is refused by campaign.AppendBatch with an
// ordinary scene collision (internal/gateway's
// TestConcurrentLookupsOfANewlyInstalledMapCompileItOnce pins the one-compile
// half of that; this cited TestTwoLoadsOfTheSameNewMapRaceCleanly, a name no
// test in the tree has ever carried — corrected 2026-09-04). Two maps naming the
// SAME art piece is the same question asked in a way the log can answer: is the
// sidecar read again, or was it remembered?
//
// THE OBSERVABLE IS THE KIND MISMATCH WARNING, not the material, and that took
// finding out: TileRef.Material comes from the map's own standard tile and never
// from the sidecar (internal/mapdef's Resolve — "the ART will never decide the
// nature of the square"), so overwriting a sidecar's material changes nothing on
// the wire. Its KIND does: it is compared against the square's, and a
// disagreement warns.
func TestArtOverwrittenInPlaceChangesWhatAReloadDraws(t *testing.T) {
	c := startArtCampaign(t, filepath.Join(t.TempDir(), "campaign"))

	// A wall drawn over a floor square: legitimate (an illusory wall), and it
	// warns.
	c.installArt("hall-stone", `{"format_version":1,"kind":"wall","material":"stone"}`)
	c.installMap("hall-a", oneOverrideMap("hall-a", "hall-stone"))
	c.installMap("hall-b", oneOverrideMap("hall-b", "hall-stone"))

	first := c.loadMap("hall-a")
	if !first.GetOk() {
		t.Fatalf("load_map hall-a: %s", first.GetError())
	}
	// "drawn for wall", not merely the art's name: with no art directory wired
	// at all this same square warns that hall-stone is NOT INSTALLED, which
	// also names it — so a name-only assertion passes on a server that resolved
	// nothing, and the overwrite below then proves nothing either.
	if said := strings.Join(first.GetWarnings(), "\n"); !strings.Contains(said, "hall-stone") ||
		!strings.Contains(said, "drawn for wall") {
		t.Fatalf("warnings %q, want the kind mismatch named before the overwrite — "+
			"without it the second load proves nothing", first.GetWarnings())
	}

	// The overwrite: same stem, same filename, better art. `cp` is the whole
	// interface (design spec §3.3).
	c.installArt("hall-stone", `{"format_version":1,"kind":"floor","material":"stone"}`)

	second := c.loadMap("hall-b")
	if !second.GetOk() {
		t.Fatalf("load_map hall-b: %s", second.GetError())
	}
	if len(second.GetWarnings()) != 0 {
		t.Fatalf("warnings %q after the overwrite: the sidecar on disk now agrees with "+
			"the square, so the old one was remembered rather than read", second.GetWarnings())
	}
}

// --- 2026-09-02-art-is-a-flat-library Task 4b -------------------------------

// TestACampaignHoldingACorruptSidecarStillBoots is the measurement this task
// was written from, inverted into a requirement. Task 4's review put one
// truncated sidecar in a campaign whose committed map named it and started the
// server: exit status 1, every other map fine, and the boot log printing "the
// server is starting anyway" one line before it did not. `composeServer` turns
// ANY map-load error into a refusal to start, and until Patrik's ruling of
// 2026-09-04 a sidecar that could not be parsed was one.
//
// IT MUST GO THROUGH composeServer, and that is the whole reason this test is
// here rather than only in internal/mapdef. A Resolve unit test sees one square
// degrade; only a boot sees the campaign come up. The failure this guards is
// not "the square is wrong", it is "nobody plays".
//
// THE ART AND THE MAP ARE BOTH IN PLACE BEFORE THE BOOT, unlike every other
// test in this file, which installs during the session: a file that arrives
// after the walk has run cannot fail the walk.
//
// A SECOND, WELL-FORMED PIECE AND A SECOND MAP sit beside the broken one so the
// test can say what the refusal actually cost. Without them a green run is
// consistent with a server that boots and serves nothing.
func TestACampaignHoldingACorruptSidecarStillBoots(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign")
	artDir := filepath.Join(campaignPath, "art")
	mapsDir := filepath.Join(campaignPath, "maps")
	for _, dir := range []string{artDir, mapsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	// A truncated copy — the shape an interrupted cp leaves, and spec §4's own
	// example of a sidecar that cannot be read.
	writeFile(t, filepath.Join(artDir, "half-copied.json"), `{"format_version":1,"kind":"flo`)
	writeFile(t, filepath.Join(artDir, "half-copied.png"), "fake-png")
	writeFile(t, filepath.Join(artDir, "good-stone.json"),
		`{"format_version":1,"kind":"floor","material":"stone"}`)
	writeFile(t, filepath.Join(artDir, "good-stone.png"), "fake-png")
	writeFile(t, filepath.Join(mapsDir, "hall.json"), `{"format_version":1,"id":"hall","name":"Hall",
		"grid_width":2,"grid_height":1,"tiles":{"0,0":"stone","1,0":"stone"},
		"overrides":{"0,0":"half-copied","1,0":"good-stone"}}`)
	writeFile(t, filepath.Join(mapsDir, "vault.json"), oneOverrideMap("vault", "good-stone"))

	// startArtCampaign fails the test if composeServer refuses, which is
	// exactly the exit status 1 this task exists to remove.
	c := startArtCampaign(t, campaignPath)

	res := c.loadMap("hall")
	if !res.GetOk() {
		t.Fatalf("load_map: %s — the corrupt piece costs its own square, not the map",
			res.GetError())
	}
	said := strings.Join(res.GetWarnings(), "\n")
	if !strings.Contains(said, "half-copied") || !strings.Contains(said, "cannot be used") {
		t.Fatalf("warnings %q, want the broken piece named and the reason given: a square "+
			"that silently draws plain is a file nobody ever goes and fixes",
			res.GetWarnings())
	}
	if strings.Contains(said, "not installed") {
		t.Fatalf("warnings %q say the piece is not installed; it is sitting in art/, and "+
			"sending the DM to look for a missing file wastes the trip", res.GetWarnings())
	}
	sc := c.scene()
	if got := sc.GetTiles()["0,0"]; got.GetArt() != "" || got.GetKind() != "floor" {
		t.Errorf("tiles[0,0] = %+v, want no art and the kind its map declared", got)
	}
	if got := sc.GetTiles()["1,0"].GetArt(); got != "good-stone" {
		t.Errorf("tiles[1,0].art = %q, want good-stone — the well-formed piece beside the "+
			"broken one must be unaffected", got)
	}
	// The other map booted too. One broken file used to cost every map in the
	// campaign, not only the one that named it.
	if vault := c.loadMap("vault"); !vault.GetOk() {
		t.Fatalf("load_map vault: %s — it names none of the broken art", vault.GetError())
	}
}

// TestACampaignWhoseArtDeclaresANewerFormatRefusesToBoot is the half of
// Patrik's 2026-09-04 ruling that goes the other way, driven at the boot where
// the difference is visible. A declared format_version this server does not
// understand is not "this file is broken", it is "this content is newer than
// this server", and one refusal naming both versions says that where a wall of
// warnings would read as "my art is broken".
//
// It calls composeServer directly rather than through startArtCampaign,
// because startArtCampaign's job is to boot successfully and this asserts the
// opposite.
func TestACampaignWhoseArtDeclaresANewerFormatRefusesToBoot(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign")
	artDir := filepath.Join(campaignPath, "art")
	mapsDir := filepath.Join(campaignPath, "maps")
	for _, dir := range []string{artDir, mapsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(artDir, "from-the-future.json"),
		`{"format_version":99,"kind":"floor","material":"stone"}`)
	writeFile(t, filepath.Join(artDir, "from-the-future.png"), "fake-png")
	writeFile(t, filepath.Join(mapsDir, "hall.json"), oneOverrideMap("hall", "from-the-future"))

	_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose art was written for a later format started a server; " +
			"the operator is then reading a plain square as broken art when the " +
			"diagnosis is that this server is too old — one square here, and one per " +
			"square of a whole v2 art set")
	}
	said := err.Error()
	if !strings.Contains(said, "99") || !strings.Contains(said, "understands 1") {
		t.Fatalf("boot refusal = %q, want both versions named — the remedy is a newer "+
			"server, and nothing else in the message says which", said)
	}
}

// TestACampaignWhoseArtHasATypodVersionStillBoots is the boundary between the
// two tests above, driven where it costs something. `"format_version": 0` is
// the shape a hand-written sidecar takes when somebody types the field and
// leaves the number at zero, or copies a line and truncates it — and until this
// task it took the SAME path as 99: mapdef.Resolve refused the map, and
// composeServer refused the boot, for a file no server has ever written and no
// newer server would fix.
//
// It is the same absent-versus-zero defect fixed twice already on this branch
// (mapJSON.Pack, and campaigncfg's two fields), and it stayed harmless only
// while no real sidecar existed. Task 8 of the art-is-a-flat-library plan is
// what makes campaigns/example/art/ real, so it closes this first.
//
// THE ASSERTION IS THE BOOT, not the square, for the reason
// TestACampaignHoldingACorruptSidecarStillBoots gives: a Resolve unit test sees
// one square degrade; only a boot sees whether anybody gets to play. A second,
// well-formed piece sits beside the typo so a green run cannot be a server that
// came up serving nothing.
func TestACampaignWhoseArtHasATypodVersionStillBoots(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign")
	artDir := filepath.Join(campaignPath, "art")
	mapsDir := filepath.Join(campaignPath, "maps")
	for _, dir := range []string{artDir, mapsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(artDir, "typod-version.json"),
		`{"format_version":0,"kind":"floor","material":"stone"}`)
	writeFile(t, filepath.Join(artDir, "typod-version.png"), "fake-png")
	writeFile(t, filepath.Join(artDir, "good-stone.json"),
		`{"format_version":1,"kind":"floor","material":"stone"}`)
	writeFile(t, filepath.Join(artDir, "good-stone.png"), "fake-png")
	writeFile(t, filepath.Join(mapsDir, "hall.json"), `{"format_version":1,"id":"hall","name":"Hall",
		"grid_width":2,"grid_height":1,"tiles":{"0,0":"stone","1,0":"stone"},
		"overrides":{"0,0":"typod-version","1,0":"good-stone"}}`)

	c := startArtCampaign(t, campaignPath)

	res := c.loadMap("hall")
	if !res.GetOk() {
		t.Fatalf("load_map: %s — a typo'd version costs its own square, not the map",
			res.GetError())
	}
	said := strings.Join(res.GetWarnings(), "\n")
	if !strings.Contains(said, "typod-version") || !strings.Contains(said, "cannot be used") {
		t.Fatalf("warnings %q, want the piece named and the reason given: a square that "+
			"silently draws plain is a file nobody ever goes and fixes", res.GetWarnings())
	}
	sc := c.scene()
	if got := sc.GetTiles()["0,0"]; got.GetArt() != "" || got.GetKind() != "floor" {
		t.Errorf("tiles[0,0] = %+v, want no art and the kind its map declared", got)
	}
	if got := sc.GetTiles()["1,0"].GetArt(); got != "good-stone" {
		t.Errorf("tiles[1,0].art = %q, want good-stone — the well-formed piece beside the "+
			"typo must be unaffected", got)
	}
}

// --- 2026-09-02-art-is-a-flat-library Task 6 --------------------------------

// TestCampaignsArtServesEndToEnd is the round trip TestCampaignsMapsAndPacksServeEndToEnd's
// pack half used to make and nothing has made since Task 7 deleted it: a
// campaign's OWN INSTALLED BYTES coming back over a real listener, through a
// real composeServer, from a real directory on disk.
//
// IT IS NOT REDUNDANT WITH internal/gateway's route tests. Those build a
// gateway.Server by hand and hand it an art directory, so they prove what the
// HANDLER does; this proves that composeServer wires campaignPath/art to it at
// all. That distinction is exactly the one design spec §1's defect lived in —
// the pack load was gated on maps/ existing, and no gateway fixture could see
// it, because a constructed Server has whatever the test handed it and no boot
// order at all.
//
// NO maps/ HERE, deliberately, for the same reason: a campaign with art and no
// map yet is §1's own case, and art must be reachable in it.
func TestCampaignsArtServesEndToEnd(t *testing.T) {
	campaignPath := t.TempDir()
	artDir := filepath.Join(campaignPath, "art")
	if err := os.MkdirAll(artDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(artDir, "masonry-1.png"), "the campaign's own installed bytes")
	writeFile(t, filepath.Join(artDir, "masonry-1.json"),
		`{"format_version":1,"kind":"wall","material":"stone"}`)

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer with art installed and no maps/: %v", err)
	}
	t.Cleanup(func() {
		if err := closeFn(); err != nil {
			t.Errorf("closeFn: %v", err)
		}
	})
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	base := "http://" + ln.Addr().String()
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	if err := waitForHealthz(base, 3*time.Second); err != nil {
		t.Fatalf("healthz never became ready: %v", err)
	}

	ids, err := identity.Open(campaign.LogPath(campaignPath))
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, body
	}

	code, body := get("/api/art/masonry-1.png")
	if code != http.StatusOK {
		t.Fatalf("/api/art/masonry-1.png = %d (%s), want 200 — composeServer must wire "+
			"campaignPath/art to the route, and it must not be gated on maps/ existing", code, body)
	}
	if string(body) != "the campaign's own installed bytes" {
		t.Fatalf("body = %q, want the installed file's own bytes", body)
	}

	// Installed DURING the session, with nothing restarted and nothing told to
	// reload (design spec §3.6/§10 criterion 4).
	writeFile(t, filepath.Join(artDir, "earth-1.png"), "installed while the server was running")
	if code, body := get("/api/art/earth-1.png"); code != http.StatusOK ||
		string(body) != "installed while the server was running" {
		t.Fatalf("mid-session install: %d %q — art installed while the server runs must be served "+
			"with no restart", code, body)
	}
}

// TestTheCampaignsDeclaredCellPxReachesTheClient is campaign.json's own end-to-
// end proof: the file is optional (design spec §6), so a campaign that DOES
// write one has to see its number arrive, or nothing distinguishes the file
// from a file nobody reads.
//
// Both halves in one test on purpose — a default-only assertion passes against
// a server that hard-codes 64 and never opens campaign.json at all.
func TestTheCampaignsDeclaredCellPxReachesTheClient(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		want float64
	}{
		{"no campaign.json at all", "", 64},
		{"a campaign that declares its own grid", `{"format_version":1,"cell_px":32}`, 32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaignPath := t.TempDir()
			if tc.file != "" {
				writeFile(t, filepath.Join(campaignPath, "campaign.json"), tc.file)
			}
			srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
			if err != nil {
				t.Fatalf("composeServer: %v", err)
			}
			t.Cleanup(func() {
				if err := closeFn(); err != nil {
					t.Errorf("closeFn: %v", err)
				}
			})
			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				t.Fatal(err)
			}
			base := "http://" + ln.Addr().String()
			go func() { _ = srv.Serve(ln) }()
			t.Cleanup(func() { _ = srv.Close() })
			if err := waitForHealthz(base, 3*time.Second); err != nil {
				t.Fatalf("healthz never became ready: %v", err)
			}
			ids, err := identity.Open(campaign.LogPath(campaignPath))
			if err != nil {
				t.Fatal(err)
			}
			defer ids.Close()
			tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
			if err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequest(http.MethodGet, base+"/api/maps", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode: %v (body %s)", err, body)
			}
			if got["cellPx"] != tc.want {
				t.Fatalf("cellPx = %v, want %v (body %s)", got["cellPx"], tc.want, body)
			}
		})
	}
}

// TestAMapsOwnCellPxSurvivesTheWholeBootPath is the per-map half of the ruling,
// through the REAL composeServer: a map file that declares cell_px is loaded by
// mapdef's boot walk, carried on the *Map the gateway holds, and reported on its
// own /api/maps entry — while a sibling that declares nothing reports the
// campaign's. Neither the mapdef unit test nor the gateway fixture can see that
// chain: one stops at the *Map and the other starts from one handed in.
func TestAMapsOwnCellPxSurvivesTheWholeBootPath(t *testing.T) {
	campaignPath := t.TempDir()
	writeFile(t, filepath.Join(campaignPath, "campaign.json"), `{"format_version":1,"cell_px":48}`)
	mapsDir := filepath.Join(campaignPath, "maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(mapsDir, "attic.json"), `{"format_version":1,"id":"attic",
		"name":"The Attic","grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"}}`)
	writeFile(t, filepath.Join(mapsDir, "cellar.json"), `{"format_version":1,"id":"cellar",
		"name":"The Cellar","grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},"cell_px":128}`)

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	t.Cleanup(func() {
		if err := closeFn(); err != nil {
			t.Errorf("closeFn: %v", err)
		}
	})
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	if err := waitForHealthz(base, 3*time.Second); err != nil {
		t.Fatalf("healthz never became ready: %v", err)
	}
	ids, err := identity.Open(campaign.LogPath(campaignPath))
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodGet, base+"/api/maps", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		CellPx float64          `json:"cellPx"`
		Maps   []map[string]any `json:"maps"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v (body %s)", err, body)
	}
	if got.CellPx != 48 {
		t.Errorf("top-level cellPx = %v, want the campaign's declared 48", got.CellPx)
	}
	if len(got.Maps) != 2 {
		t.Fatalf("got %d maps, want 2 (body %s)", len(got.Maps), body)
	}
	// Sorted by id: attic, then cellar.
	if got.Maps[0]["cellPx"] != float64(48) {
		t.Errorf("attic cellPx = %v, want the campaign's 48 by inheritance", got.Maps[0]["cellPx"])
	}
	if got.Maps[1]["cellPx"] != float64(128) {
		t.Errorf("cellar cellPx = %v, want its own 128", got.Maps[1]["cellPx"])
	}
}

// TestAMapDeclaringAnImpossibleCellPxStopsTheBoot keeps the map-format refusal
// reachable where an operator reads it. A map file is loaded at boot and fails
// loud there, exactly as every other malformed map field does — this is a MAP
// error, not an art one, so §4's degrade ruling does not reach it.
func TestAMapDeclaringAnImpossibleCellPxStopsTheBoot(t *testing.T) {
	campaignPath := t.TempDir()
	mapsDir := filepath.Join(campaignPath, "maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(mapsDir, "cellar.json"), `{"format_version":1,"id":"cellar",
		"name":"The Cellar","grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"},"cell_px":100000}`)

	_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a map declaring cell_px 100000 started a server")
	}
	if !strings.Contains(err.Error(), "cell_px") {
		t.Fatalf("boot refusal = %q, want it to name the field", err)
	}
}

// TestABrokenCampaignJSONIsRefusedBeforeAnythingIsOpened pins the ORDER of
// composeServer's first two acts, which is a claim its own comment made and its
// code did not (review finding F3, 2026-09-05): the call sat after
// campaign.Open and identity.Open, under a sentence saying it was read "before
// any handle-closing work below has to be undone, and before anything else
// touches the campaign".
//
// THE ORDER IS WORTH PINNING RATHER THAN JUST THE REFUSAL. A config refusal
// needs nothing open, so reading it first means a campaign.json with a typo in
// it costs the operator nothing — no SQLite file created, no identity schema
// migrated, nothing to undo. Reading it third means every boot against a broken
// settings file opens two handles and closes them again, and the "best-effort"
// close of a handle nobody wanted is a failure mode invented by ordering alone.
//
// log.db IS THE OBSERVABLE, and it is the campaign's own (campaign.LogPath):
// campaign.Open creates the directory and the store, so its absence after a
// refusal is the proof that the refusal came first. A test asserting only the
// error passes with the call anywhere at all.
func TestABrokenCampaignJSONIsRefusedBeforeAnythingIsOpened(t *testing.T) {
	campaignPath := t.TempDir()
	writeFile(t, filepath.Join(campaignPath, "campaign.json"), `{"format_version":1,"cell_px":`)

	_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose campaign.json cannot be read started a server")
	}
	if _, statErr := os.Stat(campaign.LogPath(campaignPath)); statErr == nil {
		t.Fatalf("%s exists after the refusal — the campaign log was opened for a boot that was "+
			"never going to happen, which is the ordering composeServer's own comment claims it "+
			"has", campaign.LogPath(campaignPath))
	}
}

// TestABrokenCampaignJSONStopsTheBoot pins campaigncfg's strict-at-boot posture
// where it actually binds. The file has exactly one server-side reader and it
// is this one, with an operator at a terminal — the same posture a broken MAP
// already gets here, and deliberately NOT art's degrade ruling, which exists
// because a DM in a browser cannot act on a filesystem path (spec §4).
//
// Booting on the default instead would leave a campaign that DECLARED 32 being
// served 64 with nothing said, which is the silence §4's warning channel exists
// to prevent, arrived at from the other direction.
func TestABrokenCampaignJSONStopsTheBoot(t *testing.T) {
	campaignPath := t.TempDir()
	writeFile(t, filepath.Join(campaignPath, "campaign.json"), `{"format_version":1,"cell_px":`)

	_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err == nil {
		if closeErr := closeFn(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal("a campaign whose campaign.json cannot be read started a server; the operator " +
			"is at a terminal and can fix the file, and a silent fallback to the default would " +
			"serve a grid the campaign never declared")
	}
	if !strings.Contains(err.Error(), "campaign.json") {
		t.Fatalf("boot refusal = %q, want it to name campaign.json", err)
	}
}
