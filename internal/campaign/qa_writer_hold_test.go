package campaign_test

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	_ "modernc.org/sqlite"
)

const (
	qaHoldPromptly    = time.Second
	qaHoldHolderStart = 20 * time.Second
	qaHoldHolderEnv   = "VTT_CAMPAIGN_HOLDER_DIR"
)

type qaHoldLines struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	all   bytes.Buffer
	lines chan string
}

func (w *qaHoldLines) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.all.Write(p)
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.Reset()
			w.buf.WriteString(line)
			return len(p), nil
		}
		select {
		case w.lines <- strings.TrimRight(line, "\r\n"):
		default:
		}
	}
}

func (w *qaHoldLines) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.all.String()
}

type qaHoldHolder struct {
	cmd    *exec.Cmd
	first  string
	stderr *qaHoldLines
}

func qaHoldSpawnHolder(t *testing.T, dir string) *qaHoldHolder {
	t.Helper()
	out := &qaHoldLines{lines: make(chan string, 64)}
	errOut := &qaHoldLines{lines: make(chan string, 64)}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), qaHoldHolderEnv+"="+dir)
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Start(); err != nil {
		t.Fatalf("start holder: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	deadline := time.After(qaHoldHolderStart)
	for {
		select {
		case line := <-out.lines:
			if line == "HELD" || strings.HasPrefix(line, "REFUSED") {
				return &qaHoldHolder{cmd: cmd, first: line, stderr: errOut}
			}
		case <-deadline:
			t.Fatalf("holder printed neither HELD nor REFUSED within %v; stdout %q stderr %q", qaHoldHolderStart, out.String(), errOut.String())
		}
	}
}

func qaHoldMustHold(t *testing.T, dir string) *qaHoldHolder {
	t.Helper()
	h := qaHoldSpawnHolder(t, dir)
	if h.first != "HELD" {
		t.Fatalf("holder on %s answered %q, want HELD", dir, h.first)
	}
	return h
}

type qaHoldOpened struct {
	c   *campaign.Campaign
	err error
}

func qaHoldOpenPromptly(t *testing.T, dir string) (*campaign.Campaign, error) {
	t.Helper()
	done := make(chan qaHoldOpened, 1)
	start := time.Now()
	go func() {
		c, err := campaign.Open(dir)
		done <- qaHoldOpened{c, err}
	}()
	select {
	case r := <-done:
		t.Logf("campaign.Open(%s) returned in %v", dir, time.Since(start))
		if r.c != nil {
			t.Cleanup(func() { _ = r.c.Close() })
		}
		return r.c, r.err
	case <-time.After(qaHoldPromptly):
		t.Fatalf("campaign.Open(%s) did not return within %v", dir, qaHoldPromptly)
		return nil, nil
	}
}

func qaHoldMustOpen(t *testing.T, dir string) *campaign.Campaign {
	t.Helper()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open(%s): %v", dir, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func qaHoldAssertHeldRefusal(t *testing.T, c *campaign.Campaign, err error, dir string) {
	t.Helper()
	if c != nil {
		t.Fatalf("campaign.Open(%s) returned a Campaign on a held directory", dir)
	}
	if !errors.Is(err, campaign.ErrHeld) {
		t.Fatalf("campaign.Open(%s) error %v does not wrap ErrHeld", dir, err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("campaign.Open error %q does not name the directory %s", err.Error(), dir)
	}
}

func qaHoldFlock(t *testing.T, dir string, how int) (release func(), err error) {
	t.Helper()
	fd, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open %s for flock: %v", dir, err)
	}
	if err := syscall.Flock(fd, how|syscall.LOCK_NB); err != nil {
		_ = syscall.Close(fd)
		return func() {}, err
	}
	var once sync.Once
	release = func() { once.Do(func() { _ = syscall.Close(fd) }) }
	t.Cleanup(release)
	return release, nil
}

func qaHoldLogDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+campaign.LogPath(dir)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open log.db raw: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func qaHoldInsertCorruptEvent(t *testing.T, dir string) {
	t.Helper()
	db := qaHoldLogDB(t, dir)
	if _, err := db.Exec(`INSERT INTO events(event_id, session_id, occurred_at, payload) VALUES ('qa-corrupt', '', '2026-10-05T00:00:00Z', X'FFFFFFFF')`); err != nil {
		t.Fatalf("insert corrupt event: %v", err)
	}
	_ = db.Close()
}

func qaHoldDeleteCorruptEvent(t *testing.T, dir string) {
	t.Helper()
	db := qaHoldLogDB(t, dir)
	if _, err := db.Exec(`DELETE FROM events WHERE event_id = 'qa-corrupt'`); err != nil {
		t.Fatalf("delete corrupt event: %v", err)
	}
	_ = db.Close()
}

func qaHoldUmask() os.FileMode {
	old := syscall.Umask(0)
	syscall.Umask(old)
	return os.FileMode(old)
}

func qaHoldAssertMode0750(t *testing.T, dir string) {
	t.Helper()
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if !fi.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
	want := os.FileMode(0o750) &^ qaHoldUmask()
	if got := fi.Mode().Perm(); got != want {
		t.Fatalf("%s has mode %o, want %o", dir, got, want)
	}
}

// VTT-267
func TestQAHoldOpenRefusesADirectoryHeldInThisProcessAtOnce(t *testing.T) {
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)

	c, err := qaHoldOpenPromptly(t, dir)
	qaHoldAssertHeldRefusal(t, c, err, dir)

	c, err = qaHoldOpenPromptly(t, dir)
	qaHoldAssertHeldRefusal(t, c, err, dir)
}

