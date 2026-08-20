package gateway

import (
	"context"
	"log/slog"
	"sync"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// viewerFor is the seat a participant occupies, as the projection sees it.
//
// Viewpoint is deliberately empty, and it STAYS empty until the spectator
// themselves names a shoulder over SetViewpoint (seat.perch, below). A
// connection therefore opens perched on nobody: eyes returns no eyes, and the
// watcher sees no board at all until they choose one.
//
// That is the fail-closed direction (spec §4.4) and it is the only direction
// available here, because the server has no way to know which shoulder this
// person meant. A default chosen for them would be a shoulder nobody asked to
// ride, and Viewer's own doc comment records why the wrong shoulder undoes
// this whole arc in one click.
//
// IT IS ALSO WHY A PERCH DOES NOT SURVIVE A RECONNECT (spec §3.1.1): the perch
// is connection state, like the catch-up point, so a client that had one
// re-sends it after redialling.
func viewerFor(p *identity.Participant) Viewer {
	return Viewer{ParticipantID: p.ID, Role: p.Role}
}

// projected reports whether this seat's stream is filtered.
//
// FALSE FOR EXACTLY TWO ROLES and true for everything else, including a role
// this build has never heard of. identity.Role is a string in a database row,
// so the set is open; naming the two that receive the log unfiltered (spec
// §3.1, exit criterion 8) and projecting the rest is the only direction that
// stays closed when the set grows.
//
// IT IS NOT THE SECURITY BOUNDARY, and saying so plainly is worth more than
// the reassurance of pretending otherwise. Projector.Project switches on the
// role again and hands the DM and the agent the identity projection whatever
// this function says, so making this return true for them changes no byte on
// any wire — MEASURED, by making exactly that change and watching the suite
// stay green. What it decides is the SUBSCRIPTION and the COST: a projected
// seat replays from 0 and folds the log so far on every event, and an
// unprojected one resumes where it asked to and folds nothing. The direction
// that matters is the other one — a PLAYER wrongly answered false here gets
// seat.pr == nil and therefore the whole log, which is the leak, and
// TestSessionZeroCannotHappenAgain fails the moment it does.
func projected(r identity.Role) bool {
	switch r {
	case identity.RoleDM, identity.RoleAgent:
		return false
	}
	return true
}

// seat is one connection's view of the log: what it may receive, and where it
// is resuming from.
//
// THE PROJECTOR IS FED THE LOG FROM THE BEGINNING AND ITS OUTPUT IS DISCARDED
// UP TO THE RESUME POINT. That is a hard requirement, not an optimisation
// choice made the other way round, and Task 4 proved it rather than asserting
// it: a Projector's maps are the accumulator of a fold over the log PREFIX,
// and the actor roster is path-dependent in a way no snapshot can reconstruct
// — an NPC seen at seq 7 and hidden at seq 8 is still in the viewer's roster
// at 8, while any function of state-at-8 says it is unknown. A projector
// CONSTRUCTED at a resume point therefore leaks in the direction this arc
// exists to close: a token that left view during the gap gets no TokenHidden
// and stays on the reconnecting player's board.
// project_test.go's TestAReconnectingSeatIsCaughtUpToExactlyWhatItMissed
// builds a projector the forbidden way and requires that it corrupts the seat.
//
// So serve subscribes a projected seat from 0 regardless of the `after` it
// asked for, and `resume` throws away what that seat already holds.
// ONE GOROUTINE OWNS A SEAT, and there is no lock here because of it. serve
// builds it and drains its catch-up; from the moment the pump goroutine starts,
// the pump is the only thing that touches it — receive per event, perch when a
// spectator hops. The command goroutine hands a shoulder ACROSS A CHANNEL
// rather than reaching in.
//
// A mutex was tried and was the wrong tool: it made the fields safe and left
// the ORDER of what reached the socket to chance, because each goroutine
// computed its batch under the lock and sent it after releasing. Serialising
// the memory without serialising the emission is the bug, not the fix. If a
// second goroutine ever needs a seat, give it a channel, not a lock.
type seat struct {
	// pr is nil for the DM and the agent. Not "a projector that happens to
	// forward everything": nil, so that neither the fold below nor
	// sight.VisibleFrom is ever reached on their path, and their stream stays
	// byte-for-byte what it is today at no cost at all.
	pr *Projector
	// resume is the sequence the client says it already has. Output at or
	// below it is dropped; INPUT at or below it is still folded, because that
	// is what builds the projector's memory of what this seat has been shown.
	resume int64
	// received is every envelope this seat's projector has been fed, kept
	// because the state each event produced is a fold of the log so far and a
	// retraction is retroactive (see campaign.FoldPrefix).
	//
	// This is per-connection state, and the pump's own comment used to say it
	// had none. It is bounded by the log, holds no locks and no sockets, and
	// is dropped with the connection; it is the same log a browser client
	// keeps for the same reason (client/src/session.ts re-folds its whole log
	// on every event).
	received []*vttv1.Envelope

	// world is the state this seat last judged an event against, which is what
	// lets perch answer "what can these new eyes see" against exactly the world
	// the old eyes were last shown — not against HEAD, which during catch-up is
	// the future (receive's own comment says why that matters).
	//
	// Nil until the first event folds: a seat that has been shown nothing has
	// nothing to re-show, whatever shoulder it climbs onto.
	world *engine.State
}

// perchBox is the one slot a spectator's chosen shoulder travels in, from the
// command goroutine to the pump that owns the seat.
//
// LATEST WINS, and the coalescing is the point rather than a side effect. A
// perch is a SETTING — which shoulder am I on — not an operation to be queued,
// so a shoulder that was never applied has no trace to leave: transitions
// computes its frames as a diff against what the viewer has ALREADY been shown,
// so going straight to the newest shoulder emits exactly what going through
// every intermediate one would have converged on.
//
// It is also what keeps a hopping spectator from taxing the table. MEASURED:
// with the perch applied on the pump and every hop queued, forty hops sent as
// fast as a socket allows made a DM's own command time out (>3s) once in every
// four runs; at five hops it never did, so the cost scaled with the number of
// re-projections rather than being a stall. Coalescing bounds that work by the
// pump's speed instead of by the sender's.
//
// The mutex here is not the one seat's comment warns about: it guards a single
// string being HANDED ACROSS, never the order anything reaches the socket.
type perchBox struct {
	mu       sync.Mutex
	shoulder string
	full     bool
	// wake has capacity 1 and carries no data. A signal already waiting is a
	// signal for whatever the slot holds when the pump gets there, which is why
	// dropping a second one loses nothing.
	wake chan struct{}
}

func newPerchBox() *perchBox { return &perchBox{wake: make(chan struct{}, 1)} }

// set records a shoulder and wakes the pump. It NEVER BLOCKS: the command
// goroutine must not wait on the pump, or a busy projection would stall the
// connection's whole command loop.
func (b *perchBox) set(actorID string) {
	b.mu.Lock()
	b.shoulder, b.full = actorID, true
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default: // already signalled; the pump will read whatever the slot holds
	}
}

