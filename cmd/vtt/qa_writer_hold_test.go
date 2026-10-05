package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
	_ "modernc.org/sqlite"
)

const (
	qaHoldCommandBound = 30 * time.Second
	qaHoldPromptly     = time.Second
)

type qaHoldBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *qaHoldBuf) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *qaHoldBuf) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

func qaHoldBuild(t *testing.T) string {
	t.Helper()
	return buildVTTBinary(t)
}

func qaHoldFreeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func qaHoldRun(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), qaHoldCommandBound)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("vtt %v did not finish within %v:\n%s", args, qaHoldCommandBound, out)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatalf("vtt %v: %v", args, err)
	}
	return string(out), 0
}

func qaHoldMustRun(t *testing.T, bin string, args ...string) string {
	t.Helper()
	out, code := qaHoldRun(t, bin, args...)
	if code != 0 {
		t.Fatalf("vtt %v exited %d:\n%s", args, code, out)
	}
	return out
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

func qaHoldIdentity(t *testing.T, dir string) *identity.DB {
	t.Helper()
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func qaHoldHasParticipant(t *testing.T, db *identity.DB, name string, role identity.Role) (string, bool) {
	t.Helper()
	ps, err := db.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, p := range ps {
		if p.Name == name && p.Role == role {
			return p.ID, true
		}
	}
	return "", false
}

type qaHoldServer struct {
	cmd    *exec.Cmd
	addr   string
	out    *qaHoldBuf
	exited chan struct{}
}

func qaHoldStartServe(t *testing.T, bin, dir string) *qaHoldServer {
	t.Helper()
	s := &qaHoldServer{addr: qaHoldFreeAddr(t), out: &qaHoldBuf{}, exited: make(chan struct{})}
	s.cmd = exec.Command(bin, "serve", "--campaign", dir, "--addr", s.addr)
	s.cmd.Stdout = s.out
	s.cmd.Stderr = s.out
	if err := s.cmd.Start(); err != nil {
		t.Fatalf("start serve: %v", err)
	}
	go func() {
		_ = s.cmd.Wait()
		close(s.exited)
	}()
	t.Cleanup(func() {
		_ = s.cmd.Process.Signal(os.Interrupt)
		select {
		case <-s.exited:
		case <-time.After(10 * time.Second):
			_ = s.cmd.Process.Kill()
			<-s.exited
		}
	})
	return s
}

func qaHoldHealthy(addr string) bool {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func qaHoldServeUntilHealthy(t *testing.T, bin, dir string) *qaHoldServer {
	t.Helper()
	s := qaHoldStartServe(t, bin, dir)
	deadline := time.Now().Add(qaHoldCommandBound)
	for time.Now().Before(deadline) {
		select {
		case <-s.exited:
			t.Fatalf("vtt serve exited before it served:\n%s", s.out.String())
		default:
		}
		if qaHoldHealthy(s.addr) {
			return s
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("vtt serve did not answer /healthz within %v:\n%s", qaHoldCommandBound, s.out.String())
	return nil
}

func qaHoldOpenPromptly(t *testing.T, dir string) (*campaign.Campaign, error) {
	t.Helper()
	type opened struct {
		c   *campaign.Campaign
		err error
	}
	done := make(chan opened, 1)
	go func() {
		c, err := campaign.Open(dir)
		done <- opened{c, err}
	}()
	select {
	case r := <-done:
		if r.c != nil {
			t.Cleanup(func() { _ = r.c.Close() })
		}
		return r.c, r.err
	case <-time.After(qaHoldPromptly):
		t.Fatalf("campaign.Open(%s) did not return within %v", dir, qaHoldPromptly)
		return nil, nil
	}
}

func qaHoldHeldText(dir string) string {
	return "campaign: another writer holds the campaign directory " + dir +
		"; a campaign has one writer at a time, so stop the vtt serve or close the Campaign that has it open"
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

func qaHoldDirFlockFree(t *testing.T, dir string) error {
	t.Helper()
	fd, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open %s for flock: %v", dir, err)
	}
	defer func() { _ = syscall.Close(fd) }()
	return syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
}

func qaHoldAdminCommands(dir, revokeID string) [][]string {
	return [][]string{
		{"invite", "--campaign", dir, "--name", "QA Nell", "--role", "spectator"},
		{"revoke", "--campaign", dir, "--id", revokeID},
		{"join-link", "open", "--campaign", dir},
		{"join-link", "show", "--campaign", dir},
		{"join-link", "rotate", "--campaign", dir},
		{"join-link", "close", "--campaign", dir},
	}
}

// VTT-267
func TestQAHoldServeIsRefusedADirectoryThisProcessHolds(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)

	out, code := qaHoldRun(t, bin, "serve", "--campaign", dir, "--addr", qaHoldFreeAddr(t))
	if code == 0 {
		t.Fatalf("vtt serve on a held directory exited 0:\n%s", out)
	}
	if !strings.Contains(out, campaign.ErrHeld.Error()+" "+dir) {
		t.Fatalf("vtt serve refusal does not carry ErrHeld naming %s:\n%s", dir, out)
	}
}

// SPEC-019 How it works: "vtt serve prints it after vtt serve: open campaign: ,
// exits 1 and listens on nothing."
func TestQAHoldServeRefusalIsPrintedAfterItsPrefixExitsOneAndListensOnNothing(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)

	s := qaHoldStartServe(t, bin, dir)
	answered := false
	deadline := time.After(qaHoldCommandBound)
poll:
	for {
		select {
		case <-s.exited:
			break poll
		case <-deadline:
			t.Fatalf("refused vtt serve still running after %v:\n%s", qaHoldCommandBound, s.out.String())
		default:
		}
		if conn, err := net.DialTimeout("tcp", s.addr, 20*time.Millisecond); err == nil {
			answered = true
			_ = conn.Close()
		}
		time.Sleep(2 * time.Millisecond)
	}
	if answered {
		t.Fatalf("refused vtt serve accepted a connection on %s", s.addr)
	}
	if code := s.cmd.ProcessState.ExitCode(); code != 1 {
		t.Fatalf("refused vtt serve exited %d, want 1:\n%s", code, s.out.String())
	}
	want := "vtt serve: open campaign: " + qaHoldHeldText(dir)
	if !strings.Contains(s.out.String(), want) {
		t.Fatalf("refused vtt serve output\n%s\nlacks\n%s", s.out.String(), want)
	}
}

// VTT-267
func TestQAHoldAServingDirectoryRefusesThisProcessAtOnce(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldServeUntilHealthy(t, bin, dir)

	c, err := qaHoldOpenPromptly(t, dir)
	if c != nil {
		t.Fatalf("campaign.Open succeeded on a directory vtt serve holds")
	}
	if !errors.Is(err, campaign.ErrHeld) || !strings.Contains(err.Error(), dir) {
		t.Fatalf("campaign.Open beside vtt serve: %v, want ErrHeld naming %s", err, dir)
	}
}

// VTT-269
func TestQAHoldAKilledServeLeavesNoHold(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	s := qaHoldServeUntilHealthy(t, bin, dir)

	if err := s.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("kill serve: %v", err)
	}
	<-s.exited
	if c, err := qaHoldOpenPromptly(t, dir); err != nil || c == nil {
		t.Fatalf("campaign.Open after vtt serve was killed: c=%v err=%v", c, err)
	}
}

// VTT-269
func TestQAHoldAnInterruptedServeLeavesNoHold(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	s := qaHoldServeUntilHealthy(t, bin, dir)

	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupt serve: %v", err)
	}
	select {
	case <-s.exited:
	case <-time.After(qaHoldCommandBound):
		t.Fatalf("vtt serve did not stop on interrupt within %v", qaHoldCommandBound)
	}
	t.Logf("vtt serve ended: %v", s.cmd.ProcessState)
	if c, err := qaHoldOpenPromptly(t, dir); err != nil || c == nil {
		t.Fatalf("campaign.Open after vtt serve stopped: c=%v err=%v", c, err)
	}
}

// VTT-270
func TestQAHoldInviteWorksAgainstAHeldDirectory(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)

	qaHoldMustRun(t, bin, "invite", "--campaign", dir, "--name", "QA Ann", "--role", "player")
	if _, ok := qaHoldHasParticipant(t, qaHoldIdentity(t, dir), "QA Ann", identity.RolePlayer); !ok {
		t.Fatalf("vtt invite against a held directory minted nobody")
	}
}

