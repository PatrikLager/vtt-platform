package main

import (
	"bytes"
	"encoding/json"
	"os"
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

// TestServeEmptyAdventuresDirFailsLoudBeforeListening covers fix-wave F4:
// an EXISTING --adventures-dir with zero subdirectories (a typo'd or
// never-synced path pointing at a real-but-wrong or empty directory) fails
// loud at boot, before ever starting to listen — mirroring
// TestServeBadAdventuresDirFailsLoudBeforeListening's nonexistent-dir case,
// which already failed loud; before this fix, an EXISTING-but-empty dir
// booted "successfully" with zero adventures configured, deferring the
// misconfiguration to the table (spec §7's fail-loud-at-boot posture).
func TestServeEmptyAdventuresDirFailsLoudBeforeListening(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	rulesetDir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir(dnd45e-minimal): %v", err)
	}
	emptyAdventuresDir := t.TempDir() // exists, zero subdirectories

	_, err = runCLI(t, "serve", "--campaign", campaignPath, "--ruleset", rulesetDir, "--adventures-dir", emptyAdventuresDir)
	if err == nil {
		t.Fatal("want error for an existing-but-empty --adventures-dir")
	}
	if !strings.Contains(err.Error(), "no adventures") {
		t.Fatalf("error = %q, want it to contain %q", err.Error(), "no adventures")
	}
}

