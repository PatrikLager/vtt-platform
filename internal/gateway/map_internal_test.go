package gateway

// map_internal_test.go pins the one property of on-demand map loading that
// a client cannot see: a map installed while the server is running is
// compiled into the map set ONCE, however many requests ask for it at the
// same moment (2026-09-01-create-scene-leaves design spec §5,
// "Concurrency"). From outside, two racing load_maps are indistinguishable
// whether the set gained one entry or was overwritten twice — the losing
// command is refused by the log either way (map_test.go's
// TestTwoRacingLoadsOfANewlyInstalledMapProduceOneScene is that boundary
// half). What is only visible from in here is WHICH *mapdef.Map every
// caller ends up holding, and how many the set holds afterwards.
//
// The listing goroutines are not decoration: this is also the only test
// that runs a reader of the map set concurrently with a writer of it, and
// the map set had no writer at all before 2026-09-01-create-scene-leaves
// Task 6 — handleMaps (metadata.go) read a field that was set once at boot
// and never touched again. Under -race an unguarded read there is a
// failure here.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	"github.com/PatrikLager/vtt-platform/internal/mapdef"
)

func TestConcurrentLookupsOfANewlyInstalledMapCompileItOnce(t *testing.T) {
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
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}

	mapsDir := filepath.Join(path, "maps")
	if err := os.MkdirAll(mapsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	// "stone" is a real name in mapdef's standard vocabulary (standard.go)
	// and format_version 1 is what this server understands: a fixture
	// broken in either way would fail every lookup below for a reason that
	// has nothing to do with concurrency.
	if err := os.WriteFile(filepath.Join(mapsDir, "level-2.json"), []byte(
		`{"format_version":1,"id":"level-2","name":"Level Two",
		  "grid_width":1,"grid_height":1,"tiles":{"0,0":"stone"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	s := New(c, ids).WithMapsDir(mapsDir)

	const loaders, listers = 6, 4
	got := make([]*mapdef.Map, loaders)
	errs := make([]error, loaders)
	codes := make([]int, listers)

	// One barrier for every goroutine, so the lookups genuinely overlap
	// rather than running one after another because the earlier ones were
	// still being spawned.
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range loaders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got[i], errs[i] = s.mapByID("level-2")
		}()
	}
	for i := range listers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/maps", nil)
			r.Header.Set("Authorization", "Bearer "+tok)
			s.handleMaps(w, r)
			codes[i] = w.Code
		}()
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("lookup %d: %v", i, err)
		}
		if got[i] == nil {
			t.Fatalf("lookup %d returned no map and no error", i)
		}
	}
	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("listing %d: status = %d, want 200", i, code)
		}
	}
	// EVERY caller must hold the same map value. Different pointers would
	// mean each racing lookup compiled and installed its own copy, and
	// whichever one the set kept would be arbitrary.
	for i := 1; i < loaders; i++ {
		if got[i] != got[0] {
			t.Fatalf("lookup %d returned a different *mapdef.Map than lookup 0; "+
				"racing lookups of one id must all end up on one map", i)
		}
	}
	// wg.Wait above orders every goroutine's writes before this read, so
	// the set is read directly rather than through the lock.
	if len(s.maps) != 1 || s.maps["level-2"] != got[0] {
		t.Fatalf("map set = %v, want exactly the one map every lookup returned", s.maps)
	}
}
