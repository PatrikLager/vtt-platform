package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// errScenarioFailed is client run's exit-1 signal: at least one step or
// probe in the Report did not pass. It carries no extra text — the report
// (human log, or --json's Report body) is what names which step, printed
// to stdout BEFORE this error is returned, never folded into the error
// string itself.
var errScenarioFailed = errors.New("vtt client run: scenario failed")

// newClientCmd is the `vtt client` command group: `run` today, `soak`
// joins it in a later task (task-3-brief.md / plan Task 5).
func newClientCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Drive declarative scenarios against a gateway",
	}
	cmd.AddCommand(newClientRunCmd())
	return cmd
}

// newClientRunCmd runs a scenario file: self-contained by default (boots a
// throwaway server via harness_boot.go's bootSelfContained), or against a
// live server when --server/--tokens are both given. See
// docs/superpowers/specs/2026-07-24-simulation-harness-design.md §5.
func newClientRunCmd() *cobra.Command {
	var serverURL, tokensPath string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "run <scenario.json>",
		Short: "Run a scenario, self-contained by default or against a live server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := harness.LoadScenario(args[0])
			if err != nil {
				return err
			}

			dial, teardown, err := buildDialer(sc, serverURL, tokensPath)
			if err != nil {
				return err
			}
			defer teardown()

			progress := cmd.OutOrStdout()
			if jsonOut {
				progress = io.Discard
			}
			rep, err := harness.RunScenario(cmd.Context(), sc, dial, progress)
			if err != nil {
				return fmt.Errorf("vtt client run: %w", err)
			}

			if jsonOut {
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(rep); err != nil {
					return fmt.Errorf("vtt client run: encode report: %w", err)
				}
			}
			if !rep.Pass {
				return errScenarioFailed
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&serverURL, "server", "", "live gateway ws:// URL (omit for self-contained)")
	cmd.Flags().StringVar(&tokensPath, "tokens", "", `tokens.json for live mode: {"participants": {"<name>": "<token>"}}`)
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a machine-readable JSON report instead of the human step log")

	return cmd
}

// tokensFile is tokens.json's shape (spec §10, decided at plan time):
// {"participants": {"<name>": "<token>"}}.
type tokensFile struct {
	Participants map[string]string `json:"participants"`
}

// buildDialer picks self-contained or live mode from which flags were
// given, and returns a harness.Dialer plus a teardown func the caller must
// always call exactly once (a no-op in live mode, where this process
// didn't start the server it's dialing).
func buildDialer(sc *harness.Scenario, serverURL, tokensPath string) (harness.Dialer, func() error, error) {
	if serverURL == "" && tokensPath == "" {
		boot, err := bootSelfContained(sc)
		if err != nil {
			return nil, nil, err
		}
		return dialerFor(boot.WSURL, boot.Tokens), boot.close, nil
	}
	if serverURL == "" || tokensPath == "" {
		return nil, nil, errors.New("vtt client run: --server and --tokens must be given together")
	}

	data, err := os.ReadFile(tokensPath)
	if err != nil {
		return nil, nil, fmt.Errorf("vtt client run: read tokens file: %w", err)
	}
	var tf tokensFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, nil, fmt.Errorf("vtt client run: parse tokens file %s: %w", tokensPath, err)
	}
	return dialerFor(serverURL, tf.Participants), func() error { return nil }, nil
}

// dialerFor adapts a fixed ws URL and a name->token map into a
// harness.Dialer: the same shape RunScenario calls at scenario start and
// on every reconnect step, regardless of which mode built it.
func dialerFor(wsURL string, tokens map[string]string) harness.Dialer {
	return func(name string, after int64) (harness.Conn, error) {
		token, ok := tokens[name]
		if !ok {
			return nil, fmt.Errorf("vtt client run: no token for participant %q", name)
		}
		return harness.Dial(context.Background(), wsURL, token, after)
	}
}
