// Soak mode is the wire-level keystone (docs/superpowers/specs/2026-07-24-
// simulation-harness-design.md §5, task-5-brief.md): a seeded generator
// drives the platform's fixed four-role roster (dm, two players, agent)
// through a long, mixed, AUTHORIZED-by-construction command sequence (plus
// a deliberate slice of authz-denied attempts, asserted denied), over the
// SAME wire-only Client/Dialer/Conn surface client.go and engine.go already
// established — RunSoak imports nothing beyond what this package already
// imports (see client.go's package comment for the P1 boundary this file is
// bound by too). At checkpoints and at the end, it proves the platform's
// central promise the same way `vtt state dump` and the reconnect-equality
// scenarios do, just continuously: an INCREMENTALLY folded client-side
// state (built live from one participant's own streamed Events()) must
// DEEP-EQUAL a FRESH catch-up fold obtained by dialing a brand-new second
// connection from scratch (after=0) — rebuild==live, proven over and over
// across a long run, not just once at reconnect.
package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/engine"
)

// SoakConfig configures RunSoak. Seed/Events/CheckEvery are task-5-brief.md's
// pinned shape (CheckEvery defaults to 100 when <= 0). IDs is an ADDITIVE
// field beyond that literal shape — the same pattern RunScenario's own ids
// parameter and cmd/vtt's tokensFile.IDs already establish (see engine.go's
// resolveParticipantIDPlaceholders doc comment for the precedent this
// mirrors): the generator's addActor lifecycle assigns each of the two
// players a controlled actor by setting Actor.controller_id to their REAL,
// server-assigned identity.Participant.ID — a value nothing in this package
// can invent (minted fresh, randomly, per invite; harness may not import
// internal/identity at all — client.go's package comment) — so the caller
// supplies it here, keyed by SoakParticipants()'s participant names, in
// exactly the shape cmd/vtt/harness_boot.go's bootResult.IDs
// (self-contained mode) or tokensFile.IDs (live mode, additive "ids" field)
// already produce for `vtt client run`. IDs may be nil: the run still
// completes (addActor's assignment loop simply never finds a real id for a
// player, so that player's actors stay uncontrolled, and the
// moveOwn/deniedAttempt buckets fall back for them) — degraded mix
// coverage, not a crash. Production callers (cmd/vtt/client_soak.go) always
// supply it, reusing the same bootSelfContained/mintInvites path `vtt
// client run` uses today.
type SoakConfig struct {
	Seed       int64
	Events     int
	CheckEvery int
	IDs        map[string]string
}

// defaultCheckEvery is CheckEvery's default when the caller leaves it <= 0
// (task-5-brief.md).
const defaultCheckEvery = 100

// SoakReport is RunSoak's outcome.
type SoakReport struct {
	// Events is the number of actions RunSoak attempted to issue (==
	// cfg.Events on any run that didn't abort with a framework error).
	Events int
	// Accepted/Denied count issued actions by the wire's own verdict
	// (result.Ok), not by what the generator intended — Denied is expected
	// to equal Counts["deniedAttempt"] exactly on a passing run (the
	// players-only-move-own invariant: nothing outside the deliberate
	// authz-denied bucket should ever come back denied).
	Accepted int
	Denied   int
	// Checkpoints counts how many incremental-fold-vs-fresh-catch-up-fold
	// comparisons ran (periodic, every CheckEvery accepted events, plus
	// exactly one more at the end) — a passing soak report always shows
	// Checkpoints > 0 for any Events large enough to accept at least one
	// action.
	Checkpoints int
	// Counts is a per-action-kind draw count (the mix-ratio bookkeeping):
	// keys are this package's soakAction string values ("createScene",
	// "addActor", "placeToken", "moveOwn", "sessionChurn", "retraction",
	// "deniedAttempt").
	Counts map[string]int
	// Pass is true iff every checkpoint's fold-equality held AND no action
	// outside the deliberate denied-attempt bucket ever came back denied.
	Pass bool
	// Report carries the human action log on a FAILING run, so that --json —
	// the mode CI uses — can say which action or checkpoint failed.
	//
	// It exists because it was documented before it was real: errSoakFailed's
	// comment in cmd/vtt/client_soak.go promised "--json's Report body ... is
	// what names which action/checkpoint", while --json sent the progress
	// writer to io.Discard and this struct had no such field. A soak failed on
	// CI reporting `"Pass":false` and NOTHING ELSE — every FAIL line naming
	// the cause had been thrown away at the one moment it mattered.
	//
	// Omitted on a passing run: the log is one line per action (500+ on the
	// pinned keystone), and nobody needs it when everything held.
	Report string `json:",omitempty"`
}

// SoakParticipants is the soak generator's fixed roster (task-5-brief.md:
// "dm + 2 players + agent") — exported so cmd/vtt/client_soak.go can mint
// exactly these invites via the SAME bootSelfContained/mintInvites path
// `vtt client run` already uses (harness_boot.go), keeping the participant
// list defined in exactly one place.
func SoakParticipants() []Participant {
	return []Participant{
		{Name: soakDM, Role: "dm"},
		{Name: soakPlayer1, Role: "player"},
		{Name: soakPlayer2, Role: "player"},
		{Name: soakAgent, Role: "agent"},
	}
}

const (
	soakDM      = "dm"
	soakPlayer1 = "player1"
	soakPlayer2 = "player2"
	soakAgent   = "agent"

	// soakObserverName is the participant whose continuously-drained
	// history serves as the checkpoint's "incremental" side (any of the
	// four would do — every accepted broadcast reaches all of them — dm is
	// picked arbitrarily and fixed for reproducibility of the report log).
	soakObserverName = soakDM
)

var soakPlayerNames = []string{soakPlayer1, soakPlayer2}

// soakAction names one mix bucket. String values double as SoakReport.Counts
// keys.
type soakAction string

