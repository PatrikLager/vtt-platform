package main

import (
	"fmt"
	"net/http"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// composeServer opens the campaign and identity handles for campaignPath
// and wires them into a gateway.Server's Handler on an *http.Server bound
// to addr (not yet listening — the caller starts it, e.g. via
// ListenAndServe or, for tests that need the assigned port, its own
// net.Listener + Serve).
//
// The returned close func closes both handles (identity first, then
// campaign). CAUTION: srv.Shutdown returning does NOT by itself guarantee
// it is safe to call closeFn. http.Server.Shutdown gracefully closes idle
// listeners and connections and waits for active HTTP handlers to return,
// but it does NOT wait for connections that have been hijacked out of
// HTTP's request/response cycle — which is exactly what every WebSocket
// connection is (coder/websocket hijacks the net.Conn on upgrade). A live
// WS connection can still be reading/writing against campaign/identity
// after Shutdown has returned, so closeFn is only actually safe once every
// gateway connection has itself finished closing.
//
// Today, closeFn's callers are responsible for that guarantee themselves:
// the composeServer e2e test (serve_e2e_test.go) explicitly closes its one
// WS connection before calling Shutdown, so no live connection remains by
// the time closeFn runs. `vtt serve` (serve.go) now has a SIGINT/SIGTERM
// shutdown path (RunE watches cmd.Context().Done()), but it does not close
// this gap either — it calls srv.Shutdown then srv.Close as a best-effort
// bound on wall-clock time, then runs closeFn regardless of whether any WS
// connection is still actually mid-teardown, so a connection racing the
// signal can still observe campaign/identity closing under it. Making this
// safe unconditionally — draining/closing every open gateway connection as
// part of shutdown, rather than trusting the caller (or a timeout) — is a
// ledgered carry-forward (see .superpowers/sdd/progress.md), not solved by
// this comment.
func composeServer(campaignPath, addr string) (*http.Server, func() error, error) {
	c, err := campaign.Open(campaignPath)
	if err != nil {
		return nil, nil, fmt.Errorf("vtt serve: open campaign: %w", err)
	}

	ids, err := identity.Open(campaignPath)
	if err != nil {
		c.Close()
		return nil, nil, fmt.Errorf("vtt serve: open identity: %w", err)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: gateway.New(c, ids).Handler(),
	}
	closeFn := func() error {
		idsErr := ids.Close()
		cErr := c.Close()
		if idsErr != nil {
			return idsErr
		}
		return cErr
	}
	return srv, closeFn, nil
}