// take empties the slot. ok is false for a wake-up whose shoulder a previous
// take already collected — see set's dropped second signal.
func (b *perchBox) take() (actorID string, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	actorID, ok = b.shoulder, b.full
	b.full = false
	return actorID, ok
}

// newSeat builds the seat for one connection. after is the client's resume
// cursor, ignored for the unprojected roles because their subscription starts
// there instead.
func newSeat(p *identity.Participant, after int64) *seat {
	if !projected(p.Role) {
		return &seat{}
	}
	return &seat{pr: NewProjector(viewerFor(p)), resume: after}
}

// subscribeFrom is where this seat's SUBSCRIPTION starts, which is not where
// its output starts. A projected seat always replays the whole log (see the
// type's doc comment); everybody else resumes strictly after their cursor, as
// they always have.
func (s *seat) subscribeFrom(after int64) int64 {
	if s.pr == nil {
		return after
	}
	return 0
}

// receive is what this seat is given for one log event: zero, one, or several
// envelopes.
//
// The unprojected seats get the event itself, by pointer, unchanged — the same
// value the pump encoded before this function existed.
func (s *seat) receive(env *vttv1.Envelope) []*vttv1.Envelope {
	if s.pr == nil {
		return []*vttv1.Envelope{env}
	}

	s.received = append(s.received, env)
	// The state AFTER env, which is what Project is specified to read. Not
	// campaign.State(): that is HEAD, and during catch-up head is the future.
	// Judging a two-hour-old move against where the tokens stand now would
	// forward a walk between squares nobody occupied at the time.
	world, err := campaign.FoldPrefix(s.received)
	if err != nil {
		// FAIL CLOSED. A fold that does not replay means this connection
		// cannot be told what it may see, and forwarding an event nobody
		// judged is the leak direction (spec §4.4).
		//
		// Unreachable while campaign.Append is the only writer: it applies
		// every envelope to the live projection before it persists, so an
		// envelope that cannot fold never reaches the log — and a log that
		// cannot fold poisons the Campaign at Open, which fails the
		// subscription before any of this runs. Logged rather than swallowed
		// so that if it ever does happen it is not a board that quietly
		// stopped updating.
		//
		// THE SEQUENCE AND NOTHING ELSE FROM THE ENVELOPE, and the one
		// exemption below is measured rather than assumed. gosec's G706 (log
		// injection) taints everything reached through a decoded envelope,
		// and it flags this call for the SEQUENCE alone — checked by removing
		// each argument in turn until the finding cleared. That one is a false
		// positive on its own terms: Envelope.Sequence is an int64, so there
		// is no newline for a forged log line to hide behind, and slog is
		// given it as a structured attribute rather than interpolated into
		// the message. The event id, which IS participant-supplied text and
		// would be a real finding, is deliberately not logged; the sequence
		// names the event exactly as well.
		//
		// The alternative was to log nothing at all, and a board that quietly
		// stopped updating with no record anywhere is worse than a suppressed
		// false positive with its reasoning written down.
		// #nosec G706 -- int64 sequence, structured attribute; see above.
		slog.Error("gateway: projection fold failed; withholding event",
			"sequence", env.GetSequence(), "error", err)
		return nil
	}

	// Only after the fold succeeded: a failed fold leaves the last good world
	// in place rather than a nil one a perch would then have nothing to look at.
	s.world = world

	return s.pastResume(s.pr.Project(env, world))
}