const (
	actionCreateScene   soakAction = "createScene"
	actionAddActor      soakAction = "addActor"
	actionPlaceToken    soakAction = "placeToken"
	actionMoveOwn       soakAction = "moveOwn"
	actionSessionChurn  soakAction = "sessionChurn"
	actionRetraction    soakAction = "retraction"
	actionDeniedAttempt soakAction = "deniedAttempt"
)

// pickBucket maps a uniform [0,1) draw to a mix bucket per task-5-brief.md's
// action mix — create scene 5%, add actor 10%, place 15%, move-own 50%,
// session churn 5%, retraction 10%, deliberate authz-denied 5% (summing to
// 100%). A pure function of r: RunSoak's own same-seed-twice determinism
// obligation depends on every draw consulting nothing but the rng stream and
// the model state accumulated so far, never wall-clock or map-iteration
// order.
func pickBucket(r float64) soakAction {
	switch {
	case r < 0.05:
		return actionCreateScene
	case r < 0.15:
		return actionAddActor
	case r < 0.30:
		return actionPlaceToken
	case r < 0.80:
		return actionMoveOwn
	case r < 0.85:
		return actionSessionChurn
	case r < 0.95:
		return actionRetraction
	default:
		return actionDeniedAttempt
	}
}

// RunSoak dials SoakParticipants() (after=0), then issues cfg.Events
// actions in a seeded, mixed, authorized-by-construction sequence (see the
// package comment), checkpointing fold-equality every cfg.CheckEvery
// accepted actions and once more at the end. Human-readable progress is
// written to report as actions/checkpoints complete; report may be nil
// (io.Discard). The returned error is reserved for framework failures (a
// participant's dial failing, a send erroring, a fold erroring) —
// generator/assertion failures are reported through SoakReport.Pass, never
// as a non-nil error.
func RunSoak(ctx context.Context, cfg SoakConfig, dial Dialer, report io.Writer) (*SoakReport, error) {
	if report == nil {
		report = io.Discard
	}
	if cfg.Events <= 0 {
		return nil, fmt.Errorf("harness: soak: cfg.Events must be > 0, got %d", cfg.Events)
	}
	checkEvery := cfg.CheckEvery
	if checkEvery <= 0 {
		checkEvery = defaultCheckEvery
	}

	participants := SoakParticipants()
	participantNames := make([]string, 0, len(participants))
	conns := make(map[string]Conn, len(participants))
	for _, p := range participants {
		c, err := dial(p.Name, 0)
		if err != nil {
			return nil, fmt.Errorf("harness: soak: dial participant %q: %w", p.Name, err)
		}
		conns[p.Name] = c
		participantNames = append(participantNames, p.Name)
	}
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	// The fresh-campaign assumption (engine.go's errFreshCampaignRequired
	// doc comment applies here identically): checked once, on the fixed
	// observer participant, BEFORE any of the continuous per-participant
	// drain goroutines below start consuming Events() — every participant's
	// after=0 catch-up replays the same history, so one representative
	// check is sufficient.
	preExisting, err := drainPreExisting(conns[soakObserverName], denialAbsenceWindow)
	if err != nil {
		return nil, fmt.Errorf("harness: soak: cannot confirm a fresh campaign: %w", err)
	}
	if preExisting > 0 {
		return nil, errFreshCampaignRequired(preExisting)
	}

	histories := newSoakHistories()
	for _, p := range participants {
		histories.start(p.Name, conns[p.Name])
	}

	model := newSoakModel()
	// #nosec G404 -- a SEEDED, reproducible generator is the entire point: the
	// soak keystone pins seed-1 to exact accepted/denied/checkpoint counts.
	// crypto/rand would make the suite unreproducible.
	rng := rand.New(rand.NewSource(cfg.Seed))
	rep := &SoakReport{Events: cfg.Events, Counts: map[string]int{}, Pass: true}
	var lastAcceptedSeq int64
	// A QUEUE, not a single slot. Denied attempts run back to back over a few
	// hundred generated actions, and overwriting the slot silently DROPPED the
	// earlier denial's claim — worse here than in the scenario path, because
	// the per-participant drain goroutines fold a leaked envelope into
	// histories during the round trip to the next denial, so it is already
	// counted in that denial's own snapshot and invisible to it. Keeping every
	// outstanding denial, and settling against the EARLIEST snapshot, is what
	// makes the leak visible again.
	var pending []*deniedPending

	for i := 0; i < cfg.Events; i++ {
		// planRetraction's eligibility pool reads the agent's own observed
		// history (wire-only knowledge — see its doc comment), but that
		// history is appended by a background goroutine racing this loop
		// (histories.start): without waiting for it to have caught up to
		// the last CONFIRMED accepted sequence first, the exact set of
		// envelopes visible here would depend on goroutine-scheduling
		// timing rather than purely on prior actions' results — breaking
		// RunSoak's same-seed-twice determinism obligation (nothing new can
		// have been broadcast beyond lastAcceptedSeq yet, since this loop
		// never issues two commands concurrently, so waiting for exactly
		// that sequence yields a stable, deterministic snapshot).
		if lastAcceptedSeq > 0 {
			if !histories.waitFor(soakAgent, lastAcceptedSeq, observeTimeout) {
				rep.Pass = false
				fmt.Fprintf(report, "[action %d] FAIL: agent history never caught up to sequence %d within %s (planRetraction's eligibility snapshot may be stale)\n", i, lastAcceptedSeq, observeTimeout)
			}
		}
		agentHistory := histories.snapshot(soakAgent)
		step := model.planStep(rng, cfg.IDs, agentHistory)
		step.cmd.RequestId = fmt.Sprintf("soak-%d", i)

		conn, ok := conns[step.issuer]
		if !ok {
			return nil, fmt.Errorf("harness: soak: action %d: no connection for participant %q", i, step.issuer)
		}

		var preLens map[string]int
		if step.wantDenied {
			// Every participant's history must be caught up to the last
			// ACCEPTED sequence before this "before" snapshot is taken —
			// otherwise a prior command's broadcast still sitting in a
			// background drain goroutine's backlog (start's per-participant
			// goroutine races the main loop; nothing forces it to have
			// drained a legitimate broadcast before the NEXT action fires)
			// would finish draining during the denial-absence window below
			// and be mistaken for a leaked broadcast from THIS denied
			// command.
			if lastAcceptedSeq > 0 {
				if !histories.waitAllCaughtUp(participantNames, lastAcceptedSeq, observeTimeout) {
					rep.Pass = false
					fmt.Fprintf(report, "[action %d] FAIL: not every participant caught up to sequence %d within %s before the denial-attempt snapshot\n", i, lastAcceptedSeq, observeTimeout)
				}
			}
			preLens = histories.lengths()
		}

		result, err := conn.SendCommand(ctx, step.cmd)
		if err != nil {
			return nil, fmt.Errorf("harness: soak: action %d (%s by %s): send command: %w", i, step.kind, step.issuer, err)
		}
		rep.Counts[string(step.kind)]++

		switch {
		case step.wantDenied:
			rep.Denied++
			if result.Ok {
				rep.Pass = false
				fmt.Fprintf(report, "[action %d] by=%s kind=%s FAIL: want denied, got ok=true\n", i, step.issuer, step.kind)
				continue
			}
			if !strings.Contains(result.Error, "not authorized") {
				rep.Pass = false
				fmt.Fprintf(report, "[action %d] by=%s kind=%s FAIL: denied but error %q does not contain \"not authorized\"\n", i, step.issuer, step.kind, result.Error)
				continue
			}
			// Absence is settled by the NEXT accepted action's sequence
			// rather than by waiting — see leakedBelow. grewSince's
			// unconditional 300ms sleep ran on every denied action and, with
			// the scenario path, accounted for most of this package's
			// runtime, which is why it was excluded from check:mutation.
			pending = append(pending, &deniedPending{idx: i, issuer: step.issuer, kind: string(step.kind), lens: preLens})
			fmt.Fprintf(report, "[action %d] by=%s kind=%s denied as expected\n", i, step.issuer, step.kind)

		case !result.Ok:
			rep.Pass = false
			rep.Denied++
			fmt.Fprintf(report, "[action %d] by=%s kind=%s FAIL: unexpected denial: %s\n", i, step.issuer, step.kind, result.Error)

		default:
			if len(pending) > 0 {
				// The witness must have ARRIVED before its absence proves
				// anything. A leaked envelope carries a lower sequence, so
				// per-connection ordering delivers it FIRST — but "first" is
				// not "already": SendCommand returning means the server
				// accepted the command, not that any broadcast has reached a
				// participant's Events() channel. Settling immediately raced
				// the per-participant drain goroutines and could discard a
				// real leak, reporting Pass: true. waitFor's own doc comment
				// names this exact race; the denial snapshot above already
				// guards against it and the settle did not.
				//
				// This costs nothing on a healthy run: the witness event is
				// one the server just accepted, so it is already in flight.
				if !histories.waitAllCaughtUp(participantNames, result.Sequence, observeTimeout) {
					rep.Pass = false
					fmt.Fprintf(report, "[action %d] FAIL: not every participant observed the witness event %d within %s, so the preceding denied action's absence claim could not be settled\n",
						i, result.Sequence, observeTimeout)
				}
				if leaked := histories.leakedBelow(pending[0].lens, result.Sequence); len(leaked) > 0 {
					rep.Pass = false
					fmt.Fprint(report, deniedLeakLine(pending, leaked))
				}
				pending = nil
			}
			step.apply(result.Sequence)
			rep.Accepted++
			lastAcceptedSeq = result.Sequence
			fmt.Fprintf(report, "[action %d] by=%s kind=%s accepted (sequence %d)\n", i, step.issuer, step.kind, result.Sequence)

			if rep.Accepted%checkEvery == 0 {
				if err := runSoakCheckpoint(rep, histories, dial, lastAcceptedSeq, report); err != nil {
					return nil, err
				}
			}
		}
	}

	// Denials with no accepted action after them have no ordering witness, so
	// they fall back to the original bounded wait. Only a TRAILING RUN of
	// denials can reach this, so the wait is paid once per soak.
	if len(pending) > 0 {
		settleTrailingDenials(rep, histories, pending, report)
	}

	if err := runSoakCheckpoint(rep, histories, dial, lastAcceptedSeq, report); err != nil {
		return nil, err
	}

	return rep, nil
}

