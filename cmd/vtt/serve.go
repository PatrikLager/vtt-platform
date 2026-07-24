package main

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/gateway"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// newServeCmd runs the gateway over one campaign (spec §6: one campaign per
// serve invocation — multi-campaign management is a later concern). All
// serving logic lives in internal/gateway.Server; this command only opens
// the two handles it needs and wires them to an HTTP listener.
func newServeCmd() *cobra.Command {
	var campaignPath, addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve a campaign over the WebSocket/HTTP gateway",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := campaign.Open(campaignPath)
			if err != nil {
				return fmt.Errorf("vtt serve: open campaign: %w", err)
			}
			defer c.Close()

			ids, err := identity.Open(campaignPath)
			if err != nil {
				return fmt.Errorf("vtt serve: open identity: %w", err)
			}
			defer ids.Close()

			fmt.Fprintf(cmd.OutOrStdout(), "vtt serve: listening on %s (campaign %s)\n", addr, campaignPath)
			return http.ListenAndServe(addr, gateway.New(c, ids).Handler())
		},
	}

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign SQLite file (required)")
	cmd.Flags().StringVar(&addr, "addr", ":8080", "address to listen on")
	_ = cmd.MarkFlagRequired("campaign")

	return cmd
}
