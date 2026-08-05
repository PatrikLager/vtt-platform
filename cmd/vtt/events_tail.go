package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
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

			return tailUntilDone(cmd, c.Events(), c.CloseErr)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "gateway ws:// URL (required)")
	cmd.Flags().StringVar(&token, "token", "", "invite/session token (required)")
	cmd.Flags().Int64Var(&after, "after", 0, "catch-up cursor: replay events with sequence > after")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("token")

	return cmd
}

// tailUntilDone writes one protojson line per envelope to cmd's stdout until
// either the event channel closes or cmd's context is canceled. Cancellation
// (Ctrl-C) and a clean server close are both normal, non-error stops.
//
// A close caused by OVERFLOW is not. harness.Client tears the channel down
// when its 256-envelope buffer fills, and the slow party is usually this
// command's own writer: pipe the tail into a consumer that stops reading and
// fmt.Fprintln blocks on the 64KB pipe buffer while envelopes pile up behind
// it. This used to `return nil` for that case too, so the stream ended
// mid-campaign and the command exited 0 — a consumer diffing the output
// against the log would find events missing with nothing to say why.
//
// That is the same silent truncation `vtt state dump` refuses to commit
// (state_dump.go's drainToHead), in the sibling command reading the same
// channel. closeErr is what tells the two closes apart; nil means no
// connection to ask, and no evidence of truncation.
func tailUntilDone(cmd *cobra.Command, events <-chan *vttv1.Envelope, closeErr func() error) error {
	out := cmd.OutOrStdout()
	for {
		select {
		case env, ok := <-events:
			if !ok {
				if closeErr != nil {
					if err := closeErr(); errors.Is(err, harness.ErrEventsOverflow) {
						return fmt.Errorf("vtt events tail: stream truncated after the events printed above: %w", err)
					}
				}
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
