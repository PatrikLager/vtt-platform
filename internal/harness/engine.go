package harness

import (
	"bytes"
	"context"
	"errors"
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
	// CloseErr reports why the stream ended, or nil while healthy. Events()
	// hands a consumer a closed channel and nothing else, so without this a
	// pure reader cannot tell "the server finished" from "I was disconnected
	// for reading too slowly" — opposite problems with opposite responses.
	//
	// Specifically errors.Is(CloseErr(), ErrEventsOverflow). A clean close
	// also reports non-nil (the read error that ended the loop), so the bare
	// nil check answers nothing the closed channel did not already.
	//
	// On the interface rather than behind a type assertion so a future fake
	// cannot silently no-op it.
	//
	// As of 2026-08-06 every Events() consumer in this package accounts for a
	// closed channel. Directly: drainPreExisting, the reconnect catch-up loop
	// AND its trailing "nothing extra arrived" check, observeOnAll,
	// recvWithin (so collectBatchRun), drainAllForSilence, and
	// soakHistories.start. Indirectly: soak.go's drainToSequence takes a raw
	// channel and cannot ask, so its caller drainFreshCatchUp does it instead.
	// Verified by grepping every Events() use rather than by recollection —
	// an earlier version of this comment claimed the conversion was complete
	// while soakHistories.start, the evidence base for RunSoak's own denial
	// assertion, still had the hole.
	//
	// FOUR of those were returning a WRONG VERDICT, not merely a vague
	// message: a batch truncated by a teardown counted as complete; a
	// scenario denial "proved" by the silence of a participant that could no
	// longer hear; a soak denial proved the same way through histories that
	// had stopped growing; and a reconnect-at-head certifying "nothing
	// replayed" against a socket that could not replay. All four PASSED.
	//
	// The rule that follows, for anything added later: a bounded wait proves
	// a negative only against a LIVE connection. If silence is your evidence,
	// ask this first.
	CloseErr() error
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
// SERVER-ASSIGNED identity — e.g. GrantActorControl's participant_id, which
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

// errFreshCampaignRequired is the framework error RunScenario and RunSoak
// both return when a participant's initial after=0 catch-up delivers ANY
// event before the first command is ever dispatched: this library's steps,
// probes, and reconnect-catch-up assertions all reason in ABSOLUTE sequence
// numbers starting from a fresh campaign (the scenario format's binding
// assumption — see Scenario's own doc comment), so pre-existing history
// from a shared/reused campaign does not fail cleanly — it silently shifts
// every later sequence comparison, producing a confusing step-level
// observe-mismatch instead of naming the real problem. Relative-sequence
// scenarios (expressing assertions relative to the catch-up cursor rather
// than absolute numbers) are a planned format extension, not implemented
// here.
func errFreshCampaignRequired(n int) error {
	return fmt.Errorf("harness: scenario requires a fresh campaign; found %d pre-existing events (relative-sequence scenarios are a planned format extension)", n)
}

// drainPreExisting waits up to window for events already queued on conn's
// Events() channel — the initial catch-up replay a non-fresh campaign
// delivers immediately on dial, before any command has been issued — and
// returns how many arrived. A genuinely fresh campaign's after=0 catch-up
// is empty, so this window elapses in silence and returns 0 (the same
// "bounded wait proves a negative" reasoning denialAbsenceWindow's own doc
// comment already relies on for the denial assertion).
// The error separates "heard nothing on a live stream" (a fresh campaign, the
// negative this is meant to prove) from "heard nothing because the stream was
// already gone" (no evidence either way). Silence proves freshness only from a
// connection that could have spoken.
func drainPreExisting(conn Conn, window time.Duration) (int, error) {
	n := 0
	deadline := time.After(window)
	for {
		select {
		case _, ok := <-conn.Events():
			if !ok {
				return n, streamEndReason(conn)
			}
			n++
		case <-deadline:
			return n, nil
		}
	}
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
	// absenceUnproven marks a passing DENIED step whose "no broadcast
	// reached anyone" claim has not been established yet. RunScenario
	// resolves it against the next accepted step. Unexported on purpose:
	// `vtt client run --json` encodes this struct, and this is internal
	// bookkeeping, not part of the report contract.
	absenceUnproven bool
	// firstSeq is the first sequence an ACCEPTED step produced (0 otherwise),
	// used as the ordering witness for a preceding denial.
	firstSeq int64

	Index  int
	By     string
	Kind   string
	Pass   bool
	Detail string
}

// ProbeResult is one probe's outcome. Kind is "tokenAt", "sessionCount",
// "actorExists", "resourceAt", "hasCondition", or "noteAt".
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
//     the event it produced (matched by Sequence) is observed by every
//     UNPROJECTED participant — the DM and the agent, whose streams are the
//     log itself. A player's or a spectator's stream is a projection of it
//     (visibility spec §3.1), so the number of envelopes they receive for one
//     event is between zero and several and no count can be required of them;
//     what they DO receive is still collected, because the two assertions
//     below are computed from it;
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
// framework failures (e.g. a participant's initial dial failing, an
// unresolved id placeholder, or a non-fresh campaign — see
// errFreshCampaignRequired) — scenario assertion failures are reported
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
	projected := projectedSeats(sc.Participants)
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

	// The fresh-campaign assumption (see errFreshCampaignRequired's doc
	// comment): checked once, on the first participant, right after dialing
	// and before any step runs — every participant's after=0 catch-up
	// replays the SAME history, so one representative check is sufficient
	// (Fold's own probe evaluation below makes the same "participant 0
	// stands in for the group" choice).
	if len(sc.Participants) > 0 {
		n, err := drainPreExisting(conns[sc.Participants[0].Name], denialAbsenceWindow)
		if err != nil {
			return nil, fmt.Errorf("harness: cannot confirm a fresh campaign: %w", err)
		}
		if n > 0 {
			return nil, errFreshCampaignRequired(n)
		}
	}

	rep := &Report{Pass: true}
	// A denied step passes provisionally: its "no broadcast reached anyone"
	// claim is settled by the next accepted step's event (see leakedSince).
	//
	// This is a QUEUE, not a single slot. Denials run back to back in five of
	// the six committed scenarios (denials.json has fourteen), and a denial
	// produces no event to settle against — so a single slot was simply
	// overwritten by the next denial, leaving the first one's claim never
	// settled and any leak blamed on the wrong step.
	var pending []pendingDenial

	// Printing lags the queue by design: a step's line is written only once
	// its verdict is FINAL. Printing eagerly meant a denied step that later
	// flipped to failed had already been logged as pass=true, and without
	// --json that log is the only output an operator sees.
	nextToPrint := 0
	printFinal := func(upto int) {
		for ; nextToPrint < upto; nextToPrint++ {
			s := rep.Steps[nextToPrint]
			fmt.Fprintf(report, "[step %d] by=%s kind=%s pass=%t %s\n",
				s.Index, s.By, s.Kind, s.Pass, s.Detail)
		}
	}

	for i, st := range sc.Steps {
		var sr StepResult
		switch {
		case len(st.Command) > 0:
			sr = runCommandStep(ctx, i, st, conns, projected, history)
		case st.Reconnect != nil:
			sr = runReconnectStep(i, st, dial, conns, history)
		default:
			sr = StepResult{Index: i, By: st.By, Kind: "unknown", Detail: "step has neither command nor reconnect"}
		}
		rep.Steps = append(rep.Steps, sr)

		// Settle EVERY outstanding denial against this step's own event: they
		// all precede it, so anything they wrongly produced carries a lower
		// sequence and must already have arrived.
		if len(pending) > 0 && sr.firstSeq > 0 {
			if leaked := leakedSince(history, pending[0].lens, sr.firstSeq); len(leaked) > 0 {
				markLeaked(rep, pending, leaked)
				// The witness step is collateral: it read the leaked event
				// instead of its own, so its "not observed" message would
				// otherwise send an operator after the wrong command.
				if !sr.Pass {
					rep.Steps[i].Detail += " (caused by the leaked broadcast from the denied step above)"
				}
			}
			pending = nil
		}
		if sr.absenceUnproven {
			pending = append(pending, pendingDenial{idx: i, lens: historyLens(history)})
		}

		if !rep.Steps[i].Pass {
			rep.Pass = false
		}
		// Everything before the earliest outstanding denial is now final.
		limit := i + 1
		if len(pending) > 0 {
			limit = pending[0].idx
		}
		printFinal(limit)
	}

	// Denials with no accepted step after them have no ordering witness, so
	// they fall back to the original bounded wait. Only a trailing run of
	// denials can reach this, so the cost is paid once per scenario rather
	// than once per denial.
	if len(pending) > 0 {
		leaked, ended := drainAllForSilence(conns, history, denialAbsenceWindow)
		switch {
		case len(leaked) > 0:
			markLeaked(rep, pending, leaked)
		case len(ended) > 0:
			// Silence from a dead stream is not evidence of anything. Failing
			// here rather than passing is the whole point: this assertion is
			// PROVED BY ABSENCE, so a participant that could no longer receive
			// makes the proof void, not satisfied.
			markUnprovableSilence(rep, pending, ended)
		}
	}
	printFinal(len(rep.Steps))

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

// pendingDenial is a denied step whose "no broadcast reached anyone" claim is
// waiting on a later accepted step's sequence to settle it.
type pendingDenial struct {
	idx  int
	lens map[string]int
}

// markLeaked records a leaked broadcast against the outstanding denials.
//
// Consecutive denials produce no events, so nothing orders them relative to
// each other: when several are outstanding, the leak provably came from one of
// them but which one is genuinely unknowable from the wire. The earliest
// carries the failure — it is the first place an operator should look — and
// the message states the range rather than implying a precision the proof does
// not have.
func markLeaked(rep *Report, pending []pendingDenial, leaked []string) {
	first := pending[0]
	detail := fmt.Sprintf("unexpected broadcast observed by %s after a denied command",
		strings.Join(leaked, ", "))
	if len(pending) > 1 {
		detail += fmt.Sprintf(" (one of the denied steps %d-%d produced it; no accepted event"+
			" separates them, so it cannot be pinned further)", first.idx, pending[len(pending)-1].idx)
	}
	rep.Steps[first.idx].Pass = false
	rep.Steps[first.idx].Detail = detail
	rep.Pass = false
}

// markUnprovableSilence fails the denial the same way markLeaked does, but for
// the opposite evidence: nothing was heard AND at least one participant could
// not have heard anything. denialAbsenceWindow's doc justifies proving the
// negative by silence on the grounds that "a connection that truly never
// broadcasts stays silent no matter how long the window is" — which holds for
// a LIVE connection and not for a torn-down one.
func markUnprovableSilence(rep *Report, pending []pendingDenial, ended []string) {
	// EVERY pending denial, unlike markLeaked's first-only. That asymmetry is
	// the point: a leak provably came from exactly one denied step and which
	// one is unknowable, so blaming the first is the honest summary. Here
	// there is no single culprit — every outstanding denial rests its absence
	// claim on the SAME dead participant, so every one of them is equally
	// unproven. Marking one would leave the rest printing pass=true on
	// evidence that does not exist (scenarios/denials.json has 14 in a row).
	detail := fmt.Sprintf(
		"denial unprovable: the stream ended for %s, so their silence is not evidence that "+
			"the denied command broadcast nothing", strings.Join(ended, ", "))
	for _, d := range pending {
		rep.Steps[d.idx].Pass = false
		rep.Steps[d.idx].Detail = detail
	}
	rep.Pass = false
}

// historyLens snapshots how many envelopes each participant has observed.
func historyLens(history map[string][]*vttv1.Envelope) map[string]int {
	out := make(map[string]int, len(history))
	for name, envs := range history {
		out[name] = len(envs)
	}
	return out
}

// leakedSince returns the participants who received an envelope with a
// sequence BELOW witnessSeq after the snapshot in lens — i.e. an event that
// was already in the log before the accepted step's own, and therefore was
// produced by the denied command that preceded it.
//
// This is the ordering proof. Per-connection delivery is sequence-ordered and
// the gateway broadcasts only from the store subscription, so anything a
// denied command wrongly produced must arrive before the next accepted event.
// Finding none by the time that event lands is proof of absence — no waiting,
// and no dependence on how fast the server happens to be.
func leakedSince(history map[string][]*vttv1.Envelope, lens map[string]int, witnessSeq int64) []string {
	var leaked []string
	for name, envs := range history {
		// min: a reconnect step REBUILDS history from what the redialled
		// connection actually replayed, so it can be SHORTER than a pending
		// denial's snapshot. Indexing that unguarded panicked — the soak twin
		// (leakedBelow) has always had this guard; this copy did not.
		for _, env := range envs[min(lens[name], len(envs)):] {
			if env.GetSequence() < witnessSeq {
				leaked = append(leaked, name)
				break
			}
		}
	}
	sort.Strings(leaked)
	return leaked
}

// runCommandStep sends st.Command on the issuing participant's connection
// and checks it against st.Expect (denial or acceptance — see RunScenario's
// doc comment for the exact contract of each).
func runCommandStep(ctx context.Context, idx int, st Step, conns map[string]Conn, projected map[string]bool, history map[string][]*vttv1.Envelope) StepResult {
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
		// Absence is NOT proven here. Proving it by waiting costs
		// denialAbsenceWindow on every denial — 64s of this package's 93s
		// and 28s of cmd/vtt's 42s — and blames the wrong step when a leak
		// does occur: observeOnAll reads one event per participant and fails
		// on a sequence mismatch, so the leak surfaces as the NEXT step's
		// "event not observed", pointing an operator at the innocent command.
		//
		// Instead the verdict is deferred to RunScenario, which resolves it
		// against the next accepted step's own event: per-connection delivery
		// is sequence-ordered and the gateway broadcasts only from the store
		// subscription, so anything the denied command wrongly produced
		// carries a LOWER sequence and must already have arrived by then.
		// That is a proof rather than a timeout, it costs nothing, and it
		// names the step that caused it.
		sr.Pass = true
		sr.absenceUnproven = true
		return sr
	}

	// Accepted-command expectation.
	if !result.Ok {
		sr.Detail = fmt.Sprintf("want ok=true, got ok=false (error %q)", result.Error)
		return sr
	}

	// use_ability and load_adventure are BATCH-aware (ruleset-interpreter
	// Task 6, binding; extended to load_adventure by adventure-format Task
	// 4 — both commands' CommandResult carries only the FIRST sequence of a
	// campaign.AppendBatch batch): rules.Resolve can emit anywhere from one
	// (AbilityUsed alone, e.g. a miss with no on-miss outcomes) to several
	// events per use, and adventure.Compile emits one envelope per scene/
	// actor/placement/note plus a leading AdventureLoaded and trailing
	// NarrationAdded — neither step has any way to know its batch's length
	// in advance. Every other accepted command still produces exactly ONE
	// event, matched by observeOnAll below.
	if isBatchCommand(&cmd) {
		sr.firstSeq = result.Sequence
		missing, detail := observeBatchOnAll(conns, projected, history, result.Sequence, observeTimeout, denialAbsenceWindow)
		if len(missing) > 0 {
			sr.Detail = fmt.Sprintf("batch (first sequence %d) mismatch for %s: %s", result.Sequence, strings.Join(missing, ", "), detail)
			return sr
		}
		sr.Pass = true
		return sr
	}

	sr.firstSeq = result.Sequence
	missing, ended := observeOnAll(conns, projected, history, result.Sequence, observeTimeout, denialAbsenceWindow)
	if len(ended) > 0 || len(missing) > 0 {
		// Both facts, never one instead of the other: "did not observe" sends
		// the reader after a broadcast that may have been perfect, and a
		// participant whose stream died says nothing about one that genuinely
		// missed it. Reporting only the first found would hide the second.
		parts := make([]string, 0, 2)
		if len(ended) > 0 {
			parts = append(parts, fmt.Sprintf("unobservable — stream ended for: %s", strings.Join(ended, ", ")))
		}
		if len(missing) > 0 {
			parts = append(parts, fmt.Sprintf("not observed matching by: %s", strings.Join(missing, ", ")))
		}
		sr.Detail = fmt.Sprintf("event (sequence %d) %s", result.Sequence, strings.Join(parts, "; "))
		return sr
	}
	sr.Pass = true
	return sr
}

