package campaign_test

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/store"
)

const holderDirEnv = "VTT_CAMPAIGN_HOLDER_DIR"

func TestMain(m *testing.M) {
	if dir := os.Getenv(holderDirEnv); dir != "" {
		parent := os.Getppid()
		c, err := campaign.Open(dir)
		if err != nil {
			fmt.Println("REFUSED", err)
			os.Exit(1)
		}
		defer func() { _ = c.Close() }()
		fmt.Println("HELD")
		for os.Getppid() == parent {
			time.Sleep(100 * time.Millisecond)
		}
		return
	}
	os.Exit(m.Run())
}

func startHolder(t *testing.T, dir string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), holderDirEnv+"="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	line := make(chan string, 1)
	go func() {
		s, _ := bufio.NewReader(out).ReadString('\n')
		line <- strings.TrimSpace(s)
	}()
	select {
	case got := <-line:
		if got != "HELD" {
			t.Fatalf("holder printed %q, want HELD", got)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("holder did not report HELD within 30s")
	}
	return cmd
}

func openWithin(t *testing.T, dir string) (*campaign.Campaign, error) {
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
	case o := <-done:
		return o.c, o.err
	case <-time.After(10 * time.Second):
		t.Fatal("an Open of a held directory waited instead of refusing")
		return nil, nil
	}
}

func requireHeldRefusal(t *testing.T, dir string, err error) {
	t.Helper()
	if !errors.Is(err, campaign.ErrHeld) {
		t.Fatalf("Open of a held directory: err = %v, want campaign.ErrHeld", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Fatalf("refusal %q does not name the directory %s", err, dir)
	}
}

// VTT-267
func TestASecondOpenInTheSameProcessIsRefused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	first, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()

	second, err := openWithin(t, dir)
	if err == nil {
		_ = second.Close()
	}
	requireHeldRefusal(t, dir, err)
}

// VTT-267
func TestAnOpenWhileAnotherProcessHoldsIsRefused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	startHolder(t, dir)

	c, err := openWithin(t, dir)
	if err == nil {
		_ = c.Close()
	}
	requireHeldRefusal(t, dir, err)
}

// VTT-268
func TestClosingTheHolderLetsTheNextOpenSucceed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	first, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("Open after the holder closed: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("a second Close returned %v, want nil", err)
	}
}

// VTT-269
func TestTheHoldEndsWhenItsProcessIsKilled(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	holder := startHolder(t, dir)

	if c, err := openWithin(t, dir); err == nil {
		_ = c.Close()
		t.Fatal("Open succeeded while another process held the directory")
	}
	if err := holder.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = holder.Wait()

	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("Open after the holder was killed: %v", err)
	}
	_ = c.Close()
}

// VTT-268
func TestAnOpenThatFailsReleasesTheHold(t *testing.T) {
	cases := map[string]func(t *testing.T, dir string){
		"a log that does not fold": func(t *testing.T, dir string) {
			t.Helper()
			if err := os.MkdirAll(dir, 0o750); err != nil {
				t.Fatal(err)
			}
			s, err := store.Open(campaign.LogPath(dir))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Close() }()
			for _, e := range []*vttv1.Envelope{
				cenv(nextID(), &vttv1.SessionStarted{Name: "s"}),
				cenv(nextID(), &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 2, GridHeight: 2}),
				cenv(nextID(), &vttv1.SceneCreated{SceneId: "s", Name: "S", GridWidth: 2, GridHeight: 2}),
			} {
				if _, err := s.Append(e); err != nil {
					t.Fatal(err)
				}
			}
		},
		"a log path that is a directory": func(t *testing.T, dir string) {
			t.Helper()
			if err := os.MkdirAll(campaign.LogPath(dir), 0o750); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setUp := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "campaign")
			setUp(t, dir)
			for attempt := 1; attempt <= 2; attempt++ {
				c, err := campaign.Open(dir)
				if err == nil {
					_ = c.Close()
					t.Fatalf("attempt %d: Open succeeded, want it to fail", attempt)
				}
				if errors.Is(err, campaign.ErrHeld) {
					t.Fatalf("attempt %d: %v: a failed Open kept the hold", attempt, err)
				}
			}
		})
	}
}

// VTT-269
func TestAChildProcessDoesNotInheritTheHold(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "campaign")
	c, err := campaign.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := campaign.Open(dir)
	if err != nil {
		t.Fatalf("Open after Close, a child still running: %v", err)
	}
	_ = again.Close()
}
