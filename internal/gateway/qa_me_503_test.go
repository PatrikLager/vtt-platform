package gateway_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

type qa503Rig struct {
	srv   *httptest.Server
	db    *sql.DB
	token string
	other string
	gone  string
}

func qa503Start(t *testing.T) *qa503Rig {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	live, _, err := ids.CreateInvite("Live", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := ids.CreateInvite("Other", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}
	gone, goneID, err := ids.CreateInvite("Gone", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if err := ids.Revoke(goneID); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gateway.New(c, ids).Handler())
	t.Cleanup(srv.Close)
	db, err := sql.Open("sqlite", campaign.LogPath(dir)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &qa503Rig{srv: srv, db: db, token: live, other: other, gone: gone}
}

func (r *qa503Rig) storeDown(t *testing.T) {
	t.Helper()
	if _, err := r.db.Exec(`ALTER TABLE participants RENAME TO participants_elsewhere`); err != nil {
		t.Fatal(err)
	}
}

func (r *qa503Rig) storeUp(t *testing.T) {
	t.Helper()
	if _, err := r.db.Exec(`ALTER TABLE participants_elsewhere RENAME TO participants`); err != nil {
		t.Fatal(err)
	}
}

func qa503Get(t *testing.T, base, path, token string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

var qa503Routes = []string{
	"/api/me", "/api/ruleset", "/api/ruleset/guide", "/api/adventures",
	"/api/adventures/x/guide", "/api/maps", "/api/art/x.png",
	"/api/join-link", "/api/participants",
}

func TestQAIdentityStoreDownIsNot401(t *testing.T) {
	// VTT-578
	r := qa503Start(t)
	r.storeDown(t)
	for _, p := range qa503Routes {
		for name, tok := range map[string]string{"live": r.token, "dm": r.other, "unknown": "nope", "revoked": r.gone} {
			if got := qa503Get(t, r.srv.URL, p, tok); got != http.StatusServiceUnavailable {
				t.Errorf("%s with %s token while store down: got %d, want 503", p, name, got)
			}
		}
	}
}

func TestQAIdentityStoreUpStill401AndLive200(t *testing.T) {
	// VTT-581
	r := qa503Start(t)
	for _, p := range []string{"/api/ruleset", "/api/me", "/api/art/x.png"} {
		for name, tok := range map[string]string{"none": "", "unknown": "nope", "revoked": r.gone} {
			if got := qa503Get(t, r.srv.URL, p, tok); got != http.StatusUnauthorized {
				t.Errorf("%s with %s token: got %d, want 401", p, name, got)
			}
		}
	}
	if got := qa503Get(t, r.srv.URL, "/api/me", r.token); got != 200 {
		t.Errorf("/api/me live: got %d", got)
	}
}

func TestQAIdentityStoreRecoversTo200And401(t *testing.T) {
	// VTT-578 VTT-581
	r := qa503Start(t)
	r.storeDown(t)
	if got := qa503Get(t, r.srv.URL, "/api/me", r.token); got != 503 {
		t.Fatalf("down: got %d", got)
	}
	r.storeUp(t)
	if got := qa503Get(t, r.srv.URL, "/api/me", r.token); got != 200 {
		t.Errorf("recovered live: got %d, want 200", got)
	}
	if got := qa503Get(t, r.srv.URL, "/api/me", r.gone); got != 401 {
		t.Errorf("recovered revoked: got %d, want 401", got)
	}
}
