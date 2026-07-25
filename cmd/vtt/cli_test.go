package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// runCLI drives the cobra root command in-process (no exec of the built
// binary): a fresh root is built per call, args are set, output is
// captured, and Execute runs synchronously.
func runCLI(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	var buf bytes.Buffer
	root := newRootCmd()
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err = root.Execute()
	return buf.String(), err
}

func TestVersionPrintsVersionShapedOutput(t *testing.T) {
	out, err := runCLI(t, "version")
	if err != nil {
		t.Fatalf("version: unexpected error: %v", err)
	}
	if !strings.Contains(out, "vtt") || !strings.Contains(out, Version) {
		t.Fatalf("version output not version-shaped: %q", out)
	}
}

// TestInviteThenRevoke drives invite → identity.Verify (accepts) → revoke →
// identity.Verify (rejects), the brief's Step 1 round-trip.
func TestInviteThenRevoke(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	out, err := runCLI(t, "invite",
		"--campaign", campaignPath,
		"--name", "Lera",
		"--role", "player",
		"--controls", "act-lera",
	)
	if err != nil {
		t.Fatalf("invite: unexpected error: %v", err)
	}
	if !strings.Contains(out, "shown once") {
		t.Fatalf("invite output missing shown-once warning: %q", out)
	}

	id := extractField(t, out, "participant id: ")
	token := extractField(t, out, "token (shown once — store it now, it cannot be recovered): ")

	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()

	p, err := ids.Verify(token)
	if err != nil {
		t.Fatalf("Verify before revoke: %v", err)
	}
	if p.ID != id {
		t.Fatalf("Verify participant id: got %q, want %q", p.ID, id)
	}

	if _, err := runCLI(t, "revoke", "--campaign", campaignPath, "--id", id); err != nil {
		t.Fatalf("revoke: unexpected error: %v", err)
	}

	if _, err := ids.Verify(token); err == nil {
		t.Fatal("want error from Verify after revoke, got nil")
	}
}

// TestInviteMissingCampaignErrors proves the required-flag validation cobra
// enforces before RunE ever runs also covers invite, not just serve.
func TestInviteMissingCampaignErrors(t *testing.T) {
	if _, err := runCLI(t, "invite", "--name", "Lera", "--role", "player"); err == nil {
		t.Fatal("want error for missing --campaign flag")
	}
}

// TestServeMissingCampaignErrors covers serve's flag validation only — no
// server is ever started here (Task 7 does the end-to-end serve test).
func TestServeMissingCampaignErrors(t *testing.T) {
	if _, err := runCLI(t, "serve"); err == nil {
		t.Fatal("want error for missing --campaign flag")
	}
}

// TestServeOptionalRulesetOmittedStillRequiresCampaign proves --ruleset is
// genuinely optional (ruleset-interpreter Task 6): omitting it must NOT
// itself be an error — the same missing-required-flag error as
// TestServeMissingCampaignErrors fires for the SAME reason (no --campaign),
// not because --ruleset was left out.
func TestServeOptionalRulesetOmittedStillRequiresCampaign(t *testing.T) {
	_, err := runCLI(t, "serve")
	if err == nil || !strings.Contains(err.Error(), "campaign") {
		t.Fatalf("want the missing-campaign error (not a ruleset-related one), got %v", err)
	}
}

// TestServeBadRulesetDirFailsLoudBeforeListening proves a --ruleset
// directory that fails rules.Load fails the command loudly, at boot,
// BEFORE ever starting to listen (composeServer's own fail-loud-at-open
// posture, matching a bad --campaign path) — this is why runCLI (in-
// process, synchronous) can drive this case directly without hanging: a
// bad ruleset never reaches srv.ListenAndServe.
func TestServeBadRulesetDirFailsLoudBeforeListening(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	badRulesetDir := filepath.Join(t.TempDir(), "no-such-ruleset")

	_, err := runCLI(t, "serve", "--campaign", campaignPath, "--ruleset", badRulesetDir)
	if err == nil {
		t.Fatal("want error for a --ruleset directory that does not exist")
	}
	if !strings.Contains(err.Error(), "ruleset") {
		t.Fatalf("error = %q, want it to name the ruleset load failure", err.Error())
	}
}

func extractField(t *testing.T, out, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(line, prefix); ok {
			return rest
		}
	}
	t.Fatalf("no line with prefix %q in output: %q", prefix, out)
	return ""
}
