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

// TestServeAdventuresDirWithoutRulesetFailsLoudBeforeListening covers the
// adventure-format Task 4 flag pairing binding (spec §7, mirroring the MCP
// flag precedent for `vtt mcp`): `--adventures-dir` without `--ruleset` is
// a boot-time error, before ever starting to listen — every adventure
// declares the ruleset id it was written for, and there is no "the served
// ruleset" to validate it against without one.
func TestServeAdventuresDirWithoutRulesetFailsLoudBeforeListening(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	_, err := runCLI(t, "serve", "--campaign", campaignPath, "--adventures-dir", t.TempDir())
	if err == nil {
		t.Fatal("want error for --adventures-dir without --ruleset")
	}
	if !strings.Contains(err.Error(), "--adventures-dir") || !strings.Contains(err.Error(), "--ruleset") {
		t.Fatalf("error = %q, want it to name both --adventures-dir and --ruleset", err.Error())
	}
}

// TestServeBadAdventuresDirFailsLoudBeforeListening mirrors
// TestServeBadRulesetDirFailsLoudBeforeListening: a --adventures-dir that
// does not exist fails the command loudly, at boot, before ever starting
// to listen.
func TestServeBadAdventuresDirFailsLoudBeforeListening(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	rulesetDir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir(dnd45e-minimal): %v", err)
	}
	badAdventuresDir := filepath.Join(t.TempDir(), "no-such-adventures-dir")

	_, err = runCLI(t, "serve", "--campaign", campaignPath, "--ruleset", rulesetDir, "--adventures-dir", badAdventuresDir)
	if err == nil {
		t.Fatal("want error for a --adventures-dir that does not exist")
	}
	if !strings.Contains(err.Error(), "adventures") {
		t.Fatalf("error = %q, want it to name the adventures load failure", err.Error())
	}
}

// TestServeAdventureDeclaringDifferentRulesetFailsLoudBeforeListening
// covers the spec §7 binding literally: "adventures declaring a different
// ruleset than served = boot error too — the dir is for THIS table." The
// real, committed adventures/cellar-rats declares ruleset "tavern-brawl";
// serving with --ruleset dnd45e-minimal must reject it at boot, naming the
// mismatch (adventure.Load's own ruleset-id-match error, propagated
// verbatim through loadAdventuresDir/composeServer).
func TestServeAdventureDeclaringDifferentRulesetFailsLoudBeforeListening(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	rulesetDir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir(dnd45e-minimal): %v", err)
	}
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}

	_, err = runCLI(t, "serve", "--campaign", campaignPath, "--ruleset", rulesetDir,
		"--adventures-dir", filepath.Join(root, "adventures"))
	if err == nil {
		t.Fatal("want error serving --adventures-dir adventures/ (mixed rulesets) against --ruleset dnd45e-minimal")
	}
	if !strings.Contains(err.Error(), "ruleset") {
		t.Fatalf("error = %q, want it to name the ruleset mismatch", err.Error())
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
