package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// Keep these as multiples of the bound: a second-literal here would show in
// the ticket's grep for literals at a subprocess wait.
var (
	qaCeiling  = 40 * neverAnswersBound
	qaPromptly = 10 * neverAnswersBound
)

func qaBounded(t *testing.T, what string, call func() error) (time.Duration, error) {
	t.Helper()
	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- call() }()
	select {
	case err := <-done:
		return time.Since(start), err
	case <-time.After(qaCeiling):
		t.Fatalf("%s did not return within %s", what, qaCeiling)
		return 0, nil
	}
}

func qaHoldingListener(t *testing.T, prefix string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []net.Conn
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			if prefix != "" {
				_, _ = c.Write([]byte(prefix))
			}
			mu.Lock()
			held = append(held, c)
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range held {
			_ = c.Close()
		}
	})
	return "http://" + ln.Addr().String()
}

func qaHealthzServer(t *testing.T, status func(n int32) int) (string, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status(hits.Add(1)))
	}))
	t.Cleanup(srv.Close)
	return srv.URL, &hits
}

func qaScript(t *testing.T, setup, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	path := filepath.Join(dir, "fake-vtt.sh")
	src := "#!/bin/sh\n" + setup + "\necho $$ > '" + pidFile + "'\n" + body + "\n"
	if err := os.WriteFile(path, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, pidFile
}

func qaReadPID(t *testing.T, pidFile string) int {
	t.Helper()
	b, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("the fixture never got to run: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("pid file: %v", err)
	}
	t.Cleanup(func() {
		if qaProcessState(pid) != "gone" {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	return pid
}

// Ask wait4, not kill(pid, 0): kill reports a zombie as present, so it cannot
// tell alive from unreaped.
func qaProcessState(pid int) string {
	var ws syscall.WaitStatus
	got, err := syscall.Wait4(pid, &ws, syscall.WNOHANG, nil)
	switch {
	case got == pid:
		return "zombie"
	case err == nil:
		return "alive"
	case errors.Is(err, syscall.ECHILD):
		return "gone"
	default:
		return err.Error()
	}
}

func qaAwaitEnded(pid int) string {
	deadline := time.Now().Add(qaPromptly)
	for {
		state := qaProcessState(pid)
		if state != "alive" || time.Now().After(deadline) {
			return state
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// VTT-079
func TestQAHealthzWaitEndsWhenTheListenerAcceptsAndNeverAnswers(t *testing.T) {
	base := qaHoldingListener(t, "")
	took, err := qaBounded(t, "waitForHealthz", func() error {
		return waitForHealthz(base, neverAnswersBound)
	})
	if err == nil {
		t.Fatal("waitForHealthz returned nil against a listener that never answers")
	}
	if took >= qaPromptly {
		t.Fatalf("waitForHealthz took %s against a %s bound", took, neverAnswersBound)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-079
func TestQAHealthzWaitEndsWhenTheListenerSendsHalfAHeaderAndStalls(t *testing.T) {
	base := qaHoldingListener(t, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n")
	took, err := qaBounded(t, "waitForHealthz", func() error {
		return waitForHealthz(base, neverAnswersBound)
	})
	if err == nil {
		t.Fatal("waitForHealthz returned nil against a stalled half-header")
	}
	if took >= qaPromptly {
		t.Fatalf("waitForHealthz took %s against a %s bound", took, neverAnswersBound)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-079
func TestQAHealthzWaitEndsWhenNothingListens(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + ln.Addr().String()
	_ = ln.Close()
	took, err := qaBounded(t, "waitForHealthz", func() error {
		return waitForHealthz(base, neverAnswersBound)
	})
	if err == nil {
		t.Fatal("waitForHealthz returned nil against a closed port")
	}
	if took < neverAnswersBound {
		t.Fatalf("waitForHealthz gave up after %s, before its %s bound", took, neverAnswersBound)
	}
	if took >= qaPromptly {
		t.Fatalf("waitForHealthz took %s against a %s bound", took, neverAnswersBound)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-079
func TestQAHealthzWaitReportsTheStatusWhenTheServerNeverBecomesReady(t *testing.T) {
	base, hits := qaHealthzServer(t, func(int32) int { return http.StatusServiceUnavailable })
	took, err := qaBounded(t, "waitForHealthz", func() error {
		return waitForHealthz(base, neverAnswersBound)
	})
	if err == nil {
		t.Fatal("waitForHealthz returned nil against a server that only answers 503")
	}
	if took >= qaPromptly {
		t.Fatalf("waitForHealthz took %s against a %s bound", took, neverAnswersBound)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("the error does not name the status it saw: %q", err)
	}
	if hits.Load() < 2 {
		t.Errorf("waitForHealthz polled %d time(s); it is meant to keep polling", hits.Load())
	}
	t.Logf("reported %q after %s and %d polls", err, took, hits.Load())
}

// VTT-079
func TestQAHealthzWaitReturnsNilOnceTheServerAnswers200(t *testing.T) {
	base, _ := qaHealthzServer(t, func(int32) int { return http.StatusOK })
	took, err := qaBounded(t, "waitForHealthz", func() error {
		return waitForHealthz(base, subprocessAnswers)
	})
	if err != nil {
		t.Fatalf("waitForHealthz against a ready server: %v", err)
	}
	if took >= qaCeiling {
		t.Fatalf("waitForHealthz took %s to notice a ready server", took)
	}
}

// VTT-079
func TestQAHealthzWaitKeepsPollingThroughEarly503s(t *testing.T) {
	base, hits := qaHealthzServer(t, func(n int32) int {
		if n <= 5 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	took, err := qaBounded(t, "waitForHealthz", func() error {
		return waitForHealthz(base, subprocessAnswers)
	})
	if err != nil {
		t.Fatalf("waitForHealthz against a server that becomes ready: %v", err)
	}
	if hits.Load() < 6 {
		t.Fatalf("waitForHealthz returned nil after %d polls, before the server answered 200", hits.Load())
	}
	if took >= qaCeiling {
		t.Fatalf("waitForHealthz took %s to notice a server that became ready", took)
	}
}

func qaConnect(t *testing.T, script, pidFile string, bound time.Duration) (time.Duration, func(), error) {
	t.Helper()
	p, err := startMCPSubprocess(script, "ws://127.0.0.1:1/ws", "qa-token")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(subprocessAnswers)
	for {
		if b, err := os.ReadFile(pidFile); err == nil && strings.HasSuffix(string(b), "\n") {
			break
		}
		if time.Now().After(deadline) {
			p.kill()
			t.Fatalf("the fixture did not write its pid within %s", subprocessAnswers)
		}
		time.Sleep(10 * time.Millisecond)
	}
	var closer func()
	took, err := qaBounded(t, "connectMCPSubprocess", func() error {
		session, c, err := p.connect(t, bound)
		closer = c
		if err != nil && session != nil {
			t.Errorf("a failed connect handed back a session: %v", session)
		}
		return err
	})
	return took, closer, err
}

// VTT-079
func TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessNeverWrites(t *testing.T) {
	script, pidFile := qaScript(t, "", "while read -r line; do :; done")
	took, _, err := qaConnect(t, script, pidFile, neverAnswersBound)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if took >= qaPromptly {
		t.Fatalf("connectMCPSubprocess took %s against a %s bound", took, neverAnswersBound)
	}
	pid := qaReadPID(t, pidFile)
	if state := qaProcessState(pid); state != "gone" {
		t.Fatalf("the subprocess is %s after the failed connect returned", state)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-079
func TestQAMCPConnectFailsWithADeadlineWhenTheSubprocessIgnoresStdio(t *testing.T) {
	script, pidFile := qaScript(t, "", "exec sleep 1000")
	took, _, err := qaConnect(t, script, pidFile, neverAnswersBound)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if took >= qaPromptly {
		t.Fatalf("connectMCPSubprocess took %s against a %s bound", took, neverAnswersBound)
	}
	pid := qaReadPID(t, pidFile)
	if state := qaProcessState(pid); state != "gone" {
		t.Fatalf("the subprocess is %s after the failed connect returned", state)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-079
func TestQAMCPConnectEndsASubprocessThatIgnoresSIGTERM(t *testing.T) {
	script, pidFile := qaScript(t, "trap '' TERM", "exec sleep 1000")
	took, _, err := qaConnect(t, script, pidFile, neverAnswersBound)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if took >= qaPromptly {
		t.Fatalf("connectMCPSubprocess took %s against a %s bound", took, neverAnswersBound)
	}
	pid := qaReadPID(t, pidFile)
	if state := qaProcessState(pid); state != "gone" {
		t.Fatalf("the subprocess is %s after the failed connect returned", state)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-079
func TestQAMCPConnectFailsPromptlyWhenTheSubprocessExitsWithoutAnswering(t *testing.T) {
	script, pidFile := qaScript(t, "", "exit 0")
	took, _, err := qaConnect(t, script, pidFile, neverAnswersBound)
	if err == nil {
		t.Fatal("connectMCPSubprocess returned nil for a subprocess that exited unanswered")
	}
	if took >= qaPromptly {
		t.Fatalf("connectMCPSubprocess took %s against a %s bound", took, neverAnswersBound)
	}
	pid := qaReadPID(t, pidFile)
	if state := qaProcessState(pid); state != "gone" {
		t.Fatalf("the subprocess is %s after the failed connect returned", state)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-253
func TestQAMCPConnectReachesItsAssertionsWhenTheFixtureStartsLate(t *testing.T) {
	setup := fmt.Sprintf("sleep %g", (2 * neverAnswersBound).Seconds())
	script, pidFile := qaScript(t, setup, "while read -r line; do :; done")
	took, _, err := qaConnect(t, script, pidFile, neverAnswersBound)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if took >= qaPromptly {
		t.Fatalf("connectMCPSubprocess took %s against a %s bound", took, neverAnswersBound)
	}
	pid := qaReadPID(t, pidFile)
	if state := qaProcessState(pid); state != "gone" {
		t.Fatalf("the subprocess is %s after the failed connect returned", state)
	}
	t.Logf("reported %q after %s", err, took)
}

// VTT-079
func TestQAConnectMCPSubprocessFailsWithinItsOwnBound(t *testing.T) {
	script := neverAnswersScript(t)
	took, err := qaBounded(t, "connectMCPSubprocess", func() error {
		session, cleanup, err := connectMCPSubprocess(t, script, "ws://127.0.0.1:1/ws", "qa-token", neverAnswersBound)
		if err == nil {
			_ = session.Close()
			cleanup()
		}
		return err
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if took >= qaPromptly {
		t.Fatalf("connectMCPSubprocess took %s against a %s bound", took, neverAnswersBound)
	}
}

// VTT-079
func TestQAMCPConnectSucceedsAgainstASubprocessThatAnswers(t *testing.T) {
	// Keep the canned reply's id 1 and its protocol version in step with the
	// go-sdk: Connect numbers its requests from 1 and refuses a version off its list.
	script, pidFile := qaScript(t, "", strings.Join([]string{
		"read -r line",
		`printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-11-25","capabilities":{},"serverInfo":{"name":"qa-fake","version":"0"}}}'`,
		"while read -r line; do :; done",
	}, "\n"))
	var name string
	took, err := qaBounded(t, "connectMCPSubprocess", func() error {
		session, closer, err := connectMCPSubprocess(t, script, "ws://127.0.0.1:1/ws", "qa-token", subprocessAnswers)
		if err != nil {
			return err
		}
		name = session.InitializeResult().ServerInfo.Name
		closer()
		return nil
	})
	if err != nil {
		t.Fatalf("connectMCPSubprocess against a subprocess that answers: %v", err)
	}
	if name != "qa-fake" {
		t.Fatalf("connected to %q, not the fixture", name)
	}
	if took >= qaCeiling {
		t.Fatalf("connect and close took %s", took)
	}
	pid := qaReadPID(t, pidFile)
	if state := qaAwaitEnded(pid); state != "gone" {
		t.Fatalf("the subprocess is %s after the closer returned", state)
	}
}

// VTT-079
func TestQAWaitWithTimeoutEndsWhenTheSubprocessNeverExits(t *testing.T) {
	cmd := exec.Command("sleep", "1000")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		if qaProcessState(pid) != "gone" {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	took, err := qaBounded(t, "waitWithTimeout", func() error {
		return waitWithTimeout(cmd, neverAnswersBound)
	})
	if err == nil {
		t.Fatal("waitWithTimeout returned nil for a subprocess that never exits")
	}
	if took >= qaPromptly {
		t.Fatalf("waitWithTimeout took %s against a %s bound", took, neverAnswersBound)
	}
	state := qaAwaitEnded(pid)
	if state == "alive" {
		t.Fatalf("the subprocess is still alive after waitWithTimeout returned")
	}
	t.Logf("reported %q after %s; the subprocess is %s", err, took, state)
}

// VTT-079
func TestQAWaitWithTimeoutReturnsNilWhenTheSubprocessExitsCleanly(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	took, err := qaBounded(t, "waitWithTimeout", func() error {
		return waitWithTimeout(cmd, subprocessExits)
	})
	if err != nil {
		t.Fatalf("waitWithTimeout on a clean exit: %v", err)
	}
	if took >= qaCeiling {
		t.Fatalf("waitWithTimeout took %s to notice a clean exit", took)
	}
}

// VTT-079
func TestQAWaitWithTimeoutReturnsPromptlyWhenTheSubprocessExitsNonZero(t *testing.T) {
	cmd := exec.Command("false")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	took, err := qaBounded(t, "waitWithTimeout", func() error {
		return waitWithTimeout(cmd, subprocessExits)
	})
	if took >= qaCeiling {
		t.Fatalf("waitWithTimeout took %s to notice a non-zero exit", took)
	}
	t.Logf("non-zero exit reported as %v after %s", err, took)
}
