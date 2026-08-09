package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// newJoinLinkCmd is the DM's control over the shared join link
// (joining-a-table spec §2): show it, open the door, close it, rotate it.
//
// It exists for the reason plan T6 exists at all. Every layer of this feature
// was complete and gated — identity, the endpoint, promotion, live
// authorization, the client view — and identity.SetJoinOpen had NO CALLER
// anywhere outside its own tests. The door ships closed, so the whole feature
// admitted nobody and no human could reach it. A feature is not done when its
// layers work; it is done when something that already existed can reach it.
//
// CLI-side rather than only in the browser, matching invite/revoke: the DM
// needs this before a browser exists, and it is the only path that works when
// the client bundle is not being served.
func newJoinLinkCmd() *cobra.Command {
	var campaignPath string

	cmd := &cobra.Command{
		Use:   "join-link",
		Short: "Show, open, close or rotate the table's shared join link",
	}
	cmd.PersistentFlags().StringVar(&campaignPath, "campaign", "", "path to the campaign SQLite file (required)")
	_ = cmd.MarkPersistentFlagRequired("campaign")

	withIdentity := func(fn func(*identity.DB, *cobra.Command) error) func(*cobra.Command, []string) error {
		return func(c *cobra.Command, _ []string) error {
			ids, err := identity.Open(campaignPath)
			if err != nil {
				return fmt.Errorf("vtt join-link: open identity: %w", err)
			}
			defer ids.Close()
			return fn(ids, c)
		}
	}

	show := &cobra.Command{
		Use:   "show",
		Short: "Print the join link and whether the door is open",
		RunE: withIdentity(func(ids *identity.DB, c *cobra.Command) error {
			secret, err := ids.JoinSecret()
			if err != nil {
				return fmt.Errorf("vtt join-link show: %w", err)
			}
			// The state word comes FIRST and is unambiguous. A DM reading this
			// in a hurry needs to know whether the thing they are about to
			// paste into a chat window actually admits anyone.
			state := "closed"
			if ids.JoinOpen() {
				state = "open"
			}
			fmt.Fprintf(c.OutOrStdout(), "door: %s\nsecret: %s\nshare: <your-server-url>/?join=%s\n",
				state, secret, secret)
			return nil
		}),
	}

	open := &cobra.Command{
		Use:   "open",
		Short: "Let anyone holding the link join as a spectator",
		RunE: withIdentity(func(ids *identity.DB, c *cobra.Command) error {
			if err := ids.SetJoinOpen(true); err != nil {
				return fmt.Errorf("vtt join-link open: %w", err)
			}
			// Named for what it actually admits. "Open" alone invites the
			// reading that the link now grants access to the game; it grants
			// the right to WATCH, and the DM promotes from there.
			fmt.Fprintln(c.OutOrStdout(), "door: open — anyone with the link can now join as a spectator")
			return nil
		}),
	}

	closeCmd := &cobra.Command{
		Use:   "close",
		Short: "Stop the link admitting anyone new",
		RunE: withIdentity(func(ids *identity.DB, c *cobra.Command) error {
			if err := ids.SetJoinOpen(false); err != nil {
				return fmt.Errorf("vtt join-link close: %w", err)
			}
			// Says what it does NOT do, because that is the question a DM
			// closing the door mid-session actually has.
			fmt.Fprintln(c.OutOrStdout(), "door: closed — nobody new can join; everyone already here is unaffected")
			return nil
		}),
	}

	rotate := &cobra.Command{
		Use:   "rotate",
		Short: "Replace the link's secret, locking out a leaked link",
		RunE: withIdentity(func(ids *identity.DB, c *cobra.Command) error {
			secret, err := ids.RotateJoinSecret()
			if err != nil {
				return fmt.Errorf("vtt join-link rotate: %w", err)
			}
			// Deliberately silent about the door: rotating does not open or
			// close it, and saying so here would suggest otherwise.
			fmt.Fprintf(c.OutOrStdout(),
				"rotated — the old link no longer works; everyone already here keeps their own\nsecret: %s\nshare: <your-server-url>/?join=%s\n",
				secret, secret)
			return nil
		}),
	}

	cmd.AddCommand(show, open, closeCmd, rotate)
	return cmd
}