// TestServeBootsAMixedAdventuresDirServingOnlyThisTable replaces a test that
// asserted the OPPOSITE, and the reversal is deliberate.
//
// The P12 plan bound it: "adventures with a DIFFERENT ruleset id than served:
// boot error too — the dir is for THIS table". In practice that made the
// repo's own ./adventures unbootable (cellar-rats declares tavern-brawl,
// goblin-ambush declares dnd45e-minimal), which is why a symlinked
// single-ruleset fixture existed at all — and that symlink is the one
// gremlins' workdir copy drops, which is what made cmd/vtt unmeasurable.
//
// AMENDED by Patrik 2026-08-06: an adventures dir is a LIBRARY. Serve what is
// written for this table, skip what is not, and still fail loud when nothing
// matches (loadAdventuresDir's own tests cover that side).
func TestServeBootsAMixedAdventuresDirServingOnlyThisTable(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	rulesetDir, err := resolveRulesetDir("dnd45e-minimal")
	if err != nil {
		t.Fatalf("resolveRulesetDir(dnd45e-minimal): %v", err)
	}
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("findRepoRoot: %v", err)
	}

	// Port 0: compose and tear down without ever occupying a real port. An
	// earlier version of this test drove the full `serve` command, which
	// under the amended binding no longer fails — so it BOOTED and blocked
	// the suite on :8080 until it was killed.
	_, closeFn, err := composeServer(campaignPath, "127.0.0.1:0", rulesetDir,
		filepath.Join(root, "adventures"), "")
	if err != nil {
		t.Fatalf("composeServer against the real mixed adventures/ = %v; "+
			"a library holding one adventure for another table must still boot", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("closeFn: %v", err)
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

// joinURLFormat is contract/testdata/join_url_format.json, the one place the
// ?join= shape is written down (#46).
//
// It is a FIXTURE rather than a Go constant because the format spans two
// languages: three writers here and in client/src/view/dm.ts, one reader in
// client/src/app.ts, and neither mutation gate can see it whole — gremlins does
// not mutate string literals, and Stryker cannot see Go at all. Renaming the
// parameter used to leave `vtt join-link show` printing a dead link with every
// gate green. Now all four sites derive from this file and fail together.
func joinURLFormat(t *testing.T) struct {
	QueryParameter string `json:"queryParameter"`
	ShareSuffix    string `json:"shareSuffix"`
} {
	t.Helper()
	var f struct {
		QueryParameter string `json:"queryParameter"`
		ShareSuffix    string `json:"shareSuffix"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "testdata", "join_url_format.json"))
	if err != nil {
		t.Fatalf("reading the join-url fixture: %v", err)
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parsing the join-url fixture: %v", err)
	}
	if f.ShareSuffix == "" || f.QueryParameter == "" {
		t.Fatal("the join-url fixture is empty — every assertion built from it would be vacuous")
	}
	return f
}

// TestJoinLinkOpenShareCloseRotate walks the whole door from the command line.
//
// The CLI matters more here than it does for invite/revoke, and not only as a
// convenience: until this existed, identity.SetJoinOpen had NO caller anywhere
// outside its own tests. Five completed tasks, every gate green, and the shared
// join link admitted nobody because nothing in the product could open it.
func TestJoinLinkOpenShareCloseRotate(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	// SHOW works before anything else does — a DM needs the link in hand
	// before deciding to open the door, not after.
	out, err := runCLI(t, "join-link", "show", "--campaign", campaignPath)
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	// The FIELD, not the whole blob: the output also carries a base64 secret,
	// and a substring search over it is one lucky random string away from
	// asserting nothing.
	if door := extractField(t, out, "door: "); door != "closed" {
		t.Fatalf("a fresh campaign must report the door CLOSED, got %q", door)
	}
	first := secretFrom(t, campaignPath)
	// The SHARE line specifically. `Contains(out, secret)` is satisfied by the
	// `secret:` line on its own, so the whole share clause could be deleted and
	// this stayed green — and the share line is the only part a DM actually
	// pastes to somebody. It is one of THREE places the ?join= spelling is
	// written against app.ts's single reader — see joinURLFormat below.
	// DERIVED from the fixture, not written out: a literal here would be a
	// fourth independent copy of the format, which is the defect (#46).
	format := joinURLFormat(t)
	if share := extractField(t, out, "share: "); !strings.Contains(share, format.ShareSuffix+first) {
		t.Fatalf("show must print a pasteable link carrying the secret in the %s form, got %q",
			format.ShareSuffix, share)
	}

	if _, err := runCLI(t, "join-link", "open", "--campaign", campaignPath); err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err = runCLI(t, "join-link", "show", "--campaign", campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	if door := extractField(t, out, "door: "); door != "open" {
		t.Fatalf("after open, show must report it open, got %q", door)
	}

	// Rotating changes the secret and leaves the door where it was — a rotate
	// that quietly shut the table would be a very unwelcome surprise.
	rotated, err := runCLI(t, "join-link", "rotate", "--campaign", campaignPath)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	second := secretFrom(t, campaignPath)
	if second == first {
		t.Fatal("rotate must actually change the secret")
	}
	// ROTATE PRINTS ITS OWN SHARE LINE, and nothing asserted it: this test
	// checked that the secret changed and the door stayed open, then re-ran
	// `show`. That left the THIRD writer of the ?join= format (joinlink.go's
	// rotate) unpinned — the one a DM reads at the worst possible moment,
	// having just locked everybody out of the old link. Derived from the
	// fixture like the others (#46).
	if share := extractField(t, rotated, "share: "); !strings.Contains(share, format.ShareSuffix+second) {
		t.Fatalf("rotate must print a pasteable link carrying the NEW secret in the %s "+
			"form, got %q", format.ShareSuffix, share)
	}
	out, err = runCLI(t, "join-link", "show", "--campaign", campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	if door := extractField(t, out, "door: "); door != "open" {
		t.Fatalf("rotating the link must not close the door, got %q", door)
	}

	if _, err := runCLI(t, "join-link", "close", "--campaign", campaignPath); err != nil {
		t.Fatalf("close: %v", err)
	}
	out, err = runCLI(t, "join-link", "show", "--campaign", campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	if door := extractField(t, out, "door: "); door != "closed" {
		t.Fatalf("a door that only opens is not a door, got %q", door)
	}
}

// secretFrom reads the current join secret straight from identity, so the test
// asserts against the STORED value rather than against whatever the command
// happened to print.
func secretFrom(t *testing.T, campaignPath string) string {
	t.Helper()
	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	s, err := ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestJoinLinkOpenTakesAnAdmissionBudget is the CLI half of the cap (spec §2,
// amended 2026-08-11).
//
// The number is asserted in the OUTPUT as well as in the behaviour, and that is
// deliberate: a budget nobody was told about is one the DM discovers when their
// last player is turned away with the same message a stranger gets. `open`
// prints it even when it was defaulted.
func TestJoinLinkOpenTakesAnAdmissionBudget(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")

	out, err := runCLI(t, "join-link", "open", "--campaign", campaignPath, "--admit", "3")
	if err != nil {
		t.Fatalf("open --admit 3: %v", err)
	}
	if !strings.Contains(out, "next 3 people") {
		t.Fatalf("open --admit 3 said %q — a DM who is not told the number finds out "+
			"when somebody is refused", out)
	}

	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	secret, err := ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if ok, err := ids.JoinAdmits(secret); !ok {
			t.Fatalf("admission %d of 3 refused (%v) — --admit did not reach the door", i+1, err)
		}
	}
	if ok, _ := ids.JoinAdmits(secret); ok {
		t.Fatal("a fourth joiner got in against --admit 3")
	}
}

// TestJoinLinkOpenWithNoBudgetAdmitsSomebody pins the direction the default
// must fail in.
//
// An absent count is 0, and 0 admits nobody. Defaulting it to that would open
// a door that refuses everyone, with nothing on either side saying why — the
// DM sees "door: open", the player sees the same 403 a stranger gets, and no
// log distinguishes them. So the default is a NUMBER, and this asserts a
// plain `open` still lets a person in.
func TestJoinLinkOpenWithNoBudgetAdmitsSomebody(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	if _, err := runCLI(t, "join-link", "open", "--campaign", campaignPath); err != nil {
		t.Fatalf("open: %v", err)
	}
	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ids.Close()
	secret, err := ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := ids.JoinAdmits(secret); !ok {
		t.Fatalf("a door opened without --admit let nobody in (%v) — the default is zero, "+
			"which is an open door that refuses everyone", err)
	}
}

// TestJoinLinkShowReportsWhatIsLeftOfTheBudget closes the gap review named:
// a door now has THREE states — open, shut, and open but spent — and without
// this the DM's only way to learn the third is a player reporting they were
// turned away, with the same message a stranger gets.
func TestJoinLinkShowReportsWhatIsLeftOfTheBudget(t *testing.T) {
	campaignPath := filepath.Join(t.TempDir(), "campaign.db")
	if _, err := runCLI(t, "join-link", "open", "--campaign", campaignPath, "--admit", "2"); err != nil {
		t.Fatal(err)
	}

	out, err := runCLI(t, "join-link", "show", "--campaign", campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := extractField(t, out, "admissions: "); got != "2 of 2 left" {
		t.Fatalf("a freshly opened door reports admissions %q, want \"2 of 2 left\"", got)
	}
	// `door` stays the machine-readable answer; the budget is its own field.
	if door := extractField(t, out, "door: "); door != "open" {
		t.Fatalf("door reads %q — the budget leaked into the field a caller parses", door)
	}

	ids, err := identity.Open(campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	secret, err := ids.JoinSecret()
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := ids.JoinAdmits(secret); !ok {
		t.Fatalf("admission refused: %v", err)
	}
	ids.Close()

	out, err = runCLI(t, "join-link", "show", "--campaign", campaignPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := extractField(t, out, "admissions: "); got != "1 of 2 left" {
		t.Fatalf("after one joiner, admissions reads %q, want \"1 of 2 left\" — the count "+
			"does not move, so the DM cannot tell a live door from a spent one", got)
	}
}