// VTT-267
func TestQAHoldOpenRefusesADirectoryHeldByAnotherProcessAtOnce(t *testing.T) {
	dir := t.TempDir()
	qaHoldMustHold(t, dir)

	c, err := qaHoldOpenPromptly(t, dir)
	qaHoldAssertHeldRefusal(t, c, err, dir)
}

// VTT-267
func TestQAHoldAnotherProcessIsRefusedADirectoryThisProcessHolds(t *testing.T) {
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)

	h := qaHoldSpawnHolder(t, dir)
	if !strings.HasPrefix(h.first, "REFUSED") {
		t.Fatalf("second process answered %q on a held directory, want REFUSED", h.first)
	}
	if !strings.Contains(h.first, campaign.ErrHeld.Error()) || !strings.Contains(h.first, dir) {
		t.Fatalf("second process refusal %q does not carry ErrHeld's text and the directory %s", h.first, dir)
	}
	waited := make(chan error, 1)
	go func() { waited <- h.cmd.Wait() }()
	select {
	case <-waited:
	case <-time.After(qaHoldHolderStart):
		t.Fatalf("refused process did not exit within %v", qaHoldHolderStart)
	}
	if code := h.cmd.ProcessState.ExitCode(); code != 1 {
		t.Fatalf("refused process exit code %d, want 1", code)
	}
}

// SPEC-019 How it works: "an error that wraps campaign.ErrHeld and names the
// directory: campaign: another writer holds the campaign directory <dir>; a
// campaign has one writer at a time, so stop the vtt serve or close the
// Campaign that has it open"
func TestQAHoldOpenRefusalIsTheRecordedText(t *testing.T) {
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)

	_, err := qaHoldOpenPromptly(t, dir)
	want := "campaign: another writer holds the campaign directory " + dir +
		"; a campaign has one writer at a time, so stop the vtt serve or close the Campaign that has it open"
	if err == nil || err.Error() != want {
		t.Fatalf("refusal text\n got %v\nwant %s", err, want)
	}
}

// SPEC-019 How it works: "The hold is an exclusive flock on the campaign directory."
func TestQAHoldAHeldDirectoryRefusesAForeignFlock(t *testing.T) {
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)

	if _, err := qaHoldFlock(t, dir, syscall.LOCK_SH); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("shared flock on a held directory: got %v, want EWOULDBLOCK", err)
	}
	if _, err := qaHoldFlock(t, dir, syscall.LOCK_EX); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("exclusive flock on a held directory: got %v, want EWOULDBLOCK", err)
	}
}

// SPEC-019 How it works: "The hold is an exclusive flock on the campaign directory."
func TestQAHoldAForeignFlockOnTheDirectoryRefusesOpen(t *testing.T) {
	dir := t.TempDir()
	if _, err := qaHoldFlock(t, dir, syscall.LOCK_EX); err != nil {
		t.Fatalf("foreign flock: %v", err)
	}

	c, err := qaHoldOpenPromptly(t, dir)
	qaHoldAssertHeldRefusal(t, c, err, dir)
}