// VTT-270
func TestQAHoldRevokeWorksAgainstAHeldDirectory(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)
	db := qaHoldIdentity(t, dir)
	_, id, err := db.CreateInvite("QA Bo", identity.RolePlayer)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	qaHoldMustRun(t, bin, "revoke", "--campaign", dir, "--id", id)
	if _, err := db.Lookup(id); !errors.Is(err, identity.ErrInvalidToken) {
		t.Fatalf("participant revoked against a held directory still resolves: %v", err)
	}
}

// VTT-270
func TestQAHoldJoinLinkWorksAgainstAHeldDirectory(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)
	db := qaHoldIdentity(t, dir)
	before, err := db.JoinSecret()
	if err != nil {
		t.Fatalf("JoinSecret: %v", err)
	}

	qaHoldMustRun(t, bin, "join-link", "open", "--campaign", dir)
	if !db.JoinOpen() {
		t.Fatalf("join-link open against a held directory left the door shut")
	}
	qaHoldMustRun(t, bin, "join-link", "show", "--campaign", dir)
	qaHoldMustRun(t, bin, "join-link", "rotate", "--campaign", dir)
	after, err := db.JoinSecret()
	if err != nil {
		t.Fatalf("JoinSecret: %v", err)
	}
	if after == before {
		t.Fatalf("join-link rotate against a held directory kept the secret")
	}
	qaHoldMustRun(t, bin, "join-link", "close", "--campaign", dir)
	if db.JoinOpen() {
		t.Fatalf("join-link close against a held directory left the door open")
	}
}

