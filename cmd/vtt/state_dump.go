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

// newStateDumpCmd catches up from sequence 0, folds (harness.Fold), and
// prints the derived state as JSON — a point-in-time snapshot, not a live
// guarantee (dumpQuietWindow's doc comment). The printed JSON carries one
// field beyond the folded state itself: "headSequence", the highest
// envelope sequence this dump actually received before folding — the
// caller's staleness check (e.g. sub-project 6's LLM tools comparing it
// against a sequence they already know about).
// dumpAfter is the catch-up cursor `vtt state dump` always dials with: a dump
// is a full snapshot from the beginning of the log, never a tail. Named rather
// than written as a bare 0 at both call sites because drainToHead's
// already-caught-up test is `head <= after` — the two have to agree, and a
// literal in two places is how they would stop agreeing.
const dumpAfter = 0

func newStateDumpCmd() *cobra.Command {
	var serverURL, token string

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Catch up, fold, and print derived state as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := harness.Dial(cmd.Context(), serverURL, token, dumpAfter)
			if err != nil {
				return fmt.Errorf("vtt state dump: %w", err)
			}
			defer c.Close()

			head, err := c.CatchUpHead(cmd.Context())
			if err != nil {
				return fmt.Errorf("vtt state dump: catch-up head: %w", err)
			}

			events, reached, why := drainToHead(c.Events(), c.CloseErr, dumpAfter, head, dumpQuietWindow, dumpCatchUpTimeout)
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
// bool reports whether head was reached; the error, when it was not, reports
// WHY — and closeErr is what makes that answerable.
//
// A short read has two causes that call for OPPOSITE responses. The deadline
// expiring means the SERVER is slow or the head never came. The channel
// closing early means harness.Client tore itself down because THIS process
// could not drain Events() fast enough (ErrEventsOverflow at 256 buffered).
// A closed channel looks identical either way, so this used to return the same
// (events, false) for both and the caller blamed dumpCatchUpTimeout for every
// short read — printing a 30s timeout for a connection that died in under
// three seconds, and sending the next reader after gateway latency when the
// fix was on the reading side. The same misdiagnosis cost a CI investigation
// in internal/harness/soak.go before drainFreshCatchUp started asking.
//
// closeErr may be nil for callers holding a raw channel with no connection to
// ask; they get errStreamClosed without a cause, which is still not the clock.
//
// Both conditions matter. Stopping at head alone would drop envelopes
// broadcast live just behind it, which is a truncation from the other side;
// stopping at quiet alone is the bug this replaced.
//
// `head <= after` means there was never anything to catch up to, so the quiet
// window is the only sensible signal — the old behaviour, correctly scoped to
// the case where it was never a guess. Note this is NOT just head == 0:
// Store.Subscribe answers with the cursor ITSELF when nothing newer exists, so
// a connection dialed at after=5 against a 5-event log is told head=5 and
// would otherwise wait out the whole timeout for a Sequence >= 5 that cannot
// arrive — everything at or below the cursor was excluded from its backlog by
// definition.
//
// Deliberately a sibling of internal/harness/soak.go's drainToSequence rather
// than an import: cmd/vtt depends on harness, never the reverse.
func drainToHead(events <-chan *vttv1.Envelope, closeErr func() error, after, head int64, window, timeout time.Duration) ([]*vttv1.Envelope, bool, error) {
	var out []*vttv1.Envelope
	deadline := time.After(timeout)
	reached := head <= after
	for {
		select {
		case env, ok := <-events:
			if !ok {
				if reached {
					return out, true, nil
				}
				return out, false, streamClosedReason(closeErr)
			}
			out = append(out, env)
			if env.Sequence >= head {
				reached = true
			}
		case <-time.After(window):
			if reached {
				return out, true, nil
			}
		case <-deadline:
			if reached {
				return out, true, nil
			}
			return out, false, fmt.Errorf("%w (%s)", errCatchUpDeadline, timeout)
		}
	}
}

// The two ways a catch-up read can end short. Both are sentinels so a caller
// — or a test — can ask errors.Is instead of matching on the sentence: the
// overflow side already had that lever (harness.ErrEventsOverflow) and the
// timeout side did not, which made the only available assertion a
// strings.Contains on wording that any rephrasing would break.
var (
	errCatchUpDeadline = errors.New("no envelope reaching head arrived before the catch-up deadline")
	errStreamClosed    = errors.New("the event stream closed")
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
