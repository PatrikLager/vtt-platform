package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// newRevokeCmd flips a participant's revoked flag (spec §5): DM-side,
// CLI-only, permanent, not undoable via game-log retraction.
func newRevokeCmd() *cobra.Command {
	var campaignPath, id string

	cmd := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a participant's invite token",
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := identity.Open(campaignPath)
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

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign SQLite file (required)")
	cmd.Flags().StringVar(&id, "id", "", "participant id to revoke (required)")
	_ = cmd.MarkFlagRequired("campaign")
	_ = cmd.MarkFlagRequired("id")

	return cmd
}
