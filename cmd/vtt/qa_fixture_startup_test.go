package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const qaAnswerAtOnce = `read -r line
id=${line#*'"id":'}
id=${id%%,*}
ver=${line#*'"protocolVersion":"'}
ver=${ver%%'"'*}
printf '{"jsonrpc":"2.0","id":%s,"result":{"protocolVersion":"%s","capabilities":{},"serverInfo":{"name":"fixture","version":"0"}}}\n' "$id" "$ver"
while read -r _; do :; done`

type qaStartupBody struct {
	name     string
	trap     bool
	body     string
	deadline bool
}

var qaStartupBodies = []qaStartupBody{
	{name: "never-writes", body: "while read -r _; do :; done", deadline: true},
	{name: "ignores-stdio", body: "exec sleep 30", deadline: true},
	{name: "ignores-sigterm", trap: true, body: "while :; do sleep 0.1; done", deadline: true},
	{name: "exits-at-once", body: "exit 0"},
}

func qaSleep(d time.Duration) string {
	return fmt.Sprintf("sleep %g", d.Seconds())
}

func qaStartupSetup(trap bool, startUp time.Duration, marker string) string {
	setup := qaSleep(startUp) + "\n: > '" + marker + "'"
	if trap {
		setup = "trap '' TERM\n" + setup
	}
	return setup
}

func qaAwaitPIDLine(tb testing.TB, pidFile string) int {
	tb.Helper()
	deadline := time.Now().Add(subprocessAnswers)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(pidFile)
		if err == nil && strings.HasSuffix(string(b), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				tb.Fatalf("pid line %q: %v", b, err)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	tb.Fatalf("no pid line in %s within %v", pidFile, subprocessAnswers)
	return 0
}

func qaRequireGone(tb testing.TB, pid int, rule string) {
	tb.Helper()
	if st := qaAwaitEnded(pid); st != "gone" {
		tb.Errorf("%s: subprocess is %q afterwards, want gone", rule, st)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		tb.Errorf("%s: pid %d still answers signal 0 (%v), want no such process", rule, pid, err)
	}
}

// VTT-253 VTT-079
func TestQAFixtureStartupDeadlineTestsReachEveryAssertion(t *testing.T) {
	t.Parallel()
	for _, b := range qaStartupBodies {
		for k := 0; k <= 3; k++ {
			t.Run(fmt.Sprintf("%s/start-%dx-bound", b.name, k), func(t *testing.T) {
				t.Parallel()
				startUp := time.Duration(k) * neverAnswersBound
				marker := filepath.Join(t.TempDir(), "setup-done")
				script, pidFile := qaScript(t, qaStartupSetup(b.trap, startUp, marker), b.body)
				began := time.Now()
				elapsed, cleanup, err := qaConnect(t, script, pidFile, neverAnswersBound)
				wall := time.Since(began)
				if cleanup != nil {
					t.Cleanup(cleanup)
				}
				if _, statErr := os.Stat(marker); statErr != nil {
					t.Errorf("VTT-253: setup had not finished when the handshake ran: %v", statErr)
				}
				if err == nil {
					t.Fatalf("VTT-079: a fixture that never answers connected")
				}
				if b.deadline && !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("VTT-079: want the deadline error, got %v", err)
				}
				if elapsed >= qaPromptly {
					t.Errorf("VTT-079: bounded handshake took %v, want under %v", elapsed, qaPromptly)
				}
				if b.deadline && elapsed < neverAnswersBound {
					t.Errorf("VTT-253: deadline fired after %v, before the whole bound %v", elapsed, neverAnswersBound)
				}
				if wall < startUp+elapsed {
					t.Errorf("VTT-253: bounded part %v includes the %v start-up (call took %v)", elapsed, startUp, wall)
				}
				qaRequireGone(t, qaReadPID(t, pidFile), "VTT-079")
			})
		}
	}
}

// VTT-253
func TestQAFixtureStartupASlowStartStillGetsTheWholeBoundToAnswer(t *testing.T) {
	t.Parallel()
	startUp := 3 * neverAnswersBound
	marker := filepath.Join(t.TempDir(), "setup-done")
	script, pidFile := qaScript(t, qaStartupSetup(false, startUp, marker), qaAnswerAtOnce)
	began := time.Now()
	elapsed, cleanup, err := qaConnect(t, script, pidFile, neverAnswersBound)
	wall := time.Since(began)
	if err != nil {
		t.Fatalf("VTT-253: an answer at once after a %v start-up was refused: %v", startUp, err)
	}
	if wall < startUp+elapsed {
		t.Errorf("VTT-253: bounded part %v includes the %v start-up (call took %v)", elapsed, startUp, wall)
	}
	cleanup()
	qaRequireGone(t, qaReadPID(t, pidFile), "VTT-079")
}

// VTT-079 VTT-253
func TestQAFixtureStartupALateAnswerStillMeetsTheDeadline(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "setup-done")
	body := qaSleep(3*neverAnswersBound) + "\n" + qaAnswerAtOnce
	script, pidFile := qaScript(t, qaStartupSetup(false, neverAnswersBound, marker), body)
	elapsed, cleanup, err := qaConnect(t, script, pidFile, neverAnswersBound)
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("VTT-079: an answer 3x the bound after start-up got %v, want the deadline", err)
	}
	if elapsed >= qaPromptly {
		t.Errorf("VTT-079: bounded handshake took %v, want under %v", elapsed, qaPromptly)
	}
	qaRequireGone(t, qaReadPID(t, pidFile), "VTT-079")
}

// VTT-253
func TestQAFixtureStartupTheTrapIsInstalledBeforeThePidLine(t *testing.T) {
	t.Parallel()
	script, pidFile := qaScript(t, qaSleep(neverAnswersBound)+"\ntrap '' TERM", "while :; do sleep 0.1; done")
	cmd := exec.Command(script)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	pid := qaAwaitPIDLine(t, pidFile)
	if pid != cmd.Process.Pid {
		t.Fatalf("VTT-253: pid line names %d, the fixture shell is %d", pid, cmd.Process.Pid)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	time.Sleep(neverAnswersBound)
	if st := qaProcessState(pid); st != "alive" {
		t.Errorf("VTT-253: shell is %q after SIGTERM, want alive behind its trap", st)
	}
}
