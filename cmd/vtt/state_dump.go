package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// dumpQuietWindow is how long state dump waits for the NEXT envelope
// before concluding the connection has caught up and gone quiet. The wire
// protocol gives no explicit "catch-up finished" marker (the server just
// keeps streaming catch-up backlog straight into live broadcast on the
// same Events() channel — see internal/harness/client.go's Dial), so a
// quiescence window is the only signal available to a client that wants a
// point-in-time snapshot rather than an unbounded tail. 300ms mirrors
// internal/harness/engine.go's own denialAbsenceWindow, which makes the
// identical bet (a real gap this long, either in catch-up backlog or on a
// live table between commands, is what this mode is FOR) for the same
// wire, at the same LAN/loopback latency scale.
const dumpQuietWindow = 300 * time.Millisecond

// newStateCmd is the `vtt state` command group: `dump` today.
func newStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Inspect a campaign's derived state over the wire",
	}
	cmd.AddCommand(newStateDumpCmd())
	return cmd
}

// newStateDumpCmd catches up from sequence 0, folds (harness.Fold), and
// prints the derived state as JSON — a point-in-time snapshot, not a live
// guarantee (dumpQuietWindow's doc comment). The printed JSON carries one
// field beyond the folded state itself: "headSequence", the highest
// envelope sequence this dump actually received before folding — the
// caller's staleness check (e.g. sub-project 6's LLM tools comparing it
// against a sequence they already know about).
func newStateDumpCmd() *cobra.Command {
	var serverURL, token string

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Catch up, fold, and print derived state as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := harness.Dial(cmd.Context(), serverURL, token, 0)
			if err != nil {
				return fmt.Errorf("vtt state dump: %w", err)
			}
			defer c.Close()

			events := drainQuiescent(c.Events(), dumpQuietWindow)
			st, err := harness.Fold(events)
			if err != nil {
				return fmt.Errorf("vtt state dump: %w", err)
			}

			return writeDump(cmd.OutOrStdout(), st, headSequence(events))
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "gateway ws:// URL (required)")
	cmd.Flags().StringVar(&token, "token", "", "invite/session token (required)")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("token")

	return cmd
}

// drainQuiescent collects every envelope from events until window elapses
// with no new arrival, or the channel closes.
func drainQuiescent(events <-chan *vttv1.Envelope, window time.Duration) []*vttv1.Envelope {
	var out []*vttv1.Envelope
	for {
		select {
		case env, ok := <-events:
			if !ok {
				return out
			}
			out = append(out, env)
		case <-time.After(window):
			return out
		}
	}
}

// headSequence returns the highest Sequence among events, or 0 if events
// is empty (sequence 0 is never a real event — the log starts at 1 — so 0
// unambiguously means "nothing received").
func headSequence(events []*vttv1.Envelope) int64 {
	var highest int64
	for _, env := range events {
		if env.Sequence > highest {
			highest = env.Sequence
		}
	}
	return highest
}

// writeDump marshals st (harness.Fold's *engine.State — named only by
// inference, see newStateDumpCmd's doc comment on why cmd/vtt never
// imports internal/engine directly) to JSON, re-decodes it into a
// key->raw-value map, adds "headSequence" as a sibling top-level key, and
// writes the result indented. Round-tripping through a generic map (rather
// than an anonymous struct embedding the state's type by name) is what
// lets this stay generic over st's actual Go type.
func writeDump(w io.Writer, st any, head int64) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("vtt state dump: marshal state: %w", err)
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return fmt.Errorf("vtt state dump: re-decode state: %w", err)
	}
	headRaw, err := json.Marshal(head)
	if err != nil {
		return fmt.Errorf("vtt state dump: marshal headSequence: %w", err)
	}
	fields["headSequence"] = headRaw

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fields)
}
