package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

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
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign SQLite file (required)")
	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	_ = cmd.MarkFlagRequired("campaign")

	return cmd
}
