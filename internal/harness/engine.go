package harness

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// Conn is the wire-connection surface the scenario engine drives. *Client
// (client.go) satisfies it in production; this package's own tests drive a
// scripted fake — the engine never depends on anything beyond this
// interface plus the contract, so it is transparent to which one it holds.
type Conn interface {
	SendCommand(ctx context.Context, cmd *vttv1.ClientCommand) (*vttv1.CommandResult, error)
	Events() <-chan *vttv1.Envelope
	Close() error
}

// Compile-time proof that *Client actually satisfies Conn (task-2-brief.md
// self-review requirement) — a signature drift in either type breaks the
// build here, not at some later call site.
var _ Conn = (*Client)(nil)

// Dialer connects (or reconnects) a named participant at a given catch-up
// cursor. RunScenario calls it once per declared participant at scenario
// start (after=0), and again for every reconnect step (after=AfterSequence).
type Dialer func(name string, after int64) (Conn, error)

// denialAbsenceWindow is the mandatory wait for a denied step's "no
// broadcast reaches ANY participant" assertion (binding, task-2-brief.md) —
// reused as the reconnect catch-up's "nothing extra arrived" tail check.
// Fixed, not tunable: it can only ever false-PASS an implementation that
// eventually (but slowly) broadcasts past this window — it can never
// flake-FAIL a correct one, since a connection that truly never broadcasts
// stays silent no matter how long the window is. Keeping it a constant
// (rather than, say, a CLI-tunable var) removes any path to accidentally
// shortening it below that safety margin.
const denialAbsenceWindow = 300 * time.Millisecond

// observeTimeout bounds how long an OK step waits for its produced event to
// reach a given participant, and how long a reconnect step waits for each
// catch-up item, before giving up. Not brief-mandated (only the denial
// window is); chosen generous enough for a real wire round trip while still
// failing a genuinely-broken case in a few seconds under test.
const observeTimeout = 2 * time.Second

// participantIDPlaceholderPrefix/Suffix delimit this library's one
// templating convention: `{{id:<participantName>}}` inside a command
// step's raw JSON, resolved by resolveParticipantIDPlaceholders. This
// exists because some commands need to reference a participant's
// SERVER-ASSIGNED identity — e.g. AddActor's Actor.controller_id, which
// gateway authz checks with a strict equality against the real
// identity.Participant.ID behind an invite token — a value a static,
// committed scenario file cannot embed directly: the id is minted fresh,
// randomly, per invite, and is unknowable at scenario-authoring time (P6
// Task 4 report, reviewer-adjudicated fix round: this resolution
// originally lived in test-side helpers outside the engine; moved here so
// EVERY caller — self-contained boot glue, live-mode `--tokens` files, the
// scenario library runner — gets it for free via one parameter, rather
// than each caller reimplementing its own substitution pass).
const (
	participantIDPlaceholderPrefix = "{{id:"
	participantIDPlaceholderSuffix = "}}"
)

// resolveParticipantIDPlaceholders substitutes every `{{id:<name>}}`
// occurrence across every step's Command bytes with ids[<name>], in place,
// exactly once, before RunScenario dials any participant or dispatches any
// command. Resolution stays a plain byte substitution — no protojson or
// other payload parsing — matching scenario.go's own doc comment that
// Command bytes are opaque data at this layer until execution; identifying
// a placeholder's name is a byte scan, not a JSON decode.
//
// A scenario with no placeholders at all (the overwhelmingly common case)
// is a no-op regardless of what ids contains, or whether ids is nil. A step
// that DOES reference a name missing from ids is a hard, named error —
// naming both the step index and the unresolved participant name — raised
// as a framework failure (RunScenario returns it as-is, before dialing
// anything) rather than silently dispatching the literal placeholder text
// as if it were real command data.
func resolveParticipantIDPlaceholders(steps []Step, ids map[string]string) error {
	for i := range steps {
		if len(steps[i].Command) == 0 {
			continue
		}
		raw := steps[i].Command
		for name, id := range ids {
			raw = bytes.ReplaceAll(raw,
				[]byte(participantIDPlaceholderPrefix+name+participantIDPlaceholderSuffix),
				[]byte(id))
		}
		if idx := bytes.Index(raw, []byte(participantIDPlaceholderPrefix)); idx >= 0 {
			rest := raw[idx+len(participantIDPlaceholderPrefix):]
			name := string(rest)
			if end := bytes.Index(rest, []byte(participantIDPlaceholderSuffix)); end >= 0 {
				name = string(rest[:end])
			}
			return fmt.Errorf("harness: scenario step %d command references {{id:%s}} but no id was supplied for participant %q", i, name, name)
		}
		steps[i].Command = raw
	}
	return nil
}

