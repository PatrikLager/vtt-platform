package gateway_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

func newStaticFixture(t *testing.T, withStatic bool) *httptest.Server {
	t.Helper()
	path := t.TempDir() + "/campaign.db"

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

	srv := gateway.New(c, ids)
	if withStatic {
		srv = srv.WithStatic(fstest.MapFS{
			"index.html":      {Data: []byte("<!doctype html><title>VTT</title>")},
			"assets/index.js": {Data: []byte("console.log('vtt')")},
		})
	}
	s := httptest.NewServer(srv.Handler())
	t.Cleanup(s.Close)
	return s
}

func get(t *testing.T, base, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body)
}

func TestStaticServesTheClientAtRoot(t *testing.T) {
	s := newStaticFixture(t, true)

	code, body := get(t, s.URL, "/")
	if code != http.StatusOK {
		t.Fatalf("GET /: status = %d, want 200", code)
	}
	if body == "" {
		t.Error("GET / returned an empty body")
	}

	code, body = get(t, s.URL, "/assets/index.js")
	if code != http.StatusOK {
		t.Fatalf("GET /assets/index.js: status = %d, want 200", code)
	}
	if body != "console.log('vtt')" {
		t.Errorf("asset body = %q", body)
	}
}

// TestStaticIsUnauthenticated pins a deliberate decision: the client BUNDLE is
// public. It has to be — the browser must load the app before it has anywhere
// to type a token. Every route the bundle then calls is authenticated, so
// what leaks here is the program, not the campaign.
func TestStaticIsUnauthenticated(t *testing.T) {
	s := newStaticFixture(t, true)
	if code, _ := get(t, s.URL, "/"); code != http.StatusOK {
		t.Fatalf("GET / without a token: status = %d, want 200", code)
	}
}

// TestStaticDoesNotShadowTheAPI is the regression this file exists for. A
// catch-all static handler registered at "/" competes with /api and /ws, and
// getting the precedence wrong serves index.html to the client's own fetch
// calls — which then fail to parse as JSON with a message that says nothing
// about routing.
func TestStaticDoesNotShadowTheAPI(t *testing.T) {
	s := newStaticFixture(t, true)

	// Unauthenticated, so 401 — the point is that it is the API answering,
	// not the static handler returning index.html with a 200.
	if code, body := get(t, s.URL, "/api/ruleset"); code != http.StatusUnauthorized {
		t.Errorf("GET /api/ruleset: status = %d, want 401 (body %q)", code, body)
	}
	if code, _ := get(t, s.URL, "/healthz"); code != http.StatusOK {
		t.Errorf("GET /healthz: status = %d, want 200", code)
	}
}

// TestStaticAbsentIsNotAServerError covers `vtt serve` built without the
// client bundle: the API must keep working and the browser must get an honest
// 404 rather than a panic on a nil FS.
func TestStaticAbsentIsNotAServerError(t *testing.T) {
	s := newStaticFixture(t, false)

	if code, _ := get(t, s.URL, "/"); code != http.StatusNotFound {
		t.Errorf("GET / with no bundle: status = %d, want 404", code)
	}
	if code, _ := get(t, s.URL, "/healthz"); code != http.StatusOK {
		t.Errorf("GET /healthz: status = %d, want 200", code)
	}
}

// TestStaticRefusesPathTraversal pins that the embedded FS cannot be walked
// out of. http.FS rejects this, but the assertion belongs here rather than in
// the stdlib's tests, because it is our handler's registration that decides
// whether that protection is even in the path.
func TestStaticRefusesPathTraversal(t *testing.T) {
	s := newStaticFixture(t, true)
	for _, p := range []string{"/../go.mod", "/assets/../../go.mod", "/%2e%2e/go.mod"} {
		code, body := get(t, s.URL, p)
		if code == http.StatusOK && len(body) > 0 && body[0] == 'm' {
			t.Errorf("%s served something outside the bundle: %.40q", p, body)
		}
	}
}
