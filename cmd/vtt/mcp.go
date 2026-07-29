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
	"github.com/PatrikLager/vtt-platform/internal/rules"
)

// errMCPAdventuresRequireRuleset is newMCPCmd's boot-time flag error when
// --adventures-dir is given without --ruleset (adventure-format Task 4,
// spec §7 binding: "MCP flag precedent" — require both, documented here).
// get_adventure_guide itself needs no ruleset (it serves guide.md text
// verbatim, untouched by any rule vocabulary), but LOADING an adventures
// directory does: adventure.Load validates every adventure's statblocks
// against a *rules.Ruleset, and there is no "the served ruleset" to
// validate against without --ruleset. Mirrors errAdventuresRequireRuleset's
// exact reasoning (serve_compose.go) for `vtt serve`'s own pairing of the
// same two flags.
const errMCPAdventuresRequireRuleset = "vtt mcp: --adventures-dir requires --ruleset (adventures load+validate against it)"

// toolsJSON is a COMMITTED COPY of contract/gen/tools/tools.json (Taskfile
// generate:contract's `cp` line keeps it byte-identical; check:drift fails
// the gate the moment it ever diverges — plan Task 3's binding tools.json
// delivery decision). GENERATED — do not hand-edit cmd/vtt/tools.json
// itself (it is plain JSON and cannot carry its own "generated" header
// without breaking that byte-identical copy, and check:drift's
// regenerate-and-diff would immediately revert it anyway); edit
// tools/toolgen's manifest source and regenerate instead.
//
// The embed directive cannot reach outside its own package directory (the
// reason tools.json is copied here at all, rather than embedded straight
// from contract/gen/tools/), and a runtime file read would break vtt's
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
	var serverURL, token, rulesetDir, adventuresDir string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve an MCP server over stdio, seating an LLM as the agent participant",
		RunE: func(cmd *cobra.Command, args []string) error {
			tok, err := resolveMCPToken(token)
			if err != nil {
				return err
			}

			if adventuresDir != "" && rulesetDir == "" {
				return errors.New(errMCPAdventuresRequireRuleset)
			}

			// --ruleset is OPTIONAL (ruleset-interpreter Task 6, controller
			// decision: no wire contract addition — see the task report).
			// A bad directory fails loud here, at boot, exactly like `vtt
			// serve --ruleset` (serve_compose.go's composeServer): this
			// package never hands internal/mcp a directory path, only the
			// already-validated guide TEXT (mcp.Config.RulesetGuide's own
			// doc comment) — internal/mcp may not import internal/rules at
			// all (.go-arch-lint.yml's P1 boundary).
			guide := ""
			var rs *rules.Ruleset
			if rulesetDir != "" {
				rs, err = rules.Load(rulesetDir)
				if err != nil {
					return fmt.Errorf("vtt mcp: load ruleset %s: %w", rulesetDir, err)
				}
				guide = rs.Guide
			}

			// --adventures-dir is OPTIONAL (adventure-format Task 4): same
			// fail-loud-at-boot posture, and the same P1 boundary as
			// --ruleset above — this package hands internal/mcp only the
			// already-loaded, already-validated guide TEXT per adventure id
			// (mcp.Config.AdventureGuides), never a directory path.
			// Required together with --ruleset (checked above): every
			// adventure declares the ruleset id it was written for, and
			// adventure.Load needs an already-loaded *rules.Ruleset to
			// validate against.
			var adventureGuides map[string]string
			if adventuresDir != "" {
				advs, err := loadAdventuresDir(adventuresDir, rs)
				if err != nil {
					return fmt.Errorf("vtt mcp: load adventures %s: %w", adventuresDir, err)
				}
				adventureGuides, err = loadAdventureGuides(advs)
				if err != nil {
					return fmt.Errorf("vtt mcp: %w", err)
				}
			}

			srv, err := mcp.New(mcp.Config{
				WSURL: serverURL, Token: tok, ToolsJSON: toolsJSON,
				RulesetGuide: guide, AdventureGuides: adventureGuides,
			})
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
	cmd.Flags().StringVar(&rulesetDir, "ruleset", "", "path to a ruleset directory (optional; enables get_ruleset_guide's guide.md content)")
	cmd.Flags().StringVar(&adventuresDir, "adventures-dir", "", "path to a directory of adventure subdirectories (optional; enables get_adventure_guide's guide.md content — requires --ruleset)")
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