// VTT-270
func TestQAHoldArtInstallWorksAgainstAHeldDirectory(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldMustOpen(t, dir)
	src := filepath.Join(t.TempDir(), "qa-hold.png")
	content := []byte("qa art bytes")
	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatalf("write art: %v", err)
	}

	qaHoldMustRun(t, bin, "art", "install", "--campaign", dir, src)
	got, err := os.ReadFile(filepath.Join(dir, "art", "qa-hold.png"))
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("art install against a held directory: %q, %v", got, err)
	}
}

// VTT-270
func TestQAHoldAdminCommandsWorkBesideARunningServe(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	qaHoldServeUntilHealthy(t, bin, dir)
	_, id, err := qaHoldIdentity(t, dir).CreateInvite("QA Cy", identity.RolePlayer)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	src := filepath.Join(t.TempDir(), "qa-beside.png")
	if err := os.WriteFile(src, []byte("qa"), 0o600); err != nil {
		t.Fatalf("write art: %v", err)
	}

	for _, args := range append(qaHoldAdminCommands(dir, id), []string{"art", "install", "--campaign", dir, src}) {
		qaHoldMustRun(t, bin, args...)
	}
	if _, ok := qaHoldHasParticipant(t, qaHoldIdentity(t, dir), "QA Nell", identity.RoleSpectator); !ok {
		t.Fatalf("vtt invite beside vtt serve minted nobody")
	}
}

// VTT-270
func TestQAHoldAdminCommandsHoldNothingWhileTheyRun(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	_, id, err := qaHoldIdentity(t, dir).CreateInvite("QA Di", identity.RolePlayer)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	for _, args := range qaHoldAdminCommands(dir, id) {
		conn, err := qaHoldLogDB(t, dir).Conn(context.Background())
		if err != nil {
			t.Fatalf("conn: %v", err)
		}
		if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
			t.Fatalf("begin exclusive: %v", err)
		}
		out := &qaHoldBuf{}
		cmd := exec.Command(bin, args...)
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Start(); err != nil {
			t.Fatalf("start vtt %v: %v", args, err)
		}
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()

		select {
		case err := <-exited:
			t.Fatalf("vtt %v finished while log.db was locked (%v), so it was never observed mid-run:\n%s", args, err, out.String())
		case <-time.After(time.Second):
		}
		flockErr := qaHoldDirFlockFree(t, dir)
		_, _ = conn.ExecContext(context.Background(), "COMMIT")
		_ = conn.Close()
		if flockErr != nil {
			<-exited
			t.Fatalf("directory could not be flocked while vtt %v ran: %v", args, flockErr)
		}
		select {
		case err := <-exited:
			if err != nil {
				t.Fatalf("vtt %v: %v\n%s", args, err, out.String())
			}
		case <-time.After(qaHoldCommandBound):
			_ = cmd.Process.Kill()
			t.Fatalf("vtt %v did not finish within %v", args, qaHoldCommandBound)
		}
	}
}

