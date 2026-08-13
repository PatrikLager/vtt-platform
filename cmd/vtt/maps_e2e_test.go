package main

// maps_e2e_test.go proves `vtt serve --maps-dir` end to end through
// composeServer's real lifecycle (maps-as-geometry Task 7) — the same
// concern serve_e2e_test.go's TestComposeServerFailsLoudlyOnAnUnreadable-
// AdventureGuide covers for --adventures-dir, and the reason this repo's
// own method change (progress.md, Task 5) requires it: "every task so far
// was green on its own packages and then failed the [real] gate, because
// the couplings that bite live outside the layer a task is named after".
// maps_test.go proves loadMapsDir's OWN logic in isolation; this file
// proves the WIRING — the flag, composeServer, gateway.Handler() — actually
// connects them over a real listener.

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

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// TestServeMapsDirEndToEnd boots composeServer with a real --maps-dir (one
// map, one pack, one real file) and drives GET /api/maps and GET
// /api/packs/{pack}/{file} over an actual HTTP listener — not the
// gateway-package fixture, which never goes through composeServer/the CLI
// flag at all.
func TestServeMapsDirEndToEnd(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	mapsDir := t.TempDir()
	sub := filepath.Join(mapsDir, "shrine")
	if err := os.MkdirAll(filepath.Join(sub, "tiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "tiles", "pack.json"), []byte(`{
		"id": "mossy-keep", "name": "Mossy Keep", "cell_px": 64,
		"tiles": [{"name":"wood-planks-split-3","file":"planks_03.png","kind":"floor","material":"wood"}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "tiles", "planks_03.png"), []byte("stand-in image bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "map.json"), []byte(`{
		"id": "shrine", "name": "Obsidian Shrine",
		"grid_width": 1, "grid_height": 1, "pack": "mossy-keep",
		"tiles": {"0,0":"wood"},
		"overrides": {"0,0":"wood-planks-split-3"}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "", mapsDir)
	if err != nil {
		t.Fatalf("composeServer with --maps-dir: %v", err)
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

	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM, nil)
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
// under --maps-dir must stop composeServer from returning a server at all.
func TestComposeServerFailsLoudlyOnABrokenMap(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	srv, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "", "testdata/maps-with-one-broken")
	if err == nil {
		if closeFn != nil {
			_ = closeFn()
		}
		_ = srv
		t.Fatal("composeServer succeeded with a broken map in --maps-dir; " +
			"the table would find out instead of us")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should name the offending map, got: %v", err)
	}
}
