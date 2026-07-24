// Command vtt is the campaign server and admin CLI: `serve` runs the
// WebSocket/HTTP gateway over one campaign, `invite`/`revoke` manage
// participant identity, `version` reports the build. ADR-008 records the
// shell's design: cobra, ultra-thin commands whose RunE delegates directly
// to internal/ (pattern borrowed from Peiman's ckeletin-go; its updateable
// framework layer is deliberately not used — see the ADR).
package main

import (
	"fmt"
	"os"

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
	root.AddCommand(newServeCmd(), newInviteCmd(), newRevokeCmd(), newVersionCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
