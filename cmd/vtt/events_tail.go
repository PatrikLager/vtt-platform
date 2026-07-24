package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// newEventsCmd is the `vtt events` command group: `tail` today.
func newEventsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Inspect a campaign's event stream over the wire",
	}
	cmd.AddCommand(newEventsTailCmd())
	return cmd
}

// newEventsTailCmd streams envelopes from a live gateway as protojson
// lines until the command's context is canceled: Ctrl-C (SIGINT) or
// SIGTERM in a real terminal — main.go wires those into the context it
// passes to root.ExecuteContext via signal.NotifyContext, so cmd.Context()
// here is that same cancelable context, not context.Background(); an
// explicit context cancel in tests (see client_e2e_test.go's
// runCLIStreaming for in-process cancellation, and
// TestEventsTailBinaryExitsCleanlyOnSIGINT for real OS signal delivery to
// the built binary).
func newEventsTailCmd() *cobra.Command {
	var serverURL, token string
	var after int64

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream envelopes as protojson lines until interrupted",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := harness.Dial(cmd.Context(), serverURL, token, after)
			if err != nil {
				return fmt.Errorf("vtt events tail: %w", err)
			}
			defer c.Close()

			return tailUntilDone(cmd, c)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "gateway ws:// URL (required)")
	cmd.Flags().StringVar(&token, "token", "", "invite/session token (required)")
	cmd.Flags().Int64Var(&after, "after", 0, "catch-up cursor: replay events with sequence > after")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("token")

	return cmd
}

// tailUntilDone writes one protojson line per envelope to cmd's stdout
// until either c's event channel closes (server/local close) or cmd's
// context is canceled — the latter is a normal, non-error stop, not a
// failure to report.
func tailUntilDone(cmd *cobra.Command, c *harness.Client) error {
	out := cmd.OutOrStdout()
	for {
		select {
		case env, ok := <-c.Events():
			if !ok {
				return nil
			}
			line, err := protojson.Marshal(env)
			if err != nil {
				return fmt.Errorf("vtt events tail: marshal envelope: %w", err)
			}
			fmt.Fprintln(out, string(line))
		case <-cmd.Context().Done():
			return nil
		}
	}
}