// settleTrailingDenials resolves the denials that have no accepted action
// after them, the only ones that fall back to a bounded wait. Extracted from
// RunSoak rather than inlined: the second case below pushed that function past
// the gocyclo limit, and a quality gate is not weakened to fit a change.
//
// Three outcomes, not two. A history that GREW is a leak. A history that
// stopped growing because its stream ENDED is not evidence of anything —
// grewSince cannot tell that from a quiet connection by length alone, which is
// why it reports the two separately. Proving a negative by silence requires a
// connection that could have spoken.
func settleTrailingDenials(rep *SoakReport, histories *soakHistories, pending []*deniedPending, report io.Writer) {
	leaked, ended := histories.grewSince(pending[0].lens, denialAbsenceWindow)
	switch {
	case len(leaked) > 0:
		rep.Pass = false
		fmt.Fprint(report, deniedLeakLine(pending, leaked))
	case len(ended) > 0:
		rep.Pass = false
		fmt.Fprintf(report, "[soak] FAIL: denial absence unprovable — the stream ended for %s, "+
			"so their silence is not evidence that the %d trailing denied command(s) broadcast nothing\n",
			strings.Join(ended, ", "), len(pending))
	}
}

// deniedLeakLine formats a leak against the outstanding denied actions.
// Nothing orders consecutive denials relative to each other — they produce no
// events — so the EARLIEST carries the failure and the line discloses the
// range instead of implying the leak was pinned to one action.
func deniedLeakLine(pending []*deniedPending, leaked []string) string {
	first := pending[0]
	line := fmt.Sprintf("[action %d] by=%s kind=%s FAIL: unexpected broadcast observed by %s after a denied command",
		first.idx, first.issuer, first.kind, strings.Join(leaked, ", "))
	if len(pending) > 1 {
		line += fmt.Sprintf(" (one of the denied actions %d-%d produced it; no accepted event separates them)",
			first.idx, pending[len(pending)-1].idx)
	}
	return line + "\n"
}

