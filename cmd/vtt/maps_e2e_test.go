package main

// maps_e2e_test.go proves composeServer's real map-serving lifecycle end to
// end (maps-as-geometry Task 7; the --maps-dir flag itself is gone as of
// 2026-09-01-create-scene-leaves Task 5 — "the kernel serves maps, it does
// not make them" — maps now come from the campaign directory's own maps/
// and packs/, so these tests write straight into campaignPath instead of a
// separate operator-pointed directory) — the same concern serve_e2e_test.go's
// TestComposeServerFailsLoudlyOnAnUnreadableAdventureGuide covers for
// --adventures-dir, and the reason a wiring change like Task 5's
// (docs/superpowers/plans/2026-09-01-create-scene-leaves.md's own Task 5
// section) needs an end-to-end test of its own rather than trusting
// loadMapsDir's unit coverage alone: a change to composeServer's own call
// site is exactly the kind of coupling a package's own unit tests, run in
// isolation, cannot see. maps_test.go proves loadMapsDir's OWN logic in
// isolation; this file proves the WIRING — a real composeServer, a real
// gateway.Handler(), a real listener — actually connects a campaign's
// maps/ and packs/ to GET /api/maps and GET /api/packs/{pack}/{file}.

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// TestCampaignsMapsAndPacksServeEndToEnd boots composeServer against a
// campaign directory holding one real map and one real pack (Task 3 layout:
// maps are flat files under campaignPath/maps/, named by their own id;
// packs are directories under the sibling campaignPath/packs/, keyed by
// pack.json's own declared id) and drives GET /api/maps and GET
// /api/packs/{pack}/{file} over an actual HTTP listener — not the
// gateway-package fixture, which never goes through composeServer/the real
// wiring at all.
func TestCampaignsMapsAndPacksServeEndToEnd(t *testing.T) {
	campaignPath := t.TempDir()

	packDir := filepath.Join(campaignPath, "packs", "mossy-keep")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "pack.json"), []byte(`{
		"format_version": 1,
		"id": "mossy-keep", "name": "Mossy Keep", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3","file":"planks_03.png","kind":"floor","material":"wood"}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "planks_03.png"), []byte("stand-in image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	mapsSubDir := filepath.Join(campaignPath, "maps")
	if err := os.MkdirAll(mapsSubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mapsSubDir, "shrine.json"), []byte(`{
		"format_version": 1,
		"id": "shrine", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1, "pack": "mossy-keep",
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
	var got struct {
		Maps []struct {
			ID   string `json:"id"`
			Pack *struct {
				ID string `json:"id"`
			} `json:"pack"`
		} `json:"maps"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode /api/maps: %v (body %s)", err, body)
	}
	if len(got.Maps) != 1 || got.Maps[0].ID != "shrine" || got.Maps[0].Pack == nil || got.Maps[0].Pack.ID != "mossy-keep" {
		t.Fatalf("/api/maps = %+v, want one shrine map with pack mossy-keep", got.Maps)
	}

	code, body = get("/api/packs/mossy-keep/planks_03.png")
	if code != http.StatusOK || string(body) != "stand-in image bytes" {
		t.Fatalf("/api/packs/mossy-keep/planks_03.png: status = %d, body = %q", code, body)
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
