package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

// Keep serveShutdownTimeout finite: the process exits only once RunE returns
// (SPEC-011).
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
			defer func() { _ = closeFn() }() // closeFn under a live connection: docs/verification-debt.md

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
				// cmd.Context() is already cancelled here, so derive the deadline from
				// context.Background(): a derived one is already cancelled. Shutdown and
				// Close leave hijacked WebSocket connections open, and closeFn runs under
				// them (SPEC-011).
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

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign directory (required)")
	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	cmd.Flags().StringVar(&rulesetDir, "ruleset", "", "path to a ruleset directory (optional; enables use_ability/remove_condition — omit to keep serving without one)")
	cmd.Flags().StringVar(&adventuresDir, "adventures-dir", "", "path to a directory of adventure subdirectories (optional; enables load_adventure — requires --ruleset, every adventure is loaded and validated at boot)")
	_ = cmd.MarkFlagRequired("campaign")

	return cmd
}