// deniedPending is a denied action whose "no broadcast" claim is waiting on
// the next accepted action's sequence to settle it.
type deniedPending struct {
	idx    int
	issuer string
	kind   string
	lens   map[string]int
}

// runSoakCheckpoint folds the observer's incrementally-drained history,
// folds a FRESH catch-up (a brand-new second connection, dialed after=0,
// drained to quiescence, then closed), and compares the two via
// statesEqual — RunSoak's central, repeated proof (see the package
// comment). waitForSeq > 0 makes the observer's history wait to include
// that sequence first (bounded by observeTimeout), closing the race between
// SendCommand returning and that same command's OWN broadcast reaching
// Events() a beat later (client.go's SendCommand doc comment).
func runSoakCheckpoint(rep *SoakReport, histories *soakHistories, dial Dialer, waitForSeq int64, report io.Writer) error {
	if waitForSeq > 0 && !histories.waitFor(soakObserverName, waitForSeq, observeTimeout) {
		rep.Pass = false
		fmt.Fprintf(report, "[checkpoint %d] FAIL: observer %q never caught up to sequence %d within %s\n", rep.Checkpoints, soakObserverName, waitForSeq, observeTimeout)
	}

	incremental := histories.snapshot(soakObserverName)
	incrementalState, err := Fold(incremental)
	if err != nil {
		return fmt.Errorf("harness: soak: checkpoint %d: fold incremental history: %w", rep.Checkpoints, err)
	}

	// Drain to a KNOWN HEAD, not to a silence.
	//
	// This used to be drainQuiescentEnvelopes(..., denialAbsenceWindow): stop
	// once 300ms passes with no arrival, treating quiet as "catch-up
	// finished". Mid-replay on a loaded CI runner a 300ms gap is ordinary, so
	// the fresh history truncated — 257 envelopes against the observer's 480 —
	// and the comparison below then reported
	//
	//     incremental fold (480 events) != fresh catch-up fold (257 events)
	//
	// which is this repo's KEYSTONE claim (rebuild == live) failing. It was
	// not failing. A false alarm there is worse than a flake: it sends someone
	// hunting a fold divergence that does not exist, and it discredits the one
	// check the event-sourcing design rests on.
	//
	// The observer's own history gives a target the wire protocol does not:
	// every sequence it has seen, the fresh connection must also see. So wait
	// for THAT, and keep the silence window only as the signal that the tail
	// after the target has settled.
	target := highestSequence(incremental)
	freshHistory, reached := drainFreshCatchUp(dial, target, denialAbsenceWindow, freshCatchUpTimeout)
	if !reached {
		// Reported as what it is. The fold comparison still runs below, but
		// this line is what names the cause when it disagrees.
		//
		// WHY, not just THAT. A closed Events() channel looks identical to a
		// finished replay, so this used to print the 30s timeout for every
		// short read — including a connection torn down for buffer overflow in
		// under three seconds, which sent the reader after slow CI while the
		// real cause was that this consumer could not keep up. CloseErr is the
		// only thing that can tell them apart.
		rep.Pass = false
		fmt.Fprintf(report, "[checkpoint %d] FAIL: fresh catch-up never reached sequence %d within %s total across up to %d dials — drained %d envelopes (head %d), so any fold difference below is a TRUNCATED replay, not a divergence\n",
			rep.Checkpoints, target, freshCatchUpTimeout, freshCatchUpAttempts, len(freshHistory), highestSequence(freshHistory))
	}

	freshState, err := Fold(freshHistory)
	if err != nil {
		return fmt.Errorf("harness: soak: checkpoint %d: fold fresh catch-up: %w", rep.Checkpoints, err)
	}

	rep.Checkpoints++
	if !statesEqual(incrementalState, freshState) {
		rep.Pass = false
		fmt.Fprintf(report, "[checkpoint %d] FAIL: incremental fold (%d events) != fresh catch-up fold (%d events)\n", rep.Checkpoints, len(incremental), len(freshHistory))
		return nil
	}
	fmt.Fprintf(report, "[checkpoint %d] pass: incremental fold == fresh catch-up fold (%d events)\n", rep.Checkpoints, len(freshHistory))
	return nil
}

// freshCatchUpTimeout bounds the wait for a checkpoint's fresh connection to
// replay up to the observer's head. Generous on purpose: it is a BACKSTOP
// against a genuinely stuck replay, not a measurement, and a catch-up that
// keeps up never reaches it. The value it replaced was an implicit 300ms —
// the silence window — which is why a slow runner looked like a fold bug.
const freshCatchUpTimeout = 30 * time.Second

// freshCatchUpAttempts bounds the overflow-resume loop. Each attempt must make
// forward progress or the loop stops anyway, so this is a backstop against a
// connection that overflows instantly and forever, not a retry budget.
const freshCatchUpAttempts = 8

// highestSequence returns the largest sequence in a history, or 0 if empty.
func highestSequence(envs []*vttv1.Envelope) int64 {
	var head int64
	for _, e := range envs {
		if e.GetSequence() > head {
			head = e.GetSequence()
		}
	}
	return head
}

