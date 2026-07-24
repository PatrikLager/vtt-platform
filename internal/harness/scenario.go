// Scenario types and strict loading live in this file; execution is
// engine.go, client-side state derivation is fold.go. See the package
// comment in client.go for the P1 import boundary this whole package
// (including this file) is bound by.
package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Scenario is a declarative, LLM-authorable script: participants, an
// ordered sequence of steps (each either a command with an expectation, or a
// reconnect), and final-state probes evaluated once every step has run
// (docs/superpowers/specs/2026-07-24-simulation-harness-design.md §4).
type Scenario struct {
	Name         string        `json:"name"`
	Participants []Participant `json:"participants"`
	Steps        []Step        `json:"steps"`
	Probes       []Probe       `json:"probes"`
}

// Participant declares one connection the engine dials at scenario start
// (after=0): a display Name (Step.By and reconnect steps reference it),
// authz Role, and — for player-role participants — the actor ids Controls
// lists (informational at this layer; authz enforcement lives server-side).
type Participant struct {
	Name     string   `json:"name"`
	Role     string   `json:"role"`
	Controls []string `json:"controls,omitempty"`
}

// Step is exactly one of a command (with its expectation) or a reconnect —
// LoadScenario enforces that exclusivity; RunScenario trusts it.
type Step struct {
	By string `json:"by"`
	// Command is protojson of the contract's vttv1.ClientCommand oneof,
	// parsed at execution time (not at load time — see LoadScenario's doc
	// comment on why validation stops at structural checks here).
	Command   json.RawMessage `json:"command,omitempty"`
	Expect    *Expect         `json:"expect,omitempty"`
	Reconnect *ReconnectSpec  `json:"reconnect,omitempty"`
}

// Expect describes a command step's required result: EITHER an accepted
// command ({"ok": true}) OR a denial whose error contains DeniedContaining
// ({"deniedContaining": "<substring>"}) — never both meaningfully at once
// (the engine treats DeniedContaining != "" as authoritative; see engine.go).
// Exactly one of those two shapes is valid; LoadScenario rejects any other
// combination (e.g. {"ok": false} alone, or {}) at load time rather than
// letting it silently run as an ok-expectation.
type Expect struct {
	OK               bool   `json:"ok"`
	DeniedContaining string `json:"deniedContaining"`
}

// ReconnectSpec is a reconnect step's payload: drop the participant's
// current connection and redial with after=AfterSequence.
type ReconnectSpec struct {
	AfterSequence int64 `json:"afterSequence"`
}

// Probe is exactly one of the three v1 probe kinds, evaluated against
// Fold(participant 0's observed events) once every step has run.
type Probe struct {
	TokenAt      *TokenAtProbe      `json:"tokenAt,omitempty"`
	SessionCount *SessionCountProbe `json:"sessionCount,omitempty"`
	ActorExists  *ActorExistsProbe  `json:"actorExists,omitempty"`
}

// TokenAtProbe asserts a token exists at exactly (X, Y).
type TokenAtProbe struct {
	TokenId string `json:"tokenId"`
	X       int32  `json:"x"`
	Y       int32  `json:"y"`
}

// SessionCountProbe asserts the folded state's open and total session counts.
type SessionCountProbe struct {
	Open  int `json:"open"`
	Total int `json:"total"`
}

// ActorExistsProbe asserts an actor with this id exists.
type ActorExistsProbe struct {
	ActorId string `json:"actorId"`
}

// errUnimplemented is the ADR-009 Step-1 stub sentinel (task-2-brief.md):
// every not-yet-implemented function in this package returns it, so tests
// written against the stub fail on a real runtime assertion (behavioral
// RED), never a missing-symbol compile error.
var errUnimplemented = errors.New("harness: unimplemented")

