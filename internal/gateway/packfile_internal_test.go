package gateway

// packfile_internal_test.go covers handlePackFile's traversal defence in
// isolation (maps-as-geometry Task 7), against TWO independent escape
// mechanisms that turned out to need TWO independent proofs:
//
//  1. A literal ".." in the requested name (fs.ValidPath's own contract —
//     TestHandlePackFileRefusesTraversalEvenWithAPathValueSetDirectly).
//  2. A symlink INSIDE the pack directory pointing OUTSIDE it (fixed after
//     review: os.DirFS's own doc comment states plainly that it "does not
//     stop the access any more than using os.Open does" when a file is a
//     symlink pointing outside the tree — TestHandlePackFileRefusesSymlink-
//     Escape). No ".." appears anywhere in THIS request; fs.ValidPath never
//     engages, because a symlink target is resolved by the OS, not by path
//     syntax, so mechanism 1's defence says nothing about mechanism 2.
//
// Production (cmd/vtt/maps.go) uses os.OpenRoot(packDir).FS(), not
// os.DirFS — os.Root's own doc comment: "Methods on Root will follow
// symbolic links, but symbolic links may not reference a location outside
// the root." Both tests below build their OWN Server via New(...).With-
// PackFiles(...) directly and call handlePackFile without going through
// ServeMux, because the property under test is a property of the HANDLER
// (and the fs.FS it is handed), not of the HTTP surface: the full-round-
// trip test in metadata_test.go (TestPackImagesAreServedAndUnknownOnesAre404)
// sends a literal ".." over the wire, but net/http's own ServeMux redirects
// any request whose path contains a ".." element to the cleaned path
// BEFORE pattern matching ever runs, and separately http.ServeFileFS has
// its OWN precaution against a dirty r.URL.Path — both verified directly
// while fault-injecting this task (task-7-report.md). Neither layer says
// anything about a symlink, since neither one involves ".." at all. So
// these two tests are what actually isolates the fs.FS-level defence the
// brief asked for ("rejected by the filesystem abstraction itself rather
// than by a string check").

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// TestHandlePackFileRefusesTraversalEvenWithAPathValueSetDirectly hands
// handlePackFile a PathValue that already IS a traversal string, WHILE
// r.URL.Path itself stays clean (no ".." anywhere in it) — decoupling the
// two on purpose. Fault-injecting this test (task-7-report.md) found TWO
// layers that catch a literal ".." in r.URL.Path before ever reaching
// fsys.Open: net/http's ServeMux (redirects a dirty path before pattern
// matching — irrelevant here, this test calls the handler directly) AND
// http.ServeFileFS's OWN precaution ("reject requests where r.URL.Path
// contains a '..' path element" — its own doc comment), which fires
// regardless of routing. Both would mask a broken fsys, so this test keeps
// r.URL.Path clean and puts the traversal ONLY in PathValue("file") — the
// one input path handlePackFile actually hands to ServeFileFS as the name
// to open — so the only thing standing between this request and the
// secret file is fsys.Open's own fs.ValidPath refusal.
func TestHandlePackFileRefusesTraversalEvenWithAPathValueSetDirectly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
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
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}

	packDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(packDir, "planks_03.png"), []byte("legit"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The "secret" this traversal attempt targets: a file OUTSIDE packDir,
	// in the temp dir's own parent, standing in for /etc/passwd (using a
	// real path this test controls rather than the actual system file,
	// which may not exist or be readable in every sandbox).
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("THE SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(packDir, secretPath)
	if err != nil {
		t.Fatal(err)
	}
	// filepath.Rel on two independent t.TempDir() results is exactly the
	// ".." shape a real traversal payload needs — verified rather than
	// assumed, since a same-parent tempdir layout could in principle
	// collapse this to something with no ".." at all.
	if !strings.Contains(rel, "..") {
		t.Fatalf("test setup bug: rel = %q does not traverse (want a path containing \"..\")", rel)
	}

	// os.OpenRoot, matching production (cmd/vtt/maps.go) — see this file's
	// own package doc for why plain os.DirFS is not used here either.
	root, err := os.OpenRoot(packDir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(c, ids).WithPackFiles(map[string]fs.FS{"mossy-keep": root.FS()})

	w := httptest.NewRecorder()
	// r.URL.Path is deliberately CLEAN (no ".." anywhere in it) — only
	// PathValue("file") carries the traversal string. See this test's own
	// doc comment for why: a dirty r.URL.Path would be caught by
	// http.ServeFileFS's own precaution regardless of fsys, which would
	// prove nothing about fsys.Open's independent defence.
	r := httptest.NewRequest(http.MethodGet, "/api/packs/mossy-keep/planks_03.png", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	r.SetPathValue("pack", "mossy-keep")
	r.SetPathValue("file", rel) // the traversal lives ONLY here

	s.handlePackFile(w, r)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("traversal escaped the pack directory: status 200, body %q", body)
	}
	if strings.Contains(string(body), "THE SECRET") {
		t.Fatalf("response body leaked the secret file's content: %q", body)
	}
}

// TestHandlePackFileRefusesSymlinkEscape proves the SECOND escape mechanism
// (this file's own package doc): a symlink INSIDE the pack directory
// pointing OUTSIDE it, requested by its ordinary name — no ".." anywhere.
// os.DirFS does not defend against this at all (its own doc comment says
// so); os.Root does ("symbolic links may not reference a location outside
// the root").
func TestHandlePackFileRefusesSymlinkEscape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.db")
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
	tok, _, err := ids.CreateInvite("DM", identity.RoleDM)
	if err != nil {
		t.Fatal(err)
	}

	packDir := t.TempDir()
	// The secret this symlink targets — standing in for something real like
	// the campaign's own SQLite file, per the review's own threat model: an
	// operator installs a community pack; ANY authenticated participant of
	// ANY role can fetch a pack file (spec §7, "everyone still sees the
	// whole map" — pack routes are role-open by design).
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("THE SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A pack.json can declare a "file" entry that is, on disk, a symlink —
	// nothing in mapdef or this handler inspects whether a pack's files are
	// regular files. Requested by its perfectly ordinary name; the escape
	// lives entirely in what evil.png POINTS AT, not in the request.
	if err := os.Symlink(secretPath, filepath.Join(packDir, "evil.png")); err != nil {
		t.Fatal(err)
	}

	// os.OpenRoot: the fix. See TestHandlePackFileRefusesSymlinkEscape's own
	// fault injection below (swapped to os.DirFS) for the proof this line
	// is load-bearing.
	root, err := os.OpenRoot(packDir)
	if err != nil {
		t.Fatal(err)
	}
	s := New(c, ids).WithPackFiles(map[string]fs.FS{"mossy-keep": root.FS()})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/packs/mossy-keep/evil.png", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	r.SetPathValue("pack", "mossy-keep")
	r.SetPathValue("file", "evil.png") // no ".." anywhere — that is the whole point

	s.handlePackFile(w, r)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("symlink escaped the pack directory: status 200, body %q", body)
	}
	if strings.Contains(string(body), "THE SECRET") {
		t.Fatalf("response body leaked the secret file's content via a symlink: %q", body)
	}
}
