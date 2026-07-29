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
	var campaignPath, addr, rulesetDir, adventuresDir string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve a campaign over the WebSocket/HTTP gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, closeFn, err := composeServer(campaignPath, addr, rulesetDir, adventuresDir)
			if err != nil {
				return err
			}
			defer func() { _ = closeFn() }() // see composeServer's hijack-contract note

			if rulesetDir != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "vtt serve: listening on %s (campaign %s, ruleset %s)\n", addr, campaignPath, rulesetDir)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "vtt serve: listening on %s (campaign %s)\n", addr, campaignPath)
			}

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
				// context.Background() is deliberate, not an oversight:
				// cmd.Context() is ALREADY CANCELLED — that is why this branch
				// is running — so deriving the shutdown deadline from it would
				// hand Shutdown an expired context and it would return
				// instantly, turning graceful shutdown into an abrupt one.
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
	cmd.Flags().StringVar(&rulesetDir, "ruleset", "", "path to a ruleset directory (optional; enables use_ability/remove_condition — omit to keep serving without one)")
	cmd.Flags().StringVar(&adventuresDir, "adventures-dir", "", "path to a directory of adventure subdirectories (optional; enables load_adventure — requires --ruleset, every adventure is loaded and validated at boot)")
	_ = cmd.MarkFlagRequired("campaign")

	return cmd
}
