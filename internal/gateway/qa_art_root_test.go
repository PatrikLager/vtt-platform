package gateway_test

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const qaNoSuchArt = "gateway: no such art"

var (
	qaPictureTime = time.Date(2026, time.March, 14, 15, 9, 26, 0, time.UTC)
	qaSidecarTime = time.Date(2025, time.October, 2, 7, 41, 53, 0, time.UTC)
)

type qaArtTable struct {
	srv   *httptest.Server
	ids   *identity.DB
	token string
}

type qaArtAnswer struct {
	status int
	header http.Header
	body   []byte
	length int64
}

func qaArtServe(t *testing.T, artDir string, setDir bool) *qaArtTable {
	t.Helper()
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ids, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = ids.Close() })
	token, _, err := ids.CreateInvite("Rowan", identity.RolePlayer)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	s := gateway.New(c, ids)
	if setDir {
		s = s.WithArtDir(artDir)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return &qaArtTable{srv: srv, ids: ids, token: token}
}

func qaArtDo(t *testing.T, srv *httptest.Server, method, path, authorization string, extra map[string]string) qaArtAnswer {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest %s %s: %v", method, path, err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	client := srv.Client()
	client.Timeout = 5 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: reading the body: %v", method, path, err)
	}
	return qaArtAnswer{status: resp.StatusCode, header: resp.Header, body: body, length: resp.ContentLength}
}

func (tb *qaArtTable) get(t *testing.T, path string, extra map[string]string) qaArtAnswer {
	t.Helper()
	return qaArtDo(t, tb.srv, http.MethodGet, path, "Bearer "+tb.token, extra)
}

func qaPicture(t *testing.T, w, h int, shade byte) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = shade + byte(i*37)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func qaInstall(t *testing.T, path string, data []byte, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", filepath.Base(path), err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("Chtimes %s: %v", filepath.Base(path), err)
	}
}

func qaArtLayout(t *testing.T) (base, art string, picture, sidecar []byte) {
	t.Helper()
	base = t.TempDir()
	art = filepath.Join(base, "art")
	if err := os.Mkdir(art, 0o755); err != nil {
		t.Fatalf("Mkdir art: %v", err)
	}
	picture = qaPicture(t, 5, 3, 11)
	sidecar = []byte(`{"picture":"brick-path-2.png","note":"<script>alert(1)</script>"}`)
	qaInstall(t, filepath.Join(art, "brick-path-2.png"), picture, qaPictureTime)
	qaInstall(t, filepath.Join(art, "brick-path-2.json"), sidecar, qaSidecarTime)
	return base, art, picture, sidecar
}

func qaWantRefusal(t *testing.T, label string, a qaArtAnswer) {
	t.Helper()
	if a.status != http.StatusNotFound {
		t.Errorf("%s: status = %d, want 404 (body %q)", label, a.status, a.body)
	}
	if got := strings.TrimSuffix(string(a.body), "\n"); got != qaNoSuchArt {
		t.Errorf("%s: body = %q, want %q", label, a.body, qaNoSuchArt)
	}
	if got := a.header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", label, got)
	}
	if cc, ok := a.header["Cache-Control"]; ok {
		t.Errorf("%s: a refusal carries Cache-Control %q, want none", label, cc)
	}
}

func qaWantBytes(t *testing.T, label string, a qaArtAnswer, want []byte) {
	t.Helper()
	if a.status != http.StatusOK {
		t.Fatalf("%s: status = %d, want 200 (body %q)", label, a.status, a.body)
	}
	if !bytes.Equal(a.body, want) {
		t.Errorf("%s: body is %d bytes, want the file's own %d bytes", label, len(a.body), len(want))
	}
	if a.length != int64(len(want)) {
		t.Errorf("%s: Content-Length = %d, want %d", label, a.length, len(want))
	}
}

