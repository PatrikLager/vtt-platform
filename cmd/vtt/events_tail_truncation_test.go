package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// `vtt state dump` refuses to print a truncated state. `vtt events tail` used
// to PRINT one and exit 0.
//
// Both read the same Events() channel, and harness.Client tears that channel
// down when its 256-envelope buffer overflows — which is exactly what happens
// when the tail's own writer is the slow party: pipe it into anything that
// stops reading and fmt.Fprintln blocks on the 64KB pipe buffer while
// envelopes pile up behind it. The stream then ends mid-campaign and the
// command reported success, so a consumer diffing that output against the log
// sees missing events with no signal that anything went wrong.
//
// A clean server close is still exit 0: the tail ran to the end of what there
// was. Only a close that DROPPED events is a failure, which is why this asks
// CloseErr rather than treating every close alike.

func tailCmd(ctx context.Context, out *bytes.Buffer) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetContext(ctx)
	return cmd
}

func TestEventsTailFailsRatherThanSilentlyTruncatingOnOverflow(t *testing.T) {
	ch := make(chan *vttv1.Envelope, 1)
	ch <- envAt(1)
	close(ch) // the teardown an overflow performs

	var out bytes.Buffer
	err := tailUntilDone(tailCmd(context.Background(), &out), ch,
		func() error { return harness.ErrEventsOverflow })

	if err == nil {
		t.Fatal("want an error: the stream dropped events, so this output is incomplete")
	}
	if !errors.Is(err, harness.ErrEventsOverflow) {
		t.Fatalf("want the overflow named so the reader knows WHO was slow, got %v", err)
	}
	// What it did manage to read is still worth printing — the failure is the
	// silent part, not the partial output.
	if !strings.Contains(out.String(), `"sequence":"1"`) {
		t.Fatalf("want the envelopes read before teardown still written, got %q", out.String())
	}
}

func TestEventsTailStopsCleanlyWhenTheServerClosesTheStream(t *testing.T) {
	// The other side: a tail that reached the end of the stream has not
	// truncated anything, and turning every close into a failure would make
	// the command useless for its ordinary job.
	ch := make(chan *vttv1.Envelope, 1)
	ch <- envAt(1)
	close(ch)

	var out bytes.Buffer
	err := tailUntilDone(tailCmd(context.Background(), &out), ch,
		func() error {
			return errors.New(`received close frame: status = StatusNormalClosure and reason = "gateway: done"`)
		})

	if err != nil {
		t.Fatalf("a clean server close is a normal end of tail, got %v", err)
	}
}

func TestEventsTailStopsCleanlyOnContextCancel(t *testing.T) {
	// Ctrl-C is not a truncation either. Pinned because the overflow check
	// sits on the same close path and a careless version would fail here too.
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *vttv1.Envelope)
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() { done <- tailUntilDone(tailCmd(ctx, &out), ch, func() error { return nil }) }()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cancel is a normal stop, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tailUntilDone did not return after cancel")
	}
}

func TestEventsTailToleratesNoCloseErrAccessor(t *testing.T) {
	ch := make(chan *vttv1.Envelope)
	close(ch)
	var out bytes.Buffer
	if err := tailUntilDone(tailCmd(context.Background(), &out), ch, nil); err != nil {
		t.Fatalf("no accessor means no evidence of truncation, got %v", err)
	}
}