// pastResume drops what this seat's client already holds.
//
// NO FAST PATH FOR resume == 0, and its absence is deliberate. The obvious
// `if s.resume <= 0 { return out }` is a pure optimisation with no behavioural
// content — every logged envelope carries a sequence of at least 1, so at
// resume 0 the filter below keeps every one of them — and the mutation gate
// says so: weakening that guard to `< 0` survives every test in this package
// because nothing can distinguish the two. Removing it removes an unkillable
// mutant rather than adjudicating one, and the adjudications this branch has
// had to re-key four times are the argument for preferring the deletion.
//
// REPLAYED OUTPUT ONLY. A perch does NOT come through here, and that is a
// correction rather than an omission: it was filtered here at first, and a
// spectator who dialled with a cursor at or past the log's head was then
// answered ok and sent no board at all — measured at resume=6 against a seat
// whose head was 6, three frames became zero. The filter's question is "does
// this client already hold this?", and it answers it by SEQUENCE, which only
// means anything for output the log caused. A perch is caused by no event
// (perchSequence says why it carries no number), so the question does not apply
// to it and asking it anyway silently discarded the answer.
func (s *seat) pastResume(out []*vttv1.Envelope) []*vttv1.Envelope {
	kept := make([]*vttv1.Envelope, 0, len(out))
	for _, e := range out {
		// STRICTLY GREATER, matching store.Subscribe's own `seq > afterSeq`:
		// `after=N` means "I have N", so N is not sent again.
		if e.GetSequence() > s.resume {
			kept = append(kept, e)
		}
	}
	return kept
}