// drainFreshCatchUp replays the whole log into a fresh connection, RESUMING
// across the overflow disconnects the client is designed to produce.
//
// eventBuffer bounds Events() at the same 256 the gateway uses, and
// deliverEvent tears the connection down rather than blocking when it fills —
// deliberately, since this side cannot tell the server to slow down. The
// documented recovery is the CALLER'S: "Dial again with a fresh `after`
// cursor." This is the soak implementing that contract. NOTE it covers the
// CLIENT-side overflow only: a store/gateway-side overflow closes the socket
// outright and surfaces as a plain read error, which this deliberately does
// not resume from. Unreachable here (Store.Subscribe sizes its channel to
// len(history)+buffer, and the soak's main loop is blocked during a
// checkpoint), but the distinction is real.
//
// It did not, before 2026-08-04. It dialled once, was disconnected at 257
// envelopes of 480 (the buffer plus the one in flight), and reported the short
// history as "incremental fold (480) != fresh catch-up fold (257)" — rebuild
// != live, the keystone claim of the whole event-sourcing design, appearing to
// fail when it had not. A false alarm there is worse than a flake: it sends
// someone hunting a fold divergence that does not exist.
//
// The resume cursor is the last sequence actually RECEIVED, not the target:
// re-dialling from 0 would double-count and from anything ahead would lose
// envelopes silently. Attempts are bounded so a connection that overflows
// immediately, forever, fails as a real failure rather than spinning.
func drainFreshCatchUp(dial Dialer, target int64, window, timeout time.Duration) ([]*vttv1.Envelope, bool) {
	var all []*vttv1.Envelope
	after := int64(0)
	// ONE budget for the whole catch-up, not one per attempt. Per-attempt,
	// eight attempts could spend 8*30s inside a single checkpoint and five
	// checkpoints could exceed `go test`'s 10-minute package default -- which
	// would replace an honest "[checkpoint N] FAIL" line with `panic: test
	// timed out` and no SoakReport at all, the same class of confusing failure
	// this function exists to remove. The attempt cap is the backstop; the
	// clock is the bound.
	deadline := time.Now().Add(timeout)
	for attempt := 0; attempt < freshCatchUpAttempts; attempt++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return all, false
		}
		conn, err := dial(soakObserverName, after)
		if err != nil {
			return all, false
		}
		got, reached := drainToSequence(conn.Events(), target, window, remaining)
		overflowed := errors.Is(conn.CloseErr(), ErrEventsOverflow)
		_ = conn.Close()

		all = append(all, got...)
		if reached {
			return all, true
		}
		if !overflowed {
			// Ended for some other reason (or simply ran out of time); a
			// re-dial would not change that and would only mask it.
			return all, false
		}
		next := highestSequence(all)
		if next <= after {
			// No forward progress — re-dialling the same cursor would spin.
			return all, false
		}
		after = next
	}
	return all, false
}

// drainToSequence collects envelopes until the history has reached `head` AND
// then gone quiet for `window`, or until `timeout` elapses. The bool reports
// whether `head` was reached.
//
// The two conditions are both needed. Waiting only for `head` would stop mid
// tail and drop envelopes that arrive after it; waiting only for quiet is the
// bug this replaced. A head of 0 (an empty observer history) is reached
// immediately, which degrades this to the old quiescent drain — correctly, as
// there is then nothing to catch up to.
func drainToSequence(events <-chan *vttv1.Envelope, head int64, window, timeout time.Duration) ([]*vttv1.Envelope, bool) {
	var out []*vttv1.Envelope
	deadline := time.After(timeout)
	reached := head == 0
	for {
		select {
		case env, ok := <-events:
			if !ok {
				return out, reached
			}
			out = append(out, env)
			if env.GetSequence() >= head {
				reached = true
			}
		case <-time.After(window):
			// Quiet. Done only if the target is already in hand; otherwise the
			// replay is still coming and this is exactly the gap that used to
			// end the drain early.
			if reached {
				return out, true
			}
		case <-deadline:
			return out, reached
		}
	}
}

// statesEqual mirrors internal/campaign/scenario_test.go's own statesEqual
// (same semantics, independently duplicated here — the harness may not
// import campaign even in test code, let alone production code; see
// client.go's package comment): plain equality for Scenes/Tokens/Sessions,
// proto.Equal per actor for Actors (protobuf messages carry unexported
// state that reflect.DeepEqual does not compare correctly — proto.Equal is
// the documented way; Conditions and Notes, like Scenes/Tokens/Sessions, are
// plain struct maps compared with reflect.DeepEqual).
func statesEqual(a, b *engine.State) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !reflect.DeepEqual(a.Scenes, b.Scenes) {
		return false
	}
	if !reflect.DeepEqual(a.Tokens, b.Tokens) {
		return false
	}
	if !reflect.DeepEqual(a.Sessions, b.Sessions) {
		return false
	}
	if !reflect.DeepEqual(a.Conditions, b.Conditions) {
		return false
	}
	if !reflect.DeepEqual(a.Notes, b.Notes) {
		return false
	}
	if len(a.Actors) != len(b.Actors) {
		return false
	}
	for id, av := range a.Actors {
		bv, ok := b.Actors[id]
		if !ok || !proto.Equal(av, bv) {
			return false
		}
	}
	return true
}

// --- continuously-drained per-participant history ---------------------------

