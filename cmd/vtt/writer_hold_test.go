package main

import (
	"bytes"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
)

// VTT-270
func TestCommandsThatAppendNothingWorkWhileTheCampaignIsServed(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign")
	_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", "", "")
	if err != nil {
		t.Fatalf("serve: %v", err)
	}
	t.Cleanup(func() { _ = closeFn() })

	out, err := runCLI(t, "invite", "--campaign", campaignPath, "--name", "Lera", "--role", "player")
	if err != nil {
		t.Fatalf("invite beside serve: %v (output %s)", err, out)
	}
	id := extractField(t, out, "participant id: ")
	if out, err := runCLI(t, "revoke", "--campaign", campaignPath, "--id", id); err != nil {
		t.Fatalf("revoke beside serve: %v (output %s)", err, out)
	}
	for _, sub := range []string{"show", "open", "close", "rotate"} {
		if out, err := runCLI(t, "join-link", sub, "--campaign", campaignPath); err != nil {
			t.Fatalf("join-link %s beside serve: %v (output %s)", sub, err, out)
		}
	}
	pic := srcPNG(t, t.TempDir(), "masonry-1.png", 64, 64)
	if out, err := runCLI(t, "art", "install", "--campaign", campaignPath, pic); err != nil {
		t.Fatalf("art install beside serve: %v (output %s)", err, out)
	}
}

// VTT-267 VTT-269
func TestASecondServeOnAHeldDirectoryExitsWithTheRefusal(t *testing.T) {
	binPath := buildVTTBinary(t)
	campaignPath := filepath.Join(t.TempDir(), "campaign")

	first := exec.Command(binPath, "serve", "--campaign", campaignPath, "--addr", mustFreeAddr(t))
	firstAddr := first.Args[len(first.Args)-1]
	if err := first.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = first.Process.Kill()
		_ = first.Wait()
	})
	if err := waitForHealthz("http://"+firstAddr, subprocessAnswers); err != nil {
		t.Fatalf("first serve never answered healthz: %v", err)
	}

	secondAddr := mustFreeAddr(t)
	var out bytes.Buffer
	second := exec.Command(binPath, "serve", "--campaign", campaignPath, "--addr", secondAddr)
	second.Stdout, second.Stderr = &out, &out
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	if err := waitWithTimeout(second, subprocessExits); err == nil {
		t.Fatalf("second serve exited 0 on a held directory (output %s)", out.String())
	} else if second.ProcessState == nil || second.ProcessState.ExitCode() <= 0 {
		t.Fatalf("second serve did not exit on its own: %v (output %s)", err, out.String())
	}
	text := out.String()
	if !strings.Contains(text, campaign.ErrHeld.Error()) || !strings.Contains(text, campaignPath) {
		t.Fatalf("second serve's output lacks the refusal naming %s: %s", campaignPath, text)
	}
	if strings.Contains(text, "listening on") {
		t.Fatalf("second serve said it was listening: %s", text)
	}
	client := http.Client{Timeout: time.Second}
	if resp, err := client.Get("http://" + secondAddr + "/healthz"); err == nil {
		resp.Body.Close()
		t.Fatalf("something answered healthz at the refused serve's address: %s", resp.Status)
	}

	if err := first.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = first.Wait()

	thirdAddr := mustFreeAddr(t)
	third := exec.Command(binPath, "serve", "--campaign", campaignPath, "--addr", thirdAddr)
	if err := third.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = third.Process.Kill()
		_ = third.Wait()
	})
	if err := waitForHealthz("http://"+thirdAddr, subprocessAnswers); err != nil {
		t.Fatalf("serve after the holder was killed never answered healthz: %v", err)
	}
}