// perch moves a spectator onto a new shoulder and returns the envelopes that
// bring their board up to what those eyes can see (spec §3.1.1). The caller
// sends them; this function touches no wire and appends nothing.
//
// AT ONCE, rather than at the next event, because that is what "you can choose
// to shift to another character's view, whenever" means at a quiet table. It
// costs one look() — the same sight computation every event already pays for —
// against the world this seat last folded.
//
// AGAINST THAT WORLD AND NOT AGAINST HEAD, deliberately. The two differ during
// catch-up, where head is the future, and judging a perch against a world this
// seat has not been shown yet would introduce a token whose arrival it is still
// replaying.
//
// CALLED ONLY BY THE PUMP, like receive, and that is what makes the two safe
// beside each other: the goroutine that changes what this seat has been shown
// is the goroutine that says so on the wire. See the type's own comment for the
// mutex that used to be here and why it was the wrong tool.
//
// ITS OUTPUT SKIPS pastResume — see that function; a perch is not replay.
func (s *seat) perch(actorID string) []*vttv1.Envelope {
	if s.pr == nil {
		// The DM and the agent (see the field's comment). Authorize denies
		// them set_viewpoint, so this is defence in depth rather than a path:
		// an unprojected seat has no projection to re-run, and inventing one
		// here would put a filtered stream on the one wire exit criterion 8
		// says is byte-for-byte unchanged.
		return nil
	}
	return s.pr.reperch(actorID, s.world)
}

// canSee reports whether this viewer can see one square of one scene right
// now. It is look() asked a single question, and it holds no memory: a fresh
// Projector's maps are untouched by look, so nothing here records that the
// viewer was shown anything.
func canSee(v Viewer, st *engine.State, sceneID string, at *vttv1.GridPosition) bool {
	pr := NewProjector(v)
	return pr.canSeeSquare(pr.look(st), sceneID, at)
}

// catchUp drains this seat's preloaded backlog, projects it, and answers the
// two things serve needs: the frames to send, and the sequence THIS SEAT's
// catch-up actually ends at.
//
// WHY THE HEAD CANNOT SIMPLY BE THE LOG'S. CatchUpHead is a promise — a client
// wanting a point-in-time snapshot "reads until it has seen head_sequence"
// (commands.proto), and `vtt state dump` does exactly that. While every seat
// received every event, the log's head and a seat's head were the same number.
// A projection breaks that: the last events in the log may be withheld from
// this seat entirely, so it would wait for a frame that never comes and fail
// on its deadline. Answering with the last sequence this seat will be SENT
// keeps the promise keepable for every role.
//
// UNPROJECTED SEATS DO NOT ENTER THIS AT ALL. They return the log's head
// untouched and drain nothing, so their opening frames and their timing are
// what they have always been (spec §3.1, exit criterion 8).
//
// THE COST, since it inverts an ordering the pump's own comment defends: for a
// projected seat the head frame now waits for the whole backlog to be
// projected, where it used to go out first. Nothing else moves — the frames
// still leave in the same order — and the wait is dominated by the projection
// this seat was going to pay for anyway (visibility spec §8's cliff). A head
// that arrives promptly and names a sequence that never comes is not the
// better trade.
func (s *seat) catchUp(ctx context.Context, events <-chan *vttv1.Envelope, logHead int64) ([]*vttv1.Envelope, int64) {
	if s.pr == nil || logHead <= 0 {
		return nil, logHead
	}
	var out []*vttv1.Envelope
	var head int64
	for {
		select {
		case env, ok := <-events:
			if !ok {
				// The subscription ended mid-backlog. Answer with what this
				// seat actually got: the connection is finished either way,
				// and a head it cannot reach would be a second failure on top
				// of the first.
				return out, head
			}
			projected := s.receive(env)
			out = append(out, projected...)
			// THE LAST ONE, not the largest of them, and they are the same
			// number by construction: every envelope Project returns for one
			// event carries that event's sequence — the synthesized ones are
			// stamped with it and the forwarded one IS it — so a batch is flat
			// and batches only ascend. Written as a max comparison first,
			// which the mutation gate correctly called unkillable: taking the
			// max of equal values cannot tell `>` from `>=`.
			if n := len(projected); n > 0 {
				head = projected[n-1].GetSequence()
			}
			// Envelopes arrive in order, so the first one at or past the log's
			// head is the end of the backlog and everything after it is live.
			if env.GetSequence() >= logHead {
				return out, head
			}
		case <-ctx.Done():
			return out, head
		}
	}
}
