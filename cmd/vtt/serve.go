package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

// serveShutdownTimeout bounds how long RunE waits for srv.Shutdown to drain
// active HTTP handlers once cmd.Context() is canceled (SIGINT/SIGTERM — see
// main.go), before forcing anything still open closed via srv.Close (see
// RunE's own comment on why Shutdown alone cannot be trusted to finish).
const serveShutdownTimeout = 5 * time.Second

// newServeCmd runs the gateway over one campaign (spec §6: one campaign per
// serve invocation — multi-campaign management is a later concern). All
// composition (opening the campaign/identity handles and wiring them into
// an *http.Server) lives in composeServer (serve_compose.go); this command
// only calls it and runs the result.
func newServeCmd() *cobra.Command {
	var campaignPath, addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve a campaign over the WebSocket/HTTP gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, closeFn, err := composeServer(campaignPath, addr)
			if err != nil {
				return err
			}
			defer closeFn()

			fmt.Fprintf(cmd.OutOrStdout(), "vtt serve: listening on %s (campaign %s)\n", addr, campaignPath)

			serveErrCh := make(chan error, 1)
			go func() { serveErrCh <- srv.ListenAndServe() }()

			select {
			case err := <-serveErrCh:
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					return err
				}
				return nil
			case <-cmd.Context().Done():
				// Ctrl-C / SIGTERM (main.go wires both into cmd.Context() via
				// signal.NotifyContext): shut down gracefully, bounded by
				// serveShutdownTimeout, then srv.Close the listener and any
				// non-hijacked conns. NOTE: neither Shutdown nor Close touches
				// HIJACKED WebSocket connections (Go stdlib contract; see
				// composeServer's doc comment) — those die with the process
				// exit that follows. The exit itself never depends on them:
				// this RunE returns within serveShutdownTimeout regardless.
				shutdownCtx, cancel := context.WithTimeout(context.Background(), serveShutdownTimeout)
				defer cancel()
				shutdownErr := srv.Shutdown(shutdownCtx)
				_ = srv.Close()
				<-serveErrCh // Shutdown/Close close the listener, so ListenAndServe has already returned by now.
				if shutdownErr != nil && !errors.Is(shutdownErr, context.DeadlineExceeded) {
					return shutdownErr
				}
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign SQLite file (required)")
	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	_ = cmd.MarkFlagRequired("campaign")

	return cmd
}
