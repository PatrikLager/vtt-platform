package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// newRevokeCmd flips a participant's revoked flag (spec §5): DM-side,
// CLI-only, permanent, not undoable via game-log retraction.
//
// --campaign is a campaign DIRECTORY (2026-09-01-create-scene-leaves Task
// 4, fix round 1) — same directory contract as invite.go, so it opens the
// campaign the way `vtt serve` does before opening identity on
// campaign.LogPath(campaignPath) inside it.
func newRevokeCmd() *cobra.Command {
	var campaignPath, id string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a participant's invite token",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := campaign.Open(campaignPath)
			if err != nil {
				return fmt.Errorf("vtt revoke: open campaign: %w", err)
			}
			defer c.Close()

			ids, err := identity.Open(campaign.LogPath(campaignPath))
			if err != nil {
				return fmt.Errorf("vtt revoke: open identity: %w", err)
			}
			defer ids.Close()

			if err := ids.Revoke(id); err != nil {
				return fmt.Errorf("vtt revoke: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "revoked participant %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign directory (required)")
	cmd.Flags().StringVar(&id, "id", "", "participant id to revoke (required)")
	_ = cmd.MarkFlagRequired("campaign")
	_ = cmd.MarkFlagRequired("id")

	return cmd
}