// SPEC-019 How it works: "campaign.Open first calls EnsureDir, which refuses a
// path that is a plain file and creates a missing directory with mode 0o750."
func TestQAHoldInviteCreatesAMissingDirectoryWithMode0750(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := filepath.Join(t.TempDir(), "qa-new")

	qaHoldMustRun(t, bin, "invite", "--campaign", dir, "--name", "QA Ed", "--role", "player")
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("vtt invite did not create %s: %v", dir, err)
	}
	old := syscall.Umask(0)
	syscall.Umask(old)
	if want := os.FileMode(0o750) &^ os.FileMode(old); fi.Mode().Perm() != want {
		t.Fatalf("%s has mode %o, want %o", dir, fi.Mode().Perm(), want)
	}
}

// SPEC-019 Consequences: "A directory one of them creates holds identity's
// tables and no events table until a Campaign opens it"
func TestQAHoldADirectoryInviteCreatesHoldsNoEventsTable(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := filepath.Join(t.TempDir(), "qa-new")

	qaHoldMustRun(t, bin, "invite", "--campaign", dir, "--name", "QA Flo", "--role", "player")
	if qaHoldHasTable(t, dir, "events") {
		t.Fatalf("a directory vtt invite created holds an events table")
	}
	c, err := qaHoldOpenPromptly(t, dir)
	if err != nil || c == nil {
		t.Fatalf("campaign.Open on a directory vtt invite created: c=%v err=%v", c, err)
	}
	if !qaHoldHasTable(t, dir, "events") {
		t.Fatalf("campaign.Open left no events table, so its absence above proves nothing")
	}
}

func qaHoldHasTable(t *testing.T, dir, name string) bool {
	t.Helper()
	var n int
	err := qaHoldLogDB(t, dir).QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	return n == 1
}

// SPEC-019 Consequences: "invite, revoke and join-link neither read nor fold
// the log. ... they work on a campaign whose log does not fold."
func TestQAHoldAdminCommandsWorkOnALogOpenRefuses(t *testing.T) {
	bin := qaHoldBuild(t)
	dir := t.TempDir()
	c := qaHoldMustOpen(t, dir)
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := qaHoldLogDB(t, dir).Exec(`INSERT INTO events(event_id, session_id, occurred_at, payload) VALUES ('qa-corrupt', '', '2026-10-05T00:00:00Z', X'FFFFFFFF')`); err != nil {
		t.Fatalf("insert corrupt event: %v", err)
	}
	if c, err := qaHoldOpenPromptly(t, dir); c != nil || err == nil || errors.Is(err, campaign.ErrHeld) {
		t.Fatalf("precondition: campaign.Open must refuse this log: c=%v err=%v", c, err)
	}
	_, id, err := qaHoldIdentity(t, dir).CreateInvite("QA Gus", identity.RolePlayer)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	for _, args := range qaHoldAdminCommands(dir, id) {
		qaHoldMustRun(t, bin, args...)
	}
}

// SPEC-019 Who does not: "Identity's handle on log.db is its own (SPEC-009) and
// takes no hold."
func TestQAHoldIdentityOnAHeldDirectoryWorksAndTakesNoHold(t *testing.T) {
	dir := t.TempDir()
	c := qaHoldMustOpen(t, dir)

	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open beside a holder: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, _, err := db.CreateInvite("QA Ida", identity.RolePlayer); err != nil {
		t.Fatalf("CreateInvite beside a holder: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if c2, err := qaHoldOpenPromptly(t, dir); err != nil || c2 == nil {
		t.Fatalf("campaign.Open while identity is open: c=%v err=%v", c2, err)
	}
	ps, err := db.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, p := range ps {
		if p.Name == "QA Ida" && p.Role == identity.RolePlayer {
			found = true
		}
	}
	if !found {
		t.Fatalf("participant minted beside a holder is missing: %+v", ps)
	}
}