// VTT-121
func TestQAArtRootServesThePictureAndTheSidecarWithTheirOwnBytes(t *testing.T) {
	_, art, picture, sidecar := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	qaWantBytes(t, "picture", tb.get(t, "/api/art/brick-path-2.png", nil), picture)
	qaWantBytes(t, "sidecar", tb.get(t, "/api/art/brick-path-2.json", nil), sidecar)
}

// VTT-121
func TestQAArtRootAPieceNotInstalledIs404WithTheOneBody(t *testing.T) {
	_, art, _, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	for _, name := range []string{"absent-piece.png", "absent-piece.json", "brick-path-3.png"} {
		qaWantRefusal(t, name, tb.get(t, "/api/art/"+name, nil))
	}
}

// VTT-122
func TestQAArtRootANameThatIsNotAnArtFilenameIs404EvenWhenTheFileExists(t *testing.T) {
	_, art, picture, _ := qaArtLayout(t)
	planted := map[string]string{
		"notes.txt":       "notes",
		"sketch.svg":      `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"page.html":       "<!doctype html><script>alert(1)</script>",
		".hidden.png":     string(picture),
		"a--b.png":        string(picture),
		"-lead.png":       string(picture),
		"trail-.png":      string(picture),
		"piece.png.bak":   string(picture),
		"piece.jpeg":      string(picture),
		"piece":           string(picture),
		"under_score.png": string(picture),
		"dot.ted.png":     string(picture),
		"space d.png":     string(picture),
	}
	for name, body := range planted {
		qaInstall(t, filepath.Join(art, name), []byte(body), qaPictureTime)
	}
	tb := qaArtServe(t, art, true)
	if a := tb.get(t, "/api/art/brick-path-2.png", nil); a.status != http.StatusOK {
		t.Fatalf("control: brick-path-2.png status = %d, want 200", a.status)
	}
	requests := []string{
		"notes.txt", "sketch.svg", "page.html", ".hidden.png", "a--b.png",
		"-lead.png", "trail-.png", "piece.png.bak", "piece.jpeg", "piece",
		"under_score.png", "dot.ted.png", "space%20d.png",
		"brick-path-2.PNG", "Brick-path-2.png", "BRICK-PATH-2.JSON",
		"brick-path-2.png.", "brick-path-2.png%20", "brick-path-2.png%00",
	}
	for _, name := range requests {
		qaWantRefusal(t, name, tb.get(t, "/api/art/"+name, nil))
	}
}

// VTT-123
func TestQAArtRootNothingUnderASubdirectoryIsReachableInAnySpelling(t *testing.T) {
	_, art, _, _ := qaArtLayout(t)
	inner := qaPicture(t, 7, 2, 90)
	if err := os.Mkdir(filepath.Join(art, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	qaInstall(t, filepath.Join(art, "sub", "inner.png"), inner, qaPictureTime)
	tb := qaArtServe(t, art, true)
	for _, path := range []string{
		"/api/art/sub%2Finner.png",
		"/api/art/sub%2finner.png",
		"/api/art/sub%5Cinner.png",
		"/api/art/.%2Fsub%2Finner.png",
		"/api/art/sub%252Finner.png",
		"/api/art/sub%2F%2Finner.png",
	} {
		a := tb.get(t, path, nil)
		qaWantRefusal(t, path, a)
		if bytes.Contains(a.body, inner) {
			t.Errorf("%s: answered the nested file's bytes", path)
		}
	}
	a := tb.get(t, "/api/art/sub/inner.png", nil)
	if a.status == http.StatusOK || bytes.Contains(a.body, inner) {
		t.Errorf("/api/art/sub/inner.png: status %d, nested bytes served = %v", a.status, bytes.Contains(a.body, inner))
	}
}

// VTT-124
func TestQAArtRootADirectoryWearingAnArtFilenameIs404AndServesNothingInside(t *testing.T) {
	_, art, _, _ := qaArtLayout(t)
	inner := qaPicture(t, 4, 4, 200)
	for _, dir := range []string{"folder.png", "folder.json"} {
		if err := os.Mkdir(filepath.Join(art, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		qaInstall(t, filepath.Join(art, dir, "inner.png"), inner, qaPictureTime)
	}
	tb := qaArtServe(t, art, true)
	for _, path := range []string{
		"/api/art/folder.png",
		"/api/art/folder.json",
		"/api/art/folder.png%2Finner.png",
		"/api/art/folder.json%2Finner.png",
	} {
		a := tb.get(t, path, nil)
		qaWantRefusal(t, path, a)
		if bytes.Contains(a.body, inner) {
			t.Errorf("%s: answered the bytes inside the directory", path)
		}
	}
}

// VTT-125
func TestQAArtRootNoRequestReachesAFileOutsideTheArtDirectory(t *testing.T) {
	base, art, _, _ := qaArtLayout(t)
	outside := qaPicture(t, 6, 5, 123)
	outsideJSON := []byte(`{"secret":"outside the art directory"}`)
	qaInstall(t, filepath.Join(base, "outside.png"), outside, qaPictureTime)
	qaInstall(t, filepath.Join(base, "outside.json"), outsideJSON, qaSidecarTime)
	links := map[string]string{
		"rel-out.png":  filepath.Join("..", "outside.png"),
		"abs-out.png":  filepath.Join(base, "outside.png"),
		"rel-out.json": filepath.Join("..", "outside.json"),
		"dir-out.png":  base,
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(art, name)); err != nil {
			t.Fatalf("Symlink %s: %v", name, err)
		}
	}
	tb := qaArtServe(t, art, true)
	leaked := func(a qaArtAnswer) bool {
		return bytes.Contains(a.body, outside) || bytes.Contains(a.body, outsideJSON)
	}
	for _, path := range []string{
		"/api/art/rel-out.png",
		"/api/art/abs-out.png",
		"/api/art/rel-out.json",
		"/api/art/dir-out.png",
		"/api/art/dir-out.png%2Fart%2Fbrick-path-2.png",
		"/api/art/..%2Foutside.png",
		"/api/art/%2E%2E%2Foutside.png",
		"/api/art/..%5Coutside.png",
		"/api/art/..%2F..%2Foutside.json",
	} {
		a := tb.get(t, path, nil)
		qaWantRefusal(t, path, a)
		if leaked(a) {
			t.Errorf("%s: answered bytes from outside the art directory", path)
		}
	}
	a := tb.get(t, "/api/art/../outside.png", nil)
	if a.status == http.StatusOK || leaked(a) {
		t.Errorf("/api/art/../outside.png: status %d, outside bytes served = %v", a.status, leaked(a))
	}
}

// VTT-126
func TestQAArtRootAnArtDirectoryThatIsUnsetAbsentOrUnopenableIs404(t *testing.T) {
	t.Run("never named", func(t *testing.T) {
		tb := qaArtServe(t, "", false)
		qaWantRefusal(t, "unset", tb.get(t, "/api/art/brick-path-2.png", nil))
	})
	t.Run("named empty", func(t *testing.T) {
		tb := qaArtServe(t, "", true)
		qaWantRefusal(t, "empty", tb.get(t, "/api/art/brick-path-2.png", nil))
	})
	t.Run("absent", func(t *testing.T) {
		base := t.TempDir()
		tb := qaArtServe(t, filepath.Join(base, "no-such-art"), true)
		qaWantRefusal(t, "absent", tb.get(t, "/api/art/brick-path-2.png", nil))
	})
	t.Run("a regular file", func(t *testing.T) {
		base := t.TempDir()
		path := filepath.Join(base, "art")
		qaInstall(t, path, qaPicture(t, 2, 2, 1), qaPictureTime)
		tb := qaArtServe(t, path, true)
		qaWantRefusal(t, "file as dir", tb.get(t, "/api/art/brick-path-2.png", nil))
	})
	for _, mode := range []os.FileMode{0o000, 0o600, 0o300} {
		t.Run("mode "+strconv.FormatUint(uint64(mode), 8), func(t *testing.T) {
			if os.Geteuid() == 0 {
				t.Skip("root reads a directory whatever its mode")
			}
			_, art, _, _ := qaArtLayout(t)
			if err := os.Chmod(art, mode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(art, 0o755) })
			tb := qaArtServe(t, art, true)
			qaWantRefusal(t, "picture", tb.get(t, "/api/art/brick-path-2.png", nil))
			qaWantRefusal(t, "sidecar", tb.get(t, "/api/art/brick-path-2.json", nil))
		})
	}
}

// VTT-126
func TestQAArtRootAnArtDirectoryRemovedWhileServingIs404OnTheNextRequest(t *testing.T) {
	_, art, picture, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	qaWantBytes(t, "before removal", tb.get(t, "/api/art/brick-path-2.png", nil), picture)
	if err := os.RemoveAll(art); err != nil {
		t.Fatal(err)
	}
	qaWantRefusal(t, "after removal", tb.get(t, "/api/art/brick-path-2.png", nil))
}

// VTT-127
func TestQAArtRootNoResponseNamesAPathOnTheServer(t *testing.T) {
	base, art, _, _ := qaArtLayout(t)
	if err := os.Mkdir(filepath.Join(art, "folder.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "elsewhere.png"), filepath.Join(art, "dangling.png")); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(art, "locked.png")
	qaInstall(t, locked, qaPicture(t, 3, 3, 3), qaPictureTime)
	if os.Geteuid() != 0 {
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	}
	tb := qaArtServe(t, art, true)
	absent := qaArtServe(t, filepath.Join(base, "no-such-art"), true)
	paths := []string{base, art}
	if resolved, err := filepath.EvalSymlinks(base); err == nil {
		paths = append(paths, resolved)
	}
	check := func(label string, a qaArtAnswer) {
		t.Helper()
		for _, p := range paths {
			if bytes.Contains(a.body, []byte(p)) {
				t.Errorf("%s: body names %q", label, p)
			}
			for k, vs := range a.header {
				for _, v := range vs {
					if strings.Contains(v, p) {
						t.Errorf("%s: header %s names %q", label, k, p)
					}
				}
			}
		}
	}
	for _, path := range []string{
		"/api/art/brick-path-2.png",
		"/api/art/brick-path-2.json",
		"/api/art/absent-piece.png",
		"/api/art/folder.png",
		"/api/art/dangling.png",
		"/api/art/locked.png",
		"/api/art/notes.txt",
		"/api/art/..%2Fart%2Fbrick-path-2.png",
	} {
		check(path, tb.get(t, path, nil))
	}
	check("absent directory", absent.get(t, "/api/art/brick-path-2.png", nil))
	check("unsatisfiable range", tb.get(t, "/api/art/brick-path-2.png", map[string]string{"Range": "bytes=100000-"}))
}

// VTT-128
func TestQAArtRootAPictureIsServedInlineAsImagePNGWithSniffingForbidden(t *testing.T) {
	_, art, _, _ := qaArtLayout(t)
	decoy := []byte("<!doctype html><html><body><script>alert(document.cookie)</script></body></html>")
	qaInstall(t, filepath.Join(art, "decoy-page.png"), decoy, qaPictureTime)
	tb := qaArtServe(t, art, true)
	for _, name := range []string{"brick-path-2.png", "decoy-page.png"} {
		a := tb.get(t, "/api/art/"+name, nil)
		if a.status != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", name, a.status)
		}
		if got := a.header.Get("Content-Type"); got != "image/png" {
			t.Errorf("%s: Content-Type = %q, want image/png", name, got)
		}
		if got := a.header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", name, got)
		}
		if got := a.header.Get("Content-Disposition"); strings.HasPrefix(strings.ToLower(got), "attachment") {
			t.Errorf("%s: Content-Disposition = %q, want the picture inline", name, got)
		}
	}
}

// VTT-129
func TestQAArtRootASidecarIsServedAsAnOctetStreamAttachmentWithSniffingForbidden(t *testing.T) {
	_, art, _, sidecar := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	a := tb.get(t, "/api/art/brick-path-2.json", nil)
	qaWantBytes(t, "sidecar", a, sidecar)
	if got := a.header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", got)
	}
	if got := a.header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := a.header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", got)
	}
}

// VTT-130
func TestQAArtRootArtInstalledOrOverwrittenWhileServingIsServedOnTheNextRequest(t *testing.T) {
	base, art, _, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	path := filepath.Join(art, "late-piece.png")
	qaWantRefusal(t, "before install", tb.get(t, "/api/art/late-piece.png", nil))

	first := qaPicture(t, 3, 3, 40)
	qaInstall(t, path, first, qaPictureTime)
	qaWantBytes(t, "installed", tb.get(t, "/api/art/late-piece.png", nil), first)

	second := qaPicture(t, 9, 4, 77)
	qaInstall(t, path, second, qaPictureTime.Add(90*time.Minute))
	qaWantBytes(t, "overwritten", tb.get(t, "/api/art/late-piece.png", nil), second)

	if err := os.Rename(art, filepath.Join(base, "art-old")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(art, 0o755); err != nil {
		t.Fatal(err)
	}
	third := qaPicture(t, 2, 8, 150)
	qaInstall(t, path, third, qaPictureTime.Add(3*time.Hour))
	qaWantBytes(t, "directory replaced", tb.get(t, "/api/art/late-piece.png", nil), third)
	qaWantRefusal(t, "gone with the old directory", tb.get(t, "/api/art/brick-path-2.png", nil))
}

// VTT-131
func TestQAArtRootAnArtResponseRequiresRevalidationAndCarriesItsModificationTime(t *testing.T) {
	_, art, _, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	for name, mtime := range map[string]time.Time{
		"brick-path-2.png":  qaPictureTime,
		"brick-path-2.json": qaSidecarTime,
	} {
		a := tb.get(t, "/api/art/"+name, nil)
		if a.status != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", name, a.status)
		}
		if got := a.header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want no-cache", name, got)
		}
		if got, want := a.header.Get("Last-Modified"), mtime.Format(http.TimeFormat); got != want {
			t.Errorf("%s: Last-Modified = %q, want %q", name, got, want)
		}
	}
}

// VTT-131
func TestQAArtRootAConditionalRequestForUnchangedArtIs304WithNoBody(t *testing.T) {
	_, art, _, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	for name, mtime := range map[string]time.Time{
		"brick-path-2.png":  qaPictureTime,
		"brick-path-2.json": qaSidecarTime,
	} {
		for label, since := range map[string]time.Time{
			"equal": mtime,
			"later": mtime.Add(time.Hour),
		} {
			a := tb.get(t, "/api/art/"+name, map[string]string{"If-Modified-Since": since.Format(http.TimeFormat)})
			if a.status != http.StatusNotModified {
				t.Errorf("%s since %s: status = %d, want 304", name, label, a.status)
			}
			if len(a.body) != 0 {
				t.Errorf("%s since %s: 304 carries a %d-byte body", name, label, len(a.body))
			}
			if got := a.header.Get("Cache-Control"); got != "no-cache" {
				t.Errorf("%s since %s: Cache-Control = %q, want no-cache", name, label, got)
			}
		}
		a := tb.get(t, "/api/art/"+name, map[string]string{"If-Modified-Since": mtime.Add(-time.Second).Format(http.TimeFormat)})
		if a.status != http.StatusOK || len(a.body) == 0 {
			t.Errorf("%s since a second before: status = %d with %d bytes, want 200 with the file", name, a.status, len(a.body))
		}
	}
	changed := qaPicture(t, 6, 6, 9)
	qaInstall(t, filepath.Join(art, "brick-path-2.png"), changed, qaPictureTime.Add(24*time.Hour))
	a := tb.get(t, "/api/art/brick-path-2.png", map[string]string{"If-Modified-Since": qaPictureTime.Format(http.TimeFormat)})
	qaWantBytes(t, "overwritten since the browser's copy", a, changed)
}

func TestQAArtRootAHeadRequestAnswersTheHeadersWithoutTheBytes(t *testing.T) {
	_, art, picture, sidecar := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	head := func(name string) qaArtAnswer {
		return qaArtDo(t, tb.srv, http.MethodHead, "/api/art/"+name, "Bearer "+tb.token, nil)
	}
	for name, want := range map[string]struct {
		size  int
		ctype string
		mtime time.Time
	}{
		"brick-path-2.png":  {len(picture), "image/png", qaPictureTime},
		"brick-path-2.json": {len(sidecar), "application/octet-stream", qaSidecarTime},
	} {
		a := head(name)
		if a.status != http.StatusOK {
			t.Fatalf("HEAD %s: status = %d, want 200", name, a.status)
		}
		if len(a.body) != 0 {
			t.Errorf("HEAD %s: body is %d bytes, want none", name, len(a.body))
		}
		if a.length != int64(want.size) {
			t.Errorf("HEAD %s: Content-Length = %d, want %d", name, a.length, want.size)
		}
		if got := a.header.Get("Content-Type"); got != want.ctype {
			t.Errorf("HEAD %s: Content-Type = %q, want %q", name, got, want.ctype)
		}
		if got := a.header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("HEAD %s: X-Content-Type-Options = %q, want nosniff", name, got)
		}
		if got := a.header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("HEAD %s: Cache-Control = %q, want no-cache", name, got)
		}
		if got := a.header.Get("Last-Modified"); got != want.mtime.Format(http.TimeFormat) {
			t.Errorf("HEAD %s: Last-Modified = %q, want %q", name, got, want.mtime.Format(http.TimeFormat))
		}
	}
	if got := head("brick-path-2.json").header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Errorf("HEAD sidecar: Content-Disposition = %q, want attachment", got)
	}
	if a := head("absent-piece.png"); a.status != http.StatusNotFound {
		t.Errorf("HEAD absent-piece.png: status = %d, want 404", a.status)
	}
}

func TestQAArtRootARangeRequestIsAnsweredWithThoseBytes(t *testing.T) {
	_, art, picture, sidecar := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	n := len(picture)
	cases := []struct {
		rng       string
		from, end int
	}{
		{"bytes=2-5", 2, 6},
		{"bytes=-4", n - 4, n},
		{"bytes=10-", 10, n},
		{"bytes=7-7", 7, 8},
	}
	for _, c := range cases {
		a := tb.get(t, "/api/art/brick-path-2.png", map[string]string{"Range": c.rng})
		if a.status != http.StatusPartialContent {
			t.Errorf("%s: status = %d, want 206", c.rng, a.status)
			continue
		}
		if !bytes.Equal(a.body, picture[c.from:c.end]) {
			t.Errorf("%s: body = %x, want %x", c.rng, a.body, picture[c.from:c.end])
		}
		wantRange := "bytes " + strconv.Itoa(c.from) + "-" + strconv.Itoa(c.end-1) + "/" + strconv.Itoa(n)
		if got := a.header.Get("Content-Range"); got != wantRange {
			t.Errorf("%s: Content-Range = %q, want %q", c.rng, got, wantRange)
		}
		if got := a.header.Get("Content-Type"); got != "image/png" {
			t.Errorf("%s: Content-Type = %q, want image/png", c.rng, got)
		}
		if got := a.header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", c.rng, got)
		}
		if got := a.header.Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control = %q, want no-cache", c.rng, got)
		}
	}
	a := tb.get(t, "/api/art/brick-path-2.json", map[string]string{"Range": "bytes=3-9"})
	if a.status != http.StatusPartialContent || !bytes.Equal(a.body, sidecar[3:10]) {
		t.Errorf("sidecar bytes=3-9: status %d body %q, want 206 %q", a.status, a.body, sidecar[3:10])
	}
	if got := a.header.Get("Content-Type"); got != "application/octet-stream" {
		t.Errorf("sidecar range: Content-Type = %q, want application/octet-stream", got)
	}
	if got := a.header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Errorf("sidecar range: Content-Disposition = %q, want attachment", got)
	}
}

func TestQAArtRootAMultiRangeRequestIsAnsweredPartByPartAsAPicture(t *testing.T) {
	_, art, picture, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	a := tb.get(t, "/api/art/brick-path-2.png", map[string]string{"Range": "bytes=1-3,12-16"})
	if a.status != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", a.status)
	}
	mt, params, err := mime.ParseMediaType(a.header.Get("Content-Type"))
	if err != nil || mt != "multipart/byteranges" {
		t.Fatalf("Content-Type = %q (%v), want multipart/byteranges", a.header.Get("Content-Type"), err)
	}
	mr := multipart.NewReader(bytes.NewReader(a.body), params["boundary"])
	want := [][]byte{picture[1:4], picture[12:17]}
	for i := 0; ; i++ {
		part, err := mr.NextPart()
		if err == io.EOF {
			if i != len(want) {
				t.Errorf("got %d parts, want %d", i, len(want))
			}
			break
		}
		if err != nil {
			t.Fatalf("part %d: %v", i, err)
		}
		got, _ := io.ReadAll(part)
		if i < len(want) && !bytes.Equal(got, want[i]) {
			t.Errorf("part %d = %x, want %x", i, got, want[i])
		}
		if ct := part.Header.Get("Content-Type"); ct != "image/png" {
			t.Errorf("part %d: Content-Type = %q, want image/png", i, ct)
		}
	}
}

func TestQAArtRootIfRangeServesTheRangeOnlyForTheUnchangedFile(t *testing.T) {
	_, art, picture, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	same := tb.get(t, "/api/art/brick-path-2.png", map[string]string{
		"Range":    "bytes=2-5",
		"If-Range": qaPictureTime.Format(http.TimeFormat),
	})
	if same.status != http.StatusPartialContent || !bytes.Equal(same.body, picture[2:6]) {
		t.Errorf("If-Range unchanged: status %d body %x, want 206 %x", same.status, same.body, picture[2:6])
	}
	stale := tb.get(t, "/api/art/brick-path-2.png", map[string]string{
		"Range":    "bytes=2-5",
		"If-Range": qaPictureTime.Add(-time.Hour).Format(http.TimeFormat),
	})
	qaWantBytes(t, "If-Range stale", stale, picture)
}

func TestQAArtRootAnUnsatisfiableRangeIsRefusedWithoutTheBytes(t *testing.T) {
	_, art, picture, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	a := tb.get(t, "/api/art/brick-path-2.png", map[string]string{"Range": "bytes=100000-"})
	if a.status != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("status = %d, want 416", a.status)
	}
	if want := "bytes */" + strconv.Itoa(len(picture)); a.header.Get("Content-Range") != want {
		t.Errorf("Content-Range = %q, want %q", a.header.Get("Content-Range"), want)
	}
	if bytes.Contains(a.body, picture[:8]) {
		t.Errorf("an unsatisfiable range answered picture bytes")
	}
}

// VTT-682
func TestQAArtRootAnArtFileTheServerCanStatButNotReadIs404WithTheOneBody(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a file whatever its mode")
	}
	_, art, _, _ := qaArtLayout(t)
	for _, name := range []string{"brick-path-2.png", "brick-path-2.json"} {
		p := filepath.Join(art, name)
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("fixture: %s must stay statable: %v", name, err)
		}
		if f, err := os.Open(p); err == nil {
			f.Close()
			t.Fatalf("fixture: %s must be unreadable", name)
		}
	}
	tb := qaArtServe(t, art, true)
	for _, name := range []string{"brick-path-2.png", "brick-path-2.json"} {
		path := "/api/art/" + name
		qaWantRefusal(t, "GET "+name, tb.get(t, path, nil))
		qaWantRefusal(t, "range "+name, tb.get(t, path, map[string]string{"Range": "bytes=2-5"}))
		qaWantRefusal(t, "conditional "+name, tb.get(t, path, map[string]string{
			"If-Modified-Since": qaPictureTime.Add(48 * time.Hour).Format(http.TimeFormat),
		}))
		head := qaArtDo(t, tb.srv, http.MethodHead, path, "Bearer "+tb.token, nil)
		if head.status != http.StatusNotFound {
			t.Errorf("HEAD %s: status = %d, want 404", name, head.status)
		}
	}
}

func TestQAArtRootAFIFOWearingAnArtFilenameIs404WithoutBlocking(t *testing.T) {
	_, art, _, _ := qaArtLayout(t)
	fifo := filepath.Join(art, "pipe-piece.png")
	if err := exec.Command("mkfifo", fifo).Run(); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	tb := qaArtServe(t, art, true)
	req, err := http.NewRequest(http.MethodGet, tb.srv.URL+"/api/art/pipe-piece.png", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tb.token)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		go func() {
			if f, err := os.OpenFile(fifo, os.O_WRONLY, 0); err == nil {
				_ = f.Close()
			}
		}()
		t.Fatalf("GET a FIFO wearing an art name did not answer: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	qaWantRefusal(t, "fifo", qaArtAnswer{status: resp.StatusCode, header: resp.Header, body: body, length: resp.ContentLength})
}

// VTT-709
func TestQAArtRootArtIsAnsweredToEveryRole(t *testing.T) {
	_, art, picture, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	for _, role := range []identity.Role{identity.RoleDM, identity.RoleAgent, identity.RolePlayer, identity.RoleSpectator} {
		token, _, err := tb.ids.CreateInvite("seat-"+string(role), role)
		if err != nil {
			t.Fatalf("CreateInvite %s: %v", role, err)
		}
		a := qaArtDo(t, tb.srv, http.MethodGet, "/api/art/brick-path-2.png", "Bearer "+token, nil)
		qaWantBytes(t, string(role), a, picture)
	}
}

// VTT-581
func TestQAArtRootARequestWithoutAGoodBearerIs401(t *testing.T) {
	_, art, picture, _ := qaArtLayout(t)
	tb := qaArtServe(t, art, true)
	revokedToken, revokedID, err := tb.ids.CreateInvite("Leaver", identity.RolePlayer)
	if err != nil {
		t.Fatal(err)
	}
	if err := tb.ids.Revoke(revokedID); err != nil {
		t.Fatal(err)
	}
	if a := qaArtDo(t, tb.srv, http.MethodGet, "/api/art/brick-path-2.png", "Bearer "+revokedToken, nil); a.status != http.StatusUnauthorized {
		t.Fatalf("control: the revoked token answered %d, want 401", a.status)
	}
	auths := map[string]string{
		"no header":     "",
		"no prefix":     tb.token,
		"basic":         "Basic " + tb.token,
		"lower bearer":  "bearer " + tb.token,
		"unknown token": "Bearer not-a-token-anyone-was-given",
		"revoked token": "Bearer " + revokedToken,
	}
	for _, name := range []string{"brick-path-2.png", "absent-piece.png", "notes.txt", "sub%2Finner.png"} {
		for label, auth := range auths {
			a := qaArtDo(t, tb.srv, http.MethodGet, "/api/art/"+name, auth, nil)
			if a.status != http.StatusUnauthorized {
				t.Errorf("%s, %s: status = %d, want 401", name, label, a.status)
			}
			if got := strings.TrimSuffix(string(a.body), "\n"); got != "gateway: unauthorized" {
				t.Errorf("%s, %s: body = %q, want gateway: unauthorized", name, label, a.body)
			}
			if bytes.Contains(a.body, picture[:8]) {
				t.Errorf("%s, %s: answered picture bytes", name, label)
			}
		}
		a := qaArtDo(t, tb.srv, http.MethodGet, "/api/art/"+name+"?token="+tb.token, "", nil)
		if a.status != http.StatusUnauthorized {
			t.Errorf("%s with ?token=: status = %d, want 401", name, a.status)
		}
	}
}
