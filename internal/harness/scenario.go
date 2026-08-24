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
//
// Binding assumption: every step's and reconnect's Sequence-based reasoning
// (an accepted command's result.Sequence, a reconnect's AfterSequence) is
// ABSOLUTE, counted from a FRESH campaign with no prior history — the
// format has no way to express an assertion relative to whatever the
// catch-up cursor happens to already be. RunScenario enforces this at
// runtime (returns a clear framework error if the initial catch-up finds
// any pre-existing events — see engine.go's errFreshCampaignRequired) rather
// than letting a non-fresh campaign silently misbehave. Relative-sequence
// scenarios are a planned format extension, not implemented here.
type Scenario struct {
	Name         string        `json:"name"`
	Participants []Participant `json:"participants"`
	Steps        []Step        `json:"steps"`
	Probes       []Probe       `json:"probes"`
	// Ruleset is OPTIONAL (ruleset-interpreter Task 6): a bare ruleset id
	// (e.g. "tavern-brawl"), never a path — resolution to an actual
	// directory is a caller concern (cmd/vtt's self-contained boot glue
	// resolves it relative to the repository root; see
	// resolveRulesetDir's doc comment there). Empty means this scenario
	// exercises no ruleset-dependent commands (use_ability/
	// remove_condition), the overwhelmingly common case for the
	// pre-existing library — those scenarios never set this field.
	Ruleset string `json:"ruleset,omitempty"`
	// Adventures is OPTIONAL (adventure-format Task 4): a bare directory
	// path relative to the REPOSITORY ROOT (e.g. "adventures" — the repo's
	// own top-level adventures/ directory, each of whose subdirectories is
	// one loadable adventure), mirroring Ruleset's own repo-root-relative
	// resolution (cmd/vtt's self-contained boot glue resolves it the same
	// way; see resolveAdventuresDir's doc comment there). Unlike Ruleset
	// (a bare id further joined under rulesets/<id>), this field is already
	// the directory itself — --adventures-dir's own shape — so no further
	// joining happens beyond the repo-root prefix. Empty means this
	// scenario exercises no load_adventure command, the common case for the
	// pre-existing library.
	Adventures string `json:"adventures,omitempty"`
}

// Participant declares one connection the engine dials at scenario start
// (after=0): a display Name (Step.By and reconnect steps reference it) and an
// authz Role. Nothing else — a participant IS a connection at a role here.
//
// There is no "controls" key (2026-08-24). One existed, was described in this
// comment as "informational at this layer", and was informational everywhere
// else too: it reached identity.CreateInvite and stopped. Control in a
// scenario has always come from its STEPS — which is why removing the key from
// all six scenarios that declared it left every golden stream byte-identical.
//
// Those steps are now exactly ONE command: grant_actor_control. An add_actor
// carrying a controllerId was the other route until later the same day, and it
// is refused at the command boundary and in the fold alike (visibility spec
// §5.1) — so a scenario that wants a player to hold a character writes two
// steps, and the second one says what the character is.
type Participant struct {
	Name string `json:"name"`
	Role string `json:"role"`
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

// Probe is exactly one of the six probe kinds, evaluated against
// Fold(participant 0's observed events) once every step has run.
// ResourceAt/HasCondition were added in ruleset-interpreter Task 6, so
// scenarios (e.g. scenarios/toy-brawl.json) can assert the concrete,
// structural result of a use_ability/remove_condition run — a resource's
// final current value, or whether a condition is present on an actor —
// without needing exact-dice matching (the harness cannot predict a live
// crypto Roller's draws; see resolve.go's Roller doc comment). NoteAt was
// added in world-layer Task 3, so scenarios (e.g. scenarios/story-table.json)
// can assert a world note's current key/title/text after upsert/replace/
// delete — the SAME "assert structural state, not exact wire bytes" shape.
type Probe struct {
	TokenAt      *TokenAtProbe      `json:"tokenAt,omitempty"`
	SessionCount *SessionCountProbe `json:"sessionCount,omitempty"`
	ActorExists  *ActorExistsProbe  `json:"actorExists,omitempty"`
	ResourceAt   *ResourceAtProbe   `json:"resourceAt,omitempty"`
	HasCondition *HasConditionProbe `json:"hasCondition,omitempty"`
	NoteAt       *NoteAtProbe       `json:"noteAt,omitempty"`
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

// ResourceAtProbe asserts a named resource's CURRENT value on an actor
// (ruleset-interpreter Task 6) — e.g. proving a use_ability hit's
// resource_change outcome actually landed, deterministically, without
// needing to predict a live crypto Roller's draws.
type ResourceAtProbe struct {
	ActorId  string `json:"actorId"`
	Resource string `json:"resource"`
	Value    int32  `json:"value"`
}

// HasConditionProbe asserts whether a named condition is present (or
// absent, when Present is false) on an actor (ruleset-interpreter Task 6).
type HasConditionProbe struct {
	ActorId     string `json:"actorId"`
	ConditionId string `json:"conditionId"`
	Present     bool   `json:"present"`
}

// NoteAtProbe asserts a world note's current state by key (world-layer Task
// 3): Key must be present (a probe against an absent/deleted key always
// fails — there is no Present-style "assert absent" flag here, since
// deleteNote's own denial/rejection posture and the story-table library
// scenario's "deleted-absent rejection" step already cover the absence
// case at the command layer). TitleIs and TextContains are BOTH optional
// and independently checked ONLY when non-empty: a probe that sets neither
// asserts presence alone; a probe that sets one or both narrows the
// assertion to that note's exact Title (TitleIs) and/or a substring of its
// Text (TextContains, for asserting content without pinning it verbatim —
// same shape as Expect.DeniedContaining's substring match).
type NoteAtProbe struct {
	Key          string `json:"key"`
	TitleIs      string `json:"titleIs,omitempty"`
	TextContains string `json:"textContains,omitempty"`
}

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
		Ruleset      string            `json:"ruleset,omitempty"`
		Adventures   string            `json:"adventures,omitempty"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse scenario: %w", err)
	}

	sc := &Scenario{Name: raw.Name, Participants: raw.Participants, Probes: raw.Probes, Ruleset: raw.Ruleset, Adventures: raw.Adventures}
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
	// The converse ambiguity (P6 final review): Expect.OK == true WITH a
	// non-empty DeniedContaining ALSO isn't either valid shape on its own
	// terms — it sets both at once. engine.go's runCommandStep treats
	// DeniedContaining != "" as authoritative when present, so this shape
	// loads fine and silently runs as a denial expectation today, quietly
	// discarding the author's "ok": true. Reject it at load time for the
	// same reason as the neither-set case above.
	if hasCommand && st.Expect.OK && st.Expect.DeniedContaining != "" {
		return errors.New(`expect must be either {"ok": true} or {"deniedContaining": "<substring>"}, got both`)
	}
	return nil
}

// validateProbe enforces Probe's exactly-one-of-six-kinds contract.
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
	if p.ResourceAt != nil {
		n++
	}
	if p.HasCondition != nil {
		n++
	}
	if p.NoteAt != nil {
		n++
	}
	if n != 1 {
		return fmt.Errorf("exactly one of tokenAt/sessionCount/actorExists/resourceAt/hasCondition/noteAt required, got %d", n)
	}
	return nil
}
