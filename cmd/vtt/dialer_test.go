package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildDialer and buildSoakDialer choose between self-contained mode (boot a
// throwaway server) and live mode (dial a running one), and parse the
// tokens file that carries live-mode credentials. Neither had any test.
//
// Everything here exercises the LIVE and ERROR paths only — no server is
// booted, so these run in milliseconds. The self-contained path is already
// covered end-to-end by library_test.go's scenario runs.
//
// The half-specified-flags rejection matters more than it looks: --server
// without --tokens would otherwise fall through to self-contained mode and
// silently boot a throwaway server, so a scenario the operator believed was
// running against their live table would quietly run against an empty
// temporary one and report success.

// writeRawTokensFile writes arbitrary bytes, for the malformed-input cases
// the well-formed writeTokensFile helper (client_e2e_test.go:584) cannot
// produce.
func writeRawTokensFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func validTokensPath(t *testing.T) string {
	t.Helper()
	return writeTokensFile(t,
		map[string]string{"dm": "tok-dm", "player": "tok-player"},
		map[string]string{"player": "pid-player"})
}

func TestBuildDialerRejectsHalfSpecifiedFlags(t *testing.T) {
	cases := []struct {
		name       string
		serverURL  string
		tokensPath string
	}{
		{"server without tokens", "ws://localhost:8080/ws", ""},
		{"tokens without server", "", "/tmp/tokens.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := buildDialer(nil, tc.serverURL, tc.tokensPath)
			if err == nil {
				t.Fatal("want error when only one of --server/--tokens is given")
			}
			if !strings.Contains(err.Error(), "together") {
				t.Errorf("error should say the flags go together, got: %v", err)
			}
		})
	}
}

func TestBuildSoakDialerRejectsHalfSpecifiedFlags(t *testing.T) {
	cases := []struct {
		name       string
		serverURL  string
		tokensPath string
	}{
		{"server without tokens", "ws://localhost:8080/ws", ""},
		{"tokens without server", "", "/tmp/tokens.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := buildSoakDialer(tc.serverURL, tc.tokensPath)
			if err == nil {
				t.Fatal("want error when only one of --server/--tokens is given")
			}
			if !strings.Contains(err.Error(), "together") {
				t.Errorf("error should say the flags go together, got: %v", err)
			}
		})
	}
}

func TestBuildSoakDialerReportsMissingTokensFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-tokens.json")
	_, _, _, err := buildSoakDialer("ws://localhost:8080/ws", missing)
	if err == nil {
		t.Fatal("want error for a tokens file that does not exist")
	}
}

func TestBuildSoakDialerReportsMalformedTokensFile(t *testing.T) {
	path := writeRawTokensFile(t, "{not json")
	_, _, _, err := buildSoakDialer("ws://localhost:8080/ws", path)
	if err == nil {
		t.Fatal("want error for a malformed tokens file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error should name the offending file, got: %v", err)
	}
}

func TestBuildSoakDialerLiveModeReturnsIDsAndNoopTeardown(t *testing.T) {
	path := validTokensPath(t)

	dial, ids, teardown, err := buildSoakDialer("ws://localhost:8080/ws", path)
	if err != nil {
		t.Fatal(err)
	}
	if dial == nil {
		t.Fatal("want a dialer in live mode")
	}
	if ids["player"] != "pid-player" {
		t.Errorf("ids: got %v, want player=pid-player — {{id:}} placeholders resolve from this", ids)
	}
	// Live mode did not start anything, so teardown must not try to tear
	// down the operator's running server.
	if err := teardown(); err != nil {
		t.Errorf("live-mode teardown must be a no-op, got: %v", err)
	}
}

func TestBuildDialerLiveModeReturnsIDsAndNoopTeardown(t *testing.T) {
	path := validTokensPath(t)

	dial, ids, teardown, err := buildDialer(nil, "ws://localhost:8080/ws", path)
	if err != nil {
		t.Fatal(err)
	}
	if dial == nil {
		t.Fatal("want a dialer in live mode")
	}
	if ids["player"] != "pid-player" {
		t.Errorf("ids: got %v, want player=pid-player", ids)
	}
	if err := teardown(); err != nil {
		t.Errorf("live-mode teardown must be a no-op, got: %v", err)
	}
}

// TestDialerForRejectsUnknownParticipant pins the failure a scenario hits
// when its participant list and the tokens file disagree — a named error,
// not a nil-token dial that fails later with something opaque.
func TestDialerForRejectsUnknownParticipant(t *testing.T) {
	dial := dialerFor("ws://localhost:8080/ws", map[string]string{"dm": "tok-dm"})
	_, err := dial("nobody", 0)
	if err == nil {
		t.Fatal("want error dialing a participant with no token")
	}
	if !strings.Contains(err.Error(), "nobody") {
		t.Errorf("error must name the participant, got: %v", err)
	}
}
