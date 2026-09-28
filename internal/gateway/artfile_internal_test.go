package gateway

// Every test here calls handleArtFile directly, with a clean r.URL.Path and the
// payload only in PathValue: ServeMux cleans ".." before matching and
// http.ServeFileFS guards a dirty path itself, and both would mask a broken
// handler. Every test asserts its control first, since a handler that refuses
// everything is green on every refusal (SPEC-012).

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// artHandlerFixture is a Server with a real art directory and a DM token, built
// for calling handleArtFile directly.
type artHandlerFixture struct {
	s      *Server
	token  string
	artDir string
}

const legitArt = "planks-03.png"
const legitBytes = "the picture that proves this handler serves anything at all"

func newArtHandlerFixture(t *testing.T) *artHandlerFixture {
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
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}

	artDir := filepath.Join(t.TempDir(), "art")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artDir, legitArt), []byte(legitBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	return &artHandlerFixture{s: New(c, ids).WithArtDir(artDir), token: tok, artDir: artDir}
}

// call drives handleArtFile with r.URL.Path CLEAN and the given name in
// PathValue only — see this file's own doc comment for why that separation is
// the whole point.
func (f *artHandlerFixture) call(name string) (int, string) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/art/"+legitArt, nil)
	r.Header.Set("Authorization", "Bearer "+f.token)
	r.SetPathValue("file", name)
	f.s.handleArtFile(w, r)
	resp := w.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

// control asserts the handler serves the legitimate file through this exact
// path, so that every refusal below is a refusal of the PAYLOAD rather than of
// everything.
func (f *artHandlerFixture) control(t *testing.T) {
	t.Helper()
	code, body := f.call(legitArt)
	if code != http.StatusOK || body != legitBytes {
		t.Fatalf("control: handleArtFile(%q) = %d %q, want 200 and the file's own bytes — "+
			"without this, the refusal below proves nothing", legitArt, code, body)
	}
}

// TestHandleArtFileRefusesTraversalEvenWithAPathValueSetDirectly is
// TestHandlePackFileRefusesTraversalEvenWithAPathValueSetDirectly's successor.
// The secret is a real file this test controls, outside the art root, standing
// in for /etc/passwd (which may not exist or be readable in every sandbox).
// VTT-125
func TestHandleArtFileRefusesTraversalEvenWithAPathValueSetDirectly(t *testing.T) {
	f := newArtHandlerFixture(t)
	f.control(t)

	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.png")
	if err := os.WriteFile(secretPath, []byte("THE SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(f.artDir, secretPath)
	if err != nil {
		t.Fatal(err)
	}
	// Verified rather than assumed: two independent t.TempDir() results could
	// in principle share a parent in a way that collapses to no ".." at all,
	// and then this test would be asserting nothing.
	if !strings.Contains(rel, "..") {
		t.Fatalf("test setup bug: rel = %q does not traverse", rel)
	}

	code, body := f.call(rel)
	if code == http.StatusOK {
		t.Fatalf("traversal escaped the art directory: status 200, body %q", body)
	}
	if strings.Contains(body, "THE SECRET") {
		t.Fatalf("response leaked the secret file's content: %q", body)
	}
}

// TestHandleArtFileRefusesSymlinkEscape is TestHandlePackFileRefusesSymlinkEscape's
// successor, and it is the one internal/artlib pins at the LOOKUP layer
// (TestLookupWillNotFollowASymlinkOutOfTheArtDirectory) and nothing pinned at a
// ROUTE between Task 7 and this task.
//
// evil.png IS A PERFECTLY ORDINARY ART FILENAME — that is the point. It passes
// artlib.IsArtFileName, so the name check says nothing here and the refusal has
// to come from the filesystem layer. No ".." appears anywhere in the request.
// VTT-125
func TestHandleArtFileRefusesSymlinkEscape(t *testing.T) {
	f := newArtHandlerFixture(t)
	f.control(t)

	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("THE SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secretPath, filepath.Join(f.artDir, "evil.png")); err != nil {
		t.Fatal(err)
	}

	code, body := f.call("evil.png")
	if code == http.StatusOK {
		t.Fatalf("a symlink escaped the art directory: status 200, body %q", body)
	}
	if strings.Contains(body, "THE SECRET") {
		t.Fatalf("response leaked the secret file's content via a symlink: %q", body)
	}
}

// TestHandleArtFileRefusesANestedNameThatRoutingWouldNeverProduce is the guard
// with no pack precedent, proved where ServeMux cannot help.
//
// ROUTING IS OUT OF THE PICTURE HERE, deliberately: PathValue is set by hand to
// a name no ServeMux pattern could hand over, so what refuses is the handler's
// own artlib.IsArtFileName check and nothing else. The wire half
// (TestArtInsideASubdirectoryIsNotReachable) drives the same rule through the
// real route, including the %2F spelling that the single-segment wildcard does
// NOT stop — see this file's own doc comment.
//
// os.OpenRoot WOULD NOT SAVE THIS. art/pack-ish/x.png is legitimately inside
// the root: a root confines, it does not flatten.
// VTT-123
func TestHandleArtFileRefusesANestedNameThatRoutingWouldNeverProduce(t *testing.T) {
	f := newArtHandlerFixture(t)
	f.control(t)

	nested := filepath.Join(f.artDir, "pack-ish")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "x.png"), []byte("NESTED ART"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, body := f.call("pack-ish/x.png")
	if code == http.StatusOK {
		t.Fatalf("a file inside a subdirectory of art/ was served: status 200, body %q — "+
			"design spec §3.3's whole no-subfolders rule rests on nothing in there being reachable",
			body)
	}
	if strings.Contains(body, "NESTED ART") {
		t.Fatalf("response leaked a file from a subdirectory: %q", body)
	}
}

// TestHandleArtFileRefusesADirectoryWearingAnArtFilename pins the one shape that
// passes artlib.IsArtFileName and is not a file: `mkdir art/masonry-1.png`.
//
// artlib REFUSES it — its own test table carries the case ("stat: the picture is
// a directory") and a square naming that id degrades as art that cannot be read
// — so a route answering anything else is a second opinion disagreeing with the
// loader about the same bytes, which is the divergence hazard
// mapdef.LoadInstalled's doc comment is written about.
//
// IT WAS NEVER A LEAK, and this test is not a security fix. Measured 2026-09-05
// before the guard: http.ServeFileFS answered 301 to the same path with a
// trailing slash, and that target matches no route ({file} does not match a
// trailing empty segment), so a real client followed it into a 404 and nothing
// from inside the directory ever came back — which is exactly why
// metadata_test.go's end-to-end version of this case passed WITHOUT the fix. The
// 301 is only observable here, one layer below the client's own redirect
// following, and what it was is an answer nobody asked for from a handler whose
// whole job is to hand back one file.
// VTT-124
func TestHandleArtFileRefusesADirectoryWearingAnArtFilename(t *testing.T) {
	f := newArtHandlerFixture(t)
	f.control(t)

	dir := filepath.Join(f.artDir, "earth-1.png")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inside.txt"), []byte("INSIDE"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, body := f.call("earth-1.png")
	if code != http.StatusNotFound {
		t.Fatalf("handleArtFile on a directory = %d %q, want 404 — art/ holds files", code, body)
	}
	if strings.Contains(body, "INSIDE") {
		t.Fatalf("the response carried something from inside the directory: %q", body)
	}
}
