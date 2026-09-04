package main

// serve_test.go proves composeServer's Task 5 contract (2026-09-01-create-
// scene-leaves plan — "the kernel serves maps, it does not make them"):
// maps come from the campaign directory itself, not a separate --maps-dir
// operator flag. Two tests, each pinning one half of the same guard:
//
//   - TestTheServerServesTheCampaignsOwnMaps: a campaign whose maps/ holds
//     a map is served — task-5-brief.md's own Step 1 test, filled in with
//     the auth GET /api/maps actually requires (s.authed,
//     internal/gateway/metadata.go) — the brief's sketch omits it.
//   - TestTheServerBootsWithNoMapsInstalledYet: a campaign with no maps/ AT
//     ALL still boots. Unlike --maps-dir, which was an operator's explicit
//     declaration (an EMPTY declared directory was refused — maps.go's own
//     "zero maps loaded from an EXISTING dir is a boot error"), an ABSENT
//     maps/ under the campaign declares nothing: design spec
//     2026-09-01-create-scene-leaves-design.md §4 ("Install, then load")
//     starts a brand-new campaign with nothing installed. Treating that as
//     a boot failure would stop `vtt client run`'s self-contained
//     throwaway campaign (harness_boot.go, composeServer's own caller) from
//     ever starting.
//
// maps_e2e_test.go covers the rest of the same wiring (pack file serving,
// fail-loud-on-a-broken-map) through a real network listener; these two
// stay narrow and use srv.Handler.ServeHTTP directly (composeServer never
// starts the listener itself — see its own doc comment), since nothing
// here needs a live socket.

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// getJSON issues an authenticated GET straight against srv's own Handler.
// tok must be a live participant token — GET /api/maps requires one
// (s.authed) even though every role may read it.
func getJSON(t *testing.T, srv *http.Server, tok, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, body %s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// mintToken opens dir's identity handle — a SEPARATE handle from the one
// composeServer already holds open, the same pattern
// TestCampaignsMapsServeEndToEnd (maps_e2e_test.go) uses — and
// returns a fresh DM token good against the running server.
func mintToken(t *testing.T, dir string) string {
	t.Helper()
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestTheServerServesTheCampaignsOwnMaps(t *testing.T) {
	dir := t.TempDir()
	writeMap(t, filepath.Join(dir, "maps", "cellar.json"), "cellar")

	srv, closeFn, err := composeServer(dir, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer: %v", err)
	}
	defer func() { _ = closeFn() }()

	tok := mintToken(t, dir)
	// GET /api/maps answers from the campaign's own maps/, not an operator
	// flag. Asserting the JSON key/value pair, not the bare word "cellar":
	// writeMap's fixture also names the map "Test Map" in its "name" field,
	// so a bare substring check would pass even if the id were wired
	// wrong and only the display name happened to leak through.
	body := getJSON(t, srv, tok, "/api/maps")
	if !strings.Contains(body, `"id":"cellar"`) {
		t.Fatalf(`GET /api/maps = %s, want "id":"cellar" — the campaign's own map`, body)
	}
}

func TestTheServerBootsWithNoMapsInstalledYet(t *testing.T) {
	dir := t.TempDir() // deliberately: no maps/ subdirectory created at all.

	srv, closeFn, err := composeServer(dir, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("composeServer with no maps/ installed: %v", err)
	}
	defer func() { _ = closeFn() }()

	tok := mintToken(t, dir)
	body := getJSON(t, srv, tok, "/api/maps")
	if !strings.Contains(body, `"maps":[]`) {
		t.Fatalf("GET /api/maps = %s, want an empty list — nothing installed is not a boot failure", body)
	}
}
