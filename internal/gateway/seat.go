package gateway

import (
	"log/slog"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
	"github.com/PatrikLager/vtt-platform/internal/campaign"
	"github.com/PatrikLager/vtt-platform/internal/engine"
	"github.com/PatrikLager/vtt-platform/internal/identity"
)

// viewerFor is the seat a participant occupies, as the projection sees it.
//
// Viewpoint is deliberately empty. It is the SPECTATOR PERCH — which shoulder
// a watcher is riding — and it arrives from the client over SetViewpoint,
// which is Task 6's command and does not exist yet. Until it does, a spectator
// perches on nobody, Projector.eyes returns no eyes for them, and they see no
// board at all. That is the fail-closed direction (spec §4.4) and it is the
// direction a missing feature has to run in: a default perch chosen by the
// server would be a shoulder nobody asked to ride, and Viewer's own doc
// comment records why the wrong shoulder undoes this whole arc in one click.
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

	out := s.pr.Project(env, world)
	// NO FAST PATH FOR resume == 0, and its absence is deliberate. The obvious
	// `if s.resume <= 0 { return out }` is a pure optimisation with no
	// behavioural content — every logged envelope carries a sequence of at
	// least 1, so at resume 0 the filter below keeps every one of them — and
	// the mutation gate says so: weakening that guard to `< 0` survives every
	// test in this package because nothing can distinguish the two. Removing
	// it removes an unkillable mutant rather than adjudicating one, and the
	// adjudications this branch has had to re-key four times are the argument
	// for preferring the deletion.
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

// canSee reports whether this viewer can see one square of one scene right
// now. It is look() asked a single question, and it holds no memory: a fresh
// Projector's maps are untouched by look, so nothing here records that the
// viewer was shown anything.
func canSee(v Viewer, st *engine.State, sceneID string, at *vttv1.GridPosition) bool {
	pr := NewProjector(v)
	return pr.canSeeSquare(pr.look(st), sceneID, at)
}