// Report is RunScenario's outcome: one StepResult per scenario step (in
// order, every step always runs — a failing step does not abort the run,
// so a single pass surfaces every problem a scenario has, not just the
// first), one ProbeResult per probe, and Pass = true iff every one of those
// individual results passed.
type Report struct {
	Steps  []StepResult
	Probes []ProbeResult
	Pass   bool
}

// StepResult is one step's outcome. Kind is "command" or "reconnect".
type StepResult struct {
	Index  int
	By     string
	Kind   string
	Pass   bool
	Detail string
}

// ProbeResult is one probe's outcome. Kind is "tokenAt", "sessionCount", or
// "actorExists".
type ProbeResult struct {
	Index  int
	Kind   string
	Pass   bool
	Detail string
}

// RunScenario dials every declared participant (after=0), executes sc.Steps
// in order against the binding engine semantics (task-2-brief.md):
//
//   - a denied step (Expect.DeniedContaining != "") asserts the command
//     result is ok=false with an error containing that substring, AND that
//     NO participant observes any event broadcast within a 300ms absence
//     window;
//   - an accepted step (Expect.OK) asserts the result is ok=true, AND that
//     the event it produced (matched by Sequence) is observed by EVERY
//     participant;
//   - a reconnect step closes that participant's Conn, redials via dial
//     with the given afterSequence, and asserts the resulting catch-up
//     replay equals — in event_id order — the subset of that participant's
//     own previously-observed live events with Sequence > afterSequence.
//
// ids is this library's participant-id-placeholder resolution map (P6 Task
// 4 fix round): every `{{id:<name>}}` occurrence across sc.Steps' Command
// bytes is resolved to ids[<name>] ONCE, before any participant is dialed
// or any command dispatched — see resolveParticipantIDPlaceholders' doc
// comment for why this exists and stays a plain byte substitution. ids may
// be nil for a scenario that uses no placeholders at all (the common case).
//
// Once every step has run, each probe is evaluated against
// Fold(everything participant 0 has ever observed, live or via catch-up).
// Human-readable progress is written to report as steps/probes complete;
// report may be nil (io.Discard). The returned error is reserved for
// framework failures (e.g. a participant's initial dial failing, or an
// unresolved id placeholder) — scenario assertion failures are reported
// through Report.Pass/StepResult/ProbeResult, never as a non-nil error.
func RunScenario(ctx context.Context, sc *Scenario, dial Dialer, ids map[string]string, report io.Writer) (*Report, error) {
	if report == nil {
		report = io.Discard
	}

	if err := resolveParticipantIDPlaceholders(sc.Steps, ids); err != nil {
		return nil, err
	}

	conns := make(map[string]Conn, len(sc.Participants))
	history := make(map[string][]*vttv1.Envelope, len(sc.Participants))
	for _, p := range sc.Participants {
		c, err := dial(p.Name, 0)
		if err != nil {
			return nil, fmt.Errorf("harness: dial participant %q: %w", p.Name, err)
		}
		conns[p.Name] = c
		history[p.Name] = nil
	}
	defer func() {
		for _, c := range conns {
			if c != nil {
				_ = c.Close()
			}
		}
	}()

	rep := &Report{Pass: true}
	for i, st := range sc.Steps {
		var sr StepResult
		switch {
		case len(st.Command) > 0:
			sr = runCommandStep(ctx, i, st, conns, history)
		case st.Reconnect != nil:
			sr = runReconnectStep(i, st, dial, conns, history)
		default:
			sr = StepResult{Index: i, By: st.By, Kind: "unknown", Detail: "step has neither command nor reconnect"}
		}
		rep.Steps = append(rep.Steps, sr)
		if !sr.Pass {
			rep.Pass = false
		}
		fmt.Fprintf(report, "[step %d] by=%s kind=%s pass=%t %s\n", sr.Index, sr.By, sr.Kind, sr.Pass, sr.Detail)
	}

	if len(sc.Participants) > 0 {
		first := sc.Participants[0].Name
		state, err := Fold(history[first])
		if err != nil {
			return nil, fmt.Errorf("harness: fold participant %q's observed events for probes: %w", first, err)
		}
		for i, p := range sc.Probes {
			pr := evaluateProbe(i, p, state)
			rep.Probes = append(rep.Probes, pr)
			if !pr.Pass {
				rep.Pass = false
			}
			fmt.Fprintf(report, "[probe %d] kind=%s pass=%t %s\n", pr.Index, pr.Kind, pr.Pass, pr.Detail)
		}
	}

	return rep, nil
}