// VTT-271
func TestQAHoldARefusedOpenerNeverOpensTheLog(t *testing.T) {
	dir := t.TempDir()
	if _, err := qaHoldFlock(t, dir, syscall.LOCK_EX); err != nil {
		t.Fatalf("foreign flock: %v", err)
	}

	if c, err := qaHoldOpenPromptly(t, dir); c != nil || err == nil {
		t.Fatalf("campaign.Open on a flocked directory: c=%v err=%v, want a refusal", c, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("a refused opener left %v in the directory, want it empty", names)
	}
}

// SPEC-019 How it works: "Nothing is created in the directory for the hold."
func TestQAHoldOpenCreatesNothingInTheDirectoryForTheHold(t *testing.T) {
	dir := t.TempDir()
	logName := filepath.Base(campaign.LogPath(dir))
	entries := func(when string) []string {
		t.Helper()
		list, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		names := make([]string, 0, len(list))
		for _, e := range list {
			if !e.IsDir() && !strings.HasPrefix(e.Name(), logName) {
				t.Fatalf("%s: directory holds the file %q, which is not the log %s", when, e.Name(), logName)
			}
			names = append(names, e.Name())
		}
		return names
	}
	c := qaHoldMustOpen(t, dir)
	held := entries("while held")
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if after := entries("after Close"); !slices.Equal(held, after) {
		t.Fatalf("the directory held %v while held and %v after Close", held, after)
	}
}

// SPEC-019 How it works: "EnsureDir, which refuses a path that is a plain file
// and creates a missing directory with mode 0o750"
func TestQAHoldEnsureDirCreatesAMissingDirectoryWithMode0750(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "qa-campaign")
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir(%s): %v", dir, err)
	}
	qaHoldAssertMode0750(t, dir)
	if err := campaign.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir on the directory it made: %v", err)
	}
}

// SPEC-019 How it works: "campaign.Open first calls EnsureDir, which refuses a
// path that is a plain file and creates a missing directory with mode 0o750."
func TestQAHoldOpenCreatesAMissingDirectoryWithMode0750(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "qa-campaign")
	qaHoldMustOpen(t, dir)
	qaHoldAssertMode0750(t, dir)
}

// SPEC-019 How it works: "campaign.Open first calls EnsureDir, which refuses a
// path that is a plain file and creates a missing directory with mode 0o750."
func TestQAHoldEnsureDirAndOpenRefuseAPlainFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "qa-plain")
	content := []byte("not a campaign")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write plain file: %v", err)
	}

	if err := campaign.EnsureDir(path); err == nil {
		t.Fatalf("EnsureDir(%s) accepted a plain file", path)
	}
	c, err := qaHoldOpenPromptly(t, path)
	if c != nil || err == nil {
		t.Fatalf("campaign.Open(%s) on a plain file: c=%v err=%v, want a refusal", path, c, err)
	}
	if errors.Is(err, campaign.ErrHeld) {
		t.Fatalf("campaign.Open on a plain file answered ErrHeld: %v", err)
	}
	got, rerr := os.ReadFile(path)
	if rerr != nil || !bytes.Equal(got, content) {
		t.Fatalf("plain file changed: %q, %v", got, rerr)
	}
}

// SPEC-019 Who does not: "vtt invite, vtt revoke and vtt join-link call
// campaign.EnsureDir and then identity.Open(campaign.LogPath(dir)), so they
// work beside a vtt serve on the same directory."
func TestQAHoldEnsureDirWorksBesideAHolderAndTakesNoHold(t *testing.T) {
	held := t.TempDir()
	qaHoldMustOpen(t, held)
	if err := campaign.EnsureDir(held); err != nil {
		t.Fatalf("EnsureDir on a held directory: %v", err)
	}

	fresh := filepath.Join(t.TempDir(), "qa-fresh")
	if err := campaign.EnsureDir(fresh); err != nil {
		t.Fatalf("EnsureDir(%s): %v", fresh, err)
	}
	if c, err := qaHoldOpenPromptly(t, fresh); err != nil || c == nil {
		t.Fatalf("campaign.Open after EnsureDir: c=%v err=%v", c, err)
	}
}

// VTT-268
func TestQAHoldClosingACampaignEndsTheHold(t *testing.T) {
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	c2, err := qaHoldOpenPromptly(t, dir)
	if err != nil || c2 == nil {
		t.Fatalf("campaign.Open after Close: c=%v err=%v", c2, err)
	}
	if err := c2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	release, err := qaHoldFlock(t, dir, syscall.LOCK_EX)
	if err != nil {
		t.Fatalf("exclusive flock after Close: %v", err)
	}
	release()
	qaHoldMustHold(t, dir)
}

// SPEC-019 When the hold ends: "Close closes the log and then the directory's
// descriptor; a second Close returns nil."
func TestQAHoldSecondCloseReturnsNil(t *testing.T) {
	c, err := campaign.Open(t.TempDir())
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v, want nil", err)
	}
}

