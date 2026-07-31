package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/PatrikLager/vtt-platform/internal/harness"
)

// errSoakFailed is `vtt client soak`'s exit-1 signal: at least one action or
// checkpoint in the SoakReport did not pass — mirrors client_run.go's
// errScenarioFailed (the report itself, human log or --json's Report body,
// is what names which action/checkpoint, printed to stdout BEFORE this
// error is returned).
var errSoakFailed = errors.New("vtt client soak: soak failed")

// newClientSoakCmd runs the wire-level soak (docs/superpowers/specs/
// 2026-07-24-simulation-harness-design.md §5): self-contained by default
// (boots a throwaway server via harness_boot.go's bootSelfContained, minting
// invites for harness.SoakParticipants()'s fixed four-role roster), or
// against a live server when --server/--tokens are both given — the exact
// same two-mode shape newClientRunCmd already establishes.
func newClientSoakCmd() *cobra.Command {
	var serverURL, tokensPath string
	var seed int64
	var events, checkEvery int
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "soak",
		Short: "Run the wire-level soak: a seeded, mixed, authorized-by-construction command sequence",
		RunE: func(cmd *cobra.Command, args []string) error {
			dial, ids, teardown, err := buildSoakDialer(serverURL, tokensPath)
			if err != nil {
				return err
			}
			defer func() {
				if tErr := teardown(); tErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "vtt client soak: teardown: %v\n", tErr)
				}
			}()

			// --json still CAPTURES the human log rather than discarding it,
			// so a failing run can carry the reason in Report. Discarding it
			// left CI with "Pass":false and no way to tell a fold-equality
			// break from a caught-up-in-time timeout.
			progress := cmd.OutOrStdout()
			var captured bytes.Buffer
			if jsonOut {
				progress = &captured
			}
			rep, err := harness.RunSoak(cmd.Context(), harness.SoakConfig{
				Seed: seed, Events: events, CheckEvery: checkEvery, IDs: ids,
			}, dial, progress)
			if err != nil {
				return fmt.Errorf("vtt client soak: %w", err)
			}

			if jsonOut {
				if !rep.Pass {
					rep.Report = captured.String()
				}
				if err := json.NewEncoder(cmd.OutOrStdout()).Encode(rep); err != nil {
					return fmt.Errorf("vtt client soak: encode report: %w", err)
				}
			}
			if !rep.Pass {
				return errSoakFailed
			}
			return nil
		},
	}

	cmd.Flags().Int64Var(&seed, "seed", 0, "deterministic generator seed (required)")
	cmd.Flags().IntVar(&events, "events", 0, "number of actions to issue (required)")
	cmd.Flags().IntVar(&checkEvery, "check-every", 0, "checkpoint every N accepted actions (default 100)")
	cmd.Flags().StringVar(&serverURL, "server", "", "live gateway ws:// URL (omit for self-contained)")
	cmd.Flags().StringVar(&tokensPath, "tokens", "", `tokens.json for live mode: {"participants": {"<name>": "<token>"}, "ids": {"<name>": "<id>"}}`)
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit a machine-readable JSON report instead of the human action log")
	_ = cmd.MarkFlagRequired("seed")
	_ = cmd.MarkFlagRequired("events")

	return cmd
}

// buildSoakDialer picks self-contained or live mode from which flags were
// given — client_run.go's buildDialer, adapted for soak's FIXED participant
// roster (harness.SoakParticipants(), rather than a *harness.Scenario's own
// declared list): self-contained mode wraps that roster in a throwaway
// *harness.Scenario purely so it can reuse bootSelfContained/mintInvites
// unchanged (harness_boot.go — the SAME boot glue `vtt client run` uses),
// keeping the "how to mint invites for a set of named participants" logic
// defined in exactly one place. Returns the dialer, the participant-id map
// harness.RunSoak's SoakConfig.IDs needs (nil in live mode when tokens.json
// carries no "ids" — see client_run.go's tokensFile doc comment), and a
// teardown func the caller must always call exactly once.
func buildSoakDialer(serverURL, tokensPath string) (harness.Dialer, map[string]string, func() error, error) {
	if serverURL == "" && tokensPath == "" {
		sc := &harness.Scenario{Participants: harness.SoakParticipants()}
		boot, err := bootSelfContained(sc)
		if err != nil {
			return nil, nil, nil, err
		}
		return dialerFor(boot.WSURL, boot.Tokens), boot.IDs, boot.close, nil
	}
	if serverURL == "" || tokensPath == "" {
		return nil, nil, nil, errors.New("vtt client soak: --server and --tokens must be given together")
	}

	data, err := os.ReadFile(tokensPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("vtt client soak: read tokens file: %w", err)
	}
	var tf tokensFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, nil, nil, fmt.Errorf("vtt client soak: parse tokens file %s: %w", tokensPath, err)
	}
	return dialerFor(serverURL, tf.Participants), tf.IDs, func() error { return nil }, nil
}
