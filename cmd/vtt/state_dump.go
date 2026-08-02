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

// dumpQuietWindow bounds the wait for the TAIL: envelopes broadcast live
// while the catch-up backlog was being read. It is no longer how the dump
// decides it has caught up — see dumpCatchUpTimeout.
//
// It USED to be that decision, and it was a guess. The comment here said the
// wire gives no explicit "catch-up finished" marker, which was true, so this
// stopped after 300ms of silence and called it caught up. Mid-replay a 300ms
// gap is ordinary, and the dump then printed a SILENTLY INCOMPLETE state —
// indistinguishable from a correct one, from a command whose output the golden
// corpus and the TypeScript fold-parity keystone are compared against.
//
// The marker exists now (contract CatchUpHead): the server announces the head
// of this connection's backlog before sending any of it.
const dumpQuietWindow = 300 * time.Millisecond

// dumpCatchUpTimeout bounds the wait to reach the announced head. A BACKSTOP
// against a stuck replay, not a measurement — a connection that keeps up never
// reaches it.
const dumpCatchUpTimeout = 30 * time.Second

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

			head, err := c.CatchUpHead(cmd.Context())
			if err != nil {
				return fmt.Errorf("vtt state dump: catch-up head: %w", err)
			}

			events, reached := drainToHead(c.Events(), head, dumpQuietWindow, dumpCatchUpTimeout)
			if !reached {
				// FAIL, never print. A short dump that looks complete is the
				// failure this whole path exists to prevent; the caller can
				// retry, and a partial snapshot on stdout could not be told
				// apart from a real one.
				return fmt.Errorf("vtt state dump: caught up only to sequence %d of %d within %s — refusing to print a truncated state",
					headSequence(events), head, dumpCatchUpTimeout)
			}

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

// drainToHead collects envelopes until the announced catch-up head has been
// seen AND the stream has then been quiet for window, or until timeout. The
// bool reports whether head was reached.
//
// Both conditions matter. Stopping at head alone would drop envelopes
// broadcast live just behind it, which is a truncation from the other side;
// stopping at quiet alone is the bug this replaced. A head of 0 means the log
// was empty at subscribe time, so there is nothing to catch up to and the
// quiet window is the only sensible signal — which is the old behaviour,
// correctly scoped to the one case where it was never a guess.
//
// Deliberately a sibling of internal/harness/soak.go's drainToSequence rather
// than an import: cmd/vtt depends on harness, never the reverse.
func drainToHead(events <-chan *vttv1.Envelope, head int64, window, timeout time.Duration) ([]*vttv1.Envelope, bool) {
	var out []*vttv1.Envelope
	deadline := time.After(timeout)
	reached := head == 0
	for {
		select {
		case env, ok := <-events:
			if !ok {
				return out, reached
			}
			out = append(out, env)
			if env.Sequence >= head {
				reached = true
			}
		case <-time.After(window):
			if reached {
				return out, true
			}
		case <-deadline:
			return out, reached
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
