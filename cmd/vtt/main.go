// Command vtt is the campaign server and admin CLI: `serve` runs the
// WebSocket/HTTP gateway over one campaign, `invite`/`revoke` manage
// participant identity, `version` reports the build. ADR-008 records the
// shell's design: cobra, ultra-thin commands whose RunE delegates directly
// to internal/ (pattern borrowed from Peiman's ckeletin-go; its updateable
// framework layer is deliberately not used — see the ADR).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// newRootCmd assembles the `vtt` command tree. Kept separate from main() so
// tests can drive it in-process (SetArgs/SetOut/Execute) without exec-ing
// the built binary.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "vtt",
		Short:         "vtt is the campaign server and admin CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newServeCmd(), newInviteCmd(), newRevokeCmd(), newVersionCmd(),
		newClientCmd(), newEventsCmd(), newStateCmd())
	return root
}

// main wires SIGINT/SIGTERM into the root command's context: a command
// whose RunE watches cmd.Context().Done() (today, only `vtt events tail` —
// see its doc comment) gets a chance to unwind cleanly (close its
// WebSocket, run its own deferred cleanup) instead of being torn down by
// the signal's OS-default action. stop() un-registers the handler once
// Execute returns, so a caller that ignores a first Ctrl-C and sends a
// second one gets the OS-default (immediate kill) rather than a stuck
// process — signal.NotifyContext's own documented behavior.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