// soakHistories accumulates every envelope each dialed participant observes
// over its OWN connection, for the full lifetime of that connection — begun
// immediately at dial time (see start), the same obligation any honest
// continuous client has (client.go's eventBuffer doc comment: a consumer
// that falls behind for 256 envelopes gets torn down with
// ErrEventsOverflow). This serves three purposes at once: it is what keeps
// every one of the four long-lived connections drained (never overflowing)
// across a long run; it is the "own observed events" source
// planRetraction's wire-only eligibility pool reads (agent's history); and
// it is the checkpoint's "incremental" side (the observer's history).
type soakHistories struct {
	mu   sync.Mutex
	data map[string][]*vttv1.Envelope
	// ended records, per participant, why its drain goroutine stopped. A
	// participant whose stream is gone contributes SILENCE to every later
	// absence check, and silence from a dead connection is not evidence —
	// see grewSince.
	ended map[string]error
}

func newSoakHistories() *soakHistories {
	return &soakHistories{
		data:  map[string][]*vttv1.Envelope{},
		ended: map[string]error{},
	}
}

// start launches the one goroutine that drains name's connection for as
// long as it stays open (c.Events() closes on disconnect — see client.go's
// Events doc comment — which ends this goroutine too).
func (h *soakHistories) start(name string, c Conn) {
	go func() {
		for env := range c.Events() {
			h.mu.Lock()
			h.data[name] = append(h.data[name], env)
			h.mu.Unlock()
		}
		// The range ended, so the stream is gone. Recorded rather than merely
		// returned: this participant's history can no longer grow, and an
		// absence check that reads that stillness as "nothing was broadcast"
		// is proving a negative against a connection that could not speak.
		h.mu.Lock()
		h.ended[name] = streamEndReason(c)
		h.mu.Unlock()
	}()
}

// snapshot returns a point-in-time copy of name's accumulated history —
// safe for the caller to read without racing the still-running drain
// goroutine.
func (h *soakHistories) snapshot(name string) []*vttv1.Envelope {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*vttv1.Envelope(nil), h.data[name]...)
}

// lengths snapshots every participant's history length at once — the
// "before" side of grewSince's leaked-broadcast check.
func (h *soakHistories) lengths() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]int, len(h.data))
	for name, envs := range h.data {
		out[name] = len(envs)
	}
	return out
}

// leakedBelow reports (sorted) which participants received an envelope with a
// sequence BELOW witnessSeq since the snapshot in before — an event that was
// already in the log before the next accepted action's own, and therefore
// produced by the denied action that preceded it.
//
// This replaces grewSince's unconditional sleep on the denial path. Per
// connection, delivery is sequence-ordered and the gateway broadcasts only
// from the store subscription, so anything a denied command wrongly produced
// must arrive before the next accepted event. Finding none by then is a proof,
// not a timeout: no wait, and it holds however slow the server is.
func (h *soakHistories) leakedBelow(before map[string]int, witnessSeq int64) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var leaked []string
	for name, envs := range h.data {
		for _, env := range envs[min(before[name], len(envs)):] {
			if env.GetSequence() < witnessSeq {
				leaked = append(leaked, name)
				break
			}
		}
	}
	sort.Strings(leaked)
	return leaked
}

// grewSince waits window, then reports (sorted) which participants' history
// is longer now than the lengths captured in before — engine.go's
// drainAllForSilence semantics, reused here over the histories this type
// already continuously maintains instead of a fresh per-call goroutine
// race.
func (h *soakHistories) grewSince(before map[string]int, window time.Duration) (grewNames, endedNames []string) {
	time.Sleep(window)
	after := h.lengths()
	var grew []string
	for name, n := range before {
		if after[name] > n {
			grew = append(grew, name)
		}
	}
	sort.Strings(grew)

	// Independent of `before`: a torn-down participant invalidates the
	// absence claim whether or not it had accumulated history first.
	h.mu.Lock()
	ended := make([]string, 0, len(h.ended))
	for name := range h.ended {
		ended = append(ended, name)
	}
	h.mu.Unlock()
	sort.Strings(ended)

	return grew, ended
}

