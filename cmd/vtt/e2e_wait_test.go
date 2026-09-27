package main

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// subprocessAnswers bounds a subprocess these tests start giving its first
// answer: healthz 200, the first line of `events tail`, the MCP initialize reply.
const subprocessAnswers = 40 * time.Second

// subprocessExits bounds a subprocess ending after SIGINT, SIGTERM or stdin EOF.
const subprocessExits = 30 * time.Second

// neverAnswersBound is what the negative tests hand a helper: a helper proven
// to honour it is proven to honour the two above (VTT-079).
const neverAnswersBound = 500 * time.Millisecond

// neverAnswersScript writes a program that reads stdin and never writes.
func neverAnswersScript(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "never-answers.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nwhile read line; do :; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// VTT-079
func TestWaitForHealthzGivesUpOnAListenerThatNeverAnswers(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(io.Discard, conn) }()
		}
	}()
	done := make(chan error, 1)
	go func() { done <- waitForHealthz("http://"+ln.Addr().String(), neverAnswersBound) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("waitForHealthz returned nil against a listener that never answers")
		}
	case <-time.After(subprocessAnswers):
		t.Fatalf("waitForHealthz(%s) did not return within %s against a listener that never answers", neverAnswersBound, subprocessAnswers)
	}
}

// VTT-079
func TestConnectMCPSubprocessGivesUpOnASubprocessThatNeverWrites(t *testing.T) {
	script := neverAnswersScript(t)
	done := make(chan error, 1)
	go func() {
		cs, cleanup, err := connectMCPSubprocess(t, script, "ws://127.0.0.1:1", "token", neverAnswersBound)
		if err == nil {
			cs.Close()
			cleanup()
		}
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("connectMCPSubprocess against a subprocess that never writes: want a deadline error, got %v", err)
		}
	case <-time.After(subprocessAnswers):
		t.Fatalf("connectMCPSubprocess(%s) did not return within %s against a subprocess that never writes", neverAnswersBound, subprocessAnswers)
	}
}

// VTT-079
func TestWaitWithTimeoutKillsASubprocessThatNeverExits(t *testing.T) {
	cmd := exec.Command(neverAnswersScript(t))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- waitWithTimeout(cmd, neverAnswersBound) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("waitWithTimeout returned nil for a subprocess that never exits")
		}
		if cmd.ProcessState == nil || cmd.ProcessState.Success() {
			t.Fatalf("waitWithTimeout returned %v but the subprocess is not ended: %v", err, cmd.ProcessState)
		}
	case <-time.After(subprocessAnswers):
		t.Fatalf("waitWithTimeout(%s) did not return within %s for a subprocess that never exits", neverAnswersBound, subprocessAnswers)
	}
}
