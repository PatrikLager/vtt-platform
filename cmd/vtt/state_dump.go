package main

import (
	"encoding/json"
	"errors"
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

// dumpAfter is the cursor every dump dials with; keep the dial and
// drainToHead's head <= after test on this one name.
const dumpAfter = 0

// newStateDumpCmd prints the fold of one log as JSON with "headSequence" and,
// for a player's or a spectator's seat, "log" (writeDump, VTT-426).
func newStateDumpCmd() *cobra.Command {
	var serverURL, token string

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Catch up, fold, and print derived state as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := harness.Dial(cmd.Context(), serverURL, token, dumpAfter, "")
			if err != nil {
				return fmt.Errorf("vtt state dump: %w", err)
			}
			defer c.Close()

			head, err := c.CatchUpHead(cmd.Context())
			if err != nil {
				return fmt.Errorf("vtt state dump: catch-up head: %w", err)
			}

			events, log, reached, why := drainToHead(c.Events(), c.CloseErr, dumpAfter, head, dumpQuietWindow, dumpCatchUpTimeout)
			if !reached {
				// FAIL, never print. A short dump that looks complete is the
				// failure this whole path exists to prevent; the caller can
				// retry, and a partial snapshot on stdout could not be told
				// apart from a real one.
				//
				// `why` rather than dumpCatchUpTimeout: this message used to
				// name the 30s backstop unconditionally, including for a
				// connection torn down in under three seconds because THIS
				// process read too slowly. Wrapped with %w so a caller can ask
				// errors.Is(err, harness.ErrEventsOverflow) instead of parsing
				// the sentence.
				return fmt.Errorf("vtt state dump: caught up only to sequence %d of %d — refusing to print a truncated state: %w",
					headSequence(events), head, why)
			}

			st, err := harness.Fold(events)
			if err != nil {
				return fmt.Errorf("vtt state dump: %w", err)
			}

			return writeDump(cmd.OutOrStdout(), st, headSequence(events), log)
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "gateway ws:// URL (required)")
	cmd.Flags().StringVar(&token, "token", "", "invite/session token (required)")
	_ = cmd.MarkFlagRequired("server")
	_ = cmd.MarkFlagRequired("token")

	return cmd
}

// drainToHead reads until it has seen head and then gone quiet for window, or
// until timeout, and returns the entries, the log the first ViewpointChanged
// names (nil when none comes), whether head was reached and why not. A later
// ViewpointChanged ends the read, refused before head (SPEC-007, VTT-423 to
// VTT-425, VTT-435). closeErr may be nil; head <= after is reached at once.
func drainToHead(events <-chan harness.Delivery, closeErr func() error, after, head int64, window, timeout time.Duration) ([]*vttv1.Envelope, *string, bool, error) {
	var out []*vttv1.Envelope
	var log *string
	deadline := time.After(timeout)
	reached := head <= after
	quiet := time.After(window)
	for {
		select {
		case d, ok := <-events:
			if !ok {
				if reached {
					return out, log, true, nil
				}
				return out, log, false, streamClosedReason(closeErr)
			}
			if v := d.Viewpoint; v != nil {
				if log == nil {
					id := v.GetActorId()
					log = &id
					continue
				}
				if reached {
					return out, log, true, nil
				}
				return out, log, false, fmt.Errorf("%w: from %s to %s", errMovedBeforeHead, logOf(*log), logOf(v.GetActorId()))
			}
			env := d.Envelope
			if env == nil {
				continue
			}
			out = append(out, env)
			if env.Sequence >= head {
				reached = true
			}
			quiet = time.After(window)
		case <-quiet:
			if reached {
				return out, log, true, nil
			}
			quiet = time.After(window)
		case <-deadline:
			if reached {
				return out, log, true, nil
			}
			return out, log, false, fmt.Errorf("%w (%s)", errCatchUpDeadline, timeout)
		}
	}
}

func logOf(actorID string) string {
	if actorID == "" {
		return "nobody"
	}
	return actorID + "'s log"
}

// The ways a catch-up read ends short; ask errors.Is, never match the sentence.
var (
	errCatchUpDeadline = errors.New("no envelope reaching head arrived before the catch-up deadline")
	errStreamClosed    = errors.New("the event stream closed")
	errMovedBeforeHead = errors.New("the seat was moved before the catch-up reached its head")
)

// streamClosedReason names why a closed Events() channel ended the read.
//
// The cause is wrapped, not summarised, because it carries the server's own
// words — a clean close arrives here as `received close frame: status =
// StatusNormalClosure and reason = "..."`, and that reason string is the most
// actionable fragment in the message. Per Client.CloseErr's doc, NON-NIL DOES
// NOT MEAN FAILURE: a clean close reports non-nil too, so the useful question
// is errors.Is(err, harness.ErrEventsOverflow), never err != nil.
//
// Multi-%w so BOTH questions stay answerable: errors.Is(_, errStreamClosed)
// for "the stream ended under me" and errors.Is(_, ErrEventsOverflow) for
// "because I was too slow".
func streamClosedReason(closeErr func() error) error {
	if closeErr != nil {
		if err := closeErr(); err != nil {
			return fmt.Errorf("%w: %w", errStreamClosed, err)
		}
	}
	return errStreamClosed
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

// writeDump writes st as indented JSON with "headSequence" beside its fields,
// and "log" unless log is nil (VTT-426). Keep it in step by hand with
// internal/mcp's marshalStateWithHead, and a world seat's dump with
// internal/harness's dumpJSON and client/src/fold.ts's foldToDumpJSON.
func writeDump(w io.Writer, st any, head int64, log *string) error {
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
	if log != nil {
		logRaw, err := json.Marshal(*log)
		if err != nil {
			return fmt.Errorf("vtt state dump: marshal log: %w", err)
		}
		fields["log"] = logRaw
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fields)
}