// waitFor blocks (bounded by timeout) until name's history contains an
// envelope with the given sequence, returning false on timeout — closes the
// race between SendCommand returning and that command's own broadcast
// reaching the SAME connection's Events() a beat later.
func (h *soakHistories) waitFor(name string, seq int64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		h.mu.Lock()
		for _, env := range h.data[name] {
			if env.Sequence == seq {
				h.mu.Unlock()
				return true
			}
		}
		h.mu.Unlock()
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitAllCaughtUp blocks (bounded by timeout) until EVERY name in names has
// an envelope with the given sequence in its history — the multi-participant
// form of waitFor, used before a denied-attempt step's "before" snapshot
// (RunSoak's main loop) so a prior command's broadcast still draining
// through one participant's background goroutine is never mistaken for a
// leak from the denied command that follows it.
func (h *soakHistories) waitAllCaughtUp(names []string, seq int64, timeout time.Duration) bool {
	for _, name := range names {
		if !h.waitFor(name, seq, timeout) {
			return false
		}
	}
	return true
}

// --- the generator's model ---------------------------------------------------

// soakStep is one planned action: who issues it, the command itself, its
// mix-bucket kind, whether it is EXPECTED to be denied (the deliberate
// authz-denied bucket — the only bucket where that's true), and apply — a
// closure that commits this step's bookkeeping into the model. RunSoak
// calls apply ONLY after confirming the wire actually accepted the command
// (result.Ok), so the model never assumes success ahead of the ground truth
// the server returns (the same discipline internal/campaign/property_test.go's
// doUndo uses for its own not-guaranteed-to-succeed action).
type soakStep struct {
	issuer     string
	cmd        *vttv1.ClientCommand
	kind       soakAction
	wantDenied bool
	apply      func(seq int64)
}

// soakModel tracks just enough shape of the world (mirroring
// internal/campaign/property_test.go's propModel, adapted for the soak's
// fixed 4-participant roster and role-aware ownership) to generate only
// forward-valid commands: which scenes/actors/tokens exist, which actor (if
// any) each player controls, and current session-open state.
type soakModel struct {
	scenes []string
	actors []string
	// actorController maps actorID -> the participant NAME that controls
	// it ("" = uncontrolled, DM/agent-only — Actor.controller_id's own
	// doc comment).
	actorController map[string]string

	tokenIDs   []string
	tokenActor map[string]string
	tokenPos   map[string][2]int32

	sessionOpen bool

	sceneN, actorN, tokenN int

	// playerControlledActor records that a player's addActor assignment has
	// already been ATTEMPTED (see planAddActor) — keyed regardless of
	// whether cfg.IDs actually supplied a real id for them, so a missing id
	// degrades that one player's ownership coverage without starving the
	// other player of their own turn.
	playerControlledActor map[string]bool

	// retracted marks sequences already retracted, so planRetraction never
	// offers the same one twice.
	retracted map[int64]bool
}

func newSoakModel() *soakModel {
	return &soakModel{
		actorController:       map[string]string{},
		tokenActor:            map[string]string{},
		tokenPos:              map[string][2]int32{},
		playerControlledActor: map[string]bool{},
		retracted:             map[int64]bool{},
	}
}

func (m *soakModel) canPlaceToken() bool { return len(m.scenes) > 0 && len(m.actors) > 0 }

// planStep decides the action for one draw: it consumes exactly one
// rng.Float64() call to pick the mix bucket (pickBucket), then
// (bucket-dependent) zero or more further rng draws to pick WHICH
// scene/actor/token/participant — the same draw sequence, given the same
// model state, on every call with the same seed (RunSoak's own determinism
// obligation). agentHistory is a point-in-time snapshot of every envelope
// the "agent" participant has ITSELF observed over the wire (planRetraction's
// "own observed events, wire-only knowledge" requirement) — never the
// model's a priori bookkeeping. When a bucket's precondition isn't met
// (nothing to place on top of, no tokens to move, nothing eligible to
// retract or deny), this falls back to addActor — always valid — the same
// "guarantee forward progress" shape property_test.go's step() uses.
func (m *soakModel) planStep(rng *rand.Rand, ids map[string]string, agentHistory []*vttv1.Envelope) soakStep {
	switch pickBucket(rng.Float64()) {
	case actionCreateScene:
		return m.planCreateScene(rng)
	case actionAddActor:
		return m.planAddActor(rng, ids)
	case actionPlaceToken:
		if m.canPlaceToken() {
			return m.planPlaceToken(rng)
		}
	case actionMoveOwn:
		if step, ok := m.planMoveOwn(rng); ok {
			return step
		}
	case actionSessionChurn:
		return m.planSessionChurn(rng)
	case actionRetraction:
		if step, ok := m.planRetraction(rng, agentHistory); ok {
			return step
		}
	case actionDeniedAttempt:
		if step, ok := m.planDeniedAttempt(rng); ok {
			return step
		}
	}
	return m.planAddActor(rng, ids)
}

// pickDMOrAgent draws one bit to choose between the two lifecycle-authorized
// issuers (dm, agent) — reused by every bucket where either is valid.
func pickDMOrAgent(rng *rand.Rand) string {
	if rng.Intn(2) == 0 {
		return soakDM
	}
	return soakAgent
}

func (m *soakModel) planCreateScene(rng *rand.Rand) soakStep {
	m.sceneN++
	id := fmt.Sprintf("soak-scn-%d", m.sceneN)
	issuer := pickDMOrAgent(rng)
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_CreateScene{CreateScene: &vttv1.CreateScene{
		SceneId: id, Name: id, GridWidth: 30, GridHeight: 30,
	}}}
	return soakStep{issuer: issuer, cmd: cmd, kind: actionCreateScene, apply: func(int64) {
		m.scenes = append(m.scenes, id)
	}}
}

// planAddActor assigns the FIRST two addActor draws (in whatever order they
// land across the whole run — deterministic per seed, not a fixed prefix of
// the run) to player1 then player2's controller_id, in that priority order;
// every subsequent addActor stays uncontrolled (NPC). This guarantees both
// players eventually get a controllable actor without the mix needing a
// dedicated "assign ownership" bucket of its own.
func (m *soakModel) planAddActor(rng *rand.Rand, ids map[string]string) soakStep {
	m.actorN++
	id := fmt.Sprintf("soak-actor-%d", m.actorN)
	issuer := pickDMOrAgent(rng)

	controllerName := ""
	for _, p := range soakPlayerNames {
		if !m.playerControlledActor[p] {
			controllerName = p
			break
		}
	}
	var controllerID string
	if controllerName != "" {
		controllerID = ids[controllerName]
	}

	actor := &vttv1.Actor{ActorId: id, Name: id}
	if controllerID != "" {
		actor.ControllerId = controllerID
	}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_AddActor{AddActor: &vttv1.AddActor{Actor: actor}}}
	return soakStep{issuer: issuer, cmd: cmd, kind: actionAddActor, apply: func(int64) {
		m.actors = append(m.actors, id)
		m.actorController[id] = ""
		if controllerName != "" {
			m.playerControlledActor[controllerName] = true
			if controllerID != "" {
				m.actorController[id] = controllerName
			}
		}
	}}
}

func (m *soakModel) planPlaceToken(rng *rand.Rand) soakStep {
	m.tokenN++
	id := fmt.Sprintf("soak-tok-%d", m.tokenN)
	issuer := pickDMOrAgent(rng)
	scene := m.scenes[rng.Intn(len(m.scenes))]
	actor := m.actors[rng.Intn(len(m.actors))]
	// #nosec G115 -- rng.Intn(50) is bounded to 0..49; int32 cannot overflow.
	x, y := int32(rng.Intn(50)), int32(rng.Intn(50))
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_PlaceToken{PlaceToken: &vttv1.PlaceToken{
		TokenId: id, SceneId: scene, ActorId: actor, Position: &vttv1.GridPosition{X: x, Y: y},
	}}}
	return soakStep{issuer: issuer, cmd: cmd, kind: actionPlaceToken, apply: func(int64) {
		m.tokenIDs = append(m.tokenIDs, id)
		m.tokenActor[id] = actor
		m.tokenPos[id] = [2]int32{x, y}
	}}
}

