package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// newInviteCmd mints a participant + one-time invite token (spec §5, §6):
// DM-side, CLI-only. All minting logic lives in identity.CreateInvite; this
// command only wires flags to it and prints the result.
//
// --campaign is a campaign DIRECTORY (2026-09-01-create-scene-leaves Task
// 4, fix round 1): `vtt invite` may be the FIRST command run against a
// campaign (README's own first worked example), so it opens the campaign
// the same way `vtt serve` does — campaign.Open(campaignPath), which
// creates the directory if it does not exist yet — before opening identity
// on campaign.LogPath(campaignPath) inside it. Without this, invite-first
// left a bare SQLite file that a later `vtt serve` refused as "is a file",
// and serve-first left a directory that invite's old identity.Open(campaignPath)
// could not open as a SQLite file at all — both README sequences were
// dead. See TestCampaignDirectoryWorksInEitherCLIOrdering (cli_test.go).
func newInviteCmd() *cobra.Command {
	var campaignPath, name, role string

	cmd := &cobra.Command{
		Use:   "invite",
		Short: "Mint a one-time invite token for a new participant",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := campaign.Open(campaignPath)
			if err != nil {
				return fmt.Errorf("vtt invite: open campaign: %w", err)
			}
			defer c.Close()

			ids, err := identity.Open(campaign.LogPath(campaignPath))
			if err != nil {
				return fmt.Errorf("vtt invite: open identity: %w", err)
			}
			defer ids.Close()

			token, id, err := ids.CreateInvite(name, identity.Role(role))
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

	cmd.Flags().StringVar(&campaignPath, "campaign", "", "path to the campaign directory (required)")
	cmd.Flags().StringVar(&name, "name", "", "participant display name (required)")
	cmd.Flags().StringVar(&role, "role", "", "participant role: dm, agent, player, spectator (required)")
	// THERE IS NO --controls FLAG (2026-08-24). It wrote a column nothing read,
	// so a DM who used it was told by /api/me that the invitee controlled an
	// actor while authz, the roster and every sight rule said otherwise.
	// Control is conferred by grant_actor_control, and only by that.
	_ = cmd.MarkFlagRequired("campaign")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("role")

	return cmd
}