// LoadScenario reads and strictly parses a scenario file: unknown JSON
// fields are errors (json.Decoder.DisallowUnknownFields), and every step is
// decoded and validated individually so failures name the offending step
// index (task-2-brief.md's binding requirement) — e.g. "harness: scenario
// step 2: json: unknown field \"bogus\"", or "harness: scenario step 0:
// expect is required when command is set".
//
// Validation here is deliberately structural only (exactly one of
// Command/Reconnect per step; Expect required with Command; exactly one
// probe kind per Probe) — it does NOT protojson-parse Command's contents;
// that happens in the engine at execution time (RunScenario), where a
// malformed ClientCommand surfaces as that step's own failure rather than a
// load-time error, keeping the two concerns (shape vs. wire semantics)
// separate.
func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("harness: load scenario %s: %w", path, err)
	}
	sc, err := parseScenario(data)
	if err != nil {
		return nil, fmt.Errorf("harness: load scenario %s: %w", path, err)
	}
	return sc, nil
}

// parseScenario is LoadScenario's testable core (path I/O split out so a
// future caller — or a test — can feed bytes directly).
func parseScenario(data []byte) (*Scenario, error) {
	// The top-level decode captures Steps as raw JSON per-element (rather
	// than directly into []Step) so each step can be re-decoded and
	// validated individually below — the only way a strict-decode error can
	// name WHICH step it came from, since encoding/json's own errors carry
	// no path/index information for elements inside a slice.
	var raw struct {
		Name         string            `json:"name"`
		Participants []Participant     `json:"participants"`
		Steps        []json.RawMessage `json:"steps"`
		Probes       []Probe           `json:"probes"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse scenario: %w", err)
	}

	sc := &Scenario{Name: raw.Name, Participants: raw.Participants, Probes: raw.Probes}
	sc.Steps = make([]Step, len(raw.Steps))
	for i, rawStep := range raw.Steps {
		var st Step
		stepDec := json.NewDecoder(bytes.NewReader(rawStep))
		stepDec.DisallowUnknownFields()
		if err := stepDec.Decode(&st); err != nil {
			return nil, fmt.Errorf("scenario step %d: %w", i, err)
		}
		if err := validateStep(st); err != nil {
			return nil, fmt.Errorf("scenario step %d: %w", i, err)
		}
		sc.Steps[i] = st
	}

	for i, p := range sc.Probes {
		if err := validateProbe(p); err != nil {
			return nil, fmt.Errorf("scenario probe %d: %w", i, err)
		}
	}

	return sc, nil
}

// validateStep enforces Step's exactly-one-of-Command/Reconnect contract,
// that Expect is present whenever Command is set, and that a present Expect
// is itself coherent (see the check below).
func validateStep(st Step) error {
	hasCommand := len(st.Command) > 0
	hasReconnect := st.Reconnect != nil
	switch {
	case hasCommand && hasReconnect:
		return errors.New("exactly one of command or reconnect required, got both")
	case !hasCommand && !hasReconnect:
		return errors.New("exactly one of command or reconnect required, got neither")
	case hasCommand && st.Expect == nil:
		return errors.New("expect is required when command is set")
	}
	if st.By == "" {
		return errors.New(`"by" is required`)
	}
	// Coherence check (fast-follow, reviewed): Expect.OK == false with an
	// empty DeniedContaining is neither valid shape — {"ok": true} or
	// {"deniedContaining": "<substring>"} — and today would silently run as
	// an ok-expectation, the opposite of what an author writing "ok": false
	// (or omitting expect's contents entirely) almost certainly intended.
	// Reject it at load time instead of letting it through as a trap for
	// the LLM-authorable format.
	if hasCommand && !st.Expect.OK && st.Expect.DeniedContaining == "" {
		return errors.New(`expect must be either {"ok": true} or {"deniedContaining": "<substring>"}, got neither`)
	}
	return nil
}

// validateProbe enforces Probe's exactly-one-of-three-kinds contract.
func validateProbe(p Probe) error {
	n := 0
	if p.TokenAt != nil {
		n++
	}
	if p.SessionCount != nil {
		n++
	}
	if p.ActorExists != nil {
		n++
	}
	if n != 1 {
		return fmt.Errorf("exactly one of tokenAt/sessionCount/actorExists required, got %d", n)
	}
	return nil
}