// tokensControlledBy returns every token whose actor's controller is
// exactly player — the player-ownership-restricted eligibility pool
// moveOwn and deniedAttempt both read from (in opposite senses).
func (m *soakModel) tokensControlledBy(player string) []string {
	var out []string
	for _, tok := range m.tokenIDs {
		if m.actorController[m.tokenActor[tok]] == player {
			out = append(out, tok)
		}
	}
	return out
}

// planMoveOwn picks uniformly among every currently-eligible (issuer,
// token-pool) pair — dm/agent (any token, no ownership restriction — only
// RolePlayer is ownership-checked, gateway/authz.go's Authorize) and each
// player with at least one controlled token — then a token from that pool.
// Returns ok=false when no participant has anything eligible to move yet
// (no tokens exist at all).
func (m *soakModel) planMoveOwn(rng *rand.Rand) (soakStep, bool) {
	type pool struct {
		issuer string
		tokens []string
	}
	var options []pool
	if len(m.tokenIDs) > 0 {
		options = append(options, pool{soakDM, m.tokenIDs}, pool{soakAgent, m.tokenIDs})
	}
	for _, p := range soakPlayerNames {
		if toks := m.tokensControlledBy(p); len(toks) > 0 {
			options = append(options, pool{p, toks})
		}
	}
	if len(options) == 0 {
		return soakStep{}, false
	}
	choice := options[rng.Intn(len(options))]
	tok := choice.tokens[rng.Intn(len(choice.tokens))]
	// #nosec G115 -- rng.Intn(50) is bounded to 0..49; int32 cannot overflow.
	x, y := int32(rng.Intn(50)), int32(rng.Intn(50))
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: tok, To: &vttv1.GridPosition{X: x, Y: y},
	}}}
	return soakStep{issuer: choice.issuer, cmd: cmd, kind: actionMoveOwn, apply: func(int64) {
		m.tokenPos[tok] = [2]int32{x, y}
	}}, true
}

func (m *soakModel) planSessionChurn(rng *rand.Rand) soakStep {
	issuer := pickDMOrAgent(rng)
	if !m.sessionOpen {
		cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_StartSession{StartSession: &vttv1.StartSession{Name: "soak-session"}}}
		return soakStep{issuer: issuer, cmd: cmd, kind: actionSessionChurn, apply: func(int64) { m.sessionOpen = true }}
	}
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_EndSession{EndSession: &vttv1.EndSession{}}}
	return soakStep{issuer: issuer, cmd: cmd, kind: actionSessionChurn, apply: func(int64) { m.sessionOpen = false }}
}

// planRetraction picks a random SAFE TokenMoved sequence from the agent's
// OWN observed wire history (agentHistory — never the model's a priori
// bookkeeping, per the brief's "wire-only knowledge; P1" requirement) that
// hasn't already been retracted. Every candidate is safe unconditionally:
// retracting a single TokenMoved never orphans anything a later event
// depends on (unlike, say, a SessionEnded a later SessionStarted needs) —
// campaign.Undo's own dry-run viability check (internal/campaign/campaign.go)
// backs this, so — unlike internal/campaign/property_test.go's doUndo, which
// must tolerate rejection for its GENERIC any-non-marker-sequence
// eligibility pool — this generator never needs to handle a retraction
// rejection as anything but a framework-level surprise.
func (m *soakModel) planRetraction(rng *rand.Rand, agentHistory []*vttv1.Envelope) (soakStep, bool) {
	var candidates []int64
	for _, env := range agentHistory {
		if _, ok := env.Payload.(*vttv1.Envelope_TokenMoved); ok && !m.retracted[env.Sequence] {
			candidates = append(candidates, env.Sequence)
		}
	}
	if len(candidates) == 0 {
		return soakStep{}, false
	}
	seq := candidates[rng.Intn(len(candidates))]
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_RetractEvents{RetractEvents: &vttv1.RetractEvents{
		FromSequence: seq, ToSequence: seq, Reason: "soak retraction",
	}}}
	return soakStep{issuer: soakAgent, cmd: cmd, kind: actionRetraction, apply: func(int64) {
		m.retracted[seq] = true
	}}, true
}

// planDeniedAttempt picks a random player and a token NOT controlled by
// them (an NPC token, or the OTHER player's token) and issues a moveToken
// as that player — the deliberate authz-denied 5% (task-5-brief.md):
// asserted ok=false + no broadcast by RunSoak's caller, never actually
// applied to the model. Returns ok=false when no such (player, foreign
// token) pairing exists yet.
func (m *soakModel) planDeniedAttempt(rng *rand.Rand) (soakStep, bool) {
	player := soakPlayerNames[rng.Intn(len(soakPlayerNames))]
	var candidates []string
	for _, tok := range m.tokenIDs {
		if m.actorController[m.tokenActor[tok]] != player {
			candidates = append(candidates, tok)
		}
	}
	if len(candidates) == 0 {
		return soakStep{}, false
	}
	tok := candidates[rng.Intn(len(candidates))]
	// #nosec G115 -- rng.Intn(50) is bounded to 0..49; int32 cannot overflow.
	x, y := int32(rng.Intn(50)), int32(rng.Intn(50))
	cmd := &vttv1.ClientCommand{Command: &vttv1.ClientCommand_MoveToken{MoveToken: &vttv1.MoveTokenRequest{
		TokenId: tok, To: &vttv1.GridPosition{X: x, Y: y},
	}}}
	return soakStep{issuer: player, cmd: cmd, kind: actionDeniedAttempt, wantDenied: true, apply: func(int64) {}}, true
}
