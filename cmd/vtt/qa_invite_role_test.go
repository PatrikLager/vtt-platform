package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

const qaInviteRolePlayerRefusal = "identity: a participant becomes a " +
	"player when given a character; invite them as a spectator"

func qaInviteRoleBuild(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "vtt")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func qaInviteRoleRun(
	t *testing.T, bin string, args ...string,
) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var o, e bytes.Buffer
	cmd.Stdout, cmd.Stderr = &o, &e
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &ee):
		code = ee.ExitCode()
	default:
		t.Fatalf("run %v: %v", args, err)
	}
	return o.String(), e.String(), code
}

func qaInviteRoleToken(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(strings.ToLower(line), "token") {
			f := strings.Fields(line)
			if len(f) > 0 {
				return f[len(f)-1]
			}
		}
	}
	return ""
}

func qaInviteRoleList(t *testing.T, dir string) []*identity.Participant {
	t.Helper()
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	ps, err := db.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return ps
}

// VTT-911
func TestQAInviteRoleMintsDMAgentSpectator(t *testing.T) {
	bin := qaInviteRoleBuild(t)
	dir := t.TempDir()
	cases := []struct {
		name string
		role identity.Role
	}{
		{"Ada", identity.RoleDM},
		{"Bix", identity.RoleAgent},
		{"Cato", identity.RoleSpectator},
	}
	tokens := map[string]string{}
	for _, c := range cases {
		out, errOut, code := qaInviteRoleRun(t, bin, "invite",
			"--campaign", dir, "--name", c.name, "--role", string(c.role))
		if code != 0 {
			t.Fatalf("invite --role %s exit %d\nstdout %q\nstderr %q",
				c.role, code, out, errOut)
		}
		tok := qaInviteRoleToken(out)
		if tok == "" {
			t.Fatalf("invite --role %s printed no token line: %q",
				c.role, out)
		}
		tokens[c.name] = tok
	}
	db, err := identity.Open(campaign.LogPath(dir))
	if err != nil {
		t.Fatalf("identity.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	for _, c := range cases {
		p, err := db.Verify(tokens[c.name])
		if err != nil {
			t.Fatalf("Verify printed token for %s: %v", c.name, err)
		}
		if p.Name != c.name || p.Role != c.role {
			t.Fatalf("Verify = %+v, want %s as %s", p, c.name, c.role)
		}
	}
}

// VTT-911
func TestQAInviteRoleRefusesPlayer(t *testing.T) {
	bin := qaInviteRoleBuild(t)
	dir := t.TempDir()
	out, errOut, code := qaInviteRoleRun(t, bin, "invite",
		"--campaign", dir, "--name", "Ada", "--role", "dm")
	if code != 0 {
		t.Fatalf("seed dm exit %d: %q %q", code, out, errOut)
	}
	out, errOut, code = qaInviteRoleRun(t, bin, "invite",
		"--campaign", dir, "--name", "Pell", "--role", "player")
	if code == 0 {
		t.Fatalf("invite --role player exited 0\nstdout %q\nstderr %q",
			out, errOut)
	}
	all := out + errOut
	if strings.Contains(strings.ToLower(all), "token") {
		t.Fatalf("invite --role player printed a token line: %q", all)
	}
	if !strings.Contains(all, qaInviteRolePlayerRefusal) {
		t.Fatalf("invite --role player output %q lacks %q",
			all, qaInviteRolePlayerRefusal)
	}
	for _, p := range qaInviteRoleList(t, dir) {
		if p.Name == "Pell" {
			t.Fatalf("refused player was minted: %+v", p)
		}
	}
}

// VTT-911
func TestQAInviteRoleRefusesUnknownRole(t *testing.T) {
	bin := qaInviteRoleBuild(t)
	for _, role := range []string{"gm", "Player", "owner"} {
		dir := t.TempDir()
		out, errOut, code := qaInviteRoleRun(t, bin, "invite",
			"--campaign", dir, "--name", "Rho", "--role", role)
		if code == 0 {
			t.Fatalf("invite --role %q exited 0: %q %q",
				role, out, errOut)
		}
		if qaInviteRoleToken(out) != "" {
			t.Fatalf("invite --role %q printed a token: %q", role, out)
		}
	}
}

// VTT-911
func TestQAInviteRoleHelpNamesThreeRoles(t *testing.T) {
	bin := qaInviteRoleBuild(t)
	out, errOut, code := qaInviteRoleRun(t, bin, "invite", "--help")
	if code != 0 {
		t.Fatalf("invite --help exit %d: %q", code, errOut)
	}
	var roleLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "--role") {
			roleLine = line
		}
	}
	if roleLine == "" {
		t.Fatalf("invite --help names no --role flag: %q", out)
	}
	for _, r := range []string{"dm", "agent", "spectator"} {
		if !strings.Contains(roleLine, r) {
			t.Fatalf("--role help %q does not name %s", roleLine, r)
		}
	}
	if strings.Contains(roleLine, "player") {
		t.Fatalf("--role help %q still offers player", roleLine)
	}
}
