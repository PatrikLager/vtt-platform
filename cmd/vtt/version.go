package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the vtt binary's version string. Overridable at build time via
// -ldflags "-X main.Version=...". No release process exists yet (spec §6,
// YAGNI), so "dev" is the honest default until one does.
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the vtt version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "vtt version %s\n", Version)
			return nil
		},
	}
}
