// harness_boot.go is the self-contained boot glue for `vtt client run`
// (task-3-brief.md): it composes a real gateway server (composeServer,
// serve_compose.go) on a throwaway temp campaign, mints one invite token
// per scenario participant directly via identity, and hands back only
// PLAIN STRINGS (a ws:// URL and a name→token map) — never the
// *http.Server, *campaign.Campaign, or *identity.DB it built them from.
// That boundary is deliberate and load-bearing: internal/harness's own
// package comment (client.go) documents the P1 rule that the harness core
// may act only through the wire, the same way a live `--server`/`--tokens`
// run does — self-contained mode must not become a back door that hands
// the harness a server object just because the process happens to own
// both ends.
package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/PatrikLager/vtt-platform/internal/harness"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// bootResult is bootSelfContained's output: everything `vtt client run`
// needs to drive the scenario, expressed only as strings plus a teardown
// func — the strings (WSURL, Tokens) are exactly what a live `--server`/
// `--tokens` invocation would have supplied instead, so the two modes
// converge on the same harness.Dialer shape (client_run.go's dialerFor).
type bootResult struct {
	WSURL  string
	Tokens map[string]string // participant name -> invite token
	// close stops the server, closes the campaign/identity handles, and
	// removes the temp campaign dir. Always non-nil on a successful
	// bootSelfContained; the caller must call it exactly once, after the
	// scenario run has finished closing every harness Conn (mirrors
	// serve_compose.go's composeServer doc comment on why closeFn is only
	// safe once no live WS connection remains).
	close func() error
}

// bootSelfContained starts an in-process gateway server on a fresh temp
// campaign file, mints one invite token per sc.Participants (role/controls
// taken straight from the scenario), and returns a bootResult ready for
// dialerFor(boot.WSURL, boot.Tokens).
func bootSelfContained(sc *harness.Scenario) (*bootResult, error) {
	dir, err := os.MkdirTemp("", "vtt-harness-run-*")
	if err != nil {
		return nil, fmt.Errorf("vtt client run: boot temp dir: %w", err)
	}
	campaignPath := filepath.Join(dir, "campaign.db")

	srv, closeCompose, err := composeServer(campaignPath, "127.0.0.1:0")
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("vtt client run: boot server: %w", err)
	}

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		closeCompose()
		os.RemoveAll(dir)
		return nil, fmt.Errorf("vtt client run: listen: %w", err)
	}
	serveErrCh := make(chan error, 1)
	go func() { serveErrCh <- srv.Serve(ln) }()

	tokens, err := mintInvites(campaignPath, sc)
	if err != nil {
		srv.Close()
		closeCompose()
		os.RemoveAll(dir)
		return nil, err
	}

	wsURL := "ws://" + ln.Addr().String() + "/ws"
	closeFn := func() error {
		shutdownErr := srv.Close()
		<-serveErrCh
		composeErr := closeCompose()
		removeErr := os.RemoveAll(dir)
		return firstNonNil(shutdownErr, composeErr, removeErr)
	}
	return &bootResult{WSURL: wsURL, Tokens: tokens, close: closeFn}, nil
}

// mintInvites opens its own identity.DB handle on campaignPath (a second,
// short-lived handle alongside the one composeServer's gateway holds open —
// the same pattern serve_e2e_test.go and internal/gateway's exit fixture
// both use to mint invites against a server they didn't mint them through)
// and mints one invite per participant, closing the handle before
// returning either way.
func mintInvites(campaignPath string, sc *harness.Scenario) (map[string]string, error) {
	ids, err := identity.Open(campaignPath)
	if err != nil {
		return nil, fmt.Errorf("vtt client run: open identity for minting: %w", err)
	}
	defer ids.Close()

	tokens := make(map[string]string, len(sc.Participants))
	for _, p := range sc.Participants {
		token, _, err := ids.CreateInvite(p.Name, identity.Role(p.Role), p.Controls)
		if err != nil {
			return nil, fmt.Errorf("vtt client run: mint invite for %q: %w", p.Name, err)
		}
		tokens[p.Name] = token
	}
	return tokens, nil
}

// firstNonNil returns the first non-nil error in errs, or nil if every one
// is nil — used by closeFn to report a real failure from any of its three
// independent cleanup steps without masking the other two silently.
func firstNonNil(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