// runCommandStep sends st.Command on the issuing participant's connection
// and checks it against st.Expect (denial or acceptance — see RunScenario's
// doc comment for the exact contract of each).
func runCommandStep(ctx context.Context, idx int, st Step, conns map[string]Conn, history map[string][]*vttv1.Envelope) StepResult {
	sr := StepResult{Index: idx, By: st.By, Kind: "command"}

	conn, ok := conns[st.By]
	if !ok || conn == nil {
		sr.Detail = fmt.Sprintf("no live connection for participant %q", st.By)
		return sr
	}

	var cmd vttv1.ClientCommand
	if err := protojson.Unmarshal(st.Command, &cmd); err != nil {
		sr.Detail = fmt.Sprintf("parse command: %v", err)
		return sr
	}
	if cmd.RequestId == "" {
		cmd.RequestId = fmt.Sprintf("scenario-step-%d", idx)
	}

	result, err := conn.SendCommand(ctx, &cmd)
	if err != nil {
		sr.Detail = fmt.Sprintf("send command: %v", err)
		return sr
	}

	wantDenied := st.Expect != nil && st.Expect.DeniedContaining != ""
	if wantDenied {
		if result.Ok {
			sr.Detail = "want ok=false (denied), got ok=true"
			return sr
		}
		if !strings.Contains(result.Error, st.Expect.DeniedContaining) {
			sr.Detail = fmt.Sprintf("error %q does not contain %q", result.Error, st.Expect.DeniedContaining)
			return sr
		}
		leaked := drainAllForSilence(conns, history, denialAbsenceWindow)
		if len(leaked) > 0 {
			sr.Detail = fmt.Sprintf("unexpected broadcast observed by %s after a denied command", strings.Join(leaked, ", "))
			return sr
		}
		sr.Pass = true
		return sr
	}

	// Accepted-command expectation.
	if !result.Ok {
		sr.Detail = fmt.Sprintf("want ok=true, got ok=false (error %q)", result.Error)
		return sr
	}
	missing := observeOnAll(conns, history, result.Sequence, observeTimeout)
	if len(missing) > 0 {
		sr.Detail = fmt.Sprintf("event (sequence %d) not observed matching by: %s", result.Sequence, strings.Join(missing, ", "))
		return sr
	}
	sr.Pass = true
	return sr
}

// runReconnectStep closes the participant's current connection, redials via
// dial at st.Reconnect.AfterSequence, and asserts the catch-up replay
// equals (event_id order) the subset of that participant's own previously
// observed live history with Sequence > AfterSequence. history[st.By] is
// updated to reflect what was ACTUALLY received (ground truth for any later
// step or probe), regardless of whether it matched expectations.
func runReconnectStep(idx int, st Step, dial Dialer, conns map[string]Conn, history map[string][]*vttv1.Envelope) StepResult {
	sr := StepResult{Index: idx, By: st.By, Kind: "reconnect"}
	name := st.By

	oldConn, ok := conns[name]
	if !ok || oldConn == nil {
		sr.Detail = fmt.Sprintf("no live connection for participant %q", name)
		return sr
	}
	after := st.Reconnect.AfterSequence

	var prefix, want []*vttv1.Envelope
	for _, env := range history[name] {
		if env.Sequence > after {
			want = append(want, env)
		} else {
			prefix = append(prefix, env)
		}
	}

	_ = oldConn.Close()

	newConn, err := dial(name, after)
	if err != nil {
		sr.Detail = fmt.Sprintf("redial: %v", err)
		return sr
	}
	conns[name] = newConn

	got := make([]*vttv1.Envelope, 0, len(want))
	timedOut := false
	for range want {
		select {
		case env, ok := <-newConn.Events():
			if !ok {
				timedOut = true
			} else {
				got = append(got, env)
			}
		case <-time.After(observeTimeout):
			timedOut = true
		}
		if timedOut {
			break
		}
	}

	extra := false
	if !timedOut {
		select {
		case env, ok := <-newConn.Events():
			if ok {
				got = append(got, env)
				extra = true
			}
		case <-time.After(denialAbsenceWindow):
		}
	}

	history[name] = append(append([]*vttv1.Envelope{}, prefix...), got...)

	switch {
	case timedOut:
		sr.Detail = fmt.Sprintf("catch-up: got %d/%d events before timing out", len(got), len(want))
		return sr
	case extra:
		sr.Detail = fmt.Sprintf("catch-up: got %d events, want exactly %d (extra arrived)", len(got), len(want))
		return sr
	case len(got) != len(want):
		sr.Detail = fmt.Sprintf("catch-up length = %d, want %d", len(got), len(want))
		return sr
	}
	for i := range want {
		if got[i].EventId != want[i].EventId {
			sr.Detail = fmt.Sprintf("catch-up[%d] event_id = %q, want %q (matching live-observed order)", i, got[i].EventId, want[i].EventId)
			return sr
		}
	}
	sr.Pass = true
	return sr
}

