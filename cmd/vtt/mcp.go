package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/PatrikLager/vtt-platform/internal/mcp"
)

// toolsJSON is a COMMITTED COPY of contract/gen/tools/tools.json (Taskfile
// generate:contract's `cp` line keeps it byte-identical; check:drift fails
// the gate the moment it ever diverges — plan Task 3's binding tools.json
// delivery decision). GENERATED — do not hand-edit cmd/vtt/tools.json
// itself (it is plain JSON and cannot carry its own "generated" header
// without breaking that byte-identical copy, and check:drift's
// regenerate-and-diff would immediately revert it anyway); edit
// tools/toolgen's manifest source and regenerate instead.
//
// go:embed cannot reach outside this package's own directory (the reason
// tools.json is copied here at all rather than embedded straight from
// contract/gen/tools/), and a runtime file read would break vtt's
// single-binary distribution — internal/mcp.Config.ToolsJSON takes raw
// bytes precisely so this is the ONLY place cmd/vtt touches the
// filesystem for it.
//
//go:embed tools.json
var toolsJSON []byte

// newMCPCmd is `vtt mcp`: an MCP server that seats an LLM at the table as
// the agent participant (docs/superpowers/specs/2026-07-24-mcp-gateway-
// design.md). It serves over stdio (the standard MCP host transport, e.g.
// Claude Code's `.mcp.json`) for the process lifetime; all session
// lifecycle, reconnect, and generic tool dispatch live in internal/mcp —
// this command only resolves its two inputs and wires them together.
func newMCPCmd() *cobra.Command {
	var serverURL, token string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve an MCP server over stdio, seating an LLM as the agent participant",
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, err := resolveMCPToken(token)
			if err != nil {
				return err
			}
			srv, err := mcp.New(mcp.Config{WSURL: serverURL, Token: tok, ToolsJSON: toolsJSON})
			if err != nil {
				return fmt.Errorf("vtt mcp: %w", err)
			}
			runErr := srv.Run(cmd.Context(), &mcpsdk.StdioTransport{})
			if errors.Is(runErr, context.Canceled) {
				// SIGINT/SIGTERM (main.go wires both into cmd.Context() via
				// signal.NotifyContext): the SDK's own Server.Run returns
				// ctx.Err() verbatim on cancellation (context.Canceled),
				// which is a clean, expected stop here — not a failure —
				// exactly the exit-code parity `vtt serve` (serve.go's
				// RunE) and `vtt events tail` (events_tail.go's
				// tailUntilDone) already give every other cancelable
				// command. Propagating it unfiltered would exit 1 on an
				// ordinary Ctrl-C, which main.go's os.Exit(1) path treats
				// as a real error.
				return nil
			}
			return runErr
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "gateway ws:// URL (required)")
	cmd.Flags().StringVar(&token, "token", "", "agent invite/session token (env VTT_TOKEN honored; flag wins)")
	_ = cmd.MarkFlagRequired("server")

	return cmd
}

// resolveMCPToken applies plan Task 3's binding precedence: an explicit
// --token flag always wins; otherwise the VTT_TOKEN environment variable is
// honored (the README's `.mcp.json` snippet uses this form so the token
// never sits in a config file or shell history — spec §6); if neither is
// set, a clear, immediate error names both ways to supply one, rather than
// surfacing as an opaque downstream harness.Dial failure with no token in
// sight.
func resolveMCPToken(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if env := os.Getenv("VTT_TOKEN"); env != "" {
		return env, nil
	}
	return "", errors.New("vtt mcp: a token is required: pass --token or set VTT_TOKEN")
}