// isBatchCommand reports whether cmd's oneof case produces a whole
// campaign.AppendBatch batch (result.Sequence names only the batch's FIRST
// sequence) rather than a single Envelope via campaign.Append — the set of
// commands runCommandStep must route to observeBatchOnAll instead of
// observeOnAll. Grows as new batch-producing commands are added (use_ability
// — ruleset-interpreter Task 6; load_adventure — adventure-format Task 4).
func isBatchCommand(cmd *vttv1.ClientCommand) bool {
	switch cmd.GetCommand().(type) {
	case *vttv1.ClientCommand_UseAbility, *vttv1.ClientCommand_LoadAdventure:
		return true
	default:
		return false
	}
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
	// Tracked apart from timedOut. Setting that flag on a channel close is the
	// misdiagnosis this separation exists to stop: it reported "before timing
	// out" for a redial whose stream was torn down in milliseconds, naming an
	// observeTimeout that never elapsed.
	var streamEnd error
	for range want {
		select {
		case env, ok := <-newConn.Events():
			if !ok {
				streamEnd = streamEndReason(newConn)
			} else {
				got = append(got, env)
			}
		case <-time.After(observeTimeout):
			timedOut = true
		}
		if timedOut || streamEnd != nil {
			break
		}
	}

	extra := false
	if !timedOut && streamEnd == nil {
		select {
		case env, ok := <-newConn.Events():
			if ok {
				got = append(got, env)
				extra = true
			} else {
				// "Nothing extra arrived" is another proof by ABSENCE, and a
				// dead socket satisfies it by construction. Reachable with an
				// EMPTY want (a reconnect at the head, asserting nothing
				// replays), where the loop above never runs and so never asks.
				streamEnd = streamEndReason(newConn)
			}
		case <-time.After(denialAbsenceWindow):
		}
	}

	history[name] = append(append([]*vttv1.Envelope{}, prefix...), got...)

	switch {
	case streamEnd != nil:
		sr.Detail = fmt.Sprintf("catch-up: got %d/%d events before the stream ended: %v", len(got), len(want), streamEnd)
		return sr
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

// observeOnAll reads exactly one new event from every UNPROJECTED
// participant's connection (in parallel, each bounded by timeout), records it
// into history regardless of outcome, and returns the names of participants
// that either didn't see anything in time or saw an event with the wrong
// Sequence.
//
// Projected participants are drained rather than matched — see the block
// below for why that is the only honest contract once a seat's stream is a
// projection of the log rather than the log.
func observeOnAll(conns map[string]Conn, projected map[string]bool, history map[string][]*vttv1.Envelope, wantSeq int64, timeout, quietWindow time.Duration) (missingNames, endedNames []string) {
	names := sortedParticipantNames(conns)
	type outcome struct {
		name  string
		ok    bool
		ended bool
	}
	results := make(chan outcome, len(names))
	var mu sync.Mutex

	required := 0
	for _, name := range names {
		name, conn := name, conns[name]
		if projected[name] {
			continue // drained below, after the required seats have observed
		}
		required++
		go func() {
			select {
			case env, chOK := <-conn.Events():
				if !chOK {
					// Did not observe it, but because the stream was gone —
					// a different fault with a different fix than a broadcast
					// that never arrived.
					results <- outcome{name: name, ended: true}
					return
				}
				mu.Lock()
				history[name] = append(history[name], env)
				mu.Unlock()
				results <- outcome{name: name, ok: env.Sequence == wantSeq}
			case <-time.After(timeout):
				results <- outcome{name: name}
			}
		}()
	}

	var missing, ended []string
	for i := 0; i < required; i++ {
		r := <-results
		switch {
		case r.ended:
			ended = append(ended, r.name)
		case !r.ok:
			missing = append(missing, r.name)
		}
	}

	// A PROJECTED SEAT IS NOT REQUIRED TO OBSERVE ANYTHING, and that is the
	// visibility arc's doing rather than a relaxation: what such a seat
	// receives for one event is between zero and several envelopes, and zero
	// is the right answer for an event about a room they are not standing in
	// (visibility spec §4.2). Requiring one would fail every honest
	// projection.
	//
	// It is still DRAINED, and that half is load-bearing twice over: the
	// denial checks prove a negative by counting what arrived since
	// (leakedSince), and a reconnect step compares catch-up against what the
	// seat saw LIVE. Both read history, so a seat nobody collected would make
	// one assertion vacuous and the other wrong.
	//
	// AFTER the required seats, never beside them, and that ordering is a
	// measured requirement rather than tidiness. The drain is bounded by a
	// quiet window and the required read by a much longer timeout, so a
	// broadcast that arrives late — which is exactly the shape of the leak
	// TestDeniedCommandLeakFailsTheDeniedStep injects — reaches the required
	// seat and would have been missed entirely by a projected one running in
	// parallel with it. Starting the drain once the event has demonstrably
	// been delivered somewhere means the projected copy is already queued.
	if len(names) > required {
		var wg sync.WaitGroup
		var endedMu sync.Mutex
		for _, name := range names {
			if !projected[name] {
				continue
			}
			name, conn := name, conns[name]
			wg.Add(1)
			go func() {
				defer wg.Done()
				if drainQuietInto(name, conn, history, &mu, quietWindow) {
					endedMu.Lock()
					ended = append(ended, name)
					endedMu.Unlock()
				}
			}()
		}
		wg.Wait()
	}

	sort.Strings(missing)
	sort.Strings(ended)
	return missing, ended
}

// observeBatchOnAll asserts every participant observes the FULL batch a
// use_ability command's result implies, as a CONTIGUOUS run of sequences
// starting at firstSeq (ruleset-interpreter Task 6, binding, documented
// here as the one place this is implemented): a use_ability CommandResult
// carries only the batch's first sequence (campaign.AppendBatch's own
// contract — see internal/gateway/ruleset.go), never its length, so this
// cannot wait for a known event count the way observeOnAll does for a
// single-event command.
//
// Per participant, independently and in parallel: wait up to firstTimeout
// for an event whose Sequence equals firstSeq (the batch's leading event,
// AbilityUsed); once seen, keep collecting subsequent events as long as
// each arrives within quietWindow of the previous one AND its Sequence is
// exactly one past the last one collected (a gap-free run) — the run ends
// the instant either condition fails: quietWindow elapses with nothing
// further (the batch is over — the SAME "bounded wait proves a negative"
// reasoning denialAbsenceWindow's own doc comment already relies on), or an
// event arrives breaking contiguity (a genuine bug, surfaced as a mismatch
// rather than silently accepted). Every observed event — whether or not it
// ends up part of the counted run — is still recorded into history, ground
// truth for any later step or probe, exactly like observeOnAll's own
// per-event recording.
//
// Correctness of "any event observed here belongs to this batch" rests on
// RunScenario's own serial execution: exactly one command is ever in
// flight at a time (steps run strictly in order, and this function is only
// reached after `result.Ok` has already been confirmed for THIS step's own
// command) — no other participant is issuing a concurrent command whose
// events could interleave with this batch's broadcast while this function
// is collecting, so there is nothing to "push back" onto conn.Events() (a
// channel offers no such operation) even in the contiguity-break failure
// case above.
//
// The step passes only if every participant's collected run has the SAME
// length and the SAME sequence of event ids, in order (compared against
// the first participant's own run) — one atomic AppendBatch broadcast to
// every subscriber must look identical to every observer, or the step
// fails naming which participant(s) diverged and how.
func observeBatchOnAll(conns map[string]Conn, projected map[string]bool, history map[string][]*vttv1.Envelope, firstSeq int64, firstTimeout, quietWindow time.Duration) (missing []string, detail string) {
	all := sortedParticipantNames(conns)
	// Same split, same reason, as observeOnAll's: a projected seat receives a
	// projection of the batch, which is legitimately shorter than the batch
	// and legitimately empty, so it is drained rather than matched.
	var names []string
	for _, n := range all {
		if !projected[n] {
			names = append(names, n)
		}
	}
	results := make(chan batchOutcome, len(names))
	var mu sync.Mutex

	for _, name := range names {
		name, conn := name, conns[name]
		go func() {
			results <- collectBatchRun(name, conn, history, &mu, firstSeq, firstTimeout, quietWindow)
		}()
	}

	byName := make(map[string]batchOutcome, len(names))
	for range names {
		r := <-results
		byName[r.name] = r
	}

	// Projected seats: drained, never matched, and drained AFTER — the same
	// split and the same ordering reason as observeOnAll's.
	if len(all) > len(names) {
		var wg sync.WaitGroup
		for _, name := range all {
			if !projected[name] {
				continue
			}
			name, conn := name, conns[name]
			wg.Add(1)
			go func() {
				defer wg.Done()
				drainQuietInto(name, conn, history, &mu, quietWindow)
			}()
		}
		wg.Wait()
	}

	for _, name := range names {
		if o := byName[name]; o.err != "" {
			missing = append(missing, name)
			if detail == "" {
				detail = fmt.Sprintf("%s: %s", name, o.err)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return missing, detail
	}

	if len(names) == 0 {
		// EVERY seat is projected, so there is no unfiltered stream to compare
		// the batch against and this function has nothing left to assert. Not
		// silently: the drain above still recorded what each seat received, so
		// the denial and reconnect checks downstream keep their evidence. A
		// scenario that wants the batch itself pinned needs a DM or an agent
		// in it, which every committed scenario has.
		return nil, ""
	}
	want := byName[names[0]].envs
	for _, name := range names[1:] {
		got := byName[name].envs
		if len(got) != len(want) {
			missing = append(missing, name)
			if detail == "" {
				detail = fmt.Sprintf("%s observed %d batch events, %s observed %d", name, len(got), names[0], len(want))
			}
			continue
		}
		for i := range want {
			if got[i].EventId != want[i].EventId {
				missing = append(missing, name)
				if detail == "" {
					detail = fmt.Sprintf("%s batch event %d (sequence %d) id mismatch vs %s", name, i, got[i].Sequence, names[0])
				}
				break
			}
		}
	}
	sort.Strings(missing)
	return missing, detail
}

// batchOutcome is one participant's result from collectBatchRun: envs is
// the contiguous run collected so far (possibly partial, on an err); err is
// empty on success.
type batchOutcome struct {
	name string
	envs []*vttv1.Envelope
	err  string
}

// collectBatchRun is observeBatchOnAll's single-participant worker: wait
// for the batch's leading event (Sequence == firstSeq) within firstTimeout,
// then keep collecting a gap-free run of subsequent events, each within
// quietWindow of the last, until quietWindow elapses (success: the batch is
// over) or a non-contiguous event breaks the run (failure). Every event
// observed — including the one that breaks contiguity, if any — is
// recorded into history under mu, regardless of outcome (ground truth for
// any later step or probe, matching observeOnAll's own per-event recording
// contract).
func collectBatchRun(name string, conn Conn, history map[string][]*vttv1.Envelope, mu *sync.Mutex, firstSeq int64, firstTimeout, quietWindow time.Duration) batchOutcome {
	record := func(env *vttv1.Envelope) {
		mu.Lock()
		history[name] = append(history[name], env)
		mu.Unlock()
	}

	first, ok, why := recvWithin(conn, firstTimeout)
	if !ok {
		if why != nil {
			// The subscriber was gone, not the broadcast missing. Without
			// this the reader goes looking for an event that was probably
			// sent, to a connection that could no longer receive it.
			return batchOutcome{name: name, err: fmt.Sprintf("no event observed: %v", why)}
		}
		return batchOutcome{name: name, err: "no event observed"}
	}
	record(first)
	if first.Sequence != firstSeq {
		return batchOutcome{name: name, err: fmt.Sprintf("first observed event sequence = %d, want %d", first.Sequence, firstSeq)}
	}

	envs := []*vttv1.Envelope{first}
	last := first.Sequence
	for {
		next, ok, why := recvWithin(conn, quietWindow)
		if !ok {
			if why != nil {
				// NOT "batch complete". The stream died mid-run, so whether
				// more events were owed is unknowable — and returning a clean
				// outcome here let a truncated observation satisfy a scenario
				// assertion about a longer batch.
				return batchOutcome{name: name, envs: envs, err: fmt.Sprintf(
					"batch truncated after %d event(s): %v", len(envs), why)}
			}
			return batchOutcome{name: name, envs: envs} // quiet window elapsed: batch complete.
		}
		record(next)
		if next.Sequence != last+1 {
			return batchOutcome{name: name, envs: envs, err: fmt.Sprintf(
				"batch run broken: sequence %d arrived immediately after %d (non-contiguous)", next.Sequence, last)}
		}
		envs = append(envs, next)
		last = next.Sequence
	}
}

// errStreamEnded marks the reason a read failed because the Events() channel
// was torn down, as opposed to nothing having arrived in time. Callers ask
// errors.Is(why, errStreamEnded) for "the stream went away" and
// errors.Is(why, ErrEventsOverflow) for "because I was too slow".
var errStreamEnded = errors.New("the event stream ended")

// streamEndReason names why conn's Events() channel closed, wrapping the
// connection's own CloseErr so the cause survives errors.Is.
func streamEndReason(conn Conn) error {
	if err := conn.CloseErr(); err != nil {
		return fmt.Errorf("%w: %w", errStreamEnded, err)
	}
	return errStreamEnded
}

// recvWithin reads one event from conn within timeout. ok=false means no event
// arrived, and the error separates the two ways that happens: nil when the
// window merely elapsed on a live stream, non-nil when the stream was TORN
// DOWN under the read.
//
// Those are opposite facts. A window elapsing on an open stream is how a
// finished batch looks — the ordinary success path. A channel closing means
// the rest of the batch is unobservable, and reporting that as "the batch
// ended" turns a truncated observation into a passing assertion. This used to
// return the same (nil, false) for both.
func recvWithin(conn Conn, timeout time.Duration) (*vttv1.Envelope, bool, error) {
	select {
	case env, ok := <-conn.Events():
		if !ok {
			return nil, false, streamEndReason(conn)
		}
		return env, true, nil
	case <-time.After(timeout):
		return nil, false, nil
	}
}

// drainAllForSilence waits window for EVERY participant in parallel (so the
// total wall-clock cost is ~window regardless of participant count, not
// window*count) and returns the names of any that received an event —
// recording whatever arrived into history either way, since it really was
// received over the wire.
func drainAllForSilence(conns map[string]Conn, history map[string][]*vttv1.Envelope, window time.Duration) (leakedNames, endedNames []string) {
	names := sortedParticipantNames(conns)
	type outcome struct {
		name  string
		env   *vttv1.Envelope
		ended bool
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
					results <- outcome{name: name, env: env}
				} else {
					// Silent, but NOT evidence. This used to report the same
					// outcome as a live quiet stream, which is what let a
					// torn-down participant satisfy a denial assertion.
					results <- outcome{name: name, ended: true}
				}
			case <-time.After(window):
				results <- outcome{name: name}
			}
		}()
	}

	var leaked, ended []string
	for range names {
		r := <-results
		switch {
		case r.env != nil:
			leaked = append(leaked, r.name)
		case r.ended:
			ended = append(ended, r.name)
		}
	}
	sort.Strings(leaked)
	sort.Strings(ended)
	return leaked, ended
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
	case p.ResourceAt != nil:
		actor, ok := state.Actors[p.ResourceAt.ActorId]
		var got int32
		var resPresent bool
		if ok {
			if r, rok := actor.GetResources()[p.ResourceAt.Resource]; rok {
				got = r.GetCurrent()
				resPresent = true
			}
		}
		pass := ok && resPresent && got == p.ResourceAt.Value
		return ProbeResult{
			Index: idx, Kind: "resourceAt", Pass: pass,
			Detail: fmt.Sprintf("resourceAt %s/%s: got actorPresent=%t resourcePresent=%t current=%d, want current=%d",
				p.ResourceAt.ActorId, p.ResourceAt.Resource, ok, resPresent, got, p.ResourceAt.Value),
		}
	case p.HasCondition != nil:
		present := false
		for _, c := range state.Conditions[p.HasCondition.ActorId] {
			if c.ID == p.HasCondition.ConditionId {
				present = true
				break
			}
		}
		pass := present == p.HasCondition.Present
		return ProbeResult{
			Index: idx, Kind: "hasCondition", Pass: pass,
			Detail: fmt.Sprintf("hasCondition %s/%s: got present=%t, want present=%t",
				p.HasCondition.ActorId, p.HasCondition.ConditionId, present, p.HasCondition.Present),
		}
	case p.NoteAt != nil:
		note, ok := state.Notes[p.NoteAt.Key]
		titleOK := p.NoteAt.TitleIs == "" || note.Title == p.NoteAt.TitleIs
		textOK := p.NoteAt.TextContains == "" || strings.Contains(note.Text, p.NoteAt.TextContains)
		pass := ok && titleOK && textOK
		return ProbeResult{
			Index: idx, Kind: "noteAt", Pass: pass,
			Detail: fmt.Sprintf("noteAt %s: got present=%t title=%q text=%q, want titleIs=%q textContains=%q",
				p.NoteAt.Key, ok, note.Title, note.Text, p.NoteAt.TitleIs, p.NoteAt.TextContains),
		}
	default:
		return ProbeResult{Index: idx, Kind: "unknown", Detail: "probe has none of tokenAt/sessionCount/actorExists/resourceAt/hasCondition/noteAt set"}
	}
}