// observeOnAll reads exactly one new event from every participant's
// connection (in parallel, each bounded by timeout), records it into
// history regardless of outcome, and returns the names of participants that
// either didn't see anything in time or saw an event with the wrong
// Sequence.
func observeOnAll(conns map[string]Conn, history map[string][]*vttv1.Envelope, wantSeq int64, timeout time.Duration) []string {
	names := sortedParticipantNames(conns)
	type outcome struct {
		name string
		ok   bool
	}
	results := make(chan outcome, len(names))
	var mu sync.Mutex

	for _, name := range names {
		name, conn := name, conns[name]
		go func() {
			select {
			case env, chOK := <-conn.Events():
				if !chOK {
					results <- outcome{name, false}
					return
				}
				mu.Lock()
				history[name] = append(history[name], env)
				mu.Unlock()
				results <- outcome{name, env.Sequence == wantSeq}
			case <-time.After(timeout):
				results <- outcome{name, false}
			}
		}()
	}

	var missing []string
	for range names {
		r := <-results
		if !r.ok {
			missing = append(missing, r.name)
		}
	}
	sort.Strings(missing)
	return missing
}

// drainAllForSilence waits window for EVERY participant in parallel (so the
// total wall-clock cost is ~window regardless of participant count, not
// window*count) and returns the names of any that received an event —
// recording whatever arrived into history either way, since it really was
// received over the wire.
func drainAllForSilence(conns map[string]Conn, history map[string][]*vttv1.Envelope, window time.Duration) []string {
	names := sortedParticipantNames(conns)
	type outcome struct {
		name string
		env  *vttv1.Envelope
	}
	results := make(chan outcome, len(names))
	var mu sync.Mutex

	for _, name := range names {
		name, conn := name, conns[name]
		go func() {
			select {
			case env, chOK := <-conn.Events():
				if chOK {
					mu.Lock()
					history[name] = append(history[name], env)
					mu.Unlock()
					results <- outcome{name, env}
				} else {
					results <- outcome{name, nil}
				}
			case <-time.After(window):
				results <- outcome{name, nil}
			}
		}()
	}

	var leaked []string
	for range names {
		r := <-results
		if r.env != nil {
			leaked = append(leaked, r.name)
		}
	}
	sort.Strings(leaked)
	return leaked
}

func sortedParticipantNames(conns map[string]Conn) []string {
	names := make([]string, 0, len(conns))
	for name := range conns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// evaluateProbe checks one probe against the folded state.
func evaluateProbe(idx int, p Probe, state *engine.State) ProbeResult {
	switch {
	case p.TokenAt != nil:
		tok, ok := state.Tokens[p.TokenAt.TokenId]
		pass := ok && tok.X == p.TokenAt.X && tok.Y == p.TokenAt.Y
		return ProbeResult{
			Index: idx, Kind: "tokenAt", Pass: pass,
			Detail: fmt.Sprintf("tokenAt %s: got present=%t x=%d y=%d, want present=true x=%d y=%d",
				p.TokenAt.TokenId, ok, tok.X, tok.Y, p.TokenAt.X, p.TokenAt.Y),
		}
	case p.SessionCount != nil:
		open, total := 0, len(state.Sessions)
		for _, s := range state.Sessions {
			if s.EndSeq == 0 {
				open++
			}
		}
		pass := open == p.SessionCount.Open && total == p.SessionCount.Total
		return ProbeResult{
			Index: idx, Kind: "sessionCount", Pass: pass,
			Detail: fmt.Sprintf("sessionCount: got open=%d total=%d, want open=%d total=%d",
				open, total, p.SessionCount.Open, p.SessionCount.Total),
		}
	case p.ActorExists != nil:
		_, ok := state.Actors[p.ActorExists.ActorId]
		return ProbeResult{
			Index: idx, Kind: "actorExists", Pass: ok,
			Detail: fmt.Sprintf("actorExists %s: got present=%t", p.ActorExists.ActorId, ok),
		}
	default:
		return ProbeResult{Index: idx, Kind: "unknown", Detail: "probe has none of tokenAt/sessionCount/actorExists set"}
	}
}
