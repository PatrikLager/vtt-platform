package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// newInviteCmd mints a participant + one-time invite token (spec §5, §6):
// DM-side, CLI-only. All minting logic lives in identity.CreateInvite; this
// command only wires flags to it and prints the result.
func newInviteCmd() *cobra.Command {
	var campaignPath, name, role string
	var controls []string

	cmd := &cobra.Command{
		Use:   "invite",
		Short: "Mint a one-time invite token for a new participant",
		RunE: func(cmd *cobra.Command, args []string) error {
			ids, err := identity.Open(campaignPath)
			if err != nil {
				return fmt.Errorf("vtt invite: open identity: %w", err)
			}
			defer ids.Close()

			token, id, err := ids.CreateInvite(name, identity.Role(role), controls)
			if err != nil {
				return fmt.Errorf("vtt invite: %w", err)
			}

			// The token is printed here, once, and nowhere else in this
			// process (never logged) — identity.CreateInvite's doc comment:
			// only its SHA-256 hash is persisted, so this is the one and
			// only chance to see it again.
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "participant id: %s\n", id)
			fmt.Fprintf(out, "token (shown once — store it now, it cannot be recovered): %s\n", token)
			return nil
		},
	}

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign SQLite file (required)")
	cmd.Flags().StringVar(&name, "name", "", "participant display name (required)")
	cmd.Flags().StringVar(&role, "role", "", "participant role: dm, agent, player, spectator (required)")
	cmd.Flags().StringSliceVar(&controls, "controls", nil, "comma-separated actor ids this participant controls")
	_ = cmd.MarkFlagRequired("campaign")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("role")

	return cmd
}