// projectedSeats names the participants whose stream is a PROJECTION of the
// log rather than the log itself (visibility spec §3.1): a player sees what
// their actors can see, a spectator sees over the shoulder they are perched
// on, and the DM and the agent receive every event unchanged (exit criterion
// 8).
//
// BY NAMING THE PROJECTED ROLES, which is the OPPOSITE direction from
// gateway.projected — and the difference is deliberate rather than an
// oversight. The gateway decides what goes on a wire, so an unrecognised role
// there must receive nothing: fail closed, silently, on the safe side. This is
// a TEST HARNESS, and its job is to notice. An unrecognised role here is
// treated as an ordinary observer, so a scenario that introduces one FAILS its
// steps ("not observed matching by: ...") instead of quietly asserting nothing
// about it. Both defaults put the failure where it can be seen; they differ
// because "seen" means different things at the two ends.
//
// An EMPTY role is likewise an ordinary observer: identity mints no invite for
// one, so it cannot reach a real gateway — it only ever names a hand-built
// connection in this package's own tests, where every participant is an
// observer by construction.
//
// Role strings, not identity.Role: this package may not import
// internal/identity (the P1 boundary — the harness proves a client can drive a
// table using the wire constitution alone), and the scenario format carries
// the role as the same string the invite does.
func projectedSeats(ps []Participant) map[string]bool {
	out := make(map[string]bool, len(ps))
	for _, p := range ps {
		switch p.Role {
		case "player", "spectator":
			out[p.Name] = true
		}
	}
	return out
}

// drainQuietInto collects everything conn has to offer, into history, until
// nothing further arrives within quiet. Reports whether the stream ENDED,
// which is a fault even for a seat that is entitled to receive nothing:
// silence and a dead socket are different facts, and the assertions that read
// history downstream rely on the difference.
func drainQuietInto(name string, conn Conn, history map[string][]*vttv1.Envelope, mu *sync.Mutex, quiet time.Duration) (ended bool) {
	for {
		select {
		case env, ok := <-conn.Events():
			if !ok {
				return true
			}
			mu.Lock()
			history[name] = append(history[name], env)
			mu.Unlock()
		case <-time.After(quiet):
			return false
		}
	}
}
