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
// campaign) and should be called once the server has stopped serving
// (after ListenAndServe/Serve returns, or after a graceful Shutdown) —
// never before, since gateway.Server holds live references to both for the
// life of every open connection.
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