// VTT-268
func TestQAHoldOpenThatFailsOpeningTheLogEndsTheHold(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(campaign.LogPath(dir), 0o750); err != nil {
		t.Fatalf("make the log path a directory: %v", err)
	}

	for i := 0; i < 2; i++ {
		c, err := qaHoldOpenPromptly(t, dir)
		if c != nil || err == nil {
			t.Fatalf("campaign.Open with a directory at the log path: c=%v err=%v, want a failure", c, err)
		}
		if errors.Is(err, campaign.ErrHeld) {
			t.Fatalf("Open %d after a failed Open answered ErrHeld: %v", i+1, err)
		}
	}
	release, err := qaHoldFlock(t, dir, syscall.LOCK_EX)
	if err != nil {
		t.Fatalf("exclusive flock after a failed Open: %v", err)
	}
	release()

	if err := os.Remove(campaign.LogPath(dir)); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if c, err := qaHoldOpenPromptly(t, dir); err != nil || c == nil {
		t.Fatalf("campaign.Open once the log path is free: c=%v err=%v", c, err)
	}
}

// VTT-268
func TestQAHoldOpenThatFailsReadingTheLogEndsTheHold(t *testing.T) {
	dir := t.TempDir()
	c := qaHoldMustOpen(t, dir)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	qaHoldInsertCorruptEvent(t, dir)

	for i := 0; i < 2; i++ {
		c, err := qaHoldOpenPromptly(t, dir)
		if c != nil || err == nil {
			t.Fatalf("campaign.Open on a corrupt log: c=%v err=%v, want a failure", c, err)
		}
		if errors.Is(err, campaign.ErrHeld) {
			t.Fatalf("Open %d after a failed Open answered ErrHeld: %v", i+1, err)
		}
	}
	release, err := qaHoldFlock(t, dir, syscall.LOCK_EX)
	if err != nil {
		t.Fatalf("exclusive flock after a failed Open: %v", err)
	}
	release()

	qaHoldDeleteCorruptEvent(t, dir)
	if c, err := qaHoldOpenPromptly(t, dir); err != nil || c == nil {
		t.Fatalf("campaign.Open once the log is whole: c=%v err=%v", c, err)
	}
}

// VTT-269
func TestQAHoldEndsWhenItsHolderIsKilled(t *testing.T) {
	dir := t.TempDir()
	h := qaHoldMustHold(t, dir)

	if err := h.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("kill holder: %v", err)
	}
	_ = h.cmd.Wait()
	ws, ok := h.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() || ws.Signal() != syscall.SIGKILL {
		t.Fatalf("holder was not killed by SIGKILL: %v", h.cmd.ProcessState)
	}

	if c, err := qaHoldOpenPromptly(t, dir); err != nil || c == nil {
		t.Fatalf("campaign.Open after the holder was killed: c=%v err=%v", c, err)
	}
}

// VTT-269
func TestQAHoldEndsWhenItsHolderExitsWithoutClosing(t *testing.T) {
	dir := t.TempDir()
	h := qaHoldMustHold(t, dir)

	if err := h.cmd.Process.Signal(syscall.SIGQUIT); err != nil {
		t.Fatalf("signal holder: %v", err)
	}
	_ = h.cmd.Wait()
	if !h.cmd.ProcessState.Exited() {
		t.Fatalf("holder did not exit on its own: %v", h.cmd.ProcessState)
	}

	if c, err := qaHoldOpenPromptly(t, dir); err != nil || c == nil {
		t.Fatalf("campaign.Open after the holder exited: c=%v err=%v", c, err)
	}
}

// VTT-269
func TestQAHoldNoChildOfTheHolderKeepsTheHold(t *testing.T) {
	dir := t.TempDir()
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("campaign.Open: %v", err)
	}
	child := exec.Command("sleep", "60")
	if err := child.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := child.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("child is not alive after Close, so it proves nothing: %v", err)
	}
	if c2, err := qaHoldOpenPromptly(t, dir); err != nil || c2 == nil {
		t.Fatalf("campaign.Open while the holder's child lives: c=%v err=%v", c2, err)
	}
}

// SPEC-019 How it works: "Any other failure to open the directory or lock it is
// returned as campaign: take the writer hold on <dir>: <err>, and that Open
// fails too."
func TestQAHoldOpenThatCannotOpenTheDirectoryNamesTheHold(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a mode-0 directory")
	}
	dir := filepath.Join(t.TempDir(), "qa-sealed")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o750) })

	c, err := qaHoldOpenPromptly(t, dir)
	if c != nil || err == nil {
		t.Fatalf("campaign.Open on an unreadable directory: c=%v err=%v, want a failure", c, err)
	}
	prefix := "campaign: take the writer hold on " + dir + ": "
	if !strings.HasPrefix(err.Error(), prefix) || len(err.Error()) == len(prefix) {
		t.Fatalf("failure text %q, want %q followed by the cause", err.Error(), prefix)
	}
	if errors.Is(err, campaign.ErrHeld) {
		t.Fatalf("an unreadable directory answered ErrHeld: %v", err)
	}
}
